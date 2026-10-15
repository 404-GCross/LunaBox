package library

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/common/enums"
)

// filterDrawer 是 FilterBar 里的 BetterDrawer：标签、状态、来源、预设、排序。
func (l *Page) filterDrawer(c *ui.Context, t *theme.Theme) {
	components.Drawer(c, t, "filter-drawer", components.DrawerProps{
		Open:       &l.drawerOpen,
		Title:      "筛选",
		CloseLabel: "取消",
		TopOffset:  env.TopbarHeight,
		HeaderAction: func() {
			canClear := l.search != "" || l.activeFilterCount() > 0 ||
				l.sortBy != enums.GameListSortByCreatedAt || l.sortOrder != enums.SortOrderDesc || l.secondarySortBy != ""
			btn := ui.ButtonBase(c.Key("clear-all")).Size(32, 32).Radius(8).Center().FocusRing(false).Label("清除全部筛选与排序")
			if !canClear {
				components.Disable(btn, 0.35)
			}
			btn.OnClick(l.clearFiltersAndSort).Children(func() {
				components.Icon(c, "filter-off-outline", 18, t.TextMuted)
			})
		},
		Body: func() { l.drawerBody(c, t) },
	})
}

func (l *Page) drawerBody(c *ui.Context, t *theme.Theme) {
	ui.Column(c).Children(func() {
		l.tagFilterSection(c, t)
		chipSection(c, t, "status", "状态", l.status != "", l.statusInverted, func() {
			l.statusInverted = !l.statusInverted
			l.Reload()
		}, func() {
			for _, o := range statusOptions {
				if chip(c, t, string(o.value), o.label, l.status == o.value).Clicked() {
					l.status = o.value
					if o.value == "" {
						l.statusInverted = false
					}
					l.Reload()
				}
			}
		})
		chipSection(c, t, "source", "元数据来源", l.source != "", l.sourceInverted, func() {
			l.sourceInverted = !l.sourceInverted
			l.Reload()
		}, func() {
			for _, o := range sourceOptions {
				if chip(c, t, string(o.value), o.label, l.source == o.value).Clicked() {
					l.source = o.value
					if o.value == "" {
						l.sourceInverted = false
					}
					l.Reload()
				}
			}
		})
		components.Divider(c, t)
		l.presetSection(c, t)
		components.Divider(c, t)

		// 主排序
		ui.Column(c).Padding(6, 8).Children(func() {
			ui.Box(c).Margin(0, 0, 6, 0).Children(func() { components.SectionLabel(c, t, "主排序字段") })
			components.Select(c, t, "sort", components.SelectProps{
				Value:   string(l.sortBy),
				Options: sortOptions,
				OnChange: func(v string) {
					l.sortBy = enums.GameListSortBy(v)
					if l.secondarySortBy == l.sortBy {
						l.secondarySortBy = ""
					}
					l.Reload()
				},
			})
		})
		orderButtons(c, t, "order", l.sortOrder, true, func(o enums.SortOrder) { l.sortOrder = o; l.Reload() })
		components.Divider(c, t)

		// 次排序
		ui.Column(c).Padding(6, 8).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Margin(0, 0, 6, 0).Children(func() {
				components.SectionLabel(c, t, "次排序字段")
				help := ui.ButtonBase(c.Key("secondary-help")).Size(20, 20).Radius(999).Center().FocusRing(false)
				help.Children(func() { components.Icon(c, "help-circle-outline", 14, t.TextMuted) })
				components.Tooltip(c, t, help, "先按 [主排序字段] 排序，当 [主排序字段] 相同时，再按 [次排序字段] 排序")
			})
			opts := []components.SelectOption{{Label: "默认顺序", Value: ""}}
			for _, o := range sortOptions {
				if o.Value != string(l.sortBy) {
					opts = append(opts, o)
				}
			}
			components.Select(c, t, "secondary", components.SelectProps{
				Value:   string(l.secondarySortBy),
				Options: opts,
				OnChange: func(v string) {
					l.secondarySortBy = enums.GameListSortBy(v)
					l.Reload()
				},
			})
		})
		orderButtons(c, t, "secondary-order", l.secondarySortOrder, l.secondarySortBy != "", func(o enums.SortOrder) {
			l.secondarySortOrder = o
			l.Reload()
		})

		// 封面展示排序字段
		ui.Row(c).Padding(2, 8, 6, 8).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
			components.SectionLabel(c, t, "封面展示排序字段")
			components.Switch(c, t, "show-sort-field", l.showSortField, false, func(on bool) { l.showSortField = on })
		})
	})
}

