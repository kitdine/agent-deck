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

	func testAutomaticUpdatesWatchOnlyWhileEnabledAndNeverSchedulePeriodically() {
		let probe = AutomaticUpdateProbe()
		let updates = probe.make(enabled: true)
		XCTAssertFalse(updates.isWatching, "nothing watches before the startup restore settles")

		updates.start()
		XCTAssertTrue(updates.isWatching)
		XCTAssertEqual(probe.starts, 1)
		XCTAssertTrue(probe.triggers.isEmpty, "the startup update is the launch reconciliation")

		updates.setEnabled(false)
		XCTAssertFalse(updates.isWatching)
		XCTAssertEqual(probe.stops, 1)
		updates.didWake()
		XCTAssertTrue(probe.triggers.isEmpty, "off means no watching and no wake reconciliation")

		updates.setEnabled(true)
		XCTAssertTrue(updates.isWatching)
		XCTAssertEqual(probe.triggers, [.wake], "re-enabling reconciles changes made while off")

		updates.stop()
		XCTAssertFalse(updates.isWatching)
		updates.didWake()
		XCTAssertEqual(probe.triggers, [.wake])
		XCTAssertEqual(probe.starts, 2)
		XCTAssertEqual(probe.stops, 2)
	}

	func testWakeRequestsOneReconciliationEachWhileEnabled() {
		let probe = AutomaticUpdateProbe()
		let updates = probe.make(enabled: true)
		updates.didWake()
		XCTAssertTrue(probe.triggers.isEmpty, "a wake before start is covered by the startup update")
		updates.start()
		updates.didWake()
		updates.didWake()
		XCTAssertEqual(probe.triggers, [.wake, .wake])
	}

	func testDisabledAtLaunchStartsNothingUntilEnabled() {
		let probe = AutomaticUpdateProbe()
		let updates = probe.make(enabled: false)
		updates.start()
		XCTAssertFalse(updates.isWatching)
		XCTAssertEqual(probe.starts, 0)
		updates.setEnabled(true)
		XCTAssertTrue(updates.isWatching)
		XCTAssertEqual(probe.triggers, [.wake])
	}

	func testToggleBeforeStartOnlyRecordsThePreference() {
		let probe = AutomaticUpdateProbe()
		let updates = probe.make(enabled: true)
		updates.setEnabled(false)
		updates.setEnabled(true)
		XCTAssertEqual(probe.starts, 0)
		XCTAssertTrue(probe.triggers.isEmpty)
		updates.start()
		XCTAssertTrue(updates.isWatching)
		XCTAssertTrue(probe.triggers.isEmpty)
	}

	func testFailedWatchStartIsNotReportedAsWatching() {
		let probe = AutomaticUpdateProbe()
		probe.startSucceeds = false
		let updates = probe.make(enabled: true)
		updates.start()
		XCTAssertFalse(updates.isWatching)
		updates.setEnabled(false)
		XCTAssertEqual(probe.stops, 0, "a watch that never started is not stopped")
	}

	func testQuotaTickNeverRequestsDataAndQuotaRemainsOwnedUntilStopped() async {
		let quota = SuspendedPeriodicQuota()
		let driver = DesktopRefreshPeriodicDriver {
			await quota.run()
		}

		let tick = Task { await driver.tick(quotaEnabled: true) }
		await quota.waitUntilSuspended()
		await driver.tick(quotaEnabled: true)

		let quotaRuns = await quota.runCount
		XCTAssertEqual(quotaRuns, 1)
		await tick.value
		XCTAssertTrue(driver.hasActiveQuotaRequest)
		driver.cancel()
		XCTAssertTrue(driver.hasActiveQuotaRequest, "cancelled quota work remains owned until its operation exits")
		await quota.resume()
		await driver.waitForQuotaIdle()
		XCTAssertFalse(driver.hasActiveQuotaRequest)
		await driver.tick(quotaEnabled: false)
		XCTAssertFalse(driver.hasActiveQuotaRequest)
	}
}

@MainActor
private final class AutomaticUpdateProbe {
	var starts = 0
	var stops = 0
	var startSucceeds = true
	var triggers = [DesktopFullRefreshTrigger]()

	func make(enabled: Bool) -> DesktopAutomaticUpdates {
		DesktopAutomaticUpdates(
			enabled: enabled,
			startWatching: { [unowned self] in
				starts += 1
				return startSucceeds
			},
			stopWatching: { [unowned self] in stops += 1 },
			request: { [unowned self] in triggers.append($0) }
		)
	}
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
