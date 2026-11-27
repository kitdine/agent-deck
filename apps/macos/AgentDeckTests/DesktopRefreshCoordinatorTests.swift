import Foundation
import XCTest
@_spi(DesktopApp) @testable import AgentDeckShared

@MainActor
final class DesktopRefreshCoordinatorTests: XCTestCase {
	func testStandaloneRejectsStrictlyOlderQuota() async throws {
		let page = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let installed = page.replacingSubscription(try newerQuota(than: page, available: true))
		let quota = RecordingQuotaRefresher()
		let c = DesktopRefreshCoordinator(host: ScriptedSnapshotRefresher(responses: [.snapshot(installed)]),
		                                  quotaRefresher: quota, snapshotStore: nil)
		await c.startInitialRefresh().value
		await quota.setSubscription(page.data.subscription)
		await c.refreshQuotaAlertsOnly(manual: true)
		XCTAssertEqual(c.latestSnapshot, installed)
		XCTAssertEqual(c.state, .ready(installed))
	}

	func testWidgetFilePublishesOnlyTheAllowedIdentityFields() async throws {
		let file = desktopFixtureURL("../../../internal/desktop/testdata/subscription-identity-v1.json").standardizedFileURL
		let sub = try JSONDecoder().decode(DesktopSubscriptionSnapshotV1.self, from: Data(contentsOf: file))
		let identified = try XCTUnwrap(sub.clients.first { $0.identity.value?.emailMasked.value != nil })
		let identity = try XCTUnwrap(identified.identity.value)
		let email = try XCTUnwrap(identity.emailMasked.value)
		let organization = try XCTUnwrap(identity.organizationName.value)
		let page = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json")).replacingSubscription(sub)
		let store = AppGroupSnapshotStore(directoryURL: storeDirectory())
		let c = DesktopRefreshCoordinator(host: ScriptedSnapshotRefresher(responses: [.snapshot(page)]), snapshotStore: store)
		await c.startInitialRefresh().value
		XCTAssertEqual(c.latestSnapshot?.data.subscription, sub)
		let bytes = try Data(contentsOf: store.snapshotURL)
		let object = try XCTUnwrap(JSONSerialization.jsonObject(with: bytes) as? [String: Any])
		let disk = try XCTUnwrap(object["subscription"] as? [String: Any])
		XCTAssertNil(disk["projection"])
		let clients = try XCTUnwrap(disk["clients"] as? [[String: Any]])
		XCTAssertEqual(clients.count, sub.clients.count)
		for client in clients {
			let name = try XCTUnwrap(client["client"] as? String)
			let input = try XCTUnwrap(sub.clients.first { $0.client == name })
			XCTAssertNil(client["failure_detail"])
			switch input.identity {
			case .absent:
				XCTAssertNil(client["identity"])
			case .null:
				XCTAssertTrue(client["identity"] is NSNull)
			case .value:
				let storedIdentity = try XCTUnwrap(client["identity"] as? [String: Any])
				for key in ["organization_kind", "organization_name", "plan_display", "observed_at",
					"attempted_at", "failure", "failure_at", "source", "unknown_reason"] {
					XCTAssertNil(storedIdentity[key], key)
				}
			}
		}
		let inputCodex = try XCTUnwrap(sub.clients.first { $0.client == "codex" })
		let storedCodex = try XCTUnwrap(clients.first { $0["client"] as? String == "codex" })
		XCTAssertEqual(storedCodex["plan"] as? String, try XCTUnwrap(inputCodex.plan))
		let codexIdentity = try XCTUnwrap(storedCodex["identity"] as? [String: Any])
		XCTAssertEqual(codexIdentity["quota_association"] as? String, "confirmed")
		XCTAssertEqual((storedCodex["plan_display"] as? [String: Any])?["short_name"] as? String, "Pro 100")
		let storedClient = try XCTUnwrap(clients.first { $0["client"] as? String == identified.client })
		let storedIdentity = try XCTUnwrap(storedClient["identity"] as? [String: Any])
		XCTAssertEqual(storedIdentity["email_masked"] as? String, email)
		XCTAssertFalse(String(decoding: bytes, as: UTF8.self).contains(organization))
	}

	func testNonBootstrapFullInstallsNewerQuota() async throws {
		let page = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let newer = try newerQuota(than: page, available: false)
		let quota = RecordingQuotaRefresher()
		await quota.setSubscription(newer)
		let c = DesktopRefreshCoordinator(host: ScriptedSnapshotRefresher(responses: [.snapshot(page)]), quotaRefresher: quota, snapshotStore: nil)
		await c.startInitialRefresh().value
		XCTAssertEqual(c.latestSnapshot, page.replacingSubscription(newer))
	}

