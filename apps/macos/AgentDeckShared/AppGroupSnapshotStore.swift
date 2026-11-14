import Darwin
import Foundation

public struct AppGroupDesktopSnapshotV1: Codable, Equatable, Sendable {
	public static let schemaVersion = 1

	public let schemaVersion: Int
	public let generatedAt: String
	public let nextRefreshAt: String
	public let partial: Bool
	public let issueCodes: [String]
	public let provider: AppGroupProviderSnapshotV1
	public let usage: AppGroupUsageSnapshotV1
	public let sessions: AppGroupSessionsSnapshotV1
	public let health: AppGroupHealthSnapshotV1
	public let subscription: DesktopSubscriptionSnapshotV1
	/// The Widget's session-scope summaries (architecture §7). Nil only for a
	/// file written before Task 7; a current host always writes a set, even an
	/// unavailable one, so a reader can tell legacy from unavailable.
	public let sessionScopeSummary: AppGroupSessionScopeSummarySetV1?

	public init(envelope: DesktopWireEnvelopeV1, sessionScopeSummary: AppGroupSessionScopeSummarySetV1? = nil) {
		self.sessionScopeSummary = sessionScopeSummary
		schemaVersion = Self.schemaVersion
		generatedAt = envelope.data.generatedAt
		nextRefreshAt = envelope.data.nextRefreshAt
		partial = envelope.partial
		issueCodes = AppGroupPresentationCode.filter(envelope.warnings)
		provider = AppGroupProviderSnapshotV1(envelope.data.provider)
		usage = AppGroupUsageSnapshotV1(envelope.data.usage)
		sessions = AppGroupSessionsSnapshotV1(envelope.data.sessions)
		health = AppGroupHealthSnapshotV1(envelope.data.health)
		subscription = envelope.data.subscription
	}

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case generatedAt = "generated_at"
		case nextRefreshAt = "next_refresh_at"
		case partial
		case issueCodes = "issue_codes"
		case provider
		case usage
		case sessions
		case health
		case subscription
		case sessionScopeSummary = "session_scope_summary"
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		sessionScopeSummary = try? container.decodeIfPresent(AppGroupSessionScopeSummarySetV1.self, forKey: .sessionScopeSummary)
		schemaVersion = try container.decode(Int.self, forKey: .schemaVersion)
		generatedAt = try container.decode(String.self, forKey: .generatedAt)
		nextRefreshAt = try container.decode(String.self, forKey: .nextRefreshAt)
		partial = try container.decode(Bool.self, forKey: .partial)
		issueCodes = try container.decode([String].self, forKey: .issueCodes)
		provider = try container.decode(AppGroupProviderSnapshotV1.self, forKey: .provider)
		usage = try container.decode(AppGroupUsageSnapshotV1.self, forKey: .usage)
		sessions = try container.decode(AppGroupSessionsSnapshotV1.self, forKey: .sessions)
		health = try container.decode(AppGroupHealthSnapshotV1.self, forKey: .health)
		subscription = try container.decodeIfPresent(DesktopSubscriptionSnapshotV1.self, forKey: .subscription) ?? .unavailable
	}
}

