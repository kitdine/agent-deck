#!/usr/bin/env python3
"""Report Beads coordination state that the repository has already moved past.

The development-workflow Skill deliberately does not know Beads exists — its
own boundaries say `AGENTS.md` is not a runtime prerequisite — so running a
phase never moves a Beads task by itself. The only thing that does is an agent
following AGENTS.md's routed workflow rules, and a forgotten
transition is invisible: dispatch keeps asserting the previous state. That is
how a task once sat `in_progress` for a day while nothing was being implemented.

This hook closes the loop from the project side rather than by binding the Skill
to an implementation. It compares two facts that are both cheap and unambiguous:
what the working tree shows was just done, and what Beads currently claims. It
only ever reports; it never writes to Beads and never blocks a stop.

Runs on Stop for both Codex and Claude Code. Silent unless something disagrees.
It never writes to Beads. A disagreement holds the turn open in both runtimes so
the agent reconciles it before finishing — Claude Code through the blocker JSON,
Codex through the stderr exit-code-2 transport it accepts. Either way the report
must reach the actor that can act on it, which a user-facing message alone does
not.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

# This hook encodes one repository's task-title grammar and Beads deployment, so
# it must stay inert anywhere else. Identify that repository by a file it owns,
# never by its path: a checkout lives wherever its owner cloned it, and a path
# match would silently disable the hook for every clone but one.
REPO_MARKER = Path(".agent-instructions/beads.md")
BEADS_ROOT = Path.home() / ".local/state/agentdeck-beads"
# NOT the agent-facing way to call Beads. This hook is a non-interactive reader
# that supplies its own -C and needs no audit identity, so it invokes the raw
# binary. An agent MUST instead use the wrapper, which requires an actor and
# sets BEADS_DIR itself:
#     env BEADS_ACTOR=claude-code BEADS_ROOT/bin/agentdeck-bd <command>
# Calling this path directly leaves BEADS_ACTOR unset, and bd then falls back to
# git user.name — which records the human operator as the author of an agent's
# comments and status transitions. See .agent-instructions/beads.md.
BD_BIN = "/usr/local/bin/bd"

# Status names `.agent-instructions/beads.md` retired in commit b3ca412, when the
# lifecycle became open -> in_progress -> in_review -> awaiting_commit -> closed.
# They are no longer valid `bd` statuses, so their only remaining home is prose
# that was copied before the change.
RETIRED_STATUS_NAMES = frozenset({"drafting", "repairing"})
# Every status a task can hold while it is still live work. `closed` is excluded
# deliberately: a closed task records what happened under the contract in force
# at the time, and rewriting it would falsify history rather than fix anything.
LIVE_STATUSES = frozenset(
    {"open", "in_progress", "in_review", "awaiting_commit", "blocked", "deferred"}
)

# "文档：<topic> / <document>" and "任务：<task-anchor>" — the two title shapes
# .agent-instructions/beads.md defines.
DOC_TITLE = re.compile(r"^文档：\s*([^/\s]+)\s*/\s*(.+)$")
TASK_TITLE = re.compile(r"^任务：\s*(.+)$")

# A review record's verdict line, e.g. "- Verdict: PASS".
VERDICT = re.compile(r"Verdict:\s*(PASS|FAIL|REOPEN)", re.IGNORECASE)
COMPLETION_GATE = re.compile(
    r"Completion gate:\s*`?(VERIFIED|NOT_VERIFIED|FAILED|BLOCKED|NOT_REQUIRED)`?",
    re.IGNORECASE,
)
# A finding ID as `.agent-instructions/review-records.md` defines it: A6-F1,
# DW-R11-F2, D1-F1. The audit that produced that rule found 103 of them across
# every review record.
FINDING_ID = re.compile(r"\b([A-Z]+[0-9]+-F[0-9]+)\b")
# Words a later round actually uses to close one, gathered from the records
# themselves rather than invented: `A1-F1 closed:`, `-> repaired in candidate.`
FINDING_CLOSED = re.compile(
    r"repaired|closed|resolved|addressed|fixed|已修复|已关闭|已解决|已处理", re.I
)
# A carrier is a Beads issue or a named Backlog item. Nothing else counts,
# because nothing else is read again after the record is archived.
FINDING_CARRIER = re.compile(r"\bad-[a-z0-9][a-z0-9-]*\b|roadmap\.md Backlog:")

AUTHORIZATION_WAIT = re.compile(
    r"(?m)^WORKFLOW_AUTHORIZATION_WAIT:\s*"
    r"[A-Za-z0-9][A-Za-z0-9._:/-]*(?:\s+[A-Za-z0-9_-]+)?[ \t]*$"
)

TIMEOUT = 8
HOOK_BUDGET = 10.0


def remaining_timeout(deadline: float) -> float | None:
    """Return one subprocess timeout inside the shared Stop-hook budget."""
    remaining = deadline - time.monotonic()
    return min(float(TIMEOUT), remaining) if remaining > 0 else None


def authorization_wait(event: dict[str, Any]) -> bool:
    message = event.get("last_assistant_message")
    return isinstance(message, str) and bool(AUTHORIZATION_WAIT.search(message))


def repo_root(deadline: float) -> Path | None:
    timeout = remaining_timeout(deadline)
    if timeout is None:
        return None
    try:
        out = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if out.returncode != 0:
        return None
    root = Path(out.stdout.strip())
    return root if (root / REPO_MARKER).is_file() else None


def bd_json(args: list[str], deadline: float) -> Any:
    """Run bd read-only. Any failure yields None — a hook must never be the
    reason a session cannot stop."""
    env = {
        **os.environ,
        "BEADS_DIR": str(BEADS_ROOT / ".beads"),
        "GIT_CONFIG_GLOBAL": str(BEADS_ROOT / "beads.gitconfig"),
        # Reads are audited too, and the wrapper rejects an omitted actor.
        "BEADS_ACTOR": os.environ.get("BEADS_ACTOR") or "consistency-hook",
    }
    timeout = remaining_timeout(deadline)
    if timeout is None:
        return None
    try:
        out = subprocess.run(
            [BD_BIN, "-C", str(BEADS_ROOT), *args, "--json"],
            capture_output=True,
            text=True,
            timeout=timeout,
            env=env,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if out.returncode != 0:
        return None
    try:
        parsed = json.loads(out.stdout)
    except json.JSONDecodeError:
        return None
    # bd wraps some payloads in a JSON envelope.
    if isinstance(parsed, dict) and isinstance(parsed.get("data"), list):
        return parsed["data"]
    return parsed


def changed_paths(root: Path, deadline: float) -> list[str]:
    timeout = remaining_timeout(deadline)
    if timeout is None:
        return []
    try:
        out = subprocess.run(
            ["git", "-C", str(root), "status", "--porcelain"],
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except (OSError, subprocess.SubprocessError):
        return []
    if out.returncode != 0:
        return []
    paths: list[str] = []
    for line in out.stdout.splitlines():
        if len(line) > 3:
            # Rename entries are "old -> new"; the destination is what changed.
            paths.append(line[3:].split(" -> ")[-1].strip())
    return paths


def latest_review_state(path: Path) -> tuple[str | None, str | None]:
    """Return the latest round's verdict and completion gate.

    Records append rounds. Restrict the gate lookup to the section containing
    the final verdict so an older VERIFIED gate cannot leak across a later
    REOPEN or BLOCKED round.
    """
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return None, None
    verdicts = list(VERDICT.finditer(text))
    if not verdicts:
        return None, None
    latest = verdicts[-1]
    round_start = text.rfind("\n## ", 0, latest.start())
    section = text[round_start if round_start >= 0 else 0 : latest.end()]
    gates = COMPLETION_GATE.findall(section)
    gate = gates[-1].upper() if gates else None
    return latest.group(1).upper(), gate


def latest_verdict(path: Path) -> str | None:
    return latest_review_state(path)[0]


def ownerless_findings(path: Path) -> list[str]:
    """Finding IDs in `path` that are neither closed nor carried.

    A review record retires with its topic. Once it is under `docs/archive/`,
    nobody opens it looking for outstanding work, so a finding left with a bare
    `-> open` stops existing the moment the topic is archived. `A6-F1` did
    exactly that: raised as a blocking P1 in Round 6 and never named again
    through the Round 20 that passed.

    A finding is accounted for when a later line names its ID alongside a
    closure word, or when its own bullet names a carrier. The bullet is read to
    its end rather than one line, because these findings run several lines and
    the carrier is usually on the last of them.
    """
    try:
        lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    except OSError:
        return []
    first_seen: dict[str, int] = {}
    closed: set[str] = set()
    for number, line in enumerate(lines):
        for match in FINDING_ID.finditer(line):
            identifier = match.group(1)
            first_seen.setdefault(identifier, number)
            if FINDING_CLOSED.search(line):
                closed.add(identifier)
    ownerless: list[str] = []
    for identifier, start in sorted(first_seen.items(), key=lambda item: item[1]):
        if identifier in closed:
            continue
        # The bullet the finding was raised in: up to the next bullet at the
        # same or shallower indent, or a blank line followed by a new block.
        bullet = [lines[start]]
        for line in lines[start + 1 :]:
            if re.match(r"^\s*-\s", line) or line.startswith("#") or not line.strip():
                break
            bullet.append(line)
        if not FINDING_CARRIER.search("\n".join(bullet)):
            ownerless.append(identifier)
    return ownerless


def doc_subject_of(bead: dict[str, Any]) -> tuple[str, str] | None:
    """`(topic, document)` for a `文档：<topic> / <document>` title.

    Both halves matter. A topic owns several document tasks, and each one is
    reviewed on its own record with its own verdict, so keying by topic alone
    makes one record's verdict speak for every sibling document.
    """
    doc = DOC_TITLE.match(str(bead.get("title") or ""))
    if not doc:
        return None
    return doc.group(1), doc.group(2).strip()


def anchor_of(bead: dict[str, Any]) -> str | None:
    task = TASK_TITLE.match(str(bead.get("title") or ""))
    return task.group(1).strip() if task else None


def review_subject(rel: str) -> tuple[str, str] | None:
    """`docs/topics/<topic>/reviews/<subject>.md` -> `(topic, subject)`."""
    parts = Path(rel).parts
    if (
        len(parts) != 5
        or parts[0] != "docs"
        or parts[1] != "topics"
        or parts[3] != "reviews"
    ):
        return None
    return parts[2], Path(parts[4]).stem


def record_stem(document: str) -> str:
    """The record name a document is reviewed under.

    `.agent-instructions/review-records.md` names a document's record after the
    document itself, flattening `ux/<surface>.md` to `ux-<surface>.md`. A record
    whose stem matches no document task is a task-anchor record, and it says
    nothing about any document's status.
    """
    return document[:-3].replace("/", "-") if document.endswith(".md") else document


# A task-matrix row, in either shape a topic uses: `| 1. `anchor` | [x] | [ ] |`
# and `| 1 | `anchor` | [x] | [ ] |`. The Documents matrix never matches, because
# its subject cell carries no backticks.
CHECKBOX = ("[ ]", "[x]")
ANCHOR_CELL = re.compile(r"`([a-z0-9][a-z0-9-]*)`")


def matrix_rows(text: str) -> list[tuple[str, str, str]]:
    """Return (anchor, dev, review) for every task row in a topic's tasks.md."""
    rows: list[tuple[str, str, str]] = []
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("|") or not line.endswith("|"):
            continue
        cells = [c.strip() for c in line.strip("|").split("|")]
        if len(cells) < 3 or cells[-1] not in CHECKBOX or cells[-2] not in CHECKBOX:
            continue
        found = ANCHOR_CELL.search(" ".join(cells[:-2]))
        if found:
            rows.append((found.group(1), cells[-2], cells[-1]))
    return rows


