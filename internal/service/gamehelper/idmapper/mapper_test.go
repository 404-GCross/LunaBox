package idmapper

import (
	"testing"

	"lunabox/internal/common/enums"
)

func TestMapperResolvesHikarinagiID(t *testing.T) {
	mapper := New([]IDs{{VNDBID: 10, BangumiID: 20, SteamID: 30, HikarinagiID: 40}})

	mapping, found := mapper.Resolve(enums.Hikarinagi, "40")
	if !found || mapping.VNDBID != 10 || mapping.HikarinagiID != 40 {
		t.Fatalf("Resolve(Hikarinagi, 40) = %+v, %t", mapping, found)
	}
}

func TestEmbeddedMappingsAreReciprocal(t *testing.T) {
	mapper, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	if len(mapper.byVNDB) == 0 {
		t.Fatal("embedded mapper contains no VNDB records")
	}

	for vndbID, record := range mapper.byVNDB {
		if record.VNDBID != vndbID {
			t.Fatalf("VNDB key %d points to record v%d", vndbID, record.VNDBID)
		}
		if record.BangumiID > 0 {
			if reverse, ok := mapper.byBangumi[record.BangumiID]; !ok || reverse.VNDBID != vndbID {
				t.Fatalf("Bangumi %d is not reciprocal with VNDB v%d", record.BangumiID, vndbID)
			}
		}
		if record.SteamID > 0 {
			if reverse, ok := mapper.bySteam[record.SteamID]; !ok || reverse.VNDBID != vndbID {
				t.Fatalf("Steam %d is not reciprocal with VNDB v%d", record.SteamID, vndbID)
			}
		}
		if record.HikarinagiID > 0 {
			if reverse, ok := mapper.byHikarinagi[record.HikarinagiID]; !ok || reverse.VNDBID != vndbID {
				t.Fatalf("Hikarinagi %d is not reciprocal with VNDB v%d", record.HikarinagiID, vndbID)
			}
		}
	}

	for bangumiID, record := range mapper.byBangumi {
		if record.BangumiID != bangumiID || mapper.byVNDB[record.VNDBID] != record {
			t.Fatalf("Bangumi %d has a non-reciprocal mapping", bangumiID)
		}
	}
	for steamID, record := range mapper.bySteam {
		if record.SteamID != steamID || mapper.byVNDB[record.VNDBID] != record {
			t.Fatalf("Steam %d has a non-reciprocal mapping", steamID)
		}
	}
	for hikarinagiID, record := range mapper.byHikarinagi {
		if record.HikarinagiID != hikarinagiID || mapper.byVNDB[record.VNDBID] != record {
			t.Fatalf("Hikarinagi %d has a non-reciprocal mapping", hikarinagiID)
		}
	}
}