	func testFirstServingInstallRejectsForeignSubscription() async throws {
		let page = try servingCoordinatorFixture(id: "first", full: true)
		let foreign = DesktopSubscriptionSnapshotV1(available: true, clients: [], projection: .value(.init(coreEpoch: 99, revision: 9, boot: "fixture", mono: 10)))
		let bad = DesktopServingSnapshotV2(header: page.header, payload: page.payload.replacingSubscription(foreign))
		let host = ControlledServingRefresher(restore: nil, partial: bad, final: bad)
		let c = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: widgetBatchNow)
		let task = c.startInitialRefresh()
		await host.waitUntilSuspended()
		host.resume()
		await task.value
		XCTAssertEqual(c.servingHeader, page.header)
		XCTAssertEqual(c.latestSnapshot, page.payload.replacingSubscription(.unavailable))
	}

	func testEqualStandaloneQuotaRetriesFailedWidgetWrite() async throws {
		let page = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let dir = storeDirectory()
		let permit = dir.appendingPathComponent("permit")
		let store = AppGroupSnapshotStore(directoryURL: dir, atomicReplace: { src, dest in
			guard FileManager.default.fileExists(atPath: permit.path) else { throw CacheReplacementError.failed }
			try FileManager.default.moveItem(at: src, to: dest)
		})
		let quota = RecordingQuotaRefresher()
		let c = DesktopRefreshCoordinator(host: ScriptedSnapshotRefresher(responses: [.snapshot(page)]), quotaRefresher: quota, snapshotStore: store)
		await c.startInitialRefresh().value
		guard case .failedBeforeCommit = c.widgetPublication else { return XCTFail("fixture must fail first write") }
		try Data().write(to: permit)
		await quota.setSubscription(page.data.subscription)
		await c.refreshQuotaAlertsOnly(manual: true)
		guard case .succeeded = c.widgetPublication else { return XCTFail("equal section did not repair Widget write") }
		XCTAssertEqual(try store.read().generatedAt, page.generatedAt)
		XCTAssertEqual(c.latestSnapshot, page)
	}

	func testReleaseDuringRunningFullOnlyPublishesChangedSection() async throws {
		let first = try servingCoordinatorFixture(id: "first", full: false)
		let full = try servingCoordinatorFixture(id: "held", full: true)
		let newer = try newerQuota(than: full.payload, available: false)
		let held = DesktopServingSnapshotV2(header: full.header, payload: full.payload.replacingSubscription(newer))
		let host = ControlledServingRefresher(restore: nil, partial: first, final: full)
		let reader = ScriptedWidgetBatchReader([.failure(.timedOut)])
		let store = AppGroupSnapshotStore(directoryURL: storeDirectory())
		let c = DesktopRefreshCoordinator(host: host, snapshotStore: store,
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow), wallNow: widgetBatchNow)
		let task = c.startInitialRefresh()
		await host.waitUntilSuspended()
		let deadline = ContinuousClock.now.advanced(by: .seconds(3))
		while (try? store.read()) == nil, ContinuousClock.now < deadline { await Task.yield() }
		XCTAssertTrue(try store.read().subscription.available)
		c.holdPublications()
		host.publish(held)
		// The publication callback queues its installation on MainActor.
		await Task { @MainActor in }.value
		XCTAssertEqual(c.servingHeader?.publicationID, "first")
		c.releasePublications()
		let written = ContinuousClock.now.advanced(by: .seconds(3))
		while (try? store.read().subscription.available) != false, ContinuousClock.now < written { await Task.yield() }
		XCTAssertFalse(try store.read().subscription.available)
		let asks = await reader.deliveryAsks
		XCTAssertTrue(asks.isEmpty, "held release before full success must not recollect session scope")
		host.resume()
		await task.value
	}

	func testHeldServingReleaseKeepsNewerStandaloneSubscription() async throws {
		let partial = try servingCoordinatorFixture(id: "early", full: false)
		let full = try servingCoordinatorFixture(id: "held", full: true)
		let host = ControlledServingRefresher(restore: nil, partial: partial, final: full)
		let newer = try newerQuota(than: full.payload, available: true)
		let quota = SuspendingQuotaRefresher(alerts: [], subscription: newer)
		let now = try XCTUnwrap(DesktopTimestamp.date(from: full.header.generatedAt))
		let coordinator = DesktopRefreshCoordinator(host: host, quotaRefresher: quota, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		await quota.waitUntilSuspended()
		for _ in 0..<100 where coordinator.latestSnapshot == nil { await Task.yield() }
		// Advance the serving baseline while the independent quota request
		// is suspended. It must publish before the full job returns.
		let intervening = try servingCoordinatorFixture(id: "intervening", full: false, sessionTotal: 91)
		host.publish(intervening)
		let installedBy = ContinuousClock.now.advanced(by: .seconds(2))
		while coordinator.servingHeader?.publicationID != "intervening", ContinuousClock.now < installedBy { await Task.yield() }
		coordinator.holdPublications()
		host.publish(full)
		await quota.resume()
		let mergedBy = ContinuousClock.now.advanced(by: .seconds(2))
		while coordinator.latestSnapshot?.data.subscription != newer, ContinuousClock.now < mergedBy { await Task.yield() }
		XCTAssertEqual(coordinator.latestSnapshot, intervening.payload.replacingSubscription(newer))
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "intervening")
		coordinator.releasePublications()
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "held")
		XCTAssertEqual(coordinator.latestSnapshot, full.payload.replacingSubscription(newer))
		XCTAssertEqual(coordinator.latestSnapshot?.data.sessions.available, true)
	}

	func testProjectionOrderingThroughServingInstallation() async throws {
		func publication(_ id: String, headerEpoch: Int64 = 1, keyEpoch: Int64 = 1, revision: Int64, mono: Int64,
		                 boot: String = "boot", legacy: Bool = false, nullProjection: Bool = false, available: Bool = true) throws -> DesktopServingSnapshotV2 {
			let page = try servingCoordinatorFixture(id: id, full: true, generationChanges: ["core_epoch": headerEpoch])
			let section = DesktopSubscriptionSnapshotV1(available: available, clients: [], projection: legacy ? .absent : nullProjection ? .null :
				.value(.init(coreEpoch: keyEpoch, revision: revision, boot: boot, mono: mono)))
			return .init(header: page.header, payload: page.payload.replacingSubscription(section))
		}
		let first = try publication("base", revision: 2, mono: 20)
		let nextEpoch = try publication("next", headerEpoch: 9, keyEpoch: 9, revision: 1, mono: 1)
		let host = ControlledServingRefresher(restore: nil, partial: first, final: nextEpoch)
		let now = try XCTUnwrap(DesktopTimestamp.date(from: first.header.generatedAt))
		let c = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let task = c.startInitialRefresh()
		await host.waitUntilSuspended()
		var expected = first.payload.data.subscription
		for (candidate, wins) in [
			(try publication("older revision", revision: 1, mono: 90), false),
			(try publication("equal", revision: 2, mono: 20, available: false), false),
			(try publication("smaller mono", revision: 2, mono: 19), false),
			(try publication("larger mono", revision: 2, mono: 21), true),
			(try publication("other boot same revision", revision: 2, mono: 90, boot: "other"), false),
			(try publication("larger revision", revision: 3, mono: 1), true),
			(try publication("legacy", revision: 0, mono: 0, legacy: true), false),
			(try publication("null", revision: 0, mono: 0, nullProjection: true), false),
			(try publication("foreign", keyEpoch: 9, revision: 99, mono: 99), false)
		] {
			host.publish(candidate)
			let deadline = ContinuousClock.now.advanced(by: .seconds(2))
			while c.servingHeader?.publicationID != candidate.header.publicationID, ContinuousClock.now < deadline { await Task.yield() }
			XCTAssertEqual(c.servingHeader?.publicationID, candidate.header.publicationID)
			if wins { expected = candidate.payload.data.subscription }
			XCTAssertEqual(c.latestSnapshot?.data.subscription, expected, candidate.header.publicationID)
		}
		host.resume()
		await task.value
		XCTAssertEqual(c.latestSnapshot, nextEpoch.payload)
	}

	func testLateBootstrapQuotaMergePublishesTheFinalWidgetSnapshot() async throws {
		let partial = try servingCoordinatorFixture(id: "quota-partial", full: false)
		let final = try servingCoordinatorFixture(id: "quota-full", full: true)
		let host = ControlledServingRefresher(restore: nil, partial: partial, final: final)
		let subscription = try newerQuota(than: final.payload, available: !final.payload.data.subscription.available)
		let quota = SuspendingQuotaRefresher(alerts: [], subscription: subscription)
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: directory) }
		let store = AppGroupSnapshotStore(directoryURL: directory)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, quotaRefresher: quota, snapshotStore: store, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		await quota.waitUntilSuspended()
		host.resume()
		let noSummary = AppGroupSessionScopeSummarySetV1.unavailable(AppGroupSessionScopeSummaryReason.summaryUnavailable)
		let baseline = AppGroupDesktopSnapshotV1(envelope: final.payload, sessionScopeSummary: noSummary)
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: .seconds(3))
		while (try? store.read()) != baseline, clock.now < deadline {
			try await Task.sleep(for: .milliseconds(10))
		}
		let fullPublishedBeforeQuota = (try? store.read()) == baseline
		await quota.resume()
		await initial.value
		XCTAssertTrue(fullPublishedBeforeQuota, "the fixture must finish full publication before quota")
		let updated = final.payload.replacingSubscription(subscription)
		XCTAssertNotEqual(updated, final.payload)
		XCTAssertEqual(coordinator.latestSnapshot, updated)
		XCTAssertTrue(try store.read() == AppGroupDesktopSnapshotV1(envelope: updated, sessionScopeSummary: noSummary), "Widget snapshot must contain the final quota merge without changing the full snapshot")
		if case .succeeded = coordinator.widgetPublication {} else { XCTFail("merged widget publication did not succeed") }
	}

	func testServingAcceptsSessionOriginSchemaAndRejectsUnknownFutureSchema() throws {
		let snapshot = try servingCoordinatorFixture(id: "session-origin-schema", full: true,
			generationChanges: ["core_schema": 35])
		XCTAssertEqual(snapshot.header.generation.coreSchema, 35)
		let supportedSchema = DesktopServingGenerationV2.maximumSupportedCoreSchema
		let current = try servingCoordinatorFixture(id: "current-schema", full: true,
			generationChanges: ["core_schema": supportedSchema])
		XCTAssertEqual(current.header.generation.coreSchema, supportedSchema)
		XCTAssertThrowsError(try servingCoordinatorFixture(id: "future-schema", full: true,
			generationChanges: ["core_schema": supportedSchema + 1]))
		XCTAssertTrue(DesktopServingGenerationV2.compatibleCoreSchema(stored: 31, current: 33))
		XCTAssertTrue(DesktopServingGenerationV2.compatibleCoreSchema(stored: 32, current: 33))
		XCTAssertTrue(DesktopServingGenerationV2.compatibleCoreSchema(stored: 33, current: 34))
		XCTAssertTrue(DesktopServingGenerationV2.compatibleCoreSchema(stored: 31, current: 34))
		XCTAssertFalse(DesktopServingGenerationV2.compatibleCoreSchema(stored: 33, current: 32))
		XCTAssertFalse(DesktopServingGenerationV2.compatibleCoreSchema(stored: 34, current: 33))
		XCTAssertTrue(DesktopServingGenerationV2.compatibleCoreSchema(stored: 34, current: 35))
		XCTAssertTrue(DesktopServingGenerationV2.compatibleCoreSchema(stored: 31, current: 35))
		XCTAssertFalse(DesktopServingGenerationV2.compatibleCoreSchema(stored: 30, current: 35))
		XCTAssertFalse(DesktopServingGenerationV2.compatibleCoreSchema(stored: 35, current: 34))
		XCTAssertTrue(DesktopServingGenerationV2.compatibleCoreSchema(stored: 35, current: 36))
		XCTAssertFalse(DesktopServingGenerationV2.compatibleCoreSchema(stored: 36, current: 35))
	}

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

	// Observation invalidates readers on every assignment; installing the same
	// publication again must not, or the open popover lays out again for nothing.
	func testReinstallingTheSamePublicationDoesNotInvalidateReaders() async throws {
		let published = try servingCoordinatorFixture(id: "same-full", full: true)
		let host = ControlledServingRefresher(restore: nil, partial: published, final: published)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		for _ in 0 ..< 40 where coordinator.latestSnapshot == nil { await Task.yield() }
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "same-full")
		final class Flag: @unchecked Sendable { var changed = false }
		let flag = Flag()
		withObservationTracking {
			_ = coordinator.latestSnapshot
			_ = coordinator.servingHeader
		} onChange: { flag.changed = true }
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.latestSnapshot, published.payload)
		XCTAssertFalse(flag.changed, "an equal publication must not invalidate its readers")
	}

	// While the surface draws its first frame after opening, a publication of
	// the same identity waits and is installed on release, not dropped.
	func testHeldPublicationIsInstalledOnRelease() async throws {
		let first = try servingCoordinatorFixture(id: "first-partial", full: false)
		let final = try servingCoordinatorFixture(id: "final-full", full: true)
		let host = ControlledServingRefresher(restore: nil, partial: first, final: final)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		for _ in 0 ..< 40 where coordinator.latestSnapshot == nil { await Task.yield() }
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "first-partial")
		coordinator.holdPublications()
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "first-partial", "a held publication is not shown yet")
		coordinator.holdPublications()
		coordinator.releasePublications()
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "first-partial", "another hold is still open")
		coordinator.releasePublications()
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "final-full")
		XCTAssertEqual(coordinator.latestSnapshot, final.payload)
		if case .ready(let shown) = coordinator.state { XCTAssertEqual(shown, final.payload) } else { XCTFail("released publication not ready") }
	}

	// An identity change is never held: the old identity must not stay visible.
	func testIdentityChangeIsNotHeld() async throws {
		let first = try servingCoordinatorFixture(id: "first-full", full: true)
		let other = try servingCoordinatorFixture(id: "other-epoch", full: true, generationChanges: ["core_epoch": 99])
		let host = ControlledServingRefresher(restore: nil, partial: first, final: other)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		for _ in 0 ..< 40 where coordinator.latestSnapshot == nil { await Task.yield() }
		coordinator.holdPublications()
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "other-epoch")
		coordinator.releasePublications()
	}

	// SP-SOP-R1-F1: a same-identity publication held before an identity change
	// must not come back on release.
	func testHeldPublicationIsDiscardedByAnIdentityChange() async throws {
		try await assertHeldPublicationDiscarded(generationChanges: ["core_epoch": 99], expected: "replacement")
	}

	func testHeldPublicationIsDiscardedByACoreSchemaChangeUnderNestedHolds() async throws {
		try await assertHeldPublicationDiscarded(generationChanges: ["core_schema": 30], nestedHolds: 2, expected: "replacement")
	}

	func testHeldPublicationIsDiscardedByAnUnusableIdentityChange() async throws {
		try await assertHeldPublicationDiscarded(generationChanges: ["state_identity": "other-state"],
			replacementDay: ("2026-08-12", "2026-08-13"), expected: nil)
	}

	private func assertHeldPublicationDiscarded(generationChanges: [String: Any], nestedHolds: Int = 1,
		replacementDay: (String, String) = ("2026-08-13", "2026-08-14"), expected: String?) async throws {
		let first = try servingCoordinatorFixture(id: "first-full", full: true)
		let held = try servingCoordinatorFixture(id: "held-full", full: true, generationChanges: ["usage_revision": 5])
		let replacement = try servingCoordinatorFixture(id: "replacement", full: true, generationChanges: generationChanges,
			day: replacementDay.0, nextDay: replacementDay.1)
		let host = ControlledServingRefresher(restore: nil, partial: first, final: replacement, publishesFinal: false)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		for _ in 0 ..< 40 where coordinator.latestSnapshot == nil { await Task.yield() }
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "first-full")
		for _ in 0 ..< nestedHolds { coordinator.holdPublications() }
		host.publish(held)
		for _ in 0 ..< 40 { await Task.yield() }
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "first-full", "same identity is held")
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.servingHeader?.publicationID, expected)
		for _ in 0 ..< nestedHolds { coordinator.releasePublications() }
		XCTAssertEqual(coordinator.servingHeader?.publicationID, expected, "release must not restore the old identity")
		if expected == nil { XCTAssertNil(coordinator.latestSnapshot) }
		else { XCTAssertEqual(coordinator.latestSnapshot, replacement.payload) }
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
		XCTAssertEqual(coordinator.latestSnapshot, next.replacingSubscription(previous.data.subscription))
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
		XCTAssertEqual(try store.read(), AppGroupDesktopSnapshotV1(
			envelope: complete,
			sessionScopeSummary: .unavailable(AppGroupSessionScopeSummaryReason.summaryUnavailable)
		))
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

		let freshSubscription = try newerQuota(than: complete, available: true)
		await quotaRefresher.setSubscription(freshSubscription)
		await coordinator.refreshQuotaAlertsOnly(manual: false)

		XCTAssertEqual(coordinator.latestSnapshot?.data.subscription, freshSubscription)
		XCTAssertEqual(coordinator.latestSnapshot?.data.usage, complete.data.usage, "the session/usage scan must not be rerun")
		XCTAssertEqual(coordinator.state, .ready(try XCTUnwrap(coordinator.latestSnapshot)))
		XCTAssertEqual(try store.read().subscription, AppGroupSubscriptionV1(freshSubscription), "the App Group projection widgets read must also carry the fresher figures")
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
		for _ in 0..<20 { await Task.yield() }
		host.resumeCall(1)
		for _ in 0..<200 where host.callCount < 2 { await Task.yield() }
		XCTAssertEqual(host.callCount, 2)
		guard host.callCount == 2 else {
			coordinator.stopAppWork()
			await initial.value
			await passiveJoin.value
			await providerFollowUp.value
			return
		}
		host.resumeCall(2)
		await initial.value
		await passiveJoin.value
		await providerFollowUp.value

		XCTAssertEqual(host.callCount, 2)
		XCTAssertEqual(coordinator.latestSnapshot, second.replacingSubscription(first.data.subscription))
	}

	func testExplicitRequestsDuringAnAutomaticRoundCoalesceIntoOneImmediateFollowUp() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		for trigger in [DesktopFullRefreshTrigger.manual, .providerSwitch] {
			let host = ControlledSnapshotRefresher(responses: [snapshot], failures: [1])
			let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
			let clock = RefreshTestClock(coordinator)
			var finished = 0
			let initial = Task {
				await coordinator.requestFullRefresh(trigger: .startup)
				finished += 1
			}
			await host.waitForCall(1)
			coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
			let requests = (0..<3).map { _ in
				Task {
					await coordinator.requestFullRefresh(trigger: trigger)
					finished += 1
				}
			}
			for _ in 0..<20 { await Task.yield() }
			XCTAssertEqual(host.callCount, 1, "an explicit action must not run a parallel scan")
			XCTAssertEqual(finished, 0)
			host.resumeCall(1)
			for _ in 0..<200 where host.callCount < 2 { await Task.yield() }
			XCTAssertEqual(host.callCount, 2, "\(trigger) must cause an immediate follow-up after the failure")
			XCTAssertNil(clock.scheduledSecond)
			if host.callCount == 2 {
				XCTAssertEqual(finished, 0, "callers await the immediate follow-up")
				host.resumeCall(2)
				for _ in 0..<200 where finished < 4 { await Task.yield() }
				XCTAssertEqual(finished, 4)
				XCTAssertEqual(host.callCount, 2)
				XCTAssertEqual(coordinator.latestSnapshot, snapshot)
			}
			coordinator.stopAppWork()
			host.resumeCall(3)
			await initial.value
			for request in requests { await request.value }
		}
	}

	func testManualThenProviderSwitchShareOneRoundThatStartsAfterTheSwitch() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let second = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		var switchCompleted = false
		var switchCompleteAtStart: [Bool] = []
		let host = ControlledSnapshotRefresher(responses: [first, second], onStart: {
			switchCompleteAtStart.append(switchCompleted)
		})
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let transport = CompletingRefreshTestSwitch { switchCompleted = true }
		let switcher = SwitchController(transport: transport, refreshCoordinator: coordinator)
		let target = ProviderSwitchTarget(client: "codex", provider: "provider-after-switch", credential: nil, viaWrapper: false)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)

		let manual = Task { await coordinator.requestFullRefresh(trigger: .manual) }
		for _ in 0..<20 { await Task.yield() }
		XCTAssertTrue(switcher.start(target))
		for _ in 0..<1000 where switcher.state != .succeeded(target) { await Task.yield() }
		XCTAssertEqual(switcher.state, .succeeded(target))
		XCTAssertEqual(switchCompleteAtStart, [false], "no round starts while the original scan is active")
		host.resumeCall(1)
		for _ in 0..<200 where host.callCount < 2 { await Task.yield() }
		XCTAssertEqual(switchCompleteAtStart, [false, true], "the single follow-up starts after the switch transport completes")
		if host.callCount == 2 {
			XCTAssertEqual(coordinator.latestSnapshot, first)
			host.resumeCall(2)
			await initial.value
			await manual.value
			XCTAssertEqual(coordinator.latestSnapshot, second.replacingSubscription(first.data.subscription))
			XCTAssertEqual(host.callCount, 2)
		}
		coordinator.stopAppWork()
		await initial.value
		await manual.value
	}

	func testProviderSwitchBypassesBackoffAndSourceCooldown() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		for first in [ScriptedSnapshotRefresher.Response.helperFailure(.nonZeroExit(1)), .snapshot(snapshot)] {
			let host = ScriptedSnapshotRefresher(responses: [first, .snapshot(snapshot)])
			let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
			let clock = RefreshTestClock(coordinator)
			await coordinator.requestFullRefresh(trigger: .startup)
			coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
			XCTAssertEqual(clock.scheduledSecond, 30)

			var completed = false
			let switched = Task {
				await coordinator.requestFullRefresh(trigger: .providerSwitch)
				completed = true
			}
			for _ in 0..<100 where !completed { await Task.yield() }
			XCTAssertTrue(completed, "the switch refresh finishes without advancing the automatic clock")
			XCTAssertEqual(host.callCount, 2)
			XCTAssertNil(clock.scheduledSecond)
			XCTAssertEqual(coordinator.latestSnapshot, snapshot)
			coordinator.stopAppWork()
			await switched.value
		}
	}

	func testSourceBurstDuringScanCreatesOneFollowUpAndRetainsPublishedValue() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let second = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		let host = ControlledSnapshotRefresher(responses: [first, second])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .sourceChange) }
		await host.waitForCall(1)
		for _ in 0..<8 { coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true) }
		host.resumeCall(1)
		await initial.value
		XCTAssertEqual(host.callCount, 1)
		let followUp = try XCTUnwrap(clock.advance(to: 30))
		await host.waitForCall(2)
		XCTAssertEqual(coordinator.latestSnapshot, first)
		host.resumeCall(2)
		await followUp.value
		XCTAssertEqual(host.callCount, 2)
		XCTAssertEqual(coordinator.latestSnapshot, second.replacingSubscription(first.data.subscription))
	}

	func testWakeDuringScanLeavesOneFollowUpMergedWithSourceEvents() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let second = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		let host = ControlledSnapshotRefresher(responses: [first, second])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)
		coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true)
		for _ in 0..<4 { coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true) }
		host.resumeCall(1)
		await initial.value
		XCTAssertEqual(host.callCount, 1)
		let followUp = try XCTUnwrap(clock.advance(to: 30))
		await host.waitForCall(2)
		XCTAssertEqual(host.callCount, 2, "no third update is queued behind the merged follow-up")
		host.resumeCall(2)
		await followUp.value
		XCTAssertEqual(host.callCount, 2, "the wake and the source burst share one follow-up")
		XCTAssertEqual(coordinator.latestSnapshot, second.replacingSubscription(first.data.subscription))
	}

	func testWakeAloneDuringScanStillGetsItsReconciliation() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let second = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		let host = ControlledSnapshotRefresher(responses: [first, second])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)
		let wake = Task { await coordinator.requestFullRefresh(trigger: .wake) }
		for _ in 0..<50 { await Task.yield() }
		host.resumeCall(1)
		// Bounded: a dropped wake leaves no second call, and must fail, not hang.
		let deadline = ContinuousClock.now + .seconds(2)
		while host.callCount < 2 && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
		XCTAssertEqual(host.callCount, 2, "a wake during a running update is not dropped")
		guard host.callCount == 2 else { return }
		host.resumeCall(2)
		await initial.value
		await wake.value
	}

	func testWakeDuringScanQueuesOneFollowUpWhilePeriodicAndStartupOnlyJoin() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		for sendsWake in [false, true] {
			let host = ControlledSnapshotRefresher(responses: [first, first])
			let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
			let clock = RefreshTestClock(coordinator)
			let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
			await host.waitForCall(1)
			if sendsWake {
				for _ in 0..<4 { coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true) }
			}
			var completed = 0
			let joiners = [DesktopFullRefreshTrigger.periodic, .startup].map { trigger in
				Task {
					await coordinator.requestFullRefresh(trigger: trigger)
					completed += 1
				}
			}
			for _ in 0..<20 { await Task.yield() }
			XCTAssertEqual(completed, 0)
			XCTAssertEqual(host.callCount, 1)
			host.resumeCall(1)
			if sendsWake {
				for _ in 0..<200 where host.callCount < 2 { await Task.yield() }
				XCTAssertEqual(host.callCount, 2)
				XCTAssertEqual(completed, 0, "joiners wait through the immediate wake follow-up")
				host.resumeCall(2)
			}
			for _ in 0..<200 where completed < 2 { await Task.yield() }
			XCTAssertEqual(completed, 2)
			XCTAssertEqual(host.callCount, sendsWake ? 2 : 1)
			XCTAssertNil(clock.advance(to: 30))
			coordinator.stopAppWork()
			host.resumeCall(2)
			await initial.value
			for joiner in joiners { await joiner.value }
		}
	}

	func testWakeFollowUpRemainsPendingThroughFailureBackoff() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [first], failures: [1])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)
		for _ in 0..<4 { coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true) }
		host.resumeCall(1)
		await initial.value
		XCTAssertEqual(clock.scheduledSecond, 30)
		XCTAssertNil(clock.advance(to: 29))
		XCTAssertEqual(host.callCount, 1)
		let followUp = try XCTUnwrap(clock.advance(to: 30))
		await host.waitForCall(2)
		host.resumeCall(2)
		await followUp.value
		XCTAssertEqual(host.callCount, 2)
		XCTAssertEqual(coordinator.latestSnapshot, first)
	}

	func testQuotaRequestsQueuedAfterTheQuotaPhaseWaitThroughTheImmediateFollowUp() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [first, first])
		let quota = SequencedQuotaRefresher()
		let coordinator = DesktopRefreshCoordinator(host: host, quotaRefresher: quota, snapshotStore: nil)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		// The legacy host reaches the scan only after the full round's quota phase.
		await host.waitForCall(1)
		var quotaReturned = false
		let manualQuota = Task {
			await coordinator.requestQuotaRefresh(manual: true)
			quotaReturned = true
		}
		coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true)
		for _ in 0..<20 { await Task.yield() }
		XCTAssertFalse(quotaReturned)
		host.resumeCall(1)
		await quota.waitForSecondOperation()
		let priorities = await quota.manualValues
		XCTAssertEqual(priorities, [false, true], "the automatic follow-up consumes the manual quota request")
		for _ in 0..<50 { await Task.yield() }
		XCTAssertFalse(quotaReturned, "the quota call must wait while the follow-up probe is held")
		XCTAssertEqual(host.callCount, 1)
		await quota.resumeSecondOperation()
		for _ in 0..<200 where host.callCount < 2 { await Task.yield() }
		XCTAssertEqual(host.callCount, 2)
		XCTAssertFalse(quotaReturned, "the quota call must wait for the follow-up scan's terminal")
		host.resumeCall(2)
		await initial.value
		await manualQuota.value
		XCTAssertTrue(quotaReturned)
		let finalPriorities = await quota.manualValues
		XCTAssertEqual(finalPriorities, [false, true], "no extra quota probe follows the sequence")
	}

	func testStopAppWorkDropsQueuedFollowUpSoQuitStartsNoNewUpdate() async throws {
		let first = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let second = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-empty-client.json"))
		let host = ControlledSnapshotRefresher(responses: [first, second])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)
		let source = Task { await coordinator.requestFullRefresh(trigger: .sourceChange) }
		for _ in 0..<50 { await Task.yield() }
		coordinator.stopAppWork()
		host.resumeCall(1)
		await initial.value
		await source.value
		for _ in 0..<50 { await Task.yield() }
		XCTAssertEqual(host.callCount, 1, "quit leaves no follow-up update behind")
	}

	func testAutomaticRefreshBackoffDoublesAndCapsAtFifteenMinutes() async throws {
		let delays = [30, 60, 120, 240, 480, 900, 900]
		let host = ScriptedSnapshotRefresher(responses: Array(repeating: .helperFailure(.nonZeroExit(1)), count: delays.count + 1))
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		await coordinator.requestFullRefresh(trigger: .startup)
		var time = 0
		for (index, delay) in delays.enumerated() {
			XCTAssertNil(clock.scheduledSecond, "a failure alone does not enqueue a scan")
			for _ in 0..<6 { coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true) }
			XCTAssertEqual(clock.scheduledSecond, time + delay)
			XCTAssertEqual(clock.scheduledCount, index + 1, "a trigger burst owns only one timer")
			XCTAssertNil(clock.advance(to: time + delay - 1))
			XCTAssertEqual(host.callCount, index + 1)
			time += delay
			let refresh = try XCTUnwrap(clock.advance(to: time))
			await refresh.value
			XCTAssertEqual(host.callCount, index + 2)
		}
	}

	func testSuccessfulRefreshRestartsBackoffAtThirtySeconds() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ScriptedSnapshotRefresher(responses: [
			.helperFailure(.nonZeroExit(1)), .helperFailure(.nonZeroExit(1)), .snapshot(snapshot),
			.helperFailure(.nonZeroExit(1)), .snapshot(snapshot),
		])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		await coordinator.requestFullRefresh(trigger: .startup)
		for time in [30, 90] {
			coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true)
			let refresh = try XCTUnwrap(clock.advance(to: time))
			await refresh.value
		}
		XCTAssertEqual(coordinator.latestSnapshot, snapshot)
		await coordinator.requestFullRefresh(trigger: .periodic)
		XCTAssertEqual(host.callCount, 4, "success clears the previous retry deadline")
		coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true)
		XCTAssertEqual(clock.scheduledSecond, 120, "the next failure starts a new streak")
		let refresh = try XCTUnwrap(clock.advance(to: 120))
		await refresh.value
		XCTAssertEqual(host.callCount, 5)
	}

	func testContinuousSourceWritesRefreshAtFixedIntervalsAndDeliverTheLastWrite() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ScriptedSnapshotRefresher(responses: Array(repeating: .snapshot(snapshot), count: 4))
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		await coordinator.requestFullRefresh(trigger: .sourceChange)
		for start in [0, 30, 60] {
			for event in [1, 10, 29] {
				XCTAssertNil(clock.advance(to: start + event))
				coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
				XCTAssertEqual(clock.scheduledSecond, start + 30)
				XCTAssertEqual(host.callCount, start / 30 + 1)
			}
			let refresh = try XCTUnwrap(clock.advance(to: start + 30))
			await refresh.value
			XCTAssertEqual(host.callCount, start / 30 + 2)
		}
		XCTAssertEqual(clock.scheduledCount, 3)
		XCTAssertNil(clock.advance(to: 120))
		XCTAssertEqual(host.callCount, 4)
	}

	func testSourceCooldownAndFailureBackoffUseTheLaterDeadline() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [snapshot, snapshot], failures: [1])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .sourceChange) }
		await host.waitForCall(1)
		XCTAssertNil(clock.advance(to: 10))
		coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
		host.resumeCall(1)
		await initial.value

		XCTAssertEqual(clock.scheduledSecond, 40, "cooldown starts at 0, backoff starts at the failed terminal at 10")
		XCTAssertNil(clock.advance(to: 30))
		XCTAssertEqual(host.callCount, 1)
		let refresh = try XCTUnwrap(clock.advance(to: 40))
		await host.waitForCall(2)
		host.resumeCall(2)
		await refresh.value
		XCTAssertEqual(host.callCount, 2)
	}

	func testContinuousSourceWritesDoNotMoveTheFailureRetryDeadline() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ScriptedSnapshotRefresher(responses: [
			.helperFailure(.nonZeroExit(1)), .helperFailure(.nonZeroExit(1)), .snapshot(snapshot),
		])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		await coordinator.requestFullRefresh(trigger: .startup)
		for (start, next, calls) in [(0, 30, 1), (30, 90, 2)] {
			for second in (start + 1)..<next {
				XCTAssertNil(clock.advance(to: second))
				coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
				XCTAssertEqual(clock.scheduledSecond, next)
				XCTAssertEqual(host.callCount, calls)
			}
			let refresh = try XCTUnwrap(clock.advance(to: next))
			await refresh.value
			XCTAssertEqual(host.callCount, calls + 1)
		}
		XCTAssertEqual(clock.scheduledCount, 2)
		XCTAssertNil(clock.advance(to: 120))
		XCTAssertEqual(coordinator.latestSnapshot, snapshot)
	}

	func testBootstrapFailureBacksOffCancellationPreservesBackoffAndSuccessResetsIt() async throws {
		let snapshot = try servingCoordinatorFixture(id: "bootstrap-final", full: true)
		let host = ScriptedBootstrapRefresher(responses: [
			.failure(HelperExecutionError.nonZeroExit(1)), .failure(HelperExecutionError.cancelled),
			.failure(CancellationError()), .success(snapshot), .failure(HelperExecutionError.nonZeroExit(1)),
		])
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now })
		let clock = RefreshTestClock(coordinator)
		await coordinator.requestFullRefresh(trigger: .startup)
		guard case .failed = coordinator.fullAttempt else { return XCTFail("failed bootstrap must publish failure") }
		XCTAssertNil(clock.advance(to: 10))
		await coordinator.requestFullRefresh(trigger: .manual)
		await coordinator.requestFullRefresh(trigger: .manual)
		coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true)
		XCTAssertEqual(clock.scheduledSecond, 30, "neither form of cancellation changes the existing deadline")
		XCTAssertEqual(host.bootstrapCalls, 3)
		let successful = try XCTUnwrap(clock.advance(to: 30))
		await successful.value
		guard case .succeeded = coordinator.fullAttempt else { return XCTFail("complete bootstrap must publish success") }
		XCTAssertEqual(coordinator.servingHeader, snapshot.header)

		await coordinator.requestFullRefresh(trigger: .manual)
		XCTAssertEqual(host.bootstrapCalls, 5)
		coordinator.enqueueAutomaticRefresh(trigger: .wake, fromWatcher: true)
		XCTAssertEqual(clock.scheduledSecond, 60, "the failure after success starts a fresh 30-second backoff")
		coordinator.stopAppWork()
	}

	func testManualRefreshBypassesBothDeadlines() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [snapshot, snapshot, snapshot], failures: [1])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .sourceChange) }
		await host.waitForCall(1)
		host.resumeCall(1)
		await initial.value
		coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
		XCTAssertEqual(clock.scheduledSecond, 30)

		let manual = Task { await coordinator.requestFullRefresh(trigger: .manual) }
		for _ in 0..<100 where host.callCount < 2 { await Task.yield() }
		XCTAssertEqual(host.callCount, 2, "manual scan starts before either deadline")
		guard host.callCount == 2 else {
			coordinator.stopAppWork()
			await manual.value
			return
		}
		XCTAssertNil(clock.scheduledSecond)
		host.resumeCall(2)
		await manual.value
		XCTAssertEqual(host.callCount, 2)
	}

	func testStopAppWorkCancelsTimerAndRejectsLateCallbacksAndRequests() async throws {
		let host = ScriptedSnapshotRefresher(responses: [.helperFailure(.nonZeroExit(1))])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		await coordinator.requestFullRefresh(trigger: .startup)
		coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
		let staleCallback = try XCTUnwrap(clock.pendingFire)
		coordinator.stopAppWork()

		XCTAssertEqual(clock.cancelledCount, 1)
		XCTAssertNil(clock.scheduledSecond)
		XCTAssertNil(clock.advance(to: 30))
		XCTAssertNil(staleCallback(), "a callback already dispatched before cancellation is harmless")
		coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
		await coordinator.requestFullRefresh(trigger: .manual)
		XCTAssertEqual(host.callCount, 1)
	}

	func testStoppingBackgroundWatchCancelsItsTimerAndManualRefreshStaysAvailable() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ScriptedSnapshotRefresher(responses: [.snapshot(snapshot), .snapshot(snapshot)])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		await coordinator.requestFullRefresh(trigger: .startup)
		coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
		XCTAssertEqual(clock.scheduledSecond, 30)
		coordinator.cancelBackgroundRefresh()
		XCTAssertEqual(clock.cancelledCount, 1)
		XCTAssertNil(clock.advance(to: 30))
		XCTAssertEqual(host.callCount, 1)
		await coordinator.requestFullRefresh(trigger: .manual)
		XCTAssertEqual(host.callCount, 2)
	}

	func testSettingsRefreshesBypassDeadlinesAndKeepPeriodicQuotaPriority() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		for requiresFollowUp in [false, true] {
			let host = ScriptedSnapshotRefresher(responses: [.helperFailure(.nonZeroExit(1)), .snapshot(snapshot)])
			let quota = RecordingQuotaRefresher()
			let coordinator = DesktopRefreshCoordinator(host: host, quotaRefresher: quota, snapshotStore: nil)
			let clock = RefreshTestClock(coordinator)
			await coordinator.requestFullRefresh(trigger: .startup)
			coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
			XCTAssertEqual(clock.scheduledSecond, 30)

			var completed = false
			let save = Task {
				await coordinator.requestSettingsRefresh(requiresFollowUp: requiresFollowUp)
				completed = true
			}
			for _ in 0..<1000 where !completed { await Task.yield() }
			XCTAssertTrue(completed, "an explicit settings save does not wait for a timer")
			XCTAssertEqual(host.callCount, 2)
			XCTAssertNil(clock.scheduledSecond)
			let quotaCalls = await quota.recordedManualValues()
			XCTAssertEqual(quotaCalls, [false, false])
			coordinator.stopAppWork()
			await save.value
		}
	}

	func testQuotaSettingsRefreshJoinsAnActiveRoundWithoutAddingFollowUp() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [snapshot, snapshot])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)
		var completed = false
		let save = Task {
			await coordinator.requestSettingsRefresh(requiresFollowUp: false)
			completed = true
		}
		for _ in 0..<20 { await Task.yield() }
		XCTAssertFalse(completed)
		host.resumeCall(1)
		for _ in 0..<200 where !completed { await Task.yield() }
		XCTAssertTrue(completed)
		XCTAssertEqual(host.callCount, 1)
		coordinator.stopAppWork()
		host.resumeCall(2)
		await initial.value
		await save.value
	}

	func testSettingsSavesWaitForOneImmediateFollowUpAfterAnActiveRoundFails() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [snapshot, snapshot], failures: [1])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		let initial = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)
		coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
		var completed = 0
		let saves = (0..<2).map { _ in
			Task {
				await coordinator.requestSettingsRefresh(requiresFollowUp: true)
				completed += 1
			}
		}
		for _ in 0..<20 { await Task.yield() }
		XCTAssertEqual(completed, 0)
		host.resumeCall(1)
		for _ in 0..<200 where host.callCount < 2 { await Task.yield() }
		XCTAssertEqual(host.callCount, 2, "settings reconcile immediately after the first round fails")
		XCTAssertEqual(completed, 0, "settings cannot use the first round's terminal")
		XCTAssertNil(clock.scheduledSecond)
		if host.callCount == 2 {
			host.resumeCall(2)
			await initial.value
			for save in saves { await save.value }
			XCTAssertEqual(completed, 2)
			XCTAssertEqual(coordinator.latestSnapshot, snapshot)
			XCTAssertEqual(host.callCount, 2)
		}
		coordinator.stopAppWork()
		await initial.value
		for save in saves { await save.value }
	}

	func testCancellingDeferredWaiterRestoresWatcherOnlyCancellation() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		for cancelBeforeDisabling in [true, false] {
			let host = ScriptedSnapshotRefresher(responses: [.snapshot(snapshot)])
			let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
			let clock = RefreshTestClock(coordinator)
			await coordinator.requestFullRefresh(trigger: .startup)
			var completed = false
			let waiter = Task {
				await coordinator.requestFullRefresh(trigger: .sourceChange)
				completed = true
			}
			for _ in 0..<100 where clock.scheduledSecond == nil { await Task.yield() }
			XCTAssertEqual(clock.scheduledSecond, 30)
			XCTAssertFalse(completed)
			coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: true)
			if !cancelBeforeDisabling { coordinator.cancelBackgroundRefresh() }
			waiter.cancel()
			for _ in 0..<100 where !completed { await Task.yield() }
			XCTAssertTrue(completed, "cancelled callers return without advancing the clock")
			if cancelBeforeDisabling {
				XCTAssertEqual(clock.scheduledSecond, 30, "the watcher still owns pending work")
				coordinator.cancelBackgroundRefresh()
			}
			XCTAssertEqual(clock.cancelledCount, 1, "the cancelled caller no longer keeps watcher work alive")
			XCTAssertNil(clock.advance(to: 30))
			XCTAssertEqual(host.callCount, 1)
			coordinator.stopAppWork()
			await waiter.value
		}
	}

	func testCancellingLastWaiterClearsTheTimerAndDoesNotAffectIndependentNotifications() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		for independentNotification in [false, true] {
			let host = ScriptedSnapshotRefresher(responses: [.snapshot(snapshot), .snapshot(snapshot)])
			let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
			let clock = RefreshTestClock(coordinator)
			await coordinator.requestFullRefresh(trigger: .startup)
			var completed = false
			let waiter = Task {
				await coordinator.requestFullRefresh(trigger: .sourceChange)
				completed = true
			}
			for _ in 0..<100 where clock.scheduledSecond == nil { await Task.yield() }
			XCTAssertEqual(clock.scheduledSecond, 30)
			if independentNotification {
				coordinator.enqueueAutomaticRefresh(trigger: .sourceChange, fromWatcher: false)
			}
			waiter.cancel()
			for _ in 0..<100 where !completed { await Task.yield() }
			XCTAssertTrue(completed)
			coordinator.cancelBackgroundRefresh()
			XCTAssertEqual(clock.scheduledSecond, independentNotification ? 30 : nil)
			let refresh = clock.advance(to: 30)
			await refresh?.value
			XCTAssertEqual(host.callCount, independentNotification ? 2 : 1)
			coordinator.stopAppWork()
			await waiter.value
		}
	}

	func testCancellingAnActiveJoinerDoesNotCancelTheAutomaticRound() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [snapshot])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let automatic = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)
		var completed = false
		let periodic = Task {
			await coordinator.requestFullRefresh(trigger: .periodic)
			completed = true
		}
		for _ in 0..<20 { await Task.yield() }
		XCTAssertFalse(completed)
		periodic.cancel()
		for _ in 0..<100 where !completed { await Task.yield() }
		XCTAssertTrue(completed, "only the caller's wait is cancelled")
		XCTAssertNil(coordinator.latestSnapshot)
		host.resumeCall(1)
		await automatic.value
		await periodic.value
		XCTAssertEqual(host.callCount, 1)
		XCTAssertEqual(coordinator.latestSnapshot, snapshot)
	}

	func testManualRefreshWakesDeferredWaitersAndUsesLargestRecentLimit() async throws {
		let snapshot = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let host = ControlledSnapshotRefresher(responses: [snapshot], failures: [1])
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil)
		let clock = RefreshTestClock(coordinator)
		let startup = Task { await coordinator.requestFullRefresh(trigger: .startup) }
		await host.waitForCall(1)
		host.resumeCall(1)
		await startup.value
		var completed = Set<Int>()
		let waiters = [4, 17].map { recentLimit in
			Task {
				await coordinator.requestFullRefresh(trigger: .sourceChange, recentLimit: recentLimit)
				completed.insert(recentLimit)
			}
		}
		for _ in 0..<20 { await Task.yield() }
		XCTAssertEqual(clock.scheduledSecond, 30)
		let manual = Task { await coordinator.requestFullRefresh(trigger: .manual, recentLimit: 8) }
		for _ in 0..<100 where host.callCount < 2 { await Task.yield() }
		XCTAssertEqual(host.callCount, 2)
		guard host.callCount == 2 else {
			coordinator.stopAppWork()
			await manual.value
			for waiter in waiters { _ = await waiter.value }
			return
		}
		XCTAssertTrue(completed.isEmpty)
		XCTAssertNil(clock.scheduledSecond)
		XCTAssertEqual(host.recentLimits.last, 17)
		host.resumeCall(2)
		await manual.value
		for waiter in waiters { _ = await waiter.value }
		XCTAssertEqual(completed, [4, 17])
		XCTAssertEqual(host.callCount, 2)
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
		XCTAssertEqual(try store.read(), AppGroupDesktopSnapshotV1(
			envelope: complete,
			sessionScopeSummary: .unavailable(AppGroupSessionScopeSummaryReason.summaryUnavailable)
		))
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
		XCTAssertEqual(try store.read(), AppGroupDesktopSnapshotV1(
			envelope: complete,
			sessionScopeSummary: .unavailable(AppGroupSessionScopeSummaryReason.summaryUnavailable)
		))
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
		let subscription = try newerQuota(than: complete, available: true)
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
		XCTAssertEqual(try store.read(), AppGroupDesktopSnapshotV1(
			envelope: complete,
			sessionScopeSummary: .unavailable(AppGroupSessionScopeSummaryReason.summaryUnavailable)
		))
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

