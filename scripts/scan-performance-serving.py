#!/usr/bin/env python3
"""E1 private native A/B harness; never modifies live sources or client settings."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import plistlib
import shutil
import sqlite3
import subprocess
import tempfile
import time
import uuid

import importlib.util
import scan_performance_serving_matrix as matrix

ROOTS = (".codex/sessions", ".codex/archived_sessions", ".claude/projects")
REPO = Path(__file__).resolve().parents[1]


def sha(path):
    value = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            value.update(block)
    return value.hexdigest()


def put(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")
    path.chmod(0o600)


def capture(home, destination):
    """Frozen per-file prefixes, not a claimed global filesystem snapshot."""
    inventory = []
    for relative_root in ROOTS:
        source_root = home / relative_root
        if not source_root.exists():
            continue
        for source in sorted(source_root.rglob("*.jsonl")):
            if source.is_symlink() or not source.is_file():
                continue
            before = source.stat()
            target = destination / source.relative_to(home)
            target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            with source.open("rb") as input_stream, target.open("xb") as output_stream:
                remaining = before.st_size
                while remaining:
                    block = input_stream.read(min(remaining, 1 << 20))
                    if not block:
                        raise RuntimeError("source_truncated_during_capture")
                    output_stream.write(block)
                    remaining -= len(block)
            target.chmod(0o600)
            os.utime(target, ns=(before.st_atime_ns, before.st_mtime_ns))
            inventory.append({"relative": str(source.relative_to(home)), "bytes": before.st_size,
                              "mtime_ns": before.st_mtime_ns, "sha256": sha(target)})
    return inventory


def backup_databases(source, destination):
    destination.mkdir(mode=0o700, exist_ok=True)
    result = {}
    # Exactly the approved databases. No credential.key or authentication files.
    for name in ("agentdeck.sqlite3", "sessions.sqlite3"):
        original = source / name
        if not original.is_file() or original.is_symlink():
            continue
        with sqlite3.connect(f"file:{original}?mode=ro", uri=True) as reader, sqlite3.connect(destination / name) as writer:
            reader.backup(writer)
        (destination / name).chmod(0o600)
        result[name] = sha(destination / name)
    return result


def run(command, cwd, log, environment=None, timeout=None):
    with log.open("wb") as output:
        completed = subprocess.run(command, cwd=cwd, env=environment, stdout=output,
                                   stderr=subprocess.STDOUT, timeout=timeout)
    log.chmod(0o600)
    if completed.returncode:
        raise RuntimeError("private_command_failed; inspect private log locally")


def export_sources(destination, candidate):
    destination.mkdir(mode=0o700)
    if not candidate:
        archive = subprocess.Popen(["git", "archive", "HEAD"], cwd=REPO, stdout=subprocess.PIPE)
        unpack = subprocess.run(["tar", "-x", "-C", str(destination)], stdin=archive.stdout)
        archive.stdout.close()
        if archive.wait() or unpack.returncode:
            raise RuntimeError("private_head_export_failed")
        return
    # Include uncommitted production/test inputs, never another mutable index,
    # runtime configuration, build output, or repository history.
    for relative in ("internal", "cmd", "vendor", "desktop", "go.mod", "go.sum", "Makefile", "apps/macos", "scripts"):
        source, target = REPO / relative, destination / relative
        if source.is_dir():
            shutil.copytree(source, target, ignore=shutil.ignore_patterns("build", "__pycache__", ".DS_Store"))
        else:
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, target)


def click_stimulus(original, activate_before_click=False, click_dispatch="posted"):
    if click_dispatch == "manual":
        if activate_before_click:
            raise ValueError("manual_click_cannot_pre_activate")
        return "Operator physical click at isolated TEST status button; timing starts at native button action entry; no event injection"
    if activate_before_click:
        original = original.replace("activation after initial routing; not global physical input",
                                    "verified active App before first injection; not inactive or global physical input")
    if click_dispatch == "window":
        original = original.replace("App-local posted leftMouseDown/leftMouseUp at status button",
                                    "App-local NSWindow.sendEvent leftMouseDown with queued leftMouseUp at status button")
    elif click_dispatch == "global":
        original = original.replace("App-local posted leftMouseDown/leftMouseUp at status button",
                                    "WindowServer CGEvent leftMouseDown/leftMouseUp at isolated status button")
        original = original.replace("not global physical input", "synthetic global input, not a physical mouse").replace(
            "not inactive or global physical input", "synthetic global input, not a physical mouse")
    elif click_dispatch != "posted":
        raise ValueError("unsupported_click_dispatch")
    return original


def patch_manual_probe(text, wait_seconds):
    """Change only the exported probe, never the product's click handling."""
    replacements = {
        "\tprivate var baselineActionCount = 0": """\tprivate var baselineActionCount = 0
\tprivate var baselineManualClickArmed = false
\tprivate var baselineManualClickUptime: UInt64?
\tprivate var baselineManualFirstFrame: (end: UInt64, image: CGImage)?
\tprivate var baselineManualFirstFrameHero: MenuBarHero?
\tprivate var baselineManualResumeAfterAction: UInt64?
\tprivate var baselineManualHome: URL?
\tprivate var baselineManualRequests = 0
\tprivate var baselineManualClicks = 0
\tprivate var baselineManualWaitNanoseconds: UInt64 = 0
\tprivate var baselineManualDataReadyAtFirstClick = false""",
        'button.title = model.menuBarText ?? ""':
            'button.title = "TEST " + (ProcessInfo.processInfo.environment["AGENTDECK_MANUAL_ARM"] ?? "")\n'
            '\t\tbutton.toolTip = "Manual measurement: click TEST once when the harness requests it."',
        "\t\tif wantsMenu {": """\t\tif baselineManualClickArmed, !wantsMenu, !popover.isShown,
\t\t\tlet event, event.type == .leftMouseUp || event.type == .leftMouseDown {
\t\t\t// Action-entry timing excludes the operator's reaction/movement time.
\t\t\tbaselineManualClickUptime = DispatchTime.now().uptimeNanoseconds
\t\t\tbaselineManualClicks += 1
\t\t\tif baselineManualClicks == 1 {
\t\t\t\tbaselineManualDataReadyAtFirstClick = model.hero != nil
\t\t\t\t\t&& model.coordinator.latestSnapshot?.data.usage.available == true
\t\t\t}
\t\t\tbaselineManualClickArmed = false
\t\t}
\t\tif wantsMenu {""",
        "\t\tlet initialDataPresent = model.hero != nil":
            "\t\tbaselineManualHome = home\n\t\tlet initialDataPresent = model.hero != nil",
        "\t\ttogglePopover(sender)\n": """\t\ttogglePopover(sender)
\t\t// Capture the first eligible visible frame in the normal action turn.
\t\t// Waiting for the probe Task to resume can add unrelated run-loop delay.
\t\t// OCR remains after the timed endpoint; loading clicks keep their ready path.
\t\tif baselineManualClicks == 1, baselineManualFirstFrame == nil,
\t\t\tmodel.coordinator.latestSnapshot?.data.usage.available == true, let hero = model.hero {
\t\t\tif let frame = try? baselineFrame() {
\t\t\t\tbaselineManualFirstFrame = frame
\t\t\t\tbaselineManualFirstFrameHero = hero
\t\t\t}
\t\t}
""",
        "let firstFrame = try baselineFrame()":
            "let firstFrame = try baselineManualFirstFrame ?? baselineFrame()",
        "guard try baselineFrameIsValid(firstFrame, hero: hero)":
            "guard try baselineFrameIsValid(firstFrame, hero: baselineManualFirstFrameHero ?? hero)",
        "\t\t\tlet firstClickToDataNanoseconds: UInt64?": """\t\t\tlet firstClickToDataNanoseconds: UInt64?
\t\t\tlet operatorWaitNanoseconds: UInt64
\t\t\tlet manualClickCount: Int
\t\t\tlet firstClickDataReady: Bool
\t\t\tlet clickTimingOrigin: String
\t\t\tlet firstFrameCaptureOrigin: String
\t\t\tlet probeResumeAfterActionNanoseconds: UInt64?""",
        "\t\t\tfirstClickUptime = DispatchTime.now().uptimeNanoseconds\n\t\t\tdo { try await baselineClick(button); initialClickRouted = true }":
            "\t\t\tdo { try await baselineClick(button); firstClickUptime = baselineManualClickUptime; initialClickRouted = true }",
        "\t\t\t\tlet start = DispatchTime.now().uptimeNanoseconds\n\t\t\t\ttry await baselineClick(button)":
            "\t\t\t\ttry await baselineClick(button)\n\t\t\t\tguard let start = baselineManualClickUptime else { throw BaselineFailure.clickNotRouted }",
        "firstClickToDataNanoseconds: firstClickToData, initialDataPresent: initialDataPresent,":
            'firstClickToDataNanoseconds: firstClickToData, operatorWaitNanoseconds: baselineManualWaitNanoseconds,\n'
            '\t\t\tmanualClickCount: baselineManualClicks, firstClickDataReady: baselineManualDataReadyAtFirstClick,\n'
            '\t\t\tclickTimingOrigin: "native_left_mouse_button_action_entry",\n'
            '\t\t\tfirstFrameCaptureOrigin: baselineManualFirstFrame == nil ? "ready_probe_path" : "native_action_return",\n'
            '\t\t\tprobeResumeAfterActionNanoseconds: baselineManualResumeAfterAction, initialDataPresent: initialDataPresent,',
        '\t\tcase clickNotRouted = "native_mouse_event_not_routed"':
            '\t\tcase clickNotRouted = "native_mouse_event_not_routed"\n\t\tcase manualClickTimeout = "operator_click_timeout"\n\t\tcase manualInputBeforeReady = "manual_input_before_first_request"',
    }
    for original, replacement in replacements.items():
        if text.count(original) != 1:
            raise RuntimeError("private_manual_probe_anchor_mismatch")
        text = text.replace(original, replacement)
    start = text.index("\tprivate func baselineClick(_ button: NSStatusBarButton) async throws {")
    end = text.index("\n\tprivate func baselineFrame()", start)
    text = text[:start] + """\tprivate func baselineClick(_ button: NSStatusBarButton) async throws {
\t\tguard let window = button.window, window.isVisible, button.isEnabled,
\t\t\tlet home = baselineManualHome else { throw BaselineFailure.statusWindowUnavailable }
\t\tguard baselineManualRequests != 0 || baselineActionCount == 0 else {
\t\t\t// Never call a later armed click the first click after early input.
\t\t\tthrow BaselineFailure.manualInputBeforeReady
\t\t}
\t\tbaselineManualClickUptime = nil
\t\tbaselineManualClickArmed = false
\t\tbaselineManualRequests += 1
\t\tlet instance = UUID().uuidString
\t\tlet progress: [String: Any] = ["request": baselineManualRequests,
\t\t\t"instance": instance, "panel": model.selectedPanel.rawValue,
\t\t\t"status": baselineManualRequests == 1 ? "awaiting_visibility" : "waiting_for_click"]
\t\tlet destination = home.appendingPathComponent("manual-click-progress.json")
\t\ttry JSONSerialization.data(withJSONObject: progress).write(to: destination, options: [.atomic])
\t\ttry FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: destination.path)
\t\tlet waitStarted = DispatchTime.now().uptimeNanoseconds
\t\tlet deadline = waitStarted + """ + str(wait_seconds * 1_000_000_000) + """
\t\t// Keep this SAME process alive while the operator locates its icon.
\t\t// A real click is accepted even before a visibility acknowledgement.
\t\tlet visibilityDeadline = waitStarted + 600_000_000_000
\t\tvar operatorDeadline: UInt64? = baselineManualRequests == 1 ? nil : deadline
\t\tbaselineManualClickArmed = true
\t\tdefer { baselineManualClickArmed = false }
\t\twhile baselineManualClickUptime == nil || !popover.isShown {
\t\t\tif operatorDeadline == nil,
\t\t\t\tlet data = try? Data(contentsOf: home.appendingPathComponent("manual-visible.json")),
\t\t\t\tlet signal = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
\t\t\t\tsignal["instance"] as? String == instance {
\t\t\t\toperatorDeadline = DispatchTime.now().uptimeNanoseconds + """ + str(wait_seconds * 1_000_000_000) + """
\t\t\t\tvar ready = progress
\t\t\t\tready["status"] = "waiting_for_click"
\t\t\t\ttry JSONSerialization.data(withJSONObject: ready).write(to: destination, options: [.atomic])
\t\t\t}
\t\t\tguard DispatchTime.now().uptimeNanoseconds < (operatorDeadline ?? visibilityDeadline) else {
\t\t\t\tthrow BaselineFailure.manualClickTimeout
\t\t\t}
\t\t\ttry await Task.sleep(for: .milliseconds(10))
\t\t}
\t\tguard let clicked = baselineManualClickUptime else { throw BaselineFailure.clickNotRouted }
\t\tbaselineManualResumeAfterAction = DispatchTime.now().uptimeNanoseconds - clicked
\t\tbaselineManualWaitNanoseconds += clicked - waitStarted
\t}
""" + text[end:]
    return text


