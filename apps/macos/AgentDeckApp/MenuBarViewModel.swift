import AgentDeckShared
import Foundation
import Observation

enum MenuBarPanel: String, CaseIterable, Identifiable, Sendable {
	case usage
	case breakdown
	case attribution
	case sessions

	var id: String { rawValue }

	var title: String {
		switch self {
		case .usage: t(DesktopCopy.panelUsage)
		case .breakdown: t(DesktopCopy.panelBreakdown)
		case .attribution: t(DesktopCopy.panelAttribution)
		case .sessions: t(DesktopCopy.panelSessions)
		}
	}

	var symbol: String {
		switch self {
		case .usage: "chart.line.uptrend.xyaxis"
		case .breakdown: "chart.pie"
		case .attribution: "checkmark.shield"
		case .sessions: "chevron.left.forwardslash.chevron.right"
		}
	}
}

enum NoticeSeverity: Equatable, Sendable {
	case error
	case warning

	var symbol: String {
		switch self {
		case .error: "xmark.octagon"
		case .warning: "exclamationmark.triangle"
		}
	}
}

struct MenuBarNotice: Identifiable, Equatable, Sendable {
	let id: String
	let text: String
	let severity: NoticeSeverity
	let opensHealthDetail: Bool
}

struct FilterOption: Identifiable, Equatable, Sendable {
	let id: String
	let label: String
	let value: String?
}

struct PanelTab: Identifiable, Equatable, Sendable {
	let id: MenuBarPanel
	let title: String
	let symbol: String
	let marked: Bool
	let accessibilityLabel: String
}

struct MenuBarHero: Equatable, Sendable {
	let scopeLine: String
	let amount: String
	let tokens: String
	let counts: String
	let costIncomplete: String?
}

struct StatChip: Identifiable, Equatable, Sendable {
	let id: String
	let label: String
	let value: String
	let note: String?
}

struct TrendBucket: Identifiable, Equatable, Sendable {
	let id: String
	let label: String
	let tokens: Int64
	let events: Int64
	let cost: String
	let magnitude: Double
	let inWindow: Bool
	let accessibilityValue: String
}

struct ShareRow: Identifiable, Equatable, Sendable {
	let id: String
	let label: String
	let value: String
	let share: Double
	let shareText: String
}

struct UsagePanelModel: Equatable, Sendable {
	let available: Bool
	let windowLabel: String
	let buckets: [TrendBucket]
	let chips: [StatChip]
	let emptyCopy: String?
}

struct BreakdownPanelModel: Equatable, Sendable {
	let available: Bool
	let models: [ShareRow]
	let modelsEmptyCopy: String?
	let tokenMix: [ShareRow]
	let clientRows: [ShareRow]
}

struct ProviderQualityGroup: Identifiable, Equatable, Sendable {
	let id: String
	let title: String
	let tiers: [ShareRow]
}

struct AttributionPanelModel: Equatable, Sendable {
	let qualityAvailable: Bool
	let tiers: [ShareRow]
	let providerGroups: [ProviderQualityGroup]
	let pricingAvailable: Bool
	let pricingHeadline: String
	let pricingCoverage: Double
	let unpricedIdentifiers: [String]
}

struct RecentSessionRow: Identifiable, Equatable, Sendable {
	let id: String
	let title: String
	let detail: String
	let when: String
}

struct SessionsPanelModel: Equatable, Sendable {
	let available: Bool
	let stats: [StatChip]
	let projects: [ShareRow]
	let recent: [RecentSessionRow]
	let emptyCopy: String?
}

struct RhythmCell: Identifiable, Equatable, Sendable {
	let id: String
	let weekday: Int
	let hour: Int
	let intensity: Int
	let tokens: Int64
	let cost: String
	let accessibilityLabel: String
	let accessibilityValue: String
}

struct RhythmBlockModel: Equatable, Sendable {
	let available: Bool
	let scopeLine: String
	let figures: [StatChip]
	let cells: [RhythmCell]
	let calendar: [TrendBucket]
}

struct SwitchTargetChoice: Identifiable, Equatable, Sendable {
	let id: String
	let label: String
	let target: ProviderSwitchTarget
}

struct SwitchOptionRow: Identifiable, Equatable, Sendable {
	let id: String
	let label: String
	let detail: String?
	let enabled: Bool
	let isCurrent: Bool
	let target: ProviderSwitchTarget?
	let choices: [SwitchTargetChoice]
}

struct SwitchClientSection: Identifiable, Equatable, Sendable {
	let id: String
	let title: String
	let rows: [SwitchOptionRow]
}

struct FooterModel: Equatable, Sendable {
	let routesText: String
	let sections: [SwitchClientSection]
	let switchingAvailable: Bool
}

enum SwitchPresentation: Equatable, Sendable {
	case none
	case confirming(target: ProviderSwitchTarget, message: String)
	case inFlight(message: String)
	case failed(message: String, detail: String)
	case indeterminate(message: String, detail: String)
	case succeeded(message: String)

	var blocksSurface: Bool {
		if case .inFlight = self { return true }
		return false
	}
}

