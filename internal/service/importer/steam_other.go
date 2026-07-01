//go:build !windows

package importer

import "fmt"

func findSteamInstallPath() (string, error) {
	return "", fmt.Errorf("Steam 本地库扫描当前仅支持 Windows")
}
