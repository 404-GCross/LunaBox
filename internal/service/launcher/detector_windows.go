//go:build windows

package launcher

import (
	"lunabox/internal/utils/processutils"
	"lunabox/internal/utils/timerutils/focusing"
	"strings"
	"time"
)

// DetectStagedProcess resolves the actual game process behind a Windows launcher.
// The timing mirrors the existing Windows launch flow: a 5s initial wait and
// a 15s observation window give launchers time to hand off to the real game.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	launcher := input.Launcher

	if HasReliableSavedProcessName(input.SavedProcessName, input.LauncherExeName) {
		logInfo(logger, "Game %s has saved process_name: %s, will search for it after initial delay", input.GameID, input.SavedProcessName)

		time.Sleep(5 * time.Second)

		pid, err := processutils.GetProcessPIDByName(input.SavedProcessName)
		if err != nil {
			logWarning(logger, "Failed to find saved process %s: %v, falling back to launcher monitoring", input.SavedProcessName, err)
			if !processutils.IsProcessPresentByPID(launcher.PID) {
				if detected, ok := detectLaunchedGameProcess(input, logger); ok {
					return resultForExternalProcess(input, detected, true)
				}
				return promptProcessSelectionResult()
			}
			return resultForLauncher(input)
		}

		logInfo(logger, "Found saved process %s with PID %d", input.SavedProcessName, pid)
		return StagedProcessDetectionResult{
			ProcessID:           pid,
			ProcessName:         input.SavedProcessName,
			CloseLauncherHandle: true,
		}
	}

	if !input.AutoDetectGameProcess {
		logInfo(logger, "Auto-detect disabled for game %s, using launcher process: %s (PID %d)", input.GameID, launcher.Name, launcher.PID)
		return resultForLauncher(input)
	}

	logInfo(logger, "Starting staged detection for game %s, launcher: %s (PID %d)", input.GameID, launcher.Name, launcher.PID)

	time.Sleep(5 * time.Second)

	if detected, ok := detectVisibleGameProcess(input, logger); ok {
		return resultForExternalProcess(input, detected, true)
	}

	if !processutils.IsProcessPresentByPID(launcher.PID) {
		logInfo(logger, "Launcher %s exited quickly (within 5s), resolving actual game process", input.LauncherExeName)
		if detected, ok := detectLaunchedGameProcess(input, logger); ok {
			return resultForExternalProcess(input, detected, true)
		}
		return promptProcessSelectionResult()
	}

	logInfo(logger, "Launcher %s still running, entering observation period (15s)", input.LauncherExeName)

	observationPeriod := 15 * time.Second
	checkInterval := 2 * time.Second
	observationStart := time.Now()

	for time.Since(observationStart) < observationPeriod {
		time.Sleep(checkInterval)

		if detected, ok := detectVisibleGameProcess(input, logger); ok {
			return resultForExternalProcess(input, detected, true)
		}

		if !processutils.IsProcessPresentByPID(launcher.PID) {
			logInfo(logger, "Launcher %s exited during observation period, resolving actual game process", input.LauncherExeName)
			if detected, ok := detectLaunchedGameProcess(input, logger); ok {
				return resultForExternalProcess(input, detected, true)
			}
			return promptProcessSelectionResult()
		}
	}

	logInfo(logger, "Launcher %s still running after 20s total, treating it as the game process", input.LauncherExeName)
	return resultForLauncher(input)
}

func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	logInfo(logger, "Starting Steam directory detection for game %s, install dir: %s", input.GameID, input.LaunchDir)

	time.Sleep(5 * time.Second)

	if HasReliableSavedProcessName(input.SavedProcessName, "steam.exe") {
		pid, err := processutils.GetProcessPIDByName(input.SavedProcessName)
		if err == nil {
			logInfo(logger, "Found saved Steam game process %s with PID %d", input.SavedProcessName, pid)
			return StagedProcessDetectionResult{
				ProcessID:           pid,
				ProcessName:         input.SavedProcessName,
				CloseLauncherHandle: true,
			}
		}
		logWarning(logger, "Failed to find saved Steam game process %s: %v", input.SavedProcessName, err)
	}

	observationPeriod := 30 * time.Second
	checkInterval := 2 * time.Second
	observationStart := time.Now()
	for time.Since(observationStart) < observationPeriod {
		if detected, ok := detectVisibleProcessInSteamDir(input, logger); ok {
			return resultForSteamProcess(input, detected)
		}
		time.Sleep(checkInterval)
	}

	if detected, ok := detectSingleStableProcessInSteamDir(input, logger); ok {
		return resultForSteamProcess(input, detected)
	}

	logWarning(logger, "Steam directory detection failed for game %s, requiring manual process selection", input.GameID)
	return promptProcessSelectionResult()
}

