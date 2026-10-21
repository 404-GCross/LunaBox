package library

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/bar"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// batchActions 是 library.tsx 传给 FilterBar 的 batchActions：导入到 Steam、设为状态、加入收藏夹、删除。
func (l *Page) batchActions(c *ui.Context, t *theme.Theme) {
	none := len(l.selected) == 0
	l.batchAction(c, t, "steam", "steam", locales.LibraryBatchImportToSteam(), none, t.NavText)
	setStatus := l.batchAction(c, t, "set-status", "tag-edit-outline", locales.LibrarySetStatus(), none, t.NavText)
	l.statusMenu(c, t, setStatus, none)
	l.batchAction(c, t, "add-category", "folder-plus-outline", locales.LibraryBatchAddToFilter(), none, t.NavText)
	l.batchAction(c, t, "delete", "delete", locales.LibraryToastBatchDeleteTitle(), none, t.Pick(theme.Error600, theme.Error400))
}

// batchAction：glass-panel px-3 py-2 rounded-lg border 的图标按钮，外面包 BetterTooltip。
func (l *Page) batchAction(c *ui.Context, t *theme.Theme, key, icon, label string, disabled bool, color ui.Color) ui.Element {
	bg, border := t.GlassPanel(t.PanelBg, t.PanelBorder)
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 12).Radius(8).Border(t.HL, border).Background(bg).
		AlignItems(ui.Center).FocusRing(false).Label(label)
	if disabled {
		btn.Opacity(0.5)
	} else {
		if btn.Hovered() && !t.Glass { // glass-panel 的 data-glass:bg 比 hover:bg 更具体
			btn.Background(t.MenuHover)
		}
		if key != "set-status" {
			btn.OnClick(func() { l.env.Toast(c, env.Info(locales.NativeFeatureNotMigrated(label))) })
		}
	}
	btn.Children(func() { components.Icon(c, icon, 18, color) })
	better.Tooltip(c, t, btn, label)
	return btn
}

// statusMenu 是“设为状态”的 BetterDropdownMenu（pill 样式，min-w-[130px]）。
func (l *Page) statusMenu(c *ui.Context, t *theme.Theme, trigger ui.Element, disabled bool) {
	var items []better.MenuItem
	for _, o := range bar.StatusOptions[1:] {
		icon, bg, fg := components.GameStatusStyle(t, o.Value)
		items = append(items, better.MenuItem{
			Key: string(o.Value), Label: o.Label(), Icon: icon, Pill: true, PillBg: bg, PillText: fg,
			OnClick: func() { l.env.Toast(c, env.Info(locales.NativeFeatureNotMigrated(o.Label()))) },
		})
	}
	better.DropdownMenu(c, t, trigger, better.MenuProps{Title: locales.LibrarySetStatus(), MinWidth: 130, Items: items, Disabled: disabled})
}
