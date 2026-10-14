package remotestatus

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"time"
)

const remoteStatusSyncInterval = 250 * time.Millisecond

type Options struct {
	Context context.Context
	DB      *sql.DB
	Source  enums.SourceType
	Prepare func(context.Context) error
	Push    func(context.Context, models.Game) error
	Emit    func(vo.RemoteStatusSyncProgress)
}

func SyncAll(options Options) (vo.RemoteStatusSyncProgress, error) {
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}

	games, err := LoadGames(ctx, options.DB, options.Source)
	progress := vo.RemoteStatusSyncProgress{
		Provider:        string(options.Source),
		Status:          "started",
		Total:           len(games),
		FailedGameNames: make([]string, 0),
	}
	fail := func(err error) (vo.RemoteStatusSyncProgress, error) {
		progress.Status = "failed"
		progress.LastError = err.Error()
		emit(options.Emit, progress)
		return progress, err
	}
	if err != nil {
		return fail(err)
	}
	emit(options.Emit, progress)

	if len(games) == 0 {
		progress.Status = "done"
		emit(options.Emit, progress)
		return progress, nil
	}
	if options.Prepare != nil {
		if err := options.Prepare(ctx); err != nil {
			return fail(err)
		}
	}
	if options.Push == nil {
		return fail(fmt.Errorf("缺少 %s 状态同步实现", options.Source))
	}

	for index, game := range games {
		progress.Status = "running"
		progress.GameName = game.Name
		emit(options.Emit, progress)

		if err := options.Push(ctx, game); err != nil {
			progress.FailedGames++
			progress.FailedGameNames = append(progress.FailedGameNames, game.Name)
			progress.LastError = err.Error()
			applog.LogWarningf(ctx, "failed to sync %s status for game %s (%s): %v", options.Source, game.Name, game.ID, err)
		} else {
			progress.SucceededGames++
		}
		progress.Current = index + 1
		emit(options.Emit, progress)

		if index+1 < len(games) {
			if err := wait(ctx); err != nil {
				return fail(err)
			}
		}
	}

	progress.Status = "done"
	progress.GameName = ""
	emit(options.Emit, progress)
	return progress, nil
}

func emit(handler func(vo.RemoteStatusSyncProgress), progress vo.RemoteStatusSyncProgress) {
	if handler != nil {
		handler(progress)
	}
}

func LoadGames(
	ctx context.Context,
	db *sql.DB,
	source enums.SourceType,
) ([]models.Game, error) {
	if db == nil {
		return nil, fmt.Errorf("游戏数据库未初始化")
	}

	rows, err := db.QueryContext(ctx, `
		SELECT g.id, g.name, g.status, s.source_id
		FROM games g
		JOIN game_metadata_sources s ON s.game_id = g.id
		WHERE s.source_type = ?
		  AND TRIM(COALESCE(s.source_id, '')) <> ''
		UNION ALL
		SELECT g.id, g.name, g.status, g.source_id
		FROM games g
		WHERE g.source_type = ?
		  AND TRIM(COALESCE(g.source_id, '')) <> ''
		  AND NOT EXISTS (
			SELECT 1 FROM game_metadata_sources s WHERE s.game_id = g.id
		  )
		ORDER BY name, id
	`, string(source), string(source))
	if err != nil {
		return nil, fmt.Errorf("读取 %s 游戏条目失败: %w", source, err)
	}
	defer rows.Close()

	games := make([]models.Game, 0)
	for rows.Next() {
		var game models.Game
		var status string
		if err := rows.Scan(&game.ID, &game.Name, &status, &game.SourceID); err != nil {
			return nil, fmt.Errorf("读取 %s 游戏状态失败: %w", source, err)
		}
		game.Status = enums.GameStatus(status)
		game.SourceType = source
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 %s 游戏条目失败: %w", source, err)
	}
	return games, nil
}

func wait(ctx context.Context) error {
	timer := time.NewTimer(remoteStatusSyncInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