private func newerQuota(than envelope: DesktopWireEnvelopeV1, available: Bool) throws -> DesktopSubscriptionSnapshotV1 {
	let p = try XCTUnwrap(envelope.data.subscription.projection.value)
	return .init(available: available, clients: [], projection: .value(.init(coreEpoch: p.coreEpoch, revision: p.revision, boot: p.boot, mono: p.mono + 1)))
}

private func servingCoordinatorFixture(id: String, full: Bool, generationChanges: [String: Any] = [:],
	day: String = "2026-08-13", nextDay: String = "2026-08-14", sessionTotal: Int? = nil) throws -> DesktopServingSnapshotV2 {
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
		if let sessionTotal { sessions["total"] = sessionTotal }
		data["sessions"] = sessions
		payload["data"] = data
	}
	let coverage: [String: Any] = ["state": full ? "complete" : "partial", "inventory_id": "fixture", "observed_at": generatedAt, "sources": 2, "committed": full ? 2 : 1]
	var generation: [String: Any] = ["state_identity": "fixture-state", "core_epoch": 1, "session_epoch": 2, "core_schema": 31, "session_schema": 1, "usage_revision": 3, "session_revision": 4]
	generation.merge(generationChanges) { _, new in new }
	// A valid serving fixture carries the generation of its actual header.
	if var data = payload["data"] as? [String: Any], var sub = data["subscription"] as? [String: Any], var p = sub["projection"] as? [String: Any] {
		p["core_epoch"] = generation["core_epoch"]
		sub["projection"] = p; data["subscription"] = sub; payload["data"] = data
	}
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
	private let subscription: DesktopSubscriptionSnapshotV1?

	init(alerts: [DesktopQuotaAlertV1], subscription: DesktopSubscriptionSnapshotV1? = nil) {
		self.alerts = alerts
		self.subscription = subscription
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
		return subscription
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
private final class RefreshTestClock {
	private let epoch = ContinuousClock.now
	private var second = 0
	private var pending: (deadline: ContinuousClock.Instant, fire: DesktopRefreshClock.Fire)?
	private(set) var scheduledCount = 0
	private(set) var cancelledCount = 0

	var pendingFire: DesktopRefreshClock.Fire? { pending?.fire }

	var scheduledSecond: Int? {
		pending.map { Int(epoch.duration(to: $0.deadline).components.seconds) }
	}

	init(_ coordinator: DesktopRefreshCoordinator) {
		coordinator.refreshClock = DesktopRefreshClock(
			now: { [self] in epoch.advanced(by: .seconds(second)) },
			schedule: { [self] deadline, fire in
				XCTAssertNil(pending, "only one timer may be pending")
				scheduledCount += 1
				pending = (deadline, fire)
				return {
					self.cancelledCount += 1
					self.pending = nil
				}
			}
		)
	}

	func advance(to second: Int) -> Task<Void, Never>? {
		XCTAssertGreaterThanOrEqual(second, self.second)
		self.second = second
		guard let pending, pending.deadline <= epoch.advanced(by: .seconds(second)) else { return nil }
		self.pending = nil
		return pending.fire()
	}
}

@MainActor
private final class ControlledSnapshotRefresher: DesktopSnapshotRefreshing {
	private var responses: [DesktopWireEnvelopeV1]
	private let failures: Set<Int>
	private let onStart: @MainActor () -> Void
	private var continuations = [Int: CheckedContinuation<Void, Never>]()
	private(set) var callCount = 0
	private(set) var recentLimits: [Int] = []

	init(responses: [DesktopWireEnvelopeV1], failures: Set<Int> = [], onStart: @escaping @MainActor () -> Void = {}) {
		self.responses = responses
		self.failures = failures
		self.onStart = onStart
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		try await refresh(recentLimit: recentLimit, progress: { _ in })
	}

	func refresh(
		recentLimit: Int,
		progress: @escaping @Sendable (DesktopScanProgress) -> Void
	) async throws -> DesktopWireEnvelopeV1 {
		callCount += 1
		onStart()
		recentLimits.append(recentLimit)
		let call = callCount
		await withCheckedContinuation { continuations[call] = $0 }
		if failures.contains(call) { throw HelperExecutionError.nonZeroExit(1) }
		return responses.removeFirst()
	}

	func waitForCall(_ expected: Int) async {
		while callCount < expected || continuations[expected] == nil { await Task.yield() }
	}

	func resumeCall(_ call: Int) {
		continuations.removeValue(forKey: call)?.resume()
	}
}

private struct CompletingRefreshTestSwitch: ProviderSwitching {
	let complete: @MainActor @Sendable () -> Void

	func switchProvider(_ target: ProviderSwitchTarget) async -> ProviderSwitchTransportOutcome {
		await complete()
		return .succeeded
	}
}

@MainActor
private final class ScriptedBootstrapRefresher: DesktopSnapshotRefreshing, DesktopSnapshotBootstrapping {
	private var responses: [Result<DesktopServingSnapshotV2, any Error>]
	private(set) var bootstrapCalls = 0

	init(responses: [Result<DesktopServingSnapshotV2, any Error>]) {
		self.responses = responses
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		XCTFail("expected bootstrap instead of the legacy helper")
		throw HelperExecutionError.malformedOutput
	}

	func readServingSnapshot() async throws -> DesktopServingSnapshotV2 {
		throw HelperExecutionError.malformedOutput
	}

	func bootstrapServingSnapshot(progress: @escaping @Sendable (DesktopScanProgress) -> Void,
		publication: @escaping @Sendable (DesktopServingSnapshotV2) -> Void) async throws -> DesktopServingSnapshotV2? {
		bootstrapCalls += 1
		return try responses.removeFirst().get()
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
	private(set) var callCount = 0

	fileprivate init(responses: [Response]) {
		self.responses = responses
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		callCount += 1
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

// Startup order (architecture): the startup refresh, and with it bootstrap and
// quota, waits for the restore, but never longer than the fallback.
extension DesktopRefreshCoordinatorTests {
	func testStartupRefreshWaitsForTheRestore() async throws {
		let saved = try servingCoordinatorFixture(id: "saved-full", full: true)
		let host = HeldRestoreRefresher(restore: saved)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now },
			restoreFirstFallback: .seconds(60))
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilRestoreHeld()
		for _ in 0 ..< 100 { await Task.yield() }
		XCTAssertEqual(host.events, ["restore"], "the startup refresh started before the restore finished")
		host.releaseRestore()
		await initial.value
		XCTAssertEqual(host.events, ["restore", "restored", "bootstrap"])
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "saved-full")
	}

	func testStartupRefreshStartsAfterTheFallbackWhenTheRestoreHangs() async throws {
		let saved = try servingCoordinatorFixture(id: "saved-full", full: true)
		let host = HeldRestoreRefresher(restore: saved)
		let now = ISO8601DateFormatter().date(from: "2026-08-13T10:00:00Z")!
		let coordinator = DesktopRefreshCoordinator(host: host, snapshotStore: nil, wallNow: { now },
			restoreFirstFallback: .milliseconds(50))
		await coordinator.startInitialRefresh().value
		XCTAssertEqual(host.events, ["restore", "bootstrap"], "a hung restore must not postpone the refresh indefinitely")
		host.releaseRestore()
	}
}

@MainActor
private final class HeldRestoreRefresher: DesktopSnapshotRefreshing, DesktopSnapshotBootstrapping {
	private let restore: DesktopServingSnapshotV2
	private var held: CheckedContinuation<Void, Never>?
	private var released = false
	private(set) var events = [String]()
	init(restore: DesktopServingSnapshotV2) { self.restore = restore }
	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 { throw HelperExecutionError.malformedOutput }
	func readServingSnapshot() async throws -> DesktopServingSnapshotV2 {
		events.append("restore")
		if !released { await withCheckedContinuation { held = $0 } }
		events.append("restored")
		return restore
	}
	func bootstrapServingSnapshot(progress: @escaping @Sendable (DesktopScanProgress) -> Void,
		publication: @escaping @Sendable (DesktopServingSnapshotV2) -> Void) async throws -> DesktopServingSnapshotV2? {
		events.append("bootstrap")
		throw HelperExecutionError.nonZeroExit(1)
	}
	func waitUntilRestoreHeld() async { while held == nil { await Task.yield() } }
	func releaseRestore() { released = true; held?.resume(); held = nil }
}

// MARK: Task 7 — the coordinator publishes the Widget session-scope batch

/// Fails every read: a helper that cannot answer.
private struct FailingWidgetScopeReader: WidgetSessionScopeReading {
	func widgetSessionScopeView(_ selection: DesktopSessionScopeSelectionV1, generationID: String?, asOf: String) async throws -> DesktopSessionScopeViewV1 {
		throw HelperExecutionError.timedOut
	}
}

private extension XCTestCase {
	@MainActor func storeDirectory() -> URL {
		let url = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		addTeardownBlock { try? FileManager.default.removeItem(at: url) }
		return url
	}
}

@MainActor
final class DesktopRefreshCoordinatorWidgetSummaryTests: XCTestCase {
	/// The Widget batch is asked of the bootstrap's last publication even while
	/// that publication is held behind an opening surface, and again once it is
	/// released: neither publish has to start a read for another publication.
	func testAHeldFinalPublicationStillAsksForItsDeliveredBatch() async throws {
		let first = try servingCoordinatorFixture(id: "first-partial", full: false)
		let final = try servingCoordinatorFixture(id: "final-full", full: true)
		let host = ControlledServingRefresher(restore: nil, partial: first, final: final)
		let reader = ScriptedWidgetBatchReader([.failure(.timedOut)])
		let coordinator = DesktopRefreshCoordinator(
			host: host, snapshotStore: AppGroupSnapshotStore(directoryURL: storeDirectory()),
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow),
			wallNow: widgetBatchNow
		)
		let initial = coordinator.startInitialRefresh()
		await host.waitUntilSuspended()
		for _ in 0 ..< 40 where coordinator.latestSnapshot == nil { await Task.yield() }
		coordinator.holdPublications()
		host.resume()
		await initial.value
		XCTAssertEqual(coordinator.servingHeader?.publicationID, "first-partial")
		var asks = await reader.deliveryAsks
		XCTAssertEqual(asks, ["final-full"])
		coordinator.releasePublications()
		for _ in 0 ..< 200 {
			asks = await reader.deliveryAsks
			if asks.count == 2 { break }
			try await Task.sleep(for: .milliseconds(10))
		}
		XCTAssertEqual(asks, ["final-full", "final-full"])
	}

	func testSuccessfulRefreshPublishesTheConfirmedDefaultsBatch() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let fixture = try WidgetBatchFixture.load()
		let store = AppGroupSnapshotStore(directoryURL: storeDirectory())
		// The publication reports a confirmed Work default; Go answers the
		// Work read with the saved Auxiliary default, so the batch is redone
		// for Auxiliary rather than published under a stale scope.
		let reader = ScriptedWidgetScopeReader(views: try XCTUnwrap(fixture.batches["auxiliary"]) + [fixture.popover])
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			snapshotStore: store,
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: reader, now: widgetBatchNow)
		)
		await coordinator.refresh()

		let summary = try XCTUnwrap(try store.read().sessionScopeSummary)
		XCTAssertFalse(summary.stale)
		XCTAssertEqual(summary.items.count, 9)
		XCTAssertEqual(Set(summary.items.map(\.scope)), [.auxiliary])
		let reads = await reader.reads
		XCTAssertEqual(reads.first?.selection.scope, .work, "the first attempt uses the publication's confirmed default")
		XCTAssertEqual(Set(reads.dropFirst().map(\.selection.scope)), [.auxiliary])
	}

	func testFailedBatchKeepsThePreviousAcceptedNumbersWholeAndStale() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let fixture = try WidgetBatchFixture.load()
		let directory = storeDirectory()
		let store = AppGroupSnapshotStore(directoryURL: directory)
		let first = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			snapshotStore: store,
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(
				reader: ScriptedWidgetScopeReader(views: try XCTUnwrap(fixture.batches["auxiliary"]) + [fixture.popover]), now: widgetBatchNow
			)
		)
		await first.refresh()
		let accepted = try XCTUnwrap(try store.read().sessionScopeSummary)

		// A later App process (or refresh) whose batch fails: the accepted
		// numbers, scope and time stay together and are marked stale.
		let second = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			snapshotStore: AppGroupSnapshotStore(directoryURL: directory),
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: FailingWidgetScopeReader(), now: widgetBatchNow)
		)
		await second.refresh()
		let retained = try XCTUnwrap(try store.read().sessionScopeSummary)
		XCTAssertTrue(retained.stale)
		XCTAssertEqual(retained.reason, AppGroupSessionScopeSummaryReason.selectionUnavailable)
		XCTAssertEqual(retained.items, accepted.items, "no relabelled or partially replaced figures")
	}

	func testWithoutAnAcceptedBatchTheSummaryIsUnavailableNotZero() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let store = AppGroupSnapshotStore(directoryURL: storeDirectory())
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			snapshotStore: store,
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: FailingWidgetScopeReader(), now: widgetBatchNow)
		)
		await coordinator.refresh()
		XCTAssertEqual(try store.read().sessionScopeSummary, .unavailable(AppGroupSessionScopeSummaryReason.selectionUnavailable))
	}
}

