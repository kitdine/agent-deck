// Independent visible-behavior assertions for the shared scan specimens.
export function runScanProbe() {
  if (new URLSearchParams(location.search).get("scanProbe") !== "1") return;
  const results = [];
  const check = (name, ok) => results.push({ name, pass: !!ok });
  const q = (selector) => document.querySelector(selector);
  const click = (selector) => q(selector).click();
  const until = async (predicate) => {
    const deadline = performance.now() + 3000;
    while (!predicate()) {
      if (performance.now() > deadline) throw new Error("DOM condition timed out");
      await new Promise((r) => requestAnimationFrame(r));
    }
  };
  const settle = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  const select = async (selector, value) => { q(selector).value = value; q(selector).dispatchEvent(new Event("change", { bubbles: true })); await settle(); };
  const phase = async (value) => select("[data-scan-phase]", value);
  const run = async () => {
    await until(() => q("[data-scan-phase]"));
    if (q("[data-scan-stdout]")) {
      await select("[data-cli-scope]", "usage");
      await phase("importing");
      check("running progress on stderr only", !q("[data-scan-stdout]").textContent.trim() && /640/.test(q("[data-scan-stderr]").textContent));
      click("[data-scan-next]"); await until(() => q("[data-scan-exit]").textContent === "Exit 0");
      const receipt = q("[data-scan-stdout]").textContent;
      check("usage scope returns before session completion", /usage/.test(receipt) && q("[data-scan-phase]").value === "usage-complete");
      await select("[data-cli-mode]", "json");
      const scoped = JSON.parse(q("[data-scan-stdout]").textContent);
      check("scoped receipt reports ongoing session truthfully", scoped.data.sessions === "processing" && scoped.partial === false);
      await select("[data-cli-mode]", "tty");
      click("[data-scan-next]"); await settle();
      check("background emits no trailing foreground output", q("[data-scan-stdout]").textContent === receipt && !q("[data-scan-stderr]").textContent.trim());
      await select("[data-cli-scope]", "all"); await phase("waiting");
      await select("[data-cli-mode]", "pipe");
      check("non TTY has no progress or control sequences", !q("[data-scan-stderr]").textContent.trim());
      await select("[data-cli-mode]", "quiet");
      check("quiet suppresses progress", !q("[data-scan-stderr]").textContent.trim());
      await phase("failed"); await until(() => q("[data-scan-exit]").textContent === "Exit 1");
      check("quiet preserves errors", /failed/i.test(q("[data-scan-stderr]").textContent));
      await select("[data-cli-mode]", "json");
      const output = JSON.parse(q("[data-scan-stdout]").textContent);
      check("machine result is parseable, no mixed progress", output.command === "scan" && output.partial === true && output.data.sessions === "failed");
      await select("[data-cli-mode]", "tty"); await phase("checking");
      click("[data-cli-detach]"); await settle(); const text = q("[data-scan-stderr]").textContent;
      click("[data-scan-next]"); await settle();
      check("Ctrl-C detaches and freezes output", q("[data-scan-exit]").textContent === "Exit 130" && q("[data-scan-stderr]").textContent === text);
      await phase("waiting"); await select("[data-cli-scope]", "session");
      await phase("usage-complete");
      check("session waiter does not return with usage", q("[data-scan-exit]").textContent === "Foreground running" && /1280/.test(q("[data-scan-stderr]").textContent));
      await select("[data-cli-cols]", "40");
      check("narrow terminal stream has no horizontal overflow", [...document.querySelectorAll(".scan-terminal pre")].every((e) => e.scrollWidth <= e.clientWidth + 1));
    } else {
      await phase("waiting");
      check("unknown totals do not display fictitious count", !q(".scan-domain-counts") && !q("[data-scan-status]").textContent.includes("%"));
      if (q("[data-scan-prior]").checked) { click("[data-scan-prior]"); await settle(); }
      check("first use shows loading without fabricated data cards", !q(".hero") && q(".popover").dataset.scanSnapshot === "none");
      await phase("importing");
      check("source counts represent committed work", /640/.test(q("[data-scan-status]").textContent));
      click(".menubar-item"); await settle();
      check("popover closes independently of worker", !q(".popover"));
      click("[data-scan-next]"); await settle();
      check("background advances without reopening popover", !q(".popover") && q("[data-scan-phase]").value === "usage-complete");
      click(".menubar-item"); await settle();
      check("reopen receives latest phase", q("[data-scan-status]").dataset.phase === "usage-complete");
      click("[data-scan-next]"); await settle();
      check("indexes complete does not publish before statistics", q(".popover").dataset.scanSnapshot === "none");
      click("[data-scan-next]"); await until(() => q(".popover").dataset.scanSnapshot === "new");
      check("completed snapshot published as whole result", !!q(".hero"));
      click(".refresh"); await settle(); click("[data-scan-play]"); await settle();
      check("refresh retains newly obtained prior snapshot", q(".popover").dataset.scanSnapshot === "previous" && !!q(".hero"));
      await phase("failed");
      check("failure retains prior data and retry", !!q(".hero") && !!q("[data-scan-retry]"));
      await phase("partial");
      check("partial outcomes expose independent session failure", /Session scan failed|会话扫描失败/.test(q("[data-scan-status]").textContent));
      check("one atomic stage live region", document.querySelectorAll("[data-scan-status] [role=status]").length === 1 && q("[data-scan-status] [role=status]").getAttribute("aria-atomic") === "true");
      q("[data-scan-phase]").focus(); const focus = document.activeElement;
      await phase("failed");
      check("stage updates do not move focus", document.activeElement === focus);
      check("progress text fits narrow card", [...q("[data-scan-status]").querySelectorAll("p,strong,b,small")].every((e) => e.scrollWidth <= e.clientWidth + 1));
    }
    window.scanProbeResult = { results, passed: results.filter((r) => r.pass).length, failed: results.filter((r) => !r.pass) };
  };
  run().catch((error) => { window.scanProbeResult = { results, error: String(error) }; });
}
