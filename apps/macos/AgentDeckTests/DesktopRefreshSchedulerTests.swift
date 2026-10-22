import XCTest
@testable import AgentDeckShared

@MainActor
final class DesktopRefreshSchedulerTests: XCTestCase {
	func testSourceMonitorObservesRealWritesAndStopsWithOwner() async throws {
		let home = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
		let root = home.appendingPathComponent(".codex/sessions")
		try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
		defer { try? FileManager.default.removeItem(at: home) }
		let recorder = SchedulerRequestRecorder()
		let monitor = DesktopSourceChangeMonitor(home: home) { recorder.triggers.append(.sourceChange) }
		XCTAssertTrue(monitor.start())
		defer { monitor.stop() }
		let source = root.appendingPathComponent("synthetic.jsonl")
		try Data("{}\n".utf8).write(to: source)
		let deadline = ContinuousClock.now + .seconds(3)
		while recorder.triggers.isEmpty && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
		XCTAssertFalse(recorder.triggers.isEmpty)
		monitor.stop()
		let count = recorder.triggers.count
		try Data("{}\n{}\n".utf8).write(to: source)
		try await Task.sleep(for: .milliseconds(300))
		XCTAssertEqual(recorder.triggers.count, count)
	}

	func testCompletionSchedulesOneRequestAtSixtySecondsWithoutReplay() {
		let clock = SchedulerClock()
		let recorder = SchedulerRequestRecorder()
		let scheduler = DesktopRefreshScheduler(enabled: true, now: { clock.now }) {
			recorder.triggers.append($0)
		}

		scheduler.fullAttemptCompleted()
		XCTAssertEqual(scheduler.deadline, 60)
		clock.advance(by: 59)
		scheduler.evaluate(.periodic)
		XCTAssertTrue(recorder.triggers.isEmpty)
		clock.advance(by: 1)
		scheduler.evaluate(.periodic)
		scheduler.evaluate(.periodic)

		XCTAssertEqual(recorder.triggers, [.periodic])
		XCTAssertNil(scheduler.deadline)
	}

	func testWakeCatchUpFiresImmediatelyAfterElapsedDeadline() {
		let clock = SchedulerClock()
		let recorder = SchedulerRequestRecorder()
		let scheduler = DesktopRefreshScheduler(enabled: true, now: { clock.now }) {
			recorder.triggers.append($0)
		}

		scheduler.fullAttemptCompleted()
		clock.advance(by: 600)
		scheduler.evaluate(.wake)

		XCTAssertEqual(recorder.triggers, [.wake])
		XCTAssertNil(scheduler.deadline)
	}

	func testDisableClearsDeadlineAndReenableDoesNotBackfill() {
		let clock = SchedulerClock()
		let recorder = SchedulerRequestRecorder()
		let scheduler = DesktopRefreshScheduler(enabled: true, now: { clock.now }) {
			recorder.triggers.append($0)
		}

		scheduler.fullAttemptCompleted()
		scheduler.updateEnabled(false)
		clock.advance(by: 600)
		scheduler.evaluate(.wake)
		XCTAssertNil(scheduler.deadline)
		XCTAssertTrue(recorder.triggers.isEmpty)

		scheduler.updateEnabled(true)
		XCTAssertEqual(scheduler.deadline, 660)
		clock.advance(by: 60)
		scheduler.evaluate(.wake)
		scheduler.evaluate(.wake)
		XCTAssertEqual(recorder.triggers, [.wake])
	}

	func testTenTerminalCyclesHaveOneRequestEachAndNoOverlapReplay() {
		let clock = SchedulerClock()
		let recorder = SchedulerRequestRecorder()
		let scheduler = DesktopRefreshScheduler(enabled: true, now: { clock.now }) {
			recorder.triggers.append($0)
		}

		for _ in 0 ..< 10 {
			scheduler.fullAttemptCompleted()
			clock.advance(by: 60)
			scheduler.evaluate(.periodic)
			scheduler.evaluate(.periodic)
		}

		XCTAssertEqual(recorder.triggers, Array(repeating: .periodic, count: 10))
	}

	func testDueFullEvaluationDoesNotWaitForSuspendedQuotaAndQuotaRemainsOwnedUntilStopped() async {
		let clock = SchedulerClock()
		let recorder = SchedulerRequestRecorder()
		let quota = SuspendedPeriodicQuota()
		let scheduler = DesktopRefreshScheduler(enabled: true, now: { clock.now }) {
			recorder.triggers.append($0)
		}
		let driver = DesktopRefreshPeriodicDriver(scheduler: scheduler) {
			await quota.run()
		}
		scheduler.fullAttemptCompleted()
		clock.advance(by: 60)

		let tick = Task { await driver.tick(quotaEnabled: true) }
		await quota.waitUntilSuspended()
		await driver.tick(quotaEnabled: true)

		XCTAssertEqual(recorder.triggers, [.periodic])
		let quotaRuns = await quota.runCount
		XCTAssertEqual(quotaRuns, 1)
		await tick.value
		XCTAssertTrue(driver.hasActiveQuotaRequest)
		driver.cancel()
		XCTAssertTrue(driver.hasActiveQuotaRequest, "cancelled quota work remains owned until its operation exits")
		await quota.resume()
		await driver.waitForQuotaIdle()
		XCTAssertFalse(driver.hasActiveQuotaRequest)
	}
}

@MainActor
private final class SchedulerClock {
	private(set) var now: TimeInterval = 0
	func advance(by interval: TimeInterval) { now += interval }
}

@MainActor
private final class SchedulerRequestRecorder {
	var triggers = [DesktopFullRefreshTrigger]()
}

private actor SuspendedPeriodicQuota {
	private(set) var runCount = 0
	private var continuation: CheckedContinuation<Void, Never>?

	func run() async {
		runCount += 1
		await withCheckedContinuation { continuation = $0 }
	}

	func waitUntilSuspended() async {
		while continuation == nil { await Task.yield() }
	}

	func resume() {
		continuation?.resume()
		continuation = nil
	}
}
