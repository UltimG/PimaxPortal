package commands

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
)

func CheckPreflight() error {
	if _, err := exec.LookPath("adb"); err != nil {
		hint := adbInstallHint(runtime.GOOS)
		return fmt.Errorf("ADB (Android Debug Bridge) is not installed.\n\n%s\n\nSee ADB_INSTALLATION.md for detailed instructions.", hint)
	}
	if _, err := exec.LookPath("7z"); err != nil {
		hint := sevenzipInstallHint(runtime.GOOS)
		return fmt.Errorf("7z is not installed.\n\n%s", hint)
	}
	return nil
}

// CheckRootPreflight verifies that both ADB and Fastboot are available.
func CheckRootPreflight() error {
	if !adb.Available() {
		hint := platformToolsHint(runtime.GOOS)
		return fmt.Errorf("ADB (Android Debug Bridge) is not installed.\n\n%s", hint)
	}
	if !adb.FastbootAvailable() {
		hint := platformToolsHint(runtime.GOOS)
		return fmt.Errorf("Fastboot is not installed.\n\n%s", hint)
	}
	return nil
}

func platformToolsHint(goos string) string {
	switch goos {
	case "darwin":
		return "  macOS:   brew install android-platform-tools"
	case "linux":
		return "  Linux:   sudo apt install adb fastboot (Debian/Ubuntu)\n           sudo pacman -S android-tools (Arch)"
	case "windows":
		return "  Windows: Download from https://developer.android.com/tools/releases/platform-tools\n           Extract and add to PATH."
	default:
		return "  Install Android platform-tools for your OS."
	}
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

func sevenzipInstallHint(goos string) string {
	switch goos {
	case "darwin":
		return "  macOS: brew install p7zip"
	case "linux":
		return "  Ubuntu/Debian: sudo apt install p7zip-full\n  Arch: sudo pacman -S p7zip"
	case "windows":
		return "  Windows: Download from https://7-zip.org and add to PATH."
	default:
		return "  Install p7zip for your OS."
	}
}
