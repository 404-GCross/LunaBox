package launcher

import (
	"fmt"
	"lunabox/internal/utils/processutils"
	"runtime"
	"strings"
)

type DetectionLogger interface {
	Infof(format string, args ...any)
	Warningf(format string, args ...any)
}

type LaunchedProcessInfo struct {
	PID  uint32
	Name string
}

type StagedProcessDetectionInput struct {
	GameID                string
	Launcher              LaunchedProcessInfo
	LauncherExeName       string
	LaunchDir             string
	SavedProcessName      string
	AutoDetectGameProcess bool
}

type StagedProcessDetectionResult struct {
	ProcessID               uint32
	ProcessName             string
	UseLauncherHandle       bool
	CloseLauncherHandle     bool
	RequireProcessSelection bool
	PersistProcessName      string
}

func resultForLauncher(input StagedProcessDetectionInput) StagedProcessDetectionResult {
	result := StagedProcessDetectionResult{
		ProcessID:         input.Launcher.PID,
		ProcessName:       input.Launcher.Name,
		UseLauncherHandle: true,
	}
	if ShouldPersistLauncherProcessName(input.SavedProcessName) {
		result.PersistProcessName = strings.TrimSpace(input.LauncherExeName)
	}
	return result
}

func resultForExternalProcess(input StagedProcessDetectionInput, proc processutils.ProcessInfo, closeLauncher bool) StagedProcessDetectionResult {
	return StagedProcessDetectionResult{
		ProcessID:           proc.PID,
		ProcessName:         proc.Name,
		CloseLauncherHandle: closeLauncher,
		PersistProcessName:  ProcessNameForPersistence(input.LauncherExeName, proc.Name),
	}
}

func promptProcessSelectionResult() StagedProcessDetectionResult {
	return StagedProcessDetectionResult{
		CloseLauncherHandle:     true,
		RequireProcessSelection: true,
	}
}

func HasReliableSavedProcessName(savedProcessName string, launcherExeName string) bool {
	saved := strings.TrimSpace(savedProcessName)
	if saved == "" || strings.EqualFold(saved, strings.TrimSpace(launcherExeName)) {
		return false
	}
	return IsPersistableProcessName(saved)
}

func ShouldPersistLauncherProcessName(savedProcessName string) bool {
	saved := strings.TrimSpace(savedProcessName)
	return saved == "" || !IsPersistableProcessName(saved)
}

func ProcessNameForPersistence(launcherExeName string, detectedProcessName string) string {
	detected := strings.TrimSpace(detectedProcessName)
	if IsPersistableProcessName(detected) {
		return detected
	}
	launcher := strings.TrimSpace(launcherExeName)
	if IsPersistableProcessName(launcher) {
		return launcher
	}
	return ""
}

func IsPersistableProcessName(processName string) bool {
	name := strings.ToLower(strings.TrimSpace(processName))
	if name == "" || IsLikelyHelperProcess(name) {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.HasSuffix(name, ".exe")
	}
	return true
}

func IsLikelyHelperProcess(processName string) bool {
	name := strings.ToLower(strings.TrimSpace(processName))
	if name == "" {
		return true
	}
	switch name {
	case "conhost.exe",
		"crashpad_handler.exe",
		"crashreporter.exe",
		"cef_server.exe",
		"cefsharp.browsersubprocess.exe",
		"steam.exe",
		"werfault.exe",
		"crashpad_handler",
		"crashreporter",
		"plugin-container":
		return true
	default:
		return strings.Contains(name, " helper") ||
			strings.Contains(name, "helper (") ||
			strings.Contains(name, "crashpad") ||
			strings.Contains(name, "crash reporter")
	}
}

func FormatProcessCandidates(processes []processutils.ProcessInfo) string {
	if len(processes) == 0 {
		return "(none)"
	}

	parts := make([]string, 0, len(processes))
	for _, proc := range processes {
		parts = append(parts, fmt.Sprintf("%s(PID %d)", proc.Name, proc.PID))
	}
	return strings.Join(parts, ", ")
}

func logInfo(logger DetectionLogger, format string, args ...any) {
	if logger != nil {
		logger.Infof(format, args...)
	}
}

func logWarning(logger DetectionLogger, format string, args ...any) {
	if logger != nil {
		logger.Warningf(format, args...)
	}
}
