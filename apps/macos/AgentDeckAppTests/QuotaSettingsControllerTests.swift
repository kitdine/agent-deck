import AppKit
import XCTest
@testable import AgentDeck
@_spi(DesktopApp) @testable import AgentDeckShared

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

	// Codex PR #5 tenth review, P2: neither core settings nor this
	// controller's local preference mirror refresh the desktop snapshot or
	// App Group projection on their own. Since periodic refresh is
	// independently opt-in and off by default, turning reading off (or back
	// on) used to leave the menu bar/widgets stuck on stale or reading-off
	// figures until an unrelated refresh happened to run.
	func testSetReadingTriggersASnapshotRefreshAfterAConfirmedChange() async {
		let refreshCalls = ActorCounter()
		let controller = makeQuotaSettingsController(refreshQuotaSnapshot: { await refreshCalls.increment() })
		await controller.load()

		await controller.setReading(true)

		let count = await refreshCalls.count
		XCTAssertEqual(count, 1, "a confirmed reading change must trigger exactly one snapshot refresh")
	}

	func testNextQuotaWriteRemainsQueuedWhileRefreshAwaitsPublication() async {
		let refreshStarted = expectation(description: "confirmed reading write requests publication")
		var finishRefresh: CheckedContinuation<Void, Never>?
		let transport = StubQuotaSettingsTransport()
		let controller = makeQuotaSettingsController(transport: transport, refreshQuotaSnapshot: {
			await withCheckedContinuation { continuation in
				finishRefresh = continuation
				refreshStarted.fulfill()
			}
		})
		await controller.load()

		let reading = Task { await controller.setReading(true) }
		let started = await XCTWaiter.fulfillment(of: [refreshStarted], timeout: 3)
		XCTAssertEqual(started, .completed)
		guard let finishRefresh else {
			reading.cancel()
			return
		}
		await controller.setInterval(.fifteenMinutes)
		let callsBeforePublication = await transport.applyCalls
		XCTAssertEqual(callsBeforePublication.count, 1, "the publisher still owns the write sequence")
		finishRefresh.resume()
		await reading.value
		let calls = await transport.applyCalls
		XCTAssertEqual(calls.count, 2)
		XCTAssertEqual(calls.last?.interval, .fifteenMinutes)
	}

	func testReadingWritesDuringAScanPublishTheLastCommittedSetting() async {
		let transport = StubQuotaSettingsTransport()
		let firstScan = expectation(description: "first scan captured reading on")
		let host = QuotaSettingsRefreshHost(transport: transport, firstScan: firstScan)
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let controller = makeQuotaSettingsController(transport: transport, refreshQuotaSnapshot: {
			await coordinator.requestSettingsRefresh(requiresFollowUp: false)
		})
		await controller.load()

		let enable = Task { await controller.setReading(true) }
		let firstResult = await XCTWaiter.fulfillment(of: [firstScan], timeout: 3)
		XCTAssertEqual(firstResult, .completed)
		guard firstResult == .completed else {
			coordinator.stopAppWork()
			host.resumeFirstScan()
			await enable.value
			return
		}
		await controller.setReading(false)
		let pendingWrites = await transport.applyCalls
		XCTAssertEqual(pendingWrites.map(\.reading), [true], "the second write waits for the first publication")
		XCTAssertEqual(host.readingAtStart, [true], "the first scan owns only the first write")
		XCTAssertNil(coordinator.latestSnapshot)

		host.resumeFirstScan()
		await enable.value
		let writes = await transport.applyCalls
		XCTAssertEqual(writes.map(\.reading), [true, false])
		XCTAssertEqual(host.readingAtStart, [true, false], "each round reads its committed write")
		XCTAssertEqual(coordinator.latestSnapshot?.data.subscription.reading, false)
	}

	func testSetReadingDoesNotTriggerASnapshotRefreshWhenTheWriteFails() async {
		let transport = AlwaysUndecodableQuotaSettingsTransport()
		let refreshCalls = ActorCounter()
		let controller = makeQuotaSettingsController(transport: transport, refreshQuotaSnapshot: { await refreshCalls.increment() })
		await controller.load()

		await controller.setReading(true)

		let count = await refreshCalls.count
		XCTAssertEqual(count, 0, "a failed write has nothing new to reflect and must not trigger a refresh")
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

	func testClientSelectionPersistsAndTheLastClientCannotBeDeselected() async {
		let transport = StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
			reading: true, clients: ["codex", "claude"], interval: .fiveMinutes,
			alerts: false, thresholds: [75, 90], resetNotice: true, statusline: false
		))
		let controller = makeQuotaSettingsController(transport: transport)
		await controller.load()

		await controller.setClient("claude", enabled: false)
		XCTAssertEqual(controller.settings?.clients, ["codex"])
		let afterClaude = await transport.applyCalls
		XCTAssertEqual(afterClaude.last?.clients, ["codex"])

		await controller.setClient("codex", enabled: false)
		XCTAssertEqual(controller.settings?.clients, ["codex"])
		let afterLastClient = await transport.applyCalls
		XCTAssertEqual(afterLastClient.count, afterClaude.count, "the last selected client must remain remembered")
	}

	/// requirements.md's default-off contract: a control moved before `load()`
	/// returns must not invent an on value for alerts/thresholds/reset-notice —
	/// it sends the quiet defaults alongside the one field the user actually
	/// touched.
	// Codex PR #5 second/third review, P2: alerts default off (requirements.md),
	// but thresholds and resetNotice must default to internal/quota/settings.go's
	// own DefaultSettings() values, not the empty/false a naive "off" reading
	// would suggest -- [] fails ParseAlertThresholds outright, and
	// DefaultSettings() itself sets ResetNotice: true.
	func testAChangeBeforeLoadCompletesSendsCoresOwnDefaultsAlongsideTheOneFieldTouched() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let transport = StubQuotaSettingsTransport()
		let controller = makeQuotaSettingsController(preferences: preferences, transport: transport)

		await controller.setReading(true)

		let calls = await transport.applyCalls
		XCTAssertEqual(calls.count, 1)
		XCTAssertEqual(calls[0].reading, true)
		XCTAssertFalse(calls[0].alerts)
		XCTAssertEqual(calls[0].thresholds, [75, 90], "must be a value ParseAlertThresholds accepts, not empty")
		XCTAssertTrue(calls[0].resetNotice, "DefaultSettings() itself sets ResetNotice true")
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

	func testInitialLoadCannotOverwriteAWriteThatCompletedWhileItWasInFlight() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		preferences.quotaProbeEnabled = true
		let stale = DesktopQuotaSettingsValuesV1(
			reading: true, interval: .fiveMinutes, alerts: false,
			thresholds: [75, 90], resetNotice: true, statusline: false
		)
		let transport = SupersededLoadQuotaSettingsTransport(staleLoad: stale)
		let controller = makeQuotaSettingsController(preferences: preferences, transport: transport)

		let load = Task { await controller.load() }
		await transport.waitForLoad()
		await controller.setAlerts(true)
		await transport.completeLoad()
		await load.value

		XCTAssertEqual(controller.settings?.alerts, true, "the stale initial load must not replace the confirmed write")
		await controller.setResetNotice(false)
		let calls = await transport.applyCalls
		XCTAssertEqual(calls.last?.alerts, true, "the next write must not resend the stale load's alerts-off value")
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

	// Codex PR #5 second review, P2: once AgentDeck's own route is registered,
	// the file's "current" command is AgentDeck's, not the prior one it will
	// chain to -- previewing it back would falsely claim AgentDeck chains
	// itself.
	func testChainedStatusLineCommandPreviewOmitsAgentDecksOwnRegisteredCommand() async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let settingsURL = directory.appendingPathComponent("settings.json")
		try #"{"statusLine":{"type":"command","command":"agentdeck --state-dir /tmp/state quota capture"}}"#
			.write(to: settingsURL, atomically: true, encoding: .utf8)

		let controller = makeQuotaSettingsController(claudeSettingsURL: settingsURL)
		await controller.load()

		XCTAssertNil(controller.chainedStatusLineCommand)
	}

	func testChainedStatusLineCommandPreviewOmitsTheEmbeddedHelpersAbsoluteRoute() async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let settingsURL = directory.appendingPathComponent("settings.json")
		try #"{"statusLine":{"type":"command","command":"'/Applications/AgentDeck.app/Contents/Helpers/agentdeck' --state-dir '/tmp/state dir' quota capture"}}"#
			.write(to: settingsURL, atomically: true, encoding: .utf8)

		let controller = makeQuotaSettingsController(claudeSettingsURL: settingsURL)
		await controller.load()

		XCTAssertNil(controller.chainedStatusLineCommand)
	}

	// Codex PR #5 seventh review, P2: disabling capture restores the prior
	// command into ~/.claude/settings.json, but this preview was last read
	// by load() and never reread the file -- an immediate re-enable then
	// offered consent while still claiming no command would be chained.
	func testStatuslineConsentRefreshesTheChainedCommandPreviewAfterAWrite() async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let settingsURL = directory.appendingPathComponent("settings.json")
		try #"{"statusLine":{"type":"command","command":"agentdeck quota capture"}}"#
			.write(to: settingsURL, atomically: true, encoding: .utf8)

		let controller = makeQuotaSettingsController(claudeSettingsURL: settingsURL)
		await controller.load()
		XCTAssertNil(controller.chainedStatusLineCommand, "AgentDeck's own route must not preview as something it would chain to")

		// Simulate the disable's own restore: the file now has the prior
		// command back on disk, written by something other than this
		// controller's own in-memory state.
		try #"{"statusLine":{"type":"command","command":"python3 ~/.claude/statusline.py"}}"#
			.write(to: settingsURL, atomically: true, encoding: .utf8)
		await controller.setStatuslineConsent(false)

		XCTAssertEqual(controller.chainedStatusLineCommand, "python3 ~/.claude/statusline.py", "must reread the file after the write completes instead of keeping the stale load()-time preview")
	}

	// Codex PR #5 eighth review, P2: this control is interactive as soon as
	// the persisted reading preference mirror says reading is on, before
	// load() has necessarily supplied `settings`. A confirmed statusline
	// write arriving in that window must not be silently dropped.
	func testStatuslineConsentSeedsSettingsWhenTheInitialLoadHasNotResolvedYet() async {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		preferences.quotaProbeEnabled = true
		let controller = makeQuotaSettingsController(preferences: preferences)
		XCTAssertNil(controller.settings, "precondition: load() was deliberately never awaited")

		await controller.setStatuslineConsent(true)

		XCTAssertEqual(controller.settings?.statusline, true, "a confirmed statusline write must not be dropped just because load() has not resolved yet")
		XCTAssertEqual(controller.settings?.reading, true, "seeded from the persisted reading preference mirror")
	}

	// Codex PR #5 tenth review, P2 (refinement of the seventh review's P2
	// above): when the statusline toggle is queued behind an in-flight
	// settings write, drainPendingWrites() returns immediately (the guard
	// sees isApplyingWrite already true), so a reread tied to that immediate
	// return happens before the queued write is ever processed. The preview
	// must instead reflect what is on disk once the queued write actually
	// completes, not whatever was there when it was merely queued.
	func testChainedStatusLineCommandPreviewRefreshesOnlyAfterAQueuedStatuslineWriteActuallyCompletes() async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let settingsURL = directory.appendingPathComponent("settings.json")
		try #"{"statusLine":{"type":"command","command":"agentdeck quota capture"}}"#
			.write(to: settingsURL, atomically: true, encoding: .utf8)

		let transport = SuspendingBothQuotaSettingsTransport()
		let controller = makeQuotaSettingsController(transport: transport, claudeSettingsURL: settingsURL)
		await controller.load()
		XCTAssertNil(controller.chainedStatusLineCommand)

		let settingsWrite = Task { await controller.setAlerts(true) }
		await transport.waitForApplyCount(1)

		let statuslineWrite = Task { await controller.setStatuslineConsent(false) }
		for _ in 0 ..< 50 { await Task.yield() }

		// The file changes only now, after the statusline call has already
		// queued (and, under the old code, already performed its premature
		// reread) but before either write in this test resolves.
		try #"{"statusLine":{"type":"command","command":"python3 ~/.claude/statusline.py"}}"#
			.write(to: settingsURL, atomically: true, encoding: .utf8)

		await transport.completeNextSettings(.success)
		await transport.waitForStatuslineCount(1)
		await transport.completeNextStatusline(.success)
		await settingsWrite.value
		await statuslineWrite.value

		XCTAssertEqual(
			controller.chainedStatusLineCommand, "python3 ~/.claude/statusline.py",
			"must reread after the queued write actually completed, not at the moment it was merely queued"
		)
	}

	// Codex PR #5 twelfth review, P2: currentDesired() falls back to product
	// defaults for alerts/thresholds/resetNotice only while settings is nil
	// -- the reading and interval controls this gates must stay disabled
	// until the initial load resolves, so a change can never resend those
	// defaults over an existing installation's real values.
	func testReadingControlsStayDisabledUntilTheInitialLoadResolves() async {
		let controller = makeQuotaSettingsController(transport: StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
			reading: false, interval: .fiveMinutes, alerts: true, thresholds: [90], resetNotice: false, statusline: false
		)))
		XCTAssertFalse(controller.readingControlsEnabled, "must be disabled before load() resolves")

		await controller.load()

		XCTAssertTrue(controller.readingControlsEnabled, "must be enabled once the initial read has resolved")
	}

	func testResetNoticeControlRequiresBothReadingAndAlerts() async {
		for (reading, alerts, expected) in [(false, true, false), (true, false, false), (true, true, true)] {
			let controller = makeQuotaSettingsController(transport: StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
				reading: reading, interval: .fiveMinutes, alerts: alerts,
				thresholds: [75, 90], resetNotice: true, statusline: false
			)))
			await controller.load()
			XCTAssertEqual(controller.resetNoticeControlEnabled, expected, "reading=\(reading) alerts=\(alerts)")
		}
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

	// Codex PR #5 third review, P1: quota-settings and quota-statusline both
	// do a full read-modify-write of the one settings row core state keeps,
	// so this controller must never have one of each write in flight at the
	// same time -- the slower one's stale read would silently overwrite the
	// other's already-completed write. A statusline change requested while a
	// settings write is in flight must queue, not start its own transport
	// call until the settings write has actually resolved.
	func testStatuslineAndSettingsWritesNeverOverlap() async {
		let transport = SuspendingBothQuotaSettingsTransport()
		let controller = makeQuotaSettingsController(transport: transport)
		await controller.load()

		let settingsWrite = Task { await controller.setAlerts(true) }
		await transport.waitForApplyCount(1)

		// Not awaited: a buggy implementation would start its own transport
		// call immediately and suspend there, so awaiting .value here before
		// completing anything would deadlock the test rather than surface a
		// clean assertion failure.
		let statuslineWrite = Task { await controller.setStatuslineConsent(true) }
		for _ in 0 ..< 50 { await Task.yield() }
		let statuslineCallsWhileSettingsInFlight = await transport.statuslineCalls
		XCTAssertEqual(statuslineCallsWhileSettingsInFlight, 0, "the statusline write must not start until the in-flight settings write resolves")

		await transport.completeNextSettings(.success)
		await transport.waitForStatuslineCount(1)
		await transport.completeNextStatusline(.success)
		await settingsWrite.value
		await statuslineWrite.value

		XCTAssertEqual(controller.settings?.alerts, true)
		XCTAssertEqual(controller.settings?.statusline, true)
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

	func testQueuedClientChangeRefreshesOnlyAfterItsOwnWritePersists() async {
		let transport = SuspendingQuotaSettingsTransport()
		let refreshCalls = ActorCounter()
		let controller = makeQuotaSettingsController(
			transport: transport,
			refreshQuotaSnapshot: { await refreshCalls.increment() }
		)
		await controller.load()

		let first = Task { await controller.setInterval(.fifteenMinutes) }
		await transport.waitForApplyCount(1)
		let client = Task { await controller.setClient("claude", enabled: false) }
		await Task.yield()
		let beforeFirstCompletion = await refreshCalls.count
		XCTAssertEqual(beforeFirstCompletion, 0, "a queued client change must not refresh old settings")

		await transport.completeNext(.success)
		await transport.waitForApplyCount(2)
		let beforeClientCompletion = await refreshCalls.count
		XCTAssertEqual(beforeClientCompletion, 0, "the first write is not the queued client change")
		await transport.completeNext(.success)
		await first.value
		await client.value

		XCTAssertEqual(controller.settings?.clients, ["codex"])
		let finalRefreshCount = await refreshCalls.count
		XCTAssertEqual(finalRefreshCount, 1)
	}

	func testQueuedReadingChangeRefreshesOnlyAfterItsOwnWritePersists() async {
		let transport = SuspendingQuotaSettingsTransport()
		let refreshCalls = ActorCounter()
		let controller = makeQuotaSettingsController(
			transport: transport,
			refreshQuotaSnapshot: { await refreshCalls.increment() }
		)
		await controller.load()

		let first = Task { await controller.setInterval(.fifteenMinutes) }
		await transport.waitForApplyCount(1)
		let reading = Task { await controller.setReading(true) }
		await Task.yield()
		let beforeFirstCompletion = await refreshCalls.count
		XCTAssertEqual(beforeFirstCompletion, 0, "a queued reading change must not refresh old settings")

		await transport.completeNext(.success)
		await transport.waitForApplyCount(2)
		let beforeReadingCompletion = await refreshCalls.count
		XCTAssertEqual(beforeReadingCompletion, 0, "the first write is not the queued reading change")
		await transport.completeNext(.success)
		await first.value
		await reading.value

		XCTAssertEqual(controller.settings?.reading, true)
		let finalRefreshCount = await refreshCalls.count
		XCTAssertEqual(finalRefreshCount, 1)
	}
}

