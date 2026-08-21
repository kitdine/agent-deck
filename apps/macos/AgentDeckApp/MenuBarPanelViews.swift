import SwiftUI

/// A panel whose own data is absent shows this in place of its values rather
/// than an empty header, and never lets the whole surface claim emptiness.
struct UnavailableRow: View {
	var body: some View {
		Label(t(DesktopCopy.sectionUnavailable), systemImage: "minus.circle")
			.font(.caption)
			.foregroundStyle(DesktopVisualTheme.muted)
			.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
			.fixedSize(horizontal: false, vertical: true)
	}
}

struct SectionHeader: View {
	let title: String
	var trailing: String?

	var body: some View {
		HStack(alignment: .firstTextBaseline) {
			Text(title)
				.font(.subheadline.weight(.semibold))
				.foregroundStyle(DesktopVisualTheme.muted)
			Spacer()
			if let trailing {
				Text(trailing)
					.font(.caption)
					.foregroundStyle(DesktopVisualTheme.dim)
					.monospacedDigit()
					.lineLimit(1)
					.minimumScaleFactor(0.75)
			}
		}
		.accessibilityElement(children: .combine)
	}
}

struct CollapsibleSection<Content: View>: View {
	let title: String
	var trailing: String?
	private let content: Content
	@Binding private var isExpanded: Bool

	init(
		title: String,
		trailing: String? = nil,
		isExpanded: Binding<Bool>,
		@ViewBuilder content: () -> Content
	) {
		self.title = title
		self.trailing = trailing
		self.content = content()
		_isExpanded = isExpanded
	}

	var body: some View {
		VStack(alignment: .leading, spacing: 0) {
			Button {
				isExpanded.toggle()
			} label: {
				HStack(alignment: .firstTextBaseline, spacing: MenuBarGeometry.betweenRows) {
					SectionHeader(title: title, trailing: trailing)
					Image(systemName: isExpanded ? "chevron.down" : "chevron.right")
						.font(.caption.weight(.semibold))
						.foregroundStyle(DesktopVisualTheme.dim)
						.accessibilityHidden(true)
				}
				.frame(maxWidth: .infinity, minHeight: MenuBarGeometry.rowMinimumHeight, alignment: .leading)
				.contentShape(Rectangle())
			}
			.buttonStyle(.plain)
			.accessibilityLabel(title)
			.accessibilityAddTraits(.isButton)
			if isExpanded {
				content.padding(.top, MenuBarGeometry.betweenRows)
			}
		}
		.desktopCard()
		.transaction { $0.animation = nil }
	}
}

typealias SectionExpansion = (String) -> Binding<Bool>

private struct EqualHeightRow: Layout {
	let spacing: CGFloat

	func sizeThatFits(
		proposal: ProposedViewSize,
		subviews: Subviews,
		cache _: inout ()
	) -> CGSize {
		guard !subviews.isEmpty else { return .zero }
		let totalSpacing = spacing * CGFloat(max(0, subviews.count - 1))
		if let width = proposal.width {
			let cellWidth = max(0, (width - totalSpacing) / CGFloat(subviews.count))
			let height = subviews.map {
				$0.sizeThatFits(ProposedViewSize(width: cellWidth, height: nil)).height
			}.max() ?? 0
			return CGSize(width: width, height: height)
		}
		let sizes = subviews.map { $0.sizeThatFits(.unspecified) }
		return CGSize(
			width: sizes.map(\.width).reduce(0, +) + totalSpacing,
			height: sizes.map(\.height).max() ?? 0
		)
	}

	func placeSubviews(
		in bounds: CGRect,
		proposal _: ProposedViewSize,
		subviews: Subviews,
		cache _: inout ()
	) {
		guard !subviews.isEmpty else { return }
		let totalSpacing = spacing * CGFloat(max(0, subviews.count - 1))
		let cellWidth = max(0, (bounds.width - totalSpacing) / CGFloat(subviews.count))
		for (index, subview) in subviews.enumerated() {
			let x = bounds.minX + CGFloat(index) * (cellWidth + spacing)
			subview.place(
				at: CGPoint(x: x, y: bounds.minY),
				anchor: .topLeading,
				proposal: ProposedViewSize(width: cellWidth, height: bounds.height)
			)
		}
	}
}

