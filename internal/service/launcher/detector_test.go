package launcher

import (
	"testing"
	"time"
)

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

func TestGenericUnixWrapperProcessNamesAreNotPersistable(t *testing.T) {
	for _, name := range []string{"bash", "sh", "python", "python3", "python3.12"} {
		if IsPersistableProcessName(name) {
			t.Fatalf("expected %s not to be persisted as game process", name)
		}
	}
}

func TestProcessDetectionDeadlineUsesProvidedDeadline(t *testing.T) {
	want := time.Now().Add(3 * time.Minute)
	got := processDetectionDeadline(StagedProcessDetectionInput{DetectionDeadline: want})
	if !got.Equal(want) {
		t.Fatalf("expected deadline %v, got %v", want, got)
	}
}

func TestWaitForProcessDetectionStopsWhenSessionEnds(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if waitForProcessDetection(time.Now().Add(time.Minute), time.Minute, done) {
		t.Fatal("expected process detection wait to stop for an ended session")
	}
}

func TestStartExitWatchReturnsDisabledForDefaultConfiguration(t *testing.T) {
	exitChan, ok := StartExitWatch(ExitWatchInput{}, nil)
	if ok || exitChan != nil {
		t.Fatalf("expected disabled exit watch, got channel=%v enabled=%v", exitChan, ok)
	}
}
