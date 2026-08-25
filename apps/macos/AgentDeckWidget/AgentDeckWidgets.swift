import SwiftUI
import WidgetKit

@main
struct AgentDeckWidgetBundle: WidgetBundle {
	var body: some Widget {
		MagnitudeWidget()
		CompositionWidget()
		TrustWidget()
		RhythmWidget()
	}
}

private struct MagnitudeWidget: Widget {
	var body: some WidgetConfiguration {
		AppIntentConfiguration(kind: "com.kitdine.agentdeck.widget.magnitude", intent: ClientPeriodWidgetIntent.self, provider: ClientPeriodTimelineProvider(kind: .magnitude)) { entry in
			AgentDeckWidgetView(entry: entry)
		}
		.configurationDisplayName("Magnitude")
		.description("How much am I spending?")
		.supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
	}
}

private struct CompositionWidget: Widget {
	var body: some WidgetConfiguration {
		AppIntentConfiguration(kind: "com.kitdine.agentdeck.widget.composition", intent: ClientPeriodWidgetIntent.self, provider: ClientPeriodTimelineProvider(kind: .composition)) { entry in
			AgentDeckWidgetView(entry: entry)
		}
		.configurationDisplayName("Composition")
		.description("Where does it go?")
		.supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
	}
}

private struct TrustWidget: Widget {
	var body: some WidgetConfiguration {
		AppIntentConfiguration(kind: "com.kitdine.agentdeck.widget.trust", intent: ClientWidgetIntent.self, provider: ClientTimelineProvider(kind: .trust)) { entry in
			AgentDeckWidgetView(entry: entry)
		}
		.configurationDisplayName("Trust")
		.description("Is the number real?")
		.supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
	}
}

private struct RhythmWidget: Widget {
	var body: some WidgetConfiguration {
		AppIntentConfiguration(kind: "com.kitdine.agentdeck.widget.rhythm", intent: ClientWidgetIntent.self, provider: ClientTimelineProvider(kind: .rhythm)) { entry in
			AgentDeckWidgetView(entry: entry)
		}
		.configurationDisplayName("Rhythm")
		.description("When do I actually work?")
		.supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
	}
}
