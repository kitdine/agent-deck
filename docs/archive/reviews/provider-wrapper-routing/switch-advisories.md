---
status: historical
plan: provider-wrapper-routing
task: switch-advisories
---

# Review log — provider-wrapper-routing / switch-advisories

## Round 1 — 2026-07-27 (summary, delivered in session)

- Reviewed state: base `a9baa86`, uncommitted working tree.
- Verdict: **PASS with improvements**, no blocking finding. All three
  acceptance criteria hold: advisories reach stderr, the JSON envelope matches
  a switch made with no conflict present field for field, and both unowned
  credential sources plus both unrelated fields survive the switch untouched,
  satisfying the plan invariant that `env.ANTHROPIC_API_KEY` is reported and
  never removed. Three independent revert checks confirmed the tests were RED
  without their fix (empty-value filtering, stderr routing, `official`-only
  scoping).
- Findings:
  1. **`configuresCredential` reported malformed non-string values.**
     `default: true` meant `false`, `0`, `{}`, and `[]` were announced as
     overriding the official selection, contradicting the function's own stated
     rule; conversely `strings.TrimSpace` hid a blank-but-non-empty string,
     which Claude would actually use.
  2. **The detection boundary was undocumented**, so a missing advisory could
     be read as "no conflict".
  3. **Two scope decisions were unrecorded**: a custom-provider selection is
     never checked, and a shell-exported credential is invisible.
- Checked and dismissed: printing the settings path (`extension show` already
  prints absolute paths); non-JSON lines on stderr during a successful JSON-mode
  run (the v10 scan-progress precedent, `cli-design.md:1741`); `--quiet`
  suppressing the conflict note; reading the settings file after the write
  (that is the state the advisory describes); a failed switch printing nothing.

## Round 2 — 2026-07-27 (re-review)

- Reviewed state: base `a9baa86`, uncommitted working tree, five modified files
  plus two new test files. The repository was not modified by this pass.
- Method: each finding re-derived from the current source; the converged
  detector independently reproduced in its RED state in an out-of-tree copy.

### Finding-by-finding disposition

- **[1] Value test — FIXED, and the rule is now one sentence.**
  `configuresCredential` reports exactly one shape, a non-empty string, with
  the reasoning stated at the function: both keys are string-valued to Claude,
  so every other shape is empty or malformed for the key it sits on and no
  credential can be derived from it. The blank-string case is now reported and
  the comment says why (Claude receives it, uses it, and fails to
  authenticate). Comment, tests, and behavior agree: twelve silent shapes
  (absent, `""`, `null`, no `env` object, non-object `env`, `false`, `true`,
  number, `{}`, `[]`, non-empty object, non-empty array) and one reported
  blank-string case. Independently reproduced RED against presence-only
  detection: nine subcases fail, including all four the Round 1 finding named.
  No false negative found — the reported set is exactly "a non-empty string on
  either key", which is the only shape from which Claude can read a credential
  out of this file.
- **[2] Documented boundary — FIXED, with one omission (see below).** The
  manual now carries a `检测边界（没有提示 ≠ 没有冲突）` block. Each of its three
  claims matches the implementation: the path is `--config-path` when given and
  `~/.claude/settings.json` otherwise (`defaultConfigPath`); the conflict note
  is gated on `name == OfficialProviderName`; and the value rule is the
  converged non-empty-string rule including the blank-string case.
