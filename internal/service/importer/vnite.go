package importer

import (
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/vnite"
	"lunabox/internal/utils/imageutils"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

type VniteImporter struct {
	deps Dependencies
}

func NewVniteImporter(deps Dependencies) *VniteImporter {
	return &VniteImporter{deps: deps}
}

func (v *VniteImporter) Preview(vniteDir string) ([]PreviewGame, error) {
	data, err := vnite.LoadExportData(vniteDir)
	if err != nil {
		applog.LogErrorf(v.deps.Ctx, "PreviewVniteImport: failed to load vnite data: %v", err)
		return nil, fmt.Errorf("读取 Vnite 导出目录失败: %w", err)
	}

	existingNames, err := v.deps.existingNameSet("PreviewVniteImport")
	if err != nil {
		return nil, err
	}

	allIDs := collectVniteIDs(data)
	previews := make([]PreviewGame, 0, len(allIDs))
	for id := range allIDs {
		gameDoc, hasGame := data.GameDocs[id]
		localDoc, hasLocal := data.GameLocalDocs[id]
		if !hasGame {
			continue
		}

		name := pickVniteName(gameDoc)
		if name == "" {
			continue
		}

		previews = append(previews, PreviewGame{
			Name:       name,
			Developer:  pickVniteDeveloper(gameDoc),
			SourceType: string(mapVniteSourceType(gameDoc)),
			Exists:     existingNames[strings.ToLower(name)],
			AddTime:    parseVniteTimeOrNow(gameDoc.Record.AddDate),
			HasPath:    hasLocal && pickVniteGamePath(localDoc) != "",
		})
	}

	return previews, nil
}

func (v *VniteImporter) Import(vniteDir string, skipNoPath bool) (ImportResult, error) {
	result := newImportResult()

	data, err := vnite.LoadExportData(vniteDir)
	if err != nil {
		applog.LogErrorf(v.deps.Ctx, "ImportFromVnite: failed to load vnite data: %v", err)
		return result, fmt.Errorf("读取 Vnite 导出目录失败: %w", err)
	}

	existingGames, existingNames, existingPaths, err := v.deps.existingGames("ImportFromVnite")
	if err != nil {
		return result, err
	}

	allIDs := collectVniteIDs(data)
	for id := range allIDs {
		gameDoc, hasGame := data.GameDocs[id]
		localDoc := data.GameLocalDocs[id]
		if !hasGame {
			continue
		}

		gameName := pickVniteName(gameDoc)
		if gameName == "" {
			continue
		}

		exePath := pickVniteGamePath(localDoc)
		hasPath := exePath != ""
		if skipExistingGame(v.deps.Ctx, "ImportFromVnite", &result, existingGames, existingNames, existingPaths, gameName, exePath) {
			continue
		}

		if skipNoPath && !hasPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}

		game, sessions := v.convertToGame(gameDoc, localDoc)
		v.applyCover(&game, vniteDir, gameDoc)
		if err := addImportedGame(v.deps, vo.GameMetadataFromWebVO{
			Source: game.SourceType,
			Game:   game,
		}); err != nil {
			applog.LogErrorf(v.deps.Ctx, "ImportFromVnite: failed to add game %s: %v", gameName, err)
			result.Failed++
			result.FailedNames = append(result.FailedNames, gameName)
			continue
		}

		addPlaySessions(v.deps, "ImportFromVnite", &result, gameName, sessions)
		updateExistingIndexes(existingNames, existingPaths, game, gameName, exePath)
		result.Success++
	}

	return result, nil
}

func collectVniteIDs(data *vnite.ExportData) map[string]bool {
	allIDs := make(map[string]bool)
	for id := range data.GameDocs {
		allIDs[id] = true
	}
	for id := range data.GameLocalDocs {
		allIDs[id] = true
	}
	return allIDs
}

func (v *VniteImporter) convertToGame(gameDoc vnite.GameDoc, localDoc vnite.GameLocalDoc) (models.Game, []models.PlaySession) {
	gameID := uuid.New().String()
	game := models.Game{
		ID:         gameID,
		Name:       pickVniteName(gameDoc),
		Company:    pickVniteDeveloper(gameDoc),
		Summary:    gameDoc.Metadata.Description,
		Path:       pickVniteGamePath(localDoc),
		SavePath:   pickVniteSavePath(localDoc),
		SourceType: mapVniteSourceType(gameDoc),
		SourceID:   pickVniteSourceID(gameDoc),
		CreatedAt:  parseVniteTimeOrNow(gameDoc.Record.AddDate),
		CachedAt:   time.Now(),
		UseMagpie:  localDoc.Launcher.UseMagpie,
	}

	sessions := parseVniteTimers(gameID, gameDoc.Record.Timers)
	return game, sessions
}

