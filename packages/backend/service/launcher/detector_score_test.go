package launcher

import (
	"lunabox/backend/utils/processutils"
	"math"
	"testing"
	"time"
)

const testDetectionTimeout = 60 * time.Second

type timelineFunc func(at time.Duration) (samples []candidateSample, launcherAlive bool)

// runDetection replays a timeline at the detection sample rate and returns the
// first committed candidate with its commit time.
func runDetection(t *testing.T, scorer *processScorer, until time.Duration, timeline timelineFunc) (candidateSample, time.Duration, bool) {
	t.Helper()
	for at := time.Duration(0); at <= until; at += detectionSampleInterval {
		samples, launcherAlive := timeline(at)
		scorer.observe(at, launcherAlive, samples)
		if picked, ok := scorer.decide(); ok {
			return picked, at, true
		}
	}
	return candidateSample{}, 0, false
}

func proc(name string, pid uint32) processutils.ProcessInfo {
	return processutils.ProcessInfo{Name: name, PID: pid}
}

func TestScorerCommitsLauncherThatIsTheGameWithinSeconds(t *testing.T) {
	launcher := proc("game.exe", 100)
	scorer := newProcessScorer(testDetectionTimeout)
	picked, at, ok := runDetection(t, scorer, testDetectionTimeout, func(at time.Duration) ([]candidateSample, bool) {
		sample := candidateSample{Process: launcher, IsLauncher: true, InLaunchDir: true}
		if at >= time.Second {
			sample.Window = true
			sample.Foreground = true
		}
		return []candidateSample{sample}, true
	})
	if !ok || picked.Process.PID != launcher.PID {
		t.Fatalf("expected launcher to be committed, got %+v ok=%v", picked, ok)
	}
	if at < minDetectionObservation || at > 8*time.Second {
		t.Fatalf("expected commit within a few seconds, got %s", at)
	}
}

func TestScorerPrefersRealGameOverSameNameSplash(t *testing.T) {
	splash := proc("game.exe", 100)
	game := proc("game.exe", 200)
	scorer := newProcessScorer(testDetectionTimeout)
	picked, _, ok := runDetection(t, scorer, testDetectionTimeout, func(at time.Duration) ([]candidateSample, bool) {
		if at < 3*time.Second {
			return []candidateSample{{Process: splash, IsLauncher: true, InLaunchDir: true, Window: at >= 500*time.Millisecond, Foreground: at >= 500*time.Millisecond}}, true
		}
		return []candidateSample{{Process: game, Descendant: true, InLaunchDir: true, Window: true, Foreground: true}}, false
	})
	if !ok || picked.Process.PID != game.PID {
		t.Fatalf("expected re-executed game process to be committed, got %+v ok=%v", picked, ok)
	}
}

func TestScorerWaitsOutQuickLauncherExit(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	scorer.observe(0, true, []candidateSample{{Process: proc("launcher.exe", 100), IsLauncher: true, Window: true, Foreground: true}})
	scorer.observe(time.Second, true, []candidateSample{{Process: proc("launcher.exe", 100), IsLauncher: true, Window: true, Foreground: true}})
	if _, ok := scorer.decide(); ok {
		t.Fatal("expected no commit before the minimum observation time")
	}
}

func TestScorerDoesNotCommitAmbiguousBackgroundWindows(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	_, _, ok := runDetection(t, scorer, testDetectionTimeout, func(time.Duration) ([]candidateSample, bool) {
		return []candidateSample{
			{Process: proc("a.exe", 200), Descendant: true, InLaunchDir: true, Window: true},
			{Process: proc("b.exe", 300), Descendant: true, InLaunchDir: true, Window: true},
		}, true
	})
	if ok {
		t.Fatal("expected two equally strong candidates to stay undecided")
	}
	if _, ok := scorer.fallback(); ok {
		t.Fatal("expected tied candidates to have no fallback winner")
	}
}

