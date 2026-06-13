//go:build windows

package apputils

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

const (
	userEnvKeyPath  = `Environment`
	userPathValue   = "Path"
	hwndBroadcast   = 0xFFFF
	wmSettingChange = 0x001A
	smtoAbortIfHung = 0x0002
	smtoTimeoutMS   = 5000
)

var (
	userEnvUser32       = syscall.NewLazyDLL("user32.dll")
	procSendMsgTimeoutW = userEnvUser32.NewProc("SendMessageTimeoutW")
)

// IsDirInUserPath reports whether dir is registered in the current user PATH
// (HKCU\Environment\Path). Comparison is case-insensitive and ignores
// trailing separators and surrounding spaces.
func IsDirInUserPath(dir string) (bool, error) {
	entries, _, err := readUserPath()
	if err != nil {
		return false, err
	}
	return containsUserPathEntry(entries, dir), nil
}

// AddDirToUserPath appends dir to the current user PATH if not already
// present. The new value is written back using REG_EXPAND_SZ when the
// original key used %VAR% expansion, otherwise REG_SZ is preserved.
// Returns (changed, error); changed is false when the directory was
// already present.
func AddDirToUserPath(dir string) (bool, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false, fmt.Errorf("dir must not be empty")
	}

	entries, expand, err := readUserPath()
	if err != nil {
		return false, err
	}
	if containsUserPathEntry(entries, dir) {
		return false, nil
	}

	entries = append(entries, dir)
	if err := writeUserPath(entries, expand); err != nil {
		return false, err
	}
	broadcastEnvChange()
	return true, nil
}

// RemoveDirFromUserPath removes every case-insensitive occurrence of dir
// from the user PATH. Returns (changed, error); changed is false when the
// directory was not present.
func RemoveDirFromUserPath(dir string) (bool, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false, fmt.Errorf("dir must not be empty")
	}

	entries, expand, err := readUserPath()
	if err != nil {
		return false, err
	}

	filtered := make([]string, 0, len(entries))
	removed := false
	for _, entry := range entries {
		if userPathEntryEqual(entry, dir) {
			removed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !removed {
		return false, nil
	}

	if err := writeUserPath(filtered, expand); err != nil {
		return false, err
	}
	broadcastEnvChange()
	return true, nil
}

func readUserPath() ([]string, bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, userEnvKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("open environment key: %w", err)
	}
	defer key.Close()

	raw, valType, err := key.GetStringValue(userPathValue)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read PATH value: %w", err)
	}
	return splitUserPath(raw), valType == registry.EXPAND_SZ, nil
}

func writeUserPath(entries []string, expand bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, userEnvKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open environment key for write: %w", err)
	}
	defer key.Close()

	value := strings.Join(entries, ";")
	if expand || containsExpansion(value) {
		return key.SetExpandStringValue(userPathValue, value)
	}
	return key.SetStringValue(userPathValue, value)
}

func splitUserPath(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func containsUserPathEntry(entries []string, dir string) bool {
	for _, entry := range entries {
		if userPathEntryEqual(entry, dir) {
			return true
		}
	}
	return false
}

func userPathEntryEqual(a, b string) bool {
	return strings.EqualFold(normalizeUserPathEntry(a), normalizeUserPathEntry(b))
}

func normalizeUserPathEntry(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, `\/`)
	return s
}

func containsExpansion(s string) bool {
	// Simple heuristic: a literal "%VAR%" substring forces REG_EXPAND_SZ.
	first := strings.Index(s, "%")
	if first < 0 {
		return false
	}
	return strings.Index(s[first+1:], "%") >= 0
}

// broadcastEnvChange notifies running processes that the environment block
// has changed. Existing shells still need to reopen to see the new PATH.
func broadcastEnvChange() {
	envPtr, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	_, _, _ = procSendMsgTimeoutW.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(envPtr)),
		smtoAbortIfHung,
		smtoTimeoutMS,
		uintptr(unsafe.Pointer(&result)),
	)
}
