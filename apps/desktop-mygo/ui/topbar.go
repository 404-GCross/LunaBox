package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// TopbarHeight 对应 TopBar.tsx 的 TOPBAR_HEIGHT。
const TopbarHeight = 28

// topBar 对应 components/bar/TopBar.tsx。
func (a *App) topBar(c *ui.Context) {
	p := a.p
	bar := ui.Row(c).Height(TopbarHeight).Shrink(0).Background(p.topbarBg).
		BorderWidth(0, 0, a.hl, 0).BorderColor(p.topbarBorder).DragWindow()
	bar.Children(func() {
		// 中央标题：h-[20px]，绝对居中
		title := a.assets.titleLight
		if p.dark {
			title = a.assets.titleDark
		}
		w, h := title.Size()
		ui.Image(c, title).Size(20*float32(w)/float32(h), 20).
			Attach(ui.AnchorCenter, ui.AnchorCenter).PassThrough()

		ui.Spacer(c)
		a.windowButton(c, "minimise", iconMinimise, false, func() { a.win.Minimize() })
		maxIcon := iconMaximise
		if a.maximised {
			maxIcon = iconRestore
		}
		a.windowButton(c, "maximise", maxIcon, false, func() { a.win.ToggleMaximize() })
		a.windowButton(c, "close", iconClose, true, func() { a.win.Close() })
	})
}

// windowButton：h-[28px] w-[44px]，hover:bg-brand-200 dark:hover:bg-brand-700，关闭按钮 hover:bg-red-500。
func (a *App) windowButton(c *ui.Context, key string, svg *ui.SVG, danger bool, onClick func()) {
	p := a.p
	btn := ui.ButtonBase(c.Key(key)).Size(44, TopbarHeight).Center().FocusRing(false)
	color := p.controlIcon
	if btn.Hovered() {
		if danger {
			btn.Background(red500)
			color = white
		} else {
			btn.Background(p.controlHover)
		}
	}
	btn.Transition(colorsTransition(150 * time.Millisecond))
	s := btn.Animate("press", 1-0.02*boolf(btn.Pressed()), 150*time.Millisecond)
	color = btn.AnimateColor("icon", color, ui.Animation{Duration: 150 * time.Millisecond, Ease: easeTailwind})
	btn.Scale(s, s).OnClick(onClick).Children(func() {
		ui.Icon(c, svg).Size(10, 10).TextColor(color)
	})
}
