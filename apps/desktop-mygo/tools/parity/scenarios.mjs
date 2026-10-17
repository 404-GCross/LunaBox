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
];
