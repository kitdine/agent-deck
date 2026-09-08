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
only reports; it never writes to Beads. A detected mismatch may block Stop once.

UserPromptSubmit records explicit session scope; Stop checks only that scope.
Unattributed work is left for the explicit --audit command, which never blocks.
It never writes to Beads. A disagreement holds the turn open in both runtimes so
the agent reconciles it before finishing — Claude Code through the blocker JSON,
Codex through the stderr exit-code-2 transport it accepts. Either way the report
must reach the actor that can act on it, which a user-facing message alone does
not.
"""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
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
# that supplies its own -C and explicit audit actor when invoking the binary.
# An agent MUST instead use the wrapper, which requires an actor and
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
VERDICT = re.compile(
    r"^[ \t]*(?:[-*]\s+|✅\s*)?(?:Verdict|结论|裁决|评审结论|复评结论)"
    r"\s*[:：]\s*(PASS|FAIL|REOPEN)\b", re.IGNORECASE | re.MULTILINE
)
COMPLETION_GATE = re.compile(
    r"^[ \t]*(?:[-*]\s+)?(?:Completion gate|完成门禁|证据门禁|验收门禁)"
    r"\s*[:：]\s*(VERIFIED|NOT_VERIFIED|FAILED|BLOCKED|NOT_REQUIRED)\b",
    re.IGNORECASE | re.MULTILINE,
)
REVIEW_ROUND = re.compile(
    r"^##\s+(?:(?:Review\s*[—–-]\s*)?Round\s+\d+\b|(?:评审|复评)\s*[—–-]?\s*第?\d+轮)",
    re.IGNORECASE | re.MULTILINE,
)
# A finding ID as `.agent-instructions/review-records.md` defines it: A6-F1,
# DW-R11-F2, D1-F1. The audit that produced that rule found 103 of them across
# every review record.
FINDING_ID = re.compile(r"\b((?:[A-Z][A-Z0-9]*-)*[A-Z]+[0-9]+-F[0-9]+)\b")
# Words a later round actually uses to close one, gathered from the records
# themselves rather than invented: `A1-F1 closed:`, `A6-F1 — SUPERSEDED.`
FINDING_CLOSED = re.compile(
    r"repaired|closed|resolved|addressed|fixed|superseded|已修复|已关闭|已解决|已处理",
    re.I,
)
# Group dispositions occur on both sides of the IDs in existing records:
# `all closed: A1-F1 ...` and `A1-F1、A1-F2 均已关闭`.
FINDING_SUFFIX_GROUP = re.compile(r"\b(?:all|both|are|were)\b|均|都|全部", re.I)
FINDING_CLAUSE_BREAK = re.compile(r"[.;。；](?:[*_`]+)?\s+")
FINDING_REFERENCE_CLAUSE = re.compile(r"^(?:see\b|参见)", re.I)
# A carrier is a Beads issue or a named Backlog item. Nothing else counts,
# because nothing else is read again after the record is archived.
FINDING_CARRIER = re.compile(r"\bad-[a-z0-9][a-z0-9-]*\b|roadmap\.md Backlog:")

AUTHORIZATION_WAIT = re.compile(
    r"(?m)^WORKFLOW_AUTHORIZATION_WAIT:\s*"
    r"[A-Za-z0-9][A-Za-z0-9._:/-]*(?:\s+[A-Za-z0-9_-]+)?[ \t]*$"
)

TIMEOUT = 8
HOOK_BUDGET = 10.0


def session_state_path(root: Path, event: dict[str, Any], runtime: str) -> Path | None:
    session = event.get("session_id")
    if not isinstance(session, str) or not session:
        return None
    identity = "\0".join((repository_identity(root), runtime, session))
    base = Path(os.environ.get("AGENTDECK_BEADS_HOOK_STATE_DIR", str(
        Path(os.environ.get("XDG_STATE_HOME", str(Path.home() / ".local/state")))
        / "agentdeck/beads-hook"
    )))
    return base / (hashlib.sha256(identity.encode()).hexdigest() + ".json")


def event_turn(event: dict[str, Any], runtime: str) -> str | None:
    fields = ("turn_id", "prompt_id") if runtime == "codex" else ("prompt_id", "turn_id")
    return next((event[k] for k in fields if isinstance(event.get(k), str) and event[k]), None)


def read_session(path: Path | None) -> dict[str, Any]:
    if path is None:
        return {}
    try:
        data = json.loads(path.read_text())
        return data if isinstance(data, dict) else {}
    except (OSError, ValueError):
        return {}


def write_session(path: Path, state: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    temporary = path.with_suffix(f".{os.getpid()}.tmp")
    try:
        temporary.write_text(json.dumps(state, ensure_ascii=False))
        temporary.chmod(0o600)
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


def valid_scope(scope: object) -> bool:
    if not isinstance(scope, dict):
        return False
    topic, target = scope.get("topic"), scope.get("subject")
    if not isinstance(topic, str) or not re.fullmatch(r"[a-z0-9][a-z0-9-]*", topic):
        return False
    if not isinstance(target, str):
        return False
    if target and (not re.fullmatch(r"[a-zA-Z0-9_./-]+", target)
                   or any(p in {"", ".", ".."} for p in target.split("/"))):
        return False
    return topic != "fix" or bool(re.fullmatch(r"[a-z0-9][a-z0-9-]*", target))


def repository_identity(root: Path) -> str:
    try:
        result = subprocess.run(
            ("git", "rev-parse", "--path-format=absolute", "--git-common-dir"),
            cwd=root,
            capture_output=True,
            text=True,
            timeout=3,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        result = None
    if result is not None and result.returncode == 0 and result.stdout.strip():
        return f"git:{Path(result.stdout.strip()).resolve()}"
    return f"path:{root.resolve()}"


def workflow_router(runtime: str) -> Any | None:
    hook = Path(os.environ.get("AGENTDECK_WORKFLOW_HOOK", str(
        Path.home() / f".{runtime}/hooks/development-workflow/workflow_hook.py"
    )))
    try:
        spec = importlib.util.spec_from_file_location("beads_workflow_router", hook)
        if spec is None or spec.loader is None:
            return None
        router = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(router)
        return router
    except (OSError, ValueError, AttributeError, ImportError):
        return None


def selected_workspace_root(
    prompt: str, runtime: str, fallback: Path
) -> Path | None:
    router = workflow_router(runtime)
    if router is None or not hasattr(router, "workspace_binding_for_prompt"):
        return fallback
    binding, error = router.workspace_binding_for_prompt(prompt, str(fallback))
    if error is not None:
        return None
    if binding is None:
        return fallback
    path = Path(binding["workspace_path"])
    return path if path.is_dir() and (path / REPO_MARKER).is_file() else None


def selected_scope(prompt: str, runtime: str) -> dict[str, str] | None:
    """Reuse the workflow's command matcher; this observer never starts a phase."""
    router = workflow_router(runtime)
    if router is None:
        return None
    try:
        if router.route_prompt(prompt) not in {"DESIGN", "IMPLEMENT", "REVIEW", "REPAIR", "REREVIEW"}:
            return None
        text = prompt.lstrip()
        for prefix in router.INVOCATION_PREFIXES:
            if text.startswith(prefix) and text[len(prefix):len(prefix)+1].isspace():
                text = text[len(prefix):].lstrip()
                break
        command = next(c for c in sorted(router.COMMAND_ROUTES, key=len, reverse=True)
                       if text.startswith(c))
        subject = text[len(command):].lstrip("\r\n:：").strip().splitlines()[0]
        parts = re.split(r"\s+/\s+", subject)
        topic = parts[0]
        target = parts[1] if len(parts) > 1 else ""
        scope = {"topic": topic, "subject": target}
        return scope if valid_scope(scope) else None
    except (ValueError, AttributeError, IndexError, StopIteration):
        return None


