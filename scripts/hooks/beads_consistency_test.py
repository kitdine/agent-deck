from __future__ import annotations

import importlib.util
import io
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).with_name("beads-consistency.py")
SPEC = importlib.util.spec_from_file_location("beads_consistency", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class BeadsConsistencyHookTest(unittest.TestCase):
    def test_authorization_wait_skips_all_heavy_consistency_work(self) -> None:
        event = {
            "hook_event_name": "Stop",
            "last_assistant_message": (
                "Waiting for one exact approval.\n"
                "WORKFLOW_AUTHORIZATION_WAIT: ce-write phase-token"
            ),
        }
        with (
            mock.patch.object(sys, "argv", [str(SCRIPT), "--runtime", "codex"]),
            mock.patch.object(sys, "stdin", io.StringIO(json.dumps(event))),
            mock.patch.object(MODULE, "repo_root") as repo_root,
            mock.patch.object(MODULE, "findings") as findings,
        ):
            self.assertEqual(MODULE.main(), 0)

        repo_root.assert_not_called()
        findings.assert_not_called()

    def test_project_contract_grants_stage_internal_state_transitions(self) -> None:
        agents = SCRIPT.parents[2] / "AGENTS.md"
        normalized = " ".join(agents.read_text(encoding="utf-8").split())

        for contract in (
            "Stage Command Authority",
            "review and status artifacts",
            "completion-evidence",
            "Beads",
            "without additional user authorization",
            "commit, push, release, or deploy",
        ):
            self.assertIn(contract, normalized)

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

        # Assert the intent — the awaiting_commit query is the one a dirty tree
        # makes meaningless — rather than the total call count, which any
        # unrelated check that also queries Beads would break.
        queried = [call.args[0] for call in bd_json.call_args_list]
        self.assertIn(["list", "--status", "open"], queried)
        self.assertNotIn(["list", "--status", "awaiting_commit"], queried)

    def test_live_task_restating_a_retired_status_name_is_reported(self) -> None:
        root = Path("/repo")
        beads = [
            {
                "id": "ad-x-doc-req-design",
                "description": (
                    "Author requirements.md. Lifecycle: open -> drafting -> "
                    "in_review -> repairing -> in_review -> closed."
                ),
            }
        ]

        with (
            mock.patch.object(MODULE, "changed_paths", return_value=[]),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", return_value=beads),
        ):
            notes = MODULE.findings(root, 123.0)

        stale = [n for n in notes if "retired status name" in n]
        self.assertEqual(len(stale), 1)
        self.assertIn("ad-x-doc-req-design", stale[0])
        self.assertIn("`drafting`", stale[0])
        self.assertIn("`repairing`", stale[0])
        self.assertIn(".agent-instructions/beads.md", stale[0])

    def test_retired_status_scan_excludes_closed_tasks(self) -> None:
        root = Path("/repo")
        seen: list[list[str]] = []

        def bd_json(args: list[str], _deadline: float) -> list[dict[str, str]]:
            seen.append(args)
            return []

        with (
            mock.patch.object(MODULE, "changed_paths", return_value=[]),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=bd_json),
        ):
            MODULE.findings(root, 123.0)

        # A closed task recorded what happened under the contract in force then;
        # rewriting it would falsify history, so it must never be scanned.
        scans = [a for a in seen if a[:2] == ["list", "--status"] and "in_review" in a[2]]
        self.assertTrue(scans)
        for args in scans:
            self.assertNotIn("closed", args[2].split(","))

    def test_current_lifecycle_vocabulary_is_not_flagged(self) -> None:
        root = Path("/repo")
        beads = [
            {
                "id": "ad-x-doc-req-design",
                "description": (
                    "Author requirements.md. The status lifecycle and its "
                    "transitions are defined in .agent-instructions/beads.md."
                ),
            }
        ]

        with (
            mock.patch.object(MODULE, "changed_paths", return_value=[]),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", return_value=beads),
        ):
            notes = MODULE.findings(root, 123.0)

        self.assertEqual([n for n in notes if "retired status name" in n], [])

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

    def test_latest_review_state_reads_the_latest_round_gate(self) -> None:
        review = mock.MagicMock()
        review.read_text.return_value = "\n".join(
            (
                "## Round 1",
                "- Completion gate: VERIFIED",
                "- Verdict: PASS",
                "## Round 2",
                "- Completion gate: BLOCKED",
                "- Verdict: PASS",
            )
        )

        self.assertEqual(MODULE.latest_review_state(review), ("PASS", "BLOCKED"))

    def test_pass_with_blocked_gate_may_remain_in_review(self) -> None:
        root = Path("/repo")

        def beads(args: list[str], _deadline: float) -> list[dict[str, str]]:
            if args == ["list", "--status", "in_review"]:
                return [{"id": "task-1", "title": "任务：anchor"}]
            return []

        with (
            mock.patch.object(
                MODULE,
                "changed_paths",
                return_value=["docs/topics/example/reviews/anchor.md"],
            ),
            mock.patch.object(
                MODULE.Path,
                "read_text",
                return_value="- Completion gate: BLOCKED\n- Verdict: PASS\n",
            ),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=beads),
        ):
            self.assertEqual(MODULE.findings(root, 123.0), [])

    def test_pass_requires_awaiting_commit_only_after_verified_gate(self) -> None:
        root = Path("/repo")

        def beads(args: list[str], _deadline: float) -> list[dict[str, str]]:
            if args == ["list", "--status", "in_review"]:
                return [{"id": "task-1", "title": "任务：anchor"}]
            return []

        with (
            mock.patch.object(
                MODULE,
                "changed_paths",
                return_value=["docs/topics/example/reviews/anchor.md"],
            ),
            mock.patch.object(
                MODULE.Path,
                "read_text",
                return_value="- Completion gate: VERIFIED\n- Verdict: PASS\n",
            ),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=beads),
        ):
            notes = MODULE.findings(root, 123.0)

        self.assertEqual(len(notes), 1)
        self.assertIn("completion gate is `VERIFIED`", notes[0])

    def test_pass_with_blocked_gate_rejects_awaiting_commit(self) -> None:
        root = Path("/repo")

        def beads(args: list[str], _deadline: float) -> list[dict[str, str]]:
            if args == ["list", "--status", "awaiting_commit"]:
                return [{"id": "task-1", "title": "任务：anchor"}]
            return []

        with (
            mock.patch.object(
                MODULE,
                "changed_paths",
                return_value=["docs/topics/example/reviews/anchor.md"],
            ),
            mock.patch.object(
                MODULE.Path,
                "read_text",
                return_value="- Completion gate: BLOCKED\n- Verdict: PASS\n",
            ),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=beads),
        ):
            notes = MODULE.findings(root, 123.0)

        self.assertEqual(len(notes), 1)
        self.assertIn("completion gate is `BLOCKED`", notes[0])
        self.assertIn("must remain `in_review`", notes[0])

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


