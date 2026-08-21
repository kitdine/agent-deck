// 一份数据，popover 与 widget 共用。
// 所有派生值（各期总额、峰值、活跃天、最忙星期）都从 90 天日序列与 7x24 网格算出，
// 不写死，因此两个界面之间以及同一屏的各个图表之间不会互相矛盾。
//
// 字段命名对齐 internal/usage/presentation.go 的投影：
//   scopes[].periods[].totals / average_per_day / peak / cache_hit_share / models
//   scopes[].daily.items[]        90 天
//   scopes[].quality.items[]      归因质量三档，可带 provider
//   scopes[].pricing              coverage / unpriced_identifiers
//   scopes[].rhythm               7x24 cells + active_days + busiest/quietest
//   sessions.items[]              client / model / project / first_at / last_at
//   health                        status / problems / warnings / checks[]

const TODAY = new Date(Date.UTC(2026, 7, 18)); // 2026-08-18，周二
const DAYS = 90;

// 确定性伪随机，保证每次刷新看到的是同一份数据
function rng(seed) {
  let state = seed >>> 0;
  return () => {
    state = (state * 1664525 + 1013904223) >>> 0;
    return state / 4294967296;
  };
}

// 星期权重只有这一份：日序列、7x24 网格、活跃天数全部由它派生，
// 否则会出现"热图说周日不干活、日历却显示周日有花费"这种同屏互相打脸的情况。
export const WEEKDAY_WEIGHT = [0.1, 0.86, 1.0, 0.94, 0.9, 0.66, 0.2]; // 周日..周六
const weekdayWeight = WEEKDAY_WEIGHT;
const spikes = { 88: 1.9, 74: 1.55, 61: 1.4, 45: 1.6, 22: 1.45 }; // 若干个尖峰日

function buildDaily() {
  const random = rng(20260818);
  const rows = [];
  for (let index = 0; index < DAYS; index += 1) {
    const date = new Date(TODAY);
    date.setUTCDate(date.getUTCDate() - (DAYS - 1 - index));
    const weekday = date.getUTCDay();
    const base = weekdayWeight[weekday];
    const noise = 0.55 + random() * 0.9;
    const idle = base < 0.3 && random() < 0.72 ? 0 : 1; // 周末多数整天没干活
    const weight = base * noise * (spikes[index] ?? 1) * idle;
    rows.push({ date, weekday, weight });
  }
  return rows;
}

const rawDaily = buildDaily();

// 把 30 天窗口缩放到 $151.52，与实现里那张截图的量级对齐
const last30Weight = rawDaily.slice(DAYS - 30).reduce((sum, row) => sum + row.weight, 0);
const COST_SCALE = 151.52 / last30Weight;
const CLAUDE_SHARE = 0.176;

function splitByClient(weight, index) {
  const random = rng(index * 7919 + 13);
  const drift = 0.72 + random() * 0.62;
  const claude = Math.min(0.42, CLAUDE_SHARE * drift);
  return { codex: weight * (1 - claude), claude: weight * claude };
}

function dayTotals(weight, seed) {
  if (weight <= 0) {
    return { cost: 0, tokens: 0, input: 0, output: 0, cacheRead: 0, cacheWrite: 0, events: 0, sessions: 0 };
  }
  const random = rng(seed);
  const cost = weight * COST_SCALE;
  // 缓存命中率在 60%~80% 之间浮动，token 与成本不是简单线性，避免看起来像同一根柱子
  const cacheHit = 0.6 + random() * 0.2;
  const tokens = Math.round(cost * (1.55e6 + random() * 4e5));
  const cacheRead = Math.round(tokens * cacheHit);
  const cacheWrite = Math.round(tokens * (0.03 + random() * 0.05));
  const output = Math.round(tokens * (0.05 + random() * 0.03));
  const input = Math.max(0, tokens - cacheRead - cacheWrite - output);
  return {
    cost,
    tokens,
    input,
    output,
    cacheRead,
    cacheWrite,
    events: Math.max(1, Math.round(cost * 7.4 + random() * 6)),
    sessions: Math.max(1, Math.round(cost * 0.31 + random() * 1.4)),
  };
}

