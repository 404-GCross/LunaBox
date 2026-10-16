package better

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// tooltipDelay 是 BetterTooltip 的默认 delay。
const tooltipDelay = 300 * time.Millisecond

// Tooltip 对应 BetterTooltip（side="top"）：悬停 300ms 后在 anchor 上方 8px 处显示
// rounded-md px-2.5 py-1.5 text-xs font-medium 的提示，max-w-72。
func Tooltip(c *ui.Context, t *theme.Theme, anchor ui.Element, content string) {
	since := ui.Local(anchor, "tooltip", func() time.Time { return time.Time{} })
	if !anchor.Hovered() {
		*since = time.Time{}
		return
	}
	now := c.Now()
	if since.IsZero() {
		*since = now
	}
	if wait := tooltipDelay - now.Sub(*since); wait > 0 {
		c.After(wait)
		return
	}
	ui.Overlay(c, func() {
		ui.Box(c).AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Translate(0, -8).
			Padding(6, 10).Radius(6).MaxWidth(288).Background(t.TooltipBg).Border(t.HL, t.TooltipBorder).
			PassThrough().Children(func() {
			components.Text(c, content, theme.TextXS, theme.WeightMedium, t.TooltipText).NoWrap()
		})
	})
}
