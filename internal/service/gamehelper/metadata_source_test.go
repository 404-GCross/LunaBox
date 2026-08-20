package gamehelper

import (
	"testing"

	"lunabox/internal/common/enums"
	"lunabox/internal/models"
)

func TestNormalizeMetadataSource(t *testing.T) {
	source, sourceID, err := NormalizeMetadataSource(enums.SourceType(" VNDB "), " v123 ")
	if err != nil {
		t.Fatalf("NormalizeMetadataSource returned error: %v", err)
	}
	if source != enums.VNDB || sourceID != "v123" {
		t.Fatalf("unexpected normalized source: %q %q", source, sourceID)
	}
}

func TestValidateInitialMetadataSourcesRejectsDuplicateProvider(t *testing.T) {
	err := ValidateInitialMetadataSources([]models.GameMetadataSource{
		{SourceType: enums.VNDB, SourceID: "v1"},
		{SourceType: enums.SourceType("vndb"), SourceID: "v2"},
	})
	if err == nil {
		t.Fatal("expected duplicate metadata provider to be rejected")
	}
}
