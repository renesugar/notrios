package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestJ19FTSInsideVersusAfterTransaction measures the external performance
// review's claim that writing the full-text index inside the import transaction
// amplifies write cost, and that a separate post-import sweep would be cheaper.
//
// Both modes write the same rows the import create path writes (document,
// revision, projection job) in batches of 100. "inline" also writes the
// full-text row inside each batch, as ApplyImportDocumentBatch does today.
// "after" writes nothing to the index during the batches, then indexes every
// document in its own batches of 100 afterwards. The number that matters is
// the time until the last note is searchable, so "after" is timed through the
// end of its sweep.
//
// The bodies come from a JSONL file of {"title","body"} objects exported from
// a real library, kept outside the repository:
//
//	NOTRIOS_J19_BODIES=/disk/bodies.jsonl NOTRIOS_J19_DIR=/disk/under/test \
//	  go test ./internal/store -run TestJ19FTSInsideVersusAfterTransaction -count=1 -v -timeout 60m
//
// The replicated write path is a measurement aid, not a proposal: if "after"
// wins, the product change still goes to the owner as a decision, because it
// makes notes invisible to search until the sweep reaches them.
func TestJ19FTSInsideVersusAfterTransaction(t *testing.T) {
	bodiesPath := os.Getenv("NOTRIOS_J19_BODIES")
	dir := os.Getenv("NOTRIOS_J19_DIR")
	if bodiesPath == "" || dir == "" {
		t.Skip("set NOTRIOS_J19_BODIES and NOTRIOS_J19_DIR to measure full-text write placement on a real disk")
	}
	notes := readJ19Bodies(t, bodiesPath)
	const batchSize = 100
	for _, mode := range []string{"inline", "after", "inline", "after"} {
		elapsed, searchable := runJ19FTSMode(t, dir, mode, notes, batchSize)
		t.Logf("%-6s %d notes: %s until the last note is searchable (%.2f ms/note); index rows %d",
			mode, len(notes), elapsed.Round(time.Millisecond),
			float64(elapsed.Microseconds())/1000/float64(len(notes)), searchable)
	}
}

type j19Note struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func readJ19Bodies(t *testing.T, path string) []j19Note {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 64<<20)
	notes := []j19Note{}
	for scanner.Scan() {
		var note j19Note
		if err := json.Unmarshal(scanner.Bytes(), &note); err != nil {
			t.Fatal(err)
		}
		notes = append(notes, note)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(notes) == 0 {
		t.Fatal("no notes in the bodies file")
	}
	return notes
}

func runJ19FTSMode(t *testing.T, dir, mode string, notes []j19Note, batchSize int) (time.Duration, int) {
	t.Helper()
	path := filepath.Join(dir, fmt.Sprintf("j19-fts-%s-%d.sqlite", mode, time.Now().UnixNano()))
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(path + suffix); err == nil {
			t.Fatalf("%s already exists", path+suffix)
		}
	}
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = st.Close()
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(path + suffix)
		}
	}()
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(notes))
	for index := range notes {
		ids[index] = fmt.Sprintf("doc_j19_%07d", index)
	}
	inline := mode == "inline"

	started := time.Now()
	for start := 0; start < len(notes); start += batchSize {
		end := min(start+batchSize, len(notes))
		if err := st.j19WriteBatch(ids[start:end], notes[start:end], inline); err != nil {
			t.Fatal(err)
		}
	}
	if !inline {
		for start := 0; start < len(notes); start += batchSize {
			end := min(start+batchSize, len(notes))
			if err := st.j19IndexBatch(ids[start:end], notes[start:end]); err != nil {
				t.Fatal(err)
			}
		}
	}
	elapsed := time.Since(started)

	st.mu.Lock()
	defer st.mu.Unlock()
	searchable, err := st.countLocked(`SELECT count(*) FROM documents_fts_rowid`)
	if err != nil {
		t.Fatal(err)
	}
	if searchable != int64(len(notes)) {
		t.Fatalf("%s: %d of %d notes are indexed", mode, searchable, len(notes))
	}
	return elapsed, int(searchable)
}

func (s *SQLiteStore) j19WriteBatch(ids []string, notes []j19Note, withFTS bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	for index, note := range notes {
		revisionID, err := NewID("rev")
		if err != nil {
			return err
		}
		if err := s.execPreparedLocked(`INSERT INTO documents(id, collection_id, notebook_id, title, body_mime_type, current_revision_id)
			VALUES(?, ?, ?, ?, ?, ?)`, ids[index], "default", DefaultNotebookID, note.Title, "text/markdown", revisionID); err != nil {
			return err
		}
		if err := s.insertRevisionLocked(revisionID, ids[index], note.Title, note.Body, "text/markdown", "j19", ""); err != nil {
			return err
		}
		if withFTS {
			if err := s.insertDocumentFTSLocked(ids[index], "default", note.Title, note.Body); err != nil {
				return err
			}
		}
		if err := s.enqueueProjectionLocked(ids[index], "upsert"); err != nil {
			return err
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SQLiteStore) j19IndexBatch(ids []string, notes []j19Note) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	for index, note := range notes {
		if err := s.insertDocumentFTSLocked(ids[index], "default", note.Title, note.Body); err != nil {
			return err
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}