struct StatChipRow: View {
	let chips: [StatChip]

	var body: some View {
		EqualHeightRow(spacing: MenuBarGeometry.betweenRows) {
			ForEach(chips) { chip in
				VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
					Text(chip.label)
						.font(.caption)
						.foregroundStyle(.secondary)
						.lineLimit(1)
						.minimumScaleFactor(0.72)
					Text(chip.value)
						.font(.body)
						.monospacedDigit()
						.lineLimit(1)
						.minimumScaleFactor(0.7)
					if let note = chip.note {
						Text(note)
							.font(.caption)
							.foregroundStyle(.secondary)
							.lineLimit(1)
							.minimumScaleFactor(0.72)
					}
				}
				.padding(MenuBarGeometry.betweenRows)
				.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
				.background(DesktopVisualTheme.surfaceEmphasis, in: RoundedRectangle(cornerRadius: 7, style: .continuous))
				.accessibilityElement(children: .combine)
			}
		}
	}
}

struct ShareRowsView: View {
	let rows: [ShareRow]

	var body: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
			ForEach(Array(rows.enumerated()), id: \.element.id) { index, row in
				VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
					HStack {
						Text(row.label).font(.body).lineLimit(1).truncationMode(.middle)
						Spacer()
						Text(row.value).font(.body).monospacedDigit().foregroundStyle(.secondary)
						Text(row.shareText)
							.font(.body)
							.monospacedDigit()
							.foregroundStyle(.secondary)
							.frame(minWidth: 52, alignment: .trailing)
							.lineLimit(1)
							.minimumScaleFactor(0.7)
					}
					.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
					GeometryReader { geometry in
						Capsule()
							.fill(DesktopVisualTheme.series[index % DesktopVisualTheme.series.count].opacity(0.68))
							.frame(width: max(0, geometry.size.width * row.share), height: 5)
					}
					.frame(height: 5)
					.background(DesktopVisualTheme.line, in: Capsule())
					.accessibilityHidden(true)
				}
				.accessibilityElement(children: .combine)
			}
		}
	}
}

struct TrendChartInteraction: Equatable {
	var hoveredBucketID: String?
	var pinnedBucketID: String?
	var selectedIndex = 0

	mutating func setHover(_ id: String, inside: Bool) {
		hoveredBucketID = inside ? id : hoveredBucketID == id ? nil : hoveredBucketID
	}

	mutating func togglePin(_ id: String) {
		pinnedBucketID = pinnedBucketID == id ? nil : id
	}

	mutating func move(by delta: Int, bucketCount: Int) {
		guard bucketCount > 0 else { return }
		selectedIndex = min(bucketCount - 1, max(0, selectedIndex + delta))
	}

	func activeID(bucketIDs: [String], focused: Bool) -> String? {
		let focusedID = focused && bucketIDs.indices.contains(selectedIndex) ? bucketIDs[selectedIndex] : nil
		return pinnedBucketID ?? hoveredBucketID ?? focusedID
	}

	static func heightFraction(magnitude: Double, maximum: Double) -> Double {
		guard maximum > 0 else { return 0 }
		return min(1, max(0, magnitude / maximum))
	}

	static func hourlyAxis(bucketIDs: [String], nowLabel: String) -> TrendChartAxis? {
		let hours = bucketIDs.compactMap { id -> Int? in
			guard id.hasPrefix("hour.") else { return nil }
			return Int(id.dropFirst("hour.".count))
		}
		guard !hours.isEmpty,
			hours.count == bucketIDs.count,
			hours.enumerated().allSatisfy({ offset, hour in hour == offset }),
			let last = hours.last
		else {
			return nil
		}
		let middleHour = last >= 2 ? (last + 1) / 2 : nil
		return TrendChartAxis(
			leading: "00:00",
			middle: middleHour.map(hourTick),
			trailing: nowLabel
		)
	}

