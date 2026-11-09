import Foundation

// The Settings bridge to the session-origin CLI contract (architecture §5 and
// §7, ux/settings.md 已定协议). Settings reads and writes only through these Go
// JSON commands; nothing here opens SQLite or a configuration file.

/// One override key: an explicit client and a complete canonical session ID.
public struct DesktopOriginTargetV1: Codable, Equatable, Hashable, Sendable {
	public let client: String
	public let sessionID: String

	public init(client: String, sessionID: String) {
		self.client = client
		self.sessionID = sessionID
	}

	enum CodingKeys: String, CodingKey {
		case client
		case sessionID = "session_id"
	}
}

/// `session classify --id <id> --client <client>`: one visible session's
/// current classification. The query is read only.
public struct DesktopClassificationQueryV1: Codable, Equatable, Sendable {
	public let target: DesktopOriginTargetV1
	public let classificationAvailable: Bool
	public let origin: DesktopSessionOriginV1?

	public init(target: DesktopOriginTargetV1, classificationAvailable: Bool, origin: DesktopSessionOriginV1?) {
		self.target = target
		self.classificationAvailable = classificationAvailable
		self.origin = origin
	}

	enum CodingKeys: String, CodingKey {
		case target, origin
		case classificationAvailable = "classification_available"
	}
}

/// The receipt of one recorded operation: `session scope set`,
/// `session classify --set/--clear`, or a `--request` read-back of one. It
/// describes that operation, never necessarily the current state.
public struct DesktopOriginReceiptV1: Codable, Equatable, Sendable {
	public enum Kind: String, Codable, Sendable {
		case scopeSet = "scope_set"
		case overrideSet = "override_set"
		case overrideClear = "override_clear"
	}

	public enum Outcome: String, Codable, Sendable {
		case saved
		case noChange = "no_change"
	}

	public static let permissionsWarning = "origin_permissions_not_secured"

	public let requestID: String
	public let kind: Kind
	public let target: DesktopOriginTargetV1?
	public let outcome: Outcome
	/// The value the operation wrote; nil for a clear.
	public let value: String?
	public let revision: Int64
	public let replayed: Bool
	public let warnings: [String]
	/// True when the write committed and derived views have not been
	/// republished yet. A read-back never claims a pending publication.
	public let publicationPending: Bool
	/// A clear never reads the session index: its classification is fixed
	/// `not_queried`. Nil on other kinds.
	public let classificationReason: String?

	public init(
		requestID: String, kind: Kind, target: DesktopOriginTargetV1? = nil, outcome: Outcome, value: String?, revision: Int64,
		replayed: Bool = false, warnings: [String] = [], publicationPending: Bool = false, classificationReason: String? = nil
	) {
		self.requestID = requestID
		self.kind = kind
		self.target = target
		self.outcome = outcome
		self.value = value
		self.revision = revision
		self.replayed = replayed
		self.warnings = warnings
		self.publicationPending = publicationPending
		self.classificationReason = classificationReason
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		requestID = try container.decode(String.self, forKey: .requestID)
		kind = try container.decode(Kind.self, forKey: .kind)
		target = try container.decodeIfPresent(DesktopOriginTargetV1.self, forKey: .target)
		outcome = try container.decode(Outcome.self, forKey: .outcome)
		value = try container.decodeIfPresent(String.self, forKey: .value)
		revision = try container.decode(Int64.self, forKey: .revision)
		replayed = try container.decodeIfPresent(Bool.self, forKey: .replayed) ?? false
		warnings = try container.decodeIfPresent([String].self, forKey: .warnings) ?? []
		publicationPending = try container.decodeIfPresent(Bool.self, forKey: .publicationPending) ?? false
		classificationReason = try container.decodeIfPresent(String.self, forKey: .classificationReason)
		if kind == .scopeSet, target != nil { throw DecodingError.dataCorruptedError(forKey: .target, in: container, debugDescription: "scope receipt with a target") }
		if kind != .scopeSet, target == nil { throw DecodingError.dataCorruptedError(forKey: .target, in: container, debugDescription: "override receipt without a target") }
	}

	public var permissionsNotSecured: Bool { warnings.contains(Self.permissionsWarning) }

