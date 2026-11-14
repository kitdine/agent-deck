import AppKit
import SwiftUI
import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

/// One scripted transport. Each queue answers in order and repeats its last
/// answer; a held write stays in flight until the test releases it.
actor StubOriginTransport: SessionOriginSettingsTransport {
	struct Write: Equatable {
		let action: OriginPendingOperation.Action
		let expectedRevision: Int64
		let requestID: String
	}

	private var defaults: [DesktopOriginReadOutcome<DesktopSessionScopeDefaultV1>]
	private var pages: [DesktopOriginReadOutcome<DesktopOriginOverridePageV1>]
	private var statuses: [DesktopOriginReadOutcome<DesktopOriginSourceStatusesV1>]
	private var lookups: [DesktopOriginLookupOutcome]
	private var writeOutcomes: [DesktopOriginWriteOutcome]
	private var receipts: [DesktopOriginReceiptOutcome]
	private(set) var writes: [Write] = []
	private(set) var lookupCalls: [DesktopOriginTargetV1] = []
	private(set) var receiptCalls: [String] = []
	private(set) var defaultReads = 0
	private(set) var pageReads: [Int] = []
	private var holdWrites = false
	private var held: CheckedContinuation<Void, Never>?

	/// A read held once: it takes its answer when called and returns it only
	/// when released, so a write can complete while it is in flight.
	enum Read: Hashable { case defaultScope, overridePage, lookup, sourceStatus }
	private var holdNextRead: Set<Read> = []
	private var heldReads: [Read: CheckedContinuation<Void, Never>] = [:]

	func holdNext(_ read: Read) { holdNextRead.insert(read) }
	func release(_ read: Read) { heldReads.removeValue(forKey: read)?.resume() }
	func isHolding(_ read: Read) -> Bool { heldReads[read] != nil }

	private func pause(_ read: Read) async {
		guard holdNextRead.remove(read) != nil else { return }
		await withCheckedContinuation { heldReads[read] = $0 }
	}

	init(
		defaults: [DesktopOriginReadOutcome<DesktopSessionScopeDefaultV1>] = [.value(.confirmedWork(revision: 0))],
		pages: [DesktopOriginReadOutcome<DesktopOriginOverridePageV1>] = [.value(DesktopOriginOverridePageV1(items: [], page: 1, limit: 20, total: 0))],
		statuses: [DesktopOriginReadOutcome<DesktopOriginSourceStatusesV1>] = [.unavailable],
		lookups: [DesktopOriginLookupOutcome] = [.notFound],
		writes: [DesktopOriginWriteOutcome] = [.unknown],
		receipts: [DesktopOriginReceiptOutcome] = [.unknown]
	) {
		self.defaults = defaults
		self.pages = pages
		self.statuses = statuses
		self.lookups = lookups
		writeOutcomes = writes
		self.receipts = receipts
	}

	func setHoldWrites(_ hold: Bool) { holdWrites = hold }
	func releaseWrite() { held?.resume(); held = nil }
	var isHoldingWrite: Bool { held != nil }

	private func next<Value>(_ queue: inout [Value]) -> Value {
		queue.count == 1 ? queue[0] : queue.removeFirst()
	}

	private func write(_ action: OriginPendingOperation.Action, _ expected: Int64, _ requestID: String) async -> DesktopOriginWriteOutcome {
		writes.append(Write(action: action, expectedRevision: expected, requestID: requestID))
		if holdWrites { await withCheckedContinuation { held = $0 } }
		return next(&writeOutcomes)
	}

	func readDefaultScope() async -> DesktopOriginReadOutcome<DesktopSessionScopeDefaultV1> {
		defaultReads += 1
		let answer = next(&defaults)
		await pause(.defaultScope)
		return answer
	}

	func saveDefaultScope(_ value: DesktopSessionScopeV1, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		await write(.saveDefault(value), expectedRevision, requestID)
	}

	func lookupSession(client: String, sessionID: String) async -> DesktopOriginLookupOutcome {
		lookupCalls.append(DesktopOriginTargetV1(client: client, sessionID: sessionID))
		let answer = next(&lookups)
		await pause(.lookup)
		return answer
	}

	func setOverride(_ target: DesktopOriginTargetV1, value: String, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		await write(.setOverride(target, value: value), expectedRevision, requestID)
	}

	func clearOverride(_ target: DesktopOriginTargetV1, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		await write(.clearOverride(target), expectedRevision, requestID)
	}

	func readReceipt(requestID: String) async -> DesktopOriginReceiptOutcome {
		receiptCalls.append(requestID)
		return next(&receipts)
	}

	func listOverrides(page: Int, limit: Int) async -> DesktopOriginReadOutcome<DesktopOriginOverridePageV1> {
		pageReads.append(page)
		let answer = next(&pages)
		await pause(.overridePage)
		return answer
	}

	func readOriginSourceStatus() async -> DesktopOriginReadOutcome<DesktopOriginSourceStatusesV1> {
		let answer = next(&statuses)
		await pause(.sourceStatus)
		return answer
	}
}

/// A value a publish stub records into from its main-actor closure.
@MainActor
final class OriginTestBox<Value> {
	var value: Value
	init(_ value: Value) { self.value = value }
}

extension DesktopSessionScopeDefaultV1 {
	static func confirmedWork(revision: Int64) -> Self { Self(confirmed: true, value: .work, revision: revision, reason: nil) }
	static func confirmed(_ scope: DesktopSessionScopeV1, revision: Int64) -> Self { Self(confirmed: true, value: scope, revision: revision, reason: nil) }
}

@MainActor
final class SessionOriginSettingsControllerTests: XCTestCase {
	private let maintenance = DesktopOriginTargetV1(client: "claude", sessionID: "maint-1")
	private let archived = DesktopOriginTargetV1(client: "codex", sessionID: "archived-1")

	private func makeController(
		_ transport: StubOriginTransport,
		publish: @escaping @MainActor () async -> OriginPublicationResult = { .failed }
	) -> SessionOriginSettingsController {
		var sequence = 0
		return SessionOriginSettingsController(transport: transport, publish: publish, newRequestID: {
			sequence += 1
			return "settings-test-\(sequence)"
		})
	}

	private func page(_ items: [DesktopOriginOverrideV1], page number: Int = 1, total: Int? = nil) -> DesktopOriginReadOutcome<DesktopOriginOverridePageV1> {
		.value(DesktopOriginOverridePageV1(items: items, page: number, limit: 20, total: total ?? items.count))
	}

	private func query(_ target: DesktopOriginTargetV1, automatic: String = "auxiliary", effective: String? = nil, override: String? = nil, revision: Int64) -> DesktopOriginLookupOutcome {
		let state = override.map { DesktopSessionOriginOverrideV1(value: $0, bindingStatus: "matched", bindingLimited: false, applied: true) }
		return .found(DesktopClassificationQueryV1(target: target, classificationAvailable: true, origin: DesktopSessionOriginV1(
			automatic: automatic, effective: effective ?? override ?? automatic, reasonCode: "maintenance_origin",
			ruleVersion: "claude-mem-sdk-map-v1", override: state, revision: revision
		)))
	}

	private func waitUntilHeld(_ transport: StubOriginTransport) async {
		for _ in 0 ..< 200 {
			if await transport.isHoldingWrite { return }
			await Task.yield()
		}
		XCTFail("the write never reached the transport")
	}


	private func clearReceipt(_ target: DesktopOriginTargetV1, requestID: String = "settings-test-1", revision: Int64, replayed: Bool = false) -> DesktopOriginWriteOutcome {
		.committed(DesktopOriginReceiptV1(requestID: requestID, kind: .overrideClear, target: target, outcome: .saved, value: nil, revision: revision,
			replayed: replayed, publicationPending: !replayed, classificationReason: "not_queried"))
	}

	private func findMaintenance(_ controller: SessionOriginSettingsController) async {
		controller.setLookupClient("claude")
		controller.setLookupID("maint-1")
		await controller.findSession()
	}

	// MARK: Default scope

