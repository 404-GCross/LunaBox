package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
)

// 与 routes/library.tsx 相同的分页窗口。
const (
	pageSize          = 120
	windowBufferSize  = pageSize
	windowRequestSize = pageSize * 2
	searchDebounce    = 250 * time.Millisecond
)

// library 是游戏库页面的状态，对应 routes/library.tsx。
type library struct {
	a *App

	// 查询条件
	search             string
	debounced          string
	searchEditedAt     time.Time
	sortBy             enums.GameListSortBy
	sortOrder          enums.SortOrder
	secondarySortBy    enums.GameListSortBy
	secondarySortOrder enums.SortOrder
	status             enums.GameStatus
	statusInverted     bool
	source             enums.SourceType
	sourceInverted     bool
	tags               []string
	tagsInverted       bool
	tagInput           string
	tagSuggestions     []string
	showSortField      bool
	landscape          bool

	// 数据：按下标缓存的分页结果
	games     map[int]*models.Game
	total     int
	loading   bool
	loaded    bool
	loadedKey string
	queryKey  string
	gen       int
	inflight  map[string]bool
	loadErr   error
	presets   []models.GameFilterPreset

	// 界面
	scroll        scrollBar
	drawer        drawerState
	addMenuOpen   bool
	statusMenu    bool
	batch         bool
	selected      map[string]bool
	randomLoading bool
	grid          gridGeometry
}

func newLibrary(a *App) *library {
	l := &library{
		a:                  a,
		sortBy:             enums.GameListSortByCreatedAt,
		sortOrder:          enums.SortOrderDesc,
		secondarySortOrder: enums.SortOrderAsc,
		games:              map[int]*models.Game{},
		inflight:           map[string]bool{},
		selected:           map[string]bool{},
		loading:            true,
	}
	l.scroll.gutter = true
	if cfg := a.backend.Config(); cfg != nil {
		l.showSortField = cfg.ShowSortFieldOnCover
		l.landscape = cfg.GameCardLayout == "landscape"
	}
	return l
}

// request 组装 GetGames 的查询条件（不含分页）。
func (l *library) request() vo.GameListRequest {
	req := vo.GameListRequest{
		SearchQuery: strings.TrimSpace(l.debounced),
		Tags:        l.tags,
		ExcludeTags: l.tagsInverted && len(l.tags) > 0,
		SortBy:      l.sortBy,
		SortOrder:   l.sortOrder,
	}
	if l.status != "" {
		s := l.status
		req.Status, req.ExcludeStatus = &s, l.statusInverted
	}
	if l.source != "" {
		s := l.source
		req.MetadataSource, req.ExcludeMetadataSource = &s, l.sourceInverted
	}
	if l.secondarySortBy != "" {
		req.SecondarySortBy, req.SecondarySortOrder = l.secondarySortBy, l.secondarySortOrder
	}
	return req
}

