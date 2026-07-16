// Package archive implements the native Notrios export/import format
// (redesign task R12). An archive is a directory holding a manifest, the
// exported notebook structure, notes as Markdown with YAML front matter, and
// referenced resource bytes.
//
// Exports may be scoped to a query (a search notebook's contents instead of
// the whole database). Exported notes become plain notes on re-import: no
// provenance rows are written, so re-imported notes are ordinary local notes,
// not references to their original data source.
package archive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

const manifestVersion = 1

// Manifest describes one archive.
type Manifest struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	Query     string `json:"query,omitempty"`
	CreatedAt string `json:"created_at"`
}

// NotebookEntry captures an exported notebook by path so nesting survives.
type NotebookEntry struct {
	Path      string `json:"path"` // "Work/Reports"
	IconEmoji string `json:"icon_emoji,omitempty"`
}

// ExportOptions scopes an export.
type ExportOptions struct {
	Query        string // Notrios query language; empty = all non-deleted notes
	CollectionID string
}

// ExportReport summarizes an export.
type ExportReport struct {
	Notes     int      `json:"notes"`
	Notebooks int      `json:"notebooks"`
	Resources int      `json:"resources"`
	Query     string   `json:"query,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Export writes the archive directory.
func Export(ctx context.Context, st store.Store, outDir string, options ExportOptions) (ExportReport, error) {
	report := ExportReport{Query: options.Query}
	if strings.TrimSpace(options.CollectionID) == "" {
		options.CollectionID = "default"
	}
	if err := os.MkdirAll(filepath.Join(outDir, "notes"), 0o755); err != nil {
		return report, err
	}

	notebookPaths := map[string]NotebookEntry{}
	exportedResources := map[string]bool{}

	cursor := ""
	for {
		page, err := st.Search(ctx, store.SearchRequest{Query: options.Query, CollectionID: options.CollectionID, Limit: 100, Cursor: cursor})
		if err != nil {
			return report, err
		}
		for _, hit := range page.Hits {
			doc, err := st.GetDocument(ctx, hit.ID)
			if err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("note %s skipped: %v", hit.ID, err))
				continue
			}
			chain, err := notebookChain(ctx, st, doc.NotebookID)
			if err != nil {
				return report, err
			}
			path := ""
			for i, level := range chain {
				prefix := strings.Join(chainNames(chain[:i+1]), "/")
				notebookPaths[prefix] = NotebookEntry{Path: prefix, IconEmoji: level.IconEmoji}
				path = prefix
			}
			tags, err := st.ListDocumentTags(ctx, doc.ID)
			if err != nil {
				return report, err
			}
			refs, err := st.ListDocumentResources(ctx, doc.ID)
			if err != nil {
				return report, err
			}
			for _, ref := range refs {
				if exportedResources[ref.ResourceID] {
					continue
				}
				if err := exportResource(ctx, st, outDir, ref.ResourceID); err != nil {
					report.Warnings = append(report.Warnings, fmt.Sprintf("resource %s skipped: %v", ref.ResourceID, err))
					continue
				}
				exportedResources[ref.ResourceID] = true
				report.Resources++
			}
			if err := writeNoteFile(outDir, doc, path, tags, refs); err != nil {
				return report, err
			}
			report.Notes++
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	entries := make([]NotebookEntry, 0, len(notebookPaths))
	for _, entry := range notebookPaths {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	if err := writeJSONFile(filepath.Join(outDir, "notebooks.json"), entries); err != nil {
		return report, err
	}
	report.Notebooks = len(entries)

	manifest := Manifest{Format: "notrios-archive", Version: manifestVersion, Query: options.Query, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	return report, writeJSONFile(filepath.Join(outDir, "manifest.json"), manifest)
}

// notebookChain returns the notebook and its ancestors, root first, so every
// level's name and emoji survive export.
func notebookChain(ctx context.Context, st store.Store, notebookID string) ([]store.Notebook, error) {
	if strings.TrimSpace(notebookID) == "" {
		return nil, nil
	}
	chain := []store.Notebook{}
	current := notebookID
	for current != "" && len(chain) < 32 {
		nb, err := st.GetNotebook(ctx, current)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				break
			}
			return nil, err
		}
		chain = append([]store.Notebook{nb}, chain...)
		current = nb.ParentID
	}
	return chain, nil
}

func chainNames(chain []store.Notebook) []string {
	names := make([]string, 0, len(chain))
	for _, nb := range chain {
		names = append(names, nb.Name)
	}
	return names
}

func writeNoteFile(outDir string, doc store.Document, notebookPath string, tags []store.Tag, refs []store.ResourceReference) error {
	var b strings.Builder
	b.WriteString("---\n")
	writeScalar(&b, "id", doc.ID)
	writeScalar(&b, "title", doc.Title)
	writeScalar(&b, "notebook", notebookPath)
	writeScalar(&b, "created", doc.CreatedAt.UTC().Format(time.RFC3339))
	writeScalar(&b, "updated", doc.UpdatedAt.UTC().Format(time.RFC3339))
	if len(tags) > 0 {
		b.WriteString("tags:\n")
		for _, tag := range tags {
			b.WriteString("  - " + quote(tag.Name) + "\n")
		}
	}
	if len(refs) > 0 {
		b.WriteString("resources:\n")
		for _, ref := range refs {
			b.WriteString("  - " + quote(ref.ResourceID+"|"+ref.Resource.Filename) + "\n")
		}
	}
	b.WriteString("---\n\n")
	b.WriteString(doc.Body)
	if !strings.HasSuffix(doc.Body, "\n") {
		b.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(outDir, "notes", doc.ID+".md"), []byte(b.String()), 0o644)
}

func exportResource(ctx context.Context, st store.Store, outDir, resourceID string) error {
	resource, content, err := st.OpenResourceContent(ctx, resourceID)
	if err != nil {
		return err
	}
	defer content.Close()
	dir := filepath.Join(outDir, "resources")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out, err := os.Create(filepath.Join(dir, resource.ID+"__"+filepath.Base(resource.Filename)))
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, content)
	return err
}

func writeScalar(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString(key + ": " + quote(value) + "\n")
}

func quote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", " ")
	return `"` + value + `"`
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
