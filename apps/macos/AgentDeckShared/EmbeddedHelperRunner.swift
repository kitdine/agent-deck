import CryptoKit
import Foundation
import Observation

public struct HelperProcessOutput: Equatable, Sendable {
    public let exitStatus: Int32
    public let stdout: Data
    public let stderr: Data
    public let stdoutTruncated: Bool
    public let stderrTruncated: Bool

    public init(
        exitStatus: Int32,
        stdout: Data,
        stderr: Data = Data(),
        stdoutTruncated: Bool = false,
        stderrTruncated: Bool = false
    ) {
        self.exitStatus = exitStatus
        self.stdout = stdout
        self.stderr = stderr
        self.stdoutTruncated = stdoutTruncated
        self.stderrTruncated = stderrTruncated
    }
}

public struct HelperProcessLinesOutput: Equatable, Sendable {
	public let exitStatus: Int32
	public let stdoutLines: [Data]
	public let stdoutBytes: Int
	public let stderr: Data
	public let stdoutLineTruncated: Bool
	public let stderrTruncated: Bool

	public init(
		exitStatus: Int32,
		stdoutLines: [Data],
		stdoutBytes: Int,
		stderr: Data = Data(),
		stdoutLineTruncated: Bool = false,
		stderrTruncated: Bool = false
	) {
		self.exitStatus = exitStatus
		self.stdoutLines = stdoutLines
		self.stdoutBytes = stdoutBytes
		self.stderr = stderr
		self.stdoutLineTruncated = stdoutLineTruncated
		self.stderrTruncated = stderrTruncated
	}
}

public protocol EmbeddedHelperProcess: Sendable {
    func run(
        executableURL: URL,
        arguments: [String],
        environment: [String: String],
        timeout: Duration
    ) async throws -> HelperProcessOutput

	func runLines(
		executableURL: URL,
		arguments: [String],
		environment: [String: String],
		timeout: Duration,
		maximumLineBytes: Int,
		maximumLines: Int
	) async throws -> HelperProcessLinesOutput

	func runLines(
		executableURL: URL,
		arguments: [String],
		environment: [String: String],
		timeout: Duration,
		maximumLineBytes: Int,
		maximumLines: Int,
		onLine: @escaping @Sendable (Data) -> Void
	) async throws -> HelperProcessLinesOutput
}

public extension EmbeddedHelperProcess {
	func runLines(
		executableURL: URL,
		arguments: [String],
		environment: [String: String],
		timeout: Duration,
		maximumLineBytes: Int,
		maximumLines: Int,
		onLine: @escaping @Sendable (Data) -> Void
	) async throws -> HelperProcessLinesOutput {
		let output = try await runLines(
			executableURL: executableURL,
			arguments: arguments,
			environment: environment,
			timeout: timeout,
			maximumLineBytes: maximumLineBytes,
			maximumLines: maximumLines
		)
		for line in output.stdoutLines {
			onLine(line)
		}
		return output
	}

	func runLines(
		executableURL: URL,
		arguments: [String],
		environment: [String: String],
		timeout: Duration,
		maximumLineBytes _: Int,
		maximumLines _: Int
	) async throws -> HelperProcessLinesOutput {
		let output = try await run(
			executableURL: executableURL,
			arguments: arguments,
			environment: environment,
			timeout: timeout
		)
		return HelperProcessLinesOutput(
			exitStatus: output.exitStatus,
			stdoutLines: output.stdout.isEmpty ? [] : [output.stdout],
			stdoutBytes: output.stdout.count,
			stderr: output.stderr,
			stdoutLineTruncated: output.stdoutTruncated,
			stderrTruncated: output.stderrTruncated
		)
	}
}

public enum HelperExecutionError: Error, Equatable, Sendable, LocalizedError {
    case missingEmbeddedHelper
    case invalidRecentLimit(Int)
    case launchFailed
    case timedOut
    case cancelled
    case nonZeroExit(Int32)
    case outputLimitExceeded
    case malformedOutput

    public var errorDescription: String? {
        switch self {
        case .missingEmbeddedHelper:
            return "The embedded AgentDeck helper is unavailable."
        case .invalidRecentLimit:
            return "The desktop recent-session limit is invalid."
        case .launchFailed:
            return "The embedded AgentDeck helper could not start."
        case .timedOut:
            return "The embedded AgentDeck helper timed out."
        case .cancelled:
            return "The desktop refresh was cancelled."
        case .nonZeroExit:
            return "The embedded AgentDeck helper failed."
        case .outputLimitExceeded:
            return "The embedded AgentDeck helper produced too much output."
        case .malformedOutput:
            return "The embedded AgentDeck helper returned invalid output."
        }
    }
}

public enum DesktopScanStage: String, Codable, Equatable, Sendable {
	case waiting
	case checking
	case importing
	case statistics
	case completed
}

public struct DesktopScanDomainProgress: Codable, Equatable, Sendable {
	public let state: String
	public let committed: Int
	/// Absent until discovery/planning establishes the domain's inventory
	/// size — never a guessed zero.
	public let total: Int?
	public let skipped: Int
}

public struct DesktopScanProgress: Codable, Equatable, Sendable {
	public let sequence: UInt64
	public let stage: DesktopScanStage
	public let usage: DesktopScanDomainProgress
	public let session: DesktopScanDomainProgress

	public static let waiting = DesktopScanProgress(
		sequence: 0,
		stage: .waiting,
		usage: DesktopScanDomainProgress(state: "pending", committed: 0, total: nil, skipped: 0),
		session: DesktopScanDomainProgress(state: "pending", committed: 0, total: nil, skipped: 0)
	)
}

public final class FoundationEmbeddedHelperProcess: EmbeddedHelperProcess, @unchecked Sendable {
	/// The compact desktop snapshot is held below 128 KiB by contract tests. The
	/// process cap keeps one additional bounded margin for environment-dependent
	/// provider candidates and warnings without accepting unbounded output.
    public static let maximumCapturedBytes = 512 * 1024

    public init() {}

    public func run(
        executableURL: URL,
        arguments: [String],
        environment: [String: String],
        timeout: Duration
    ) async throws -> HelperProcessOutput {
        if Task.isCancelled {
            throw HelperExecutionError.cancelled
        }

        let process = Process()
        let stdoutPipe = Pipe()
        let stderrPipe = Pipe()
        let stdout = BoundedData(maximumBytes: Self.maximumCapturedBytes)
        let stderr = BoundedData(maximumBytes: Self.maximumCapturedBytes)

        process.executableURL = executableURL
        process.arguments = arguments
        process.environment = environment
        process.standardInput = FileHandle.nullDevice
        process.standardOutput = stdoutPipe
        process.standardError = stderrPipe

        let stdoutBarrier = startCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
        let stderrBarrier = startCapture(from: stderrPipe.fileHandleForReading, into: stderr)

        let running = RunningProcess(process)
        do {
            try process.run()
        } catch {
            try? stdoutPipe.fileHandleForWriting.close()
            try? stderrPipe.fileHandleForWriting.close()
            finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout, barrier: stdoutBarrier)
            finishCapture(from: stderrPipe.fileHandleForReading, into: stderr, barrier: stderrBarrier)
            throw HelperExecutionError.launchFailed
        }
        try? stdoutPipe.fileHandleForWriting.close()
        try? stderrPipe.fileHandleForWriting.close()