def patch_probe(repository, warm_samples, activate_before_click=False, click_dispatch="posted", require_first_click=True,
                manual_wait_seconds=120):
    if click_dispatch == "manual" and (activate_before_click or not require_first_click):
        raise ValueError("manual_click_requires_unmodified_first_click")
    if not 5 <= manual_wait_seconds <= 600:
        raise ValueError("bounded_manual_wait_required")
    path = repository / "apps/macos/AgentDeckApp/MenuBarItemController.swift"
    text = path.read_text()
    replacements = {
        "while model.coordinator.latestSnapshot == nil {":
            "while model.coordinator.latestSnapshot?.data.usage.available != true || model.hero == nil {",
        "snapshot.data.usage.available, snapshot.data.sessions.available else":
            "snapshot.data.usage.available else",
        "for panel in MenuBarPanel.allCases {":
            "for panel in Array(repeating: MenuBarPanel.allCases, count: " + str(warm_samples) + ").flatMap({ $0 }) {",
        "let valid = try baselineFrameIsValid(frame, hero: hero)":
            "let valid = try baselineFrameIsValid(frame, hero: model.hero ?? hero)",
        "\t\t\tlet launchToDataNanoseconds: UInt64?":
            "\t\t\tlet launchToDataNanoseconds: UInt64?\n\t\t\tlet firstClickToDataNanoseconds: UInt64?",
        "\t\tlet initialDataPresent = model.hero != nil":
            "\t\tvar firstClickUptime: UInt64?\n\t\tvar firstClickToData: UInt64?\n\t\tlet initialDataPresent = model.hero != nil",
        "\t\t\tdo { try await baselineClick(button) }":
            "\t\t\tfirstClickUptime = DispatchTime.now().uptimeNanoseconds\n\t\t\tdo { try await baselineClick(button); initialClickRouted = true }",
        "\t\tvar initialClickRouted = true":
            "\t\tvar initialClickRouted = false",
        "\tprivate let openSettings: () -> Void":
            "\tprivate var baselineActionCount = 0\n\tprivate let openSettings: () -> Void",
        "\t\tlet event = NSApp.currentEvent":
            "\t\tbaselineActionCount += 1\n\t\tlet event = NSApp.currentEvent",
        "\t\tvar readyClickRequired = false":
            "\t\tvar readyClickRequired = false\n\t\tvar firstActionReached = false\n\t\tvar firstButtonVisible = false\n\t\tvar firstAppActive = false\n\t\tvar firstEventPostingAllowed = false",
        "\t\t\t// First click uses the status item's normal action, including the empty surface.":
            "\t\t\tfirstButtonVisible = button.window?.isVisible == true\n\t\t\tfirstAppActive = NSApp.isActive\n\t\t\tfirstEventPostingAllowed = CGPreflightPostEventAccess()\n\t\t\t// First click uses the status item's normal action, including the empty surface.",
        "\t\t\tcatch BaselineFailure.clickNotRouted { initialClickRouted = false }":
            "\t\t\tcatch BaselineFailure.clickNotRouted { initialClickRouted = false }\n\t\t\tfirstActionReached = baselineActionCount > 0",
        '"appIsActive": NSApp.isActive]':
            '"appIsActive": NSApp.isActive, "firstAppActive": firstAppActive, "firstButtonVisible": firstButtonVisible, "firstActionReached": firstActionReached, "firstEventPostingAllowed": firstEventPostingAllowed]',
        "\t\t\tlaunchToData = firstFrame.end - processEntryUptime":
            "\t\t\tlaunchToData = firstFrame.end - processEntryUptime\n\t\t\tif initialClickRouted, !readyClickRequired, let firstClickUptime { firstClickToData = firstFrame.end - firstClickUptime }",
        "\t\t\tlaunchToDataNanoseconds: launchToData, initialDataPresent: initialDataPresent,":
            "\t\t\tlaunchToDataNanoseconds: launchToData, firstClickToDataNanoseconds: firstClickToData, initialDataPresent: initialDataPresent,",
    }
    for original, replacement in replacements.items():
        if text.count(original) != 1:
            raise RuntimeError("private_probe_anchor_mismatch")
        text = text.replace(original, replacement)
    if require_first_click:
        original = "catch BaselineFailure.clickNotRouted { initialClickRouted = false }"
        text = text.replace(original, "catch BaselineFailure.clickNotRouted { firstActionReached = baselineActionCount > 0; throw BaselineFailure.clickNotRouted }")
    event_loop = """\t\tfor type in [NSEvent.EventType.leftMouseDown, .leftMouseUp] {
\t\t\tguard let event = NSEvent.mouseEvent(with: type, location: point, modifierFlags: [],
\t\t\t\ttimestamp: ProcessInfo.processInfo.systemUptime, windowNumber: window.windowNumber,
\t\t\t\tcontext: nil, eventNumber: 1, clickCount: 1, pressure: 1) else { throw BaselineFailure.clickNotRouted }
\t\t\tNSApp.postEvent(event, atStart: false)
\t\t}"""
    if click_dispatch == "window":
        replacement = """\t\tlet events = [NSEvent.EventType.leftMouseDown, .leftMouseUp].compactMap { type in
\t\t\tNSEvent.mouseEvent(with: type, location: point, modifierFlags: [],
\t\t\t\ttimestamp: ProcessInfo.processInfo.systemUptime, windowNumber: window.windowNumber,
\t\t\t\tcontext: nil, eventNumber: 1, clickCount: 1, pressure: 1)
\t\t}
\t\tguard events.count == 2 else { throw BaselineFailure.clickNotRouted }
\t\t// mouseDown may enter NSButton's synchronous tracking loop. Its matching
\t\t// mouseUp must already be queued, rather than dispatched after that returns.
\t\tNSApp.postEvent(events[1], atStart: true)
\t\twindow.sendEvent(events[0])"""
        if text.count(event_loop) != 1:
            raise RuntimeError("private_dispatch_probe_anchor_mismatch")
        text = text.replace(event_loop, replacement)
    elif click_dispatch == "global":
        replacement = """\t\t// Never request permissions, activate another App, or click an unplaced item.
\t\tguard CGPreflightPostEventAccess(), window.isVisible, button.isEnabled,
\t\t\tlet screen = window.screen, let primaryScreen = NSScreen.screens.first,
\t\t\tbutton.bounds.width > 0, button.bounds.height > 0 else { throw BaselineFailure.clickNotRouted }
\t\tlet screenPoint = window.convertPoint(toScreen: point)
\t\tguard screen.frame.contains(screenPoint) else { throw BaselineFailure.clickNotRouted }
\t\tlet target = CGPoint(x: screenPoint.x, y: primaryScreen.frame.maxY - screenPoint.y)
\t\tfor type in [CGEventType.leftMouseDown, .leftMouseUp] {
\t\t\tguard let event = CGEvent(mouseEventSource: nil, mouseType: type,
\t\t\t\tmouseCursorPosition: target, mouseButton: .left) else { throw BaselineFailure.clickNotRouted }
\t\t\tevent.setIntegerValueField(.mouseEventClickState, value: 1)
\t\t\tevent.post(tap: .cghidEventTap)
\t\t}"""
        if text.count(event_loop) != 1:
            raise RuntimeError("private_global_dispatch_anchor_mismatch")
        text = text.replace(event_loop, replacement)
    elif click_dispatch not in ("posted", "manual"):
        raise ValueError("unsupported_click_dispatch")
    if activate_before_click:
        original = "\t\t\t// First click uses the status item's normal action, including the empty surface."
        replacement = """\t\t\t// App-local injection needs an active application; this is a distinct
\t\t\t// stimulus from an inactive physical menu-bar click.
\t\t\tNSApp.activate(ignoringOtherApps: true)
\t\t\tlet inputDeadline = DispatchTime.now().uptimeNanoseconds + 2_000_000_000
\t\t\twhile !NSApp.isActive {
\t\t\t\tguard DispatchTime.now().uptimeNanoseconds < inputDeadline else {
\t\t\t\t\tthrow BaselineFailure.activationUnavailable
\t\t\t\t}
\t\t\t\ttry await Task.sleep(for: .milliseconds(10))
\t\t\t}
""" + original
        if text.count(original) != 1:
            raise RuntimeError("private_activation_probe_anchor_mismatch")
        text = text.replace(original, replacement)
    original = "App-local posted leftMouseDown/leftMouseUp at status button; activation after initial routing; not global physical input"
    if text.count(original) != 1:
        raise RuntimeError("private_click_stimulus_anchor_mismatch")
    text = text.replace(original, click_stimulus(original, activate_before_click, click_dispatch))
    if click_dispatch == "manual":
        text = patch_manual_probe(text, manual_wait_seconds)
    path.write_text(text)
    return sha(path)


