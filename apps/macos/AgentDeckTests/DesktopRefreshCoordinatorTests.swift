import Foundation
import XCTest
@testable import AgentDeckShared

@MainActor
final class DesktopRefreshCoordinatorTests: XCTestCase {
	func testServingPartialPublicationDoesNotWaitForQuotaOrGlobalTerminal() async throws {
		let partial = try servingCoordinatorFixture(id: "first-partial", full: false)
		let final = try servingCoordinatorFixture(id: "final-full", full: true)
		let host = ControlledServingRefresher(restore: nil, partial: partial, final: final)
		let quota = SuspendingQuotaRefresher(alerts: [])
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, quotaRefresher: quota, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		await quota.waitUntilSuspended()
		for _ in 0 ..< 40 where coordinator.latestSnapshot == nil { await Task.yield() }
		XCTAssertEqual(coordinator.latestSnapshot, partial.payload)
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "first-partial")
		XCTAssertEqual(host.legacyCalls, 0)
		if case .running = coordinator.fullAttempt {} else { XCTFail("partial publication ended the finite job") }
		host.resume()
		await quota.resume()
		await initial.value
		XCTAssertEqual(coordinator.latestSnapshot, final.payload)
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "final-full")
	}

	func testServingRestoreSurvivesNarrowPartialAndFailedUpdate() async throws {
		let saved = try servingCoordinatorFixture(id: "saved-full", full: true)
		let partial = try servingCoordinatorFixture(id: "first-partial", full: false,
			generationChanges: ["usage_revision": 5, "session_revision": 6])
		let host = ControlledServingRefresher(restore: saved, partial: partial, final: nil)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		for _ in 0 ..< 40 where coordinator.servingHeader?.publicationID != "saved-full" { await Task.yield() }
		XCTAssertEqual(coordinator.latestSnapshot, saved.payload)
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.latestSnapshot, saved.payload)
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "saved-full")
		if case let .degraded(previous, _) = coordinator.state { XCTAssertEqual(previous, saved.payload) }
		else { XCTFail("failed publication claimed ready") }
	}

	func testServingCoreEpochChangeReplacesFullViewBeforeFailedUpdate() async throws {
		try await assertServingIdentityChange(generationChanges: ["core_epoch": 2])
	}

	func testServingDerivedOnlySchemaUpgradeKeepsFullViewDuringPartialUpdate() async throws {
		let saved = try servingCoordinatorFixture(id: "schema31-full", full: true)
		let partial = try servingCoordinatorFixture(id: "schema32-partial", full: false,
			generationChanges: ["core_schema": 32, "usage_revision": 5])
		let host = ControlledServingRefresher(restore: saved, partial: partial, final: nil)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		for _ in 0 ..< 40 where coordinator.servingHeader?.publicationID != "schema31-full" { await Task.yield() }
		XCTAssertEqual(coordinator.latestSnapshot, saved.payload)
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "schema31-full")
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.latestSnapshot, saved.payload)
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "schema31-full")
	}

	func testServingSessionEpochChangeReplacesFullViewBeforeFailedUpdate() async throws {
		try await assertServingIdentityChange(generationChanges: ["session_epoch": 3])
	}

	func testServingStateIdentityChangeReplacesFullViewBeforeFailedUpdate() async throws {
		try await assertServingIdentityChange(generationChanges: ["state_identity": "replacement-state"])
	}

	func testServingSchemaChangeReplacesFullViewBeforeFailedUpdate() async throws {
		try await assertServingIdentityChange(generationChanges: ["core_schema": 30])
	}

	func testServingUnusableNewIdentityClearsFullViewBeforeFailedUpdate() async throws {
		try await assertServingIdentityChange(generationChanges: ["core_epoch": 2], currentDay: false)
	}

	func testServingUnusableNewIdentityNormalReturnFinishesWithoutPublicationCallback() async throws {
		try await assertUnusableServingNormalReturn(publishesFinal: false, waitsForQuota: false)
	}

	func testServingUnusableNewIdentityNormalReturnFinishesWithQuota() async throws {
		try await assertUnusableServingNormalReturn(publishesFinal: true, waitsForQuota: true)
	}

	private func assertUnusableServingNormalReturn(publishesFinal: Bool, waitsForQuota: Bool) async throws {
		let saved = try servingCoordinatorFixture(id: "saved-full", full: true)
		let replacement = try servingCoordinatorFixture(id: "next-day-partial", full: false,
			generationChanges: ["core_epoch": 2], day: "2026-08-14", nextDay: "2026-08-15")
		let host = ControlledServingRefresher(restore: nil, partial: saved, final: replacement, publishesFinal: publishesFinal)
		let quota = waitsForQuota ? SuspendingQuotaRefresher(alerts: []) : nil
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, quotaRefresher: quota, snapshotStore: nil, wallNow: { now })
		var terminalCalls = 0
		coordinator.setFullAttemptTerminalHandler { terminalCalls += 1 }
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		if let quota { await quota.waitUntilSuspended() }
		for _ in 0 ..< 100 where coordinator.servingHeader != saved.header { await Task.yield() }
		XCTAssertEqual(coordinator.servingHeader, saved.header)
		XCTAssertEqual(coordinator.latestSnapshot, saved.payload)
		host.resume()
		if let quota {
			for _ in 0 ..< 100 {
				if case .failed = coordinator.fullAttempt { break }
				await Task.yield()
			}
			XCTAssertEqual(coordinator.state, .degraded(previous: nil, issue: .unavailable))
			XCTAssertEqual(terminalCalls, 0, "quota is still suspended")
			await quota.resume()
		}
		await initial.value
		XCTAssertNil(coordinator.servingHeader)
		XCTAssertNil(coordinator.latestSnapshot)
		XCTAssertNil(coordinator.scanProgress)
		XCTAssertEqual(coordinator.state, .degraded(previous: nil, issue: .unavailable))
		if case let .failed(_, issue, _) = coordinator.fullAttempt { XCTAssertEqual(issue, .unavailable) }
		else { XCTFail("normal bootstrap return left the finite job without a failed terminal outcome") }
		XCTAssertEqual(terminalCalls, 1)
		XCTAssertEqual(host.legacyCalls, 0)
	}

	private func assertServingIdentityChange(generationChanges: [String: Any], currentDay: Bool = true) async throws {
		let saved = try servingCoordinatorFixture(id: "saved-full", full: true)
		let replacement = try servingCoordinatorFixture(id: "replacement-partial", full: false,
			generationChanges: generationChanges, day: currentDay ? "2026-08-13" : "2026-08-14",
			nextDay: currentDay ? "2026-08-14" : "2026-08-15")
		let host = ControlledServingRefresher(restore: nil, partial: saved, final: nil)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		for _ in 0 ..< 100 where coordinator.servingHeader?.publicationID != "saved-full" { await Task.yield() }
		XCTAssertEqual(coordinator.latestSnapshot, saved.payload)
		XCTAssertEqual(coordinator.servingHeader, saved.header)

		host.publish(replacement)
		let expectedHeader = currentDay ? replacement.header : nil
		let expectedPayload = currentDay ? replacement.payload : nil
		for _ in 0 ..< 100 where coordinator.servingHeader != expectedHeader { await Task.yield() }
		XCTAssertEqual(coordinator.servingHeader, expectedHeader)
		XCTAssertEqual(coordinator.latestSnapshot, expectedPayload)
		XCTAssertEqual(coordinator.state, .refreshing(previous: expectedPayload))
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.servingHeader, expectedHeader)
		XCTAssertEqual(coordinator.latestSnapshot, expectedPayload)
		if case let .degraded(previous, _) = coordinator.state { XCTAssertEqual(previous, expectedPayload) }
		else { XCTFail("failed publication claimed ready") }
		XCTAssertEqual(host.legacyCalls, 0)
	}

	func testServingWindowUsesCalendarAcrossDSTAndRejectsTomorrow() throws {
		let original = try servingCoordinatorFixture(id: "window", full: true)
		var body = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(original)) as? [String: Any])
		var header = try XCTUnwrap(body["header"] as? [String: Any])
		header["timezone"] = "America/New_York"
		header["window_start"] = "2026-11-01T04:00:00Z"
		header["window_end"] = "2026-11-02T05:00:00Z"
		header["generated_at"] = "2026-11-01T16:00:00Z"
		body["header"] = header
		var payload = try XCTUnwrap(body["payload"] as? [String: Any])
		var data = try XCTUnwrap(payload["data"] as? [String: Any])
		payload["generated_at"] = "2026-11-01T16:00:00Z"
		data["generated_at"] = "2026-11-01T16:00:00Z"
		payload["data"] = data; body["payload"] = payload
		let decoded = try decodeDesktopServingSnapshotV2(JSONSerialization.data(withJSONObject: body))
		let clock = ISO8601DateFormatter()
		XCTAssertTrue(decoded.header.matchesCurrentDay(now: clock.date(from: "2026-11-01T16:00:00Z")!))
		XCTAssertFalse(decoded.header.matchesCurrentDay(now: clock.date(from: "2026-11-02T16:00:00Z")!))
	}

	func testWaitingProgressKeepsInventoryTotalUnknown() {
		XCTAssertNil(DesktopScanProgress.waiting.usage.total)
		XCTAssertNil(DesktopScanProgress.waiting.session.total)
	}

	func testRefreshPublishesLiveProgressAndKeepsPreviousSnapshotUntilSuccess() async throws {
		let previous = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let next = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		let host = ProgressingSnapshotRefresher(initial: previous, next: next)
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		await coordinator.startInitialRefresh().value

		let refresh = Task { await coordinator.refresh() }
		await host.waitUntilSuspended()
		await Task.yield()
		XCTAssertEqual(coordinator.latestSnapshot, previous)
		XCTAssertEqual(coordinator.scanProgress?.stage, .importing)
		XCTAssertEqual(coordinator.scanProgress?.usage.committed, 3)
		if case let .refreshing(retained) = coordinator.state {
			XCTAssertEqual(retained, previous)
		} else {
			XCTFail("expected refreshing state")
		}

		host.resume()
		await refresh.value
		XCTAssertEqual(coordinator.latestSnapshot, next)
		XCTAssertNil(coordinator.scanProgress)
	}

	func testInitialRefreshPublishesMemoryAndAppGroupProjection() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let quotaRefresher = RecordingQuotaRefresher()
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(directoryURL: temporaryDirectory)
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			quotaRefresher: quotaRefresher,
			snapshotStore: store
		)

		await coordinator.startInitialRefresh().value

		XCTAssertEqual(coordinator.state, .ready(complete))
		XCTAssertEqual(coordinator.latestSnapshot, complete)
		XCTAssertEqual(try store.read(), AppGroupDesktopSnapshotV1(envelope: complete))
		let quotaCalls = await quotaRefresher.recordedManualValues()
		XCTAssertEqual(quotaCalls, [false])
	}

	// Codex PR #5 tenth review, P1: quota alert evaluation must be runnable
	// on its own, independent of the full desktop snapshot refresh -- so a
	// caller can schedule it on a cadence that does not also force a
	// session/usage rescan. `responses: []` makes the host throw if
	// `refreshQuotaAlertsOnly` ever touches it, proving it does not.
	func testRefreshQuotaAlertsOnlyDeliversAlertsWithoutTouchingTheSnapshot() async throws {
		let posted = DesktopQuotaAlertV1(id: "qa1.posted", kind: .threshold, client: "codex", windowMinutes: 300, usedPercent: 80, threshold: 75)
		let quotaRefresher = RecordingQuotaRefresher(alerts: [posted])
		let deliverer = RecordingAlertDeliverer(accepts: ["qa1.posted"])
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: []),
			quotaRefresher: quotaRefresher,
			alertDeliverer: deliverer,
			snapshotStore: nil
		)

		await coordinator.refreshQuotaAlertsOnly(manual: false)

		let quotaCalls = await quotaRefresher.recordedManualValues()
		XCTAssertEqual(quotaCalls, [false])
		let acknowledgements = await quotaRefresher.recordedAcknowledgements()
		XCTAssertEqual(acknowledgements, [["qa1.posted"]])
		XCTAssertNil(coordinator.latestSnapshot, "must not have touched the full snapshot refresh")
		XCTAssertEqual(coordinator.state, .uninitialized)
	}

	// Codex PR #5 twelfth review, P2: when the full snapshot refresh's own
	// separate, opt-in-and-off-by-default preference never runs,
	// refreshQuotaAlertsOnly is the only recurring caller into the quota
	// probe -- so it must also publish the fresher figures that probe just
	// persisted, without rerunning the expensive session/usage scan.
	func testRefreshQuotaAlertsOnlySplicesFreshSubscriptionIntoTheRetainedSnapshot() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let quotaRefresher = RecordingQuotaRefresher()
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(directoryURL: temporaryDirectory)
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			quotaRefresher: quotaRefresher,
			snapshotStore: store
		)
		await coordinator.startInitialRefresh().value
		XCTAssertEqual(coordinator.latestSnapshot, complete)

		let freshSubscription = DesktopSubscriptionSnapshotV1(available: true, clients: [])
		await quotaRefresher.setSubscription(freshSubscription)
		await coordinator.refreshQuotaAlertsOnly(manual: false)

		XCTAssertEqual(coordinator.latestSnapshot?.data.subscription, freshSubscription)
		XCTAssertEqual(coordinator.latestSnapshot?.data.usage, complete.data.usage, "the session/usage scan must not be rerun")
		XCTAssertEqual(coordinator.state, .ready(try XCTUnwrap(coordinator.latestSnapshot)))
		XCTAssertEqual(try store.read().subscription, freshSubscription, "the App Group projection widgets read must also carry the fresher figures")
	}

	func testUserRefreshRequestsManualQuotaBeforeReadingTheSnapshot() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let quotaRefresher = RecordingQuotaRefresher()
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			quotaRefresher: quotaRefresher,
			snapshotStore: nil
		)

		await coordinator.refresh()

		let quotaCalls = await quotaRefresher.recordedManualValues()
		XCTAssertEqual(quotaCalls, [true])
		XCTAssertEqual(coordinator.latestSnapshot, complete)
	}

	func testProviderSwitchDuringFullRefreshCreatesExactlyOneFollowUp() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let second = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		let host = ControlledSnapshotRefresher(responses: [first, second])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)

		let initial = Task { await coordinator.requestFullRefresh(trigger: .manual) }
		await host.waitForCall(1)
		let passiveJoin = Task { await coordinator.requestFullRefresh(trigger: .periodic) }
		let providerFollowUp = Task { await coordinator.requestFullRefresh(trigger: .providerSwitch) }
		host.resumeCall(1)
		await host.waitForCall(2)
		XCTAssertEqual(host.callCount, 2)
		host.resumeCall(2)
		await initial.value
		await passiveJoin.value
		await providerFollowUp.value

		XCTAssertEqual(host.callCount, 2)
		XCTAssertEqual(coordinator.latestSnapshot, second)
	}

	func testSourceBurstDuringScanCreatesOneFollowUpAndRetainsPublishedValue() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let second = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		let host = ControlledSnapshotRefresher(responses: [first, second])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .sourceChange) }
		await host.waitForCall(1)
		let notifications = (0..<8).map { _ in Task { await coordinator.requestFullRefresh(trigger: .sourceChange) } }
		for _ in 0..<50 { await Task.yield() }
		host.resumeCall(1)
		await host.waitForCall(2)
		XCTAssertEqual(coordinator.latestSnapshot, first)
		host.resumeCall(2)
		await initial.value
		for notification in notifications { await notification.value }
		XCTAssertEqual(host.callCount, 2)
		XCTAssertEqual(coordinator.latestSnapshot, second)
	}

	func testConcurrentQuotaRequestsShareOneOwnerSideEffectSequence() async throws {
		let alert = DesktopQuotaAlertV1(id: "quota.shared", kind: .threshold, client: "codex", usedPercent: 80, threshold: 75)
		let quota = SuspendingQuotaRefresher(alerts: [alert])
		let deliverer = RecordingAlertDeliverer(accepts: [alert.id])
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: []),
			quotaRefresher: quota,
			alertDeliverer: deliverer,
			snapshotStore: nil
		)

		let periodic = Task { await coordinator.requestQuotaRefresh(manual: false) }
		await quota.waitUntilSuspended()
		let manualJoiner = Task { await coordinator.requestQuotaRefresh(manual: true) }
		await Task.yield()
		let countWhileSuspended = await quota.refreshCount
		XCTAssertEqual(countWhileSuspended, 1)
		await quota.resume()
		await periodic.value
		await manualJoiner.value

		let finalRefreshCount = await quota.refreshCount
		let finalFetchCount = await quota.fetchCount
		let acknowledgements = await quota.recordedAcknowledgements()
		let offers = await deliverer.recordedOffers()
		XCTAssertEqual(finalRefreshCount, 1)
		XCTAssertEqual(finalFetchCount, 1)
		XCTAssertEqual(acknowledgements, [[alert.id]])
		XCTAssertEqual(offers, [[alert]])
	}

	func testQuotaFirstFullRefreshJoinsExistingOperationBeforeSnapshot() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let quota = SuspendingQuotaRefresher(alerts: [])
		let host = ScriptedSnapshotRefresher(responses: [.snapshot(snapshot)])
		let coordinator = DesktopRefreshCoordinator(host: host, quotaRefresher: quota, snapshotStore: nil)

		let quotaRequest = Task { await coordinator.requestQuotaRefresh(manual: false) }
		await quota.waitUntilSuspended()
		let full = Task { await coordinator.requestFullRefresh(trigger: .manual) }
		await Task.yield()
		let countBeforeResume = await quota.refreshCount
		XCTAssertEqual(countBeforeResume, 1)
		await quota.resume()
		await quotaRequest.value
		await full.value

		let finalCount = await quota.refreshCount
		let manualValues = await quota.recordedManualValues()
		XCTAssertEqual(finalCount, 1)
		XCTAssertEqual(manualValues, [false])
		XCTAssertEqual(coordinator.latestSnapshot, snapshot)
	}

	func testFullFirstQuotaRequestJoinsFullOwnedOperation() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let quota = SuspendingQuotaRefresher(alerts: [])
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(snapshot)]),
			quotaRefresher: quota,
			snapshotStore: nil
		)

		let full = Task { await coordinator.requestFullRefresh(trigger: .manual) }
		await quota.waitUntilSuspended()
		let quotaJoiner = Task { await coordinator.requestQuotaRefresh(manual: false) }
		await Task.yield()
		let countBeforeResume = await quota.refreshCount
		XCTAssertEqual(countBeforeResume, 1)
		await quota.resume()
		await full.value
		await quotaJoiner.value

		let finalCount = await quota.refreshCount
		let manualValues = await quota.recordedManualValues()
		XCTAssertEqual(finalCount, 1)
		XCTAssertEqual(manualValues, [true])
		XCTAssertEqual(coordinator.latestSnapshot, snapshot)
	}

	func testQuotaTickStormAfterFullQuotaPhaseCollapsesAndManualUpgradesPending() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [snapshot])
		let quota = SequencedQuotaRefresher()
		let coordinator = DesktopRefreshCoordinator(
			host: host,
			quotaRefresher: quota,
			snapshotStore: nil
		)

		let full = Task { await coordinator.requestFullRefresh(trigger: .periodic) }
		await host.waitForCall(1)
		let periodicJoiners = (0 ..< 10).map { _ in
			Task { await coordinator.requestQuotaRefresh(manual: false) }
		}
		let manualJoiner = Task { await coordinator.requestQuotaRefresh(manual: true) }
		host.resumeCall(1)
		await quota.waitForSecondOperation()

		let manualValuesWhilePending = await quota.manualValues
		XCTAssertEqual(manualValuesWhilePending, [false, true])
		await quota.resumeSecondOperation()
		await full.value
		for joiner in periodicJoiners { await joiner.value }
		await manualJoiner.value
		let finalManualValues = await quota.manualValues
		XCTAssertEqual(finalManualValues, [false, true])
	}

	/// architecture.md C10: the app acknowledges only what the notification
	/// service accepted, so a refused alert stays due for the next refresh.
	func testRefreshAcknowledgesOnlyTheAlertsTheDelivererAccepted() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let posted = DesktopQuotaAlertV1(id: "qa1.posted", kind: .threshold, client: "codex", windowMinutes: 300, usedPercent: 80, threshold: 75)
		let refused = DesktopQuotaAlertV1(id: "qa1.refused", kind: .reset, client: "claude", windowMinutes: 10080, usedPercent: 3)
		let quotaRefresher = RecordingQuotaRefresher(alerts: [posted, refused])
		let deliverer = RecordingAlertDeliverer(accepts: ["qa1.posted"])
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			quotaRefresher: quotaRefresher,
			alertDeliverer: deliverer,
			snapshotStore: nil
		)

		await coordinator.refresh()

		let offers = await deliverer.recordedOffers()
		XCTAssertEqual(offers, [[posted, refused]])
		let acknowledgements = await quotaRefresher.recordedAcknowledgements()
		XCTAssertEqual(acknowledgements, [["qa1.posted"]])
		XCTAssertEqual(coordinator.latestSnapshot, complete)
	}

	func testRefreshWithNoDueAlertsNeitherDeliversNorAcknowledges() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let quotaRefresher = RecordingQuotaRefresher()
		let deliverer = RecordingAlertDeliverer(accepts: [])
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			quotaRefresher: quotaRefresher,
			alertDeliverer: deliverer,
			snapshotStore: nil
		)

		await coordinator.refresh()

		let offers = await deliverer.recordedOffers()
		XCTAssertTrue(offers.isEmpty)
		let acknowledgements = await quotaRefresher.recordedAcknowledgements()
		XCTAssertTrue(acknowledgements.isEmpty)
	}

	func testRefreshFailureRetainsLastGoodStateAndCache() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(directoryURL: temporaryDirectory)
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [
				.snapshot(complete),
				.helperFailure(.nonZeroExit(1)),
			]),
			snapshotStore: store
		)

		await coordinator.startInitialRefresh().value
		await coordinator.refresh()

		XCTAssertEqual(
			coordinator.state,
			.degraded(previous: complete, issue: .helper(.nonZeroExit(1)))
		)
		XCTAssertEqual(coordinator.latestSnapshot, complete)
		XCTAssertEqual(try store.read(), AppGroupDesktopSnapshotV1(envelope: complete))
		// The failing helper never reported real progress past the synthetic
		// `.waiting` set at refresh start, so the failure surface must not keep
		// showing a stale "waiting to scan" state.
		XCTAssertNil(coordinator.scanProgress)
	}

	func testLateQueuedNonterminalProgressIsRejectedAfterFailure() async {
		let host = LateFailureProgressRefresher(stage: .importing)
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)

		await coordinator.refresh()
		await host.releaseProgress()
		await Task.yield()

		XCTAssertNil(coordinator.scanProgress)
	}

	func testLateQueuedCompletedProgressIsRetainedAfterFailure() async {
		let host = LateFailureProgressRefresher(stage: .completed)
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)

		await coordinator.refresh()
		await host.releaseProgress()
		await Task.yield()

		XCTAssertEqual(coordinator.scanProgress?.stage, .completed)
	}

	func testMalformedTimestampFailureDoesNotReplaceLastGoodStateOrCache() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(directoryURL: temporaryDirectory)
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [
				.snapshot(complete),
				.wireFailure(.invalidTimestamp),
			]),
			snapshotStore: store
		)

		await coordinator.startInitialRefresh().value
		await coordinator.refresh()

		XCTAssertEqual(
			coordinator.state,
			.degraded(previous: complete, issue: .invalidWire(.invalidTimestamp))
		)
		XCTAssertEqual(coordinator.latestSnapshot, complete)
		XCTAssertEqual(try store.read(), AppGroupDesktopSnapshotV1(envelope: complete))
	}

	func testUnavailableContainerRecordsFullAndQuotaPublicationFailures() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let quota = RecordingQuotaRefresher()
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			quotaRefresher: quota,
			snapshotStore: nil
		)
		XCTAssertEqual(coordinator.widgetPublication, .neverPublished)
		await coordinator.startInitialRefresh().value
		XCTAssertEqual(coordinator.state, .ready(complete))
		XCTAssertEqual(coordinator.widgetPublication, .failedBeforeCommit(generation: 1, issue: .storageUnavailable))
		let subscription = DesktopSubscriptionSnapshotV1(available: true, clients: [])
		await quota.setSubscription(subscription)
		await coordinator.requestQuotaRefresh(manual: false)
		let updated = complete.replacingSubscription(subscription)
		XCTAssertEqual(coordinator.state, .ready(updated))
		XCTAssertEqual(coordinator.latestSnapshot, updated)
		XCTAssertEqual(coordinator.widgetPublication, .failedBeforeCommit(generation: 2, issue: .storageUnavailable))
		let calls = await quota.recordedManualValues()
		XCTAssertEqual(calls, [false, false])
		let presentation = DesktopPresentationState.derive(from: coordinator.state)
		XCTAssertFalse(presentation.qualifiers.contains(.stale))
		XCTAssertFalse(presentation.isBadged)
	}

	func testUsableContainerPublishesAfterCoordinatorRecreation() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let unavailable = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]), snapshotStore: nil
		)
		await unavailable.refresh()
		XCTAssertEqual(unavailable.widgetPublication, .failedBeforeCommit(generation: 1, issue: .storageUnavailable))
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let store = AppGroupSnapshotStore(directoryURL: directory)
		let recovered = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]), snapshotStore: store
		)
		await recovered.refresh()
		XCTAssertEqual(recovered.widgetPublication, .succeeded(generation: 1, affectedKinds: Set(AppGroupWidgetKind.allCases)))
		XCTAssertEqual(try store.read(), AppGroupDesktopSnapshotV1(envelope: complete))
		XCTAssertEqual(recovered.state, .ready(complete))
	}

	func testCacheWriteFailureKeepsFreshMenuDataAndRecordsPublicationFailure() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let store = AppGroupSnapshotStore(
			directoryURL: temporaryDirectory,
			atomicReplace: { _, _ in throw CacheReplacementError.failed }
		)
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			snapshotStore: store
		)

		await coordinator.startInitialRefresh().value

		XCTAssertEqual(coordinator.latestSnapshot, complete)
		XCTAssertEqual(coordinator.state, .ready(complete))
		XCTAssertEqual(
			coordinator.widgetPublication,
			.failedBeforeCommit(generation: 1, issue: .storageUnavailable)
		)
		let presentation = DesktopPresentationState.derive(from: coordinator.state)
		XCTAssertEqual(presentation.surface, .dataSurface)
		XCTAssertFalse(presentation.qualifiers.contains(.failing))
	}

	func testPresentationDerivesSurfaceAndOrderedOrthogonalQualifiers() throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let partial = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-partial.json"))
		let current = try XCTUnwrap(ISO8601DateFormatter().date(from: "2026-08-13T10:10:00Z"))
		let aged = try XCTUnwrap(ISO8601DateFormatter().date(from: "2026-08-13T10:20:01Z"))

		XCTAssertEqual(
			DesktopPresentationState.derive(from: .uninitialized, now: current),
			DesktopPresentationState(surface: .loadingSurface, qualifiers: [], snapshot: nil)
		)
		XCTAssertEqual(
			DesktopPresentationState.derive(from: .refreshing(previous: complete), now: current).qualifiers,
			[.stale]
		)
		XCTAssertEqual(
			DesktopPresentationState.derive(from: .ready(complete), now: aged).qualifiers,
			[.aged]
		)
		XCTAssertEqual(
			DesktopPresentationState.derive(from: .ready(partial), now: current).qualifiers,
			[.partial]
		)
		let retainedFailure = DesktopPresentationState.derive(
			from: .degraded(previous: complete, issue: .helper(.timedOut)),
			now: current
		)
		XCTAssertEqual(retainedFailure.surface, .dataSurface)
		XCTAssertEqual(retainedFailure.qualifiers, [.stale, .failing])
		XCTAssertTrue(retainedFailure.isBadged)
		let emptyFailure = DesktopPresentationState.derive(
			from: .degraded(previous: nil, issue: .invalidWire(.invalidEnvelope)),
			now: current
		)
		XCTAssertEqual(emptyFailure.surface, .errorSurface)
		XCTAssertEqual(emptyFailure.qualifiers, [.failing])
	}

	func testPresentationAgeReevaluatesTheClockAtTheExactBoundary() throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let generated = try XCTUnwrap(DesktopTimestamp.date(from: complete.data.generatedAt))
		for offset in [900.0, 900.001, 10.0, 901.0] {
			let presentation = DesktopPresentationState.derive(from: .ready(complete), now: generated.addingTimeInterval(offset))
			XCTAssertEqual(presentation.qualifiers.contains(.aged), offset > 900)
			XCTAssertEqual(presentation.snapshot, complete)
		}
	}

	func testDesktopTimestampValueParserPreservesOffsetsAndFractionalPrecision() throws {
		let whole = try XCTUnwrap(DesktopTimestamp.date(from: "2026-08-13T10:00:00Z"))
		for value in ["2026-08-13T12:00:00+02:00", "2026-08-13T05:00:00-05:00", "2026-08-13T10:00:00.000Z"] {
			XCTAssertEqual(DesktopTimestamp.date(from: value), whole)
		}
		let fractional = try XCTUnwrap(DesktopTimestamp.date(from: "2026-08-13T10:00:00.123456Z"))
		XCTAssertEqual(fractional.timeIntervalSince(whole), 0.123456, accuracy: 0.000001)
		XCTAssertEqual(DesktopTimestamp.date(from: "2026-08-13T12:00:00.123456+02:00"), fractional)
		let legacy = ISO8601DateFormatter()
		legacy.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
		let extended = "2026-08-13T10:00:00.123456789123Z"
		XCTAssertEqual(DesktopTimestamp.date(from: extended), try XCTUnwrap(legacy.date(from: extended)))
		XCTAssertNil(DesktopTimestamp.date(from: "not-a-timestamp"))
		XCTAssertNil(DesktopTimestamp.date(from: "2026-08-13"))
		XCTAssertEqual(DesktopTimestamp.date(from: "2026-08-13T10:00:00Z"), whole)
	}
}

