---
status: active
created: 2026-09-08
updated: 2026-09-10
---

# Subscription Quota — Settings Window

Scope: the subscription-quota group this topic adds to the settings window, and
the disabled-state treatment the group's internal dependencies force. The
existing General and Menu bar groups are unchanged.

This surface exists because three of this topic's requirements are user
decisions, not product defaults: quota reading is opt-in, alerts are opt-in and
off by default, and the Claude status-line route needs explicit consent before
AgentDeck writes to `~/.claude/settings.json`.

## Where the specimens come from

The specimen is the prototype at [`/prototype/`](../../../../prototype/).
**Where this document and the prototype disagree, the prototype is right.**

| What | URL |
| --- | --- |
| The settings window, defaults | `?settings=1&lang=zh` |
| In `en` | `?settings=1&lang=en` |
| The overflow and truncation gauge | `?settings=1&lang=zh&measure=1` |

Settings is a separate window, not a popover page — on macOS the thing `⌘,`
opens has never lived inside a popover. Every control here maps to a real
preference key; none is invented for the specimen.

## The default state is the contract

All three switches are off, and that is the specimen's default. It is not a
placeholder: `requirements.md` requires that with notifications disabled no
reminder is evaluated or delivered, and that AgentDeck does not modify
`~/.claude/settings.json` without explicit consent. The prototype's
`DEFAULT_PREFS` encodes it, because the default value is the only copy of that
promise a running product will actually read.

## What turning the parent off actually does

`requirements.md` clauses 1 and 2 make `quotaProbe` the topic's single kill
switch, and the settings window is where a user learns that. Off means nothing
is read by any trigger — a manual refresh included — nothing is written to
`~/.claude/settings.json`, and no alert is evaluated. It does **not** mean the
observations already read are discarded: they are kept and simply stop being
displayed, so turning the switch back on is instant rather than a wait for the
next probe.

That last part is deliberately not in the hint copy. A hint that tried to carry
the retention rule as well as the scope and the credential promise would be
three sentences under a switch, and the fact it would be explaining is one the
user meets on the surfaces rather than here — the quota tab says `未读取`, and
the figure that returns when they switch it back on carries its own real age.
The hint's job is to say what the switch reads and what it does not read.

## Dependency, shown as disabled rather than hidden

The group's controls are not siblings; they form a chain:

```
读取额度 (quotaProbe)
├── 启用 Claude 状态栏通路 (quotaStatusline)
└── 额度提醒 (quotaAlerts)
    ├── 提醒阈值 (quotaThresholds)
    └── 窗口重置时提醒 (quotaResetNotice)
```

A dependent control whose parent is off renders **disabled**, not hidden, and
disabled is visually distinct from off. Two identical grey switches, one off and
one unavailable, teach the user they turned something off that they never
touched. Hiding is worse still: a control the user cannot see reads as a
capability the product does not have.

The treatment is reduced opacity on the control plus a lighter label and hint,
with `disabled` on the button so it is not focusable and announces as
unavailable. The reason it is unavailable is legible from the group: its parent
switch is directly above it.

## Consent must state its consequence

The status-line switch is the only control in this product that writes to a file
another tool owns. Its hint therefore names the exact file, the exact key, what
happens to any existing value, and what turning it off does:

> 写入 ~/.claude/settings.json 的 statusLine，并串接你已有的命令；关闭后恢复原配置
> · 将串接：python3 ~/.claude/statusline.py

The chained command is read from the user's current configuration and shown
before they consent. When there is none, the hint says so instead — `当前没有已
配置的 statusLine` — so the sentence is never a promise about a command that does
not exist. **Consenting to an action whose consequence is not visible is not
consent**, and a user who already has a status line is entitled to know their
own command is about to be wrapped rather than replaced.

## The read-interval control

Three choices — 5m, 15m, 30m — with the hint stating the rule that makes the
number meaningful: a manual refresh reads quota together with the snapshot, and
this interval governs only background refresh. Without that sentence the control
reads as a delay imposed on the user's own refresh, which is the opposite of the
behaviour.

The values are a deliberate design choice, not a range: a free-form interval
invites a one-minute setting that turns a menu-bar refresh into a subprocess
spawn per minute, which `requirements.md` rules out.

## The threshold control

Three choices — 75 %, 90 %, both. Both is the default because a single threshold
either warns too late to change plans or too early to be worth an interruption.
The values match the bar tone thresholds used on the other two surfaces, so a
bar turning warning and an alert firing are the same event rather than two
systems with their own opinions.

## Copy

