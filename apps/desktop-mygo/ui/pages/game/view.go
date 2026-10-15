package game

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/apputils"
)

const (
	pagePadding = 32   // p-8
	contentMaxW = 1408 // max-w-8xl
	coverW      = 240  // grid-cols-[15rem_minmax(0,1fr)]
	headerGap   = 24   // gap-6
	toolbarGap  = 12   // gap-3
)

// View 构建页面。滚动容器是原版的 <main>（overflow-auto）：内容超出时才出现 9px 的滚动条槽。
func (p *Page) View(c *ui.Context) {
	t := p.env.Theme
	switch {
	case p.loading && p.game == nil:
		if wait := skeletonDelay - c.Now().Sub(p.openedAt); wait > 0 {
			c.After(wait)
			return
		}
		p.skeleton(c, t)
		return
	case p.game == nil:
		p.notFound(c, t)
		return
	}

	gutter := float32(0)
	if p.scroll.Overflowing() {
		gutter = t.Gutter()
	}
	contentW := min(contentMaxW, p.env.MainWidth-2*pagePadding-gutter)
	components.ScrollArea(c, t, "game-scroll", &p.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding)
		sc.Children(func() {
			// space-y-8 max-w-8xl mx-auto
			ui.Column(c).Width(contentW).Margin(0, ui.Auto).Gap(32).Children(func() {
				p.backButton(c, t)
				p.header(c, t, contentW)
				p.tabBar(c, t)
				if p.tab == "stats" {
					p.statsView(c, t)
				} else {
					ui.Column(c).Padding(48, 0).AlignItems(ui.Center).Children(func() {
						components.Text(c, "此标签页未迁移到原型", theme.TextSM, theme.WeightNormal, t.TextMuted)
					})
				}
			})
		})
	})
}

// backButton：flex items-center rounded-md text-brand-750 hover:text-brand-900，i-mdi-arrow-left text-2xl mr-1。
func (p *Page) backButton(c *ui.Context, t *theme.Theme) {
	btn := ui.ButtonBase(c.Key("back")).AlignSelf(ui.Start).AlignItems(ui.Center).Radius(6).FocusRing(false)
	color := t.Pick(theme.Brand750, theme.Brand400)
	if btn.Hovered() {
		color = t.Pick(theme.Brand900, theme.Brand200)
	}
	color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	btn.OnClick(func() { p.env.Router.Back() }).Children(func() {
		components.Icon(c, "arrow-left", 24, color).Margin(0, 4, 0, 0)
		components.Text(c, "返回", theme.TextBase, theme.WeightNormal, color).NoWrap()
	})
}

// header：grid-cols-[15rem_minmax(0,1fr)] items-stretch gap-6，左侧封面在 min-h-64 的格子里垂直居中。
func (p *Page) header(c *ui.Context, t *theme.Theme, contentW float32) {
	g := p.game
	rightW := contentW - coverW - headerGap
	ui.Row(c.Key("header")).Gap(headerGap).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(coverW).MinHeight(256).Justify(ui.Center).Shrink(0).Children(func() {
			p.cover(c, t, g)
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(16).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				components.Text(c, g.Name, theme.Text4XL, theme.WeightBold, t.TextStrong)
				p.toolbar(c, t, rightW)
			})
			p.infoGrid(c, t)
			p.summary(c, t)
			components.GameTags(c, t, components.GameTagsProps{
				Tags:     p.tags,
				OnClick:  func(tag models.GameTag) { p.links.FilterByTag(tag.Name) },
				OnDelete: func(models.GameTag) { p.env.Toast(c, "原型不删除标签") },
				OnAdd:    func() { p.env.Toast(c, "原型未迁移标签编辑") },
			})
		})
	})
}