private func servingCoordinatorFixture(id: String, full: Bool, generationChanges: [String: Any] = [:],
	day: String = "2026-08-13", nextDay: String = "2026-08-14") throws -> DesktopServingSnapshotV2 {
	var payload = try XCTUnwrap(JSONSerialization.jsonObject(with: desktopFixtureData("snapshot-complete.json")) as? [String: Any])
	let generatedAt = "\(day)T10:00:00Z"
	payload["generated_at"] = generatedAt
	var payloadData = try XCTUnwrap(payload["data"] as? [String: Any])
	payloadData["generated_at"] = generatedAt
	payload["data"] = payloadData
	if !full {
		payload["partial"] = true
		var data = try XCTUnwrap(payload["data"] as? [String: Any])
		var sessions = try XCTUnwrap(data["sessions"] as? [String: Any])
		sessions["available"] = false
		data["sessions"] = sessions
		payload["data"] = data
	}
	let coverage: [String: Any] = ["state": full ? "complete" : "partial", "inventory_id": "fixture", "observed_at": generatedAt, "sources": 2, "committed": full ? 2 : 1]
	var generation: [String: Any] = ["state_identity": "fixture-state", "core_epoch": 1, "session_epoch": 2, "core_schema": 31, "session_schema": 1, "usage_revision": 3, "session_revision": 4]
	generation.merge(generationChanges) { _, new in new }
	let body: [String: Any] = ["payload": payload, "header": [
		"format_version": 1, "wire_version": 2, "publication_id": id, "projection_version": 1,
		"usage_parser_version": 1, "session_prepared": full, "timezone": "UTC",
		"window_start": "\(day)T00:00:00Z", "window_end": "\(nextDay)T00:00:00Z", "generated_at": generatedAt,
		"active_generation": "1:3/2:4", "pricing_revision": 3, "data_observed_through": generatedAt, "freshness": "stale",
		"usage_coverage": coverage, "session_coverage": coverage,
		"generation": generation,
	]]
	return try decodeDesktopServingSnapshotV2(JSONSerialization.data(withJSONObject: body))
}

