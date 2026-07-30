//go:build !darwin

package appconf

func detectDefaultCrossOverRunnerPath(config *AppConfig) bool {
	return false
}
