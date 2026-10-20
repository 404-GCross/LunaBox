//go:build windows

package launcher

import (
	"lunabox/backend/utils/processutils"
	"lunabox/backend/utils/timerutils/focusing"
	"os"
	"strings"
	"time"
)

// detectionProgressLogInterval throttles "still detecting" logs so a slow
// launcher does not flood the log at the 500ms sample rate.
const detectionProgressLogInterval = 10 * time.Second

// DetectStagedProcess resolves the actual game process behind a Windows launcher.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	return detectScoredProcess(input, false, logger)
}

// DetectSteamDirectoryProcess resolves the game process Steam started from the
// game's install directory.
func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	return detectScoredProcess(input, true, logger)
}

// detectScoredProcess samples candidates every detectionSampleInterval and
// commits as soon as one candidate's score clears the time-decaying threshold.
// An early commit keeps verifying in the background until the deadline so a
// process that takes over later (launcher/settings window that stays open)
// can still be adopted.
func detectScoredProcess(input StagedProcessDetectionInput, steam bool, logger DetectionLogger) StagedProcessDetectionResult {
	deadline := processDetectionDeadline(input)
	started := time.Now()
	sampler := newWindowsCandidateSampler(input, steam)
	scorer := newProcessScorer(deadline.Sub(started))

	if steam {
		logInfo(logger, "Starting scored Steam directory detection for game %s until %s, install dir: %s", input.GameID, deadline.Format(time.RFC3339), input.LaunchDir)
	} else {
		logInfo(logger, "Starting scored process detection for game %s until %s, launcher: %s (PID %d), dir: %s", input.GameID, deadline.Format(time.RFC3339), input.Launcher.Name, input.Launcher.PID, input.LaunchDir)
	}
	if sampler.savedName != "" {
		logInfo(logger, "Game %s has saved process_name: %s, will commit to it as soon as it runs", input.GameID, sampler.savedName)
	}

	nextProgressLog := detectionProgressLogInterval
	sampleErrorLogged := false
	for {
		at := time.Since(started)
		samples, launcherAlive, err := sampler.sample()
		if err != nil {
			if !sampleErrorLogged {
				logWarning(logger, "Failed to sample process candidates for game %s: %v", input.GameID, err)
				sampleErrorLogged = true
			}
		} else {
			scorer.observe(at, launcherAlive, samples)
			if saved, ok := savedNameCandidate(samples); ok {
				logInfo(logger, "Found saved process %s with PID %d for game %s after %s", saved.Process.Name, saved.Process.PID, input.GameID, at.Round(time.Millisecond))
				result := StagedProcessDetectionResult{
					ProcessID:           saved.Process.PID,
					ProcessName:         saved.Process.Name,
					CloseLauncherHandle: true,
				}
				return startProcessVerification(input, sampler, scorer, started, deadline, result, logger)
			}
			if picked, ok := scorer.decide(); ok {
				logInfo(logger, "Detected game process for game %s after %s: %s (PID %d, score %.2f); candidates: %s", input.GameID, at.Round(time.Millisecond), picked.Process.Name, picked.Process.PID, scorer.score(picked.Process.PID), scorer.summary(5))
				return startProcessVerification(input, sampler, scorer, started, deadline, resultForScoredCandidate(input, steam, picked), logger)
			}
			if at >= nextProgressLog {
				logInfo(logger, "Still detecting game process for game %s after %s (threshold %.2f): %s", input.GameID, at.Round(time.Second), commitThreshold(at, scorer.timeout), scorer.summary(5))
				nextProgressLog += detectionProgressLogInterval
			}
		}

		if !waitForProcessDetection(deadline, detectionSampleInterval, input.Done) {
			break
		}
	}

	if processDetectionCancelled(input.Done) {
		return StagedProcessDetectionResult{}
	}
	return fallbackScoredDetection(input, steam, scorer, logger)
}

func fallbackScoredDetection(input StagedProcessDetectionInput, steam bool, scorer *processScorer, logger DetectionLogger) StagedProcessDetectionResult {
	if picked, ok := scorer.fallback(); ok {
		logInfo(logger, "Process detection deadline reached for game %s; using best candidate %s (PID %d, score %.2f)", input.GameID, picked.Process.Name, picked.Process.PID, scorer.score(picked.Process.PID))
		return resultForScoredCandidate(input, steam, picked)
	}
	if !steam && processutils.IsProcessPresentByPID(input.Launcher.PID) {
		logInfo(logger, "Process detection timed out for game %s; monitoring launcher %s (PID %d); candidates: %s", input.GameID, input.LauncherExeName, input.Launcher.PID, scorer.summary(5))
		return resultForLauncher(input)
	}
	if picked, ok := scorer.soleStableCandidate(); ok {
		logInfo(logger, "Process detection timed out for game %s; using the only remaining process %s (PID %d)", input.GameID, picked.Process.Name, picked.Process.PID)
		return resultForScoredCandidate(input, steam, picked)
	}
	logWarning(logger, "Process detection failed for game %s, requiring manual process selection; candidates: %s", input.GameID, scorer.summary(5))
	return promptProcessSelectionResult()
}

func resultForScoredCandidate(input StagedProcessDetectionInput, steam bool, picked candidateSample) StagedProcessDetectionResult {
	switch {
	case picked.IsLauncher:
		return resultForLauncher(input)
	case steam:
		return resultForSteamProcess(picked.Process)
	default:
		return resultForExternalProcess(input, picked.Process, true)
	}
}