// cover 是 GameCoverImage（w-full h-auto rounded-lg），点击打开大图；没有封面时显示“没有封面”。
func (p *Page) cover(c *ui.Context, t *theme.Theme, g *models.Game) {
	bmp := p.env.Covers.Get(g)
	switch {
	case bmp != nil:
		w, h := bmp.Size()
		// cursor-zoom-in：MyGo 没有放大镜光标，用手形。
		btn := ui.ButtonBase(c.Key("cover")).Width(coverW).Height(coverW * float32(h) / float32(max(1, w))).
			Radius(8).Clip().FocusRing(false).Cursor(ui.CursorPointer).Label("查看 " + g.Name + " 的封面")
		btn.OnClick(func() { p.env.Toast(c, "原型未迁移图片查看器") }).Children(func() {
			ui.Image(c, bmp).Fill().Fit(ui.Cover)
		})
	case p.env.Covers.Missing(g):
		ui.Box(c).FillWidth().Grow(1).MinHeight(256).Center().Children(func() {
			components.Text(c, "没有封面", theme.TextBase, theme.WeightNormal, theme.Brand400)
		})
	}
}

// toolbar：relative flex flex-wrap items-center gap-3。放不下全部状态按钮时（requiredWidth + 4 > clientWidth）
// 改成状态下拉菜单；宽度按文字量出来，不用等布局。
func (p *Page) toolbar(c *ui.Context, t *theme.Theme, width float32) {
	g := p.game
	status := currentStatus(g)
	links := sourceLinks(g, erogameScapeBase(p.env))
	launch := launchOption(p.launchMode)

	launchW := 2*t.HL + 10 + 16 + 4 + textWidth(c, launch.Label) + 10 + 2*t.HL + 6 + 16 + 6
	statusW := float32(len(components.GameStatuses)-1)*(40+6) + 12 + 16 + 6 + textWidth(c, components.GameStatusLabel(status)) + 12
	actionsW := float32(1 + toolbarGap + 3*32 + 3*6 + 32)
	if len(links) > 1 {
		actionsW += 48 - 32
	}
	compact := launchW+statusW+actionsW+2*toolbarGap+4 > width

	ui.Row(c.Key("toolbar")).Wrap().GapX(toolbarGap).GapY(toolbarGap).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(toolbarGap).AlignItems(ui.Center).Shrink(0).Children(func() {
			components.SplitButton(c, t, "launch", components.SplitButtonProps{
				Label:       launch.Label,
				Icon:        launch.Icon,
				Options:     launchOptions,
				SelectedKey: string(p.launchMode),
				MenuTitle:   "临时启动方式",
				OnClick:     func() { p.env.Toast(c, "原型不启动游戏："+g.Name) },
				OnSelect:    func(key string) { p.launchMode = enums.LaunchMode(key) },
			})
			if compact {
				p.statusMenu(c, t, status)
			} else {
				p.statusButtons(c, t, status)
			}
		})
		ui.Row(c).Gap(toolbarGap).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Box(c).Size(1, 24).Background(t.Divider)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				p.sourceButton(c, t, links)
				dir := g.GameDirectory
				if dir == "" {
					dir = g.Path
				}
				actionButton(c, t, "explorer", "folder-open-outline", "在资源管理器中打开", dir == "", func() { p.openPath(c, dir) })
				actionButton(c, t, "guide", "text-box-search-outline", "打开说明文档", dir == "", func() { p.env.Toast(c, "原型未迁移说明文档") })
				actionButton(c, t, "category", "folder-plus-outline", "添加到收藏", false, func() { p.env.Toast(c, "原型未迁移收藏") })
			})
		})
	})
}

func erogameScapeBase(e *env.Env) string {
	if cfg := e.Config(); cfg != nil {
		return cfg.ErogameScapeBaseURL
	}
	return ""
}

// openPath 对应 OpenLocalPath：目录直接打开，文件在资源管理器中选中。
func (p *Page) openPath(c *ui.Context, path string) {
	p.env.Async(func() func() {
		err := apputils.OpenFileOrFolder(path)
		return func() {
			if err != nil {
				p.env.Toast(c, "打开路径失败")
			}
		}
	})
}

