package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

func decodeBatch(t *testing.T, body string) api.BatchResult {
	t.Helper()
	var result api.BatchResult
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&result); err != nil {
		t.Fatalf("decode batch result: %v (%s)", err, body)
	}
	return result
}

// A request that ran is a 200 even when every item failed. Per-item failure is
// the report's content, not the request's fate — a caller that gets a 4xx
// cannot tell "your call was wrong" from "one note was protected".
func TestBatchRESTReportsItemFailuresWithTwoHundred(t *testing.T) {
	s := newNotebookServer(t)
	ctx := context.Background()
	nb, err := s.store.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	docID := seedTaggedNote(t, s, "one")

	body := `{"operation":"move","notebook_id":"` + nb.ID + `","items":[{"document_id":"` + docID + `"},{"document_id":"doc_absent"}]}`
	rr := doJSON(t, s, http.MethodPost, "/api/v1/batch", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", rr.Code, rr.Body.String())
	}
	result := decodeBatch(t, rr.Body.String())
	if result.Mode != store.BatchModeBestEffort {
		t.Fatalf("best-effort is the default mode: %+v", result)
	}
	if result.Applied != 1 || result.Failed != 1 || len(result.Items) != 2 {
		t.Fatalf("expected one applied and one failed: %+v", result)
	}
	if result.Items[1].Error == "" {
		t.Fatalf("a failed item must carry its error: %+v", result.Items[1])
	}
}

// A malformed request is a 4xx: that is the caller getting the call wrong,
// which is a different thing from a note refusing an operation.
func TestBatchRESTRejectsMalformedRequests(t *testing.T) {
	s := newNotebookServer(t)
	docID := seedTaggedNote(t, s, "one")

	cases := []struct {
		name string
		body string
		want int
	}{
		{"unknown operation", `{"operation":"explode","items":[{"document_id":"` + docID + `"}]}`, http.StatusBadRequest},
		{"no items", `{"operation":"restore","items":[]}`, http.StatusBadRequest},
		{"move without a notebook", `{"operation":"move","items":[{"document_id":"` + docID + `"}]}`, http.StatusBadRequest},
		{"repeated document", `{"operation":"duplicate","items":[{"document_id":"` + docID + `"},{"document_id":"` + docID + `"}]}`, http.StatusBadRequest},
		{"trash without a base revision", `{"operation":"trash","items":[{"document_id":"` + docID + `"}]}`, http.StatusPreconditionRequired},
	}
	for _, tc := range cases {
		rr := doJSON(t, s, http.MethodPost, "/api/v1/batch", tc.body)
		if rr.Code != tc.want {
			t.Fatalf("%s: expected %d, got %d %s", tc.name, tc.want, rr.Code, rr.Body.String())
		}
	}
}

// The retry a client actually makes: same key, same arguments, after a dropped
// connection. The work must happen once and the second answer must say it is a
// replay.
func TestBatchRESTReplaysAKeyedRequest(t *testing.T) {
	s := newNotebookServer(t)
	docID := seedTaggedNote(t, s, "one")
	body := `{"request_key":"abc","operation":"add_tags","tags":["batched"],"items":[{"document_id":"` + docID + `"}]}`

	first := decodeBatch(t, doJSON(t, s, http.MethodPost, "/api/v1/batch", body).Body.String())
	if first.Replayed || first.Applied != 1 {
		t.Fatalf("first run should apply: %+v", first)
	}
	second := decodeBatch(t, doJSON(t, s, http.MethodPost, "/api/v1/batch", body).Body.String())
	if !second.Replayed {
		t.Fatalf("the retry must be reported as a replay: %+v", second)
	}
	if second.Applied != first.Applied {
		t.Fatalf("a replay must return the first outcomes: %+v vs %+v", second, first)
	}

	// And the tag exists once, not twice.
	tags, err := s.store.ListDocumentTags(context.Background(), docID)
	if err != nil {
		t.Fatalf("ListDocumentTags: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("the work must have happened once: %+v", tags)
	}

	// A key reused for different work is refused rather than answered.
	reused := `{"request_key":"abc","operation":"add_tags","tags":["different"],"items":[{"document_id":"` + docID + `"}]}`
	rr := doJSON(t, s, http.MethodPost, "/api/v1/batch", reused)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("a reused key with different arguments must be refused: %d %s", rr.Code, rr.Body.String())
	}
}

// Atomic mode over REST: nothing applied, and the report still names every item.
func TestBatchRESTAtomicRollsBack(t *testing.T) {
	s := newNotebookServer(t)
	ctx := context.Background()
	nb, _ := s.store.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Work"})
	docID := seedTaggedNote(t, s, "one")

	body := `{"operation":"move","mode":"atomic","notebook_id":"` + nb.ID + `","items":[{"document_id":"` + docID + `"},{"document_id":"doc_absent"}]}`
	result := decodeBatch(t, doJSON(t, s, http.MethodPost, "/api/v1/batch", body).Body.String())
	if result.RolledBack != 1 || result.Failed != 1 {
		t.Fatalf("expected one rolled back and one failed: %+v", result)
	}
	doc, err := s.store.GetDocument(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.NotebookID == nb.ID {
		t.Fatal("an atomic run that failed must leave the library untouched")
	}
}