def unique_bundle(app, suffix):
    for info in (app / "Contents/Info.plist", app / "Contents/PlugIns/AgentDeckWidget.appex/Contents/Info.plist"):
        with info.open("rb") as stream:
            document = plistlib.load(stream)
        document["CFBundleIdentifier"] += ".serving." + suffix
        with info.open("wb") as stream:
            plistlib.dump(document, stream)


def stop_private_app(app):
    """Terminate only this export's unique bundle and matching executable."""
    with (app / "Contents/Info.plist").open("rb") as stream:
        identity = plistlib.load(stream)["CFBundleIdentifier"]
    if ".serving." not in identity:
        raise RuntimeError("private_bundle_identity_required_for_cleanup")
    code = r'''import AppKit
import Foundation
let identity = CommandLine.arguments[1]
let expected = URL(fileURLWithPath: CommandLine.arguments[2]).resolvingSymlinksInPath().path
let apps = NSRunningApplication.runningApplications(withBundleIdentifier: identity).filter {
    $0.executableURL?.resolvingSymlinksInPath().path == expected
}
for app in apps { _ = app.terminate() }
let deadline = Date().addingTimeInterval(2)
while Date() < deadline && apps.contains(where: { !$0.isTerminated }) {
    RunLoop.current.run(until: Date().addingTimeInterval(0.05))
}
for app in apps where !app.isTerminated { _ = app.forceTerminate() }
let forcedDeadline = Date().addingTimeInterval(3)
while Date() < forcedDeadline && apps.contains(where: { !$0.isTerminated }) {
    RunLoop.current.run(until: Date().addingTimeInterval(0.05))
}
let remaining = NSRunningApplication.runningApplications(withBundleIdentifier: identity).filter {
    $0.executableURL?.resolvingSymlinksInPath().path == expected && !$0.isTerminated
}
print(remaining.isEmpty ? "private_app_stopped" : "private_app_cleanup_unverified")
exit(remaining.isEmpty ? 0 : 1)
'''
    completed = subprocess.run(["swift", "-e", code, identity, str(app / "Contents/MacOS/AgentDeck")],
                               capture_output=True, text=True, timeout=30)
    return completed.returncode == 0 and completed.stdout.strip() == "private_app_stopped"


