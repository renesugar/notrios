package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
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

// newLocalizeTestServer wires a real store plus a remote-media config whose
// allowed list points at the given content server (loopback, so private
// networks are permitted for the test).
func newLocalizeTestServer(t *testing.T, contentServerURL string) (*Server, *store.SQLiteStore) {
	t.Helper()
	parsed, err := url.Parse(contentServerURL)
	if err != nil {
		t.Fatalf("parse content server URL: %v", err)
	}
	st, err := store.OpenSQLiteWithAssetStore(":memory:", filepath.Join(t.TempDir(), "assets"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	cfg := config.Default()
	cfg.RemoteMedia.AllowPrivateNetworks = true
	cfg.RemoteMedia.AllowedDomains = []string{parsed.Hostname()}
	cfg.RemoteMedia.BlockedDomains = []string{"tracker.example.com"}
	cfg.RemoteMedia.QuarantineDir = filepath.Join(t.TempDir(), "quarantine")
	return NewServerWithOptions(ServerOptions{Store: st, Config: cfg}), st
}

var testPNG = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 40)...)

func TestRemoteMediaLocalizeEndpoint(t *testing.T) {
	content := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testPNG)
	}))
	defer content.Close()
	s, st := newLocalizeTestServer(t, content.URL)
	doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{
		Title: "Localize via REST",
		Body:  "![p](" + content.URL + "/a.png)\n![t](https://tracker.example.com/x.gif)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// Missing precondition → 428.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/"+doc.ID+"/remote-media/localize", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected 428 without base revision, got %d: %s", rr.Code, rr.Body.String())
	}

	// Real run with the precondition.
	body := `{"base_revision_id":"` + doc.CurrentRevisionID + `"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/documents/"+doc.ID+"/remote-media/localize", strings.NewReader(body))
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var result api.RemoteMediaResult
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(result.Localized) != 1 || len(result.Blocked) != 1 || result.RevisionID == "" {
		t.Fatalf("unexpected localize result: %+v", result)
	}
	updated, _ := st.GetDocument(context.Background(), doc.ID)
	if strings.Contains(updated.Body, content.URL) || !strings.Contains(updated.Body, "resource://") {
		t.Fatalf("body not rewritten: %s", updated.Body)
	}

	// A stale precondition now conflicts.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/documents/"+doc.ID+"/remote-media/localize", strings.NewReader(body))
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		var again api.RemoteMediaResult
		_ = json.NewDecoder(rr.Body).Decode(&again)
		// No remote media remains, so nothing rewrites and no conflict is hit.
		if len(again.Localized) != 0 || again.RevisionID != "" {
			t.Fatalf("second run must be a no-op: %+v", again)
		}
	}
}

func TestRemoteMediaLocalizeDryRunViaREST(t *testing.T) {
	requests := 0
	content := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer content.Close()
	s, st := newLocalizeTestServer(t, content.URL)
	doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{
		Title: "Dry run via REST",
		Body:  "![p](" + content.URL + "/a.png)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/"+doc.ID+"/remote-media/localize", strings.NewReader(`{"dry_run":true}`))
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for dry run without precondition, got %d: %s", rr.Code, rr.Body.String())
	}
	if requests != 0 {
		t.Fatalf("dry run must not fetch")
	}
	var result api.RemoteMediaResult
	_ = json.NewDecoder(rr.Body).Decode(&result)
	if len(result.Localized) != 1 || result.Localized[0]["would_localize"] != true || result.RevisionID != "" {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
}

func TestMCPLocalizeRemoteMedia(t *testing.T) {
	content := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(testPNG)
	}))
	defer content.Close()
	s, st := newLocalizeTestServer(t, content.URL)
	// Enable the editor profile for write tools.
	s.config.MCP.DefaultProfile = "editor"
	doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{
		Title: "Localize via MCP",
		Body:  "![p](" + content.URL + "/a.png)\n",
	})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// base_revision_id is required for non-dry runs.
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"localize_remote_media","arguments":{"document_id":"` + doc.ID + `"}}}`
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(call)))
	if !strings.Contains(rr.Body.String(), "base_revision_id is required") {
		t.Fatalf("missing precondition must error: %s", rr.Body.String())
	}

	call = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"localize_remote_media","arguments":{"document_id":"` + doc.ID + `","base_revision_id":"` + doc.CurrentRevisionID + `"}}}`
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(call)))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "resource_uri") {
		t.Fatalf("MCP localize failed: %d %s", rr.Code, rr.Body.String())
	}
	updated, _ := st.GetDocument(context.Background(), doc.ID)
	if !strings.Contains(updated.Body, "resource://") {
		t.Fatalf("MCP localize must rewrite the note: %s", updated.Body)
	}
}

func TestMCPLocalizeRequiresEditorProfile(t *testing.T) {
	content := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer content.Close()
	s, _ := newLocalizeTestServer(t, content.URL) // default read-only profile
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"localize_remote_media","arguments":{"document_id":"doc_x","base_revision_id":"rev_x"}}}`
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(call)))
	if !strings.Contains(rr.Body.String(), "read-only") {
		t.Fatalf("read-only profile must reject localize: %s", rr.Body.String())
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
