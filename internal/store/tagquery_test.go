package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// taggedLibrary is a vocabulary with a branch in it, and one tag whose name
// starts with the same letters as the branch but is not under it.
func taggedLibrary(t *testing.T) (*store.SQLiteStore, context.Context) {
	t.Helper()
	root := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatalf("opening a library: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrapping: %v", err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Tagged", Body: "body"})
	if err != nil {
		t.Fatalf("creating a note: %v", err)
	}
	for _, name := range []string{"todo", "shopping", "shopping/mall", "shopping/market", "shoppingcart", "wetland"} {
		if _, err := st.AddDocumentTag(ctx, doc.ID, name); err != nil {
			t.Fatalf("tagging %q: %v", name, err)
		}
	}
	return st, ctx
}

func names(page store.TagPage) []string {
	out := make([]string, 0, len(page.Tags))
	for _, tag := range page.Tags {
		out = append(out, tag.Name)
	}
	return out
}

func TestTagQueryByNameFindsExactlyOne(t *testing.T) {
	st, ctx := taggedLibrary(t)

	page, err := st.ListTags(ctx, store.TagQuery{Name: "todo"})
	if err != nil {
		t.Fatalf("looking one tag up: %v", err)
	}
	if got := names(page); len(got) != 1 || got[0] != "todo" {
		t.Errorf("name lookup returned %v, want [todo]", got)
	}

	// Case-insensitively, because that is how tagging already decides whether
	// to create a tag or reuse one.
	upper, err := st.ListTags(ctx, store.TagQuery{Name: "TODO"})
	if err != nil {
		t.Fatalf("looking one tag up by a different case: %v", err)
	}
	if len(upper.Tags) != 1 {
		t.Errorf("TODO matched %v, want the one tag named todo", names(upper))
	}

	absent, err := st.ListTags(ctx, store.TagQuery{Name: "nope"})
	if err != nil {
		t.Fatalf("looking up a tag that does not exist: %v", err)
	}
	if len(absent.Tags) != 0 {
		t.Errorf("a tag that does not exist matched %v", names(absent))
	}
}

// TestTagQueryByPrefixIsABranchNotAStringMatch is the distinction that makes a
// prefix mean something in a hierarchy: "shopping" contains "shopping/mall" and
// does not contain "shoppingcart".
func TestTagQueryByPrefixIsABranchNotAStringMatch(t *testing.T) {
	st, ctx := taggedLibrary(t)

	page, err := st.ListTags(ctx, store.TagQuery{Prefix: "shopping"})
	if err != nil {
		t.Fatalf("listing a branch: %v", err)
	}
	got := names(page)
	want := map[string]bool{"shopping": true, "shopping/mall": true, "shopping/market": true}
	if len(got) != len(want) {
		t.Errorf("branch listing returned %v, want the three shopping tags", got)
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("%q is not under the shopping branch and was listed", name)
		}
	}
}

// TestTagQueryPrefixDoesNotTreatWildcardsAsPattern guards a tag named with the
// characters LIKE reserves. A tag called "50%" must be a prefix of its own
// children and not of everything in the library.
func TestTagQueryPrefixDoesNotTreatWildcardsAsPattern(t *testing.T) {
	st, ctx := taggedLibrary(t)
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Odd", Body: "body"})
	if err != nil {
		t.Fatalf("creating a note: %v", err)
	}
	for _, name := range []string{"50%", "50%/off", "500"} {
		if _, err := st.AddDocumentTag(ctx, doc.ID, name); err != nil {
			t.Fatalf("tagging %q: %v", name, err)
		}
	}

	page, err := st.ListTags(ctx, store.TagQuery{Prefix: "50%"})
	if err != nil {
		t.Fatalf("listing the branch: %v", err)
	}
	for _, name := range names(page) {
		if name != "50%" && name != "50%/off" {
			t.Errorf("%q matched a prefix containing a LIKE wildcard", name)
		}
	}
	if len(page.Tags) != 2 {
		t.Errorf("the branch returned %v, want the two 50%% tags", names(page))
	}
}

// TestTagQueryLimitReportsTruncation is why the page carries the flag: a
// listing that silently stops lies about the size of a library.
func TestTagQueryLimitReportsTruncation(t *testing.T) {
	st, ctx := taggedLibrary(t)

	page, err := st.ListTags(ctx, store.TagQuery{Limit: 2})
	if err != nil {
		t.Fatalf("listing with a limit: %v", err)
	}
	if len(page.Tags) != 2 {
		t.Errorf("a limit of 2 returned %d tags", len(page.Tags))
	}
	if !page.Truncated {
		t.Error("a limit that cut the answer short reported truncated=false")
	}

	whole, err := st.ListTags(ctx, store.TagQuery{Limit: 100})
	if err != nil {
		t.Fatalf("listing with a generous limit: %v", err)
	}
	if whole.Truncated {
		t.Error("a limit larger than the vocabulary reported truncated=true")
	}

	// The boundary: a limit exactly equal to the number of tags is not a
	// truncation, and an off-by-one here would report one on every full page.
	exact, err := st.ListTags(ctx, store.TagQuery{Limit: len(whole.Tags)})
	if err != nil {
		t.Fatalf("listing with an exact limit: %v", err)
	}
	if exact.Truncated {
		t.Errorf("a limit equal to the %d tags reported truncated=true", len(whole.Tags))
	}
}
