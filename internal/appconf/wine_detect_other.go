//go:build !darwin && !linux

package appconf

func detectDefaultWineRunnerPath(config *AppConfig) bool {
	return false
}
