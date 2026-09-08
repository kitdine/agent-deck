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


class SessionScopeTest(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.enterContext(mock.patch.dict(MODULE.os.environ, {
            "AGENTDECK_BEADS_HOOK_STATE_DIR": str(self.root / "state")
        }))
        self.enterContext(mock.patch.object(MODULE, "repo_root", return_value=self.root))

    def event(self, kind: str, *, session: str = "a", runtime: str = "codex",
              turn: str = "1", prompt: str = "") -> tuple[int, str, str]:
        data = {"hook_event_name": kind, "session_id": session, "turn_id": turn, "prompt": prompt}
        stdout, stderr = io.StringIO(), io.StringIO()
        with (
            mock.patch.object(sys, "argv", [str(SCRIPT), "--runtime", runtime]),
            mock.patch.object(sys, "stdin", io.StringIO(json.dumps(data))),
            mock.patch.object(sys, "stdout", stdout),
            mock.patch.object(sys, "stderr", stderr),
        ):
            code = MODULE.main()
        return code, stdout.getvalue(), stderr.getvalue()

    def select(self, **kwargs: str) -> None:
        with mock.patch.object(MODULE, "selected_scope", return_value={"topic": "current", "subject": "tasks.md"}):
            self.event("UserPromptSubmit", prompt="评审：current / tasks.md", **kwargs)

    def test_current_scope_blocks_in_both_runtimes(self) -> None:
        for runtime in ("codex", "claude"):
            with self.subTest(runtime=runtime):
                self.select(runtime=runtime)
                with mock.patch.object(MODULE, "findings", return_value=["current mismatch"]) as scan:
                    code, out, err = self.event("Stop", runtime=runtime)
                scan.assert_called_once()
                self.assertEqual(scan.call_args.kwargs['scope'], {"topic": "current", "subject": "tasks.md"})
                if runtime == "codex":
                    self.assertEqual(code, 2)
                    self.assertIn("current mismatch", err)
                else:
                    self.assertEqual(json.loads(out)['decision'], "block")

    def test_other_session_runtime_and_old_turn_cannot_borrow_scope(self) -> None:
        self.select()
        with mock.patch.object(MODULE, "findings") as scan:
            self.event("Stop", session="b")
            self.event("Stop", runtime="claude")
            self.event("Stop", turn="old")
            scan.assert_not_called()

    def test_unmatched_new_task_clears_old_scope(self) -> None:
        self.select()
        with mock.patch.object(MODULE, "selected_scope", return_value=None):
            self.event("UserPromptSubmit", turn="2", prompt="Explain a different problem")
        with mock.patch.object(MODULE, "findings") as scan:
            self.event("Stop", turn="2")
            scan.assert_not_called()

    def test_continue_preserves_scope_with_new_turn(self) -> None:
        self.select()
        self.event("UserPromptSubmit", turn="2", prompt="继续")
        with mock.patch.object(MODULE, "findings", return_value=["still missing"]):
            self.assertEqual(self.event("Stop", turn="2")[0], 2)

    def test_identical_content_is_reported_once_but_changed_content_is_new(self) -> None:
        self.select()
        with mock.patch.object(MODULE, "findings", return_value=["missing dispatch"]):
            self.assertEqual(self.event("Stop")[0], 2)
            self.assertEqual(self.event("Stop"), (0, "", ""))
            path = self.root / "docs/topics/current/tasks.md"
            path.parent.mkdir(parents=True)
            path.write_text("changed content\n")
            self.assertEqual(self.event("Stop")[0], 2)

    def test_unrelated_dirty_file_does_not_change_fingerprint(self) -> None:
        scope = {"topic": "current", "subject": "tasks.md"}
        before = MODULE.report_fingerprint(self.root, scope, ["missing"])
        path = self.root / "docs/topics/other/tasks.md"
        path.parent.mkdir(parents=True)
        path.write_text("unrelated\n")
        self.assertEqual(before, MODULE.report_fingerprint(self.root, scope, ["missing"]))

    def test_unknown_router_is_nonblocking(self) -> None:
        with mock.patch.dict(MODULE.os.environ, {"AGENTDECK_WORKFLOW_HOOK": str(self.root / "missing.py")}):
            self.assertIsNone(MODULE.selected_scope("评审：current / tasks.md", "codex"))

    def test_invalid_persisted_scope_cannot_escape_repository(self) -> None:
        for scope in ({"topic": "../other", "subject": "tasks.md"},
                      {"topic": "current", "subject": "../../secret.md"},
                      {"topic": "fix", "subject": ""}, {"topic": "current"}):
            self.assertFalse(MODULE.valid_scope(scope))

    def test_session_identity_includes_repository(self) -> None:
        event = {"session_id": "same"}
        self.assertNotEqual(MODULE.session_state_path(self.root / "one", event, "codex"),
                            MODULE.session_state_path(self.root / "two", event, "codex"))

    def test_turn_identity_uses_runtime_field_precedence(self) -> None:
        event = {"turn_id": "turn", "prompt_id": "prompt"}
        self.assertEqual(MODULE.event_turn(event, "codex"), "turn")
        self.assertEqual(MODULE.event_turn(event, "claude"), "prompt")
        self.assertIsNone(MODULE.event_turn({"turn_id": 123}, "codex"))

    def test_explicit_audit_is_nonblocking_without_session_state(self) -> None:
        output = io.StringIO()
        with (
            mock.patch.object(sys, "argv", [str(SCRIPT), "--runtime", "codex", "--audit"]),
            mock.patch.object(sys, "stdout", output),
            mock.patch.object(MODULE, "findings", return_value=["outside current work"]),
        ):
            self.assertEqual(MODULE.main(), 0)
        self.assertEqual(json.loads(output.getvalue()), {"notes": ["outside current work"]})

    def test_fixed_then_reintroduced_mismatch_is_reported_again(self) -> None:
        self.select()
        with mock.patch.object(MODULE, "findings", side_effect=[["missing"], [], ["missing"]]):
            self.assertEqual(self.event("Stop")[0], 2)
            self.assertEqual(self.event("Stop")[0], 0)
            self.assertEqual(self.event("Stop")[0], 2)

    def test_subject_scope_excludes_other_tasks_in_same_topic(self) -> None:
        scope = {"topic": "current", "subject": "alpha"}
        self.assertTrue(MODULE.in_scope("docs/topics/current/reviews/alpha.md", scope))
        self.assertFalse(MODULE.in_scope("docs/topics/current/reviews/beta.md", scope))
        self.assertFalse(MODULE.in_scope("docs/topics/other/reviews/alpha.md", scope))


class ReviewTaskOwnershipTest(unittest.TestCase):
    def setUp(self) -> None:
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        for topic in ("alpha", "beta"):
            base = self.root / "docs/topics" / topic
            (base / "reviews").mkdir(parents=True)
            (base / "tasks.md").write_text("| 1 | `build` | [x] | [ ] |\n")
            (base / "reviews/build.md").write_text("Verdict: PASS\nCompletion gate: VERIFIED\n")

    def task(self, topic: str, *, structured: bool = True) -> dict:
        task = {"id": f"{topic}-build", "title": "任务：build",
                "description": f"Implement docs/topics/{topic}/tasks.md; dependency: docs/topics/alpha/architecture.md."}
        if structured:
            task['dependencies'] = [{"id": f"{topic}-decomposition", "title": f"文档：{topic} / tasks.md", "dependency_type": "blocks"}]
        return task

    def diagnose(self, tasks: list[dict], *, scope: bool = True) -> list[str]:
        def query(args, deadline):
            return tasks if args == ['list', '--status', 'in_review'] else []
        with (
            mock.patch.object(MODULE, 'changed_paths', return_value=['docs/topics/alpha/reviews/build.md']),
            mock.patch.object(MODULE, 'bd_json', side_effect=query),
        ):
            return MODULE.findings(self.root, 123.0, scope={'topic':'alpha', 'subject':'build'} if scope else None)

    def test_cross_topic_references_cannot_override_unique_structured_owner(self) -> None:
        alpha, beta = self.task('alpha'), self.task('beta')
        for tasks in ([alpha, beta], [beta, alpha]):
            for scope in (True, False):
                notes = self.diagnose(tasks, scope=scope)
                self.assertEqual(len(notes), 1)
                self.assertIn('alpha-build (alpha)', notes[0])
                self.assertNotIn('beta-build', notes[0])

    def test_description_only_reproducer_has_no_proven_owner(self) -> None:
        alpha, beta = self.task('alpha', structured=False), self.task('beta', structured=False)
        for tasks in ([alpha, beta], [beta, alpha]):
            self.assertEqual(self.diagnose(tasks), [])

    def test_multiple_decomposition_owners_and_duplicate_subjects_are_ambiguous(self) -> None:
        alpha = self.task('alpha')
        alpha['dependencies'] += self.task('beta')['dependencies']
        self.assertEqual(self.diagnose([alpha]), [])
        first, second = self.task('alpha'), self.task('alpha')
        second['id'] = 'another-alpha-build'
        for tasks in ([first, second], [second, first]):
            self.assertEqual(self.diagnose(tasks), [])

    def test_decomposition_must_actually_contain_the_task_anchor(self) -> None:
        (self.root/'docs/topics/alpha/tasks.md').write_text('| 1 | `different` | [ ] | [ ] |\n')
        self.assertEqual(self.diagnose([self.task('alpha')]), [])

    def test_list_candidate_loads_its_own_structured_detail(self) -> None:
        candidate = self.task('alpha', structured=False)
        with mock.patch.object(MODULE, 'bd_json', return_value=[self.task('alpha')]) as query:
            self.assertEqual(MODULE.implementation_subject(self.root, candidate, 123.0), ('alpha', 'build'))
        query.assert_called_once_with(['show', 'alpha-build'], 123.0)

    def test_wrong_detail_or_unavailable_provider_does_not_invent_ownership(self) -> None:
        for response in (None, [], [self.task('beta')], [self.task('alpha'), self.task('beta')]):
            with mock.patch.object(MODULE, 'bd_json', return_value=response):
                self.assertIsNone(MODULE.implementation_subject(self.root, self.task('alpha', structured=False), 123.0))


class BeadsConsistencyHookTest(unittest.TestCase):
    def test_stop_without_session_scope_does_not_scan_repository(self) -> None:
        event = {"hook_event_name": "Stop", "session_id": "unrelated-session"}
        with (
            tempfile.TemporaryDirectory() as directory,
            mock.patch.dict(MODULE.os.environ, {"AGENTDECK_BEADS_HOOK_STATE_DIR": directory}),
            mock.patch.object(sys, "argv", [str(SCRIPT), "--runtime", "codex"]),
            mock.patch.object(sys, "stdin", io.StringIO(json.dumps(event))),
            mock.patch.object(MODULE, "repo_root", return_value=Path(directory)),
            mock.patch.object(MODULE, "findings", return_value=[]) as scan,
        ):
            self.assertEqual(MODULE.main(), 0)
            scan.assert_not_called()

    def test_missing_decomposition_tasks_are_scoped_to_selected_topic(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for topic in ("current", "other"):
                plan = root / "docs" / "topics" / topic / "tasks.md"
                plan.parent.mkdir(parents=True)
                plan.write_text("| tasks.md | [x] | [x] |\n| 1 | `build` | [ ] | [ ] |\n")
            with (
                mock.patch.object(MODULE, "changed_paths", return_value=[]),
                mock.patch.object(MODULE, "bd_json", return_value=[]),
            ):
                notes = MODULE.findings(root, 123.0, scope={"topic": "current", "subject": "tasks.md"})
            self.assertEqual(len(notes), 1)
            self.assertIn("current passed", notes[0])
            self.assertNotIn("other", notes[0])

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

    def test_head_text_returns_committed_content_and_empty_for_an_unrecorded_path(
        self,
    ) -> None:
        # The guard for `tasks.md` rests on this contract in both directions: an
        # unrecorded path must read as empty, or a topic created at stage 1 would
        # be reported, and a recorded one must read back its content, or a real
        # stage-8 draft would compare against nothing and always look changed. The
        # regressions below mock `head_text`, so without this test the mock's
        # assumed contract is bound to nothing.
        root = SCRIPT.parents[2]
        deadline = MODULE.time.monotonic() + MODULE.HOOK_BUDGET

        self.assertNotEqual(
            MODULE.head_text(root, ".agent-instructions/beads.md", deadline), ""
        )
        self.assertEqual(
            MODULE.head_text(root, "docs/topics/zzz-not-a-topic/tasks.md", deadline), ""
        )

        # An exhausted budget yields the same empty reading without spending a
        # subprocess, so a slow git can never be the reason a session cannot stop.
        with (
            mock.patch.object(MODULE.time, "monotonic", return_value=101.0),
            mock.patch.object(MODULE.subprocess, "run") as run,
        ):
            self.assertEqual(MODULE.head_text(root, "AGENTS.md", 100.0), "")
            run.assert_not_called()

    def test_documents_matrix_sync_does_not_claim_the_decomposition_task(self) -> None:
        # `tasks.md` is both a deliverable and the topic's status authority. Every
        # stage ticks the Documents matrix row it just drafted; only stage 8 writes
        # the Tasks matrix. Reporting the first asks an agent to claim
        # decomposition that has not started.
        root = Path("/repo")
        current = (
            "| Document | Draft | Review |\n"
            "| requirements.md | [x] | [x] |\n"
            "| ux/menubar.md | [x] | [ ] |\n"
            "| tasks.md | [ ] | [ ] |\n"
            "\n## Task breakdown\n\nNot yet decomposed.\n"
        )
        head = current.replace(
            "| ux/menubar.md | [x] | [ ] |", "| ux/menubar.md | [ ] | [ ] |"
        )

        def beads(args: list[str], _deadline: float) -> list[dict[str, str]]:
            if args == ["list", "--status", "open"]:
                return [{"id": "doc-tasks", "title": "文档：example / tasks.md"}]
            return []

        with (
            mock.patch.object(
                MODULE, "changed_paths", return_value=["docs/topics/example/tasks.md"]
            ),
            mock.patch.object(MODULE.Path, "read_text", return_value=current),
            mock.patch.object(MODULE, "head_text", return_value=head),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=beads),
        ):
            self.assertEqual(MODULE.findings(root, 123.0), [])

    def test_tasks_matrix_drafting_still_claims_the_decomposition_task(self) -> None:
        root = Path("/repo")
        head = "| Document | Draft | Review |\n| tasks.md | [ ] | [ ] |\n"
        current = head + "| 1. `store-boundaries` | [ ] | [ ] |\n"

        def beads(args: list[str], _deadline: float) -> list[dict[str, str]]:
            if args == ["list", "--status", "open"]:
                return [{"id": "doc-tasks", "title": "文档：example / tasks.md"}]
            return []

        with (
            mock.patch.object(
                MODULE, "changed_paths", return_value=["docs/topics/example/tasks.md"]
            ),
            mock.patch.object(MODULE.Path, "read_text", return_value=current),
            mock.patch.object(MODULE, "head_text", return_value=head),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=beads),
        ):
            notes = MODULE.findings(root, 123.0)

        self.assertEqual(len(notes), 1)
        self.assertIn("doc-tasks", notes[0])
        self.assertIn("is still `open`", notes[0])

    def test_an_untracked_tasks_file_holding_no_task_rows_is_not_reported(self) -> None:
        # A topic created at stage 1 has a `tasks.md` with a Documents matrix and
        # nothing else, and no committed version to compare against.
        root = Path("/repo")
        current = "| Document | Draft | Review |\n| requirements.md | [x] | [ ] |\n"

        def beads(args: list[str], _deadline: float) -> list[dict[str, str]]:
            if args == ["list", "--status", "open"]:
                return [{"id": "doc-tasks", "title": "文档：example / tasks.md"}]
            return []

        with (
            mock.patch.object(
                MODULE, "changed_paths", return_value=["docs/topics/example/tasks.md"]
            ),
            mock.patch.object(MODULE.Path, "read_text", return_value=current),
            mock.patch.object(MODULE, "head_text", return_value=""),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=beads),
        ):
            self.assertEqual(MODULE.findings(root, 123.0), [])

    def test_every_other_document_still_reports_on_a_path_match(self) -> None:
        # The exception is `tasks.md` alone; drafting any other document is still
        # the `in_progress` state and a path match is still the whole trigger.
        root = Path("/repo")

        def beads(args: list[str], _deadline: float) -> list[dict[str, str]]:
            if args == ["list", "--status", "open"]:
                return [{"id": "doc-req", "title": "文档：example / requirements.md"}]
            return []

        with (
            mock.patch.object(
                MODULE,
                "changed_paths",
                return_value=["docs/topics/example/requirements.md"],
            ),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=beads),
        ):
            notes = MODULE.findings(root, 123.0)

        self.assertEqual(len(notes), 1)
        self.assertIn("doc-req", notes[0])

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
                return [{"id": "task-1", "title": "任务：anchor", "dependencies": [{"title": "文档：example / tasks.md", "dependency_type": "blocks"}]}]
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
                return_value="- Completion gate: BLOCKED\n- Verdict: PASS\n| 1 | `anchor` | [x] | [ ] |\n",
            ),
            mock.patch.object(MODULE.Path, "glob", return_value=[]),
            mock.patch.object(MODULE, "bd_json", side_effect=beads),
        ):
            self.assertEqual(MODULE.findings(root, 123.0), [])

    def test_pass_requires_awaiting_commit_only_after_verified_gate(self) -> None:
        root = Path("/repo")

        def beads(args: list[str], _deadline: float) -> list[dict[str, str]]:
            if args == ["list", "--status", "in_review"]:
                return [{"id": "task-1", "title": "任务：anchor", "dependencies": [{"title": "文档：example / tasks.md", "dependency_type": "blocks"}]}]
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
                return_value="- Completion gate: VERIFIED\n- Verdict: PASS\n| 1 | `anchor` | [x] | [ ] |\n",
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
                return [{"id": "task-1", "title": "任务：anchor", "dependencies": [{"title": "文档：example / tasks.md", "dependency_type": "blocks"}]}]
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
                return_value="- Completion gate: BLOCKED\n- Verdict: PASS\n| 1 | `anchor` | [x] | [ ] |\n",
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


