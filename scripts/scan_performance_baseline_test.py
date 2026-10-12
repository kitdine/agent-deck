"""Isolated fixture behavior and bounded native-verifier source contracts."""
import ast
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("baseline", ROOT / "scripts/scan-performance-baseline.py")
baseline = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(baseline)
SWIFT = ROOT / "apps/macos/AgentDeckApp/MenuBarItemController.swift"


def sample_reset_code():
    """Execute the real per-sample reset, not a copy of its intended order."""
    tree = ast.parse(Path(baseline.__file__).read_text())
    for node in ast.walk(tree):
        if isinstance(node, ast.For) and ast.unparse(node.target) == "condition":
            start = next(i for i, item in enumerate(node.body) if ast.unparse(item) == "home = frozen")
            end = next(i for i in range(start + 1, len(node.body)) if isinstance(node.body[i], ast.If))
            return compile(ast.Module(body=node.body[start:end], type_ignores=[]), "sample-reset", "exec")
    raise AssertionError("sample reset missing")


class BaselineIsolationTests(unittest.TestCase):
    def fixture(self, root):
        home = Path(root) / "home"
        today = home / ".codex/sessions/baseline-today.jsonl"
        today.parent.mkdir(parents=True)
        today.write_bytes(b"previous delta")
        state = home / ".agentdeck"
        state.mkdir()
        lock = state / "scan.lock"
        lock.write_bytes(b"accepted worker")
        database = state / "fixture.sqlite3"
        database.write_bytes(b"previous database")
        group = home / "app-group"
        group.mkdir()
        (group / "snapshot").write_bytes(b"previous display")
        return home, today, lock, database, group

    def reset(self, home, today):
        exec(sample_reset_code(), {**vars(baseline), "frozen": home, "today": today,
             "original_today": b"seed source", "today_mtime": 1_000_000_000,
             "seeds": {"empty": None}, "condition": "empty"})

    def test_timeout_handoff_waits_before_any_fixture_mutation(self):
        with tempfile.TemporaryDirectory() as root:
            home, today, lock, database, group = self.fixture(root)
            before = today.stat()
            observed = []

            def finish_worker(_):
                observed.append((today.read_bytes(), today.stat().st_mtime_ns,
                                 database.read_bytes(), (group / "snapshot").read_bytes()))
                lock.unlink()

            with patch.object(baseline.time, "sleep", side_effect=finish_worker):
                self.reset(home, today)
            self.assertEqual(observed, [(b"previous delta", before.st_mtime_ns,
                                       b"previous database", b"previous display")])
            self.assertEqual(today.read_bytes(), b"seed source")
            self.assertEqual(today.stat().st_mtime_ns, 1_000_000_000)
            self.assertFalse(database.exists())
            self.assertFalse(group.exists())

    def test_failed_barrier_leaves_source_database_and_display_unchanged(self):
        with tempfile.TemporaryDirectory() as root:
            home, today, lock, database, group = self.fixture(root)
            before = today.stat()
            with patch.object(baseline.time, "monotonic", side_effect=[0, 301]):
                with self.assertRaisesRegex(RuntimeError, "accepted_worker_not_idle"):
                    self.reset(home, today)
            self.assertTrue(lock.exists())
            self.assertEqual(today.read_bytes(), b"previous delta")
            self.assertEqual(today.stat().st_mtime_ns, before.st_mtime_ns)
            self.assertEqual(database.read_bytes(), b"previous database")
            self.assertEqual((group / "snapshot").read_bytes(), b"previous display")


class NativeBaselineContractTests(unittest.TestCase):
    def test_update_captures_both_endpoints_before_validation(self):
        source = SWIFT.read_text()
        update = source.split("let update = Task", 1)[1].split("} catch let error", 1)[0]
        capture = source.split("private func baselineFrame(", 1)[1].split("private func", 1)[0]
        self.assertNotIn("VNImageRequestHandler", capture)
        self.assertNotIn("baselineFrameIsValid", capture)
        self.assertLess(update.index("let old = try baselineFrame()"), update.index("await update.value"))
        self.assertLess(update.index("await update.value"), update.index("let new = try baselineFrame()"))
        self.assertLess(update.index("let new = try baselineFrame()"), update.index("baselineFrameIsValid"))
        self.assertIn("elapsedNanoseconds: new.end - start", update)
        self.assertIn("guard oldValid, newValid, latestDataChanged", update)
        self.assertIn("VNImageRequestHandler", source.split("private func baselineFrameIsValid", 1)[1])

    def test_report_stimulus_matches_native_event_path(self):
        source = SWIFT.read_text()
        self.assertIn('static let baselineClickStimulus = "' + baseline.CLICK_STIMULUS + '"', source)
        click = source.split("private func baselineClick(", 1)[1].split("private func", 1)[0]
        self.assertIn("[NSEvent.EventType.leftMouseDown, .leftMouseUp]", click)
        self.assertIn("NSApp.postEvent(event, atStart: false)", click)
        self.assertNotIn("performClick", click)
        self.assertIn("clickStimulus: Self.baselineClickStimulus", source)

    def test_launch_rejects_mismatched_stimulus(self):
        with tempfile.TemporaryDirectory() as root:
            home = Path(root)
            baseline.write_json(home / "baseline-result.json", {"clickStimulus": "wrong"})
            with patch.object(baseline, "file_hash", return_value="synthetic-binary"), \
                    patch.object(baseline.subprocess, "run") as run:
                run.return_value.returncode = 0
                result = baseline.launch(home / "App.app", home)
            self.assertEqual(result.get("failure"), "native_click_stimulus_mismatch")

    def test_launch_accepts_matching_stimulus_and_binds_binary_identity(self):
        with tempfile.TemporaryDirectory() as root:
            home = Path(root)
            baseline.write_json(home / "baseline-result.json", {"clickStimulus": baseline.CLICK_STIMULUS})
            with patch.object(baseline, "file_hash", return_value="synthetic-binary"), \
                    patch.object(baseline.subprocess, "run") as run:
                run.return_value.returncode = 0
                result = baseline.launch(home / "App.app", home)
            self.assertIsNone(result.get("failure"))
            self.assertEqual(result["executed_app_sha256"], "synthetic-binary")
            self.assertEqual(result["executed_helper_sha256"], "synthetic-binary")
            self.assertFalse(result["whole_process_is_launch_latency"])


if __name__ == "__main__":
    unittest.main()
