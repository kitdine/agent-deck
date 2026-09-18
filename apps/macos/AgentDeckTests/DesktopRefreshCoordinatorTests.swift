import Foundation
import XCTest
@testable import AgentDeckShared

@MainActor
final class DesktopRefreshCoordinatorTests: XCTestCase {
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

		await host.resume()
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

	func testCacheWriteFailureKeepsFreshSnapshotAvailableInMemory() async throws {
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
		XCTAssertEqual(
			coordinator.state,
			.degraded(previous: complete, issue: .storageUnavailable)
		)
		let presentation = DesktopPresentationState.derive(from: coordinator.state)
		XCTAssertEqual(presentation.surface, .dataSurface)
		XCTAssertTrue(presentation.qualifiers.contains(.failing))
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
