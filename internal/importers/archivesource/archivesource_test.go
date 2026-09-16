package archivesource

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/tempspace"
)

// J26: the archive reader is shared by every importer that takes a downloaded
// export. These fixtures are synthetic; no real archive content is here.

func testSpec() Spec {
	return Spec{
		IsDataFile: func(base string) bool { return base == "conversations.json" },
		NotAnArchive: func(name string, err error) error {
			return errors.New(name + " is not a test archive: " + err.Error())
		},
		NoDataDirectory: func(where, kind string) error {
			return errors.New("no conversations.json in " + where + " (" + kind + ")")
		},
	}
}

func writeZip(t *testing.T, path string, files map[string]string) string {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for name, body := range files {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func read(t *testing.T, source Source, name string) string {
	t.Helper()
	file, err := source.Open(name, Limits.DataFileBytes)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(body)
}

var archiveFiles = map[string]string{
	"data/conversations.json": `[{"id":"a"},{"id":"b"}]`,
	"data/users.json":         `[{"id":"u"}]`,
	"data/files/one.png":      "PNGDATA",
	"data/files/two.png":      "PNGDATA2",
}

func TestAFolderAndAZipReadTheSame(t *testing.T) {
	dir := writeDir(t, archiveFiles)
	zipPath := writeZip(t, filepath.Join(t.TempDir(), "export.zip"), archiveFiles)
	for kind, path := range map[string]string{KindDirectory: dir, KindZip: zipPath} {
		source, rejected, err := Open(path, testSpec())
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		defer source.Close()
		if source.Kind() != kind || rejected != 0 {
			t.Errorf("kind=%s rejected=%d, want %s and 0", source.Kind(), rejected, kind)
		}
		if got := read(t, source, "conversations.json"); got != archiveFiles["data/conversations.json"] {
			t.Errorf("%s: conversations.json = %q", kind, got)
		}
		names, err := source.List("")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(names, ",") != "conversations.json,users.json" {
			t.Errorf("%s: List(\"\") = %v, want the data directory's files, sorted", kind, names)
		}
		media, err := source.List("files")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(media, ",") != "one.png,two.png" {
			t.Errorf("%s: List(files) = %v", kind, media)
		}
	}
}

func TestTheDataDirectoryIsFoundAtEitherDepth(t *testing.T) {
	flat := writeZip(t, filepath.Join(t.TempDir(), "flat.zip"), map[string]string{
		"conversations.json": "[]",
	})
	nestedUnderFolder := writeZip(t, filepath.Join(t.TempDir(), "deep.zip"), map[string]string{
		"export-2026-01-01/data/conversations.json": "[]",
		"export-2026-01-01/data/files/one.png":      "P",
	})
	for name, path := range map[string]string{"flat": flat, "wrapped": nestedUnderFolder} {
		source, _, err := Open(path, testSpec())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer source.Close()
		if _, err := source.Open("conversations.json", Limits.DataFileBytes); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestUnsafeEntryNamesAreRefusedAndCounted(t *testing.T) {
	path := writeZip(t, filepath.Join(t.TempDir(), "hostile.zip"), map[string]string{
		"data/conversations.json":     "[]",
		"data/files/../../escape.png": "ESCAPED",
		"/data/files/absolute.png":    "ABSOLUTE",
		`data\files\backslash.png`:    "BACKSLASH",
		"data/files/":                 "",
	})
	source, rejected, err := Open(path, testSpec())
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if rejected != 3 {
		t.Errorf("rejected=%d, want the three unsafe names (the directory entry is not a refusal)", rejected)
	}
	if _, err := source.Open("../escape.png", Limits.MediaFileBytes); err == nil {
		t.Error("an unsafe name must be refused as a lookup key too")
	}
}

func TestEachBoundIsEnforced(t *testing.T) {
	t.Run("entries", func(t *testing.T) {
		previous := Limits.Entries
		Limits.Entries = 2
		t.Cleanup(func() { Limits.Entries = previous })
		_, _, err := Open(writeZip(t, filepath.Join(t.TempDir(), "many.zip"), archiveFiles), testSpec())
		if !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), "entries") {
			t.Fatalf("err=%v, want ErrTooLarge naming entries", err)
		}
	})
	t.Run("data file", func(t *testing.T) {
		previous := Limits.DataFileBytes
		Limits.DataFileBytes = 8
		t.Cleanup(func() { Limits.DataFileBytes = previous })
		for kind, path := range map[string]string{
			KindZip:       writeZip(t, filepath.Join(t.TempDir(), "big.zip"), archiveFiles),
			KindDirectory: writeDir(t, archiveFiles),
		} {
			source, _, err := Open(path, testSpec())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := source.Open("conversations.json", Limits.DataFileBytes); !errors.Is(err, ErrTooLarge) {
				t.Errorf("%s: err=%v, want ErrTooLarge", kind, err)
			}
			source.Close()
		}
	})
	t.Run("a file larger than it claims", func(t *testing.T) {
		limited := NewLimitedReader(strings.NewReader(strings.Repeat("x", 1024)), nil, 64, "claimed-small")
		read, err := io.Copy(io.Discard, limited)
		if !errors.Is(err, ErrTooLarge) || read > 64 {
			t.Fatalf("read %d bytes, err=%v; want ErrTooLarge at no more than 64", read, err)
		}
	})
}

func TestAZipInsideAZipIsReadInPlace(t *testing.T) {
	innerPath := writeZip(t, filepath.Join(t.TempDir(), "inner.zip"), archiveFiles)
	innerBody, err := os.ReadFile(innerPath)
	if err != nil {
		t.Fatal(err)
	}
	outerPath := writeZip(t, filepath.Join(t.TempDir(), "outer.zip"), map[string]string{
		"data/conversations.json":            "[]",
		"data/nested/Conversations__001.zip": string(innerBody),
	})
	outer, _, err := Open(outerPath, testSpec())
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Close()

	t.Run("in memory with no instance temp space", func(t *testing.T) {
		inner, rejected, err := Nested(nil, outer, "nested/Conversations__001.zip", testSpec())
		if err != nil {
			t.Fatal(err)
		}
		defer inner.Close()
		if rejected != 0 || inner.Kind() != KindZip {
			t.Errorf("rejected=%d kind=%s", rejected, inner.Kind())
		}
		if got := read(t, inner, "conversations.json"); got != archiveFiles["data/conversations.json"] {
			t.Errorf("nested conversations.json = %q", got)
		}
	})

	t.Run("spooled into the instance temp space", func(t *testing.T) {
		root := t.TempDir()
		space, err := tempspace.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		defer space.Close()
		inner, _, err := Nested(space, outer, "nested/Conversations__001.zip", testSpec())
		if err != nil {
			t.Fatal(err)
		}
		if got := read(t, inner, "conversations.json"); got != archiveFiles["data/conversations.json"] {
			t.Errorf("nested conversations.json = %q", got)
		}
		spooled, err := filepath.Glob(filepath.Join(space.Dir(), "notrios-import-nested-*", "nested.zip"))
		if err != nil || len(spooled) != 1 {
			t.Fatalf("the nested archive must spool into the instance temp space: %v %v", spooled, err)
		}
		if err := inner.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Dir(spooled[0])); !errors.Is(err, os.ErrNotExist) {
			t.Error("closing a nested source must remove what it spooled")
		}
	})

	t.Run("a nested archive too large for memory", func(t *testing.T) {
		previous := Limits.NestedInMemoryBytes
		Limits.NestedInMemoryBytes = 16
		t.Cleanup(func() { Limits.NestedInMemoryBytes = previous })
		if _, _, err := Nested(nil, outer, "nested/Conversations__001.zip", testSpec()); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("err=%v, want ErrTooLarge", err)
		}
	})
}

