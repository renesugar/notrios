package archivev2

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// publicationOptions selects the public notebook, which is the boundary the
// fixture's links cross: the public note links to an included child note, an
// excluded private note, a target that never resolved, an external URL, and an
// embedded resource.
func publicationOptions(fixture *exportFixture, action string) ExportOptions {
	options := exportOptions(TargetPublicationHandoff)
	options.Selection = store.SelectionSpec{NotebookIDs: []string{fixture.public.ID}}
	options.Policy = store.PrivacyPolicy{LinkAction: action}
	return options
}

// bodyFor returns the published text of one document's revision.
func bodyFor(t *testing.T, root string, records map[string][]json.RawMessage, documentID string) string {
	t.Helper()
	for _, raw := range records[RecordRevision] {
		var revision RevisionRecord
		decode(t, raw, &revision)
		if revision.DocumentID != documentID {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, "objects", "sha256",
			revision.Body.SHA256[0:2], revision.Body.SHA256[2:4], revision.Body.SHA256))
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	t.Fatalf("no revision for %s", documentID)
	return ""
}

func TestPublicationHandoffRewritesWithheldLinksToPlainText(t *testing.T) {
	fixture := newExportFixture(t)
	root := filepath.Join(t.TempDir(), "publication")
	report, err := Export(context.Background(), fixture.store, root, publicationOptions(fixture, store.SelectionLinkActionPlainText))
	if err != nil {
		t.Fatal(err)
	}
	if !report.Verified {
		t.Fatalf("a publication handoff must still verify as an archive: %+v", report)
	}
	if report.FullBackup {
		t.Fatalf("a publication projection must never be reported as a backup: %+v", report)
	}

	records := decodeRecords(t, root)
	body := bodyFor(t, root, records, fixture.publicDoc.ID)

	// The link to the note that stayed behind must not survive in any form:
	// neither as a followable link nor as a mention of the withheld note's URI.
	if strings.Contains(body, fixture.privateDoc.URI) {
		t.Fatalf("published body still names the withheld note: %q", body)
	}
	if strings.Contains(body, "["+"private"+"](") {
		t.Fatalf("published body still carries link syntax for the withheld note: %q", body)
	}
	if !strings.Contains(body, "private") {
		t.Fatalf("plain_text should keep the display text as prose: %q", body)
	}
	// A link that never resolved is equally unusable to a reader.
	if strings.Contains(body, "](Missing Note)") {
		t.Fatalf("published body kept a broken link: %q", body)
	}
	// Links inside the published set and external links are untouched.
	if !strings.Contains(body, "["+"child"+"]("+fixture.childDoc.URI+")") {
		t.Fatalf("an included internal link must survive: %q", body)
	}
	if !strings.Contains(body, "[away](https://example.com)") {
		t.Fatalf("an external link must survive: %q", body)
	}
	if !strings.Contains(body, fixture.resource.URI) {
		t.Fatalf("a reachable resource embed must survive: %q", body)
	}
	if report.RewrittenLinks != 2 || report.RewrittenDocuments != 1 || report.SkippedRewrites != 0 {
		t.Fatalf("unexpected rewrite counters: %+v", report)
	}
}

// Rewriting the body is not enough on its own: a link record carries the raw
// target, the resolved ID, and a context excerpt of the surrounding text, so a
// published record for a withheld link would hand over exactly what rewriting
// the body just removed.
func TestPublicationHandoffDropsRecordsForWithheldLinks(t *testing.T) {
	fixture := newExportFixture(t)
	root := filepath.Join(t.TempDir(), "publication")
	if _, err := Export(context.Background(), fixture.store, root, publicationOptions(fixture, store.SelectionLinkActionPlainText)); err != nil {
		t.Fatal(err)
	}
	records := decodeRecords(t, root)
	published := 0
	for _, raw := range records[RecordLink] {
		var link LinkRecord
		decode(t, raw, &link)
		published++
		if strings.Contains(link.RawTarget, fixture.privateDoc.ID) || link.TargetDocumentID == fixture.privateDoc.ID {
			t.Fatalf("a published link record names the withheld note: %+v", link)
		}
		if link.Context != "" {
			t.Fatalf("a published link record carries a copy of note text: %+v", link)
		}
	}
	if published == 0 {
		t.Fatal("links inside the published set should still be recorded")
	}
}

