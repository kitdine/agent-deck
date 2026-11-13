import AgentDeckShared
import Foundation
import Observation

/// The Popover's session-scope state for one window. It owns the temporary
/// scope, the last complete view it accepted, and the request in flight, so
/// switching tabs or filters never loses or resurrects that state; only a new
/// open starts again from the confirmed default.
///
/// A temporary scope is never saved: nothing here writes a preference, a
/// classification, or anything a Widget or the CLI reads.
@MainActor
@Observable
final class SessionScopeController {
	enum Request: Equatable, Sendable {
		case idle
		case loading(DesktopSessionScopeSelectionV1)
		case failed(DesktopSessionScopeSelectionV1)
	}

	/// The scope asked for in this open; nil until a confirmed default or an
	/// explicit choice names one. It is never filled in with Work.
	private(set) var requestedScope: DesktopSessionScopeV1?
	/// The last complete, same-publication view accepted for display. It is
	/// always shown with its own selection and time.
	private(set) var accepted: DesktopSessionScopeViewV1?
	private(set) var request: Request = .idle
	/// The safe reason of the last unavailable answer, when it gave one.
	private(set) var failureReason: String?
	/// Which publication the accepted view was checked against. A later
	/// publication makes it a candidate for replacement, not invalid.
	private(set) var acceptedPublication: String?

	@ObservationIgnored private var scopeInitialized = false
	/// Every request and every new open takes a new generation; an answer for
	/// an older one is discarded, so a late response never overwrites a newer
	/// selection.
	@ObservationIgnored private var generation: UInt64 = 0
	@ObservationIgnored private let reader: (any DesktopSessionScopeReading)?
	@ObservationIgnored let recentLimit: Int

	init(reader: (any DesktopSessionScopeReading)?, recentLimit: Int = EmbeddedHelperRunner.defaultRecentLimit) {
		self.reader = reader
		self.recentLimit = recentLimit
	}

	/// A new open starts from the confirmed default. An unconfirmed default
	/// names no scope; the last reliable view stays, still labelled with its
	/// own selection and time, and pending answers of the previous open are
	/// discarded.
	func beginPresentation(defaults: DesktopSessionScopeDefaultV1?) {
		generation &+= 1
		request = .idle
		failureReason = nil
		scopeInitialized = false
		requestedScope = nil
		adoptDefault(defaults)
	}

	/// Scope buttons are inert while a request is pending and after one failed
	/// with a previous view still shown; retry is the way forward then.
	var acceptsChoices: Bool {
		switch request {
		case .idle: true
		case .loading: false
		case .failed: accepted == nil
		}
	}

	/// A temporary choice for this open. It writes nothing.
	func choose(_ scope: DesktopSessionScopeV1) {
		guard acceptsChoices else { return }
		requestedScope = scope
		scopeInitialized = true
		if case .failed = request {
			request = .idle
			failureReason = nil
		}
	}

	/// Brings the shown view to the selection the window now asks for. A view
	/// already accepted for this publication, or the publication's own view of
	/// exactly this selection, needs no request; anything else is read
	/// explicitly. A failed selection waits for an explicit retry.
	///
	/// Each call names the current target, so it supersedes any read still in
	/// flight — even one for this same selection. That read belongs to the
	/// previous synchronization, which SwiftUI cancels when the key changes or
	/// the area leaves; waiting on it instead could end with no request at all.
	///
	/// `preloaded` answers a selection from views the host already holds for
	/// this publication (the Widget batch), so changing client or period within
	/// the default scope starts no read.
	func synchronize(
		client: String, period: String, embedded: DesktopSessionScopeFieldV1, publication: String?,
		preloaded: (DesktopSessionScopeSelectionV1) -> DesktopSessionScopeViewV1? = { _ in nil }
	) async {
		adoptDefault(embedded.view?.default)
		guard let scope = requestedScope else { return }
		let desired = DesktopSessionScopeSelectionV1(scope: scope, client: client, period: period)
		if case let .failed(failed) = request, failed == desired { return }
		if let accepted, accepted.selection == desired, acceptedPublication == publication {
			// Returning to the shown view ends the read that left it, so its
			// late answer cannot replace this selection.
			generation &+= 1
			request = .idle
			return
		}
		if let view = embedded.view, view.available, view.selection == desired {
			accept(view, publication: publication)
			return
		}
		if let view = preloaded(desired), view.available, view.selection == desired {
			accept(view, publication: publication)
			return
		}
		// The publication's own view is the previous data a first request
		// keeps showing, under its own selection label.
		if accepted == nil, let view = embedded.view, view.available {
			accepted = view
			acceptedPublication = publication
		}
		await load(desired, publication: publication)
	}