// statusButtons：flex gap-1.5 的状态按钮，只有当前状态显示文字；当前状态带 ring-2 ring-offset-1 ring-brand-400。
func (p *Page) statusButtons(c *ui.Context, t *theme.Theme, current enums.GameStatus) {
	ui.Row(c.Key("statuses")).Gap(6).Shrink(0).Children(func() {
		for _, s := range components.GameStatuses {
			label := components.GameStatusLabel(s)
			active := s == current
			btn := ui.ButtonBase(c.Key(string(s))).Height(32).Padding(0, 12).Radius(999).Gap(6).
				AlignItems(ui.Center).FocusRing(false).Label(label)
			icon, bg, fg := components.GameStatusStyle(t, s)
			if active {
				btn.Shadow(0, 0, 0, 1, t.Pick(theme.White, theme.Brand900)).Shadow(0, 0, 0, 3, theme.Brand400)
			} else {
				bg, fg = t.Pick(theme.Brand150, theme.Brand700), t.Pick(theme.Brand500, theme.Brand400)
				if btn.Hovered() {
					bg = t.Pick(theme.Brand200, theme.Brand600)
				}
			}
			btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
			btn.OnClick(func() { p.setStatus(s) }).Children(func() {
				components.Icon(c, icon, 16, fg)
				if active {
					components.Text(c, label, theme.TextXS, theme.WeightMedium, fg).NoWrap()
				}
			})
			components.Tooltip(c, t, btn, label)
		}
	})
}

// statusMenu 是窄屏时的状态下拉菜单（openOnHover align="start"）：触发器 h-8 rounded-full px-3 ring-1 ring-inset。
func (p *Page) statusMenu(c *ui.Context, t *theme.Theme, current enums.GameStatus) {
	label := components.GameStatusLabel(current)
	icon, bg, fg := components.GameStatusStyle(t, current)
	trigger := ui.ButtonBase(c.Key("status-menu")).Height(32).Padding(0, 12).Radius(999).Gap(6).AlignItems(ui.Center).
		Background(bg).InsetShadow(0, 0, 0, 1, theme.Brand400.Alpha(0.5)).FocusRing(false).Label("状态: " + label)
	trigger.Children(func() {
		components.Icon(c, icon, 16, fg)
		components.Text(c, label, theme.TextXS, theme.WeightMedium, fg).NoWrap()
		components.Icon(c, "chevron-down", 14, fg)
	})
	items := make([]components.MenuItem, 0, len(components.GameStatuses))
	for _, s := range components.GameStatuses {
		icon, bg, fg := components.GameStatusStyle(t, s)
		items = append(items, components.MenuItem{
			Key: string(s), Label: components.GameStatusLabel(s), Icon: icon,
			Pill: true, PillBg: bg, PillText: fg,
			OnClick: func() { p.setStatus(s) },
		})
	}
	components.DropdownMenu(c, t, trigger, components.MenuProps{
		Open: &p.statusOpen, Items: items, Title: "状态", SelectedKey: string(current),
		AlignStart: true, OpenOnHover: true,
	})
}

// setStatus 只改页面里的状态；原型的数据库是只读副本的临时拷贝，不写回。
func (p *Page) setStatus(s enums.GameStatus) {
	if p.game != nil {
		p.game.Status = s
	}
}

// sourceButton 是“打开数据源网页”：只有一个来源时是圆形按钮，多个来源时带下拉箭头并弹出来源列表。
func (p *Page) sourceButton(c *ui.Context, t *theme.Theme, links []metadataSource) {
	if len(links) <= 1 {
		url := ""
		if len(links) == 1 {
			url = links[0].url
		}
		actionButton(c, t, "source", "open-in-new", "打开数据源网页", url == "", func() { c.OpenURL(url) })
		return
	}
	// flex h-8 items-center gap-0.5 rounded-full px-2
	btn := actionBase(c, t, "source", false).Padding(0, 8).Gap(2).Label("选择要打开的数据源网页")
	fg := actionColor(t, btn.Hovered())
	btn.Children(func() {
		components.Icon(c, "open-in-new", 16, fg)
		components.Icon(c, "chevron-down", 14, fg)
	})
	components.Tooltip(c, t, btn, "选择要打开的数据源网页")
	items := make([]components.MenuItem, 0, len(links))
	for i, l := range links {
		desc := "条目 ID：" + l.id
		if i == 0 && l.source == p.game.SourceType {
			desc = "默认来源 · 条目 ID：" + l.id
		}
		items = append(items, components.MenuItem{
			Key: string(l.source), Label: sourceLabel(l.source), Description: desc,
			OnClick: func() { c.OpenURL(l.url) },
		})
	}
	components.DropdownMenu(c, t, btn, components.MenuProps{Items: items, Title: "打开数据源网页", MinWidth: 240, AlignStart: true})
}

