import AgentDeckShared
import AppKit
import SwiftUI

/// The Settings window's toolbar pages, in toolbar order (ux/settings.md
/// 窗口分页). Pages split by why someone opens Settings; each group's rows and
/// contracts are unchanged, only the container is.
enum SettingsPane: String, CaseIterable, Identifiable {
	case general
	case sessions
	case liveUsage
	case quota

	var id: String { rawValue }

	var title: String {
		switch self {
		case .general: t(DesktopCopy.settingsGroupGeneral)
		case .sessions: t(DesktopCopy.panelSessions)
		case .liveUsage: t(DesktopCopy.settingsGroupLiveUsage)
		case .quota: t(DesktopCopy.panelQuota)
		}
	}

	var symbol: String {
		switch self {
		case .general: "gearshape"
		case .sessions: "square.stack.3d.up"
		case .liveUsage: "waveform.path.ecg"
		case .quota: "gauge.with.dots.needle.50percent"
		}
	}
}

/// One page of the Settings window. A page with one group shows no group
/// title, because the page name already says it; General keeps its "Menu bar"
/// section title. Colours follow ux/settings.md 整窗视觉对齐: no window-wide
/// accent tint, so segmented controls and secondary buttons stay neutral and
/// only switches, status links and the primary actions carry the accent.
struct SettingsPaneView: View {
	let pane: SettingsPane
	@Bindable var preferences: DesktopPreferences
	var quotaSettings: QuotaSettingsController
	var telemetrySettings: TelemetrySettingsController
	/// The App-level session scope and origin controller; the Sessions page
	/// exists only when there is one.
	var sessionOrigin: SessionOriginSettingsController? = nil
	/// The window scrolls a page only when it is taller than the screen; the
	/// rendering checks lay the page out at its natural height instead.
	var scrolls = true
	/// Reports the page's natural height whenever it changes while shown, so
	/// the window follows a row that appears or goes away.
	var onNaturalHeight: ((CGFloat) -> Void)? = nil

	/// The tallest a page's content may be before it scrolls inside the page,
	/// leaving the title bar and toolbar on screen.
	static var maximumHeight: CGFloat {
		max(320, (NSScreen.main?.visibleFrame.height ?? 800) - 140)
	}

	var body: some View {
		Group {
			if scrolls {
				// The page's ideal size is its natural height, which sizes the
				// window; only a page taller than the screen scrolls.
				ViewThatFits(in: .vertical) {
					page
					ScrollView(.vertical) { page }
						.frame(width: 460)
						.frame(maxHeight: Self.maximumHeight)
				}
			} else {
				page
			}
		}
		.modifier(AcceptanceAppearance())
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
	}

	private var page: some View {
		VStack(alignment: .leading, spacing: 18) {
			switch pane {
			case .general:
				group(nil) { generalRows }
				group(t(DesktopCopy.settingsGroupMenuBar)) { menuBarRows }
			case .sessions:
				if let sessionOrigin {
					SessionOriginSettingsGroup(controller: sessionOrigin)
				}
			case .liveUsage:
				group(nil) { liveUsageRows }
			case .quota:
				group(nil) { quotaRows }
			}
		}
		.padding(18)
		.frame(width: 460, alignment: .leading)
		// The rows never stretch, so the page's own height is its natural one
		// whether it is shown whole or inside the scroll view.
		.onGeometryChange(for: CGFloat.self) { $0.size.height } action: { height in
			onNaturalHeight?(height)
		}
	}

	@ViewBuilder
	private var generalRows: some View {
		SettingsRow(
			label: t(DesktopCopy.settingsLoginItem),
			note: t(DesktopCopy.settingsLoginItemNote),
			status: loginItemStatus
		) {
			Toggle(
				t(DesktopCopy.settingsLoginItem),
				isOn: Binding(
					get: { preferences.loginItem.isOn },
					set: { preferences.setLoginItem(enabled: $0) }
				)
			)
			.toggleStyle(.switch)
			.tint(DesktopVisualTheme.accent)
			.labelsHidden()
			.accessibilityLabel(t(DesktopCopy.settingsLoginItem))
		}
		Divider()
		SettingsRow(
			label: t(DesktopCopy.settingsAutomaticUpdate),
			note: t(DesktopCopy.settingsAutomaticUpdateNote),
			status: nil
		) {
			Toggle(t(DesktopCopy.settingsAutomaticUpdate), isOn: $preferences.automaticUpdateEnabled)
				.toggleStyle(.switch)
				.tint(DesktopVisualTheme.accent)
				.labelsHidden()
				.accessibilityLabel(t(DesktopCopy.settingsAutomaticUpdate))
				.accessibilityHint(t(DesktopCopy.settingsAutomaticUpdateNote))
		}
	}

	@ViewBuilder
	private var menuBarRows: some View {
		SettingsRow(
			label: t(DesktopCopy.settingsMenuBarValue),
			note: t(DesktopCopy.settingsMenuBarValueNote),
			status: nil
		) {
			Picker(t(DesktopCopy.settingsMenuBarValue), selection: $preferences.menuBarValue) {
				ForEach(MenuBarValueMode.allCases) { mode in
					Text(mode.label).tag(mode)
				}
			}
			.settingsSegmented()
			.labelsHidden()
			.fixedSize()
			.accessibilityLabel(t(DesktopCopy.settingsMenuBarValue))
		}
		Divider()
		SettingsRow(
			label: t(DesktopCopy.settingsMenuBarScope),
			note: t(DesktopCopy.settingsMenuBarScopeNote),
			status: nil
		) {
			Picker(t(DesktopCopy.settingsMenuBarScope), selection: $preferences.menuBarScope) {
				ForEach(MenuBarScopeMode.allCases) { mode in
					Text(mode.label).tag(mode)
				}
			}
			.settingsSegmented()
			.labelsHidden()
			.fixedSize()
			.accessibilityLabel(t(DesktopCopy.settingsMenuBarScope))
		}
	}

