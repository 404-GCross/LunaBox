//go:build linux

package launcher

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lunabox/backend/utils/processutils"
	"lunabox/backend/utils/timerutils/focusing"
)

// linuxRelationFullScore is the static match score treated as a certain link.
// A name match (launched exe, truncated comm or saved name) plus any path
// evidence reaches it; path-only matches stay around 0.7 so a name-matched
// process wins by the commit margin.
const linuxRelationFullScore = 200

func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	return detectScoredProcess(input, scoredDetection{
		label:            "Linux staged",
		launcherFallback: true,
		sampler:          newLinuxCandidateSampler(input, input.LauncherExeName, logger),
	}, logger)
}

func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	return detectScoredProcess(input, scoredDetection{
		label:   "Linux Steam directory",
		steam:   true,
		sampler: newLinuxCandidateSampler(input, "steam", logger),
	}, logger)
}

// linuxCandidateSampler gathers launcher descendants and install-directory
// processes from one /proc snapshot. Linux has no window enumeration, so the
// evidence is the static /proc match plus the KDE foreground window when
// kdotool is available.
type linuxCandidateSampler struct {
	input     StagedProcessDetectionInput
	savedName string
}

func newLinuxCandidateSampler(input StagedProcessDetectionInput, launcherExeName string, logger DetectionLogger) *linuxCandidateSampler {
	savedName := ""
	if HasReliableSavedProcessName(input.SavedProcessName, launcherExeName) {
		savedName = strings.TrimSpace(input.SavedProcessName)
		logInfo(logger, "Game %s has saved process_name: %s, will commit to it as soon as it runs", input.GameID, savedName)
	}
	return &linuxCandidateSampler{input: input, savedName: savedName}
}

func (s *linuxCandidateSampler) sample() ([]candidateSample, error) {
	snapshot, err := processutils.CaptureLinuxProcessSnapshot()
	if err != nil {
		return nil, err
	}

	groups := [][]linuxProcessCandidate{
		linuxCandidatesFromDetails(snapshot.DescendantProcessDetails(s.input.Launcher.PID), true, false),
	}
	if strings.TrimSpace(s.input.LaunchDir) != "" {
		if details, err := snapshot.ProcessDetailsByExecutableDir(s.input.LaunchDir); err == nil {
			groups = append(groups, linuxCandidatesFromDetails(details, false, true))
		}
	}
	samples := linuxCandidateSamples(s.input, linuxMergeProcessCandidates(groups...))

	if s.savedName != "" {
		if pid, found := snapshot.ProcessPIDByName(s.savedName); found && pid != s.input.Launcher.PID {
			samples = markLinuxSavedProcess(samples, processutils.ProcessInfo{Name: s.savedName, PID: pid})
		}
	}

	foregroundPID, foregroundKnown := focusing.GetForegroundProcessID()
	for i := range samples {
		samples[i].ForegroundKnown = foregroundKnown
		samples[i].Foreground = foregroundKnown && samples[i].Process.PID == foregroundPID
	}
	return samples, nil
}

// linuxCandidateSamples converts static /proc matches into scorer samples.
// Candidates without any name or path match (and the wine/proton launcher
// itself) are not candidates at all.
func linuxCandidateSamples(input StagedProcessDetectionInput, candidates []linuxProcessCandidate) []candidateSample {
	targetNames := linuxTargetProcessNames(input, candidates)
	samples := make([]candidateSample, 0, len(candidates))
	for _, candidate := range candidates {
		score := linuxProcessCandidateScore(candidate, input, targetNames)
		if score <= 0 {
			continue
		}
		samples = append(samples, candidateSample{
			Process:  candidate.detail.ProcessInfo,
			Relation: math.Min(1, float64(score)/linuxRelationFullScore),
		})
	}
	return samples
}

func markLinuxSavedProcess(samples []candidateSample, saved processutils.ProcessInfo) []candidateSample {
	for i := range samples {
		if samples[i].Process.PID == saved.PID {
			samples[i].SavedName = true
			samples[i].Relation = relationSavedName
			return samples
		}
	}
	return append(samples, candidateSample{Process: saved, SavedName: true, Relation: relationSavedName})
}

type linuxProcessCandidate struct {
	detail         processutils.ProcessDetails
	fromDescendant bool
	fromDirectory  bool
}

