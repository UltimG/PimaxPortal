package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// eglLibs are the 6 EGL/GLES shared objects present in both lib/ and lib64/.
var eglLibs = []string{
	"eglSubDriverAndroid",
	"libEGL_adreno",
	"libGLESv1_CM_adreno",
	"libGLESv2_adreno",
	"libq3dtools_adreno",
	"libq3dtools_esx",
}

// supportLibs are the support shared objects present in both lib/ and lib64/.
// libVkLayer_q3dtools is 64-bit only and handled separately.
var supportLibs = []string{
	"libadreno_app_profiles",
	"libadreno_utils",
	"libgsl",
	"libllvm-glnext",
	"libllvm-qcom",
	"libllvm-qgl",
}

// firmwareFiles are the Adreno 650 firmware blobs.
var firmwareFiles = []string{
	"a650_gmu.bin",
	"a650_sqe.fw",
	"a650_zap.b00",
	"a650_zap.b01",
	"a650_zap.b02",
	"a650_zap.elf",
	"a650_zap.mdt",
}

// DriverFileList returns the 34 GPU driver file paths (relative to the vendor
// partition root) that must be extracted from vendor_a.img.
func DriverFileList() []string {
	var files []string

	files = append(files, "lib64/hw/vulkan.kona.so")
	for _, name := range eglLibs {
		files = append(files, fmt.Sprintf("lib64/egl/%s.so", name))
	}
	for _, name := range supportLibs {
		files = append(files, fmt.Sprintf("lib64/%s.so", name))
	}
	files = append(files, "lib64/libVkLayer_q3dtools.so")

	files = append(files, "lib/hw/vulkan.kona.so")
	for _, name := range eglLibs {
		files = append(files, fmt.Sprintf("lib/egl/%s.so", name))
	}
	for _, name := range supportLibs {
		files = append(files, fmt.Sprintf("lib/%s.so", name))
	}

	for _, name := range firmwareFiles {
		files = append(files, fmt.Sprintf("firmware/%s", name))
	}

	return files
}

// ExtractDrivers extracts the 34 GPU driver files from vendor_a.img using 7z.
func ExtractDrivers(ctx context.Context, cacheDir string, send func(ProgressMsg)) error {
	imgPath := filepath.Join(cacheDir, "extracted", "vendor_a.img")
	driversDir := filepath.Join(cacheDir, "drivers")

	if err := os.MkdirAll(driversDir, 0755); err != nil {
		return fmt.Errorf("creating drivers directory: %w", err)
	}

	files := DriverFileList()
	total := len(files)

	for i, relPath := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		send(ProgressMsg{
			Text:    "Extracting GPU drivers",
			Percent: float64(i) / float64(total),
		})

		destPath := filepath.Join(driversDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Use 7z to extract single file from ext4 image to stdout
		cmd := exec.CommandContext(ctx, "7z", "e", imgPath, relPath, fmt.Sprintf("-o%s", filepath.Dir(destPath)), "-y")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("extracting %s: %w", relPath, err)
		}

		if !fileExists(destPath) {
			return fmt.Errorf("7z did not produce %s", relPath)
		}
	}

	send(ProgressMsg{Text: "GPU drivers extracted", Percent: 1.0})
	return nil
}
