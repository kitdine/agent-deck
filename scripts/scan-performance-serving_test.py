#!/usr/bin/env python3
"""Private E1 instrumentation contracts; no App launches or real data access."""
import importlib.util
import contextlib
import copy
import io
import json
from pathlib import Path
import plistlib
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("serving_probe", ROOT / "scripts/scan-performance-serving.py")
PROBE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROBE)
CONTROLLER = "apps/macos/AgentDeckApp/MenuBarItemController.swift"
STIMULUS = "App-local posted leftMouseDown/leftMouseUp at status button; activation after initial routing; not global physical input"


class MatrixReuseTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="agentdeck-macos-xctest.serving.reuse-test.", dir="/private/tmp")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.reference = {"kind": "scan-performance-e1-v1", "click_dispatch": "manual",
                          "require_first_click": True, "manual_wait_seconds": 600, "matrix_probe": True,
                          "preparation_only": True, "identity": {}, "environment": {
                              "head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()}}
        self.apps = {}
        sources = (CONTROLLER, "apps/macos/AgentDeckApp/AgentDeckApp.swift",
                   "apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift", "cmd/agentdeck/main.go")
        artifacts = {"app_sha256": "Contents/MacOS/AgentDeck", "helper_sha256": "Contents/Helpers/agentdeck",
                     "framework_sha256": "Contents/Frameworks/AgentDeckShared.framework/Versions/A/AgentDeckShared",
                     "widget_sha256": "Contents/PlugIns/AgentDeckWidget.appex/Contents/MacOS/AgentDeckWidget"}
        for arm in ("A", "B"):
            repository = self.root / "run-fixture" / ("repo-" + arm)
            source_hashes = {}
            for relative in sources:
                text = (ROOT / relative).read_text() if arm == "B" else subprocess.check_output(
                    ["git", "show", self.reference["environment"]["head"] + ":" + relative], cwd=ROOT, text=True)
                path = repository / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(text)
                source_hashes[relative] = PROBE.sha(path)
            PROBE.patch_probe(repository, 0, click_dispatch="manual", manual_wait_seconds=600)
            identity = PROBE.matrix.patch_matrix_probe(repository, PROBE.sha)
            identity["source_files_sha256"] = source_hashes
            app = repository / "apps/macos/build/DerivedData/Build/Products/Debug/AgentDeck.app"
            # Tiny artifact fixtures exercise selection/hashing, not a build or App launch.
            for key, relative in artifacts.items():
                path = app / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes((arm + key).encode())
                identity[key] = PROBE.sha(path)
            (app / "Contents/Info.plist").write_bytes(plistlib.dumps({"CFBundleIdentifier": "test.serving." + arm}))
            self.reference["identity"][arm] = identity
            self.apps[arm] = app

    def reuse(self):
        return PROBE.reusable_apps(self.root, self.reference, 0, False, "manual", True,
                                   manual_wait_seconds=600, matrix_probe=True)

    def test_matching_preparation_reuses_both_arms_and_cleans_reconstruction(self):
        self.assertEqual(self.reuse(), self.apps)
        self.assertEqual(list(self.root.glob("probe-reuse.*")), [])

    def test_candidate_source_drift_is_rejected(self):
        self.reference["identity"]["B"]["source_files_sha256"]["cmd/agentdeck/main.go"] = "prior-source"
        with self.assertRaisesRegex(RuntimeError, "private_app_reuse_production_source_changed"):
            self.reuse()

    def test_changed_pressure_instrumentation_is_rejected(self):
        with patch.object(PROBE.matrix, "REBUILD_PRESSURE_GO", PROBE.matrix.REBUILD_PRESSURE_GO + "\n// changed probe\n"):
            with self.assertRaisesRegex(RuntimeError, "private_app_reuse_matrix_probe_changed"):
                self.reuse()

    def test_every_matrix_identity_hash_is_checked_for_each_arm(self):
        for arm in ("A", "B"):
            for key in ("probe_sha256", "delegate_probe_sha256", "shared_probe_sha256",
                        "pressure_main_sha256", "pressure_go_sha256"):
                with self.subTest(arm=arm, key=key):
                    with patch.dict(self.reference["identity"][arm], {key: "prior-probe"}):
                        with self.assertRaisesRegex(RuntimeError, "private_app_reuse_matrix_probe_changed"):
                            self.reuse()


class ProbeTests(unittest.TestCase):
    def sources(self):
        current = (ROOT / CONTROLLER).read_text()
        baseline = subprocess.check_output(["git", "show", "HEAD:" + CONTROLLER], cwd=ROOT, text=True)
        return (baseline, current)

    def patch(self, source, **options):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / CONTROLLER
            path.parent.mkdir(parents=True)
            path.write_text(source)
            PROBE.patch_probe(root, 2, **options)
            return path.read_text()

    def test_default_preserves_input_and_separates_first_click_from_retry(self):
        for source in self.sources():
            with self.subTest(source=source[:40]):
                text = self.patch(source)
                self.assertIn(STIMULUS, text)
                self.assertIn("NSApp.postEvent(event, atStart: false)", text)
                self.assertIn("var initialClickRouted = false", text)
                self.assertIn("baselineClick(button); initialClickRouted = true", text)
                self.assertIn("if initialClickRouted, !readyClickRequired, let firstClickUptime", text)
                self.assertIn("firstActionReached = baselineActionCount > 0", text)
                self.assertIn('"firstAppActive": firstAppActive', text)
                self.assertIn('"firstButtonVisible": firstButtonVisible', text)

    def test_window_control_retains_target_action_and_labels_distinct_input(self):
        for source in self.sources():
            text = self.patch(source, click_dispatch="window")
            self.assertIn("window.sendEvent(events[0])", text)
            self.assertLess(text.index("NSApp.postEvent(events[1], atStart: true)"), text.index("window.sendEvent(events[0])"))
            self.assertNotIn("NSApp.postEvent(event, atStart: false)", text)
            self.assertIn(PROBE.click_stimulus(STIMULUS, click_dispatch="window"), text)
            self.assertIn("button.action = #selector(handleClick(_:))", text)
            self.assertIn("togglePopover(sender)", text)
            self.assertNotIn("button.performClick", text)

    def test_activation_control_remains_explicit_and_checks_actual_state(self):
        for dispatch in ("posted", "window", "global"):
            for source in self.sources():
                text = self.patch(source, activate_before_click=True, click_dispatch=dispatch)
                self.assertIn("while !NSApp.isActive", text)
                self.assertIn("throw BaselineFailure.activationUnavailable", text)
                self.assertIn(PROBE.click_stimulus(STIMULUS, True, dispatch), text)

    def test_global_control_fails_closed_without_permission_or_visible_placement(self):
        for source in self.sources():
            text = self.patch(source, click_dispatch="global")
            self.assertIn("CGPreflightPostEventAccess(), window.isVisible, button.isEnabled", text)
            self.assertIn("screen.frame.contains(screenPoint)", text)
            self.assertIn("window.convertPoint(toScreen: point)", text)
            self.assertIn("event.post(tap: .cghidEventTap)", text)
            self.assertNotIn("CGRequestPostEventAccess", text)
            self.assertNotIn("NSApp.postEvent(event, atStart: false)", text)
            self.assertIn(PROBE.click_stimulus(STIMULUS, click_dispatch="global"), text)
            self.assertIn("button.action = #selector(handleClick(_:))", text)

    def test_probe_fails_closed_on_source_drift_or_unknown_dispatch(self):
        source = self.sources()[1]
        with self.assertRaisesRegex(RuntimeError, "anchor_mismatch"):
            self.patch(source.replace("var initialClickRouted = true", "var initialClickRouted = false"))
        with self.assertRaisesRegex(ValueError, "unsupported_click_dispatch"):
            self.patch(source, click_dispatch="physical")

    def test_lost_first_click_is_fail_fast_unless_diagnostic_retry_is_explicit(self):
        source = self.sources()[1]
        text = self.patch(source)
        self.assertIn("catch BaselineFailure.clickNotRouted { firstActionReached = baselineActionCount > 0; throw BaselineFailure.clickNotRouted }", text)
        control = self.patch(source, require_first_click=False)
        self.assertIn("catch BaselineFailure.clickNotRouted { initialClickRouted = false }", control)
        self.assertIn("if initialClickRouted, !readyClickRequired, let firstClickUptime", control)

    def test_cleanup_rejects_non_private_identity_and_requires_verified_stop(self):
        with tempfile.TemporaryDirectory() as directory:
            app = Path(directory) / "Fixture.app"
            info = app / "Contents/Info.plist"
            info.parent.mkdir(parents=True)
            info.write_bytes(plistlib.dumps({"CFBundleIdentifier": "test.regular.app"}))
            with patch.object(PROBE.subprocess, "run") as run:
                with self.assertRaisesRegex(RuntimeError, "private_bundle_identity_required"):
                    PROBE.stop_private_app(app)
                run.assert_not_called()
            identity = "test.regular.app.serving.fixture"
            info.write_bytes(plistlib.dumps({"CFBundleIdentifier": identity}))
            for status, output, expected in ((0, "private_app_stopped\n", True),
                                              (1, "private_app_cleanup_unverified\n", False),
                                              (0, "unexpected", False)):
                with self.subTest(status=status, output=output):
                    result = subprocess.CompletedProcess([], status, stdout=output)
                    with patch.object(PROBE.subprocess, "run", return_value=result) as run:
                        self.assertEqual(PROBE.stop_private_app(app), expected)
                        arguments = run.call_args.args[0]
                        self.assertEqual(arguments[-2:], [identity, str(app / "Contents/MacOS/AgentDeck")])
                        self.assertIn("resolvingSymlinksInPath().path == expected", arguments[2])
                        self.assertEqual(run.call_args.kwargs["timeout"], 30)

    def test_reuse_rejects_changed_input_control_before_looking_for_apps(self):
        valid = {"kind": "scan-performance-e1-v1", "click_dispatch": "global", "require_first_click": True}
        for key, wrong in (("kind", "other"), ("click_dispatch", "posted"), ("require_first_click", False)):
            with self.subTest(key=key):
                reference = {**valid, key: wrong}
                with self.assertRaisesRegex(RuntimeError, "private_app_reuse_control_mismatch"):
                    PROBE.reusable_apps(Path("unused"), reference, 2, False, "global", True)

    def test_reuse_rejects_arbitrary_output_locations(self):
        reference = {"kind": "scan-performance-e1-v1", "click_dispatch": "global", "require_first_click": True}
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(RuntimeError, "private_app_reuse_output_directory_required"):
                PROBE.reusable_apps(Path("unused"), reference, 2, False, "global", True, Path(directory))

    def test_manual_probe_uses_physical_action_and_excludes_operator_wait(self):
        for source in self.sources():
            text = self.patch(source, click_dispatch="manual", manual_wait_seconds=30)
            self.assertIn(PROBE.click_stimulus(STIMULUS, click_dispatch="manual"), text)
            self.assertIn('button.title = "TEST "', text)
            self.assertIn('"status": baselineManualRequests == 1 ? "awaiting_visibility" : "waiting_for_click"', text)
            self.assertIn('signal["instance"] as? String == instance', text)
            self.assertIn("let visibilityDeadline = waitStarted + 600_000_000_000", text)
            self.assertIn("(operatorDeadline ?? visibilityDeadline)", text)
            self.assertIn("let deadline = waitStarted + 30000000000", text)
            self.assertIn("baselineManualClickUptime = DispatchTime.now().uptimeNanoseconds", text)
            self.assertIn("firstClickUptime = baselineManualClickUptime", text)
            self.assertIn("baselineManualWaitNanoseconds += clicked - waitStarted", text)
            self.assertIn("firstClickDataReady: baselineManualDataReadyAtFirstClick", text)
            self.assertIn('clickTimingOrigin: "native_left_mouse_button_action_entry"', text)
            self.assertLess(text.index("try await baselineClick(button)\n\t\t\t\tguard let start"),
                            text.index("elapsedNanoseconds: frame.end - start"))
            self.assertNotIn("NSApp.postEvent", text)
            self.assertNotIn("event.post(tap:", text)
            self.assertNotIn("CGRequestPostEventAccess", text)
            self.assertIn("togglePopover(sender)", text)

    def test_manual_probe_rejects_activation_retry_and_unbounded_wait(self):
        source = self.sources()[1]
        for options in ({"activate_before_click": True}, {"require_first_click": False}):
            with self.subTest(options=options), self.assertRaisesRegex(ValueError, "unmodified_first_click"):
                self.patch(source, click_dispatch="manual", **options)
        for seconds in (0, 4, 601):
            with self.subTest(seconds=seconds), self.assertRaisesRegex(ValueError, "bounded_manual_wait"):
                self.patch(source, click_dispatch="manual", manual_wait_seconds=seconds)

    def test_first_manual_request_rejects_earlier_unmeasured_input(self):
        for source in self.sources():
            text = self.patch(source, click_dispatch="manual")
            self.assertIn("guard baselineManualRequests != 0 || baselineActionCount == 0", text)
            self.assertIn('manualInputBeforeReady = "manual_input_before_first_request"', text)
            self.assertLess(text.index("throw BaselineFailure.manualInputBeforeReady"), text.index("baselineManualRequests += 1"))

    def test_visibility_confirmation_is_bound_to_the_live_request(self):
        with tempfile.TemporaryDirectory() as directory:
            home = Path(directory)
            progress = home / "manual-click-progress.json"
            progress.write_text(json.dumps({"request": 1, "instance": "live-instance", "status": "awaiting_visibility"}))
            result = PROBE.confirm_manual_visibility(home)
            self.assertEqual(result["status"], "visibility_confirmed_no_restart")
            self.assertEqual(json.loads((home / "manual-visible.json").read_text()), {"instance": "live-instance"})
            self.assertEqual((home / "manual-visible.json").stat().st_mode & 0o777, 0o600)
            for invalid in ({"status": "waiting_for_click", "instance": "later"}, {"status": "awaiting_visibility"}):
                progress.write_text(json.dumps(invalid))
                with self.assertRaisesRegex(RuntimeError, "not_pending"):
                    PROBE.confirm_manual_visibility(home)

    def test_manual_reuse_rejects_changed_wait_before_looking_for_apps(self):
        reference = {"kind": "scan-performance-e1-v1", "click_dispatch": "manual", "require_first_click": True,
                     "manual_wait_seconds": 30}
        with self.assertRaisesRegex(RuntimeError, "manual_wait_changed"):
            PROBE.reusable_apps(Path("unused"), reference, 2, False, "manual", True, manual_wait_seconds=120)

    def test_manual_launch_marks_human_dependent_launch_time_non_evaluable(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            app, home = root / "Fixture.app", root / "home"
            home.mkdir()
            for relative in ("Contents/MacOS/AgentDeck", "Contents/Helpers/agentdeck"):
                path = app / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b"private-test-artifact")
            stimulus = PROBE.click_stimulus(STIMULUS, click_dispatch="manual")
            (home / "baseline-result.json").write_text(json.dumps({"clickStimulus": stimulus,
                "firstClickToDataNanoseconds": 123, "operatorWaitNanoseconds": 456}))
            baseline = type("Baseline", (), {"CLICK_STIMULUS": stimulus, "environment": staticmethod(lambda h: {"HOME": str(h)})})
            with patch.object(PROBE.subprocess, "Popen") as launch:
                launch.return_value.poll.return_value = 0
                launch.return_value.returncode = 0
                result = PROBE.launch_manual(baseline, app, home, "B", 1, 30)
                self.assertEqual(launch.call_args.kwargs["env"]["AGENTDECK_MANUAL_ARM"], "B")
                self.assertEqual(result["firstClickToDataNanoseconds"], 123)
                self.assertEqual(result["operatorWaitNanoseconds"], 456)
                self.assertTrue(result["launch_includes_operator_wait"])
                self.assertFalse(result["launch_latency_budget_evaluable"])
                self.assertFalse(result["whole_process_is_launch_latency"])

    def test_invalid_manual_cli_controls_fail_before_any_capture(self):
        for control in ("--activate-before-click", "--allow-ready-click-control"):
            argv = ["serving", "--click-dispatch", "manual", control, "--output", "unused.json"]
            with self.subTest(control=control), patch.object(sys, "argv", argv), \
                    contextlib.redirect_stderr(io.StringIO()), patch.object(PROBE, "capture") as capture:
                with self.assertRaises(SystemExit) as error:
                    PROBE.main()
                self.assertEqual(error.exception.code, 2)
                capture.assert_not_called()

    def seed_records(self, root):
        seed = root / "run-fixture/seed-imported"
        seed.mkdir(parents=True)
        for name in ("agentdeck.sqlite3", "sessions.sqlite3"):
            (seed / name).write_bytes(("isolated-" + name).encode())
        hashes = {path.name: PROBE.sha(path) for path in seed.glob("*.sqlite3")}
        report = {"kind": "scan-performance-e1-v1", "input": {"corpus_sha256": "fixture"},
                  "identity": {"A": {"app_sha256": "baseline"}, "B": {"app_sha256": "candidate"}},
                  "click_dispatch": "manual", "require_first_click": True, "manual_wait_seconds": 120,
                  "environment": {"head": "fixture", "macos": "fixture", "architecture": "fixture",
                                  "timezone": "UTC", "locale": "en", "otel": "off", "toolchain": {"swift": "fixture"}}}
        prior = {**copy.deepcopy(report), "samples": [{"arm": "A", "starting_database_sha256": hashes,
                 "native": {"frames": [{"valid": True}]}}, {"arm": "B", "starting_database_sha256": hashes,
                 "native": {"failure": "operator_click_timeout", "frames": []}}]}
        return seed, prior, report

    def test_seed_reuse_keeps_completed_a_and_exact_starting_databases(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            seed, prior, report = self.seed_records(root)
            self.assertEqual(PROBE.reusable_seed(root, prior, report), seed)
            (seed / "agentdeck.sqlite3").write_bytes(b"changed")
            with self.assertRaisesRegex(RuntimeError, "exact_starting_state_missing"):
                PROBE.reusable_seed(root, prior, report)

    def test_seed_reuse_rejects_artifact_environment_and_mixed_state_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            _, prior, report = self.seed_records(root)
            altered = copy.deepcopy(report)
            altered["identity"]["B"]["app_sha256"] = "new"
            with self.assertRaisesRegex(RuntimeError, "identity_changed"):
                PROBE.reusable_seed(root, prior, altered)
            altered = copy.deepcopy(report)
            altered["environment"]["timezone"] = "other"
            with self.assertRaisesRegex(RuntimeError, "environment_changed"):
                PROBE.reusable_seed(root, prior, altered)
            prior["samples"][1]["starting_database_sha256"] = {}
            with self.assertRaisesRegex(RuntimeError, "mixed_starting_states"):
                PROBE.reusable_seed(root, prior, report)

    def test_seed_reuse_rejects_missing_success_and_symlinked_database(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            seed, prior, report = self.seed_records(root)
            prior["samples"][0]["native"] = {"failure": "operator_click_timeout", "frames": []}
            with self.assertRaisesRegex(RuntimeError, "needs_completed_arm"):
                PROBE.reusable_seed(root, prior, report)
            prior["samples"][0]["native"] = {"frames": [{"valid": True}]}
            original = seed / "agentdeck.sqlite3"
            saved = root / "saved.sqlite3"
            original.rename(saved)
            original.symlink_to(saved)
            with self.assertRaisesRegex(RuntimeError, "exact_starting_state_missing"):
                PROBE.reusable_seed(root, prior, report)


if __name__ == "__main__":
    unittest.main()
