//go:build darwin && cgo

package audioutils

/*
#cgo darwin LDFLAGS: -framework CoreAudio -framework Foundation
#include <stdint.h>

int32_t lunabox_process_mute_supported(void);
int32_t lunabox_create_process_mute_tap(uint32_t process_id, uint32_t *tap_id, int32_t *os_status);
int32_t lunabox_destroy_process_mute_tap(uint32_t tap_id, int32_t *os_status);
*/
import "C"

import (
	"fmt"
	"sync"
)

const (
	processMuteResultSuccess         = 0
	processMuteResultUnavailable     = 1
	processMuteResultProcessNotFound = 2
)

var darwinProcessMuteState = struct {
	sync.Mutex
	taps map[uint32]uint32
}{
	taps: make(map[uint32]uint32),
}

func IsProcessMuteSupported() bool {
	return C.lunabox_process_mute_supported() != 0
}

// SetProcessMuted creates or destroys a private Core Audio process tap. A
// muted tap suppresses the target process output while it exists.
func SetProcessMuted(processID uint32, muted bool) (bool, error) {
	if processID == 0 || !IsProcessMuteSupported() {
		return false, nil
	}

	darwinProcessMuteState.Lock()
	defer darwinProcessMuteState.Unlock()

	tapID, exists := darwinProcessMuteState.taps[processID]
	if muted {
		if exists {
			return true, nil
		}

		var createdTapID C.uint32_t
		var osStatus C.int32_t
		result := int(C.lunabox_create_process_mute_tap(
			C.uint32_t(processID),
			&createdTapID,
			&osStatus,
		))
		switch result {
		case processMuteResultSuccess:
			darwinProcessMuteState.taps[processID] = uint32(createdTapID)
			return true, nil
		case processMuteResultUnavailable, processMuteResultProcessNotFound:
			return false, nil
		default:
			return false, fmt.Errorf("create macOS process mute tap for PID %d: OSStatus %d", processID, int32(osStatus))
		}
	}

	if !exists {
		return false, nil
	}

	var osStatus C.int32_t
	result := int(C.lunabox_destroy_process_mute_tap(C.uint32_t(tapID), &osStatus))
	if result != processMuteResultSuccess {
		return false, fmt.Errorf("destroy macOS process mute tap for PID %d: OSStatus %d", processID, int32(osStatus))
	}
	delete(darwinProcessMuteState.taps, processID)
	return true, nil
}