/// One client × period session count for the Widget. It carries only the
/// aggregate and the safe identity/time of the batch it was accepted in:
/// never a session ID, project, model, relationship, path, override list or
/// request receipt. `unknownCount` is nil when coverage is incomplete, and is
/// always encoded (as null), because a missing key is a malformed summary.
public struct AppGroupSessionScopeSummaryV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let scope: DesktopSessionScopeV1
	public let count: Int
	public let unknownCount: Int?
	public let client: String
	public let period: String
	public let publicationID: String
	public let generatedAt: String

	public init(
		scope: DesktopSessionScopeV1, count: Int, unknownCount: Int?, client: String, period: String,
		publicationID: String, generatedAt: String
	) {
		available = true
		self.scope = scope
		self.count = count
		self.unknownCount = unknownCount
		self.client = client
		self.period = period
		self.publicationID = publicationID
		self.generatedAt = generatedAt
	}

	enum CodingKeys: String, CodingKey {
		case available, scope, count, client, period
		case unknownCount = "unknown_count"
		case publicationID = "publication_id"
		case generatedAt = "generated_at"
	}

	/// Reads a summary item with the Widget's own rules, so the host never
	/// keeps (and re-publishes) an item the Widget would reject. In particular
	/// a missing `unknown_count` is malformed while an explicit null is unknown
	/// coverage: without this, Codable would read both as nil and a later
	/// retention would re-encode the malformed item as a valid null.
	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		guard try container.decode(Bool.self, forKey: .available) else {
			throw Self.invalid(.available, container, "an accepted summary item is available")
		}
		available = true
		scope = try container.decode(DesktopSessionScopeV1.self, forKey: .scope)
		count = try container.decode(Int.self, forKey: .count)
		guard count >= 0 else { throw Self.invalid(.count, container, "count is non-negative") }
		guard container.contains(.unknownCount) else {
			throw DecodingError.keyNotFound(CodingKeys.unknownCount, DecodingError.Context(
				codingPath: container.codingPath,
				debugDescription: "unknown_count must be present; only an explicit null means unknown coverage"
			))
		}
		if try container.decodeNil(forKey: .unknownCount) {
			unknownCount = nil
		} else {
			let value = try container.decode(Int.self, forKey: .unknownCount)
			guard value >= 0, value <= count else { throw Self.invalid(.unknownCount, container, "unknown_count is within 0...count") }
			unknownCount = value
		}
		// Auxiliary holds only confirmed auxiliary sessions.
		guard scope != .auxiliary || unknownCount == 0 else {
			throw Self.invalid(.unknownCount, container, "auxiliary unknown_count is 0")
		}
		client = try container.decode(String.self, forKey: .client)
		period = try container.decode(String.self, forKey: .period)
		publicationID = try container.decode(String.self, forKey: .publicationID)
		guard !publicationID.isEmpty else { throw Self.invalid(.publicationID, container, "publication_id is set") }
		generatedAt = try container.decode(String.self, forKey: .generatedAt)
		guard WidgetSessionScopeBatchCollector.date(generatedAt) != nil else {
			throw Self.invalid(.generatedAt, container, "generated_at is an ISO 8601 time")
		}
	}

	private static func invalid(
		_ key: CodingKeys, _ container: KeyedDecodingContainer<CodingKeys>, _ rule: String
	) -> DecodingError {
		.dataCorruptedError(forKey: key, in: container, debugDescription: rule)
	}

	public func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encode(available, forKey: .available)
		try container.encode(scope, forKey: .scope)
		try container.encode(count, forKey: .count)
		try container.encode(unknownCount, forKey: .unknownCount)
		try container.encode(client, forKey: .client)
		try container.encode(period, forKey: .period)
		try container.encode(publicationID, forKey: .publicationID)
		try container.encode(generatedAt, forKey: .generatedAt)
	}
}

/// The whole accepted batch: every supported client × period for one
/// confirmed default, one committed generation and one time. It is replaced
/// only as a whole. `stale` marks a previous accepted batch kept after a later
/// batch failed; `reason` is a safe code from `AppGroupSessionScopeSummaryReason`.
///
/// `unreadable` lists the selections of items the host read but could not
/// trust (only ever from a file it re-reads, never from a new batch). Each is
/// written back into `items` as `{"available": false, "client", "period"}`:
/// no figure, no identity or time, and no `unknown_count`, so a missing key is
/// never repaired into an explicit null. Keeping the selection matters: the
/// Widget shows a selection only when exactly one well-formed item answers it,
/// so a malformed duplicate must keep that selection ambiguous (unavailable)
/// on every re-publication rather than vanish and leave its twin trusted.
public struct AppGroupSessionScopeSummarySetV1: Codable, Equatable, Sendable {
	public let stale: Bool
	public let reason: String?
	public let items: [AppGroupSessionScopeSummaryV1]
	public let unreadable: [DesktopSessionScopeSummarySelectionV1]

	public init(
		stale: Bool, reason: String?, items: [AppGroupSessionScopeSummaryV1],
		unreadable: [DesktopSessionScopeSummarySelectionV1] = []
	) {
		self.stale = stale
		self.reason = reason.map(AppGroupSessionScopeSummaryReason.safe)
		self.items = items
		self.unreadable = unreadable
	}

	public static func unavailable(_ reason: String) -> Self {
		Self(stale: false, reason: reason, items: [])
	}

