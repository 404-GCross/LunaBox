package timerutils

import (
	"context"
	"lunabox/internal/utils/processutils"
	"testing"
)

func TestActiveTrackFocusByBundlePath(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundBundlePath = func() (string, bool) {
		return "/Applications/Game.app", true
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 99,
		ActiveTrack: ActiveTrack{
			Kind:       ActiveTrackBundlePath,
			BundlePath: "/Applications/Game.app",
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatalf("expected bundle path session to be focused")
	}
}

func TestActiveTrackFocusByWineRootDescendant(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 200, true
	}
	getDescendantProcesses = func(parentPID uint32) ([]processutils.ProcessInfo, error) {
		if parentPID != 100 {
			t.Fatalf("expected root pid 100, got %d", parentPID)
		}
		return []processutils.ProcessInfo{{PID: 200, Name: "wine64-preloader"}}, nil
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 100,
		ActiveTrack: ActiveTrack{
			Kind:    ActiveTrackWineRootPID,
			RootPID: 100,
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatalf("expected wine descendant session to be focused")
	}
}

func TestActiveTrackFocusByLauncherPID(t *testing.T) {
	restore := stubFocusFunctions(t)
	defer restore()
	getForegroundProcessID = func() (uint32, bool) {
		return 300, true
	}

	tracker := NewActiveTimeTracker(context.Background(), nil)
	session := &TrackingSession{
		ProcessID: 300,
		ActiveTrack: ActiveTrack{
			Kind: ActiveTrackLauncherPID,
		},
	}

	if !tracker.isSessionFocused(session) {
		t.Fatalf("expected launcher pid session to be focused")
	}
}

func stubFocusFunctions(t *testing.T) func() {
	t.Helper()
	origBundle := getForegroundBundlePath
	origPID := getForegroundProcessID
	origDescendants := getDescendantProcesses
	origFocused := isProcessFocused

	getForegroundBundlePath = func() (string, bool) { return "", false }
	getForegroundProcessID = func() (uint32, bool) { return 0, false }
	getDescendantProcesses = func(parentPID uint32) ([]processutils.ProcessInfo, error) { return nil, nil }
	isProcessFocused = func(processID uint32) bool { return false }

	return func() {
		getForegroundBundlePath = origBundle
		getForegroundProcessID = origPID
		getDescendantProcesses = origDescendants
		isProcessFocused = origFocused
	}
}
