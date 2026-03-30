package adb

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// FastbootAvailable returns true if the fastboot binary is found in PATH.
func FastbootAvailable() bool {
	_, err := exec.LookPath("fastboot")
	return err == nil
}

// FastbootDevices returns the serial numbers of all connected fastboot devices.
func FastbootDevices() ([]string, error) {
	out, err := exec.Command("fastboot", "devices").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("fastboot devices: %w", err)
	}
	return parseFastbootDevices(string(out)), nil
}

// parseFastbootDevices extracts serial numbers from `fastboot devices` output.
func parseFastbootDevices(output string) []string {
	var devs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		serial := parts[0]
		if serial != "" {
			devs = append(devs, serial)
		}
	}
	return devs
}

// FastbootFlash runs `fastboot flash <partition> <imagePath>`.
func FastbootFlash(partition, imagePath string) error {
	out, err := exec.Command("fastboot", "flash", partition, imagePath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("fastboot flash: %s: %w", string(out), err)
	}
	return nil
}

// FastbootReboot runs `fastboot reboot`.
func FastbootReboot() error {
	out, err := exec.Command("fastboot", "reboot").CombinedOutput()
	if err != nil {
		return fmt.Errorf("fastboot reboot: %s: %w", string(out), err)
	}
	return nil
}

// WaitForFastboot polls FastbootDevices every 2 seconds until at least one
// device is found or the timeout expires.
func WaitForFastboot(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		devs, err := FastbootDevices()
		if err == nil && len(devs) > 0 {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

// WaitForADB polls Devices every 2 seconds until at least one device is found
// or the timeout expires.
func WaitForADB(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		devs, err := Devices()
		if err == nil && len(devs) > 0 {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}
