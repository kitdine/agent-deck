import Foundation
import OSLog

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
