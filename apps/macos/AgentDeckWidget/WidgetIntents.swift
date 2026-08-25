import AppIntents

enum WidgetClient: String, AppEnum, CaseIterable, Codable, Sendable {
	case all
	case codex
	case claude

	static let typeDisplayRepresentation = TypeDisplayRepresentation(name: "Client")
	static let caseDisplayRepresentations: [Self: DisplayRepresentation] = [
		.all: "All clients",
		.codex: "Codex",
		.claude: "Claude",
	]
}

enum WidgetPeriod: String, AppEnum, CaseIterable, Codable, Sendable {
	case today
	case sevenDays = "7d"
	case thirtyDays = "30d"

	static let typeDisplayRepresentation = TypeDisplayRepresentation(name: "Period")
	static let caseDisplayRepresentations: [Self: DisplayRepresentation] = [
		.today: "Today",
		.sevenDays: "7 days",
		.thirtyDays: "30 days",
	]
}

struct ClientPeriodWidgetIntent: WidgetConfigurationIntent {
	static let title: LocalizedStringResource = "AgentDeck usage"
	static let description = IntentDescription("Choose the client and period shown by this widget.")

	@Parameter(title: "Client", default: .all) var client: WidgetClient?
	@Parameter(title: "Period", default: .today) var period: WidgetPeriod?

	init() {
		client = .all
		period = .today
	}
}

struct ClientWidgetIntent: WidgetConfigurationIntent {
	static let title: LocalizedStringResource = "AgentDeck usage"
	static let description = IntentDescription("Choose the client shown by this widget.")

	@Parameter(title: "Client", default: .all) var client: WidgetClient?

	init() {
		client = .all
	}
}
