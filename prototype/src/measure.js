// 临时量具：把所有溢出的容器标出来，避免靠眼睛猜哪一格放不下。
export function runMeasure() {
  if (new URLSearchParams(window.location.search).get("measure") !== "1") return;
  window.setTimeout(() => {
    const rows = [];
    // 内容区本来就可滚动，这里只查绝不该出现滚动的容器
    document.querySelectorAll(".widget").forEach((node) => {
      const over = node.scrollHeight - node.clientHeight;
      if (over > 1) {
        rows.push(`${node.className.split(" ").slice(0, 2).join(".")} | ${node.getAttribute("aria-label") ?? ""} | over ${over}px (${node.scrollHeight}/${node.clientHeight})`);
      }
    });
    document.querySelectorAll(".widget .widget-body > *").forEach((node) => {
      const parent = node.closest(".widget");
      const pb = parent.querySelector(".widget-body").getBoundingClientRect();
      const nb = node.getBoundingClientRect();
      if (nb.bottom > pb.bottom + 1) {
        rows.push(`CLIPPED ${parent.getAttribute("aria-label")} → ${node.className || node.tagName} by ${(nb.bottom - pb.bottom).toFixed(0)}px`);
      }
    });
    // 被 ellipsis 吃掉的一行文字，容器并没有被裁，上面两条都看不见它。
    // 面板里凡是 text-overflow:ellipsis 的元素都在这里逐个量一遍，报出还差多少像素——
    // 「这一行在窄边界会不会被截断」于是成为标本阶段能判定的问题，而不是留给真机观察。
    const seen = new Set();
    document.querySelectorAll(".popover, .settings-window, .stage-body").forEach((root) => {
      root.querySelectorAll("*").forEach((node) => {
        if (seen.has(node)) return;
        seen.add(node);
        if (getComputedStyle(node).textOverflow !== "ellipsis") return;
        const over = node.scrollWidth - node.clientWidth;
        if (over <= 1) return;
        const width = node.closest("[data-width]")?.dataset.width ?? "";
        const path = [node.tagName.toLowerCase(), node.className || null].filter(Boolean).join(".");
        rows.push(`TRUNCATED ${width ? `@${width}pt ` : ""}${path} | "${node.textContent}" | short by ${over}px (${node.scrollWidth}/${node.clientWidth})`);
      });
    });
    const box = document.createElement("pre");
    box.style.cssText = "position:fixed;left:0;bottom:0;z-index:999;max-height:46vh;overflow:auto;margin:0;padding:10px;background:#000;color:#0f0;font:11px ui-monospace;white-space:pre-wrap";
    box.textContent = rows.length ? rows.join("\n") : "NO OVERFLOW";
    document.body.appendChild(box);
    document.title = `overflow:${rows.length}`;
  }, 600);
}
