package commands

import (
	"runtime"
	"testing"
)

func TestADBInstallHint(t *testing.T) {
	hint := adbInstallHint(runtime.GOOS)
	if hint == "" {
		t.Fatal("expected non-empty hint for current OS")
	}
}

func TestADBInstallHint_AllPlatforms(t *testing.T) {
	for _, os := range []string{"darwin", "linux", "windows"} {
		hint := adbInstallHint(os)
		if hint == "" {
			t.Fatalf("expected hint for %s", os)
		}
	}
}
