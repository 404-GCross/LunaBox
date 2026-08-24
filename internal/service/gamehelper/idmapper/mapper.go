package idmapper

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"lunabox/internal/common/enums"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed game_id_mapper.db
var embeddedDatabase embed.FS

const embeddedDatabaseName = "game_id_mapper.db"

// IDs contains the supported metadata identifiers for one game.
type IDs struct {
	VNDBID       int64
	BangumiID    int64
	SteamID      int64
	HikarinagiID int64
}

// Mapper keeps the embedded SQLite mapping in memory for fast batch lookups.
type Mapper struct {
	byVNDB       map[int64]IDs
	byBangumi    map[int64]IDs
	bySteam      map[int64]IDs
	byHikarinagi map[int64]IDs
}

var (
	embeddedOnce   sync.Once
	embeddedMapper *Mapper
	embeddedErr    error
)

// LoadEmbedded loads the bundled mapping database once per process.
func LoadEmbedded() (*Mapper, error) {
	embeddedOnce.Do(func() {
		embeddedMapper, embeddedErr = loadEmbeddedDatabase()
	})
	return embeddedMapper, embeddedErr
}

// New creates an in-memory mapper. It is primarily useful for tests.
func New(records []IDs) *Mapper {
	mapper := &Mapper{
		byVNDB:       make(map[int64]IDs, len(records)),
		byBangumi:    make(map[int64]IDs, len(records)),
		bySteam:      make(map[int64]IDs, len(records)),
		byHikarinagi: make(map[int64]IDs, len(records)),
	}
	for _, record := range records {
		mapper.add(record)
	}
	return mapper
}

// Resolve finds a mapping from one normalized metadata source identifier.
func (m *Mapper) Resolve(source enums.SourceType, sourceID string) (IDs, bool) {
	if m == nil {
		return IDs{}, false
	}

	numericID, ok := parseSourceID(source, sourceID)
	if !ok {
		return IDs{}, false
	}

	switch source {
	case enums.VNDB:
		result, found := m.byVNDB[numericID]
		return result, found
	case enums.Bangumi:
		result, found := m.byBangumi[numericID]
		return result, found
	case enums.Steam:
		result, found := m.bySteam[numericID]
		return result, found
	case enums.Hikarinagi:
		result, found := m.byHikarinagi[numericID]
		return result, found
	default:
		return IDs{}, false
	}
}

func loadEmbeddedDatabase() (*Mapper, error) {
	databaseBytes, err := embeddedDatabase.ReadFile(embeddedDatabaseName)
	if err != nil {
		return nil, fmt.Errorf("read embedded game ID mapper: %w", err)
	}

	tempFile, err := os.CreateTemp("", "lunabox-game-id-mapper-*.db")
	if err != nil {
		return nil, fmt.Errorf("create temporary game ID mapper: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(databaseBytes); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("write temporary game ID mapper: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temporary game ID mapper: %w", err)
	}

	dsn := "file:" + filepath.ToSlash(tempPath) + "?mode=ro&_query_only=1"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open embedded game ID mapper: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	rows, err := db.Query(`
		SELECT vndb_id, bangumi_id, steam_id, hikarinagiid
		FROM id_map
		ORDER BY vndb_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query embedded game ID mapper: %w", err)
	}
	defer rows.Close()

	records := make([]IDs, 0, 30000)
	for rows.Next() {
		var vndbID sql.NullInt64
		var bangumiID sql.NullInt64
		var steamID sql.NullInt64
		var hikarinagiID sql.NullInt64
		if err := rows.Scan(&vndbID, &bangumiID, &steamID, &hikarinagiID); err != nil {
			return nil, fmt.Errorf("scan embedded game ID mapping: %w", err)
		}
		records = append(records, IDs{
			VNDBID:       nullablePositiveID(vndbID),
			BangumiID:    nullablePositiveID(bangumiID),
			SteamID:      nullablePositiveID(steamID),
			HikarinagiID: nullablePositiveID(hikarinagiID),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embedded game ID mappings: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("embedded game ID mapper contains no records")
	}
	return New(records), nil
}

func (m *Mapper) add(record IDs) {
	if record.VNDBID > 0 {
		m.byVNDB[record.VNDBID] = record
	}
	if record.BangumiID > 0 {
		m.byBangumi[record.BangumiID] = record
	}
	if record.SteamID > 0 {
		m.bySteam[record.SteamID] = record
	}
	if record.HikarinagiID > 0 {
		m.byHikarinagi[record.HikarinagiID] = record
	}
}

func nullablePositiveID(value sql.NullInt64) int64 {
	if !value.Valid || value.Int64 <= 0 {
		return 0
	}
	return value.Int64
}

func parseSourceID(source enums.SourceType, sourceID string) (int64, bool) {
	normalized := strings.ToLower(strings.TrimSpace(sourceID))
	if source == enums.VNDB {
		normalized = strings.TrimPrefix(normalized, "v")
	}
	if normalized == "" {
		return 0, false
	}

	value, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}
