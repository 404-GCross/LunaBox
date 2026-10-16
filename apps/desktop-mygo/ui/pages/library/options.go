package library

import (
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/internal/common/enums"
)

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

var sortOptions = []better.SelectOption{
	{Label: "名称", Value: string(enums.GameListSortByName)},
	{Label: "开发商", Value: string(enums.GameListSortByCompany)},
	{Label: "最近游玩", Value: string(enums.GameListSortByLastPlayedAt)},
	{Label: "添加时间", Value: string(enums.GameListSortByCreatedAt)},
	{Label: "评分", Value: string(enums.GameListSortByRating)},
	{Label: "发售日期", Value: string(enums.GameListSortByReleaseDate)},
}

type importOption struct {
	key, label, description, icon string
}

// importOptions 对应 EMPTY_STATE_IMPORT_OPTIONS 与“添加游戏”菜单里的导入来源。
var importOptions = []importOption{
	{"potatovn", "从 PotatoVN 导入", "导入 PotatoVN 导出的 ZIP 文件", ""},
	{"playnite", "从 Playnite 导入", "导入 Playnite 导出的 ZIP 或 JSON 文件", ""},
	{"yukihub", "从 YukiHub 导入", "导入 YukiHub 导出的 YKBAK 备份", ""},
	{"vnite", "从 Vnite 导入", "导入 Vnite 导出的本地数据库目录", ""},
	{"reinamanager", "从 ReinaManager 导入", "导入 ReinaManager 的 SQLite 数据库备份", ""},
	{"steam", "从 Steam 导入", "扫描本机已安装的 Steam 游戏", "steam"},
}