class SkillReportInteropTest(unittest.TestCase):
    def state(self, body):
        return MODULE.latest_review_state(mock.Mock(read_text=mock.Mock(return_value=body)))

    def test_skill_reports_read_gate_after_verdict_and_across_nested_heading(self):
        for verdict, gate in (("✅ Verdict: PASS", "Completion gate: VERIFIED"),
                              ("✅ **结论**：**PASS**", "完成门禁：VERIFIED")):
            with self.subTest(verdict=verdict):
                self.assertEqual(self.state(
                    "## Round 1\n## 📋 Review\n📊 Score: 9/10\n" + verdict +
                    "\n### 📝 Summary\n" + gate + "\n"), ("PASS", "VERIFIED"))

    def test_latest_localized_or_incomplete_round_never_reuses_old_pass(self):
        previous = "## Round 1\nVerdict: PASS\nCompletion gate: VERIFIED\n"
        self.assertEqual(self.state(previous +
            "## Round 2\n## 📋 复评\n✅ 结论: FAIL\n完成门禁: BLOCKED\n"),
            ("FAIL", "BLOCKED"))
        self.assertEqual(self.state(previous + "## Round 2\nWork pending.\n"), (None, None))

    def test_metadata_before_report_title_is_in_the_same_round(self):
        self.assertEqual(self.state("## Review — Round 2\nCompletion gate: VERIFIED\n"
                                    "## 📋 Review\n✅ Verdict: PASS\n"), ("PASS", "VERIFIED"))

    def test_quoted_examples_do_not_supply_verdict_or_round(self):
        self.assertEqual(self.state("## Round 1\nVerdict: FAIL\nCompletion gate: BLOCKED\n"
            "```markdown\n## Round 99\nVerdict: PASS\nCompletion gate: VERIFIED\n```\n"),
            ("FAIL", "BLOCKED"))

    def test_conflicting_fields_fail_closed(self):
        self.assertEqual(self.state("## Round 1\nVerdict: PASS\n结论: FAIL\n"
                                    "Completion gate: VERIFIED\n"), (None, None))

    def test_finding_prefixes_cannot_cancel_each_other(self):
        body = "DW-R11-F1 CLOSED: repaired.\nXY-R11-F1 open: still broken.\n"
        self.assertEqual(MODULE.ownerless_findings(
            mock.Mock(read_text=mock.Mock(return_value=body))), ["XY-R11-F1"])


if __name__ == "__main__":
    unittest.main()
