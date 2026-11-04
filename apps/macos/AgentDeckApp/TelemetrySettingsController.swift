import AgentDeckShared
import Foundation
import Observation

/// The Settings "Live usage" status line (ux/settings.md, 状态 and 联动).
/// Derived only from what telemetry status reports: the switch, the
/// listener, observed events, the metering capability and the Codex facts
/// are separate, and silence never implies "not configured" or "disconnected".
enum TelemetrySettingsState: Equatable {
	case off
	case unknown
	case unconfigured
	case ready
	case captureOnly
	case disconnected
	case memoriesOff
	case memoriesStopped

	init(status: DesktopTelemetryStatusV1) {
		let memoriesOff = status.codexMemories == "off"
		guard status.receiverEnabled else {
			self = memoriesOff ? .memoriesOff : .off
			return
		}
		if memoriesOff {
			self = .memoriesStopped
		} else if status.listener == "failed" || status.lastRefusedAt != nil {
			// A bind failure or a request refused because it could not be
			// persisted is an explicit failure signal, never silence.
			self = .disconnected
		} else if status.producerConfigured == "false" {
			self = .unconfigured
		} else if status.lastReceivedAt == nil {
			self = .unknown
		} else if status.metering == "reconciled" {
			self = .ready
		} else {
			self = .captureOnly
		}
	}

	var copyKey: String {
		switch self {
		case .off: DesktopCopy.settingsOTelOff
		case .unknown: DesktopCopy.settingsOTelUnknown
		case .unconfigured: DesktopCopy.settingsOTelUnconfigured
		case .ready: DesktopCopy.settingsOTelReady
		case .captureOnly: DesktopCopy.settingsOTelCaptureOnly
		case .disconnected: DesktopCopy.settingsOTelDisconnected
		case .memoriesOff: DesktopCopy.settingsOTelMemoriesOff
		case .memoriesStopped: DesktopCopy.settingsOTelMemoriesStopped
		}
	}

	var hintKey: String? {
		switch self {
		case .unknown: DesktopCopy.settingsOTelUnknownHint
		case .unconfigured: DesktopCopy.settingsOTelSetup
		case .captureOnly: DesktopCopy.settingsOTelCaptureHint
		case .memoriesOff: DesktopCopy.settingsOTelMemoriesOffHint
		case .memoriesStopped: DesktopCopy.settingsOTelMemoriesStoppedHint
		case .off, .ready, .disconnected: nil
		}
	}
}

/// The inline consent area under the switch (never a modal).
enum TelemetryConsent: Equatable {
	case write
	case conflict
}

/// The outcome of the last change this controller made. Errors and warnings
/// render in the switch row; information renders under the status line. The
/// next successful change clears it.
enum TelemetryChangeResult: Equatable {
	case written
	case refused
	case selfConfigured
	case removed
	case restoreIncomplete
	case userConfigKept

	var copyKey: String {
		switch self {
		case .written: DesktopCopy.settingsOTelWritten
		case .refused: DesktopCopy.settingsOTelRefused
		case .selfConfigured: DesktopCopy.settingsOTelSelfConfigured
		case .removed: DesktopCopy.settingsOTelRemoved
		case .restoreIncomplete: DesktopCopy.settingsOTelRestoreIncomplete
		case .userConfigKept: DesktopCopy.settingsOTelUserConfigKept
		}
	}

	var rowSeverity: NoticeSeverity? {
		switch self {
		case .refused: .error
		case .restoreIncomplete: .warning
		case .written, .selfConfigured, .removed, .userConfigKept: nil
		}
	}
}

/// Owns the App-attached listener process. The listener holds a pipe to the
/// App as its stdin and exits when the pipe closes, so it stops with the App
/// even if the App is killed. Each `publication_needed` line requests a
/// publication refresh after settlement or finite inventory reconciliation.
@MainActor
protocol TelemetryListenerSupervising: AnyObject {
	var isRunning: Bool { get }
	func start()
	func stop()
}

@MainActor
final class TelemetryListenerSupervisor: TelemetryListenerSupervising {
	private let command: () throws -> (executableURL: URL, arguments: [String], environment: [String: String])
	private let publicationNeeded: @MainActor () -> Void
	private var process: Process?
	private var stdin: Pipe?
	/// Failure exits (status 1, such as a state lock held past the
	/// listener's own retries) restart with growing backoff, at most
	/// `maximumRestarts` times until the next explicit start. Status 0
	/// (Codex Memories off) and 2 (receiver disabled) mean it should not run.
	private var restarts = 0
	private var restartTask: Task<Void, Never>?
	private let maximumRestarts = 3