def decomposition_passed(text: str) -> bool:
    """True when the topic's own `tasks.md` row shows a ticked Review cell."""
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("| tasks.md |"):
            continue
        cells = [c.strip() for c in line.strip("|").split("|")]
        return len(cells) >= 3 and cells[-1] == "[x]"
    return False


def findings(root: Path, deadline: float) -> list[str]:
    changed = changed_paths(root, deadline)
    notes: list[str] = []

    # 1. A review record was written or updated, but dispatch does not reflect
    #    both its verdict and completion gate. Review PASS alone is insufficient:
    #    a required non-VERIFIED gate intentionally keeps the task in_review.
    touched_reviews = [p for p in changed if "/reviews/" in p and p.endswith(".md")]
    if touched_reviews:
        by_status = {
            status: bd_json(["list", "--status", status], deadline) or []
            for status in ("in_review", "awaiting_commit")
        }
        # Keyed by the record each task is reviewed under, so a verdict reaches
        # exactly the one task whose subject it is.
        by_record: dict[tuple[str, str], tuple[str, str]] = {}
        for status, beads in by_status.items():
            for bead in beads:
                if not isinstance(bead, dict):
                    continue
                identity = (str(bead.get("id")), status)
                subject = doc_subject_of(bead)
                if subject:
                    topic, document = subject
                    by_record[(topic, record_stem(document))] = identity
                    continue
                anchor = anchor_of(bead)
                if anchor:
                    # A task-anchor record carries the topic in its own task title
                    # only loosely, so match it under every topic; the stem is what
                    # disambiguates.
                    by_record[("", anchor)] = identity
        for rel in touched_reviews:
            subject = review_subject(rel)
            if not subject:
                continue
            topic, stem = subject
            verdict, gate = latest_review_state(root / rel)
            if verdict != "PASS" or gate is None:
                continue
            task = by_record.get((topic, stem)) or by_record.get(("", stem))
            if not task:
                continue
            task_id, status = task
            if gate in {"VERIFIED", "NOT_REQUIRED"} and status == "in_review":
                notes.append(
                    f"{rel} records Verdict: PASS, but its subject's task "
                    f"{task_id} ({topic}) is still `in_review` even though the "
                    f"completion gate is `{gate}`. Move it to `awaiting_commit`; "
                    f"see .agent-instructions/beads.md."
                )
            elif gate in {"NOT_VERIFIED", "FAILED", "BLOCKED"} and status == "awaiting_commit":
                notes.append(
                    f"{rel} records Verdict: PASS, but its subject's task "
                    f"{task_id} ({topic}) is `awaiting_commit` while the completion "
                    f"gate is `{gate}`. It must remain `in_review` until the gate is "
                    f"VERIFIED; see .agent-instructions/beads.md."
                )

    # 2. A task is parked at `awaiting_commit` while the tree is clean. Either
    #    the commit happened and nobody closed the task, or the work is not
    #    actually in the tree. Both are worth a look; neither is guessable here.
    awaiting = (
        bd_json(["list", "--status", "awaiting_commit"], deadline) or []
        if not changed
        else []
    )
    if awaiting:
        ids = sorted(
            str(b.get("id")) for b in awaiting if isinstance(b, dict) and b.get("id")
        )
        if ids:
            notes.append(
                f"{', '.join(ids)} sit at `awaiting_commit` with a clean tree. "
                f"If the authorized commit already landed, close them; "
                f"`awaiting_commit` means delivery is still pending."
            )

    # 3. A topic document changed while its task says nobody is producing it.
    #    Writing the document IS the `in_progress` state.
    touched_docs = [
        p
        for p in changed
        if p.startswith("docs/topics/") and "/reviews/" not in p and p.endswith(".md")
    ]
    if touched_docs:
        openish = bd_json(["list", "--status", "open"], deadline) or []
        idle: dict[tuple[str, str], str] = {}
        for bead in openish:
            if not isinstance(bead, dict):
                continue
            subject = doc_subject_of(bead)
            if subject:
                idle[subject] = str(bead.get("id"))
        for rel in touched_docs:
            parts = Path(rel).parts
            if len(parts) < 4 or parts[0] != "docs" or parts[1] != "topics":
                continue
            # docs/topics/<topic>/<document>, where <document> may be `ux/<x>.md`
            subject = (parts[2], "/".join(parts[3:]))
            stuck = idle.get(subject)
            if not stuck:
                continue
            notes.append(
                f"{rel} is being edited, but its task {stuck} "
                f"({parts[2]}) is still `open`. Drafting a document is the "
                f"`in_progress` state, and it should be claimed."
            )

    # 4. A topic's decomposition passed review, so its anchors are dispatchable —
    #    but no task exists to dispatch. `.agent-instructions/beads.md` allows
    #    development tasks to be created only after `tasks.md` passes, and nothing
    #    creates them when it does, so the window between the two is exactly where
    #    they get forgotten. A `开发：` command then has no task ID to claim.
    plans = sorted((root / "docs" / "topics").glob("*/tasks.md"))
    pending: list[tuple[str, str]] = []
    for plan in plans:
        try:
            text = plan.read_text(encoding="utf-8")
        except OSError:
            continue
        if not decomposition_passed(text):
            continue
        topic = plan.parent.name
        for anchor, _dev, review in matrix_rows(text):
            if review != "[x]":
                pending.append((topic, anchor))
    if pending:
        known = bd_json(["list", "--all"], deadline) or []
        have = set()
        for bead in known:
            if not isinstance(bead, dict):
                continue
            name = anchor_of(bead)
            if name:
                have.add(name)
        missing = [(t, a) for t, a in pending if a not in have]
        for topic, anchor in missing:
            notes.append(
                f"{topic} passed its `tasks.md` review, but its task `{anchor}` "
                f"has no Beads task. Development tasks are created once the "
                f"decomposition passes; until one exists there is nothing for "
                f"`开发：{topic} / {anchor}` to claim."
            )

    # 5. A live task's description restates the status lifecycle using vocabulary
    #    the contract has retired. A description is data, but an agent dispatched
    #    to the task reads it as instruction — it arrives attached to the very
    #    work it is describing, which makes it look more authoritative than a
    #    file the agent has to go find. That is how the retired names outlived
    #    the contract: `.agent-instructions/beads.md` replaced them, and every
    #    copy already stamped into a description stayed behind, unreferenced by
    #    anything that would notice. Closed tasks are history and are left alone.
    stale_terms = tuple(sorted(RETIRED_STATUS_NAMES))
    live = bd_json(["list", "--status", ",".join(sorted(LIVE_STATUSES))], deadline) or []
    for bead in live:
        if not isinstance(bead, dict):
            continue
        description = str(bead.get("description") or "")
        hits = [term for term in stale_terms if re.search(rf"\b{term}\b", description)]
        if not hits:
            continue
        notes.append(
            f"{bead.get('id')} describes its own lifecycle with retired status "
            f"name(s) {', '.join(f'`{h}`' for h in hits)}. The lifecycle belongs "
            f"to .agent-instructions/beads.md; point at it from the description "
            f"instead of restating it, so the next contract change does not have "
            f"to be backfilled into every task."
        )

    # A record reached PASS while one of its findings has no carrier. PASS does
    # not require zero findings; it requires zero ownerless ones. See
    # `.agent-instructions/review-records.md`, "Findings must reach a carrier
    # before PASS". Only records touched in this working tree are scanned, so
    # this stays a check on work in progress rather than a repository audit.
    for rel in changed:
        if not rel.endswith(".md"):
            continue
        if "/reviews/" not in rel and not rel.startswith("docs/fixes/"):
            continue
        record = root / rel
        if not record.is_file():
            continue
        if latest_verdict(record) != "PASS":
            continue
        stranded = ownerless_findings(record)
        if stranded:
            notes.append(
                f"{rel} ends in PASS while {', '.join(stranded)} "
                f"{'has' if len(stranded) == 1 else 'have'} no carrier. Close "
                f"the finding by naming its ID in a later round, or give it a "
                f"Beads issue or a roadmap.md Backlog item on its own bullet. "
                f"A bare `-> open` is not a destination: this record retires "
                f"with its topic and nobody reads an archived record looking "
                f"for outstanding work."
            )

    return notes


