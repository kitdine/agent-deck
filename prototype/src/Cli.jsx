import { useState } from "react";
import { StageControls, useStagePrefs } from "./Stage.jsx";
import { WORK_SIGNALS } from "./data.js";
import { catalogs } from "./i18n.js";

// CLI 原型。终端里没有布局可调，能被设计的只有三件事：分节、对齐列、以及
// 一个数字读不出来时写什么。所以这里渲染的是逐字符的真实输出，不是示意图。
// 文本本身不随语言切换：AgentDeck 的 CLI 输出是单一英文的，与面板不同。

const PAD = 16;
const BAR = 10;

function pad(label) {
  return label.padEnd(PAD, " ");
}

function bar(share) {
  const filled = Math.max(1, Math.round((share / 100) * BAR));
  return "█".repeat(filled) + "░".repeat(BAR - filled);
}

function plural(count, word) {
  return `${count} ${word}${count === 1 ? "" : "s"}`;
}

function money(value) {
  return `$${value.toFixed(2)}`;
}

function activityLines({ sub }) {
  const lines = [];
  for (const item of WORK_SIGNALS.activity) {
    const name = item.key[0].toUpperCase() + item.key.slice(1);
    lines.push(
      `${pad(name)}${bar(item.share)}  ${String(item.share).padStart(3)}%  ${money(item.cost).padStart(6)}  ${plural(item.events, "event")}`,
    );
    if (!sub) continue;
    for (const child of item.sub) {
      const childName = child.key[0].toUpperCase() + child.key.slice(1);
      lines.push(`  └ ${pad(childName).slice(0, PAD - 2)}            ${String(child.share).padStart(3)}%  ${money(child.cost).padStart(6)}  ${plural(child.events, "event")}`);
    }
  }
  return lines;
}

function workflowLines() {
  const w = WORK_SIGNALS.workflow;
  return [
    `${pad("FIRST EDIT")}${w.firstEditMinutes}m (median)`,
    `${pad("FILES TOUCHED")}${w.filesTouched}`,
    `${pad("REWORK")}${w.retries}  (edit, verify, edit again)`,
    `${pad("EDITS / SESSION")}${w.editsPerSession}`,
    `${pad("MOST TOUCHED")}${w.topFile} ×${w.topFileCount}`,
  ];
}

function toolingLines() {
  const t = WORK_SIGNALS.tooling;
  const rows = t.rows.map((item) => {
    const name = item.key === "mcp" ? "MCP" : item.key[0].toUpperCase() + item.key.slice(1);
    return `${pad(name)}${String(item.calls).padStart(3)} calls  ${String(item.share).padStart(3)}%`;
  });
  return [...rows, `${pad("TOP MCP SERVER")}${t.topServer} · ${t.topServerCalls}`];
}

// usage stats 的真实分节，取自 cmd/agentdeck/usage_stats_layout.go。
// 早前这里抄的是 usage summary 的分节，那是另一条命令。
const STATS_HEAD = [
  `${pad("TOKENS")}2,113,490`,
  `${pad("COST")}$5.28`,
  `${pad("SESSIONS")}12`,
];

const STATS_WEEKDAY = [
  `${pad("BUSIEST")}Wed · 31%`,
  `${pad("QUIETEST")}Sun · 2%`,
];

