import AgentDeckShared
import AppKit
import UserNotifications

/// Notification permission as the quota alerts need it (ux/settings-quota.md:
/// "When notifications are not allowed"). Injected so hosted tests never reach
/// the real notification service or trigger its permission prompt.
protocol NotificationPermissionChecking: Sendable {
	func authorizationGranted() async -> Bool
	func requestAuthorization() async -> Bool
}

protocol NotificationPosting: Sendable {
	func post(identifier: String, title: String, body: String) async throws
}

/// The user-notification service under the app's own bundle identity.
struct SystemUserNotifications: NotificationPermissionChecking, NotificationPosting {
	func authorizationGranted() async -> Bool {
		let settings = await UNUserNotificationCenter.current().notificationSettings()
		switch settings.authorizationStatus {
		case .authorized, .provisional:
			return true
		default:
			return false
		}
	}

	func requestAuthorization() async -> Bool {
		(try? await UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound])) ?? false
	}

	func post(identifier: String, title: String, body: String) async throws {
		let content = UNMutableNotificationContent()
		content.title = title
		content.body = body
		content.sound = .default
		try await UNUserNotificationCenter.current().add(UNNotificationRequest(identifier: identifier, content: content, trigger: nil))
	}
}

/// Posts due quota alerts (architecture.md C10). Only alerts the service
/// accepted are returned for acknowledgement; without permission nothing is
/// posted, so every alert stays due and is offered again by the next refresh.
struct QuotaAlertNotifier: QuotaAlertDelivering {
	let permission: any NotificationPermissionChecking
	let poster: any NotificationPosting

	func deliver(_ alerts: [DesktopQuotaAlertV1]) async -> [String] {
		guard !alerts.isEmpty, await permission.authorizationGranted() else {
			return []
		}
		var delivered: [String] = []
		for alert in alerts {
			let content = await MainActor.run { QuotaAlertContent(alert) }
			do {
				try await poster.post(identifier: alert.id, title: content.title, body: content.body)
				delivered.append(alert.id)
			} catch {
				continue
			}
		}
		return delivered
	}
}

/// Localized notification copy in the app's language. It names the client,
/// the window, and the figure, and nothing else — the wire carries no account
/// identifier to name.
struct QuotaAlertContent: Equatable {
	let title: String
	let body: String

	@MainActor
	init(_ alert: DesktopQuotaAlertV1) {
		title = t(DesktopCopy.notificationQuotaTitle, Self.clientName(alert.client))
		let window = Self.windowName(label: alert.label, minutes: alert.windowMinutes)
		let used = Int64(alert.usedPercent.rounded())
		switch alert.kind {
		case .threshold:
			body = t(DesktopCopy.notificationQuotaThresholdBody, window, used, Int64((alert.threshold ?? 0).rounded()))
		case .reset:
			body = t(DesktopCopy.notificationQuotaResetBody, window, used)
		}
	}

	private static func clientName(_ client: String) -> String {
		switch client {
		case "codex": "Codex"
		case "claude": "Claude"
		default: client
		}
	}

	// A Codex limit's primary and secondary windows share the same vendor
	// label: the span is always appended, never replaced by the label, or a
	// limit's two notices would name the same window.
	@MainActor
	private static func windowName(label: String?, minutes: Int?) -> String {
		let span: String
		switch minutes {
		case 300: span = t(DesktopCopy.quotaWindow5h)
		case 10080: span = t(DesktopCopy.quotaWindow7d)
		case let .some(value): span = "\(value)m"
		case .none: span = t(DesktopCopy.notificationQuotaWindowFallback)
		}
		guard let label, !label.isEmpty else { return span }
		return "\(label) · \(span)"
	}
}

/// AgentDeck's page in System Settings › Notifications.
func openNotificationSettings() {
	let identifier = Bundle.main.bundleIdentifier ?? "com.kitdine.agentdeck"
	guard let url = URL(string: "x-apple.systempreferences:com.apple.Notifications-Settings.extension?id=\(identifier)") else {
		return
	}
	NSWorkspace.shared.open(url)
}
