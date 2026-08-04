package launcher

import "testing"

func TestWineAndProtonWrappersAreHelperProcesses(t *testing.T) {
	for _, name := range []string{
		"proton",
		"pressure-vessel",
		"pv-bwrap",
		"pv-adverb",
		"reaper",
		"srt-bwrap",
		"gameoverlayui",
		"wine",
		"wine-preloader",
		"wine64",
		"wineserver",
		"explorer.exe",
		"services.exe",
		"winedevice.exe",
		"xalia.exe",
	} {
		if !IsLikelyHelperProcess(name) {
			t.Fatalf("expected %s to be treated as helper process", name)
		}
		if IsPersistableProcessName(name) {
			t.Fatalf("expected %s not to be persisted as game process", name)
		}
	}
}
