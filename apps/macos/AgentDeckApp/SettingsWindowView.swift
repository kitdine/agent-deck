import SwiftUI

struct SettingsWindowView: View {
	@Bindable var preferences: DesktopPreferences
	var quotaSettings: QuotaSettingsController

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
					label: t(DesktopCopy.settingsPeriodicRefresh),
					note: t(DesktopCopy.settingsPeriodicRefreshNote),
					status: nil
				) {
					Toggle(t(DesktopCopy.settingsPeriodicRefresh), isOn: $preferences.periodicRefreshEnabled)
						.toggleStyle(.switch)
						.tint(DesktopVisualTheme.accent)
						.labelsHidden()
						.accessibilityLabel(t(DesktopCopy.settingsPeriodicRefresh))
				}
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
					.accessibilityLabel(t(DesktopCopy.settingsQuotaProbe))
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
					.disabled(!preferences.quotaProbeEnabled)
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
					.disabled(!(quotaSettings.settings?.alerts ?? false))
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
