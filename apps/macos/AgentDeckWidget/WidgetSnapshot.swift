import Foundation

struct WidgetDesktopSnapshotV1: Codable, Equatable, Sendable {
	static let schemaVersion = 1

	let schemaVersion: Int
	let generatedAt: String
	let nextRefreshAt: String
	let partial: Bool
	let usage: WidgetUsageSnapshotV1
	let subscription: DesktopSubscriptionSnapshotV1

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case generatedAt = "generated_at"
		case nextRefreshAt = "next_refresh_at"
		case partial
		case usage, subscription
	}

	init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		schemaVersion = try container.decode(Int.self, forKey: .schemaVersion)
		generatedAt = try container.decode(String.self, forKey: .generatedAt)
		nextRefreshAt = try container.decode(String.self, forKey: .nextRefreshAt)
		partial = try container.decode(Bool.self, forKey: .partial)
		usage = try container.decode(WidgetUsageSnapshotV1.self, forKey: .usage)
		subscription = try container.decodeIfPresent(DesktopSubscriptionSnapshotV1.self, forKey: .subscription) ?? .unavailable
	}
}

struct WidgetUsageSnapshotV1: Codable, Equatable, Sendable {
	let available: Bool
	let presentation: DesktopUsagePresentationV1

	enum CodingKeys: String, CodingKey {
		case available
		case presentation
	}

	init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		available = try container.decode(Bool.self, forKey: .available)
		presentation = try container.decodeIfPresent(DesktopUsagePresentationV1.self, forKey: .presentation) ?? .unavailable
	}
}

struct WidgetSnapshotReader: Sendable {
	static var appGroupIdentifier: String {
		guard let identifier = Bundle.main.object(
			forInfoDictionaryKey: "AgentDeckAppGroupIdentifier"
		) as? String else {
			return ""
		}
		return identifier
	}
	static let fileName = "desktop-snapshot-v1.json"

	let directoryURL: URL

	init(directoryURL: URL) {
		self.directoryURL = directoryURL
	}

	init?(appGroupIdentifier: String = Self.appGroupIdentifier) {
		guard !appGroupIdentifier.isEmpty,
			let directoryURL = FileManager.default.containerURL(
			forSecurityApplicationGroupIdentifier: appGroupIdentifier
		) else {
			return nil
		}
		self.init(directoryURL: directoryURL)
	}

	func read() throws -> WidgetDesktopSnapshotV1 {
		let url = directoryURL.appendingPathComponent(Self.fileName, isDirectory: false)
		let snapshot = try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: Data(contentsOf: url))
		guard snapshot.schemaVersion == WidgetDesktopSnapshotV1.schemaVersion else {
			throw WidgetSnapshotError.unsupportedSchemaVersion(snapshot.schemaVersion)
		}
		return snapshot
	}
}

enum WidgetSnapshotError: Error, Equatable, Sendable {
	case unsupportedSchemaVersion(Int)
}
