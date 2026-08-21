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
	public let candidates: [DesktopProviderCandidateV1]

	enum CodingKeys: String, CodingKey {
		case available
		case routes
		case candidates
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		available = try container.decode(Bool.self, forKey: .available)
		routes = try container.decode([DesktopProviderRouteV1].self, forKey: .routes)
		candidates = try container.decodeIfPresent([DesktopProviderCandidateV1].self, forKey: .candidates) ?? []
	}
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

public struct DesktopProviderCandidateV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { provider }
	public let provider: String
	public let builtIn: Bool
	public let clients: [String]
	public let credentials: [DesktopProviderCredentialV1]
	public let hasWrapper: Bool
	public let ready: Bool
	public let options: [DesktopProviderSwitchOptionV1]

	enum CodingKeys: String, CodingKey {
		case provider
		case builtIn = "built_in"
		case clients
		case credentials
		case hasWrapper = "has_wrapper"
		case ready
		case options
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		provider = try container.decode(String.self, forKey: .provider)
		builtIn = try container.decode(Bool.self, forKey: .builtIn)
		clients = try container.decode([String].self, forKey: .clients)
		credentials = try container.decode([DesktopProviderCredentialV1].self, forKey: .credentials)
		hasWrapper = try container.decode(Bool.self, forKey: .hasWrapper)
		ready = try container.decode(Bool.self, forKey: .ready)
		options = try container.decodeIfPresent([DesktopProviderSwitchOptionV1].self, forKey: .options) ?? []
	}
}

public struct DesktopProviderCredentialV1: Codable, Equatable, Sendable {
	public let name: String
	public let clients: [String]
	public let present: Bool
}

public struct DesktopProviderSwitchOptionV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String {
		[client, provider, credential ?? "", viaWrapper ? "via" : "direct"].joined(separator: "\u{0}")
	}
	public let client: String
	public let provider: String
	public let credential: String?
	public let viaWrapper: Bool
	public let ready: Bool
	public let reasonCode: String?

	enum CodingKeys: String, CodingKey {
		case client
		case provider
		case credential
		case viaWrapper = "via_wrapper"
		case ready
		case reasonCode = "reason_code"
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
	public let hourly: DesktopUsageHourlyV1?
	public let quality: DesktopUsageQualityV1
	public let pricing: DesktopUsagePricingV1
	public let rhythm: DesktopUsageRhythmV1

	enum CodingKeys: String, CodingKey {
		case client, periods, daily, hourly, quality, pricing, rhythm
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		client = try container.decode(String.self, forKey: .client)
		periods = try container.decode(DesktopUsagePeriodsV1.self, forKey: .periods)
		daily = try container.decode(DesktopUsageDailyV1.self, forKey: .daily)
		if container.contains(.hourly) {
			guard try !container.decodeNil(forKey: .hourly) else {
				throw DesktopWireError.invalidEnvelope
			}
			hourly = try container.decode(DesktopUsageHourlyV1.self, forKey: .hourly)
		} else {
			hourly = nil
		}
		quality = try container.decode(DesktopUsageQualityV1.self, forKey: .quality)
		pricing = try container.decode(DesktopUsagePricingV1.self, forKey: .pricing)
		rhythm = try container.decode(DesktopUsageRhythmV1.self, forKey: .rhythm)
	}
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

public struct DesktopPresentationValueV1: Codable, Equatable, Sendable {
	public let tokens: Int64
	public let events: Int64
	public let providerCost: String
	public let costIncomplete: Bool

	enum CodingKeys: String, CodingKey {
		case tokens
		case events
		case providerCost = "provider_cost"
		case costIncomplete = "cost_incomplete"
	}

	init(legacy totals: DesktopPresentationTotalsV1) {
		tokens = totals.tokens
		events = totals.events
		providerCost = totals.providerCost ?? totals.knownProviderCost
		costIncomplete = totals.providerCost == nil
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
	public let value: DesktopPresentationValueV1
	public let share: String?

	enum CodingKeys: String, CodingKey {
		case client, model, value, totals, share
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		client = try container.decodeIfPresent(String.self, forKey: .client)
		model = try container.decode(String.self, forKey: .model)
		share = try container.decodeIfPresent(String.self, forKey: .share)
		value = try container.decodeIfPresent(DesktopPresentationValueV1.self, forKey: .value)
			?? DesktopPresentationValueV1(legacy: container.decode(DesktopPresentationTotalsV1.self, forKey: .totals))
	}

	public func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encodeIfPresent(client, forKey: .client)
		try container.encode(model, forKey: .model)
		try container.encode(value, forKey: .value)
		try container.encodeIfPresent(share, forKey: .share)
	}
}

public struct DesktopUsageDailyV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let items: [DesktopUsageDailyItemV1]
}

