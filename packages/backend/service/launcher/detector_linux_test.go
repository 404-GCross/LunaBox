//go:build linux

package launcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"lunabox/backend/utils/processutils"
)

// pickTopLinuxSample returns the candidate with the strongest unique relation.
func pickTopLinuxSample(input StagedProcessDetectionInput, candidates []linuxProcessCandidate) (processutils.ProcessInfo, bool) {
	samples := linuxCandidateSamples(input, candidates)
	var best candidateSample
	tied := false
	for _, sample := range samples {
		switch {
		case sample.Relation > best.Relation:
			best, tied = sample, false
		case sample.Relation == best.Relation:
			tied = true
		}
	}
	if best.Process.PID == 0 || tied {
		return processutils.ProcessInfo{}, false
	}
	return best.Process, true
}

func TestLinuxNameMatchedWineProcessWinsByCommitMargin(t *testing.T) {
	launchDir := t.TempDir()
	input := StagedProcessDetectionInput{
		GameID:          "game",
		Launcher:        LaunchedProcessInfo{PID: 100, Name: "game.exe"},
		LauncherExeName: "wine",
		LaunchDir:       launchDir,
	}
	candidates := []linuxProcessCandidate{
		{
			detail: processutils.ProcessDetails{
				ProcessInfo:      processutils.ProcessInfo{Name: "game.exe", PID: 201},
				CurrentDirectory: launchDir,
				CommandLine:      []string{filepath.Join(launchDir, "game.exe")},
			},
			fromDescendant: true,
			fromDirectory:  true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo:      processutils.ProcessInfo{Name: "updater.exe", PID: 202},
				CurrentDirectory: launchDir,
				CommandLine:      []string{filepath.Join(launchDir, "updater.exe")},
			},
			fromDescendant: true,
			fromDirectory:  true,
		},
	}

	samples := linuxCandidateSamples(input, candidates)
	if len(samples) != 2 {
		t.Fatalf("expected both processes as candidates, got %+v", samples)
	}
	relations := map[uint32]float64{}
	for _, sample := range samples {
		relations[sample.Process.PID] = sample.Relation
		if sample.WindowKnown {
			t.Fatalf("expected Linux samples to have no window channel, got %+v", sample)
		}
	}
	if relations[201] != 1 || relations[201]-relations[202] < commitMargin {
		t.Fatalf("expected name-matched game to lead by the commit margin, got %+v", relations)
	}

	scorer := newProcessScorer(testDetectionTimeout)
	picked, at, ok := runDetection(t, scorer, testDetectionTimeout, func(time.Duration) []candidateSample {
		return samples
	})
	if !ok || picked.Process.PID != 201 {
		t.Fatalf("expected name-matched game to commit, got %+v ok=%v", picked, ok)
	}
	if at > 8*time.Second {
		t.Fatalf("expected Linux commit within seconds without window evidence, got %s", at)
	}
}

func TestMarkLinuxSavedProcessAddsOrUpgradesCandidate(t *testing.T) {
	samples := []candidateSample{{Process: processutils.ProcessInfo{Name: "game.exe", PID: 201}, Relation: 0.5}}
	samples = markLinuxSavedProcess(samples, processutils.ProcessInfo{Name: "game.exe", PID: 201})
	if len(samples) != 1 || !samples[0].SavedName || samples[0].Relation != relationSavedName {
		t.Fatalf("expected existing candidate to be upgraded, got %+v", samples)
	}
	samples = markLinuxSavedProcess(samples, processutils.ProcessInfo{Name: "other.exe", PID: 300})
	if len(samples) != 2 || !samples[1].SavedName {
		t.Fatalf("expected saved process to be appended, got %+v", samples)
	}
}

func TestLinuxProcessCandidatePrefersTruncatedGameCommFromProtonTree(t *testing.T) {
	gamePath := "/home/u/Games/Escu/haison_fd2_gemini2.5pro.exe"
	input := StagedProcessDetectionInput{
		GameID:          "game",
		Launcher:        LaunchedProcessInfo{PID: 100, Name: "steam"},
		LauncherExeName: "steam",
		LaunchDir:       "/home/u/Games/Escu",
	}
	candidates := []linuxProcessCandidate{
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "pv-adverb", PID: 201},
				CommandLine: []string{"/home/u/.steam/steamapps/common/Proton/proton", "waitforexitandrun", gamePath},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "python3", PID: 202},
				CommandLine: []string{"/home/u/.steam/steamapps/common/Proton 11.0/proton", "waitforexitandrun", gamePath},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "steam.exe", PID: 203},
				CommandLine: []string{gamePath},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "xalia.exe", PID: 204},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "haison_fd2_gemi", PID: 205},
			},
			fromDescendant: true,
		},
	}

	proc, ok := pickTopLinuxSample(input, candidates)
	if !ok {
		t.Fatal("expected Linux detector to select the truncated game process")
	}
	if proc.PID != 205 || proc.Name != "haison_fd2_gemi" {
		t.Fatalf("expected game process PID 205, got %+v", proc)
	}
}