// A retained link's offsets must describe the published bytes, not the
// canonical ones: rewriting an earlier span moves everything after it.
func TestPublicationHandoffShiftsRetainedLinkOffsets(t *testing.T) {
	fixture := newExportFixture(t)
	root := filepath.Join(t.TempDir(), "publication")
	if _, err := Export(context.Background(), fixture.store, root, publicationOptions(fixture, store.SelectionLinkActionRedact)); err != nil {
		t.Fatal(err)
	}
	records := decodeRecords(t, root)
	body := bodyFor(t, root, records, fixture.publicDoc.ID)
	checked := 0
	for _, raw := range records[RecordLink] {
		var link LinkRecord
		decode(t, raw, &link)
		if link.SourceDocumentID != fixture.publicDoc.ID || link.RawTarget == "" {
			continue
		}
		if link.SourceEndByte > len(body) {
			t.Fatalf("link span runs past the published body: %+v (body %d bytes)", link, len(body))
		}
		span := body[link.SourceStartByte:link.SourceEndByte]
		if !strings.Contains(span, link.RawTarget) {
			t.Fatalf("published span %q does not contain the link target %q", span, link.RawTarget)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("expected at least one retained link to check")
	}
}

func TestShiftOffsetMovesOnlyBytesAfterARewrite(t *testing.T) {
	deltas := []offsetDelta{{after: 10, delta: -4}, {after: 30, delta: +6}}
	cases := map[int]int{0: 0, 9: 9, 10: 6, 29: 25, 30: 32, 100: 102}
	for offset, want := range cases {
		if got := shiftOffset(offset, deltas); got != want {
			t.Fatalf("offset %d: got %d, want %d", offset, got, want)
		}
	}
	if got := shiftOffset(2, []offsetDelta{{after: 0, delta: -50}}); got != 0 {
		t.Fatalf("a shift may not go negative: %d", got)
	}
}

func TestPublicationHandoffRedactsWhenAsked(t *testing.T) {
	fixture := newExportFixture(t)
	root := filepath.Join(t.TempDir(), "publication")
	if _, err := Export(context.Background(), fixture.store, root, publicationOptions(fixture, store.SelectionLinkActionRedact)); err != nil {
		t.Fatal(err)
	}
	body := bodyFor(t, root, decodeRecords(t, root), fixture.publicDoc.ID)
	if strings.Count(body, redactedPlaceholder) != 2 {
		t.Fatalf("expected both withheld links redacted: %q", body)
	}
	// Redaction removes the display text too — that is the difference from
	// plain_text, and the reason a profile chooses it.
	if strings.Contains(body, "[private]") || strings.Contains(body, "Missing Note") {
		t.Fatalf("redaction left the withheld reference behind: %q", body)
	}
}

func TestPublicationHandoffWithholdsHistoryProvenanceAndBundles(t *testing.T) {
	fixture := newExportFixture(t)
	root := filepath.Join(t.TempDir(), "publication")
	report, err := Export(context.Background(), fixture.store, root, publicationOptions(fixture, store.SelectionLinkActionPlainText))
	if err != nil {
		t.Fatal(err)
	}
	records := decodeRecords(t, root)

	// The public note has two revisions in the library; only the current one
	// may be published, because an earlier revision can contain exactly the
	// text that was later removed.
	published := 0
	for _, raw := range records[RecordRevision] {
		var revision RevisionRecord
		decode(t, raw, &revision)
		if revision.DocumentID == fixture.publicDoc.ID {
			published++
			if revision.ID != fixture.publicDoc.CurrentRevisionID {
				// The fixture updated the note, so the current revision is the
				// second one; either way only one may appear.
				continue
			}
		}
	}
	if published != 1 {
		t.Fatalf("expected exactly the current revision, got %d", published)
	}
	if len(records[RecordProvenance]) != 0 {
		t.Fatalf("a publication must not carry provenance: %d records", len(records[RecordProvenance]))
	}
	if len(records[RecordSourceBundle]) != 0 {
		t.Fatalf("a publication must not carry exact source bundles: %d records", len(records[RecordSourceBundle]))
	}
	if len(records[RecordSearchNotebook]) != 0 {
		t.Fatalf("saved searches describe the whole library and must not be published")
	}
	for _, raw := range records[RecordDocument] {
		var document DocumentRecord
		decode(t, raw, &document)
		if document.ID == fixture.privateDoc.ID || document.ID == fixture.trashedDoc.ID {
			t.Fatalf("withheld note %s was published", document.ID)
		}
		if document.DeletedAt != "" {
			t.Fatalf("a trashed note was published: %s", document.ID)
		}
	}
	if !containsSubstring(report.Warnings, "publication projection") {
		t.Fatalf("the report must say what this artifact is: %+v", report.Warnings)
	}
	if !containsSubstring(report.Warnings, "not a complete database backup") {
		t.Fatalf("the report must say what this artifact is not: %+v", report.Warnings)
	}
}

// No current writer populates revision metadata, which is precisely why the
// projection must not depend on that staying true: the day an importer starts
// recording source paths there, a publication written by an older assumption
// would carry them.
func TestPublicationHandoffStripsRevisionMetadata(t *testing.T) {
	ctx := context.Background()
	fixture := newExportFixture(t)
	if err := fixture.store.Exec(ctx,
		`UPDATE document_revisions SET metadata_json = '{"import_source_path":"/home/user/private/vault/child.md"}' WHERE document_id = '`+fixture.childDoc.ID+`'`); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "publication")
	report, err := Export(ctx, fixture.store, root, publicationOptions(fixture, store.SelectionLinkActionPlainText))
	if err != nil {
		t.Fatal(err)
	}
	if report.StrippedMetadata == 0 {
		t.Fatalf("expected revision metadata to be stripped: %+v", report)
	}
	for _, raw := range decodeRecords(t, root)[RecordRevision] {
		var revision RevisionRecord
		decode(t, raw, &revision)
		if strings.Contains(string(revision.MetadataJSON), "private") {
			t.Fatalf("published revision metadata leaked a local path: %s", revision.MetadataJSON)
		}
	}
}

