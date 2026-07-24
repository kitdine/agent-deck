---
status: historical
created: 2026-07-24
retired: 2026-07-24
---

# Repository test-gap production fixes

## Goal

Correct the four production defects exposed by the active repository-wide
test-gap workflow, deliver the independently reviewed fixes to local `main`,
and establish the clean new baseline required to resume the four blocked test
tasks.

This plan is a separate production workflow. It does not modify or merge the
test-only audit branch, and it does not deliver the old-baseline staged test
candidates. Those candidates remain reproducible blocker evidence until they
are reconstructed and freshly reviewed under the later `new-baseline` resume.

## Baseline and authorization

- Target: local `main`
- Baseline:
  `1b12db4ab702cfb3d86daeac3520911530f2b8a5`
- Branch:
  `fix/repository-test-gaps-production-20260724`
- Worktree:
  `/private/tmp/agent-deck-repository-test-gaps-20260724/production-fix`
- Authorized scope: the four production defects and this plan's documentation
- Commit shape: one design/documentation commit followed by four logical
  production commits; later review/archive documentation remains separate
- Delivery: independent review followed by
  `git merge --ff-only fix/repository-test-gaps-production-20260724` from the
  still-clean authorized local `main`
- Push, tag, release, pull request, deployment, force operations, and unrelated
  changes: not authorized

The existing `docs/specs/cli-design.md` contract is unchanged. These fixes
restore already-promised decimal validation, bounded retry behavior, pinned
commit validation, and failure atomicity, so this plan does not raise the spec
version unless implementation reveals a real public-contract change.

## Root causes

### Permanent price-catalog validation is retried

`internal/usage/price_update.go` marks every error returned by
`liteLLMCatalog` as retryable. This correctly retries a genuinely truncated
HTTP 200 response, but also retries complete malformed JSON and permanent
semantic validation such as a catalog with no accepted direct-provider rows.

### Non-decimal multiplier syntax reaches `big.Rat`

`internal/providermeta/metadata.go` passes the raw multiplier to
`big.Rat.SetString`. That parser accepts rational, exponent, and radix syntax
that is outside the documented non-negative finite decimal contract, including
`1/3`.

### The catalog generator trusts an invalid resolved revision

`tools/genprices/main.go` returns a bare JSON decoding error from
`latestCommit` and checks only that the returned SHA is non-empty. A revision
such as `main` therefore reaches the catalog URL before downstream generation
rejects it.

### Session transitions have split transaction and ownership boundaries

`internal/session/session.go` currently:

- inserts the synthetic ReplaceDocuments source before the replacement
  transaction;
- commits Exclude control, document deletion, and metadata deletion
  independently;
- deletes project/path documents by `(client, session_id)` without
  `source_path`, which crosses source ownership;
- clears the index before calling a Scan that commits each source separately;
- lets some source queries and rename work escape the transaction that applies
  the source update.

These boundaries explain the leaked synthetic source, partially committed
exclusion and rebuild state, and deletion of a lower-priority fallback source.

## Design

### 1. Classify catalog failures at their source

Preserve retries only for transport/read/close failures, HTTP 408/429/5xx, and
JSON that is provably truncated at the end of the received body. Treat complete
malformed JSON, size violations, invalid commit identities, and semantic
catalog validation as permanent.

The classification will be typed or sentinel-based at the decoding boundary;
the retry loop will not infer retryability from arbitrary error text. Existing
error wrapping and `errors.Is` behavior for context cancellation remain intact.
No catalog state is written until download, parsing, and semantic validation
all succeed.

### 2. Freeze one decimal lexical grammar

A blank or all-whitespace multiplier retains the existing default `"1"`.
Every non-empty value must match:

```text
^[0-9]+(?:\.[0-9]+)?$
```

This accepts integers and ordinary non-negative decimals and rejects signs,
rational syntax, exponent syntax, radix prefixes, `.5`, and `1.`. Valid input
continues through `big.Rat` for exact parsing and the existing 12-decimal
canonical representation. Invalid input continues to return
`ErrInvalidMultiplier`; provider-layer sentinel translation is unchanged.

Older development databases containing syntax that was previously accepted
outside the documented contract will fail closed during metadata migration
rather than silently canonicalizing an invalid value.

### 3. Validate every generator pin before catalog fetch

`latestCommit` will wrap fetch and JSON errors with
`resolve latest LiteLLM commit`, preserve the existing empty-SHA error, and use
the shared `usage.ValidateLiteLLMCommit` rule for non-empty values. A valid pin
is exactly 40 lowercase hexadecimal characters.