// chip 对应抽屉中 rounded-full px-2.5 py-1 text-xs font-medium 的选项，选中时 ring-1。
func chip(c *ui.Context, t *theme.Theme, key, label string, active bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(4, 10).Radius(999).FocusRing(false)
	bg, fg := t.ChipBg, t.ChipText
	switch {
	case active:
		bg, fg = t.ChipActiveBg, t.ChipActiveText
		btn.Shadow(0, 0, 0, t.HL, t.ChipRing)
	case btn.Hovered():
		bg = t.ChipHover
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Text(c, label, theme.TextXS, theme.WeightMedium, fg).NoWrap()
	})
	return btn
}

// smallActionButton 对应 h-7 px-1.5 rounded-md text-sm font-medium 的“反选”“保存筛选与排序”。
func smallActionButton(c *ui.Context, t *theme.Theme, key, icon, label string, enabled, on bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Height(28).Padding(0, 6).Radius(6).Gap(4).AlignItems(ui.Center).FocusRing(false)
	color := t.SectionLabel
	if on {
		color = t.Pick(theme.Neutral600, theme.Brand200)
	}
	if !enabled {
		components.Disable(btn, 0.45)
	} else if btn.Hovered() && !on {
		btn.Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.6)))
		color = t.Pick(theme.Brand900, theme.White)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Icon(c, icon, 14, color)
		components.Text(c, label, theme.TextSM, theme.WeightMedium, color).NoWrap()
	})
	return btn
}

// chipSection：px-2 py-1.5，标题行 mb-1.5（标题 + 反选），下方 flex-wrap gap-1.5 的胶囊。
func chipSection(c *ui.Context, t *theme.Theme, key, title string, canInvert, inverted bool, onInvert func(), chips func()) {
	ui.Column(c.Key(key)).Padding(6, 8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Margin(0, 0, 6, 0).Children(func() {
			components.SectionLabel(c, t, title)
			if smallActionButton(c, t, "invert", "swap-horizontal", "反选", canInvert, canInvert && inverted).Clicked() && canInvert {
				onInvert()
			}
		})
		ui.Row(c).Wrap().GapX(6).GapY(6).Children(chips)
	})
}

// tagFilterSection 对应 TagFilterMenu。
func (l *Page) tagFilterSection(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("tags")).Padding(6, 8).Gap(8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			components.SectionLabel(c, t, "标签筛选")
			canInvert := len(l.tags) > 0
			if smallActionButton(c, t, "invert", "swap-horizontal", "反选", canInvert, canInvert && l.tagsInverted).Clicked() && canInvert {
				l.tagsInverted = !l.tagsInverted
				l.Reload()
			}
		})
		hasValue := len(l.tags) > 0 || l.tagInput != ""
		field := ui.Row(c.Key("tag-field")).Wrap().AlignItems(ui.Center).GapX(6).GapY(6).Padding(6, 8).MinHeight(34).
			Radius(8).Border(t.HL, t.PanelBorder).Background(t.ChipBg)
		if hasValue {
			field.Padding(6, 32, 20, 8)
		}
		field.Children(func() {
			components.Icon(c, "tag-outline", 16, t.TextMuted)
			for _, tag := range l.tags {
				ui.Row(c.Key("tag-"+tag)).Gap(4).AlignItems(ui.Center).Padding(2, 6).Radius(4).Background(theme.Brand800).Children(func() {
					components.Text(c, tag, theme.TextXS, theme.WeightNormal, theme.Brand200)
					x := ui.ButtonBase(c.Key("remove")).FocusRing(false).Label("移除标签")
					x.OnClick(func() { l.removeTag(tag) }).Children(func() { components.Icon(c, "close", 10, theme.Brand200) })
				})
			}
			input := ui.TextInputBase(c.Key("tag-input"), &l.tagInput).Grow(1).Basis(96).MinWidth(96).
				FontSize(theme.TextXS.Size).FixedLineHeight(theme.TextXS.Line).TextColor(t.TextStrong).Placeholder("输入标签...")
			if input.Changed() {
				l.searchTags(l.tagInput)
			}
			if input.Submitted() && len(l.tagSuggestions) > 0 {
				l.addTag(l.tagSuggestions[0])
			}
			if hasValue {
				clr := ui.ButtonBase(c.Key("clear-tags")).Absolute().Bottom(4).Right(4).Padding(2).Radius(999).
					Background(theme.Brand900.Alpha(0.7)).FocusRing(false).Label("清除标签")
				clr.OnClick(func() {
					l.tags, l.tagInput, l.tagSuggestions = nil, "", nil
					l.tagsInverted = false
					l.Reload()
				}).Children(func() { components.Icon(c, "close-circle", 14, theme.Brand400) })
			}
		})
		if l.tagInput != "" && len(l.tagSuggestions) > 0 {
			l.tagSuggestionList(c, t, field)
		}
	})
}