// MARK: ASC-WS-R2-F1 — one malformed item never clears the other eight

extension DesktopRefreshCoordinatorWidgetSummaryTests {
	/// Writes the producer's accepted nine-item Auxiliary set, then removes
	/// `unknown_count` from the items `malformed` selects, in the file itself.
	private func nineItemFile(
		_ directory: URL, envelope: DesktopWireEnvelopeV1, malformed: (String, String) -> Bool
	) async throws -> AppGroupSessionScopeSummarySetV1 {
		let fixture = try WidgetBatchFixture.load()
		let views = try XCTUnwrap(fixture.batches["auxiliary"])
		guard case let .accepted(accepted) = await WidgetSessionScopeBatchCollector(
			reader: ScriptedWidgetScopeReader(views: views), now: widgetBatchNow
		).collect(default: views[0].default) else {
			XCTFail("the producer's batch was rejected")
			throw CocoaError(.fileReadCorruptFile)
		}
		XCTAssertEqual(accepted.items.count, 9)
		try AppGroupSnapshotStore(directoryURL: directory).write(AppGroupDesktopSnapshotV1(envelope: envelope, sessionScopeSummary: accepted))
		let path = directory.appendingPathComponent(AppGroupSnapshotStore.fileName)
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: path)) as? [String: Any])
		var set = try XCTUnwrap(object["session_scope_summary"] as? [String: Any])
		var items = try XCTUnwrap(set["items"] as? [[String: Any]])
		for index in items.indices where malformed(items[index]["client"] as? String ?? "", items[index]["period"] as? String ?? "") {
			items[index].removeValue(forKey: "unknown_count")
		}
		set["items"] = items
		object["session_scope_summary"] = set
		try JSONSerialization.data(withJSONObject: object).write(to: path)
		return accepted
	}

	private func fileItems(_ directory: URL) throws -> [[String: Any]] {
		let data = try Data(contentsOf: directory.appendingPathComponent(AppGroupSnapshotStore.fileName))
		let object = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
		let set = try XCTUnwrap(object["session_scope_summary"] as? [String: Any])
		return try XCTUnwrap(set["items"] as? [[String: Any]])
	}

	func testColdHostRetentionAndRepublicationKeepTheEightWellFormedSummaries() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let directory = storeDirectory()
		let isCodexWeek: (String, String) -> Bool = { $0 == "codex" && $1 == "7d" }
		let accepted = try await nineItemFile(directory, envelope: complete, malformed: isCodexWeek)
		let wellFormed = accepted.items.filter { !isCodexWeek($0.client, $0.period) }
		XCTAssertEqual(wellFormed.count, 8)

		// A cold host (new process) reads the file before its first publication.
		let publisher = WidgetSnapshotPublisher(store: AppGroupSnapshotStore(directoryURL: directory), reloader: WidgetTimelineReloader { _ in })
		let retainedValue = await publisher.retainedSessionScopeSummary()
		let lastValue = await publisher.lastSessionScopeSummary()
		XCTAssertEqual(try XCTUnwrap(retainedValue).items, wellFormed, "the retention path keeps every other item with its own identity and time")
		XCTAssertEqual(try XCTUnwrap(lastValue).items, wellFormed, "the reuse path keeps them too")
		let codexWeek = try XCTUnwrap(DesktopSessionScopeSummarySelectionV1(client: "codex", period: "7d"))
		XCTAssertEqual(try XCTUnwrap(retainedValue).unreadable, [codexWeek], "the malformed item's selection stays untrusted")

		// Cold host whose batch fails: re-publishes the eight, whole and stale.
		let failing = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			snapshotStore: AppGroupSnapshotStore(directoryURL: directory),
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: FailingWidgetScopeReader(), now: widgetBatchNow)
		)
		await failing.refresh()
		let republished = try XCTUnwrap(try AppGroupSnapshotStore(directoryURL: directory).read().sessionScopeSummary)
		XCTAssertTrue(republished.stale)
		XCTAssertEqual(republished.reason, AppGroupSessionScopeSummaryReason.selectionUnavailable)
		XCTAssertEqual(republished.items, wellFormed)
		XCTAssertEqual(Set(republished.items.map(\.publicationID)), Set(accepted.items.map(\.publicationID)))
		XCTAssertEqual(Set(republished.items.map(\.generatedAt)), Set(accepted.items.map(\.generatedAt)))
		// The malformed item keeps only its selection, never repaired into an
		// explicit null or given figures, identity or time.
		let written = try fileItems(directory)
		XCTAssertEqual(written.count, 9)
		let marker = try XCTUnwrap(written.first { isCodexWeek($0["client"] as? String ?? "", $0["period"] as? String ?? "") })
		XCTAssertEqual(Set(marker.keys), ["available", "client", "period"])
		XCTAssertEqual(marker["available"] as? Bool, false)
		XCTAssertTrue(written.filter { $0["available"] as? Bool == true }.allSatisfy { ($0["unknown_count"] as? Int) == 0 },
			"every kept Auxiliary item keeps its explicit 0")

		// A second cold host re-publishing again keeps exactly the same eight.
		let again = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			snapshotStore: AppGroupSnapshotStore(directoryURL: directory),
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: FailingWidgetScopeReader(), now: widgetBatchNow)
		)
		await again.refresh()
		XCTAssertEqual(try AppGroupSnapshotStore(directoryURL: directory).read().sessionScopeSummary, republished)
	}

	func testColdHostKeepsNothingWhenEveryItemIsMalformed() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let directory = storeDirectory()
		_ = try await nineItemFile(directory, envelope: complete) { _, _ in true }
		let publisher = WidgetSnapshotPublisher(store: AppGroupSnapshotStore(directoryURL: directory), reloader: WidgetTimelineReloader { _ in })
		let retained = await publisher.retainedSessionScopeSummary()
		XCTAssertNil(retained)
		let last = await publisher.lastSessionScopeSummary()
		XCTAssertNil(last)
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(complete)]),
			snapshotStore: AppGroupSnapshotStore(directoryURL: directory),
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: FailingWidgetScopeReader(), now: widgetBatchNow)
		)
		await coordinator.refresh()
		XCTAssertEqual(
			try AppGroupSnapshotStore(directoryURL: directory).read().sessionScopeSummary,
			.unavailable(AppGroupSessionScopeSummaryReason.selectionUnavailable)
		)
	}
}

