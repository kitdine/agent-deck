---
status: active
created: 2026-08-25
updated: 2026-09-05
---

# AgentDeck Roadmap and Backlog

This file is the authority for later version direction, unapproved planning
intake, and withdrawn candidates. Active version membership is owned by the
applicable `vX-Y-Z-contract` topic and projected in `status.md`. Moving an item
between these sections is a planning decision, not implementation authorization.
## v0.6.0 — Trusted usage and subscription visibility

Re-planned by the operator on 2026-09-05 after stable v0.5.0. This decision
reuses the version number with a new scope; the old cancelled cost-truthfulness
epic remains historical. The iteration entry is `ad-v060-iteration`.

These are six selected feature areas for topic design, not an approved
implementation breakdown. Beads carries planning coordination; this section
owns the selection. A version-contract topic assembles the delivered topics
later. Do not create development tasks before each topic's tasks.md passes.

| Feature | Planning carrier | Reason and boundary |
| --- | --- | --- |
| Schema compatibility and Hook failure visibility | `ad-schema-compatibility` | Prevent permanent schema refusal from silently losing Hook observations; reuse the existing schema-version-signal document tasks. |
| Menu-bar and Widget refresh | `ad-desktop-refresh` | Make freshness and errors truthful; target approximately 1 minute for the menu bar and 3–5 minutes for widgets with change-driven updates. |
| Snapshot performance | `ad-snapshot-performance` | Reduce repeated aggregation before increasing refresh frequency; verify invalidation and output equivalence. |
| Cost and price transparency | `ad-cost-transparency` | Distinguish actual-spend estimates from API-equivalent estimates; audit catalog, model mapping, and request price tiers separately. |
| Actionable health recovery | `ad-health-recovery` | Resolve lock recovery guidance and stale extension inventory with cause-specific actions; retain the two origin bugs. |
| Codex/Claude subscription accounts, quota, reset and alerts | `ad-subscription-quota` | Make subscription capacity actionable, including Codex reset-count information, reset times, and opt-in reminders. |

Subscription design must verify each field's source and supported semantics.
Codex reset counts are explicitly in scope: distinguish any official total,
used or remaining reset allowance from natural quota-window resets and locally
observed reset events. Unsupported fields report unavailable with a reason,
never zero or an invented count. A missing critical source is a named delivery
gap requiring disposition, not permission to silently remove this feature.
Account/plan discovery, quota retrieval and reminder evaluation may have separate
sources. Authentication, staleness, polling budgets, account isolation and
notification deduplication are part of the design. No reset action, account
login switching, automatic app updater, or plaintext credential persistence is
included.

Design subscription-source feasibility early alongside schema and performance
work. Refresh delivery depends on snapshot performance; subscription reminder
presentation coordinates with refresh and billing labels with cost transparency,
without treating those coordination points as blanket design blockers.

Structured session search is excluded from v0.6.0 and remains an unscheduled
candidate. Credits conversion, Context Efficiency, Linux, a public adapter
protocol, full extension observability and multi-device aggregation remain
later candidates. Existing withdrawal decisions remain effective.

The old v0.7.0 subscription epic/plan/Gate are superseded by this selection;
their historical records remain parked. The remaining old version rows are
directional candidates rather than a required release order.

## Roadmap

The v0.6.0 selection above is current. Later rows retain the 2026-08-13
planning references for traceability; their version numbers and ordering are
provisional and can be reconsidered. Each feature needs a bounded topic before
development starts.

| Version | Theme | Scope |
| --- | --- | --- |
| `v0.6.0` | Trusted usage and subscription visibility | The six feature areas selected above; subscription quota includes Codex reset-count information. |
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

Pricing intake selected for v0.6.0 and remaining candidates (keep them separate
until design evidence justifies merging them):

The earlier v0.6.0 cost-truthfulness release was cancelled and its attribution
scope shipped in v0.5.0. The new v0.6.0 selection above supersedes that release
sequence. Pricing investigations below belong to its cost-transparency design;
credits and Context Efficiency remain unscheduled. The attribution defect is
no longer a Backlog candidate:
it was delivered by the archived
[`usage-attribution-precision`](archive/topics/usage-attribution-precision/tasks.md)
topic. That topic corrects the current contract that reserves `exact` for
`agentdeck run` while hardcoding otherwise determinable Hook routes as
`estimated`; a determinable event classified as `inferred` is not publishable.
There is no later attribution item to reconcile from this checklist.

Pricing catalogs and tiers remain independent investigations within v0.6.0.
Credits and Context Efficiency remain later candidates. Subscription discovery
is selected into v0.6.0 with quota, reset information and reminders.

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
- [ ] Deliver the selected v0.6.0 Codex and Claude subscription feature above.
  Keep account discovery and quota/reset sources independently verifiable even
  when they share a user-facing feature.

- [ ] Revisit ChatGPT app project attribution only if the app exposes a stable,
  reachable project configuration surface.

- [ ] Give `state_busy` actionable recovery guidance, now selected for v0.6.0
  design under health-recovery. `internal/store/store.go`
  returns one hardcoded `timed out waiting for state lock` for both locks, so the
  message never says whether `state.lock` or `scan.lock` is held, never points at
  `agentdeck doctor`, and states no safe recovery path. Deferred from the `rc.5`
  follow-up queue on 2026-09-04 as Lane C, carried by
  `ad-bug-state-busy-recovery-guidance`: the lock defect itself is already fixed
  (`rc.5` reclaims an ownerless lock), and what remains is not a contract the
  implementation failed to meet. `cli-design.md` fixes only the code's meaning and
  exit status, so a repair has to decide new user-visible behaviour — at least
  three decisions, which is why this is not a Lane A fix. First, whether the
  message names the specific lock, and how that reads against the existing rule
  that messages never render filesystem-path text, since a lock name is a file
  name under the state root. Second, whether it points at `agentdeck doctor`,
  which would also require widening `doctor`'s `checkLock`: it stats only
  `state.lock`, so a `scan.lock` holder currently answers `state_lock ok` and the
  pointer would mislead. Third, what the safe recovery path actually is now that
  ownerless reclamation is automatic. Promote into a bounded design before
  development rather than repairing the copy in place.

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
