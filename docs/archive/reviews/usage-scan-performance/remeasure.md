---
status: historical
plan: usage-scan-performance
task: remeasure
retired: 2026-07-22
---

# Review log — usage-scan-performance / remeasure

## Round 1 — 2026-07-22

- Reviewed state: current uncommitted documentation change in
  `docs/plans/usage-scan-performance.md`. No task-scoped production-code change
  was claimed for remeasurement. Other concurrent worktree changes were
  excluded from the review findings.
- Reviewer: Codex
- Scope: fixture comparability, cold-state reset, timing boundary, sample
  treatment, source/binary identity, arithmetic, limitations, claim strength,
  and auditability of the recorded performance result.
- Overall score: 6.5/10; the absolute current samples are useful, but the
  comparative claim is not yet controlled or reproducible.

### Findings

1. **[P2] The recorded `5.4-6x` improvement is not a same-fixture comparison.**
   The task requires remeasurement against the same fixture, while the current
   section compares the original 471-file, 622 MB, 221,661-line input with a
   later 479-file, 610 MB, 227,765-line copy. The limitation is disclosed, but
   the prose still presents `108.7 s` versus `18.5-20.0 s` as a direct
   improvement factor and attributes part of the difference to the current
   fixture being smaller. A 12 MB byte difference alone does not establish the
   cause of the much larger timing change, and undocumented cache state further
   weakens that attribution. Freeze one current source copy and run both the
   pre-optimization baseline binary and the current binary against that exact
   same immutable fixture, with a fresh state directory for every sample.
   Alternate execution order (for example AB, BA, AB) so cache warming and
   background load do not systematically favor one binary. Report the direct
   A/B factor from those paired inputs; keep the original 108.7 s result only
   as historical context unless its original bytes become available.

2. **[P2] The samples are not bound to an auditable code and runtime identity.**
   The document says the binary came from "the current working tree", but the
   tree contains multiple concurrent uncommitted tasks and no source-state
   digest, baseline revision, current diff digest, binary checksum, Go version,
   exact shell/time implementation, or raw timing output is recorded. A later
   reviewer cannot prove which content produced the numbers or reproduce the
   build. Record the baseline revision, current `HEAD` plus a digest of the
   relevant tracked diff, SHA-256 for both binaries, `go version`, machine/OS
   identity at a non-sensitive level, exact build and measurement commands,
   fixture aggregate counts plus a content digest, and the raw wall-time line
   for every sample. Do not commit source paths, session identifiers, source
   contents, or any real usage data.

### Positive observations

- The three current samples and arithmetic are internally consistent: the
  reported mean rounds to 20.0 seconds and the historical ratio range is
  numerically correct for those values.
- The document explicitly discloses fixture drift, uncontrolled page cache,
  single-machine sampling, and the decision not to remeasure warm stats paths.
- Resetting `$FAKE_HOME/.agentdeck` before every run correctly forces a fresh
  state database and cold logical scan.
- Progress stderr is deliberately included in the production command timing
  and correctly identified as expected behavior rather than suppressed to make
  the benchmark look faster.
- No implementation change was made merely to improve the measurement result.

### Evidence and residual uncertainty

- Documentation and diff inspection only; the reviewer did not access, copy,
  hash, or execute against the user's real Codex or Claude session sources.
- The sample arithmetic was checked from the recorded values.
- `rtk git diff --check` — pass before this review record was added.
- No product test suite was run: this task changed measurement documentation,
  and the verdict is already reopened on evidence quality rather than product
  behavior. If the repair remains documentation-only, fresh measurement/build
  evidence plus the L0 diff check is the appropriate gate.
- Verdict: **REOPEN**. Leave the plan's `Review` cell unchecked until both
  findings are closed and independently re-reviewed.

### Scoped repair instruction

> **根据评审修改：usage-scan 性能与进度 / remeasure**
>
> Read `AGENTS.md`, the `remeasure` task and baseline sections in
> `docs/plans/usage-scan-performance.md`, and Round 1 in this review record.
> Freeze one current isolated real-source fixture and calculate only a
> privacy-safe aggregate plus content digest; do not record paths, session IDs,
> contents, or real usage rows. Build a pre-optimization baseline binary from
> an explicit repository revision in a temporary copy without changing the
> current worktree, and build the current binary from an explicitly identified
> content state. Record baseline/current source identities, relevant diff
> digest, binary SHA-256 values, `go version`, non-sensitive OS/machine details,
> and exact commands. Run both binaries against the same frozen fixture with a
> fresh AgentDeck state for every run, alternating order such as AB/BA/AB, and
> preserve each raw wall-time result. Update the plan with direct A/B min/mean
> results and limitations; retain 108.7 s only as historical context unless the
> original bytes are recovered, and remove unsupported causal attribution to
> the later fixture's size. Do not modify production code unless measurement
> exposes a separately reviewed defect; do not include real source data, do not
> tick `Review`, and do not commit automatically. If only documentation changes,
> run `rtk git diff --check`, then request
> `进入复评并生成后续指令：usage-scan 性能与进度 / remeasure`.

## Round 3 — 2026-07-22 (re-review)

- Reviewed state: current documentation-only repair in
  `docs/plans/usage-scan-performance.md`; no production-code change was made
  for this task.
- Reviewer: Codex
- Scope: close Round 2's timer/raw-output evidence gap, verify the digest
  wording correction, recalculate all summary values, and complete the final
  plan gate.
