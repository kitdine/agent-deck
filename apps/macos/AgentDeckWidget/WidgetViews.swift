import SwiftUI
import WidgetKit

private enum WidgetPalette {
	static let accent = Color(red: 0.949, green: 0.396, blue: 0.059)
	static let info = Color(red: 0.231, green: 0.510, blue: 0.965)
	static let good = Color(red: 0.133, green: 0.773, blue: 0.369)
	static let warn = Color(red: 0.961, green: 0.620, blue: 0.043)
	static let modelTones = [
		Color(red: 0.447, green: 0.624, blue: 0.949),
		Color(red: 0.753, green: 0.443, blue: 0.929),
		Color(red: 0.271, green: 0.702, blue: 0.627),
		Color(red: 0.929, green: 0.557, blue: 0.275),
	]
	static let surfaceDark = Color(red: 0.071, green: 0.082, blue: 0.110)
	static let secondarySurface = Color.primary.opacity(0.055)
	static let track = Color.primary.opacity(0.11)
	static let divider = Color.primary.opacity(0.10)

	static func model(_ index: Int) -> Color {
		modelTones[index % modelTones.count]
	}

	static func quality(_ value: String) -> Color {
		switch value {
		case "determinable": good
		case "inferred": model(1)
		default: warn
		}
	}

	static func heat(_ level: Int) -> Color {
		let opacity = [0.08, 0.18, 0.32, 0.48, 0.66, 0.86][min(max(level, 0), 5)]
		return info.opacity(opacity)
	}
}

struct WidgetAccessibilityDescriptor: Equatable {
	let label: String
	let value: String
}

enum WidgetAccessibility {
	static func metric(label: String, values: [String?]) -> WidgetAccessibilityDescriptor {
		WidgetAccessibilityDescriptor(
			label: label,
			value: values.compactMap { value in
				guard let value, !value.isEmpty else { return nil }
				return value
			}.joined(separator: ", ")
		)
	}

	static func trend(
		items: [DesktopUsageDailyItemV1],
		label: String = WidgetCopy.text("Usage trend")
	) -> WidgetAccessibilityDescriptor {
		guard let first = items.first, let last = items.last else {
			return metric(label: label, values: [WidgetCopy.text("Data unavailable")])
		}
		let values = items.map { WidgetFormat.chartValue($0.value) }
		let peak = items.enumerated().max { left, right in
			values[left.offset] < values[right.offset]
		}?.element ?? first
		return metric(label: label, values: [
			"\(WidgetCopy.text("Date range")): \(WidgetFormat.date(first.date)) – \(WidgetFormat.date(last.date))",
			"\(WidgetCopy.text("Peak")): \(WidgetFormat.date(peak.date)), \(amount(peak.value))",
			"\(WidgetCopy.text("Trend")): \(trendDirection(values))",
		])
	}

	static func hourGrid(_ rhythm: DesktopUsageRhythmV1) -> WidgetAccessibilityDescriptor {
		let weekdays = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
		let range = "\(WidgetFormat.weekday(weekdays[0])) – \(WidgetFormat.weekday(weekdays[6])), 00–24"
		guard let maximum = rhythm.intensities.max(), maximum > 0,
			let index = rhythm.intensities.firstIndex(of: maximum), index < weekdays.count * 24
		else {
			return metric(label: WidgetCopy.text("Activity by hour"), values: [
				"\(WidgetCopy.text("Date range")): \(range)",
				WidgetCopy.text("No activity"),
			])
		}
		let day = index / 24
		let hour = index % 24
		return metric(label: WidgetCopy.text("Activity by hour"), values: [
			"\(WidgetCopy.text("Date range")): \(range)",
			"\(WidgetCopy.text("Peak")): \(WidgetFormat.weekday(weekdays[day])) \(WidgetFormat.hourRange(start: hour, end: min(hour + 1, 24)))",
		])
	}

	static func trendDirection(_ values: [Double]) -> String {
		guard let first = values.first, let last = values.last else { return WidgetCopy.text("Steady") }
		if last > first { return WidgetCopy.text("Rising") }
		if last < first { return WidgetCopy.text("Falling") }
		return WidgetCopy.text("Steady")
	}

	private static func amount(_ value: DesktopPresentationValueV1) -> String {
		guard !value.costIncomplete else {
			return "\(WidgetFormat.tokens(value.tokens)) \(WidgetCopy.text("Tokens"))"
		}
		return WidgetFormat.cost(value.providerCost, known: value.providerCost, incomplete: false)
	}
}

private extension View {
	func widgetAccessibility(_ descriptor: WidgetAccessibilityDescriptor) -> some View {
		accessibilityElement(children: .ignore)
			.accessibilityLabel(Text(descriptor.label))
			.accessibilityValue(Text(descriptor.value))
	}
}

struct AgentDeckWidgetView: View {
	@Environment(\.widgetFamily) private var environmentFamily
	@Environment(\.colorScheme) private var colorScheme
	@Environment(\.dynamicTypeSize) private var dynamicTypeSize
	let entry: AgentDeckWidgetEntry
	let familyOverride: WidgetFamily?

	init(entry: AgentDeckWidgetEntry, familyOverride: WidgetFamily? = nil) {
		self.entry = entry
		self.familyOverride = familyOverride
	}

	private var family: WidgetFamily {
		WidgetLayoutContract.presentationFamily(
			familyOverride ?? environmentFamily,
			dynamicTypeSize: dynamicTypeSize
		)
	}

	var body: some View {
		let model = WidgetSurfaceModel(entry: entry, now: entry.date)
		Group {
			switch model.surface {
			case .placeholder:
				widgetContent(model).redacted(reason: .placeholder)
			case .unavailable:
				UnavailableWidget(kind: entry.kind)
			case .data:
				widgetContent(model)
			}
		}
		.containerBackground(colorScheme == .dark ? WidgetPalette.surfaceDark : Color.white, for: .widget)
		.accessibilityElement(children: .contain)
	}

