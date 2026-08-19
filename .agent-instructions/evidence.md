# Completion Evidence and Neo4j Project Memory

Read this file only when current work crosses a new Document, Task, Topic, or
Release completion boundary, creates or invalidates completion evidence, or
materially depends on prior durable project knowledge.

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
- Before claiming a Document, Task, Topic, or Release complete, query every newly
  crossed WorkUnit boundary from inner to outer with its `work_unit_id` and exact
  `target_content_state`.
- The four boundaries and their bindings. Repository and evidence terminology are
  identical; there is no mapping to remember:

  | Boundary | `unit_kind` | `work_unit_id` | `target_content_state` |
  | --- | --- | --- | --- |
  | A reviewed document | `document` | `<topic>:<document>` | HEAD SHA plus that document's blob hash |
  | A task's implementation | `task` | `<topic>:<task-anchor>` | Git tree |
  | A whole topic | `topic` | `<topic>` | Git tree |
  | A release | `release` | the version | Git tree plus the preflight SHA |

  A merge additionally records `unit_kind: integration`; see
  `.agent-instructions/branching.md`.

- Write `unit_kind` in lowercase. Historical nodes carry mixed casing and the
  retired value `plan`; leave them as they are, since they record work that
  already completed, and use `topic` for anything new.
- A document boundary is crossed when its review reaches `Verdict: PASS`, so a
  frozen requirement or an approved design is queryable rather than only narrated
  in a review record.
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

### Record shape / 记录形状

The rules above are the contract. They do not describe how a record is shaped,
and a writer who reconstructs that by memory writes nodes the gate cannot see.
On 2026-08-19 that happened: six evidence nodes were written in the single-node
form this store retired on 2026-08-17, and the gate query — which walks
`work_unit -> criterion <- evidence` and matches `outcome = 'pass'` — returned
nothing from them. They looked recorded and proved nothing.

**Inspect the store before writing.** The convention lives in the graph, not in
memory and not in this file's history. One read settles it:

```cypher
MATCH (n:CEv1Node) WHERE n.ce_namespace = 'github.com/kitdine/agent-deck'
RETURN n.kind AS kind, min(n.recorded_at) AS first, max(n.recorded_at) AS last,
       count(*) AS c ORDER BY last DESC
```

A `kind` whose `last` is old is retired vocabulary, however many rows it has.
Mixed property types make this trap worse: ordering by a property that is a
string on some nodes and `DATE_TIME` on others sorts by type before value, so
`ORDER BY … DESC LIMIT n` can hide every recent record. Aggregate, do not
sample.

The current shape, as the store holds it:

```text
work_unit ──requires──▶ criterion ◀──satisfies── evidence ──observed_at──▶ content_state
```

- Node `kind` is lowercase: `work_unit`, `criterion`, `content_state`,
  `evidence`. Every node and relation carries
  `profile: 'completion-evidence/v1'`, `ce_namespace`, and `attributes_json`
  holding the same properties as JSON. Timestamps use `datetime()`, never a
  string.
- `content_state` is its own node, identified by its `subject_digest`, and it is
  where `head`, `git_commit`, `scoped_blob_fingerprint`, `subject_path`, and
  `manifest_sha256` live. An evidence node's `target_content_state` is a foreign
  key to that node's `id`, not a description of the state.
- The digest is computed from the bound identity, so two writers agree on the
  same state without coordinating:

  ```bash
  printf '%s' 'head=<HEAD-SHA>;document=<blob>' | shasum -a 256
  # append ';prototype=<manifest_sha256>' when a specimen is part of the state
  ```

- **`outcome` MUST be lowercase `pass`.** The gate filters on that exact value;
  `PASS` records a node the gate will never count.
- Records are append-only facts. When content changes, add a new
  `content_state` and `evidence` and point the new evidence at the old one with
  a `supersedes` relation. Do not rewrite an existing node — the previous state
  is what makes reuse decisions auditable, and this store's write path may
  reject the update anyway.
- Upsert with the profile's own templates: `UNWIND $nodes` for nodes, the
  relation preflight that reports `missing_endpoint` / `identity_conflict`, then
  `UNWIND $relations`. Compare the returned count with the submitted count, then
  re-run the gate query. Recording is not finished until the gate answers.

**Record after the round's own status synchronization, not before.** A review
round ticks a matrix cell and writes a current-state paragraph in the document
it reviewed, which changes that document's blob. Evidence bound to the
pre-synchronization blob is stale the moment the round finishes. Bind to the
final blob, and if evidence was already recorded against the earlier one,
supersede it rather than leaving two live records.

A denied write is not automatically `BLOCKED`. Retry as a smaller idempotent
upsert first: an environment may reject one statement's shape while permitting
the same intent expressed as the profile's template. Report `BLOCKED` only after
that, and say which statement was refused.

证据记录的形状以库中现状为准，不以记忆为准；写入前先聚合查询 `kind` 与其最后使用
时间，被淘汰的词汇不因历史条数多而正确。当前为四类小写节点加三种关系，
`outcome` 必须小写 `pass`，`content_state` 是独立节点且 `target_content_state`
是指向它的外键，`subject_digest` 由上面那条 `shasum` 得出。记录只追加，内容变化用
`supersedes` 而非改写。证据应绑定本轮状态同步之后的最终 blob，并在写完后用门禁
查询自查。写入被拒时先改用更小的幂等 upsert 重试，再决定是否报告 `BLOCKED`。

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
  conclusions, known pitfalls, and relationships among topics, versions,
  components, and contracts.
- The durability test: record a fact only when it would still be true and useful
  in a session holding none of the current context, **and** could not be derived
  by reading the repository at that later time. A decision and the reason it was
  chosen pass both. The repository's present state passes neither — it is
  derivable, so recording it only creates something that can go stale.
- The line against evidence follows from that test. A conclusion about why an
  approach failed is durable; the measurement that produced it is evidence.
  Record the conclusion here and leave the measurement in CEv1 or the review
  record. "The gate scans a diff, so a committed violation never fails" is
  memory; "the gate passed on tree X" is not.
- Writing is not gated on asking. Bounded creates and observation additions in
  the `agent-deck:` namespace are standing authority, so record a qualifying
  fact when it is established rather than deferring it to a confirmation.
- Correct a stale entry by adding a dated correcting observation. Deletion and
  replacement need explicit approval, so an addition that supersedes is the
  available repair, and it preserves what the earlier reader believed.
- Use namespaced entity names such as `agent-deck:project`,
  `agent-deck:decision:<topic>`, `agent-deck:topic:<topic>`, and
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
