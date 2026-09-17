---
status: active
created: 2026-09-10
updated: 2026-09-11
---

# CLI Scan Interaction Contract

Interaction draft for the revised snapshot-performance scope. Read requirements
and architecture for behavior. This document owns visible CLI presentation;
`tasks.md` owns readiness. The shared-prototype specimen was built and browser-
verified on 2026-09-11; current review is recorded in [the CLI review](../reviews/ux-cli-scan.md).

## Commands and completion

`agentdeck scan` waits for both domains; `--scope usage` or `--scope session`
waits/displays that domain while both execute. Invalid scope fails before launch.
Legacy usage/session scan results retain their envelope, fields and exit rules.
No-scan/read-only commands never create a request. Ctrl-C detaches this caller.
Do not add a global cancel control in this iteration.

## Presentation states

All CLI copy is English. TTY progress uses stderr, a bounded single status line
and no more than five updates/second. Non-TTY output has no cursor control;
quiet suppresses progress. Final machine output stays on stdout with its existing
schema; progress is never inserted into legacy JSON or snapshot chunk streams.

| State | Required text/content |
| --- | --- |
| Owner/round wait | Waiting for current scan |
| Inventory | Checking source files; omit unknown totals |
| Processing | Domain plus committed/total files and skipped count |
| Domain success | Final selected result; return when selected domains are terminal |
| Domain failure | Existing compatible error result; never call old data a fresh success |
| Detached | Stop this subscriber's output; no later background console writes |

Counts mean committed work; decoding or an empty channel is not completion.
Never display paths, transcript text, credentials or guessed percentages. At
narrow widths shorten optional counts before hiding domain/stage/error meaning.
No interactive focus trap or new mandatory prompt is introduced.

## Specimen and acceptance

The shared product prototype now includes a scan tab on its existing CLI surface.
Open `?surface=cli&scan=waiting`. The stage selects scope (all/usage/session),
TTY/pipe/quiet/JSON output, 40/80 columns and scan phase. These are specimen
controls, not extra product flags. Next stage and play model ongoing background
work; Ctrl-C detaches the foreground and freezes its output.

- [Session waiter, 40 columns](prototype/scan/cli-session-en-dark-40.png): usage is
  complete while the session subscriber remains active; only session progress is
  visible on stderr, stdout is empty until completion.
- [Usage receipt, JSON](prototype/scan/cli-usage-json-en-light-80.png): scope usage
  exits successfully while sessions are still processing. The receipt is frozen;
  a later background outcome cannot rewrite an exited invocation.

Text specimen final output is `Scan complete: usage.`, `Scan complete: session.`
or `Scan complete: usage and sessions.`; failure is `Scan incomplete: sessions
failed.` with `Session scan failed.` on stderr. The illustrated failure case has
usage successful and sessions failed. Scope usage therefore still succeeds.
Exit 0/1/130 means selected success/selected failure/subscriber interruption in
this new command specimen. Legacy commands retain their actual existing exits.

The JSON specimen is the proposed new scan result payload/envelope projection:
command=scan, data.scope, data.usage/data.sessions and partial for requested
failure. It is not an example of changing legacy JSON schemas. Worker IPC and
legacy envelope integration remain native implementation contracts, not browser
behavior proven by this specimen.

Browser verification: 11 independent visible-behavior assertions per language
(22 total), including scope completion, truthful processing status, stdout/stderr
separation, output freeze, non-TTY, quiet error preservation, parseable JSON,
detachment and narrow output. CLI content remains English even in the Chinese
stage. New CLI surface axe WCAG 2A/AA audit reported 0 violations/0 incomplete.

[Specimen manifest](prototype/scan/manifest.json) binds the shared source,
lockfile, screenshots and [browser check results](prototype/scan/checks.json).
Manifest SHA-256: `cf26b5d5f7afd30bf57eca9f4e7fa6c373eaec0b786e2be5eb02c7de973def8f`.
Browser proof does not establish real process lifetime, IPC, stream bounds,
SQLite outcomes or native exit semantics; task 3 retains their acceptance.