	/// The same items, kept visible after a later batch failed; selections
	/// the host could not trust stay untrusted.
	public func retained(reason: String) -> Self {
		Self(stale: true, reason: reason, items: items, unreadable: unreadable)
	}

	/// Reads a set with the Widget's rules. A reason outside the closed enum
	/// is a set-level fault and rejects the whole set. A malformed item is an
	/// item-level fault, exactly as the Widget treats it: its figures are
	/// discarded, never repaired, while every well-formed item keeps its own
	/// identity and time. When the malformed item still names a supported
	/// client and period, that selection is kept in `unreadable`, so it stays
	/// unavailable however often the set is re-published — including when it
	/// duplicates a well-formed item. A malformed item without a readable,
	/// supported selection matches no Widget entry and is dropped, which
	/// changes nothing the Widget shows. A non-empty list with no well-formed
	/// item at all carries nothing to keep and rejects the set.
	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		stale = try container.decode(Bool.self, forKey: .stale)
		let rawReason = try container.decodeIfPresent(String.self, forKey: .reason)
		if let rawReason, DesktopSessionScopeSummaryReasonV1(rawValue: rawReason) == nil {
			throw DecodingError.dataCorruptedError(
				forKey: .reason, in: container, debugDescription: "reason is outside the closed summary reason enum"
			)
		}
		reason = rawReason
		let read = try container.decode([ItemRead].self, forKey: .items)
		items = read.compactMap(\.item)
		unreadable = read.compactMap(\.unreadable)
		guard read.isEmpty || !items.isEmpty else {
			throw DecodingError.dataCorruptedError(
				forKey: .items, in: container, debugDescription: "no summary item is well-formed"
			)
		}
	}

	/// One element of `items`, decoded without failing the array.
	private struct ItemRead: Decodable {
		let item: AppGroupSessionScopeSummaryV1?
		let unreadable: DesktopSessionScopeSummarySelectionV1?

		init(from decoder: Decoder) throws {
			item = try? AppGroupSessionScopeSummaryV1(from: decoder)
			unreadable = item == nil ? try? DesktopSessionScopeSummarySelectionV1(from: decoder) : nil
		}
	}

	public func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encode(stale, forKey: .stale)
		try container.encode(reason, forKey: .reason)
		var list = container.nestedUnkeyedContainer(forKey: .items)
		for item in items { try list.encode(item) }
		for selection in unreadable { try list.encode(UnreadableItem(selection)) }
	}

	/// The figure-free form an untrusted selection is written in.
	private struct UnreadableItem: Encodable {
		let selection: DesktopSessionScopeSummarySelectionV1
		init(_ selection: DesktopSessionScopeSummarySelectionV1) { self.selection = selection }

		enum CodingKeys: String, CodingKey { case available, client, period }

		func encode(to encoder: Encoder) throws {
			var container = encoder.container(keyedBy: CodingKeys.self)
			try container.encode(false, forKey: .available)
			try container.encode(selection.client, forKey: .client)
			try container.encode(selection.period, forKey: .period)
		}
	}

	enum CodingKeys: String, CodingKey { case stale, reason, items }
}

/// A supported client × period, read from an item whose other fields may be
/// unreadable. Anything outside the supported values is not a selection.
public struct DesktopSessionScopeSummarySelectionV1: Decodable, Equatable, Hashable, Sendable {
	public let client: String
	public let period: String

	public init?(client: String, period: String) {
		guard DesktopSessionScopeSelectionV1.clients.contains(client), DesktopSessionScopeSelectionV1.periods.contains(period) else {
			return nil
		}
		self.client = client
		self.period = period
	}

	enum CodingKeys: String, CodingKey { case client, period }

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		let client = try container.decode(String.self, forKey: .client)
		let period = try container.decode(String.self, forKey: .period)
		guard let selection = Self(client: client, period: period) else {
			throw DecodingError.dataCorruptedError(forKey: .client, in: container, debugDescription: "not a supported selection")
		}
		self = selection
	}
}

