package models

import (
	"lunabox/internal/common/enums"
	"time"
)

type GameFilterPreset struct {
	ID                 string               `json:"id"`
	Name               string               `json:"name"`
	Tags               []string             `json:"tags"`
	ExcludeTags        bool                 `json:"exclude_tags"`
	Status             enums.GameStatus     `json:"status"`
	ExcludeStatus      bool                 `json:"exclude_status"`
	MetadataSource     enums.SourceType     `json:"metadata_source"`
	SortBy             enums.GameListSortBy `json:"sort_by"`
	SortOrder          enums.SortOrder      `json:"sort_order"`
	SecondarySortBy    enums.GameListSortBy `json:"secondary_sort_by"`
	SecondarySortOrder enums.SortOrder      `json:"secondary_sort_order"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}
