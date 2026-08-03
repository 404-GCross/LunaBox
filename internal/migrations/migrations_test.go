package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

func TestMigration165BackfillsCoverSourceAndGameDirectory(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE games (
			id TEXT PRIMARY KEY,
			cover_url TEXT,
			path TEXT
		)
	`); err != nil {
		t.Fatalf("create games table: %v", err)
	}

	gameRoot := filepath.Join(t.TempDir(), "game-root")
	executableDir := filepath.Join(gameRoot, "bin")
	if err := os.MkdirAll(executableDir, 0o755); err != nil {
		t.Fatalf("create executable directory: %v", err)
	}
	executablePath := filepath.Join(executableDir, "game.exe")
	if err := os.WriteFile(executablePath, []byte("test"), 0o644); err != nil {
		t.Fatalf("create executable: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO games (id, cover_url, path)
		VALUES
			('remote-cover', 'https://example.com/cover.jpg', ?),
			('directory-path', '/local/covers/local.jpg', ?)
	`, executablePath, gameRoot); err != nil {
		t.Fatalf("insert games: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin migration transaction: %v", err)
	}
	if err := migration165(tx); err != nil {
		tx.Rollback()
		t.Fatalf("run migration165: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration165: %v", err)
	}

	var coverSourceURL string
	var gameDirectory string
	if err := db.QueryRow(`
		SELECT cover_source_url, game_directory
		FROM games
		WHERE id = 'remote-cover'
	`).Scan(&coverSourceURL, &gameDirectory); err != nil {
		t.Fatalf("query migrated remote-cover game: %v", err)
	}
	if coverSourceURL != "https://example.com/cover.jpg" {
		t.Fatalf("unexpected cover source URL: %q", coverSourceURL)
	}
	if gameDirectory != executableDir {
		t.Fatalf("unexpected executable parent directory: got %q want %q", gameDirectory, executableDir)
	}

	if err := db.QueryRow(`
		SELECT cover_source_url, game_directory
		FROM games
		WHERE id = 'directory-path'
	`).Scan(&coverSourceURL, &gameDirectory); err != nil {
		t.Fatalf("query migrated directory-path game: %v", err)
	}
	if coverSourceURL != "" {
		t.Fatalf("local cover should not be copied to cover source URL: %q", coverSourceURL)
	}
	if gameDirectory != gameRoot {
		t.Fatalf("existing directory path should be preserved: got %q want %q", gameDirectory, gameRoot)
	}
}

func TestMigration166AddsLocalSteamIdentity(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE games (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create games table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO games (id) VALUES ('existing')`); err != nil {
		t.Fatalf("insert existing game: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin migration transaction: %v", err)
	}
	if err := migration166(tx); err != nil {
		tx.Rollback()
		t.Fatalf("run migration166: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration166: %v", err)
	}

	var launchID string
	var launchKind string
	var userID string
	if err := db.QueryRow(`
		SELECT steam_launch_id, steam_launch_kind, steam_user_id
		FROM games
		WHERE id = 'existing'
	`).Scan(&launchID, &launchKind, &userID); err != nil {
		t.Fatalf("query migrated Steam identity: %v", err)
	}
	if launchID != "" || launchKind != "" || userID != "" {
		t.Fatalf("unexpected Steam identity defaults: %q %q %q", launchID, launchKind, userID)
	}
}

func TestMigration167AddsSteamLaunchOptions(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE games (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create games table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO games (id) VALUES ('existing')`); err != nil {
		t.Fatalf("insert existing game: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin migration transaction: %v", err)
	}
	if err := migration167(tx); err != nil {
		tx.Rollback()
		t.Fatalf("run migration167: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration167: %v", err)
	}

	var launchOptions string
	if err := db.QueryRow(`
		SELECT steam_launch_options
		FROM games
		WHERE id = 'existing'
	`).Scan(&launchOptions); err != nil {
		t.Fatalf("query migrated Steam launch options: %v", err)
	}
	if launchOptions != "" {
		t.Fatalf("unexpected Steam launch options default: %q", launchOptions)
	}
}

func TestMigration168AddsLocaleEmulatorLocale(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE games (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create games table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO games (id) VALUES ('existing')`); err != nil {
		t.Fatalf("insert existing game: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin migration transaction: %v", err)
	}
	if err := migration168(tx); err != nil {
		tx.Rollback()
		t.Fatalf("run migration168: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit migration168: %v", err)
	}

	var locale string
	if err := db.QueryRow(`
		SELECT locale_emulator_locale
		FROM games
		WHERE id = 'existing'
	`).Scan(&locale); err != nil {
		t.Fatalf("query migrated locale emulator locale: %v", err)
	}
	if locale != "" {
		t.Fatalf("unexpected locale emulator locale default: %q", locale)
	}
}
