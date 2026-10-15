package edit

import (
	"fmt"
	"slices"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
)

// deleteModal 是页面的删除确认（ConfirmModal type="danger"）。
func (e *editor) deleteModal(c *ui.Context) {
	components.ConfirmModal(c, e.t, "delete-game", components.ConfirmProps{
		Open:        &e.s.deleteOpen,
		Title:       "删除游戏",
		Message:     fmt.Sprintf("确定要删除游戏 \"%s\" 吗？此操作将从库中移除该游戏，但不会删除本地游戏文件。", e.g.Name),
		ConfirmText: "确认删除",
		Danger:      true,
		TopOffset:   panel.TopOffset,
		OnConfirm:   func() { e.deleteGame(c) },
	})
}

// fieldLabels 是 metadataUpdateFields.<field>.label / hint。
var fieldLabels = map[enums.MetadataUpdateField][2]string{
	enums.MetadataUpdateFieldName:        {"游戏名称", "使用远端条目标题覆盖本地名称"},
	enums.MetadataUpdateFieldAliases:     {"游戏别名", "合并远端别名并保留手动添加的别名"},
	enums.MetadataUpdateFieldCover:       {"封面", "更新封面地址，并在更新后缓存图片"},
	enums.MetadataUpdateFieldCompany:     {"开发商", "更新厂商或品牌信息"},
	enums.MetadataUpdateFieldSummary:     {"简介", "使用远端简介覆盖本地简介"},
	enums.MetadataUpdateFieldRating:      {"评分", "更新远端评分"},
	enums.MetadataUpdateFieldReleaseDate: {"发售日期", "更新发售日期"},
	enums.MetadataUpdateFieldTags:        {"自动标签", "替换刮削来源标签，保留手动标签"},
}

// fieldSelectModal 对应 MetadataFieldSelectModal（单个游戏）：max-w-2xl rounded-xl border p-6 shadow-xl，
// 标题与说明、右上角关闭；“更新全部字段”开关；sm:grid-cols-2 gap-3 的字段开关；取消与“开始更新”。
func (e *editor) fieldSelectModal(c *ui.Context) {
	t, s := e.t, e.s
	all := enums.DefaultMetadataUpdateFields
	closeModal := func() { s.fieldsOpen = false }
	components.Modal(c, t, "metadata-fields", components.ModalProps{Open: s.fieldsOpen, TopOffset: panel.TopOffset, MaxWidth: 672},
		func(pnl ui.Element) {
			components.ModalPanel(t, pnl).Children(func() {
				// flex items-start justify-between gap-4
				ui.Row(c).Gap(16).AlignItems(ui.Start).Justify(ui.SpaceBetween).Children(func() {
					ui.Column(c).MinWidth(0).Gap(8).Children(func() {
						components.Text(c, "选择要更新的字段", theme.TextXL, theme.WeightBold, t.TextStrong)
						components.Text(c, "只会从远端覆盖你勾选的字段，未勾选的本地内容会保留。", theme.FontStep{Size: 14, Line: 22.75},
							theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
					})
					modalCloseButton(c, t, "close", "关闭", 8, 20, false, closeModal)
				})
				// mt-5 rounded-lg border border-brand-200 bg-brand-50 p-3 dark:border-brand-700 dark:bg-brand-700/40
				ui.Row(c.Key("all")).Margin(20, 0, 0, 0).Gap(16).Padding(12).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
					Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.4))).
					Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							components.Text(c, "更新全部字段", theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand200))
							components.Text(c, fmt.Sprintf("已选择 %d / %d 个字段", len(s.fields), len(all)), theme.TextXS,
								theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
						})
						components.Switch(c, t, "switch", len(s.fields) == len(all), false, func(on bool) {
							s.fields = nil
							if on {
								s.fields = slices.Clone(all)
							}
						})
					})
				cols := 1
				if e.winW >= 640 {
					cols = 2
				}
				ui.Grid(c.Key("fields")).Columns(cols).GapX(12).GapY(12).Margin(16, 0, 0, 0).Children(func() {
					for _, f := range all {
						fieldToggle(c.Key(string(f)), t, fieldLabels[f], slices.Contains(s.fields, f), func(on bool) {
							s.fields = slices.DeleteFunc(slices.Clone(s.fields), func(x enums.MetadataUpdateField) bool { return x == f })
							if on {
								s.fields = append(s.fields, f)
							}
						})
					}
				})
				if len(s.fields) == 0 {
					// mt-3 text-xs text-error-600 dark:text-error-400
					components.TextMargin(c, "请至少选择一个要更新的字段。", theme.TextXS, theme.WeightNormal,
						t.Pick(theme.Error600, theme.Error400), 12, 0, 0, 0)
				}
				ui.Row(c).Margin(32, 0, 0, 0).Gap(12).Justify(ui.End).Children(func() {
					components.Button(c, t, "cancel", components.ButtonProps{Label: "取消"}).OnClick(closeModal)
					components.Button(c, t, "confirm", components.ButtonProps{
						Variant: components.ButtonPrimary, Icon: "cloud-sync", Label: "开始更新",
						Disabled: len(s.fields) == 0 || s.updating, Loading: s.updating,
					}).OnClick(func() { e.updateFromRemote(c, s.fields) })
				})
			})
		})
}