	@ViewBuilder
	private var liveUsageRows: some View {
		SettingsRow(
			label: t(DesktopCopy.settingsOTel),
			note: t(DesktopCopy.settingsOTelHint),
			status: telemetrySettings.failureRow
		) {
			Toggle(
				t(DesktopCopy.settingsOTel),
				isOn: Binding(
					get: { telemetrySettings.isOn },
					set: { newValue in Task { await telemetrySettings.toggle(newValue) } }
				)
			)
			.toggleStyle(.switch)
			.tint(DesktopVisualTheme.accent)
			.labelsHidden()
			.disabled(!telemetrySettings.switchEnabled)
			.accessibilityLabel(t(DesktopCopy.settingsOTel))
			.accessibilityHint(t(DesktopCopy.settingsOTelPrivacy))
		}
		TelemetryDetailsView(controller: telemetrySettings)
	}

	@ViewBuilder
	private var quotaRows: some View {
		SettingsRow(
			label: t(DesktopCopy.settingsQuotaProbe),
			note: t(DesktopCopy.settingsQuotaProbeHint),
			status: quotaSettings.settingsRow
		) {
			Toggle(
				t(DesktopCopy.settingsQuotaProbe),
				isOn: Binding(
					get: { preferences.quotaProbeEnabled },
					set: { newValue in Task { await quotaSettings.setReading(newValue) } }
				)
			)
			.toggleStyle(.switch)
			.tint(DesktopVisualTheme.accent)
			.labelsHidden()
			.disabled(!quotaSettings.readingControlsEnabled)
			.accessibilityLabel(t(DesktopCopy.settingsQuotaProbe))
		}
		Divider()
		SettingsRow(
			label: t(DesktopCopy.settingsQuotaClients),
			note: t(DesktopCopy.settingsQuotaClientsHint),
			status: nil
		) {
			HStack(spacing: 12) {
				ForEach(["codex", "claude"], id: \.self) { client in
					QuotaClientCheckbox(
						title: client == "codex" ? "Codex" : "Claude",
						identifier: NSUserInterfaceItemIdentifier("quota-client-\(client)"),
						isOn: Binding(
							get: { quotaSettings.clientEnabled(client) },
							set: { enabled in Task { await quotaSettings.setClient(client, enabled: enabled) } }
						)
					)
					// The AppKit checkbox otherwise stretches across the row and
					// squeezes the note column into lines that overlap the next row.
					.fixedSize()
				}
			}
			.disabled(!quotaSettings.readingControlsEnabled || !preferences.quotaProbeEnabled)
		}
		Divider()
		SettingsRow(
			label: t(DesktopCopy.settingsQuotaInterval),
			note: t(DesktopCopy.settingsQuotaIntervalHint),
			status: nil
		) {
			Picker(
				t(DesktopCopy.settingsQuotaInterval),
				selection: Binding(
					get: { preferences.quotaProbeInterval },
					set: { newValue in Task { await quotaSettings.setInterval(newValue) } }
				)
			) {
				Text(t(DesktopCopy.settingsQuotaInterval5m)).tag(QuotaProbeInterval.fiveMinutes)
				Text(t(DesktopCopy.settingsQuotaInterval15m)).tag(QuotaProbeInterval.fifteenMinutes)
				Text(t(DesktopCopy.settingsQuotaInterval30m)).tag(QuotaProbeInterval.thirtyMinutes)
			}
			.settingsSegmented()
			.labelsHidden()
			.fixedSize()
			.disabled(!quotaSettings.readingControlsEnabled)
			.accessibilityLabel(t(DesktopCopy.settingsQuotaInterval))
		}
		Divider()
		SettingsRow(
			label: t(DesktopCopy.settingsQuotaStatusline),
			note: statuslineHint,
			status: quotaSettings.statuslineRow
		) {
			Toggle(
				t(DesktopCopy.settingsQuotaStatusline),
				isOn: Binding(
					get: { quotaSettings.settings?.statusline ?? false },
					set: { newValue in Task { await quotaSettings.setStatuslineConsent(newValue) } }
				)
			)
			.toggleStyle(.switch)
			.tint(DesktopVisualTheme.accent)
			.labelsHidden()
			// Dependency shown as disabled, not hidden (ux/settings-quota.md):
			// the status-line route needs reading on, and the reason is
			// legible from the group above it.
			.disabled(!quotaSettings.statuslineControlEnabled)
			.accessibilityLabel(t(DesktopCopy.settingsQuotaStatusline))
		}
		Divider()
		SettingsRow(
			label: t(DesktopCopy.settingsQuotaAlerts),
			note: t(DesktopCopy.settingsQuotaAlertsHint),
			status: quotaSettings.alertsRow,
			statusAction: quotaSettings.alertsRow == nil ? nil : SettingsRowAction(
				title: t(DesktopCopy.settingsQuotaAlertsOpenNotificationSettings),
				perform: openNotificationSettings
			)
		) {
			Toggle(
				t(DesktopCopy.settingsQuotaAlerts),
				isOn: Binding(
					get: { quotaSettings.settings?.alerts ?? false },
					set: { newValue in Task { await quotaSettings.setAlerts(newValue) } }
				)
			)
			.toggleStyle(.switch)
			.tint(DesktopVisualTheme.accent)
			.labelsHidden()
			.disabled(!preferences.quotaProbeEnabled)
			.accessibilityLabel(t(DesktopCopy.settingsQuotaAlerts))
		}
		Divider()
		SettingsRow(
			label: t(DesktopCopy.settingsQuotaThresholds),
			note: nil,
			status: nil
		) {
			Picker(
				t(DesktopCopy.settingsQuotaThresholds),
				selection: Binding(
					get: { QuotaAlertThresholdChoice(values: quotaSettings.settings?.thresholds ?? [75, 90]) },
					set: { newValue in Task { await quotaSettings.setThresholds(newValue) } }
				)
			) {
				Text(t(DesktopCopy.settingsQuotaThreshold75)).tag(QuotaAlertThresholdChoice.seventyFive)
				Text(t(DesktopCopy.settingsQuotaThreshold90)).tag(QuotaAlertThresholdChoice.ninety)
				Text(t(DesktopCopy.settingsQuotaThresholdBoth)).tag(QuotaAlertThresholdChoice.both)
			}
			.settingsSegmented()
			.labelsHidden()
			.fixedSize()
			.accessibilityLabel(t(DesktopCopy.settingsQuotaThresholds))
			// Always readable regardless of the alerts switch above —
			// "seeing what the thresholds are is useful before deciding
			// to enable alerts" (ux/settings-quota.md).
		}
		Divider()
		SettingsRow(
			label: t(DesktopCopy.settingsQuotaResetNotice),
			note: nil,
			status: nil
		) {
			Toggle(
				t(DesktopCopy.settingsQuotaResetNotice),
				isOn: Binding(
					get: { quotaSettings.settings?.resetNotice ?? false },
					set: { newValue in Task { await quotaSettings.setResetNotice(newValue) } }
				)
			)
			.toggleStyle(.switch)
			.tint(DesktopVisualTheme.accent)
			.labelsHidden()
			.disabled(!quotaSettings.resetNoticeControlEnabled)
			.accessibilityLabel(t(DesktopCopy.settingsQuotaResetNotice))
		}
	}

