package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"lunabox/internal/common/vo"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	libraryRecordGame         = "game"
	libraryRecordDownloadTask = "download_task"
)

type GameLibraryPathChangeItem struct {
	RecordType   string `json:"record_type"`
	RecordID     string `json:"record_id"`
	RecordName   string `json:"record_name"`
	Field        string `json:"field"`
	OldPath      string `json:"old_path"`
	NewPath      string `json:"new_path"`
	TargetExists bool   `json:"target_exists"`
	SteamManaged bool   `json:"steam_managed"`
}

type GameLibraryPathChangePreview struct {
	OldConfiguredPath         string                      `json:"old_configured_path"`
	NewConfiguredPath         string                      `json:"new_configured_path"`
	OldLibraryPath            string                      `json:"old_library_path"`
	NewLibraryPath            string                      `json:"new_library_path"`
	Changes                   []GameLibraryPathChangeItem `json:"changes"`
	AffectedGameCount         int                         `json:"affected_game_count"`
	AffectedDownloadTaskCount int                         `json:"affected_download_task_count"`
	MissingTargetCount        int                         `json:"missing_target_count"`
	SteamGameCount            int                         `json:"steam_game_count"`
	BlockingDownloadTaskCount int                         `json:"blocking_download_task_count"`
}

type GameLibraryPathChangeResult struct {
	NewConfiguredPath        string `json:"new_configured_path"`
	UpdatedGameCount         int    `json:"updated_game_count"`
	UpdatedDownloadTaskCount int    `json:"updated_download_task_count"`
}

type libraryPathQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type libraryGameUpdate struct {
	id            string
	path          string
	gameDirectory string
	savePath      string
}

type libraryDownloadTaskUpdate struct {
	id       string
	filePath string
}

func (s *ConfigService) PreviewGameLibraryPathChange(newPath string) (GameLibraryPathChangePreview, error) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	if s.config == nil {
		return GameLibraryPathChangePreview{}, fmt.Errorf("应用配置尚未初始化")
	}
	preview, _, _, err := s.collectGameLibraryPathChanges(s.ctx, s.db, s.config.GameLibraryPath, newPath)
	return preview, err
}

func (s *ConfigService) ApplyGameLibraryPathChange(newPath string, syncPaths bool) (GameLibraryPathChangeResult, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	if s.config == nil {
		return GameLibraryPathChangeResult{}, fmt.Errorf("应用配置尚未初始化")
	}
	if s.db == nil {
		return GameLibraryPathChangeResult{}, fmt.Errorf("数据库尚未初始化")
	}
	configuredPath, newLibraryPath, err := normalizeLibraryPath(newPath)
	if err != nil {
		return GameLibraryPathChangeResult{}, err
	}
	_, oldLibraryPath, err := normalizeLibraryPath(s.config.GameLibraryPath)
	if err != nil {
		return GameLibraryPathChangeResult{}, err
	}

	result := GameLibraryPathChangeResult{NewConfiguredPath: configuredPath}
	previousConfig := *s.config
	newConfig := previousConfig
	newConfig.GameLibraryPath = configuredPath

	if !syncPaths || sameLibraryPath(oldLibraryPath, newLibraryPath) {
		if err := s.updateAppConfigLocked(newConfig); err != nil {
			return GameLibraryPathChangeResult{}, err
		}
		return result, nil
	}
	if s.downloadService != nil {
		if count := s.downloadService.libraryChangeBlockingTaskCount(); count > 0 {
			return GameLibraryPathChangeResult{}, fmt.Errorf("仍有 %d 个下载任务尚未结束、处于暂停状态或等待重试，请先处理这些任务", count)
		}
	}

	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return GameLibraryPathChangeResult{}, fmt.Errorf("开始游戏库地址更新事务失败: %w", err)
	}
	defer tx.Rollback()

	_, gameUpdates, taskUpdates, err := s.collectGameLibraryPathChanges(s.ctx, tx, previousConfig.GameLibraryPath, configuredPath)
	if err != nil {
		return GameLibraryPathChangeResult{}, err
	}

	for _, update := range gameUpdates {
		if _, err := tx.ExecContext(s.ctx, `
			UPDATE games
			SET path = ?, game_directory = ?, save_path = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, update.path, update.gameDirectory, update.savePath, update.id); err != nil {
			return GameLibraryPathChangeResult{}, fmt.Errorf("更新游戏 %s 的本地地址失败: %w", update.id, err)
		}
	}

	for _, update := range taskUpdates {
		if _, err := tx.ExecContext(s.ctx, `
			UPDATE download_tasks
			SET file_path = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, update.filePath, update.id); err != nil {
			return GameLibraryPathChangeResult{}, fmt.Errorf("更新下载任务 %s 的本地地址失败: %w", update.id, err)
		}
	}

	if err := s.updateAppConfigLocked(newConfig); err != nil {
		return GameLibraryPathChangeResult{}, err
	}
	if err := tx.Commit(); err != nil {
		if restoreErr := s.updateAppConfigLocked(previousConfig); restoreErr != nil {
			return GameLibraryPathChangeResult{}, fmt.Errorf("提交游戏库地址更新失败: %w；恢复原配置失败: %v", err, restoreErr)
		}
		return GameLibraryPathChangeResult{}, fmt.Errorf("提交游戏库地址更新失败: %w", err)
	}

	result.UpdatedGameCount = len(gameUpdates)
	result.UpdatedDownloadTaskCount = len(taskUpdates)
	if s.downloadService != nil {
		s.downloadService.rebaseCompletedTaskPaths(oldLibraryPath, newLibraryPath)
	}
	return result, nil
}

