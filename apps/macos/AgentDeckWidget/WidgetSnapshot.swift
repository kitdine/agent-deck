import Foundation

struct WidgetDesktopSnapshotV1: Codable, Equatable, Sendable {
	static let schemaVersion = 1

	let schemaVersion: Int
	let generatedAt: String
	let nextRefreshAt: String
	let partial: Bool
	let usage: WidgetUsageSnapshotV1
	let subscription: DesktopSubscriptionSnapshotV1
	/// Decoded apart from the rest: a malformed summary degrades only the
	/// session row, never the card or the usage beside it.
	let sessionScopeSummary: WidgetSessionScopeSummaryField

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case generatedAt = "generated_at"
		case nextRefreshAt = "next_refresh_at"
		case partial
		case usage, subscription
		case sessionScopeSummary = "session_scope_summary"
	}

	init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		schemaVersion = try container.decode(Int.self, forKey: .schemaVersion)
		generatedAt = try container.decode(String.self, forKey: .generatedAt)
		nextRefreshAt = try container.decode(String.self, forKey: .nextRefreshAt)
		partial = try container.decode(Bool.self, forKey: .partial)
		usage = try container.decode(WidgetUsageSnapshotV1.self, forKey: .usage)
		subscription = try container.decodeIfPresent(DesktopSubscriptionSnapshotV1.self, forKey: .subscription) ?? .unavailable
		if !container.contains(.sessionScopeSummary) {
			sessionScopeSummary = .legacy
		} else if let set = try? container.decode(WidgetSessionScopeSummarySetV1.self, forKey: .sessionScopeSummary) {
			sessionScopeSummary = .present(set)
		} else {
			sessionScopeSummary = .invalid
		}
	}

	func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encode(schemaVersion, forKey: .schemaVersion)
		try container.encode(generatedAt, forKey: .generatedAt)
		try container.encode(nextRefreshAt, forKey: .nextRefreshAt)
		try container.encode(partial, forKey: .partial)
		try container.encode(usage, forKey: .usage)
		try container.encode(subscription, forKey: .subscription)
		switch sessionScopeSummary {
		case .legacy: break
		case .invalid: try container.encode("invalid", forKey: .sessionScopeSummary)
		case let .present(set): try container.encode(set, forKey: .sessionScopeSummary)
		}
	}
}

/// How a snapshot carries the session-scope summaries. `legacy` is a file
/// from before Task 7 and `invalid` one this reader cannot trust; neither is
/// read as Work, and neither renames the legacy session total.
enum WidgetSessionScopeSummaryField: Equatable, Sendable {
	case legacy
	case invalid
	case present(WidgetSessionScopeSummarySetV1)
}

struct WidgetSessionScopeSummarySetV1: Codable, Equatable, Sendable {
	let stale: Bool
	let reason: String?
	let items: [WidgetSessionScopeItemV1]

	enum CodingKeys: String, CodingKey { case stale, reason, items }

	init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		stale = try container.decode(Bool.self, forKey: .stale)
		reason = try container.decodeIfPresent(String.self, forKey: .reason)
		// reason is a closed safe enum (ux/widget.md). A value this reader does
		// not know makes the whole summary untrusted, so the session row shows
		// only the generic unavailable state; usage is decoded apart and stays.
		if let reason, DesktopSessionScopeSummaryReasonV1(rawValue: reason) == nil {
			throw DecodingError.dataCorruptedError(
				forKey: .reason, in: container, debugDescription: "reason is outside the closed summary reason enum"
			)
		}
		items = try container.decode([WidgetSessionScopeItemV1].self, forKey: .items)
	}
}

/// One accepted client × period summary. Decoding never throws: an item that
/// fails any rule keeps only its client and period (when readable) so the
/// entry it would have answered shows the session row as unavailable.
struct WidgetSessionScopeItemV1: Codable, Equatable, Sendable {
	struct Value: Equatable, Sendable {
		let scope: DesktopSessionScopeV1
		let count: Int
		/// nil means coverage is unknown (shown as —); a missing key is invalid.
		let unknownCount: Int?
		let publicationID: String
		let generatedAt: Date
	}

	let client: String?
	let period: String?
	let value: Value?

	enum CodingKeys: String, CodingKey {
		case available, scope, count, client, period
		case unknownCount = "unknown_count"
		case publicationID = "publication_id"
		case generatedAt = "generated_at"
	}