func resultForSteamProcess(input StagedProcessDetectionInput, proc processutils.ProcessInfo) StagedProcessDetectionResult {
	return StagedProcessDetectionResult{
		ProcessID:           proc.PID,
		ProcessName:         proc.Name,
		CloseLauncherHandle: true,
		PersistProcessName:  ProcessNameForPersistence("", proc.Name),
	}
}

func detectLaunchedGameProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	if proc, ok := detectLaunchedDescendantProcess(input, logger); ok {
		return proc, true
	}
	return detectProcessInLaunchDir(input, logger)
}

func detectVisibleGameProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	if proc, ok := detectVisibleLaunchedDescendantProcess(input, logger); ok && proc.PID != input.Launcher.PID {
		logInfo(logger, "Detected visible game window process for game %s: %s (PID %d), launcher: %s (PID %d)", input.GameID, proc.Name, proc.PID, input.LauncherExeName, input.Launcher.PID)
		return proc, true
	}
	if proc, ok := detectVisibleProcessInLaunchDir(input, logger); ok && proc.PID != input.Launcher.PID {
		logInfo(logger, "Detected visible game window process for game %s: %s (PID %d), launcher: %s (PID %d)", input.GameID, proc.Name, proc.PID, input.LauncherExeName, input.Launcher.PID)
		return proc, true
	}
	return processutils.ProcessInfo{}, false
}

func detectVisibleLaunchedDescendantProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	descendants, err := processutils.GetDescendantProcesses(input.Launcher.PID)
	if err != nil {
		logWarning(logger, "Failed to enumerate visible descendant processes for launcher %s (PID %d): %v", input.LauncherExeName, input.Launcher.PID, err)
		return processutils.ProcessInfo{}, false
	}
	if len(descendants) == 0 {
		return processutils.ProcessInfo{}, false
	}

	candidates := make([]processutils.ProcessInfo, 0, len(descendants))
	for _, proc := range descendants {
		if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		candidates = append(candidates, proc)
	}
	if len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range windowCandidates {
			if proc.PID == foregroundPID {
				logInfo(logger, "Auto-detected foreground visible-window descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Auto-detected visible-window descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	logInfo(logger, "Multiple visible-window descendant candidates found for game %s, requiring more evidence: %s", input.GameID, FormatProcessCandidates(windowCandidates))
	return processutils.ProcessInfo{}, false
}

func detectLaunchedDescendantProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	descendants, err := processutils.GetDescendantProcesses(input.Launcher.PID)
	if err != nil {
		logWarning(logger, "Failed to enumerate descendant processes for launcher %s (PID %d): %v", input.LauncherExeName, input.Launcher.PID, err)
		return processutils.ProcessInfo{}, false
	}
	if len(descendants) == 0 {
		logInfo(logger, "No descendant process found for launcher %s (PID %d)", input.LauncherExeName, input.Launcher.PID)
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range descendants {
			if proc.PID == foregroundPID && !IsLikelyHelperProcess(proc.Name) {
				logInfo(logger, "Auto-detected foreground descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	candidates := make([]processutils.ProcessInfo, 0, len(descendants))
	for _, proc := range descendants {
		if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		candidates = append(candidates, proc)
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Auto-detected visible-window descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	if len(candidates) == 1 {
		proc := candidates[0]
		if IsPersistableProcessName(proc.Name) {
			logInfo(logger, "Auto-detected stable descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
			return proc, true
		}
		logInfo(logger, "Single non-exe descendant process found for game %s without visible window, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
		return processutils.ProcessInfo{}, false
	}

	if len(candidates) > 1 {
		logInfo(logger, "Multiple descendant process candidates found for game %s, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
	} else {
		logInfo(logger, "Only helper descendant processes found for game %s, requiring manual selection: %s", input.GameID, FormatProcessCandidates(descendants))
	}
	return processutils.ProcessInfo{}, false
}

func detectVisibleProcessInLaunchDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := launchDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range windowCandidates {
			if proc.PID == foregroundPID && proc.PID != input.Launcher.PID {
				logInfo(logger, "Auto-detected foreground visible-window process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	nonLauncherWindowCandidates := make([]processutils.ProcessInfo, 0, len(windowCandidates))
	for _, proc := range windowCandidates {
		if proc.PID == input.Launcher.PID {
			continue
		}
		nonLauncherWindowCandidates = append(nonLauncherWindowCandidates, proc)
	}

	if len(nonLauncherWindowCandidates) == 1 {
		proc := nonLauncherWindowCandidates[0]
		logInfo(logger, "Auto-detected visible-window process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	if len(nonLauncherWindowCandidates) > 1 {
		logInfo(logger, "Multiple visible-window launch dir candidates found for game %s, requiring more evidence: %s", input.GameID, FormatProcessCandidates(nonLauncherWindowCandidates))
		return processutils.ProcessInfo{}, false
	}

	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Only launcher has a visible window for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	return processutils.ProcessInfo{}, false
}

func detectVisibleProcessInSteamDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := steamDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range windowCandidates {
			if proc.PID == foregroundPID {
				logInfo(logger, "Auto-detected foreground Steam game process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Auto-detected visible Steam game process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	logInfo(logger, "Multiple visible Steam game candidates found for game %s: %s", input.GameID, FormatProcessCandidates(windowCandidates))
	return processutils.ProcessInfo{}, false
}

func detectSingleStableProcessInSteamDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := steamDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}
	if len(candidates) == 1 && IsPersistableProcessName(candidates[0].Name) {
		proc := candidates[0]
		logInfo(logger, "Auto-detected single stable Steam game process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}
	logInfo(logger, "Steam game process candidates require manual selection for game %s: %s", input.GameID, FormatProcessCandidates(candidates))
	return processutils.ProcessInfo{}, false
}

func detectProcessInLaunchDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := launchDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range candidates {
			if proc.PID == foregroundPID && !IsLikelyHelperProcess(proc.Name) {
				logInfo(logger, "Auto-detected foreground process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Auto-detected visible-window process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	nonLauncherWindowCandidates := make([]processutils.ProcessInfo, 0, len(windowCandidates))
	for _, proc := range windowCandidates {
		if proc.PID == input.Launcher.PID {
			continue
		}
		nonLauncherWindowCandidates = append(nonLauncherWindowCandidates, proc)
	}
	if len(nonLauncherWindowCandidates) == 1 {
		proc := nonLauncherWindowCandidates[0]
		logInfo(logger, "Auto-detected non-launcher visible-window process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	if len(candidates) == 1 {
		proc := candidates[0]
		if IsPersistableProcessName(proc.Name) {
			logInfo(logger, "Auto-detected stable process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
			return proc, true
		}
		logInfo(logger, "Single non-exe launch dir process found for game %s without visible window, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
		return processutils.ProcessInfo{}, false
	}

	if len(candidates) > 1 {
		logInfo(logger, "Multiple launch dir process candidates found for game %s, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
	}
	return processutils.ProcessInfo{}, false
}

func launchDirProcessCandidates(input StagedProcessDetectionInput, logger DetectionLogger) ([]processutils.ProcessInfo, error) {
	candidates, err := processutils.GetProcessesByExecutableDir(input.LaunchDir)
	if err != nil {
		logWarning(logger, "Failed to enumerate processes in launch dir %s for game %s: %v", input.LaunchDir, input.GameID, err)
		return nil, err
	}
	if len(candidates) == 0 {
		logInfo(logger, "No running process found in launch dir %s for game %s", input.LaunchDir, input.GameID)
		return nil, nil
	}

	filtered := make([]processutils.ProcessInfo, 0, len(candidates))
	for _, proc := range candidates {
		if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		filtered = append(filtered, proc)
	}

	if len(filtered) == 0 {
		logInfo(logger, "Only helper processes found in launch dir for game %s, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
	}
	return filtered, nil
}

func steamDirProcessCandidates(input StagedProcessDetectionInput, logger DetectionLogger) ([]processutils.ProcessInfo, error) {
	candidates, err := processutils.GetProcessesByExecutableDir(input.LaunchDir)
	if err != nil {
		logWarning(logger, "Failed to enumerate Steam game processes in %s for game %s: %v", input.LaunchDir, input.GameID, err)
		return nil, err
	}

	filtered := make([]processutils.ProcessInfo, 0, len(candidates))
	for _, proc := range candidates {
		if strings.EqualFold(proc.Name, "steam.exe") || IsLikelyHelperProcess(proc.Name) {
			continue
		}
		filtered = append(filtered, proc)
	}
	if len(filtered) == 0 {
		logInfo(logger, "No non-Steam game process found in install dir %s for game %s", input.LaunchDir, input.GameID)
	}
	return filtered, nil
}
