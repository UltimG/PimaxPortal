package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/UltimG/PimaxPortal/cmd/pimaxportal/commands/adb"
)

const (
	magiskRepo    = "topjohnwu/Magisk"
	bootImgURL    = "https://github.com/UltimG/PimaxPortal/raw/main/assets/firmware/boot.img"
	bootImgName   = "boot.img"
	magiskApkName = "Magisk.apk"

	// Expected firmware build fingerprint for the boot.img we ship.
	// Flashing a mismatched boot.img breaks WiFi and controllers.
	expectedFingerprint = "robot09221054"
)

// RootCommand handles Magisk download/install and boot.img push/pull for the
// rooting workflow.
type RootCommand struct{}

// PrepareMagisk ensures Magisk is installed on the device. If the package is
// not present, it downloads the latest release APK from GitHub and installs it
// via ADB.
func (r RootCommand) PrepareMagisk(ctx context.Context, cacheDir string, send func(ProgressMsg)) error {
	send(ProgressMsg{Text: "Checking for Magisk...", Percent: -1})

	if adb.HasPackage("com.topjohnwu.magisk") {
		send(ProgressMsg{Text: "Magisk already installed", Percent: 1.0})
		return nil
	}

	send(ProgressMsg{Text: "Magisk not found — downloading latest release", Percent: 0})

	apkPath := filepath.Join(cacheDir, magiskApkName)

	// Fetch latest release metadata from GitHub API.
	apkURL, err := latestMagiskAPKURL(ctx)
	if err != nil {
		return fmt.Errorf("finding Magisk release: %w", err)
	}

	if err := downloadFile(ctx, apkURL, apkPath, func(pct float64) {
		send(ProgressMsg{Text: "Downloading Magisk APK", Percent: pct * 0.8})
	}); err != nil {
		return fmt.Errorf("downloading Magisk APK: %w", err)
	}

	send(ProgressMsg{Text: "Installing Magisk APK", Percent: 0.8})

	if err := adb.Install(apkPath); err != nil {
		return fmt.Errorf("installing Magisk: %w", err)
	}

	send(ProgressMsg{Text: "Magisk installed", Percent: 1.0})
	return nil
}

// latestMagiskAPKURL queries the GitHub API for the latest Magisk release and
// returns the download URL for the .apk asset.
func latestMagiskAPKURL(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", magiskRepo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parsing release JSON: %w", err)
	}

	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".apk") {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("no .apk asset found in latest Magisk release")
}

// CheckFirmware verifies the device firmware matches the boot.img we ship.
// Returns an error if the firmware is incompatible.
func (r RootCommand) CheckFirmware() error {
	fp, err := adb.GetProp("ro.build.fingerprint")
	if err != nil {
		return fmt.Errorf("cannot read device fingerprint: %w", err)
	}
	if !strings.Contains(fp, expectedFingerprint) {
		return fmt.Errorf(
			"firmware mismatch — device has %s, expected build %s.\n"+
				"Flashing our boot.img on different firmware will break WiFi and controllers.\n"+
				"Flash stock firmware first (see EDL Recovery Guide), then retry",
			fp, expectedFingerprint)
	}
	return nil
}

// PushBootImage downloads the stock boot.img (if not cached) and pushes it to
// /sdcard/boot.img on the device for Magisk patching.
func (r RootCommand) PushBootImage(ctx context.Context, cacheDir string, send func(ProgressMsg)) error {
	send(ProgressMsg{Text: "Preparing boot image", Percent: 0})

	dest := filepath.Join(cacheDir, bootImgName)

	if !fileExists(dest) {
		if err := downloadFile(ctx, bootImgURL, dest, func(pct float64) {
			send(ProgressMsg{Text: "Downloading boot.img", Percent: pct * 0.7})
		}); err != nil {
			return fmt.Errorf("downloading boot image: %w", err)
		}
	} else {
		send(ProgressMsg{Text: "boot.img cached", Percent: 0.7})
	}

	send(ProgressMsg{Text: "Pushing boot.img to device", Percent: 0.7})

	if err := adb.Push(dest, "/sdcard/boot.img"); err != nil {
		return fmt.Errorf("pushing boot image: %w", err)
	}

	// Also copy to SD card if present — Magisk file picker sometimes
	// doesn't show internal storage root.
	sdcard, err := adb.Shell("ls /storage/ 2>/dev/null")
	if err == nil {
		for _, entry := range strings.Split(strings.TrimSpace(sdcard), "\n") {
			entry = strings.TrimSpace(entry)
			if entry != "" && entry != "emulated" && entry != "self" {
				_, _ = adb.Shell("cp /sdcard/boot.img /storage/" + entry + "/boot.img")
				break
			}
		}
	}

	send(ProgressMsg{Text: "boot.img ready on device", Percent: 1.0})
	return nil
}

// PullPatchedImage finds the Magisk-patched boot image on the device and pulls
// it to the local cache directory. Returns the local file path.
func (r RootCommand) PullPatchedImage(cacheDir string) (string, error) {
	remote, err := adb.FindFile("/sdcard/Download/magisk_patched-*")
	if err != nil {
		return "", fmt.Errorf("finding patched image on device: %w", err)
	}

	localName := filepath.Base(remote)
	localPath := filepath.Join(cacheDir, localName)

	// Remove stale local copy if present.
	os.Remove(localPath)

	if err := adb.Pull(remote, localPath); err != nil {
		return "", fmt.Errorf("pulling patched image: %w", err)
	}

	return localPath, nil
}
