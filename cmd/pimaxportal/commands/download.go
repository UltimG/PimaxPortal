package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// httpClient with no idle timeout — default client kills long downloads.
var httpClient = &http.Client{
	Timeout: 0, // No overall timeout — we handle cancellation via context
}

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

	label := "Downloading RP Mini Firmware"

	for i, url := range urls {
		dest := filepath.Join(firmwareDir, names[i])

		send(ProgressMsg{Text: label, Percent: float64(i) / float64(len(urls))})

		progress := func(pct float64) {
			overall := (float64(i) + pct) / float64(len(urls))
			send(ProgressMsg{Text: label, Percent: overall})
		}

		if err := downloadFile(ctx, url, dest, progress); err != nil {
			// Only delete partial on user cancel — network errors keep the partial for resume
			if ctx.Err() != nil {
				os.Remove(dest)
			}
			return fmt.Errorf("downloading firmware: %w", err)
		}
	}

	send(ProgressMsg{Text: "RP Mini Firmware downloaded", Percent: 1.0})
	return nil
}

const maxRetries = 10

// downloadFile downloads url to dest with automatic retry and resume on
// connection drops. progress is called with a value in [0,1] as data arrives.
func downloadFile(ctx context.Context, url, dest string, progress func(float64)) error {
	// Get expected total size from a HEAD-like first request.
	var expectedSize int64

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Brief pause before retry.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}

		// Check how much we already have on disk.
		var existingSize int64
		if info, err := os.Stat(dest); err == nil {
			existingSize = info.Size()
		}

		// If we know the expected size and already have it, we're done.
		if expectedSize > 0 && existingSize >= expectedSize {
			if progress != nil {
				progress(1.0)
			}
			return nil
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		if existingSize > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			if attempt < maxRetries {
				continue
			}
			return err
		}

		switch resp.StatusCode {
		case http.StatusOK:
			expectedSize = resp.ContentLength
		case http.StatusPartialContent:
			if expectedSize == 0 {
				expectedSize = resp.ContentLength + existingSize
			}
		case http.StatusRequestedRangeNotSatisfiable:
			resp.Body.Close()
			if progress != nil {
				progress(1.0)
			}
			return nil
		default:
			resp.Body.Close()
			return fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
		}

		// Open file for writing.
		flags := os.O_CREATE | os.O_WRONLY
		if resp.StatusCode == http.StatusPartialContent {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
			existingSize = 0
		}
		f, err := os.OpenFile(dest, flags, 0644)
		if err != nil {
			resp.Body.Close()
			return err
		}

		buf := make([]byte, 256*1024)
		broken := false

		for {
			select {
			case <-ctx.Done():
				f.Close()
				resp.Body.Close()
				os.Remove(dest)
				return ctx.Err()
			default:
			}

			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, wErr := f.Write(buf[:n]); wErr != nil {
					f.Close()
					resp.Body.Close()
					return wErr
				}
				existingSize += int64(n)
				if progress != nil && expectedSize > 0 {
					progress(float64(existingSize) / float64(expectedSize))
				}
			}
			if readErr != nil {
				if readErr == io.EOF {
					break
				}
				// Connection dropped — close and retry.
				broken = true
				break
			}
		}

		f.Close()
		resp.Body.Close()

		if !broken {
			// Download completed successfully.
			if progress != nil {
				progress(1.0)
			}
			return nil
		}
		// Loop will retry with Range header from current existingSize.
	}

	return fmt.Errorf("download failed after %d retries", maxRetries)
}

