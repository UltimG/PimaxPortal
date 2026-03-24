package commands

import "testing"

func TestDriverFileList(t *testing.T) {
	files := DriverFileList()
	if len(files) != 34 {
		t.Fatalf("expected 34 driver files, got %d", len(files))
	}
	found := make(map[string]bool)
	for _, f := range files {
		found[f] = true
	}
	expected := []string{
		"lib64/hw/vulkan.kona.so",
		"lib/hw/vulkan.kona.so",
		"firmware/a650_gmu.bin",
		"lib64/libVkLayer_q3dtools.so",
	}
	for _, e := range expected {
		if !found[e] {
			t.Fatalf("missing expected file: %s", e)
		}
	}
	if found["lib/libVkLayer_q3dtools.so"] {
		t.Fatal("lib/libVkLayer_q3dtools.so should not be in the list (lib64 only)")
	}
}
