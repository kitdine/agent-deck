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

    const box = document.createElement("pre");
    box.id = "contract-out";
    box.style.cssText =
      "position:fixed;inset:0;z-index:9999;margin:0;padding:16px;overflow:auto;background:#000;color:#0f0;font:12px ui-monospace;white-space:pre-wrap";
    const failed = results.filter((line) => line.startsWith("FAIL")).length;
    box.textContent = `${results.join("\n")}\n\n${failed === 0 ? "ALL PASS" : failed + " FAILED"}`;
    document.body.appendChild(box);
  }, 600);
}
