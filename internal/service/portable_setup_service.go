package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"lunabox/internal/utils/apputils"
)

// PortableSetupService exposes lunacli PATH registration helpers to the
// frontend. Custom URL protocols are registered by Wails during packaging.
type PortableSetupService struct {
	ctx context.Context
}

func NewPortableSetupService() *PortableSetupService {
	return &PortableSetupService{}
}

//wails:ignore
func (s *PortableSetupService) Init(ctx context.Context) {
	s.ctx = ctx
}

// PortableCLIStatus describes the lunacli.exe presence and PATH registration.
type PortableCLIStatus struct {
	Available   bool   `json:"available"`
	CLIPath     string `json:"cliPath"`
	CLIDir      string `json:"cliDir"`
	InstallPath string `json:"installPath"`
	InstallDir  string `json:"installDir"`
	Registered  bool   `json:"registered"`
}

// PortableSetupStatus is the aggregate snapshot consumed by the settings UI.
type PortableSetupStatus struct {
	BuildMode      string            `json:"buildMode"`
	IsPortable     bool              `json:"isPortable"`
	Platform       string            `json:"platform"`
	ExecutablePath string            `json:"executablePath"`
	CLI            PortableCLIStatus `json:"cli"`
}

// GetStatus returns the current lunacli PATH registration state.
func (s *PortableSetupService) GetStatus() (PortableSetupStatus, error) {
	status := PortableSetupStatus{
		BuildMode:  apputils.GetBuildMode(),
		IsPortable: apputils.IsPortableMode(),
		Platform:   runtime.GOOS,
	}

	exe, err := os.Executable()
	if err == nil {
		if abs, absErr := filepath.Abs(exe); absErr == nil {
			status.ExecutablePath = abs
		} else {
			status.ExecutablePath = exe
		}
	}

	cliExists, cliPath, cliErr := apputils.CLIExists()
	if cliErr != nil {
		return status, fmt.Errorf("probe lunacli: %w", cliErr)
	}
	status.CLI.Available = cliExists
	status.CLI.CLIPath = cliPath
	if cliPath != "" {
		status.CLI.CLIDir = filepath.Dir(cliPath)
	}
	installPath, err := apputils.GetCLIInstallPath()
	if err != nil {
		return status, fmt.Errorf("resolve lunacli install path: %w", err)
	}
	status.CLI.InstallPath = installPath
	if installPath != "" {
		status.CLI.InstallDir = filepath.Dir(installPath)
	}
	registered, err := apputils.IsCLIInstalled()
	if err != nil {
		return status, fmt.Errorf("query CLI install status: %w", err)
	}
	status.CLI.Registered = registered

	return status, nil
}

// RegisterCLIPath adds the lunacli.exe directory to the per-user PATH.
func (s *PortableSetupService) RegisterCLIPath() (PortableSetupStatus, error) {
	if _, err := apputils.InstallCLI(); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("install lunacli: %w", err)
	}
	return s.GetStatus()
}

// UnregisterCLIPath removes the lunacli registration for the current platform.
func (s *PortableSetupService) UnregisterCLIPath() (PortableSetupStatus, error) {
	if _, err := apputils.UninstallCLI(); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("uninstall lunacli: %w", err)
	}
	return s.GetStatus()
}