@MainActor
private final class ControlledServingRefresher: DesktopSnapshotRefreshing, DesktopSnapshotBootstrapping {
	private let restore: DesktopServingSnapshotV2?
	private let partial: DesktopServingSnapshotV2
	private let final: DesktopServingSnapshotV2?
	private let publishesFinal: Bool
	private var publication: (@Sendable (DesktopServingSnapshotV2) -> Void)?
	private var continuation: CheckedContinuation<Void, Never>?
	private(set) var legacyCalls = 0
	init(restore: DesktopServingSnapshotV2?, partial: DesktopServingSnapshotV2, final: DesktopServingSnapshotV2?, publishesFinal: Bool = true) {
		self.restore = restore; self.partial = partial; self.final = final
		self.publishesFinal = publishesFinal
	}
	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		legacyCalls += 1
		throw HelperExecutionError.malformedOutput
	}
	func readServingSnapshot() async throws -> DesktopServingSnapshotV2 {
		guard let restore else { throw HelperExecutionError.malformedOutput }
		return restore
	}
	func bootstrapServingSnapshot(progress: @escaping @Sendable (DesktopScanProgress) -> Void,
		publication: @escaping @Sendable (DesktopServingSnapshotV2) -> Void) async throws -> DesktopServingSnapshotV2? {
		self.publication = publication
		publication(partial)
		await withCheckedContinuation { continuation = $0 }
		guard let final else { throw HelperExecutionError.nonZeroExit(1) }
		if publishesFinal { publication(final) }
		return final
	}
	func waitUntilSuspended() async { while continuation == nil { await Task.yield() } }
	func publish(_ snapshot: DesktopServingSnapshotV2) { publication?(snapshot) }
	func resume() { continuation?.resume(); continuation = nil }
}