	/// The scope this open shows before `synchronize` has run: an explicit
	/// choice, else the confirmed default of the installed publication. Never
	/// Work by assumption.
	func effectiveScope(defaults: DesktopSessionScopeDefaultV1?) -> DesktopSessionScopeV1? {
		if let requestedScope { return requestedScope }
		guard !scopeInitialized, let defaults, defaults.confirmed else { return nil }
		return defaults.value
	}

	/// What the area shows now. Before the first synchronization this is the
	/// publication's own view when it answers exactly the effective selection —
	/// the view `synchronize` would accept without a request — so an open never
	/// flashes an unconfirmed or empty state while that answer is already here.
	func displayedView(embedded: DesktopSessionScopeFieldV1, client: String, period: String) -> DesktopSessionScopeViewV1? {
		if let accepted { return accepted }
		guard let view = embedded.view, view.available, let scope = effectiveScope(defaults: view.default),
			view.selection == DesktopSessionScopeSelectionV1(scope: scope, client: client, period: period)
		else { return nil }
		return view
	}

	/// Retries exactly the failed selection.
	func retry(publication: String?) async {
		guard case let .failed(selection) = request else { return }
		await load(selection, publication: publication)
	}

	private func adoptDefault(_ defaults: DesktopSessionScopeDefaultV1?) {
		guard !scopeInitialized, let defaults, defaults.confirmed, let value = defaults.value else { return }
		requestedScope = value
		scopeInitialized = true
	}

	private func accept(_ view: DesktopSessionScopeViewV1, publication: String?) {
		generation &+= 1
		accepted = view
		acceptedPublication = publication
		request = .idle
		failureReason = nil
	}

	private func load(_ selection: DesktopSessionScopeSelectionV1, publication: String?) async {
		generation &+= 1
		let id = generation
		request = .loading(selection)
		failureReason = nil
		guard let reader else {
			request = .failed(selection)
			return
		}
		do {
			let view = try await reader.sessionScopeView(selection, recentLimit: recentLimit)
			guard isCurrent(id, selection) else { return }
			guard view.available, view.selection == selection else {
				request = .failed(selection)
				failureReason = view.reason
				return
			}
			accepted = view
			acceptedPublication = publication
			request = .idle
		} catch {
			guard isCurrent(id, selection) else { return }
			// A cancelled wait (the window closed or the panel left) is not a
			// failure of the request; the next appearance asks again.
			request = Task.isCancelled || error is CancellationError || (error as? HelperExecutionError) == .cancelled
				? .idle : .failed(selection)
		}
	}

	/// An answer is accepted only while it is still the current request for
	/// the same selection.
	private func isCurrent(_ id: UInt64, _ selection: DesktopSessionScopeSelectionV1) -> Bool {
		id == generation && request == .loading(selection)
	}
}

// MARK: Presentation

/// The session area's state, in the final menu-bar surface's vocabulary.
enum SessionScopeSurfaceState: Equatable, Sendable {
	/// The publication predates the session-scope view.
	case legacy
	/// The default could not be confirmed and no reliable view exists.
	case defaultUnknown
	/// No complete view exists: nothing is counted.
	case unavailable
	/// A complete view is shown, possibly while a newer request is pending
	/// or after one failed.
	case ready
}

struct SessionScopeChoice: Identifiable, Equatable, Sendable {
	let id: DesktopSessionScopeV1
	let label: String
	let pressed: Bool
}

/// A session's read-only origin detail. It carries only the architecture's
/// safe fields: no path, text, prompt, or credential.
struct SessionOriginDetailModel: Equatable, Sendable {
	let sessionID: String
	let subtitle: String
	let effective: String
	let automatic: String
	let reason: String
	let ruleVersion: String
	let override: String
	let overrideRevision: String
	let command: String
}

