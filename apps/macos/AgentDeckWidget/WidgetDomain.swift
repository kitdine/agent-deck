import Foundation
import SwiftUI
import WidgetKit

enum AgentDeckWidgetKind: String, CaseIterable, Codable, Sendable {
	case magnitude
	case composition
	case trust
	case rhythm

	var titleKey: String {
		switch self {
		case .magnitude: "Magnitude"
		case .composition: "Composition"
		case .trust: "Trust"
		case .rhythm: "Rhythm"
		}
	}

	var emptyKey: String {
		switch self {
		case .magnitude: "No spend in this period"
		case .composition: "No model usage in this period"
		case .trust: "No attribution data"
		case .rhythm: "No activity in the last 30 days"
		}
	}
}

enum WidgetSurfaceState: Equatable, Sendable {
	case placeholder
	case data
	case unavailable
}

enum WidgetQualifier: String, CaseIterable, Equatable, Sendable {
	case partial
	case aging
	case old
	case empty

	var titleKey: String {
		switch self {
		case .partial: "Some data unavailable"
		case .aging: "Updated over 15 minutes ago"
		case .old: "Updated over 6 hours ago"
		case .empty: "No activity"
		}
	}
}

struct WidgetFooterPresentation: Equatable {
	let updateText: String
	let qualifierText: String
	let isOld: Bool

	init(qualifiers: [WidgetQualifier], relativeTime: String?, bundle: Bundle? = nil) {
		isOld = qualifiers.contains(.old)
		if let relativeTime {
			updateText = WidgetCopy.format(
				isOld ? "Last updated %@" : "Updated %@",
				value: relativeTime,
				bundle: bundle
			)
		} else {
			updateText = WidgetCopy.text("Updated now", bundle: bundle)
		}
		qualifierText = qualifiers
			.filter { $0 != .aging && $0 != .old }
			.map { WidgetCopy.text($0.titleKey, bundle: bundle) }
			.joined(separator: " · ")
	}
}

struct AgentDeckWidgetEntry: TimelineEntry {
	let date: Date
	let snapshot: WidgetDesktopSnapshotV1?
	let kind: AgentDeckWidgetKind
	let client: WidgetClient
	let period: WidgetPeriod
	let isPlaceholder: Bool
}

struct WidgetSurfaceModel {
	let entry: AgentDeckWidgetEntry
	let now: Date

	var surface: WidgetSurfaceState {
		if entry.isPlaceholder {
			return .placeholder
		}
		guard let snapshot = entry.snapshot,
			snapshot.schemaVersion == WidgetDesktopSnapshotV1.schemaVersion,
			snapshot.usage.presentation.available,
			scope != nil
		else {
			return .unavailable
		}
		return .data
	}

	var scope: DesktopUsageScopeV1? {
		entry.snapshot?.usage.presentation.scopes.first { $0.client == entry.client.rawValue }
	}

	var period: DesktopUsagePeriodV1? {
		let requested = entry.kind == .trust ? WidgetPeriod.today : entry.kind == .rhythm ? .thirtyDays : entry.period
		return scope?.periods.items.first { $0.period == requested.rawValue }
	}

	var qualifiers: [WidgetQualifier] {
		guard surface == .data, let snapshot = entry.snapshot else { return [] }
		var result = [WidgetQualifier]()
		if snapshot.partial {
			result.append(.partial)
		}
		if let generated = WidgetTimelinePolicy.date(snapshot.generatedAt) {
			let age = now.timeIntervalSince(generated)
			if age > 6 * 60 * 60 {
				result.append(.old)
			} else if age >= 15 * 60 {
				result.append(.aging)
			}
		}
		if isEmpty {
			result.append(.empty)
		}
		return result
	}

	var isEmpty: Bool {
		guard let scope else { return false }
		switch entry.kind {
		case .magnitude:
			guard let period else { return true }
			return period.totals.tokens == 0 && period.totals.sessions == 0
		case .composition:
			guard let period else { return true }
			return period.models.isEmpty || period.models.allSatisfy { $0.value.tokens == 0 }
		case .trust:
			let current = scope.quality.items.filter { $0.period == WidgetPeriod.today.rawValue }
			return current.isEmpty || current.flatMap(\.tiers).allSatisfy { $0.value.tokens == 0 }
		case .rhythm:
			return !scope.rhythm.available || scope.rhythm.activeDays == 0 || scope.rhythm.intensities.allSatisfy { $0 == 0 }
		}
	}

