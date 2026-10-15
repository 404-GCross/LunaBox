package ui

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
)

// pendingPages 是原版有、原型还没迁移的页面。
var pendingPages = []struct{ path, title string }{
	{env.PathHome, "首页"},
	{env.PathStats, "统计"},
	{env.PathCategories, "收藏"},
	{env.PathDownloads, "下载"},
	{env.PathSettings, "设置"},
}

// routes 是路由表，对应原版 frontend/src/routes/*.tsx。侧边栏用 Router.Push 切换页面；
// Alt+←/→ 与鼠标侧键前进后退由 ui.Router 处理。
func (a *App) routes(c *ui.Context) ui.Element {
	// 原版每次进入详情页都重新挂载组件、重新加载数据；一帧可能构建多次，只在位置变化时加载一次。
	loc := a.router.Location()
	entered := loc != a.location
	a.location = loc
	return a.router.View(c, func(r *ui.Route) {
		switch {
		case r.Match(env.PathLibrary):
			r.Title("游戏库")
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
					r.Title(p.title)
					a.placeholderPage(c, "此页面未迁移到原型")
					return
				}
			}
			r.Title("未找到")
			a.placeholderPage(c, "未找到页面 "+r.Path())
		}
	})
}

func (a *App) placeholderPage(c *ui.Context, title string) {
	t := a.env.Theme
	ui.Column(c).Fill().Center().Gap(8).Children(func() {
		components.Text(c, title, theme.TextXL, theme.WeightBold, t.TextStrong)
		components.Text(c, "原型只包含顶栏、侧边栏、游戏库和游戏详情页面", theme.TextSM, theme.WeightNormal, t.TextMuted)
	})
}
