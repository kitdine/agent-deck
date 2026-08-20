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
    let presentation: DesktopUsagePresentationV1

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
        case presentation
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        available = try container.decode(Bool.self, forKey: .available)
        from = try container.decode(String.self, forKey: .from)
        to = try container.decode(String.self, forKey: .to)
        tokens = try container.decode([String: Int64].self, forKey: .tokens)
        counts = try container.decode([String: Int64].self, forKey: .counts)
        catalogBaseCost = try container.decodeIfPresent(String.self, forKey: .catalogBaseCost)
        providerCost = try container.decodeIfPresent(String.self, forKey: .providerCost)
        knownCatalogBaseCost = try container.decodeIfPresent(String.self, forKey: .knownCatalogBaseCost)
        knownProviderCost = try container.decodeIfPresent(String.self, forKey: .knownProviderCost)
        pricingComplete = try container.decode(Bool.self, forKey: .pricingComplete)
        unpricedComponents = try container.decode(Int.self, forKey: .unpricedComponents)
        warnings = try container.decode([String].self, forKey: .warnings)
        // Additive and version-neutral: a legacy payload without the object
        // decodes as unavailable rather than failing, matching the production
        // decoder in AgentDeckShared. This standalone gate previously ignored
        // presentation entirely, so it could not have caught either behavior.
        presentation = try container.decodeIfPresent(DesktopUsagePresentationV1.self, forKey: .presentation)
            ?? DesktopUsagePresentationV1.unavailable
    }
}

struct DesktopUsagePresentationV1: Codable, Sendable {
    static let unavailable = DesktopUsagePresentationV1(
        available: false,
        scopes: [],
        clientSubtotals: DesktopClientSubtotalsV1(available: false, items: [])
    )

    let available: Bool
    let scopes: [DesktopUsageScopeV1]
    let clientSubtotals: DesktopClientSubtotalsV1

    enum CodingKeys: String, CodingKey {
        case available
        case scopes
        case clientSubtotals = "client_subtotals"
    }
}

struct DesktopUsageScopeV1: Codable, Sendable {
    let client: String
    let periods: DesktopUsagePeriodsV1
    let daily: DesktopUsageDailyV1
    let quality: DesktopUsageQualityV1
    let pricing: DesktopUsagePricingV1
    let rhythm: DesktopUsageRhythmV1
}

struct DesktopUsagePeriodsV1: Codable, Sendable {
    let available: Bool
    let items: [DesktopUsagePeriodV1]
}

struct DesktopUsagePeriodV1: Codable, Sendable {
    let period: String
    let totals: DesktopPresentationTotalsV1
}

struct DesktopPresentationTotalsV1: Codable, Sendable {
    let tokens: Int64
}

struct DesktopUsageDailyV1: Codable, Sendable {
    let available: Bool
    let items: [DesktopUsageDailyItemV1]
}

struct DesktopUsageDailyItemV1: Codable, Sendable {
    let date: String
}

struct DesktopUsageQualityV1: Codable, Sendable {
    let available: Bool
    let items: [DesktopUsageQualityItemV1]
}

struct DesktopUsageQualityItemV1: Codable, Sendable {
    let period: String
    let provider: String?
}

struct DesktopUsagePricingV1: Codable, Sendable {
    let available: Bool
    let items: [DesktopUsagePricingItemV1]
}

struct DesktopUsagePricingItemV1: Codable, Sendable {
    let period: String
}

struct DesktopUsageRhythmV1: Codable, Sendable {
    let available: Bool
    let cells: [DesktopUsageRhythmCellV1]
}

struct DesktopUsageRhythmCellV1: Codable, Sendable {
    let weekday: Int
    let hour: Int
    let intensity: Int
}

struct DesktopClientSubtotalsV1: Codable, Sendable {
    let available: Bool
    let items: [DesktopClientSubtotalV1]
}

struct DesktopClientSubtotalV1: Codable, Sendable {
    let period: String
    let client: String
}

struct DesktopSessionsSnapshotV1: Codable, Sendable {
    let available: Bool
    let total: Int
    let periods: DesktopSessionsPeriodsV1
    let items: [DesktopRecentSessionV1]

    enum CodingKeys: String, CodingKey {
        case available
        case total
        case periods
        case items
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        available = try container.decode(Bool.self, forKey: .available)
        total = try container.decode(Int.self, forKey: .total)
        periods = try container.decodeIfPresent(DesktopSessionsPeriodsV1.self, forKey: .periods)
            ?? DesktopSessionsPeriodsV1.unavailable
        items = try container.decode([DesktopRecentSessionV1].self, forKey: .items)
    }
}

struct DesktopSessionsPeriodsV1: Codable, Sendable {
    static let unavailable = DesktopSessionsPeriodsV1(available: false, items: [])

    let available: Bool
    let items: [DesktopSessionsPeriodItemV1]
}