struct SessionScopeRow: Identifiable, Equatable, Sendable {
	let id: String
	let title: String
	let detail: String
	let classLabel: String
	let when: String
	let origin: SessionOriginDetailModel
}

struct SessionScopePanelModel: Equatable, Sendable {
	let state: SessionScopeSurfaceState
	let choices: [SessionScopeChoice]
	let choicesEnabled: Bool
	/// Identity of the selection the shown figures answer; a change closes an
	/// open detail, whose identity would no longer match.
	let selectionKey: String
	let selectionLine: String?
	let notice: String?
	let unknownLine: String?
	let snapshotTime: String?
	let retryLabel: String?
	let hint: String?
	let stats: [StatChip]
	let missingDurationNote: String?
	let rows: [SessionScopeRow]
	let emptyCopy: String?
	let workSignals: WorkSignalPanelModel
	/// The one result or failure summary the live region speaks.
	let announcement: String?
}

/// Classification copy. The reason enum is closed, but a code this App does
/// not know shows the generic copy; it never changes the class shown or
/// implies auxiliary.
enum SessionOriginCopy {
	static let reasonKeys: [String: String] = [
		"maintenance_origin": DesktopCopy.sessionReasonMaintenance,
		"origin_metadata_missing": DesktopCopy.sessionReasonMetadataMissing,
		"task_child_unconfirmed": DesktopCopy.sessionReasonTaskChild,
		"conflicting_origin": DesktopCopy.sessionReasonConflicting,
		"origin_source_unavailable": DesktopCopy.sessionReasonSourceUnavailable,
		"origin_rule_unsupported": DesktopCopy.sessionReasonRuleUnsupported,
		"origin_input_mismatch": DesktopCopy.sessionReasonInputMismatch,
	]

	static func reason(_ code: String) -> String {
		t(reasonKeys[code] ?? DesktopCopy.sessionReasonUnknown)
	}

	static func scope(_ scope: DesktopSessionScopeV1) -> String {
		switch scope {
		case .work: t(DesktopCopy.sessionScopeWork)
		case .auxiliary: t(DesktopCopy.sessionScopeAuxiliary)
		case .all: t(DesktopCopy.clientAll)
		}
	}

	/// The three classes; any other value reads as Unknown rather than as a
	/// guessed class.
	static func classLabel(_ value: String) -> String {
		switch value {
		case "work": t(DesktopCopy.sessionScopeWork)
		case "auxiliary": t(DesktopCopy.sessionScopeAuxiliary)
		default: t(DesktopCopy.sessionClassUnknown)
		}
	}

	/// The read-only inspect command. Every argument is single-quoted for a
	/// POSIX shell and `--id` always names the session, so an ID that equals a
	/// classify subcommand is never reinterpreted. The visible and copied text
	/// come from this one function.
	static func inspectCommand(client: String, sessionID: String) -> String {
		"agentdeck session classify --id \(shellQuoted(sessionID)) --client \(shellQuoted(client))"
	}

	static func shellQuoted(_ value: String) -> String {
		"'" + value.replacingOccurrences(of: "'", with: "'\\''") + "'"
	}

	/// The time a view was published, always stated in UTC so a reader in
	/// another zone sees the same instant the producer recorded.
	static func snapshotTime(_ value: String) -> String? {
		guard let date = DesktopFormat.timestamp(value) else { return nil }
		var style = Date.FormatStyle(date: .numeric, time: .shortened).locale(DesktopLocale.current)
		style.timeZone = TimeZone(identifier: "UTC") ?? .gmt
		return "\(date.formatted(style)) UTC"
	}

	/// A known-duration subtotal: nil is unmeasured and shows a dash; 0 is a
	/// trusted empty selection and shows zero, never a dash.
	static func knownDuration(milliseconds: Int64?) -> String {
		guard let milliseconds else { return "—" }
		let seconds = max(0, milliseconds / 1000)
		guard seconds >= 60 else {
			return Duration.seconds(0).formatted(
				.units(allowed: [.minutes], width: .narrow, zeroValueUnits: .show(length: 1)).locale(DesktopLocale.current)
			)
		}
		return DesktopFormat.duration(seconds)
	}
}

