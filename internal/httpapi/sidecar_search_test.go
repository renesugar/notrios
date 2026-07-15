package httpapi

import (
	"context"
	"encoding/json"
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
