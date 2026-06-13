//go:build windows

package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RegisterURLScheme registers lunabox:// in HKCU (no admin required).
// exePath should be the absolute path to LunaBox.exe.
func RegisterURLScheme(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}
		exePath, _ = filepath.Abs(exePath)
	}

	root, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\`+Scheme,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("create registry key: %w", err)
	}
	defer root.Close()

	if err := root.SetStringValue("", "URL:LunaBox Protocol"); err != nil {
		return err
	}
	if err := root.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}

	cmdKey, _, err := registry.CreateKey(root, `shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create command key: %w", err)
	}
	defer cmdKey.Close()

	// Windows replaces %1 with the full lunabox:// URI at invocation time.
	command := fmt.Sprintf(`"%s" "%%1"`, exePath)
	return cmdKey.SetStringValue("", command)
}

// GetRegisteredURLSchemeExe returns the executable path currently registered
// for the lunabox:// scheme in HKCU. Returns ("", nil) when not registered.
func GetRegisteredURLSchemeExe() (string, error) {
	cmdKey, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Classes\`+Scheme+`\shell\open\command`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", nil
		}
		return "", fmt.Errorf("open command key: %w", err)
	}
	defer cmdKey.Close()

	command, _, err := cmdKey.GetStringValue("")
	if err != nil {
		if err == registry.ErrNotExist {
			return "", nil
		}
		return "", fmt.Errorf("read command value: %w", err)
	}

	return extractExeFromCommand(command), nil
}

// extractExeFromCommand pulls the executable path out of a registry command
// string formatted as `"path\to\exe" "%1"` (with optional surrounding spaces).
func extractExeFromCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if strings.HasPrefix(command, `"`) {
		if end := strings.Index(command[1:], `"`); end >= 0 {
			return command[1 : 1+end]
		}
		return strings.TrimPrefix(command, `"`)
	}
	if idx := strings.Index(command, " "); idx >= 0 {
		return command[:idx]
	}
	return command
}

// UnregisterURLScheme removes the lunabox:// protocol handler from HKCU.
func UnregisterURLScheme() error {
	basePath := `Software\Classes\` + Scheme
	if err := deleteRegistryTree(registry.CURRENT_USER, basePath); err != nil {
		return fmt.Errorf("delete key %s: %w", basePath, err)
	}
	return nil
}

func deleteRegistryTree(root registry.Key, path string) error {
	key, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}

	subKeys, readErr := key.ReadSubKeyNames(-1)
	_ = key.Close()
	if readErr != nil {
		return fmt.Errorf("read sub keys: %w", readErr)
	}

	for _, subKey := range subKeys {
		if err := deleteRegistryTree(root, path+`\`+subKey); err != nil {
			return err
		}
	}

	err = registry.DeleteKey(root, path)
	if err != nil && err != registry.ErrNotExist {
		return err
	}

	return nil
}
