// Package theme 是原版前端的设计令牌：uno.config.ts 的色板、Tailwind 的字号阶梯与过渡曲线，
// 以及 Chromium 把边框吸附到设备像素的规则。数值都用 tools/parity 从运行中的原版导出的计算样式校对过。
package theme

import "github.com/egoist/mygo/ui"

// uno.config.ts 的色板。
var (
	Brand50  = ui.Hex("#fdfdfd")
	Brand100 = ui.Hex("#f3f4f6")
	Brand150 = ui.Hex("#ecedf0")
	Brand200 = ui.Hex("#e5e7eb")
	Brand300 = ui.Hex("#d1d5db")
	Brand400 = ui.Hex("#9ca3af")
	Brand500 = ui.Hex("#6b7280")
	Brand600 = ui.Hex("#4b5563")
	Brand700 = ui.Hex("#44484e")
	Brand750 = ui.Hex("#303235")
	Brand800 = ui.Hex("#1c1e1f")
	Brand900 = ui.Hex("#121416")

	Neutral50  = ui.Hex("#f8fafc")
	Neutral100 = ui.Hex("#f1f5f9")
	Neutral300 = ui.Hex("#cbd5e1")
	Neutral400 = ui.Hex("#94a3b8")
	Neutral500 = ui.Hex("#64748b")
	Neutral600 = ui.Hex("#475569")
	Neutral700 = ui.Hex("#334155")

	Success500 = ui.Hex("#10b981")
	Error400   = ui.Hex("#f87171")
	Error500   = ui.Hex("#ef4444")
	Error600   = ui.Hex("#dc2626")
	Error700   = ui.Hex("#b91c1c")
	Primary500 = ui.Hex("#7c6aef")
	Slate500   = ui.Hex("#64748b")
	Red500     = ui.Hex("#ef4444")
	Yellow500  = ui.Hex("#eab308")
	White      = ui.Hex("#ffffff")
	Black      = ui.Hex("#000000")
)

// Palette 是组件里 `x dark:y` 两套取值在当前主题下的结果。
type Palette struct {
	Dark bool

	Text, TextStrong, TextMuted, TextFaint ui.Color // brand-900/100, 900/white, 500/400, 400/500

	AppBg                  ui.Color // main: bg-brand-100 dark:bg-brand-900
	TopbarBg, TopbarBorder ui.Color
	ControlIcon            ui.Color // 窗口按钮图标 text-brand-600 dark:text-brand-400
	ControlHover           ui.Color

	SidebarBg, SidebarBorder ui.Color
	NavText, NavHover        ui.Color // text-brand-700 dark:text-brand-300 / hover:bg-brand-100 dark:bg-brand-700
	NavActiveBg, NavActive   ui.Color

	InputBg, InputBorder, InputFocus, InputIcon, Placeholder ui.Color

	ToolBtnBg, ToolBtnBorder, ToolBtnHover, ToolBtnIcon, ToolBtnText ui.Color // glass-btn-neutral
	ToolBtnActiveBg, ToolBtnActiveBorder, ToolBtnActiveText          ui.Color

	PrimaryBtn, PrimaryBtnHover ui.Color

	CardBg, CardBorder, CardBorderHover, CoverBg ui.Color

	ScrollThumb, ScrollThumbHover ui.Color

	PanelBg, PanelBorder, Divider ui.Color // 下拉菜单、抽屉
	MenuHover, MenuText           ui.Color
	SectionLabel                  ui.Color // text-sm font-medium text-brand-700 dark:text-brand-300

	ChipBg, ChipText, ChipHover, ChipActiveBg, ChipActiveText, ChipRing ui.Color

	SelectBg, SelectBorder, SelectText ui.Color
	ListFocus, ListSelected            ui.Color
	SwitchOff                          ui.Color

	TooltipBg, TooltipText, TooltipBorder ui.Color
}