	private static func hourTick(_ hour: Int) -> String {
		hour < 10 ? "0\(hour):00" : "\(hour):00"
	}
}

struct TrendChartAxis: Equatable, Sendable {
	let leading: String
	let middle: String?
	let trailing: String
}

struct TrendChart: View {
	let buckets: [TrendBucket]
	var height: CGFloat = 132
	@State private var interaction = TrendChartInteraction()
	@FocusState private var chartFocused: Bool

	var body: some View {
		let maximum = max(0.000_001, buckets.map(\.magnitude).max() ?? 0)
		let peak = buckets.max(by: { $0.magnitude < $1.magnitude })
		let active = activeBucket
		let hourlyAxis = TrendChartInteraction.hourlyAxis(
			bucketIDs: buckets.map(\.id),
			nowLabel: t(DesktopCopy.trendNow)
		)
		VStack(alignment: .leading, spacing: 0) {
			HStack(alignment: .bottom, spacing: 2) {
				ForEach(buckets) { bucket in
					Button {
						interaction.togglePin(bucket.id)
					} label: {
						VStack(spacing: 0) {
							Spacer(minLength: 0)
							RoundedRectangle(cornerRadius: 2)
								.fill(
									bucket.id == peak?.id
										? DesktopVisualTheme.accent
										: DesktopVisualTheme.info.opacity(active?.id == bucket.id ? 1 : 0.78)
								)
								.frame(
									height: max(
										3,
										height * CGFloat(TrendChartInteraction.heightFraction(
											magnitude: bucket.magnitude,
											maximum: maximum
										))
									)
								)
						}
						.frame(maxWidth: .infinity, maxHeight: .infinity)
						.contentShape(Rectangle())
					}
					.buttonStyle(.plain)
					.focusable(false)
					.onHover { inside in
						interaction.setHover(bucket.id, inside: inside)
					}
					.accessibilityLabel(bucket.label)
					.accessibilityValue(bucket.accessibilityValue)
				}
			}
			.frame(height: height)
			.overlay(alignment: .bottom) {
				Rectangle().fill(DesktopVisualTheme.line).frame(height: 1)
			}
			.focusable()
			.focused($chartFocused)
			.focusEffectDisabled()
			.onKeyPress(.leftArrow) {
				moveSelection(by: -1)
				return .handled
			}
			.onKeyPress(.rightArrow) {
				moveSelection(by: 1)
				return .handled
			}

			if let hourlyAxis {
				HStack {
					Text(hourlyAxis.leading)
					Spacer()
					if let middle = hourlyAxis.middle {
						Text(middle)
						Spacer()
					}
					Text(hourlyAxis.trailing)
				}
				.font(.caption2)
				.monospacedDigit()
				.foregroundStyle(DesktopVisualTheme.dim)
				.padding(.top, MenuBarGeometry.withinRow)
			} else if let first = buckets.first, let last = buckets.last {
				HStack {
					Text(first.label)
					Spacer()
					Text(last.label)
				}
				.font(.caption2)
				.foregroundStyle(DesktopVisualTheme.dim)
				.padding(.top, MenuBarGeometry.withinRow)
			}

			HStack(spacing: MenuBarGeometry.betweenRows) {
				if let active {
					Text(active.label).font(.caption.weight(.semibold))
					Text(active.cost).font(.caption).foregroundStyle(DesktopVisualTheme.muted)
					Text(DesktopFormat.tokens(active.tokens)).font(.caption).foregroundStyle(DesktopVisualTheme.muted)
					Text(t(DesktopCopy.trendEvents, active.events)).font(.caption).foregroundStyle(DesktopVisualTheme.muted)
					if interaction.pinnedBucketID != nil {
						Circle().fill(DesktopVisualTheme.accent).frame(width: 6, height: 6)
							.accessibilityHidden(true)
					}
				} else if let peak {
					Text("\(t(DesktopCopy.chipPeak)) \(peak.label) · \(peak.cost)")
						.font(.caption)
						.foregroundStyle(DesktopVisualTheme.dim)
				}
			}
			.frame(maxWidth: .infinity, minHeight: 26, maxHeight: 26, alignment: .leading)
			.padding(.horizontal, MenuBarGeometry.betweenRows)
			.background(DesktopVisualTheme.surfaceEmphasis, in: RoundedRectangle(cornerRadius: 7))
			.accessibilityElement(children: .combine)
		}
	}

