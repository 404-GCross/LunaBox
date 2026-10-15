// Package components 对应原版 frontend/src/components/ui：Better* 系列与通用小组件。
// 组件只依赖 theme 与 assets，不知道页面状态；页面通过参数和回调与它们交互，
// 组件自己的临时状态（弹出层是否打开、悬停计时等）用 ui.Local 挂在自己的元素上。
package components

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/theme"
)

// Text 创建一段带 Tailwind 字号和固定行高的文字，与浏览器的行盒高度一致。
func Text(c *ui.Context, s string, f theme.FontStep, weight int, color ui.Color) ui.Element {
	return TextMargin(c, s, f, weight, color, 0, 0, 0, 0)
}

// TextMargin 同 Text，另带外边距（Element.Margin 会覆盖 Text 设置的负外边距，所以合在一起设）。
//
// MyGo 的行高取 max(行高, 字体升部+降部)，而 CSS 允许行高小于字体高度、让字形上下等量溢出
// （负的半行距）。这里用对称的负外边距把文字占位压回 CSS 的行高，字形位置与浏览器一致。
func TextMargin(c *ui.Context, s string, f theme.FontStep, weight int, color ui.Color, top, right, bottom, left float32) ui.Element {
	d := LineOverflow(f)
	e := ui.Text(c, s).FontSize(f.Size).FixedLineHeight(f.Line).FontWeight(weight).TextColor(color).
		Margin(top-d, right, bottom-d, left)
	if theme.UIFont != "" {
		e.Font(theme.UIFont)
	}
	return e
}

// LineOverflow 是字形高出 CSS 行高的一半。
func LineOverflow(f theme.FontStep) float32 {
	return max(0, (f.Size*theme.NaturalLineRatio()-f.Line)/2)
}

// HighlightText 对应 HighlightText：query 命中的部分以 bg-yellow-500/50（暗色）标出。
func HighlightText(c *ui.Context, t *theme.Theme, s, query string, f theme.FontStep, weight int, color ui.Color) {
	q := strings.TrimSpace(query)
	idx := -1
	if q != "" {
		idx = strings.Index(strings.ToLower(s), strings.ToLower(q))
	}
	if idx < 0 {
		Text(c, s, f, weight, color).SingleLine()
		return
	}
	mark := t.Pick(ui.Hex("#fde047").Alpha(0.8), theme.Yellow500.Alpha(0.5))
	end := idx + len(q)
	d := LineOverflow(f)
	ui.RichText(c,
		ui.Span{Text: s[:idx]},
		ui.Span{Text: s[idx:end], Background: mark},
		ui.Span{Text: s[end:]},
	).FontSize(f.Size).FixedLineHeight(f.Line).FontWeight(weight).TextColor(color).SingleLine().Margin(-d, 0, -d, 0)
}

// SectionLabel：text-sm font-medium text-brand-700 dark:text-brand-300。
func SectionLabel(c *ui.Context, t *theme.Theme, s string) ui.Element {
	return Text(c, s, theme.TextSM, theme.WeightMedium, t.SectionLabel).NoWrap()
}

// Icon 以 1em 见方显示 i-mdi-* 图标。
func Icon(c *ui.Context, name string, size float32, color ui.Color) ui.Element {
	return ui.Icon(c, assets.Icon(name)).Size(size, size).TextColor(color).Shrink(0)
}

// Spinner 是 i-mdi-loading animate-spin。
func Spinner(c *ui.Context, size float32, color ui.Color) ui.Element {
	ic := Icon(c, "loading", size, color)
	return ic.Rotate(ic.Loop("spin", time.Second, ui.Linear) * 360)
}

// Divider：my-1 border-t。
func Divider(c *ui.Context, t *theme.Theme) {
	ui.Box(c).Height(t.HL).Margin(4, 0).Background(t.Divider).Shrink(0)
}

// Pulse 对应 animate-pulse：opacity 1 → .5 → 1，2s，cubic-bezier(.4,0,.6,1)。
func Pulse(e ui.Element) float32 {
	p := e.Loop("pulse", 2*time.Second, ui.Linear)
	if p < 0.5 {
		return 1 - 0.5*theme.EasePulse(p*2)
	}
	return 0.5 + 0.5*theme.EasePulse((p-0.5)*2)
}
