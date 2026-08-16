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

public protocol EmbeddedHelperProcess: Sendable {
    func run(
        executableURL: URL,
        arguments: [String],
        environment: [String: String],
        timeout: Duration
    ) async throws -> HelperProcessOutput
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
    public static let maximumCapturedBytes = 256 * 1024

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
}

public struct EmbeddedHelperRunner: Sendable {
    public static let defaultRecentLimit = 5
    public static let minimumRecentLimit = 1
    public static let maximumRecentLimit = 20
    public static let defaultTimeout: Duration = .seconds(10)

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

        let executableURL = try embeddedHelperURL()
        let output: HelperProcessOutput
        do {
            output = try await process.run(
                executableURL: executableURL,
                arguments: [
                    "--format", "json",
                    "desktop", "snapshot",
                    "--wire-version", "1",
                    "--recent-limit", String(recentLimit),
                ],
                environment: environment,
                timeout: timeout
            )
        } catch is CancellationError {
            throw HelperExecutionError.cancelled
        } catch let error as HelperExecutionError {
            throw error
        } catch {
            throw HelperExecutionError.launchFailed
        }

        if Task.isCancelled {
            throw HelperExecutionError.cancelled
        }
        guard !output.stdoutTruncated, !output.stderrTruncated else {
            throw HelperExecutionError.outputLimitExceeded
        }
        guard output.exitStatus == 0 else {
            throw HelperExecutionError.nonZeroExit(output.exitStatus)
        }

        do {
            return try decodeDesktopWireEnvelopeV1(output.stdout)
        } catch let error as DesktopWireError {
            throw error
        } catch {
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

    private static func defaultEnvironment() -> [String: String] {
        [
            "HOME": FileManager.default.homeDirectoryForCurrentUser.path,
            "LANG": "en_US_POSIX",
            "LC_ALL": "en_US_POSIX",
            "PATH": "/usr/bin:/bin",
        ]
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

		do {
			if let snapshotStore {
				try snapshotStore.write(AppGroupDesktopSnapshotV1(envelope: envelope))
			}
			latestSnapshot = envelope
			state = .ready(envelope)
		} catch {
			state = .degraded(previous: latestSnapshot, issue: .storageUnavailable)
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