func (l *library) key() string {
	r := l.request()
	return fmt.Sprintf("%s|%v|%v|%s|%s|%s|%s|%v|%v|%v|%v", r.SearchQuery, r.Tags, r.ExcludeTags, r.SortBy, r.SortOrder,
		r.SecondarySortBy, r.SecondarySortOrder, deref(r.Status), r.ExcludeStatus, deref(r.MetadataSource), r.ExcludeMetadataSource)
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// reload 在查询条件变化后重新请求第一页；旧数据保留到新结果返回（期间显示“加载中”）。
func (l *library) reload() {
	l.gen++
	l.queryKey = l.key()
	l.loading = true
	l.inflight = map[string]bool{}
	l.fetch(0, windowRequestSize, true)
	l.loadPresets()
}

// ensureWindow 在可见范围有缺口时请求覆盖它的窗口（getWindowRequest）。
func (l *library) ensureWindow(start, end int) {
	if l.total == 0 || l.loading {
		return
	}
	bufferedStart := max(0, start-windowBufferSize)
	offset := bufferedStart / pageSize * pageSize
	reqEnd := min(l.total, offset+windowRequestSize, max(end+1, offset+pageSize))
	for i := offset; i < reqEnd; i++ {
		if l.games[i] == nil {
			l.fetch(offset, max(1, reqEnd-offset), false)
			return
		}
	}
}

func (l *library) fetch(offset, limit int, replace bool) {
	key := fmt.Sprintf("%d:%d:%d", l.gen, offset, limit)
	if l.inflight[key] {
		return
	}
	l.inflight[key] = true
	gen := l.gen
	req := l.request()
	req.Offset, req.Limit = offset, limit
	l.a.async(func() func() {
		resp, err := l.a.backend.GetGames(req)
		return func() {
			delete(l.inflight, key)
			if gen != l.gen {
				return
			}
			l.loadErr = err
			if replace {
				l.games = map[int]*models.Game{}
			}
			for i := range resp.Games {
				l.games[offset+i] = &resp.Games[i]
			}
			if err == nil {
				l.total = resp.Total
			}
			l.loading = false
			l.loaded = true
			l.loadedKey = l.queryKey
		}
	})
}

func (l *library) loadPresets() {
	l.a.async(func() func() {
		presets, err := l.a.backend.ListGameFilterPresets()
		return func() {
			if err == nil {
				l.presets = presets
			}
		}
	})
}

func (l *library) searchTags(query string) {
	if strings.TrimSpace(query) == "" {
		l.tagSuggestions = nil
		return
	}
	l.a.async(func() func() {
		names, err := l.a.backend.SearchTagsInLibrary(strings.TrimSpace(query))
		return func() {
			if err != nil || query != l.tagInput {
				return
			}
			out := names[:0:0]
			for _, n := range names {
				if !contains(l.tags, n) {
					out = append(out, n)
				}
			}
			l.tagSuggestions = out
		}
	})
}

// async 在后台执行 work，再把它返回的函数放回主线程；没有窗口（测试）时同步执行。
func (a *App) async(work func() func()) {
	if a.win == nil {
		work()()
		return
	}
	go func() {
		apply := work()
		a.win.Update(apply)
	}()
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (l *library) hasActiveFilters() bool {
	return strings.TrimSpace(l.debounced) != "" || len(l.tags) > 0 || l.status != "" || l.source != ""
}

func (l *library) activeFilterCount() int {
	n := 0
	if l.status != "" {
		n++
	}
	if l.source != "" {
		n++
	}
	if len(l.tags) > 0 || l.tagInput != "" {
		n++
	}
	return n
}

func (l *library) countText() string {
	label := statusLabel(l.status)
	switch {
	case l.status != "" && l.statusInverted:
		return fmt.Sprintf("共 %d 个非 %s 游戏", l.total, label)
	case l.status != "":
		return fmt.Sprintf("共 %d 个 %s 游戏", l.total, label)
	}
	return fmt.Sprintf("共 %d 个游戏", l.total)
}

var statusOptions = []struct {
	label string
	value enums.GameStatus
}{
	{"全部状态", ""},
	{"未开始", enums.StatusNotStarted},
	{"想玩", enums.StatusWantToPlay},
	{"游玩中", enums.StatusPlaying},
	{"已通关", enums.StatusCompleted},
	{"搁置", enums.StatusOnHold},
	{"抛弃", enums.StatusDropped},
}

func statusLabel(s enums.GameStatus) string {
	for _, o := range statusOptions {
		if o.value == s {
			return o.label
		}
	}
	return string(s)
}

var sourceOptions = []struct {
	label string
	value enums.SourceType
}{
	{"全部来源", ""},
	{"local", enums.Local},
	{"Bangumi", enums.Bangumi},
	{"VNDB", enums.VNDB},
	{"月幕gal", enums.Ymgal},
	{"DLsite", enums.DLsite},
	{"TouchGAL", enums.TouchGal},
	{"Hikarinagi", enums.Hikarinagi},
	{"ErogameScape", enums.ErogameScape},
	{"Steam", enums.Steam},
}

var sortOptions = []selectOption{
	{"名称", string(enums.GameListSortByName)},
	{"开发商", string(enums.GameListSortByCompany)},
	{"最近游玩", string(enums.GameListSortByLastPlayedAt)},
	{"添加时间", string(enums.GameListSortByCreatedAt)},
	{"评分", string(enums.GameListSortByRating)},
	{"发售日期", string(enums.GameListSortByReleaseDate)},
}

// ---------------------------------------------------------------------------
// 页面

const pagePadding = 32 // p-8

func (l *library) view(c *ui.Context) {
	a := l.a
	p := a.p
	now := c.Now()
	if l.search != l.debounced {
		if wait := searchDebounce - now.Sub(l.searchEditedAt); wait > 0 {
			c.After(wait)
		} else {
			l.debounced = l.search
			l.reload()
		}
	}

	winW, winH := c.Size()
	mainW := winW - l.a.sidebar.width
	gutter := a.gutterWidth()
	contentW := min(1408, mainW-2*pagePadding-gutter) // max-w-8xl
	l.grid = newGridGeometry(contentW, l.landscape)

	a.scrollArea(c, "library-scroll", &l.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding)
		sc.Children(func() {
			ui.Column(c).Width(contentW).Margin(0, ui.Auto).Gap(24).MinHeight(winH - TopbarHeight - 2*pagePadding).Children(func() {
				ui.Column(c).Children(func() {
					text(c, "游戏库", text4XL, weightBold, p.textStrong)
					textMargin(c, l.countText(), textBase, weightNormal, p.textMuted, 8, 0, 0, 0)
				})
				l.filterBar(c)
				l.content(c, winH)
			})
		})
	})

	l.scrollToTop(c)
}

