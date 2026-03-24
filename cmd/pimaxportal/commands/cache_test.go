package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheDir(t *testing.T) {
	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Fatal("expected non-empty cache dir")
	}
}

func TestCacheState_Empty(t *testing.T) {
	tmp := t.TempDir()
	state := CheckCacheState(tmp)
	if state != CacheEmpty {
		t.Fatalf("expected CacheEmpty, got %v", state)
	}
}

func TestCacheState_HasZip(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "build"), 0755)
	os.WriteFile(filepath.Join(tmp, "build", "pimax-gpu-drivers.zip"), []byte("fake"), 0644)
	state := CheckCacheState(tmp)
	if state != CacheHasZip {
		t.Fatalf("expected CacheHasZip, got %v", state)
	}
}

func TestCacheState_HasDrivers(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "drivers", "lib64", "hw"), 0755)
	os.WriteFile(filepath.Join(tmp, "drivers", "lib64", "hw", "vulkan.kona.so"), []byte("fake"), 0644)
	state := CheckCacheState(tmp)
	if state != CacheHasDrivers {
		t.Fatalf("expected CacheHasDrivers, got %v", state)
	}
}

func TestCacheState_HasVendorImg(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "extracted"), 0755)
	os.WriteFile(filepath.Join(tmp, "extracted", "vendor_a.img"), []byte("fake"), 0644)
	state := CheckCacheState(tmp)
	if state != CacheHasVendor {
		t.Fatalf("expected CacheHasVendor, got %v", state)
	}
}

func TestCacheState_HasFirmware(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "firmware"), 0755)
	os.WriteFile(filepath.Join(tmp, "firmware", "rpmini.7z.001"), []byte("fake"), 0644)
	state := CheckCacheState(tmp)
	if state != CacheHasFirmware {
		t.Fatalf("expected CacheHasFirmware, got %v", state)
	}
}
