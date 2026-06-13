package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lunabox/internal/protocol"
	"lunabox/internal/utils/apputils"
)

// PortableSetupService exposes the lunabox:// protocol and lunacli PATH
// registration helpers to the frontend. It only meaningfully operates on
// Windows; on other platforms (or in installer builds) the GetStatus call
// still works and returns a disabled state.
type PortableSetupService struct {
	ctx context.Context
}

func NewPortableSetupService() *PortableSetupService {
	return &PortableSetupService{}
}

func (s *PortableSetupService) Init(ctx context.Context) {
	s.ctx = ctx
}

// PortableProtocolStatus describes the current lunabox:// scheme binding.
type PortableProtocolStatus struct {
	Registered     bool   `json:"registered"`
	RegisteredPath string `json:"registeredPath"`
	CurrentPath    string `json:"currentPath"`
	UpToDate       bool   `json:"upToDate"`
}

// PortableCLIStatus describes the lunacli.exe presence and PATH registration.
type PortableCLIStatus struct {
	Available  bool   `json:"available"`
	CLIPath    string `json:"cliPath"`
	CLIDir     string `json:"cliDir"`
	Registered bool   `json:"registered"`
}

// PortableSetupStatus is the aggregate snapshot consumed by the settings UI.
type PortableSetupStatus struct {
	BuildMode      string                 `json:"buildMode"`
	IsPortable     bool                   `json:"isPortable"`
	ExecutablePath string                 `json:"executablePath"`
	Protocol       PortableProtocolStatus `json:"protocol"`
	CLI            PortableCLIStatus      `json:"cli"`
}

// GetStatus returns the current registration state for the protocol handler
// and the lunacli PATH entry.
func (s *PortableSetupService) GetStatus() (PortableSetupStatus, error) {
	status := PortableSetupStatus{
		BuildMode:  apputils.GetBuildMode(),
		IsPortable: apputils.IsPortableMode(),
	}

	exe, err := os.Executable()
	if err == nil {
		if abs, absErr := filepath.Abs(exe); absErr == nil {
			status.ExecutablePath = abs
		} else {
			status.ExecutablePath = exe
		}
	}

	registeredExe, err := protocol.GetRegisteredURLSchemeExe()
	if err != nil {
		return status, fmt.Errorf("query protocol status: %w", err)
	}
	status.Protocol.RegisteredPath = registeredExe
	status.Protocol.Registered = registeredExe != ""
	status.Protocol.CurrentPath = status.ExecutablePath
	status.Protocol.UpToDate = status.Protocol.Registered &&
		strings.EqualFold(filepath.Clean(registeredExe), filepath.Clean(status.ExecutablePath))

	cliExists, cliPath, cliErr := apputils.CLIExists()
	if cliErr != nil {
		return status, fmt.Errorf("probe lunacli: %w", cliErr)
	}
	status.CLI.Available = cliExists
	status.CLI.CLIPath = cliPath
	if cliPath != "" {
		status.CLI.CLIDir = filepath.Dir(cliPath)
	}

	if status.CLI.CLIDir != "" {
		inPath, err := apputils.IsDirInUserPath(status.CLI.CLIDir)
		if err != nil {
			return status, fmt.Errorf("query PATH status: %w", err)
		}
		status.CLI.Registered = inPath
	}

	return status, nil
}

// RegisterProtocol writes (or refreshes) the lunabox:// handler so it points
// at the currently running executable.
func (s *PortableSetupService) RegisterProtocol() (PortableSetupStatus, error) {
	if err := protocol.RegisterURLScheme(""); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("register protocol: %w", err)
	}
	return s.GetStatus()
}

// UnregisterProtocol removes the lunabox:// handler.
func (s *PortableSetupService) UnregisterProtocol() (PortableSetupStatus, error) {
	if err := protocol.UnregisterURLScheme(); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("unregister protocol: %w", err)
	}
	return s.GetStatus()
}

// RegisterCLIPath adds the lunacli.exe directory to the per-user PATH.
func (s *PortableSetupService) RegisterCLIPath() (PortableSetupStatus, error) {
	dir, err := apputils.GetCLIDir()
	if err != nil {
		return PortableSetupStatus{}, err
	}
	if _, err := apputils.AddDirToUserPath(dir); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("add lunacli dir to PATH: %w", err)
	}
	return s.GetStatus()
}

// UnregisterCLIPath removes the lunacli.exe directory from the per-user PATH.
func (s *PortableSetupService) UnregisterCLIPath() (PortableSetupStatus, error) {
	dir, err := apputils.GetCLIDir()
	if err != nil {
		return PortableSetupStatus{}, err
	}
	if _, err := apputils.RemoveDirFromUserPath(dir); err != nil {
		return PortableSetupStatus{}, fmt.Errorf("remove lunacli dir from PATH: %w", err)
	}
	return s.GetStatus()
}