	@ViewBuilder private func widgetContent(_ model: WidgetSurfaceModel) -> some View {
		WidgetFrame(entry: entry, qualifiers: model.qualifiers, family: family) {
			switch entry.kind {
			case .magnitude: MagnitudeWidgetView(model: model, family: family)
			case .composition: CompositionWidgetView(model: model, family: family)
			case .trust: TrustWidgetView(model: model, family: family)
			case .rhythm: RhythmWidgetView(model: model, family: family)
			}
		}
	}
}

private struct WidgetFrame<Content: View>: View {
	let entry: AgentDeckWidgetEntry
	let qualifiers: [WidgetQualifier]
	let family: WidgetFamily
	let content: Content

	init(
		entry: AgentDeckWidgetEntry,
		qualifiers: [WidgetQualifier],
		family: WidgetFamily,
		@ViewBuilder content: () -> Content
	) {
		self.entry = entry
		self.qualifiers = qualifiers
		self.family = family
		self.content = content()
	}

	var body: some View {
		VStack(alignment: .leading, spacing: 0) {
			WidgetHeader(entry: entry, family: family)
			content.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
			WidgetFooter(entry: entry, qualifiers: qualifiers)
		}
		.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
	}
}

private struct WidgetHeader: View {
	let entry: AgentDeckWidgetEntry
	let family: WidgetFamily

	var body: some View {
		HStack(spacing: 5) {
			Image(systemName: icon)
				.font(.system(size: 11, weight: .semibold))
				.foregroundStyle(WidgetPalette.accent)
				.accessibilityHidden(true)
			Text(WidgetCopy.text(titleKey))
				.font(.system(size: 10.5, weight: .semibold))
				.foregroundStyle(.secondary)
				.lineLimit(1)
			Spacer(minLength: 4)
			Text(scopeText)
				.font(.system(size: 9))
				.foregroundStyle(.tertiary)
				.lineLimit(1)
				.minimumScaleFactor(0.82)
		}
		.padding(.bottom, 6)
		.widgetAccessibility(WidgetAccessibility.metric(label: WidgetCopy.text(titleKey), values: [scopeText]))
	}

	private var titleKey: String {
		switch entry.kind {
		case .magnitude: "Usage"
		case .composition: "Breakdown"
		case .trust: "Attribution"
		case .rhythm: "Activity"
		}
	}

	private var scopeText: String {
		switch entry.kind {
		case .magnitude, .composition:
			let period = WidgetCopy.period(entry.period)
			guard family != .systemSmall else { return period }
			return "\(WidgetCopy.client(entry.client)) · \(period)"
		case .trust:
			return WidgetCopy.period(.today)
		case .rhythm:
			return WidgetCopy.period(.thirtyDays)
		}
	}

	private var icon: String {
		switch entry.kind {
		case .magnitude: "chart.bar.fill"
		case .composition: "chart.pie.fill"
		case .trust: "checkmark.shield.fill"
		case .rhythm: "clock.arrow.circlepath"
		}
	}
}

private struct WidgetFooter: View {
	let entry: AgentDeckWidgetEntry
	let qualifiers: [WidgetQualifier]

	var body: some View {
		let presentation = WidgetFooterPresentation(qualifiers: qualifiers, relativeTime: relativeTime)
		HStack(spacing: 5) {
			Text(presentation.updateText)
				.foregroundStyle(presentation.isOld ? WidgetPalette.warn : Color.secondary)
			Spacer(minLength: 3)
			if !presentation.qualifierText.isEmpty {
				Text(presentation.qualifierText)
					.foregroundStyle(.secondary)
			}
		}
		.font(.system(size: 9))
		.foregroundStyle(.tertiary)
		.lineLimit(1)
		.minimumScaleFactor(0.75)
		.padding(.top, 6)
		.widgetAccessibility(WidgetAccessibility.metric(
			label: presentation.updateText,
			values: [presentation.qualifierText.isEmpty ? nil : presentation.qualifierText]
		))
	}

	private var relativeTime: String? {
		guard let generatedAt = entry.snapshot?.generatedAt,
			let generated = WidgetTimelinePolicy.date(generatedAt),
			entry.date.timeIntervalSince(generated) >= 60
		else {
			return nil
		}
		let formatter = RelativeDateTimeFormatter()
		formatter.unitsStyle = .full
		return formatter.localizedString(for: generated, relativeTo: entry.date)
	}
}

private struct MagnitudeWidgetView: View {
	let model: WidgetSurfaceModel
	let family: WidgetFamily

	@ViewBuilder var body: some View {
		if let scope = model.scope, let selected = model.period {
			switch family {
			case .systemMedium:
				medium(scope: scope)
			case .systemLarge:
				large(scope: scope, selected: selected)
			default:
				small(scope: scope, selected: selected)
			}
		} else {
			UnavailableWidget(kind: .magnitude)
		}
	}

	private func small(scope: DesktopUsageScopeV1, selected: DesktopUsagePeriodV1) -> some View {
		let items = Array(scope.daily.items.suffix(WidgetLayoutContract.bucketCount(.systemSmall)))
		return VStack(alignment: .leading, spacing: 0) {
			VStack(alignment: .leading, spacing: 0) {
				Text(cost(selected))
					.font(.system(size: 25, weight: .semibold, design: .rounded))
					.foregroundStyle(WidgetPalette.accent)
					.lineLimit(1)
				Text(volume(selected))
					.font(.system(size: 9.5))
					.foregroundStyle(.secondary)
					.lineLimit(1)
					.minimumScaleFactor(0.82)
					.padding(.top, 4)
			}
			.widgetAccessibility(WidgetAccessibility.metric(
				label: WidgetCopy.period(WidgetPeriod(rawValue: selected.period) ?? .today),
				values: [cost(selected), volume(selected)]
			))
			MiniBarChart(
				values: items.map { WidgetFormat.chartValue($0.value) },
				tone: WidgetPalette.info
			)
			.padding(.top, 7)
			.widgetAccessibility(WidgetAccessibility.trend(items: items))
		}
	}