func linuxCandidatesFromDetails(details []processutils.ProcessDetails, fromDescendant bool, fromDirectory bool) []linuxProcessCandidate {
	candidates := make([]linuxProcessCandidate, 0, len(details))
	for _, detail := range details {
		candidates = append(candidates, linuxProcessCandidate{
			detail:         detail,
			fromDescendant: fromDescendant,
			fromDirectory:  fromDirectory,
		})
	}
	return candidates
}

func linuxMergeProcessCandidates(groups ...[]linuxProcessCandidate) []linuxProcessCandidate {
	byPID := make(map[uint32]linuxProcessCandidate)
	for _, group := range groups {
		for _, candidate := range group {
			pid := candidate.detail.PID
			if pid == 0 {
				continue
			}
			existing, ok := byPID[pid]
			if !ok {
				byPID[pid] = candidate
				continue
			}
			existing.fromDescendant = existing.fromDescendant || candidate.fromDescendant
			existing.fromDirectory = existing.fromDirectory || candidate.fromDirectory
			if existing.detail.ExecutablePath == "" {
				existing.detail.ExecutablePath = candidate.detail.ExecutablePath
			}
			if existing.detail.CurrentDirectory == "" {
				existing.detail.CurrentDirectory = candidate.detail.CurrentDirectory
			}
			if len(existing.detail.CommandLine) == 0 {
				existing.detail.CommandLine = candidate.detail.CommandLine
			}
			byPID[pid] = existing
		}
	}

	candidates := make([]linuxProcessCandidate, 0, len(byPID))
	for _, candidate := range byPID {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].detail.PID < candidates[j].detail.PID
	})
	return candidates
}

func linuxProcessCandidateScore(candidate linuxProcessCandidate, input StagedProcessDetectionInput, targetNames []string) int {
	detail := candidate.detail
	if detail.PID == 0 || detail.PID == input.Launcher.PID || linuxIsLikelyHelperProcess(detail) {
		return 0
	}
	nameScore := linuxProcessNameMatchScore(detail.Name, targetNames)
	pathScore := linuxProcessPathScore(detail, input.LaunchDir)
	if nameScore == 0 && pathScore == 0 {
		return 0
	}

	score := nameScore + pathScore
	if candidate.fromDescendant {
		score += 20
	}
	if candidate.fromDirectory {
		score += 20
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(detail.Name)), ".exe") {
		score += 10
	}
	return score
}

func linuxProcessNameMatchScore(processName string, targetNames []string) int {
	processName = strings.ToLower(strings.TrimSpace(processName))
	if processName == "" {
		return 0
	}
	for _, targetName := range targetNames {
		targetName = strings.ToLower(strings.TrimSpace(filepath.Base(targetName)))
		if targetName == "" {
			continue
		}
		if processName == targetName {
			return 120
		}
		if processName == linuxTruncatedCommName(targetName) {
			return 100
		}
		withoutExt := strings.TrimSuffix(targetName, filepath.Ext(targetName))
		if withoutExt != "" && processName == withoutExt {
			return 80
		}
	}
	return 0
}

func linuxTruncatedCommName(name string) string {
	name = strings.TrimSpace(name)
	if len([]byte(name)) <= 15 {
		return strings.ToLower(name)
	}
	return strings.ToLower(string([]byte(name)[:15]))
}

func linuxProcessPathScore(detail processutils.ProcessDetails, launchDir string) int {
	score := 0
	if linuxPathUnderDir(detail.ExecutablePath, launchDir) {
		score += 70
	}
	if linuxPathUnderDir(detail.CurrentDirectory, launchDir) {
		score += 50
	}
	for _, arg := range detail.CommandLine {
		if linuxArgReferencesDir(arg, launchDir) {
			score += 40
			break
		}
	}
	return score
}

