# desktop-mygo

LunaBox 的原生桌面前端：用 [MyGo](https://github.com/egoist/mygo) v0.6.0 的原生 UI（Windows 上是 D3D11 渲染、DirectWrite 文字，没有 WebView）重写 `apps/desktop/frontend`，后端与 Wails 版共用 `packages/backend`。它和 Wails 版并行开发，目标是之后**替代 Wails + WebView 前端**，所以按正式产品的标准来做：界面与原版一比一、文字走同一套多语言、行为由真实服务驱动并有测试覆盖。目前还不参与发布，进度见[路线图](#路线图)。

## 原则

- **一比一还原**：尺寸、颜色从运行中的原版导出计算样式，再逐像素对照（[docs/parity.md](docs/parity.md)）。与原版不同的地方都记在[与原版仍不同的地方](#与原版仍不同的地方)。
- **同一份后端**：服务、数据库迁移、启动与退出流程都在 `packages/backend`（`go.mod` 里 `replace lunabox/backend => ../../packages/backend`），服务的构造、后台任务、关闭到托盘、退出前同步、协议请求与关闭顺序在 `lunabox/backend/bootstrap`，与 Wails 版是同一份代码。这里只写 MyGo 相关的部分，不复制后端代码。页面通过 `api` 包的接口调用服务（`env.Env.Svc`），与原版前端调用 Wails 绑定一样。
- **同一份翻译**：界面文字来自原版的语言包 `apps/desktop/frontend/src/locales`，由 `tools/i18n` 转换成 MyGo 的目录（[docs/i18n.md](docs/i18n.md)）。界面代码里不写死文字，`TestNoHardcodedText` 会检查。
- **测试不开窗口**：`ui.Tester` 构建视图、模拟输入、推进时钟（`tt.Advance`），测试在临时目录里新建数据库并跑完全部迁移（`hosttest.NewServices`）。

## 结构

目录对应原版 `apps/desktop/frontend/src`：

| 路径 | 内容 |
| --- | --- |
| `main.go` | 命令行参数与语言 |
| `app/` | MyGo 的应用外壳（对应 `apps/desktop/main.go`）：单实例、数据目录与日志、主窗口（隐藏标题栏与系统窗口按钮，由顶栏绘制；最大化按钮在 Windows 11 上显示贴靠布局）、托盘、启动错误窗，把窗口与框架事件交给 `bootstrap.App`；游戏库列表查询（`ui.Backend`，`-repeat` 压测时放大） |
| `host/` | 把后端接到 MyGo：`appruntime.Runtime`（原生文件对话框、事件、打开网址、开机自启）与 `bootstrap.Window` 的实现、把服务事件转到界面线程的 `Events`、给界面的 `api.Services`（`Backend.API`）、开发运行的数据目录；`host/hosttest` 给测试提供真实服务 |
| `api/` | 界面调用的后端接口（对应原版前端的 Wails 绑定），界面只依赖这些接口 |
| `ui/app.go`、`ui/routes.go` | 根布局（`routes/__root.tsx`）与路由表，用 MyGo 的 `ui.Router` |
| `ui/pages/home` | 首页（`routes/index.tsx`，默认路由 `/`）：最近游玩的游戏轮播（模糊封面背景、封面大图、上次游玩时间与总时长）、继续游戏，随 `home:refresh-requested`、`game-runtime:changed`、`app:main-window-shown` 刷新 |
| `ui/pages/library` | 游戏库页面（`routes/library.tsx`） |
| `ui/pages/categories`、`category` | 收藏夹列表（`routes/categories.tsx`）与收藏详情（`routes/category.tsx`，路由 `/categories/{id}`，含 AddGameToCategoryModal） |
| `ui/pages/game` | 游戏详情页（`routes/game.tsx`，路由 `/game/{id}`）：页头、“游戏统计”标签页（`components/panel/GameStatsPanel.tsx`）、添加到收藏、说明文档、封面大图等页头弹窗 |
| `ui/pages/downloads` | 下载管理（`routes/downloads.tsx`）：按日期分组的任务，暂停、继续、重试、取消、删除记录、复制下载地址、打开所在文件夹与导入为游戏，随 `download:progress`、`download:game-imported` 事件更新 |
| `ui/pages/stats` | 统计（`routes/stats.tsx`）：时间维度与自定义日期范围（`StatsToolbar`）、AI 总结（`AiSummaryCard`，按维度缓存、离开页面后保留）、汇总卡片、游玩时长排行榜与 `StatsLeaderboardModal`、标签分布、游玩时段分布、总时长与常玩游戏趋势 |
| `ui/pages/startup` | 启动错误窗（`StartupWindow.tsx`） |
| `ui/pages/settings` | 设置页（`routes/settings.tsx`）：各分区的 CollapsibleSection、草稿与 250ms 后自动保存（store 的 `setDraftConfig`、`saveDraftConfig`、`patchLiveConfig`）、版本信息 |
| `ui/components/panel/*` | 详情页其余标签页（`components/panel/Game*Panel.tsx`），每个一个子包：`edit`、`launch`（含启动游戏与运行状态）、`backup`、`progress`、`review`；`panel.Game` 是它们共用的输入。设置页的各分区（`components/panel/*SettingsPanel.tsx` 等）同样每个一个子包：`basic`、`account`（账号授权卡片）、`metadata`、`background`、`gamesettings`、`proxy`、`cloudbackup`、`autobackup`、`ai`、`dbbackup`、`fullbackup`、`update`、`appdata`、`portable`；`panel.Settings` 是它们共用的输入。首页底部的封面选择面板（`HomeGameRailPanel.tsx`）是 `homerail` |
| `ui/components` | `components/ui`：Better 系列（`better` 子包：Button、SplitButton、Select、Switch、Tooltip、DropdownMenu、Drawer、Input、ActionInput、ImageViewer、Textarea、EdgeIconButton）、Modal / ConfirmModal / CategoryModal、SlideButton、GameTags、EmojiPicker、NSFW 封面模糊、游戏状态配色、滚动区域、回到顶部等 |
| `ui/components/picker` | BetterDateRangePicker、BetterWheelPicker |
| `ui/components/bar`、`card`、`grid`、`chart` | `components/bar`（TopBar、SideBar、PlayingIsland、FilterBar 及其中的 TagFilterMenu、GameFilterPresetMenu）、`components/card`（GameCard、CategoryCard、DownloadCard）、`components/grid`（VirtualGameGrid 与分页加载）、`components/chart`（原版用到的全部图表，用 MyGo 的 `ui/chart`） |
| `ui/locales` | 多语言目录（由原版语言包生成）与 `mygo generate` 写出的类型化函数，见 [docs/i18n.md](docs/i18n.md) |
| `ui/theme`、`ui/assets` | `uno.config.ts` 的色板、字号、过渡曲线、像素吸附；图标、元数据来源图标、表情数据与图片 |
| `ui/env`、`ui/covers`、`ui/perf`、`ui/timefmt` | 页面共享的运行环境（后端、路由、主题、语言）、封面缓存、性能 HUD 与压测、`utils/time.ts` 的时长与日期格式（按语言） |
| `ui/tagname` | `utils/tagTranslation.ts`：VNDB 标签按界面语言显示译名、按译名搜索标签（详情页标签、筛选抽屉、统计页标签分布），数据是原版的 `vndbTagTranslations.*.json` |
| `tools/i18n` | 把原版语言包转换成 `ui/locales`，把 VNDB 标签译名复制到 `ui/tagname/data`；`extra/` 是原版没有的补充文字 |
| `tools/parity` | 与原版逐像素对照的工具，见 [docs/parity.md](docs/parity.md) |
| `tools/icons`、`tools/perf`、`tools/emoji` | 导出 MDI 图标；测进程内存与 CPU；下载表情选择器的数据（原版运行时从 CDN 下载） |

依赖方向：`pages` → `components/*` → `components` → `theme`、`assets`、`locales`。组件不读页面状态，通过参数与回调交互；组件自己的临时状态（弹层开关、悬停计时）用 `ui.Local` 挂在自己的元素上。

DuckDB 需要 cgo，构建时要 `CGO_ENABLED=1`。

### 数据

`mygo build` 构建的正式版与 Wails 版使用同一个数据目录（由 `version.BuildMode` 决定：便携版是程序所在目录，安装版是 `%APPDATA%/LunaBox`），启动时同样运行云同步、定时数据库备份、MCP、CLI 的 IPC 服务与开机自启。

开发运行（`go run`、`go build` 或 `mygo dev`，即 `mygo.IsDev()`）有自己的数据目录 `build/data`（可执行文件在 `build/` 下时用它旁边的 `data/`）。第一次运行时从安装版的 `%APPDATA%/LunaBox` **复制**（不是移动）设置、数据库、封面、背景、头像、模板与备份，之后所有读写都在这份拷贝上进行，安装版的数据不受影响；`-reseed` 重新复制一次覆盖这份数据，`-data-dir` 改用别的目录。开发运行与安装版隔离（`bootstrap.Options.Isolated`）：单实例锁的名字是“LunaBox Dev”，不注册开机自启，不启动 CLI 的 IPC 服务与 MCP 服务，不自动云同步——IPC 的端口文件与开机启动项都是全局的，会影响同时运行的安装版。日志在数据目录的 `logs/`（`app-<日期>.log`）。开发运行还会读取项目目录下的 `.env.build` 与 `.env`（格式见 `.env.build.example`），补上没有用 ldflags 注入的 OAuth Client ID 等构建期凭据，与 `wails3 dev` 相同。

编辑、状态、标签、收藏、评价、进度都会写进这份数据库。备份页的“恢复”会覆盖游戏真实的存档目录，“上传到云端”使用设置里的云备份账号——与原版行为相同，操作前请确认。设置页安排的数据库、全量数据恢复在退出（`SafeQuit`）后的下一次启动时执行。

### 多语言

与原版相同的四种语言：简体中文（源语言、回退语言）、繁体中文、英语、日语，显示配置里 `language` 的语言（原版 `App.tsx` 的 `i18n.changeLanguage(config.language)`）。改了原版的语言包后运行 `go run ./tools/i18n` 重新生成，`TestCatalogsMatchFrontend` 会在两边不一致时失败。约定、日期格式与 MyGo 自带文字的翻译见 [docs/i18n.md](docs/i18n.md)。

## 运行

日常开发用 MyGo 的 CLI（仓库根目录 `pnpm mygo:dev`）。MyGo 的 CLI 在 `CGO_ENABLED` 未设置时关闭 cgo，而 DuckDB 需要它，所以通过 `tools/mygo` 运行（它设好 `CGO_ENABLED=1`，参数原样交给 `go tool mygo`；在 bash 里也可以直接写 `CGO_ENABLED=1 go tool mygo dev`）：

```sh
cd apps/desktop-mygo
go run ./tools/mygo dev                     # 构建开发版（exe 带图标、manifest 与版本信息）并启动，Go 代码、配置或图标改了自动重新构建、重启
go run ./tools/mygo dev -lang en-US         # 这次运行显示英语，不写进配置
```

`mygo dev` 监听应用依赖的全部本地包（包括 `packages/backend`）；它在 Windows 上重启或退出（Ctrl+C、`q`）时直接结束进程，不走退出流程（不检查点数据库、不保存窗口尺寸），下次启动由 WAL 恢复。从界面或托盘退出则正常关闭。`mygo dev` 不能给应用传命令行参数，下面这些参数要用 `go build` 构建后运行（这样构建的 exe 没有图标资源，启动后由 `Dock.SetIcon` 设置窗口与任务栏图标）：

```sh
cd apps/desktop-mygo
CGO_ENABLED=1 go build -o build/desktop-mygo.exe .   # 即 pnpm mygo:build
./build/desktop-mygo.exe                    # 数据在 build/data（首次运行从 %APPDATA%/LunaBox 复制）
./build/desktop-mygo.exe -reseed            # 重新从 %APPDATA%/LunaBox 复制
./build/desktop-mygo.exe -data-dir <目录>   # 使用别的数据目录
./build/desktop-mygo.exe -lang en-US        # 这次运行显示英语（zh-CN、zh-TW、en-US、ja-JP），不写进配置
./build/desktop-mygo.exe -lang en-XA        # 伪语言：检查漏翻的文字与被截断的布局（ar-XB 检查从右到左）
./build/desktop-mygo.exe -hud               # 性能 HUD（F2 切换，F5 滚动压测）
CGO_ENABLED=1 go test ./...                 # 不开窗口测试视图
go run ./tools/i18n                         # 原版语言包改了以后重新生成 ui/locales
```

便携版（对应 Wails 版的 `scripts\build.bat portable`）用 `tools/release` 构建（即 `pnpm mygo:portable`）：`mygo build` 编译，注入版本号（默认取最近的 `v*` 标签）与 `.env.build` 里的构建期凭据，和 lunacli、7z 一起打成 `build/release/LunaBox-mygo-<版本>-windows-<架构>-portable.zip`，数据在 `LunaBox.exe` 旁边。`mygo build` 的 `-ldflags` 是固定的，所以构建期间临时写一个 `release_gen.go` 在 `init` 里给 `version` 包赋值。

```sh
go run ./tools/release                          # amd64，版本取最近的 v* 标签
go run ./tools/release -version 1.2.0 -arch arm64
```

截图与压测参数：`-screenshot out.png`、`-state drawer|addmenu|batch|collapsed|picker|tab:<标签页>|settings:<分区>`（`picker` 展开首页的封面选择面板；设置页只展开这个分区并滚到顶部，`settings:collapsed` 全部收起；配合 `-route /settings`）、`-route /library`（默认是首页 `/`；详情页是 `-route /game/<游戏 ID>`；在 Git Bash 里要加 `MSYS_NO_PATHCONV=1`，否则路径会被改写）、`-width/-height`、`-repeat N`、`-bench -bench-exit`（配合 `MYGO_FRAME_STATS=all` 输出各阶段耗时）。侧边栏切换页面走 `ui.Router`，Alt+←/→ 与鼠标侧键前进后退。

## 开发约定

- 迁移一个原版组件时，在 Go 代码的注释里写它对应的原版文件与类名（`对应 GameCard.tsx：rounded-xl border …`），尺寸颜色按 [docs/parity.md](docs/parity.md) 取值。
- 文字用 `ui/locales` 的函数，key 与原版组件在同一处用的 key 相同；原版没有的加到 `tools/i18n/extra` 的 `native` 下，四种语言都写（[docs/i18n.md](docs/i18n.md)）。
- 时间相关的状态用帧时钟：构建时用 `c.Now()`，输入事件用 `ev.Time`，异步回调里记一个标记、到下一次构建再取 `c.Now()`。测试里用 `tt.Advance(d)` 推进时钟，不要 `time.Sleep`。
- 改完跑 `CGO_ENABLED=1 go vet ./... && CGO_ENABLED=1 go test ./...`。

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

与原版截图逐像素比较，差值大于 24 的像素占比：页头与工具栏 0.9%、侧边栏 0.3%、筛选抽屉 4%、添加游戏菜单 9%（主要是窗口宽度差 1 像素带来的整体偏移和抗锯齿差异），各区域明细见 [docs/parity.md](docs/parity.md#读结果)。元素坐标与原版 DOM 一致到 0.1 DIP（见 `TestLayoutDump`、`grid.TestGeometryMatchesWeb`）。以上是简体中文界面的结果，其他语言还没有对照过。

性能（自动滚动压测，240Hz 显示器）：

| 场景 | 结果 |
| --- | --- |
| 56 款 | 帧间隔 p50 4.17 ms / p95 7.7 ms，2 帧超过 25 ms |
| 2800 款（`-repeat 50`），21,179 帧 | 帧间隔 p50 4.12 ms / p95 9.1 ms，17 帧超过 25 ms |
| 首帧 | 约 580 ms（启动时先复制数据库、初始化服务） |
| 内存 | working set 约 200 MB（含 DuckDB、封面位图的 CPU 侧副本） |

## 路线图

已完成：顶栏、侧边栏、游戏运行时的灵动岛、首页、游戏库、游戏详情页（全部标签页与弹窗）、收藏夹列表与收藏详情、设置页（全部分区与弹窗）、下载管理、统计、四种语言。替代 Wails 版之前还要做：

1. **统计页的美化导出**：原版的 TemplateExportModal 用 WebView 渲染海报模板，暂不迁移（不想为 Linux 引入 WebKit：`mygo.json` 的 `"linux": {"webview": false}` 让 Linux 安装包不依赖 WebKitGTK）。
2. **后台服务**：启动时的更新检查。云同步、定时数据库备份、MCP、IPC、开机自启、托盘与退出前同步已由 `bootstrap` 提供。
3. **与 Wails 版并存**：两个版本的单实例锁不同，正式版可能同时打开同一份数据（DuckDB 会拒绝第二个进程，后启动的那个显示启动错误窗）。
4. **跨平台**：在 macOS（Metal）与 Linux 上跑通并对照（启动配置的 macOS 兼容层、Linux 的 Wine / Proton 已迁移，只在 Windows 上用替换的服务测过）。
5. **打包发布**：`mygo build` 的安装包、签名与自动更新，替换 Wails 版的发布流程。
6. **其他语言的对照**：英语、日语界面的截图对照，检查较长的文字有没有被截断。

## 与原版仍不同的地方

首页全部迁移，接的是真实服务（首页数据、全局统计、启动游戏与运行状态），首页数据里正在游玩的游戏同步为运行状态（`setGameRuntimeFromHome`，详情页共用同一份）。还没有和原版截图对照过（`scenarios.mjs` 里没有首页场景）。已知差异：

- 没有触控、触控笔的上下滑动展开/收起封面选择面板（`useVerticalSwipe`）。
- 标题与说明文字的 `drop-shadow` 没有还原；选中封面的旋转描边没有 `blur-[1px]`。
- 封面主色（`useImageAccentRgb`）按 fast-average-color 的 sqrt 算法在解码后的封面上计算，缩小时的重采样与浏览器不同，颜色可能略有出入。
- 封面大图最大缓存 1024×576 像素（200% 缩放下的最大显示尺寸），原图更大时按缓存的尺寸限制“不放大”。
- 原版悬停在展开的面板上时暂停轮播，但面板展开时轮播本来就停了，所以这里不跟踪悬停。

灵动岛（`PlayingIsland.tsx`）全部迁移：显示 `activeGameRuntimeId` 的游戏（启动中、已游玩时长或活跃时长、结束中），结束当前会话（`EndCurrentPlaySession`），多个游戏时点封面切换，开始游玩 5 秒后自动收起，上下拖动或点击收起的岛切换展开与收起，入场、退场与切换动画。运行状态由 `launch.Launcher` 持有（store 的 `gameRuntimes`），首页、详情页、卡片与灵动岛共用。没有和原版截图对照过。已知差异：

- 多个游戏的切换顺序按启动时间（原版按 `gameRuntimes` 的插入顺序）。
- 封面悬停的 `brightness-75` 用 25% 的黑色遮罩近似；标题跑马灯的宽度按文字测量，不跟随 ResizeObserver。

游戏库：

- 导入、批量操作、保存预设只弹提示。卡片上的开始游戏接的是真实启动（store.startGame），游戏运行时按钮的 `disabled:opacity-65` 最多 0.5（同通用差异）。
- 批量“设为状态”菜单、空状态导入按钮与浅色主题没有和原版截图对照过（`scenarios.mjs` 里还没有这些场景）。
- 筛选条件不会像原版那样存进 localStorage（在一次运行内跨页面保留）。
- 原版卡片标题的 `drop-shadow-sm`、GIF 动图封面（只显示第一帧）、背景图毛玻璃模式未还原。

收藏夹列表与收藏详情全部迁移，接的是真实服务，和原版截图对照过（`scenarios.mjs` 的 categories、category）。已知差异：

- 没有拖动框选（`useDragSelection`），批量模式下逐个点选或全选；游戏库同样没有。
- 名称排序用 `golang.org/x/text/collate`（中文按拼音），多音字的读音可能与 Chromium 自带的 ICU 不同：如“测似乎”与“测试”，原版排在“测试”之后，这里排在之前。
- 收藏卡片悬停时展开的封面不会浮到相邻卡片之上（原版 `hover:z-30`），展开后最多盖住 2 像素的间距。
- 卡片右上角“从收藏夹中移除”按钮在原版没有无障碍名称，这里用 `native.removeFromCategory`。
- 批量删除收藏夹、重命名、添加游戏弹窗、空收藏夹与浅色主题没有和原版截图对照过。

游戏详情页全部迁移，接的是真实服务。没有逐像素对照整页，图表（`components/chart`）与日期、滚轮选择器（`components/picker`）和运行中的原版截图对照过。已知差异：

- 通用：Toast 按原版 AppToaster 绘制（`components.Toaster`），但最多同时显示 3 条（MyGo 的上限，原版 4 条）；输入框的占位文字颜色跟随窗口主题（MyGo 不支持逐个设置）；禁用元素的不透明度最高 0.5（MyGo 会把禁用元素再减半，`components.Disable` 补偿，`opacity-60` 做不到）；没有 `cursor-zoom-in` 光标。
- 页头：表情选择器（`components/emoji_picker.go`）没有照搬 emoji-picker-element 的布局，改成九列网格：分组导航与表情列对齐、面板按内容收紧、“移除”是带图标的按钮（没有图标时禁用）；数据内嵌（`tools/emoji` 下载，原版运行时从 CDN 下载），不记录常用表情；选择器内部的文字（搜索、分组名）只有英文，原版按语言显示中文、日文。
- 编辑：评分没有数字输入框的上下箭头；别名输入框里选完输入法候选后要再按一次 Enter；从剪贴板粘贴封面只在真实窗口里可用；数据源 ID 在抽屉打开时离开页面，未保存的修改会丢失。
- 启动配置：按平台（`GetGOOS`）显示，macOS、Linux 的部分没有在真实系统上运行过；原版 `GetLocalProtonTools` 失败时的提示没有（服务方法不返回错误）。
- 评价：文本框固定 240px 高，可滚动（MyGo 没有 `resize-y` 拖动柄）；星级评分不随键盘焦点预览。
- 图表：没有 Chart.js 的首次出现与数据变化动画；旋转的标签比 Chrome 略粗糙。
- 原版里本来就不显示的图标（`content-save-clock-outline`、`application-search-outline`、`database-star-outline` 在 MDI 里不存在）同样留空。

设置页全部迁移（13 个分区、账号授权卡片、元数据批量刷新与 ID 补全、背景裁剪、更新弹窗等），接的是真实服务。每个分区都和原版截图对照过（`scenarios.mjs` 的 settings、settings-collapsed、settings-*，深色主题、简体中文）。已知差异：

- 保存：离开页面时补存还没保存的草稿（原版丢掉）；保存失败时提示并把草稿恢复成生效的配置（原版只打日志，草稿留着，之后每次保存都会失败）。配置在主线程同步读取，没有骨架屏。
- 还做不到的：界面缩放只保存不生效（原生界面没有页面缩放）；开发运行时开机自启只记日志、不改系统设置；自定义背景（图片、模糊、遮罩、隐藏封面）只保存，界面不显示背景图；“立即更新”只弹提示（不下载、不替换安装版），也不在启动时检查更新。
- 本地集成设置：没有用 ldflags 指定时 `version.BuildMode` 是 portable，所以这一节会显示；点“注册协议”“添加到 PATH”会让 `lunabox://` 与 PATH 指向 MyGo 版的可执行文件（替换安装版的关联），它旁边没有 lunacli。
- 账号授权：Bangumi 与鲲 Galgame 没有内置的 OAuth Client ID，没有在构建时注入时授权直接失败（同未配置的原版）；Hikarinagi 会打开浏览器。离开页面不会取消进行中的 Umbra 授权。
- 面板的状态在页面的整个生命周期里保留（原版每次进入设置页重新挂载）：更新检查结果、元数据刷新进度会保留；数据库备份、本地集成、账号卡片在分区重新展开时重新加载。分区收起时不构建面板，数据库备份与全量备份的提示和“1.5 秒后退出”在分区再次展开时才继续。
- 控件：分区展开的缓动用 MyGo Collapsible 自带的（原版 ease-out）；滑块是自己画的，禁用时拇指的颜色没有对照；MCP 端口输入框没有上下箭头；数据表格的表头在滚动区外，滚动条只在表体旁边；裁剪框的控制点不能用 Tab 聚焦；游戏库路径变更弹窗只有内容区能滚动，比窗口还高时会被截断。原版裁剪弹窗的一个问题照样保留：换了比例后不拖动直接确认，用的还是上一次的裁剪区域。
- 浅色主题、其他语言与各个弹窗没有和原版截图对照过。

下载管理全部迁移，接的是真实服务，和原版截图对照过（`scenarios.mjs` 的 downloads，深色主题、简体中文）。已知差异：

- `lunabox://` 安装请求由 `bootstrap` 记下并发出 `install:pending`，但还没有 InstallConfirmModal，所以只会出现封面批量下载任务和从原版数据里带来的任务；原版在根布局里的下载完成、失败提示（`useDownloadNotifications`）也还没有迁移。
- “已取消选择可执行文件”是普通提示，没有原版的 ⚠️ 图标（Toaster 不支持自定义图标）；MyGo 写剪贴板不报告失败，所以没有“复制失败”的提示。
- “已导入”按钮原版是 `disabled:opacity-70`，这里最多 0.5（MyGo 禁用元素的上限，同通用差异）。
- 下载中、空列表、出错、自动解压失败的卡片与浅色主题没有和原版截图对照过（对照数据里只有已完成的任务）。

统计页迁移了除美化导出以外的全部内容，接的是真实服务（`GetGlobalPeriodStats`、`AISummarize`）。没有和原版截图对照过（`scenarios.mjs` 里没有统计页场景；`DESKTOP_MYGO_PNG=<目录> go test ./ui/pages/stats -run TestScreenshots` 画出整页供目测）。已知差异：

- “美化导出”按钮只弹提示（见路线图）。
- 加载较慢时的骨架屏与 AI 总结的占位条都按同一节奏闪烁（原版三条占位条各自延迟 75ms、150ms）；排行榜名次没有 `tabular-nums`。
- 两张趋势图的高度在构建时就要确定：并排时“游玩时长趋势”取“游玩时段分布”上一帧的高度，数据变化后的第一帧可能差一点。

多语言：

- 日期按应用的语言与系统的区域格式显示；原版按 WebView 的默认语言（`toLocaleDateString(undefined)`），两者在语言与系统不一致时可能不同。
- 原版硬编码的中文（如 BetterSelect 的“请选择”）在这里也翻译了（`tools/i18n/extra` 的 `native`）。
- 日语、英语的文字比中文长，有的地方会顶到边缘（如筛选抽屉的“游戏筛选预设”一行），还没有逐页检查。编辑页日期选择器的星期与翻页按钮同原版一样是写死的英文。

原版后端的问题（未修改）：`BackupService.RestoreBackup` 在存档目录里只有一个文件时会把它当作单文件存档，恢复到目录自身的路径上。
