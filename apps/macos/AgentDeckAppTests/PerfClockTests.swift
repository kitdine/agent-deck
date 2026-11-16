import AgentDeckShared
import Darwin
import XCTest
@testable import AgentDeck

/// Go's perftrace and Swift's PerfTrace must read one clock. The embedded Go
/// helper writes its trace header at process start; that reading must fall
/// between Swift readings taken before launch and after exit.
final class PerfClockTests: XCTestCase {
	func testEmbeddedGoHelperSharesTheSwiftUptimeClock() throws {
		let helper = Bundle.main.bundleURL.appendingPathComponent("Contents/Helpers/agentdeck")
		try XCTSkipUnless(FileManager.default.isExecutableFile(atPath: helper.path), "App build has no embedded helper")
		let directory = URL(fileURLWithPath: NSTemporaryDirectory(), isDirectory: true)
			.appendingPathComponent("agentdeck-perfclock-\(UUID().uuidString)", isDirectory: true)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: false)
		defer { try? FileManager.default.removeItem(at: directory) }
		XCTAssertEqual(chmod(directory.path, 0o700), 0)

		let process = Process()
		process.executableURL = helper
		process.arguments = ["--version"]
		process.environment = ["PATH": "/usr/bin:/bin", "HOME": directory.path, PerfTrace.environmentKey: directory.path]
		process.standardOutput = FileHandle.nullDevice
		let before = PerfTrace.now()
		try process.run()
		process.waitUntilExit()
		let after = PerfTrace.now()
		XCTAssertEqual(process.terminationStatus, 0)

		let trace = try String(contentsOf: directory.appendingPathComponent("cli-000001.jsonl"), encoding: .utf8)
		let header = try XCTUnwrap(trace.split(separator: "\n").first)
		let fields = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(header.utf8)) as? [String: Any])
		XCTAssertEqual(fields["clock"] as? String, "CLOCK_UPTIME_RAW")
		let goStart = UInt64(try XCTUnwrap(fields["t"] as? Int))
		XCTAssertGreaterThanOrEqual(goStart + 1_000_000, before)
		XCTAssertLessThanOrEqual(goStart, after + 1_000_000)
	}
}

/// `cmd/agentdeck-perf` checks every data frame's hero against its oracle with
/// the same hashes; its TestFNVDigestMatchesTheAppProbe pins the same value.
final class PerfDataFrameDigestTests: XCTestCase {
	func testDigestMatchesTheMeasurementTool() {
		XCTAssertEqual(PerfDataFrame.fnv1a(["pub", "all", "today", "breakdown"]), 5_653_972_754_369_679_809)
		XCTAssertEqual(PerfDataFrame.key(publication: "pub", client: "all", period: "today", panel: "breakdown"),
			5_653_972_754_369_679_809)
		XCTAssertEqual(PerfDataFrame.code(client: "claude"), 3)
		XCTAssertEqual(PerfDataFrame.code(period: "30d"), 3)
	}
}
