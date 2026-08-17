---
status: active
created: 2026-08-17
---

# Switch Effectiveness Boundary — Architecture

Three contracts, one corrected premise. Every claim below about existing code
names the file and line it was read at, on `main` at
`8beacdb1a412fc4cbe59f84cbe76512ee2c41025`.

## The asymmetry this design is built on

`WriteClaudeConfig` (`internal/provider/config.go:638-671`) owns exactly two
fields and evaluates each one independently:

```go
if config.Endpoint == "" {
        delete(env, "ANTHROPIC_BASE_URL")     // :655
} else {
        env["ANTHROPIC_BASE_URL"] = ...       // :657
}
if config.Credential == "" {
        delete(env, "ANTHROPIC_AUTH_TOKEN")   // :660
} else {
        env["ANTHROPIC_AUTH_TOKEN"] = ...     // :662
}
```

Each field is written when its provider configuration value is present and
deleted when that value is absent. The three relevant shapes are therefore:

- a custom provider carries both an endpoint and a credential, so both fields
  are written;
- direct `official` carries neither, so both fields are deleted; and
- `official --via` carries a wrapper endpoint but no credential, so the endpoint
  is written while the credential is deleted.

The switch-effectiveness boundary therefore does not reduce to one shared
write/delete branch. For billing attribution, it is specifically the credential
deletion shared by the two `official` shapes that marks a switch the running
session cannot adopt.

The asymmetry is not in our code — it is in what a running client can observe. A
newly written environment value is read on the next request. A deleted one
cannot undo an authentication that already happened: the session presented that
token at startup and negotiated its capabilities against it. So:

| Selection | Endpoint field | Credential field | Reaches a running session |
| --- | --- | --- | --- |
| custom provider | written | written | Yes |
| `official`, direct | deleted | deleted | No |
| `official`, `--via` wrapper | written | deleted | Not for billing |

The wrapper row is why this design keys on the credential rather than on the
provider name. A `--via official` selection writes an endpoint and deletes the
credential (`ConfigMatchesOfficialWrapper`, `:109-141`, checks exactly that
shape). Its endpoint change is observable and its credential change is not, so
the session may reach a new endpoint while still presenting the old token. What
is billed, and at whose rate, is decided by the credential — so for attribution
this row behaves like the direct one, and both `official` rows take the same
treatment. `requirements.md`'s defect section states the same three rows.

**The discriminant is `config.Credential == ""`** — whether this switch deletes
the credential. Not the provider name, which would get the wrapper row wrong,
and not a comparison against the previous selection, which is not available at
the advisory site.

## Contract 1 — the switch advisory is direction-aware

`SwitchAdvisories` (`internal/provider/service.go:780-805`) returns
`claudeRestartAdvisory` (`:808`) for every Claude switch. That constant is
replaced by two, selected by the discriminant above.

| Direction | Advisory |
| --- | --- |
| credential written | `restart running Claude sessions: a running client reads its settings file live, so this switch can reach a session mid-conversation` |
| credential deleted | `restart running Claude sessions: a running session keeps the credential it authenticated with, so this switch does not apply to it until it restarts` |

The first is today's text, unchanged, because the direction it describes is the
direction it was true for. The second states the boundary rather than implying a
live change.

Both begin with the same imperative. The user's action is identical in both
directions — restart the session — and only the reason differs; an advisory that
changed its opening verb would suggest the required action changed too.

Unchanged in this contract:

- The conflict advisories for `official`
  (`env.ANTHROPIC_API_KEY`, `apiKeyHelper`, `:797-802`) keep their text,
  ordering, and position before the restart note. They describe sources
  AgentDeck does not own, which is a different concern from effectiveness.
- Advisory generation never fails a completed switch (`:777-779`), and no
  advisory carries a credential value.
- `codexRestartAdvisory` (`:807`) is untouched. Codex activates nothing without
  a restart, so its note has no direction to distinguish.

`SwitchAdvisories` currently takes `(client, name, configPath)`. It needs the
credential-emptiness of the selection being reported. Passing the resolved
`ClientConfig`, or a boolean derived from it at the one call site, are both
acceptable; `tasks.md` owns which, and the signature change is internal — no
command surface takes a new flag.

## Contract 2 — what `ClaudeConfigMatchesSnapshot` proves

`ClaudeConfigMatchesSnapshot` (`internal/provider/config.go:145-153`) reads the
settings file and compares the AgentDeck-owned fields. Its result is a fact
about the file and nothing else. Today's callers use it as though it were a fact
about the running client, which is the second half of this defect.

The function's behavior does not change. What changes is that the boundary is
stated, in its own doc comment and at the caller, so the next reader does not
re-derive the same wrong conclusion:

> A match proves the managed fields on disk equal the completed selection. It
> proves nothing about which credential a running client is currently
> presenting. For a selection that deletes the credential, the two can differ
> indefinitely: a match is exactly what deletion produces, whether or not any
> running session honored it.

This is a documentation-and-caller contract rather than a code change because
the function is correct for what it measures. Renaming it to imply a narrower
claim was considered and rejected: every caller would still have to decide what
to conclude, and the name would carry the reasoning instead of the reasoning
being written where the decision is made.

## Contract 3 — a credential-deleting switch records no route

`reconcileClaudeConfigChange` (`cmd/agentdeck/main.go:2930-2950`) reconciles the
file against the completed selection and calls
`RecordClaudeConfigChange(ctx, sessionID, snapshot, matched)`
(`internal/usage/routes.go:45-54`). That function already has the branch this
contract needs:

```go
if !matched {
        return s.recordSessionRoute(ctx, route, "unknown", "1", false)   // :51
}
return s.recordSessionRoute(ctx, route, runtimeProviderName(snapshot.Name), ...)  // :53
```

Today a switch to `official` takes the second branch: deletion makes the file
match, so `matched` is true and the route records `official` with its
multiplier. `priceForEvent` then adopts that multiplier verbatim
(`internal/usage/usage.go:2617-2621`) for every subsequent event in the session,
while the running client bills against the API key it still holds.

**A credential-deleting switch records no `ConfigChange` route at all.**

The provider is not unknown. It is the *previous* selection — the session is
still presenting the credential it authenticated with, and AgentDeck recorded
that selection when it was made. Writing `unknown` would discard information the
store already holds and would report a session as unattributable when it is
precisely attributable.

Recording nothing is what expresses that. `sessionRouteAt` positions an event
against the most recent route at or before its time
(`internal/usage/usage.go:2504-2510`):

```go
index := sort.Search(len(routes), func(i int) bool { return routes[i].observedAt.After(at) })
if index == 0 { return readSessionRoute{}, false }
return routes[index-1], true
```

With no new route written, every subsequent event in that session continues to
resolve to the route that was already in effect — the custom provider and its
multiplier, which is what the session is actually billing at. The absence of a
route is not a gap; it is the statement that nothing about this session's
routing changed, which is exactly true.

A session may also have no route of its own, since `RecordSessionRoute` writes
nothing without a completed selection (`internal/usage/routes.go:33-34`) and skips
`compact` (`:29`). `priceForEvent` then falls back to the provider timeline at
session start (`internal/usage/usage.go:2622-2634`), which resolves to the
selection the session started under — the same answer. `requirements.md`'s
"What a credential-deleting switch attributes to" states the full hierarchy and
the quality each case carries; this contract only decides that no row is written.

This also removes the need for a quality decision. There is no new row, so there
is no quality to assign, and nothing here touches
`usage-attribution-precision`'s redesign of that dimension.

The state does not persist indefinitely. A restart writes a fresh route from
`SessionStart` reading the completed selection
(`internal/usage/routes.go:22-40`), and `startup`, `resume`, and `clear` all
reach that path (`internal/usagehook/event.go:88`). So the old route governs
exactly the interval where the old credential is in effect, and the new selection
takes over at the boundary where the client actually adopts it. The advisory from
contract 1 is what tells the user that boundary is a restart.

What this deliberately does not attempt:

- **It does not verify that the running session is in fact still using the old
  credential.** No supported mechanism exists. The design rests on the mechanism
  in the asymmetry table above — a deletion cannot revoke an authentication that
  already happened — which is why `tasks.md` requires a real-session acceptance
  rather than treating unit tests as proof.
- **It does not distinguish a session that started before the switch from one
  that started after.** A `ConfigChange` hook fires for the session that observed
  the change, so by construction it predates the change. A session started after
  gets its route from `SessionStart`, which this defect never touched.
- **It does not suppress the route for a credential-writing switch.** That
  direction is observable and its route is correct today.

### Why not `unknown`

An earlier draft of this contract recorded the unknown route here, reasoning that
if we cannot observe which credential the process holds then the provider is
undeterminable. That conflates two different things: we cannot *observe* it, but
we can *derive* it, because a credential-deleting switch does not change what a
running session presents and the previous selection is recorded. Reporting
`unknown` would have replaced a correct attribution with an absent one and made
the topic's own goal — never present a figure at a confidence the evidence does
not support — fail in the other direction, by withholding a figure the evidence
does support.

## Corrected premise in the `v0.6.0` attribution architecture

`docs/topics/usage-attribution-precision/architecture.md:53-55` reads:

> **Claude** activates immediately, and mid-session changes are separately
> recorded as `ConfigChange` routes. Positioning by event time inside that route
> sequence yields the provider actually in effect.

This is the stated ground for promoting Claude `ConfigChange` routes to `exact`.
Immediate activation holds for a credential-writing switch and not for a
credential-deleting one, so the conclusion does not follow for the second, and
positioning by event time inside the route sequence yields the provider the
*file* named, not the one in effect.

That document's `Review` cell is unticked and it has no review record
(`../usage-attribution-precision/tasks.md:49-54`), so this is a draft correction
rather than a reopened verdict, and no completion evidence is invalidated. The
passage is amended to state the direction dependency and to point at this
topic's Contract 3: a credential-deleting switch creates no resulting
`ConfigChange` route. The retained prior route continues to supply its already
stored provider, multiplier, and quality (currently `estimated`); when no
session route exists, the unchanged session-start fallback applies. This topic
therefore assigns no new quality, while that topic remains free to redesign the
quality of the evidence sources that actually exist.

## Verification

L2 for the route contract: it changes a persisted attribution value that cost
output reads. L1 suffices for the advisory text alone, but the two ship
together, so L2 covers both.

Automated tests can prove the file-level behavior — which advisory text each
direction produces, and which route a reconcile writes. They cannot prove the
claim this topic exists for, which is about a process that authenticated before
the file changed. `tasks.md` owns the manual procedure; `requirements.md`'s
acceptance boundary requires it, and it is not satisfiable by unit tests over
`settings.json`.