	private var activeBucket: TrendBucket? {
		let id = interaction.activeID(bucketIDs: buckets.map(\.id), focused: chartFocused)
		return buckets.first(where: { $0.id == id })
	}

	private func moveSelection(by delta: Int) {
		interaction.move(by: delta, bucketCount: buckets.count)
	}
}

struct UsagePanelView: View {
	let panel: UsagePanelModel

	var body: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
			if panel.available {
				VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
					TrendChart(buckets: panel.buckets)
					if let emptyCopy = panel.emptyCopy {
						Text(emptyCopy)
							.font(.caption)
							.foregroundStyle(DesktopVisualTheme.muted)
							.frame(maxWidth: .infinity, alignment: .center)
							.fixedSize(horizontal: false, vertical: true)
					}
					StatChipRow(chips: panel.chips)
				}
				.desktopCard()
			} else {
				UnavailableRow()
			}
		}
	}
}

struct BreakdownPanelView: View {
	let panel: BreakdownPanelModel
	let expansion: SectionExpansion

	init(panel: BreakdownPanelModel, expansion: @escaping SectionExpansion = { _ in .constant(true) }) {
		self.panel = panel
		self.expansion = expansion
	}

	var body: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenSections) {
			CollapsibleSection(title: t(DesktopCopy.modelsTitle), isExpanded: expansion("breakdown.models")) {
				if !panel.available {
					UnavailableRow()
				} else if let emptyCopy = panel.modelsEmptyCopy {
					Text(emptyCopy)
						.font(.caption)
						.foregroundStyle(.secondary)
						.fixedSize(horizontal: false, vertical: true)
				} else {
					ShareRowsView(rows: panel.models)
				}
			}
			if panel.available {
				CollapsibleSection(title: t(DesktopCopy.tokenMixTitle), trailing: t(DesktopCopy.tokenMixNote), isExpanded: expansion("breakdown.token-mix")) {
					ShareRowsView(rows: panel.tokenMix)
				}
				CollapsibleSection(title: t(DesktopCopy.perClientTitle), isExpanded: expansion("breakdown.per-client")) {
					if panel.clientRows.isEmpty {
						UnavailableRow()
					} else {
						ShareRowsView(rows: panel.clientRows)
					}
				}
			}
		}
	}
}

struct AttributionPanelView: View {
	let panel: AttributionPanelModel
	let expansion: SectionExpansion

	init(panel: AttributionPanelModel, expansion: @escaping SectionExpansion = { _ in .constant(true) }) {
		self.panel = panel
		self.expansion = expansion
	}