	/// ASC-SET-R1-F1: an unread or unreadable default is never Work, and no
	/// write can be based on it.
	func testAnUnconfirmedDefaultIsNeverWorkAndLocksTheGroup() async {
		let transport = StubOriginTransport(defaults: [.unavailable])
		let controller = makeController(transport)
		XCTAssertEqual(controller.defaultState, .loading)
		XCTAssertEqual(controller.status, .loading)
		XCTAssertNil(controller.defaultState.scope, "nothing is selected before the read")
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .busy, text: t(DesktopCopy.originDefaultReading)))
		XCTAssertFalse(controller.writesEnabled)

		await controller.open()
		XCTAssertEqual(controller.defaultState, .unavailable)
		XCTAssertNil(controller.defaultState.scope, "no Work selection appears for an unknown default")
		XCTAssertEqual(controller.status, .loadFailed)
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .bad, text: t(DesktopCopy.originDefaultReadFailed), actions: [.readAgain]))
		XCTAssertNil(controller.statusLine(for: .manualClassification), "the failure shows once, under the default")
		await controller.selectDefault(.auxiliary)
		let writes = await transport.writes
		XCTAssertEqual(writes, [], "an unconfirmed default has no revision to save against")
	}

	/// Choosing is saving: the confirmed revision and a request ID the App
	/// assigned go out at once. Closing the window during the save keeps the
	/// operation and drops only drafts; the result applies when it arrives.
	func testChoosingTheDefaultSavesItOutlivesTheWindowAndPublishes() async throws {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 2))],
			writes: [.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .saved, value: "auxiliary", revision: 3, publicationPending: true))]
		)
		let published = OriginTestBox(0)
		let controller = makeController(transport) {
			published.value += 1
			return .published(.confirmed(.auxiliary, revision: 3))
		}
		await controller.open()
		XCTAssertEqual(controller.defaultState, .confirmed(.work, revision: 2))
		XCTAssertTrue(controller.writesEnabled)
		XCTAssertNil(controller.statusLine(for: .defaultScope), "nothing to say while idle")

		await transport.setHoldWrites(true)
		let save = Task { await controller.selectDefault(.auxiliary) }
		await waitUntilHeld(transport)
		XCTAssertEqual(controller.status, .saving)
		XCTAssertEqual(controller.defaultState.scope, .work, "the selection stays on the confirmed value until the write is confirmed")
		XCTAssertFalse(controller.writesEnabled)
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .busy, text: t(DesktopCopy.originDefaultSaving, t(DesktopCopy.sessionScopeAuxiliary))))
		XCTAssertEqual(controller.statusLine(for: .manualClassification), OriginStatusLine(tone: .neutral, text: t(DesktopCopy.originLockedByDefault)),
			"the other control only says why it is locked")

		controller.windowClosed()
		XCTAssertEqual(controller.status, .saving, "closing does not cancel an accepted write")

		await transport.releaseWrite()
		await save.value
		let writes = await transport.writes
		XCTAssertEqual(writes, [StubOriginTransport.Write(action: .saveDefault(.auxiliary), expectedRevision: 2, requestID: "settings-test-1")])
		XCTAssertEqual(controller.defaultState, .confirmed(.auxiliary, revision: 3))
		XCTAssertEqual(controller.notice, .saved(.scopeSet))
		XCTAssertTrue(controller.writesEnabled)
		await controller.publicationTask?.value
		XCTAssertEqual(published.value, 1)
		XCTAssertFalse(controller.publicationPending)
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .good, text: t(DesktopCopy.originDefaultSaved)))
		XCTAssertNil(controller.statusLine(for: .manualClassification), "unlocked once the write settled")
	}

	/// Choosing the value already confirmed sends nothing, for the default and
	/// for a session's classification.
	func testChoosingTheConfirmedValueSendsNothing() async {
		let transport = StubOriginTransport(lookups: [query(maintenance, override: "work", revision: 2)])
		let controller = makeController(transport)
		await controller.open()
		await controller.selectDefault(.work)
		await findMaintenance(controller)
		guard case let .found(result) = controller.lookup else { return XCTFail("found") }
		XCTAssertEqual(result.choice, .work, "the stored override is what the card shows")
		await controller.selectOrigin(.work)
		let writes = await transport.writes
		XCTAssertEqual(writes, [])
		XCTAssertNil(controller.notice)
	}

	/// Saved is not published: a failed or older publication keeps the
	/// pending notice and never rolls the saved default back.
	func testPublicationPendingNeverRollsBackTheSavedDefault() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 0))],
			writes: [
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .saved, value: "all", revision: 1, publicationPending: true)),
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-2", kind: .scopeSet, outcome: .saved, value: "work", revision: 2, publicationPending: true)),
			]
		)
		let answers = OriginTestBox<[OriginPublicationResult]>([.failed, .published(.confirmed(.all, revision: 1))])
		let controller = makeController(transport) { answers.value.removeFirst() }
		await controller.open()
		await controller.selectDefault(.all)
		await controller.publicationTask?.value
		XCTAssertTrue(controller.publicationPending)
		XCTAssertEqual(controller.defaultState, .confirmed(.all, revision: 1))
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .info, text: t(DesktopCopy.originDefaultPublicationPending)))

		// A publication still carrying the previous default revision is not ours.
		await controller.selectDefault(.work)
		await controller.publicationTask?.value
		XCTAssertTrue(controller.publicationPending)
		XCTAssertEqual(controller.defaultState, .confirmed(.work, revision: 2))
	}

	func testNoChangeRequestsNoPublicationAndPermissionWarningStaysSaved() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmed(.auxiliary, revision: 4))],
			writes: [
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .noChange, value: "work", revision: 4)),
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-2", kind: .scopeSet, outcome: .saved, value: "all", revision: 5,
					warnings: [DesktopOriginReceiptV1.permissionsWarning], publicationPending: true)),
			]
		)
		let publications = OriginTestBox(0)
		let controller = makeController(transport) { publications.value += 1; return .published(.confirmed(.all, revision: 5)) }
		await controller.open()
		await controller.selectDefault(.work)
		XCTAssertEqual(controller.notice, .noChange)
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .neutral, text: t(DesktopCopy.originNoChange)))
		XCTAssertNil(controller.publicationTask)
		XCTAssertEqual(publications.value, 0, "no change does no derived publication work")

		await controller.selectDefault(.all)
		await controller.publicationTask?.value
		XCTAssertEqual(controller.status, .ready, "a permission warning is not a save failure")
		XCTAssertEqual(controller.notice, .saved(.scopeSet))
		XCTAssertTrue(controller.permissionWarning)
		XCTAssertEqual(controller.permissionWarningText(for: .defaultScope), t(DesktopCopy.originPermissionWarning))
		XCTAssertNil(controller.permissionWarningText(for: .manualClassification), "the warning stays with the control that saved")
		XCTAssertEqual(controller.defaultState, .confirmed(.all, revision: 5), "the committed value is not rolled back")
		XCTAssertEqual(publications.value, 1)
	}

	// MARK: Unknown, conflict and failure

	/// ASC-SET-R3-F1: an unknown outcome is read back by its request ID; when
	/// the read-back still finds nothing, the same request is retried with the
	/// same ID, payload and expected revision, and a definite result unlocks.
	func testAnUnknownOutcomeIsReadBackThenRetriedUnderTheSameRequest() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 2)), .value(.confirmed(.auxiliary, revision: 3))],
			writes: [.unknown, .committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .saved, value: "auxiliary", revision: 3, replayed: true, publicationPending: true))],
			receipts: [.unknown]
		)
		let controller = makeController(transport)
		await controller.open()
		await controller.selectDefault(.auxiliary)
		XCTAssertEqual(controller.status, .unknown)
		XCTAssertFalse(controller.writesEnabled, "no new write while the outcome is unknown")
		XCTAssertFalse(controller.retryAvailable, "read the receipt before any retry")
		XCTAssertEqual(controller.defaultState, .confirmed(.work, revision: 2), "the old value is only the last confirmed one")
		XCTAssertEqual(controller.statusLine(for: .defaultScope),
			OriginStatusLine(tone: .warning, text: t(DesktopCopy.originUnknown), requestID: "settings-test-1", actions: [.checkResult]),
			"the request ID shows only while the outcome is unknown")
		await controller.retry()
		var writes = await transport.writes
		XCTAssertEqual(writes.count, 1)

		await controller.readRequestReceipt()
		let receiptCalls = await transport.receiptCalls
		XCTAssertEqual(receiptCalls, ["settings-test-1"])
		XCTAssertEqual(controller.status, .unknown, "finding no receipt is not \"not saved\"")
		XCTAssertTrue(controller.retryAvailable)
		XCTAssertEqual(controller.statusLine(for: .defaultScope),
			OriginStatusLine(tone: .warning, text: t(DesktopCopy.originStillUnknown), requestID: "settings-test-1", actions: [.checkAgain, .retryOriginal]))

		controller.windowClosed()
		XCTAssertEqual(controller.status, .unknown, "the pending request survives closing the window")
		await controller.retry()
		writes = await transport.writes
		XCTAssertEqual(writes, [
			StubOriginTransport.Write(action: .saveDefault(.auxiliary), expectedRevision: 2, requestID: "settings-test-1"),
			StubOriginTransport.Write(action: .saveDefault(.auxiliary), expectedRevision: 2, requestID: "settings-test-1"),
		], "the retry reuses the request ID, payload and expected revision")
		XCTAssertEqual(controller.status, .ready)
		XCTAssertEqual(controller.notice, .replayed)
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .info, text: t(DesktopCopy.originReplayed)))
		XCTAssertEqual(controller.defaultState, .confirmed(.auxiliary, revision: 3), "a replayed receipt is followed by a read of the current value")
		XCTAssertTrue(controller.writesEnabled)
	}

	/// A receipt found on read-back ends the pending operation, but the
	/// current value is read again: the receipt is not the current state.
	func testAReceiptFoundOnReadBackEndsPendingAndTheCurrentValueIsReadAgain() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 2)), .value(.confirmed(.all, revision: 6))],
			writes: [.unknown],
			receipts: [.known(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .saved, value: "auxiliary", revision: 3))]
		)
		let controller = makeController(transport)
		await controller.open()
		await controller.selectDefault(.auxiliary)
		await controller.readRequestReceipt()
		XCTAssertEqual(controller.operation, .idle)
		XCTAssertEqual(controller.notice, .saved(.scopeSet))
		XCTAssertEqual(controller.defaultState, .confirmed(.all, revision: 6), "a later write elsewhere is shown, not the receipt's value")
		XCTAssertNil(controller.statusLine(for: .defaultScope)?.requestID, "a settled result shows no request ID")
	}

	func testAConflictLocksUntilTheLatestValuesAreReadAndNeverReplaysTheChoice() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 2)), .value(.confirmed(.all, revision: 3))],
			writes: [.conflict(currentRevision: 3)]
		)
		let controller = makeController(transport)
		await controller.open()
		await controller.selectDefault(.auxiliary)
		XCTAssertEqual(controller.status, .conflict)
		XCTAssertFalse(controller.writesEnabled)
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .warning, text: t(DesktopCopy.originConflict), actions: [.loadLatest]))
		await controller.retry()
		await controller.open()
		var writes = await transport.writes
		XCTAssertEqual(writes.count, 1, "neither retry nor reopening replays the old choice")
		XCTAssertEqual(controller.defaultState, .confirmed(.work, revision: 2), "reopening during a conflict keeps the values the choice was based on")

		await controller.readLatestValues()
		XCTAssertEqual(controller.operation, .idle)
		XCTAssertEqual(controller.notice, .reloaded)
		XCTAssertEqual(controller.defaultState, .confirmed(.all, revision: 3))
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .info, text: t(DesktopCopy.originDefaultReloaded, t(DesktopCopy.clientAll))),
			"the user chooses again from the latest value")
		writes = await transport.writes
		XCTAssertEqual(writes.count, 1)
	}

	func testACertainFailureKeepsTheConfirmedValueAndAllowsTheSameRetryOrANewChoice() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 2))],
			writes: [.notSaved(.writeFailed), .notSaved(.busy), .committed(DesktopOriginReceiptV1(requestID: "settings-test-2", kind: .scopeSet, outcome: .saved, value: "all", revision: 3))]
		)
		let controller = makeController(transport)
		await controller.open()
		await controller.selectDefault(.auxiliary)
		XCTAssertEqual(controller.status, .failed)
		XCTAssertEqual(controller.statusLine(for: .defaultScope),
			OriginStatusLine(tone: .bad, text: t(DesktopCopy.originDefaultNotSaved, t(DesktopCopy.sessionScopeWork)), actions: [.retry]))
		XCTAssertNil(controller.statusLine(for: .manualClassification), "a certain failure does not lock the other control")
		XCTAssertEqual(controller.defaultState, .confirmed(.work, revision: 2))
		XCTAssertTrue(controller.writesEnabled, "a certain failure does not lock")
		XCTAssertTrue(controller.retryAvailable)
		await controller.retry()
		XCTAssertEqual(controller.status, .failed, "the error persists until a read-back or a successful operation")
		await controller.selectDefault(.all)
		XCTAssertEqual(controller.status, .ready)
		let writes = await transport.writes
		XCTAssertEqual(writes.map(\.requestID), ["settings-test-1", "settings-test-1", "settings-test-2"])
	}

	func testOneWriteAtATime() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 0))],
			pages: [page([DesktopOriginOverrideV1(client: "codex", sessionID: "archived-1", value: "auxiliary", bindingStatus: "unmatched", revision: 4)])],
			writes: [.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .saved, value: "all", revision: 1))]
		)
		let controller = makeController(transport)
		await controller.open()
		await transport.setHoldWrites(true)
		let save = Task { await controller.selectDefault(.all) }
		await waitUntilHeld(transport)
		await controller.selectDefault(.auxiliary)
		if case let .loaded(list) = controller.overrides { await controller.removeOverride(list.items[0]) }
		await controller.retry()
		let writes = await transport.writes
		XCTAssertEqual(writes.count, 1, "a second write is not accepted while one is saving")
		await transport.releaseWrite()
		await save.value
	}

	// MARK: Reads that predate a write (ASC-SC-R1-F1)

	private func waitUntilHolding(_ transport: StubOriginTransport, _ read: StubOriginTransport.Read) async {
		for _ in 0 ..< 400 {
			if await transport.isHolding(read) { return }
			await Task.yield()
		}
		XCTFail("the \(read) read never reached the transport")
	}

	/// ASC-SC-R1-F1: the default read a reopened window starts can return after
	/// a save it predates has committed. It must not roll the confirmed default
	/// or the next expected revision back; a read started after the write
	/// still applies.
	func testAReopenReadThatPredatesASaveCannotRollTheConfirmedDefaultBack() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 0)), .value(.confirmedWork(revision: 0)), .value(.confirmed(.auxiliary, revision: 7))],
			writes: [
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .saved, value: "all", revision: 1, publicationPending: true)),
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-2", kind: .scopeSet, outcome: .saved, value: "work", revision: 2, publicationPending: true)),
			]
		)
		let controller = makeController(transport)
		await controller.open()
		controller.windowClosed()
		await transport.holdNext(.defaultScope)
		let reopen = Task { await controller.open() }
		await waitUntilHolding(transport, .defaultScope)
		XCTAssertTrue(controller.writesEnabled, "a reopen read does not lock the group")
		await controller.selectDefault(.all)
		XCTAssertEqual(controller.defaultState, .confirmed(.all, revision: 1))

		await transport.release(.defaultScope)
		await reopen.value
		XCTAssertEqual(controller.defaultState, .confirmed(.all, revision: 1), "the older read does not replace the confirmed save")
		await controller.selectDefault(.work)
		let writes = await transport.writes
		XCTAssertEqual(writes.map(\.expectedRevision), [0, 1], "the next save is based on the confirmed revision")

		await controller.open()
		XCTAssertEqual(controller.defaultState, .confirmed(.auxiliary, revision: 7), "a read started after the write still shows a change made elsewhere")
	}

	/// The same reopen also reads the override list; a list read that predates a
	/// removal must not bring the removed override back.
	func testAReopenListReadThatPredatesARemovalCannotRestoreTheRow() async {
		let row = DesktopOriginOverrideV1(client: "codex", sessionID: "archived-1", value: "auxiliary", bindingStatus: "unmatched", revision: 4)
		let empty = DesktopOriginOverridePageV1(items: [], page: 1, limit: 20, total: 0)
		let transport = StubOriginTransport(
			pages: [page([row]), page([row]), .value(empty)],
			writes: [clearReceipt(archived, revision: 5)]
		)
		let controller = makeController(transport)
		await controller.open()
		controller.windowClosed()
		await transport.holdNext(.overridePage)
		let reopen = Task { await controller.open() }
		await waitUntilHolding(transport, .overridePage)
		await controller.removeOverride(row)
		XCTAssertEqual(controller.overrides, .loaded(empty))

		await transport.release(.overridePage)
		await reopen.value
		XCTAssertEqual(controller.overrides, .loaded(empty), "the older list does not restore the removed override")
	}

	/// A classification query that predates a save of the same session must not
	/// restore the older override and revision the next save would be based on.
	func testALookupThatPredatesASaveCannotRestoreTheOlderOverride() async {
		let transport = StubOriginTransport(
			lookups: [query(maintenance, override: "work", revision: 2), query(maintenance, revision: 3)],
			writes: [
				clearReceipt(maintenance, revision: 3),
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-2", kind: .overrideSet, target: maintenance, outcome: .saved, value: "auxiliary", revision: 4, publicationPending: true)),
				clearReceipt(maintenance, requestID: "settings-test-3", revision: 5),
			]
		)
		let controller = makeController(transport)
		await controller.open()
		await findMaintenance(controller)
		await controller.selectOrigin(.automatic)

		await transport.holdNext(.lookup)
		let query = Task { await controller.readCurrentClassification() }
		await waitUntilHolding(transport, .lookup)
		await controller.selectOrigin(.auxiliary)
		await transport.release(.lookup)
		await query.value
		guard case let .found(result) = controller.lookup else { return XCTFail("found") }
		XCTAssertEqual(result.override, OriginLookupResult.Override(value: "auxiliary", bindingStatus: "matched", applied: true), "the older query does not restore the cleared state")
		XCTAssertEqual(result.revision, 4)

		await controller.selectOrigin(.automatic)
		let writes = await transport.writes
		XCTAssertEqual(writes.map(\.expectedRevision), [2, 3, 4], "each save is based on the revision its predecessor confirmed")
	}

	// MARK: Single session and overrides

	func testEditingTheLookupDiscardsTheOldTargetAndLookupsStaySeparateFromFailures() async {
		let transport = StubOriginTransport(lookups: [query(maintenance, revision: 0), .notFound, .unavailable])
		let controller = makeController(transport)
		await controller.open()
		controller.setLookupClient("claude")
		controller.setLookupID("  maint-1 ")
		await controller.findSession()
		let calls = await transport.lookupCalls
		XCTAssertEqual(calls, [maintenance], "an exact client and trimmed complete ID")
		guard case let .found(result) = controller.lookup else { return XCTFail("found") }
		XCTAssertEqual(result.automatic?.value, "auxiliary")
		XCTAssertEqual(result.choice, .automatic)

		controller.setLookupID("maint-2")
		XCTAssertEqual(controller.lookup, .idle, "an edit clears the old result")
		await controller.selectOrigin(.work)
		let writes = await transport.writes
		XCTAssertEqual(writes, [], "the old card cannot act on the new input")

		await controller.findSession()
		XCTAssertEqual(controller.lookup, .notFound)
		await controller.findSession()
		XCTAssertEqual(controller.lookup, .unavailable)
		controller.setLookupID("   ")
		XCTAssertFalse(controller.lookupEnabled, "an empty ID cannot be looked up")
	}

	/// A set confirms the override and the effective class it produces; the
	/// automatic class stays what the query found.
	func testSettingAnOverrideUsesTheShownRevisionAndKeepsAutomaticSeparate() async {
		let transport = StubOriginTransport(
			lookups: [query(maintenance, revision: 1)],
			writes: [.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .overrideSet, target: maintenance, outcome: .saved, value: "work", revision: 2, publicationPending: true))]
		)
		let controller = makeController(transport)
		await controller.open()
		await findMaintenance(controller)
		await controller.selectOrigin(.work)
		let writes = await transport.writes
		XCTAssertEqual(writes, [StubOriginTransport.Write(action: .setOverride(maintenance, value: "work"), expectedRevision: 1, requestID: "settings-test-1")])
		guard case let .found(result) = controller.lookup else { return XCTFail("found") }
		XCTAssertEqual(result.effective, "work")
		XCTAssertEqual(result.override, OriginLookupResult.Override(value: "work", bindingStatus: "matched", applied: true))
		XCTAssertEqual(result.automatic?.value, "auxiliary", "automatic is never rewritten as the user's choice")
		XCTAssertEqual(result.revision, 2)
		XCTAssertEqual(OriginSettingsCopy.effectiveText(result), "\(t(DesktopCopy.sessionScopeWork)) · \(t(DesktopCopy.originCountsManual))")
		XCTAssertEqual(OriginSettingsCopy.automaticText(result.automatic), "\(t(DesktopCopy.sessionScopeAuxiliary)) · \(SessionOriginCopy.reason("maintenance_origin"))")
		XCTAssertEqual(OriginSettingsCopy.automaticHelp(result.automatic), "maintenance_origin · claude-mem-sdk-map-v1", "the reason code and rule stay in the tooltip")
		XCTAssertEqual(controller.notice, .saved(.overrideSet))
		let pageReads = await transport.pageReads
		XCTAssertEqual(pageReads.count, 2, "the override list is read again")
	}

	/// The clear's receipt carries no classification (not_queried). A card
	/// still shown reads it at once with a separate visible query, shows
	/// "Reading…" meanwhile, and never the value from before the clear.
	func testAClearIsFollowedAtOnceByASeparateVisibleQuery() async {
		let transport = StubOriginTransport(
			lookups: [query(maintenance, automatic: "unknown", override: "work", revision: 2), query(maintenance, automatic: "unknown", revision: 3)],
			writes: [clearReceipt(maintenance, revision: 3)]
		)
		let controller = makeController(transport)
		await controller.open()
		await findMaintenance(controller)
		await transport.holdNext(.lookup)
		let clear = Task { await controller.selectOrigin(.automatic) }
		await waitUntilHolding(transport, .lookup)
		let writes = await transport.writes
		XCTAssertEqual(writes, [StubOriginTransport.Write(action: .clearOverride(maintenance), expectedRevision: 2, requestID: "settings-test-1")])
		guard case let .found(reading) = controller.lookup else { return XCTFail("found") }
		XCTAssertFalse(reading.classificationAvailable, "the receipt is not the current classification")
		XCTAssertNil(reading.override)
		XCTAssertEqual(reading.classificationQuery, .reading)
		XCTAssertNil(OriginSettingsCopy.effectiveText(reading), "nothing from before the clear is reused")

		await transport.release(.lookup)
		await clear.value
		let lookups = await transport.lookupCalls
		XCTAssertEqual(lookups, [maintenance, maintenance], "one lookup, then the separate query the clear triggered")
		guard case let .found(queried) = controller.lookup else { return XCTFail("found") }
		XCTAssertTrue(queried.classificationAvailable)
		XCTAssertEqual(queried.revision, 3)
		XCTAssertEqual(queried.choice, .automatic)
		XCTAssertEqual(OriginSettingsCopy.effectiveText(queried),
			"\(t(DesktopCopy.sessionClassUnknown)) · \(t(DesktopCopy.originCountsDetected))\(t(DesktopCopy.originCountsIncludedInWork))")
		XCTAssertEqual(controller.statusLine(for: .manualClassification), OriginStatusLine(tone: .info, text: t(DesktopCopy.originManualPublicationPending)))
	}

	/// A failed, not-found or excluded query after a clear keeps the card with
	/// its classification unavailable; reading again shows it once readable.
	func testAFailedQueryAfterAClearStaysUnavailableUntilReadAgain() async {
		let transport = StubOriginTransport(
			lookups: [query(maintenance, override: "work", revision: 2), .unavailable, .notFound, query(maintenance, revision: 3)],
			writes: [clearReceipt(maintenance, revision: 3)]
		)
		let controller = makeController(transport)
		await controller.open()
		await findMaintenance(controller)
		await controller.selectOrigin(.automatic)
		guard case let .found(failed) = controller.lookup else { return XCTFail("the card stays") }
		XCTAssertEqual(failed.classificationQuery, .failed)
		XCTAssertFalse(failed.classificationAvailable)
		XCTAssertNil(OriginSettingsCopy.automaticText(failed.automatic))
		XCTAssertEqual(failed.revision, 3, "the next choice is based on the clear's revision")

		await controller.readCurrentClassification()
		guard case let .found(excluded) = controller.lookup else { return XCTFail("a hidden session keeps the card, unavailable") }
		XCTAssertEqual(excluded.classificationQuery, .failed)

		await controller.readCurrentClassification()
		guard case let .found(queried) = controller.lookup else { return XCTFail("found") }
		XCTAssertTrue(queried.classificationAvailable)
		XCTAssertEqual(queried.effective, "auxiliary")
	}

	/// The architecture's clear chain for an unmatched override whose session
	/// is gone: the row's own key and revision, no session lookup, and the
	/// list read again decides what remains.
	func testRemovingAnUnmatchedOverrideUsesItsKeyAndRevisionWithoutTheSession() async throws {
		let row = DesktopOriginOverrideV1(client: "codex", sessionID: "archived-1", value: "auxiliary", bindingStatus: "unmatched", revision: 4)
		let transport = StubOriginTransport(pages: [page([row]), page([])], writes: [clearReceipt(archived, revision: 5)])
		let controller = makeController(transport)
		await controller.open()
		XCTAssertEqual(controller.manualSummary,
			t(DesktopCopy.originOverridesSummaryCount, Int64(1)) + t(DesktopCopy.originOverridesSummaryUnmatched, Int64(1)))
		await controller.openOverride(row)
		var lookups = await transport.lookupCalls
		XCTAssertEqual(lookups, [], "an unmatched row is not opened as a session")
		await controller.removeOverride(row)
		let writes = await transport.writes
		XCTAssertEqual(writes, [StubOriginTransport.Write(action: .clearOverride(archived), expectedRevision: 4, requestID: "settings-test-1")])
		lookups = await transport.lookupCalls
		XCTAssertEqual(lookups, [], "removing an override never needs a visible session")
		XCTAssertEqual(controller.overrides, .loaded(DesktopOriginOverridePageV1(items: [], page: 1, limit: 20, total: 0)))
		XCTAssertEqual(controller.manualSummary, t(DesktopCopy.originOverridesSummaryNone))
		XCTAssertEqual(controller.statusLine(for: .manualClassification), OriginStatusLine(tone: .info, text: t(DesktopCopy.originManualPublicationPending)))
		XCTAssertEqual(OriginSettingsCopy.focusAfterRemoval(of: archived, at: 0, remaining: []), .findSession)
	}

	/// A matched row opens its session in the lookup.
	func testAMatchedRowOpensItsSession() async {
		let row = DesktopOriginOverrideV1(client: "claude", sessionID: "maint-1", value: "work", bindingStatus: "matched", revision: 2)
		let transport = StubOriginTransport(pages: [page([row])], lookups: [query(maintenance, override: "work", revision: 2)])
		let controller = makeController(transport)
		await controller.open()
		await controller.openOverride(row)
		XCTAssertEqual(controller.lookupClient, "claude")
		XCTAssertEqual(controller.lookupID, "maint-1")
		guard case let .found(result) = controller.lookup else { return XCTFail("found") }
		XCTAssertEqual(result.choice, .work)
	}

	/// Replaying an old clear's request returns its old receipt; the list read
	/// afterwards still shows the newer override, and nothing is removed from
	/// the receipt alone.
	func testAnOldReplayedClearCannotRemoveANewerOverride() async {
		let older = DesktopOriginOverrideV1(client: "codex", sessionID: "archived-1", value: "auxiliary", bindingStatus: "unmatched", revision: 4)
		let newer = DesktopOriginOverrideV1(client: "codex", sessionID: "archived-1", value: "work", bindingStatus: "matched", revision: 6)
		let transport = StubOriginTransport(
			pages: [page([older]), page([newer])],
			writes: [.unknown, clearReceipt(archived, revision: 5, replayed: true)],
			receipts: [.unavailable]
		)
		let controller = makeController(transport)
		await controller.open()
		await controller.removeOverride(older)
		XCTAssertEqual(controller.status, .unknown)
		XCTAssertTrue(controller.isRemoving(older), "the row stays in place with its removal unresolved")
		XCTAssertEqual(controller.overrides, page([older]).asListState, "an unconfirmed removal does not remove the row")
		await controller.readRequestReceipt()
		XCTAssertTrue(controller.retryAvailable, "an unreadable receipt store stays unknown")
		await controller.retry()
		XCTAssertEqual(controller.notice, .replayed)
		XCTAssertFalse(controller.isRemoving(newer))
		XCTAssertEqual(controller.overrides, .loaded(DesktopOriginOverridePageV1(items: [newer], page: 1, limit: 20, total: 1)))
		XCTAssertEqual(OriginSettingsCopy.focusAfterRemoval(of: archived, at: 0, remaining: [newer]), .removeOverride(archived), "the row is still there")
	}

	func testAnUnreadableListIsNeverEmptyAndARemovalThatEmptiesALastPageStepsBack() async {
		let transport = StubOriginTransport(pages: [.unavailable])
		let controller = makeController(transport)
		await controller.open()
		XCTAssertEqual(controller.overrides, .unavailable)
		XCTAssertEqual(controller.status, .loadFailed)
		XCTAssertEqual(controller.manualSummary, t(DesktopCopy.originOverridesSummaryUnavailable))
		XCTAssertFalse(controller.writesEnabled)

		let rows = (0 ..< 21).map { DesktopOriginOverrideV1(client: "codex", sessionID: String(format: "s-%02d", $0), value: "work", bindingStatus: "matched", revision: 1) }
		let paged = StubOriginTransport(pages: [
			page(Array(rows.prefix(20)), total: 21),
			page([rows[20]], page: 2, total: 21),
			page([], page: 2, total: 20),
			page(Array(rows.prefix(20)), total: 20),
		], writes: [.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .overrideClear, target: rows[20].target, outcome: .saved, value: nil, revision: 2))])
		let listed = makeController(paged)
		await listed.open()
		XCTAssertEqual(listed.manualSummary, t(DesktopCopy.originOverridesSummaryCount, Int64(21)), "the whole list's count, from the page total")
		await listed.showOverridePage(2)
		XCTAssertEqual(listed.overridePageNumber, 2)
		await listed.removeOverride(rows[20])
		let pageReads = await paged.pageReads
		XCTAssertEqual(pageReads, [1, 2, 2, 1])
		XCTAssertEqual(listed.overridePageNumber, 1)
		guard case let .loaded(current) = listed.overrides else { return XCTFail("loaded") }
		XCTAssertEqual(current.items.count, 20)
		XCTAssertEqual(OriginSettingsCopy.focusAfterRemoval(of: rows[20].target, at: 0, remaining: current.items), .removeOverride(rows[0].target))
		XCTAssertEqual(OriginSettingsCopy.focusAfterRemoval(of: rows[0].target, at: 25, remaining: Array(rows[1 ..< 4])), .removeOverride(rows[3].target), "the previous row when the last was removed")
	}

	// MARK: Where each status shows

	/// A write's status shows under the control that started it; the other
	/// control only says why it is locked. While the sheet is closed, the
	/// Manual classification row still reports a write in flight or a pending
	/// publication, but not a settled result.
	func testEachWriteReportsUnderTheControlThatStartedIt() async {
		let transport = StubOriginTransport(
			lookups: [query(maintenance, revision: 1)],
			writes: [
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .overrideSet, target: maintenance, outcome: .saved, value: "auxiliary", revision: 2, publicationPending: true)),
				.committed(DesktopOriginReceiptV1(requestID: "settings-test-2", kind: .scopeSet, outcome: .saved, value: "all", revision: 1, publicationPending: true)),
			]
		)
		let publications = OriginTestBox<[OriginPublicationResult]>([.failed, .published(.confirmed(.all, revision: 1))])
		let controller = makeController(transport) { publications.value.removeFirst() }
		await controller.open()
		await findMaintenance(controller)
		await transport.setHoldWrites(true)
		let set = Task { await controller.selectOrigin(.auxiliary) }
		await waitUntilHeld(transport)
		let saving = OriginStatusLine(tone: .busy, text: t(DesktopCopy.originManualSaving, "maint-1", t(DesktopCopy.sessionScopeAuxiliary)))
		XCTAssertEqual(controller.statusLine(for: .manualClassification), saving)
		XCTAssertEqual(controller.manualRowStatus, saving, "a closed sheet still reports the write")
		XCTAssertEqual(controller.sheetStatus, saving)
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .neutral, text: t(DesktopCopy.originLockedByManual)))
		await transport.releaseWrite()
		await set.value
		await controller.publicationTask?.value
		let pending = OriginStatusLine(tone: .info, text: t(DesktopCopy.originManualPublicationPending))
		XCTAssertEqual(controller.manualRowStatus, pending, "a pending publication is still reported")
		XCTAssertNil(controller.statusLine(for: .defaultScope))

		await transport.setHoldWrites(false)
		await controller.selectDefault(.all)
		await controller.publicationTask?.value
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .good, text: t(DesktopCopy.originDefaultSaved)))
		XCTAssertNil(controller.statusLine(for: .manualClassification), "a default result does not show under the manual control")
		XCTAssertNil(controller.manualRowStatus)
	}

	/// The sheet says plainly when the core is unreadable; the main surface
	/// shows the failure once, under the default.
	func testAnUnreadableCoreIsSaidInTheSheetAndNowhereElseTwice() async {
		let transport = StubOriginTransport(defaults: [.unavailable], pages: [.unavailable])
		let controller = makeController(transport)
		await controller.open()
		XCTAssertEqual(controller.sheetStatus, OriginStatusLine(tone: .bad, text: t(DesktopCopy.originSheetCoreUnavailable)))
		XCTAssertNil(controller.manualRowStatus)
		XCTAssertEqual(controller.manualSummary, t(DesktopCopy.originOverridesSummaryUnavailable))
		XCTAssertFalse(controller.lookupEnabled)
	}

	// MARK: An explicit core failure (ASC-SC-R4-F1)

	private var coreDown: DesktopOriginSourceStatusesV1 {
		DesktopOriginSourceStatusesV1(claudeMem: claudeMem(available: false, reason: "core_unavailable"),
			codexBackground: codex(available: false, reason: "core_unavailable", count: nil), partial: true)
	}

	private var healthy: DesktopOriginSourceStatusesV1 {
		DesktopOriginSourceStatusesV1(claudeMem: claudeMem(available: true, reason: nil, version: "12.1.0"),
			codexBackground: codex(available: true, reason: nil, count: 1), partial: false)
	}

	/// The default and the list read fine, then the status read that follows
	/// reports the core unreadable: the group is unreadable, nothing can be
	/// found or written, and a successful read again restores it.
	func testACoreFailureReportedAfterSuccessfulReadsMakesTheGroupUnreadable() async {
		let row = DesktopOriginOverrideV1(client: "codex", sessionID: "existing-session", value: "work", bindingStatus: "matched", revision: 2)
		let transport = StubOriginTransport(defaults: [.value(.confirmedWork(revision: 0))], pages: [page([row])], statuses: [.value(coreDown), .value(healthy)])
		let controller = makeController(transport)
		await controller.open()
		controller.setLookupID("existing-session")
		XCTAssertEqual(controller.sourceNote, .coreUnavailable)
		XCTAssertEqual(controller.status, .loadFailed)
		XCTAssertNil(controller.defaultState.scope, "the default read before the failure is no longer current")
		XCTAssertNil(controller.selectedDefault)
		XCTAssertEqual(controller.overrides, .unavailable, "nor is the list")
		XCTAssertEqual(controller.manualSummary, t(DesktopCopy.originOverridesSummaryUnavailable))
		XCTAssertEqual(controller.statusLine(for: .defaultScope), OriginStatusLine(tone: .bad, text: t(DesktopCopy.originDefaultReadFailed), actions: [.readAgain]))
		XCTAssertEqual(controller.sheetStatus, OriginStatusLine(tone: .bad, text: t(DesktopCopy.originSheetCoreUnavailable)))
		XCTAssertFalse(controller.writesEnabled)
		XCTAssertFalse(controller.lookupEnabled)
		await controller.selectDefault(.auxiliary)
		await controller.removeOverride(row)
		await controller.openOverride(row)
		await controller.findSession()
		let writes = await transport.writes
		XCTAssertEqual(writes, [], "nothing is written against values the core no longer confirms")
		let lookups = await transport.lookupCalls
		XCTAssertEqual(lookups, [], "and nothing is looked up")

		await controller.readLatestValues()
		XCTAssertEqual(controller.status, .ready, "a successful read again restores the group")
		XCTAssertEqual(controller.selectedDefault, .work)
		XCTAssertEqual(controller.overrides, page([row]).asListState)
		XCTAssertNil(controller.sourceNote)
		XCTAssertTrue(controller.lookupEnabled)
	}

	/// Only the core failing locks the group; an adapter outage does not.
	func testAnAdapterOutageAloneLeavesTheGroupWritable() async {
		let statuses = DesktopOriginSourceStatusesV1(claudeMem: claudeMem(available: false, reason: "source_unavailable"),
			codexBackground: codex(available: true, reason: nil, count: 1), partial: true)
		let controller = makeController(StubOriginTransport(statuses: [.value(statuses)]))
		await controller.open()
		XCTAssertEqual(controller.sourceNote, .adapterUnavailable)
		XCTAssertEqual(controller.status, .ready)
		XCTAssertEqual(controller.selectedDefault, .work)
		XCTAssertTrue(controller.writesEnabled)
	}

	/// Reads that started before the core was reported unreadable return late:
	/// neither the default nor a lookup comes back from them. The pending
	/// operation keeps its request throughout.
	func testReadsThatPredateACoreFailureCannotBringOldValuesBack() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 0))],
			pages: [page([])],
			statuses: [.value(healthy), .value(coreDown), .value(healthy), .value(healthy), .value(coreDown)],
			lookups: [query(maintenance, revision: 1)],
			writes: [.conflict(currentRevision: 1)]
		)
		let controller = makeController(transport)
		await controller.open()
		await controller.selectDefault(.all)
		XCTAssertEqual(controller.status, .conflict)

		// Loading the latest values; its default read is still in flight when
		// a reopen, which reads only the source status during a conflict,
		// learns that the core is unreadable.
		await transport.holdNext(.defaultScope)
		let reload = Task { await controller.readLatestValues() }
		await waitUntilHolding(transport, .defaultScope)
		await controller.open()
		XCTAssertEqual(controller.operation, .conflict(OriginPendingOperation(action: .saveDefault(.all), expectedRevision: 0, requestID: "settings-test-1")),
			"the pending operation keeps its request and its way out")
		XCTAssertEqual(controller.statusLine(for: .defaultScope)?.actions, [.loadLatest])
		XCTAssertNil(controller.selectedDefault)
		await transport.release(.defaultScope)
		await reload.value
		XCTAssertEqual(controller.defaultState, .unavailable, "the default read that began before the failure does not restore Work")
		XCTAssertEqual(controller.status, .loadFailed)
		XCTAssertFalse(controller.writesEnabled)

		await controller.readLatestValues()
		XCTAssertEqual(controller.status, .ready, "a read begun after the failure restores it")

		// A lookup still in flight when the core is reported unreadable shows nothing.
		controller.setLookupClient("claude")
		controller.setLookupID("maint-1")
		await transport.holdNext(.lookup)
		let find = Task { await controller.findSession() }
		await waitUntilHolding(transport, .lookup)
		await controller.readLatestValues()
		XCTAssertEqual(controller.status, .loadFailed)
		await transport.release(.lookup)
		await find.value
		XCTAssertEqual(controller.lookup, .idle, "the older lookup does not show a session the core cannot confirm")
		let writes = await transport.writes
		XCTAssertEqual(writes.count, 1)
	}

	/// ASC-SC-R4-F1 (Round 5): the status read began first; a reopen's default
	/// read and a page read started afterwards and are still in flight when it
	/// reports the core unreadable. A read that has only started confirms
	/// nothing, so the group is unreadable and unwritable at once, and those
	/// reads cannot restore it when they return, even with a page.
	func testReadsStillInFlightWhenTheCoreIsReportedUnreadableDoNotKeepTheGroupWritable() async {
		let rows = (0 ..< 20).map { DesktopOriginOverrideV1(client: "codex", sessionID: String(format: "s-%02d", $0), value: "work", bindingStatus: "matched", revision: 1) }
		let last = DesktopOriginOverrideV1(client: "codex", sessionID: "s-20", value: "auxiliary", bindingStatus: "matched", revision: 1)
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 0))],
			pages: [page(rows, total: 21), page(rows, total: 21), page([last], page: 2, total: 21)],
			statuses: [.value(healthy), .value(coreDown)]
		)
		let controller = makeController(transport)
		await controller.open()
		XCTAssertEqual(controller.status, .ready)

		await transport.holdNext(.sourceStatus)
		let reported = Task { await controller.open() }
		await waitUntilHolding(transport, .sourceStatus)
		await transport.holdNext(.defaultScope)
		let reopen = Task { await controller.open() }
		await waitUntilHolding(transport, .defaultScope)
		await transport.holdNext(.overridePage)
		let paging = Task { await controller.showOverridePage(2) }
		await waitUntilHolding(transport, .overridePage)

		await transport.release(.sourceStatus)
		await reported.value
		let stillInFlight = await (transport.isHolding(.defaultScope), transport.isHolding(.overridePage))
		XCTAssertTrue(stillInFlight.0 && stillInFlight.1, "both newer reads are still in flight")
		XCTAssertEqual(controller.sourceNote, .coreUnavailable)
		XCTAssertEqual(controller.status, .loadFailed, "unreadable as soon as the failure is reported")
		XCTAssertFalse(controller.writesEnabled)
		XCTAssertNil(controller.selectedDefault)
		XCTAssertEqual(controller.overrides, .unavailable)
		XCTAssertEqual(controller.sheetStatus, OriginStatusLine(tone: .bad, text: t(DesktopCopy.originSheetCoreUnavailable)))
		controller.setLookupID("existing-session")
		XCTAssertFalse(controller.lookupEnabled)
		await controller.selectDefault(.auxiliary)
		let writes = await transport.writes
		XCTAssertEqual(writes, [], "no write goes out while the newer reads are pending")

		await transport.release(.overridePage)
		await paging.value
		XCTAssertEqual(controller.overrides, .unavailable, "a page read in flight at the report does not restore the list")
		XCTAssertFalse(controller.writesEnabled)
		await transport.release(.defaultScope)
		await reopen.value
		XCTAssertEqual(controller.status, .loadFailed, "the core is still reported unreadable")
		XCTAssertNil(controller.selectedDefault)
	}

	/// A status read that began before a newer confirmed value returns late
	/// with a core failure: the newer value stays, what predates the report
	/// does not, and the group stays unreadable until read again.
	func testALateCoreFailureKeepsOnlyValuesConfirmedAfterItBegan() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 0))],
			statuses: [.value(healthy), .value(coreDown), .value(healthy)],
			writes: [.committed(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .saved, value: "auxiliary", revision: 1, publicationPending: true))]
		)
		let controller = makeController(transport)
		await controller.open()
		await transport.holdNext(.sourceStatus)
		let reopen = Task { await controller.open() }
		await waitUntilHolding(transport, .sourceStatus)
		await controller.selectDefault(.auxiliary)
		XCTAssertEqual(controller.defaultState, .confirmed(.auxiliary, revision: 1))
		await transport.release(.sourceStatus)
		await reopen.value
		XCTAssertEqual(controller.defaultState, .confirmed(.auxiliary, revision: 1), "a value confirmed after the status read began is newer than its report")
		XCTAssertEqual(controller.overrides, .unavailable, "the list read before it is not")
		XCTAssertEqual(controller.status, .loadFailed)
		XCTAssertNil(controller.selectedDefault, "nothing is selected while the group is unreadable")

		await controller.readLatestValues()
		XCTAssertEqual(controller.status, .ready)
		XCTAssertEqual(controller.selectedDefault, .work, "the read again shows the stored value")
	}

	/// After a write or read-back answers while the core is reported
	/// unreadable, everything is read again so the note and the group agree.
	func testAResultWhileTheCoreIsReportedUnreadableReadsEverythingAgain() async {
		let transport = StubOriginTransport(
			defaults: [.value(.confirmedWork(revision: 0)), .value(.confirmed(.auxiliary, revision: 1))],
			statuses: [.value(healthy), .value(coreDown), .value(healthy)],
			writes: [.unknown],
			receipts: [.known(DesktopOriginReceiptV1(requestID: "settings-test-1", kind: .scopeSet, outcome: .saved, value: "auxiliary", revision: 1))]
		)
		let controller = makeController(transport)
		await controller.open()
		await controller.selectDefault(.auxiliary)
		await controller.open()
		XCTAssertEqual(controller.sourceNote, .coreUnavailable)
		XCTAssertEqual(controller.statusLine(for: .defaultScope),
			OriginStatusLine(tone: .warning, text: t(DesktopCopy.originUnknown), requestID: "settings-test-1", actions: [.checkResult]),
			"an unknown outcome keeps its request ID and its check while the core is unreadable")
		await controller.readRequestReceipt()
		XCTAssertNil(controller.sourceNote, "the source status was read again")
		XCTAssertEqual(controller.status, .ready)
		XCTAssertEqual(controller.defaultState, .confirmed(.auxiliary, revision: 1))
		XCTAssertTrue(controller.writesEnabled)
	}

	func testTheSheetIsAWindowDraft() async {
		let controller = makeController(StubOriginTransport())
		await controller.open()
		controller.openManage()
		XCTAssertTrue(controller.isManaging)
		controller.setLookupID("draft")
		controller.windowClosed()
		XCTAssertFalse(controller.isManaging, "closing the window closes the sheet")
		XCTAssertEqual(controller.lookupID, "")
		controller.openManage()
		controller.closeManage()
		XCTAssertFalse(controller.isManaging)
	}

	// MARK: Sources

	private func claudeMem(available: Bool, reason: String?, configured: Bool? = true, enabled: Bool? = true, version: String? = nil) -> DesktopOriginSourceStatusV1 {
		DesktopOriginSourceStatusV1(source: "claude-mem", kind: "adapter", configured: configured, enabled: enabled, available: available,
			supportedVersion: version, ruleVersion: "claude-mem-sdk-map-v1", reasonCode: reason)
	}

	private func codex(available: Bool, reason: String?, count: Int?, receiver: String = "enabled") -> DesktopOriginSourceStatusV1 {
		DesktopOriginSourceStatusV1(source: "codex-otel-background", kind: "builtin", available: available,
			ruleVersion: "codex-otel-background-v1", reasonCode: reason, receiver: receiver, backgroundSessions: count)
	}

	/// The built-in row's five states, and an unknown code that is never shown
	/// as available (ux/settings.md 自动识别).
	func testTheCodexBackgroundRowFollowsItsFiveStates() {
		func row(_ status: DesktopOriginSourceStatusV1) -> OriginSourceRow { OriginSettingsCopy.codexBackgroundRow(status) }
		let name = t(DesktopCopy.originCodexName)
		XCTAssertEqual(row(codex(available: true, reason: nil, count: 3, receiver: "disabled")),
			OriginSourceRow(name: name, tone: .good, text: t(DesktopCopy.originCodexRecords, Int64(3)), code: "available"),
			"records stay available after capture is turned off")
		XCTAssertEqual(row(codex(available: true, reason: "otel_receiver_disabled", count: 0, receiver: "disabled")),
			OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originCodexReceiverOff), code: "otel_receiver_disabled"))
		XCTAssertEqual(row(codex(available: true, reason: "no_background_sessions", count: 0)),
			OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originCodexNoRecords), code: "no_background_sessions"))
		XCTAssertEqual(row(codex(available: true, reason: "otel_receiver_unknown", count: 0, receiver: "unknown")),
			OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originCodexReceiverUnknown), code: "otel_receiver_unknown"))
		XCTAssertEqual(row(codex(available: false, reason: "core_unavailable", count: nil)),
			OriginSourceRow(name: name, tone: .bad, text: t(DesktopCopy.originSourceCoreUnavailable), code: "core_unavailable"))
		XCTAssertEqual(row(codex(available: true, reason: "a_future_reason", count: 0)),
			OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originSourceReasonUnknown), code: "a_future_reason"))
		for reason in ["otel_receiver_disabled", "no_background_sessions", "otel_receiver_unknown"] {
			XCTAssertNotEqual(row(codex(available: true, reason: reason, count: 0)).tone, .bad, "no-records states are not errors: \(reason)")
		}
	}

	func testTheClaudeMemRowShowsEachReasonAndNeverCallsAnUnknownCodeAvailable() {
		let reasons: [(String, OriginStatusLine.Tone, String)] = [
			("source_not_configured", .neutral, DesktopCopy.originSourceNotConfigured),
			("source_unavailable", .warning, DesktopCopy.originSourceMetadataUnreadable),
			("source_version_unsupported", .warning, DesktopCopy.originSourceVersionUnsupported),
			("source_disabled", .neutral, DesktopCopy.originSourceDisabled),
			("source_not_yet_read", .neutral, DesktopCopy.originSourceNotYetRead),
			("source_config_unreadable", .warning, DesktopCopy.originSourceConfigUnreadable),
			("core_unavailable", .bad, DesktopCopy.originSourceCoreUnavailable),
			("a_future_reason", .neutral, DesktopCopy.originSourceReasonUnknown),
		]
		for (code, tone, key) in reasons {
			let row = OriginSettingsCopy.claudeMemRow(claudeMem(available: code == "a_future_reason", reason: code))
			XCTAssertEqual(row, OriginSourceRow(name: "claude-mem", tone: tone, text: t(key), code: code), code)
		}
		XCTAssertEqual(OriginSettingsCopy.claudeMemRow(claudeMem(available: true, reason: nil, version: "12.1.0")),
			OriginSourceRow(name: "claude-mem", tone: .good, text: t(DesktopCopy.originSourceAvailableVersion, "12.1.0"), code: "available"))
	}

	/// An adapter outage leaves manual classification working; an unreadable
	/// core says that nothing can be read or changed.
	func testTheSourceNoteSeparatesAnAdapterOutageFromAnUnreadableCore() async {
		func note(_ statuses: DesktopOriginReadOutcome<DesktopOriginSourceStatusesV1>) async -> OriginSourceNote? {
			let controller = makeController(StubOriginTransport(statuses: [statuses]))
			await controller.open()
			return controller.sourceNote
		}
		let records = codex(available: true, reason: nil, count: 1)
		let available = await note(.value(DesktopOriginSourceStatusesV1(claudeMem: claudeMem(available: true, reason: nil, version: "12.1.0"), codexBackground: records, partial: false)))
		XCTAssertNil(available)
		let adapter = await note(.value(DesktopOriginSourceStatusesV1(claudeMem: claudeMem(available: false, reason: "source_unavailable"), codexBackground: records, partial: true)))
		XCTAssertEqual(adapter, .adapterUnavailable)
		// claude-mem checks its configuration before the core, so the built-in
		// row alone can be the one that names the unreadable core.
		let core = await note(.value(DesktopOriginSourceStatusesV1(claudeMem: claudeMem(available: false, reason: "source_not_configured"),
			codexBackground: codex(available: false, reason: "core_unavailable", count: nil), partial: true)))
		XCTAssertEqual(core, .coreUnavailable)
		let unreadable = await note(.unavailable)
		XCTAssertEqual(unreadable, .statusUnavailable)
		XCTAssertNotEqual(t(DesktopCopy.originSourceNoteCore), t(DesktopCopy.originSourceNoteAdapter))
	}

	// MARK: Window and rendering

	/// The real window opens by reading the latest values and its close drops
	/// only the drafts.
	func testTheSettingsWindowOpensWithAReadAndClosingKeepsTheOperation() async throws {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let transport = StubOriginTransport(defaults: [.value(.confirmedWork(revision: 1))], writes: [.unknown])
		let controller = makeController(transport)
		let window = SettingsWindowController(
			preferences: preferences, quotaSettings: makeQuotaSettingsController(preferences: preferences),
			telemetrySettings: makeTelemetrySettingsController(), sessionOrigin: controller
		)
		window.show()
		let shown = try XCTUnwrap(window.window)
		for _ in 0 ..< 400 where controller.status == .loading || controller.sources == .loading { await Task.yield() }
		XCTAssertEqual(controller.defaultState, .confirmed(.work, revision: 1))
		XCTAssertEqual(controller.status, .ready, "opening read the default and the override list")
		await controller.selectDefault(.all)
		controller.setLookupID("draft-id")
		shown.performClose(nil)
		XCTAssertFalse(shown.isVisible)
		XCTAssertEqual(controller.lookupID, "", "closing discards the lookup draft")
		XCTAssertFalse(controller.isManaging)
		XCTAssertEqual(controller.status, .unknown, "the unresolved request stays with the App")
		XCTAssertLessThanOrEqual(shown.frame.height, (NSScreen.main?.visibleFrame.height ?? 800) + 1, "the window body scrolls rather than outgrowing the screen")
		shown.close()
	}

	/// Native renderings in both languages at the window's 460pt width: the
	/// group confirmed with an unmatched override, after an unknown save and
	/// with an unreadable core; and the sheet with a card and the list, and
	/// with an unreadable core. Nothing may be wider than the window.
	func testTheGroupAndSheetRenderInBothLanguages() async throws {
		let previous = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		defer {
			if let previous { setenv("AGENTDECK_TEST_LOCALE", previous, 1) } else { unsetenv("AGENTDECK_TEST_LOCALE") }
		}
		let statuses = DesktopOriginSourceStatusesV1(
			claudeMem: claudeMem(available: false, reason: "source_not_configured", configured: false, enabled: false),
			codexBackground: codex(available: true, reason: nil, count: 1), partial: false
		)
		let coreDown = DesktopOriginSourceStatusesV1(claudeMem: claudeMem(available: false, reason: "core_unavailable"),
			codexBackground: codex(available: false, reason: "core_unavailable", count: nil), partial: true)
		let rows = [
			DesktopOriginOverrideV1(client: "claude", sessionID: "old-01", value: "work", bindingStatus: "matched", revision: 2),
			DesktopOriginOverrideV1(client: "codex", sessionID: "archived-1-with-a-long-canonical-identifier-0123456789abcdef", value: "auxiliary", bindingStatus: "unmatched", revision: 4),
		]
		for language in ["en", "zh-Hans"] {
			setenv("AGENTDECK_TEST_LOCALE", language, 1)
			let normal = makeController(StubOriginTransport(pages: [page(rows)], statuses: [.value(statuses)],
				lookups: [query(maintenance, override: "work", revision: 2)]))
			await normal.open()
			await findMaintenance(normal)
			let unknown = makeController(StubOriginTransport(pages: [page(rows)], statuses: [.value(statuses)], writes: [.unknown]))
			await unknown.open()
			await unknown.selectDefault(.auxiliary)
			let failed = makeController(StubOriginTransport(defaults: [.unavailable], pages: [.unavailable], statuses: [.value(coreDown)]))
			await failed.open()
			let renderings: [(String, AnyView)] = [
				("group normal", AnyView(SessionOriginSettingsGroup(controller: normal))),
				("group unknown", AnyView(SessionOriginSettingsGroup(controller: unknown))),
				("group core unavailable", AnyView(SessionOriginSettingsGroup(controller: failed))),
				("sheet normal", AnyView(OriginManageSheet(controller: normal))),
				("sheet core unavailable", AnyView(OriginManageSheet(controller: failed))),
			]
			for (state, view) in renderings {
				let sheet = state.hasPrefix("sheet")
				let root = sheet ? view : AnyView(view.padding(18).frame(width: 460).background(DesktopVisualTheme.background))
				let hosting = NSHostingView(rootView: root)
				hosting.frame = NSRect(x: 0, y: 0, width: 460, height: sheet ? OriginManageSheet.height : 1600)
				hosting.layoutSubtreeIfNeeded()
				XCTAssertLessThanOrEqual(hosting.fittingSize.width, 461, "\(language) \(state)")
				if !sheet { hosting.frame.size.height = hosting.fittingSize.height }
				hosting.layoutSubtreeIfNeeded()
				hosting.displayIfNeeded()
				let representation = try XCTUnwrap(hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds))
				hosting.cacheDisplay(in: hosting.bounds, to: representation)
				// Content wider than its column is pushed into the 18pt padding;
				// the outer 12pt on each side must stay background, apart from
				// the sheet's full-width dividers.
				XCTAssertTrue(marginsAreBlank(representation, points: 12, viewWidth: hosting.bounds.width), "\(language) \(state): content overflows the width")
				if !sheet {
					// The measured height must be the natural one: laid out at the
					// top of a taller canvas, the same rows render identically.
					// A short measurement squeezes rows into each other.
					let tall = NSHostingView(rootView: VStack(spacing: 0) { root; Spacer(minLength: 0) })
					tall.frame = NSRect(x: 0, y: 0, width: 460, height: hosting.bounds.height + 240)
					tall.layoutSubtreeIfNeeded()
					tall.displayIfNeeded()
					let tallRepresentation = try XCTUnwrap(tall.bitmapImageRepForCachingDisplay(in: tall.bounds))
					tall.cacheDisplay(in: tall.bounds, to: tallRepresentation)
					XCTAssertTrue(sameTopRows(representation, tallRepresentation), "\(language) \(state): the measured height squeezes the rows")
				}
				let png = try XCTUnwrap(representation.representation(using: .png, properties: [:]))
				XCTAssertGreaterThan(png.count, 8_000, "\(language) \(state)")
				let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
				attachment.name = "Sessions — \(state) — \(language) — 460pt"
				attachment.lifetime = .keepAlways
				add(attachment)
			}
		}
	}

	// MARK: Real helper

	/// The bridge against the embedded Go helper in an isolated, empty state
	/// root: every Swift DTO decodes what the CLI actually writes, and the
	/// idempotency and CAS contract holds end to end. No real state is read.
	func testTheEmbeddedHelperRoundTripsTheSettingsContractInAnIsolatedStateRoot() async throws {
		let testHome = try XCTUnwrap(ProcessInfo.processInfo.environment["AGENTDECK_TEST_HOME"], "hosted tests run with an isolated home")
		XCTAssertTrue(testHome.hasPrefix("/private/tmp/agentdeck-macos-xctest.") || testHome.hasPrefix("/tmp/agentdeck-macos-xctest."))
		let home = URL(fileURLWithPath: testHome, isDirectory: true).appendingPathComponent("origin-bridge-\(UUID().uuidString)", isDirectory: true)
		try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: home) }
		let runner = EmbeddedHelperRunner(
			appBundleURL: Bundle.main.bundleURL,
			environment: ["HOME": home.path, "LANG": "en_US_POSIX", "LC_ALL": "en_US_POSIX", "PATH": "/usr/bin:/bin"]
		)
		guard (try? runner.embeddedHelperURL()) != nil else { throw XCTSkip("the test host has no embedded helper") }

		guard case let .value(sources) = await runner.readOriginSourceStatus() else { return XCTFail("status reads without a core database") }
		XCTAssertEqual(sources.claudeMem.reasonCode, "source_not_configured")
		XCTAssertEqual(sources.codexBackground.reasonCode, "core_unavailable")
		XCTAssertNil(sources.codexBackground.backgroundSessions)
		let missingCore = await runner.readDefaultScope()
		XCTAssertEqual(missingCore, .unavailable, "no core database is not a confirmed Work default")

		let first = await runner.saveDefaultScope(.auxiliary, expectedRevision: 0, requestID: "e2e-scope-1")
		XCTAssertEqual(first, .committed(DesktopOriginReceiptV1(requestID: "e2e-scope-1", kind: .scopeSet, outcome: .saved, value: "auxiliary", revision: 1, publicationPending: true)))
		let confirmed = await runner.readDefaultScope()
		XCTAssertEqual(confirmed, .value(.confirmed(.auxiliary, revision: 1)))
		let same = await runner.saveDefaultScope(.auxiliary, expectedRevision: 1, requestID: "e2e-scope-2")
		XCTAssertEqual(same, .committed(DesktopOriginReceiptV1(requestID: "e2e-scope-2", kind: .scopeSet, outcome: .noChange, value: "auxiliary", revision: 1)))
		let stale = await runner.saveDefaultScope(.work, expectedRevision: 0, requestID: "e2e-scope-3")
		guard case .conflict = stale else { return XCTFail("a stale revision conflicts: \(stale)") }
		let replay = await runner.saveDefaultScope(.auxiliary, expectedRevision: 0, requestID: "e2e-scope-1")
		guard case let .committed(replayed) = replay else { return XCTFail("the same request replays: \(replay)") }
		XCTAssertTrue(replayed.replayed)
		XCTAssertEqual(replayed.revision, 1)
		let reused = await runner.saveDefaultScope(.all, expectedRevision: 1, requestID: "e2e-scope-1")
		XCTAssertEqual(reused, .invalidRequest, "a request ID with another payload never runs")

		guard case let .known(receipt) = await runner.readReceipt(requestID: "e2e-scope-1") else { return XCTFail("the receipt reads back") }
		XCTAssertEqual(receipt.kind, .scopeSet)
		XCTAssertEqual(receipt.revision, 1)
		let missingReceipt = await runner.readReceipt(requestID: "e2e-never-sent")
		XCTAssertEqual(missingReceipt, .unknown)

		let empty = await runner.listOverrides(page: 1, limit: SessionOriginCommand.overridePageLimit)
		XCTAssertEqual(empty, .value(DesktopOriginOverridePageV1(items: [], page: 1, limit: 20, total: 0)))
		let target = DesktopOriginTargetV1(client: "codex", sessionID: "e2e-never-present")
		let clear = await runner.clearOverride(target, expectedRevision: 0, requestID: "e2e-clear-1")
		XCTAssertEqual(clear, .committed(DesktopOriginReceiptV1(requestID: "e2e-clear-1", kind: .overrideClear, target: target, outcome: .noChange, value: nil, revision: 0,
			classificationReason: "not_queried")), "a never-present key clears as no change at revision 0")
		let noIndex = await runner.lookupSession(client: "codex", sessionID: "e2e-never-present")
		XCTAssertEqual(noIndex, .unavailable, "without a session index the lookup is unavailable, not not-found")
	}
}

