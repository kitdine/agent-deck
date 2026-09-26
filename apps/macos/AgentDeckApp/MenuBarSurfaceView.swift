import AgentDeckShared
import AppKit
import SwiftUI

enum DesktopVisualTheme {
	static let accent = adaptive(light: 0xB24A0B, dark: 0xF2650F)
	static let accentStrong = adaptive(light: 0xB24A0B, dark: 0xC4520C)
	static let info = adaptive(light: 0x2F67C4, dark: 0x3B82F6)
	static let good = adaptive(light: 0x237843, dark: 0x2FA15A)
	static let warning = adaptive(light: 0x74500E, dark: 0xD9971A)
	static let error = adaptive(light: 0xA53937, dark: 0xEF5350)
	static let series = [
		adaptive(light: 0x6F8FD6, dark: 0x6F8FD6),
		adaptive(light: 0xB07BE0, dark: 0xB07BE0),
		adaptive(light: 0x3FA08A, dark: 0x3FA08A),
		adaptive(light: 0xCF8B5C, dark: 0xCF8B5C),
	]
	static let background = adaptive(light: 0xF7F9FC, dark: 0x0B0E13)
	static let surface = adaptive(light: 0xFFFFFF, dark: 0x141922)
	static let surfaceRaised = adaptive(light: 0xF2F5F9, dark: 0x1A202A)
	static let surfaceEmphasis = adaptive(light: 0xE8EDF4, dark: 0x212936)
	static let line = adaptive(light: 0xDDE3EC, dark: 0x262E3A)
	static let lineSoft = adaptive(light: 0xE8EDF4, dark: 0x1E2530)
	static let text = adaptive(light: 0x10161F, dark: 0xE9EEF5)
	static let muted = adaptive(light: 0x5D6A7A, dark: 0x98A3B2)
	static let dim = adaptive(light: 0x676F7B, dark: 0x7F8995)

	private static func adaptive(light: UInt32, dark: UInt32) -> Color {
		Color(nsColor: NSColor(name: nil) { appearance in
			appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
				? color(dark)
				: color(light)
		})
	}

	private static func color(_ value: UInt32) -> NSColor {
		let red = CGFloat((value >> 16) & 0xFF) / 255
		let green = CGFloat((value >> 8) & 0xFF) / 255
		let blue = CGFloat(value & 0xFF) / 255
		return NSColor(
			srgbRed: red,
			green: green,
			blue: blue,
			alpha: 1
		)
	}
}

private struct DesktopCardStyle: ViewModifier {
	let padding: CGFloat

	func body(content: Content) -> some View {
		content
			.padding(padding)
			.background(DesktopVisualTheme.surface, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
			.overlay {
				RoundedRectangle(cornerRadius: 10, style: .continuous)
					.stroke(DesktopVisualTheme.lineSoft, lineWidth: 1)
			}
	}
}

private struct DesktopSelectedSegmentStyle: ViewModifier {
	let selected: Bool

	func body(content: Content) -> some View {
		content.background {
			if selected {
				RoundedRectangle(cornerRadius: 6, style: .continuous)
					.fill(DesktopVisualTheme.surface)
					.shadow(color: .black.opacity(0.26), radius: 1, y: 1)
			}
		}
	}
}

extension View {
	func desktopCard(padding: CGFloat = MenuBarGeometry.padding) -> some View {
		modifier(DesktopCardStyle(padding: padding))
	}

	func desktopSelectedSegment(_ selected: Bool) -> some View {
		modifier(DesktopSelectedSegmentStyle(selected: selected))
	}
}

enum MenuBarGeometry {
	static let defaultWidth: CGFloat = 420
	static let narrowWidth: CGFloat = 280
	static let maximumHeight: CGFloat = 760
	static let screenMargin: CGFloat = 72
	static let rowMinimumHeight: CGFloat = 28
	static let padding: CGFloat = 12
	static let withinRow: CGFloat = 4
	static let betweenRows: CGFloat = 8
	static let betweenSections: CGFloat = 16

	static var width: CGFloat {
		#if DEBUG
		if ProcessInfo.processInfo.environment["AGENTDECK_TEST_WIDTH"] == "280" {
			return narrowWidth
		}
		#endif
		return defaultWidth
	}

	static func height(visibleFrameHeight: CGFloat) -> CGFloat {
		max(0, min(maximumHeight, visibleFrameHeight - screenMargin))
	}

}

extension NoticeSeverity {
	var tint: Color {
		switch self {
		case .error: DesktopVisualTheme.error
		case .warning: DesktopVisualTheme.warning
		}
	}
}

struct MenuBarSurfaceView: View {
	@Bindable var model: MenuBarViewModel
	var height: CGFloat = MenuBarGeometry.maximumHeight
	@Environment(\.accessibilityReduceMotion) private var reduceMotion

