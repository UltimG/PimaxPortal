package adb

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// GPUInfo holds parsed GPU details from SurfaceFlinger GLES line.
type GPUInfo struct {
	GPU           string
	DriverVersion string
}

// DeviceInfo aggregates all collected information about the connected device.
type DeviceInfo struct {
	Serial          string
	Model           string
	Variant         string
	PanelType       string
	GPU             string
	DriverVersion   string
	Connected       bool
	MultipleDevices bool
}

// Available returns true if the adb binary is found in PATH.
func Available() bool {
	_, err := exec.LookPath("adb")
	return err == nil
}

// Devices returns the serial numbers of all connected and authorised devices.
func Devices() ([]string, error) {
	out, err := exec.Command("adb", "devices").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}
	return parseDevices(string(out)), nil
}

// parseDevices extracts serial numbers from `adb devices` output.
// Only devices with status "device" are included (unauthorized, offline, etc.
// are skipped).
func parseDevices(output string) []string {
	var devs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of") || strings.HasPrefix(line, "*") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		serial := parts[0]
		status := parts[1]
		if status == "device" {
			devs = append(devs, serial)
		}
	}
	return devs
}

// GetProp returns the value of an Android system property.
func GetProp(prop string) (string, error) {
	out, err := exec.Command("adb", "shell", "getprop", prop).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("getprop %s: %w", prop, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Shell runs a command via `adb shell` and returns the trimmed output.
func Shell(cmd string) (string, error) {
	out, err := exec.Command("adb", "shell", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("adb shell %s: %w", cmd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ShellSu runs a command as root via `adb shell su -c`.
func ShellSu(cmd string) (string, error) {
	out, err := exec.Command("adb", "shell", "su", "-c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("adb shell su -c %s: %w", cmd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Push copies a local file to a remote path on the device.
func Push(local, remote string) error {
	out, err := exec.Command("adb", "push", local, remote).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb push %s %s: %s: %w", local, remote, string(out), err)
	}
	return nil
}

// Reboot reboots the connected device.
func Reboot() error {
	out, err := exec.Command("adb", "reboot").CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb reboot: %s: %w", string(out), err)
	}
	return nil
}

// CheckRoot attempts `su -c id` on the device and returns true when the
// output contains uid=0.
func CheckRoot() (bool, error) {
	out, err := ShellSu("id")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "uid=0"), nil
}

// GetGPUInfo parses the GLES line from `dumpsys SurfaceFlinger`.
func GetGPUInfo() (GPUInfo, error) {
	out, err := Shell("dumpsys SurfaceFlinger")
	if err != nil {
		return GPUInfo{}, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "GLES:") {
			return parseGLES(line), nil
		}
	}
	return GPUInfo{}, fmt.Errorf("GLES line not found in SurfaceFlinger output")
}

// gleVersionRe matches V@<version> in the GLES string.
var gleVersionRe = regexp.MustCompile(`V@[\d.]+`)

// parseGLES extracts GPU name and driver version from a GLES line such as:
//
//	GLES: Qualcomm, Adreno (TM) 650, OpenGL ES 3.2 V@0764.0 ...
func parseGLES(line string) GPUInfo {
	var info GPUInfo

	// Strip leading "GLES:" and split by comma.
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "GLES:")
	parts := strings.SplitN(line, ",", 3)

	if len(parts) >= 2 {
		info.GPU = strings.TrimSpace(parts[1])
	}

	if len(parts) >= 3 {
		if m := gleVersionRe.FindString(parts[2]); m != "" {
			info.DriverVersion = m
		}
	}

	return info
}

// Pull copies a remote file from the device to a local path.
func Pull(remote, local string) error {
	out, err := exec.Command("adb", "pull", remote, local).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb pull %s %s: %s: %w", remote, local, string(out), err)
	}
	return nil
}

// Install installs an APK on the connected device.
func Install(apkPath string) error {
	out, err := exec.Command("adb", "install", apkPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb install %s: %s: %w", apkPath, string(out), err)
	}
	return nil
}

// RebootFastboot reboots the device into FastbootD mode.
func RebootFastboot() error {
	out, err := exec.Command("adb", "reboot", "fastboot").CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb reboot fastboot: %s: %w", string(out), err)
	}
	return nil
}

// HasPackage returns true if the given package is installed on the device.
func HasPackage(pkg string) bool {
	out, err := exec.Command("adb", "shell", "pm", "list", "packages", pkg).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "package:"+pkg)
}

// FindFile runs `ls <pattern>` on the device and returns the first matching
// path. Returns an error if no match is found.
func FindFile(pattern string) (string, error) {
	out, err := exec.Command("adb", "shell", "ls", pattern, "2>/dev/null").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("adb shell ls %s: %w", pattern, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no file found matching %s", pattern)
	}
	return lines[0], nil
}

// GetDeviceInfo collects comprehensive information about the connected device.
// If more than one authorised device is attached, MultipleDevices is set to
// true and no further queries are made.
func GetDeviceInfo() (DeviceInfo, error) {
	devs, err := Devices()
	if err != nil {
		return DeviceInfo{}, err
	}

	if len(devs) == 0 {
		return DeviceInfo{Connected: false}, nil
	}

	if len(devs) > 1 {
		return DeviceInfo{
			Connected:       true,
			MultipleDevices: true,
		}, nil
	}

	info := DeviceInfo{
		Serial:    devs[0],
		Connected: true,
	}

	if model, err := GetProp("ro.product.model"); err == nil {
		info.Model = model
	}

	if variant, err := GetProp("ro.pmx.proj.sub.name"); err == nil {
		info.Variant = variant
	}

	if panel, err := GetProp("ro.boot.lcd_panel_type"); err == nil {
		info.PanelType = panel
	}

	if gpu, err := GetGPUInfo(); err == nil {
		info.GPU = gpu.GPU
		info.DriverVersion = gpu.DriverVersion
	}

	return info, nil
}