The explicit write-mode commit will also be validated before any catalog
request. Check mode continues to use and validate the artifact-recorded commit
without querying current main. Invalid input must make zero catalog requests
and leave the output byte-for-byte unchanged.

### 4. Use one executor contract for DB and Tx session work

Introduce a narrow package-private executor implemented by both `*sql.DB` and
`*sql.Tx`:

```go
type sessionExecutor interface {
    ExecContext(context.Context, string, ...any) (sql.Result, error)
    QueryContext(context.Context, string, ...any) (*sql.Rows, error)
    QueryRowContext(context.Context, string, ...any) *sql.Row
}
```

Separate transaction ownership from SQL execution. Executor helpers do not
open nested transactions:

- normal incremental Scan retains one short transaction per changed source;
- source load, rename, delete, source-state save, exclusion lookup, and insert
  use that same source transaction;
- ReplaceDocuments inserts its synthetic source and replaces content in one
  transaction;
- Exclude inserts the control row and deletes matching documents and metadata
  in one transaction;
- Rebuild opens one outer transaction, clears rebuildable rows, performs the
  complete scan and missing-source cleanup through that transaction, and
  commits only after every source succeeds.

Project deletion uses the complete
`(source_path, client, session_id)` ownership key. Path deletion uses the exact
source path. Session and client exclusions preserve their current cross-source
semantics. FTS5 documents remain explicitly deleted because the virtual table
has no foreign-key cascade.

`excluded` will return `(bool, error)` so a database failure cannot be silently
treated as an exclusion.

The explicit Rebuild transaction may hold the SQLite writer lock longer than a
normal Scan. That is an accepted bounded trade-off for all-or-nothing rebuild
semantics; normal scanning keeps its existing short-transaction behavior.

## Rejected alternatives

### Build a temporary session database and replace the live file

This avoids a long transaction but requires a new Store handoff API plus safe
WAL checkpointing, sidecar handling, open-connection replacement, permission
and directory durability handling, and exclusion copying. It is not a bounded
repair.

### Restore a snapshot after a failed rebuild

This is compensation rather than atomicity. A crash or cancellation between
partial commits and restore exposes broken state, and restore can itself fail
or hide the original error. It does not meet the existing preservation
contract.

## Tasks

| Task | Production paths | Dev | Review |
| --- | --- | --- | --- |
| Bound permanent catalog failures | `internal/usage/price_update.go` and directly required parsing helpers | ✅ | ✅ |
| Reject non-decimal multipliers | `internal/providermeta/metadata.go` | ✅ | ✅ |
| Validate generator commit resolution | `tools/genprices/main.go` | ✅ | ✅ |
| Make session transitions atomic and exact | `internal/session/session.go` | ✅ | ✅ |

The planned production commit titles are:

```text
fix(usage): stop retrying permanent catalog validation failures
fix(providermeta): reject non-decimal multiplier syntax
fix(genprices): validate resolved commit before catalog fetch
fix(session): make index transitions atomic and exclusions exact
```

Each candidate receives a fresh read-only review before commit. Findings are
repaired only within that task's bounded scope and re-reviewed by a fresh
reviewer. No production commit contains the old-baseline test candidate files.

## RED/GREEN and verification

The four old task worktrees and their staged manifests remain the authoritative
RED evidence:

| Task | Test manifest |
| --- | --- |
| Usage | `0a5ce2e960c348c48820bec0040e38337c9087c048efcbf297c51717ed1e0965` |
| Providermeta | `60c9a14b62adf9145a5cb1bdc0f22560691a754d3dd636b81b4934aa0377ae42` |
| Genprices | `2f2e9c4cff9cbec5678d8ebbc69a606e024e8ae2fae49638c8e0a52b339266d7` |
| Session | `cce7277dfd7006e3d3eb366360d7516822dedbc92488398d31574889f3262d0b` |

For development verification, reconstruct those exact test candidates
temporarily in the authorized production-fix worktree, prove RED before the
relevant production edit and GREEN afterward, and exclude all candidate test
paths from the production commits. Preserve their manifest identity and remove
only the temporary copies created by this workflow before final delivery.

Targeted verification covers:

- permanent, malformed, truncated, transient, cancellation, and unchanged
  price-catalog failure boundaries;
- decimal lexical boundaries plus provider and v8 metadata-migration behavior;
- resolved, explicit, and recorded generator commits with exact request counts
  and unchanged output;
- ReplaceDocuments, Exclude, Rebuild, rename, missing-source, fallback-source,
  table snapshot, List, Search, and FTS rollback behavior.