/// The closed set of reasons a summary set may carry. Any other code (a newer
/// producer's, or a source-specific one) is written as the generic
/// `selection_unavailable`, so no identifying input reaches the Widget.
public enum AppGroupSessionScopeSummaryReason {
	public static let defaultUnconfirmed = DesktopSessionScopeSummaryReasonV1.defaultUnconfirmed.rawValue
	public static let defaultChanged = DesktopSessionScopeSummaryReasonV1.defaultChanged.rawValue
	public static let generationUnavailable = DesktopSessionScopeSummaryReasonV1.generationUnavailable.rawValue
	public static let batchInconsistent = DesktopSessionScopeSummaryReasonV1.batchInconsistent.rawValue
	public static let selectionUnavailable = DesktopSessionScopeSummaryReasonV1.selectionUnavailable.rawValue
	public static let summaryUnavailable = DesktopSessionScopeSummaryReasonV1.summaryUnavailable.rawValue

	static let allowed = Set(DesktopSessionScopeSummaryReasonV1.allCases.map(\.rawValue))

	public static func safe(_ value: String) -> String {
		allowed.contains(value) ? value : selectionUnavailable
	}
}

public struct AppGroupProviderSnapshotV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let routes: [AppGroupProviderRouteV1]

	init(_ snapshot: DesktopProviderSnapshotV1) {
		available = snapshot.available
		routes = snapshot.routes.map(AppGroupProviderRouteV1.init)
	}
}

public struct AppGroupProviderRouteV1: Codable, Equatable, Sendable {
	public let client: String
	public let provider: String
	public let selectedAt: String?
	public let viaWrapper: Bool

	init(_ route: DesktopProviderRouteV1) {
		client = route.client
		provider = route.provider
		selectedAt = route.selectedAt
		viaWrapper = route.viaWrapper
	}

	enum CodingKeys: String, CodingKey {
		case client
		case provider
		case selectedAt = "selected_at"
		case viaWrapper = "via_wrapper"
	}
}

public struct AppGroupUsageSnapshotV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let from: String
	public let to: String
	public let tokens: [String: Int64]
	public let counts: [String: Int64]
	public let catalogBaseCost: String?
	public let providerCost: String?
	public let pricingComplete: Bool
	public let unpricedComponents: Int
	public let issueCodes: [String]
	public let presentation: DesktopUsagePresentationV1

	init(_ snapshot: DesktopUsageSnapshotV1) {
		available = snapshot.available
		from = snapshot.from
		to = snapshot.to
		tokens = snapshot.tokens
		counts = snapshot.counts
		catalogBaseCost = snapshot.catalogBaseCost
		providerCost = snapshot.providerCost
		pricingComplete = snapshot.pricingComplete
		unpricedComponents = snapshot.unpricedComponents
		issueCodes = AppGroupPresentationCode.filter(snapshot.warnings)
		presentation = snapshot.presentation
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		available = try container.decode(Bool.self, forKey: .available)
		from = try container.decode(String.self, forKey: .from)
		to = try container.decode(String.self, forKey: .to)
		tokens = try container.decode([String: Int64].self, forKey: .tokens)
		counts = try container.decode([String: Int64].self, forKey: .counts)
		catalogBaseCost = try container.decodeIfPresent(String.self, forKey: .catalogBaseCost)
		providerCost = try container.decodeIfPresent(String.self, forKey: .providerCost)
		pricingComplete = try container.decode(Bool.self, forKey: .pricingComplete)
		unpricedComponents = try container.decode(Int.self, forKey: .unpricedComponents)
		issueCodes = try container.decode([String].self, forKey: .issueCodes)
		presentation = try container.decodeIfPresent(DesktopUsagePresentationV1.self, forKey: .presentation) ?? .unavailable
	}

	enum CodingKeys: String, CodingKey {
		case available
		case from
		case to
		case tokens
		case counts
		case catalogBaseCost = "catalog_base_cost"
		case providerCost = "provider_cost"
		case pricingComplete = "pricing_complete"
		case unpricedComponents = "unpriced_components"
		case issueCodes = "issue_codes"
		case presentation
	}
}

// Widgets receive count-only session information. Session IDs, project names,
// models, and timestamps remain in the host process and never enter App Group
// storage.
public struct AppGroupSessionsSnapshotV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let total: Int

	init(_ snapshot: DesktopSessionsSnapshotV1) {
		available = snapshot.available
		total = snapshot.total
	}
}

