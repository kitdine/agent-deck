# Completion Evidence and Neo4j Project Memory

Read this file only when current work crosses a new Task, Plan, or Release
completion boundary, creates or invalidates completion evidence, or materially
depends on prior durable project knowledge.

## Completion Evidence and Neo4j / 验收证据与 Neo4j

AgentDeck opts into `completion-evidence/v1` whenever the current environment
exposes a compatible provider or local store. The current Neo4j binding is
capability-based: a Cypher read/write interface whose schema contains
`CEv1Node` and `CEv1Relation` is a configured provider even when no tool is
named `completion-evidence`.

- Before using fallback evidence rules, probe available capabilities once per
  repository session. When a Neo4j Cypher interface is available, inspect its
  schema and treat the CEv1 labels above as provider discovery.
- Use repository namespace `github.com/kitdine/agent-deck`. Never read, write,
  merge, invalidate, or delete another repository's CEv1 records as part of an
  AgentDeck workflow.
- Before claiming a Task, Plan, or Release complete, query every newly crossed
  WorkUnit boundary from inner to outer with its `work_unit_id` and exact
  `target_content_state`.
- A CEv1 WorkUnit is an evidence scope, not a dispatch task. It may cover work
  coordinated through several Beads tasks, and one Beads task may reference
  several WorkUnits. Store only correlation identifiers across systems.
- Beads-only changes do not change `target_content_state` and do not trigger
  CEv1 discovery, queries, invalidation, or upserts. A CEv1 result does not
  mutate Beads or replace repository phase and review state.
- If local verification creates new evidence or an impact assessment, record it
  with idempotent CEv1 upserts and query the gate again. Only `VERIFIED` closes
  the evidence gate; `NOT_VERIFIED`, `FAILED`, and `BLOCKED` keep the WorkUnit
  open according to the development workflow contract.
- Bind evidence to the exact content identity required by this repository's
  evidence-reuse rules. Use the Git tree for committed content. For an
  uncommitted review candidate, record HEAD plus the scoped blob or diff
  fingerprint, then relate or re-record the immutable Git tree if an authorized
  delivery later creates a commit.
- Provider discovery and gate reads are read-only diagnostics. Idempotent CEv1
  node and relationship upserts limited to this repository namespace are
  standing workflow authority when they record evidence produced within the
  already authorized phase. This authority does not permit schema changes,
  deletions, arbitrary Cypher writes, or changes to evidence owned by another
  repository.
- Fallback is allowed only when no compatible provider or local store exists.
  Report it explicitly as `COMPLETION_EVIDENCE_FALLBACK: <reason>`; never
  silently degrade. A configured provider that is unreachable, rejects a
  query, or lacks required write authority is `BLOCKED`, not absent.
- CEv1 synchronization never grants commit, push, release, deployment, or
  product-change authority. A Review `PASS` remains distinct from a
  `VERIFIED` WorkUnit gate.

## Neo4j Project Memory / Neo4j 项目记忆

`neo4j-memory` is a non-authoritative, durable project-knowledge aid. It is a
separate concern from `completion-evidence/v1`: project memory explains durable
decisions and relationships, while CEv1 proves an exact content state passed a
defined gate.

- Query relevant project memory when resuming work or investigating prior
  architecture decisions, release policies, workflow conventions, or recurring
  failures. Do not query it mechanically for unrelated, self-contained work.
- Record only durable, reusable, non-sensitive knowledge supported by an
  authoritative repository source, or knowledge the user explicitly requests
  to preserve. Suitable facts include approved architecture and product
  decisions, stable workflow and release conventions, reusable diagnostic
  conclusions, known pitfalls, and relationships among plans, versions,
  components, and contracts.
- Use namespaced entity names such as `agent-deck:project`,
  `agent-deck:decision:<topic>`, `agent-deck:plan:<topic>`, and
  `agent-deck:version:<version>`. Prefer small, idempotent entity, observation,
  and relationship updates over duplicated narrative documents.
- Bounded idempotent creates, observation additions, and relationship upserts in
  the `agent-deck:` namespace are standing knowledge-synchronization authority
  when the fact already has an authoritative source. Deletion, replacement,
  broad imports, and mutation of another namespace require explicit approval.
- Do not store Task verification evidence, PASS/FAIL state, raw command output,
  ordinary progress logs, current Git status, credentials, session content,
  private source paths, or other sensitive data. Task and release evidence
  belongs in CEv1, not project memory.
- Code, tests, configuration, living documentation, and Git history remain the
  source of truth. If project memory conflicts with repository truth, follow
  the repository and report the stale memory before correcting or deleting it.
- Missing or unavailable `neo4j-memory` does not affect CEv1 gates and does not
  trigger completion-evidence fallback. Continue with repository sources and
  report the memory limitation only when it materially affects the task.