@MainActor
private final class QuotaSettingsRefreshHost: DesktopSnapshotRefreshing {
	private let transport: StubQuotaSettingsTransport
	private let firstScan: XCTestExpectation
	private var resumeScan: CheckedContinuation<Void, Never>?
	private(set) var readingAtStart: [Bool] = []

	init(transport: StubQuotaSettingsTransport, firstScan: XCTestExpectation) {
		self.transport = transport
		self.firstScan = firstScan
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		guard case let .decoded(settings) = await transport.loadQuotaSettings() else {
			throw HelperExecutionError.malformedOutput
		}
		let reading = settings.settings.reading
		readingAtStart.append(reading)
		if readingAtStart.count == 1 {
			await withCheckedContinuation { continuation in
				resumeScan = continuation
				firstScan.fulfill()
			}
		}
		return WireFixture.envelope().replacingSubscription(
			DesktopSubscriptionSnapshotV1(available: true, reading: reading, clients: [])
		)
	}

	func resumeFirstScan() {
		resumeScan?.resume()
		resumeScan = nil
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
			reading: desired.reading, clients: desired.clients, interval: desired.interval, alerts: desired.alerts,
			thresholds: desired.thresholds, resetNotice: desired.resetNotice, statusline: settings.statusline
		)
		continuation.resume(returning: .decoded(DesktopQuotaSettingsResultV1(settings: settings, statuslineRestore: nil)))
	}
}

