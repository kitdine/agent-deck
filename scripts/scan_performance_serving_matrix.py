"""Private E1 condition preparation and honest, separately attributed clocks.

Imported by scan-performance-serving.py. No live HOME, credentials, event
permission changes, or acceptance writes. Model-only observations are NOT frames.
"""
import json
import base64
import hashlib
import math
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid
from datetime import datetime, timezone
from zoneinfo import ZoneInfo


CONDITIONS = ("warm", "restore", "committed-today-no-cache", "first-import", "rebuilding", "slow-quota")
DIAGNOSTIC_CONDITIONS = ("upgrade-no-cache",)
CACHE = "desktop-serving-snapshot.json"


def clone_frozen_input(root, home, digest):
    if root.is_symlink() or not str(root.resolve()).startswith("/private/tmp/agentdeck-macos-xctest.serving."):
        raise RuntimeError("private_frozen_source_root_required")
    source_home = (root / "home").resolve()
    inventory = json.loads((root / "private-inventory.json").read_text())
    for item in inventory:
        relative = Path(item["relative"])
        if relative.is_absolute() or ".." in relative.parts or relative.suffix != ".jsonl" or not any(
                relative.is_relative_to(prefix) for prefix in (".codex/sessions", ".codex/archived_sessions", ".claude/projects")):
            raise RuntimeError("private_frozen_inventory_path_rejected")
        source = source_home / relative
        if source.is_symlink() or not source.resolve().is_relative_to(source_home):
            raise RuntimeError("private_frozen_inventory_symlink_rejected")
        if source.stat().st_size != item["bytes"] or digest(source) != item["sha256"]:
            raise RuntimeError("private_frozen_inventory_content_changed")
        target = home / relative
        target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        with source.open("rb") as original, target.open("xb") as copy:
            shutil.copyfileobj(original, copy, 1 << 20)
        target.chmod(0o600)
        os.utime(target, ns=(item["mtime_ns"], item["mtime_ns"]))
        if digest(target) != item["sha256"]:
            raise RuntimeError("private_frozen_clone_changed_during_copy")
    return inventory


def add_today_sentinel(home, digest):
    """An explicitly declared oracle source, not invented real user activity."""
    stamp = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    events = [
        {"type": "session_meta", "timestamp": stamp, "payload": {"id": "e1-controlled-today", "cwd": "/synthetic-e1"}},
        {"type": "turn_context", "timestamp": stamp, "payload": {"turn_id": "e1-controlled-turn", "model": "gpt-5"}},
        {"type": "event_msg", "timestamp": stamp, "payload": {"type": "token_count", "info": {
            "last_token_usage": {"input_tokens": 1000000, "output_tokens": 10},
            "total_token_usage": {"input_tokens": 1000000, "output_tokens": 10}}}},
    ]
    source = home / ".codex/sessions/e1-controlled-today.jsonl"
    with source.open("x") as stream:
        for event in events:
            stream.write(json.dumps(event) + "\n")
    source.chmod(0o600)
    return {"relative": str(source.relative_to(home)), "bytes": source.stat().st_size,
            "mtime_ns": source.stat().st_mtime_ns, "sha256": digest(source)}


def apply_input_provenance(value, current, inventory):
    prior = value["input"]
    if any(prior[key] != current[key] for key in ("corpus_sha256", "files", "bytes")):
        raise RuntimeError("private_input_provenance_content_changed")
    controlled = prior.get("controlled_today_source")
    if controlled and not any(all(item.get(key) == controlled[key] for key in ("sha256", "bytes", "mtime_ns")) for item in inventory):
        raise RuntimeError("private_input_provenance_controlled_source_changed")
    for key in ("kind", "controlled_today_source"):
        if key in prior:
            current[key] = prior[key]
    if prior.get("kind", "").startswith("verified_frozen_clone"):
        current["capture_consistency"] = "verified earlier per-file frozen JSONL copied with original mtime;databases generated privately,not new live backups"
    return current


def replace_once(text, anchor, replacement):
    if text.count(anchor) != 1:
        raise RuntimeError("private_matrix_probe_anchor_mismatch")
    return text.replace(anchor, replacement)