private extension DesktopOriginReadOutcome where Value == DesktopOriginOverridePageV1 {
	var asListState: OriginOverrideListState {
		switch self {
		case let .value(page): .loaded(page)
		case .unavailable: .unavailable
		}
	}
}

// MARK: Pixel checks shared by the Settings renderings

/// Every row of `fitted` matches the same row of `tall`, which has more room
/// below; sampled every other pixel.
fileprivate func sameTopRows(_ fitted: NSBitmapImageRep, _ tall: NSBitmapImageRep) -> Bool {
	guard fitted.pixelsWide == tall.pixelsWide, fitted.pixelsHigh <= tall.pixelsHigh else { return false }
	for y in stride(from: 0, to: fitted.pixelsHigh, by: 2) {
		for x in stride(from: 0, to: fitted.pixelsWide, by: 2) {
			guard let a = fitted.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB),
				let b = tall.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB) else { return false }
			if abs(a.redComponent - b.redComponent) > 0.03 || abs(a.greenComponent - b.greenComponent) > 0.03
				|| abs(a.blueComponent - b.blueComponent) > 0.03 { return false }
		}
	}
	return true
}

/// The outer columns match the top-left background pixel, except rows a
/// full-width rule crosses (its colour reaches the centre too).
fileprivate func marginsAreBlank(_ bitmap: NSBitmapImageRep, points: CGFloat, viewWidth: CGFloat) -> Bool {
	let columns = Int((points * CGFloat(bitmap.pixelsWide) / viewWidth).rounded(.down))
	guard columns > 0, let reference = bitmap.colorAt(x: 0, y: 0)?.usingColorSpace(.deviceRGB) else { return false }
	func same(_ a: NSColor, _ b: NSColor) -> Bool {
		abs(a.redComponent - b.redComponent) <= 0.03 && abs(a.greenComponent - b.greenComponent) <= 0.03 && abs(a.blueComponent - b.blueComponent) <= 0.03
	}
	let xs = Array(0 ..< columns) + Array((bitmap.pixelsWide - columns) ..< bitmap.pixelsWide)
	for y in 0 ..< bitmap.pixelsHigh {
		guard let edge = bitmap.colorAt(x: 0, y: y)?.usingColorSpace(.deviceRGB),
			let centre = bitmap.colorAt(x: bitmap.pixelsWide / 2, y: y)?.usingColorSpace(.deviceRGB),
			let far = bitmap.colorAt(x: bitmap.pixelsWide - 1, y: y)?.usingColorSpace(.deviceRGB) else { return false }
		if !same(edge, reference), same(edge, centre), same(edge, far) { continue }
		for x in xs {
			guard let color = bitmap.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB) else { return false }
			if !same(color, reference) { return false }
		}
	}
	return true
}

