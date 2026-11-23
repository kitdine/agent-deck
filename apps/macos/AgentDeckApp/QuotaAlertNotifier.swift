import AgentDeckShared
import AppKit
import Darwin
import UserNotifications

/// Notification permission as the quota alerts need it (ux/settings-quota.md:
/// "When notifications are not allowed"). Injected so hosted tests never reach
/// the real notification service or trigger its permission prompt.
protocol NotificationPermissionChecking: Sendable {
	func authorizationGranted() async -> Bool
	func requestAuthorization() async -> Bool
}

protocol NotificationPosting: Sendable {
	func post(identifier: String, title: String, body: String, completion: @escaping @Sendable (Bool) -> Void)
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

	func post(identifier: String, title: String, body: String, completion: @escaping @Sendable (Bool) -> Void) {
		let content = UNMutableNotificationContent()
		content.title = title
		content.body = body
		content.sound = .default
		UNUserNotificationCenter.current().add(UNNotificationRequest(identifier: identifier, content: content, trigger: nil),
		                                       withCompletionHandler: { completion($0 == nil) })
	}
}

/// Posts due quota alerts (architecture.md C10). Only alerts the service
/// accepted are returned for acknowledgement; without permission nothing is
/// posted, so every alert stays due and is offered again by the next refresh.
struct QuotaAlertNotifier: QuotaAlertDelivering {
	let permission: any NotificationPermissionChecking
	let poster: any NotificationPosting
	var stateRoot: (@Sendable () async throws -> URL)? = nil
	var lockWait: Duration = .seconds(1)
	var callbackWait: Duration = .seconds(5)
	var onLockContention: (@Sendable () -> Void)? = nil

	func deliver(_ alerts: [DesktopQuotaAlertV1]) async -> [String] {
		guard !alerts.isEmpty, await permission.authorizationGranted() else {
			return []
		}
		var seen = Set<String>()
		let unique = alerts.filter { seen.insert($0.id).inserted }
		let prepared = await MainActor.run { unique.map { ($0, QuotaAlertContent($0)) } }
		let (events, sink) = AsyncStream<(String, Bool, ContinuousClock.Instant)>.makeStream()
		var pending = Set<String>()
		func submit(_ alert: DesktopQuotaAlertV1, _ content: QuotaAlertContent) {
			pending.insert(alert.id)
			poster.post(identifier: alert.id, title: content.title, body: content.body) { success in
				sink.yield((alert.id, success, .now))
			}
		}
		for (alert, content) in prepared where alert.client != "claude" { submit(alert, content) }
		let guarded = prepared.filter { $0.0.client == "claude" && $0.0.stateRootID != nil && ($0.0.generation ?? -1) >= 0 }
		let guardDeadline = ContinuousClock.now.advanced(by: min(lockWait, .seconds(1)))
		if !guarded.isEmpty, let stateRoot, let root = try? await stateRoot() {
			await withQuotaEpoch(root, deadline: guardDeadline, onContention: onLockContention) { identity, epoch, stillBound in
				for (alert, content) in guarded where alert.stateRootID == identity && alert.generation == epoch {
					guard stillBound() else { return }
					submit(alert, content)
				}
			}
		}
		let deadline = ContinuousClock.now.advanced(by: callbackWait)
		let timeout = Task {
			try await Task.sleep(until: deadline, clock: .continuous)
			sink.finish()
		}
		defer { timeout.cancel(); sink.finish() }
		var accepted = Set<String>()
		if !pending.isEmpty {
			for await (id, success, at) in events {
				if pending.remove(id) != nil, success, at <= deadline { accepted.insert(id) }
				if pending.isEmpty { break }
			}
		}
		return unique.compactMap { accepted.contains($0.id) ? $0.id : nil }
	}
}

func isPrivateQuotaDeliveryFile(_ st: stat) -> Bool {
	st.st_mode & S_IFMT == S_IFREG && st.st_uid == geteuid() && st.st_mode & 0o7777 == 0o600 && st.st_nlink == 1
}