public struct DesktopUsageDailyItemV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { date }
	public let date: String
	public let value: DesktopPresentationValueV1

	enum CodingKeys: String, CodingKey { case date, value, totals }

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		date = try container.decode(String.self, forKey: .date)
		value = try container.decodeIfPresent(DesktopPresentationValueV1.self, forKey: .value)
			?? DesktopPresentationValueV1(legacy: container.decode(DesktopPresentationTotalsV1.self, forKey: .totals))
	}

	public func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encode(date, forKey: .date)
		try container.encode(value, forKey: .value)
	}
}

public struct DesktopUsageHourlyV1: Codable, Equatable, Sendable {
	public let available: Bool
	public let throughHour: Int
	public let items: [DesktopUsageHourlyItemV1]

	enum CodingKeys: String, CodingKey {
		case available
		case throughHour = "through_hour"
		case items
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		let available = try container.decode(Bool.self, forKey: .available)
		let throughHour = try container.decode(Int.self, forKey: .throughHour)
		let items = try container.decode([DesktopUsageHourlyItemV1].self, forKey: .items)
		guard (0 ... 23).contains(throughHour) else {
			throw DesktopWireError.invalidEnvelope
		}
		if available {
			guard items.count == throughHour + 1,
				items.enumerated().allSatisfy({ offset, item in item.hour == offset })
			else {
				throw DesktopWireError.invalidEnvelope
			}
		} else if !items.isEmpty {
			throw DesktopWireError.invalidEnvelope
		}
		self.available = available
		self.throughHour = throughHour
		self.items = items
	}
}

public struct DesktopUsageHourlyItemV1: Codable, Equatable, Sendable, Identifiable {
	public var id: Int { hour }
	public let hour: Int
	public let value: DesktopPresentationValueV1

	enum CodingKeys: String, CodingKey { case hour, value, totals }

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		hour = try container.decode(Int.self, forKey: .hour)
		value = try container.decodeIfPresent(DesktopPresentationValueV1.self, forKey: .value)
			?? DesktopPresentationValueV1(legacy: container.decode(DesktopPresentationTotalsV1.self, forKey: .totals))
	}

	public func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encode(hour, forKey: .hour)
		try container.encode(value, forKey: .value)
	}
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
	public let value: DesktopPresentationValueV1
	public let share: String?

	enum CodingKeys: String, CodingKey { case quality, value, totals, share }

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		quality = try container.decode(String.self, forKey: .quality)
		share = try container.decodeIfPresent(String.self, forKey: .share)
		value = try container.decodeIfPresent(DesktopPresentationValueV1.self, forKey: .value)
			?? DesktopPresentationValueV1(legacy: container.decode(DesktopPresentationTotalsV1.self, forKey: .totals))
	}

	public func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encode(quality, forKey: .quality)
		try container.encode(value, forKey: .value)
		try container.encodeIfPresent(share, forKey: .share)
	}
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
	public let intensities: [Int]
	public let tokens: [Int64]
	public let providerCosts: [String]
	public let costIncomplete: [Bool]
	public let activeDays: Int
	public let busiestDay: String
	public let quietestDay: String
	private let hoverAvailable: Bool

	public var cells: [DesktopUsageRhythmCellV1] {
		guard intensities.count == tokens.count,
			intensities.count == providerCosts.count,
			intensities.count == costIncomplete.count
		else {
			return []
		}
		return intensities.indices.map { index in
			DesktopUsageRhythmCellV1(
				weekday: index / 24,
				hour: index % 24,
				intensity: intensities[index],
				tokens: hoverAvailable ? tokens[index] : nil,
				providerCost: hoverAvailable ? providerCosts[index] : nil,
				costIncomplete: hoverAvailable ? costIncomplete[index] : nil
			)
		}
	}

	enum CodingKeys: String, CodingKey {
		case available
		case intensities
		case tokens
		case providerCosts = "provider_costs"
		case costIncomplete = "cost_incomplete"
		case cells
		case activeDays = "active_days"
		case busiestDay = "busiest_day"
		case quietestDay = "quietest_day"
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		available = try container.decode(Bool.self, forKey: .available)
		activeDays = try container.decode(Int.self, forKey: .activeDays)
		busiestDay = try container.decode(String.self, forKey: .busiestDay)
		quietestDay = try container.decode(String.self, forKey: .quietestDay)
		if let packedIntensities = try container.decodeIfPresent([Int].self, forKey: .intensities) {
			hoverAvailable = true
			intensities = packedIntensities
			tokens = try container.decode([Int64].self, forKey: .tokens)
			providerCosts = try container.decode([String].self, forKey: .providerCosts)
			costIncomplete = try container.decode([Bool].self, forKey: .costIncomplete)
			guard intensities.count == tokens.count,
				intensities.count == providerCosts.count,
				intensities.count == costIncomplete.count
			else {
				throw DecodingError.dataCorruptedError(
					forKey: .intensities,
					in: container,
					debugDescription: "rhythm parallel arrays have different lengths"
				)
			}
		} else {
			let legacyCells = try container.decode([DesktopUsageRhythmCellV1].self, forKey: .cells)
			hoverAvailable = legacyCells.allSatisfy {
				$0.tokens != nil && $0.providerCost != nil && $0.costIncomplete != nil
			}
			intensities = legacyCells.map(\.intensity)
			tokens = legacyCells.map { $0.tokens ?? 0 }
			providerCosts = legacyCells.map { $0.providerCost ?? "" }
			costIncomplete = legacyCells.map { $0.costIncomplete ?? true }
		}
	}

	public func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encode(available, forKey: .available)
		try container.encode(intensities, forKey: .intensities)
		try container.encode(tokens, forKey: .tokens)
		try container.encode(providerCosts, forKey: .providerCosts)
		try container.encode(costIncomplete, forKey: .costIncomplete)
		try container.encode(activeDays, forKey: .activeDays)
		try container.encode(busiestDay, forKey: .busiestDay)
		try container.encode(quietestDay, forKey: .quietestDay)
	}
}