	var chartValues: [Double] {
		guard let scope else { return [] }
		if entry.kind == .rhythm {
			return scope.rhythm.intensities.map(Double.init)
		}
		return scope.daily.items.map { WidgetFormat.chartValue($0.value) }
	}

}

enum WidgetLayoutContract {
	static func presentationFamily(_ family: WidgetFamily, dynamicTypeSize: DynamicTypeSize) -> WidgetFamily {
		guard dynamicTypeSize == .accessibility5 else { return family }
		switch family {
		case .systemLarge: return .systemMedium
		case .systemMedium: return .systemSmall
		default: return family
		}
	}

	static func canvas(_ family: WidgetFamily) -> CGSize {
		switch family {
		case .systemMedium: CGSize(width: 338, height: 155)
		case .systemLarge: CGSize(width: 338, height: 354)
		default: CGSize(width: 155, height: 155)
		}
	}

	/// The prototype defines each family as its own composition. Medium is not a
	/// stretched small Widget, and large is not a vertically padded medium one.
	static func sections(_ kind: AgentDeckWidgetKind, family: WidgetFamily) -> [String] {
		switch (kind, family) {
		case (.magnitude, .systemMedium): ["periods", "mini-bars", "date-axis"]
		case (.magnitude, .systemLarge): ["headline", "volume", "periods", "area", "date-axis", "statistics"]
		case (.magnitude, _): ["headline", "volume", "mini-bars"]
		case (.composition, .systemMedium): ["models", "share-tracks"]
		case (.composition, .systemLarge): ["models", "share-tracks", "token-mix", "clients"]
		case (.composition, _): ["eyebrow", "top-model", "headline-share", "share-track", "volume"]
		case (.trust, .systemMedium): ["quality", "share-tracks", "coverage"]
		case (.trust, .systemLarge): ["eyebrow", "headline", "quality", "providers", "pricing-summary"]
		case (.trust, _): ["eyebrow", "headline", "share-track", "support"]
		case (.rhythm, .systemMedium): ["hour-axis", "legend", "hour-grid"]
		case (.rhythm, .systemLarge): ["legend", "hour-axis", "hour-grid", "daily-grid", "day-statistics"]
		case (.rhythm, _): ["eyebrow", "active-days", "busiest"]
		}
	}

	static func sections(_ kind: AgentDeckWidgetKind) -> [String] {
		sections(kind, family: .systemLarge)
	}

	static func depth(_ kind: AgentDeckWidgetKind, family: WidgetFamily) -> Int {
		sections(kind, family: family).count
	}

	static func bucketCount(_ family: WidgetFamily) -> Int {
		switch family {
		case .systemMedium: 20
		case .systemLarge: 90
		default: 7
		}
	}
}

enum WidgetFormat {
	static func decimal(_ value: String?) -> Double {
		guard let value, let parsed = Double(value), parsed.isFinite else { return 0 }
		return parsed
	}

	static func cost(_ current: String?, known: String, incomplete: Bool) -> String {
		let value = decimal(current ?? known)
		let prefix = incomplete ? "≈" : ""
		if value >= 1_000 {
			return String(format: "%@$%.1fk", prefix, value / 1_000)
		}
		return String(format: "%@$%.2f", prefix, value)
	}

	static func tokens(_ value: Int64) -> String {
		switch abs(value) {
		case 1_000_000...: String(format: "%.1fM", Double(value) / 1_000_000)
		case 1_000...: String(format: "%.1fk", Double(value) / 1_000)
		default: String(value)
		}
	}

	static func share(_ value: String?) -> String {
		guard let value else { return "—" }
		let parsed = decimal(value)
		if abs(parsed.rounded() - parsed) < 0.05 {
			return String(format: "%.0f%%", parsed)
		}
		return String(format: "%.1f%%", parsed)
	}

	static func percentage(_ value: Int64, total: Int64) -> String? {
		guard total > 0 else { return nil }
		return String(format: "%.2f", Double(value) * 100 / Double(total))
	}

