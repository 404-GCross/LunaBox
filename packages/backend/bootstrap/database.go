package bootstrap

import (
	"context"
	"database/sql"
	"fmt"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/migrations"
	"lunabox/backend/service"
	"lunabox/backend/utils/dbutils"
	_ "lunabox/backend/utils/dbutils/duckdbdriver" // DuckDB 驱动与 Appender
)

// RunPendingRestores 执行设置页安排的恢复：先恢复完整数据，再恢复数据库，都要在打开数据库之前。
func RunPendingRestores(config *appconf.AppConfig, logger *applog.FileLogger) {
	if config.PendingFullRestore != "" {
		restored, err := service.ExecuteFullDataRestore(config)
		if err != nil {
			logger.Error("full data restore failed: " + err.Error())
		} else if restored {
			logger.Info("full data restore completed")
		}
	}
	if config.PendingDBRestore != "" {
		restored, err := service.ExecuteDBRestore(config)
		if err != nil {
			logger.Error("database restore failed: " + err.Error())
		} else if restored {
			logger.Info("database restore completed")
		}
	}
}

// OpenDatabase 打开数据库（WAL 损坏时恢复）并设置时区，再初始化结构、执行迁移与索引。
// 初始化或迁移失败时也返回已打开的数据库，由调用方在退出时关闭。
func OpenDatabase(ctx context.Context, path, timeZone string, logger *applog.FileLogger) (*sql.DB, error) {
	db, err := dbutils.OpenDuckDBWithWALRecovery(ctx, path, logger)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	if _, err = db.Exec("SET GLOBAL checkpoint_threshold = '4 MiB'"); err != nil {
		logger.Warning("Failed to set DuckDB automatic checkpoint threshold; using the default: " + err.Error())
	}
	if timeZone == "" {
		timeZone = "UTC"
	}
	if _, err = db.Exec(fmt.Sprintf("SET TimeZone = '%s'", timeZone)); err != nil {
		logger.Warning("Failed to set timezone: " + err.Error())
	}

	if err := migrations.InitSchema(db); err != nil {
		return db, fmt.Errorf("初始化数据库结构失败: %w", err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		return db, fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := migrations.InitIndexes(db); err != nil {
		return db, fmt.Errorf("初始化数据库索引失败: %w", err)
	}
	if err := dbutils.CheckpointDuckDB(ctx, db); err != nil {
		logger.Warning("Database checkpoint after schema initialization failed; committed changes remain in WAL: " + err.Error())
	}
	return db, nil
}
