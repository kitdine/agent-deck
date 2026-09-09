// 尺寸合同自检：ux/widget.md 的 size-as-depth 表是规范面，这个 board 是它声明的
// 渲染面，两面必须说同一件事。W-F12 就是它们各说各话——文档写 medium 三期各带
// cost 与 tokens、20 根柱子、large 的 peak 带日期，specimen 三条全不是。
//
// 截图证明不了这些：30 根柱子和 20 根柱子在缩略图上分不出来，缺一行 token 也看不出
// 是缺了还是本来就没有。这里逐条数出来。
//
// 也查 ellipsis。溢出量具只看容器有没有被裁，看不见一行文字被 text-overflow 吃掉——
// peak 的日期第一版就是这样悄悄少了 8px，DOM 里有、屏幕上没有。
import { catalogs } from "./i18n.js";

// 这三个数是从 ux/widget.md:133-135 抄下来的，故意不从 Widgets.jsx import：
// 共用一个常量的话，把常量改错会同时挪动渲染和期望，断言就永远为真。断言要能失败，
// 期望值就必须独立于被测代码。
const SMALL_BUCKETS = 7;
const MEDIUM_BUCKETS = 20;
const LARGE_BUCKETS = 90;

export function runContract() {
  const params = new URLSearchParams(window.location.search);
  if (params.get("contract") !== "1") return;
  const lang = params.get("lang") === "en" ? "en" : "zh";
  const dict = catalogs[lang];

  window.setTimeout(() => {
    const results = [];
    const check = (name, condition) => results.push(`${condition ? "PASS" : "FAIL"}  ${name}`);
    const card = (size) =>
      [...document.querySelectorAll(`.widget-${size}`)].find((node) =>
        (node.getAttribute("aria-label") ?? "").startsWith(dict.widgets.kinds.magnitude.title),
      );
    const clipped = (node) => !!node && node.scrollWidth > node.clientWidth + 1;

    const small = card("small");
    const medium = card("medium");
    const large = card("large");
    check("三个尺寸的用量卡片都在", !!small && !!medium && !!large);

    // small：7 桶 sparkline
    check(
      `small 的 sparkline 是 ${SMALL_BUCKETS} 桶`,
      small.querySelectorAll(".mini-bars > i").length === SMALL_BUCKETS,
    );

    // medium：三期各带 cost 与 tokens、20 根柱子、轴跟着这 20 天
    const rows = [...medium.querySelectorAll(".w-periods > div")];
    check("medium 是三期对比", rows.length === 3);
    check(
      "medium 三期各有一个成本",
      rows.every((row) => !!row.querySelector("strong")?.textContent.trim()),
    );
    check(
      "medium 三期各有一个 token 值，紧跟在成本下面",
      rows.every((row) => {
        const cost = row.querySelector("strong");
        const tokens = row.querySelector("small");
        return (
          !!tokens?.textContent.trim() &&
          !!cost &&
          cost.compareDocumentPosition(tokens) & Node.DOCUMENT_POSITION_FOLLOWING
        );
      }),
    );
    check(
      `medium 的柱状图正好 ${MEDIUM_BUCKETS} 根`,
      medium.querySelectorAll(".mini-bars > i").length === MEDIUM_BUCKETS,
    );
    check("medium 的柱状图带日期轴", medium.querySelectorAll(".w-axis span").length === 2);

    // large：90 桶填充线，三个 stat chip，peak 带日期
    check("large 的图表是填充线而不是柱子", !!large.querySelector(".area") && !large.querySelector(".area i"));
    const areaLine = large.querySelector(".area .area-line");
    check(
      `large 的填充线取满 ${LARGE_BUCKETS} 天`,
      (areaLine?.getAttribute("d") ?? "").split(/[ML]/).filter(Boolean).length === LARGE_BUCKETS,
    );
    const chips = [...large.querySelectorAll(".w-stats > div")];
    check("large 有三个 stat chip", chips.length === 3);
    const peakChip = chips.find((chip) => (chip.querySelector("span")?.textContent ?? "").startsWith(dict.usage.peak));
    check("large 有 peak chip", !!peakChip);
    check("large 的 peak 带日期", !!peakChip?.querySelector("em")?.textContent.trim());

    // 三个 chip 都不能被 ellipsis 吃掉，日期在里面才算真的在屏幕上
    check(
      "stat chip 的标签与值都没有被 ellipsis 截断",
      chips.every((chip) => !clipped(chip.querySelector("span")) && !clipped(chip.querySelector("strong"))),
    );

    // 额度卡。期望值从 ux/widget-quota.md 的 size-as-depth 表抄下，
    // 不从 Widgets.jsx import；两边共用一个常量会让断言永远为真。
    const quotaCard = (size) =>
      [...document.querySelectorAll(`.widget-${size}`)].find((node) =>
        (node.getAttribute("aria-label") ?? "").startsWith(dict.widgets.kinds.quota.title),
      );
    const qSmall = quotaCard("small");
    const qMedium = quotaCard("medium");
    const qLarge = quotaCard("large");
    check("三个尺寸的额度卡片都在", !!qSmall && !!qMedium && !!qLarge);

    const widgetClientGroup = [...document.querySelectorAll(".stage-group")].find(
      (node) => node.querySelector(":scope > span")?.textContent === dict.states.widgetClient,
    );
    check("small / medium 有 Codex / Claude 两个配置选项", widgetClientGroup?.querySelectorAll("button").length === 2);

    const params = new URLSearchParams(window.location.search);
    const quotaVariant = params.get("quota") ?? "normal";
    const configuredClient = params.get("widgetClient") ?? "codex";
    const largeClients = [...qLarge.querySelectorAll(".w-quota-client")];

    if (quotaVariant === "codexPlus") {
      // Codex Plus：Codex 四个窗口（已用 45/62/12/4），Claude 两个（22/3）。
      // small / medium 呈现的是"配置选定的那一端"，所以期望值必须跟着配置走。
      // 先前这两条写死了 Codex 的数字，于是这块契约只在默认配置下跑过；把
      // 小组件配置成 Claude 时它报错，报的却是自己的假设，不是产品的行为。
      const CODEX_WINDOWS = 4;
      const CLAUDE_WINDOWS = 2;
      const CODEX_WORST = "62%";
      const CLAUDE_WORST = "22%";
      const configuredWindows = configuredClient === "claude" ? CLAUDE_WINDOWS : CODEX_WINDOWS;
      const configuredWorst = configuredClient === "claude" ? CLAUDE_WORST : CODEX_WORST;

      // small：选定端 + 已用最高的那个窗口，不是返回顺序里的第一个。
      check(
        `small 只显示选定的 ${configuredClient}`,
        qSmall.querySelector("header small")?.textContent === dict.clients[configuredClient],
      );
      check("small 只显示一个窗口", qSmall.querySelectorAll(".w-track").length === 1);
      check(
        `small 显示的是 ${configuredClient} 已用最高的窗口`,
        qSmall.querySelector(".w-headline")?.textContent.trim() === configuredWorst,
      );
      check("small 不显示另一端", qSmall.querySelectorAll(".w-quota-client").length === 0);

      // medium：同一个选定端的完整内容——所有窗口加官方重置次数。
      check(
        `medium 只显示选定的 ${configuredClient}`,
        qMedium.querySelector("header small")?.textContent === dict.clients[configuredClient],
      );
      check(
        `medium 显示该端全部 ${configuredWindows} 个窗口`,
        qMedium.querySelectorAll(".w-quota-row").length === configuredWindows,
      );
      check("medium 不显示另一端", qMedium.querySelectorAll(".w-quota-client").length === 0);

      // large：两端同时在，上下等分。
      check("large 上下排列 2 个端", largeClients.length === 2);
      check(
        "large 端顺序是 Codex 然后 Claude",
        largeClients.map((node) => node.dataset.client).join(",") === "codex,claude",
      );
      const largeCodex = largeClients.find((node) => node.dataset.client === "codex");
      const largeClaude = largeClients.find((node) => node.dataset.client === "claude");
      check(`large Codex 显示 ${CODEX_WINDOWS} 个窗口`, largeCodex?.querySelectorAll(".w-quota-row").length === CODEX_WINDOWS);
      check(`large Claude 显示 ${CLAUDE_WINDOWS} 个窗口`, largeClaude?.querySelectorAll(".w-quota-row").length === CLAUDE_WINDOWS);
      // 重置次数属于 popover，不属于小组件。这条断言不是多余的重复：
      // 它是在防止有人日后"顺手"把它加回来——小组件没有展开的位置。
      check(
        "小组件三个尺寸都不出现官方重置次数",
        [qSmall, qMedium, qLarge].every((card) => !card.textContent.includes(dict.quota.allowance)),
      );
      // 上下等分：两块高度一致，且一起填满卡片。只断言"等高"是不够的——
      // 容器不撑开时两块同样等高，却挤在顶部、底下留一大片空白，断言照样通过。
      // 所以必须同时量"是否填满"，那才是"上下各自居中"真正依赖的性质。
      const heights = largeClients.map((node) => Math.round(node.getBoundingClientRect().height));
      check("large 两端等高", heights.length === 2 && Math.abs(heights[0] - heights[1]) <= 1);
      const split = qLarge.querySelector(".w-quota-split");
      const body = qLarge.querySelector(".widget-body");
      const splitBox = split?.getBoundingClientRect();
      const bodyBox = body?.getBoundingClientRect();
      check(
        "large 的上下两半填满卡片，不是挤在顶部",
        !!splitBox && !!bodyBox && bodyBox.height - splitBox.height <= 1,
      );
    } else if (quotaVariant === "normal") {
      // 门禁：Codex 非 official，只有 Claude 有数据。缺失端不留占位。
      check("gate 下 large 只显示有数据的 Claude", largeClients.map((node) => node.dataset.client).join(",") === "claude");
      // 只有一个端时不拉满：等分是两端之间的关系，一个端没有对手可分，
      // 拉满只会得到一大片空的底色。整体居中，底色贴着内容。
      const soleBox = largeClients[0]?.getBoundingClientRect();
      const soleBody = qLarge.querySelector(".widget-body")?.getBoundingClientRect();
      check(
        "large 只有一个端时该端不被拉满整张卡",
        !!soleBox && !!soleBody && soleBody.height - soleBox.height > 20,
      );
      if (configuredClient === "codex") {
        check("small 配置到无数据端时不偷换成另一端", !qSmall.querySelector(".w-headline"));
        check("medium 配置到无数据端时不偷换成另一端", !qMedium.querySelector(".w-quota-row"));
      }
    }

    // 归属说明的存在性。量具只证明没有溢出，证明不了该在的内容在不在——
    // 上一轮正是这样：NO OVERFLOW 全绿，而三个尺寸一个字都没显示。
    // 断言按"看得见的文本"写，title 与辅助名称不算数。
    const visibleAttribution = (node) => {
      const line = node?.querySelector(".w-attribution");
      if (!line) return false;
      const box = line.getBoundingClientRect();
      return line.textContent.trim() === dict.quota.attributionShort && box.width > 0 && box.height > 0;
    };
    const claudeBlock = largeClients.find((node) => node.dataset.client === "claude");
    const codexBlock = largeClients.find((node) => node.dataset.client === "codex");

    check("large 的 Claude 端块带可见的归属说明", !claudeBlock || visibleAttribution(claudeBlock));
    check("large 的 Codex 端块不带归属说明（它有 accountId）", !codexBlock || !codexBlock.querySelector(".w-attribution"));
    if (configuredClient === "claude" && qSmall?.querySelector(".w-headline")) {
      check("small 配置为 Claude 时带可见的归属说明", visibleAttribution(qSmall));
      check("medium 配置为 Claude 时带可见的归属说明", visibleAttribution(qMedium));
    }
    if (configuredClient === "codex" && qSmall?.querySelector(".w-headline")) {
      check("small 配置为 Codex 时不带归属说明", !qSmall.querySelector(".w-attribution"));
      check("medium 配置为 Codex 时不带归属说明", !qMedium.querySelector(".w-attribution"));
    }
    check(
      "归属说明没有被 ellipsis 截断，也没有溢出所在卡片",
      [...document.querySelectorAll(".w-attribution")].every((node) => {
        const card = node.closest(".widget");
        const box = node.getBoundingClientRect();
        const cardBox = card.getBoundingClientRect();
        return !clipped(node) && box.right <= cardBox.right + 0.5 && box.bottom <= cardBox.bottom + 0.5;
      }),
    );

    // 左色条不是唯一的区分：每块必须仍有端名，颜色不可见时也能读懂。
    check(
      "large 每个端块都有明文端名",
      largeClients.every((node) => !!node.querySelector("header strong")?.textContent.trim()),
    );
    check(
      "额度行标签没有被 ellipsis 截断",
      [...document.querySelectorAll(".w-quota-row span")].every((node) => !clipped(node)),
    );

    const box = document.createElement("pre");
    box.id = "contract-out";
    box.style.cssText =
      "position:fixed;inset:0;z-index:9999;margin:0;padding:16px;overflow:auto;background:#000;color:#0f0;font:12px ui-monospace;white-space:pre-wrap";
    const failed = results.filter((line) => line.startsWith("FAIL")).length;
    box.textContent = `${results.join("\n")}\n\n${failed === 0 ? "ALL PASS" : failed + " FAILED"}`;
    document.body.appendChild(box);
  }, 600);
}
