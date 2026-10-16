package library

import (
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/internal/common/enums"
)

// statusOptions 对应 consts/options.ts 的 statusOptions。
var statusOptions = []struct {
	label func() string
	value enums.GameStatus
}{
	{locales.CommonAllStatus, ""},
	{locales.CommonNotStarted, enums.StatusNotStarted},
	{locales.CommonWantToPlay, enums.StatusWantToPlay},
	{locales.CommonPlaying, enums.StatusPlaying},
	{locales.CommonCompleted, enums.StatusCompleted},
	{locales.CommonOnHold, enums.StatusOnHold},
	{locales.CommonDropped, enums.StatusDropped},
}

func statusLabel(s enums.GameStatus) string {
	for _, o := range statusOptions {
		if o.value == s {
			return o.label()
		}
	}
	return string(s)
}

func literal(s string) func() string { return func() string { return s } }

// sourceOptions 对应 library.tsx 的 metadataSourceOptions（来源名称同 importFlow.sourceLabel）。
var sourceOptions = []struct {
	label func() string
	value enums.SourceType
}{
	{locales.FilterBarAllMetadataSources, ""},
	{locales.FilterBarNoMetadataSource, enums.Local},
	{literal("Bangumi"), enums.Bangumi},
	{literal("VNDB"), enums.VNDB},
	{locales.GameEditSourceYmgal, enums.Ymgal},
	{locales.GameEditSourceDlsite, enums.DLsite},
	{locales.GameEditSourceTouchGal, enums.TouchGal},
	{locales.GameEditSourceHikarinagi, enums.Hikarinagi},
	{locales.GameEditSourceErogameScape, enums.ErogameScape},
	{literal("Steam"), enums.Steam},
}

// sortOptions 对应 consts/options.ts 的 sortOptions。
func sortOptions() []better.SelectOption {
	return []better.SelectOption{
		{Label: locales.CommonName(), Value: string(enums.GameListSortByName)},
		{Label: locales.CommonCompany(), Value: string(enums.GameListSortByCompany)},
		{Label: locales.CommonLastPlayedAt(), Value: string(enums.GameListSortByLastPlayedAt)},
		{Label: locales.CommonCreatedAt(), Value: string(enums.GameListSortByCreatedAt)},
		{Label: locales.CommonRating(), Value: string(enums.GameListSortByRating)},
		{Label: locales.CommonReleaseDate(), Value: string(enums.GameListSortByReleaseDate)},
	}
}

type importOption struct {
	key, icon          string
	label, description func() string
}

// importOptions 对应 EMPTY_STATE_IMPORT_OPTIONS 与“添加游戏”菜单里的导入来源。
var importOptions = []importOption{
	{"potatovn", "", locales.LibraryImportPotatoVN, locales.LibraryImportPotatoVNDesc},
	{"playnite", "", locales.LibraryImportPlaynite, locales.LibraryImportPlayniteDesc},
	{"yukihub", "", locales.LibraryImportYukiHub, locales.LibraryImportYukiHubDesc},
	{"vnite", "", locales.LibraryImportVnite, locales.LibraryImportVniteDesc},
	{"reinamanager", "", locales.LibraryImportReinaManager, locales.LibraryImportReinaManagerDesc},
	{"steam", "steam", locales.LibraryImportSteam, locales.LibraryImportSteamDesc},
}
