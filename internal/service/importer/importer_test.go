package importer

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/potatovn"
	"lunabox/internal/models/vnite"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPotatoVNConvertToGameImportsLaunchFields(t *testing.T) {
	exePath := `D:\Games\potato\game.exe`
	processName := "actual.exe"

	releaseDate := potatovn.FlexibleTime(time.Date(2024, 5, 6, 0, 0, 0, 0, time.UTC))
	galgame := potatovn.Galgame{
		Name:                potatovn.LockableProperty[string]{Value: "Potato Game"},
		Developer:           potatovn.LockableProperty[string]{Value: "Dev"},
		Description:         potatovn.LockableProperty[string]{Value: "Summary"},
		Rating:              potatovn.LockableProperty[float64]{Value: 8.5},
		ReleaseDate:         potatovn.LockableProperty[potatovn.FlexibleTime]{Value: releaseDate},
		ExePath:             &exePath,
		ProcessName:         &processName,
		RunInLocaleEmulator: true,
		EnableMagpie:        true,
	}

	game, _ := NewPotatoVNImporter(Dependencies{}).convertToGame(galgame, "", "")

	if game.Path != exePath {
		t.Fatalf("expected path %q, got %q", exePath, game.Path)
	}
	if game.ProcessName != processName {
		t.Fatalf("expected process name %q, got %q", processName, game.ProcessName)
	}
	if !game.UseLocaleEmulator {
		t.Fatal("expected Locale Emulator flag to be imported")
	}
	if !game.UseMagpie {
		t.Fatal("expected Magpie flag to be imported")
	}
	if game.ReleaseDate != "2024-05-06" {
		t.Fatalf("expected release date 2024-05-06, got %q", game.ReleaseDate)
	}
	if game.Rating != 8.5 {
		t.Fatalf("expected rating 8.5, got %f", game.Rating)
	}
}

func TestAddImportedItemsUsesBatchDependency(t *testing.T) {
	called := false
	deps := Dependencies{
		AddItems: func(items []ImportItem) (ImportResult, error) {
			called = true
			if len(items) != 1 {
				t.Fatalf("expected 1 item, got %d", len(items))
			}
			if items[0].Source.Game.Name != "Batch Game" {
				t.Fatalf("unexpected item: %+v", items[0])
			}
			return ImportResult{Success: 1}, nil
		},
	}

	result, err := addImportedItems(deps, []ImportItem{
		{
			Source: vo.GameMetadataFromWebVO{
				Source: enums.Local,
				Game: models.Game{
					ID:   "batch-game",
					Name: "Batch Game",
				},
			},
			DisplayName: "Batch Game",
		},
	})
	if err != nil {
		t.Fatalf("addImportedItems returned error: %v", err)
	}
	if !called {
		t.Fatal("expected AddItems dependency to be used")
	}
	if result.Success != 1 {
		t.Fatalf("expected success 1, got %d", result.Success)
	}
}

