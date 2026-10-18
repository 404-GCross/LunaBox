// Package ui 是 LunaBox 前端在 MyGo 原生 UI 上的一比一实现，目录结构对应原版 frontend/src：
//
//	ui/app.go、routes.go   routes/__root.tsx 与路由表（MyGo 的 ui.Router）
//	ui/pages/home          routes/index.tsx（首页）
//	ui/pages/library       routes/library.tsx
//	ui/pages/game          routes/game.tsx（游戏详情）
//	ui/pages/categories    routes/categories.tsx（收藏夹列表）
//	ui/pages/category      routes/category.tsx（收藏详情）
//	ui/pages/settings      routes/settings.tsx（设置）
//	ui/pages/downloads     routes/downloads.tsx（下载管理）
//	ui/pages/stats         routes/stats.tsx（统计）
//	ui/pages/startup       components/startup/StartupWindow.tsx（启动失败窗口）
//	ui/components          components/ui（Better* 系列等通用组件）
//	ui/components/bar      components/bar（TopBar、SideBar）
//	ui/components/card     components/card（GameCard、CategoryCard、DownloadCard）
//	ui/components/grid     components/grid（VirtualGameGrid）
//	ui/theme、ui/assets    uno.config.ts 的设计令牌、图标与图片
//	ui/env、covers、perf   页面共享的运行环境、封面缓存、性能统计
package ui

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/bar"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/pages/categories"
	"lunabox/desktop-mygo/ui/pages/category"
	"lunabox/desktop-mygo/ui/pages/downloads"
	"lunabox/desktop-mygo/ui/pages/game"
	"lunabox/desktop-mygo/ui/pages/home"
	"lunabox/desktop-mygo/ui/pages/library"
	"lunabox/desktop-mygo/ui/pages/settings"
	"lunabox/desktop-mygo/ui/pages/stats"
	"lunabox/desktop-mygo/ui/perf"
	"lunabox/desktop-mygo/ui/theme"
)

// Backend 是界面用到的后端能力。
type Backend = env.Backend

// Options 是界面的启动参数与调试开关。
type Options struct {
	Route     string // 启动时显示的页面，默认首页 /
	HUD       bool
	Bench     bool // 加载后自动滚动压测
	BenchExit bool
	// ConfigSaved 在设置页保存配置、界面应用新的语言与主题之后（主线程）调用，app 包用它更新托盘菜单。
	ConfigSaved func(prev, next appconf.AppConfig)
}

// App 是整个窗口的状态，View 由它构建。
type App struct {
	env   *env.Env
	opts  Options
	perf  *perf.Perf
	bench *perf.Bench

	router     *ui.Router
	sidebar    bar.Sidebar
	home       *home.Page
	library    *library.Page
	detail     *game.Page
	categories *categories.Page
	category   *category.Page
	settings   *settings.Page
	downloads  *downloads.Page
	stats      *stats.Page
	location   string  // 上一次构建时的路由位置，变化时详情页、收藏页重新加载
	scale      float64 // 窗口所在显示器的缩放比例
	maximised  bool

	showHUD bool
	hudOn   atomic.Bool

	notices     []env.Notice // 事件回调里产生、下一帧显示的提示
	quitSyncing bool         // 正在退出前同步（只做一次）
}

// New 创建应用；list 是游戏库的列表查询，svc 是其余页面用的后端服务。
func New(list Backend, svc *api.Services, opts Options) *App {
	route := opts.Route
	if route == "" {
		route = env.PathHome
	}
	router := ui.NewRouter(route)
	router.Transition = ui.TransitionNone // 原版切换页面没有过渡
	e := &env.Env{Backend: list, Svc: svc, Router: router, Covers: covers.New(), Theme: theme.New(true, 1)}
	cfg := list.Config()
	p := perf.New()
	lib := library.New(e)
	a := &App{
		env:     e,
		opts:    opts,
		perf:    p,
		bench:   perf.NewBench(p, opts.BenchExit),
		router:  router,
		sidebar: bar.NewSidebar(cfg == nil || cfg.SidebarOpen),
		home:    home.New(e),
		library: lib,
		detail: game.New(e, game.Links{
			SearchLibrary:  func(q string) { lib.SearchFor(q); router.Push(env.PathLibrary) },
			FilterByTag:    func(tag string) { lib.FilterByTag(tag); router.Push(env.PathLibrary) },
			LibraryChanged: lib.Reload,
		}),
		categories: categories.New(e),
		category:   category.New(e),
		settings:   settings.New(e),
		downloads:  downloads.New(e, lib.Reload),
		stats:      stats.New(e),
		scale:      1,
		showHUD:    opts.HUD,
	}
	e.SidebarOpen = func() bool { return a.sidebar.Open }
	e.ConfigSaved = func(prev, next appconf.AppConfig) {
		applyConfig(prev, next)
		if opts.ConfigSaved != nil {
			opts.ConfigSaved(prev, next)
		}
	}
	return a
}

// applyConfig 让保存后的配置立即生效，对应原版 App.tsx 的 i18n.changeLanguage 与 useAppTheme。
// 缩放（window_zoom_factor）没有对应：原生界面没有页面缩放。
func applyConfig(prev, next appconf.AppConfig) {
	if next.Language != prev.Language {
		env.ApplyLanguage(next.Language)
	}
	if next.Theme != prev.Theme {
		env.ApplyTheme(next.Theme)
	}
}

