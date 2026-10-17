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

func TestExtractVNDBTagsFiltersLieBeforeLimit(t *testing.T) {
	tags := extractVNDBTags([]vndbTag{
		{Name: "lie", Rating: 3.0, Lie: true},
		{Name: "high", Rating: 2.0},
		{Name: "low", Rating: 1.0},
	}, 2)

	if len(tags) != 2 {
		t.Fatalf("expected lie tag to be filtered before limit, got %#v", tags)
	}
	if tags[0].Name != "high" || tags[1].Name != "low" {
		t.Fatalf("expected high and low tags after filtering lie tag, got %#v", tags)
	}
}

func TestConvertVNDBResultMapsCoverSexualRatingToNSFW(t *testing.T) {
	getter := NewVNDBInfoGetter()

	sfw := getter.convertResultToGame(vndbQueryResult{
		ID:    "v1",
		Title: "SFW",
		Image: vndbImage{Sexual: vndbNSFWCoverThreshold - 0.01},
	})
	if sfw.IsNSFW {
		t.Fatal("expected VNDB cover below the threshold to remain SFW")
	}

	nsfw := getter.convertResultToGame(vndbQueryResult{
		ID:    "v2",
		Title: "NSFW",
		Image: vndbImage{Sexual: vndbNSFWCoverThreshold},
	})
	if !nsfw.IsNSFW {
		t.Fatal("expected VNDB cover at the threshold to be NSFW")
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