        do {
            let exitStatus = try await withTaskCancellationHandler(
                operation: { try await waitForExit(running, timeout: timeout) },
                onCancel: { running.terminate() }
            )
            if Task.isCancelled {
                throw HelperExecutionError.cancelled
            }
            finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout, barrier: stdoutBarrier)
            finishCapture(from: stderrPipe.fileHandleForReading, into: stderr, barrier: stderrBarrier)
            return HelperProcessOutput(
                exitStatus: exitStatus,
                stdout: stdout.value,
                stderr: stderr.value,
                stdoutTruncated: stdout.wasTruncated,
                stderrTruncated: stderr.wasTruncated
            )
        } catch is CancellationError {
            running.terminate()
            finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout, barrier: stdoutBarrier)
            finishCapture(from: stderrPipe.fileHandleForReading, into: stderr, barrier: stderrBarrier)
            throw HelperExecutionError.cancelled
        } catch {
            running.terminate()
            finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout, barrier: stdoutBarrier)
            finishCapture(from: stderrPipe.fileHandleForReading, into: stderr, barrier: stderrBarrier)
            if let helperError = error as? HelperExecutionError {
                throw helperError
            }
            throw HelperExecutionError.launchFailed
        }
    }

	public func runLines(
		executableURL: URL,
		arguments: [String],
		environment: [String: String],
		timeout: Duration,
		maximumLineBytes: Int,
		maximumLines: Int
	) async throws -> HelperProcessLinesOutput {
		try await runLines(
			executableURL: executableURL,
			arguments: arguments,
			environment: environment,
			timeout: timeout,
			maximumLineBytes: maximumLineBytes,
			maximumLines: maximumLines,
			onLine: { _ in }
		)
	}

	public func runLines(
		executableURL: URL,
		arguments: [String],
		environment: [String: String],
		timeout: Duration,
		maximumLineBytes: Int,
		maximumLines: Int,
		onLine: @escaping @Sendable (Data) -> Void
	) async throws -> HelperProcessLinesOutput {
		if Task.isCancelled {
			throw HelperExecutionError.cancelled
		}

		let process = Process()
		let stdoutPipe = Pipe()
		let stderrPipe = Pipe()
		let stdout = BoundedLines(maximumLineBytes: maximumLineBytes, maximumLines: maximumLines, onLine: onLine)
		let stderr = BoundedData(maximumBytes: Self.maximumCapturedBytes)

		process.executableURL = executableURL
		process.arguments = arguments
		process.environment = environment
		process.standardInput = FileHandle.nullDevice
		process.standardOutput = stdoutPipe
		process.standardError = stderrPipe

		let stdoutBarrier = startCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
		let stderrBarrier = startCapture(from: stderrPipe.fileHandleForReading, into: stderr)

		let running = RunningProcess(process)
		do {
			try process.run()
		} catch {
			try? stdoutPipe.fileHandleForWriting.close()
			try? stderrPipe.fileHandleForWriting.close()
			finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout, barrier: stdoutBarrier)
			finishCapture(from: stderrPipe.fileHandleForReading, into: stderr, barrier: stderrBarrier)
			throw HelperExecutionError.launchFailed
		}
		try? stdoutPipe.fileHandleForWriting.close()
		try? stderrPipe.fileHandleForWriting.close()

		do {
			let exitStatus = try await withTaskCancellationHandler(
				operation: { try await waitForExit(running, timeout: timeout) },
				onCancel: { running.terminate() }
			)
			if Task.isCancelled {
				throw HelperExecutionError.cancelled
			}
			finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout, barrier: stdoutBarrier)
			finishCapture(from: stderrPipe.fileHandleForReading, into: stderr, barrier: stderrBarrier)
			return HelperProcessLinesOutput(
				exitStatus: exitStatus,
				stdoutLines: stdout.lines,
				stdoutBytes: stdout.totalBytes,
				stderr: stderr.value,
				stdoutLineTruncated: stdout.wasTruncated,
				stderrTruncated: stderr.wasTruncated
			)
		} catch is CancellationError {
			running.terminate()
			finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout, barrier: stdoutBarrier)
			finishCapture(from: stderrPipe.fileHandleForReading, into: stderr, barrier: stderrBarrier)
			throw HelperExecutionError.cancelled
		} catch {
			running.terminate()
			finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout, barrier: stdoutBarrier)
			finishCapture(from: stderrPipe.fileHandleForReading, into: stderr, barrier: stderrBarrier)
			if let helperError = error as? HelperExecutionError {
				throw helperError
			}
			throw HelperExecutionError.launchFailed
		}
	}

    private func startCapture(from handle: FileHandle, into capture: BoundedData) -> CaptureBarrier {
        let barrier = CaptureBarrier()
        handle.readabilityHandler = { readableHandle in
            barrier.withCallback {
                let chunk = readableHandle.availableData
                if chunk.isEmpty {
                    readableHandle.readabilityHandler = nil
                    return
                }
                capture.append(chunk)
            }
        }
        return barrier
    }

    private func finishCapture(from handle: FileHandle, into capture: BoundedData, barrier: CaptureBarrier) {
        barrier.stopAccepting()
        handle.readabilityHandler = nil
        barrier.waitUntilIdle()
        capture.append(handle.readDataToEndOfFile())
        try? handle.close()
    }

	private func startCapture(from handle: FileHandle, into capture: BoundedLines) -> CaptureBarrier {
		let barrier = CaptureBarrier()
		handle.readabilityHandler = { readableHandle in
			barrier.withCallback {
				let chunk = readableHandle.availableData
				if chunk.isEmpty {
					readableHandle.readabilityHandler = nil
					return
				}
				capture.append(chunk)
			}
		}
		return barrier
	}

	private func finishCapture(from handle: FileHandle, into capture: BoundedLines, barrier: CaptureBarrier) {
		barrier.stopAccepting()
		handle.readabilityHandler = nil
		barrier.waitUntilIdle()
		capture.append(handle.readDataToEndOfFile())
		capture.finish()
		try? handle.close()
	}
}

private final class CaptureBarrier: @unchecked Sendable {
	private let condition = NSCondition()
	private var accepting = true
	private var activeCallbacks = 0

	func withCallback(_ body: () -> Void) {
		condition.lock()
		guard accepting else {
			condition.unlock()
			return
		}
		activeCallbacks += 1
		condition.unlock()

		body()

		condition.lock()
		activeCallbacks -= 1
		if activeCallbacks == 0 {
			condition.broadcast()
		}
		condition.unlock()
	}

	func stopAccepting() {
		condition.lock()
		accepting = false
		condition.unlock()
	}

	func waitUntilIdle() {
		condition.lock()
		while activeCallbacks > 0 {
			condition.wait()
		}
		condition.unlock()
	}
}

public struct EmbeddedHelperRunner: Sendable {
    public static let defaultRecentLimit = 5
	public static let minimumRecentLimit = 1
	public static let maximumRecentLimit = 20
	public static let defaultTimeout: Duration = .seconds(30)
	public static let quotaRefreshTimeout: Duration = .seconds(40)
	public static let indexRefreshTimeout: Duration = .seconds(120)
	public static let maximumStreamLineBytes = 96 * 1024
	public static let maximumStreamLines = 2_048
	public static let maximumSnapshotBytes = 64 * 1024 * 1024

    private let appBundleURL: URL
    private let process: any EmbeddedHelperProcess
    private let environment: [String: String]
    private let stateRoot: String
    private let timeout: Duration

    public init(
        appBundleURL: URL,
        process: any EmbeddedHelperProcess = FoundationEmbeddedHelperProcess(),
        environment: [String: String]? = nil,
        timeout: Duration = Self.defaultTimeout
    ) {
        var resolvedEnvironment = environment ?? Self.defaultEnvironment()
        if resolvedEnvironment["HOME"] == nil {
            resolvedEnvironment["HOME"] = Self.defaultEnvironment()["HOME"]
        }
        self.appBundleURL = appBundleURL
        self.process = process
        self.environment = resolvedEnvironment
        stateRoot = URL(fileURLWithPath: resolvedEnvironment["HOME"]!, isDirectory: true)
            .appendingPathComponent(".agentdeck", isDirectory: true)
            .standardizedFileURL.path
        self.timeout = timeout
    }

    public init(
        bundle: Bundle = .main,
        process: any EmbeddedHelperProcess = FoundationEmbeddedHelperProcess(),
        timeout: Duration = Self.defaultTimeout
    ) {
        self.init(appBundleURL: bundle.bundleURL, process: process, timeout: timeout)
    }

	public func snapshot(
		recentLimit: Int = Self.defaultRecentLimit,
		progress: @escaping @Sendable (DesktopScanProgress) -> Void = { _ in }
	) async throws -> DesktopWireEnvelopeV1 {
        guard (Self.minimumRecentLimit ... Self.maximumRecentLimit).contains(recentLimit) else {
            throw HelperExecutionError.invalidRecentLimit(recentLimit)
        }
		if Task.isCancelled {
			throw HelperExecutionError.cancelled
		}

		let refreshID = UUID().uuidString.lowercased()
		let refreshStartedAt = Date()
		DesktopLogger.recordRefreshStart(id: refreshID, recentLimit: recentLimit)
		let executableURL: URL
		do {
			executableURL = try embeddedHelperURL()
		} catch {
			DesktopLogger.recordRefreshFailure(
				id: refreshID,
				durationMilliseconds: elapsedMilliseconds(since: refreshStartedAt),
				errorCode: DesktopLogPolicy.helperFailureEvent(.missingEmbeddedHelper)
			)
			throw error
		}
		try await scanIndexes(executableURL: executableURL, refreshID: refreshID, progress: progress)
		let requestStartedAt = Date()
		let output: HelperProcessLinesOutput
        do {
			output = try await process.runLines(
                executableURL: executableURL,
				arguments: helperArguments([
                    "--format", "json",
                    "desktop", "snapshot",
                    "--wire-version", "1",
                    "--recent-limit", String(recentLimit),
					"--stream",
				]),
                environment: environment,
				timeout: timeout,
				maximumLineBytes: Self.maximumStreamLineBytes,
				maximumLines: Self.maximumStreamLines
            )
        } catch is CancellationError {
			DesktopLogger.recordHelperRequestFailure(id: refreshID, stage: "snapshot_stream", error: .cancelled, durationMilliseconds: elapsedMilliseconds(since: requestStartedAt))
            throw HelperExecutionError.cancelled
        } catch let error as HelperExecutionError {
			DesktopLogger.recordHelperRequestFailure(id: refreshID, stage: "snapshot_stream", error: error, durationMilliseconds: elapsedMilliseconds(since: requestStartedAt))
            throw error
        } catch {
			DesktopLogger.recordHelperRequestFailure(id: refreshID, stage: "snapshot_stream", error: .launchFailed, durationMilliseconds: elapsedMilliseconds(since: requestStartedAt))
            throw HelperExecutionError.launchFailed
        }

        if Task.isCancelled {
            throw HelperExecutionError.cancelled
        }
		DesktopLogger.recordHelperRequest(
			id: refreshID,
			stage: "snapshot_stream",
			durationMilliseconds: elapsedMilliseconds(since: requestStartedAt),
			stdoutBytes: output.stdoutBytes,
			stderrBytes: output.stderr.count,
			chunks: output.stdoutLines.count,
			exitStatus: output.exitStatus,
			stdoutTruncated: output.stdoutLineTruncated,
			stderrTruncated: output.stderrTruncated,
			errorCode: desktopHelperErrorCode(stderr: output.stderr)
		)
		guard !output.stdoutLineTruncated, !output.stderrTruncated else {
			DesktopLogger.recordRefreshFailure(
				id: refreshID,
				durationMilliseconds: elapsedMilliseconds(since: refreshStartedAt),
				errorCode: DesktopLogPolicy.helperFailureEvent(.outputLimitExceeded)
			)
            throw HelperExecutionError.outputLimitExceeded
        }
        guard output.exitStatus == 0 else {
			DesktopLogger.recordRefreshFailure(
				id: refreshID,
				durationMilliseconds: elapsedMilliseconds(since: refreshStartedAt),
				errorCode: DesktopLogPolicy.helperFailureEvent(.nonZeroExit(output.exitStatus))
			)
            throw HelperExecutionError.nonZeroExit(output.exitStatus)
        }

		do {
			let payload = try decodeDesktopSnapshotStream(output.stdoutLines)
			let envelope = try decodeDesktopWireEnvelopeV1(payload)
			DesktopLogger.recordRefreshFinish(
				id: refreshID,
				durationMilliseconds: elapsedMilliseconds(since: refreshStartedAt),
				snapshotBytes: payload.count,
				chunks: output.stdoutLines.count,
				partial: envelope.partial
			)
			return envelope
        } catch let error as DesktopWireError {
			DesktopLogger.recordRefreshFailure(
				id: refreshID,
				durationMilliseconds: elapsedMilliseconds(since: refreshStartedAt),
				errorCode: "invalid_wire"
			)
            throw error
		} catch let error as HelperExecutionError {
			DesktopLogger.recordRefreshFailure(
				id: refreshID,
				durationMilliseconds: elapsedMilliseconds(since: refreshStartedAt),
				errorCode: DesktopLogPolicy.helperFailureEvent(error)
			)
			throw error
        } catch {
			DesktopLogger.recordRefreshFailure(
				id: refreshID,
				durationMilliseconds: elapsedMilliseconds(since: refreshStartedAt),
				errorCode: DesktopLogPolicy.helperFailureEvent(.malformedOutput)
			)
            throw HelperExecutionError.malformedOutput
		}
	}