/// Suspends both `applyQuotaSettings` and `setQuotaStatusLine` independently,
/// so a test can prove the two never run concurrently through this
/// controller.
private actor SuspendingBothQuotaSettingsTransport: QuotaSettingsTransport {
	enum Completion { case success, failure }
	private var settings = StubQuotaSettingsTransport.defaultSettings
	private var pendingSettings = [(DesktopQuotaSettingsDesiredV1, CheckedContinuation<DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1>, Never>)]()
	private var pendingStatusline = [CheckedContinuation<DesktopQuotaTransportOutcome<DesktopQuotaStatusLineResultV1>, Never>]()
	private(set) var applyCalls = [DesktopQuotaSettingsDesiredV1]()
	private(set) var statuslineCalls = 0

	func loadQuotaSettings() async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		.decoded(DesktopQuotaSettingsResultV1(settings: settings, statuslineRestore: nil))
	}

	func applyQuotaSettings(_ desired: DesktopQuotaSettingsDesiredV1) async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		applyCalls.append(desired)
		return await withCheckedContinuation { pendingSettings.append((desired, $0)) }
	}

	func setQuotaStatusLine(enabled _: Bool) async -> DesktopQuotaTransportOutcome<DesktopQuotaStatusLineResultV1> {
		statuslineCalls += 1
		return await withCheckedContinuation { pendingStatusline.append($0) }
	}

	func waitForApplyCount(_ count: Int) async {
		while applyCalls.count < count { await Task.yield() }
	}

	func waitForStatuslineCount(_ count: Int) async {
		while statuslineCalls < count { await Task.yield() }
	}

	func completeNextSettings(_ completion: Completion) {
		let (desired, continuation) = pendingSettings.removeFirst()
		guard case .success = completion else {
			continuation.resume(returning: .undecodable)
			return
		}
		settings = DesktopQuotaSettingsValuesV1(
			reading: desired.reading, clients: desired.clients, interval: desired.interval, alerts: desired.alerts,
			thresholds: desired.thresholds, resetNotice: desired.resetNotice, statusline: settings.statusline
		)
		continuation.resume(returning: .decoded(DesktopQuotaSettingsResultV1(settings: settings, statuslineRestore: nil)))
	}

	func completeNextStatusline(_ completion: Completion) {
		let continuation = pendingStatusline.removeFirst()
		guard case .success = completion else {
			continuation.resume(returning: .undecodable)
			return
		}
		settings = DesktopQuotaSettingsValuesV1(
			reading: settings.reading, clients: settings.clients, interval: settings.interval, alerts: settings.alerts,
			thresholds: settings.thresholds, resetNotice: settings.resetNotice, statusline: true
		)
		continuation.resume(returning: .decoded(DesktopQuotaStatusLineResultV1(consent: true, result: DesktopUsageHookResultV1(outcome: .configured))))
	}
}

