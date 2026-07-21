# LunaBox 升级 Wails v3 Alpha 调研报告

> 调研日期：2026-07-20  
> 调研基线版本：Wails v2.13.0  
> 调研目标版本：Wails v3.0.0-alpha2.117

## 1. 结论

截至 2026-07-20，Wails v3 最新 Go module 版本为 `v3.0.0-alpha2.117`，发布于 2026-07-08。对应 Go module 为：

```text
github.com/wailsapp/wails/v3
```

对应 CLI 安装命令为：

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
```

该版本要求 Go 1.25，LunaBox 当前使用 Go 1.25.7，工具链版本已经满足要求。

Wails v3 仍然是 alpha 预发布版本，Wails v2 仍是稳定版本。对 LunaBox 来说，这不是一次简单的 `go.mod` 依赖升级，而是涉及应用启动、窗口生命周期、服务绑定、前后端 runtime、生成绑定以及 Windows 构建系统的中大型迁移。

当前影响范围：

- 16 个 Go 文件直接导入 Wails v2。
- 后端有 22 处文件对话框调用。
- 后端有 17 处 Wails 事件发送。
- 12 个前端文件使用 v2 runtime。
- 69 个前端文件、135 处 import 使用旧生成绑定。
- 17 个前端文件包含 116 处旧枚举成员引用。
- 项目包含自定义退出同步、关闭到托盘、URL 协议、Windows 会话结束、amd64/arm64 构建、签名和自定义 NSIS 安装流程。

官方迁移指南所说的“典型项目 1-4 小时”不适用于 LunaBox。合理预估为 4-7 个工程日，Windows 生命周期、ARM64、协议和安装包回归测试会占主要时间。建议在独立升级分支完成迁移，不应直接在生产主线上替换。

### 1.1 第一阶段实施状态

第一阶段迁移已在 `wails-v3-new` 分支完成，目标是让应用能够通过 `wails3 dev` 启动并完成前后端调用。当前已经完成：

- Go module 升级到 `github.com/wailsapp/wails/v3 v3.0.0-alpha2.117`。
- 前端 runtime 固定为 `@wailsio/runtime 3.0.0-alpha.97`。
- 后端启动、service 注册、窗口生命周期、关闭 hook、事件、文件拖放、URL 协议、浏览器、对话框、日志和通知 API 迁移。
- 前端 runtime、生成绑定 import、枚举成员、事件 payload 和系统通知调用迁移。
- v3 绑定输出到 `frontend/bindings`，业务侧兼容入口位于 `frontend/src/bindings`；旧 `frontend/wailsjs` 已删除。
- 增加第一阶段开发用 `Taskfile.yml` 和 `build/config.yml`。

2026-07-20 本机验证结果：

| 验证项 | 结果 |
|---|---|
| `wails3 generate bindings -clean=true -ts` | 通过；生成 20 services、250 methods、10 enums、85 models |
| `wails3 build DEV=true` | 通过 |
| `go build -tags dev ./...` | 通过；仅有本机 macOS linker version 告警 |
| `pnpm build` | 通过 |
| `wails3 dev` | 通过；Vite 启动于 `127.0.0.1:9245`，LunaBox 进程和 WebView 正常启动 |
| 前后端通信 | 通过；已观察到环境、窗口、首页、统计和更新检查等实际绑定调用 |
| `go test ./...` | 除未改动的 ReinaManager Windows 路径用例在 macOS 失败外，其余 package 通过 |

验证环境为 macOS，因此只能证明开发链路和跨平台可编译部分可用，不能替代 Windows 行为回归。LunaBox 只支持 Windows，v3 通知 service 仅在 Windows 注册；生成器仍会静态发现并生成通知绑定。

当前 alpha 生成器仍有 8 条非致命告警。其中 6 条来自 service 类型同时被识别为 model，导致生成的 `frontend/bindings/lunabox/internal/service/index.ts` 聚合入口存在重复导出。业务代码使用具体 service 文件而不使用该聚合入口，`tsconfig.json` 也不把整个生成目录作为根文件，因此不影响类型检查和构建。后续升级 Wails alpha 时需要重新确认此问题。

### 1.2 第二阶段遗留工作

当前 `Taskfile.yml` 仅覆盖开发模式，不具备原有发布脚本的完整能力。合并或发布前仍需完成：

- 基于 v3 官方 Windows Taskfile 补齐 production、portable 和 installer 构建。
- 迁移 `scripts/build.bat` 中的版本/commit/build time、第三方 API 配置和 `BuildMode` ldflags 注入。
- 迁移 amd64/arm64、DuckDB ARM64 CGO toolchain、`duckdb_use_lib` tag 和动态库打包。
- 重新生成并移植 v3 NSIS 模板、WebView2 bootstrapper、协议注册、CLI、快捷方式和卸载清理逻辑。
- 恢复 `.syso`、可执行文件与 installer 签名，以及签名后的 portable ZIP 流程。
- 更新 CI workflow、发布输出路径、README 开发/构建命令，并在不再需要后删除 v2 `wails.json`。
- 在 Windows amd64 和 arm64 上完成第 17 节所列的生命周期与安装包回归测试。

## 2. 依赖与工具链

### 2.1 Go 依赖

`go.mod` 中：

```go
github.com/wailsapp/wails/v2 v2.13.0
```

需要替换为：

```go
github.com/wailsapp/wails/v3 v3.0.0-alpha2.117
```

所有 `github.com/wailsapp/wails/v2/...` import 均需要迁移到 v3 对应 package。

### 2.2 前端 runtime

Wails v3 不再把前端 runtime 生成到 `frontend/wailsjs/runtime`。需要在 `frontend/package.json` 增加：

```json
"@wailsio/runtime": "3.0.0-alpha.97"
```

`3.0.0-alpha.97` 是 `v3.0.0-alpha2.117` 源码中使用的 runtime package 版本，应精确锁定，不建议使用 `latest` 或范围版本。

### 2.3 Vite 插件

`frontend/vite.config.ts` 需要加入 Wails v3 Vite 插件，并保留现有 React、UnoCSS 和代理配置：

```ts
import wails from "@wailsio/runtime/plugins/vite";

