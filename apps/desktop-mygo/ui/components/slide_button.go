package components

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// SlideButtonProps 是 SlideButton 的属性。
type SlideButtonProps struct {
	Options  []SelectOption
	Value    string
	Disabled bool
	OnChange func(value string)
}

// SlideButton 对应 SlideButton.tsx：bg-brand-150 dark:bg-brand-800 p-1 rounded-lg 的分段按钮，
// 选中项下面的白色滑块（rounded-md shadow-sm）以 300ms ease-out 移动到新位置。
func SlideButton(c *ui.Context, t *theme.Theme, key string, p SlideButtonProps) ui.Element {
	// 按钮 px-4 py-1.5 text-sm font-medium：宽度由文字决定，所以滑块位置不用等布局就能算出。
	left, width := float32(4), float32(0)
	x := float32(4)
	for _, o := range p.Options {
		w := textWidth(c, o.Label, theme.TextSM.Size, theme.WeightMedium) + 32
		if o.Value == p.Value {
			left, width = x, w
		}
		x += w
	}

	box := ui.Row(c.Key(key)).Padding(4).Radius(8).Background(t.Pick(theme.Brand150, theme.Brand800)).Shrink(0)
	ease := theme.EaseTailwind
	left = box.AnimateWith("slider-left", left, 300*time.Millisecond, ease)
	width = box.AnimateWith("slider-width", width, 300*time.Millisecond, ease)
	// 滑块画在容器自己的绘制层里，位于按钮文字之下（绝对定位的元素会盖住流内的按钮）。
	slider := t.Pick(theme.White, theme.Brand700)
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		if width <= 0 {
			return
		}
		s := ui.Rect{X: r.X + left, Y: r.Y + 4, W: width, H: r.H - 8}
		pt.Shadow(s, 6, 0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
		pt.Fill(s, slider, 6)
	})
	box.Children(func() {
		for _, o := range p.Options {
			btn := ui.ButtonBase(c.Key(o.Value)).Padding(6, 16).Radius(6).AlignItems(ui.Center).FocusRing(false).Label(o.Label)
			color := t.Pick(theme.Brand600, theme.Brand400)
			switch {
			case o.Value == p.Value:
				color = t.Pick(theme.Neutral600, theme.Neutral400)
			case btn.Hovered() && !p.Disabled:
				color = t.Pick(theme.Brand900, theme.Brand200)
			}
			if p.Disabled {
				Disable(btn, 0.5)
			} else if p.OnChange != nil {
				btn.OnClick(func() { p.OnChange(o.Value) })
			}
			color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
			btn.Children(func() {
				Text(c, o.Label, theme.TextSM, theme.WeightMedium, color).NoWrap()
			})
		}
	})
	return box
}

// textWidth 是一行文字在界面字体下的宽度。
func textWidth(c *ui.Context, s string, size float32, weight int) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: s, Font: theme.UIFont, Size: size, Weight: weight})
	return w
}
