---
status: historical
created: 2026-08-20
updated: 2026-08-27
retired: 2026-09-01
---

# Work Signals — Architecture

Every decision below was made by the user on 2026-08-20 or derives from one that
was. Where a decision was informed by an existing implementation, that
implementation is named and what was taken from it is stated, because "we follow
CodeBurn" is not a contract and the next reader cannot check it.

## Reference implementation

CodeBurn (`codeburn` 0.9.20, a Node CLI installed on the development machine)
solves the same derivation and was read as a reference. It ships a source map,
so the relevant modules were read rather than inferred: `src/classifier.ts`,
`src/types.ts`, `src/providers/codex.ts`, `src/providers/claude.ts`.

Three things were taken from it, and each is marked at the decision it informs:
the turn as one user message plus the assistant calls that follow it; the
edit → verify → edit rework counter; and the observation that a usable
classifier must read the user's message, because tool shape alone cannot
separate coding from debugging.

Two things were deliberately **not** taken. CodeBurn keeps full file paths and
command strings in its parse cache; AgentDeck persists neither. CodeBurn exposes
thirteen flat categories; AgentDeck uses four with subcategories, because the panel is
280 pt wide and thirteen peer rows do not read at that width.

## Decision 1 — the turn is the unit, and its boundary is the user's message

**A turn is one user message plus every assistant API call and tool call that
follows it, up to the next user message.** Everything downstream is per-turn:
classification, cost attribution, and the rework counter.

This is CodeBurn's `ParsedTurn`, and it was chosen over the alternatives because
it is the only unit that means the same thing on both clients.

| Client | Boundary marker | Notes |
| --- | --- | --- |
| Codex | `turn_context` payload carries `turn_id`; a new `turn_id` opens a turn | The parser already reads `turn_context` and holds `turn_id` in `Parser`. It currently uses it only for the anonymous-call digest |
| Claude | A log entry whose `message.role` is `user` opens a turn **only if** all three hold: it is not a `tool_result` continuation, `isMeta` is not `true`, and its content is not a synthetic wrapper. Additionally, a turn is emitted only if at least one assistant API call followed it | Claude logs carry no turn identifier. Assistant messages are **not** boundaries: one Claude turn commonly contains several, and using them would make Claude's turn count systematically higher than Codex's, so any per-turn ratio would stop being comparable across clients |

The three-part Claude exclusion is not defensive coding. A large minority of
user-role text entries are injected rather than typed: skill preambles, image
cache references, `<local-command-*>` wrappers, and `<system-reminder>` blocks.
Measured in this repository's own Claude logs, `isMeta` alone accounts for 12 of
40, 14 of 73, 12 of 41, and 5 of 46 user-role text entries.

Excluding only `tool_result` would open a turn on each of them. Each such turn
calls no tool, so Decision 3 rule 4 classifies it `conversation`: a 40-turn
session would report roughly 52, and Claude's `conversation` share would inflate
by about a quarter — the exact defect this decision cites when it refuses to
treat assistant messages as boundaries.

The second rule — **no assistant API call, no turn** — is CodeBurn's
`groupIntoTurns`, and it is kept because it catches injected shapes nobody has
enumerated yet. It is not sufficient on its own: measured in one session, 10 of
12 `isMeta` entries are followed by an assistant message before the next real
user text, so under that rule alone each of the ten would open a turn *and* take
the real user message's assistant calls with it — inflating the count and feeding
the classifier a skill preamble instead of the user's request. CodeBurn has this
defect; this design does not, because the exclusion list runs first.

It does not make `conversation` unreachable. A chat-only turn still draws an
assistant reply, which is an API call; what it lacks is a *tool* call, and no
rule here keys on that.

Codex needs no equivalent exclusion. `turn_context` entries were verified to
align one-to-one with real user turns in the sampled sessions.

A turn that called no tool is still a turn and still produces a row. Emitting
only on the first tool call would make `conversation` unreachable, which is the
defect that kept the Activity module empty for chat-only work in the discarded
pass.

Turns are numbered per session in log order, starting at 1. That ordinal —
`turn_index` — is the join key for cost, because it is derivable on both clients
whereas `turn_id` is not.

## Decision 2 — what may be read, and what may be kept

The two are different, and conflating them is what made the earlier pass treat a
readable classifier as impossible.

**Readable at parse time, in process:** tool argument objects, user message text,
and shell command strings.

**Persisted:** only the values in this table. Everything read and not listed here
is dropped in the same function that read it, before the record is constructed.

| Persisted value | Shape | Consumer |
| --- | --- | --- |
| `path_digest` | `sha256(machine identity ‖ absolute path)`, hex | Distinct-file counting; most-touched-file grouping |
| `base_name` | The final path segment, e.g. `tasks.md`, capped at 128 bytes | The most-touched-file display on both surfaces |
| `tool_kind` | One of `bash`, `read`, `edit`, `mcp`, `other` | Tooling module |
| `mcp_server` | The server segment of an `mcp__<server>__<tool>` name | Top MCP server |
| `turn_index` | Integer, per session | Join key for everything per-turn |
| `activity_kind`, `activity_sub` | Decision 3's vocabulary | Activity module |
| `message_class` | One of `build`, `fault`, `none` | Decision 11's incremental classifier state |
| `intent_sub` | One of `coding`'s four subcategories, or empty | Decision 11's incremental classifier state |

