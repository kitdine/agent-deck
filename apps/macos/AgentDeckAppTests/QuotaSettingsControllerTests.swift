import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

@MainActor
final class QuotaSettingsControllerTests: XCTestCase {
	func testLoadAdoptsCoreStateIntoBothTheControllerAndTheLocalPreferenceMirror() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let seeded = DesktopQuotaSettingsValuesV1(
			reading: true, interval: .fifteenMinutes, alerts: true, thresholds: [75, 90], resetNotice: true, statusline: true
		)
		let controller = makeQuotaSettingsController(preferences: preferences, transport: StubQuotaSettingsTransport(settings: seeded))

		await controller.load()

		XCTAssertEqual(controller.settings, seeded)
		XCTAssertTrue(preferences.quotaProbeEnabled, "the periodic-refresh scheduler reads this local mirror, not core state, on every tick")
		XCTAssertEqual(preferences.quotaProbeInterval, .fifteenMinutes)
	}

	func testSetReadingWritesCoreStateAndMirrorsLocallyForTheScheduler() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let transport = StubQuotaSettingsTransport()
		let controller = makeQuotaSettingsController(preferences: preferences, transport: transport)
		await controller.load()

		await controller.setReading(true)

		XCTAssertTrue(preferences.quotaProbeEnabled)
		XCTAssertEqual(controller.settings?.reading, true)
		let calls = await transport.applyCalls
		XCTAssertEqual(calls.count, 1)
		XCTAssertEqual(calls[0].reading, true)
	}

	/// requirements.md's default-off contract: a control moved before `load()`
	/// returns must not invent an on value for alerts/thresholds/reset-notice —
	/// it sends the quiet defaults alongside the one field the user actually
	/// touched.
	func testAChangeBeforeLoadCompletesSendsDefaultOffAlertFieldsAlongsideTheOneFieldTouched() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let transport = StubQuotaSettingsTransport()
		let controller = makeQuotaSettingsController(preferences: preferences, transport: transport)

		await controller.setReading(true)

		let calls = await transport.applyCalls
		XCTAssertEqual(calls.count, 1)
		XCTAssertEqual(calls[0].reading, true)
		XCTAssertFalse(calls[0].alerts)
		XCTAssertEqual(calls[0].thresholds, [])
		XCTAssertFalse(calls[0].resetNotice)
	}

	func testSetThresholdsSendsExactlyTheChosenPairNeverAPartialUpdate() async {
		let transport = StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
			reading: true, interval: .fiveMinutes, alerts: true, thresholds: [75], resetNotice: false, statusline: false
		))
		let controller = makeQuotaSettingsController(transport: transport)
		await controller.load()

		await controller.setThresholds(.both)

		XCTAssertEqual(controller.settings?.thresholds, [75, 90])
		let calls = await transport.applyCalls
		XCTAssertEqual(calls.last?.thresholds, [75, 90])
		// The write must still carry reading/alerts as they were — a
		// thresholds-only control change is not license to reset the rest.
		XCTAssertEqual(calls.last?.reading, true)
		XCTAssertEqual(calls.last?.alerts, true)
	}

	/// ux/settings-quota.md: turning reading off restores an installed
	/// status-line route as part of the same action, and a completed restore
	/// leaves no failure row.
	func testTurningReadingOffWithASuccessfulRestoreClearsConsentAndShowsNoRow() async {
		let transport = StubQuotaSettingsTransport(
			settings: DesktopQuotaSettingsValuesV1(reading: true, interval: .fiveMinutes, alerts: false, thresholds: [], resetNotice: false, statusline: true),
			statuslineOutcome: .removed
		)
		let controller = makeQuotaSettingsController(transport: transport)
		await controller.load()

		await controller.setReading(false)

		XCTAssertEqual(controller.settings?.statusline, false)
		XCTAssertNil(controller.statuslineRow)
	}

	/// ux/settings-quota.md's "Restore incomplete" outcome: the switch still
	/// goes off (the disable itself succeeded), but the row must say the
	/// previous value was not restored, in the warning tone, not the error one.
	func testTurningReadingOffWithAnIncompleteRestoreShowsAWarningRowAndStillClearsConsent() async {
		let transport = StubQuotaSettingsTransport(
			settings: DesktopQuotaSettingsValuesV1(reading: true, interval: .fiveMinutes, alerts: false, thresholds: [], resetNotice: false, statusline: true),
			statuslineOutcome: .restoreIncomplete
		)
		let controller = makeQuotaSettingsController(transport: transport)
		await controller.load()

		await controller.setReading(false)

		XCTAssertEqual(controller.settings?.statusline, false)
		XCTAssertEqual(controller.statuslineRow?.severity, .warning)
		XCTAssertEqual(controller.statuslineRow?.text, t(DesktopCopy.settingsQuotaStatuslineRestoreIncomplete))
	}

	/// ux/settings-quota.md's "Write refused" outcome: the switch must never
	/// flicker on when the write to `~/.claude/settings.json` fails.
	func testEnablingStatuslineOnAFailedWriteLeavesConsentOffWithAnErrorRow() async {
		let transport = StubQuotaSettingsTransport(
			settings: DesktopQuotaSettingsValuesV1(reading: true, interval: .fiveMinutes, alerts: false, thresholds: [], resetNotice: false, statusline: false),
			statuslineOutcome: .failed
		)
		let controller = makeQuotaSettingsController(transport: transport)
		await controller.load()

		await controller.setStatuslineConsent(true)

		XCTAssertEqual(controller.settings?.statusline, false, "a refused write must never leave the switch on")
		XCTAssertEqual(controller.statuslineRow?.severity, .error)
		XCTAssertEqual(controller.statuslineRow?.text, t(DesktopCopy.settingsQuotaStatuslineWriteRefused))
	}

	func testEnablingStatuslineSuccessfullyClearsAnyPriorFailureRow() async {
		let transport = StubQuotaSettingsTransport(
			settings: DesktopQuotaSettingsValuesV1(reading: true, interval: .fiveMinutes, alerts: false, thresholds: [], resetNotice: false, statusline: false),
			statuslineOutcome: .configured
		)
		let controller = makeQuotaSettingsController(transport: transport)
		await controller.load()

		await controller.setStatuslineConsent(true)

		XCTAssertEqual(controller.settings?.statusline, true)
		XCTAssertNil(controller.statuslineRow)
	}

	/// A transport-layer failure (helper missing, timeout, undecodable
	/// stdout) is distinct from a decoded `failed` outcome: nothing about the
	/// attempted state is known, so the row must say so generically rather
	/// than claim either of the two documented file-write failures.
	func testATransportFailureShowsTheGenericWriteFailedRowRatherThanAFileSpecificOne() async {
		let controller = makeQuotaSettingsController(transport: AlwaysUndecodableQuotaSettingsTransport())
		await controller.load()

		await controller.setReading(true)

		XCTAssertEqual(controller.settingsRow?.severity, .error)
		XCTAssertEqual(controller.settingsRow?.text, t(DesktopCopy.settingsQuotaWriteFailed))
	}

	/// A load that cannot decode leaves the controller with nothing to show
	/// rather than crashing or inventing a value — the window's controls
	/// simply stay at their local-mirror defaults until a write succeeds.
	func testLoadTransportFailureLeavesSettingsNilWithoutCrashing() async {
		let controller = makeQuotaSettingsController(transport: AlwaysUndecodableQuotaSettingsTransport())

		await controller.load()

		XCTAssertNil(controller.settings)
	}

	func testChainedStatusLineCommandPreviewReadsTheConfiguredCommandReadOnly() async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let settingsURL = directory.appendingPathComponent("settings.json")
		try #"{"statusLine":{"type":"command","command":"python3 ~/.claude/statusline.py"}}"#
			.write(to: settingsURL, atomically: true, encoding: .utf8)

		let controller = makeQuotaSettingsController(claudeSettingsURL: settingsURL)
		await controller.load()

		XCTAssertEqual(controller.chainedStatusLineCommand, "python3 ~/.claude/statusline.py")
	}

	func testChainedStatusLineCommandPreviewIsNilWithoutAFile() async {
		let controller = makeQuotaSettingsController()
		await controller.load()

		XCTAssertNil(controller.chainedStatusLineCommand)
	}

	func testOverlappingChangesCoalesceToTheLatestCompleteSettingsWithoutDroppingIntent() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let transport = SuspendingQuotaSettingsTransport()
		let controller = makeQuotaSettingsController(preferences: preferences, transport: transport)
		await controller.load()

		let first = Task { await controller.setAlerts(true) }
		await transport.waitForApplyCount(1)
		await controller.setThresholds(.both)
		let optimistic = try? XCTUnwrap(controller.settings)
		XCTAssertEqual(optimistic?.alerts, true)
		XCTAssertEqual(optimistic?.thresholds, [75, 90])

		await transport.completeNext(.success)
		await transport.waitForApplyCount(2)
		let calls = await transport.applyCalls
		XCTAssertEqual(calls[1].alerts, true)
		XCTAssertEqual(calls[1].thresholds, [75, 90])
		await transport.completeNext(.success)
		await first.value

		XCTAssertEqual(controller.settings?.alerts, true)
		XCTAssertEqual(controller.settings?.thresholds, [75, 90])
		XCTAssertEqual(preferences.quotaProbeEnabled, false)
		XCTAssertEqual(preferences.quotaProbeInterval, .fiveMinutes)
	}

	func testFailedWriteRetainsDesiredSettingsForTheNextRetryingChange() async {
		let transport = SuspendingQuotaSettingsTransport()
		let controller = makeQuotaSettingsController(transport: transport)
		await controller.load()

		let first = Task { await controller.setAlerts(true) }
		await transport.waitForApplyCount(1)
		await transport.completeNext(.failure)
		await first.value
		XCTAssertEqual(controller.settingsRow?.severity, .error)

		let retry = Task { await controller.setResetNotice(true) }
		await transport.waitForApplyCount(2)
		let calls = await transport.applyCalls
		XCTAssertEqual(calls[1].alerts, true)
		XCTAssertEqual(calls[1].resetNotice, true)
		await transport.completeNext(.success)
		await retry.value

		XCTAssertEqual(controller.settings?.alerts, true)
		XCTAssertEqual(controller.settings?.resetNotice, true)
		XCTAssertNil(controller.settingsRow)
	}
}

