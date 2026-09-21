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

	public init(envelope: DesktopWireEnvelopeV1) {
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
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
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
