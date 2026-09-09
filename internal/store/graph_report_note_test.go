package store

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"testing"
)

// The property the whole design rests on: writing the report must not change
// what the report says.
//
// The note links to every hub it ranks, so without the source filter each of
// those hubs would gain an incoming link and the next generation would report
// different numbers — an observer effect built in by construction. Excluding
// the note from its own ranking would not have been enough, because the links
// would still count.
func TestWritingTheReportDoesNotChangeTheReport(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	linkedPair(t, st, "rn_source", "", "rn_hub")

	before, err := st.GraphReport(ctx, GraphReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if before.DocumentCount != 2 || inDegreeOf(t, before, "rn_hub") != 1 {
		t.Fatalf("fixture: expected two notes and one link: %+v", before)
	}

	doc, _, err := st.WriteGraphReportNote(ctx, GraphReportRequest{})
	if err != nil {
		t.Fatalf("WriteGraphReportNote: %v", err)
	}
	// The note really does link to the hub — otherwise the assertion below
	// would be measuring nothing.
	if !strings.Contains(doc.Body, DocumentURI("default", "rn_hub")) {
		t.Fatalf("the report should link to the hub it ranks:\n%s", doc.Body)
	}
	if err := st.RebuildDocumentLinks(ctx, doc.ID); err != nil {
		t.Fatalf("RebuildDocumentLinks: %v", err)
	}

	after, err := st.GraphReport(ctx, GraphReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if after.DocumentCount != before.DocumentCount || after.LinkCount != before.LinkCount ||
		after.OrphanCount != before.OrphanCount || after.IsolatedCount != before.IsolatedCount {
		t.Fatalf("generating the report changed what it measures:\nbefore %+v\nafter  %+v", before, after)
	}
	if got := inDegreeOf(t, after, "rn_hub"); got != 1 {
		t.Fatalf("the report's own link inflated the hub's in-degree to %d", got)
	}
}

// A stable ID, overwritten in place: regenerating must not litter the library
// with dated copies, and a link to the report must keep working.
func TestTheReportNoteIsOverwrittenInPlace(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)

	first, _, err := st.WriteGraphReportNote(ctx, GraphReportRequest{})
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if first.ID != GraphReportNoteID || first.NotebookID != ReportsNotebookID {
		t.Fatalf("the report has a stable home: %+v", first)
	}

	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Later note", Body: "x\n"}); err != nil {
		t.Fatal(err)
	}
	second, report, err := st.WriteGraphReportNote(ctx, GraphReportRequest{})
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("regeneration must reuse the note, got %q then %q", first.ID, second.ID)
	}
	if second.CurrentRevisionID == first.CurrentRevisionID {
		t.Fatal("the second write should have produced a new revision")
	}
	if report.DocumentCount != 1 {
		t.Fatalf("the second report should see the new note: %+v", report)
	}

	page, err := st.ListNotebookDocuments(ctx, ReportsNotebookID, DocumentPageRequest{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Documents) != 1 {
		t.Fatalf("Reports should hold exactly one report note, got %d", len(page.Documents))
	}
}

// An empty library still gets a report. A report that appears only once there
// is something to say is one nobody discovers.
func TestTheReportNoteIsWrittenForAnEmptyLibrary(t *testing.T) {
	st := newNotebookTestStore(t)
	doc, report, err := st.WriteGraphReportNote(context.Background(), GraphReportRequest{})
	if err != nil {
		t.Fatalf("WriteGraphReportNote: %v", err)
	}
	if report.DocumentCount != 0 {
		t.Fatalf("nothing to measure: %+v", report)
	}
	for _, want := range []string{"Nothing in this library is linked to yet.", "| Notes measured | 0 |"} {
		if !strings.Contains(doc.Body, want) {
			t.Fatalf("an empty report should still say so; missing %q:\n%s", want, doc.Body)
		}
	}
}

