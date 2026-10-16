// Package card 对应原版 frontend/src/components/card：游戏卡片与加载占位卡片。
package card

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
)

// CoverRatio 是封面高宽比：竖版 aspect-[3/3.6]，横版 aspect-video。
func CoverRatio(landscape bool) float32 {
	if landscape {
		return 9.0 / 16
	}
	return 3.6 / 3
}

// GameCardProps 是 GameCard 的输入。
type GameCardProps struct {
	Game      *models.Game
	Width     float32
	Landscape bool
	Cover     *ui.Bitmap // 已解码的封面，未就绪为 nil
	NoCover   bool       // 没有封面或加载失败，显示 i-mdi-image-off
	SortField enums.GameListSortBy
	BlurNSFW  bool   // config.blur_nsfw_game_covers：NSFW 游戏的封面模糊
	Highlight string // 搜索词，标题与开发商里命中的部分高亮

	Batch, Selected bool
	OnToggleSelect  func()
	OnPlay          func()
	OnDetail        func()
}

// GameCard 对应 GameCard.tsx：rounded-xl border 卡片，封面 + 标题/开发商，悬停显示开始游戏与详情按钮；
// 批量模式下点击切换选中，左上角显示勾选框。
func GameCard(c *ui.Context, t *theme.Theme, p GameCardProps) {
	g := p.Game
	card := ui.Column(c).Width(p.Width).Shrink(0).Radius(12).Clip().Background(t.CardBg).Border(t.HL, t.CardBorder)
	hovered := card.Hovered()
	if hovered {
		card.BorderColor(t.CardBorderHover)
	}
	card.Transition(theme.ColorsTransition(200 * time.Millisecond))
	selected := p.Selected
	if p.Batch {
		card.Cursor(ui.CursorPointer)
		if card.Clicked() && p.OnToggleSelect != nil {
			p.OnToggleSelect()
			selected = !selected
		}
		if selected {
			card.InsetShadow(0, 0, 0, 2, theme.Neutral400) // ring-2 ring-inset ring-neutral-400
		}
	}
	overlay := card.AnimateWith("hover", theme.Bool(hovered && !p.Batch), 200*time.Millisecond, theme.EaseTailwind)

	card.Children(func() {
		if p.Batch {
			selectionBadge(c, t, selected, p.OnToggleSelect)
		}
		innerW := p.Width - 2*t.HL
		// 嵌套裁剪时 MyGo 只在着色器里按最内层裁剪的形状切，外层只用矩形 scissor，
		// 所以封面自己的裁剪要带上卡片内侧的顶部圆角，否则图片会盖住卡片圆角。
		innerR := 12 - t.HL
		ui.Box(c).Height(innerW*CoverRatio(p.Landscape)).Radius(innerR, innerR, 0, 0).Clip().Background(t.CoverBg).Shrink(0).Children(func() {
			switch {
			case p.Cover != nil:
				img := ui.Image(c, p.Cover).Fill().Fit(ui.Cover)
				if g.IsNSFW && p.BlurNSFW {
					components.CoverBlur(c, card, img, false)
				}
			case p.NoCover:
				ui.Box(c).Fill().Center().Children(func() {
					components.Icon(c, "image-off", 36, theme.Brand400)
				})
			}

			if g.Status == enums.StatusCompleted {
				ui.Box(c).Absolute().Top(6).Right(6).Size(24, 24).Radius(999).Center().Background(theme.Yellow500).
					Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1)).
					Children(func() { components.Icon(c, "trophy", 14, theme.White) })
			}

			if s := SortFieldText(g, p.SortField); s != "" {
				sortStrip(c, s)
			}

			if !p.Batch {
				hoverOverlay(c, p, overlay)
			}
		})

		pad := []float32{4, 8, 8, 8} // px-2 pt-1 pb-2
		if p.Landscape {
			pad = []float32{8, 12, 12, 12} // px-3 pt-2 pb-3
		}
		ui.Column(c).Padding(pad...).Background(t.CardBg).Children(func() {
			components.HighlightText(c, t, g.Name, p.Highlight, theme.FontStep{Size: 14, Line: 17.5}, theme.WeightBold, t.TextStrong)
			company := g.Company
			if company == "" {
				company = "Unknown Developer"
			}
			components.HighlightText(c, t, company, p.Highlight, theme.FontStep{Size: 12, Line: 15}, theme.WeightNormal, t.TextMuted)
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
			components.Text(c, s, theme.TextXS, theme.WeightSemibold, theme.White).SingleLine()
		})
	})
}

