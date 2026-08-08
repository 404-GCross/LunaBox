package service

import (
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/metadata"
	"testing"
)

func TestFetchImportMetadataSourceReturnsEverySameNameCandidate(t *testing.T) {
	source := metadataSearchSource{
		source: enums.VNDB,
		fetchCandidatesByName: func(string) ([]metadata.MetadataResult, error) {
			return []metadata.MetadataResult{
				{Game: models.Game{Name: "同名游戏", SourceID: "v1"}},
				{Game: models.Game{Name: "同名游戏", SourceID: "v2"}},
			}, nil
		},
	}

	matches, sourceErr := fetchImportMetadataSource(source, "同名游戏")
	if sourceErr != nil {
		t.Fatalf("unexpected source error: %v", sourceErr)
	}
	if len(matches) != 2 {
		t.Fatalf("expected two candidates, got %d", len(matches))
	}
	if matches[0].Game.SourceID != "v1" || matches[1].Game.SourceID != "v2" {
		t.Fatalf("unexpected candidate IDs: %q, %q", matches[0].Game.SourceID, matches[1].Game.SourceID)
	}
}
