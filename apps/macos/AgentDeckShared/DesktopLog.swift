import Foundation
import OSLog
import Synchronization

/// Closed, content-free stages. Uptime measures elapsed work, never data freshness.
public enum DesktopPerformanceStage: String, Codable, Hashable, Sendable {
	case quota, scanRequest, workerDrain, snapshotRequest, snapshotDecode, publication, presentation, popoverPreparation, popoverOpen
}

public struct DesktopPerformanceSample: Codable, Sendable {
	public let stage: DesktopPerformanceStage
	public let durationNanoseconds: UInt64
	public let completedAtUptimeNanoseconds: UInt64
}

public struct DesktopPerformanceAggregate: Codable, Sendable {
	public let stage: DesktopPerformanceStage
	public var count: UInt64
	public var totalNanoseconds: UInt64
	public var maximumNanoseconds: UInt64
}

public struct DesktopPerformanceFailure: Codable, Sendable {
	public let stage: DesktopPerformanceStage
	public let code: String
	public let exitStatus: Int32
	public let stdoutBytes: Int
	public let stderrBytes: Int
	public init(stage: DesktopPerformanceStage, code: String, exitStatus: Int32, stdoutBytes: Int, stderrBytes: Int) {
		self.stage = stage
		self.code = code.utf8.count <= 64 && code.utf8.allSatisfy({
			($0 >= 65 && $0 <= 90) || ($0 >= 97 && $0 <= 122) || ($0 >= 48 && $0 <= 57) || $0 == 95 || $0 == 46
		}) ? code : "unclassified"
		self.exitStatus = exitStatus
		self.stdoutBytes = stdoutBytes
		self.stderrBytes = stderrBytes
	}
}

public struct DesktopPerformanceInterval: Sendable {
	private let startedAt: UInt64
	public init(startedAt: UInt64 = DispatchTime.now().uptimeNanoseconds) {
		self.startedAt = startedAt
	}
	public func sample(_ stage: DesktopPerformanceStage, endedAt: UInt64 = DispatchTime.now().uptimeNanoseconds) -> DesktopPerformanceSample {
		DesktopPerformanceSample(stage: stage, durationNanoseconds: endedAt >= startedAt ? endedAt - startedAt : 0,
			completedAtUptimeNanoseconds: endedAt)
	}
}

/// Bounded capture is explicit and used by the isolated native baseline harness.
/// No envelope, identifiers, paths, helper output or recognized text is retained.
public enum DesktopPerformanceTrace {
	private struct Capture: Sendable {
		var samples = [DesktopPerformanceSample]()
		var dropped = 0
		var failures = [DesktopPerformanceFailure]()
		var aggregates = [DesktopPerformanceStage: DesktopPerformanceAggregate]()
	}
	private static let capture = Mutex<Capture?>(nil)
	private static let capturing = Atomic<Bool>(false)
	static var isCapturing: Bool { capturing.load(ordering: .relaxed) }
	public static func start() {
		capture.withLock { $0 = Capture() }
		capturing.store(true, ordering: .relaxed)
	}
	public static func finish() -> (samples: [DesktopPerformanceSample], dropped: Int, failures: [DesktopPerformanceFailure], aggregates: [DesktopPerformanceAggregate]) {
		capturing.store(false, ordering: .relaxed)
		return capture.withLock {
			let result = ($0?.samples ?? [], $0?.dropped ?? 0, $0?.failures ?? [],
				($0?.aggregates.values.map { $0 } ?? []).sorted { $0.stage.rawValue < $1.stage.rawValue })
			$0 = nil
			return result
		}
	}
	public static func recordFailure(_ failure: DesktopPerformanceFailure) {
		guard isCapturing else { return }
		capture.withLock {
			guard $0 != nil, $0!.failures.count < 16 else { return }
			$0!.failures.append(failure)
		}
	}
	static func append(_ sample: DesktopPerformanceSample) {
		capture.withLock {
			guard $0 != nil else { return }
			var aggregate = $0!.aggregates[sample.stage] ?? DesktopPerformanceAggregate(
				stage: sample.stage, count: 0, totalNanoseconds: 0, maximumNanoseconds: 0)
			aggregate.count += 1
			aggregate.totalNanoseconds += sample.durationNanoseconds
			aggregate.maximumNanoseconds = max(aggregate.maximumNanoseconds, sample.durationNanoseconds)
			$0!.aggregates[sample.stage] = aggregate
			// Hot computed properties must not crowd out helper/publication stages.
			// Their complete count/total/max stays in the closed-stage map.
			guard sample.stage != .presentation else { return }
			if $0!.samples.count < 1024 { $0!.samples.append(sample) }
			else { $0!.dropped += 1 }
		}
	}
}

public enum DesktopLogPolicy {
    public static let subsystem = "com.kitdine.agentdeck"

