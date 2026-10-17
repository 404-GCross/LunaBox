package bar

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

const (
	sidebarOpenWidth   = 256 // 16rem
	sidebarClosedWidth = 64  // 4rem
)

// Sidebar 是侧边栏的展开状态。
type Sidebar struct {
	Open        bool
	delayedOpen bool      // 底部按钮的布局，在淡出一半时才切换
	fadeAt      time.Time // 切换开始的时间
	toggled     bool      // 刚切换，下一次构建时以 c.Now() 作为 fadeAt
	width       float32   // 当前动画中的宽度
}

// NewSidebar 按配置的 sidebar_open 创建状态。
func NewSidebar(open bool) Sidebar {
	return Sidebar{Open: open, delayedOpen: open, width: sidebarWidth(open)}
}

// Toggle 展开或收起。
func (s *Sidebar) Toggle() {
	s.Open = !s.Open
	s.toggled = true
}

// SetOpen 立即切换到目标状态，不播放动画（截图调试用）。
func (s *Sidebar) SetOpen(open bool) {
	s.Open, s.delayedOpen, s.fadeAt, s.toggled, s.width = open, open, time.Time{}, false, sidebarWidth(open)
}

// Width 是当前帧（动画中）的宽度，主内容区据此计算网格列数。
func (s *Sidebar) Width() float32 {
	if s.width == 0 {
		return sidebarWidth(s.Open)
	}
	return s.width
}

func sidebarWidth(open bool) float32 {
	if open {
		return sidebarOpenWidth
	}
	return sidebarClosedWidth
}

type navItem struct {
	path  string
	label func() string // 每帧按当前语言取
	icon  string
}

var navItems = []navItem{
	{env.PathHome, locales.SideBarHome, "home"},
	{env.PathLibrary, locales.SideBarLibrary, "gamepad-variant"},
	{env.PathStats, locales.SideBarStats, "chart-bar"},
	{env.PathCategories, locales.SideBarCategories, "format-list-bulleted"},
}

// SideBarProps 是侧边栏的输入。导航用 Router：当前路径决定高亮，点击 Push 对应路径。
type SideBarProps struct {
	State  *Sidebar
	Router *ui.Router
	Config *appconf.AppConfig // 云同步状态，可为 nil
}

// SideBar 对应 SideBar.tsx（未启用背景图时的样式）。
func SideBar(c *ui.Context, t *theme.Theme, p SideBarProps) {
	s := p.State
	now := c.Now()

	aside := ui.Column(c).Shrink(0).Background(t.SidebarBg).BorderWidth(0, t.HL, 0, 0).BorderColor(t.SidebarBorder)
	// style={{ transition: "width 300ms ease" }}
	s.width = aside.AnimateWith("width", sidebarWidth(s.Open), 300*time.Millisecond, theme.EaseCSS)
	aside.Width(s.width)

	// transition-all duration-300 的标签/标题，展开程度 0..1
	reveal := aside.AnimateWith("reveal", theme.Bool(s.Open), 300*time.Millisecond, theme.EaseTailwind)

	// 底部按钮：先淡出 150ms，再切换布局并淡入（SideBar.tsx 的 isFading/delayedOpen）。
	fadeOpacity := float32(1)
	if s.toggled {
		s.fadeAt, s.toggled = now, false
	}
	if !s.fadeAt.IsZero() {
		since := now.Sub(s.fadeAt)
		if since < 150*time.Millisecond {
			fadeOpacity = 0
			c.After(150*time.Millisecond - since)
		} else {
			s.delayedOpen = s.Open
			s.fadeAt = time.Time{}
		}
	}

	aside.Children(func() {
		sidebarHeader(c, t, s, reveal)
		ui.Column(c).Grow(1).Padding(16, 8).Gap(8).Children(func() {
			for _, item := range navItems {
				navLink(c, t, p.Router, item, reveal)
			}
		})
		sidebarFooter(c, t, p, fadeOpacity)
	})
}