def patch_matrix_probe(repository, digest):
    """Install instrumentation only in an exported App, not production sources."""
    controller = repository / "apps/macos/AgentDeckApp/MenuBarItemController.swift"
    text = controller.read_text()
    text = replace_once(text, "\tprivate let openSettings: () -> Void", """\tvar e1AppReadyUptime: UInt64 = 0
\tprivate var e1ModelReadyUptime: UInt64?
\tprivate var e1ModelClockTask: Task<Void, Never>?
\tprivate var e1QuotaPressureAtFirstAction = false
\tprivate var e1RebuildPressureAtFirstAction = false
\tprivate var e1RebuildProcessingAtFirstAction: [String: Any] = [:]
\tprivate let openSettings: () -> Void""")
    text = replace_once(text, "\t\tstruct Frame: Codable {", """\t\te1ModelClockTask = Task { @MainActor in
\t\t\tawait self.e1ObserveModel(home: home, processEntryUptime: processEntryUptime)
\t\t}
\t\tdefer { e1ModelClockTask?.cancel() }
\t\tstruct Frame: Codable {""")
    warm_anchor = "\t\t\t// First click uses the status item's normal action, including the empty surface."
    text = replace_once(text, warm_anchor, """\t\t\tif ProcessInfo.processInfo.environment["AGENTDECK_SERVING_CONDITION"] == "warm" {
\t\t\t\tawait e1ModelClockTask?.value
\t\t\t\tguard e1ModelReadyUptime != nil else { throw BaselineFailure.noData }
\t\t\t\tlet warmDeadline = DispatchTime.now().uptimeNanoseconds + 150_000_000_000
\t\t\t\twhile model.coordinator.latestSnapshot?.data.sessions.available != true {
\t\t\t\t\tguard DispatchTime.now().uptimeNanoseconds < warmDeadline else { throw BaselineFailure.noData }
\t\t\t\t\ttry await Task.sleep(for: .milliseconds(10))
\t\t\t\t}
\t\t\t}
""" + warm_anchor)
    action = "\t\t\tbaselineManualClickUptime = DispatchTime.now().uptimeNanoseconds"
    text = replace_once(text, action, action + """
\t\t\tif baselineManualClicks == 0, let home = baselineManualHome {
\t\t\t\te1QuotaPressureAtFirstAction = e1MarkerPending(home: home, name: "e1-quota-delay.json")
\t\t\t\tif let data = try? Data(contentsOf: home.appendingPathComponent("e1-rebuild.json")),
\t\t\t\t\tvar marker = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
\t\t\t\t\tmarker["action_uptime_ns"] = baselineManualClickUptime
\t\t\t\t\te1RebuildProcessingAtFirstAction = marker
\t\t\t\t\te1RebuildPressureAtFirstAction = marker["phase"] as? String == "processing"
\t\t\t\t}
\t\t\t}""")
    environment = '"firstActionReached": firstActionReached, "firstEventPostingAllowed": firstEventPostingAllowed]'
    text = replace_once(text, environment, '"firstActionReached": firstActionReached, "firstEventPostingAllowed": firstEventPostingAllowed, '
                        '"quotaPressureAtFirstAction": e1QuotaPressureAtFirstAction, "rebuildPressureAtFirstAction": e1RebuildPressureAtFirstAction], '
                        'rebuildProcessingAtFirstActionJSON: e1RebuildActionJSON()')
    text = replace_once(text, 'let nativeEnvironment: [String: Bool]',
                        'let nativeEnvironment: [String: Bool]\n\t\t\tlet rebuildProcessingAtFirstActionJSON: String')
    text = replace_once(text, "\tprivate enum BaselineFailure: String, Error {", MODEL_CLOCK + STARTUP_FRAME + "\n\tprivate enum BaselineFailure: String, Error {")
    controller.write_text(text)

    delegate = repository / "apps/macos/AgentDeckApp/AgentDeckApp.swift"
    text = delegate.read_text()
    text = replace_once(text, "\tfunc applicationDidFinishLaunching(_ notification: Notification) {",
                        "\tfunc applicationDidFinishLaunching(_ notification: Notification) {\n\t\tlet e1Ready = DispatchTime.now().uptimeNanoseconds")
    text = replace_once(text, "\t\tif automaticRefreshEnabled {", "\t\titemController?.e1AppReadyUptime = e1Ready\n\t\tif automaticRefreshEnabled {")
    text = replace_once(text, "\t\t\t\tawait itemController.captureNativeBaseline(home: home, processEntryUptime: baselineProcessEntryUptime)", """\t\t\t\tif ProcessInfo.processInfo.environment["AGENTDECK_SERVING_MEASUREMENT"] == "model" {
\t\t\t\t\tawait itemController.captureE1ModelOnly(home: home, processEntryUptime: baselineProcessEntryUptime)
\t\t\t\t} else if ProcessInfo.processInfo.environment["AGENTDECK_SERVING_MEASUREMENT"] == "startup-frame" {
\t\t\t\t\tawait itemController.captureE1StartupFrame(home: home, processEntryUptime: baselineProcessEntryUptime)
\t\t\t\t} else {
\t\t\t\t\tawait itemController.captureNativeBaseline(home: home, processEntryUptime: baselineProcessEntryUptime)
\t\t\t\t\tawait itemController.e1WaitForQuotaDelay(home: home)
\t\t\t\t}""")
    delegate.write_text(text)

    shared = repository / "apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift"
    text = shared.read_text()
    # The worker canonicalizes state roots; Foundation shortens /private/tmp to
    # /tmp. Keep the private experiment's reader/writer spelling identical while
    # retaining its unchanged HOME/source spelling. This is not a product patch.
    text = replace_once(text, '.appendingPathComponent(".agentdeck", isDirectory: true)\n            .standardizedFileURL.path',
                        '.appendingPathComponent(".agentdeck", isDirectory: true)\n            .standardizedFileURL.path.replacingOccurrences(of: "/tmp/agentdeck-macos-xctest.serving.",\n                with: "/private/tmp/agentdeck-macos-xctest.serving.", options: .anchored)')
    anchor = "\tpublic func refreshQuota(manual: Bool) async -> [DesktopQuotaAlertV1] {"
    text = replace_once(text, anchor, anchor + """
\t\t// Private controlled dependency delay; not a real quota-network sample.
\t\tif ProcessInfo.processInfo.environment["AGENTDECK_SERVING_CONDITION"] == "slow-quota" {
\t\t\tlet start = DispatchTime.now().uptimeNanoseconds
\t\t\tlet home = FileManager.default.homeDirectoryForCurrentUser
\t\t\tlet marker = home.appendingPathComponent("e1-quota-delay.json")
\t\t\ttry? JSONSerialization.data(withJSONObject: ["pending": true]).write(to: marker, options: [.atomic])
\t\t\ttry? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: marker.path)
\t\t\tdo { try await Task.sleep(for: .seconds(30)) } catch { return [] }
\t\t\ttry? JSONSerialization.data(withJSONObject: ["pending": false,
\t\t\t\t"durationNanoseconds": DispatchTime.now().uptimeNanoseconds - start]).write(to: marker, options: [.atomic])
\t\t}
""")
    shared.write_text(text)
    pressure_identity = patch_rebuild_pressure(repository, digest)
    return {"probe_sha256": digest(controller), "delegate_probe_sha256": digest(delegate), "shared_probe_sha256": digest(shared),
            **pressure_identity}


def patch_rebuild_pressure(repository, digest):
    """Private maintenance execution instrumentation, in both exported arms only."""
    main = repository / "cmd/agentdeck/main.go"
    text = main.read_text()
    anchor = "\t\t\tvar data map[string]int\n\t\t\tvar warnings []string\n\t\t\terr = scanruntime.WithMaintenance(command.Context(), stateRoot, 5*time.Second, func(ctx context.Context) error {"
    text = replace_once(text, anchor, anchor.replace("\t\t\terr =", "\t\t\te1Pressure := e1NewRebuildPressure()\n\t\t\terr =")
                        + "\n\t\t\t\te1Pressure.acquired()")
    anchor = "\t\t\t\tdata, warnings, err = service.Rebuild(ctx)"
    text = replace_once(text, anchor, "\t\t\t\te1Pressure.beginProcessing()\n" + anchor + "\n\t\t\t\te1Pressure.finishProcessing(err)")
    instrumentation = repository / "cmd/agentdeck/e1_rebuild_pressure_darwin.go"
    if instrumentation.exists():
        raise RuntimeError("private_rebuild_instrumentation_already_exists")
    main.write_text(text)
    instrumentation.write_text(REBUILD_PRESSURE_GO)
    return {"pressure_main_sha256": digest(main), "pressure_go_sha256": digest(instrumentation)}