extension MenuBarViewModel {
	/// The identity a session-scope answer is checked against: the installed
	/// publication, or the snapshot time of an unpublished snapshot.
	var sessionScopePublicationKey: String? {
		coordinator.servingHeader?.publicationID ?? snapshot?.generatedAt
	}

	var sessionScopeField: DesktopSessionScopeFieldV1 {
		snapshot?.sessionScopeView ?? .absent
	}

	/// Called when the Popover opens: the temporary scope starts again from
	/// the confirmed default.
	func beginSessionScopePresentation() {
		sessionScope.beginPresentation(defaults: sessionScopeField.view?.default)
	}

	func chooseSessionScope(_ scope: DesktopSessionScopeV1) {
		sessionScope.choose(scope)
	}

	/// Brings the session area to the current client, period and scope. It
	/// reads committed state only and never starts a scan.
	func synchronizeSessionScope() async {
		guard case .present = sessionScopeField else { return }
		await sessionScope.synchronize(
			client: selectedClient, period: selectedPeriod,
			embedded: sessionScopeField, publication: sessionScopePublicationKey,
			preloaded: { [coordinator, key = sessionScopePublicationKey] in coordinator.sessionScopeBatchView($0, publication: key) }
		)
	}

	func retrySessionScope() async {
		await sessionScope.retry(publication: sessionScopePublicationKey)
	}

	/// The key that changes whenever the session area may need another view.
	var sessionScopeSynchronizationKey: String {
		[
			sessionScope.effectiveScope(defaults: sessionScopeField.view?.default)?.rawValue ?? "-", selectedClient, selectedPeriod,
			sessionScopePublicationKey ?? "-", sessionScopeField.view == nil ? "no-view" : "view",
		].joined(separator: "|")
	}

	/// Whether `sessionScopePanel` is ready, decided without building it. The
	/// tab strip asks this for every tab on every layout; building the whole
	/// panel model there cost each open and each tab switch several frames.
	var sessionScopePanelIsReady: Bool {
		guard case .present = sessionScopeField,
			let view = sessionScope.displayedView(embedded: sessionScopeField, client: selectedClient, period: selectedPeriod)
		else { return false }
		return view.selection != nil && view.counts != nil
	}