	private func medium(scope: DesktopUsageScopeV1) -> some View {
		let buckets = Array(scope.daily.items.suffix(WidgetLayoutContract.bucketCount(.systemMedium)))
		return VStack(alignment: .leading, spacing: 0) {
			PeriodColumns(items: Array(scope.periods.items.prefix(3)), showsTokens: true)
			MiniBarChart(values: buckets.map { WidgetFormat.chartValue($0.value) }, tone: WidgetPalette.info)
				.padding(.top, 7)
				.widgetAccessibility(WidgetAccessibility.trend(items: buckets))
			DateAxis(first: buckets.first?.date, last: buckets.last?.date)
				.accessibilityHidden(true)
		}
	}

	private func large(scope: DesktopUsageScopeV1, selected: DesktopUsagePeriodV1) -> some View {
		let referencePeriods = scope.periods.items.filter {
			$0.period == WidgetPeriod.today.rawValue || $0.period == WidgetPeriod.sevenDays.rawValue
		}
		return VStack(alignment: .leading, spacing: 0) {
			VStack(alignment: .leading, spacing: 0) {
				HStack(alignment: .firstTextBaseline, spacing: 8) {
					Text(cost(selected))
						.font(.system(size: 19, weight: .semibold, design: .rounded))
						.foregroundStyle(WidgetPalette.accent)
					Spacer(minLength: 4)
					if !selected.totals.pricingComplete {
						PricingFlag(unpricedComponents: selected.totals.unpricedComponents)
					}
				}
				Text(volume(selected))
					.font(.system(size: 9.5))
					.foregroundStyle(.secondary)
					.padding(.top, 3)
			}
			.widgetAccessibility(WidgetAccessibility.metric(
				label: WidgetCopy.period(WidgetPeriod(rawValue: selected.period) ?? .thirtyDays),
				values: [
					cost(selected),
					volume(selected),
					selected.totals.pricingComplete ? nil : WidgetCopy.text("Cost incomplete"),
				]
			))
			PeriodColumns(items: referencePeriods, showsTokens: false)
				.frame(maxWidth: 150, alignment: .leading)
				.padding(.top, 7)
			AreaSeriesChart(values: dailyValues(scope))
				.frame(height: 66)
				.padding(.top, 8)
				.widgetAccessibility(WidgetAccessibility.trend(items: scope.daily.items))
			DateAxis(first: scope.daily.items.first?.date, last: scope.daily.items.last?.date)
				.accessibilityHidden(true)
			Spacer(minLength: 5)
			StatChipRow(items: [
				StatItem(label: WidgetCopy.text("Average per day"), value: averageCost(selected)),
				StatItem(
					label: "\(WidgetCopy.text("Peak")) · \(WidgetFormat.date(selected.peak.date))",
					value: cost(selected.peak.totals)
				),
				StatItem(label: WidgetCopy.text("Cache hit"), value: WidgetFormat.share(selected.cacheHitShare)),
			])
		}
	}

	private func dailyValues(_ scope: DesktopUsageScopeV1) -> [Double] {
		scope.daily.items.map { WidgetFormat.chartValue($0.value) }
	}

	private func volume(_ item: DesktopUsagePeriodV1) -> String {
		"\(WidgetFormat.tokens(item.totals.tokens)) · \(item.totals.sessions) \(WidgetCopy.text("Sessions"))"
	}

	private func averageCost(_ item: DesktopUsagePeriodV1) -> String {
		WidgetFormat.cost(
			item.averagePerDay.providerCost,
			known: item.averagePerDay.knownProviderCost,
			incomplete: item.averagePerDay.providerCost == nil
		)
	}

	private func cost(_ item: DesktopUsagePeriodV1) -> String {
		cost(item.totals)
	}

	private func cost(_ totals: DesktopPresentationTotalsV1) -> String {
		WidgetFormat.cost(totals.providerCost, known: totals.knownProviderCost, incomplete: !totals.pricingComplete)
	}
}

private struct CompositionWidgetView: View {
	let model: WidgetSurfaceModel
	let family: WidgetFamily

	@ViewBuilder var body: some View {
		if let period = model.period, let top = period.models.first {
			switch family {
			case .systemMedium:
				modelList(period)
			case .systemLarge:
				large(period)
			default:
				small(period: period, top: top)
			}
		} else {
			UnavailableWidget(kind: .composition)
		}
	}

	private func small(period: DesktopUsagePeriodV1, top: DesktopPresentationModelV1) -> some View {
		VStack(alignment: .leading, spacing: 0) {
			Eyebrow(WidgetCopy.text("Top model"))
			Text(top.model)
				.font(.system(size: 13, weight: .semibold))
				.lineLimit(1)
				.minimumScaleFactor(0.75)
				.padding(.top, 3)
			Text(WidgetFormat.share(top.share))
				.font(.system(size: 25, weight: .semibold, design: .rounded))
				.foregroundStyle(WidgetPalette.model(1))
				.padding(.top, 3)
			ShareTrack(share: top.share, tone: WidgetPalette.model(1))
				.padding(.top, 3)
			Text("\(WidgetFormat.tokens(top.value.tokens)) \(WidgetCopy.text("of")) \(WidgetFormat.tokens(period.totals.tokens))")
				.font(.system(size: 9.5))
				.foregroundStyle(.secondary)
				.lineLimit(1)
				.padding(.top, 5)
		}
		.widgetAccessibility(WidgetAccessibility.metric(
			label: "\(WidgetCopy.text("Top model")): \(top.model)",
			values: [
				WidgetFormat.share(top.share),
				"\(WidgetFormat.tokens(top.value.tokens)) \(WidgetCopy.text("of")) \(WidgetFormat.tokens(period.totals.tokens)) \(WidgetCopy.text("Tokens"))",
			]
		))
	}

