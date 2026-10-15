// Package chart 对应原版 frontend/src/components/chart：用 MyGo 的 ui/chart 画 Chart.js 风格的图表。
package chart

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	plot "github.com/egoist/mygo/ui/chart"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// Point 是一天（年维度为一个月）的游戏时长。
type Point struct {
	Label   string    // 分类轴的标签，后端返回的 YYYY-MM-DD 或 YYYY-MM
	Time    time.Time // 时间轴上的位置
	Seconds int
}

// State 是图表跨帧保留的状态：悬停选中的点与上一帧绘图区的宽度（决定刻度疏密）。
type State struct {
	selected int
	ready    bool
	plotW    float32
}

// DurationLineProps 是 DurationLineChart 的属性。
type DurationLineProps struct {
	Points    []Point
	TimeScale bool      // scaleType="time"：按日期稀疏分布的点
	From, To  time.Time // 时间轴的范围（TimeScale 时）
	Height    float32
	Label     string // 数据集名称，显示在提示框里
	State     *State
}

// Chart.js 数据集与 useChartTheme 的颜色。
var (
	lineColor  = ui.RGBA(59, 130, 246, 1)
	pointColor = ui.RGBA(59, 130, 246, 0.5)
)

const (
	dayMS        = 24 * time.Hour
	labelPadding = 12 // ticks.autoSkipPadding
	tickLength   = 8  // grid.tickLength：网格线伸进刻度标签一侧的长度
)

// DurationLineChart 对应 DurationLineChart.tsx（不显示图例）：y 轴从 0 开始、以“5时 2分”标注；没有游玩数据时
// y 轴固定 0–10 分钟、每分钟一格。悬停（interaction mode index）时放大该点并在旁边显示提示框。
//
// 与 Chart.js 不同：x 轴标签放不下时只跳过，不旋转 45°。
func DurationLineChart(c *ui.Context, t *theme.Theme, p DurationLineProps) ui.Element {
	s := p.State
	if !s.ready {
		s.selected, s.ready = -1, true
	}
	textColor := t.Pick(ui.Hex("#374151"), ui.Hex("#e5e7eb"))
	gridColor := t.Pick(ui.Hex("#e5e7eb"), ui.Hex("#374151"))
	n := len(p.Points)

	hasData, maxV := false, 0.0
	for _, pt := range p.Points {
		if pt.Seconds > 0 {
			hasData = true
		}
		maxV = max(maxV, float64(pt.Seconds))
	}
	var y plot.Linear
	var yTicks []plot.Tick
	if hasData {
		y = plot.NewLinear(0, maxV).Nice(10)
		yTicks = y.Ticks(10)
	} else {
		y = plot.NewLinear(0, 600)
		for v := 0; v <= 600; v += 60 {
			yTicks = append(yTicks, plot.Tick{At: y.At(float64(v)), Value: float64(v)})
		}
	}
	for i := range yTicks {
		yTicks[i].Label = timefmt.DurationChart(int(yTicks[i].Value))
	}

	plotW := s.plotW
	if plotW <= 0 {
		plotW = 800
	}
	labelW := textWidth(c, "0000-00-00")
	xs, ys := make([]float32, n), make([]float32, n)
	for i, pt := range p.Points {
		ys[i] = y.At(float64(pt.Seconds))
	}
	var xTicks []plot.Tick
	if p.TimeScale {
		x := plot.NewTime(p.From, p.To)
		for i, pt := range p.Points {
			xs[i] = x.At(pt.Time)
		}
		xTicks = timeTicks(x, p.From.Location(), plotW, labelW)
	} else {
		for i := range xs {
			xs[i] = 0.5
			if n > 1 {
				xs[i] = float32(i) / float32(n-1)
			}
		}
		step := 1
		if n > 1 {
			step = max(1, int(math.Ceil(float64((labelW+labelPadding)/(plotW/float32(n-1))))))
		}
		for i := 0; i < n; i += step {
			xTicks = append(xTicks, plot.Tick{At: xs[i], Label: p.Points[i].Label})
		}
	}

	label := func(_ plot.Side, tk plot.Tick) {
		components.Text(c, tk.Label, theme.TextXS, theme.WeightNormal, textColor).NoWrap()
	}
	radius, hoverRadius := float32(3), float32(4)
	if p.TimeScale {
		radius, hoverRadius = 4, 6
	}
	describe := func(i int) string {
		return p.Points[i].Label + " " + p.Label + ": " + timefmt.Duration(p.Points[i].Seconds)
	}
	return plot.Plot(c, plot.Axes{Left: yTicks, Bottom: xTicks, Label: label}, func(area ui.Element) {
		if b := area.Bounds(); b.W > 0 {
			s.plotW = b.W
		}
		area.Draw(func(pt *ui.Painter, r ui.Rect) {
			for _, tk := range yTicks {
				yy := plot.Y(r, tk.At)
				pt.Line(r.X-tickLength, yy, r.X+r.W, yy, 1, gridColor)
			}
			for _, tk := range xTicks {
				xx := plot.X(r, tk.At)
				pt.Line(xx, r.Y, xx, r.Y+r.H+tickLength, 1, gridColor)
			}
			if n > 1 {
				pt.StrokePath(plot.Line(r, xs, ys, plot.CurveMonotone), 3, lineColor)
			}
		})
		for i := range n {
			r := radius
			if i == s.selected {
				r = hoverRadius
			}
			plot.Point(c, xs[i], ys[i], 2*r, 2*r).Radius(r).Background(pointColor).Border(1, lineColor).PassThrough()
		}
		if n > 0 {
			plot.Select(c, area, &s.selected, xs, describe)
		}
		if i := s.selected; i >= 0 && i < n {
			tooltip(c, p, xs[i], ys[i], i)
		}
	}).Label(p.Label).Height(p.Height).Gap(3+tickLength).
		Padding(0, labelW/2, 0, 0) // Chart.js 给图表区留出边距，让最后一个 x 轴标签不被裁掉
}

