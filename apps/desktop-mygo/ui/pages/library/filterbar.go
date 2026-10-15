package library

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// filterBar 对应 components/bar/FilterBar.tsx（flex flex-wrap items-center justify-between gap-4 my-4）
// 加上 library.tsx 传给它的 batchActions、actionButton。
func (l *Page) filterBar(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("filterbar")).Margin(16, 0).Gap(16).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(16).Children(func() {
			l.searchInput(c, t)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
				l.randomButton(c, t)
				l.batchToggle(c, t)
				l.filterButton(c, t)
				l.addGameButton(c, t)
			})
		})
		if l.batch {
			l.batchBar(c, t)
		}
	})
}

// searchInput：relative flex-1 max-w-md，input p-2 pl-10 pr-10 text-sm rounded-lg border。
func (l *Page) searchInput(c *ui.Context, t *theme.Theme) {
	box := ui.Row(c.Key("search")).Grow(1).MaxWidth(448).AlignItems(ui.Center).Radius(8).
		Background(t.InputBg).Border(t.HL, t.InputBorder)
	box.Children(func() {
		ui.Box(c).Absolute().Top(0).Bottom(0).Left(12).Center().PassThrough().Children(func() {
			components.Icon(c, "magnify", 18, t.InputIcon)
		})
		input := ui.TextInputBase(c.Key("search-input"), &l.search).Grow(1).Padding(8, 40).
			FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.TextStrong).
			Placeholder("搜索游戏...").Label("搜索游戏")
		if input.Changed() {
			l.searchEditedAt = time.Now()
		}
		if input.Focused() {
			box.BorderColor(t.InputFocus)
		}
		if l.search != "" {
			clear := ui.ButtonBase(c.Key("clear")).Absolute().Top(0).Bottom(0).Right(6).Width(32).Center().
				Radius(6).FocusRing(false).Label("清除搜索")
			clear.OnClick(func() {
				l.search, l.debounced = "", ""
				l.Reload()
			}).Children(func() {
				components.Icon(c, "close-circle-outline", 18, t.Pick(theme.Brand500, theme.Brand400))
			})
		}
	})
}

func (l *Page) randomButton(c *ui.Context, t *theme.Theme) {
	disabled := l.loading || l.search != l.debounced || l.total == 0 || l.randomLoading
	btn := components.ToolButton(c, t, "random", false).Label("随机打开游戏")
	if disabled {
		btn.Opacity(0.5).Disabled(true)
	}
	btn.OnClick(func() { l.openRandomGame(c) }).Children(func() {
		color := components.ToolButtonColor(t, false)
		if l.randomLoading {
			components.Spinner(c, 18, color)
		} else {
			components.Icon(c, "dice-multiple-outline", 18, color)
		}
	})
}

// openRandomGame 随机取一个游戏。原版会打开详情页，原型以提示代替。
func (l *Page) openRandomGame(c *ui.Context) {
	if l.total == 0 {
		return
	}
	l.randomLoading = true
	req := l.request()
	req.Offset, req.Limit = rand.IntN(l.total), 1
	l.env.Async(func() func() {
		resp, err := l.env.Backend.GetGames(req)
		return func() {
			l.randomLoading = false
			if err != nil || len(resp.Games) == 0 {
				l.env.Toast(c, "随机选择游戏失败")
				return
			}
			l.env.Toast(c, "随机打开："+resp.Games[0].Name+"（原型未迁移详情页）")
		}
	})
}

func (l *Page) batchToggle(c *ui.Context, t *theme.Theme) {
	btn := components.ToolButton(c, t, "batch", l.batch).Label("批量选择")
	icon := "checkbox-multiple-marked-outline"
	if l.batch {
		icon = "close-circle-outline"
	}
	btn.OnClick(func() {
		l.batch = !l.batch
		if !l.batch {
			clear(l.selected)
		}
	}).Children(func() {
		components.Icon(c, icon, 18, components.ToolButtonColor(t, l.batch))
	})
}

func (l *Page) filterButton(c *ui.Context, t *theme.Theme) {
	count := l.activeFilterCount()
	btn := components.ToolButton(c, t, "filters", false).Label(fmt.Sprintf("筛选 (%d)", count))
	color := t.Pick(theme.Brand700, theme.Brand300)
	btn.OnClick(func() { l.drawerOpen = true }).Children(func() {
		ui.Box(c).Size(20, 20).Center().Shrink(0).Children(func() {
			components.Icon(c, "filter-variant", 18, color)
			if count > 0 {
				ui.Box(c).Absolute().Top(0).Right(0).Size(8, 8).Radius(999).Background(theme.Primary500).
					Shadow(0, 0, 0, 2, t.Pick(theme.Brand150, theme.Brand700))
			}
		})
		components.Text(c, "筛选", theme.TextSM, theme.WeightNormal, color).NoWrap()
	})
}

// addGameButton 是 library.tsx 的 actionButton：bg-neutral-600 px-4 py-2 text-sm font-medium 的触发按钮
// 和 min-w-[220px] 的 BetterDropdownMenu。
func (l *Page) addGameButton(c *ui.Context, t *theme.Theme) {
	btn := ui.ButtonBase(c.Key("add-game")).Padding(8, 16).Radius(8).AlignItems(ui.Center).FocusRing(false).Label("添加游戏")
	bg := t.PrimaryBtn
	if btn.Hovered() {
		bg = t.PrimaryBtnHover
	}
	btn.Background(bg).Children(func() {
		components.Icon(c, "plus", 18, theme.White).Margin(0, 8, 0, 0)
		components.Text(c, "添加游戏", theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
		components.Icon(c, "chevron-down", 18, theme.White).Margin(0, 0, 0, 8)
	})

	soon := func(what string) func() { return func() { l.env.Toast(c, "原型未迁移"+what) } }
	items := []components.MenuItem{
		{Key: "manual", Label: "手动添加", Description: "选择可执行文件并搜索元数据", Icon: "gamepad-variant", IconColor: theme.Neutral500, OnClick: soon("手动添加")},
		{Key: "batch", Label: "批量导入", Description: "扫描游戏库目录批量添加", Icon: "folder-multiple", IconColor: theme.Success500, OnClick: soon("批量导入")},
	}
	for i, imp := range importOptions {
		it := components.MenuItem{Key: imp.key, Label: imp.label, Description: imp.description, DividerBefore: i == 0, OnClick: soon("导入流程")}
		if imp.icon != "" {
			it.Icon, it.IconColor = imp.icon, theme.Slate500
		} else {
			it.Image = assets.Importer(imp.key)
		}
		items = append(items, it)
	}
	components.DropdownMenu(c, t, btn, components.MenuProps{Open: &l.addMenuOpen, MinWidth: 220, Items: items})
}
