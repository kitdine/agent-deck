import AgentDeckShared
import Foundation
import Observation

/// The saved default scope as Settings knows it. Only a successful read
/// confirms a value; nothing here ever falls back to Work.
enum OriginDefaultState: Equatable {
	case loading
	case unavailable
	case confirmed(DesktopSessionScopeV1, revision: Int64)

	var scope: DesktopSessionScopeV1? {
		if case let .confirmed(scope, _) = self { return scope }
		return nil
	}

	var revision: Int64? {
		if case let .confirmed(_, revision) = self { return revision }
		return nil
	}
}

/// The effective-override list. An unreadable list is never shown as empty.
enum OriginOverrideListState: Equatable {
	case loading
	case unavailable
	case loaded(DesktopOriginOverridePageV1)
}

enum OriginSourceState: Equatable {
	case loading
	case unavailable
	case loaded(DesktopOriginSourceStatusesV1)
}

/// The control that started a write. Its status is shown under that control
/// only; the other control says just why it is locked (ux/settings.md 写入状态与反馈).
enum OriginControl: Equatable {
	case defaultScope
	case manualClassification
}

/// One accepted write: what it changes, the revision it was based on and the
/// request ID the App assigned before sending it. It belongs to the App, not
/// to the window, and a retry resends exactly this operation.
struct OriginPendingOperation: Equatable {
	enum Action: Equatable {
		case saveDefault(DesktopSessionScopeV1)
		case setOverride(DesktopOriginTargetV1, value: String)
		case clearOverride(DesktopOriginTargetV1)
	}

	let action: Action
	let expectedRevision: Int64
	let requestID: String

	var target: DesktopOriginTargetV1? {
		switch action {
		case .saveDefault: nil
		case let .setOverride(target, _), let .clearOverride(target): target
		}
	}

	var control: OriginControl {
		if case .saveDefault = action { return .defaultScope }
		return .manualClassification
	}
}

/// The write lifecycle of the group (ux/settings.md 写入状态与反馈). One write
/// is accepted at a time; other settings groups are never locked by it.
enum OriginOperationState: Equatable {
	case idle
	case saving(OriginPendingOperation)
	/// Certainly not committed. The same operation may be retried, or a new
	/// choice submitted.
	case failed(OriginPendingOperation, DesktopOriginWriteFailure)
	/// The outcome is not established. `receiptChecked` turns true once a
	/// read-back by request ID still found no committed result.
	case unknown(OriginPendingOperation, receiptChecked: Bool)
	/// Changed elsewhere. Only reading the latest values continues.
	case conflict(OriginPendingOperation)

	var pending: OriginPendingOperation? {
		switch self {
		case .idle: nil
		case let .saving(pending), let .failed(pending, _), let .unknown(pending, _), let .conflict(pending): pending
		}
	}
}

/// The settled result the status line reports until the next read-back or
/// operation. It never disappears on a timer.
enum OriginNotice: Equatable {
	case saved(DesktopOriginReceiptV1.Kind)
	case noChange
	case replayed
	case reloaded
	case invalidRequest
}

enum OriginGroupStatus: Equatable {
	case loading
	case loadFailed
	case ready
	case saving
	case failed
	case unknown
	case conflict
}

/// A manual classification as the card offers it. Automatic withdraws the
/// override and nothing else.
enum OriginChoice: String, CaseIterable, Identifiable {
	case automatic
	case work
	case auxiliary

	var id: String { rawValue }
}

/// One looked-up session. `automatic` and `effective` are nil when the
/// classification was not queried: a clear never reads the session index.
struct OriginLookupResult: Equatable {
	struct Automatic: Equatable {
		let value: String
		let reasonCode: String
		let ruleVersion: String?
	}

	struct Override: Equatable {
		let value: String
		let bindingStatus: String
		let applied: Bool
	}

	/// The separate visible query that alone shows a classification again
	/// after a clear.
	enum ClassificationQuery: Equatable {
		case idle
		case reading
		case failed
	}

	let target: DesktopOriginTargetV1
	var automatic: Automatic?
	var effective: String?
	var override: Override?
	var revision: Int64
	var classificationQuery: ClassificationQuery = .idle

	var classificationAvailable: Bool { automatic != nil && effective != nil }

	/// What the card's control shows: the stored override, else Automatic.
	var choice: OriginChoice { override.flatMap { OriginChoice(rawValue: $0.value) } ?? .automatic }

