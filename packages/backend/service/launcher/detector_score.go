package launcher

import (
	"fmt"
	"lunabox/backend/utils/processutils"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Scored process detection
//
// Every sample tick each candidate gets an instant evidence value in [0,1]
// (visible window, foreground window, relation to the launch). Its score is a
// time-weighted EMA of that evidence, so it stays in [0,1], does not depend on
// the sample rate, and favours recent evidence (a closed splash fades out).
//
// The early-commit tiers are anchored to absolute times rather than to the
// configurable timeout: the timeout bounds the slowest launchers, while normal
// games settle within seconds. Between strictCommitPhase and the timeout the
// commit threshold decays linearly, so a longer timeout only stretches the
// relaxed tail.
const (
	detectionSampleInterval    = 500 * time.Millisecond
	verificationSampleInterval = time.Second

	scoreTimeConstant       = 2 * time.Second
	minDetectionObservation = 3 * time.Second
	minCandidateAge         = 3 * time.Second
	strictCommitPhase       = 15 * time.Second
	strictCommitThreshold   = 0.85
	relaxedCommitThreshold  = 0.55
	commitMargin            = 0.25
	fallbackCommitThreshold = 0.4
	// soleSurvivorAge commits the only process left after the launcher exited
	// even without a window (games that are still loading).
	soleSurvivorAge = 10 * time.Second

	retargetMinScore         = 0.6
	retargetMargin           = 0.3
	retargetSustain          = 5 * time.Second
	maxVerificationRetargets = 3
)

const (
	windowEvidence     = 0.35
	foregroundEvidence = 0.40
	// Relation evidence: only the strongest applicable relation counts.
	descendantInDirEvidence = 0.25
	savedNameEvidence       = 0.25
	launcherEvidence        = 0.20
	launchDirEvidence       = 0.15
	// A descendant outside the game directory may be the real game unpacked
	// elsewhere, but it may also be a browser opened by the launcher. Keep it
	// below the strict threshold so it only commits in the relaxed phase.
	descendantEvidence = 0.10
)

// candidateSample is what a platform sampler observed about one process in a
// single tick.
type candidateSample struct {
	Process     processutils.ProcessInfo
	IsLauncher  bool
	Descendant  bool
	InLaunchDir bool
	SavedName   bool
	Window      bool
	Foreground  bool
}

func (c candidateSample) evidence() float64 {
	value := 0.0
	if c.Window {
		value += windowEvidence
	}
	if c.Foreground {
		value += foregroundEvidence
	}
	switch {
	case c.Descendant && c.InLaunchDir:
		value += descendantInDirEvidence
	case c.SavedName:
		value += savedNameEvidence
	case c.IsLauncher:
		value += launcherEvidence
	case c.InLaunchDir:
		value += launchDirEvidence
	case c.Descendant:
		value += descendantEvidence
	}
	return value
}

type scoredCandidate struct {
	sample    candidateSample
	score     float64
	firstSeen time.Duration
	// challengeSince marks when this candidate started beating the verified
	// target; negative when it is not challenging.
	challengeSince time.Duration
}

type processScorer struct {
	timeout       time.Duration
	at            time.Duration
	observed      bool
	launcherAlive bool
	candidates    map[uint32]*scoredCandidate
}

func newProcessScorer(timeout time.Duration) *processScorer {
	return &processScorer{
		timeout:    timeout,
		candidates: make(map[uint32]*scoredCandidate),
	}
}

// observe folds one tick of samples into the scores. at is the elapsed time
// since detection started; candidates missing from samples have exited.
func (s *processScorer) observe(at time.Duration, launcherAlive bool, samples []candidateSample) {
	dt := at - s.at
	if !s.observed || dt <= 0 {
		dt = detectionSampleInterval
	}
	alpha := 1 - math.Exp(-dt.Seconds()/scoreTimeConstant.Seconds())

	present := make(map[uint32]bool, len(samples))
	for _, sample := range samples {
		pid := sample.Process.PID
		if pid == 0 || present[pid] {
			continue
		}
		present[pid] = true
		candidate, ok := s.candidates[pid]
		if !ok || !strings.EqualFold(candidate.sample.Process.Name, sample.Process.Name) {
			candidate = &scoredCandidate{firstSeen: at, challengeSince: -1}
			s.candidates[pid] = candidate
		}
		candidate.sample = sample
		candidate.score += alpha * (sample.evidence() - candidate.score)
	}
	for pid := range s.candidates {
		if !present[pid] {
			delete(s.candidates, pid)
		}
	}

	s.at = at
	s.observed = true
	s.launcherAlive = launcherAlive
}

// ranked returns candidates by descending score, PID as a stable tie-breaker.
func (s *processScorer) ranked() []*scoredCandidate {
	ranked := make([]*scoredCandidate, 0, len(s.candidates))
	for _, candidate := range s.candidates {
		ranked = append(ranked, candidate)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].sample.Process.PID < ranked[j].sample.Process.PID
	})
	return ranked
}

func commitThreshold(at time.Duration, timeout time.Duration) float64 {
	if at < strictCommitPhase || timeout <= strictCommitPhase {
		return strictCommitThreshold
	}
	if at >= timeout {
		return relaxedCommitThreshold
	}
	progress := float64(at-strictCommitPhase) / float64(timeout-strictCommitPhase)
	return strictCommitThreshold - (strictCommitThreshold-relaxedCommitThreshold)*progress
}