struct HealthCheckRow: Identifiable, Equatable, Sendable {
	let id: String
	let name: String
	let status: String
	let severity: NoticeSeverity?
	let recovery: String?
}

struct HealthDetailModel: Equatable, Sendable {
	let rows: [HealthCheckRow]
	let source: String
}

/// The only place coordinator state becomes presentation values. It owns no
/// refresh state machine, executes no process, decodes no wire, and never
/// touches the App Group cache; views read this and compute nothing themselves.
@MainActor
@Observable
final class MenuBarViewModel {
	let coordinator: DesktopRefreshCoordinator
	let switchController: SwitchController
	let preferences: DesktopPreferences

	var selectedClient = "all"
	var selectedPeriod = "today"
	var selectedPanel: MenuBarPanel = .usage
	var showsHealthDetail = false
	var pendingConfirmation: ProviderSwitchTarget?
	private(set) var collapsedSectionIDs = Set<String>()

	@ObservationIgnored private let now: () -> Date

	init(
		coordinator: DesktopRefreshCoordinator,
		switchController: SwitchController,
		preferences: DesktopPreferences,
		now: @escaping () -> Date = Date.init
	) {
		self.coordinator = coordinator
		self.switchController = switchController
		self.preferences = preferences
		self.now = now
	}

	// MARK: Surface and qualifiers

	var presentation: DesktopPresentationState {
		DesktopPresentationState.derive(from: coordinator.state, now: now())
	}

	var surface: DesktopPresentationSurface { presentation.surface }

	var qualifiers: [DesktopPresentationQualifier] { presentation.qualifiers }

	var isRefreshing: Bool {
		if case .refreshing = coordinator.state {
			return true
		}
		return false
	}

	var snapshot: DesktopSnapshotV1? { presentation.snapshot?.data }

	/// `aged` replaces `stale`'s wording rather than adding a second freshness
	/// statement, and the relative time is the snapshot's own `generated_at`.
	var freshnessText: String? {
		guard let generatedAt = presentation.snapshot?.data.generatedAt else { return nil }
		let relative = DesktopFormat.relative(generatedAt, now: now())
		return qualifiers.contains(.aged)
			? t(DesktopCopy.freshnessLastUpdated, relative)
			: t(DesktopCopy.freshnessUpdated, relative)
	}

	/// The same words the user reads, in the fixed qualifier order, so an
	/// assistive reader is told what a sighted reader is told.
	var qualifierSummary: String? {
		var parts = [String]()
		if qualifiers.contains(.stale) || qualifiers.contains(.aged), let freshnessText {
			parts.append(freshnessText)
		}
		if qualifiers.contains(.offline) { parts.append(t(DesktopCopy.offline)) }
		if qualifiers.contains(.failing) { parts.append(t(DesktopCopy.failing)) }
		if qualifiers.contains(.partial) { parts.append(t(DesktopCopy.partial)) }
		if qualifiers.contains(.empty) { parts.append(emptyCopy) }
		guard !parts.isEmpty else { return nil }
		return t(DesktopCopy.qualifierList, parts.joined(separator: " · "))
	}

	/// `empty` describes the day only on a current, issue-free surface; with any
	/// freshness or reachability qualifier it describes the snapshot instead.
	var emptyCopy: String {
		let qualified = qualifiers.contains(.stale) || qualifiers.contains(.aged)
			|| qualifiers.contains(.offline) || qualifiers.contains(.failing)
		return qualified ? t(DesktopCopy.emptySnapshot) : t(DesktopCopy.emptyToday)
	}

	var errorCopy: String {
		if case .degraded(_, .helper(.timedOut)) = coordinator.state {
			return t(DesktopCopy.refreshTimedOut)
		}
		return qualifiers.contains(.failing) ? t(DesktopCopy.failing) : t(DesktopCopy.offline)
	}

	// MARK: Notice strip

	var notices: [MenuBarNotice] {
		guard let envelope = presentation.snapshot else { return [] }
		var result = [MenuBarNotice]()
		if qualifiers.contains(.offline) {
			result.append(MenuBarNotice(id: "offline", text: t(DesktopCopy.offline), severity: .error, opensHealthDetail: false))
		} else if qualifiers.contains(.failing) {
			result.append(MenuBarNotice(id: "failing", text: errorCopy, severity: .error, opensHealthDetail: false))
		}
		if qualifiers.contains(.partial) {
			result.append(MenuBarNotice(id: "partial", text: t(DesktopCopy.partial), severity: .warning, opensHealthDetail: false))
		}
		let health = envelope.data.health
		if health.available, health.problems > 0 {
			result.append(
				MenuBarNotice(
					id: "health",
					text: t(DesktopCopy.healthNotice, Int64(health.problems)),
					severity: health.errors > 0 ? .error : .warning,
					opensHealthDetail: true
				)
			)
		}
		let warnings = envelope.warnings
		for code in warnings.prefix(3) {
			result.append(MenuBarNotice(id: "warning.\(code)", text: warningCopy(code), severity: .warning, opensHealthDetail: false))
		}
		if warnings.count > 3 {
			result.append(
				MenuBarNotice(
					id: "warning.more",
					text: t(DesktopCopy.noticeMore, Int64(warnings.count - 3)),
					severity: .warning,
					opensHealthDetail: true
				)
			)
		}
		return result
	}

