package ui

import (
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/internal/appconf"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
)

// Backend 是界面用到的后端能力，由 internal/service 的真实服务实现。
type Backend interface {
	GetGames(req vo.GameListRequest) (vo.GameListResponse, error)
	SearchTagsInLibrary(query string) ([]string, error)
	ListGameFilterPresets() ([]models.GameFilterPreset, error)
	Config() *appconf.AppConfig
	DataDir() string
}

// Options 是原型的调试开关。
type Options struct {
	HUD       bool
	Bench     bool // 加载后自动滚动压测
	BenchExit bool
}

type Page int

const (
	PageHome Page = iota
	PageLibrary
	PageStats
	PageCategories
	PageDownloads
	PageSettings
)

// App 是整个窗口的状态，View 由它构建。
type App struct {
	backend Backend
	opts    Options
	win     *mygo.Window
	assets  *assets
	covers  *coverCache
	perf    *perf

	p     palette
	scale float64 // 窗口所在显示器的缩放比例
	hl    float32 // 一个物理像素的 DIP 宽度

	maximised bool
	page      Page
	sidebar   sidebarState
	lib       *library

	showHUD bool
	hudOn   atomic.Bool
	bench   benchState
}

func New(backend Backend, opts Options) *App {
	cfg := backend.Config()
	a := &App{
		backend: backend,
		opts:    opts,
		assets:  loadAssets(),
		covers:  newCoverCache(),
		perf:    newPerf(),
		scale:   1,
		page:    PageLibrary,
		showHUD: opts.HUD,
	}
	a.sidebar.open = cfg == nil || cfg.SidebarOpen
	a.sidebar.delayedOpen = a.sidebar.open
	a.lib = newLibrary(a)
	return a
}

// Attach 在窗口创建后调用：开始加载数据与封面，并跟踪最大化状态与缩放。
func (a *App) Attach(win *mygo.Window) {
	a.win = win
	a.covers.start(win, a.backend.DataDir())
	a.maximised = win.IsMaximized()
	a.syncScale()
	sync := func() { a.maximised = win.IsMaximized(); a.syncScale(); win.Invalidate() }
	win.OnMaximize(sync)
	win.OnUnmaximize(sync)
	win.OnRestore(sync)
	win.OnMove(func() { a.syncScale(); win.Invalidate() })
	go a.perf.sampleMemory(a.hudOn.Load, win.Invalidate)
	a.lib.reload()
	if a.opts.Bench {
		a.bench.arm()
	}
}

func (a *App) syncScale() {
	d := mygo.Screen.DisplayMatching(a.win.Bounds())
	if d.ScaleFactor > 0 {
		a.scale = d.ScaleFactor
	}
	a.hl = hairline(a.scale)
}

// SetScale 供测试在没有窗口时指定缩放。
func (a *App) SetScale(scale float64) {
	a.scale = scale
	a.hl = hairline(scale)
}

// View 构建整个窗口，对应 routes/__root.tsx。
func (a *App) View(c *ui.Context) {
	start := time.Now()
	if a.hl == 0 {
		a.hl = hairline(a.scale)
	}
	a.p = newPalette(c.Theme().Dark)
	a.applyTheme(c)

	if c.Shortcut(0, ui.KeyF2) {
		a.showHUD = !a.showHUD
	}
	a.hudOn.Store(a.showHUD)
	if c.Shortcut(0, ui.KeyF5) {
		a.bench.arm()
	}
	a.bench.step(c, a)

	c.Root().Background(a.p.appBg).TextColor(a.p.text)
	ui.Column(c).Fill().Children(func() {
		a.topBar(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.sideBar(c)
			main := ui.Box(c).Grow(1).Basis(0).Clip().Background(a.p.appBg)
			main.Children(func() {
				switch a.page {
				case PageLibrary:
					a.lib.view(c)
				default:
					a.placeholderPage(c)
				}
			})
		})
	})
	a.lib.overlays(c)
	if a.showHUD {
		a.hud(c)
	}
	a.perf.viewCalled(time.Now(), time.Since(start))
}

// uiFont 是 CSS system-ui 在本机对应的字体。
var uiFont = systemUIFont()

// applyTheme 让 MyGo 自带控件（文本框光标、选区、提示等）跟随原版配色。
func (a *App) applyTheme(c *ui.Context) {
	t := *c.Theme()
	t.Accent = neutral600
	t.Text = a.p.text
	t.TextMuted = a.p.placeholder
	t.Background = a.p.appBg
	t.Scrollbar = ui.Transparent // 滚动条由 scrollArea 自己画，与 ::-webkit-scrollbar 一致
	t.ScrollbarWidth = 0
	t.Inverse, t.InverseText = a.p.tooltipBg, a.p.tooltipText
	t.FontSize = textBase.size
	if uiFont != "" {
		t.Font = uiFont
	}
	c.SetTheme(&t)
}

func (a *App) placeholderPage(c *ui.Context) {
	ui.Column(c).Fill().Center().Gap(8).Children(func() {
		text(c, "此页面未迁移到原型", textXL, weightBold, a.p.textStrong)
		text(c, "原型只包含顶栏、侧边栏和游戏库页面", textSM, weightNormal, a.p.textMuted)
	})
}

// toast 显示提示；原版用 react-hot-toast，这里用 MyGo 的。
func (a *App) toast(c *ui.Context, msg string) { c.Toast(msg) }

// Screenshot 等可见封面解码完成后截图并退出，用于和原版逐像素对比。
func (a *App) Screenshot(path string) {
	time.Sleep(800 * time.Millisecond)
	for range 60 {
		pending := make(chan int, 1)
		a.win.Update(func() { pending <- len(a.covers.pending) + boolInt(a.lib.loading) })
		if <-pending == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(400 * time.Millisecond)
	data, err := a.win.CapturePage()
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		log.Printf("screenshot: %v", err)
	}
	mygo.App.Quit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func boolf(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// SetDebugState 让截图直接进入某个界面状态：drawer、addmenu、batch、collapsed。
func (a *App) SetDebugState(state string) {
	switch state {
	case "drawer":
		a.lib.drawer.open = true
	case "addmenu":
		a.lib.addMenuOpen = true
	case "batch":
		a.lib.batch = true
	case "collapsed":
		a.sidebar.open, a.sidebar.delayedOpen = false, false
	}
}
