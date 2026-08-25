import Darwin
import Foundation
import WidgetKit

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

public struct AppGroupSnapshotStore: Sendable {
	typealias AtomicReplace = @Sendable (_ temporaryURL: URL, _ destinationURL: URL) throws -> Void
	typealias TimelineReload = @Sendable () -> Void

	public static var appGroupIdentifier: String {
		guard let identifier = Bundle.main.object(
			forInfoDictionaryKey: "AgentDeckAppGroupIdentifier"
		) as? String else {
			return ""
		}
		return identifier
	}
	public static let fileName = "desktop-snapshot-v1.json"

	public let directoryURL: URL
	private let atomicReplace: AtomicReplace
	private let timelineReload: TimelineReload

	public init(directoryURL: URL) {
		self.init(
			directoryURL: directoryURL,
			atomicReplace: Self.replaceAtomically,
			timelineReload: Self.reloadWidgetTimelines
		)
	}

	init(
		directoryURL: URL,
		atomicReplace: @escaping AtomicReplace,
		timelineReload: @escaping TimelineReload = Self.reloadWidgetTimelines
	) {
		self.directoryURL = directoryURL
		self.atomicReplace = atomicReplace
		self.timelineReload = timelineReload
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
		let fileManager = FileManager.default
		try Self.ensurePrivateDirectory(directoryURL, fileManager: fileManager)

		let encoder = JSONEncoder()
		encoder.outputFormatting = [.sortedKeys]
		let data = try encoder.encode(snapshot)
		let temporaryURL = directoryURL.appendingPathComponent(
			".\(Self.fileName).\(UUID().uuidString).tmp",
			isDirectory: false
		)
		var removeTemporaryFile = true
		defer {
			if removeTemporaryFile {
				try? fileManager.removeItem(at: temporaryURL)
			}
		}

		try Self.writePrivateFile(data, to: temporaryURL)
		try Self.verifyPrivateRegularFile(temporaryURL, fileManager: fileManager)
		try atomicReplace(temporaryURL, snapshotURL)
		removeTemporaryFile = false
		try Self.verifyPrivateRegularFile(snapshotURL, fileManager: fileManager)
		timelineReload()
	}

	public func read() throws -> AppGroupDesktopSnapshotV1 {
		let snapshot = try JSONDecoder().decode(
			AppGroupDesktopSnapshotV1.self,
			from: Data(contentsOf: snapshotURL)
		)
		guard snapshot.schemaVersion == AppGroupDesktopSnapshotV1.schemaVersion else {
			throw AppGroupSnapshotStoreError.unsupportedSchemaVersion(snapshot.schemaVersion)
		}
		return snapshot
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

	private static func reloadWidgetTimelines() {
		WidgetCenter.shared.reloadAllTimelines()
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
	case unsupportedSchemaVersion(Int)
}