// MARK: ASC-WS-R3-F1 — a malformed duplicate keeps its selection ambiguous

extension DesktopRefreshCoordinatorWidgetSummaryTests {
	/// Nine well-formed Auxiliary items plus `extra` raw items appended to the
	/// file; returns the accepted nine.
	private func nineItemFileWithExtras(
		_ directory: URL, envelope: DesktopWireEnvelopeV1, extras: ([[String: Any]]) -> [[String: Any]]
	) async throws -> AppGroupSessionScopeSummarySetV1 {
		let accepted = try await nineItemFile(directory, envelope: envelope) { _, _ in false }
		let path = directory.appendingPathComponent(AppGroupSnapshotStore.fileName)
		var object = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: path)) as? [String: Any])
		var set = try XCTUnwrap(object["session_scope_summary"] as? [String: Any])
		let items = try XCTUnwrap(set["items"] as? [[String: Any]])
		set["items"] = items + extras(items)
		object["session_scope_summary"] = set
		try JSONSerialization.data(withJSONObject: object).write(to: path)
		return accepted
	}

	private func refreshColdHost(_ directory: URL, envelope: DesktopWireEnvelopeV1) async throws -> AppGroupSessionScopeSummarySetV1 {
		let coordinator = DesktopRefreshCoordinator(
			host: ScriptedSnapshotRefresher(responses: [.snapshot(envelope)]),
			snapshotStore: AppGroupSnapshotStore(directoryURL: directory),
			widgetSessionScopeCollector: WidgetSessionScopeBatchCollector(reader: FailingWidgetScopeReader(), now: widgetBatchNow)
		)
		await coordinator.refresh()
		return try XCTUnwrap(try AppGroupSnapshotStore(directoryURL: directory).read().sessionScopeSummary)
	}

	func testMalformedDuplicateKeepsItsSelectionUntrustedAcrossColdHostRepublications() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let codexWeek = try XCTUnwrap(DesktopSessionScopeSummarySelectionV1(client: "codex", period: "7d"))
		func twin(_ items: [[String: Any]]) throws -> [String: Any] {
			var copy = try XCTUnwrap(items.first { $0["client"] as? String == "codex" && $0["period"] as? String == "7d" })
			copy.removeValue(forKey: "unknown_count")
			copy["count"] = 105
			return copy
		}
		let cases: [(String, ([[String: Any]]) throws -> [[String: Any]], [DesktopSessionScopeSummarySelectionV1])] = [
			("readable malformed twin", { [try twin($0)] }, [codexWeek]),
			("two malformed twins", { [try twin($0), try twin($0)] }, [codexWeek, codexWeek]),
			("unreadable twin", { items in var value = try twin(items); value.removeValue(forKey: "client"); return [value] }, []),
			("unsupported client", { items in var value = try twin(items); value["client"] = "gemini"; return [value] }, []),
		]
		for (name, extras, untrusted) in cases {
			let directory = storeDirectory()
			var extraItems = [[String: Any]]()
			let accepted = try await nineItemFileWithExtras(directory, envelope: complete) { items in
				extraItems = (try? extras(items)) ?? []
				return extraItems
			}
			XCTAssertEqual(extraItems.count, max(untrusted.count, 1), name)
			var previous: AppGroupSessionScopeSummarySetV1?
			for round in 1 ... 2 {
				let republished = try await refreshColdHost(directory, envelope: complete)
				XCTAssertTrue(republished.stale, "\(name) round \(round)")
				XCTAssertEqual(republished.items, accepted.items, "\(name) round \(round): the nine keep their identity and time")
				XCTAssertEqual(republished.unreadable, untrusted, "\(name) round \(round)")
				if let previous { XCTAssertEqual(republished, previous, "\(name): re-publication is stable") }
				previous = republished
				let bytes = String(decoding: try Data(contentsOf: directory.appendingPathComponent(AppGroupSnapshotStore.fileName)), as: UTF8.self)
				XCTAssertFalse(bytes.contains("\"unknown_count\":null"), "\(name): no missing key became null")
				XCTAssertFalse(bytes.contains("105"), "\(name): the malformed figure is never re-published")
			}
		}
	}

	func testWellFormedDuplicateIsKeptSoItsSelectionStaysAmbiguous() async throws {
		let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let directory = storeDirectory()
		let accepted = try await nineItemFileWithExtras(directory, envelope: complete) { items in
			var copy = items.first { $0["client"] as? String == "codex" && $0["period"] as? String == "7d" } ?? [:]
			copy["count"] = 105
			return [copy]
		}
		for round in 1 ... 2 {
			let republished = try await refreshColdHost(directory, envelope: complete)
			XCTAssertEqual(republished.items.count, 10, "round \(round)")
			XCTAssertEqual(Array(republished.items.prefix(9)), accepted.items, "round \(round)")
			XCTAssertEqual(republished.items.filter { $0.client == "codex" && $0.period == "7d" }.count, 2, "round \(round)")
		}
	}
}