def report_context(notes: list[str]) -> str:
    return "\n".join(
        [
            "Beads coordination state disagrees with the working tree.",
            "Reconcile it now, in this turn, before reporting the work complete:",
            *(f"- {note}" for note in notes),
            "Beads owns dispatch only. Do not change review verdicts, plan status,",
            "or CEv1 evidence to make them agree with it.",
        ]
    )


def report_output(
    notes: list[str], stop_hook_active: bool = False
) -> dict[str, Any]:
    """Shape one report. The transport, not the shape, differs per runtime.

    `systemMessage` reaches the user; it does not reach the model and does not
    hold the turn. Reporting only through it is how this check lost its effect —
    for six consecutive review rounds it matched correctly and no agent ever saw
    a word of it. A report the actor cannot read is not a check.

    `decision: "block"` with `reason` is the channel that reaches the model and
    keeps the turn open until the mismatch is reconciled, which is the point of
    noticing it at Stop rather than later. `emit_output` carries it to each
    runtime the way that runtime accepts.

    `stop_hook_active` means the turn is already continuing because of this
    hook. Blocking again would loop with no exit, so the second pass reports and
    releases.
    """
    context = report_context(notes)
    if stop_hook_active:
        return {"systemMessage": context}
    return {"decision": "block", "reason": context}