	/// `settings.quotaStatuslineHint` plus whichever of `quotaStatuslineChained`
	/// or `quotaStatuslineNone` applies, joined the way the rendered specimen
	/// shows them: one note line, a lone `·` between the fixed hint and the
	/// live preview of what consenting would actually do.
	private var statuslineHint: String {
		let chained = quotaSettings.chainedStatusLineCommand.map { t(DesktopCopy.settingsQuotaStatuslineChained, $0) }
			?? t(DesktopCopy.settingsQuotaStatuslineNone)
		return "\(t(DesktopCopy.settingsQuotaStatuslineHint)) · \(chained)"
	}

	/// `requiresApproval` is not a failure and is not worded as one.
	private var loginItemStatus: SettingsRowStatus? {
		switch preferences.loginItem {
		case .refused:
			SettingsRowStatus(text: t(DesktopCopy.settingsLoginItemRefused), severity: .warning)
		case .requiresApproval:
			SettingsRowStatus(text: t(DesktopCopy.settingsLoginItemApproval), severity: .warning)
		case .enabled, .disabled:
			nil
		}
	}

	private func group<Content: View>(_ title: String?, @ViewBuilder content: () -> Content) -> some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
			if let title {
				Text(title)
					.font(.caption.weight(.semibold))
					.foregroundStyle(DesktopVisualTheme.dim)
			}
			content()
		}
	}
}

extension View {
	/// A segmented control whose selected segment is neutral, the prototype's
	/// `--surface` pill with body-coloured text (ux/settings.md 整窗视觉对齐).
	/// On macOS 26 the untinted system control fills the selected segment with
	/// the system accent in an active window; tinting it with the surface
	/// colour keeps the system control and lets AppKit pick the contrasting
	/// label colour in both appearances.
	func settingsSegmented() -> some View {
		pickerStyle(.segmented)
			.tint(DesktopVisualTheme.surface)
	}

	/// A status action: a link in the accent colour, set on the link itself
	/// because the window no longer tints everything (ux/settings.md 整窗视觉对齐).
	func settingsLink() -> some View {
		buttonStyle(.link)
			.tint(DesktopVisualTheme.accent)
			.foregroundStyle(DesktopVisualTheme.accent)
	}
}

private struct QuotaClientCheckbox: NSViewRepresentable {
	let title: String
	let identifier: NSUserInterfaceItemIdentifier
	@Binding var isOn: Bool
	@Environment(\.isEnabled) private var isEnabled

	func makeCoordinator() -> Coordinator {
		Coordinator(isOn: $isOn)
	}

	func makeNSView(context: Context) -> NSButton {
		let button = NSButton(
			checkboxWithTitle: title,
			target: context.coordinator,
			action: #selector(Coordinator.changed(_:))
		)
		button.identifier = identifier
		button.setAccessibilityRole(.checkBox)
		button.setAccessibilityLabel(title)
		return button
	}

	func updateNSView(_ button: NSButton, context: Context) {
		context.coordinator.isOn = $isOn
		button.state = isOn ? .on : .off
		button.setAccessibilityValue(NSNumber(value: isOn))
		button.isEnabled = isEnabled
	}

	@MainActor
	final class Coordinator: NSObject {
		var isOn: Binding<Bool>

		init(isOn: Binding<Bool>) {
			self.isOn = isOn
		}

