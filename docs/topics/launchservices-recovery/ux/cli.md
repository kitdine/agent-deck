---
status: active
created: 2026-10-04
updated: 2026-10-04
---

# LaunchServices Recovery — CLI

Candidate framework and final surface; awaiting independent batch review.
Requirements are [requirements.md](../requirements.md); each requested field is
provisioned by [architecture.md](../architecture.md). The primary product
prototype is [Cli.jsx](../../../../prototype/src/Cli.jsx), `launchservices` tab.
Its specimen data is [launchservices.js](../../../../prototype/src/launchservices.js).

## Hierarchy and states

Keep the existing report summary and existing checks in their current order.
Append one `launchservices` check only after the normal completed doctor path.
State/database/extension early returns retain their original checks and partial
semantics; no added acquisition occurs on those paths. No stdin, prompt, ANSI,
pager, cursor control, new flag, command, or terminal dependency is introduced.

The diagnostic section presents status/code, scope/completeness, host and Widget
source reasons, quoted entries, next steps and the limitation, in that order.
One warning check contributes one warning regardless of entry count. The source
scope is returned host URLs and targeted PlugInKit matches, never all OS records.

| State | Code | Presentation |
| --- | --- | --- |
| Complete matching copies | launchservices_consistent | Scope-limited consistency; same-build copies are not a conflict |
| Complete different live builds | launchservices_conflict | Possible registration conflict; identify obsolete copies |
| Complete observed missing paths | launchservices_stale | Observed stale paths; verify obsolete identity first |
| Incomplete source/control/metadata | launchservices_unknown | Lead with uncertainty; preserve positive observed evidence |
| Unsupported platform or absent supported host | launchservices_not_applicable | Explain applicability; no OS-health claim |

Unknown takes precedence over conflict, then stale, then consistent. Empty host
or Widget output, failed positive control, invalid format, timeout, cancellation,
limits, unreadable metadata and absent canonical Widget never produce consistent.
Unknown details still show observed different builds or missing paths.

## Field requests and disposition

The CLI requests applicability/reason, stable scope, complete flag, source names,
source completeness/reasons and entries (path, canonical flag, state, version,
build). Architecture supplies them in optional `registration_details`. Build
and path are data, never commands. The existing report provides status/code,
resource/reason and counts. A recovery command, executable unregister example,
raw probe output, OS error prose, elapsed-time field and new GUI action are
refused for safety, stability and privacy. No field is persisted.

JSON uses the existing success envelope and exit 0 for diagnostic warnings.
Optional details are confined to doctor JSON. Desktop uses its existing explicit
HealthCheck DTO; it receives the stable check/status/code/resource/reason/counts,
with no registration details or action metadata for this check. Native decoder
allowlists recognize this resource and all five reasons, preserving ok for
consistent/not-applicable and warning for conflict/stale/unknown. Other unknown
tokens still fail closed. Existing health counts still include the check; an ok
check must not synthesize a desktop warning notice or a copy action.

## Manual guidance

For conflict/stale, print: `Verify which copy is obsolete and confirm the
canonical app and Widget. Quit obsolete app copies normally. Only then manually
unregister or remove the verified obsolete registration using its exact path;
protect the canonical app and Widget.` No generated command, shell quoting,
wildcard, reset, database rebuild, restart or elevation is supplied.

For unknown, print: `Inspect the canonical app and Widget and the incomplete
source before choosing any obsolete copy. Run agentdeck doctor --full again.`
Malformed or control-character paths remain escaped evidence and cannot become
an actionable target. Consistent says: `Matching copies are not a version
conflict within the returned-source scope.` Every applicable state concludes:
`A later diagnosis confirms observed registrations only. Check the Widget
separately.` Not-applicable states give their applicability reason alone.

## Character specimens and verification

The prototype renders consistent, conflict, stale, unknown and not-applicable
states as actual character output and a JSON specimen; state selection is
visible. The conflict includes a Unicode path; unknown includes escaped newline
and terminal-control data. These fixtures establish copy and hierarchy, not
native acquisition or Widget operation. Source identities belong in the review.

Text never truncates codes or path identities. Paths/version/build use Go `%q`
escaping, including ASCII control characters; JSON uses the standard encoder.
At 80x24, 40x10 and non-TTY/narrow dimensions the output remains the same logical
lines, with terminal wrapping or scrolling permitted. No layout-dependent
summary omission is allowed. Renderer tests compare text/JSON and a failing
writer, assert escaped controls and no recovery command, and prove no input read.
Desktop tests assert complete raw-path/build/output exclusion. Runtime tests,
ordinary-user native observations and bounded timing remain separate acceptance.
