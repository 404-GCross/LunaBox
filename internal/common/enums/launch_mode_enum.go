package enums

type LaunchMode string

const (
	LaunchModeNormal LaunchMode = "normal"
	LaunchModeSteam  LaunchMode = "steam"
)

var AllLaunchModes = []struct {
	Value  LaunchMode
	TSName string
}{
	{LaunchModeNormal, "NORMAL"},
	{LaunchModeSteam, "STEAM"},
}

func NormalizeLaunchMode(mode LaunchMode) LaunchMode {
	switch mode {
	case LaunchModeSteam:
		return LaunchModeSteam
	default:
		return LaunchModeNormal
	}
}