const daily = rawDaily.map((row, index) => {
  const split = splitByClient(row.weight, index);
  const codex = dayTotals(split.codex, index * 31 + 1);
  const claude = dayTotals(split.claude, index * 31 + 2);
  return {
    date: row.date,
    iso: row.date.toISOString().slice(0, 10),
    weekday: row.weekday,
    codex,
    claude,
    all: mergeTotals([codex, claude]),
  };
});

function mergeTotals(list) {
  return list.reduce(
    (acc, item) => ({
      cost: acc.cost + item.cost,
      tokens: acc.tokens + item.tokens,
      input: acc.input + item.input,
      output: acc.output + item.output,
      cacheRead: acc.cacheRead + item.cacheRead,
      cacheWrite: acc.cacheWrite + item.cacheWrite,
      events: acc.events + item.events,
      sessions: acc.sessions + item.sessions,
    }),
    { cost: 0, tokens: 0, input: 0, output: 0, cacheRead: 0, cacheWrite: 0, events: 0, sessions: 0 },
  );
}

export const CLIENTS = ["all", "codex", "claude"];
export const PERIODS = ["today", "7d", "30d"];
const PERIOD_DAYS = { today: 1, "7d": 7, "30d": 30 };

function sliceOf(client, period) {
  return daily.slice(DAYS - PERIOD_DAYS[period]).map((row) => row[client]);
}

function periodTotals(client, period) {
  return mergeTotals(sliceOf(client, period));
}

function peakOf(client, period) {
  const rows = daily.slice(DAYS - PERIOD_DAYS[period]);
  let best = rows[0];
  rows.forEach((row) => {
    if (row[client].cost > best[client].cost) best = row;
  });
  return { date: best.date, iso: best.iso, totals: best[client] };
}

// 模型构成：每个客户端有自己的模型，份额随周期轻微漂移，但四个模型的占比之和恒为 100%
const MODEL_DEFS = [
  { model: "gpt-5.6-sol", client: "codex", base: 0.62, tone: "model-a" },
  { model: "claude-opus-5", client: "claude", base: 0.14, tone: "model-b" },
  { model: "codex-auto-review", client: "codex", base: 0.16, tone: "model-c" },
  { model: "gpt-5.5", client: "codex", base: 0.08, tone: "model-d" },
];

function modelsOf(client, period) {
  const pool = MODEL_DEFS.filter((item) => client === "all" || item.client === client);
  const random = rng(PERIODS.indexOf(period) * 977 + CLIENTS.indexOf(client) * 31 + 5);
  const drifted = pool.map((item) => ({ ...item, raw: item.base * (0.82 + random() * 0.36) }));
  const sum = drifted.reduce((acc, item) => acc + item.raw, 0);
  const totals = periodTotals(client, period);
  return drifted
    .map((item) => ({
      model: item.model,
      client: item.client,
      tone: item.tone,
      share: (item.raw / sum) * 100,
      tokens: Math.round((item.raw / sum) * totals.tokens),
      cost: (item.raw / sum) * totals.cost,
    }))
    .sort((left, right) => right.share - left.share);
}

// 归因质量：可确定 / 推断 / 未归因。codex 的可确定比例略低于 claude。
const QUALITY_BASE = {
  all: [0.972, 0.026, 0.002],
  codex: [0.965, 0.032, 0.003],
  claude: [0.994, 0.006, 0],
};

function qualityOf(client, period) {
  const totals = periodTotals(client, period);
  const shares = QUALITY_BASE[client];
  const names = ["determinable", "inferred", "unattributed"];
  return names.map((quality, index) => ({
    quality,
    share: shares[index] * 100,
    cost: totals.cost * shares[index],
  }));
}

function providerQualityOf(period) {
  return ["codex", "claude"].map((client) => ({
    provider: client,
    cost: periodTotals(client, period).cost,
    share: QUALITY_BASE[client][0] * 100,
  }));
}

const PRICING = {
  all: { coverage: 98.6, unpricedEvents: 14, identifiers: ["customer-ranker-v1", "gpt-5.6-sol-preview"] },
  codex: { coverage: 98.1, unpricedEvents: 14, identifiers: ["customer-ranker-v1", "gpt-5.6-sol-preview"] },
  claude: { coverage: 100, unpricedEvents: 0, identifiers: [] },
};

