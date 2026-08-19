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

    def test_matrix_rows_read_both_task_table_shapes(self) -> None:
        text = "\n".join(
            (
                "| Document | Draft | Review |",
                "| tasks.md | [x] | [x] |",
                "| `ux/` | n/a | n/a |",
                "| 1. `numbered-with-dot` | [x] | [ ] |",
                "| 2 | `numbered-own-cell` | [ ] | [ ] |",
            )
        )

        # The Documents matrix must never be read as a task row; its subject cell
        # carries no backticks, which is what separates the two tables.
        self.assertEqual(
            MODULE.matrix_rows(text),
            [("numbered-with-dot", "[x]", "[ ]"), ("numbered-own-cell", "[ ]", "[ ]")],
        )
        self.assertTrue(MODULE.decomposition_passed(text))

    def test_draft_decomposition_creates_no_dispatch_expectation(self) -> None:
        text = "| tasks.md | [x] | [ ] |\n| 1. `anchor` | [ ] | [ ] |"

        # Before PASS the matrix is a draft: anchors still get renamed, merged or
        # dropped, so demanding tasks for them would assert work that may not exist.
        self.assertFalse(MODULE.decomposition_passed(text))

    def test_missing_development_task_is_reported_after_pass(self) -> None:
        root = Path("/repo")
        plan = mock.MagicMock()
        plan.read_text.return_value = (
            "| tasks.md | [x] | [x] |\n"
            "| 1. `delivered` | [x] | [x] |\n"
            "| 2. `pending` | [ ] | [ ] |"
        )
        plan.parent.name = "example"

        with (
            mock.patch.object(MODULE, "changed_paths", return_value=[]),
            mock.patch.object(MODULE.Path, "glob", return_value=[plan]),
            mock.patch.object(MODULE, "bd_json", return_value=[]),
        ):
            notes = MODULE.findings(root, 123.0)

        # `delivered` is already reviewed, so it needs no dispatch object; only the
        # anchor that still has work left does.
        joined = "\n".join(notes)
        self.assertIn("`pending` has no Beads task", joined)
        self.assertNotIn("`delivered`", joined)

    def test_existing_development_task_is_not_reported(self) -> None:
        root = Path("/repo")
        plan = mock.MagicMock()
        plan.read_text.return_value = "| tasks.md | [x] | [x] |\n| 1. `pending` | [ ] | [ ] |"
        plan.parent.name = "example"

        with (
            mock.patch.object(MODULE, "changed_paths", return_value=[]),
            mock.patch.object(MODULE.Path, "glob", return_value=[plan]),
            mock.patch.object(
                MODULE, "bd_json", return_value=[{"id": "x", "title": "任务：pending"}]
            ),
        ):
            notes = MODULE.findings(root, 123.0)

        self.assertEqual([n for n in notes if "has no Beads task" in n], [])

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