func TestScorerOnlyCommitsOutOfDirDescendantInRelaxedPhase(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	picked, at, ok := runDetection(t, scorer, testDetectionTimeout, func(time.Duration) ([]candidateSample, bool) {
		return []candidateSample{{Process: proc("browser.exe", 200), Descendant: true, Window: true, Foreground: true}}, true
	})
	if !ok || picked.Process.PID != 200 {
		t.Fatalf("expected out-of-dir descendant to commit eventually, got %+v ok=%v", picked, ok)
	}
	if at < strictCommitPhase {
		t.Fatalf("expected out-of-dir descendant to wait for the relaxed phase, committed at %s", at)
	}
}

func TestScorerCommitsSoleSurvivorWithoutWindowAfterLauncherExit(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	picked, at, ok := runDetection(t, scorer, testDetectionTimeout, func(at time.Duration) ([]candidateSample, bool) {
		if at < 2*time.Second {
			return []candidateSample{{Process: proc("launcher.exe", 100), IsLauncher: true}}, true
		}
		return []candidateSample{{Process: proc("game.exe", 200), Descendant: true, InLaunchDir: true}}, false
	})
	if !ok || picked.Process.PID != 200 {
		t.Fatalf("expected sole surviving process to commit, got %+v ok=%v", picked, ok)
	}
	if at < 2*time.Second+soleSurvivorAge {
		t.Fatalf("expected sole survivor to be stable for %s first, committed at %s", soleSurvivorAge, at)
	}
}

func TestScorerScoreIsBoundedAndSampleRateIndependent(t *testing.T) {
	sample := []candidateSample{{Process: proc("game.exe", 100), IsLauncher: true, Window: true, Foreground: true}}
	scoreAt := func(step time.Duration) float64 {
		scorer := newProcessScorer(testDetectionTimeout)
		for at := step; at <= 10*time.Minute; at += step {
			scorer.observe(at, true, sample)
		}
		return scorer.score(100)
	}
	fast, slow := scoreAt(100*time.Millisecond), scoreAt(2*time.Second)
	if fast > 1 || slow > 1 {
		t.Fatalf("expected bounded scores, got fast=%f slow=%f", fast, slow)
	}
	if math.Abs(fast-slow) > 0.01 {
		t.Fatalf("expected sample-rate independent scores, got fast=%f slow=%f", fast, slow)
	}

	early := func(step time.Duration) float64 {
		scorer := newProcessScorer(testDetectionTimeout)
		scorer.observe(0, true, nil)
		for at := step; at <= 4*time.Second; at += step {
			scorer.observe(at, true, sample)
		}
		return scorer.score(100)
	}
	if diff := math.Abs(early(250*time.Millisecond) - early(time.Second)); diff > 0.01 {
		t.Fatalf("expected rising score to be sample-rate independent, diff=%f", diff)
	}
}

func TestCommitThresholdDecaysTowardsConfiguredTimeout(t *testing.T) {
	timeout := 10 * time.Minute
	if got := commitThreshold(10*time.Second, timeout); got != strictCommitThreshold {
		t.Fatalf("expected strict threshold early, got %f", got)
	}
	if got := commitThreshold(timeout, timeout); got != relaxedCommitThreshold {
		t.Fatalf("expected relaxed threshold at timeout, got %f", got)
	}
	mid := commitThreshold(strictCommitPhase+(timeout-strictCommitPhase)/2, timeout)
	want := (strictCommitThreshold + relaxedCommitThreshold) / 2
	if math.Abs(mid-want) > 1e-9 {
		t.Fatalf("expected linear decay midpoint %f, got %f", want, mid)
	}
	if got := commitThreshold(20*time.Second, 5*time.Second); got != strictCommitThreshold {
		t.Fatalf("expected strict threshold when timeout is shorter than the strict phase, got %f", got)
	}
}

