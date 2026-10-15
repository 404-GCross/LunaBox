package ui

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/internal/common/enums"
)

const drawerWidth = 416 // BetterDrawer placement="right"：w-[26rem]

type drawerState struct {
	open          bool
	scroll        scrollBar
	sortOpen      bool
	secondaryOpen bool
}

// filterDrawer 对应 FilterBar 里的 BetterDrawer（右侧抽屉）。
func (l *library) filterDrawer(c *ui.Context) {
	a := l.a
	p := a.p
	d := &l.drawer
	if d.open && c.Shortcut(0, ui.KeyEscape) {
		d.open = false
	}

	ui.Overlay(c, func() {
		host := ui.Box(c.Key("drawer-host")).Absolute().Top(TopbarHeight).Left(0).Right(0).Bottom(0)
		// 打开 300ms cubic-bezier(.22,1,.36,1)，关闭 200ms ease-in
		dur, ease := 300*time.Millisecond, easeDrawer
		if !d.open {
			dur, ease = 200*time.Millisecond, cubicBezier(0.4, 0, 1, 1)
		}
		t := host.AnimateWith("open", boolf(d.open), dur, ease)
		if t == 0 {
			host.Invisible()
			return
		}
		if !d.open {
			host.PassThrough()
		}
		host.Children(func() {
			// 遮罩：bg-black/35 backdrop-blur-[2px]
			backdrop := ui.ButtonBase(c.Key("backdrop")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				Material(glass.Blur{Radius: 2}).Opacity(t).FocusRing(false).Label("关闭筛选")
			backdrop.OnClick(func() { d.open = false }).Children(func() {
				ui.Box(c).Fill().Background(ui.RGBA(0, 0, 0, 0.35))
			})

			panel := ui.Column(c.Key("drawer-panel")).Absolute().Top(0).Right(0).Bottom(0).Width(drawerWidth).
				Material(glass.Blur{Radius: 20}).Translate((1-t)*drawerWidth, 0).Opacity(0.95 + 0.05*t)
			panel.Children(func() {
				ui.Column(c).Fill().Background(p.panelBg.Alpha(0.96)).BorderWidth(0, 0, 0, a.hl).BorderColor(p.panelBorder).Children(func() {
					l.drawerHeader(c)
					a.scrollArea(c, "drawer-body", &d.scroll, func(w ui.Element) { w.Grow(1).Basis(0) }, func(sc ui.Element) {
						// 没有 scrollbar-gutter: stable，滚动条只在溢出时占位
						right := float32(16)
						if d.scroll.state.MaxY > 0 {
							right += a.gutterWidth()
						}
						sc.Padding(16, right, 16, 16).Children(func() { l.drawerBody(c) })
					})
				})
			})
		})
	})
}

func (l *library) drawerHeader(c *ui.Context) {
	a := l.a
	p := a.p
	ui.Row(c).Padding(16, 20).Gap(16).Justify(ui.SpaceBetween).AlignItems(ui.Center).
		BorderWidth(0, 0, a.hl, 0).BorderColor(p.divider).Shrink(0).Children(func() {
		ui.Row(c).MinHeight(32).Gap(4).AlignItems(ui.Center).Children(func() {
			text(c, "筛选", textBase, weightSemibold, p.textStrong)
			canClear := l.search != "" || l.activeFilterCount() > 0 ||
				l.sortBy != enums.GameListSortByCreatedAt || l.sortOrder != enums.SortOrderDesc || l.secondarySortBy != ""
			clearBtn := ui.ButtonBase(c.Key("clear-all")).Size(32, 32).Radius(8).Center().FocusRing(false).Label("清除全部筛选与排序")
			if !canClear {
				clearBtn.Opacity(0.35).Disabled(true)
			}
			clearBtn.OnClick(l.clearFiltersAndSort).Children(func() {
				icon(c, a.icon("filter-off-outline"), 18, p.textMuted)
			})
		})
		closeBtn := ui.ButtonBase(c.Key("close")).Size(32, 32).Radius(8).Center().Margin(0, -4, 0, 0).FocusRing(false).Label("取消")
		color := p.textMuted
		if closeBtn.Hovered() {
			closeBtn.Background(p.menuHover)
			color = p.textStrong
		}
		closeBtn.Transition(colorsTransition(150 * time.Millisecond))
		closeBtn.OnClick(func() { l.drawer.open = false }).Children(func() {
			icon(c, a.icon("close"), 20, color)
		})
	})
}

func (l *library) clearFiltersAndSort() {
	l.search, l.debounced = "", ""
	l.status, l.statusInverted = "", false
	l.source, l.sourceInverted = "", false
	l.tags, l.tagsInverted, l.tagInput, l.tagSuggestions = nil, false, "", nil
	l.sortBy, l.sortOrder = enums.GameListSortByCreatedAt, enums.SortOrderDesc
	l.secondarySortBy, l.secondarySortOrder = "", enums.SortOrderAsc
	l.reload()
}

func (l *library) drawerBody(c *ui.Context) {
	a := l.a
	ui.Column(c).Children(func() {
		l.tagFilterSection(c)
		l.chipSection(c, "status", "状态", l.status != "", l.statusInverted, func() {
			l.statusInverted = !l.statusInverted
			l.reload()
		}, func() {
			for _, o := range statusOptions {
				if a.chip(c, string(o.value), o.label, l.status == o.value).Clicked() {
					l.status = o.value
					if o.value == "" {
						l.statusInverted = false
					}
					l.reload()
				}
			}
		})
		l.chipSection(c, "source", "元数据来源", l.source != "", l.sourceInverted, func() {
			l.sourceInverted = !l.sourceInverted
			l.reload()
		}, func() {
			for _, o := range sourceOptions {
				if a.chip(c, string(o.value), o.label, l.source == o.value).Clicked() {
					l.source = o.value
					if o.value == "" {
						l.sourceInverted = false
					}
					l.reload()
				}
			}
		})
		a.divider(c)
		l.presetSection(c)
		a.divider(c)

		// 主排序
		ui.Column(c).Padding(6, 8).Children(func() {
			ui.Box(c).Margin(0, 0, 6, 0).Children(func() { a.sectionLabel(c, "主排序字段") })
			a.selectBox(c, "sort", string(l.sortBy), sortOptions, &l.drawer.sortOpen, func(v string) {
				l.sortBy = enums.GameListSortBy(v)
				if l.secondarySortBy == l.sortBy {
					l.secondarySortBy = ""
				}
				l.reload()
			})
		})
		l.orderButtons(c, "order", l.sortOrder, true, func(o enums.SortOrder) { l.sortOrder = o; l.reload() })
		a.divider(c)

		// 次排序
		ui.Column(c).Padding(6, 8).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Margin(0, 0, 6, 0).Children(func() {
				a.sectionLabel(c, "次排序字段")
				help := ui.ButtonBase(c.Key("secondary-help")).Size(20, 20).Radius(999).Center().FocusRing(false)
				help.Children(func() { icon(c, a.icon("help-circle-outline"), 14, a.p.textMuted) })
				a.tooltip(c, help, "先按 [主排序字段] 排序，当 [主排序字段] 相同时，再按 [次排序字段] 排序")
			})
			opts := []selectOption{{"默认顺序", ""}}
			for _, o := range sortOptions {
				if o.value != string(l.sortBy) {
					opts = append(opts, o)
				}
			}
			a.selectBox(c, "secondary", string(l.secondarySortBy), opts, &l.drawer.secondaryOpen, func(v string) {
				l.secondarySortBy = enums.GameListSortBy(v)
				l.reload()
			})
		})
		l.orderButtons(c, "secondary-order", l.secondarySortOrder, l.secondarySortBy != "", func(o enums.SortOrder) {
			l.secondarySortOrder = o
			l.reload()
		})

		// 封面展示排序字段
		ui.Row(c).Padding(2, 8, 6, 8).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
			a.sectionLabel(c, "封面展示排序字段")
			a.switchToggle(c, "show-sort-field", l.showSortField, func(on bool) { l.showSortField = on })
		})
	})
}