	init(query: DesktopClassificationQueryV1) {
		target = query.target
		if query.classificationAvailable, let origin = query.origin {
			automatic = Automatic(value: origin.automatic, reasonCode: origin.reasonCode, ruleVersion: origin.ruleVersion)
			effective = origin.effective
			override = origin.override.map { Override(value: $0.value, bindingStatus: $0.bindingStatus, applied: $0.applied) }
		}
		revision = query.origin?.revision ?? 0
	}
}

enum OriginLookupState: Equatable {
	case idle
	case searching(DesktopOriginTargetV1)
	case notFound
	case unavailable
	case found(OriginLookupResult)

	/// The session being looked up or shown.
	var target: DesktopOriginTargetV1? {
		switch self {
		case let .searching(target): target
		case let .found(result): result.target
		case .idle, .notFound, .unavailable: nil
		}
	}
}

/// What republishing derived views after a committed write established.
enum OriginPublicationResult: Equatable {
	/// A publication built after the write is installed; it carries this
	/// default (nil when the publication has no session-scope view).
	case published(DesktopSessionScopeDefaultV1?)
	case failed
}

/// Where keyboard focus goes after an override row is removed
/// (ux/settings.md 键盘、焦点与辅助功能).
enum OriginFocusTarget: Hashable {
	case removeOverride(DesktopOriginTargetV1)
	case findSession
}

/// One status line under the control that owns it. Icon, colour and words
/// say the same thing; a request ID appears only while the outcome is unknown.
struct OriginStatusLine: Equatable {
	enum Tone: Equatable {
		case busy
		case good
		case info
		case warning
		case bad
		case neutral
	}

	enum Action: Equatable {
		case readAgain
		case retry
		case checkResult
		case checkAgain
		case retryOriginal
		case loadLatest
	}

	let tone: Tone
	let text: String
	var requestID: String?
	var actions: [Action] = []
}

/// One read-only source row: a dot, the source's name and what its status
/// JSON says. The controlled reason code goes only into the tooltip.
struct OriginSourceRow: Equatable {
	let name: String
	let tone: OriginStatusLine.Tone
	let text: String
	let code: String
}

/// The sentence under the two source rows. An adapter outage leaves manual
/// classification working; an unreadable core leaves nothing readable.
enum OriginSourceNote: Equatable {
	case adapterUnavailable
	case coreUnavailable
	case statusUnavailable
}

/// The Settings "Sessions" group. App-owned: closing the window releases only
/// its drafts, never an accepted operation, its request ID or its result.
@MainActor
@Observable
final class SessionOriginSettingsController {
	static let clients = ["codex", "claude"]

	private(set) var defaultState: OriginDefaultState = .loading
	private(set) var overrides: OriginOverrideListState = .loading
	private(set) var sources: OriginSourceState = .loading
	private(set) var operation: OriginOperationState = .idle
	private(set) var notice: OriginNotice?
	/// The control whose status line shows the current result.
	private(set) var lastControl: OriginControl?
	private(set) var permissionWarning = false
	private(set) var publicationPending = false
	private(set) var lookup: OriginLookupState = .idle

	// Window drafts; closing the window discards them.
	/// Whether the manual-classification sheet is shown.
	var isManaging = false
	private(set) var lookupClient = "codex"
	private(set) var lookupID = ""

	@ObservationIgnored private let transport: any SessionOriginSettingsTransport
	@ObservationIgnored private let publish: @MainActor () async -> OriginPublicationResult
	@ObservationIgnored private let newRequestID: () -> String
	@ObservationIgnored private var overridePage = 1
	/// One generation per kind of value. A read applies its answer only while
	/// its generation is still current: a newer read of the same value, or a
	/// committed write that confirmed a newer one, supersedes every read that
	/// started before it (ASC-SC-R1-F1).
	@ObservationIgnored private var defaultGeneration: UInt64 = 0
	@ObservationIgnored private var overridesGeneration: UInt64 = 0
	@ObservationIgnored private var lookupGeneration: UInt64 = 0
	@ObservationIgnored private var sourcesGeneration: UInt64 = 0
	/// How many answers or confirmed writes have actually been applied to each
	/// value. Unlike a generation, which advances when a read merely starts,
	/// these advance only when a value lands, so they tell whether anything
	/// newer than a core failure report is really known (ASC-SC-R4-F1).
	@ObservationIgnored private var defaultApplied: UInt64 = 0
	@ObservationIgnored private var overridesApplied: UInt64 = 0
	@ObservationIgnored private var lookupApplied: UInt64 = 0
	@ObservationIgnored private var publicationToken: UInt64 = 0
	/// The publication requested by the last committed write; tests await it.
	@ObservationIgnored private(set) var publicationTask: Task<Void, Never>?