export default defineConfig({
  plugins: [react(), UnoCSS(), wails("./bindings")],
});
```

### 2.4 CLI 与开发命令

主要命令映射：

| Wails v2 | Wails v3 |
|---|---|
| `wails dev` | `wails3 dev` |
| `wails build` | `wails3 build` |
| `wails generate module` | `wails3 generate bindings -ts` |
| `wails doctor` | `wails3 doctor` |

`wails3 generate bindings` 通过静态分析 Go 源码生成绑定，不会启动应用。因此 `main.go` 中通过 `isBindingsBuild()` 跳过数据库初始化的特殊分支可以删除。

## 3. 应用启动与服务绑定

当前 `main.go` 使用：

```go
wails.Run(&options.App{
    Bind:     bindServices,
    EnumBind: enumBindings,
})
```

Wails v3 将应用、窗口和执行阶段拆开：

```go
app := application.New(application.Options{
    Name: "LunaBox",
    Services: []application.Service{
        application.NewService(gameService),
        application.NewService(configService),
        // 其他前端可调用服务
    },
    Assets: application.AssetOptions{
        Handler:    application.AssetFileServerFS(assets),
        Middleware: assetMiddleware,
    },
    Logger:     slogLogger,
    OnShutdown: shutdown,
})

mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
    Title:          "LunaBox",
    Width:          initWidth,
    Height:         initHeight,
    MinWidth:       970,
    MinHeight:      563,
    Hidden:         true,
    Frameless:      true,
    EnableFileDrop: true,
    BackgroundType: application.BackgroundTypeTranslucent,
    BackgroundColour: application.NewRGBA(18, 20, 22, 0),
    Windows: application.WindowsWindow{
        BackdropType: application.Auto,
        Theme:        application.SystemDefault,
    },
})

mainWindow.RegisterHook(events.Common.WindowClosing, handleWindowClosing)