func (l *library) content(c *ui.Context, winH float32) {
	p := l.a.p
	waiting := l.total == 0 && (l.loading || l.search != l.debounced || l.loadedKey != l.queryKey)
	switch {
	case waiting:
		l.loadingGrid(c)
	case l.total == 0:
		l.emptyState(c)
	default:
		ui.Box(c).Children(func() {
			grid := ui.Box(c.Key("grid-wrap"))
			if l.loading {
				grid.Opacity(0.6).PassThrough()
			}
			grid.Children(func() { l.virtualGrid(c, winH) })
			if l.loading {
				// 加载中提示：absolute inset-x-0 top-0 flex justify-center py-3
				ui.Row(c).Absolute().Top(12).Left(0).Right(0).Justify(ui.Center).PassThrough().Children(func() {
					pill := ui.Row(c).Padding(6, 12).Radius(999).Gap(8).AlignItems(ui.Center).
						Background(brand900.Alpha(0.75)).Border(l.a.hl, brand700.Alpha(0.7)).
						Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
					if !p.dark {
						pill.Background(white.Alpha(0.85)).BorderColor(brand200.Alpha(0.7))
					}
					pill.Children(func() {
						l.spinner(c, 14, brand300)
						text(c, "加载中...", textSM, weightNormal, brand300)
					})
				})
			}
		})
	}
}

func (l *library) spinner(c *ui.Context, size float32, color ui.Color) {
	ic := icon(c, l.a.icon("loading"), size, color)
	ic.Rotate(ic.Loop("spin", time.Second, ui.Linear) * 360)
}

// loadingGrid 对应 LibraryGridLoadingState：16 张骨架卡片。
func (l *library) loadingGrid(c *ui.Context) {
	p := l.a.p
	// grid-cols-[repeat(auto-fill,minmax(max(8rem,11%),1fr))] gap-3
	minW := max(128, l.grid.width*0.11)
	cols := max(1, int((l.grid.width+gridGap)/(minW+gridGap)))
	cardW := (l.grid.width - gridGap*float32(cols-1)) / float32(cols)
	ui.Row(c).Wrap().GapX(gridGap).GapY(gridGap).Children(func() {
		for i := range 16 {
			card := ui.Column(c.Key(i)).Width(cardW).Radius(12).Clip().Background(p.cardBg).Border(l.a.hl, p.cardBorder).
				Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
			card.Opacity(pulse(card)).Children(func() {
				ui.Box(c).Height((cardW - 2*l.a.hl) * coverRatio(l.landscape)).Background(p.coverBg.Alpha(0.8))
				ui.Column(c).Padding(4, 8, 8, 8).Gap(4).Children(func() {
					ui.Box(c).Height(16).WidthPercent(80).Radius(4).Background(p.coverBg)
					ui.Box(c).Height(12).WidthPercent(60).Radius(4).Background(p.coverBg.Alpha(0.8))
				})
			})
		}
	})
}

