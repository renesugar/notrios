package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/query"
	"github.com/renesugar/notrios/internal/recoll"
)

type fakeSidecar struct {
	hits []recoll.Hit
	err  error
}

func (f fakeSidecar) Search(ctx context.Context, q query.Query, limit int) ([]recoll.Hit, error) {
	return f.hits, f.err
}

func TestSearchMergesSidecarOnlyHits(t *testing.T) {
	s := newNotebookServer(t)

	// A note whose match ("shopping") lives only in its tag: FTS5 misses it,
	// the sidecar (which indexes projected tags) finds it.
	rr := doJSON(t, s, http.MethodPost, "/api/v1/documents", `{"title":"tagged only","body":"nothing relevant"}`)
	var doc struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rr = doJSON(t, s, http.MethodPost, "/api/v1/documents/"+doc.ID+"/tags/shopping", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("tag: %d", rr.Code)
	}

	rr = doJSON(t, s, http.MethodPost, "/api/v1/search", `{"query":"shopping","limit":10}`)
	if strings.Contains(rr.Body.String(), doc.ID) {
		t.Fatalf("FTS5 alone should not find tag-only matches yet: %s", rr.Body.String())
	}

	s.AttachSidecar(fakeSidecar{hits: []recoll.Hit{{DocumentID: doc.ID, Title: "tagged only", Abstract: "tag match"}}})
	rr = doJSON(t, s, http.MethodPost, "/api/v1/search", `{"query":"shopping","limit":10}`)
	if !strings.Contains(rr.Body.String(), doc.ID) {
		t.Fatalf("sidecar hit not merged: %s", rr.Body.String())
	}

	// Sidecar failures degrade gracefully to FTS5-only results.
	s.AttachSidecar(fakeSidecar{err: context.DeadlineExceeded})
	rr = doJSON(t, s, http.MethodPost, "/api/v1/search", `{"query":"shopping","limit":10}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("search must survive sidecar failure: %d %s", rr.Code, rr.Body.String())
	}
}

func TestSearchGETReturnsLiveKeysetResults(t *testing.T) {
	s := newNotebookServer(t)
	rr := doJSON(t, s, http.MethodPost, "/api/v1/documents", `{"title":"GET search target","body":"needleviaget"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var doc struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil {
		t.Fatalf("decode document: %v", err)
	}

	rr = doJSON(t, s, http.MethodGet, "/api/v1/search?q=needleviaget&limit=1", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), doc.ID) {
		t.Fatalf("GET search: %d %s", rr.Code, rr.Body.String())
	}
}

func TestSidecarDuplicatesKeepCanonicalKeysetCursor(t *testing.T) {
	s := newNotebookServer(t)
	var firstID string
	for i := 0; i < 3; i++ {
		rr := doJSON(t, s, http.MethodPost, "/api/v1/documents",
			fmt.Sprintf(`{"title":"canonical %d","body":"duplicatequery"}`, i))
		var doc struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil {
			t.Fatalf("decode document: %v", err)
		}
		if i == 0 {
			firstID = doc.ID
		}
	}
	s.AttachSidecar(fakeSidecar{hits: []recoll.Hit{{DocumentID: firstID}}})

	rr := doJSON(t, s, http.MethodPost, "/api/v1/search", `{"query":"duplicatequery","limit":1}`)
	var page struct {
		NextCursor string `json:"next_cursor"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&page); err != nil {
		t.Fatalf("decode search page: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(page.NextCursor)
	if err != nil || !strings.Contains(string(raw), `"v":"k2"`) {
		t.Fatalf("duplicate-only sidecar replaced canonical keyset: %q (%s)", page.NextCursor, raw)
	}
}

func TestMergedSearchSnapshotPagesSidecarHitsStably(t *testing.T) {
	s := newNotebookServer(t)
	hits := []recoll.Hit{}
	ids := []string{}
	for _, title := range []string{"one", "two", "three"} {
		rr := doJSON(t, s, http.MethodPost, "/api/v1/documents",
			fmt.Sprintf(`{"title":%q,"body":"does not contain the query"}`, title))
		var doc struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil {
			t.Fatalf("decode document: %v", err)
		}
		ids = append(ids, doc.ID)
		hits = append(hits, recoll.Hit{DocumentID: doc.ID, Title: title, Abstract: "tag-only match"})
	}
	s.AttachSidecar(fakeSidecar{hits: hits})

	rr := doJSON(t, s, http.MethodPost, "/api/v1/search", `{"query":"sidecar-only","limit":2}`)
	var page1 struct {
		Hits       []struct{ ID string } `json:"hits"`
		NextCursor string                `json:"next_cursor"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&page1); err != nil {
		t.Fatalf("decode page 1: %v", err)
	}
	if len(page1.Hits) != 2 || page1.NextCursor == "" {
		t.Fatalf("page 1: %+v", page1)
	}

	rr = doJSON(t, s, http.MethodPost, "/api/v1/search",
		fmt.Sprintf(`{"query":"sidecar-only","limit":2,"cursor":%q}`, page1.NextCursor))
	var page2 struct {
		Hits       []struct{ ID string } `json:"hits"`
		NextCursor string                `json:"next_cursor"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&page2); err != nil {
		t.Fatalf("decode page 2: %v", err)
	}
	if len(page2.Hits) != 1 || page2.NextCursor != "" {
		t.Fatalf("page 2: %+v", page2)
	}
	seen := map[string]bool{}
	for _, hit := range append(page1.Hits, page2.Hits...) {
		if seen[hit.ID] {
			t.Fatalf("duplicate merged hit %s", hit.ID)
		}
		seen[hit.ID] = true
	}
	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("missing merged hit %s", id)
		}
	}

	rr = doJSON(t, s, http.MethodPost, "/api/v1/search",
		fmt.Sprintf(`{"query":"different","limit":2,"cursor":%q}`, page1.NextCursor))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "cursor_invalid") {
		t.Fatalf("snapshot cursor replay: %d %s", rr.Code, rr.Body.String())
	}
}
