package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	sidebarOpenWidth   = 256 // 16rem
	sidebarClosedWidth = 64  // 4rem
)

type sidebarState struct {
	open        bool
	delayedOpen bool      // 底部按钮的布局，在淡出一半时才切换
	fadeAt      time.Time // 切换开始的时间
	width       float32   // 当前动画中的宽度，网格用它计算列数
}

func (s *sidebarState) toggle(now time.Time) {
	s.open = !s.open
	s.fadeAt = now
}

type navItem struct {
	page  Page
	label string
	icon  string
}

var navItems = []navItem{
	{PageHome, "首页", "home"},
	{PageLibrary, "游戏库", "gamepad-variant"},
	{PageStats, "统计", "chart-bar"},
	{PageCategories, "收藏", "format-list-bulleted"},
}

// sideBar 对应 components/bar/SideBar.tsx（未启用背景图时的样式）。
func (a *App) sideBar(c *ui.Context) {
	p := a.p
	s := &a.sidebar
	now := c.Now()

	aside := ui.Column(c).Shrink(0).Background(p.sidebarBg).BorderWidth(0, a.hl, 0, 0).BorderColor(p.sidebarBorder)
	target := float32(sidebarClosedWidth)
	if s.open {
		target = sidebarOpenWidth
	}
	// style={{ transition: "width 300ms ease" }}
	s.width = aside.AnimateWith("width", target, 300*time.Millisecond, easeCSS)
	aside.Width(s.width)

	// transition-all duration-300 的标签/标题，展开程度 0..1
	reveal := aside.AnimateWith("reveal", boolf(s.open), 300*time.Millisecond, easeTailwind)

	// 底部按钮：先淡出 150ms，再切换布局并淡入（SideBar.tsx 的 isFading/delayedOpen）。
	fadeOpacity := float32(1)
	if !s.fadeAt.IsZero() {
		since := now.Sub(s.fadeAt)
		if since < 150*time.Millisecond {
			fadeOpacity = 0
			c.After(150*time.Millisecond - since)
		} else {
			s.delayedOpen = s.open
			s.fadeAt = time.Time{}
		}
	}

	aside.Children(func() {
		a.sidebarHeader(c, reveal)
		ui.Column(c).Grow(1).Padding(16, 8).Gap(8).Children(func() {
			for _, item := range navItems {
				a.navLink(c, item, reveal)
			}
		})
		a.sidebarFooter(c, fadeOpacity)
	})
}

func (a *App) sidebarHeader(c *ui.Context, reveal float32) {
	p := a.p
	ui.Row(c).Height(64).PaddingX(12).Shrink(0).Children(func() {
		appIcon, title := a.assets.appIcon, a.assets.titleLight
		if p.dark {
			appIcon, title = a.assets.appIconDark, a.assets.titleDark
		}
		tw, th := title.Size()
		// opacity-100 max-w-[200px] pl-1 ↔ opacity-0 max-w-0 pl-0
		ui.Row(c).Gap(4).Clip().MaxWidth(200*reveal).Padding(0, 0, 0, 4*reveal).Opacity(reveal).Shrink(1).Children(func() {
			ui.Image(c, appIcon).Size(32, 32).Shrink(0)
			ui.Image(c, title).Size(24*float32(tw)/float32(th), 24).Shrink(0)
		})
		justify := ui.Center
		if a.sidebar.open {
			justify = ui.End
		}
		ui.Row(c).Grow(1).MinWidth(40).Justify(justify).Children(func() {
			btn := a.roundedIconButton(c, "toggle", "menu", 8, false, true)
			btn.Label("切换侧边栏").OnClick(func() { a.sidebar.toggle(time.Now()) })
		})
	})
}

func (a *App) navLink(c *ui.Context, item navItem, reveal float32) {
	p := a.p
	active := a.page == item.page
	link := ui.ButtonBase(c.Key(item.label)).Padding(8).Radius(12).AlignItems(ui.Center).
		Justify(ui.Start).FocusRing(false).Label(item.label)
	color := p.navText
	switch {
	case active:
		link.Background(p.navActiveBg)
		color = p.navActive
	case link.Hovered():
		link.Background(p.navHover)
	}
	link.Transition(colorsTransition(150 * time.Millisecond))
	color = link.AnimateColor("text", color, ui.Animation{Duration: 150 * time.Millisecond, Ease: easeTailwind})
	link.OnClick(func() { a.page = item.page }).Children(func() {
		ui.Box(c).Size(32, 32).Center().Shrink(0).Children(func() {
			icon(c, a.icon(item.icon), 20, color)
		})
		// w-[120px] ml-2 opacity-100 ↔ w-0 ml-0 opacity-0
		ui.Row(c).Width(120*reveal).Margin(0, 0, 0, 8*reveal).Opacity(reveal).Clip().Shrink(0).Children(func() {
			text(c, item.label, textBase, weightNormal, color).NoWrap().Shrink(0)
		})
	})
}

