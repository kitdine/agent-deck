---
status: active
created: 2026-09-21
updated: 2026-09-21
---

# Health Recovery — Menu-Bar Experience

## Purpose and stage

Make the existing menu-bar Health detail state the same cause-specific recovery
contract as the delivered CLI framework: the user sees what is unhealthy, which
AgentDeck resource owns the condition, what is safe now, and whether the copied
content diagnoses, synchronizes or only describes a manual prerequisite.

This is the stage-3 surface framework for the reviewed
[`requirements.md`](../requirements.md). Architecture still owns field
provisioning and native implementation. Final-surface reconciliation must return
here after architecture settles the wire and execution boundaries.

## Design authority and CLI boundary

The shared [`/prototype/`](../../../../prototype/) is the visual and interaction
authority for this surface and for the CLI recovery specimens. Design, later
implementation comparison and review use these stable deep links:

| Surface | URL pattern |
| --- | --- |
| Menu-bar | `?health=<scenario>&lang=zh|en&theme=dark|light&width=420|280` |
| CLI | `?surface=cli&health=<scenario>&theme=dark|light` |
| Menu-bar interaction probe | `?healthProbe=1&lang=en&theme=light&width=420` |
| CLI contract probe | `?surface=cli&healthProbe=1&health=stateLive` |

Both surfaces read stable resource/reason/action values from
`prototype/src/healthRecovery.js`. A screenshot or prose table cannot override
that model. If the prototype, this document and the delivered CLI contract
disagree, the disagreement blocks review until they are reconciled.

The CLI framework passed Re-review Round 2 and was delivered by signed commit
`bc6df888edb9dd342129865184ff0ca426819675`. Its document, Review, Beads and
CEv1 state remain unchanged. The prototype renders delivered byte-exact text and
required JSON fields as a read-only reference; architecture still owns the final
JSON container. This menu-bar design does not reopen the CLI task.

## Goals and non-goals

### Goals

1. Preserve one cause-first route from the notice strip into Health detail for
   live/uncertain locks and extension inventory recovery.
2. Distinguish copyable synchronization, copyable diagnosis, copied safe prose
   and conditions with no action.
3. Keep executable commands byte-exact and keep destructive lock-removal text
   out of command affordances.
4. Cover English and Simplified Chinese, 420 pt and 280 pt, light and dark
   appearances, keyboard use, VoiceOver order and large text wrapping.
5. Let a later refresh prove recovery; copying content never claims the
   condition was fixed.

### Non-goals

- Run `agentdeck extension scan`, `agentdeck doctor`, process termination or
  filesystem deletion from the menu-bar app.
- Add a wizard, modal confirmation, progress timeline, dismissal record or
  notification for these conditions.
- Expose lock tokens, unverified PIDs, native configuration, paths, SQLite text
  or discovery errors.
- Change the existing popover width, tab hierarchy, palette, footer or refresh
  coordinator.
- Change the delivered CLI contract or its completion state.

## Existing surface and conflict

The current app already provides the right container:

- a cause notice opens the secondary `HealthDetailView`;
- every check row has status text and may disclose cause/recovery prose;
- `recovery_command` renders in monospace with a copy button;
- Back/Escape returns to the unchanged panel; and
- the detail scrolls independently without expanding the main dashboard or
  moving the provider footer.

The current wire and view model provide only `name`, `status`, `code`, counts
and one generic `recovery_command`. That is insufficient here: it cannot tell a
diagnostic command from synchronization, cannot carry a manual prerequisite,
cannot state resource/reason/effect and cannot represent
`inventory_committed: true`. A generic “Copy recovery command” would also
mislabel diagnosis and encourage unsafe inference.

## Information hierarchy

The menu-bar path remains two levels.

1. **Notice strip:** one cause sentence per independent condition, with severity
   icon, text and disclosure chevron. It says what is wrong, not the command.
2. **Health detail header:** Back at leading edge, Health title and warning icon
   at trailing edge, preserving current keyboard cancellation.
3. **Check row:** localized check label, localized status, stable cause, safe next
   step, optional effect/manual prerequisite, then optional copy affordance.
4. **Source note:** doctor attribution below the rows.

