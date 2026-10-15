package library

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
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

// statusPill 对应 library.tsx 的 statusConfig：批量“设为状态”菜单的图标与胶囊颜色。
func statusPill(t *theme.Theme, s enums.GameStatus) (icon string, bg, fg ui.Color) {
	switch s {
	case enums.StatusWantToPlay: // info
		return "bookmark-outline", t.Pick(ui.Hex("#e0f2fe"), ui.Hex("#0c181d")), t.Pick(ui.Hex("#0369a1"), ui.Hex("#7dd3fc"))
	case enums.StatusPlaying: // neutral
		return "gamepad-variant", t.Pick(theme.Neutral100, ui.Hex("#0f172a")), t.Pick(theme.Neutral700, theme.Neutral300)
	case enums.StatusCompleted: // yellow
		return "trophy", t.Pick(ui.Hex("#fef9c3"), ui.Hex("#713f12")), t.Pick(ui.Hex("#a16207"), ui.Hex("#fde047"))
	case enums.StatusOnHold: // orange
		return "pause-circle-outline", t.Pick(ui.Hex("#ffedd5"), ui.Hex("#7c2d12")), t.Pick(ui.Hex("#c2410c"), ui.Hex("#fdba74"))
	case enums.StatusDropped: // rose
		return "delete-outline", t.Pick(ui.Hex("#ffe4e6"), ui.Hex("#881337")), t.Pick(ui.Hex("#be123c"), ui.Hex("#fda4af"))
	}
	// gray
	return "clock-outline", t.Pick(ui.Hex("#f3f4f6"), ui.Hex("#374151")), t.Pick(ui.Hex("#374151"), ui.Hex("#d1d5db"))
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

var sortOptions = []components.SelectOption{
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