// MARK: - Settings window pages

/// Counts status reads so a test can tell that switching pages read nothing
/// again; everything else is the shared stub.
final class CountingTelemetryTransport: TelemetryTransport, @unchecked Sendable {
	let base: StubTelemetryTransport
	private let lock = NSLock()
	private var reads = 0

	init(_ base: StubTelemetryTransport) { self.base = base }

	var statusReads: Int { lock.withLock { reads } }

	func loadTelemetryStatus() async -> DesktopQuotaTransportOutcome<DesktopTelemetryStatusV1> {
		lock.withLock { reads += 1 }
		return await base.loadTelemetryStatus()
	}

	func enableTelemetryReceiver(writeCodexConfig: Bool) async -> DesktopQuotaTransportOutcome<DesktopTelemetryReceiverResultV1> {
		await base.enableTelemetryReceiver(writeCodexConfig: writeCodexConfig)
	}

	func disableTelemetryReceiver() async -> DesktopQuotaTransportOutcome<DesktopTelemetryReceiverResultV1> {
		await base.disableTelemetryReceiver()
	}

	func loadTelemetryPreview() async -> DesktopQuotaTransportOutcome<DesktopTelemetryPreviewV1> {
		await base.loadTelemetryPreview()
	}
}

/// The native Settings window as toolbar pages (ux/settings.md 窗口分页 and
/// 原生整窗验收): page order, names and icons; title and height follow the
/// page; the last page comes back on reopen; the sheet holds the page; a
/// round trip through another page keeps every state still to be acted on;
/// and each whole page renders at its natural height without accent-filled
/// segmented controls.
@MainActor
final class SettingsWindowPaneTests: XCTestCase {
	private var opened: [SettingsWindowController] = []

