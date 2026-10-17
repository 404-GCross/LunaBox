# 页面对照工作流

desktop-mygo 的目标是和原版（`apps/desktop`，Wails + React）在同样的数据、窗口尺寸和缩放下画出一样的像素。尺寸和颜色不靠看截图估：先从运行中的原版导出每个元素的计算样式，照着实现，再用截图逐像素检查。这份文档说明怎么跑对照、怎么读结果，以及迁移新页面时怎么用这套工具。

## 工具

| 路径 | 作用 |
| --- | --- |
| `tools/parity/prepare-data.mjs` | 从 `%APPDATA%/LunaBox` 复制两份数据（原版、原型各一份），清空云同步、备份和凭据相关的配置 |
| `tools/parity/build-reference.mjs` | 在临时目录构建一份打开了 WebView2 调试端口的原版，仓库里的文件不改 |
| `tools/parity/compare.mjs` | 跑 `scenarios.mjs` 里的全部场景：给两边截图，逐区域比较，输出报告和并排图 |
| `tools/parity/scenarios.mjs` | 场景定义：原版上要执行的操作、原型对应的 `-state`、要比较的区域 |
| `tools/parity/cdp.mjs` | CDP 客户端，`compare.mjs` 用它操作原版，也可以单独在命令行用（点击、悬停、按键、执行脚本、截图） |
| `tools/parity/dump-styles.js` | 在原版页面里执行，导出可见元素的位置、字号、颜色、边框、圆角、内边距 |
| `tools/parity/pixdiff` | Go 工具：比较两张截图的同一区域，输出差异像素占比和并排图 |
| `tools/icons/export-icons.mjs` | 从 `@iconify-json/mdi` 导出 `i-mdi-*` 图标到 `ui/assets/icons` |
| `tools/perf/measure.ps1` | 统计进程树（包括 WebView2 子进程）的内存和空闲 CPU |

需要 Windows、Node 22 以上、Go，并且 `apps/desktop/frontend` 已经 `pnpm install`。

## 跑一次完整对照

```sh
cd apps/desktop-mygo
node tools/parity/prepare-data.mjs      # → %TEMP%/lunabox-parity/{ref,mygo}
node tools/parity/build-reference.mjs   # → %TEMP%/lunabox-reference/lunabox-reference.exe，约 2 分钟
node tools/parity/compare.mjs           # → %TEMP%/lunabox-parity/report/report.md
```

`compare.mjs` 依次做这些事：

1. 构建 `build/desktop-mygo.exe` 和 `build/pixdiff.exe`（加 `--no-build` 跳过）。
2. 用 `ref` 数据启动参考版，等启动窗口关闭后连上调试端口，把视口模拟成 1280×800、缩放 1.5。
3. 对每个场景：回到游戏库并滚到顶部，等封面加载完，执行场景的操作，截图，再把界面恢复原样。
4. 关掉参考版，用 `mygo` 数据运行原型，加上 `-width 1295 -height 807 -state <状态> -screenshot`。原型直接进入对应状态，等封面解码完后截图。
5. 用 `pixdiff` 逐区域比较，写出 `report.md` 和 `sbs-<场景>-<区域>.png`（左边原版，中间原型，右边差异放大 3 倍）。

常用参数：

- `--only drawer,addmenu`：只跑这几个场景。
- `--attach`：参考版已经在运行（比如正在手动调试），就直接用它，不启动也不关闭。
- `--scale`：显示器缩放，必须和原型窗口所在的显示器一致，默认 1.5。

截图期间不要把鼠标放在原型窗口上。原型截的是真实窗口，指针下的卡片会显示悬停边框。

## 读结果

报告里的数字是亮度差超过 24（满分 255）的像素占比。2026-10-15 的基线（categories、category 是 2026-10-17 加的）：

| 场景 / 区域 | 差异 | 来源 |
| --- | --- | --- |
| library / header | 0.88% | 抗锯齿 |
| library / sidebar | 0.27% | 抗锯齿 |
| library / cards | 7.6% | 封面缩放算法不同。原型先把封面缩到 360px，再交给 GPU 缩放，Chromium 用的是另一种重采样，图片区域的差异都来自这里 |
| collapsed / full | 6.5% | 整窗比较，大部分在封面上 |
| drawer / drawer | 4.05% | 背景模糊的实现不同 |
| addmenu / menu | 9.08% | 原型窗口宽 1921 像素，比原版多 1 像素，菜单右对齐后整体右移 1 像素，文字边缘全算成差异 |
| batch / toolbar | 7.7% | 同上，再加封面 |
| categories / header | 5.46% | 工具栏右对齐后右移 1 像素，区域底部带到卡片顶端（见下一行） |
| categories / cards | 12.1% | 卡片顺序不同：“测似乎”的多音字读音与 Chromium 的 ICU 不同（见 README），逐张对比卡片本身一致 |
| category / header | 1.18% | 抗锯齿；❤️ 表情字形不同（Chromium 自带的表情渲染） |
| category / cards | 2.09% | 封面缩放 |

