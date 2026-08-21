import CryptoKit
import Foundation
import XCTest
@testable import AgentDeckShared

final class EmbeddedHelperRunnerTests: XCTestCase {
	func testHelperErrorLoggingKeepsOnlyAValidatedClassificationCode() {
		let typed = Data(#"{"error":{"code":"state_busy","message":"credential=secret"}}"#.utf8)
		let injected = Data(#"{"error":{"code":"state_busy\nsecret","message":"discarded"}}"#.utf8)

		XCTAssertEqual(desktopHelperErrorCode(stderr: typed), "state_busy")
		XCTAssertNil(desktopHelperErrorCode(stderr: injected))
		XCTAssertNil(desktopHelperErrorCode(stderr: Data("not-json secret".utf8)))
	}

	func testParallelIndexRefreshEnvelopeKeepsPerDomainOutcome() throws {
		let payload = Data(#"{"schema_version":1,"command":"desktop.refresh-indexes","generated_at":"2026-08-20T12:00:00Z","data":{"usage":{"success":true,"duration_ms":120,"changes":{"updated":1}},"sessions":{"success":false,"duration_ms":80,"error_code":"state_busy"}},"warnings":["session_index_refresh_failed"],"partial":true,"error":null}"#.utf8)
		let result = try decodeDesktopIndexRefreshResult(payload)

		XCTAssertEqual(result.usage, DesktopIndexDomainResult(success: true, durationMilliseconds: 120, errorCode: nil))
		XCTAssertEqual(result.sessions, DesktopIndexDomainResult(success: false, durationMilliseconds: 80, errorCode: "state_busy"))
	}

	func testFoundationProcessStreamsPastLegacyTotalCaptureLimit() async throws {
		let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: directory) }
		let payloadURL = directory.appendingPathComponent("stream.ndjson")
		let executableURL = directory.appendingPathComponent("emit")
		var payload = Data()
		for _ in 0 ..< 20 {
			payload.append(Data(repeating: 0x61, count: 48 * 1024))
			payload.append(0x0A)
		}
		try payload.write(to: payloadURL)
		try Data("#!/bin/sh\n/bin/cat \"$1\"\n".utf8).write(to: executableURL)
		try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: executableURL.path)

		let output = try await FoundationEmbeddedHelperProcess().runLines(
			executableURL: executableURL,
			arguments: [payloadURL.path],
			environment: ["PATH": "/usr/bin:/bin"],
			timeout: .seconds(5),
			maximumLineBytes: EmbeddedHelperRunner.maximumStreamLineBytes,
			maximumLines: EmbeddedHelperRunner.maximumStreamLines
		)
		XCTAssertEqual(output.exitStatus, 0)
		XCTAssertEqual(output.stdoutBytes, payload.count)
		XCTAssertEqual(output.stdoutLines.count, 20)
		XCTAssertFalse(output.stdoutLineTruncated)
		XCTAssertTrue(output.stdoutBytes > FoundationEmbeddedHelperProcess.maximumCapturedBytes)
	}

	func testStreamedSnapshotReassemblesAndRejectsIntegrityMismatch() throws {
		let snapshot = try desktopFixtureData("snapshot-complete.json")
		let lines = try snapshotChunkLines(snapshot, chunkBytes: 8 * 1024)
		XCTAssertGreaterThan(lines.count, 1)
		XCTAssertEqual(try decodeDesktopSnapshotStream(lines), snapshot)

		var first = try XCTUnwrap(JSONSerialization.jsonObject(with: lines[0]) as? [String: Any])
		var data = try XCTUnwrap(first["data"] as? [String: Any])
		data["payload"] = Data("corrupt".utf8).base64EncodedString()
		first["data"] = data
		var corrupted = lines
		corrupted[0] = try JSONSerialization.data(withJSONObject: first, options: [.sortedKeys])
		XCTAssertThrowsError(try decodeDesktopSnapshotStream(corrupted)) { error in
			XCTAssertEqual(error as? HelperExecutionError, .malformedOutput)
		}
	}