		@objc func changed(_ sender: NSButton) {
			isOn.wrappedValue = sender.state == .on
		}
	}
}

struct SettingsRowStatus: Equatable {
	let text: String
	let severity: NoticeSeverity
}

/// A follow-up the failure row offers, such as opening System Settings. It is
/// rendered outside the row's combined accessibility element so it stays a
/// separately actionable button.
struct SettingsRowAction {
	let title: String
	let perform: () -> Void
}

struct SettingsRow<Control: View>: View {
	let label: String
	/// `nil` when the row has no documented hint (ux/settings-quota.md's
	/// rendered specimen shows the alert-threshold and reset-notice rows with
	/// no note line at all) — omitted rather than passed as an empty string,
	/// which would still reserve a line of caption height nothing occupies.
	let note: String?
	let status: SettingsRowStatus?
	var statusAction: SettingsRowAction? = nil
	@ViewBuilder let control: () -> Control

	var body: some View {
		HStack(alignment: .firstTextBaseline, spacing: MenuBarGeometry.betweenSections) {
			VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
				VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
					Text(label).font(.body)
					if let note {
						Text(note)
							.font(.caption)
							.foregroundStyle(DesktopVisualTheme.dim)
							.fixedSize(horizontal: false, vertical: true)
					}
					if let status {
						Label(status.text, systemImage: status.severity.symbol)
							.font(.caption)
							.foregroundStyle(status.severity.tint)
							.fixedSize(horizontal: false, vertical: true)
					}
				}
				.accessibilityElement(children: .combine)
				if let statusAction {
					Button(statusAction.title, action: statusAction.perform)
						.settingsLink()
						.font(.caption)
				}
			}
			.frame(maxWidth: .infinity, alignment: .leading)
			control()
		}
		.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
	}
}

/// The status, consent and preview area under the OTel switch
/// (ux/settings.md). The consent area is inline, never a modal, so the
/// switch and status stay reachable.
private struct TelemetryDetailsView: View {
	@Bindable var controller: TelemetrySettingsController

	var body: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
			if let stateText = controller.stateText {
				// Polite and atomic: a state change is announced without
				// moving focus.
				Text(stateText)
					.font(.caption.weight(.semibold))
					.fixedSize(horizontal: false, vertical: true)
					.accessibilityAddTraits(.updatesFrequently)
			}
			if let hint = controller.stateHint {
				caption(hint)
			}
			if controller.showsNotCounted {
				Label {
					Text("\(t(DesktopCopy.settingsOTelNotCounted))\n\(t(DesktopCopy.settingsOTelNotCountedHint))")
				} icon: {
					Image(systemName: NoticeSeverity.warning.symbol)
				}
				.font(.caption)
				.foregroundStyle(DesktopVisualTheme.warning)
				.fixedSize(horizontal: false, vertical: true)
			}
			if let consent = controller.consent {
				consentView(consent)
			}
			if let result = controller.resultText {
				caption(result)
			}
			caption(t(DesktopCopy.settingsOTelPrivacy))
			Button(t(controller.showsPreview ? DesktopCopy.settingsOTelHideConfiguration : DesktopCopy.settingsOTelViewConfiguration)) {
				controller.showsPreview.toggle()
				if controller.showsPreview { Task { await controller.loadPreview() } }
			}
			.font(.caption)
			if controller.showsPreview {
				caption(t(DesktopCopy.settingsOTelPreviewNote))
				if let snippet = controller.preview?.snippet {
					Text(snippet)
						.font(.caption.monospaced())
						.textSelection(.enabled)
						.fixedSize(horizontal: false, vertical: true)
				}
			}
		}
	}

	@ViewBuilder
	private func consentView(_ consent: TelemetryConsent) -> some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
			switch consent {
			case .write:
				Text(t(DesktopCopy.settingsOTelConsentTitle)).font(.caption.weight(.semibold))
				caption(t(DesktopCopy.settingsOTelConsentFile))
				Text("# BEGIN AgentDeck OTel (managed)\n[otel]\nlog_user_prompt = false\nexporter = { otlp-http = { endpoint = \"http://127.0.0.1:\(controller.status?.port ?? 4318)/v1/logs\", protocol = \"json\", headers = { \"X-AgentDeck-Token\" = \"<local-token>\" } } }\n# END AgentDeck OTel (managed)")
					.font(.caption.monospaced())
					.fixedSize(horizontal: false, vertical: true)
				caption(t(DesktopCopy.settingsOTelConsentBody))
				HStack {
					Button(t(DesktopCopy.settingsOTelWriteAndOn)) { Task { await controller.writeAndEnable() } }
						.buttonStyle(.borderedProminent)
						.tint(DesktopVisualTheme.accent)
					Button(t(DesktopCopy.settingsOTelSelfConfigure)) { Task { await controller.configureMyself() } }
					Button(t(DesktopCopy.switchCancel)) { controller.cancelConsent() }
				}
			case .conflict:
				caption(t(DesktopCopy.settingsOTelConflict))
				HStack {
					Button(t(DesktopCopy.settingsOTelReceiverOnly)) { Task { await controller.enableReceiverOnly() } }
						.buttonStyle(.borderedProminent)
						.tint(DesktopVisualTheme.accent)
					Button(t(DesktopCopy.switchCancel)) { controller.cancelConsent() }
				}
			}
		}
		.font(.caption)
		.padding(8)
		.overlay(RoundedRectangle(cornerRadius: 6).stroke(DesktopVisualTheme.dim.opacity(0.4)))
		.accessibilityElement(children: .contain)
		.accessibilityLabel(t(consent == .write ? DesktopCopy.settingsOTelConsentTitle : DesktopCopy.settingsOTelConflict))
	}

	private func caption(_ text: String) -> some View {
		Text(text)
			.font(.caption)
			.foregroundStyle(DesktopVisualTheme.dim)
			.fixedSize(horizontal: false, vertical: true)
	}
}

