import Foundation

private final class WidgetBundleToken: NSObject {}

enum WidgetCopy {
	static let resourceBundle = Bundle(for: WidgetBundleToken.self)

	static let allKeys = [
		"Magnitude", "Composition", "Trust", "Rhythm",
		"Usage", "Breakdown", "Attribution", "Activity",
		"All clients", "Client", "Period", "Today", "7 days", "30 days",
		"Some data unavailable", "Updated over 15 minutes ago", "Updated over 6 hours ago", "No activity",
		"Data unavailable", "No spend in this period", "No model usage in this period",
		"No attribution data", "No activity in the last 30 days", "Cost incomplete",
		"Tokens", "Sessions", "Average per day", "Peak", "Cache hit", "Top model", "of", "Usage trend", "Date range",
		"Trend", "Rising", "Falling", "Steady",
		"Input", "Output", "Cache read", "Cache write", "Cache write is billed", "Client subtotals",
		"Models", "Token mix", "Determinable", "Inferred", "Unattributed", "Pricing coverage", "By provider",
		"Measurement quality", "Determinate cost", "Unpriced identifiers", "Cost remains visibly incomplete",
		"Active days", "Busiest", "Busiest at", "Quietest", "Activity by hour", "Hour of week",
		"Low", "High", "90-day context", "Updated now", "Updated %@", "Last updated %@", "unpriced",
	]

	static func text(_ key: String, bundle: Bundle? = nil) -> String {
		NSLocalizedString(key, bundle: bundle ?? resourceBundle, comment: "")
	}

	static func format(_ key: String, value: String, bundle: Bundle? = nil) -> String {
		String(format: text(key, bundle: bundle), value)
	}

	static func client(_ client: WidgetClient, bundle: Bundle? = nil) -> String {
		client == .all ? text("All clients", bundle: bundle) : client.rawValue.capitalized
	}

	static func period(_ period: WidgetPeriod, bundle: Bundle? = nil) -> String {
		switch period {
		case .today: text("Today", bundle: bundle)
		case .sevenDays: text("7 days", bundle: bundle)
		case .thirtyDays: text("30 days", bundle: bundle)
		}
	}
}