// fieldToggle：flex min-w-0 items-center justify-between gap-3 rounded-lg border p-3，左侧 label（点击也切换）是 text-sm
// font-medium 的名称与 text-xs leading-relaxed 的说明（space-y-1）。
func fieldToggle(c *ui.Context, t *theme.Theme, text [2]string, checked bool, onChange func(bool)) {
	ui.Row(c).MinWidth(0).Gap(12).Padding(12).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		lb := ui.ButtonBase(c.Key("label")).Column().Grow(1).Basis(0).MinWidth(0).Gap(4).AlignItems(ui.Start).
			FocusRing(false).Label(text[0])
		lb.OnClick(func() { onChange(!checked) }).Children(func() {
			components.Text(c, text[0], theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand200))
			components.Text(c, text[1], theme.FontStep{Size: 12, Line: 19.5}, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		components.Switch(c, t, "switch", checked, false, onChange)
	})
}

// modalCloseButton 是弹窗右上角的关闭按钮：rounded-lg p-{pad} text-brand-500 hover:bg-brand-100 hover:text-brand-800
// dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-100。
func modalCloseButton(c *ui.Context, t *theme.Theme, key, label string, pad, size float32, disabled bool, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).Padding(pad).Radius(8).Center().Shrink(0).FocusRing(false).Label(label)
	color := t.Pick(theme.Brand500, theme.Brand400)
	if disabled {
		components.Disable(btn, 0.6)
	} else if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
		color = t.Pick(theme.Brand800, theme.Brand100)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	btn.OnClick(onClick).Children(func() { components.Icon(c, "close", size, color) })
}

// searchModal 对应 MetadataSourceSearchModal：max-w-2xl rounded-xl bg-white p-6 shadow-2xl dark:bg-brand-800，
// text-4xl 的标题与关闭按钮，下面是 MetadataSearchResultsStep。应用结果时（isApplying）按钮都禁用。
func (e *editor) searchModal(c *ui.Context) {
	t, s := e.t, e.s
	closeModal := func() { s.searchOpen, s.helpOpen = false, false }
	components.Modal(c, t, "metadata-search", components.ModalProps{Open: s.searchOpen, TopOffset: panel.TopOffset, MaxWidth: 672},
		func(pnl ui.Element) {
			pnl.Radius(12).Padding(24).Background(t.PanelBg).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Children(func() {
				// mb-2 flex items-center justify-between gap-4
				ui.Row(c).Margin(0, 0, 8, 0).Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					components.Text(c, "重新选择元数据来源", theme.Text4XL, theme.WeightBold, t.TextStrong)
					// i-mdi-close rounded-lg p-1 text-2xl：按钮本身就是图标，悬停只变色。
					btn := ui.ButtonBase(c.Key("close")).Size(32, 32).Center().Shrink(0).FocusRing(false).Label("取消")
					color := t.Pick(theme.Brand500, theme.Brand400)
					if s.applying {
						components.Disable(btn, 0.6)
					} else if btn.Hovered() {
						color = t.Pick(theme.Brand700, theme.Brand200)
					}
					btn.OnClick(closeModal).Children(func() { components.Icon(c, "close", 32, color) })
				})
				e.searchResultsStep(c, closeModal)
			})
		})
}