The last two are the user message reduced to its decision, not to a summary of
itself. Together they take fifteen values; no message can be recovered from
them, and no directory, path, or command fragment reaches them. They exist
because Decision 11 must carry a message's verdict across a scan boundary the
message itself does not survive.

Never persisted: the absolute or relative path, any directory segment, the user
message, the command string, tool results, environment, or reasoning. The
classifier's inputs leave the process as one category label and one subcategory
label.

The digest is **salted with the stable machine identity** — the same identity the
credential key already derives from. An unsalted `sha256` of a path is recoverable
from any candidate path list and is identical on every machine, which makes it a
reversible pointer to the path rather than an opaque identity. Salting costs
nothing: every consumer counts or groups within one install, so a value that
means nothing off this machine is exactly the value they need.

Two comments in `internal/activity` carry the absolute claim — the package doc
comment and the one on **`Record`** ("deliberately has no fields for arguments,
results, command text, environment, or reasoning"). Both must be rewritten to
state what is not **retained**, in the same change that narrows the boundary.
`Detail`'s comment says only that it is the safe user-visible form and needs no
rewrite. A comment left contradicting the code is what produced a wrong premise
on 2026-08-18.

This guarantee is weaker than the one it replaces and is worth nothing unasserted.
The negative test in [`tasks.md`](tasks.md) task 1 is a completion condition of
this decision, not of that task alone. That test asserts on more than the
database: the read values must also be absent from log lines, error and warning
strings, the `Page`/`Detail` JSON, and the source-file cache, and the assertion is
made on **substrings** rather than whole strings, because `internal/activity`
truncates metadata at `maxMetadataLength = 256` and a truncated path fragment
would pass a whole-string absence check.

### How a path is obtained on Codex

Codex never puts a path in an argument key the way Claude does. It has emitted
two different edit surfaces, and both are in the store, because Decision 8
backfills already-indexed sources.

| Surface | Census over the whole `~/.codex/sessions` corpus | Path source |
| --- | --- | --- |
| `apply_patch` | 3,816 calls in 93 sessions, through 2026-07-30 | Patch headers — `*** Update File:` (15,449), `*** Add File:` (2,764), `*** Delete File:` (640), each carrying an absolute path |
| `exec_command` | 18,847 calls | The `cmd` shell string |
| `exec` | 35,427 calls, and 3,996 against 281 `exec_command` in the forty most recent sessions | A JavaScript payload that calls `tools.exec_command({cmd: …})` one or more times |

Three extraction paths follow, in decreasing reliability:

1. **`apply_patch` headers.** Deterministic. The tool is a first-class edit
   signal in Decision 7, not an `other`, and its headers need no shell parsing.
2. **`exec_command`'s `cmd`.** The write/read question is answered by the ported
   allowlist below; the which-file question by matching `apply_patch` heredocs,
   `sed -i … FILE`, `> FILE`, `cat > FILE`, `tee FILE`, and `cp`/`mv`
   destinations.
3. **`exec`'s JavaScript payload.** Scan for `tools.exec_command({…})` argument
   objects and take each `cmd` literal. One `exec` yields zero, one, or several
   commands; each is then treated exactly like case 2. A payload the scan cannot
   parse contributes no command, and the call falls to `bash`.

Treating `exec` as an opaque shell call was considered and rejected: it is 93% of
current Codex tool calls, so it would make `coding` effectively unreachable on
Codex and leave `files_touched` empty rather than merely incomplete.

**Is this command a write?** Ported from CodeBurn's `bash-utils.ts`: split on
`&&`/`;`/`|`, strip quoted strings, skip `sudo`/`npx`/`env`-assignment prefixes,
match the head against a closed read-only allowlist plus a git read-subcommand
list, and treat anything unrecognized as not-read. This half is reliable, and it
is ported rather than cited. CodeBurn does not solve the which-file half at all —
it reads a `file_path` argument, which is the Claude path — so that half is
written here.

The unclassifiable direction is always **`bash`, never `edit`**. Miscounting a
read as an edit would corrupt `files_touched`, `retries`, and the category rules
at once.

### What the parse being incomplete costs

Every consumer of the Codex write-detection inherits a lower bound, and the list
is all of them, not the file metrics alone:

- `files_touched`, `top_file`, `retries`, `first_edit_seconds`, and
  `edits_per_session` (Decision 6);
- the `coding` category rule and the `debugging` `investigation`/`repair` split
  (Decision 3);
- the `edit` and `read` shares of the Tooling module (Decision 7).

A displayed share with an unstated bound is the part that matters against
`requirements.md` acceptance 4, so both surfaces state it rather than footnote
it. Claude is unaffected: `Edit`/`Write`/`NotebookEdit` carry `file_path`
directly.

## Decision 3 — four categories, eleven subcategories

The panel shows four rows and expands one of them; the CLI prints four rows and
expands all of them under `--sub`. The category is decided first, from tool
shape; the subcategory is decided second, and may consult text.

### The message is scanned once, and that scan decides both levels

Every message-derived rule in this decision reads from **one** pass over the user
message that records, for each vocabulary below, the position of its earliest
match:

- the **fault** vocabulary — `fix`, `bug`, `error`, `broken`, `failing`,
  `crash`, `traceback`, `exception`, `not working`, and the numeric HTTP failure
  codes;
- the three **build** vocabularies that have message rules — `feature`,
  `refactoring`, and `testing`. `maintenance` has no message vocabulary; it is
  command-shaped only, and so takes no part in this scan.

`testing`'s vocabulary was named but never enumerated by the earlier draft,
which was survivable while it only picked a subcategory and is not survivable now
that it can decide a category. It is: `test`, `tests`, `testing`, `spec`,
`coverage`, `assertion`, `regression`. `failing` is deliberately absent — it
belongs to the fault vocabulary, and a message about a failing test is a
`debugging` message.

The earliest match across all five decides `message_class`:

| Earliest match is | `message_class` | `intent_sub` |
| --- | --- | --- |
| a fault word | `fault` | empty |
| a build word | `build` | the subcategory that word belongs to |
| nothing matched | `none` | empty |

Ties break toward `fault`, then in the order the Subcategories table lists them.

### Categories, in precedence order

| # | Category | Rule |
| --- | --- | --- |
| 1 | `delegation` | The turn spawned a subagent or invoked a skill or workflow. Tool-shaped, so it does not compete on message text |
| 2 | `debugging` | `message_class` is `fault`, **and** the turn either edits a file or reads/searches without editing |
| 3 | `coding` | The turn contains at least one edit-shaped tool call |
| 4 | `conversation` | Anything else, including a turn with no tool call at all |

Without the message text rule 2 cannot exist, which is the whole reason
Decision 2 permits reading it.

`debugging` still outranks `coding`, because a turn that fixes a bug also edits a
file and the reverse order would make `debugging` unreachable. What changed is
its entry condition: it is no longer "a fault word appears somewhere in the
message" but "a fault word is the message's earliest intent signal".

**Why the earlier form was wrong, stated so it is not reintroduced.** This
decision previously ran fixed precedence on presence alone, and separately told
implementers to resolve `coding`'s four subcategories by earliest match — citing
`add error handling`, which CodeBurn had persistently misclassified, as the
reason. That fix was correct and installed one level too low. Presence-only
precedence sends `add error handling` to `debugging` at the category level, and
a subcategory rule that only runs *inside* `coding` never gets to see it: the
turn edits a file, `error` matches the fault vocabulary, and the message never
reaches the rule written to save it. `tasks.md` task 2's acceptance requires that
message to classify as `feature`, which is a `coding` subcategory, so the two
halves of this document contradicted each other and the test would have failed
whichever half an implementer followed.

Lifting the same scan to the category level resolves it without an exception
table. In `add error handling`, `add` precedes `error`, so `message_class` is
`build` and the turn is `coding/feature`. In `fix the add button`, `fix`
precedes `add`, so it is `debugging`. Neither needs a phrase list, and a phrase
list is what this would otherwise become: `error handling`, `error message`,
`error type`, each added after it is reported.

### The identifiers are per-client

Decision 7 is per-client and this decision must be too; the discarded phrasing
named only Claude's tools and left Codex undefined. The vocabulary below was
verified by reading twelve Codex sessions and this repository's Claude logs, not
inferred.

| Signal | Codex | Claude |
| --- | --- | --- |
| Subagent spawn | `spawn_agent` | `Task`, `Agent` |
| Skill or workflow | **none — Codex has no skill tool** | `Skill`, `Workflow` |
| Plan or to-do | `update_plan` | plan mode, `TodoWrite` |
| Edit-shaped | `apply_patch`, or a command parsed out of `exec_command`/`exec` that writes | `Edit`, `Write`, `NotebookEdit` |
| Read-shaped | A parsed command that is read-shaped by the ported allowlist | `Read`, `Grep`, `Glob` |

`delegation` is therefore reachable on both clients — `spawn_agent` was observed
in real Codex sessions — and so is `conversation/planning`, through
`update_plan`. The four categories stay four on both clients, which is what
`ux/session-work-signals.md`'s "four rows, always four" requires.

**One subcategory is client-specific:** `delegation/workflow` has no Codex
signal. A subcategory with no signal for the selected client is **omitted from
the expanded list**, not rendered as a zero row. A permanently empty row invites
the reader to conclude the user never uses skills, when the truth is that the
measurement does not exist there. Both surfaces follow this rule, and a category
whose subcategories are all omitted still renders its own row.

No rule consults tool failure status. Codex records every output item with a
hardcoded `"completed"` while Claude sets `is_error`, so any failure-keyed rule
would classify all Codex work as non-debugging. This is a property of the source
logs, verified in `parseCodex` and `parseClaude`, not a stylistic choice.

### Subcategories

Every category has at least two. A category with one subcategory produces a
detail view with a single indented row under it, which reads as a rendering bug
rather than as a hierarchy.

| Category | Subcategory | Rule |
| --- | --- | --- |
| `coding` | `feature` | Message matches `add`, `create`, `implement`, `new`, `build`, `scaffold`, `generate` |
| | `refactoring` | Message matches `refactor`, `rename`, `clean up`, `simplify`, `extract`, `restructure`, `split`, `migrate` |
| | `testing` | A command string names a test runner, or the message matches the test vocabulary |
| | `maintenance` | A command string names `git`, a build, or a dependency install, and the turn matched no rule above |
| `debugging` | `investigation` | The turn read or searched and edited nothing |
| | `repair` | The turn edited at least one file |
| `conversation` | `exploration` | The turn read, searched, or called a read-shaped MCP tool |
| | `brainstorming` | The turn called no tool and the message matches `brainstorm`, `idea`, `what if`, `approach`, `should we`, `opinion`, `suggest` |
| | `planning` | The turn used plan mode or a to-do tool |
| `delegation` | `subagent` | A subagent spawn |
| | `workflow` | A skill or workflow invocation |

Within `coding`, the message-derived subcategory is `intent_sub` — already
decided by the single scan above, by earliest match position rather than by rule
order. `testing` and `maintenance` additionally have command-shaped rules, which
apply when `intent_sub` is empty; a message-derived `intent_sub` wins over them,
because the user's stated intent outranks an incidental command in the same turn.

A subcategory that matches nothing falls back to the category's first
subcategory, and that fallback is visible: `coding` with no keyword match reads
as `feature`. This is a definition, not a silent default, and the CLI's `--sub`
output shows it.

## Decision 4 — cost reaches a category through the turn, structurally

A tool call consumes no tokens. The turn that issued it does. So cost attaches to
the turn, and the turn carries it to whatever category it was classified as.

The attachment is **structural, not temporal**. `usage_events` rows are produced
by the same scan of the same log file that produces turns, so the parser knows
which turn boundary an event falls after and records `turn_index` on the event.
Matching events to turns by timestamp window afterwards was the earlier pass's
plan and is strictly worse: it re-derives from a weaker signal something the
parser already knows exactly.

A `cost_basis` discriminator travels with every cost figure:

| Value | Meaning |
| --- | --- |
| `turn` | Every event in scope carried a `turn_index`. The normal case |
| `partial` | Some events in scope predate the migration's backfill or came from a log the parser could not segment |
| `none` | No priced event covers the scope. The surface renders unavailable, not zero |

**The Tooling module carries no cost.** Splitting a turn's cost across the tool
calls inside it is an apportionment with no defensible divisor, and printing it
next to `bash` would present that apportionment as a measurement. Tooling reports
calls and share of calls. This is the user's decision of 2026-08-20 and it
changes the prototype, which now shows a percentage in that column.

## Decision 5 — the session-level category

`session show --activity` prints one `SIGNALS` line for one session, and that
line names a category. A session has many turns and therefore many categories, so
the reduction must be defined or two implementers will invent two different ones.

**A session's category is the category holding the largest share of the
session's attributed cost. Ties break toward the larger turn count, then toward
the category that appears first in Decision 3's precedence order.**

Cost is the divisor rather than turn count because a session of twenty
one-sentence conversation turns and three long coding turns is a coding session,
and counting turns would call it a conversation.

When a session's `cost_basis` is `none`, the line omits the category and prints
the three counted values alone. Those three — tool calls, distinct files, first
edit — aggregate unambiguously and need no rule.

## Decision 6 — the five workflow metrics

| Metric | Definition | Unavailable when |
| --- | --- | --- |
| `first_edit_seconds` | Median across sessions in scope of the seconds from the session's first turn to its first edit-shaped tool call | No session in scope reached an edit |
| `files_touched` | Count of distinct `path_digest` values in `usage_tool_files` rows with `wrote = 1` in scope | No written file in scope |
| `retries` | The rework counter below | No session in scope reached an edit |
| `edits_per_session` | `usage_tool_files` rows with `wrote = 1` in scope ÷ sessions in scope | No session in scope |
| `top_file` | The `base_name` of the most frequent `path_digest` among `wrote = 1` rows, with its edit count | No written file in scope |

All four file-derived metrics read `usage_tool_files`, not the parent call. A
call that both reads and writes contributes only its written rows here, which is
why Decision 8 carries `wrote` per file rather than a single kind per call: a
Codex `exec` that greps two files and patches a third must count as one file
touched, not three.

**Rework** replaces the prototype's earlier "iteration depth". One rework is
counted when, within a session, a file is edited, then a **non-read-shaped**
shell command runs, then the same file is edited again. The algorithm and the read-shaped exclusion are
CodeBurn's `countRetries` — `edit → rg → edit` is research, not rework, and
counting it penalized shell-first workflows — but the **window differs**:
CodeBurn counts within one turn, and this decision counts within a session, so a
fix that spans two turns is counted here and is not counted there. The
denominator is deliberately wider; the identity is in the rule, not the scope.

The prototype's earlier metric was turns-per-edit, a ratio dominated by how much
the user chatted, which is not a property of the work. The user replaced it on
2026-08-20.

Read-shaped commands are recognized by the command string, which Decision 2
permits reading and forbids keeping.

## Decision 7 — tool kinds and MCP identity

Tool names map to a fixed per-client kind. The mapping is a table in code, not a
heuristic, so an unknown tool lands in `other` rather than being guessed into a
kind that changes a displayed share.

| Kind | Codex | Claude |
| --- | --- | --- |
| `edit` | `apply_patch`; or an `exec_command`/`exec` whose parsed commands include a write | `Edit`, `Write`, `NotebookEdit` |
| `read` | An `exec_command`/`exec` whose parsed commands are all read-shaped | `Read`, `Grep`, `Glob` |
| `bash` | Any other `exec_command`, `exec`, `js`, or `write_stdin` | `Bash` |
| `mcp` | A `custom_tool_call`/`function_call` **item** of type `mcp_tool_call` | Any tool name matching `mcp__<server>__<tool>` |
| `other` | `wait`, `wait_agent`, `spawn_agent`, `list_agents`, `interrupt_agent`, `send_message`, `followup_task`, `update_plan`, `view_image`, and anything unrecognized | everything else |

A Codex tool call is classified by the strongest kind among its parsed commands,
`edit` > `read` > `bash`, because the row is one call and one call can do
several things.

The `mcp` cell names an **item type**, not a tool name — `mcp_tool_call` is the
same class of thing as `function_call` and `custom_tool_call`, and it appears
zero times as a `name` in the corpus. Codex's tool-name vocabulary is the
thirteen names in Decision 2; MCP is recognized one level up, at the item type,
which is why this row reads differently from the others.

Codex's shell calls are split into `read`, `edit`, and `bash` by the command
parse of Decision 2, so its Tooling breakdown is comparable with Claude's.

`other` is counted in the total and, when non-empty, rendered as its own row.
Hiding it would make the visible shares sum to less than 100% with no
explanation on screen. On Codex `other` is routinely non-empty, because agent
orchestration and `update_plan` land there.

`mcp_server` is the `<server>` segment. Codex's `mcp_tool_call` carries the
server in its own field; Claude's is embedded in the tool name. The top MCP
server is the one with the most calls in scope, ties broken alphabetically so
the value is stable between runs.

## Decision 8 — the next schema migration

**This decision does not name a version number.** It was written on 2026-08-20
naming `v19`, and the first design review recorded "schema v19 is the correct
next version" as a checked fact so a later round would not re-derive it. That was
true when it was written and stopped being true when `switch-effectiveness-boundary`
landed `usage_session_observations` as `version: 19` and shipped it.

A migration number is not a design choice this topic gets to make. It is assigned
by landing order across every topic in the version, so any number written here is
a copy of repository state that goes stale the moment another topic commits
first — which is exactly what happened once already. The normative rule is
therefore the procedure, not the number: **task 1 reads `CurrentSchemaVersion`
and the last entry in `internal/store/migrations.go` at implementation time and
appends the next one.** Everything below that depends on the number is expressed
against `next` rather than against a literal.

As an observation that may already be stale by the time this is read:
`CurrentSchemaVersion` was `19` on 2026-08-27, which would make `next` v20. That
sentence is a dated reading of the repository, not an instruction, and a
disagreement between it and `migrations.go` is resolved in favour of
`migrations.go` without a correction round.

The historical review record keeps its v19 sentence unchanged — it states what
was true when that round ran, and rewriting it would erase the evidence that
landing order, not review error, moved the number.

Three columns on `usage_tool_calls`:

```
turn_index  INTEGER
tool_kind   TEXT NOT NULL DEFAULT 'other'
mcp_server  TEXT
```

One column on `usage_events`:

```
turn_index  INTEGER
```

**File identity lives in its own table, one row per file per call.** A single
Codex `exec` can wrap several commands touching several files — measured on one
session, 37 calls wrapped none, 73 wrapped one, and 17 wrapped two to four — so a
`path_digest` column on `usage_tool_calls` could represent only the first of
them, and `files_touched` would undercount by construction. Claude's one-file
tools are the degenerate case of the same shape.

```
CREATE TABLE usage_tool_files (
  activity_key TEXT NOT NULL REFERENCES usage_tool_calls(activity_key) ON DELETE CASCADE,
  path_digest  TEXT NOT NULL,
  base_name    TEXT NOT NULL,
  wrote        INTEGER NOT NULL,
  PRIMARY KEY (activity_key, path_digest)
);
CREATE INDEX usage_tool_files_digest ON usage_tool_files(path_digest);
```

**A call that both reads and writes one path is one row, and the write wins.**
The key allows a single row per call per file, so `wrote` is `1` if any extracted
command wrote the path, whatever else the same call did to it. Read-then-write of
one file is the ordinary edit workflow, and it also appears within a single
command — `go test … > LOG …; tail -80 LOG` writes and then reads one path — so a
last-one-seen rule would drop real edits out of `files_touched`, `top_file`, and
`edits_per_session`.

`wrote` separates a file that was edited from one that was only read, which
`files_touched` and `top_file` need and a single `tool_kind` on the parent row
cannot express once a call does both.

One new table for the classification:

```
CREATE TABLE usage_work_signals (
  client        TEXT NOT NULL,
  session_id    TEXT NOT NULL,
  turn_index    INTEGER NOT NULL,
  started_at    TEXT NOT NULL,
  activity_kind TEXT NOT NULL,
  activity_sub  TEXT NOT NULL,
  PRIMARY KEY (client, session_id, turn_index)
);
CREATE INDEX usage_work_signals_started ON usage_work_signals(started_at);
CREATE INDEX usage_work_signals_kind    ON usage_work_signals(activity_kind, started_at);
```

**`usage_work_signals` as written here is superseded by Decision 11.** Task 1
created it in this shape and never wrote a row to it; task 2 replaces it with a
form that carries the incremental classifier state, source ownership, and the
reset path. The shape above is what the delivered migration contains, and it is
left stated rather than edited so a reader of the committed schema finds the
decision that produced it.

`usage_source_files.parser_version` is bumped so indexed sources are re-scanned
and all four tables backfill, following the `v0.4.1` cache-write precedent. The
backfill is what makes the 93 `apply_patch`-era Codex sessions visible; a
database with no prior tool-call rows backfills from the source logs like any
first scan, and that path is tested, not assumed.

**Raising the version invalidates the two canonical desktop fixtures.** They
embed the schema migration count as a doctor check — the `schema` entry in
`desktop/fixtures/v1/snapshot-complete.json` and `snapshot-empty-client.json`
carries the count — and `TestCanonicalFixturesAreReproducibleProducerOutput`
compares the producer's output against them byte for byte, so the whole Go suite
fails until they are regenerated. `switch-effectiveness-boundary` hit exactly
this as a P1 finding when it raised the count, and closed it by regenerating both
through the official producer rather than hand-editing them; the resulting diff
was one count replacement per file and nothing else. Task 1 regenerates them the
same way and asserts the same shape of diff — each file's current count replaced
by `next`, no other change. This is a known consequence of the migration, not a
discovery to be made during review.

## Decision 9 — the wire projection

Three additive families under `sessions.work_signals`, producer-computed and
producer-bounded across the `Client` × `Period` product. `wire_version` does not
change; the host guards it with an exact equality check, so raising it would make
an older host reject the whole snapshot rather than degrade to its shipped
uncaptured rendering.

**Each family is a keyed item list, not a single object.** The panel's filters
change client and period, and a family that carries one unkeyed set cannot answer
two different filter positions — the modules would show the same numbers on
`today` and `30d`. The shape follows the pattern the panel's filters already
read, `SessionsPeriods` / `SessionsPeriodItem`
(`internal/desktop/desktop.go`), where every item carries its own
`period` and `client`.

```
sessions.work_signals.activity { available, items[] }
  items[]: period, client, cost_basis, kinds[4]
    kinds[]: kind, share, cost, events, sub[]
      sub[]: kind, share, cost, events

sessions.work_signals.workflow { available, items[] }
  items[]: period, client, first_edit_seconds, files_touched, retries,
           edits_per_session, top_file, top_file_edits

sessions.work_signals.tooling { available, items[] }
  items[]: period, client, calls, groups, rows[], top_mcp_server, top_mcp_calls
    rows[]: kind, calls, share
```

`available` stays on the family, not the item: it answers "does this host's
payload carry this family at all", which is the question the uncaptured rendering
turns on. An item that exists but has nothing to report carries `cost_basis:
"none"` or null metric values; a client/period position with no data may be
omitted from `items` entirely, and the host renders a missing position exactly as
it renders an unavailable one.

`groups` is **the number of distinct `tool_kind` values present in `rows` for
that item**, `other` included when it is present. It is not a second bound and
not a category count: with the vocabulary fixed at five, `groups` ranges 0–5 and
tells the reader how many kinds the scope actually exercised.

`rows` carries **one entry per non-empty tool kind, and is not truncated**.
Decision 7 fixes the vocabulary at exactly five, so the list is bounded by the
vocabulary rather than by a cap, and both surfaces print every entry.

The CLI's `--format json` uses **the same field names and units** as this
projection, with one structural difference that is a consequence of the two
readers rather than a disagreement: the CLI renders one scope per invocation, so
it emits the item's fields at the top level under `period` and `client` keys
instead of an `items` array.

## Decision 10 — the two surfaces are independent

Neither surface is derived from the other. Both read the same derivation
directly. The CLI depends on the classifier, not on the projection, and can
therefore be built before the GUI — which is the order that makes the numbers
checkable while the panel work is still in flight.

The three periods both surfaces share — `today`, `7d`, `30d` — carry a
reproducibility guarantee: the same figure, for the same client and period, on
both. The CLI accepts periods the panel has no control for; those carry no
cross-surface guarantee because there is nothing to compare them to.

## Decision 11 — the classifier survives an incremental scan

A source log is scanned incrementally: the parser resumes at a stored offset and
sees only what was appended. Decision 1 already handles a turn boundary that
straddles two scans, by persisting the pending-user state. Classification needs
more than the boundary, and the difference is what this decision exists for.

**The problem.** A turn's category is a function of two things that arrive at
different times: the user message, which opens the turn, and the tool calls that
follow it. When a scan ends between them, the message has been read and dropped —
Decision 2 forbids keeping it — and the next scan has no way to recover it. The
turn would be classified from tool shape alone, silently losing every
message-derived rule: `debugging` becomes unreachable across a scan boundary,
and `coding` always falls back to `feature`.

**The reduction, not the message, is what persists.** When the parser sees a
turn-opening message it performs Decision 3's single scan immediately and stores
the verdict: `message_class` and `intent_sub`, fifteen values between them. That
is the same reduction Decision 2 permits for `activity_kind` — a classifier input
leaving the process as a bounded label — applied one step earlier because the
classification cannot be completed yet.

Storing a position, a matched word, or a truncated message would each be a weaker
form of the same idea and a worse one: all three carry message content, and
`message_class` carries only its decision.

**This is not the pending state task 1 already stores.** Task 1 persists a
pending-user marker on `usage_source_files`, and it belongs to the parser: it is
part of the resume cursor, answers "was the last thing I read an unconsumed turn
boundary", and is scoped to a source file. Decision 11's pending row belongs to
the classifier, answers "which turn is awaiting its tool shape", and is scoped to
a turn. They are set by the same scan and cleared by different events — the
parser's marker clears when the assistant entry advances `turn_index`, the
classifier's row clears when it is recomputed as `classified` — so neither can be
derived from the other, and collapsing them would tie the resume cursor to
classification state that a re-scan legitimately rebuilds.

**Tool behavior is not carried in memory.** It does not need to be. Tool calls
are already persisted with their `turn_index` by task 1, so the classifier reads
a turn's tool shape back from `usage_tool_calls` rather than accumulating it
across scans. Only the message reduction has no other home.

### The row's states

| `state` | Meaning | Visible to Decisions 4, 5, 6 |
| --- | --- | --- |
| `pending` | The message was seen; no assistant API call has followed it yet | **No** |
| `classified` | At least one assistant call has arrived; `activity_kind` and `activity_sub` are computed | Yes |

`pending` is invisible to every aggregate on purpose, and it is the same rule as
Decision 1's "no assistant API call, no turn" seen from the storage side. A
pending row is a turn that does not exist yet. Counting it would inflate turn
counts, and classifying it would have to guess the tool shape that has not
happened.

### Which `turn_index` a pending row carries

The primary key needs an index at the moment the message arrives, and the two
clients advance `turn_index` at different moments, so the rule is per-client. It
is stated here because task 1's parser makes one of the two non-obvious.

| Client | When `turn_index` advances | Index a pending row carries |
| --- | --- | --- |
| Codex | On a changed `turn_id`, which arrives in `turn_context` **before** the message | The current index — it is already this turn's |
| Claude | On the first assistant entry **after** the message | The **next** index, the one this turn will commit to when that assistant entry arrives |

Claude's row is therefore written against an index that does not exist yet, and
promotion is what makes it exist: when the assistant entry advances the counter,
the pending row is already keyed to the value it lands on, so it is recomputed as
`classified` in place rather than moved.

Three consequences follow, and each is a case an implementer would otherwise
have to invent:

- **Consecutive user messages replace, they do not accumulate.** Several user
  entries with no assistant between them all target the same next index, so each
  overwrites the previous pending row. This is the storage-side reading of
  Decision 1: those entries have produced no turn yet, and the one that finally
  draws an assistant reply is the one whose intent the turn carries. The last
  message before the assistant wins.
- **Promotion is an update, never an insert plus delete.** The key is unchanged
  across it, so no window exists in which the turn is absent or duplicated.
- **A session reset clears pending rows with everything else.** A pending row
  keyed to an index that a re-scan will re-derive must not survive that re-scan;
  it is deleted by `source_path` along with the classified rows and rebuilt.

Replay is idempotent under all three: re-scanning unchanged content re-derives
the same index, the same reduction, and the same row.

A pending row's `started_at` is the timestamp of the entry that opened the turn.
When that entry carries none — Codex `user_message` events do, Claude user
entries do, but a malformed line may not — no pending row is written, and the
turn is recorded directly as `classified` when its first assistant call arrives,
carrying that call's timestamp. A row is never written with a synthesized time,
because `started_at` is what `first_edit_seconds` measures from.

`classified` is a conclusion, not a seal. A turn that gains more tool calls in a
later scan is recomputed from the same inputs and re-upserted; the computation is
a pure function of the message reduction and the turn's rows in
`usage_tool_calls`, so recomputation is idempotent and needs no ordering
guarantee. A turn stops changing when the next turn boundary passes it, but
nothing has to record that moment.

### Ownership and reset

`usage_work_signals` gains `source_path`, which the other two usage tables have
carried since before this topic. Without it the table cannot participate in the
reset path that already exists for events and tool calls, and a rewritten or
truncated log leaves rows behind that nothing will ever delete or correct.

- **Reset** — when a source's identity changes, its rows are deleted by
  `source_path` and rebuilt from the re-scan, exactly as `usage_events` and
  `usage_tool_calls` are.
- **Ownership on conflict — signals do not get their own policy.** The primary
  key stays `(client, session_id, turn_index)`, because one turn is one row and a
  composite including `source_path` would silently split a turn in two. When a
  second source claims a key, **the winner is whichever `source_path` sorts
  last, and an existing owner yields only while it is still an indexed source.**
  `upsertTx` and `upsertToolActivityTx` already decide duplicate events and tool
  calls this way, and `TestUsageToolActivityFollowsDuplicateSourceOwnership`
  pins it: a session under `.codex/sessions/…` keeps ownership when a copy
  appears under `.codex/archived_sessions/…`, and the archive takes over only
  once the live path is gone.

  **Read the winner off that example, not off the adjective.** The direction is
  easy to state backwards — the first repair of this finding did exactly that,
  asserting the opposite winner in the same sentence that told implementers to
  match the delivered code — so the example is the definition here and the
  wording is a summary of it. If the delivered comparison ever changes, this
  paragraph is wrong and the code is right; signals follow events and tool calls
  rather than restating their rule independently.

  Scan order must not decide this. A last-writer rule looks equivalent and is
  not: scanning a live log and then an archived copy of it hands the row to the
  archive, after which removing the archive deletes a turn whose live source was
  never re-scanned, and a later archive scan can overwrite a newer live
  classification with stale content. Making the winner a function of the paths
  themselves removes both.

  **The binding requirement is agreement, not the direction itself.** Signals,
  events, and tool calls must name the same owner for the same conflict, and
  that is what task 2 asserts — three tables disagreeing about which source owns
  a session is worse than any one of them being wrong, and it is invisible from
  inside any single table. A test that compares the three against each other
  catches a reversed direction; a paragraph describing the direction does not,
  which is how this finding survived its first repair.
- **Orphans** — a row belonging to a source whose content no longer produces it,
  which is what a rewritten or truncated log leaves behind. The reset path
  removes them as a consequence of deleting by `source_path`; no separate sweep
  is needed, and none should be added.

  **A row is not an orphan merely because no `usage_tool_calls` row shares its
  `turn_index`.** A `conversation` turn legitimately calls no tool at all, and
  `conversation` is the category Decision 3 assigns to "anything else, including
  a turn with no tool call". A sweep keyed on tool-row absence would delete every
  chat-only turn in the store, plus every pending row whose tool calls have not
  arrived yet — it would delete precisely the rows that are correct.

### Schema

Task 1 created `usage_work_signals` and never wrote to it, so task 2 replaces the
table rather than altering it column by column. This is not a data migration —
there is no data — and the resulting statement is one a later reader can check
against this decision in one pass:

```
DROP TABLE usage_work_signals;
CREATE TABLE usage_work_signals (
  client        TEXT NOT NULL,
  session_id    TEXT NOT NULL,
  turn_index    INTEGER NOT NULL,
  started_at    TEXT NOT NULL,
  state         TEXT NOT NULL,
  message_class TEXT NOT NULL,
  intent_sub    TEXT NOT NULL DEFAULT '',
  activity_kind TEXT NOT NULL DEFAULT '',
  activity_sub  TEXT NOT NULL DEFAULT '',
  source_path   TEXT NOT NULL,
  PRIMARY KEY (client, session_id, turn_index)
);
CREATE INDEX usage_work_signals_started ON usage_work_signals(started_at);
CREATE INDEX usage_work_signals_kind    ON usage_work_signals(activity_kind, started_at);
CREATE INDEX usage_work_signals_source  ON usage_work_signals(source_path);
```

`activity_kind` and `activity_sub` are empty while `state` is `pending`, which is
why they carry a default rather than the `NOT NULL` with no default that
Decision 8's version used. The `source_path` index exists for the reset path,
which deletes by that column on every source identity change.

The migration number is read from `migrations.go` at implementation time, for the
reason Decision 8 gives.

**And it carries Decision 8's consequence with it.** Raising the migration count
invalidates the two canonical desktop fixtures, which embed it as a doctor check
and are compared byte for byte by
`TestCanonicalFixturesAreReproducibleProducerOutput`. Task 2 regenerates
`desktop/fixtures/v1/snapshot-complete.json` and `snapshot-empty-client.json`
through the official producer, never by hand, and the whole diff is each file's
current count replaced by the new one — no other change in either file.

This is repeated here rather than left as a cross-reference because task 2 cites
Decision 11 and not Decision 8, and an implementer reading only what a task cites
is the reader this document has to serve. `switch-effectiveness-boundary` hit
this as a P1, and task 1 was amended after the fact to prevent it; a second
rediscovery would mean the amendment only ever protected the task that made it.
