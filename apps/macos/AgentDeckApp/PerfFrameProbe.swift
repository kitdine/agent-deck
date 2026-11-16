import AgentDeckShared
import AppKit
import SwiftUI

/// Identifies the data a frame shows: publication, client, period and panel,
/// hashed so no identifier enters a trace, plus the hero's summary for the
/// correctness oracle: its integer totals, the client and period codes, the
/// calendar day its label names and a digest of the cost fields its amount is
/// formatted from.
struct PerfDataFrame: Equatable {
	let key: Int64
	let tokens: Int64
	let events: Int64
	let sessions: Int64
	var client: Int64 = 0
	var period: Int64 = 0
	/// yyyymmdd of the hero's date label; 0 when it has none.
	var day: Int64 = 0
	var cost: Int64 = 0
	/// The publication's usage coverage and whether the surface shows its
	/// partial marker: P11's first batch is checked against the expected hero
	/// of exactly that coverage.
	var partial: Int64 = 0
	var committed: Int64 = 0
	var sources: Int64 = 0
	/// 1 when the usage coverage state is complete: a bootstrap publishes the
	/// final count once before and once after it finalizes.
	var coverageComplete: Int64 = 0

	static func key(publication: String, client: String, period: String, panel: String) -> Int64 {
		fnv1a([publication, client, period, panel])
	}

	/// FNV-1a over NUL-joined fields: stable across processes, unlike Hasher.
	/// `cmd/agentdeck-perf` computes the same digests for its oracle.
	static func fnv1a(_ fields: [String]) -> Int64 {
		var hash: UInt64 = 0xcbf2_9ce4_8422_2325
		for byte in fields.joined(separator: "\u{0}").utf8 {
			hash = (hash ^ UInt64(byte)) &* 0x0000_0100_0000_01b3
		}
		return Int64(bitPattern: hash)
	}

	/// Integer codes shared with the oracle; 0 is any other identifier.
	static func code(client: String) -> Int64 {
		["all": 1, "codex": 2, "claude": 3][client] ?? 0
	}

	static func code(period: String) -> Int64 {
		["today": 1, "7d": 2, "30d": 3][period] ?? 0
	}

	/// The cost fields the hero amount is formatted from; a missing value is
	/// U+0001 so it differs from an empty string.
	static func cost(_ totals: DesktopPresentationTotalsV1) -> Int64 {
		fnv1a([totals.catalogBaseCost ?? "\u{1}", totals.providerCost ?? "\u{1}", totals.knownCatalogBaseCost,
			totals.knownProviderCost, totals.pricingComplete ? "1" : "0", String(totals.unpricedComponents)])
	}

	var counts: [PerfCount] {
		[PerfCount("key", key), PerfCount("tokens", tokens), PerfCount("events", events), PerfCount("sessions", sessions),
			PerfCount("client", client), PerfCount("period", period), PerfCount("day", day), PerfCount("cost", cost),
			PerfCount("partial", partial), PerfCount("committed", committed), PerfCount("sources", sources),
			PerfCount("coverage_complete", coverageComplete)]
	}
}

/// Implements the data-frame endpoint: a data view's layout records
/// `popover.layout.data`; the run-loop pass after Core Animation's commit
/// records `frame.commit` once the window is visible, key and shows the expected
/// key; the window's next display-link callback records `frame.vsync`.
@MainActor
final class PerfFrameProbe {
	static let shared = PerfFrameProbe()

	/// The frame the driver waits for; nil accepts any data frame.
	var expectedKey: Int64?
	/// Set by a selection intent so the next layout is recorded as a selection.
	private(set) var selectionPending = false

	/// A selection's frame must come from a layout after its intent, never
	/// from one already pending (for example a publication update).
	func beginSelection() {
		selectionPending = true
		pending = nil
		committed = nil
	}
	private(set) var lastVsync: (frame: PerfDataFrame, time: UInt64)?
	var onVsync: ((PerfDataFrame) -> Void)?
	private(set) var inputActions = 0
	var onInputAction: (() -> Void)?

	func inputActionHandled() {
		inputActions += 1
		onInputAction?()
	}

	private var pending: (frame: PerfDataFrame, window: NSWindow)?
	private var blockedRecorded = false
	fileprivate let probes = NSHashTable<NSView>.weakObjects()
	private var committed: PerfDataFrame?
	private var observer: CFRunLoopObserver?
	private var displayLink: CADisplayLink?
	private var displayLinkTarget: DisplayLinkTarget?

	private init() {}

