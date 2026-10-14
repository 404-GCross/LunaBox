//go:build linux

package launcher

import (
	"lunabox/internal/utils/processutils"
	"strings"
	"time"
)

const (
	linuxExitWatchCheckInterval = 2 * time.Second
	linuxExitWatchStartupGrace  = 2 * time.Minute
	linuxExitWatchMissingGrace  = 8 * time.Second
)

// StartExitWatch monitors the Linux process group associated with a game
// session and signals when no tracked game process remains.
func StartExitWatch(input ExitWatchInput, logger DetectionLogger) (<-chan struct{}, bool) {
	if input.Config.Mode != ExitWatchGameProcessPresence {
		return nil, false
	}

	result := make(chan struct{})
	go runLinuxExitWatch(input, logger, result)
	return result, true
}

func runLinuxExitWatch(input ExitWatchInput, logger DetectionLogger, result chan<- struct{}) {
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
	processTracker := processutils.NewLinuxProcessTracker(input.RootPID)
	if snapshot, err := processutils.CaptureLinuxProcessSnapshot(); err == nil {
		processTracker.Observe(snapshot)
	}

	for {
		select {
		case <-input.Done:
			return
		case <-ticker.C:
			snapshot, err := processutils.CaptureLinuxProcessSnapshot()
			if err != nil {
				continue
			}
			processes := linuxExitWatchGameProcesses(input.RootPID, input.Config, snapshot, processTracker)
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

			logInfo(
				logger,
				"Linux exit watch detected no game process for %s (PID %d) under %s; ending session %s",
				input.ProcessName,
				input.RootPID,
				input.Config.DetectionDir,
				input.SessionID,
			)
			triggered = true
			return
		}
	}
}

func linuxExitWatchGameProcesses(rootPID uint32, config ExitWatch, snapshot *processutils.LinuxProcessSnapshot, processTracker *processutils.LinuxProcessTracker) []processutils.ProcessInfo {
	seen := make(map[uint32]bool)
	processes := make([]processutils.ProcessInfo, 0)

	add := func(proc processutils.ProcessInfo) bool {
		if proc.PID == 0 || seen[proc.PID] {
			return false
		}
		if config.IgnoreRootProcess && proc.PID == rootPID {
			return false
		}
		if IsLikelyHelperProcess(proc.Name) || !snapshot.ContainsPID(proc.PID) {
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

	if strings.TrimSpace(config.DetectionDir) != "" {
		if dirProcesses, err := snapshot.ProcessesByExecutableDir(config.DetectionDir); err == nil {
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
