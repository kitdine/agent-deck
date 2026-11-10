import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

@MainActor
final class TelemetrySettingsControllerTests: XCTestCase {
	private func status(
		enabled: Bool = true, listener: String = "running", configured: String = "unknown", received: String? = nil,
		refused: String? = nil, metering: String = "capture_only", memories: String = "unknown", exporter: String = "absent"
	) -> DesktopTelemetryStatusV1 {
		DesktopTelemetryStatusV1(
			receiver: enabled ? "enabled" : "disabled", listener: listener, producerConfigured: configured,
			lastReceivedAt: received, lastRefusedAt: refused, metering: enabled ? metering : "disabled",
			codexMemories: memories, codexExporter: exporter
		)
	}

	func testStatesFollowObservedFactsAndNeverInferFromSilence() {
		XCTAssertEqual(TelemetrySettingsState(status: status(enabled: false, listener: "stopped")), .off)
		XCTAssertEqual(TelemetrySettingsState(status: status()), .unknown, "enabled with no event is unknown, not unconfigured or disconnected")
		XCTAssertEqual(TelemetrySettingsState(status: status(listener: "stopped")), .unknown)
		XCTAssertEqual(TelemetrySettingsState(status: status(configured: "false")), .unconfigured)
		XCTAssertEqual(TelemetrySettingsState(status: status(received: "2026-10-29T00:00:00Z")), .captureOnly)
		XCTAssertEqual(TelemetrySettingsState(status: status(received: "2026-10-29T00:00:00Z", metering: "reconciled")), .ready)
		XCTAssertEqual(TelemetrySettingsState(status: status(listener: "failed")), .disconnected)
		XCTAssertEqual(TelemetrySettingsState(status: status(refused: "2026-10-29T00:00:00Z")), .disconnected)
		XCTAssertEqual(TelemetrySettingsState(status: status(enabled: false, listener: "stopped", memories: "off")), .memoriesOff)
		XCTAssertEqual(TelemetrySettingsState(status: status(listener: "stopped", memories: "off")), .memoriesStopped)
	}

	func testCollectionNeverTurnsOnByItselfAndMemoriesOnOnlyWarns() async {
		let supervisor = StubTelemetryListenerSupervisor()
		let transport = StubTelemetryTransport(statuses: [status(enabled: false, listener: "stopped", memories: "on")])
		let controller = makeTelemetrySettingsController(transport: transport, supervisor: supervisor)
		await controller.load()
		XCTAssertFalse(controller.isOn)
		XCTAssertTrue(controller.showsNotCounted)
		XCTAssertTrue(controller.switchEnabled)
		XCTAssertEqual(supervisor.starts, 0)
		XCTAssertEqual(transport.enableCalls, [])
	}

	func testMemoriesOffDisablesAnOffSwitchAndKeepsTheListenerStopped() async {
		let supervisor = StubTelemetryListenerSupervisor()
		let controller = makeTelemetrySettingsController(
			transport: StubTelemetryTransport(statuses: [status(enabled: false, listener: "stopped", memories: "off")]), supervisor: supervisor)
		await controller.load()
		XCTAssertFalse(controller.switchEnabled)
		XCTAssertEqual(controller.stateText, t(DesktopCopy.settingsOTelMemoriesOff))
		await controller.toggle(true)
		XCTAssertNil(controller.consent)
		let enabledButOff = makeTelemetrySettingsController(
			transport: StubTelemetryTransport(statuses: [status(listener: "stopped", memories: "off")]), supervisor: supervisor)
		await enabledButOff.load()
		XCTAssertEqual(supervisor.starts, 0, "the listener does not run while Memories is off")
		XCTAssertTrue(enabledButOff.isOn)
		XCTAssertTrue(enabledButOff.switchEnabled, "an enabled preference can still be turned off")
		XCTAssertEqual(enabledButOff.stateText, t(DesktopCopy.settingsOTelMemoriesStopped))
	}