public struct AppGroupHealthSnapshotV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let healthy: Bool
	public let problems: Int
	public let warnings: Int
	public let errors: Int
	public let issueCodes: [String]

	init(_ snapshot: DesktopHealthSnapshotV1) {
		available = snapshot.available
		healthy = snapshot.healthy
		problems = snapshot.problems
		warnings = snapshot.warnings
		errors = snapshot.errors
		issueCodes = AppGroupPresentationCode.filter(snapshot.checks.compactMap(\.code))
	}

	enum CodingKeys: String, CodingKey {
		case available
		case healthy
		case problems
		case warnings
		case errors
		case issueCodes = "issue_codes"
	}
}

public enum AppGroupSnapshotBaselineUnknownReason: Equatable, Sendable {
	case missing
	case unsafeFile
	case oversized
	case unreadable
	case unsupportedVersion
}

public enum AppGroupSnapshotExisting: Equatable, Sendable {
	case known(AppGroupDesktopSnapshotV1)
	case unknown(AppGroupSnapshotBaselineUnknownReason)
}

public struct AppGroupSnapshotPreparedWrite: Sendable {
	fileprivate let temporaryURL: URL
}

public enum AppGroupSnapshotCommitError: Error, Equatable, Sendable {
	case failedBeforeCommit
	case indeterminateAfterCommit
}

public struct AppGroupSnapshotStore: Sendable {
	typealias AtomicReplace = @Sendable (_ temporaryURL: URL, _ destinationURL: URL) throws -> Void
	typealias DurabilitySync = @Sendable (_ directoryURL: URL, _ snapshotURL: URL) throws -> Void

	public static var appGroupIdentifier: String {
		guard let identifier = Bundle.main.object(
			forInfoDictionaryKey: "AgentDeckAppGroupIdentifier"
		) as? String else {
			return ""
		}
		return identifier
	}
	public static let fileName = "desktop-snapshot-v1.json"
	static let abandonedTemporaryFilePrefix = ".\(fileName)."
	static let abandonedTemporaryFileSuffix = ".tmp"
	static let abandonedTemporaryFileCleanupLimit = 16
	static let abandonedTemporaryFileMinimumAge: TimeInterval = 60 * 60

	public let directoryURL: URL
	private let atomicReplace: AtomicReplace
	private let durabilitySync: DurabilitySync

	public init(directoryURL: URL) {
		self.init(
			directoryURL: directoryURL,
			atomicReplace: Self.replaceAtomically,
			durabilitySync: Self.synchronizeDurably
		)
	}

	init(
		directoryURL: URL,
		atomicReplace: @escaping AtomicReplace,
		durabilitySync: @escaping DurabilitySync = Self.synchronizeDurably
	) {
		self.directoryURL = directoryURL
		self.atomicReplace = atomicReplace
		self.durabilitySync = durabilitySync
	}

	public init?(appGroupIdentifier: String = Self.appGroupIdentifier) {
		guard !appGroupIdentifier.isEmpty,
			let directoryURL = FileManager.default.containerURL(
			forSecurityApplicationGroupIdentifier: appGroupIdentifier
		) else {
			return nil
		}
		self.init(directoryURL: directoryURL)
	}

	public var snapshotURL: URL {
		directoryURL.appendingPathComponent(Self.fileName, isDirectory: false)
	}

	public func write(_ snapshot: AppGroupDesktopSnapshotV1) throws {
		let prepared = try prepareWrite(snapshot)
		do {
			try commit(prepared)
		} catch {
			discard(prepared)
			throw error
		}
	}

	public func readExisting() -> AppGroupSnapshotExisting {
		do {
			let snapshot = try JSONDecoder().decode(
				AppGroupDesktopSnapshotV1.self,
				from: AppGroupSnapshotBytes.readBounded(at: snapshotURL)
			)
			guard snapshot.schemaVersion == AppGroupDesktopSnapshotV1.schemaVersion else {
				return .unknown(.unsupportedVersion)
			}
			return .known(snapshot)
		} catch let error as AppGroupSnapshotByteReadError {
			switch error {
			case .missing: return .unknown(.missing)
			case .unsafeFile: return .unknown(.unsafeFile)
			case .oversized: return .unknown(.oversized)
			case .unreadable: return .unknown(.unreadable)
			}
		} catch {
			return .unknown(.unreadable)
		}
	}