// 三段工作信号在 usage stats 里是默认输出的一部分，没有开关。一个只有
// GUI 才看得到的度量，也就是脚本读不到、diff 不出来、对不上账的度量。
const SAMPLES = [
  {
    id: "stats",
    command: "agentdeck usage stats --period 7d",
    note: { zh: "默认输出，无开关。插在 ACTIVITY BY WEEKDAY / HOUR 之后、COVERAGE 之前；此处标题是 WORK KIND，避开与既有 ACTIVITY 分节重名。", en: "Default output, no flag. Inserted after ACTIVITY BY WEEKDAY / HOUR and before COVERAGE; titled WORK KIND here to avoid colliding with the existing ACTIVITY section." },
    blocks: [
      ["📊 USAGE STATS · LAST 7 DAYS", STATS_HEAD],
      ["▦ ACTIVITY BY WEEKDAY / HOUR · TOKENS", STATS_WEEKDAY],
      ["🧭 WORK KIND", activityLines({ sub: false })],
      ["🧱 WORKFLOW", workflowLines()],
      ["🔧 TOOLING", toolingLines()],
    ],
  },
  {
    id: "signals",
    command: "agentdeck usage signals --period 7d",
    note: { zh: "单独子命令：只出三段信号，供筛选与脚本读取。", en: "The dedicated subcommand: the three sections alone, for filtering and scripting." },
    blocks: [
      ["🧭 ACTIVITY", activityLines({ sub: false })],
      ["🧱 WORKFLOW", workflowLines()],
      ["🔧 TOOLING", toolingLines()],
    ],
  },
  {
    id: "kind",
    command: "agentdeck usage signals --kind activity --sub",
    note: { zh: "--kind 收到一个模块，--sub 展开子类。面板第二层看到的同一组数字。", en: "--kind narrows to one module, --sub expands the subcategories — the same numbers the panel's second level shows." },
    blocks: [["🧭 ACTIVITY", activityLines({ sub: true })]],
  },
  {
    id: "filter",
    command: "agentdeck usage signals --activity debugging --client claude",
    note: { zh: "--activity 按类型或子类筛，与 --client、--period 组合。", en: "--activity filters by category or subcategory, and composes with --client and --period." },
    blocks: [
      [
        "🧭 ACTIVITY",
        [
          `${pad("Debugging")}${bar(100)}  100%  ${money(0.91).padStart(6)}   7 events`,
          `  └ ${pad("Investigation").slice(0, PAD - 2)}             34%  ${money(0.31).padStart(6)}   3 events`,
          `  └ ${pad("Repair").slice(0, PAD - 2)}             66%  ${money(0.60).padStart(6)}   4 events`,
        ],
      ],
    ],
  },
  {
    id: "session",
    command: "agentdeck session show 01J8F2 --activity",
    note: { zh: "单会话一行。会话级类型有定义：成本占比最高的大类，并列时取 turn 数多的。", en: "One line per session. The session-level kind is defined: the category holding the largest share of cost, ties broken by turn count." },
    blocks: [
      [
        "🗂  SESSION",
        [
          `${pad("CLIENT")}Codex`,
          `${pad("PROJECT")}agent-deck`,
          `${pad("TURNS")}18`,
          `${pad("SIGNALS")}Coding · 12 tool calls · 3 files · first edit 4m`,
        ],
      ],
    ],
  },
  {
    id: "empty",
    command: "agentdeck usage signals --period today --client codex",
    note: { zh: "不可确定与零是两回事：没有 turn 就说没有 turn，不写 0%。退出码仍是 0。", en: "Unavailable is not zero. With no turn in scope it says so rather than printing 0%. Exit code stays 0." },
    blocks: [
      ["🧭 ACTIVITY", ["No turn in the selected scope."]],
      [
        "🧱 WORKFLOW",
        [
          `${pad("FIRST EDIT")}—`,
          `${pad("FILES TOUCHED")}—`,
          `${pad("REWORK")}—`,
          `${pad("EDITS / SESSION")}—`,
          `${pad("MOST TOUCHED")}—`,
        ],
      ],
      ["🔧 TOOLING", ["No tool call in the selected scope."]],
    ],
  },
];

const JSON_SAMPLE = `$ agentdeck usage signals --period 7d --format json
{
  "period": "7d",
  "client": "all",
  "activity": {
    "available": true,
    "cost_basis": "turn",
    "kinds": [
      {
        "kind": "coding",
        "share": 52.0,
        "cost": 2.74,
        "events": 21,
        "sub": [
          { "kind": "feature", "share": 24.0, "cost": 1.26, "events": 9 }
        ]
      }
    ]
  },
  "workflow": {
    "available": true,
    "first_edit_seconds": 132,
    "files_touched": 7,
    "retries": 3,
    "edits_per_session": 4.0,
    "top_file": "tasks.md",
    "top_file_edits": 4
  },
  "tooling": {
    "available": true,
    "calls": 82,
    "groups": 4,
    "rows": [{ "kind": "bash", "calls": 32, "share": 39.0 }],
    "top_mcp_server": "codegraph",
    "top_mcp_calls": 5
  }
}`;

export function CliSurface() {
  const stage = useStagePrefs();
  const { lang, theme } = stage;
  const dict = catalogs[lang];
  const [active, setActive] = useState(SAMPLES[0].id);
  const sample = SAMPLES.find((item) => item.id === active);

  return (
    <main className="stage" data-theme={theme}>
      <StageControls prefs={stage} showState={false} />
      <div className="stage-body cli-body">
        <nav className="cli-tabs">
          {SAMPLES.map((item) => (
            <button type="button" key={item.id} className={item.id === active ? "active" : ""} onClick={() => setActive(item.id)}>
              {item.id}
            </button>
          ))}
        </nav>
        <p className="cli-note">{sample.note[lang]}</p>
        <div className="terminal">
          <div className="terminal-bar">
            <i />
            <i />
            <i />
            <span>{dict.sessions.signals} — agentdeck</span>
          </div>
          <pre>
            <code>
              <b>$ {sample.command}</b>
              {"\n"}
              {sample.blocks.map(([title, lines]) => (
                <span key={title}>
                  {"\n"}
                  <em>{title}</em>
                  {"\n"}
                  {lines.join("\n")}
                  {"\n"}
                </span>
              ))}
            </code>
          </pre>
        </div>
        <div className="terminal">
          <div className="terminal-bar">
            <i />
            <i />
            <i />
            <span>--format json</span>
          </div>
          <pre>
            <code>{JSON_SAMPLE}</code>
          </pre>
        </div>
      </div>
    </main>
  );
}