	func testTurningOnAsksForConsentBeforeWritingCodexConfiguration() async {
		let supervisor = StubTelemetryListenerSupervisor()
		let transport = StubTelemetryTransport(statuses: [status(enabled: false, listener: "stopped", memories: "on"), status(listener: "stopped", memories: "on", exporter: "managed")])
		let controller = makeTelemetrySettingsController(transport: transport, supervisor: supervisor)
		await controller.load()
		await controller.toggle(true)
		XCTAssertEqual(controller.consent, .write)
		XCTAssertEqual(transport.enableCalls, [], "nothing is written before consent")
		controller.cancelConsent()
		XCTAssertNil(controller.consent)
		XCTAssertEqual(transport.enableCalls, [])
		await controller.toggle(true)
		await controller.writeAndEnable()
		XCTAssertEqual(transport.enableCalls, [true])
		XCTAssertEqual(controller.result, .written)
		XCTAssertEqual(controller.resultText, t(DesktopCopy.settingsOTelWritten))
		XCTAssertNil(controller.failureRow)
		XCTAssertTrue(supervisor.isRunning)
		XCTAssertTrue(controller.isOn)
	}

	func testConfigurationChangesResumeReceptionWithoutReopeningSettings() async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let config = directory.appendingPathComponent("config.toml")
		try "[features]\nmemories = false\n".write(to: config, atomically: true, encoding: .utf8)
		let supervisor = StubTelemetryListenerSupervisor()
		let transport = StubTelemetryTransport(statuses: [status(listener: "stopped", memories: "off"), status(listener: "stopped", memories: "on")])
		let controller = TelemetrySettingsController(transport: transport, supervisor: supervisor, configurationURLs: [config])
		defer { controller.stop() }
		await controller.load()
		XCTAssertEqual(supervisor.starts, 0)
		// A real atomic configuration replacement, while the receiver is stopped.
		try "[features]\nmemories = true\n".write(to: config, atomically: true, encoding: .utf8)
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: .seconds(5))
		while !supervisor.isRunning, clock.now < deadline { try await Task.sleep(for: .milliseconds(20)) }
		XCTAssertTrue(supervisor.isRunning)
		XCTAssertEqual(supervisor.starts, 1)
		XCTAssertEqual(transport.enableCalls, [], "the existing preference resumes without enabling or rewriting configuration")
	}

	func testEnablingTheReceiverElsewhereStartsReceptionWithoutReopeningSettings() async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let codexConfig = directory.appendingPathComponent("config.toml")
		let receiverConfig = directory.appendingPathComponent("receiver.json")
		try "[features]\nmemories = true\n".write(to: codexConfig, atomically: true, encoding: .utf8)
		try #"{"enabled":false}"#.write(to: receiverConfig, atomically: true, encoding: .utf8)
		let supervisor = StubTelemetryListenerSupervisor()
		let transport = StubTelemetryTransport(statuses: [status(enabled: false, listener: "stopped", memories: "on", exporter: "managed"), status(listener: "stopped", memories: "on", exporter: "managed")])
		let controller = TelemetrySettingsController(transport: transport, supervisor: supervisor, configurationURLs: [codexConfig, receiverConfig])
		defer { controller.stop() }
		await controller.load()
		XCTAssertEqual(supervisor.starts, 0)
		// `telemetry enable` with a configured exporter changes only this file.
		try #"{"enabled":true}"#.write(to: receiverConfig, atomically: true, encoding: .utf8)
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: .seconds(5))
		while !supervisor.isRunning, clock.now < deadline { try await Task.sleep(for: .milliseconds(20)) }
		XCTAssertTrue(supervisor.isRunning)
		XCTAssertEqual(supervisor.starts, 1)
		XCTAssertEqual(transport.enableCalls, [], "the CLI already saved the preference")
	}

	func testARefusedWriteKeepsTheSwitchOffAndRetryClearsTheError() async {
		var refuse = true
		let transport = StubTelemetryTransport(
			statuses: [status(enabled: false, listener: "stopped", memories: "on"), status(enabled: false, listener: "stopped", memories: "on"), status(memories: "on", exporter: "managed")],
			enableResult: { write in refuse ? nil : DesktopTelemetryReceiverResultV1(receiver: "enabled", codexConfig: write ? "written" : "unchanged") }
		)
		let controller = makeTelemetrySettingsController(transport: transport)
		await controller.load()
		await controller.toggle(true)
		await controller.writeAndEnable()
		XCTAssertFalse(controller.isOn)
		XCTAssertEqual(controller.failureRow, SettingsRowStatus(text: t(DesktopCopy.settingsOTelRefused), severity: .error))
		refuse = false
		await controller.toggle(true)
		await controller.writeAndEnable()
		XCTAssertNil(controller.failureRow)
		XCTAssertEqual(controller.result, .written)
	}

	func testAnExistingExporterIsNeverOverwrittenAndUserConfigurationIsKept() async {
		let conflict = StubTelemetryTransport(statuses: [status(enabled: false, listener: "stopped", memories: "on", exporter: "other"), status(memories: "on", exporter: "other")])
		let controller = makeTelemetrySettingsController(transport: conflict)
		await controller.load()
		await controller.toggle(true)
		XCTAssertEqual(controller.consent, .conflict)
		await controller.enableReceiverOnly()
		XCTAssertEqual(conflict.enableCalls, [false], "receiver only never writes Codex configuration")
		XCTAssertTrue(controller.showsPreview)
		XCTAssertEqual(conflict.previewLoads, 1)

		let userOwned = StubTelemetryTransport(statuses: [status(enabled: false, listener: "stopped", memories: "on", exporter: "agentdeck"), status(memories: "on", exporter: "agentdeck"), status(enabled: false, listener: "stopped", memories: "on", exporter: "agentdeck")])
		let direct = makeTelemetrySettingsController(transport: userOwned)
		await direct.load()
		await direct.toggle(true)
		XCTAssertNil(direct.consent, "a user-owned exporter enables directly")
		XCTAssertEqual(userOwned.enableCalls, [false])
		await direct.toggle(false)
		XCTAssertEqual(direct.result, .userConfigKept)
		XCTAssertEqual(direct.resultText, t(DesktopCopy.settingsOTelUserConfigKept))
	}

	func testTurningOffReportsRemovalOrAnIncompleteRestore() async {
		let removing = StubTelemetryTransport(
			statuses: [status(memories: "on", exporter: "managed"), status(enabled: false, listener: "stopped", memories: "on")],
			disableResult: { DesktopTelemetryReceiverResultV1(receiver: "disabled", codexConfig: "removed") }
		)
		let supervisor = StubTelemetryListenerSupervisor()
		let controller = makeTelemetrySettingsController(transport: removing, supervisor: supervisor)
		await controller.load()
		await controller.toggle(false)
		XCTAssertEqual(controller.result, .removed)
		XCTAssertFalse(supervisor.isRunning)
		XCTAssertFalse(controller.isOn)

		let incomplete = StubTelemetryTransport(
			statuses: [status(memories: "on", exporter: "managed"), status(enabled: false, listener: "stopped", memories: "on", exporter: "other")],
			disableResult: { DesktopTelemetryReceiverResultV1(receiver: "disabled", codexConfig: "restore_incomplete") }
		)
		let warned = makeTelemetrySettingsController(transport: incomplete)
		await warned.load()
		await warned.toggle(false)
		XCTAssertFalse(warned.isOn)
		XCTAssertEqual(warned.failureRow, SettingsRowStatus(text: t(DesktopCopy.settingsOTelRestoreIncomplete), severity: .warning))
	}

	func testAListenerThatFailsToStartLeavesTheSwitchOffAndTheFailureVisible() async {
		let supervisor = StubTelemetryListenerSupervisor()
		let transport = StubTelemetryTransport(statuses: [status(enabled: false, listener: "stopped", exporter: "managed"), status(listener: "failed", exporter: "managed")])
		let controller = makeTelemetrySettingsController(transport: transport, supervisor: supervisor)
		await controller.load()
		await controller.toggle(true)
		XCTAssertEqual(transport.disableCalls, 1, "the preference returns to the real state")
		XCTAssertFalse(supervisor.isRunning)
		XCTAssertFalse(controller.isOn)
		XCTAssertEqual(controller.failureRow?.text, t(DesktopCopy.settingsOTelDisconnected))
	}

	func testLiveUsageCopyMatchesThePrototype() throws {
		let expected: [String: [String: String]] = [
			"en": [
				DesktopCopy.settingsOTelPrivacy: "Only usage, model and required identity fields are retained. Prompts, response text and tool output are not stored.",
				DesktopCopy.settingsOTelMemoriesStopped: "Codex Memories turned off · Receiver stopped",
				DesktopCopy.settingsOTelRefused: "Could not update the Codex configuration; nothing was changed. Retry or configure it yourself.",
			],
			"zh-Hans": [
				DesktopCopy.settingsOTel: "启用 OTel 用量采集",
				DesktopCopy.settingsOTelCaptureOnly: "正在接收 · 用量尚未计入",
				DesktopCopy.settingsOTelConsentTitle: "开启采集需要修改 Codex 配置",
				DesktopCopy.settingsOTelUserConfigKept: "接收端已关闭 · 你自行添加的 Codex OTel 配置未改动",
				DesktopCopy.settingsAutomaticUpdateNote: "日志变化时更新，无需设置周期。关闭后仍可手动更新；OTel 采集单独控制。",
			],
		]
		let keys = [TelemetrySettingsState.off, .unknown, .unconfigured, .ready, .captureOnly, .disconnected, .memoriesOff, .memoriesStopped].map(\.copyKey)
			+ [TelemetryChangeResult.written, .refused, .selfConfigured, .removed, .restoreIncomplete, .userConfigKept].map(\.copyKey)
		for language in ["en", "zh-Hans"] {
			let path = try XCTUnwrap(Bundle.main.path(forResource: language, ofType: "lproj"))
			let localized = try XCTUnwrap(Bundle(path: path))
			for key in keys {
				XCTAssertNotEqual(localized.localizedString(forKey: key, value: "missing", table: nil), "missing", "\(language): \(key)")
			}
			for (key, value) in expected[language] ?? [:] {
				XCTAssertEqual(localized.localizedString(forKey: key, value: nil, table: nil), value)
			}
		}
	}
}