// A title is arbitrary user text, and Notrios' own link parser forbids `]`
// inside link text — even backslash-escaped, since the pattern is
// `\[([^\]\n]*)\]\(`. Escaping the bracket, which is what this test was
// written to check first, produced a link the store could not resolve: a broken
// link in a generated note, in the user's lint report, blamed on a note they
// cannot edit.
//
// So the title is kept verbatim beside a short link instead of being altered to
// fit. Both branches are asserted, because a fallback that never fires and a
// fallback that always fires are both wrong.
func TestReportTitlesCannotBreakTheLinksTheyLabel(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "rn_bracket", Title: "Reading [2026] notes", Body: "target\n",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "rn_linker", Title: "Linker",
		Body: "[to](" + DocumentURI("default", "rn_bracket") + ")\n",
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"rn_bracket", "rn_linker"} {
		if err := st.RebuildDocumentLinks(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	doc, _, err := st.WriteGraphReportNote(ctx, GraphReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Body, "Reading [2026] notes ([open](") {
		t.Fatalf("a title carrying `]` stays verbatim beside a short link:\n%s", doc.Body)
	}
	// The ordinary case is untouched: still title-as-link-text.
	if !strings.Contains(doc.Body, "[Linker](document://default/documents/rn_linker)") {
		t.Fatalf("a title with no bracket should still label its own link:\n%s", doc.Body)
	}
	// And the link resolves, which is the whole reason the form changed.
	if err := st.RebuildDocumentLinks(ctx, doc.ID); err != nil {
		t.Fatal(err)
	}
	page, err := st.ListDocumentLinks(ctx, doc.ID, GraphDirectionOutgoing)
	if err != nil {
		t.Fatal(err)
	}
	resolved := false
	for _, link := range page.Outgoing {
		if link.TargetDocumentID == "rn_bracket" && link.ResolutionStatus == "resolved" {
			resolved = true
		}
	}
	if !resolved {
		t.Fatalf("the report's link to a bracketed title should resolve: %+v", page.Outgoing)
	}
}

// The export is the same graph the report measures. Two tables that disagreed
// about what the library contains would be worse than either alone.
func TestGraphCSVExportStreamsTheMeasuredGraph(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	linkedPair(t, st, "ex_source", "", "ex_target")
	// A title with a comma and a quote, to prove the CSV writer is doing the
	// quoting rather than string concatenation pretending to.
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "ex_comma", Title: `Notes, "quoted"`, Body: "x\n",
	}); err != nil {
		t.Fatal(err)
	}
	// A Help note and a trashed note: neither is part of the exported graph.
	linkedPair(t, st, "ex_help", HelpNotebookID, "ex_help_target")
	trashed, err := st.CreateDocument(ctx, CreateDocumentRequest{PreferredID: "ex_trashed", Title: "Trashed", Body: "x\n"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: trashed.ID, BaseRevisionID: trashed.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}

	var nodesBuf, edgesBuf bytes.Buffer
	summary, err := st.ExportGraphCSV(ctx, ExportGraphRequest{}, &nodesBuf, &edgesBuf)
	if err != nil {
		t.Fatalf("ExportGraphCSV: %v", err)
	}

	nodes, err := csv.NewReader(bytes.NewReader(nodesBuf.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("nodes.csv is not valid CSV: %v", err)
	}
	if nodes[0][0] != "Id" || nodes[0][1] != "Label" {
		t.Fatalf("Gephi looks for Id and Label: %v", nodes[0])
	}
	ids := map[string]string{}
	for _, row := range nodes[1:] {
		ids[row[0]] = row[1]
	}
	if int64(len(ids)) != summary.Nodes {
		t.Fatalf("summary says %d nodes, file has %d", summary.Nodes, len(ids))
	}
	// The Help note is not exported; the ordinary note it happens to link to
	// is, because it is the user's — with an in-degree of zero, since the only
	// link into it comes from Help.
	if _, found := ids["ex_help_target"]; !found {
		t.Fatalf("a note linked from Help is still the user's note: %v", ids)
	}
	for _, absent := range []string{"ex_trashed", "ex_help"} {
		if _, found := ids[absent]; found {
			t.Fatalf("%q is not part of the measured graph: %v", absent, ids)
		}
	}
	if ids["ex_comma"] != `Notes, "quoted"` {
		t.Fatalf("the CSV writer should round-trip a comma and a quote, got %q", ids["ex_comma"])
	}

	edges, err := csv.NewReader(bytes.NewReader(edgesBuf.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("edges.csv is not valid CSV: %v", err)
	}
	if edges[0][0] != "Source" || edges[0][1] != "Target" || edges[0][2] != "Type" {
		t.Fatalf("Gephi looks for Source, Target and Type: %v", edges[0])
	}
	if len(edges)-1 != 1 || edges[1][0] != "ex_source" || edges[1][1] != "ex_target" {
		t.Fatalf("expected exactly the one measured edge: %v", edges[1:])
	}
	if edges[1][2] != "Directed" {
		t.Fatalf("links point one way: %v", edges[1])
	}
	if summary.Edges != 1 {
		t.Fatalf("summary says %d edges, want 1", summary.Edges)
	}
}
