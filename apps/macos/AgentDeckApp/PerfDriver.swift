#if AGENTDECK_PERF
import AgentDeckShared
import AppKit

/// One step of a measurement script read from `AGENTDECK_PERF_SCRIPT`.
struct PerfScriptStep: Decodable {
	enum Operation: String, Decodable {
		case waitUsable = "wait_usable"
		case waitRebuild = "wait_rebuild"
		case sleep
		case click
		case close
		case select
		case idleWindow = "idle_window"
		case waitComplete = "wait_complete"
		/// The pre-declared alternative stimulus: send the status button's
		/// action directly (start = input.action). Used where the condition's
		/// start is not the click, e.g. P04/P11 measure from app.ready.
		case openAction = "open_action"
	}

	let op: Operation
	/// Milliseconds for sleep and the end-point timeout for click/select/wait_usable.
	var ms: Int?
	var repeatCount: Int?
	var dimension: String?
	var values: [String]?
	/// For select: the exact number of selections, cycling through `values`.
	var count: Int?
	/// Seconds for idle_window.
	var seconds: Int?
	/// For click: "event" posts mouse events routed by AppKit (start =
	/// input.event); "action" is the pre-declared alternative stimulus adopted
	/// in the measurement-harness review: the same events are built at the same
	/// place, then the button's action is sent to its target (start =
	/// input.action).
	var stimulus: String?
	/// For an event click: "report" records an unrouted click as
	/// input.unrouted and continues, for runs that measure routing itself.
	var unrouted: String?
	/// For idle_window: the window starts this long after model.usable, once
	/// no helper is alive; a helper still alive at the deadline is a product
	/// failure.
	var afterUsableMs: Int?
	var helperDeadlineMs: Int?

	enum CodingKeys: String, CodingKey {
		case op, ms, dimension, values, seconds, count, stimulus, unrouted
		case repeatCount = "repeat"
		case afterUsableMs = "after_usable_ms"
		case helperDeadlineMs = "helper_deadline_ms"
	}
}

struct PerfScript: Decodable {
	let steps: [PerfScriptStep]
}

/// Drives the App the way a user would, without system permissions: clicks are
/// mouse events posted to this process's own queue and routed by AppKit to the
/// status item button; selections use the view model's public intents. Every
/// timing comes from product markers, so the driver records only stimulus
/// markers and its own outcome. Only the Perf configuration compiles it.
@MainActor
final class PerfDriver {
	private let model: MenuBarViewModel
	private let itemController: MenuBarItemController
	private var eventNumber = 0
	/// When wait_usable last succeeded; the P06 window is timed from it.
	private var usableAt: ContinuousClock.Instant?
	/// Watches for another app taking the front while the popover is shown.
	private var resignObserver: NSObjectProtocol?
	/// Watches for the popover closing while the driver holds it open.
	private var closeObserver: NSObjectProtocol?
	/// The driver's stimulus showed the popover and the driver has not closed
	/// it since; a close it did not ask for is then an outside dismissal.
	private var holdingPopover = false
	/// Closes the driver asked for whose notification has not arrived yet.
	private var expectedCloses = 0
	private var interrupted = false

	init(model: MenuBarViewModel, itemController: MenuBarItemController) {
		self.model = model
		self.itemController = itemController
	}

