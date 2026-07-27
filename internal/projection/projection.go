// Package projection mirrors managed notes into a filesystem tree of Markdown
// files with YAML front matter. The projection is derived, reconstructible
// data: the Recoll sidecar indexes it (RECOLL_INTEGRATION.md), exporters may
// reuse it, and deleting it loses nothing. SQLite stays canonical.
package projection

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// Writer renders notes under Dir. Files are named by document ID so renames
// and moves never orphan projection files.
type Writer struct {
	Dir string
}

func (w Writer) notePath(documentID string) (string, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" || documentID == "." || documentID == ".." ||
		len(documentID) > 240 || filepath.Base(documentID) != documentID ||
		strings.ContainsAny(documentID, `/\`) {
		return "", fmt.Errorf("unsafe projection document ID %q", documentID)
	}
	return filepath.Join(w.Dir, "notes", documentID+".md"), nil
}

// WriteNote renders one note. Front matter carries the searchable fields the
// Recoll handler maps (title, author, author_id, published, tags, thread_id,
// reply_to, id, notebook, source_url).
func (w Writer) WriteNote(doc store.Document, source store.DocumentSource, tags []store.Tag, notebookName string) error {
	content := renderNote(doc, source, tags, notebookName)
	return w.writeNoteBytes(doc.ID, content)
}

func renderNote(doc store.Document, source store.DocumentSource, tags []store.Tag, notebookName string) []byte {
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
	return []byte(b.String())
}

func (w Writer) writeNoteBytes(documentID string, content []byte) error {
	path, err := w.notePath(documentID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".notrios-projection-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// RemoveNote deletes a projected note file; a missing file is not an error.
func (w Writer) RemoveNote(documentID string) error {
	path, err := w.notePath(documentID)
	if err != nil {
		return err
	}
	err = os.Remove(path)
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
	Jobs    int
	Batches int
	Backlog int
	Due     int
}

// SyncOutbox drains pending projection jobs. Job failures are recorded on the
// outbox row and never abort the pass.
func SyncOutbox(ctx context.Context, st store.Store, w Writer, limit int) (Report, error) {
	return DrainOutbox(ctx, st, w, limit, 1)
}

// DrainOutbox processes multiple bounded batches, then returns queue telemetry.
// Failed jobs receive their durable retry schedule in the Store and therefore
// do not block later sequence numbers or spin inside this pass.
func DrainOutbox(ctx context.Context, st store.Store, w Writer, batchSize, maxBatches int) (Report, error) {
	report := Report{}
	if batchSize <= 0 || batchSize > 500 {
		batchSize = 200
	}
	if maxBatches <= 0 || maxBatches > 100 {
		maxBatches = 20
	}
	for batch := 0; batch < maxBatches; batch++ {
		jobs, err := st.PendingProjectionJobs(ctx, batchSize)
		if err != nil {
			return report, err
		}
		if len(jobs) == 0 {
			break
		}
		report.Batches++
		report.Jobs += len(jobs)
		for _, job := range jobs {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			if job.ObjectType != "document" {
				if err := st.CompleteProjectionJob(ctx, job.Sequence, nil); err != nil {
					return report, err
				}
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
		if len(jobs) < batchSize {
			break
		}
	}
	queue, err := st.ProjectionQueueStatus(ctx)
	if err != nil {
		return report, err
	}
	report.Backlog = queue.Pending
	report.Due = queue.Due
	return report, nil
}

func projectionBytes(ctx context.Context, st store.Store, documentID string) (store.Document, []byte, error) {
	doc, err := st.GetDocument(ctx, documentID)
	if err != nil {
		return store.Document{}, nil, err
	}
	source, err := st.GetDocumentSource(ctx, doc.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.Document{}, nil, err
	}
	tags, err := st.ListDocumentTags(ctx, doc.ID)
	if err != nil {
		return store.Document{}, nil, err
	}
	notebookName := ""
	if doc.NotebookID != "" {
		if notebook, err := st.GetNotebook(ctx, doc.NotebookID); err == nil {
			notebookName = notebook.Name
		}
	}
	return doc, renderNote(doc, source, tags, notebookName), nil
}

func projectDocument(ctx context.Context, st store.Store, w Writer, documentID string) error {
	doc, content, err := projectionBytes(ctx, st, documentID)
	if err != nil {
		// A note upserted and then trashed before the worker ran: treat the
		// stale upsert as a removal.
		if errors.Is(err, store.ErrNotFound) {
			return w.RemoveNote(documentID)
		}
		return err
	}
	return w.writeNoteBytes(doc.ID, content)
}

// ReconcileReport describes an exact canonical/projection comparison.
type ReconcileReport struct {
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	Canonical    int       `json:"canonical"`
	FilesScanned int       `json:"files_scanned"`
	Missing      int       `json:"missing"`
	Stale        int       `json:"stale"`
	Orphaned     int       `json:"orphaned"`
	Repaired     int       `json:"repaired"`
	Failed       int       `json:"failed"`
	Complete     bool      `json:"complete"`
	Warnings     []string  `json:"warnings,omitempty"`
}

// Reconcile repairs missing/stale canonical projections and removes orphaned
// managed note files. Both canonical traversal and directory inspection are
// bounded by batchSize.
func Reconcile(ctx context.Context, st store.Store, w Writer, batchSize int) (ReconcileReport, error) {
	if batchSize <= 0 || batchSize > 500 {
		batchSize = 200
	}
	report := ReconcileReport{StartedAt: time.Now().UTC()}
	notebooks, err := st.ListNotebooks(ctx)
	if err != nil {
		return report, err
	}
	notebookNames := make(map[string]string, len(notebooks))
	for _, notebook := range notebooks {
		notebookNames[notebook.ID] = notebook.Name
	}
	collections, err := st.ListCollections(ctx)
	if err != nil {
		return report, err
	}
	for _, collection := range collections {
		if err := reconcileCanonicalCollection(ctx, st, w, collection.ID, notebookNames, batchSize, &report); err != nil {
			return report, err
		}
	}
	if err := reconcileOrphans(ctx, st, w, batchSize, &report); err != nil {
		return report, err
	}
	report.CompletedAt = time.Now().UTC()
	report.Complete = report.Failed == 0
	return report, nil
}

func reconcileCanonicalCollection(
	ctx context.Context,
	st store.Store,
	w Writer,
	collectionID string,
	notebookNames map[string]string,
	batchSize int,
	report *ReconcileReport,
) error {
	cursor := ""
	for {
		page, err := st.Search(ctx, store.SearchRequest{
			CollectionID: collectionID, Query: "", Limit: batchSize, Cursor: cursor,
		})
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(page.Hits))
		for _, hit := range page.Hits {
			ids = append(ids, hit.ID)
		}
		documents, err := st.GetDocuments(ctx, ids)
		if err != nil {
			return err
		}
		sources, err := st.GetDocumentSources(ctx, ids)
		if err != nil {
			return err
		}
		tags, err := st.GetDocumentTags(ctx, ids)
		if err != nil {
			return err
		}
		for _, hit := range page.Hits {
			if err := ctx.Err(); err != nil {
				return err
			}
			report.Canonical++
			doc, found := documents[hit.ID]
			if !found {
				report.recordFailure(fmt.Sprintf("canonical document %s disappeared during reconciliation", hit.ID))
				continue
			}
			expected := renderNote(doc, sources[doc.ID], tags[doc.ID], notebookNames[doc.NotebookID])
			path, err := w.notePath(doc.ID)
			if err != nil {
				report.recordFailure(err.Error())
				continue
			}
			matches, exists, err := fileMatches(path, expected)
			if err != nil {
				report.recordFailure(fmt.Sprintf("inspect %s: %v", doc.ID, err))
				continue
			}
			if matches {
				continue
			}
			if exists {
				report.Stale++
			} else {
				report.Missing++
			}
			if err := w.writeNoteBytes(doc.ID, expected); err != nil {
				report.recordFailure(fmt.Sprintf("repair %s: %v", doc.ID, err))
				continue
			}
			report.Repaired++
		}
		if page.NextCursor == "" {
			return nil
		}
		cursor = page.NextCursor
	}
}

func fileMatches(path string, expected []byte) (matches, exists bool, err error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, false, nil
	}
	if err != nil {
		return false, true, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, true, err
	}
	if info.Size() != int64(len(expected)) {
		return false, true, nil
	}
	actualHash := sha256.New()
	if _, err := io.Copy(actualHash, file); err != nil {
		return false, true, err
	}
	expectedHash := sha256.Sum256(expected)
	return bytes.Equal(actualHash.Sum(nil), expectedHash[:]), true, nil
}

func reconcileOrphans(ctx context.Context, st store.Store, w Writer, batchSize int, report *ReconcileReport) error {
	notesDir := filepath.Join(w.Dir, "notes")
	dir, err := os.Open(notesDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		entries, readErr := dir.ReadDir(batchSize)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		if len(entries) == 0 {
			return nil
		}
		ids := []string{}
		paths := map[string]string{}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			report.FilesScanned++
			name := entry.Name()
			path := filepath.Join(notesDir, name)
			if entry.IsDir() {
				report.recordFailure("unexpected directory in projection notes: " + name)
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(name, ".md") {
				report.Orphaned++
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					report.recordFailure(fmt.Sprintf("remove orphan %s: %v", name, err))
				} else {
					report.Repaired++
				}
				continue
			}
			id := strings.TrimSuffix(name, ".md")
			if _, err := w.notePath(id); err != nil {
				report.Orphaned++
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					report.recordFailure(fmt.Sprintf("remove unsafe orphan %s: %v", name, err))
				} else {
					report.Repaired++
				}
				continue
			}
			ids = append(ids, id)
			paths[id] = path
		}
		documents, err := st.GetDocuments(ctx, ids)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, found := documents[id]; found {
				continue
			}
			report.Orphaned++
			if err := os.Remove(paths[id]); err != nil && !os.IsNotExist(err) {
				report.recordFailure(fmt.Sprintf("remove orphan %s: %v", id, err))
			} else {
				report.Repaired++
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
	}
}

func (r *ReconcileReport) recordFailure(message string) {
	r.Failed++
	if len(r.Warnings) < 100 {
		r.Warnings = append(r.Warnings, message)
	} else if len(r.Warnings) == 100 {
		r.Warnings = append(r.Warnings, "additional reconciliation warnings omitted")
	}
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
	return fmt.Sprintf("projection: %d jobs/%d batches, %d written, %d removed, %d failed, %d pending",
		r.Jobs, r.Batches, r.Written, r.Removed, r.Failed, r.Backlog)
}
