package store

import (
	"context"
	"testing"
)

// reportFor is the whole-collection report, unlimited enough for these fixtures.
func reportFor(t *testing.T, st *SQLiteStore) GraphReport {
	t.Helper()
	report, err := st.GraphReport(context.Background(), GraphReportRequest{})
	if err != nil {
		t.Fatalf("GraphReport: %v", err)
	}
	return report
}

func inDegreeOf(t *testing.T, report GraphReport, documentID string) int64 {
	t.Helper()
	for _, list := range [][]GraphReportEntry{report.Hubs, report.Orphans, report.Isolated} {
		for _, entry := range list {
			if entry.DocumentID == documentID {
				return entry.InDegree
			}
		}
	}
	t.Fatalf("note %q appears in no example list: %+v", documentID, report)
	return -1
}

func reportContains(report GraphReport, documentID string) bool {
	for _, list := range [][]GraphReportEntry{report.Hubs, report.Orphans, report.Isolated} {
		for _, entry := range list {
			if entry.DocumentID == documentID {
				return true
			}
		}
	}
	return false
}

func isOrphan(report GraphReport, documentID string) bool {
	for _, entry := range report.Orphans {
		if entry.DocumentID == documentID {
			return true
		}
	}
	return false
}

// linkedPair creates `source -> target` with links resolved.
func linkedPair(t *testing.T, st *SQLiteStore, sourceID, sourceNotebook, targetID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: targetID, Title: targetID, Body: "the target\n",
	}); err != nil {
		t.Fatalf("create %s: %v", targetID, err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: sourceID, Title: sourceID, NotebookID: sourceNotebook,
		Body: "[to](" + DocumentURI("default", targetID) + ")\n",
	}); err != nil {
		t.Fatalf("create %s: %v", sourceID, err)
	}
	for _, id := range []string{targetID, sourceID} {
		if err := st.RebuildDocumentLinks(ctx, id); err != nil {
			t.Fatalf("rebuild %s: %v", id, err)
		}
	}
}

// The pre-existing defect F5 fixes.
//
// Soft delete writes a revision, clears the FTS row, and enqueues a projection,
// but deliberately **leaves `document_links` intact** so a restore can use them.
// The report's row set already excluded trashed notes — so one was never ranked
// — but its in-degree subquery placed no condition on the link's *source*. A
// note sitting in the Trash therefore kept propping up everything it had linked
// to, and a note linked only from the Trash was never reported as an orphan.
func TestTrashedNotesDoNotProlongAnInDegree(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	linkedPair(t, st, "gr_source", "", "gr_target")

	// Asserted before as well as after, so the test cannot pass because the
	// fixture never produced a link in the first place.
	before := reportFor(t, st)
	if got := inDegreeOf(t, before, "gr_target"); got != 1 {
		t.Fatalf("a live note linking in gives in-degree 1, got %d", got)
	}
	if isOrphan(before, "gr_target") {
		t.Fatal("something links to it, so it is not an orphan yet")
	}

	source, err := st.GetDocument(ctx, "gr_source")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: source.ID, BaseRevisionID: source.CurrentRevisionID}); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	after := reportFor(t, st)
	if got := inDegreeOf(t, after, "gr_target"); got != 0 {
		t.Fatalf("the only note linking in is in the Trash, so in-degree should be 0, got %d", got)
	}
	if !isOrphan(after, "gr_target") {
		t.Fatalf("a note linked only from the Trash is an orphan: %+v", after.Orphans)
	}
	// The link row still exists — this is a reporting filter, not a deletion.
	page, err := st.ListDocumentLinks(ctx, source.ID, GraphDirectionOutgoing)
	if err != nil || len(page.Outgoing) == 0 {
		t.Fatalf("soft delete must keep links so a restore can use them: %d links, err %v", len(page.Outgoing), err)
	}
}