For combined state-lock and stale-inventory conditions, the notice strip shows
two cause rows in resource order: lock first, extension inventory second. Health
shows both expanded checks in the same order. A generic “2 checks not passing”
row adds no information and is suppressed for prototype recovery scenarios.

## Presentation states

`health=` is a separate prototype axis from data `state=`, refresh `refresh=`
and quota `quota=`. Each scenario replaces only the health payload shown by the
specimen; the rest of the dashboard remains readable.

| Scenario | Stable reason/action | Notice | Health detail and action |
| --- | --- | --- | --- |
| `baseline` | Existing product sample | Existing health count behavior | Existing rows; no health-recovery contract asserted |
| `stateLive` | `state` / `lock_live` / `retry` | State is busy | Explain the active state operation; wait and retry; no copy action |
| `scanLive` | `scan` / `lock_live` / `retry` | Scan is running | Explain the active scan; wait and retry; no copy action |
| `legacy` | `state` / `lock_legacy` / `manual_prerequisite` | Legacy lock needs confirmation | Copy safe prose only; no `rm` command and no automatic deletion |
| `ownerUnknown` | `scan` / `lock_owner_unknown` / null | Ownership unknown | Fail closed; do not remove; no copy action |
| `extensionStale` | `extension_inventory` / `extension_stale_inventory` / `synchronize_inventory` | Inventory is out of date | Show effect and copy `agentdeck extension scan` |
| `discoveryFailed` | `extension_inventory` / `extension_discovery_failed` / `manual_prerequisite` | Discovery failed | Resolve discovery first; no sync command because the same prerequisite would fail |
| `syncIncomplete` | `extension_inventory` / `extension_fingerprint_update_failed` / `diagnose` | Inventory committed, fingerprint failed | State no rollback, copy `agentdeck extension doctor`, then decide whether to retry |
| `combined` | `lock_live` plus `extension_stale_inventory` | Two independent cause rows | Both checks remain expanded; only the independently safe sync command is copyable |

Healthy or cleared checks disappear on the next accepted snapshot. The app keeps
no local “recovered” badge, because a copied command is not proof that it ran.

## Action contract

The menu-bar app never executes these commands. It only copies content already
classified by the producer:

| `action_kind` | Visible affordance | Clipboard content | Safety rule |
| --- | --- | --- | --- |
| `synchronize_inventory` | Copy sync command / 复制同步命令 | Exact `recovery_command` | Only when successful discovery made synchronization applicable |
| `diagnose` | Copy diagnostic command / 复制诊断命令 | Exact diagnostic command | Never labeled Fix, Repair or Recovery |
| `manual_prerequisite` with safe copy text | Copy safety steps / 复制安全步骤 | Localized prose, not a shell command | May describe confirmation; must not contain a copyable destructive command |
| `retry` or null | No action button | Nothing | Waiting or uncertainty is not a command |

After a successful copy, the same button position exposes `Copied` / `已复制`
through one polite live region for 1.6 seconds. Focus does not move. The row,
notice and health status do not change. The user runs a copied command in
Terminal and uses the existing Refresh action; only a later accepted snapshot
may clear or reclassify the condition.

At 420 pt command and button share a row. At 280 pt they stack, the button fills
the row, commands wrap at safe boundaries and none is ellipsized. Prose always
wraps. The existing Health scroll owns vertical overflow; the footer does not
move.

## Copy contract

| Meaning | English | Simplified Chinese |
| --- | --- | --- |
| State-live notice | `AgentDeck state is busy` | `AgentDeck 状态正在被占用` |
| Scan-live notice | `An AgentDeck scan is running` | `AgentDeck 扫描正在进行` |
| Legacy notice | `A legacy state lock needs confirmation` | `旧式状态锁需要人工确认` |
| Unknown-owner notice | `Scan-lock ownership is unknown` | `无法确认扫描锁的归属` |
| Stale-inventory notice | `Extension inventory is out of date` | `扩展库存已过期` |
| Discovery-failed notice | `Extension discovery failed` | `扩展发现失败` |
| Incomplete-sync notice | `Inventory updated, but its scan fingerprint did not` | `库存已更新，但扫描指纹未更新` |
| Synchronization action | `Copy sync command` | `复制同步命令` |
| Diagnostic action | `Copy diagnostic command` | `复制诊断命令` |
| Manual action | `Copy safety steps` | `复制安全步骤` |
| Copy feedback | `Copied` | `已复制` |

