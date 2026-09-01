package protocolcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"lunabox/internal/protocol"
	"lunabox/internal/utils/apputils"

	"github.com/spf13/cobra"
)

// NewCommand creates the local protocol management command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protocol",
		Short: "Manage the local lunabox:// URL protocol handler",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newRegisterCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newUnregisterCmd())
	return cmd
}

func newRegisterCmd() *cobra.Command {
	var exePath string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register lunabox:// for a local build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !supportsLocalProtocolRegistration() {
				return fmt.Errorf("installed builds manage lunabox:// through the Wails installer")
			}
			if exePath == "" {
				var err error
				exePath, err = localProtocolExecutablePath()
				if err != nil {
					return err
				}
			}
			if err := protocol.RegisterPortableURLScheme(exePath); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "lunabox:// protocol registered for this local build")
			return nil
		},
	}

	cmd.Flags().StringVar(&exePath, "exe", "", "Override the executable path")
	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the local lunabox:// handler status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !supportsLocalProtocolRegistration() {
				return fmt.Errorf("installed builds manage lunabox:// through the Wails installer")
			}
			exePath, err := protocol.GetRegisteredURLSchemeExe()
			if err != nil {
				return err
			}
			if strings.TrimSpace(exePath) == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "lunabox:// protocol is not registered")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "lunabox:// protocol registered: %s\n", exePath)
			if localPath, err := localProtocolExecutablePath(); err == nil {
				if samePath(exePath, localPath) || protocol.IsAppImageProtocolLauncherFor(exePath, localPath) {
					fmt.Fprintln(cmd.OutOrStdout(), "registered executable matches this local build")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "this local build executable: %s\n", localPath)
				}
			}
			return nil
		},
	}
}

func newUnregisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unregister",
		Short: "Unregister the local lunabox:// handler",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !supportsLocalProtocolRegistration() {
				return fmt.Errorf("installed builds manage lunabox:// through the Wails installer")
			}
			if err := protocol.UnregisterPortableURLScheme(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "lunabox:// protocol unregistered for this local build")
			return nil
		},
	}
}

func supportsLocalProtocolRegistration() bool {
	return apputils.IsPortableMode() || apputils.IsAppImageMode()
}

func localProtocolExecutablePath() (string, error) {
	if apputils.IsAppImageMode() {
		return apputils.GetAppImagePath()
	}
	return siblingPortableGUIPath()
}

func siblingPortableGUIPath() (string, error) {
	guiName, err := portableGUIExecutableName()
	if err != nil {
		return "", err
	}
	cliPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get lunacli executable path: %w", err)
	}
	guiPath := filepath.Join(filepath.Dir(cliPath), guiName)
	info, err := os.Stat(guiPath)
	if err != nil {
		return "", fmt.Errorf("find portable %s next to lunacli: %w", guiName, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("portable %s path is a directory: %s", guiName, guiPath)
	}
	return guiPath, nil
}

func portableGUIExecutableName() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return "LunaBox.exe", nil
	case "linux":
		return "LunaBox", nil
	default:
		return "", fmt.Errorf("portable protocol registration is not supported on %s", runtime.GOOS)
	}
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbs, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
