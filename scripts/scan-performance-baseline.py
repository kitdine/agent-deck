#!/usr/bin/env python3
"""E0 baseline: isolated standalone App launch, native popover and source metrics.

Captures only the named JSONL roots when --capture-authorized is supplied.
Private inputs, databases and helper output remain under a mode-0700 scratch root.
The exported report contains hashes, timings, counters and classified failures.
"""
import argparse
import hashlib
import json
import os
import platform
import re
from pathlib import Path
import shutil
import sqlite3
import subprocess
import tempfile
import time
import uuid
from datetime import datetime, timedelta
from zoneinfo import ZoneInfo

SOURCE_ROOTS = (".codex/sessions", ".codex/archived_sessions", ".claude/projects")
CLICK_STIMULUS = "App-local posted leftMouseDown/leftMouseUp at status button; activation after initial routing; not global physical input"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")
    path.chmod(0o600)


def file_hash(path):
    value = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def environment(home):
    # Foundation standardizedFileURL uses /tmp for /private/tmp. The worker
    # compares source-home strings, so preparation and App must use one alias.
    home = Path(str(home).replace("/private/tmp/", "/tmp/", 1))
    return {"HOME": str(home), "CFFIXED_USER_HOME": str(home), "CODEX_HOME": str(home / ".codex"),
            "XDG_STATE_HOME": str(home / ".local/state"), "XDG_CONFIG_HOME": str(home / ".config"),
            "PATH": "/usr/bin:/bin", "LANG": "en_US_POSIX", "LC_ALL": "en_US_POSIX", "TZ": "America/New_York",
            "TMPDIR": str(home / "tmp"), "AGENTDECK_TEST_HOME": str(home), "AGENTDECK_TEST_LOCALE": "en",
            "AGENTDECK_NATIVE_BASELINE": "1", "AGENTDECK_SCAN_PROFILE": "1"}


def cli(helper, home, arguments):
    start = time.monotonic_ns()
    output = subprocess.run([str(helper), "--state-dir", str(home / ".agentdeck"), "--format", "json", *arguments],
                            env=environment(home), capture_output=True, timeout=300)
    if output.returncode:
        raise RuntimeError("isolated_helper_failed")
    value = json.loads(output.stdout)
    # stdout keeps its stable v1 shape; classified profiling is opt-in stderr.
    for line in output.stderr.splitlines():
        try:
            profile = json.loads(line)
        except ValueError:
            continue
        if profile.get("type") == "scan_performance":
            value["baseline_profile"] = profile["stages"]
    return value, (time.monotonic_ns() - start) / 1e6


def prepare_directories(home):
    for relative in (*SOURCE_ROOTS, "tmp", ".local/state", ".config"):
        (home / relative).mkdir(parents=True, exist_ok=True, mode=0o700)


def backup_state(home, destination):
    destination.mkdir(mode=0o700)
    state = home / ".agentdeck"
    for source in state.glob("*.sqlite3"):
        with sqlite3.connect(f"file:{source}?mode=ro", uri=True) as original, sqlite3.connect(destination / source.name) as backup:
            original.backup(backup)
    key = state / "credential.key"
    if key.exists():
        shutil.copy2(key, destination / key.name)


def restore_state(home, seed):
    # Only this harness's private generated state is reset. Sources keep one
    # absolute path across every seed/sample; relocation would force rereads.
    state = home / ".agentdeck"
    # The current finite worker intentionally survives a disconnected waiter.
    # A native helper's 120s timeout does NOT release its accepted scan or lock.
    # Never reset a fixture database while that work is still writing it.
    deadline = time.monotonic() + 300
    while (state / "scan.lock").exists():
        if time.monotonic() >= deadline:
            raise RuntimeError("accepted_worker_not_idle_before_fixture_reset")
        time.sleep(0.1)
    if state.exists():
        shutil.rmtree(state)
    if seed:
        shutil.copytree(seed, state)
    for relative in ("app-group", "baseline-result.json"):
        target = home / relative
        if target.is_dir():
            shutil.rmtree(target)
        elif target.exists():
            target.unlink()


