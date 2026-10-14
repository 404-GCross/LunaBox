package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/internal/common/enums"
	"lunabox/internal/models"
)

// 与 VirtualGameGrid.tsx 相同的网格参数（rem 按 16px）。
const (
	portraitMinWidth  = 140 // 8.75rem
	landscapeMinWidth = 256 // 16rem
	gridGap           = 12  // 0.75rem
	cardMetaHeight    = 56  // 3.5rem，行高估算用
	gridOverscan      = 2
)

func coverRatio(landscape bool) float32 {
	if landscape {
		return 9.0 / 16
	}
	return 3.6 / 3
}

type gridGeometry struct {
	width     float32
	cols      int
	cardW     float32
	rowH      float32
	landscape bool
}

func newGridGeometry(width float32, landscape bool) gridGeometry {
	minW := float32(portraitMinWidth)
	if landscape {
		minW = landscapeMinWidth
	}
	cols := max(1, int(math.Floor(float64((width+gridGap)/(minW+gridGap)))))
	cardW := (width - gridGap*float32(cols-1)) / float32(cols)
	return gridGeometry{
		width:     width,
		cols:      cols,
		cardW:     cardW,
		rowH:      float32(math.Ceil(float64(cardW*coverRatio(landscape) + cardMetaHeight))),
		landscape: landscape,
	}
}

// virtualGrid 对应 VirtualGameGrid：只构建视口内（加 overscan 两行）的行，每行绝对定位在 index*rowH。
func (l *library) virtualGrid(c *ui.Context, winH float32) {
	g := l.grid
	rows := (l.total + g.cols - 1) / g.cols
	box := ui.Box(c.Key("vgrid")).FillWidth().Height(float32(rows) * g.rowH)
	top := box.Bounds().Y // 上一帧在窗口中的位置，已含滚动偏移
	if box.Bounds().H == 0 {
		top = TopbarHeight + 250
	}
	first := max(0, int(math.Floor(float64((TopbarHeight-top)/g.rowH)))-gridOverscan)
	last := min(rows-1, int(math.Floor(float64((winH-top)/g.rowH)))+gridOverscan)
	if last < first {
		return
	}
	l.ensureWindow(first*g.cols, min(l.total-1, (last+1)*g.cols-1))
	sortField := enums.GameListSortBy("")
	if l.showSortField {
		sortField = l.sortBy
	}
	box.Children(func() {
		for r := first; r <= last; r++ {
			ui.Row(c.Key(r)).Absolute().Top(float32(r) * g.rowH).Left(0).Right(0).Gap(gridGap).AlignItems(ui.Start).Children(func() {
				for col := range g.cols {
					i := r*g.cols + col
					if i >= l.total {
						break
					}
					if game := l.games[i]; game != nil {
						l.gameCard(c.Key(game.ID), game, sortField)
					} else {
						l.placeholderCard(c.Key(fmt.Sprintf("placeholder-%d", i)))
					}
				}
			})
		}
	})
}