	private func modelList(_ period: DesktopUsagePeriodV1) -> some View {
		VStack(alignment: .leading, spacing: 4) {
			ForEach(Array(period.models.prefix(4).enumerated()), id: \.element.id) { index, item in
				ShareMetricRow(
					label: item.model,
					value: WidgetFormat.tokens(item.value.tokens),
					share: item.share,
					tone: WidgetPalette.model(index),
					showsDot: true
				)
			}
		}
	}

	private func large(_ period: DesktopUsagePeriodV1) -> some View {
		let components = TokenComponent.items(period.totals)
		return VStack(alignment: .leading, spacing: 0) {
			Eyebrow(WidgetCopy.text("Models"))
			modelList(period).padding(.top, 2)
			SectionDivider(
				title: WidgetCopy.text("Token mix"),
				trailing: WidgetCopy.text("Cache write is billed"),
				trailingColor: WidgetPalette.warn
			)
			TokenStackBar(items: components).padding(.top, 7)
			VStack(spacing: 3) {
				ForEach(components) { item in
					TokenMixRow(item: item)
				}
			}
			.padding(.top, 7)
			// The summary is the large surface's bottom anchor. Flexible space
			// belongs above it and collapses before any content is compressed.
			Spacer(minLength: 0)
			ClientSubtotals(period: period.period, model: model)
				.padding(.top, 10)
		}
	}
}

private struct TrustWidgetView: View {
	let model: WidgetSurfaceModel
	let family: WidgetFamily

	@ViewBuilder var body: some View {
		if let headline = model.trustHeadline {
			switch family {
			case .systemMedium:
				medium
			case .systemLarge:
				large(headline)
			default:
				small(headline)
			}
		} else {
			UnavailableWidget(kind: .trust)
		}
	}

	private func small(_ headline: WidgetTrustTier) -> some View {
		VStack(alignment: .leading, spacing: 0) {
			Eyebrow(WidgetCopy.text("Determinable"))
			Text(headlineText(headline))
				.font(.system(size: 25, weight: .semibold, design: .rounded))
				.foregroundStyle(WidgetPalette.good)
				.padding(.top, 3)
			if headline.share != nil {
				ShareTrack(share: headline.share, tone: WidgetPalette.good).padding(.top, 3)
			}
			ViewThatFits(in: .horizontal) {
				Text(secondaryLine).fixedSize(horizontal: true, vertical: false)
				VStack(alignment: .leading, spacing: 1) {
					ForEach(model.trustTiers.filter { $0.quality != "determinable" }) { tier in
						Text("\(quality(tier.quality)) \(amountText(tier))").lineLimit(1)
					}
				}
			}
			.font(.system(size: 9.5))
			.foregroundStyle(.secondary)
			.padding(.top, 5)
			if model.trustCostIncomplete {
				Text(WidgetCopy.text("Cost incomplete"))
					.font(.system(size: 9))
					.foregroundStyle(WidgetPalette.warn)
					.padding(.top, 3)
			}
		}
		.widgetAccessibility(WidgetAccessibility.metric(
			label: WidgetCopy.text("Measurement quality"),
			values: model.trustTiers.map { tier in
				"\(quality(tier.quality)): \(amountText(tier)), \(WidgetFormat.share(tier.share))"
			} + (model.trustCostIncomplete ? [WidgetCopy.text("Cost incomplete")] : [])
		))
	}

	private var medium: some View {
		VStack(alignment: .leading, spacing: 4) {
			ForEach(model.trustTiers) { tier in
				ShareMetricRow(
					label: quality(tier.quality),
					value: amountText(tier),
					share: tier.share,
					tone: WidgetPalette.quality(tier.quality)
				)
			}
			Spacer(minLength: 3)
			if let pricing = model.trustPricing {
				PricingCoverage(coverage: pricing.coverage)
			}
			if model.trustCostIncomplete {
				Text(WidgetCopy.text("Cost incomplete"))
					.font(.system(size: 9))
					.foregroundStyle(WidgetPalette.warn)
					.accessibilityLabel(Text(WidgetCopy.text("Cost incomplete")))
			}
		}
	}

	private func large(_ headline: WidgetTrustTier) -> some View {
		VStack(alignment: .leading, spacing: 0) {
			Eyebrow(WidgetCopy.text("Measurement quality"))
			Text(headlineText(headline))
				.font(.system(size: 27, weight: .semibold, design: .rounded))
				.foregroundStyle(WidgetPalette.good)
				.padding(.top, 3)
				.widgetAccessibility(WidgetAccessibility.metric(
					label: quality(headline.quality),
					values: [amountText(headline), headline.share.map(WidgetFormat.share)]
				))
			Text(WidgetCopy.text("Determinate cost"))
				.font(.system(size: 9.5))
				.foregroundStyle(.secondary)
				.padding(.top, 2)
				.accessibilityHidden(true)
			VStack(spacing: 4) {
				ForEach(model.trustTiers.filter { $0.quality != "determinable" }) { tier in
					ShareMetricRow(
						label: quality(tier.quality),
						value: amountText(tier),
						share: tier.share,
						tone: WidgetPalette.quality(tier.quality)
					)
				}
			}
			.padding(.top, 7)
			SectionDivider(
				title: WidgetCopy.text("By provider"),
				trailing: model.trustPricing.map { WidgetFormat.share($0.coverage) }
			)
			VStack(spacing: 4) {
				ForEach(model.trustProviders) { row in
					ShareMetricRow(
						label: row.provider.capitalized,
						value: amountText(row.tier),
						share: row.tier.share,
						tone: WidgetPalette.good
					)
				}
			}
			.padding(.top, 5)
			if let pricing = model.trustPricing {
				// Keep both complete and incomplete pricing summaries aligned with
				// the other large surfaces' bottom information element.
				Spacer(minLength: 0)
				Group {
					if pricing.incomplete {
						UnpricedNote(identifiers: pricing.unpricedIdentifiers)
					} else {
						PricingCoverage(coverage: pricing.coverage)
							.padding(.horizontal, 9)
							.padding(.vertical, 8)
							.background(WidgetPalette.info.opacity(0.13), in: RoundedRectangle(cornerRadius: 8))
					}
				}
				.padding(.top, 10)
			}
		}
		}

