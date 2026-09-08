# Runtime Toolchain

Read this file when a required capability is missing, misbehaving, or being
configured. `AGENTS.md` is the short entry point; this file owns capability
availability and runtime diagnostics. [Evidence](evidence.md) owns gate semantics
and records; [Project Rules](project-rules.md) owns verification scope.

Declare capability requirements rather than machine-specific endpoints, ports,
credentials, or install paths. Resolve those from the operator's effective
configuration and the applicable installation authority. Do not change a working
configuration merely to match an example server name or path in documentation.

## MCP servers / MCP 服务

MCP connection configuration is operator-owned. This repository does not provide
a `.mcp.json`. Typical server labels below identify capabilities, not a requirement
to rename equivalent tools or assume one transport on every machine.

| Capability | Typical server | Without usable access |
| --- | --- | --- |
| CEv1 query and authorized record operations | `neo4j` | Required evidence remains unresolved under Evidence's failure/fallback rules; never silently skip the gate |
| Optional durable project knowledge | `neo4j-mem` | Continue from repository sources; memory availability does not control CEv1 gates |
| Indexed symbol/call-path lookup | `codegraph` | Use `rg`/`fd` for the needed scope; do not create an index without authorization |

Distinguish configuration, installation, current-session tool exposure, and
successful operation. A reachable backend does not prove that the current client
has a usable MCP tool; an exposed tool does not prove a query or write succeeded.
Use the client's supported reconnect or tool-refresh mechanism when available.
If access cannot be refreshed in the current session, report that limitation and
the required reconnect/new-session step rather than repeatedly probing the same
endpoint. Do not assert that every client always connects only once.

Never route around an unavailable MCP by directly writing its backend. Provider
schema, namespace, authorization, target-state binding, and data-gap handling
remain under Evidence. A NOT_VERIFIED result for missing graph records is not a
transport outage; a successful RPC is not proof of a VERIFIED gate.

For indexed code, use the project's CodeGraph routing before broad structural
search. If it does not cover the requested script or metadata, inspect the named
source directly. Graph relationships guide investigation but do not prove runtime
configuration, SQL behavior, or client integration.

## Hooks / 钩子

The repository registers `scripts/hooks/beads-consistency.py` on
UserPromptSubmit (scope capture) and Stop (scoped diagnostics):

| Runtime | Repository registration | Blocking transport |
| --- | --- | --- |
| Claude Code | `.claude/settings.json` | Blocker JSON |
| Codex | `.codex/hooks.json` | stderr and exit code 2 |

These entries are part of the effective Hook chain, which can also include
user-level and plugin registrations. Check the active runtime's full applicable
chain before attributing a generic Hook error to this repository script.

The observer reuses the installed shared workflow command matcher and extracts
the project's explicit `<topic> / <subject>` or `fix / <slug>` scope. It never
starts a phase. Scope state is keyed by repository, runtime, and session; a new
unmatched request clears it, while an exact continuation retains it. A stale
turn, absent scope, or unavailable parser produces no blocking diagnostic.
Ordinary requests without an explicit supported scope remain unclassified;
neither dirty files nor an assignee name supplies session ownership.

Normal Stop checks only the selected subject, or the whole topic when the user
explicitly selects that topic. Repository-wide clean-tree and stale-vocabulary
checks remain available through the explicit, non-blocking audit command:

```bash
python3 scripts/hooks/beads-consistency.py --runtime codex --audit
```

Use `--runtime claude` for that client. This audit does not create or claim work.
Repeated scoped reports are suppressed while their notes and relevant document
content remain unchanged; resolved and subsequently recurring mismatches are
reported again. Silence means no attributable new diagnostic, not workflow PASS
or a completed evidence gate. Local scope state uses
`$XDG_STATE_HOME/agentdeck/beads-hook` (default `~/.local/state/agentdeck/beads-hook`);
isolated tests can override `AGENTDECK_BEADS_HOOK_STATE_DIR` and the read-only
parser binding `AGENTDECK_WORKFLOW_HOOK`.

The consistency Hook reports possible disagreement; it does not mutate Beads.
Its current implementation and focused tests establish what it actually checks.
Read a report as diagnostic input, then bind it to the exact record, content
state, task, and active user scope before acting. A dirty path alone does not
establish that the current actor authored it or owns the task.

Reconcile a confirmed, in-scope mismatch under the already granted authority in
[Beads](beads.md). A diagnostic about another task does not authorize claiming it,
changing its verdict, or rewriting its evidence. Explicit read-only or scope
limits remain effective. Report an out-of-scope or contradicted diagnostic rather
than manufacturing a matching state, and do not suppress a valid in-scope problem.