def reusable_apps(root, reference, warm_samples, activate_before_click, click_dispatch, require_first_click, apps_dir=None,
                  manual_wait_seconds=120, matrix_probe=False):
    """Reuse the exact private bundles the operator granted, never new identities."""
    if (reference.get("kind") != "scan-performance-e1-v1" or reference.get("click_dispatch") != click_dispatch
            or reference.get("require_first_click") != require_first_click):
        raise RuntimeError("private_app_reuse_control_mismatch")
    if click_dispatch == "manual" and reference.get("manual_wait_seconds") != manual_wait_seconds:
        raise RuntimeError("private_app_reuse_manual_wait_changed")
    if bool(reference.get("matrix_probe")) != matrix_probe:
        raise RuntimeError("private_app_reuse_matrix_control_changed")
    apps = {}
    if apps_dir is not None:
        allowed = (REPO / "output/scan-performance-e1-permission").resolve()
        if apps_dir.is_symlink() or not apps_dir.resolve().is_relative_to(allowed):
            raise RuntimeError("private_app_reuse_output_directory_required")
    artifacts = {"app_sha256": "Contents/MacOS/AgentDeck", "helper_sha256": "Contents/Helpers/agentdeck",
                 "framework_sha256": "Contents/Frameworks/AgentDeckShared.framework/Versions/A/AgentDeckShared",
                 "widget_sha256": "Contents/PlugIns/AgentDeckWidget.appex/Contents/MacOS/AgentDeckWidget"}
    controller = "apps/macos/AgentDeckApp/MenuBarItemController.swift"
    for label, candidate in (("A", False), ("B", True)):
        identity = reference["identity"][label]
        if not all(identity.get(key) for key in artifacts):
            raise RuntimeError("private_app_reuse_missing_artifact_identity")
        # A source change must not silently reuse an older B binary. Python
        # orchestration changes do not alter the already compiled App/probe.
        if candidate:
            for relative, expected in identity["source_files_sha256"].items():
                if Path(relative).is_absolute() or ".." in Path(relative).parts:
                    raise RuntimeError("private_app_reuse_invalid_source_path")
                if Path(relative).suffix not in (".go", ".swift", ".xcstrings", ".plist", ".pbxproj") and relative not in (
                        "go.mod", "go.sum", "vendor/modules.txt", "Makefile", "apps/macos/Package.swift"):
                    continue
                path = REPO / relative
                if not path.is_file() or sha(path) != expected:
                    raise RuntimeError("private_app_reuse_production_source_changed")
        source = (REPO / controller).read_text() if candidate else subprocess.check_output(
            ["git", "show", reference["environment"]["head"] + ":" + controller], cwd=REPO, text=True)
        with tempfile.TemporaryDirectory(prefix="probe-reuse.", dir=root) as directory:
            scratch = Path(directory)
            path = scratch / controller
            path.parent.mkdir(parents=True)
            path.write_text(source)
            expected_probe = patch_probe(scratch, warm_samples, activate_before_click, click_dispatch, require_first_click,
                                          manual_wait_seconds)
            if matrix_probe:
                # Reconstruct every matrix patch input from this arm's source.
                # The exported Go pressure probe is part of the compiled identity too.
                for relative in ("apps/macos/AgentDeckApp/AgentDeckApp.swift", "apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift",
                                 "cmd/agentdeck/main.go"):
                    target = scratch / relative
                    target.parent.mkdir(parents=True, exist_ok=True)
                    contents = (REPO / relative).read_text() if candidate else subprocess.check_output(
                        ["git", "show", reference["environment"]["head"] + ":" + relative], cwd=REPO, text=True)
                    target.write_text(contents)
                expected = matrix.patch_matrix_probe(scratch, sha)
                expected_probe = expected["probe_sha256"]
                if any(identity.get(key) != value for key, value in expected.items()):
                    raise RuntimeError("private_app_reuse_matrix_probe_changed")
        if expected_probe != identity["probe_sha256"]:
            raise RuntimeError("private_app_reuse_probe_changed")
        matches = []
        candidates = ([apps_dir / label / "AgentDeck.app"] if apps_dir is not None else
                      [batch / ("repo-" + label) / "apps/macos/build/DerivedData/Build/Products/Debug/AgentDeck.app"
                       for batch in root.glob("run-*")])
        boundary = apps_dir.resolve() if apps_dir is not None else root.resolve()
        for app in candidates:
            if app.is_symlink() or not app.is_dir() or not app.resolve().is_relative_to(boundary):
                continue
            if all((app / relative).is_file() and sha(app / relative) == identity[key]
                   for key, relative in artifacts.items()):
                with (app / "Contents/Info.plist").open("rb") as stream:
                    bundle_id = plistlib.load(stream)["CFBundleIdentifier"]
                if ".serving." in bundle_id:
                    matches.append(app)
        if len(matches) != 1:
            raise RuntimeError("private_app_reuse_identity_not_unique")
        apps[label] = matches[0]
    return apps