private enum CacheReplacementError: Error {
	case failed
}

private actor RecordingQuotaRefresher: DesktopQuotaRefreshing {
	private var manualValues = [Bool]()
	private var acknowledgements = [[String]]()
	private let alerts: [DesktopQuotaAlertV1]

	init(alerts: [DesktopQuotaAlertV1] = []) {
		self.alerts = alerts
	}

	func refreshQuota(manual: Bool) async -> [DesktopQuotaAlertV1] {
		manualValues.append(manual)
		return alerts
	}

	func acknowledgeQuotaAlerts(ids: [String]) async { acknowledgements.append(ids) }
	func recordedManualValues() -> [Bool] { manualValues }
	func recordedAcknowledgements() -> [[String]] { acknowledgements }
	func fetchSubscription() async -> DesktopSubscriptionSnapshotV1? { subscription }

	private var subscription: DesktopSubscriptionSnapshotV1?
	func setSubscription(_ value: DesktopSubscriptionSnapshotV1?) { subscription = value }
}

private actor RecordingAlertDeliverer: QuotaAlertDelivering {
	private var offered = [[DesktopQuotaAlertV1]]()
	private let accepts: Set<String>

	init(accepts: Set<String>) {
		self.accepts = accepts
	}

	func deliver(_ alerts: [DesktopQuotaAlertV1]) async -> [String] {
		offered.append(alerts)
		return alerts.map(\.id).filter { accepts.contains($0) }
	}

	func recordedOffers() -> [[DesktopQuotaAlertV1]] { offered }
}