	override func tearDown() async throws {
		for controller in opened { controller.window?.close() }
		opened = []
		try await super.tearDown()
	}

	private func telemetryStatus(enabled: Bool, exporter: String = "absent") -> DesktopTelemetryStatusV1 {
		DesktopTelemetryStatusV1(
			receiver: enabled ? "enabled" : "disabled", listener: enabled ? "running" : "stopped", producerConfigured: "unknown",
			lastReceivedAt: nil, lastRefusedAt: nil, metering: enabled ? "capture_only" : "disabled",
			codexMemories: "on", codexExporter: exporter
		)
	}

	private func makeWindow(
		origin: SessionOriginSettingsController? = SessionOriginSettingsController(
			transport: StubOriginTransport(), publish: { .failed }, newRequestID: { UUID().uuidString }
		),
		telemetry: TelemetrySettingsController = makeTelemetrySettingsController(),
		preferences: DesktopPreferences? = nil
	) -> SettingsWindowController {
		let preferences = preferences ?? DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let controller = SettingsWindowController(
			preferences: preferences, quotaSettings: makeQuotaSettingsController(preferences: preferences),
			telemetrySettings: telemetry, sessionOrigin: origin
		)
		opened.append(controller)
		return controller
	}

	private func settle(_ window: NSWindow?) {
		for _ in 0 ..< 8 {
			window?.contentView?.layoutSubtreeIfNeeded()
			RunLoop.main.run(until: Date().addingTimeInterval(0.03))
		}
	}

