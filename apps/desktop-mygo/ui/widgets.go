package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// ---------------------------------------------------------------------------
// 滚动区域：还原 style.css 里的 ::-webkit-scrollbar（9px 槽、3px 透明左边框、常显滑块）。

type scrollBar struct {
	state  ui.ScrollState
	thumb  ui.Point // 拖动滑块时的偏移
	viewH  float32
	gutter bool // scrollbar-gutter: stable，为滑块预留槽宽
}

// scrollArea 构建一个纵向滚动区域：wrap 设置外框尺寸，content 设置滚动容器的内边距并构建内容。
func (a *App) scrollArea(c *ui.Context, key string, sb *scrollBar, wrap func(ui.Element), content func(scroll ui.Element)) {
	w := ui.Box(c.Key(key)).Clip()
	wrap(w)
	if b := w.Bounds(); b.H > 0 {
		sb.viewH = b.H
	}
	w.Children(func() {
		sc := ui.Scroll(c).Fill().TrackScroll(&sb.state)
		content(sc)
		a.scrollThumb(c, sb)
	})
}

// gutterWidth 是 9px 滚动条在当前缩放下吸附到设备像素后的宽度。
func (a *App) gutterWidth() float32 { return devicePx(9, a.scale) }

func (a *App) scrollThumb(c *ui.Context, sb *scrollBar) {
	st := &sb.state
	if st.MaxY <= 0 || sb.viewH <= 0 {
		return
	}
	viewH := sb.viewH
	contentH := viewH + st.MaxY
	thumbH := max(viewH*viewH/contentH, 20)
	travel := viewH - thumbH
	gutter := a.gutterWidth()
	border := devicePx(3, a.scale)

	thumb := ui.Box(c.Key("scroll-thumb")).Absolute().Top(0).Right(0).Width(gutter).Height(thumbH).
		Padding(0, 0, 0, border)
	if !thumb.Dragging() {
		sb.thumb = ui.Point{Y: st.Y / st.MaxY * travel}
	}
	thumb.Draggable(&sb.thumb, ui.DraggableOptions{
		Axis:      ui.Vertical,
		Bounds:    &ui.Rect{H: travel},
		Rigid:     true,
		Animation: ui.Animation{Duration: time.Nanosecond, Ease: ui.Linear},
	})
	if thumb.Dragging() && travel > 0 {
		st.Y = sb.thumb.Y / travel * st.MaxY
	}
	color := a.p.scrollThumb
	if thumb.Hovered() || thumb.Dragging() {
		color = a.p.scrollThumbHover
	}
	thumb.Children(func() {
		ui.Box(c).Fill().Background(color).Radius(3, 6, 6, 3)
	})
}

// ---------------------------------------------------------------------------
// 按钮

// toolButton 对应 FilterBar 的 glass-btn-neutral 按钮：px-3 py-2 text-sm border rounded-lg。
func (a *App) toolButton(c *ui.Context, key string, active bool) ui.Element {
	p := a.p
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 12).Radius(8).Gap(8).AlignItems(ui.Center).FocusRing(false)
	bg, border := p.toolBtnBg, p.toolBtnBorder
	if active {
		bg, border = p.toolBtnActiveBg, p.toolBtnActiveBorder
	}
	if btn.Hovered() {
		bg = p.toolBtnHover
	}
	btn.Background(bg).Border(a.hl, border)
	return btn
}

// iconTextColor 是 toolButton 内图标与文字的颜色。
func (a *App) toolButtonColor(active bool) ui.Color {
	if active {
		return a.p.toolBtnActiveText
	}
	return a.p.toolBtnIcon
}

// chip 对应筛选抽屉中 rounded-full px-2.5 py-1 text-xs font-medium 的选项。
func (a *App) chip(c *ui.Context, key, label string, active bool) ui.Element {
	p := a.p
	btn := ui.ButtonBase(c.Key(key)).Padding(4, 10).Radius(999).FocusRing(false)
	bg, fg := p.chipBg, p.chipText
	switch {
	case active:
		bg, fg = p.chipActiveBg, p.chipActiveText
		btn.Shadow(0, 0, 0, a.hl, p.chipRing) // ring-1
	case btn.Hovered():
		bg = p.chipHover
	}
	btn.Background(bg).Transition(colorsTransition(150 * time.Millisecond)).Children(func() {
		text(c, label, textXS, weightMedium, fg).NoWrap()
	})
	return btn
}