/// The "Sessions" group (ux/settings.md, 2026-11-07 redesign): the default
/// scope, one summary row that opens the manual-classification sheet, and the
/// read-only automatic-detection sources. It reads like the other groups: a
/// title and note on the left, the control on the right, and choosing is
/// saving. Every value comes from the App-level controller; the view holds
/// only focus.
struct SessionOriginSettingsGroup: View {
	@Bindable var controller: SessionOriginSettingsController
	@FocusState private var manageFocused: Bool

	var body: some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
			defaultScope
			Divider()
			manualClassification
			Divider()
			automaticDetection
		}
		.sheet(isPresented: $controller.isManaging, onDismiss: { manageFocused = true }) {
			OriginManageSheet(controller: controller)
		}
	}

	/// Selected is always the confirmed value; a write in flight shows only in
	/// the status line. Nothing is selected while the value is unconfirmed.
	private var defaultScope: some View {
		OriginSettingsRow(label: t(DesktopCopy.originDefaultTitle), note: t(DesktopCopy.originDefaultNote)) {
			OriginStatusView(
				line: controller.statusLine(for: .defaultScope),
				warning: controller.permissionWarningText(for: .defaultScope),
				controller: controller
			)
		} control: {
			Picker(
				t(DesktopCopy.originDefaultTitle),
				selection: Binding(
					get: { controller.selectedDefault },
					set: { scope in
						guard let scope else { return }
						Task { await controller.selectDefault(scope) }
					}
				)
			) {
				ForEach(DesktopSessionScopeV1.allCases, id: \.self) { scope in
					Text(SessionOriginCopy.scope(scope)).tag(DesktopSessionScopeV1?.some(scope))
				}
			}
			.settingsSegmented()
			.labelsHidden()
			.fixedSize()
			.disabled(!controller.writesEnabled)
			.accessibilityLabel(t(DesktopCopy.originDefaultTitle))
			.accessibilityHint(t(DesktopCopy.originDefaultNote))
		}
	}

	/// While the sheet is closed this row still reports anything unfinished.
	private var manualClassification: some View {
		OriginSettingsRow(label: t(DesktopCopy.originManualTitle), note: controller.manualSummary) {
			if !controller.isManaging {
				OriginStatusView(
					line: controller.manualRowStatus,
					warning: controller.permissionWarningText(for: .manualClassification),
					controller: controller
				)
			}
		} control: {
			Button(t(DesktopCopy.originManage)) { controller.openManage() }
				.focused($manageFocused)
				.accessibilityHint(controller.manualSummary)
		}
	}

	private var automaticDetection: some View {
		OriginSettingsRow(label: t(DesktopCopy.originDetectionTitle), note: t(DesktopCopy.originDetectionNote)) {
			if let rows = controller.sourceRows {
				// Plain rows with a fixed name column rather than a Grid, whose
				// measured height did not match its wrapped rendering.
				VStack(alignment: .leading, spacing: 3) {
					ForEach(rows, id: \.name) { row in
						HStack(alignment: .firstTextBaseline, spacing: 8) {
							Circle()
								.fill(row.tone.tint)
								.frame(width: 7, height: 7)
								.accessibilityHidden(true)
							Text(row.name)
								.frame(minWidth: 104, alignment: .leading)
								.fixedSize()
							Text(row.text)
								.foregroundStyle(row.tone.tint)
								.fixedSize(horizontal: false, vertical: true)
								.frame(maxWidth: .infinity, alignment: .leading)
						}
						.font(.caption)
						.help(row.code)
						.accessibilityElement(children: .combine)
					}
				}
				.padding(.top, 2)
			}
			sourceNote
		} control: {
			EmptyView()
		}
	}

	@ViewBuilder
	private var sourceNote: some View {
		switch controller.sourceNote {
		case .adapterUnavailable:
			OriginCaption(t(DesktopCopy.originSourceNoteAdapter))
		case .coreUnavailable:
			OriginCaption(t(DesktopCopy.originSourceNoteCore))
			Button(t(DesktopCopy.originActionReadAgain)) { Task { await controller.readLatestValues() } }
				.settingsLink()
				.font(.caption)
		case .statusUnavailable:
			OriginCaption(t(DesktopCopy.originSourcesUnavailable))
		case nil:
			EmptyView()
		}
	}
}

/// The settings-row layout (`SettingsRow`) with room for this group's richer
/// status: a spinner, a request line and more than one action.
private struct OriginSettingsRow<Status: View, Control: View>: View {
	let label: String
	let note: String
	@ViewBuilder let status: () -> Status
	@ViewBuilder let control: () -> Control

	var body: some View {
		HStack(alignment: .firstTextBaseline, spacing: MenuBarGeometry.betweenSections) {
			VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
				Text(label).font(.body)
				OriginCaption(note)
				status()
			}
			.frame(maxWidth: .infinity, alignment: .leading)
			control()
		}
		.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
	}
}

private struct OriginCaption: View {
	let text: String

	init(_ text: String) { self.text = text }

	var body: some View {
		Text(text)
			.font(.caption)
			.foregroundStyle(DesktopVisualTheme.dim)
			.fixedSize(horizontal: false, vertical: true)
	}
}