	private var secondaryLine: String {
		model.trustTiers
			.filter { $0.quality != "determinable" }
			.map { "\(quality($0.quality)) \(amountText($0))" }
			.joined(separator: " · ")
	}

	/// The share is the headline only while it is a meaningful measurement.
	private func headlineText(_ tier: WidgetTrustTier) -> String {
		guard tier.share != nil else { return amountText(tier) }
		return WidgetFormat.share(tier.share)
	}

	private func amountText(_ tier: WidgetTrustTier) -> String {
		switch tier.amount {
		case let .cost(text): text
		case let .tokens(text): "\(text) \(WidgetCopy.text("Tokens"))"
		}
	}

	private func quality(_ value: String) -> String {
		switch value {
		case "determinable": WidgetCopy.text("Determinable")
		case "inferred": WidgetCopy.text("Inferred")
		default: WidgetCopy.text("Unattributed")
		}
	}
}

private struct RhythmWidgetView: View {
	let model: WidgetSurfaceModel
	let family: WidgetFamily

	@ViewBuilder var body: some View {
		if let scope = model.scope, scope.rhythm.available {
			switch family {
			case .systemMedium:
				medium(scope)
			case .systemLarge:
				large(scope)
			default:
				small(scope)
			}
		} else {
			UnavailableWidget(kind: .rhythm)
		}
	}

	private func small(_ scope: DesktopUsageScopeV1) -> some View {
		VStack(alignment: .leading, spacing: 0) {
			Eyebrow(WidgetCopy.text("Active days"))
			HStack(alignment: .firstTextBaseline, spacing: 3) {
				Text("\(scope.rhythm.activeDays)")
					.font(.system(size: 25, weight: .semibold, design: .rounded))
					.foregroundStyle(WidgetPalette.accent)
				Text("/ 30")
					.font(.system(size: 13, weight: .semibold))
					.foregroundStyle(.tertiary)
			}
			.padding(.top, 3)
			.widgetAccessibility(WidgetAccessibility.metric(
				label: WidgetCopy.text("Active days"),
				values: ["\(scope.rhythm.activeDays) / 30"]
			))
			Text(busiestDescription(scope.rhythm))
				.font(.system(size: 9.5))
				.foregroundStyle(.secondary)
				.lineLimit(1)
				.minimumScaleFactor(0.78)
				.padding(.top, 5)
				.widgetAccessibility(WidgetAccessibility.metric(
					label: WidgetCopy.text("Busiest"),
					values: [busiestDescription(scope.rhythm)]
				))
		}
	}

	private func medium(_ scope: DesktopUsageScopeV1) -> some View {
		VStack(alignment: .leading, spacing: 0) {
			HStack(alignment: .center, spacing: 9) {
				HourAxis().frame(maxWidth: .infinity)
				HeatLegend()
			}
			.accessibilityHidden(true)
			WeeklyHeatGrid(values: scope.rhythm.intensities, compact: true)
				.padding(.top, 4)
				.widgetAccessibility(WidgetAccessibility.hourGrid(scope.rhythm))
		}
	}

	private func large(_ scope: DesktopUsageScopeV1) -> some View {
		let calendarValues = scope.daily.items.map { WidgetFormat.chartValue($0.value) }
		return VStack(alignment: .leading, spacing: 0) {
			HStack(alignment: .center, spacing: 8) {
				Eyebrow(WidgetCopy.text("Hour of week"))
				Spacer(minLength: 3)
				HeatLegend()
			}
			.accessibilityHidden(true)
			HourAxis().padding(.top, 4)
				.accessibilityHidden(true)
			WeeklyHeatGrid(values: scope.rhythm.intensities, compact: false)
				.padding(.top, 3)
				.widgetAccessibility(WidgetAccessibility.hourGrid(scope.rhythm))
			SectionDivider(
				title: WidgetCopy.text("90-day context"),
				trailing: dateRange(scope.daily.items)
			)
			CalendarGrid(values: calendarValues)
				.padding(.top, 6)
				.widgetAccessibility(WidgetAccessibility.trend(
					items: scope.daily.items,
					label: WidgetCopy.text("90-day context")
				))
			Spacer(minLength: 5)
			StatChipRow(items: [
				StatItem(label: WidgetCopy.text("Active days"), value: "\(scope.rhythm.activeDays) / 30"),
				StatItem(label: WidgetCopy.text("Busiest"), value: WidgetFormat.weekday(scope.rhythm.busiestDay)),
				StatItem(label: WidgetCopy.text("Quietest"), value: WidgetFormat.weekday(scope.rhythm.quietestDay)),
			])
		}
	}

	private func busiestDescription(_ rhythm: DesktopUsageRhythmV1) -> String {
		let weekday = WidgetFormat.weekday(rhythm.busiestDay)
		guard let range = peakHourRange(rhythm) else {
			return "\(WidgetCopy.text("Busiest at")) \(weekday)"
		}
		return "\(WidgetCopy.text("Busiest at")) \(weekday) \(WidgetFormat.hourRange(start: range.lowerBound, end: range.upperBound))"
	}

	private func peakHourRange(_ rhythm: DesktopUsageRhythmV1) -> ClosedRange<Int>? {
		let weekdays = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
		guard let day = weekdays.firstIndex(of: rhythm.busiestDay.lowercased()) else { return nil }
		let startIndex = day * 24
		guard rhythm.intensities.count >= startIndex + 24 else { return nil }
		let row = Array(rhythm.intensities[startIndex ..< startIndex + 24])
		guard let maximum = row.max(), maximum > 0, let start = row.firstIndex(of: maximum) else { return nil }
		var end = start + 1
		while end < row.count, row[end] == maximum {
			end += 1
		}
		return start ... end
	}

