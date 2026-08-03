---
status: historical
plan: release-versioning-contract
task: version-semantics-rule
retired: 2026-08-03
---

# Review log — release-versioning-contract / version-semantics-rule

## Round 1 — 2026-08-02
- Reviewed state: working tree on top of `308feb0`, uncommitted changes to
  `docs/specs/cli-design.md` and new `docs/plans/release-versioning-contract.md`
- Reviewer: independent review pass (read-only)
- Scope: the new `#### Version Number Semantics` subsection, Acceptance
  Criteria rule 55, the version-22 changelog row, and the frontmatter version
  bump in `docs/specs/cli-design.md`; the Dev evidence paragraph in
  `docs/plans/release-versioning-contract.md`, checked against the plan's
  Decision and Task 1 Acceptance.
- Findings:
  - [nit] The prose subsection lists the typed-error-code trigger as its own
    clause (matching Decision item 5), while rule 55 merges it into the
    command/subcommand/flag clause -> both enumerate all 7 Decision triggers
    with no additions, so this is a stylistic asymmetry, not a defect; no
    change required to pass.
  - [nit] `docs/README.md` still reads "Currently version 21" for
    `specs/cli-design.md` (line 418) and uses "version 21" in two narrative
    paragraphs (lines 120, 411) written for other plans -> out of Task 1's
    Acceptance criteria and the plan's own instruction to not restate the rule
    in `docs/README.md`; left for a separate doc-sync pass rather than
    reopening this task.
- Evidence:
  - `git diff --check -- docs/specs/cli-design.md docs/plans/release-versioning-contract.md`
    exits 0 (clean).
  - `grep -n "^5[0-5]\." docs/specs/cli-design.md` confirms rules 50-54
    unchanged and rule 55 newly appended; no renumbering.
  - Read `docs/specs/cli-design.md:1715-1745` and confirmed "Output and
    Errors" actually pins typed error codes through stable fixtures, so the
    cross-reference in the new subsection is accurate.
  - Compared the new subsection and rule 55 against the plan's Decision
    line-by-line: all 7 MINOR triggers present in both with no additions, the
    PATCH downgrade-safety property is stated, the error-text/error-code
    boundary is explicit, and the specification-version/release-version
    independence is explicit.
  - Confirmed placement: the only existing tag-shape statement in the
    Release and Distribution section is the strict RC-tag sentence at
    (pre-edit) line 365, embedded mid-paragraph; the new `####` subsection
    was inserted immediately after that paragraph ends, which is the closest
    a heading-level block can sit without splitting the paragraph.
- Verdict: PASS
