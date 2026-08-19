from __future__ import annotations

import importlib.util
import io
import json
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).with_name("beads-consistency.py")
SPEC = importlib.util.spec_from_file_location("beads_consistency", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class BeadsConsistencyHookTest(unittest.TestCase):
    def test_remaining_timeout_is_bounded_by_command_and_hook_budgets(self) -> None:
        with mock.patch.object(MODULE.time, "monotonic", return_value=100.0):
            self.assertEqual(MODULE.remaining_timeout(120.0), 8.0)
            self.assertEqual(MODULE.remaining_timeout(103.5), 3.5)
            self.assertIsNone(MODULE.remaining_timeout(100.0))

    def test_exhausted_budget_skips_every_subprocess(self) -> None:
        with (
            mock.patch.object(MODULE.time, "monotonic", return_value=101.0),
            mock.patch.object(MODULE.subprocess, "run") as run,
        ):
            self.assertIsNone(MODULE.repo_root(100.0))
            self.assertIsNone(MODULE.bd_json(["list"], 100.0))
            self.assertEqual(MODULE.changed_paths(Path("/repo"), 100.0), [])
            run.assert_not_called()

    def test_dirty_tree_skips_irrelevant_awaiting_commit_query(self) -> None:
        root = Path("/repo")
        with (
            mock.patch.object(
                MODULE,
                "changed_paths",
                return_value=["docs/topics/example/requirements.md"],
            ),
            mock.patch.object(MODULE, "bd_json", return_value=[]) as bd_json,
        ):
            self.assertEqual(MODULE.findings(root, 123.0), [])

        bd_json.assert_called_once_with(["list", "--status", "open"], 123.0)

    def test_stop_report_blocks_so_the_model_sees_it(self) -> None:
        output = MODULE.report_output(["example mismatch"])

        # `systemMessage` alone reaches the user, not the model. That is how the
        # check went unheard for six review rounds, so the model-visible field
        # is the assertion that matters here.
        self.assertEqual(output["decision"], "block")
        self.assertIn("example mismatch", output["reason"])
        self.assertIn("Beads owns dispatch only", output["reason"])
        self.assertNotIn("hookSpecificOutput", output)

    def test_report_does_not_block_twice_in_one_turn(self) -> None:
        output = MODULE.report_output(["example mismatch"], stop_hook_active=True)

        # The turn is already continuing because of this hook; blocking again
        # would loop with no way out.
        self.assertEqual(set(output), {"systemMessage"})
        self.assertNotIn("decision", output)

    def test_codex_stop_blocker_uses_the_stderr_transport(self) -> None:
        output = MODULE.report_output(["example mismatch"])
        stderr = io.StringIO()
        stdout = io.StringIO()

        with (
            mock.patch.object(MODULE.sys, "stderr", stderr),
            mock.patch.object(MODULE.sys, "stdout", stdout),
        ):
            code = MODULE.emit_output(output, "Stop", "codex")

        # Codex does not parse blocker JSON on Stop; sending it JSON is the same
        # as saying nothing.
        self.assertEqual(code, 2)
        self.assertIn("example mismatch", stderr.getvalue())
        self.assertEqual(stdout.getvalue(), "")

    def test_claude_stop_blocker_stays_on_the_json_transport(self) -> None:
        output = MODULE.report_output(["example mismatch"])
        stderr = io.StringIO()
        stdout = io.StringIO()

        with (
            mock.patch.object(MODULE.sys, "stderr", stderr),
            mock.patch.object(MODULE.sys, "stdout", stdout),
        ):
            code = MODULE.emit_output(output, "Stop", "claude")

        self.assertEqual(code, 0)
        self.assertEqual(stderr.getvalue(), "")
        self.assertEqual(json.loads(stdout.getvalue())["decision"], "block")

    def test_released_turn_never_uses_the_blocking_transport(self) -> None:
        output = MODULE.report_output(["example mismatch"], stop_hook_active=True)
        stderr = io.StringIO()
        stdout = io.StringIO()

        with (
            mock.patch.object(MODULE.sys, "stderr", stderr),
            mock.patch.object(MODULE.sys, "stdout", stdout),
        ):
            code = MODULE.emit_output(output, "Stop", "codex")

        self.assertEqual(code, 0)
        self.assertEqual(stderr.getvalue(), "")
        self.assertEqual(set(json.loads(stdout.getvalue())), {"systemMessage"})

if __name__ == "__main__":
    unittest.main()
