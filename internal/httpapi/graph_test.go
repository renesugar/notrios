package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// newGraphServer builds the same chain the store fixtures use: a -> b -> c,
// with an island nothing links to.
func newGraphServer(t *testing.T) *Server {
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
	uri := func(id string) string { return store.DocumentURI("default", id) }
	for _, entry := range []struct{ id, body string }{
		{"gr_island", "alone\n"},
		{"gr_c", "the end\n"},
		{"gr_b", "[next](" + uri("gr_c") + ")\n"},
		{"gr_a", "[next](" + uri("gr_b") + ")\n"},
	} {
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: entry.id, Title: entry.id, Body: entry.body,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"gr_a", "gr_b", "gr_c"} {
		if err := st.RebuildDocumentLinks(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	return NewServerWithStore(st)
}

func TestGraphRouteHonoursDepthAndReportsItBack(t *testing.T) {
	s := newGraphServer(t)

	var shallow api.GraphResponse
	rr := doJSON(t, s, http.MethodPost, "/api/v1/graph", `{"roots":["gr_a"],"direction":"outgoing","depth":1}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &shallow); err != nil {
		t.Fatal(err)
	}
	if len(shallow.Nodes) != 2 || shallow.RequestedDepth != 1 || shallow.CompletedDepth != 1 {
		t.Fatalf("depth 1 = %+v", shallow)
	}

	var deep api.GraphResponse
	rr = doJSON(t, s, http.MethodPost, "/api/v1/graph", `{"roots":["gr_a"],"direction":"outgoing","depth":2}`)
	if err := json.Unmarshal(rr.Body.Bytes(), &deep); err != nil {
		t.Fatal(err)
	}
	if len(deep.Nodes) != 3 {
		t.Fatalf("depth 2 should reach the third note: %+v", deep)
	}
	// The response is typed now, so a client can rely on `depth` being present.
	last := deep.Nodes[len(deep.Nodes)-1]
	if last.ID != "gr_c" || last.Depth != 2 {
		t.Fatalf("last node = %+v, want gr_c at depth 2", last)
	}
}

func TestGraphRouteRefusesRequestsOverTheCeilings(t *testing.T) {
	s := newGraphServer(t)
	for name, body := range map[string]string{
		"depth":     `{"roots":["gr_a"],"depth":99}`,
		"nodes":     `{"roots":["gr_a"],"max_nodes":999999}`,
		"edges":     `{"roots":["gr_a"],"max_edges":999999}`,
		"direction": `{"roots":["gr_a"],"direction":"sideways"}`,
	} {
		rr := doJSON(t, s, http.MethodPost, "/api/v1/graph", body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: status=%d body=%s, want 400", name, rr.Code, rr.Body.String())
		}
	}
}

func TestGraphPathRoute(t *testing.T) {
	s := newGraphServer(t)

	var found api.GraphPathResponse
	rr := doJSON(t, s, http.MethodPost, "/api/v1/graph/path", `{"from":"gr_a","to":"gr_c","direction":"outgoing"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &found); err != nil {
		t.Fatal(err)
	}
	if found.Status != store.GraphPathFound || found.Length != 2 || len(found.Nodes) != 3 {
		t.Fatalf("path = %+v", found)
	}

	var none api.GraphPathResponse
	rr = doJSON(t, s, http.MethodPost, "/api/v1/graph/path", `{"from":"gr_a","to":"gr_island","direction":"both"}`)
	if err := json.Unmarshal(rr.Body.Bytes(), &none); err != nil {
		t.Fatal(err)
	}
	if none.Status != store.GraphPathNoPath || len(none.Nodes) != 0 {
		t.Fatalf("unreachable path = %+v", none)
	}

	if rr := doJSON(t, s, http.MethodPost, "/api/v1/graph/path", `{"from":"gr_a","to":"gr_missing"}`); rr.Code != http.StatusNotFound {
		t.Fatalf("missing endpoint status=%d, want 404", rr.Code)
	}
	if rr := doJSON(t, s, http.MethodPost, "/api/v1/graph/path", `{"from":"gr_a"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("missing target status=%d, want 400", rr.Code)
	}
}

func TestGraphReportRoute(t *testing.T) {
	s := newGraphServer(t)

	var report api.GraphReport
	rr := doJSON(t, s, http.MethodGet, "/api/v1/graph/report", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.DocumentCount == 0 || report.LinkCount != 2 {
		t.Fatalf("report = %+v, want the two chain links", report)
	}
	isolated := map[string]bool{}
	for _, entry := range report.Isolated {
		isolated[entry.DocumentID] = true
	}
	if !isolated["gr_island"] {
		t.Fatalf("gr_island is isolated: %+v", report.Isolated)
	}

	if rr := doJSON(t, s, http.MethodGet, "/api/v1/graph/report?limit=0", ""); rr.Code != http.StatusBadRequest {
		t.Fatalf("limit=0 status=%d, want 400", rr.Code)
	}
	if rr := doJSON(t, s, http.MethodGet, "/api/v1/graph/report?limit=100000", ""); rr.Code != http.StatusBadRequest {
		t.Fatalf("over-ceiling limit status=%d, want 400", rr.Code)
	}
}

// Traversal is read-only over REST for the same reason lint is: there is no
// apply surface to accidentally reach.
func TestGraphRoutesAreReadOnly(t *testing.T) {
	s := newGraphServer(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rr := doJSON(t, s, method, "/api/v1/graph/report", `{}`)
		if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
			t.Fatalf("%s /api/v1/graph/report status=%d, want no route", method, rr.Code)
		}
	}
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rr := doJSON(t, s, method, "/api/v1/graph/path", `{}`)
		if rr.Code != http.StatusMethodNotAllowed && rr.Code != http.StatusNotFound {
			t.Fatalf("%s /api/v1/graph/path status=%d, want no route", method, rr.Code)
		}
	}
}