private actor SupersededLoadQuotaSettingsTransport: QuotaSettingsTransport {
	private let staleLoad: DesktopQuotaSettingsValuesV1
	private var current: DesktopQuotaSettingsValuesV1
	private var loadContinuation: CheckedContinuation<DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1>, Never>?
	private(set) var applyCalls = [DesktopQuotaSettingsDesiredV1]()

	init(staleLoad: DesktopQuotaSettingsValuesV1) {
		self.staleLoad = staleLoad
		current = staleLoad
	}

	func loadQuotaSettings() async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		await withCheckedContinuation { loadContinuation = $0 }
	}

	func applyQuotaSettings(_ desired: DesktopQuotaSettingsDesiredV1) async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		applyCalls.append(desired)
		current = DesktopQuotaSettingsValuesV1(
			reading: desired.reading, clients: desired.clients, interval: desired.interval, alerts: desired.alerts,
			thresholds: desired.thresholds, resetNotice: desired.resetNotice, statusline: current.statusline
		)
		return .decoded(DesktopQuotaSettingsResultV1(settings: current, statuslineRestore: nil))
	}

	func setQuotaStatusLine(enabled: Bool) async -> DesktopQuotaTransportOutcome<DesktopQuotaStatusLineResultV1> {
		current = DesktopQuotaSettingsValuesV1(
			reading: current.reading, interval: current.interval, alerts: current.alerts,
			thresholds: current.thresholds, resetNotice: current.resetNotice, statusline: enabled
		)
		return .decoded(DesktopQuotaStatusLineResultV1(consent: enabled, result: DesktopUsageHookResultV1(outcome: .configured)))
	}

	func waitForLoad() async {
		while loadContinuation == nil { await Task.yield() }
	}

	func completeLoad() {
		loadContinuation?.resume(returning: .decoded(DesktopQuotaSettingsResultV1(settings: staleLoad, statuslineRestore: nil)))
		loadContinuation = nil
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

	// Codex PR #5 second review, P2: turning alerts on then off again while the
	// first write is still in flight coalesces into one queued round trip
	// (QuotaSettingsController.applySettings). The "on" call's own stale
	// intent must not still show the system permission prompt once the
	// coalesced write has actually persisted alerts off.
	func testSetAlertsRechecksTheFinalPersistedStateBeforeRequestingPermission() async {
		let transport = SuspendingQuotaSettingsTransport()
		let permission = StubNotificationPermission(granted: true)
		let controller = makeQuotaSettingsController(transport: transport, notifications: permission)
		await controller.load()

		let turnOn = Task { await controller.setAlerts(true) }
		await transport.waitForApplyCount(1)
		let turnOff = Task { await controller.setAlerts(false) }
		await turnOff.value

		await transport.completeNext(.success)
		await transport.waitForApplyCount(2)
		await transport.completeNext(.success)
		await turnOn.value

		let requests = await permission.requestCount
		XCTAssertEqual(requests, 0, "alerts were already persisted off by the coalesced write; turnOn's stale intent must not still prompt")
		XCTAssertEqual(controller.settings?.alerts, false)
	}

	// Codex PR #5 third review, P2: settings?.alerts alone is not "persisted"
	// -- stage() sets it optimistically before the write, and a failed or
	// undecodable response never rolls it back. Gating solely on that value
	// still shows the permission prompt after a write core state never
	// actually accepted.
	func testFailedAlertWriteDoesNotPromptOnTheStaleOptimisticValue() async {
		let permission = StubNotificationPermission(granted: true)
		let controller = makeQuotaSettingsController(transport: AlwaysUndecodableQuotaSettingsTransport(), notifications: permission)

		await controller.setAlerts(true)

		XCTAssertEqual(controller.settingsRow?.severity, .error)
		XCTAssertEqual(controller.settings?.alerts, true, "stage() still optimistically shows the toggle on")
		let requests = await permission.requestCount
		XCTAssertEqual(requests, 0, "the write was never confirmed; must not still prompt on the unconfirmed optimistic value")
	}
}

private final class RecordingNotificationPoster: NotificationPosting, @unchecked Sendable {
	struct Posted: Equatable {
		let identifier: String
		let title: String
		let body: String
	}

	private let lock = NSLock()
	private var records = [Posted]()
	var posted: [Posted] { lock.withLock { records } }
	private let failing: Set<String>
	let observe: (@Sendable (String, @escaping @Sendable (Bool) -> Void) -> Void)?

	init(failing: Set<String> = [], observe: (@Sendable (String, @escaping @Sendable (Bool) -> Void) -> Void)? = nil) {
		self.failing = failing
		self.observe = observe
	}

	func post(identifier: String, title: String, body: String, completion: @escaping @Sendable (Bool) -> Void) {
		if failing.contains(identifier) {
			completion(false)
			return
		}
		lock.withLock { records.append(Posted(identifier: identifier, title: title, body: body)) }
		if let observe { observe(identifier, completion) } else { completion(true) }
	}
}