func linuxTargetProcessNames(input StagedProcessDetectionInput, candidates []linuxProcessCandidate) []string {
	names := make([]string, 0, 4)
	seen := make(map[string]bool)
	add := func(name string) {
		name = strings.TrimSpace(filepath.Base(linuxNormalizeArgumentPath(name)))
		if name == "" || IsLikelyHelperProcess(name) {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		names = append(names, name)
	}

	add(input.SavedProcessName)
	add(input.Launcher.Name)
	for _, candidate := range candidates {
		for _, arg := range candidate.detail.CommandLine {
			if !linuxArgReferencesDir(arg, input.LaunchDir) {
				continue
			}
			name := filepath.Base(linuxNormalizeArgumentPath(arg))
			switch strings.ToLower(filepath.Ext(name)) {
			case ".exe", ".bat":
				add(name)
			}
		}
	}
	return names
}

func linuxIsLikelyHelperProcess(detail processutils.ProcessDetails) bool {
	if IsLikelyHelperProcess(detail.Name) {
		return true
	}
	if isLinuxSteamRuntimeHelperPath(detail.ExecutablePath) {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(detail.Name))
	if name == "bash" || name == "sh" || name == "python" || strings.HasPrefix(name, "python") {
		return linuxCommandLineContainsProton(detail.CommandLine)
	}
	return false
}

// isLinuxSteamRuntimeHelperPath reports executables that belong to Steam's
// runtime tooling. Those helpers are short-lived and inherit the game's
// working directory, so they must never be mistaken for the game process.
func isLinuxSteamRuntimeHelperPath(path string) bool {
	path = strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if path == "" {
		return false
	}
	for _, marker := range []string{
		"/steam-runtime",
		"/steamlinuxruntime",
		"/pressure-vessel/",
		"/steam-runtime-tools-0/",
		"/ubuntu12_32/",
		"/ubuntu12_64/",
	} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

func linuxCommandLineContainsProton(commandLine []string) bool {
	for _, arg := range commandLine {
		arg = strings.ToLower(filepath.ToSlash(arg))
		if strings.Contains(arg, "/proton") || strings.Contains(arg, "proton ") || strings.HasSuffix(arg, "proton") {
			return true
		}
	}
	return false
}

func linuxNormalizeArgumentPath(arg string) string {
	arg = strings.Trim(strings.TrimSpace(arg), "\"'")
	arg = strings.ReplaceAll(arg, "\\", "/")
	if len(arg) >= 3 && arg[1] == ':' && arg[2] == '/' {
		arg = arg[2:]
	}
	return arg
}

func linuxArgReferencesDir(arg string, rootDir string) bool {
	arg = linuxNormalizeArgumentPath(arg)
	if arg == "" {
		return false
	}
	if linuxPathUnderDir(arg, rootDir) {
		return true
	}
	rootDir = filepath.ToSlash(strings.TrimSpace(rootDir))
	return rootDir != "" && strings.Contains(filepath.ToSlash(arg), rootDir)
}

func linuxPathUnderDir(path string, rootDir string) bool {
	path = linuxNormalizeArgumentPath(path)
	rootDir = strings.TrimSpace(rootDir)
	if path == "" || rootDir == "" || !filepath.IsAbs(path) {
		return false
	}
	absPath, err := linuxCanonicalPath(path)
	if err != nil {
		return false
	}
	absRoot, err := linuxCanonicalPath(rootDir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func linuxCanonicalPath(path string) (string, error) {
	absPath, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return filepath.Clean(resolvedPath), nil
	}
	return absPath, nil
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
	snapshot, err := processutils.CaptureLinuxProcessSnapshot()
	if err != nil {
		logWarning(logger, "Failed to capture Linux process snapshot while finding successor for game %s: %v", input.GameID, err)
		return processutils.ProcessInfo{}, false
	}
	var dirCandidates []processutils.ProcessInfo
	if strings.TrimSpace(input.LaunchDir) != "" {
		if details, err := snapshot.ProcessDetailsByExecutableDir(input.LaunchDir); err == nil {
			dirCandidates = filterSuccessorProcessDetails(details, input, logger)
		}
	}

	descendantPIDs := make(map[uint32]bool)
	for _, proc := range snapshot.DescendantProcesses(input.ExitedPID) {
		descendantPIDs[proc.PID] = true
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
		pid, found := snapshot.ProcessPIDByName(name)
		if !found || pid == 0 || pid == input.ExitedPID || pid == input.SelfPID {
			continue
		}
		proc := processutils.ProcessInfo{Name: name, PID: pid}
		if startedWithinSession(proc, input, logger) {
			return proc, true
		}
	}
	return processutils.ProcessInfo{}, false
}

func filterSuccessorProcessDetails(details []processutils.ProcessDetails, input SuccessorDetectionInput, logger DetectionLogger) []processutils.ProcessInfo {
	candidates := make([]processutils.ProcessInfo, 0, len(details))
	for _, detail := range details {
		proc := detail.ProcessInfo
		if proc.PID == 0 || proc.PID == input.ExitedPID || proc.PID == input.SelfPID {
			continue
		}
		if linuxIsLikelyHelperProcess(detail) || !startedWithinSession(proc, input, logger) {
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
	return !created.Before(input.SessionStart.Add(-sessionStartTolerance))
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
