---
status: historical
created: 2026-08-18
updated: 2026-08-18
retired: 2026-09-01
---

# Settings Window

Surface: the AgentDeck macOS settings window.
Task: `menubar-experience`.
Consumes the foundation runtime in [`../architecture.md`](../architecture.md) and
the preference domain described in [`menubar.md`](menubar.md#preferences).

## Purpose

Define every preference the desktop app exposes, its default, what it does when
the operating system refuses it, and its copy in both shipped languages.

This document is normative. Product behavior must follow this contract even when
an earlier implementation differs from it.

## Why this is a separate surface

The documentation rule is that two candidate documents reviewed by the same
question against the same evidence are one document. This is not that case. The
reading surface is judged by whether every data state has a presentation rule;
the settings window has no data states at all — it is judged by whether every
preference has a default, an idempotent effect, and an honest failure
presentation. Different question, different evidence, so it is its own file.

It is also a separate **window**, not a page inside the popover. On macOS `⌘,`
opens a window; a preference pane that lives inside a transient popover
disappears the moment the user clicks elsewhere to check something, which is
exactly what someone configuring an application does.

## Scope

- the settings window, its geometry, and its dismissal;
- four preferences: launch at login, periodic refresh, menu-bar value, menu-bar
  scope;
- the login-item failure presentation;
- English and Simplified Chinese copy;
- accessibility and keyboard behavior.

## Non-goals

- **Any update check.** Withdrawn from this version; see
  [`menubar.md`](menubar.md#non-goals). No control, no copy, no version
  comparison, and no release page appears here.
- credential, provider, or wrapper management — none of it is a preference;
- log level, data directory, or any path control;
- import, export, or reset of preferences;
- a preference for anything the reading surface can already do inline, such as
  the client or period filter.

## Decisions

| Area | Decision |
| --- | --- |
| Window, not pane | Opened by `⌘,` and by the menu-bar item's menu. Standard title bar, close only. Escape closes it. |
| Storage | The application's own `UserDefaults` domain. Never `~/.agentdeck`, never the App Group cache. |
| Defaults | Everything that costs the user something — background work, or a number permanently on screen — defaults to the quiet choice. Login item and periodic refresh default off; the menu-bar value defaults to cost because that is the reason the app exists. |
| Reported state | Every control reports the real post-operation state, never the requested one. |
| Failure | A refused preference stays visibly off and says why, in place. It never silently reverts and never opens a modal. |
| Menu duplication | The menu-bar value also appears in the item's own menu. It is the one preference a user changes situationally — sharing a screen — so it is reachable without opening a window. Both controls write the same key. |

## Preferences

| Preference | Default | Behavior |
| --- | --- | --- |
| Launch at login | Off | Registers or unregisters `SMAppService.mainApp`. Idempotent. Installs no daemon, agent, or privileged helper. The control reflects `SMAppService.mainApp.status`, including `requiresApproval`. |
| Periodic refresh | Off | When on, the coordinator schedules the next refresh from the snapshot's `next_refresh_at`. Never runs while the app is inactive-suspended; a missed interval refreshes once on reactivation. |
| Menu-bar value | Cost | Selects what the menu-bar item renders: cost, tokens, or icon only. |
| Menu-bar scope | All clients | Selects whether the item follows the popover's client filter or always reports all clients. |

Each preference carries one line of explanatory copy. That line states the
consequence, not the mechanism — `Registered as a system login item; no
background daemon is installed` is a consequence a user can decide about, while
naming the API is not.

### Login-item failure

`SMAppService` can refuse registration, and the honest presentation of that is
the one case here worth specifying in full:

1. The switch returns to, and stays at, the real status — which is off.
2. A failure line appears under the label, in the warning tone, with an icon and
   text both.
3. Nothing else on the window is disabled, and no modal appears. The user was
   configuring one thing; one thing failed.
4. The line clears on the next successful change, not on a timer.

When the status is `requiresApproval`, the switch shows on and the line states
that the system is waiting for approval. That is not a failure and must not be
worded as one.

## Geometry

| Property | Value |
| --- | --- |
| Window width | 460 pt |
| Height | Fits content; the window does not scroll at default Dynamic Type |
| Grouping | Three groups — General, Menu bar — each with a title, separated by 18 pt |
| Row | Label and explanatory line on the leading side, control on the trailing side, separated by a hairline |

Label-leading and control-trailing is the platform's own settings idiom. The
explanatory line sits under the label rather than beside the control, so the
control column stays a single scannable edge.

## Rendered specimen

The reviewable prototype is [`../prototype/interactive-v7/`](../prototype/interactive-v7/):
`http://127.0.0.1:4175/?settings=1` opens this window directly, and `lang` and
`theme` address every combination. The prototype's `?probe=1` run asserts that
each switch toggles, that the menu-bar value modes change what the item renders,
and that Escape closes the window.

```text
┌──────────────────────────────────────────────────┐
│ ● ○ ○              AgentDeck Settings            │
├──────────────────────────────────────────────────┤
│ GENERAL                                          │
│ Launch at login                          (  ●)   │
│ Registered as a system login item; no            │
│ background daemon is installed                   │
│ ────────────────────────────────────────────     │
│ Periodic refresh                         (●  )   │
│ Refreshes at the time the snapshot suggests;     │
│ when off, only opening the panel or refreshing   │
│ manually updates it                              │
│                                                  │
│ MENU BAR                                         │
│ Shows                        [ Cost ] Tokens Icon│
│ Switch to icon only when sharing your screen     │
│ ────────────────────────────────────────────     │
│ Scope                    [ All clients ] Follow  │
│ When following the panel, picking Codex there    │
│ also narrows the menu bar to Codex               │
└──────────────────────────────────────────────────┘
```

Login item refused:

```text
│ Launch at login                          (●  )   │
│ Registered as a system login item; no            │
│ background daemon is installed                   │
│ ⚠ Could not change the login item                │
```

## Copy

| Element | `en` | `zh-Hans` |
| --- | --- | --- |
| Window title | `AgentDeck Settings` | `AgentDeck 设置` |
| Group | `General` | `通用` |
| Group | `Menu bar` | `菜单栏` |
| Launch at login | `Launch at login` | `开机时启动` |
| — its line | `Registered as a system login item; no background daemon is installed` | `通过系统登录项注册，不安装后台守护进程` |
| — refused | `Could not change the login item` | `无法修改登录项` |
| — awaiting approval | `Waiting for approval in System Settings` | `等待在系统设置中批准` |
| Periodic refresh | `Periodic refresh` | `定时刷新` |
| — its line | `Refreshes at the time the snapshot suggests; when off, only opening the panel or refreshing manually updates it` | `按快照给出的下次刷新时间刷新；关闭时只在打开面板和手动刷新时更新` |
| Menu-bar value | `Shows` | `显示内容` |
| — its line | `Switch to icon only when sharing your screen` | `共享屏幕时可切到仅图标` |
| — options | `Cost` / `Tokens` / `Icon only` | `成本` / `Token` / `仅图标` |
| Menu-bar scope | `Scope` | `统计范围` |
| — its line | `When following the panel, picking Codex there also narrows the menu bar to Codex` | `跟随面板筛选时，面板里选了 Codex，菜单栏也只显示 Codex` |
| — options | `All clients` / `Follow panel filter` | `全部客户端` / `跟随面板筛选` |

## Accessibility

- Each switch is a `switch` role with an accessible label naming the preference,
  and its explanatory line is its accessible description.
- Each mode selector is a radio group; the group carries the preference name and
  each option is announced with its selected state.
- The login-item failure is announced when it appears, and is text plus icon,
  never color alone.
- Every control is reachable by keyboard in visual order, with a visible focus
  indicator. Escape closes the window from anywhere in it.
- The window opens with focus on the window itself, not on the close button — a
  focus ring on a close control invites dismissing what was just opened.

## Security and privacy

- The window reads and writes only the application's own preference domain.
- No preference names a path, a credential, an endpoint, or a client
  configuration file.
- Enabling the login item is the only operation that touches system state, and it
  goes through `SMAppService` with no shell and no privileged helper.
- Preference changes log a fixed event code and no dynamic value.

## Verification

Verification level L3, shared with `menubar-experience`.

### Swift

- Each preference: default value on a clean domain, persistence across a
  relaunch, and idempotent enable and disable.
- Login item: success, refusal, and `requiresApproval`, each asserted to report
  the real status rather than the requested one, with the refusal presenting its
  line and leaving the rest of the window enabled.
- Menu-bar value: each of the three modes changes what the item renders, with
  `icon` rendering no text at all.
- Menu-bar scope: `follow` tracks the popover's client filter, `all` ignores it.
- Both controls that write the menu-bar value agree, in both directions.
- Localization: both catalogs resolve every key here; no key falls back to its
  identifier.

### Manual checklist

Recorded in the review record with the observed result for each item on
macOS 26:

1. `⌘,` opens the window from the popover and from the item's menu, and Escape
   closes it.
2. VoiceOver announces each switch with its label, state, and explanatory line.
3. Full keyboard access reaches every control in visual order.
4. Denying the login item in System Settings leaves the switch off with its
   failure line, and the rest of the window usable.
5. `en` and `zh-Hans` both render without truncation, including the longest
   explanatory line.
6. Light and Dark appearance both keep every control and the failure line
   legible.

## Data requirements

This surface reads no wire snapshot and no App Group projection. Its entire state
is the application's own preference domain plus `SMAppService.mainApp.status`.

The only cross-surface coupling is the menu-bar value and scope, which
[`menubar.md`](menubar.md#menu-bar-item) renders.

## Downstream contracts

`desktop-app-contract` reconciles the delivered preference behavior into
`docs/specs/cli-design.md` and `docs/specs/cli-manual.md`.

`unified-desktop-distribution` owns whether the shipped bundle can register a
login item at all; an unsigned local build may not, and the failure presentation
above is what the user sees when it cannot.

## Approval boundary

Design approval authorizes this contract to become the implementation and review
authority for the settings window within `menubar-experience`. It does not
authorize repair, review, commit, push, signing, notarization, release, or work
on later desktop tasks.