	private func scanIndexes(
		executableURL: URL,
		refreshID: String,
		progress: @escaping @Sendable (DesktopScanProgress) -> Void
	) async throws {
		let stage = "global_scan_stream"
		let startedAt = Date()
		do {
			let output = try await process.runLines(
				executableURL: executableURL,
				arguments: helperArguments(["--format", "ndjson", "scan"]),
				environment: environment,
				timeout: Self.indexRefreshTimeout,
				maximumLineBytes: Self.maximumStreamLineBytes,
				maximumLines: Self.maximumStreamLines,
				onLine: { line in
					if let value = try? decodeDesktopScanProgress(line) {
						progress(value)
					}
				}
			)
			DesktopLogger.recordHelperRequest(
				id: refreshID,
				stage: stage,
				durationMilliseconds: elapsedMilliseconds(since: startedAt),
				stdoutBytes: output.stdoutBytes,
				stderrBytes: output.stderr.count,
				chunks: output.stdoutLines.count,
				exitStatus: output.exitStatus,
				stdoutTruncated: output.stdoutLineTruncated,
				stderrTruncated: output.stderrTruncated,
				errorCode: desktopHelperErrorCode(stderr: output.stderr)
			)
			guard !output.stdoutLineTruncated, !output.stderrTruncated else {
				throw HelperExecutionError.outputLimitExceeded
			}
			guard output.exitStatus == 0 else {
				throw HelperExecutionError.nonZeroExit(output.exitStatus)
			}
			_ = try decodeDesktopScanEventStream(output.stdoutLines)
			if Task.isCancelled {
				throw HelperExecutionError.cancelled
			}
		} catch is CancellationError {
			throw HelperExecutionError.cancelled
		} catch let error as HelperExecutionError where error == .cancelled {
			throw error
		} catch let error as HelperExecutionError {
			DesktopLogger.recordHelperRequestFailure(
				id: refreshID,
				stage: stage,
				error: error,
				durationMilliseconds: elapsedMilliseconds(since: startedAt)
			)
			throw error
		} catch {
			DesktopLogger.recordHelperRequestFailure(id: refreshID, stage: stage, error: .malformedOutput, durationMilliseconds: elapsedMilliseconds(since: startedAt))
			throw HelperExecutionError.malformedOutput
		}
	}

	public func embeddedHelperURL() throws -> URL {
        let candidate = appBundleURL
            .appendingPathComponent("Contents", isDirectory: true)
            .appendingPathComponent("Helpers", isDirectory: true)
            .appendingPathComponent("agentdeck", isDirectory: false)
        guard FileManager.default.isExecutableFile(atPath: candidate.path) else {
            throw HelperExecutionError.missingEmbeddedHelper
        }
        return candidate
    }

	public func switchProvider(_ target: ProviderSwitchTarget) async -> ProviderSwitchTransportOutcome {
		let executableURL: URL
		do {
			executableURL = try embeddedHelperURL()
		} catch {
			return .opaque
		}
		var arguments = [
			"--quiet", "--format", "json",
			"provider", "use", target.provider,
			"--client", target.client,
		]
		if let credential = target.credential {
			arguments.append(contentsOf: ["--credential", credential])
		}
		if target.viaWrapper {
			arguments.append("--via")
		}
		arguments.append("--no-shell-setup")

		do {
			let output = try await process.run(
				executableURL: executableURL,
				arguments: helperArguments(arguments),
				environment: environment,
				timeout: timeout
			)
			guard !output.stdoutTruncated, !output.stderrTruncated else {
				return .opaque
			}
			return classifyProviderUseOutput(output)
		} catch let error as HelperExecutionError where error == .timedOut {
			return .indeterminate
		} catch {
			return .opaque
		}
	}

	private func helperArguments(_ arguments: [String]) -> [String] {
		["--state-dir", stateRoot] + arguments
	}

    private static func defaultEnvironment() -> [String: String] {
        var environment = [
            "HOME": FileManager.default.homeDirectoryForCurrentUser.path,
            "LANG": "en_US_POSIX",
            "LC_ALL": "en_US_POSIX",
			// App-launched helpers do not inherit an interactive shell PATH.
			// Keep the search path fixed to supported, trusted installation
			// roots so quota probes can resolve Homebrew-installed codex/claude
			// without admitting arbitrary user-writable directories.
			"PATH": "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin",
        ]
		if let temporaryDirectory = ProcessInfo.processInfo.environment["TMPDIR"], !temporaryDirectory.isEmpty {
			environment["TMPDIR"] = temporaryDirectory
		}
		return environment
    }
}

private struct DesktopSnapshotChunkEnvelopeV1: Decodable {
	let schemaVersion: Int
	let command: String
	let data: DesktopSnapshotChunkV1
	let warnings: [String]
	let partial: Bool

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case command, data, warnings, partial
	}
}

private struct DesktopSnapshotChunkV1: Decodable {
	let index: Int
	let count: Int
	let totalBytes: Int
	let sha256: String
	let payload: String

	enum CodingKeys: String, CodingKey {
		case index, count
		case totalBytes = "total_bytes"
		case sha256, payload
	}
}

private struct DesktopSnapshotCommandProbe: Decodable {
	let command: String
}

private struct DesktopHelperErrorProbe: Decodable {
	let error: DesktopHelperErrorCode?
}

private struct DesktopHelperErrorCode: Decodable {
	let code: String
}

private struct DesktopIndexRefreshEnvelope: Decodable {
	let schemaVersion: Int
	let command: String
	let data: DesktopIndexRefreshResult
	let error: DesktopWireOutputErrorV1?

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case command, data, error
	}
}

struct DesktopIndexRefreshResult: Decodable, Equatable, Sendable {
	let usage: DesktopIndexDomainResult
	let sessions: DesktopIndexDomainResult
}

struct DesktopIndexDomainResult: Decodable, Equatable, Sendable {
	let success: Bool
	let durationMilliseconds: Int64
	let errorCode: String?

	enum CodingKeys: String, CodingKey {
		case success
		case durationMilliseconds = "duration_ms"
		case errorCode = "error_code"
	}
}

func decodeDesktopIndexRefreshResult(_ data: Data) throws -> DesktopIndexRefreshResult {
	let envelope = try JSONDecoder().decode(DesktopIndexRefreshEnvelope.self, from: data)
	guard envelope.schemaVersion == DesktopWireEnvelopeV1.schemaVersion,
		envelope.command == "desktop.refresh-indexes",
		envelope.error == nil
	else {
		throw HelperExecutionError.malformedOutput
	}
	return envelope.data
}

private struct DesktopScanEventProbe: Decodable {
	let schemaVersion: Int
	let command: String
	let type: String
	let scope: String
	let partial: Bool

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case command, type, scope, partial
	}
}

private struct DesktopScanProgressEnvelope: Decodable {
	let schemaVersion: Int
	let command: String
	let type: String
	let scope: String
	let data: DesktopScanProgress

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case command, type, scope, data
	}
}

private struct DesktopScanResultEnvelope: Decodable {
	struct DataValue: Decodable {
		struct Domain: Decodable { let state: String }
		let scope: String
		let usage: Domain
		let session: Domain
	}