	enum CodingKeys: String, CodingKey {
		case kind, target, outcome, value, revision, replayed, warnings
		case requestID = "request_id"
		case publicationPending = "publication_pending"
		case classificationReason = "classification_reason"
	}
}

/// One effective user override; unmatched ones are listed too.
public struct DesktopOriginOverrideV1: Codable, Equatable, Hashable, Sendable {
	public let client: String
	public let sessionID: String
	public let value: String
	public let bindingStatus: String
	public let bindingLimited: Bool
	public let revision: Int64

	public init(client: String, sessionID: String, value: String, bindingStatus: String, bindingLimited: Bool = false, revision: Int64) {
		self.client = client
		self.sessionID = sessionID
		self.value = value
		self.bindingStatus = bindingStatus
		self.bindingLimited = bindingLimited
		self.revision = revision
	}

	public var target: DesktopOriginTargetV1 { DesktopOriginTargetV1(client: client, sessionID: sessionID) }
	/// Anything other than a confirmed match is not applied.
	public var matched: Bool { bindingStatus == "matched" }

	enum CodingKeys: String, CodingKey {
		case client, value, revision
		case sessionID = "session_id"
		case bindingStatus = "binding_status"
		case bindingLimited = "binding_limited"
	}
}

/// One page of `session classify list`, ordered by client and session ID. The
/// total and the rows come from the same read.
public struct DesktopOriginOverridePageV1: Codable, Equatable, Sendable {
	public let items: [DesktopOriginOverrideV1]
	public let page: Int
	public let limit: Int
	public let total: Int

	public init(items: [DesktopOriginOverrideV1], page: Int, limit: Int, total: Int) {
		self.items = items
		self.page = page
		self.limit = limit
		self.total = total
	}

	public var pageCount: Int { max(1, (total + max(limit, 1) - 1) / max(limit, 1)) }
}

/// One row of `session origin-source status` (architecture §7). Fields that
/// do not belong to a row's kind are absent; nil keeps "unknown" distinct
/// from false or zero.
public struct DesktopOriginSourceStatusV1: Codable, Equatable, Sendable {
	public static let claudeMem = "claude-mem"
	public static let codexBackground = "codex-otel-background"

	public let source: String
	public let kind: String
	public let configured: Bool?
	public let enabled: Bool?
	public let available: Bool
	public let supportedVersion: String?
	public let ruleVersion: String
	public let reasonCode: String?
	public let lastRunAt: String?
	public let receiver: String?
	public let backgroundSessions: Int?

	public init(
		source: String, kind: String, configured: Bool? = nil, enabled: Bool? = nil, available: Bool,
		supportedVersion: String? = nil, ruleVersion: String, reasonCode: String?, lastRunAt: String? = nil,
		receiver: String? = nil, backgroundSessions: Int? = nil
	) {
		self.source = source
		self.kind = kind
		self.configured = configured
		self.enabled = enabled
		self.available = available
		self.supportedVersion = supportedVersion
		self.ruleVersion = ruleVersion
		self.reasonCode = reasonCode
		self.lastRunAt = lastRunAt
		self.receiver = receiver
		self.backgroundSessions = backgroundSessions
	}

	enum CodingKeys: String, CodingKey {
		case source, kind, configured, enabled, available, receiver
		case supportedVersion = "supported_version"
		case ruleVersion = "rule_version"
		case reasonCode = "reason_code"
		case lastRunAt = "last_run_at"
		case backgroundSessions = "background_sessions"
	}
}

/// The two fixed rows. A payload without exactly these two sources, in this
/// order, is not a status this App can show.
public struct DesktopOriginSourceStatusesV1: Equatable, Sendable {
	public let claudeMem: DesktopOriginSourceStatusV1
	public let codexBackground: DesktopOriginSourceStatusV1
	/// The envelope's partial flag: one of the three single reads failed.
	public let partial: Bool

	public init(claudeMem: DesktopOriginSourceStatusV1, codexBackground: DesktopOriginSourceStatusV1, partial: Bool) {
		self.claudeMem = claudeMem
		self.codexBackground = codexBackground
		self.partial = partial
	}
}

public enum DesktopOriginReadOutcome<Value: Equatable & Sendable>: Equatable, Sendable {
	case value(Value)
	/// The read did not establish a value. It is never an empty result.
	case unavailable
}

