package docexec

import (
	"archive/zip"
	"os"
	"path/filepath"
)

// zipTwitterFixture writes the extracted Twitter fixture as the ZIP a user
// downloads, with its data/ directory at the archive root (J25).
func zipTwitterFixture(dir, zipPath string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(out)
	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entry, err := writer.Create(filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		_, err = entry.Write(body)
		return err
	})
	closeErr := writer.Close()
	fileErr := out.Close()
	for _, err := range []error{walkErr, closeErr, fileErr} {
		if err != nil {
			return err
		}
	}
	return nil
}
