package importer

import (
	"context"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"strings"
	"time"
)

// ImportResult 导入结果
type ImportResult struct {
	Success          int      `json:"success"`           // 成功导入数量
	Skipped          int      `json:"skipped"`           // 跳过数量（已存在）
	Failed           int      `json:"failed"`            // 失败数量
	FailedNames      []string `json:"failed_names"`      // 失败的游戏名称
	SkippedNames     []string `json:"skipped_names"`     // 跳过的游戏名称
	SessionsImported int      `json:"sessions_imported"` // 导入的游玩记录数量
}

// PreviewGame 预览导入的游戏信息
type PreviewGame struct {
	Name       string    `json:"name"`
	Developer  string    `json:"developer"`
	SourceType string    `json:"source_type"`
	Exists     bool      `json:"exists"`
	AddTime    time.Time `json:"add_time"`
	HasPath    bool      `json:"has_path"`
}

type Dependencies struct {
	Ctx         context.Context
	ListGames   func() ([]models.Game, error)
	AddGame     func(vo.GameMetadataFromWebVO) error
	AddSessions func([]models.PlaySession) error
}

func newImportResult() ImportResult {
	return ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}
}

func (d Dependencies) existingGames(logPrefix string) ([]models.Game, map[string]string, map[string]string, error) {
	if d.ListGames == nil {
		return nil, nil, nil, fmt.Errorf("缺少游戏列表依赖")
	}

	existingGames, err := d.ListGames()
	if err != nil {
		applog.LogErrorf(d.Ctx, "%s: failed to get existing games: %v", logPrefix, err)
		return nil, nil, nil, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}

	existingNames := make(map[string]string)
	existingPaths := make(map[string]string)
	for _, g := range existingGames {
		if g.Name != "" {
			existingNames[strings.ToLower(g.Name)] = g.ID
		}
		if g.Path != "" {
			existingPaths[g.Path] = g.Name
		}
	}

	return existingGames, existingNames, existingPaths, nil
}

func (d Dependencies) existingNameSet(logPrefix string) (map[string]bool, error) {
	if d.ListGames == nil {
		return nil, fmt.Errorf("缺少游戏列表依赖")
	}

	existingGames, err := d.ListGames()
	if err != nil {
		applog.LogErrorf(d.Ctx, "%s: failed to get existing games: %v", logPrefix, err)
		return nil, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}

	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}
	return existingNames, nil
}

func skipExistingGame(
	ctx context.Context,
	logPrefix string,
	result *ImportResult,
	existingGames []models.Game,
	existingNames map[string]string,
	existingPaths map[string]string,
	gameName string,
	exePath string,
) bool {
	if exePath != "" {
		if existingName, exists := existingPaths[exePath]; exists {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (路径已存在: "+existingName+")")
			return true
		}
	}

	if existingID, exists := existingNames[strings.ToLower(gameName)]; exists {
		for _, g := range existingGames {
			if g.ID == existingID && g.Path == exePath {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
				return true
			}
		}
		applog.LogInfof(ctx, "%s: importing duplicate name %s with different path: %s", logPrefix, gameName, exePath)
	}

	return false
}

func addImportedGame(deps Dependencies, source vo.GameMetadataFromWebVO) error {
	if deps.AddGame == nil {
		return fmt.Errorf("缺少游戏导入依赖")
	}
	return deps.AddGame(source)
}

func addPlaySessions(deps Dependencies, logPrefix string, result *ImportResult, gameName string, sessions []models.PlaySession) {
	if len(sessions) == 0 || deps.AddSessions == nil {
		return
	}

	if err := deps.AddSessions(sessions); err != nil {
		applog.LogWarningf(deps.Ctx, "%s: failed to import play sessions for game %s: %v", logPrefix, gameName, err)
		return
	}

	applog.LogInfof(deps.Ctx, "%s: imported %d play sessions for game %s", logPrefix, len(sessions), gameName)
	result.SessionsImported += len(sessions)
}

func updateExistingIndexes(existingNames map[string]string, existingPaths map[string]string, game models.Game, gameName string, exePath string) {
	existingNames[strings.ToLower(gameName)] = game.ID
	if exePath != "" {
		existingPaths[exePath] = gameName
	}
}