private actor TelemetryRootSequence {
    var roots: [URL]
    init(_ roots: [URL]) { self.roots = roots }
    func next() throws -> URL {
        guard !roots.isEmpty else { throw HelperExecutionError.launchFailed }
        return roots.removeFirst()
    }
}

extension TelemetrySettingsControllerTests {
    func testReceiverMonitorFollowsResolvedRootAndDropsItOnFailure() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let first = directory.appendingPathComponent("first")
        let second = directory.appendingPathComponent("second")
        let roots = TelemetryRootSequence([first, second])
        let supervisor = StubTelemetryListenerSupervisor()
        let transport = StubTelemetryTransport(statuses: [status(enabled: false, listener: "stopped")])
        let controller = TelemetrySettingsController(transport: transport, supervisor: supervisor,
            stateRoot: { try await roots.next() })
        defer { controller.stop() }
        await controller.load()
        XCTAssertEqual(controller.receiverURL, first.appendingPathComponent("telemetry/receiver.json"))
        await controller.load()
        XCTAssertEqual(controller.receiverURL, second.appendingPathComponent("telemetry/receiver.json"))
        await controller.load()
        XCTAssertNil(controller.receiverURL)
        XCTAssertNil(controller.status)
        XCTAssertFalse(supervisor.isRunning)
        controller.stop()
        await controller.load()
        XCTAssertNil(controller.receiverURL)
    }
}
