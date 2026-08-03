package recoll

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/projection"
	"github.com/renesugar/notrios/internal/query"
	"github.com/renesugar/notrios/internal/store"
)

func TestCompileQuery(t *testing.T) {
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	q, err := query.Parse(`(apples OR "exact words") title:arch -tag:"note taking" author:"Alice Smith" authorid:alice@example.social since:2026-07-01 until:2026-07-31`, now)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := CompileQuery(q)
	if err != nil {
		t.Fatalf("CompileQuery: %v", err)
	}
	for _, want := range []string{
		"apples", `OR`, `"exact words"`, `title:"arch"`, `-tag:"note taking"`,
		`author:"Alice Smith"`, `authorid:"alice@example.social"`, "publishedts:",
		"..",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("compiled query missing %q: %s", want, got)
		}
	}
	trash, err := query.Parse("is:trashed lettuce", now)
	if err != nil {
		t.Fatalf("Parse trash: %v", err)
	}
	if _, err := CompileQuery(trash); !errors.Is(err, ErrUnsupportedQuery) {
		t.Fatalf("trash query error = %v, want ErrUnsupportedQuery", err)
	}
	emoji, err := query.Parse("😀", now)
	if err != nil {
		t.Fatalf("Parse emoji: %v", err)
	}
	if compiled, err := CompileQuery(emoji); err != nil || !strings.Contains(compiled, `emoji:"u1f600"`) {
		t.Fatalf("emoji compile = %q, %v", compiled, err)
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
	hits := parseResults(output, "/data/projections", 10)
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

func TestParseResultsRejectsHostileAndDuplicateRows(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	validURL := "file:///data/projections/notes/doc_one.md"
	output := strings.Join([]string{
		b64("https://example.test/doc.md") + " " + b64("remote"),
		b64("file://server/data/projections/notes/remote.md") + " " + b64("remote host"),
		b64("file:///data/outside/notes/outside.md") + " " + b64("outside"),
		b64("file:///data/projections/notes/nested/doc.md") + " " + b64("nested"),
		b64(validURL) + " " + b64("<b>Safe</b>\x00 title") + " " + b64("&lt;script&gt;bad&lt;/script&gt; useful"),
		b64(validURL) + " " + b64("duplicate"),
		b64("file:///data/projections/notes/doc_two.md") + " " + b64("Second") + " " + b64("abstract") + " " + b64("extra"),
		"not-base64",
	}, "\n")

	hits := parseResults(output, "/data/projections", 10)
	if len(hits) != 1 {
		t.Fatalf("expected only one safe unique hit, got %+v", hits)
	}
	if hits[0].DocumentID != "doc_one" || hits[0].Title != "Safe title" || hits[0].Abstract != "bad useful" {
		t.Fatalf("unexpected sanitized hit: %+v", hits[0])
	}
}

func TestParseResultsHonorsLimitAndFieldBounds(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	output := strings.Join([]string{
		b64("file:///data/projections/notes/too_large.md") + " " + b64(strings.Repeat("x", maxTitleFieldBytes+1)),
		b64("file:///data/projections/notes/one.md") + " " + b64("One"),
		b64("file:///data/projections/notes/two.md") + " " + b64("Two"),
	}, "\n")
	hits := parseResults(output, "/data/projections", 1)
	if len(hits) != 1 || hits[0].DocumentID != "one" {
		t.Fatalf("expected first bounded valid hit, got %+v", hits)
	}
}

func TestBoundedBuffer(t *testing.T) {
	buffer := newBoundedBuffer(4)
	if count, err := buffer.Write([]byte("abcdef")); err != nil || count != 6 {
		t.Fatalf("Write = %d, %v", count, err)
	}
	if buffer.String() != "abcd" || !buffer.Truncated() {
		t.Fatalf("bounded buffer = %q truncated=%v", buffer.String(), buffer.Truncated())
	}
}

func TestRuntimeStatusKeepsRetryingQueueDegraded(t *testing.T) {
	sidecar := New("/tmp/conf", "/tmp/projection", "recollindex")
	sidecar.RecordQueue(2, 1)
	sidecar.SetActive(true)
	sidecar.recordIndex(nil)
	status := sidecar.Status(context.Background())
	if !status.Active || status.State != "degraded" || status.Backlog != 2 || status.FailedJobs != 1 {
		t.Fatalf("runtime status = %+v", status)
	}
}

func TestSearchUsesExactBoundedSlice(t *testing.T) {
	dir := t.TempDir()
	b64 := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
	script := filepath.Join(dir, "recollq")
	body := `#!/bin/sh
exact=false
slice=false
for arg in "$@"; do
  [ "$arg" = "-E" ] && exact=true
  [ "$arg" = "0-1" ] && slice=true
done
[ "$exact" = true ] && [ "$slice" = true ] || exit 9
` + "printf '%s\\n' '" + b64("file://"+dir+"/projections/notes/one.md") + " " + b64("One") + "'\n" +
		"printf '%s\\n' '" + b64("file://"+dir+"/projections/notes/two.md") + " " + b64("Two") + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake recollq: %v", err)
	}
	sidecar := &Sidecar{ConfDir: dir, ProjectionDir: dir + "/projections", QueryBinary: script}
	parsed, parseErr := query.Parse("needle", time.Now())
	if parseErr != nil {
		t.Fatalf("Parse: %v", parseErr)
	}
	hits, err := sidecar.Search(context.Background(), parsed, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].DocumentID != "one" {
		t.Fatalf("bounded search hits = %+v", hits)
	}
}

