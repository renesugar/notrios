// Package helpdocs seeds the built-in read-only "Help" notebook from the
// Markdown files under docs/ (Notrios redesign task R15). The same content
// builds the GitHub Pages documentation site, so the docs are available
// offline inside the app (`notebook:help` searches them). Seeding is
// deterministic and repeatable: notes are keyed by file path, updated in
// place, and removed when their source file disappears.
package helpdocs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// Report summarizes one seeding run.
type Report struct {
	FilesSeen    int      `json:"files_seen"`
	NotesCreated int      `json:"notes_created"`
	NotesUpdated int      `json:"notes_updated"`
	NotesKept    int      `json:"notes_kept"`
	NotesRemoved int      `json:"notes_removed"`
	Warnings     []string `json:"warnings,omitempty"`
}

// Seed mirrors docsDir into the Help notebook.
func Seed(ctx context.Context, st store.Store, docsDir string) (Report, error) {
	report := Report{}
	wanted := map[string]bool{}

	err := filepath.WalkDir(docsDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(docsDir, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		report.FilesSeen++
		docID := helpNoteID(rel)
		wanted[docID] = true
		return upsertHelpNote(ctx, st, docID, rel, string(raw), &report)
	})
	if err != nil {
		return report, err
	}

	// Remove help notes whose source file is gone (trash, then purge — the
	// notes are local, so permanent deletion is allowed).
	page, err := st.ListNotebookDocuments(ctx, store.HelpNotebookID, store.DocumentPageRequest{Limit: 500})
	if err != nil {
		return report, err
	}
	for _, doc := range page.Documents {
		if wanted[doc.ID] {
			continue
		}
		if err := st.DeleteDocument(ctx, store.DeleteDocumentRequest{ID: doc.ID, BaseRevisionID: doc.CurrentRevisionID, Message: "help docs seed removal"}); err != nil {
			return report, err
		}
		if err := st.PurgeDocument(ctx, doc.ID); err != nil {
			return report, err
		}
		report.NotesRemoved++
	}
	return report, nil
}

func helpNoteID(relPath string) string {
	slug := strings.TrimSuffix(filepath.ToSlash(relPath), ".md")
	slug = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '_'
		}
	}, slug)
	return "doc_help_" + strings.Trim(slug, "_")
}

func titleFrom(relPath, body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(trimmed[2:])
		}
	}
	base := strings.ReplaceAll(strings.TrimSuffix(filepath.Base(relPath), ".md"), "-", " ")
	if base == "" {
		return "Help"
	}
	return strings.ToUpper(base[:1]) + base[1:]
}

func upsertHelpNote(ctx context.Context, st store.Store, docID, relPath, body string, report *Report) error {
	title := titleFrom(relPath, body)
	existing, err := st.GetDocument(ctx, docID)
	switch {
	case err == nil:
		if existing.Title == title && existing.Body == body {
			report.NotesKept++
			return nil
		}
		if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID: docID, Title: title, Body: body, BodyMIMEType: "text/markdown",
			BaseRevisionID: existing.CurrentRevisionID, Message: "help docs seed update",
		}); err != nil {
			return err
		}
		report.NotesUpdated++
		return nil
	case errors.Is(err, store.ErrNotFound):
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: docID, NotebookID: store.HelpNotebookID,
			Title: title, Body: body, BodyMIMEType: "text/markdown",
			Message: "help docs seed",
		}); err != nil {
			return fmt.Errorf("seed %s: %w", relPath, err)
		}
		report.NotesCreated++
		return nil
	default:
		return err
	}
}