if err := app.Run(); err != nil {
    // 处理启动错误
}
```

### 3.1 Bind 迁移

```go
Bind: []interface{}{gameService}
```

改为：

```go
Services: []application.Service{
    application.NewService(gameService),
}
```

绑定生成器会静态分析 `application.NewService(...)`。注册代码应保持直接、明确，避免封装到生成器无法分析的动态 helper 中。

### 3.2 EnumBind 迁移

Wails v3 会自动发现服务方法和模型可达的 Go 枚举类型，不再提供 `EnumBind`。

当前 `enumBindings` 应删除。仅为 v2 `EnumBind` 准备的 `AllGameStatuses`、`AllSourceTypes` 等 `TSName` 映射，如果没有其他后端用途，也可以随后删除。

需要确认所有前端使用的枚举仍能通过服务参数、返回值或模型字段被生成器发现。`PromptType` 尤其需要验证，因为它是否可达取决于配置模型的字段类型。

## 4. 生命周期与窗口退出

这是 LunaBox 迁移风险最高的部分。

当前 `OnBeforeClose` 同时负责：

- 保存非最大化状态下的窗口尺寸。
- 识别托盘或系统会话结束触发的强制退出。
- 阻止重复退出请求。
- 关闭到托盘。
- 请求前端完成退出前云同步和本地备份。
- 最终允许或取消关闭。

Wails v3 中应改为可取消的窗口 hook：

```go
mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
    if !mainWindow.IsMaximised() {
        width, height := mainWindow.Size()
        config.WindowWidth = width
        config.WindowHeight = height
    }

    if appState.ShouldForceQuit() {
        return
    }

    if appState.HasPendingQuitRequest() {
        event.Cancel()
        return
    }

    if config.CloseToTray && appState.IsTrayAvailable() {
        mainWindow.Hide()
        event.Cancel()
        return
    }

    if shouldRunFrontendQuitSync(config) {
        appState.RequestFrontendQuitSync("window-close")
        event.Cancel()
    }
})
```

只有 `RegisterHook` 可以取消关闭，`OnWindowEvent` 只能观察事件。

### 4.1 OnStartup

v3 的 `application.Options` 不再有 `OnStartup`。当前启动逻辑需要按职责拆分到以下位置：

- 应用创建前或 `app.Run()` 前：配置、数据库、migration、服务依赖组装。
- `ServiceStartup(ctx, options)`：需要生命周期 context 的服务初始化。
- `events.Common.ApplicationStarted`：依赖原生应用已经启动的操作。
- 窗口 runtime ready 事件：依赖前端 runtime 已经建立的操作。

LunaBox 当前启动任务中，系统托盘、Windows session-end hook、IPC server、MCP server、启动云同步和待处理协议请求的执行时机都需要重新确认。

### 4.2 OnShutdown

v2：

```go
OnShutdown: func(ctx context.Context) {}
```

v3：

```go
OnShutdown: func() {}
```

当前关闭 IPC、MCP、图片代理、DuckDB、session-end hook 和托盘的顺序可以保留，但不能继续依赖 v2 runtime context。

### 4.3 前端关闭按钮

当前自定义标题栏调用 `Quit()`。迁移后应该调用：

```ts
Window.Close();
```

这样会触发 `WindowClosing` hook，保留“关闭到托盘”和退出同步行为。

真正的强制退出路径才调用：

```ts
Application.Quit();
```

或后端 `app.Quit()`。

## 5. 后端 Runtime API

Wails v3 删除了依赖 `context.Context` 的全局 runtime 调用，改为 application/window 对象方法。

常用映射：

| Wails v2 | Wails v3 |
|---|---|
| `runtime.EventsEmit(ctx, name, data)` | `app.Event.Emit(name, data)` |
| `runtime.WindowShow(ctx)` | `mainWindow.Show()` |
| `runtime.WindowHide(ctx)` | `mainWindow.Hide()` |
| `runtime.WindowUnminimise(ctx)` | `mainWindow.Restore()` |
| `runtime.WindowIsMaximised(ctx)` | `mainWindow.IsMaximised()` |
| `runtime.WindowGetSize(ctx)` | `mainWindow.Size()` |
| `runtime.BrowserOpenURL(ctx, url)` | `app.Browser.OpenURL(url)` |
| `runtime.Quit(ctx)` | `app.Quit()` |

LunaBox 当前通过 `internal/wailsruntime.Runtime` 将 `*application.App` 与主窗口包装为窄接口，并在 `main.go` 中显式注入相关 service。该适配器只保留 v3 对象式方法，不再模拟依赖 `context.Context` 的 v2 全局 runtime；在 v3 正式版 API 稳定前继续用它集中隔离 alpha 变动。

现有业务 context 仍然可以用于数据库、网络请求和协程取消，不需要因为 Wails runtime 改造而全部移除。

## 6. 文件对话框

LunaBox 有 22 处 v2 文件对话框调用，分布在 config、game、import、backup、stats 和 template service。

打开文件：

```go
path, err := app.Dialog.OpenFile().
    SetTitle("选择图片").
    SetDirectory(defaultDirectory).
    AddFilter("图片", "*.png;*.jpg;*.jpeg").
    PromptForSingleSelection()
