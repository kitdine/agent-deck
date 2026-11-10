import CryptoKit
import Darwin
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

	func testScanEventStreamRequiresOrderedProgressAndTerminalResult() throws {
		let bytes = [UInt8](successfulScanStream())
		let slices = bytes.split(separator: UInt8(0x0A), omittingEmptySubsequences: true)
		let lines = slices.map { Data($0) }
		let progress = try decodeDesktopScanEventStream(lines)
		XCTAssertEqual(progress.map(\.stage), [.waiting, .checking, .completed])
		XCTAssertEqual(progress.map(\.sequence), [0, 1, 2])

		var malformed = lines
		malformed.insert(Data(#"{"schema_version":1,"command":"scan","type":"unknown","partial":false}"#.utf8), at: 1)
		XCTAssertThrowsError(try decodeDesktopScanEventStream(malformed)) { error in
			XCTAssertEqual(error as? HelperExecutionError, .malformedOutput)
		}
	}

    func testUsesOnlyEmbeddedHelperAndArrayArguments() async throws {
        let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
        let bundleURL = try makeEmbeddedHelperBundle(in: temporaryDirectory)
        let process = RecordingHelperProcess(
			behaviors: [
				.output(HelperProcessOutput(exitStatus: 0, stdout: successfulScanStream())),
				.output(HelperProcessOutput(exitStatus: 0, stdout: try desktopFixtureData("snapshot-complete.json"))),
			]
        )
        let runner = EmbeddedHelperRunner(
            appBundleURL: bundleURL,
            process: process,
            environment: ["HOME": "/tmp/isolated-home", "PATH": "/tmp/untrusted-path"],
            timeout: .seconds(1)
        )

		_ = try await runner.snapshot()
		let allInvocations = await process.recordedInvocations(includingStateLocation: true)
        XCTAssertEqual(allInvocations.first?.arguments, ["desktop", "state-location", "--format", "json"])
        XCTAssertEqual(allInvocations.count, 3)
        let invocations = await process.recordedInvocations()
		XCTAssertEqual(invocations.count, 2)
		XCTAssertTrue(invocations.allSatisfy {
			$0.executableURL.path == bundleURL.appendingPathComponent("Contents/Helpers/agentdeck").path
		})
		XCTAssertEqual(invocations[0].arguments, [
			"--format", "ndjson", "scan",
		])
		XCTAssertEqual(
			invocations[1].arguments,
			[
					"--format", "json", "desktop", "snapshot", "--wire-version", "1", "--recent-limit", "5", "--stream",
			]
		)
		XCTAssertEqual(invocations[0].timeout, EmbeddedHelperRunner.indexRefreshTimeout)
		XCTAssertEqual(invocations[1].timeout, .seconds(1))
		XCTAssertTrue(invocations.allSatisfy { $0.environment["PATH"] == "/tmp/untrusted-path" })
	}

	func testSnapshotWaitsForItsCompletedScanWorkerToReleaseTheLock() async throws {
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let bundleURL = try makeEmbeddedHelperBundle(in: temporaryDirectory)
		let home = temporaryDirectory.appendingPathComponent("home", isDirectory: true)
		let state = home.appendingPathComponent(".agentdeck", isDirectory: true)
		try FileManager.default.createDirectory(at: state, withIntermediateDirectories: true)
		let lock = state.appendingPathComponent("scan.lock")
		try Data("v1:123:fixture".utf8).write(to: lock)
		let process = ScanLockReleasingProcess(lockURL: lock, snapshot: try desktopFixtureData("snapshot-complete.json"))
		let runner = EmbeddedHelperRunner(
			appBundleURL: bundleURL,
			process: process,
			environment: ["HOME": home.path, "PATH": "/usr/bin:/bin"],
			timeout: .seconds(2)
		)

		_ = try await runner.snapshot()

		let snapshotObservedLock = await process.snapshotObservedLock()
		XCTAssertFalse(snapshotObservedLock, "desktop snapshot ran while its own completed scan worker still held scan.lock")
	}

	func testSnapshotStillReportsAScanLockHeldBeyondTheReleaseGrace() async throws {
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let bundleURL = try makeEmbeddedHelperBundle(in: temporaryDirectory)
		let home = temporaryDirectory.appendingPathComponent("home", isDirectory: true)
		let state = home.appendingPathComponent(".agentdeck", isDirectory: true)
		try FileManager.default.createDirectory(at: state, withIntermediateDirectories: true)
		let lock = state.appendingPathComponent("scan.lock")
		try Data("v1:456:external".utf8).write(to: lock)
		let process = ScanLockReleasingProcess(
			lockURL: lock,
			snapshot: try desktopFixtureData("snapshot-complete.json"),
			releaseDelay: nil
		)
		let runner = EmbeddedHelperRunner(
			appBundleURL: bundleURL,
			process: process,
			environment: ["HOME": home.path, "PATH": "/usr/bin:/bin"],
			timeout: .seconds(4)
		)

		_ = try await runner.snapshot()

		let snapshotObservedLock = await process.snapshotObservedLock()
		XCTAssertTrue(snapshotObservedLock, "a continuing scan lock must remain visible after the bounded grace period")
	}

	func testDefaultEnvironmentFindsSupportedClientInstallLocations() async throws {
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		defer { try? FileManager.default.removeItem(at: temporaryDirectory) }
		let bundleURL = try makeEmbeddedHelperBundle(in: temporaryDirectory)
		let process = RecordingHelperProcess(behaviors: [
			.output(HelperProcessOutput(exitStatus: 0, stdout: successfulScanStream())),
			.output(HelperProcessOutput(exitStatus: 0, stdout: try desktopFixtureData("snapshot-complete.json"))),
		])
		let runner = EmbeddedHelperRunner(appBundleURL: bundleURL, process: process)

		_ = try await runner.snapshot()

		let invocations = await process.recordedInvocations()
		XCTAssertTrue(invocations.allSatisfy {
			$0.environment["PATH"] == "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"
		})
	}

	func testScanFailureDoesNotPublishAReplacementSnapshot() async throws {
		let process = RecordingHelperProcess(behaviors: [
			.output(HelperProcessOutput(exitStatus: 7, stdout: Data(), stderr: Data("scan failed".utf8))),
			.output(HelperProcessOutput(exitStatus: 0, stdout: try desktopFixtureData("snapshot-complete.json"))),
		])
		let runner = try makeRunner(process: process)

		await assertHelperError(runner, equals: .nonZeroExit(7))
		let invocations = await process.recordedInvocations()
		XCTAssertEqual(invocations.count, 1)
	}

    func testPartialSnapshotRemainsUsable() async throws {
        let runner = try makeSnapshotRunner(
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
        let runner = try makeSnapshotRunner(behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: unsupported)))

        await assertDesktopWireError(runner, equals: .unsupportedWireVersion(2))
    }

    func testHelperFailureDoesNotExposeStderr() async throws {
        let runner = try makeSnapshotRunner(
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
		let timeoutRunner = try makeSnapshotRunner(behavior: .helperError(.timedOut))
        await assertHelperError(timeoutRunner, equals: .timedOut)

		let cancelledRunner = try makeSnapshotRunner(behavior: .cancellation)
        await assertHelperError(cancelledRunner, equals: .cancelled)

		let limitedRunner = try makeSnapshotRunner(
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

	func testQuotaRefreshUsesBackgroundAndManualCommandShapes() async throws {
		let payload = Data(#"{"data":{"gate_reasons":{"codex":null,"claude":"not_official"},"alerts":[{"id":"qa1.abc","kind":"threshold","client":"codex","label":"GPT-5 Codex","window_minutes":300,"used_percent":80,"threshold":75},{"id":"qa1.def","kind":"reset","client":"claude","window_minutes":null,"used_percent":3}]}}"#.utf8)
		let process = RecordingHelperProcess(behaviors: [
			.output(HelperProcessOutput(exitStatus: 0, stdout: payload)),
			.output(HelperProcessOutput(exitStatus: 0, stdout: payload)),
		])
		let runner = try makeRunner(process: process)

		let alerts = await runner.refreshQuota(manual: false)
		_ = await runner.refreshQuota(manual: true)

		XCTAssertEqual(alerts, [
			DesktopQuotaAlertV1(id: "qa1.abc", kind: .threshold, client: "codex", label: "GPT-5 Codex", windowMinutes: 300, usedPercent: 80, threshold: 75),
			DesktopQuotaAlertV1(id: "qa1.def", kind: .reset, client: "claude", usedPercent: 3),
		])

		let invocations = await process.recordedInvocations()
		XCTAssertEqual(invocations.map(\.arguments), [
			["--format", "json", "desktop", "quota-refresh"],
			["--format", "json", "desktop", "quota-refresh", "--manual"],
		])
		XCTAssertTrue(invocations.allSatisfy { $0.environment["HOME"] == "/tmp/isolated-home" })
		XCTAssertTrue(invocations.allSatisfy { $0.timeout == EmbeddedHelperRunner.quotaRefreshTimeout })
	}

	func testQuotaAlertAcknowledgementPassesEveryIDAndSkipsAnEmptyBatch() async throws {
		let payload = Data(#"{"data":{"acknowledged":2}}"#.utf8)
		let process = RecordingHelperProcess(behaviors: [.output(HelperProcessOutput(exitStatus: 0, stdout: payload))])
		let runner = try makeRunner(process: process)

		await runner.acknowledgeQuotaAlerts(ids: [])
		await runner.acknowledgeQuotaAlerts(ids: ["qa1.abc", "qa1.def"])

		let invocations = await process.recordedInvocations()
		XCTAssertEqual(invocations.map(\.arguments), [
			["--format", "json", "desktop", "quota-alerts", "ack", "--id", "qa1.abc", "--id", "qa1.def"],
		])
	}

	func testQuotaRefreshWithUndecodableOutputOffersNoAlerts() async throws {
		let process = RecordingHelperProcess(behaviors: [.output(HelperProcessOutput(exitStatus: 0, stdout: Data("not json".utf8)))])
		let runner = try makeRunner(process: process)

		let alerts = await runner.refreshQuota(manual: false)

		XCTAssertEqual(alerts, [])
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
		await waitUntil { refresher.calls == 1 }
		let targets = await transport.targets()
		XCTAssertEqual(targets, [target, target])
		XCTAssertEqual(refresher.calls, 1)
		controller.dismiss()
		XCTAssertEqual(controller.state, .idle)
	}

	// An empty state root has nothing to restore or fall back to: the restore
	// fails without launching the helper, so a first import is not delayed.
	// Committed facts without a serving file still go to the helper.
	func testRestoreOfAnEmptyStateRootQueriesLocationButDoesNotReadSnapshot() async throws {
		let home = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		try FileManager.default.createDirectory(at: home, withIntermediateDirectories: true)
		addTeardownBlock { try? FileManager.default.removeItem(at: home) }
		let bundleURL = try makeEmbeddedHelperBundle(in: home.appendingPathComponent("bundle"))
		let process = RecordingHelperProcess(behaviors: [.output(HelperProcessOutput(exitStatus: 1, stdout: Data()))])
		let runner = EmbeddedHelperRunner(appBundleURL: bundleURL, process: process,
			environment: ["HOME": home.path], timeout: .milliseconds(50))
		do {
			_ = try await runner.readServingSnapshot()
			XCTFail("an empty state root restored a snapshot")
		} catch {
			XCTAssertEqual(error as? HelperExecutionError, .nonZeroExit(1))
		}
		let none = await process.recordedInvocations()
		XCTAssertTrue(none.isEmpty)
        let queries = await process.recordedInvocations(includingStateLocation: true)
        XCTAssertEqual(queries.count, 1)
        XCTAssertEqual(queries[0].arguments, ["desktop", "state-location", "--format", "json"])
		let stateRoot = home.appendingPathComponent(".agentdeck")
		try FileManager.default.createDirectory(at: stateRoot, withIntermediateDirectories: true)
		XCTAssertTrue(FileManager.default.createFile(atPath: stateRoot.appendingPathComponent("agentdeck.sqlite3").path, contents: Data()))
		_ = try? await runner.readServingSnapshot()
		let launched = await process.recordedInvocations()
		XCTAssertEqual(launched.count, 1, "committed facts without a serving file still reach the helper")
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

	private func makeSnapshotRunner(behavior: TestHelperBehavior) throws -> EmbeddedHelperRunner {
		try makeRunner(process: RecordingHelperProcess(behaviors: [
			.output(HelperProcessOutput(exitStatus: 0, stdout: successfulScanStream())),
			behavior,
		]))
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

private func successfulScanStream() -> Data {
	let domain = #"{"state":"pending","committed":0,"total":0,"skipped":0}"#
	let completed = #"{"state":"completed","committed":0,"total":0,"skipped":0}"#
	return Data(([
		#"{"schema_version":1,"command":"scan","generated_at":"2026-08-20T12:00:00Z","type":"progress","scope":"both","data":{"sequence":0,"stage":"waiting","usage":\#(domain),"session":\#(domain)},"partial":false}"#,
		#"{"schema_version":1,"command":"scan","generated_at":"2026-08-20T12:00:01Z","type":"progress","scope":"both","data":{"sequence":1,"stage":"checking","usage":\#(domain),"session":\#(domain)},"partial":false}"#,
		#"{"schema_version":1,"command":"scan","generated_at":"2026-08-20T12:00:02Z","type":"progress","scope":"both","data":{"sequence":2,"stage":"completed","usage":\#(completed),"session":\#(completed)},"partial":false}"#,
		#"{"schema_version":1,"command":"scan","generated_at":"2026-08-20T12:00:02Z","type":"result","scope":"both","data":{"scope":"both","usage":{"state":"completed","duration_ms":1},"session":{"state":"completed","duration_ms":1}},"partial":false}"#,
	].joined(separator: "\n") + "\n").utf8)
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

private actor ScanLockReleasingProcess: EmbeddedHelperProcess {
	private let lockURL: URL
	private let snapshot: Data
	private let releaseDelay: Duration?
	private var calls = 0
	private var sawLockDuringSnapshot = false

	init(lockURL: URL, snapshot: Data, releaseDelay: Duration? = .milliseconds(50)) {
		self.lockURL = lockURL
		self.snapshot = snapshot
		self.releaseDelay = releaseDelay
	}

	func run(executableURL _: URL, arguments: [String], environment _: [String: String], timeout _: Duration) async throws -> HelperProcessOutput {
        if arguments.contains("state-location") { return try fixtureStateLocation(root: lockURL.deletingLastPathComponent().path) }
		return HelperProcessOutput(exitStatus: 1, stdout: Data(), stderr: Data("unexpected non-stream invocation".utf8))
	}

	func runLines(
		executableURL _: URL,
		arguments _: [String],
		environment _: [String: String],
		timeout _: Duration,
		maximumLineBytes _: Int,
		maximumLines _: Int
	) async throws -> HelperProcessLinesOutput {
		calls += 1
		if calls == 1 {
			if let releaseDelay {
				let lockURL = lockURL
				Task {
					try? await Task.sleep(for: releaseDelay)
					try? FileManager.default.removeItem(at: lockURL)
				}
			}
			let lines = [UInt8](successfulScanStream()).split(separator: 0x0A).map { Data($0) }
			return HelperProcessLinesOutput(exitStatus: 0, stdoutLines: lines, stdoutBytes: successfulScanStream().count)
		}
		sawLockDuringSnapshot = FileManager.default.fileExists(atPath: lockURL.path)
		let lines = try snapshotChunkLines(snapshot, chunkBytes: 8 * 1024)
		return HelperProcessLinesOutput(exitStatus: 0, stdoutLines: lines, stdoutBytes: lines.reduce(0) { $0 + $1.count })
	}

	func snapshotObservedLock() -> Bool { sawLockDuringSnapshot }
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

private actor NativeLocationSequence {
    var values: [Result<NativeStateLocation, HelperExecutionError>]
    private(set) var calls = 0
    init(_ values: [Result<NativeStateLocation, HelperExecutionError>]) { self.values = values }
    func next() throws -> NativeStateLocation {
        calls += 1
        return try (values.count > 1 ? values.removeFirst() : values[0]).get()
    }
}

extension EmbeddedHelperRunnerTests {
    private func ownershipAccount() throws -> URL {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: root) }
        return root.resolvingSymlinksInPath()
    }

    private func exclusiveDescriptor(_ account: URL) throws -> Int32 {
        let directory = account.appendingPathComponent("Library/Application Support/AgentDeck")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        let fd = Darwin.open(directory.appendingPathComponent("ownership.lock").path, O_CREAT | O_RDWR | O_CLOEXEC, 0o600)
        XCTAssertGreaterThanOrEqual(fd, 0)
        return fd
    }

    func testOwnershipSessionRechecksUnderLockAndReleasesOnLossAndClose() async throws {
        let account = try ownershipAccount()
        let real = NativeStateLocation(installed: true, stateRoot: account.appendingPathComponent(".agentdeck").path)
        let isolated = NativeStateLocation(installed: false, stateRoot: account.appendingPathComponent("isolated").path)
        let query = NativeLocationSequence([.success(real), .success(real), .success(isolated)])
        let session = NativeOwnershipSession(accountHome: { account })
        let first = try await session.resolve { try await query.next() }
        XCTAssertEqual(first.path, real.stateRoot)
        let count = await query.calls
        XCTAssertEqual(count, 2, "the first ownership observation must be checked under flock")
        let fd = try exclusiveDescriptor(account)
        defer { Darwin.close(fd); session.close() }
        XCTAssertNotEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
        let second = try await session.resolve { try await query.next() }
        XCTAssertEqual(second.path, isolated.stateRoot)
        XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
        flock(fd, LOCK_UN)
        _ = try await session.resolve { real }
        XCTAssertNotEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
        session.close()
        XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
        do { _ = try await session.resolve { real }; XCTFail("closed session reopened") } catch {}
    }

    func testOwnershipTransferBeforeLockConfirmationDoesNotRetainLock() async throws {
        let account = try ownershipAccount()
        let query = NativeLocationSequence([
            .success(NativeStateLocation(installed: true, stateRoot: account.appendingPathComponent(".agentdeck").path)),
            .success(NativeStateLocation(installed: false, stateRoot: account.appendingPathComponent("isolation").path))
        ])
        let session = NativeOwnershipSession(accountHome: { account })
        defer { session.close() }
        let path = try await session.resolve { try await query.next() }
        XCTAssertEqual(path.lastPathComponent, "isolation")
        let fd = try exclusiveDescriptor(account); defer { Darwin.close(fd) }
        XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
    }

    func testInstalledHelperUsingIsolatedStateDoesNotTakeOwnershipLock() async throws {
        let account = try ownershipAccount()
        let session = NativeOwnershipSession(accountHome: { account })
        defer { session.close() }
        _ = try await session.resolve { NativeStateLocation(installed: true, stateRoot: account.appendingPathComponent("isolation").path) }
        let fd = try exclusiveDescriptor(account); defer { Darwin.close(fd) }
        XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
    }

    func testOwnershipQueryFailureReleasesExistingSession() async throws {
        let account = try ownershipAccount()
        let session = NativeOwnershipSession(accountHome: { account })
        defer { session.close() }
        _ = try await session.resolve { NativeStateLocation(installed: true, stateRoot: account.appendingPathComponent(".agentdeck").path) }
        do { _ = try await session.resolve { throw HelperExecutionError.malformedOutput }; XCTFail("query failure accepted") } catch {}
        let fd = try exclusiveDescriptor(account); defer { Darwin.close(fd) }
        XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
    }

    func testOwnershipLockTimeoutIsBoundedAndNeverQueriesUnderExclusiveLock() async throws {
        let account = try ownershipAccount()
        let fd = try exclusiveDescriptor(account); defer { Darwin.close(fd) }
        XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
        let query = NativeLocationSequence([.success(NativeStateLocation(installed: true, stateRoot: account.appendingPathComponent(".agentdeck").path))])
        let session = NativeOwnershipSession(accountHome: { account }, lockWait: .milliseconds(30))
        defer { session.close() }
        do { _ = try await session.resolve { try await query.next() }; XCTFail("exclusive lock ignored") }
        catch { XCTAssertEqual(error as? HelperExecutionError, .timedOut) }
        let calls = await query.calls
        XCTAssertEqual(calls, 1)
    }

    func testStateLocationRejectsInvalidWireBeforeAnyDirectProbe() throws {
        for payload in [
            #"{"schema_version":2,"command":"desktop.state-location","partial":false,"data":{"installed":true,"state_root":"/tmp/x"}}"#,
            #"{"schema_version":1,"command":"desktop.snapshot","partial":false,"data":{"installed":true,"state_root":"/tmp/x"}}"#,
            #"{"schema_version":1,"command":"desktop.state-location","partial":false,"data":{"installed":true,"state_root":"relative"}}"#,
            #"{"schema_version":1,"command":"desktop.state-location","partial":true,"data":{"installed":true,"state_root":"/tmp/x"}}"#
        ] { XCTAssertThrowsError(try NativeStateLocation.decode(Data(payload.utf8))) }
    }

    func testNativeHelperAndGoReceiptDriverShareOwnershipLock() async throws {
        let env = ProcessInfo.processInfo.environment
        let path = try XCTUnwrap(env["AGENTDECK_NATIVE_FIXTURE_ROOT"], "native harness must supply its isolated fixture; no production fallback")
        let root = URL(fileURLWithPath: path)
        guard root.path.hasPrefix("/private/tmp/agentdeck-macos-xctest.") else {
            throw CocoaError(.fileReadNoPermission)
        }
        let account = root.appendingPathComponent("account")
        let process = FoundationEmbeddedHelperProcess()
        let fixtureEnvironment = ["HOME": account.path, "PATH": "/usr/bin:/bin", "AGENTDECK_ACCEPTANCE_ACCOUNT_HOME": account.path,
                                  "AGENTDECK_NATIVE_FIXTURE_ROOT": root.path]
        func drive(_ action: String) async throws -> String {
            var environment = fixtureEnvironment
            environment["AGENTDECK_NATIVE_FIXTURE_ACTION"] = action
            let result = try await process.run(executableURL: root.appendingPathComponent("native-driver"),
                arguments: ["-test.run=^TestNativeOwnershipFixture$", "-test.v"], environment: environment, timeout: .seconds(10))
            guard result.exitStatus == 0 else {
                XCTFail(String(decoding: result.stdout + result.stderr, as: UTF8.self))
                throw HelperExecutionError.nonZeroExit(result.exitStatus)
            }
            return String(decoding: result.stdout, as: UTF8.self)
        }
        let registration = try await drive("register")
        XCTAssertTrue(registration.contains(root.appendingPathComponent("AgentDeck.app/Contents/Helpers/agentdeck").path))
        XCTAssertTrue(registration.contains(account.appendingPathComponent("Library/Application Support/AgentDeck/ownership.lock").path))
        let runner = EmbeddedHelperRunner(appBundleURL: root.appendingPathComponent("AgentDeck.app"), process: process,
            environment: fixtureEnvironment, ownershipSession: NativeOwnershipSession(accountHome: { account }))
        defer { runner.closeOwnershipSession() }
        let installed = try await runner.resolveStateRoot()
        XCTAssertEqual(installed.path, account.appendingPathComponent(".agentdeck").path)
        let blocked = try await drive("probe")
        XCTAssertTrue(blocked.contains("NATIVE_LOCK_BUSY"))
        _ = try await drive("delete")
        let isolated = try await runner.resolveStateRoot()
        XCTAssertNotEqual(isolated.path, installed.path)
        let released = try await drive("probe")
        XCTAssertTrue(released.contains("NATIVE_LOCK_ACQUIRED"))
        _ = try await drive("mismatch")
        let mismatch = try await runner.resolveStateRoot()
        XCTAssertNotEqual(mismatch.path, installed.path)
        let uninstalled = try await drive("probe")
        XCTAssertTrue(uninstalled.contains("NATIVE_LOCK_ACQUIRED"))
        _ = try await drive("register")
        _ = try await runner.resolveStateRoot()
        runner.closeOwnershipSession()
        let closed = try await drive("probe")
        XCTAssertTrue(closed.contains("NATIVE_LOCK_ACQUIRED"))
        print("Native ownership fixture: helper=\(try runner.embeddedHelperURL().path) lock=\(account.appendingPathComponent("Library/Application Support/AgentDeck/ownership.lock").path); receipt and lock scenario assertions executed")
    }
}

private actor SuspendedOwnershipQuery {
    private var continuation: CheckedContinuation<NativeStateLocation, Never>?
    var waiting: Bool { continuation != nil }
    func query() async -> NativeStateLocation {
        await withCheckedContinuation { continuation = $0 }
    }
    func finish(_ location: NativeStateLocation) { continuation?.resume(returning: location); continuation = nil }
}

extension EmbeddedHelperRunnerTests {
    func testClosingWhileQueryIsInFlightCannotReacquireOwnership() async throws {
        let account = try ownershipAccount()
        let session = NativeOwnershipSession(accountHome: { account })
        let suspended = SuspendedOwnershipQuery()
        let work = Task { try await session.resolve { await suspended.query() } }
        for _ in 0..<1000 {
            if await suspended.waiting { break }
            await Task.yield()
        }
        let waiting = await suspended.waiting
        XCTAssertTrue(waiting)
        session.close()
        await suspended.finish(NativeStateLocation(installed: true, stateRoot: account.appendingPathComponent(".agentdeck").path))
        do { _ = try await work.value; XCTFail("closed during query but returned a usable path") }
        catch { XCTAssertEqual(error as? HelperExecutionError, .cancelled) }
        let fd = try exclusiveDescriptor(account); defer { Darwin.close(fd) }
        XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
    }
}

extension EmbeddedHelperRunnerTests {
    func testRestoreUsesEachQueriedRootInsteadOfHomeOrCachedPath() async throws {
        let home = try ownershipAccount()
        let bundle = try makeEmbeddedHelperBundle(in: home.appendingPathComponent("bundle"))
        let legacy = home.appendingPathComponent(".agentdeck")
        let empty = home.appendingPathComponent("empty")
        let populated = home.appendingPathComponent("populated")
        for root in [legacy, populated] {
            try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
            try Data().write(to: root.appendingPathComponent("agentdeck.sqlite3"))
        }
        let process = RecordingHelperProcess(behaviors: [.output(HelperProcessOutput(exitStatus: 1, stdout: Data()))],
                                            stateRoots: [empty.path, populated.path])
        let runner = EmbeddedHelperRunner(appBundleURL: bundle, process: process, environment: ["HOME": home.path])
        _ = try? await runner.readServingSnapshot()
        let first = await process.recordedInvocations()
        XCTAssertTrue(first.isEmpty, "HOME has a database but the helper-selected root does not")
        _ = try? await runner.readServingSnapshot()
        let second = await process.recordedInvocations()
        XCTAssertEqual(second.count, 1, "second query points to committed facts; do not cache the first root")
        XCTAssertFalse(second[0].arguments.contains("--state-dir"))
        let all = await process.recordedInvocations(includingStateLocation: true)
        XCTAssertEqual(all.filter { $0.arguments.contains("state-location") }.count, 2)
    }
}

private actor SuspendedOwnershipRecheck {
    let location: NativeStateLocation
    var calls = 0
    private var continuation: CheckedContinuation<NativeStateLocation, Never>?
    var waiting: Bool { continuation != nil }
    init(_ location: NativeStateLocation) { self.location = location }
    func query() async -> NativeStateLocation {
        calls += 1
        if calls == 1 { return location }
        return await withCheckedContinuation { continuation = $0 }
    }
    func finish() { continuation?.resume(returning: location); continuation = nil }
}

extension EmbeddedHelperRunnerTests {
    func testCloseReleasesProvisionalLockBeforeRecheckReturns() async throws {
        let account = try ownershipAccount()
        let session = NativeOwnershipSession(accountHome: { account })
        let suspended = SuspendedOwnershipRecheck(NativeStateLocation(installed: true, stateRoot: account.appendingPathComponent(".agentdeck").path))
        let work = Task { try await session.resolve { await suspended.query() } }
        for _ in 0..<1000 {
            if await suspended.waiting { break }
            await Task.yield()
        }
        let waiting = await suspended.waiting
        XCTAssertTrue(waiting)
        let fd = try exclusiveDescriptor(account); defer { Darwin.close(fd) }
        XCTAssertNotEqual(flock(fd, LOCK_EX | LOCK_NB), 0)
        session.close()
        XCTAssertEqual(flock(fd, LOCK_EX | LOCK_NB), 0, "termination must release even before recheck finishes")
        await suspended.finish()
        do { _ = try await work.value; XCTFail("closed recheck returned a path") }
        catch { XCTAssertEqual(error as? HelperExecutionError, .cancelled) }
    }
}

extension EmbeddedHelperRunnerTests {
    func testPhysicalStatePathMatchesAliasesBeforeFirstDatabaseExists() throws {
        let suffix = "agentdeck-path-check-" + UUID().uuidString + "/.agentdeck"
        let a = try NativeOwnershipSession.physicalPath(URL(fileURLWithPath: "/tmp/" + suffix))
        let b = try NativeOwnershipSession.physicalPath(URL(fileURLWithPath: "/private/tmp/" + suffix))
        XCTAssertEqual(a, b)
    }
}
