# desktop-mygo

用 [MyGo](https://github.com/egoist/mygo) v0.4.8 原生 UI（D3D11 渲染、DirectWrite 文字、无 WebView）一比一还原 LunaBox 的顶栏、侧边栏和游戏库页面。不参与发布。

## 结构

| 路径 | 内容 |
| --- | --- |
| `main.go` | 命令行参数、窗口创建 |
| `backend.go` | 复制数据库到临时目录后，初始化 `apps/desktop/internal/service` 的真实服务（`GameService`、`TagService`、`GameFilterPresetService`），配置来自 `appconf.LoadConfig()` |
| `ui/` | 界面（`package ui`）：`app.go` 根布局、`topbar.go`、`sidebar.go`、`library.go` 分页与页面、`filterbar.go`、`drawer.go` 筛选抽屉、`card.go` 虚拟网格与卡片、`widgets.go` 滚动条/选择框/开关/菜单/提示、`theme.go` 色板与字号 |
| `scripts/` | 与原版对照、测性能的脚本 |

后端没有复制，也没有用 submodule：`go.mod` 里的 `replace lunabox => ../desktop` 直接引用同一仓库的代码（`lunabox/desktop-mygo/...` 的导入路径以 `lunabox/` 开头，可以使用 `internal` 包）。

DuckDB 需要 cgo，构建时要 `CGO_ENABLED=1`。

## 运行

```sh
cd apps/desktop-mygo
CGO_ENABLED=1 go build -o build/desktop-mygo.exe .
./build/desktop-mygo.exe                    # 读取 %APPDATA%/LunaBox
./build/desktop-mygo.exe -data-dir <目录>   # 便携版数据目录
./build/desktop-mygo.exe -hud               # 性能 HUD（F2 切换，F5 滚动压测）
CGO_ENABLED=1 go test ./...                 # 不开窗口测试视图
```

截图与压测参数：`-screenshot out.png`、`-state drawer|addmenu|batch|collapsed`、`-width/-height`、`-repeat N`、`-bench -bench-exit`（配合 `MYGO_FRAME_STATS=all` 输出各阶段耗时）。

## 怎样做到一比一

尺寸与颜色不是看截图估的，而是从运行中的原版导出计算样式：

1. 把 `apps/desktop` 复制到临时目录，在 `main.go` 的 `application.Options` 里加
   `Windows: application.WindowsOptions{AdditionalBrowserArgs: []string{"--remote-debugging-port=9333"}}` 后编译
   （Wails 会覆盖 `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` 环境变量，只能这样开调试端口）。
2. 用清理过云同步配置的数据副本运行：`LUNABOX_PORTABLE_ROOT=<副本> lunabox-debug.exe`。
3. `node scripts/cdp.mjs --attach 9333 --emulate --click 80,189 --out ref.png --eval scripts/dump-styles.js` 截图并导出每个元素的盒子、字号、颜色。
4. 原型用同一份数据副本 `-width 1295 -height 807 -screenshot` 截图（无边框窗口保留了约 11 物理像素的边框，这样客户区才是 1280×800），逐块做差异对比。

还原时踩到的几处差异：

- **字体**：原版字体栈的 `ui-sans-serif` 在 Chromium 中不生效，`system-ui` 映射到系统消息字体，中文 Windows 上是 Microsoft YaHei UI，英文也用它渲染。`ui/font_windows.go` 用同样的方式取字体。
- **行高**：MyGo 的行高取 `max(行高, 字体升部+降部)`，CSS 允许更小的行高并让字形上下溢出。`text()` 用对称的负外边距压回 CSS 行高（`TestLineBoxHeights` 校验）。
- **细线**：Chromium 把 1px 边框吸附到设备像素（150% 下是 0.667 DIP），`hairline()` 按显示器缩放计算。
- **滚动条**：`::-webkit-scrollbar` 是常显的 9px 槽，MyGo 的是悬停时浮现的覆盖式滚动条，所以自己画（可拖动）。
- **绘制顺序**：绝对定位的元素画在流内元素之上（同 CSS），排序字段覆盖条的文字也要绝对定位。

## 2026-10-14 对照结果（150% 缩放，深色主题，同一份 56 款游戏的数据）

与原版截图逐像素比较，差值大于 24 的像素占比：页头与工具栏 0.9%、侧边栏 0.3%、筛选抽屉 4%、添加游戏菜单 9%（主要是窗口宽度差 1 像素带来的整体偏移和抗锯齿差异）。元素坐标与原版 DOM 一致到 0.1 DIP（见 `TestLayoutDump`、`TestGridGeometryMatchesWeb`）。

性能（自动滚动压测，240Hz 显示器）：

| 场景 | 结果 |
| --- | --- |
| 56 款 | 帧间隔 p50 4.17 ms / p95 7.7 ms，2 帧超过 25 ms |
| 2800 款（`-repeat 50`），21,179 帧 | 帧间隔 p50 4.12 ms / p95 9.1 ms，17 帧超过 25 ms |
| 首帧 | 约 580 ms（启动时先复制数据库、初始化服务） |
| 内存 | working set 约 200 MB（含 DuckDB、封面位图的 CPU 侧副本） |

## 与原版仍不同的地方

- 交互只做了界面：开始游戏、详情页、导入、批量操作、保存预设都只弹提示；Toast 用的是 MyGo 自带样式。
- 标签筛选没有 VNDB 标签翻译；筛选条件不会像原版那样存进 localStorage。
- 原版卡片标题的 `drop-shadow-sm`、GIF 动图封面（只显示第一帧）、NSFW 模糊、背景图毛玻璃模式未还原。
- 首页、统计、收藏、下载、设置是占位页。
