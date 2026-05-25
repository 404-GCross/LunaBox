package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/migrations"
	"lunabox/internal/models"
	"lunabox/internal/utils/metadata"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

func TestNormalizeImportPathMatchesEquivalentWindowsPaths(t *testing.T) {
	base := filepath.Join("C:\\", "Games", "Example")
	a := filepath.Join(base, "..", "Example", "Game.exe")
	b := strings.ToLower(filepath.Join("C:\\", "Games", "Example", "Game.exe"))

	if normalizeImportPath(a) != normalizeImportPath(b) {
		t.Fatalf("expected equivalent paths to match, got %q and %q", normalizeImportPath(a), normalizeImportPath(b))
	}
}

func TestSplitScanCandidatesKeepsOnlyNewPathsInMainList(t *testing.T) {
	idx := newImportIndex([]importGameRef{
		{ID: "existing-1", Name: "Existing One", Path: `C:\Games\ExistingOne\game.exe`},
		{ID: "existing-2", Name: "Existing Two", Path: `C:\Games\ExistingTwo\game.exe`},
	})

	candidates := []vo.BatchImportCandidate{
		{
			FolderPath:  `C:\Games\ExistingOne`,
			FolderName:  "ExistingOne",
			SelectedExe: `c:\games\existingone\game.exe`,
			SearchName:  "Existing One",
			IsSelected:  true,
		},
		{
			FolderPath:  `C:\Games\NewGame`,
			FolderName:  "NewGame",
			SelectedExe: `C:\Games\NewGame\game.exe`,
			SearchName:  "New Game",
			IsSelected:  true,
		},
		{
			FolderPath:  `C:\Games\ExistingTwo`,
			FolderName:  "ExistingTwo",
			SelectedExe: `C:\Games\ExistingTwo\game.exe`,
			SearchName:  "Existing Two",
			IsSelected:  true,
		},
	}

	result := splitScanCandidates(candidates, idx)
	if len(result.Candidates) != 1 {
		t.Fatalf("expected only one importable candidate, got %d", len(result.Candidates))
	}
	if result.Candidates[0].SearchName != "New Game" {
		t.Fatalf("expected New Game to remain importable, got %s", result.Candidates[0].SearchName)
	}
	if len(result.SkippedCandidates) != 2 || result.Skipped != 2 {
		t.Fatalf("expected two skipped candidates, got skipped=%d details=%d", result.Skipped, len(result.SkippedCandidates))
	}
}

func TestSplitScanCandidatesSkipsWhenAnyExecutablePathExists(t *testing.T) {
	idx := newImportIndex([]importGameRef{
		{ID: "existing", Name: "Existing Game", Path: `C:\Games\MultiExe\existing.exe`},
	})

	result := splitScanCandidates([]vo.BatchImportCandidate{
		{
			FolderPath:  `C:\Games\MultiExe`,
			FolderName:  "MultiExe",
			Executables: []string{`C:\Games\MultiExe\new-default.exe`, `C:\Games\MultiExe\existing.exe`},
			SelectedExe: `C:\Games\MultiExe\new-default.exe`,
			SearchName:  "Existing Game",
			IsSelected:  true,
		},
	}, idx)

	if len(result.Candidates) != 0 {
		t.Fatalf("expected candidate to be skipped when any executable path exists, got %d importable", len(result.Candidates))
	}
	if len(result.SkippedCandidates) != 1 {
		t.Fatalf("expected one skipped candidate, got %d", len(result.SkippedCandidates))
	}
	if result.SkippedCandidates[0].ImportStatus != importStatusExistsPath {
		t.Fatalf("expected exists_path status, got %s", result.SkippedCandidates[0].ImportStatus)
	}
}

