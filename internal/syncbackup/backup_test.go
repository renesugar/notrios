package syncbackup

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, KeyBytes)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestFramesRoundTripAcrossFrameBoundaries(t *testing.T) {
	key := testKey(t)
	for name, size := range map[string]int{
		"empty":                0,
		"one byte":             1,
		"just under one frame": FrameBytes - 1,
		"exactly one frame":    FrameBytes,
		"just over one frame":  FrameBytes + 1,
		"several frames":       3*FrameBytes + 17,
	} {
		payload := make([]byte, size)
		if _, err := rand.Read(payload); err != nil {
			t.Fatal(err)
		}
		var sealed bytes.Buffer
		length, digest, err := Seal(bytes.NewReader(payload), key, &sealed)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if length != int64(sealed.Len()) || digest == "" {
			t.Fatalf("%s: sealed length %d does not match %d", name, length, sealed.Len())
		}
		var opened bytes.Buffer
		if err := Open(bytes.NewReader(sealed.Bytes()), key, &opened); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(opened.Bytes(), payload) {
			t.Fatalf("%s: %d bytes came back as %d", name, size, opened.Len())
		}
	}
}

func TestASealedBackupNeedsItsOwnKeyAndItsOwnBytes(t *testing.T) {
	key := testKey(t)
	payload := bytes.Repeat([]byte("notrios snapshot payload\n"), 100_000)
	var sealed bytes.Buffer
	if _, _, err := Seal(bytes.NewReader(payload), key, &sealed); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed.Bytes(), []byte("notrios snapshot payload")) {
		t.Fatal("the sealed backup contains its own plaintext")
	}

	var opened bytes.Buffer
	if err := Open(bytes.NewReader(sealed.Bytes()), testKey(t), &opened); err == nil {
		t.Fatal("a different key opened the backup")
	}

	for name, mutate := range map[string]func([]byte) []byte{
		"a flipped bit": func(value []byte) []byte {
			altered := append([]byte(nil), value...)
			altered[len(altered)/2] ^= 0x01
			return altered
		},
		// Truncation is the case that matters most for a resumed download: an
		// incomplete transfer must not open as a complete backup.
		"truncation": func(value []byte) []byte { return value[:len(value)-FrameBytes/2] },
		"a dropped final frame": func(value []byte) []byte {
			return value[:len(value)/2]
		},
		"wrong magic": func(value []byte) []byte {
			altered := append([]byte(nil), value...)
			altered[0] = 'X'
			return altered
		},
	} {
		var out bytes.Buffer
		if err := Open(bytes.NewReader(mutate(sealed.Bytes())), key, &out); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestFramesCannotBeReordered(t *testing.T) {
	key := testKey(t)
	payload := make([]byte, 3*FrameBytes)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	var sealed bytes.Buffer
	if _, _, err := Seal(bytes.NewReader(payload), key, &sealed); err != nil {
		t.Fatal(err)
	}
	raw := sealed.Bytes()
	// Each frame is 8 header bytes plus its sealed body, all the same size
	// until the last. Swapping the first two bodies is what a carrier that
	// reassembled ranges out of order would produce.
	const header = 8
	frame := (len(raw) - len(Magic)) / 4
	first := len(Magic) + header
	second := first + frame
	if second+frame-header > len(raw) {
		t.Skip("frame arithmetic does not fit this payload")
	}
	swapped := append([]byte(nil), raw...)
	copy(swapped[first:first+frame-header], raw[second:second+frame-header])
	copy(swapped[second:second+frame-header], raw[first:first+frame-header])
	var out bytes.Buffer
	if err := Open(bytes.NewReader(swapped), key, &out); err == nil && bytes.Equal(out.Bytes(), payload) {
		t.Fatal("reordered frames reassembled into the original payload")
	}
}

func TestPackAndUnpackRoundTripDeterministically(t *testing.T) {
	source := t.TempDir()
	for _, name := range []string{"manifest.json", "objects/ab/cd/one", "objects/ef/01/two"} {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("contents of "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var first, second bytes.Buffer
	if _, err := Pack(source, &first); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(source, &second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("two packs of one directory produced different bytes")
	}

	container := filepath.Join(t.TempDir(), "container.tar")
	if err := os.WriteFile(container, first.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := Unpack(container, destination); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"manifest.json", "objects/ab/cd/one", "objects/ef/01/two"} {
		contents, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(name)))
		if err != nil || string(contents) != "contents of "+name {
			t.Fatalf("%s did not survive the round trip: %v", name, err)
		}
	}
}

// TestUnpackRefusesAnEscapingEntry is the check that matters for a container
// that arrives from a peer. A peer is authenticated, not trusted, and a tar
// entry named ../../etc/something is the oldest trick there is.
func TestUnpackRefusesAnEscapingEntry(t *testing.T) {
	container := filepath.Join(t.TempDir(), "hostile.tar")
	file, err := os.Create(container)
	if err != nil {
		t.Fatal(err)
	}
	writer := tar.NewWriter(file)
	for _, name := range []string{"../escaped.txt", "objects/../../escaped-too.txt"} {
		contents := []byte("should never be written")
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	file.Close()

	destination := t.TempDir()
	err = Unpack(container, destination)
	if err == nil {
		t.Fatal("a container that escapes its destination was extracted")
	}
	if !strings.Contains(err.Error(), "not safe to extract") {
		t.Fatalf("the refusal came from somewhere else: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(destination), "escaped.txt")); statErr == nil {
		t.Fatal("a file was written outside the destination")
	}
}
