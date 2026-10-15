# desktop-mygo

用 [MyGo](https://github.com/egoist/mygo) v0.4.8 原生 UI（D3D11 渲染、DirectWrite 文字、无 WebView）一比一还原 LunaBox 的顶栏、侧边栏、游戏库与游戏详情页面。不参与发布。

## 结构

目录对应原版 `apps/desktop/frontend/src`：

| 路径 | 内容 |
| --- | --- |
| `main.go`、`backend.go` | 命令行参数、窗口创建；游戏库列表查询（`ui.Backend`，`-repeat` 压测时放大） |
| `backend/` | 原型自己的数据目录与种子数据、数据库迁移、`apps/desktop/internal/service` 真实服务的构造与注入（照搬 `apps/desktop/main.go`），以及用 MyGo 实现的 `wailsruntime.Runtime`（原生文件对话框、事件、打开网址） |
| `ui/app.go`、`ui/routes.go` | 根布局（`routes/__root.tsx`）与路由表，用 MyGo 的 `ui.Router` |
| `ui/pages/library` | 游戏库页面（`routes/library.tsx`，以及它专用的 FilterBar、TagFilterMenu、GameFilterPresetMenu） |
| `ui/pages/game` | 游戏详情页（`routes/game.tsx`，路由 `/game/{id}`）：页头、“游戏统计”标签页（`components/panel/GameStatsPanel.tsx`）、添加到收藏、说明文档、封面大图等页头弹窗 |
| `ui/components/panel/*` | 详情页其余标签页（`components/panel/Game*Panel.tsx`），每个一个子包：`edit`、`launch`（含启动游戏与运行状态）、`backup`、`progress`、`review`；`panel.Game` 是它们共用的输入 |
| `ui/components` | `components/ui`：Better 系列（Button、SplitButton、Select、Switch、Tooltip、DropdownMenu、Drawer、Input、ActionInput、ImageViewer）、Textarea、Modal / ConfirmModal、SlideButton、GameTags、EmojiPicker、NSFW 封面模糊、游戏状态配色、滚动区域、回到顶部等 |
| `ui/components/picker` | BetterDateRangePicker、BetterWheelPicker |
| `ui/components/bar`、`card`、`grid`、`chart` | `components/bar`（TopBar、SideBar）、`components/card`（GameCard）、`components/grid`（VirtualGameGrid）、`components/chart`（原版用到的全部图表，用 MyGo 的 `ui/chart`） |
| `ui/theme`、`ui/assets` | `uno.config.ts` 的色板、字号、过渡曲线、像素吸附；图标、元数据来源图标、表情数据与图片 |
| `ui/env`、`ui/covers`、`ui/perf`、`ui/timefmt` | 页面共享的运行环境（后端、路由、主题）、封面缓存、性能 HUD 与压测、`utils/time.ts` 的时长与日期格式 |
| `tools/parity` | 与原版逐像素对照的工具，见 [docs/parity.md](docs/parity.md) |
| `tools/icons`、`tools/perf`、`tools/emoji` | 导出 MDI 图标；测进程内存与 CPU；下载表情选择器的数据（原版运行时从 CDN 下载） |

依赖方向：`pages` → `components/*` → `components` → `theme`、`assets`。组件不读页面状态，通过参数与回调交互；组件自己的临时状态（弹层开关、悬停计时）用 `ui.Local` 挂在自己的元素上。

后端没有复制，也没有用 submodule：`go.mod` 里的 `replace lunabox => ../desktop` 直接引用同一仓库的代码（`lunabox/desktop-mygo/...` 的导入路径以 `lunabox/` 开头，可以使用 `internal` 包）。

DuckDB 需要 cgo，构建时要 `CGO_ENABLED=1`。

### 数据

原型有自己的数据目录，默认是 `build/data`（可执行文件在 `build/` 下时用它旁边的 `data/`）。第一次运行时从安装版的 `%APPDATA%/LunaBox` **复制**（不是移动）设置、数据库、封面、背景、头像、模板与备份，之后所有读写都在这份拷贝上进行，安装版的数据不受影响；`-reseed` 重新复制一次覆盖原型的数据。服务的日志在 `build/data/logs/app.log`。

页面直接调用 `apps/desktop/internal/service` 的真实服务（`env.Env.Svc`），与原版前端调用 Wails 绑定一样：编辑、状态、标签、收藏、评价、进度都会写进这份数据库。备份页的“恢复”会覆盖游戏真实的存档目录，“上传到云端”使用设置里的云备份账号——与原版行为相同，操作前请确认。原型不启动云同步、定时数据库备份、MCP 与更新检查。

测试（`envtest.NewServices`）在临时目录里新建数据库并跑完全部迁移，用的也是真实服务。

## 运行

```sh
cd apps/desktop-mygo
CGO_ENABLED=1 go build -o build/desktop-mygo.exe .
./build/desktop-mygo.exe                    # 数据在 build/data（首次运行从 %APPDATA%/LunaBox 复制）
./build/desktop-mygo.exe -reseed            # 重新从 %APPDATA%/LunaBox 复制
./build/desktop-mygo.exe -data-dir <目录>   # 使用别的数据目录
./build/desktop-mygo.exe -hud               # 性能 HUD（F2 切换，F5 滚动压测）
CGO_ENABLED=1 go test ./...                 # 不开窗口测试视图
```

截图与压测参数：`-screenshot out.png`、`-state drawer|addmenu|batch|collapsed|tab:<标签页>`、`-route /library`（详情页是 `-route /game/<游戏 ID>`；在 Git Bash 里要加 `MSYS_NO_PATHCONV=1`，否则路径会被改写）、`-width/-height`、`-repeat N`、`-bench -bench-exit`（配合 `MYGO_FRAME_STATS=all` 输出各阶段耗时）。侧边栏切换页面走 `ui.Router`，Alt+←/→ 与鼠标侧键前进后退。

## 怎样做到一比一

尺寸与颜色不是看截图估的，而是从运行中的原版导出计算样式，再用截图逐像素检查。完整流程、结果解读和已知的坑见 [docs/parity.md](docs/parity.md)，三条命令跑完全部对照：

```sh
node tools/parity/prepare-data.mjs && node tools/parity/build-reference.mjs && node tools/parity/compare.mjs
```

还原时踩到的几处差异：

- **字体**：原版字体栈的 `ui-sans-serif` 在 Chromium 中不生效，`system-ui` 映射到系统消息字体，中文 Windows 上是 Microsoft YaHei UI，英文也用它渲染。`ui/theme/font_windows.go` 用同样的方式取字体。
- **行高**：MyGo 的行高取 `max(行高, 字体升部+降部)`，CSS 允许更小的行高并让字形上下溢出。`components.Text` 用对称的负外边距压回 CSS 行高（`TestLineBoxHeights` 校验）。
- **细线**：Chromium 把 1px 边框吸附到设备像素（150% 下是 0.667 DIP），用 `Theme.HL`。
- **滚动条**：`::-webkit-scrollbar` 是常显的 9px 槽，MyGo 的是悬停时浮现的覆盖式滚动条，所以 `components.ScrollArea` 自己画（可拖动）。
- **绘制顺序**：绝对定位的元素画在流内元素之上（同 CSS），排序字段覆盖条的文字也要绝对定位。

## 对照结果（150% 缩放，深色主题，同一份 56 款游戏的数据）

与原版截图逐像素比较，差值大于 24 的像素占比：页头与工具栏 0.9%、侧边栏 0.3%、筛选抽屉 4%、添加游戏菜单 9%（主要是窗口宽度差 1 像素带来的整体偏移和抗锯齿差异），各区域明细见 [docs/parity.md](docs/parity.md#读结果)。元素坐标与原版 DOM 一致到 0.1 DIP（见 `TestLayoutDump`、`grid.TestGeometryMatchesWeb`）。

性能（自动滚动压测，240Hz 显示器）：

| 场景 | 结果 |
| --- | --- |
| 56 款 | 帧间隔 p50 4.17 ms / p95 7.7 ms，2 帧超过 25 ms |
| 2800 款（`-repeat 50`），21,179 帧 | 帧间隔 p50 4.12 ms / p95 9.1 ms，17 帧超过 25 ms |
| 首帧 | 约 580 ms（启动时先复制数据库、初始化服务） |
| 内存 | working set 约 200 MB（含 DuckDB、封面位图的 CPU 侧副本） |

## 与原版仍不同的地方

游戏库：

- 开始游戏（在卡片上）、导入、批量操作、保存预设只弹提示。
- 批量“设为状态”菜单、空状态导入按钮与浅色主题没有和原版截图对照过（`scenarios.mjs` 里还没有这些场景）。
- 标签筛选没有 VNDB 标签翻译；筛选条件不会像原版那样存进 localStorage。
- 原版卡片标题的 `drop-shadow-sm`、GIF 动图封面（只显示第一帧）、背景图毛玻璃模式未还原。

游戏详情页全部迁移，接的是真实服务。没有逐像素对照整页，图表（`components/chart`）与日期、滚轮选择器（`components/picker`）和运行中的原版截图对照过。已知差异：

- 通用：Toast 用 MyGo 自带样式，不区分成功与失败；输入框的占位文字颜色跟随窗口主题（MyGo 不支持逐个设置）；禁用元素的不透明度最高 0.5（MyGo 会把禁用元素再减半，`components.Disable` 补偿，`opacity-60` 做不到）；没有 `cursor-zoom-in` 光标。
- 页头：表情选择器是按 emoji-picker-element 的样式重写的，数据内嵌（`tools/emoji` 下载，原版运行时从 CDN 下载），不记录常用表情、界面文字与原版一样是英文。
- 编辑：评分没有数字输入框的上下箭头；别名输入框里选完输入法候选后要再按一次 Enter；从剪贴板粘贴封面只在真实窗口里可用；数据源 ID 在抽屉打开时离开页面，未保存的修改会丢失。
- 启动配置：只有 Windows 的行为（没有 macOS 兼容层与 Linux 的 Proton 部分）；不从首页数据同步正在运行的游戏。
- 评价：文本框固定 240px 高，可滚动（MyGo 没有 `resize-y` 拖动柄）；星级评分不随键盘焦点预览。
- 图表：没有 Chart.js 的首次出现与数据变化动画；旋转的标签比 Chrome 略粗糙。
- 原版里本来就不显示的图标（`content-save-clock-outline`、`application-search-outline`、`database-star-outline` 在 MDI 里不存在）同样留空。

首页、统计、收藏、下载、设置是占位页（统计页要用的图表与日期选择器已经在 `components` 里）。

原版后端的问题（未修改）：`BackupService.RestoreBackup` 在存档目录里只有一个文件时会把它当作单文件存档，恢复到目录自身的路径上。
