import { CLI_HEALTH_RECOVERY, HEALTH_RECOVERY, HEALTH_RECOVERY_STATES } from "./healthRecovery.js";
import { catalogs } from "./i18n.js";

const wait = (ms) => new Promise((resolve) => window.setTimeout(resolve, ms));
const click = (node) => node?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

export function runHealthRecoveryProbe() {
  if (new URLSearchParams(window.location.search).get("healthProbe") !== "1") return;
  const results = [];
  const check = (name, condition) => results.push(`${condition ? "PASS" : "FAIL"}  ${name}`);
  const $ = (selector) => document.querySelector(selector);
  const $$ = (selector) => [...document.querySelectorAll(selector)];

  window.setTimeout(async () => {
    try {
      const states = HEALTH_RECOVERY_STATES.filter((state) => state !== "baseline");
      const healthButton = (state) => $(`[data-stage-axis="health"] button[data-value="${state}"]`);
      const surface = new URLSearchParams(window.location.search).get("surface");

      if (surface === "cli") {
        click($("[data-cli-recovery-tab]"));
        await wait(60);
        for (const state of states) {
          click(healthButton(state));
          await wait(60);
          const terminal = $("[data-cli-health-state]");
          const text = terminal?.textContent ?? "";
          check(`CLI ${state} 选择同一共享状态`, terminal?.dataset.cliHealthState === state);
          check(`CLI ${state} text 逐字符 specimen`, text.includes(CLI_HEALTH_RECOVERY[state].text));
          check(`CLI ${state} JSON 含稳定 reason`, $$(".terminal").at(-1)?.textContent.includes(HEALTH_RECOVERY[state].checks.find((item) => item.reason)?.reason));
        }
      } else {
        let copied = "";
        Object.defineProperty(navigator, "clipboard", {
          configurable: true,
          value: { writeText: async (value) => { copied = value; } },
        });
        for (const state of states) {
          click(healthButton(state));
          await wait(60);
          check(`Popover ${state} 选择同一共享状态`, $(".popover")?.dataset.healthScenario === state);
          const notice = $("button.notice");
          check(`Popover ${state} 有可进入 Health 的原因提示`, !!notice);
          click(notice);
          await wait(60);
          const expectedCodes = HEALTH_RECOVERY[state].checks.filter((item) => item.code).map((item) => item.code);
          const renderedCodes = $$('[data-health-code]').map((node) => node.dataset.healthCode);
          check(`Popover ${state} Health 行保留精确 reason`, expectedCodes.every((code) => renderedCodes.includes(code)));
          const copyable = HEALTH_RECOVERY[state].checks.filter((item) => item.recovery_command || item.diagnostic_command || item.copy_kind);
          check(`Popover ${state} 只给安全可复制动作`, $$(".health-copy-action button").length === copyable.length);
          if (copyable.length > 0) {
            copied = "";
            click($(".health-copy-action button"));
            await wait(40);
            check(`Popover ${state} 复制动作有反馈`, !!copied && !!$(".health-copy-action [role=status]")?.textContent.trim());
          }
          click($(".detail-head button"));
          await wait(50);
          check(`Popover ${state} 可返回主面板`, !$(".detail-head"));
        }

        // The visible check label is product copy, not a wire token. Exercise
        // both shipped languages against each distinct health-recovery check
        // name so a dictionary value that merely echoes its key fails here.
        for (const lang of ["en", "zh"]) {
          click($(`[data-stage-axis="language"] button[data-value="${lang}"]`));
          await wait(60);
          for (const state of ["stateLive", "scanLive", "extensionStale"]) {
            click(healthButton(state));
            await wait(60);
            click($("button.notice"));
            await wait(60);
            const names = HEALTH_RECOVERY[state].checks.map((item) => item.name);
            const visible = $$('[data-health-code] > b').map((node) => node.textContent.trim());
            const expected = names.map((name) => catalogs[lang].status.checks[name]);
            check(`Popover ${state} ${lang} 使用本地化 check labels`, expected.every((label) => visible.includes(label)));
            check(`Popover ${state} ${lang} 不显示内部 check token`, names.every((name) => !visible.includes(name)));
            click($(".detail-head button"));
            await wait(50);
          }
        }
      }

      const failed = results.filter((line) => line.startsWith("FAIL")).length;
      window.healthRecoveryProbeResult = { checks: results.length, failed, results };
      const box = document.createElement("pre");
      box.id = "health-probe-out";
      box.style.cssText = "position:fixed;inset:0;z-index:9999;margin:0;padding:16px;overflow:auto;background:#000;color:#0f0;font:12px ui-monospace;white-space:pre-wrap";
      box.textContent = `${results.join("\n")}\n\n${failed === 0 ? "ALL PASS" : `${failed} FAILED`}`;
      document.body.appendChild(box);
    } catch (error) {
      window.healthRecoveryProbeResult = { checks: results.length, failed: 1, results, error: String(error) };
      const box = document.createElement("pre");
      box.id = "health-probe-out";
      box.style.cssText = "position:fixed;inset:0;z-index:9999;margin:0;padding:16px;overflow:auto;background:#000;color:#f66;font:12px ui-monospace;white-space:pre-wrap";
      box.textContent = `${results.join("\n")}\n\nPROBE CRASHED\n${error?.stack ?? error}`;
      document.body.appendChild(box);
    }
  }, 700);
}