class OwnerlessFindingsTest(unittest.TestCase):
    """`.agent-instructions/review-records.md` — findings must reach a carrier.

    The rule is structural: a review record retires with its topic, so every
    finding must be closed or carried before PASS. `A6-F1` is the regression
    for an existing SUPERSEDED disposition, not the reason the rule exists.
    """

    def record(self, body: str) -> Path:
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        path = Path(directory.name) / "r.md"
        path.write_text(body, encoding="utf-8")
        return path

    def test_bare_open_finding_is_ownerless(self):
        path = self.record(
            "## Round 1\n- Findings:\n"
            "  - [P1] **X1-F1** a defect\n    spanning two lines -> open\n"
            "- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), ["X1-F1"])

    def test_beads_carrier_on_the_bullet_accounts_for_it(self):
        path = self.record(
            "## Round 1\n- Findings:\n"
            "  - [P1] **X1-F1** a defect -> `ad-bug-something`\n"
            "- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), [])

    def test_backlog_carrier_accounts_for_it(self):
        path = self.record(
            "## Round 1\n- Findings:\n"
            "  - [P1] **X1-F1** a defect -> roadmap.md Backlog: pricing tiers\n"
            "- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), [])

    def test_later_round_naming_the_id_closes_it(self):
        path = self.record(
            "## Round 1\n- Findings:\n  - [P1] **Y1-F1** a defect -> open\n"
            "- Verdict: REOPEN\n"
            "## Round 2\n- Y1-F1 closed: repaired in candidate.\n"
            "- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), [])

    def test_closure_word_applies_only_to_the_id_it_follows(self):
        path = self.record(
            "## Round 1\n- Findings:\n"
            "  - [P1] **X1-F1** first defect -> open\n"
            "  - [P1] **X1-F2** second defect -> open\n"
            "- Verdict: REOPEN\n"
            "## Repair — Round 1\n"
            "- Verdict: REOPEN — X1-F1 closed; X1-F2 moved to follow-up.\n"
            "## Round 2\n- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), ["X1-F2"])

    def test_archived_superseded_disposition_counts_as_closed(self):
        path = (
            SCRIPT.parents[2]
            / "docs/archive/topics/switch-effectiveness-boundary/reviews/architecture.md"
        )
        self.assertNotIn("A6-F1", MODULE.ownerless_findings(path))

    def test_prefix_group_closure_accounts_for_each_named_finding(self):
        path = self.record(
            "## Round 1\n- Findings:\n"
            "  - [P1] **X1-F1** first defect -> open\n"
            "  - [P1] **X1-F2** second defect -> open\n"
            "- Verdict: REOPEN\n"
            "## Round 2\n"
            "- Both findings are closed: X1-F1 changed one path, and X1-F2 changed another.\n"
            "- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), [])

    def test_suffix_group_closure_accounts_for_each_named_finding(self):
        path = self.record(
            "## Round 1\n- Findings:\n"
            "  - [P1] **X1-F1** first defect -> open\n"
            "  - [P1] **X1-F2** second defect -> open\n"
            "- Verdict: REOPEN\n"
            "## Round 2\n- X1-F1、X1-F2 均已关闭。\n- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), [])

    def test_real_prefix_group_record_introduces_no_ownerless_findings(self):
        path = (
            SCRIPT.parents[2]
            / "docs/archive/topics/desktop-app/reviews/desktop-app-contract.md"
        )
        self.assertNotIn("CD1-F1", MODULE.ownerless_findings(path))
        self.assertNotIn("CD1-F2", MODULE.ownerless_findings(path))
        self.assertNotIn("CD1-F3", MODULE.ownerless_findings(path))

    def test_cross_record_see_clause_does_not_introduce_a_finding(self):
        path = self.record(
            "## Round 1\n"
            "- [P1] R1-F1 local defect -> open. See A1-F1 for the other record.\n"
            "- Verdict: REOPEN\n"
            "## Round 2\n- R1-F1 closed: repaired locally.\n- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), [])

    def test_reference_ids_do_not_make_a_passing_record_ownerless(self):
        path = self.record(
            "# Fix record\n"
            "Example syntax: X1-F1 closed; X1-F2 moved to follow-up.\n"
            "## Review — Round 1\n"
            "- [P1] R1-F1 parser compatibility -> open\n"
            "- Evidence references CD1-F1, R9-F2, A1-F1, and H3-F1.\n"
            "- Verdict: REOPEN\n"
            "## Repair — Round 1\n- R1-F1 closed: clause parsing repaired.\n"
            "- Reference IDs X1-F2, CD1-F1, R9-F2, A1-F1, and H3-F1 are closed as examples.\n"
            "## Review — Round 2\n- Verdict: PASS\n"
        )
        self.assertEqual(MODULE.ownerless_findings(path), [])

    def test_missing_file_is_not_an_error(self):
        self.assertEqual(MODULE.ownerless_findings(Path("/nonexistent/x.md")), [])


if __name__ == "__main__":
    unittest.main()
