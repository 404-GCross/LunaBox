package importer

import (
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/potatovn"
	"lunabox/internal/models/vnite"
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

	game, _ := NewPotatoVNImporter(Dependencies{}).convertToGame(galgame, "")

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
