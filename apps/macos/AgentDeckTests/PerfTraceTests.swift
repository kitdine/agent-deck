import Darwin
import Foundation
import XCTest
@testable import AgentDeckShared

final class PerfTraceTests: XCTestCase {
	private var directory: URL!

	override func setUpWithError() throws {
		directory = URL(fileURLWithPath: NSTemporaryDirectory(), isDirectory: true)
			.appendingPathComponent("agentdeck-perftrace-\(UUID().uuidString)", isDirectory: true)
		try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: false)
		XCTAssertEqual(chmod(directory.path, 0o700), 0)
	}

	override func tearDownWithError() throws {
		PerfTrace.configure(directory: nil)
		try? FileManager.default.removeItem(at: directory)
	}

	private func traceLines() throws -> [[String: Any]] {
		let file = directory.appendingPathComponent("app-000001.jsonl")
		let text = try String(contentsOf: file, encoding: .utf8)
		return try text.split(separator: "\n").map {
			try XCTUnwrap(JSONSerialization.jsonObject(with: Data($0.utf8)) as? [String: Any])
		}
	}

	func testDisabledRecordingWritesNothing() throws {
		PerfTrace.configure(directory: nil)
		XCTAssertFalse(PerfTrace.isEnabled)
		XCTAssertEqual(PerfTrace.begin(), 0)
		PerfTrace.mark(.inputEvent)
		PerfTrace.flush()
		XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: directory.path), [])
	}

	func testRejectsSharedOutputDirectory() throws {
		XCTAssertEqual(chmod(directory.path, 0o755), 0)
		PerfTrace.configure(directory: directory)
		XCTAssertFalse(PerfTrace.isEnabled)
	}

	func testRecordsFixedStagesCountsAndRejectsFreeFormKeys() throws {
		PerfTrace.configure(directory: directory)
		XCTAssertTrue(PerfTrace.isEnabled)
		PerfTrace.mark(.inputEvent, at: 100)
		PerfTrace.end(.popoverShowEnd, since: PerfTrace.begin(), PerfCount("tokens", 42))
		PerfTrace.mark(.frameVsync, PerfCount("/Users/someone/session-019a", 1))
		PerfTrace.flush()
		PerfTrace.mark(.frameCommit)
		PerfTrace.flush()

		let path = directory.appendingPathComponent("app-000001.jsonl").path
		let mode = try XCTUnwrap(FileManager.default.attributesOfItem(atPath: path)[.posixPermissions] as? Int)
		XCTAssertEqual(mode, 0o600)
		let raw = try String(contentsOfFile: path, encoding: .utf8)
		XCTAssertFalse(raw.contains("someone"))
		XCTAssertFalse(raw.contains("session"))
		let lines = try traceLines()
		XCTAssertEqual(lines[0]["role"] as? String, "app")
		XCTAssertEqual(lines[0]["seq"] as? Int, 1)
		XCTAssertEqual(lines[0]["clock"] as? String, "CLOCK_UPTIME_RAW")
		XCTAssertEqual(lines.dropFirst().compactMap { $0["stage"] as? String },
			["input.event", "popover.show.end", "resource.lifetime", "trace.invalid"])
		XCTAssertEqual(lines[1]["t"] as? Int, 100)
		XCTAssertEqual((lines[2]["n"] as? [String: Int])?["tokens"], 42)
		XCTAssertNotNil(lines[2]["dur"])
		XCTAssertEqual((lines[4]["n"] as? [String: Int])?["rejected"], 1)
		// One file per process lifetime: markers after flush are not written.
		XCTAssertEqual(try FileManager.default.contentsOfDirectory(atPath: directory.path), ["app-000001.jsonl"])
	}

	func testBufferIsBoundedAndOverflowInvalidatesTheTrace() throws {
		PerfTrace.configure(directory: directory)
		for _ in 0..<(PerfTrace.capacity + 3) { PerfTrace.mark(.jobEnqueue) }
		PerfTrace.flush()
		let invalid = try XCTUnwrap(try traceLines().last)
		XCTAssertEqual(invalid["stage"] as? String, "trace.invalid")
		XCTAssertEqual((invalid["n"] as? [String: Int])?["dropped"], 4)
	}

	/// The contract's single clock: clock_gettime_nsec_np(CLOCK_UPTIME_RAW),
	/// the uptime seconds NSEvent timestamps use, and the kernel's process start
	/// (ri_proc_start_abstime) agree within 1 ms. Go's reading of the same clock
	/// is checked across processes by the App test that launches the helper.
	func testClockSourcesAgree() throws {
		let before = PerfTrace.now()
		let uptime = PerfTrace.nanoseconds(fromUptimeSeconds: ProcessInfo.processInfo.systemUptime)
		let after = PerfTrace.now()
		XCTAssertLessThanOrEqual(before, uptime + 1_000_000)
		XCTAssertLessThanOrEqual(uptime, after + 1_000_000)

		let child = Process()
		child.executableURL = URL(fileURLWithPath: "/bin/sleep")
		child.arguments = ["1"]
		let launchBefore = PerfTrace.now()
		try child.run()
		let launchAfter = PerfTrace.now()
		defer { child.terminate() }
		let started = try XCTUnwrap(PerfTrace.processStartUptime(pid: child.processIdentifier))
		XCTAssertGreaterThanOrEqual(started + 1_000_000, launchBefore)
		XCTAssertLessThanOrEqual(started, launchAfter + 1_000_000)
		XCTAssertLessThanOrEqual(try XCTUnwrap(PerfTrace.processStartUptime()), before)
	}
}
