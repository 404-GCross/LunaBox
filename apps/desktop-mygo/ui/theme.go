// Package ui 是 LunaBox 前端在 MyGo 原生 UI 上的一比一实现。
// 尺寸、颜色、字号均取自原版前端（uno.config.ts + 组件 className），
// 并用 scripts/cdp.mjs + scripts/dump-styles.js 从运行中的 Wails 版导出的计算样式校对。
package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// uno.config.ts 的色板。
var (
	brand50  = ui.Hex("#fdfdfd")
	brand100 = ui.Hex("#f3f4f6")
	brand150 = ui.Hex("#ecedf0")
	brand200 = ui.Hex("#e5e7eb")
	brand300 = ui.Hex("#d1d5db")
	brand400 = ui.Hex("#9ca3af")
	brand500 = ui.Hex("#6b7280")
	brand600 = ui.Hex("#4b5563")
	brand700 = ui.Hex("#44484e")
	brand750 = ui.Hex("#303235")
	brand800 = ui.Hex("#1c1e1f")
	brand900 = ui.Hex("#121416")

	neutral50  = ui.Hex("#f8fafc")
	neutral100 = ui.Hex("#f1f5f9")
	neutral300 = ui.Hex("#cbd5e1")
	neutral400 = ui.Hex("#94a3b8")
	neutral500 = ui.Hex("#64748b")
	neutral600 = ui.Hex("#475569")
	neutral700 = ui.Hex("#334155")

	success500 = ui.Hex("#10b981")
	error400   = ui.Hex("#f87171")
	error500   = ui.Hex("#ef4444")
	error600   = ui.Hex("#dc2626")
	primary500 = ui.Hex("#7c6aef")
	slate500   = ui.Hex("#64748b")
	blue500    = ui.Hex("#3b82f6")
	red500     = ui.Hex("#ef4444")
	yellow500  = ui.Hex("#eab308")
	white      = ui.Hex("#ffffff")
	black      = ui.Hex("#000000")
)

// palette 是组件里 `x dark:y` 两套取值在当前主题下的结果。
type palette struct {
	dark bool

	text, textStrong, textMuted, textFaint ui.Color // brand-900/100, 900/white, 500/400, 400/500

	appBg                  ui.Color // main: bg-brand-100 dark:bg-brand-900
	topbarBg, topbarBorder ui.Color
	controlIcon            ui.Color // 窗口按钮图标 text-brand-600 dark:text-brand-400
	controlHover           ui.Color

	sidebarBg, sidebarBorder ui.Color
	navText, navHover        ui.Color // text-brand-700 dark:text-brand-300 / hover:bg-brand-100 dark:bg-brand-700
	navActiveBg, navActive   ui.Color

	inputBg, inputBorder, inputFocus, inputIcon, placeholder ui.Color

	toolBtnBg, toolBtnBorder, toolBtnHover, toolBtnIcon, toolBtnText ui.Color
	toolBtnActiveBg, toolBtnActiveBorder, toolBtnActiveText          ui.Color

	primaryBtn, primaryBtnHover ui.Color

	cardBg, cardBorder, cardBorderHover, coverBg ui.Color

	scrollThumb, scrollThumbHover ui.Color

	panelBg, panelBorder, divider ui.Color // 下拉菜单、抽屉
	menuHover                     ui.Color
	sectionLabel                  ui.Color // text-sm font-medium text-brand-700 dark:text-brand-300

	chipBg, chipText, chipHover, chipActiveBg, chipActiveText, chipRing ui.Color

	selectBg, selectBorder, selectText ui.Color
	listFocus, listSelected            ui.Color
	switchOff                          ui.Color

	tooltipBg, tooltipText, tooltipBorder ui.Color
}

func newPalette(dark bool) palette {
	if dark {
		return palette{
			dark: true,
			text: brand100, textStrong: white, textMuted: brand400, textFaint: brand500,
			appBg:    brand900,
			topbarBg: brand800, topbarBorder: brand700.Alpha(0.5),
			controlIcon: brand400, controlHover: brand700,
			sidebarBg: brand800, sidebarBorder: brand700,
			navText: brand300, navHover: brand700, navActiveBg: brand700, navActive: brand100,
			inputBg: brand900, inputBorder: brand700, inputFocus: neutral500, inputIcon: brand400, placeholder: brand400,
			toolBtnBg: brand700, toolBtnBorder: brand700, toolBtnHover: brand600, toolBtnIcon: brand300, toolBtnText: brand300,
			toolBtnActiveBg: brand600, toolBtnActiveBorder: brand500, toolBtnActiveText: white,
			primaryBtn: neutral600, primaryBtnHover: neutral700,
			cardBg: brand800, cardBorder: brand700, cardBorderHover: brand600, coverBg: brand700,
			scrollThumb: brand600, scrollThumbHover: brand500,
			panelBg: brand800, panelBorder: brand700, divider: brand700, menuHover: brand700,
			sectionLabel: brand300,
			chipBg:       brand900.Alpha(0.5), chipText: brand400, chipHover: brand700.Alpha(0.7),
			chipActiveBg: brand700, chipActiveText: brand200, chipRing: brand500,
			selectBg: brand700, selectBorder: brand600, selectText: white,
			listFocus: brand700, listSelected: brand750,
			switchOff: brand750,
			tooltipBg: brand100, tooltipText: brand900, tooltipBorder: brand200,
		}
	}
	return palette{
		text: brand900, textStrong: brand900, textMuted: brand500, textFaint: brand400,
		appBg:    brand100,
		topbarBg: brand50, topbarBorder: brand200.Alpha(0.5),
		controlIcon: brand600, controlHover: brand200,
		sidebarBg: white, sidebarBorder: brand200,
		navText: brand700, navHover: brand100, navActiveBg: brand200, navActive: brand900,
		inputBg: white, inputBorder: brand300, inputFocus: neutral600, inputIcon: brand700, placeholder: brand700,
		toolBtnBg: brand150, toolBtnBorder: brand200, toolBtnHover: brand200, toolBtnIcon: brand600, toolBtnText: brand700,
		toolBtnActiveBg: brand200, toolBtnActiveBorder: brand300, toolBtnActiveText: brand900,
		primaryBtn: neutral600, primaryBtnHover: neutral700,
		cardBg: white, cardBorder: brand100, cardBorderHover: brand300, coverBg: brand200,
		scrollThumb: ui.Hex("#cccccc"), scrollThumbHover: ui.Hex("#aaaaaa"),
		panelBg: white, panelBorder: brand200, divider: brand200, menuHover: brand100,
		sectionLabel: brand700,
		chipBg:       brand50, chipText: brand500, chipHover: brand100,
		chipActiveBg: brand100, chipActiveText: brand700, chipRing: brand300,
		selectBg: white, selectBorder: brand300, selectText: brand900,
		listFocus: neutral100, listSelected: neutral50,
		switchOff: brand300,
		tooltipBg: brand900, tooltipText: white, tooltipBorder: white.Alpha(0.1),
	}
}

