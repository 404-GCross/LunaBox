// Package envtest 提供测试用的内存后端。
package envtest

import (
	"fmt"
	"strings"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
)

// Backend 在内存中实现 env.Backend，按名称搜索与状态筛选，其余条件忽略。
type Backend struct {
	Games []models.Game
	Cfg   appconf.AppConfig
	Reqs  []vo.GameListRequest
}

// NewBackend 返回 n 个游戏、侧边栏展开、封面显示排序字段的后端。
func NewBackend(n int) *Backend {
	b := &Backend{Cfg: appconf.AppConfig{SidebarOpen: true, ShowSortFieldOnCover: true}}
	names := []string{"橘光柠水随想曲", "Summer Pockets", "星空鉄道とシロの旅"}
	for i := range n {
		b.Games = append(b.Games, models.Game{
			ID:        fmt.Sprint(i),
			Name:      names[i%len(names)],
			Company:   "ゆずソフト",
			CreatedAt: time.Date(2026, 9, 11, 12, 0, 0, 0, time.Local),
			Status:    []enums.GameStatus{enums.StatusNotStarted, enums.StatusCompleted}[i%2],
		})
	}
	return b
}

func (b *Backend) GetGames(req vo.GameListRequest) (vo.GameListResponse, error) {
	b.Reqs = append(b.Reqs, req)
	var match []models.Game
	for _, g := range b.Games {
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

func (b *Backend) SearchTagsInLibrary(string) ([]string, error)              { return nil, nil }
func (b *Backend) ListGameFilterPresets() ([]models.GameFilterPreset, error) { return nil, nil }
func (b *Backend) Config() *appconf.AppConfig                                { return &b.Cfg }
func (b *Backend) DataDir() string                                           { return "" }