    // These classifications intentionally carry no helper output, paths,
    // session identifiers, provider configuration, or snapshot fields.
    public static func snapshotEvent(for envelope: DesktopWireEnvelopeV1) -> String {
        envelope.partial ? "desktop_snapshot_partial" : "desktop_snapshot_complete"
    }

    public static func helperFailureEvent(_ error: HelperExecutionError) -> String {
        switch error {
        case .missingEmbeddedHelper:
            return "embedded_helper_missing"
        case .invalidRecentLimit:
            return "embedded_helper_invalid_request"
        case .launchFailed:
            return "embedded_helper_launch_failed"
        case .timedOut:
            return "embedded_helper_timed_out"
        case .cancelled:
            return "embedded_helper_cancelled"
        case .nonZeroExit:
            return "embedded_helper_failed"
        case .outputLimitExceeded:
            return "embedded_helper_output_limited"
        case .malformedOutput:
            return "embedded_helper_invalid_output"
        }
    }
}

public enum DesktopLogger {
    private static let logger = Logger(subsystem: DesktopLogPolicy.subsystem, category: "desktop")
	private static let performanceEnabled = ProcessInfo.processInfo.environment["AGENTDECK_PERFORMANCE"] == "1"

	public static func recordPerformance(_ sample: DesktopPerformanceSample) {
		guard performanceEnabled || DesktopPerformanceTrace.isCapturing else { return }
		DesktopPerformanceTrace.append(sample)
		guard performanceEnabled else { return }
		logger.info("performance stage=\(sample.stage.rawValue, privacy: .public) duration_ns=\(sample.durationNanoseconds, privacy: .public)")
	}

	public static func recordRefreshStart(id: String, recentLimit: Int) {
		logger.info("refresh_start id=\(id, privacy: .public) recent_limit=\(recentLimit, privacy: .public)")
	}

	public static func recordHelperRequest(
		id: String,
		stage: String,
		durationMilliseconds: Int64,
		stdoutBytes: Int,
		stderrBytes: Int,
		chunks: Int,
		exitStatus: Int32,
		stdoutTruncated: Bool,
		stderrTruncated: Bool,
		errorCode: String? = nil
	) {
		let result = exitStatus == 0 && !stdoutTruncated && !stderrTruncated ? "success" : "failed"
		let classifiedCode = errorCode ?? "none"
		logger.info("helper_request id=\(id, privacy: .public) stage=\(stage, privacy: .public) result=\(result, privacy: .public) duration_ms=\(durationMilliseconds, privacy: .public) stdout_bytes=\(stdoutBytes, privacy: .public) stderr_bytes=\(stderrBytes, privacy: .public) chunks=\(chunks, privacy: .public) exit_status=\(exitStatus, privacy: .public) error_code=\(classifiedCode, privacy: .public) stdout_truncated=\(stdoutTruncated, privacy: .public) stderr_truncated=\(stderrTruncated, privacy: .public)")
	}

	public static func recordHelperRequestFailure(
		id: String,
		stage: String,
		error: HelperExecutionError,
		durationMilliseconds: Int64
	) {
		let event = DesktopLogPolicy.helperFailureEvent(error)
		logger.error("helper_request id=\(id, privacy: .public) stage=\(stage, privacy: .public) result=failed duration_ms=\(durationMilliseconds, privacy: .public) error=\(event, privacy: .public)")
	}

	public static func recordIndexDomain(
		id: String,
		domain: String,
		success: Bool,
		durationMilliseconds: Int64,
		errorCode: String?
	) {
		let result = success ? "success" : "failed"
		let classifiedCode = errorCode ?? "none"
		logger.info("index_domain id=\(id, privacy: .public) domain=\(domain, privacy: .public) result=\(result, privacy: .public) duration_ms=\(durationMilliseconds, privacy: .public) error_code=\(classifiedCode, privacy: .public)")
	}

	public static func recordRefreshFinish(
		id: String,
		durationMilliseconds: Int64,
		snapshotBytes: Int,
		chunks: Int,
		partial: Bool
	) {
		logger.info("refresh_finish id=\(id, privacy: .public) result=success duration_ms=\(durationMilliseconds, privacy: .public) snapshot_bytes=\(snapshotBytes, privacy: .public) chunks=\(chunks, privacy: .public) partial=\(partial, privacy: .public)")
	}

	public static func recordRefreshFailure(
		id: String,
		durationMilliseconds: Int64,
		errorCode: String
	) {
		logger.error("refresh_finish id=\(id, privacy: .public) result=failed duration_ms=\(durationMilliseconds, privacy: .public) error=\(errorCode, privacy: .public)")
	}

    public static func recordSnapshot(_ envelope: DesktopWireEnvelopeV1) {
        logger.info("\(DesktopLogPolicy.snapshotEvent(for: envelope), privacy: .public)")
    }

    public static func recordHelperFailure(_ error: HelperExecutionError) {
        logger.error("\(DesktopLogPolicy.helperFailureEvent(error), privacy: .public)")
    }
}