// searchResultsStep 对应 MetadataSearchResultsStep（space-y-6）：提示与选择说明，横向滚动的候选卡片（两侧翻页按钮），
// 没有结果时的空状态，以及 border-t pt-4 的“← 返回”。
func (e *editor) searchResultsStep(c *ui.Context, closeModal func()) {
	t, s := e.t, e.s
	ui.Column(c.Key("results-step")).Gap(24).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			components.Text(c, "哪些结果是您期望的？", theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)).NoWrap()
			e.selectionHelp(c)
		})
		if len(s.results) == 0 {
			// flex min-h-64 flex-col items-center justify-center gap-3 text-brand-500 dark:text-brand-400
			color := t.Pick(theme.Brand500, theme.Brand400)
			ui.Column(c.Key("empty")).MinHeight(256).Center().Gap(12).Children(func() {
				components.Icon(c, "database-search-outline", 36, color)
				components.Text(c, "没有找到匹配的元数据", theme.TextSM, theme.WeightNormal, color)
			})
		} else {
			e.resultCards(c)
		}
		// flex items-center justify-between border-t border-brand-200 pt-4 dark:border-brand-700
		ui.Row(c.Key("footer")).Padding(16, 0, 0, 0).BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Divider).Children(func() {
			back := ui.ButtonBase(c.Key("back")).FocusRing(false).Label("返回")
			color := t.Pick(theme.Brand500, theme.Brand400)
			if s.applying {
				components.Disable(back, 0.6)
			} else if back.Hovered() {
				color = t.Pick(theme.Brand700, theme.Brand200)
			}
			back.OnClick(closeModal).Children(func() {
				components.Text(c, "←返回", theme.TextSM, theme.WeightNormal, color).NoWrap()
			})
		})
	})
}