	init(
		transport: any SessionOriginSettingsTransport,
		publish: @escaping @MainActor () async -> OriginPublicationResult = { .failed },
		newRequestID: @escaping () -> String = { "settings-" + UUID().uuidString.lowercased() }
	) {
		self.transport = transport
		self.publish = publish
		self.newRequestID = newRequestID
	}

	// MARK: Lifecycle and reads

	/// Opening Settings reads the latest committed values. An operation still
	/// saving, unknown or in conflict keeps the values it was based on until
	/// the user resolves it.
	func open() async {
		switch operation {
		case .idle, .failed: await readValues()
		case .saving, .unknown, .conflict: break
		}
		await readSources()
	}

	/// Closing discards the sheet and the lookup; the operation, its notice
	/// and its request stay.
	func windowClosed() {
		isManaging = false
		lookupClient = "codex"
		lookupID = ""
		lookup = .idle
	}

	func openManage() { isManaging = true }

	func closeManage() { isManaging = false }

	/// Read again after a load failure, or load the latest values after a
	/// conflict, where it ends the old pending operation so the user chooses
	/// again. Source status is read again too: an unreadable core may have
	/// recovered.
	func readLatestValues() async {
		let endsConflict: Bool
		if case .conflict = operation { endsConflict = true } else { endsConflict = false }
		await readValues()
		if case let .found(result) = lookup { await refreshLookup(result.target) }
		if endsConflict, case .conflict = operation {
			operation = .idle
			notice = .reloaded
		}
		await readSources()
	}

	func showOverridePage(_ page: Int) async {
		guard page >= 1 else { return }
		overridePage = page
		await readOverrides()
	}

	var overridePageNumber: Int { overridePage }

	private func readValues() async {
		await readDefault()
		await readOverrides()
	}

	private func readDefault() async {
		defaultGeneration &+= 1
		let generation = defaultGeneration
		let outcome = await transport.readDefaultScope()
		guard generation == defaultGeneration else { return }
		defaultApplied &+= 1
		switch outcome {
		case let .value(defaults):
			guard let scope = defaults.value, let revision = defaults.revision, defaults.confirmed else {
				defaultState = .unavailable
				return
			}
			defaultState = .confirmed(scope, revision: revision)
		case .unavailable:
			defaultState = .unavailable
		}
	}

	private func readOverrides() async {
		overridesGeneration &+= 1
		let generation = overridesGeneration
		let outcome = await transport.listOverrides(page: overridePage, limit: SessionOriginCommand.overridePageLimit)
		guard generation == overridesGeneration else { return }
		switch outcome {
		case let .value(page):
			// A removal that empties the last page shows the page before it.
			if page.items.isEmpty, page.total > 0, overridePage > 1 {
				overridePage = page.pageCount
				await readOverrides()
				return
			}
			overridesApplied &+= 1
			overrides = .loaded(page)
		case .unavailable:
			overridesApplied &+= 1
			overrides = .unavailable
		}
	}

	private func readSources() async {
		sourcesGeneration &+= 1
		let generation = sourcesGeneration
		let appliedAtStart = AppliedValues(defaults: defaultApplied, overrides: overridesApplied, lookup: lookupApplied)
		let outcome = await transport.readOriginSourceStatus()
		guard generation == sourcesGeneration else { return }
		switch outcome {
		case let .value(statuses):
			sources = .loaded(statuses)
			if OriginSettingsCopy.reportsCoreUnavailable(statuses) { invalidateValues(notAppliedSince: appliedAtStart) }
		case .unavailable:
			sources = .unavailable
		}
	}

	private struct AppliedValues {
		let defaults: UInt64
		let overrides: UInt64
		let lookup: UInt64
	}

	/// The default, the override list and the lookup all live in the core. An
	/// explicit core_unavailable answer means a value that nothing has
	/// replaced since that status read began can no longer be shown as current
	/// or written against: it becomes unavailable at once. A read that has
	/// only started proves nothing, so every read of that value still in
	/// flight is discarded too; only a read begun after the report can restore
	/// it (ASC-SC-R4-F1). A value an answer or a confirmed write replaced after
	/// the status read began is newer than the report and stays. A pending
	/// operation keeps its request and its actions; an adapter's own outage
	/// never reaches here.
	private func invalidateValues(notAppliedSince start: AppliedValues) {
		if defaultApplied == start.defaults {
			defaultGeneration &+= 1
			defaultState = .unavailable
		}
		if overridesApplied == start.overrides {
			overridesGeneration &+= 1
			overrides = .unavailable
		}
		if lookupApplied == start.lookup {
			lookupGeneration &+= 1
			lookup = .idle
		}
	}

	// MARK: Lookup

