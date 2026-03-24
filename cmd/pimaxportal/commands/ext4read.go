package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/filesystem"
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

	// 64-bit core: vulkan HAL + 6 EGL libs = 7
	files = append(files, "lib64/hw/vulkan.kona.so")
	for _, name := range eglLibs {
		files = append(files, fmt.Sprintf("lib64/egl/%s.so", name))
	}

	// 64-bit support: 6 common + libVkLayer_q3dtools = 7
	for _, name := range supportLibs {
		files = append(files, fmt.Sprintf("lib64/%s.so", name))
	}
	files = append(files, "lib64/libVkLayer_q3dtools.so")

	// 32-bit core: vulkan HAL + 6 EGL libs = 7
	files = append(files, "lib/hw/vulkan.kona.so")
	for _, name := range eglLibs {
		files = append(files, fmt.Sprintf("lib/egl/%s.so", name))
	}

	// 32-bit support: 6 common (no libVkLayer_q3dtools) = 6
	for _, name := range supportLibs {
		files = append(files, fmt.Sprintf("lib/%s.so", name))
	}

	// Firmware: 7 files
	for _, name := range firmwareFiles {
		files = append(files, fmt.Sprintf("firmware/%s", name))
	}

	return files
}

// ExtractDrivers opens vendor_a.img from cacheDir/extracted/ and extracts the
// 34 GPU driver files into cacheDir/drivers/. Progress is reported via send.
func ExtractDrivers(ctx context.Context, cacheDir string, send func(ProgressMsg)) error {
	imgPath := filepath.Join(cacheDir, "extracted", "vendor_a.img")
	driversDir := filepath.Join(cacheDir, "drivers")

	send(ProgressMsg{Text: "Opening vendor_a.img...", Percent: 0})

	disk, err := diskfs.Open(imgPath)
	if err != nil {
		return fmt.Errorf("opening vendor_a.img: %w", err)
	}

	fs, err := disk.GetFilesystem(0)
	if err != nil {
		return fmt.Errorf("reading ext4 filesystem: %w", err)
	}

	files := DriverFileList()
	total := len(files)

	for i, relPath := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		srcPath := "/" + relPath
		destPath := filepath.Join(driversDir, relPath)

		send(ProgressMsg{
			Text:    fmt.Sprintf("Extracting %s (%d/%d)", relPath, i+1, total),
			Percent: float64(i) / float64(total),
		})

		if err := extractFileFromFS(fs, srcPath, destPath); err != nil {
			return fmt.Errorf("extracting %s: %w", relPath, err)
		}
	}

	// Validate all files were extracted.
	for _, relPath := range files {
		destPath := filepath.Join(driversDir, relPath)
		if !fileExists(destPath) {
			return fmt.Errorf("missing extracted file: %s", relPath)
		}
	}

	send(ProgressMsg{Text: "Driver extraction complete", Percent: 1.0})
	return nil
}

// extractFileFromFS reads srcPath from the ext4 filesystem and writes it to
// destPath on the local filesystem.
func extractFileFromFS(fs filesystem.FileSystem, srcPath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	src, err := fs.OpenFile(srcPath, os.O_RDONLY)
	if err != nil {
		return fmt.Errorf("opening %s in image: %w", srcPath, err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", destPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copying %s: %w", srcPath, err)
	}

	return nil
}