REBUILD_PRESSURE_GO = r'''//go:build darwin

package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "golang.org/x/sys/unix"
)

// The rebuild CLI owns a maintenance execution, not a scan-runtime RPC round.
// Its private round ID is minted in that process and never aliases an RPC ID.
type e1RebuildPressure struct {
    Profile string `json:"profile"`
    Clock string `json:"clock"`
    ExecutionKind string `json:"execution_kind"`
    RequestID string `json:"request_id"`
    RoundID string `json:"round_id"`
    WorkerPID int `json:"worker_pid"`
    Phase string `json:"phase"`
    Requested int64 `json:"requested_uptime_ns"`
    Acquired int64 `json:"acquired_uptime_ns"`
    Announced int64 `json:"processing_announced_uptime_ns"`
    Started int64 `json:"processing_started_uptime_ns"`
    Completed int64 `json:"processing_completed_uptime_ns"`
    QueueDuration int64 `json:"queue_duration_ns"`
    PreparationDuration int64 `json:"preparation_duration_ns"`
    ProcessingDuration int64 `json:"processing_duration_ns"`
    Succeeded bool `json:"processing_succeeded"`
    path string
}

func e1PressureUptime() int64 {
    var clock unix.Timespec
    if unix.ClockGettime(unix.CLOCK_UPTIME_RAW, &clock) != nil { return 0 }
    return clock.Nano()
}

func e1NewRebuildPressure() *e1RebuildPressure {
    path, request := os.Getenv("AGENTDECK_E1_PRESSURE_TRACE"), os.Getenv("AGENTDECK_E1_PRESSURE_REQUEST")
    // Foundation and baseline.environment use /tmp while the collector uses
    // /private/tmp. Compare physical directories without changing source HOME.
    home, err := filepath.EvalSymlinks(os.Getenv("HOME"))
    if err != nil || !filepath.IsAbs(home) ||
        !strings.HasPrefix(home, "/private/tmp/agentdeck-macos-xctest.serving.") { return nil }
    parent, err := filepath.EvalSymlinks(filepath.Dir(path))
    if err != nil || request == "" || !filepath.IsAbs(path) ||
        filepath.Base(path) != "e1-rebuild.json" || parent != home { return nil }
    path = filepath.Join(home, "e1-rebuild.json")
    now, pid := e1PressureUptime(), os.Getpid()
    p := &e1RebuildPressure{Profile: "agentdeck-e1-rebuild-pressure/v1", Clock: "mach_absolute_time_ns",
        ExecutionKind: "usage_rebuild_maintenance", RequestID: request, RoundID: fmt.Sprintf("maintenance:%d:%d", pid, now),
        WorkerPID: pid, Phase: "queued", Requested: now, path: path}
    p.write()
    return p
}

func (p *e1RebuildPressure) acquired() {
    if p == nil { return }
    p.Acquired = e1PressureUptime()
    p.QueueDuration = p.Acquired - p.Requested
    p.Phase = "preparing"
    p.write()
}

func (p *e1RebuildPressure) beginProcessing() {
    if p == nil { return }
    p.Phase, p.Announced = "processing", e1PressureUptime()
    p.write()
    // Exclude the marker write from measured processing. The final interval
    // rejects an action in this announcement-to-call gap.
    p.Started = e1PressureUptime()
    p.PreparationDuration = p.Started - p.Acquired
}

func (p *e1RebuildPressure) finishProcessing(err error) {
    if p == nil { return }
    p.Completed = e1PressureUptime()
    p.ProcessingDuration = p.Completed - p.Started
    p.Phase, p.Succeeded = "completed", err == nil
    p.write()
}

func (p *e1RebuildPressure) write() {
    data, err := json.Marshal(p)
    if err != nil { return }
    file, err := os.CreateTemp(filepath.Dir(p.path), ".e1-pressure-*")
    if err != nil { return }
    defer os.Remove(file.Name())
    if err = file.Chmod(0o600); err == nil { _, err = file.Write(data) }
    if err == nil { err = file.Sync() }
    closed := file.Close()
    if err == nil && closed == nil { _ = os.Rename(file.Name(), p.path) }
}
'''


MODEL_CLOCK = r'''
\tprivate func e1RebuildActionJSON() -> String {
\t\tguard let data = try? JSONSerialization.data(withJSONObject: e1RebuildProcessingAtFirstAction),
\t\t\tlet value = String(data: data, encoding: .utf8) else { return "" }
\t\treturn value
\t}

\tprivate func e1MarkerPending(home: URL, name: String) -> Bool {
\t\tguard let data = try? Data(contentsOf: home.appendingPathComponent(name)),
\t\t\tlet marker = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { return false }
\t\treturn marker["pending"] as? Bool == true
\t}

\tfunc e1WaitForQuotaDelay(home: URL) async {
\t\tguard ProcessInfo.processInfo.environment["AGENTDECK_SERVING_CONDITION"] == "slow-quota" else { return }
\t\tlet deadline = DispatchTime.now().uptimeNanoseconds + 40_000_000_000
\t\twhile e1MarkerPending(home: home, name: "e1-quota-delay.json"), DispatchTime.now().uptimeNanoseconds < deadline {
\t\t\ttry? await Task.sleep(for: .milliseconds(10))
\t\t}
\t}

\tprivate func e1ObserveModel(home: URL, processEntryUptime: UInt64) async {
\t\tlet deadline = DispatchTime.now().uptimeNanoseconds + 150_000_000_000
\t\twhile model.coordinator.latestSnapshot?.data.usage.available != true || model.hero == nil {
\t\t\tguard !Task.isCancelled, DispatchTime.now().uptimeNanoseconds < deadline else { return }
\t\t\tdo { try await Task.sleep(for: .milliseconds(10)) } catch { return }
\t\t}
\t\tlet ready = DispatchTime.now().uptimeNanoseconds
\t\te1ModelReadyUptime = ready
\t\tvar result: [String: Any] = ["kind": "independent-model-readiness-v1",
\t\t\t"processEntryToModelNanoseconds": ready - processEntryUptime,
\t\t\t"appReadyToModelNanoseconds": ready - e1AppReadyUptime,
\t\t\t"operatorWaitIncluded": false, "nativeFrameMeasured": false,
\t\t\t"pollIntervalNanoseconds": 10_000_000]
\t\tif let snapshot = model.coordinator.latestSnapshot {
\t\t\tresult["partial"] = snapshot.partial
\t\t\tresult["sessionDomainAvailable"] = snapshot.data.sessions.available
\t\t}
\t\tlet quota = home.appendingPathComponent("e1-quota-delay.json")
\t\tif let data = try? Data(contentsOf: quota),
\t\t\tlet marker = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
\t\t\tresult["controlledQuotaDelayPendingAtModel"] = marker["pending"] as? Bool ?? false
\t\t}
\t\tlet destination = home.appendingPathComponent("e1-model-readiness.json")
\t\ttry? JSONSerialization.data(withJSONObject: result).write(to: destination, options: [.atomic])
\t\ttry? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: destination.path)
\t}

\tfunc captureE1ModelOnly(home: URL, processEntryUptime: UInt64) async {
\t\tawait e1ObserveModel(home: home, processEntryUptime: processEntryUptime)
\t\tvar result: [String: Any] = ["version": 1, "measurement": "model-only",
\t\t\t"nativeFrameMeasured": false, "launch_latency_budget_evaluable": false]
\t\tif e1ModelReadyUptime == nil { result["failure"] = "no_usable_model_within_original_deadline" }
\t\tlet destination = home.appendingPathComponent("baseline-result.json")
\t\ttry? JSONSerialization.data(withJSONObject: result).write(to: destination, options: [.atomic])
\t\ttry? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: destination.path)
\t}
'''.replace(r"\t", "\t")


