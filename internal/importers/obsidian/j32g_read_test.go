package obsidian

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadBoundedStopsAtTheLimit proves the read cannot be grown by a file
// that grows: it returns at most the limit, which is how the caller tells a
// note that is too large from one that fits (v1.0 J32-G).
func TestReadBoundedStopsAtTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.md")
	body := strings.Repeat("a", 1000)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	whole, err := readBounded(context.Background(), path, 1001)
	if err != nil || string(whole) != body {
		t.Fatalf("whole file: %d bytes, err %v", len(whole), err)
	}
	bounded, err := readBounded(context.Background(), path, 100)
	if err != nil || len(bounded) != 100 {
		t.Fatalf("bounded: %d bytes, err %v", len(bounded), err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readBounded(cancelled, path, 1001); err == nil {
		t.Fatal("a cancelled context must not read")
	}
}

// TestInventoryFingerprintsMatchStreamedHashes holds the hash taken from the
// bytes already read to the streaming hash it replaced, for notes and assets
// alike, including an empty note and one with multi-byte text.
func TestInventoryFingerprintsMatchStreamedHashes(t *testing.T) {
	vault := t.TempDir()
	files := map[string]string{
		"Note.md":              "---\naliases: [A]\n---\n# Note\n\nBody ελληνικά.\n",
		"empty.md":             "",
		"no-newline.md":        "# Ends without a newline",
		"Folder/Deep/Other.md": strings.Repeat("word ünïcode\n", 500),
		"assets/thing.bin":     strings.Repeat("\x00\x01binary", 300),
		"Folder/note.markdown": "# Markdown extension\n",
	}
	for name, body := range files {
		path := filepath.Join(vault, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inv, err := readInventory(context.Background(), vault, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Files) != len(files) {
		t.Fatalf("inventory holds %d files, wrote %d", len(inv.Files), len(files))
	}
	for _, file := range inv.Files {
		streamed, size, err := hashFile(context.Background(), filepath.Join(vault, filepath.FromSlash(file.RelPath)))
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(file.Fingerprint[:]); got != streamed {
			t.Errorf("%s: inventory hash %s, streamed hash %s", file.RelPath, got, streamed)
		}
		if file.SizeBytes != size {
			t.Errorf("%s: inventory size %d, streamed size %d", file.RelPath, file.SizeBytes, size)
		}
	}
}

// TestInventoryRefusesAnOversizeNote keeps the refusal the size check gives,
// which now runs before the read rather than after it.
func TestInventoryRefusesAnOversizeNote(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "huge.md")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, maxMarkdownBytes+1); err != nil {
		t.Skipf("cannot make a sparse oversize file here: %v", err)
	}
	_, err := readInventory(context.Background(), vault, false)
	if err == nil || !strings.Contains(err.Error(), "exceeds the") {
		t.Fatalf("expected the size refusal, got %v", err)
	}
}