```

选择目录：

```go
path, err := app.Dialog.OpenFile().
    SetTitle("选择目录").
    CanChooseDirectories(true).
    CanChooseFiles(false).
    PromptForSingleSelection()
```

保存文件：

```go
path, err := app.Dialog.SaveFile().
    SetTitle("导出").
    SetDirectory(defaultDirectory).
    SetFilename(defaultFilename).
    AddFilter("JSON", "*.json").
    PromptForSingleSelection()
```

过滤器的多个扩展名使用分号分隔，例如 `*.png;*.jpg`。

## 7. 日志系统

当前 `internal/applog/FileLogger` 实现的是 v2 `logger.Logger`。Wails v3 的 application options 接收：

```go
Logger *slog.Logger
```

需要将 `FileLogger` 改为 `slog.Handler`，或者增加一个小型 `slog.Handler` 适配器，再构造：

```go
slog.New(fileHandler)
```

`internal/applog/logger.go` 中的 `runtime.LogInfof` 等调用也需要改为新的 logger。

项目约有 447 处通过 `applog` wrapper 写日志，但只要保持现有 `applog.LogInfof` 等对外 API 不变，绝大多数业务调用不需要修改。

## 8. Asset Server 与代理

当前资源 middleware 的函数形态与 v3 基本相同：

```go
func(next http.Handler) http.Handler
```

因此以下逻辑可以保留：

- CORS header。
- `OPTIONS` 请求处理。
- `/local/` 本地文件 handler。
- `/proxy/image` 远程图片代理。
- 其余请求交给内嵌 `frontend/dist`。

挂载位置改为：

```go
Assets: application.AssetOptions{
    Handler:    application.AssetFileServerFS(assets),
    Middleware: middleware,
}
```

开发模式下当前 Vite `/proxy/image` 到 `127.0.0.1:23680` 的代理仍可保留。

## 9. 前端 Runtime 迁移

v3 统一从 `@wailsio/runtime` 导入：

```ts
import {
  Application,
  Browser,
  Clipboard,
  Events,
  System,
  Window,
} from "@wailsio/runtime";
```

主要映射：

| Wails v2 | Wails v3 |
|---|---|
| `EventsOn(name, handler)` | `Events.On(name, handler)` |
| handler 直接收到 payload | handler 收到 event，payload 为 `event.data` |
| `BrowserOpenURL(url)` | `Browser.OpenURL(url)` |
| `ClipboardSetText(text)` | `Clipboard.SetText(text)` |
| `WindowShow()` | `Window.Show()` |
| `WindowMinimise()` | `Window.Minimise()` |
| `WindowMaximise()` | `Window.Maximise()` |
| `WindowUnmaximise()` | `Window.UnMaximise()` 或 `Window.Restore()` |
| `WindowIsMaximised()` | `Window.IsMaximised()` |
| `Quit()` | `Application.Quit()` |
| `Environment()` | `System.Environment()` |

环境字段从 v2 的 `environment.platform` 改为 v3 的 `environment.OS`。

`Clipboard.SetText()` 返回 `Promise<void>`，不能继续依赖 v2 的布尔返回值。

## 10. 文件拖放

v3 删除了：

```ts
OnFileDrop(...)
OnFileDropOff()
```

窗口需要启用：

```go
EnableFileDrop: true
```

前端目标元素需要使用：

```tsx
<div data-file-drop-target>
```

当前根元素上的：

```tsx
style={{ "--wails-drop-target": "drop" }}
```

应删除。

后端监听：

```go
mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
    paths := event.Context().DroppedFiles()
    app.Event.Emit("files-dropped", paths)
})
```

前端再通过 `Events.On("files-dropped", ...)` 打开现有导入弹窗。现有 HTML drag overlay 逻辑可以保留，但要验证它不会拦截 Wails v3 的外部文件拖放处理。

## 11. 系统通知

当前 `frontend/src/utils/systemNotification.ts` 使用的以下 v2 runtime API 在 v3 中不存在：

- `InitializeNotifications`
- `IsNotificationAvailable`
- `SendNotification`

Wails v3 将通知实现为 Go service：

```go
import "github.com/wailsapp/wails/v3/pkg/services/notifications"

