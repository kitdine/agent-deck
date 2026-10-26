import Darwin
import Foundation
import Synchronization

/// Fixed marker names from the scan-performance measurement contract.
public enum PerfStage: String, Sendable, CaseIterable {
	case inputEvent = "input.event"
	case inputAction = "input.action"
	case popoverShowBegin = "popover.show.begin"
	case popoverShowEnd = "popover.show.end"
	case popoverLayoutData = "popover.layout.data"
	case frameCommit = "frame.commit"
	case frameVsync = "frame.vsync"
	case selectIntent = "select.intent"
	case selectLayoutData = "select.layout.data"
	case processStart = "process.start"
	case appReady = "app.ready"
	case restoreIdentity = "restore.identity"
	case restoreDecode = "restore.decode"
	case modelUsable = "model.usable"
	case sourceFSEvent = "source.fsevent"
	case jobEnqueue = "job.enqueue"
	case publicationBound = "publication.bound"
	case publicationComplete = "publication.complete"
	case helperStart = "helper.start"
	case helperExit = "helper.exit"
	case idleWindowBegin = "idle.window.begin"
	case idleWindowEnd = "idle.window.end"
	case resourceLifetime = "resource.lifetime"
	case scanRequest = "app.scan_request"
	case workerDrain = "app.worker_drain"
	case snapshotRequest = "app.snapshot_request"
	case snapshotDecode = "app.snapshot_decode"
	case quota = "app.quota"
	case servingInstall = "app.serving_install"
	case sourceProcessing = "app.source_processing"
	case driverDone = "driver.done"
	case driverInvalid = "driver.invalid"
	/// Another app took the front from the shown popover: the run unit measured
	/// nothing about the product and is rerun.
	case driverInterrupted = "driver.interrupted"
	case driverPreconditionTimeout = "driver.precondition_timeout"
	case selectNoData = "select.no_data"
	/// An event-stimulus click that never reached `handleClick` in a run that
	/// reports routing instead of measuring latency (P01 routing pairs).
	case inputUnrouted = "input.unrouted"
	/// A laid-out data frame the endpoint could not commit yet, with integer
	/// window-state flags; recorded once per pending frame (diagnostic only).
	case frameBlocked = "frame.blocked"
	/// A bound-publication candidate the App did not install, with a reason code
	/// (1 = other day, 2 = would replace a session-prepared publication).
	case servingSkip = "app.serving_skip"
	/// A full refresh that ended in failure, with an integer issue code.
	case refreshFailed = "app.refresh_failed"
}

/// One integer attribute of a marker. Keys are declared identifiers.
public struct PerfCount: Sendable {
	public let key: String
	public let value: Int64
	public init(_ key: String, _ value: Int64) {
		self.key = key
		self.value = value
	}
}

/// In-process marker recorder. Recording is off unless `AGENTDECK_PERF_OUT`
/// names a private (0700) directory owned by this user; when off every call
/// is a single branch. Records carry stage names, CLOCK_UPTIME_RAW nanoseconds,
/// integer counts and fixed codes only — never paths, identifiers or text.
public enum PerfTrace {
	public static let environmentKey = "AGENTDECK_PERF_OUT"
	static let capacity = 1 << 16

	struct Record: Sendable {
		let stage: String
		let time: UInt64
		let duration: UInt64?
		let counts: [PerfCount]
	}

	private struct Buffer: Sendable {
		var records = [Record]()
		var dropped = 0
		var rejected = 0
		var flushed = false
	}

	private static let directory = Mutex<URL?>(validatedDirectory(ProcessInfo.processInfo.environment[environmentKey]))
	private static let enabled = Atomic<Bool>(directory.withLock { $0 != nil })
	private static let buffer = Mutex(Buffer())

	@inline(__always)
	public static var isEnabled: Bool { enabled.load(ordering: .relaxed) }

	/// Starts a fresh recording into `directory` (or stops recording for nil).
	/// Tests use it; product code is configured only by the environment.
	public static func configure(directory url: URL?) {
		let valid = url.flatMap { validatedDirectory($0.path) }
		directory.withLock { $0 = valid }
		buffer.withLock { $0 = Buffer() }
		enabled.store(valid != nil, ordering: .relaxed)
	}

	/// CLOCK_UPTIME_RAW in nanoseconds; the same clock Go's perftrace reads.
	@inline(__always)
	public static func now() -> UInt64 { clock_gettime_nsec_np(CLOCK_UPTIME_RAW) }

	/// The kernel's start time of a process on the same clock.
	public static func processStartUptime(pid: pid_t = getpid()) -> UInt64? {
		rusage(pid: pid).map { machToNanoseconds($0.ri_proc_start_abstime) }
	}

	static func machToNanoseconds(_ ticks: UInt64) -> UInt64 {
		var base = mach_timebase_info_data_t()
		mach_timebase_info(&base)
		return ticks &* UInt64(base.numer) / UInt64(base.denom)
	}

	/// NSEvent and ProcessInfo timestamps are seconds on the same uptime clock.
	public static func nanoseconds(fromUptimeSeconds seconds: TimeInterval) -> UInt64 {
		UInt64((seconds * 1_000_000_000).rounded())
	}

	public static func mark(_ stage: PerfStage, at time: UInt64? = nil, _ counts: PerfCount...) {
		mark(stage, at: time, counts: counts)
	}

	public static func mark(_ stage: PerfStage, at time: UInt64? = nil, counts: [PerfCount]) {
		guard isEnabled else { return }
		append(Record(stage: stage.rawValue, time: time ?? now(), duration: nil, counts: counts))
	}