func TestPotatoVNImportSamePathMergeTargetsExistingGame(t *testing.T) {
	exePath := `D:\Games\Same\game.exe`
	existingGame := models.Game{
		ID:   "existing-game",
		Name: "Existing Game",
		Path: exePath,
	}
	galgame := potatovn.Galgame{
		Name:    potatovn.LockableProperty[string]{Value: "Imported Game"},
		ExePath: &exePath,
		PlayedTime: map[string]int{
			"2024/1/2": 30,
		},
	}
	tempDir := t.TempDir()
	data, err := json.Marshal([]potatovn.Galgame{galgame})
	if err != nil {
		t.Fatalf("marshal galgame: %v", err)
	}
	zipPath := filepath.Join(tempDir, "potatovn.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zipWriter := zip.NewWriter(zipFile)
	entry, err := zipWriter.Create("data.galgames.json")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := zipFile.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	var committed []ImportItem
	deps := Dependencies{
		ListGames: func() ([]models.Game, error) {
			return []models.Game{existingGame}, nil
		},
		AddItems: func(items []ImportItem) (ImportResult, error) {
			committed = items
			sessionsImported := 0
			if len(items) > 0 {
				sessionsImported = len(items[0].Sessions)
			}
			return ImportResult{Success: len(items), SessionsImported: sessionsImported}, nil
		},
	}

	result, err := NewPotatoVNImporter(deps).Import(zipPath, true, SamePathActionMerge)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if result.Skipped != 0 || result.Success != 1 {
		t.Fatalf("expected one merged success without skips, got result=%+v", result)
	}
	if len(committed) != 1 {
		t.Fatalf("expected one committed item, got %d", len(committed))
	}
	if committed[0].Action != ImportActionUpdateExisting {
		t.Fatalf("expected update action, got %q", committed[0].Action)
	}
	if committed[0].ExistingGameID != existingGame.ID || committed[0].Source.Game.ID != existingGame.ID {
		t.Fatalf("expected existing game id %q, got item=%+v", existingGame.ID, committed[0])
	}
	if len(committed[0].Sessions) != 1 || committed[0].Sessions[0].GameID != existingGame.ID {
		t.Fatalf("expected session to target existing game, got %+v", committed[0].Sessions)
	}
}

func TestPreviewExistsChecksSourceDuplicate(t *testing.T) {
	idx := newExistingPreviewIndex([]models.Game{
		{
			ID:         "existing-source",
			Name:       "Existing Source",
			Path:       `D:\Games\Existing\game.exe`,
			SourceType: enums.VNDB,
			SourceID:   "v123",
		},
	})

	if !previewExists(idx, "Existing Source Volume 2", `D:\Games\Volume2\game.exe`, string(enums.VNDB), "v123") {
		t.Fatal("expected preview to warn when metadata source/id already exists")
	}
}

func TestPreviewExistsChecksParentChildPathDuplicate(t *testing.T) {
	idx := newExistingPreviewIndex([]models.Game{
		{
			ID:   "existing-path",
			Name: "Existing Path",
			Path: `D:\Games\Existing`,
		},
	})

	if !previewExists(idx, "Existing Path Child", `D:\Games\Existing\game.exe`, string(enums.Local), "") {
		t.Fatal("expected preview to warn when path is inside existing game folder")
	}
}

func TestSkipExistingGameChecksParentChildPathDuplicate(t *testing.T) {
	result := newImportResult()
	existingPaths := map[string]string{
		normalizeImportPath(`D:\Games\Existing`): "Existing Path",
	}

	if !skipExistingGame(nil, "TestImport", &result, nil, map[string]string{}, existingPaths, "Existing Path Child", `D:\Games\Existing\game.exe`) {
		t.Fatal("expected import to skip when path is inside existing game folder")
	}
	if result.Skipped != 1 {
		t.Fatalf("expected skipped 1, got %d", result.Skipped)
	}
}

func TestVniteConvertToGameImportsLaunchFields(t *testing.T) {
	gameDoc := vnite.GameDoc{
		Metadata: vnite.GameMetadata{
			Name:        "Vnite Game",
			ReleaseDate: "2025-07-08",
			Developers:  []string{"Dev"},
			Tags:        []string{"ADV", "ADV", "Visual Novel"},
		},
		Record: vnite.GameRecord{
			AddDate: "2025-01-02T03:04:05Z",
		},
	}
	localDoc := vnite.GameLocalDoc{
		Path: vnite.GameLocalPath{
			GamePath:  `D:\Games\vnite-folder`,
			SavePaths: []string{`D:\Saves\vnite`},
		},
		Launcher: vnite.GameLauncher{
			Mode:                "file",
			UseMagpie:           true,
			UseLocaleEmulator:   true,
			RunInLocaleEmulator: false,
			FileConfig: vnite.GameFileConfig{
				Path:        `D:\Games\vnite\start.exe`,
				MonitorMode: "process",
				MonitorPath: "actual.exe",
			},
		},
	}

	game, _ := NewVniteImporter(Dependencies{}).convertToGame(gameDoc, localDoc)

	if game.Path != localDoc.Launcher.FileConfig.Path {
		t.Fatalf("expected file launcher path %q, got %q", localDoc.Launcher.FileConfig.Path, game.Path)
	}
	if game.SavePath != localDoc.Path.SavePaths[0] {
		t.Fatalf("expected save path %q, got %q", localDoc.Path.SavePaths[0], game.SavePath)
	}
	if game.ProcessName != localDoc.Launcher.FileConfig.MonitorPath {
		t.Fatalf("expected process name %q, got %q", localDoc.Launcher.FileConfig.MonitorPath, game.ProcessName)
	}
	if !game.UseLocaleEmulator {
		t.Fatal("expected Locale Emulator flag to be imported")
	}
	if !game.UseMagpie {
		t.Fatal("expected Magpie flag to be imported")
	}
	if game.ReleaseDate != "2025-07-08" {
		t.Fatalf("expected release date 2025-07-08, got %q", game.ReleaseDate)
	}
}

func TestVniteGamePathFallsBackToGamePath(t *testing.T) {
	localDoc := vnite.GameLocalDoc{
		Path: vnite.GameLocalPath{
			GamePath: `D:\Games\folder-mode`,
		},
		Launcher: vnite.GameLauncher{
			Mode: "url",
			FileConfig: vnite.GameFileConfig{
				Path: `D:\Games\unused.exe`,
			},
		},
	}

	if got := pickVniteGamePath(localDoc); got != localDoc.Path.GamePath {
		t.Fatalf("expected fallback gamePath %q, got %q", localDoc.Path.GamePath, got)
	}
}

func TestVniteGamePathFallsBackToMarkPath(t *testing.T) {
	const rawLocalDoc = `{
		"path": {"gamePath": "", "savePaths": []},
		"launcher": {
			"mode": "file",
			"fileConfig": {"path": "", "args": [], "monitorMode": "folder", "monitorPath": ""}
		},
		"utils": {"markPath": "D:\\Games\\unplayed-folder"}
	}`

	var localDoc vnite.GameLocalDoc
	if err := json.Unmarshal([]byte(rawLocalDoc), &localDoc); err != nil {
		t.Fatalf("unmarshal local doc: %v", err)
	}

	if got := pickVniteGamePath(localDoc); got != localDoc.Utils.MarkPath {
		t.Fatalf("expected markPath fallback %q, got %q", localDoc.Utils.MarkPath, got)
	}
}

func TestTagsFromNamesDeduplicatesAsUserTags(t *testing.T) {
	tags := tagsFromNames([]string{"ADV", " adv ", "", "Visual Novel"})
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Name != "ADV" || tags[0].Source != "user" {
		t.Fatalf("unexpected first tag: %+v", tags[0])
	}
	if tags[1].Name != "Visual Novel" || tags[1].Source != "user" {
		t.Fatalf("unexpected second tag: %+v", tags[1])
	}
}

func TestLoadReinaManagerDataFromSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "reina.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite fixture: %v", err)
	}
	statements := []string{
		`CREATE TABLE games (
			id INTEGER PRIMARY KEY, id_type TEXT, date TEXT, localpath TEXT, executable TEXT,
			savepath TEXT, clear INTEGER, le_launch INTEGER, magpie INTEGER, custom_data TEXT,
			created_at INTEGER, updated_at INTEGER
		)`,
		`CREATE TABLE game_sources (game_id INTEGER, source TEXT, external_id TEXT, data TEXT)`,
		`CREATE TABLE game_sessions (game_id INTEGER, start_time INTEGER, end_time INTEGER, duration INTEGER)`,
		`INSERT INTO games VALUES (
			1, 'vndb', '2024-01-02', 'D:\Games\Reina', 'game.exe', 'D:\Saves\Reina',
			3, 1, 0, '{}', 1700000000, 1700000100
		)`,
		`INSERT INTO game_sources VALUES (
			1, 'vndb', 'v123', '{"name":"Reina Game","developer":"Reina Studio","tags":["ADV"]}'
		)`,
		`INSERT INTO game_sessions VALUES (1, 1700000200, 1700000260, 1)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatalf("prepare sqlite fixture: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite fixture: %v", err)
	}

	data, err := loadReinaManagerData(dbPath)
	if err != nil {
		t.Fatalf("load ReinaManager sqlite data: %v", err)
	}
	if len(data.Games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(data.Games))
	}
	game := data.Games[0]
	if game.ID != 1 || game.Sources["vndb"].Data.Name != "Reina Game" || len(game.Sessions) != 1 {
		t.Fatalf("unexpected loaded ReinaManager data: %+v", game)
	}
}

func TestReinaManagerMixedMappingUsesMergedFieldsAndBangumiIdentity(t *testing.T) {
	bgmScore := 8.4
	vndbScore := 7.9
	nsfw := true
	source := reinaManagerGame{
		ID:         25,
		IDType:     "mixed",
		Date:       "2024-06-01",
		LocalPath:  `D:\Games\Mixed`,
		Executable: "start.exe",
		SavePath:   `D:\Saves\Mixed`,
		Clear:      3,
		CreatedAt:  1_700_000_000,
		UpdatedAt:  1_700_000_100,
		Sources: map[string]reinaManagerSource{
			"bgm": {
				Source:     "bgm",
				ExternalID: "12345",
				Data: reinaManagerMetadata{
					Name:    "BGM Name",
					NameCN:  "Bangumi 中文名",
					Image:   "https://example.com/bgm.jpg",
					Summary: "BGM Summary",
					Tags:    []string{"ADV", "Drama"},
					Score:   &bgmScore,
				},
			},
			"vndb": {
				Source:     "vndb",
				ExternalID: "v999",
				Data: reinaManagerMetadata{
					Name:      "VNDB Name",
					Developer: "VNDB Studio",
					Tags:      []string{"Drama", "Visual Novel"},
					Score:     &vndbScore,
					NSFW:      &nsfw,
				},
			},
		},
		Sessions: []reinaManagerSession{{
			StartTime: 1_700_001_000,
			EndTime:   1_700_001_620,
			Duration:  10,
		}},
	}

	game, sessions := convertReinaManagerGame(source)

	if game.SourceType != enums.Bangumi || game.SourceID != "12345" {
		t.Fatalf("expected Bangumi identity for mixed game, got %s/%s", game.SourceType, game.SourceID)
	}
	if game.Name != "Bangumi 中文名" || game.CoverURL != "https://example.com/bgm.jpg" {
		t.Fatalf("unexpected mixed basic fields: name=%q cover=%q", game.Name, game.CoverURL)
	}
	if game.Company != "VNDB Studio" || game.Summary != "BGM Summary" || game.Rating != bgmScore {
		t.Fatalf("unexpected merged metadata: %+v", game)
	}
	if game.Path != `D:\Games\Mixed\start.exe` || game.SavePath != source.SavePath {
		t.Fatalf("unexpected imported paths: path=%q save=%q", game.Path, game.SavePath)
	}
	if game.Status != enums.StatusPlaying {
		t.Fatalf("expected playing status, got %q", game.Status)
	}
	if len(sessions) != 1 || sessions[0].Duration != 600 || sessions[0].GameID != game.ID {
		t.Fatalf("expected 10-minute session converted to 600 seconds, got %+v", sessions)
	}

	tags := tagsFromNames(collectReinaManagerTags(source))
	if len(tags) != 3 || tags[0].Name != "ADV" || tags[1].Name != "Drama" || tags[2].Name != "Visual Novel" {
		t.Fatalf("unexpected merged tags: %+v", tags)
	}
}

func TestReinaManagerCustomDataOverridesMixedMetadata(t *testing.T) {
	rating := 9.6
	nsfw := false
	source := reinaManagerGame{
		IDType: "mixed",
		Custom: reinaManagerCustomData{
			Name:       "Custom Name",
			Image:      "https://example.com/custom.png",
			Summary:    "Custom Summary",
			Developer:  "Custom Studio",
			UserRating: &rating,
			NSFW:       &nsfw,
		},
		Sources: map[string]reinaManagerSource{
			"bgm": {
				ExternalID: "88",
				Data: reinaManagerMetadata{
					Name:      "Source Name",
					Image:     "https://example.com/source.png",
					Summary:   "Source Summary",
					Developer: "Source Studio",
				},
			},
		},
	}

	game, _ := convertReinaManagerGame(source)
	if game.Name != "Custom Name" || game.CoverURL != "https://example.com/custom.png" ||
		game.Summary != "Custom Summary" || game.Company != "Custom Studio" ||
		game.Rating != rating || game.IsNSFW {
		t.Fatalf("expected custom data to override source metadata, got %+v", game)
	}
}
