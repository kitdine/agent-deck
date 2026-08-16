import Foundation
import XCTest
@testable import AgentDeckShared

final class EmbeddedHelperRunnerTests: XCTestCase {
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
        let invocation = try XCTUnwrap(invocations.only)
        XCTAssertEqual(
            invocation.executableURL.path,
            bundleURL.appendingPathComponent("Contents/Helpers/agentdeck").path
        )
        XCTAssertEqual(
            invocation.arguments,
            ["--format", "json", "desktop", "snapshot", "--wire-version", "1", "--recent-limit", "5"]
        )
        XCTAssertEqual(invocation.environment["PATH"], "/tmp/untrusted-path")
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

private extension Array {
    var only: Element? {
        count == 1 ? first : nil
    }
}