	let schemaVersion: Int
	let command: String
	let type: String
	let scope: String
	let data: DataValue
	let partial: Bool

	enum CodingKeys: String, CodingKey {
		case schemaVersion = "schema_version"
		case command, type, scope, data, partial
	}
}

func decodeDesktopScanProgress(_ line: Data) throws -> DesktopScanProgress {
	let envelope = try JSONDecoder().decode(DesktopScanProgressEnvelope.self, from: line)
	guard envelope.schemaVersion == DesktopWireEnvelopeV1.schemaVersion,
		envelope.command == "scan",
		envelope.type == "progress",
		envelope.scope == "both"
	else {
		throw HelperExecutionError.malformedOutput
	}
	return envelope.data
}

@discardableResult
func decodeDesktopScanEventStream(_ lines: [Data]) throws -> [DesktopScanProgress] {
	do {
		return try decodeDesktopScanEventStreamUnchecked(lines)
	} catch let error as HelperExecutionError {
		throw error
	} catch {
		throw HelperExecutionError.malformedOutput
	}
}

private func decodeDesktopScanEventStreamUnchecked(_ lines: [Data]) throws -> [DesktopScanProgress] {
	guard !lines.isEmpty else {
		throw HelperExecutionError.malformedOutput
	}
	var progress = [DesktopScanProgress]()
	var lastSequence: UInt64?
	var resultCount = 0
	for (index, line) in lines.enumerated() {
		let probe = try JSONDecoder().decode(DesktopScanEventProbe.self, from: line)
		guard probe.schemaVersion == DesktopWireEnvelopeV1.schemaVersion,
			probe.command == "scan",
			probe.scope == "both"
		else {
			throw HelperExecutionError.malformedOutput
		}
		switch probe.type {
		case "progress":
			guard resultCount == 0 else {
				throw HelperExecutionError.malformedOutput
			}
			let value = try decodeDesktopScanProgress(line)
			if let lastSequence, value.sequence <= lastSequence {
				throw HelperExecutionError.malformedOutput
			}
			lastSequence = value.sequence
			progress.append(value)
		case "result":
			let result = try JSONDecoder().decode(DesktopScanResultEnvelope.self, from: line)
			guard index == lines.count - 1,
				!probe.partial,
				result.schemaVersion == DesktopWireEnvelopeV1.schemaVersion,
				result.command == "scan",
				result.type == "result",
				result.scope == "both",
				result.data.scope == "both",
				result.data.usage.state == "completed",
				result.data.session.state == "completed"
			else {
				throw HelperExecutionError.malformedOutput
			}
			resultCount += 1
		default:
			throw HelperExecutionError.malformedOutput
		}
	}
	guard resultCount == 1, progress.last?.stage == .completed else {
		throw HelperExecutionError.malformedOutput
	}
	return progress
}

func desktopHelperErrorCode(stderr: Data) -> String? {
	guard !stderr.isEmpty,
		let code = try? JSONDecoder().decode(DesktopHelperErrorProbe.self, from: stderr).error?.code,
		(1 ... 64).contains(code.utf8.count),
		code.utf8.allSatisfy({ byte in
			(byte >= 0x61 && byte <= 0x7A) || (byte >= 0x30 && byte <= 0x39) || byte == 0x5F
		})
	else {
		return nil
	}
	return code
}

func decodeDesktopSnapshotStream(_ lines: [Data]) throws -> Data {
	guard !lines.isEmpty else {
		throw HelperExecutionError.malformedOutput
	}
	if lines.count == 1,
		let probe = try? JSONDecoder().decode(DesktopSnapshotCommandProbe.self, from: lines[0]),
		probe.command == DesktopWireEnvelopeV1.command
	{
		return lines[0]
	}

	var rebuilt = Data()
	var expectedCount: Int?
	var expectedBytes: Int?
	var expectedDigest: String?
	for (expectedIndex, line) in lines.enumerated() {
		let frame = try JSONDecoder().decode(DesktopSnapshotChunkEnvelopeV1.self, from: line)
		guard frame.schemaVersion == DesktopWireEnvelopeV1.schemaVersion,
			frame.command == "desktop.snapshot.chunk",
			frame.warnings.isEmpty,
			!frame.partial,
			frame.data.index == expectedIndex,
			frame.data.count == lines.count,
			frame.data.count > 0,
			frame.data.totalBytes >= 0,
			frame.data.totalBytes <= EmbeddedHelperRunner.maximumSnapshotBytes,
			let chunk = Data(base64Encoded: frame.data.payload)
		else {
			throw HelperExecutionError.malformedOutput
		}
		if let expectedCount {
			guard expectedCount == frame.data.count,
				expectedBytes == frame.data.totalBytes,
				expectedDigest == frame.data.sha256
			else {
				throw HelperExecutionError.malformedOutput
			}
		} else {
			expectedCount = frame.data.count
			expectedBytes = frame.data.totalBytes
			expectedDigest = frame.data.sha256
		}
		rebuilt.append(chunk)
		guard rebuilt.count <= EmbeddedHelperRunner.maximumSnapshotBytes else {
			throw HelperExecutionError.outputLimitExceeded
		}
	}
	guard expectedCount == lines.count,
		expectedBytes == rebuilt.count,
		let expectedDigest,
		SHA256.hash(data: rebuilt).hexString == expectedDigest
	else {
		throw HelperExecutionError.malformedOutput
	}
	return rebuilt
}

private func elapsedMilliseconds(since start: Date) -> Int64 {
	Int64(max(0, Date().timeIntervalSince(start) * 1_000).rounded())
}

private extension Digest {
	var hexString: String {
		map { String(format: "%02x", $0) }.joined()
	}
}

@MainActor
public final class DesktopHost {
    private let runner: EmbeddedHelperRunner

    public init(runner: EmbeddedHelperRunner = EmbeddedHelperRunner()) {
        self.runner = runner
    }

	public func refresh(recentLimit: Int = EmbeddedHelperRunner.defaultRecentLimit) async throws -> DesktopWireEnvelopeV1 {
		try await refresh(recentLimit: recentLimit, progress: { _ in })
	}

	public func refresh(
		recentLimit: Int,
		progress: @escaping @Sendable (DesktopScanProgress) -> Void
	) async throws -> DesktopWireEnvelopeV1 {
		do {
			let envelope = try await runner.snapshot(recentLimit: recentLimit, progress: progress)
            DesktopLogger.recordSnapshot(envelope)
            return envelope
        } catch let error as HelperExecutionError {
            DesktopLogger.recordHelperFailure(error)
            throw error
        }
	}
}

public struct ProviderSwitchTarget: Equatable, Sendable, Identifiable {
	public var id: String {
		[client, provider, credential ?? "", viaWrapper ? "via" : "direct"].joined(separator: "\u{0}")
	}
	public let client: String
	public let provider: String
	public let credential: String?
	public let viaWrapper: Bool

	public init(client: String, provider: String, credential: String?, viaWrapper: Bool) {
		self.client = client
		self.provider = provider
		self.credential = credential
		self.viaWrapper = viaWrapper
	}

	public init(_ option: DesktopProviderSwitchOptionV1) {
		self.init(client: option.client, provider: option.provider, credential: option.credential, viaWrapper: option.viaWrapper)
	}
}

public enum ProviderSwitchTransportOutcome: Equatable, Sendable {
	case succeeded
	case failed(code: String)
	case indeterminate
	case opaque
}

public protocol ProviderSwitching: Sendable {
	func switchProvider(_ target: ProviderSwitchTarget) async -> ProviderSwitchTransportOutcome
}

extension EmbeddedHelperRunner: ProviderSwitching {}

private func classifyProviderUseOutput(_ output: HelperProcessOutput) -> ProviderSwitchTransportOutcome {
	let stdoutEmpty = output.stdout.isEmpty
	let stderrEmpty = output.stderr.isEmpty
	let stdoutEnvelope = stdoutEmpty ? nil : try? decodeProviderUseEnvelopeV1(output.stdout)
	let stderrEnvelope = stderrEmpty ? nil : try? decodeProviderUseEnvelopeV1(output.stderr)

	if output.exitStatus == 0,
		stderrEmpty,
		let stdoutEnvelope,
		stdoutEnvelope.errorCode == nil
	{
		return .succeeded
	}
	if output.exitStatus != 0,
		stdoutEmpty,
		let stderrEnvelope,
		let code = stderrEnvelope.errorCode
	{
		return .failed(code: code)
	}
	if stdoutEnvelope != nil || stderrEnvelope != nil {
		return .indeterminate
	}
	return .opaque
}

// MARK: - Quota settings transport (subscription-quota task 7)
//
// `desktop quota-settings` and `desktop quota-statusline` always write their
// full result to stdout before returning a non-zero exit on a partial
// failure (task 6's `runDesktopQuotaSettings`/`runDesktopQuotaStatusLine`):
// the settings view or the statusline outcome, including a `failed` or
// `restore_incomplete` sub-result, is data on stdout, not something read off
// the exit code or a stderr error envelope. So unlike `switchProvider`'s
// `classifyProviderUseOutput`, these two only need to decode stdout — an
// exit status is not part of their presentation contract.

/// C9's three background cadences, spelled the way `time.Duration.String()`
/// renders them (`5m0s`) — the wire form `QuotaProbeInterval`'s own raw value
/// does not share, since it decodes an integer number of minutes, not text.
public enum DesktopQuotaIntervalV1: String, Codable, Equatable, Sendable {
	case fiveMinutes = "5m0s"
	case fifteenMinutes = "15m0s"
	case thirtyMinutes = "30m0s"

