package ui

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// text() 的占位高度必须等于 CSS 行高，后面元素的位置才会与浏览器一致。
func TestLineBoxHeights(t *testing.T) {
	cases := []struct {
		s string
		f fontStep
		w int
	}{
		{"游戏库", text4XL, weightBold},
		{"共 12 个游戏", textBase, weightNormal},
		{"橘光柠水随想曲", fontStep{14, 17.5}, weightBold},
		{"ゆずソフト", fontStep{12, 15}, weightNormal},
		{"Navel", fontStep{12, 15}, weightNormal},
		{"添加游戏", textSM, weightMedium},
	}
	for i, k := range cases {
		mark := fmt.Sprintf("mark%d", i)
		view := func(c *ui.Context) {
			ui.Column(c).Children(func() {
				text(c, k.s, k.f, k.w, white)
				ui.Text(c, mark)
			})
		}
		tt := ui.NewTester(view, 800, 400)
		tt.SetScale(1.5)
		tt.Frame()
		r, _ := tt.Find(mark)
		if math.Abs(float64(r.Y-k.f.line)) > 0.5 {
			t.Errorf("%s (%v): next element at y=%.2f, want %.1f", k.s, k.f, r.Y, k.f.line)
		}
	}
}
