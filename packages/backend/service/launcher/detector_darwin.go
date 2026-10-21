//go:build darwin

package launcher

import (
	"lunabox/backend/utils/processutils"
	"lunabox/backend/utils/timerutils/focusing"
	"os"
	"strings"
)

var darwinSteamProcessLookup = processutils.GetProcessesByExecutableDir

// DetectStagedProcess keeps native macOS launch strategies on launcher-only
// monitoring. Steam uses its dedicated install-directory handoff below.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	logInfo(logger, "Staged process detection is not supported on macOS for game %s, using launcher process: %s (PID %d)", input.GameID, input.Launcher.Name, input.Launcher.PID)
	return resultForLauncher(input)
}

// DetectSteamDirectoryProcess resolves the game Steam started from the install
// directory. macOS has no successor detection, so the background verification
// is the only way to correct an early pick (e.g. a launcher in the game dir).
func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	return detectScoredProcess(input, scoredDetection{
		label:            "macOS Steam directory",
		steam:            true,
		launcherFallback: true,
		sampler:          newDarwinSteamSampler(input),
	}, logger)
}

// darwinSteamSampler scores processes running from the Steam install
// directory. macOS has no window enumeration; the frontmost application PID
// is the foreground channel.
type darwinSteamSampler struct {
	input   StagedProcessDetectionInput
	selfPID uint32
}

func newDarwinSteamSampler(input StagedProcessDetectionInput) *darwinSteamSampler {
	return &darwinSteamSampler{input: input, selfPID: uint32(os.Getpid())}
}

func (s *darwinSteamSampler) sample() ([]candidateSample, error) {
	processes, err := darwinSteamProcessLookup(s.input.LaunchDir)
	if err != nil {
		return nil, err
	}
	foregroundPID, foregroundKnown := focusing.GetForegroundProcessID()
	return darwinSteamSamples(processes, s.input.SavedProcessName, s.selfPID, foregroundPID, foregroundKnown), nil
}

func darwinSteamSamples(processes []processutils.ProcessInfo, savedProcessName string, selfPID uint32, foregroundPID uint32, foregroundKnown bool) []candidateSample {
	savedName := strings.TrimSpace(savedProcessName)
	samples := make([]candidateSample, 0, len(processes))
	for _, proc := range processes {
		if proc.PID == 0 || proc.PID == selfPID || strings.EqualFold(proc.Name, "steam_osx") || IsLikelyHelperProcess(proc.Name) {
			continue
		}
		links := processLinks{
			InLaunchDir:  true,
			DirIsPrimary: true,
			SavedName:    savedName != "" && strings.EqualFold(proc.Name, savedName),
		}
		samples = append(samples, candidateSample{
			Process:         proc,
			SavedName:       links.SavedName,
			Relation:        links.relation(),
			Foreground:      foregroundKnown && proc.PID == foregroundPID,
			ForegroundKnown: foregroundKnown,
		})
	}
	return samples
}

func DetectSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	return processutils.ProcessInfo{}, false
}