	public var flagValue: String {
		switch self {
		case .fiveMinutes: "5m"
		case .fifteenMinutes: "15m"
		case .thirtyMinutes: "30m"
		}
	}
}

public struct DesktopQuotaSettingsValuesV1: Codable, Equatable, Sendable {
	public let reading: Bool
	public let interval: DesktopQuotaIntervalV1
	public let alerts: Bool
	public let thresholds: [Double]
	public let resetNotice: Bool
	public let statusline: Bool

	enum CodingKeys: String, CodingKey {
		case reading, alerts, thresholds, statusline, interval
		case resetNotice = "reset_notice"
	}

	public init(reading: Bool, interval: DesktopQuotaIntervalV1, alerts: Bool, thresholds: [Double], resetNotice: Bool, statusline: Bool) {
		self.reading = reading
		self.interval = interval
		self.alerts = alerts
		self.thresholds = thresholds
		self.resetNotice = resetNotice
		self.statusline = statusline
	}
}

/// `usagehook.Outcome`'s closed set (internal/usagehook/config.go). An
/// unrecognized raw value decodes as `.unknown` rather than failing the whole
/// settings read — a forward-compatible outcome must still let the rest of
/// the result render.
public enum DesktopUsageHookOutcomeV1: String, Codable, Equatable, Sendable {
	case configured
	case unchanged
	case removed
	case absent
	case skipped
	case failed
	case restoreIncomplete = "restore_incomplete"
	case unknown

	public init(from decoder: Decoder) throws {
		let raw = try decoder.singleValueContainer().decode(String.self)
		self = DesktopUsageHookOutcomeV1(rawValue: raw) ?? .unknown
	}
}

/// The status-line write/restore result the settings-quota UX names: only
/// `outcome` and `error` reach presentation, so the client/path/configuration
/// state fields `usagehook.Result` also carries are not decoded here.
public struct DesktopUsageHookResultV1: Codable, Equatable, Sendable {
	public let outcome: DesktopUsageHookOutcomeV1
	public let error: String?

	enum CodingKeys: String, CodingKey {
		case outcome, error
	}

	public init(outcome: DesktopUsageHookOutcomeV1, error: String? = nil) {
		self.outcome = outcome
		self.error = error
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		outcome = try container.decodeIfPresent(DesktopUsageHookOutcomeV1.self, forKey: .outcome) ?? .unknown
		error = try container.decodeIfPresent(String.self, forKey: .error)
	}
}

public struct DesktopQuotaSettingsResultV1: Codable, Equatable, Sendable {
	public let settings: DesktopQuotaSettingsValuesV1
	public let statuslineRestore: DesktopUsageHookResultV1?

	enum CodingKeys: String, CodingKey {
		case settings
		case statuslineRestore = "statusline_restore"
	}
}

public struct DesktopQuotaStatusLineResultV1: Codable, Equatable, Sendable {
	public let consent: Bool
	public let result: DesktopUsageHookResultV1
}

private struct DesktopQuotaEnvelopeV1<Data: Codable & Equatable & Sendable>: Codable, Equatable, Sendable {
	let data: Data
}

/// What the desired write leaves unspecified stays at its current value —
/// callers always resend every field they know, matching `quota-settings`'
/// per-flag `Changed()` semantics without needing to track which single field
/// moved.
public struct DesktopQuotaSettingsDesiredV1: Equatable, Sendable {
	public var reading: Bool
	public var interval: DesktopQuotaIntervalV1
	public var alerts: Bool
	public var thresholds: [Double]
	public var resetNotice: Bool

	public init(reading: Bool, interval: DesktopQuotaIntervalV1, alerts: Bool, thresholds: [Double], resetNotice: Bool) {
		self.reading = reading
		self.interval = interval
		self.alerts = alerts
		self.thresholds = thresholds
		self.resetNotice = resetNotice
	}
}

/// One transport call's outcome as the controller needs to render it: a
/// decoded result, or a reason nothing could be decoded at all (the helper is
/// missing, the call timed out, or stdout was not the expected JSON) — as
/// distinct from a *decoded* `failed`/`restore_incomplete` sub-result, which
/// is success at the transport layer carrying a real failure as data.
public enum DesktopQuotaTransportOutcome<Value: Equatable & Sendable>: Equatable, Sendable {
	case decoded(Value)
	case undecodable
}

public protocol QuotaSettingsTransport: Sendable {
	func loadQuotaSettings() async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1>
	func applyQuotaSettings(_ desired: DesktopQuotaSettingsDesiredV1) async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1>
	func setQuotaStatusLine(enabled: Bool) async -> DesktopQuotaTransportOutcome<DesktopQuotaStatusLineResultV1>
}

public protocol DesktopQuotaRefreshing: Sendable {
	/// Runs one quota refresh and returns the alerts that are due. The helper
	/// records none of them as sent (architecture.md C10): the app posts them
	/// and acknowledges only the ones the notification service accepted.
	func refreshQuota(manual: Bool) async -> [DesktopQuotaAlertV1]
	func acknowledgeQuotaAlerts(ids: [String]) async
	/// Reads the current subscription section without probing anything --
	/// the same read-only, no-network `agentdeck quota` (C12) a quota-only
	/// refresh reuses to publish fresh figures without paying for the full
	/// desktop snapshot's session/usage scan.
	func fetchSubscription() async -> DesktopSubscriptionSnapshotV1?
}

/// Posts due quota alerts under the app's own identity and returns the ids the
/// notification service accepted. Anything not returned stays due and is
/// offered again by the next refresh.
public protocol QuotaAlertDelivering: Sendable {
	func deliver(_ alerts: [DesktopQuotaAlertV1]) async -> [String]
}

extension EmbeddedHelperRunner: DesktopQuotaRefreshing {
	public func refreshQuota(manual: Bool) async -> [DesktopQuotaAlertV1] {
		var arguments = ["desktop", "quota-refresh"]
		if manual { arguments.append("--manual") }
		let outcome: DesktopQuotaTransportOutcome<DesktopQuotaRefreshResultV1> = await runQuotaCommand(arguments, requestTimeout: Self.quotaRefreshTimeout)
		guard case let .decoded(result) = outcome else { return [] }
		return result.alerts
	}

	public func acknowledgeQuotaAlerts(ids: [String]) async {
		guard !ids.isEmpty else { return }
		let arguments = ["desktop", "quota-alerts", "ack"] + ids.flatMap { ["--id", $0] }
		let _: DesktopQuotaTransportOutcome<DesktopQuotaAlertsAckResultV1> = await runQuotaCommand(arguments)
	}

	public func fetchSubscription() async -> DesktopSubscriptionSnapshotV1? {
		let outcome: DesktopQuotaTransportOutcome<DesktopSubscriptionSnapshotV1> = await runQuotaCommand(["quota"])
		guard case let .decoded(subscription) = outcome else { return nil }
		return subscription
	}
}

public struct DesktopQuotaRefreshResultV1: Codable, Equatable, Sendable {
	public let gateReasons: [String: String?]
	public let alerts: [DesktopQuotaAlertV1]

	enum CodingKeys: String, CodingKey {
		case gateReasons = "gate_reasons"
		case alerts
	}

	public init(gateReasons: [String: String?], alerts: [DesktopQuotaAlertV1]) {
		self.gateReasons = gateReasons
		self.alerts = alerts
	}

	public init(from decoder: Decoder) throws {
		let container = try decoder.container(keyedBy: CodingKeys.self)
		gateReasons = try container.decodeIfPresent([String: String?].self, forKey: .gateReasons) ?? [:]
		alerts = try container.decodeIfPresent([DesktopQuotaAlertV1].self, forKey: .alerts) ?? []
	}
}

/// One due quota alert (architecture.md C10). `id` is opaque and passed back
/// unchanged on acknowledgement; it doubles as the notification request
/// identifier so a re-offered alert replaces rather than duplicates.
public struct DesktopQuotaAlertV1: Codable, Equatable, Sendable {
	public enum Kind: String, Codable, Sendable {
		case threshold
		case reset
	}

	public let id: String
	public let kind: Kind
	public let client: String
	public let label: String?
	public let windowMinutes: Int?
	public let usedPercent: Double
	public let threshold: Double?

	enum CodingKeys: String, CodingKey {
		case id, kind, client, label, threshold
		case windowMinutes = "window_minutes"
		case usedPercent = "used_percent"
	}

	public init(id: String, kind: Kind, client: String, label: String? = nil, windowMinutes: Int? = nil, usedPercent: Double, threshold: Double? = nil) {
		self.id = id
		self.kind = kind
		self.client = client
		self.label = label
		self.windowMinutes = windowMinutes
		self.usedPercent = usedPercent
		self.threshold = threshold
	}
}

public struct DesktopQuotaAlertsAckResultV1: Codable, Equatable, Sendable {
	public let acknowledged: Int
}

