package library

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/bar"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/card"
	"lunabox/desktop-mygo/ui/components/grid"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/common/enums"
)

const (
	pagePadding  = 32   // p-8
	contentMaxW  = 1408 // max-w-8xl
	scrollTopMin = 225  // usePageScrollControls：工具栏滚出视口后显示“回到顶部”
)

// View 构建页面。筛选抽屉是页面的浮层，也在这里构建。
func (l *Page) View(c *ui.Context) {
	t := l.env.Theme
	l.filters.Debounce(c, l.Reload)
	fp := l.filterProps(c, t)

	_, winH := c.Size()
	gutter := t.Gutter()
	contentW := min(contentMaxW, l.env.MainWidth-2*pagePadding-gutter)
	l.grid = grid.NewGeometry(contentW, l.landscape)

	components.ScrollArea(c, t, "library-scroll", &l.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		// scrollbar-gutter: stable
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding)
		sc.Children(func() {
			ui.Column(c).Width(contentW).Margin(0, ui.Auto).Gap(24).MinHeight(winH - env.TopbarHeight - 2*pagePadding).Children(func() {
				ui.Column(c).Children(func() {
					components.Text(c, locales.LibraryTitle(), theme.Text4XL, theme.WeightBold, t.TextStrong)
					components.TextMargin(c, l.filters.CountText(l.pages.Total), theme.TextBase, theme.WeightNormal, t.TextMuted, 8, 0, 0, 0)
				})
				bar.FilterBar(c, t, fp)
				l.content(c, t, winH)
			})
		})
	})

	components.ScrollToTopButton(c, t, locales.CommonBackToTop(), l.scroll.State.Y > scrollTopMin, func() {
		l.scroll.State.ScrollTo(0, 0, ui.Animation{Duration: 400 * time.Millisecond, Ease: theme.EaseCSS})
	})
	bar.FilterDrawer(c, t, fp)
}

func (l *Page) content(c *ui.Context, t *theme.Theme, winH float32) {
	switch {
	case l.pages.Waiting(l.filters.Settling()):
		l.loadingGrid(c, t)
	case l.pages.Total == 0:
		l.emptyState(c, t)
	default:
		ui.Box(c).Children(func() {
			wrap := ui.Box(c.Key("grid-wrap"))
			if l.pages.Loading {
				wrap.Opacity(0.6).PassThrough()
			}
			wrap.Children(func() { l.gameGrid(c, t, winH) })
			if l.pages.Loading {
				loadingPill(c, t)
			}
		})
	}
}

func (l *Page) gameGrid(c *ui.Context, t *theme.Theme, winH float32) {
	sortField := enums.GameListSortBy("")
	if l.filters.ShowSortField {
		sortField = enums.GameListSortBy(l.filters.SortBy)
	}
	grid.VirtualGrid(c, grid.Props{
		Geometry:   l.grid,
		Total:      l.pages.Total,
		ViewTop:    env.TopbarHeight,
		ViewBottom: winH,
		OnRange:    l.pages.Ensure,
		Item: func(c *ui.Context, i int) {
			g := l.pages.Games[i]
			if g == nil {
				card.Placeholder(c.Key(fmt.Sprintf("placeholder-%d", i)), t, l.grid.CardW, l.landscape)
				return
			}
			card.GameCard(c.Key(g.ID), t, card.GameCardProps{
				Game:           g,
				Width:          l.grid.CardW,
				Landscape:      l.landscape,
				Cover:          l.env.Covers.Get(g),
				NoCover:        l.env.Covers.Missing(g),
				SortField:      sortField,
				BlurNSFW:       l.blurNSFW,
				Highlight:      l.filters.Debounced,
				Batch:          l.filters.Batch,
				Selected:       l.selected[g.ID],
				OnToggleSelect: func() { l.toggleSelected(g.ID) },
				OnPlay:         func() { l.env.Toast(c, env.Info(locales.NativeFeatureNotMigrated(locales.GameCardStartGame()))) },
				OnDetail:       func() { l.env.Router.Push(env.GamePath(g.ID)) },
			})
		},
	})
}