| Key | `zh-Hans` | `en-US` |
| --- | --- | --- |
| `settings.quota` | 订阅额度 | Subscription quota |
| `settings.quotaProbe` | 读取额度 | Read quota |
| `settings.quotaProbeHint` | 只对 provider 为 official 的客户端读取；调用本机已登录的客户端命令，不读取任何凭证 | Only for clients whose provider is official. Invokes your already-signed-in client command; reads no credential |
| `settings.quotaInterval` | 读取间隔 | Read interval |
| `settings.quotaIntervalHint` | 手动刷新时随快照一起读取；后台刷新按此间隔单独计时 | A manual refresh reads quota with the snapshot; background refresh keeps this separate, slower interval |
| `settings.quotaStatusline` | 启用 Claude 状态栏通路 | Enable the Claude status-line route |
| `settings.quotaStatuslineHint` | 写入 ~/.claude/settings.json 的 statusLine，并串接你已有的命令；关闭后恢复原配置 | Writes statusLine into ~/.claude/settings.json and chains your existing command; turning it off restores the previous configuration |
| `settings.quotaStatuslineChained(cmd)` | 将串接：{cmd} | Will chain: {cmd} |
| `settings.quotaStatuslineNone` | 当前没有已配置的 statusLine | No statusLine is currently configured |
| `settings.quotaStatuslineWriteRefused` | 写入 ~/.claude/settings.json 失败，开关保持关闭。检查该文件的权限后再打开一次即可重试。 | Could not write ~/.claude/settings.json, so the switch stayed off. Check the file's permissions, then turn it on again to retry. |
| `settings.quotaStatuslineRestoreIncomplete` | 已移除 AgentDeck 写入的 statusLine，但原有配置在此期间被改过，没有自动还原。请手动检查 ~/.claude/settings.json。 | AgentDeck's statusLine was removed, but the previous value had changed in the meantime and was not restored. Check ~/.claude/settings.json by hand. |
| `settings.quotaAlerts` | 额度提醒 | Quota alerts |
| `settings.quotaAlertsHint` | 默认关闭。开启后每个窗口每次跨过阈值只提醒一次 | Off by default. When on, each window notifies once per threshold crossing |
| `settings.quotaThresholds` | 提醒阈值 | Alert thresholds |
| `settings.quotaResetNotice` | 窗口重置时提醒 | Notify when a window resets |
| `settings.quotaAlertsNotificationsDenied` | 系统已关闭 AgentDeck 的通知，额度提醒不会显示。 | Notifications for AgentDeck are turned off in System Settings, so quota alerts will not appear. |
| `settings.quotaAlertsOpenNotificationSettings` | 打开通知设置 | Open Notification Settings |
| `notification.quotaTitle(client)` | {client} 额度 | {client} quota |
| `notification.quotaThresholdBody(window, used, threshold)` | {window} 已用 {used}%（提醒阈值 {threshold}%） | {window} at {used}% (alert threshold {threshold}%) |
| `notification.quotaResetBody(window, used)` | {window} 已重置，当前 {used}% | {window} has reset — now at {used}% |
| `notification.quotaWindowFallback` | 额度窗口 | Quota window |

The probe hint carries "不读取任何凭证" / "reads no credential" deliberately. It
is the one claim in this topic a user cannot verify by looking, it is the
boundary the operator drew on 2026-09-08, and a settings screen that asks for
permission to read account data should say what it will not touch.

## When notifications are not allowed

Quota alerts are posted by the app under AgentDeck's own name (architecture.md
C10), so whether they appear is decided by the user's notification permission
for AgentDeck. Operator decision, 2026-09-16:

- **Permission is asked when 额度提醒 is turned on**, not at launch. The system
  prompt then appears right after the user asked for alerts, which is the only
  moment its question makes sense.
- **A refusal does not turn the switch back off.** The user asked for alerts;
  the setting is kept, and nothing is recorded as sent, so turning notifications
  on later starts delivering without another trip to this window.
- **While alerts are on and permission is denied or switched off**, the alerts
  field's failure row shows `settings.quotaAlertsNotificationsDenied` in the
  warning tone, followed by a `settings.quotaAlertsOpenNotificationSettings`
  button that opens AgentDeck's page in System Settings. The row is re-checked
  when the window becomes active, so returning from System Settings clears it
  without a restart. With alerts off the row is empty whatever the permission
  is.

This row reuses the same failure-row component as the two write outcomes below.
It is a warning rather than an error: nothing failed in AgentDeck, and the
setting took effect. The prototype specimen does not render this state; its
presentation is settled here and verified on the native window.

## When the write does not go through

