import Foundation
import WidgetKit

enum WidgetTimelinePolicy {
	static let minimumRefresh: TimeInterval = 3 * 60
	static let defaultRefresh: TimeInterval = 4 * 60
	static let maximumRefresh: TimeInterval = 5 * 60

	static func refreshDate(suggestedAt: String?, now: Date) -> Date {
		let minimum = now.addingTimeInterval(minimumRefresh)
		let maximum = now.addingTimeInterval(maximumRefresh)
		guard let suggestedAt, let suggested = date(suggestedAt) else {
			return now.addingTimeInterval(defaultRefresh)
		}
		return min(max(suggested, minimum), maximum)
	}

	static func date(_ value: String) -> Date? {
		let fractional = ISO8601DateFormatter()
		fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
		if let result = fractional.date(from: value) {
			return result
		}
		return ISO8601DateFormatter().date(from: value)
	}
}

struct WidgetSnapshotLoader: Sendable {
	private let loadOutcome: @Sendable () -> WidgetLoadOutcome

	init(reader: WidgetSnapshotReader? = WidgetSnapshotReader()) {
		loadOutcome = {
			guard let reader else { return .failed(.containerUnavailable) }
			do {
				return .loaded(try reader.read())
			} catch let failure as WidgetLoadFailure {
				return .failed(failure)
			} catch {
				return .failed(.unreadable(category: .io))
			}
		}
	}

	init(readSnapshot: @escaping @Sendable () throws -> WidgetDesktopSnapshotV1) {
		loadOutcome = {
			do {
				return .loaded(try readSnapshot())
			} catch let failure as WidgetLoadFailure {
				return .failed(failure)
			} catch {
				return .failed(.unreadable(category: .io))
			}
		}
	}

	init(loadOutcome: @escaping @Sendable () -> WidgetLoadOutcome) {
		self.loadOutcome = loadOutcome
	}

	func entry(
		kind: AgentDeckWidgetKind,
		client: WidgetClient,
		period: WidgetPeriod,
		now: Date,
		placeholder: Bool = false
	) -> AgentDeckWidgetEntry {
		AgentDeckWidgetEntry(
			date: now,
			outcome: placeholder ? .placeholder : loadOutcome(),
			kind: kind,
			client: client,
			period: period
		)
	}
}

struct ClientPeriodTimelineProvider: AppIntentTimelineProvider {
	let kind: AgentDeckWidgetKind
	private let loader: WidgetSnapshotLoader
	private let now: @Sendable () -> Date

	init(
		kind: AgentDeckWidgetKind,
		loader: WidgetSnapshotLoader = WidgetSnapshotLoader(),
		now: @escaping @Sendable () -> Date = Date.init
	) {
		self.kind = kind
		self.loader = loader
		self.now = now
	}

	func entry(client: WidgetClient, period: WidgetPeriod, placeholder: Bool = false) -> AgentDeckWidgetEntry {
		loader.entry(kind: kind, client: client, period: period, now: now(), placeholder: placeholder)
	}

	func placeholder(in context: Context) -> AgentDeckWidgetEntry {
		entry(client: .all, period: .today, placeholder: true)
	}

	func snapshot(for configuration: ClientPeriodWidgetIntent, in context: Context) async -> AgentDeckWidgetEntry {
		entry(client: configuration.client ?? .all, period: configuration.period ?? .today)
	}

	func timeline(for configuration: ClientPeriodWidgetIntent, in context: Context) async -> Timeline<AgentDeckWidgetEntry> {
		let entry = entry(client: configuration.client ?? .all, period: configuration.period ?? .today)
		let refresh = WidgetTimelinePolicy.refreshDate(suggestedAt: entry.snapshot?.nextRefreshAt, now: entry.date)
		return Timeline(entries: [entry], policy: .after(refresh))
	}
}

struct ClientTimelineProvider: AppIntentTimelineProvider {
	let kind: AgentDeckWidgetKind
	private let loader: WidgetSnapshotLoader
	private let now: @Sendable () -> Date

	init(
		kind: AgentDeckWidgetKind,
		loader: WidgetSnapshotLoader = WidgetSnapshotLoader(),
		now: @escaping @Sendable () -> Date = Date.init
	) {
		self.kind = kind
		self.loader = loader
		self.now = now
	}

	func entry(client: WidgetClient, placeholder: Bool = false) -> AgentDeckWidgetEntry {
		loader.entry(kind: kind, client: client, period: fixedPeriod, now: now(), placeholder: placeholder)
	}

	func placeholder(in context: Context) -> AgentDeckWidgetEntry {
		entry(client: .all, placeholder: true)
	}

	func snapshot(for configuration: ClientWidgetIntent, in context: Context) async -> AgentDeckWidgetEntry {
		entry(client: configuration.client ?? .all)
	}

	func timeline(for configuration: ClientWidgetIntent, in context: Context) async -> Timeline<AgentDeckWidgetEntry> {
		let entry = entry(client: configuration.client ?? .all)
		let refresh = WidgetTimelinePolicy.refreshDate(suggestedAt: entry.snapshot?.nextRefreshAt, now: entry.date)
		return Timeline(entries: [entry], policy: .after(refresh))
	}

	private var fixedPeriod: WidgetPeriod {
		kind == .rhythm ? .thirtyDays : .today
	}
}

// Quota's own provider, distinct from ClientTimelineProvider because it takes
// QuotaWidgetIntent (no .all case -- see that type's doc).
struct QuotaTimelineProvider: AppIntentTimelineProvider {
	private let loader: WidgetSnapshotLoader
	private let now: @Sendable () -> Date

	init(
		loader: WidgetSnapshotLoader = WidgetSnapshotLoader(),
		now: @escaping @Sendable () -> Date = Date.init
	) {
		self.loader = loader
		self.now = now
	}

	func entry(client: WidgetClient, placeholder: Bool = false) -> AgentDeckWidgetEntry {
		loader.entry(kind: .quota, client: client, period: .today, now: now(), placeholder: placeholder)
	}

	func placeholder(in context: Context) -> AgentDeckWidgetEntry {
		entry(client: .codex, placeholder: true)
	}

	func snapshot(for configuration: QuotaWidgetIntent, in context: Context) async -> AgentDeckWidgetEntry {
		entry(client: (configuration.client ?? .codex).widgetClient)
	}

	func timeline(for configuration: QuotaWidgetIntent, in context: Context) async -> Timeline<AgentDeckWidgetEntry> {
		let entry = entry(client: (configuration.client ?? .codex).widgetClient)
		let refresh = WidgetTimelinePolicy.refreshDate(suggestedAt: entry.snapshot?.nextRefreshAt, now: entry.date)
		return Timeline(entries: [entry], policy: .after(refresh))
	}
}