extension EmbeddedHelperRunner: QuotaSettingsTransport {
	public func loadQuotaSettings() async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		await runQuotaSettings(arguments: [])
	}

	public func applyQuotaSettings(_ desired: DesktopQuotaSettingsDesiredV1) async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		await runQuotaSettings(arguments: [
			"--reading", desired.reading ? "on" : "off",
			"--interval", desired.interval.flagValue,
			"--alerts", desired.alerts ? "on" : "off",
			"--thresholds", desired.thresholds.map { String(format: $0.truncatingRemainder(dividingBy: 1) == 0 ? "%.0f" : "%g", $0) }.joined(separator: ","),
			"--reset-notice", desired.resetNotice ? "on" : "off",
		])
	}

	private func runQuotaSettings(arguments: [String]) async -> DesktopQuotaTransportOutcome<DesktopQuotaSettingsResultV1> {
		await runQuotaCommand(["desktop", "quota-settings"] + arguments)
	}

	public func setQuotaStatusLine(enabled: Bool) async -> DesktopQuotaTransportOutcome<DesktopQuotaStatusLineResultV1> {
		await runQuotaCommand(["desktop", "quota-statusline", enabled ? "enable" : "disable"])
	}

	private func runQuotaCommand<Value: Codable & Equatable & Sendable>(
		_ subcommand: [String],
		requestTimeout: Duration? = nil
	) async -> DesktopQuotaTransportOutcome<Value> {
		let executableURL: URL
		do {
			executableURL = try embeddedHelperURL()
		} catch {
			return .undecodable
		}
		let output: HelperProcessOutput
		do {
			output = try await process.run(
				executableURL: executableURL,
				arguments: ["--format", "json"] + subcommand,
				environment: environment,
				timeout: requestTimeout ?? timeout
			)
		} catch {
			return .undecodable
		}
		guard !output.stdout.isEmpty,
			let envelope = try? JSONDecoder().decode(DesktopQuotaEnvelopeV1<Value>.self, from: output.stdout)
		else {
			return .undecodable
		}
		return .decoded(envelope.data)
	}
}

@MainActor
public protocol DesktopSnapshotRefreshing: AnyObject {
	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1
	func refresh(recentLimit: Int, progress: @escaping @Sendable (DesktopScanProgress) -> Void) async throws -> DesktopWireEnvelopeV1
}

public extension DesktopSnapshotRefreshing {
	func refresh(recentLimit: Int, progress: @escaping @Sendable (DesktopScanProgress) -> Void) async throws -> DesktopWireEnvelopeV1 {
		progress(.waiting)
		return try await refresh(recentLimit: recentLimit)
	}
}

extension DesktopHost: DesktopSnapshotRefreshing {}

public enum DesktopRefreshState: Equatable, Sendable {
	case uninitialized
	case refreshing(previous: DesktopWireEnvelopeV1?)
	case ready(DesktopWireEnvelopeV1)
	case degraded(previous: DesktopWireEnvelopeV1?, issue: DesktopRefreshIssue)
}

public enum DesktopRefreshIssue: Equatable, Sendable {
	case helper(HelperExecutionError)
	case invalidWire(DesktopWireError)
	case storageUnavailable
	case unavailable
}

public enum DesktopPresentationSurface: Equatable, Sendable {
	case loadingSurface
	case dataSurface
	case errorSurface
}

public enum DesktopPresentationQualifier: String, Equatable, Sendable {
	case stale
	case aged
	case offline
	case failing
	case partial
	case empty
}

public struct DesktopPresentationState: Equatable, Sendable {
	public let surface: DesktopPresentationSurface
	public let qualifiers: [DesktopPresentationQualifier]
	public let snapshot: DesktopWireEnvelopeV1?

	public var isBadged: Bool {
		qualifiers.contains(.offline) || qualifiers.contains(.failing)
	}

	public static func derive(from state: DesktopRefreshState, now: Date = Date()) -> DesktopPresentationState {
		var surface: DesktopPresentationSurface
		var snapshot: DesktopWireEnvelopeV1?
		var stale = false
		var issue: DesktopRefreshIssue?

		switch state {
		case .uninitialized:
			surface = .loadingSurface
		case let .refreshing(previous):
			snapshot = previous
			surface = previous == nil ? .loadingSurface : .dataSurface
			stale = previous != nil
		case let .ready(envelope):
			snapshot = envelope
			surface = .dataSurface
		case let .degraded(previous, currentIssue):
			snapshot = previous
			issue = currentIssue
			surface = previous == nil ? .errorSurface : .dataSurface
			stale = previous != nil
		}

		var qualifiers = [DesktopPresentationQualifier]()
		if stale {
			qualifiers.append(.stale)
		}
		if let snapshot,
			let generatedAt = desktopTimestamp(snapshot.data.generatedAt),
			now.timeIntervalSince(generatedAt) > 15 * 60
		{
			qualifiers.append(.aged)
		}
		if let issue {
			switch issue {
			case .invalidWire, .storageUnavailable:
				qualifiers.append(.failing)
			case let .helper(error):
				switch error {
				case .missingEmbeddedHelper, .launchFailed:
					qualifiers.append(.offline)
				default:
					qualifiers.append(.failing)
				}
			case .unavailable:
				qualifiers.append(.offline)
			}
		}
		if let snapshot {
			let partial = desktopSnapshotIsPartial(snapshot)
			if partial {
				qualifiers.append(.partial)
			} else if desktopSnapshotIsEmpty(snapshot) {
				qualifiers.append(.empty)
			}
		}
		return DesktopPresentationState(surface: surface, qualifiers: qualifiers, snapshot: snapshot)
	}
}

private func desktopTimestamp(_ value: String) -> Date? {
	let fractional = ISO8601DateFormatter()
	fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
	if let parsed = fractional.date(from: value) {
		return parsed
	}
	let basic = ISO8601DateFormatter()
	basic.formatOptions = [.withInternetDateTime]
	return basic.date(from: value)
}

private func desktopSnapshotIsPartial(_ envelope: DesktopWireEnvelopeV1) -> Bool {
	let data = envelope.data
	guard !envelope.partial,
		data.provider.available,
		data.usage.available,
		data.sessions.available,
		data.health.available,
		data.usage.presentation.available
	else {
		return true
	}
	return data.usage.presentation.scopes.contains { scope in
		!scope.periods.available || !scope.daily.available || !scope.quality.available || !scope.pricing.available || !scope.rhythm.available
	} || !data.usage.presentation.clientSubtotals.available
}

private func desktopSnapshotIsEmpty(_ envelope: DesktopWireEnvelopeV1) -> Bool {
	guard let scope = envelope.data.usage.presentation.scopes.first(where: { $0.client == "all" }),
		let today = scope.periods.items.first(where: { $0.period == "today" })
	else {
		return true
	}
	return today.totals.tokens == 0 && today.totals.events == 0 && today.totals.sessions == 0
}

public enum SwitchControllerState: Equatable, Sendable {
	case idle
	case inFlight(ProviderSwitchTarget)
	case succeeded(ProviderSwitchTarget)
	case failed(ProviderSwitchTarget, code: String)
	case indeterminate(ProviderSwitchTarget)

	public var target: ProviderSwitchTarget? {
		switch self {
		case .idle:
			return nil
		case let .inFlight(target), let .succeeded(target), let .failed(target, _), let .indeterminate(target):
			return target
		}
	}

	public var isInFlight: Bool {
		if case .inFlight = self {
			return true
		}
		return false
	}
}

@MainActor
@Observable
public final class SwitchController {
	public private(set) var state: SwitchControllerState = .idle

	private let transport: any ProviderSwitching
	private let refreshCoordinator: DesktopRefreshCoordinator
	@ObservationIgnored private var activeTask: Task<Void, Never>?
	@ObservationIgnored private var successTimer: Task<Void, Never>?

	public init(
		transport: any ProviderSwitching = EmbeddedHelperRunner(),
		refreshCoordinator: DesktopRefreshCoordinator
	) {
		self.transport = transport
		self.refreshCoordinator = refreshCoordinator
	}

	@discardableResult
	public func start(_ target: ProviderSwitchTarget) -> Bool {
		guard case .idle = state else {
			return false
		}
		launch(target)
		return true
	}

	@discardableResult
	public func retry() -> Bool {
		let target: ProviderSwitchTarget
		switch state {
		case let .failed(value, _), let .indeterminate(value):
			target = value
		default:
			return false
		}
		launch(target)
		return true
	}

	public func dismiss() {
		switch state {
		case .failed, .indeterminate, .succeeded:
			state = .idle
		default:
			break
		}
	}

	private func launch(_ target: ProviderSwitchTarget) {
		successTimer?.cancel()
		state = .inFlight(target)
		let transport = self.transport
		let detached = Task.detached(priority: .userInitiated) {
			await transport.switchProvider(target)
		}
		activeTask = Task { [weak self] in
			let outcome = await detached.value
			await self?.finish(outcome, target: target)
		}
	}

	private func finish(_ outcome: ProviderSwitchTransportOutcome, target: ProviderSwitchTarget) async {
		guard state == .inFlight(target) else {
			return
		}
		activeTask = nil
		switch outcome {
		case .succeeded:
			state = .succeeded(target)
			successTimer = Task { [weak self] in
				try? await Task.sleep(for: .seconds(10))
				guard let self, self.state == .succeeded(target) else { return }
				self.state = .idle
			}
			await refreshCoordinator.requestFullRefresh(trigger: .providerSwitch)
			if state == .succeeded(target) {
				state = .idle
			}
		case let .failed(code):
			state = .failed(target, code: code)
		case .indeterminate, .opaque:
			state = .indeterminate(target)
			await refreshCoordinator.requestFullRefresh(trigger: .providerSwitch)
		}
	}
}

