import Darwin
import Foundation
import AgentDeckShared

@main
enum AgentDeckFoundationVerifier {
    static func main() async {
        let paths = Array(CommandLine.arguments.dropFirst())
        guard paths.count == 2 else {
            fail("expected complete and partial fixture paths")
        }

        do {
            try await verify(
                completeData: Data(contentsOf: URL(fileURLWithPath: paths[0])),
                partialData: Data(contentsOf: URL(fileURLWithPath: paths[1]))
            )
            print("verified AgentDeck macOS foundation fixtures and helper boundaries")
        } catch {
            fail("verification failed: \(error.localizedDescription)")
        }
    }

    private static func verify(completeData: Data, partialData: Data) async throws {
        let complete = try decodeDesktopWireEnvelopeV1(completeData)
        let partial = try decodeDesktopWireEnvelopeV1(partialData)
        try require(!complete.partial && complete.warnings.isEmpty, "complete fixture must remain complete")
        try require(partial.partial && !partial.warnings.isEmpty, "partial fixture must remain usable")
        try require(partial.data.health.available, "available sections survive a partial snapshot")

        let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
        let bundleURL = try makeEmbeddedHelperBundle(in: temporaryDirectory, script: "#!/bin/sh\nexit 0\n")

        let recordingProcess = VerifierProcess(
            behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: completeData))
        )
        let embeddedRunner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: recordingProcess,
            environment: ["HOME": "/tmp/agentdeck-fixture-home", "PATH": "/tmp/untrusted-path"],
            timeout: .seconds(1)
        )
        _ = try await embeddedRunner.snapshot()
        let invocations = await recordingProcess.recordedInvocations()
        try require(invocations.count == 1, "expected exactly one embedded-helper invocation")
        try require(
            invocations[0].executableURL.path == bundleURL.appendingPathComponent("Contents/Helpers/agentdeck").path,
            "host must not resolve agentdeck from PATH"
        )
        try require(
            invocations[0].arguments == ["--format", "json", "desktop", "snapshot", "--wire-version", "1", "--recent-limit", "5"],
            "helper command must use the approved argument array"
        )

        let partialRunner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: VerifierProcess(behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: partialData)))
        )
        let partialEnvelope = try await partialRunner.snapshot()
        try require(partialEnvelope.partial, "partial helper output must be returned, not discarded")

        let unsupportedData = try replacingWireVersion(in: completeData, with: 2)
        let unsupportedRunner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: VerifierProcess(behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: unsupportedData)))
        )
        try await expectDesktopWireError(.unsupportedWireVersion(2)) {
            _ = try await unsupportedRunner.snapshot()
        }

        let failureRunner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: VerifierProcess(
                behavior: .output(
                    HelperProcessOutput(
                        exitStatus: 23,
                        stdout: Data(),
                        stderr: Data("credential=never-log-this".utf8)
                    )
                )
            )
        )
        try await expectHelperError(.nonZeroExit(23)) {
            _ = try await failureRunner.snapshot()
        }

        let timeoutRunner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: VerifierProcess(behavior: .helperError(.timedOut))
        )
        try await expectHelperError(.timedOut) {
            _ = try await timeoutRunner.snapshot()
        }

        let cancellationRunner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: VerifierProcess(behavior: .cancellation)
        )
        try await expectHelperError(.cancelled) {
            _ = try await cancellationRunner.snapshot()
        }

        let limitedRunner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: VerifierProcess(
                behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: Data(), stdoutTruncated: true))
            )
        )
        try await expectHelperError(.outputLimitExceeded) {
            _ = try await limitedRunner.snapshot()
        }

        try Data("#!/bin/sh\nsleep 1\n".utf8).write(
            to: bundleURL.appendingPathComponent("Contents/Helpers/agentdeck"),
            options: .atomic
        )
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o700],
            ofItemAtPath: bundleURL.appendingPathComponent("Contents/Helpers/agentdeck").path
        )
        let realTimeoutRunner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            timeout: .milliseconds(20)
        )
        try await expectHelperError(.timedOut) {
            _ = try await realTimeoutRunner.snapshot()
        }

        let snapshotStore = AppGroupSnapshotStore(directoryURL: temporaryDirectory.appendingPathComponent("AppGroup"))
        let projection = AppGroupDesktopSnapshotV1(envelope: complete)
        try snapshotStore.write(projection)
        let restoredProjection = try snapshotStore.read()
        try require(restoredProjection == projection, "App Group store must round-trip atomically")
        let persisted = try String(contentsOf: snapshotStore.snapshotURL, encoding: .utf8)
        try require(!persisted.contains("session_id") && !persisted.contains("session-1"), "App Group data must omit session identity")
        try require(!persisted.contains("recovery_command") && !persisted.contains("credential"), "App Group data must omit sensitive fields")

        try require(
            DesktopLogPolicy.snapshotEvent(for: complete) == "desktop_snapshot_complete",
            "OSLog policy must use a fixed snapshot classification"
        )
        try require(
            DesktopLogPolicy.helperFailureEvent(.nonZeroExit(23)) == "embedded_helper_failed",
            "OSLog policy must not include helper output or exit detail"
        )
    }
}

