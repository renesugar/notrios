package store

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// newEditorLinkFixture builds a library with titles chosen to separate the two
// suggestion passes: several sharing a prefix, one whose match is an interior
// word, one trashed, and one resource to link at.
func newEditorLinkFixture(t *testing.T) *SQLiteStore {
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
	if _, err := st.CreateResource(ctx, CreateResourceRequest{
		PreferredID: "res_diagram", Filename: "diagram.png", MIMEType: "image/png",
		Content: bytes.NewReader([]byte("\x89PNG\r\n\x1a\ndiagram")),
	}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct{ id, title, body string }{
		{"el_kitchen", "Kitchen", "# Kitchen\n\nThe room.\n"},
		{"el_kitchenplan", "Kitchen Plan", "# Kitchen Plan\n\n## Install and Setup\n\nSteps. ^step-one\n"},
		{"el_kitsch", "Kitsch", "Unrelated but shares a prefix.\n"},
		{"el_annual", "Annual Kitchen Review", "An interior-word match only.\n"},
		{"el_trashed", "Kitchen Archive", "About to be trashed.\n"},
	} {
		if _, err := st.CreateDocument(ctx, CreateDocumentRequest{
			PreferredID: entry.id, Title: entry.title, Body: entry.body,
		}); err != nil {
			t.Fatalf("create %s: %v", entry.id, err)
		}
	}
	doc, err := st.GetDocument(ctx, "el_trashed")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDocument(ctx, DeleteDocumentRequest{ID: "el_trashed", BaseRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatal(err)
	}
	return st
}

func suggestionIDs(resp DocumentSuggestionResponse) []string {
	ids := make([]string, 0, len(resp.Suggestions))
	for _, s := range resp.Suggestions {
		ids = append(ids, s.DocumentID)
	}
	return ids
}

func TestSuggestDocumentsRanksTitlePrefixesFirstAndInTitleOrder(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()

	resp, err := st.SuggestDocuments(ctx, DocumentSuggestionRequest{Query: "kit"})
	if err != nil {
		t.Fatal(err)
	}
	ids := suggestionIDs(resp)
	// Title order, NOCASE: Kitchen, Kitchen Plan, Kitsch. The interior-word
	// match comes after every prefix match.
	want := []string{"el_kitchen", "el_kitchenplan", "el_kitsch", "el_annual"}
	if len(ids) != len(want) {
		t.Fatalf("suggestions = %v, want %v", ids, want)
	}
	for i, id := range want {
		if ids[i] != id {
			t.Fatalf("suggestion %d = %s, want %s (all: %v)", i, ids[i], id, ids)
		}
	}
	for i, s := range resp.Suggestions {
		wantMatch := SuggestionMatchTitlePrefix
		if s.DocumentID == "el_annual" {
			wantMatch = SuggestionMatchWordPrefix
		}
		if s.Match != wantMatch {
			t.Fatalf("suggestion %d (%s) match = %q, want %q", i, s.DocumentID, s.Match, wantMatch)
		}
	}
	// A trashed note is never a link target offered to an author.
	for _, id := range ids {
		if id == "el_trashed" {
			t.Fatalf("a trashed note was suggested: %v", ids)
		}
	}
}

func TestSuggestDocumentsFindsAnInteriorWord(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()

	// "plan" matches no title's start; it is the second pass that has to find
	// "Kitchen Plan", which is the whole reason that pass exists.
	resp, err := st.SuggestDocuments(ctx, DocumentSuggestionRequest{Query: "plan"})
	if err != nil {
		t.Fatal(err)
	}
	ids := suggestionIDs(resp)
	if len(ids) != 1 || ids[0] != "el_kitchenplan" {
		t.Fatalf("suggestions for \"plan\" = %v, want [el_kitchenplan]", ids)
	}
	if resp.Suggestions[0].Match != SuggestionMatchWordPrefix {
		t.Fatalf("match = %q, want %q", resp.Suggestions[0].Match, SuggestionMatchWordPrefix)
	}
	if resp.Suggestions[0].URI != DocumentURI("default", "el_kitchenplan") {
		t.Fatalf("a suggestion carries the canonical URI, got %q", resp.Suggestions[0].URI)
	}
}

func TestSuggestDocumentsExcludesTheNoteBeingEditedAndReportsTruncation(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()

	resp, err := st.SuggestDocuments(ctx, DocumentSuggestionRequest{Query: "kit", ExcludeDocumentID: "el_kitchen"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range suggestionIDs(resp) {
		if id == "el_kitchen" {
			t.Fatalf("the excluded note was suggested: %v", suggestionIDs(resp))
		}
	}

	capped, err := st.SuggestDocuments(ctx, DocumentSuggestionRequest{Query: "kit", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(capped.Suggestions) != 1 || !capped.Truncated {
		t.Fatalf("limit 1 = %+v, want one suggestion and truncated", capped)
	}
}

func TestSuggestDocumentsValidatesItsBounds(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()
	for name, req := range map[string]DocumentSuggestionRequest{
		"too short": {Query: "k"},
		"empty":     {Query: "   "},
		"too long":  {Query: strings.Repeat("k", MaxSuggestionQueryBytes+1)},
		"limit":     {Query: "kit", Limit: MaxDocumentSuggestions + 1},
		"negative":  {Query: "kit", Limit: -1},
	} {
		if _, err := st.SuggestDocuments(ctx, req); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
}

// A typed `%` is a literal, not a wildcard. Without escaping it would match
// every note in the library — a wrong answer rather than an unsafe one, but
// wrong is enough.
func TestSuggestDocumentsTreatsWildcardCharactersAsText(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()
	resp, err := st.SuggestDocuments(ctx, DocumentSuggestionRequest{Query: "%%"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Suggestions) != 0 {
		t.Fatalf("a literal %%%% matched %v", suggestionIDs(resp))
	}
	resp, err = st.SuggestDocuments(ctx, DocumentSuggestionRequest{Query: "_i"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Suggestions) != 0 {
		t.Fatalf("a literal underscore matched %v", suggestionIDs(resp))
	}
}

func checkedByTarget(resp CheckLinksResponse, raw string) (CheckedLink, bool) {
	for _, link := range resp.Links {
		if link.RawTarget == raw {
			return link, true
		}
	}
	return CheckedLink{}, false
}

func TestCheckLinksClassifiesAnUnsavedBuffer(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()

	body := strings.Join([]string{
		"[by title](Kitchen)",
		"[canonical](" + DocumentURI("default", "el_kitchen") + ")",
		"[missing](document://default/documents/el_nope)",
		"[external](https://example.com/page)",
		"[resource](resource://default/resources/res_diagram)",
		"[stale anchor](" + DocumentURI("default", "el_kitchenplan") + "#no-such-heading)",
		"[good anchor](" + DocumentURI("default", "el_kitchenplan") + "#install-and-setup)",
		"[marker](" + DocumentURI("default", "el_kitchenplan") + "#^step-one)",
	}, "\n\n")

	resp, err := st.CheckLinks(ctx, CheckLinksRequest{Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 8 {
		t.Fatalf("total = %d, want 8: %+v", resp.Total, resp.Links)
	}
	for raw, want := range map[string]string{
		"Kitchen":                                  "resolved",
		DocumentURI("default", "el_kitchen"):       "resolved",
		"document://default/documents/el_nope":     "unresolved",
		"https://example.com/page":                 "external",
		"resource://default/resources/res_diagram": "resolved",
	} {
		link, ok := checkedByTarget(resp, raw)
		if !ok {
			t.Fatalf("%q was not checked: %+v", raw, resp.Links)
		}
		if link.Status != want {
			t.Fatalf("%q status = %q, want %q", raw, link.Status, want)
		}
	}

	// A title-resolved link knows the URI it already points at, which is what
	// an editor offers to substitute.
	byTitle, _ := checkedByTarget(resp, "Kitchen")
	if byTitle.CanonicalTarget != DocumentURI("default", "el_kitchen") {
		t.Fatalf("canonical target = %q, want the document URI", byTitle.CanonicalTarget)
	}
	// A link already canonical needs no substitution offered.
	canonical, _ := checkedByTarget(resp, DocumentURI("default", "el_kitchen"))
	if canonical.CanonicalTarget != "" {
		t.Fatalf("a canonical link should offer no rewrite, got %q", canonical.CanonicalTarget)
	}

	for _, link := range resp.Links {
		switch link.AnchorValue {
		case "no-such-heading":
			if link.Status != "stale_anchor" {
				t.Fatalf("a missing heading is stale_anchor, got %q", link.Status)
			}
		case "install-and-setup", "step-one":
			if link.Status != "resolved" {
				t.Fatalf("anchor %q status = %q, want resolved", link.AnchorValue, link.Status)
			}
		}
	}
	if resp.Unresolved != 2 {
		t.Fatalf("unresolved = %d, want the missing note and the stale anchor", resp.Unresolved)
	}
	// Every link is located, which is what a marker needs.
	for _, link := range resp.Links {
		if link.Line <= 0 || link.EndByte <= link.StartByte {
			t.Fatalf("link %q is not located: %+v", link.RawTarget, link)
		}
	}
}

// While typing, the buffer is the truth about its own headings. Checking a
// just-typed anchor against yesterday's saved blocks would mark a correct link
// broken.
func TestCheckLinksResolvesSelfAnchorsAgainstTheBuffer(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()

	// The heading exists only in the unsaved buffer.
	body := "# Kitchen\n\n## Brand New Section\n\n[here](#brand-new-section)\n\n[gone](#the-room)\n"
	resp, err := st.CheckLinks(ctx, CheckLinksRequest{DocumentID: "el_kitchen", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Links) != 2 {
		t.Fatalf("anchor-only links were not checked: %+v", resp.Links)
	}
	statuses := map[string]string{}
	for _, link := range resp.Links {
		statuses[link.AnchorValue] = link.Status
	}
	if statuses["brand-new-section"] != "resolved" {
		t.Fatalf("a heading typed in this buffer must resolve, got %q", statuses["brand-new-section"])
	}
	if statuses["the-room"] != "stale_anchor" {
		t.Fatalf("an anchor naming nothing in this buffer is stale, got %q", statuses["the-room"])
	}

	// Without a document ID there is no self to resolve against.
	anonymous, err := st.CheckLinks(ctx, CheckLinksRequest{Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if anonymous.Unresolved != anonymous.Total {
		t.Fatalf("anchor-only links cannot resolve without a note: %+v", anonymous)
	}
}

func TestCheckLinksIsReadOnlyAndBounded(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()

	before, err := st.GetDocument(ctx, "el_kitchen")
	if err != nil {
		t.Fatal(err)
	}
	revisionsBefore, err := st.ListDocumentRevisions(ctx, "el_kitchen")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CheckLinks(ctx, CheckLinksRequest{
		DocumentID: "el_kitchen",
		Body:       "# Replaced entirely\n\n[x](Kitchen Plan)\n",
	}); err != nil {
		t.Fatal(err)
	}
	after, err := st.GetDocument(ctx, "el_kitchen")
	if err != nil {
		t.Fatal(err)
	}
	revisionsAfter, err := st.ListDocumentRevisions(ctx, "el_kitchen")
	if err != nil {
		t.Fatal(err)
	}
	if before.CurrentRevisionID != after.CurrentRevisionID || before.Body != after.Body || len(revisionsBefore) != len(revisionsAfter) {
		t.Fatalf("checking a buffer changed the note: %q -> %q", before.CurrentRevisionID, after.CurrentRevisionID)
	}

	if _, err := st.CheckLinks(ctx, CheckLinksRequest{Body: strings.Repeat("x", MaxCheckBodyBytes+1)}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an oversized buffer must be refused, got %v", err)
	}
	empty, err := st.CheckLinks(ctx, CheckLinksRequest{Body: ""})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total != 0 || len(empty.Links) != 0 {
		t.Fatalf("an empty buffer has no links: %+v", empty)
	}
}

// The check must agree with what a save would record — that is the reason the
// server parses the body instead of trusting a client-extracted target list.
func TestCheckLinksAgreesWithTheSavedLinkRecords(t *testing.T) {
	st := newEditorLinkFixture(t)
	ctx := context.Background()

	body := strings.Join([]string{
		"[by title](Kitchen)",
		"[missing](document://default/documents/el_nope)",
		"[external](https://example.com/page)",
	}, "\n\n")

	checked, err := st.CheckLinks(ctx, CheckLinksRequest{DocumentID: "el_kitsch", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := st.GetDocument(ctx, "el_kitsch")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateDocument(ctx, UpdateDocumentRequest{
		ID: "el_kitsch", Title: doc.Title, Body: body, BaseRevisionID: doc.CurrentRevisionID,
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := st.ListDocumentLinks(ctx, "el_kitsch", "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Outgoing) != len(checked.Links) {
		t.Fatalf("checked %d links, saved %d", len(checked.Links), len(saved.Outgoing))
	}
	for i, link := range saved.Outgoing {
		if link.ResolutionStatus != checked.Links[i].Status {
			t.Fatalf("link %d: saved %q, checked %q", i, link.ResolutionStatus, checked.Links[i].Status)
		}
		if link.RawTarget != checked.Links[i].RawTarget || link.SourceStartByte != checked.Links[i].StartByte {
			t.Fatalf("link %d located differently: saved %+v checked %+v", i, link, checked.Links[i])
		}
	}
}