public enum DesktopFullRefreshTrigger: Int, Equatable, Sendable {
	case startup
	case periodic
	case wake
	case manual
	case providerSwitch

	var requestsFollowUp: Bool { self == .manual || self == .providerSwitch }
	var quotaIsManual: Bool { self == .manual || self == .providerSwitch }

	static func merged(_ current: Self?, _ incoming: Self) -> Self? {
		guard incoming.requestsFollowUp else { return current }
		guard let current else { return incoming }
		return current.rawValue >= incoming.rawValue ? current : incoming
	}
}

public enum DesktopFullAttemptState: Equatable, Sendable {
	case idle
	case running(generation: Int)
	case succeeded(generation: Int, completedAt: Date)
	case failed(generation: Int, issue: DesktopRefreshIssue, completedAt: Date)
}

public enum DesktopWidgetPublicationState: Equatable, Sendable {
	case neverPublished
	case succeeded(generation: UInt64, affectedKinds: Set<AppGroupWidgetKind>)
	case failedBeforeCommit(generation: UInt64, issue: WidgetSnapshotPublisherIssue)
	case indeterminateAfterCommit(generation: UInt64, issue: WidgetSnapshotPublisherIssue)
}

public struct QuotaOperationID: Hashable, Sendable {
	public let rawValue: UInt64
}

private enum QuotaRequestPriority: Int, Sendable {
	case periodic
	case manual

	static func merged(_ current: Self?, _ incoming: Self) -> Self {
		guard let current else { return incoming }
		return current.rawValue >= incoming.rawValue ? current : incoming
	}
}

private enum QuotaPublicationMode: Sendable {
	case standalone
	case deferToFull(generation: Int)
}

private struct QuotaOperationResult: Sendable {
	let id: QuotaOperationID
	let subscription: DesktopSubscriptionSnapshotV1?
}

@MainActor
@Observable
public final class DesktopRefreshCoordinator {
	public private(set) var state: DesktopRefreshState = .uninitialized
	public private(set) var latestSnapshot: DesktopWireEnvelopeV1?
	public private(set) var scanProgress: DesktopScanProgress?
	public private(set) var fullAttempt: DesktopFullAttemptState = .idle
	public private(set) var widgetPublication: DesktopWidgetPublicationState = .neverPublished

	private let host: any DesktopSnapshotRefreshing
	private let quotaRefresher: (any DesktopQuotaRefreshing)?
	private let alertDeliverer: (any QuotaAlertDelivering)?
	private let snapshotPublisher: WidgetSnapshotPublisher?
	@ObservationIgnored private let wallNow: @MainActor @Sendable () -> Date
	@ObservationIgnored private var activeRefresh: Task<Void, Never>?
	@ObservationIgnored private var activeQuotaOperation: Task<QuotaOperationResult, Never>?
	@ObservationIgnored private var activeQuotaID: QuotaOperationID?
	@ObservationIgnored private var pendingFullTrigger: DesktopFullRefreshTrigger?
	@ObservationIgnored private var pendingQuotaPriority: QuotaRequestPriority?
	@ObservationIgnored private var activeFullGeneration: Int?
	@ObservationIgnored private var fullQuotaPhaseCompleted = false
	@ObservationIgnored private var generation = 0
	@ObservationIgnored private var quotaOperationSequence: UInt64 = 0
	@ObservationIgnored private var widgetPublicationGeneration: UInt64 = 0
	@ObservationIgnored private var successResetToken: UInt64 = 0
	@ObservationIgnored private var fullAttemptTerminalHandler: (@MainActor @Sendable () -> Void)?

	public init(
		host: any DesktopSnapshotRefreshing = DesktopHost(),
		quotaRefresher: (any DesktopQuotaRefreshing)? = nil,
		alertDeliverer: (any QuotaAlertDelivering)? = nil,
		snapshotStore: AppGroupSnapshotStore? = AppGroupSnapshotStore(),
		wallNow: @escaping @MainActor @Sendable () -> Date = Date.init
	) {
		self.host = host
		self.quotaRefresher = quotaRefresher
		self.alertDeliverer = alertDeliverer
		self.wallNow = wallNow
		snapshotPublisher = snapshotStore.map { WidgetSnapshotPublisher(store: $0) }
	}

	public func setFullAttemptTerminalHandler(_ handler: @escaping @MainActor @Sendable () -> Void) {
		fullAttemptTerminalHandler = handler
	}

	// Starts startup work without making the application launch wait for the
	// embedded helper. Later views use refresh() for an awaitable manual retry.
	@discardableResult
	public func startInitialRefresh() -> Task<Void, Never> {
		Task { [weak self] in
			await self?.requestFullRefresh(trigger: .startup)
		}
	}

	/// Runs only C9's quota probe and C10's alert delivery, without the full
	/// desktop snapshot refresh `refresh(...)` otherwise couples it to.
	///
	/// Codex PR #5 tenth review, P1: quota alerts were evaluated only as a
	/// side effect of `refresh(...)`, whose own background schedule
	/// (`AgentDeckApp.startPeriodicRefresh`) is gated on the separate,
	/// independently opt-in-and-off-by-default "Periodic refresh" General
	/// preference -- rescanning sessions/usage on every tick. A user who
	/// enables quota reading and alerts but leaves that unrelated preference
	/// untouched got no threshold or reset notifications until a manual
	/// refresh or relaunch. This lighter entry point lets a caller schedule
	/// alert evaluation on its own cadence, independent of that preference
	/// and without paying for a full snapshot rescan every tick.
	public func refreshQuotaAlertsOnly(manual: Bool) async {
		await requestQuotaRefresh(manual: manual)
	}

	public func requestQuotaRefresh(manual: Bool) async {
		let priority: QuotaRequestPriority = manual ? .manual : .periodic
		if activeQuotaOperation != nil {
			_ = await runQuotaOperation(priority: priority, mode: .standalone)
			return
		}
		if let activeRefresh, fullQuotaPhaseCompleted {
			pendingQuotaPriority = QuotaRequestPriority.merged(pendingQuotaPriority, priority)
			await activeRefresh.value
			await drainPendingQuotaIfPossible()
			return
		}
		let mode = activeFullGeneration.map(QuotaPublicationMode.deferToFull) ?? .standalone
		_ = await runQuotaOperation(priority: priority, mode: mode)
	}

	public func refresh(
		recentLimit: Int = EmbeddedHelperRunner.defaultRecentLimit,
		manualQuota: Bool = true
	) async {
		let trigger: DesktopFullRefreshTrigger = manualQuota ? .manual : .periodic
		await requestFullRefresh(trigger: trigger, recentLimit: recentLimit)
	}

	public func requestFullRefresh(
		trigger: DesktopFullRefreshTrigger,
		recentLimit: Int = EmbeddedHelperRunner.defaultRecentLimit
	) async {
		if let activeRefresh {
			pendingFullTrigger = DesktopFullRefreshTrigger.merged(pendingFullTrigger, trigger)
			await activeRefresh.value
			return
		}

		let task = Task { [weak self] in
			guard let self else { return }
			await self.runFullSequence(first: trigger, recentLimit: recentLimit)
		}
		activeRefresh = task
		await task.value
	}

	private func runFullSequence(first: DesktopFullRefreshTrigger, recentLimit: Int) async {
		var next: DesktopFullRefreshTrigger? = first
		while let trigger = next {
			await performFullRefresh(trigger: trigger, recentLimit: recentLimit)
			if let pendingFullTrigger {
				next = pendingFullTrigger
				self.pendingFullTrigger = nil
			} else {
				next = nil
			}
		}
		activeFullGeneration = nil
		fullQuotaPhaseCompleted = false
		activeRefresh = nil
		fullAttemptTerminalHandler?()
		await drainPendingQuotaIfPossible()
	}

	private func performFullRefresh(trigger: DesktopFullRefreshTrigger, recentLimit: Int) async {
		generation &+= 1
		let currentGeneration = generation
		activeFullGeneration = currentGeneration
		fullQuotaPhaseCompleted = false
		successResetToken &+= 1
		fullAttempt = .running(generation: currentGeneration)
		state = .refreshing(previous: latestSnapshot)
		scanProgress = .waiting

		let defaultPriority: QuotaRequestPriority = trigger.quotaIsManual ? .manual : .periodic
		let priority = QuotaRequestPriority.merged(pendingQuotaPriority, defaultPriority)
		pendingQuotaPriority = nil
		_ = await runQuotaOperation(priority: priority, mode: .deferToFull(generation: currentGeneration))
		fullQuotaPhaseCompleted = true
		guard currentGeneration == generation, !Task.isCancelled else { return }

		do {
			let envelope = try await host.refresh(recentLimit: recentLimit) { [weak self] progress in
				Task { @MainActor in
					self?.publishProgress(progress, generation: currentGeneration)
				}
			}
			guard currentGeneration == generation, !Task.isCancelled else { return }
			await publishSuccess(envelope, generation: currentGeneration)
		} catch is CancellationError {
			return
		} catch let error as HelperExecutionError {
			guard error != .cancelled else { return }
			publishFailure(.helper(error), generation: currentGeneration)
		} catch let error as DesktopWireError {
			publishFailure(.invalidWire(error), generation: currentGeneration)
		} catch {
			publishFailure(.unavailable, generation: currentGeneration)
		}
	}

