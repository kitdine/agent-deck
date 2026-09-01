---
status: active
created: 2026-08-25
updated: 2026-08-25
---

# AgentDeck Roadmap and Backlog

This file is the authority for later version direction, unapproved planning
intake, and withdrawn candidates. Active version membership is owned by the
applicable `vX-Y-Z-contract` topic and projected in `status.md`. Moving an item
between these sections is a planning decision, not implementation authorization.
## Roadmap

Planned after `v0.5.0`. Re-planned on 2026-08-13; this table supersedes the
release sequence previously recorded in the desktop plan. Each version has a
Beads tracking epic and a blocked design task, and needs a bounded topic under
`docs/topics/` before development starts. Version themes are commitments of
sequence, not of scope detail.

| Version | Theme | Scope |
| --- | --- | --- |
| `v0.7.0` | Subscription quota | Quota interface feasibility, in-app network quota lookup, allowance-window and reset modelling, quota alerting, automatic update download, prerelease channel selection. |
| `v0.8.0` | Boundary consolidation and Linux | Versioned client adapter contract, Linux machine identity, de-darwin PTY tests, Linux CI matrix and release artifacts. |
| `v0.9.0` | Observability completion | Extension enabled state, cross-client duplication and drift, source authenticity, structured session search filters, wrapper health probing, richer desktop session window. |
| `v1.0.0` | Multi-device and trust | Device dimension, backup merge import, read-only aggregation views, CLI archive signing and notarization. |

Direction decisions that shape this roadmap:

- Client breadth stays at Codex and Claude. An explicit versioned client adapter
  contract is extracted so a later out-of-process plugin model can add clients
  externally; no third client is added in-tree.
- Each machine remains its own authoritative store. Cross-device support is
  read-only aggregation, never bidirectional synchronization.
- Proactive behavior — alerting and scheduled evaluation — is hosted by the
  menu-bar app. No daemon, LaunchAgent, or network listener is introduced, and
  alert rules stay in Go.
- The CLI targets macOS and Linux; the GUI stays macOS-only. Capability layering
  is explicit rather than accidental.
- Cost has three coexisting dimensions. Third-party API with a multiplier and
  official API are real spend computed locally; official subscription is quota,
  requires network access, and is therefore handled inside the app. Equivalent
  API cost is retained as a reference baseline for every mode.
- Extension work is bounded to cross-client observability. Each client already
  owns its own management surface; no tool reports the cross-client view.

## Backlog

These candidates have no approved implementation plan. Promote each into a
bounded plan before development; do not expand an active plan opportunistically.
Candidates that carry a delivery version live in the Roadmap above. An item
labelled as planning intake for a version is only scheduled for separate design
and disposition while that version is planned; it is not yet part of that
version's delivery scope.

Planning intake for `v0.5.0` cost truthfulness (keep these as separate candidates
until design evidence justifies merging them):

`v0.6.0` is cancelled as a release unit. Its cost-truthfulness scope moves into
`v0.5.0`, but version membership does not merge the independent candidates
below. The attribution-classification defect is no longer a Backlog candidate:
it is owned by the active
[`usage-attribution-precision`](archive/topics/usage-attribution-precision/tasks.md)
topic. That topic corrects the current contract that reserves `exact` for
`agentdeck run` while hardcoding otherwise determinable Hook routes as
`estimated`; a determinable event classified as `inferred` is not publishable.
There is no later attribution item to reconcile from this checklist.

Pricing catalogs and tiers, credits, Context Efficiency, subscription
discovery, and the other candidates below remain independent even though they
share the `v0.5.0` planning boundary.

- [ ] Model the two public-API price tiers for the GPT-5.6 family around the
  `272K` context boundary. Determine the tier for each request from the concrete
  context evidence in its JSONL record rather than from a session-wide or model
  name assumption; confirm the exact boundary semantics during design.
- [ ] Audit and correct the current public-API price assigned to `gpt-5.6-sol`,
  including its model identity and input, cached-input, and output rates.
- [ ] Diagnose why the latest LiteLLM price catalog produces incorrect AgentDeck
  prices. Keep upstream catalog data, catalog retrieval/versioning, parsing, and
  model-name matching as distinct hypotheses until evidence identifies the
  failing layer.
- [ ] Treat `codex-auto-review` as a separately auditable fallback-classification
  candidate whose currently public model mapping is `gpt-5.4`; retain the
  [OpenAI credit rate card](https://help.openai.com/en/articles/11481834-chatgpt-rate-card-business-enterpriseedu-credit-based-pricing)
  as the cited public source and define freshness/fallback behavior during
  design.
- [ ] Preserve public API-equivalent pricing as the cost estimator's baseline,
  while evaluating a separate credit-denominated pricing and presentation
  format. Never present credits, API-equivalent cost, or an actual subscription
  charge as interchangeable values without an authoritative conversion.
- [ ] Define a `Context Efficiency` diagnostic before choosing a presentation
  surface. Evaluate model, token volume, cache hit, context size, a `>272K`
  marker, long-context multiplier, credit cost, and API-equivalent cost; define
  an observable meaning for "useful context" and "wasted context" rather than
  deriving those values from input-token count alone.
- [ ] Investigate a Codex and Claude subscription-query surface as its own
  capability. Keep plan/account discovery separate from the existing `v0.7.0`
  allowance-window, reset, quota-alerting, and quota-cycle cost work unless the
  later design proves that they share one authoritative data source and
  lifecycle.

- [ ] Revisit ChatGPT app project attribution only if the app exposes a stable,
  reachable project configuration surface.

## Withdrawn Candidates

Recorded so they are not rediscovered as gaps. Reopen only if the stated reason
stops holding.

- **Desktop update check.** Withdrawn from `v0.5.0` entirely on 2026-08-18 — no
  menu item, no preference, no copy, no network request. The desktop app is
  installed through the Cask or a direct download, both of which already carry an
  upgrade path, so an in-app check would add the product's only outbound request
  to duplicate one.
- **Homebrew core submission.** Not important to this project; the personal tap
  already serves stable and release-candidate channels.
- **Claude subscription/account switching.** Technically reachable — the login
  state is a single per-system-user macOS Keychain entry — but withdrawn: OAuth
  refresh tokens rotate server-side so a saved snapshot silently expires and
  cannot be validated offline, persisting another product's credential
  contradicts this project's no-plaintext-credential rule, cross-application
  Keychain access is hard to justify in the trust model, and a failed write
  leaves the user unable to authenticate with no rollback path.
- **Extension mutation lifecycle.** The preview/plan/apply/ownership/rollback
  engine and its GUI, previously planned as two whole releases, chase each
  client's evolving extension format and duplicate management surfaces the
  clients already ship. Replaced by cross-client observability in `v0.9.0`.
