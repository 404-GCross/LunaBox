//go:build darwin

package launcher

import (
	"lunabox/backend/utils/processutils"
	"testing"
)

func TestDarwinSteamSamplesMarkSavedProcessAndSkipHelpers(t *testing.T) {
	candidates := []processutils.ProcessInfo{
		{Name: "Game Helper", PID: 10},
		{Name: "FallbackGame", PID: 20},
		{Name: "SavedGame", PID: 30},
	}

	samples := darwinSteamSamples(candidates, "savedgame", 1, 20, true)
	if len(samples) != 2 {
		t.Fatalf("expected helper to be skipped, got %+v", samples)
	}
	saved, ok := savedNameCandidate(samples)
	if !ok || saved.Process.PID != 30 {
		t.Fatalf("expected saved process, got %+v, %v", saved, ok)
	}
	for _, sample := range samples {
		if sample.Relation != relationDescendantInDir || sample.WindowKnown {
			t.Fatalf("expected install dir as primary link without window channel, got %+v", sample)
		}
		if sample.Foreground != (sample.Process.PID == 20) {
			t.Fatalf("expected only the frontmost app to be foreground, got %+v", sample)
		}
	}
}

func TestDarwinSteamSamplesRejectOnlyHelpers(t *testing.T) {
	candidates := []processutils.ProcessInfo{
		{Name: "steam_osx", PID: 10},
		{Name: "Game Helper", PID: 20},
	}
	if samples := darwinSteamSamples(candidates, "", 1, 0, false); len(samples) != 0 {
		t.Fatalf("expected no game process, got %+v", samples)
	}
}
