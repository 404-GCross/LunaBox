package ui

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/internal/common/enums"
)

// filterBar 对应 components/bar/FilterBar.tsx（flex flex-wrap items-center justify-between gap-4 my-4）。
func (l *library) filterBar(c *ui.Context) {
	ui.Column(c.Key("filterbar")).Margin(16, 0).Gap(16).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(16).Children(func() {
			l.searchInput(c)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
				l.randomButton(c)
				l.batchToggle(c)
				l.filterButton(c)
				l.addGameButton(c)
			})
		})
		if l.batch {
			l.batchBar(c)
		}
	})
}

// searchInput：relative flex-1 max-w-md，input p-2 pl-10 pr-10 text-sm rounded-lg border。
func (l *library) searchInput(c *ui.Context) {
	a := l.a
	p := a.p
	box := ui.Row(c.Key("search")).Grow(1).MaxWidth(448).AlignItems(ui.Center).Radius(8).
		Background(p.inputBg).Border(a.hl, p.inputBorder)
	box.Children(func() {
		ui.Box(c).Absolute().Top(0).Bottom(0).Left(12).Center().PassThrough().Children(func() {
			icon(c, a.icon("magnify"), 18, p.inputIcon)
		})
		input := ui.TextInputBase(c.Key("search-input"), &l.search).Grow(1).Padding(8, 40).
			FontSize(textSM.size).FixedLineHeight(textSM.line).TextColor(p.textStrong).
			Placeholder("搜索游戏...").Label("搜索游戏")
		if input.Changed() {
			l.searchEditedAt = time.Now()
		}
		if input.Focused() {
			box.BorderColor(p.inputFocus)
		}
		if l.search != "" {
			clear := ui.ButtonBase(c.Key("clear")).Absolute().Top(0).Bottom(0).Right(6).Width(32).Center().
				Radius(6).FocusRing(false).Label("清除搜索")
			clear.OnClick(func() {
				l.search, l.debounced = "", ""
				l.reload()
			}).Children(func() {
				color := brand400
				if !p.dark {
					color = brand500
				}
				icon(c, a.icon("close-circle-outline"), 18, color)
			})
		}
	})
}

func (l *library) randomButton(c *ui.Context) {
	a := l.a
	disabled := l.loading || l.search != l.debounced || l.total == 0 || l.randomLoading
	btn := a.toolButton(c, "random", false).Label("随机打开游戏")
	if disabled {
		btn.Opacity(0.5).Disabled(true)
	}
	name := "dice-multiple-outline"
	if l.randomLoading {
		name = "loading"
	}
	btn.OnClick(func() { l.openRandomGame(c) }).Children(func() {
		ic := icon(c, a.icon(name), 18, a.toolButtonColor(false))
		if l.randomLoading {
			ic.Rotate(ic.Loop("spin", time.Second, ui.Linear) * 360)
		}
	})
}

// openRandomGame 随机取一个游戏。原版会打开详情页，原型以提示代替。
func (l *library) openRandomGame(c *ui.Context) {
	if l.total == 0 {
		return
	}
	l.randomLoading = true
	req := l.request()
	req.Offset, req.Limit = rand.IntN(l.total), 1
	l.a.async(func() func() {
		resp, err := l.a.backend.GetGames(req)
		return func() {
			l.randomLoading = false
			if err != nil || len(resp.Games) == 0 {
				l.a.toast(c, "随机选择游戏失败")
				return
			}
			l.a.toast(c, "随机打开："+resp.Games[0].Name+"（原型未迁移详情页）")
		}
	})
}

func (l *library) batchToggle(c *ui.Context) {
	a := l.a
	btn := a.toolButton(c, "batch", l.batch).Label("批量选择")
	name := "checkbox-multiple-marked-outline"
	if l.batch {
		name = "close-circle-outline"
	}
	btn.OnClick(func() {
		l.batch = !l.batch
		if !l.batch {
			clear(l.selected)
		}
	}).Children(func() {
		icon(c, a.icon(name), 18, a.toolButtonColor(l.batch))
	})
}

