package library

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// batchBar 是批量模式下的第二行（FilterBar 的 batchActions）：全选 / 清空 / 已选数量 / 帮助 / 批量操作。
func (l *Page) batchBar(c *ui.Context, t *theme.Theme) {
	bar := ui.Row(c.Key("batch-bar")).Wrap().AlignItems(ui.Center).Gap(8).Padding(8, 12).Radius(8).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700.Alpha(0.5)))
	if t.Dark {
		bar.LinearGradient(ui.LinearGradient{From: theme.Brand800.Alpha(0.5), To: theme.Brand900.Alpha(0.3), Angle: 90})
	} else {
		bar.LinearGradient(ui.LinearGradient{From: theme.Brand50, To: theme.Brand50.Alpha(0.5), Angle: 90})
	}
	bar.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			batchPill(c, t, "select-all", locales.CommonSelectAll(), true).OnClick(func() {
				for _, g := range l.games {
					l.selected[g.ID] = true
				}
			})
			batchPill(c, t, "clear-selection", locales.CommonClearStore(), true).OnClick(func() { clear(l.selected) })
		})
		batchPill(c, t, "selected-count", "", false).Children(func() {
			components.Text(c, locales.CommonSelected(), theme.TextXS, theme.WeightMedium, t.Text)
			components.TextMargin(c, fmt.Sprint(len(l.selected)), theme.TextXS, theme.WeightSemibold, t.Text, 0, 0, 0, 4)
		})
		help := ui.ButtonBase(c.Key("batch-help")).Size(28, 28).Radius(999).Center().FocusRing(false).Label(locales.CommonBatchSelectionHelp())
		help.Children(func() { components.Icon(c, "help-circle-outline", 18, t.TextMuted) })
		better.Tooltip(c, t, help, locales.CommonBatchSelectionHelp())

		ui.Row(c).Gap(6).Margin(0, 0, 0, ui.Auto).Children(func() {
			none := len(l.selected) == 0
			l.batchAction(c, t, "steam", "steam", locales.LibraryBatchImportToSteam(), none, t.NavText)
			setStatus := l.batchAction(c, t, "set-status", "tag-edit-outline", locales.LibrarySetStatus(), none, t.NavText)
			l.statusMenu(c, t, setStatus, none)
			l.batchAction(c, t, "add-category", "folder-plus-outline", locales.LibraryBatchAddToFilter(), none, t.NavText)
			l.batchAction(c, t, "delete", "delete", locales.LibraryToastBatchDeleteTitle(), none, t.Pick(theme.Error600, theme.Error400))
		})
	})
}

func batchPill(c *ui.Context, t *theme.Theme, key, label string, clickable bool) ui.Element {
	pill := ui.ButtonBase(c.Key(key)).Padding(4, 10).Radius(6).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand600)).Background(t.Pick(theme.White, theme.Brand700.Alpha(0.6))).
		AlignItems(ui.Center).FocusRing(false)
	if clickable && pill.Hovered() {
		pill.Background(t.Pick(theme.Brand50, theme.Brand700))
	}
	if label != "" {
		pill.Children(func() { components.Text(c, label, theme.TextXS, theme.WeightMedium, t.NavText) })
	}
	return pill
}

// batchAction：glass-panel px-3 py-2 rounded-lg border 的图标按钮，外面包 BetterTooltip。
func (l *Page) batchAction(c *ui.Context, t *theme.Theme, key, icon, label string, disabled bool, color ui.Color) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 12).Radius(8).Border(t.HL, t.PanelBorder).Background(t.PanelBg).
		AlignItems(ui.Center).FocusRing(false).Label(label)
	if disabled {
		btn.Opacity(0.5)
	} else {
		if btn.Hovered() {
			btn.Background(t.MenuHover)
		}
		if key != "set-status" {
			btn.OnClick(func() { l.env.Toast(c, locales.NativeFeatureNotMigrated(label)) })
		}
	}
	btn.Children(func() { components.Icon(c, icon, 18, color) })
	better.Tooltip(c, t, btn, label)
	return btn
}

// statusMenu 是“设为状态”的 BetterDropdownMenu（pill 样式，min-w-[130px]）。
func (l *Page) statusMenu(c *ui.Context, t *theme.Theme, trigger ui.Element, disabled bool) {
	var items []better.MenuItem
	for _, o := range statusOptions[1:] {
		icon, bg, fg := components.GameStatusStyle(t, o.value)
		items = append(items, better.MenuItem{
			Key: string(o.value), Label: o.label(), Icon: icon, Pill: true, PillBg: bg, PillText: fg,
			OnClick: func() { l.env.Toast(c, locales.NativeFeatureNotMigrated(o.label())) },
		})
	}
	better.DropdownMenu(c, t, trigger, better.MenuProps{Title: locales.LibrarySetStatus(), MinWidth: 130, Items: items, Disabled: disabled})
}