	/// Records `stage` from `start` to now. Pass `begin()`'s result as start.
	public static func end(_ stage: PerfStage, since start: UInt64, _ counts: PerfCount...) {
		guard isEnabled, start != 0 else { return }
		let ended = now()
		append(Record(stage: stage.rawValue, time: start, duration: ended >= start ? ended - start : 0, counts: counts))
	}

	/// The start time for `end(_:since:)`, or 0 when recording is off.
	public static func begin() -> UInt64 { isEnabled ? now() : 0 }

	static func append(_ record: Record) {
		let valid = record.counts.allSatisfy { isIdentifier($0.key) }
		buffer.withLock {
			guard !$0.flushed else { return }
			guard valid else {
				$0.rejected += 1
				return
			}
			if $0.records.count < capacity { $0.records.append(record) } else { $0.dropped += 1 }
		}
	}

	/// Writes the buffered records once, with lifetime resources last. Later
	/// markers are ignored, so a flushed file is complete and immutable.
	public static func flush() {
		guard let directory = directory.withLock({ $0 }) else { return }
		let resources = lifetime()
		mark(.resourceLifetime, PerfCount("phys_footprint_max_bytes", Int64(clamping: resources.footprint)),
			PerfCount("pkg_idle_wkups", Int64(clamping: resources.idleWakeups)),
			PerfCount("interrupt_wkups", Int64(clamping: resources.interruptWakeups)))
		let snapshot = buffer.withLock { state -> Buffer? in
			guard !state.flushed else { return nil }
			state.flushed = true
			return state
		}
		guard let snapshot else { return }
		var body = snapshot.records.map(line(for:))
		if snapshot.dropped > 0 || snapshot.rejected > 0 {
			body.append(line(for: Record(stage: "trace.invalid", time: now(), duration: nil,
				counts: [PerfCount("dropped", Int64(snapshot.dropped)), PerfCount("rejected", Int64(snapshot.rejected))])))
		}
		for sequence in 1...100_000 {
			let path = directory.appendingPathComponent(String(format: "app-%06d.jsonl", sequence)).path
			let descriptor = open(path, O_WRONLY | O_CREAT | O_EXCL | O_CLOEXEC, 0o600)
			if descriptor < 0 {
				if errno == EEXIST { continue }
				return
			}
			let lines = [headerLine(sequence: sequence, start: snapshot.records.first?.time ?? now())] + body
			let data = Data((lines.joined(separator: "\n") + "\n").utf8)
			_ = data.withUnsafeBytes { write(descriptor, $0.baseAddress, $0.count) }
			fsync(descriptor)
			close(descriptor)
			return
		}
	}

	public struct WindowResources: Sendable {
		public let userNanoseconds: UInt64
		public let systemNanoseconds: UInt64
		public let residentBytes: UInt64
		public let wakeups: UInt64
	}

	/// Current CPU, resident size and wakeups of this process for P06 windows.
	public static func windowResources() -> WindowResources? {
		guard let info = rusage() else { return nil }
		return WindowResources(userNanoseconds: machToNanoseconds(info.ri_user_time),
			systemNanoseconds: machToNanoseconds(info.ri_system_time), residentBytes: info.ri_resident_size,
			wakeups: info.ri_pkg_idle_wkups &+ info.ri_interrupt_wkups)
	}

	private static func lifetime() -> (footprint: UInt64, idleWakeups: UInt64, interruptWakeups: UInt64) {
		guard let info = rusage() else { return (0, 0, 0) }
		return (info.ri_lifetime_max_phys_footprint, info.ri_pkg_idle_wkups, info.ri_interrupt_wkups)
	}

	private static func rusage(pid: pid_t = getpid()) -> rusage_info_v4? {
		var info = rusage_info_v4()
		let status = withUnsafeMutablePointer(to: &info) {
			$0.withMemoryRebound(to: rusage_info_t?.self, capacity: 1) {
				proc_pid_rusage(pid, RUSAGE_INFO_V4, $0)
			}
		}
		return status == 0 ? info : nil
	}

	private static func headerLine(sequence: Int, start: UInt64) -> String {
		"{\"v\":1,\"role\":\"app\",\"seq\":\(sequence),\"clock\":\"CLOCK_UPTIME_RAW\",\"t\":\(start)}"
	}

	private static func line(for record: Record) -> String {
		var text = "{\"stage\":\"\(record.stage)\",\"t\":\(record.time)"
		if let duration = record.duration { text += ",\"dur\":\(duration)" }
		if !record.counts.isEmpty {
			text += ",\"n\":{" + record.counts.map { "\"\($0.key)\":\($0.value)" }.joined(separator: ",") + "}"
		}
		return text + "}"
	}

	static func isIdentifier(_ value: String) -> Bool {
		let bytes = Array(value.utf8)
		guard let first = bytes.first, (97...122).contains(first), bytes.count <= 48 else { return false }
		return bytes.allSatisfy { (97...122).contains($0) || (48...57).contains($0) || $0 == 95 }
	}

	static func validatedDirectory(_ raw: String?) -> URL? {
		guard let raw, raw.hasPrefix("/") else { return nil }
		var status = stat()
		guard lstat(raw, &status) == 0, (status.st_mode & S_IFMT) == S_IFDIR,
			status.st_mode & 0o777 == 0o700, status.st_uid == geteuid() else { return nil }
		return URL(fileURLWithPath: raw, isDirectory: true)
	}
}