func (l *library) filterButton(c *ui.Context) {
	a := l.a
	p := a.p
	count := l.activeFilterCount()
	btn := a.toolButton(c, "filters", false).Label(fmt.Sprintf("筛选 (%d)", count))
	color := brand300
	if !p.dark {
		color = brand700
	}
	btn.OnClick(func() { l.drawer.open = true }).Children(func() {
		ui.Box(c).Size(20, 20).Center().Shrink(0).Children(func() {
			icon(c, a.icon("filter-variant"), 18, color)
			if count > 0 {
				ring := brand700
				if !p.dark {
					ring = brand150
				}
				ui.Box(c).Absolute().Top(0).Right(0).Size(8, 8).Radius(999).Background(primary500).Shadow(0, 0, 0, 2, ring)
			}
		})
		text(c, "筛选", textSM, weightNormal, color).NoWrap()
	})
}

func (l *library) addGameButton(c *ui.Context) {
	a := l.a
	p := a.p
	btn := ui.ButtonBase(c.Key("add-game")).Padding(8, 16).Radius(8).AlignItems(ui.Center).FocusRing(false).Label("添加游戏")
	bg := p.primaryBtn
	if btn.Hovered() {
		bg = p.primaryBtnHover
	}
	btn.Background(bg).OnClick(func() { l.addMenuOpen = !l.addMenuOpen }).Children(func() {
		icon(c, a.icon("plus"), 18, white).Margin(0, 8, 0, 0)
		text(c, "添加游戏", textSM, weightMedium, white).NoWrap()
		icon(c, a.icon("chevron-down"), 18, white).Margin(0, 0, 0, 8)
	})

	im := a.assets.importers
	soon := func(what string) func() { return func() { a.toast(c, "原型未迁移"+what) } }
	a.dropdownMenu(c, btn, &l.addMenuOpen, 220, []menuItem{
		{key: "manual", label: "手动添加", description: "选择可执行文件并搜索元数据", icon: "gamepad-variant", iconColor: neutral500, onClick: soon("手动添加")},
		{key: "batch", label: "批量导入", description: "扫描游戏库目录批量添加", icon: "folder-multiple", iconColor: success500, onClick: soon("批量导入")},
		{key: "potatovn", label: "从 PotatoVN 导入", description: "导入 PotatoVN 导出的 ZIP 文件", image: im["potatovn"], dividerBefore: true, onClick: soon("导入流程")},
		{key: "playnite", label: "从 Playnite 导入", description: "导入 Playnite 导出的 ZIP 或 JSON 文件", image: im["playnite"], onClick: soon("导入流程")},
		{key: "yukihub", label: "从 YukiHub 导入", description: "导入 YukiHub 导出的 YKBAK 备份", image: im["yukihub"], onClick: soon("导入流程")},
		{key: "vnite", label: "从 Vnite 导入", description: "导入 Vnite 导出的本地数据库目录", image: im["vnite"], onClick: soon("导入流程")},
		{key: "reinamanager", label: "从 ReinaManager 导入", description: "导入 ReinaManager 的 SQLite 数据库备份", image: im["reinamanager"], onClick: soon("导入流程")},
		{key: "steam", label: "从 Steam 导入", description: "扫描本机已安装的 Steam 游戏", icon: "steam", iconColor: slate500, onClick: soon("导入流程")},
	})
}

// batchBar 是批量模式下的第二行：全选 / 清空 / 已选数量 / 帮助 / 批量操作。
func (l *library) batchBar(c *ui.Context) {
	a := l.a
	p := a.p
	bar := ui.Row(c.Key("batch-bar")).Wrap().AlignItems(ui.Center).Gap(8).Padding(8, 12).Radius(8).
		Border(a.hl, brand700.Alpha(0.5)).
		LinearGradient(ui.LinearGradient{From: brand800.Alpha(0.5), To: brand900.Alpha(0.3), Angle: 90})
	if !p.dark {
		bar.BorderColor(brand200).LinearGradient(ui.LinearGradient{From: brand50, To: brand50.Alpha(0.5), Angle: 90})
	}
	bar.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			l.batchPill(c, "select-all", "全选", true).OnClick(func() {
				for _, g := range l.games {
					l.selected[g.ID] = true
				}
			})
			l.batchPill(c, "clear-selection", "清空", true).OnClick(func() { clear(l.selected) })
		})
		l.batchPill(c, "selected-count", "", false).Children(func() {
			text(c, "已选", textXS, weightMedium, p.text)
			textMargin(c, fmt.Sprint(len(l.selected)), textXS, weightSemibold, p.text, 0, 0, 0, 4)
		})
		help := ui.ButtonBase(c.Key("batch-help")).Size(28, 28).Radius(999).Center().FocusRing(false).Label("批量选择帮助")
		help.Children(func() { icon(c, a.icon("help-circle-outline"), 18, p.textMuted) })
		a.tooltip(c, help, "(按住右键)滑动游戏可以多选哦~")

		ui.Row(c).Gap(6).Margin(0, 0, 0, ui.Auto).Children(func() {
			none := len(l.selected) == 0
			l.batchAction(c, "steam", "steam", "批量导入 Steam", none, p.navText)
			setStatus := l.batchAction(c, "set-status", "tag-edit-outline", "设为状态", none, p.navText)
			setStatus.OnClick(func() { l.statusMenu = !none && !l.statusMenu })
			l.statusDropdown(c, setStatus)
			l.batchAction(c, "add-category", "folder-plus-outline", "批量添加到收藏", none, p.navText)
			l.batchAction(c, "delete", "delete", "批量删除游戏", none, error400)
		})
	})
}