func sidebarHeader(c *ui.Context, t *theme.Theme, s *Sidebar, reveal float32) {
	ui.Row(c).Height(64).PaddingX(12).Shrink(0).Children(func() {
		title := assets.Title(t.Dark)
		tw, th := title.Size()
		// opacity-100 max-w-[200px] pl-1 ↔ opacity-0 max-w-0 pl-0
		ui.Row(c).Gap(4).Clip().MaxWidth(200*reveal).Padding(0, 0, 0, 4*reveal).Opacity(reveal).Shrink(1).Children(func() {
			ui.Image(c, assets.AppIcon(t.Dark)).Size(32, 32).Shrink(0)
			ui.Image(c, title).Size(24*float32(tw)/float32(th), 24).Shrink(0)
		})
		justify := ui.Center
		if s.Open {
			justify = ui.End
		}
		ui.Row(c).Grow(1).MinWidth(40).Justify(justify).Children(func() {
			btn := better.IconButton(c, t, "toggle", "menu", 8, false, true)
			btn.Label(locales.SideBarToggle()).OnClick(s.Toggle)
		})
	})
}

// active 与 TanStack Router 的 activeProps 一致：完整匹配，或在其下的子路径（首页只完整匹配）。
func active(r *ui.Router, path string) bool {
	cur := r.Path()
	if path == env.PathHome {
		return cur == path
	}
	return cur == path || len(cur) > len(path) && cur[:len(path)] == path && cur[len(path)] == '/'
}

func navLink(c *ui.Context, t *theme.Theme, r *ui.Router, item navItem, reveal float32) {
	on := active(r, item.path)
	label := item.label()
	link := ui.ButtonBase(c.Key(item.path)).Padding(8).Radius(12).AlignItems(ui.Center).
		Justify(ui.Start).FocusRing(false).Label(label)
	color := t.NavText
	switch {
	case on:
		link.Background(t.NavActiveBg)
		color = t.NavActive
	case link.Hovered():
		link.Background(t.NavHover)
	}
	link.Transition(theme.ColorsTransition(150 * time.Millisecond))
	color = link.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	link.OnClick(func() { r.Push(item.path) }).Children(func() {
		ui.Box(c).Size(32, 32).Center().Shrink(0).Children(func() {
			components.Icon(c, item.icon, 20, color)
		})
		// w-[120px] ml-2 opacity-100 ↔ w-0 ml-0 opacity-0
		ui.Row(c).Width(120*reveal).Margin(0, 0, 0, 8*reveal).Opacity(reveal).Clip().Shrink(0).Children(func() {
			components.Text(c, label, theme.TextBase, theme.WeightNormal, color).NoWrap().Shrink(0)
		})
	})
}

func sidebarFooter(c *ui.Context, t *theme.Theme, p SideBarProps, opacity float32) {
	ui.Box(c).Padding(16).Shrink(0).Children(func() {
		var row ui.Element
		if p.State.delayedOpen {
			row = ui.Row(c).Justify(ui.End).Gap(4)
		} else {
			row = ui.Column(c).AlignItems(ui.Center).Gap(8)
		}
		op := row.AnimateWith("fade", opacity, 150*time.Millisecond, theme.EaseTailwind)
		row.Opacity(op).Children(func() {
			cloudSyncButton(c, t, p)

			dl := better.IconButton(c, t, "downloads", "download", 10, active(p.Router, env.PathDownloads), true)
			dl.Label(locales.DownloadsTitle()).OnClick(func() { p.Router.Push(env.PathDownloads) })

			st := better.IconButton(c, t, "settings", "cog", 10, active(p.Router, env.PathSettings), true)
			st.Label(locales.SettingsTitle()).OnClick(func() { p.Router.Push(env.PathSettings) })
		})
	})
}

