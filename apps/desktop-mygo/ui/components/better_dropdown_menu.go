package components

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// MenuItem 对应 BetterDropdownMenu 的 DropdownMenuItem。
type MenuItem struct {
	Key, Label, Description string
	Icon                    string     // i-mdi-* 图标
	IconColor               ui.Color   // 默认 text-brand-400 dark:text-brand-500
	Image                   *ui.Bitmap // iconSrc
	Pill                    bool       // 彩色胶囊样式（状态标签）
	PillBg, PillText        ui.Color
	DividerBefore           bool
	Disabled                bool
	OnClick                 func()
}

// MenuProps 是 BetterDropdownMenu 的属性。
type MenuProps struct {
	Open        *bool // 受控的打开状态；为 nil 时由组件自己保存
	Items       []MenuItem
	Title       string
	MinWidth    float32 // menuWidth，默认 min-w-[180px]
	Disabled    bool
	SelectedKey string
	AlignStart  bool // align="start"：菜单与 trigger 左对齐
	OpenOnHover bool // openOnHover：悬停 150ms 打开，移出 200ms 后关闭；点击则固定打开
}

// menuHover 是 openOnHover 的计时状态。
type menuHover struct {
	enterAt, leaveAt time.Time
	byHover          bool // 由悬停打开，移出后自动关闭
}

// DropdownMenu 对应 BetterDropdownMenu：点击 trigger 开关，菜单在 trigger 下方右对齐（AlignStart 时左对齐），
// mt-1.5 加 6px 锚点间距；面板 rounded-xl border p-1.5，菜单项 rounded-lg px-3 py-2.5 text-sm。
func DropdownMenu(c *ui.Context, t *theme.Theme, trigger ui.Element, p MenuProps) {
	open := p.Open
	if open == nil {
		open = ui.Local(trigger, "menu-open", func() bool { return false })
	}
	hover := ui.Local(trigger, "menu-hover", func() menuHover { return menuHover{} })
	if p.Disabled {
		*open = false
	} else {
		trigger.OnClick(func() {
			if *open && hover.byHover {
				hover.byHover = false // 点击悬停打开的菜单：固定，不再随移出关闭
				return
			}
			hover.byHover = false
			*open = !*open
		})
	}
	minWidth := p.MinWidth
	if minWidth == 0 {
		minWidth = 180
	}
	from, to := ui.AnchorBottomRight, ui.AnchorTopRight
	if p.AlignStart {
		from, to = ui.AnchorBottomLeft, ui.AnchorTopLeft
	}
	overPanel := false
	ui.PopoverBase(c, trigger, open, func(panel ui.Element) {
		panel.AttachTo(trigger, from, to).Top(12).MinWidth(minWidth).
			Padding(6).Radius(12).Background(t.PanelBg).Border(t.HL, t.PanelBorder)
		overPanel = panel.Hovered()
		panel.Children(func() {
			if p.Title != "" {
				Text(c, p.Title, theme.TextXS, theme.WeightMedium, t.TextFaint).Padding(2, 8, 4, 8)
			}
			for _, it := range p.Items {
				if it.DividerBefore {
					Divider(c, t)
				}
				choose := func() {
					*open = false
					hover.byHover = false
					if it.OnClick != nil {
						it.OnClick()
					}
				}
				if it.Pill {
					pillItem(c, t, it, it.Key == p.SelectedKey, choose)
				} else {
					menuItem(c, t, it, choose)
				}
			}
		})
	})
	if p.OpenOnHover && !p.Disabled {
		hoverOpen(c, trigger.Hovered(), overPanel, open, hover)
	}
}

// hoverOpen 是 openOnHover 的计时：悬停 trigger 150ms 打开；由悬停打开时，指针离开 trigger 与菜单 200ms 后关闭。
func hoverOpen(c *ui.Context, overTrigger, overPanel bool, open *bool, h *menuHover) {
	now := c.Now()
	if overTrigger && !*open {
		if h.enterAt.IsZero() {
			h.enterAt = now
		}
		if wait := 150*time.Millisecond - now.Sub(h.enterAt); wait > 0 {
			c.After(wait)
		} else {
			*open, h.byHover = true, true
		}
	} else if !overTrigger {
		h.enterAt = time.Time{}
	}
	if !*open || !h.byHover || overTrigger || overPanel {
		h.leaveAt = time.Time{}
		return
	}
	if h.leaveAt.IsZero() {
		h.leaveAt = now
	}
	if wait := 200*time.Millisecond - now.Sub(h.leaveAt); wait > 0 {
		c.After(wait)
	} else {
		*open, h.byHover, h.leaveAt = false, false, time.Time{}
	}
}

func menuItem(c *ui.Context, t *theme.Theme, it MenuItem, choose func()) {
	row := ui.ButtonBase(c.Key(it.Key)).FillWidth().Padding(10, 12).Radius(8).AlignItems(ui.Center).
		Justify(ui.Start).FocusRing(false)
	if it.Disabled {
		row.Opacity(0.5).Disabled(true)
	} else if row.Hovered() {
		row.Background(t.MenuHover)
	}
	row.Transition(theme.ColorsTransition(150 * time.Millisecond))
	row.OnClick(choose).Children(func() {
		switch {
		case it.Image != nil:
			ui.Image(c, it.Image).Size(24, 24).Fit(ui.Contain).Margin(0, 12, 0, 0).Shrink(0)
		case it.Icon != "":
			color := it.IconColor
			if color == (ui.Color{}) {
				color = t.TextFaint
			}
			Icon(c, it.Icon, 20, color).Margin(0, 12, 0, 0)
		}
		ui.Column(c).Children(func() {
			// font-medium leading-tight / mt-0.5 text-xs leading-tight
			Text(c, it.Label, theme.FontStep{Size: 14, Line: 17.5}, theme.WeightMedium, t.MenuText).NoWrap()
			if it.Description != "" {
				TextMargin(c, it.Description, theme.FontStep{Size: 12, Line: 15}, theme.WeightNormal, t.TextFaint, 2, 0, 0, 0).NoWrap()
			}
		})
	})
}

// pillItem：flex w-full items-center gap-1.5 rounded-full px-2.5 py-1.5 text-xs font-medium，选中项右侧 check。
func pillItem(c *ui.Context, t *theme.Theme, it MenuItem, selected bool, choose func()) {
	bg, fg := it.PillBg, it.PillText
	if bg == (ui.Color{}) {
		bg, fg = t.Pick(theme.Brand100, theme.Brand700), t.Pick(theme.Brand700, theme.Brand300)
	}
	item := ui.ButtonBase(c.Key(it.Key)).FillWidth().Padding(6, 10).Radius(999).Gap(6).
		AlignItems(ui.Center).Justify(ui.Start).Background(bg).FocusRing(false)
	if it.Disabled {
		item.Opacity(0.5).Disabled(true)
	} else if item.Hovered() {
		item.Shadow(0, 0, 0, 2, theme.Brand400) // focus 时 ring-2 ring-brand-400
	}
	item.OnClick(choose).Children(func() {
		if it.Icon != "" {
			Icon(c, it.Icon, 14, fg)
		}
		Text(c, it.Label, theme.TextXS, theme.WeightMedium, fg).NoWrap()
		if selected {
			Icon(c, "check", 16, fg).Margin(0, 0, 0, ui.Auto)
		}
	})
}
