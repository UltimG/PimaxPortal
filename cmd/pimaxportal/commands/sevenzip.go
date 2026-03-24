package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
)

const superImagePath = "flash/lun0_super.bin"

func ExtractSuperImage(ctx context.Context, cacheDir string, send func(ProgressMsg)) error {
	archivePath := filepath.Join(cacheDir, "firmware", "rpmini.7z.001")
	destDir := filepath.Join(cacheDir, "extracted")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	send(ProgressMsg{Text: "Opening firmware archive", Percent: -1})
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open 7z archive: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !strings.HasSuffix(f.Name, superImagePath) && f.Name != superImagePath {
			continue
		}
		send(ProgressMsg{Text: fmt.Sprintf("Extracting %s (%.1f GB)", f.Name, float64(f.UncompressedSize)/(1024*1024*1024)), Percent: -1})
		destPath := filepath.Join(destDir, superImagePath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s in archive: %w", f.Name, err)
		}
		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return err
		}
		totalSize := int64(f.UncompressedSize)
		var extracted int64
		_, err = copyWithCancel(ctx, out, rc, func(n int64) {
			extracted += n
			if totalSize > 0 {
				send(ProgressMsg{
					Text:    fmt.Sprintf("Extracting firmware (%.1f GB)", float64(totalSize)/(1024*1024*1024)),
					Percent: float64(extracted) / float64(totalSize),
				})
			}
		})
		rc.Close()
		out.Close()
		if err != nil {
			os.Remove(destPath)
			return fmt.Errorf("extract %s: %w", f.Name, err)
		}
		send(ProgressMsg{Text: "Super partition extracted", Percent: 1.0})
		return nil
	}
	return fmt.Errorf("%s not found in archive", superImagePath)
}
