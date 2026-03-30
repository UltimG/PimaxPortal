package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
)

// FlashCommand handles rebooting to FastbootD and flashing a patched boot
// image.
type FlashCommand struct{}

// Flash reboots the device into FastbootD, flashes the given boot image, then
// reboots back to Android.
func (f FlashCommand) Flash(ctx context.Context, imagePath string, send func(ProgressMsg)) error {
	send(ProgressMsg{Text: "Rebooting to FastbootD...", Percent: 0})

	if err := adb.RebootFastboot(); err != nil {
		return fmt.Errorf("rebooting to fastboot: %w", err)
	}

	send(ProgressMsg{Text: "Waiting for FastbootD...", Percent: 0.1})

	if !adb.WaitForFastboot(30 * time.Second) {
		return fmt.Errorf("device did not appear in fastboot mode within 30 seconds")
	}

	send(ProgressMsg{Text: "Flashing patched boot image...", Percent: 0.3})

	if err := adb.FastbootFlash("boot", imagePath); err != nil {
		return fmt.Errorf("flashing boot image: %w", err)
	}

	send(ProgressMsg{Text: "Rebooting device...", Percent: 0.7})

	if err := adb.FastbootReboot(); err != nil {
		return fmt.Errorf("rebooting from fastboot: %w", err)
	}

	send(ProgressMsg{Text: "Waiting for device to boot...", Percent: 0.8})

	if !adb.WaitForADB(60 * time.Second) {
		// Device may be slow booting — warn but don't fail.
		send(ProgressMsg{Text: "WARNING: device not yet visible after 60s — it may still be booting", Percent: -1})
	}

	send(ProgressMsg{Text: "Flash complete", Percent: 1.0})
	return nil
}