func TestNotAnArchiveAndNoDataFileAreRefusedInTheCallersWords(t *testing.T) {
	plain := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(plain, []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(plain, testSpec()); err == nil || !strings.Contains(err.Error(), "is not a test archive") {
		t.Fatalf("err=%v, want the caller's wording", err)
	}
	empty := writeZip(t, filepath.Join(t.TempDir(), "empty.zip"), map[string]string{"data/users.json": "[]"})
	if _, _, err := Open(empty, testSpec()); err == nil || !strings.Contains(err.Error(), "no conversations.json") {
		t.Fatalf("err=%v, want the caller's wording", err)
	}
}

func TestDecodeArrayStreamsEntries(t *testing.T) {
	type entry struct {
		ID string `json:"id"`
	}
	ids := []string{}
	count, err := DecodeArray(bytes.NewReader([]byte(`[{"id":"a"},{"id":"b"},{"id":"c"}]`)), "conversations.json",
		func(e entry) error {
			ids = append(ids, e.ID)
			return nil
		})
	if err != nil || count != 3 || strings.Join(ids, "") != "abc" {
		t.Fatalf("count=%d ids=%v err=%v", count, ids, err)
	}
	if _, err := DecodeArray(bytes.NewReader([]byte(`{"id":"a"}`)), "conversations.json", func(entry) error { return nil }); err == nil {
		t.Fatal("an object where an array belongs must be refused")
	}
	stop := errors.New("stop")
	if _, err := DecodeArray(bytes.NewReader([]byte(`[{"id":"a"},{"id":"b"}]`)), "c.json", func(entry) error { return stop }); !errors.Is(err, stop) {
		t.Fatalf("err=%v, want the callback's error", err)
	}
}