func (s *ConfigService) collectGameLibraryPathChanges(
	ctx context.Context,
	queryer libraryPathQueryer,
	oldConfiguredPath string,
	newConfiguredPath string,
) (GameLibraryPathChangePreview, []libraryGameUpdate, []libraryDownloadTaskUpdate, error) {
	oldConfiguredPath = strings.TrimSpace(oldConfiguredPath)
	configuredPath, newLibraryPath, err := normalizeLibraryPath(newConfiguredPath)
	if err != nil {
		return GameLibraryPathChangePreview{}, nil, nil, err
	}
	_, oldLibraryPath, err := normalizeLibraryPath(oldConfiguredPath)
	if err != nil {
		return GameLibraryPathChangePreview{}, nil, nil, err
	}

	preview := GameLibraryPathChangePreview{
		OldConfiguredPath: oldConfiguredPath,
		NewConfiguredPath: configuredPath,
		OldLibraryPath:    oldLibraryPath,
		NewLibraryPath:    newLibraryPath,
		Changes:           make([]GameLibraryPathChangeItem, 0),
	}
	if s.downloadService != nil {
		preview.BlockingDownloadTaskCount = s.downloadService.libraryChangeBlockingTaskCount()
	}
	if sameLibraryPath(oldLibraryPath, newLibraryPath) {
		return preview, nil, nil, nil
	}

	gameUpdates, err := collectGamePathChanges(ctx, queryer, oldLibraryPath, newLibraryPath, &preview)
	if err != nil {
		return GameLibraryPathChangePreview{}, nil, nil, err
	}
	taskUpdates, err := collectDownloadTaskPathChanges(ctx, queryer, oldLibraryPath, newLibraryPath, &preview)
	if err != nil {
		return GameLibraryPathChangePreview{}, nil, nil, err
	}
	return preview, gameUpdates, taskUpdates, nil
}

