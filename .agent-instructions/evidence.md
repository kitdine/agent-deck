# Completion Evidence and Neo4j Project Memory

Read this file when crossing a new Document, Task, Topic, Integration, or Release
completion boundary, creating or invalidating relevant evidence, or materially
depending on durable project knowledge. Ordinary phase changes and Beads-only
coordination do not by themselves create evidence work.

## Completion Evidence and Neo4j / 验收证据与 Neo4j

The shared `completion-evidence/v1` profile owns canonical record semantics and
provider operations. This file supplies AgentDeck's namespace and work-unit
bindings. The workflow Skill owns phase execution; [Review Records](review-records.md)
owns review artifacts, and [Beads](beads.md) owns coordination.

A compatible Neo4j MCP exposes Cypher operations and the `CEv1Node` /
`CEv1Relation` mapping. Discovery establishes availability, not correct gate
behavior, namespace scope, or write authorization. Resolve these separately.
Use namespace `github.com/kitdine/agent-deck`; do not inspect or mutate another
repository's evidence as part of this workflow.

Discover configured capability once per session and reuse the result until it
changes or an operation fails. An enabled provider that is unavailable or denies
required access is not an absent provider. Follow [Toolchain](toolchain.md) and
[AGENTS.md](../AGENTS.md#runtime-contract--运行时契约) for availability and
transport limits; never bypass the MCP by writing its backend directly.

| Boundary | unit_kind | work_unit_id | Content identity |
| --- | --- | --- | --- |
| Reviewed document | `document` | `<topic>:<document>` | HEAD plus scoped document blob; include required specimen identity |
| Task implementation | `task` | `<topic>:<task-anchor>` | Committed Git tree, or HEAD plus scoped candidate fingerprint before commit |
| Whole topic | `topic` | `<topic>` | Applicable Git tree or scoped candidate identity |
| Integration | `integration` | Resolve through the integration contract | Parent/result content identities under [Branching](branching.md) |
| Release | `release` | Version | Release content identity plus the required preflight SHA |

These are repository WorkUnit bindings, not extra canonical node kinds. Preserve
historical identifiers and values; do not rename old records to normalize them.

A Lane A fix uses `unit_kind: task` and `work_unit_id: fix:<slug>`. It has no
containing topic gate, but its own required task gate still applies. Do not use
NOT_REQUIRED merely because no topic exists. After an authorized commit, bind
its reusable evidence to the immutable delivered content and finish the existing
WorkUnit lifecycle through the permitted operation; do not equate a Beads closure
with this evidence result.

### Gate evaluation and reuse

Before claiming a required boundary complete:

1. Resolve the actual WorkUnit, repository namespace, applicable target content,
   expected criteria, and direct hierarchy from their authorities. Document
   dispatch tasks and CEv1 WorkUnits need not map one-to-one.
2. Confirm the expected required criterion set and its `requires` relationships.
   The provider evaluates the graph's declared criteria; it cannot infer an
   unwritten or undeclared requirement. A missing WorkUnit, missing relationship,
   empty set, or optional-only set cannot prove a required gate complete. NOT_REQUIRED is a project decision that no gate applies,
   not a substitute for missing graph data.
3. Call the Neo4j gate with `namespace`, `work_unit_id`, and
   `target_content_state`. The last two are stable node IDs in that namespace;
   a commit SHA, digest, or description is not a substitute for resolving the
   ContentState ID. A top-level passing observation must bind that exact target
   through `observed_at`; inspect its lineage and applicability diagnostics.
4. Query each newly crossed boundary from inner to outer. Query a containing
   unit only when its actual hierarchy reaches that boundary. Do not rerun all
   child checks or gates merely because a parent or release is being evaluated.
5. Record only new or changed evidence/impact decisions and query the affected
   gate again. Preserve reusable exact-state results instead of repeating tests.

Reuse requires passing evidence for the right criterion and compatible subjects,
valid dependencies, no applicable contradictory supersession or invalidation,
and resolved candidate impacts. Follow the shared profile's `change`,
`impact_assessment`, `depends_on`, `rolls_up`, `may_invalidate`, `preserves`, and
`invalidates` semantics rather than treating the basic three-edge path as a
complete reuse algorithm.

For a different target ContentState, do not silently relabel old observations or
assume equal digests are sufficient. After the profile's scope-aware assessment,
record an explicitly target-bound roll-up referencing still-valid earlier
evidence when appropriate. The child observations retain their original state.
A preserves decision alone does not retarget an old top-level observation, and
new roll-up records must not manufacture a passing outcome without support.
This reuses verified checks rather than rerunning them merely for a new phase.

| Result | Meaning and action |
| --- | --- |
| VERIFIED | All expected required criteria have reusable passing evidence for the target; continue only within existing authorization |
| NOT_VERIFIED | Evidence is missing, insufficient, invalidated, or has unresolved impacts; keep the boundary open and report the scoped gap |
| FAILED | Current applicable evidence disproves a required criterion; keep the boundary open and report it |
| BLOCKED | Required evidence or an authoritative operation cannot be obtained; report the exact prerequisite |

Check the result envelope, not only its status string. The Neo4j provider reports
`missing_work_unit`, `missing_target_state`, `invalid_criterion`, and
`no_required_criteria` as NOT_VERIFIED input/data gaps. Missing query parameters
are errors, never an implicit target selection. Transport or permission failures
must not be treated as an empty result. Follow the provider contract for the
returned diagnostics; invalidated/unresolved lists identify affected top-level
evidence IDs, whose lineage explains the underlying assessments.

Check review and delivery state independently. Review PASS, CEv1 VERIFIED,
commit, and Beads status remain distinct. Evidence work grants no product-change,
Git, release, or deployment authority.

Fallback follows the configured authoritative profile and project policy. Never
create an empty local store to mask an unavailable external provider. Use a mirror
only when it is already configured, complete, synchronized, and permitted by the
applicable policy; otherwise report BLOCKED. Report any permitted fallback and
its reason explicitly. Do not install or initialize a provider as an incidental
step in a review.

### Record shape / 记录形状

Resolve the current profile, schema, provider mapping, and parameterized templates
before writing. Store inventory is diagnostic evidence, not a schema authority:
an old last-used timestamp does not prove that a record kind was retired, and a
recent timestamp does not prove that a shape is valid. Mixed timestamp types
also prevent treating a simple sort or aggregate as a migration/version check.

The basic evidence path is:

```text
work_unit ──requires──▶ criterion ◀──satisfies── evidence ──observed_at──▶ content_state
```

- Canonical node/edge vocabulary follows the profile. The basic path uses
  lowercase `work_unit`, `criterion`, `evidence`, and `content_state`; change and
  impact records are additional profile kinds used for reuse decisions.
- Use stable IDs, the project namespace, profile identifier, and canonical
  attributes. Keep provider query properties consistent with `attributes_json`;
  do not infer identity from Neo4j internal IDs.
- Evidence identifies its observed ContentState through the canonical
  `observed_state_id` and `observed_at` relation. If a provider also uses a
  `target_content_state` property, follow its declared mapping; a descriptive
  string is not a substitute for a valid state reference.
- Use lowercase outcomes `pass`, `fail`, `blocked`, or `not_verified` as defined
  by the profile. Only applicable passing evidence satisfies a criterion.
- The ContentState records the relevant commit/tree or scoped fingerprint and
  required dependency, configuration, toolchain, environment, or specimen
  identity. Keep raw logs, diffs, plans, and artifacts outside the graph. Record
  the necessary check facts directly; cite an original's URI and digest only
  when it meets the durable-reference rules below.
- Preserve established digest recipes for existing subjects. For an uncommitted
  document, the established form is SHA-256 of `head=<HEAD-SHA>;document=<blob>`;
  append `;prototype=<manifest_sha256>` when the specimen is part of the state.
  Do not silently change the identity recipe or conflate content identity with
  the node ID that references it.
- Evidence and observed content are append-only facts. New observations use new
  identities and explicit supersession; do not overwrite an old observation to
  make it describe new content. Follow permitted WorkUnit lifecycle operations
  for administrative state rather than treating them as verification evidence.
- An upsert template alone does not guarantee append-only behavior. Before
  reusing an existing record ID, check that it denotes the same fact or an
  explicitly permitted lifecycle update. Report incompatible identity/payload
  changes instead of silently overwriting them.
- Run the provider's relation preflight before each relationship batch. Reject
  missing endpoints and identity conflicts; compare processed and submitted
  counts, then read back/query the affected gate. Use stable IDs for retries.

Bind evidence after required local status synchronization reaches its final
content state. If synchronization changes a reviewed document or specimen,
evidence for the earlier state cannot simply be relabeled; add the correct
state/evidence or an explicit permitted reuse assessment.

### Durable artifact references

These rules apply to formal project documents, review records, retained Beads
comments and CEv1 records. Storage location does not change a reference's
retention requirements.

- Do not cite temporary or scheduled-for-deletion artifacts as references.
  This includes `/tmp` and `/private/tmp` outputs, worktree-local generated
  files, build caches and retention-bound CI artifacts. Writing the path as
  plain text, calling it temporary, or copying it into Beads or Neo4j does not
  make it an acceptable reference.
- Capture the minimum facts needed to understand the observation directly in
  its owning record: inspected content identity, method or reproducible command,
  relevant measurements or failure details, result and limitations. A future
  reader must not need a temporary file to understand the finding or conclusion.
  A reproduction command may create fresh temporary output; it must not imply
  that a previous temporary output remains available for inspection.
- Retain an original only when the summary cannot adequately support its use,
  such as a necessary visual specimen. Before citing it, verify access, content
  identity and a defined retention commitment covering the record's intended
  use. Use a pinned Git object or another verified durable location; a URL,
  object ID or checksum alone does not establish retention. Do not archive every
  test run, log or build by default.
- Beads may retain concise, self-contained diagnostic or disposition facts and
  stable pointers under its coordination rules. CEv1 retains structured check
  facts, content identities and evidence relationships under the shared profile.
  Neither is a raw-log or binary attachment dump, nor a substitute for the
  authoritative review record.
- Before authorized cleanup, inspect references to the artifacts in that cleanup
  scope. Preserve necessary facts and any essential original under the rules
  above; do not require preservation of unrelated process output. Update live
  pointers to verified durable replacements, or retire those pointers when the
  original is unnecessary. Do not leave a record claiming an original is retained
  at a location being deleted.
- For existing historical records or immutable observations, preserve the
  original content identity, finding and verdict. Add a dated correction or
  disposition identifying the obsolete citation by its owning record/section;
  do not rewrite historical observations or repeat a dead path as a usable
  reference. If necessary facts cannot be recovered, state that limitation
  rather than inventing a replacement account. Follow the provider profile for
  any supported reference correction; do not overwrite immutable CEv1 evidence.
- Cleanup does not by itself turn a historical passing observation into a
  failure. However, a later verification or reuse decision that requires an
  unavailable original must obtain adequate new evidence or report the gap;
  the old verdict or digest cannot establish present retrievability.

### Failure handling and provider compatibility

Classify a failed write before retrying. Permission denial follows
AUTHORIZATION_WAIT in AGENTS.md; do not reformulate a denied action to bypass it.
A specific statement-shape or validation error may justify a corrected,
authorized idempotent retry. For ambiguous partial failure, inspect the exact
intended records before repeating writes. Unknown causes or unavailable providers
keep the evidence boundary open.

Provider behavior must satisfy the profile, including target-state checks and
missing-WorkUnit handling. A query that returns VERIFIED for an unresolved ID or
ignores the requested target is not a valid completion gate for this project.
Do not compensate by claiming success from the status word; surface the provider
contract gap and require scoped repair/verification before relying on it.

Historical rationale for the evidence-shape rule is retained in commit
`acbd91186d27bfb9f799f7a6751d09fa718403f5`. It is provenance, not the current schema.

## Neo4j Project Memory / Neo4j 项目记忆

Durable project memory is optional and non-authoritative. It explains decisions
and reusable lessons; CEv1 establishes evidence for a defined content state.
Memory availability does not determine whether the evidence provider is usable.

- Query memory when a relevant prior decision or recurring failure may help.
  Do not query it mechanically for unrelated, self-contained tasks.
- Preserve supported, reusable, non-sensitive decisions and their rationale;
  do not duplicate current repository state, ordinary progress, gate results,
  raw logs, session content, credentials, or private source material.
- A useful memory adds durable context that a future reader could not recover
  merely by inspecting the repository at that time. Keep measurements and their
  content identities in evidence or review records, not in project memory.
- Use `agent-deck:` namespaced entities and small idempotent additions within
  the standing scope. Explicit user limits and higher-priority runtime memory
  policies still apply. Do not interpret this repository convention as permission
  to edit a different memory store or another namespace.
- Correct outdated memory with a dated, supported observation when permitted.
  Deletion, replacement, broad imports, and cross-namespace changes need explicit
  authority. Prefer correction over erasing historical context.
- Repository code, tests, configuration, living documents, and Git history remain
  authoritative. Report material conflicts instead of silently following stale
  memory. If memory is unavailable, continue from repository sources and report
  the limitation only when it affects the task.