	/// An unrecognized code is shown verbatim rather than dropped.
	func warningCopy(_ code: String) -> String {
		switch code {
		case "provider_unavailable": t(DesktopCopy.warningProviderUnavailable)
		case "provider_candidates_unavailable": t(DesktopCopy.warningProviderCandidatesUnavailable)
		case "usage_unavailable": t(DesktopCopy.warningUsageUnavailable)
		case "sessions_unavailable": t(DesktopCopy.warningSessionsUnavailable)
		case "health_unavailable": t(DesktopCopy.warningHealthUnavailable)
		case "state_close_failed": t(DesktopCopy.warningStateCloseFailed)
		case "sessions_close_failed": t(DesktopCopy.warningSessionsCloseFailed)
		default: t(DesktopCopy.warningUnknown, code)
		}
	}

	// MARK: Filters

	var clientTabs: [FilterOption] {
		guard let snapshot else { return [] }
		let subtotals = snapshot.usage.presentation.clientSubtotals
		return snapshot.usage.presentation.scopes.map { scope in
			let subtotal = subtotals.available
				? subtotals.items.first(where: { $0.period == selectedPeriod && $0.client == scope.client })?.value
				: nil
			let totals = scope.periods.items.first(where: { $0.period == selectedPeriod })?.totals
			return FilterOption(
				id: scope.client,
				label: clientLabel(scope.client),
				value: subtotal.map(costText) ?? totals.map(costText) ?? "—"
			)
		}
	}

	var periodOptions: [FilterOption] {
		guard let scope = activeScope else { return [] }
		return scope.periods.items.map { FilterOption(id: $0.period, label: periodLabel($0.period), value: nil) }
	}

	var panelTabs: [PanelTab] {
		MenuBarPanel.allCases.map { panel in
			let marked = panelHasUnavailableData(panel)
			return PanelTab(
				id: panel,
				title: panel.title,
				symbol: panel.symbol,
				marked: marked,
				accessibilityLabel: marked ? t(DesktopCopy.panelUnavailableMark, panel.title) : panel.title
			)
		}
	}

	private func panelHasUnavailableData(_ panel: MenuBarPanel) -> Bool {
		guard let snapshot else { return false }
		guard let scope = activeScope else { return true }
		switch panel {
		case .usage:
			return !scope.periods.available || !scope.daily.available
		case .breakdown:
			return !scope.periods.available || !snapshot.usage.presentation.clientSubtotals.available
		case .attribution:
			return !scope.quality.available || !scope.pricing.available
		case .sessions:
			return !snapshot.sessions.available || !snapshot.sessions.periods.available
		}
	}

	var activeScope: DesktopUsageScopeV1? {
		guard let snapshot else { return nil }
		let scopes = snapshot.usage.presentation.scopes
		return scopes.first(where: { $0.client == selectedClient })
			?? scopes.first(where: { $0.client == "all" })
			?? scopes.first
	}

	var activePeriod: DesktopUsagePeriodV1? {
		activeScope?.periods.items.first(where: { $0.period == selectedPeriod })
	}

	// MARK: Hero

	var hero: MenuBarHero? {
		guard let snapshot, let period = activePeriod else { return nil }
		let projects = sessionStatistics?.distinctProjects ?? 0
		return MenuBarHero(
			scopeLine: "\(periodLabel(period.period)) · \(DesktopFormat.day(snapshot.generatedAt))",
			amount: costText(period.totals),
			tokens: DesktopFormat.tokens(period.totals.tokens),
			counts: t(DesktopCopy.heroCounts, period.totals.events, period.totals.sessions, Int64(projects)),
			costIncomplete: period.totals.pricingComplete
				? nil
				: t(DesktopCopy.costIncomplete, Int64(period.totals.unpricedComponents))
		)
	}

	// MARK: Usage panel

	var usagePanel: UsagePanelModel {
		guard let scope = activeScope, let period = activePeriod, scope.periods.available else {
			return UsagePanelModel(available: false, windowLabel: "", buckets: [], chips: [], emptyCopy: nil)
		}
		let daily = Array(scope.daily.items.suffix(90))
		let window = periodWindow(scope: scope)
		let usesHourly = selectedPeriod == "today" && scope.hourly?.available == true
		let hourly = usesHourly ? scope.hourly?.items ?? [] : []
		let hourlyByHour = Dictionary(uniqueKeysWithValues: hourly.map { ($0.hour, $0) })
		let buckets = usesHourly
			? (0 ..< 24).map { hour in
				if let item = hourlyByHour[hour] {
					trendBucket(
						id: "hour.\(item.hour)",
						label: DesktopFormat.hourWindow(item.hour),
						value: item.value,
						inWindow: true
					)
				} else {
					emptyHourlyTrendBucket(hour: hour)
				}
			}
			: daily.filter { window.contains($0.date) }.map { item in
				trendBucket(
					id: item.date,
					label: DesktopFormat.compactDay(item.date),
					value: item.value,
					inWindow: true
				)
			}
		let isEmpty = period.totals.tokens == 0 && period.totals.events == 0
		return UsagePanelModel(
			available: scope.daily.available,
			windowLabel: usesHourly ? t(DesktopCopy.trendHours) : t(DesktopCopy.trendWindow, Int64(buckets.count)),
			buckets: buckets,
			chips: usageChips(period, hourly: hourly),
			emptyCopy: isEmpty ? emptyCopy : nil
		)
	}