After the final production edit, run the L3 evidence set:

```text
rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go test -mod=vendor -count=1 ./...
rtk test env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go test -mod=vendor -race -count=1 ./internal/session ./internal/usage ./internal/providermeta ./tools/genprices
rtk lint env GOCACHE=/private/tmp/agent-deck-go-build GOMODCACHE=/private/tmp/agent-deck-go-mod go vet -mod=vendor ./...
rtk git diff --check
```

L4 `make release-verify` is not required because this workflow does not change
release artifacts, installers, distribution, dependencies, or deployment.

## Delivery and recovery

After four task reviews PASS and the selected verification is bound to the
unchanged production content:

1. tick the final plan state, create and independently review the required
   review records, move the completed plan and reviews to `docs/archive/`, and
   update the documentation indexes in one final documentation commit on the
   production-fix branch;
2. verify signatures, messages, intended path sets, hooks, and a clean
   production-fix worktree;
3. prove local `main` still equals the authorized baseline and is clean;
4. fast-forward local `main` once to the reviewed production-fix tip;
5. verify the target tree, history, archived records, status, and
   `Push: not performed`.

Any changed target baseline, unexpected path, failed review, failed signature,
hook rewrite, or verification failure stops delivery and preserves the branch
for repair. No merge commit, rebase, force update, or push is permitted.

## New-baseline resume

Once the production fixes are delivered to local `main`, the test-gap workflow
must resume in `new-baseline` mode:

1. record the production-fix commit and new clean `main` baseline;
2. rerun broad tests, race, vet, and atomic coverage at that exact state;
3. generate and obtain approval for a new authorization package with exact new
   branches and worktrees;
4. reconstruct the four old test candidates from their reviewed evidence
   rather than cherry-picking or rebasing old-baseline task or audit commits;
5. obtain fresh Writer evidence, task reviews, audit integration and aggregate
   review;
6. build a new final-state delivery branch from the new baseline;
7. retire the original active test-gap plan only after all four modules are
   adequately protected and the complete retirement gate passes.

The current approval does not authorize those future new-baseline branches,
worktrees, target delivery, plan retirement, cleanup, or push.

## Completion evidence

The four production candidates passed independent read-only review before their
signed commits:

| Task | Commit | Reviewed manifest |
| --- | --- | --- |
| Usage retry classification | `571a0e3ba454e9789c0dae3932dc2e296bb684d8` | `51a32e0aa1db01d98653f53d6767080b409cd3b3e9b2b7b408d6273cf4837c8d` |
| Multiplier decimal grammar | `e934f0042de5d7c7eeb945727b4fd655675d6efd` | `5e874019fb01083b7d65485ac179962a19f2bc32114f7876fd51bf1ef64b38ed` |
| Generator commit resolution | `c4abf8700757c5429b6c24d139b077dde01a0183` | `4480337d6f2b229d7464f49dd01ff7e4a6a4a52b5ed3bb55fe80cff8b366c877` |
| Session transition atomicity | `3c80e4a9ad025375d337a7ef8f9cda065bc797f5` | `ef203c258b800da65b2b32a55afc3ccd8988e1c4441d84b0087633eedd04b98c` |

The usage implementation differs narrowly from the proposed typed-or-sentinel
classification. It first requires `*json.SyntaxError` through `errors.As`, then
recognizes truncation by exact equality with Go's
`unexpected end of JSON input` text. The independent task review accepted this
bounded check because complete malformed JSON and semantic failures remain
non-retryable; the residual compatibility risk is dependence on that standard
library error text.

After the last production edit, the authorized L3 gate passed:

```text
go test -mod=vendor -count=1 ./...
go test -mod=vendor -race -count=1 ./internal/session ./internal/usage ./internal/providermeta ./tools/genprices
go vet -mod=vendor ./...
git diff --check
```

The four blocker-test manifests were reverified before their temporary copies
were removed. The production-fix worktree was clean afterward. Push, tag,
release, deployment, and creation of the later `new-baseline` test-gap branches
were not performed.

## Starting a task

Select one row from the Tasks table. Read `AGENTS.md`, this plan, the named
production file, the matching blocker section on
`audit/repository-test-gaps-20260723`, and the matching old staged test
candidate. Reconfirm its manifest and RED result, implement only the approved
root-cause repair, run focused GREEN and affected-package verification, stage
only the production paths, and request a fresh read-only review. Tick `Dev`
only after targeted verification passes and `Review` only after a recorded
PASS.
