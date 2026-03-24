package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
)

func init() {
	Register(&InstallCommand{})
}

type InstallCommand struct{}

func (i *InstallCommand) Name() string        { return "install" }
func (i *InstallCommand) Description() string { return "Install GPU Driver Module" }

func (i *InstallCommand) Run(ctx context.Context, send func(ProgressMsg)) error {
	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}

	zipPath := filepath.Join(cacheDir, "build", moduleZipName)
	if !fileExists(zipPath) {
		return fmt.Errorf("module not built yet — run build first")
	}

	info, err := adb.GetDeviceInfo()
	if err != nil {
		return fmt.Errorf("device info: %w", err)
	}

	if !info.Connected {
		send(ProgressMsg{Text: fmt.Sprintf("No device connected. Module is at:\n  %s\nConnect device and re-run to install.", zipPath)})
		return nil
	}

	if info.Variant != "fujilite" {
		send(ProgressMsg{Text: fmt.Sprintf("WARNING: Device is '%s', not fujilite (Pimax Portal Retro).", info.Variant)})
	}

	send(ProgressMsg{Text: "Pushing module to device", Percent: -1})
	if err := adb.Push(zipPath, "/sdcard/pimax-gpu-drivers.zip"); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	// Root access check with interactive polling
	send(ProgressMsg{Text: "Checking root access", Percent: -1})
	hasRoot, err := adb.CheckRoot()
	if err != nil || !hasRoot {
		// Signal TUI to show root access overlay
		send(ProgressMsg{Text: "ROOT_CHECK_WAITING"})
		granted := false
		deadline := time.After(60 * time.Second)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for !granted {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-deadline:
				send(ProgressMsg{Text: "ROOT_CHECK_TIMEOUT"})
				return fmt.Errorf("root access not granted within 60 seconds")
			case <-ticker.C:
				if ok, err := adb.CheckRoot(); err == nil && ok {
					granted = true
					send(ProgressMsg{Text: "ROOT_CHECK_GRANTED"})
				}
			}
		}
	}
	send(ProgressMsg{Text: "Root access confirmed.", Percent: -1})

	send(ProgressMsg{Text: "Installing module via Magisk", Percent: -1})
	out, err := adb.ShellSu(`magisk --install-module /sdcard/pimax-gpu-drivers.zip`)
	if err != nil {
		return fmt.Errorf("magisk install failed: %s\nCheck that Magisk is installed and root is granted.", out)
	}
	send(ProgressMsg{Text: out, Percent: -1})

	_, _ = adb.Shell("rm /sdcard/pimax-gpu-drivers.zip")

	// Signal TUI to show reboot prompt
	send(ProgressMsg{Text: "INSTALL_COMPLETE"})
	return nil
}