STARTUP_FRAME = r'''
\tfunc captureE1StartupFrame(home: URL, processEntryUptime: UInt64) async {
\t\tvar result: [String: Any] = ["version": 1, "measurement": "startup-frame",
\t\t\t"presentationControl": "automatic normal togglePopover after model readiness;post-show activation;not icon input",
\t\t\t"operatorWaitIncluded": false, "nativeFrameMeasured": false]
\t\tdo {
\t\t\tguard let button = statusItem.button else { throw BaselineFailure.noStatusItem }
\t\t\tlet placementDeadline = DispatchTime.now().uptimeNanoseconds + 2_000_000_000
\t\t\twhile button.window == nil {
\t\t\t\tguard DispatchTime.now().uptimeNanoseconds < placementDeadline else { throw BaselineFailure.statusWindowUnavailable }
\t\t\t\ttry await Task.sleep(for: .milliseconds(10))
\t\t\t}
\t\t\tawait e1ObserveModel(home: home, processEntryUptime: processEntryUptime)
\t\t\tguard e1ModelReadyUptime != nil, model.hero != nil else { throw BaselineFailure.noData }
\t\t\tawait Task.yield()
\t\t\t// Define the ready-state startup presentation control explicitly. Early
\t\t\t// automatic show was not reliable; do NOT reopen/retry it into success.
\t\t\ttogglePopover(button)
\t\t\tNSApp.activate(ignoringOtherApps: true)
\t\t\tMenuBarItemController.activatePopoverWindow(popover.contentViewController?.view.window)
\t\t\t// Bind the oracle after the asynchronous readiness/presentation boundary.
\t\t\t// Capture and validate one publication synchronously on the main actor;
\t\t\t// never retry a wrong bitmap or accept arbitrary numeric content.
\t\t\tguard let hero = model.hero else { throw BaselineFailure.noData }
\t\t\tlet publication = model.coordinator.latestSnapshot
\t\t\tlet frame = try baselineFrame()
\t\t\tguard model.coordinator.latestSnapshot == publication,
\t\t\t\tmodel.hero?.tokens == hero.tokens, model.hero?.amount == hero.amount else {
\t\t\t\tthrow BaselineFailure.numericFrameMissing
\t\t\t}
\t\t\t// Timestamp precedes all Vision validation. Raw pixels/text stay in RAM.
\t\t\tlet valid = try baselineFrameIsValid(frame, hero: hero)
\t\t\tguard valid else { throw BaselineFailure.numericFrameMissing }
\t\t\tresult["nativeFrameMeasured"] = true
\t\t\tresult["appReadyToNativeDataFrameNanoseconds"] = frame.end - e1AppReadyUptime
\t\t\tresult["processEntryToNativeDataFrameNanoseconds"] = frame.end - processEntryUptime
\t\t\tresult["frames"] = [["panel": "quota", "elapsedNanoseconds": frame.end - e1AppReadyUptime, "valid": true]]
\t\t} catch let error as BaselineFailure { result["failure"] = error.rawValue }
\t\tcatch { result["failure"] = "isolated_startup_frame_failed" }
\t\tlet destination = home.appendingPathComponent("baseline-result.json")
\t\ttry? JSONSerialization.data(withJSONObject: result).write(to: destination, options: [.atomic])
\t\ttry? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: destination.path)
\t\tclosePopover()
\t}
'''.replace(r"\t", "\t")


def state_identity(home, digest):
    state = home / ".agentdeck"
    return {path.name: digest(path) for path in state.iterdir()
            if path.is_file() and any(path.name == name + suffix
                                      for name in ("agentdeck.sqlite3", "sessions.sqlite3")
                                      for suffix in ("", "-wal", "-shm", "-journal"))} if state.exists() else {}


def wait_for_private_worker(home, seconds=300):
    deadline = time.monotonic() + seconds
    while (home / ".agentdeck/scan.lock").exists():
        if time.monotonic() >= deadline:
            raise RuntimeError("accepted_worker_not_idle_before_matrix_preparation")
        time.sleep(0.1)


def create_cache_seed(baseline, probe, helper, home, seed):
    """Real candidate finite bootstrap prepares the cache; no forged envelope."""
    baseline.restore_state(home, seed)
    completed = subprocess.run([str(helper), "--state-dir", str(home / ".agentdeck"), "--format", "ndjson",
                                "desktop", "bootstrap"], env=baseline.environment(home),
                               capture_output=True, timeout=300)
    if completed.returncode:
        raise RuntimeError("private_cache_seed_bootstrap_failed")
    wait_for_private_worker(home)
    cache = home / ".agentdeck" / CACHE
    snapshot = decode_cache_document(cache)
    if snapshot["header"].get("session_prepared") is not True:
        raise RuntimeError("private_full_cache_seed_not_prepared")
    # The real readonly helper independently checks generation/owner/day. Do not
    # accept a parseable-but-invalid saved file or a fallback today projection.
    checked, _ = baseline.cli(helper, home, ["desktop", "snapshot", "--wire-version", "2"])
    if checked["data"]["header"]["publication_id"] != snapshot["header"]["publication_id"]:
        raise RuntimeError("private_cache_seed_not_restored_by_real_reader")
    cache_seed = seed.parent / "seed-serving-full"
    probe.backup_databases(home / ".agentdeck", cache_seed)
    shutil.copy2(cache, cache_seed / CACHE)
    (cache_seed / CACHE).chmod(0o600)
    return cache_seed