def scope_paths(scope: dict[str, str]) -> list[str]:
    topic, subject = scope["topic"], scope["subject"]
    if topic == "fix":
        return [f"docs/fixes/{subject}.md"]
    base = f"docs/topics/{topic}/"
    if not subject:
        return [base]
    if subject.startswith("reviews/"):
        return [base + subject]
    if subject.endswith(".md"):
        return [base + subject, base + "reviews/" + record_stem(subject) + ".md"]
    return [base + "reviews/" + subject + ".md"]


def in_scope(path: str, scope: dict[str, str]) -> bool:
    return any(path.startswith(p) if p.endswith("/") else path == p for p in scope_paths(scope))


def remember_prompt(root: Path, event: dict[str, Any], runtime: str) -> None:
    path = session_state_path(root, event, runtime)
    prompt = event.get("prompt")
    if path is None or not isinstance(prompt, str):
        return
    old = read_session(path)
    if prompt.strip().lower() in {"继续", "继续开发", "继续执行", "continue"}:
        state = old
    else:
        scope = selected_scope(prompt, runtime)
        state = {"scope": scope, "workspace_root": str(root.resolve())}
        if scope and scope == old.get("scope"):
            state["reported"] = old.get("reported")
    if not valid_scope(state.get("scope")):
        path.unlink(missing_ok=True)
        return
    state["turn"] = event_turn(event, runtime)
    state["workspace_root"] = str(root.resolve())
    write_session(path, state)