- **[3] Scope decisions — FIXED (recorded, not implemented).** Both are in the
  plan with their reasoning, and neither widened the implementation beyond the
  spec sentence at `cli-design.md:665-670`. The custom-route rationale is
  consistent with `## Out of Scope`: that section says the proxy this design
  targets matches Bearer first, which is a statement about one proxy rather
  than a guarantee for every upstream, so "AgentDeck cannot predict which one
  wins" does not contradict it — it is the same fact stated for the general
  case. The environment-variable rationale (AgentDeck's own process environment
  is not necessarily the environment the user's Claude client runs under) is
  sound and argues against implementing it rather than for it.

### Requested cross-checks

- **`claude-writer-routes` write behavior is untouched.** The diff to
  `internal/provider/config.go` is purely additive and sits above
  `type ClientConfig`; no `Write*` or `ConfigMatches*` function line changed.
  `configuresCredential` has exactly two callers, both inside
  `ClaudeCredentialConflicts`.
- **`ConfigMatchesOfficialClaude` is not confused with the new detector.** They
  answer adjacent but different questions on disjoint keys.
  `ConfigMatchesOfficialClaude` asks whether AgentDeck's own write survived, so
  it tests *presence* of `ANTHROPIC_BASE_URL`/`ANTHROPIC_AUTH_TOKEN` — correct,
  because the writer deletes those keys outright, so a key present at all is
  drift regardless of its value. The advisory asks whether Claude can obtain a
  credential from a field AgentDeck does not own, so it tests the *value* of
  `ANTHROPIC_API_KEY`/`apiKeyHelper`. The differing predicates are justified by
  the differing questions, and the key sets do not overlap. The drift and
  writer tests still pass.

### Open finding

- **🟡 [docs/specs/cli-manual.md, 检测边界 block] The boundary list names the
  shell environment as the only unchecked source, which over-promises.** Claude
  resolves settings from more than one scope — at least a project-level
  `.claude/settings.json` and `.claude/settings.local.json` alongside the user
  file AgentDeck manages. A credential parked in one of those is exactly the
  "advisory absent, conflict present" case this block exists to prevent, and a
  reader of the current text would conclude that every settings-file source is
  covered.
  -> Widen the first bullet to say AgentDeck checks only the settings file it
  manages (or the `--config-path` override), and that other sources — the shell
  environment and any other settings scope Claude consults — are outside
  detection. Confirm the current scope list against Claude's own documentation
  before naming specific filenames; wording that stays general is preferable to
  a list that goes stale.

### Verdict

**One open improvement.** All three Round 1 findings are closed and the code
side is settled: the detection rule, its comment, and its tests agree, the RED
state was independently reproduced, and neither the Claude writers nor the
route-composition drift matcher was disturbed. What remains is a single
documentation clause in the boundary block added this round. `Review` left
unticked until it closes.

Next pass: Round 3 in this file.

## Round 3 — 2026-07-27 (re-review of the documentation round)

- Reviewed state: base `a9baa86`, uncommitted working tree. Scope limited to
  the single finding left open by Round 2; the repository was not modified by
  this pass.

### Disposition

- **[Round 2 open finding] CLOSED.** The first boundary bullet no longer casts
  the shell environment as the only unchecked source. It now leads with "only
  the one Claude settings file AgentDeck manages", then lists the excluded
  sources as a set: the shell environment *and* any other settings scope Claude
  consults. The wording is appropriately conservative — it pins no filenames
  beyond the managed one and defers the scope list to Claude's own
  documentation, so it cannot go stale as that client evolves.
- **Matches the implementation, clause by clause.** "the file AgentDeck
  manages, or the `--config-path` override" is exactly `SwitchAdvisories`'s
  resolution order (`configPath` when non-empty, otherwise
  `defaultConfigPath(Home, ClientClaude)` → `<home>/.claude/settings.json`), and
  "only the two fields" is exactly what `ClaudeCredentialConflicts` reads out of
  that one file.
- **Plan's third scope decision is sound and does not repeat the other two.**
  It cross-references the shell-environment decision for its rationale rather
  than restating it, and adds the reason specific to this boundary: widening
  detection would mean adopting Claude's whole settings-resolution order, a
  client behavior this project does not track. The three decisions partition
  cleanly — custom-provider selections, the process environment, other settings
  scopes.
- **Value semantics now live in one place.** The conflict bullet defers with
  "哪些取值才算配置了凭据见下方检测边界" and the boundary block's third bullet
  carries the full rule. The reference resolves within the same section; no
  dangling pointer and no second, shallower description.
- **No Go file was touched this round.** `git diff --stat` reports the same Go
  line counts as before the documentation round (`main.go` +16,
  `config.go` +55, `service.go` +36), and the targeted test run returned
  `(cached)` for both packages — Go's build cache keys on package inputs, so a
  cache hit is positive proof that the sources and test files are byte-identical
  to the state Round 2 verified.

### Nit (no fix required)

- The bullet closes with "AgentDeck 只写、也只读它管理的这一个文件". Read inside
  its bullet the scope is obviously the Claude settings file, but the sentence
  is unqualified, and AgentDeck does read other Claude-owned paths elsewhere
  (session logs) and write a redacted copy of this file into its state root.
  Adding "settings" to that sentence would remove the ambiguity entirely.

### Verdict

**PASS.** The Round 2 finding is closed, the documentation now matches the
implementation clause by clause without over-promising, and the round was
genuinely documentation-only. `Review` ticked for `switch-advisories`; no
further round follows.
