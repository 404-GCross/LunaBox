//go:build linux

package focusing

import (
	"lunabox/internal/utils/processutils"
	"sync"
	"time"
)

// WindowFocusInfo 窗口焦点信息。
type WindowFocusInfo struct {
	HWnd      uintptr
	ProcessID uint32
	IsFocused bool
}

// FocusTracker provides a conservative Linux fallback. Without a reliable
// desktop-agnostic foreground-window API, LunaBox treats the tracked process as
// active while it is alive.
type FocusTracker struct {
	mu           sync.Mutex
	targetPID    uint32
	isFocused    bool
	callbackChan chan WindowFocusInfo
	running      bool
	stopChan     chan struct{}
}

func NewFocusTracker(targetPID uint32) *FocusTracker {
	return &FocusTracker{
		targetPID:    targetPID,
		callbackChan: make(chan WindowFocusInfo, 10),
		stopChan:     make(chan struct{}),
	}
}

func (ft *FocusTracker) Start() (<-chan WindowFocusInfo, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if ft.running {
		return ft.callbackChan, nil
	}

	ft.running = true
	ft.isFocused = ft.isCurrentlyFocused()
	go ft.checkLoop()

	return ft.callbackChan, nil
}

func (ft *FocusTracker) checkLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentlyFocused := ft.isCurrentlyFocused()

			ft.mu.Lock()
			wasFocused := ft.isFocused
			ft.isFocused = currentlyFocused
			ft.mu.Unlock()

			if currentlyFocused != wasFocused {
				info := WindowFocusInfo{
					ProcessID: ft.targetPID,
					IsFocused: currentlyFocused,
				}
				select {
				case ft.callbackChan <- info:
				default:
				}
			}
		case <-ft.stopChan:
			return
		}
	}
}

func (ft *FocusTracker) Stop() {
	ft.mu.Lock()
	if !ft.running {
		ft.mu.Unlock()
		return
	}
	ft.running = false
	stopChan := ft.stopChan
	callbackChan := ft.callbackChan
	ft.mu.Unlock()

	select {
	case <-stopChan:
	default:
		close(stopChan)
	}

	select {
	case <-callbackChan:
	default:
		close(callbackChan)
	}
}

func (ft *FocusTracker) IsFocused() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.isFocused
}

func (ft *FocusTracker) isCurrentlyFocused() bool {
	return processutils.IsProcessPresentByPID(ft.targetPID)
}

func GetForegroundProcessID() (uint32, bool) {
	return 0, false
}

func GetForegroundBundlePath() (string, bool) {
	return "", false
}

func IsBundlePathFocused(bundlePath string) bool {
	return false
}

func IsProcessFocused(processID uint32) bool {
	return processutils.IsProcessPresentByPID(processID)
}
