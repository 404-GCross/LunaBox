package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"

	"lunabox/internal/appconf"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service"
	"lunabox/internal/utils/apputils"
)

// backend 用主程序的真实服务实现 ui.Backend。数据库是临时拷贝，不会与正在运行的 LunaBox 抢锁，
// 也不会写回用户数据。
type backend struct {
	db       *sql.DB
	config   *appconf.AppConfig
	dataDir  string
	games    *service.GameService
	tags     *service.TagService
	presets  *service.GameFilterPresetService
	stats    *service.StatsService
	sessions *service.SessionService
}

func openBackend(ctx context.Context) (*backend, error) {
	dataDir, err := apputils.GetDataDir()
	if err != nil {
		return nil, fmt.Errorf("get data dir: %w", err)
	}
	config, err := appconf.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	tmpDir := filepath.Join(os.TempDir(), "lunabox-mygo-prototype")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}
	src := filepath.Join(dataDir, "lunabox.db")
	dst := filepath.Join(tmpDir, "lunabox.db")
	if err := copyFile(src, dst); err != nil {
		return nil, fmt.Errorf("copy database: %w", err)
	}
	_ = os.Remove(dst + ".wal")
	if _, err := os.Stat(src + ".wal"); err == nil {
		if err := copyFile(src+".wal", dst+".wal"); err != nil {
			return nil, fmt.Errorf("copy database wal: %w", err)
		}
	}
	db, err := sql.Open("duckdb", dst)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if tz := config.TimeZone; tz != "" {
		_, _ = db.Exec(fmt.Sprintf("SET TimeZone = '%s'", tz))
	}

	b := &backend{
		db:       db,
		config:   config,
		dataDir:  dataDir,
		games:    service.NewGameService(),
		tags:     service.NewTagService(),
		presets:  service.NewGameFilterPresetService(),
		stats:    service.NewStatsService(),
		sessions: service.NewSessionService(),
	}
	b.games.Init(ctx, db, config)
	b.tags.Init(ctx, db, config)
	b.presets.Init(ctx, db, config)
	b.stats.Init(ctx, db, config)
	b.sessions.Init(ctx, db, config)
	return b, nil
}

func (b *backend) GetGames(req vo.GameListRequest) (vo.GameListResponse, error) {
	return b.games.GetGames(req)
}

func (b *backend) SearchTagsInLibrary(query string) ([]string, error) {
	return b.tags.SearchTagsInLibrary(query)
}

func (b *backend) ListGameFilterPresets() ([]models.GameFilterPreset, error) {
	return b.presets.ListGameFilterPresets()
}

func (b *backend) GetGameByID(id string) (models.Game, error) { return b.games.GetGameByID(id) }

func (b *backend) GetTagsByGame(gameID string) ([]models.GameTag, error) {
	return b.tags.GetTagsByGame(gameID)
}

func (b *backend) GetGameStats(req vo.GameStatsRequest) (vo.GameDetailStats, error) {
	return b.stats.GetGameStats(req)
}

func (b *backend) GetPlaySessions(gameID string) ([]models.PlaySession, error) {
	return b.sessions.GetPlaySessions(gameID)
}

func (b *backend) Config() *appconf.AppConfig { return b.config }
func (b *backend) DataDir() string            { return b.dataDir }

// repeatedBackend 把列表虚拟放大 n 倍，用于压测大列表；封面相同，所以封面缓存不会随之膨胀。
type repeatedBackend struct {
	*backend
	n int
}

func (r repeatedBackend) GetGames(req vo.GameListRequest) (vo.GameListResponse, error) {
	var base []models.Game
	page := req
	page.Limit = 240 // gamehelper.MaxGameListLimit
	for page.Offset = 0; ; page.Offset += page.Limit {
		resp, err := r.backend.GetGames(page)
		if err != nil {
			return resp, err
		}
		base = append(base, resp.Games...)
		if !resp.HasMore || len(resp.Games) == 0 {
			break
		}
	}
	if len(base) == 0 {
		return vo.GameListResponse{Limit: req.Limit, Offset: req.Offset}, nil
	}
	resp := vo.GameListResponse{Games: base}
	n := len(resp.Games)
	out := vo.GameListResponse{Limit: req.Limit, Offset: req.Offset, Total: n * r.n}
	for i := req.Offset; i < min(out.Total, req.Offset+req.Limit); i++ {
		g := resp.Games[i%n]
		if copyIndex := i / n; copyIndex > 0 {
			g.ID = fmt.Sprintf("%s#%d", g.ID, copyIndex)
			g.Name = fmt.Sprintf("%s #%d", g.Name, copyIndex)
		}
		out.Games = append(out.Games, g)
	}
	out.HasMore = req.Offset+len(out.Games) < out.Total
	return out, nil
}

// 放大出来的副本 ID 形如 "<原 ID>#<序号>"，详情页按原 ID 查询。
func baseID(id string) string {
	if i := strings.LastIndexByte(id, '#'); i >= 0 {
		return id[:i]
	}
	return id
}

func (r repeatedBackend) GetGameByID(id string) (models.Game, error) {
	g, err := r.backend.GetGameByID(baseID(id))
	g.ID = id
	return g, err
}

func (r repeatedBackend) GetTagsByGame(id string) ([]models.GameTag, error) {
	return r.backend.GetTagsByGame(baseID(id))
}

func (r repeatedBackend) GetGameStats(req vo.GameStatsRequest) (vo.GameDetailStats, error) {
	req.GameID = baseID(req.GameID)
	return r.backend.GetGameStats(req)
}

func (r repeatedBackend) GetPlaySessions(id string) ([]models.PlaySession, error) {
	return r.backend.GetPlaySessions(baseID(id))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
