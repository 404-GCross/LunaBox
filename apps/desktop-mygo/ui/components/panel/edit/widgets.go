package edit

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/theme"
)

// fieldLabel：block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1。
func fieldLabel(c *ui.Context, t *theme.Theme, s string) ui.Element {
	return components.TextMargin(c, s, theme.TextSM, theme.WeightMedium, t.SectionLabel, 0, 0, 4, 0)
}

// hint：mt-1 text-xs text-brand-500；darkLighter 时另有 dark:text-brand-400（原版各处写法不一）。
func hint(c *ui.Context, t *theme.Theme, s string, darkLighter bool) ui.Element {
	color := theme.Brand500
	if darkLighter {
		color = t.Pick(theme.Brand500, theme.Brand400)
	}
	return components.TextMargin(c, s, theme.TextXS, theme.WeightNormal, color, 4, 0, 0, 0)
}

// blurred 报告输入框在这一遍构建里失去了焦点（onBlur）。
func blurred(in ui.Element) bool {
	was := ui.Local(in, "was-focused", func() bool { return false })
	focused := in.Focused()
	lost := *was && !focused
	*was = focused
	return lost
}

// inputAction 是 BetterActionInput 的 actions 之一。
type inputAction struct {
	Icon     string
	Label    string // aria-label，也是提示
	Disabled bool
	Loading  bool // i-mdi-loading animate-spin
	OnClick  func()
}

// actionInputProps 是 BetterActionInput 的输入框属性。
type actionInputProps struct {
	Placeholder string
	Label       string                 // 无障碍名称，默认用 Placeholder
	OnPaste     func(text string) bool // onPaste
}