	public func read() throws -> AppGroupDesktopSnapshotV1 {
		switch readExisting() {
		case let .known(snapshot):
			return snapshot
		case .unknown(.unsupportedVersion):
			let data = try AppGroupSnapshotBytes.readBounded(at: snapshotURL)
			let snapshot = try JSONDecoder().decode(AppGroupDesktopSnapshotV1.self, from: data)
			throw AppGroupSnapshotStoreError.unsupportedSchemaVersion(snapshot.schemaVersion)
		case .unknown(.missing), .unknown(.unreadable):
			throw AppGroupSnapshotStoreError.unreadableSnapshot
		case .unknown(.unsafeFile):
			throw AppGroupSnapshotStoreError.insecureFile
		case .unknown(.oversized):
			throw AppGroupSnapshotStoreError.oversizedSnapshot
		}
	}

	public func prepareWrite(_ snapshot: AppGroupDesktopSnapshotV1) throws -> AppGroupSnapshotPreparedWrite {
		let fileManager = FileManager.default
		try Self.ensurePrivateDirectory(directoryURL, fileManager: fileManager)
		Self.cleanupAbandonedTemporaryFiles(in: directoryURL, fileManager: fileManager)

		let encoder = JSONEncoder()
		encoder.outputFormatting = [.sortedKeys]
		let data = try encoder.encode(snapshot)
		guard data.count <= AppGroupSnapshotBytes.maximumBytes else {
			throw AppGroupSnapshotStoreError.oversizedSnapshot
		}
		let temporaryURL = directoryURL.appendingPathComponent(
			"\(Self.abandonedTemporaryFilePrefix)\(UUID().uuidString)\(Self.abandonedTemporaryFileSuffix)",
			isDirectory: false
		)
		do {
			try Self.writePrivateFile(data, to: temporaryURL)
			try Self.verifyPrivateRegularFile(temporaryURL, fileManager: fileManager)
			return AppGroupSnapshotPreparedWrite(temporaryURL: temporaryURL)
		} catch {
			try? fileManager.removeItem(at: temporaryURL)
			throw error
		}
	}

	public func commit(_ prepared: AppGroupSnapshotPreparedWrite) throws {
		do {
			try atomicReplace(prepared.temporaryURL, snapshotURL)
		} catch {
			throw AppGroupSnapshotCommitError.failedBeforeCommit
		}
		do {
			try Self.verifyPrivateRegularFile(snapshotURL, fileManager: .default)
			try durabilitySync(directoryURL, snapshotURL)
		} catch {
			throw AppGroupSnapshotCommitError.indeterminateAfterCommit
		}
	}

	public func discard(_ prepared: AppGroupSnapshotPreparedWrite) {
		try? FileManager.default.removeItem(at: prepared.temporaryURL)
	}

	private static func cleanupAbandonedTemporaryFiles(in directoryURL: URL, fileManager: FileManager) {
		let resourceKeys: [URLResourceKey] = [
			.isRegularFileKey,
			.isSymbolicLinkKey,
			.contentModificationDateKey,
		]
		guard let entries = try? fileManager.contentsOfDirectory(
			at: directoryURL,
			includingPropertiesForKeys: resourceKeys,
			options: []
		) else {
			return
		}

		let cutoff = Date().addingTimeInterval(-abandonedTemporaryFileMinimumAge)
		let stale = entries.compactMap { url -> (url: URL, modifiedAt: Date)? in
			let name = url.lastPathComponent
			guard name.hasPrefix(abandonedTemporaryFilePrefix),
				name.hasSuffix(abandonedTemporaryFileSuffix)
			else {
				return nil
			}
			let identifier = String(
				name.dropFirst(abandonedTemporaryFilePrefix.count)
					.dropLast(abandonedTemporaryFileSuffix.count)
			)
			guard UUID(uuidString: identifier) != nil,
				let values = try? url.resourceValues(forKeys: Set(resourceKeys)),
				values.isRegularFile == true,
				values.isSymbolicLink != true,
				let modifiedAt = values.contentModificationDate,
				modifiedAt <= cutoff
			else {
				return nil
			}
			return (url, modifiedAt)
		}
		.sorted { $0.modifiedAt < $1.modifiedAt }

		for candidate in stale.prefix(abandonedTemporaryFileCleanupLimit) {
			try? fileManager.removeItem(at: candidate.url)
		}
	}

