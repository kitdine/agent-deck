import Darwin
import Foundation
import CoreServices

/// A lifetime-bound event source. All scan scheduling, merging and cancellation
/// remain owned by DesktopRefreshCoordinator. No timer or second scanner lives here.
@MainActor
public final class DesktopSourceChangeMonitor {
	private let roots: [String]
	private let request: @MainActor @Sendable () -> Void
	private var stream: FSEventStreamRef?

	public init(home: URL, request: @escaping @MainActor @Sendable () -> Void) {
		roots = [".codex/sessions", ".codex/archived_sessions", ".claude/projects"].map {
			home.appendingPathComponent($0).standardizedFileURL.path
		}
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
					(path.hasSuffix(".jsonl") || flags[index] & FSEventStreamEventFlags(kFSEventStreamEventFlagItemIsDir) != 0)
			}
		}
		guard relevant else { return }
		DesktopLogger.recordPerformance(DesktopPerformanceInterval().sample(.sourceNotification))
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

public enum DesktopRefreshSchedulerReason: Equatable, Sendable {
	case periodic
	case wake

	var trigger: DesktopFullRefreshTrigger {
		switch self {
		case .periodic: .periodic
		case .wake: .wake
		}
	}
}

@MainActor
public final class DesktopRefreshScheduler {
	public static let fullRefreshInterval: TimeInterval = 60
	public static let evaluatorInterval: Duration = .seconds(30)

	public private(set) var deadline: TimeInterval?
	public private(set) var isEnabled: Bool

	private let now: @MainActor @Sendable () -> TimeInterval
	private let request: @MainActor @Sendable (DesktopFullRefreshTrigger) -> Void

	public init(
		enabled: Bool,
		now: (@MainActor @Sendable () -> TimeInterval)? = nil,
		request: @escaping @MainActor @Sendable (DesktopFullRefreshTrigger) -> Void
	) {
		isEnabled = enabled
		self.now = now ?? { Self.sleepInclusiveUptime() }
		self.request = request
	}

	private static func sleepInclusiveUptime() -> TimeInterval {
		var timebase = mach_timebase_info_data_t()
		_ = mach_timebase_info(&timebase)
		return Double(mach_continuous_time())
			* Double(timebase.numer) / Double(timebase.denom) / 1_000_000_000
	}

	public func updateEnabled(_ enabled: Bool) {
		guard enabled != isEnabled else { return }
		isEnabled = enabled
		deadline = enabled ? now() + Self.fullRefreshInterval : nil
	}

	public func fullAttemptCompleted() {
		deadline = isEnabled ? now() + Self.fullRefreshInterval : nil
	}

	public func evaluate(_ reason: DesktopRefreshSchedulerReason) {
		guard isEnabled, let deadline, now() >= deadline else { return }
		self.deadline = nil
		request(reason.trigger)
	}
}

@MainActor
public final class DesktopRefreshPeriodicDriver {
	private let scheduler: DesktopRefreshScheduler
	private let requestQuota: @MainActor @Sendable () async -> Void
	private var quotaTask: Task<Void, Never>?

	public var hasActiveQuotaRequest: Bool { quotaTask != nil }

	public init(
		scheduler: DesktopRefreshScheduler,
		requestQuota: @escaping @MainActor @Sendable () async -> Void
	) {
		self.scheduler = scheduler
		self.requestQuota = requestQuota
	}

	public func tick(
		_ reason: DesktopRefreshSchedulerReason = .periodic,
		quotaEnabled: Bool
	) async {
		scheduler.evaluate(reason)
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