extension OriginStatusLine.Tone {
	var tint: Color {
		switch self {
		case .busy, .neutral: DesktopVisualTheme.dim
		case .good: DesktopVisualTheme.good
		case .info: DesktopVisualTheme.info
		case .warning: DesktopVisualTheme.warning
		case .bad: DesktopVisualTheme.error
		}
	}

	var symbol: String? {
		switch self {
		case .busy: nil
		case .good: "checkmark.circle.fill"
		case .info: "info.circle.fill"
		case .warning: "exclamationmark.triangle.fill"
		case .bad: "exclamationmark.octagon.fill"
		case .neutral: "minus.circle"
		}
	}
}

/// One status line and its actions. The words are one polite region; the
/// actions sit outside it so each stays separately focusable.
private struct OriginStatusView: View {
	let line: OriginStatusLine?
	let warning: String?
	let controller: SessionOriginSettingsController

	var body: some View {
		if line != nil || warning != nil {
			VStack(alignment: .leading, spacing: 3) {
				if let line {
					HStack(alignment: .firstTextBaseline, spacing: 5) {
						if let symbol = line.tone.symbol {
							Image(systemName: symbol).accessibilityHidden(true)
						} else {
							ProgressView().controlSize(.mini).accessibilityHidden(true)
						}
						VStack(alignment: .leading, spacing: 2) {
							Text(line.text).fixedSize(horizontal: false, vertical: true)
							if let requestID = line.requestID {
								HStack(alignment: .firstTextBaseline, spacing: 4) {
									Text(t(DesktopCopy.originRequestLabel))
									Text(requestID)
										.monospaced()
										.textSelection(.enabled)
										.fixedSize(horizontal: false, vertical: true)
								}
							}
						}
					}
					.font(.caption)
					.foregroundStyle(line.tone.tint)
					.accessibilityElement(children: .combine)
					.accessibilityAddTraits(.updatesFrequently)
					if !line.actions.isEmpty {
						HStack(spacing: 12) {
							ForEach(Array(line.actions.enumerated()), id: \.offset) { _, action in
								Button(title(action)) { Task { await perform(action) } }
									.settingsLink()
									.font(.caption)
							}
						}
					}
				}
				if let warning {
					Label(warning, systemImage: NoticeSeverity.warning.symbol)
						.font(.caption)
						.foregroundStyle(DesktopVisualTheme.warning)
						.fixedSize(horizontal: false, vertical: true)
				}
			}
		}
	}

	private func title(_ action: OriginStatusLine.Action) -> String {
		switch action {
		case .readAgain: t(DesktopCopy.originActionReadAgain)
		case .retry: t(DesktopCopy.retry)
		case .checkResult: t(DesktopCopy.originActionCheckResult)
		case .checkAgain: t(DesktopCopy.originActionCheckAgain)
		case .retryOriginal: t(DesktopCopy.originActionRetryOriginal)
		case .loadLatest: t(DesktopCopy.originActionLoadLatest)
		}
	}

	private func perform(_ action: OriginStatusLine.Action) async {
		switch action {
		case .readAgain, .loadLatest: await controller.readLatestValues()
		case .retry, .retryOriginal: await controller.retry()
		case .checkResult, .checkAgain: await controller.readRequestReceipt()
		}
	}
}

/// The manual-classification sheet: exact single-session lookup and the list
/// of manual classifications. Escape or Done closes only the sheet; the
/// system sheet keeps keyboard focus inside it while it is open.
struct OriginManageSheet: View {
	@Bindable var controller: SessionOriginSettingsController
	@FocusState private var focus: OriginFocusTarget?
	/// The row whose removal is in flight and where it was, so focus moves to
	/// the next row once the list is read again.
	@State private var removal: OriginRemovalFocus?

	static var height: CGFloat {
		min(600, max(360, (NSScreen.main?.visibleFrame.height ?? 800) - 160))
	}

