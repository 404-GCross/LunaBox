package service

import (
	"context"
	"testing"

	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/service/gamehelper/idmapper"
)

func TestPreviewAndEnrichLegacyGameMetadataSourceIDs(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()
	gameService := NewGameService()
	gameService.Init(ctx, db, &appconf.AppConfig{})
	gameService.SetGameIDMapper(idmapper.New([]idmapper.IDs{
		{VNDBID: 10, BangumiID: 20, SteamID: 30},
		{VNDBID: 50, BangumiID: 40, SteamID: 60},
	}))

	for _, game := range []struct {
		id         string
		sourceType enums.SourceType
		sourceID   string
	}{
		{id: "from-vndb", sourceType: enums.VNDB, sourceID: "v10"},
		{id: "from-bangumi", sourceType: enums.Bangumi, sourceID: "40"},
		{id: "unmatched", sourceType: enums.Steam, sourceID: "70"},
		{id: "other-default", sourceType: enums.Ymgal, sourceID: "ga80"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO games (id, name, source_type, source_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, game.id, game.id, string(game.sourceType), game.sourceID); err != nil {
			t.Fatalf("insert game %s: %v", game.id, err)
		}
	}

	for _, source := range []struct {
		gameID   string
		source   enums.SourceType
		sourceID string
	}{
		{gameID: "from-vndb", source: enums.VNDB, sourceID: "v10"},
		{gameID: "from-bangumi", source: enums.Bangumi, sourceID: "40"},
		{gameID: "from-bangumi", source: enums.VNDB, sourceID: "v999"},
		{gameID: "unmatched", source: enums.Steam, sourceID: "70"},
		{gameID: "other-default", source: enums.VNDB, sourceID: "v10"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO game_metadata_sources (game_id, source_type, source_id)
			VALUES (?, ?, ?)
		`, source.gameID, string(source.source), source.sourceID); err != nil {
			t.Fatalf("insert metadata source for %s: %v", source.gameID, err)
		}
	}

	preview, err := gameService.PreviewLegacyGameMetadataSourceIDs()
	if err != nil {
		t.Fatalf("preview game ID enrichment: %v", err)
	}
	if preview.ScannedGames != 3 || preview.EnrichableGames != 2 ||
		preview.UnchangedGames != 1 || preview.AddedSources != 3 || len(preview.Items) != 3 {
		t.Fatalf("unexpected enrichment preview: %+v", preview)
	}
	vndbPreview := findEnrichmentPreviewItem(t, preview.Items, "from-vndb")
	if !vndbPreview.CanEnrich || len(vndbPreview.AddedSources) != 2 {
		t.Fatalf("unexpected VNDB enrichment preview: %+v", vndbPreview)
	}
	unmatchedPreview := findEnrichmentPreviewItem(t, preview.Items, "unmatched")
	if unmatchedPreview.CanEnrich || unmatchedPreview.Reason != "no_mapping" {
		t.Fatalf("unexpected unmatched enrichment preview: %+v", unmatchedPreview)
	}

	result, err := gameService.EnrichLegacyGameMetadataSourceIDs()
	if err != nil {
		t.Fatalf("enrich game IDs: %v", err)
	}
	if result.UpdatedGames != 2 || result.AddedSources != 3 || result.UnmatchedGames != 1 {
		t.Fatalf("unexpected enrichment result: %+v", result)
	}
}

func findEnrichmentPreviewItem(
	t *testing.T,
	items []GameIDEnrichmentPreviewItem,
	gameID string,
) GameIDEnrichmentPreviewItem {
	t.Helper()
	for _, item := range items {
		if item.GameID == gameID {
			return item
		}
	}
	t.Fatalf("missing enrichment preview for %s", gameID)
	return GameIDEnrichmentPreviewItem{}
}
