---
status: active
plan: project-attribution
task: attribution-guidance
---

# Review log — project-attribution / attribution-guidance

## Round 1 — 2026-07-29

### Findings

- [P2] JSON/quiet equivalence was asserted with manually constructed identical
  data rather than the real `provider use --via` Headroom CLI outputs.
- [P2] The advisory URL and forbidden-content assertions inspected a duplicate
  expected string instead of the actual stderr line, so regressions in command
  output could escape the test.
- [P2] The CLI manual presented the not-yet-implemented `agentdeck shell-init`
  as available instead of describing the current user-maintained shell-function
  mechanism.

**Verdict: REOPEN.**

## Round 2 — 2026-07-29

This is the implementer's fix record, not an independent review pass.

- Replaced the synthetic JSON check with real guided and `--quiet`
  `provider use --via` invocations, comparing decoded envelopes after removing
  only `generated_at`.
- Asserted against the actual project-attribution stderr line: exactly one line,
  exactly one URL equal to `provider.ProjectAttributionGuideURL`, and no
  third-party hostname, issue, or release content. Removed the test's duplicate
  hard-coded documentation URL.
- Corrected the manual to describe a user-maintained shell function while
  preserving all three mechanisms, the Claude settings recipe, and
  documentation-only upstream references.
- Compilable mutation evidence: appending
  `https://third-party.example/issues/1` to the production advisory failed both
  focused tests with `project attribution advisory URLs = 2`; restoring it
  returned the focused command to GREEN.

Final verification:

- `go test -count=1 -mod=vendor ./cmd/agentdeck -run ProjectAttributionGuidance`
  — passed.
- `go test -count=1 -mod=vendor ./cmd/agentdeck` — passed.
- `gofmt -l` on the four listed Go files — no output.
- `git diff --check` — passed.

Next workflow:
`进入复评并生成后续指令:project attribution / attribution-guidance`.

## Round 3 — 2026-07-29

### Re-review

All three Round 1 findings are closed:

- The JSON/quiet equivalence assertion consumes two real
  `provider use --via` CLI outputs and removes only `generated_at`.
- The stderr assertion inspects the actual project-attribution advisory line,
  requires exactly one line and one production guide URL, and rejects upstream
  hostname, issue, and release content. The recorded, compilable two-URL
  mutation failed this behavioral assertion before being restored.
- The manual describes the currently available user-maintained shell-function
  mechanism without presenting `agentdeck shell-init` as implemented, while
  preserving all three mechanisms, the Claude settings recipe, and upstream
  references in documentation only.

Independent focused verification:

- `rtk test env GOCACHE=/private/tmp/agent-deck-go-build go test -count=1
  -mod=vendor ./cmd/agentdeck -run ProjectAttributionGuidance` passed.
- `rtk proxy gofmt -l cmd/agentdeck/main.go
  cmd/agentdeck/provider_wrapper_kind_test.go
  cmd/agentdeck/project_attribution_guidance_test.go
  internal/provider/service.go` produced no output.
- `rtk git diff --check` exited 0.

The immediately preceding full `./cmd/agentdeck` package result was reused
after confirming the relevant content state had not changed.

**Verdict: PASS.**

Next task:
`进入开发:project attribution / shell-helpers`.
