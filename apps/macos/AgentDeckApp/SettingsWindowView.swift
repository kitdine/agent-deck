import AppKit
import SwiftUI

struct SettingsWindowView: View {
	@Bindable var preferences: DesktopPreferences
	var quotaSettings: QuotaSettingsController
	var telemetrySettings: TelemetrySettingsController

	var body: some View {
		VStack(alignment: .leading, spacing: 18) {
			group(t(DesktopCopy.settingsGroupGeneral)) {
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

			group(t(DesktopCopy.settingsGroupLiveUsage)) {
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

			group(t(DesktopCopy.settingsGroupMenuBar)) {
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
						.pickerStyle(.segmented)
						.tint(DesktopVisualTheme.accent)
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
						.pickerStyle(.segmented)
						.tint(DesktopVisualTheme.accent)
					.labelsHidden()
					.fixedSize()
					.accessibilityLabel(t(DesktopCopy.settingsMenuBarScope))
				}
			}

			group(t(DesktopCopy.settingsGroupQuota)) {
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
					.pickerStyle(.segmented)
					.tint(DesktopVisualTheme.accent)
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
					.pickerStyle(.segmented)
					.tint(DesktopVisualTheme.accent)
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
		}
		.padding(18)
		.frame(width: 460, alignment: .leading)
		.modifier(AcceptanceAppearance())
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
		.task { await quotaSettings.load() }
		.task { await telemetrySettings.load() }
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

	private func group<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
		VStack(alignment: .leading, spacing: MenuBarGeometry.betweenRows) {
			Text(title)
				.font(.caption.weight(.semibold))
				.foregroundStyle(DesktopVisualTheme.dim)
			content()
		}
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
						.buttonStyle(.link)
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
					Button(t(DesktopCopy.settingsOTelSelfConfigure)) { Task { await controller.configureMyself() } }
					Button(t(DesktopCopy.switchCancel)) { controller.cancelConsent() }
				}
			case .conflict:
				caption(t(DesktopCopy.settingsOTelConflict))
				HStack {
					Button(t(DesktopCopy.settingsOTelReceiverOnly)) { Task { await controller.enableReceiverOnly() } }
						.buttonStyle(.borderedProminent)
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