Visible Health check labels are localized independently from stable wire names:

| Wire check name | English | Simplified Chinese |
| --- | --- | --- |
| `state_permissions` | `State file permissions` | `状态文件权限` |
| `state_lock` | `State lock` | `状态锁` |
| `scan_lock` | `Scan lock` | `扫描锁` |
| `extensions` | `Extension inventory` | `扩展库存` |
| `database` | `Core database` | `核心数据库` |
| `hook_deliveries` | `Hook deliveries` | `Hook 投递` |
| `state` | `State` | `状态` |
| `schema` | `Database schema` | `数据库结构` |
| `database_integrity` | `Database integrity` | `数据库完整性` |
| `pending_operations` | `Pending operations` | `待处理操作` |
| `provider_operation_state` | `Provider operation state` | `服务商操作状态` |
| `provider_configuration` | `Provider configuration` | `服务商配置` |
| `provider_credentials` | `Provider credentials` | `服务商凭据` |
| `project_attribution_gate` | `Project attribution gate` | `项目归因门禁` |
| `sessions` | `Session index` | `会话索引` |
| `session_sources` | `Session sources` | `会话来源` |
| `usage` | `Usage index` | `用量索引` |
| `usage_sources` | `Usage sources` | `用量来源` |
| `prices` | `Price catalog` | `价格目录` |
| `price_provenance` | `Price provenance` | `价格来源` |
| `unpriced_models` | `Unpriced models` | `未计价模型` |

An unknown future check name uses `Other health check` / `其他运行状况检查`
instead of showing its raw wire token. Positive counts on classified health
reasons are spoken as `Affected: <count>` / `受影响：<count> 项` in the disclosure's
accessible label; schema-version counts keep their separate version meaning.

The full localized cause, next, effect and prerequisite paragraphs live in
`prototype/src/i18n.js` and are rendered in the specimens below. Machine tokens,
commands and wire check names remain stable ASCII; only their visible labels are
localized.

## Data requirements for architecture

| UI element | Required data | Constraint |
| --- | --- | --- |
| Notice and row identity | typed `resource` and `reason` | Use the complete reviewed CLI vocabulary; unknown values fail closed |
| Status/severity | check status independent of action | Color never carries the only meaning |
| Action label | nullable typed `action_kind` | Do not infer it from command text |
| Sync action | nullable `recovery_command` | Present only when execution safely advances recovery |
| Diagnostic action | nullable diagnostic command | Separate from `recovery_command` |
| Manual action | localized safe prerequisite identifier/text | Never raw backend prose or a destructive shell command |
| Effect | bounded mutation summary | Inventory + fingerprint are one truthful visible boundary |
| Partial commit | `inventory_committed` for incomplete sync | Must say inventory changed and did not roll back |
| Stale count | bounded safe count | Extension IDs/configuration/paths remain private |

Architecture may choose the additive Desktop wire container and compatibility
defaults, but it must provision every element or return here with a reasoned
refusal. The Swift app must not parse localized CLI prose or execute shell text.

## Accessibility, input and motion

- Notice rows and copy buttons are reachable in visual order with visible focus.
- The Health Back button keeps Escape semantics; copying does not close Health.
- Check name/status form the row label/value. Expanded cause, next, effect and
  prerequisite follow once in the accessible reading order.
- Copy buttons name the action class, not merely “Copy”. `Copied` uses one
  polite, atomic live region and does not steal focus.
- Icons are supplemental. Warning/Failed/OK remain text; semantic colors support
  light, dark and Increase Contrast.
- Prose and commands wrap at 280 pt and large Dynamic Type. No executable command
  or resource ID is ellipsized.
- Reduce Motion removes any incidental feedback transition; no timing-only
  meaning or mandatory pointer gesture exists.

## Shared prototype specimens

The browser specimens establish hierarchy, bilingual copy, action labeling,
420/280 wrapping and shared CLI tokens. They do not establish native typography,
VoiceOver speech, Dynamic Type, AppKit pasteboard behavior, real lock liveness or
SQLite transaction semantics.