Use Beads for command forms, store location, and status vocabulary; do not
reconstruct them from Hook output or historical task descriptions. The script
uses `.agent-instructions/beads.md` as its repository marker. Its emitted message
and exit behavior, including recursion handling, require runtime-specific tests;
a source comment or silent invocation alone is insufficient proof.

## Shared workflow and handoff Hooks

The workflow and handoff Skills work without their optional Hooks. Their source
packages own routing and output contracts; installed copies and runtime links
must match the authorized source when a repair is delivered.

The workflow Hook distinguishes progress from phase completion. Claims and writes
can satisfy its anti-idling guard while the route remains unfinished. Use the
Skill's current completion and continuation contract, including the current
phase token after compaction; do not infer completion from silence or a successful
tool call. Do not duplicate receipt grammar in this file.

The handoff Hook requires configured synchronization rules. Without a rule it
does not enforce automatic synchronization, while explicit Skill use still
finds the project's current authorities and pointers. Do not invent a handoff
file or a timestamp-only update to satisfy a Hook.

For a Hook failure, use the smallest evidence chain that identifies the layer:

1. Identify the runtime, event, registered command, exit status, and relevant
   stderr or structured output. Distinguish failure from an intentional blocker.
2. Resolve the active source or plugin cache, installed file, and runtime link.
   A previously working version or cache path is not evidence for the current one.
3. Check the implicated interpreter, dependency, path, or permission operation.
   A process health check alone does not prove that the failing Hook can load.
4. Repair only the established, authorized boundary. If shared source code must
   change, update its maintenance source and verify the intended installation.
   Restoring missing dependencies from an existing lockfile does not by itself
   require a source change. Do not alter credentials, unrelated settings, or
   broad permissions just to make validation pass.
5. Test relevant event payloads with isolated state; perform real-client
   acceptance when required and authorized. Label those evidence types separately
   and preserve the source/install content identity used by the checks.

Do not execute real-session Hooks directly as a diagnostic shortcut. Use isolated
fixtures or normal lifecycle invocation under the applicable authorization.

## Required command wrappers / 必用命令包装

| Operation | Required entry point |
| --- | --- |
| Go tests | `scripts/run-go-test.sh` with the required package/filter arguments; runner options and log handling are documented in Project Rules |
| Beads reads and writes | The current actor-qualified wrapper documented in Beads; never substitute bare `bd` or guess the deployment path |
| Verification selection | Project Rules' L0–L4 matrix and affected-subsystem scope |

[Makefile](../Makefile) owns build/verification target implementations. Do not
copy an aggregate into every phase: `make verify` and `make release-verify`
contain multiple checks and are used only when the selected scope requires them.
Resolve source, test, and dependency paths before invoking commands rather than
reusing a stale machine-specific command from an old report.

## Workflow command syntax / 工作流命令语法

The `development-workflow` Skill's `references/protocol-commands.md` owns command
matching. Before emitting a next instruction, apply the self-check in
[AGENTS.md](../AGENTS.md#runtime-contract--运行时契约) against the current Skill.
The command must be usable as pasted; do not maintain a second parser here.

Use the project's subject scope, such as `work-signals / architecture.md`, or
`<topic> / reviews/<record>.md / <finding IDs>` for the applicable repair scope.
Resolve the record's actual subject and history before selecting that scope.
A generated next instruction never grants authority to run the next phase.

## Local-only runtime files / 仅本地的运行时文件

The repository includes only these two runtime registration files under
`.claude/` and `.codex/`: `.claude/settings.json` and `.codex/hooks.json`.
The `.gitignore` patterns use `dir/*` so these files can be re-included.

| Path | Role |
| --- | --- |
| `.claude/settings.json`, `.codex/hooks.json` | Tracked repository Hook registrations; they are not the entire user/plugin configuration |
| `.claude/settings.local.json` | Local permissions and overrides; not a shared product contract |
| `.claude/RESUME.md` | Local session checkpoint, not project handoff authority; do not act on an expired checkpoint reference |
| `.codegraph/` | Optional local index; its absence is not an instruction to generate one |
| `output/` | Local generated exports, kept out of the source distribution |

Resolve local state through its owning runtime; do not read authentication or
session files merely to infer configuration. Preserve unrelated local files.
`CLAUDE.md` remains a symlink to `AGENTS.md`, not an independently maintained copy.

Historical toolchain explanations remain available in commit
`36e4b0e87f0ac0eda603e022afd52db26517a043`, file
`.agent-instructions/toolchain.md`. Treat that snapshot as provenance rather than
proof of current client behavior or deployment configuration.