	init(
		command: @escaping () throws -> (executableURL: URL, arguments: [String], environment: [String: String]),
		publicationNeeded: @escaping @MainActor () -> Void = {}
	) {
		self.command = command
		self.publicationNeeded = publicationNeeded
	}

	var isRunning: Bool { process?.isRunning == true }

	func start() {
		restarts = 0
		launch()
	}

	private func launch() {
		restartTask = nil
		guard !isRunning, let resolved = try? command() else { return }
		let process = Process()
		let input = Pipe()
		let events = Pipe()
		process.executableURL = resolved.executableURL
		process.arguments = resolved.arguments
		process.environment = resolved.environment
		process.standardInput = input
		process.standardOutput = events
		process.standardError = FileHandle.nullDevice
		process.qualityOfService = .utility
		let notify = publicationNeeded
		events.fileHandleForReading.readabilityHandler = { handle in
			let data = handle.availableData
			guard !data.isEmpty else {
				handle.readabilityHandler = nil
				return
			}
			if String(decoding: data, as: UTF8.self).contains("publication_needed") {
				Task { @MainActor in notify() }
			}
		}
		process.terminationHandler = { [weak self] ended in
			let status = ended.terminationStatus
			let reason = ended.terminationReason
			Task { @MainActor in
				guard let self, self.process === ended else { return }
				self.process = nil
				self.stdin = nil
				self.restartAfterFailure(status: status, reason: reason)
			}
		}
		do {
			try process.run()
		} catch {
			events.fileHandleForReading.readabilityHandler = nil
			return
		}
		self.process = process
		stdin = input
	}

	private func restartAfterFailure(status: Int32, reason: Process.TerminationReason) {
		guard reason == .exit, status == 1, restarts < maximumRestarts else { return }
		restarts += 1
		let delay = Duration.seconds(5 * restarts)
		restartTask = Task { @MainActor [weak self] in
			try? await Task.sleep(for: delay)
			guard !Task.isCancelled else { return }
			self?.launch()
		}
	}

	func stop() {
		restartTask?.cancel()
		restartTask = nil
		guard let process else { return }
		try? stdin?.fileHandleForWriting.close()
		if process.isRunning {
			process.terminate()
		}
		self.process = nil
		stdin = nil
	}
}

/// Drives the Settings "Live usage" group (ux/settings.md). Collection never
/// turns on by itself. Turning it on writes the managed Codex block only after
/// the user agrees in the inline consent area; turning it off removes only an
/// unchanged managed block and never the user's own configuration. Stored
/// observations and the journal are always kept.
@MainActor
@Observable
final class TelemetrySettingsController {
	private let transport: any TelemetryTransport
	private let supervisor: any TelemetryListenerSupervising
	private let settleDelay: Duration
	@ObservationIgnored private var configurationMonitors: [DesktopSourceChangeMonitor] = []

	private(set) var status: DesktopTelemetryStatusV1?
	private(set) var preview: DesktopTelemetryPreviewV1?
	private(set) var consent: TelemetryConsent?
	private(set) var result: TelemetryChangeResult?
	private(set) var isApplying = false
	var showsPreview = false

	init(transport: any TelemetryTransport, supervisor: any TelemetryListenerSupervising, settleDelay: Duration = .milliseconds(800), configurationURLs: [URL] = []) {
		self.transport = transport
		self.supervisor = supervisor
		self.settleDelay = settleDelay
		configurationMonitors = configurationURLs.map { configurationURL in
			DesktopSourceChangeMonitor(configurationURL: configurationURL) { [weak self] in
				Task { @MainActor [weak self] in
					guard let self, !self.isApplying else { return }
					await self.load()
				}
			}
		}
	}

	var state: TelemetrySettingsState? { status.map(TelemetrySettingsState.init(status:)) }

	/// The switch shows the collection preference. A listener that failed to
	/// start is off. With Codex Memories off the switch stays usable only to
	/// turn collection off.
	var isOn: Bool {
		guard let status else { return false }
		return status.receiverEnabled && status.listener != "failed"
	}

