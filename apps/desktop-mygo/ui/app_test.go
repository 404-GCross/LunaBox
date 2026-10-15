package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
)

// fakeBackend 在内存中实现 Backend，按名称搜索与状态筛选，其余条件忽略。
type fakeBackend struct {
	games []models.Game
	cfg   appconf.AppConfig
	reqs  []vo.GameListRequest
}

func (f *fakeBackend) GetGames(req vo.GameListRequest) (vo.GameListResponse, error) {
	f.reqs = append(f.reqs, req)
	var match []models.Game
	for _, g := range f.games {
		if req.SearchQuery != "" && !strings.Contains(strings.ToLower(g.Name), strings.ToLower(req.SearchQuery)) {
			continue
		}
		if req.Status != nil && (g.Status == *req.Status) == req.ExcludeStatus {
			continue
		}
		match = append(match, g)
	}
	resp := vo.GameListResponse{Total: len(match), Offset: req.Offset, Limit: req.Limit}
	if req.Offset < len(match) {
		resp.Games = match[req.Offset:min(len(match), req.Offset+req.Limit)]
	}
	return resp, nil
}

func (f *fakeBackend) SearchTagsInLibrary(string) ([]string, error)              { return nil, nil }
func (f *fakeBackend) ListGameFilterPresets() ([]models.GameFilterPreset, error) { return nil, nil }
func (f *fakeBackend) Config() *appconf.AppConfig                                { return &f.cfg }
func (f *fakeBackend) DataDir() string                                           { return "" }

func newFakeBackend(n int) *fakeBackend {
	f := &fakeBackend{cfg: appconf.AppConfig{SidebarOpen: true, ShowSortFieldOnCover: true}}
	names := []string{"橘光柠水随想曲", "Summer Pockets", "星空鉄道とシロの旅"}
	for i := range n {
		f.games = append(f.games, models.Game{
			ID:        fmt.Sprint(i),
			Name:      names[i%len(names)],
			Company:   "ゆずソフト",
			CreatedAt: time.Date(2026, 9, 11, 12, 0, 0, 0, time.Local),
			Status:    []enums.GameStatus{enums.StatusNotStarted, enums.StatusCompleted}[i%2],
		})
	}
	return f
}

func newTestApp(t *testing.T, n int) (*App, *fakeBackend, *ui.Tester) {
	t.Helper()
	b := newFakeBackend(n)
	a := New(b, Options{})
	a.SetScale(1.5)
	a.lib.reload()
	tt := ui.NewTester(a.View, 1280, 800)
	tt.SetScale(1.5)
	tt.SetDark(true)
	for range 3 {
		tt.Frame()
	}
	return a, b, tt
}

func TestLibraryLoadsAndFilters(t *testing.T) {
	a, b, tt := newTestApp(t, 7)
	for _, s := range []string{"游戏库", "共 7 个游戏", "橘光柠水随想曲", "ゆずソフト", "2026/09/11"} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}

	// 搜索有 250ms 防抖，之后才发请求。
	a.lib.search = "summer"
	a.lib.searchEditedAt = time.Now().Add(-time.Second)
	tt.Frame()
	if got := b.reqs[len(b.reqs)-1].SearchQuery; got != "summer" {
		t.Fatalf("last request search = %q", got)
	}
	tt.Frame()
	if !tt.HasText("共 2 个游戏") {
		t.Errorf("texts after search %q", tt.Texts())
	}

	// 状态筛选写进请求，并改变计数文案。
	a.lib.search, a.lib.debounced = "", ""
	a.lib.status = enums.StatusCompleted
	a.lib.reload()
	tt.Frame()
	if !tt.HasText("共 3 个 已通关 游戏") {
		t.Errorf("texts after status filter %q", tt.Texts())
	}
}

func TestSidebarToggle(t *testing.T) {
	a, _, tt := newTestApp(t, 3)
	if err := tt.Click("切换侧边栏"); err != nil {
		t.Fatal(err)
	}
	if a.sidebar.open {
		t.Fatal("sidebar still open after toggle")
	}
}

func TestGridGeometryMatchesWeb(t *testing.T) {
	// 1280 宽窗口、展开侧边栏、150% 缩放：原版容器 951.3，6 列，卡片 148.6，行高 235。
	g := newGridGeometry(1024-64-devicePx(9, 1.5), false)
	if g.cols != 6 || fmt.Sprintf("%.1f", g.cardW) != "148.6" || g.rowH != 235 {
		t.Fatalf("geometry = %+v", g)
	}
}

// DESKTOP_MYGO_LAYOUT=1 时打印关键文字的位置（DIP），用于与 dump-styles.js 导出的原版坐标对照。
func TestLayoutDump(t *testing.T) {
	if os.Getenv("DESKTOP_MYGO_LAYOUT") == "" {
		t.Skip("DESKTOP_MYGO_LAYOUT not set")
	}
	_, _, tt := newTestApp(t, 12)
	for _, s := range []string{"游戏库", "共 12 个游戏", "首页", "筛选", "添加游戏", "橘光柠水随想曲", "ゆずソフト", "2026/09/11"} {
		r, ok := tt.Find(s)
		t.Logf("%-16s found=%v x=%.1f y=%.1f w=%.1f h=%.1f", s, ok, r.X, r.Y, r.W, r.H)
	}
	if dir := os.Getenv("DESKTOP_MYGO_PNG"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "tester.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_ = png.Encode(f, tt.Image())
	}
}