// loadingPill：加载中提示 absolute inset-x-0 top-0 flex justify-center py-3。
func loadingPill(c *ui.Context, t *theme.Theme) {
	ui.Row(c).Absolute().Top(12).Left(0).Right(0).Justify(ui.Center).PassThrough().Children(func() {
		pill := ui.Row(c).Padding(6, 12).Radius(999).Gap(8).AlignItems(ui.Center).
			Background(t.Pick(theme.White.Alpha(0.85), theme.Brand900.Alpha(0.75))).
			Border(t.HL, t.Pick(theme.Brand200.Alpha(0.7), theme.Brand700.Alpha(0.7))).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
		pill.Children(func() {
			color := t.Pick(theme.Brand600, theme.Brand300)
			components.Spinner(c, 14, color)
			components.Text(c, locales.CommonLoading(), theme.TextSM, theme.WeightNormal, color)
		})
	})
}

// loadingGrid 对应 LibraryGridLoadingState：16 张骨架卡片。
func (l *Page) loadingGrid(c *ui.Context, t *theme.Theme) {
	// grid-cols-[repeat(auto-fill,minmax(max(8rem,11%),1fr))] gap-3
	minW := max(128, l.grid.Width*0.11)
	cols := max(1, int((l.grid.Width+grid.Gap)/(minW+grid.Gap)))
	cardW := (l.grid.Width - grid.Gap*float32(cols-1)) / float32(cols)
	ui.Row(c).Wrap().GapX(grid.Gap).GapY(grid.Gap).Children(func() {
		for i := range 16 {
			card.Placeholder(c.Key(i), t, cardW, l.landscape)
		}
	})
}

// emptyState 对应 total === 0 时的两种空状态。
func (l *Page) emptyState(c *ui.Context, t *theme.Theme) {
	ui.Column(c).Grow(1).FillWidth().Center().Padding(80, 0).Children(func() {
		if l.filters.HasActiveFilters() {
			components.Icon(c, "magnify", 60, t.TextMuted).Margin(0, 0, 16, 0)
			components.Text(c, locales.LibraryNotFound(), theme.TextXL, theme.WeightNormal, t.TextMuted)
			return
		}
		components.Icon(c, "gamepad-variant-outline", 60, t.TextMuted).Margin(0, 0, 16, 0)
		components.Text(c, locales.LibraryEmptyState(), theme.TextXL, theme.WeightNormal, t.TextMuted)
		components.TextMargin(c, locales.LibraryEmptyStateAction(), theme.TextSM, theme.WeightNormal, t.TextMuted, 8, 0, 0, 0)
		// mt-5 grid w-full max-w-lg grid-cols-2 gap-3
		ui.Grid(c).Columns(2).GapX(12).GapY(12).Width(512).Margin(20, 0, 0, 0).Children(func() {
			for _, imp := range importOptions {
				l.emptyImportButton(c, t, imp)
			}
		})
	})
}

// emptyImportButton：BetterButton secondary lg，className 改成 rounded-full、半透明背景、悬停上移。
func (l *Page) emptyImportButton(c *ui.Context, t *theme.Theme, imp importOption) {
	btn := better.Button(c, t, imp.key, better.ButtonProps{
		Variant: better.ButtonSecondary,
		Size:    better.ButtonLG,
		Children: func(color ui.Color) {
			if imp.icon != "" {
				components.Icon(c, imp.icon, 20, color)
			} else {
				ui.Image(c, assets.Importer(imp.key)).Size(20, 20).Radius(6).Fit(ui.Cover).Shrink(0)
			}
			components.Text(c, imp.label(), theme.FontStep{Size: 14, Line: 14}, theme.WeightMedium, color).SingleLine()
		},
	})
	hovered := btn.Hovered()
	bg, border := t.Pick(theme.Brand100.Alpha(0.55), theme.Brand800.Alpha(0.55)), t.Pick(theme.Brand300.Alpha(0.7), theme.Brand600)
	if hovered {
		bg, border = t.Pick(theme.Brand150.Alpha(0.85), theme.Brand700.Alpha(0.7)), t.Pick(theme.Brand400, theme.Brand500)
	}
	lift := btn.AnimateWith("lift", theme.Bool(hovered), 200*time.Millisecond, theme.EaseTailwind)
	btn.FillWidth().Radius(999).Background(bg).BorderColor(border).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).
		Translate(0, -2*lift).OnClick(func() { l.env.Toast(c, env.Info(locales.NativeImportNotMigrated())) })
}
