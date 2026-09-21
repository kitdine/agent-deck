import Foundation

public enum AppGroupWidgetKind: String, CaseIterable, Codable, Hashable, Sendable {
	case magnitude
	case composition
	case trust
	case rhythm
	case quota

	public var widgetKitKindIdentifier: String {
		"com.kitdine.agentdeck.widget.\(rawValue)"
	}
}

public struct WidgetSemanticProjection: Equatable, Sendable {
	private let values: [AppGroupWidgetKind: Data]

	public init(snapshot: AppGroupDesktopSnapshotV1) throws {
		let encoder = JSONEncoder()
		encoder.outputFormatting = [.sortedKeys]
		let data = try encoder.encode(snapshot)
		guard let root = try JSONSerialization.jsonObject(with: data) as? [String: Any],
			let usage = root["usage"] as? [String: Any],
			let presentation = usage["presentation"] as? [String: Any]
		else {
			throw WidgetSemanticProjectionError.invalidSnapshot
		}

		var projections = [AppGroupWidgetKind: Data]()
		for kind in AppGroupWidgetKind.allCases {
			let object = try Self.semanticObject(
				kind: kind,
				root: root,
				usage: usage,
				presentation: presentation
			)
			projections[kind] = try JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
		}
		values = projections
	}

	public func affectedKinds(comparedTo previous: WidgetSemanticProjection) -> Set<AppGroupWidgetKind> {
		Set(AppGroupWidgetKind.allCases.filter { values[$0] != previous.values[$0] })
	}

	private static func semanticObject(
		kind: AppGroupWidgetKind,
		root: [String: Any],
		usage: [String: Any],
		presentation: [String: Any]
	) throws -> [String: Any] {
		let base: [String: Any] = [
			"schema_version": root["schema_version"] ?? NSNull(),
			"partial": root["partial"] ?? NSNull(),
		]
		if kind == .quota {
			return base.merging(["subscription": root["subscription"] ?? NSNull()]) { _, new in new }
		}

		let scopes = presentation["scopes"] as? [[String: Any]] ?? []
		let semanticScopes: [[String: Any]]
		switch kind {
		case .magnitude:
			semanticScopes = scopes.map { scope in
				var result = selected(scope, keys: ["client", "daily"])
				result["periods"] = periodView(scope["periods"], itemKeys: ["period", "totals", "average_per_day", "peak", "cache_hit_share"])
				return result
			}
		case .composition:
			semanticScopes = scopes.map { scope in
				var result = selected(scope, keys: ["client"])
				result["periods"] = periodView(scope["periods"], itemKeys: ["period", "models", "totals"])
				return result
			}
		case .trust:
			semanticScopes = scopes.map { selected($0, keys: ["client", "quality", "pricing"]) }
		case .rhythm:
			semanticScopes = scopes.map { selected($0, keys: ["client", "daily", "hourly", "rhythm"]) }
		case .quota:
			semanticScopes = []
		}

		var semanticPresentation: [String: Any] = [
			"available": presentation["available"] ?? NSNull(),
			"scopes": semanticScopes,
		]
		if kind == .composition {
			semanticPresentation["client_subtotals"] = presentation["client_subtotals"] ?? NSNull()
		}
		let semanticUsage: [String: Any] = [
			"available": usage["available"] ?? NSNull(),
			"presentation": semanticPresentation,
		]
		return base.merging(["usage": semanticUsage]) { _, new in new }
	}

	private static func selected(_ object: [String: Any], keys: Set<String>) -> [String: Any] {
		Dictionary(uniqueKeysWithValues: keys.compactMap { key in
			object[key].map { (key, $0) }
		})
	}

	private static func periodView(_ raw: Any?, itemKeys: Set<String>) -> [String: Any] {
		guard let periods = raw as? [String: Any] else { return [:] }
		var result = selected(periods, keys: ["available"])
		result["items"] = (periods["items"] as? [[String: Any]] ?? []).map {
			selected($0, keys: itemKeys)
		}
		return result
	}
}

public enum WidgetSemanticProjectionError: Error, Equatable, Sendable {
	case invalidSnapshot
}
