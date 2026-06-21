package metadata

import "testing"

func TestExtractVNDBTagsKeepsLowRatedTags(t *testing.T) {
	tags := extractVNDBTags([]vndbTag{
		{Name: "low", Rating: 0.4},
		{Name: "high", Rating: 2.0, Spoiler: 2},
		{Name: " ", Rating: 3.0},
	}, -1)

	if len(tags) != 2 {
		t.Fatalf("expected low and high VNDB tags to be kept, got %#v", tags)
	}
	if tags[0].Name != "high" || !tags[0].IsSpoiler {
		t.Fatalf("expected high rated spoiler tag first, got %#v", tags[0])
	}
	if tags[1].Name != "low" {
		t.Fatalf("expected low rated tag to be kept, got %#v", tags[1])
	}
}

func TestExtractBangumiTagsKeepsLowCountTags(t *testing.T) {
	tags := extractBangumiTags([]bangumiTag{
		{Name: "low", Count: 1},
		{Name: "high", Count: 8},
		{Name: " ", Count: 20},
	}, -1)

	if len(tags) != 2 {
		t.Fatalf("expected low and high Bangumi tags to be kept, got %#v", tags)
	}
	if tags[0].Name != "high" {
		t.Fatalf("expected high count tag first, got %#v", tags[0])
	}
	if tags[1].Name != "low" {
		t.Fatalf("expected low count tag to be kept, got %#v", tags[1])
	}
}
