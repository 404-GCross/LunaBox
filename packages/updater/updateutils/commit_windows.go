//go:build windows

package updateutils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const replaceFileWriteThrough = 0x00000001

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func waitForProcessExit(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if err == windows.ERROR_INVALID_PARAMETER {
			return nil
		}
		return err
	}
	defer windows.CloseHandle(handle)

	milliseconds := uint32(timeout / time.Millisecond)
	if timeout <= 0 || milliseconds == 0 {
		milliseconds = windows.INFINITE
	}
	result, err := windows.WaitForSingleObject(handle, milliseconds)
	if err != nil {
		return err
	}
	if result == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("timed out after %s", timeout)
	}
	return nil
}

func replaceFile(targetPath string, replacementPath string, backupPath string) (bool, error) {
	_, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return false, os.Rename(replacementPath, targetPath)
	}
	if err != nil {
		return false, err
	}
	target, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return true, err
	}
	replacement, err := windows.UTF16PtrFromString(replacementPath)
	if err != nil {
		return true, err
	}
	backup, err := windows.UTF16PtrFromString(backupPath)
	if err != nil {
		return true, err
	}
	if err := callReplaceFile(target, replacement, backup); err != nil {
		return true, err
	}
	return true, nil
}

func restoreBackup(targetPath string, backupPath string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return err
	}
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return os.Rename(backupPath, targetPath)
	}
	target, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return err
	}
	backup, err := windows.UTF16PtrFromString(backupPath)
	if err != nil {
		return err
	}
	return callReplaceFile(target, backup, nil)
}

func callReplaceFile(target *uint16, replacement *uint16, backup *uint16) error {
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(unsafe.Pointer(replacement)),
		uintptr(unsafe.Pointer(backup)),
		replaceFileWriteThrough,
		0,
		0,
	)
	if result == 0 {
		return callErr
	}
	return nil
}

func updateInstallMetadata(buildMode string, version string) error {
	if buildMode != "installer" {
		return nil
	}
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\LunaBoxLunaBox`,
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("DisplayVersion", version)
}

func configureRestartCommand(command *exec.Cmd) error {
	// HideWindow must remain false because STARTUPINFO would otherwise override
	// Wails' first ShowWindow call with SW_HIDE.
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
	}
	return nil
}

// CanRetryElevated reports whether a replacement failure is likely caused by
// the installation directory requiring administrator permission.
func CanRetryElevated(err error) bool {
	if err == nil {
		return false
	}
	permissionDenied := errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) ||
		errors.Is(err, os.ErrPermission)
	var preExitErr *preExitCommitError
	if errors.As(err, &preExitErr) {
		return permissionDenied
	}
	return permissionDenied
}

// StartElevatedCommit starts a second updater instance through the Windows
// runas verb. The child receives the same task and performs the actual retry.
func StartElevatedCommit(taskPath string, workDir string) error {
	updaterPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve updater executable: %w", err)
	}
	file, err := windows.UTF16PtrFromString(updaterPath)
	if err != nil {
		return fmt.Errorf("encode updater executable: %w", err)
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	parameters, err := windows.UTF16PtrFromString(windows.ComposeCommandLine([]string{"commit", "--task", taskPath, "--elevated"}))
	if err != nil {
		return fmt.Errorf("encode updater parameters: %w", err)
	}
	directory, err := windows.UTF16PtrFromString(strings.TrimSpace(workDir))
	if err != nil {
		return fmt.Errorf("encode updater working directory: %w", err)
	}
	result, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(parameters)),
		uintptr(unsafe.Pointer(directory)),
		1,
	)
	if result <= 32 {
		if callErr != nil {
			return fmt.Errorf("start elevated updater: %w", callErr)
		}
		return fmt.Errorf("start elevated updater failed with code %d", result)
	}
	return nil
}

var procShellExecuteW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")
