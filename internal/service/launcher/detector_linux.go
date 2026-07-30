//go:build linux

package launcher

import (
	"lunabox/internal/utils/processutils"
	"strings"
	"time"
)

func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	launcher := input.Launcher

	if HasReliableSavedProcessName(input.SavedProcessName, input.LauncherExeName) {
		logInfo(logger, "Game %s has saved process_name: %s, will search for it after initial delay", input.GameID, input.SavedProcessName)
		time.Sleep(5 * time.Second)

		pid, err := processutils.GetProcessPIDByName(input.SavedProcessName)
		if err == nil {
			logInfo(logger, "Found saved process %s with PID %d", input.SavedProcessName, pid)
			return StagedProcessDetectionResult{
				ProcessID:           pid,
				ProcessName:         input.SavedProcessName,
				CloseLauncherHandle: true,
			}
		}

		logWarning(logger, "Failed to find saved process %s: %v, falling back to launcher monitoring", input.SavedProcessName, err)
		if !processutils.IsProcessPresentByPID(launcher.PID) {
			if detected, ok := detectLaunchedGameProcessWithRetry(input, logger); ok {
				return resultForExternalProcess(input, detected, true)
			}
			return promptProcessSelectionResult()
		}
		return resultForLauncher(input)
	}

	if !input.AutoDetectGameProcess {
		logInfo(logger, "Auto-detect disabled for game %s, using launcher process: %s (PID %d)", input.GameID, launcher.Name, launcher.PID)
		return resultForLauncher(input)
	}

	logInfo(logger, "Starting Linux staged detection for game %s, launcher: %s (PID %d)", input.GameID, launcher.Name, launcher.PID)
	time.Sleep(5 * time.Second)

	if detected, ok := detectLaunchedGameProcess(input, logger); ok && detected.PID != launcher.PID {
		return resultForExternalProcess(input, detected, true)
	}

	if !processutils.IsProcessPresentByPID(launcher.PID) {
		if detected, ok := detectLaunchedGameProcessWithRetry(input, logger); ok {
			return resultForExternalProcess(input, detected, true)
		}
		return promptProcessSelectionResult()
	}

	observationPeriod := 15 * time.Second
	checkInterval := 2 * time.Second
	observationStart := time.Now()
	for time.Since(observationStart) < observationPeriod {
		time.Sleep(checkInterval)
		if detected, ok := detectLaunchedGameProcess(input, logger); ok && detected.PID != launcher.PID {
			return resultForExternalProcess(input, detected, true)
		}
		if !processutils.IsProcessPresentByPID(launcher.PID) {
			if detected, ok := detectLaunchedGameProcessWithRetry(input, logger); ok {
				return resultForExternalProcess(input, detected, true)
			}
			return promptProcessSelectionResult()
		}
	}

	return resultForLauncher(input)
}

func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	logInfo(logger, "Starting Linux Steam directory detection for game %s, install dir: %s", input.GameID, input.LaunchDir)
	time.Sleep(5 * time.Second)

	if HasReliableSavedProcessName(input.SavedProcessName, "steam") {
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
		if detected, ok := detectProcessInSteamDir(input, logger); ok {
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

func detectLaunchedGameProcessWithRetry(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	const attempts = 3
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(1 * time.Second)
		}
		if proc, ok := detectLaunchedGameProcess(input, logger); ok {
			return proc, true
		}
	}
	return processutils.ProcessInfo{}, false
}

func detectLaunchedGameProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	if proc, ok := detectLaunchedDescendantProcess(input, logger); ok {
		return proc, true
	}
	return detectProcessInLaunchDir(input, logger)
}

func detectLaunchedDescendantProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	descendants, err := processutils.GetDescendantProcesses(input.Launcher.PID)
	if err != nil {
		logWarning(logger, "Failed to enumerate descendant processes for launcher %s (PID %d): %v", input.LauncherExeName, input.Launcher.PID, err)
		return processutils.ProcessInfo{}, false
	}

	candidates := make([]processutils.ProcessInfo, 0, len(descendants))
	for _, proc := range descendants {
		if proc.PID == input.Launcher.PID || IsLikelyHelperProcess(proc.Name) {
			continue
		}
		candidates = append(candidates, proc)
	}
	return pickStableCandidate(candidates, input.GameID, "descendant", logger)
}

func detectProcessInLaunchDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := launchDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}
	return pickStableCandidate(candidates, input.GameID, "launch dir", logger)
}

func detectProcessInSteamDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := steamDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}
	return pickStableCandidate(candidates, input.GameID, "Steam game", logger)
}

func detectSingleStableProcessInSteamDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := steamDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}
	return pickStableCandidate(candidates, input.GameID, "Steam game", logger)
}

func pickStableCandidate(candidates []processutils.ProcessInfo, gameID string, source string, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	if len(candidates) == 1 && IsPersistableProcessName(candidates[0].Name) {
		proc := candidates[0]
		logInfo(logger, "Auto-detected %s process for game %s: %s (PID %d)", source, gameID, proc.Name, proc.PID)
		return proc, true
	}
	if len(candidates) > 1 {
		logInfo(logger, "Multiple %s candidates found for game %s, requiring manual selection: %s", source, gameID, FormatProcessCandidates(candidates))
	}
	return processutils.ProcessInfo{}, false
}

