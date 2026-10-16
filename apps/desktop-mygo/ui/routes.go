package ui

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// pendingPages 是原版有、还没迁移的页面；标题是函数，每帧按当前语言取。
var pendingPages = []struct {
	path  string
	title func() string
}{
	{env.PathHome, locales.HomeTitle},
	{env.PathStats, locales.StatsTitle},
	{env.PathCategories, locales.CategoriesTitle},
	{env.PathDownloads, locales.DownloadsTitle},
	{env.PathSettings, locales.SettingsTitle},
}

// routes 是路由表，对应原版 frontend/src/routes/*.tsx。侧边栏用 Router.Push 切换页面；
// Alt+←/→ 与鼠标侧键前进后退由 ui.Router 处理。
func (a *App) routes(c *ui.Context) ui.Element {
	// 原版每次进入详情页都重新挂载组件、重新加载数据；一帧可能构建多次，只在位置变化时加载一次。
	loc := a.router.Location()
	entered := loc != a.location
	if entered {
		a.detail.Flush() // 离开详情页前补存未保存的修改
	}
	a.location = loc
	return a.router.View(c, func(r *ui.Route) {
		switch {
		case r.Match(env.PathLibrary):
			r.Title(locales.LibraryTitle())
			a.library.View(c)
		case r.Match(env.PathGame):
			if entered {
				a.detail.Open(r.Param("id"), c.Now())
			}
			r.Title(a.detail.Title())
			a.detail.View(c)
		default:
			for _, p := range pendingPages {
				if r.Match(p.path) {
					r.Title(p.title())
					a.placeholderPage(c, locales.NativeNotMigrated())
					return
				}
			}
			r.Title(locales.NativeNotFound())
			a.placeholderPage(c, locales.NativeNotFoundPage(r.Path()))
		}
	})
}

func (a *App) placeholderPage(c *ui.Context, title string) {
	t := a.env.Theme
	ui.Column(c).Fill().Center().Gap(8).Children(func() {
		components.Text(c, title, theme.TextXL, theme.WeightBold, t.TextStrong)
		components.Text(c, locales.NativeNotMigratedHint(), theme.TextSM, theme.WeightNormal, t.TextMuted)
	})
}