	private func dateRange(_ items: [DesktopUsageDailyItemV1]) -> String? {
		guard let first = items.first?.date, let last = items.last?.date else { return nil }
		return "\(WidgetFormat.date(first)) – \(WidgetFormat.date(last))"
	}
}

private struct Eyebrow: View {
	let text: String
	init(_ text: String) { self.text = text }
	var body: some View {
		Text(text)
			.font(.system(size: 9, weight: .semibold))
			.foregroundStyle(.tertiary)
			.tracking(0.35)
			.accessibilityLabel(Text(text))
			.accessibilityAddTraits(.isHeader)
	}
}

private struct PricingFlag: View {
	let unpricedComponents: Int
	var body: some View {
		Text(flagText)
			.font(.system(size: 9))
			.foregroundStyle(WidgetPalette.warn)
			.padding(.horizontal, 6)
			.padding(.vertical, 3)
			.background(WidgetPalette.warn.opacity(0.16), in: RoundedRectangle(cornerRadius: 5))
			.lineLimit(1)
			.accessibilityLabel(Text(flagText))
	}

	private var flagText: String {
		guard unpricedComponents > 0 else { return WidgetCopy.text("Cost incomplete") }
		return "\(WidgetCopy.text("Cost incomplete")) · \(unpricedComponents) \(WidgetCopy.text("unpriced"))"
	}
}

private struct PeriodColumns: View {
	let items: [DesktopUsagePeriodV1]
	let showsTokens: Bool

	var body: some View {
		HStack(alignment: .top, spacing: 8) {
			ForEach(items) { item in
				VStack(alignment: .leading, spacing: 1) {
					Text(WidgetCopy.period(WidgetPeriod(rawValue: item.period) ?? .today))
						.font(.system(size: 9, weight: .semibold))
						.foregroundStyle(.tertiary)
					Text(WidgetFormat.cost(item.totals.providerCost, known: item.totals.knownProviderCost, incomplete: !item.totals.pricingComplete))
						.font(.system(size: 13, weight: .semibold))
						.lineLimit(1)
					if showsTokens {
						Text(WidgetFormat.tokens(item.totals.tokens))
							.font(.system(size: 9))
							.foregroundStyle(.tertiary)
					}
				}
				.frame(maxWidth: .infinity, alignment: .leading)
				.widgetAccessibility(WidgetAccessibility.metric(
					label: WidgetCopy.period(WidgetPeriod(rawValue: item.period) ?? .today),
					values: [
						WidgetFormat.cost(item.totals.providerCost, known: item.totals.knownProviderCost, incomplete: !item.totals.pricingComplete),
						showsTokens ? "\(WidgetFormat.tokens(item.totals.tokens)) \(WidgetCopy.text("Tokens"))" : nil,
					]
				))
			}
		}
	}
}

private struct MiniBarChart: View {
	let values: ArraySlice<Double>
	let tone: Color

	init(values: ArraySlice<Double>, tone: Color) {
		self.values = values
		self.tone = tone
	}

	init(values: [Double], tone: Color) {
		self.values = values[...]
		self.tone = tone
	}

	var body: some View {
		GeometryReader { geometry in
			let maximum = max(values.max() ?? 0, 0.001)
			HStack(alignment: .bottom, spacing: 2) {
				ForEach(Array(values.enumerated()), id: \.offset) { _, value in
					RoundedRectangle(cornerRadius: 1.5)
						.fill(tone.opacity(0.88))
						.frame(maxWidth: .infinity)
						.frame(height: max(3, geometry.size.height * value / maximum))
				}
			}
		}
		.frame(minHeight: 16)
	}
}

private struct AreaSeriesChart: View {
	let values: [Double]

	var body: some View {
		GeometryReader { geometry in
			let points = chartPoints(size: geometry.size)
			ZStack {
				Path { path in
					guard let first = points.first, let last = points.last else { return }
					path.move(to: CGPoint(x: first.x, y: geometry.size.height))
					path.addLine(to: first)
					points.dropFirst().forEach { path.addLine(to: $0) }
					path.addLine(to: CGPoint(x: last.x, y: geometry.size.height))
					path.closeSubpath()
				}
				.fill(WidgetPalette.info.opacity(0.16))
				Path { path in
					guard let first = points.first else { return }
					path.move(to: first)
					points.dropFirst().forEach { path.addLine(to: $0) }
				}
				.stroke(WidgetPalette.info, style: StrokeStyle(lineWidth: 1.5, lineJoin: .round))
			}
		}
	}

	private func chartPoints(size: CGSize) -> [CGPoint] {
		guard !values.isEmpty else { return [] }
		let maximum = max(values.max() ?? 0, 0.001)
		let divisor = Double(max(values.count - 1, 1))
		return values.enumerated().map { index, value in
			CGPoint(x: size.width * Double(index) / divisor, y: size.height * (1 - value / maximum))
		}
	}
}

private struct DateAxis: View {
	let first: String?
	let last: String?
	var body: some View {
		HStack {
			Text(first.map(WidgetFormat.date) ?? "—")
			Spacer()
			Text(last.map(WidgetFormat.date) ?? "—")
		}
		.font(.system(size: 9))
		.foregroundStyle(.tertiary)
		.padding(.top, 3)
	}
}

private struct ShareMetricRow: View {
	let label: String
	let value: String
	let share: String?
	let tone: Color
	var showsDot = false