// Tailwind 的字号阶梯：字号与行高（px = DIP）。
type fontStep struct{ size, line float32 }

var (
	textXS   = fontStep{12, 16}
	textSM   = fontStep{14, 20}
	textBase = fontStep{16, 24}
	textXL   = fontStep{20, 28}
	text4XL  = fontStep{36, 40}
)

const (
	weightNormal   = 400
	weightMedium   = 500
	weightSemibold = 600
	weightBold     = 700
)

// text 创建一段带 Tailwind 字号和固定行高的文字，与浏览器的行盒高度一致。
func text(c *ui.Context, s string, f fontStep, weight int, color ui.Color) ui.Element {
	return textMargin(c, s, f, weight, color, 0, 0, 0, 0)
}

// textMargin 同 text，另带外边距（Element.Margin 会覆盖 text 设置的负外边距，所以合在一起设）。
//
// MyGo 的行高取 max(行高, 字体升部+降部)，而 CSS 允许行高小于字体高度、让字形上下等量溢出
// （负的半行距）。这里用对称的负外边距把文字占位压回 CSS 的行高，字形位置与浏览器一致。
func textMargin(c *ui.Context, s string, f fontStep, weight int, color ui.Color, top, right, bottom, left float32) ui.Element {
	d := lineOverflow(c, s, f, weight)
	e := ui.Text(c, s).FontSize(f.size).FixedLineHeight(f.line).FontWeight(weight).TextColor(color).
		Margin(top-d, right, bottom-d, left)
	if uiFont != "" {
		e.Font(uiFont)
	}
	return e
}

// naturalLineRatio 是 MyGo 在 Windows 上一行文字的自然高度（升部+降部）与字号之比，按字体实测；
// 中文、假名回退也落在同一行盒里。TestLineBoxHeights 校验 text() 的占位等于 CSS 行高。
func naturalLineRatio() float32 {
	switch uiFont {
	case "Microsoft YaHei UI", "Microsoft YaHei":
		return 1.27
	}
	return 4.0 / 3 // Segoe UI：(2210+514)/2048
}

func lineOverflow(_ *ui.Context, _ string, f fontStep, _ int) float32 {
	return max(0, (f.size*naturalLineRatio()-f.line)/2)
}

// icon 以 1em 见方显示图标，与 UnoCSS 的 i-mdi-* 一致。
func icon(c *ui.Context, s *ui.SVG, size float32, color ui.Color) ui.Element {
	return ui.Icon(c, s).Size(size, size).TextColor(color).Shrink(0)
}

// cubicBezier 返回 CSS cubic-bezier() 的缓动函数。
func cubicBezier(x1, y1, x2, y2 float64) ui.Easing {
	sample := func(a1, a2, t float64) float64 {
		u := 1 - t
		return 3*u*u*t*a1 + 3*u*t*t*a2 + t*t*t
	}
	return func(x float32) float32 {
		if x <= 0 {
			return 0
		}
		if x >= 1 {
			return 1
		}
		// 二分求 t 使 bezierX(t) = x。
		lo, hi := 0.0, 1.0
		t := float64(x)
		for range 24 {
			if sample(x1, x2, t) < float64(x) {
				lo = t
			} else {
				hi = t
			}
			t = (lo + hi) / 2
		}
		return float32(sample(y1, y2, t))
	}
}

var (
	easeCSS      = cubicBezier(0.25, 0.1, 0.25, 1) // CSS 的 ease
	easeTailwind = cubicBezier(0.4, 0, 0.2, 1)     // Tailwind transition 默认曲线
	easeDrawer   = cubicBezier(0.22, 1, 0.36, 1)
)

// Tailwind 的 transition-colors 等默认 150ms。
func colorsTransition(d time.Duration) ui.ElementTransition {
	return ui.ElementTransition{Colors: true, Duration: d, Ease: easeTailwind}
}

// hairline 返回一个物理像素的 DIP 宽度：Chromium 会把 1px 边框吸附到设备像素。
func hairline(scale float64) float32 {
	if scale <= 0 {
		return 1
	}
	return float32(math.Max(1, math.Floor(scale)) / scale)
}

// devicePx 把 DIP 尺寸向下取整到设备像素（滚动条槽宽等）。
func devicePx(v float32, scale float64) float32 {
	if scale <= 0 {
		return v
	}
	return float32(math.Floor(float64(v)*scale) / scale)
}