	init(from decoder: Decoder) throws {
		guard let container = try? decoder.container(keyedBy: CodingKeys.self) else {
			client = nil
			period = nil
			value = nil
			return
		}
		client = try? container.decode(String.self, forKey: .client)
		period = try? container.decode(String.self, forKey: .period)
		value = Self.value(container)
	}

	/// Re-encodes what was read; an invalid item stays invalid.
	func encode(to encoder: Encoder) throws {
		var container = encoder.container(keyedBy: CodingKeys.self)
		try container.encodeIfPresent(client, forKey: .client)
		try container.encodeIfPresent(period, forKey: .period)
		guard let value else {
			try container.encode(false, forKey: .available)
			return
		}
		try container.encode(true, forKey: .available)
		try container.encode(value.scope, forKey: .scope)
		try container.encode(value.count, forKey: .count)
		try container.encode(value.unknownCount, forKey: .unknownCount)
		try container.encode(value.publicationID, forKey: .publicationID)
		let formatter = ISO8601DateFormatter()
		formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
		try container.encode(formatter.string(from: value.generatedAt), forKey: .generatedAt)
	}

	private static func value(_ container: KeyedDecodingContainer<CodingKeys>) -> Value? {
		guard (try? container.decode(Bool.self, forKey: .available)) == true,
			let raw = try? container.decode(String.self, forKey: .scope),
			let scope = DesktopSessionScopeV1(rawValue: raw),
			let count = try? container.decode(Int.self, forKey: .count), count >= 0,
			container.contains(.unknownCount),
			let publicationID = try? container.decode(String.self, forKey: .publicationID), !publicationID.isEmpty,
			let rawTime = try? container.decode(String.self, forKey: .generatedAt),
			let generatedAt = WidgetTimelinePolicy.date(rawTime)
		else { return nil }
		let unknownCount: Int?
		if (try? container.decodeNil(forKey: .unknownCount)) == true {
			unknownCount = nil
		} else if let value = try? container.decode(Int.self, forKey: .unknownCount), value >= 0, value <= count {
			unknownCount = value
		} else {
			return nil
		}
		// Auxiliary holds only confirmed auxiliary sessions.
		if scope == .auxiliary, unknownCount != 0 { return nil }
		return Value(scope: scope, count: count, unknownCount: unknownCount, publicationID: publicationID, generatedAt: generatedAt)
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

enum WidgetUnreadableCategory: String, Equatable, Sendable {
	case io
	case decode
	case oversized
	case unsafeFile
}

enum WidgetLoadFailure: Error, Equatable, Sendable {
	case missing
	case containerUnavailable
	case unreadable(category: WidgetUnreadableCategory)
	case unsupportedSchemaVersion(found: Int)
}

enum WidgetLoadOutcome: Equatable, Sendable {
	case placeholder
	case loaded(WidgetDesktopSnapshotV1)
	case failed(WidgetLoadFailure)
}

private struct WidgetSnapshotSchemaHeader: Decodable {
	let schemaVersion: Int

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
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
		let data: Data
		do {
			data = try AppGroupSnapshotBytes.readBounded(at: url)
		} catch let error as AppGroupSnapshotByteReadError {
			switch error {
			case .missing: throw WidgetLoadFailure.missing
			case .unsafeFile: throw WidgetLoadFailure.unreadable(category: .unsafeFile)
			case .oversized: throw WidgetLoadFailure.unreadable(category: .oversized)
			case .unreadable: throw WidgetLoadFailure.unreadable(category: .io)
			}
		} catch {
			throw WidgetLoadFailure.unreadable(category: .io)
		}

		let header: WidgetSnapshotSchemaHeader
		do {
			header = try JSONDecoder().decode(WidgetSnapshotSchemaHeader.self, from: data)
		} catch {
			throw WidgetLoadFailure.unreadable(category: .decode)
		}
		guard header.schemaVersion == WidgetDesktopSnapshotV1.schemaVersion else {
			throw WidgetLoadFailure.unsupportedSchemaVersion(found: header.schemaVersion)
		}
		do {
			return try JSONDecoder().decode(WidgetDesktopSnapshotV1.self, from: data)
		} catch {
			throw WidgetLoadFailure.unreadable(category: .decode)
		}
	}
}
