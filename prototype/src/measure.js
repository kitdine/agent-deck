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
    // 横向溢出：flex 会把子项压缩到比它的文字还窄，于是盒子留在界内、文字顶出去，
    // 既没有滚动容器也没有 ellipsis——上面两条和「比较矩形」都看不见它。
    // 头部的刷新文字就是这样顶出过面板右缘而量具报 NO OVERFLOW。
    // 判据换成 scrollWidth 与 clientWidth：本来就该横向滚动的容器排除在外。
    document.querySelectorAll(".popover, .settings-window, .quota-flyout, .stage-controls").forEach((root) => {
      root.querySelectorAll("*").forEach((node) => {
        // 420pt 下弹层有意位于 popover 右侧，不能拿 popover 的边界判它溢出；
        // 弹层自己也是一个 root，内部仍会被完整测量。
        if (root.matches(".popover") && node.closest(".quota-flyout")) return;
        const style = getComputedStyle(node);
        if (style.textOverflow === "ellipsis") return; // 下一条专门量它
        if (style.overflowX === "auto" || style.overflowX === "scroll") return;
        // 视觉隐藏（clip-path 掐成 1px，文字留给读屏）是有意为之，不是溢出。
        if (style.clipPath !== "none") return;
        const over = node.scrollWidth - node.clientWidth;
        if (over <= 1) return;
        const path = [node.tagName.toLowerCase(), node.className || null].filter(Boolean).join(".");
        rows.push(`OVERFLOW-X ${path} | "${(node.textContent ?? "").slice(0, 34)}" | over ${over}px (${node.scrollWidth}/${node.clientWidth})`);
      });
    });

    // 被 ellipsis 吃掉的一行文字，容器并没有被裁，上面两条都看不见它。
    // 面板里凡是 text-overflow:ellipsis 的元素都在这里逐个量一遍，报出还差多少像素——
    // 「这一行在窄边界会不会被截断」于是成为标本阶段能判定的问题，而不是留给真机观察。
    const seen = new Set();
    document.querySelectorAll(".popover, .settings-window, .quota-flyout, .stage-controls, .stage-body, .board").forEach((root) => {
      root.querySelectorAll("*").forEach((node) => {
        if (seen.has(node)) return;
        seen.add(node);
        const style = getComputedStyle(node);
        if (style.textOverflow !== "ellipsis") return;
        // 与上一条同一个排除：clip-path 掐成 1px 的标签是留给读屏的，不是被截断的。
        // tab 标签在 280pt 正是这个形态，少了这一条会报出五个必然的假阳性。
        if (style.clipPath !== "none") return;
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