	/// Two of the three chips are degenerate over a single day, so `today` shows
	/// a different set. The chart and chip receive the same decoder-validated
	/// hourly family; an unavailable legacy family cannot feed either one.
	private func usageChips(_ period: DesktopUsagePeriodV1, hourly: [DesktopUsageHourlyItemV1]) -> [StatChip] {
		if period.period == "today" {
			let peak = hourly.reduce(nil as DesktopUsageHourlyItemV1?) { selected, candidate in
				guard candidate.value.events > 0,
					let candidateCost = Double(candidate.value.providerCost)
				else {
					return selected
				}
				guard let selected,
					let selectedCost = Double(selected.value.providerCost)
				else {
					return candidate
				}
				if candidateCost != selectedCost {
					return candidateCost > selectedCost ? candidate : selected
				}
				return candidate.hour < selected.hour ? candidate : selected
			}
			return [
				StatChip(
					id: "priciest-hour",
					label: t(DesktopCopy.chipPriciestHour),
					value: peak.map { DesktopFormat.hourWindow($0.hour) } ?? "—",
					note: peak.map { costText($0.value) } ?? t(DesktopCopy.notCapturedYet)
				),
				StatChip(id: "events", label: t(DesktopCopy.chipEvents), value: DesktopFormat.count(period.totals.events), note: nil),
				StatChip(
					id: "cache-hit",
					label: t(DesktopCopy.chipCacheHit),
					value: DesktopFormat.percent(period.cacheHitShare),
					note: nil
				),
			]
		}
		return [
			StatChip(
				id: "average",
				label: t(DesktopCopy.chipAveragePerDay),
				value: DesktopFormat.decimalCompact(period.averagePerDay.tokens),
				note: nil
			),
			StatChip(
				id: "peak",
				label: t(DesktopCopy.chipPeak),
				value: DesktopFormat.tokens(period.peak.totals.tokens),
				note: period.peak.date.isEmpty ? nil : DesktopFormat.compactDay(period.peak.date)
			),
			StatChip(
				id: "cache-hit",
				label: t(DesktopCopy.chipCacheHit),
				value: DesktopFormat.percent(period.cacheHitShare),
				note: nil
			),
		]
	}

	// MARK: Breakdown panel

	var breakdownPanel: BreakdownPanelModel {
		guard let snapshot, let scope = activeScope, let period = activePeriod, scope.periods.available else {
			return BreakdownPanelModel(available: false, models: [], modelsEmptyCopy: nil, tokenMix: [], clientRows: [])
		}
		let models = period.models.prefix(12).enumerated().map { index, model in
			ShareRow(
				id: "model.\(index).\(model.model)",
				label: model.model,
				value: DesktopFormat.tokens(model.value.tokens),
				share: shareFraction(model.share),
				shareText: DesktopFormat.percent(model.share)
			)
		}
		let totals = period.totals
		let mixTotal = max(1, totals.inputTokens + totals.outputTokens + totals.cachedReadTokens + totals.cacheWriteTokens)
		let components: [(String, String, Int64)] = [
			("input", t(DesktopCopy.tokenInput), totals.inputTokens),
			("output", t(DesktopCopy.tokenOutput), totals.outputTokens),
			("cache-read", t(DesktopCopy.tokenCacheRead), totals.cachedReadTokens),
			("cache-write", t(DesktopCopy.tokenCacheWrite), totals.cacheWriteTokens),
		]
		let mix = components.map { identifier, label, value in
			let fraction = Double(value) / Double(mixTotal)
			return ShareRow(
				id: identifier,
				label: label,
				value: DesktopFormat.tokens(value),
				share: fraction,
				shareText: (fraction).formatted(.percent.precision(.fractionLength(0 ... 1)).locale(DesktopLocale.current))
			)
		}
		let subtotals = snapshot.usage.presentation.clientSubtotals
		let clientItems = subtotals.available
			? subtotals.items.filter { $0.period == selectedPeriod && $0.client != "all" }
			: []
		let clientTotal = max(Int64(1), clientItems.reduce(Int64(0)) { $0 + $1.value.tokens })
		let clientRows = clientItems.map { item in
			let fraction = Double(item.value.tokens) / Double(clientTotal)
			return ShareRow(
				id: "client.\(item.client)",
				label: clientLabel(item.client),
				value: costText(item.value),
				share: fraction,
				shareText: fraction.formatted(.percent.precision(.fractionLength(0 ... 1)).locale(DesktopLocale.current))
			)
		}
		return BreakdownPanelModel(
			available: true,
			models: Array(models),
			modelsEmptyCopy: models.isEmpty ? t(DesktopCopy.modelsEmpty) : nil,
			tokenMix: mix,
			clientRows: clientRows
		)
	}

