package commands

import "testing"

func TestFirmwareURLs(t *testing.T) {
	urls := FirmwareURLs()
	if len(urls) != 4 {
		t.Fatalf("expected 4 URLs, got %d", len(urls))
	}
	for i, u := range urls {
		if u == "" {
			t.Fatalf("URL %d is empty", i)
		}
	}
}

func TestFirmwareFilenames(t *testing.T) {
	names := FirmwareFilenames()
	if len(names) != 4 {
		t.Fatalf("expected 4 filenames, got %d", len(names))
	}
	if names[0] != "rpmini.7z.001" {
		t.Fatalf("expected rpmini.7z.001, got %s", names[0])
	}
}