def decode_cache_document(path):
    """The Go file wraps exact UTF-8 snapshot bytes in base64 plus SHA-256."""
    if path.stat().st_size > 8 << 20:
        raise RuntimeError("private_cache_seed_too_large")
    document = json.loads(path.read_text())
    if document.get("format_version") != 1:
        raise RuntimeError("private_cache_seed_format_mismatch")
    try:
        body = base64.b64decode(document["body"], validate=True)
    except (KeyError, TypeError, ValueError) as error:
        raise RuntimeError("private_cache_seed_invalid_body") from error
    if hashlib.sha256(body).hexdigest() != document.get("checksum"):
        raise RuntimeError("private_cache_seed_checksum_mismatch")
    snapshot = json.loads(body)
    if not isinstance(snapshot.get("header"), dict) or not isinstance(snapshot.get("payload"), dict):
        raise RuntimeError("private_cache_seed_snapshot_shape_mismatch")
    return snapshot


def validate_imported_seed(root, seed):
    if (seed.is_symlink() or seed.name != "seed-imported" or not seed.parent.name.startswith("run-")
            or seed.resolve().parent.parent != root.resolve()):
        raise RuntimeError("private_matrix_imported_seed_boundary_required")
    entries = list(seed.iterdir())
    if {path.name for path in entries} != {"agentdeck.sqlite3", "sessions.sqlite3"} or any(
            path.is_symlink() or not path.is_file() for path in entries):
        raise RuntimeError("private_matrix_imported_seed_database_set_mismatch")
    return seed


def condition_seed(condition, imported_seed, cache_seed):
    if condition not in CONDITIONS + DIAGNOSTIC_CONDITIONS:
        raise ValueError("unknown_e1_condition")
    if condition == "first-import":
        return None
    if condition in ("restore", "committed-today-no-cache", "rebuilding", "slow-quota"):
        if cache_seed is None:
            raise RuntimeError("private_valid_cache_seed_required")
        return cache_seed
    return imported_seed


def database_schema_versions(home):
    import sqlite3
    versions = {}
    for name in ("agentdeck.sqlite3", "sessions.sqlite3"):
        path = home / ".agentdeck" / name
        if path.is_file():
            with sqlite3.connect(f"file:{path}?mode=ro", uri=True) as database:
                if name == "agentdeck.sqlite3":
                    versions[name] = database.execute("SELECT version FROM schema_metadata").fetchone()[0]
                else:
                    present = database.execute("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='serving_generation'").fetchone()[0]
                    versions["sessions.serving_storage"] = database.execute(
                        "SELECT schema_version FROM serving_generation WHERE singleton=1").fetchone()[0] if present else "legacy-unversioned"
    return versions


def refresh_cache_in_place(baseline, helper, home):
    """Copies change inode-bound identity. Regenerate through the real producer.

    This is untimed preparation AFTER DB restoration, not a header/hash rewrite.
    No state copy or rename is allowed between this check and App launch.
    """
    completed = subprocess.run([str(helper), "--state-dir", str(home / ".agentdeck"), "--format", "ndjson",
                                "desktop", "bootstrap"], env=baseline.environment(home),
                               capture_output=True, timeout=300)
    if completed.returncode:
        raise RuntimeError("private_in_place_cache_preparation_failed")
    wait_for_private_worker(home)
    return validate_cache_in_place(baseline, helper, home)


def validate_cache_in_place(baseline, helper, home):
    """Validate an existing publication without producing a new generation."""
    snapshot = decode_cache_document(home / ".agentdeck" / CACHE)
    native_root = str((home / ".agentdeck").resolve())
    read = subprocess.run([str(helper), "--state-dir", native_root, "--format", "json", "desktop", "snapshot", "--wire-version", "2"],
                          env=baseline.environment(home), capture_output=True, timeout=30)
    if read.returncode:
        raise RuntimeError("private_native_cache_reader_failed")
    checked = json.loads(read.stdout)["data"]
    if checked["header"]["publication_id"] != snapshot["header"]["publication_id"] or not checked["header"]["session_prepared"]:
        raise RuntimeError("private_copied_cache_fell_back_to_today_projection")
    return {"cache_validated_after_last_state_copy": True,
            "publication_id_sha256": hashlib.sha256(snapshot["header"]["publication_id"].encode()).hexdigest(),
            "cache_sha256": hashlib.sha256((home / ".agentdeck" / CACHE).read_bytes()).hexdigest()}


class ExactCacheState:
    """Private byte checkpoint plus retained directory/DB inodes, never a rebase.

    Park the live directory while other conditions use the same HOME. Restore
    its files in place only after App cleanup and worker idle, then ask the real
    reader to validate the unchanged publication. No SQLite backup or bootstrap
    is performed between samples.
    """

    def __init__(self, baseline, helper, home, parent):
        self.baseline, self.helper, self.home = baseline, helper, home
        self.state = home / ".agentdeck"
        self.storage = parent / ("exact-cache-" + uuid.uuid4().hex)
        self.storage.mkdir(mode=0o700)
        self.parked = self.storage / "live"
        self.checkpoint = self.storage / "bytes"
        wait_for_private_worker(home)
        self.validation = validate_cache_in_place(baseline, helper, home)
        self.schemas = database_schema_versions(home)
        self.inodes = self.identity()
        if any(p.is_symlink() for p in self.state.rglob("*")):
            raise RuntimeError("private_cache_checkpoint_symlink")
        shutil.copytree(self.state, self.checkpoint)
        self.database_bytes = state_identity(home, self.digest)
        self.active = True
        self.park()

    @staticmethod
    def digest(path):
        return hashlib.sha256(path.read_bytes()).hexdigest()

    def identity(self):
        paths = (self.state, self.state / "agentdeck.sqlite3", self.state / "sessions.sqlite3")
        if any(p.is_symlink() for p in paths):
            raise RuntimeError("private_cache_checkpoint_symlink")
        return [(p.stat().st_dev, p.stat().st_ino) for p in paths]

    def park(self):
        if not self.active:
            return
        wait_for_private_worker(self.home)
        if self.identity() != self.inodes:
            raise RuntimeError("private_cache_checkpoint_inode_changed")
        self.state.rename(self.parked)
        self.active = False

    def restore(self):
        self.park()
        # This reset touches only the other condition's private state, never
        # the retained directory and databases parked beside the checkpoint.
        self.baseline.restore_state(self.home, None)
        self.parked.rename(self.state)
        self.active = True
        if self.identity() != self.inodes:
            raise RuntimeError("private_cache_checkpoint_inode_changed")
        if any(p.is_symlink() for p in self.state.rglob("*")):
            raise RuntimeError("private_cache_checkpoint_symlink")
        for path in self.state.iterdir():
            if not (self.checkpoint / path.name).exists():
                shutil.rmtree(path) if path.is_dir() else path.unlink()
        for source in self.checkpoint.iterdir():
            target = self.state / source.name
            if source.is_dir():
                if target.exists():
                    shutil.rmtree(target)
                shutil.copytree(source, target)
            else:
                # copy2 truncates the existing file, preserving its inode.
                shutil.copy2(source, target)
        validation = validate_cache_in_place(self.baseline, self.helper, self.home)
        if (self.identity() != self.inodes or validation != self.validation
                or database_schema_versions(self.home) != self.schemas
                or state_identity(self.home, self.digest) != self.database_bytes):
            raise RuntimeError("private_cache_checkpoint_restore_changed")
        return validation