func (l *library) batchPill(c *ui.Context, key, label string, clickable bool) ui.Element {
	p := l.a.p
	pill := ui.ButtonBase(c.Key(key)).Padding(4, 10).Radius(6).Border(l.a.hl, brand600).Background(brand700.Alpha(0.6)).
		AlignItems(ui.Center).FocusRing(false)
	if !p.dark {
		pill.Background(white).BorderColor(brand200)
	}
	if clickable && pill.Hovered() {
		pill.Background(brand700)
		if !p.dark {
			pill.Background(brand50)
		}
	}
	if label != "" {
		pill.Children(func() { text(c, label, textXS, weightMedium, p.navText) })
	}
	return pill
}

func (l *library) batchAction(c *ui.Context, key, iconName, label string, disabled bool, color ui.Color) ui.Element {
	a := l.a
	p := a.p
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 12).Radius(8).Border(a.hl, p.panelBorder).Background(p.panelBg).
		AlignItems(ui.Center).FocusRing(false).Label(label)
	if disabled {
		btn.Opacity(0.5)
	} else {
		if btn.Hovered() {
			btn.Background(brand700)
		}
		if key != "set-status" {
			btn.OnClick(func() { a.toast(c, "原型未接入批量操作："+label) })
		}
	}
	btn.Children(func() { icon(c, a.icon(iconName), 18, color) })
	a.tooltip(c, btn, label)
	return btn
}

// statusDropdown 是“设为状态”的胶囊菜单（BetterDropdownMenu 的 pill 样式）。
func (l *library) statusDropdown(c *ui.Context, anchor ui.Element) {
	a := l.a
	p := a.p
	ui.PopoverBase(c, anchor, &l.statusMenu, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight).Top(6).MinWidth(130).Padding(6).Gap(4).
			Radius(12).Background(p.panelBg).Border(a.hl, p.panelBorder)
		panel.Children(func() {
			text(c, "设为状态", textXS, weightMedium, p.textFaint).Padding(2, 8, 4, 8)
			for _, o := range statusOptions[1:] {
				bg, fg := statusPillColors(o.value)
				item := ui.ButtonBase(c.Key(o.value)).FillWidth().Padding(6, 10).Radius(999).Gap(6).
					AlignItems(ui.Center).Justify(ui.Start).Background(bg).FocusRing(false)
				item.OnClick(func() {
					l.statusMenu = false
					a.toast(c, fmt.Sprintf("原型未接入批量操作：设为「%s」", o.label))
				}).Children(func() {
					text(c, o.label, textXS, weightMedium, fg)
				})
			}
		})
	})
}

func statusPillColors(s enums.GameStatus) (bg, fg ui.Color) {
	switch s {
	case enums.StatusWantToPlay:
		return ui.Hex("#0c181d"), ui.Hex("#7dd3fc")
	case enums.StatusPlaying:
		return ui.Hex("#0f172a"), ui.Hex("#cbd5e1")
	case enums.StatusCompleted:
		return ui.Hex("#713f12"), ui.Hex("#fde047")
	case enums.StatusOnHold:
		return ui.Hex("#7c2d12"), ui.Hex("#fdba74")
	case enums.StatusDropped:
		return ui.Hex("#881337"), ui.Hex("#fda4af")
	}
	return ui.Hex("#374151"), ui.Hex("#d1d5db")
}