	// MARK: Attribution panel

	var attributionPanel: AttributionPanelModel {
		guard let scope = activeScope else {
			return AttributionPanelModel(
				qualityAvailable: false, tiers: [], providerGroups: [],
				pricingAvailable: false, pricingHeadline: "", pricingCoverage: 0, unpricedIdentifiers: []
			)
		}
		let periodItems = scope.quality.items.filter { $0.period == selectedPeriod }
		let aggregate = periodItems.first(where: { $0.provider == nil })
		let tiers = (aggregate?.tiers ?? []).map { tier in
			ShareRow(
				id: "tier.\(tier.quality)",
				label: qualityLabel(tier.quality),
				value: costText(tier.value),
				share: shareFraction(tier.share),
				shareText: DesktopFormat.percent(tier.share)
			)
		}
		let groups = periodItems.compactMap { item -> ProviderQualityGroup? in
			guard let provider = item.provider else { return nil }
			return ProviderQualityGroup(
				id: "provider.\(provider)",
				title: provider,
				tiers: item.tiers.map { tier in
					ShareRow(
						id: "provider.\(provider).\(tier.quality)",
						label: qualityLabel(tier.quality),
						value: costText(tier.value),
						share: shareFraction(tier.share),
						shareText: DesktopFormat.percent(tier.share)
					)
				}
			)
		}
		let pricing = scope.pricing.items.first(where: { $0.period == selectedPeriod })
		let priced = pricing?.pricedEvents ?? 0
		let total = priced + (pricing?.unpricedEvents ?? 0)
		return AttributionPanelModel(
			qualityAvailable: scope.quality.available && aggregate != nil,
			tiers: tiers,
			providerGroups: groups,
			pricingAvailable: scope.pricing.available && pricing != nil,
			pricingHeadline: t(DesktopCopy.pricingPriced, priced, total),
			pricingCoverage: min(1, max(0, (Double(pricing?.coverage ?? "0") ?? 0) / 100)),
			unpricedIdentifiers: Array((pricing?.unpricedIdentifiers ?? []).prefix(12))
		)
	}

	// MARK: Sessions panel

	var sessionStatistics: DesktopSessionsPeriodItemV1? {
		guard let snapshot, snapshot.sessions.periods.available else { return nil }
		return snapshot.sessions.periods.items.first(where: { $0.period == selectedPeriod && $0.client == selectedClient })
	}

	var sessionsPanel: SessionsPanelModel {
		guard let snapshot, snapshot.sessions.available else {
			return SessionsPanelModel(available: false, stats: [], projects: [], recent: [], emptyCopy: nil)
		}
		let statistics = sessionStatistics
		let stats: [StatChip] = [
			StatChip(
				id: "sessions",
				label: t(DesktopCopy.sessionsCount),
				value: statistics.map { DesktopFormat.count(Int64($0.sessions)) } ?? "—",
				note: nil
			),
			StatChip(
				id: "average",
				label: t(DesktopCopy.sessionsAverage),
				value: statistics.map {
					DesktopFormat.duration($0.sessions > 0 ? $0.totalDurationSeconds / Int64($0.sessions) : 0)
				} ?? "—",
				note: nil
			),
			StatChip(
				id: "projects",
				label: t(DesktopCopy.sessionsProjects),
				value: statistics.map { DesktopFormat.count(Int64($0.distinctProjects)) } ?? "—",
				note: nil
			),
		]
		let rows = filteredRecentSessions(snapshot)
		let projectTotal = max(Int64(1), statistics?.projects.reduce(0) { $0 + $1.durationSeconds } ?? 0)
		let projects = (statistics?.projects ?? []).map { project -> ShareRow in
			let fraction = Double(project.durationSeconds) / Double(projectTotal)
			return ShareRow(
				id: "project.\(project.project ?? "")",
				label: project.project ?? t(DesktopCopy.sessionsProjectUnnamed),
				value: t(DesktopCopy.sessionsProjectCount, Int64(project.sessions)),
				share: fraction,
				shareText: DesktopFormat.duration(project.durationSeconds)
			)
		}
		// The row identity is positional on purpose: the wire's session
		// identifier is a prohibited value and never enters presentation state.
		let recent = rows.enumerated().map { index, session in
			RecentSessionRow(
				id: "session.\(index)",
				title: session.project ?? t(DesktopCopy.sessionsProjectUnnamed),
				detail: [clientLabel(session.client), session.model].compactMap(\.self).joined(separator: " · "),
				when: sessionDuration(session)
			)
		}
		return SessionsPanelModel(
			available: snapshot.sessions.periods.available,
			stats: stats,
			projects: projects,
			recent: recent,
			emptyCopy: rows.isEmpty ? t(DesktopCopy.sessionsEmpty) : nil
		)
	}

