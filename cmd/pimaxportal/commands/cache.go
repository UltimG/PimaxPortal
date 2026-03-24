package commands

import (
	"os"
	"path/filepath"
)

type CacheState int

const (
	CacheEmpty       CacheState = iota
	CacheHasFirmware
	CacheHasVendor
	CacheHasDrivers
	CacheHasZip
)

func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "pimaxportal")
	return dir, os.MkdirAll(dir, 0755)
}

func CheckCacheState(cacheDir string) CacheState {
	if fileExists(filepath.Join(cacheDir, "build", "pimax-gpu-drivers.zip")) {
		return CacheHasZip
	}
	if fileExists(filepath.Join(cacheDir, "drivers", "lib64", "hw", "vulkan.kona.so")) {
		return CacheHasDrivers
	}
	if fileExists(filepath.Join(cacheDir, "extracted", "vendor_a.img")) {
		return CacheHasVendor
	}
	if fileExists(filepath.Join(cacheDir, "firmware", "rpmini.7z.001")) {
		return CacheHasFirmware
	}
	return CacheEmpty
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