public enum DesktopOriginLookupOutcome: Equatable, Sendable {
	case found(DesktopClassificationQueryV1)
	/// Not visible: absent, excluded or never indexed look the same.
	case notFound
	case unavailable
}

/// Why a write certainly did not commit.
public enum DesktopOriginWriteFailure: Equatable, Sendable {
	case writeFailed
	case identityUnavailable
	case notFound
	case busy
	case rejected
}

/// What one write established. Anything the bridge cannot prove either way
/// is `unknown`: the request may have committed, so the caller reads its
/// receipt back by request ID instead of writing again under a new one.
public enum DesktopOriginWriteOutcome: Equatable, Sendable {
	case committed(DesktopOriginReceiptV1)
	case conflict(currentRevision: Int64?)
	case notSaved(DesktopOriginWriteFailure)
	/// The request ID was already used with a different payload; nothing ran.
	case invalidRequest
	case unknown
}

public enum DesktopOriginReceiptOutcome: Equatable, Sendable {
	case known(DesktopOriginReceiptV1)
	/// No committed result is recorded for this request ID.
	case unknown
	/// The receipt store could not be read; the outcome stays unknown.
	case unavailable
}

public protocol SessionOriginSettingsTransport: Sendable {
	func readDefaultScope() async -> DesktopOriginReadOutcome<DesktopSessionScopeDefaultV1>
	func saveDefaultScope(_ value: DesktopSessionScopeV1, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome
	func lookupSession(client: String, sessionID: String) async -> DesktopOriginLookupOutcome
	func setOverride(_ target: DesktopOriginTargetV1, value: String, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome
	func clearOverride(_ target: DesktopOriginTargetV1, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome
	func readReceipt(requestID: String) async -> DesktopOriginReceiptOutcome
	func listOverrides(page: Int, limit: Int) async -> DesktopOriginReadOutcome<DesktopOriginOverridePageV1>
	func readOriginSourceStatus() async -> DesktopOriginReadOutcome<DesktopOriginSourceStatusesV1>
}

/// One helper run as the bridge classifies it, before any command-specific
/// decoding.
enum DesktopOriginHelperRun: Equatable, Sendable {
	/// Exit 0 with this stdout.
	case succeeded(stdout: Data)
	/// A non-zero exit whose stderr carried a JSON error envelope.
	case failed(code: String, currentRevision: Int64?)
	/// The helper never started: nothing can have been written.
	case notStarted
	/// Started, but nothing can be concluded: a timeout, truncated output,
	/// or a non-zero exit without a readable error.
	case indeterminate
}

/// The arguments of every command the bridge runs. `--id=` and `--client=`
/// keep an ID that begins with `-` or equals a subcommand an argument, never
/// a flag or a subcommand.
public enum SessionOriginCommand {
	public static let overridePageLimit = 20

	public static func readDefaultScope() -> [String] { ["--format", "json", "session", "scope", "get"] }

	public static func saveDefaultScope(_ value: DesktopSessionScopeV1, expectedRevision: Int64, requestID: String) -> [String] {
		["--format", "json", "session", "scope", "set", value.rawValue, "--expected-revision", String(expectedRevision), "--request-id", requestID]
	}

	public static func lookup(client: String, sessionID: String) -> [String] {
		["--format", "json", "session", "classify", "--id=\(sessionID)", "--client=\(client)"]
	}

	public static func setOverride(_ target: DesktopOriginTargetV1, value: String, expectedRevision: Int64, requestID: String) -> [String] {
		lookup(client: target.client, sessionID: target.sessionID) + ["--set", value, "--expected-revision", String(expectedRevision), "--request-id", requestID]
	}

	public static func clearOverride(_ target: DesktopOriginTargetV1, expectedRevision: Int64, requestID: String) -> [String] {
		lookup(client: target.client, sessionID: target.sessionID) + ["--clear", "--expected-revision", String(expectedRevision), "--request-id", requestID]
	}

	public static func readReceipt(requestID: String) -> [String] {
		["--format", "json", "session", "classify", "--request", requestID]
	}

	public static func listOverrides(page: Int, limit: Int) -> [String] {
		["--format", "json", "session", "classify", "list", "--page", String(page), "--limit", String(limit)]
	}

	public static func readOriginSourceStatus() -> [String] {
		["--format", "json", "session", "origin-source", "status"]
	}
}

private struct OriginEnvelope<Data: Decodable>: Decodable {
	let schemaVersion: Int
	let command: String
	let data: Data
	let partial: Bool?

	enum CodingKeys: String, CodingKey {
		case command, data, partial
		case schemaVersion = "schema_version"
	}
}

private struct OriginErrorEnvelope: Decodable {
	struct Body: Decodable {
		struct Details: Decodable {
			let currentRevision: Int64?
			enum CodingKeys: String, CodingKey { case currentRevision = "current_revision" }
		}
		let code: String
		let details: Details?
	}
	let error: Body
}

private struct OriginSourceStatusData: Decodable {
	let sources: [DesktopOriginSourceStatusV1]
}

private struct OriginOutcomeProbe: Decodable {
	let outcome: String
}

enum SessionOriginDecoding {
	static func envelope<Value: Decodable>(_ stdout: Data, command: String, as _: Value.Type = Value.self) -> (data: Value, partial: Bool)? {
		guard let envelope = try? JSONDecoder().decode(OriginEnvelope<Value>.self, from: stdout),
			envelope.schemaVersion == 1, envelope.command == command
		else { return nil }
		return (envelope.data, envelope.partial ?? false)
	}

	static func error(_ stderr: Data) -> (code: String, currentRevision: Int64?)? {
		guard let code = desktopHelperErrorCode(stderr: stderr) else { return nil }
		let details = (try? JSONDecoder().decode(OriginErrorEnvelope.self, from: stderr))?.error.details
		return (code, details?.currentRevision)
	}

	/// A write's committed receipt must name the operation that was asked
	/// for; any other answer cannot be shown as this request's result.
	static func write(
		_ run: DesktopOriginHelperRun, command: String, kind: DesktopOriginReceiptV1.Kind, requestID: String, target: DesktopOriginTargetV1?
	) -> DesktopOriginWriteOutcome {
		switch run {
		case let .succeeded(stdout):
			guard let receipt = envelope(stdout, command: command, as: DesktopOriginReceiptV1.self)?.data,
				receipt.kind == kind, receipt.requestID == requestID, receipt.target == target
			else { return .unknown }
			return .committed(receipt)
		case let .failed(code, currentRevision):
			switch code {
			case "origin_conflict": return .conflict(currentRevision: currentRevision)
			case "invalid_request": return .invalidRequest
			case "origin_write_failed": return .notSaved(.writeFailed)
			case "origin_identity_unavailable": return .notSaved(.identityUnavailable)
			case "session_not_found": return .notSaved(.notFound)
			case "state_busy": return .notSaved(.busy)
			case "invalid_argument", "schema_ahead": return .notSaved(.rejected)
			// origin_write_unknown, runtime_error and any code this App does
			// not know may follow a commit.
			default: return .unknown
			}
		case .notStarted:
			return .notSaved(.rejected)
		case .indeterminate:
			return .unknown
		}
	}

	static func receipt(_ run: DesktopOriginHelperRun, requestID: String) -> DesktopOriginReceiptOutcome {
		guard case let .succeeded(stdout) = run,
			let probe = envelope(stdout, command: "session.classify", as: OriginOutcomeProbe.self)?.data
		else { return .unavailable }
		if probe.outcome == "unknown" { return .unknown }
		guard let receipt = envelope(stdout, command: "session.classify", as: DesktopOriginReceiptV1.self)?.data,
			receipt.requestID == requestID
		else { return .unavailable }
		return .known(receipt)
	}

	static func lookup(_ run: DesktopOriginHelperRun, target: DesktopOriginTargetV1) -> DesktopOriginLookupOutcome {
		switch run {
		case let .succeeded(stdout):
			guard let query = envelope(stdout, command: "session.classify", as: DesktopClassificationQueryV1.self)?.data,
				query.target == target, !query.classificationAvailable || query.origin != nil
			else { return .unavailable }
			return .found(query)
		case let .failed(code, _) where code == "session_not_found":
			return .notFound
		default:
			return .unavailable
		}
	}

	static func defaultScope(_ run: DesktopOriginHelperRun) -> DesktopOriginReadOutcome<DesktopSessionScopeDefaultV1> {
		guard case let .succeeded(stdout) = run,
			let value = envelope(stdout, command: "session.scope.get", as: DesktopSessionScopeDefaultV1.self)?.data,
			value.confirmed, value.revision != nil
		else { return .unavailable }
		return .value(value)
	}

	static func overrides(_ run: DesktopOriginHelperRun, page: Int, limit: Int) -> DesktopOriginReadOutcome<DesktopOriginOverridePageV1> {
		guard case let .succeeded(stdout) = run,
			let value = envelope(stdout, command: "session.classify.list", as: DesktopOriginOverridePageV1.self)?.data,
			value.page == page, value.limit == limit, value.total >= 0, value.items.count <= limit
		else { return .unavailable }
		return .value(value)
	}

	static func sourceStatus(_ run: DesktopOriginHelperRun) -> DesktopOriginReadOutcome<DesktopOriginSourceStatusesV1> {
		guard case let .succeeded(stdout) = run,
			let decoded = envelope(stdout, command: "session.origin_source.status", as: OriginSourceStatusData.self),
			decoded.data.sources.count == 2,
			decoded.data.sources[0].source == DesktopOriginSourceStatusV1.claudeMem, decoded.data.sources[0].kind == "adapter",
			decoded.data.sources[1].source == DesktopOriginSourceStatusV1.codexBackground, decoded.data.sources[1].kind == "builtin"
		else { return .unavailable }
		let sources = decoded.data.sources
		return .value(DesktopOriginSourceStatusesV1(claudeMem: sources[0], codexBackground: sources[1], partial: decoded.partial))
	}
}

extension EmbeddedHelperRunner: SessionOriginSettingsTransport {
	public func readDefaultScope() async -> DesktopOriginReadOutcome<DesktopSessionScopeDefaultV1> {
		SessionOriginDecoding.defaultScope(await runSessionOriginCommand(SessionOriginCommand.readDefaultScope()))
	}

	public func saveDefaultScope(_ value: DesktopSessionScopeV1, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		let run = await runSessionOriginCommand(SessionOriginCommand.saveDefaultScope(value, expectedRevision: expectedRevision, requestID: requestID))
		return SessionOriginDecoding.write(run, command: "session.scope.set", kind: .scopeSet, requestID: requestID, target: nil)
	}

	public func lookupSession(client: String, sessionID: String) async -> DesktopOriginLookupOutcome {
		let run = await runSessionOriginCommand(SessionOriginCommand.lookup(client: client, sessionID: sessionID))
		return SessionOriginDecoding.lookup(run, target: DesktopOriginTargetV1(client: client, sessionID: sessionID))
	}

	public func setOverride(_ target: DesktopOriginTargetV1, value: String, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		let run = await runSessionOriginCommand(SessionOriginCommand.setOverride(target, value: value, expectedRevision: expectedRevision, requestID: requestID))
		return SessionOriginDecoding.write(run, command: "session.classify", kind: .overrideSet, requestID: requestID, target: target)
	}

	public func clearOverride(_ target: DesktopOriginTargetV1, expectedRevision: Int64, requestID: String) async -> DesktopOriginWriteOutcome {
		let run = await runSessionOriginCommand(SessionOriginCommand.clearOverride(target, expectedRevision: expectedRevision, requestID: requestID))
		return SessionOriginDecoding.write(run, command: "session.classify", kind: .overrideClear, requestID: requestID, target: target)
	}

	public func readReceipt(requestID: String) async -> DesktopOriginReceiptOutcome {
		SessionOriginDecoding.receipt(await runSessionOriginCommand(SessionOriginCommand.readReceipt(requestID: requestID)), requestID: requestID)
	}

	public func listOverrides(page: Int, limit: Int) async -> DesktopOriginReadOutcome<DesktopOriginOverridePageV1> {
		SessionOriginDecoding.overrides(await runSessionOriginCommand(SessionOriginCommand.listOverrides(page: page, limit: limit)), page: page, limit: limit)
	}

	public func readOriginSourceStatus() async -> DesktopOriginReadOutcome<DesktopOriginSourceStatusesV1> {
		SessionOriginDecoding.sourceStatus(await runSessionOriginCommand(SessionOriginCommand.readOriginSourceStatus()))
	}
}
