package cloudsync

import (
	"testing"
	"time"
)

func TestMergeCoversPrefersExistingCoverOverImplicitDeletionOnTie(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 49, 2, 0, time.UTC)
	game := Game{ID: "game-with-avif", Name: "AVIF Game", CreatedAt: now, UpdatedAt: now}
	cover := CoverAsset{GameID: game.ID, Ext: ".avif", UpdatedAt: now}

	helper := &Helper{}
	merged := helper.MergeSnapshots(
		Snapshot{Games: []Game{game}, Covers: []CoverAsset{cover}},
		Snapshot{Games: []Game{game}},
		true,
	)

	if len(merged.Covers) != 1 || merged.Covers[0] != cover {
		t.Fatalf("MergeSnapshots() covers = %+v, want existing AVIF cover", merged.Covers)
	}
}

func TestMergeCoversPrefersContentHashOverLegacyHashOnTie(t *testing.T) {
	now := time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC)
	game := Game{ID: "game-1", UpdatedAt: now}
	localCover := CoverAsset{
		GameID: game.ID, Ext: ".webp", UpdatedAt: now,
		Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	remoteCover := CoverAsset{GameID: game.ID, Ext: ".webp", UpdatedAt: now, Hash: "legacy-hash"}

	merged := (&Helper{}).MergeSnapshots(
		Snapshot{Games: []Game{game}, Covers: []CoverAsset{localCover}},
		Snapshot{Games: []Game{game}, Covers: []CoverAsset{remoteCover}},
		true,
	)

	if len(merged.Covers) != 1 || merged.Covers[0] != localCover {
		t.Fatalf("MergeSnapshots() covers = %+v, want local content hash", merged.Covers)
	}
}