func (l *Page) tagSuggestionList(c *ui.Context, t *theme.Theme, field ui.Element) {
	ui.Overlay(c, func() {
		list := ui.Column(c.Key("tag-suggestions")).AttachTo(field, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(4).
			Width(field.Bounds().W).MaxHeight(144).Padding(4).Radius(8).
			Background(theme.Brand900.Alpha(0.95)).Border(t.HL, t.PanelBorder).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		list.Children(func() {
			ui.Scroll(c).Grow(1).Children(func() {
				for _, s := range l.tagSuggestions {
					item := ui.ButtonBase(c.Key(s)).FillWidth().Padding(6, 10).Radius(6).Justify(ui.Start).FocusRing(false)
					if item.Hovered() {
						item.Background(theme.Brand700)
					}
					item.OnClick(func() { l.addTag(s) }).Children(func() {
						components.Text(c, s, theme.TextXS, theme.WeightNormal, theme.Brand200)
					})
				}
			})
		})
	})
}

// presetSection 对应 GameFilterPresetMenu（只读：列出并应用已保存的预设）。
func (l *Page) presetSection(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("presets")).Padding(6, 8).Gap(8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			components.SectionLabel(c, t, "游戏筛选预设")
			if smallActionButton(c, t, "save", "bookmark-plus-outline", "保存筛选与排序", true, false).Clicked() {
				l.env.Toast(c, "原型未迁移保存预设")
			}
		})
		if len(l.presets) == 0 {
			empty := ui.ButtonBase(c.Key("empty")).FillWidth().Padding(16, 12).Gap(6).Center().Radius(8).
				Border(t.HL, t.PanelBorder).BorderStyle(ui.BorderDashed).FocusRing(false)
			color := t.TextFaint
			if empty.Hovered() {
				empty.Background(theme.Brand900.Alpha(0.4))
				color = theme.Brand300
			}
			empty.Children(func() {
				components.Text(c, "尚未保存预设", theme.TextXS, theme.WeightNormal, color)
				components.Icon(c, "help-circle-outline", 16, color)
			})
			return
		}
		ui.Column(c).Gap(6).Children(func() {
			for _, preset := range l.presets {
				row := ui.ButtonBase(c.Key(preset.ID)).FillWidth().Padding(6, 8).Radius(8).Justify(ui.Start).
					Border(t.HL, t.PanelBorder).Background(theme.Brand900.Alpha(0.4)).FocusRing(false)
				if row.Hovered() {
					row.BorderColor(theme.Brand600)
				}
				row.OnClick(func() { l.applyPreset(preset) }).Children(func() {
					components.Text(c, preset.Name, theme.TextXS, theme.WeightMedium, theme.Brand200).SingleLine()
				})
			}
		})
	})
}

// orderButtons：grid grid-cols-2 gap-1.5，按钮 rounded-lg px-2.5 py-2 text-xs font-medium。
func orderButtons(c *ui.Context, t *theme.Theme, key string, current enums.SortOrder, enabled bool, onChange func(enums.SortOrder)) {
	ui.Row(c.Key(key)).Padding(2, 8, 6, 8).Gap(6).Children(func() {
		for _, o := range []struct {
			value enums.SortOrder
			label string
			icon  string
		}{
			{enums.SortOrderAsc, "升序", "sort-ascending"},
			{enums.SortOrderDesc, "降序", "sort-descending"},
		} {
			btn := ui.ButtonBase(c.Key(o.value)).Grow(1).Basis(0).Padding(8, 10).Radius(8).Gap(6).Center().FocusRing(false)
			bg, fg := t.ChipBg, t.ChipText
			switch {
			case current == o.value:
				bg, fg = t.ChipActiveBg, t.ChipActiveText
			case enabled && btn.Hovered():
				bg = t.ChipHover
			}
			if !enabled {
				components.Disable(btn, 0.4)
			}
			btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
			btn.OnClick(func() { onChange(o.value) }).Children(func() {
				components.Icon(c, o.icon, 16, fg)
				components.Text(c, o.label, theme.TextXS, theme.WeightMedium, fg)
			})
		}
	})
}
