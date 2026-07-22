---
status: historical
plan: usage-stats-readability
task: remeasure
retired: 2026-07-22
---

# Review log — usage-stats-readability / remeasure

## Round 1 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `908a71bfe0c3e8b1caea2491c3187c060e2d757d1e85c333f6d21c43b7aac58f`
  over the relevant diff in `docs/plans/usage-stats-readability.md` and
  `docs/README.md` before this review record was added.
- Reviewer: Codex
- Scope: reproducibility and comparability of the two required line-count
  measurements, support for the documented before/after percentages and
  causal explanations, isolation from real session scanning, and task-scope
  discipline. Previously reviewed renderer behavior and unrelated worktree
  changes were excluded.
- Findings:
  - [P1] The recorded historical baseline and remeasurement do not use the
    same database content, so the document cannot attribute their difference
    to tasks 1-4 as a controlled before/after result. The baseline used the
    database when it was 48.5 MB and had 709 hourly trend buckets; the new run
    used the same live database path only after it had grown to 49.5 MB and
    now reports 720 buckets (`48` retained plus `672` omitted). The document
    discloses that drift, but still labels the comparison `Change` and states
    that each reduction “comes from” or “is dominated by” particular fixes.
    Those causal claims are not established when both the input data and the
    renderer changed. Make the comparison like-for-like by running the
    pre-readability renderer and current renderer against one immutable
    scratch database copy with identical `NO_COLOR=1`, `COLUMNS=100`,
    `--state-dir`, `--no-scan`, period, and grouping arguments; record the
    snapshot identity, exact line-count commands/results, and both binaries'
    source revisions. If a valid pre-change binary cannot be produced, keep
    the current 120/142 values only as current observations, relabel the old
    numbers as a non-comparable historical baseline, remove causal percentage
    claims, and state the residual limitation explicitly.
- Evidence: read-only inspection of the measured baseline, remeasurement
  section, completion note, task requirement, and current relevant diff. The
  arithmetic (`140` to `120` = about `-14%`; `822` to `142` = about `-83%`)
  is correct, the commands use `--no-scan` against an isolated copy, and no
  task-specific production-code change was introduced. `git diff --check`
  was clean before this review record was added. The real database and real
  session sources were not accessed during review.
- Verdict: REOPEN

## Round 2 — 2026-07-22

- Reviewed state: uncommitted worktree; SHA-256
  `184cec5fb75d0794aacb21ce827afaab1368f1ed5f3619740c73ab4bdf28a212`
  over the relevant diff in `docs/plans/usage-stats-readability.md` and
  `docs/README.md` before this review update.
- Reviewer: Codex
- Scope: closure of the Round 1 same-content comparison finding, identity of
  both renderer revisions and database inputs, reproducibility of all four
  line counts, correctness of the paired percentages and causal statements,
  and newly introduced problems.
- Findings:
  - [closed] The repair replaced the mismatched historical comparison with a
    controlled paired A/B. `HEAD` `5bf3356aa26f45e170b625930831a809a328ad17`
    is explicitly identified as the pre-plan baseline, while the current
    binary is bound to the relevant two-file code diff digest
    `d0e68ffc45f707b14e3dd5bd5df31652a15a3fa22981c1a92be51944615dc08c`;
    both identities match the state inspected in this round.
  - [closed] All four runs derive fresh byte-identical copies from one frozen
    snapshot. This correctly accounts for the discovered metadata write on
    database open even with `--no-scan`, rather than allowing an earlier run
    to mutate the input of a later one. The same environment, terminal width,
    state isolation, scan flag, periods, and grouping are recorded for both
    binaries.
  - [closed] The paired results are now computed only from same-snapshot data:
    `139` to `120` is `-13.7%`, and `832` to `142` is `-82.9%`. The original
    `140`/`822` figures are retained only as a clearly non-comparable
    historical reference, and the causal explanation is scoped to observable
    renderer differences rather than input drift.
  - No new blocking or medium-severity findings.
- Evidence: read-only inspection of the controlled-A/B method, identities,
  commands, results, limitations, and completion note; fresh confirmation of
  `HEAD`, the relevant code-diff digest, and `git diff --check` (clean). The
  real database, scratch snapshot, binaries, and real session sources were not
  accessed during this review round.
- Verdict: PASS