	var body: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenSections) {
			CollapsibleSection(title: t(DesktopCopy.qualityTitle), trailing: t(DesktopCopy.qualityAllProviders), isExpanded: expansion("attribution.quality")) {
				if panel.qualityAvailable {
					ShareRowsView(rows: panel.tiers)
				} else {
					UnavailableRow()
				}
			}
			if !panel.providerGroups.isEmpty {
				CollapsibleSection(title: t(DesktopCopy.qualityByProvider), isExpanded: expansion("attribution.providers")) {
					ForEach(panel.providerGroups) { group in
						VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
							Text(group.title).font(.body).lineLimit(1)
							ShareRowsView(rows: group.tiers)
						}
					}
				}
			}
			CollapsibleSection(title: t(DesktopCopy.pricingTitle), trailing: panel.pricingAvailable ? panel.pricingHeadline : nil, isExpanded: expansion("attribution.pricing")) {
				if panel.pricingAvailable {
					GeometryReader { geometry in
						Capsule()
							.fill(.tint)
							.frame(width: max(0, geometry.size.width * panel.pricingCoverage), height: 5)
					}
					.frame(height: 5)
					.background(.quaternary, in: Capsule())
					.accessibilityLabel(t(DesktopCopy.pricingTitle))
					.accessibilityValue(panel.pricingHeadline)
					if !panel.unpricedIdentifiers.isEmpty {
						Text(t(DesktopCopy.pricingUnpriced, panel.unpricedIdentifiers.joined(separator: " · ")))
							.font(.caption)
							.foregroundStyle(.secondary)
							.textSelection(.enabled)
							.fixedSize(horizontal: false, vertical: true)
					}
				} else {
					UnavailableRow()
				}
			}
		}
	}
}

struct SessionsPanelView: View {
	let panel: SessionsPanelModel
	let expansion: SectionExpansion

	init(panel: SessionsPanelModel, expansion: @escaping SectionExpansion = { _ in .constant(true) }) {
		self.panel = panel
		self.expansion = expansion
	}

	var body: some View {
			VStack(alignment: .leading, spacing: MenuBarGeometry.betweenSections) {
				if panel.available {
					StatChipRow(chips: panel.stats)
					VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
						HStack(alignment: .firstTextBaseline) {
							Text(t(DesktopCopy.sessionsSignals))
								.font(.caption.weight(.semibold))
								.foregroundStyle(DesktopVisualTheme.muted)
							Spacer()
							Text(t(DesktopCopy.notCapturedYet))
								.font(.caption2.weight(.medium))
								.foregroundStyle(DesktopVisualTheme.warning)
								.padding(.horizontal, 6)
								.padding(.vertical, 2)
								.background(DesktopVisualTheme.warning.opacity(0.14), in: Capsule())
						}
						HStack(alignment: .top, spacing: MenuBarGeometry.betweenRows) {
							WorkSignalPlaceholder(
								title: t(DesktopCopy.sessionsActivity),
								symbol: "chevron.left.forwardslash.chevron.right",
								tint: DesktopVisualTheme.info
							)
							WorkSignalPlaceholder(
								title: t(DesktopCopy.sessionsWorkflow),
								symbol: "doc.text",
								tint: DesktopVisualTheme.series[1]
							)
							WorkSignalPlaceholder(
								title: t(DesktopCopy.sessionsTooling),
								symbol: "wrench.and.screwdriver",
								tint: DesktopVisualTheme.accent
							)
						}
					}
					CollapsibleSection(title: t(DesktopCopy.sessionsProjectRows), isExpanded: expansion("sessions.projects")) {
					if let emptyCopy = panel.emptyCopy {
						Text(emptyCopy)
							.font(.caption)
							.foregroundStyle(.secondary)
							.fixedSize(horizontal: false, vertical: true)
					} else {
							SessionProjectRowsView(rows: panel.projects)
						}
					}
					if !panel.recent.isEmpty {
					CollapsibleSection(title: t(DesktopCopy.sessionsRecent), isExpanded: expansion("sessions.recent")) {
						ForEach(Array(panel.recent.enumerated()), id: \.element.id) { index, row in
							HStack(alignment: .firstTextBaseline) {
								VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
									Text(row.title).font(.body).lineLimit(1).truncationMode(.middle)
									Text(row.detail).font(.caption).foregroundStyle(.secondary).lineLimit(1)
								}
								Spacer()
								Text(row.when).font(.caption).foregroundStyle(.secondary)
							}
							.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
							.accessibilityElement(children: .combine)
							if index < panel.recent.count - 1 {
								Divider().overlay(DesktopVisualTheme.lineSoft)
							}
							}
						}
					}
			} else {
				UnavailableRow()
			}
		}
	}
}