// smallActionButton 对应 h-7 px-1.5 rounded-md text-sm font-medium 的“反选”“保存筛选与排序”。
func (a *App) smallActionButton(c *ui.Context, key, iconName, label string, enabled, on bool) ui.Element {
	p := a.p
	btn := ui.ButtonBase(c.Key(key)).Height(28).Padding(0, 6).Radius(6).Gap(4).AlignItems(ui.Center).FocusRing(false)
	color := p.sectionLabel
	if on {
		color = brand200
		if !p.dark {
			color = neutral600
		}
	}
	if !enabled {
		btn.Opacity(0.45).Disabled(true)
	} else if btn.Hovered() && !on {
		btn.Background(brand700.Alpha(0.6))
		if !p.dark {
			btn.Background(brand50)
		}
		color = white
		if !p.dark {
			color = brand900
		}
	}
	btn.Transition(colorsTransition(150 * time.Millisecond)).Children(func() {
		icon(c, a.icon(iconName), 14, color)
		text(c, label, textSM, weightMedium, color).NoWrap()
	})
	return btn
}

// sectionLabel：text-sm font-medium text-brand-700 dark:text-brand-300。
func (a *App) sectionLabel(c *ui.Context, s string) ui.Element {
	return text(c, s, textSM, weightMedium, a.p.sectionLabel).NoWrap()
}

// divider：my-1 border-t。
func (a *App) divider(c *ui.Context) {
	ui.Box(c).Height(a.hl).Margin(4, 0).Background(a.p.divider).Shrink(0)
}

// ---------------------------------------------------------------------------
// BetterSelect

type selectOption struct {
	label string
	value string
}

// selectBox 对应 BetterSelect：按钮 px-3 py-2 pr-10 rounded-md，弹出列表 py-1 rounded-md。
func (a *App) selectBox(c *ui.Context, key string, value string, options []selectOption, open *bool, onChange func(string)) {
	p := a.p
	label := ""
	for _, o := range options {
		if o.value == value {
			label = o.label
		}
	}
	btn := ui.ButtonBase(c.Key(key)).FillWidth().Padding(8, 40, 8, 12).Radius(6).
		Background(p.selectBg).Border(a.hl, p.selectBorder).Justify(ui.Start).FocusRing(false)
	btn.OnClick(func() { *open = !*open }).Children(func() {
		text(c, label, textBase, weightNormal, p.selectText).SingleLine().Grow(1)
		ui.Box(c).Absolute().Top(0).Bottom(0).Right(12).Center().Children(func() {
			icon(c, a.icon("chevron-down"), 20, p.textFaint)
		})
	})
	width := btn.Bounds().W
	ui.PopoverBase(c, btn, open, func(panel ui.Element) {
		panel.Width(width).Margin(4, 0, 0, 0).Padding(4, 0).Radius(6).MaxHeight(240).
			Background(p.panelBg).Border(a.hl, p.selectBorder)
		panel.Children(func() {
			for _, o := range options {
				item := ui.ButtonBase(c.Key("opt-"+o.value)).FillWidth().Padding(8, 16, 8, 40).Justify(ui.Start).FocusRing(false)
				selected := o.value == value
				switch {
				case item.Hovered():
					item.Background(p.listFocus)
				case selected:
					item.Background(p.listSelected)
				}
				weight := weightNormal
				if selected {
					weight = weightMedium
				}
				item.OnClick(func() {
					*open = false
					onChange(o.value)
				}).Children(func() {
					if selected {
						ui.Box(c).Absolute().Top(0).Bottom(0).Left(12).Center().Children(func() {
							icon(c, a.icon("check"), 20, neutral400)
						})
					}
					text(c, o.label, textBase, weight, p.selectText).SingleLine()
				})
			}
		})
	})
}