	/// Editing the client or ID clears the old result, so its control can
	/// never act on the new input.
	func setLookupClient(_ client: String) {
		guard client != lookupClient, Self.clients.contains(client) else { return }
		lookupClient = client
		lookup = .idle
	}

	func setLookupID(_ id: String) {
		guard id != lookupID else { return }
		lookupID = id
		lookup = .idle
	}

	var lookupEnabled: Bool {
		guard writesEnabled, !lookupID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return false }
		if case .searching = lookup { return false }
		return true
	}

	/// A read-only, no-scan lookup of exactly this client and complete ID.
	func findSession() async {
		guard lookupEnabled else { return }
		let id = lookupID.trimmingCharacters(in: .whitespacesAndNewlines)
		let target = DesktopOriginTargetV1(client: lookupClient, sessionID: id)
		lookup = .searching(target)
		await refreshLookup(target)
	}

	/// A matched list row fills the lookup with its key and opens its card.
	func openOverride(_ item: DesktopOriginOverrideV1) async {
		guard item.matched, Self.clients.contains(item.client) else { return }
		setLookupClient(item.client)
		setLookupID(item.sessionID)
		await findSession()
	}

	/// Reads the shown session's current classification again after a clear,
	/// or after that read failed.
	func readCurrentClassification() async {
		guard writesEnabled, case let .found(result) = lookup else { return }
		await requeryClassification(result.target)
	}

	/// Queries the target still being looked up or shown. An edit made while
	/// the query ran, a newer query, or a committed write to this session
	/// discards its answer.
	private func refreshLookup(_ target: DesktopOriginTargetV1) async {
		lookupGeneration &+= 1
		let generation = lookupGeneration
		let outcome = await transport.lookupSession(client: target.client, sessionID: target.sessionID)
		guard generation == lookupGeneration, lookup.target == target else { return }
		lookupApplied &+= 1
		switch outcome {
		case let .found(query): lookup = .found(OriginLookupResult(query: query))
		case .notFound: lookup = .notFound
		case .unavailable: lookup = .unavailable
		}
	}

	/// The independent visible query for a card still shown. The card stays:
	/// a failed, not-found or excluded answer leaves the classification
	/// unavailable instead of reusing what was shown before the clear.
	private func requeryClassification(_ target: DesktopOriginTargetV1) async {
		lookupGeneration &+= 1
		let generation = lookupGeneration
		if case var .found(result) = lookup, result.target == target {
			result.classificationQuery = .reading
			lookup = .found(result)
		}
		let outcome = await transport.lookupSession(client: target.client, sessionID: target.sessionID)
		guard generation == lookupGeneration, case var .found(result) = lookup, result.target == target else { return }
		lookupApplied &+= 1
		if case let .found(query) = outcome, query.classificationAvailable {
			lookup = .found(OriginLookupResult(query: query))
		} else {
			if case let .found(query) = outcome { result.revision = query.origin?.revision ?? result.revision }
			result.automatic = nil
			result.effective = nil
			result.classificationQuery = .failed
			lookup = .found(result)
		}
	}

	// MARK: Writes

	/// Writes need confirmed values and no unresolved operation. A failed
	/// operation does not lock: the user may retry it or choose again.
	var writesEnabled: Bool {
		guard case .confirmed = defaultState, case .loaded = overrides else { return false }
		switch operation {
		case .idle, .failed: return true
		case .saving, .unknown, .conflict: return false
		}
	}

	var status: OriginGroupStatus {
		switch operation {
		case .saving: return .saving
		case .failed: return .failed
		case .unknown: return .unknown
		case .conflict: return .conflict
		case .idle:
			if defaultState == .loading || overrides == .loading { return .loading }
			if defaultState == .unavailable || overrides == .unavailable { return .loadFailed }
			return .ready
		}
	}

	/// What the default control selects: the confirmed value, and nothing
	/// while the group is loading or unreadable (ux/settings.md 默认会话范围).
	var selectedDefault: DesktopSessionScopeV1? {
		switch status {
		case .loading, .loadFailed: nil
		case .ready, .saving, .failed, .unknown, .conflict: defaultState.scope
		}
	}

	/// Choosing a default is the save, based on the last confirmed revision.
	/// Choosing the value already confirmed sends nothing.
	func selectDefault(_ scope: DesktopSessionScopeV1) async {
		guard writesEnabled, let revision = defaultState.revision, scope != defaultState.scope else { return }
		await submit(OriginPendingOperation(action: .saveDefault(scope), expectedRevision: revision, requestID: newRequestID()))
	}

	/// Choosing on the card is the save, against the identity and revision
	/// that were shown. Automatic withdraws the override and nothing else;
	/// choosing what is already stored sends nothing.
	func selectOrigin(_ choice: OriginChoice) async {
		guard writesEnabled, case let .found(result) = lookup, choice != result.choice else { return }
		let action: OriginPendingOperation.Action = switch choice {
		case .automatic: .clearOverride(result.target)
		case .work, .auxiliary: .setOverride(result.target, value: choice.rawValue)
		}
		await submit(OriginPendingOperation(action: action, expectedRevision: result.revision, requestID: newRequestID()))
	}

	/// A list row's own key and revision; the session need not be visible.
	func removeOverride(_ item: DesktopOriginOverrideV1) async {
		guard writesEnabled else { return }
		await submit(OriginPendingOperation(action: .clearOverride(item.target), expectedRevision: item.revision, requestID: newRequestID()))
	}

	/// Resends the pending operation unchanged: same request ID, payload and
	/// expected revision. Offered after a certain failure, or after a
	/// read-back that still found no result.
	func retry() async {
		switch operation {
		case let .failed(pending, _), let .unknown(pending, true):
			await submit(pending)
		default:
			return
		}
	}

	var retryAvailable: Bool {
		switch operation {
		case .failed, .unknown(_, true): true
		default: false
		}
	}

	/// Reads the pending request's receipt back by its ID. Finding none keeps
	/// the outcome unknown; it is never read as "not saved".
	func readRequestReceipt() async {
		guard case let .unknown(pending, _) = operation else { return }
		let outcome = await transport.readReceipt(requestID: pending.requestID)
		guard case let .unknown(current, _) = operation, current == pending else { return }
		switch outcome {
		case let .known(found):
			await accept(found, historical: true)
		case .unknown, .unavailable:
			operation = .unknown(pending, receiptChecked: true)
		}
	}

	private func submit(_ pending: OriginPendingOperation) async {
		operation = .saving(pending)
		lastControl = pending.control
		permissionWarning = false
		let outcome: DesktopOriginWriteOutcome
		switch pending.action {
		case let .saveDefault(scope):
			outcome = await transport.saveDefaultScope(scope, expectedRevision: pending.expectedRevision, requestID: pending.requestID)
		case let .setOverride(target, value):
			outcome = await transport.setOverride(target, value: value, expectedRevision: pending.expectedRevision, requestID: pending.requestID)
		case let .clearOverride(target):
			outcome = await transport.clearOverride(target, expectedRevision: pending.expectedRevision, requestID: pending.requestID)
		}
		guard operation == .saving(pending) else { return }
		switch outcome {
		case let .committed(committed):
			await accept(committed, historical: false)
		case .conflict:
			operation = .conflict(pending)
		case let .notSaved(failure):
			operation = .failed(pending, failure)
		case .invalidRequest:
			operation = .idle
			notice = .invalidRequest
		case .unknown:
			operation = .unknown(pending, receiptChecked: false)
		}
	}

	/// A committed receipt ends the pending operation and unlocks the group. A
	/// replayed or read-back receipt describes an earlier commit, so current
	/// values are read again instead of taken from it: an old receipt never
	/// removes a newer override. A fresh receipt confirms a value newer than
	/// any read that started before the write, so those reads are superseded
	/// and cannot roll it back when they return (ASC-SC-R1-F1).
	private func accept(_ committed: DesktopOriginReceiptV1, historical readBack: Bool) async {
		operation = .idle
		permissionWarning = committed.permissionsNotSecured
		let historical = readBack || committed.replayed
		notice = committed.replayed ? .replayed : committed.outcome == .noChange ? .noChange : .saved(committed.kind)
		switch committed.kind {
		case .scopeSet:
			if !historical, let value = committed.value, let scope = DesktopSessionScopeV1(rawValue: value) {
				defaultGeneration &+= 1
				defaultApplied &+= 1
				defaultState = .confirmed(scope, revision: committed.revision)
			} else {
				await readDefault()
			}
		case .overrideSet, .overrideClear:
			if let target = committed.target, lookup.target == target {
				if historical {
					await refreshLookup(target)
				} else if case var .found(result) = lookup {
					lookupGeneration &+= 1
					apply(committed, to: &result)
					// The clear's receipt carries no classification; a card
					// still shown reads it at once with a separate visible
					// query instead of waiting for the user to ask.
					if committed.kind == .overrideClear { await requeryClassification(target) }
				} else {
					// Still being looked up: that query predates the write.
					await refreshLookup(target)
				}
			}
			// The list is always read again; that read supersedes older ones.
			await readOverrides()
		}
		// A core reported unreadable has just answered a write or read-back:
		// read everything again, so the values, the source note and the group's
		// usability agree after this result.
		if sourceNote == .coreUnavailable {
			await readValues()
			await readSources()
		}
		if committed.outcome == .saved { requestPublication(kind: committed.kind, revision: committed.revision) }
	}

	/// A fresh receipt for the session shown in the lookup.
	private func apply(_ committed: DesktopOriginReceiptV1, to result: inout OriginLookupResult) {
		if committed.kind == .overrideSet, let value = committed.value {
			// The confirmed result: the override and the effective class it
			// now produces. Automatic stays what the query found.
			result.override = OriginLookupResult.Override(value: value, bindingStatus: "matched", applied: true)
			result.effective = value
		} else {
			// A clear never queries the session index.
			result.override = nil
			result.automatic = nil
			result.effective = nil
		}
		result.revision = committed.revision
		result.classificationQuery = .idle
		lookupApplied &+= 1
		lookup = .found(result)
	}

	/// Saved is not published: views keep their previous snapshot until a
	/// publication built after this write is installed. A default counts as
	/// published only once the installed publication carries its revision.
	private func requestPublication(kind: DesktopOriginReceiptV1.Kind, revision: Int64) {
		publicationToken &+= 1
		let token = publicationToken
		publicationPending = true
		publicationTask = Task { [weak self] in
			guard let self else { return }
			let result = await self.publish()
			guard token == self.publicationToken else { return }
			guard case let .published(defaults) = result else { return }
			if kind == .scopeSet {
				guard let defaults, defaults.confirmed, let published = defaults.revision, published >= revision else { return }
			}
			self.publicationPending = false
		}
	}

	// MARK: Presentation

	/// The status line under one control. A write's result shows under the
	/// control that started it; the other control only says why it is locked.
	func statusLine(for control: OriginControl) -> OriginStatusLine? {
		switch status {
		case .loading:
			return control == .defaultScope ? OriginStatusLine(tone: .busy, text: t(DesktopCopy.originDefaultReading)) : nil
		case .loadFailed:
			return control == .defaultScope
				? OriginStatusLine(tone: .bad, text: t(DesktopCopy.originDefaultReadFailed), actions: [.readAgain]) : nil
		case .saving, .failed, .unknown, .conflict, .ready:
			break
		}
		let owner = operation.pending?.control ?? lastControl
		guard owner == control else {
			switch status {
			case .saving, .unknown, .conflict:
				return OriginStatusLine(tone: .neutral, text: t(control == .defaultScope ? DesktopCopy.originLockedByManual : DesktopCopy.originLockedByDefault))
			default:
				return nil
			}
		}
		switch operation {
		case let .saving(pending):
			return OriginStatusLine(tone: .busy, text: OriginSettingsCopy.savingText(pending.action))
		case let .failed(pending, _):
			let text = pending.control == .defaultScope
				? t(DesktopCopy.originDefaultNotSaved, defaultState.scope.map(SessionOriginCopy.scope) ?? t(DesktopCopy.quotaUnavailable))
				: t(DesktopCopy.originManualNotSaved)
			return OriginStatusLine(tone: .bad, text: text, actions: [.retry])
		case let .unknown(pending, receiptChecked):
			return receiptChecked
				? OriginStatusLine(tone: .warning, text: t(DesktopCopy.originStillUnknown), requestID: pending.requestID, actions: [.checkAgain, .retryOriginal])
				: OriginStatusLine(tone: .warning, text: t(DesktopCopy.originUnknown), requestID: pending.requestID, actions: [.checkResult])
		case .conflict:
			return OriginStatusLine(tone: .warning, text: t(DesktopCopy.originConflict), actions: [.loadLatest])
		case .idle:
			break
		}
		switch notice {
		case .reloaded:
			let text = control == .defaultScope
				? t(DesktopCopy.originDefaultReloaded, defaultState.scope.map(SessionOriginCopy.scope) ?? t(DesktopCopy.quotaUnavailable))
				: t(DesktopCopy.originManualReloaded)
			return OriginStatusLine(tone: .info, text: text)
		case let .saved(kind):
			if publicationPending {
				return OriginStatusLine(tone: .info, text: t(kind == .scopeSet ? DesktopCopy.originDefaultPublicationPending : DesktopCopy.originManualPublicationPending))
			}
			let key = switch kind {
			case .scopeSet: DesktopCopy.originDefaultSaved
			case .overrideSet: DesktopCopy.originManualSaved
			case .overrideClear: DesktopCopy.originManualCleared
			}
			return OriginStatusLine(tone: .good, text: t(key))
		case .noChange:
			return OriginStatusLine(tone: .neutral, text: t(DesktopCopy.originNoChange))
		case .replayed:
			return OriginStatusLine(tone: .info, text: t(DesktopCopy.originReplayed))
		case .invalidRequest:
			return OriginStatusLine(tone: .bad, text: t(DesktopCopy.originInvalidRequest))
		case nil:
			return nil
		}
	}

	/// Saved, but hardening the state file's permissions failed: shown on its
	/// own line under the control that saved, never as a failure.
	func permissionWarningText(for control: OriginControl) -> String? {
		guard permissionWarning, operation.pending == nil, lastControl == control else { return nil }
		return t(DesktopCopy.originPermissionWarning)
	}

	/// While the sheet is closed, the Manual classification row still reports
	/// anything unfinished: a write in flight, a failure, an unknown result, a
	/// conflict, a pending publication or a warning. Settled results stay in
	/// the sheet.
	var manualRowStatus: OriginStatusLine? {
		if status == .ready {
			switch notice {
			case .saved: if !publicationPending { return nil }
			case .noChange, .replayed, .reloaded: return nil
			case .invalidRequest, nil: break
			}
		}
		return statusLine(for: .manualClassification)
	}

	/// The sheet's status bar. An unreadable core is said plainly, since
	/// nothing in the sheet can be found or changed then.
	var sheetStatus: OriginStatusLine? {
		if status == .loadFailed { return OriginStatusLine(tone: .bad, text: t(DesktopCopy.originSheetCoreUnavailable)) }
		return statusLine(for: .manualClassification)
	}

	/// The Manual classification row's one-line summary.
	var manualSummary: String {
		switch overrides {
		case .loading, .unavailable:
			return t(DesktopCopy.originOverridesSummaryUnavailable)
		case let .loaded(page):
			guard page.total > 0 else { return t(DesktopCopy.originOverridesSummaryNone) }
			let summary = t(DesktopCopy.originOverridesSummaryCount, Int64(page.total))
			// The list carries no unmatched total, so the count is given only
			// when the whole list is in view; a partial count would mislead.
			guard page.pageCount <= 1 else { return summary }
			let unmatched = page.items.filter { !$0.matched }.count
			return unmatched > 0 ? summary + t(DesktopCopy.originOverridesSummaryUnmatched, Int64(unmatched)) : summary
		}
	}

	/// The pending removal of this row, shown as a spinner in place of Remove.
	func isRemoving(_ item: DesktopOriginOverrideV1) -> Bool {
		switch operation {
		case let .saving(pending), let .unknown(pending, _):
			return pending.action == .clearOverride(item.target)
		case .idle, .failed, .conflict:
			return false
		}
	}

	var sourceRows: [OriginSourceRow]? {
		guard case let .loaded(statuses) = sources else { return nil }
		return [OriginSettingsCopy.claudeMemRow(statuses.claudeMem), OriginSettingsCopy.codexBackgroundRow(statuses.codexBackground)]
	}

	var sourceNote: OriginSourceNote? {
		switch sources {
		case .loading:
			return nil
		case .unavailable:
			return .statusUnavailable
		case let .loaded(statuses):
			if OriginSettingsCopy.reportsCoreUnavailable(statuses) { return .coreUnavailable }
			return statuses.claudeMem.available ? nil : .adapterUnavailable
		}
	}
}