func TestSplitScanCandidatesLargeIncrementalScan(t *testing.T) {
	refs := make([]importGameRef, 0, 3000)
	candidates := make([]vo.BatchImportCandidate, 0, 3005)
	for i := 0; i < 3000; i++ {
		name := "Existing Game"
		path := filepath.Join("C:\\Games", "Existing", fmt.Sprintf("%04d", i), "game.exe")
		refs = append(refs, importGameRef{
			ID:   fmt.Sprintf("existing-%04d", i),
			Name: name,
			Path: path,
		})
		candidates = append(candidates, vo.BatchImportCandidate{
			FolderPath:  filepath.Dir(path),
			FolderName:  fmt.Sprintf("%04d", i),
			SelectedExe: path,
			SearchName:  name,
			IsSelected:  true,
		})
	}
	for i := 0; i < 5; i++ {
		path := filepath.Join("C:\\Games", "New", fmt.Sprintf("%04d", i), "game.exe")
		candidates = append(candidates, vo.BatchImportCandidate{
			FolderPath:  filepath.Dir(path),
			FolderName:  fmt.Sprintf("new-%04d", i),
			SelectedExe: path,
			SearchName:  fmt.Sprintf("New Game %d", i),
			IsSelected:  true,
		})
	}

	result := splitScanCandidates(candidates, newImportIndex(refs))
	if len(result.Candidates) != 5 {
		t.Fatalf("expected 5 importable candidates, got %d", len(result.Candidates))
	}
	if len(result.SkippedCandidates) != 3000 {
		t.Fatalf("expected 3000 skipped candidates, got %d", len(result.SkippedCandidates))
	}
}

func TestSplitScanCandidatesDoesNotApplySourceDedupBeforeMetadataPreview(t *testing.T) {
	idx := newImportIndex([]importGameRef{
		{
			ID:         "existing-source",
			Name:       "Existing Source",
			Path:       `C:\Games\ExistingSource\game.exe`,
			SourceType: enums.VNDB,
			SourceID:   "v123",
		},
	})

	result := splitScanCandidates([]vo.BatchImportCandidate{
		{
			FolderPath:  `C:\Games\DifferentPath`,
			FolderName:  "DifferentPath",
			SelectedExe: `C:\Games\DifferentPath\game.exe`,
			SearchName:  "Existing Source",
			IsSelected:  true,
			MatchedGame: &models.Game{
				Name:       "Existing Source",
				SourceType: enums.VNDB,
				SourceID:   "v123",
			},
			MatchSource: enums.VNDB,
		},
	}, idx)

	if len(result.Candidates) != 1 {
		t.Fatalf("expected source duplicate to remain in scan preview, got %d importable", len(result.Candidates))
	}
	if result.Candidates[0].ImportStatus != importStatusPossibleDuplicate {
		t.Fatalf("expected only possible duplicate status before submit, got %s", result.Candidates[0].ImportStatus)
	}
}

func TestBatchImportGamesSkipsSourceDuplicateOnSubmit(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	if err := migrations.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	ctx := context.Background()
	config := &appconf.AppConfig{}
	gameService := NewGameService()
	gameService.Init(ctx, db, config)
	importService := NewImportService()
	importService.Init(ctx, db, config, gameService)

	existing := models.Game{
		ID:         "existing-source",
		Name:       "Existing Source",
		Path:       `C:\Games\ExistingSource\game.exe`,
		SourceType: enums.VNDB,
		SourceID:   "v123",
		CreatedAt:  time.Now(),
		CachedAt:   time.Now(),
	}
	if err := gameService.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{Source: enums.VNDB, Game: existing}); err != nil {
		t.Fatalf("add existing game: %v", err)
	}

	result, err := importService.BatchImportGames([]vo.BatchImportCandidate{
		{
			SelectedExe: `C:\Games\DifferentPath\game.exe`,
			SearchName:  "Existing Source",
			IsSelected:  true,
			MatchedGame: &models.Game{
				Name:       "Existing Source",
				SourceType: enums.VNDB,
				SourceID:   "v123",
			},
			MatchSource: enums.VNDB,
			MatchStatus: "matched",
		},
	})
	if err != nil {
		t.Fatalf("batch import: %v", err)
	}
	if result.Success != 0 || result.Skipped != 1 {
		t.Fatalf("expected source duplicate to skip on submit, got success=%d skipped=%d", result.Success, result.Skipped)
	}
}

