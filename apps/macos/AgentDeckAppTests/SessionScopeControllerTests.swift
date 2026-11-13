import XCTest
@testable import AgentDeck
@testable import AgentDeckShared

/// A failure switch the nonisolated stub reader consults.
private final class FailureSwitch: @unchecked Sendable {
	private let lock = NSLock()
	private var value = true
	var failing: Bool {
		get { lock.withLock { value } }
		set { lock.withLock { value = newValue } }
	}
}

/// The Popover's session-scope behavior over real wire DTOs. Every view and
/// answer is synthetic; no helper, database or real state is touched.
@MainActor
final class SessionScopeControllerTests: XCTestCase {
	private func selection(_ scope: DesktopSessionScopeV1, _ client: String = "all", _ period: String = "today") -> DesktopSessionScopeSelectionV1 {
		DesktopSessionScopeSelectionV1(scope: scope, client: client, period: period)
	}

	private func fixtureReader() -> StubSessionScopeReader {
		let views = WireFixture.fixtureScopeViews()
		return StubSessionScopeReader { .view(views[$0]!) }
	}

	private func openModel(
		_ envelope: DesktopWireEnvelopeV1 = WireFixture.envelope(),
		reader: StubSessionScopeReader? = nil,
		host: StubDesktopHost? = nil
	) async -> (MenuBarViewModel, StubSessionScopeReader) {
		let reader = reader ?? fixtureReader()
		let host = host ?? StubDesktopHost(behavior: .envelope(envelope))
		let model = await makeModel(host: host, sessionScopeReader: reader)
		await model.coordinator.refresh()
		model.selectedPanel = .sessions
		model.beginSessionScopePresentation()
		return (model, reader)
	}

	private func waitUntil(_ condition: @escaping () async -> Bool, file: StaticString = #filePath, line: UInt = #line) async {
		let clock = ContinuousClock()
		let deadline = clock.now.advanced(by: .seconds(2))
		while clock.now < deadline {
			if await condition() { return }
			try? await Task.sleep(for: .milliseconds(5))
		}
		XCTFail("condition not reached", file: file, line: line)
	}

	private func pressed(_ model: MenuBarViewModel) -> DesktopSessionScopeV1? {
		model.sessionsPanel.scope.choices.first(where: \.pressed)?.id
	}

	private let maintenanceRow = WireFixture.scopeRecent(id: "maint-1", effective: "auxiliary", automatic: "auxiliary",
		reason: "maintenance_origin", rule: "codex-otel-background-v1")

	// MARK: Default and temporary scope

	func testEachOpenStartsFromTheConfirmedDefaultAndTemporaryChoicesPersistNothing() async {
		let view = WireFixture.sessionScopeView(scope: "auxiliary", defaults: WireFixture.confirmedDefault("auxiliary", revision: 3),
			sessions: 1, projects: 1, knownDurationMS: 600_000, unknownCount: 0, recent: [maintenanceRow])
		let (model, reader) = await openModel(WireFixture.envelope(sessionScope: .custom(view)))

		// The publication already answers the default selection: no request.
		XCTAssertEqual(pressed(model), .auxiliary)
		XCTAssertEqual(model.sessionsPanel.scope.state, .ready)
		XCTAssertEqual(model.sessionScopePanelIsReady, model.sessionScopePanel.state == .ready, "the tab strip's cheap check must agree with the panel")
		await model.synchronizeSessionScope()
		var requests = await reader.requests
		XCTAssertTrue(requests.isEmpty)

		model.chooseSessionScope(.all)
		await model.synchronizeSessionScope()
		requests = await reader.requests
		XCTAssertEqual(requests, [selection(.all)])
		XCTAssertEqual(pressed(model), .all)

		// Tabs and filters keep the temporary scope for this open.
		model.selectedPanel = .usage
		model.selectedPanel = .sessions
		model.selectedPeriod = "7d"
		await model.synchronizeSessionScope()
		XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "all|all|7d")

