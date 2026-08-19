// 中英两套文案。中文标签普遍比英文宽 30~50%，两套都能切是为了在原型阶段就看出会不会撞版。
// 数字、货币、日期一律走 Intl，跟着语言走，不在字符串里写死 $ 或 en-US。

const zh = {
  locale: "zh-Hans",
  currency: "USD",
  app: "AgentDeck",
  refresh: "刷新",
  refreshing: "刷新中…",
  updated: "已更新",
  retry: "重试",
  refreshFailed: "刷新失败",
  clients: { all: "全部", codex: "Codex", claude: "Claude" },
  periods: { today: "今日", "7d": "7 天", "30d": "30 天" },
  periodsShort: { today: "今日", "7d": "7天", "30d": "30天" },
  tabs: { usage: "用量", breakdown: "构成", attribution: "归因", sessions: "会话" },
  tabQuestions: {
    usage: "花了多少",
    breakdown: "花在哪儿",
    attribution: "数字可信吗",
    sessions: "怎么干的活",
  },
  hero: { events: "事件", sessions: "会话", projects: "项目" },
  usage: {
    trend: "趋势",
    avgPerDay: "日均",
    peak: "峰值",
    cacheHit: "缓存命中",
    busiestHour: "最贵时段",
    hourAxis: { start: "00:00", mid: "12:00", end: "现在" },
  },
  breakdown: {
    models: "模型",
    tokenMix: "Token 构成",
    cacheWriteBilled: "写缓存计费",
    subtotals: "客户端小计",
    input: "输入",
    output: "输出",
    cacheRead: "读缓存",
    cacheWrite: "写缓存",
    others: (count) => `其余 ${count} 个模型`,
  },
  attribution: {
    quality: "归因质量",
    determinable: "可确定",
    inferred: "推断",
    unattributed: "未归因",
    coverage: "计价覆盖",
    byProvider: "按服务商",
    unpriced: "未计价标识",
    unpricedNone: "全部事件都已计价",
    incompleteNote: "成本仍然可见地不完整",
  },
  sessions: {
    count: "会话",
    average: "均时长",
    projects: "项目",
    recent: "最近会话",
    byProject: "按项目",
    signals: "工作信号",
    activity: "活动",
    workflow: "工作流",
    tooling: "工具",
    pending: "待采集",
    pendingHint: "当前快照没有这些字段，需要新增采集",
    back: "返回",
    activityLead: (share) => `编码占今日支出的 ${share}%`,
    activityKinds: { coding: "编码", debugging: "调试", conversation: "对话", delegation: "委派" },
    firstEdit: "首次编辑",
    filesTouched: "触及文件",
    iterationDepth: "迭代深度",
    editsPerSession: "每会话编辑",
    median: "中位数",
    turnsPerEdit: "轮次/编辑",
    topFile: "最常改动",
    workflowLead: "起步快，编辑集中",
    toolCalls: "工具调用",
    toolGroups: "工具组",
    topServer: "主要 MCP 服务",
    shareOfCost: "占今日成本",
    toolKinds: { bash: "Bash", read: "读取", edit: "编辑", mcp: "MCP" },
  },
  rhythm: {
    title: "节律",
    scope: "近 30 天 · 不受上方筛选影响",
    active: "活跃",
    busiest: "最忙",
    quietest: "最闲",
    peak: "高峰",
    hourOfWeek: "一周作息",
    calendar: "90 天日历",
    low: "低",
    high: "高",
    streak: "最长连续",
    lateNight: "深夜",
    weekend: "周末",
    days: (value) => `${value} 天`,
    weekdays: ["周日", "周一", "周二", "周三", "周四", "周五", "周六"],
    weekdaysShort: ["日", "一", "二", "三", "四", "五", "六"],
  },
  footer: { providers: "服务商", switchTo: "切换到", current: "当前", ready: "可用" },
  reasons: { already_selected: "当前正在使用", wrapper_not_configured: "未配置 wrapper" },
  status: {
    costIncomplete: (count) => `成本不完整 · ${count} 项未计价`,
    unreadable: "数据读取失败",
    offline: "无法连接本地助手",
    partial: "部分数据不可用",
    empty: "今天还没有本地活动",
    emptyOther: "暂无可分解数据",
    unavailable: "打开 AgentDeck 以刷新",
    justNow: "刚刚更新",
    relative: (text) => `${text}更新`,
    aged: (text) => `上次更新于${text}`,
    healthProblem: (count) => `${count} 项检查未通过`,
    healthTitle: "运行状况",
    healthNote: "这些检查来自 agentdeck doctor，可在终端运行以查看修复建议。",
    expand: "展开",
    collapse: "收起",
    checks: {
      state_root_permissions: "状态目录权限",
      credential_key: "凭据密钥",
      usage_index: "用量索引",
      price_catalog: "价格目录",
      session_index: "会话索引",
    },
    checkStatus: { ok: "正常", warning: "警告", failed: "失败" },
  },
  menu: {
    menubarDisplay: "菜单栏显示",
    displayTotal: "今日总额",
    displayFollow: "跟随面板筛选",
    displayIcon: "仅图标",
    settings: "设置…",
    about: "关于 AgentDeck",
    quit: "退出",
    hint: "左键开关面板 · 右键或双击弹出菜单",
  },
  confirm: {
    title: (provider, client) => `为 ${client} 使用 ${provider}？`,
    body: "只会改动这个客户端的服务商。用量历史与凭据保持不变。",
    cancel: "取消",
    ok: "使用该服务商",
    done: (provider, client) => `${client} 已切换到 ${provider}`,
    already: (provider, client) => `${client} 正在使用 ${provider}`,
  },
  widgets: {
    boardTitle: "AgentDeck 桌面小组件",
    boardSubtitle: "四个问题，三种深度，十二个原生尺寸",
    backToPopover: "← 返回 Popover",
    kinds: {
      magnitude: { name: "用量", title: "用量", question: "花了多少？" },
      composition: { name: "构成", title: "构成", question: "花在哪儿？" },
      trust: { name: "归因", title: "归因", question: "数字可信吗？" },
      rhythm: { name: "节律", title: "活动", question: "什么时候在干活？" },
    },
    sizes: { small: "小", medium: "中", large: "大" },
    allClients: "全部客户端",
    topModel: "最大模型",
    ofTokens: (part, total) => `${total} 中的 ${part}`,
    activeDays: "活跃天数",
    busiestAt: (day, hour) => `${day} ${hour} 最忙`,
    context90: "90 天活动",
    measurement: "归因质量",
    determinateCost: "可确定成本",
  },
  settings: {
    title: "AgentDeck 设置",
    close: "关闭",
    general: "通用",
    launchAtLogin: "开机时启动",
    launchAtLoginHint: "通过系统登录项注册，不安装后台守护进程",
    loginItemRefused: "无法修改登录项",
    periodicRefresh: "定时刷新",
    periodicRefreshHint: "按快照给出的下次刷新时间刷新；关闭时只在打开面板和手动刷新时更新",
    menubar: "菜单栏",
    menubarValue: "显示内容",
    menubarValueHint: "共享屏幕时可切到仅图标",
    valueCost: "成本",
    valueTokens: "Token",
    valueIcon: "仅图标",
    menubarScope: "统计范围",
    menubarScopeHint: "跟随面板筛选时，面板里选了 Codex，菜单栏也只显示 Codex",
    scopeAll: "全部客户端",
    scopeFollow: "跟随面板筛选",
  },
  states: {
    boardTitle: "状态一览",
    boardSubtitle: "同一份界面在五种数据状态下的样子",
    normal: "正常",
    empty: "今日无花费",
    aged: "数据过期",
    partial: "部分不可用",
    unavailable: "完全不可用",
    language: "语言",
    theme: "外观",
    themeDark: "深色",
    themeLight: "浅色",
    state: "状态",
  },
};

