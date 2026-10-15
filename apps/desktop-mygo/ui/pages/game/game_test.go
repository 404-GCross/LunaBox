package game

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
)

type testPage struct {
	*Page
	b        *envtest.Backend
	tt       *ui.Tester
	searched []string
	tagged   []string
}

// newTestPage 打开第一个游戏的详情页；width 是窗口宽度，侧边栏展开时主内容区少 256。
func newTestPage(t *testing.T, width int) *testPage {
	t.Helper()
	b := envtest.NewBackend(3)
	g := &b.Games[0]
	g.Summary = "夏日的岛上，\n与少女们相遇。"
	g.SourceType, g.SourceID = enums.Bangumi, "123"
	g.Rating, g.ReleaseDate = 8.5, "2024-05-31"
	b.Tags = map[string][]models.GameTag{"0": {
		{ID: "t1", Name: "纯爱", Source: "bangumi"},
		{ID: "t2", Name: "我的标签", Source: "user"},
	}}
	day := time.Date(2026, 9, 11, 20, 0, 0, 0, time.Local)
	b.Stats = vo.GameDetailStats{TotalPlayCount: 3, TotalPlayTime: 3900, TodayPlayTime: 600, StartDate: "2026-09-05", EndDate: "2026-09-11"}
	for i := range 7 {
		b.Stats.RecentPlayHistory = append(b.Stats.RecentPlayHistory, vo.DailyPlayTime{
			Date: day.AddDate(0, 0, i-6).Format("2006-01-02"), Duration: []int{0, 0, 1200, 0, 0, 2100, 600}[i],
		})
	}
	b.Sessions = []models.PlaySession{{ID: "s1", GameID: "0", StartTime: day, Duration: 3900}}

	tp := &testPage{b: b}
	e := &env.Env{Backend: b, Router: ui.NewRouter(env.GamePath("0")), Covers: covers.New(), Theme: theme.New(true, 1.5),
		MainWidth: float32(width - 256)}
	tp.Page = New(e, Links{
		SearchLibrary: func(q string) { tp.searched = append(tp.searched, q) },
		FilterByTag:   func(tag string) { tp.tagged = append(tp.tagged, tag) },
	})
	tp.Open("0", time.Now())
	tp.tt = ui.NewTester(tp.View, width, 800)
	tp.tt.SetScale(1.5)
	tp.tt.SetDark(true)
	tp.frames(3)
	return tp
}

func (tp *testPage) frames(n int) {
	for range n {
		tp.tt.Frame()
	}
}

func (tp *testPage) expectTexts(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !tp.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tp.tt.Texts())
		}
	}
}

func TestShowsDetails(t *testing.T) {
	tp := newTestPage(t, 1280)
	tp.expectTexts(t,
		"橘光柠水随想曲", "返回", "启动游戏", "未开始",
		"数据源", "bangumi", "开发", "ゆずソフト", "添加时间", "2026-09-11", "评分", "8.5 / 10", "发售日期", "2024-05-31",
		"简介", "夏日的岛上，\n与少女们相遇。", "纯爱", "我的标签", "添加标签",
		"游戏统计", "编辑", "启动配置", "备份", "游玩进度", "评价",
		"本周游戏次数", "3", "今日游戏时长", "10分钟", "本周总时长", "1小时5分钟",
		"周", "月", "年", "全部", "手动添加",
	)
	if tp.Title() != "橘光柠水随想曲" {
		t.Errorf("title = %q", tp.Title())
	}
}

func TestLinks(t *testing.T) {
	tp := newTestPage(t, 1280)
	if err := tp.tt.Click("打开数据源网页"); err != nil {
		t.Fatal(err)
	}
	if got := tp.tt.OpenedURLs(); !slices.Equal(got, []string{"https://bgm.tv/subject/123"}) {
		t.Errorf("opened %q", got)
	}
	if err := tp.tt.Click("ゆずソフト"); err != nil {
		t.Fatal(err)
	}
	if err := tp.tt.Click("纯爱"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tp.searched, []string{"ゆずソフト"}) || !slices.Equal(tp.tagged, []string{"纯爱"}) {
		t.Errorf("searched %q, tagged %q", tp.searched, tp.tagged)
	}
}