| Specimen | Deep link | What it proves |
| --- | --- | --- |
| Stale inventory, English/light/420 | `?health=extensionStale&lang=en&theme=light&width=420` | Cause → effect → exact sync command and bounded copy action |
| Legacy lock, Chinese/dark/280 | `?health=legacy&lang=zh&theme=dark&width=280` | Manual prerequisite wraps; no destructive command; button stacks |
| Combined, English/dark/420 | `?health=combined&lang=en&theme=dark&width=420` | Independent causes coexist; only safe sync action appears |
| CLI stale inventory | `?surface=cli&health=extensionStale&theme=dark&lang=en` | Delivered text and required JSON fields use the same stable tokens |
| CLI incomplete sync | `?surface=cli&health=syncIncomplete&theme=light&lang=en` | Post-commit failure keeps `inventory_committed: true` and diagnose semantics |

- [Stale inventory](prototype/health-recovery/menubar-extension-stale-en-light-420.png)
- [Legacy lock at 280 pt](prototype/health-recovery/menubar-legacy-zh-dark-280.png)
- [Combined conditions](prototype/health-recovery/menubar-combined-en-dark-420.png)
- [CLI stale inventory](prototype/health-recovery/cli-extension-stale-en-dark.png)
- [CLI incomplete sync](prototype/health-recovery/cli-sync-incomplete-en-light.png)

The content-bound inventory and browser summary are in
[`manifest.json`](prototype/health-recovery/manifest.json) and
[`checks.json`](prototype/health-recovery/checks.json).

## Verification design

| Risk | Observable oracle |
| --- | --- |
| Menu-bar and CLI tokens drift | Both read `healthRecovery.js`; CLI probe checks text/required JSON fields against every shared reason |
| Internal check names leak into UI | Popover probe renders state, scan and extension checks in both shipped languages, asserts the exact localized labels and rejects raw wire names |
| Diagnostic is labeled as repair | Scenario tests assert action-specific button labels and zero action for retry/unknown |
| Unsafe lock deletion becomes copyable | Legacy clipboard fixture contains prose and no shell command; unknown/live states have no button |
| Stale inventory has no recovery | Stale fixture exposes exactly one sync command and names inventory + fingerprint effect |
| Partial commit is presented as rollback | Incomplete fixture asserts `inventory_committed: true`, diagnose action and no rollback copy |
| Copy claims recovery | Copy probe asserts only feedback changes; Health remains open and status unchanged |
| Narrow layout loses content | 280 pt screenshot and overflow/truncation gauge; command/button stack without ellipsis |
| Accessibility regresses | Scoped WCAG 2A/AA axe, keyboard focus/copy/back path, native VoiceOver and Dynamic Type acceptance |
| Private/raw values leak | Fixtures and Swift tests reject tokens, PIDs, paths, native config and raw driver/discovery text |

Final browser evidence for this candidate: prototype build PASS; Popover probe
56/56; CLI probe 24/24; scoped axe 0 violations for three menu-bar specimens
and the CLI recovery surface; 280 pt and 420 pt gauges report `NO OVERFLOW`.

Native implementation later requires Swift model/view tests, localization key
inventory, pasteboard tests with exact safe content, snapshot rendering at both
widths/languages/appearances, keyboard and real VoiceOver/Dynamic Type manual
acceptance. Architecture and task decomposition select the final project
verification level.

## Assumptions and architecture questions

The UX decisions above are complete. Architecture must still decide:

- the additive Desktop wire types and safe defaults for older helpers/apps;
- whether manual prerequisites are keyed localized copy or another typed safe
  representation;
- how aggregate doctor orders lock and extension checks; and
- which refresh generation proves a copied external action changed the snapshot.

These are provisioning decisions. They must not change action meanings, copy
labels, destructive safeguards, state order or prototype specimens without a
new surface reconciliation.

## Review boundary

Framework review decides whether the shared prototype, state/action model,
bilingual copy, 420/280 specimens, accessibility rules, data requirements and
verification oracles completely cover both reviewed cause tracks. It does not
approve Desktop wire types, Swift/Go implementation, native acceptance or final
surface reconciliation.
