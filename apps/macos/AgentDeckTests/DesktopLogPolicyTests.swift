import XCTest
@testable import AgentDeckShared

final class DesktopLogPolicyTests: XCTestCase {
	func testPopoverPreparationHasItsOwnContentFreeTimingStage() throws {
		DesktopPerformanceTrace.start()
		DesktopLogger.recordPerformance(DesktopPerformanceInterval(startedAt: 10).sample(.popoverPreparation, endedAt: 25))
		DesktopLogger.recordPerformance(DesktopPerformanceInterval(startedAt: 30).sample(.popoverOpen, endedAt: 35))
		let capture = DesktopPerformanceTrace.finish()
		XCTAssertEqual(capture.samples.map(\.stage), [.popoverPreparation, .popoverOpen])
		XCTAssertEqual(capture.samples.map(\.durationNanoseconds), [15, 5])
		let record = try XCTUnwrap(try JSONSerialization.jsonObject(with: JSONEncoder().encode(capture.samples[0])) as? [String: Any])
		XCTAssertEqual(record["stage"] as? String, "popoverPreparation")
		XCTAssertEqual(Set(record.keys), ["stage", "durationNanoseconds", "completedAtUptimeNanoseconds"])
	}
	func testPerformanceCaptureIsBoundedAndContainsOnlyClassifiedTimings() throws {
		DesktopPerformanceTrace.start()
		for _ in 0..<1027 {
			DesktopLogger.recordPerformance(DesktopPerformanceInterval(startedAt: 50).sample(.publication, endedAt: 75))
		}
		let capture = DesktopPerformanceTrace.finish()
		XCTAssertEqual(capture.samples.count, 1024)
		XCTAssertEqual(capture.dropped, 3)
		XCTAssertEqual(capture.samples[0].durationNanoseconds, 25)
		let json = try JSONSerialization.jsonObject(with: JSONEncoder().encode(capture.samples[0])) as? [String: Any]
		XCTAssertEqual(Set(json?.keys.map { $0 } ?? []), ["stage", "durationNanoseconds", "completedAtUptimeNanoseconds"])
		XCTAssertEqual(DesktopPerformanceInterval(startedAt: 75).sample(.quota, endedAt: 50).durationNanoseconds, 0)
		DesktopLogger.recordPerformance(DesktopPerformanceInterval().sample(.quota))
		XCTAssertTrue(DesktopPerformanceTrace.finish().samples.isEmpty)
		DesktopPerformanceTrace.start()
		for _ in 0..<20 {
			DesktopPerformanceTrace.recordFailure(DesktopPerformanceFailure(stage: .scanRequest,
				code: "/private/session/secret body", exitStatus: 1, stdoutBytes: 0, stderrBytes: 42))
		}
		let failures = DesktopPerformanceTrace.finish().failures
		XCTAssertEqual(failures.count, 16)
		XCTAssertTrue(failures.allSatisfy { $0.code == "unclassified" })
		DesktopPerformanceTrace.start()
		for _ in 0..<5000 {
			DesktopLogger.recordPerformance(DesktopPerformanceInterval(startedAt: 1).sample(.presentation, endedAt: 3))
		}
		DesktopLogger.recordPerformance(DesktopPerformanceInterval(startedAt: 1).sample(.snapshotDecode, endedAt: 4))
		let hot = DesktopPerformanceTrace.finish()
		XCTAssertEqual(hot.dropped, 0)
		XCTAssertEqual(hot.samples.count, 1)
		XCTAssertEqual(hot.aggregates.first { $0.stage == .presentation }?.count, 5000)
		XCTAssertEqual(hot.aggregates.first { $0.stage == .presentation }?.totalNanoseconds, 10000)
	}
    func testLogPolicyUsesClassificationsInsteadOfSnapshotContent() throws {
        let complete = try decodeDesktopWireEnvelopeV1(desktopFixtureData("snapshot-complete.json"))

        XCTAssertEqual(DesktopLogPolicy.snapshotEvent(for: complete), "desktop_snapshot_complete")
        XCTAssertEqual(DesktopLogPolicy.helperFailureEvent(.nonZeroExit(23)), "embedded_helper_failed")
        XCTAssertFalse(DesktopLogPolicy.helperFailureEvent(.nonZeroExit(23)).contains("23"))
    }
}