notifier := notifications.New()

app := application.New(application.Options{
    Services: []application.Service{
        application.NewService(notifier),
    },
})
```

前端通过生成绑定调用：

- `CheckNotificationAuthorization`
- `RequestNotificationAuthorization`
- `SendNotification`

Windows 上授权检查始终返回 true，但仍应保留错误处理。现有前端 `NotificationOptions` 也要映射到 v3 `notifications.NotificationOptions`。

## 12. 生成绑定与枚举

### 12.1 输出目录

v2 输出目录：

```text
frontend/wailsjs/go/...
```

v3 输出目录：

```text
frontend/bindings/<完整 Go import path>/...
```

LunaBox 的 module 名为 `lunabox`，因此预计会生成类似：

```text
frontend/bindings/lunabox/internal/service/...
frontend/bindings/lunabox/internal/common/enums/models.ts
```

最终路径应以生成器实际输出为准，不应提前手工创建生成文件。

### 12.2 interface 与 class

当前 v3 React 模板的 Taskfile 使用：

```bash
wails3 generate bindings -ts -i
```

`-i` 会生成 TypeScript interface，不包含构造函数。LunaBox 当前存在：

```ts
new models.Game(...)
new models.GameProgress(...)
```

为了最小改动，建议生成绑定时去掉 `-i`：

```bash
wails3 generate bindings -ts
```

这样继续生成 class 模型。另一种方案是保留 interface 并改造所有模型构造，但没有明显收益。

### 12.3 枚举成员名

v2 当前通过 `TSName` 生成：

```ts
enums.GameStatus.NOT_STARTED
```

v3 使用 Go 常量标识符，预计变为：

```ts
GameStatus.StatusNotStarted
```

其他例子：

- `SourceType.BANGUMI` -> `SourceType.Bangumi`
- `LaunchMode.NORMAL` -> 对应 Go 常量名
- `Period.WEEK` -> 对应 Go 常量名

命名类型枚举还会额外生成 `$zero` 成员。需要在生成绑定后以编译错误为依据批量调整 116 处引用。

## 13. Windows 构建系统

Wails v3 使用 Taskfile 驱动构建。当前 alpha 模板的主要文件为：

- `Taskfile.yml`
- `build/Taskfile.yml`
- `build/windows/Taskfile.yml`
- `build/config.yml`
- `build/windows/nsis/project.nsi`
- `build/windows/nsis/wails_tools.nsh`

当前官方迁移页面仍展示 v3 `wails.json` 配置，但该部分已经落后于 `v3.0.0-alpha2.117` 的实际模板和构建源码。迁移时应以当前模板的 Taskfile 和 `build/config.yml` 为准。

### 13.1 wails3 build 参数变化

`wails3 build` 是 `wails3 task build` 的薄封装，不再支持 v2 的：

- `-platform`
- `-o`
- `-ldflags`
- `-devtools`
- `-clean`
- `-skipbindings`

跨平台参数使用：

```bash
wails3 build GOOS=windows GOARCH=amd64
wails3 build GOOS=windows GOARCH=arm64
```

更复杂的控制通过 Taskfile 变量和 platform task 实现。

### 13.2 scripts/build.bat

当前 `scripts/build.bat` 不能只替换命令名。以下能力需要接入 v3 Taskfile 或重新组织：

- portable 与 installer 的不同 `BuildMode` ldflags。
- 版本、Git commit、构建时间注入。
- Bangumi、Hikarinagi、TouchGal、Umbra 配置注入。
- DuckDB ARM64 CGO toolchain 和 `duckdb_use_lib` tag。
- GUI 输出名称和路径。
- CLI 单独构建。
- portable ZIP 内容布局。
- installer payload 签名和签名后打包。
- 7z 与 DuckDB DLL 打包。

推荐基于当前 v3 Windows Taskfile 增加 LunaBox 所需变量，不要完全绕过 v3 build task，否则容易漏掉前端构建、绑定生成、production tag 和 `.syso` 生成。

### 13.3 WebView2 与 NSIS

当前脚本从 Wails v2 module 内部目录复制 WebView2 bootstrapper。v3 提供正式命令：

```bash
wails3 generate webview2bootstrapper -dir build/windows/nsis
```

NSIS 目录从当前：

```text
build/windows/installer
```

迁移为 v3 模板的：

```text
build/windows/nsis
```

不能继续完整复用 v2 自动生成的 `wails_tools.nsh`。应以 v3 模板重新生成，然后把 LunaBox 的快捷方式、协议、CLI、运行库和卸载清理逻辑移植到新模板。

### 13.4 CI

以下工作流需要修改 CLI 安装和构建命令：

- `.github/workflows/build.yml`
- `.github/workflows/release.yml`
- `.github/workflows/autobuild.yml`

建议从 `go.mod` 读取 v3 版本并安装同版本 CLI：

```powershell
$WailsVersion = (go list -m -f "{{.Version}}" github.com/wailsapp/wails/v3).Trim()
go install "github.com/wailsapp/wails/v3/cmd/wails3@$WailsVersion"
```

同时需要调整构建输出路径、Taskfile 参数和 NSIS 路径。

## 14. URL 协议与单实例

当前 `wails.json` 中的 `lunabox` 协议需要迁移到：

```yaml
# build/config.yml
protocols:
  - scheme: lunabox
    description: LunaBox Protocol
