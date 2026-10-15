package backend

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/duckdb/duckdb-go/v2"

	"lunabox/internal/appconf"
	"lunabox/internal/migrations"
	"lunabox/internal/service"
	"lunabox/internal/utils/dbutils"
	"lunabox/internal/wailsruntime"
)

// Services 是原型用到的 apps/desktop 真实服务，构造与相互注入照搬 apps/desktop/main.go 的 initBoundServices。
// 不启动的后台任务：云同步、定时数据库备份、MCP、更新检查。
type Services struct {
	DB      *sql.DB
	Config  *appconf.AppConfig
	DataDir string
	Events  *Events

	AppConfig   *service.ConfigService
	Game        *service.GameService
	Tag         *service.TagService
	Preset      *service.GameFilterPresetService
	Stats       *service.StatsService
	Session     *service.SessionService
	Category    *service.CategoryService
	Backup      *service.BackupService
	Start       *service.StartService
	Integration *service.IntegrationService
	Progress    *service.GameProgressService
	Review      *service.GameReviewService
	Bangumi     *service.BangumiService
	Hikarinagi  *service.HikarinagiService
	Download    *service.DownloadService // 远程封面批量下载（更新元数据后）
}

// Open 读取 dataDir（须先 UseDataDir）里的配置与数据库，执行与主程序相同的迁移，再初始化服务。
func Open(ctx context.Context, dataDir string) (*Services, error) {
	return OpenFile(ctx, dataDir, filepath.Join(dataDir, "lunabox.db"))
}

// OpenFile 同 Open，但数据库在 dbPath（测试里每个用例一个数据库文件）。
func OpenFile(ctx context.Context, dataDir, dbPath string) (*Services, error) {
	config, err := appconf.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	db, err := dbutils.OpenDuckDBWithWALRecovery(ctx, dbPath, nil)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	tz := config.TimeZone
	if tz == "" {
		tz = "UTC"
	}
	if _, err := db.Exec(fmt.Sprintf("SET TimeZone = '%s'", tz)); err != nil {
		db.Close()
		return nil, fmt.Errorf("set timezone: %w", err)
	}
	if err := migrations.InitSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := migrations.InitIndexes(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("init indexes: %w", err)
	}

	s := &Services{
		DB: db, Config: config, DataDir: dataDir, Events: NewEvents(),
		AppConfig:   service.NewConfigService(),
		Game:        service.NewGameService(),
		Tag:         service.NewTagService(),
		Preset:      service.NewGameFilterPresetService(),
		Stats:       service.NewStatsService(),
		Session:     service.NewSessionService(),
		Category:    service.NewCategoryService(),
		Backup:      service.NewBackupService(),
		Start:       service.NewStartService(),
		Integration: service.NewIntegrationService(),
		Progress:    service.NewGameProgressService(),
		Review:      service.NewGameReviewService(),
		Bangumi:     service.NewBangumiService(),
		Hikarinagi:  service.NewHikarinagiService(),
		Download:    service.NewDownloadService(),
	}
	s.AppConfig.Init(ctx, db, config)
	s.Download.Init(ctx, db, config)
	s.Game.Init(ctx, db, config)
	s.Bangumi.Init(ctx, db, config)
	s.Hikarinagi.Init(ctx, db, config)
	s.Tag.Init(ctx, db, config)
	s.Preset.Init(ctx, db, config)
	s.Backup.Init(ctx, db, config)
	s.Stats.Init(ctx, db, config)
	s.Session.Init(ctx, db, config)
	s.Start.Init(ctx, db, config)
	s.Integration.Init(ctx, db, config)
	s.Category.Init(ctx, db, config)
	s.Progress.Init(ctx, db, config)
	s.Review.Init(ctx, db, config)

	s.Start.SetBackupService(s.Backup)
	s.Start.SetGameService(s.Game)
	s.Start.SetIntegrationService(s.Integration)
	s.Start.SetSessionService(s.Session)
	s.Game.SetTagService(s.Tag)
	s.Game.SetBangumiService(s.Bangumi)
	s.Game.SetHikarinagiService(s.Hikarinagi)
	s.Review.SetBangumiService(s.Bangumi)
	s.Review.SetHikarinagiService(s.Hikarinagi)
	s.Integration.SetGameService(s.Game)
	s.Download.SetGameService(s.Game)
	s.AppConfig.SetDownloadService(s.Download)
	s.Game.SetImageDownloadTaskStarter(s.Download.StartCoverImageDownloadTask)
	return s, nil
}

// SetRuntime 把对话框、事件与打开网址交给 rt（主程序里是 Wails 的窗口，这里是 MyGo 的）。
func (s *Services) SetRuntime(rt wailsruntime.Runtime) {
	s.AppConfig.SetRuntime(rt)
	s.Download.SetRuntime(rt)
	s.Backup.SetRuntime(rt)
	s.Bangumi.SetRuntime(rt)
	s.Hikarinagi.SetRuntime(rt)
	s.Game.SetRuntime(rt)
	s.Start.SetRuntime(rt)
	s.Stats.SetRuntime(rt)
}

// Close 关闭数据库。
func (s *Services) Close() error {
	_ = dbutils.CheckpointDuckDB(context.Background(), s.DB)
	return s.DB.Close()
}