public struct DesktopUsageRhythmCellV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { "\(weekday):\(hour)" }
	public let weekday: Int
	public let hour: Int
	public let intensity: Int
	public let tokens: Int64?
	public let providerCost: String?
	public let costIncomplete: Bool?

	enum CodingKeys: String, CodingKey {
		case weekday
		case hour
		case intensity
		case tokens
		case providerCost = "provider_cost"
		case costIncomplete = "cost_incomplete"
	}
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
	public let value: DesktopPresentationValueV1

	enum CodingKeys: String, CodingKey { case period, client, value, totals }

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		period = try container.decode(String.self, forKey: .period)
		client = try container.decode(String.self, forKey: .client)
		value = try container.decodeIfPresent(DesktopPresentationValueV1.self, forKey: .value)
			?? DesktopPresentationValueV1(legacy: container.decode(DesktopPresentationTotalsV1.self, forKey: .totals))
	}

	public func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encode(period, forKey: .period)
		try container.encode(client, forKey: .client)
		try container.encode(value, forKey: .value)
	}
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
	public let projects: [DesktopSessionsProjectV1]

    enum CodingKeys: String, CodingKey {
        case period
        case client
        case sessions
        case totalDurationSeconds = "total_duration_seconds"
        case medianDurationSeconds = "median_duration_seconds"
        case distinctProjects = "distinct_projects"
		case projects
    }

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		period = try container.decode(String.self, forKey: .period)
		client = try container.decode(String.self, forKey: .client)
		sessions = try container.decode(Int.self, forKey: .sessions)
		totalDurationSeconds = try container.decode(Int64.self, forKey: .totalDurationSeconds)
		medianDurationSeconds = try container.decode(Int64.self, forKey: .medianDurationSeconds)
		distinctProjects = try container.decode(Int.self, forKey: .distinctProjects)
		projects = try container.decodeIfPresent([DesktopSessionsProjectV1].self, forKey: .projects) ?? []
	}
}

public struct DesktopSessionsProjectV1: Codable, Equatable, Sendable, Identifiable {
	public var id: String { project ?? "" }
	public let project: String?
	public let sessions: Int
	public let durationSeconds: Int64

	enum CodingKeys: String, CodingKey {
		case project
		case sessions
		case durationSeconds = "duration_seconds"
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

public struct ProviderUseEnvelopeV1: Decodable, Equatable, Sendable {
	public static let schemaVersion = 1
	public static let command = "provider.use"

	public let schemaVersion: Int
	public let command: String
	public let generatedAt: String
	public let warnings: [String]
	public let partial: Bool
	public let errorCode: String?

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case command
		case generatedAt = "generated_at"
		case data
		case warnings
		case partial
		case error
	}

	private struct EncodedError: Decodable {
		let code: String
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		let schemaVersion = try container.decode(Int.self, forKey: .schemaVersion)
		let command = try container.decode(String.self, forKey: .command)
		let generatedAt = try container.decode(String.self, forKey: .generatedAt)
		let dataIsNull = try (!container.contains(.data) || container.decodeNil(forKey: .data))
		guard schemaVersion == Self.schemaVersion,
			command == Self.command,
			isRFC3339Timestamp(generatedAt),
			dataIsNull
		else {
			throw DesktopWireError.invalidEnvelope
		}
		self.schemaVersion = schemaVersion
		self.command = command
		self.generatedAt = generatedAt
		warnings = try container.decode([String].self, forKey: .warnings)
		partial = try container.decode(Bool.self, forKey: .partial)
		errorCode = try container.decodeIfPresent(EncodedError.self, forKey: .error)?.code
	}
}

public func decodeProviderUseEnvelopeV1(_ data: Data) throws -> ProviderUseEnvelopeV1 {
	try JSONDecoder().decode(ProviderUseEnvelopeV1.self, from: data)
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