	private func contentHeight(_ window: NSWindow) -> CGFloat {
		window.contentView?.frame.height ?? 0
	}

	func testThePagesFollowThePrototypeInOrderNameAndIcon() throws {
		let controller = makeWindow()
		controller.show()
		let window = try XCTUnwrap(controller.window)
		let tabs = try XCTUnwrap(controller.tabs)
		XCTAssertEqual(tabs.tabStyle, .toolbar)
		XCTAssertEqual(window.toolbarStyle, .preference)
		XCTAssertEqual(tabs.tabViewItems.map { $0.identifier as? String }, ["general", "sessions", "liveUsage", "quota"])
		XCTAssertEqual(tabs.tabViewItems.map(\.label), [
			t(DesktopCopy.settingsGroupGeneral), t(DesktopCopy.panelSessions), t(DesktopCopy.settingsGroupLiveUsage), t(DesktopCopy.panelQuota),
		])
		XCTAssertEqual(SettingsPane.allCases.map(\.symbol), ["gearshape", "square.stack.3d.up", "waveform.path.ecg", "gauge.with.dots.needle.50percent"])
		XCTAssertTrue(tabs.tabViewItems.allSatisfy { $0.image != nil }, "every page has its SF Symbol")
		XCTAssertEqual(controller.selectedPane, .general, "the first open shows General")
		XCTAssertEqual(window.title, t(DesktopCopy.settingsGroupGeneral), "the title bar names the page")
		XCTAssertEqual(window.contentView?.frame.width ?? 0, 460, accuracy: 0.5)

		let withoutSessions = makeWindow(origin: nil)
		withoutSessions.show()
		XCTAssertEqual(withoutSessions.tabs?.tabViewItems.map { $0.identifier as? String }, ["general", "liveUsage", "quota"])
	}

	func testSwitchingChangesTitleAndHeightAndKeepsEveryPageInstance() throws {
		let controller = makeWindow()
		controller.show()
		let window = try XCTUnwrap(controller.window)
		settle(window)
		let instances = SettingsPane.allCases.map { controller.paneController($0).map(ObjectIdentifier.init) }
		var heights: [SettingsPane: CGFloat] = [:]
		for pane in [SettingsPane.quota, .sessions, .liveUsage, .general, .quota] {
			controller.select(pane)
			settle(window)
			XCTAssertEqual(controller.selectedPane, pane)
			XCTAssertEqual(window.title, pane.title)
			let natural = try XCTUnwrap(controller.naturalSize(of: pane))
			XCTAssertEqual(contentHeight(window), natural.height.rounded(.up), accuracy: 1, "\(pane): the window fits the page")
			heights[pane] = contentHeight(window)
		}
		XCTAssertGreaterThan(heights[.quota] ?? 0, heights[.liveUsage] ?? .infinity, "the height follows the page")
		XCTAssertEqual(
			SettingsPane.allCases.map { controller.paneController($0).map(ObjectIdentifier.init) }, instances,
			"switching pages never rebuilds one"
		)
	}

	func testReopeningReturnsToTheLastPageAndAPageOpensDirectly() throws {
		let controller = makeWindow()
		controller.show()
		controller.select(.quota)
		let window = try XCTUnwrap(controller.window)
		window.performClose(nil)
		XCTAssertFalse(window.isVisible)
		controller.show()
		XCTAssertEqual(controller.selectedPane, .quota, "a reopened window comes back to the page shown last")
		XCTAssertEqual(controller.tabs?.selectedTabViewItemIndex, 3)
		XCTAssertEqual(window.title, SettingsPane.quota.title)
		controller.show(pane: .liveUsage)
		XCTAssertEqual(controller.selectedPane, .liveUsage, "another surface can open one page by its key")

		let fresh = makeWindow()
		fresh.show()
		XCTAssertEqual(fresh.selectedPane, .general)
	}

	func testTheSheetHoldsThePage() throws {
		let origin = SessionOriginSettingsController(transport: StubOriginTransport(), publish: { .failed }, newRequestID: { UUID().uuidString })
		let controller = makeWindow(origin: origin)
		controller.show(pane: .sessions)
		let window = try XCTUnwrap(controller.window)
		origin.openManage()
		settle(window)
		XCTAssertNotNil(window.attachedSheet, "the manual-classification sheet is presented on the Settings window")
		controller.select(.quota)
		XCTAssertEqual(controller.selectedPane, .sessions, "no other page while the sheet is open")
		XCTAssertEqual(controller.tabs?.selectedTabViewItemIndex, 1)
		// The toolbar's own action takes the same refused path.
		let quotaItem = try XCTUnwrap(window.toolbar?.items.first { $0.label == SettingsPane.quota.title })
		let action = try XCTUnwrap(quotaItem.action)
		NSApp.sendAction(action, to: quotaItem.target, from: quotaItem)
		settle(window)
		XCTAssertEqual(controller.selectedPane, .sessions, "a toolbar click is refused while the sheet is open")
		XCTAssertEqual(window.title, SettingsPane.sessions.title)
		origin.closeManage()
		settle(window)
		XCTAssertNil(window.attachedSheet)
		NSApp.sendAction(action, to: quotaItem.target, from: quotaItem)
		settle(window)
		XCTAssertEqual(controller.selectedPane, .quota, "the toolbar switches pages once the sheet is closed")
	}

	/// ux/settings.md 窗口分页: with the App active and Settings the key window,
	/// a toolbar switch asks for an animated resize to the new page's natural
	/// height and names the page.
	func testAToolbarSwitchAnimatesTheHeightInTheActiveWindow() async throws {
		let controller = makeWindow()
		controller.show(pane: .liveUsage)
		let window = try XCTUnwrap(controller.window)
		let previousApplication = NSWorkspace.shared.frontmostApplication
		defer {
			if NSApp.isActive, let previousApplication,
				previousApplication.processIdentifier != ProcessInfo.processInfo.processIdentifier,
				!previousApplication.isTerminated {
				previousApplication.activate(options: [.activateIgnoringOtherApps])
			}
		}
		NSRunningApplication.current.activate(options: [.activateIgnoringOtherApps])
		window.makeKeyAndOrderFront(nil)
		let deadline = ContinuousClock.now.advanced(by: .seconds(2))
		while !(NSApp.isActive && window.isKeyWindow && window.isVisible), ContinuousClock.now < deadline {
			try await Task.sleep(for: .milliseconds(20))
		}
		guard NSApp.isActive && window.isKeyWindow && window.isVisible else {
			return XCTFail("the animation needs an active host and a visible key window: active=\(NSApp.isActive), key=\(window.isKeyWindow), visible=\(window.isVisible)")
		}
		settle(window)
		let start = contentHeight(window)
		let target = try XCTUnwrap(controller.naturalSize(of: .quota)).height.rounded(.up)
		XCTAssertGreaterThan(target - start, 40, "the two pages differ enough to see a transition")
		var requested: [Bool] = []
		let resizeWindow = controller.resizeWindow
		controller.resizeWindow = { window, frame, animate in
			requested.append(animate)
			resizeWindow(window, frame, animate)
		}
		let quotaItem = try XCTUnwrap(window.toolbar?.items.first { $0.label == SettingsPane.quota.title })
		NSApp.sendAction(try XCTUnwrap(quotaItem.action), to: quotaItem.target, from: quotaItem)
		settle(window)
		// Whether the transition is drawn is the system's: Reduce Motion (on
		// for the hosted macOS 15 runners) and a busy host both land the same
		// request in one step, so the request is what is asserted.
		XCTAssertEqual(requested.first, true, "the switch asks the system for an animated resize: \(requested)")
		XCTAssertEqual(contentHeight(window), target, accuracy: 1, "the animation ends at the page's natural height")
		XCTAssertEqual(window.title, SettingsPane.quota.title)
		XCTAssertEqual(controller.selectedPane, .quota)
	}

