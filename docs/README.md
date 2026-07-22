---
status: active
created: 2026-07-14
---

# AgentDeck Documentation

This is both the documentation index and the execution baseline. Decide what to
work on next from this file. Repository code, tests, configuration, and Git
history remain the source of truth when they disagree with any document.

## Current State (2026-07-22)

v0.1.0 is published and installable through `kitdine/homebrew-tap`. Every
follow-up in the retired phase-one plan passed independent review, so there is
no outstanding review debt.

Delivered and reviewed: the Go CLI baseline; provider and credential
management; usage collection, pricing, and run attribution; local session
search; extension inventory; encrypted backup and device migration; unified
ASCII table output; machine-bound encrypted SQLite credential storage;
automatic LiteLLM price updates; active-log-safe usage rebuild; the usage stats
runtime provider dimension; and GitHub release plus Homebrew tap distribution.

Per-task history lives in
[the retired phase-one plan](archive/plans/agentdeck-cli.md). Read it only for
historical detail; it is not a current tracker.

## Documents

| Document | Purpose |
| --- | --- |
| [specs/cli-design.md](specs/cli-design.md) | What the system does and must keep doing: provider, credential, usage, pricing, session, backup, and distribution behavior. Currently version 10; see its changelog. |
| [specs/cli-manual.md](specs/cli-manual.md) | The implemented command surface, flags, and output shapes. |
| [plans/usage-scan-performance.md](plans/usage-scan-performance.md) | Make a full usage re-read fast and visible. Design approved with a profiled baseline. active — 0/6 done. |
| [plans/usage-stats-readability.md](plans/usage-stats-readability.md) | Keep `usage stats` text scannable as data grows. Design approved with a profiled baseline. active — 0/5 done. |
| [plans/test-coverage.md](plans/test-coverage.md) | Repository test coverage queue from the 2026-07-22 gap scan. active — 1/5 done (task 1 passed Round 2 review; task 2 reopened for test repairs). |
| [reviews/](reviews/README.md) | Per-task review records that back each plan's ticked `Review` cell. |
| [archive/](archive/README.md) | Retired plans and superseded contracts. Not a starting point for new work. |

## Open Tasks Not Owned by a Plan

- [ ] Dispatch the Release workflow for the next stable tag (v0.1.1+) and
      confirm the automated Homebrew tap flow works end to end: it must open an
      `agentdeck-<tag>` pull request against `kitdine/homebrew-tap` rather than
      pushing directly, must never fire for prerelease tags, and after the PR
      merges a normal `brew reinstall kitdine/tap/agentdeck` must expose bash,
      zsh, and fish completions. `HOMEBREW_TAP_TOKEN` was configured
      2026-07-22, but this automation has never run end to end — v0.1.0 shipped
      through a manual tap push and is not being retagged, so this can only be
      verified on the next real release.

## Backlog

Candidate work with no approved specification. Each item needs its own plan
before implementation starts; promote it out of this list at that point rather
than expanding the entry in place.

- [ ] Add the ability to switch Claude subscription/account — analogous to the
      existing AI provider switching, but selecting a Claude account or plan
      rather than an API base URL and token.
- [ ] Implement a GUI, including a persistent menu-bar presence, as an
      alternative front end to the CLI.
- [ ] Broaden the bundled fallback price catalog. `internal/usage/model-prices.json`
      currently ships exactly two models — `gpt-5.4` (openai) and
      `claude-sonnet-4-6` (anthropic) — so a fresh install cannot price most
      real usage until the first successful `agentdeck price update` reaches the
      network. Carry a reasonably complete current model set for **both**
      vendors, and define how that bundled set is refreshed over time: who
      regenerates it, from which reviewed source, and on what cadence, most
      plausibly a release-time regeneration step rather than a hand-edited file.
      The catalog version string `2026-07-13-openai-standard-v1` already
      misdescribes its contents now that it carries an Anthropic model; rename
      it as part of this work. Keep the existing provenance and immutability
      contract intact — bundled entries must remain an explicit reviewed layer,
      never a silent guess.
- [ ] Address two defense-in-depth findings from the 2026-07-22 credential
      vault security review. Neither is exploitable today; take them the next
      time `internal/credentialvault/vault.go` is opened.
      (a) **Durability, higher priority despite lower likelihood**
      (`vault.go:244`): `os.Link` is not followed by a parent-directory
      `Sync()`, so the key file's contents are durable but its directory entry
      is not. A crash in that window, after SQLite has already committed
      ciphertext, leaves ciphertext with no recoverable key — and the design
      deliberately refuses to regenerate a key when encrypted rows exist, so the
      credentials are permanently lost. One `Sync()` on the state root closes it.
      (b) **Cryptographic hygiene** (`vault.go:181-182`): the persisted key ID
      is SHA-256 of the live AES key truncated to 16 bytes, which publishes a
      hash of the key and gives an offline oracle for verifying guesses at key
      material. Not exploitable against a 256-bit random seed, but avoidable:
      expand HKDF to 48 bytes and take bytes 32..48 as the ID so it is derived
      alongside the key rather than from it. Requires a key-version increment;
      existing ciphertext must keep verifying under version 1.
      Plaintext and key bytes are not zeroed after use. That is an accepted
      residual risk, not a task — Go's copying GC makes wiping unreliable and
      `Open` returns an immutable `string`.
