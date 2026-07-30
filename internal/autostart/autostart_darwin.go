//go:build darwin

package autostart

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Foundation -framework ServiceManagement

#include <stdlib.h>
#include <string.h>

#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>

static int lunaboxSyncLoginItem(int enabled, char **errOut) {
	@autoreleasepool {
		if (@available(macOS 13.0, *)) {
			NSError *error = nil;
			SMAppService *service = [SMAppService mainAppService];
			if (enabled && service.status == SMAppServiceStatusEnabled) {
				return 0;
			}
			if (!enabled && service.status == SMAppServiceStatusNotRegistered) {
				return 0;
			}
			BOOL ok = enabled ? [service registerAndReturnError:&error] : [service unregisterAndReturnError:&error];
			if (ok) {
				return 0;
			}
			if (error != nil) {
				const char *msg = [[error localizedDescription] UTF8String];
				if (msg != NULL) {
					*errOut = strdup(msg);
				}
			}
			return 1;
		}
		*errOut = strdup("Launch at login requires macOS 13 or later");
		return 2;
	}
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func ExtractLaunchFlag(args []string) ([]string, bool) {
	return args, false
}

func Sync(enabled bool) error {
	var errMsg *C.char
	code := C.lunaboxSyncLoginItem(C.int(boolToInt(enabled)), &errMsg)
	if errMsg != nil {
		defer C.free(unsafe.Pointer(errMsg))
	}
	if code == 0 {
		return nil
	}
	msg := C.GoString(errMsg)
	if msg == "" {
		msg = "unknown ServiceManagement error"
	}
	return fmt.Errorf("sync macOS login item: %s", msg)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
