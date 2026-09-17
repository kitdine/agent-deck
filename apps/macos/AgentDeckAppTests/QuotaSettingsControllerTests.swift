import AppKit
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

@MainActor
final class QuotaAlertPermissionTests: XCTestCase {
	func testTurningAlertsOnRequestsPermissionAndARefusalKeepsTheSwitchOnWithAWarning() async {
		let transport = StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
			reading: true, interval: .fiveMinutes, alerts: false, thresholds: [75, 90], resetNotice: false, statusline: false
		))
		let permission = StubNotificationPermission(granted: false)
		let controller = makeQuotaSettingsController(transport: transport, notifications: permission)
		await controller.load()
		let checksBeforeEnabling = await permission.checkCount
		XCTAssertEqual(checksBeforeEnabling, 0, "with alerts off, loading never consults the notification service")

		await controller.setAlerts(true)

		let requests = await permission.requestCount
		XCTAssertEqual(requests, 1)
		XCTAssertEqual(controller.settings?.alerts, true, "a refused permission must not turn the setting back off")
		XCTAssertEqual(
			controller.alertsRow,
			SettingsRowStatus(text: t(DesktopCopy.settingsQuotaAlertsNotificationsDenied), severity: .warning)
		)
		let calls = await transport.applyCalls
		XCTAssertEqual(calls.last?.alerts, true)
	}

	func testGrantedPermissionShowsNoRowAndTurningAlertsOffClearsItWithoutPrompting() async {
		let granted = StubNotificationPermission(granted: true)
		let controller = makeQuotaSettingsController(
			transport: StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
				reading: true, interval: .fiveMinutes, alerts: false, thresholds: [75], resetNotice: false, statusline: false
			)),
			notifications: granted
		)
		await controller.load()
		await controller.setAlerts(true)
		XCTAssertNil(controller.alertsRow)

		let denied = StubNotificationPermission(granted: false)
		let deniedController = makeQuotaSettingsController(
			transport: StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
				reading: true, interval: .fiveMinutes, alerts: true, thresholds: [75], resetNotice: false, statusline: false
			)),
			notifications: denied
		)
		await deniedController.load()
		XCTAssertNotNil(deniedController.alertsRow, "alerts already on with permission denied shows the row on load")
		let promptsOnLoad = await denied.requestCount
		XCTAssertEqual(promptsOnLoad, 0, "loading re-reads permission but never prompts")

		await deniedController.setAlerts(false)
		XCTAssertNil(deniedController.alertsRow)
		let promptsAfterOff = await denied.requestCount
		XCTAssertEqual(promptsAfterOff, 0)
	}
}

private actor RecordingNotificationPoster: NotificationPosting {
	struct Posted: Equatable {
		let identifier: String
		let title: String
		let body: String
	}

	private(set) var posted = [Posted]()
	private let failing: Set<String>

	init(failing: Set<String> = []) {
		self.failing = failing
	}

	func post(identifier: String, title: String, body: String) async throws {
		if failing.contains(identifier) {
			throw CocoaError(.featureUnsupported)
		}
		posted.append(Posted(identifier: identifier, title: title, body: body))
	}
}

@MainActor
final class QuotaAlertNotifierTests: XCTestCase {
	private let threshold = DesktopQuotaAlertV1(id: "qa1.threshold", kind: .threshold, client: "codex", windowMinutes: 300, usedPercent: 80.4, threshold: 75)
	private let reset = DesktopQuotaAlertV1(id: "qa1.reset", kind: .reset, client: "claude", label: "Opus", windowMinutes: 10080, usedPercent: 3)

	func testWithoutPermissionNothingIsPostedAndNothingIsReturnedForAcknowledgement() async {
		let poster = RecordingNotificationPoster()
		let notifier = QuotaAlertNotifier(permission: StubNotificationPermission(granted: false), poster: poster)

		let delivered = await notifier.deliver([threshold, reset])

		XCTAssertEqual(delivered, [])
		let posted = await poster.posted
		XCTAssertTrue(posted.isEmpty)
	}

	func testOnlyAcceptedPostsAreReturnedAndEachUsesTheAlertIDAsItsIdentifier() async {
		let poster = RecordingNotificationPoster(failing: ["qa1.reset"])
		let notifier = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster)

		let delivered = await notifier.deliver([threshold, reset])

		XCTAssertEqual(delivered, ["qa1.threshold"])
		let posted = await poster.posted
		XCTAssertEqual(posted.map(\.identifier), ["qa1.threshold"])
	}

	func testContentNamesTheClientWindowAndFigureInTheActiveLanguage() {
		let thresholdContent = QuotaAlertContent(threshold)
		XCTAssertEqual(thresholdContent.title, t(DesktopCopy.notificationQuotaTitle, "Codex"))
		XCTAssertEqual(
			thresholdContent.body,
			t(DesktopCopy.notificationQuotaThresholdBody, t(DesktopCopy.quotaWindow5h), Int64(80), Int64(75))
		)
		XCTAssertTrue(thresholdContent.body.contains("80") && thresholdContent.body.contains("75"))

		let resetContent = QuotaAlertContent(reset)
		XCTAssertEqual(resetContent.title, t(DesktopCopy.notificationQuotaTitle, "Claude"))
		XCTAssertEqual(resetContent.body, t(DesktopCopy.notificationQuotaResetBody, "Opus", Int64(3)))

		let unnamed = QuotaAlertContent(DesktopQuotaAlertV1(id: "qa1.x", kind: .reset, client: "codex", usedPercent: 1))
		XCTAssertTrue(unnamed.body.contains(t(DesktopCopy.notificationQuotaWindowFallback)))
	}
}

@MainActor
final class SettingsWindowLayoutTests: XCTestCase {
	/// MA-F3: the denied-permission warning and its action appear after the
	/// window was sized. The window must grow to hold them instead of letting
	/// SwiftUI overlap the warning, the action, and the threshold control.
	func testWindowGrowsWhenTheNotificationWarningAppears() async throws {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let transport = StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
			reading: true, interval: .fiveMinutes, alerts: false, thresholds: [75, 90], resetNotice: false, statusline: false
		))
		let quotaSettings = makeQuotaSettingsController(
			preferences: preferences, transport: transport, notifications: StubNotificationPermission(granted: false)
		)
		let controller = SettingsWindowController(preferences: preferences, quotaSettings: quotaSettings)
		controller.show()
		let window = try XCTUnwrap(controller.window)
		defer { window.close() }
		await quotaSettings.load()
		settle(window)
		let heightWithoutWarning = window.contentLayoutRect.height

		await quotaSettings.setAlerts(true)
		XCTAssertNotNil(quotaSettings.alertsRow)
		settle(window)

		let hosting = try XCTUnwrap(window.contentViewController?.view)
		XCTAssertGreaterThan(window.contentLayoutRect.height, heightWithoutWarning, "the window did not grow for the warning row")
		XCTAssertGreaterThanOrEqual(
			window.contentLayoutRect.height + 0.5, hosting.fittingSize.height,
			"content taller than the window is compressed and overlaps"
		)
	}

	private func settle(_ window: NSWindow) {
		for _ in 0..<10 {
			window.contentView?.layoutSubtreeIfNeeded()
			RunLoop.main.run(until: Date().addingTimeInterval(0.05))
		}
	}
}

