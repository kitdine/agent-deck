// 交互探针：用真实事件走一遍关键路径并断言结果。
// 截图只能证明"长什么样"，证明不了"点了会怎样"，这段补的是后者。
export function runProbe() {
  if (new URLSearchParams(window.location.search).get("probe") !== "1") return;
  const results = [];
  const check = (name, condition) => results.push(`${condition ? "PASS" : "FAIL"}  ${name}`);
  const $ = (selector) => document.querySelector(selector);
  const $$ = (selector) => [...document.querySelectorAll(selector)];
  const click = (node) => node?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  const down = (node) => node?.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  // React 的 onMouseEnter/Leave 是用 mouseover/mouseout 委托实现的，
  // 直接派发 mouseenter 它收不到，探针必须发它真正监听的那两个事件。
  const hover = (node) => node?.dispatchEvent(new MouseEvent("mouseover", { bubbles: true, relatedTarget: document.body }));
  const leave = (node) => node?.dispatchEvent(new MouseEvent("mouseout", { bubbles: true, relatedTarget: document.body }));
  const key = (node, k) => node?.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));
  const wait = (ms) => new Promise((resolve) => window.setTimeout(resolve, ms));

  window.setTimeout(async () => {
    // 客户端筛选联动四个面板
    const before = $(".hero strong").textContent;
    click($$(".segmented.clients button")[1]);
    await wait(60);
    check("切客户端后 hero 跟着变", $(".hero strong").textContent !== before);

    click($(".tabs button:nth-child(3)"));
    await wait(60);
    const codexTrust = $(".data-row b")?.textContent;
    click($$(".segmented.clients button")[2]);
    await wait(60);
    check("归因随客户端联动（这是 v6 不动的地方）", $(".data-row b")?.textContent !== codexTrust);

    // 时段联动归因：数据侧还需要补 period 分组，界面先按联动设计
    const claudeToday = $(".data-row b")?.textContent;
    click($$(".segmented.periods button")[2]);
    await wait(60);
    check("归因随时段联动", $(".data-row b")?.textContent !== claudeToday);

    click($$(".segmented.clients button")[0]);
    click($$(".segmented.periods button")[0]);
    click($(".tabs button:nth-child(1)"));
    await wait(80);

    // 趋势图：悬停出读数、移出清掉、点击钉住
    const bars = $$(".bar");
    check("趋势图有柱子", bars.length > 0);
    hover(bars[3]);
    await wait(40);
    check("悬停出现读数", !$(".readout").classList.contains("idle"));
    leave($(".bars"));
    await wait(40);
    check("移出后读数恢复（v6 会一直粘住）", $(".readout").classList.contains("idle"));
    click(bars[5]);
    await wait(40);
    check("点击钉住读数", !!$(".readout .pin"));
    leave($(".bars"));
    await wait(40);
    check("钉住后移出仍保留", !!$(".readout .pin"));
    click(bars[5]);
    await wait(40);
    check("再点取消钉住", !$(".readout .pin"));

    // 键盘：整张图只占一个 Tab 位，方向键在柱子间移动
    check("图表只有一个 Tab 停留点", $$(".bar[tabindex='0']").length === 1);
    bars[0].focus();
    key(bars[0], "ArrowRight");
    await wait(40);
    check("方向键移动焦点", document.activeElement === $$(".bar")[1]);

    // 服务商菜单：点外部关闭
    click($(".provider-entry"));
    await wait(60);
    check("服务商菜单打开", !!$(".provider-menu"));
    down(document.body);
    await wait(60);
    check("点空白处关闭菜单（v6 只能按 Esc）", !$(".provider-menu"));

    // 切换服务商要确认，且不可用项不可点
    click($(".provider-entry"));
    await wait(60);
    const items = $$('.provider-menu button[role="menuitem"]');
    check("未配置 wrapper 的选项被禁用", items.some((node) => node.disabled));
    click(items.find((node) => !node.disabled && !node.querySelector("svg")));
    await wait(60);
    check("切换服务商前弹确认", !!$(".dialog"));
    check("确认框自动聚焦（可直接 Esc/Tab）", $(".dialog").contains(document.activeElement));
    key(window, "Escape");
    await wait(60);
    check("Esc 关闭确认框", !$(".dialog"));

    // 健康提示：在内容区顶部，点进去是二级页面而不是就地展开
    const notice = $("button.notice");
    check("异常时出现健康提示条", !!notice);
    check("footer 只剩服务商一行", $$(".popover > footer > *").length === 1);
    check("更新时间在右上角", !!$(".header-right .freshness"));
    check("提示条在内容区内，不覆盖内容", !!$(".scroll .notices"));
    click(notice);
    await wait(60);
    check("健康详情是二级页面", $$(".list-row").length >= 5 && !!$(".detail-head"));
    check("健康详情不挤占 footer", !$(".popover > footer .notices"));
    click($(".detail-head button"));
    await wait(60);
    check("能从健康详情返回", !$(".detail-head"));

    // 会话 tab 的三块信号进得去、回得来，且详情态 tab 仍保持选中
    click($(".tabs button:nth-child(4)"));
    await wait(60);
    click($(".signal-card"));
    await wait(60);
    check("工作信号可进入详情", !!$(".detail-head"));
    check("详情态下所属 tab 仍选中（v6 会四个全灰）", $(".tabs button:nth-child(4)").classList.contains("active"));
    check("详情里标注了待采集", !!$(".pending-banner"));
    click($(".detail-head button"));
    await wait(60);
    check("能返回会话总览", !$(".detail-head") && !!$(".signal-grid"));

    // 刷新：第一次失败保留旧数据并给重试
    click($(".refresh"));
    await wait(1400);
    check("刷新失败时保留数据并给重试", !!$(".refresh.failed button") && !!$(".hero strong"));
    click($(".refresh.failed button"));
    await wait(1500);
    check("重试后恢复", !$(".refresh.failed"));

    // 菜单栏图标：右键与双击都要能弹出菜单，菜单里的设置要真的打得开
    const item = $(".menubar-item");
    item.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true }));
    await wait(60);
    check("右键弹出菜单", !!$(".menubar-menu"));
    down(document.body);
    await wait(60);
    check("点空白处关闭菜单栏菜单", !$(".menubar-menu"));
    item.dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    await wait(60);
    check("双击也弹出菜单", !!$(".menubar-menu"));

    click($(".menu-settings"));
    await wait(80);
    check("菜单里能打开设置窗口", !!$(".settings-window"));
    const switches = $$(".settings-window .switch");
    check("设置里有开关", switches.length === 2);
    // 定时刷新没有被拒路径，普通切换用它验；登录项的拒绝路径在下面单独走。
    const wasOn = switches[1].classList.contains("on");
    click(switches[1]);
    await wait(60);
    check("开关可切换", switches[1].classList.contains("on") !== wasOn);

    // 每个偏好的解释文字必须是控件的 accessible description，靠 aria-describedby 指过去，
    // 而不是靠视觉上排在下面。这里连 ID 指向的文本一起比，空 ID 或指向不存在的节点都会挂。
    const described = $$(".settings-window [role=switch], .settings-window [role=radiogroup]");
    check("四项偏好都有控件", described.length === 4);
    check(
      "每项偏好的解释文字都是控件的 description",
      described.every((node) => {
        const target = document.getElementById(node.getAttribute("aria-describedby") ?? "");
        return !!target?.textContent.trim();
      }),
    );

    // 登录项被拒：SMAppService 会拒绝注册，契约要求开关留在真实状态、失败行出现并被宣布、
    // 其余控件不禁用、也不弹 modal，且失败行在下一次成功修改时清除而不是定时消失。
    const loginError = () => $(".settings-window .settings-error");
    check("失败行的 live region 在失败前就已存在且为空", !!loginError() && !loginError().textContent.trim());
    click(switches[0]);
    await wait(60);
    check("登录项被拒后开关留在关闭", !switches[0].classList.contains("on"));
    check("登录项被拒后出现失败行", !!loginError().textContent.trim());
    check("失败行带图标而不是只靠颜色", !!loginError().querySelector("svg"));
    check(
      "一次失败只落在一个 live region 里",
      $$(".settings-window [role=status]").filter((node) => node.textContent.trim()).length === 1,
    );
    check(
      "登录项被拒不禁用其余控件，也不弹 modal",
      $$(".settings-window button").every((node) => !node.disabled) && $$("[role=dialog]").length === 1,
    );
    click(switches[0]);
    await wait(60);
    check("再次开启成功后开关打开", switches[0].classList.contains("on"));
    check("失败行在下一次成功修改时清除", !loginError().textContent.trim());

    // 菜单栏显示模式：切到仅图标后金额不再常驻
    click($$(".settings-segmented button")[2]);
    await wait(60);
    check("切到仅图标后菜单栏不再显示金额", !$(".menubar-item strong"));
    click($$(".settings-segmented button")[1]);
    await wait(60);
    check("切到 Token 后菜单栏显示 token", ($(".menubar-item strong")?.textContent ?? "").includes("M"));
    click($$(".settings-segmented button")[0]);
    await wait(60);

    key(window, "Escape");
    await wait(80);
    check("Esc 关闭设置窗口", !$(".settings-window"));


    const box = document.createElement("pre");
    box.id = "probe-out";
    box.style.cssText = "position:fixed;inset:0;z-index:9999;margin:0;padding:16px;overflow:auto;background:#000;color:#0f0;font:12px ui-monospace;white-space:pre-wrap";
    const failed = results.filter((line) => line.startsWith("FAIL")).length;
    box.textContent = `${results.join("\n")}\n\n${failed === 0 ? "ALL PASS" : failed + " FAILED"}`;
    document.body.appendChild(box);
  }, 700);
}