    func testUsesOnlyEmbeddedHelperAndArrayArguments() async throws {
        let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
        let bundleURL = try makeEmbeddedHelperBundle(in: temporaryDirectory)
        let process = RecordingHelperProcess(
            behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: try desktopFixtureData("snapshot-complete.json")))
        )
        let runner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: process,
            environment: ["HOME": "/tmp/isolated-home", "PATH": "/tmp/untrusted-path"],
            timeout: .seconds(1)
        )

		_ = try await runner.snapshot()
		let invocations = await process.recordedInvocations()
		XCTAssertEqual(invocations.count, 2)
		XCTAssertTrue(invocations.allSatisfy {
			$0.executableURL.path == bundleURL.appendingPathComponent("Contents/Helpers/agentdeck").path
		})
		XCTAssertEqual(invocations[0].arguments, ["--quiet", "--format", "json", "desktop", "refresh-indexes"])
		XCTAssertEqual(
			invocations[1].arguments,
			["--format", "json", "desktop", "snapshot", "--wire-version", "1", "--recent-limit", "5", "--stream"]
		)
		XCTAssertEqual(invocations[0].timeout, EmbeddedHelperRunner.indexRefreshTimeout)
		XCTAssertEqual(invocations[1].timeout, .seconds(1))
		XCTAssertTrue(invocations.allSatisfy { $0.environment["PATH"] == "/tmp/untrusted-path" })
	}

	func testIndexRefreshFailureFallsBackToLastCommittedSnapshot() async throws {
		let process = RecordingHelperProcess(behaviors: [
			.output(HelperProcessOutput(exitStatus: 7, stdout: Data(), stderr: Data("scan failed".utf8))),
			.output(HelperProcessOutput(exitStatus: 0, stdout: try desktopFixtureData("snapshot-complete.json"))),
		])
		let runner = try makeRunner(process: process)

		let envelope = try await runner.snapshot()

		XCTAssertFalse(envelope.partial)
		let invocations = await process.recordedInvocations()
		XCTAssertEqual(invocations.count, 2)
	}

    func testPartialSnapshotRemainsUsable() async throws {
        let runner = try makeRunner(
            behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: try desktopFixtureData("snapshot-partial.json")))
        )

        let envelope = try await runner.snapshot()
        XCTAssertTrue(envelope.partial)
        XCTAssertFalse(envelope.warnings.isEmpty)
    }

    func testUnsupportedWireVersionIsSurfaced() async throws {
        var object = try XCTUnwrap(JSONSerialization.jsonObject(with: desktopFixtureData("snapshot-complete.json")) as? [String: Any])
        var data = try XCTUnwrap(object["data"] as? [String: Any])
        data["wire_version"] = 2
        object["data"] = data
        let unsupported = try JSONSerialization.data(withJSONObject: object)
        let runner = try makeRunner(behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: unsupported)))

        await assertDesktopWireError(runner, equals: .unsupportedWireVersion(2))
    }

    func testHelperFailureDoesNotExposeStderr() async throws {
        let runner = try makeRunner(
            behavior: .output(
                HelperProcessOutput(
                    exitStatus: 23,
                    stdout: Data(),
                    stderr: Data("credential=should-not-escape".utf8)
                )
            )
        )

        await assertHelperError(runner, equals: .nonZeroExit(23))
    }

    func testTimeoutCancellationAndOutputLimitAreClassified() async throws {
        let timeoutRunner = try makeRunner(behavior: .helperError(.timedOut))
        await assertHelperError(timeoutRunner, equals: .timedOut)

        let cancelledRunner = try makeRunner(behavior: .cancellation)
        await assertHelperError(cancelledRunner, equals: .cancelled)

        let limitedRunner = try makeRunner(
            behavior: .output(
                HelperProcessOutput(
                    exitStatus: 0,
                    stdout: Data(),
                    stdoutTruncated: true
                )
            )
        )
        await assertHelperError(limitedRunner, equals: .outputLimitExceeded)
    }

    func testInvalidLimitDoesNotLaunchHelper() async throws {
        let process = RecordingHelperProcess(
            behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: try desktopFixtureData("snapshot-complete.json")))
        )
        let runner = try makeRunner(process: process)

        await assertHelperError(runner, equals: .invalidRecentLimit(0), recentLimit: 0)
        let invocations = await process.recordedInvocations()
        XCTAssertTrue(invocations.isEmpty)
    }

	func testProviderSwitchUsesCanonicalArgumentsAndClassifiesSuccess() async throws {
		let process = RecordingHelperProcess(
			behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: providerUseEnvelope()))
		)
		let runner = try makeRunner(process: process)
		let target = ProviderSwitchTarget(client: "codex", provider: "relay", credential: "work", viaWrapper: true)

		let outcome = await runner.switchProvider(target)
		XCTAssertEqual(outcome, .succeeded)
		let invocations = await process.recordedInvocations()
		let invocation = try XCTUnwrap(invocations.only)
		XCTAssertEqual(invocation.arguments, [
			"--quiet", "--format", "json", "provider", "use", "relay",
			"--client", "codex", "--credential", "work", "--via", "--no-shell-setup",
		])
	}

	func testProviderSwitchClassifiesCanonicalFailureAndDiscardsMessage() async throws {
		let failure = providerUseEnvelope(code: "state_busy", message: "sql: secret storage detail")
		let runner = try makeRunner(
			behavior: .output(HelperProcessOutput(exitStatus: 7, stdout: Data(), stderr: failure))
		)
		let outcome = await runner.switchProvider(ProviderSwitchTarget(client: "claude", provider: "official", credential: nil, viaWrapper: false))
		XCTAssertEqual(outcome, .failed(code: "state_busy"))
		let decoded = try decodeProviderUseEnvelopeV1(failure)
		XCTAssertEqual(decoded.errorCode, "state_busy")
		XCTAssertFalse(String(describing: decoded).contains("secret storage detail"))
	}

	func testProviderSwitchWrongStreamAndOpaqueOutputAreNeverConclusive() async throws {
		let failureOnStdout = try makeRunner(
			behavior: .output(HelperProcessOutput(exitStatus: 7, stdout: providerUseEnvelope(code: "state_busy")))
		)
		let successOnStderr = try makeRunner(
			behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: Data(), stderr: providerUseEnvelope()))
		)
		let opaque = try makeRunner(
			behavior: .output(HelperProcessOutput(exitStatus: 1, stdout: Data(), stderr: Data("not-json secret".utf8)))
		)
		let target = ProviderSwitchTarget(client: "codex", provider: "official", credential: nil, viaWrapper: false)
		let failureOnStdoutOutcome = await failureOnStdout.switchProvider(target)
		let successOnStderrOutcome = await successOnStderr.switchProvider(target)
		let opaqueOutcome = await opaque.switchProvider(target)
		XCTAssertEqual(failureOnStdoutOutcome, .indeterminate)
		XCTAssertEqual(successOnStderrOutcome, .indeterminate)
		XCTAssertEqual(opaqueOutcome, .opaque)
	}

	@MainActor
	func testSwitchControllerIsSingleFlightAndRetryRetainsExactTarget() async throws {
		let envelope = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))
		let refresher = StaticSnapshotRefresher(envelope: envelope)
		let coordinator = DesktopRefreshCoordinator(host: refresher, snapshotStore: nil)
		let transport = BlockingSwitchTransport()
		let controller = SwitchController(transport: transport, refreshCoordinator: coordinator)
		let target = ProviderSwitchTarget(client: "codex", provider: "relay", credential: "work", viaWrapper: true)
		let other = ProviderSwitchTarget(client: "claude", provider: "official", credential: nil, viaWrapper: false)

		XCTAssertTrue(controller.start(target))
		XCTAssertFalse(controller.start(other))
		await transport.waitForInvocationCount(1)
		await transport.complete(.failed(code: "state_busy"))
		await waitUntil { controller.state == .failed(target, code: "state_busy") }

		XCTAssertTrue(controller.retry())
		XCTAssertEqual(controller.state, .inFlight(target))
		XCTAssertFalse(controller.start(other))
		await transport.waitForInvocationCount(2)
		await transport.complete(.indeterminate)
		await waitUntil { controller.state == .indeterminate(target) }
		let targets = await transport.targets()
		XCTAssertEqual(targets, [target, target])
		XCTAssertEqual(refresher.calls, 1)
		controller.dismiss()
		XCTAssertEqual(controller.state, .idle)
	}

	private func makeRunner(
		behavior: TestHelperBehavior? = nil,
		process: (any EmbeddedHelperProcess)? = nil
	) throws -> EmbeddedHelperRunner {
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		let bundleURL = try makeEmbeddedHelperBundle(in: temporaryDirectory)
		let helperProcess: any EmbeddedHelperProcess
		if let process {
			helperProcess = process
		} else {
			helperProcess = RecordingHelperProcess(behavior: try XCTUnwrap(behavior))
		}
		addTeardownBlock {
			try? FileManager.default.removeItem(at: temporaryDirectory)
		}
		return EmbeddedHelperRunner(
			appBundleURL: bundleURL,
			process: helperProcess,
            environment: ["HOME": "/tmp/isolated-home"],
            timeout: .milliseconds(50)
        )
    }

    private func assertDesktopWireError(
        _ runner: EmbeddedHelperRunner,
        equals expected: DesktopWireError,
        recentLimit: Int = EmbeddedHelperRunner.defaultRecentLimit
    ) async {
        do {
            _ = try await runner.snapshot(recentLimit: recentLimit)
            XCTFail("snapshot unexpectedly succeeded")
        } catch let error as DesktopWireError {
            XCTAssertEqual(error, expected)
        } catch {
            XCTFail("unexpected error: \(error)")
        }
    }

    private func assertHelperError(
        _ runner: EmbeddedHelperRunner,
        equals expected: HelperExecutionError,
        recentLimit: Int = EmbeddedHelperRunner.defaultRecentLimit
    ) async {
        do {
            _ = try await runner.snapshot(recentLimit: recentLimit)
            XCTFail("snapshot unexpectedly succeeded")
        } catch let error as HelperExecutionError {
            XCTAssertEqual(error, expected)
        } catch {
            XCTFail("unexpected error: \(error)")
        }
    }
}

