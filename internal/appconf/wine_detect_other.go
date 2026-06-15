//go:build !darwin

package appconf

func detectDefaultWineRunnerPath(config *AppConfig) bool {
	return false
}
