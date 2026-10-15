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

func TestNormalizeDefaultMetadataSource(t *testing.T) {
	tests := []struct {
		name         string
		source       enums.SourceType
		sourceID     string
		wantSource   enums.SourceType
		wantSourceID string
	}{
		{name: "empty source", source: "", sourceID: "unused", wantSource: enums.Local, wantSourceID: ""},
		{name: "local source", source: enums.SourceType(" LOCAL "), sourceID: "unused", wantSource: enums.Local, wantSourceID: ""},
		{name: "remote source", source: enums.SourceType(" VNDB "), sourceID: " v123 ", wantSource: enums.VNDB, wantSourceID: "v123"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, sourceID := NormalizeDefaultMetadataSource(test.source, test.sourceID)
			if source != test.wantSource || sourceID != test.wantSourceID {
				t.Fatalf("NormalizeDefaultMetadataSource returned %q/%q, want %q/%q", source, sourceID, test.wantSource, test.wantSourceID)
			}
		})
	}
}