	var sessionScopePanel: SessionScopePanelModel {
		let controller = sessionScope
		switch sessionScopeField {
		case .absent:
			return SessionScopePanelModel.message(.legacy, notice: t(DesktopCopy.sessionScopeLegacy), hint: t(DesktopCopy.sessionScopeLegacyHint), signals: workSignalPanel(nil, period: selectedPeriod, client: selectedClient))
		case .invalid:
			return SessionScopePanelModel.message(.unavailable, notice: t(DesktopCopy.sessionScopeUnavailable), hint: t(DesktopCopy.sessionScopeUnavailableHint), signals: workSignalPanel(nil, period: selectedPeriod, client: selectedClient))
		case .present:
			break
		}
		let displayed = controller.displayedView(embedded: sessionScopeField, client: selectedClient, period: selectedPeriod)
		guard let view = displayed, let selection = view.selection, let counts = view.counts else {
			if controller.effectiveScope(defaults: sessionScopeField.view?.default) == nil {
				return SessionScopePanelModel.message(.defaultUnknown, notice: t(DesktopCopy.sessionScopeDefaultUnknown), hint: t(DesktopCopy.sessionScopeDefaultUnknownHint), signals: workSignalPanel(nil, period: selectedPeriod, client: selectedClient))
			}
			return unavailableScopePanel(controller)
		}
		let choices = DesktopSessionScopeV1.allCases.map {
			SessionScopeChoice(id: $0, label: SessionOriginCopy.scope($0), pressed: $0 == selection.scope)
		}
		let unknown = view.originCoverage?.unknownCount
		let unknownLine = selection.scope == .auxiliary ? nil
			: unknown.map { t(DesktopCopy.sessionScopeUnknownIncluded, Int64($0)) } ?? t(DesktopCopy.sessionScopeUnknownUnavailable)
		let hasOverride = view.recent.contains { $0.origin.override?.applied == true }
		let reasons = Set(view.originCoverage?.reasons ?? [])
		var notice: String?
		var snapshotTime: String?
		var retryLabel: String?
		var announcement: String?
		switch controller.request {
		case let .loading(pending):
			notice = pending == selection
				? t(DesktopCopy.sessionScopeStale)
				: t(DesktopCopy.sessionScopeLoading, SessionOriginCopy.scope(pending.scope), SessionOriginCopy.scope(selection.scope))
			snapshotTime = view.generatedAt.flatMap(SessionOriginCopy.snapshotTime).map { t(DesktopCopy.sessionScopeSnapshotTime, $0) }
		case let .failed(failed):
			notice = t(DesktopCopy.sessionScopeFailed, SessionOriginCopy.scope(failed.scope), SessionOriginCopy.scope(selection.scope))
			snapshotTime = view.generatedAt.flatMap(SessionOriginCopy.snapshotTime).map { t(DesktopCopy.sessionScopeSnapshotTime, $0) }
			retryLabel = t(DesktopCopy.sessionScopeRetry, SessionOriginCopy.scope(failed.scope))
			announcement = notice
		case .idle:
			if qualifiers.contains(.stale) {
				notice = t(DesktopCopy.sessionScopeStale)
				snapshotTime = view.generatedAt.flatMap(SessionOriginCopy.snapshotTime).map { t(DesktopCopy.sessionScopeSnapshotTime, $0) }
			} else if reasons.contains("origin_source_unavailable") {
				notice = t(DesktopCopy.sessionScopeSourceUnavailable)
			} else if reasons.contains("origin_input_mismatch") {
				notice = t(DesktopCopy.sessionScopeInputMismatch)
			} else if view.originCoverage?.complete == false || unknown == nil {
				notice = t(DesktopCopy.sessionScopePartial)
			} else if counts.sessions == 0 {
				notice = t(DesktopCopy.sessionScopeEmpty)
			} else if hasOverride {
				notice = t(DesktopCopy.sessionScopeOverride)
			}
		}
		if announcement == nil, case .idle = controller.request {
			announcement = t(DesktopCopy.sessionScopeAnnouncement, SessionOriginCopy.scope(selection.scope), Int64(counts.sessions))
		}
		let rows = view.recent.map(sessionScopeRow)
		let stats = [
			StatChip(id: "sessions", label: t(DesktopCopy.sessionScopeSessions), value: DesktopFormat.count(Int64(counts.sessions)), note: nil),
			StatChip(id: "projects", label: t(DesktopCopy.sessionsProjects), value: DesktopFormat.count(Int64(counts.projects)), note: nil),
			StatChip(id: "known-duration", label: t(DesktopCopy.sessionScopeKnownDuration), value: SessionOriginCopy.knownDuration(milliseconds: counts.knownDurationMS), note: nil),
		]
		return SessionScopePanelModel(
			state: .ready,
			choices: choices,
			choicesEnabled: controller.acceptsChoices,
			selectionKey: "\(selection.scope.rawValue)|\(selection.client)|\(selection.period)",
			selectionLine: [SessionOriginCopy.scope(selection.scope), sessionScopeClientLabel(selection.client), periodLabel(selection.period)].joined(separator: " · "),
			notice: notice,
			unknownLine: unknownLine,
			snapshotTime: snapshotTime,
			retryLabel: retryLabel,
			hint: nil,
			stats: stats,
			missingDurationNote: counts.missingDurationCount > 0 ? t(DesktopCopy.sessionScopeMissingDuration, Int64(counts.missingDurationCount)) : nil,
			rows: rows,
			emptyCopy: rows.isEmpty ? t(DesktopCopy.sessionScopeNoMatches) : nil,
			workSignals: workSignalPanel(view.workSignals, period: selection.period, client: selection.client),
			announcement: announcement
		)
	}