// NewPalette 返回浅色或深色主题的色板。
func NewPalette(dark bool) Palette {
	if dark {
		return Palette{
			Dark: true,
			Text: Brand100, TextStrong: White, TextMuted: Brand400, TextFaint: Brand500,
			AppBg:    Brand900,
			TopbarBg: Brand800, TopbarBorder: Brand700.Alpha(0.5),
			ControlIcon: Brand400, ControlHover: Brand700,
			SidebarBg: Brand800, SidebarBorder: Brand700,
			NavText: Brand300, NavHover: Brand700, NavActiveBg: Brand700, NavActive: Brand100,
			InputBg: Brand900, InputBorder: Brand700, InputFocus: Neutral500, InputIcon: Brand400, Placeholder: Brand400,
			ToolBtnBg: Brand700, ToolBtnBorder: Brand700, ToolBtnHover: Brand600, ToolBtnIcon: Brand300, ToolBtnText: Brand300,
			ToolBtnActiveBg: Brand600, ToolBtnActiveBorder: Brand500, ToolBtnActiveText: White,
			PrimaryBtn: Neutral600, PrimaryBtnHover: Neutral700,
			CardBg: Brand800, CardBorder: Brand700, CardBorderHover: Brand600, CoverBg: Brand700,
			ScrollThumb: Brand600, ScrollThumbHover: Brand500,
			PanelBg: Brand800, PanelBorder: Brand700, Divider: Brand700, MenuHover: Brand700, MenuText: Brand200,
			SectionLabel: Brand300,
			ChipBg:       Brand900.Alpha(0.5), ChipText: Brand400, ChipHover: Brand700.Alpha(0.7),
			ChipActiveBg: Brand700, ChipActiveText: Brand200, ChipRing: Brand500,
			SelectBg: Brand700, SelectBorder: Brand600, SelectText: White,
			ListFocus: Brand700, ListSelected: Brand750,
			SwitchOff: Brand750,
			TooltipBg: Brand100, TooltipText: Brand900, TooltipBorder: Brand200,
		}
	}
	return Palette{
		Text: Brand900, TextStrong: Brand900, TextMuted: Brand500, TextFaint: Brand400,
		AppBg:    Brand100,
		TopbarBg: Brand50, TopbarBorder: Brand200.Alpha(0.5),
		ControlIcon: Brand600, ControlHover: Brand200,
		SidebarBg: White, SidebarBorder: Brand200,
		NavText: Brand700, NavHover: Brand100, NavActiveBg: Brand200, NavActive: Brand900,
		InputBg: White, InputBorder: Brand300, InputFocus: Neutral600, InputIcon: Brand700, Placeholder: Brand700,
		ToolBtnBg: Brand150, ToolBtnBorder: Brand200, ToolBtnHover: Brand200, ToolBtnIcon: Brand600, ToolBtnText: Brand700,
		ToolBtnActiveBg: Brand200, ToolBtnActiveBorder: Brand300, ToolBtnActiveText: Brand900,
		PrimaryBtn: Neutral600, PrimaryBtnHover: Neutral700,
		CardBg: White, CardBorder: Brand100, CardBorderHover: Brand300, CoverBg: Brand200,
		ScrollThumb: ui.Hex("#cccccc"), ScrollThumbHover: ui.Hex("#aaaaaa"),
		PanelBg: White, PanelBorder: Brand200, Divider: Brand200, MenuHover: Brand100, MenuText: Brand700,
		SectionLabel: Brand700,
		ChipBg:       Brand50, ChipText: Brand500, ChipHover: Brand100,
		ChipActiveBg: Brand100, ChipActiveText: Brand700, ChipRing: Brand300,
		SelectBg: White, SelectBorder: Brand300, SelectText: Brand900,
		ListFocus: Neutral100, ListSelected: Neutral50,
		SwitchOff: Brand300,
		TooltipBg: Brand900, TooltipText: White, TooltipBorder: White.Alpha(0.1),
	}
}

// Pick 返回 `light dark:dark` 在当前主题下的取值，用于只出现一次的颜色。
func (p *Palette) Pick(light, dark ui.Color) ui.Color {
	if p.Dark {
		return dark
	}
	return light
}
