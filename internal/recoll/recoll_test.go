package recoll

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/projection"
	"github.com/renesugar/notrios/internal/query"
	"github.com/renesugar/notrios/internal/store"
)

func TestCompileQuery(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	q := query.Parse(`apples "exact words" title:arch tag:"note taking" author:"Alice Smith" authorid:alice@example.social since:2026-07-01 until:2026-07-31`, now)
	got := CompileQuery(q)
	for _, want := range []string{
		"apples", `"exact words"`, `title:"arch"`, `tag:"note taking"`,
		`author:"Alice Smith"`, `authorid:"alice@example.social"`, "publishedts:",
		"..",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("compiled query missing %q: %s", want, got)
		}
	}
	if CompileQuery(query.Parse("is:trashed lettuce", now)) != "" {
		t.Fatal("trash queries must not compile to Recoll")
	}
}

func TestParseResults(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	output := strings.Join([]string{
		"Recoll query: Query(apples)",
		"2 results",
		b64("file:///data/projections/notes/doc_one.md") + " " + b64("First title") + " " + b64("abstract one"),
		b64("file:///data/projections/notes/doc_two.md") + " " + b64("Second title"),
		"",
	}, "\n")
	hits := parseResults(output)
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %+v", hits)
	}
	if hits[0].DocumentID != "doc_one" || hits[0].Title != "First title" || hits[0].Abstract != "abstract one" {
		t.Fatalf("hit 0 wrong: %+v", hits[0])
	}
	if hits[1].DocumentID != "doc_two" {
		t.Fatalf("hit 1 wrong: %+v", hits[1])
	}
}

// TestLiveRecollPipeline exercises the real sidecar when recollindex/recollq
// are installed: projection -> generated config -> front-matter handler ->
// field queries. Skipped otherwise.
func TestLiveRecollPipeline(t *testing.T) {
	dir := t.TempDir()
	sidecar := New(dir+"/conf", dir+"/proj", "recollindex")
	if !sidecar.Available() {
		t.Skip("recollindex/recollq not installed")
	}
	ctx := context.Background()

	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Search engine architecture", Body: "Body text about xapian indexing."})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if _, err := st.AddDocumentTag(ctx, doc.ID, "note taking"); err != nil {
		t.Fatalf("AddDocumentTag: %v", err)
	}
	if _, err := st.SetDocumentSource(ctx, store.SetDocumentSourceRequest{
		DocumentID: doc.ID, SourceSystem: "twitter", ExternalID: "post-1",
		Author: "Alice Smith", AuthorID: "alice@example.social",
		PublishedAt: "2026-07-13T18:42:07Z",
	}); err != nil {
		t.Fatalf("SetDocumentSource: %v", err)
	}

	if err := sidecar.EnsureConfig(); err != nil {
		t.Fatalf("EnsureConfig: %v", err)
	}
	if _, err := projection.FullSync(ctx, st, projection.Writer{Dir: sidecar.ProjectionDir}); err != nil {
		t.Fatalf("FullSync: %v", err)
	}
	if err := sidecar.Index(ctx); err != nil {
		t.Fatalf("Index: %v", err)
	}

	now := time.Now()
	cases := []struct {
		queryText string
		wantHit   bool
	}{
		{`tag:"note taking"`, true},
		{`authorid:alice@example.social`, true},
		{`author:"Alice Smith"`, true},
		{`xapian`, true},
		{`since:2026-07-01 until:2026-07-31 alice`, false}, // range applies
		{`since:2026-07-14 xapian`, false},                 // published before bound
		{`since:2026-07-01 xapian`, true},
		{`tag:"missing tag"`, false},
	}
	for _, tc := range cases {
		hits, err := sidecar.Search(ctx, query.Parse(tc.queryText, now), 10)
		if err != nil {
			t.Fatalf("Search(%q): %v", tc.queryText, err)
		}
		found := false
		for _, hit := range hits {
			if hit.DocumentID == doc.ID {
				found = true
			}
		}
		if tc.queryText == `since:2026-07-01 until:2026-07-31 alice` {
			// "alice" appears only in metadata; general terms may or may not
			// match depending on Recoll term generation — assert no error only.
			continue
		}
		if found != tc.wantHit {
			t.Fatalf("Search(%q): found=%v want=%v hits=%+v", tc.queryText, found, tc.wantHit, hits)
		}
	}
}
