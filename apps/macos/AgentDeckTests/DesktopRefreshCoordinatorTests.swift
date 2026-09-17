import Foundation
import XCTest
@testable import AgentDeckShared

@MainActor
final class DesktopRefreshCoordinatorTests: XCTestCase {
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