	var body: some View {
		VStack(alignment: .leading, spacing: 0) {
			VStack(alignment: .leading, spacing: 4) {
				Text(t(DesktopCopy.originManualTitle)).font(.headline)
				OriginCaption(t(DesktopCopy.originSheetDescription))
			}
			.padding(18)
			Divider()
			ScrollView(.vertical) {
				VStack(alignment: .leading, spacing: 16) {
					OriginStatusView(
						line: controller.sheetStatus,
						warning: controller.permissionWarningText(for: .manualClassification),
						controller: controller
					)
					find
					overridesSection
				}
				.padding(18)
				.frame(maxWidth: .infinity, alignment: .leading)
			}
			Divider()
			HStack {
				Spacer()
				// Return finds while the ID field has focus; elsewhere it is Done.
				Button(t(DesktopCopy.originDone)) { controller.closeManage() }
					.buttonStyle(.borderedProminent)
					// White text needs the stronger accent for 4.5:1 in both themes.
					.tint(DesktopVisualTheme.accentStrong)
					.keyboardShortcut(focus == .findSession ? nil : .defaultAction)
			}
			.padding(.horizontal, 18)
			.padding(.vertical, 12)
		}
		.frame(width: 460, height: Self.height)
		.modifier(AcceptanceAppearance())
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.surface)
		.onExitCommand { controller.closeManage() }
		.onAppear { if controller.writesEnabled { focus = .findSession } }
		.onChange(of: controller.operation) { restoreFocusAfterRemoval() }
		.onChange(of: controller.overrides) { restoreFocusAfterRemoval() }
	}

	// MARK: Find

	private var find: some View {
		VStack(alignment: .leading, spacing: 6) {
			sectionTitle(t(DesktopCopy.originFindTitle))
			HStack(spacing: 8) {
				Picker(
					t(DesktopCopy.clientFilter),
					selection: Binding(get: { controller.lookupClient }, set: { controller.setLookupClient($0) })
				) {
					ForEach(SessionOriginSettingsController.clients, id: \.self) { client in
						Text(OriginSettingsCopy.clientLabel(client)).tag(client)
					}
				}
				.settingsSegmented()
				.labelsHidden()
				.fixedSize()
				.disabled(!controller.writesEnabled)
				.accessibilityLabel(t(DesktopCopy.clientFilter))
				TextField(
					t(DesktopCopy.originSessionID),
					text: Binding(get: { controller.lookupID }, set: { controller.setLookupID($0) })
				)
				.textFieldStyle(.roundedBorder)
				.font(.body.monospaced())
				.focused($focus, equals: .findSession)
				.disabled(!controller.writesEnabled)
				.accessibilityLabel(t(DesktopCopy.originSessionID))
				.accessibilityHint(t(DesktopCopy.originFindHelp))
				.onSubmit { Task { await controller.findSession() } }
				Button(t(DesktopCopy.originFind)) { Task { await controller.findSession() } }
					.disabled(!controller.lookupEnabled)
					.keyboardShortcut(focus == .findSession ? .defaultAction : nil)
			}
			OriginCaption(t(DesktopCopy.originFindHelp))
			lookupResult
		}
	}

	@ViewBuilder
	private var lookupResult: some View {
		switch controller.lookup {
		case .idle, .searching:
			EmptyView()
		case .notFound:
			Label(t(DesktopCopy.originNotFound), systemImage: "info.circle.fill")
				.font(.caption)
				.foregroundStyle(DesktopVisualTheme.muted)
				.fixedSize(horizontal: false, vertical: true)
		case .unavailable:
			OriginCaption(t(DesktopCopy.originLookupUnavailable))
		case let .found(result):
			card(result)
		}
	}

	private func card(_ result: OriginLookupResult) -> some View {
		VStack(alignment: .leading, spacing: 8) {
			HStack(alignment: .firstTextBaseline, spacing: 8) {
				Text(result.target.sessionID)
					.font(.callout.monospaced())
					.textSelection(.enabled)
					.fixedSize(horizontal: false, vertical: true)
					.frame(maxWidth: .infinity, alignment: .leading)
				OriginTag(text: OriginSettingsCopy.clientLabel(result.target.client))
			}
			HStack(alignment: .center, spacing: 8) {
				Text(t(DesktopCopy.originCardClassification)).font(.caption)
				Spacer(minLength: 8)
				Picker(
					t(DesktopCopy.originCardClassificationLabel),
					selection: Binding(get: { result.choice }, set: { choice in Task { await controller.selectOrigin(choice) } })
				) {
					ForEach(OriginChoice.allCases) { choice in
						Text(OriginSettingsCopy.choiceLabel(choice)).tag(choice)
					}
				}
				.settingsSegmented()
				.labelsHidden()
				.fixedSize()
				.disabled(!controller.writesEnabled)
				.accessibilityLabel(t(DesktopCopy.originCardClassificationLabel))
				.accessibilityHint(t(DesktopCopy.originCardNote))
			}
			VStack(alignment: .leading, spacing: 4) {
				fact(t(DesktopCopy.originCountsAs)) {
					classification(OriginSettingsCopy.effectiveText(result), result)
				}
				fact(t(DesktopCopy.originDetected)) {
					classification(OriginSettingsCopy.automaticText(result.automatic), result)
						.help(OriginSettingsCopy.automaticHelp(result.automatic) ?? "")
				}
			}
			.font(.caption)
			if !result.classificationAvailable, result.classificationQuery != .reading {
				Button(t(DesktopCopy.originActionReadAgain)) { Task { await controller.readCurrentClassification() } }
					.settingsLink()
					.font(.caption)
					.disabled(!controller.writesEnabled)
			}
			OriginCaption(t(DesktopCopy.originCardNote))
		}
		.padding(10)
		.background(RoundedRectangle(cornerRadius: 8).fill(DesktopVisualTheme.background))
		.accessibilityElement(children: .contain)
	}

	/// One label/value line of the card, the labels in one fixed column.
	private func fact<Value: View>(_ label: String, @ViewBuilder value: () -> Value) -> some View {
		HStack(alignment: .firstTextBaseline, spacing: 12) {
			Text(label)
				.foregroundStyle(DesktopVisualTheme.dim)
				.frame(minWidth: 64, alignment: .leading)
				.fixedSize()
			value()
				.frame(maxWidth: .infinity, alignment: .leading)
		}
		.accessibilityElement(children: .combine)
	}

	/// The value, else "Reading…" while the separate query runs, else
	/// unavailable: never the value shown before a clear.
	@ViewBuilder
	private func classification(_ text: String?, _ result: OriginLookupResult) -> some View {
		if let text {
			Text(text).fixedSize(horizontal: false, vertical: true)
		} else if result.classificationQuery == .reading {
			Text(t(DesktopCopy.originReading)).foregroundStyle(DesktopVisualTheme.dim)
		} else {
			Text(t(DesktopCopy.originClassificationUnavailable)).foregroundStyle(DesktopVisualTheme.warning)
		}
	}

	// MARK: Manual classifications

	private var overridesSection: some View {
		VStack(alignment: .leading, spacing: 6) {
			HStack(spacing: 6) {
				sectionTitle(t(DesktopCopy.originOverridesTitle))
				if case let .loaded(page) = controller.overrides, page.total > 0 {
					OriginTag(text: String(page.total))
				}
			}
			overridesContent
		}
	}

	@ViewBuilder
	private var overridesContent: some View {
		switch controller.overrides {
		case .loading:
			EmptyView()
		case .unavailable:
			emptyBox {
				Text(t(DesktopCopy.originOverridesUnavailable))
				Button(t(DesktopCopy.originActionReadAgain)) { Task { await controller.readLatestValues() } }
					.settingsLink()
			}
		case let .loaded(page):
			if page.items.isEmpty {
				emptyBox { Text(t(DesktopCopy.originOverridesEmpty)) }
			} else {
				VStack(spacing: 0) {
					ForEach(Array(page.items.enumerated()), id: \.element.target) { index, item in
						if index > 0 { Divider() }
						overrideRow(item, index: index)
					}
				}
				.background(RoundedRectangle(cornerRadius: 8).fill(DesktopVisualTheme.background))
				if page.pageCount > 1 {
					HStack(spacing: 8) {
						Button(t(DesktopCopy.originPreviousPage)) { Task { await controller.showOverridePage(page.page - 1) } }
							.disabled(page.page <= 1 || !controller.writesEnabled)
						Text(t(DesktopCopy.originPage, Int64(page.page), Int64(page.pageCount))).font(.caption)
						Button(t(DesktopCopy.originNextPage)) { Task { await controller.showOverridePage(page.page + 1) } }
							.disabled(page.page >= page.pageCount || !controller.writesEnabled)
					}
				}
			}
		}
	}

	private func overrideRow(_ item: DesktopOriginOverrideV1, index: Int) -> some View {
		let client = OriginSettingsCopy.clientLabel(item.client)
		let identity = HStack(alignment: .firstTextBaseline, spacing: 6) {
			OriginTag(text: client)
			Text(item.sessionID)
				.font(.caption.monospaced())
				.multilineTextAlignment(.leading)
				.fixedSize(horizontal: false, vertical: true)
		}
		return HStack(alignment: .firstTextBaseline, spacing: 8) {
			Group {
				if item.matched {
					// A matched row opens its session in the lookup above.
					Button { Task { await controller.openOverride(item) } } label: { identity }
						.buttonStyle(.plain)
						.disabled(!controller.writesEnabled)
						.accessibilityLabel(t(DesktopCopy.originShowLabel, client, item.sessionID))
				} else {
					// Never applied to another session; only the record can go.
					VStack(alignment: .leading, spacing: 2) {
						identity
						Label(t(DesktopCopy.originUnmatched), systemImage: NoticeSeverity.warning.symbol)
							.font(.caption2)
							.foregroundStyle(DesktopVisualTheme.warning)
							.fixedSize(horizontal: false, vertical: true)
					}
				}
			}
			.frame(maxWidth: .infinity, alignment: .leading)
			OriginTag(text: SessionOriginCopy.classLabel(item.value))
			if controller.isRemoving(item) {
				ProgressView().controlSize(.small).frame(minWidth: 40)
			} else {
				Button(t(DesktopCopy.originRemove)) {
					removal = OriginRemovalFocus(target: item.target, index: index)
					Task { await controller.removeOverride(item) }
				}
				.buttonStyle(.borderless)
				.foregroundStyle(DesktopVisualTheme.muted)
				.font(.caption)
				.focused($focus, equals: .removeOverride(item.target))
				.disabled(!controller.writesEnabled)
				.accessibilityLabel(t(DesktopCopy.originRemoveLabel, client, item.sessionID))
			}
		}
		.padding(.vertical, 6)
		.padding(.horizontal, 10)
	}

	/// After a removal settles: the next row, else the previous one, else the
	/// lookup entry. A failed removal keeps focus on its own row; an unknown
	/// one waits until its outcome is read back.
	private func restoreFocusAfterRemoval() {
		guard let removal else { return }
		switch controller.operation {
		case .saving, .unknown, .conflict: return
		case .idle, .failed: break
		}
		guard case let .loaded(page) = controller.overrides else { return }
		focus = OriginSettingsCopy.focusAfterRemoval(of: removal.target, at: removal.index, remaining: page.items)
		self.removal = nil
	}

	private func sectionTitle(_ text: String) -> some View {
		Text(text)
			.font(.caption.weight(.semibold))
			.foregroundStyle(DesktopVisualTheme.dim)
	}

	private func emptyBox<Content: View>(@ViewBuilder _ content: () -> Content) -> some View {
		VStack(spacing: 6) { content() }
			.font(.caption)
			.foregroundStyle(DesktopVisualTheme.dim)
			.multilineTextAlignment(.center)
			.frame(maxWidth: .infinity)
			.padding(14)
			.overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(DesktopVisualTheme.line, style: StrokeStyle(lineWidth: 1, dash: [4, 3])))
	}
}

/// A small neutral capsule: a client, a class or a count.
private struct OriginTag: View {
	let text: String

	var body: some View {
		Text(text)
			.font(.caption2.weight(.medium))
			.foregroundStyle(DesktopVisualTheme.muted)
			.padding(.horizontal, 6)
			.padding(.vertical, 1)
			.background(Capsule().fill(DesktopVisualTheme.surfaceEmphasis))
			.fixedSize()
	}
}

struct OriginRemovalFocus: Equatable {
	let target: DesktopOriginTargetV1
	let index: Int
}