// switchToggle 对应 BetterSwitch：w-11 h-6 rounded-full，滑块 w-5 h-5，translate-x-0.5 ↔ 5.5。
func (a *App) switchToggle(c *ui.Context, key string, on bool, onChange func(bool)) {
	p := a.p
	track := ui.ButtonBase(c.Key(key)).Size(44, 24).Radius(999).FocusRing(false).Justify(ui.Start).AlignItems(ui.Center)
	bg := p.switchOff
	if on {
		bg = neutral600
	}
	if track.Hovered() {
		bg = bg.Mix(white, 0.1) // hover:brightness-110
	}
	track.Background(bg).Transition(colorsTransition(150 * time.Millisecond))
	x := track.AnimateWith("knob", 2+20*boolf(on), 100*time.Millisecond, easeTailwind)
	track.OnClick(func() { onChange(!on) }).Children(func() {
		ui.Box(c).Size(20, 20).Radius(999).Background(white).Translate(x, 0)
	})
}

// ---------------------------------------------------------------------------
// BetterDropdownMenu

type menuItem struct {
	key, label, description string
	icon                    string
	iconColor               ui.Color
	image                   *ui.Bitmap
	dividerBefore           bool
	onClick                 func()
}

// dropdownMenu 在 anchor 下方右对齐弹出菜单：rounded-xl border p-1.5，菜单项 rounded-lg px-3 py-2.5 text-sm。
func (a *App) dropdownMenu(c *ui.Context, anchor ui.Element, open *bool, minWidth float32, items []menuItem) {
	p := a.p
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight).Top(12).MinWidth(minWidth).
			Padding(6).Radius(12).Background(p.panelBg).Border(a.hl, p.panelBorder)
		panel.Children(func() {
			for _, it := range items {
				if it.dividerBefore {
					a.divider(c)
				}
				row := ui.ButtonBase(c.Key(it.key)).FillWidth().Padding(10, 12).Radius(8).AlignItems(ui.Center).
					Justify(ui.Start).FocusRing(false)
				if row.Hovered() {
					row.Background(p.menuHover)
				}
				row.Transition(colorsTransition(150 * time.Millisecond))
				row.OnClick(func() {
					*open = false
					if it.onClick != nil {
						it.onClick()
					}
				}).Children(func() {
					switch {
					case it.image != nil:
						ui.Image(c, it.image).Size(24, 24).Fit(ui.Contain).Margin(0, 12, 0, 0).Shrink(0)
					case it.icon != "":
						color := it.iconColor
						if color == (ui.Color{}) {
							color = p.textFaint
						}
						icon(c, a.icon(it.icon), 20, color).Margin(0, 12, 0, 0)
					}
					ui.Column(c).Children(func() {
						text(c, it.label, fontStep{14, 17.5}, weightMedium, brand200Or(p)).NoWrap()
						if it.description != "" {
							textMargin(c, it.description, fontStep{12, 15}, weightNormal, p.textFaint, 2, 0, 0, 0).NoWrap()
						}
					})
				})
			}
		})
	})
}

func brand200Or(p palette) ui.Color {
	if p.dark {
		return brand200
	}
	return brand700
}

// ---------------------------------------------------------------------------
// BetterTooltip：悬停 300ms 后在上方显示 rounded-md px-2.5 py-1.5 text-xs font-medium。

func (a *App) tooltip(c *ui.Context, anchor ui.Element, label string) {
	p := a.p
	since := ui.Local(anchor, "tooltip", func() time.Time { return time.Time{} })
	if !anchor.Hovered() {
		*since = time.Time{}
		return
	}
	now := c.Now()
	if since.IsZero() {
		*since = now
	}
	if wait := 300*time.Millisecond - now.Sub(*since); wait > 0 {
		c.After(wait)
		return
	}
	ui.Overlay(c, func() {
		ui.Box(c).AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Translate(0, -8).
			Padding(6, 10).Radius(6).MaxWidth(288).Background(p.tooltipBg).Border(a.hl, p.tooltipBorder).
			PassThrough().Children(func() {
			text(c, label, textXS, weightMedium, p.tooltipText).NoWrap()
		})
	})
}

// pulse 对应 animate-pulse：opacity 1 → .5 → 1，2s，cubic-bezier(.4,0,.6,1)。
func pulse(e ui.Element) float32 {
	t := e.Loop("pulse", 2*time.Second, ui.Linear)
	ease := cubicBezier(0.4, 0, 0.6, 1)
	if t < 0.5 {
		return 1 - 0.5*ease(t*2)
	}
	return 0.5 + 0.5*ease((t-0.5)*2)
}