// 7x24 活动网格，近 30 天。最忙/最闲的星期由网格自己算出来，不另外写死。
function buildRhythm(client) {
  const random = rng(CLIENTS.indexOf(client) * 101 + 77);
  const cells = [];
  const dayWeight = WEEKDAY_WEIGHT;
  for (let weekday = 0; weekday < 7; weekday += 1) {
    for (let hour = 0; hour < 24; hour += 1) {
      const afternoon = Math.max(0, 1 - Math.abs(hour - 15) / 7);
      const morning = Math.max(0, 1 - Math.abs(hour - 10) / 5) * 0.55;
      const night = hour >= 22 || hour <= 1 ? 0.22 : 0;
      const shape = Math.max(afternoon, morning, night);
      const value = shape * dayWeight[weekday] * (0.72 + random() * 0.56);
      cells.push({ weekday, hour, intensity: Math.max(0, Math.min(5, Math.round(value * 5))) });
    }
  }
  const perDay = new Array(7).fill(0);
  cells.forEach((cell) => {
    perDay[cell.weekday] += cell.intensity;
  });
  const order = perDay.map((total, weekday) => ({ weekday, total })).sort((a, b) => b.total - a.total);
  const activeDays = daily.slice(DAYS - 30).filter((row) => row[client].cost > 0).length;
  const peakCell = cells.reduce((best, cell) => (cell.intensity > best.intensity ? cell : best), cells[0]);
  const peakHours = cells
    .filter((cell) => cell.weekday === peakCell.weekday && cell.intensity >= peakCell.intensity - 1)
    .map((cell) => cell.hour);
  return {
    cells,
    perDay,
    activeDays,
    busiestDay: order[0].weekday,
    quietestDay: order[order.length - 1].weekday,
    peakStart: Math.min(...peakHours),
    peakEnd: Math.max(...peakHours) + 1,
    longestStreak: longestStreak(client),
    lateNightShare: shareOfHours(cells, (hour) => hour >= 22 || hour <= 5),
    weekendShare: shareOfDays(perDay, [0, 6]),
  };
}

function longestStreak(client) {
  let best = 0;
  let run = 0;
  daily.forEach((row) => {
    if (row[client].cost > 0) {
      run += 1;
      best = Math.max(best, run);
    } else {
      run = 0;
    }
  });
  return best;
}

function shareOfHours(cells, predicate) {
  const total = cells.reduce((sum, cell) => sum + cell.intensity, 0);
  const part = cells.filter((cell) => predicate(cell.hour)).reduce((sum, cell) => sum + cell.intensity, 0);
  return total === 0 ? 0 : (part / total) * 100;
}

function shareOfDays(perDay, days) {
  const total = perDay.reduce((sum, value) => sum + value, 0);
  const part = days.reduce((sum, day) => sum + perDay[day], 0);
  return total === 0 ? 0 : (part / total) * 100;
}

// 会话。快照有 client / model / project / 起止时间，没有单条会话的成本，所以这里也不编成本。
// 会话按每天每客户端的会话数逐条生成，因此"今日 4 会话"与按项目分组之和永远相等——
// 两套口径各算各的，正是同一屏自相矛盾的来源。
const PROJECTS = ["agent-deck", "teslector", "ai-tools", "headroom", "codegraph"];

function buildSessions() {
  const items = [];
  daily.forEach((row, dayIndex) => {
    ["codex", "claude"].forEach((client, clientIndex) => {
      const count = row[client].sessions;
      if (row[client].cost <= 0) return;
      const random = rng(dayIndex * 131 + clientIndex * 17 + 3);
      for (let index = 0; index < count; index += 1) {
        const model =
          client === "codex" ? (random() < 0.76 ? "gpt-5.6-sol" : "codex-auto-review") : "claude-opus-5";
        const startHour = 9 + Math.floor(random() * 9);
        const duration = Math.max(4, Math.round(8 + random() * 66));
        const firstAt = new Date(row.date.getTime() + (startHour * 60 + Math.floor(random() * 50)) * 60e3);
        items.push({
          sessionId: `s-${row.iso}-${client}-${index}`,
          client,
          model,
          project: PROJECTS[Math.floor(random() * PROJECTS.length)],
          firstAt,
          lastAt: new Date(firstAt.getTime() + duration * 60e3),
          minutes: duration,
        });
      }
    });
  });
  return items.sort((left, right) => right.lastAt - left.lastAt);
}

