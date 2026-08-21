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
}

public extension EmbeddedHelperProcess {
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

        startCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
        startCapture(from: stderrPipe.fileHandleForReading, into: stderr)

        let running = RunningProcess(process)
        do {
            try process.run()
        } catch {
            finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
            finishCapture(from: stderrPipe.fileHandleForReading, into: stderr)
            throw HelperExecutionError.launchFailed
        }

        do {
            let exitStatus = try await withTaskCancellationHandler(
                operation: { try await waitForExit(running, timeout: timeout) },
                onCancel: { running.terminate() }
            )
            if Task.isCancelled {
                throw HelperExecutionError.cancelled
            }
            finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
            finishCapture(from: stderrPipe.fileHandleForReading, into: stderr)
            return HelperProcessOutput(
                exitStatus: exitStatus,
                stdout: stdout.value,
                stderr: stderr.value,
                stdoutTruncated: stdout.wasTruncated,
                stderrTruncated: stderr.wasTruncated
            )
        } catch is CancellationError {
            running.terminate()
            finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
            finishCapture(from: stderrPipe.fileHandleForReading, into: stderr)
            throw HelperExecutionError.cancelled
        } catch {
            running.terminate()
            finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
            finishCapture(from: stderrPipe.fileHandleForReading, into: stderr)
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
		if Task.isCancelled {
			throw HelperExecutionError.cancelled
		}

		let process = Process()
		let stdoutPipe = Pipe()
		let stderrPipe = Pipe()
		let stdout = BoundedLines(maximumLineBytes: maximumLineBytes, maximumLines: maximumLines)
		let stderr = BoundedData(maximumBytes: Self.maximumCapturedBytes)

		process.executableURL = executableURL
		process.arguments = arguments
		process.environment = environment
		process.standardInput = FileHandle.nullDevice
		process.standardOutput = stdoutPipe
		process.standardError = stderrPipe

		startCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
		startCapture(from: stderrPipe.fileHandleForReading, into: stderr)

		let running = RunningProcess(process)
		do {
			try process.run()
		} catch {
			finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
			finishCapture(from: stderrPipe.fileHandleForReading, into: stderr)
			throw HelperExecutionError.launchFailed
		}

		do {
			let exitStatus = try await withTaskCancellationHandler(
				operation: { try await waitForExit(running, timeout: timeout) },
				onCancel: { running.terminate() }
			)
			if Task.isCancelled {
				throw HelperExecutionError.cancelled
			}
			finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
			finishCapture(from: stderrPipe.fileHandleForReading, into: stderr)
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
			finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
			finishCapture(from: stderrPipe.fileHandleForReading, into: stderr)
			throw HelperExecutionError.cancelled
		} catch {
			running.terminate()
			finishCapture(from: stdoutPipe.fileHandleForReading, into: stdout)
			finishCapture(from: stderrPipe.fileHandleForReading, into: stderr)
			if let helperError = error as? HelperExecutionError {
				throw helperError
			}
			throw HelperExecutionError.launchFailed
		}
	}

    private func startCapture(from handle: FileHandle, into capture: BoundedData) {
        handle.readabilityHandler = { readableHandle in
            let chunk = readableHandle.availableData
            if chunk.isEmpty {
                readableHandle.readabilityHandler = nil
                return
            }
            capture.append(chunk)
        }
    }

    private func finishCapture(from handle: FileHandle, into capture: BoundedData) {
        handle.readabilityHandler = nil
        capture.append(handle.readDataToEndOfFile())
        try? handle.close()
    }

	private func startCapture(from handle: FileHandle, into capture: BoundedLines) {
		handle.readabilityHandler = { readableHandle in
			let chunk = readableHandle.availableData
			if chunk.isEmpty {
				readableHandle.readabilityHandler = nil
				return
			}
			capture.append(chunk)
		}
	}

	private func finishCapture(from handle: FileHandle, into capture: BoundedLines) {
		handle.readabilityHandler = nil
		capture.append(handle.readDataToEndOfFile())
		capture.finish()
		try? handle.close()
	}
}

public struct EmbeddedHelperRunner: Sendable {
    public static let defaultRecentLimit = 5
	public static let minimumRecentLimit = 1
	public static let maximumRecentLimit = 20
	public static let defaultTimeout: Duration = .seconds(30)
	public static let indexRefreshTimeout: Duration = .seconds(120)
	public static let maximumStreamLineBytes = 96 * 1024
	public static let maximumStreamLines = 2_048
	public static let maximumSnapshotBytes = 64 * 1024 * 1024

    private let appBundleURL: URL
    private let process: any EmbeddedHelperProcess
    private let environment: [String: String]
    private let timeout: Duration

