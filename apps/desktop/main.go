package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"lunabox/backend/applog"
	"lunabox/backend/bootstrap"
	"lunabox/backend/platform"
	"lunabox/backend/utils"
	"lunabox/backend/utils/apputils"
	"lunabox/backend/utils/imageutils"
	"lunabox/wailsruntime"
	"net"
	"net/http"
	"os"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

//go:embed build/darwin/appicon.png
var darwinAppIcon []byte

//go:embed build/darwin/tray-template.png
var darwinTrayIcon []byte

//go:embed build/windows/tray.png
var windowsTrayIcon []byte

const (
	applicationUniqueID      = "io.github.saramanda9988.lunabox"
	remoteImageProxyHTTPAddr = "127.0.0.1:23680"
)

// mainWindow 用 Wails 的窗口实现 bootstrap.Window。
type mainWindow struct {
	window *application.WebviewWindow
}

func (w mainWindow) Show()                     { w.window.Show() }
func (w mainWindow) Hide()                     { w.window.Hide() }
func (w mainWindow) Focus()                    { w.window.Focus() }
func (w mainWindow) UnMinimise()               { w.window.UnMinimise() }
func (w mainWindow) IsMinimised() bool         { return w.window.IsMinimised() }
func (w mainWindow) IsMaximised() bool         { return w.window.IsMaximised() }
func (w mainWindow) Size() (width, height int) { return w.window.Size() }

func configureTray(app *application.App, core *bootstrap.App, showStartupErrorPreview func()) {
	menu := app.NewMenu()
	menu.Add("显示主窗口").OnClick(func(_ *application.Context) {
		core.ShowMainWindow()
	})
	if showStartupErrorPreview != nil {
		menu.AddSeparator()
		menu.Add("开发：预览启动错误窗").OnClick(func(_ *application.Context) {
			showStartupErrorPreview()
		})
	}
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(_ *application.Context) {
		core.RequestQuit("tray-menu")
	})

	tray := app.SystemTray.New()
	if goruntime.GOOS == "linux" {
		tray.SetLabel("LunaBox")
	}
	tray.SetMenu(menu)
	tray.SetTooltip("LunaBox")
	if goruntime.GOOS == "darwin" {
		tray.SetTemplateIcon(darwinTrayIcon)
	} else if goruntime.GOOS == "linux" {
		tray.SetIcon(appIcon)
		tray.OnClick(core.ShowMainWindow)
		tray.OnDoubleClick(core.ShowMainWindow)
	} else {
		// Wails v3 alpha passes a complete ICO container to an API that expects
		// one image resource. Use the extracted 32x32 ICO frame for the tray.
		tray.SetIcon(windowsTrayIcon)
		tray.OnClick(core.ShowMainWindow)
		tray.OnDoubleClick(core.ShowMainWindow)
	}
	core.SetTrayAvailable()
}

type startupCoordinator struct {
	mu      sync.RWMutex
	ctx     context.Context
	startup func(context.Context)
}

func frontendAssetHandler(assets fs.FS) http.Handler {
	fileServer := application.AssetFileServerFS(assets)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wails serves files without an SPA fallback. Keep the browser URL at
		// /startup so main.tsx selects StartupWindow, but serve the shared entry.
		if r.URL.Path == "/startup" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			r = r.Clone(r.Context())
			r.URL.Path = "/index.html"
			r.URL.RawPath = ""
		}
		fileServer.ServeHTTP(w, r)
	})
}

func (s *startupCoordinator) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	s.startup(ctx)
	return nil
}

func (s *startupCoordinator) Context() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ctx
}

