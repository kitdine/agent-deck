import Foundation
import XCTest
@testable import AgentDeckShared

/// The Settings bridge to the session-origin CLI contract. Payloads follow the
/// Go JSON tags of cmd/agentdeck/session_origin.go and internal/sessionorigin;
/// the hosted end-to-end test in AgentDeckAppTests runs the real helper.
final class SessionOriginTransportTests: XCTestCase {
	private let target = DesktopOriginTargetV1(client: "codex", sessionID: "s-1")

	func testCommandsPassIdentityAsValuesAndCarryTheRequest() async throws {
		XCTAssertEqual(SessionOriginCommand.readDefaultScope(), ["--format", "json", "session", "scope", "get"])
		XCTAssertEqual(
			SessionOriginCommand.saveDefaultScope(.auxiliary, expectedRevision: 2, requestID: "req-a"),
			["--format", "json", "session", "scope", "set", "auxiliary", "--expected-revision", "2", "--request-id", "req-a"]
		)
		// An ID that begins with "-" or equals the list subcommand stays one value.
		let odd = DesktopOriginTargetV1(client: "claude", sessionID: "-list")
		XCTAssertEqual(
			SessionOriginCommand.setOverride(odd, value: "work", expectedRevision: 0, requestID: "req-b"),
			["--format", "json", "session", "classify", "--id=-list", "--client=claude", "--set", "work", "--expected-revision", "0", "--request-id", "req-b"]
		)
		XCTAssertEqual(
			SessionOriginCommand.clearOverride(target, expectedRevision: 4, requestID: "req-c"),
			["--format", "json", "session", "classify", "--id=s-1", "--client=codex", "--clear", "--expected-revision", "4", "--request-id", "req-c"]
		)
		XCTAssertEqual(SessionOriginCommand.readReceipt(requestID: "req-c"), ["--format", "json", "session", "classify", "--request", "req-c"])
		XCTAssertEqual(SessionOriginCommand.listOverrides(page: 2, limit: 20), ["--format", "json", "session", "classify", "list", "--page", "2", "--limit", "20"])
		XCTAssertEqual(SessionOriginCommand.readOriginSourceStatus(), ["--format", "json", "session", "origin-source", "status"])

		let process = RecordingHelperProcess(behavior: .output(HelperProcessOutput(exitStatus: 0, stdout: envelope("session.scope.get", #"{"confirmed":true,"value":"all","revision":3,"reason":null}"#))))
		let runner = try makeRunner(process: process)
		let read = await runner.readDefaultScope()
		XCTAssertEqual(read, .value(DesktopSessionScopeDefaultV1(confirmed: true, value: .all, revision: 3, reason: nil)))
		let invocations = await process.recordedInvocations()
		let invocation = try XCTUnwrap(invocations.first)
		XCTAssertEqual(Array(invocation.arguments.prefix(2)), ["--state-dir", "/tmp/isolated-home/.agentdeck"], "the explicit isolated state root, never an implied one")
		XCTAssertEqual(Array(invocation.arguments.dropFirst(2)), SessionOriginCommand.readDefaultScope())
	}

	func testCommittedReceiptsDecodeTheGoShapesAndMustNameTheRequestedOperation() {
		let set = run(envelope("session.classify", #"{"request_id":"req-1","kind":"override_set","target":{"client":"codex","session_id":"s-1"},"outcome":"saved","value":"auxiliary","revision":3,"replayed":false,"publication_pending":true}"#))
		XCTAssertEqual(
			SessionOriginDecoding.write(set, command: "session.classify", kind: .overrideSet, requestID: "req-1", target: target),
			.committed(DesktopOriginReceiptV1(requestID: "req-1", kind: .overrideSet, target: target, outcome: .saved, value: "auxiliary", revision: 3, publicationPending: true))
		)
		let clear = run(envelope("session.classify", #"{"request_id":"req-2","kind":"override_clear","target":{"client":"codex","session_id":"s-1"},"outcome":"no_change","value":null,"revision":5,"replayed":true,"override":null,"automatic":null,"effective":null,"classification_available":false,"classification_reason":"not_queried","publication_pending":false}"#))
		XCTAssertEqual(
			SessionOriginDecoding.write(clear, command: "session.classify", kind: .overrideClear, requestID: "req-2", target: target),
			.committed(DesktopOriginReceiptV1(requestID: "req-2", kind: .overrideClear, target: target, outcome: .noChange, value: nil, revision: 5, replayed: true, classificationReason: "not_queried"))
		)
		let scope = run(envelope("session.scope.set", #"{"request_id":"req-3","kind":"scope_set","outcome":"saved","value":"all","revision":1,"replayed":false,"warnings":["origin_permissions_not_secured"],"publication_pending":true}"#))
		guard case let .committed(receipt) = SessionOriginDecoding.write(scope, command: "session.scope.set", kind: .scopeSet, requestID: "req-3", target: nil) else {
			return XCTFail("a scope receipt decodes")
		}
		XCTAssertTrue(receipt.permissionsNotSecured, "saved with a permission warning is still saved")

		// A receipt for another request, kind or target is not this request's
		// result, and a command name from elsewhere is not this command's.
		XCTAssertEqual(SessionOriginDecoding.write(set, command: "session.classify", kind: .overrideSet, requestID: "req-other", target: target), .unknown)
		XCTAssertEqual(SessionOriginDecoding.write(set, command: "session.classify", kind: .overrideClear, requestID: "req-1", target: target), .unknown)
		XCTAssertEqual(SessionOriginDecoding.write(set, command: "session.classify", kind: .overrideSet, requestID: "req-1", target: DesktopOriginTargetV1(client: "claude", sessionID: "s-1")), .unknown)
		XCTAssertEqual(SessionOriginDecoding.write(set, command: "session.scope.set", kind: .overrideSet, requestID: "req-1", target: target), .unknown)
	}

	func testOnlyProvenPreCommitFailuresAreNotSavedAndEverythingElseIsUnknown() async throws {
		let expectations: [(String, DesktopOriginWriteOutcome)] = [
			(#"{"error":{"code":"origin_conflict","message":"m","details":{"current_revision":5}}}"#, .conflict(currentRevision: 5)),
			(#"{"error":{"code":"invalid_request","message":"m"}}"#, .invalidRequest),
			(#"{"error":{"code":"origin_write_failed","message":"m"}}"#, .notSaved(.writeFailed)),
			(#"{"error":{"code":"origin_identity_unavailable","message":"m"}}"#, .notSaved(.identityUnavailable)),
			(#"{"error":{"code":"session_not_found","message":"m"}}"#, .notSaved(.notFound)),
			(#"{"error":{"code":"state_busy","message":"m","details":{"resource":"state"}}}"#, .notSaved(.busy)),
			(#"{"error":{"code":"origin_write_unknown","message":"m","details":{"request_id":"req-1"}}}"#, .unknown),
			(#"{"error":{"code":"runtime_error","message":"m"}}"#, .unknown),
			(#"{"error":{"code":"a_future_code","message":"m"}}"#, .unknown),
			("not json", .unknown),
		]
		for (stderr, expected) in expectations {
			let runner = try makeRunner(process: RecordingHelperProcess(behavior: .output(HelperProcessOutput(exitStatus: 1, stdout: Data(), stderr: Data(stderr.utf8)))))
			let outcome = await runner.setOverride(target, value: "work", expectedRevision: 0, requestID: "req-1")
			XCTAssertEqual(outcome, expected, stderr)
		}
		for behavior: TestHelperBehavior in [.helperError(.timedOut), .helperError(.launchFailed), .cancellation,
			.output(HelperProcessOutput(exitStatus: 0, stdout: Data("{".utf8))),
			.output(HelperProcessOutput(exitStatus: 0, stdout: Data(), stdoutTruncated: true))] {
			let runner = try makeRunner(process: RecordingHelperProcess(behavior: behavior))
			let outcome = await runner.clearOverride(target, expectedRevision: 1, requestID: "req-1")
			XCTAssertEqual(outcome, .unknown, "\(behavior): a started helper may have committed")
		}
		// Only a helper that is not there proves nothing ran.
		let missing = EmbeddedHelperRunner(appBundleURL: FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString),
			process: RecordingHelperProcess(behavior: .helperError(.timedOut)), environment: ["HOME": "/tmp/isolated-home"])
		let notStarted = await missing.saveDefaultScope(.work, expectedRevision: 0, requestID: "req-1")
		XCTAssertEqual(notStarted, .notSaved(.rejected))
	}

	func testReceiptReadBackKeepsUnknownDistinctFromUnreadable() {
		XCTAssertEqual(SessionOriginDecoding.receipt(run(envelope("session.classify", #"{"request_id":"req-9","outcome":"unknown"}"#)), requestID: "req-9"), .unknown)
		XCTAssertEqual(
			SessionOriginDecoding.receipt(run(envelope("session.classify", #"{"request_id":"req-9","kind":"scope_set","outcome":"saved","value":"work","revision":2,"replayed":false}"#)), requestID: "req-9"),
			.known(DesktopOriginReceiptV1(requestID: "req-9", kind: .scopeSet, outcome: .saved, value: "work", revision: 2))
		)
		XCTAssertEqual(SessionOriginDecoding.receipt(.failed(code: "runtime_error", currentRevision: nil), requestID: "req-9"), .unavailable)
		XCTAssertEqual(SessionOriginDecoding.receipt(.indeterminate, requestID: "req-9"), .unavailable)
		XCTAssertEqual(
			SessionOriginDecoding.receipt(run(envelope("session.classify", #"{"request_id":"req-other","kind":"scope_set","outcome":"saved","value":"work","revision":2,"replayed":false}"#)), requestID: "req-9"),
			.unavailable, "a receipt for another request is never this one's"
		)
	}

	func testDefaultScopeIsConfirmedOnlyByASuccessfulReadOfAKnownValue() {
		XCTAssertEqual(
			SessionOriginDecoding.defaultScope(run(envelope("session.scope.get", #"{"confirmed":true,"value":"work","revision":0,"reason":null}"#))),
			.value(DesktopSessionScopeDefaultV1(confirmed: true, value: .work, revision: 0, reason: nil))
		)
		for data in [#"{"confirmed":true,"value":"everything","revision":4,"reason":null}"#, #"{"confirmed":false,"value":null,"revision":null,"reason":"read_failed"}"#,
			#"{"confirmed":true,"value":"all","revision":null,"reason":null}"#] {
			XCTAssertEqual(SessionOriginDecoding.defaultScope(run(envelope("session.scope.get", data))), .unavailable, data)
		}
		XCTAssertEqual(SessionOriginDecoding.defaultScope(.failed(code: "runtime_error", currentRevision: nil)), .unavailable, "an unreadable default is never Work")
	}

	func testLookupSeparatesNotVisibleFromUnreadable() {
		let found = run(envelope("session.classify", #"{"target":{"client":"codex","session_id":"s-1"},"classification_available":true,"origin":{"automatic":"auxiliary","effective":"work","reason_code":"maintenance_origin","rule_version":"codex-otel-background-v1","override":{"value":"work","binding_status":"matched","binding_limited":false,"applied":true},"revision":2}}"#))
		guard case let .found(query) = SessionOriginDecoding.lookup(found, target: target) else { return XCTFail("a visible session decodes") }
		XCTAssertEqual(query.origin?.automatic, "auxiliary")
		XCTAssertEqual(query.origin?.override?.applied, true)
		XCTAssertEqual(query.origin?.revision, 2)
		XCTAssertEqual(SessionOriginDecoding.lookup(.failed(code: "session_not_found", currentRevision: nil), target: target), .notFound)
		XCTAssertEqual(SessionOriginDecoding.lookup(.failed(code: "runtime_error", currentRevision: nil), target: target), .unavailable)
		XCTAssertEqual(SessionOriginDecoding.lookup(found, target: DesktopOriginTargetV1(client: "claude", sessionID: "s-1")), .unavailable, "another client's session is not this lookup's answer")
	}

	func testOverridePagesAndSourceStatusKeepTheirFixedShapes() {
		let page = run(envelope("session.classify.list", #"{"items":[{"client":"codex","session_id":"archived-1","value":"auxiliary","binding_status":"unmatched","binding_limited":false,"revision":4}],"page":1,"limit":20,"total":1}"#))
		guard case let .value(decoded) = SessionOriginDecoding.overrides(page, page: 1, limit: 20) else { return XCTFail("a page decodes") }
		XCTAssertFalse(decoded.items[0].matched, "unmatched is not applied")
		XCTAssertEqual(decoded.items[0].target, DesktopOriginTargetV1(client: "codex", sessionID: "archived-1"))
		XCTAssertEqual(SessionOriginDecoding.overrides(page, page: 2, limit: 20), .unavailable, "a page other than the one asked for")
		XCTAssertEqual(SessionOriginDecoding.overrides(.failed(code: "runtime_error", currentRevision: nil), page: 1, limit: 20), .unavailable, "an unreadable list is never empty")

		let claude = #"{"source":"claude-mem","kind":"adapter","configured":null,"enabled":null,"available":false,"supported_version":null,"rule_version":"claude-mem-sdk-map-v1","reason_code":"source_config_unreadable","last_run_at":null}"#
		let codex = #"{"source":"codex-otel-background","kind":"builtin","configured":null,"enabled":null,"available":true,"supported_version":null,"rule_version":"codex-otel-background-v1","reason_code":"otel_receiver_unknown","receiver":"unknown","background_sessions":0}"#
		let status = run(envelope("session.origin_source.status", #"{"sources":[\#(claude),\#(codex)]}"#, partial: true))
		guard case let .value(statuses) = SessionOriginDecoding.sourceStatus(status) else { return XCTFail("the two rows decode") }
		XCTAssertTrue(statuses.partial)
		XCTAssertNil(statuses.claudeMem.configured, "an unreadable configuration is unknown, not false")
		XCTAssertEqual(statuses.codexBackground.backgroundSessions, 0)
		XCTAssertEqual(statuses.codexBackground.receiver, "unknown")
		XCTAssertEqual(SessionOriginDecoding.sourceStatus(run(envelope("session.origin_source.status", #"{"sources":[\#(codex),\#(claude)]}"#))), .unavailable)
		XCTAssertEqual(SessionOriginDecoding.sourceStatus(run(envelope("session.origin_source.status", #"{"sources":[\#(claude)]}"#))), .unavailable)
	}

	private func run(_ stdout: Data) -> DesktopOriginHelperRun { .succeeded(stdout: stdout) }

	private func envelope(_ command: String, _ data: String, partial: Bool = false) -> Data {
		Data(#"{"schema_version":1,"command":"\#(command)","generated_at":"2026-11-07T00:00:00Z","data":\#(data),"warnings":[],"partial":\#(partial)}"#.utf8)
	}

	private func makeRunner(process: any EmbeddedHelperProcess) throws -> EmbeddedHelperRunner {
		let temporaryDirectory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		let bundleURL = try makeEmbeddedHelperBundle(in: temporaryDirectory)
		addTeardownBlock { try? FileManager.default.removeItem(at: temporaryDirectory) }
		return EmbeddedHelperRunner(appBundleURL: bundleURL, process: process, environment: ["HOME": "/tmp/isolated-home"], timeout: .milliseconds(50))
	}
}
