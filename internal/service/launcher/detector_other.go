//go:build !windows

package launcher

// DetectStagedProcess keeps non-Windows staged detection conservative.
// macOS launch strategies normally use DetectionLauncherOnly; if a staged plan
// reaches this fallback, monitor the launcher PID directly.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	logInfo(logger, "Staged process detection is not supported on this platform for game %s, using launcher process: %s (PID %d)", input.GameID, input.Launcher.Name, input.Launcher.PID)
	return resultForLauncher(input)
}
