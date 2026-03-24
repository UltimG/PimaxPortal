package commands

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	pimaxportal "github.com/UltimG/PimaxPortal"
)

const moduleZipName = "pimax-gpu-drivers.zip"

func PackageModule(driversDir, buildDir string) (string, error) {
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", err
	}

	zipPath := filepath.Join(buildDir, moduleZipName)
	f, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// Add template files from embedded FS
	templateRoot := "modules/gpu-drivers"
	if err := fs.WalkDir(pimaxportal.ModuleTemplate, templateRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath := strings.TrimPrefix(path, templateRoot+"/")
		data, err := pimaxportal.ModuleTemplate.ReadFile(path)
		if err != nil {
			return err
		}
		header := &zip.FileHeader{Name: relPath}
		if strings.HasSuffix(relPath, "update-binary") {
			header.SetMode(0755)
		} else {
			header.SetMode(0644)
		}
		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	}); err != nil {
		return "", fmt.Errorf("walk template: %w", err)
	}

	// Add extracted driver files under system/vendor/
	err = filepath.Walk(driversDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relPath, _ := filepath.Rel(driversDir, path)
		zipEntryPath := "system/vendor/" + filepath.ToSlash(relPath)
		header := &zip.FileHeader{Name: zipEntryPath}
		header.SetMode(0644)
		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(writer, src)
		return err
	})

	return zipPath, err
}