	private func runQuotaOperation(
		priority: QuotaRequestPriority,
		mode: QuotaPublicationMode
	) async -> QuotaOperationResult {
		if let activeQuotaOperation {
			return await activeQuotaOperation.value
		}

		quotaOperationSequence &+= 1
		let id = QuotaOperationID(rawValue: quotaOperationSequence)
		let baseline = latestSnapshot
		let task = Task { @MainActor [weak self] in
			guard let self else { return QuotaOperationResult(id: id, subscription: nil) }
			return await self.performQuotaOperation(id: id, priority: priority, mode: mode, baseline: baseline)
		}
		activeQuotaID = id
		activeQuotaOperation = task
		let result = await task.value
		if activeQuotaID == id {
			activeQuotaOperation = nil
			activeQuotaID = nil
		}
		return result
	}

	private func performQuotaOperation(
		id: QuotaOperationID,
		priority: QuotaRequestPriority,
		mode: QuotaPublicationMode,
		baseline: DesktopWireEnvelopeV1?
	) async -> QuotaOperationResult {
		guard let quotaRefresher else {
			return QuotaOperationResult(id: id, subscription: nil)
		}
		let alerts = await quotaRefresher.refreshQuota(manual: priority == .manual)
		if !alerts.isEmpty, let alertDeliverer {
			let delivered = await alertDeliverer.deliver(alerts)
			await quotaRefresher.acknowledgeQuotaAlerts(ids: delivered)
		}
		let subscription = await quotaRefresher.fetchSubscription()
		let result = QuotaOperationResult(id: id, subscription: subscription)
		if case .standalone = mode {
			await publishStandaloneQuota(result, baseline: baseline)
		}
		return result
	}

	private func publishStandaloneQuota(
		_ result: QuotaOperationResult,
		baseline: DesktopWireEnvelopeV1?
	) async {
		guard let subscription = result.subscription,
			let baseline,
			latestSnapshot == baseline
		else { return }
		let updated = baseline.replacingSubscription(subscription)
		latestSnapshot = updated
		switch state {
		case .ready:
			state = .ready(updated)
		case let .degraded(_, issue):
			state = .degraded(previous: updated, issue: issue)
		case .refreshing:
			state = .refreshing(previous: updated)
		case .uninitialized:
			return
		}
		await publishWidgetSnapshot(updated)
	}

	private func drainPendingQuotaIfPossible() async {
		guard activeRefresh == nil, activeQuotaOperation == nil, let priority = pendingQuotaPriority else { return }
		pendingQuotaPriority = nil
		_ = await runQuotaOperation(priority: priority, mode: .standalone)
	}

	private func publishProgress(_ progress: DesktopScanProgress, generation: Int) {
		guard generation == self.generation else {
			return
		}
		switch state {
		case .refreshing:
			break
		case .degraded where progress.stage == .completed:
			break
		default:
			return
		}
		acceptProgress(progress)
	}

	private func acceptProgress(_ progress: DesktopScanProgress) {
		// The synthetic .waiting seed is reported by DesktopSnapshotRefreshing's
		// default extension through an unawaited Task, so it can arrive at any
		// point relative to the refresh's own completion — including after a
		// fast failure has already cleared it (see publishFailure). It never
		// carries information beyond the seed refresh() already set directly,
		// so it is never worth accepting, early or late.
		guard progress.stage != .waiting else { return }
		if let current = scanProgress, progress.sequence < current.sequence {
			return
		}
		scanProgress = progress
	}

	private func publishSuccess(_ envelope: DesktopWireEnvelopeV1, generation: Int) async {
		guard generation == self.generation else {
			return
		}

		latestSnapshot = envelope
		scanProgress = nil
		state = .ready(envelope)
		fullAttempt = .succeeded(generation: generation, completedAt: wallNow())
		await publishWidgetSnapshot(envelope)
		scheduleSuccessReset(generation: generation)
	}

	private func publishWidgetSnapshot(_ envelope: DesktopWireEnvelopeV1) async {
		guard let snapshotPublisher else { return }
		widgetPublicationGeneration &+= 1
		let publicationGeneration = widgetPublicationGeneration
		let result = await snapshotPublisher.publish(
			AppGroupDesktopSnapshotV1(envelope: envelope),
			generation: publicationGeneration
		)
		switch result {
		case let .published(affectedKinds):
			widgetPublication = .succeeded(generation: publicationGeneration, affectedKinds: affectedKinds)
		case .superseded:
			break
		case let .failedBeforeCommit(_, issue):
			widgetPublication = .failedBeforeCommit(generation: publicationGeneration, issue: issue)
		case let .indeterminateAfterCommit(_, issue):
			widgetPublication = .indeterminateAfterCommit(generation: publicationGeneration, issue: issue)
		}
	}

	private func publishFailure(_ issue: DesktopRefreshIssue, generation: Int) {
		guard generation == self.generation else {
			return
		}
		fullAttempt = .failed(generation: generation, issue: issue, completedAt: wallNow())
		state = .degraded(previous: latestSnapshot, issue: issue)
		// A retained terminal progress report (e.g. completed with a domain
		// marked failed) is meaningful and the menu bar displays it alongside
		// the failure. Only the synthetic seed set at refresh start — meaning
		// the helper never reported real progress before failing — is stale.
		if scanProgress == .waiting {
			scanProgress = nil
		}
	}

	private func scheduleSuccessReset(generation: Int) {
		successResetToken &+= 1
		let token = successResetToken
		DispatchQueue.main.asyncAfter(deadline: .now() + 1.6) { [weak self] in
			guard let self, self.successResetToken == token else { return }
			if case let .succeeded(currentGeneration, _) = self.fullAttempt,
				currentGeneration == generation
			{
				self.fullAttempt = .idle
			}
		}
	}
}

private func waitForExit(_ running: RunningProcess, timeout: Duration) async throws -> Int32 {
    try await withThrowingTaskGroup(of: Int32.self) { group in
        group.addTask {
            await running.waitForTermination()
        }
        group.addTask {
            try await Task.sleep(for: timeout)
            throw HelperExecutionError.timedOut
        }

        defer { group.cancelAll() }
        do {
            guard let result = try await group.next() else {
                throw HelperExecutionError.launchFailed
            }
            return result
        } catch {
            running.terminate()
            throw error
        }
    }
}

private final class RunningProcess: @unchecked Sendable {
    private let process: Process
    private let lock = NSLock()

    init(_ process: Process) {
        self.process = process
    }

    func waitForTermination() async -> Int32 {
        await withCheckedContinuation { continuation in
            lock.lock()
            if !process.isRunning {
                let exitStatus = process.terminationStatus
                lock.unlock()
                continuation.resume(returning: exitStatus)
                return
            }
            process.terminationHandler = { finishedProcess in
                continuation.resume(returning: finishedProcess.terminationStatus)
            }
            lock.unlock()
        }
    }

    func terminate() {
        lock.lock()
        defer { lock.unlock() }
        if process.isRunning {
            process.terminate()
        }
    }
}

private final class BoundedData: @unchecked Sendable {
    private let maximumBytes: Int
    private let lock = NSLock()
    private var storage = Data()
    private var truncated = false

    init(maximumBytes: Int) {
        self.maximumBytes = maximumBytes
    }

    func append(_ data: Data) {
        lock.lock()
        defer { lock.unlock() }

        let remaining = max(0, maximumBytes - storage.count)
        if remaining > 0 {
            storage.append(data.prefix(remaining))
        }
        if data.count > remaining {
            truncated = true
        }
    }

    var value: Data {
        lock.lock()
        defer { lock.unlock() }
        return storage
    }

    var wasTruncated: Bool {
        lock.lock()
        defer { lock.unlock() }
        return truncated
    }
}

private final class BoundedLines: @unchecked Sendable {
	private let maximumLineBytes: Int
	private let maximumLines: Int
	private let lock = NSLock()
	private var storage = [Data]()
	private var current = Data()
	private var byteCount = 0
	private var truncated = false
	private var discardingLine = false
	private let onLine: @Sendable (Data) -> Void

	init(maximumLineBytes: Int, maximumLines: Int, onLine: @escaping @Sendable (Data) -> Void = { _ in }) {
		self.maximumLineBytes = maximumLineBytes
		self.maximumLines = maximumLines
		self.onLine = onLine
	}

	func append(_ data: Data) {
		lock.lock()
		defer { lock.unlock() }
		byteCount += data.count
		for byte in data {
			if byte == 0x0A {
				finishCurrentLine()
				continue
			}
			guard !discardingLine else { continue }
			guard current.count < maximumLineBytes else {
				truncated = true
				discardingLine = true
				current.removeAll(keepingCapacity: false)
				continue
			}
			current.append(byte)
		}
	}

	func finish() {
		lock.lock()
		defer { lock.unlock() }
		if !current.isEmpty || discardingLine {
			finishCurrentLine()
		}
	}

	private func finishCurrentLine() {
		defer {
			current.removeAll(keepingCapacity: true)
			discardingLine = false
		}
		guard !discardingLine, !current.isEmpty else { return }
		guard storage.count < maximumLines else {
			truncated = true
			return
		}
		storage.append(current)
		onLine(current)
	}

	var lines: [Data] {
		lock.lock()
		defer { lock.unlock() }
		return storage
	}

	var totalBytes: Int {
		lock.lock()
		defer { lock.unlock() }
		return byteCount
	}

	var wasTruncated: Bool {
		lock.lock()
		defer { lock.unlock() }
		return truncated
	}
}