// chipSection：px-2 py-1.5，标题行 mb-1.5（标题 + 反选），下方 flex-wrap gap-1.5 的胶囊。
func (l *library) chipSection(c *ui.Context, key, title string, canInvert, inverted bool, onInvert func(), chips func()) {
	a := l.a
	ui.Column(c.Key(key)).Padding(6, 8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Margin(0, 0, 6, 0).Children(func() {
			a.sectionLabel(c, title)
			if a.smallActionButton(c, "invert", "swap-horizontal", "反选", canInvert, canInvert && inverted).Clicked() && canInvert {
				onInvert()
			}
		})
		ui.Row(c).Wrap().GapX(6).GapY(6).Children(chips)
	})
}

// tagFilterSection 对应 TagFilterMenu。
func (l *library) tagFilterSection(c *ui.Context) {
	a := l.a
	p := a.p
	ui.Column(c.Key("tags")).Padding(6, 8).Gap(8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			a.sectionLabel(c, "标签筛选")
			canInvert := len(l.tags) > 0
			if a.smallActionButton(c, "invert", "swap-horizontal", "反选", canInvert, canInvert && l.tagsInverted).Clicked() && canInvert {
				l.tagsInverted = !l.tagsInverted
				l.reload()
			}
		})
		hasValue := len(l.tags) > 0 || l.tagInput != ""
		field := ui.Row(c.Key("tag-field")).Wrap().AlignItems(ui.Center).GapX(6).GapY(6).Padding(6, 8).MinHeight(34).
			Radius(8).Border(a.hl, p.panelBorder).Background(p.chipBg)
		if hasValue {
			field.Padding(6, 32, 20, 8)
		}
		field.Children(func() {
			icon(c, a.icon("tag-outline"), 16, p.textMuted)
			for _, tag := range l.tags {
				ui.Row(c.Key("tag-"+tag)).Gap(4).AlignItems(ui.Center).Padding(2, 6).Radius(4).Background(brand800).Children(func() {
					text(c, tag, textXS, weightNormal, brand200)
					x := ui.ButtonBase(c.Key("remove")).FocusRing(false).Label("移除标签")
					x.OnClick(func() { l.removeTag(tag) }).Children(func() { icon(c, a.icon("close"), 10, brand200) })
				})
			}
			input := ui.TextInputBase(c.Key("tag-input"), &l.tagInput).Grow(1).Basis(96).MinWidth(96).
				FontSize(textXS.size).FixedLineHeight(textXS.line).TextColor(p.textStrong).Placeholder("输入标签...")
			if input.Changed() {
				l.searchTags(l.tagInput)
			}
			if input.Submitted() && len(l.tagSuggestions) > 0 {
				l.addTag(l.tagSuggestions[0])
			}
			if hasValue {
				clr := ui.ButtonBase(c.Key("clear-tags")).Absolute().Bottom(4).Right(4).Padding(2).Radius(999).
					Background(brand900.Alpha(0.7)).FocusRing(false).Label("清除标签")
				clr.OnClick(func() {
					l.tags, l.tagInput, l.tagSuggestions = nil, "", nil
					l.tagsInverted = false
					l.reload()
				}).Children(func() { icon(c, a.icon("close-circle"), 14, brand400) })
			}
		})
		if l.tagInput != "" && len(l.tagSuggestions) > 0 {
			ui.Overlay(c, func() {
				list := ui.Column(c.Key("tag-suggestions")).AttachTo(field, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(4).
					Width(field.Bounds().W).MaxHeight(144).Padding(4).Radius(8).
					Background(brand900.Alpha(0.95)).Border(a.hl, p.panelBorder).
					Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
				list.Children(func() {
					ui.Scroll(c).Grow(1).Children(func() {
						for _, s := range l.tagSuggestions {
							item := ui.ButtonBase(c.Key(s)).FillWidth().Padding(6, 10).Radius(6).Justify(ui.Start).FocusRing(false)
							if item.Hovered() {
								item.Background(brand700)
							}
							item.OnClick(func() { l.addTag(s) }).Children(func() { text(c, s, textXS, weightNormal, brand200) })
						}
					})
				})
			})
		}
	})
}