// selectionHelp 是“查看候选结果选择说明”：h-7 w-7 rounded-full 的问号按钮，点开 w-80 rounded-xl border p-4 shadow-xl 的说明。
func (e *editor) selectionHelp(c *ui.Context) {
	t, s := e.t, e.s
	btn := ui.ButtonBase(c.Key("help")).Size(28, 28).Radius(999).Center().Shrink(0).FocusRing(false).Label("查看候选结果选择说明")
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() || s.helpOpen {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
		color = t.Pick(theme.Brand700, theme.Brand200)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	btn.OnClick(func() { s.helpOpen = !s.helpOpen }).Children(func() {
		components.Icon(c, "help-circle-outline", 18, color)
	})
	steps := []string{
		"点击卡片右上角的关闭按钮，移除错误的刮削结果。",
		"每个数据源仅保留一个正确结果。",
		"点击准备导入的卡片，该来源会作为默认展示来源，其余保留结果会一并关联到游戏。",
	}
	ui.PopoverBase(c, btn, &s.helpOpen, func(pnl ui.Element) {
		pnl.AttachTo(btn, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(8).Width(min(320, e.winW-32)).Padding(16).Radius(12).
			Background(t.PanelBg).Border(t.HL, t.PanelBorder).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		pnl.Children(func() {
			components.Text(c, "如何选择候选结果", theme.TextSM, theme.WeightSemibold, t.TextStrong)
			// mt-3 space-y-2.5 text-sm leading-5
			ui.Column(c).Margin(12, 0, 0, 0).Gap(10).Children(func() {
				for i, step := range steps {
					ui.Row(c.Key(i)).Gap(10).AlignItems(ui.Start).Children(func() {
						ui.Box(c).Size(20, 20).Radius(999).Center().Shrink(0).Background(t.Pick(theme.Brand100, theme.Brand700)).Children(func() {
							components.Text(c, fmt.Sprint(i+1), theme.TextXS, theme.WeightSemibold, t.Pick(theme.Brand700, theme.Brand200))
						})
						components.Text(c, step, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)).Grow(1).Basis(0).MinWidth(0)
					})
				}
			})
		})
	})
}

// resultCards：relative 容器里 flex snap-x gap-4 overflow-x-auto p-2 pb-6 pt-2 的卡片（隐藏滚动条），
// 能向前/向后滚动时两侧显示 BetterEdgeIconButton，每次滚动 max(宽度 - 96, 240)。
func (e *editor) resultCards(c *ui.Context) {
	t, s := e.t, e.s
	st := &s.resultsScroll
	cardW := float32(144) // w-36 sm:w-40
	if e.winW >= 640 {
		cardW = 160
	}
	wrap := ui.Box(c.Key("results"))
	viewW := wrap.Bounds().W
	wrap.Children(func() {
		ui.ScrollHorizontal(c.Key("scroller")).FillWidth().TrackScroll(st).Children(func() {
			ui.Row(c).Gap(16).Padding(8, 8, 24, 8).AlignItems(ui.Start).Children(func() {
				for i, item := range s.results {
					e.resultCard(c.Key(string(item.Source)+":"+item.Game.SourceID+fmt.Sprint(":", i)), t, item, i, cardW)
				}
			})
		})
		step := max(viewW-96, 240)
		if st.X > 2 {
			edgeButton(c, t, "prev", "chevron-left", "向前查看更多结果", true, func() { st.ScrollTo(st.X-step, 0, ui.Smooth) })
		}
		if st.X < st.MaxX-2 {
			edgeButton(c, t, "next", "chevron-right", "向后查看更多结果", false, func() { st.ScrollTo(st.X+step, 0, ui.Smooth) })
		}
	})
}

// resultCard：relative w-36 shrink-0 rounded-xl border border-brand-200 bg-brand-50/50 shadow-sm hover:border-brand-400
// hover:shadow-md dark:border-brand-700 dark:bg-brand-800/50 dark:hover:border-brand-500。里面 p-3 的按钮：aspect-[3/4]
// rounded-md 的封面、mt-2 text-sm font-bold 的名称与 text-xs 的来源；右上角 -right-3 -top-3 的移除按钮。
func (e *editor) resultCard(c *ui.Context, t *theme.Theme, item vo.GameMetadataFromWebVO, index int, width float32) {
	s := e.s
	card := ui.Column(c).Width(width).Shrink(0).Radius(12).Background(t.Pick(theme.Brand50.Alpha(0.5), theme.Brand800.Alpha(0.5)))
	hovered := card.Hovered()
	border := t.Pick(theme.Brand200, theme.Brand700)
	if hovered {
		border = t.Pick(theme.Brand400, theme.Brand500)
		card.Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
	} else {
		card.Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 1, 2, -1, ui.RGBA(0, 0, 0, 0.1))
	}
	card.Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond))
	source := resultSourceLabel(item.Source)
	card.Children(func() {
		btn := ui.ButtonBase(c.Key("select")).Column().FillWidth().Padding(12).AlignItems(ui.Stretch).FocusRing(false).
			Cursor(ui.CursorPointer).Label(item.Game.Name)
		if s.applying {
			components.Disable(btn, 0.6)
		}
		btn.OnClick(func() { e.selectSearchResult(c, item) }).Children(func() {
			ui.Box(c).FillWidth().AspectRatio(3.0 / 4).Radius(6).Clip().Center().Background(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
				g := item.Game
				switch {
				case g.CoverURL == "" && g.CoverSourceURL == "":
					components.Icon(c, "image-off", 36, theme.Brand400)
				case e.p.Env.Covers == nil:
				default:
					if bmp := e.p.Env.Covers.Get(&g); bmp != nil {
						ui.Image(c, bmp).Fill().Fit(ui.Cover)
					} else if e.p.Env.Covers.Missing(&g) {
						components.Icon(c, "image-off", 36, theme.Brand400)
					}
				}
			})
			components.TextMargin(c, item.Game.Name, theme.TextSM, theme.WeightBold, t.TextStrong, 8, 0, 0, 0).SingleLine()
			components.Text(c, "来自 "+source, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).SingleLine()
		})
		// group absolute -right-3 -top-3 z-10 flex h-11 w-11：里面 h-8 w-8 rounded-full border-2 border-white bg-brand-300/85
		// text-white shadow-md backdrop-blur-sm group-hover:bg-error-600 dark:border-brand-700 dark:bg-brand-900/90。
		rm := ui.ButtonBase(c.Key("remove")).Absolute().Top(-12).Right(-12).Size(44, 44).Center().FocusRing(false).
			Label("移除 " + source + " 的这个结果")
		if s.applying {
			components.Disable(rm, 0.6)
		}
		bg := t.Pick(theme.Brand300.Alpha(0.85), theme.Brand900.Alpha(0.9))
		if rm.Hovered() && !s.applying {
			bg = theme.Error600
		}
		rm.OnClick(func() {
			s.results = slices.Delete(slices.Clone(s.results), index, index+1)
		}).Children(func() {
			ui.Box(c).Size(32, 32).Radius(999).Center().Background(bg).Border(2, t.Pick(theme.White, theme.Brand700)).
				Material(glass.Blur{Radius: 4}).Transition(theme.ColorsTransition(150*time.Millisecond)).
				Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1)).Children(func() {
				components.Icon(c, "close", 18, theme.White)
			})
		})
	})
}

