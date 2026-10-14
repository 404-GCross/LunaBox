package ui

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// hud 是右下角的性能面板（F2 切换）。它每秒随内存采样重绘一次，测空闲 CPU 时请先关掉。
func (a *App) hud(c *ui.Context) {
	now := time.Now()
	lines := []string{
		fmt.Sprintf("fps %d   view build %.2f ms", a.perf.fps(now), float64(a.perf.lastBuild.Microseconds())/1000),
	}
	if mem := a.perf.mem.Load(); mem != nil {
		lines = append(lines,
			fmt.Sprintf("working set %.1f MB   private %.1f MB", mb(mem.workingSet), mb(mem.private)),
			fmt.Sprintf("go heap %.1f MB", mb(mem.heapInuse)))
	}
	lines = append(lines,
		fmt.Sprintf("games %d   loaded %d   covers %d decoded, %d pending", a.lib.total, len(a.lib.games), a.covers.decoded, len(a.covers.pending)),
		fmt.Sprintf("first frame %d ms   scale %.2f", a.perf.firstFrameMs.Load(), a.scale),
	)
	if a.covers.decoded > 0 {
		lines = append(lines, fmt.Sprintf("cover load avg %.1f ms", float64(a.covers.decodeTotal.Microseconds())/1000/float64(a.covers.decoded)))
	}
	switch {
	case a.bench.phase != benchIdle:
		lines = append(lines, "bench running...")
	case a.bench.report != "":
		lines = append(lines, "last bench: see bench-result.txt")
	}
	lines = append(lines, "F2 hide HUD   F5 scroll bench")

	ui.Overlay(c, func() {
		ui.Column(c).Absolute().Left(12).Bottom(12).Padding(8, 10).Radius(8).Gap(2).
			Background(ui.RGBA(0, 0, 0, 0.72)).PassThrough().Children(func() {
			for _, l := range lines {
				ui.Text(c, l).FontSize(11).Font("Consolas").TextColor(white).NoWrap()
			}
		})
	})
}

type benchPhase int

const (
	benchIdle benchPhase = iota
	benchWarmup
	benchDown
	benchUp
)

// benchSpeed 是自动滚动速度（DIP/秒），约等于快速拖动滚动条。
const benchSpeed = 2400

type benchState struct {
	phase    benchPhase
	since    time.Time
	last     time.Time
	scrolled float32
	before   *memSample
	report   string
}

func (b *benchState) arm() {
	b.phase = benchWarmup
	b.since = time.Now()
	b.report = ""
}

// step 在每次 view 时推进压测：预热 1.5s 等数据与封面加载，然后滚到底再滚回顶部。
func (b *benchState) step(c *ui.Context, a *App) {
	if b.phase == benchIdle {
		return
	}
	st := &a.lib.scroll.state
	now := time.Now()
	c.AnimationFrame()
	if b.phase == benchWarmup {
		if now.Sub(b.since) < 1500*time.Millisecond || a.lib.loading {
			return
		}
		st.Y = 0
		b.before = readMem()
		b.phase, b.since, b.last, b.scrolled = benchDown, now, now, 0
		a.perf.startRecording()
		return
	}

	dt := float32(now.Sub(b.last).Seconds())
	b.last = now
	delta := benchSpeed * dt
	b.scrolled += delta
	switch b.phase {
	case benchDown:
		st.Y += delta
		if st.MaxY > 0 && st.Y >= st.MaxY {
			st.Y = st.MaxY
			b.phase = benchUp
		}
	case benchUp:
		st.Y = float32(math.Max(0, float64(st.Y-delta)))
		if st.Y == 0 {
			b.finish(a, now)
		}
	}
	if b.phase == benchDown && st.MaxY == 0 && now.Sub(b.since) > 2*time.Second {
		b.finish(a, now)
	}
}

func (b *benchState) finish(a *App, now time.Time) {
	frames := a.perf.stopRecording()
	b.report = a.perf.benchReport(frames, a.lib.total, b.scrolled, now.Sub(b.since), b.before, readMem())
	b.phase = benchIdle
	fmt.Print(b.report)
	if f, err := os.OpenFile("bench-result.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(f, "%s\n", b.report)
		f.Close()
	}
	if a.opts.BenchExit {
		mygo.App.Quit()
	}
}