	private func sessionDuration(_ session: DesktopRecentSessionV1) -> String {
		guard let firstAt = session.firstAt,
			let lastAt = session.lastAt,
			let first = DesktopFormat.timestamp(firstAt),
			let last = DesktopFormat.timestamp(lastAt),
			last > first
		else {
			return "—"
		}
		return DesktopFormat.duration(Int64(last.timeIntervalSince(first)))
	}

	/// Both filters reach the recent list: the client narrows it and the period
	/// bounds it. These rows are a display list, never the panel's statistics,
	/// which stay producer-computed.
	private func filteredRecentSessions(_ snapshot: DesktopSnapshotV1) -> [DesktopRecentSessionV1] {
		let bounds = periodBounds(snapshot.generatedAt)
		return snapshot.sessions.items.filter { session in
			guard selectedClient == "all" || session.client == selectedClient else { return false }
			guard let bounds else { return true }
			guard let raw = session.lastAt ?? session.firstAt, let stamp = DesktopFormat.timestamp(raw) else { return false }
			return stamp >= bounds.start && stamp <= bounds.end
		}
	}

	private func periodBounds(_ generatedAt: String) -> (start: Date, end: Date)? {
		guard let end = DesktopFormat.timestamp(generatedAt) else { return nil }
		var calendar = Calendar.current
		calendar.locale = DesktopLocale.current
		let startOfDay = calendar.startOfDay(for: end)
		let days: Int
		switch selectedPeriod {
		case "today": days = 0
		case "7d": days = 6
		case "30d": days = 29
		default: return nil
		}
		guard let start = calendar.date(byAdding: .day, value: -days, to: startOfDay) else { return nil }
		return (start, end)
	}

	private func periodWindow(scope: DesktopUsageScopeV1) -> Set<String> {
		let dates = scope.daily.items.map(\.date).sorted()
		let count: Int
		switch selectedPeriod {
		case "today": count = 1
		case "7d": count = 7
		default: count = 30
		}
		return Set(dates.suffix(count))
	}

	// MARK: Rhythm block

	/// Neither filter reaches this block, so it always reads the all-client
	/// scope and says so in its own heading.
	var rhythmBlock: RhythmBlockModel {
		guard let snapshot,
			let scope = snapshot.usage.presentation.scopes.first(where: { $0.client == "all" })
		else {
			return RhythmBlockModel(available: false, scopeLine: t(DesktopCopy.rhythmScope), figures: [], cells: [], calendar: [])
		}
		let rhythm = scope.rhythm
		let peak = rhythm.cells.max(by: { $0.intensity < $1.intensity })
		let figures: [StatChip] = [
			StatChip(
				id: "active",
				label: t(DesktopCopy.rhythmActive),
				value: t(DesktopCopy.rhythmActiveValue, Int64(rhythm.activeDays), Int64(30)),
				note: t(DesktopCopy.rhythmActiveNote)
			),
			StatChip(
				id: "busiest",
				label: t(DesktopCopy.rhythmBusiest),
				value: DesktopFormat.weekdayName(rhythm.busiestDay),
				note: t(DesktopCopy.rhythmBusiestNote)
			),
			StatChip(
				id: "quietest",
				label: t(DesktopCopy.rhythmQuietest),
				value: DesktopFormat.weekdayName(rhythm.quietestDay),
				note: t(DesktopCopy.rhythmQuietestNote)
			),
			StatChip(
				id: "peak-window",
				label: t(DesktopCopy.rhythmPeakWindow),
				value: peak.map { DesktopFormat.hourWindow($0.hour) } ?? "—",
				note: peak.map {
					t(DesktopCopy.rhythmPeakNote, DesktopFormat.weekday(mondayBased: $0.weekday))
				} ?? t(DesktopCopy.rhythmBusiestNote)
			),
		]
		let cells = rhythm.cells.map { cell in
			let tokens = cell.tokens ?? 0
			let cost = cell.providerCost.map {
				DesktopFormat.cost($0, known: $0, approximate: cell.costIncomplete == true)
			} ?? "—"
			return RhythmCell(
				id: cell.id,
				weekday: cell.weekday,
				hour: cell.hour,
				intensity: cell.intensity,
				tokens: tokens,
				cost: cost,
				accessibilityLabel: t(
					DesktopCopy.rhythmCell,
					DesktopFormat.weekday(mondayBased: cell.weekday),
					DesktopFormat.hour(cell.hour)
				),
				accessibilityValue: "\(DesktopFormat.tokens(tokens)) \(t(DesktopCopy.settingsMenuBarValueTokens)), \(cost)"
			)
		}
		let calendar = scope.daily.items.suffix(90).map { item in
			trendBucket(
				id: "calendar.\(item.date)",
				label: DesktopFormat.compactDay(item.date),
				value: item.value,
				inWindow: true
			)
		}
		return RhythmBlockModel(
			available: rhythm.available,
			scopeLine: t(DesktopCopy.rhythmScope),
			figures: figures,
			cells: cells,
			calendar: Array(calendar)
		)
	}

	// MARK: Footer and switching