    public init(
        appBundleURL: URL,
        process: any EmbeddedHelperProcess = FoundationEmbeddedHelperProcess(),
        environment: [String: String]? = nil,
        timeout: Duration = Self.defaultTimeout
    ) {
        self.appBundleURL = appBundleURL
        self.process = process
        self.environment = environment ?? Self.defaultEnvironment()
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
        recentLimit: Int = Self.defaultRecentLimit
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
		try await refreshIndexes(executableURL: executableURL, refreshID: refreshID)
		let requestStartedAt = Date()
		let output: HelperProcessLinesOutput
        do {
			output = try await process.runLines(
                executableURL: executableURL,
                arguments: [
                    "--format", "json",
                    "desktop", "snapshot",
                    "--wire-version", "1",
                    "--recent-limit", String(recentLimit),
					"--stream",
                ],
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

	private func refreshIndexes(executableURL: URL, refreshID: String) async throws {
		let stage = "index_refresh_parallel"
		let startedAt = Date()
		do {
			let output = try await process.run(
				executableURL: executableURL,
				arguments: ["--quiet", "--format", "json", "desktop", "refresh-indexes"],
				environment: environment,
				timeout: Self.indexRefreshTimeout
			)
			DesktopLogger.recordHelperRequest(
				id: refreshID,
				stage: stage,
				durationMilliseconds: elapsedMilliseconds(since: startedAt),
				stdoutBytes: output.stdout.count,
				stderrBytes: output.stderr.count,
				chunks: 0,
				exitStatus: output.exitStatus,
				stdoutTruncated: output.stdoutTruncated,
				stderrTruncated: output.stderrTruncated,
				errorCode: desktopHelperErrorCode(stderr: output.stderr)
			)
			if output.exitStatus == 0,
				!output.stdoutTruncated,
				!output.stderrTruncated,
				let result = try? decodeDesktopIndexRefreshResult(output.stdout)
			{
				for (domain, value) in [("usage", result.usage), ("sessions", result.sessions)] {
					DesktopLogger.recordIndexDomain(
						id: refreshID,
						domain: domain,
						success: value.success,
						durationMilliseconds: value.durationMilliseconds,
						errorCode: value.errorCode
					)
				}
			}
			if Task.isCancelled {
				throw HelperExecutionError.cancelled
			}
		} catch is CancellationError {
			throw HelperExecutionError.cancelled
		} catch let error as HelperExecutionError where error == .cancelled {
			throw error
		} catch {
			DesktopLogger.recordHelperRequestFailure(
				id: refreshID,
				stage: stage,
				error: (error as? HelperExecutionError) ?? .launchFailed,
				durationMilliseconds: elapsedMilliseconds(since: startedAt)
			)
			// A refresh remains useful with the last committed indexes. The
			// read-only desktop snapshot below still returns that bounded state.
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
				arguments: arguments,
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

    private static func defaultEnvironment() -> [String: String] {
        [
            "HOME": FileManager.default.homeDirectoryForCurrentUser.path,
            "LANG": "en_US_POSIX",
            "LC_ALL": "en_US_POSIX",
            "PATH": "/usr/bin:/bin",
        ]
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
        do {
            let envelope = try await runner.snapshot(recentLimit: recentLimit)
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

@MainActor
public protocol DesktopSnapshotRefreshing: AnyObject {
	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1
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
			await refreshCoordinator.refresh(replacingActiveRefresh: true)
			if state == .succeeded(target) {
				state = .idle
			}
		case let .failed(code):
			state = .failed(target, code: code)
		case .indeterminate, .opaque:
			state = .indeterminate(target)
			await refreshCoordinator.refresh(replacingActiveRefresh: true)
		}
	}
}

@MainActor
@Observable
public final class DesktopRefreshCoordinator {
	public private(set) var state: DesktopRefreshState = .uninitialized
	public private(set) var latestSnapshot: DesktopWireEnvelopeV1?

	private let host: any DesktopSnapshotRefreshing
	private let snapshotStore: AppGroupSnapshotStore?
	@ObservationIgnored private var activeRefresh: Task<Void, Never>?
	@ObservationIgnored private var generation = 0

	public init(
		host: any DesktopSnapshotRefreshing = DesktopHost(),
		snapshotStore: AppGroupSnapshotStore? = AppGroupSnapshotStore()
	) {
		self.host = host
		self.snapshotStore = snapshotStore
	}

	// Starts startup work without making the application launch wait for the
	// embedded helper. Later views use refresh() for an awaitable manual retry.
	@discardableResult
	public func startInitialRefresh() -> Task<Void, Never> {
		Task { [weak self] in
			await self?.refresh()
		}
	}

	public func refresh(
		recentLimit: Int = EmbeddedHelperRunner.defaultRecentLimit,
		replacingActiveRefresh: Bool = false
	) async {
		if let activeRefresh {
			guard replacingActiveRefresh else {
				await activeRefresh.value
				return
			}
			generation &+= 1
			activeRefresh.cancel()
			await activeRefresh.value
		}

		generation &+= 1
		let currentGeneration = generation
		let previousSnapshot = latestSnapshot
		state = .refreshing(previous: previousSnapshot)

		let task = Task { [weak self] in
			guard let self else {
				return
			}
			do {
				let envelope = try await self.host.refresh(recentLimit: recentLimit)
				guard !Task.isCancelled else {
					return
				}
				self.publishSuccess(envelope, generation: currentGeneration)
			} catch is CancellationError {
				return
			} catch let error as HelperExecutionError {
				guard error != .cancelled else {
					return
				}
				self.publishFailure(.helper(error), generation: currentGeneration)
			} catch let error as DesktopWireError {
				self.publishFailure(.invalidWire(error), generation: currentGeneration)
			} catch {
				self.publishFailure(.unavailable, generation: currentGeneration)
			}
		}
		activeRefresh = task
		await task.value
	}

	private func publishSuccess(_ envelope: DesktopWireEnvelopeV1, generation: Int) {
		guard generation == self.generation else {
			return
		}

		latestSnapshot = envelope
		do {
			if let snapshotStore {
				try snapshotStore.write(AppGroupDesktopSnapshotV1(envelope: envelope))
			}
			state = .ready(envelope)
		} catch {
			state = .degraded(previous: envelope, issue: .storageUnavailable)
		}
		activeRefresh = nil
	}

	private func publishFailure(_ issue: DesktopRefreshIssue, generation: Int) {
		guard generation == self.generation else {
			return
		}
		state = .degraded(previous: latestSnapshot, issue: issue)
		activeRefresh = nil
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

	init(maximumLineBytes: Int, maximumLines: Int) {
		self.maximumLineBytes = maximumLineBytes
		self.maximumLines = maximumLines
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