This is the only switch in the product that writes to a file another tool owns,
so it is the only one with two ways to end badly — and they are different
endings, not two shades of one. An earlier version of this document left both to
the implementer, with the second one written down as something the specimen does
not settle. That was the wrong place to leave it: what a surface shows when an
operation fails is a UX decision, and an implementer forced to invent it has
every reason to reach for the shorter answer, which is to show the switch on and
say nothing.

| Outcome | What actually happened | The switch | The line |
| --- | --- | --- | --- |
| **Write refused** | Nothing. `~/.claude/settings.json` could not be written — permissions, a read-only volume, a lock. | Stays **off**. It never flickers on. | Error tone. Names the file, says the switch stayed off, and says that turning it on again is the retry. |
| **Restore incomplete** | Half. AgentDeck's own `statusLine` was removed, but the recorded prior value no longer matched what was in the file, so nothing was written back over what the user had edited. | Goes **off**, correctly — the disable did happen. | Warning tone. Says the removal succeeded, says the previous value was not restored, and sends the reader to the file. |

The distinction is the whole design. A refused write must not leave the switch
on, because a switch that is on is a promise that Claude Code is now chained to
AgentDeck's command; showing it on after nothing was written is the interface
lying about a file the user cannot see from here. Conversely a restore conflict
must not read as a failure to turn off — it turned off — or the user will keep
clicking a switch that is already where they want it.

**Both use the field's existing failure row**, the same one the launch-at-login
refusal uses: a live region that is present and empty before the failure so the
message is announced when it arrives, an icon so the state does not rest on
colour alone, and clearing on the next successful change rather than on a timer.

Reusing that row is not laziness. A second failure idiom in a window this small
would teach the reader that the two rows mean different kinds of thing, when the
only difference that matters is the one in the table above.

Colour is the second carrier, not the first — `--bad` for the refusal, `--warn`
for the conflict, with the icon inheriting it. It is worth naming because the
first attempt at this row shipped both messages in plain grey: the hint style
that dresses every label in this window, `.settings-label small`, outweighs a
single-class tone by CSS specificity and quietly won. The copy was right, the
icon was there, the class was on the element, and the one layer that says *how
bad is this* never rendered. The tone is now asserted at the row itself, so the
window's own hint styling cannot take it back.

**Retry is the switch itself**, not a separate button. The action the user
wanted is "turn this on"; the retry is the same action, and a dedicated retry
control would sit there permanently as a scar from one bad attempt. The error
copy says so explicitly, because a retry affordance nobody recognises is not one.

**Neither failure disables anything.** The dependent controls below are greyed
by the parent switch's state and nothing else; a failure that also greyed them
would tell the reader the feature is gone when one permission fix would bring it
back.

## Data requirements

| Requested | Why |
| --- | --- |
| The user's current `statusLine` command, or its absence | The consent hint must name what will be chained before consent is given |
| Whether the write succeeded, and the prior value | Turning the switch off restores it; a failed write must not leave the switch on |
| Which of the two failures occurred, as a distinguishable result rather than a boolean | The surface says different things for "nothing happened" and "half happened", so one flag cannot carry both |

`architecture.md` owns where the prior value is stored, how a restore behaves if
the user edited the file in between, and how the file is protected while it is
written. This surface owns what the window then shows, which is the table above.

## Rendered specimen

Indexes `?settings=1&lang=zh`, at defaults.

```
┌────────────────────────────────────────────────────────┐
│ ● ○ ○                AgentDeck 设置                    │
├────────────────────────────────────────────────────────┤
│ 通用                                                   │
│ 开机时启动                                     ( ○   ) │
│ 定时刷新                                       ( ○   ) │
│                                                        │
│ 菜单栏                                                 │
│ 显示内容                        [ 成本 ][Token][仅图标]│
│ 统计范围                     [ 全部客户端 ][跟随面板筛选]│
│                                                        │
│ 订阅额度                                               │
│ 读取额度                                       ( ○   ) │
│ 只对 provider 为 official 的客户端读取；调用本机已登录  │
│ 的客户端命令，不读取任何凭证                           │
│ 读取间隔                              [ 5m ][15m][30m] │
│ 手动刷新时随快照一起读取；后台刷新按此间隔单独计时     │
│ 启用 Claude 状态栏通路                         ( ○   ) │  ← 禁用
│ 写入 ~/.claude/settings.json 的 statusLine，并串接你已  │
│ 有的命令；关闭后恢复原配置 · 将串接：python3 ~/.claude/ │
│ statusline.py                                          │
│ 额度提醒                                       ( ○   ) │  ← 禁用
│ 默认关闭。开启后每个窗口每次跨过阈值只提醒一次         │
│ 提醒阈值                        [75%][90%][75% · 90%]  │
│ 窗口重置时提醒                                 ( ○   ) │  ← 禁用
└────────────────────────────────────────────────────────┘
```

