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

// Toast 显示提示；原版用 react-hot-toast，这里用 MyGo 的。
func (e *Env) Toast(c *ui.Context, msg string) { c.Toast(msg) }

// ApplyLanguage 对应 App.tsx 里的 i18n.changeLanguage(config.language)：显示配置里的语言（zh-CN、zh-TW、
// en-US、ja-JP）。为空或不是支持的语言时跟随系统的语言（原版的 LanguageDetector），都不匹配时是 zh-CN。
// 运行时设置 MYGO_LOCALE（-lang）时它优先，启动时这里的设置不生效。
func ApplyLanguage(lang string) {
	if !slices.Contains(mygo.App.Languages(), lang) {
		lang = ""
	}
	mygo.App.SetLocale(lang)
}
