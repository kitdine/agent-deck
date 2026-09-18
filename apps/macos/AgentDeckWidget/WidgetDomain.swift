import Foundation
import SwiftUI
import WidgetKit

enum AgentDeckWidgetKind: String, CaseIterable, Codable, Sendable {
	case magnitude
	case composition
	case trust
	case rhythm
	case quota

	var titleKey: String {
		switch self {
		case .magnitude: "Magnitude"
		case .composition: "Composition"
		case .trust: "Trust"
		case .rhythm: "Rhythm"
		case .quota: "Quota"
		}
	}

	var emptyKey: String {
		switch self {
		case .magnitude: "No spend in this period"
		case .composition: "No model usage in this period"
		case .trust: "No attribution data"
		case .rhythm: "No activity in the last 30 days"
		case .quota: "No quota window to show"
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

	init(qualifiers: [WidgetQualifier], relativeTime: String?, unavailableText: String? = nil, bundle: Bundle? = nil) {
		isOld = qualifiers.contains(.old)
		if let unavailableText {
			updateText = unavailableText
		} else if let relativeTime {
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
			snapshot.schemaVersion == WidgetDesktopSnapshotV1.schemaVersion
		else {
			return .unavailable
		}
		if entry.kind == .quota { return snapshot.subscription.available ? .data : .unavailable }
		guard snapshot.usage.presentation.available, scope != nil else { return .unavailable }
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
		qualifiers(family: .systemLarge)
	}

	func qualifiers(family: WidgetFamily) -> [WidgetQualifier] {
		guard surface == .data, let snapshot = entry.snapshot else { return [] }
		var result = [WidgetQualifier]()
		// Codex PR #5 sixth review, P1: a shown client's latest probe can fail
		// while its prior windows are retained and displayed (C9); quotaFooterReason
		// alone stays silent whenever there is still a freshness instant to show,
		// so the failure needs its own visible qualifier rather than being
		// suppressed by the retained figures. .partial's existing "Some data
		// unavailable" wording already fits this: the *current* reading, not the
		// displayed figures themselves, is what is missing.
		let quotaHasFailure = entry.kind == .quota && presentedQuotaClients(family: family).contains { $0.failure != nil }
		if snapshot.partial || quotaHasFailure {
			result.append(.partial)
		}
		if entry.kind == .quota {
			// Codex PR #5 eleventh review, P2: quota clients already carry a
			// window-aware `stale` flag (a five-hour window at the default
			// interval is stale after 30 minutes, per architecture.md C9),
			// distinct from the generic snapshot-wide six-hour/fifteen-minute
			// cutoffs below. Deriving quota's freshness qualifier from that
			// generic ladder instead disagreed with the same client's own
			// `stale` reading on the CLI and menu bar.
			if presentedQuotaClients(family: family).contains(where: \.stale) {
				result.append(.old)
			}
		} else if let generated = WidgetTimelinePolicy.date(snapshot.generatedAt) {
			let age = now.timeIntervalSince(generated)
			if age > 6 * 60 * 60 {
				result.append(.old)
			} else if age >= 15 * 60 {
				result.append(.aging)
			}
		}
		if isEmpty(family: family) {
			result.append(.empty)
		}
		return result
	}

	// Codex PR #5 eighth review, P2: the quota branch must derive emptiness
	// from the clients actually presented for family, not from
	// quotaClients, which stays narrowed to the configured single client
	// even on the large family, where presentedQuotaClients shows both. A
	// configured client with no windows alongside a client that does have
	// them was rendering real figures while also appending the "No
	// activity" qualifier for the other, unpresented client's emptiness.
	func isEmpty(family: WidgetFamily) -> Bool {
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
		case .quota:
			return presentedQuotaClients(family: family).allSatisfy { $0.windows.isEmpty }
		}
	}

	var quotaClients: [DesktopSubscriptionClientV1] {
		guard let subscription = entry.snapshot?.subscription else { return [] }
		if entry.client == .all { return subscription.clients }
		return subscription.clients.filter { $0.client == entry.client.rawValue }
	}

	func presentedQuotaClients(family: WidgetFamily) -> [DesktopSubscriptionClientV1] {
		guard let all = entry.snapshot?.subscription.clients else { return [] }
		if family == .systemLarge {
			// The wire reports one explicit state per client. A client with no
			// windows is still meaningful (not official, never probed, parse
			// failed, reading off, and so on), so preserve it for the large
			// widget's per-client reason instead of silently dropping the card.
			return Array(all.prefix(2))
		}
		return Array(quotaClients.prefix(1))
	}

	/// The header's scope label for a quota widget. Small and medium narrow
	/// an `.all`-configured widget down to one client (presentedQuotaClients'
	/// own selection), so the header must name that client rather than
	/// keep claiming "All clients" while showing only one of them. Large
	/// keeps the configured client unchanged: it labels each client inside
	/// its own per-client block instead.
	func quotaScopeClient(family: WidgetFamily) -> WidgetClient {
		guard family != .systemLarge, entry.client == .all,
			let shown = presentedQuotaClients(family: family).first ?? quotaClients.first,
			let resolved = WidgetClient(rawValue: shown.client)
		else {
			return entry.client
		}
		return resolved
	}

	/// The oldest observation among the windows actually displayed -- not
	/// the client-level observed_at, which BuildSubscription derives from
	/// the newest window or envelope and can therefore be fresher than a
	/// displayed window after a partial update leaves the client's windows
	/// at different ages.
	func quotaFooterObservedAt(family: WidgetFamily) -> String? {
		presentedQuotaClients(family: family)
			.flatMap { quotaWindows(for: $0, family: family) }
			.compactMap(\.observedAt)
			.compactMap { value in WidgetTimelinePolicy.date(value).map { (value, $0) } }
			.min { $0.1 < $1.1 }?.0
	}

	func quotaFooterReason(family: WidgetFamily) -> DesktopQuotaReasonV1? {
		let shown = presentedQuotaClients(family: family)
		guard quotaFooterObservedAt(family: family) == nil else { return nil }
		return shown.compactMap { !$0.applicable ? ($0.applicableReason ?? .notOfficial) : $0.failure }.first ?? .neverProbed
	}

	func quotaWindows(for client: DesktopSubscriptionClientV1, family: WidgetFamily) -> [DesktopSubscriptionWindowV1] {
		if family == .systemSmall {
			// The wire resolves the tightest window once, from vendor order
			// (C11); recomputing a tie-break here by key could disagree with
			// that producer-selected choice whenever two windows share the
			// highest percentage and their key order differs from vendor
			// order. Honor tightestWindowKey when it names one of this
			// client's windows; fall back to the local tie-break only when
			// it does not (defensive, not expected in practice).
			if let key = client.tightestWindowKey, let tightest = client.windows.first(where: { $0.key == key }) {
				return [tightest]
			}
			return Array(client.windows.sorted { lhs, rhs in
				lhs.usedPercent == rhs.usedPercent ? lhs.key < rhs.key : lhs.usedPercent > rhs.usedPercent
			}.prefix(1))
		}
		if family == .systemLarge {
			return Array(client.windows.prefix(4))
		}
		// Codex PR #5 eleventh review, P2: the Codex adapter deliberately
		// supports an arbitrary window count, and the documented Codex Plus
		// presentation alone already has four -- a fixed three-row cap here
		// silently dropped the fourth (often the limiting) bucket even in
		// that base case. Render every reported window instead of truncating.
		return client.windows
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
		case (.quota, .systemMedium): ["client", "windows", "attribution"]
		case (.quota, .systemLarge): ["codex", "claude", "windows", "attribution"]
		case (.quota, _): ["client", "tightest-window", "attribution"]
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

enum QuotaWidgetAxis: Equatable { case single, vertical }

struct QuotaWidgetLayoutContract: Equatable {
	let axis: QuotaWidgetAxis
	let equalHeightSlots: Int

	static func presentation(family: WidgetFamily, clientCount: Int) -> Self {
		guard family == .systemLarge, clientCount > 1 else {
			return Self(axis: .single, equalHeightSlots: 1)
		}
		return Self(axis: .vertical, equalHeightSlots: clientCount)
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