private struct SessionProjectRowsView: View {
	let rows: [ShareRow]

	var body: some View {
		VStack(alignment: .leading, spacing: 0) {
			ForEach(Array(rows.enumerated()), id: \.element.id) { index, row in
				HStack(alignment: .firstTextBaseline, spacing: MenuBarGeometry.betweenRows) {
					Text(row.label)
						.font(.body)
						.lineLimit(1)
						.truncationMode(.middle)
					Spacer()
					Text(row.value)
						.font(.caption)
						.foregroundStyle(DesktopVisualTheme.dim)
						.lineLimit(1)
					Text(row.shareText)
						.font(.body.weight(.semibold))
						.monospacedDigit()
						.frame(minWidth: 74, alignment: .trailing)
						.lineLimit(1)
				}
				.frame(minHeight: 32)
				.accessibilityElement(children: .combine)
				if index < rows.count - 1 {
					Divider().overlay(DesktopVisualTheme.lineSoft)
				}
			}
		}
	}
}

private struct WorkSignalPlaceholder: View {
	let title: String
	let symbol: String
	let tint: Color

	var body: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
			HStack(spacing: MenuBarGeometry.withinRow) {
				Image(systemName: symbol).foregroundStyle(tint)
				Text(title).font(.caption.weight(.semibold)).lineLimit(1)
			}
		}
		.padding(MenuBarGeometry.betweenRows)
		.frame(maxWidth: .infinity, alignment: .leading)
		.background(DesktopVisualTheme.surfaceEmphasis, in: RoundedRectangle(cornerRadius: 7))
		.accessibilityElement(children: .combine)
	}
}

struct RhythmHoverState: Equatable {
	var hourCellID: String?
	var calendarBucketID: String?

	mutating func setHour(_ id: String, inside: Bool) {
		hourCellID = inside ? id : hourCellID == id ? nil : hourCellID
	}

	mutating func setCalendar(_ id: String, inside: Bool) {
		calendarBucketID = inside ? id : calendarBucketID == id ? nil : calendarBucketID
	}
}

/// Below the filtered panels, stating its own fixed window. Neither filter
/// changes it, and the scope line is what keeps that honest where the user is
/// looking.
struct RhythmBlockView: View {
	let block: RhythmBlockModel
	let expansion: SectionExpansion
	@State private var hover = RhythmHoverState()

	init(
		block: RhythmBlockModel,
		initialHover: RhythmHoverState = RhythmHoverState(),
		expansion: @escaping SectionExpansion = { _ in .constant(true) }
	) {
		self.block = block
		self.expansion = expansion
		_hover = State(initialValue: initialHover)
	}

