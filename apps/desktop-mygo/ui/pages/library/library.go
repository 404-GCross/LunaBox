// Package library 是游戏库页面，对应原版 routes/library.tsx，以及它专用的 FilterBar、TagFilterMenu、
// GameFilterPresetMenu 部分。页面状态在 Page 里，路由切走再回来时保持不变。
package library

import (
	"fmt"
	"strings"
	"time"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/grid"
	"lunabox/desktop-mygo/ui/env"
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

// Page 是游戏库页面的状态。
type Page struct {
	env *env.Env

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
	loadedKey string
	queryKey  string
	gen       int
	inflight  map[string]bool
	presets   []models.GameFilterPreset

	// 界面
	scroll        components.ScrollBar
	drawerOpen    bool
	addMenuOpen   bool
	batch         bool
	selected      map[string]bool
	randomLoading bool
	grid          grid.Geometry
}

// New 创建页面并按配置初始化卡片布局与“封面展示排序字段”。数据在 Reload 时加载。
func New(e *env.Env) *Page {
	l := &Page{
		env:                e,
		sortBy:             enums.GameListSortByCreatedAt,
		sortOrder:          enums.SortOrderDesc,
		secondarySortOrder: enums.SortOrderAsc,
		games:              map[int]*models.Game{},
		inflight:           map[string]bool{},
		selected:           map[string]bool{},
		loading:            true,
	}
	if cfg := e.Config(); cfg != nil {
		l.showSortField = cfg.ShowSortFieldOnCover
		l.landscape = cfg.GameCardLayout == "landscape"
	}
	return l
}

// Scroll 是页面的滚动状态（压测用）。
func (l *Page) Scroll() *components.ScrollBar { return &l.scroll }

// Loading 报告第一页是否还在加载。
func (l *Page) Loading() bool { return l.loading }

// Stats 返回总数与已加载条数（HUD 用）。
func (l *Page) Stats() (total, loaded int) { return l.total, len(l.games) }

// SetDebugState 让截图直接进入某个界面状态：drawer、addmenu、batch。
func (l *Page) SetDebugState(state string) {
	switch state {
	case "drawer":
		l.drawerOpen = true
	case "addmenu":
		l.addMenuOpen = true
	case "batch":
		l.batch = true
	}
}

// request 组装 GetGames 的查询条件（不含分页）。
func (l *Page) request() vo.GameListRequest {
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

func (l *Page) key() string {
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

// Reload 在查询条件变化后重新请求第一页；旧数据保留到新结果返回（期间显示“加载中”）。
func (l *Page) Reload() {
	l.gen++
	l.queryKey = l.key()
	l.loading = true
	l.inflight = map[string]bool{}
	l.fetch(0, windowRequestSize, true)
	l.loadPresets()
}

// ensureWindow 在可见范围有缺口时请求覆盖它的窗口（getWindowRequest）。
func (l *Page) ensureWindow(start, end int) {
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

func (l *Page) fetch(offset, limit int, replace bool) {
	key := fmt.Sprintf("%d:%d:%d", l.gen, offset, limit)
	if l.inflight[key] {
		return
	}
	l.inflight[key] = true
	gen := l.gen
	req := l.request()
	req.Offset, req.Limit = offset, limit
	l.env.Async(func() func() {
		resp, err := l.env.Backend.GetGames(req)
		return func() {
			delete(l.inflight, key)
			if gen != l.gen {
				return
			}
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
			l.loadedKey = l.queryKey
		}
	})
}

func (l *Page) loadPresets() {
	l.env.Async(func() func() {
		presets, err := l.env.Backend.ListGameFilterPresets()
		return func() {
			if err == nil {
				l.presets = presets
			}
		}
	})
}

func (l *Page) searchTags(query string) {
	if strings.TrimSpace(query) == "" {
		l.tagSuggestions = nil
		return
	}
	l.env.Async(func() func() {
		names, err := l.env.Backend.SearchTagsInLibrary(strings.TrimSpace(query))
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

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (l *Page) hasActiveFilters() bool {
	return strings.TrimSpace(l.debounced) != "" || len(l.tags) > 0 || l.status != "" || l.source != ""
}

func (l *Page) activeFilterCount() int {
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

func (l *Page) countText() string {
	label := statusLabel(l.status)
	switch {
	case l.status != "" && l.statusInverted:
		return fmt.Sprintf("共 %d 个非 %s 游戏", l.total, label)
	case l.status != "":
		return fmt.Sprintf("共 %d 个 %s 游戏", l.total, label)
	}
	return fmt.Sprintf("共 %d 个游戏", l.total)
}

func (l *Page) toggleSelected(id string) {
	if l.selected[id] {
		delete(l.selected, id)
	} else {
		l.selected[id] = true
	}
}

func (l *Page) clearFiltersAndSort() {
	l.search, l.debounced = "", ""
	l.status, l.statusInverted = "", false
	l.source, l.sourceInverted = "", false
	l.tags, l.tagsInverted, l.tagInput, l.tagSuggestions = nil, false, "", nil
	l.sortBy, l.sortOrder = enums.GameListSortByCreatedAt, enums.SortOrderDesc
	l.secondarySortBy, l.secondarySortOrder = "", enums.SortOrderAsc
	l.Reload()
}

func (l *Page) addTag(tag string) {
	if !contains(l.tags, tag) {
		l.tags = append(l.tags, tag)
	}
	l.tagInput, l.tagSuggestions = "", nil
	l.Reload()
}

func (l *Page) removeTag(tag string) {
	out := l.tags[:0:0]
	for _, t := range l.tags {
		if t != tag {
			out = append(out, t)
		}
	}
	l.tags = out
	if len(out) == 0 {
		l.tagsInverted = false
	}
	l.Reload()
}

func (l *Page) applyPreset(preset models.GameFilterPreset) {
	l.tags, l.tagsInverted = preset.Tags, len(preset.Tags) > 0 && preset.ExcludeTags
	l.status, l.statusInverted = preset.Status, preset.Status != "" && preset.ExcludeStatus
	l.source, l.sourceInverted = preset.MetadataSource, false
	if preset.SortBy != "" {
		l.sortBy, l.sortOrder = preset.SortBy, preset.SortOrder
	}
	l.secondarySortBy, l.secondarySortOrder = preset.SecondarySortBy, preset.SecondarySortOrder
	l.Reload()
}