	var body: some View {
		VStack(spacing: 0) {
			header
				.padding(.horizontal, MenuBarGeometry.padding)
				.padding(.top, MenuBarGeometry.betweenRows)
				.padding(.bottom, MenuBarGeometry.betweenRows)
			Group {
				switch model.surface {
				case .loadingSurface:
					loadingSurface
				case .errorSurface:
					errorSurface
				case .dataSurface:
					dataSurface
				}
			}
		}
		.environment(\.schemaSignal, model.hasSchemaSignal)
		.frame(width: MenuBarGeometry.width)
		.frame(height: height)
		.modifier(AcceptanceAppearance())
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
	}

	private var loadingSurface: some View {
		VStack(spacing: MenuBarGeometry.betweenRows) {
			if reduceMotion {
				Image(systemName: "hourglass").accessibilityHidden(true)
			} else {
				ProgressView()
			}
			scanProgressStatus
		}
		.frame(maxWidth: .infinity, maxHeight: .infinity)
		.accessibilityElement(children: .contain)
	}

	private var errorSurface: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
			Label(model.errorCopy, systemImage: NoticeSeverity.error.symbol)
				.font(.body)
				.fixedSize(horizontal: false, vertical: true)
			if let body = model.errorBodyCopy {
				Text(body)
					.font(.caption)
					.foregroundStyle(DesktopVisualTheme.dim)
					.fixedSize(horizontal: false, vertical: true)
			}
			if model.showsScanProgressStatus {
				scanProgressStatus
			}
		}
		.padding(.horizontal, MenuBarGeometry.padding)
		.padding(.bottom, MenuBarGeometry.padding)
		.frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
	}

	private var dataSurface: some View {
		VStack(spacing: 0) {
			VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
				if model.showsScanProgressStatus {
					scanProgressStatus
				}
				if model.selectedPanel != .quota {
					clientTabs
					hero
					periodSwitcher
				}
				panelSwitcher
			}
			.padding(.horizontal, MenuBarGeometry.padding)
			.padding(.bottom, MenuBarGeometry.betweenRows)

			Divider()

			if model.showsHealthDetail {
				HealthDetailView(model: model)
			} else {
				content
			}

			SwitchFlowView(model: model)
			Divider()
			FooterView(model: model)
		}
		.background(DesktopVisualTheme.background)
	}

	private var scanProgressStatus: some View {
		VStack(alignment: .leading, spacing: 2) {
			Text(model.scanProgressStageText ?? t(DesktopCopy.loading))
				.font(.body)
				.fixedSize(horizontal: false, vertical: true)
				.accessibilityAddTraits(.updatesFrequently)
			if let counts = model.scanProgressCountsText {
				Text(counts)
					.font(.caption)
					.foregroundStyle(DesktopVisualTheme.dim)
					.fixedSize(horizontal: false, vertical: true)
			}
		}
		.frame(maxWidth: .infinity, alignment: .leading)
	}

	private var header: some View {
		HStack(spacing: MenuBarGeometry.betweenRows) {
			HStack(spacing: MenuBarGeometry.betweenRows) {
				brandIcon
				Text(t(DesktopCopy.appName))
					.font(.headline.weight(.semibold))
			}
			Spacer()
			if let freshnessText = model.freshnessText {
				Text(freshnessText)
					.font(.caption)
					.foregroundStyle(DesktopVisualTheme.dim)
					.fixedSize(horizontal: false, vertical: true)
			}
			StableRefreshControl(
				model: model,
				reduceMotion: reduceMotion
			)
		}
		.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
		.accessibilityValue(model.qualifierSummary ?? "")
		.overlay(alignment: .topLeading) {
			if let announcement = model.refreshAnnouncement {
				Text(announcement)
					.frame(width: 1, height: 1)
					.opacity(0.001)
					.accessibilityAddTraits(.updatesFrequently)
			}
		}
	}

	@ViewBuilder
	private var brandIcon: some View {
		if let url = Bundle.main.url(forResource: "AgentDeckAppIcon", withExtension: "png"),
			let image = NSImage(contentsOf: url)
		{
			Image(nsImage: image)
				.resizable()
				.interpolation(.high)
				.frame(width: 22, height: 22)
				.accessibilityHidden(true)
		} else {
			Image(systemName: "app.dashed")
				.frame(width: 22, height: 22)
				.accessibilityHidden(true)
		}
	}

	private var clientTabs: some View {
		Picker(t(DesktopCopy.clientFilter), selection: $model.selectedClient) {
			ForEach(model.clientTabs) { tab in
				Text(verbatim: "\(tab.label) \(tab.value ?? "")").tag(tab.id)
			}
		}
		.pickerStyle(.segmented)
		.tint(DesktopVisualTheme.accentStrong)
		.labelsHidden()
		.disabled(model.switchPresentation.blocksSurface)
		.accessibilityLabel(t(DesktopCopy.clientFilter))
	}

	@ViewBuilder
	private var hero: some View {
		if let hero = model.hero {
			VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
				Text(hero.scopeLine)
					.font(.caption)
					.foregroundStyle(DesktopVisualTheme.dim)
				HStack(alignment: .firstTextBaseline) {
					Text(hero.amount)
						.font(.largeTitle)
						.monospacedDigit()
						.foregroundStyle(DesktopVisualTheme.accent)
						.lineLimit(1)
						.minimumScaleFactor(0.6)
					Spacer()
					VStack(alignment: .trailing, spacing: MenuBarGeometry.withinRow) {
						Text(hero.tokens).monospacedDigit()
						Text(hero.counts)
					}
					.font(.caption)
					.foregroundStyle(DesktopVisualTheme.dim)
					.multilineTextAlignment(.trailing)
					.fixedSize(horizontal: false, vertical: true)
				}
				if let costIncomplete = hero.costIncomplete {
					Text(costIncomplete)
						.font(.caption)
						.foregroundStyle(DesktopVisualTheme.warning)
						.fixedSize(horizontal: false, vertical: true)
				}
			}
			.frame(maxWidth: .infinity, alignment: .leading)
			.accessibilityElement(children: .combine)
		}
	}

	private var periodSwitcher: some View {
		HStack(spacing: 2) {
			ForEach(model.periodOptions) { option in
				Button {
					model.selectedPeriod = option.id
				} label: {
					Text(option.label)
						.frame(maxWidth: .infinity, minHeight: 26)
						.contentShape(Rectangle())
				}
				.buttonStyle(.plain)
				.foregroundStyle(model.selectedPeriod == option.id ? DesktopVisualTheme.text : DesktopVisualTheme.muted)
				.desktopSelectedSegment(model.selectedPeriod == option.id)
				.accessibilityAddTraits(model.selectedPeriod == option.id ? [.isSelected, .isButton] : .isButton)
			}
		}
		.padding(2)
		.background(DesktopVisualTheme.surfaceRaised, in: RoundedRectangle(cornerRadius: 8, style: .continuous))
		.disabled(model.switchPresentation.blocksSurface)
		.accessibilityLabel(t(DesktopCopy.periodFilter))
	}

	/// The switcher carries an icon and a name per panel and no value: the
	/// headline number already sits directly above it.
	private var panelSwitcher: some View {
		HStack(spacing: MenuBarGeometry.withinRow) {
			ForEach(model.panelTabs) { tab in
				Button {
					model.selectedPanel = tab.id
				} label: {
					HStack(spacing: MenuBarGeometry.withinRow) {
						Image(systemName: tab.symbol)
							.foregroundStyle(model.selectedPanel == tab.id ? DesktopVisualTheme.accent : DesktopVisualTheme.muted)
						Text(tab.title).lineLimit(1)
							.foregroundStyle(model.selectedPanel == tab.id ? DesktopVisualTheme.text : DesktopVisualTheme.muted)
						if tab.marked {
							Image(systemName: NoticeSeverity.warning.symbol)
								.foregroundStyle(DesktopVisualTheme.warning)
						}
					}
					.font(.subheadline)
					.frame(maxWidth: .infinity, minHeight: MenuBarGeometry.rowMinimumHeight)
					.contentShape(Rectangle())
				}
				.buttonStyle(.borderless)
				.desktopSelectedSegment(model.selectedPanel == tab.id)
				.accessibilityLabel(tab.accessibilityLabel)
				.accessibilityAddTraits(model.selectedPanel == tab.id ? [.isSelected, .isButton] : .isButton)
			}
		}
		.padding(2)
		.background(DesktopVisualTheme.surfaceRaised, in: RoundedRectangle(cornerRadius: 8, style: .continuous))
		.disabled(model.switchPresentation.blocksSurface)
	}

	private var content: some View {
		ScrollView(.vertical) {
			VStack(alignment: .leading, spacing: MenuBarGeometry.betweenSections) {
				NoticeStripView(model: model)
				panel
					RhythmBlockView(block: model.rhythmBlock, expansion: sectionExpansion)
			}
			.padding(MenuBarGeometry.padding)
			.frame(maxWidth: .infinity, alignment: .leading)
		}
		.frame(maxHeight: .infinity)
		.scrollIndicators(.hidden)
	}

	@ViewBuilder
		private var panel: some View {
			switch model.selectedPanel {
			case .quota:
				QuotaPanelView(clients: model.quotaClients)
			case .usage:
			UsagePanelView(panel: model.usagePanel)
			case .breakdown:
				BreakdownPanelView(panel: model.breakdownPanel)
			case .attribution:
				AttributionPanelView(panel: model.attributionPanel, expansion: sectionExpansion)
			case .sessions:
				SessionsPanelView(panel: model.sessionsPanel, expansion: sectionExpansion)
			}
		}

	private func sectionExpansion(_ id: String) -> Binding<Bool> {
		Binding(
			get: { model.sectionIsExpanded(id) },
			set: { model.setSection(id, expanded: $0) }
		)
	}
}

