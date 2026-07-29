---
status: historical
plan: project-attribution
task: attribution-contract
---

# Review log — project-attribution / attribution-contract

## Round 1 — 2026-07-29

Independent review of the uncommitted worktree raising `docs/specs/cli-design.md`
to version 20. Judged against the task's acceptance criterion: every behavior the
other tasks shipped is described, and every behavior described is shipped.

### Findings

- [P2] The fail-open sentence describes behavior `agentdeck run` does not have.
  `docs/specs/cli-design.md:709-712` says "Missing or unreadable state, no
  completed selection, a direct selection, a stale wrapper URL, or any other
  wrapper kind produces no attribution and never blocks client launch." Probed
  with a fake `codex` first on `PATH` and an empty state directory,
  `agentdeck run codex --` exits 1 with `no provider selection for client` and
  the fake client never runs — no `CLIENT_STARTED` line. "Never blocks client
  launch" holds for the `shell-init` helper, which resolves best-effort and
  execs the real binary regardless, but not for `agentdeck run`, which requires
  a completed selection before it launches anything. The contract merges two
  delivery paths with different preconditions into one guarantee, and one half
  of it is not shipped.

- [P2] The `--kind` declaration is absent from the section that owns wrapper
  behavior. "Provider Wrappers" (`cli-design.md:472-522`) treats the command
  surface as part of the contract and spells it out:

  ```text
  agentdeck provider set-wrapper <provider> --url <url>
  agentdeck provider set-wrapper <provider> --clear
  agentdeck provider use <name> [--via]
  ```

  `--kind headroom|plain` is missing, and so are its default (`plain`), the
  rule that `--clear` removes the declaration along with the URL, and its
  additive reporting in `provider list|show|status`. Those are exactly what
  `headroom-wrapper-kind` shipped. The consequence is internal: the new
  "Project Attribution" section leans on the phrase "declared `headroom`" five
  times without the spec ever saying how a wrapper comes to be declared.
  `docs/specs/cli-manual.md:65,102-126` documents the flag properly, so this is
  a spec self-containment gap rather than a missing user-facing document.

- [P3] `docs/README.md:238` records the plan as
  `active — 5/6 reviewed; final task awaiting review`. The same file's Document
  Conventions require `active — X/N done` and warn against copying per-task grid
  state into the index — "that duplicate is how a status document and its
  checklists drift apart". Under the stated definition of *done* (last required
  gate ticked), the value is `active — 5/6 done`.

- [P3] The contract does not record `shell-init`'s output boundary, which
  `shell-helpers` shipped and pinned with tests: the command writes the script
  to stdout only, is deliberately outside the GUI JSON data contract alongside
  `completion`, and still answers a syntax error with the standard envelope
  (exit 2, `command: shell-init`, `code: invalid_argument`). Whether that
  belongs in a contract or only in the manual is a judgment call; recording the
  decision either way would close it.

### Strengths

- The Codex mapping boundary is precise and matches the implementation exactly.
  Probed end to end: a `--via` switch to a `headroom` wrapper produced
  `env_http_headers = { "X-Headroom-Project" = "HEADROOM_PROJECT", "X-Unrelated" = "OTHER_ENV" }`,
  and a subsequent direct switch left `{ "X-Unrelated" = "OTHER_ENV" }` — it
  removed only the attribution entry, exactly as `cli-design.md:640-643` claims.
  `TestCodexProjectHeadersDisabledLeavesUnrelatedInlineFieldUntouched` backs the
  stronger "never reorders" wording by pinning an unrelated inline field's
  original order, spacing, and trailing comment byte for byte.
- The `Owned Client Configuration Fields` table's Claude cell — "None;
  attribution is launch-environment only" — draws the sharpest line in the
  contract. Confirmed: a `--via` Claude switch wrote only `ANTHROPIC_AUTH_TOKEN`
  and `ANTHROPIC_BASE_URL`, left a pre-existing `ANTHROPIC_CUSTOM_HEADERS` value
  untouched, and preserved an unrelated top-level key.
- Version 20, the changelog row, the prose, and Invariant 50 agree with each
  other, and the version bump was chased through every referring document,
  including `docs/README.md`'s display-timezone paragraph. It also corrected a
  pre-existing staleness there: the documents table had still said "Currently
  version 18".