- [ ] Address two low-severity findings from the 2026-07-22 price update
      review, ideally folded into the next change that already touches
      `internal/usage/price_update.go`: (a) `price_update.go:68` treats every
      catalog parse failure as retryable, so a genuinely malformed
      non-transient catalog burns three attempts before failing — distinguish
      truncation from validation failure; (b) `price_update.go:143-148` checks
      the byte-size cap before the HTTP status, so an oversized 5xx body is
      reported as non-retryable "response exceeds N bytes" instead of a
      retryable transient failure.

## Document Conventions

One shape for every document. The **directory decides what a document is**, the
**filename is just its topic**, and **frontmatter carries its attributes** —
none of that is encoded in the filename.

```yaml
---
status: active | historical
version: N            # specs only, raised with each contract change
created: YYYY-MM-DD
retired: YYYY-MM-DD   # archived documents only
---
```

Filenames are lowercase and hyphenated, with no date and no type suffix:
`docs/specs/cli-design.md`, `docs/plans/test-coverage.md`.

| Directory | Holds | Lifecycle |
| --- | --- | --- |
| `docs/specs/` | Contracts — what the system does and must keep doing | Revised in place; stays active as long as the system stands |
| `docs/plans/` | Execution — how and when finite work gets done | Retires to `docs/archive/` once delivered and reviewed |
| `docs/reviews/` | Per-task review records, mirroring `plans/` by topic | Archived alongside the plan it belongs to |
| `docs/archive/` | Retired documents, mirroring `plans/`, `specs/`, and `reviews/` | Historical; never a starting point |
| `docs/README.md` | This file: the document map and the execution state | Updated in place |

**Specs additionally carry a version and changelog.** Raise the version and add
a changelog row whenever promised behavior changes. A spec does not retire when
a feature ships — that is when it becomes most authoritative. Never record
execution state in a spec; "implemented", "awaiting review", and "in flight"
belong in this file.

**Plans are scoped work, not a permanent home.**

- One plan owns one coherent piece of work with its own goal, evidence,
  checklist, and verification level.
- Start a new plan when work has its own goal and acceptance criteria, even if
  it touches an already-delivered feature. Only append a follow-up section to an
  existing plan when that plan's work is still in flight and the follow-up is
  genuinely part of finishing it.
- Retire a plan once every task's final gate is ticked (all done): move it under
  `docs/archive/plans/`, move its `docs/reviews/<plan-topic>/` directory into
  `docs/archive/reviews/`, set `status: historical` and `retired:`, record why in
  `docs/archive/README.md`, and collapse it into one line of "Current State"
  above.
- Watch the size. A plan past a few hundred lines, or one whose sections no
  longer share a goal, should be split or retired rather than extended. An
  earlier convention said to keep appending dated follow-up sections to the
  existing plan; that grew one file to roughly 950 lines spanning phase-one
  bootstrapping through release automation, and it was retired on 2026-07-22
  for exactly that reason.
- End a plan with a **Starting a task** recipe: a single hit-method that turns
  any Status-matrix anchor into a scoped `进入开发` instruction, so a task can be
  handed to a developer (human or agent) without hand-writing a fresh prompt.
  The recipe names what to read (`AGENTS.md`, the task's own section and its
  named files, the verification routing) and what to do on completion (tick
  `Dev`, record evidence, leave the review trail). One recipe per plan, keyed by
  anchor, rather than a duplicated instruction under every task — per-task
  specifics already live in each task's section. A plan whose tasks are already
  governed by an equivalent execution contract satisfies this without a second
  section.

**Each plan tracks its own tasks in a status matrix; this file records only a
coarse rollup.** A plan's task content lives in its prose (a `## Tasks` list or
per-task sections); the matrix carries only status — one row per task, one
column per gate (`Dev` and `Review`, plus `Test` or `Acceptance` later as a
plan needs them), and a tick when that gate passes for that task. The
implementer ticks `Dev` when the task is built and its own targeted
verification passes; an independent reviewer ticks `Review` when findings are
closed, and reopens the task rather than ticking it when review finds problems.
Each `Review` tick is backed by a `Verdict: PASS` round recorded in
`docs/reviews/<plan-topic>/<task-anchor>.md`; see `docs/reviews/README.md`.
A task is *done* when its last required gate is ticked. This file records only
`active — X/N done` per plan, where N is the task count and X counts done
tasks. Never copy the per-task grid here — that duplicate is how a status
document and its checklists drift apart.

**Archiving means moving, not deleting.** Preserve content, set the frontmatter,
and record in `docs/archive/README.md` why the document was retired and where
its conclusions now live. Do not re-list archived files in this index.

## Status Vocabulary

Only two values, matching the frontmatter above:

- `active`: a current contract, or unfinished work.
- `historical`: superseded or completed material kept only for context. It
  lives under `docs/archive/`, never in `docs/specs/` or `docs/plans/`.