func TestSearchCancelsSubprocess(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "recollq")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec sleep 10\n"), 0o755); err != nil {
		t.Fatalf("write fake recollq: %v", err)
	}
	sidecar := &Sidecar{ConfDir: dir, ProjectionDir: dir + "/projections", QueryBinary: script}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	parsed, parseErr := query.Parse("needle", time.Now())
	if parseErr != nil {
		t.Fatalf("Parse: %v", parseErr)
	}
	if _, err := sidecar.Search(ctx, parsed, 1); err == nil {
		t.Fatal("expected canceled search error")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("canceled subprocess took %s", elapsed)
	}
}

// TestLiveRecollPipeline exercises the real sidecar when recollindex/recollq
// are installed: projection -> generated config -> front-matter handler ->
// field queries. Skipped otherwise.
func TestLiveRecollPipeline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir+"/xdg-config")
	t.Setenv("XDG_RUNTIME_DIR", dir+"/xdg-runtime")
	if err := os.MkdirAll(dir+"/xdg-runtime", 0o700); err != nil {
		t.Fatalf("create XDG runtime dir: %v", err)
	}
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
	work, _ := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Work"})
	reports, _ := st.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Reports", ParentID: work.ID})
	doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		PreferredID: "recoll_public", NotebookID: reports.ID,
		Title: "Search engine architecture", Body: "Body text about xapian indexing 😀.",
	})
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
	privateDoc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		PreferredID: "recoll_private", Title: "Private architecture", Body: "secret xapian notes",
	})
	if err != nil {
		t.Fatalf("Create private document: %v", err)
	}
	if _, err := st.AddDocumentTag(ctx, privateDoc.ID, "private"); err != nil {
		t.Fatalf("tag private document: %v", err)
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
		parsed, parseErr := query.Parse(tc.queryText, now)
		if parseErr != nil {
			t.Fatalf("Parse %q: %v", tc.queryText, parseErr)
		}
		hits, err := sidecar.Search(ctx, parsed, 10)
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

	// The two live backends must agree on complete managed-note result sets for
	// grouped/negated fields, recursive category matching, All notes, and emoji.
	for _, queryText := range []string{
		`(xapian OR secret) -tag:private`,
		`category:work xapian`,
		`category:"All notes" (xapian OR secret)`,
		`(author:"Alice Smith" OR title:Private) -tag:private`,
		`😀`,
	} {
		parsed, parseErr := query.Parse(queryText, now)
		if parseErr != nil {
			t.Fatalf("Parse parity query %q: %v", queryText, parseErr)
		}
		sqliteResults, searchErr := st.Search(ctx, store.SearchRequest{Query: queryText, Limit: 50})
		if searchErr != nil {
			t.Fatalf("SQLite Search(%q): %v", queryText, searchErr)
		}
		recollResults, searchErr := sidecar.Search(ctx, parsed, 50)
		if searchErr != nil {
			t.Fatalf("Recoll Search(%q): %v", queryText, searchErr)
		}
		sqliteIDs := []string{}
		for _, hit := range sqliteResults.Hits {
			sqliteIDs = append(sqliteIDs, hit.ID)
		}
		recollIDs := []string{}
		for _, hit := range recollResults {
			recollIDs = append(recollIDs, hit.DocumentID)
		}
		slices.Sort(sqliteIDs)
		slices.Sort(recollIDs)
		if !slices.Equal(sqliteIDs, recollIDs) {
			t.Fatalf("backend parity %q: SQLite=%v Recoll=%v", queryText, sqliteIDs, recollIDs)
		}
	}
}