// edgeButton 对应 BetterEdgeIconButton placement="left|right"：absolute top-1/2 -translate-y-1/2 的 h-16 w-10，
// 贴边一侧不圆角（rounded-r-xl / rounded-l-xl），border border-white/30 bg-white/40 hover:bg-white/65
// dark:border-white/15 dark:bg-black/35 dark:hover:bg-black/55 backdrop-blur-md，opacity-75 hover:opacity-100，图标 text-3xl。
func edgeButton(c *ui.Context, t *theme.Theme, key, icon, label string, left bool, onClick func()) {
	holder := ui.Column(c.Key(key)).Absolute().Top(0).Bottom(0).Justify(ui.Center).PassThrough()
	if left {
		holder.Left(0)
	} else {
		holder.Right(0)
	}
	holder.Children(func() {
		btn := ui.ButtonBase(c.Key("btn")).Size(40, 64).Center().FocusRing(false).Label(label).
			Material(glass.Blur{Radius: 12}).Border(t.HL, t.Pick(theme.White.Alpha(0.3), theme.White.Alpha(0.15)))
		if left {
			btn.Radius(0, 12, 12, 0)
		} else {
			btn.Radius(12, 0, 0, 12)
		}
		hovered := btn.Hovered()
		bg, opacity := t.Pick(theme.White.Alpha(0.4), theme.Black.Alpha(0.35)), float32(0.75)
		if hovered {
			bg, opacity = t.Pick(theme.White.Alpha(0.65), theme.Black.Alpha(0.55)), 1
		}
		scale := float32(1)
		if btn.Pressed() {
			scale = 0.95
		}
		sc := btn.AnimateWith("press", scale, 200*time.Millisecond, theme.EaseTailwind)
		btn.Background(bg).Opacity(btn.AnimateWith("opacity", opacity, 200*time.Millisecond, theme.EaseTailwind)).Scale(sc, sc).
			Transition(theme.ColorsTransition(200 * time.Millisecond))
		btn.OnClick(onClick).Children(func() {
			components.Icon(c, icon, 30, t.Pick(theme.Brand700, theme.Brand200))
		})
	})
}