func TestCheckImportMetadataDuplicatesReturnsExistingGame(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	if err := migrations.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	ctx := context.Background()
	config := &appconf.AppConfig{}
	gameService := NewGameService()
	gameService.Init(ctx, db, config)
	importService := NewImportService()
	importService.Init(ctx, db, config, gameService)

	existing := models.Game{
		ID:         "existing-source",
		Name:       "Existing Source",
		Path:       `C:\Games\ExistingSource\game.exe`,
		SourceType: enums.VNDB,
		SourceID:   "v123",
		CreatedAt:  time.Now(),
		CachedAt:   time.Now(),
	}
	if err := gameService.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{Source: enums.VNDB, Game: existing}); err != nil {
		t.Fatalf("add existing game: %v", err)
	}

	results, err := importService.CheckImportMetadataDuplicates([]vo.ImportMetadataDuplicateRequest{
		{Source: enums.VNDB, SourceID: "v123"},
		{Source: enums.Bangumi, SourceID: "456"},
	})
	if err != nil {
		t.Fatalf("check metadata duplicates: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected two results, got %d", len(results))
	}
	if !results[0].Exists || results[0].ExistingName != "Existing Source" || results[0].ExistingID == "" {
		t.Fatalf("expected first result to point at existing game, got %+v", results[0])
	}
	if results[1].Exists {
		t.Fatalf("expected second result to be new, got %+v", results[1])
	}
}

func TestCommitImportedItemsStagingAppenderImportsRelatedRows(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	if err := migrations.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	ctx := context.Background()
	config := &appconf.AppConfig{}
	gameService := NewGameService()
	gameService.Init(ctx, db, config)
	importService := NewImportService()
	importService.Init(ctx, db, config, gameService)

	gameID := "import-game"
	sessionID := "import-session"
	tagSource := "vndb"
	tagName := "Visual Novel"
	if _, err := db.ExecContext(ctx, `
		INSERT INTO sync_tombstones (entity_type, entity_id, parent_id, secondary_id, deleted_at)
		VALUES
			(?, ?, '', '', CURRENT_TIMESTAMP),
			(?, ?, '', '', CURRENT_TIMESTAMP),
			(?, ?, '', '', CURRENT_TIMESTAMP)
	`, cloudSyncEntityGame, gameID, cloudSyncEntityGameTag, tagTombstoneID(gameID, tagSource, tagName), cloudSyncEntityPlaySession, sessionID); err != nil {
		t.Fatalf("seed tombstones: %v", err)
	}

	success, sessionsImported, err := importService.commitImportedItems([]importItem{
		{
			Game: models.Game{
				ID:         gameID,
				Name:       "Imported Game",
				Path:       `C:\Games\Imported\game.exe`,
				SourceType: enums.VNDB,
				SourceID:   "v999",
			},
			Tags: []metadata.TagItem{
				{Name: tagName, Source: tagSource, Weight: 0.9},
			},
			Sessions: []models.PlaySession{
				{
					ID:        sessionID,
					StartTime: time.Now().Add(-time.Hour),
					EndTime:   time.Now(),
					Duration:  3600,
				},
			},
			Source: enums.VNDB,
		},
	})
	if err != nil {
		t.Fatalf("commit imported items: %v", err)
	}
	if success != 1 || sessionsImported != 1 {
		t.Fatalf("expected one game and one session, got games=%d sessions=%d", success, sessionsImported)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM games WHERE id = ?`, gameID).Scan(&count); err != nil {
		t.Fatalf("count game: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected imported game, got %d", count)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_tags WHERE game_id = ? AND name = ? AND source = ?`, gameID, tagName, tagSource).Scan(&count); err != nil {
		t.Fatalf("count tag: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected imported tag, got %d", count)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM play_sessions WHERE id = ? AND game_id = ?`, sessionID, gameID).Scan(&count); err != nil {
		t.Fatalf("count session: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected imported session, got %d", count)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_tombstones`).Scan(&count); err != nil {
		t.Fatalf("count tombstones: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected import tombstones to be cleared, got %d", count)
	}
}