struct RefreshControlIdentityPreferenceKey: PreferenceKey {
	static let defaultValue: UUID? = nil
	static func reduce(value: inout UUID?, nextValue: () -> UUID?) {
		value = value ?? nextValue()
	}
}

private struct StableRefreshControl: View {
	@Bindable var model: MenuBarViewModel
	let reduceMotion: Bool
	@State private var identity = UUID()

	var body: some View {
		StableRefreshButtonRepresentable(
			model: model,
			reduceMotion: reduceMotion,
			identity: identity
		)
		.frame(minWidth: MenuBarGeometry.rowMinimumHeight, minHeight: MenuBarGeometry.rowMinimumHeight)
		.preference(key: RefreshControlIdentityPreferenceKey.self, value: identity)
	}
}

private struct StableRefreshButtonRepresentable: NSViewRepresentable {
	@Bindable var model: MenuBarViewModel
	let reduceMotion: Bool
	let identity: UUID

	func makeCoordinator() -> Coordinator { Coordinator(model: model) }

	func makeNSView(context: Context) -> StableRefreshNSButton {
		let button = StableRefreshNSButton(title: "", target: context.coordinator, action: #selector(Coordinator.activate))
		button.identifier = NSUserInterfaceItemIdentifier("menubar.refresh.\(identity.uuidString)")
		button.bezelStyle = .inline
		button.isBordered = false
		button.focusRingType = .default
		button.keyEquivalent = "r"
		button.keyEquivalentModifierMask = [.command]
		return button
	}

	func updateNSView(_ button: StableRefreshNSButton, context: Context) {
		context.coordinator.model = model
		let enabled = !model.switchPresentation.blocksSurface && !model.isRefreshing
		let wasFocused = button.window?.firstResponder === button
		if context.coordinator.wasEnabled == true, !enabled, wasFocused {
			context.coordinator.restoreFocusWhenEnabled = true
		}
		button.update(
			title: MenuBarGeometry.width > MenuBarGeometry.narrowWidth ? model.refreshActionText : "",
			symbol: model.refreshActionSymbol,
			running: model.isRefreshing,
			reduceMotion: reduceMotion,
			enabled: enabled,
			accessibilityLabel: model.refreshActionAccessibilityLabel,
			accessibilityValue: model.isRefreshing ? model.scanProgressStageText ?? t(DesktopCopy.loading) : ""
		)
		if context.coordinator.wasEnabled == false, enabled {
			let shouldRestore = context.coordinator.restoreFocusWhenEnabled
			context.coordinator.restoreFocusWhenEnabled = false
			if shouldRestore, button.window?.firstResponder !== button {
				DispatchQueue.main.async { button.window?.makeFirstResponder(button) }
			}
		}
		context.coordinator.wasEnabled = enabled
	}

	@MainActor
	final class Coordinator: NSObject {
		var model: MenuBarViewModel
		var restoreFocusWhenEnabled = false
		var wasEnabled: Bool?
		init(model: MenuBarViewModel) { self.model = model }
		@objc func activate() { model.refresh() }
	}
}

private final class StableRefreshNSButton: NSButton {
	private let spinner = NSProgressIndicator()