	static func chartValue(_ value: DesktopPresentationValueV1) -> Double {
		value.costIncomplete ? Double(value.tokens) : decimal(value.providerCost)
	}

	static func date(_ value: String) -> String {
		let parser = DateFormatter()
		parser.locale = Locale(identifier: "en_US_POSIX")
		parser.calendar = Calendar(identifier: .gregorian)
		parser.dateFormat = "yyyy-MM-dd"
		guard let date = parser.date(from: value) else { return value }
		let formatter = DateFormatter()
		formatter.locale = .current
		formatter.setLocalizedDateFormatFromTemplate("MMM d")
		return formatter.string(from: date)
	}

	static func weekday(_ canonical: String, short: Bool = false) -> String {
		let weekdays = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
		guard let index = weekdays.firstIndex(of: canonical.lowercased()) else { return canonical.capitalized }
		let formatter = DateFormatter()
		formatter.locale = .current
		guard let symbols = short ? formatter.veryShortStandaloneWeekdaySymbols : formatter.standaloneWeekdaySymbols,
			symbols.count == 7
		else {
			return canonical.capitalized
		}
		return symbols[(index + 1) % 7]
	}

	static func hourRange(start: Int, end: Int) -> String {
		let suffix = Locale.current.language.languageCode?.identifier == "zh" ? "时" : "h"
		return "\(start)–\(end)\(suffix)"
	}
}

/// A tier amount that never presents an unpriced zero as a known zero.
enum WidgetTrustAmount: Equatable, Sendable {
	case cost(String)
	case tokens(String)
}

struct WidgetTrustTier: Identifiable, Equatable, Sendable {
	var id: String { quality }
	let quality: String
	let amount: WidgetTrustAmount
	let share: String?
	let costIncomplete: Bool

	init(tier: DesktopUsageQualityTierV1) {
		quality = tier.quality
		share = tier.share
		costIncomplete = tier.value.costIncomplete
		// An incomplete cost is unknown, not zero, so the projected token amount
		// is the only figure this tier can honestly state.
		amount = tier.value.costIncomplete
			? .tokens(WidgetFormat.tokens(tier.value.tokens))
			: .cost(WidgetFormat.cost(tier.value.providerCost, known: tier.value.providerCost, incomplete: false))
	}
}

struct WidgetTrustProviderRow: Identifiable, Equatable, Sendable {
	var id: String { provider }
	let provider: String
	let tier: WidgetTrustTier
}

struct WidgetTrustPricing: Equatable, Sendable {
	let coverage: String
	let incomplete: Bool
	let unpricedIdentifiers: [String]

	init(item: DesktopUsagePricingItemV1) {
		coverage = item.coverage
		incomplete = item.unpricedEvents > 0
		unpricedIdentifiers = item.unpricedIdentifiers
	}
}

extension WidgetSurfaceModel {
	var trustTiers: [WidgetTrustTier] {
		let aggregate = scope?.quality.items.first {
			$0.period == WidgetPeriod.today.rawValue && $0.provider == nil
		}
		return (aggregate?.tiers ?? []).map(WidgetTrustTier.init(tier:))
	}

	var trustHeadline: WidgetTrustTier? {
		trustTiers.first { $0.quality == "determinable" }
	}

	var trustProviders: [WidgetTrustProviderRow] {
		let items = scope?.quality.items.filter {
			$0.period == WidgetPeriod.today.rawValue && $0.provider != nil
		} ?? []
		return items.compactMap { item in
			guard let provider = item.provider else { return nil }
			let tiers = item.tiers.map(WidgetTrustTier.init(tier:))
			guard let tier = tiers.first(where: { $0.quality == "determinable" }) ?? tiers.first else {
				return nil
			}
			return WidgetTrustProviderRow(provider: provider, tier: tier)
		}
	}

	var trustPricing: WidgetTrustPricing? {
		scope?.pricing.items
			.first { $0.period == WidgetPeriod.today.rawValue }
			.map(WidgetTrustPricing.init(item:))
	}

	/// True when today's attribution or pricing leaves a monetary figure unknown.
	var trustCostIncomplete: Bool {
		trustTiers.contains { $0.costIncomplete } || trustPricing?.incomplete == true
	}
}
