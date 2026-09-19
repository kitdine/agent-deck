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

// Codex PR #5 sixth review, P2: the quota design supports a single
// configured Codex or Claude client at small and medium sizes -- there is no
// honest "all clients" presentation at those sizes, unlike trust and rhythm
// (which share ClientWidgetIntent/WidgetClient's .all case). presentedQuotaClients
// silently truncated an .all selection to the first client, showing only
// Codex while the picker still said "All clients." A quota-specific intent
// without .all removes that mismatch instead of implementing a presentation
// the design does not define.
enum QuotaWidgetClient: String, AppEnum, CaseIterable, Codable, Sendable {
	case codex
	case claude

	static let typeDisplayRepresentation = TypeDisplayRepresentation(name: "Client")
	static let caseDisplayRepresentations: [Self: DisplayRepresentation] = [
		.codex: "Codex",
		.claude: "Claude",
	]

	var widgetClient: WidgetClient {
		switch self {
		case .codex: .codex
		case .claude: .claude
		}
	}
}

struct QuotaWidgetIntent: WidgetConfigurationIntent {
	static let title: LocalizedStringResource = "AgentDeck quota"
	static let description = IntentDescription("Choose the client shown by this widget.")

	@Parameter(title: "Client", default: .codex) var client: QuotaWidgetClient?

	init() {
		client = .codex
	}
}