	private static func ensurePrivateDirectory(_ directoryURL: URL, fileManager: FileManager) throws {
		if fileManager.fileExists(atPath: directoryURL.path) {
			let values = try directoryURL.resourceValues(forKeys: [.isDirectoryKey, .isSymbolicLinkKey])
			guard values.isDirectory == true, values.isSymbolicLink != true else {
				throw AppGroupSnapshotStoreError.insecureDirectory
			}
		}

		try fileManager.createDirectory(
			at: directoryURL,
			withIntermediateDirectories: true,
			attributes: [.posixPermissions: 0o700]
		)
		try fileManager.setAttributes([.posixPermissions: 0o700], ofItemAtPath: directoryURL.path)

		let values = try directoryURL.resourceValues(forKeys: [.isDirectoryKey, .isSymbolicLinkKey])
		guard values.isDirectory == true,
			values.isSymbolicLink != true,
			try fileMode(at: directoryURL, fileManager: fileManager) == 0o700
		else {
			throw AppGroupSnapshotStoreError.insecureDirectory
		}
	}

	private static func writePrivateFile(_ data: Data, to url: URL) throws {
		let descriptor = open(url.path, O_WRONLY | O_CREAT | O_EXCL | O_NOFOLLOW, mode_t(S_IRUSR | S_IWUSR))
		guard descriptor >= 0 else {
			throw currentPOSIXError()
		}

		let handle = FileHandle(fileDescriptor: descriptor, closeOnDealloc: true)
		do {
			try handle.write(contentsOf: data)
			try handle.synchronize()
			try handle.close()
		} catch {
			try? handle.close()
			throw error
		}
	}

	private static func replaceAtomically(_ temporaryURL: URL, _ destinationURL: URL) throws {
		guard rename(temporaryURL.path, destinationURL.path) == 0 else {
			throw currentPOSIXError()
		}
	}

	private static func verifyPrivateRegularFile(_ url: URL, fileManager: FileManager) throws {
		let values = try url.resourceValues(forKeys: [.isRegularFileKey, .isSymbolicLinkKey])
		guard values.isRegularFile == true,
			values.isSymbolicLink != true,
			try fileMode(at: url, fileManager: fileManager) == 0o600
		else {
			throw AppGroupSnapshotStoreError.insecureFile
		}
	}

	private static func fileMode(at url: URL, fileManager: FileManager) throws -> Int {
		let attributes = try fileManager.attributesOfItem(atPath: url.path)
		guard let permissions = attributes[.posixPermissions] as? NSNumber else {
			throw AppGroupSnapshotStoreError.insecureFile
		}
		return permissions.intValue & 0o777
	}

	private static func currentPOSIXError() -> POSIXError {
		POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
	}

	private static func synchronizeDurably(directoryURL: URL, snapshotURL: URL) throws {
		try synchronize(path: snapshotURL.path, flags: O_RDONLY | O_NOFOLLOW)
		try synchronize(path: directoryURL.path, flags: O_RDONLY | O_DIRECTORY | O_NOFOLLOW)
	}

	private static func synchronize(path: String, flags: Int32) throws {
		let descriptor = open(path, flags)
		guard descriptor >= 0 else { throw currentPOSIXError() }
		defer { close(descriptor) }
		guard fsync(descriptor) == 0 else { throw currentPOSIXError() }
	}
}

private enum AppGroupPresentationCode {
	private static let allowed = Set([
		"database_missing",
		"health_unavailable",
		"pricing_incomplete",
		"provider_unavailable",
		"sessions_unavailable",
		"usage_unavailable",
	])

	static func filter(_ values: [String]) -> [String] {
		Array(Set(values.filter(allowed.contains))).sorted()
	}
}

public enum AppGroupSnapshotStoreError: Error, Equatable, Sendable {
	case insecureDirectory
	case insecureFile
	case oversizedSnapshot
	case unreadableSnapshot
	case unsupportedSchemaVersion(Int)
}