	var footer: FooterModel {
		guard let snapshot else {
			return FooterModel(routesText: t(DesktopCopy.footerProviderUnavailable), sections: [], switchingAvailable: false)
		}
		let routes = snapshot.provider.routes
		let routesText = routes.isEmpty
			? t(DesktopCopy.footerProviderUnavailable)
			: routes.map { "\(clientLabel($0.client)) \($0.provider)" }.joined(separator: " · ")
		let candidates = snapshot.provider.candidates
		let blocked = switchController.state.isInFlight
		let sections = ["codex", "claude"].map { client -> SwitchClientSection in
			let current = routes.first(where: { $0.client == client })?.provider
			let options = candidates.flatMap(\.options).filter { $0.client == client }
			var providers = current.map { [$0] } ?? []
			for option in options where !providers.contains(option.provider) {
				providers.append(option.provider)
			}
			let rows = providers.map { provider -> SwitchOptionRow in
				let providerOptions = options.filter { $0.provider == provider }
				let isCurrent = provider == current
				let ready = blocked || isCurrent ? [] : providerOptions.filter(\.ready)
				let choices = ready.map { option in
					SwitchTargetChoice(
						id: option.id,
						label: targetChoiceLabel(option),
						target: ProviderSwitchTarget(option)
					)
				}
				let detail: String?
				if blocked {
					detail = t(DesktopCopy.switchOptionBlocked)
				} else if isCurrent {
					detail = t(DesktopCopy.reasonAlreadySelected)
				} else if choices.count > 1 {
					detail = t(DesktopCopy.switchChooseTarget, Int64(choices.count))
				} else if choices.count == 1 {
					detail = t(DesktopCopy.switchReady)
				} else {
					detail = providerOptions.compactMap { reasonCopy($0.reasonCode) }.first
						?? t(DesktopCopy.switchingUnavailable)
				}
				return SwitchOptionRow(
					id: "\(client):\(provider)",
					label: provider,
					detail: detail,
					enabled: !isCurrent && !blocked && !choices.isEmpty,
					isCurrent: isCurrent,
					target: choices.count == 1 ? choices[0].target : nil,
					choices: choices.count > 1 ? choices : []
				)
			}
			return SwitchClientSection(
				id: client,
				title: clientLabel(client),
				rows: rows
			)
		}
		return FooterModel(
			routesText: routesText,
			sections: sections,
			switchingAvailable: !routes.isEmpty || !candidates.isEmpty
		)
	}

	var switchPresentation: SwitchPresentation {
		switch switchController.state {
		case .idle:
			guard let pendingConfirmation else { return .none }
			return .confirming(target: pendingConfirmation, message: confirmationCopy(pendingConfirmation))
		case .inFlight:
			return .inFlight(message: t(DesktopCopy.switchInFlight))
		case let .failed(_, code):
			return .failed(message: t(DesktopCopy.switchFailed, code), detail: t(DesktopCopy.switchFailedDetail))
		case .indeterminate:
			return .indeterminate(message: t(DesktopCopy.switchIndeterminate), detail: t(DesktopCopy.switchIndeterminateDetail))
		case let .succeeded(target):
			return .succeeded(message: t(DesktopCopy.switchSucceeded, clientLabel(target.client), target.provider))
		}
	}

	func confirmationCopy(_ target: ProviderSwitchTarget) -> String {
		let client = clientLabel(target.client)
		if let credential = target.credential {
			return target.viaWrapper
				? t(DesktopCopy.switchConfirmCredentialWrapper, client, target.provider, credential)
				: t(DesktopCopy.switchConfirmCredentialDirect, client, target.provider, credential)
		}
		return target.viaWrapper
			? t(DesktopCopy.switchConfirmWrapper, client, target.provider)
			: t(DesktopCopy.switchConfirmDirect, client, target.provider)
	}

	func reasonCopy(_ code: String?) -> String? {
		guard let code else { return nil }
		switch code {
		case "credential_missing": return t(DesktopCopy.reasonCredentialMissing)
		case "credential_client_mismatch": return t(DesktopCopy.reasonCredentialClientMismatch)
		case "wrapper_not_configured": return t(DesktopCopy.reasonWrapperNotConfigured)
		case "already_selected": return t(DesktopCopy.reasonAlreadySelected)
		default: return t(DesktopCopy.reasonUnknown, code)
		}
	}

	private func targetChoiceLabel(_ option: DesktopProviderSwitchOptionV1) -> String {
		var parts = [String]()
		if let credential = option.credential {
			parts.append(credential)
		}
		parts.append(option.viaWrapper ? t(DesktopCopy.switchWrapper) : t(DesktopCopy.switchDirect))
		return parts.joined(separator: " · ")
	}

	// MARK: Health detail

	var healthDetail: HealthDetailModel {
		let checks = snapshot?.health.checks ?? []
		let rows = checks.enumerated().map { index, check in
			HealthCheckRow(
				id: "check.\(index).\(check.name)",
				name: check.name,
				status: healthStatusLabel(check.status),
				severity: healthSeverity(check.status),
				recovery: check.recoveryCommand
			)
		}
		return HealthDetailModel(rows: rows, source: t(DesktopCopy.healthSource))
	}