	override init(frame frameRect: NSRect) {
		super.init(frame: frameRect)
		configureSpinner()
	}

	required init?(coder: NSCoder) {
		super.init(coder: coder)
		configureSpinner()
	}

	private func configureSpinner() {
		spinner.style = .spinning
		spinner.controlSize = .small
		spinner.isDisplayedWhenStopped = false
		addSubview(spinner)
	}

	override func layout() {
		super.layout()
		spinner.frame = NSRect(x: 5, y: (bounds.height - 14) / 2, width: 14, height: 14)
	}

	func update(
		title: String,
		symbol: String,
		running: Bool,
		reduceMotion: Bool,
		enabled: Bool,
		accessibilityLabel: String,
		accessibilityValue: String
	) {
		self.title = title
		isEnabled = enabled
		setAccessibilityLabel(accessibilityLabel)
		setAccessibilityValue(accessibilityValue)
		if running, !reduceMotion {
			image = nil
			spinner.startAnimation(nil)
		} else {
			spinner.stopAnimation(nil)
			image = NSImage(systemSymbolName: symbol, accessibilityDescription: nil)
		}
		imagePosition = title.isEmpty ? .imageOnly : .imageLeading
		invalidateIntrinsicContentSize()
	}
}

/// The acceptance harness pins the appearance so the manual contrast and
/// appearance checks address one state at a time. It is inert in release.
struct AcceptanceAppearance: ViewModifier {
	@ViewBuilder
	func body(content: Content) -> some View {
		#if DEBUG
		let appearance = ProcessInfo.processInfo.environment["AGENTDECK_TEST_APPEARANCE"]
		let scheme: ColorScheme? = appearance == "light" ? .light : appearance == "dark" ? .dark : nil
		content.preferredColorScheme(scheme)
		#else
		content
		#endif
	}
}

struct NoticeStripView: View {
	@Bindable var model: MenuBarViewModel

