// Package game 是游戏详情页，对应原版 routes/game.tsx，以及“游戏统计”标签页 GameStatsPanel。
// 其余标签页（编辑、启动配置、备份、游玩进度、评价）与各个弹窗还没有迁移。
package game

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
)

// Links 是详情页跳回游戏库的入口：点击开发商按名称搜索，点击标签按标签筛选。
type Links struct {
	SearchLibrary func(query string)
	FilterByTag   func(tag string)
}

// 标签页，顺序与原版一致。
var tabs = []struct{ key, label string }{
	{"stats", "游戏统计"},
	{"edit", "编辑"},
	{"launch", "启动配置"},
	{"backup", "备份"},
	{"progress", "游玩进度"},
	{"review", "评价"},
}

// skeletonDelay 是原版显示骨架屏前的等待，数据很快返回时不闪一下骨架。
const skeletonDelay = 300 * time.Millisecond

// Page 是详情页的状态。每次进入页面（Open）都重新加载，与原版路由重新挂载组件一致。
type Page struct {
	env   *env.Env
	links Links

	id       string
	gen      int
	game     *models.Game
	tags     []models.GameTag
	loading  bool
	openedAt time.Time

	tab        string
	launchMode enums.LaunchMode
	statusOpen bool // 窄屏时状态下拉菜单
	scroll     components.ScrollBar

	stats statsPanel
}

// New 创建页面。数据在 Open 时加载。
func New(e *env.Env, links Links) *Page {
	return &Page{env: e, links: links, tab: "stats"}
}

// Open 进入某个游戏的详情页：清空上一个游戏的状态并加载数据。
func (p *Page) Open(id string, now time.Time) {
	p.id = id
	p.gen++
	p.game, p.tags = nil, nil
	p.loading, p.openedAt = true, now
	p.tab = "stats"
	p.statusOpen = false
	p.scroll = components.ScrollBar{}
	p.stats = statsPanel{}

	gen := p.gen
	p.env.Async(func() func() {
		g, err := p.env.Backend.GetGameByID(id)
		tags, _ := p.env.Backend.GetTagsByGame(id)
		return func() {
			if gen != p.gen {
				return
			}
			p.loading = false
			if err != nil {
				return
			}
			p.game, p.tags = &g, tags
			p.launchMode = defaultLaunchMode(g)
		}
	})
	p.stats.load(p.env, id)
}

// Title 是页面标题（窗口历史里显示）。
func (p *Page) Title() string {
	if p.game != nil {
		return p.game.Name
	}
	return "游戏详情"
}

// Loading 报告游戏数据是否还在加载（截图用）。
func (p *Page) Loading() bool { return p.loading || p.stats.loading }

// SetTab 切换标签页（截图调试用）。
func (p *Page) SetTab(tab string) { p.tab = tab }

// defaultLaunchMode 对应 defaultLaunchModeForGame；原型只在 Windows 上运行，没有兼容层启动。
func defaultLaunchMode(g models.Game) enums.LaunchMode {
	switch g.LaunchMode {
	case enums.LaunchModeSteam, enums.LaunchModeAdmin:
		return g.LaunchMode
	}
	return enums.LaunchModeNormal
}

// launchOptions 是 Windows 上的启动方式：普通、Steam、管理员。
var launchOptions = []components.SplitOption{
	{Key: string(enums.LaunchModeNormal), Label: "启动游戏", Description: "按当前游戏启动配置运行", Icon: "play"},
	{Key: string(enums.LaunchModeSteam), Label: "通过 Steam 启动", Description: "使用 steam 客户端启动该游戏", Icon: "steam"},
	{Key: string(enums.LaunchModeAdmin), Label: "管理员启动", Description: "触发 UAC 并以管理员权限运行", Icon: "shield-account"},
}

func launchOption(mode enums.LaunchMode) components.SplitOption {
	for _, o := range launchOptions {
		if o.Key == string(mode) {
			return o
		}
	}
	return launchOptions[0]
}

// currentStatus 是游戏的状态，未知状态按“未开始”。
func currentStatus(g *models.Game) enums.GameStatus {
	for _, s := range components.GameStatuses {
		if g.Status == s {
			return s
		}
	}
	return enums.StatusNotStarted
}

// metadataSource 是一个有网页的元数据来源。
type metadataSource struct {
	source enums.SourceType
	id     string
	url    string
}

// metadataSources 对应 metadataSources：没有来源列表时退回默认来源。
func metadataSources(g *models.Game) []models.GameMetadataSource {
	if len(g.MetadataSources) > 0 {
		return g.MetadataSources
	}
	if g.SourceType != "" && g.SourceID != "" {
		return []models.GameMetadataSource{{SourceType: g.SourceType, SourceID: g.SourceID}}
	}
	return nil
}

// metadataSourceText 对应 metadataSourceText：“bangumi 等 2 个”。
func metadataSourceText(g *models.Game) string {
	if g.SourceType == "" {
		return "-"
	}
	if n := len(metadataSources(g)); n > 1 {
		return fmt.Sprintf("%s 等 %d 个", g.SourceType, n)
	}
	return string(g.SourceType)
}

// sourceLinks 对应 metadataSourceLinks：有网页的来源，默认来源排在最前。
func sourceLinks(g *models.Game, erogameScapeBase string) []metadataSource {
	var def, rest []metadataSource
	for _, s := range metadataSources(g) {
		id := strings.TrimSpace(s.SourceID)
		u := sourceURL(s.SourceType, id, erogameScapeBase)
		if u == "" {
			continue
		}
		link := metadataSource{source: s.SourceType, id: id, url: u}
		if s.SourceType == g.SourceType {
			def = append(def, link)
		} else {
			rest = append(rest, link)
		}
	}
	return append(def, rest...)
}

// sourceURL 对应 utils/metadataSources.ts 的 getMetadataSourceURL。
func sourceURL(source enums.SourceType, id, erogameScapeBase string) string {
	if source == "" || id == "" {
		return ""
	}
	id = strings.ReplaceAll(url.QueryEscape(id), "+", "%20") // encodeURIComponent
	switch source {
	case enums.VNDB:
		return "https://vndb.org/" + id
	case enums.Bangumi:
		return "https://bgm.tv/subject/" + id
	case enums.Ymgal:
		return "https://www.ymgal.games/ga/" + id
	case enums.Steam:
		return "https://store.steampowered.com/app/" + id
	case enums.DLsite:
		return "https://www.dlsite.com/maniax/work/=/product_id/" + id + ".html"
	case enums.TouchGal:
		return "https://www.touchgal.ink/" + id
	case enums.Hikarinagi:
		return "https://www.hikarinagi.org/galgames/" + id
	case enums.ErogameScape:
		base := strings.TrimRight(strings.TrimSpace(erogameScapeBase), "/")
		if base == "" {
			base = appconf.DefaultErogameScapeBaseURL
		}
		return base + "/game.php?game=" + id
	}
	return ""
}

// sourceLabel 对应 importFlow.ts 的 sourceLabel。
func sourceLabel(source enums.SourceType) string {
	switch source {
	case enums.Bangumi:
		return "Bangumi"
	case enums.VNDB:
		return "VNDB"
	case enums.Ymgal:
		return "月幕gal"
	case enums.DLsite:
		return "DLsite"
	case enums.TouchGal:
		return "TouchGAL"
	case enums.Hikarinagi:
		return "Hikarinagi"
	case enums.ErogameScape:
		return "ErogameScape"
	}
	return "Steam"
}
