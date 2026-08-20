//go:build !linux

package launcher

// StartExitWatch reports that game-process presence monitoring is unavailable
// on platforms whose launch strategies do not request it.
func StartExitWatch(input ExitWatchInput, logger DetectionLogger) (<-chan struct{}, bool) {
	return nil, false
}