		// A new open starts again from the saved default, which the choice never changed.
		model.selectedPeriod = "today"
		model.beginSessionScopePresentation()
		XCTAssertEqual(pressed(model), .all, "the previous open's view stays labelled with its own scope until synchronized")
		await model.synchronizeSessionScope()
		XCTAssertEqual(pressed(model), .auxiliary)
		XCTAssertEqual(model.sessionScopeField.view?.default, DesktopSessionScopeDefaultV1(confirmed: true, value: .auxiliary, revision: 3, reason: nil))
		requests = await reader.requests
		XCTAssertEqual(requests, [selection(.all), selection(.all, "all", "7d")], "reopening read the default from the publication, not the helper")
	}

	func testAnUnconfirmedDefaultNeverFallsBackToWork() async {
		let view = WireFixture.unavailableSessionScopeView(reason: "default_unconfirmed", defaults: WireFixture.unconfirmedDefault, selection: nil)
		let (model, reader) = await openModel(WireFixture.envelope(sessionScope: .custom(view)))
		await model.synchronizeSessionScope()

		let scope = model.sessionsPanel.scope
		XCTAssertEqual(scope.state, .defaultUnknown)
		XCTAssertEqual(model.sessionScopePanelIsReady, model.sessionScopePanel.state == .ready, "the tab strip's cheap check must agree with the panel")
		XCTAssertTrue(scope.choices.isEmpty)
		XCTAssertTrue(scope.stats.isEmpty)
		XCTAssertTrue(scope.rows.isEmpty)
		XCTAssertEqual(scope.notice, t(DesktopCopy.sessionScopeDefaultUnknown))
		let requests = await reader.requests
		XCTAssertTrue(requests.isEmpty, "no selection is implied, so nothing is read")
		XCTAssertNotNil(model.hero, "the usage overview stays readable")
		XCTAssertEqual(model.panelTabs.first { $0.id == .sessions }?.marked, true)
	}

	// MARK: Legacy and untrusted publications

	func testALegacyPublicationIsShownAsLegacyAndNeverAsWork() async {
		let (model, reader) = await openModel(WireFixture.envelope(sessionScope: .absent))
		await model.synchronizeSessionScope()

		XCTAssertEqual(model.sessionScopeField, .absent)
		XCTAssertEqual(model.snapshot?.sessions.total, 1, "the legacy all-session total is still decoded")
		let scope = model.sessionsPanel.scope
		XCTAssertEqual(scope.state, .legacy)
		XCTAssertEqual(model.sessionScopePanelIsReady, model.sessionScopePanel.state == .ready, "the tab strip's cheap check must agree with the panel")
		XCTAssertEqual(scope.notice, t(DesktopCopy.sessionScopeLegacy))
		XCTAssertEqual(scope.hint, t(DesktopCopy.sessionScopeLegacyHint))
		XCTAssertTrue(scope.stats.isEmpty && scope.rows.isEmpty && scope.choices.isEmpty)
		let requests = await reader.requests
		XCTAssertTrue(requests.isEmpty)
		XCTAssertNotNil(model.hero)
	}

	func testAnUntrustedViewDegradesOnlyTheSessionArea() async {
		var view = WireFixture.sessionScopeView()
		view["selection"] = ["scope": "maintenance", "client": "all", "period": "today"]
		let (model, _) = await openModel(WireFixture.envelope(sessionScope: .custom(view)))

		XCTAssertEqual(model.sessionScopeField, .invalid)
		XCTAssertEqual(model.sessionsPanel.scope.state, .unavailable)
		XCTAssertEqual(model.sessionScopePanelIsReady, model.sessionScopePanel.state == .ready, "the tab strip's cheap check must agree with the panel")
		XCTAssertTrue(model.sessionsPanel.scope.stats.isEmpty)
		XCTAssertNotNil(model.hero)
		XCTAssertTrue(model.usagePanel.available)
	}

	// MARK: Request ordering

	func testALateAnswerNeverReplacesANewerSelection() async {
		let reader = fixtureReader()
		await reader.hold(true)
		let (model, _) = await openModel(reader: reader)

		model.chooseSessionScope(.auxiliary)
		let first = Task { await model.synchronizeSessionScope() }
		await waitUntil { await reader.heldCount() == 1 }
		// The previous Work data stays, labelled Work, while Auxiliary loads.
		XCTAssertEqual(model.sessionScope.request, .loading(selection(.auxiliary)))
		XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "work|all|today")
		XCTAssertFalse(model.sessionsPanel.scope.choicesEnabled)
		XCTAssertEqual(model.sessionsPanel.scope.notice, t(DesktopCopy.sessionScopeLoading, t(DesktopCopy.sessionScopeAuxiliary), t(DesktopCopy.sessionScopeWork)))

		model.selectedClient = "codex"
		let second = Task { await model.synchronizeSessionScope() }
		await waitUntil { await reader.heldCount() == 2 }
		await reader.release(1)
		await second.value
		XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "auxiliary|codex|today")

		await reader.release(0)
		await first.value
		XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "auxiliary|codex|today", "the earlier request's late answer is discarded")
		XCTAssertEqual(model.sessionScope.request, .idle)
		XCTAssertTrue(model.sessionsPanel.scope.choicesEnabled)
	}

	func testANewOpenDiscardsThePreviousOpensPendingAnswer() async {
		let reader = fixtureReader()
		await reader.hold(true)
		let (model, _) = await openModel(reader: reader)

		model.chooseSessionScope(.all)
		let pending = Task { await model.synchronizeSessionScope() }
		await waitUntil { await reader.heldCount() == 1 }
		model.beginSessionScopePresentation()
		await reader.release(0)
		await pending.value

		XCTAssertEqual(model.sessionScope.request, .idle)
		XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "work|all|today")
		XCTAssertEqual(pressed(model), .work)
	}

	/// ASC-DM-R1-F1: going back to the selection already shown must end the
	/// read that left it. Cancelling that read's Swift task is no answer
	/// identity check — a finished answer can still arrive — so its late
	/// success or error must never replace, or fail, the current selection.
	func testReturningToTheShownSelectionDiscardsTheReadThatLeftIt() async {
		let views = WireFixture.fixtureScopeViews()
		let filters: [(ReferenceWritableKeyPath<MenuBarViewModel, String>, String, String)] = [
			(\.selectedClient, "codex", "all"),
			(\.selectedPeriod, "7d", "today"),
		]
		let lateAnswers: [(String, StubSessionScopeReader.Answer?, Bool)] = [
			("success after cancellation", nil, true),
			("error after cancellation", .failure(.nonZeroExit(1)), true),
			("success without cancellation", nil, false),
			("error without cancellation", .failure(.nonZeroExit(1)), false),
		]
		for (filter, away, back) in filters {
			for (outcome, late, cancelOwner) in lateAnswers {
				let label = "\(away) and back, \(outcome)"
				let reader = StubSessionScopeReader { .view(views[$0]!) }
				let (model, _) = await openModel(reader: reader)
				await model.synchronizeSessionScope()
				XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "work|all|today", label)

				await reader.hold(true)
				if let late { await reader.answer(request: 0, with: late) }
				model[keyPath: filter] = away
				let left = Task { await model.synchronizeSessionScope() }
				await waitUntil { await reader.heldCount() == 1 }
				if cancelOwner { left.cancel() }
				model[keyPath: filter] = back
				await model.synchronizeSessionScope()
				XCTAssertEqual(model.sessionScope.request, .idle, label)

				await reader.release(0)
				await left.value
				XCTAssertEqual(model.sessionScope.accepted?.selection, selection(.work), label)
				XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "work|all|today", label)
				XCTAssertEqual(model.sessionScope.request, .idle, label)
				XCTAssertNil(model.sessionScope.failureReason, label)
				XCTAssertNil(model.sessionsPanel.scope.retryLabel, label)
				XCTAssertTrue(model.sessionsPanel.scope.choicesEnabled, label)
				let requests = await reader.requests
				XCTAssertEqual(requests.count, 1, "returning to the shown view reads nothing: \(label)")
			}
		}
	}

	/// ASC-DM-R1-F2: a read whose owning synchronization was cancelled — the
	/// publication changed, or the area left and came back — must not swallow
	/// the synchronization that replaces it, whichever of the two ends first.
	func testACancelledReadNeverSwallowsTheSynchronizationThatReplacesIt() async {
		for (newPublication, cancellationFirst) in [(true, false), (true, true), (false, false), (false, true)] {
			let label = "new publication: \(newPublication), old cancellation ends first: \(cancellationFirst)"
			let reader = fixtureReader()
			let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
			let (model, _) = await openModel(reader: reader, host: host)
			await model.synchronizeSessionScope()
			let firstPublication = model.sessionScopePublicationKey

			await reader.hold(true)
			await reader.answer(request: 0, with: .failure(.cancelled))
			model.chooseSessionScope(.auxiliary)
			let first = Task { await model.synchronizeSessionScope() }
			await waitUntil { await reader.heldCount() == 1 }
			if newPublication {
				host.behavior = .envelope(WireFixture.envelope(generatedAt: "2026-08-13T10:01:00Z"))
				await model.coordinator.refresh()
				XCTAssertNotEqual(model.sessionScopePublicationKey, firstPublication, label)
			}
			first.cancel()

			let second: Task<Void, Never>
			if cancellationFirst {
				await reader.release(0)
				await first.value
				second = Task { await model.synchronizeSessionScope() }
			} else {
				second = Task { await model.synchronizeSessionScope() }
				await waitUntil { await reader.requests.count == 2 }
				await reader.release(0)
				await first.value
			}
			await waitUntil { await reader.requests.count == 2 }
			// The current target is still being read; the previous view stays
			// under its own label meanwhile.
			XCTAssertEqual(model.sessionScope.request, .loading(selection(.auxiliary)), label)
			XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "work|all|today", label)

			await reader.release(1)
			await second.value
			XCTAssertEqual(model.sessionScope.request, .idle, label)
			XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "auxiliary|all|today", label)
			XCTAssertEqual(pressed(model), .auxiliary, label)
			XCTAssertEqual(model.sessionScope.acceptedPublication, model.sessionScopePublicationKey, label)
			XCTAssertNil(model.sessionsPanel.scope.retryLabel, label)
			XCTAssertTrue(model.sessionsPanel.scope.choicesEnabled, label)
			let requests = await reader.requests
			XCTAssertEqual(requests, [selection(.auxiliary), selection(.auxiliary)], label)
		}
	}

	// MARK: Failure, retry and stale data

	func testAFailedRequestKeepsThePreviousSelectionUntilRetry() async {
		let views = WireFixture.fixtureScopeViews()
		let failure = FailureSwitch()
		let reader = StubSessionScopeReader { selection in
			selection.scope == .auxiliary && failure.failing ? .failure(.nonZeroExit(1)) : .view(views[selection]!)
		}
		let (model, _) = await openModel(reader: reader)

		model.chooseSessionScope(.auxiliary)
		await model.synchronizeSessionScope()
		var scope = model.sessionsPanel.scope
		XCTAssertEqual(scope.selectionKey, "work|all|today")
		XCTAssertEqual(pressed(model), .work)
		XCTAssertFalse(scope.choicesEnabled)
		XCTAssertEqual(scope.retryLabel, t(DesktopCopy.sessionScopeRetry, t(DesktopCopy.sessionScopeAuxiliary)))
		XCTAssertEqual(scope.notice, t(DesktopCopy.sessionScopeFailed, t(DesktopCopy.sessionScopeAuxiliary), t(DesktopCopy.sessionScopeWork)))
		XCTAssertEqual(scope.announcement, scope.notice)
		XCTAssertTrue(scope.snapshotTime?.hasSuffix("UTC") == true)
		XCTAssertFalse(scope.rows.isEmpty, "previous data is retained, not replaced by zeros")

		// Leaving the tab and coming back does not change the failed state.
		model.selectedPanel = .usage
		model.selectedPanel = .sessions
		await model.synchronizeSessionScope()
		XCTAssertEqual(model.sessionsPanel.scope, scope)

		failure.failing = false
		await model.retrySessionScope()
		scope = model.sessionsPanel.scope
		XCTAssertEqual(scope.selectionKey, "auxiliary|all|today")
		XCTAssertNil(scope.retryLabel)
		XCTAssertTrue(scope.choicesEnabled)
		// The recovered selection survives tab and filter round trips.
		model.selectedPanel = .usage
		model.selectedPanel = .sessions
		model.selectedPeriod = "7d"
		await model.synchronizeSessionScope()
		model.selectedPeriod = "today"
		await model.synchronizeSessionScope()
		XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "auxiliary|all|today")
		XCTAssertNil(model.sessionsPanel.scope.retryLabel)
	}

	func testAnUnavailableAnswerWithoutPriorDataReportsNoCounts() async {
		let unavailable = WireFixture.decodedSessionScopeView(WireFixture.unavailableSessionScopeView(reason: "sessions_unavailable"))
		let reader = StubSessionScopeReader { _ in .view(unavailable) }
		let (model, _) = await openModel(WireFixture.envelope(sessionScope: .custom(WireFixture.unavailableSessionScopeView(reason: "sessions_unavailable"))), reader: reader)
		await model.synchronizeSessionScope()

		let scope = model.sessionsPanel.scope
		XCTAssertEqual(scope.state, .unavailable)
		XCTAssertEqual(model.sessionScopePanelIsReady, model.sessionScopePanel.state == .ready, "the tab strip's cheap check must agree with the panel")
		XCTAssertTrue(scope.stats.isEmpty)
		XCTAssertTrue(scope.rows.isEmpty)
		XCTAssertEqual(scope.notice, t(DesktopCopy.sessionScopeUnavailable))
		XCTAssertEqual(scope.retryLabel, t(DesktopCopy.sessionScopeRetry, t(DesktopCopy.sessionScopeWork)))
		XCTAssertTrue(scope.choicesEnabled, "with nothing shown, another scope may still be chosen")
		XCTAssertEqual(model.sessionScope.failureReason, "sessions_unavailable")
	}

	func testANewPublicationKeepsTheLastSuccessfulViewWhileItIsReread() async throws {
		let reader = fixtureReader()
		let host = StubDesktopHost(behavior: .envelope(WireFixture.envelope()))
		let (model, _) = await openModel(reader: reader, host: host)
		model.chooseSessionScope(.auxiliary)
		await model.synchronizeSessionScope()
		XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "auxiliary|all|today")

		host.behavior = .envelope(WireFixture.envelope(generatedAt: "2026-08-13T10:01:00Z"))
		await model.coordinator.refresh()
		await reader.hold(true)
		let reread = Task { await model.synchronizeSessionScope() }
		await waitUntil { await reader.heldCount() == 1 }
		let scope = model.sessionsPanel.scope
		XCTAssertEqual(scope.selectionKey, "auxiliary|all|today")
		XCTAssertEqual(scope.notice, t(DesktopCopy.sessionScopeStale))
		XCTAssertNotNil(scope.snapshotTime)
		XCTAssertFalse(scope.rows.isEmpty)
		await reader.release(1)
		await reread.value
		XCTAssertEqual(model.sessionScope.request, .idle)
		let requests = await reader.requests
		XCTAssertEqual(requests, [selection(.auxiliary), selection(.auxiliary)])
	}

	// MARK: Work signals

	/// Work's signals as the Go producer emits them when every indexed session
	/// is Auxiliary and one usage-only session has no text index
	/// (`TestSessionScopeViewKeepsUsageOnlySignalsWhenNoIndexedSessionIsInScope`):
	/// one coding/feature event, one file touched, one edit call.
	private func usageOnlySignals(activityAvailable: Bool) -> [String: Any] {
		let kinds: [[String: Any]] = ["coding", "debugging", "conversation", "delegation"].map { kind in
			let coding = kind == "coding"
			let sub: [[String: Any]] = coding ? [["kind": "feature", "share": 100.0, "cost": 0.01, "events": 1]] : []
			return ["kind": kind, "share": coding ? 100.0 : 0.0, "cost": coding ? 0.01 : 0.0, "events": coding ? 1 : 0, "sub": sub]
		}
		let activity: [[String: Any]] = activityAvailable ? [["period": "today", "client": "all", "cost_basis": "turn", "kinds": kinds]] : []
		let workflow: [[String: Any]] = [["period": "today", "client": "all", "first_edit_seconds": NSNull(), "files_touched": 1,
			"retries": 0, "edits_per_session": 1.0, "top_file": "main.go", "top_file_edits": 1]]
		let tooling: [[String: Any]] = [["period": "today", "client": "all", "calls": 1, "groups": 1,
			"rows": [["kind": "edit", "calls": 1, "share": 100.0]], "top_mcp_server": NSNull(), "top_mcp_calls": NSNull()]]
		return [
			"activity": ["available": activityAvailable, "items": activity] as [String: Any],
			"workflow": ["available": true, "items": workflow] as [String: Any],
			"tooling": ["available": true, "items": tooling] as [String: Any],
		]
	}

	/// ASC-DM-R2-F1: the session list comes from the text index, the signals
	/// from usage, and usage with no indexed session still belongs to Work. A
	/// selection that lists no session therefore keeps its own empty state
	/// while its signals stay visible, each family with its own availability;
	/// switching scope never moves the all-session usage, cost or quota.
	func testWorkSignalsStayVisibleWhenTheSelectionListsNoSession() async {
		for activityAvailable in [true, false] {
			let label = "activity available: \(activityAvailable)"
			let signals = usageOnlySignals(activityAvailable: activityAvailable)
			let work = WireFixture.sessionScopeView(sessions: 0, projects: 0, knownDurationMS: 0, unknownCount: 0, recent: [], workSignals: signals)
			let auxiliary = WireFixture.decodedSessionScopeView(WireFixture.sessionScopeView(scope: "auxiliary", sessions: 1, projects: 1,
				knownDurationMS: 1_800_000, unknownCount: 0, recent: [maintenanceRow], workSignals: WireFixture.workSignals(available: false)))
			let all = WireFixture.decodedSessionScopeView(WireFixture.sessionScopeView(scope: "all", sessions: 1, projects: 1,
				knownDurationMS: 1_800_000, unknownCount: 0, recent: [maintenanceRow], workSignals: signals))
			let reader = StubSessionScopeReader { $0.scope == .auxiliary ? .view(auxiliary) : .view(all) }
			let (model, _) = await openModel(WireFixture.envelope(sessionScope: .custom(work)), reader: reader)
			await model.synchronizeSessionScope()

			let scope = model.sessionsPanel.scope
			XCTAssertEqual(scope.state, .ready, label)
			XCTAssertEqual(model.sessionScopePanelIsReady, model.sessionScopePanel.state == .ready, "the tab strip's cheap check must agree with the panel")
			XCTAssertEqual(scope.selectionKey, "work|all|today", label)
			XCTAssertTrue(scope.rows.isEmpty, "no session is invented: \(label)")
			XCTAssertEqual(scope.notice, t(DesktopCopy.sessionScopeEmpty), label)
			XCTAssertEqual(scope.emptyCopy, t(DesktopCopy.sessionScopeNoMatches), label)

			let panel = model.sessionsPanel.workSignals
			XCTAssertEqual(panel.state, .captured, label)
			XCTAssertEqual(panel.summaries.map(\.id), [.activity, .workflow, .tooling], label)
			// Report a hidden area as a failure rather than crash the test host.
			guard panel.summaries.count == 3 else { continue }
			XCTAssertEqual(panel.summaries[2].value, "\(DesktopFormat.count(1)) \(t(DesktopCopy.sessionsToolCalls))", label)
			XCTAssertEqual(panel.toolingRows.map(\.id), ["edit"], label)
			XCTAssertEqual(panel.workflowMetrics.first { $0.id == "files-touched" }?.value, DesktopFormat.workSignalCount(1), label)
			XCTAssertEqual(panel.topFileText, "main.go ×\(DesktopFormat.count(1))", label)
			if activityAvailable {
				XCTAssertTrue(panel.uncapturedSections.isEmpty, label)
				XCTAssertNotNil(panel.summaries[0].value, label)
				XCTAssertEqual(panel.activityRows.first?.eventsText, DesktopFormat.count(1), label)
			} else {
				// The unavailable family is marked on its own; the others still show.
				XCTAssertEqual(panel.uncapturedSections, [.activity], label)
				XCTAssertNil(panel.summaries[0].value, label)
				XCTAssertNotNil(panel.summaries[1].value, label)
			}

			let hero = model.hero
			let usage = model.usagePanel
			let quota = model.quotaClients
			let tabs = model.clientTabs.map(\.value)
			XCTAssertNotNil(hero, label)
			for choice in [DesktopSessionScopeV1.auxiliary, .all, .work] {
				model.chooseSessionScope(choice)
				await model.synchronizeSessionScope()
				XCTAssertEqual(model.sessionsPanel.scope.selectionKey, "\(choice.rawValue)|all|today", label)
				XCTAssertEqual(model.hero, hero, "\(choice) changed the all-session usage: \(label)")
				XCTAssertEqual(model.usagePanel, usage, "\(choice): \(label)")
				XCTAssertEqual(model.quotaClients, quota, "\(choice) changed quota: \(label)")
				XCTAssertEqual(model.clientTabs.map(\.value), tabs, "\(choice): \(label)")
			}
			XCTAssertEqual(model.sessionsPanel.workSignals, panel, "back on Work, the same signals: \(label)")
		}
	}

	// MARK: Consumption and classification

	func testScopeChangesNeverChangeUsageAndTaskAgentsStayInWork() async {
		let (model, _) = await openModel()
		model.selectedClient = "codex"
		await model.synchronizeSessionScope()
		let hero = model.hero
		let tabs = model.clientTabs.map(\.value)
		let usage = model.usagePanel
		XCTAssertNotNil(hero)
		for scope in [DesktopSessionScopeV1.auxiliary, .all, .work] {
			model.chooseSessionScope(scope)
			await model.synchronizeSessionScope()
			XCTAssertEqual(model.hero, hero, "\(scope) changed the full-consumption hero")
			XCTAssertEqual(model.clientTabs.map(\.value), tabs)
			XCTAssertEqual(model.usagePanel, usage)
		}
		let work = model.sessionsPanel.scope.rows
		let child = work.first { $0.origin.sessionID == "child-1" }
		XCTAssertEqual(child?.classLabel, t(DesktopCopy.sessionClassUnknown))
		XCTAssertEqual(child?.origin.reason, t(DesktopCopy.sessionReasonTaskChild))
		model.chooseSessionScope(.auxiliary)
		await model.synchronizeSessionScope()
		XCTAssertFalse(model.sessionsPanel.scope.rows.contains { $0.origin.sessionID == "child-1" })
		XCTAssertNil(model.sessionsPanel.scope.unknownLine, "Auxiliary holds no unknown sessions")
	}

	func testEveryKnownReasonHasItsOwnCopyAndAnUnknownCodeFallsBackWithoutChangingTheClass() async {
		let known = ["maintenance_origin", "origin_metadata_missing", "task_child_unconfirmed", "conflicting_origin",
			"origin_source_unavailable", "origin_rule_unsupported", "origin_input_mismatch"]
		XCTAssertEqual(Set(SessionOriginCopy.reasonKeys.keys), Set(known))
		let texts = known.map(SessionOriginCopy.reason)
		XCTAssertEqual(Set(texts).count, known.count)
		XCTAssertFalse(texts.contains(t(DesktopCopy.sessionReasonUnknown)))

		let future = WireFixture.scopeRecent(id: "future-1", effective: "auxiliary", automatic: "auxiliary", reason: "future_reason", rule: NSNull())
		let view = WireFixture.sessionScopeView(sessions: 2, unknownCount: 0, recent: [maintenanceRow, future])
		let (model, _) = await openModel(WireFixture.envelope(sessionScope: .custom(view)))
		let rows = model.sessionsPanel.scope.rows
		XCTAssertEqual(rows.count, 2, "an unknown reason never hides its session")
		XCTAssertEqual(rows[0].origin.reason, t(DesktopCopy.sessionReasonMaintenance))
		XCTAssertEqual(rows[0].origin.ruleVersion, "codex-otel-background-v1")
		XCTAssertEqual(rows[1].origin.reason, t(DesktopCopy.sessionReasonUnknown))
		XCTAssertEqual(rows[1].classLabel, t(DesktopCopy.sessionScopeAuxiliary), "the class shown is the producer's, unchanged")
		XCTAssertEqual(rows[1].origin.ruleVersion, t(DesktopCopy.sessionOriginUnavailable))
	}

	func testOverrideAndCoverageStatesNameTheirCause() async {
		let override: [String: Any] = ["value": "work", "binding_status": "matched", "binding_limited": false, "applied": true]
		let overridden = WireFixture.scopeRecent(id: "task-01", effective: "work", automatic: "unknown", override: override, revision: 2)
		let (model, _) = await openModel(WireFixture.envelope(sessionScope: .custom(WireFixture.sessionScopeView(recent: [overridden]))))
		let row = model.sessionsPanel.scope.rows[0]
		XCTAssertEqual(model.sessionsPanel.scope.notice, t(DesktopCopy.sessionScopeOverride))
		XCTAssertEqual(row.classLabel, t(DesktopCopy.sessionScopeWork) + t(DesktopCopy.sessionOriginOverrideSuffix))
		XCTAssertEqual(row.origin.effective, t(DesktopCopy.sessionScopeWork) + t(DesktopCopy.sessionOriginUserOverrideSuffix))
		XCTAssertEqual(row.origin.automatic, t(DesktopCopy.sessionClassUnknown))
		XCTAssertEqual(row.origin.reason, t(DesktopCopy.sessionReasonMetadataMissing), "the override does not rewrite the automatic reason")
		XCTAssertEqual(row.origin.override, t(DesktopCopy.sessionOriginOverrideApplied, t(DesktopCopy.sessionScopeWork)))
		XCTAssertEqual(row.origin.overrideRevision, DesktopFormat.count(2))

		for (reasons, complete, unknown, notice) in [
			(["origin_source_unavailable"], false, 1 as Any, DesktopCopy.sessionScopeSourceUnavailable),
			(["origin_input_mismatch"], false, 1 as Any, DesktopCopy.sessionScopeInputMismatch),
			([String](), false, 1 as Any, DesktopCopy.sessionScopePartial),
			([String](), true, NSNull() as Any, DesktopCopy.sessionScopePartial),
		] {
			let view = WireFixture.sessionScopeView(unknownCount: unknown, complete: complete, reasons: reasons)
			let (coverage, _) = await openModel(WireFixture.envelope(sessionScope: .custom(view)))
			XCTAssertEqual(coverage.sessionsPanel.scope.notice, t(notice), "\(reasons)")
			if unknown is NSNull {
				XCTAssertEqual(coverage.sessionsPanel.scope.unknownLine, t(DesktopCopy.sessionScopeUnknownUnavailable))
			}
		}
	}

	func testInspectCommandQuotesEveryArgumentAndAlwaysNamesTheID() {
		XCTAssertEqual(SessionOriginCopy.inspectCommand(client: "codex", sessionID: "list"),
			"agentdeck session classify --id 'list' --client 'codex'")
		XCTAssertEqual(SessionOriginCopy.inspectCommand(client: "claude", sessionID: "it's $(touch x)"),
			"agentdeck session classify --id 'it'\\''s $(touch x)' --client 'claude'")
	}

	func testKnownDurationDistinguishesUnmeasuredFromATrustedZero() {
		XCTAssertEqual(SessionOriginCopy.knownDuration(milliseconds: nil), "—")
		XCTAssertNotEqual(SessionOriginCopy.knownDuration(milliseconds: 0), "—")
		XCTAssertEqual(SessionOriginCopy.knownDuration(milliseconds: 5_400_000), DesktopFormat.duration(5_400))
	}

	/// A view the host already holds for this publication (the Widget batch)
	/// answers a client or period change without a read; a selection the batch
	/// does not hold, or a batch of another publication, still reads.
	func testAPreloadedBatchViewAnswersAClientChangeWithoutARead() async {
		let embedded = WireFixture.decodedSessionScopeView(WireFixture.sessionScopeView())
		let claudeWeek = DesktopSessionScopeSelectionV1(scope: .work, client: "claude", period: "7d")
		let batch = WireFixture.decodedSessionScopeView(WireFixture.sessionScopeView(client: "claude", period: "7d", sessions: 7))
		let codexToday = DesktopSessionScopeSelectionV1(scope: .work, client: "codex", period: "today")
		let read = WireFixture.decodedSessionScopeView(WireFixture.sessionScopeView(client: "codex", sessions: 3))
		let reader = StubSessionScopeReader { _ in .view(read) }
		let controller = SessionScopeController(reader: reader)
		controller.beginPresentation(defaults: embedded.default)
		let preloaded: (DesktopSessionScopeSelectionV1) -> DesktopSessionScopeViewV1? = { $0 == claudeWeek ? batch : nil }

		await controller.synchronize(client: "claude", period: "7d", embedded: .present(embedded), publication: "p1", preloaded: preloaded)
		XCTAssertEqual(controller.accepted?.selection, claudeWeek)
		XCTAssertEqual(controller.accepted?.counts?.sessions, 7)
		XCTAssertEqual(controller.request, .idle)
		var requests = await reader.requests
		XCTAssertTrue(requests.isEmpty, "a selection the batch holds must not start a read")

		await controller.synchronize(client: "codex", period: "today", embedded: .present(embedded), publication: "p1", preloaded: preloaded)
		requests = await reader.requests
		XCTAssertEqual(requests, [codexToday], "a selection outside the batch still reads")
		XCTAssertEqual(controller.accepted?.selection, codexToday)
	}
}
