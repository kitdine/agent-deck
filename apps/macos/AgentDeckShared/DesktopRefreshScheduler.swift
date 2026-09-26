import Darwin
import Foundation

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