	/// ux/settings.md 原生整窗验收: a write made on Live usage survives a round
	/// trip through General, so turning collection off still takes the
	/// managed-section removal branch; a refused write, an existing-config
	/// conflict and the write consent stay shown; Sessions keeps its status
	/// line and in-flight save. Switching reads nothing again.
	func testARoundTripThroughAnotherPageKeepsWhatStillNeedsActingOn() async throws {
		func roundTrip(_ controller: SettingsWindowController, from pane: SettingsPane) async {
			controller.select(.general)
			settle(controller.window)
			controller.select(pane)
			settle(controller.window)
			// Let anything a switch might have started run before checking.
			for _ in 0 ..< 50 { await Task.yield() }
		}
		func loaded(_ telemetry: TelemetrySettingsController) async {
			for _ in 0 ..< 400 where telemetry.status == nil { await Task.yield() }
		}

		// Written, then turned off after the round trip: removal.
		let writing = CountingTelemetryTransport(StubTelemetryTransport(
			statuses: [telemetryStatus(enabled: false), telemetryStatus(enabled: true, exporter: "managed"), telemetryStatus(enabled: false)],
			disableResult: { DesktopTelemetryReceiverResultV1(receiver: "disabled", codexConfig: "removed") }
		))
		let telemetry = makeTelemetrySettingsController(transport: writing)
		let window = makeWindow(telemetry: telemetry)
		window.show(pane: .liveUsage)
		await loaded(telemetry)
		await telemetry.toggle(true)
		XCTAssertEqual(telemetry.consent, .write)
		let readsAtConsent = writing.statusReads
		await roundTrip(window, from: .liveUsage)
		XCTAssertEqual(telemetry.consent, .write, "the write consent stays open")
		XCTAssertEqual(writing.statusReads, readsAtConsent, "switching pages reads nothing again")
		await telemetry.writeAndEnable()
		XCTAssertEqual(telemetry.result, .written)
		let readsBefore = writing.statusReads
		await roundTrip(window, from: .liveUsage)
		XCTAssertEqual(telemetry.result, .written, "the write fact survives the round trip")
		XCTAssertEqual(writing.statusReads, readsBefore, "switching pages reads nothing again")
		await telemetry.toggle(false)
		XCTAssertEqual(writing.base.disableCalls, 1)
		XCTAssertEqual(telemetry.result, .removed, "turning off still removes the managed section")

		// A refused write.
		let refusing = StubTelemetryTransport(statuses: [telemetryStatus(enabled: false)], enableResult: { _ in nil })
		let refusedTelemetry = makeTelemetrySettingsController(transport: refusing)
		let refusedWindow = makeWindow(telemetry: refusedTelemetry)
		refusedWindow.show(pane: .liveUsage)
		await refusedTelemetry.load()
		await refusedTelemetry.toggle(true)
		await refusedTelemetry.writeAndEnable()
		let refusal = try XCTUnwrap(refusedTelemetry.failureRow)
		await roundTrip(refusedWindow, from: .liveUsage)
		XCTAssertEqual(refusedTelemetry.failureRow, refusal, "a refused write stays shown")

		// An existing exporter: the conflict area.
		let conflicting = StubTelemetryTransport(statuses: [telemetryStatus(enabled: false, exporter: "other")])
		let conflictTelemetry = makeTelemetrySettingsController(transport: conflicting)
		let conflictWindow = makeWindow(telemetry: conflictTelemetry)
		conflictWindow.show(pane: .liveUsage)
		await conflictTelemetry.load()
		await conflictTelemetry.toggle(true)
		XCTAssertEqual(conflictTelemetry.consent, .conflict)
		await roundTrip(conflictWindow, from: .liveUsage)
		XCTAssertEqual(conflictTelemetry.consent, .conflict, "the conflict area stays open")

		// Sessions: an unknown save keeps its status line and request.
		let originTransport = StubOriginTransport(writes: [.unknown])
		let origin = SessionOriginSettingsController(transport: originTransport, publish: { .failed }, newRequestID: { "round-trip-1" })
		let sessionsWindow = makeWindow(origin: origin)
		sessionsWindow.show(pane: .sessions)
		for _ in 0 ..< 400 where origin.status == .loading { await Task.yield() }
		await origin.selectDefault(.all)
		XCTAssertEqual(origin.status, .unknown)
		let line = origin.statusLine(for: .defaultScope)
		let defaultReads = await originTransport.defaultReads
		await roundTrip(sessionsWindow, from: .sessions)
		XCTAssertEqual(origin.status, .unknown, "the unresolved save stays with the page")
		XCTAssertEqual(origin.statusLine(for: .defaultScope), line)
		let readsAfter = await originTransport.defaultReads
		XCTAssertEqual(readsAfter, defaultReads, "switching pages reads nothing again")
	}

	// MARK: Whole-window renderings

	/// Pixels in the accent's hue band (B24A0B / F2650F sit near 22°); the
	/// warning amber (~39°) and the error red (~1°) fall outside it.
	private func accentPixels(_ bitmap: NSBitmapImageRep, above limit: Int? = nil) -> Int {
		countPixels(bitmap, above: limit) { hue, saturation, brightness in
			(14 ... 31).contains(hue) && saturation > 0.6 && brightness > 0.35
		}
	}

	/// Pixels in the system-accent blue band: macOS 26 fills an untinted
	/// selected segment with it in an active window, which a standalone
	/// rendering reproduces. No page shows an info-blue status here.
	/// The Quota page's client checkboxes keep the system style that
	/// subscription-quota owns (ux/settings.md 整窗视觉对齐), so their pixel
	/// rectangles are left out.
	private func systemBluePixels(_ bitmap: NSBitmapImageRep, excluding excluded: [NSRect] = []) -> Int {
		countPixels(bitmap, above: nil, excluding: excluded) { hue, saturation, brightness in
			(200 ... 235).contains(hue) && saturation > 0.6 && brightness > 0.5
		}
	}

	private func countPixels(
		_ bitmap: NSBitmapImageRep, above limit: Int?, excluding excluded: [NSRect] = [],
		_ matches: (CGFloat, CGFloat, CGFloat) -> Bool
	) -> Int {
		var count = 0
		let rows = limit ?? bitmap.pixelsHigh
		for y in stride(from: 0, to: rows, by: 2) {
			for x in stride(from: 0, to: bitmap.pixelsWide, by: 2) {
				if excluded.contains(where: { $0.contains(NSPoint(x: x, y: y)) }) { continue }
				guard let color = bitmap.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB) else { continue }
				if matches(color.hueComponent * 360, color.saturationComponent, color.brightnessComponent) { count += 1 }
			}
		}
		return count
	}

	private func render(_ root: AnyView, width: CGFloat, height: CGFloat, dark: Bool) -> NSBitmapImageRep? {
		renderWithCheckboxes(root, width: width, height: height, dark: dark)?.bitmap
	}

	/// The rendering plus the pixel rectangles (top-left origin) of any
	/// AppKit checkbox in it.
	private func renderWithCheckboxes(_ root: AnyView, width: CGFloat, height: CGFloat, dark: Bool) -> (bitmap: NSBitmapImageRep, checkboxes: [NSRect])? {
		let hosting = NSHostingView(rootView: root)
		hosting.appearance = NSAppearance(named: dark ? .darkAqua : .aqua)
		hosting.frame = NSRect(x: 0, y: 0, width: width, height: height)
		hosting.layoutSubtreeIfNeeded()
		hosting.displayIfNeeded()
		guard let representation = hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds) else { return nil }
		hosting.cacheDisplay(in: hosting.bounds, to: representation)
		let scale = CGFloat(representation.pixelsWide) / hosting.bounds.width
		func buttons(in view: NSView) -> [NSButton] {
			((view as? NSButton).map { [$0] } ?? []) + view.subviews.flatMap(buttons(in:))
		}
		let checkboxes = buttons(in: hosting)
			.filter { $0.identifier?.rawValue.hasPrefix("quota-client-") == true }
			.map { button -> NSRect in
				let rect = hosting.convert(button.bounds, from: button)
				let top = hosting.isFlipped ? rect.minY : hosting.bounds.height - rect.maxY
				return NSRect(x: rect.minX * scale, y: top * scale, width: rect.width * scale, height: rect.height * scale)
			}
		return (representation, checkboxes)
	}

	/// Each page renders whole in both languages and both appearances, plus
	/// the sheet: no wider than 460pt, at its natural height (the same rows as
	/// at the top of a taller canvas), short enough not to scroll on a 900pt
	/// screen, and with neutral selected segments — the only accent fill left
	/// is the sheet's Done button.
	func testEveryPageRendersWholeAtItsNaturalHeightWithNeutralSegments() async throws {
		let previous = ProcessInfo.processInfo.environment["AGENTDECK_TEST_LOCALE"]
		defer {
			if let previous { setenv("AGENTDECK_TEST_LOCALE", previous, 1) } else { unsetenv("AGENTDECK_TEST_LOCALE") }
		}
		let chromeWindow = makeWindow()
		chromeWindow.show()
		let shown = try XCTUnwrap(chromeWindow.window)
		let chrome = shown.frame.height - contentHeight(shown)
		XCTAssertGreaterThan(chrome, 40, "the title bar and toolbar sit above the page")
		let statuses = DesktopOriginSourceStatusesV1(
			claudeMem: DesktopOriginSourceStatusV1(source: "claude-mem", kind: "adapter", configured: false, enabled: false, available: false,
				ruleVersion: "claude-mem-sdk-map-v1", reasonCode: "source_not_configured"),
			codexBackground: DesktopOriginSourceStatusV1(source: "codex-otel-background", kind: "builtin", available: true,
				ruleVersion: "codex-otel-background-v1", reasonCode: nil, receiver: "enabled", backgroundSessions: 1),
			partial: false
		)
		let rows = [
			DesktopOriginOverrideV1(client: "claude", sessionID: "old-01", value: "work", bindingStatus: "matched", revision: 2),
			DesktopOriginOverrideV1(client: "codex", sessionID: "archived-1", value: "auxiliary", bindingStatus: "unmatched", revision: 4),
		]
		for language in ["en", "zh-Hans"] {
			setenv("AGENTDECK_TEST_LOCALE", language, 1)
			for dark in [false, true] {
				let appearance = dark ? "dark" : "light"
				let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
				preferences.automaticUpdateEnabled = false
				let origin = SessionOriginSettingsController(
					transport: StubOriginTransport(
						pages: [.value(DesktopOriginOverridePageV1(items: rows, page: 1, limit: 20, total: rows.count))],
						statuses: [.value(statuses)]
					),
					publish: { .failed }, newRequestID: { UUID().uuidString }
				)
				await origin.open()
				let quota = makeQuotaSettingsController(preferences: preferences)
				await quota.load()
				let telemetry = makeTelemetrySettingsController()
				await telemetry.load()
				var renderings: [(String, AnyView, Bool)] = SettingsPane.allCases.map { pane in
					(pane.rawValue, AnyView(SettingsPaneView(
						pane: pane, preferences: preferences, quotaSettings: quota, telemetrySettings: telemetry,
						sessionOrigin: origin, scrolls: false
					)), false)
				}
				renderings.append(("sheet", AnyView(OriginManageSheet(controller: origin)), true))
				for (name, view, sheet) in renderings {
					let label = "\(name) \(language) \(appearance)"
					let root = AnyView(view.environment(\.colorScheme, dark ? .dark : .light))
					let measuring = NSHostingView(rootView: root)
					measuring.frame = NSRect(x: 0, y: 0, width: 460, height: 2000)
					measuring.layoutSubtreeIfNeeded()
					let size = measuring.fittingSize
					XCTAssertLessThanOrEqual(size.width, 461, "\(label): wider than the window")
					let height = sheet ? OriginManageSheet.height : size.height
					let rendered = try XCTUnwrap(renderWithCheckboxes(root, width: 460, height: height, dark: dark))
					let fitted = rendered.bitmap
					if !sheet {
						XCTAssertLessThanOrEqual(height + chrome, 875, "\(label): the page scrolls on a 900pt screen")
						XCTAssertTrue(marginsAreBlank(fitted, points: 12, viewWidth: 460), "\(label): content overflows the width")
						let tall = try XCTUnwrap(render(
							AnyView(VStack(spacing: 0) { root; Spacer(minLength: 0) }), width: 460, height: height + 240, dark: dark
						))
						XCTAssertTrue(sameTopRows(fitted, tall), "\(label): the measured height squeezes the rows")
						XCTAssertEqual(accentPixels(fitted), 0, "\(label): accent fill on a page whose switches are all off")
						if name == SettingsPane.quota.rawValue {
							XCTAssertEqual(rendered.checkboxes.count, 2, "\(label): both client checkboxes found")
						}
						XCTAssertEqual(
							systemBluePixels(fitted, excluding: rendered.checkboxes), 0, "\(label): a selected segment takes the system accent"
						)
					} else {
						// Above the footer only the Done button may carry the accent.
						let footer = Int(64 * CGFloat(fitted.pixelsHigh) / height)
						XCTAssertEqual(accentPixels(fitted, above: fitted.pixelsHigh - footer), 0, "\(label): accent fill above the footer")
						XCTAssertEqual(systemBluePixels(fitted), 0, "\(label): a selected segment takes the system accent")
						XCTAssertGreaterThan(accentPixels(fitted), 0, "\(label): Done is the accent-filled primary")
					}
					let png = try XCTUnwrap(fitted.representation(using: .png, properties: [:]))
					let attachment = XCTAttachment(data: png, uniformTypeIdentifier: "public.png")
					attachment.name = "Settings page — \(label) — 460pt"
					attachment.lifetime = .keepAlways
					add(attachment)
				}
			}
		}
	}
}

// MARK: - Settings acceptance against the real helper

/// Forwards to the real embedded helper and fails one default-scope save on
/// request: before it reaches the helper (nothing committed), or after the
/// helper committed while its answer is lost.
actor FaultInjectingOriginTransport: SessionOriginSettingsTransport {
	enum Fault { case beforeCommit, afterCommit }

	let base: EmbeddedHelperRunner
	private var next: Fault?
	private(set) var saveRequestIDs: [String] = []

	init(_ base: EmbeddedHelperRunner) { self.base = base }

	func arm(_ fault: Fault) { next = fault }

	func saveDefaultScope(_ value: DesktopSessionScopeV1, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		saveRequestIDs.append(requestID)
		let fault = next
		next = nil
		switch fault {
		case .beforeCommit:
			return .unknown
		case .afterCommit:
			_ = await base.saveDefaultScope(value, expectedRevision: expectedRevision, requestID: requestID)
			return .unknown
		case nil:
			return await base.saveDefaultScope(value, expectedRevision: expectedRevision, requestID: requestID)
		}
	}

	func readDefaultScope() async -> DesktopOriginReadOutcome<DesktopSessionScopeDefaultV1> { await base.readDefaultScope() }
	func lookupSession(client: String, sessionID: String) async -> DesktopOriginLookupOutcome {
		await base.lookupSession(client: client, sessionID: sessionID)
	}
	func setOverride(_ target: DesktopOriginTargetV1, value: String, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		await base.setOverride(target, value: value, expectedRevision: expectedRevision, requestID: requestID)
	}
	func clearOverride(_ target: DesktopOriginTargetV1, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		await base.clearOverride(target, expectedRevision: expectedRevision, requestID: requestID)
	}
	func readReceipt(requestID: String) async -> DesktopOriginReceiptOutcome { await base.readReceipt(requestID: requestID) }
	func listOverrides(page: Int, limit: Int) async -> DesktopOriginReadOutcome<DesktopOriginOverridePageV1> {
		await base.listOverrides(page: page, limit: limit)
	}
	func readOriginSourceStatus() async -> DesktopOriginReadOutcome<DesktopOriginSourceStatusesV1> { await base.readOriginSourceStatus() }
}

/// Task 6 acceptance with the real Settings window, the real embedded Go
/// helper and a real SQLite store, all inside the isolated test home the
/// macOS test script creates; no user state is read or written. A second
/// helper process with the same state directory stands in for the CLI.
/// Within this process, fresh window, settings controller and helper runner
/// instances read the same persisted store; a real App restart through the
/// App's startup wiring is `SettingsRestartAcceptanceTests`.
@MainActor
final class SettingsHelperAcceptanceTests: XCTestCase {
	private var opened: [SettingsWindowController] = []
	private var homes: [URL] = []

	override func tearDown() async throws {
		for controller in opened { controller.window?.close() }
		opened = []
		for home in homes { try? FileManager.default.removeItem(at: home) }
		homes = []
		try await super.tearDown()
	}

