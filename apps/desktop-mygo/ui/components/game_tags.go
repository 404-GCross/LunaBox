package components

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/models"
)

// GameTagsProps 是 GameTags 的属性。增删标签与点击跳转都交给调用方。
type GameTagsProps struct {
	Tags     []models.GameTag
	OnClick  func(tag models.GameTag)
	OnDelete func(tag models.GameTag)
	OnAdd    func()
}

// GameTags 对应 GameTags.tsx：flex flex-wrap items-center gap-2 的标签胶囊，末尾是虚线的“添加标签”按钮。
// 原版点击添加后变成输入框，原型交给 OnAdd。
func GameTags(c *ui.Context, t *theme.Theme, p GameTagsProps) {
	ui.Row(c.Key("game-tags")).Wrap().GapX(8).GapY(8).AlignItems(ui.Center).Children(func() {
		for _, tag := range p.Tags {
			tagPill(c, t, tag, p)
		}
		addTagButton(c, t, p.OnAdd)
	})
}

// tagPill 对应 TagPill：rounded-full px-3 py-1.5 text-xs；用户标签是虚线边框。剧透标签在点开前遮住文字。
func tagPill(c *ui.Context, t *theme.Theme, tag models.GameTag, p GameTagsProps) {
	pill := ui.Row(c.Key(tag.ID)).MaxWidthPercent(100).Padding(6, 12).Radius(999).AlignItems(ui.Center)
	if tag.Source == "user" {
		pill.Border(t.HL, t.Pick(theme.Brand400.Alpha(0.8), theme.Brand500.Alpha(0.7))).BorderStyle(ui.BorderDashed).
			Background(t.Pick(theme.White.Alpha(0.7), theme.Brand900.Alpha(0.45)))
	} else {
		pill.Border(t.HL, t.Pick(theme.Brand200.Alpha(0.9), theme.Brand700.Alpha(0.8))).
			Background(t.Pick(theme.Brand50.Alpha(0.7), theme.Brand800.Alpha(0.55)))
	}
	revealed := ui.Local(pill, "revealed", func() bool { return false })
	spoiler := tag.IsSpoiler && !*revealed
	hovered := pill.Hovered()
	pill.Children(func() {
		label := ui.ButtonBase(c.Key("label")).Padding(0, 2).Radius(999).FocusRing(false).Label(tag.Name)
		color := t.Pick(theme.Brand700, theme.Brand200)
		if label.Hovered() {
			color = t.Pick(theme.Brand900, theme.White)
		}
		color = label.AnimateColor("text", color, theme.Tween(200*time.Millisecond))
		switch {
		case spoiler:
			label.OnClick(func() { *revealed = true })
		case p.OnClick != nil:
			label.OnClick(func() { p.OnClick(tag) })
		}
		label.Children(func() {
			text := Text(c, tag.Name, theme.TextXS, theme.WeightNormal, color).SingleLine()
			if spoiler && !label.Hovered() {
				// 原版是 blur-sm，悬停时清晰；MyGo 的文字没有模糊滤镜，用同色色块遮住。
				text.TextColor(ui.Transparent)
				ui.Box(c).Absolute().Left(0).Right(0).Top(3).Bottom(3).Radius(4).PassThrough().Background(color.Alpha(0.35))
			}
		})
		if p.OnDelete != nil {
			tagDeleteButton(c, t, hovered, func() { p.OnDelete(tag) })
		}
	})
}

// tagDeleteButton：absolute -right-1 -top-1 h-4.5 w-4.5 rounded-full border shadow-sm，悬停胶囊时淡入。
func tagDeleteButton(c *ui.Context, t *theme.Theme, show bool, onClick func()) {
	btn := ui.ButtonBase(c.Key("delete")).Absolute().Right(-4).Top(-4).Size(18, 18).Radius(999).Center().
		FocusRing(false).Label("删除标签").Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
	// dark:bg-brand-950 不在色板里，深色主题下也是 bg-white。
	bg, border, fg := theme.White, t.Pick(theme.Brand200, theme.Brand700), t.Pick(theme.Brand400, theme.Brand500)
	if btn.Hovered() {
		bg = t.Pick(ui.Hex("#fef2f2"), theme.Red500.Alpha(0.12))
		border = t.Pick(ui.Hex("#fecaca"), theme.Red500.Alpha(0.4))
		fg = t.Pick(theme.Red500, ui.Hex("#fca5a5"))
	}
	btn.Background(bg).Border(t.HL, border).Transition(theme.ColorsTransition(200 * time.Millisecond))
	opacity := btn.AnimateWith("show", theme.Bool(show || btn.Hovered()), 200*time.Millisecond, theme.EaseTailwind)
	if opacity == 0 {
		btn.Invisible()
		return
	}
	btn.Opacity(opacity).OnClick(onClick).Children(func() {
		Icon(c, "close", 10, fg)
	})
}

// addTagButton：inline-flex gap-1.5 rounded-full border border-dashed px-3 py-1.5 text-xs。
func addTagButton(c *ui.Context, t *theme.Theme, onAdd func()) {
	btn := ui.ButtonBase(c.Key("add-tag")).Padding(6, 12).Radius(999).Gap(6).AlignItems(ui.Center).FocusRing(false)
	border, fg, bg := t.Pick(theme.Brand300, theme.Brand600), t.Pick(theme.Brand500, theme.Brand400), ui.Transparent
	if btn.Hovered() {
		border, fg, bg = t.Pick(theme.Brand500, theme.Brand400), t.Pick(theme.Brand600, theme.Brand200), t.Pick(theme.Brand50, theme.Brand800.Alpha(0.5))
	}
	btn.Border(t.HL, border).BorderStyle(ui.BorderDashed).Background(bg).Transition(theme.ColorsTransition(200 * time.Millisecond))
	if onAdd != nil {
		btn.OnClick(onAdd)
	}
	btn.Children(func() {
		Icon(c, "plus", 14, fg)
		Text(c, "添加标签", theme.TextXS, theme.WeightNormal, fg).NoWrap()
	})
}