// Attach 在窗口创建后调用：开始加载数据与封面，订阅应用级事件，并跟踪最大化状态与缩放。
func (a *App) Attach(win *mygo.Window) {
	a.env.Win = win
	a.subscribe()
	a.env.Covers.Start(win, a.env.Backend.DataDir())
	a.maximised = win.IsMaximized()
	a.syncScale()
	sync := func() { a.maximised = win.IsMaximized(); a.syncScale(); win.Invalidate() }
	win.OnMaximize(sync)
	win.OnUnmaximize(sync)
	win.OnRestore(sync)
	win.OnMove(func() { a.syncScale(); win.Invalidate() })
	go a.perf.SampleMemory(a.hudOn.Load, win.Invalidate)
	a.library.Reload()
	if a.opts.Bench {
		a.bench.Arm()
	}
}

func (a *App) syncScale() {
	d := mygo.Screen.DisplayMatching(a.env.Win.Bounds())
	if d.ScaleFactor > 0 {
		a.scale = d.ScaleFactor
	}
}

// SetScale 供测试在没有窗口时指定缩放。
func (a *App) SetScale(scale float64) { a.scale = scale }

// View 构建整个窗口，对应 routes/__root.tsx：顶栏，下面是侧边栏与路由出口。
func (a *App) View(c *ui.Context) {
	start := time.Now()
	t := theme.New(c.Theme().Dark, a.scale)
	a.env.Theme = t
	applyTheme(c, t)

	if c.Shortcut(0, ui.KeyF2) {
		a.showHUD = !a.showHUD
	}
	a.hudOn.Store(a.showHUD)
	if c.Shortcut(0, ui.KeyF5) {
		a.bench.Arm()
	}
	total, _ := a.library.Stats()
	a.bench.Step(c, perf.Target{Scroll: &a.library.Scroll().State, Ready: !a.library.Loading(), Items: total})

	winW, _ := c.Size()
	c.Root().Background(t.AppBg).TextColor(t.Text)
	ui.Column(c).Fill().Children(func() {
		win := a.env.Win
		bar.TopBar(c, t, bar.TopBarProps{
			Maximised:  a.maximised,
			OnMinimise: func() { win.Minimize() },
			OnMaximise: func() { win.ToggleMaximize() },
			OnClose:    func() { win.Close() },
		})
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			bar.SideBar(c, t, bar.SideBarProps{State: &a.sidebar, Router: a.router, Config: a.env.Config()})
			a.env.MainWidth = winW - a.sidebar.Width()
			// <main>：flex-1 overflow-hidden
			a.routes(c).Basis(0).Clip().Background(t.AppBg)
		})
	})
	for _, n := range a.notices {
		a.env.Toast(c, n)
	}
	a.notices = nil
	a.env.FlushToasts(c)
	// <AppToaster topOffset={(TOPBAR_HEIGHT + 12) * zoomFactor} />
	components.Toaster(c, t, env.TopbarHeight+12)
	if a.showHUD {
		a.hud(c)
	}
	a.perf.ViewCalled(time.Now(), time.Since(start))
}

// applyTheme 让 MyGo 自带控件（文本框光标、选区、提示等）跟随原版配色。
func applyTheme(c *ui.Context, t *theme.Theme) {
	mt := *c.Theme()
	mt.Accent = theme.Neutral600
	mt.Text = t.Text
	mt.TextMuted = t.Placeholder
	mt.Background = t.AppBg
	mt.Scrollbar = ui.Transparent // 滚动条由 components.ScrollArea 自己画，与 ::-webkit-scrollbar 一致
	mt.ScrollbarWidth = 0
	mt.Inverse, mt.InverseText = t.TooltipBg, t.TooltipText
	mt.FontSize = theme.TextBase.Size
	if theme.UIFont != "" {
		mt.Font = theme.UIFont
	}
	c.SetTheme(&mt)
}

func (a *App) hud(c *ui.Context) {
	total, loaded := a.library.Stats()
	n, avg := a.env.Covers.Decoded()
	perf.HUD(c, a.perf, a.bench, []string{
		fmt.Sprintf("route %s   scale %.2f", a.router.Location(), a.scale),
		fmt.Sprintf("games %d   loaded %d   covers %d decoded, %d pending", total, loaded, n, a.env.Covers.Pending()),
		fmt.Sprintf("cover load avg %.1f ms", float64(avg.Microseconds())/1000),
	})
}

// Screenshot 等可见封面解码完成后截图并退出，用于和原版逐像素对比（tools/parity）。
func (a *App) Screenshot(path string) {
	time.Sleep(800 * time.Millisecond)
	for range 60 {
		pending := make(chan int, 1)
		a.env.Win.Update(func() {
			n := a.env.Covers.Pending()
			if a.home.Loading() || a.library.Loading() || a.detail.Loading() || a.categories.Loading() || a.category.Loading() || a.settings.Loading() {
				n++
			}
			pending <- n
		})
		if <-pending == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(400 * time.Millisecond)
	data, err := a.env.Win.CapturePage()
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		log.Printf("screenshot: %v", err)
	}
	mygo.App.Quit()
}

// SetDebugState 让截图直接进入某个界面状态：drawer、addmenu、batch、collapsed，picker（首页展开封面选择面板），
// tab:<key>（详情页的标签页：stats、edit、launch、backup、progress、review），
// 或 settings:<分区>（设置页只展开这个分区并滚到顶部，settings:collapsed 全部收起）。
func (a *App) SetDebugState(state string) {
	switch {
	case state == "picker":
		a.home.SetDebugState(state)
	case strings.HasPrefix(state, "settings:"):
		a.settings.SetDebugState(strings.TrimPrefix(state, "settings:"))
	case strings.HasPrefix(state, "tab:"):
		a.detail.SetTab(strings.TrimPrefix(state, "tab:"))
	case state == "collapsed":
		a.sidebar.SetOpen(false)
	default:
		a.library.SetDebugState(state)
	}
}