// gameCard 对应 components/card/GameCard.tsx。
func (l *library) gameCard(c *ui.Context, g *models.Game, sortField enums.GameListSortBy) {
	a := l.a
	p := a.p
	geo := l.grid
	selected := l.selected[g.ID]

	card := ui.Column(c).Width(geo.cardW).Shrink(0).Radius(12).Clip().Background(p.cardBg).Border(a.hl, p.cardBorder)
	hovered := card.Hovered()
	if hovered {
		card.BorderColor(p.cardBorderHover)
	}
	card.Transition(colorsTransition(200 * time.Millisecond))
	if l.batch {
		card.Cursor(ui.CursorPointer)
		if card.Clicked() {
			l.toggleSelected(g.ID)
			selected = !selected
		}
		if selected {
			card.InsetShadow(0, 0, 0, 2, neutral400) // ring-2 ring-inset ring-neutral-400
		}
	}
	overlay := card.AnimateWith("hover", boolf(hovered && !l.batch), 200*time.Millisecond, easeTailwind)

	card.Children(func() {
		if l.batch {
			l.selectionBadge(c, g.ID, selected)
		}
		innerW := geo.cardW - 2*a.hl
		ui.Box(c).Height(innerW * coverRatio(geo.landscape)).Clip().Background(p.coverBg).Shrink(0).Children(func() {
			if bmp := a.covers.get(g); bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			} else if key, _ := coverKey(g); key == "" || a.covers.failed[key] {
				ui.Box(c).Fill().Center().Children(func() {
					icon(c, a.icon("image-off"), 36, brand400)
				})
			}

			if g.Status == enums.StatusCompleted {
				ui.Box(c).Absolute().Top(6).Right(6).Size(24, 24).Radius(999).Center().Background(yellow500).
					Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1)).
					Children(func() { icon(c, a.icon("trophy"), 14, white) })
			}

			if s := sortFieldText(g, sortField); s != "" {
				sortStrip(c, s)
			}

			if !l.batch {
				l.hoverOverlay(c, g, overlay)
			}
		})

		pad := []float32{4, 8, 8, 8} // px-2 pt-1 pb-2
		if geo.landscape {
			pad = []float32{8, 12, 12, 12} // px-3 pt-2 pb-3
		}
		ui.Column(c).Padding(pad...).Background(p.cardBg).Children(func() {
			l.highlightText(c, g.Name, fontStep{14, 17.5}, weightBold, p.textStrong)
			company := g.Company
			if company == "" {
				company = "Unknown Developer"
			}
			l.highlightText(c, company, fontStep{12, 15}, weightNormal, p.textMuted)
		})
	})
}

// sortStrip 是封面底部的排序字段覆盖条：bg-gradient-to-t from-black/75 via-black/45 to-transparent px-2 pt-5 pb-1.5。
func sortStrip(c *ui.Context, s string) {
	// 高度 = pt-5 + 16 + pb-1.5 = 42。三段渐变拆成上下两半；全部绝对定位，按顺序绘制，文字在最上层。
	const h = 20 + 16 + 6
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(h).PassThrough().Children(func() {
		ui.Box(c).Absolute().Left(0).Right(0).Top(0).Height(h / 2).
			LinearGradient(ui.LinearGradient{From: ui.RGBA(0, 0, 0, 0), To: ui.RGBA(0, 0, 0, 0.45), Angle: 180, Oklab: true})
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(h / 2).
			LinearGradient(ui.LinearGradient{From: ui.RGBA(0, 0, 0, 0.45), To: ui.RGBA(0, 0, 0, 0.75), Angle: 180, Oklab: true})
		ui.Box(c).Absolute().Left(8).Right(8).Bottom(6).Children(func() {
			text(c, s, textXS, weightSemibold, white).SingleLine()
		})
	})
}

// hoverOverlay：absolute inset-0 bg-black/50，开始游戏 / 详情两个圆形按钮。
func (l *library) hoverOverlay(c *ui.Context, g *models.Game, opacity float32) {
	a := l.a
	ov := ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(8).
		Background(ui.RGBA(0, 0, 0, 0.5)).Opacity(opacity)
	if opacity == 0 {
		ov.Invisible()
	}
	ov.Children(func() {
		play := l.roundOverlayButton(c, "play", "play", neutral600, neutral500, true)
		play.Label("开始游戏").OnClick(func() { a.toast(c, "原型不启动游戏："+g.Name) })
		info := l.roundOverlayButton(c, "info", "information-variant", white.Alpha(0.2), white.Alpha(0.3), false)
		info.Label("详情").OnClick(func() { a.toast(c, "原型未迁移详情页："+g.Name) })
	})
}

// roundOverlayButton：h-8 w-8 rounded-full，hover:scale-110 active:scale-95（transition-transform 150ms）。
func (l *library) roundOverlayButton(c *ui.Context, key, iconName string, bg, bgHover ui.Color, shadow bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(32, 32).Radius(999).Center().FocusRing(false)
	btn.Background(bg)
	scale := float32(1)
	switch {
	case btn.Pressed():
		scale = 0.95
	case btn.Hovered():
		btn.Background(bgHover)
		scale = 1.1
	}
	if shadow {
		btn.Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
	}
	s := btn.AnimateWith("scale", scale, 150*time.Millisecond, easeTailwind)
	btn.Scale(s, s).Children(func() { icon(c, l.a.icon(iconName), 18, white) })
	return btn
}