/// Copy for the group. Codes this App does not know show generic copy and are
/// never shown as available.
enum OriginSettingsCopy {
	static let coreUnavailable = "core_unavailable"

	/// Either row naming the core. claude-mem checks its configuration before
	/// the core, so the built-in row alone may be the one that does.
	static func reportsCoreUnavailable(_ statuses: DesktopOriginSourceStatusesV1) -> Bool {
		statuses.claudeMem.reasonCode == coreUnavailable || statuses.codexBackground.reasonCode == coreUnavailable
	}

	static func clientLabel(_ client: String) -> String {
		switch client {
		case "codex": "Codex"
		case "claude": "Claude"
		default: client
		}
	}

	static func savingText(_ action: OriginPendingOperation.Action) -> String {
		switch action {
		case let .saveDefault(scope): t(DesktopCopy.originDefaultSaving, SessionOriginCopy.scope(scope))
		case let .setOverride(target, value): t(DesktopCopy.originManualSaving, target.sessionID, SessionOriginCopy.classLabel(value))
		case let .clearOverride(target): t(DesktopCopy.originManualClearing, target.sessionID)
		}
	}

	static func choiceLabel(_ choice: OriginChoice) -> String {
		switch choice {
		case .automatic: t(DesktopCopy.originChoiceAutomatic)
		case .work: t(DesktopCopy.sessionScopeWork)
		case .auxiliary: t(DesktopCopy.sessionScopeAuxiliary)
		}
	}