	private func unavailableScopePanel(_ controller: SessionScopeController) -> SessionScopePanelModel {
		let requested = controller.effectiveScope(defaults: sessionScopeField.view?.default)
		let choices = DesktopSessionScopeV1.allCases.map {
			SessionScopeChoice(id: $0, label: SessionOriginCopy.scope($0), pressed: $0 == requested)
		}
		var notice = t(DesktopCopy.sessionScopeUnavailable)
		var hint: String? = t(DesktopCopy.sessionScopeUnavailableHint)
		var retryLabel: String?
		switch controller.request {
		case let .loading(pending):
			notice = t(DesktopCopy.sessionScopeLoadingFirst, SessionOriginCopy.scope(pending.scope))
			hint = nil
		case let .failed(failed):
			retryLabel = t(DesktopCopy.sessionScopeRetry, SessionOriginCopy.scope(failed.scope))
		case .idle:
			break
		}
		return SessionScopePanelModel(
			state: .unavailable, choices: choices, choicesEnabled: controller.acceptsChoices,
			selectionKey: "unavailable", selectionLine: nil, notice: notice, unknownLine: nil, snapshotTime: nil,
			retryLabel: retryLabel, hint: hint, stats: [], missingDurationNote: nil, rows: [], emptyCopy: nil,
			workSignals: workSignalPanel(nil, period: selectedPeriod, client: selectedClient),
			announcement: controller.request == .idle || retryLabel != nil ? notice : nil
		)
	}

	private func sessionScopeClientLabel(_ client: String) -> String {
		client == "all" ? t(DesktopCopy.sessionScopeAllClients) : clientLabel(client)
	}

	private func sessionScopeRow(_ item: DesktopSessionScopeRecentV1) -> SessionScopeRow {
		let origin = item.origin
		let applied = origin.override?.applied == true
		let effective = SessionOriginCopy.classLabel(origin.effective) + (applied ? t(DesktopCopy.sessionOriginUserOverrideSuffix) : "")
		let overrideText: String
		if let override = origin.override {
			overrideText = override.applied
				? t(DesktopCopy.sessionOriginOverrideApplied, SessionOriginCopy.classLabel(override.value))
				: t(DesktopCopy.sessionOriginOverrideUnmatched, SessionOriginCopy.classLabel(override.value))
		} else {
			overrideText = t(DesktopCopy.sessionOriginOverrideNone)
		}
		let project = item.project ?? t(DesktopCopy.sessionsProjectUnnamed)
		var duration = "—"
		if let firstAt = item.firstAt, let lastAt = item.lastAt,
			let first = DesktopFormat.timestamp(firstAt), let last = DesktopFormat.timestamp(lastAt), last >= first {
			duration = SessionOriginCopy.knownDuration(milliseconds: Int64(last.timeIntervalSince(first) * 1000))
		}
		return SessionScopeRow(
			id: "\(item.client)/\(item.sessionID)",
			title: project,
			detail: "\(clientLabel(item.client)) · \(item.sessionID)",
			classLabel: SessionOriginCopy.classLabel(origin.effective) + (applied ? t(DesktopCopy.sessionOriginOverrideSuffix) : ""),
			when: duration,
			origin: SessionOriginDetailModel(
				sessionID: item.sessionID,
				subtitle: "\(clientLabel(item.client)) · \(project)",
				effective: effective,
				automatic: SessionOriginCopy.classLabel(origin.automatic),
				reason: SessionOriginCopy.reason(origin.reasonCode),
				ruleVersion: origin.ruleVersion ?? t(DesktopCopy.sessionOriginUnavailable),
				override: overrideText,
				overrideRevision: DesktopFormat.count(origin.revision),
				command: SessionOriginCopy.inspectCommand(client: item.client, sessionID: item.sessionID)
			)
		)
	}
}

extension SessionScopePanelModel {
	static func message(_ state: SessionScopeSurfaceState, notice: String, hint: String, signals: WorkSignalPanelModel) -> SessionScopePanelModel {
		SessionScopePanelModel(
			state: state, choices: [], choicesEnabled: false, selectionKey: "\(state)", selectionLine: nil,
			notice: notice, unknownLine: nil, snapshotTime: nil, retryLabel: nil, hint: hint, stats: [],
			missingDurationNote: nil, rows: [], emptyCopy: nil, workSignals: signals, announcement: nil
		)
	}
}