func resultForSteamProcess(proc processutils.ProcessInfo) StagedProcessDetectionResult {
	return StagedProcessDetectionResult{
		ProcessID:           proc.PID,
		ProcessName:         proc.Name,
		CloseLauncherHandle: true,
		PersistProcessName:  ProcessNameForPersistence("", proc.Name),
	}
}

func savedNameCandidate(samples []candidateSample) (candidateSample, bool) {
	for _, sample := range samples {
		if sample.SavedName {
			return sample, true
		}
	}
	return candidateSample{}, false
}

// startProcessVerification attaches a verifier that keeps scoring until the
// detection deadline. The sampler and scorer are handed over to the verifier
// goroutine and must not be used by the caller afterwards.
func startProcessVerification(input StagedProcessDetectionInput, sampler *windowsCandidateSampler, scorer *processScorer, started time.Time, deadline time.Time, result StagedProcessDetectionResult, logger DetectionLogger) StagedProcessDetectionResult {
	if result.ProcessID == 0 || time.Until(deadline) <= 0 {
		return result
	}
	verifier := newProcessVerifier(result.ProcessID)
	result.Verifier = verifier
	go verifyDetectedProcess(input, sampler, scorer, started, deadline, verifier, logger)
	return result
}

func verifyDetectedProcess(input StagedProcessDetectionInput, sampler *windowsCandidateSampler, scorer *processScorer, started time.Time, deadline time.Time, verifier *ProcessVerifier, logger DetectionLogger) {
	for retargets := 0; retargets < maxVerificationRetargets; {
		if !waitForProcessDetection(deadline, verificationSampleInterval, input.Done) {
			return
		}
		samples, launcherAlive, err := sampler.sample()
		if err != nil {
			continue
		}
		scorer.observe(time.Since(started), launcherAlive, samples)
		target := verifier.Target()
		next, ok := scorer.challenger(target)
		if !ok {
			continue
		}
		logInfo(logger, "Process verification for game %s: %s (PID %d, score %.2f) took over from PID %d (score %.2f)", input.GameID, next.Process.Name, next.Process.PID, scorer.score(next.Process.PID), target, scorer.score(target))
		verifier.offer(next.Process)
		retargets++
	}
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

func newWindowsCandidateSampler(input StagedProcessDetectionInput, steam bool) *windowsCandidateSampler {
	launcherExeName := input.LauncherExeName
	if steam {
		launcherExeName = "steam.exe"
	}
	savedName := ""
	if HasReliableSavedProcessName(input.SavedProcessName, launcherExeName) {
		savedName = strings.TrimSpace(input.SavedProcessName)
	}
	return &windowsCandidateSampler{
		input:      input,
		steam:      steam,
		savedName:  savedName,
		selfPID:    uint32(os.Getpid()),
		imagePaths: processutils.NewProcessImagePathCache(),
	}
}

// sample returns this tick's candidates and whether the launcher still runs.
// In Steam mode the launcher is steam.exe, which is never a game candidate.
func (s *windowsCandidateSampler) sample() ([]candidateSample, bool, error) {
	snapshot, err := processutils.CaptureWindowsProcessSnapshot()
	if err != nil {
		return nil, false, err
	}

	byPID := make(map[uint32]*candidateSample)
	order := make([]uint32, 0)
	add := func(proc processutils.ProcessInfo) *candidateSample {
		if proc.PID == 0 || proc.PID == s.selfPID {
			return nil
		}
		if candidate, ok := byPID[proc.PID]; ok {
			return candidate
		}
		if IsLikelyHelperProcess(proc.Name) || (s.steam && strings.EqualFold(proc.Name, "steam.exe")) {
			return nil
		}
		candidate := &candidateSample{Process: proc}
		byPID[proc.PID] = candidate
		order = append(order, proc.PID)
		return candidate
	}

	launcherAlive := false
	if !s.steam {
		if launcher, ok := snapshot.Process(s.input.Launcher.PID); ok {
			launcherAlive = true
			if candidate := add(launcher); candidate != nil {
				candidate.IsLauncher = true
			}
		}
		for _, proc := range snapshot.DescendantProcesses(s.input.Launcher.PID) {
			if candidate := add(proc); candidate != nil {
				candidate.Descendant = true
			}
		}
	}
	if strings.TrimSpace(s.input.LaunchDir) != "" {
		dirProcesses, err := snapshot.ProcessesByExecutableDir(s.input.LaunchDir, s.imagePaths)
		if err != nil {
			return nil, launcherAlive, err
		}
		for _, proc := range dirProcesses {
			if candidate := add(proc); candidate != nil {
				candidate.InLaunchDir = true
			}
		}
	}
	if s.savedName != "" {
		for _, proc := range snapshot.ProcessesByName(s.savedName) {
			if candidate := add(proc); candidate != nil {
				candidate.SavedName = true
			}
		}
	}
	if len(order) == 0 {
		return nil, launcherAlive, nil
	}

	processes := make([]processutils.ProcessInfo, 0, len(order))
	for _, pid := range order {
		processes = append(processes, byPID[pid].Process)
	}
	windowPIDs := make(map[uint32]bool, len(processes))
	for _, proc := range processutils.FilterProcessesWithVisibleWindows(processes) {
		windowPIDs[proc.PID] = true
	}
	foregroundPID, hasForeground := focusing.GetForegroundProcessID()

	samples := make([]candidateSample, 0, len(order))
	for _, pid := range order {
		candidate := byPID[pid]
		candidate.Window = windowPIDs[pid]
		candidate.Foreground = hasForeground && pid == foregroundPID
		samples = append(samples, *candidate)
	}
	return samples, launcherAlive, nil
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
	return !created.Before(input.SessionStart.Add(-2 * time.Second))
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
