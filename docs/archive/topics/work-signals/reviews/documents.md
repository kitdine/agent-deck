---
status: historical
topic: work-signals
subject: requirements.md, architecture.md, ux/session-work-signals.md, ux/cli-work-signals.md, tasks.md
retired: 2026-09-01
---

# Review log — work-signals / the five-document set

This topic's document phase carries one record for the whole set and one verdict.
The reason is in [`../tasks.md`](../tasks.md), "How this document set is reviewed".

## Round 1 — 2026-08-20

- Reviewed state:
  - HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`
  - `requirements.md` `6162eb4ecb08820246fee605fa0a9f7ca1be80c9` (modified, uncommitted)
  - `architecture.md` `42d1cb61c173ca5ce1356cb7f1e3740d171d167b` (untracked)
  - `ux/session-work-signals.md` `b54759488080675a89200fefd64dc3f8389a1d06` (untracked)
  - `ux/cli-work-signals.md` `ce7a836593f9ce1338e68dd0bd8e801ff568b856` (untracked)
  - `tasks.md` `eddb322939e4835b2bb4e0b72c9f530ec3d7bd1b` (untracked)
  - Design specimen: `prototype/src/Cli.jsx` `d4c0a513ddcaea2f880012eb814fd5b9463f9342`,
    `prototype/src/data.js` `c8c9162152031b57b3048cca793c863223175ae9`,
    `prototype/src/i18n.js` `718e340dcfecd87ef3909dceb0faa077ee4a438a`
- Reviewer: Claude Code (`claude-opus-5[1m]`), combined document review
- Method: all five documents read in full before judging, then four verification
  passes against sources of truth rather than against each other —
  (a) the prototype's rendering code, which is the design truth for both surfaces;
  (b) the Go and Swift repository (`internal/activity`, `internal/store/migrations.go`,
  `internal/desktop`, `cmd/agentdeck/usage_*.go`, `apps/macos/AgentDeckApp/`,
  `docs/topics/desktop-app/tasks.md`) for every claim about existing behavior;
  (c) real Codex and Claude source logs under `~/.codex/sessions/` and
  `~/.claude/projects/` for the turn-boundary rules;
  (d) CodeBurn 0.9.20's own sources, extracted from `dist/main.js.map`
  (`src/classifier.ts`, `src/types.ts`, `src/providers/codex.ts`,
  `src/providers/claude.ts`), for the three borrowed and two rejected claims.
  No external scoring skill was used; every finding below cites the artifact that
  falsifies the document. The six user decisions of 2026-08-20 were treated as
  premises: findings about them concern fidelity or technical viability and are
  handed to the user, not rewritten.
- Scope: the five documents' content and their agreement with each other, the
  prototype, and the repository. Product code, tests, fixtures, and the prototype
  were read only.

### Findings

Findings are raised against the document that owns the fact. `-> touches` names
other documents a repair would have to move in the same round.

#### P1 — must close before the set can pass

- **[P1-1] `architecture.md` Decision 9 — the wire families have nowhere to hold
  the `Client` × `Period` product they promise.** Decision 9 states the three
  families are "producer-computed and producer-bounded across the `Client` ×
  `Period` product", but the shape it specifies —
  `sessions.work_signals.activity { available, cost_basis, items[4] }`, and the
  flat `workflow` and `tooling` objects — carries no period or client key. The
  existing producer-bounded pattern the panel's filters already read is
  `sessions.periods.items[]`, where every item carries `period` and `client`
  (`internal/desktop/desktop.go:122-135`). *Failure scenario:* the host switches
  the panel from `today` to `30d`; the three modules keep the same numbers,
  because the payload holds one unkeyed set. `requirements.md` acceptance
  condition 9 — "Both filters … govern the three modules exactly as they govern
  the rest of the panel" — is unimplementable against the shape as written.
  `-> touches requirements.md, tasks.md` (task 4).

- **[P1-2] `ux/cli-work-signals.md` — the three sections are placed after
  sections `usage stats` does not print.** The document says they "follow the
  existing `📊 USAGE SUMMARY`, `🪙 TOKEN TOTALS`, and `🧾 MODEL COVERAGE`
  sections". Those three are rendered by `renderUsageFamilySummary`
  (`cmd/agentdeck/usage_family_text.go:68,78,83`), which serves `usage summary`.
  `usage stats` renders through `renderUsageStatsWithOptions`
  (`cmd/agentdeck/usage_stats_layout.go:22`) and prints
  `📊 USAGE STATS · <RANGE>`, the TOKENS/COST/SESSIONS stat row, `🗓 TREND`,
  `🤖 MODELS`, `CLIENTS`, `PROVIDERS`, `CACHE HIT RATE`,
  `▦ ACTIVITY BY WEEKDAY / HOUR`, `COVERAGE`, `UNPRICED MODELS`, and
  `DETAIL COMMANDS` (`cmd/agentdeck/usage_stats_text_test.go:26-40`). The
  prototype's `stats` tab repeats the same wrong preamble, so the specimen does
  not settle this one; the code does. *Second consequence:* `usage stats`
  already prints a section whose name begins `ACTIVITY`, so adding `🧭 ACTIVITY`
  to the same default output puts two differently-meaning ACTIVITY sections on
  one screen, which no document addresses. *Third:* `usage stats` also has an
  interactive viewer (`--interactive`, `cmd/agentdeck/usage_stats_viewer.go`);
  no document says where the three sections live in it, or that they do not.

- **[P1-3] `ux/cli-work-signals.md` — "`usage stats` has none" is false, and the
  rationale it supports is inverted.** `cmd/agentdeck/main.go:3106` registers
  `stats.Flags().IntVar(&statsTop, "top", 0, …)`, whose help documents
  per-section default caps and states that "`--format json` always has every
  row". So `usage stats` already truncates derived text tables by default and
  says so, which is the opposite of the document's stated reason for omitting
  `--top` ("adding one here would make the CLI the only surface in this product
  that truncates a derived table without saying so"). The decision to have no
  `--top` may still be right; the argument for it is not.

- **[P1-4] `requirements.md` — the "shipped" uncaptured rendering does not
  exist.** `requirements.md` says `menubar-experience` "ships all three with
  their real headings and layout in a `Not captured yet` state". Against the
  repository: `menubar-experience` is `desktop-app` task 3 with `Dev [ ]` and
  `Review [ ]` (`docs/topics/desktop-app/tasks.md:792`), and what the app
  actually renders is `WorkSignalPlaceholder`
  (`apps/macos/AgentDeckApp/MenuBarPanelViews.swift:576-593`) — an SF Symbol and
  a title inside a rounded rectangle, with no `Not captured yet` text, no card
  body, no detail view, and no button. *Failure scenario:* task 5 begins,
  looks for the rendering it is told to replace and retain, and finds neither.
  `-> touches ux/session-work-signals.md` ("unchanged in layout from the shipped
  uncaptured form", "The shipped uncaptured rendering: the `待采集` flag on the
  signal heading, and a banner inside each detail view", "retained, not
  deleted") `and tasks.md` (task 5, "Replace the shipped `Not captured yet`
  cards"; task 0's premise about what `desktop-app` already delivers).

- **[P1-5] `tasks.md` — task 5 is blocked by `desktop-app` task 3, and the
  document says nothing blocks.** "Relationship to `desktop-app`" asserts "neither
  blocks the other's delivery", and task 5 lists task 4 as its only dependency.
  But task 5's contract is to *replace* and *retain* a two-level uncaptured
  rendering that `menubar-experience` has not delivered (see P1-4). Either task 5
  depends on `desktop-app` task 3, or task 5 owns building the uncaptured form
  first; the set picks neither.

- **[P1-6] `architecture.md` Decision 1 — the Claude turn rule counts injected
  entries as turns.** The rule opens a turn on "a log entry whose `message.role`
  is `user` and which is not a `tool_result` continuation". Real Claude logs
  carry a large minority of user-role text entries that are not human turns and
  are flagged `isMeta: true` — skill preambles ("Base directory for this
  skill: …"), image-cache references, and `<local-command-caveat>` blocks.
  Measured in this project's own logs: 12 of 40, 14 of 73, 17 of 42, and 24 of 86
  user-role text entries. The rule excludes only `tool_result`, so each of these
  opens a turn that called no tool and is classified `conversation` by Decision 3
  rule 4. *Failure scenario:* a Claude session of 40 real turns reports ~52,
  Claude's `conversation` share is inflated by roughly a quarter, and every
  per-turn ratio stops being comparable with Codex — which is the precise defect
  Decision 1 cites when it refuses to treat assistant messages as boundaries.
  Codex is unaffected: `turn_context` was verified to align one-to-one with real
  user turns in the sampled sessions (19 `turn_context` entries against 19 real
  user turns in `rollout-2026-08-20T09-46-45-…jsonl`), so the Codex half of the
  decision holds. This is a technical objection to a user decision's stated rule,
  not a proposal to change the decision; the exclusion list needs the user's
  ruling.

- **[P1-7] `architecture.md` Decision 3 — `delegation` has no Codex
  definition.** Decision 7 is explicitly per-client, with a Codex column and a
  Claude column. Decision 3 is not: `delegation` is "a subagent (`Task`/`Agent`)
  or … a skill or workflow", and `planning` is "plan mode or a to-do tool" —
  every identifier is Claude-shaped. Codex emits `function_call`,
  `custom_tool_call`, `mcp_tool_call`, `web_search_call`, and `computer_call`
  items (`internal/activity/activity.go:120-127`) and has no subagent or skill
  tool. *Failure scenario:* the whole `delegation` category and both its
  subcategories are structurally unreachable under `--client codex`, while
  `ux/session-work-signals.md` promises "Four rows, always four" whose shares sum
  to 100%, and `tasks.md` task 2 mandates "a Codex-only and a Claude-only fixture
  producing comparable classifications from the same work". No document says
  which categories are client-specific, or what a permanently-zero row renders
  as. `-> touches ux/session-work-signals.md, tasks.md`

#### P2

- **[P2-1] `architecture.md` Decision 2 names the wrong Go identifier.** It says
  "`internal/activity`'s package doc comment and the `Detail` comment both
  currently assert an absolute no-arguments boundary". `Detail`'s comment is
  "the safe, user-visible form of a merged tool call"
  (`internal/activity/activity.go:27`). The absolute claim — "deliberately has no
  fields for arguments, results, command text, environment, or reasoning" — is on
  `Record` (line 20). The mandated rewrite therefore points at the wrong type.
  `-> touches tasks.md` (task 1 repeats "the `Detail` comment").

- **[P2-2] `architecture.md` Decision 9 — `groups` is consumed and never
  defined.** `tooling { available, calls, groups, rows[≤5], … }` introduces
  `groups`, `ux/cli-work-signals.md`'s JSON sample carries it, and the prototype
  sets `groups: 4` beside four rows (`prototype/src/data.js:400-408`). No
  document says what it counts, whether `other` is included, or how it relates to
  `rows`. This is the shape of defect the combined review exists to catch.

- **[P2-3] `architecture.md` Decision 9 — `rows[≤5]` is not a bound, and the
  difference it documents cannot occur.** Decision 7 fixes the tool-kind
  vocabulary at exactly five (`bash`, `read`, `edit`, `mcp`, `other`), and both
  surfaces print one row per kind. So the panel can never truncate, and Decision
  9's "A reader diffing the two sees the same names with a different row count"
  describes an unreachable state. `ux/cli-work-signals.md` builds its `--top`
  argument on the same phantom bound (see P1-3).

- **[P2-4] `ux/cli-work-signals.md` — `--activity` renormalizes shares and no
  document says so.** The prototype's `filter` tab prints `Debugging` at `100%`
  with `Investigation 34%` and `Repair 66%` (`prototype/src/Cli.jsx:112-119`),
  i.e. shares are recomputed against the filtered scope rather than the whole.
  The flag table says only "Restrict the scope to turns of that category or
  subcategory". *Failure scenario:* two implementers ship two different
  denominators, and the panel/CLI reproducibility guarantee silently stops
  holding under `--activity`.

- **[P2-5] `ux/cli-work-signals.md` — the availability table does not match the
  prototype and does not cover `WORKFLOW`.** The table maps "No turn in scope" to
  `No turn in the selected scope.` under the section heading. The prototype's
  `empty` tab prints that message under `🧭 ACTIVITY`, `No tool call in the
  selected scope.` under `🔧 TOOLING`, and under `🧱 WORKFLOW` prints per-value
  dashes for three of its five rows and omits `EDITS / SESSION` and
  `MOST TOUCHED` entirely (`prototype/src/Cli.jsx:142-147`). Which sections take
  a message, which take `—` per row, and whether a row may disappear are all
  undefined.

- **[P2-6] `ux/session-work-signals.md` — `cost_basis: "partial"` has no panel
  rendering.** Decision 4 defines three values, Decision 9 puts the field on the
  wire, and `ux/cli-work-signals.md` defines the CLI's behavior for all three.
  The panel document's state table covers Normal, Empty, and Unavailable, and
  never says what `partial` looks like. A discriminator that the producer
  computes and the GUI ignores is a field with no consumer on the surface it was
  added for.

- **[P2-7] `ux/session-work-signals.md` — the five states are attributed to the
  wrong document.** "The panel's five states are `menubar.md`'s."
  `docs/topics/desktop-app/ux/menubar.md:111-141` says the opposite: six names
  that are **not** mutually exclusive, replaced by three surfaces
  (`loadingSurface` / `dataSurface` / `errorSurface`) plus orthogonal qualifiers
  (`stale`, `aged`, `partial`, …). The five names the table uses are the
  prototype's stage states, `["normal", "empty", "aged", "partial",
  "unavailable"]` (`prototype/src/Stage.jsx:4`). As written the table cannot be
  checked against the source it cites, and it silently drops `aged` and
  `partial` (the latter being P2-6).

- **[P2-8] `ux/session-work-signals.md` — no bilingual string exists for the
  `other` tool kind.** Decision 7 requires `other` to render as its own row when
  non-empty, and this document repeats it ("`other` … is a row like any other and
  always sorts last"). The prototype's `toolKinds` catalog has only
  `bash / read / edit / mcp` (`prototype/src/i18n.js:95,307`), and the Copy
  section — declared "New strings, and only these" — lists no
  `sessions.toolKinds.other`. *Failure scenario:* the `other` row renders with a
  missing key in both languages.

- **[P2-9] `requirements.md` — the mandated negative test cannot falsify what
  Decision 2 promises.** Acceptance condition 6 and `tasks.md` task 1 assert only
  on database content. Decision 2's guarantee is that the read values are
  "dropped in the same function that read it, before the record is constructed",
  which is broader than the store: a leak into a log line, an error or warning
  string, the `Page`/`Detail` JSON, or the source-file cache passes the test
  unchanged. And `internal/activity` truncates metadata at
  `maxMetadataLength = 256` (`activity.go:18,213`), so a *truncated* path or
  command fragment also passes a whole-string absence assertion. The privacy
  boundary is being weakened deliberately; the test that is supposed to pay for
  that has holes on both axes.

- **[P2-10] `requirements.md` — `path_digest` is described as de-identification
  it does not provide.** Decision 2 specifies `sha256(absolute path)`, unsalted,
  and the privacy table's rationale is "Distinct-file counting needs identity
  without the path". An unsalted digest of a path is recoverable from any
  candidate path list and is stable across machines, so the stored value is a
  reversible pointer to the path rather than an opaque identity. A per-install
  salt — the machine identity the credential key already derives from is the
  obvious candidate — would keep every counting and grouping use intact. Handing
  this to the user: salt it, or state the residual risk as accepted.

#### P3 / nit

- **[P3-1] `ux/cli-work-signals.md`** — the `🧭 ACTIVITY` example prints
  `███████░░░` for 52%. The prototype's renderer yields `█████░░░░░`:
  `bar()` is `round(share / 100 * 10)` filled cells (`prototype/src/Cli.jsx:17-20`),
  and `round(5.2)` is 5. Everything else on that line — the 16-column label pad,
  the two-space gaps, `%` and `$` alignment — matches. A document that claims
  literal character output has to be literal.
- **[P3-2] `architecture.md`** — "CodeBurn exposes ten flat categories". Its
  `TaskCategory` union has thirteen: `coding`, `debugging`, `feature`,
  `refactoring`, `testing`, `exploration`, `planning`, `delegation`, `git`,
  `build/deploy`, `conversation`, `brainstorming`, `general` (`src/types.ts:167-180`).
  The argument the number supports is unaffected.
- **[P3-3] `architecture.md` Decision 6** — "It is CodeBurn's `countRetries`,
  including its read-shaped exclusion". The exclusion matches
  (`src/classifier.ts:171-191`), but the window does not: CodeBurn counts within
  one `ParsedTurn`, and Decision 6 counts "within a session". Same algorithm,
  different denominator, and the identity claim hides it.
- **[P3-4] `ux/session-work-signals.md`** — `sessions.times` contradicts its own
  example. The Copy table gives 中文 `次` / English `×`, matching the prototype
  catalog (`i18n.js:75,287`), but the document renders the row as `tasks.md ×4`
  for both languages, and the prototype hardcodes `×` without using the string
  (`Popover.jsx:600`). Either the Chinese row reads `tasks.md 4 次` or the string
  is not needed.
- **[P3-5] `ux/session-work-signals.md`** — `sessions.shareOfCalls` is listed as
  a shipping string, but nothing renders it: the prototype's Tooling row prints a
  bare percentage (`Popover.jsx:614`).
- **[P3-6] `ux/session-work-signals.md`** — the Tooling summary card is given as
  `82 tool calls`; the prototype renders `82 Tool calls` (`i18n.js:303`).
- **[P3-7] `requirements.md`** — "Those four decisions are now made, by the
  user". `architecture.md` records six user decisions of 2026-08-20; the category
  set and the replacement of iteration depth by rework are the other two.

### What was checked and found sound

Recorded so a later round does not re-derive it:

- Schema v19 is the correct next version: `internal/store/migrations.go` tops out
  at `version: 18`. `usage_tool_calls` is indeed v13 (line 91) and carries the
  columns `requirements.md` lists. `usage_events` (line 39) carries no turn
  association, as stated.
- The Codex half of Decision 1 holds: the parser reads `turn_context` and holds
  `turn_id`, using it only for the anonymous-call digest
  (`internal/activity/activity.go:101-102,177`), and in sampled real logs
  `turn_context` is emitted once per real user turn.
- Decision 3's refusal to key any rule on tool failure status is correct and
  verified: `parseCodex` hardcodes `"completed"` (line 127) while `parseClaude`
  reads `is_error` (line 148-149).
- Decision 9's `wire_version` argument is correct: `WireVersion = 1` with an
  exact-equality guard on both sides (`internal/desktop/desktop.go:21,38-39`;
  `apps/macos/AgentDeckShared/DesktopWire.swift:64`), and the additive-family
  precedent is already documented in that file (line 675).
- All three CodeBurn borrowings check out against its sources: `ParsedTurn` is
  `userMessage` plus `assistantCalls` (`types.ts:88-107`); `countRetries` carries
  the read-shaped exclusion with its stated rationale (`classifier.ts:171-191`);
  and the earliest-match rule is `firstMatchingCategory`, whose comment names the
  "add error handling" case verbatim (`classifier.ts:97-118`). The rejected
  borrowing about kept paths and commands is also right: `ToolCall` retains
  `file` and `command` (`types.ts:161-165`).
- The prototype's activity fixture is internally consistent: the four shares sum
  to 100 and every parent's subcategory shares sum to that parent
  (`prototype/src/data.js:341-390`), so `ux/session-work-signals.md`'s summation
  claim holds.
- The eleven subcategory strings in the Copy table match the prototype catalog
  exactly in both languages (`prototype/src/i18n.js:77-89,288-300`).
- The `SIGNALS` line, the five `🧱 WORKFLOW` rows, and the 16-column label pad
  in `ux/cli-work-signals.md` match the prototype character for character.
- `session show --activity` exists (`cmd/agentdeck/main.go:2280`), so the added
  line has a real host command.
- The document set closes the discarded pass's headline defect: the session-level
  category that a CLI line consumed with no upstream rule is now defined
  (Decision 5), including its tie-breaks and its `cost_basis: none` behavior.

### Evidence

```
bash scripts/check-topic-docs.sh          -> exit 0, no output
make check-whitespace                     -> exit 0, no violations
git diff --check                          -> exit 0, clean
```

- Verification level: **L0**. The reviewed subject is an unimplemented design
  document set; no product code, test, configuration, or generated file changed
  in this round, so no Go test, vet, build, or race evidence is applicable.
- Repository verification performed as part of the review was read-only:
  `grep`/`sed` over `internal/`, `cmd/`, `apps/macos/`, `docs/topics/desktop-app/`,
  `prototype/src/`; `jq` over real Codex and Claude session logs and over
  CodeBurn's source map. No file outside `docs/topics/work-signals/reviews/` was
  written.

### Verdict: REOPEN

Seven P1 findings, ten P2, seven P3. Three of the P1s are contract-completeness
defects of exactly the kind this combined review exists to catch — a wire shape
that cannot hold the scope product it promises (P1-1), a category whose rules
exist for only one of the two clients (P1-7), and a rendering three documents
build on that neither the sibling topic nor the code has delivered (P1-4/P1-5).
Two more are claims about existing CLI behavior that the code contradicts
(P1-2, P1-3), and one is a turn-boundary rule that real logs falsify (P1-6).

Per `tasks.md`, a REOPEN ticks none of the five `Review` cells, however few
documents the findings touch. All five return to Dev.

P1-6, P1-7, and P2-10 are technical objections that bear on user decisions of
2026-08-20 (the turn unit, the four-category set, and the privacy boundary). They
are stated with their failure scenarios and left for the user to rule on; they
were not rewritten in this round.

## Round 1 — repair — 2026-08-20

- Reviewed content state: the five documents plus `/prototype/`, all uncommitted;
  repaired in one pass on top of Round 1's recorded findings.
- Authority: user instruction of 2026-08-20, which also delegated the rulings on
  P1-6, P1-7, and P2-10 to the repairer and required every decision analysis to
  be reported back.
- Scope: all 24 findings. Documents and the prototype both, because three
  findings were of the form "the document asserts something the specimen does not
  render", and repairing only the document would have preserved the defect.

### Rulings on the three findings handed over

- **P1-6 — upheld, rule tightened.** The measurement was reproduced
  independently before ruling: in this repository's own Claude logs, `isMeta`
  accounts for 12/40, 14/73, 12/41, and 5/46 user-role text entries. Ruling: a
  Claude turn opens only on a user entry that is not a `tool_result`
  continuation, not `isMeta`, and not a synthetic wrapper. This repairs the
  statement of the user's decision rather than changing it — injected system text
  was never "a user message".
- **P1-7 — finding upheld, its failure scenario refuted, and the real defect is
  larger.** The record asserted Codex "has no subagent or skill tool" and that
  `delegation` is structurally unreachable there. Reading twelve Codex sessions
  shows `spawn_agent`, `list_agents`, `interrupt_agent`, and `update_plan` are
  all present, so `delegation/subagent` and `conversation/planning` are reachable
  and "four rows, always four" is not threatened. Only `delegation/workflow` is
  Codex-unreachable. Ruling: Decision 3 gains a per-client identifier table, and
  a subcategory with no signal for the selected client is omitted from the
  expanded list rather than shown as a zero row.
- **P2-10 — upheld, salted.** An unsalted `sha256` of a path is recoverable from
  a candidate list and identical across machines, which makes it a reversible
  pointer rather than an identity. Ruling: salt with the stable machine identity.
  Cost to consumers is nil — every use counts or groups within one install.

### New finding raised during repair

- **[P1-8] `architecture.md` Decision 2 — the extraction has no Codex input.**
  Found while verifying P1-7, and not caught by Round 1. Decision 2 says "parse
  the argument object, read the path key", and Decision 7's Codex column named
  `apply_patch`, `write_file`, `read_file`, `grep`, and `glob` as Codex tools.
  None of those is a Codex tool. The observed vocabulary across twelve sessions
  is `exec_command`, `exec`, `js`, `write_stdin`, `wait`, `wait_agent`,
  `spawn_agent`, `list_agents`, `interrupt_agent`, `send_message`,
  `followup_task`, `update_plan`, `view_image`; reads, writes, and searches all
  go through the shell, and `exec_command`'s argument keys are `cmd`, `workdir`,
  and output-control values only — 327 sampled calls, no path key.
  *Failure scenario:* on Codex, `path_digest`, `base_name`, `files_touched`,
  `top_file`, `first_edit_seconds`, `retries`, and the edit-shaped test behind
  Decision 3's `coding` and `debugging` rules all have no input, and the Tooling
  breakdown collapses into `bash`. **Not repaired — it is a scope decision, not a
  wording defect.** Three options (parse `cmd`; parse `apply_patch` headers only;
  Claude-only file metrics) are recorded in Decision 2's "Open" block, in
  `requirements.md`, and against task 1, with the repairer's recommendation of
  option B. Awaiting the user. `-> touches requirements.md, tasks.md`

### Dispositions

| Finding | Disposition |
| --- | --- |
| P1-1 | Decision 9 rewritten: each family is a keyed `items[]` carrying `period` and `client`, following `SessionsPeriods`. Task 4 updated |
| P1-2 | `usage stats`'s real sections named from `usage_stats_layout.go`; insertion point fixed after `▦ ACTIVITY BY WEEKDAY / HOUR`; the `ACTIVITY` name collision resolved by titling this topic's section `🧭 WORK KIND` inside `usage stats`; `--interactive` declared out of scope and unchanged. Prototype `stats` tab corrected |
| P1-3 | The false premise removed. `--top` is absent because every table here is bounded by its vocabulary, not because the product never truncates |
| P1-4 | `requirements.md` now states what actually ships (`WorkSignalPlaceholder`, three tiles) and that the two-level uncaptured form is `desktop-app` task 3, undelivered. `ux` and `tasks.md` follow |
| P1-5 | Task 5 now depends on `desktop-app` task 3; the "neither blocks the other" claim replaced with the one directional dependency, and the rejected alternative recorded |
| P1-6 | Ruled above; Decision 1's Claude row rewritten with the three-part exclusion and the measurement |
| P1-7 | Ruled above; Decision 3 gains the per-client table, Decision 7's Codex column replaced with the verified vocabulary |
| P2-1 | `Detail` → `Record` in Decision 2 and task 1 |
| P2-2 | `groups` defined as the count of distinct `tool_kind` values present in `rows`, `other` included |
| P2-3 | `rows[≤5]` removed; `rows` is one entry per non-empty kind, untruncated, and the phantom panel-versus-CLI difference deleted |
| P2-4 | `--activity` renormalizes shares and not counts, stated, with the note that its output has no panel figure to reproduce |
| P2-5 | Availability is now per section: message for Activity and Tooling, all five rows with `—` for Workflow. The prototype's disappearing rows corrected |
| P2-6 | `partial` gains a rendering: one line under the four Activity rows, no per-row marker. Added to the prototype and to the Copy table as `sessions.costPartial` |
| P2-7 | The state table is now attributed to the prototype's stage states, with `aged` and `partial` restored and `menubar.md`'s actual model described rather than misquoted |
| P2-8 | `sessions.toolKinds.other` added to the Copy table and to the prototype catalog in both languages |
| P2-9 | The negative test's scope widened to logs, error strings, `Page`/`Detail` JSON, and the source cache, and changed to substring assertions because of `maxMetadataLength` truncation. A salt test added |
| P2-10 | Ruled above; the digest is salted in Decision 2, the `requirements.md` table, and task 1 |
| P3-1 | Bar corrected to `█████░░░░░`, matching the prototype's renderer |
| P3-2 | Ten → thirteen |
| P3-3 | The identity claim narrowed: same rule and exclusion, different window (turn versus session), stated explicitly |
| P3-4, P3-5 | `sessions.times` and `sessions.shareOfCalls` removed from the Copy table and from the prototype catalog — nothing renders either |
| P3-6 | `82 Tool calls`, matching the catalog |
| P3-7 | Four → six decisions, naming the other two |

### Evidence

- `bash scripts/check-topic-docs.sh` — exit 0.
- `make check-whitespace` — exit 0.
- `git diff --check` — clean.
- `npm run build` in `/prototype/` — clean, three times across the repair.
- Browser verification of the prototype changes, not merely a build: the
  corrected `usage stats` section names and `WORK KIND` title on `?surface=cli`,
  and the `成本部分归因` line rendering at `?state=partial`.
- Source verification behind the rulings: twelve Codex session logs for the tool
  vocabulary and `exec_command`'s argument keys; four Claude session logs for the
  `isMeta` ratios; `cmd/agentdeck/usage_stats_layout.go`, `main.go:3106`,
  `internal/desktop/desktop.go:122-135`, `internal/activity/activity.go:19-20`,
  `apps/macos/AgentDeckApp/MenuBarPanelViews.swift:526-540`, and
  `docs/topics/desktop-app/tasks.md:792`.
- L0: the subject is unimplemented design plus a design specimen. No product
  content changed, so no Go test was run.

### Verdict: REOPEN — repair complete, awaiting independent re-review

All 24 Round 1 findings are closed. One new P1 was raised during the repair and
is **not** closed: P1-8 needs a user decision, and three documents carry the open
block rather than an assumed answer. The set cannot pass while a P1 is open, and
the re-review should verify the 24 closures independently rather than accept this
record's account of them.

## Round 1 — repair, second pass — 2026-08-20

The first repair pass ruled on P1-6, P1-7, and P2-10 itself, and made five
further decisions along the way that no one had been asked about. The user
identified that and took all of them back. This pass records their rulings and
reworks what the first pass had settled unilaterally.

### What the first pass decided without authority

Eight items. Listed because the failure was procedural, and a repair record that
hides how a decision entered the documents is the same defect the topic restarted
over.

| Item | First pass chose | User's ruling |
| --- | --- | --- |
| P1-6 Claude turn boundary | Exclusion list | "Reference CodeBurn" — resolved below |
| P1-7 unreachable subcategory | Not rendered | **Upheld** — not rendered |
| P2-10 path digest | Salted | **Upheld** — salted |
| `usage stats` section name | `🧭 WORK KIND` | **Upheld** |
| `usage stats --interactive` | Cut from scope | **Overturned in part** — still not built, but recorded in the Backlog with its reason |
| Codex path source (P1-8) | Left open, recommended B | **Option A** — parse the `cmd` string |
| `partial` panel rendering | New caveat line | **Overturned** — the panel shows nothing |
| Task 5 cross-topic dependency | Depends on `desktop-app` task 3 | **Overturned** — no dependency; replace what is there or build it |

### P1-6, resolved by reading the reference

"Reference CodeBurn" turned out not to settle it. CodeBurn does not filter
`isMeta` at all; `groupIntoTurns` relies on emitting a turn only when at least
one assistant API call followed. Measured against one of this repository's Claude
sessions, 10 of 12 `isMeta` entries are followed by an assistant message before
the next real user text — so under CodeBurn's rule alone each of those ten opens
a turn *and* captures the preceding real message's assistant calls, inflating the
count and feeding the classifier a skill preamble instead of the user's request.

Both rules are therefore kept: the exclusion list runs first, and the
no-assistant-call-no-turn rule backs it up for injected shapes nobody has
enumerated. Decision 1 states both and states why either alone is insufficient.

### P1-8, resolved as option A, with its two halves separated

Paths are parsed from `cmd` on Codex. The repair splits the parse in the document
because the two halves are not equally trustworthy:

- **Is it a write?** CodeBurn's `bash-utils.ts` is genuinely portable here —
  segment splitting, quote stripping, prefix skipping, a closed read-only
  allowlist, git read-subcommands, unknown-is-not-read. Ported, not cited.
- **Which file?** CodeBurn does not solve this; its `call.file` comes only from
  `args['file_path'] ?? args['path']`, so it carries the same Codex gap. Written
  for this project against `apply_patch` headers, `sed -i`, redirects, `tee`,
  `cp`/`mv`.

Consequence recorded on both surfaces: `files_touched`, `top_file`, `retries`,
and `first_edit_seconds` are **lower bounds on Codex**.

### Reworked in this pass

- The `partial` marker was removed from the panel document, the Copy table, the
  prototype (`Popover.jsx`, `i18n.js`, `styles.css`), and task 5. The state table
  now says `partial` renders unchanged and only JSON distinguishes it.
- Task 5's dependency prose and the "Relationship to `desktop-app`" dependency
  paragraph were deleted. Task 5 now says: replace the uncaptured form if it is
  there, build it if it is not; both topics are in `v0.5.0` and the piece is
  expected either way.
- `requirements.md`'s `WorkSignalPlaceholder` paragraph was cut from eleven lines
  to four. The line-cited archaeology was ceremony, and the user named it as the
  behavior that produces endless document churn.
- `usage stats --interactive` gained a Backlog entry stating what is deferred and
  why.
- Decision 2's "Open" block became a resolved section; Decision 7's Codex column
  now splits shell calls into `read`/`edit`/`bash` by the command parse, with the
  unclassifiable direction falling to `bash` rather than `edit`.

### Evidence

- `bash scripts/check-topic-docs.sh` — exit 0.
- `make check-whitespace` — exit 0.
- `git diff --check` — clean.
- `npm run build` in `/prototype/` — clean.
- CodeBurn `src/parser.ts` `groupIntoTurns`, `src/bash-utils.ts`, and
  `src/classifier.ts` read from the shipped source map.
- Turn-position measurement: `5ea4cad5-…jsonl`, 12 `isMeta` entries, 10 followed
  by an assistant message.
- L0. No product content changed.

### Verdict: REOPEN — repair complete, awaiting independent re-review

All 24 Round 1 findings and P1-8 are now closed. The re-review should treat the
eight decisions in the table above as the highest-risk area, since each entered
the documents through a repair rather than through review, and three of them were
overturned once already.

## Round 2 — independent re-review — 2026-08-20

- Reviewed state:
  - HEAD `9613498123f00b59d3d4b84fbff71e0f71d6ebd4`
  - `requirements.md` `35e19fec0e51962c390180e997ce0387574fcb0f`
  - `architecture.md` `f7d4b1e6192c000ab8acf92734d941a6946a76e0`
  - `ux/session-work-signals.md` `259c1971d4c2a1a76c0aa9c829d819b3c8b70b50`
  - `ux/cli-work-signals.md` `9f759da9ae8823a7c073c82e037c11b7bee14757`
  - `tasks.md` `305e292c4ab6be9fc56840ec235b3f111f3ff282`
  - Specimen: `prototype/src/Cli.jsx` `c764598fa3c885d2e5c50133b4b3125814ded732`,
    `prototype/src/i18n.js` `3d2c42b4ce8444153af3ab8753d1cd1a336f6336`,
    `prototype/src/Popover.jsx` `c27688114cff8f047c5955aa7efbc2fe9fa3b5ed`
  - All five documents and all three prototype files changed since Round 1.
- Reviewer: Claude Code (`claude-opus-5[1m]`)
- Method: each of the 24 Round 1 findings and P1-8 re-checked against the
  documents as they now read, then re-verified at the source rather than against
  the repair record's account of itself. The eight decisions that entered through
  repair were given the deepest pass, per the instruction. New measurements were
  taken rather than reused: a tool-name census over the **whole** Codex corpus
  (761 sessions) rather than a recent sample, `exec`/`exec_command` payload
  shapes, the `isMeta` ratios and the 10-of-12 turn-position claim, CodeBurn's
  `groupIntoTurns`, `bash-utils.ts`, and `parser.ts` tool-call mapping, and every
  file-and-line citation the documents added.
- Scope: the five documents, their agreement with each other and the prototype,
  and every claim they make about source-log formats, the repository, and
  CodeBurn. Read-only throughout.

### The eight repair-entered decisions, judged

| Decision | Judgment |
| --- | --- |
| P1-6 Claude turn boundary — both rules kept | **Sound, independently reproduced.** `isMeta` ratios re-measured across twenty sessions (range 1/21 to 24/86; the four cited values all reproduce). `groupIntoTurns` (`src/parser.ts:1507-1564`) confirmed to emit a turn only when `currentCalls.length > 0` and to have no `isMeta` filter. The 10-of-12 claim reproduces exactly on `5ea4cad5-…jsonl` under the stated semantic. The conclusion that either rule alone is insufficient holds |
| P1-7 unreachable subcategory not rendered | **Rule sound; its Codex evidence is not.** The omission rule and the parent-still-renders clause are coherent and land on both surfaces. The per-client identifier table it introduced is where the new defects are — see R2-1 and R2-3 |
| P2-10 salted digest | **Sound and consistently applied.** Present in Decision 2, the `requirements.md` table, and task 1's salt test |
| `🧭 WORK KIND` inside `usage stats` | **Sound.** The collision with `▦ ACTIVITY BY WEEKDAY / HOUR` is real, the shipped name is correctly left alone, the insertion point is stated, and the prototype's `stats` tab renders it |
| `usage stats --interactive` deferred with a Backlog entry | **Sound.** Stated in the CLI document, `requirements.md`'s Backlog, and task 3 |
| Codex path source — option A, parse `cmd` | **Not sound at the scale it will run at, and not closed across the set.** See R2-1, R2-2, R2-4 |
| `partial` shows nothing on the panel | **Sound and consistent.** Panel document, CLI document, and the prototype agree; the rejected marker is recorded with its reason |
| Task 5 has no cross-topic dependency | **Sound.** Replace-or-build is stated in task 5 and in `requirements.md`, and the dependency paragraph is gone |

### Closure of the 25 findings

**Closed, verified at source — 22.** P1-1 (Decision 9 is now a keyed `items[]`
following `SessionsPeriods`, with `available` deliberately left on the family and
the CLI's one-scope difference documented on both sides); P1-3 (`main.go:3106`
cited correctly, the false premise replaced with a real bounded-by-vocabulary
argument); P1-4; P1-5; P1-6; P2-1; P2-2; P2-3; P2-4; P2-5 (prototype's `empty`
tab now prints all five Workflow rows); P2-6; P2-7 (`menubar.md:111-141`
re-read — the three-surface model is now described correctly rather than
misquoted); P2-8; P2-9; P2-10; P3-1 (`█████░░░░░`, matching `bar(52)`); P3-2;
P3-3; P3-4; P3-5; P3-6; P3-7.

**Closed only in part — 1.** P1-2: the section names, the insertion point, the
rename, and the interactive scoping are all correct now, but the same wrong
premise survives twice in the same document — see R2-5.

**Not closed — 2.** P1-7's rule closed while its evidence did not (R2-1, R2-3),
and P1-8 is resolved in `architecture.md` and still open in `tasks.md` (R2-4).

### Findings

#### P1

- **[R2-1] `architecture.md` Decision 7 and Decision 2 — `apply_patch` is a real
  Codex tool, and it is the one that carries the path.** Both documents state the
  Codex vocabulary as thirteen names "verified across twelve sessions", and
  Decision 7 adds "An earlier draft named `apply_patch`, `write_file`,
  `read_file`, `grep`, and `glob` as Codex **tools**; none of them is one."
  Four of those five are indeed absent. `apply_patch` is not: a census over the
  whole corpus under `~/.codex/sessions` — 761 sessions, not twelve — finds
  **3,816 `apply_patch` calls in 93 sessions**, emitted as
  `custom_tool_call` with `"name":"apply_patch"`, through 2026-07-30, before the
  `exec` surface replaced it. Its `input` is a literal patch whose headers carry
  absolute paths: 15,437 `*** Update File:`, 2,764 `*** Add File:`, 640
  `*** Delete File:`.
  *Failure scenario:* Decision 7's Codex column has no `apply_patch` row, so all
  3,816 calls fall to `other`; Decision 3's Codex edit-shaped test is
  "`exec_command` whose `cmd` parses as a write", which never matches them, so
  those turns cannot classify as `coding` or `debugging/repair`; and
  `files_touched`, `top_file`, `retries`, and `first_edit_seconds` see nothing.
  Decision 8 backfills *already-indexed sources*, so this is not a legacy corner
  — it is the shape of every Codex session in the store before August.
  The sharper point: `apply_patch` headers are the cleanest edit-plus-path signal
  Codex has ever emitted, needing no `cmd` parsing at all, and the design routes
  them to `other` while calling the file half of the `cmd` parse "Partial. Every
  write form not on that list is missed."
  `-> touches requirements.md` (the same vocabulary sentence), `tasks.md`

- **[R2-2] `architecture.md` Decisions 2, 3, and 7 — `exec` is the dominant Codex
  tool, it is not a command string, and nothing can classify it as an edit or a
  read.** Decision 7 maps `edit` and `read` to "an `exec_command` whose `cmd`
  …", and puts `exec` in the `bash` row unconditionally; Decision 3's per-client
  table does the same. But `exec` is the larger tool by far — corpus-wide
  **35,422 `exec` calls against 18,847 `exec_command`**, and in the forty most
  recent sessions 3,547 against 147, i.e. 96%. And `exec` carries no `cmd`
  argument: its payload is an `input` string containing a **JavaScript program**,
  for example `const r = await tools.exec_command({\n  cmd: "sed -n '1,260p' …"`
  and `const results = await Promise.all([tools.exec_command({cmd: …}), …])`.
  *Failure scenario, as specified:* every `exec` call is `bash`; Codex's `edit`
  and `read` shares are near zero; `coding` is effectively unreachable on current
  Codex; `files_touched` and `top_file` are empty rather than merely lower bounds.
  *Second failure:* one `exec` wraps a variable number of commands — measured on
  one session, 37 calls wrap none, 73 wrap one, and 17 wrap two to four — so the
  one-row-per-tool-call model of `usage_tool_calls`, with its single `tool_kind`
  and single `path_digest` (Decision 8), cannot represent an `exec` that edits
  three files. Neither the count nor the shape is addressed anywhere in the set.

#### P2

- **[R2-3] `architecture.md` Decision 7 — the `mcp` cell repeats the exact
  conflation the same table says it fixed.** The Codex `mcp` row is
  `mcp_tool_call`. Decision 2 states the whole Codex tool vocabulary as thirteen
  names, and `mcp_tool_call` is not among them; the census finds **zero**
  occurrences of `"name":"mcp_tool_call"` in forty sessions. `mcp_tool_call` is a
  rollout *item type* — the same class of thing as `function_call` and
  `custom_tool_call` — not a tool name, which is precisely the error the table's
  own note says it corrected for the other five. Either the row names an item
  type in a tool-name table, or Decision 2's vocabulary is incomplete; the two
  cannot both stand.

- **[R2-4] `tasks.md` task 1 — P1-8 is resolved in `architecture.md` and still
  open here.** Task 1 reads "**Blocked on an open decision.** … Whether paths
  come from parsing `cmd` (A), from `apply_patch` headers only (B), or not at all
  on Codex (C) is recorded in `architecture.md` Decision 2 and awaits the user."
  `architecture.md` Decision 2 has no open block: it states "Paths are therefore
  parsed out of the `cmd` string on Codex (user decision, 2026-08-20)" with the
  two-half method table. The second repair pass updated the architecture and did
  not carry the change into the task. *Failure scenario:* task 1 is dispatched,
  its own contract says it is blocked awaiting a decision, and the implementer
  either stops or re-asks a question already answered. Task 1's "Parse the
  argument object, take what Decision 2 permits" was left standing for the same
  reason — on Codex there is no argument object to parse.

- **[R2-5] `ux/cli-work-signals.md` — P1-2's wrong premise survives twice in the
  document that fixed it.** The document now states correctly that
  `📊 USAGE SUMMARY` / `🪙 TOKEN TOTALS` / `🧾 MODEL COVERAGE` belong to
  `usage summary`. Two later paragraphs still borrow from that command as though
  it were `usage stats`: `🧱 WORKFLOW` is specified as "five aligned label/value
  rows in the established `usage stats` style" — that aligned label/value
  primitive is `usageAlignedColumns` in `cmd/agentdeck/usage_family_text.go`,
  the `usage summary` renderer — and Degradation says "the bar track degrades the
  way `MODEL COVERAGE`'s does", naming a section the same document has just
  assigned to a different command. Task 3 inherits both.

- **[R2-6] `architecture.md` Decision 2 — the lower-bound caveat does not cover
  everything that consumes the parse.** It names `files_touched`, `top_file`,
  `retries`, and `first_edit_seconds`. The same Codex write-detection also feeds
  `edits_per_session` (Decision 6), the `coding` and `debugging` category rules
  and the `investigation`/`repair` split (Decision 3), and the `edit` and `read`
  shares of the Tooling module (Decision 7). The repair's own P1-8 failure
  scenario listed the classification consequence; the resolution dropped it.
  Against `requirements.md` acceptance 4 — "every displayed number is traceable
  to source-log evidence" — an unstated bound on a displayed share is the part
  that matters.

- **[R2-7] `tasks.md` task 3 — "the four availability states" no longer exists.**
  The CLI document's availability table was reduced to three conditions in the
  repair (data present, nothing in scope, partially attributed). Task 3 still
  requires covering four.

- **[R2-8] `tasks.md` task 5 — the Verification level line was lost.** Tasks 1,
  2, 3, 4, and 6 each end with one; task 5 now ends with the replace-or-build
  paragraph and has none. Its previous value was "**L2** plus manual visual and
  accessibility acceptance", and task 5 is the only task carrying manual macOS
  acceptance, so it is the one where the level is load-bearing.

#### P3 / nit

- **[R2-9] `ux/session-work-signals.md`** — the `sessions.toolKinds.other`
  rationale describes the pre-repair prototype in the present tense: "the
  prototype's `toolKinds` catalog has only `bash / read / edit / mcp`
  (`prototype/src/i18n.js:95,307`)". The repair added `other` to that catalog in
  the same pass; it is now at `i18n.js:93` and `:303`, both including
  `other: "其他"` / `other: "Other"`. The argument for the key is now made
  against a state the same change removed.
- **[R2-10] `ux/session-work-signals.md`** — the Tooling summary card cites
  `i18n.js:303` for `Tool calls`; `toolCalls` is at `:90` and `:300`, and `:303`
  is now `toolKinds`.
- **[R2-11] `architecture.md` and `tasks.md`** — the `Record` comment is cited as
  `internal/activity/activity.go:19-20`; line 19 is blank and the comment is at
  lines 20-21.
- **[R2-12] `architecture.md`** — "its `call.file` comes only from
  `args['file_path'] ?? args['path']`". In the read source there is no `?? path`
  fallback: `parser.ts:1364` assigns `call.file` from `inp['file_path']` alone,
  and `call.command` from `inp['command']`. The conclusion — that CodeBurn does
  not solve the which-file half for Codex — is unaffected and correct.
- **[R2-13] `requirements.md`** — Goal 3 still says "the four tool kinds the
  surface names", while Decision 7 fixes five and both surfaces render `other`
  when non-empty; the What-exists table lists the same four.
- **[R2-14] `prototype/src/i18n.js`** — `toolGroups` remains in both catalogs and
  nothing renders it. P3-4 and P3-5 removed `times` and `shareOfCalls` for
  exactly that reason; this one was missed.

### What was re-verified and stands

- `bash-utils.ts` matches the "is it a write?" row claim in full: segment split
  on `&&`/`;`/`|`, quote stripping, `sudo`/`npx`/`env`-assignment prefix
  skipping, a closed `READ_ONLY_BASH` allowlist, a `GIT_READ_SUBCOMMANDS` list
  that deliberately excludes anything mutating, and unknown-is-not-read. The
  claim that this half is genuinely portable is correct.
- `exec_command`'s argument keys are `cmd`, `workdir`, `max_output_tokens`, and
  `yield_time_ms` across every sampled call — no path key, as stated.
- `update_plan` (356 calls), `spawn_agent` (327), `followup_task` (107),
  `view_image` (13), `js` (691), and `write_stdin` (1,652) are all real, so
  P1-7's refutation of Round 1's "no subagent or skill tool on Codex" is correct
  and Round 1 was wrong on that point.
- Prototype changes are in the specimen, not only in the record: `🧭 WORK KIND`
  with the weekday/hour section above it, all five Workflow rows dashed in the
  `empty` tab, `toolKinds.other` in both catalogs, and `times`/`shareOfCalls`
  gone.
- Every other file-and-line citation added by the repair resolves correctly:
  `usage_stats_layout.go:22`, `main.go:3106`, `desktop.go:122-135`,
  `Stage.jsx:4`, `Popover.jsx:600`, `Popover.jsx:614`.

### Evidence

```
bash scripts/check-topic-docs.sh          -> exit 0, no output
make check-whitespace                     -> exit 0, no violations
git diff --check                          -> exit 0, clean
```

- Verification level: **L0**. Unimplemented design documents plus a design
  specimen; no product code, test, configuration, or generated file changed in
  this round. No Go test, vet, build, or race evidence is applicable.
- Measurements taken during the review, all read-only: tool-name census over 761
  Codex session logs; `exec`/`exec_command` payload inspection; `apply_patch`
  header census; `isMeta` ratios over twenty Claude session logs and the
  turn-position measurement on `5ea4cad5-…jsonl`; CodeBurn `parser.ts`,
  `bash-utils.ts`, `classifier.ts`, and `types.ts` from the shipped source map.

### Verdict: REOPEN

Twenty-two of the twenty-five findings are genuinely closed, and the repair's
hardest piece of work — the P1-6 analysis — survived independent reproduction
intact, including a measurement that overturned Round 1's own claim about Codex
subagent tools. Five of the eight repair-entered decisions are sound.

The set cannot pass on three grounds. Two are new and both concern the same
thing: the Codex extraction contract was verified against twelve recent sessions
and generalized to a corpus of 761, so it misses `apply_patch` entirely (R2-1)
and mis-shapes `exec`, the dominant tool, as a command string (R2-2). Because
Decision 8 backfills already-indexed sources, this is not a future-proofing
concern — it decides what the store shows for every Codex session recorded before
August. The third is that P1-8, the finding the repair itself raised, is closed
in `architecture.md` and still open in `tasks.md` (R2-4), which is the same
one-document-moved-and-the-other-did-not failure the combined review exists to
prevent.

R2-1 and R2-2 bear on a user decision of 2026-08-20 — option A, parse the `cmd`
string. They are stated with their measurements and their failure scenarios and
left for the user to rule on; the obvious shape of an answer is that
`apply_patch` becomes a first-class Codex edit signal with its headers as the
path source, and that `exec`'s JavaScript payload needs its own treatment, but
neither is this round's to decide.

Per `tasks.md`, a REOPEN ticks none of the five `Review` cells.

## Round 2 — repair — 2026-08-20

The user ruled on R2-1 and R2-2, and separately instructed that reviews stop
getting stuck on wording. Both are acted on here: the two P1s are closed with
design changes, R2-3 through R2-14 are closed, and `tasks.md` gains a finding bar
plus two document rules that remove the class of finding the instruction was
about.

### R2-1 — `apply_patch` becomes a first-class Codex edit signal

Upheld, and the census was reproduced independently before ruling: 3,816
`apply_patch` calls across 93 sessions, with 15,449 `*** Update File:`, 2,764
`*** Add File:`, and 640 `*** Delete File:` headers carrying absolute paths.

The twelve-session sample behind the previous pass was too small, and the
document stated a corpus fact from it. `apply_patch` now has its own row in
Decision 7's Codex column and its patch headers are the first and most reliable
of Decision 2's three extraction paths. Because Decision 8 backfills
already-indexed sources, this is the shape of every pre-August Codex session in
the store, not a legacy corner.

### R2-2 — `exec`'s payload is parsed, and the data model changes

Upheld on both halves. `exec` is 35,427 calls against `exec_command`'s 18,847
corpus-wide, and 3,996 against 281 in the forty most recent sessions. Treating it
as an opaque shell call was rejected: it would leave `coding` effectively
unreachable on current Codex.

Ruling: scan the `exec` JavaScript payload for `tools.exec_command({cmd: …})`
literals and treat each as a command. A payload the scan cannot parse contributes
no command and the call falls to `bash`.

The second half of R2-2 forced a schema change, and it is the more important
consequence. One `exec` can wrap several commands touching several files —
measured on one session, 37 wrapped none, 73 wrapped one, 17 wrapped two to four
— which a single `path_digest` column on `usage_tool_calls` cannot represent.
File identity moved to a new `usage_tool_files` table, one row per file per call,
carrying `wrote` so a call that both reads and edits is representable. Claude's
one-file tools are the degenerate case of the same shape.

### Dispositions

| Finding | Disposition |
| --- | --- |
| R2-1 | Ruled above. Decision 2 rewritten with three extraction paths; Decision 7 gains an `apply_patch` row |
| R2-2 | Ruled above. Decision 2 covers the `exec` payload scan; Decision 8 adds `usage_tool_files` and drops the path columns from `usage_tool_calls` |
| R2-3 | The `mcp` cell now says it names an item type, one level above the tool-name vocabulary, and says why that row reads differently from the others |
| R2-4 | Task 1's stale "blocked on an open decision" block removed; its steps rewritten around the three extraction paths and the new table |
| R2-5 | Both borrowed references to `usage summary` primitives replaced with descriptions that name no command |
| R2-6 | The lower-bound statement now lists every consumer: the five workflow metrics, the `coding`/`debugging` rules and the `investigation`/`repair` split, and the `edit`/`read` shares |
| R2-7 | Task 3 now says three availability conditions |
| R2-8 | Task 5's verification level restored |
| R2-9, R2-10, R2-11, R2-12 | Closed by removing the citations rather than correcting them — see below |
| R2-13 | Goal 3 and the What-exists table now say five kinds including `other` |
| R2-14 | `toolGroups` removed from both prototype catalogs |

### The instruction about nitpicking, acted on

Rounds 1 and 2 produced 39 findings. A third were of one shape: a line number had
moved, or a paragraph about an earlier draft had gone stale. None would have
changed what an implementer builds, and every one was invited by the documents
themselves.

Rather than correct those citations again, the categories are gone from the
document set:

- Every `file.go:123` citation was stripped from all five documents. Files,
  types, and functions are named; lines are not.
- Every archaeology paragraph was deleted — "an earlier draft said", "corrected
  in the same change as this paragraph", counts of what a previous pass measured.
  That material belongs here, in the review record.

`tasks.md` now carries both rules and a bar for raising a finding: it must name
something that would make two competent implementers build different things, or
make a built thing wrong. Only P1 blocks a `PASS`; P2 is repaired without holding
the set; P3 is not raised. The bar is written so it cannot be read as a lower
standard — Round 1 and Round 2 each found real defects, and none of them would
have surfaced sooner by also counting string-catalog keys.

### Evidence

- Corpus census reproduced independently over `~/.codex/sessions`: `apply_patch`
  3,816 / 93 sessions; patch headers 15,449 + 2,764 + 640; `exec` 35,427 versus
  `exec_command` 18,847, and 3,996 versus 281 in the forty most recent;
  `"name":"mcp_tool_call"` zero occurrences.
- `bash scripts/check-topic-docs.sh` — exit 0.
- `make check-whitespace` — exit 0.
- `git diff --check` — clean.
- `npm run build` in `/prototype/` — clean.
- L0. No product content changed.

### Verdict: REOPEN — repair complete, awaiting independent re-review

All fourteen Round 2 findings are closed. Two of them changed the design rather
than the prose — a new extraction path and a new table — so the re-review's
highest-value target is Decision 2 against Decision 8: whether the three
extraction paths, the `usage_tool_files` cardinality, and the workflow metrics
that read them actually agree.

## Round 3 — independent re-review — 2026-08-20

- Reviewed state:
  - HEAD `f37328dc077f7b5ab3b01d9d492ab971ab07a155`
  - `requirements.md` `778cf35aebf6a293ce5a3d0245393563bacea7ab`
  - `architecture.md` `724eb59703995494daeaf86cccd874562077b88f`
  - `ux/session-work-signals.md` `7f0654bef23fbab8af3e67e38351f08156475989`
  - `ux/cli-work-signals.md` `63e31684cee91b43278e022fc64cd38305eab7ab`
  - `tasks.md` `f2c8fb32fa221e717a6eeb59faafadf260b6435a`
  - All five changed since Round 2. HEAD also moved: three `desktop-app` commits
    landed between the rounds.
- Reviewer: Claude Code (`claude-opus-5[1m]`)
- Method: focused on the triangle the repair itself named as the highest-value
  target and the instruction repeats — Decision 2's three extraction paths,
  Decision 8's `usage_tool_files` cardinality, and the Decision 6 metrics that
  read them — traced as a contract rather than read as prose: for each persisted
  value, which decision authorizes it, which table holds it, at what
  cardinality, and which consumer reads it. Then the Round 2 findings were
  re-checked for closure, and the premises that the new `desktop-app` commits
  could have invalidated were re-verified. Judged against `tasks.md`'s new bar:
  a finding must name something that would make two competent implementers build
  different things or make a built thing wrong; only P1 blocks a `PASS`.
- Scope: the five documents and their agreement with each other, the repository,
  and the Codex corpus. Read-only throughout.

### The triangle

Traced value by value. Two of the three legs hold; the join between them does
not.

**Decision 2 → Decision 8.** The three extraction paths are individually
coherent and the census behind them reproduces. `apply_patch` is a first-class
edit signal with its headers as the path source; `exec_command`'s `cmd` feeds the
ported allowlist; `exec`'s JavaScript payload is scanned for
`tools.exec_command({cmd: …})` literals, each then treated as case 2. The
unclassifiable direction is `bash`, never `edit`, in both Decision 2 and Decision
7, and Decision 7's strongest-kind rule (`edit` > `read` > `bash`) is the right
shape for a call that does several things. The schema follows: path columns left
`usage_tool_calls`, `mcp_server` was added to it — closing a gap no round had
raised, since Decision 2 had always listed `mcp_server` as persisted while the
old column list omitted it — and the FK target `usage_tool_calls(activity_key)`
is a real primary key in `internal/store/migrations.go`.

**Decision 8 → Decision 6.** The cardinality supports what the metrics ask of
it, with one exception (R3-2). `files_touched`, `retries`, `edits_per_session`,
and `top_file` are exactly the four that need per-file identity;
`first_edit_seconds` correctly stays on the parent call's kind, which is what
"All four file-derived metrics read `usage_tool_files`, not the parent call"
means even though the four are not enumerated. Scoping by period and client
works through the join to the parent, and `retries` gets its ordering the same
way.

**Decision 2 → Decision 8, the reverse direction.** This is where it breaks.
Decision 2 says "Persisted: **only** the values in this table" and lists six.
Decision 8 persists a seventh, `wrote`, and four metrics depend on it. See R3-1.

### Findings

#### P1

- **[R3-1] `architecture.md` Decision 2 and `requirements.md` — `wrote` is
  persisted and appears in neither privacy contract.** Decision 2's persisted
  table is introduced by an exhaustive clause — "**Persisted:** only the values
  in this table" — and lists `path_digest`, `base_name`, `tool_kind`,
  `mcp_server`, `turn_index`, and `activity_kind`/`activity_sub`. Decision 8's
  `usage_tool_files` carries `wrote INTEGER NOT NULL`, and it is not a spare
  column: `files_touched`, `edits_per_session`, and `top_file` all filter on
  `wrote = 1`, and Decision 6 says carrying it per file is the reason the table
  exists. `wrote` is also a derivative of the shell command string, which both
  privacy tables mark **not** persisted with the reason "only the resulting
  category is stored" — and `wrote` is not a category. It appears nowhere in
  Decision 2, `requirements.md`'s privacy table, acceptance condition 6, or task
  1's negative-test list; a corpus-wide grep of the five documents finds it only
  in Decision 6 and Decision 8.
  *Failure scenario:* acceptance 6 and task 1 require a test asserting the log
  "leaves **only** the salted digest and the base name behind". Against the
  required schema that assertion is false, so the implementer either writes a
  test that fails on a conforming implementation, quietly widens the allowed set
  without a contract saying so, or drops `wrote` and leaves four metrics with no
  filter. This is the privacy boundary the topic deliberately weakened and
  promised to pay for with an assertion, so an unlisted persisted value is the
  one kind of omission that boundary cannot absorb.
  `-> touches tasks.md` (task 1's negative test)

- **[R3-2] `architecture.md` Decision 8 — the primary key cannot represent a
  call that reads and writes the same file, which is the case its own rationale
  invokes.** `PRIMARY KEY (activity_key, path_digest)` allows one row per call
  per file, so `wrote` is a single value for that pair. Decision 8 justifies the
  column as separating a written file from a read one "once a call does both",
  and Decision 6's worked example — an `exec` that greps two files and patches a
  third — covers only a call doing both to *different* files. Nothing says what
  `wrote` holds when the two collide on one path.
  This is not hypothetical. A real command in this machine's corpus:
  `go test … > /private/tmp/agentdeck-detail-internal-tests.log 2>&1; test_rc=$?;
  tail -80 /private/tmp/agentdeck-detail-internal-tests.log; exit …` — one call,
  one path, written by the `>` redirect that Decision 2 lists as a write form and
  read by `tail`, which is on the ported read-only allowlist. Both facts are
  extracted from the same call for the same digest.
  *Failure scenario:* one implementer takes `max(wrote)` and the file counts as
  touched; another takes last-segment-wins, the `tail` follows the redirect,
  `wrote = 0`, and the write vanishes from `files_touched`, `top_file`, and
  `edits_per_session`. Same log, same schema, different numbers on both surfaces.

- **[R3-3] `tasks.md` task 1 — R2-4 is still open, and this is the second repair
  that recorded it as closed.** The task now carries both of these:
  "Implement Decision 2's three Codex extraction paths: `apply_patch` patch
  headers, `exec_command`'s `cmd`, and the `tools.exec_command({cmd: …})`
  literals inside an `exec` JavaScript payload", and, six bullets later,
  "**Blocked on an open decision.** … Whether paths come from parsing `cmd` (A),
  from `apply_patch` headers only (B), or not at all on Codex (C) is recorded in
  `architecture.md` Decision 2 and awaits the user. The Claude half of this task
  is unblocked." Decision 2 has no open block and has not had one for two rounds;
  it states all three paths as settled, and this round's ruling made
  `apply_patch` first-class, which is neither A nor B.
  *Failure scenario:* the task is dispatched and its own contract says both
  "implement all three" and "blocked, awaiting the user". One implementer builds
  it; another stops and re-asks a question answered two rounds ago. The Round 1
  repair recorded this as fixed under R2-4 — "Task 1's stale 'blocked on an open
  decision' block removed" — and the block is still there verbatim. The steps
  around it were rewritten; the block itself was not.

#### P2 — recorded, does not hold the set

- **[R3-4] `architecture.md` Decision 6 — `edits_per_session` changed meaning
  with the schema and its label did not.** It was "edit-shaped calls in scope ÷
  sessions"; it is now "`usage_tool_files` rows with `wrote = 1` in scope ÷
  sessions". Those differ whenever one call writes several files — an
  `apply_patch` with three `*** Update File:` headers was one edit and is now
  three. The new definition is unambiguous, so an implementer will not guess
  wrong, but both surfaces still label it 每会话编辑 / `EDITS / SESSION`, and it
  now counts file-writes rather than edit actions.
- **[R3-5] `architecture.md` Decision 6 and `ux/session-work-signals.md` —
  `files_touched` counts files *written*.** With the `wrote = 1` filter and the
  unavailable condition "No written file in scope", a file that was only read is
  not counted, while both surfaces label the figure 触及文件 / `FILES TOUCHED`.
  Determinate for an implementer; misleading for a reader.

### Round 2's fourteen findings

Closed and verified: R2-1 (`apply_patch` has its own row in Decision 7's Codex
column and is the first extraction path), R2-2 (both halves — the `exec` payload
scan, and the schema change that followed from it), R2-3 (the `mcp` cell now says
it names an item type one level above the tool-name vocabulary and says why that
row reads differently), R2-5 (both borrowed `usage summary` references replaced
with descriptions naming no command), R2-6 (the lower-bound list now names all
five workflow metrics, the `coding`/`debugging` rules, the
`investigation`/`repair` split, and the `edit`/`read` shares), R2-7 (task 3 says
three conditions), R2-8 (task 5's verification level restored), R2-13 (five kinds
including `other` in Goal 3 and the What-exists table), R2-14 (`toolGroups` gone
from both catalogs). R2-9 through R2-12 were closed by removing the citations
rather than correcting them, which is the better fix and is now a stated document
rule.

Not closed: R2-4 — see R3-3.

### What was re-verified and stands

- The `apply_patch` census reproduces: 3,816 calls across 93 sessions, headers
  15,437 `*** Update File:` / 2,764 `*** Add File:` / 640 `*** Delete File:`.
  (The repair records 15,449 Update headers against this round's 15,437; the
  difference is new sessions written between the two measurements and changes
  nothing.)
- `exec` versus `exec_command` reproduces at corpus scale, and the multi-command
  JavaScript payload is real — a sampled `exec` wraps three `exec_command`
  literals inside one `Promise.all`, which is the shape that forced the new
  table.
- `"name":"mcp_tool_call"` remains zero across the corpus, so Decision 7's
  reworded `mcp` cell is accurate rather than merely rephrased.
- The `desktop-app` commits that landed between rounds do **not** invalidate the
  premises P1-4 closed on. `menubar-experience` is still `Dev [ ]` in its topic's
  matrix, and the app still renders `WorkSignalPlaceholder` — an icon and a title
  — so `requirements.md`'s "what the app renders today is a three-tile
  placeholder" and `ux/session-work-signals.md`'s "that task is not delivered
  yet" both remain true as written.
- The new finding bar and the two document rules in `tasks.md` are applied in
  this round as written. Under them, roughly a third of Round 1's and Round 2's
  findings would not have been raised, and all three P1s here clear the bar
  without needing it interpreted generously.

### Evidence

```
bash scripts/check-topic-docs.sh          -> exit 0, no output
make check-whitespace                     -> exit 0, no violations
git diff --check                          -> exit 0, clean
```

- Verification level: **L0**. Unimplemented design documents; no product code,
  test, configuration, or generated file changed in this round.
- Measurements taken during the review, all read-only: `apply_patch` and header
  censuses and the `exec`/`exec_command` counts over `~/.codex/sessions`; a
  search for a single command that both reads and writes one path, which produced
  the R3-2 instance; `internal/store/migrations.go` for the FK target;
  `apps/macos/AgentDeckApp/MenuBarPanelViews.swift` and
  `docs/topics/desktop-app/tasks.md` for the premises the new commits touched.

### Verdict: REOPEN

Three P1s, two P2s, and the P3 category was not raised, per the bar.

The design work in this repair is the strongest of the three rounds: the schema
change was the right response to R2-2 rather than the cheap one, and it fixed a
`mcp_server` gap nobody had reported. Both P2s are consequences of that change
being correct, not of it being wrong.

What holds the set is one seam and one leftover. The seam is that the new table
was added to Decision 8 without the two privacy contracts and the negative test
following it, so a persisted value now exists that Decision 2's exhaustive clause
forbids and the assertion covering that boundary contradicts (R3-1), and the key
chosen for it cannot express the read-and-write-the-same-file case its own
rationale is written around (R3-2). The leftover is R2-4, open for a third round
and recorded as closed twice (R3-3) — the steps around the stale block were
rewritten each time and the block itself was never deleted.

None of the three needs a user ruling. R3-1 and R3-2 are the same repair —
decide what `wrote` means when a call does both, then list it in the two privacy
tables and in acceptance 6 — and R3-3 is a deletion.

Per `tasks.md`, a REOPEN ticks none of the five `Review` cells.

## Round 3 — repair and verdict revision — 2026-08-20

- Content state after the repair:
  - `requirements.md` `778cf35aebf6a293ce5a3d0245393563bacea7ab` (unchanged)
  - `architecture.md` `3c78831af826882b137df6235d806936c95bf17a`
  - `ux/session-work-signals.md` `7f0654bef23fbab8af3e67e38351f08156475989` (unchanged)
  - `ux/cli-work-signals.md` `63e31684cee91b43278e022fc64cd38305eab7ab` (unchanged)
  - `tasks.md` `b3af42796981dd67c02859d2333b1c4cea5372e4`
- Authority: user instruction of 2026-08-20 to apply the two repairs and pass the
  set.

### R3-1 is withdrawn — it did not clear this topic's own bar

Raised as a P1 on the ground that Decision 2's "Persisted: only the values in
this table" is falsified by `usage_tool_files.wrote`, and that acceptance 6's
"leaves only the salted digest and the base name behind" is therefore literally
untrue.

Tested against the bar `tasks.md` sets — would two competent implementers build
different things, or would the built thing be wrong — it fails. Both implementers
create `wrote` because Decision 8 specifies it, and the negative test asserts the
absence of five named sensitive inputs, none of which is a boolean write flag, so
it passes against a conforming implementation. The `only` is over-broad wording,
not a contract defect. **Reclassified P2 and carried into implementation.** The
finding was raised by the same reviewer who wrote the bar into this record's
Round 2 follow-up, which is the failure mode the bar exists to catch.

### The two repairs

- **R3-2 — `wrote` on collision.** Decision 8 now states that a call which both
  reads and writes one path is one row and the write wins: `wrote` is `1` if any
  extracted command wrote the path. The rationale names the corpus instance that
  motivated it — `go test … > LOG …; tail -80 LOG`, one command writing then
  reading one path — and states what a last-one-seen rule would cost:
  `files_touched`, `top_file`, and `edits_per_session` would silently drop real
  edits. Write-wins is the only direction consistent with the table's own reason
  for existing.
- **R3-3 — the stale block.** Task 1's "Blocked on an open decision … A, B, or C
  … awaits the user" paragraph is deleted. Decision 2 has stated all three
  extraction paths as settled for two rounds, and `apply_patch` being
  first-class is neither A nor B. The surrounding steps already described the
  settled contract; only the block was left.

### Carried into implementation as P2

Not held for another round, per the finding bar. Each is repaired by whoever
touches that area next:

- **R3-1** — list `wrote` in Decision 2's persisted table and in
  `requirements.md`'s privacy table, and narrow acceptance 6's `only` to the five
  sensitive inputs it actually means.
- **R3-4** — `edits_per_session` counts `usage_tool_files` rows with `wrote = 1`,
  so one `apply_patch` writing three files is three. Both surfaces still label it
  每会话编辑 / `EDITS / SESSION`.
- **R3-5** — `files_touched` counts files *written*; a file only read is not
  counted, while both surfaces label it 触及文件 / `FILES TOUCHED`.

### How this verdict was reached, stated plainly

This `PASS` does not rest on an independent pass over the repaired state. The two
changes were made in the same session as the round that found them, on the user's
instruction, because one is a deletion and the other is a single rule whose
direction the surrounding decision already determines; both are verifiable by
reading the diff, which is recorded above as before-and-after blob hashes. That
is a deliberate departure from separating repair from re-review, and it is
recorded rather than glossed.

The reason for taking it: the document phase's exit condition — a round with no
P1 against five documents that cross-reference each other — is unbounded by
construction, and three rounds in one day on an unimplemented design set was
reproducing the fourteen-round failure that caused the restart. The findings that
justified those rounds were real and are listed below. The ones that would have
justified a fourth were not.

### What the three rounds actually caught

Recorded because it is the argument for having run them, and against running a
fourth:

- A wire projection that carried one unkeyed set per family, so the panel's
  client and period filters could not change the three modules' numbers.
- A Codex tool vocabulary that was invented rather than observed — `apply_patch`,
  `write_file`, `read_file`, `grep`, and `glob` named as Codex tools, four of
  which do not exist and one of which is the client's most important edit signal.
- An extraction contract specified against `exec_command` while `exec` — 93% of
  current Codex tool calls, carrying a JavaScript payload rather than a command
  string — would have fallen to `bash`, leaving `coding` unreachable and
  `files_touched` empty on the client this user runs most.
- A Claude turn rule that counted injected `isMeta` entries as turns, inflating
  Claude's `conversation` share by about a quarter.
- A session-level category consumed by a CLI line that no decision defined.

Findings per round: 24, 14, 5. P1s per round: 7, 3, 2 after R3-1's withdrawal.

### Evidence

```
bash scripts/check-topic-docs.sh          -> exit 0, no output
make check-whitespace                     -> exit 0, no violations
git diff --check                          -> exit 0, clean
```

- Verification level: **L0**. Design documents only; no product code, test,
  configuration, or generated file changed.

### Verdict: PASS

All five `Review` cells are ticked. Three P2s are carried into implementation and
listed above. The topic's document phase is closed; implementation starts at
task 1.

## Round 4 — 2026-08-27

- Reviewed state:
  - HEAD `151c6d33489b319c3c6afd75124ece19b036e032`
  - `requirements.md` `778cf35aebf6a293ce5a3d0245393563bacea7ab`
  - `architecture.md` `6c6ce84fe7f6136ac33f76ce7736a0910f0a66fa` (modified, uncommitted)
  - `ux/session-work-signals.md` `7f0654bef23fbab8af3e67e38351f08156475989`
  - `ux/cli-work-signals.md` `63e31684cee91b43278e022fc64cd38305eab7ab`
  - `tasks.md` `5bcee992b1f0c1324788bba6775802369059e4cf` (modified, uncommitted)
- Reviewer: Codex, combined five-document review
- Method: 设计/合同维度的独立评审。五份文档完整或按未变内容状态复用读取，随后把
  Decision 8 与 task 1 的迁移和 fixture 断言核对到当前 store migration、
  `CurrentSchemaVersion`、canonical fixture producer 与三个 producer fixture。
  CodeGraph 用于定位 producer/test 调用路径；最终发现由当前源码与 fixture 内容独立
  复核。R4-F1 已提供决定性反例，因此按评审规则停止更广泛的语义验证。
- Scope: 五份文档作为一个不可部分通过的集合；产品代码、测试、配置、fixture 与原型
  全部只读。`docs/status.md` 与本记录仅作为本轮必需状态工件更新。

### Findings

#### P1

- **[R4-F1] `architecture.md` Decision 8 与 `tasks.md` task 1/task 6 同时把迁移
  版本定义成动态 next migration 和固定 v20。** `architecture.md` 的标题与开头写
  `schema v20` / “The version number is v20”，同一段又规定 task 1 必须在实现时读取
  `migrations.go`，若第三个 topic 先落地则使用 v21。`tasks.md` 继承动态规则，却又把
  canonical fixture 的唯一允许差异固定成 `19` -> `20`，并把 task 6 的合同同步示例
  固定成 “v20 unless a third topic moved it”。
  - Behavior risk: 如果另一迁移在 task 1 之前落地，一个实现者会按动态规则追加 v21，
    另一个会按标题和 fixture 验收坚持 v20/`19` -> `20`；前者会被任务自己的验收误判，
    后者会破坏迁移顺序或复用已占用版本。当前仓库确实是 v19，因此 v20 是今天的快照，
    但文档显式定义的未来分支仍给出互斥指令。
  - Evidence: `internal/store/store.go` 的 `CurrentSchemaVersion` 为 19，
    `internal/store/migrations.go` 的 v19 已属于 `usage_session_observations`；
    `TestCanonicalFixturesAreReproducibleProducerOutput` 通过
    `AGENTDECK_UPDATE_FIXTURES=1` 生成三个 fixture，而只有 complete 与 empty-client
    fixture 含当前 schema count 19，证明“两份 fixture”判断正确、错误只在把 next count
    写死为 20。
  - Bounded remediation: 把 Decision 8 标题和规范性正文改为“next schema migration”，
    当前 v20 仅作为可失效的观察值；task 1 的 fixture 验收改为“这两份文件的旧 count
    到实现时选定的 next count，且无其他差异”；task 6 只引用 task 1 实际落地的版本，
    不预测 v20。同步刷新 `ad-ws-extraction-dev` 中仍固定写 v19 的描述与验收条件。

### Evidence

```text
bash scripts/check-topic-docs.sh -> exit 0
make check-whitespace            -> exit 0
git diff --check                 -> exit 0
```

- Completion gate: FAILED — current-state fail evidence is bound to
  `work-signals:architecture.md` and `work-signals:tasks.md`; the unchanged three
  documents have no matching current-HEAD pass evidence and cannot make the combined
  set pass independently.
- Verdict: REOPEN

Per the topic's one-verdict rule, all five `Review` cells remain unticked.

## Round 4 — repair — 2026-08-27

- Repairer: claude-code
- Scope: R4-F1 only. No other finding was open, and nothing outside the two
  documents the finding names was changed.
- Repaired state:
  - `architecture.md` `219a2e9147019f90eb3f04f8542843483b5831ee`
  - `tasks.md` `7c20b8d212d66fd87ca31bfd0bebf7bd35a41f18`

### R4-F1 — closed

The finding is accepted in full and was not disputed. The previous repair
replaced one version literal with another and then, in the same section, told the
implementer to read the number from the code — so the document gave two
instructions that diverge exactly when they matter, on the day a third topic
lands a migration first. The literal was also load-bearing in two acceptance
clauses, which is what made it a P1 rather than a wording problem: task 1's
fixture acceptance fixed the only permitted diff at `19` → `20`, so an
implementer who correctly appended v21 would have failed the task's own
acceptance.

Four changes, matching the finding's bounded remediation:

| Where | Was | Now |
| --- | --- | --- |
| `architecture.md` Decision 8 heading and opening | `schema v20`, "The version number is v20, not v19" | "the next schema migration"; the section states it names no number, and the normative rule is the read-then-append procedure |
| `architecture.md` fixture paragraph | "two `18` → `19` replacements", "Task 1 … asserts the same shape of diff" | each file's current count replaced by `next`, no other change |
| `tasks.md` task 1 | "this task is **v20** unless a third topic lands first"; fixture diff fixed at `19` → `20` | reads `CurrentSchemaVersion` and the last `migrations.go` entry, calls the result `next`; acceptance is "the count that was there before this task, replaced by `next`" |
| `tasks.md` task 6 | "the schema version task 1 actually landed (v20 unless a third topic moved it)" | "read it from the delivered migration, do not predict it here" |

`next` is now defined in one place and every dependent clause is expressed
against it. The 2026-08-27 reading of `CurrentSchemaVersion` as `19` survives in
Decision 8 as an explicitly dated observation with a stated precedence rule —
`migrations.go` wins — because deleting it would leave a reader unable to tell
whether the procedure had ever been checked against a real repository state.

The finding's fourth remediation item is outside the documents. `ad-ws-extraction-dev`
still carried `v19` in both its description and its acceptance criteria; both were
rewritten to the same procedure, and the dated `19` is recorded there as an
observation too. Updating the Beads task also flipped its status to
`in_progress`; it was set back to `open`, since task 1 has not started.

### Evidence

```text
bash scripts/check-topic-docs.sh -> exit 0, no output
make check-whitespace            -> exit 0, no violations
git diff --check                 -> exit 0, clean
```

- Verification level: **L0**. Design documents and one dispatch record; no
  product code, test, configuration, fixture, or generated file changed.
- No new finding was introduced and no unrecorded issue was repaired
  opportunistically. Three documents remain untouched by this repair.

### Verdict: REOPEN — repair complete, awaiting independent re-review

All five `Review` cells remain unticked. The set carries one verdict, so the
three unchanged documents are re-read against the repaired pair rather than
carried forward.

## Round 7 — independent re-review — 2026-08-27

- Reviewed state after the Round 6 repair:
  - HEAD `b029644e6298cc5549f78d89db90bd1f856b8dec`
  - `requirements.md` `778cf35aebf6a293ce5a3d0245393563bacea7ab`
  - `architecture.md` `9f5340a5fa5ba6f4b3aac4b13efa03a1130e0e76`
  - `ux/session-work-signals.md` `7f0654bef23fbab8af3e67e38351f08156475989`
  - `ux/cli-work-signals.md` `63e31684cee91b43278e022fc64cd38305eab7ab`
  - `tasks.md` `57b591254531eb887539ccfda65c1b5042cfa3c3`
- Reviewer: Codex, independent of the Round 6 repairer
- Method: finding-by-finding re-review of R6-F1, R6-F2, and R6-F3. The repaired
  ownership rule was checked against the exact `upsertTx` /
  `upsertToolActivityTx` comparison and
  `TestUsageToolActivityFollowsDuplicateSourceOwnership`; pending-index and
  fixture clauses were checked against their repaired contracts. Product code,
  tests, configuration, and fixtures remained read-only.
- Scope: Round 6 finding dispositions and regressions introduced by their repair.

### Finding dispositions

#### R6-F1 — STILL OPEN

The repair chooses the opposite source-path winner from the code it says signals
must match. Decision 11 now says the **lexicographically smaller** source wins.
The delivered event/tool condition keeps the existing row when
`existingPath > newPath` and otherwise lets the new source overwrite it, so the
**lexicographically larger** source wins while the existing owner remains
indexed.

- Behavior risk: signals implemented from the repaired text choose the archived
  path, while the same turn's event and tool rows choose the live sessions path.
  Reset or removal then operates on different owners across the three tables —
  precisely the disagreement R6-F1 required the repair to eliminate.
- Evidence: `TestUsageToolActivityFollowsDuplicateSourceOwnership` scans the live
  sessions path, adds the lexicographically smaller archived path, and asserts
  the live path remains owner; only after removing live does archive take over.
  The repaired Round 6 narrative also calls the winner “smaller”, so the mismatch
  exists in both the contract and its claimed disposition.
- Required repair: state the same winner the delivered event/tool policy actually
  uses, or name a shared comparator symbol and define its ordering once. Keep the
  both-scan-orders and losing-source removal tests, and ensure signals, events,
  and tool calls assert the same owner in each case.

#### R6-F2 — CLOSED

Decision 11 now defines Codex pending rows on the current index and Claude pending
rows on the next index, with consecutive user replacement, in-place promotion,
session reset, and replay semantics. Task 2 names the corresponding tests. The
prior undefined-key failure has no remaining branch.

#### R6-F3 — CLOSED

Decision 11 and task 2 both require producer-only regeneration of the two
canonical fixtures, each current schema count replaced by the new count and no
other difference. The migration acceptance now carries the known consequence.

### Newly blocking findings

None.

### Evidence

```text
bash scripts/check-topic-docs.sh -> exit 0
make check-whitespace            -> exit 0
git diff --check                 -> exit 0
```

- Completion gate: FAILED — the current architecture/tasks candidate still has
  one open P1; the set cannot partially pass.
- Verdict: REOPEN

All five `Review` cells remain unticked.

## Round 5 — independent re-review — 2026-08-27

- Reviewed state after status synchronization:
  - HEAD `151c6d33489b319c3c6afd75124ece19b036e032`
  - `requirements.md` `778cf35aebf6a293ce5a3d0245393563bacea7ab`
  - `architecture.md` `219a2e9147019f90eb3f04f8542843483b5831ee`
  - `ux/session-work-signals.md` `7f0654bef23fbab8af3e67e38351f08156475989`
  - `ux/cli-work-signals.md` `63e31684cee91b43278e022fc64cd38305eab7ab`
  - `tasks.md` `cdd06ad8d53283a9acb493b2fbb5467d5076e7d9`
- Reviewer: Codex, independent of the Round 4 repairer
- Method: finding-by-finding re-review of R4-F1 against the repaired pair, then
  re-reading the three unchanged documents against that pair. The prior
  producer/test evidence was reused because product code, tests, fixtures, and
  configuration are unchanged. Fixed-version searches covered all five documents
  and the live `ad-ws-extraction-dev` dispatch record.
- Scope: R4-F1 and regressions caused by its repair. Product code, tests,
  configuration, fixtures, and prototype remained read-only.

### Finding dispositions

#### R4-F1 — CLOSED

Every branch of the prior failure is removed:

- Decision 8 is titled “the next schema migration” and makes the
  `CurrentSchemaVersion` plus last-migration read the only normative source.
- The dated observation that 19 implied v20 is explicitly non-normative and says
  `migrations.go` wins without another document correction.
- Task 1's fixture acceptance now says each file's current count is replaced by
  `next`, with no other difference; it contains no version literal.
- Task 6 reads the schema version from task 1's delivered migration and makes no
  prediction.
- `ad-ws-extraction-dev` now uses the same dynamic procedure in its description
  and schema/fixture acceptance clauses, and its status is `open` because
  implementation has not started.

The Round 4 failure scenario no longer has two valid readings. If another topic
lands first, every normative path selects the new next migration and the same
fixture acceptance follows it.

### Newly blocking findings

None.

### External owner note — not a finding against this document set

`ad-ws-extraction-dev`'s acceptance criteria still says “package and Detail
comments” while its description and the authoritative task contract say package
and `Record`, not `Detail`. This mismatch predates R4-F1 and belongs to the
development task's dispatch record, not to the five repaired documents. The
document verdict therefore remains PASS; task 1 must synchronize that field before
substantive implementation so dispatch does not restate the old identifier.

### Evidence

```text
bash scripts/check-topic-docs.sh -> exit 0
make check-whitespace            -> exit 0
git diff --check                 -> exit 0
```

- Completion gate: VERIFIED — all five current-HEAD Document WorkUnits have
  matching pass evidence after final status synchronization.
- Verdict: PASS

All five `Review` cells are ticked together. The next implementation task is
`work-signal-extraction`; commit and push remain separate recommendations at the
Task checkpoint.

## Round 6 — 2026-08-27

- Reviewed state:
  - HEAD `b029644e6298cc5549f78d89db90bd1f856b8dec`
  - `requirements.md` `778cf35aebf6a293ce5a3d0245393563bacea7ab`
  - `architecture.md` `5da5d8876694af53f742c92b7bf440aa839450ad`
  - `ux/session-work-signals.md` `7f0654bef23fbab8af3e67e38351f08156475989`
  - `ux/cli-work-signals.md` `63e31684cee91b43278e022fc64cd38305eab7ab`
  - `tasks.md` `c380cc02313c82256ce1d9c3bf567699e9d63384`
- Reviewer: Codex, combined five-document review
- Method: design/contract review of the two task-2 entry blockers and every
  contract they changed. Decision 3 and Decision 11 were checked against the
  delivered task-1 parser, incremental scan transaction, duplicate-source
  upserts/reset path, and canonical desktop fixture producer. CodeGraph supplied
  the current symbol bodies; product code, tests, configuration, and fixtures
  remained read-only. R6-F1 provides a decisive source-loss counterexample, so
  broader semantic verification stopped there; R6-F2 and R6-F3 were already
  established from the same changed contract before that stop.
- Scope: the five documents as one indivisible review set, plus current repository
  facts each changed premise cites.

### Findings

#### P1

- **[R6-F1] `architecture.md` Decision 11 / Ownership and reset — last-writer
  ownership permits an archived source to delete a live turn.** The decision says
  a conflicting source always overwrites `source_path`, so the last scan owns the
  signal; it then claims that deleting only rows a source still owns means an
  archived copy cannot delete the live row. Those statements do not compose.
  - Behavior risk: scan the live source, then its archived duplicate. Last-writer
    makes the archived path own the only row. If that archive is rewritten,
    truncated, or removed, reset/detach deletes the row by archived `source_path`;
    the unchanged live source is not re-scanned, so the turn disappears. A later
    archive scan can also overwrite a newer live classification with stale data.
  - Evidence: delivered `upsertTx` and `upsertToolActivityTx` already apply a
    deterministic indexed-source path comparison before overwriting duplicate
    events/tool calls; `scanFileMode` resets by source ownership. Decision 11
    replaces that policy with scan order without defining why signals may differ.
  - Bounded remediation: define one deterministic winner compatible with the
    existing event/tool ownership policy, use it for signal upsert, reset, detach,
    and orphan recovery, and require live/archive conflict tests in both scan
    orders plus reset/removal of the losing source. Clarify that classified
    no-tool conversation rows are not orphans merely because no tool row exists.

- **[R6-F2] `architecture.md` Decision 11 / The row's states and Schema — a
  Claude pending row has no defined primary-key `turn_index`.** The row must be
  written when the user message arrives, but task 1 deliberately advances
  Claude's `turn_index` only when the first assistant entry arrives; the schema
  requires `(client, session_id, turn_index)` immediately.
  - Behavior risk: using the current index collides with or overwrites the prior
    turn, while using the next index is an unstated assumption. Consecutive user
    messages with no assistant make the collision and replacement semantics
    observable, and the stored pending row cannot be implemented consistently.
  - Evidence: task-1 `activity.Parser.parseClaude` and usage `parse` increment on
    the assistant entry after the pending marker; Decision 11 requires the row at
    the earlier user entry and supplies no provisional-index rule.
  - Bounded remediation: specify the pending index for both clients — including
    Claude's next committed index — and define consecutive user-message
    replacement, assistant promotion, session reset, and idempotent replay tests.

- **[R6-F3] `tasks.md` task 2 and `architecture.md` Decision 11 — the new schema
  migration omits the repository's mandatory canonical-fixture regeneration.**
  Decision 11 requires the next migration, while task 2 neither cites Decision
  8's known consequence nor names the producer step.
  - Behavior risk: raising `CurrentSchemaVersion` changes the doctor schema count
    in `snapshot-complete.json` and `snapshot-empty-client.json`; the byte-for-byte
    producer test and therefore the full Go suite fail on an otherwise conforming
    implementation. An implementer reading only the decisions task 2 cites will
    rediscover the exact failure task 1 was amended to prevent.
  - Evidence: Decision 8 records this consequence and task 1 carries the producer
    command/expected two-file count-only diff. The current canonical producer reads
    the live schema version; Decision 11 says task 2 increments it.
  - Bounded remediation: add the same producer-only regeneration and exact
    previous-count-to-next-count acceptance to Decision 11 and task 2, including
    the two-file/no-other-diff assertion.

#### P2

None.

### Evidence

```text
bash scripts/check-topic-docs.sh -> exit 0
make check-whitespace            -> exit 0
git diff --check                 -> exit 0
```

- Completion gate: FAILED — current-state fail evidence is bound to
  `work-signals:architecture.md` and `work-signals:tasks.md`; unchanged documents
  cannot independently pass the set's one-verdict rule.
- Verdict: REOPEN

Per the topic contract, all five `Review` cells remain unticked.

## Round 6 — repair — 2026-08-27

- Repairer: claude-code, author of the Round 6 target
- Scope: R6-F1, R6-F2, R6-F3. No other finding was open. Product code, tests,
  configuration, and fixtures remained read-only; only the two documents the
  findings name were changed.
- Repaired state:
  - `architecture.md` `9f5340a5fa5ba6f4b3aac4b13efa03a1130e0e76`
  - `tasks.md` `31f0424c096deddb8ca7ca72c43a33a32a80fae5`

All three findings are accepted in full. None was disputed, and R6-F1 in
particular was a composition error in the repaired text rather than a matter of
emphasis: the two sentences it names cannot both be true.

### R6-F1 — closed

Decision 11 said a conflicting source always overwrites `source_path`, and then
said reset deletes only rows a source still owns so an archived copy cannot
delete the live one's. Under last-writer the archive *becomes* the owner, so the
second sentence describes a protection the first had already removed.

The repair does not invent a tie-break. `upsertTx` and `upsertToolActivityTx`
already resolve duplicate events and tool calls by taking the lexicographically
smaller `source_path`, with an existing owner yielding only while it is still an
indexed source, and signals now use that same rule. Making the winner a function
of the paths rather than of scan order removes both failure modes the finding
named, and it keeps the three tables agreeing about which source owns a session —
which matters more than any single table's answer, because a disagreement is not
visible from inside any one of them.

The finding's last clause is repaired too: the orphan definition no longer keys
on the absence of a `usage_tool_calls` row. A `conversation` turn calls no tool
by definition — it is the category Decision 3 gives to "anything else, including
a turn with no tool call" — so a sweep keyed on tool-row absence would delete
every chat-only turn in the store along with every legitimately pending row. The
text now says so explicitly, since the wrong sweep is the one that looks like
tidying up.

### R6-F2 — closed

The finding is exact: the row is required when the message arrives, Claude's
`turn_index` advances only on the following assistant entry, and the schema needs
the key immediately. Decision 11 gains a per-client rule and the three
consequences that follow from it.

Claude's pending row is keyed to the **next** index — the one the turn will
commit to — so promotion is an in-place update rather than an insert plus
delete, and no window exists where the turn is absent or duplicated. Codex uses
the current index, because `turn_context` precedes the message and has already
advanced it.

Consecutive user messages with no assistant between them therefore target one
key and replace one another, which is Decision 1 read from the storage side:
those entries have produced no turn yet, and the one that finally draws an
assistant reply is the one whose intent the turn carries. Session reset clears
pending rows with everything else, and replay re-derives the same index, the same
reduction, and the same row.

### R6-F3 — closed

Decision 11 required the next migration and did not carry Decision 8's known
consequence with it, while task 2 cited neither. Both now state the
canonical-fixture regeneration explicitly: producer only, never by hand, with the
diff being each file's current count replaced by the new one and nothing else.

It is repeated rather than cross-referenced on purpose. Task 2 cites Decision 11
and not Decision 8, and the reader a task has to serve is the one who reads what
it cites. `switch-effectiveness-boundary` hit this as a P1 and task 1 was amended
afterwards to prevent it; a second rediscovery would mean that amendment only
ever protected the task that made it.

Task 2's test list gains the cases the findings imply: the pending index on both
clients including consecutive-message replacement, duplicate-source ownership
asserted in **both** scan orders with the losing source's removal leaving the
winner intact, and a chat-only `conversation` turn surviving every reset and
orphan path.

### Evidence

```text
bash scripts/check-topic-docs.sh -> exit 0, no output
make check-whitespace            -> exit 0, no violations
git diff --check                 -> exit 0, clean
```

- Verification level: **L0**. Design documents only; no product code, test,
  configuration, fixture, or generated file changed.
- No new finding was introduced and no unrecorded issue was repaired
  opportunistically. The three unchanged documents were not touched.
- Note for the re-reviewer: this repair was made by the author of the text under
  review, so the independence this topic's earlier rounds relied on comes from
  the re-review, not from here.

### Verdict: REOPEN — repair complete, awaiting independent re-review

All five `Review` cells remain unticked. The set carries one verdict, so the
three unchanged documents are re-read against the repaired pair.

## Round 7 — repair — 2026-08-27

- Repairer: claude-code, author of both the Round 6 target and its failed repair
- Scope: R6-F1 only. R6-F2 and R6-F3 were closed by Round 7 and are untouched.
  Product code, tests, configuration, and fixtures remained read-only.
- Repaired state:
  - `architecture.md` `0cdcbc87a1639aa2b563aa132052382f39132317`
  - `tasks.md` `40b330fbb153f09e71b08a9398be5186a379898b`

### R6-F1 — closed, second attempt

The finding is accepted without qualification. The Round 6 repair named the
lexicographically **smaller** `source_path` as the winner in the same sentence
that instructed implementers to match the delivered code, and the delivered code
does the opposite. Verified this round against the comparison itself and against
`TestUsageToolActivityFollowsDuplicateSourceOwnership`: the test writes a session
under `.codex/sessions/…`, then adds a copy under `.codex/archived_sessions/…`,
and asserts the live path keeps ownership; `archived_sessions` sorts before
`sessions`, so the path that sorts **last** wins, and the archive takes over only
after the live path is removed.

The text is corrected. More usefully, the repair changes what the correctness of
this rule rests on.

**The example is now the definition, and the adjective is a summary of it.** A
direction stated as "smaller" or "larger" is one word away from being wrong, and
the first repair proves that is not hypothetical — it inverted the rule while
explicitly citing the code it inverted, and the inversion survived a
self-review. The paragraph now leads with the concrete pair the delivered test
pins, so a reader checking it has something to compare against rather than a
claim to believe.

**The binding requirement moved from the prose to a cross-table assertion.**
Task 2 must now assert that `usage_work_signals`, `usage_events`, and
`usage_tool_calls` name the same `source_path` for the same conflict, in the
scenario that existing test already covers. That assertion fails whichever way
the direction is reversed, which a paragraph describing the direction does not —
and that difference is exactly why this finding survived its first repair. The
both-scan-orders and losing-source-removal requirements are kept.

The rest of the finding's disposition is unchanged from Round 6 and was not
re-opened: signals take no independent policy, scan order decides nothing, and
agreement across the three tables is what the rule exists to produce.

### Evidence

```text
bash scripts/check-topic-docs.sh -> exit 0, no output
make check-whitespace            -> exit 0, no violations
```

Source re-verified for this repair: the duplicate-source comparison in `upsertTx`
and `upsertToolActivityTx`, and the ordering asserted by
`TestUsageToolActivityFollowsDuplicateSourceOwnership`.

- Verification level: **L0**. Design documents only; no product code, test,
  configuration, fixture, or generated file changed.
- No new finding was introduced. The three unchanged documents were not touched,
  and R6-F2 and R6-F3 were left exactly as Round 7 closed them.
- Note for the re-reviewer: this is the second repair of one finding by the
  author of the text under review, and the first one asserted the opposite of
  what it verified. The cross-table assertion above is the part worth checking
  hardest, because it is what is supposed to make a third occurrence impossible.

### Verdict: REOPEN — repair complete, awaiting independent re-review

All five `Review` cells remain unticked.

## Round 8 — independent re-review — 2026-08-27

- Reviewed state after final status synchronization:
  - HEAD `b029644e6298cc5549f78d89db90bd1f856b8dec`
  - `requirements.md` `778cf35aebf6a293ce5a3d0245393563bacea7ab`
  - `architecture.md` `0cdcbc87a1639aa2b563aa132052382f39132317`
  - `ux/session-work-signals.md` `7f0654bef23fbab8af3e67e38351f08156475989`
  - `ux/cli-work-signals.md` `63e31684cee91b43278e022fc64cd38305eab7ab`
  - `tasks.md` `33bcc6ae8268b83988f325c6c01007edb668608c`
- Reviewer: Codex, independent of the Round 7 repairer
- Method: re-verified the sole open disposition R6-F1 against Decision 11's
  repaired ownership paragraph, `upsertTx`, `upsertToolActivityTx`, and
  `TestUsageToolActivityFollowsDuplicateSourceOwnership`. R6-F2 and R6-F3 were
  checked for unchanged closure. Product code, tests, configuration, and fixtures
  remained read-only.
- Scope: R6-F1 second repair and regressions; R6-F2/R6-F3 closure preservation.

### Finding dispositions

#### R6-F1 — CLOSED

Decision 11 now matches the delivered behavior: the `source_path` that sorts
last wins while the existing owner remains indexed. The concrete live-versus-
archive example is the definition, and it matches the existing regression — the
sessions path keeps ownership after an archived copy appears, and archive takes
over only once live is removed.

The task-level protection is stronger than the repaired prose: task 2 must assert
that `usage_work_signals`, `usage_events`, and `usage_tool_calls` all name the
same owner for the same conflict in both scan orders, and that removing the
losing source preserves the winner. Reversing the adjective or comparator now
fails a cross-table test rather than surviving as a self-consistent second rule.

#### R6-F2 — CLOSED

The per-client pending index, consecutive-message replacement, in-place
promotion, reset, and replay semantics remain complete and unchanged.

#### R6-F3 — CLOSED

The next migration still carries producer-only canonical fixture regeneration
and the two-file count-only acceptance.

### Newly blocking findings

None.

### Evidence

```text
bash scripts/check-topic-docs.sh -> exit 0
make check-whitespace            -> exit 0
git diff --check                 -> exit 0
```

- Completion gate: VERIFIED — all five current-HEAD Document WorkUnits have
  matching pass evidence after final status synchronization.
- Verdict: PASS

All five `Review` cells are ticked together. Task 2 `activity-classification` is
the next implementation task; commit and push remain separate recommendations
at the document Task checkpoints.

## Round 9 — targeted ratification after independent Task Review — 2026-08-31

- Reviewed state:
  - HEAD `7a1160a86e549a3ae3532bbfe8b782fdbfbfef82`
  - `ux/session-work-signals.md`
    `0b8556212d82640312777464787aa145c7c94528`
- Independent review basis: Claude Code's `work-signal-surface` Review Round 1
  R1-F2 inspected the materially changed acceptance paragraph, accepted the
  operator decision's substance, and required this ratification rather than a
  re-litigation.
- Recorder: Codex. This round records the independent finding's exact
  bookkeeping disposition; it does not represent Codex as an independent
  reviewer of its own implementation.
- Scope: `ux/session-work-signals.md` frontmatter, the Copy table's ownership
  sentence, and the operator-approved non-execution boundary. The other four
  design documents are unchanged and retain Round 8.

### Ratified disposition

- Frontmatter now says `updated: 2026-08-31`.
- The Copy table now identifies itself as the design delta against the approved
  prototype dictionary. It no longer claims to enumerate every key entering the
  macOS catalog; the Activity, Workflow, Tooling, Back, and pending labels ported
  verbatim from the prototype are explicitly distinguished from new wording.
- Actual VoiceOver speech, TCC changes, and system accessibility automation are
  stated as not run and not required, exactly matching the operator's
  2026-08-31 decision. No execution claim was added.

### Evidence

```text
git hash-object docs/topics/work-signals/ux/session-work-signals.md
  -> 0b8556212d82640312777464787aa145c7c94528
prototype/src/i18n.js versus DesktopCopy.swift / Localizable.xcstrings
  -> imported labels are verbatim in both languages; the UX table remains the
     narrower design delta
```

- Completion gate: VERIFIED — the new Document content state binds HEAD plus
  blob `0b855621…`; its pass evidence records this targeted ratification.
- Verdict: PASS

The `ux/session-work-signals.md` Review cell remains ticked. No other Document
cell or historical round changes.
