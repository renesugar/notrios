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

func newEditorLinkServer(t *testing.T) *Server {
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
		{"eh_kitchen", "Kitchen", "# Kitchen\n\nThe room.\n"},
		{"eh_kitchenplan", "Kitchen Plan", "# Kitchen Plan\n\nSteps.\n"},
	} {
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: entry.id, Title: entry.title, Body: entry.body,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return NewServerWithStore(st)
}

func TestSuggestRouteReturnsIdsAndTitlesOnly(t *testing.T) {
	s := newEditorLinkServer(t)

	rr := doJSON(t, s, http.MethodGet, "/api/v1/links/suggest?q=kit", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp api.DocumentSuggestionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Suggestions) != 2 || resp.Suggestions[0].DocumentID != "eh_kitchen" {
		t.Fatalf("suggestions = %+v", resp.Suggestions)
	}
	if resp.Suggestions[0].URI == "" || resp.Suggestions[0].Match == "" {
		t.Fatalf("a suggestion carries a URI and a match kind: %+v", resp.Suggestions[0])
	}
	// A suggestion is an ID and a title. A body reaching an autocomplete
	// dropdown would put note content somewhere it was never asked for.
	if strings.Contains(rr.Body.String(), "The room") {
		t.Fatalf("suggestion response leaked note body: %s", rr.Body.String())
	}
}

func TestSuggestRouteValidatesItsBounds(t *testing.T) {
	s := newEditorLinkServer(t)
	for name, path := range map[string]string{
		"too short": "/api/v1/links/suggest?q=k",
		"missing":   "/api/v1/links/suggest",
		"limit":     "/api/v1/links/suggest?q=kit&limit=9999",
		"zero":      "/api/v1/links/suggest?q=kit&limit=0",
	} {
		if rr := doJSON(t, s, http.MethodGet, path, ""); rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: status=%d, want 400", name, rr.Code)
		}
	}
}

func TestCheckLinksRouteClassifiesABufferWithoutSaving(t *testing.T) {
	s := newEditorLinkServer(t)

	body := `{"document_id":"eh_kitchen","body":"[a](Kitchen Plan)\n\n[b](document://default/documents/eh_missing)\n"}`
	rr := doJSON(t, s, http.MethodPost, "/api/v1/links/check", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp api.CheckLinksResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 || resp.Unresolved != 1 {
		t.Fatalf("counts = %+v", resp)
	}
	// `[a](Kitchen Plan)` truncates at the space — the Markdown rule E1a and E3
	// both met — so the raw target is "Kitchen" and it resolves by title.
	if resp.Links[0].Status != "resolved" || resp.Links[0].CanonicalTarget == "" {
		t.Fatalf("first link = %+v, want a resolved title link offering its canonical URI", resp.Links[0])
	}
	if resp.Links[1].Status != "unresolved" || resp.Links[1].Line != 3 {
		t.Fatalf("second link = %+v, want an unresolved link located on line 3", resp.Links[1])
	}

	// Nothing was written: the note still reads what it always did.
	get := doJSON(t, s, http.MethodGet, "/api/v1/documents/eh_kitchen", "")
	if !strings.Contains(get.Body.String(), "The room") {
		t.Fatalf("checking a buffer changed the note: %s", get.Body.String())
	}
	revisions := doJSON(t, s, http.MethodGet, "/api/v1/documents/eh_kitchen/revisions", "")
	if strings.Count(revisions.Body.String(), `"id"`) != 1 {
		t.Fatalf("checking a buffer wrote a revision: %s", revisions.Body.String())
	}
}

func TestCheckLinksRouteRefusesAnOversizedBuffer(t *testing.T) {
	s := newEditorLinkServer(t)
	huge, err := json.Marshal(map[string]string{"body": strings.Repeat("x", store.MaxCheckBodyBytes+1)})
	if err != nil {
		t.Fatal(err)
	}
	if rr := doJSON(t, s, http.MethodPost, "/api/v1/links/check", string(huge)); rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rr.Code)
	}
	if rr := doJSON(t, s, http.MethodPost, "/api/v1/links/check", "not json"); rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed body status=%d, want 400", rr.Code)
	}
}

// Both surfaces are read-only. There is no write method on either.
func TestEditorLinkRoutesAreReadOnly(t *testing.T) {
	s := newEditorLinkServer(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rr := doJSON(t, s, method, "/api/v1/links/suggest?q=kit", `{}`)
		if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
			t.Fatalf("%s /api/v1/links/suggest status=%d, want no route", method, rr.Code)
		}
	}
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rr := doJSON(t, s, method, "/api/v1/links/check", `{}`)
		if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
			t.Fatalf("%s /api/v1/links/check status=%d, want no route", method, rr.Code)
		}
	}
}