func TestLinuxProcessCandidateMatchesWineLauncherDisplayNameTruncation(t *testing.T) {
	input := StagedProcessDetectionInput{
		GameID:          "game",
		Launcher:        LaunchedProcessInfo{PID: 100, Name: "haison_fd2_gemini2.5pro.exe"},
		LauncherExeName: "wine",
		LaunchDir:       "/home/u/Games/Escu",
	}
	candidates := []linuxProcessCandidate{
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "wineserver", PID: 201},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "haison_fd2_gemi", PID: 202},
			},
			fromDescendant: true,
		},
	}

	proc, ok := pickTopLinuxSample(input, candidates)
	if !ok {
		t.Fatal("expected Wine detector to select the truncated child process")
	}
	if proc.PID != 202 {
		t.Fatalf("expected game process PID 202, got %+v", proc)
	}
}

func TestLinuxPathUnderDirResolvesSteamDirectorySymlink(t *testing.T) {
	realSteamRoot := filepath.Join(t.TempDir(), "Steam")
	realGameDir := filepath.Join(realSteamRoot, "steamapps", "common", "SlayTheSpire")
	if err := os.MkdirAll(realGameDir, 0o755); err != nil {
		t.Fatalf("create real Steam game directory: %v", err)
	}
	gameExecutable := filepath.Join(realGameDir, "SlayTheSpire")
	if err := os.WriteFile(gameExecutable, []byte("game"), 0o755); err != nil {
		t.Fatalf("create game executable: %v", err)
	}

	aliasRoot := filepath.Join(t.TempDir(), "steam")
	if err := os.Symlink(realSteamRoot, aliasRoot); err != nil {
		t.Fatalf("create Steam directory symlink: %v", err)
	}
	aliasGameDir := filepath.Join(aliasRoot, "steamapps", "common", "SlayTheSpire")

	if !linuxPathUnderDir(gameExecutable, aliasGameDir) {
		t.Fatalf("expected %q to match symlinked root %q", gameExecutable, aliasGameDir)
	}
}

func TestLinuxSuccessorCandidatesRejectProtonPythonWrapper(t *testing.T) {
	gamePath := "/home/u/Games/Escu/totsulover_chs.exe"
	input := SuccessorDetectionInput{
		GameID:            "game",
		ExitedPID:         205,
		ExitedProcessName: "totsulover.exe",
		LaunchDir:         "/home/u/Games/Escu",
	}
	details := []processutils.ProcessDetails{
		{
			ProcessInfo: processutils.ProcessInfo{Name: "python3", PID: 202},
			CommandLine: []string{"python3", "/home/u/.steam/steamapps/common/Proton 11.0/proton", "waitforexitandrun", gamePath},
		},
		{
			ProcessInfo: processutils.ProcessInfo{Name: "totsulover.exe", PID: 205},
			CommandLine: []string{"totsulover.exe"},
		},
	}

	candidates := filterSuccessorProcessDetails(details, input, nil)
	if len(candidates) != 0 {
		t.Fatalf("expected Proton python wrapper to be rejected, got %+v", candidates)
	}
}

func TestLinuxProcessCandidateRejectsSteamRuntimeHelper(t *testing.T) {
	input := StagedProcessDetectionInput{
		GameID:          "game",
		Launcher:        LaunchedProcessInfo{PID: 100, Name: "steam"},
		LauncherExeName: "steam",
		LaunchDir:       "/home/u/.steam/steam/steamapps/common/Aokana",
	}
	candidates := []linuxProcessCandidate{
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "x86_64-linux-gn", PID: 201},
				ExecutablePath: "/home/u/.local/share/Steam/ubuntu12_32/steam-runtime.old" +
					"/usr/libexec/steam-runtime-tools-0/x86_64-linux-gnu/x86_64-linux-gnu",
				CurrentDirectory: input.LaunchDir,
			},
			fromDirectory: true,
		},
	}

	if proc, ok := pickTopLinuxSample(input, candidates); ok {
		t.Fatalf("expected Steam runtime helper to be rejected, got %+v", proc)
	}
}