private actor SuspendingQuotaRefresher: DesktopQuotaRefreshing {
	private(set) var refreshCount = 0
	private(set) var fetchCount = 0
	private var acknowledgements = [[String]]()
	private var manualValues = [Bool]()
	private var continuation: CheckedContinuation<Void, Never>?
	private let alerts: [DesktopQuotaAlertV1]

	init(alerts: [DesktopQuotaAlertV1]) {
		self.alerts = alerts
	}

	func refreshQuota(manual: Bool) async -> [DesktopQuotaAlertV1] {
		refreshCount += 1
		manualValues.append(manual)
		await withCheckedContinuation { continuation = $0 }
		return alerts
	}

	func acknowledgeQuotaAlerts(ids: [String]) async { acknowledgements.append(ids) }
	func fetchSubscription() async -> DesktopSubscriptionSnapshotV1? {
		fetchCount += 1
		return nil
	}
	func recordedAcknowledgements() -> [[String]] { acknowledgements }
	func recordedManualValues() -> [Bool] { manualValues }

	func waitUntilSuspended() async {
		while continuation == nil { await Task.yield() }
	}

	func resume() {
		continuation?.resume()
		continuation = nil
	}
}

private actor SequencedQuotaRefresher: DesktopQuotaRefreshing {
	private(set) var manualValues = [Bool]()
	private var secondContinuation: CheckedContinuation<Void, Never>?

	func refreshQuota(manual: Bool) async -> [DesktopQuotaAlertV1] {
		manualValues.append(manual)
		if manualValues.count == 2 {
			await withCheckedContinuation { secondContinuation = $0 }
		}
		return []
	}

	func acknowledgeQuotaAlerts(ids: [String]) async {}
	func fetchSubscription() async -> DesktopSubscriptionSnapshotV1? { nil }

	func waitForSecondOperation() async {
		while manualValues.count < 2 || secondContinuation == nil { await Task.yield() }
	}

	func resumeSecondOperation() {
		secondContinuation?.resume()
		secondContinuation = nil
	}
}