def emit_output(output: dict[str, Any], event_name: object, runtime: str) -> int:
    """Deliver one report over the transport its runtime supports.

    Claude Code reads the JSON. Codex does not parse blocker JSON on Stop — it
    takes the reason on stderr with exit code 2 — so sending it JSON is the same
    as saying nothing. This mirrors `emit_output` in the `development-workflow`
    and `handoff-sync` hooks, which were moved to this transport in ai-tools
    `1aa8c06`; this hook is the one that was left on the user-facing field.
    """
    if (
        runtime == "codex"
        and event_name == "Stop"
        and output.get("decision") == "block"
    ):
        reason = output.get("reason")
        if isinstance(reason, str) and reason:
            print(reason, file=sys.stderr)
            return 2
    print(json.dumps(output, ensure_ascii=False, separators=(",", ":")))
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--runtime", choices=("codex", "claude"), required=True)
    args = parser.parse_args()

    try:
        event = json.load(sys.stdin)
    except (json.JSONDecodeError, OSError):
        return 0
    if not isinstance(event, dict) or event.get("hook_event_name") != "Stop":
        return 0
    if authorization_wait(event):
        return 0
    stop_hook_active = bool(event.get("stop_hook_active"))

    deadline = time.monotonic() + HOOK_BUDGET
    root = repo_root(deadline)
    if root is None:
        return 0

    try:
        notes = findings(root, deadline)
    except Exception:  # noqa: BLE001 - a hook must never break the session
        return 0
    if not notes:
        return 0

    return emit_output(
        report_output(notes, stop_hook_active), "Stop", args.runtime
    )


if __name__ == "__main__":
    raise SystemExit(main())