func main() {
	applog.SetMode(applog.ModeCLI)
	appLogger, logDirErr := bootstrap.NewLogger()
	if logDirErr != nil {
		fmt.Fprintf(os.Stderr, "prepare application log directory failed: %v\n", logDirErr)
		os.Exit(1)
	}
	appLogger.Info("application startup initiated")
	for _, runtimeEnv := range platform.ConfigureRuntimeEnvironment() {
		appLogger.Info(fmt.Sprintf("runtime environment configured: %s=%s (%s)", runtimeEnv.Key, runtimeEnv.Value, runtimeEnv.Reason))
	}

	// ================================================================
	// 启动参数预处理：在 Wails 初始化之前处理协议参数
	// ================================================================
	args, launchedByAutostart := bootstrap.ExtractAutostartLaunchFlag(os.Args[1:])

	// lunabox:// URL：GUI 已运行时转交给它
	initialProtocolRequest, forwarded, err := bootstrap.TakeProtocolRequest(args, appLogger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if forwarded {
		return
	}

	runGUI(appLogger, launchedByAutostart, initialProtocolRequest)
}

func runGUI(
	appLogger *applog.FileLogger,
	launchedByAutostart bool,
	initialProtocolRequest *bootstrap.ProtocolRequest,
) {
	core := bootstrap.New(bootstrap.Options{
		Logger:         appLogger,
		InitialRequest: initialProtocolRequest,
	})
	svc := core.Services

	var localFileHandler http.Handler
	var remoteImageProxyHandler http.Handler
	var remoteImageProxyHTTPServer *http.Server
	var assetHandlersMu sync.RWMutex

	coordinator := &startupCoordinator{
		startup: func(ctx context.Context) {
			applog.SetMode(applog.ModeGUI)
			applog.SetLogger(appLogger)
			if os.Getenv("FRONTEND_DEVSERVER_URL") != "" {
				if err := utils.LoadEnvFilesIfExists(".env.build", ".env"); err != nil {
					appLogger.Warning("failed to load dev env files: " + err.Error())
				}
				utils.ApplyDevBuildEnvFallbacks()
			}
		},
	}

	applicationServices := []application.Service{
		application.NewService(coordinator),
		application.NewService(svc.Startup),
		application.NewService(svc.Game),
		application.NewService(svc.Bangumi),
		application.NewService(svc.Hikarinagi),
		application.NewService(svc.Kungal),
		application.NewService(svc.AI),
		application.NewService(svc.Backup),
		application.NewService(svc.CloudSync),
		application.NewService(svc.Home),
		application.NewService(svc.Stats),
		application.NewService(svc.Start),
		application.NewService(svc.Integration),
		application.NewService(svc.Category),
		application.NewService(svc.Config),
		application.NewService(svc.Import),
		application.NewService(svc.Version),
		application.NewService(svc.Template),
		application.NewService(svc.Update),
		application.NewService(svc.Session),
		application.NewService(svc.Download),
		application.NewService(svc.GameProgress),
		application.NewService(svc.GameReview),
		application.NewService(svc.Tag),
		application.NewService(svc.GameFilterPreset),
		application.NewService(svc.PortableSetup),
	}

	core.OnShutdown("shutdown remote image proxy server", func() {
		if remoteImageProxyHTTPServer == nil {
			return
		}
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := remoteImageProxyHTTPServer.Shutdown(closeCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Error("failed to shutdown remote image proxy server: " + err.Error())
		}
		remoteImageProxyHTTPServer = nil
	})

	applicationIcon := appIcon
	if goruntime.GOOS == "darwin" {
		applicationIcon = darwinAppIcon
	}

	wailsApp := application.New(application.Options{
		Name:        "LunaBox",
		Description: "LunaBox game library manager",
		Icon:        applicationIcon,
		Logger:      appLogger.Slog(),
		LogLevel:    bootstrap.LogLevel,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: applicationUniqueID,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				core.HandleSecondInstance(nil)
			},
		},
		Assets: application.AssetOptions{
			Handler: frontendAssetHandler(assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					if r.Method == http.MethodOptions {
						w.WriteHeader(http.StatusOK)
						return
					}
					assetHandlersMu.RLock()
					localHandler := localFileHandler
					imageHandler := remoteImageProxyHandler
					assetHandlersMu.RUnlock()
					if localHandler != nil && strings.HasPrefix(r.URL.Path, "/local/") {
						localHandler.ServeHTTP(w, r)
						return
					}
					if imageHandler != nil && r.URL.Path == "/proxy/image" {
						imageHandler.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			ApplicationID: applicationUniqueID,
		},
		Services:   applicationServices,
		OnShutdown: core.Shutdown,
		ShouldQuit: func() bool {
			if goruntime.GOOS != "darwin" {
				return true
			}
			return core.ShouldQuitApplication()
		},
	})

	svc.Startup.SetEventEmitter(func(name string, data ...interface{}) {
		wailsApp.Event.Emit(name, data...)
	})
	newStartupErrorWindow := func(name string, hidden bool) *application.WebviewWindow {
		return wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:             name,
			Title:            "LunaBox",
			URL:              "/startup",
			Width:            760,
			Height:           360,
			MinWidth:         760,
			MinHeight:        360,
			MaxWidth:         760,
			MaxHeight:        360,
			AlwaysOnTop:      true,
			Hidden:           hidden,
			DisableResize:    true,
			Frameless:        goruntime.GOOS != "darwin",
			InitialPosition:  application.WindowCentered,
			BackgroundType:   application.BackgroundTypeTranslucent,
			BackgroundColour: application.NewRGBA(18, 20, 22, 0),
			Windows: application.WindowsWindow{
				BackdropType: application.Auto,
				Theme:        application.SystemDefault,
			},
			Mac: application.MacWindow{
				TitleBar: application.MacTitleBarHidden,
			},
		})
	}
	// 启动错误窗只在启动失败时按需创建：若预先创建隐藏窗口并在启动成功后 Close，
	// 可能在其 WebView2 仍在初始化时销毁窗口，触发 Wails 的 fatal 错误导致闪退。
	showStartupFailureWindow := func() {
		startupWindow := newStartupErrorWindow("startup", true)
		startupWindow.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
			core.ForceQuit()
			go wailsApp.Quit()
		})
		startupWindow.Center()
		startupWindow.Show()
		startupWindow.Focus()
	}
	var startupPreviewSequence atomic.Uint64
	var showStartupErrorPreview func()
	if strings.TrimSpace(os.Getenv("FRONTEND_DEVSERVER_URL")) != "" {
		showStartupErrorPreview = func() {
			svc.Startup.ReportFailure(
				"开发预览：数据库启动失败\n\n" +
					"打开数据库失败: IO Error: 无法打开 lunabox.db，文件可能正由另一个进程使用\n\n" +
					"此信息仅用于检查启动错误窗的界面样式。",
			)
			previewWindow := newStartupErrorWindow(
				fmt.Sprintf("startup-preview-%d", startupPreviewSequence.Add(1)),
				true,
			)
			previewWindow.Center()
			previewWindow.Show()
			previewWindow.Focus()
		}
	}

	// prepareAssetHandlers 让 WebView 读取本地文件（/local/）与远程图片（/proxy/image 与独立的图片代理端口）。
	prepareAssetHandlers := func() {
		preparedLocalFileHandler, err := apputils.NewLocalFileHandler()
		if err != nil {
			appLogger.Error("Failed to create local file handler: " + err.Error())
		}
		preparedImageProxyHandler := imageutils.NewRemoteImageProxyHandler(core.Config)
		assetHandlersMu.Lock()
		if err == nil {
			localFileHandler = preparedLocalFileHandler
		}
		remoteImageProxyHandler = preparedImageProxyHandler
		assetHandlersMu.Unlock()
		remoteImageProxyListener, listenErr := net.Listen("tcp", remoteImageProxyHTTPAddr)
		if listenErr != nil {
			appLogger.Warning("Failed to start remote image proxy server: " + listenErr.Error())
			return
		}
		imageProxyMux := http.NewServeMux()
		imageProxyMux.Handle("/proxy/image", preparedImageProxyHandler)
		server := &http.Server{Handler: imageProxyMux, ReadHeaderTimeout: 5 * time.Second}
		remoteImageProxyHTTPServer = server
		go func() {
			if serveErr := server.Serve(remoteImageProxyListener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				appLogger.Error("Remote image proxy server failed: " + serveErr.Error())
			}
		}()
	}

	createMainWindow := func() error {
		config := core.Config
		initWidth := config.WindowWidth
		if initWidth < 970 {
			initWidth = 970
		}
		initHeight := config.WindowHeight
		if initHeight < 563 {
			initHeight = 563
		}
		startState := application.WindowStateNormal
		if config.WindowMaximised {
			startState = application.WindowStateMaximised
		}
		window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:             "main",
			Title:            "LunaBox",
			URL:              "/",
			Width:            initWidth,
			Height:           initHeight,
			MinWidth:         970,
			MinHeight:        563,
			StartState:       startState,
			Hidden:           true,
			Frameless:        goruntime.GOOS != "darwin",
			EnableFileDrop:   true,
			BackgroundType:   application.BackgroundTypeTranslucent,
			BackgroundColour: application.NewRGBA(18, 20, 22, 0),
			Windows: application.WindowsWindow{
				BackdropType: application.Auto,
				Theme:        application.SystemDefault,
			},
			Mac: application.MacWindow{
				TitleBar: application.MacTitleBarHidden,
				Backdrop: application.MacBackdropTranslucent,
			},
		})
		core.Attach(mainWindow{window}, wailsruntime.New(wailsApp, window), wailsApp.Quit)
		configureTray(wailsApp, core, showStartupErrorPreview)

		window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
			wailsApp.Event.Emit("files-dropped", event.Context().DroppedFiles())
		})
		window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
			if !core.MainWindowClosing() {
				event.Cancel()
			}
		})
		return nil
	}

	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		go func() {
			err := core.Startup(coordinator.Context(), func() error {
				prepareAssetHandlers()
				return createMainWindow()
			}, launchedByAutostart)
			if err != nil {
				showStartupFailureWindow()
			}
		}()
	})
	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(event *application.ApplicationEvent) {
		core.HandleLaunchedWithURL(event.Context().URL())
	})

	if err := wailsApp.Run(); err != nil {
		appLogger.Fatal(err.Error())
	}
}