private enum VerificationError: Error, LocalizedError {
    case failed(String)

    var errorDescription: String? {
        switch self {
        case let .failed(message):
            return message
        }
    }
}

private func require(_ condition: @autoclosure () -> Bool, _ message: String) throws {
    guard condition() else {
        throw VerificationError.failed(message)
    }
}

private func expectDesktopWireError(
    _ expected: DesktopWireError,
    operation: () async throws -> Void
) async throws {
    do {
        try await operation()
        throw VerificationError.failed("expected desktop wire error \(expected)")
    } catch let error as DesktopWireError {
        try require(error == expected, "unexpected desktop wire error")
    }
}

private func expectHelperError(
    _ expected: HelperExecutionError,
    operation: () async throws -> Void
) async throws {
    do {
        try await operation()
        throw VerificationError.failed("expected helper error \(expected)")
    } catch let error as HelperExecutionError {
        try require(error == expected, "unexpected helper error")
    }
}

private func replacingWireVersion(in data: Data, with version: Int) throws -> Data {
    guard var envelope = try JSONSerialization.jsonObject(with: data) as? [String: Any],
          var snapshot = envelope["data"] as? [String: Any]
    else {
        throw VerificationError.failed("fixture is not a desktop envelope")
    }
    snapshot["wire_version"] = version
    envelope["data"] = snapshot
    return try JSONSerialization.data(withJSONObject: envelope)
}

private func makeEmbeddedHelperBundle(in temporaryDirectory: URL, script: String) throws -> URL {
    let fileManager = FileManager.default
    let bundleURL = temporaryDirectory.appendingPathComponent("AgentDeck.app", isDirectory: true)
    let helperURL = bundleURL
        .appendingPathComponent("Contents", isDirectory: true)
        .appendingPathComponent("Helpers", isDirectory: true)
        .appendingPathComponent("agentdeck", isDirectory: false)
    try fileManager.createDirectory(at: helperURL.deletingLastPathComponent(), withIntermediateDirectories: true)
    try Data(script.utf8).write(to: helperURL, options: .atomic)
    try fileManager.setAttributes([.posixPermissions: 0o700], ofItemAtPath: helperURL.path)
    return bundleURL
}

private enum VerifierProcessBehavior: Sendable {
    case output(HelperProcessOutput)
    case helperError(HelperExecutionError)
    case cancellation
}

private struct VerifierInvocation: Sendable {
    let executableURL: URL
    let arguments: [String]
}

private actor VerifierProcess: EmbeddedHelperProcess {
    private let behavior: VerifierProcessBehavior
    private var invocations = [VerifierInvocation]()

    init(behavior: VerifierProcessBehavior) {
        self.behavior = behavior
    }

    func run(
        executableURL: URL,
        arguments: [String],
        environment _: [String: String],
        timeout _: Duration
    ) async throws -> HelperProcessOutput {
        invocations.append(VerifierInvocation(executableURL: executableURL, arguments: arguments))
        switch behavior {
        case let .output(output):
            return output
        case let .helperError(error):
            throw error
        case .cancellation:
            throw CancellationError()
        }
    }

    func recordedInvocations() -> [VerifierInvocation] {
        invocations
    }
}

private func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data("AgentDeck macOS foundation verifier: \(message)\n".utf8))
    exit(EXIT_FAILURE)
}