// actionButton：h-8 w-8 rounded-full bg-brand-150 text-brand-500 的图标按钮，disabled:opacity-45。
func actionButton(c *ui.Context, t *theme.Theme, key, icon, label string, disabled bool, onClick func()) {
	btn := actionBase(c, t, key, disabled).Width(32).Label(label)
	fg := actionColor(t, btn.Hovered() && !disabled)
	if !disabled {
		btn.OnClick(onClick)
	}
	btn.Children(func() { components.Icon(c, icon, 16, fg) })
	components.Tooltip(c, t, btn, label)
}

func actionBase(c *ui.Context, t *theme.Theme, key string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Height(32).Radius(999).AlignItems(ui.Center).Justify(ui.Center).FocusRing(false)
	bg := t.Pick(theme.Brand150, theme.Brand700)
	if btn.Hovered() && !disabled {
		bg = t.Pick(theme.Brand200, theme.Brand600)
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		btn.Opacity(0.45).Disabled(true)
	}
	return btn
}

func actionColor(t *theme.Theme, hovered bool) ui.Color {
	if hovered {
		return t.Pick(theme.Brand900, theme.Brand100)
	}
	return t.Pick(theme.Brand500, theme.Brand400)
}

// infoGrid：grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4 text-sm（断点按窗口宽度）。
func (p *Page) infoGrid(c *ui.Context, t *theme.Theme) {
	g := p.game
	winW, _ := c.Size()
	cols := 2
	switch {
	case winW >= 1024:
		cols = 5
	case winW >= 768:
		cols = 3
	}
	color := t.Pick(theme.Brand750, theme.Brand400)
	rating, release := "-", strings.TrimSpace(g.ReleaseDate)
	if g.Rating > 0 {
		rating = fmt.Sprintf("%.1f / 10", g.Rating)
	}
	if release == "" {
		release = "-"
	}
	created := strings.ReplaceAll(timefmt.Date(g.CreatedAt, timefmt.Location(p.env.Config())), "/", "-")

	cell := func(label string, value func()) {
		ui.Column(c).MinWidth(0).Children(func() {
			components.TextMargin(c, label, theme.TextSM, theme.WeightSemibold, color, 0, 0, 4, 0)
			value()
		})
	}
	text := func(s string) func() {
		return func() { components.Text(c, s, theme.TextSM, theme.WeightNormal, color) }
	}
	ui.Grid(c.Key("info")).Columns(cols).GapX(16).GapY(16).Children(func() {
		cell("数据源", text(metadataSourceText(g)))
		cell("开发", func() {
			company := strings.TrimSpace(g.Company)
			if company == "" {
				components.Text(c, "-", theme.TextSM, theme.WeightNormal, color)
				return
			}
			btn := ui.ButtonBase(c.Key("company")).AlignSelf(ui.Start).Justify(ui.Start).FocusRing(false)
			btn.OnClick(func() { p.links.SearchLibrary(company) }).Children(func() {
				components.Text(c, g.Company, theme.TextSM, theme.WeightNormal, color)
			})
		})
		cell("添加时间", text(created))
		cell("评分", text(rating))
		cell("发售日期", text(release))
	})
}

// summary：font-semibold mb-2 的“简介”，正文 text-sm leading-relaxed whitespace-pre-wrap max-h-60 overflow-y-auto pr-2。
func (p *Page) summary(c *ui.Context, t *theme.Theme) {
	text := p.game.Summary
	if text == "" {
		text = "暂无简介"
	}
	ui.Column(c.Key("summary")).MinWidth(0).Children(func() {
		components.TextMargin(c, "简介", theme.TextBase, theme.WeightSemibold, t.TextStrong, 0, 0, 8, 0)
		ui.Scroll(c.Key("summary-scroll")).MaxHeight(240).Children(func() {
			components.Text(c, text, theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand750, theme.Brand400)).
				Margin(0, 8, 0, 0)
		})
	})
}