	var switchEnabled: Bool {
		guard let status, !isApplying else { return false }
		return !(status.codexMemories == "off" && !status.receiverEnabled)
	}

	/// Memories is on while collection is off: its background usage is not counted.
	var showsNotCounted: Bool {
		guard let status else { return false }
		return status.codexMemories == "on" && !status.receiverEnabled
	}

	/// The switch row carries confirmed failures: the listener could not bind,
	/// a Codex write was refused, or the managed block could not be restored.
	var failureRow: SettingsRowStatus? {
		if let result, let severity = result.rowSeverity {
			return SettingsRowStatus(text: t(result.copyKey), severity: severity)
		}
		guard state == .disconnected else { return nil }
		return SettingsRowStatus(text: t(DesktopCopy.settingsOTelDisconnected), severity: .warning)
	}

	var stateText: String? {
		guard let state, state != .disconnected else { return nil }
		return t(state.copyKey)
	}

	var stateHint: String? { state?.hintKey.map { t($0) } }

	var resultText: String? {
		guard let result, result.rowSeverity == nil else { return nil }
		return t(result.copyKey)
	}

	/// Reads status; the App-attached listener runs only while collection is
	/// on and Codex Memories is not off. Called at launch (after startup
	/// settles) and when Settings opens.
	func load() async {
		configurationMonitors.forEach { $0.start() }
		await refresh()
		startListenerIfNeeded()
	}

	func toggle(_ on: Bool) async {
		guard let status, !isApplying else { return }
		if on {
			guard status.codexMemories != "off" else { return }
			if status.codexExporter == "managed" || status.codexExporter == "agentdeck" {
				await enable(writeCodexConfig: false, outcome: nil)
			} else {
				result = nil
				consent = status.codexExporter == "other" ? .conflict : .write
			}
		} else {
			await disable()
		}
	}

	func writeAndEnable() async {
		consent = nil
		await enable(writeCodexConfig: true, outcome: .written)
	}

	func configureMyself() async {
		consent = nil
		await enable(writeCodexConfig: false, outcome: .selfConfigured)
		await expandPreview()
	}

	func enableReceiverOnly() async {
		consent = nil
		await enable(writeCodexConfig: false, outcome: nil)
		await expandPreview()
	}

	func cancelConsent() {
		consent = nil
	}

	func loadPreview() async {
		guard case let .decoded(value) = await transport.loadTelemetryPreview() else { return }
		preview = value
	}

	func stop() {
		configurationMonitors.forEach { $0.stop() }
		supervisor.stop()
	}

	private func enable(writeCodexConfig: Bool, outcome: TelemetryChangeResult?) async {
		isApplying = true
		defer { isApplying = false }
		guard case .decoded = await transport.enableTelemetryReceiver(writeCodexConfig: writeCodexConfig) else {
			// A refused write changes nothing; the switch stays off.
			result = .refused
			await refresh()
			return
		}
		result = outcome
		await refresh()
		startListenerIfNeeded()
		try? await Task.sleep(for: settleDelay)
		await refresh()
		if status?.listener == "failed" {
			// The listener could not bind: keep the failure visible, and
			// return the preference to the real (off) state.
			let failed = status
			_ = await transport.disableTelemetryReceiver()
			supervisor.stop()
			status = failed
		}
	}

	private func disable() async {
		isApplying = true
		defer { isApplying = false }
		let exporter = status?.codexExporter
		consent = nil
		guard case let .decoded(value) = await transport.disableTelemetryReceiver() else {
			await refresh()
			return
		}
		supervisor.stop()
		switch value.codexConfig {
		case "removed": result = .removed
		case "restore_incomplete": result = .restoreIncomplete
		default: result = exporter == "agentdeck" ? .userConfigKept : nil
		}
		await refresh()
	}

	private func expandPreview() async {
		showsPreview = true
		await loadPreview()
	}

	private func startListenerIfNeeded() {
		guard let status, status.receiverEnabled, status.codexMemories != "off", status.listener != "running", status.listener != "failed",
			!supervisor.isRunning
		else { return }
		supervisor.start()
	}

	private func refresh() async {
		if case let .decoded(current) = await transport.loadTelemetryStatus() {
			status = current
		}
	}
}