	func start() {
		guard PerfTrace.isEnabled, let path = ProcessInfo.processInfo.environment["AGENTDECK_PERF_SCRIPT"],
			let data = FileManager.default.contents(atPath: path),
			let script = try? JSONDecoder().decode(PerfScript.self, from: data)
		else {
			PerfTrace.mark(.driverInvalid, PerfCount("script", 1))
			finish()
			return
		}
		// The system dismisses the transient popover when another app takes the
		// front. Whatever the step (a click, a selection, or P10 holding it open
		// while sources are written), its frames can then no longer appear: the
		// stimulus was interrupted from outside. The run ends at once and
		// `agentdeck-perf run` reruns the unit instead of counting a product
		// failure.
		resignObserver = NotificationCenter.default.addObserver(forName: NSApplication.willResignActiveNotification,
			object: nil, queue: .main) { [weak self] _ in
			MainActor.assumeIsolated {
				guard let self, self.itemController.popoverIsShown else { return }
				self.markInterrupted()
			}
		}
		// A click in another app, or a system prompt taking the key window,
		// closes the transient popover before the App resigns active, so the
		// resign check above no longer sees it shown. Any close of a held
		// popover that the driver did not ask for is that same interruption.
		closeObserver = NotificationCenter.default.addObserver(forName: NSPopover.didCloseNotification,
			object: nil, queue: .main) { [weak self] _ in
			MainActor.assumeIsolated {
				guard let self else { return }
				if self.expectedCloses > 0 {
					self.expectedCloses -= 1
				} else if self.holdingPopover {
					self.holdingPopover = false
					self.markInterrupted()
				}
			}
		}
		Task { @MainActor in
			for (index, step) in script.steps.enumerated() {
				guard !interrupted else { break }
				// A product that never becomes usable is a product failure, not a
				// tool failure: record it and end the run.
				let succeeded = await run(step)
				if interrupted { break }
				if !succeeded, step.op == .waitUsable {
					PerfTrace.mark(.driverPreconditionTimeout, PerfCount("step", Int64(index)))
					break
				}
				guard succeeded else {
					PerfTrace.mark(.driverInvalid, PerfCount("step", Int64(index)))
					break
				}
			}
			PerfTrace.mark(.driverDone)
			finish()
		}
	}

	private func finish() {
		if let resignObserver {
			NotificationCenter.default.removeObserver(resignObserver)
			self.resignObserver = nil
		}
		if let closeObserver {
			NotificationCenter.default.removeObserver(closeObserver)
			self.closeObserver = nil
		}
		PerfFrameProbe.shared.stop()
		PerfTrace.flush()
		fromRunLoop { NSApp.terminate(nil) }
	}

	/// Runs AppKit work as a plain main-queue block instead of inside the
	/// driver's Task job. Terminating or sending an action can spin a nested
	/// run loop; inside a Swift job that leaves the job's executor current while
	/// run-loop observers run, and their main-actor check then reads a stale
	/// executor. The main queue also wakes the run loop, which a run-loop
	/// perform block does not.
	private func fromRunLoop(_ body: @escaping @MainActor () -> Void) {
		DispatchQueue.main.async { MainActor.assumeIsolated(body) }
	}

	/// Runs one step; false means the run is tool-invalid. A product failure
	/// (no data frame before the timeout) is recorded and does not stop the run.
	private func run(_ step: PerfScriptStep) async -> Bool {
		let timeout = Duration.milliseconds(step.ms ?? 5000)
		switch step.op {
		case .waitUsable:
			let usable = await waitUntil(timeout) { self.model.surface == .dataSurface && self.model.perfDataFrame != nil }
			if usable { usableAt = ContinuousClock.now }
			return usable
		case .waitRebuild:
			guard let directory = ProcessInfo.processInfo.environment[PerfTrace.environmentKey],
				FileManager.default.createFile(atPath: directory + "/rebuild-request", contents: nil,
					attributes: [.posixPermissions: 0o600]) else { return false }
			return await waitUntil(timeout) {
				FileManager.default.fileExists(atPath: directory + "/rebuild-ready")
			}
		case .sleep:
			// An outside interruption ends a long hold (P10) at once.
			let end = ContinuousClock.now.advanced(by: timeout)
			while !interrupted, ContinuousClock.now < end {
				try? await Task.sleep(for: min(.milliseconds(100), end - ContinuousClock.now))
			}
			return true
		case .close:
			closeHeldPopover()
			try? await Task.sleep(for: .milliseconds(100))
			return !itemController.popoverIsShown
		case .click:
			for _ in 0..<(step.repeatCount ?? 1) {
				if itemController.popoverIsShown {
					closeHeldPopover()
					try? await Task.sleep(for: .milliseconds(100))
				}
				guard await click(timeout: timeout, action: step.stimulus == "action", reportUnrouted: step.unrouted == "report")
				else { return false }
			}
			return true
		case .select:
			guard let dimension = step.dimension, let values = step.values, !values.isEmpty else { return false }
			let total = step.count ?? values.count * (step.repeatCount ?? 1)
			for index in 0..<total {
				guard await select(dimension: dimension, value: values[index % values.count], timeout: timeout) else {
					return false
				}
			}
			return true
		case .idleWindow:
			guard let usableAt else { return false }
			let begin = usableAt.advanced(by: .milliseconds(step.afterUsableMs ?? 120_000))
			let deadline = usableAt.advanced(by: .milliseconds(step.helperDeadlineMs ?? 420_000))
			try? await Task.sleep(until: begin, clock: .continuous)
			guard await noHelperAlive(until: deadline) else {
				// A finite helper should have exited: a product failure for this
				// run, recorded without extending the wait.
				PerfTrace.mark(.driverPreconditionTimeout, PerfCount("helper_alive", 1))
				return true
			}
			return await idleWindow(seconds: step.seconds ?? 1800)
		case .openAction:
			// The earliest open a user can make: the status item is on screen.
			// An action sent before that shows nothing, so it is not a stimulus:
			// during launch a placed item is not yet a usable anchor.
			guard await statusItemReady(onScreen: true, required: true), let button = itemController.statusItem.button,
				let action = button.action
			else { return false }
			let routed = signal(timeout) { PerfFrameProbe.shared.onInputAction = $0 }
			let framed = signal(timeout) { done in PerfFrameProbe.shared.onVsync = { _ in done() } }
			PerfFrameProbe.shared.expectedKey = nil
			fromRunLoop { NSApp.sendAction(action, to: button.target, from: button) }
			guard await routed.value else {
				PerfTrace.mark(.driverInvalid, PerfCount("unrouted", 1))
				return false
			}
			// The item is placed, so the action shows the popover; whether its
			// frame arrived, or the system dismissed it, is judged as for a click.
			return await framedOrUndisturbed(framed)
		case .waitComplete:
			// Readiness polling only; the end time is the product's own marker.
			return await waitUntil(timeout) {
				self.model.coordinator.servingHeader?.usageCoverage.state == "complete"
					&& self.model.coordinator.servingHeader?.sessionCoverage.state == "complete"
			}
		}
	}