func collectGamePathChanges(
	ctx context.Context,
	queryer libraryPathQueryer,
	oldLibraryPath string,
	newLibraryPath string,
	preview *GameLibraryPathChangePreview,
) ([]libraryGameUpdate, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, COALESCE(name, ''), COALESCE(path, ''), COALESCE(game_directory, ''),
		       COALESCE(save_path, ''), COALESCE(launch_mode, 'normal'), COALESCE(steam_launch_kind, '')
		FROM games
		ORDER BY name, id
	`)
	if err != nil {
		return nil, fmt.Errorf("查询游戏本地地址失败: %w", err)
	}
	defer rows.Close()

	updates := make([]libraryGameUpdate, 0)
	for rows.Next() {
		var id, name, path, gameDirectory, savePath, launchMode, steamLaunchKind string
		if err := rows.Scan(&id, &name, &path, &gameDirectory, &savePath, &launchMode, &steamLaunchKind); err != nil {
			return nil, fmt.Errorf("读取游戏本地地址失败: %w", err)
		}

		update := libraryGameUpdate{id: id, path: path, gameDirectory: gameDirectory, savePath: savePath}
		steamManaged := strings.EqualFold(strings.TrimSpace(launchMode), "steam") || strings.TrimSpace(steamLaunchKind) != ""
		changed := false
		for _, field := range []struct {
			name    string
			current string
			set     func(string)
		}{
			{name: "path", current: path, set: func(value string) { update.path = value }},
			{name: "game_directory", current: gameDirectory, set: func(value string) { update.gameDirectory = value }},
			{name: "save_path", current: savePath, set: func(value string) { update.savePath = value }},
		} {
			newValue, matches := rebaseLibraryPath(field.current, oldLibraryPath, newLibraryPath)
			if !matches {
				continue
			}
			field.set(newValue)
			preview.addChange(newLibraryPathChangeItem(
				libraryRecordGame, id, name, field.name, field.current, newValue, steamManaged,
			))
			changed = true
		}
		if changed {
			updates = append(updates, update)
			preview.AffectedGameCount++
			if steamManaged {
				preview.SteamGameCount++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历游戏本地地址失败: %w", err)
	}
	return updates, nil
}

func collectDownloadTaskPathChanges(
	ctx context.Context,
	queryer libraryPathQueryer,
	oldLibraryPath string,
	newLibraryPath string,
	preview *GameLibraryPathChangePreview,
) ([]libraryDownloadTaskUpdate, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, COALESCE(request_json, ''), COALESCE(file_path, '')
		FROM download_tasks
		WHERE TRIM(COALESCE(file_path, '')) <> ''
		ORDER BY created_at DESC, id
	`)
	if err != nil {
		return nil, fmt.Errorf("查询下载任务本地地址失败: %w", err)
	}
	defer rows.Close()

	updates := make([]libraryDownloadTaskUpdate, 0)
	for rows.Next() {
		var id, requestJSON, filePath string
		if err := rows.Scan(&id, &requestJSON, &filePath); err != nil {
			return nil, fmt.Errorf("读取下载任务本地地址失败: %w", err)
		}
		newValue, matches := rebaseLibraryPath(filePath, oldLibraryPath, newLibraryPath)
		if !matches {
			continue
		}
		name := id
		var request vo.InstallRequest
		if json.Unmarshal([]byte(requestJSON), &request) == nil && strings.TrimSpace(request.Title) != "" {
			name = strings.TrimSpace(request.Title)
		}
		preview.addChange(newLibraryPathChangeItem(
			libraryRecordDownloadTask, id, name, "file_path", filePath, newValue, false,
		))
		preview.AffectedDownloadTaskCount++
		updates = append(updates, libraryDownloadTaskUpdate{id: id, filePath: newValue})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历下载任务本地地址失败: %w", err)
	}
	return updates, nil
}

func newLibraryPathChangeItem(
	recordType string,
	recordID string,
	recordName string,
	field string,
	oldPath string,
	newPath string,
	steamManaged bool,
) GameLibraryPathChangeItem {
	_, err := os.Stat(newPath)
	targetExists := err == nil
	return GameLibraryPathChangeItem{
		RecordType:   recordType,
		RecordID:     recordID,
		RecordName:   recordName,
		Field:        field,
		OldPath:      oldPath,
		NewPath:      newPath,
		TargetExists: targetExists,
		SteamManaged: steamManaged,
	}
}

func (preview *GameLibraryPathChangePreview) addChange(item GameLibraryPathChangeItem) {
	preview.Changes = append(preview.Changes, item)
	if !item.TargetExists {
		preview.MissingTargetCount++
	}
}

func normalizeLibraryPath(configuredPath string) (string, string, error) {
	configuredPath = strings.TrimSpace(configuredPath)
	effectivePath := configuredPath
	if effectivePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("获取默认游戏库目录失败: %w", err)
		}
		effectivePath = filepath.Join(homeDir, "Games")
	}
	absPath, err := filepath.Abs(filepath.Clean(effectivePath))
	if err != nil {
		return "", "", fmt.Errorf("解析游戏库目录失败: %w", err)
	}
	if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
		return "", "", fmt.Errorf("游戏库地址指向文件: %s", absPath)
	} else if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("检查游戏库目录失败: %w", err)
	}
	if configuredPath != "" {
		configuredPath = absPath
	}
	return configuredPath, absPath, nil
}

func rebaseLibraryPath(currentPath string, oldRoot string, newRoot string) (string, bool) {
	currentPath = strings.TrimSpace(currentPath)
	if currentPath == "" || !filepath.IsAbs(currentPath) {
		return currentPath, false
	}
	absCurrent, err := filepath.Abs(filepath.Clean(currentPath))
	if err != nil {
		return currentPath, false
	}
	relative, err := filepath.Rel(oldRoot, absCurrent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return currentPath, false
	}
	return filepath.Clean(filepath.Join(newRoot, relative)), true
}

func sameLibraryPath(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
