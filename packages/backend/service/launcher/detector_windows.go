//go:build windows

package launcher

import (
	"lunabox/backend/utils/processutils"
	"lunabox/backend/utils/timerutils/focusing"
	"os"
	"strings"
	"time"
)

// DetectStagedProcess resolves the actual game process behind a Windows launcher.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	return detectScoredProcess(input, scoredDetection{
		label:            "Windows staged",
		launcherFallback: true,
		sampler:          newWindowsCandidateSampler(input, false, logger),
	}, logger)
}

// DetectSteamDirectoryProcess resolves the game process Steam started from the
// game's install directory.
func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	return detectScoredProcess(input, scoredDetection{
		label:   "Windows Steam directory",
		steam:   true,
		sampler: newWindowsCandidateSampler(input, true, logger),
	}, logger)
}

// windowsCandidateSampler builds one tick of candidate evidence from a single
// process snapshot, one window enumeration and one foreground query.
type windowsCandidateSampler struct {
	input      StagedProcessDetectionInput
	steam      bool
	savedName  string
	selfPID    uint32
	imagePaths *processutils.ProcessImagePathCache
}

func newWindowsCandidateSampler(input StagedProcessDetectionInput, steam bool, logger DetectionLogger) *windowsCandidateSampler {
	launcherExeName := input.LauncherExeName
	if steam {
		launcherExeName = "steam.exe"
	}
	savedName := ""
	if HasReliableSavedProcessName(input.SavedProcessName, launcherExeName) {
		savedName = strings.TrimSpace(input.SavedProcessName)
		logInfo(logger, "Game %s has saved process_name: %s, will commit to it as soon as it runs", input.GameID, savedName)
	}
	return &windowsCandidateSampler{
		input:      input,
		steam:      steam,
		savedName:  savedName,
		selfPID:    uint32(os.Getpid()),
		imagePaths: processutils.NewProcessImagePathCache(),
	}
}

// sample returns this tick's candidates. In Steam mode the launcher is
// steam.exe, which is never a game candidate.
func (s *windowsCandidateSampler) sample() ([]candidateSample, error) {
	snapshot, err := processutils.CaptureWindowsProcessSnapshot()
	if err != nil {
		return nil, err
	}

	links := make(map[uint32]*processLinks)
	infos := make([]processutils.ProcessInfo, 0)
	add := func(proc processutils.ProcessInfo) *processLinks {
		if proc.PID == 0 || proc.PID == s.selfPID {
			return nil
		}
		if link, ok := links[proc.PID]; ok {
			return link
		}
		if IsLikelyHelperProcess(proc.Name) || (s.steam && strings.EqualFold(proc.Name, "steam.exe")) {
			return nil
		}
		link := &processLinks{DirIsPrimary: s.steam}
		links[proc.PID] = link
		infos = append(infos, proc)
		return link
	}

	if !s.steam {
		if launcher, ok := snapshot.Process(s.input.Launcher.PID); ok {
			if link := add(launcher); link != nil {
				link.IsLauncher = true
			}
		}
		for _, proc := range snapshot.DescendantProcesses(s.input.Launcher.PID) {
			if link := add(proc); link != nil {
				link.Descendant = true
			}
		}
	}
	if strings.TrimSpace(s.input.LaunchDir) != "" {
		dirProcesses, err := snapshot.ProcessesByExecutableDir(s.input.LaunchDir, s.imagePaths)
		if err != nil {
			return nil, err
		}
		for _, proc := range dirProcesses {
			if link := add(proc); link != nil {
				link.InLaunchDir = true
			}
		}
	}
	if s.savedName != "" {
		for _, proc := range snapshot.ProcessesByName(s.savedName) {
			if link := add(proc); link != nil {
				link.SavedName = true
			}
		}
	}
	if len(infos) == 0 {
		return nil, nil
	}

	windowPIDs := make(map[uint32]bool, len(infos))
	for _, proc := range processutils.FilterProcessesWithVisibleWindows(infos) {
		windowPIDs[proc.PID] = true
	}
	// No foreground window (desktop, lock screen) still means none of the
	// candidates is in front, so the channel stays observable.
	foregroundPID, hasForeground := focusing.GetForegroundProcessID()

	samples := make([]candidateSample, 0, len(infos))
	for _, proc := range infos {
		link := links[proc.PID]
		samples = append(samples, candidateSample{
			Process:         proc,
			IsLauncher:      link.IsLauncher,
			SavedName:       link.SavedName,
			Relation:        link.relation(),
			Window:          windowPIDs[proc.PID],
			WindowKnown:     true,
			Foreground:      hasForeground && proc.PID == foregroundPID,
			ForegroundKnown: true,
		})
	}
	return samples, nil
}