func (v *VniteImporter) applyCover(game *models.Game, vniteDir string, gameDoc vnite.GameDoc) {
	coverBytes, ext, err := vnite.LoadGameCoverBytes(vniteDir, gameDoc)
	if err != nil {
		applog.LogWarningf(v.deps.Ctx, "ImportFromVnite: failed to load cover bytes for game %s: %v", game.Name, err)
		return
	}
	if len(coverBytes) == 0 {
		return
	}

	if ext == "" {
		ext = ".jpg"
	}

	tempFile, err := os.CreateTemp("", "vnite_cover_*"+ext)
	if err != nil {
		applog.LogWarningf(v.deps.Ctx, "ImportFromVnite: failed to create temp cover file for game %s: %v", game.Name, err)
		return
	}
	tempFilePath := tempFile.Name()
	if _, err := tempFile.Write(coverBytes); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		applog.LogWarningf(v.deps.Ctx, "ImportFromVnite: failed to write temp cover for game %s: %v", game.Name, err)
		return
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempFilePath)
		applog.LogWarningf(v.deps.Ctx, "ImportFromVnite: failed to close temp cover for game %s: %v", game.Name, err)
		return
	}
	defer os.Remove(tempFilePath)

	savedPath, err := imageutils.SaveCoverImage(tempFilePath, game.ID)
	if err != nil {
		applog.LogWarningf(v.deps.Ctx, "ImportFromVnite: failed to save cover image for game %s: %v", game.Name, err)
		return
	}

	game.CoverURL = savedPath
}

func pickVniteName(gameDoc vnite.GameDoc) string {
	if gameDoc.Metadata.Name != "" {
		return gameDoc.Metadata.Name
	}
	return gameDoc.Metadata.OriginalName
}

func pickVniteDeveloper(gameDoc vnite.GameDoc) string {
	if len(gameDoc.Metadata.Developers) > 0 {
		return gameDoc.Metadata.Developers[0]
	}
	if len(gameDoc.Metadata.Publishers) > 0 {
		return gameDoc.Metadata.Publishers[0]
	}
	return ""
}

func pickVniteGamePath(localDoc vnite.GameLocalDoc) string {
	if localDoc.Path.GamePath != "" {
		return localDoc.Path.GamePath
	}
	return localDoc.Launcher.FileConfig.Path
}

func pickVniteSavePath(localDoc vnite.GameLocalDoc) string {
	if len(localDoc.Path.SavePaths) > 0 {
		return localDoc.Path.SavePaths[0]
	}
	return ""
}

func pickVniteSourceID(gameDoc vnite.GameDoc) string {
	if gameDoc.Metadata.VNDBID != "" {
		return gameDoc.Metadata.VNDBID
	}
	if gameDoc.Metadata.YmgalID != "" {
		return gameDoc.Metadata.YmgalID
	}
	if gameDoc.Metadata.BangumiID != "" {
		return gameDoc.Metadata.BangumiID
	}
	if gameDoc.Metadata.SteamID != "" {
		return gameDoc.Metadata.SteamID
	}
	return ""
}

func mapVniteSourceType(gameDoc vnite.GameDoc) enums.SourceType {
	if gameDoc.Metadata.VNDBID != "" {
		return enums.VNDB
	}
	if gameDoc.Metadata.YmgalID != "" {
		return enums.Ymgal
	}
	if gameDoc.Metadata.BangumiID != "" {
		return enums.Bangumi
	}
	if gameDoc.Metadata.SteamID != "" {
		return enums.Steam
	}
	return enums.Local
}

func parseVniteTimeOrNow(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}

	return time.Now()
}

func parseVniteTimers(gameID string, timers []vnite.GameTimer) []models.PlaySession {
	sessions := make([]models.PlaySession, 0, len(timers))
	for _, timer := range timers {
		if timer.Start == "" || timer.End == "" {
			continue
		}

		startTime := parseVniteTimeOrNow(timer.Start)
		endTime := parseVniteTimeOrNow(timer.End)
		duration := int(endTime.Sub(startTime).Seconds())
		if duration <= 0 {
			continue
		}

		sessions = append(sessions, models.PlaySession{
			ID:        uuid.New().String(),
			GameID:    gameID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
		})
	}

	return sessions
}
