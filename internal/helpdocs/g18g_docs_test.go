package helpdocs

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// TestG18gRepositoryDocsAreSeededByteForByte proves that the repository
// manual, rather than a fixture or generated copy, is the Help notebook's
// source of truth. The test deliberately resolves docs/ from this file so it
// remains independent of the caller's working directory.
func TestG18gRepositoryDocsAreSeededByteForByte(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	docsDir := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", "docs"))

	if info, err := os.Stat(docsDir); err != nil {
		t.Fatalf("stat docs directory %s: %v", docsDir, err)
	} else if !info.IsDir() {
		t.Fatalf("docs path is not a directory: %s", docsDir)
	}
	var paths []string
	var files = make(map[string][]byte)
	var collect func(string) error
	collect = func(dir string) error {
		items, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, item := range items {
			path := filepath.Join(dir, item.Name())
			if item.IsDir() {
				if err := collect(path); err != nil {
					return err
				}
				continue
			}
			if filepath.Ext(item.Name()) != ".md" {
				continue
			}
			rel, err := filepath.Rel(docsDir, path)
			if err != nil {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			paths = append(paths, rel)
			files[rel] = raw
		}
		return nil
	}
	if err := collect(docsDir); err != nil {
		t.Fatalf("collect repository docs: %v", err)
	}
	sort.Strings(paths)
	// 18 -> 19 in v1.0 J12: docs/configuration.md. The README referred a reader
	// to a detailed configuration document that did not exist, for a surface of
	// 53 settable keys documented by one example file. The count is pinned so
	// that a page reaching the Help notebook is a decision somebody made.
	if len(paths) != 19 {
		t.Fatalf("expected exactly 19 repository Markdown files, got %d (%v)", len(paths), paths)
	}

	ctx := context.Background()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	first, err := Seed(ctx, st, docsDir)
	if err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	if first.FilesSeen != 19 || first.NotesCreated != 19 || first.NotesUpdated != 0 || first.NotesRemoved != 0 {
		t.Fatalf("unexpected first seed report: %+v", first)
	}

	page, err := st.ListNotebookDocuments(ctx, store.HelpNotebookID, store.DocumentPageRequest{Limit: 100})
	if err != nil {
		t.Fatalf("ListNotebookDocuments: %v", err)
	}
	if len(page.Documents) != 19 {
		t.Fatalf("expected 18 Help notes, got %d", len(page.Documents))
	}
	for _, rel := range paths {
		doc, err := st.GetDocument(ctx, helpNoteID(rel))
		if err != nil {
			t.Fatalf("GetDocument(%s): %v", rel, err)
		}
		body := string(files[rel])
		if doc.ID != helpNoteID(rel) || doc.NotebookID != store.HelpNotebookID {
			t.Fatalf("unstable Help identity for %s: id=%q notebook=%q", rel, doc.ID, doc.NotebookID)
		}
		if doc.Title != titleFrom(rel, body) {
			t.Fatalf("wrong title for %s: got %q want %q", rel, doc.Title, titleFrom(rel, body))
		}
		if doc.Body != body {
			t.Fatalf("body changed for %s: got %d bytes want %d", rel, len(doc.Body), len(body))
		}
	}

	second, err := Seed(ctx, st, docsDir)
	if err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	if second.FilesSeen != 19 || second.NotesKept != 19 || second.NotesCreated != 0 || second.NotesUpdated != 0 || second.NotesRemoved != 0 {
		t.Fatalf("non-idempotent second seed report: %+v", second)
	}
}
