package obsidian

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestReadNoteReusesItsBuffer covers what a reused read buffer can get wrong:
// a shorter note after a longer one must not see the longer one's tail, the
// bytes must be the note's own, and a note that changed since the inventory
// hashed it must still be refused (v1.0 J32-W2).
func TestReadNoteReusesItsBuffer(t *testing.T) {
	vault := t.TempDir()
	run := &importRun{sourceDir: vault}

	bodies := []string{
		strings.Repeat("a long note with plenty of text\n", 400),
		"short\n",
		strings.Repeat("medium ελληνικά\n", 50),
		"",
		strings.Repeat("x", 4096),
	}
	for index, body := range bodies {
		name := filepath.Join(vault, "note.md")
		if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		item := vaultFile{
			RelPath:     "note.md",
			SizeBytes:   int64(len(body)),
			Fingerprint: sha256.Sum256([]byte(body)),
		}
		raw, err := run.readNote(item)
		if err != nil {
			t.Fatalf("note %d: %v", index, err)
		}
		if string(raw) != body {
			t.Fatalf("note %d: read %d bytes %q, wrote %d bytes", index, len(raw), truncate(string(raw)), len(body))
		}
	}
}

// TestReadNoteRefusesAChangedNote keeps the conflict the fingerprint check
// gives, which is what stops a note edited mid-import being imported as though
// it were the note the inventory saw.
func TestReadNoteRefusesAChangedNote(t *testing.T) {
	vault := t.TempDir()
	run := &importRun{sourceDir: vault}
	if err := os.WriteFile(filepath.Join(vault, "note.md"), []byte("changed since the scan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	item := vaultFile{
		RelPath:     "note.md",
		SizeBytes:   int64(len("what the scan saw\n")),
		Fingerprint: sha256.Sum256([]byte("what the scan saw\n")),
	}
	_, err := run.readNote(item)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected a conflict, got %v", err)
	}
}

// TestRetainNoteBufferStopsAtTheNoteLimit states the rule: a buffer up to the
// largest note the importer accepts is kept for the next note, and one beyond
// it -- grown by a file that is being refused anyway -- is given back (v1.0
// J32-W2).
func TestRetainNoteBufferStopsAtTheNoteLimit(t *testing.T) {
	for capacity, want := range map[int]bool{
		0:                        true,
		4096:                     true,
		maxRetainedNoteBytes - 1: true,
		maxRetainedNoteBytes:     true,
		maxRetainedNoteBytes + 1: false,
		maxRetainedNoteBytes * 2: false,
	} {
		if got := retainNoteBuffer(capacity); got != want {
			t.Errorf("retainNoteBuffer(%d) = %v, want %v", capacity, got, want)
		}
	}
	if maxRetainedNoteBytes != maxMarkdownBytes {
		t.Errorf("the retained buffer should be bounded by the note limit: %d against %d",
			maxRetainedNoteBytes, maxMarkdownBytes)
	}
}

func truncate(value string) string {
	if len(value) > 60 {
		return value[:60] + "…"
	}
	return value
}
