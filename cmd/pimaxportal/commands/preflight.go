package commands

import (
	"fmt"
	"os/exec"
	"runtime"
)

func CheckPreflight() error {
	if _, err := exec.LookPath("adb"); err != nil {
		hint := adbInstallHint(runtime.GOOS)
		return fmt.Errorf("ADB (Android Debug Bridge) is not installed.\n\n%s\n\nSee ADB_INSTALLATION.md for detailed instructions.", hint)
	}
	return nil
}

func adbInstallHint(goos string) string {
	switch goos {
	case "darwin":
		return "  macOS: brew install android-platform-tools"
	case "linux":
		return "  Ubuntu/Debian: sudo apt install adb\n  Arch: sudo pacman -S android-tools"
	case "windows":
		return "  Windows: Download from https://developer.android.com/tools/releases/platform-tools\n  Extract and add to PATH."
	default:
		return "  Install Android platform-tools for your OS."
	}
}
