package migrations

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

func TestMigrationsFromInitToLatest(t *testing.T) {
	assertMigrationRegistry(t)

	dbPath := filepath.Join(t.TempDir(), "migration-e2e.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	if err := InitSchema(db); err != nil {
		t.Fatalf("initialize schema: %v", err)
	}

	executableDir := filepath.Join(t.TempDir(), "seed-game", "bin")
	if err := os.MkdirAll(executableDir, 0o755); err != nil {
		t.Fatalf("create seed executable directory: %v", err)
	}
	executablePath := filepath.Join(executableDir, "game.exe")
	if err := os.WriteFile(executablePath, []byte("migration e2e fixture"), 0o644); err != nil {
		t.Fatalf("create seed executable: %v", err)
	}

	seedMigrationE2EData(t, db, executablePath)

	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("run all migrations: %v", err)
	}
	if err := InitIndexes(db); err != nil {
		t.Fatalf("initialize indexes: %v", err)
	}

	assertAllMigrationsApplied(t, db)
	assertMigrationE2EData(t, db, executableDir)

	// A second startup must leave both the schema and migration history intact.
	if err := InitSchema(db); err != nil {
		t.Fatalf("initialize schema a second time: %v", err)
	}
	if err := Run(context.Background(), db); err != nil {
		t.Fatalf("run migrations a second time: %v", err)
	}
	if err := InitIndexes(db); err != nil {
		t.Fatalf("initialize indexes a second time: %v", err)
	}
	assertAllMigrationsApplied(t, db)
	assertMigrationE2EData(t, db, executableDir)
}

func assertMigrationRegistry(t *testing.T) {
	t.Helper()
	if len(migrations) == 0 {
		t.Fatal("migration registry is empty")
	}

	versions := make(map[int]struct{}, len(migrations))
	previousVersion := 0
	for _, migration := range migrations {
		if migration.Version <= previousVersion {
			t.Fatalf("migration versions must be strictly increasing: %d follows %d", migration.Version, previousVersion)
		}
		if _, exists := versions[migration.Version]; exists {
			t.Fatalf("duplicate migration version: %d", migration.Version)
		}
		if migration.Description == "" {
			t.Fatalf("migration %d has an empty description", migration.Version)
		}
		if migration.Up == nil {
			t.Fatalf("migration %d has no Up function", migration.Version)
		}
		versions[migration.Version] = struct{}{}
		previousVersion = migration.Version
	}
}