def launch_manual(baseline, app, home, arm, warm_samples, wait_seconds, observer=None):
    """Operator waits are bounded separately and never enter click latency."""
    progress_path = home / "manual-click-progress.json"
    progress_path.unlink(missing_ok=True)
    (home / "manual-visible.json").unlink(missing_ok=True)
    environment = {**baseline.environment(home), "AGENTDECK_MANUAL_ARM": arm}
    arguments = ["/usr/bin/open", "-n", "-W"]
    for key, value in environment.items():
        arguments += ["--env", f"{key}={value}"]
    arguments.append(str(app.resolve()))
    start = time.monotonic_ns()
    identity = {"executed_app_sha256": sha(app / "Contents/MacOS/AgentDeck"),
                "executed_helper_sha256": sha(app / "Contents/Helpers/agentdeck")}
    # Native data preparation retains its existing 150s deadline. Extra time
    # belongs only to the declared operator waits, not a product timeout change.
    timeout = 180 + 600 + (1 + 5 * warm_samples) * wait_seconds
    process = subprocess.Popen(arguments, env=environment, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    seen_request = None
    deadline = time.monotonic() + timeout
    while process.poll() is None:
        if observer is not None:
            observer()
        if time.monotonic() >= deadline:
            process.terminate()
            process.wait(timeout=5)
            return {"failure": "manual_session_timeout", "timeout_seconds": timeout, **identity}
        if progress_path.is_file():
            progress = json.loads(progress_path.read_text())
            current = (progress.get("request"), progress.get("status"))
            if current != seen_request:
                seen_request = current
                print(json.dumps({"manual_click": "waiting", "arm": arm, "request": progress.get("request"),
                                  "panel": progress.get("panel"), "status": progress.get("status"),
                                  "instruction": "Single-click TEST " + arm}), flush=True)
        time.sleep(0.1)
    result_path = home / "baseline-result.json"
    if process.returncode or not result_path.is_file():
        return {"failure": "standalone_app_failed_or_no_result", "process_exit_status": process.returncode, **identity}
    result = json.loads(result_path.read_text())
    # This reader owns the native click path. Its Swift output predates the
    # matrix discriminator; preserve explicit clocks so mismatches stay rejected.
    result.setdefault("measurement", "native")
    if result.get("clickStimulus") != baseline.CLICK_STIMULUS:
        result["failure"] = "native_click_stimulus_mismatch"
    result.update(identity)
    result["whole_process_ms"] = (time.monotonic_ns() - start) / 1e6
    result["whole_process_is_launch_latency"] = False
    result["launch_includes_operator_wait"] = True
    result["launch_latency_budget_evaluable"] = False
    return result


def confirm_manual_visibility(home):
    """Acknowledge one live request, not another process or a later click."""
    progress = json.loads((home / "manual-click-progress.json").read_text())
    if progress.get("status") != "awaiting_visibility" or not progress.get("instance"):
        raise RuntimeError("private_manual_visibility_request_not_pending")
    put(home / "manual-visible.json", {"instance": progress["instance"]})
    return {"request": progress["request"], "status": "visibility_confirmed_no_restart"}


def reusable_seed(root, reference, report):
    """Keep a completed arm reusable by restoring its exact starting databases."""
    for key in ("kind", "input", "identity", "click_dispatch", "require_first_click", "manual_wait_seconds"):
        if reference.get(key) != report.get(key):
            raise RuntimeError("private_seed_reuse_control_or_identity_changed")
    for key in ("head", "macos", "architecture", "timezone", "locale", "otel", "toolchain"):
        if reference.get("environment", {}).get(key) != report.get("environment", {}).get(key):
            raise RuntimeError("private_seed_reuse_environment_changed")
    successful = [sample for sample in reference.get("samples", [])
                  if not sample.get("native", {}).get("failure")
                  and any(frame.get("valid") for frame in sample.get("native", {}).get("frames", []))]
    if not successful:
        raise RuntimeError("private_seed_reuse_needs_completed_arm")
    expected = successful[0]["starting_database_sha256"]
    if set(expected) != {"agentdeck.sqlite3", "sessions.sqlite3"}:
        raise RuntimeError("private_seed_reuse_database_set_changed")
    if any(sample.get("starting_database_sha256") != expected for sample in reference.get("samples", [])):
        raise RuntimeError("private_seed_reuse_mixed_starting_states")
    for seed in sorted(root.glob("run-*/seed-imported")):
        if seed.is_symlink() or not seed.resolve().is_relative_to(root.resolve()):
            continue
        files = list(seed.glob("*.sqlite3"))
        if any(path.is_symlink() for path in files):
            continue
        if {path.name: sha(path) for path in files} == expected:
            return seed
    raise RuntimeError("private_seed_reuse_exact_starting_state_missing")


def main():
    harness_hash = sha(Path(__file__))
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--capture-authorized", action="store_true")
    parser.add_argument("--corpus-home", type=Path)
    parser.add_argument("--frozen-input-root", type=Path, help="Clone a verified earlier frozen corpus; never recapture live HOME")
    parser.add_argument("--input-provenance", type=Path, help="Retain verified same-corpus origin/synthetic declarations while rebuilding a changed probe")
    parser.add_argument("--today-sentinel", action="store_true", help="Declare one controlled current-day source separately from the real corpus")
    parser.add_argument("--root", type=Path)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--pairs", type=int, default=3)
    parser.add_argument("--warm-samples", type=int, default=1)
    parser.add_argument("--first-click-only", action="store_true", help="Collect one original action/frame per launch; panel repeats are a separate sample set")
    parser.add_argument("--matrix", action="store_true", help="Use the six-condition runner and independent model clock")
    parser.add_argument("--conditions", nargs="+", choices=matrix.CONDITIONS + matrix.DIAGNOSTIC_CONDITIONS)
    parser.add_argument("--measurement", choices=("native", "model", "startup-frame"), default="native",
                        help="model never measures a frame; startup-frame automatically presents the real popover, never measures icon input")
    parser.add_argument("--confirm-visible", action="store_true", help="Acknowledge the pending first manual request without restarting its App")
    parser.add_argument("--arms", nargs="+", choices=("A", "B"), default=["A", "B"],
                        help="Run only selected arms; retain a completed arm instead of asking the operator to repeat it")
    parser.add_argument("--reuse-seed-from-report", type=Path,
                        help="Restore the exact verified starting databases of a prior completed arm; reject input/artifact/control drift")
    parser.add_argument("--imported-seed", type=Path,
                        help="Untimed matrix preparation from this root's original imported DB pair; scan revalidates it, never reuses latency")
    parser.add_argument("--capture-only", action="store_true")
    parser.add_argument("--prepare-apps-only", action="store_true", help="Build frozen private A/B Apps without launching or timing them")
    parser.add_argument("--reuse-corpus", action="store_true", help="Reuse only this harness's frozen inputs; do not recapture live data")
    parser.add_argument("--activate-before-click", action="store_true", help="Separate active-App injection control; not inactive-user first-click acceptance")
    parser.add_argument("--click-dispatch", choices=("posted", "window", "global", "manual"), default="posted",
                        help="manual waits for operator physical clicks; global uses already-allowed CGEvent input")
    parser.add_argument("--manual-wait-seconds", type=int, default=120, help="Bound each operator wait separately from measured click latency (5..600)")
    parser.add_argument("--allow-ready-click-control", action="store_true",
                        help="Explicit diagnostic only: keep a lost first click and separate ready-click retry; never first-click acceptance")
    parser.add_argument("--reuse-apps-from-report", type=Path,
                        help="Reuse verified private App artifacts and their existing permission identities; refuse code/probe/control drift")
    parser.add_argument("--apps-dir", type=Path,
                        help="Explicit generated project output directory containing the same verified A/B permission bundles")
    args = parser.parse_args()
    if args.first_click_only:
        args.warm_samples = 0
    if args.confirm_visible:
        if not args.root or args.root.is_symlink() or not str(args.root.resolve()).startswith("/private/tmp/agentdeck-macos-xctest.serving."):
            parser.error("visibility acknowledgement requires the exact private fixture root")
        print(json.dumps(confirm_manual_visibility(args.root / "home")))
        return
    if args.output is None:
        parser.error("--output is required for a new observation")
    args.matrix = args.matrix or bool(args.conditions) or args.measurement != "native"
    if args.matrix and args.click_dispatch != "manual":
        parser.error("matrix native input requires manual; model observations do not inject events")
    if args.measurement != "native" and any(c in ("warm", "rebuilding", "slow-quota") for c in (args.conditions or matrix.CONDITIONS)):
        parser.error("warm/pressure require native icon frames, not a model or auto-presentation substitution")
    if args.conditions and len(args.conditions) != len(set(args.conditions)):
        parser.error("duplicate conditions are not samples")
    if args.frozen_input_root and (args.corpus_home or args.reuse_corpus):
        parser.error("frozen cloning, live capture and in-place reuse are distinct")
    if args.input_provenance and not args.reuse_corpus:
        parser.error("input provenance requires an independently verified reused corpus")
    if args.today_sentinel and (not args.frozen_input_root or not args.matrix):
        parser.error("controlled today source requires a new matrix clone, never mutation of an earlier capture")
    if args.matrix and (args.reuse_seed_from_report or args.allow_ready_click_control or args.activate_before_click):
        parser.error("matrix conditions cannot reuse a seed from another condition or diagnostic first-click retry")
    if args.imported_seed and (not args.matrix or not args.reuse_corpus or args.prepare_apps_only):
        parser.error("imported seed is only for actual matrix preparation with this verified frozen corpus")
    if len(args.arms) != len(set(args.arms)):
        parser.error("--arms must not contain duplicates")
    if args.reuse_seed_from_report and (not args.reuse_corpus or not args.reuse_apps_from_report
                                       or args.capture_only or args.prepare_apps_only):
        parser.error("seed reuse requires an actual run with the frozen corpus and verified prior App artifacts")
    if args.click_dispatch == "manual" and (args.activate_before_click or args.allow_ready_click_control):
        parser.error("manual input requires the original physical first click; activation/retry controls are incompatible")
    if not 5 <= args.manual_wait_seconds <= 600:
        parser.error("--manual-wait-seconds must be in 5..600")
    if args.capture_only and args.prepare_apps_only:
        parser.error("capture-only and prepare-apps-only are distinct boundaries")
    if args.corpus_home and not args.capture_authorized:
        parser.error("live capture needs explicit authorization")
    if not 1 <= args.pairs <= 100 or not (1 <= args.warm_samples <= 100 or (args.first_click_only and args.warm_samples == 0)):
        parser.error("bounded pair/sample counts required")
    if args.output.exists():
        parser.error("new output required; never overwrite a prior observation")
    if args.apps_dir is not None and not args.reuse_apps_from_report:
        parser.error("--apps-dir requires an exact prior artifact report")
    os.umask(0o077)
    root = args.root or Path(tempfile.mkdtemp(prefix="agentdeck-macos-xctest.serving.", dir="/private/tmp"))
    if root.is_symlink() or not str(root.resolve()).startswith("/private/tmp/agentdeck-macos-xctest.serving."):
        parser.error("private serving fixture root required")
    root = root.resolve()
    root.mkdir(mode=0o700, exist_ok=True)
    home = root / "home"
    if home.exists() and not args.reuse_corpus:
        parser.error("new fixture root required; never overwrite prior inputs")
    for relative in (*ROOTS, "tmp", ".local/state", ".config"):
        (home / relative).mkdir(parents=True, exist_ok=True, mode=0o700)
    if args.reuse_corpus:
        inventory = json.loads((root / "private-inventory.json").read_text())
        for item in inventory:
            path = home / item["relative"]
            if (path.is_symlink() or path.stat().st_size != item["bytes"]
                    or path.stat().st_mtime_ns != item["mtime_ns"] or sha(path) != item["sha256"]):
                raise RuntimeError("private_frozen_input_changed")
        live_hashes = {path.name: sha(path) for path in (root / "live-db").glob("*.sqlite3")}
    elif args.frozen_input_root:
        inventory = matrix.clone_frozen_input(args.frozen_input_root, home, sha)
        live_hashes = {}
        if args.today_sentinel:
            inventory.append(matrix.add_today_sentinel(home, sha))
    else:
        live_hashes = backup_databases(args.corpus_home / ".agentdeck", root / "live-db") if args.corpus_home else {}
        inventory = capture(args.corpus_home, home) if args.corpus_home else []
    if not inventory:
        parser.error("a nonempty authorized corpus is required")
    put(root / "private-inventory.json", inventory)
    material = "\n".join(sorted(f'{item["bytes"]}:{item["sha256"]}' for item in inventory))
    report = {"kind": "scan-performance-e1-v1", "input": {"files": len(inventory), "bytes": sum(item["bytes"] for item in inventory),
              "corpus_sha256": hashlib.sha256(material.encode()).hexdigest(), "live_db_backup_sha256": live_hashes,
              "capture_consistency": "two independent SQLite backups followed by per-file bounded JSONL prefixes; not cross-DB/filesystem atomic"},
              "samples": [], "limitations": ["AppKit bitmap+numeric OCR endpoint, not compositor/photon timing",
                  "No authentication files or model requests; isolated default quota has no live credentials",
                   "Small pair counts are pilots, not >=100-condition acceptance", "No page-cache purge", "No E1 PASS inferred from capture or build"]}
    report["click_precondition"] = "verified-active-App" if args.activate_before_click else "original-inactive-App-posted-events"
    report["click_dispatch"] = args.click_dispatch
    report["matrix_probe"] = args.matrix
    report["measurement"] = args.measurement
    if args.frozen_input_root:
        report["input"]["kind"] = "verified_frozen_clone_plus_controlled_today" if args.today_sentinel else "verified_frozen_clone"
        report["input"]["capture_consistency"] = "verified earlier per-file frozen JSONL copied with original mtime; databases generated privately,not new live backups"
        if args.today_sentinel:
            report["input"]["controlled_today_source"] = {key: inventory[-1][key] for key in ("bytes", "sha256", "mtime_ns")}
    if args.input_provenance:
        matrix.apply_input_provenance(json.loads(args.input_provenance.read_text()), report["input"], inventory)
    report["selected_arms"] = args.arms
    if len(args.arms) == 1:
        report["limitations"].append("Single-arm scoped continuation; not a standalone paired A/B result. Reuse a prior arm only with matching input, artifacts, controls, environment and exact starting databases.")
    report["require_first_click"] = not args.allow_ready_click_control
    if args.click_dispatch == "window":
        report["click_precondition"] = "verified-active-App" if args.activate_before_click else "no-App-activation-precondition"
        report["limitations"].append("NSWindow.sendEvent bypasses the App event queue but keeps its status button target/action; not proof of physical input routing")
    elif args.click_dispatch == "global":
        report["click_precondition"] = "verified-active-App" if args.activate_before_click else "visible-isolated-status-item; existing-event-posting-permission; no-App-activation-precondition"
        report["limitations"].append("CGEvent input goes through WindowServer at the private App's visible status item; permission is checked without requesting or changing it; synthetic input, not physical mouse timing")
    elif args.click_dispatch == "manual":
        report["click_precondition"] = "operator-physical-click; no event injection or pre-activation"
        report["manual_wait_seconds"] = args.manual_wait_seconds
        report["limitations"].append("Manual click latency starts at native button action entry, not physical mouse-down or compositor timing; operator waits are separate. Launch-to-frame includes operator wait and cannot pass a launch budget. Data already ready at first click is explicitly recorded.")
    report["limitations"].append("First-click timing is emitted only for the original routed click without a ready-click retry; process-entry timing remains separate")
    if args.activate_before_click:
        report["limitations"].append("Activation precedes injected click; not evidence that the inactive user's first physical click routes")
    put(args.output, report)
    if args.capture_only:
        print(json.dumps(report["input"]))
        return
    spec = importlib.util.spec_from_file_location("e0", REPO / "scripts/scan-performance-baseline.py")
    baseline = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(baseline)
    baseline.CLICK_STIMULUS = click_stimulus(baseline.CLICK_STIMULUS, args.activate_before_click, args.click_dispatch)
    batch = root / ("run-" + uuid.uuid4().hex)
    batch.mkdir(mode=0o700)
    apps = {}
    if args.reuse_apps_from_report:
        reference = json.loads(args.reuse_apps_from_report.read_text())
        if reference["input"]["corpus_sha256"] != report["input"]["corpus_sha256"]:
            raise RuntimeError("private_app_reuse_corpus_changed")
        matrix.apply_input_provenance(reference, report["input"], inventory)
        apps = reusable_apps(root, reference, args.warm_samples, args.activate_before_click,
                              args.click_dispatch, not args.allow_ready_click_control, args.apps_dir, args.manual_wait_seconds, args.matrix)
        report["identity"] = reference["identity"]
        report["reused_app_report_sha256"] = sha(args.reuse_apps_from_report)
    for label, candidate in (() if apps else (("A", False), ("B", True))):
        repository = batch / ("repo-" + label)
        export_sources(repository, candidate)
        input_files = [path for directory in ("internal", "cmd", "apps/macos", "scripts")
                       for path in (repository / directory).rglob("*")
                       if path.is_file() and path.suffix in (".go", ".swift", ".py", ".sh", ".xcstrings", ".plist")]
        input_files.extend(repository / relative for relative in ("go.mod", "go.sum", "vendor/modules.txt", "Makefile", "apps/macos/Package.swift", "apps/macos/AgentDeck.xcodeproj/project.pbxproj"))
        source_identity = {str(path.relative_to(repository)): sha(path) for path in sorted(set(input_files))}
        patch_hash = patch_probe(repository, args.warm_samples, args.activate_before_click, args.click_dispatch,
                                  not args.allow_ready_click_control, args.manual_wait_seconds)
        matrix_identity = matrix.patch_matrix_probe(repository, sha) if args.matrix else {}
        patch_hash = matrix_identity.get("probe_sha256", patch_hash)
        run(["bash", "scripts/build-macos-app.sh"], repository, batch / ("build-" + label + ".log"))
        app = repository / "apps/macos/build/DerivedData/Build/Products/Debug/AgentDeck.app"
        unique_bundle(app, uuid.uuid4().hex)
        apps[label] = app
        report.setdefault("identity", {})[label] = {"probe_sha256": patch_hash, "app_sha256": sha(app / "Contents/MacOS/AgentDeck"),
                                                    "helper_sha256": sha(app / "Contents/Helpers/agentdeck"),
                                                    "framework_sha256": sha(app / "Contents/Frameworks/AgentDeckShared.framework/Versions/A/AgentDeckShared"),
                                                    "widget_sha256": sha(app / "Contents/PlugIns/AgentDeckWidget.appex/Contents/MacOS/AgentDeckWidget"),
                                                     "source_files_sha256": source_identity}
        report["identity"][label].update(matrix_identity)
    report["environment"] = {"macos": platform.mac_ver()[0], "architecture": platform.machine(),
                             "timezone": "America/New_York", "locale": "en", "otel": "off",
                               "harness_sha256": harness_hash,
                               "matrix_module_sha256": sha(Path(matrix.__file__)),
                             "head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=REPO, text=True).strip(),
                              "toolchain": {name: subprocess.check_output(command, text=True, stderr=subprocess.DEVNULL).strip()
                                            for name, command in {"go": ["go", "version"], "xcode": ["xcodebuild", "-version"], "swift": ["swift", "--version"]}.items()}}
    if args.prepare_apps_only:
        report["preparation_only"] = True
        put(args.output, report)
        print(json.dumps({"prepared_arms": sorted(apps), "samples": 0, "click_dispatch": args.click_dispatch}))
        return
    put(args.output, report)
    if args.reuse_seed_from_report:
        prior = json.loads(args.reuse_seed_from_report.read_text())
        seed = reusable_seed(root, prior, report)
        report["reused_seed_report_sha256"] = sha(args.reuse_seed_from_report)
    else:
        helper = apps["A"] / "Contents/Helpers/agentdeck"
        if args.imported_seed:
            matrix.validate_imported_seed(root, args.imported_seed)
            report["preparation_seed_database_sha256"] = {path.name: sha(path) for path in args.imported_seed.iterdir()}
            baseline.restore_state(home, args.imported_seed)
        elif (root / "seed-imported").exists():
            baseline.restore_state(home, root / "seed-imported")
        baseline.cli(helper, home, ["scan"])
        while (home / ".agentdeck/scan.lock").exists():
            time.sleep(0.1)
        seed = batch / "seed-imported"
        backup_databases(home / ".agentdeck", seed)
    if args.matrix:
        import types
        operations = types.SimpleNamespace(sha=sha, put=put, backup_databases=backup_databases,
                                           stop_private_app=stop_private_app, launch_manual=launch_manual,
                                           __file__=__file__)
        matrix.run_matrix(args, baseline, operations, apps, home, seed, report)
        return
    # A/B source paths remain fixed. Only this harness's stopped fixture state
    # is restored; neither the live state nor an accepted worker is reset.
    for pair in range(args.pairs):
        for label in (("A", "B") if pair % 2 == 0 else ("B", "A")):
            if label not in args.arms:
                continue
            baseline.restore_state(home, seed)
            starting_state = {path.name: sha(path) for path in (home / ".agentdeck").glob("*.sqlite3")}
            result = (launch_manual(baseline, apps[label], home, label, args.warm_samples, args.manual_wait_seconds)
                      if args.click_dispatch == "manual" else baseline.launch(apps[label], home))
            if result.get("failure"):
                result["private_app_cleanup_verified"] = stop_private_app(apps[label])
            report["samples"].append({"pair": pair, "arm": label, "condition": "committed-today-no-cache", "starting_database_sha256": starting_state, "native": result})
            if sha(Path(__file__)) != harness_hash:
                report["failure"] = "harness_source_changed_during_observation"
            put(args.output, report)
            if result.get("failure") or report.get("failure"):
                print(json.dumps({"arm": label, "failure": result.get("failure") or report.get("failure")}))
                raise SystemExit(1)
    print(json.dumps({"samples": len(report["samples"]), "failures": sum(bool(item["native"].get("failure")) for item in report["samples"])}))


if __name__ == "__main__":
    main()
