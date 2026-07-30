---
status: active
plan: shell-integration
task: installation-onboarding
---

# Review log — shell-integration / installation-onboarding

## Round 1 — 2026-07-30

- Reviewed state: `0e9be70f7087decad7a29b0b650962cd6f31879f` plus reviewed file-set SHA-256 `ea671a7d67e0ed986ce734f4fe5aebe04d3215a541cd29b96b1ea425be311979`
- Reviewer: Claude Opus 5
- Scope: `README.md`, `docs/specs/cli-manual.md`, `packaging/homebrew/agentdeck.rb.tmpl`, `scripts/manage-install.sh`, `scripts/test-completion-install.sh`, `scripts/test-release-distribution.sh`; Task 4 acceptance on non-mutating installation, optional/conditional presentation, completion-versus-wrapper separation, prerequisite and cost disclosure, and `shell-init` wording
- Findings:
  - [P2] The measured per-invocation cost required by acceptance item 4 is absent, and nothing records that it is still owed. All four onboarding texts — the formula caveat, `scripts/manage-install.sh:509-518`, `README.md:157-159`, and `docs/specs/cli-manual.md:147-148` — disclose the cost qualitatively ("one extra AgentDeck process plus a read-only database access"), never a number. That is unavoidable today because task 5 owns the measurement and has not run, but the plan gives task 5 only "records the numbers in this plan", so the four texts and their `grep` assertions in `scripts/test-completion-install.sh` and `scripts/test-release-distribution.sh` would keep the qualitative wording and the acceptance item would die silently. Either record in `docs/plans/shell-integration.md` that task 5 must backfill the measured figures into these four texts and their assertions, or record the deliberate decision that qualitative disclosure satisfies acceptance item 4 and amend the acceptance wording accordingly. The same paragraphs also need revisiting once task 6's negative-gate marker lands, because the cost then becomes conditional on the marker rather than unconditional per invocation.
  - [P2] `docs/README.md:349` claimed `active — 4/8 done` while only tasks 1-3 hold a `Verdict: PASS` round. The counter's established meaning in this repository is review-passed, not dev-complete: the same cell read `1/8 done` when only task 1 had passed review. Corrected to `3/8 done` as part of this review's index-synchronization duty; recorded here so the change is attributable rather than silent. No developer action is required for this item.
  - [P2] The `COMPLETION_SHELL=none` narration branch has no coverage. `scripts/manage-install.sh:519-526` chooses between "Command completion is already installed" and "Command completion was explicitly skipped", and `scripts/test-completion-install.sh:268-282` asserts only the former, for the `fish`, `zsh`, and `bash` cases; the file contains no `COMPLETION_SHELL=none` install case at all. Task 4's verification is explicitly installer-output checks, so leaving half of a new user-facing branch unasserted is an evidence gap, not a style preference. Add a `none` install case that asserts the skipped-completion wording, the shared attribution paragraph, and that no rc file gained either managed block.
  - [P3] `docs/specs/cli-manual.md:171-174` indents the ordered-list continuation lines with one space where lines 167-169, 175-176, and 182-186 of the same item use three. CommonMark lazy continuation still renders it as one paragraph, so this is cosmetic, but it is a formatting regression inside the block the task rewrote. Restore three-space continuation indentation.
  - [P3] No onboarding text names `agentdeck shell status`. The plan's Goal lifecycle lists it as the optional self-check after setup, and the three independent states it reports are the only way a user can tell configuration from activation from route eligibility, yet `README.md`'s new "Optional Project Attribution" section, the manual's new paragraphs, the formula caveat, and the installer narration all stop at `setup` and `remove`. Add the self-check to at least `README.md` and the manual; task 8 still owns the full contract text.