- Round 2 finding: **Closed.** The document now records the exact
  `/usr/bin/time -p` command, including the `sh -c` argument mapping and child
  stdout/stderr redirection. Because the earlier timer output was not retained,
  both unchanged binaries were rerun against the same still-frozen fixture in
  AB/BA/AB order rather than reconstructing old evidence. Six genuine
  `real/user/sys` blocks are recorded, and their `real` values exactly match the
  summary table. Recalculation confirms baseline min 96.63 s / mean 105.213 s,
  current min 17.73 s / mean 19.5 s, min-to-min 5.45x, and mean-to-mean 5.40x.
- Round 2 nit: **Closed.** The fixture hash is accurately described as a
  path-insensitive content-multiset digest, with path identity attributed to
  reuse of the same frozen directory tree. No real paths or path-derived hashes
  were added.
- New findings: none.
- Evidence: direct inspection of the exact timing command and six raw output
  blocks; independent arithmetic check of all min/mean/factor values; and
  `rtk git diff --check` before the final lifecycle update.
- Verdict: **PASS**. All findings are closed. This is the plan's final review
  gate, so the plan and its review directory retire together after status sync.

## Round 2 — 2026-07-22 (re-review)

- Reviewed state: current uncommitted documentation repair in
  `docs/plans/usage-scan-performance.md`. No production-code change was made
  for this task.
- Reviewer: Codex
- Scope: verify both Round 1 findings, independently check the recorded source
  identity where possible without accessing real session data, recalculate the
  reported statistics, and look for new documentation or evidence defects.

### Round 1 findings — verification

1. **Closed.** The repair freezes one fixture and runs the pre-optimization
   baseline and current binaries against the same physical copy with fresh
   AgentDeck state for every sample. AB/BA/AB order gives both binaries each
   paired ordinal position. The direct comparison now uses only those paired
   samples: baseline min 106.547 s / mean 110.348 s, current min 20.211 s /
   mean 21.602 s, yielding 5.27x min-to-min and 5.11x mean-to-mean. The original
   108.7 s value is correctly retained as historical context rather than used
   in the factor.
2. **Partially closed.** Baseline revision, full current `HEAD`, full and
   binary-affecting diff digests, both binary SHA-256 values, Go version,
   OS/architecture, fixture aggregates and content digest, build commands, run
   order, and per-run wall values are now recorded. Independent review confirms
   current `HEAD` is `c6c54c4e211d96a9cb85e117cae9e85fec8fd2ae` and the raw diff digest for
   `cmd/agentdeck/main.go`, `internal/store/migrations.go`,
   `internal/store/store.go`, and `internal/usage/usage.go` is exactly
   `1edf347061e5b61e7fe07000566b9b8e3375ce9ba42914d576c427dd902bf0fb`,
   matching the record. However, the requested exact timing implementation and
   raw timing output are still absent: the method says only "timing wall clock
   around the process", and the table contains extracted numbers rather than
   the actual output lines. A reviewer cannot tell whether the measurement used
   the shell `time` keyword, `/usr/bin/time -p`, another binary, or a custom
   wrapper, nor verify how its output was parsed. Record the exact invocation
   and the six original timing-output lines. If those outputs were not retained,
   rerun the paired measurement rather than reconstructing or fabricating them.

### New finding

- **[nit] The fixture digest description overstates path sensitivity.** A
  SHA-256 over a sorted list of per-file content hashes identifies the multiset
  of file bytes and counts duplicates, but a pure rename or move leaves it
  unchanged. Reword it as a path-insensitive content-multiset digest and note
  that the same frozen directory tree, not the digest alone, guarantees path
  identity during this A/B. Do not add real paths or path-derived hashes to the
  repository.

### Evidence

- `rtk git rev-parse HEAD` —
  `c6c54c4e211d96a9cb85e117cae9e85fec8fd2ae`, matching the plan.
- `rtk proxy git diff -- cmd/agentdeck/main.go internal/store/migrations.go internal/store/store.go internal/usage/usage.go | rtk proxy shasum -a 256`
  — `1edf347061e5b61e7fe07000566b9b8e3375ce9ba42914d576c427dd902bf0fb`,
  matching the plan.
- The three-sample means and min/mean factors were recalculated from the six
  recorded values and agree with the rounded figures in the plan.
- `rtk git diff --check` — pass before this Round 2 record was appended.
- No real session source or benchmark binary was opened by the reviewer.
- Verdict: **REOPEN**. The controlled A/B conclusion is credible, but the
  audit trail remains incomplete until the timer identity and genuine raw
  timing output are recorded. Leave the plan's `Review` cell unchecked.

### Scoped repair instruction

> **根据评审修改：usage-scan 性能与进度 / remeasure**
>
> Read Round 2 in this review record. Preserve the controlled A/B design and
> existing results. Add the exact timing tool and complete command form used to
> wrap `env HOME=$FAKE_HOME <binary> usage scan`, including output redirection,
> and include the six genuine raw wall-time output lines. If the raw outputs
> were not retained, rerun both binaries against the same still-frozen fixture
> with fresh state and AB/BA/AB order; do not reconstruct or invent raw output.
> Recalculate the table only if rerun values differ. Reword the fixture hash as
> a path-insensitive content-multiset digest and state that frozen directory
> reuse, not the digest alone, preserves path layout. Do not record paths,
> session IDs, source content, or other real usage data; do not modify
> production code, tick `Review`, or commit automatically. Run
> `rtk git diff --check`, then request
> `进入复评并生成后续指令：usage-scan 性能与进度 / remeasure`.