	private func click(timeout: Duration, action: Bool, reportUnrouted: Bool) async -> Bool {
		// Before the stimulus. An event click is routed by position, so its item
		// should be on screen; one that stays off screen is reported as
		// unrouted. The action stimulus needs only a placed item.
		if action {
			guard await statusItemReady(onScreen: false, required: true) else { return false }
		} else {
			_ = await statusItemReady(onScreen: true, required: false)
		}
		guard let button = itemController.statusItem.button, let window = button.window,
			let buttonAction = button.action
		else { return false }
		PerfFrameProbe.shared.expectedKey = nil
		let routed = signal(timeout) { PerfFrameProbe.shared.onInputAction = $0 }
		let framed = signal(timeout) { done in PerfFrameProbe.shared.onVsync = { _ in done() } }
		let location = button.convert(NSPoint(x: button.bounds.midX, y: button.bounds.midY), to: nil)
		let timestamp = ProcessInfo.processInfo.systemUptime
		for type in [NSEvent.EventType.leftMouseDown, .leftMouseUp] {
			eventNumber += 1
			guard let event = NSEvent.mouseEvent(with: type, location: location, modifierFlags: [], timestamp: timestamp,
				windowNumber: window.windowNumber, context: nil, eventNumber: eventNumber, clickCount: 1, pressure: 1)
			else { return false }
			// The alternative stimulus builds the same events and posts none;
			// its start is the input.action that handleClick records.
			guard !action else { continue }
			if type == .leftMouseDown {
				PerfTrace.mark(.inputEvent, at: PerfTrace.nanoseconds(fromUptimeSeconds: event.timestamp))
			}
			NSApp.postEvent(event, atStart: false)
		}
		if action {
			fromRunLoop { NSApp.sendAction(buttonAction, to: button.target, from: button) }
		}
		// Unrouted means AppKit never delivered the click to handleClick within
		// the endpoint timeout. A click queued behind a busy main thread is
		// routed late, and that delay is product latency from input.event.
		guard await routed.value else {
			if reportUnrouted {
				PerfTrace.mark(.inputUnrouted)
				return true
			}
			PerfTrace.mark(.driverInvalid, PerfCount("unrouted", 1))
			return false
		}
		return await framedOrUndisturbed(framed)
	}

	/// A missing frame is a product failure unless another app took the front
	/// and the system dismissed the transient popover: then the stimulus was
	/// interrupted from outside and the unit is rerun.
	private func framedOrUndisturbed(_ framed: Task<Bool, Never>) async -> Bool {
		if await framed.value {
			if itemController.popoverIsShown { holdingPopover = true }
			return true
		}
		if !itemController.popoverIsShown, !NSApp.isActive {
			markInterrupted()
			return false
		}
		return true
	}

