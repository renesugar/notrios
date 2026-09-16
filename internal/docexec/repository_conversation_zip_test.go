package docexec

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
)

// J26: the ChatGPT and Claude importers take the archive as downloaded, so the
// documentation's own examples run against ZIPs built from the same fixtures —
// including the OpenAI Privacy Portal's shape, whose conversations live in a
// ZIP inside the ZIP.

// zipPortalFixture writes dir as a Privacy Portal export: an outer ZIP whose
// "User Online Activity" folder holds the conversations ZIP.
func zipPortalFixture(dir, zipPath string) error {
	inner := &bytes.Buffer{}
	innerWriter := zip.NewWriter(inner)
	if err := addTree(innerWriter, dir); err != nil {
		return err
	}
	if err := innerWriter.Close(); err != nil {
		return err
	}
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(out)
	entry, err := writer.Create("User Online Activity/Conversations__fixture-chatgpt-0001.zip")
	if err != nil {
		out.Close()
		return err
	}
	if _, err := entry.Write(inner.Bytes()); err != nil {
		out.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// addTree writes every regular file under dir into writer, at its path
// relative to dir.
func addTree(writer *zip.Writer, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
}