所以判断还原得对不对，要看并排图右边那张差异图：

- 只有细细的边缘轮廓：是偏移 1 像素或者抗锯齿，可以不管。
- 出现整块的亮区：是颜色、尺寸或位置真的不一样。拿 `dump-styles.js` 导出的数值去对代码。

数字适合拿来比较前后两次的变化。一次改动让某个区域的差异明显上升，说明改坏了。

## 迁移新页面或新状态

1. **取尺寸**。启动参考版，进入目标界面，导出计算样式：

   ```sh
   LUNABOX_PORTABLE_ROOT=%TEMP%/lunabox-parity/ref %TEMP%/lunabox-reference/lunabox-reference.exe
   node tools/parity/cdp.mjs --click 'a[href="/stats"]' --eval tools/parity/dump-styles.js --eval-out stats.txt --shot stats.png
   # 只导出某个浮层
   node tools/parity/cdp.mjs --js 'window.__dumpRoot = "[id^=headlessui-menu-items]"' --eval tools/parity/dump-styles.js
   ```

   每行的格式是 `标签.类名 | x,y,w,h | 字体 | 背景/边框/圆角/内边距`，单位都是 DIP。
2. **实现**。页面放在 `ui/pages/<名字>`，在 `ui/routes.go` 里注册路由；能复用的部分放到 `ui/components`。颜色用 `theme` 色板，文字用 `components.Text`，1px 边框用 `Theme.HL`。缺少的图标用 `node tools/icons/export-icons.mjs <名字>` 导出，`--check` 可以列出代码里引用了但还没导出的图标。
3. **对坐标**。`DESKTOP_MYGO_LAYOUT=1 go test ./ui -run TestLayoutDump -v` 会打印关键文字的位置，和导出结果比较，应该相差不超过 0.5 DIP。需要的话可以照着它写一个新页面的版本。
4. **加场景**。在 `scenarios.mjs` 里加一项。原版上的操作尽量用 `i-mdi-*` 类名和 `href` 作为选择器，这样不依赖界面语言。原型上加一个对应的 `-state`（`App.SetDebugState`），或者用 `-route` 指定页面。
5. **跑 `compare.mjs --only <场景>`**，看差异图，修到只剩边缘轮廓为止。

改代码结构但不应该改变外观时，可以拿原型自己做前后对比。用 `-threshold 0` 时，任何不同都会显示出来：

```sh
./build/desktop-mygo.exe -data-dir %TEMP%/lunabox-parity/mygo -width 1295 -height 807 -state drawer -screenshot before.png
# ……改代码、重新构建……
go run ./tools/parity/pixdiff -ref before.png -mine after.png -threshold 0
```

## 已知的坑

- **调试端口只能改代码打开**。Wails 会用 `AdditionalBrowserArgs` 覆盖 `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` 环境变量，所以 `build-reference.mjs` 在临时副本的 `main.go` 里加上这个选项。Wails 的 server 模式（`-tags server`）也用不了，因为 `ApplicationStarted` 不会触发，服务都没有初始化。
- **在 VS Code 里构建前端会丢图标**。UnoCSS 的 presetIcons 检测到 `VSCODE_CWD` 环境变量时，不会从 node 加载 `@iconify-json`，构建产物里的 `i-mdi-*` 全是空的。`build-reference.mjs` 构建时去掉了这个变量。主程序在 VS Code 终端里构建也有这个问题。
- **启动窗口**。原版先打开 `/startup` 窗口，初始化完成后再换成主窗口。`cdp.mjs` 会等启动窗口关闭再连接，否则会连到一个马上就关掉的页面。
- **数据库独占**。原版运行时 DuckDB 独占数据库文件，原型启动时复制数据库会失败，所以要准备两份数据。
- **字体**。原版字体栈里的 `ui-sans-serif` 在 Chromium 里不生效，实际用的是 `system-ui`，也就是系统消息字体（中文 Windows 上是 Microsoft YaHei UI）。对比时两边要在同一台机器上跑。
- **窗口尺寸**。无边框窗口仍然保留大约 11 物理像素的边框，所以原型要用 1295×807 才能得到 1280×800 的客户区（150% 缩放下）。在其他缩放下要相应调整 `--mygo-size`；报告会提示两边截图尺寸不同。