def report_fingerprint(root: Path, scope: dict[str, str], notes: list[str]) -> str:
    # Bind deduplication to the current scoped document content, not timestamps.
    paths = scope_paths(scope)
    if scope["topic"] != "fix":
        paths.append(f"docs/topics/{scope['topic']}/tasks.md")
    files: set[Path] = set()
    for rel in paths:
        path = root / rel
        files.update(path.rglob("*.md") if rel.endswith("/") else [path])
    digest = hashlib.sha256(json.dumps(notes).encode())
    for path in sorted(files):
        digest.update(str(path.relative_to(root)).encode())
        try:
            digest.update(path.read_bytes())
        except OSError:
            digest.update(b"missing")
    return digest.hexdigest()


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

    Read the whole latest round, including nested Skill report headings and
    metadata after the verdict. Never borrow a result from an earlier round.
    """
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return None, None
    # Fenced examples are not declarations of the current review state.
    visible: list[str] = []
    fence = ""
    for line in text.splitlines():
        marker = re.match(r"^ {0,3}(`{3,}|~{3,})(.*)$", line)
        if marker:
            run, tail = marker.groups()
            if not fence:
                fence = run
            elif run[0] == fence[0] and len(run) >= len(fence) and not tail.strip():
                fence = ""
            continue
        if not fence:
            visible.append(line.replace("**", "").replace("__", "").replace("`", ""))
    text = "\n".join(visible)
    rounds = list(REVIEW_ROUND.finditer(text))
    if not rounds:
        rounds = list(re.finditer(r"^##\s+📋", text, re.MULTILINE))
    section = text[rounds[-1].start():] if rounds else text
    verdicts = {value.upper() for value in VERDICT.findall(section)}
    gates = {value.upper() for value in COMPLETION_GATE.findall(section)}
    if len(verdicts) != 1 or len(gates) > 1:
        return None, None
    return next(iter(verdicts)), next(iter(gates)) if gates else None


def latest_verdict(path: Path) -> str | None:
    return latest_review_state(path)[0]


def finding_clauses(lines: list[str]) -> list[str]:
    """Join wrapped Markdown bullets, then split their disposition clauses."""
    blocks: list[str] = []
    current: list[str] = []
    for line in lines:
        stripped = line.strip()
        if not stripped:
            if current:
                blocks.append(" ".join(current))
                current = []
            continue
        if line.startswith("#") or re.match(r"^\s*-\s", line):
            if current:
                blocks.append(" ".join(current))
            current = []
        current.append(stripped)
    if current:
        blocks.append(" ".join(current))
    return [
        clause
        for block in blocks
        for clause in FINDING_CLAUSE_BREAK.split(block)
        if clause
    ]


def direct_disposition_bridge(value: str) -> bool:
    """Whether only Markdown punctuation or a small copula joins ID and state."""
    if re.search(r"(?:->|[—–-])\s*(?:[*_`]+)?$", value):
        return True
    normalized = re.sub(r"[\s`*_~\[\](){}<>:：,，\-—–>]", "", value).lower()
    return normalized in {"", "is", "was", "isnow", "hasbeen", "noregression", "处置"}


def closed_finding_ids(lines: list[str]) -> set[str]:
    """Bind closure tokens to direct or explicitly grouped finding clauses."""
    closed: set[str] = set()
    for clause in finding_clauses(lines):
        identifiers = list(FINDING_ID.finditer(clause))
        if not identifiers:
            continue
        if FINDING_REFERENCE_CLAUSE.match(clause):
            closed.update(match.group(1) for match in identifiers)
            continue
        for disposition in FINDING_CLOSED.finditer(clause):
            before = [match for match in identifiers if match.end() <= disposition.start()]
            after = [match for match in identifiers if match.start() >= disposition.end()]

            if not before and after:
                closed.update(match.group(1) for match in after)
                continue

            if len(before) > 1 and disposition.start() >= before[-1].end():
                group = clause[before[0].start() : disposition.start()]
                suffix = clause[disposition.end() : disposition.end() + 24]
                if FINDING_SUFFIX_GROUP.search(group + suffix):
                    closed.update(match.group(1) for match in before)
                    continue

            if before:
                match = before[-1]
                if direct_disposition_bridge(clause[match.end() : disposition.start()]):
                    closed.add(match.group(1))
                    continue

    return closed


def ownerless_findings(path: Path) -> list[str]:
    """Finding IDs in `path` that are neither closed nor carried.

    A review record retires with its topic. Once it is under `docs/archive/`,
    nobody opens it looking for outstanding work, so a finding left with a bare
    `-> open` stops existing the moment the topic is archived. `A6-F1` is the
    opposite regression: Round 8 marks it SUPERSEDED, which is a real closure.

    A finding is accounted for when a logical clause directly or collectively
    disposes of its ID, or when its own bullet names a carrier. Prefix closure
    applies to the IDs that follow in that clause; suffix closure applies to the
    nearest preceding ID unless an explicit group marker covers its siblings.
    The bullet is read to its end rather than one line, because these findings
    run several lines and the carrier is usually on the last of them.
    """
    try:
        lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    except OSError:
        return []
    first_seen: dict[str, int] = {}
    for number, line in enumerate(lines):
        for match in FINDING_ID.finditer(line):
            first_seen.setdefault(match.group(1), number)
    closed = closed_finding_ids(lines)
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


def head_text(root: Path, rel: str, deadline: float) -> str:
    """The committed content of one path, or `""` when it has none.

    A path that is new, untracked, or unreadable has nothing recorded for it
    yet, and an empty string is the honest reading of that rather than a reason
    to report.
    """
    timeout = remaining_timeout(deadline)
    if timeout is None:
        return ""
    try:
        out = subprocess.run(
            ["git", "-C", str(root), "show", f"HEAD:{rel}"],
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    return out.stdout if out.returncode == 0 else ""


def decomposition_changed(root: Path, rel: str, deadline: float) -> bool:
    """True when an edit to a topic's `tasks.md` touched its Tasks matrix.

    `matrix_rows` reads task rows only — the Documents matrix never matches it,
    because its subject cell carries no backticks — so a Draft tick, a row
    added to the document set, or any prose change around them leaves this
    False.
    """
    try:
        current = (root / rel).read_text(encoding="utf-8")
    except OSError:
        return False
    return matrix_rows(current) != matrix_rows(head_text(root, rel, deadline))


def decomposition_passed(text: str) -> bool:
    """True when the topic's own `tasks.md` row shows a ticked Review cell."""
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("| tasks.md |"):
            continue
        cells = [c.strip() for c in line.strip("|").split("|")]
        return len(cells) >= 3 and cells[-1] == "[x]"
    return False