def nearest_rank(values, percentile):
    if not values:
        return None
    return sorted(values)[max(0, math.ceil(len(values) * percentile) - 1)]


def cohort_identity(sample):
    """Exact bytes only; no inferred equivalence across regenerated databases."""
    def digest(value):
        return isinstance(value, str) and len(value) == 64 and all(c in "0123456789abcdef" for c in value)

    databases = sample.get("starting_database_sha256")
    versions = sample.get("starting_schema_versions")
    cached = sample.get("cache_present_at_start")
    cache_sha = sample.get("starting_cache_sha256")
    condition = sample.get("condition")
    if (not digest(sample.get("cohort_context_sha256")) or not isinstance(databases, dict)
            or not isinstance(versions, dict) or type(cached) is not bool
            or "starting_cache_sha256" not in sample
            or sample.get("measurement") not in ("native", "model", "startup-frame")):
        return None, "starting_identity_missing_or_invalid"
    names = ("agentdeck.sqlite3", "sessions.sqlite3")
    if condition == "first-import":
        if databases or versions or cached:
            return None, "first_import_state_not_empty"
    elif (not set(names).issubset(databases)
          or any(name not in {base + suffix for base in names for suffix in ("", "-wal", "-shm", "-journal")}
                 or not digest(value) for name, value in databases.items())
          or set(versions) != {"agentdeck.sqlite3", "sessions.serving_storage"}
          or type(versions["agentdeck.sqlite3"]) is not int or versions["agentdeck.sqlite3"] <= 0
          or not (versions["sessions.serving_storage"] == "legacy-unversioned"
                  or type(versions["sessions.serving_storage"]) is int and versions["sessions.serving_storage"] > 0)):
        return None, "starting_database_identity_incompatible"
    expects_cache = condition in ("restore", "rebuilding", "slow-quota")
    if condition not in CONDITIONS + DIAGNOSTIC_CONDITIONS or cached != expects_cache:
        return None, "starting_cache_condition_incompatible"
    validation = sample.get("cache_validation")
    if cached:
        if (not digest(cache_sha) or not isinstance(validation, dict)
                or validation.get("cache_validated_after_last_state_copy") is not True
                or validation.get("cache_sha256") != cache_sha
                or not digest(validation.get("publication_id_sha256"))):
            return None, "starting_cache_identity_unverified"
    elif cache_sha is not None or validation is not None:
        return None, "unexpected_starting_cache_identity"
    identity = {key: sample.get(key) for key in (
        "arm", "condition", "measurement", "cohort_context_sha256", "starting_database_sha256",
        "starting_schema_versions", "cache_present_at_start", "starting_cache_sha256", "cache_validation")}
    return hashlib.sha256(json.dumps(identity, sort_keys=True, separators=(",", ":")).encode()).hexdigest(), None


def summarize(samples, conditions, arms, required=100):
    """Never pool different starting states, clocks, arms or repeated panels."""
    groups = []
    for condition in conditions:
        for arm in arms:
            selected = [s for s in samples if s["condition"] == condition and s["arm"] == arm]
            cohorts = {}
            for sample in selected:
                key, error = cohort_identity(sample)
                cohorts.setdefault((key, error), []).append(sample)
            if not cohorts:
                cohorts[(None, "no_samples")] = []
            for (key, error), cohort in cohorts.items():
                groups.append(summarize_cohort(cohort, condition, arm, key, error, required))
    return {"groups": groups, "status": "NOT_VERIFIED",
            "reason": "Exact starting-state cohorts only; missing identities excluded. Collector summary only; pressure,oracle,P03,startup endpoints and independent gates require their own evidence."}


def summarize_cohort(selected, condition, arm, key, error, required):
    valid = [s for s in selected if error is None and not s["native"].get("failure")
             and s["native"].get("measurement") == ("model-only" if s["measurement"] == "model" else s["measurement"])
             and (condition != "rebuilding" or rebuild_pressure_established(s.get("rebuild_pressure", {}), s["native"]))]
    frames = [s for s in valid if s["measurement"] == "native" and s["native"].get("initialClickRouted") is True
              and not s["native"].get("readyClickRequired")
              and s["native"].get("frames") and s["native"]["frames"][0].get("valid") is True]
    values = [s["native"]["firstClickToDataNanoseconds"] for s in frames
              if type(s["native"].get("firstClickToDataNanoseconds")) is int
              and s["native"]["firstClickToDataNanoseconds"] > 0]
    startup_values = [s["native"]["appReadyToNativeDataFrameNanoseconds"] for s in valid
                      if s["native"].get("measurement") == "startup-frame"
                      and s["native"].get("nativeFrameMeasured") is True
                      and type(s["native"].get("appReadyToNativeDataFrameNanoseconds")) is int]
    group = {"condition": condition, "arm": arm, "cohort_sha256": key, "cohort_error": error,
                   "identity_rejections": len(selected) if error else 0, "attempts": len(selected),
                   "failures": len(selected) - len(valid), "valid_first_click_frames": len(values),
                   "model_observations": sum("model_readiness" in s for s in valid),
                   "first_click_ms": {"median": nearest_rank(values, 0.5) / 1e6 if values else None,
                                      "maximum": max(values) / 1e6 if values else None,
                                      "p95": nearest_rank(values, 0.95) / 1e6 if len(values) >= required else None,
                                      "p99": nearest_rank(values, 0.99) / 1e6 if len(values) >= required else None},
                   "tail_sample_requirement_met": len(values) >= required}
    group["startup_control_frames"] = len(startup_values)
    group["startup_control_frame_ms"] = {"median": nearest_rank(startup_values, 0.5) / 1e6 if startup_values else None,
                                               "maximum": max(startup_values) / 1e6 if startup_values else None,
                                               "p95": nearest_rank(startup_values, 0.95) / 1e6 if len(startup_values) >= required else None}
    return group


