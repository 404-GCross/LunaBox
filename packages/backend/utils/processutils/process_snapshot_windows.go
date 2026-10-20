//go:build windows

package processutils

import (
	"fmt"
	"path/filepath"
	"strings"
)

// WindowsProcessSnapshot captures the process table once so a detection tick
// can answer descendant, directory and name queries without re-enumerating.
type WindowsProcessSnapshot struct {
	entries  []processSnapshotEntry
	byPID    map[uint32]processSnapshotEntry
	children map[uint32][]processSnapshotEntry
}

// ProcessImagePathCache remembers image paths across snapshots. Opening every
// process to read its path is the most expensive part of a detection tick, and
// image paths never change for a live process.
type ProcessImagePathCache struct {
	entries map[uint32]cachedImagePath
}

type cachedImagePath struct {
	name      string
	parentPID uint32
	path      string
	ok        bool
}

func NewProcessImagePathCache() *ProcessImagePathCache {
	return &ProcessImagePathCache{entries: make(map[uint32]cachedImagePath)}
}

// CaptureWindowsProcessSnapshot takes one Toolhelp process snapshot and indexes it.
func CaptureWindowsProcessSnapshot() (*WindowsProcessSnapshot, error) {
	entries, err := getProcessSnapshotEntries()
	if err != nil {
		return nil, err
	}
	snapshot := &WindowsProcessSnapshot{
		entries:  entries,
		byPID:    make(map[uint32]processSnapshotEntry, len(entries)),
		children: make(map[uint32][]processSnapshotEntry),
	}
	for _, entry := range entries {
		snapshot.byPID[entry.PID] = entry
		snapshot.children[entry.ParentPID] = append(snapshot.children[entry.ParentPID], entry)
	}
	return snapshot, nil
}

func (s *WindowsProcessSnapshot) Process(pid uint32) (ProcessInfo, bool) {
	if s == nil || pid == 0 {
		return ProcessInfo{}, false
	}
	entry, ok := s.byPID[pid]
	if !ok {
		return ProcessInfo{}, false
	}
	return ProcessInfo{Name: entry.Name, PID: entry.PID}, true
}

// DescendantProcesses walks the parent links recorded in the snapshot. Orphans
// keep the parent PID of an exited launcher, so its children stay discoverable.
func (s *WindowsProcessSnapshot) DescendantProcesses(parentPID uint32) []ProcessInfo {
	if s == nil || parentPID == 0 {
		return nil
	}
	seen := map[uint32]bool{parentPID: true}
	queue := []uint32{parentPID}
	descendants := make([]ProcessInfo, 0)
	for len(queue) > 0 {
		currentPID := queue[0]
		queue = queue[1:]
		for _, child := range s.children[currentPID] {
			if seen[child.PID] {
				continue
			}
			seen[child.PID] = true
			queue = append(queue, child.PID)
			descendants = append(descendants, ProcessInfo{Name: child.Name, PID: child.PID})
		}
	}
	return descendants
}

func (s *WindowsProcessSnapshot) ProcessesByName(processName string) []ProcessInfo {
	if s == nil {
		return nil
	}
	targetName := strings.TrimSpace(processName)
	if targetName == "" {
		return nil
	}
	matches := make([]ProcessInfo, 0, 1)
	for _, entry := range s.entries {
		if strings.EqualFold(entry.Name, targetName) {
			matches = append(matches, ProcessInfo{Name: entry.Name, PID: entry.PID})
		}
	}
	return matches
}

// ProcessesByExecutableDir returns processes whose image lives under rootDir.
// cache may be nil; when provided it is pruned to the processes in this snapshot.
func (s *WindowsProcessSnapshot) ProcessesByExecutableDir(rootDir string, cache *ProcessImagePathCache) ([]ProcessInfo, error) {
	if s == nil {
		return nil, nil
	}
	normalizedRoot, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootDir)))
	if err != nil {
		return nil, fmt.Errorf("normalize executable dir: %w", err)
	}

	var retained map[uint32]cachedImagePath
	if cache != nil {
		retained = make(map[uint32]cachedImagePath, len(s.entries))
	}

	processes := make([]ProcessInfo, 0)
	for _, entry := range s.entries {
		var imagePath string
		var ok bool
		if cache != nil {
			cached, hit := cache.entries[entry.PID]
			if !hit || cached.name != entry.Name || cached.parentPID != entry.ParentPID {
				imagePath, ok = queryProcessImagePath(entry.PID)
				cached = cachedImagePath{name: entry.Name, parentPID: entry.ParentPID, path: imagePath, ok: ok}
			}
			retained[entry.PID] = cached
			imagePath, ok = cached.path, cached.ok
		} else {
			imagePath, ok = queryProcessImagePath(entry.PID)
		}
		if !ok || !isPathUnderDir(imagePath, normalizedRoot) {
			continue
		}
		processes = append(processes, ProcessInfo{Name: entry.Name, PID: entry.PID})
	}
	if cache != nil {
		cache.entries = retained
	}
	return processes, nil
}
