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
	public let presentation: DesktopUsagePresentationV1

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

	public init(from decoder: Decoder) throws {
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
		presentation = try container.decodeIfPresent(DesktopUsagePresentationV1.self, forKey: .presentation) ?? .unavailable
	}
}

public struct DesktopUsagePresentationV1: Codable, Equatable, Sendable {
	public static let unavailable = DesktopUsagePresentationV1(
		available: false,
		scopes: [],
		clientSubtotals: DesktopClientSubtotalsV1(available: false, items: [])
	)

	public let available: Bool
	public let scopes: [DesktopUsageScopeV1]
	public let clientSubtotals: DesktopClientSubtotalsV1

	public init(available: Bool, scopes: [DesktopUsageScopeV1], clientSubtotals: DesktopClientSubtotalsV1) {
		self.available = available
		self.scopes = scopes
		self.clientSubtotals = clientSubtotals
	}

	enum CodingKeys: String, CodingKey {
		case available
		case scopes
		case clientSubtotals = "client_subtotals"
	}
}

public struct DesktopUsageScopeV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { client }
	public let client: String
	public let periods: DesktopUsagePeriodsV1
	public let daily: DesktopUsageDailyV1
	public let quality: DesktopUsageQualityV1
	public let pricing: DesktopUsagePricingV1
	public let rhythm: DesktopUsageRhythmV1
}

public struct DesktopUsagePeriodsV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let items: [DesktopUsagePeriodV1]
}

public struct DesktopUsagePeriodV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { period }
	public let period: String
	public let totals: DesktopPresentationTotalsV1
	public let averagePerDay: DesktopPresentationAverageV1
	public let peak: DesktopPresentationPeakV1
	public let cacheHitShare: String?
	public let models: [DesktopPresentationModelV1]

	enum CodingKeys: String, CodingKey {
		case period
		case totals
		case averagePerDay = "average_per_day"
		case peak
		case cacheHitShare = "cache_hit_share"
		case models
	}
}

public struct DesktopPresentationTotalsV1: Codable, Equatable, Sendable {
	public let tokens: Int64
	public let inputTokens: Int64
	public let outputTokens: Int64
	public let cachedReadTokens: Int64
	public let cacheWriteTokens: Int64
	public let events: Int64
	public let sessions: Int64
	public let catalogBaseCost: String?
	public let providerCost: String?
	public let knownCatalogBaseCost: String
	public let knownProviderCost: String
	public let pricingComplete: Bool
	public let unpricedComponents: Int

	enum CodingKeys: String, CodingKey {
		case tokens
		case inputTokens = "input_tokens"
		case outputTokens = "output_tokens"
		case cachedReadTokens = "cached_read_tokens"
		case cacheWriteTokens = "cache_write_tokens"
		case events
		case sessions
		case catalogBaseCost = "catalog_base_cost"
		case providerCost = "provider_cost"
		case knownCatalogBaseCost = "known_catalog_base_cost"
		case knownProviderCost = "known_provider_cost"
		case pricingComplete = "pricing_complete"
		case unpricedComponents = "unpriced_components"
	}
}

public struct DesktopPresentationAverageV1: Codable, Equatable, Sendable {
	public let tokens: String
	public let providerCost: String?
	public let knownProviderCost: String

	enum CodingKeys: String, CodingKey {
		case tokens
		case providerCost = "provider_cost"
		case knownProviderCost = "known_provider_cost"
	}
}

public struct DesktopPresentationPeakV1: Codable, Equatable, Sendable {
	public let date: String
	public let totals: DesktopPresentationTotalsV1
}

public struct DesktopPresentationModelV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { [client ?? "", model].joined(separator: "\u{0}") }
	public let client: String?
	public let model: String
	public let totals: DesktopPresentationTotalsV1
	public let share: String?
}

public struct DesktopUsageDailyV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let items: [DesktopUsageDailyItemV1]
}

public struct DesktopUsageDailyItemV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { date }
	public let date: String
	public let totals: DesktopPresentationTotalsV1
}