	/// A close the driver asks for is never an interruption.
	private func closeHeldPopover() {
		holdingPopover = false
		if itemController.popoverIsShown { expectedCloses += 1 }
		itemController.closePopover()
	}

	private func markInterrupted() {
		guard !interrupted else { return }
		interrupted = true
		PerfTrace.mark(.driverInterrupted)
	}

	/// The status item is placed once its window is visible with a window
	/// number; an action sent before then shows nothing. It can be clicked once
	/// that window is also on a screen and not occluded. Window notifications
	/// end the wait as soon as that holds, so the stimulus is not delayed by a
	/// polling interval; the bounded poll is only a fallback for a change that
	/// posts no notification. Only a required wait that times out marks the run
	/// tool-invalid.
	private func statusItemReady(onScreen requiredOnScreen: Bool, required: Bool) async -> Bool {
		let satisfied = { () -> Bool in
			guard let window = self.itemController.statusItem.button?.window,
				window.isVisible, window.windowNumber > 0
			else { return false }
			return !requiredOnScreen || (window.screen != nil && window.occlusionState.contains(.visible))
		}
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: .seconds(10))
		var ready = satisfied()
		while !ready, clock.now < deadline {
			// A status item sits left of the frontmost app's menus; behind a wide
			// menu bar it is placed off screen and cannot be clicked. For an event
			// click the measured App asks for the front, as a user's own launch
			// does. It asks from here, outside launch, and only while its item is
			// off screen.
			if requiredOnScreen, !NSApp.isActive { NSApp.activate(ignoringOtherApps: true) }
			await nextWindowChange(within: .milliseconds(50))
			ready = satisfied()
		}
		if !ready, required, requiredOnScreen, !NSApp.isActive {
			// Another app keeps the front and its menu bar hides the item: the
			// stimulus is blocked from outside, so the unit is rerun.
			markInterrupted()
			return false
		}
		if !ready, required {
			let window = itemController.statusItem.button?.window
			PerfTrace.mark(.driverInvalid, PerfCount("status_item_window", window == nil ? 0 : 1),
				PerfCount("visible", window?.isVisible == true ? 1 : 0),
				PerfCount("window_number", Int64(window?.windowNumber ?? 0)),
				PerfCount("screen", window?.screen == nil ? 0 : 1),
				PerfCount("occlusion_visible", window?.occlusionState.contains(.visible) == true ? 1 : 0),
				PerfCount("frame_y", Int64(window?.frame.origin.y ?? -1)),
				PerfCount("app_active", NSApp.isActive ? 1 : 0))
		}
		return ready
	}

	private func select(dimension: String, value: String, timeout: Duration) async -> Bool {
		let framed = signal(timeout) { done in PerfFrameProbe.shared.onVsync = { _ in done() } }
		let before = model.perfDataFrame?.key
		switch dimension {
		case "client": model.selectClient(value)
		case "period": model.selectPeriod(value)
		case "panel":
			guard let panel = MenuBarPanel(rawValue: value) else { return false }
			model.selectPanel(panel)
		default: return false
		}
		// A selection that leaves the presented frame unchanged has no endpoint;
		// the script, not the product, is wrong.
		let expected = model.perfDataFrame?.key
		if let expected, expected == before {
			// The script selected what is already presented: no endpoint exists.
			PerfTrace.mark(.driverInvalid, PerfCount("noop_select", 1))
			return false
		}
		if expected == nil {
			// The selection presents no data frame; the select then times out
			// as a product failure. Record why, with integer flags only.
			PerfTrace.mark(.selectNoData, PerfCount("has_snapshot", model.snapshot == nil ? 0 : 1),
				PerfCount("has_period", model.activePeriod == nil ? 0 : 1))
		}
		// A nil frame can never be matched, so no unrelated frame ends the wait.
		PerfFrameProbe.shared.expectedKey = expected ?? Int64.min
		// As for a click: a frame that never arrives because another app took
		// the front and the system dismissed the popover is an interrupted
		// stimulus, not a product failure.
		let undisturbed = await framedOrUndisturbed(framed)
		PerfFrameProbe.shared.expectedKey = nil
		return undisturbed
	}

	/// P06's window starts only when no helper is alive. Helpers, including a
	/// detached scan worker, are visible only from outside the App: the driver
	/// asks `cmd/agentdeck-perf` through files in the trace directory, which
	/// answers once none is running. This polling precedes the window.
	private func noHelperAlive(until deadline: ContinuousClock.Instant) async -> Bool {
		guard let directory = ProcessInfo.processInfo.environment[PerfTrace.environmentKey] else { return false }
		FileManager.default.createFile(atPath: directory + "/idle-window-request", contents: nil,
			attributes: [.posixPermissions: 0o600])
		while !FileManager.default.fileExists(atPath: directory + "/idle-window-ready") {
			guard ContinuousClock.now < deadline else { return false }
			try? await Task.sleep(for: .seconds(1))
		}
		return true
	}

	/// P06: two self-readings at the window boundaries and one timer between.
	private func idleWindow(seconds: Int) async -> Bool {
		guard let begin = PerfTrace.windowResources() else { return false }
		PerfTrace.mark(.idleWindowBegin, counts: counts(begin))
		try? await Task.sleep(for: .seconds(seconds))
		guard let end = PerfTrace.windowResources() else { return false }
		PerfTrace.mark(.idleWindowEnd, counts: counts(end))
		// Tells the external sampler to take its App-only ps reading now.
		if let directory = ProcessInfo.processInfo.environment[PerfTrace.environmentKey] {
			FileManager.default.createFile(atPath: directory + "/idle-window-end", contents: nil,
				attributes: [.posixPermissions: 0o600])
		}
		// Stay alive, outside the window, until the sampler's App-only reading.
		try? await Task.sleep(for: .seconds(2))
		return true
	}

	private func counts(_ resources: PerfTrace.WindowResources) -> [PerfCount] {
		[PerfCount("user_ns", Int64(clamping: resources.userNanoseconds)),
			PerfCount("sys_ns", Int64(clamping: resources.systemNanoseconds)),
			PerfCount("resident_bytes", Int64(clamping: resources.residentBytes)),
			PerfCount("wakeups", Int64(clamping: resources.wakeups))]
	}

	/// Completes true when the installed callback fires, false at the timeout.
	/// Waiting is event-driven: nothing wakes the main thread while measured.
	private func signal(_ timeout: Duration, install: (@escaping @MainActor () -> Void) -> Void) -> Task<Bool, Never> {
		let gate = PerfSignal()
		install { gate.fire(true) }
		Task { @MainActor in
			try? await Task.sleep(for: timeout)
			gate.fire(false)
		}
		return Task { @MainActor in await gate.value() }
	}

	/// Completes on the next window occlusion or screen change, or after `limit`.
	private func nextWindowChange(within limit: Duration) async {
		let gate = PerfSignal()
		let center = NotificationCenter.default
		let observers = [NSWindow.didChangeOcclusionStateNotification, NSWindow.didChangeScreenNotification,
			NSApplication.didChangeScreenParametersNotification].map { name in
			center.addObserver(forName: name, object: nil, queue: .main) { _ in
				MainActor.assumeIsolated { gate.fire(true) }
			}
		}
		let timer = Task { @MainActor in
			try? await Task.sleep(for: limit)
			gate.fire(false)
		}
		_ = await gate.value()
		timer.cancel()
		observers.forEach(center.removeObserver)
	}

	/// Polls only before measurement starts (readiness). The interval stays
	/// coarse: a fast poll on the main actor during launch delays the App's own
	/// launch work (status item placement, helper completions) it would measure.
	private func waitUntil(_ timeout: Duration, _ condition: @MainActor () -> Bool) async -> Bool {
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: timeout)
		while !condition() {
			guard clock.now < deadline else { return false }
			try? await Task.sleep(for: .milliseconds(50))
		}
		return true
	}
}
@MainActor
private final class PerfSignal {
	private var result: Bool?
	private var waiter: CheckedContinuation<Bool, Never>?

	func fire(_ value: Bool) {
		guard result == nil else { return }
		result = value
		waiter?.resume(returning: value)
		waiter = nil
	}

	func value() async -> Bool {
		if let result { return result }
		return await withCheckedContinuation { waiter = $0 }
	}
}
#endif