// tabBar：border-b 下的 nav（-mb-px space-x-8），按钮 py-4 px-1 border-b-2 text-sm font-medium。
func (p *Page) tabBar(c *ui.Context, t *theme.Theme) {
	bar := ui.Row(c.Key("tabs")).Gap(32)
	hl, line := t.HL, t.Divider
	// 底边框画在标签下面，选中标签的 2px 下划线盖住它（-mb-px）。
	bar.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - hl, W: r.W, H: hl}, line, 0)
	})
	bar.Children(func() {
		for _, tab := range tabs {
			btn := ui.ButtonBase(c.Key(tab.key)).Column().Padding(16, 0, 0, 0).FocusRing(false).Label(tab.label)
			active := tab.key == p.tab
			color, border := t.Pick(theme.Brand700, theme.Brand400), ui.Transparent
			switch {
			case active:
				color, border = t.Pick(theme.Brand700, theme.Neutral400), theme.Neutral500
			case btn.Hovered():
				color, border = t.Pick(theme.Brand750, theme.Brand300), theme.Brand300
			}
			btn.OnClick(func() { p.tab = tab.key }).Children(func() {
				components.TextMargin(c, tab.label, theme.TextSM, theme.WeightMedium, color, 0, 4, 16, 4).NoWrap()
				ui.Box(c).Height(2).FillWidth().Background(border)
			})
		}
	})
}

// skeleton 对应 GameDetailSkeleton（animate-pulse）。
func (p *Page) skeleton(c *ui.Context, t *theme.Theme) {
	bg := t.Pick(theme.Brand200, theme.Brand800)
	contentW := min(contentMaxW, p.env.MainWidth-2*pagePadding)
	wrap := ui.Column(c.Key("skeleton")).Fill().Clip().Padding(pagePadding).PassThrough()
	wrap.Opacity(components.Pulse(wrap)).Children(func() {
		ui.Column(c).Width(contentW).Margin(0, ui.Auto).Gap(32).Children(func() {
			ui.Box(c).Size(96, 32).Radius(6).Background(bg)
			ui.Row(c).Gap(24).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(240, 320).Radius(8).Background(bg).Shrink(0)
				ui.Column(c).Grow(1).Gap(16).Children(func() {
					ui.Box(c).Height(48).WidthPercent(75).Radius(8).Background(bg)
					ui.Grid(c).Columns(4).GapX(16).Children(func() {
						for range 4 {
							ui.Box(c).Height(48).Radius(6).Background(bg)
						}
					})
					ui.Box(c).Height(128).Radius(8).Background(bg)
				})
			})
			tabs := ui.Row(c).Gap(32)
			hl, line := t.HL, t.Divider
			tabs.Draw(func(pt *ui.Painter, r ui.Rect) {
				pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - hl, W: r.W, H: hl}, line, 0)
			})
			tabs.Children(func() {
				for range 3 {
					ui.Box(c).Size(80, 40).Radius(6, 6, 0, 0).Background(bg)
				}
			})
			ui.Grid(c).Columns(3).GapX(24).Children(func() {
				for range 3 {
					ui.Box(c).Height(96).Radius(8).Background(bg)
				}
			})
			ui.Box(c).Height(320).Radius(8).Background(bg)
		})
	})
}

// notFound：未找到游戏时的提示与“返回库”。
func (p *Page) notFound(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("not-found")).Fill().Center().Gap(16).Children(func() {
		components.Icon(c, "gamepad-variant-outline", 60, theme.Brand500)
		components.Text(c, "未找到该游戏", theme.TextXL, theme.WeightNormal, theme.Brand500)
		// text-neutral-600 hover:underline
		btn := ui.ButtonBase(c.Key("back-to-library")).FocusRing(false)
		btn.OnClick(func() { p.env.Router.Push(env.PathLibrary) }).Children(func() {
			text := components.Text(c, "返回库", theme.TextBase, theme.WeightNormal, theme.Neutral600)
			if btn.Hovered() {
				text.Underline()
			}
		})
	})
}

func textWidth(c *ui.Context, s string) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: s, Font: theme.UIFont, Size: theme.TextXS.Size, Weight: theme.WeightMedium})
	return w
}