public struct DesktopUsageQualityV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let items: [DesktopUsageQualityItemV1]
}

public struct DesktopUsageQualityItemV1: Codable, Equatable, Sendable, Identifiable {
	// The family carries one record set per period, so the identity has to name
	// both: a provider alone repeats across periods and would collapse them.
	public var id: String { period + "/" + (provider ?? "all") }
	public let period: String
	public let provider: String?
	public let tiers: [DesktopUsageQualityTierV1]
}

public struct DesktopUsageQualityTierV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { quality }
	public let quality: String
	public let totals: DesktopPresentationTotalsV1
	public let share: String?
}

public struct DesktopUsagePricingV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let items: [DesktopUsagePricingItemV1]
}

public struct DesktopUsagePricingItemV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { period }
	public let period: String
	public let pricedEvents: Int64
	public let unpricedEvents: Int64
	public let coverage: String
	public let unpricedIdentifiers: [String]

	enum CodingKeys: String, CodingKey {
		case period
		case pricedEvents = "priced_events"
		case unpricedEvents = "unpriced_events"
		case coverage
		case unpricedIdentifiers = "unpriced_identifiers"
	}
}

public struct DesktopUsageRhythmV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let cells: [DesktopUsageRhythmCellV1]
	public let activeDays: Int
	public let busiestDay: String
	public let quietestDay: String

	enum CodingKeys: String, CodingKey {
		case available
		case cells
		case activeDays = "active_days"
		case busiestDay = "busiest_day"
		case quietestDay = "quietest_day"
	}
}

public struct DesktopUsageRhythmCellV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { "\(weekday):\(hour)" }
	public let weekday: Int
	public let hour: Int
	public let intensity: Int
}

public struct DesktopClientSubtotalsV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let items: [DesktopClientSubtotalV1]

	public init(available: Bool, items: [DesktopClientSubtotalV1]) {
		self.available = available
		self.items = items
	}
}

public struct DesktopClientSubtotalV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { "\(period):\(client)" }
	public let period: String
	public let client: String
	public let totals: DesktopPresentationTotalsV1
}

public struct DesktopSessionsSnapshotV1: Codable, Equatable, Sendable {
    public let available: Bool
    public let total: Int
    public let periods: DesktopSessionsPeriodsV1
    public let items: [DesktopRecentSessionV1]

    enum CodingKeys: String, CodingKey {
        case available
        case total
        case periods
        case items
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        available = try container.decode(Bool.self, forKey: .available)
        total = try container.decode(Int.self, forKey: .total)
        // The field is additive and does not raise `wire_version`, so a payload
        // that predates it must decode as an unavailable family rather than as
        // an error — the same rule `presentation` already follows.
        periods = try container.decodeIfPresent(DesktopSessionsPeriodsV1.self, forKey: .periods) ?? .unavailable
        items = try container.decode([DesktopRecentSessionV1].self, forKey: .items)
    }
}

/// Per-period session statistics, producer-computed. The host selects a
/// `(Client, Period)` record and formats it; it never derives these from the
/// bounded recent list below, which stays a recent list rather than a query.
public struct DesktopSessionsPeriodsV1: Codable, Equatable, Sendable {
    public static let unavailable = DesktopSessionsPeriodsV1(available: false, items: [])

    public let available: Bool
    public let items: [DesktopSessionsPeriodItemV1]

    public init(available: Bool, items: [DesktopSessionsPeriodItemV1]) {
        self.available = available
        self.items = items
    }
}

public struct DesktopSessionsPeriodItemV1: Codable, Equatable, Sendable, Identifiable {
    public var id: String { period + "/" + client }
    public let period: String
    public let client: String
    public let sessions: Int
    public let totalDurationSeconds: Int64
    public let medianDurationSeconds: Int64
    public let distinctProjects: Int

    enum CodingKeys: String, CodingKey {
        case period
        case client
        case sessions
        case totalDurationSeconds = "total_duration_seconds"
        case medianDurationSeconds = "median_duration_seconds"
        case distinctProjects = "distinct_projects"
    }
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