	var body: some View {
		VStack(spacing: 2) {
			HStack(alignment: .firstTextBaseline, spacing: 6) {
				HStack(spacing: 5) {
					if showsDot { Circle().fill(tone).frame(width: 6, height: 6) }
					Text(label).lineLimit(1)
				}
				.frame(maxWidth: .infinity, alignment: .leading)
				Text(value).foregroundStyle(.secondary).lineLimit(1)
				Text(WidgetFormat.share(share))
					.fontWeight(.semibold)
					.frame(width: 42, alignment: .trailing)
			}
			.font(.system(size: 10))
			if share != nil { ShareTrack(share: share, tone: tone) }
		}
		.widgetAccessibility(WidgetAccessibility.metric(
			label: label,
			values: [value, share.map { WidgetFormat.share($0) }]
		))
	}
}

private struct ShareTrack: View {
	let share: String?
	let tone: Color
	var body: some View {
		GeometryReader { geometry in
			let percentage = min(max(WidgetFormat.decimal(share), 0), 100)
			ZStack(alignment: .leading) {
				Capsule().fill(WidgetPalette.track)
				Capsule().fill(tone).frame(width: geometry.size.width * max(percentage, 1.5) / 100)
			}
		}
		.frame(height: 3)
		.accessibilityHidden(true)
	}
}

private struct PricingCoverage: View {
	let coverage: String
	var body: some View {
		HStack(spacing: 8) {
			HStack {
				Text(WidgetCopy.text("Pricing coverage"))
				Spacer()
				Text(WidgetFormat.share(coverage)).fontWeight(.semibold).foregroundStyle(WidgetPalette.info)
			}
			.frame(maxWidth: .infinity)
			ShareTrack(share: coverage, tone: WidgetPalette.info).frame(width: 46)
		}
		.font(.system(size: 10))
		.widgetAccessibility(WidgetAccessibility.metric(
			label: WidgetCopy.text("Pricing coverage"),
			values: [WidgetFormat.share(coverage)]
		))
	}
}

private struct SectionDivider: View {
	let title: String
	let trailing: String?
	var trailingColor: Color = .secondary

	var body: some View {
		HStack(alignment: .firstTextBaseline, spacing: 8) {
			Eyebrow(title)
			Spacer(minLength: 3)
			if let trailing {
				Text(trailing).font(.system(size: 9)).foregroundStyle(trailingColor).lineLimit(1)
			}
		}
		.padding(.top, 7)
		.overlay(alignment: .top) { Rectangle().fill(WidgetPalette.divider).frame(height: 1) }
		.padding(.top, 8)
		.widgetAccessibility(WidgetAccessibility.metric(label: title, values: [trailing]))
	}
}

private struct TokenComponent: Identifiable {
	let id: String
	let label: String
	let value: Int64
	let share: String?
	let color: Color

	static func items(_ totals: DesktopPresentationTotalsV1) -> [TokenComponent] {
		let values: [(String, String, Int64)] = [
			("input", "Input", totals.inputTokens),
			("output", "Output", totals.outputTokens),
			("cache-read", "Cache read", totals.cachedReadTokens),
			("cache-write", "Cache write", totals.cacheWriteTokens),
		]
		let total = values.reduce(Int64(0)) { $0 + $1.2 }
		return values.enumerated().map { index, item in
			TokenComponent(
				id: item.0,
				label: WidgetCopy.text(item.1),
				value: item.2,
				share: WidgetFormat.percentage(item.2, total: total),
				color: WidgetPalette.model(index)
			)
		}
	}
}

private struct TokenStackBar: View {
	let items: [TokenComponent]
	var body: some View {
		GeometryReader { geometry in
			ZStack(alignment: .leading) {
				ForEach(Array(items.enumerated()), id: \.element.id) { index, item in
					let preceding = items.prefix(index).reduce(0.0) { $0 + WidgetFormat.decimal($1.share) }
					RoundedRectangle(cornerRadius: 2)
						.fill(item.color)
						.frame(width: geometry.size.width * WidgetFormat.decimal(item.share) / 100)
						.offset(x: geometry.size.width * preceding / 100)
				}
			}
		}
		.frame(height: 6)
		.clipShape(RoundedRectangle(cornerRadius: 3))
		.widgetAccessibility(WidgetAccessibility.metric(
			label: WidgetCopy.text("Token mix"),
			values: items.map { item in
				"\(item.label): \(WidgetFormat.tokens(item.value)) \(WidgetCopy.text("Tokens")), \(WidgetFormat.share(item.share))"
			}
		))
	}
}

private struct TokenMixRow: View {
	let item: TokenComponent
	var body: some View {
		HStack(spacing: 7) {
			HStack(spacing: 5) {
				Circle().fill(item.color).frame(width: 6, height: 6)
				Text(item.label)
			}
			.frame(maxWidth: .infinity, alignment: .leading)
			Text(WidgetFormat.tokens(item.value)).foregroundStyle(.secondary)
			Text(WidgetFormat.share(item.share)).foregroundStyle(.tertiary).frame(width: 40, alignment: .trailing)
		}
		.font(.system(size: 10))
		.widgetAccessibility(WidgetAccessibility.metric(
			label: item.label,
			values: [
				"\(WidgetFormat.tokens(item.value)) \(WidgetCopy.text("Tokens"))",
				WidgetFormat.share(item.share),
			]
		))
	}
}

private struct ClientSubtotals: View {
	let period: String
	let model: WidgetSurfaceModel
	var body: some View {
		HStack(spacing: 8) {
			Text(WidgetCopy.text("Client subtotals")).foregroundStyle(.tertiary)
			Spacer(minLength: 3)
			ForEach(items) { item in
				Text("\(item.client.capitalized) \(WidgetFormat.cost(item.value.providerCost, known: item.value.providerCost, incomplete: item.value.costIncomplete))")
					.fontWeight(.semibold)
					.lineLimit(1)
			}
		}
		.font(.system(size: 9.5))
		.padding(.horizontal, 8)
		.padding(.vertical, 7)
		.background(WidgetPalette.secondarySurface, in: RoundedRectangle(cornerRadius: 7))
		.widgetAccessibility(WidgetAccessibility.metric(
			label: WidgetCopy.text("Client subtotals"),
			values: items.map { item in
				"\(item.client.capitalized): \(WidgetFormat.cost(item.value.providerCost, known: item.value.providerCost, incomplete: item.value.costIncomplete))"
			}
		))
	}