- Out-of-scope finding, routed rather than counted against this task:
  - [P1 — folded into task 5 `cross-shell-acceptance` by the user's decision on 2026-07-30; scope and acceptance recorded there] The generated wrappers omit step 1 of the wrapper resolution order in "Eligibility, Activation, and Cost" — the `agentdeck`-on-`PATH` test before forking. `cmd/agentdeck/main.go:1074` and `:1084` call `command agentdeck shell-init --project-environment <client>` inside a fish command substitution with no `type -q agentdeck` guard, so with the block installed and `agentdeck` off `PATH` — the state after a bare `brew uninstall` — every `codex` invocation prints `Unknown command` and a source excerpt to stderr before running the real client. The bash and zsh bodies (`:1098`, `:1107`) are silent only because their `2>/dev/null` catches the shell's own error. `scripts/test-completion-install.sh:208-226` exercises exactly this `unavailable` mode but captures stdout only, so `make check-install` passes while the error is printed; observed during this review's run. Task 5 now owns both the guard for all three shells and a regression that captures stderr and asserts it is empty, alongside its existing "a fresh shell starts with no error output" acceptance.
- Test review:
  - The narration assertions are behavioral rather than snapshot-shaped: `install_for` now tees installer stdout and the `fish`/`zsh`/`bash` cases assert each sentence of the attribution paragraph plus both command lines, so dropping or rewording any of them fails.
  - `scripts/test-completion-install.sh:280-283` asserts that a completion install leaves no `# >>> agentdeck shell integration >>>` marker in the rc file, which is the direct behavioral protection for "package installation never edits shell integration automatically". `scripts/test-release-distribution.sh:71-78` protects the rendered formula's caveat and, together with the formula having no `post_install`, covers the Homebrew half of the same claim.
  - Gaps are the `none` branch above and the stderr blindness noted in the routed finding.
- Evidence:
  - `env GOCACHE=/private/tmp/agent-deck-go-build make check-install` passed (exit 0), covering `scripts/test-install.sh` and `scripts/test-completion-install.sh` including the new installer-narration assertions.
  - `env GOCACHE=/private/tmp/agent-deck-go-build make check-release-distribution` passed (exit 0), covering rendered-formula caveat assertions for the release and rc channels.
  - `awk 'length>80'` over `README.md` and `docs/specs/cli-manual.md` shows every over-long line predates this change set; the added paragraphs introduce no line-length regression.
  - Read-only inspection confirmed `packaging/homebrew/agentdeck.rb.tmpl` defines `install`, `caveats`, and `test` only, with no `post_install`, and that the installer narration in `scripts/manage-install.sh` is emitted after `install_succeeded=1`, so a rolled-back install prints no onboarding text.
- Verdict: REOPEN

## Round 2 — 2026-07-30

- Reviewed state: `0e9be70f7087decad7a29b0b650962cd6f31879f` plus reviewed six-file-set SHA-256 `e51f5cdc98946e76fcab679539a3ac661b30d5272e4f9b594924e7404012e323` and `docs/plans/shell-integration.md` SHA-256 `a37ad9dd4564249da2e38a443299a7c9182cb3d9fbb09ec12cb8fe7fb9fa383f`
- Reviewer: Claude Opus 5
- Scope: Round 1 findings and regression risk in `README.md`, `docs/specs/cli-manual.md`, `scripts/test-completion-install.sh`, and the task 5 disclosure follow-up in `docs/plans/shell-integration.md`
- Finding resolution:
  - [CLOSED] The measured-cost obligation is now recorded rather than left to lapse. `docs/plans/shell-integration.md:1060-1070` adds a mandatory disclosure follow-up to task 5: task 6 must revise the wording from "on each invocation" to marker-dependent once the negative gate lands, and task 5 must backfill its measured figures into the formula caveat, `scripts/manage-install.sh`, `README.md`, and `docs/specs/cli-manual.md` plus both assertion sets, with qualitative disclosure explicitly declared insufficient for acceptance item 4. This is Round 1's option A, and it names the same four texts and two assertion files the finding did.
  - [CLOSED] The `COMPLETION_SHELL=none` branch is covered. `scripts/test-completion-install.sh:315-345` asserts every line of the shared attribution paragraph, both command lines, the skipped-completion wording, `completion_shell=none` in the manifest, the absence of a completions directory, an rc file byte-identical to its pre-install content, the absence of both managed markers, and rc stability across uninstall. The assertions are per-sentence rather than snapshot-shaped, so rewording either branch fails.
  - [CLOSED] `docs/specs/cli-manual.md:170-176` restores three-space ordered-list continuation indentation, matching the neighbouring items.
  - [CLOSED] The self-check is documented. `README.md:168-175` adds `agentdeck shell status` after setup and names the three states it distinguishes; `docs/specs/cli-manual.md:155-156` adds the same in the manual.
- Findings:
  - [P2] `docs/specs/cli-manual.md:156` ends the new sentence with "完整输出契约由任务 8 收口", putting an internal plan task number into a living specification. The manual is permanent and this plan retires to `docs/archive/plans/` when its last task passes, so the reference becomes dangling; it is also meaningless to a reader who has never seen the plan. Task 8's own scope list does not include removing it, so nothing guarantees it goes away. Delete the clause, or replace it with a forward reference that stays true after archival. No other new text in either document references plan internals.
- Test review:
  - No regression in the previously reviewed assertions: the `fish`/`zsh`/`bash` narration greps, the "completion install leaves no integration marker" check, and the rendered-formula caveat assertions are unchanged and still pass.
  - The `none` case closes the last coverage gap that was attributable to this task. The stderr blindness noted in Round 1 remains, correctly, task 5's.
- Evidence:
  - `env GOCACHE=/private/tmp/agent-deck-go-build make check-install` passed (exit 0) on the current file set.
  - `env GOCACHE=/private/tmp/agent-deck-go-build make check-release-distribution` passed (exit 0) on the current file set.
  - `grep` over `README.md` and `docs/specs/cli-manual.md` for plan task references returns only `docs/specs/cli-manual.md:156`.
  - `awk 'length>80'` over the changed regions shows no line-length regression; the flagged `docs/specs/cli-manual.md` lines are byte counts over CJK text, well inside the character-width guidance, and the pattern predates this change set.
- Verdict: REOPEN

## Round 3 — 2026-07-30

- Reviewed state: `0e9be70f7087decad7a29b0b650962cd6f31879f` plus reviewed six-file-set SHA-256 `d43267489fe61fe903b4c93c222069653b67e5b5cf84b19eda44db81e3940262`
- Reviewer: Claude Opus 5
- Scope: Round 2's plan-reference finding, and a re-read of every paragraph the fix rounds added to `docs/specs/cli-manual.md:143-165` and `README.md:152-189`
- Finding resolution:
  - [CLOSED] The internal plan reference is gone. `docs/specs/cli-manual.md:155-156` now ends at the three states, and `grep -n '任务 [0-9]\|task [0-9]\|Task [0-9]'` over `README.md` and `docs/specs/cli-manual.md` returns nothing.
- Findings:
  - [P2] Inserting the `shell status` paragraph broke the referent of the paragraph that follows it. `docs/specs/cli-manual.md:155` uses 该命令 for `agentdeck shell status`, and then `:158` opens with 该命令为所有当前使用中的 shell 写入 attribution wrapper while meaning `agentdeck shell setup` — whose code block is now two paragraphs above. The nearest antecedent is the read-only status command, so the sentence reads as a claim that `shell status` writes wrappers to every shell in use, which is the opposite of the read-only contract task 3 delivered and this task documents. Name `shell setup` explicitly at `:158` instead of relying on 该命令, or move the status paragraph below the setup description so the pronoun binds correctly. `README.md:168-177` has the same insertion but says "the status output" and "Setup is optional" explicitly, so it is unaffected. This was introduced by the Round 1 fix and missed by Round 2's read; recorded here rather than left implicit.
- Test review: no test change in this round; the assertion sets reviewed in Round 2 are unchanged and still pass.
- Evidence:
  - `env GOCACHE=/private/tmp/agent-deck-go-build make check-install` passed (exit 0) on the current file set.
  - `grep -n '任务 [0-9]\|task [0-9]\|Task [0-9]' README.md docs/specs/cli-manual.md` returned no matches (exit 1).
  - `git diff --check` clean.
  - `make check-release-distribution` is reused from Round 2: neither `packaging/homebrew/agentdeck.rb.tmpl` nor `scripts/test-release-distribution.sh` changed between the two rounds, and this round's only edit is prose in `docs/specs/cli-manual.md`, which no distribution assertion reads.
- Verdict: REOPEN

## Round 4 — 2026-07-30

- Reviewed state: `0e9be70f7087decad7a29b0b650962cd6f31879f` plus reviewed six-file-set SHA-256 `0b245dc4799ba704facb781311508973f38a9cb58daeeb7a047baa15c7de4b0f`
- Reviewer: Claude Opus 5
- Scope: Round 3's referent finding, and every remaining pronoun in `docs/specs/cli-manual.md`'s project-attribution section
- Finding resolution:
  - [CLOSED] `docs/specs/cli-manual.md:158` now opens with `agentdeck shell setup` explicitly instead of 该命令, so the write-to-every-shell-in-use claim can no longer be read as describing the read-only status command. The two remaining 该命令 uses in the file are unambiguous: `:155` follows `agentdeck shell status` in the same sentence, and `:174` follows `agentdeck shell-init <bash|fish|zsh>` at the head of the same list item.
- Findings:
  - [nit] `docs/specs/cli-manual.md:158` is now 95 display columns where every other line in the section wraps at 77-81. Cosmetic only, and the next edit to this paragraph should rewrap it. Not a blocker.
- Test review: no test change in this round. All assertion sets reviewed in Round 2 are unchanged.
- Evidence:
  - `grep -n '该命令' docs/specs/cli-manual.md` returns exactly two occurrences, both checked above for a unique antecedent.
  - `git diff --check` clean.
  - `make check-install` and `make check-release-distribution` are reused from Round 3 and Round 2 respectively. This round's only change is one prose line in `docs/specs/cli-manual.md`; no assertion in `scripts/test-completion-install.sh` or `scripts/test-release-distribution.sh` reads that file, and the formula template, installer script, and both test scripts are byte-identical to the states those runs covered.
- Verdict: PASS