	var body: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
			HStack(alignment: .firstTextBaseline, spacing: MenuBarGeometry.betweenRows) {
				HStack(spacing: MenuBarGeometry.withinRow) {
					Image(systemName: "clock.arrow.circlepath")
						.foregroundStyle(DesktopVisualTheme.accent)
						.accessibilityHidden(true)
					Text(t(DesktopCopy.rhythmTitle))
						.font(.subheadline.weight(.semibold))
				}
				Spacer()
				Text(block.scopeLine)
					.font(.caption2)
					.foregroundStyle(DesktopVisualTheme.dim)
					.lineLimit(1)
					.minimumScaleFactor(0.8)
			}
				if block.available {
					StatChipRow(chips: block.figures)
					CollapsibleSection(title: t(DesktopCopy.rhythmHourOfWeek), trailing: hoveredHourReadout, isExpanded: expansion("rhythm.hour-of-week")) {
						VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
						hourAxis
						grid
						}
					}
					.desktopCard(padding: MenuBarGeometry.betweenRows)
					CollapsibleSection(title: t(DesktopCopy.calendarTitle), trailing: hoveredCalendarReadout, isExpanded: expansion("rhythm.calendar")) {
						calendarGrid
				}
				.desktopCard(padding: MenuBarGeometry.betweenRows)
			} else {
				UnavailableRow()
			}
		}
		}

	private var hourAxis: some View {
		HStack(spacing: 2) {
			Color.clear.frame(width: 32, height: 1)
			HStack {
				Text("00")
				Spacer()
				Text("06")
				Spacer()
				Text("12")
				Spacer()
				Text("18")
				Spacer()
				Text("24")
			}
			.font(.caption2)
			.monospacedDigit()
			.foregroundStyle(DesktopVisualTheme.dim)
		}
	}

	private var hoveredHourReadout: String? {
		guard let id = hover.hourCellID,
			let cell = block.cells.first(where: { $0.id == id })
		else {
			return nil
		}
		return "\(DesktopFormat.weekday(mondayBased: cell.weekday)) \(DesktopFormat.hourWindow(cell.hour)) · \(DesktopFormat.tokens(cell.tokens)) \(t(DesktopCopy.settingsMenuBarValueTokens)) · \(cell.cost)"
	}

	private var hoveredCalendarReadout: String? {
		guard let id = hover.calendarBucketID,
			let bucket = block.calendar.first(where: { $0.id == id })
		else {
			return nil
		}
		return "\(bucket.label) · \(DesktopFormat.tokens(bucket.tokens)) \(t(DesktopCopy.settingsMenuBarValueTokens)) · \(bucket.cost)"
	}

	private var grid: some View {
		VStack(alignment: .leading, spacing: 2) {
			ForEach(0 ..< 7, id: \.self) { weekday in
				HStack(spacing: 2) {
					Text(DesktopFormat.weekday(mondayBased: weekday))
						.font(.caption)
						.foregroundStyle(DesktopVisualTheme.dim)
						.frame(width: 32, alignment: .leading)
					ForEach(block.cells.filter { $0.weekday == weekday }) { cell in
						RoundedRectangle(cornerRadius: 1)
							.fill(DesktopVisualTheme.info.opacity(max(0.12, Double(cell.intensity) / 100)))
							.overlay {
								if hover.hourCellID == cell.id {
									RoundedRectangle(cornerRadius: 1).stroke(DesktopVisualTheme.text, lineWidth: 1)
								}
							}
							.frame(maxWidth: .infinity)
							.frame(height: 9)
							.contentShape(Rectangle())
							.onHover { inside in hover.setHour(cell.id, inside: inside) }
							.help("\(cell.accessibilityLabel) · \(cell.accessibilityValue)")
							.accessibilityLabel(cell.accessibilityLabel)
							.accessibilityValue(cell.accessibilityValue)
					}
				}
			}
		}
	}

	private var calendarGrid: some View {
		let maximum = max(Int64(1), block.calendar.map(\.tokens).max() ?? 1)
		return LazyVGrid(
			columns: Array(repeating: GridItem(.flexible(), spacing: 3), count: 18),
			spacing: 3
		) {
			ForEach(block.calendar) { bucket in
				RoundedRectangle(cornerRadius: 2)
					.fill(DesktopVisualTheme.info.opacity(max(0.12, Double(bucket.tokens) / Double(maximum))))
					.overlay {
						if hover.calendarBucketID == bucket.id {
							RoundedRectangle(cornerRadius: 2).stroke(DesktopVisualTheme.text, lineWidth: 1)
						}
					}
					.aspectRatio(1, contentMode: .fit)
					.contentShape(Rectangle())
					.onHover { inside in hover.setCalendar(bucket.id, inside: inside) }
							.help("\(bucket.label) · \(DesktopFormat.tokens(bucket.tokens)) \(t(DesktopCopy.settingsMenuBarValueTokens)) · \(bucket.cost)")
					.accessibilityLabel(bucket.label)
					.accessibilityValue(bucket.accessibilityValue)
			}
		}
	}
}