@MainActor
private final class ControlledSnapshotRefresher: DesktopSnapshotRefreshing {
	private var responses: [DesktopWireEnvelopeV1]
	private var continuations = [Int: CheckedContinuation<Void, Never>]()
	private(set) var callCount = 0

	init(responses: [DesktopWireEnvelopeV1]) {
		self.responses = responses
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		try await refresh(recentLimit: recentLimit, progress: { _ in })
	}

	func refresh(
		recentLimit: Int,
		progress: @escaping @Sendable (DesktopScanProgress) -> Void
	) async throws -> DesktopWireEnvelopeV1 {
		callCount += 1
		let call = callCount
		await withCheckedContinuation { continuations[call] = $0 }
		return responses.removeFirst()
	}

	func waitForCall(_ expected: Int) async {
		while callCount < expected || continuations[expected] == nil { await Task.yield() }
	}

	func resumeCall(_ call: Int) {
		continuations.removeValue(forKey: call)?.resume()
	}
}

@MainActor
private final class LateFailureProgressRefresher: DesktopSnapshotRefreshing {
	private let stage: DesktopScanStage
	private var queuedProgress: (@Sendable (DesktopScanProgress) -> Void)?

	init(stage: DesktopScanStage) {
		self.stage = stage
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		throw HelperExecutionError.nonZeroExit(1)
	}