def implementation_subject(
    root: Path, bead: dict[str, Any], deadline: float
) -> tuple[str, str] | None:
    """Resolve an anchor through its unique structured decomposition dependency.

    Description links are references, never ownership. List responses may omit
    dependency details; load only this candidate's detail within the Hook budget.
    Multiple decompositions or a missing matrix anchor remain unattributed.
    """
    anchor = anchor_of(bead)
    if not anchor:
        return None
    if "dependencies" not in bead:
        task_id = bead.get("id")
        if not isinstance(task_id, str) or not task_id:
            return None
        detail = bd_json(["show", task_id], deadline)
        if isinstance(detail, list) and len(detail) == 1:
            detail = detail[0]
        if not isinstance(detail, dict) or detail.get("id") != task_id or anchor_of(detail) != anchor:
            return None
        bead = detail
    dependencies = bead.get("dependencies")
    if not isinstance(dependencies, list):
        return None
    topics = set()
    for dependency in dependencies:
        if not isinstance(dependency, dict) or dependency.get("dependency_type") != "blocks":
            continue
        subject = doc_subject_of(dependency)
        if subject and subject[1] == "tasks.md":
            topics.add(subject[0])
    if len(topics) != 1:
        return None
    topic = next(iter(topics))
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", topic):
        return None
    try:
        matrix = (root / "docs/topics" / topic / "tasks.md").read_text(encoding="utf-8")
    except OSError:
        return None
    return (topic, anchor) if any(row[0] == anchor for row in matrix_rows(matrix)) else None