// roundedIconButton：rounded-xl p-2/p-2.5 text-xl 的纯图标按钮，hover:bg-brand-100 dark:hover:bg-brand-700。
func (a *App) roundedIconButton(c *ui.Context, key, name string, pad float32, active, enabled bool) ui.Element {
	p := a.p
	btn := ui.ButtonBase(c.Key(key)).Padding(pad).Radius(12).Center().FocusRing(false)
	color := p.navText
	switch {
	case active:
		btn.Background(p.navActiveBg)
		color = p.navActive
	case enabled && btn.Hovered():
		btn.Background(p.navHover)
	}
	btn.Transition(colorsTransition(150 * time.Millisecond))
	color = btn.AnimateColor("text", color, ui.Animation{Duration: 150 * time.Millisecond, Ease: easeTailwind})
	btn.Children(func() { icon(c, a.icon(name), 20, color) })
	return btn
}

func (a *App) sidebarFooter(c *ui.Context, opacity float32) {
	p := a.p
	ui.Box(c).Padding(16).Shrink(0).Children(func() {
		var row ui.Element
		if a.sidebar.delayedOpen {
			row = ui.Row(c).Justify(ui.End).Gap(4)
		} else {
			row = ui.Column(c).AlignItems(ui.Center).Gap(8)
		}
		op := row.AnimateWith("fade", opacity, 150*time.Millisecond, easeTailwind)
		row.Opacity(op).Children(func() {
			a.cloudSyncButton(c)

			dl := a.roundedIconButton(c, "downloads", "download", 10, a.page == PageDownloads, true)
			dl.Label("下载").OnClick(func() { a.page = PageDownloads })

			st := a.roundedIconButton(c, "settings", "cog", 10, a.page == PageSettings, true)
			st.Label("设置").OnClick(func() { a.page = PageSettings })
		})
	})
	_ = p
}

// cloudSyncButton 对应侧边栏的云同步按钮与悬停面板。原型不执行同步。
func (a *App) cloudSyncButton(c *ui.Context) {
	p := a.p
	cfg := a.backend.Config()
	serviceOn := cfg != nil && cfg.CloudBackupEnabled
	syncOn := serviceOn && cfg.CloudSyncEnabled

	name, color := "cloud-sync-outline", p.navText
	switch {
	case !serviceOn:
		name, color = "cloud-off-outline", p.textFaint
	case !syncOn:
		name, color = "cloud-outline", p.textFaint
	case cfg.LastCloudSyncStatus == "success":
		name, color = "cloud-check-outline", success500
	case cfg.LastCloudSyncStatus == "failed":
		name, color = "cloud-alert-outline", error500
	}

	btn := ui.ButtonBase(c.Key("cloud")).Padding(10).Radius(12).Center().FocusRing(false).Label("云同步")
	if syncOn && btn.Hovered() {
		btn.Background(p.navHover)
	}
	// canSyncNow 为假（原型不同步）时 opacity-75
	btn.Opacity(0.75).Transition(colorsTransition(150 * time.Millisecond)).Children(func() {
		icon(c, a.icon(name), 20, color)
	})
	if !syncOn {
		return
	}
	show := btn.AnimateWith("panel", boolf(btn.Hovered()), 200*time.Millisecond, easeTailwind)
	if show == 0 {
		return
	}
	ui.Overlay(c, func() {
		panel := ui.Column(c).Width(176).Padding(10).Gap(8).Radius(8).
			Background(brand900.Alpha(0.88)).Border(a.hl, brand700.Alpha(0.8)).
			Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1)).
			Opacity(show).PassThrough()
		if !p.dark {
			panel.Background(white.Alpha(0.92)).BorderColor(brand200.Alpha(0.8))
		}
		if a.sidebar.open {
			// bottom-full left-1/2 mb-3 -translate-x-1/2，悬停时 translate-y-2 → 0
			panel.AttachTo(btn, ui.AnchorTop, ui.AnchorBottom).Translate(0, -12+8*(1-show))
		} else {
			panel.AttachTo(btn, ui.AnchorBottomRight, ui.AnchorBottomLeft).Translate(12+8*(1-show), 0)
		}
		panel.Children(func() {
			text(c, "云同步", fontStep{10, 15}, weightMedium, p.textMuted).NoWrap()
			label, bg, fg := cloudStatus(cfg.LastCloudSyncStatus, p.dark)
			ui.Row(c).Children(func() {
				text(c, label, fontStep{9, 13.5}, weightSemibold, fg).Padding(2, 6).Radius(999).Background(bg)
			})
			ui.Column(c).Gap(4).Children(func() {
				text(c, "上次同步", fontStep{10, 15}, weightNormal, p.textFaint).NoWrap()
				when := "从未"
				if t := cfg.LastCloudSyncTime; t != "" {
					when = formatCloudTime(t)
				}
				text(c, when, fontStep{10, 15}, weightMedium, p.text).NoWrap()
			})
		})
	})
}

func cloudStatus(status string, dark bool) (label string, bg, fg ui.Color) {
	switch status {
	case "success":
		label = "成功"
		bg, fg = ui.Hex("#064e3b").Alpha(0.4), ui.Hex("#6ee7b7")
		if !dark {
			bg, fg = ui.Hex("#d1fae5"), ui.Hex("#047857")
		}
	case "failed":
		label = "失败"
		bg, fg = ui.Hex("#7f1d1d").Alpha(0.4), ui.Hex("#fca5a5")
		if !dark {
			bg, fg = ui.Hex("#fee2e2"), ui.Hex("#b91c1c")
		}
	default:
		label = "未同步"
		bg, fg = brand700.Alpha(0.7), brand200
		if !dark {
			bg, fg = brand100, brand700
		}
	}
	return label, bg, fg
}

func formatCloudTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006/01/02 15:04:05")
}