// Only the wait for flock may suspend. Closing the descriptor releases the
// lock after the synchronous submission body, including every failure path.
private func withQuotaEpoch(_ root: URL, deadline: ContinuousClock.Instant, onContention: (@Sendable () -> Void)?,
                            body: (String, Int64, () -> Bool) -> Void) async {
	func inTime() -> Bool { !Task.isCancelled && ContinuousClock.now < deadline }
	let path = root.path
	guard inTime(), root.isFileURL, path.hasPrefix("/"), !path.utf8.contains(0),
	      let physical = realpath(path, nil) else { return }
	defer { free(physical) }
	guard inTime() else { return }
	let dir = open(physical, O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC)
	guard dir >= 0 else { return }
	defer { close(dir) }
	let flags = O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC
	let name = "quota-delivery.lock"
	var named = stat()
	guard inTime(), fstatat(dir, name, &named, AT_SYMLINK_NOFOLLOW) == 0, isPrivateQuotaDeliveryFile(named), inTime() else { return }
	let file = openat(dir, name, flags)
	guard file >= 0 else { return }
	defer { close(file) }
	var observeContention = onContention
	while true {
		guard inTime() else { return }
		if flock(file, LOCK_EX | LOCK_NB) == 0 { break }
		guard errno == EWOULDBLOCK || errno == EINTR else { return }
		observeContention?()
		observeContention = nil
		do { try await Task.sleep(until: min(deadline, ContinuousClock.now.advanced(by: .milliseconds(10))), clock: .continuous) }
		catch { return }
	}
	func verifiedRootID() -> String? {
		var held = stat(), resolved = stat(), supplied = stat(), guardInfo = stat(), entry = stat()
		guard inTime(), fstat(dir, &held) == 0, lstat(physical, &resolved) == 0, stat(path, &supplied) == 0,
		      held.st_dev == resolved.st_dev, held.st_ino == resolved.st_ino,
		      held.st_dev == supplied.st_dev, held.st_ino == supplied.st_ino,
		      held.st_uid == geteuid(), held.st_mode & S_IFMT == S_IFDIR, held.st_mode & 0o7777 == 0o700,
		      fstat(file, &guardInfo) == 0, fstatat(dir, name, &entry, AT_SYMLINK_NOFOLLOW) == 0,
		      isPrivateQuotaDeliveryFile(guardInfo), isPrivateQuotaDeliveryFile(entry), guardInfo.st_dev == entry.st_dev, guardInfo.st_ino == entry.st_ino else { return nil }
		var pending = stat()
		guard fstatat(dir, ".quota-delivery.lock.pending", &pending, AT_SYMLINK_NOFOLLOW) == -1, errno == ENOENT else { return nil }
		var bytes = [UInt8](repeating: 0, count: 16)
		let count = bytes.withUnsafeMutableBytes { fgetxattr(dir, "user.agentdeck.quota-delivery-lock", $0.baseAddress, $0.count, 0, 0) }
		guard count == 16, bytes.prefix(8).reduce(UInt64(0), { ($0 << 8) | UInt64($1) }) == UInt64(bitPattern: Int64(guardInfo.st_dev)),
		      bytes.suffix(8).reduce(UInt64(0), { ($0 << 8) | UInt64($1) }) == guardInfo.st_ino else { return nil }
		return "\(UInt64(bitPattern: Int64(held.st_dev))):\(held.st_ino)"
	}
	guard let identity = verifiedRootID(), inTime() else { return }
	let image = openat(dir, "quota-delivery.epoch", flags)
	guard image >= 0 else { return }
	defer { close(image) }
	var info = stat()
	guard inTime(), fstat(image, &info) == 0, isPrivateQuotaDeliveryFile(info), info.st_size == 24 else { return }
	var bytes = [UInt8](repeating: 0, count: 24)
	guard inTime(), read(image, &bytes, 24) == 24, bytes.starts(with: "AGENTDECK-QDG-1\n".utf8),
	      let epoch = Int64(exactly: bytes.suffix(8).reduce(UInt64(0)) { ($0 << 8) | UInt64($1) }) else { return }
	body(identity, epoch) { verifiedRootID() == identity && inTime() }
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