// A publication must be usable by the same verifier as any other archive: it
// is a scoped archive, not a second format.
func TestPublicationHandoffIsAnOrdinaryArchiveToTheVerifier(t *testing.T) {
	fixture := newExportFixture(t)
	root := filepath.Join(t.TempDir(), "publication")
	if _, err := Export(context.Background(), fixture.store, root, publicationOptions(fixture, store.SelectionLinkActionPlainText)); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyDirectory(root, DefaultLimits())
	if err != nil {
		t.Fatalf("publication archive failed verification: %v", err)
	}
	if report.Target != TargetPublicationHandoff {
		t.Fatalf("the manifest must record what this archive is: %+v", report)
	}
}

func TestPublicationHandoffRequiresASelector(t *testing.T) {
	fixture := newExportFixture(t)
	options := exportOptions(TargetPublicationHandoff)
	options.Policy = store.PrivacyPolicy{LinkAction: store.SelectionLinkActionPlainText}
	if _, err := Export(context.Background(), fixture.store, filepath.Join(t.TempDir(), "publication"), options); err == nil {
		t.Fatal("a publication with no selector would publish the whole library")
	}
}

// rewriteBodyLinks is the one place the exporter changes what a note says, so
// its refusal cases matter as much as its rewrites.
func TestRewriteBodyLinksLeavesUnmatchedSpansAlone(t *testing.T) {
	body := "before [private](document://default/documents/doc_private) after"
	start := strings.Index(body, "[private]")
	end := strings.Index(body, ") after") + 1

	rewritten, applied, skipped, _ := rewriteBodyLinks(body, store.SelectionLinkActionPlainText, []linkRewrite{
		{start: start, end: end, displayText: "private", rawTarget: "document://default/documents/doc_private"},
	})
	if applied != 1 || skipped != 0 || rewritten != "before private after" {
		t.Fatalf("applied=%d skipped=%d body=%q", applied, skipped, rewritten)
	}

	// A stale offset must leave the body alone rather than cut a hole in an
	// unrelated sentence.
	cases := map[string]linkRewrite{
		"span past the end":      {start: len(body) - 3, end: len(body) + 40, displayText: "x", rawTarget: "y"},
		"span not a link":        {start: 0, end: 6, displayText: "before", rawTarget: "nothing"},
		"target no longer there": {start: start, end: end, displayText: "private", rawTarget: "document://default/documents/doc_other"},
		"inverted span":          {start: 10, end: 5},
		"negative start":         {start: -1, end: 5},
	}
	for name, rewrite := range cases {
		got, applied, skipped, _ := rewriteBodyLinks(body, store.SelectionLinkActionPlainText, []linkRewrite{rewrite})
		if applied != 0 || skipped != 1 || got != body {
			t.Fatalf("%s: applied=%d skipped=%d body=%q", name, applied, skipped, got)
		}
	}
}