func seedMigrationE2EData(t *testing.T, db *sql.DB, executablePath string) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO users (id, created_at, default_backup_target)
		VALUES ('seed-user', TIMESTAMPTZ '2025-01-01 08:00:00+08', 'local');

		INSERT INTO categories (id, name, emoji, created_at, updated_at, is_system) VALUES
			('system:favorites', '最喜欢的游戏', '❤️', TIMESTAMPTZ '2025-01-01 08:00:00+08', TIMESTAMPTZ '2025-01-01 08:00:00+08', TRUE),
			('legacy:favorites', '最喜欢的游戏', '', TIMESTAMPTZ '2024-01-01 08:00:00+08', TIMESTAMPTZ '2024-01-01 08:00:00+08', TRUE);
	`); err != nil {
		t.Fatalf("seed users and categories: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, aliases, cover_url, path, wine_runner, launch_mode,
			status, source_type, source_id, cached_at, created_at, updated_at
		) VALUES (
			'seed-game', 'Seed Game', '', 'https://example.com/cover.jpg', ?, 'custom', 'normal',
			'playing', ' Bangumi ', '42', TIMESTAMPTZ '2025-01-02 08:00:00+08',
			TIMESTAMPTZ '2025-01-01 08:00:00+08', TIMESTAMPTZ '2025-01-02 08:00:00+08'
		), (
			'invalid-bangumi', 'Invalid Bangumi', '[]', '', '', '', 'normal',
			'not_started', 'Bangumi', '  ', NULL,
			TIMESTAMPTZ '2025-01-01 08:00:00+08', TIMESTAMPTZ '2025-01-01 08:00:00+08'
		)
	`, executablePath); err != nil {
		t.Fatalf("seed games: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO game_categories (game_id, category_id, updated_at)
		VALUES ('seed-game', 'legacy:favorites', TIMESTAMPTZ '2025-01-02 08:00:00+08');

		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		VALUES ('seed-session', 'seed-game', TIMESTAMPTZ '2025-01-02 08:00:00+08', TIMESTAMPTZ '2025-01-02 09:00:00+08', 3600, NULL);

		INSERT INTO game_progress (id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at)
		VALUES ('seed-progress', 'seed-game', 'chapter-1', 'route-a', 'seed note', 'chapter', TIMESTAMPTZ '2025-01-02 09:00:00+08');

		INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at)
		VALUES ('seed-tag', 'seed-game', 'story', 'user', 1.0, FALSE, TIMESTAMPTZ '2025-01-02 08:00:00+08', NULL);

		INSERT INTO game_filter_presets (id, name, tags, status)
		VALUES ('seed-preset', 'Seed Preset', '["story"]', 'playing');

		INSERT INTO game_reviews (game_id, rating, content, is_spoiler)
		VALUES ('seed-game', 9, 'seed review', FALSE);

		INSERT INTO sync_tombstones (entity_type, entity_id, parent_id, secondary_id, deleted_at)
		VALUES ('game', 'deleted-game', '', '', TIMESTAMPTZ '2025-01-03 08:00:00+08');
	`); err != nil {
		t.Fatalf("seed related records: %v", err)
	}
}

func assertAllMigrationsApplied(t *testing.T, db *sql.DB) {
	t.Helper()

	rows, err := db.Query(`
		SELECT version, description
		FROM schema_migrations
		ORDER BY version
	`)
	if err != nil {
		t.Fatalf("query migration history: %v", err)
	}
	defer rows.Close()

	applied := make([]Migration, 0, len(migrations))
	for rows.Next() {
		var migration Migration
		if err := rows.Scan(&migration.Version, &migration.Description); err != nil {
			t.Fatalf("scan migration history: %v", err)
		}
		applied = append(applied, migration)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migration history: %v", err)
	}

	if len(applied) != len(migrations) {
		t.Fatalf("applied migration count: got %d, want %d", len(applied), len(migrations))
	}
	for index, expected := range migrations {
		if applied[index].Version != expected.Version || applied[index].Description != expected.Description {
			t.Fatalf(
				"migration history at index %d: got %d %q, want %d %q",
				index,
				applied[index].Version,
				applied[index].Description,
				expected.Version,
				expected.Description,
			)
		}
	}
}

func assertMigrationE2EData(t *testing.T, db *sql.DB, executableDir string) {
	t.Helper()

	var coverSourceURL string
	var gameDirectory string
	var wineRunner string
	var launchMode string
	var aliases string
	if err := db.QueryRow(`
		SELECT cover_source_url, game_directory, wine_runner, launch_mode, aliases
		FROM games
		WHERE id = 'seed-game'
	`).Scan(&coverSourceURL, &gameDirectory, &wineRunner, &launchMode, &aliases); err != nil {
		t.Fatalf("query migrated seed game: %v", err)
	}
	if coverSourceURL != "https://example.com/cover.jpg" {
		t.Errorf("cover source URL: got %q", coverSourceURL)
	}
	if gameDirectory != executableDir {
		t.Errorf("game directory: got %q, want %q", gameDirectory, executableDir)
	}
	if wineRunner != "system" || launchMode != "compatibility" {
		t.Errorf("compatibility launch settings: got runner=%q mode=%q", wineRunner, launchMode)
	}
	if aliases != "[]" {
		t.Errorf("aliases: got %q, want []", aliases)
	}

	var sourceType string
	var sourceID string
	if err := db.QueryRow(`
		SELECT source_type, source_id
		FROM game_metadata_sources
		WHERE game_id = 'seed-game'
	`).Scan(&sourceType, &sourceID); err != nil {
		t.Fatalf("query migrated metadata source: %v", err)
	}
	if sourceType != "bangumi" || sourceID != "42" {
		t.Errorf("metadata source: got %q/%q, want bangumi/42", sourceType, sourceID)
	}

	if err := db.QueryRow(`
		SELECT source_type, source_id
		FROM games
		WHERE id = 'invalid-bangumi'
	`).Scan(&sourceType, &sourceID); err != nil {
		t.Fatalf("query normalized Bangumi game: %v", err)
	}
	if sourceType != "local" || sourceID != "" {
		t.Errorf("normalized Bangumi source: got %q/%q, want local with an empty ID", sourceType, sourceID)
	}

	var legacyFavoriteCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM categories WHERE id = 'legacy:favorites'`).Scan(&legacyFavoriteCount); err != nil {
		t.Fatalf("count legacy favorites categories: %v", err)
	}
	if legacyFavoriteCount != 0 {
		t.Errorf("legacy favorites category count: got %d, want 0", legacyFavoriteCount)
	}

	var favoriteRelationCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM game_categories
		WHERE game_id = 'seed-game' AND category_id = 'system:favorites'
	`).Scan(&favoriteRelationCount); err != nil {
		t.Fatalf("count migrated favorites relations: %v", err)
	}
	if favoriteRelationCount != 1 {
		t.Errorf("stable favorites relation count: got %d, want 1", favoriteRelationCount)
	}

	for table, identity := range map[string][2]string{
		"users":               {"id", "seed-user"},
		"play_sessions":       {"id", "seed-session"},
		"game_progress":       {"id", "seed-progress"},
		"game_tags":           {"id", "seed-tag"},
		"game_filter_presets": {"id", "seed-preset"},
		"game_reviews":        {"game_id", "seed-game"},
		"sync_tombstones":     {"entity_id", "deleted-game"},
	} {
		var count int
		query := "SELECT COUNT(*) FROM " + table + " WHERE " + identity[0] + " = ?"
		if err := db.QueryRow(query, identity[1]).Scan(&count); err != nil {
			t.Fatalf("count seed record in %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("seed record count in %s: got %d, want 1", table, count)
		}
	}

	for table, id := range map[string]string{
		"play_sessions": "seed-session",
		"game_tags":     "seed-tag",
	} {
		var missingTimestampCount int
		query := "SELECT COUNT(*) FROM " + table + " WHERE id = ? AND updated_at IS NULL"
		if err := db.QueryRow(query, id).Scan(&missingTimestampCount); err != nil {
			t.Fatalf("inspect updated_at in %s: %v", table, err)
		}
		if missingTimestampCount != 0 {
			t.Errorf("record in %s still has a NULL updated_at", table)
		}
	}

	var indexCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM duckdb_indexes()
		WHERE index_name IN (
			'idx_games_rating',
			'idx_games_path',
			'idx_game_metadata_sources_identity',
			'idx_game_progress_game_timeline'
		)
	`).Scan(&indexCount); err != nil {
		t.Fatalf("inspect representative indexes: %v", err)
	}
	if indexCount != 4 {
		t.Errorf("representative index count: got %d, want 4", indexCount)
	}
}
