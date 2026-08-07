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

func newNoteQueryServer(t *testing.T) *Server {
	t.Helper()
	ctx := context.Background()
	st, err := store.OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct{ id, title, body string }{
		{"nqh_a", "Sourdough", "flour water salt"},
		{"nqh_b", "Focaccia", "flour water olive oil"},
	} {
		doc, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: entry.id, Title: entry.title, Body: entry.body,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddDocumentTag(ctx, doc.ID, "todo"); err != nil {
			t.Fatal(err)
		}
	}
	return NewServerWithStore(st)
}

func runBlock(t *testing.T, s *Server, block string) api.NoteQueryResult {
	t.Helper()
	body, err := json.Marshal(map[string]string{"block": block})
	if err != nil {
		t.Fatal(err)
	}
	rr := doJSON(t, s, http.MethodPost, "/api/v1/note-queries/run", string(body))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var result api.NoteQueryResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestNoteQueryRouteRunsABlock(t *testing.T) {
	s := newNoteQueryServer(t)
	result := runBlock(t, s, "query: tag:todo\nfields: updated\nlimit: 5")
	if result.Error != "" || len(result.Rows) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if result.Rows[0].URI == "" || result.Rows[0].UpdatedAt == "" {
		t.Fatalf("row = %+v, want a URI and the requested timestamp", result.Rows[0])
	}
	if result.Spec.Limit != 5 || result.Spec.Sort != "updated" {
		t.Fatalf("spec = %+v", result.Spec)
	}
}

// A broken block is a 200 carrying `error`, not a 400. The note has to render;
// only the block should show a problem.
func TestNoteQueryRouteReportsBadBlocksAsData(t *testing.T) {
	s := newNoteQueryServer(t)
	for _, block := range []string{"limit: 5", "query: a\ndrop: table", "query: (unclosed"} {
		result := runBlock(t, s, block)
		if result.Error == "" {
			t.Fatalf("block %q should have reported an error", block)
		}
		if len(result.Rows) != 0 {
			t.Fatalf("block %q returned rows despite failing", block)
		}
	}
}

// The block is note content, which is untrusted. It reaches the Q1 parser and
// nothing else — there is no path from a block to SQL or to a file.
func TestNoteQueryRouteAcceptsNoSQLOrPaths(t *testing.T) {
	s := newNoteQueryServer(t)
	for _, block := range []string{
		"query: tag:todo\nsql: SELECT * FROM documents",
		"query: tag:todo\nfile: /etc/passwd",
		"query: tag:todo\nfields: title\nexec: sh",
	} {
		result := runBlock(t, s, block)
		if result.Error == "" {
			t.Fatalf("block %q was accepted", block)
		}
	}
	// A SQL-looking *query* is just text to the Q1 parser: it matches nothing
	// and changes nothing.
	injected := runBlock(t, s, "query: \"'; DROP TABLE documents; --\"")
	if injected.Error != "" && !strings.Contains(injected.Error, "invalid") {
		t.Fatalf("unexpected failure: %s", injected.Error)
	}
	after := runBlock(t, s, "query: tag:todo")
	if len(after.Rows) != 2 {
		t.Fatalf("the library changed after a hostile query block: %+v", after)
	}
}

func TestNoteQueryRouteIsReadOnly(t *testing.T) {
	s := newNoteQueryServer(t)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rr := doJSON(t, s, method, "/api/v1/note-queries/run", `{"block":"query: tag:todo"}`)
		if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
			t.Fatalf("%s should have no route, got %d", method, rr.Code)
		}
	}
	if rr := doJSON(t, s, http.MethodPost, "/api/v1/note-queries/run", "not json"); rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed request should be 400")
	}
}