	private func isolatedHome() throws -> URL {
		let testHome = try XCTUnwrap(ProcessInfo.processInfo.environment["AGENTDECK_TEST_HOME"], "hosted tests run with an isolated home")
		XCTAssertTrue(testHome.hasPrefix("/private/tmp/agentdeck-macos-xctest.") || testHome.hasPrefix("/tmp/agentdeck-macos-xctest."))
		let home = URL(fileURLWithPath: testHome, isDirectory: true).appendingPathComponent("settings-acceptance-\(UUID().uuidString)", isDirectory: true)
		try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
		homes.append(home)
		return home
	}

	/// A fresh helper runner over `home`: the App's bridge, or the CLI.
	private func helper(_ home: URL) throws -> EmbeddedHelperRunner {
		let runner = EmbeddedHelperRunner(
			appBundleURL: Bundle.main.bundleURL,
			environment: ["HOME": home.path, "LANG": "en_US_POSIX", "LC_ALL": "en_US_POSIX", "PATH": "/usr/bin:/bin"]
		)
		guard (try? runner.embeddedHelperURL()) != nil else { throw XCTSkip("the test host has no embedded helper") }
		return runner
	}

	private func window(_ origin: SessionOriginSettingsController) -> SettingsWindowController {
		let preferences = DesktopPreferences(defaults: isolatedDefaults(), registrar: StubLoginItemRegistrar())
		let controller = SettingsWindowController(
			preferences: preferences, quotaSettings: makeQuotaSettingsController(preferences: preferences),
			telemetrySettings: makeTelemetrySettingsController(), sessionOrigin: origin
		)
		opened.append(controller)
		return controller
	}

	private func waitUntil(_ what: String, _ condition: () -> Bool) async throws {
		let deadline = ContinuousClock.now + .seconds(20)
		while !condition() {
			guard ContinuousClock.now < deadline else { return XCTFail("timed out waiting until \(what)") }
			try await Task.sleep(for: .milliseconds(20))
		}
	}

	private func committedRevision(_ outcome: DesktopOriginWriteOutcome) -> Int64? {
		if case let .committed(receipt) = outcome { return receipt.revision }
		return nil
	}

	/// lifecycle-concurrency: the window reads the CLI's value; a CLI write
	/// made while the window holds an older value makes the window's save
	/// conflict without writing, and only reading the latest values unlocks
	/// it, with no replay of the old choice; after a restart the persisted
	/// value reads back through fresh instances.
	func testConcurrentCLIWritesConflictAndARestartReadsThePersistedDefaultBack() async throws {
		let home = try isolatedHome()
		let cli = try helper(home)
		let seeded = await cli.saveDefaultScope(.auxiliary, expectedRevision: 0, requestID: "cli-seed")
		XCTAssertEqual(committedRevision(seeded), 1)

		var sequence = 0
		let origin = SessionOriginSettingsController(transport: try helper(home), publish: { .published(nil) }, newRequestID: {
			sequence += 1
			return "app-run-1-\(sequence)"
		})
		let settings = window(origin)
		settings.show(pane: .sessions)
		try await waitUntil("the window confirms the CLI default") { origin.defaultState == .confirmed(.auxiliary, revision: 1) && origin.status == .ready }

		await origin.selectDefault(.all)
		XCTAssertEqual(origin.defaultState, .confirmed(.all, revision: 2))
		XCTAssertEqual(origin.status, .ready)
		let afterSave = await cli.readDefaultScope()
		XCTAssertEqual(afterSave, .value(.confirmed(.all, revision: 2)), "the CLI reads what the window saved")

		let concurrent = await cli.saveDefaultScope(.work, expectedRevision: 2, requestID: "cli-concurrent")
		XCTAssertEqual(committedRevision(concurrent), 3)
		XCTAssertEqual(origin.defaultState, .confirmed(.all, revision: 2), "the open window still shows its last confirmed value")
		await origin.selectDefault(.auxiliary)
		guard case let .conflict(pending) = origin.operation else { return XCTFail("a stale window save conflicts: \(origin.operation)") }
		XCTAssertEqual(pending.expectedRevision, 2)
		XCTAssertEqual(origin.status, .conflict)
		XCTAssertFalse(origin.writesEnabled, "a conflict locks the group until the latest values are read")
		let afterConflict = await cli.readDefaultScope()
		XCTAssertEqual(afterConflict, .value(.confirmed(.work, revision: 3)), "the conflicting save wrote nothing")

		await origin.readLatestValues()
		XCTAssertEqual(origin.defaultState, .confirmed(.work, revision: 3))
		XCTAssertEqual(origin.operation, .idle)
		XCTAssertTrue(origin.writesEnabled)
		let afterReload = await cli.readDefaultScope()
		XCTAssertEqual(afterReload, .value(.confirmed(.work, revision: 3)), "reading the latest values never replays the old choice")

		// Fresh instances over the same store (the process-level restart is
		// SettingsRestartAcceptanceTests).
		settings.window?.close()
		let restarted = SessionOriginSettingsController(transport: try helper(home), publish: { .published(nil) }, newRequestID: { "app-run-2-\(UUID().uuidString)" })
		let restartedWindow = window(restarted)
		restartedWindow.show()
		XCTAssertEqual(restartedWindow.selectedPane, .general, "a new App run opens Settings on General")
		try await waitUntil("the restarted App reads the default back") { restarted.status == .ready && restarted.defaultState != .loading }
		XCTAssertEqual(restarted.defaultState, .confirmed(.work, revision: 3), "the persisted default reads back after a restart")
		XCTAssertEqual(restarted.operation, .idle)
	}

	/// mutation-recovery: a save that fails before the helper commits stays
	/// unknown across a window close and reopen, its receipt reads back as
	/// unknown, and the explicit retry commits once under the same request
	/// ID; a save whose answer is lost after the helper committed is never
	/// called saved until its receipt is read back, and is not written twice.
	func testUnknownSavesBeforeAndAfterCommitRecoverUnderTheOriginalRequest() async throws {
		let home = try isolatedHome()
		let cli = try helper(home)
		let seeded = await cli.saveDefaultScope(.auxiliary, expectedRevision: 0, requestID: "cli-seed")
		XCTAssertEqual(committedRevision(seeded), 1)
		let transport = FaultInjectingOriginTransport(try helper(home))
		var sequence = 0
		let origin = SessionOriginSettingsController(transport: transport, publish: { .published(nil) }, newRequestID: {
			sequence += 1
			return "acceptance-\(sequence)"
		})
		let settings = window(origin)
		settings.show(pane: .sessions)
		try await waitUntil("the window confirms the default") { origin.defaultState == .confirmed(.auxiliary, revision: 1) && origin.status == .ready }

		// Before commit: nothing reached the helper.
		await transport.arm(.beforeCommit)
		await origin.selectDefault(.work)
		let firstPending = OriginPendingOperation(action: .saveDefault(.work), expectedRevision: 1, requestID: "acceptance-1")
		XCTAssertEqual(origin.operation, .unknown(firstPending, receiptChecked: false))
		XCTAssertEqual(origin.defaultState, .confirmed(.auxiliary, revision: 1), "an unknown save never shows as saved")
		XCTAssertFalse(origin.writesEnabled)
		let beforeRetry = await cli.readDefaultScope()
		XCTAssertEqual(beforeRetry, .value(.confirmed(.auxiliary, revision: 1)), "nothing reached the helper")

		let window = try XCTUnwrap(settings.window)
		window.performClose(nil)
		settings.show()
		try await waitUntil("the reopened window settles") { origin.sources != .loading }
		XCTAssertEqual(origin.operation, .unknown(firstPending, receiptChecked: false), "closing and reopening keeps the unresolved request")

		await origin.readRequestReceipt()
		XCTAssertEqual(origin.operation, .unknown(firstPending, receiptChecked: true), "no receipt keeps the outcome unknown")
		XCTAssertTrue(origin.retryAvailable)
		await origin.retry()
		XCTAssertEqual(origin.operation, .idle)
		XCTAssertEqual(origin.defaultState, .confirmed(.work, revision: 2))
		guard case let .known(first) = await cli.readReceipt(requestID: "acceptance-1") else { return XCTFail("the retried request has a receipt") }
		XCTAssertEqual(first.outcome, .saved)
		XCTAssertEqual(first.value, "work")
		XCTAssertEqual(first.revision, 2, "one commit: revision 1 to 2")
		XCTAssertFalse(first.replayed)

		// After commit, answer lost.
		await transport.arm(.afterCommit)
		await origin.selectDefault(.all)
		let secondPending = OriginPendingOperation(action: .saveDefault(.all), expectedRevision: 2, requestID: "acceptance-2")
		XCTAssertEqual(origin.operation, .unknown(secondPending, receiptChecked: false))
		XCTAssertEqual(origin.defaultState, .confirmed(.work, revision: 2), "a lost answer is not claimed as saved")
		let committedMeanwhile = await cli.readDefaultScope()
		XCTAssertEqual(committedMeanwhile, .value(.confirmed(.all, revision: 3)), "the helper did commit")

		await origin.readRequestReceipt()
		XCTAssertEqual(origin.operation, .idle, "the receipt ends the pending operation")
		XCTAssertEqual(origin.defaultState, .confirmed(.all, revision: 3))
		guard case let .known(second) = await cli.readReceipt(requestID: "acceptance-2") else { return XCTFail("the lost request has a receipt") }
		XCTAssertEqual(second.revision, 3)
		let final = await cli.readDefaultScope()
		XCTAssertEqual(final, .value(.confirmed(.all, revision: 3)), "recovering never wrote the value a second time")
		let requests = await transport.saveRequestIDs
		XCTAssertEqual(requests, ["acceptance-1", "acceptance-1", "acceptance-2"], "the retry reused the original request; no new ID was invented")
	}
}

// MARK: - Settings restart through the App's own startup

/// Task 6 lifecycle-concurrency: application restart readback. The isolation
/// script runs phase 1 and phase 2 as two separate test-host launches over
/// one fresh temporary home; each phase drives the Settings controllers the
/// App's real delegate built at startup and opens Settings through its menu
/// action. Without the script's phase variable both tests skip.
@MainActor
final class SettingsRestartAcceptanceTests: XCTestCase {
	private struct PhaseOneRecord: Codable {
		let pid: Int32
		let scope: String
		let revision: Int64
	}

	private func phase(_ expected: String) throws -> (home: URL, delegate: AgentDeckApplicationDelegate) {
		guard ProcessInfo.processInfo.environment["AGENTDECK_SETTINGS_RESTART_PHASE"] == expected else {
			throw XCTSkip("opt-in: phase \(expected) runs from scripts/test-macos-app.sh")
		}
		let testHome = try XCTUnwrap(ProcessInfo.processInfo.environment["AGENTDECK_TEST_HOME"], "hosted tests run with an isolated home")
		XCTAssertTrue(testHome.hasPrefix("/private/tmp/agentdeck-macos-xctest.") || testHome.hasPrefix("/tmp/agentdeck-macos-xctest."))
		let delegate = try XCTUnwrap(NSApp.delegate as? AgentDeckApplicationDelegate, "the test host runs the App's own delegate")
		return (URL(fileURLWithPath: testHome, isDirectory: true), delegate)
	}

	private func cli(_ home: URL) throws -> EmbeddedHelperRunner {
		let runner = EmbeddedHelperRunner(
			appBundleURL: Bundle.main.bundleURL,
			environment: ["HOME": home.path, "LANG": "en_US_POSIX", "LC_ALL": "en_US_POSIX", "PATH": "/usr/bin:/bin"]
		)
		guard (try? runner.embeddedHelperURL()) != nil else { throw XCTSkip("the test host has no embedded helper") }
		return runner
	}

	private func record(_ home: URL) -> URL { home.appendingPathComponent("settings-restart-phase1.json") }

	private func waitUntil(_ what: String, _ condition: () -> Bool) async throws {
		let deadline = ContinuousClock.now + .seconds(30)
		while !condition() {
			guard ContinuousClock.now < deadline else { return XCTFail("timed out waiting until \(what)") }
			try await Task.sleep(for: .milliseconds(20))
		}
	}

	private func openSettings(_ delegate: AgentDeckApplicationDelegate) {
		XCTAssertTrue(NSApp.sendAction(Selector(("openSettings")), to: delegate, from: nil), "the App's Settings menu action runs")
	}

	/// Phase 1: save a default through the Settings the App built, then end.
	func testRestartPhase1() async throws {
		let (home, delegate) = try phase("1")
		let cli = try cli(home)
		let seeded = await cli.saveDefaultScope(.auxiliary, expectedRevision: 0, requestID: "restart-seed")
		guard case let .committed(seed) = seeded, seed.revision == 1 else { return XCTFail("the CLI seeds the default: \(seeded)") }
		let origin = delegate.sessionOrigin
		openSettings(delegate)
		try await waitUntil("Settings confirms the seeded default") { origin.defaultState == .confirmed(.auxiliary, revision: 1) && origin.status == .ready }
		delegate.settingsController.select(.sessions)
		XCTAssertEqual(delegate.settingsController.selectedPane, .sessions)
		await origin.selectDefault(.all)
		XCTAssertEqual(origin.defaultState, .confirmed(.all, revision: 2))
		let persisted = await cli.readDefaultScope()
		XCTAssertEqual(persisted, .value(.confirmed(.all, revision: 2)))
		let data = try JSONEncoder().encode(PhaseOneRecord(pid: ProcessInfo.processInfo.processIdentifier, scope: "all", revision: 2))
		try data.write(to: record(home))
	}

	/// Phase 2: a new host process reads the saved default back through the
	/// App's startup wiring; nothing from phase 1 is in memory.
	func testRestartPhase2() async throws {
		let (home, delegate) = try phase("2")
		let earlier = try JSONDecoder().decode(PhaseOneRecord.self, from: Data(contentsOf: record(home)))
		XCTAssertNotEqual(earlier.pid, ProcessInfo.processInfo.processIdentifier, "phase 2 runs in a new App process")
		let origin = delegate.sessionOrigin
		XCTAssertEqual(origin.defaultState, .loading, "a new process starts with nothing read")
		XCTAssertEqual(origin.operation, .idle)
		openSettings(delegate)
		try await waitUntil("Settings reads the default back") { origin.status == .ready && origin.defaultState != .loading }
		XCTAssertEqual(origin.defaultState, .confirmed(.all, revision: earlier.revision), "the default saved before the restart reads back")
		XCTAssertEqual(delegate.settingsController.selectedPane, .general, "a new App run opens Settings on General")
		XCTAssertEqual(delegate.settingsController.window?.title, SettingsPane.general.title)
	}
}