	/// A popover show re-lays out its data view, so each open has its own frame.
	func rearm() {
		pending = nil
		committed = nil
		for case let probe as PerfLayoutProbe.ProbeView in probes.allObjects { probe.rearm() }
	}

	func layoutCompleted(_ frame: PerfDataFrame, in window: NSWindow?) {
		guard let window else { return }
		PerfTrace.mark(selectionPending ? .selectLayoutData : .popoverLayoutData, counts: frame.counts)
		selectionPending = false
		pending = (frame, window)
		blockedRecorded = false
		installObserverIfNeeded()
	}

	private func installObserverIfNeeded() {
		guard observer == nil else { return }
		// Core Animation commits in a beforeWaiting observer at order 2,000,000;
		// this one runs after it in the same pass.
		observer = CFRunLoopObserverCreateWithHandler(nil, CFRunLoopActivity.beforeWaiting.rawValue, true, 2_000_001) {
			_, _ in MainActor.assumeIsolated { PerfFrameProbe.shared.afterCommit() }
		}
		CFRunLoopAddObserver(CFRunLoopGetMain(), observer, .commonModes)
	}

	/// Removes the observer and display link before the process terminates, so
	/// no callback runs during teardown.
	func stop() {
		if let observer { CFRunLoopRemoveObserver(CFRunLoopGetMain(), observer, .commonModes) }
		observer = nil
		pending = nil
		committed = nil
		displayLink?.invalidate()
		displayLink = nil
		displayLinkTarget = nil
	}

	private func afterCommit() {
		guard let (frame, window) = pending else { return }
		guard window.isVisible, window.occlusionState.contains(.visible), window.isKeyWindow,
			expectedKey == nil || expectedKey == frame.key
		else {
			if !blockedRecorded {
				blockedRecorded = true
				PerfTrace.mark(.frameBlocked, PerfCount("visible", window.isVisible ? 1 : 0),
					PerfCount("occluded", window.occlusionState.contains(.visible) ? 0 : 1),
					PerfCount("key", window.isKeyWindow ? 1 : 0), PerfCount("app_active", NSApp.isActive ? 1 : 0),
					PerfCount("key_match", expectedKey == nil || expectedKey == frame.key ? 1 : 0))
			}
			return
		}
		pending = nil
		PerfTrace.mark(.frameCommit, counts: frame.counts)
		committed = frame
		let target = DisplayLinkTarget()
		displayLinkTarget = target
		displayLink?.invalidate()
		let link = window.displayLink(target: target, selector: #selector(DisplayLinkTarget.tick(_:)))
		link.add(to: .main, forMode: .common)
		displayLink = link
	}

	fileprivate func vsync() {
		displayLink?.invalidate()
		displayLink = nil
		displayLinkTarget = nil
		guard let frame = committed else { return }
		committed = nil
		let now = PerfTrace.now()
		PerfTrace.mark(.frameVsync, at: now, counts: frame.counts)
		lastVsync = (frame, now)
		onVsync?(frame)
	}
}

private final class DisplayLinkTarget: NSObject {
	@MainActor @objc func tick(_ link: CADisplayLink) {
		PerfFrameProbe.shared.vsync()
	}
}

/// Records the data view's completed layout. It exists only while recording,
/// so a normal process registers no view, observer or display link.
struct PerfLayoutProbe: NSViewRepresentable {
	let frame: PerfDataFrame

	func makeNSView(context: Context) -> ProbeView {
		let view = ProbeView()
		PerfFrameProbe.shared.probes.add(view)
		return view
	}

	func updateNSView(_ view: ProbeView, context: Context) {
		view.frameData = frame
		view.needsLayout = true
	}

	final class ProbeView: NSView {
		var frameData: PerfDataFrame?
		private var recorded: PerfDataFrame?

		override func layout() {
			super.layout()
			guard let frameData, frameData != recorded, window != nil else { return }
			recorded = frameData
			PerfFrameProbe.shared.layoutCompleted(frameData, in: window)
		}

		func rearm() {
			recorded = nil
			needsLayout = true
		}

		override func viewDidMoveToWindow() {
			super.viewDidMoveToWindow()
			// A reused hosting view entering a new window lays out a new frame.
			recorded = nil
			needsLayout = true
		}
	}
}

extension View {
	@ViewBuilder
	func perfDataFrame(_ frame: PerfDataFrame?) -> some View {
		if PerfTrace.isEnabled, let frame {
			background(PerfLayoutProbe(frame: frame).allowsHitTesting(false))
		} else {
			self
		}
	}
}
