import SwiftUI

struct SettingsWindowView: View {
	@Bindable var preferences: DesktopPreferences

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
		}
		.padding(18)
		.frame(width: 460, alignment: .leading)
		.modifier(AcceptanceAppearance())
		.tint(DesktopVisualTheme.accent)
		.foregroundStyle(DesktopVisualTheme.text)
		.background(DesktopVisualTheme.background)
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

struct SettingsRow<Control: View>: View {
	let label: String
	let note: String
	let status: SettingsRowStatus?
	@ViewBuilder let control: () -> Control

	var body: some View {
		HStack(alignment: .firstTextBaseline, spacing: MenuBarGeometry.betweenSections) {
			VStack(alignment: .leading, spacing: MenuBarGeometry.withinRow) {
				Text(label).font(.body)
				Text(note)
					.font(.caption)
					.foregroundStyle(DesktopVisualTheme.dim)
					.fixedSize(horizontal: false, vertical: true)
				if let status {
					Label(status.text, systemImage: status.severity.symbol)
						.font(.caption)
						.foregroundStyle(status.severity.tint)
						.fixedSize(horizontal: false, vertical: true)
				}
			}
			.frame(maxWidth: .infinity, alignment: .leading)
			.accessibilityElement(children: .combine)
			control()
		}
		.frame(minHeight: MenuBarGeometry.rowMinimumHeight)
	}
}