	var body: some View {
		let notices = model.notices
		if !notices.isEmpty {
			VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
				ForEach(notices) { notice in
					if notice.opensHealthDetail {
						Button {
							model.showsHealthDetail = true
						} label: {
							noticeRow(notice, disclosure: true)
						}
						.buttonStyle(.plain)
						.accessibilityHint(t(DesktopCopy.healthTitle))
					} else {
						noticeRow(notice, disclosure: false)
					}
				}
			}
			.frame(maxWidth: .infinity, alignment: .leading)
		}
	}

	private func noticeRow(_ notice: MenuBarNotice, disclosure: Bool) -> some View {
		let tint = notice.severity.tint
		return HStack(spacing: MenuBarGeometry.betweenRows) {
			Image(systemName: notice.severity.symbol)
				.foregroundStyle(tint)
			Text(notice.text)
				.font(.caption)
				.foregroundStyle(tint)
				.fixedSize(horizontal: false, vertical: true)
			Spacer()
			if disclosure {
				Image(systemName: "chevron.right").foregroundStyle(tint)
			}
		}
		.padding(.horizontal, MenuBarGeometry.padding)
		.padding(.vertical, MenuBarGeometry.withinRow)
		.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
		.frame(maxWidth: .infinity, alignment: .leading)
		.background(tint.opacity(0.14), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
		.overlay {
			RoundedRectangle(cornerRadius: 8, style: .continuous)
				.stroke(tint.opacity(0.55), lineWidth: 1)
		}
		.contentShape(Rectangle())
		.accessibilityElement(children: .combine)
	}
}

struct HealthDetailView: View {
	@Bindable var model: MenuBarViewModel

	var body: some View {
		VStack(alignment: .leading, spacing: 0) {
			HStack {
				Button {
					model.showsHealthDetail = false
				} label: {
					Label(t(DesktopCopy.healthBack), systemImage: "chevron.left")
				}
				.buttonStyle(.borderless)
				.keyboardShortcut(.cancelAction)
				Spacer()
				Text(t(DesktopCopy.healthTitle)).font(.headline)
			}
			.padding(.horizontal, MenuBarGeometry.padding)
			.padding(.vertical, MenuBarGeometry.betweenRows)
			.frame(minHeight: MenuBarGeometry.rowMinimumHeight)

				ScrollView(.vertical) {
					VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
						ForEach(model.healthDetail.rows) { row in
								HealthCheckRowView(row: row, model: model)
						}
					Text(model.healthDetail.source)
						.font(.caption)
						.foregroundStyle(.secondary)
						.fixedSize(horizontal: false, vertical: true)
				}
				.padding(MenuBarGeometry.padding)
				.frame(maxWidth: .infinity, alignment: .leading)
				}
				.frame(maxHeight: .infinity)
				.scrollIndicators(.hidden)
			}
		.frame(maxHeight: .infinity)
	}
}

struct HealthCheckRowView: View {
	let row: HealthCheckRow
	var model: MenuBarViewModel? = nil
	@State private var isExpanded: Bool

	init(row: HealthCheckRow, model: MenuBarViewModel? = nil, initiallyExpanded: Bool = true) {
		self.row = row
		self.model = model
		_isExpanded = State(initialValue: initiallyExpanded)
	}

