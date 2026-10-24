package launcher

import (
	"lunabox/backend/utils/processutils"
	"time"
)

// detectionProgressLogInterval throttles "still detecting" logs so a slow
// launcher does not flood the log at the 500ms sample rate.
const detectionProgressLogInterval = 10 * time.Second

// candidateSampler observes one tick of candidate evidence. Platform samplers
// own their snapshot caches and are used from one goroutine at a time.
type candidateSampler interface {
	sample() ([]candidateSample, error)
}

type scoredDetection struct {
	// label prefixes log lines, e.g. "Linux Steam directory".
	label string
	// steam selects Steam results: the launcher is the Steam client, which is
	// never the game, and the detected name is always persisted.
	steam bool
	// launcherFallback monitors a still-running launcher at the deadline
	// instead of asking the user to pick a process.
	launcherFallback bool
	sampler          candidateSampler
}

// detectScoredProcess samples candidates every detectionSampleInterval and
// commits as soon as one candidate's score clears the time-decaying threshold.
// An early commit keeps verifying in the background until the deadline so a
// process that takes over later (launcher/settings window that stays open)
// can still be adopted.
func detectScoredProcess(input StagedProcessDetectionInput, detection scoredDetection, logger DetectionLogger) StagedProcessDetectionResult {
	deadline := processDetectionDeadline(input)
	started := time.Now()
	scorer := newProcessScorer(deadline.Sub(started))
	detection.sampler = newSessionCandidateFilter(detection.sampler, input, logger)

	logInfo(logger, "Starting scored %s detection for game %s until %s, launcher: %s (PID %d), dir: %s", detection.label, input.GameID, deadline.Format(time.RFC3339), input.Launcher.Name, input.Launcher.PID, input.LaunchDir)

	nextProgressLog := detectionProgressLogInterval
	sampleErrorLogged := false
	for {
		at := time.Since(started)
		samples, err := detection.sampler.sample()
		if err != nil {
			if !sampleErrorLogged {
				logWarning(logger, "Failed to sample process candidates for game %s: %v", input.GameID, err)
				sampleErrorLogged = true
			}
		} else {
			scorer.observe(at, samples)
			if saved, ok := savedNameCandidate(samples); ok {
				logInfo(logger, "Found saved process %s with PID %d for game %s after %s", saved.Process.Name, saved.Process.PID, input.GameID, at.Round(time.Millisecond))
				result := StagedProcessDetectionResult{
					ProcessID:           saved.Process.PID,
					ProcessName:         saved.Process.Name,
					CloseLauncherHandle: true,
				}
				return startProcessVerification(input, detection, scorer, started, deadline, result, logger)
			}
			if picked, ok := scorer.decide(); ok {
				logInfo(logger, "Detected game process for game %s after %s: %s (PID %d, score %.2f); candidates: %s", input.GameID, at.Round(time.Millisecond), picked.Process.Name, picked.Process.PID, scorer.score(picked.Process.PID), scorer.summary(5))
				return startProcessVerification(input, detection, scorer, started, deadline, resultForScoredCandidate(input, detection, picked), logger)
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
	return fallbackScoredDetection(input, detection, scorer, logger)
}

func fallbackScoredDetection(input StagedProcessDetectionInput, detection scoredDetection, scorer *processScorer, logger DetectionLogger) StagedProcessDetectionResult {
	if picked, ok := scorer.fallback(); ok {
		logInfo(logger, "Process detection deadline reached for game %s; using best candidate %s (PID %d, score %.2f)", input.GameID, picked.Process.Name, picked.Process.PID, scorer.score(picked.Process.PID))
		return resultForScoredCandidate(input, detection, picked)
	}
	if detection.launcherFallback && processutils.IsProcessPresentByPID(input.Launcher.PID) {
		logInfo(logger, "Process detection timed out for game %s; monitoring launcher %s (PID %d); candidates: %s", input.GameID, input.LauncherExeName, input.Launcher.PID, scorer.summary(5))
		return resultForLauncher(input)
	}
	if picked, ok := scorer.soleStableCandidate(); ok {
		logInfo(logger, "Process detection timed out for game %s; using the only remaining process %s (PID %d)", input.GameID, picked.Process.Name, picked.Process.PID)
		return resultForScoredCandidate(input, detection, picked)
	}
	logWarning(logger, "Process detection failed for game %s, requiring manual process selection; candidates: %s", input.GameID, scorer.summary(5))
	return promptProcessSelectionResult()
}

func resultForScoredCandidate(input StagedProcessDetectionInput, detection scoredDetection, picked candidateSample) StagedProcessDetectionResult {
	switch {
	case picked.IsLauncher:
		return resultForLauncher(input)
	case detection.steam:
		return resultForSteamProcess(picked.Process)
	default:
		return resultForExternalProcess(input, picked.Process, true)
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
func startProcessVerification(input StagedProcessDetectionInput, detection scoredDetection, scorer *processScorer, started time.Time, deadline time.Time, result StagedProcessDetectionResult, logger DetectionLogger) StagedProcessDetectionResult {
	if result.ProcessID == 0 || time.Until(deadline) <= 0 {
		return result
	}
	verifier := newProcessVerifier(result.ProcessID)
	result.Verifier = verifier
	go verifyDetectedProcess(input, detection.sampler, scorer, started, deadline, verifier, logger)
	return result
}

func verifyDetectedProcess(input StagedProcessDetectionInput, sampler candidateSampler, scorer *processScorer, started time.Time, deadline time.Time, verifier *ProcessVerifier, logger DetectionLogger) {
	for retargets := 0; retargets < maxVerificationRetargets; {
		if !waitForProcessDetection(deadline, verificationSampleInterval, input.Done) {
			return
		}
		samples, err := sampler.sample()
		if err != nil {
			continue
		}
		scorer.observe(time.Since(started), samples)
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

// sessionCandidateFilter drops candidates that were already running before the
// launch, e.g. a tool left open from the game directory or a stale instance.
// The launcher is ours, and a saved process name identifies the game itself
// (single-instance games may hand over to an already running copy), so both
// are always kept.
type sessionCandidateFilter struct {
	inner        candidateSampler
	input        StagedProcessDetectionInput
	logger       DetectionLogger
	creationTime func(pid uint32) (time.Time, error)
	// stale caches the verdict per PID; the name guards against PID reuse.
	stale map[uint32]staleVerdict
}

type staleVerdict struct {
	name  string
	stale bool
}

func newSessionCandidateFilter(inner candidateSampler, input StagedProcessDetectionInput, logger DetectionLogger) candidateSampler {
	if input.SessionStart.IsZero() {
		return inner
	}
	return &sessionCandidateFilter{
		inner:        inner,
		input:        input,
		logger:       logger,
		creationTime: processutils.GetProcessCreationTime,
		stale:        make(map[uint32]staleVerdict),
	}
}

func (f *sessionCandidateFilter) sample() ([]candidateSample, error) {
	samples, err := f.inner.sample()
	if err != nil {
		return nil, err
	}

	retained := make(map[uint32]staleVerdict, len(samples))
	filtered := samples[:0]
	for _, sample := range samples {
		if sample.IsLauncher || sample.SavedName {
			filtered = append(filtered, sample)
			continue
		}
		pid := sample.Process.PID
		verdict, ok := f.stale[pid]
		if !ok || verdict.name != sample.Process.Name {
			verdict = staleVerdict{name: sample.Process.Name, stale: f.startedBeforeSession(sample.Process)}
		}
		retained[pid] = verdict
		if !verdict.stale {
			filtered = append(filtered, sample)
		}
	}
	f.stale = retained
	return filtered, nil
}

// startedBeforeSession keeps candidates whose creation time is unreadable
// (elevated games, platforms without support) so detection does not regress.
func (f *sessionCandidateFilter) startedBeforeSession(proc processutils.ProcessInfo) bool {
	created, err := f.creationTime(proc.PID)
	if err != nil || created.IsZero() {
		return false
	}
	if !created.Before(f.input.SessionStart.Add(-sessionStartTolerance)) {
		return false
	}
	logInfo(f.logger, "Ignoring process %s (PID %d) for game %s: it was already running before the launch (created %s)", proc.Name, proc.PID, f.input.GameID, created.Format(time.RFC3339))
	return true
}