func TestChallengerRetargetsFromLauncherThatStaysOpen(t *testing.T) {
	launcher := proc("config.exe", 100)
	game := proc("game.exe", 200)
	scorer := newProcessScorer(testDetectionTimeout)
	committed, commitAt, ok := runDetection(t, scorer, testDetectionTimeout, func(time.Duration) ([]candidateSample, bool) {
		return []candidateSample{{Process: launcher, IsLauncher: true, InLaunchDir: true, Window: true, Foreground: true}}, true
	})
	if !ok || committed.Process.PID != launcher.PID {
		t.Fatalf("expected settings launcher to be committed first, got %+v ok=%v", committed, ok)
	}

	gameStart := 20 * time.Second
	var retargetAt time.Duration
	for at := commitAt + verificationSampleInterval; at <= testDetectionTimeout; at += verificationSampleInterval {
		samples := []candidateSample{{Process: launcher, IsLauncher: true, InLaunchDir: true, Window: true, Foreground: at < gameStart}}
		if at >= gameStart {
			samples = append(samples, candidateSample{Process: game, Descendant: true, InLaunchDir: true, Window: true, Foreground: true})
		}
		scorer.observe(at, true, samples)
		if next, ok := scorer.challenger(launcher.PID); ok {
			if next.Process.PID != game.PID {
				t.Fatalf("expected game to take over, got %+v", next)
			}
			retargetAt = at
			break
		}
	}
	if retargetAt == 0 {
		t.Fatal("expected verification to retarget to the game process")
	}
	if retargetAt < gameStart+retargetSustain {
		t.Fatalf("expected challenger to be sustained for %s, retargeted at %s", retargetAt, retargetSustain)
	}
}

func TestChallengerNeverMovesBackToOlderProcess(t *testing.T) {
	settings := proc("settings.exe", 150)
	game := proc("game.exe", 200)
	scorer := newProcessScorer(testDetectionTimeout)
	for at := time.Duration(0); at <= testDetectionTimeout; at += verificationSampleInterval {
		gameFocused := at < 10*time.Second
		samples := []candidateSample{{Process: settings, Descendant: true, InLaunchDir: true, Window: true, Foreground: !gameFocused}}
		if at >= 2*time.Second {
			samples = append(samples, candidateSample{Process: game, Descendant: true, InLaunchDir: true, Window: true, Foreground: gameFocused})
		}
		scorer.observe(at, true, samples)
		if next, ok := scorer.challenger(game.PID); ok {
			t.Fatalf("expected no retarget away from the game when the user clicks the launcher, got %+v at %s", next, at)
		}
	}
}

func TestChallengerIgnoresExitedTarget(t *testing.T) {
	scorer := newProcessScorer(testDetectionTimeout)
	for at := time.Duration(0); at <= 20*time.Second; at += verificationSampleInterval {
		scorer.observe(at, false, []candidateSample{{Process: proc("game.exe", 200), Descendant: true, InLaunchDir: true, Window: true, Foreground: true}})
		if _, ok := scorer.challenger(100); ok {
			t.Fatal("expected successor detection, not verification, to handle an exited target")
		}
	}
}

func TestProcessVerifierOfferReplacesUnconsumedTarget(t *testing.T) {
	verifier := newProcessVerifier(100)
	verifier.offer(proc("a.exe", 200))
	verifier.offer(proc("b.exe", 300))
	if got := verifier.Target(); got != 300 {
		t.Fatalf("expected latest target, got %d", got)
	}
	select {
	case next := <-verifier.Retargets():
		if next.PID != 300 {
			t.Fatalf("expected latest offer, got %+v", next)
		}
	default:
		t.Fatal("expected a pending retarget")
	}

	var nilVerifier *ProcessVerifier
	nilVerifier.SetTarget(1)
	if nilVerifier.Retargets() != nil || nilVerifier.Target() != 0 {
		t.Fatal("expected nil verifier to be inert")
	}
}