func launchDirProcessCandidates(input StagedProcessDetectionInput, logger DetectionLogger) ([]processutils.ProcessInfo, error) {
	candidates, err := processutils.GetProcessesByExecutableDir(input.LaunchDir)
	if err != nil {
		logWarning(logger, "Failed to enumerate processes in launch dir %s for game %s: %v", input.LaunchDir, input.GameID, err)
		return nil, err
	}

	filtered := make([]processutils.ProcessInfo, 0, len(candidates))
	for _, proc := range candidates {
		if proc.PID == input.Launcher.PID || IsLikelyHelperProcess(proc.Name) {
			continue
		}
		filtered = append(filtered, proc)
	}
	if len(filtered) == 0 {
		logInfo(logger, "No non-helper process found in launch dir %s for game %s", input.LaunchDir, input.GameID)
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
		name := strings.ToLower(strings.TrimSpace(proc.Name))
		if name == "" || name == "steam" || name == "steamwebhelper" || IsLikelyHelperProcess(proc.Name) {
			continue
		}
		filtered = append(filtered, proc)
	}
	if len(filtered) == 0 {
		logInfo(logger, "No Steam game process found in install dir %s for game %s", input.LaunchDir, input.GameID)
	}
	return filtered, nil
}

var successorGraceDelays = []time.Duration{0, 1 * time.Second, 2 * time.Second, 3 * time.Second}

const successorStartupPhase = 60 * time.Second

func DetectSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	delays := successorGraceDelays
	if !input.SessionStart.IsZero() && time.Since(input.SessionStart) >= successorStartupPhase {
		delays = successorGraceDelays[:1]
	}

	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}
		if proc, ok := findSuccessorProcess(input, logger); ok {
			logInfo(logger, "Detected successor process for game %s on attempt %d: %s (PID %d), previous: %s (PID %d)", input.GameID, attempt+1, proc.Name, proc.PID, input.ExitedProcessName, input.ExitedPID)
			return proc, true
		}
	}
	return processutils.ProcessInfo{}, false
}

func findSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	var dirCandidates []processutils.ProcessInfo
	if strings.TrimSpace(input.LaunchDir) != "" {
		if dirProcs, err := processutils.GetProcessesByExecutableDir(input.LaunchDir); err == nil {
			dirCandidates = filterSuccessorCandidates(dirProcs, input, logger)
		}
	}

	descendantPIDs := make(map[uint32]bool)
	if descendants, err := processutils.GetDescendantProcesses(input.ExitedPID); err == nil {
		for _, proc := range descendants {
			descendantPIDs[proc.PID] = true
		}
	}

	descendantDirCandidates := make([]processutils.ProcessInfo, 0, len(dirCandidates))
	for _, proc := range dirCandidates {
		if descendantPIDs[proc.PID] {
			descendantDirCandidates = append(descendantDirCandidates, proc)
		}
	}
	if proc, ok := pickSuccessorCandidate(descendantDirCandidates); ok {
		return proc, true
	}

	if proc, ok := pickSuccessorCandidate(dirCandidates); ok {
		return proc, true
	}

	for _, name := range successorNameCandidates(input) {
		pid, err := processutils.GetProcessPIDByName(name)
		if err != nil || pid == 0 || pid == input.ExitedPID || pid == input.SelfPID {
			continue
		}
		proc := processutils.ProcessInfo{Name: name, PID: pid}
		if startedWithinSession(proc, input, logger) {
			return proc, true
		}
	}
	return processutils.ProcessInfo{}, false
}

func filterSuccessorCandidates(processes []processutils.ProcessInfo, input SuccessorDetectionInput, logger DetectionLogger) []processutils.ProcessInfo {
	candidates := make([]processutils.ProcessInfo, 0, len(processes))
	for _, proc := range processes {
		if proc.PID == 0 || proc.PID == input.ExitedPID || proc.PID == input.SelfPID {
			continue
		}
		if IsLikelyHelperProcess(proc.Name) || !startedWithinSession(proc, input, logger) {
			continue
		}
		candidates = append(candidates, proc)
	}
	return candidates
}

func startedWithinSession(proc processutils.ProcessInfo, input SuccessorDetectionInput, logger DetectionLogger) bool {
	if input.SessionStart.IsZero() {
		return true
	}
	created, err := processutils.GetProcessCreationTime(proc.PID)
	if err != nil {
		logInfo(logger, "Cannot read creation time of successor candidate %s (PID %d) for game %s, keeping it: %v", proc.Name, proc.PID, input.GameID, err)
		return true
	}
	return !created.Before(input.SessionStart.Add(-2 * time.Second))
}

func pickSuccessorCandidate(candidates []processutils.ProcessInfo) (processutils.ProcessInfo, bool) {
	if len(candidates) == 1 && IsPersistableProcessName(candidates[0].Name) {
		return candidates[0], true
	}
	return processutils.ProcessInfo{}, false
}

func successorNameCandidates(input SuccessorDetectionInput) []string {
	names := make([]string, 0, 2)
	seen := make(map[string]bool, 2)
	for _, name := range []string{input.SavedProcessName, input.ExitedProcessName} {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || !IsPersistableProcessName(trimmed) {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, trimmed)
	}
	return names
}
