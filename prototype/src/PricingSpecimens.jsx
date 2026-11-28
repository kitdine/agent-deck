import { useLayoutEffect, useRef } from "react";
import "./pricingSpecimens.css";

const examples = [
  { id: "complete", amount: "$3.51", en: "API equivalent", zh: "API 等价", hasReviewGap: true },
  { id: "partial", amount: "$1.66", en: "Known API cost", zh: "已知 API 金额", hasReviewGap: true },
  { id: "reference", amount: "$0.84", en: "API reference", zh: "API 参考" },
  { id: "zero", amount: "$0.00", en: "API equivalent", zh: "API 等价" },
  { id: "unavailable", amount: "—", en: "Cost unavailable", zh: "金额不可用" },
];

function PricingAmount({ example, lang }) {
  return (
    <div className="pricing-amount" data-priced-amount={example.id}>
      <span className="pricing-basis">{example[lang]}</span>
      <strong>{example.amount}</strong>
    </div>
  );
}

function PricingRows({ example, lang }) {
  const zh = lang === "zh";
  const comparable = example.id === "complete";
  const known = example.id === "partial";
  const codexAmount = comparable ? "$1.86" : known ? "$1.66" : example.id === "zero" ? "$0.00" : "—";
  const rows = [
    [zh ? `Codex · 标准 · ${known ? "已知 API 金额" : "API 等价"}` : `Codex · Standard · ${known ? "Known API cost" : "API equivalent"}`, codexAmount],
    [zh ? "渠道估算 · 推断归因" : "Provider estimate · inferred", comparable ? "$2.32" : "—"],
    [zh ? "未归属 · API 等价" : "Unattributed · API equivalent", comparable ? "$1.65" : "—"],
    [zh ? "日峰值 · Codex 金额占比" : "Daily peak · Codex cost share", comparable ? "$3.51 · 53%" : "—"],
  ];
  return (
    <dl className="pricing-rows">
      {rows.map(([name, amount]) => <div key={name} data-price-fit><dt>{name}</dt><dd>{amount}</dd></div>)}
    </dl>
  );
}

function approvalCoverage(lang) {
  return lang === "zh" ? "审批审查用量可能缺失" : "Approval review usage may be missing";
}

function MenuBoard({ lang }) {
  return (
    <div className="pricing-menu-board">
      {[280, 420].map(width => (
        <section className="pricing-menu-column" key={width} aria-label={`${width} pt`} style={{ width }}>
          {examples.map(example => (
            <article className="pricing-menu-card" key={example.id}>
              <PricingAmount example={example} lang={lang} />
              {example.id === "reference" && <p>{lang === "zh" ? "2026-11-18 价卡" : "Rates as of 2026-11-18"}</p>}
              <PricingRows example={example} lang={lang} />
              {example.hasReviewGap && <p data-price-fit>{approvalCoverage(lang)}</p>}
              {example.id === "partial" && <p>{lang === "zh" ? "写入时长未证，见诊断详情" : "Cache TTL unverified. See pricing diagnostics."}</p>}
            </article>
          ))}
          <details open className="pricing-menu-card pricing-diagnostics" data-price-fit>
            <summary>{lang === "zh" ? "计价诊断" : "Pricing diagnostics"}</summary>
            <p>{lang === "zh" ? "Standard · 假定写入为 30m 或 12h" : "Standard · 30m/12h cache write assumption"}</p>
            <p>{lang === "zh" ? "参考区间 " : "Reference range "}<strong>$1.86–$2.06</strong></p>
          </details>
        </section>
      ))}
    </div>
  );
}

export function PricingSpecimens() {
  const query = new URLSearchParams(window.location.search);
  const lang = query.get("lang") === "zh" ? "zh" : "en";
  const appearance = query.get("theme") === "light" ? "light" : "dark";
  const board = useRef(null);
  useLayoutEffect(() => {
    const root = board.current;
    const amounts = [...root.querySelectorAll("[data-priced-amount]")];
    const labels = [...amounts.flatMap(amount => [...amount.children]), ...root.querySelectorAll("[data-price-fit]")];
    const horizontalClip = labels.some(label => label.scrollWidth > label.clientWidth + 1);
    root.dataset.pricingLayout = `${amounts.length}:${horizontalClip ? "overflow" : "fit"}`;
  }, []);

  return (
    <main className="pricing-specimens" data-theme={appearance} ref={board} lang={lang === "zh" ? "zh-CN" : "en"}>
      <MenuBoard lang={lang} />
    </main>
  );
}