// actionInput 对应 BetterActionInput（同 better.ActionInput，另支持 onPaste 与加载中的旋转图标）：
// glass-input flex w-full min-w-0 items-center gap-1 rounded-md border border-brand-300 bg-white pr-1
// focus-within:ring-2 focus-within:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700；
// 输入框 min-w-0 flex-1 bg-transparent px-3 py-2；按钮 h-10 w-10 text-xl，hover:text-brand-900 dark:hover:text-brand-100，
// disabled:opacity-50。返回输入框，调用方用 Changed 观察编辑。
func actionInput(c *ui.Context, t *theme.Theme, key string, value *string, p actionInputProps, actions ...inputAction) ui.Element {
	row := ui.Row(c.Key(key)).FillWidth().MinWidth(0).Gap(4).Padding(0, 4, 0, 0).AlignItems(ui.Center).Radius(6).
		Background(t.Pick(theme.White, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	var in ui.Element
	row.Children(func() {
		in = ui.TextInputBase(c.Key("input"), value).Grow(1).Basis(0).MinWidth(0).Padding(8, 12).
			FontSize(theme.TextBase.Size).FixedLineHeight(theme.TextBase.Line).TextColor(t.TextStrong).
			Placeholder(p.Placeholder)
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		label := p.Label
		if label == "" {
			label = p.Placeholder
		}
		if label != "" {
			in.Label(label)
		}
		if p.OnPaste != nil {
			in.OnPaste(p.OnPaste)
		}
		for _, a := range actions {
			actionButton(c, t, a)
		}
	})
	if in.Focused() {
		row.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
	return in
}

// actionButton 是 actionInput 右侧的图标按钮（带 BetterTooltip）。
func actionButton(c *ui.Context, t *theme.Theme, a inputAction) {
	btn := ui.ButtonBase(c.Key(a.Label)).Size(40, 40).Center().Shrink(0).FocusRing(false).Label(a.Label)
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() && !a.Disabled {
		color = t.Pick(theme.Brand900, theme.Brand100)
	}
	color = btn.AnimateColor("text", color, theme.Tween(200*time.Millisecond))
	if a.Disabled {
		components.Disable(btn, 0.5)
	} else if a.OnClick != nil {
		btn.OnClick(a.OnClick)
	}
	btn.Children(func() {
		if a.Loading {
			components.Spinner(c, 20, color)
		} else {
			components.Icon(c, a.Icon, 20, color)
		}
	})
	better.Tooltip(c, t, btn, a.Label)
}

// tableColumn 是 BetterDataTableColumn。
type tableColumn[T any] struct {
	Key    string
	Header string
	// Center 对应 cellClassName 的 text-center。内容比格子宽时（ContentWidth 大于格子的内容宽度），
	// 与 CSS 一样从左边开始、向右溢出。表头的 headerClassName="text-center" 在原版里被基础样式里
	// 更靠后的 text-left 盖掉，所以表头总是左对齐。
	Center       bool
	ContentWidth float32
	Cell         func(row T, index int)
}

// dataTable 是 BetterDataTable 的属性（原型只用到行、列与空状态；列筛选与多选没有用到）。
type dataTable[T any] struct {
	Key       string
	Rows      []T
	Columns   []tableColumn[T] // table-fixed：各列等宽（日历的 w-[14.285%]）
	RowKey    func(row T, index int) string
	Empty     func()
	MaxHeight float32 // max-h-*，0 为 max-h-none
}

// view 对应 BetterDataTable：overflow-auto rounded-lg border border-brand-200 bg-white/80 dark:border-brand-700
// dark:bg-brand-900/50；表头 sticky bg-brand-50/95 dark:bg-brand-800/95，th 是 border-b px-3 py-2 text-xs font-semibold
// text-brand-600 dark:text-brand-300；td 是 px-3 py-2 text-sm text-brand-700 dark:text-brand-300，行 hover:bg-brand-50/80
// dark:hover:bg-brand-800/70。表格是 border-separate，tbody 的 divide-y 画在 tr 上，不显示。
func (d dataTable[T]) view(c *ui.Context, t *theme.Theme) {
	box := ui.Column(c.Key(d.Key)).FillWidth().Radius(8).Clip().
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Background(t.Pick(theme.White.Alpha(0.8), theme.Brand900.Alpha(0.5)))
	box.Children(func() {
		if len(d.Rows) == 0 {
			// p-8 text-center text-sm text-brand-400 dark:text-brand-500
			ui.Column(c).Padding(32).AlignItems(ui.Center).Children(func() {
				if d.Empty != nil {
					d.Empty()
				}
			})
			return
		}
		body := func() {
			for i, row := range d.Rows {
				tr := ui.Row(c.Key(d.RowKey(row, i))).AlignItems(ui.Stretch)
				if tr.Hovered() {
					tr.Background(t.Pick(theme.Brand50.Alpha(0.8), theme.Brand800.Alpha(0.7)))
				}
				tr.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
					for _, col := range d.Columns {
						td := ui.Row(c.Key(col.Key)).Grow(1).Basis(0).MinWidth(0).Padding(8, 12).AlignItems(ui.Center)
						if col.Center && (col.ContentWidth == 0 || td.Bounds().W-24 >= col.ContentWidth) {
							td.Justify(ui.Center)
						}
						td.Children(func() { col.Cell(row, i) })
					}
				})
			}
		}
		header := func() {
			ui.Row(c.Key("thead")).Background(t.Pick(theme.Brand50.Alpha(0.95), theme.Brand800.Alpha(0.95))).Children(func() {
				for _, col := range d.Columns {
					ui.Row(c.Key(col.Key)).Grow(1).Basis(0).MinWidth(0).Padding(8, 12).
						BorderWidth(0, 0, t.HL, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
						components.Text(c, col.Header, theme.TextXS, theme.WeightSemibold, t.Pick(theme.Brand600, theme.Brand300)).SingleLine()
					})
				}
			})
		}
		header()
		if d.MaxHeight <= 0 {
			body()
			return
		}
		// 表头 sticky：放在滚动区外面。
		ui.Scroll(c.Key("scroll")).MaxHeight(d.MaxHeight - 32 - t.HL).Children(body)
	})
}