func TestStatusButtons(t *testing.T) {
	tp := newTestPage(t, 1280)
	if err := tp.tt.Click("想玩"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if tp.game.Status != enums.StatusWantToPlay {
		t.Fatalf("status = %q", tp.game.Status)
	}
	tp.expectTexts(t, "想玩")
}

// 窗口窄到放不下全部状态按钮时，状态改成下拉菜单。
func TestCompactStatusMenu(t *testing.T) {
	tp := newTestPage(t, 960)
	if err := tp.tt.Click("状态: 未开始"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	if err := tp.tt.Click("已通关"); err != nil {
		t.Fatalf("%v in %q", err, tp.tt.Texts())
	}
	tp.frames(2)
	if tp.game.Status != enums.StatusCompleted || tp.statusOpen {
		t.Fatalf("status = %q, menu open = %v", tp.game.Status, tp.statusOpen)
	}
	tp.expectTexts(t, "已通关")
}

func TestStatsPeriodAndSessions(t *testing.T) {
	tp := newTestPage(t, 1280)
	tp.tt.SetSize(1280, 1600) // 底部的时间维度与视图切换在 800 高的窗口里要滚动才看得到
	tp.frames(2)
	for _, period := range []struct {
		label string
		want  enums.Period
		text  string
	}{{"月", enums.Month, "本月游戏次数"}, {"全部", enums.All, "累计游戏次数"}, {"年", enums.Year, "今年总时长"}} {
		if err := tp.tt.Click(period.label); err != nil {
			t.Fatal(err)
		}
		tp.frames(2)
		if got := tp.b.StatReqs[len(tp.b.StatReqs)-1]; got.Dimension != period.want || got.GameID != "0" {
			t.Errorf("request after %s = %+v", period.label, got)
		}
		tp.expectTexts(t, period.text)
	}

	if err := tp.tt.Click("查看游玩记录"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.expectTexts(t, "2026/09/11 20:00:00", "时长: 1小时5分钟")

	tp.b.Sessions = nil
	tp.stats.load(tp.env, "0")
	tp.frames(2)
	tp.expectTexts(t, "暂无游玩记录")
}

func TestOtherTabsArePlaceholders(t *testing.T) {
	tp := newTestPage(t, 1280)
	for _, tab := range tabs[1:] {
		if err := tp.tt.Click(tab.label); err != nil {
			t.Fatal(err)
		}
		tp.frames(1)
		if !tp.tt.HasText("此标签页未迁移到原型") || tp.tt.HasText("本周游戏次数") {
			t.Errorf("tab %s: %q", tab.label, tp.tt.Texts())
		}
	}
}

func TestNotFound(t *testing.T) {
	tp := newTestPage(t, 1280)
	tp.Open("missing", time.Now())
	tp.frames(2)
	tp.expectTexts(t, "未找到该游戏", "返回库")
}

func TestSourceURLs(t *testing.T) {
	for _, tc := range []struct {
		source enums.SourceType
		id     string
		want   string
	}{
		{enums.VNDB, "v17", "https://vndb.org/v17"},
		{enums.DLsite, "RJ01", "https://www.dlsite.com/maniax/work/=/product_id/RJ01.html"},
		{enums.ErogameScape, "a b", "https://erogamescape.org/~ap2/ero/toukei_kaiseki/game.php?game=a%20b"},
		{enums.Local, "x", ""},
	} {
		if got := sourceURL(tc.source, tc.id, ""); got != tc.want {
			t.Errorf("sourceURL(%s, %q) = %q, want %q", tc.source, tc.id, got, tc.want)
		}
	}
	g := &models.Game{SourceType: enums.VNDB, MetadataSources: []models.GameMetadataSource{
		{SourceType: enums.Bangumi, SourceID: "1"}, {SourceType: enums.VNDB, SourceID: "v2"},
	}}
	links := sourceLinks(g, "")
	if len(links) != 2 || links[0].source != enums.VNDB {
		t.Errorf("default source first: %+v", links)
	}
	if got := metadataSourceText(g); got != "vndb 等 2 个" {
		t.Errorf("metadataSourceText = %q", got)
	}
}

// DESKTOP_MYGO_PNG=<目录> 时把 Tester 渲染的详情页（图表、游玩记录、窄屏状态菜单）存成图片，用于目测。
func TestScreenshotDump(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	save := func(tp *testPage, name string) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tp.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	tp := newTestPage(t, 1280)
	tp.tt.SetSize(1280, 1500)
	tp.frames(3)
	if r, ok := tp.tt.Find("2026-09-10"); ok {
		tp.tt.Move(r.X+r.W/2, r.Y-60) // 悬停在图表上显示提示框
	}
	tp.frames(3)
	save(tp, "game-chart.png")
	if err := tp.tt.Click("查看游玩记录"); err != nil {
		t.Fatal(err)
	}
	tp.frames(3)
	save(tp, "game-sessions.png")

	narrow := newTestPage(t, 960)
	if err := narrow.tt.Click("状态: 未开始"); err != nil {
		t.Fatal(err)
	}
	narrow.frames(3)
	save(narrow, "game-narrow.png")
}