```

v3 在运行时通过以下事件接收 URL：

```go
app.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(event *application.ApplicationEvent) {
    rawURL := event.Context().URL()
    // 复用现有 protocol.ParseAction 等逻辑
})
```

v3 还提供 `SingleInstanceOptions.OnSecondInstanceLaunch`，能够将第二次启动的参数传给主实例。

首轮迁移建议保留 LunaBox 现有 IPC server/client：

- CLI 仍然需要 IPC 服务。
- 当前协议安装和启动逻辑已经基于 IPC 工作。
- 同时替换单实例、协议和 CLI IPC 会扩大风险。

后续可以让 v3 SingleInstance 负责 GUI 第二次启动和协议转发，现有 HTTP IPC 只服务 CLI。采用该方案时需要避免双重实例锁和重复分发。

## 15. 系统托盘

LunaBox 当前使用 `github.com/energye/systray`，没有直接依赖 Wails v2，因此不是首轮迁移的强制项。

Wails v3 提供原生托盘：

```go
tray := app.SystemTray.New()
tray.SetIcon(icon)
tray.SetMenu(menu)
```

考虑到当前托盘与窗口显示、退出同步和 shutdown 顺序紧密耦合，建议首轮保留现有实现。在核心迁移稳定后，再单独迁移原生托盘。

## 16. 推荐实施顺序

1. 创建独立升级分支，固定 Go 和 npm alpha 版本。
2. 从全新 v3 React 项目引入 Taskfile 和 `build/config.yml` 基础结构。
3. 重写 `main.go` 的 application、window 和生命周期。
4. 适配 `slog`，建立后端 application/window 引用。
5. 转换后端事件、窗口、浏览器和 22 处文件对话框调用。
6. 注册全部 service，生成 v3 bindings。
7. 更新 69 个前端文件的绑定 import 和枚举成员。
8. 迁移前端 runtime、标题栏、文件拖放和通知。
9. 移植 Windows Taskfile、自定义 NSIS、协议注册和签名流水线。
10. 更新 README、AGENTS 和开发文档中的命令。
11. 完成 Windows amd64 与 arm64 回归测试。

## 17. Windows 验证清单

- 普通启动和开发模式热更新。
- 自启动时保持隐藏。
- 托盘显示主窗口。
- 最小化、最大化、恢复和保存窗口尺寸。
- 标题栏关闭、Alt+F4 和任务栏关闭。
- 关闭到托盘。
- 退出前云同步和本地备份。
- 托盘强制退出。
- Windows 注销、关机和 session-end hook。
- `lunabox://install` 和 `lunabox://launch` 冷启动。
- 协议唤起已经运行的实例。
- CLI IPC 请求。
- 文件和目录选择对话框。
- 保存文件对话框及过滤器。
- 外部文件拖入与导入弹窗。
- `/local/` 图片加载。
- `/proxy/image` 开发和生产模式代理。
- 系统通知。
- 更新下载和浏览器跳转。
- portable amd64/arm64 包。
- installer amd64/arm64 构建、签名、安装、升级和卸载。
- DuckDB 与 7z DLL/可执行文件随包发布。