// The observer effect the hubs report would otherwise build in by construction:
// a report note linking to the top N hubs adds an incoming link to each of
// them, changing the ranking the next generation sees. Excluding the report
// from its own ranking does not fix that, because the links still count.
//
// The same filter covers Help, whose notes link to each other heavily.
func TestSystemAuthoredNotesAreNeitherMeasuredNorCounted(t *testing.T) {
	st := newNotebookTestStore(t)

	for _, notebook := range []string{ReportsNotebookID, HelpNotebookID} {
		t.Run(notebook, func(t *testing.T) {
			target := "gr_" + notebook + "_target"
			source := "gr_" + notebook + "_source"
			linkedPair(t, st, source, notebook, target)

			report := reportFor(t, st)
			if got := inDegreeOf(t, report, target); got != 0 {
				t.Fatalf("a link from %s must not count toward in-degree, got %d", notebook, got)
			}
			if !isOrphan(report, target) {
				t.Fatalf("linked only from %s, the note is still an orphan: %+v", notebook, report.Orphans)
			}
			// And the system-authored note is not itself measured — otherwise
			// every Help note would arrive as a fresh orphan the moment the
			// link filter above started working.
			if reportContains(report, source) {
				t.Fatalf("%s is system-authored and is not part of the library being measured", source)
			}
		})
	}
}

// LinkCount has always claimed to count "edges a traversal can actually
// follow". Filtering only the source would have left it counting edges into the
// Trash and into Help, so both ends are filtered.
func TestLinkCountOnlyCountsEdgesBetweenMeasuredNotes(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)

	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "gr_help_page", Title: "Help page", NotebookID: HelpNotebookID, Body: "docs\n",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "gr_reader", Title: "Reader",
		Body: "[the docs](" + DocumentURI("default", "gr_help_page") + ")\n",
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"gr_help_page", "gr_reader"} {
		if err := st.RebuildDocumentLinks(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	report := reportFor(t, st)
	if report.LinkCount != 0 {
		t.Fatalf("an edge into Help leaves the measured library, so it is not counted: %+v", report)
	}
	if !isOrphan(report, "gr_reader") {
		t.Fatalf("the reader links only into Help, so nothing measured links to it: %+v", report.Orphans)
	}
	isolated := false
	for _, entry := range report.Isolated {
		if entry.DocumentID == "gr_reader" {
			isolated = true
		}
	}
	if !isolated {
		t.Fatalf("with no measured edge in either direction the reader is isolated: %+v", report.Isolated)
	}
}

// Traversal follows the same predicate the report does, with one exemption
// that keeps a visible feature working.
//
// The report links to every hub it ranks. Without a filter, every hub's local
// graph would show the report as a depth-1 neighbour — noise in exactly the
// view F5 argues stays readable at scale. But a Help page's *own* local graph
// must still show what it links to, or opening any system-authored note gives
// an empty picture. So a read-only builtin note's links are followed when it is
// the note being asked about and ignored when it is not.
func TestTraversalIgnoresSystemAuthoredNeighboursExceptFromTheRoot(t *testing.T) {
	ctx := context.Background()
	st := newNotebookTestStore(t)
	linkedPair(t, st, "gt_report", ReportsNotebookID, "gt_hub")

	fromHub, err := st.Graph(ctx, GraphRequest{Roots: []string{"gt_hub"}, Depth: 1})
	if err != nil {
		t.Fatalf("Graph: %v", err)
	}
	if graphContains(fromHub, "gt_report") {
		t.Fatalf("the report is not a neighbour of every note it names: %v", graphNodeIDs(fromHub))
	}

	fromReport, err := st.Graph(ctx, GraphRequest{Roots: []string{"gt_report"}, Depth: 1})
	if err != nil {
		t.Fatalf("Graph: %v", err)
	}
	if !graphContains(fromReport, "gt_hub") {
		t.Fatalf("asked about the report itself, its links are the answer: %v", graphNodeIDs(fromReport))
	}
	if len(fromReport.Edges) != 1 {
		t.Fatalf("the root's own edge should be present exactly once: %+v", fromReport.Edges)
	}
}