	func healthStatusLabel(_ status: String) -> String {
		switch status {
		case "ok": t(DesktopCopy.healthStatusOK)
		case "warning": t(DesktopCopy.healthStatusWarning)
		default: t(DesktopCopy.healthStatusFailed)
		}
	}

	func healthSeverity(_ status: String) -> NoticeSeverity? {
		switch status {
		case "ok": nil
		case "warning": .warning
		default: .error
		}
	}

	// MARK: Menu-bar item

	/// The item follows the panel filter only when the preference says so, so a
	/// filter the user set and then dismissed never silently narrows it.
	var menuBarText: String? {
		guard preferences.menuBarValue != .icon else { return nil }
		let client = preferences.menuBarScope == .followPanel ? selectedClient : "all"
		guard let snapshot else { return nil }
		let subtotals = snapshot.usage.presentation.clientSubtotals
		if subtotals.available,
			let value = subtotals.items.first(where: { $0.period == "today" && $0.client == client })?.value
		{
			return preferences.menuBarValue == .cost ? costText(value) : DesktopFormat.tokens(value.tokens)
		}
		guard let totals = snapshot.usage.presentation.scopes
			.first(where: { $0.client == client })?.periods.items
			.first(where: { $0.period == "today" })?.totals
		else {
			return nil
		}
		return preferences.menuBarValue == .cost ? costText(totals) : DesktopFormat.tokens(totals.tokens)
	}

	var menuBarBadged: Bool { presentation.isBadged }

	var menuBarAccessibilityLabel: String {
		if qualifiers.contains(.offline) { return t(DesktopCopy.badgedOffline) }
		if qualifiers.contains(.failing) { return t(DesktopCopy.badgedFailing) }
		return t(DesktopCopy.appName)
	}

	// MARK: Actions

	func refresh() {
		Task { await coordinator.refresh(replacingActiveRefresh: true) }
	}

	func sectionIsExpanded(_ id: String) -> Bool {
		!collapsedSectionIDs.contains(id)
	}

	func setSection(_ id: String, expanded: Bool) {
		if expanded {
			collapsedSectionIDs.remove(id)
		} else {
			collapsedSectionIDs.insert(id)
		}
	}

	func confirmSwitch() {
		guard let target = pendingConfirmation else { return }
		pendingConfirmation = nil
		switchController.start(target)
	}

	func cancelConfirmation() {
		pendingConfirmation = nil
	}

	func retrySwitch() {
		switchController.retry()
	}

	func dismissSwitch() {
		switchController.dismiss()
		pendingConfirmation = nil
	}

	// MARK: Labels

	func clientLabel(_ value: String) -> String {
		switch value {
		case "all": t(DesktopCopy.clientAll)
		case "codex": "Codex"
		case "claude": "Claude"
		default: value
		}
	}

	func periodLabel(_ value: String) -> String {
		switch value {
		case "today": t(DesktopCopy.periodToday)
		case "7d": t(DesktopCopy.period7d)
		case "30d": t(DesktopCopy.period30d)
		default: value
		}
	}

	func qualityLabel(_ value: String) -> String {
		switch value {
		case "determinable": t(DesktopCopy.qualityDeterminable)
		case "inferred": t(DesktopCopy.qualityInferred)
		case "unattributed": t(DesktopCopy.qualityUnattributed)
		default: value
		}
	}

	func costText(_ totals: DesktopPresentationTotalsV1) -> String {
		DesktopFormat.cost(totals.providerCost, known: totals.knownProviderCost, approximate: totals.providerCost == nil)
	}

	func costText(_ value: DesktopPresentationValueV1) -> String {
		DesktopFormat.cost(value.providerCost, known: value.providerCost, approximate: value.costIncomplete)
	}

	private func trendBucket(
		id: String,
		label: String,
		value: DesktopPresentationValueV1,
		inWindow: Bool
	) -> TrendBucket {
		TrendBucket(
			id: id,
			label: label,
			tokens: value.tokens,
			events: value.events,
			cost: costText(value),
			magnitude: costMagnitude(value),
			inWindow: inWindow,
			accessibilityValue: "\(costText(value)), \(DesktopFormat.tokens(value.tokens)), \(t(DesktopCopy.trendEvents, value.events))"
		)
	}

	private func emptyHourlyTrendBucket(hour: Int) -> TrendBucket {
		let cost = DesktopFormat.cost("0", known: "0", approximate: false)
		return TrendBucket(
			id: "hour.\(hour)",
			label: DesktopFormat.hourWindow(hour),
			tokens: 0,
			events: 0,
			cost: cost,
			magnitude: 0,
			inWindow: false,
			accessibilityValue: "\(cost), \(DesktopFormat.tokens(0)), \(t(DesktopCopy.trendEvents, Int64(0)))"
		)
	}

	private func costMagnitude(_ value: DesktopPresentationValueV1) -> Double {
		let cost = Double(value.providerCost) ?? 0
		return cost > 0 ? cost : Double(value.tokens)
	}

	private func shareFraction(_ raw: String?) -> Double {
		min(1, max(0, (Double(raw ?? "0") ?? 0) / 100))
	}
}
