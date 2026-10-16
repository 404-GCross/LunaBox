package library

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/common/enums"
)

func newTestPage(t *testing.T, n int) (*Page, *envtest.Backend, *ui.Tester) {
	t.Helper()
	b := envtest.NewBackend(n)
	e := &env.Env{Backend: b, Router: ui.NewRouter(env.PathLibrary), Covers: covers.New(), Theme: theme.New(true, 1.5), MainWidth: 1024}
	l := New(e)
	l.Reload()
	tt := ui.NewTester(l.View, 1024, 800)
	tt.SetScale(1.5)
	tt.SetDark(true)
	for range 3 {
		tt.Frame()
	}
	return l, b, tt
}

func TestLoadsAndFilters(t *testing.T) {
	l, b, tt := newTestPage(t, 7)
	for _, s := range []string{"游戏库", "共 7 个游戏", "橘光柠水随想曲", "ゆずソフト", "2026/09/11"} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}

	// 搜索有 250ms 防抖，之后才发请求。
	l.search = "summer"
	l.searchEditedAt = tt.Now().Add(-time.Second)
	tt.Frame()
	if got := b.Reqs[len(b.Reqs)-1].SearchQuery; got != "summer" {
		t.Fatalf("last request search = %q", got)
	}
	tt.Frame()
	if !tt.HasText("共 2 个游戏") {
		t.Errorf("texts after search %q", tt.Texts())
	}

	// 状态筛选写进请求，并改变计数文案。
	l.search, l.debounced = "", ""
	l.status = enums.StatusCompleted
	l.Reload()
	tt.Frame()
	if !tt.HasText("共 3 个 已通关 游戏") {
		t.Errorf("texts after status filter %q", tt.Texts())
	}
}

func TestFilterDrawerOpens(t *testing.T) {
	_, _, tt := newTestPage(t, 3)
	if err := tt.Click("筛选 (0)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second) // 等抽屉滑入
	for _, s := range []string{"标签筛选", "元数据来源", "主排序字段", "封面展示排序字段"} {
		if !tt.HasText(s) {
			t.Errorf("drawer missing %q", s)
		}
	}
}