// successorGraceDelays paces successor detection after a monitored process
// exits during the session start-up phase. The immediate check catches the
// common splash→game hand-off where the real game is spawned before the splash
// closes; the later checks cover the exec gap where the old process dies
// slightly before its successor appears.
var successorGraceDelays = []time.Duration{0, 1 * time.Second, 2 * time.Second, 3 * time.Second}

// successorStartupPhase bounds the start-up window in which splash/launcher
// hand-offs normally happen. It matches the minimum session duration: sessions
// shorter than this are deleted anyway, so the grace waits cannot distort any
// recorded play time.
const successorStartupPhase = 60 * time.Second

// DetectSuccessorProcess looks for a process that took over from an exited
// monitored process: a splash window that spawned the real game, a launcher
// that was mistakenly treated as the game, a self re-exec with the same exe
// name, or an in-game restart. It returns false when the exit looks like a
// genuine game shutdown.
func DetectSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	delays := successorGraceDelays
	if !input.SessionStart.IsZero() && time.Since(input.SessionStart) >= successorStartupPhase {
		// Past the start-up phase the game's window/process structure is
		// stable: a genuine exit leaves the game directory without processes.
		// A single immediate check keeps hand-off support for in-game restarts
		// without delaying session finalization.
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
	logInfo(logger, "No successor process found for game %s after %s (PID %d) exited, treating as game shutdown", input.GameID, input.ExitedProcessName, input.ExitedPID)
	return processutils.ProcessInfo{}, false
}

func findSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	// A successor must live under the game's directory or reuse a known process
	// name. Bare descent from the exited process is NOT enough: games commonly
	// spawn a browser (survey/official site) right before exiting, and that
	// child must not be mistaken for the game.
	var dirCandidates []processutils.ProcessInfo
	if strings.TrimSpace(input.LaunchDir) != "" {
		if dirProcs, err := processutils.GetProcessesByExecutableDir(input.LaunchDir); err == nil {
			dirCandidates = filterSuccessorCandidates(dirProcs, input, logger)
		}
	}

	descendantPIDs := make(map[uint32]bool)
	if descendants, err := processutils.GetDescendantProcesses(input.ExitedPID); err == nil {
		// Snapshots keep the parent PID of the exited process, so children it
		// spawned before dying are still discoverable.
		for _, proc := range descendants {
			descendantPIDs[proc.PID] = true
		}
	}

	// Strongest evidence first: spawned by the exited process AND running from
	// the game's directory (splash window → real game).
	descendantDirCandidates := make([]processutils.ProcessInfo, 0, len(dirCandidates))
	for _, proc := range dirCandidates {
		if descendantPIDs[proc.PID] {
			descendantDirCandidates = append(descendantDirCandidates, proc)
		}
	}
	if proc, ok := pickSuccessorCandidate(descendantDirCandidates); ok {
		return proc, true
	}

	// Then any process from the game's directory (launcher hand-off without a
	// parent-child relationship, e.g. via a broker process).
	if proc, ok := pickSuccessorCandidate(dirCandidates); ok {
		return proc, true
	}

	// Finally same-name matches anywhere: covers re-exec where the real game
	// reuses the exe name but runs from an unpacked location.
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
		if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		if !startedWithinSession(proc, input, logger) {
			continue
		}
		candidates = append(candidates, proc)
	}
	return candidates
}

// startedWithinSession rejects candidates created before the play session
// began (stale processes or PID reuse). Creation time can be unreadable for
// elevated games when the app itself is not elevated; keep those candidates so
// admin games do not lose hand-off support.
func startedWithinSession(proc processutils.ProcessInfo, input SuccessorDetectionInput, logger DetectionLogger) bool {
	if input.SessionStart.IsZero() {
		return true
	}
	created, err := processutils.GetProcessCreationTime(proc.PID)
	if err != nil {
		logInfo(logger, "Cannot read creation time of successor candidate %s (PID %d) for game %s, keeping it: %v", proc.Name, proc.PID, input.GameID, err)
		return true
	}
	return !created.Before(input.SessionStart.Add(-sessionStartTolerance))
}

func pickSuccessorCandidate(candidates []processutils.ProcessInfo) (processutils.ProcessInfo, bool) {
	if len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range candidates {
			if proc.PID == foregroundPID {
				return proc, true
			}
		}
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 1 {
		return windowCandidates[0], true
	}
	if len(windowCandidates) > 1 {
		// Ambiguous: wait for a later grace attempt instead of guessing.
		return processutils.ProcessInfo{}, false
	}

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
