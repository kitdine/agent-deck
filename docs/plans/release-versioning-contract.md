---
status: active
created: 2026-08-02
---

# Release Versioning Contract

Target release: `v0.2.2`.

The repository has shipped four releases without a written rule for what a
version-number position means. `docs/specs/cli-design.md` defines the tag shape
(`v<major>.<minor>.<patch>` and strict `vX.Y.Z-rc.N`) and the two Homebrew
channels, but nothing states which kind of change may raise which position. The
patch position has therefore been used as a release counter rather than as a
compatibility signal.

## Goal

- Give each version position a testable meaning, so release scope becomes a
  decision the contract makes rather than a judgement repeated per release.
- Record the meaning where the rest of the distribution contract already lives,
  not in a plan that will be archived.

## Non-Goals

- No change to tag syntax, the release workflow, the Homebrew channels, or
  `make release-*`.
- No retroactive renumbering, retagging, or re-release of `v0.1.0` through
  `v0.2.1`.
- No automated enforcement. This is a rule for humans and agents deciding
  scope, not a CI gate.

## Evidence Baseline

Gathered on 2026-08-02 at `308feb0`.

The specification's distribution section (`cli-design.md:341-390`) covers the
release workflow, tag derivation, both formulae, and the RC channel. The only
statement about the number itself is the accepted shape at line 365. No
numbered rule in `Acceptance Criteria` (lines 1859-1988, currently rules 1-54)
assigns meaning to a position.

Observed history, from the tags and their commit ranges:

| Range | Content | Position used | Position implied by content |
| --- | --- | --- | --- |
| `v0.1.0`→`v0.1.1` | Test-gap closure, four production fixes, tap automation | patch | patch |
| `v0.1.1`→`v0.2.0` | `provider set-wrapper`, `provider use --via`, two new columns, RC channel | minor | minor |
| `v0.2.0`→`v0.2.1` | `agentdeck shell` command group, `shell-init`, project attribution, all text timestamps relocalized | **patch** | **minor** |

The `v0.2.0`→`v0.2.1` row is the evidence that the absent rule has already cost
something: a release that added a command group and changed every human-facing
timestamp shipped in the position that promises neither.

## Decision

Two positions carry meaning during `0.x`; the third is reserved.

**MAJOR** stays `0` for the whole `0.x` line. Raising it to `1` is a separate,
explicit declaration that the CLI contract has entered a stability commitment,
not a consequence of any single change.

**MINOR** (`0.Y.0`) is required when any of the following is true:

1. a command, subcommand, or flag is added, removed, or renamed;
2. the database schema migrates, in either `agentdeck.sqlite3` or
   `sessions.sqlite3`;
3. stdout text, JSON, NDJSON, or exit-code semantics change;
4. a user-visible number changes for unchanged input — cost, coverage, token
   attribution, or a count;
5. a stable typed error code is added, removed, or renamed;
6. persisted data gains a format or version that an earlier release cannot
   read, so downgrading is unsafe;
7. a statement of promised behavior in `docs/specs/cli-design.md` is rewritten
   rather than clarified.

**PATCH** (`0.Y.Z`) covers everything else: defect fixes that restore already
promised behavior, performance and robustness work, internal refactors,
documentation accuracy corrections, and improved diagnostic or error **message**
text that keeps its error code and exit code. A patch release must be safe to
downgrade from: schema unchanged, persisted formats unchanged, stdout contract
byte-compatible.

Two boundaries are stated explicitly because both have already come up:

- **Error text versus error code.** Rewording an error message, including
  replacing leaked internal text with actionable guidance, is PATCH. Adding or
  renaming the typed `code` in the JSON error envelope is MINOR, because the
  specification's "Output and Errors" section pins typed error codes through
  stable fixtures.
- **Specification version versus release version.** `docs/specs/cli-design.md`
  carries its own independent `version:`. Raising it does not oblige a minor
  release, and a minor release does not oblige raising it. A patch release may
  raise the specification version when it adds or clarifies a rule without
  changing promised behavior.

**RC requirement.** Any release that touches persisted data, the pricing read
path, or a configuration file owned by an external client ships at least one
`-rc.N` and is validated against real local data before the stable tag.

## Tasks

### 1. `version-semantics-rule`

Record the Decision above in the living contract.

- Add the position semantics to the distribution section of
  `docs/specs/cli-design.md`, adjacent to the existing tag-shape statement at
  line 365, so the shape and the meaning are read together.
- Add one numbered rule to `Acceptance Criteria` continuing the existing
  sequence (currently ends at 54), stating the MINOR trigger set, the PATCH
  safety property, the error-text/error-code boundary, and the independence of
  the specification version from the release version.
- Raise the specification version by one from whatever is current at delivery
  and add one changelog row.
- Do not restate the rule in `AGENTS.md` or `docs/README.md`; both may link to
  it. A second copy is how the two drift.

Acceptance:

- Every MINOR trigger in the Decision appears in the shipped rule; the rule does
  not add a trigger the Decision does not have.
- The rule states that a patch release must be safe to downgrade from.
- The error-message versus error-code boundary is explicit.
- The specification-version independence is explicit, so raising `version:` in a
  patch release is not later read as a contract violation.
- No existing numbered rule is renumbered.

Verification: L0. Documentation and link checks plus `git diff --check`.

## Status

| Task | Dev | Review |
| --- | --- | --- |
| 1. `version-semantics-rule` | [ ] | [ ] |

## Starting a task

> 进入开发：`release-versioning-contract` / `version-semantics-rule`

Read `AGENTS.md`, this plan's Decision, the distribution section and
`Acceptance Criteria` of `docs/specs/cli-design.md`, and the L0 verification
route. Tick `Dev` after the documentation checks pass; an independent reviewer
records a PASS round under
`docs/reviews/release-versioning-contract/version-semantics-rule.md` before
ticking `Review`.