const sessions = buildSessions();

function sessionsOf(client, period) {
  const from = new Date(TODAY);
  from.setUTCDate(from.getUTCDate() - (PERIOD_DAYS[period] - 1));
  return sessions.filter(
    (item) => (client === "all" || item.client === client) && item.firstAt >= from,
  );
}

export function sessionStats(client, period) {
  const scoped = sessionsOf(client, period);
  const minutes = scoped.reduce((sum, item) => sum + item.minutes, 0);
  return {
    count: scoped.length,
    averageMinutes: scoped.length === 0 ? 0 : Math.round(minutes / scoped.length),
    projects: new Set(scoped.map((item) => item.project)).size,
    recent: scoped.slice(0, 4),
    byProject: Object.entries(
      scoped.reduce((acc, item) => {
        acc[item.project] = acc[item.project] ?? { sessions: 0, minutes: 0 };
        acc[item.project].sessions += 1;
        acc[item.project].minutes += item.minutes;
        return acc;
      }, {}),
    )
      .map(([project, value]) => ({ project, ...value }))
      .sort((left, right) => right.minutes - left.minutes),
  };
}

// 下面三块目前的投影里没有对应字段，是这一版原型提出的采集需求，界面上会明确标注。
// 未采集态：一份早于本能力的快照解码出来的样子。保留它，因为旧快照仍会命中这条路径。
export const PENDING_CAPTURE = {
  activity: [
    { key: "coding", share: 58, cost: 3.06, events: 24, tone: "model-a" },
    { key: "debugging", share: 21, cost: 1.11, events: 9, tone: "warn" },
    { key: "conversation", share: 12, cost: 0.63, events: 6, tone: "model-b" },
    { key: "delegation", share: 9, cost: 0.48, events: 4, tone: "model-c" },
  ],
  workflow: { firstEditMinutes: 2, filesTouched: 7, retries: 3, editsPerSession: 4, topFile: "tasks.md", topFileCount: 4 },
  tooling: { calls: 82, groups: 4, rows: [], topServer: "codegraph", topServerCalls: 5 },
};

// 已采集态。四个大类各自带子类，子类的 share/cost/events 之和等于所属大类。
// 工具那一块只有次数和占比：token 由 turn 消耗，工具调用本身不花钱，给它标金额
// 等于把一个归因当成实测值。活动类型的金额是成立的，它归给真正消耗 token 的 turn。
export const WORK_SIGNALS = {
  activity: [
    {
      key: "coding",
      share: 52,
      cost: 2.74,
      events: 21,
      tone: "model-a",
      sub: [
        { key: "feature", share: 24, cost: 1.26, events: 9 },
        { key: "refactoring", share: 13, cost: 0.69, events: 5 },
        { key: "testing", share: 9, cost: 0.47, events: 4 },
        { key: "maintenance", share: 6, cost: 0.32, events: 3 },
      ],
    },
    {
      key: "debugging",
      share: 24,
      cost: 1.27,
      events: 10,
      tone: "warn",
      sub: [
        { key: "investigation", share: 9, cost: 0.48, events: 4 },
        { key: "repair", share: 15, cost: 0.79, events: 6 },
      ],
    },
    {
      key: "conversation",
      share: 15,
      cost: 0.79,
      events: 8,
      tone: "model-b",
      sub: [
        { key: "exploration", share: 8, cost: 0.42, events: 4 },
        { key: "brainstorming", share: 4, cost: 0.21, events: 2 },
        { key: "planning", share: 3, cost: 0.16, events: 2 },
      ],
    },
    {
      key: "delegation",
      share: 9,
      cost: 0.48,
      events: 4,
      tone: "model-c",
      sub: [
        { key: "subagent", share: 6, cost: 0.32, events: 3 },
        { key: "workflow", share: 3, cost: 0.16, events: 1 },
      ],
    },
  ],
  workflow: {
    firstEditMinutes: 2,
    filesTouched: 7,
    // 返工：同一文件 编辑 -> 跑了个非查看类命令 -> 又编辑，计一次。
    retries: 3,
    editsPerSession: 4,
    topFile: "tasks.md",
    topFileCount: 4,
  },
  tooling: {
    calls: 82,
    groups: 4,
    rows: [
      { key: "bash", calls: 32, share: 39 },
      { key: "read", calls: 27, share: 33 },
      { key: "edit", calls: 14, share: 17 },
      { key: "mcp", calls: 9, share: 11 },
    ],
    topServer: "codegraph",
    topServerCalls: 5,
  },
};

