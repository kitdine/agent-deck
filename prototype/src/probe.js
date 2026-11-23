import { catalogs, spokenMaskedEmail } from "./i18n.js";

export function hasOnlyMaskedVisualEmails(root) {
  if (!root) return true;
  const texts = [root.textContent ?? ""];
  for (const node of [root, ...(root.querySelectorAll?.("*") ?? [])]) {
    const title = node.getAttribute?.("title");
    if (title) texts.push(title);
  }
  return texts.every((text) => {
    const matches = text.match(/\S+@\S+/g) ?? [];
    return matches.every((token) => {
      const at = token.indexOf("@");
      return at > 0 && at === token.lastIndexOf("@") && token.slice(0, at).includes("***");
    });
  });
}

export function hasOnlyAllowedSpokenEmails(root, dict) {
  if (!root) return true;
  const emailNodes = root.querySelectorAll(".quota-identity-email, .w-quota-signed-in-text");
  const maskedAddresses = new Set(
    Array.from(emailNodes).flatMap((node) => node.textContent.match(/\S*\*\*\*\S*@\S+/g) ?? []),
  );
  const allowed = Array.from(maskedAddresses, (email) => spokenMaskedEmail(email, dict));
  return [root, ...root.querySelectorAll("*")].every((node) => {
    const aria = node.getAttribute?.("aria-label");
    if (!aria) return true;
    if (aria.includes("***")) return false;
    return !allowed.reduce((text, email) => text.replaceAll(email, ""), aria).includes("@");
  });
}

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
  // 按 key 找 tab，不按位置。位置写死过一次的代价：额度 tab 插到最前面之后，
  // nth-child(1) 变成了额度，探针在趋势图上取到空集合并崩在下一行，
  // 而崩掉的探针什么都不渲染——看上去和"没跑"一模一样。
  const tabButton = (key) => $(`.tabs button[data-tab="${key}"]`);
  // 探针跑在当前语言下，断言不能写死中文字面量——文案表才是真相。
  // 语言要在断言执行时读，不能在 runProbe 进来时读：那一刻 React 还没把
  // lang 落到 <html> 上，en 会静默拿到中文文案表，断言于是永远为假。
  const dict = () => catalogs[document.documentElement.lang.startsWith("en") ? "en" : "zh"];

  const runSyntheticLeakNegativeCheck = () => {
    const leakVisual = document.createElement("div");
    leakVisual.textContent = "alice@example.test";
    check("合成反例：可见未脱敏邮箱被隐私检查拦截", !hasOnlyMaskedVisualEmails(leakVisual));

    const leakSpoken = document.createElement("div");
    const masked = document.createElement("span");
    masked.className = "quota-identity-email";
    masked.textContent = "a***e@example.test";
    leakSpoken.append(masked);
    const spoken = spokenMaskedEmail(masked.textContent, dict());
    leakSpoken.setAttribute("aria-label", spoken);
    check("合成正例：无障碍口语形对应可见脱敏邮箱", hasOnlyAllowedSpokenEmails(leakSpoken, dict()));
    leakSpoken.setAttribute("aria-label", `${spoken} alice@example.test`);
    check(
      "合成反例：无障碍未脱敏邮箱被隐私检查拦截",
      !hasOnlyAllowedSpokenEmails(leakSpoken, dict()),
    );
  };

  window.setTimeout(async () => {
    try {
    if (new URLSearchParams(window.location.search).get("surface") === "widgets") {
      const quotaVariant = (key) => click($(`.stage-group button[data-value="${key}"]`));
      const widgetClientBtn = (key) => click($(`.stage-group button[data-value="${key}"]`));
      const qCard = (size) =>
        $$(`.widget-${size}`).find((node) =>
          (node.getAttribute("aria-label") ?? "").startsWith(dict().widgets.kinds.quota.title),
        );

      runSyntheticLeakNegativeCheck();

      quotaVariant("identityUnconfirmed");
      await wait(80);
      const qSmall = qCard("small");
      const qMedium = qCard("medium");
      const qLarge = qCard("large");
      check("Widget small 无 @ 字符", !(qSmall?.textContent ?? "").includes("@"));
      check("Widget medium 无 @ 字符", !(qMedium?.textContent ?? "").includes("@"));
      const largeAtNodes = [...(qLarge?.querySelectorAll("*") ?? [])].filter(
        (n) => n.children.length === 0 && (n.textContent ?? "").includes("@"),
      );
      check(
        "Widget large @ 只出现在当前登录行",
        largeAtNodes.length === 2 && largeAtNodes.every((n) => !!n.closest(".w-quota-signed-in")),
      );
      check("Widget large 可见文本仅含脱敏邮箱", hasOnlyMaskedVisualEmails(qLarge));
      check(
        "Widget large 无障碍标签使用口语脱敏邮箱且无星号",
        hasOnlyAllowedSpokenEmails(qLarge, dict()),
      );
      check(
        "Widget large Codex 登录行内联标出与额度关联未确认（F4）",
        (qLarge?.querySelector('.w-quota-client[data-client="codex"] .w-quota-signed-in')?.textContent ?? "").includes(
          dict().widgets.quotaAssociationUnconfirmed,
        ),
      );
      check(
        "Widget large Codex 块显示紧凑套餐名 Pro 100，Claude 块不显示套餐",
        qLarge?.querySelector('.w-quota-client[data-client="codex"] header .w-quota-plan')?.textContent === "Pro 100" &&
          !qLarge?.querySelector('.w-quota-client[data-client="claude"] header .w-quota-plan'),
      );
      check(
        "Widget 不显示组织或工作区名称",
        [qSmall, qMedium, qLarge].every(
          (node) => !(node?.textContent ?? "").includes("Acme Studio") && !(node?.textContent ?? "").includes("Kitdine Labs"),
        ),
      );

      quotaVariant("identityConfirmed");
      await wait(80);
      check(
        "Widget large Codex 已确认时登录行不再出现未确认后缀（F4）",
        !(qCard("large")?.querySelector('.w-quota-client[data-client="codex"] .w-quota-signed-in')?.textContent ?? "").includes(
          dict().widgets.quotaAssociationUnconfirmed,
        ),
      );

      quotaVariant("identityUnknown");
      await wait(80);
      check(
        "Widget large 非 current/identified 时不显示当前登录行且无 @",
        !qCard("large")?.querySelector(".w-quota-signed-in") && !(qCard("large")?.textContent ?? "").includes("@"),
      );

      quotaVariant("expiredMailbox");
      await wait(80);
      check(
        "Widget large current_until 已过期时不显示当前登录行（A11）",
        !qCard("large")?.querySelector(".w-quota-signed-in") && !(qCard("large")?.textContent ?? "").includes("@"),
      );

      quotaVariant("contextChanged");
      await wait(80);
      check(
        "Widget contextChanged 下 large 只剩 Codex 块",
        [...(qCard("large")?.querySelectorAll(".w-quota-client") ?? [])].map((n) => n.dataset.client).join(",") === "codex",
      );
      widgetClientBtn("claude");
      await wait(80);
      check(
        "Widget small/medium 选定端失效时页脚显示登录已变化",
        qCard("small")?.querySelector("footer")?.textContent === dict().widgets.quotaContextChanged &&
          qCard("medium")?.querySelector("footer")?.textContent === dict().widgets.quotaContextChanged,
      );
      quotaVariant("contextKeyReset");
      await wait(80);
      check(
        "Widget small/medium 关联密钥重置时页脚说明旧额度无法关联",
        qCard("small")?.querySelector("footer")?.textContent === dict().widgets.quotaIdentityKeyReset &&
          qCard("medium")?.querySelector("footer")?.textContent === dict().widgets.quotaIdentityKeyReset,
      );
      widgetClientBtn("codex");
      await wait(60);

      quotaVariant("contextRecovered");
      await wait(80);
      check(
        "Widget contextRecovered 下 Claude 在首次新鲜 usage 后立即恢复展示（A5.6）",
        [...(qCard("large")?.querySelectorAll(".w-quota-client") ?? [])].map((n) => n.dataset.client).join(",") === "codex,claude",
      );

      quotaVariant("longPlan");
      await wait(80);
      const longPlanLarge = qCard("large");
      const largeBox = longPlanLarge?.getBoundingClientRect();
      const codexPlanChip = longPlanLarge?.querySelector('.w-quota-client[data-client="codex"] header .w-quota-plan');
      const claudeAttr = longPlanLarge?.querySelector('.w-quota-client[data-client="claude"] .w-attribution');
      const amounts = [...(longPlanLarge?.querySelectorAll(".w-quota-row > div:first-child strong") ?? [])];
      check(
        "Widget longPlan 64 字符套餐省略且不挤出卡片与百分比/归属（F1）",
        !!codexPlanChip &&
          codexPlanChip.scrollWidth > codexPlanChip.clientWidth &&
          !!largeBox &&
          (longPlanLarge?.scrollHeight ?? 0) <= (longPlanLarge?.clientHeight ?? 0) + 1 &&
          !!claudeAttr &&
          claudeAttr.getBoundingClientRect().right <= largeBox.right + 1 &&
          amounts.length === 5 &&
          amounts.every((amount) => {
            const box = amount.getBoundingClientRect();
            return box.width > 0 && box.left >= largeBox.left && box.right <= largeBox.right + 1;
          }),
      );

      const box = document.createElement("pre");
      box.id = "probe-out";
      box.style.cssText = "position:fixed;inset:0;z-index:9999;margin:0;padding:16px;overflow:auto;background:#000;color:#0f0;font:12px ui-monospace;white-space:pre-wrap";
      const failed = results.filter((line) => line.startsWith("FAIL")).length;
      box.textContent = `${results.join("\n")}\n\n${failed === 0 ? "ALL PASS" : failed + " FAILED"}`;
      document.body.appendChild(box);
      return;
    }

    // 每条路径从明确的普通状态开始；待采集在下面单独进入并验证。
    const stateButton = (state) => $(`.stage-group button[data-value="${state}"]`);
    click(stateButton("normal"));
    await wait(60);
    // 客户端筛选联动四个面板
    const before = $(".hero strong").textContent;
    click($$(".segmented.clients button")[1]);
    await wait(60);
    check("切客户端后 hero 跟着变", $(".hero strong").textContent !== before);

    click(tabButton("attribution"));
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
    click(tabButton("usage"));
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
    click(tabButton("sessions"));
    await wait(60);
    click($(".signal-card"));
    await wait(60);
    check("工作信号可进入详情", !!$(".detail-head"));
    check("详情态下所属 tab 仍选中（v6 会四个全灰）", tabButton("sessions").classList.contains("active"));
    check("普通详情不误标待采集", !$(".pending-banner"));
    click($(".detail-head button"));
    await wait(60);
    check("能返回会话总览", !$(".detail-head") && !!$(".signal-grid"));

    click(stateButton("pending"));
    await wait(60);
    check("待采集路径确实选择了 pending 状态", stateButton("pending")?.classList.contains("active"));
    for (let index = 0; index < 3; index += 1) {
      // 返回后 React 会重建总览，不能复用上一次的已卸载节点。
      click($$(".signal-card")[index]);
      await wait(60);
      check("待采集详情显示完整本地化提示", $(".pending-banner span")?.textContent === dict().sessions.pendingHint);
      click($(".detail-head button"));
      await wait(60);
    }
    check("待采集总览保留三种信号入口", $$(".signal-card").length === 3);
    click(stateButton("normal"));
    await wait(60);

    // 刷新：第一次失败保留旧数据并给重试
    click($(".refresh"));
    await wait(1400);
    check("刷新失败时保留数据并给重试", !!$("button.refresh.failed") && !!$(".hero strong"));
    click($("button.refresh.failed"));
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
    // 按分组定位，不按窗口内的序号。额度分组加进来之后，"窗口里有几个开关"
    // 这个数字就不再是常规分组的性质了——它变成一个每加一个偏好就要改一次的
    // 常量，而改它的人未必知道原来那条断言想说什么。
    const group = (name) => $(`.settings-window [data-group="${name}"]`);
    const switches = $$('[data-group="general"] .switch');
    check("常规分组有两个开关", switches.length === 2);
    // 定时刷新没有被拒路径，普通切换用它验；登录项的拒绝路径在下面单独走。
    const wasOn = switches[1].classList.contains("on");
    click(switches[1]);
    await wait(60);
    check("开关可切换", switches[1].classList.contains("on") !== wasOn);

    // 每个偏好的解释文字必须是控件的 accessible description，靠 aria-describedby 指过去，
    // 而不是靠视觉上排在下面。这里连 ID 指向的文本一起比，空 ID 或指向不存在的节点都会挂。
    // 断言的是"渲染出解释文字的控件，那段文字必须真的是它的 accessible
    // description"。没有解释文字的控件没有可错配的东西，把它们算进一个写死的
    // 总数只会让断言在下一次加偏好时失效，而失效的方式是报告一个不存在的缺陷。
    const described = $$(".settings-window [role=switch], .settings-window [role=radiogroup]").filter((node) =>
      node.hasAttribute("aria-describedby"),
    );
    check("带解释文字的偏好控件不止一个", described.length >= 4);
    check(
      "每项解释文字都是控件的 description",
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
    // 原文是"其余控件都没被禁用"。额度分组里的依赖控件本来就该在父开关关闭时
    // 置灰，那是 ux/settings-quota.md 的设计，不是这次拒绝造成的。断言要问的是
    // "这次拒绝有没有波及无关控件"，所以范围收到与它无关的那一组。
    check(
      "登录项被拒不禁用同组的其余控件，也不弹 modal",
      [...group("general").querySelectorAll("button")].every((node) => !node.disabled) &&
        $$("[role=dialog]").length === 1,
    );
    click(switches[0]);
    await wait(60);
    check("再次开启成功后开关打开", switches[0].classList.contains("on"));
    check("失败行在下一次成功修改时清除", !loginError().textContent.trim());

    // 状态栏通路的两种失败。这是本产品唯一写别人文件的开关，两种结果必须
    // 分得开：写入被拒是没做成，开关必须留在关闭；关闭时的恢复冲突是做了一半，
    // 命令移除了但原值没还原，只能报告并请人工检查。
    // 语义色要按最终计算颜色断言，不能按类名。类名对不等于颜色渲染出来了：
    // 上一轮 .settings-label small 的权重压过 .tone-text-*，两条提示实际都是灰的，
    // 而只查类名的断言照样全绿——一个只会在改类名时失败的断言，防不住样式覆盖。
    // 期望值从 :root 上现取，因为深浅两套外观的 token 值不同。
    const token = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    const rgb = (value) => {
      const probeNode = document.createElement("span");
      probeNode.style.color = value;
      document.body.appendChild(probeNode);
      const resolved = getComputedStyle(probeNode).color;
      probeNode.remove();
      return resolved;
    };
    const paintedColor = (node) => (node ? getComputedStyle(node).color : null);

    const quotaSwitch = (index) => $$('[data-group="quota"] .switch')[index];
    const statusRow = () => quotaSwitch(1)?.closest(".settings-field")?.querySelector(".settings-error");
    check("读取额度关闭时状态栏开关是禁用而不是隐藏", !!quotaSwitch(1) && quotaSwitch(1).disabled);
    click(quotaSwitch(0));
    await wait(60);
    check("打开读取额度后状态栏开关可用", !quotaSwitch(1).disabled);
    check("失败行的 live region 在失败前就已存在且为空", !!statusRow() && !statusRow().textContent.trim());

    // "不禁用其余控件"要按变化量问，不按当下有没有禁用的控件问：额度分组本来
    // 就有依赖置灰的控件——提醒关着时"窗口重置时提醒"就是灰的，那是这个分组的
    // 设计。断言要抓的是"这次拒绝有没有额外灰掉什么"。
    const disabledSet = () =>
      [...group("quota").querySelectorAll("button")].map((node) => (node.disabled ? "1" : "0")).join("");
    const disabledBefore = disabledSet();

    click(quotaSwitch(1));
    await wait(60);
    check("写入被拒后状态栏开关留在关闭", !quotaSwitch(1).classList.contains("on"));
    check("写入被拒出现失败行", !!statusRow().textContent.trim());
    check("写入被拒的失败行带图标而不是只靠颜色", !!statusRow().querySelector("svg"));
    check(
      "写入被拒真的画成了错误色（比计算颜色，不比类名）",
      paintedColor(statusRow().querySelector("small")) === rgb(token("--bad")),
    );
    check(
      "写入被拒的颜色不是普通灰",
      paintedColor(statusRow().querySelector("small")) !== rgb(token("--dim")),
    );
    const refusedColor = paintedColor(statusRow().querySelector("small"));
    check(
      "写入被拒时只有一个 live region 有内容",
      $$(".settings-window [role=status]").filter((node) => node.textContent.trim()).length === 1,
    );
    check("写入被拒没有额外禁用任何控件", disabledSet() === disabledBefore);

    click(quotaSwitch(1));
    await wait(60);
    check("再打开一次即重试，成功后开关打开", quotaSwitch(1).classList.contains("on"));
    check("成功后失败行清除", !statusRow().textContent.trim());

    click(quotaSwitch(1));
    await wait(60);
    check("恢复不完整时开关确实关闭了（移除本身做成了）", !quotaSwitch(1).classList.contains("on"));
    check("恢复不完整出现提示行", !!statusRow().textContent.trim());
    check(
      "恢复不完整真的画成了警告色（比计算颜色，不比类名）",
      paintedColor(statusRow().querySelector("small")) === rgb(token("--warn")),
    );
    // 拿这一轮真实画出来的两个颜色互比，而不是比两个 token。比 token 的话，
    // 两条提示都被覆盖成同一个灰色时它照样通过——那正是要防的情形。
    check(
      "两种失败画出来的颜色确实不同",
      !!refusedColor && paintedColor(statusRow().querySelector("small")) !== refusedColor,
    );
    click(quotaSwitch(1));
    await wait(60);
    check("下一次成功修改清除恢复提示", !statusRow().textContent.trim());
    click(quotaSwitch(1));
    await wait(60);
    click(quotaSwitch(0));
    await wait(60);

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


    // 额度 tab。这一段是 ux/menubar-quota.md 的交互合同：明细弹层悬停即开、
    // 账号归属声明只挂在 Claude、读取关闭是自成一格的第三种状态。
    // 截图证明不了其中任何一条。
    const quotaVariant = (key) => click($(`.stage-group button[data-value="${key}"]`));
    const card = (client) => $(`.quota-client[data-client="${client}"]`);

    click(tabButton("quota"));
    await wait(80);
    check("额度 tab 可选中", tabButton("quota").classList.contains("active"));

    check("Claude 卡声明账号归属不可确认", !!card("claude")?.querySelector(".quota-attribution"));
    check("Codex 卡不带这句声明（它有 accountId）", !card("codex")?.querySelector(".quota-attribution"));
    check("门禁下 Codex 无身份区域，Claude 有身份区域", !card("codex")?.querySelector(".quota-identity") && !!card("claude")?.querySelector(".quota-identity"));

    runSyntheticLeakNegativeCheck();

    quotaVariant("identityUnconfirmed");
    await wait(80);
    check(
      "Codex 标题胶囊显示紧凑套餐名且含原值说明",
      card("codex")?.querySelector(".quota-plan")?.textContent === "Pro 100" &&
        (card("codex")?.querySelector(".quota-plan")?.getAttribute("title") ?? "").includes("prolite"),
    );
    check("Codex 套餐相同时省略身份侧套餐且仍显示工作区（F10）",
      (card("codex")?.querySelector(".quota-identity-org")?.textContent ?? "").includes("Acme Studio") &&
        !card("codex")?.querySelector(".quota-identity-plan"),
    );
    check(
      "Claude 额度侧套餐提示写明来源且身份侧标清登录套餐（F5）",
      (card("claude")?.textContent ?? "").includes(dict().quota.reasons.plan_not_reported) &&
        (card("claude")?.querySelector(".quota-identity-plan")?.textContent ?? "").startsWith(`${dict().quota.identity.planPrefix}: `),
    );
    check("Popover 可见文本仅含脱敏邮箱（F9）", hasOnlyMaskedVisualEmails($(".popover")));
    check(
      "Popover 无障碍名称仅含允许的口语脱敏邮箱（F9）",
      hasOnlyAllowedSpokenEmails($(".popover"), dict()),
    );
    check(
      "Codex 关联说明存在且 Claude 不重复",
      card("codex")?.querySelector(".quota-identity-assoc")?.textContent === dict().quota.identity.associationUnconfirmed &&
        !card("claude")?.querySelector(".quota-identity-assoc"),
    );

    click($('.stage-group button[data-value="280"]'));
    await wait(80);
    const narrowLabel = card("codex")?.querySelector(".quota-identity-label");
    const narrowPill = card("codex")?.querySelector(".quota-plan");
    check(
      "280 pt 时登录标签保持可见并进入 aria-label（F3）",
      $(".popover")?.classList.contains("narrow") &&
        !!narrowLabel &&
        getComputedStyle(narrowLabel).clipPath === "none" &&
        narrowLabel.getBoundingClientRect().width > 0 &&
        (card("codex")?.querySelector(".quota-identity")?.getAttribute("aria-label") ?? "").includes(dict().quota.identity.title),
    );
    check(
      "280 pt 时 Pro 100 胶囊保持单行（F6）",
      !!narrowPill && narrowPill.getBoundingClientRect().height <= 20,
    );

    quotaVariant("longPlan");
    await wait(80);
    const longCodexCard = card("codex");
    const longCodexPill = longCodexCard?.querySelector(".quota-plan");
    const longClaudePlan = card("claude")?.querySelector(".quota-identity-plan");
    check(
      "280 pt 下 64 字符安全套餐省略且不超出卡片（F1）",
      !!longCodexPill &&
        longCodexPill.scrollWidth > longCodexPill.clientWidth &&
        longCodexPill.getBoundingClientRect().right <= longCodexCard.getBoundingClientRect().right + 1 &&
        !!longClaudePlan &&
        longClaudePlan.scrollWidth > longClaudePlan.clientWidth &&
        longClaudePlan.getBoundingClientRect().right <= card("claude").getBoundingClientRect().right + 1,
    );

    click($('.stage-group button[data-value="420"]'));
    await wait(80);
    check(
      "420 pt 下 64 字符安全套餐同样不超出卡片（F1）",
      (card("codex")?.querySelector(".quota-plan")?.getBoundingClientRect().right ?? 0) <=
        (card("codex")?.getBoundingClientRect().right ?? 0) + 1,
    );

    quotaVariant("identityConfirmed");
    await wait(80);
    check(
      "Codex 已确认变体展示同一账号关联文案",
      card("codex")?.querySelector(".quota-identity-assoc")?.textContent === dict().quota.identity.associationConfirmed &&
        !card("claude")?.querySelector(".quota-identity-assoc"),
    );

    quotaVariant("identityUnknown");
    await wait(80);
    const codexIdChildren = [...(card("codex")?.querySelector(".quota-identity")?.children ?? [])];
    const codexMetaIdx = codexIdChildren.findIndex((n) => n.classList.contains("quota-identity-meta"));
    const codexCtxIdx = codexIdChildren.findIndex((n) => n.classList.contains("quota-identity-context"));
    const codexAria = card("codex")?.querySelector(".quota-identity")?.getAttribute("aria-label") ?? "";
    check(
      "身份未知保留旧邮箱时明确标为上次识别并相邻显示原时间（F2）",
      card("codex")?.querySelector(".quota-identity-label")?.textContent === dict().quota.identity.lastKnown &&
        !!card("codex")?.querySelector(".quota-identity-email.is-dim") &&
        codexMetaIdx === 1 &&
        (codexCtxIdx === -1 || codexMetaIdx < codexCtxIdx) &&
        (card("codex")?.querySelector(".quota-identity-meta")?.textContent ?? "").includes(dict().quota.identity.reasons.probe_failed) &&
        codexAria.includes(dict().quota.identity.lastKnown) &&
        !codexAria.includes(`${dict().clients.codex} ${dict().quota.identity.title}`),
    );
    check(
      "身份未知无旧值显示未知状态词与原因且无邮箱",
      card("claude")?.querySelector(".quota-identity-status")?.textContent === dict().quota.identity.unknown &&
        !(card("claude")?.querySelector(".quota-identity")?.textContent ?? "").includes("@") &&
        (card("claude")?.querySelector(".quota-identity-meta")?.textContent ?? "").includes(dict().quota.identity.reasons.incomparable),
    );

    quotaVariant("expiredMailbox");
    await wait(80);
    check(
      "expiredMailbox 下 Popover 标为上次识别的登录并弱化邮箱",
      card("claude")?.querySelector(".quota-identity-label")?.textContent === dict().quota.identity.lastKnown &&
        !!card("claude")?.querySelector(".quota-identity-email.is-dim"),
    );

    quotaVariant("contextChanged");
    await wait(80);
    check(
      "登录上下文变化后 Claude 失效区无百分比并显示已失效（F9 拆分）",
      !/\d+%/.test(card("claude")?.textContent ?? "") &&
        card("claude")?.querySelector(".quota-missing b")?.textContent === dict().quota.discarded &&
        (card("claude")?.querySelector(".quota-missing small")?.textContent ?? "").includes(dict().quota.context.changed) &&
        card("claude")?.querySelector(".card-head small")?.textContent === "—",
    );
    check("上下文变化后 Popover 可见文本无未脱敏邮箱（F9 拆分）", hasOnlyMaskedVisualEmails($(".popover")));
    check(
      "上下文变化后 Popover 无障碍名称仅含口语脱敏邮箱（F9 拆分）",
      hasOnlyAllowedSpokenEmails($(".popover"), dict()),
    );

    quotaVariant("claudeApiKey");
    await wait(80);
    check(
      "claudeApiKey 下 Claude 显示非订阅失效而 Codex 正常（F10 / D3）",
      !/\d+%/.test(card("claude")?.textContent ?? "") &&
        card("claude")?.querySelector(".quota-missing b")?.textContent === dict().quota.discarded &&
        (card("claude")?.querySelector(".quota-missing small")?.textContent ?? "").includes(dict().quota.context.nonSubscription) &&
        card("claude")?.querySelector(".quota-identity-status")?.textContent === dict().quota.identity.nonSubscription &&
        /\d+%/.test(card("codex")?.textContent ?? ""),
    );

    quotaVariant("contextKeyReset");
    await wait(80);
    check(
      "contextKeyReset 下 Claude 显示密钥重置失效文案（F10）",
      !/\d+%/.test(card("claude")?.textContent ?? "") &&
        card("claude")?.querySelector(".quota-missing b")?.textContent === dict().quota.discarded &&
        (card("claude")?.querySelector(".quota-missing small")?.textContent ?? "").includes(dict().quota.context.keyReset),
    );

    quotaVariant("contextRecovered");
    await wait(80);
    check(
      "contextRecovered 下 Claude 首次新鲜 usage 后恢复窗口并保持未确认归属（A5.6）",
      /\d+%/.test(card("claude")?.textContent ?? "") &&
        !!card("claude")?.querySelector(".quota-attribution") &&
        card("claude")?.querySelector(".quota-missing b")?.textContent !== dict().quota.discarded,
    );

    quotaVariant("signedOut");
    await wait(80);
    const signedOutName = card("claude")?.querySelector(".quota-identity")?.getAttribute("aria-label") ?? "";
    check(
      "退出订阅登录的无障碍名称直接说状态",
      signedOutName.startsWith(`${dict().clients.claude} ${dict().quota.identity.signedOut}`),
    );
    check(
      "退出登录后 Claude 失效无百分比且身份区不显示邮箱",
      !/\d+%/.test(card("claude")?.textContent ?? "") &&
        card("claude")?.querySelector(".quota-missing b")?.textContent === dict().quota.discarded &&
        card("claude")?.querySelector(".quota-identity-status")?.textContent === dict().quota.identity.signedOut &&
        !(card("claude")?.querySelector(".quota-identity")?.textContent ?? "").includes("@"),
    );
    check(
      "Codex 非订阅身份不触发额度失效（D3）",
      card("codex")?.querySelector(".quota-identity-status")?.textContent === dict().quota.identity.nonSubscription &&
        /\d+%/.test(card("codex")?.textContent ?? ""),
    );

    quotaVariant("planConflict");
    await wait(80);
    check(
      "Claude 套餐来源冲突显示冲突文案并在 help 列出原值",
      (card("claude")?.querySelector(".quota-identity-plan")?.textContent ?? "").includes(dict().quota.planDisplay.conflict) &&
        (card("claude")?.querySelector(".quota-identity-plan")?.getAttribute("title") ?? "").includes("max_5x"),
    );

    quotaVariant("parseFailed");
    await wait(80);
    check(
      "parseFailed 显示 shape_invalid 第二行小字",
      card("claude")?.querySelector(".quota-missing-detail")?.textContent === dict().quota.parseDetail.shape_invalid,
    );

    quotaVariant("parseFailedMissingLine");
    await wait(80);
    check(
      "parseFailedMissingLine 显示 missing_line 第二行小字（F10）",
      card("claude")?.querySelector(".quota-missing-detail")?.textContent === dict().quota.parseDetail.missing_line,
    );

    quotaVariant("parseFailedTimezone");
    await wait(80);
    check(
      "parseFailedTimezone 显示 timezone_unresolved 第二行小字（F10）",
      card("claude")?.querySelector(".quota-missing-detail")?.textContent === dict().quota.parseDetail.timezone_unresolved,
    );

    quotaVariant("neverProbed");
    await wait(80);
    check("从未探测时不出现身份区域", !$(".quota-identity"));

    quotaVariant("bothOfficial");
    await wait(80);
    const allowance = card("codex")?.querySelector(".quota-allowance");
    check("官方重置次数只显示剩余次数，不显示不可用总数", !allowance?.querySelector("small"));
    hover(allowance);
    await wait(60);
    check("悬停官方重置次数直接弹出明细", !!$(".quota-flyout"));
    check("明细弹层里逐条列出重置次数", ($$(".quota-credits li").length ?? 0) > 0);
    check("重置明细不重复显示不可用总数", !$(".quota-flyout-note"));

    // 触发行 → 通路 → 明细 → 继续阅读 → 离开。这一段是 SQ-MB-R1-F1 的回归。
    // 关键是走真实坐标：先前这里紧接着派发 leave(触发行) 与 hover(明细)，
    // 指针从未经过两者之间那段几何空白，于是断言全绿而真实鼠标停在空白里
    // 明细照样消失。间隙宽度也不是 CSS 里的 10px——它由卡片和面板的内边距
    // 一起决定，所以坐标要从两个真实 rect 现算，不能写死。
    const at = (x, y) =>
      window.dispatchEvent(new MouseEvent("mousemove", { clientX: x, clientY: y, bubbles: true }));
    const triggerBox = allowance.getBoundingClientRect();
    const flyoutBox = $(".quota-flyout").getBoundingClientRect();
    const gapMidX =
      flyoutBox.left >= triggerBox.right
        ? (triggerBox.right + flyoutBox.left) / 2
        : (flyoutBox.right + triggerBox.left) / 2;
    const gapY = (triggerBox.top + triggerBox.bottom) / 2;
    check(
      "触发行与明细之间确实有一段空白要跨",
      Math.abs(flyoutBox.left - triggerBox.right) > 12 || Math.abs(triggerBox.left - flyoutBox.right) > 12,
    );

    leave(allowance);
    at(gapMidX, gapY);
    await wait(500);
    check("指针停在触发行与明细之间的通路上，明细不关闭", !!$(".quota-flyout"));

    at(flyoutBox.left + 20, flyoutBox.top + 20);
    await wait(500);
    check("走完通路进入明细后明细留在原地", !!$(".quota-flyout"));
    check("停在明细上阅读时不会自己关闭", ($$(".quota-credits li").length ?? 0) > 0);

    at(triggerBox.left - 60, triggerBox.top - 120);
    await wait(500);
    check("离开整个交互区域后明细才关闭", !$(".quota-flyout"));

    // 纯键盘路径：只用 focus 与 Escape，不掺任何 hover，否则它证明的是鼠标。
    const allowanceAgain = card("codex")?.querySelector(".quota-allowance");
    allowanceAgain?.focus();
    await wait(80);
    check("键盘聚焦触发行同样打开明细", !!$(".quota-flyout"));
    key(window, "Escape");
    await wait(80);
    check("Escape 关闭明细", !$(".quota-flyout"));

    quotaVariant("readingOff");
    await wait(80);
    check("读取关闭时隐藏额度 tab", !tabButton("quota"));
    check("读取关闭从额度页返回用量页", tabButton("usage")?.classList.contains("active"));
    check("读取关闭不保留额度卡或额度百分比", !$(".quota-client") && !$(".quota-track") && !$(".quota-identity"));
    check("读取关闭仍保留用量数据", !!$(".hero strong") && !!$(".bars"));

    const box = document.createElement("pre");
    box.id = "probe-out";
    box.style.cssText = "position:fixed;inset:0;z-index:9999;margin:0;padding:16px;overflow:auto;background:#000;color:#0f0;font:12px ui-monospace;white-space:pre-wrap";
    const failed = results.filter((line) => line.startsWith("FAIL")).length;
    box.textContent = `${results.join("\n")}\n\n${failed === 0 ? "ALL PASS" : failed + " FAILED"}`;
    document.body.appendChild(box);
    } catch (error) {
      // 探针抛异常时原本什么都不渲染，读者只看到一张普通页面，会当成"没跑"
      // 而不是"跑挂了"。一个静默失败的量具比没有量具更危险。
      const box = document.createElement("pre");
      box.id = "probe-out";
      box.style.cssText = "position:fixed;inset:0;z-index:9999;margin:0;padding:16px;overflow:auto;background:#000;color:#f66;font:12px ui-monospace;white-space:pre-wrap";
      box.textContent = `${results.join("\n")}\n\nPROBE CRASHED after ${results.length} checks\n${error?.stack ?? error}`;
      document.body.appendChild(box);
    }
  }, 700);
}
