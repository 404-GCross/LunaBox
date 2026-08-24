package gamehelper

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/migrations"

	_ "github.com/duckdb/duckdb-go/v2"
)

func setupGameListQueryTest(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := migrations.InitSchema(db); err != nil {
		t.Fatalf("initialize test schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, source_type, source_id, cached_at, created_at, updated_at
		) VALUES
			('legacy-bangumi', 'Legacy Bangumi', 'Bangumi', '101', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('local', 'Local', 'local', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('empty-source', 'Empty Source', '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('mixed', 'Mixed Legacy', 'mixed', 'legacy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('multi', 'Multi Source', 'bangumi', '202', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('steam', 'Steam', 'steam', '303', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
		INSERT INTO game_metadata_sources (game_id, source_type, source_id)
		VALUES
			('multi', 'bangumi', '202'),
			('multi', 'vndb', 'v202'),
			('steam', 'steam', '303');
	`); err != nil {
		t.Fatalf("insert game list fixtures: %v", err)
	}
	return db
}

func TestQueryGameListFiltersMetadataSources(t *testing.T) {
	db := setupGameListQueryTest(t)
	tests := []struct {
		name   string
		source enums.SourceType
		want   []string
	}{
		{name: "local", source: enums.Local, want: []string{"local"}},
		{name: "legacy remote", source: enums.Bangumi, want: []string{"legacy-bangumi", "multi"}},
		{name: "secondary remote", source: enums.VNDB, want: []string{"multi"}},
		{name: "single remote", source: enums.Steam, want: []string{"steam"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := test.source
			response, err := QueryGameList(context.Background(), db, vo.GameListRequest{
				Limit:          100,
				MetadataSource: &source,
				SortBy:         enums.GameListSortByName,
				SortOrder:      enums.SortOrderAsc,
			}, GameListScope{})
			if err != nil {
				t.Fatalf("query games: %v", err)
			}
			got := make([]string, 0, len(response.Games))
			for _, game := range response.Games {
				got = append(got, game.ID)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("unexpected game IDs: got %v want %v", got, test.want)
			}
			if response.Total != len(test.want) {
				t.Fatalf("unexpected total: got %d want %d", response.Total, len(test.want))
			}
		})
	}
}