	/// "Work · manual", "Auxiliary · detected", "Unknown · detected, included
	/// in Work". Nil when the classification was not read.
	static func effectiveText(_ result: OriginLookupResult) -> String? {
		guard let effective = result.effective, result.classificationAvailable else { return nil }
		let source = t(result.override?.applied == true ? DesktopCopy.originCountsManual : DesktopCopy.originCountsDetected)
		let unknown = effective == "work" || effective == "auxiliary" ? "" : t(DesktopCopy.originCountsIncludedInWork)
		return "\(SessionOriginCopy.classLabel(effective)) · \(source)\(unknown)"
	}

	/// "Auxiliary · reason". The controlled reason code and rule version stay
	/// in the tooltip.
	static func automaticText(_ automatic: OriginLookupResult.Automatic?) -> String? {
		guard let automatic else { return nil }
		return "\(SessionOriginCopy.classLabel(automatic.value)) · \(SessionOriginCopy.reason(automatic.reasonCode))"
	}

	static func automaticHelp(_ automatic: OriginLookupResult.Automatic?) -> String? {
		guard let automatic else { return nil }
		return "\(automatic.reasonCode) · \(automatic.ruleVersion ?? "—")"
	}

	private static let claudeMemStates: [String: (OriginStatusLine.Tone, String)] = [
		"source_not_configured": (.neutral, DesktopCopy.originSourceNotConfigured),
		"source_unavailable": (.warning, DesktopCopy.originSourceMetadataUnreadable),
		"source_version_unsupported": (.warning, DesktopCopy.originSourceVersionUnsupported),
		"source_disabled": (.neutral, DesktopCopy.originSourceDisabled),
		"source_not_yet_read": (.neutral, DesktopCopy.originSourceNotYetRead),
		"source_config_unreadable": (.warning, DesktopCopy.originSourceConfigUnreadable),
		coreUnavailable: (.bad, DesktopCopy.originSourceCoreUnavailable),
	]

