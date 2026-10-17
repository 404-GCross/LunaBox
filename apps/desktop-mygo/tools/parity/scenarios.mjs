// 对照场景：原版上用 CDP 执行的步骤（setup 后截图，teardown 恢复），原型对应的 -state 与 -route，
// 以及要比较的区域（DIP，x,y,w,h，窗口 1280x800）。新增页面或状态时在这里加一项。
//
// 选择器尽量用图标类名（i-mdi-*）和 href，不依赖界面语言。

// 等可见的封面加载完（最多 5 秒）；原型的 -screenshot 也会等封面解码完。
const waitImages = ["js", `Promise.race([
    Promise.all([...document.images].filter(i => !i.complete).map(i => new Promise(r => { i.onload = i.onerror = r; }))),
    new Promise(r => setTimeout(r, 5000)),
  ]).then(() => new Promise(r => setTimeout(r, 300)))`];

// 每个场景开始前：回到游戏库、滚到顶部、把指针移到侧边栏空白处（原型截图时没有悬停）。
export const prelude = [
  ["click", "a[href=\"/library\"]"],
  ["js", "document.querySelector('[data-scroll-restoration-id]')?.scrollTo(0, 0)"],
  ["hover", "5,500"],
  waitImages,
];

// 设置页只展开图标为 icon 的分区（其余收起），再把它滚到主内容区顶部下方 24px 处；原型的 -state settings:<key> 做同样的事。
function settingsSection(icon) {
  return ["js", `(async () => {
    const sleep = ms => new Promise(r => setTimeout(r, ms));
    const main = document.querySelector("main");
    // 只取 CollapsibleSection（面板里也有 <section>，它们没有展开按钮）
    const sections = [...main.querySelectorAll("section")].filter(s => s.querySelector(":scope > button[aria-expanded]"));
    for (const s of sections) {
      const btn = s.querySelector(":scope > button");
      const want = ${JSON.stringify(icon)} !== "" && !!btn.querySelector(".i-mdi-${icon}");
      if ((btn.getAttribute("aria-expanded") === "true") !== want)
        btn.click();
    }
    await sleep(400);
    const target = sections.find(s => s.querySelector(":scope > button .i-mdi-${icon}"));
    if (target)
      main.scrollBy(0, target.getBoundingClientRect().top - main.getBoundingClientRect().top - 24);
    await sleep(300);
  })()`];
}

// 设置页各分区：场景名、原型的分区 key、原版的图标。
const settingsSections = [
  ["metadata", "database-search"],
  ["appearance", "palette"],
  ["game", "timer-play-outline"],
  ["proxy", "lan-connect"],
  ["cloud-backup", "cloud-upload"],
  ["auto-backup", "backup-restore"],
  ["ai", "robot-happy"],
  ["db-backup", "database-refresh"],
  ["full-data-backup", "package-variant"],
  ["update", "update"],
  ["app-data", "folder-cog-outline"],
  ["portable-setup", "toolbox-outline"],
];

export const scenarios = [
  {
    name: "library",
    state: "",
    setup: [],
    teardown: [],
    regions: {
      header: [0, 0, 1280, 240], // 顶栏、标题、计数、工具栏
      sidebar: [0, 0, 256, 800],
      cards: [283, 243, 333, 240], // 第一行前两张卡片
    },
  },
  {
    name: "collapsed",
    state: "collapsed",
    setup: [["click", "button:has(> .i-mdi-menu)"], ["hover", "5,500"]],
    teardown: [["click", "button:has(> .i-mdi-menu)"]],
    regions: { full: [0, 0, 1280, 800] },
  },
  {
    name: "drawer",
    state: "drawer",
    setup: [["click", "button:has(.i-mdi-filter-variant)"]],
    teardown: [["press", "Escape"]],
    regions: { drawer: [864, 28, 416, 772] },
  },
  {
    name: "addmenu",
    state: "addmenu",
    setup: [["click", ":has(> .i-mdi-plus)"]],
    teardown: [["press", "Escape"]],
    regions: { menu: [933, 220, 307, 460] },
  },
  {
    name: "batch",
    state: "batch",
    setup: [["click", "button:has(> .i-mdi-checkbox-multiple-marked-outline)"], ["hover", "5,500"]],
    teardown: [["click", "button.glass-btn-neutral:has(> .i-mdi-close-circle-outline)"]],
    regions: { toolbar: [267, 153, 1000, 333] },
  },
  {
    name: "categories",
    route: "/categories",
    setup: [["click", "a[href=\"/categories\"]"], ["hover", "5,500"], waitImages],
    teardown: [],
    regions: {
      header: [256, 28, 1024, 200], // 标题与工具栏
      cards: [256, 228, 1024, 572], // 收藏夹卡片
    },
  },
  {
    // 系统收藏夹（system:favorites）每份数据里都有，卡片名称前是 ❤️。
    name: "category",
    route: "/categories/system:favorites",
    setup: [
      ["click", "a[href=\"/categories\"]"],
      waitImages,
      ["js", "[...document.querySelectorAll('article')].find(a => a.textContent.includes('❤'))?.querySelector('button').click()"],
      ["hover", "5,500"],
      waitImages,
    ],
    teardown: [],
    regions: {
      header: [256, 28, 1024, 260], // 返回、名称与系统标签、计数、工具栏
      cards: [256, 288, 1024, 512], // 游戏卡片
    },
  },
  {
    name: "downloads",
    route: "/downloads",
    setup: [["click", "a[href=\"/downloads\"]"], ["hover", "5,500"]],
    teardown: [],
    regions: {
      header: [256, 28, 1024, 160], // 标题与进行中的任务数
      tasks: [256, 188, 1024, 612], // 按日期分组的任务卡片
    },
  },
  {
    // 进入设置页：只有基础设置展开。
    name: "settings",
    route: "/settings",
    setup: [["click", "a[href=\"/settings\"]"], ["hover", "5,500"]],
    teardown: [],
    regions: {
      header: [256, 28, 1024, 100], // 标题
      basic: [256, 124, 1024, 676], // 基础设置：账号授权卡片与前几项
    },
  },
  {
    name: "settings-collapsed",
    route: "/settings",
    state: "settings:collapsed",
    setup: [["click", "a[href=\"/settings\"]"], ["hover", "5,500"], settingsSection("")],
    teardown: [],
    regions: { sections: [256, 28, 1024, 772] }, // 全部分区的标题栏与页脚
  },
  ...settingsSections.map(([key, icon]) => ({
    name: `settings-${key}`,
    route: "/settings",
    state: `settings:${key}`,
    setup: [["click", "a[href=\"/settings\"]"], ["hover", "5,500"], settingsSection(icon), ["hover", "5,500"]],
    teardown: [],
    regions: { section: [256, 28, 1024, 772] },
  })),
];
