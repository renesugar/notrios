package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

func newSelectionServer(t *testing.T) (*Server, *store.SQLiteStore) {
	t.Helper()
	st, err := store.OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	return NewServerWithStore(st), st
}

func TestSelectionPlanRESTAndMCPParityWithBoundedOutput(t *testing.T) {
	s, st := newSelectionServer(t)
	for i := 0; i < 20; i++ {
		doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{
			PreferredID: "doc_plan_" + storeTestNumber(i), Title: "private title " + storeTestNumber(i), Body: "secret body that must not enter a dry-run plan",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddDocumentTag(context.Background(), doc.ID, "transfer"); err != nil {
			t.Fatal(err)
		}
	}

	payload := `{"target":"subset_transfer","selection":{"tags":["transfer"]},"detail_limit":10,"max_documents":100}`
	rest := doJSON(t, s, http.MethodPost, "/api/v1/selection/plan", payload)
	if rest.Code != http.StatusOK {
		t.Fatalf("REST plan: %d %s", rest.Code, rest.Body.String())
	}
	var restPlan store.SelectionPlan
	if err := json.NewDecoder(rest.Body).Decode(&restPlan); err != nil {
		t.Fatal(err)
	}
	if restPlan.Counts.SelectedDocuments != 20 || len(restPlan.Documents) != 10 || !restPlan.Truncated {
		t.Fatalf("REST detail cap mismatch: %+v", restPlan)
	}
	if strings.Contains(rest.Body.String(), "secret body") || strings.Contains(rest.Body.String(), "private title") {
		t.Fatalf("REST plan leaked note content: %s", rest.Body.String())
	}

	mcpBody := `{"jsonrpc":"2.0","id":"plan","method":"tools/call","params":{"name":"plan_selection","arguments":` + payload + `}}`
	mcp := doJSON(t, s, http.MethodPost, "/mcp", mcpBody)
	if mcp.Code != http.StatusOK {
		t.Fatalf("MCP plan: %d %s", mcp.Code, mcp.Body.String())
	}
	var envelope struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Structured store.SelectionPlan `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.NewDecoder(mcp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Result.Structured.ManifestSHA256 != restPlan.ManifestSHA256 || envelope.Result.Structured.Counts != restPlan.Counts {
		t.Fatalf("REST/MCP planner divergence: rest=%+v mcp=%+v", restPlan, envelope.Result.Structured)
	}
	if len(envelope.Result.Content) != 1 || len(envelope.Result.Content[0].Text) > 64*1024 || strings.Contains(envelope.Result.Content[0].Text, "secret body") {
		t.Fatalf("MCP output is unbounded or leaked content: %d bytes", len(envelope.Result.Content[0].Text))
	}
}

func TestSelectionPlanAPIRejectsUnsafeScopeAndLimits(t *testing.T) {
	s, _ := newSelectionServer(t)
	rr := doJSON(t, s, http.MethodPost, "/api/v1/selection/plan", `{"target":"publication_handoff"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "at least one selector") {
		t.Fatalf("unscoped publication should fail: %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodPost, "/api/v1/selection/plan", `{"target":"full_archive","max_documents":100001}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "limit_too_large") {
		t.Fatalf("REST max_documents should fail: %d %s", rr.Code, rr.Body.String())
	}

	list := doJSON(t, s, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"plan_selection"`) || !strings.Contains(list.Body.String(), `"maxItems":1000`) {
		t.Fatalf("MCP planner schema missing bounded IDs: %d %s", list.Code, list.Body.String())
	}
}

func storeTestNumber(value int) string {
	return fmt.Sprintf("%02d", value)
}

func TestSelectionPlanRejectsMalformedJSON(t *testing.T) {
	s, _ := newSelectionServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/selection/plan", bytes.NewBufferString(`{"target":`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON: %d %s", rr.Code, rr.Body.String())
	}
}