private actor SuspendingQuotaSettingsTransport: QuotaSettingsTransport {
	enum Completion { case success, failure }
	private var settings = StubQuotaSettingsTransport.defaultSettings
	private var pending = [(DesktopQuotaSettingsDesiredV1, CheckedContinuation<DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1>, Never>)]()
	private(set) var applyCalls = [DesktopQuotaSettingsDesiredV1]()

	func loadQuotaSettings() async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		.decoded(DesktopQuotaSettingsResultV1(settings: settings, statuslineRestore: nil))
	}

	func applyQuotaSettings(_ desired: DesktopQuotaSettingsDesiredV1) async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		applyCalls.append(desired)
		return await withCheckedContinuation { pending.append((desired, $0)) }
	}

	func setQuotaStatusLine(enabled _: Bool) async -> DesktopQuotaTransportOutcome<DesktopQuotaStatusLineResultV1> { .undecodable }

	func waitForApplyCount(_ count: Int) async {
		while applyCalls.count < count { await Task.yield() }
	}

	func completeNext(_ completion: Completion) {
		let (desired, continuation) = pending.removeFirst()
		guard case .success = completion else {
			continuation.resume(returning: .undecodable)
			return
		}
		settings = DesktopQuotaSettingsValuesV1(
			reading: desired.reading, interval: desired.interval, alerts: desired.alerts,
			thresholds: desired.thresholds, resetNotice: desired.resetNotice, statusline: settings.statusline
		)
		continuation.resume(returning: .decoded(DesktopQuotaSettingsResultV1(settings: settings, statuslineRestore: nil)))
	}
}
