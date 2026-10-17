// Package library 是游戏库页面，对应原版 routes/library.tsx。FilterBar（含 TagFilterMenu、GameFilterPresetMenu）
// 在 components/bar。页面状态在 Page 里，路由切走再回来时保持不变。
package library

import (
	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/bar"
	"lunabox/desktop-mygo/ui/components/grid"
	"lunabox/desktop-mygo/ui/env"
)

// Page 是游戏库页面的状态。
type Page struct {
	env *env.Env

	filters   bar.Filters
	pages     *grid.Pages
	blurNSFW  bool
	landscape bool

	// 界面
	scroll        components.ScrollBar
	addMenuOpen   bool
	selected      map[string]bool
	randomLoading bool
	grid          grid.Geometry
}

// New 创建页面并按配置初始化卡片布局与“封面展示排序字段”。数据在 Reload 时加载。
func New(e *env.Env) *Page {
	l := &Page{
		env: e,
		filters: bar.Filters{
			SortBy:             string(enums.GameListSortByCreatedAt),
			SortOrder:          enums.SortOrderDesc,
			SecondarySortOrder: enums.SortOrderAsc,
		},
		pages:    grid.NewPages(e),
		selected: map[string]bool{},
	}
	if cfg := e.Config(); cfg != nil {
		l.filters.ShowSortField = cfg.ShowSortFieldOnCover
		l.blurNSFW = cfg.BlurNSFWGameCovers
		l.landscape = cfg.GameCardLayout == "landscape"
	}
	return l
}

// Scroll 是页面的滚动状态（压测用）。
func (l *Page) Scroll() *components.ScrollBar { return &l.scroll }

// Loading 报告第一页是否还在加载。
func (l *Page) Loading() bool { return l.pages.Loading }

// Stats 返回总数与已加载条数（HUD 用）。
func (l *Page) Stats() (total, loaded int) { return l.pages.Total, len(l.pages.Games) }

// SetDebugState 让截图直接进入某个界面状态：drawer、addmenu、batch。
func (l *Page) SetDebugState(state string) {
	switch state {
	case "drawer":
		l.filters.DrawerOpen = true
	case "addmenu":
		l.addMenuOpen = true
	case "batch":
		l.filters.Batch = true
	}
}

// Reload 在查询条件变化后重新请求第一页；旧数据保留到新结果返回（期间显示“加载中”）。
func (l *Page) Reload() {
	req := l.filters.GameListRequest()
	l.pages.Reload(l.filters.Key(), func(offset, limit int) ([]models.Game, int, error) {
		r := req
		r.Offset, r.Limit = offset, limit
		resp, err := l.env.Backend.GetGames(r)
		return resp.Games, resp.Total, err
	})
	l.filters.LoadPresets(l.env)
}

func (l *Page) toggleSelected(id string) {
	if l.selected[id] {
		delete(l.selected, id)
	} else {
		l.selected[id] = true
	}
}

// SearchFor 对应以 search.searchQuery 进入游戏库：按名称搜索（详情页点击开发商）。
func (l *Page) SearchFor(query string) {
	l.filters.SearchFor(query)
	l.Reload()
}

// FilterByTag 对应以 search.tagFilter 进入游戏库：只按这个标签筛选（详情页点击标签）。
func (l *Page) FilterByTag(tag string) {
	l.filters.FilterByTag(tag)
	l.Reload()
}
