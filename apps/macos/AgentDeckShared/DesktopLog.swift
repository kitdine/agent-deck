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

    public static func recordSnapshot(_ envelope: DesktopWireEnvelopeV1) {
        logger.info("\(DesktopLogPolicy.snapshotEvent(for: envelope), privacy: .public)")
    }

    public static func recordHelperFailure(_ error: HelperExecutionError) {
        logger.error("\(DesktopLogPolicy.helperFailureEvent(error), privacy: .public)")
    }
}