// decide returns a candidate once the evidence is strong enough to commit
// before the detection deadline.
func (s *processScorer) decide() (candidateSample, bool) {
	if !s.observed || s.at < minDetectionObservation {
		return candidateSample{}, false
	}
	ranked := s.ranked()
	if len(ranked) == 0 {
		return candidateSample{}, false
	}

	leader := ranked[0]
	leaderAge := s.at - leader.firstSeen
	if leaderAge >= minCandidateAge {
		runnerUp := 0.0
		if len(ranked) > 1 {
			runnerUp = ranked[1].score
		}
		if leader.score >= commitThreshold(s.at, s.timeout) && leader.score-runnerUp >= commitMargin {
			return leader.sample, true
		}
	}

	if !s.launcherAlive && len(ranked) == 1 && !leader.sample.IsLauncher &&
		leaderAge >= soleSurvivorAge && IsPersistableProcessName(leader.sample.Process.Name) {
		return leader.sample, true
	}
	return candidateSample{}, false
}

// fallback picks the best candidate at the deadline when it still carries
// meaningful evidence and is not tied.
func (s *processScorer) fallback() (candidateSample, bool) {
	ranked := s.ranked()
	if len(ranked) == 0 || ranked[0].score < fallbackCommitThreshold {
		return candidateSample{}, false
	}
	if len(ranked) > 1 && ranked[1].score >= ranked[0].score {
		return candidateSample{}, false
	}
	return ranked[0].sample, true
}

// soleStableCandidate returns the only remaining non-launcher candidate when
// it has a persistable name, mirroring the pre-scoring timeout behaviour.
func (s *processScorer) soleStableCandidate() (candidateSample, bool) {
	var found *scoredCandidate
	for _, candidate := range s.candidates {
		if candidate.sample.IsLauncher {
			continue
		}
		if found != nil {
			return candidateSample{}, false
		}
		found = candidate
	}
	if found == nil || !IsPersistableProcessName(found.sample.Process.Name) {
		return candidateSample{}, false
	}
	return found.sample, true
}

// challenger looks for a process that has clearly taken over from the
// committed target during verification. Only processes first seen no earlier
// than the target qualify, so verification moves forward along the launch
// chain (launcher/settings window → game) and never back to an older window
// the user merely clicked on. When the target is gone, successor detection owns
// the hand-off and no challenger is reported.
func (s *processScorer) challenger(targetPID uint32) (candidateSample, bool) {
	target, ok := s.candidates[targetPID]
	if !ok {
		for _, candidate := range s.candidates {
			candidate.challengeSince = -1
		}
		return candidateSample{}, false
	}

	var best *scoredCandidate
	for pid, candidate := range s.candidates {
		if pid == targetPID {
			continue
		}
		qualifies := !candidate.sample.IsLauncher &&
			candidate.firstSeen >= target.firstSeen &&
			s.at-candidate.firstSeen >= minCandidateAge &&
			candidate.score >= retargetMinScore &&
			candidate.score-target.score >= retargetMargin
		if !qualifies {
			candidate.challengeSince = -1
			continue
		}
		if candidate.challengeSince < 0 {
			candidate.challengeSince = s.at
		}
		if s.at-candidate.challengeSince >= retargetSustain && (best == nil || candidate.score > best.score) {
			best = candidate
		}
	}
	if best == nil {
		return candidateSample{}, false
	}
	for _, candidate := range s.candidates {
		candidate.challengeSince = -1
	}
	return best.sample, true
}

func (s *processScorer) score(pid uint32) float64 {
	if candidate, ok := s.candidates[pid]; ok {
		return candidate.score
	}
	return 0
}

func (s *processScorer) summary(limit int) string {
	ranked := s.ranked()
	if len(ranked) == 0 {
		return "(none)"
	}
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	parts := make([]string, 0, len(ranked))
	for _, candidate := range ranked {
		parts = append(parts, fmt.Sprintf("%s(PID %d, score %.2f)", candidate.sample.Process.Name, candidate.sample.Process.PID, candidate.score))
	}
	return strings.Join(parts, ", ")
}

// ProcessVerifier keeps scoring candidates after an early commit until the
// detection deadline and reports a process that clearly took over. The caller
// switches monitoring on each value from Retargets and reports successor
// hand-offs through SetTarget so verification follows the monitored process.
type ProcessVerifier struct {
	mu        sync.Mutex
	target    uint32
	retargets chan processutils.ProcessInfo
}

func newProcessVerifier(target uint32) *ProcessVerifier {
	return &ProcessVerifier{
		target:    target,
		retargets: make(chan processutils.ProcessInfo, 1),
	}
}

// Retargets is never closed; a nil verifier yields a nil channel that blocks.
func (v *ProcessVerifier) Retargets() <-chan processutils.ProcessInfo {
	if v == nil {
		return nil
	}
	return v.retargets
}

func (v *ProcessVerifier) SetTarget(pid uint32) {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.target = pid
	v.mu.Unlock()
}

func (v *ProcessVerifier) Target() uint32 {
	if v == nil {
		return 0
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.target
}

// offer publishes a new target, replacing an unconsumed older one.
func (v *ProcessVerifier) offer(proc processutils.ProcessInfo) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.target = proc.PID
	select {
	case <-v.retargets:
	default:
	}
	select {
	case v.retargets <- proc:
	default:
	}
}