const en = {
  ...zh,
  locale: "en-US",
  refresh: "Refresh",
  refreshing: "Refreshing…",
  updated: "Updated",
  retry: "Retry",
  refreshFailed: "Refresh failed",
  clients: { all: "All", codex: "Codex", claude: "Claude" },
  periods: { today: "Today", "7d": "7 Days", "30d": "30 Days" },
  periodsShort: { today: "Today", "7d": "7D", "30d": "30D" },
  tabs: { usage: "Usage", breakdown: "Breakdown", attribution: "Attribution", sessions: "Sessions" },
  tabQuestions: {
    usage: "How much",
    breakdown: "Where it goes",
    attribution: "Is it real",
    sessions: "How I work",
  },
  hero: { events: "events", sessions: "sessions", projects: "projects" },
  usage: {
    trend: "Trend",
    avgPerDay: "Avg / day",
    peak: "Peak",
    cacheHit: "Cache hit",
    busiestHour: "Priciest hour",
    hourAxis: { start: "00:00", mid: "12:00", end: "Now" },
  },
  breakdown: {
    models: "Models",
    tokenMix: "Token mix",
    cacheWriteBilled: "cache write is billed",
    subtotals: "Client subtotals",
    input: "Input",
    output: "Output",
    cacheRead: "Cache read",
    cacheWrite: "Cache write",
    others: (count) => `${count} more models`,
  },
  attribution: {
    quality: "Attribution quality",
    determinable: "Determinable",
    inferred: "Inferred",
    unattributed: "Unattributed",
    coverage: "Pricing coverage",
    byProvider: "By provider",
    unpriced: "Unpriced identifiers",
    unpricedNone: "Every event is priced",
    incompleteNote: "Cost remains visibly incomplete",
  },
  sessions: {
    ...zh.sessions,
    count: "Sessions",
    average: "Avg length",
    projects: "Projects",
    recent: "Recent sessions",
    byProject: "By project",
    signals: "Work signals",
    activity: "Activity",
    workflow: "Workflow",
    tooling: "Tooling",
    pending: "Not captured yet",
    pendingHint: "These fields are not in the snapshot yet",
    back: "Back",
    activityLead: (share) => `Coding drives ${share}% of today's spend`,
    activityKinds: { coding: "Coding", debugging: "Debugging", conversation: "Conversation", delegation: "Delegation" },
    firstEdit: "First edit",
    filesTouched: "Files touched",
    iterationDepth: "Iteration depth",
    editsPerSession: "Edits / session",
    median: "median",
    turnsPerEdit: "turns / edit",
    topFile: "Most touched",
    workflowLead: "Fast start, focused edits",
    toolCalls: "Tool calls",
    toolGroups: "tool groups",
    topServer: "Top MCP server",
    shareOfCost: "of today's cost",
    toolKinds: { bash: "Bash", read: "Read", edit: "Edit", mcp: "MCP" },
  },
  rhythm: {
    ...zh.rhythm,
    title: "Rhythm",
    scope: "Last 30 days · not affected by the filters above",
    active: "Active",
    busiest: "Busiest",
    quietest: "Quietest",
    peak: "Peak",
    hourOfWeek: "Hour of week",
    calendar: "90-day calendar",
    low: "Low",
    high: "High",
    streak: "Longest streak",
    lateNight: "Late night",
    weekend: "Weekend",
    days: (value) => `${value} days`,
    weekdays: ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"],
    weekdaysShort: ["S", "M", "T", "W", "T", "F", "S"],
  },
  footer: { providers: "Providers", switchTo: "Switch to", current: "Current", ready: "Ready to use" },
  reasons: { already_selected: "Currently in use", wrapper_not_configured: "Wrapper not configured" },
  status: {
    ...zh.status,
    costIncomplete: (count) => `Cost incomplete · ${count} unpriced`,
    unreadable: "Data could not be read",
    offline: "Local helper unreachable",
    partial: "Some data unavailable",
    empty: "No local activity today",
    emptyOther: "Nothing to break down yet",
    unavailable: "Open AgentDeck to refresh",
    justNow: "Updated just now",
    relative: (text) => `Updated ${text}`,
    aged: (text) => `Last updated ${text}`,
    healthProblem: (count) => `${count} checks not passing`,
    healthTitle: "Health",
    healthNote: "These checks come from agentdeck doctor; run it in a terminal for the recovery steps.",
    expand: "Show",
    collapse: "Hide",
    checks: {
      state_root_permissions: "State directory permissions",
      credential_key: "Credential key",
      usage_index: "Usage index",
      price_catalog: "Price catalog",
      session_index: "Session index",
    },
    checkStatus: { ok: "OK", warning: "Warning", failed: "Failed" },
  },
  menu: {
    menubarDisplay: "Menu bar shows",
    displayTotal: "Today's total",
    displayFollow: "Follow panel filter",
    displayIcon: "Icon only",
    settings: "Settings…",
    about: "About AgentDeck",
    quit: "Quit",
    hint: "Left click toggles the panel · right click or double click opens the menu",
  },
  confirm: {
    title: (provider, client) => `Use ${provider} for ${client}?`,
    body: "Only this client's provider changes. Usage history and credentials stay unchanged.",
    cancel: "Cancel",
    ok: "Use provider",
    done: (provider, client) => `${provider} is now active for ${client}`,
    already: (provider, client) => `${provider} is already active for ${client}`,
  },
  widgets: {
    ...zh.widgets,
    boardTitle: "AgentDeck Widgets",
    boardSubtitle: "Four questions, three depths, twelve native surfaces",
    backToPopover: "← Back to popover",
    kinds: {
      magnitude: { name: "Magnitude", title: "Usage", question: "How much am I spending?" },
      composition: { name: "Composition", title: "Breakdown", question: "Where does it go?" },
      trust: { name: "Trust", title: "Attribution", question: "Is the number real?" },
      rhythm: { name: "Rhythm", title: "Activity", question: "When do I actually work?" },
    },
    sizes: { small: "Small", medium: "Medium", large: "Large" },
    allClients: "All clients",
    topModel: "Top model",
    ofTokens: (part, total) => `${part} of ${total}`,
    activeDays: "Active days",
    busiestAt: (day, hour) => `Busiest at ${day} ${hour}`,
    context90: "90-day activity",
    measurement: "Measurement quality",
    determinateCost: "determinate cost",
  },
  settings: {
    title: "AgentDeck Settings",
    close: "Close",
    general: "General",
    launchAtLogin: "Launch at login",
    launchAtLoginHint: "Registered as a system login item; no background daemon is installed",
    loginItemRefused: "Could not change the login item",
    periodicRefresh: "Periodic refresh",
    periodicRefreshHint: "Refreshes at the time the snapshot suggests; when off, only opening the panel or refreshing manually updates it",
    menubar: "Menu bar",
    menubarValue: "Shows",
    menubarValueHint: "Switch to icon only when sharing your screen",
    valueCost: "Cost",
    valueTokens: "Tokens",
    valueIcon: "Icon only",
    menubarScope: "Scope",
    menubarScopeHint: "When following the panel, picking Codex there also narrows the menu bar to Codex",
    scopeAll: "All clients",
    scopeFollow: "Follow panel filter",
  },
  states: {
    ...zh.states,
    boardTitle: "Surface states",
    boardSubtitle: "The same surface across five data states",
    normal: "Normal",
    empty: "No spend today",
    aged: "Stale data",
    partial: "Partially unavailable",
    unavailable: "Unavailable",
    language: "Language",
    theme: "Appearance",
    themeDark: "Dark",
    themeLight: "Light",
    state: "State",
  },
};

