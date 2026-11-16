import Darwin
import Foundation
import CoreServices

/// A lifetime-bound event source. All scan scheduling, merging and cancellation
/// remain owned by DesktopRefreshCoordinator. No timer or second scanner lives here.
@MainActor
public final class DesktopSourceChangeMonitor {
	private let roots: [String]
	private let configuration: Bool
	private let request: @MainActor @Sendable () -> Void
	private var stream: FSEventStreamRef?

	public init(home: URL, request: @escaping @MainActor @Sendable () -> Void) {
		configuration = false
		roots = [".codex/sessions", ".codex/archived_sessions", ".claude/projects"].map {
			home.appendingPathComponent($0).standardizedFileURL.path
		}
		self.request = request
	}

	/// Observe one configuration file, including atomic replacement and creation.
	/// This stream is independent of source watching and the listener lifetime.
	public init(configurationURL: URL, request: @escaping @MainActor @Sendable () -> Void) {
		roots = [configurationURL.standardizedFileURL.path]
		configuration = true
		self.request = request
	}

	@discardableResult
	public func start() -> Bool {
		guard stream == nil else { return true }
		// Watch existing ancestors so a source directory created after launch
		// is also observed. Filter every callback back to the three source roots.
		let ancestors = Set(roots.map { path in
			var url = URL(fileURLWithPath: path)
			url.deleteLastPathComponent()
			while !FileManager.default.fileExists(atPath: url.path) && url.path != "/" {
				url.deleteLastPathComponent()
			}
			return url.path
		})
		var context = FSEventStreamContext(version: 0,
			info: Unmanaged.passUnretained(self).toOpaque(), retain: nil, release: nil, copyDescription: nil)
		let callback: FSEventStreamCallback = { _, info, count, paths, flags, _ in
			guard let info else { return }
			let monitor = Unmanaged<DesktopSourceChangeMonitor>.fromOpaque(info).takeUnretainedValue()
			let changed = unsafeBitCast(paths, to: NSArray.self) as! [String]
			MainActor.assumeIsolated {
				monitor.receive(paths: changed, flags: flags, count: count)
			}
		}
		guard let created = FSEventStreamCreate(nil, callback, &context, Array(ancestors) as CFArray,
			FSEventStreamEventId(kFSEventStreamEventIdSinceNow), 0.1,
			FSEventStreamCreateFlags(kFSEventStreamCreateFlagUseCFTypes | kFSEventStreamCreateFlagFileEvents)) else { return false }
		FSEventStreamSetDispatchQueue(created, .main)
		guard FSEventStreamStart(created) else {
			FSEventStreamInvalidate(created)
			FSEventStreamRelease(created)
			return false
		}
		stream = created
		return true
	}

	private func receive(paths: [String], flags: UnsafePointer<FSEventStreamEventFlags>, count: Int) {
		guard stream != nil else { return }
		let recovery = FSEventStreamEventFlags(kFSEventStreamEventFlagMustScanSubDirs | kFSEventStreamEventFlagRootChanged)
		let relevant = (0..<count).contains { index in
			let path = URL(fileURLWithPath: paths[index]).standardizedFileURL.path
			return flags[index] & recovery != 0 || roots.contains { root in
				(path == root || path.hasPrefix(root + "/")) &&
					(configuration || path.hasSuffix(".jsonl") || flags[index] & FSEventStreamEventFlags(kFSEventStreamEventFlagItemIsDir) != 0)
			}
		}
		guard relevant else { return }
		PerfTrace.mark(.sourceFSEvent)
		request()
	}

	public func stop() {
		guard let stream else { return }
		self.stream = nil
		FSEventStreamStop(stream)
		FSEventStreamInvalidate(stream)
		FSEventStreamRelease(stream)
	}
}

/// Automatic local-data updates are event driven: a source change requests a
/// finite update, and start, wake and re-enable request one finite
/// reconciliation for changes no event reported. There is no refresh period.
/// While disabled nothing is watched; startup, manual and CLI updates remain.
@MainActor
public final class DesktopAutomaticUpdates {
	public private(set) var isEnabled: Bool
	public private(set) var isWatching = false

	private let startWatching: @MainActor () -> Bool
	private let stopWatching: @MainActor () -> Void
	private let request: @MainActor @Sendable (DesktopFullRefreshTrigger) -> Void
	private var started = false

	public init(
		enabled: Bool,
		startWatching: @escaping @MainActor () -> Bool,
		stopWatching: @escaping @MainActor () -> Void,
		request: @escaping @MainActor @Sendable (DesktopFullRefreshTrigger) -> Void
	) {
		isEnabled = enabled
		self.startWatching = startWatching
		self.stopWatching = stopWatching
		self.request = request
	}

	/// Called once the startup restore and its finite update settle; that
	/// startup update is the launch reconciliation, so none is requested here.
	public func start() {
		guard !started else { return }
		started = true
		if isEnabled { watch() }
	}

	public func setEnabled(_ enabled: Bool) {
		guard enabled != isEnabled else { return }
		isEnabled = enabled
		guard started else { return }
		if enabled {
			watch()
			request(.wake)
		} else {
			unwatch()
		}
	}

	public func didWake() {
		guard started, isEnabled else { return }
		request(.wake)
	}

	public func stop() {
		started = false
		unwatch()
	}

	private func watch() {
		guard !isWatching else { return }
		isWatching = startWatching()
	}

	private func unwatch() {
		guard isWatching else { return }
		isWatching = false
		stopWatching()
	}
}

/// The quota probe's own schedule; it never requests a full data update.
@MainActor
public final class DesktopRefreshPeriodicDriver {
	public static let evaluatorInterval: Duration = .seconds(30)

	private let requestQuota: @MainActor @Sendable () async -> Void
	private var quotaTask: Task<Void, Never>?

	public var hasActiveQuotaRequest: Bool { quotaTask != nil }

	public init(requestQuota: @escaping @MainActor @Sendable () async -> Void) {
		self.requestQuota = requestQuota
	}

	public func tick(quotaEnabled: Bool) async {
		guard quotaEnabled, quotaTask == nil else { return }
		quotaTask = Task { [weak self] in
			guard let self else { return }
			await self.requestQuota()
			self.quotaTask = nil
		}
	}

	public func cancel() {
		quotaTask?.cancel()
	}

	public func waitForQuotaIdle() async {
		await quotaTask?.value
	}
}