func (l *library) addTag(tag string) {
	if !contains(l.tags, tag) {
		l.tags = append(l.tags, tag)
	}
	l.tagInput, l.tagSuggestions = "", nil
	l.reload()
}

func (l *library) removeTag(tag string) {
	out := l.tags[:0:0]
	for _, t := range l.tags {
		if t != tag {
			out = append(out, t)
		}
	}
	l.tags = out
	if len(out) == 0 {
		l.tagsInverted = false
	}
	l.reload()
}

// presetSection 对应 GameFilterPresetMenu（只读：列出并应用已保存的预设）。
func (l *library) presetSection(c *ui.Context) {
	a := l.a
	p := a.p
	ui.Column(c.Key("presets")).Padding(6, 8).Gap(8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			a.sectionLabel(c, "游戏筛选预设")
			if a.smallActionButton(c, "save", "bookmark-plus-outline", "保存筛选与排序", true, false).Clicked() {
				a.toast(c, "原型未迁移保存预设")
			}
		})
		if len(l.presets) == 0 {
			empty := ui.ButtonBase(c.Key("empty")).FillWidth().Padding(16, 12).Gap(6).Center().Radius(8).
				Border(a.hl, p.panelBorder).BorderStyle(ui.BorderDashed).FocusRing(false)
			color := p.textFaint
			if empty.Hovered() {
				empty.Background(brand900.Alpha(0.4))
				color = brand300
			}
			empty.Children(func() {
				text(c, "尚未保存预设", textXS, weightNormal, color)
				icon(c, a.icon("help-circle-outline"), 16, color)
			})
			return
		}
		ui.Column(c).Gap(6).Children(func() {
			for _, preset := range l.presets {
				row := ui.ButtonBase(c.Key(preset.ID)).FillWidth().Padding(6, 8).Radius(8).Justify(ui.Start).
					Border(a.hl, p.panelBorder).Background(brand900.Alpha(0.4)).FocusRing(false)
				if row.Hovered() {
					row.BorderColor(brand600)
				}
				row.OnClick(func() {
					l.tags, l.tagsInverted = preset.Tags, len(preset.Tags) > 0 && preset.ExcludeTags
					l.status, l.statusInverted = preset.Status, preset.Status != "" && preset.ExcludeStatus
					l.source, l.sourceInverted = preset.MetadataSource, false
					if preset.SortBy != "" {
						l.sortBy, l.sortOrder = preset.SortBy, preset.SortOrder
					}
					l.secondarySortBy, l.secondarySortOrder = preset.SecondarySortBy, preset.SecondarySortOrder
					l.reload()
				}).Children(func() {
					text(c, preset.Name, textXS, weightMedium, brand200).SingleLine()
				})
			}
		})
	})
}

// orderButtons：grid grid-cols-2 gap-1.5，按钮 rounded-lg px-2.5 py-2 text-xs font-medium。
func (l *library) orderButtons(c *ui.Context, key string, current enums.SortOrder, enabled bool, onChange func(enums.SortOrder)) {
	a := l.a
	p := a.p
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
			active := current == o.value
			bg, fg := p.chipBg, p.chipText
			switch {
			case active:
				bg, fg = p.chipActiveBg, p.chipActiveText
			case enabled && btn.Hovered():
				bg = p.chipHover
			}
			if !enabled {
				btn.Opacity(0.4).Disabled(true)
			}
			btn.Background(bg).Transition(colorsTransition(150 * time.Millisecond))
			btn.OnClick(func() { onChange(o.value) }).Children(func() {
				icon(c, a.icon(o.icon), 16, fg)
				text(c, o.label, textXS, weightMedium, fg)
			})
		}
	})
}
