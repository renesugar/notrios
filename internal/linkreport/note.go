package linkreport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// The report, written into the library as a read-only note (v1.0 J40-D), the
// way the graph report is: a stable ID overwritten in place, in the Reports
// notebook, regenerated only when asked. The read-only-notebook predicate keeps
// it out of lint, the graph report, publication handoffs and this report, so a
// candidate's title cannot reach a consumer through it.

const (
	// NoteID is the note the report is always written to.
	NoteID = "doc_link_report"
	// NoteTitle names it in the sidebar and in search results.
	NoteTitle = "Ambiguous links report"
)

// NoteStore is what writing the note needs. *store.SQLiteStore is one.
type NoteStore interface {
	Source
	CreateDocument(ctx context.Context, req store.CreateDocumentRequest) (store.Document, error)
	UpdateDocument(ctx context.Context, req store.UpdateDocumentRequest) (store.Document, error)
}

// WriteNote regenerates the report and stores it as the Reports note. It
// returns the note with the report it was rendered from.
func WriteNote(ctx context.Context, st NoteStore, req Request) (store.Document, Report, error) {
	report, err := Generate(ctx, st, req)
	if err != nil {
		return store.Document{}, Report{}, err
	}
	body := RenderNote(report, time.Now().UTC())
	existing, err := st.GetDocument(ctx, NoteID)
	switch {
	case err == nil:
		updated, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID: existing.ID, Title: NoteTitle, Body: body, BodyMIMEType: existing.BodyMIMEType,
			BaseRevisionID: existing.CurrentRevisionID, Message: "regenerated ambiguous links report",
		})
		return updated, report, err
	case errors.Is(err, store.ErrNotFound):
		created, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: NoteID, CollectionID: req.CollectionID, NotebookID: store.ReportsNotebookID,
			Title: NoteTitle, Body: body, Message: "generated ambiguous links report",
		})
		return created, report, err
	default:
		return store.Document{}, Report{}, err
	}
}

// RenderNote turns a report into Markdown.
//
// Notes and candidates are linked by their canonical URIs, so the note is
// navigable. A link the report is about is quoted as literal text and never
// written as a link, so the note cannot itself hold an ambiguous link for the
// next report, or the preview, to find.
func RenderNote(report Report, generatedAt time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", NoteTitle)
	scope := "every collection"
	if report.CollectionID != "" {
		scope = "collection `" + report.CollectionID + "`"
	}
	fmt.Fprintf(&b, "Generated %s from %s in %.1f ms.\n\n", generatedAt.Format("2006-01-02 15:04 MST"), scope, report.ElapsedMS)
	b.WriteString("This note is regenerated on request and **overwritten in place**. It is\n")
	b.WriteString("read-only, so edits to it would not survive the next run. Notes in the Trash\n")
	b.WriteString("and in read-only notebooks are not measured. Each link is quoted as written;\n")
	b.WriteString("choosing between the notes it might mean is an ordinary edit of the note it\n")
	b.WriteString("is in. Nothing here is guessed or repaired.\n\n")

	b.WriteString("| Measure | Count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Notes with an ambiguous link | %d |\n", report.NotesWithAmbiguousLinks)
	fmt.Fprintf(&b, "| Ambiguous links | %d |\n", report.AmbiguousLinks)
	fmt.Fprintf(&b, "| … that several notes or attachments in the library match | %d |\n", report.FoundByLibraryCount)
	fmt.Fprintf(&b, "| … that the Obsidian import found several notes for | %d |\n", report.FoundByImportCount)
	fmt.Fprintf(&b, "| Other links that open nothing | %d |\n\n", report.UnresolvedLinks)
	for _, warning := range report.Warnings {
		fmt.Fprintf(&b, "> %s\n\n", literal(warning))
	}

	b.WriteString("## Notes\n\n")
	if len(report.Notes) == 0 {
		b.WriteString("No link in this library is ambiguous.\n\n")
	}
	for _, note := range report.Notes {
		fmt.Fprintf(&b, "### %s\n\n", titled(note.Title, note.DocumentID, note.URI))
		for _, link := range note.Links {
			fmt.Fprintf(&b, "- Line %d, column %d: %s — %s\n", link.Line, link.Column, literal(link.Written), why(link))
			for _, candidate := range link.Candidates {
				fmt.Fprintf(&b, "  - %s\n", titled(candidate.Title, candidate.ID, candidate.URI))
			}
			if hidden := link.CandidateCount - len(link.Candidates); hidden > 0 {
				fmt.Fprintf(&b, "  - and %d more\n", hidden)
			}
		}
		if note.Truncated {
			fmt.Fprintf(&b, "\nShowing %d of this note's %d ambiguous links.\n", len(note.Links), note.LinkCount)
		}
		b.WriteString("\n")
	}
	if report.Truncated {
		fmt.Fprintf(&b, "Showing the first %d of %d notes; the counts above are complete.\n\n", len(report.Notes), report.NotesWithAmbiguousLinks)
	}

	b.WriteString("## Links that open nothing\n\n")
	switch report.UnresolvedLinks {
	case 0:
		b.WriteString("Every other link opens something.\n")
	case 1:
		b.WriteString("1 other link opens nothing. `notriosctl lint` locates it, as a\n`broken_document_link` or `broken_resource_link`.\n")
	default:
		fmt.Fprintf(&b, "%d other links open nothing. `notriosctl lint` locates each one, as a\n`broken_document_link` or `broken_resource_link`.\n", report.UnresolvedLinks)
	}
	return b.String()
}

func why(link Link) string {
	switch link.Kind {
	case KindAttachment:
		return fmt.Sprintf("%d attachments have this file name:", link.CandidateCount)
	case KindName:
		return fmt.Sprintf("the Obsidian import found %d notes by this name, title or alias:", link.CandidateCount)
	default:
		return fmt.Sprintf("%d notes have this title:", link.CandidateCount)
	}
}

// literal writes text so that Markdown shows it exactly and nothing reads it
// as syntax: every ASCII punctuation character is backslash-escaped, which
// CommonMark allows for each of them, and a line break becomes a space. A
// quoted `[[index]]` then has no `[[` and no `](` for Notrios' link extractor,
// or the preview's, to find.
func literal(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == '\n' || r == '\r':
			b.WriteByte(' ')
		case r < 0x80 && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", r):
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// titled links a title to a URI without letting the title break the link or
// become syntax. Notrios' extractor ends a link's text at the first `]`,
// escaped or not, so a title with a bracket is written as literal text beside a
// short link, as the graph report does.
func titled(title, id, uri string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = id
	}
	if strings.ContainsAny(title, "[]") {
		return literal(title) + " ([open](" + uri + "))"
	}
	return "[" + literal(title) + "](" + uri + ")"
}
