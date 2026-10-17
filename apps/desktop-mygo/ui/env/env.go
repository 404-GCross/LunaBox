// Package env 是页面与布局共享的运行环境：后端、窗口、路由、封面缓存和当前帧的主题。
package env

import (
	"net/url"
	"slices"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/backend"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/appconf"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
)

// Backend 是游戏库列表用到的查询，由真实服务实现（见 main 包的 backend.go）；单独成接口是为了
// -repeat 压测时把列表放大，以及测试时用内存实现。其余页面直接用 Env.Svc 里的真实服务。
type Backend interface {
	GetGames(req vo.GameListRequest) (vo.GameListResponse, error)
	SearchTagsInLibrary(query string) ([]string, error)
	ListGameFilterPresets() ([]models.GameFilterPreset, error)
	Config() *appconf.AppConfig
	DataDir() string
}

// 路由路径，与原版 frontend/src/routes 一致。
const (
	PathHome       = "/"
	PathLibrary    = "/library"
	PathStats      = "/stats"
	PathCategories = "/categories"
	PathCategory   = "/categories/{id}" // 收藏详情，Route.Param("id") 是收藏夹 ID
	PathDownloads  = "/downloads"
	PathSettings   = "/settings"
	PathGame       = "/game/{id}" // 游戏详情，Route.Param("id") 是游戏 ID
)

// GamePath 返回游戏详情页的路径。
func GamePath(id string) string { return "/game/" + url.PathEscape(id) }

// CategoryPath 返回收藏详情页的路径。
func CategoryPath(id string) string { return "/categories/" + url.PathEscape(id) }

// TopbarHeight 对应 TopBar.tsx 的 TOPBAR_HEIGHT。
const TopbarHeight = 28

// Env 由 ui.App 创建，各页面持有同一个指针。
type Env struct {
	Backend Backend
	Svc     *backend.Services // apps/desktop 的真实服务
	Win     *mygo.Window      // 测试时为 nil
	Router  *ui.Router
	Covers  *covers.Cache

	// SidebarOpen 报告侧边栏是否展开（store 的 isSidebarOpen）：保存配置时 sidebar_open 取它（withSidebarState）。
	// 测试里为 nil，保存时保留配置里的值。
	SidebarOpen func() bool
	// ConfigSaved 在设置页保存配置之后（主线程）调用，App 用它让语言与主题立即生效；测试里为 nil。
	ConfigSaved func(prev, next appconf.AppConfig)

	// 以下由 App.View 在每帧开始时更新。
	Theme     *theme.Theme
	MainWidth float32 // 主内容区（窗口减去侧边栏）的宽度，随侧边栏动画逐帧变化
}

// Config 返回应用配置，可能为 nil。
func (e *Env) Config() *appconf.AppConfig {
	if e.Svc != nil {
		return e.Svc.Config
	}
	return e.Backend.Config()
}

// Async 在后台执行 work，再把它返回的函数放回主线程执行；没有窗口（测试）时同步执行。
func (e *Env) Async(work func() func()) {
	if e.Win == nil {
		work()()
		return
	}
	go func() {
		apply := work()
		e.Win.Update(apply)
	}()
}

// Notice 是一条提示，对应 react-hot-toast 的 toast()、toast.success()、toast.error()：
// Type 决定 components.Toaster 里的图标与配色。
type Notice struct {
	Type string // ""（普通）、"success"、"error"
	Msg  string
}

// Info 对应 toast(msg)。
func Info(msg string) Notice { return Notice{Msg: msg} }

// Success 对应 toast.success(msg)。
func Success(msg string) Notice { return Notice{Type: "success", Msg: msg} }

// Error 对应 toast.error(msg)。
func Error(msg string) Notice { return Notice{Type: "error", Msg: msg} }

// Toast 显示提示，样式由 components.Toaster 按原版 AppToaster 绘制。相同文字的提示正在显示时重新显示它。
func (e *Env) Toast(c *ui.Context, n Notice) {
	c.AddToast(ui.Toast{ID: n.Msg, Title: n.Msg, Type: n.Type})
}

// ApplyTheme 对应 hooks/useAppTheme.ts 按 config.theme 切换 dark 类：light、dark 强制浅色或深色，其余跟随系统。
func ApplyTheme(theme string) {
	switch theme {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// ApplyLanguage 对应 App.tsx 里的 i18n.changeLanguage(config.language)：显示配置里的语言（zh-CN、zh-TW、
// en-US、ja-JP）。为空或不是支持的语言时跟随系统的语言（原版的 LanguageDetector），都不匹配时是 zh-CN。
// 运行时设置 MYGO_LOCALE（-lang）时它优先，启动时这里的设置不生效。
func ApplyLanguage(lang string) {
	if !slices.Contains(mygo.App.Languages(), lang) {
		lang = ""
	}
	mygo.App.SetLocale(lang)
}
