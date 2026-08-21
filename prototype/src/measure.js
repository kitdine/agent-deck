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
    const box = document.createElement("pre");
    box.style.cssText = "position:fixed;left:0;bottom:0;z-index:999;max-height:46vh;overflow:auto;margin:0;padding:10px;background:#000;color:#0f0;font:11px ui-monospace;white-space:pre-wrap";
    box.textContent = rows.length ? rows.join("\n") : "NO OVERFLOW";
    document.body.appendChild(box);
    document.title = `overflow:${rows.length}`;
  }, 600);
}