// emptyState 对应 total === 0 时的两种空状态。
func (l *library) emptyState(c *ui.Context) {
	a := l.a
	p := a.p
	ui.Column(c).Grow(1).FillWidth().Center().Padding(80, 0).Children(func() {
		if l.hasActiveFilters() {
			icon(c, a.icon("magnify"), 60, p.textMuted).Margin(0, 0, 16, 0)
			text(c, "未找到匹配的游戏", textXL, weightNormal, p.textMuted)
			return
		}
		icon(c, a.icon("gamepad-variant-outline"), 60, p.textMuted).Margin(0, 0, 16, 0)
		text(c, "暂无游戏", textXL, weightNormal, p.textMuted)
		textMargin(c, "添加一些游戏开始吧", textSM, weightNormal, p.textMuted, 8, 0, 0, 0)
		ui.Grid(c).Columns(2).GapX(12).GapY(12).Width(512).Margin(20, 0, 0, 0).Children(func() {
			for _, imp := range importOptions(a) {
				btn := ui.ButtonBase(c.Key(imp.key)).Height(44).Padding(0, 20).Radius(999).Gap(8).Center().
					Background(brand800.Alpha(0.55)).Border(a.hl, brand600).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).FocusRing(false)
				if btn.Hovered() {
					btn.Background(brand700.Alpha(0.7)).BorderColor(brand500)
				}
				btn.Translate(0, -2*btn.AnimateWith("lift", boolf(btn.Hovered()), 150*time.Millisecond, easeTailwind))
				btn.OnClick(func() { a.toast(c, "原型未迁移导入流程") }).Children(func() {
					if imp.image != nil {
						ui.Image(c, imp.image).Size(20, 20).Radius(6).Fit(ui.Cover)
					} else {
						icon(c, a.icon(imp.icon), 20, p.text)
					}
					text(c, imp.label, textBase, weightMedium, p.text).SingleLine()
				})
			}
		})
	})
}

type importOption struct {
	key, label, icon string
	image            *ui.Bitmap
}

func importOptions(a *App) []importOption {
	im := a.assets.importers
	return []importOption{
		{"potatovn", "从 PotatoVN 导入", "", im["potatovn"]},
		{"playnite", "从 Playnite 导入", "", im["playnite"]},
		{"yukihub", "从 YukiHub 导入", "", im["yukihub"]},
		{"vnite", "从 Vnite 导入", "", im["vnite"]},
		{"reinamanager", "从 ReinaManager 导入", "", im["reinamanager"]},
		{"steam", "从 Steam 导入", "steam", nil},
	}
}

// scrollToTop 对应 ScrollToTopButton：fixed bottom-8 right-8 h-11 w-11 rounded-full。
func (l *library) scrollToTop(c *ui.Context) {
	a := l.a
	p := a.p
	// usePageScrollControls：工具栏滚出视口后显示
	visible := l.scroll.state.Y > 225
	btn := ui.ButtonBase(c.Key("scroll-top")).Absolute().Right(32).Bottom(32).Size(44, 44).Radius(999).Center().
		Border(a.hl, p.panelBorder).Background(p.panelBg.Alpha(0.92)).FocusRing(false).Label("回到顶部").
		Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
	show := btn.AnimateWith("show", boolf(visible), 200*time.Millisecond, easeTailwind)
	if show == 0 {
		btn.Invisible()
		return
	}
	hover := btn.AnimateWith("hover", boolf(btn.Hovered()), 200*time.Millisecond, easeTailwind)
	if btn.Hovered() {
		btn.Background(brand700)
		if !p.dark {
			btn.Background(brand100)
		}
	}
	btn.Opacity(show).Translate(0, 8*(1-show)-2*hover)
	if !visible {
		btn.PassThrough()
	}
	btn.OnClick(func() { l.scroll.state.ScrollTo(0, 0, ui.Animation{Duration: 400 * time.Millisecond, Ease: easeCSS}) }).Children(func() {
		color := brand200
		if !p.dark {
			color = brand700
		}
		icon(c, a.icon("arrow-up"), 20, color)
	})
}

// overlays 构建页面级浮层：筛选抽屉。
func (l *library) overlays(c *ui.Context) {
	if l.a.page != PageLibrary {
		return
	}
	l.filterDrawer(c)
}