def findings(root: Path, deadline: float, scope: dict[str, str] | None = None) -> list[str]:
    all_changed = changed_paths(root, deadline)
    changed = [p for p in all_changed if in_scope(p, scope)] if scope else all_changed
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
        by_record: dict[tuple[str, str], set[tuple[str, str]]] = {}
        reviewed = {review_subject(path) for path in touched_reviews}
        for status, beads in by_status.items():
            for bead in beads:
                if not isinstance(bead, dict):
                    continue
                identity = (str(bead.get("id")), status)
                subject = doc_subject_of(bead)
                if subject:
                    topic, document = subject
                    by_record.setdefault((topic, record_stem(document)), set()).add(identity)
                    continue
                anchor = anchor_of(bead)
                if anchor and any(subject and subject[1] == anchor for subject in reviewed):
                    subject = implementation_subject(root, bead, deadline)
                    if subject in reviewed:
                        by_record.setdefault(subject, set()).add(identity)
        for rel in touched_reviews:
            subject = review_subject(rel)
            if not subject:
                continue
            topic, stem = subject
            verdict, gate = latest_review_state(root / rel)
            if verdict != "PASS" or gate is None:
                continue
            candidates = by_record.get((topic, stem), set())
            if len(candidates) != 1:
                continue
            task_id, status = next(iter(candidates))
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
        if not all_changed and scope is None
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
    #
    #    `tasks.md` is the one document in the set that is both a deliverable and
    #    the topic's only status authority, so a path match alone cannot tell the
    #    two apart. Every stage writes its Documents matrix — stage 1 creates it,
    #    each later stage ticks the row it just drafted — while the task named
    #    here produces only the Tasks matrix, at stage 8. Reporting a Documents
    #    matrix edit asked an agent to claim decomposition that had not started,
    #    which is the same false dispatch state this hook exists to catch, in the
    #    other direction. So for `tasks.md`, and only for it, the trigger is the
    #    Tasks matrix actually changing.
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
            if subject[1] == "tasks.md" and not decomposition_changed(root, rel, deadline):
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
    if scope is None:
        plans = sorted((root / "docs" / "topics").glob("*/tasks.md"))
    elif scope["topic"] != "fix" and scope["subject"] in {"", "tasks.md", "reviews/tasks.md"}:
        plans = [root / "docs/topics" / scope["topic"] / "tasks.md"]
    elif scope["topic"] != "fix" and not scope["subject"].endswith(".md"):
        plans = [root / "docs/topics" / scope["topic"] / "tasks.md"]
    else:
        plans = []
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
            selected = not scope or scope["subject"] in {"", "tasks.md", "reviews/tasks.md", anchor}
            if review != "[x]" and selected:
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
    live = (bd_json(["list", "--status", ",".join(sorted(LIVE_STATUSES))], deadline) or []) if scope is None else []
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
    parser.add_argument("--audit", action="store_true", help="Explicit non-blocking repository-wide coordination audit")
    args = parser.parse_args()

    if args.audit:
        deadline = time.monotonic() + HOOK_BUDGET
        root = repo_root(deadline)
        print(json.dumps({"notes": findings(root, deadline) if root else []}, ensure_ascii=False))
        return 0

    try:
        event = json.load(sys.stdin)
    except (json.JSONDecodeError, OSError):
        return 0
    if not isinstance(event, dict) or event.get("hook_event_name") not in {"Stop", "UserPromptSubmit"}:
        return 0
    if authorization_wait(event):
        return 0
    stop_hook_active = bool(event.get("stop_hook_active"))

    deadline = time.monotonic() + HOOK_BUDGET
    default_root = repo_root(deadline)
    if default_root is None:
        return 0

    try:
        if event.get("hook_event_name") == "UserPromptSubmit":
            prompt = event.get("prompt")
            if not isinstance(prompt, str):
                return 0
            root = selected_workspace_root(prompt, args.runtime, default_root)
            if root is None:
                return 0
            remember_prompt(root, event, args.runtime)
            return 0
        path = session_state_path(default_root, event, args.runtime)
        state = read_session(path)
        scope = state.get("scope")
        if not valid_scope(scope):
            return 0
        stored_root = state.get("workspace_root")
        if not isinstance(stored_root, str):
            return 0
        root = Path(stored_root)
        if (
            not root.is_dir()
            or not (root / REPO_MARKER).is_file()
            or repository_identity(root) != repository_identity(default_root)
        ):
            return 0
        turn = event_turn(event, args.runtime)
        if turn != state.get("turn"):
            return 0
        notes = findings(root, deadline, scope=scope)
        fingerprint = report_fingerprint(root, scope, notes) if notes else None
        if fingerprint == state.get("reported"):
            return 0
        state["reported"] = fingerprint
        assert path is not None
        write_session(path, state)
    except Exception:  # noqa: BLE001 - a hook must never break the session
        return 0
    if not notes:
        return 0

    return emit_output(
        report_output(notes, stop_hook_active), "Stop", args.runtime
    )


if __name__ == "__main__":
    raise SystemExit(main())
