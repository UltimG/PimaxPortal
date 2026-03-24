package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
)

const (
	firmwareRepo = "TheGammaSqueeze/Retroid_Pocket_Stock_Firmware"
	firmwareTag  = "Android10_RPMini_V1.0.0.310_20240926_181623_user"
)

// FirmwareURLs returns the 4 direct GitHub release asset URLs for the split
// firmware archive.
func FirmwareURLs() []string {
	urls := make([]string, 4)
	for i := range urls {
		urls[i] = fmt.Sprintf(
			"https://github.com/%s/releases/download/%s/%s.7z.%03d",
			firmwareRepo, firmwareTag, firmwareTag, i+1,
		)
	}
	return urls
}

// FirmwareFilenames returns the local filenames for the 4 split archive parts.
func FirmwareFilenames() []string {
	names := make([]string, 4)
	for i := range names {
		names[i] = fmt.Sprintf("rpmini.7z.%03d", i+1)
	}
	return names
}

// DownloadFirmware downloads all 4 firmware split archives into
// cacheDir/firmware/. It reports progress through send and respects
// cancellation via ctx.
func DownloadFirmware(ctx context.Context, cacheDir string, send func(ProgressMsg)) error {
	firmwareDir := filepath.Join(cacheDir, "firmware")
	if err := os.MkdirAll(firmwareDir, 0755); err != nil {
		return fmt.Errorf("creating firmware directory: %w", err)
	}

	// Warn if free disk space is under 20 GB.
	free, err := diskFree(firmwareDir)
	if err == nil && free < 20*1024*1024*1024 {
		send(ProgressMsg{
			Text:    fmt.Sprintf("WARNING: only %.1f GB free disk space (recommend >= 20 GB)", float64(free)/(1024*1024*1024)),
			Percent: -1,
		})
	}

	urls := FirmwareURLs()
	names := FirmwareFilenames()

	for i, url := range urls {
		dest := filepath.Join(firmwareDir, names[i])
		label := fmt.Sprintf("Downloading %s (%d/%d)", names[i], i+1, len(urls))

		send(ProgressMsg{Text: label, Percent: float64(i) / float64(len(urls))})

		progress := func(pct float64) {
			// Map per-file 0-1 into the overall 0-1 range across all files.
			overall := (float64(i) + pct) / float64(len(urls))
			send(ProgressMsg{Text: label, Percent: overall})
		}

		if err := downloadFile(ctx, url, dest, progress); err != nil {
			return fmt.Errorf("downloading %s: %w", names[i], err)
		}
	}

	send(ProgressMsg{Text: "Firmware download complete", Percent: 1.0})
	return nil
}

// downloadFile downloads url to dest. If dest already exists with partial
// content, it resumes via an HTTP Range header. progress is called with a
// value in [0,1] as data arrives.
func downloadFile(ctx context.Context, url, dest string, progress func(float64)) error {
	// Determine how many bytes we already have (for resume).
	var existingSize int64
	if info, err := os.Stat(dest); err == nil {
		existingSize = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// Full download — start from scratch.
		existingSize = 0
	case http.StatusPartialContent:
		// Resume in progress — append to existing file.
	case http.StatusRequestedRangeNotSatisfiable:
		// File is already complete.
		if progress != nil {
			progress(1.0)
		}
		return nil
	default:
		return fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength + existingSize

	// Open or create file for writing.
	flags := os.O_CREATE | os.O_WRONLY
	if existingSize > 0 && resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(dest, flags, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	var downloaded int64

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := f.Write(buf[:n]); wErr != nil {
				return wErr
			}
			downloaded += int64(n)
			if progress != nil && totalSize > 0 {
				progress(float64(existingSize+downloaded) / float64(totalSize))
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	if progress != nil {
		progress(1.0)
	}
	return nil
}

// diskFree returns the number of free bytes available on the filesystem
// containing path. This uses syscall.Statfs which works on macOS and Linux.
// On unsupported platforms this will fail at compile time; a build-tagged
// stub can be added for Windows if needed.
func diskFree(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// Available blocks * block size gives free bytes for unprivileged users.
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}