/// Streams scripted lines, then holds the helper open until released: the
/// stretch after the terminal line in which the batch is still being answered.
private final class HeldBootstrapProcess: EmbeddedHelperProcess, @unchecked Sendable {
	private let head: [Data]
	private let tail: [Data]
	private let exitStatus: Int32
	private let lock = NSLock()
	private var gate: CheckedContinuation<Void, Never>?
	private var released = false
	private(set) var starts = 0

	init(head: [Data], tail: [Data], exitStatus: Int32 = 0) {
		self.head = head
		self.tail = tail
		self.exitStatus = exitStatus
	}

	private func countStart() {
		lock.lock()
		starts += 1
		lock.unlock()
	}

	func release() {
		lock.lock()
		released = true
		let waiting = gate
		gate = nil
		lock.unlock()
		waiting?.resume()
	}

	func run(executableURL _: URL, arguments: [String], environment: [String: String], timeout _: Duration) async throws -> HelperProcessOutput {
		if arguments == ["desktop", "state-location", "--format", "json"] {
			return try fixtureStateLocation(root: (environment["HOME"] ?? "/tmp/isolated-home") + "/.agentdeck-dev")
		}
		countStart()
		throw HelperExecutionError.launchFailed
	}

	func runLines(
		executableURL _: URL, arguments _: [String], environment _: [String: String], timeout _: Duration,
		maximumLineBytes _: Int, maximumLines _: Int, onLine: @escaping @Sendable (Data) -> Void
	) async throws -> HelperProcessLinesOutput {
		countStart()
		head.forEach(onLine)
		await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
			lock.lock()
			if released {
				lock.unlock()
				continuation.resume()
			} else {
				gate = continuation
				lock.unlock()
			}
		}
		tail.forEach(onLine)
		return HelperProcessLinesOutput(exitStatus: exitStatus, stdoutLines: [], stdoutBytes: 0)
	}
}

