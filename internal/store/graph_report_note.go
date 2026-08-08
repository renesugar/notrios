package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// The hubs report, written into the library as an ordinary Markdown note.
//
// A ranked list is readable at any library size, which is exactly what a global
// graph canvas stops being — that is the whole argument for this deliverable
// over the one it replaced.
//
// Three properties make it trustworthy rather than merely present:
//
//   - **A stable ID, overwritten in place.** Regenerating does not litter the
//     library with dated copies, and a link to the report keeps working.
//   - **Read-only**, because it lives in the Reports notebook. A generated
//     report a reader can edit is a report that silently stops being true.
//   - **Explicitly regenerated**, never on a schedule and never on write. It
//     reads the whole collection, and a whole-collection scan on every save
//     would be the one unbounded thing in an otherwise bounded design. The note
//     therefore carries its generation time so a reader can judge its age.

const (
	// GraphReportNoteID is the note the report is always written to.
	GraphReportNoteID = "doc_graph_report"
	// GraphReportNoteTitle names it in the sidebar and in search results.
	GraphReportNoteTitle = "Library graph report"
	// GraphReportNoteItems is how many entries each list in the note shows.
	// Smaller than the API default: this is something a person reads.
	GraphReportNoteItems = 20
)

// WriteGraphReportNote regenerates the report and stores it as a note.
//
// It returns the note together with the report it was rendered from, so a
// caller can print the numbers without reading the note back and parsing its
// own output.
func (s *SQLiteStore) WriteGraphReportNote(ctx context.Context, req GraphReportRequest) (Document, GraphReport, error) {
	ctx = contextOrBackground(ctx)
	if req.Limit == 0 {
		req.Limit = GraphReportNoteItems
	}
	report, err := s.GraphReport(ctx, req)
	if err != nil {
		return Document{}, GraphReport{}, err
	}
	body := renderGraphReportNote(report, time.Now().UTC())

	existing, err := s.GetDocument(ctx, GraphReportNoteID)
	switch {
	case err == nil:
		updated, err := s.UpdateDocument(ctx, UpdateDocumentRequest{
			ID:             existing.ID,
			Title:          GraphReportNoteTitle,
			Body:           body,
			BodyMIMEType:   existing.BodyMIMEType,
			BaseRevisionID: existing.CurrentRevisionID,
			Message:        "regenerated graph report",
		})
		return updated, report, err
	case errors.Is(err, ErrNotFound):
		created, err := s.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID:  GraphReportNoteID,
			CollectionID: req.CollectionID,
			NotebookID:   ReportsNotebookID,
			Title:        GraphReportNoteTitle,
			Body:         body,
			Message:      "generated graph report",
		})
		return created, report, err
	default:
		return Document{}, GraphReport{}, err
	}
}

// renderGraphReportNote turns a report into Markdown.
//
// Links use canonical `document://` URIs so the report is navigable and lint
// has nothing to say about it.
func renderGraphReportNote(report GraphReport, generatedAt time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", GraphReportNoteTitle)
	fmt.Fprintf(&b, "Generated %s from collection `%s` in %.1f ms.\n\n",
		generatedAt.Format("2006-01-02 15:04 MST"), report.CollectionID, report.ElapsedMS)
	b.WriteString("This note is regenerated on request and **overwritten in place**; it is\n")
	b.WriteString("read-only, so edits to it would not survive the next run. Notes in the Trash\n")
	b.WriteString("and in read-only notebooks are not measured, and neither are their links —\n")
	b.WriteString("including this note's own links to the hubs below.\n\n")

	b.WriteString("| Measure | Count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Notes measured | %d |\n", report.DocumentCount)
	fmt.Fprintf(&b, "| Links between measured notes | %d |\n", report.LinkCount)
	fmt.Fprintf(&b, "| Orphans (nothing links to them) | %d |\n", report.OrphanCount)
	fmt.Fprintf(&b, "| Isolated (no link either way) | %d |\n", report.IsolatedCount)
	b.WriteString("\n")

	writeGraphReportList(&b, "Hubs, by incoming links", report.Hubs,
		"Nothing in this library is linked to yet.", report.Limit)
	writeGraphReportList(&b, "Orphans", report.Orphans,
		"Every note is linked to by something.", report.Limit)
	return b.String()
}

func writeGraphReportList(b *strings.Builder, heading string, entries []GraphReportEntry, empty string, limit int) {
	fmt.Fprintf(b, "## %s\n\n", heading)
	if len(entries) == 0 {
		fmt.Fprintf(b, "%s\n\n", empty)
		return
	}
	for _, entry := range entries {
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			title = entry.DocumentID
		}
		fmt.Fprintf(b, "- %s — %d in, %d out\n",
			graphReportLink(title, entry.URI), entry.InDegree, entry.OutDegree)
	}
	if len(entries) >= limit {
		fmt.Fprintf(b, "\nShowing the first %d; the counts above are complete.\n", limit)
	}
	b.WriteString("\n")
}

// graphReportLink labels a link with a note's title without letting the title
// break it.
//
// A title is arbitrary user text, and Notrios' own link parser forbids `]`
// inside link text — **including a backslash-escaped one**, since the pattern
// is `\[([^\]\n]*)\]\(`. So escaping is not available: a `]` in a title
// would produce a link the store itself cannot resolve, in a generated note the
// user cannot edit, and the user's lint report would carry the blame.
//
// Rather than altering the title to fit, a title carrying `]` is written as
// plain text beside a short link. The title stays verbatim, the link resolves,
// and the common case — a title with no bracket in it — reads exactly as
// before.
func graphReportLink(title, uri string) string {
	title = strings.NewReplacer("\n", " ", "\r", " ").Replace(title)
	if strings.Contains(title, "]") {
		return title + " ([open](" + uri + "))"
	}
	return "[" + title + "](" + uri + ")"
}
