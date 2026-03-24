package commands

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageModule(t *testing.T) {
	tmp := t.TempDir()
	driversDir := filepath.Join(tmp, "drivers")
	buildDir := filepath.Join(tmp, "build")

	os.MkdirAll(filepath.Join(driversDir, "lib64", "hw"), 0755)
	os.WriteFile(filepath.Join(driversDir, "lib64", "hw", "vulkan.kona.so"), []byte("fake-driver"), 0644)

	zipPath, err := PackageModule(driversDir, buildDir)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("zip not created: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	foundDriver := false
	foundProp := false
	for _, f := range r.File {
		if f.Name == "system/vendor/lib64/hw/vulkan.kona.so" {
			foundDriver = true
		}
		if f.Name == "module.prop" {
			foundProp = true
		}
		if f.Name == "META-INF/com/google/android/update-binary" {
			if f.Mode()&0111 == 0 {
				t.Fatal("update-binary should be executable")
			}
		}
	}
	if !foundDriver {
		t.Fatal("driver not found in zip")
	}
	if !foundProp {
		t.Fatal("module.prop not found in zip")
	}
}