final class BootstrapSessionScopeDeliveryTests: XCTestCase {
	private func line(_ type: String, _ data: Any) throws -> Data {
		try JSONSerialization.data(withJSONObject: ["schema_version": 2, "command": "desktop.bootstrap", "type": type, "data": data])
	}

	private func terminal(_ state: String, session: String? = nil) throws -> Data {
		try line("terminal", ["usage": ["state": state], "session": ["state": session ?? state]])
	}

	private func batch(publication: String, views: [DesktopSessionScopeViewV1], asOf: String) throws -> Data {
		try line("session_scope_batch", ["publication_id": publication, "as_of": asOf,
			"views": JSONSerialization.jsonObject(with: JSONEncoder().encode(views))])
	}

	private func runner(_ process: HeldBootstrapProcess, in directory: URL) throws -> EmbeddedHelperRunner {
		EmbeddedHelperRunner(appBundleURL: try makeEmbeddedHelperBundle(in: directory), process: process,
			environment: ["HOME": directory.path], timeout: .seconds(5))
	}

	/// The bootstrap answers at its completed terminal line while the helper is
	/// still running; the batch that helper then hands over reaches the Widget
	/// reader for that publication, and no second helper is started.
	func testACompletedBootstrapReturnsAtItsTerminalLineAndHandsOverTheBatch() async throws {
		let fixture = try WidgetBatchFixture.load()
		let views = try XCTUnwrap(fixture.batches["auxiliary"])
		let snapshot = try servingCoordinatorFixture(id: "publication-final", full: true)
		let publication = try line("publication", JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)))
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let process = HeldBootstrapProcess(head: [publication, try terminal("completed")],
			tail: [try batch(publication: "publication-final", views: views, asOf: fixture.asOf)])
		let runner = try runner(process, in: directory)

		let returned = try await runner.bootstrapServingSnapshot(progress: { _ in }, publication: { _ in })
		XCTAssertEqual(returned?.header.publicationID, "publication-final", "returned while the helper is still held open")

		let foreign = await runner.widgetSessionScopeDelivered(publication: "publication-other")
		XCTAssertNil(foreign)
		async let waiting = runner.widgetSessionScopeDelivered(publication: "publication-final")
		try await Task.sleep(for: .milliseconds(50))
		process.release()
		let delivered = await waiting
		XCTAssertEqual(delivered, WidgetSessionScopeDelivery(asOf: fixture.asOf, views: views))
		// It stays available to every publish of that publication.
		let again = await runner.widgetSessionScopeDelivered(publication: "publication-final")
		XCTAssertEqual(again, delivered)
		XCTAssertEqual(process.starts, 1, "the batch came from the bootstrap helper itself")

		// A new stream discards the previous delivery.
		let next = HeldBootstrapProcess(head: [publication, try terminal("completed")], tail: [])
		next.release()
		_ = try await EmbeddedHelperRunner(appBundleURL: try makeEmbeddedHelperBundle(in: directory), process: next,
			environment: ["HOME": directory.path], timeout: .seconds(5))
			.bootstrapServingSnapshot(progress: { _ in }, publication: { _ in })
	}

	/// A helper that exits without the batch, or with one for another
	/// publication or one that cannot be read, delivers nothing: the reader
	/// answers nil and the collector reads the batch itself.
	func testAHelperThatEndsWithoutAUsableBatchDeliversNone() async throws {
		let fixture = try WidgetBatchFixture.load()
		let views = try XCTUnwrap(fixture.batches["auxiliary"])
		let snapshot = try servingCoordinatorFixture(id: "publication-final", full: true)
		let publication = try line("publication", JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)))
		for tail in [[], [try batch(publication: "publication-stale", views: views, asOf: fixture.asOf)],
			[try line("session_scope_batch", ["publication_id": "publication-final"])]] {
			let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
			defer { try? FileManager.default.removeItem(at: directory) }
			let process = HeldBootstrapProcess(head: [publication, try terminal("completed")], tail: tail)
			let runner = try runner(process, in: directory)
			_ = try await runner.bootstrapServingSnapshot(progress: { _ in }, publication: { _ in })
			async let waiting = runner.widgetSessionScopeDelivered(publication: "publication-final")
			try await Task.sleep(for: .milliseconds(50))
			process.release()
			let delivered = await waiting
			XCTAssertNil(delivered)
		}
	}

	/// A round that did not complete is not answered at its terminal line: the
	/// helper's exit decides, as before, and nothing is promised to the Widget.
	func testAnIncompleteRoundStillWaitsForTheHelperAndFails() async throws {
		let snapshot = try servingCoordinatorFixture(id: "publication-final", full: true)
		let publication = try line("publication", JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)))
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let process = HeldBootstrapProcess(head: [publication, try terminal("failed")], tail: [], exitStatus: 1)
		let runner = try runner(process, in: directory)
		let call = Task { try await runner.bootstrapServingSnapshot(progress: { _ in }, publication: { _ in }) }
		try await Task.sleep(for: .milliseconds(100))
		let early = await runner.widgetSessionScopeDelivered(publication: "publication-final")
		XCTAssertNil(early, "no batch is promised for a round that did not complete")
		process.release()
		do {
			_ = try await call.value
			XCTFail("an incomplete round must fail")
		} catch let error as HelperExecutionError {
			XCTAssertEqual(error, .nonZeroExit(1))
		}
	}

	func testPartialTerminalWithZeroExitCannotCountAsASuccessfulRound() async throws {
		let snapshot = try servingCoordinatorFixture(id: "publication-partial", full: false)
		let publication = try line("publication", JSONSerialization.jsonObject(with: JSONEncoder().encode(snapshot)))
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let process = HeldBootstrapProcess(head: [publication, try terminal("completed", session: "failed")], tail: [])
		process.release()
		let runner = try runner(process, in: directory)
		do {
			_ = try await runner.bootstrapServingSnapshot(progress: { _ in }, publication: { _ in })
			XCTFail("both domains must complete before bootstrap can report success")
		} catch let error as HelperExecutionError {
			XCTAssertEqual(error, .malformedOutput)
		}
	}
}