export const catalogs = { zh, en };

export function formatCost(value, lang, options = {}) {
  const dict = catalogs[lang];
  const digits = options.compact && Math.abs(value) >= 100 ? 0 : 2;
  return new Intl.NumberFormat(dict.locale, {
    style: "currency",
    currency: dict.currency,
    currencyDisplay: "narrowSymbol",
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(value);
}

export function formatTokens(value) {
  if (value >= 1e9) return `${(value / 1e9).toFixed(1)}B`;
  if (value >= 1e6) return `${(value / 1e6).toFixed(value >= 1e8 ? 0 : 1)}M`;
  if (value >= 1e3) return `${(value / 1e3).toFixed(0)}K`;
  return `${value}`;
}

export function formatNumber(value, lang) {
  return new Intl.NumberFormat(catalogs[lang].locale).format(value);
}

export function formatShare(value, lang) {
  return new Intl.NumberFormat(catalogs[lang].locale, {
    style: "percent",
    minimumFractionDigits: value < 10 ? 1 : 0,
    maximumFractionDigits: 1,
  }).format(value / 100);
}

export function formatDate(date, lang, options = { month: "short", day: "numeric" }) {
  return new Intl.DateTimeFormat(catalogs[lang].locale, { ...options, timeZone: "UTC" }).format(date);
}

export function formatWeekdayDate(date, lang) {
  return new Intl.DateTimeFormat(catalogs[lang].locale, {
    weekday: "short",
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  }).format(date);
}

export function formatDuration(minutes, lang) {
  if (minutes < 60) return lang === "zh" ? `${minutes} 分钟` : `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  // 中文在窄列里不加空格，否则 "4 小时 36 分" 会折行
  if (lang === "zh") return rest === 0 ? `${hours} 小时` : `${hours}时${rest}分`;
  return rest === 0 ? `${hours}h` : `${hours}h ${rest}m`;
}

export function formatHourRange(start, end) {
  const pad = (value) => String(value).padStart(2, "0");
  return `${pad(start)}:00–${pad(end % 24)}:00`;
}

// 窄格子里用的紧凑写法：15–17 时 / 15–17h
export function formatHourRangeShort(start, end, lang) {
  return lang === "zh" ? `${start}–${end % 24} 时` : `${start}–${end % 24}h`;
}

export function relativeTime(minutes, lang) {
  const dict = catalogs[lang];
  if (minutes < 1) return dict.status.justNow;
  const formatter = new Intl.RelativeTimeFormat(dict.locale, { numeric: "auto" });
  const text =
    minutes < 60
      ? formatter.format(-minutes, "minute")
      : minutes < 60 * 24
        ? formatter.format(-Math.round(minutes / 60), "hour")
        : formatter.format(-Math.round(minutes / 1440), "day");
  return minutes > 6 * 60 ? dict.status.aged(text) : dict.status.relative(text);
}
