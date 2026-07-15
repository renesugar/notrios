// Package projection mirrors managed notes into a filesystem tree of Markdown
// files with YAML front matter. The projection is derived, reconstructible
// data: the Recoll sidecar indexes it (RECOLL_INTEGRATION.md), exporters may
// reuse it, and deleting it loses nothing. SQLite stays canonical.
package projection

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// Writer renders notes under Dir. Files are named by document ID so renames
// and moves never orphan projection files.
type Writer struct {
	Dir string
}

func (w Writer) notePath(documentID string) string {
	return filepath.Join(w.Dir, "notes", documentID+".md")
}

// WriteNote renders one note. Front matter carries the searchable fields the
// Recoll handler maps (title, author, author_id, published, tags, thread_id,
// reply_to, id, notebook, source_url).
func (w Writer) WriteNote(doc store.Document, source store.DocumentSource, tags []store.Tag, notebookName string) error {
	var b strings.Builder
	b.WriteString("---\n")
	writeScalar(&b, "id", doc.ID)
	writeScalar(&b, "title", doc.Title)
	writeScalar(&b, "notebook", notebookName)
	writeScalar(&b, "author", source.Author)
	writeScalar(&b, "author_id", source.AuthorID)
	writeScalar(&b, "published", source.PublishedAt)
	writeScalar(&b, "thread_id", source.ThreadID)
	writeScalar(&b, "reply_to", source.ReplyTo)
	writeScalar(&b, "source_url", source.SourceURL)
	if len(tags) > 0 {
		b.WriteString("tags:\n")
		for _, tag := range tags {
			b.WriteString("  - " + yamlQuote(tag.Name) + "\n")
		}
	}
	b.WriteString("---\n\n")
	b.WriteString(doc.Body)
	if !strings.HasSuffix(doc.Body, "\n") {
		b.WriteString("\n")
	}

	path := w.notePath(doc.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RemoveNote deletes a projected note file; a missing file is not an error.
func (w Writer) RemoveNote(documentID string) error {
	err := os.Remove(w.notePath(documentID))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func writeScalar(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString(key + ": " + yamlQuote(value) + "\n")
}

func yamlQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", " ")
	return `"` + value + `"`
}

// Report summarizes one sync pass.
type Report struct {
	Written int
	Removed int
	Failed  int
}

// SyncOutbox drains pending projection jobs. Job failures are recorded on the
// outbox row and never abort the pass.
func SyncOutbox(ctx context.Context, st store.Store, w Writer, limit int) (Report, error) {
	report := Report{}
	jobs, err := st.PendingProjectionJobs(ctx, limit)
	if err != nil {
		return report, err
	}
	for _, job := range jobs {
		if job.ObjectType != "document" {
			_ = st.CompleteProjectionJob(ctx, job.Sequence, nil)
			continue
		}
		var jobErr error
		switch job.Operation {
		case "delete":
			jobErr = w.RemoveNote(job.ObjectID)
			if jobErr == nil {
				report.Removed++
			}
		default:
			jobErr = projectDocument(ctx, st, w, job.ObjectID)
			if jobErr == nil {
				report.Written++
			}
		}
		if jobErr != nil {
			report.Failed++
		}
		if err := st.CompleteProjectionJob(ctx, job.Sequence, jobErr); err != nil {
			return report, err
		}
	}
	return report, nil
}

func projectDocument(ctx context.Context, st store.Store, w Writer, documentID string) error {
	doc, err := st.GetDocument(ctx, documentID)
	if err != nil {
		// A note upserted and then trashed before the worker ran: treat the
		// stale upsert as a removal.
		if errors.Is(err, store.ErrNotFound) {
			return w.RemoveNote(documentID)
		}
		return err
	}
	source, err := st.GetDocumentSource(ctx, doc.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	tags, err := st.ListDocumentTags(ctx, doc.ID)
	if err != nil {
		return err
	}
	notebookName := ""
	if doc.NotebookID != "" {
		if nb, err := st.GetNotebook(ctx, doc.NotebookID); err == nil {
			notebookName = nb.Name
		}
	}
	return w.WriteNote(doc, source, tags, notebookName)
}

// FullSync projects every current non-deleted note, for databases that
// predate the outbox wiring or projections that were deleted. It pages
// through the store with search cursors.
func FullSync(ctx context.Context, st store.Store, w Writer) (Report, error) {
	report := Report{}
	cursor := ""
	for {
		page, err := st.Search(ctx, store.SearchRequest{Query: "", Limit: 100, Cursor: cursor})
		if err != nil {
			return report, err
		}
		for _, hit := range page.Hits {
			if err := projectDocument(ctx, st, w, hit.ID); err != nil {
				report.Failed++
				continue
			}
			report.Written++
		}
		if page.NextCursor == "" {
			return report, nil
		}
		cursor = page.NextCursor
	}
}

// String renders the report for logs.
func (r Report) String() string {
	return fmt.Sprintf("projection: %d written, %d removed, %d failed", r.Written, r.Removed, r.Failed)
}