	/// The configurable adapter, from its status JSON alone.
	static func claudeMemRow(_ status: DesktopOriginSourceStatusV1) -> OriginSourceRow {
		let name = "claude-mem"
		if status.available, status.reasonCode == nil {
			let text = status.supportedVersion.map { t(DesktopCopy.originSourceAvailableVersion, $0) } ?? t(DesktopCopy.switchReady)
			return OriginSourceRow(name: name, tone: .good, text: text, code: "available")
		}
		guard let code = status.reasonCode, let (tone, key) = claudeMemStates[code] else {
			return OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originSourceReasonUnknown), code: status.reasonCode ?? "unknown")
		}
		return OriginSourceRow(name: name, tone: tone, text: t(key), code: code)
	}

	/// The built-in Codex background row (ux/settings.md five states). Every
	/// "no records" state is a confirmed, non-error state.
	static func codexBackgroundRow(_ status: DesktopOriginSourceStatusV1) -> OriginSourceRow {
		let name = t(DesktopCopy.originCodexName)
		switch (status.reasonCode, status.available, status.backgroundSessions) {
		case let (nil, true, count?) where count > 0:
			return OriginSourceRow(name: name, tone: .good, text: t(DesktopCopy.originCodexRecords, Int64(count)), code: "available")
		case ("otel_receiver_disabled"?, true, 0?):
			return OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originCodexReceiverOff), code: "otel_receiver_disabled")
		case ("no_background_sessions"?, true, 0?):
			return OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originCodexNoRecords), code: "no_background_sessions")
		case ("otel_receiver_unknown"?, true, 0?):
			return OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originCodexReceiverUnknown), code: "otel_receiver_unknown")
		case (coreUnavailable?, false, nil):
			return OriginSourceRow(name: name, tone: .bad, text: t(DesktopCopy.originSourceCoreUnavailable), code: coreUnavailable)
		default:
			return OriginSourceRow(name: name, tone: .neutral, text: t(DesktopCopy.originSourceReasonUnknown), code: status.reasonCode ?? "unknown")
		}
	}

	/// After a row's removal: the next row's remove button, else the previous
	/// one, else the lookup entry. A failed removal keeps its own row.
	static func focusAfterRemoval(of removed: DesktopOriginTargetV1, at index: Int, remaining: [DesktopOriginOverrideV1]) -> OriginFocusTarget {
		if remaining.contains(where: { $0.target == removed }) { return .removeOverride(removed) }
		guard !remaining.isEmpty else { return .findSession }
		return .removeOverride(remaining[min(index, remaining.count - 1)].target)
	}
}