def launch_model(baseline, app, home, environment):
    arguments = ["/usr/bin/open", "-n", "-W"]
    for key, value in {**baseline.environment(home), **environment}.items():
        arguments += ["--env", f"{key}={value}"]
    arguments.append(str(app.resolve()))
    try:
        completed = subprocess.run(arguments, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=180)
    except subprocess.TimeoutExpired:
        return {"failure": "standalone_model_observation_timeout"}
    result = home / "baseline-result.json"
    if completed.returncode or not result.is_file():
        return {"failure": "standalone_model_observation_missing"}
    return json.loads(result.read_text())


class RebuildPressure:
    """A real private CLI rebuild, never a sleep relabeled as scanner work."""
    def __init__(self, helper, home, baseline, probe):
        self.helper, self.home, self.baseline, self.probe = helper, home, baseline, probe
        self.process = None
        self.started = None
        self.log = None
        self.completed_ns = None
        self.request_id = uuid.uuid4().hex
        self.marker = self.home / "e1-rebuild.json"

    def observe(self):
        if self.process is None and (self.home / "e1-model-readiness.json").is_file():
            self.marker.unlink(missing_ok=True)
            self.log = (self.home / f"e1-rebuild-{self.request_id}.log").open("xb")
            Path(self.log.name).chmod(0o600)
            self.started = time.monotonic_ns()
            self.process = subprocess.Popen([str(self.helper), "--state-dir", str(self.home / ".agentdeck"),
                                             "--format", "json", "usage", "rebuild"],
                                            env={**self.baseline.environment(self.home),
                                                 "AGENTDECK_E1_PRESSURE_TRACE": str(self.marker),
                                                 "AGENTDECK_E1_PRESSURE_REQUEST": self.request_id}, stdout=self.log, stderr=self.log)
        if self.process is not None and self.process.poll() is not None and self.completed_ns is None:
            self.completed_ns = time.monotonic_ns()

    def finish(self):
        if self.process is None:
            return {"failure": "rebuild_not_started_after_model_readiness"}
        try:
            self.process.wait(timeout=300)
            self.observe()
            try:
                trace = json.loads(self.marker.read_text())
            except (OSError, ValueError):
                trace = {}
            if (not isinstance(trace, dict) or trace.get("request_id") != self.request_id
                    or trace.get("worker_pid") != self.process.pid):
                return {"failure": "rebuild_processing_trace_missing_or_identity_mismatch",
                        "exit_status": self.process.returncode}
            return {**trace, "method": "real_private_usage_rebuild",
                    "exit_status": self.process.returncode, "whole_cli_duration_ns": self.completed_ns - self.started}
        except subprocess.TimeoutExpired:
            # Never reset an accepted worker's state on timeout. The caller stops.
            return {"failure": "private_rebuild_still_running_do_not_reset"}
        finally:
            self.log.close()


def rebuild_pressure_established(stress, native):
    """Admit only a successful, identified processing interval containing the action."""
    if (not isinstance(stress, dict) or not isinstance(native, dict) or stress.get("failure")
            or type(stress.get("exit_status")) is not int or stress["exit_status"] != 0):
        return False
    if (stress.get("profile") != "agentdeck-e1-rebuild-pressure/v1" or stress.get("clock") != "mach_absolute_time_ns"
            or stress.get("execution_kind") != "usage_rebuild_maintenance" or stress.get("phase") != "completed"
            or stress.get("processing_succeeded") is not True):
        return False
    encoded_marker = native.get("rebuildProcessingAtFirstActionJSON")
    if not isinstance(encoded_marker, str):
        return False
    try:
        marker = json.loads(encoded_marker)
    except ValueError:
        return False
    if not isinstance(marker, dict) or marker.get("phase") != "processing":
        return False
    for key in ("profile", "clock", "execution_kind", "request_id", "round_id", "worker_pid", "processing_announced_uptime_ns"):
        if marker.get(key) != stress.get(key) or type(marker.get(key)) is not type(stress.get(key)):
            return False
    if (not all(isinstance(stress.get(key), str) and stress[key] for key in ("request_id", "round_id"))
            or type(stress.get("worker_pid")) is not int or stress["worker_pid"] <= 0):
        return False
    times = ("requested_uptime_ns", "acquired_uptime_ns", "processing_announced_uptime_ns",
             "processing_started_uptime_ns", "processing_completed_uptime_ns")
    if not all(type(stress.get(key)) is int and stress[key] > 0 for key in times):
        return False
    requested, acquired, announced, started, completed = (stress[key] for key in times)
    action = marker.get("action_uptime_ns")
    if not (requested <= acquired <= announced <= started < completed and type(action) is int and started <= action < completed):
        return False
    for key, expected in (("queue_duration_ns", acquired - requested), ("preparation_duration_ns", started - acquired),
                          ("processing_duration_ns", completed - started)):
        if type(stress.get(key)) is not int or stress[key] != expected:
            return False
    return stress["processing_duration_ns"] > 120_000_000_000