// selectionBadge：批量模式下左上角的圆形勾选框。
func (l *library) selectionBadge(c *ui.Context, id string, selected bool) {
	p := l.a.p
	badge := ui.ButtonBase(c.Key("select")).Absolute().Left(8).Top(8).Size(24, 24).Radius(999).Center().
		Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).FocusRing(false)
	color := ui.Transparent
	if selected {
		badge.Background(neutral600).Border(l.a.hl, neutral600)
		color = white
	} else {
		badge.Background(brand800.Alpha(0.9)).Border(l.a.hl, brand600)
		if !p.dark {
			badge.Background(white.Alpha(0.9)).BorderColor(brand300)
		}
	}
	badge.OnClick(func() { l.toggleSelected(id) }).Children(func() {
		icon(c, l.a.icon("check"), 14, color)
	})
}

func (l *library) toggleSelected(id string) {
	if l.selected[id] {
		delete(l.selected, id)
	} else {
		l.selected[id] = true
	}
}

// placeholderCard 对应 GameCardPlaceholder（数据未到达的格子）。
func (l *library) placeholderCard(c *ui.Context) {
	p := l.a.p
	geo := l.grid
	card := ui.Column(c).Width(geo.cardW).Shrink(0).Radius(12).Clip().Background(p.cardBg).Border(l.a.hl, p.cardBorder).PassThrough()
	card.Opacity(pulse(card)).Children(func() {
		ui.Box(c).Height((geo.cardW - 2*l.a.hl) * coverRatio(geo.landscape)).Background(p.coverBg.Alpha(0.8))
		ui.Column(c).Padding(4, 8, 8, 8).Gap(4).Children(func() {
			ui.Box(c).Height(16).WidthPercent(80).Radius(4).Background(p.coverBg)
			ui.Box(c).Height(12).WidthPercent(60).Radius(4).Background(p.coverBg.Alpha(0.8))
		})
	})
}

// highlightText 对应 HighlightText：搜索词以 bg-yellow-500/50 标出（暗色）。
func (l *library) highlightText(c *ui.Context, s string, f fontStep, weight int, color ui.Color) {
	q := strings.TrimSpace(l.debounced)
	idx := -1
	if q != "" {
		idx = strings.Index(strings.ToLower(s), strings.ToLower(q))
	}
	if idx < 0 {
		text(c, s, f, weight, color).SingleLine()
		return
	}
	mark := yellow500.Alpha(0.5)
	if !l.a.p.dark {
		mark = ui.Hex("#fde047").Alpha(0.8)
	}
	end := idx + len(q)
	ui.RichText(c,
		ui.Span{Text: s[:idx]},
		ui.Span{Text: s[idx:end], Background: mark},
		ui.Span{Text: s[end:]},
	).FontSize(f.size).FixedLineHeight(f.line).FontWeight(weight).TextColor(color).SingleLine().
		Margin(-lineOverflow(c, s, f, weight), 0, -lineOverflow(c, s, f, weight), 0)
}

// sortFieldText 对应 formatSortFieldValue：名称、开发商已在卡片底部展示，不再覆盖。
func sortFieldText(g *models.Game, sortBy enums.GameListSortBy) string {
	switch sortBy {
	case enums.GameListSortByLastPlayedAt:
		if g.LastPlayedAt == nil {
			return "从未"
		}
		return g.LastPlayedAt.Local().Format("2006/01/02")
	case enums.GameListSortByCreatedAt:
		return g.CreatedAt.Local().Format("2006/01/02")
	case enums.GameListSortByRating:
		return fmt.Sprintf("%.1f/10.0", g.Rating)
	case enums.GameListSortByReleaseDate:
		if g.ReleaseDate == "" {
			return "暂无"
		}
		return g.ReleaseDate
	}
	return ""
}
