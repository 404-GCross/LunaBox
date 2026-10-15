package enums

type LaunchMode string

const (
	LaunchModeNormal        LaunchMode = "normal"
	LaunchModeAdmin         LaunchMode = "admin"
	LaunchModeSteam         LaunchMode = "steam"
	LaunchModeCompatibility LaunchMode = "compatibility"
)

var AllLaunchModes = []struct {
	Value  LaunchMode
	TSName string
}{
	{LaunchModeNormal, "NORMAL"},
	{LaunchModeAdmin, "ADMIN"},
	{LaunchModeSteam, "STEAM"},
	{LaunchModeCompatibility, "COMPATIBILITY"},
}

func NormalizeLaunchMode(mode LaunchMode) LaunchMode {
	switch mode {
	case LaunchModeAdmin, LaunchModeSteam, LaunchModeCompatibility:
		return mode
	default:
		return LaunchModeNormal
	}
}
