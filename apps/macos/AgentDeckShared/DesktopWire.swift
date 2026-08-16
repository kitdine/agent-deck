import Foundation

public enum DesktopWireError: Error, Equatable, Sendable, LocalizedError {
	case invalidEnvelope
	case invalidTimestamp
	case unsupportedWireVersion(Int)

    public var errorDescription: String? {
        switch self {
		case .invalidEnvelope:
			return "Invalid desktop wire envelope."
		case .invalidTimestamp:
			return "Invalid desktop wire timestamp."
		case let .unsupportedWireVersion(version):
            return "Unsupported desktop wire version \(version)."
        }
    }
}

public struct DesktopWireOutputErrorV1: Codable, Equatable, Sendable {
    public let code: String
    public let message: String
}

public struct DesktopWireEnvelopeV1: Codable, Equatable, Sendable {
    public static let schemaVersion = 1
    public static let command = "desktop.snapshot"

    public let schemaVersion: Int
    public let command: String
    public let generatedAt: String
    public let data: DesktopSnapshotV1
    public let warnings: [String]
    public let partial: Bool
    public let error: DesktopWireOutputErrorV1?

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case command
        case generatedAt = "generated_at"
        case data
        case warnings
        case partial
        case error
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let schemaVersion = try container.decode(Int.self, forKey: .schemaVersion)
        let command = try container.decode(String.self, forKey: .command)
        let generatedAt = try container.decode(String.self, forKey: .generatedAt)
        let data = try container.decode(DesktopSnapshotV1.self, forKey: .data)
        let warnings = try container.decode([String].self, forKey: .warnings)
        let partial = try container.decode(Bool.self, forKey: .partial)
        let error = try container.decodeIfPresent(DesktopWireOutputErrorV1.self, forKey: .error)

        guard schemaVersion == Self.schemaVersion,
              command == Self.command,
              error == nil
        else {
            throw DesktopWireError.invalidEnvelope
        }
		guard data.wireVersion == DesktopSnapshotV1.wireVersion else {
			throw DesktopWireError.unsupportedWireVersion(data.wireVersion)
		}
		guard isRFC3339Timestamp(generatedAt),
			isRFC3339Timestamp(data.generatedAt),
			isRFC3339Timestamp(data.nextRefreshAt)
		else {
			throw DesktopWireError.invalidTimestamp
		}

        self.schemaVersion = schemaVersion
        self.command = command
        self.generatedAt = generatedAt
        self.data = data
        self.warnings = warnings
        self.partial = partial
        self.error = error
    }
}

public struct DesktopSnapshotV1: Codable, Equatable, Sendable {
    public static let wireVersion = 1

    public let wireVersion: Int
    public let generatedAt: String
    public let nextRefreshAt: String
    public let provider: DesktopProviderSnapshotV1
    public let usage: DesktopUsageSnapshotV1
    public let sessions: DesktopSessionsSnapshotV1
    public let health: DesktopHealthSnapshotV1

    enum CodingKeys: String, CodingKey {
        case wireVersion = "wire_version"
        case generatedAt = "generated_at"
        case nextRefreshAt = "next_refresh_at"
        case provider
        case usage
        case sessions
        case health
    }
}

public struct DesktopProviderSnapshotV1: Codable, Equatable, Sendable {
    public let available: Bool
    public let routes: [DesktopProviderRouteV1]
}

public struct DesktopProviderRouteV1: Codable, Equatable, Sendable {
    public let client: String
    public let provider: String
    public let selectedAt: String?
    public let viaWrapper: Bool

    enum CodingKeys: String, CodingKey {
        case client
        case provider
        case selectedAt = "selected_at"
        case viaWrapper = "via_wrapper"
    }
}

public struct DesktopUsageSnapshotV1: Codable, Equatable, Sendable {
    public let available: Bool
    public let from: String
    public let to: String
    public let tokens: [String: Int64]
    public let counts: [String: Int64]
    public let catalogBaseCost: String?
    public let providerCost: String?
    public let knownCatalogBaseCost: String?
    public let knownProviderCost: String?
    public let pricingComplete: Bool
    public let unpricedComponents: Int
    public let warnings: [String]

    enum CodingKeys: String, CodingKey {
        case available
        case from
        case to
        case tokens
        case counts
        case catalogBaseCost = "catalog_base_cost"
        case providerCost = "provider_cost"
        case knownCatalogBaseCost = "known_catalog_base_cost"
        case knownProviderCost = "known_provider_cost"
        case pricingComplete = "pricing_complete"
        case unpricedComponents = "unpriced_components"
        case warnings
    }
}

public struct DesktopSessionsSnapshotV1: Codable, Equatable, Sendable {
    public let available: Bool
    public let total: Int
    public let items: [DesktopRecentSessionV1]
}

public struct DesktopRecentSessionV1: Codable, Equatable, Sendable {
    public let client: String
    public let sessionID: String
    public let project: String?
    public let model: String?
    public let firstAt: String?
    public let lastAt: String?

    enum CodingKeys: String, CodingKey {
        case client
        case sessionID = "session_id"
        case project
        case model
        case firstAt = "first_at"
        case lastAt = "last_at"
    }
}

public struct DesktopHealthSnapshotV1: Codable, Equatable, Sendable {
    public let available: Bool
    public let status: String?
    public let healthy: Bool
    public let problems: Int
    public let warnings: Int
    public let errors: Int
    public let checks: [DesktopHealthCheckV1]
}

public struct DesktopHealthCheckV1: Codable, Equatable, Sendable {
    public let name: String
    public let status: String
    public let code: String?
    public let count: Int?
    public let recoveryCommand: String?

    enum CodingKeys: String, CodingKey {
        case name
        case status
        case code
        case count
        case recoveryCommand = "recovery_command"
    }
}

public func decodeDesktopWireEnvelopeV1(_ data: Data) throws -> DesktopWireEnvelopeV1 {
	try JSONDecoder().decode(DesktopWireEnvelopeV1.self, from: data)
}

private func isRFC3339Timestamp(_ value: String) -> Bool {
	let pattern = #"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$"#
	guard value.range(of: pattern, options: .regularExpression) != nil else {
		return false
	}

	let basic = ISO8601DateFormatter()
	basic.formatOptions = [.withInternetDateTime]
	if basic.date(from: value) != nil {
		return true
	}

	let fractional = ISO8601DateFormatter()
	fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
	return fractional.date(from: value) != nil
}