// cloudSyncButton 对应侧边栏的云同步按钮与悬停面板。原型不执行同步。
func cloudSyncButton(c *ui.Context, t *theme.Theme, p SideBarProps) {
	cfg := p.Config
	serviceOn := cfg != nil && cfg.CloudBackupEnabled
	syncOn := serviceOn && cfg.CloudSyncEnabled

	name, color := "cloud-sync-outline", t.NavText
	switch {
	case !serviceOn:
		name, color = "cloud-off-outline", t.TextFaint
	case !syncOn:
		name, color = "cloud-outline", t.TextFaint
	case cfg.LastCloudSyncStatus == "success":
		name, color = "cloud-check-outline", theme.Success500
	case cfg.LastCloudSyncStatus == "failed":
		name, color = "cloud-alert-outline", theme.Error500
	}

	btn := ui.ButtonBase(c.Key("cloud")).Padding(10).Radius(12).Center().FocusRing(false).Label(locales.SideBarCloudSync())
	if syncOn && btn.Hovered() {
		btn.Background(t.NavHover)
	}
	// canSyncNow 为假（原型不同步）时 opacity-75
	btn.Opacity(0.75).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Icon(c, name, 20, color)
	})
	if !syncOn {
		return
	}
	show := btn.AnimateWith("panel", theme.Bool(btn.Hovered()), 200*time.Millisecond, theme.EaseTailwind)
	if show == 0 {
		return
	}
	ui.Overlay(c, func() {
		panel := ui.Column(c).Width(176).Padding(10).Gap(8).Radius(8).
			Background(t.Pick(theme.White.Alpha(0.92), theme.Brand900.Alpha(0.88))).
			Border(t.HL, t.Pick(theme.Brand200.Alpha(0.8), theme.Brand700.Alpha(0.8))).
			Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1)).
			Opacity(show).PassThrough()
		if p.State.Open {
			// bottom-full left-1/2 mb-3 -translate-x-1/2，悬停时 translate-y-2 → 0
			panel.AttachTo(btn, ui.AnchorTop, ui.AnchorBottom).Translate(0, -12+8*(1-show))
		} else {
			panel.AttachTo(btn, ui.AnchorBottomRight, ui.AnchorBottomLeft).Translate(12+8*(1-show), 0)
		}
		panel.Children(func() {
			components.Text(c, locales.SideBarCloudSync(), theme.FontStep{Size: 10, Line: 15}, theme.WeightMedium, t.TextMuted).NoWrap()
			label, bg, fg := cloudStatus(cfg.LastCloudSyncStatus, t)
			ui.Row(c).Children(func() {
				components.Text(c, label, theme.FontStep{Size: 9, Line: 13.5}, theme.WeightSemibold, fg).Padding(2, 6).Radius(999).Background(bg)
			})
			ui.Column(c).Gap(4).Children(func() {
				components.Text(c, locales.SettingsCloudBackupSyncLastTimeLabel(), theme.FontStep{Size: 10, Line: 15}, theme.WeightNormal, t.TextFaint).NoWrap()
				when := formatCloudTime(cfg.LastCloudSyncTime, timefmt.Location(cfg))
				components.Text(c, when, theme.FontStep{Size: 10, Line: 15}, theme.WeightMedium, t.Text).NoWrap()
			})
		})
	})
}

// cloudStatus 对应 utils/cloudSync.ts 的 getCloudSyncStatusLabel 与 getCloudSyncStatusClass。
func cloudStatus(status string, t *theme.Theme) (label string, bg, fg ui.Color) {
	switch status {
	case "success":
		return locales.SettingsCloudBackupSyncStatusSuccess(), t.Pick(ui.Hex("#d1fae5"), ui.Hex("#064e3b").Alpha(0.4)), t.Pick(ui.Hex("#047857"), ui.Hex("#6ee7b7"))
	case "failed":
		return locales.SettingsCloudBackupSyncStatusFailed(), t.Pick(ui.Hex("#fee2e2"), ui.Hex("#7f1d1d").Alpha(0.4)), t.Pick(ui.Hex("#b91c1c"), ui.Hex("#fca5a5"))
	}
	return locales.SettingsCloudBackupSyncStatusIdle(), t.Pick(theme.Brand100, theme.Brand700.Alpha(0.7)), t.Pick(theme.Brand700, theme.Brand200)
}

// formatCloudTime 对应 formatCloudSyncTime：没有同步过时是 syncNever，否则是不带秒的本地日期时间。
func formatCloudTime(s string, loc *time.Location) string {
	if s == "" {
		return locales.SettingsCloudBackupSyncNever()
	}
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return timefmt.DateTimeMinutes(ts, loc)
}