	private var items: [DesktopClientSubtotalV1] {
		model.entry.snapshot?.usage.presentation.clientSubtotals.items.filter { $0.period == period } ?? []
	}
}

private struct UnpricedNote: View {
	let identifiers: [String]
	var body: some View {
		VStack(alignment: .leading, spacing: 2) {
			Text(WidgetCopy.text("Unpriced identifiers")).font(.system(size: 9)).foregroundStyle(WidgetPalette.warn)
			Text(identifiers.first ?? "—").font(.system(size: 10, weight: .semibold)).lineLimit(1)
			Text(WidgetCopy.text("Cost remains visibly incomplete")).font(.system(size: 9)).foregroundStyle(.tertiary)
		}
		.padding(.horizontal, 9)
		.padding(.vertical, 8)
		.frame(maxWidth: .infinity, alignment: .leading)
		.background(WidgetPalette.warn.opacity(0.13), in: RoundedRectangle(cornerRadius: 8))
		.widgetAccessibility(WidgetAccessibility.metric(
			label: WidgetCopy.text("Unpriced identifiers"),
			values: [identifiers.first ?? "—", WidgetCopy.text("Cost remains visibly incomplete")]
		))
	}
}

private struct HourAxis: View {
	var body: some View {
		HStack {
			ForEach(["00", "06", "12", "18", "24"], id: \.self) { mark in
				Text(mark)
				if mark != "24" { Spacer() }
			}
		}
		.font(.system(size: 9))
		.foregroundStyle(.tertiary)
		.padding(.leading, 15)
		.accessibilityHidden(true)
	}
}

private struct HeatLegend: View {
	var body: some View {
		HStack(spacing: 3) {
			Text(WidgetCopy.text("Low"))
			ForEach(0 ..< 6, id: \.self) { level in
				RoundedRectangle(cornerRadius: 1).fill(WidgetPalette.heat(level)).frame(width: 6, height: 6)
			}
			Text(WidgetCopy.text("High"))
		}
		.font(.system(size: 8))
		.foregroundStyle(.tertiary)
		.accessibilityHidden(true)
	}
}

private struct WeeklyHeatGrid: View {
	let values: [Int]
	let compact: Bool
	private let dayOrder = Array(0 ..< 7)

	var body: some View {
		VStack(spacing: compact ? 1 : 2) {
			ForEach(dayOrder, id: \.self) { day in
				HStack(spacing: 4) {
					Text(weekday(day)).font(.system(size: 9)).foregroundStyle(.tertiary).frame(width: 11, alignment: .leading)
					HStack(spacing: 2) {
						ForEach(0 ..< 24, id: \.self) { hour in
							RoundedRectangle(cornerRadius: 1.5)
								.fill(WidgetPalette.heat(level(value(day: day, hour: hour))))
								.frame(maxWidth: .infinity)
								.frame(height: compact ? 5 : 8)
						}
					}
				}
			}
		}
	}

	private func value(day: Int, hour: Int) -> Int {
		let index = day * 24 + hour
		return values.indices.contains(index) ? values[index] : 0
	}

	private func level(_ value: Int) -> Int {
		guard value > 0 else { return 0 }
		return min(5, max(1, Int(ceil(Double(value) / 20))))
	}

	private func weekday(_ day: Int) -> String {
		let names = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
		return WidgetFormat.weekday(names[day], short: true)
	}
}

private struct CalendarGrid: View {
	let values: [Double]
	private let columns = Array(repeating: GridItem(.flexible(), spacing: 2), count: 18)

	var body: some View {
		LazyVGrid(columns: columns, spacing: 2) {
			ForEach(Array(values.enumerated()), id: \.offset) { _, value in
				RoundedRectangle(cornerRadius: 1.5)
					.fill(WidgetPalette.heat(level(value)))
					.frame(height: 7)
			}
		}
	}

	private func level(_ value: Double) -> Int {
		let maximum = max(values.max() ?? 0, 0.001)
		guard value > 0 else { return 0 }
		return min(5, max(1, Int(ceil(value / maximum * 5))))
	}
}

private struct StatItem: Identifiable {
	var id: String { label }
	let label: String
	let value: String
}

private struct StatChipRow: View {
	let items: [StatItem]
	var body: some View {
		HStack(spacing: 5) {
			ForEach(items) { item in StatChip(item: item) }
		}
	}
}

private struct StatChip: View {
	let item: StatItem
	var body: some View {
		VStack(alignment: .leading, spacing: 1) {
			Text(item.label).font(.system(size: 9)).foregroundStyle(.tertiary).lineLimit(1)
			Text(item.value).font(.system(size: 11, weight: .semibold)).lineLimit(1).minimumScaleFactor(0.75)
		}
		.padding(.horizontal, 6)
		.padding(.vertical, 5)
		.frame(maxWidth: .infinity, alignment: .leading)
		.background(WidgetPalette.secondarySurface, in: RoundedRectangle(cornerRadius: 7))
		.widgetAccessibility(WidgetAccessibility.metric(label: item.label, values: [item.value]))
	}
}

private struct UnavailableWidget: View {
	let kind: AgentDeckWidgetKind
	var body: some View {
		VStack(alignment: .leading, spacing: 8) {
			Image(systemName: "exclamationmark.triangle").accessibilityHidden(true)
			Text(WidgetCopy.text("Data unavailable")).font(.headline)
			Text(WidgetCopy.text(kind.emptyKey)).font(.caption).foregroundStyle(.secondary)
		}
		.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
		.widgetAccessibility(WidgetAccessibility.metric(
			label: WidgetCopy.text("Data unavailable"),
			values: [WidgetCopy.text(kind.emptyKey)]
		))
	}
}