// hoverOverlay：absolute inset-0 bg-black/50，开始游戏 / 详情两个圆形按钮。
func hoverOverlay(c *ui.Context, p GameCardProps, opacity float32) {
	ov := ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(8).
		Background(ui.RGBA(0, 0, 0, 0.5)).Opacity(opacity)
	if opacity == 0 {
		ov.Invisible()
	}
	ov.Children(func() {
		play := roundOverlayButton(c, "play", "play", theme.Neutral600, theme.Neutral500, true)
		play.Label(locales.GameCardStartGame())
		if p.OnPlay != nil {
			play.OnClick(p.OnPlay)
		}
		info := roundOverlayButton(c, "info", "information-variant", theme.White.Alpha(0.2), theme.White.Alpha(0.3), false)
		info.Label(locales.NativeViewDetails())
		if p.OnDetail != nil {
			info.OnClick(p.OnDetail)
		}
	})
}

// roundOverlayButton：h-8 w-8 rounded-full，hover:scale-110 active:scale-95（transition-transform 150ms）。
func roundOverlayButton(c *ui.Context, key, icon string, bg, bgHover ui.Color, shadow bool) ui.Element {
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
	s := btn.AnimateWith("scale", scale, 150*time.Millisecond, theme.EaseTailwind)
	btn.Scale(s, s).Children(func() { components.Icon(c, icon, 18, theme.White) })
	return btn
}

// selectionBadge：批量模式下左上角的圆形勾选框。
func selectionBadge(c *ui.Context, t *theme.Theme, selected bool, onToggle func()) {
	badge := ui.ButtonBase(c.Key("select")).Absolute().Left(8).Top(8).Size(24, 24).Radius(999).Center().
		Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).FocusRing(false)
	color := ui.Transparent
	if selected {
		badge.Background(theme.Neutral600).Border(t.HL, theme.Neutral600)
		color = theme.White
	} else {
		badge.Background(t.Pick(theme.White.Alpha(0.9), theme.Brand800.Alpha(0.9))).
			Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	}
	if onToggle != nil {
		badge.OnClick(onToggle)
	}
	badge.Children(func() {
		components.Icon(c, "check", 14, color)
	})
}

// Placeholder 对应 GameCardPlaceholder：数据未到达的格子，以及加载中的骨架卡片。
func Placeholder(c *ui.Context, t *theme.Theme, width float32, landscape bool) {
	card := ui.Column(c).Width(width).Shrink(0).Radius(12).Clip().Background(t.CardBg).Border(t.HL, t.CardBorder).PassThrough()
	card.Opacity(components.Pulse(card)).Children(func() {
		ui.Box(c).Height((width - 2*t.HL) * CoverRatio(landscape)).Background(t.CoverBg.Alpha(0.8))
		ui.Column(c).Padding(4, 8, 8, 8).Gap(4).Children(func() {
			ui.Box(c).Height(16).WidthPercent(80).Radius(4).Background(t.CoverBg)
			ui.Box(c).Height(12).WidthPercent(60).Radius(4).Background(t.CoverBg.Alpha(0.8))
		})
	})
}

// SortFieldText 对应 formatSortFieldValue：名称、开发商已在卡片底部展示，不再覆盖。
func SortFieldText(g *models.Game, sortBy enums.GameListSortBy) string {
	switch sortBy {
	case enums.GameListSortByLastPlayedAt:
		if g.LastPlayedAt == nil {
			return locales.CommonNever()
		}
		return timefmt.Date(*g.LastPlayedAt, time.Local)
	case enums.GameListSortByCreatedAt:
		return timefmt.Date(g.CreatedAt, time.Local)
	case enums.GameListSortByRating:
		return fmt.Sprintf("%.1f/10.0", g.Rating)
	case enums.GameListSortByReleaseDate:
		if g.ReleaseDate == "" {
			return locales.CommonUnknownDate()
		}
		return g.ReleaseDate
	}
	return ""
}
