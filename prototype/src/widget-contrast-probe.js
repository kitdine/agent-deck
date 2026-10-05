// Opt-in browser regression for the cost-incomplete explanation. Keep the
// scope narrow: other existing gallery contrast findings have their own owners.
export function runWidgetContrastProbe() {
  const params = new URLSearchParams(window.location.search);
  if (params.get("widgetContrast") !== "1") return;
  window.setTimeout(async () => {
    const output = document.createElement("pre");
    output.id = "widget-contrast-out";
    try {
      const { default: axe } = await import("axe-core");
      const notes = [...document.querySelectorAll(".w-note > small")];
      if (notes.length !== 1 || !notes[0].textContent.trim() || notes[0].getBoundingClientRect().height === 0) {
        throw new Error("Expected one visible cost-incomplete explanation");
      }
      const result = await axe.run({ include: [[".w-note > small"]] }, {
        runOnly: { type: "rule", values: ["color-contrast"] },
      });
      const passed = result.violations.length === 0 && result.incomplete.length === 0 && result.passes.length > 0;
      output.textContent = JSON.stringify({
        status: passed ? "PASS" : "FAIL", theme: params.get("theme"),
        widgetRefresh: params.get("widgetRefresh") ?? "fresh",
        lang: params.get("lang") ?? "zh", axe: axe.version,
        violations: result.violations, incomplete: result.incomplete,
        passedRules: result.passes.map(({ id }) => id),
      }, null, 2);
    } catch (error) {
      output.textContent = JSON.stringify({ status: "FAIL", error: String(error) });
    }
    document.body.appendChild(output);
  }, 600);
}