	var body: some View {
		if row.hasDisclosure {
			VStack(alignment: .leading, spacing: 0) {
				Button {
					isExpanded.toggle()
				} label: {
					HStack(spacing: MenuBarGeometry.betweenRows) {
						statusRow
						Image(systemName: isExpanded ? "chevron.down" : "chevron.right")
							.font(.caption.weight(.semibold))
							.foregroundStyle(DesktopVisualTheme.dim)
							.accessibilityHidden(true)
					}
					.frame(maxWidth: .infinity, minHeight: MenuBarGeometry.rowMinimumHeight)
					.contentShape(Rectangle())
				}
				.buttonStyle(.plain)
				.accessibilityRepresentation {
					DisclosureGroup(isExpanded: $isExpanded) {
						Text("").accessibilityHidden(true)
					} label: {
						Text(row.accessibilityText(expanded: isExpanded))
					}
				}
				if isExpanded {
						VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
							if let reason = row.reasonLabel { proseRow(reason) }
							if let cause = row.cause { proseRow(cause) }
							if let recovery = row.recoveryProse { proseRow(recovery) }
							if let effect = row.effect { proseRow(effect) }
							if let content = row.actionContent, let label = row.actionLabel {
								actionRow(content: content, label: label)
							}
							if let command = row.recovery, !command.isEmpty { recoveryRow(command) }
					}
					.padding(.leading, MenuBarGeometry.rowMinimumHeight)
					.padding(.bottom, MenuBarGeometry.withinRow)
				}
			}
			.transaction { $0.animation = nil }
		} else {
			statusRow
		}
	}

	private var statusRow: some View {
		HStack {
			if let severity = row.severity {
				Image(systemName: severity.symbol).foregroundStyle(severity.tint)
			} else {
				Image(systemName: "checkmark.circle").foregroundStyle(.secondary)
			}
				Text(row.name).font(.body)
				if let count = row.count, count > 0 {
					Text(String(count)).font(.caption.weight(.semibold))
						.padding(.horizontal, 5).background(DesktopVisualTheme.surfaceRaised, in: Capsule())
				}
			Spacer()
			Text(row.status).font(.body).foregroundStyle(.secondary)
		}
		.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
		.accessibilityElement(children: .combine)
	}

	private func proseRow(_ text: String) -> some View {
		Text(text)
			.font(.caption)
			.fixedSize(horizontal: false, vertical: true)
			.accessibilityHidden(true) // The operable disclosure label speaks it once.
	}

	private func recoveryRow(_ recovery: String) -> some View {
		HStack {
			Text(recovery)
				.font(.caption)
				.monospaced()
				.textSelection(.enabled)
				.fixedSize(horizontal: false, vertical: true)
			Spacer()
			Button(t(DesktopCopy.healthCopyRecovery)) {
				NSPasteboard.general.clearContents()
				NSPasteboard.general.setString(recovery, forType: .string)
			}
			.buttonStyle(.borderless)
			.font(.caption)
		}
	}

	private func actionRow(content: String, label: String) -> some View {
		let copied = model?.copiedHealthRowID == row.id
		return HealthActionLayout {
			actionContent(content)
			HealthCopyButton(rowID: row.id, label: label, copied: copied) {
				model?.copyHealthAction(row)
			}
		}
	}

		@ViewBuilder
		private func actionContent(_ content: String) -> some View {
			Text(content)
				.font(row.actionLabel == t(DesktopCopy.healthCopySafety) ? .caption : .system(.caption, design: .monospaced))
				.fixedSize(horizontal: false, vertical: true)
				.textSelection(.enabled)
		}

}

private struct HealthActionLayout: Layout {
	private let spacing: CGFloat = 8

	private func stacked(width: CGFloat, subviews: Subviews) -> Bool {
		let contentWidth = subviews[0].sizeThatFits(.unspecified).width
		let buttonWidth = subviews[1].sizeThatFits(.unspecified).width
		return width < 320 || contentWidth + buttonWidth + spacing > width
	}

	func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
		let idealWidth = subviews[0].sizeThatFits(.unspecified).width
			+ subviews[1].sizeThatFits(.unspecified).width + spacing
		let width = proposal.width ?? idealWidth
		if stacked(width: width, subviews: subviews) {
			let content = subviews[0].sizeThatFits(ProposedViewSize(width: width, height: nil))
			let button = subviews[1].sizeThatFits(ProposedViewSize(width: width, height: nil))
			return CGSize(width: width, height: content.height + spacing + button.height)
		}
		let button = subviews[1].sizeThatFits(.unspecified)
		let content = subviews[0].sizeThatFits(ProposedViewSize(width: width - button.width - spacing, height: nil))
		return CGSize(width: width, height: max(content.height, button.height))
	}

	func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
		if stacked(width: bounds.width, subviews: subviews) {
			let content = subviews[0].sizeThatFits(ProposedViewSize(width: bounds.width, height: nil))
			subviews[0].place(at: bounds.origin, proposal: ProposedViewSize(width: bounds.width, height: content.height))
			subviews[1].place(
				at: CGPoint(x: bounds.minX, y: bounds.minY + content.height + spacing),
				proposal: ProposedViewSize(width: bounds.width, height: 28)
			)
		} else {
			let button = subviews[1].sizeThatFits(.unspecified)
			let contentWidth = bounds.width - button.width - spacing
			subviews[0].place(at: bounds.origin, proposal: ProposedViewSize(width: contentWidth, height: bounds.height))
			subviews[1].place(
				at: CGPoint(x: bounds.maxX - button.width, y: bounds.minY),
				proposal: ProposedViewSize(width: button.width, height: 28)
			)
		}
	}
}

/// AppKit keeps one focusable control while SwiftUI updates its feedback text.
private struct HealthCopyButton: NSViewRepresentable {
	let rowID: String
	let label: String
	let copied: Bool
	let action: () -> Void

	final class Coordinator: NSObject {
		var action: () -> Void
		init(action: @escaping () -> Void) { self.action = action }
		@objc func performCopy(_ sender: NSButton) { action() }
	}

	func makeCoordinator() -> Coordinator { Coordinator(action: action) }