The three marked switches are disabled because 读取额度 is off. Turning it on
enables the status-line and alerts switches; turning 额度提醒 on enables the
reset-notice switch. The threshold segmented control is always readable, because
seeing what the thresholds are is useful before deciding to enable alerts.

## Verification

| Check | Command | Result |
| --- | --- | --- |
| No overflow or truncation, both languages | `?settings=1&lang=zh&measure=1` | `NO OVERFLOW` |
| The failure row fits when it is showing, both languages × both appearances | Walk to each failure, then measure the row against the window | Two lines, except the `en` restore-incomplete row at three; every one inside the window and not clipped |
| Both failure outcomes, asserted with real events | `?probe=1&lang=zh`, `?probe=1&lang=en` | Every settings assertion passes in both languages; the run's only failure is the sessions work-signal banner, which is not this surface |
| The two tones actually render, both languages × both appearances | Walk to each failure, then read the row's **computed** colour | `--bad` and `--warn` resolved per appearance; never `--dim`; the icon inherits |
| Single-file build | `npm run build:single` | Passes |

**Row height is a measured result, not a target.** The window is a fixed
460 px, so the wrap does not move with the viewport, and the `en`
restore-incomplete copy needs a third line to say that the removal succeeded and
the previous value did not come back. Three lines is the right outcome for that
sentence: shortening it to hold a line count would drop either half of a message
whose whole job is to distinguish them. What the row must satisfy is fitting
inside the window without clipping, and it does, in all four combinations.

**The assertion cell above names no total on purpose.** It carried `18/18` for a
round while the real count was 17, which is the same failure mode as
`SQ-SQ-R2-F1` one layer out: a recorded measurement that had come loose from the
thing it measured, so a later drift could not contradict it. A hand-maintained
count rots on the next commit that adds an assertion; "every settings assertion
passes, and the run's one failure is elsewhere" is checkable by running it and
does not.

**The interaction probe now walks both endings.** It checks that the dependent
switch is disabled rather than hidden before the parent is on; that the failure
row exists and is empty before anything fails; that a refused write leaves the
switch off, produces a row with an icon in the **error** tone, populates exactly
one live region, and disables nothing that was not already disabled; that
turning the switch on again is a retry that succeeds and clears the row; and
that disabling with a restore conflict leaves the switch off with a row in the
**warning** tone, cleared by the next successful change.

The tone assertions are the more important ones, because a single shared
message would satisfy every other check in that list while erasing the
distinction the design exists for. **They compare the row's computed colour
against the resolved design token, never the class name.** The class-name
version of these assertions passed for a full round against a row that was
rendering grey — it could only ever have failed if somebody renamed a class,
which is not how this defect happens. They also compare the two failures'
painted colours against *each other*, captured in the same run, because two rows
overridden to the same grey satisfy any check that only compares tokens.

The expected values are read from `:root` at assertion time rather than written
as hex, since light and dark resolve `--bad` and `--warn` differently; the walk
was confirmed in both appearances and both languages.

They were validated by removing the failure model and re-running: nine of them
failed at once, then the model was restored. A regression never observed to fail
is a comment, not a test.

**Three of this window's older assertions were repaired in the same pass**, and
they were this group's own debt. They hardcoded "two switches" and "four
preference controls" for the whole window, and asserted that no control anywhere
was disabled — all three written before this topic added a group whose dependent
controls are *deliberately* greyed. They now address the general group by name,
check that every rendered hint really is its control's accessible description
rather than counting controls, and ask whether a refusal disabled anything
**additional** rather than whether anything is disabled at all. A check that
only holds in one layout is a check with an unstated precondition.

## What the specimen does not settle

- Whether the real `~/.claude/settings.json` write preserves an existing
  `statusLine`, and whether disabling restores it byte for byte. That is a
  behavioural contract for `architecture.md` and a test, not a specimen.
- Whether a disabled switch announces as unavailable to VoiceOver on the real
  control, and how the dependency is conveyed non-visually.
- Real notification delivery, including Do Not Disturb and Notification Centre
  permission states.
- Whether the real file system produces the two failures the window now renders,
  and under which conditions. The window's response to each is settled above and
  asserted on the specimen; reproducing the conditions against a real
  `~/.claude/settings.json` belongs to `architecture.md` and its tests, and the
  review stage does not touch the user's own configuration to get it.

These belong to `tasks.md` as manual acceptance or as tests.
