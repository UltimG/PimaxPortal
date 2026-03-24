package commands

import (
	"context"
	"fmt"
	"path/filepath"
)

func init() {
	Register(&BuildCommand{})
}

type BuildCommand struct{}

func (b *BuildCommand) Name() string        { return "build" }
func (b *BuildCommand) Description() string { return "Build GPU Driver Module" }

func (b *BuildCommand) Run(ctx context.Context, send func(ProgressMsg)) error {
	cacheDir, err := CacheDir()
	if err != nil {
		return fmt.Errorf("cache dir: %w", err)
	}

	state := CheckCacheState(cacheDir)

	if state == CacheHasZip {
		send(ProgressMsg{Text: "Module already built (cached). Skipping build.", Percent: 1.0})
		return nil
	}

	if state < CacheHasFirmware {
		send(ProgressMsg{Text: "Step 1/5: Downloading RP Mini firmware...", Percent: -1})
		if err := DownloadFirmware(ctx, cacheDir, send); err != nil {
			return fmt.Errorf("download: %w", err)
		}
	}

	if state < CacheHasVendor {
		send(ProgressMsg{Text: "Step 2/5: Extracting firmware archive...", Percent: -1})
		if err := ExtractSuperImage(ctx, cacheDir, send); err != nil {
			return fmt.Errorf("extract archive: %w", err)
		}

		send(ProgressMsg{Text: "Step 3/5: Extracting vendor partition...", Percent: -1})
		if err := ExtractVendorPartition(ctx, cacheDir, send); err != nil {
			return fmt.Errorf("lpunpack: %w", err)
		}
	}

	if state < CacheHasDrivers {
		send(ProgressMsg{Text: "Step 4/5: Extracting GPU drivers...", Percent: -1})
		if err := ExtractDrivers(ctx, cacheDir, send); err != nil {
			return fmt.Errorf("extract drivers: %w", err)
		}
	}

	send(ProgressMsg{Text: "Step 5/5: Packaging Magisk module...", Percent: -1})
	driversDir := filepath.Join(cacheDir, "drivers")
	buildDir := filepath.Join(cacheDir, "build")
	zipPath, err := PackageModule(driversDir, buildDir)
	if err != nil {
		return fmt.Errorf("package: %w", err)
	}

	send(ProgressMsg{Text: fmt.Sprintf("Module built: %s", zipPath), Percent: 1.0})
	return nil
}