func TestRewriteBodyLinksAppliesLaterSpansFirst(t *testing.T) {
	body := "[one](a) middle [two](b) end"
	first := linkRewrite{start: 0, end: 8, displayText: "one", rawTarget: "a"}
	second := linkRewrite{start: 16, end: 24, displayText: "two", rawTarget: "b"}

	// Supplied out of order on purpose: applying an earlier span first would
	// invalidate every later offset.
	got, applied, skipped, _ := rewriteBodyLinks(body, store.SelectionLinkActionPlainText, []linkRewrite{first, second})
	if applied != 2 || skipped != 0 || got != "one middle two end" {
		t.Fatalf("applied=%d skipped=%d body=%q", applied, skipped, got)
	}
}

func TestPublicationLinkRewriteDecision(t *testing.T) {
	included := []string{"doc_included"}
	resources := []string{"res_included"}
	cases := map[string]struct {
		link store.DocumentLink
		want bool
	}{
		"included note":  {store.DocumentLink{ResolutionStatus: "resolved", TargetDocumentID: "doc_included"}, false},
		"withheld note":  {store.DocumentLink{ResolutionStatus: "resolved", TargetDocumentID: "doc_private"}, true},
		"included asset": {store.DocumentLink{ResolutionStatus: "resolved", TargetResourceID: "res_included"}, false},
		"withheld asset": {store.DocumentLink{ResolutionStatus: "resolved", TargetResourceID: "res_other"}, true},
		"external":       {store.DocumentLink{ResolutionStatus: "external"}, false},
		"unresolved":     {store.DocumentLink{ResolutionStatus: "unresolved"}, true},
		"ambiguous":      {store.DocumentLink{ResolutionStatus: "ambiguous"}, true},
		"invalid":        {store.DocumentLink{ResolutionStatus: "invalid"}, true},
		"deleted target": {store.DocumentLink{ResolutionStatus: "target_deleted"}, true},
		"self anchor":    {store.DocumentLink{ResolutionStatus: "resolved"}, false},
	}
	for name, testCase := range cases {
		if got := publicationLinkNeedsRewrite(testCase.link, included, resources); got != testCase.want {
			t.Fatalf("%s: got %v, want %v", name, got, testCase.want)
		}
	}
}

// A ```note-query block is declarative and stays that way in export. v0.5 E7
// requires it: a publication carries the block's *text*, never a materialized
// result, so a published note cannot leak the notes a query would have matched
// at export time — and cannot go stale either.
func TestPublicationHandoffCarriesQueryBlocksUnevaluated(t *testing.T) {
	fixture := newExportFixture(t)
	ctx := context.Background()

	block := "```note-query\nquery: tag:private\nfields: title, snippet\nlimit: 50\n```"
	body := "Outstanding work:\n\n" + block + "\n"
	doc, err := fixture.store.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "Dashboard", Body: body, NotebookID: fixture.public.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "publication")
	if _, err := Export(ctx, fixture.store, root, publicationOptions(fixture, store.SelectionLinkActionPlainText)); err != nil {
		t.Fatal(err)
	}

	published := bodyFor(t, root, decodeRecords(t, root), doc.ID)
	if published != body {
		t.Fatalf("the block's text was not carried verbatim:\n got %q\nwant %q", published, body)
	}
	if !strings.Contains(published, "```note-query") {
		t.Fatalf("the fence must survive export: %q", published)
	}
	// The query names `tag:private`, which is exactly the boundary a
	// publication exists to protect. If anything had evaluated it, the withheld
	// note's title would be in the published body.
	if strings.Contains(published, fixture.privateDoc.Title) {
		t.Fatalf("a query block was evaluated into the publication: %q", published)
	}
}