export const HEALTH = {
  status: "warning",
  problems: 1,
  warnings: 1,
  checks: [
    { name: "state_root_permissions", status: "ok" },
    { name: "credential_key", status: "ok" },
    { name: "usage_index", status: "warning" },
    { name: "price_catalog", status: "failed" },
    { name: "session_index", status: "ok" },
  ],
};

export const PROVIDER = {
  routes: [
    { client: "codex", provider: "aigocode", viaWrapper: true },
    { client: "claude", provider: "official", viaWrapper: false },
  ],
  candidates: [
    { client: "codex", provider: "aigocode", ready: false, reason: "already_selected" },
    { client: "codex", provider: "official", ready: true, reason: null },
    { client: "claude", provider: "official", ready: false, reason: "already_selected" },
    { client: "claude", provider: "aigocode", ready: false, reason: "wrapper_not_configured" },
  ],
};

export function scope(client, period) {
  const totals = periodTotals(client, period);
  const days = PERIOD_DAYS[period];
  const peak = peakOf(client, period);
  const series = daily.map((row) => ({ iso: row.iso, date: row.date, value: row[client].cost, totals: row[client] }));
  return {
    client,
    period,
    totals,
    averagePerDay: totals.cost / days,
    peak,
    cacheHitShare: totals.tokens === 0 ? 0 : (totals.cacheRead / totals.tokens) * 100,
    models: modelsOf(client, period),
    quality: qualityOf(client, period),
    providerQuality: providerQualityOf(period),
    pricing: PRICING[client],
    daily: series,
    window: series.slice(DAYS - days),
    tokenMix: [
      { key: "input", value: totals.input, tone: "model-a" },
      { key: "output", value: totals.output, tone: "model-b" },
      { key: "cacheRead", value: totals.cacheRead, tone: "model-c" },
      { key: "cacheWrite", value: totals.cacheWrite, tone: "warn" },
    ].map((item) => ({ ...item, share: totals.tokens === 0 ? 0 : (item.value / totals.tokens) * 100 })),
    clientSubtotals: ["codex", "claude"].map((name) => ({ client: name, cost: periodTotals(name, period).cost })),
    sessions: sessionStats(client, period),
    pricingComplete: PRICING[client].unpricedEvents === 0,
  };
}

export const rhythm = { all: buildRhythm("all"), codex: buildRhythm("codex"), claude: buildRhythm("claude") };

// 趋势图按周期换用不同的桶：今日按小时，7 天与 30 天按日
export function buckets(client, period) {
  if (period !== "today") {
    return scope(client, period).window.map((row) => ({
      key: row.iso,
      date: row.date,
      cost: row.value,
      totals: row.totals,
      kind: "day",
    }));
  }
  const totals = periodTotals(client, "today");
  const random = rng(9091 + CLIENTS.indexOf(client));
  const shape = [];
  for (let hour = 0; hour < 24; hour += 1) {
    const afternoon = Math.max(0, 1 - Math.abs(hour - 15) / 7);
    const morning = Math.max(0, 1 - Math.abs(hour - 10) / 5) * 0.6;
    shape.push(Math.max(afternoon, morning) * (0.55 + random() * 0.85) * (hour > 17 ? 0.25 : 1));
  }
  const sum = shape.reduce((acc, value) => acc + value, 0);
  return shape.map((value, hour) => ({
    key: `h${hour}`,
    hour,
    cost: (value / sum) * totals.cost,
    totals: {
      cost: (value / sum) * totals.cost,
      tokens: Math.round((value / sum) * totals.tokens),
      events: Math.max(1, Math.round((value / sum) * totals.events)),
    },
    kind: "hour",
  }));
}

export const meta = { today: TODAY, days: DAYS, firstDate: daily[0].date, lastDate: daily[DAYS - 1].date };