	func refresh(recentLimit: Int, progress: @escaping @Sendable (DesktopScanProgress) -> Void) async throws -> DesktopWireEnvelopeV1 {
		queuedProgress = progress
		throw HelperExecutionError.nonZeroExit(1)
	}

	func releaseProgress() async {
		queuedProgress?(DesktopScanProgress(
			sequence: 1,
			stage: stage,
			usage: DesktopScanDomainProgress(state: stage == .completed ? "failed" : "processing", committed: 0, total: 1, skipped: 0),
			session: DesktopScanDomainProgress(state: stage == .completed ? "completed" : "processing", committed: 1, total: 1, skipped: 0)
		))
	}
}

@MainActor
private final class ProgressingSnapshotRefresher: DesktopSnapshotRefreshing {
	private let initial: DesktopWireEnvelopeV1
	private let next: DesktopWireEnvelopeV1
	private var calls = 0
	private var continuation: CheckedContinuation<Void, Never>?

	init(initial: DesktopWireEnvelopeV1, next: DesktopWireEnvelopeV1) {
		self.initial = initial
		self.next = next
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		calls += 1
		return calls == 1 ? initial : next
	}

	func refresh(recentLimit: Int, progress: @escaping @Sendable (DesktopScanProgress) -> Void) async throws -> DesktopWireEnvelopeV1 {
		calls += 1
		if calls == 1 {
			return initial
		}
		progress(.waiting)
		progress(DesktopScanProgress(
			sequence: 1,
			stage: .importing,
			usage: DesktopScanDomainProgress(state: "processing", committed: 3, total: 5, skipped: 0),
			session: DesktopScanDomainProgress(state: "processing", committed: 2, total: 5, skipped: 1)
		))
		await withCheckedContinuation { continuation = $0 }
		progress(DesktopScanProgress(
			sequence: 2,
			stage: .statistics,
			usage: DesktopScanDomainProgress(state: "completed", committed: 5, total: 5, skipped: 0),
			session: DesktopScanDomainProgress(state: "completed", committed: 5, total: 5, skipped: 1)
		))
		return next
	}

	func waitUntilSuspended() async {
		while continuation == nil {
			await Task.yield()
		}
	}

	func resume() {
		continuation?.resume()
		continuation = nil
	}
}

@MainActor
private final class ScriptedSnapshotRefresher: DesktopSnapshotRefreshing {
	fileprivate enum Response {
		case snapshot(DesktopWireEnvelopeV1)
		case helperFailure(HelperExecutionError)
		case wireFailure(DesktopWireError)
	}

	private var responses: [Response]

	fileprivate init(responses: [Response]) {
		self.responses = responses
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		guard !responses.isEmpty else {
			throw HelperExecutionError.launchFailed
		}

		switch responses.removeFirst() {
		case let .snapshot(envelope):
			return envelope
		case let .helperFailure(error):
			throw error
		case let .wireFailure(error):
			throw error
		}
	}
}
