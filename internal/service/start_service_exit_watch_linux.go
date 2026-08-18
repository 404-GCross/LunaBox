//go:build linux

package service

import (
	"lunabox/internal/applog"
	launcherpkg "lunabox/internal/service/launcher"
	"lunabox/internal/utils/processutils"
	"strings"
	"time"
)

const (
	linuxExitWatchCheckInterval = 2 * time.Second
	linuxExitWatchStartupGrace  = 2 * time.Minute
	linuxExitWatchMissingGrace  = 8 * time.Second
)

func (s *StartService) startExitWatch(session *activePlaySession, processID uint32, processName string, exitWatch launcherpkg.ExitWatch) (<-chan struct{}, bool) {
	if exitWatch.Mode != launcherpkg.ExitWatchGameProcessPresence {
		return nil, false
	}

	result := make(chan struct{})
	go s.runLinuxExitWatch(session, processID, processName, exitWatch, result)
	return result, true
}

func (s *StartService) runLinuxExitWatch(session *activePlaySession, processID uint32, processName string, exitWatch launcherpkg.ExitWatch, result chan<- struct{}) {
	triggered := false
	defer func() {
		if triggered {
			close(result)
		}
	}()

	ticker := time.NewTicker(linuxExitWatchCheckInterval)
	defer ticker.Stop()

	startedAt := time.Now()
	var missingSince time.Time
	observedGameProcess := false
	processTracker := processutils.NewLinuxProcessTracker(processID)
	if snapshot, err := processutils.CaptureLinuxProcessSnapshot(); err == nil {
		processTracker.Observe(snapshot)
	}

	for {
		select {
		case <-session.done:
			return
		case <-ticker.C:
			snapshot, err := processutils.CaptureLinuxProcessSnapshot()
			if err != nil {
				continue
			}
			processes := s.linuxExitWatchGameProcesses(processID, exitWatch, snapshot, processTracker)
			if len(processes) > 0 {
				observedGameProcess = true
				missingSince = time.Time{}
				continue
			}

			if !processTracker.RootPresent(snapshot) {
				triggered = true
				return
			}

			if !observedGameProcess && time.Since(startedAt) < linuxExitWatchStartupGrace {
				continue
			}
			if missingSince.IsZero() {
				missingSince = time.Now()
				continue
			}
			if time.Since(missingSince) < linuxExitWatchMissingGrace {
				continue
			}

			applog.LogInfof(
				s.ctx,
				"Linux exit watch detected no game process for %s (PID %d) under %s; ending session %s",
				processName,
				processID,
				exitWatch.DetectionDir,
				session.sessionID,
			)
			triggered = true
			return
		}
	}
}

func (s *StartService) linuxExitWatchGameProcesses(rootPID uint32, exitWatch launcherpkg.ExitWatch, snapshot *processutils.LinuxProcessSnapshot, processTracker *processutils.LinuxProcessTracker) []processutils.ProcessInfo {
	seen := make(map[uint32]bool)
	processes := make([]processutils.ProcessInfo, 0)

	add := func(proc processutils.ProcessInfo) bool {
		if proc.PID == 0 || seen[proc.PID] {
			return false
		}
		if exitWatch.IgnoreRootProcess && proc.PID == rootPID {
			return false
		}
		if launcherpkg.IsLikelyHelperProcess(proc.Name) || !snapshot.ContainsPID(proc.PID) {
			return false
		}
		seen[proc.PID] = true
		processes = append(processes, proc)
		return true
	}

	for _, proc := range processTracker.Observe(snapshot) {
		if proc.PID == rootPID {
			continue
		}
		add(proc)
	}

	if strings.TrimSpace(exitWatch.DetectionDir) != "" {
		if dirProcesses, err := snapshot.ProcessesByExecutableDir(exitWatch.DetectionDir); err == nil {
			accepted := make([]processutils.ProcessInfo, 0, len(dirProcesses))
			for _, proc := range dirProcesses {
				if add(proc) {
					accepted = append(accepted, proc)
				}
			}
			processTracker.Remember(snapshot, accepted)
		}
	}

	return processes
}