- The delivery-mechanism list assigns responsibility unambiguously — who writes
  what, and that AgentDeck writes nothing in mechanisms 2 and 3.

### Independent verification

- Probed the built worktree binary: Codex `--via` then direct switch against a
  `config.toml` carrying an unrelated `X-Unrelated` mapping; Claude `--via`
  against a `settings.json` carrying a pre-existing `ANTHROPIC_CUSTOM_HEADERS`
  and an unrelated key.
- Probed `agentdeck run codex --` with no completed selection and a fake client
  first on `PATH`: exit 1, client not launched.
- Read `internal/provider/config.go:212-249,383-403` and
  `internal/provider/config_test.go:484-514` to confirm the preservation and
  ordering claims.
- Local Markdown link check over `cli-design.md`, `cli-manual.md`,
  `docs/README.md`, the plan, and this directory's other record —
  `local Markdown links OK (5 files)`.
- `git diff --check` — exit 0.
- Confirmed no document still refers to specification version 19.

**Verdict: REOPEN.** Two P2 findings remain open.

## Round 2 — 2026-07-29

### Re-review

All four Round 1 findings are closed, and the two P2 corrections were verified
against the running binary rather than read.

**[P2] Fail-open split by delivery path — closed.** `cli-design.md:715-721` now
states separately that a completed selection for the requested client is a
launch precondition for `agentdeck run` — "without one, the command exits with
an error and does not start the client" — that attribution failure after
`agentdeck run` has decided to launch does not block that process, and that the
shell helper is fail-open across the wider set of outcomes. Each clause matches
what was measured: `agentdeck run` with no selection exits 1 without running the
fake client, while a configured provider on a direct switch launches it with no
attribution.

**[P2] `--kind` now defined where wrappers are defined — closed.** The command
block at `cli-design.md:494-498` carries `[--kind headroom|plain]`, followed by
the `plain` default, `--clear` removing both URL and kind, and additive
reporting in `provider list|show|status`. Every clause probed on the built
binary:

| Probe | Result |
| --- | --- |
| `set-wrapper --url` with no `--kind` | `provider show` JSON has `wrapper_url` only — no `wrapper_kind`, matching the manual's "plain is indistinguishable from undeclared" |
| `set-wrapper --url --kind headroom` | `show`/`status` JSON both carry `wrapper_kind: headroom`; `show` text prints `wrapper: https://wrapper.example (headroom)` |
| `provider list` | JSON entry carries `wrapper_kind: headroom`; text column prints `https://wrapper.example (headroom)` |
| `set-wrapper --clear` | both `wrapper_url` and `wrapper_kind` disappear |

The spec's "report the kind as additive wrapper metadata" is accurate for all
three commands, including `list`, which the wording explicitly names.

**[P3] Index rollup — closed.** `docs/README.md:236` reads `active — 5/6 done`,
the format the file's own conventions prescribe.

**[P3] `shell-init` output boundary — closed by recording the decision.**
`cli-design.md:750-754` states that, like `completion`, the command is
deliberately outside the GUI JSON data contract because stdout is the shell
program, while argument errors keep the standard envelope (`exit 2`,
`command: shell-init`, `code: invalid_argument`) — the same three values probed
in Round 1 of the `shell-helpers` review.

### Observation, not a finding

The version 20 changelog row describes the attribution contract but not that
v20 is also the first version to write down `--kind`, its default, and its
reporting. The behavior shipped under `headroom-wrapper-kind` and is unchanged;
only its documentation is new, so the row is not wrong. Worth a clause if that
section is edited again.

### Independent verification

- Probed wrapper kind default, `headroom` declaration, `list`/`show`/`status`
  reporting in both JSON and text, and `--clear` removal, on the built binary.
- Local Markdown link check over the spec, manual, index, plan, and this record
  — `local Markdown links OK (5 files)`.
- No document references `Currently version 18/19` or `cli-design.md v19`;
  `docs/specs/cli-design.md` frontmatter reads `version: 20`.
- `git diff --check` — exit 0.
- No Go source changed in this task; `git diff --stat` shows only
  `docs/specs/cli-design.md`, `docs/README.md`, and the plan moving since the
  reviewed `shell-helpers` state.

**Verdict: PASS.** The contract describes what shipped, and every behavior it
describes was confirmed shipped. This was the plan's final task.
