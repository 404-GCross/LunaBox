//go:build darwin && cgo

#import <CoreAudio/CoreAudio.h>
#import <Foundation/Foundation.h>
#include <limits.h>
#include <stdint.h>

enum {
    LunaBoxProcessMuteSuccess = 0,
    LunaBoxProcessMuteUnavailable = 1,
    LunaBoxProcessMuteProcessNotFound = 2,
    LunaBoxProcessMuteFailure = 3,
};

int32_t lunabox_process_mute_supported(void) {
    if (@available(macOS 14.2, *)) {
        return 1;
    }
    return 0;
}

int32_t lunabox_create_process_mute_tap(uint32_t process_id, uint32_t *tap_id, int32_t *os_status) {
    if (tap_id == NULL || os_status == NULL || process_id == 0 || process_id > INT32_MAX) {
        return LunaBoxProcessMuteFailure;
    }

    *tap_id = kAudioObjectUnknown;
    *os_status = noErr;

    if (@available(macOS 14.2, *)) {
        @autoreleasepool {
            pid_t target_pid = (pid_t)process_id;
            AudioObjectID process_object_id = kAudioObjectUnknown;
            UInt32 property_size = sizeof(process_object_id);
            AudioObjectPropertyAddress address = {
                kAudioHardwarePropertyTranslatePIDToProcessObject,
                kAudioObjectPropertyScopeGlobal,
                kAudioObjectPropertyElementMain,
            };

            OSStatus status = AudioObjectGetPropertyData(
                kAudioObjectSystemObject,
                &address,
                sizeof(target_pid),
                &target_pid,
                &property_size,
                &process_object_id
            );
            if (status != noErr || process_object_id == kAudioObjectUnknown) {
                *os_status = status;
                return LunaBoxProcessMuteProcessNotFound;
            }

            CATapDescription *description = [[CATapDescription alloc] init];
            [description setName:[NSString stringWithFormat:@"LunaBox background mute %u", process_id]];
            [description setProcesses:@[@(process_object_id)]];
            [description setPrivate:YES];
            [description setMuteBehavior:CATapMuted];

            AudioObjectID created_tap_id = kAudioObjectUnknown;
            status = AudioHardwareCreateProcessTap(description, &created_tap_id);
            [description release];
            if (status != noErr || created_tap_id == kAudioObjectUnknown) {
                *os_status = status;
                return LunaBoxProcessMuteFailure;
            }

            *tap_id = created_tap_id;
            return LunaBoxProcessMuteSuccess;
        }
    }

    return LunaBoxProcessMuteUnavailable;
}

int32_t lunabox_destroy_process_mute_tap(uint32_t tap_id, int32_t *os_status) {
    if (os_status == NULL || tap_id == kAudioObjectUnknown) {
        return LunaBoxProcessMuteFailure;
    }

    *os_status = noErr;
    if (@available(macOS 14.2, *)) {
        OSStatus status = AudioHardwareDestroyProcessTap((AudioObjectID)tap_id);
        if (status == noErr || status == kAudioHardwareBadObjectError) {
            return LunaBoxProcessMuteSuccess;
        }
        *os_status = status;
        return LunaBoxProcessMuteFailure;
    }

    return LunaBoxProcessMuteUnavailable;
}
