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

// TestReadNoteGivesBackALargeBuffer proves the cap: a note past it leaves
// nothing retained, so one large note does not make an import hold its buffer
// to the end.
func TestReadNoteGivesBackALargeBuffer(t *testing.T) {
	vault := t.TempDir()
	run := &importRun{sourceDir: vault}
	small := "small\n"
	if err := os.WriteFile(filepath.Join(vault, "note.md"), []byte(small), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run.readNote(vaultFile{RelPath: "note.md", SizeBytes: int64(len(small)), Fingerprint: sha256.Sum256([]byte(small))}); err != nil {
		t.Fatal(err)
	}
	if cap(run.noteBytes) == 0 {
		t.Fatal("an ordinary note's buffer should be kept")
	}

	large := strings.Repeat("y", maxRetainedNoteBytes+1024)
	if err := os.WriteFile(filepath.Join(vault, "large.md"), []byte(large), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run.readNote(vaultFile{RelPath: "large.md", SizeBytes: int64(len(large)), Fingerprint: sha256.Sum256([]byte(large))}); err != nil {
		t.Fatal(err)
	}
	if run.noteBytes != nil {
		t.Fatalf("a buffer past the cap must be given back, kept %d bytes", cap(run.noteBytes))
	}
}

func truncate(value string) string {
	if len(value) > 60 {
		return value[:60] + "…"
	}
	return value
}