	func makeNSView(context: Context) -> NSButton {
		let button = NSButton(title: label, target: context.coordinator, action: #selector(Coordinator.performCopy(_:)))
		button.identifier = NSUserInterfaceItemIdentifier("health.copy.\(rowID)")
		button.isBordered = false
		button.focusRingType = .default
		button.font = .systemFont(ofSize: NSFont.smallSystemFontSize)
		return button
	}

	func updateNSView(_ button: NSButton, context: Context) {
		context.coordinator.action = action
		button.title = copied ? t(DesktopCopy.healthCopied) : label
		button.setAccessibilityLabel(label)
		button.setAccessibilityValue(copied ? t(DesktopCopy.healthCopied) : "")
	}

	func sizeThatFits(_ proposal: ProposedViewSize, nsView: NSButton, context: Context) -> CGSize? {
		let font = NSFont.systemFont(ofSize: NSFont.smallSystemFontSize)
		let labelWidth = (label as NSString).size(withAttributes: [.font: font]).width + 20
		return CGSize(width: proposal.width ?? labelWidth, height: 28)
	}
}

struct FooterView: View {
	@Bindable var model: MenuBarViewModel
	@State private var showsProviderMenu = false

	var body: some View {
		let footer = model.footer
		Button {
			showsProviderMenu.toggle()
		} label: {
			HStack(spacing: MenuBarGeometry.betweenRows) {
				Text(t(DesktopCopy.footerProviders))
					.font(.caption)
					.foregroundStyle(DesktopVisualTheme.dim)
				Text(footer.routesText)
					.font(.caption.weight(.medium))
					.lineLimit(1)
					.truncationMode(.middle)
				Spacer()
				Image(systemName: showsProviderMenu ? "chevron.down" : "chevron.up")
					.foregroundStyle(DesktopVisualTheme.dim)
			}
		}
		.buttonStyle(.plain)
		.padding(.horizontal, MenuBarGeometry.padding)
		.padding(.vertical, MenuBarGeometry.betweenRows)
		.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
		.background(DesktopVisualTheme.surface)
		.disabled(model.switchPresentation.blocksSurface)
		.accessibilityLabel(t(DesktopCopy.switchMenuTitle))
		.popover(isPresented: $showsProviderMenu, arrowEdge: .bottom) {
			ProviderMenuView(model: model) {
				showsProviderMenu = false
			}
		}
	}
}

struct ProviderMenuView: View {
	@Bindable var model: MenuBarViewModel
	let dismiss: () -> Void
	@State private var selectedRowID: String?

	init(model: MenuBarViewModel, dismiss: @escaping () -> Void, selectedRowID: String? = nil) {
		self.model = model
		self.dismiss = dismiss
		_selectedRowID = State(initialValue: selectedRowID)
	}

	var body: some View {
		ScrollView(.vertical) {
			VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
				if let selectedRow {
					targetList(selectedRow)
				} else {
					providerList
				}
			}
			.padding(6)
		}
		.frame(width: 250)
		.frame(height: 260)
		.background(DesktopVisualTheme.surface)
		.foregroundStyle(DesktopVisualTheme.text)
		.tint(DesktopVisualTheme.accent)
	}

	private var selectedRow: SwitchOptionRow? {
		guard let selectedRowID else { return nil }
		return model.footer.sections.flatMap(\.rows).first(where: { $0.id == selectedRowID })
	}

	@ViewBuilder
	private var providerList: some View {
		let footer = model.footer
		if footer.switchingAvailable {
			ForEach(footer.sections) { section in
				Text(section.title)
					.font(.caption.weight(.semibold))
					.foregroundStyle(DesktopVisualTheme.dim)
					.padding(.horizontal, MenuBarGeometry.betweenRows)
					.padding(.top, MenuBarGeometry.withinRow)
				if section.rows.isEmpty {
					Text(t(DesktopCopy.switchNoOptions))
						.font(.caption)
						.foregroundStyle(DesktopVisualTheme.muted)
						.padding(MenuBarGeometry.betweenRows)
				} else {
					ForEach(section.rows) { row in
						providerRow(row)
					}
				}
			}
		} else {
			Text(model.switchingUnavailableText)
				.font(.caption)
				.foregroundStyle(DesktopVisualTheme.muted)
				.padding(MenuBarGeometry.betweenRows)
		}
	}

