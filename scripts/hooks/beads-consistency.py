#!/usr/bin/env python3
"""Report Beads coordination state that the repository has already moved past.

The development-workflow Skill deliberately does not know Beads exists — its
own boundaries say `AGENTS.md` is not a runtime prerequisite — so running a
phase never moves a Beads task by itself. The only thing that does is an agent
remembering AGENTS.md's "Review Artifact Finalization" rules, and a forgotten
transition is invisible: dispatch keeps asserting the previous state. That is
how a task once sat `in_progress` for a day while nothing was being implemented.

This hook closes the loop from the project side rather than by binding the Skill
to an implementation. It compares two facts that are both cheap and unambiguous:
what the working tree shows was just done, and what Beads currently claims. It
only ever reports; it never writes to Beads and never blocks a stop.

Runs on Stop for both Codex and Claude Code. Silent unless something disagrees.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

# This hook encodes one repository's task-title grammar and Beads deployment, so
# it must stay inert anywhere else. Identify that repository by a file it owns,
# never by its path: a checkout lives wherever its owner cloned it, and a path
# match would silently disable the hook for every clone but one.
REPO_MARKER = Path(".agent-instructions/beads.md")
BEADS_ROOT = Path.home() / ".local/state/agentdeck-beads"
BD_BIN = "/usr/local/bin/bd"

# "文档：<topic> / <document>" and "任务：<task-anchor>" — the two title shapes
# .agent-instructions/beads.md defines.
DOC_TITLE = re.compile(r"^文档：\s*([^/\s]+)\s*/\s*(.+)$")
TASK_TITLE = re.compile(r"^任务：\s*(.+)$")

# A review record's verdict line, e.g. "- Verdict: PASS".
VERDICT = re.compile(r"Verdict:\s*(PASS|FAIL|REOPEN)", re.IGNORECASE)

TIMEOUT = 8


def repo_root() -> Path | None:
    try:
        out = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            timeout=TIMEOUT,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if out.returncode != 0:
        return None
    root = Path(out.stdout.strip())
    return root if (root / REPO_MARKER).is_file() else None


def bd_json(args: list[str]) -> Any:
    """Run bd read-only. Any failure yields None — a hook must never be the
    reason a session cannot stop."""
    env = {
        **os.environ,
        "BEADS_DIR": str(BEADS_ROOT / ".beads"),
        "GIT_CONFIG_GLOBAL": str(BEADS_ROOT / "beads.gitconfig"),
        # Reads are audited too, and the wrapper rejects an omitted actor.
        "BEADS_ACTOR": os.environ.get("BEADS_ACTOR") or "consistency-hook",
    }
    try:
        out = subprocess.run(
            [BD_BIN, "-C", str(BEADS_ROOT), *args, "--json"],
            capture_output=True,
            text=True,
            timeout=TIMEOUT,
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


def changed_paths(root: Path) -> list[str]:
    try:
        out = subprocess.run(
            ["git", "-C", str(root), "status", "--porcelain"],
            capture_output=True,
            text=True,
            timeout=TIMEOUT,
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


def latest_verdict(path: Path) -> str | None:
    """The last verdict in a review record — records append rounds, so the
    final one is the current round's."""
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return None
    found = VERDICT.findall(text)
    return found[-1].upper() if found else None


def topic_of(bead: dict[str, Any]) -> str | None:
    title = str(bead.get("title") or "")
    doc = DOC_TITLE.match(title)
    if doc:
        return doc.group(1)
    return None


def anchor_of(bead: dict[str, Any]) -> str | None:
    task = TASK_TITLE.match(str(bead.get("title") or ""))
    return task.group(1).strip() if task else None


def findings(root: Path) -> list[str]:
    changed = changed_paths(root)
    notes: list[str] = []

    # 1. A review record was written or updated, but the task it reviews is
    #    still `in_review`. The verdict exists; dispatch has not heard about it.
    touched_reviews = [p for p in changed if "/reviews/" in p and p.endswith(".md")]
    if touched_reviews:
        in_review = bd_json(["list", "--status", "in_review"]) or []
        by_topic: dict[str, list[str]] = {}
        for bead in in_review:
            if not isinstance(bead, dict):
                continue
            topic = topic_of(bead)
            if topic:
                by_topic.setdefault(topic, []).append(str(bead.get("id")))
        for rel in touched_reviews:
            parts = Path(rel).parts
            # docs/topics/<topic>/reviews/<record>.md
            if len(parts) < 5 or parts[0] != "docs" or parts[1] != "topics":
                continue
            topic = parts[2]
            verdict = latest_verdict(root / rel)
            if verdict != "PASS":
                continue
            stuck = by_topic.get(topic)
            if stuck:
                notes.append(
                    f"{rel} records Verdict: PASS, but {', '.join(sorted(stuck))} "
                    f"({topic}) is still `in_review`. A PASS moves its task to "
                    f"`awaiting_commit`; see .agent-instructions/beads.md."
                )

    # 2. A task is parked at `awaiting_commit` while the tree is clean. Either
    #    the commit happened and nobody closed the task, or the work is not
    #    actually in the tree. Both are worth a look; neither is guessable here.
    awaiting = bd_json(["list", "--status", "awaiting_commit"]) or []
    if awaiting and not changed:
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
        openish = bd_json(["list", "--status", "open"]) or []
        idle: dict[str, list[str]] = {}
        for bead in openish:
            if not isinstance(bead, dict):
                continue
            topic = topic_of(bead)
            if topic:
                idle.setdefault(topic, []).append(str(bead.get("id")))
        seen: set[str] = set()
        for rel in touched_docs:
            parts = Path(rel).parts
            if len(parts) < 4 or parts[0] != "docs" or parts[1] != "topics":
                continue
            topic = parts[2]
            if topic in seen or topic not in idle:
                continue
            seen.add(topic)
            notes.append(
                f"{rel} is being edited, but {', '.join(sorted(idle[topic]))} "
                f"({topic}) is still `open`. Drafting a document is the "
                f"`in_progress` state, and it should be claimed."
            )

    return notes


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--runtime", choices=("codex", "claude"), required=True)
    parser.parse_args()

    try:
        event = json.load(sys.stdin)
    except (json.JSONDecodeError, OSError):
        return 0
    if not isinstance(event, dict) or event.get("hook_event_name") != "Stop":
        return 0

    root = repo_root()
    if root is None:
        return 0

    try:
        notes = findings(root)
    except Exception:  # noqa: BLE001 - a hook must never break the session
        return 0
    if not notes:
        return 0

    context = "\n".join(
        [
            "Beads coordination state disagrees with the working tree.",
            "Reconcile it now, in this turn, before reporting the work complete:",
            *(f"- {note}" for note in notes),
            "Beads owns dispatch only. Do not change review verdicts, plan status,",
            "or CEv1 evidence to make them agree with it.",
        ]
    )
    print(
        json.dumps(
            {
                "hookSpecificOutput": {
                    "hookEventName": "Stop",
                    "additionalContext": context,
                }
            },
            ensure_ascii=False,
            separators=(",", ":"),
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
