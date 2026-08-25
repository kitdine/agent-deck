import Foundation
import WidgetKit

enum WidgetTimelinePolicy {
	static let minimumRefresh: TimeInterval = 15 * 60
	static let maximumRefresh: TimeInterval = 60 * 60

	static func refreshDate(suggestedAt: String?, now: Date) -> Date {
		let minimum = now.addingTimeInterval(minimumRefresh)
		let maximum = now.addingTimeInterval(maximumRefresh)
		guard let suggestedAt, let suggested = date(suggestedAt) else {
			return maximum
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
	private let readSnapshot: @Sendable () throws -> WidgetDesktopSnapshotV1

	init(reader: WidgetSnapshotReader? = WidgetSnapshotReader()) {
		readSnapshot = {
			guard let reader else { throw WidgetSnapshotLoadError.containerUnavailable }
			return try reader.read()
		}
	}

	init(readSnapshot: @escaping @Sendable () throws -> WidgetDesktopSnapshotV1) {
		self.readSnapshot = readSnapshot
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
			snapshot: placeholder ? nil : try? readSnapshot(),
			kind: kind,
			client: client,
			period: period,
			isPlaceholder: placeholder
		)
	}
}

private enum WidgetSnapshotLoadError: Error {
	case containerUnavailable
}

struct ClientPeriodTimelineProvider: AppIntentTimelineProvider {
	let kind: AgentDeckWidgetKind
	private let loader = WidgetSnapshotLoader()

	func placeholder(in context: Context) -> AgentDeckWidgetEntry {
		loader.entry(kind: kind, client: .all, period: .today, now: Date(), placeholder: true)
	}

	func snapshot(for configuration: ClientPeriodWidgetIntent, in context: Context) async -> AgentDeckWidgetEntry {
		loader.entry(kind: kind, client: configuration.client ?? .all, period: configuration.period ?? .today, now: Date())
	}

	func timeline(for configuration: ClientPeriodWidgetIntent, in context: Context) async -> Timeline<AgentDeckWidgetEntry> {
		let now = Date()
		let entry = loader.entry(kind: kind, client: configuration.client ?? .all, period: configuration.period ?? .today, now: now)
		let refresh = WidgetTimelinePolicy.refreshDate(suggestedAt: entry.snapshot?.nextRefreshAt, now: now)
		return Timeline(entries: [entry], policy: .after(refresh))
	}
}

struct ClientTimelineProvider: AppIntentTimelineProvider {
	let kind: AgentDeckWidgetKind
	private let loader = WidgetSnapshotLoader()

	func placeholder(in context: Context) -> AgentDeckWidgetEntry {
		loader.entry(kind: kind, client: .all, period: fixedPeriod, now: Date(), placeholder: true)
	}

	func snapshot(for configuration: ClientWidgetIntent, in context: Context) async -> AgentDeckWidgetEntry {
		loader.entry(kind: kind, client: configuration.client ?? .all, period: fixedPeriod, now: Date())
	}

	func timeline(for configuration: ClientWidgetIntent, in context: Context) async -> Timeline<AgentDeckWidgetEntry> {
		let now = Date()
		let entry = loader.entry(kind: kind, client: configuration.client ?? .all, period: fixedPeriod, now: now)
		let refresh = WidgetTimelinePolicy.refreshDate(suggestedAt: entry.snapshot?.nextRefreshAt, now: now)
		return Timeline(entries: [entry], policy: .after(refresh))
	}

	private var fixedPeriod: WidgetPeriod {
		kind == .rhythm ? .thirtyDays : .today
	}
}
