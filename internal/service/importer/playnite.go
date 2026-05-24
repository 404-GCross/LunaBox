package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/playnite"
	"lunabox/internal/utils/imageutils"
	"os"
	"strings"
	"time"
)

type PlayniteImporter struct {
	deps Dependencies
}

func NewPlayniteImporter(deps Dependencies) *PlayniteImporter {
	return &PlayniteImporter{deps: deps}
}

func (p *PlayniteImporter) Import(jsonPath string, skipNoPath bool) (ImportResult, error) {
	result := newImportResult()

	playniteGames, err := p.readGames(jsonPath)
	if err != nil {
		return result, err
	}

	existingGames, existingNames, existingPaths, err := p.deps.existingGames("ImportFromPlaynite")
	if err != nil {
		return result, err
	}

	for _, pg := range playniteGames {
		if skipExistingGame(p.deps.Ctx, "ImportFromPlaynite", &result, existingGames, existingNames, existingPaths, pg.Name, pg.Path) {
			continue
		}

		if skipNoPath && pg.Path == "" {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, pg.Name+" (无路径)")
			continue
		}

		game := p.convertToGame(pg)
		if err := addImportedGame(p.deps, vo.GameMetadataFromWebVO{
			Source: game.SourceType,
			Game:   game,
		}); err != nil {
			applog.LogErrorf(p.deps.Ctx, "ImportFromPlaynite: failed to add game %s: %v", pg.Name, err)
			result.Failed++
			result.FailedNames = append(result.FailedNames, pg.Name)
			continue
		}

		updateExistingIndexes(existingNames, existingPaths, game, pg.Name, pg.Path)
		result.Success++
	}

	return result, nil
}

func (p *PlayniteImporter) Preview(jsonPath string) ([]PreviewGame, error) {
	playniteGames, err := p.readGames(jsonPath)
	if err != nil {
		return nil, err
	}

	existingNames, err := p.deps.existingNameSet("PreviewPlayniteImport")
	if err != nil {
		return nil, err
	}

	previews := make([]PreviewGame, 0, len(playniteGames))
	for _, pg := range playniteGames {
		previews = append(previews, PreviewGame{
			Name:       pg.Name,
			Developer:  pg.Company,
			SourceType: pg.SourceType,
			Exists:     existingNames[strings.ToLower(pg.Name)],
			AddTime:    pg.CreatedAt,
			HasPath:    pg.Path != "",
		})
	}

	return previews, nil
}

func (p *PlayniteImporter) readGames(jsonPath string) ([]playnite.PlayniteGame, error) {
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to read JSON file: %v", err)
		return nil, fmt.Errorf("无法读取 JSON 文件: %w", err)
	}

	utf8BOM := []byte{0xEF, 0xBB, 0xBF}
	jsonData = bytes.TrimPrefix(jsonData, utf8BOM)

	var playniteGames []playnite.PlayniteGame
	if err := json.Unmarshal(jsonData, &playniteGames); err != nil {
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to unmarshal JSON: %v", err)
		return nil, fmt.Errorf("解析 JSON 文件失败: %w", err)
	}
	return playniteGames, nil
}

func (p *PlayniteImporter) convertToGame(pg playnite.PlayniteGame) models.Game {
	game := models.Game{
		ID:          pg.ID,
		Name:        pg.Name,
		Company:     pg.Company,
		Summary:     pg.Summary,
		Rating:      pg.Rating,
		ReleaseDate: pg.ReleaseDate,
		Path:        pg.Path,
		SourceType:  stringToSourceType(pg.SourceType),
		SourceID:    pg.SourceID,
		CreatedAt:   pg.CreatedAt,
		CachedAt:    time.Now(),
	}

	if pg.SavePath != nil {
		game.SavePath = *pg.SavePath
	}

	if pg.CoverURL != "" {
		savedPath, err := imageutils.SaveCoverImage(pg.CoverURL, game.ID)
		if err == nil {
			game.CoverURL = savedPath
		} else {
			applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to save cover image for game %s: %v", game.Name, err)
			game.CoverURL = pg.CoverURL
		}
	}

	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now()
	}

	return game
}

func stringToSourceType(sourceType string) enums.SourceType {
	switch strings.ToLower(sourceType) {
	case "bangumi":
		return enums.Bangumi
	case "vndb":
		return enums.VNDB
	case "ymgal":
		return enums.Ymgal
	case "steam":
		return enums.Steam
	default:
		return enums.Local
	}
}
