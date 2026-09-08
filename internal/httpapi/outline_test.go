package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/markdownblocks"
	"github.com/renesugar/notrios/internal/store"
)

// TestOutlineAnchorsResolveAgainstStoredSlugs is the defect this file was
// written for. The outline had its own heading parser and its own slug
// function, and they disagreed with the one that stores a heading's slug on
// everything outside ASCII: `Café notes` is stored as `café-notes` and the
// outline reported `caf-notes`, and a Japanese heading was reported with an
// empty anchor. Both are anchors that resolve to nothing, handed back as
// though they were links.
func TestOutlineAnchorsResolveAgainstStoredSlugs(t *testing.T) {
	body := strings.Join([]string{
		"# Notes",
		"",
		"## Café notes",
		"",
		"Body.",
		"",
		"### 日本語の見出し",
		"",
		"More.",
		"",
		"## Notes on notes",
		"",
	}, "\n")

	stored := map[string]bool{}
	for _, block := range markdownblocks.Extract("doc_outline", body) {
		if block.Kind == markdownblocks.KindHeading {
			stored[block.Slug] = true
		}
	}
	if len(stored) < 4 {
		t.Fatalf("expected four stored heading slugs, got %d: %v", len(stored), stored)
	}

	outline := extractDocumentOutline("doc_outline", body)
	if len(outline.Headings) != 4 {
		t.Fatalf("outline has %d headings, want 4", len(outline.Headings))
	}
	for _, heading := range outline.Headings {
		if heading.Anchor == "" {
			t.Errorf("heading %q has an empty anchor, which resolves to nothing", heading.Title)
			continue
		}
		if !stored[heading.Anchor] {
			t.Errorf("outline anchor %q for %q is not a slug any block carries", heading.Anchor, heading.Title)
		}
	}
}

// TestOutlineReportsLevelsTitlesAndLines keeps the rest of the contract, since
// the parser changed underneath it.
func TestOutlineReportsLevelsTitlesAndLines(t *testing.T) {
	body := "# One\n\ntext\n\n## Two\n\n### Three\n"
	outline := extractDocumentOutline("doc_levels", body)

	want := []struct {
		level int
		title string
		line  int
	}{{1, "One", 1}, {2, "Two", 5}, {3, "Three", 7}}
	if len(outline.Headings) != len(want) {
		t.Fatalf("outline has %d headings, want %d: %+v", len(outline.Headings), len(want), outline.Headings)
	}
	for i, expected := range want {
		got := outline.Headings[i]
		if got.Level != expected.level || got.Title != expected.title || got.Line != expected.line {
			t.Errorf("heading %d = {level %d, title %q, line %d}, want {%d, %q, %d}",
				i, got.Level, got.Title, got.Line, expected.level, expected.title, expected.line)
		}
	}
}

// TestUnknownLinkDirectionIsRefused guards a wrong answer with the shape of a
// right one. An unrecognised direction fell through both branches of the store
// query and returned an empty page, so `?direction=out` reported that a note
// had no links at all -- with a 200, which a caller cannot tell from the truth.
func TestUnknownLinkDirectionIsRefused(t *testing.T) {
	server := newGraphServer(t)

	for _, direction := range []string{"outgoing", "incoming", "both", ""} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/documents/gr_b/links?direction="+direction, nil)
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("direction %q returned %d, want 200: %s", direction, recorder.Code, recorder.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/documents/gr_b/links?direction=out", nil)
	recorder := httptest.NewRecorder()
	server.mux.ServeHTTP(recorder, request)
	if recorder.Code == http.StatusOK {
		t.Errorf("an unrecognised direction returned 200 and an empty page: %s", recorder.Body.String())
	}
}

// TestTagLookupNarrowsAndRefusesAMissingTag is H26 on the REST surface: asking
// about one tag is a lookup, and a lookup that finds nothing is a 404. An empty
// list would make "no such tag" and "a tag with nothing on it" the same answer,
// which is exactly the distinction the caller asked for.
func TestTagLookupNarrowsAndRefusesAMissingTag(t *testing.T) {
	server := newGraphServer(t)
	ctx := context.Background()
	documents, err := server.store.ListNotebookDocuments(ctx, store.DefaultNotebookID, store.DocumentPageRequest{Limit: 5})
	if err != nil || len(documents.Documents) == 0 {
		t.Fatalf("seeding: %v (%d documents)", err, len(documents.Documents))
	}
	for _, name := range []string{"todo", "shopping", "shopping/mall", "shoppingcart"} {
		if _, err := server.store.AddDocumentTag(ctx, documents.Documents[0].ID, name); err != nil {
			t.Fatalf("tagging %q: %v", name, err)
		}
	}

	get := func(query string) (int, string) {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/tags"+query, nil)
		recorder := httptest.NewRecorder()
		server.mux.ServeHTTP(recorder, request)
		return recorder.Code, recorder.Body.String()
	}

	if code, body := get("?name=todo"); code != http.StatusOK || !strings.Contains(body, `"todo"`) {
		t.Errorf("looking up one tag returned %d: %s", code, body)
	}
	if code, _ := get("?name=nope"); code != http.StatusNotFound {
		t.Errorf("a tag that does not exist returned %d, want 404", code)
	}
	if code, body := get("?prefix=shopping"); code != http.StatusOK {
		t.Errorf("branch listing returned %d: %s", code, body)
	} else if strings.Contains(body, "shoppingcart") {
		t.Errorf("the shopping branch included shoppingcart: %s", body)
	}
	if code, body := get("?limit=1"); code != http.StatusOK || !strings.Contains(body, `"truncated":true`) {
		t.Errorf("a limit that cut the answer short returned %d without saying so: %s", code, body)
	}
	if code, _ := get("?name=todo&prefix=shopping"); code != http.StatusBadRequest {
		t.Errorf("asking two different questions at once returned %d, want 400", code)
	}
	if code, _ := get("?limit=0"); code != http.StatusBadRequest {
		t.Errorf("a limit of zero returned %d, want 400", code)
	}
}