def run_matrix(args, baseline, probe, apps, home, imported_seed, report):
    conditions = args.conditions or list(CONDITIONS)
    wait_for_private_worker(home)
    cache_seed = None
    exact_cache = None
    if any(c in conditions for c in ("restore", "committed-today-no-cache", "rebuilding", "slow-quota")):
        cache_seed = create_cache_seed(baseline, probe, apps["B"] / "Contents/Helpers/agentdeck", home, imported_seed)
        if any(c in conditions for c in ("restore", "rebuilding", "slow-quota")):
            exact_cache = ExactCacheState(baseline, apps["B"] / "Contents/Helpers/agentdeck", home, cache_seed.parent)
    report["matrix"] = {"conditions": conditions, "measurement": args.measurement,
                        "quota_pressure": "controlled cancellable 30s dependency delay,not real network",
                        "rebuild_pressure": "identified private maintenance processing >120s;action within worker interval;queue/setup/CLI wall separate",
                        "no_cache_precondition": "current candidate-schema committed DB;cache removed without changing facts",
                        "upgrade_control": "upgrade-no-cache is a separate optional diagnostic from the original imported schema",
                        "baseline_restore_compatibility": "not assumed;all arms receive identical prepared DB/cache;schema-ahead failures remain failures"}
    business_day = datetime.now(ZoneInfo("America/New_York")).date().isoformat()
    report["matrix"]["business_date"] = business_day
    report["matrix"]["cohort_policy"] = "exact DB/sidecar/cache/schema and input/build/probe/environment/control identity; stop before launch on drift; no logical normalization"
    admitted = {}
    for previous in report["samples"]:
        key, error = cohort_identity(previous)
        series = (previous["condition"], previous["arm"])
        if error or series in admitted and admitted[series] != key:
            raise RuntimeError("existing_matrix_cohort_unverified")
        admitted[series] = key
    probe.put(args.output, report)
    for pair in range(args.pairs):
        for condition in conditions if pair % 2 == 0 else reversed(conditions):
            for arm in (("A", "B") if pair % 2 == 0 else ("B", "A")):
                if arm not in args.arms:
                    continue
                if datetime.now(ZoneInfo("America/New_York")).date().isoformat() != business_day:
                    report["failure"] = "business_day_changed_remeasure_in_one_window"
                    probe.put(args.output, report)
                    raise SystemExit(1)
                wait_for_private_worker(home)
                if exact_cache:
                    exact_cache.park()
                cache_validation = None
                if condition in ("restore", "rebuilding", "slow-quota"):
                    cache_validation = exact_cache.restore()
                else:
                    baseline.restore_state(home, condition_seed(condition, imported_seed, cache_seed))
                if condition == "committed-today-no-cache":
                    (home / ".agentdeck" / CACHE).unlink(missing_ok=True)
                for name in ("e1-model-readiness.json", "e1-quota-delay.json"):
                    (home / name).unlink(missing_ok=True)
                environment = {"AGENTDECK_SERVING_CONDITION": condition,
                               "AGENTDECK_SERVING_MEASUREMENT": args.measurement}
                starting_versions = database_schema_versions(home)
                starting = state_identity(home, probe.sha)
                cache_present = (home / ".agentdeck" / CACHE).is_file()
                context = {"input": report["input"], "identity": report["identity"][arm],
                           "environment": report["environment"], "matrix": report["matrix"],
                           "click_dispatch": report.get("click_dispatch"), "click_precondition": report.get("click_precondition"),
                           "warm_samples": args.warm_samples, "manual_wait_seconds": args.manual_wait_seconds}
                sample = {"pair": pair, "arm": arm, "condition": condition, "measurement": args.measurement,
                          "starting_database_sha256": starting, "starting_schema_versions": starting_versions,
                          "cache_present_at_start": cache_present,
                          "starting_cache_sha256": probe.sha(home / ".agentdeck" / CACHE) if cache_present else None,
                          "cohort_context_sha256": hashlib.sha256(json.dumps(context, sort_keys=True, separators=(",", ":")).encode()).hexdigest()}
                if cache_validation is not None:
                    sample["cache_validation"] = cache_validation
                cohort, identity_error = cohort_identity(sample)
                series = (condition, arm)
                if identity_error or series in admitted and admitted[series] != cohort:
                    sample["native"] = {"failure": identity_error or "starting_cohort_changed_before_launch",
                                        "measurement_started": False}
                    report["samples"].append(sample)
                    report["failure"] = sample["native"]["failure"]
                    report["summary"] = summarize(report["samples"], conditions, args.arms)
                    probe.put(args.output, report)
                    raise SystemExit(1)
                admitted[series] = cohort
                started = time.monotonic_ns()
                pressure = RebuildPressure(apps[arm] / "Contents/Helpers/agentdeck", home, baseline, probe) if condition == "rebuilding" else None
                if args.measurement in ("model", "startup-frame"):
                    result = launch_model(baseline, apps[arm], home, environment)
                else:
                    # Preserve the original environment function for other samples.
                    original = baseline.environment
                    baseline.environment = lambda h, original=original: {**original(h), **environment}
                    try:
                        result = probe.launch_manual(baseline, apps[arm], home, arm, args.warm_samples, args.manual_wait_seconds,
                                                     observer=pressure.observe if pressure else None)
                    finally:
                        baseline.environment = original
                sample.update(native=result, whole_observation_ns=time.monotonic_ns() - started)
                metrics = home / "e1-model-readiness.json"
                if metrics.is_file():
                    sample["model_readiness"] = json.loads(metrics.read_text())
                elif args.measurement in ("model", "startup-frame"):
                    result["failure"] = result.get("failure") or "independent_model_clock_missing"
                if pressure:
                    sample["rebuild_pressure"] = pressure.finish()
                sample["cache_present_at_start"] = cache_present
                result["private_app_cleanup_verified"] = probe.stop_private_app(apps[arm])
                if not result["private_app_cleanup_verified"]:
                    result["failure"] = "private_app_cleanup_unverified"
                # No fake success for a condition that hasn't established its stress.
                if condition == "rebuilding":
                    stress = sample["rebuild_pressure"]
                    if not rebuild_pressure_established(stress, result):
                        result["failure"] = result.get("failure") or "rebuild_pressure_condition_not_established"
                if condition == "slow-quota":
                    marker = home / "e1-quota-delay.json"
                    sample["controlled_quota_delay"] = json.loads(marker.read_text()) if marker.is_file() else {}
                    if args.measurement == "native" and (
                            sample["controlled_quota_delay"].get("durationNanoseconds", 0) < 30_000_000_000
                            or not result.get("nativeEnvironment", {}).get("quotaPressureAtFirstAction")):
                        result["failure"] = result.get("failure") or "quota_pressure_condition_not_established"
                report["samples"].append(sample)
                report["summary"] = summarize(report["samples"], conditions, args.arms)
                if (probe.sha(Path(probe.__file__)) != report["environment"]["harness_sha256"]
                        or probe.sha(Path(__file__)) != report["environment"]["matrix_module_sha256"]):
                    report["failure"] = "matrix_source_changed_during_observation"
                probe.put(args.output, report)
                print(json.dumps({"pair": pair, "condition": condition, "arm": arm,
                                  "failure": result.get("failure"), "model_readiness": sample.get("model_readiness")}), flush=True)
                if result.get("failure") or report.get("failure"):
                    raise SystemExit(1)
