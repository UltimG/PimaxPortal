package commands

import "testing"

func TestSuperImagePath(t *testing.T) {
	expected := "flash/lun0_super.bin"
	if superImagePath != expected {
		t.Fatalf("expected %s, got %s", expected, superImagePath)
	}
}