// tooltip 是 Chart.js 默认的提示框：rgba(0,0,0,.8) 底、圆角 6、内边距 6，点在左半边时显示在右侧，反之在左侧。
func tooltip(c *ui.Context, p DurationLineProps, x, y float32, i int) {
	h, margin := ui.Start, [4]float32{0, 0, 0, 7}
	if x > 0.5 {
		h, margin = ui.End, [4]float32{0, 7, 0, 0}
	}
	plot.Anchor(c, x, y, h, ui.Center, func() {
		ui.Column(c).Margin(margin[:]...).Padding(6).Radius(6).Gap(6).Background(ui.RGBA(0, 0, 0, 0.8)).PassThrough().Children(func() {
			components.Text(c, p.Points[i].Label, theme.FontStep{Size: 12, Line: 14.4}, theme.WeightBold, theme.White).NoWrap()
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(12, 12).Background(theme.White).Border(1, lineColor).Children(func() {
					ui.Box(c).Fill().Background(pointColor)
				})
				components.Text(c, p.Label+": "+timefmt.Duration(p.Points[i].Seconds), theme.FontStep{Size: 12, Line: 14.4}, theme.WeightNormal, theme.White).NoWrap()
			})
		})
	})
}

// timeTicks 在时间轴上取放得下的整日、整月或整年刻度，按跨度以 yyyy-MM-dd、yyyy-MM、yyyy 标注。
func timeTicks(x plot.Time, loc *time.Location, plotW, labelW float32) []plot.Tick {
	ticks := x.Ticks(max(2, int(plotW/(labelW+labelPadding))))
	layout := "2006-01-02"
	if len(ticks) > 1 {
		step := time.Duration(ticks[1].Value-ticks[0].Value) * time.Millisecond
		switch {
		case step >= 365*dayMS:
			layout = "2006"
		case step >= 28*dayMS:
			layout = "2006-01"
		}
	}
	for i := range ticks {
		ticks[i].Label = time.UnixMilli(int64(ticks[i].Value)).In(loc).Format(layout)
	}
	return ticks
}

// PaddedRange 对应 createPaddedTimeRange：首尾留出跨度的 4%（1–14 天），只有一天时前后各留 7 天。
func PaddedRange(start, end time.Time) (time.Time, time.Time) {
	if start.Equal(end) {
		return start.Add(-7 * dayMS), end.Add(7 * dayMS)
	}
	span := max(dayMS, end.Sub(start))
	pad := min(14*dayMS, max(dayMS, time.Duration(float64(span)*0.04)))
	return start.Add(-pad), end.Add(pad)
}

func textWidth(c *ui.Context, s string) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: s, Font: theme.UIFont, Size: theme.TextXS.Size})
	return w
}