	private func providerRow(_ row: SwitchOptionRow) -> some View {
		Button {
			if row.choices.isEmpty {
				guard let target = row.target else { return }
				dismiss()
				model.pendingConfirmation = target
			} else {
				selectedRowID = row.id
			}
		} label: {
			HStack(spacing: MenuBarGeometry.betweenRows) {
				VStack(alignment: .leading, spacing: 1) {
					Text(row.label)
						.font(.caption.weight(.medium))
						.lineLimit(1)
					Text(row.isCurrent ? t(DesktopCopy.reasonAlreadySelected) : row.detail ?? t(DesktopCopy.switchReady))
						.font(.caption2)
						.foregroundStyle(row.enabled ? DesktopVisualTheme.muted : DesktopVisualTheme.dim)
						.lineLimit(1)
				}
				Spacer()
				if row.isCurrent {
					Image(systemName: "checkmark")
						.foregroundStyle(DesktopVisualTheme.accent)
						.accessibilityHidden(true)
				} else if !row.choices.isEmpty {
					Image(systemName: "chevron.right")
						.foregroundStyle(DesktopVisualTheme.dim)
						.accessibilityHidden(true)
				}
			}
			.padding(.horizontal, MenuBarGeometry.betweenRows)
			.padding(.vertical, 5)
			.frame(maxWidth: .infinity, minHeight: 36, alignment: .leading)
			.contentShape(Rectangle())
		}
		.buttonStyle(.plain)
		.disabled(!row.enabled)
		.opacity(row.enabled || row.isCurrent ? 1 : 0.68)
		.background(DesktopVisualTheme.surfaceRaised, in: RoundedRectangle(cornerRadius: 7, style: .continuous))
		.accessibilityLabel(row.label)
		.accessibilityValue(row.isCurrent ? t(DesktopCopy.reasonAlreadySelected) : row.detail ?? t(DesktopCopy.switchReady))
	}

	private func targetList(_ row: SwitchOptionRow) -> some View {
		Group {
			Button {
				selectedRowID = nil
			} label: {
				HStack(spacing: MenuBarGeometry.betweenRows) {
					Image(systemName: "chevron.left")
					Text(row.label).font(.caption.weight(.semibold))
					Spacer()
				}
				.padding(.horizontal, MenuBarGeometry.betweenRows)
				.frame(maxWidth: .infinity, minHeight: 36)
				.contentShape(Rectangle())
			}
			.buttonStyle(.plain)
			.accessibilityLabel(t(DesktopCopy.healthBack))

			ForEach(row.choices) { choice in
				Button {
					dismiss()
					model.pendingConfirmation = choice.target
				} label: {
					HStack {
						Text(choice.label)
							.font(.caption.weight(.medium))
						Spacer()
						Image(systemName: "chevron.right")
							.foregroundStyle(DesktopVisualTheme.dim)
							.accessibilityHidden(true)
					}
					.padding(.horizontal, MenuBarGeometry.betweenRows)
					.frame(maxWidth: .infinity, minHeight: 36)
					.contentShape(Rectangle())
				}
				.buttonStyle(.plain)
				.background(DesktopVisualTheme.surfaceRaised, in: RoundedRectangle(cornerRadius: 7, style: .continuous))
				.accessibilityLabel(choice.label)
			}
		}
	}
}

struct SwitchFlowView: View {
	@Bindable var model: MenuBarViewModel
	@Environment(\.accessibilityReduceMotion) private var reduceMotion

	var body: some View {
		switch model.switchPresentation {
		case .none:
			EmptyView()
		case let .confirming(_, message):
			card {
				Text(message).font(.body).fixedSize(horizontal: false, vertical: true)
				HStack {
					Button(t(DesktopCopy.switchCancel), role: .cancel) { model.cancelConfirmation() }
						.keyboardShortcut(.cancelAction)
					Spacer()
					Button(t(DesktopCopy.switchConfirm)) { model.confirmSwitch() }
						.keyboardShortcut(.defaultAction)
				}
			}
		case let .inFlight(message):
			card {
				if reduceMotion {
					Text(message).font(.body)
				} else {
					ProgressView(message).controlSize(.small)
				}
				HStack {
					Button(t(DesktopCopy.switchCancel), role: .cancel) {}.disabled(true)
					Spacer()
					Button(t(DesktopCopy.switchConfirm)) {}.disabled(true)
				}
			}
		case let .failed(message, detail):
			card {
				Label(message, systemImage: NoticeSeverity.error.symbol)
					.font(.body)
					.fixedSize(horizontal: false, vertical: true)
				Text(detail).font(.caption).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
				terminalActions
			}
		case let .indeterminate(message, detail):
			card {
				Label(message, systemImage: NoticeSeverity.warning.symbol)
					.font(.body)
					.fixedSize(horizontal: false, vertical: true)
				Text(detail).font(.caption).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
				terminalActions
			}
		case let .succeeded(message):
			card {
				Label(message, systemImage: "checkmark.circle")
					.font(.body)
					.fixedSize(horizontal: false, vertical: true)
			}
		}
	}

	private var terminalActions: some View {
		HStack {
			Button(t(DesktopCopy.switchDismiss)) { model.dismissSwitch() }
				.keyboardShortcut(.cancelAction)
			Spacer()
			Button(t(DesktopCopy.retry)) { model.retrySwitch() }
				.keyboardShortcut(.defaultAction)
		}
	}

	private func card<Content: View>(@ViewBuilder content: () -> Content) -> some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
			content()
		}
		.padding(MenuBarGeometry.padding)
		.frame(maxWidth: .infinity, alignment: .leading)
		.background(DesktopVisualTheme.surfaceRaised)
		.overlay(alignment: .top) {
			Rectangle().fill(DesktopVisualTheme.line).frame(height: 1)
		}
		.accessibilityElement(children: .contain)
	}
}