private func snapshotChunkLines(_ payload: Data, chunkBytes: Int) throws -> [Data] {
	let count = max(1, (payload.count + chunkBytes - 1) / chunkBytes)
	let digest = SHA256.hash(data: payload).map { String(format: "%02x", $0) }.joined()
	return try (0 ..< count).map { index in
		let start = index * chunkBytes
		let end = min(payload.count, start + chunkBytes)
		let chunk = Data(payload[start ..< end])
		return try JSONSerialization.data(withJSONObject: [
			"schema_version": 1,
			"command": "desktop.snapshot.chunk",
			"generated_at": "2026-08-20T12:00:00Z",
			"data": [
				"index": index,
				"count": count,
				"total_bytes": payload.count,
				"sha256": digest,
				"payload": chunk.base64EncodedString(),
			],
			"warnings": [],
			"partial": false,
		], options: [.sortedKeys])
	}
}

private func providerUseEnvelope(code: String? = nil, message: String = "discarded") -> Data {
	let error: String
	if let code {
		error = #", "error": {"code":"\#(code)","message":"\#(message)"}"#
	} else {
		error = #", "error": null"#
	}
	return Data(
		(#"{"schema_version":1,"command":"provider.use","generated_at":"2026-08-13T10:00:00Z","data":null,"warnings":[],"partial":false"# + error + "}").utf8
	)
}

@MainActor
private final class StaticSnapshotRefresher: DesktopSnapshotRefreshing {
	let envelope: DesktopWireEnvelopeV1
	private(set) var calls = 0

	init(envelope: DesktopWireEnvelopeV1) {
		self.envelope = envelope
	}

	func refresh(recentLimit: Int) async throws -> DesktopWireEnvelopeV1 {
		calls += 1
		return envelope
	}
}

private actor BlockingSwitchTransport: ProviderSwitching {
	private var invocations = [ProviderSwitchTarget]()
	private var continuations = [CheckedContinuation<ProviderSwitchTransportOutcome, Never>]()

	func switchProvider(_ target: ProviderSwitchTarget) async -> ProviderSwitchTransportOutcome {
		invocations.append(target)
		return await withCheckedContinuation { continuation in
			continuations.append(continuation)
		}
	}

	func complete(_ outcome: ProviderSwitchTransportOutcome) {
		continuations.removeFirst().resume(returning: outcome)
	}

	func waitForInvocationCount(_ count: Int) async {
		while invocations.count < count {
			await Task.yield()
		}
	}

	func targets() -> [ProviderSwitchTarget] {
		invocations
	}
}

@MainActor
private func waitUntil(_ predicate: @escaping @MainActor () -> Bool) async {
	for _ in 0 ..< 1000 {
		if predicate() {
			return
		}
		await Task.yield()
	}
}

private extension Array {
    var only: Element? {
        count == 1 ? first : nil
    }
}