def launch(app, home):
    start = time.monotonic_ns()
    executed_identity = {"executed_app_sha256": file_hash(app / "Contents/MacOS/AgentDeck"),
                         "executed_helper_sha256": file_hash(app / "Contents/Helpers/agentdeck")}
    # LaunchServices supplies the same application context as a normal App
    # launch. Direct executable launches can sit in an unlaunched AppKit loop.
    arguments = ["/usr/bin/open", "-n", "-W"]
    for key, value in environment(home).items():
        arguments += ["--env", f"{key}={value}"]
    arguments.append(str(app.resolve()))
    try:
        output = subprocess.run(arguments, env=environment(home),
                                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=180)
    except subprocess.TimeoutExpired:
        return {"failure": "standalone_app_timeout", "timeout_seconds": 180, **executed_identity}
    result_path = home / "baseline-result.json"
    if output.returncode or not result_path.is_file():
        return {"failure": "standalone_app_failed_or_no_result", "process_exit_status": output.returncode, **executed_identity}
    result = json.loads(result_path.read_text())
    if result.get("clickStimulus") != CLICK_STIMULUS:
        result["failure"] = "native_click_stimulus_mismatch"
    result.update(executed_identity)
    result["whole_process_ms"] = (time.monotonic_ns() - start) / 1e6
    # Whole process includes warm clicks, OCR and delta: NEVER label it launch latency.
    result["whole_process_is_launch_latency"] = False
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--app", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--pairs", type=int, default=3)
    parser.add_argument("--corpus-home", type=Path)
    parser.add_argument("--capture-authorized", action="store_true")
    parser.add_argument("--synthetic-mib", type=int, default=8)
    parser.add_argument("--root", type=Path)
    parser.add_argument("--cli-only", action="store_true", help="Collect engine baselines without claiming a native frame")
    parser.add_argument("--reuse-corpus", action="store_true", help="Reuse only this harness's private frozen JSONL, never recapture live inputs")
    parser.add_argument("--conditions", nargs="+", choices=["empty", "today_imported", "history_today_missing", "saved_display"],
                        help="Restrict a changed-scope verification; default keeps the four-condition baseline")
    args = parser.parse_args()
    if not 1 <= args.pairs <= 20:
        parser.error("pairs must be in 1..20")
    if args.corpus_home and not args.capture_authorized:
        parser.error("live corpus requires --capture-authorized")
    os.umask(0o077)
    root = args.root or Path(tempfile.mkdtemp(prefix="agentdeck-macos-xctest.baseline.", dir="/private/tmp"))
    if root.is_symlink():
        parser.error("symlink scratch root rejected")
    root = root.resolve()
    if not str(root).startswith("/private/tmp/agentdeck-macos-xctest.baseline."):
        parser.error("private baseline scratch root required")
    root.mkdir(mode=0o700, exist_ok=True)
    root.chmod(0o700)
    helper = args.app.resolve() / "Contents/Helpers/agentdeck"
    frozen = root / "home"
    if frozen.exists() and not args.reuse_corpus:
        parser.error("root must be unused; never overwrite prior evidence")
    prepare_directories(frozen)
    inventory = json.loads((root / "private-inventory.json").read_text()) if args.reuse_corpus else []
    if args.reuse_corpus:
        inventory = [item for item in inventory if item["relative"] != ".codex/sessions/baseline-today.jsonl"]
    elif args.corpus_home:
        for relative_root in SOURCE_ROOTS:
            source_root = args.corpus_home / relative_root
            if not source_root.exists():
                continue
            for source in sorted(source_root.rglob("*.jsonl")):
                if source.is_symlink() or not source.is_file():
                    continue
                relative = source.relative_to(args.corpus_home)
                target = frozen / relative
                target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                before = source.stat()
                remaining = before.st_size
                with source.open("rb") as input_stream, target.open("xb") as output_stream:
                    while remaining:
                        block = input_stream.read(min(remaining, 1024 * 1024))
                        if not block:
                            raise RuntimeError("source_truncated_during_capture")
                        output_stream.write(block)
                        remaining -= len(block)
                os.utime(target, ns=(before.st_atime_ns, before.st_mtime_ns))
                inventory.append({"relative": str(relative), "bytes": before.st_size,
                                  "mtime_ns": before.st_mtime_ns, "sha256": file_hash(target)})
    else:
        # Deterministic old sources exercise full parsing; this is a synthetic
        # smoke workload, never presented as the operator's representative corpus.
        padding = json.dumps({"type": "response_item", "timestamp": "2026-01-01T12:00:00Z",
                              "payload": {"type": "message", "role": "assistant", "content": "x" * 8000}}) + "\n"
        for index in range(8):
            target = frozen / ".codex/sessions" / f"old-{index}.jsonl"
            target.write_text(json.dumps({"type": "session_meta", "payload": {"id": f"old-{index}", "cwd": "/synthetic"}}) + "\n" +
                              padding * max(1, args.synthetic_mib * 1024 * 1024 // (8 * len(padding))))
            os.utime(target, (1767268800, 1767268800))
            inventory.append({"relative": str(target.relative_to(frozen)), "bytes": target.stat().st_size,
                              "mtime_ns": target.stat().st_mtime_ns, "sha256": file_hash(target)})
    # Finish earlier accepted work before changing the controlled source.
    deadline = time.monotonic() + 300
    while (frozen / ".agentdeck/scan.lock").exists():
        if time.monotonic() >= deadline:
            raise RuntimeError("accepted_worker_not_idle_before_delta_reset")
        time.sleep(0.1)
    timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    def event(tokens, total):
        return json.dumps({"type": "event_msg", "timestamp": timestamp, "payload": {"type": "token_count", "info": {
            "last_token_usage": {"input_tokens": tokens, "output_tokens": 10},
            "total_token_usage": {"input_tokens": total, "output_tokens": 10}}}}) + "\n"
    today = frozen / ".codex/sessions/baseline-today.jsonl"
    today.write_text(json.dumps({"type": "session_meta", "timestamp": timestamp, "payload": {"id": "baseline-today", "cwd": "/synthetic"}}) + "\n" +
                     json.dumps({"type": "turn_context", "timestamp": timestamp, "payload": {"turn_id": "baseline-turn", "model": "gpt-5"}}) + "\n" + event(1000, 1000))
    (frozen / "baseline-delta.jsonl").write_text(event(10_000_000, 10_001_000))
    inventory.append({"relative": str(today.relative_to(frozen)), "bytes": today.stat().st_size,
                      "mtime_ns": today.stat().st_mtime_ns, "sha256": file_hash(today)})
    write_json(root / "private-inventory.json", inventory)
    report = {"kind": "scan-performance-e0-v1", "input": {
        "kind": "reused_frozen_capture_plus_synthetic_delta_source" if args.reuse_corpus else ("captured_jsonl_plus_synthetic_delta_source" if args.corpus_home else "synthetic_smoke"),
        "files": len(inventory), "bytes": sum(x["bytes"] for x in inventory),
        "corpus_sha256": digest("\n".join(sorted(f'{x["bytes"]}:{x["sha256"]}' for x in inventory)).encode()),
        "digest_recipe": "SHA256(sorted size:content-SHA256 joined by LF without final LF)",
        "delta_sha256": file_hash(frozen / "baseline-delta.jsonl"), "frozen_at_utc": timestamp,
        "capture_consistency": "per-file bounded prefix at its initial size; not a global filesystem snapshot"},
        "identity": {"helper_sha256": file_hash(helper), "app_executable_sha256": file_hash(args.app / "Contents/MacOS/AgentDeck"),
                     "harness_sha256": file_hash(Path(__file__)), "timezone": "America/New_York", "locale": "en",
                     "default_panel": "quota", "otel": "off"}, "samples": [], "probes": [],
        "limitations": ["Pilot baseline; not E1 or >=100 native-tail acceptance",
                        "Native endpoint is visible NSPopover AppKit bitmap draw, validated by in-memory numeric OCR; not compositor photon timing",
                        "Polling contributes up to 10ms to first data readiness; update old/new bitmaps and endpoints are captured before either OCR validation",
                        "No configured official quota credentials in the isolated home; default quota surface is exercised, no live quota network timing",
                        "Saved display uses the current App Group publication; current App does not restore it at startup",
                        "Parallel worker durations overlap and must not be summed",
                        "Page cache is not purged; worker peak RSS is lifetime peak",
                        "Synthetic delta refresh is explicit; watcher/event delivery belongs to later tasks"]}
    repository = Path(__file__).resolve().parents[1]
    identity_files = ["go.mod", "go.sum", "vendor/modules.txt", "apps/macos/Package.swift", "apps/macos/AgentDeck.xcodeproj/project.pbxproj",
                      "cmd/agentdeck/main.go", "internal/scanruntime/scanruntime.go", "internal/scanruntime/profile_test.go", "internal/ingest/ingest.go",
                      "internal/ingest/metrics.go", "internal/usage/usage.go", "internal/session/session.go", "internal/store/store.go",
                      "internal/desktop/desktop.go", "internal/desktop/derived_cache.go", "apps/macos/AgentDeckShared/DesktopLog.swift",
                      "apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift", "apps/macos/AgentDeckApp/AgentDeckApp.swift",
                      "apps/macos/AgentDeckApp/MenuBarItemController.swift", "apps/macos/AgentDeckApp/MenuBarViewModel.swift"]
    report["identity"]["head"] = subprocess.check_output(["git", "-C", str(repository), "rev-parse", "HEAD"], text=True).strip()
    report["identity"]["files_sha256"] = {relative: file_hash(repository / relative) for relative in identity_files}
    report["identity"]["macos"] = platform.mac_ver()[0]
    report["identity"]["architecture"] = platform.machine()
    report["identity"]["toolchain"] = {name: subprocess.check_output(command, text=True, stderr=subprocess.DEVNULL).strip()
                                        for name, command in {"go": ["go", "version"], "xcode": ["xcodebuild", "-version"],
                                                              "swift": ["swift", "--version"]}.items()}
    report["identity"]["worker_profile"] = "AGENTDECK_SCAN_PROFILE=1; aggregate atomic counters, no source identifiers"
    report["identity"]["home_alias"] = "Foundation-standardized /tmp alias for CLI and App; one fixed source path across samples"
    report["identity"]["debug_delegate_lifetime"] = "retained through NSApplication.run in isolated baseline mode"
    report["identity"]["launch_transport"] = "LaunchServices open -n -W with explicit isolated environment"
    report["identity"]["click_stimulus"] = CLICK_STIMULUS
    versions = {}
    for name, relative, constant in [("usage_parser", "internal/usage/usage.go", "ParserVersion"),
                                     ("session_parser", "internal/session/session.go", "ParserVersion"),
                                     ("core_schema", "internal/store/store.go", "CurrentSchemaVersion"),
                                     ("session_schema", "internal/store/store.go", "CurrentSessionSchemaVersion")]:
        match = re.search(r"\b" + constant + r"\s*=\s*(\d+)", (repository / relative).read_text())
        if match:
            versions[name] = int(match.group(1))
    report["identity"]["versions"] = versions
    window_start = datetime.now(ZoneInfo("America/New_York")).replace(hour=0, minute=0, second=0, microsecond=0)
    report["identity"]["business_window"] = {"start": window_start.isoformat(), "end": (window_start + timedelta(days=1)).isoformat()}
    batch = root / ("batch-" + uuid.uuid4().hex)
    batch.mkdir(mode=0o700)
    original_full = root / "seed-full"
    if args.reuse_corpus and original_full.exists():
        restore_state(frozen, original_full)
    full = batch / "seed-full"
    scanned, elapsed = cli(helper, frozen, ["scan"])
    report["probes"].append({"condition": "empty", "elapsed_ms": elapsed, "scan": scanned["baseline_profile"]})
    snapshot, snapshot_ms = cli(helper, frozen, ["desktop", "snapshot"])
    report["probes"].append({"condition": "today_imported_readonly", "elapsed_ms": snapshot_ms,
                             "snapshot_sha256": digest(json.dumps(snapshot, sort_keys=True).encode())})
    report["identity"]["database_schemas"] = {}
    for file in (frozen / ".agentdeck").glob("*.sqlite3"):
        with sqlite3.connect(f"file:{file}?mode=ro", uri=True) as database:
            schema = database.execute("SELECT type,name,sql FROM sqlite_schema WHERE sql IS NOT NULL ORDER BY type,name").fetchall()
            report["identity"]["database_schemas"][file.name] = {
                "pragma_user_version": database.execute("PRAGMA user_version").fetchone()[0],
                "ddl_sha256": digest(json.dumps(schema, separators=(",", ":")).encode())}
    # Produce the valid last display through the real App before cloning it.
    delta = frozen / "baseline-delta.jsonl"
    delta.rename(frozen / "disabled-delta.jsonl")
    saved_seed = {"not_run": "cli_only_no_native_claim"} if args.cli_only else launch(args.app, frozen)
    report["saved_display_seed"] = saved_seed
    if saved_seed.get("failure"):
        write_json(args.output, report)
        raise SystemExit(1)
    (frozen / "disabled-delta.jsonl").rename(delta)
    backup_state(frozen, full)
    saved_group = batch / "saved-app-group"
    if (frozen / "app-group").exists():
        shutil.copytree(frozen / "app-group", saved_group)
    historical = batch / "seed-history"
    original_history = root / "seed-history"
    if args.reuse_corpus and original_history.exists():
        restore_state(frozen, original_history)
    else:
        restore_state(frozen, None)
    # Hints only construct an isolated missing-today DB; they never narrow a
    # production scan inventory. Restored paths participate in every timed scan.
    local_now = datetime.now(ZoneInfo("America/New_York"))
    day_start = local_now.replace(hour=0, minute=0, second=0, microsecond=0).timestamp()
    moved = []
    for item in inventory:
        if item["mtime_ns"] / 1e9 >= day_start:
            source = frozen / item["relative"]
            stash = batch / "stash" / item["relative"]
            stash.parent.mkdir(parents=True, exist_ok=True)
            source.rename(stash)
            moved.append((stash, source))
    cli(helper, frozen, ["scan"])
    backup_state(frozen, historical)
    for stash, source in moved:
        stash.rename(source)
    last_condition = "imported_warm_engine" if args.cli_only else "saved_display"
    seeds = {"empty": None, "today_imported": full, "history_today_missing": historical, last_condition: full}
    if args.conditions:
        seeds = {condition: seed for condition, seed in seeds.items() if condition in args.conditions}
    original_today = today.read_bytes()
    today_mtime = today.stat().st_mtime_ns
    for pair in range(args.pairs):
        conditions = list(seeds)
        if pair % 2:
            conditions.reverse()
        for condition in conditions:
            if datetime.now(ZoneInfo("America/New_York")).date() != window_start.date():
                report["failure"] = "business_day_changed_remeasure_in_one_window"
                write_json(args.output, report)
                raise SystemExit(1)
            home = frozen
            # restore_state's quiescence barrier precedes every source, database
            # and display mutation, including after a native helper timeout.
            restore_state(home, seeds[condition])
            today.write_bytes(original_today)
            os.utime(today, ns=(today_mtime, today_mtime))
            # Normalize source metadata after the controlled append rollback;
            # this untimed step is used only for already-imported conditions.
            if condition in ("today_imported", "saved_display", "imported_warm_engine"):
                cli(helper, home, ["scan"])
            if condition == "saved_display" and saved_group.exists():
                shutil.copytree(saved_group, home / "app-group")
            for cache in (home / ".agentdeck").glob("desktop-derived-cache*"):
                cache.unlink()
            starting_state = {file.name: file_hash(file) for file in (home / ".agentdeck").glob("*.sqlite3")}
            if args.cli_only:
                scanned, elapsed = cli(helper, home, ["scan"])
                snapshot, snapshot_ms = cli(helper, home, ["desktop", "snapshot"])
                result = {"native_not_run": True, "scan_ms": elapsed, "snapshot_ms": snapshot_ms,
                          "data_ready_ms": elapsed + snapshot_ms, "scan": scanned["baseline_profile"],
                          "snapshot_sha256": digest(json.dumps(snapshot, sort_keys=True).encode())}
                with today.open("ab") as stream:
                    stream.write(delta.read_bytes())
                updated, update_ms = cli(helper, home, ["scan"])
                latest, latest_ms = cli(helper, home, ["desktop", "snapshot"])
                result["update"] = {"scan_ms": update_ms, "snapshot_ms": latest_ms, "data_ready_ms": update_ms + latest_ms,
                                    "scan": updated["baseline_profile"],
                                    "usage_tokens_changed": latest["data"]["usage"]["tokens"] != snapshot["data"]["usage"]["tokens"]}
                if not result["update"]["usage_tokens_changed"]:
                    result["failure"] = "delta_did_not_change_committed_tokens"
            else:
                result = launch(args.app, home)
            report["samples"].append({"pair": pair, "condition": condition, "starting_database_sha256": starting_state, **result})
            write_json(args.output, report)
            print(json.dumps({"pair": pair, "condition": condition, "failure": result.get("failure"),
                              "launch_to_data_ns": result.get("launchToDataNanoseconds")}), flush=True)
    write_json(args.output, report)
    if any(sample.get("failure") for sample in report["samples"]):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