// Synthetic helper fixture. Production App code only opens and validates.
private func createQuotaGuardFixture(_ root: URL) throws -> Int32 {
	let dir = open(root.path, O_RDONLY | O_DIRECTORY | O_CLOEXEC)
	guard dir >= 0 else { throw POSIXError(.EIO) }
	defer { close(dir) }
	let fd = openat(dir, "quota-delivery.lock", O_CREAT | O_EXCL | O_RDWR | O_NOFOLLOW | O_CLOEXEC, 0o600)
	guard fd >= 0 else { throw POSIXError(.EIO) }
	var info = stat()
	guard fstat(fd, &info) == 0 else { close(fd); throw POSIXError(.EIO) }
	let binding = [UInt64(bitPattern: Int64(info.st_dev)).bigEndian, UInt64(info.st_ino).bigEndian]
	let rc = binding.withUnsafeBytes { fsetxattr(dir, "user.agentdeck.quota-delivery-lock", $0.baseAddress, $0.count, 0, XATTR_CREATE) }
	guard rc == 0 else { close(fd); throw POSIXError(.EIO) }
	return fd
}

// Access times may change on a read. Compare the directory, every named
// inode and the exact binding across both successful and refused delivery.
private func quotaGuardRootSnapshot(_ root: URL) throws -> [String] {
	let dir = open(root.path, O_RDONLY | O_DIRECTORY | O_CLOEXEC)
	guard dir >= 0 else { throw POSIXError(.EIO) }
	defer { close(dir) }
	let names = try FileManager.default.contentsOfDirectory(atPath: root.path).sorted()
	var result = [String]()
	for name in ["."] + names {
		var st = stat()
		guard fstatat(dir, name, &st, AT_SYMLINK_NOFOLLOW) == 0 else { throw POSIXError(.EIO) }
		result.append("\(name) \(st.st_dev) \(st.st_ino) \(st.st_uid) \(st.st_mode) \(st.st_nlink) \(st.st_size) \(st.st_mtimespec.tv_sec) \(st.st_mtimespec.tv_nsec) \(st.st_ctimespec.tv_sec) \(st.st_ctimespec.tv_nsec)")
	}
	var bytes = [UInt8](repeating: 0, count: 32)
	let size = bytes.withUnsafeMutableBytes { fgetxattr(dir, "user.agentdeck.quota-delivery-lock", $0.baseAddress, $0.count, 0, 0) }
	if size >= 0 { result.append(Data(bytes.prefix(size)).base64EncodedString()) }
	else if errno == ENOATTR { result.append("absent") }
	else { throw POSIXError(.EIO) }
	return result
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
		let posted = poster.posted
		XCTAssertTrue(posted.isEmpty)
	}

	func testOnlyAcceptedPostsAreReturnedAndEachUsesTheAlertIDAsItsIdentifier() async {
		let poster = RecordingNotificationPoster(failing: ["qa1.reset"])
		let notifier = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster)
		let legacyClient = DesktopQuotaAlertV1(id: "qa1.legacy", kind: .reset, client: "legacy", usedPercent: 3)

		let delivered = await notifier.deliver([threshold, reset, legacyClient])

		XCTAssertEqual(delivered, ["qa1.threshold", "qa1.legacy"])
		let posted = poster.posted
		XCTAssertEqual(posted.map(\.identifier), ["qa1.threshold", "qa1.legacy"])
	}

	func testGuardedSubmissionAndCallbackBoundary() async throws {
		let fm = FileManager.default
		let root = fm.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		try fm.createDirectory(at: root, withIntermediateDirectories: false, attributes: [.posixPermissions: 0o700])
		defer { try? fm.removeItem(at: root) }
		var info = stat()
		XCTAssertEqual(lstat(root.path, &info), 0)
		let identity = "\(UInt64(bitPattern: Int64(info.st_dev))):\(info.st_ino)"
		let image = root.appendingPathComponent("quota-delivery.epoch")
		let bytes = Data("AGENTDECK-QDG-1\n".utf8) + Data([0,0,0,0,0,0,1,2])
		func writeImage() throws {
			try bytes.write(to: image)
			try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: image.path)
		}
		func candidate(_ id: String, _ generation: Int64? = 258, _ proof: String? = nil) -> DesktopQuotaAlertV1 {
			.init(id: id, kind: .reset, client: "claude", usedPercent: 0, generation: generation, stateRootID: proof ?? identity)
		}
		XCTAssertEqual(close(try createQuotaGuardFixture(root)), 0)
		let fd = open(root.appendingPathComponent("quota-delivery.lock").path, O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)
		XCTAssertGreaterThanOrEqual(fd, 0)
		let rootFD = open(root.path, O_RDONLY | O_DIRECTORY | O_CLOEXEC)
		XCTAssertGreaterThanOrEqual(rootFD, 0)
		defer { close(rootFD) }
		XCTAssertEqual(flock(rootFD, LOCK_EX | LOCK_NB), 0)
		defer { close(fd) }
		let (callbacks, received) = AsyncStream<@Sendable (Bool) -> Void>.makeStream()
		defer { received.finish() }
		let poster = RecordingNotificationPoster { id, completion in
			XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), -1, "submission outside guard file lock")
			XCTAssertEqual(errno, EWOULDBLOCK)
			switch id {
			case "ok": completion(true)
			case "error": completion(false)
			default: received.yield(completion)
			}
		}
		let notifier = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster,
		                                  stateRoot: { root }, lockWait: .milliseconds(100), callbackWait: .seconds(2))
		let ok = candidate("ok")
		let denied = QuotaAlertNotifier(permission: StubNotificationPermission(granted: false), poster: poster,
		                               stateRoot: { XCTFail("resolved root before permission"); return root })
		let deniedIDs = await denied.deliver([ok])
		XCTAssertTrue(deniedIDs.isEmpty)
		func expectSkipped(_ alerts: [DesktopQuotaAlertV1]) async {
			let ids = await notifier.deliver(alerts)
			XCTAssertTrue(ids.isEmpty)
			XCTAssertTrue(poster.posted.isEmpty)
		}
		await expectSkipped([ok]) // absent
		try writeImage()
		try fm.setAttributes([.posixPermissions: 0o644], ofItemAtPath: image.path)
		await expectSkipped([ok])
		try Data(bytes.dropLast()).write(to: image)
		try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: image.path)
		await expectSkipped([ok])
		try writeImage()
		await expectSkipped([candidate("stale", 2), candidate("mismatch", 258, "1:2"), candidate("missing", nil), reset])
		XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
		await expectSkipped([ok])
		XCTAssertEqual(flock(fd, LOCK_UN), 0)
		let beforeDelivery = try quotaGuardRootSnapshot(root)
		let batch = [ok, ok, candidate("error"), candidate("timely"), candidate("late")]
		let delivery = Task { await notifier.deliver(batch) }
		var iterator = callbacks.makeAsyncIterator()
		// Independent watchdog also bounds this fixture if the expected post is missing.
		let watchdog = Task { try await Task.sleep(for: .seconds(2)); received.finish() }
		defer { watchdog.cancel() }
		let timely = await iterator.next()
		let late = await iterator.next()
		XCTAssertNotNil(late)
		// Acquisition proceeds before the missing callback has been supplied.
		let unlocked = await Task.detached { () -> Bool in
			let end = ContinuousClock.now.advanced(by: .seconds(1))
			while flock(fd, LOCK_EX | LOCK_NB) != 0 {
				if ContinuousClock.now >= end { return false }
				try? await Task.sleep(for: .milliseconds(1))
			}
			return flock(fd, LOCK_UN) == 0
		}.value
		XCTAssertTrue(unlocked)
		timely?(true)
		let ids = await delivery.value
		XCTAssertEqual(ids, ["ok", "timely"])
		XCTAssertEqual(try quotaGuardRootSnapshot(root), beforeDelivery)
		late?(true)
		XCTAssertEqual(poster.posted.map(\.identifier), ["ok", "error", "timely", "late"])

		let symlink = root.appendingPathExtension("link")
		try fm.createSymbolicLink(at: symlink, withDestinationURL: root)
		defer { try? fm.removeItem(at: symlink) }
		let linkPoster = RecordingNotificationPoster()
		let linkNotifier = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true),
		                                      poster: linkPoster, stateRoot: { symlink })
		let linkIDs = await linkNotifier.deliver([ok])
		XCTAssertEqual(linkIDs, ["ok"], "a link to the same physical state root should submit")

		let (contention, waiting) = AsyncStream<Void>.makeStream()
		defer { waiting.finish() }
		let pendingNotifier = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster,
		                                         stateRoot: { symlink }, lockWait: .seconds(4),
		                                         onLockContention: { waiting.yield(()); waiting.finish() })
		guard flock(fd, LOCK_EX | LOCK_NB) == 0 else {
			XCTFail("could not take the fixture lock")
			return
		}
		let pendingDelivery = Task { await pendingNotifier.deliver([ok]) }
		defer { pendingDelivery.cancel() }
		let contentionWatchdog = Task { try await Task.sleep(for: .seconds(2)); waiting.finish() }
		defer { contentionWatchdog.cancel() }
		var contentionIterator = contention.makeAsyncIterator()
		let didWait = await contentionIterator.next()
		XCTAssertNotNil(didWait, "notifier must wait on the old directory before it is replaced")
		let old = root.appendingPathExtension("old")
		try fm.moveItem(at: root, to: old)
		defer { try? fm.removeItem(at: old) }
		try fm.createDirectory(at: root, withIntermediateDirectories: false, attributes: [.posixPermissions: 0o700])
		try writeImage()
		let replacementFD = try createQuotaGuardFixture(root)
		defer { close(replacementFD) }
		XCTAssertEqual(flock(fd, LOCK_UN), 0)
		let replaced = await pendingDelivery.value
		XCTAssertTrue(replaced.isEmpty)
		XCTAssertEqual(poster.posted.count, 4)
	}

	func testGuardLossStopsRemainingClaudeSubmissions() async throws {
		let fm = FileManager.default
		let root = fm.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		try fm.createDirectory(at: root, withIntermediateDirectories: false, attributes: [.posixPermissions: 0o700])
		defer { try? fm.removeItem(at: root) }
		var info = stat()
		XCTAssertEqual(lstat(root.path, &info), 0)
		let identity = "\(UInt64(bitPattern: Int64(info.st_dev))):\(info.st_ino)"
		let fd = try createQuotaGuardFixture(root)
		defer { close(fd) }
		let image = root.appendingPathComponent("quota-delivery.epoch")
		try (Data("AGENTDECK-QDG-1\n".utf8) + Data(repeating: 0, count: 8)).write(to: image)
		try fm.setAttributes([.posixPermissions: 0o600], ofItemAtPath: image.path)
		let guardFile = root.appendingPathComponent("quota-delivery.lock")
		let alerts = ["first", "second"].map {
			DesktopQuotaAlertV1(id: $0, kind: .reset, client: "claude", usedPercent: 0, generation: 0, stateRootID: identity)
		}
		let control = RecordingNotificationPoster()
		let healthy = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: control, stateRoot: { root })
		let healthyIDs = await healthy.deliver(alerts)
		XCTAssertEqual(healthyIDs, ["first", "second"])

		let poster = RecordingNotificationPoster { identifier, completion in
			if identifier == "first" {
				do {
					try FileManager.default.moveItem(at: guardFile, to: root.appendingPathComponent("retained"))
					try Data().write(to: guardFile)
					try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: guardFile.path)
				} catch {
					XCTFail("could not replace the fixture guard: \(error)")
				}
			}
			completion(true)
		}
		let notifier = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster, stateRoot: { root })
		let delivered = await notifier.deliver(alerts)
		XCTAssertEqual(poster.posted.map(\.identifier), ["first"])
		XCTAssertEqual(delivered, ["first"])
	}

	private func quotaPeer(_ key: String) throws -> String {
		let environment = ProcessInfo.processInfo.environment
		if environment["AGENTDECK_TEST_REQUIRE_QUOTA_PEERS"] == "1" {
			return try XCTUnwrap(environment[key], "Missing \(key) from the macOS test runner")
		}
		guard let path = environment[key] else { throw XCTSkip("requires the isolated Go test binary") }
		return path
	}

	func testIndependentGoSwiftGuard() async throws {
		let binary = try quotaPeer("AGENTDECK_TEST_QUOTA_GUARD_PEER")
		let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		try FileManager.default.createDirectory(at: root, withIntermediateDirectories: false, attributes: [.posixPermissions: 0o700])
		defer { try? FileManager.default.removeItem(at: root) }
		try Data("synthetic delivery guard\n".utf8).write(to: root.appendingPathComponent("test-fixture"))
		let child = Process(), input = Pipe(), output = Pipe()
		child.executableURL = URL(fileURLWithPath: binary)
		child.arguments = ["-test.run=^TestQuotaDeliverySwiftPeer$", "-test.timeout=15s", "--", "sai-independent-guard-peer"]
		child.currentDirectoryURL = root
		child.standardInput = input
		child.standardOutput = output
		try child.run()
		defer {
			try? input.fileHandleForWriting.close()
			if child.isRunning { child.terminate() }
			child.waitUntilExit()
		}
		@Sendable func readLine() -> String {
			var bytes = Data()
			while let byte = try? output.fileHandleForReading.read(upToCount: 1), !byte.isEmpty {
				if byte[0] == 10 { break }
				bytes.append(byte)
			}
			return String(decoding: bytes, as: UTF8.self)
		}
		@Sendable func send(_ text: String) { input.fileHandleForWriting.write(Data((text + "\n").utf8)) }
		let ready = readLine()
		guard ready.hasPrefix("held ") else { XCTFail("peer failed before lock"); return }
		let id = String(ready.dropFirst(5))
		let alert = DesktopQuotaAlertV1(id: "old", kind: .reset, client: "claude", usedPercent: 0, generation: 0, stateRootID: id)
		let poster = RecordingNotificationPoster { _, completion in
			send("apply")
			XCTAssertEqual(readLine(), "busy", "Go entered invalidation while Swift submitted")
			completion(true)
		}
		let app = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster,
		                             stateRoot: { root }, lockWait: .milliseconds(50))
		let held = await app.deliver([alert])
		XCTAssertTrue(held.isEmpty)
		XCTAssertTrue(poster.posted.isEmpty)
		send("release")
		XCTAssertEqual(readLine(), "released")
		let accepted = await app.deliver([alert])
		XCTAssertEqual(accepted, ["old"])
		send("apply")
		XCTAssertEqual(readLine(), "committed")
		let stale = await app.deliver([alert])
		XCTAssertTrue(stale.isEmpty)
		XCTAssertEqual(poster.posted.map(\.identifier), ["old"])
		child.waitUntilExit()
		XCTAssertEqual(child.terminationStatus, 0)
	}

	func testMixedGuardWarningUsesOneCodexSubmissionAndAcknowledgement() async throws {
		let peer = try quotaPeer("AGENTDECK_TEST_QUOTA_COMMAND_PEER")
		let fm = FileManager.default
		let home = fm.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		let bundle = home.appendingPathComponent("AgentDeck.app")
		let helper = bundle.appendingPathComponent("Contents/Helpers/agentdeck")
		try fm.createDirectory(at: helper.deletingLastPathComponent(), withIntermediateDirectories: true)
		defer { try? fm.removeItem(at: home) }
		try Data("synthetic mixed guard\n".utf8).write(to: home.appendingPathComponent("test-fixture"))
		let script = #"""
		#!/bin/sh
		exec "$AGENTDECK_TEST_QUOTA_COMMAND_PEER" '-test.run=^TestDesktopQuotaMixedEnvelopePeer$' '-test.timeout=12s' -- sai-mixed-guard-peer "$@"
		"""#
		try Data(script.utf8).write(to: helper)
		XCTAssertEqual(chmod(helper.path, 0o700), 0)
		let runner = EmbeddedHelperRunner(appBundleURL: bundle, environment: [
			"HOME": home.path, "TMPDIR": home.path, "AGENTDECK_TEST_QUOTA_COMMAND_PEER": peer,
			"AGENTDECK_TEST_QUOTA_ENVELOPE_ROOT": home.path,
		])
		defer { runner.closeOwnershipSession() }
		let poster = RecordingNotificationPoster { _, done in done(true) }
		let notifier = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster,
		                                  stateRoot: { try await runner.resolveStateRoot() })
		let coordinator = DesktopRefreshCoordinator(host: DesktopHost(runner: runner), quotaRefresher: runner,
		                                            alertDeliverer: notifier, snapshotStore: nil)
		await coordinator.refreshQuotaAlertsOnly(manual: true)

		struct PartialRefresh: Decodable {
			var partial: Bool
			var warnings: [String]
			struct Payload: Decodable { var alerts: [DesktopQuotaAlertV1] }
			var data: Payload
		}
		let envelope = try JSONDecoder().decode(PartialRefresh.self, from: Data(contentsOf: home.appendingPathComponent("mixed-envelope.json")))
		XCTAssertTrue(envelope.partial)
		XCTAssertEqual(envelope.warnings, ["quota_alerts_failed"])
		XCTAssertEqual(envelope.data.alerts.map(\.client), ["codex"])
		let id = try XCTUnwrap(envelope.data.alerts.first?.id)
		XCTAssertEqual(poster.posted.map(\.identifier), [id])
		let ackIDs = try String(contentsOf: home.appendingPathComponent("ack-ids"), encoding: .utf8)
		XCTAssertEqual(ackIDs, id + "\n")
		struct Ack: Decodable { var data: DesktopQuotaAlertsAckResultV1 }
		let ack = try JSONDecoder().decode(Ack.self, from: Data(contentsOf: home.appendingPathComponent("ack-envelope.json")))
		XCTAssertEqual(ack.data.acknowledged, 1)
		print("Mixed helper envelope: quota_alerts_failed; Codex add=1 ACK=1; Claude add=0 ACK=0")
	}

	func testRootResolutionConsumesGuardBudget() async throws {
		let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		try FileManager.default.createDirectory(at: root, withIntermediateDirectories: false, attributes: [.posixPermissions: 0o700])
		defer { try? FileManager.default.removeItem(at: root) }
		let fd = try createQuotaGuardFixture(root)
		defer { close(fd) }
		let epoch = root.appendingPathComponent("quota-delivery.epoch")
		try Data(Array("AGENTDECK-QDG-1\n".utf8) + [UInt8](repeating: 0, count: 8)).write(to: epoch)
		XCTAssertEqual(chmod(epoch.path, 0o600), 0)
		var info = stat()
		XCTAssertEqual(stat(root.path, &info), 0)
		let proof = "\(UInt64(bitPattern: Int64(info.st_dev))):\(info.st_ino)"
		let alert = DesktopQuotaAlertV1(id: "late", kind: .reset, client: "claude", usedPercent: 0, generation: 0, stateRootID: proof)
		let poster = RecordingNotificationPoster()
		let app = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster, stateRoot: {
			try await Task.sleep(for: .milliseconds(50))
			return root
		}, lockWait: .milliseconds(10))
		let ids = await app.deliver([alert])
		XCTAssertTrue(ids.isEmpty)
		XCTAssertTrue(poster.posted.isEmpty)
	}

	func testGuardRejectsForeignOwnerStat() {
		var entry = stat()
		entry.st_mode = mode_t(S_IFREG) | 0o600
		entry.st_nlink = 1
		entry.st_uid = geteuid()
		XCTAssertTrue(isPrivateQuotaDeliveryFile(entry))
		entry.st_uid ^= 1
		XCTAssertFalse(isPrivateQuotaDeliveryFile(entry))
	}

	func testIndependentGuardRefusesDamagedStateAndPreservesCodex() async throws {
		let fm = FileManager.default
		let root = fm.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		try fm.createDirectory(at: root, withIntermediateDirectories: false, attributes: [.posixPermissions: 0o700])
		defer { try? fm.removeItem(at: root) }
		var st = stat()
		XCTAssertEqual(stat(root.path, &st), 0)
		let id = "\(UInt64(bitPattern: Int64(st.st_dev))):\(st.st_ino)"
		let claude = DesktopQuotaAlertV1(id: "claude", kind: .reset, client: "claude", usedPercent: 0, generation: 0, stateRootID: id)
		let path = root.appendingPathComponent("quota-delivery.lock")
		let epoch = root.appendingPathComponent("quota-delivery.epoch")
		try Data(Array("AGENTDECK-QDG-1\n".utf8) + [UInt8](repeating: 0, count: 8)).write(to: epoch)
		XCTAssertEqual(chmod(epoch.path, 0o600), 0)
		let dir = open(root.path, O_RDONLY | O_DIRECTORY | O_CLOEXEC)
		defer { close(dir) }
		func denied() async throws {
			let before = try quotaGuardRootSnapshot(root)
			let poster = RecordingNotificationPoster()
			let app = QuotaAlertNotifier(permission: StubNotificationPermission(granted: true), poster: poster, stateRoot: { root }, lockWait: .milliseconds(40))
			let ids = await app.deliver([claude, threshold])
			XCTAssertEqual(ids, [threshold.id])
			XCTAssertEqual(poster.posted.map(\.identifier), [threshold.id])
			XCTAssertEqual(try quotaGuardRootSnapshot(root), before)
		}
		try await denied()
		XCTAssertFalse(fm.fileExists(atPath: path.path))
		XCTAssertEqual(close(try createQuotaGuardFixture(root)), 0)
		let old = open(path.path, O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)
		XCTAssertGreaterThanOrEqual(old, 0)
		defer { close(old) }
		XCTAssertEqual(flock(old, LOCK_EX | LOCK_NB), 0)
		let began = ContinuousClock.now
		try await denied()
		XCTAssertGreaterThanOrEqual(began.duration(to: .now), .milliseconds(40))
		XCTAssertLessThan(began.duration(to: .now), .seconds(1))
		XCTAssertEqual(flock(old, LOCK_UN), 0)
		XCTAssertEqual(chmod(path.path, 0o644), 0)
		try await denied()
		XCTAssertEqual(chmod(path.path, 0o600), 0)
		let alias = root.appendingPathComponent("second-link")
		try fm.linkItem(at: path, to: alias)
		try await denied()
		try fm.removeItem(at: alias)
		XCTAssertEqual(flock(old, LOCK_EX | LOCK_NB), 0)
		try fm.moveItem(at: path, to: alias)
		try await denied()
		try Data().write(to: path)
		XCTAssertEqual(chmod(path.path, 0o600), 0)
		try await denied()
		try fm.removeItem(at: path)
		try fm.createSymbolicLink(at: path, withDestinationURL: alias)
		try await denied()
		try fm.removeItem(at: path)
		XCTAssertEqual(mkfifo(path.path, 0o600), 0)
		try await denied()
		try fm.removeItem(at: path)
		try fm.moveItem(at: alias, to: path)
		XCTAssertEqual(flock(old, LOCK_UN), 0)
		var bad: UInt8 = 1
		XCTAssertEqual(fsetxattr(dir, "user.agentdeck.quota-delivery-lock", &bad, 1, 0, XATTR_REPLACE), 0)
		try await denied()
		XCTAssertEqual(fremovexattr(dir, "user.agentdeck.quota-delivery-lock", 0), 0)
		try await denied()
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
		// Codex PR #5 second review, P2: a vendor label alone cannot
		// distinguish this limit's 5-hour and 7-day windows, so the span is
		// always appended.
		let resetWindowName = "Opus · " + t(DesktopCopy.quotaWindow7d)
		XCTAssertEqual(resetContent.body, t(DesktopCopy.notificationQuotaResetBody, resetWindowName, Int64(3)))

		let unnamed = QuotaAlertContent(DesktopQuotaAlertV1(id: "qa1.x", kind: .reset, client: "codex", usedPercent: 1))
		XCTAssertTrue(unnamed.body.contains(t(DesktopCopy.notificationQuotaWindowFallback)))
	}
}

@MainActor
final class SettingsWindowLayoutTests: XCTestCase {
	func testQuotaClientChoicesStayVisibleButDisabledWhenMasterReadingIsOff() async throws {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let transport = StubQuotaSettingsTransport(settings: DesktopQuotaSettingsValuesV1(
			reading: false, clients: ["codex", "claude"], interval: .fiveMinutes,
			alerts: false, thresholds: [75, 90], resetNotice: true, statusline: false
		))
		let quotaSettings = makeQuotaSettingsController(preferences: preferences, transport: transport)
		let controller = SettingsWindowController(preferences: preferences, quotaSettings: quotaSettings, telemetrySettings: makeTelemetrySettingsController())
		controller.show(pane: .quota)
		let window = try XCTUnwrap(controller.window)
		defer { window.close() }
		await quotaSettings.load()
		settle(window)

		let hosting = try XCTUnwrap(controller.paneController(.quota)?.view)
		let codex = try XCTUnwrap(findButton(in: hosting, identifier: "quota-client-codex"))
		let claude = try XCTUnwrap(findButton(in: hosting, identifier: "quota-client-claude"))
		XCTAssertEqual(codex.title, "Codex")
		XCTAssertEqual(claude.title, "Claude")
		XCTAssertEqual(codex.accessibilityRole(), .checkBox)
		XCTAssertEqual(claude.accessibilityRole(), .checkBox)
		XCTAssertEqual(codex.accessibilityLabel(), "Codex")
		XCTAssertEqual(claude.accessibilityLabel(), "Claude")
		XCTAssertFalse(codex.isEnabled)
		XCTAssertFalse(claude.isEnabled)
		XCTAssertEqual(codex.state, .on, "the remembered selection remains checked while disabled")
		XCTAssertEqual(claude.state, .on)

		await quotaSettings.setReading(true)
		settle(window)
		XCTAssertTrue(codex.isEnabled)
		XCTAssertTrue(claude.isEnabled)

		claude.performClick(nil)
		for _ in 0..<50 {
			if await transport.applyCalls.count >= 2 { break }
			await Task.yield()
		}
		settle(window)
		XCTAssertEqual(claude.state, .off)
		XCTAssertEqual((claude.accessibilityValue() as? NSNumber)?.intValue, 0)
		XCTAssertEqual(quotaSettings.settings?.clients, ["codex"])
		let calls = await transport.applyCalls
		XCTAssertEqual(calls.last?.clients, ["codex"], "the mounted checkbox action must reach the settings transport")
	}

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
		let controller = SettingsWindowController(preferences: preferences, quotaSettings: quotaSettings, telemetrySettings: makeTelemetrySettingsController())
		controller.show(pane: .quota)
		let window = try XCTUnwrap(controller.window)
		defer { window.close() }
		await quotaSettings.load()
		settle(window)
		let heightWithoutWarning = window.contentLayoutRect.height

		await quotaSettings.setAlerts(true)
		XCTAssertNotNil(quotaSettings.alertsRow)
		settle(window)

		let natural = try XCTUnwrap(controller.naturalSize(of: .quota))
		XCTAssertGreaterThan(window.contentLayoutRect.height, heightWithoutWarning, "the window did not grow for the warning row")
		XCTAssertGreaterThanOrEqual(
			window.contentLayoutRect.height + 0.5, natural.height,
			"content taller than the window is compressed and overlaps"
		)
	}

	private func settle(_ window: NSWindow) {
		for _ in 0..<10 {
			window.contentView?.layoutSubtreeIfNeeded()
			RunLoop.main.run(until: Date().addingTimeInterval(0.05))
		}
	}

	private func findButton(in view: NSView, identifier: String) -> NSButton? {
		if let button = view as? NSButton, button.identifier?.rawValue == identifier { return button }
		for child in view.subviews {
			if let button = findButton(in: child, identifier: identifier) { return button }
		}
		return nil
	}
}