## 18. 风险评估

| 风险 | 等级 | 说明 |
|---|---|---|
| Wails v3 API 继续变化 | 高 | alpha 版本可能在后续升级中再次产生破坏性变更 |
| 窗口关闭与退出同步回归 | 高 | 涉及 hook、托盘、云同步、备份和系统会话结束 |
| 自定义 NSIS 迁移 | 高 | v2 与 v3 生成宏和目录结构不同 |
| ARM64 CGO 构建 | 高 | Taskfile、toolchain、DuckDB 动态库和签名共同影响 |
| 生成绑定与枚举变更 | 中 | 修改量较大，但大部分可通过 TypeScript 编译错误机械处理 |
| 文件对话框迁移 | 中 | 调用较多，API 转换直接但需要逐个验证过滤器和默认路径 |
| 前端 runtime 迁移 | 中 | API 映射清晰，但 event payload 和关闭语义有行为变化 |
| 资源 middleware | 低 | v3 middleware 类型与当前实现基本兼容 |
| 现有第三方托盘 | 低至中 | 可以首轮保留，但必须验证与 v3 event loop 的协作 |

## 19. 官方资料

- [Wails v3.0.0-alpha2.117 Release](https://github.com/wailsapp/wails/releases/tag/v3.0.0-alpha2.117)
- [Migrating from v2 to v3](https://v3.wails.io/migration/v2-to-v3/)
- [Build System](https://v3.wails.io/concepts/build-system/)
- [Lifecycle](https://v3.wails.io/concepts/lifecycle/)
- [Manager API](https://v3.wails.io/concepts/manager-api/)
- [Frontend Runtime](https://v3.wails.io/reference/frontend-runtime/)
- [Bindings](https://v3.wails.io/features/bindings/)
- [File Dialogs](https://v3.wails.io/features/dialogs/file/)
- [File Drop](https://v3.wails.io/features/drag-and-drop/files/)
- [Custom URL Protocols](https://v3.wails.io/guides/distribution/custom-protocols/)
- [Single Instance](https://v3.wails.io/guides/single-instance/)
- [Notifications](https://v3.wails.io/features/notifications/overview/)

## 20. 最终建议

如果目标是稳定发布，当前应继续维护 Wails v2.13.0，同时建立 v3 alpha 实验分支验证核心流程。等以下条件满足后再考虑合并：

- 主窗口和退出生命周期完全通过 Windows 回归测试。
- 新绑定生成稳定且前端类型检查通过。
- amd64/arm64 portable 与 installer 均能从 CI 产出并安装。
- URL 协议、CLI IPC、系统通知和文件拖放行为一致。
- 已接受后续 alpha 版本可能需要继续跟进破坏性变更的维护成本。
