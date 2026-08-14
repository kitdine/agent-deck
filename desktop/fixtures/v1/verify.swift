import Darwin
import Foundation

enum DesktopWireError: Error, CustomStringConvertible {
    case invalidEnvelope(String)
    case unsupportedWireVersion(Int)

    var description: String {
        switch self {
        case let .invalidEnvelope(message):
            return message
        case let .unsupportedWireVersion(version):
            return "unsupported desktop wire version \(version)"
        }
    }
}

struct DesktopWireEnvelopeV1: Codable, Sendable {
    let schemaVersion: Int
    let command: String
    let generatedAt: String
    let data: DesktopSnapshotV1
    let warnings: [String]
    let partial: Bool
    let error: DesktopWireOutputErrorV1?

    enum CodingKeys: String, CodingKey {
        case schemaVersion = "schema_version"
        case command
        case generatedAt = "generated_at"
        case data
        case warnings
        case partial
        case error
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        schemaVersion = try container.decode(Int.self, forKey: .schemaVersion)
        command = try container.decode(String.self, forKey: .command)
        generatedAt = try container.decode(String.self, forKey: .generatedAt)
        data = try container.decode(DesktopSnapshotV1.self, forKey: .data)
        warnings = try container.decode([String].self, forKey: .warnings)
        partial = try container.decode(Bool.self, forKey: .partial)
        error = try container.decodeIfPresent(DesktopWireOutputErrorV1.self, forKey: .error)

        guard schemaVersion == 1, command == "desktop.snapshot", error == nil else {
            throw DesktopWireError.invalidEnvelope("invalid desktop snapshot envelope identity")
        }
        guard data.wireVersion == 1 else {
            throw DesktopWireError.unsupportedWireVersion(data.wireVersion)
        }
    }
}

struct DesktopWireOutputErrorV1: Codable, Sendable {
    let code: String
    let message: String
}

struct DesktopSnapshotV1: Codable, Sendable {
    let wireVersion: Int
    let generatedAt: String
    let nextRefreshAt: String
    let provider: DesktopProviderSnapshotV1
    let usage: DesktopUsageSnapshotV1
    let sessions: DesktopSessionsSnapshotV1
    let health: DesktopHealthSnapshotV1

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

struct DesktopProviderSnapshotV1: Codable, Sendable {
    let available: Bool
    let routes: [DesktopProviderRouteV1]
}

struct DesktopProviderRouteV1: Codable, Sendable {
    let client: String
    let provider: String
    let selectedAt: String
    let viaWrapper: Bool

    enum CodingKeys: String, CodingKey {
        case client
        case provider
        case selectedAt = "selected_at"
        case viaWrapper = "via_wrapper"
    }
}

struct DesktopUsageSnapshotV1: Codable, Sendable {
    let available: Bool
    let from: String
    let to: String
    let tokens: [String: Int64]
    let counts: [String: Int64]
    let catalogBaseCost: String?
    let providerCost: String?
    let knownCatalogBaseCost: String?
    let knownProviderCost: String?
    let pricingComplete: Bool
    let unpricedComponents: Int
    let warnings: [String]

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

struct DesktopSessionsSnapshotV1: Codable, Sendable {
    let available: Bool
    let total: Int
    let items: [DesktopRecentSessionV1]
}

struct DesktopRecentSessionV1: Codable, Sendable {
    let client: String
    let sessionID: String
    let project: String?
    let model: String?
    let firstAt: String?
    let lastAt: String?

    enum CodingKeys: String, CodingKey {
        case client
        case sessionID = "session_id"
        case project
        case model
        case firstAt = "first_at"
        case lastAt = "last_at"
    }
}

struct DesktopHealthSnapshotV1: Codable, Sendable {
    let available: Bool
    let status: String?
    let healthy: Bool
    let problems: Int
    let warnings: Int
    let errors: Int
    let checks: [DesktopHealthCheckV1]
}

struct DesktopHealthCheckV1: Codable, Sendable {
    let name: String
    let status: String
    let code: String?
    let count: Int?
    let recoveryCommand: String?

    enum CodingKeys: String, CodingKey {
        case name
        case status
        case code
        case count
        case recoveryCommand = "recovery_command"
    }
}

func verifyFixture(at path: String) throws -> DesktopWireEnvelopeV1 {
    let contents = try Data(contentsOf: URL(fileURLWithPath: path))
    let envelope = try JSONDecoder().decode(DesktopWireEnvelopeV1.self, from: contents)
    guard !envelope.generatedAt.isEmpty,
          !envelope.data.generatedAt.isEmpty,
          !envelope.data.nextRefreshAt.isEmpty
    else {
        throw DesktopWireError.invalidEnvelope("missing desktop snapshot timestamp in \(path)")
    }

    if path.hasSuffix("snapshot-complete.json") {
        guard !envelope.partial,
              envelope.warnings.isEmpty,
              envelope.data.provider.available,
              envelope.data.usage.available,
              envelope.data.sessions.available,
              envelope.data.health.available
        else {
            throw DesktopWireError.invalidEnvelope("invalid complete fixture availability")
        }
    } else if path.hasSuffix("snapshot-partial.json") {
        guard envelope.partial,
              !envelope.warnings.isEmpty,
              !envelope.data.provider.available,
              !envelope.data.usage.available,
              !envelope.data.sessions.available,
              envelope.data.health.available
        else {
            throw DesktopWireError.invalidEnvelope("invalid partial fixture availability")
        }
    }
    return envelope
}

func verifyUnsupportedVersionRejection(in path: String) throws {
    let contents = try Data(contentsOf: URL(fileURLWithPath: path))
    guard var object = try JSONSerialization.jsonObject(with: contents) as? [String: Any],
          var snapshot = object["data"] as? [String: Any]
    else {
        throw DesktopWireError.invalidEnvelope("cannot mutate fixture for version rejection")
    }
    snapshot["wire_version"] = 2
    object["data"] = snapshot
    let unsupported = try JSONSerialization.data(withJSONObject: object)
    do {
        _ = try JSONDecoder().decode(DesktopWireEnvelopeV1.self, from: unsupported)
    } catch DesktopWireError.unsupportedWireVersion(2) {
        return
    } catch {
        throw DesktopWireError.invalidEnvelope("unexpected unsupported-version error: \(error)")
    }
    throw DesktopWireError.invalidEnvelope("Swift decoder accepted unsupported wire version 2")
}

let fixturePaths = Array(CommandLine.arguments.dropFirst())
do {
    guard fixturePaths.count == 2 else {
        throw DesktopWireError.invalidEnvelope("expected complete and partial fixture paths")
    }
    for path in fixturePaths {
        _ = try verifyFixture(at: path)
    }
    try verifyUnsupportedVersionRejection(in: fixturePaths[0])
    print("verified desktop wire v1 Swift fixtures: \(fixturePaths.count)")
} catch {
    FileHandle.standardError.write(Data("desktop wire fixture verification failed: \(error)\n".utf8))
    exit(EXIT_FAILURE)
}