struct DesktopSessionsPeriodItemV1: Codable, Sendable {
    let period: String
    let client: String
    let sessions: Int
    let totalDurationSeconds: Int64
    let medianDurationSeconds: Int64
    let distinctProjects: Int

    enum CodingKeys: String, CodingKey {
        case period
        case client
        case sessions
        case totalDurationSeconds = "total_duration_seconds"
        case medianDurationSeconds = "median_duration_seconds"
        case distinctProjects = "distinct_projects"
    }
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
        // One record per supported period per client scope. A missing record is
        // what would push the host into deriving a figure from the recent list.
        guard envelope.data.sessions.periods.available,
              envelope.data.sessions.periods.items.count == 9,
              Set(envelope.data.sessions.periods.items.map(\.period)) == ["today", "7d", "30d"],
              Set(envelope.data.sessions.periods.items.map(\.client)) == ["all", "codex", "claude"]
        else {
            throw DesktopWireError.invalidEnvelope("invalid complete fixture session periods")
        }
        try verifyPresentationBounds(envelope, distinguishPeriods: true, path: path)
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
        // The real producer reports an unavailable session-period family with an
        // empty collection. The fixture used to claim the family was available
        // beside an unavailable session index, which is a state the producer
        // cannot reach and which hid a null collection this decoder rejects.
        guard !envelope.data.sessions.periods.available,
              envelope.data.sessions.periods.items.isEmpty
        else {
            throw DesktopWireError.invalidEnvelope("invalid partial fixture session periods")
        }
    } else if path.hasSuffix("snapshot-empty-client.json") {
        // A concrete client with no data keeps its record and reports that no
        // family was supplied, rather than presenting synthetic zeros.
        try verifyPresentationBounds(envelope, distinguishPeriods: false, path: path)
        guard let empty = envelope.data.usage.presentation.scopes.first(where: { $0.client == "claude" }),
              !empty.periods.available, empty.periods.items.isEmpty,
              !empty.daily.available, empty.daily.items.isEmpty,
              !empty.quality.available, empty.quality.items.isEmpty,
              !empty.pricing.available, empty.pricing.items.isEmpty,
              !empty.rhythm.available, empty.rhythm.cells.isEmpty
        else {
            throw DesktopWireError.invalidEnvelope("invalid empty-client fixture scope")
        }
    } else if path.hasSuffix("snapshot-legacy.json") {
        // Both additive families this task owns are absent here. A legacy
        // payload must decode as unavailable rather than fail, without raising
        // wire_version. `provider.candidates` is another task's additive object
        // and is asserted by that task's decoder path, not by this one.
        guard envelope.data.wireVersion == 1,
              !envelope.data.usage.presentation.available,
              envelope.data.usage.presentation.scopes.isEmpty,
              !envelope.data.usage.presentation.clientSubtotals.available,
              !envelope.data.sessions.periods.available,
              envelope.data.sessions.periods.items.isEmpty
        else {
            throw DesktopWireError.invalidEnvelope("invalid legacy fixture additive fallback")
        }
    }
    return envelope
}

// verifyPresentationBounds holds a fixture to the fixed collection bounds the
// contract states. A fixture the producer cannot emit proves nothing about the
// producer, which is what one scope, one period and one rhythm cell were doing.
func verifyPresentationBounds(_ envelope: DesktopWireEnvelopeV1, distinguishPeriods: Bool, path: String) throws {
    let presentation = envelope.data.usage.presentation
    guard presentation.available,
          presentation.scopes.map(\.client) == ["all", "codex", "claude"],
          presentation.clientSubtotals.available,
          presentation.clientSubtotals.items.count == 6
    else {
        throw DesktopWireError.invalidEnvelope("invalid presentation scope set in \(path)")
    }
    for scope in presentation.scopes where scope.periods.available {
        guard scope.periods.items.map(\.period) == ["today", "7d", "30d"],
              scope.daily.available, scope.daily.items.count == 90,
              scope.rhythm.available, scope.rhythm.cells.count == 168,
              scope.pricing.available, scope.pricing.items.map(\.period) == ["today", "7d", "30d"],
              scope.quality.available, Set(scope.quality.items.map(\.period)) == ["today", "7d", "30d"]
        else {
            throw DesktopWireError.invalidEnvelope("invalid presentation bounds for \(scope.client) in \(path)")
        }
        // Copying today's figures into the wider periods must be visible.
        if distinguishPeriods {
            let totals = scope.periods.items.map(\.totals.tokens)
            guard totals[0] != totals[1], totals[1] != totals[2] else {
                throw DesktopWireError.invalidEnvelope("period totals do not distinguish periods for \(scope.client)")
            }
        }
    }
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
    guard fixturePaths.count >= 2 else {
        throw DesktopWireError.invalidEnvelope("expected at least the complete and partial fixture paths")
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
