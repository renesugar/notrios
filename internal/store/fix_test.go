package store

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func newFixFixture(t *testing.T) (*SQLiteStore, Document, Resource) {
	t.Helper()
	ctx := context.Background()
	st, err := OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	// A single-word title on purpose: in Markdown a space ends an unquoted URL,
	// so `[x](Kitchen Plan)` never names a note at all — it truncates at the
	// space. Title-resolved Markdown links are the space-free ones.
	target, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_target", Title: "Kitchen", Body: "# Kitchen\n\nBody.\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	resource, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_photo", Filename: "kitchen-plan.final.png", MIMEType: "image/png",
		Content: bytes.NewReader([]byte("\x89PNG\r\n\x1a\nphoto")),
	})
	if err != nil {
		t.Fatal(err)
	}
	return st, target, resource
}

func TestPlanFixRewritesTitleLinksToCanonicalURIs(t *testing.T) {
	ctx := context.Background()
	st, target, _ := newFixFixture(t)
	source, err := st.CreateDocument(ctx, CreateDocumentRequest{
		PreferredID: "doc_source",
		Body:        "See [the plan](Kitchen) and [already canonical](" + target.URI + ").\n",
		Title:       "Source",
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := st.PlanWorkspaceFix(ctx, FixRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalDocuments != 1 || plan.TotalEdits != 1 {
		t.Fatalf("expected one edit in one note: %+v", plan)
	}
	edit := plan.Documents[0].Edits[0]
	if edit.Kind != FixNonCanonicalLinkTarget {
		t.Fatalf("unexpected kind: %+v", edit)
	}
	if edit.Before != "[the plan](Kitchen)" || edit.After != "[the plan]("+target.URI+")" {
		t.Fatalf("edit shows the wrong replacement: %+v", edit)
	}
	if plan.Documents[0].BaseRevisionID != source.CurrentRevisionID {
		t.Fatalf("plan must bind the revision it was computed from: %+v", plan.Documents[0])
	}

	// Planning changed nothing.
	unchanged, err := st.GetDocument(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Body != source.Body || unchanged.CurrentRevisionID != source.CurrentRevisionID {
		t.Fatal("planning a fix must not touch the note")
	}

	result, err := st.ApplyDocumentFix(ctx, plan.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 1 || result.Skipped != 0 || result.RevisionID == "" {
		t.Fatalf("apply: %+v", result)
	}
	fixed, err := st.GetDocument(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fixed.Body, "[the plan]("+target.URI+")") {
		t.Fatalf("body not rewritten: %q", fixed.Body)
	}
	// Every fix is an ordinary revision, so it is visible and revertible.
	revisions, err := st.ListDocumentRevisions(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected a new revision, got %d", len(revisions))
	}
	// The link now resolves to the same note it always did.
	links, err := st.ListDocumentLinks(ctx, source.ID, "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range links.Outgoing {
		if link.ResolutionStatus != "resolved" || link.TargetDocumentID != target.ID {
			t.Fatalf("fix changed what a link points at: %+v", link)
		}
	}
}

// The precondition is the safety story: a note edited since the plan was made
// must fail rather than be cut at stale offsets.
func TestApplyFixRefusesWhenTheNoteChanged(t *testing.T) {
	ctx := context.Background()
	st, _, _ := newFixFixture(t)
	source, err := st.CreateDocument(ctx, CreateDocumentRequest{
		Title: "Source", Body: "See [the plan](Kitchen).\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := st.PlanWorkspaceFix(ctx, FixRequest{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: source.ID, Title: "Source", Body: "Rewritten by someone else.\n",
		BaseRevisionID: source.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ApplyDocumentFix(ctx, plan.Documents[0]); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected a conflict, got %v", err)
	}
	current, err := st.GetDocument(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Body != "Rewritten by someone else.\n" {
		t.Fatalf("the concurrent edit was overwritten: %q", current.Body)
	}

	// A plan with no precondition is refused outright.
	orphan := plan.Documents[0]
	orphan.BaseRevisionID = ""
	if _, err := st.ApplyDocumentFix(ctx, orphan); !errors.Is(err, ErrPreconditionRequired) {
		t.Fatalf("expected ErrPreconditionRequired, got %v", err)
	}
}

// Alt text is off unless asked for: a filename is a starting point for a
// description, not a description.
func TestAltTextFixIsOptIn(t *testing.T) {
	ctx := context.Background()
	st, _, resource := newFixFixture(t)
	source, err := st.CreateDocument(ctx, CreateDocumentRequest{
		Title: "Gallery", Body: "![](" + resource.URI + ")\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AttachDocumentResource(ctx, AttachResourceRequest{
		DocumentID: source.ID, ResourceID: resource.ID, RelationType: "embedded",
	}); err != nil {
		t.Fatal(err)
	}

	byDefault, err := st.PlanWorkspaceFix(ctx, FixRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if byDefault.TotalEdits != 0 {
		t.Fatalf("alt text must not be fixed by default: %+v", byDefault)
	}

	asked, err := st.PlanWorkspaceFix(ctx, FixRequest{Kinds: []string{FixMissingAltText}})
	if err != nil {
		t.Fatal(err)
	}
	if asked.TotalEdits != 1 {
		t.Fatalf("expected one alt-text edit: %+v", asked)
	}
	edit := asked.Documents[0].Edits[0]
	if edit.After != "![kitchen plan final]("+resource.URI+")" {
		t.Fatalf("alt text should come from the filename the author chose: %q", edit.After)
	}
}

func TestFixLeavesWikilinksAndUnresolvedLinksAlone(t *testing.T) {
	ctx := context.Background()
	st, _, _ := newFixFixture(t)
	if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
		Title: "Mixed",
		Body: "A wikilink [[Kitchen]] keeps its syntax.\n\n" +
			"[broken](Nowhere At All) stays reported, not guessed at.\n",
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := st.PlanWorkspaceFix(ctx, FixRequest{Kinds: FixKinds()})
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalEdits != 0 {
		t.Fatalf("expected nothing to fix: %+v", plan)
	}
}

// Help notes are read-only and trashed notes are not part of the workspace.
func TestFixSkipsReadOnlyAndTrashedNotes(t *testing.T) {
	ctx := context.Background()
	st, _, _ := newFixFixture(t)
	help, err := st.CreateDocument(ctx, CreateDocumentRequest{
		NotebookID: HelpNotebookID, Title: "Help note", Body: "[the plan](Kitchen)\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	trashed, err := st.CreateDocument(ctx, CreateDocumentRequest{
		Title: "Trashed", Body: "[the plan](Kitchen)\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: trashed.ID, BaseRevisionID: trashed.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	plan, err := st.PlanWorkspaceFix(ctx, FixRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range plan.Documents {
		if document.DocumentID == help.ID || document.DocumentID == trashed.ID {
			t.Fatalf("fix planned an edit for %s", document.DocumentID)
		}
	}
}

// A span whose bytes are not what the plan recorded is skipped, never applied
// at an arbitrary position.
func TestApplyFixEditsRefusesStaleSpans(t *testing.T) {
	body := "before [the plan](Kitchen) after"
	start := strings.Index(body, "[the plan]")
	end := start + len("[the plan](Kitchen)")
	good := FixEdit{StartByte: start, EndByte: end, Before: body[start:end], After: "[the plan](document://x)"}

	rewritten, applied, skipped := applyFixEdits(body, []FixEdit{good})
	if applied != 1 || skipped != 0 || !strings.Contains(rewritten, "document://x") {
		t.Fatalf("applied=%d skipped=%d body=%q", applied, skipped, rewritten)
	}

	cases := map[string]FixEdit{
		"text moved":   {StartByte: start, EndByte: end, Before: "[the plan](Somewhere)", After: "x"},
		"past the end": {StartByte: len(body) - 2, EndByte: len(body) + 20, Before: "xx", After: "y"},
		"inverted":     {StartByte: 10, EndByte: 5, Before: "", After: "y"},
		"negative":     {StartByte: -1, EndByte: 4, Before: "", After: "y"},
	}
	for name, edit := range cases {
		got, applied, skipped := applyFixEdits(body, []FixEdit{edit})
		if applied != 0 || skipped != 1 || got != body {
			t.Fatalf("%s: applied=%d skipped=%d body=%q", name, applied, skipped, got)
		}
	}
}

func TestFixValidatesRequests(t *testing.T) {
	ctx := context.Background()
	st, _, _ := newFixFixture(t)
	if _, err := st.PlanWorkspaceFix(ctx, FixRequest{Kinds: []string{"rewrite_everything"}}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown kind: %v", err)
	}
	if _, err := st.PlanWorkspaceFix(ctx, FixRequest{MaxDocuments: MaxFixDocuments + 1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized bound: %v", err)
	}
}
