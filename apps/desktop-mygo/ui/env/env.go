// Package env 是页面与布局共享的运行环境：后端、窗口、路由、封面缓存和当前帧的主题。
package env

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/appconf"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
)

// Backend 是界面用到的后端能力，由 apps/desktop/internal/service 的真实服务实现（见 backend.go）。
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
	PathDownloads  = "/downloads"
	PathSettings   = "/settings"
)

// TopbarHeight 对应 TopBar.tsx 的 TOPBAR_HEIGHT。
const TopbarHeight = 28

// Env 由 ui.App 创建，各页面持有同一个指针。
type Env struct {
	Backend Backend
	Win     *mygo.Window // 测试时为 nil
	Router  *ui.Router
	Covers  *covers.Cache

	// 以下由 App.View 在每帧开始时更新。
	Theme     *theme.Theme
	MainWidth float32 // 主内容区（窗口减去侧边栏）的宽度，随侧边栏动画逐帧变化
}

// Config 返回应用配置，可能为 nil。
func (e *Env) Config() *appconf.AppConfig { return e.Backend.Config() }

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
