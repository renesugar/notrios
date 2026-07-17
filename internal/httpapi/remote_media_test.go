package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
)

func newRemoteMediaTestServer(t *testing.T) (*Server, *store.SQLiteStore) {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	cfg := config.Default()
	cfg.RemoteMedia.BlockedDomains = []string{"tracker.example.com"}
	cfg.RemoteMedia.AllowedDomains = []string{"*.wikimedia.org"}
	return NewServerWithOptions(ServerOptions{Store: st, Config: cfg}), st
}

func decodeScan(t *testing.T, rr *httptest.ResponseRecorder) api.RemoteMediaScanResult {
	t.Helper()
	var result api.RemoteMediaScanResult
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode scan result: %v", err)
	}
	return result
}

func TestRemoteMediaScanEndpoint(t *testing.T) {
	s, st := newRemoteMediaTestServer(t)
	doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{
		Title: "Media note",
		Body: "![ok](https://upload.wikimedia.org/a.png)\n" +
			"![tracked](https://tracker.example.com/pixel.gif)\n" +
			"![sneaky](file:///etc/passwd)\n" +
			"[paper](https://unknown.example.org/paper.pdf)\n" +
			"[page](https://unknown.example.org/page.html)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/"+doc.ID+"/remote-media/scan", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	result := decodeScan(t, rr)
	if result.DocumentID != doc.ID {
		t.Fatalf("document_id = %q, want %q", result.DocumentID, doc.ID)
	}
	if len(result.Media) != 4 {
		t.Fatalf("expected 4 decisions (page.html excluded), got %d: %+v", len(result.Media), result.Media)
	}
	if result.Counts["allow"] != 1 || result.Counts["block"] != 2 || result.Counts["review"] != 1 {
		t.Fatalf("unexpected counts: %+v", result.Counts)
	}
	for _, decision := range result.Media {
		if decision.Reason == "" || decision.MediaClass == "" {
			t.Fatalf("decision missing reason/class: %+v", decision)
		}
	}
}

func TestRemoteMediaScanUnknownDocument(t *testing.T) {
	s, _ := newRemoteMediaTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/doc_missing/remote-media/scan", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown document, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRemoteMediaScanWithExplicitURLs(t *testing.T) {
	s, _ := newRemoteMediaTestServer(t)
	body := `{"urls":["https://upload.wikimedia.org/draft.png","https://tracker.example.com/x.gif"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/doc_any/remote-media/scan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	result := decodeScan(t, rr)
	if len(result.Media) != 2 || result.Media[0].Action != "allow" || result.Media[1].Action != "block" {
		t.Fatalf("unexpected explicit-URL decisions: %+v", result.Media)
	}
}

func TestMediaPolicyEndpoints(t *testing.T) {
	s, _ := newRemoteMediaTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/media-policy", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET media-policy: expected 200, got %d", rr.Code)
	}
	var policy api.MediaPolicyStatus
	if err := json.NewDecoder(rr.Body).Decode(&policy); err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	if policy.DefaultAction != "review" || policy.BlockedDomains != 1 || policy.AllowedDomains != 1 {
		t.Fatalf("unexpected policy: %+v", policy)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/media-policy/check-url", strings.NewReader(`{"urls":["http://127.0.0.1/x.png"]}`))
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("check-url: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	result := decodeScan(t, rr)
	if len(result.Media) != 1 || result.Media[0].Action != "block" || !strings.Contains(result.Media[0].Reason, "private") {
		t.Fatalf("loopback must be blocked: %+v", result.Media)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/media-policy/check-url", strings.NewReader(`{}`))
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("check-url without urls: expected 400, got %d", rr.Code)
	}
}

func TestMCPScanRemoteMedia(t *testing.T) {
	s, st := newRemoteMediaTestServer(t)
	doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{
		Title: "MCP media note",
		Body:  "![t](https://tracker.example.com/p.gif)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(list)))
	if !strings.Contains(rr.Body.String(), "scan_remote_media") {
		t.Fatalf("tools/list must include scan_remote_media: %s", rr.Body.String())
	}

	call := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"scan_remote_media","arguments":{"document_id":"` + doc.ID + `"}}}`
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(call)))
	if rr.Code != http.StatusOK {
		t.Fatalf("tools/call: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	response := rr.Body.String()
	if !strings.Contains(response, `"action":"block"`) && !strings.Contains(response, `\"action\":\"block\"`) {
		t.Fatalf("scan_remote_media must report the block decision: %s", response)
	}
}
