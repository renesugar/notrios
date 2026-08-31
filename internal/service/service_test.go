package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
)

func TestUnavailableSidecarReportsStateAndFTSServiceStarts(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Data.Directory = dir
	cfg.Data.DatabasePath = filepath.Join(dir, "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(dir, "assets")
	cfg.Data.ProjectionDir = filepath.Join(dir, "projections")
	cfg.SearchSidecar.Enabled = true
	cfg.SearchSidecar.Binary = filepath.Join(dir, "missing-recollindex")
	cfg.SearchSidecar.IndexDir = filepath.Join(dir, "search-index")

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	response := httptest.NewRecorder()
	svc.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status: %d %s", response.Code, response.Body.String())
	}
	var status api.StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if !status.SearchSidecar.Configured || status.SearchSidecar.Available ||
		status.SearchSidecar.Active || status.SearchSidecar.State != "unavailable" {
		t.Fatalf("unexpected unavailable sidecar status: %+v", status.SearchSidecar)
	}
	if !status.Capabilities["search.fts5"] {
		t.Fatalf("FTS5 must remain available: %+v", status.Capabilities)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("idempotent Close: %v", err)
	}
}

func TestNonNoneSyncTargetEstablishesLocalJournalBoundary(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Data.Directory = dir
	cfg.Data.DatabasePath = filepath.Join(dir, "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(dir, "assets")
	cfg.Data.ProjectionDir = filepath.Join(dir, "projections")
	cfg.Sync.Target = "directory"
	cfg.Sync.Directory = filepath.Join(dir, "carrier")

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	status, err := svc.Store.JournalStatus(context.Background())
	if err != nil {
		t.Fatalf("JournalStatus: %v", err)
	}
	if !status.Enabled || status.LastSequence != 0 || status.SnapshotBoundaryID == "" {
		t.Fatalf("non-none target did not establish boundary: %+v", status)
	}
	doc, err := svc.Store.CreateDocument(context.Background(), store.CreateDocumentRequest{Title: "after enrollment", Body: "journaled"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	operations, err := svc.Store.ListLocalOperations(context.Background(), 0, 10)
	if err != nil || len(operations) != 2 || operations[0].RecordID != doc.ID {
		t.Fatalf("operations=%+v err=%v", operations, err)
	}
}

func TestNonLoopbackListenerExposesOnlyPeerSyncToRemoteClients(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := restrictRemoteRequests(next)

	for _, testCase := range []struct {
		name       string
		path       string
		remoteAddr string
		forwarded  string
		want       int
	}{
		{name: "remote ordinary REST", path: "/api/v1/documents/doc", remoteAddr: "192.0.2.8:42000", want: http.StatusForbidden},
		{name: "remote MCP", path: "/mcp", remoteAddr: "192.0.2.8:42000", want: http.StatusForbidden},
		{name: "remote web", path: "/", remoteAddr: "192.0.2.8:42000", want: http.StatusForbidden},
		{name: "remote local sync UI", path: "/api/v1/sync-ui", remoteAddr: "192.0.2.8:42000", want: http.StatusForbidden},
		{name: "remote peer sync", path: "/api/v1/sync/handshake", remoteAddr: "192.0.2.8:42000", want: http.StatusNoContent},
		{name: "unknown sync prefix does not reach web fallback", path: "/api/v1/sync/not-a-route", remoteAddr: "192.0.2.8:42000", want: http.StatusNotFound},
		{name: "loopback ordinary REST", path: "/api/v1/documents/doc", remoteAddr: "127.0.0.1:42000", want: http.StatusNoContent},
		{name: "DNS rebinding host", path: "/api/v1/documents/doc", remoteAddr: "127.0.0.1:42000", forwarded: "host=evil.example", want: http.StatusForbidden},
		{name: "forwarded header cannot forge loopback", path: "/api/v1/documents/doc", remoteAddr: "192.0.2.8:42000", forwarded: "127.0.0.1", want: http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			request.RemoteAddr = testCase.remoteAddr
			request.Host = "127.0.0.1:8443"
			if strings.HasPrefix(testCase.forwarded, "host=") {
				request.Host = strings.TrimPrefix(testCase.forwarded, "host=")
			} else {
				request.Header.Set("X-Forwarded-For", testCase.forwarded)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.want, response.Body.String())
			}
		})
	}
}

func TestLoopbackListenerKeepsTheOrdinaryAPI(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := restrictRemoteRequests(next)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/documents/doc", nil)
	request.RemoteAddr = "127.0.0.1:42000"
	request.Host = "localhost:8080"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("loopback listener changed ordinary API behavior: %d", response.Code)
	}
}

type servingHTTPServerProbe struct {
	plain int
	tls   int
	cert  string
	key   string
}

func (p *servingHTTPServerProbe) ListenAndServe() error {
	p.plain++
	return nil
}

func (p *servingHTTPServerProbe) ListenAndServeTLS(certificate, key string) error {
	p.tls++
	p.cert, p.key = certificate, key
	return nil
}

func TestSharedServingPathConsumesTLSConfiguration(t *testing.T) {
	tlsProbe := &servingHTTPServerProbe{}
	if err := listenAndServe(tlsProbe, "/cert.pem", "/key.pem"); err != nil {
		t.Fatal(err)
	}
	if tlsProbe.tls != 1 || tlsProbe.plain != 0 || tlsProbe.cert != "/cert.pem" || tlsProbe.key != "/key.pem" {
		t.Fatalf("TLS serving selection = %+v", tlsProbe)
	}
	plainProbe := &servingHTTPServerProbe{}
	if err := listenAndServe(plainProbe, "", ""); err != nil {
		t.Fatal(err)
	}
	if plainProbe.plain != 1 || plainProbe.tls != 0 {
		t.Fatalf("plaintext loopback selection = %+v", plainProbe)
	}
	partialProbe := &servingHTTPServerProbe{}
	if err := listenAndServe(partialProbe, "", "/key.pem"); err == nil {
		t.Fatal("a lone TLS key selected plaintext serving")
	}
	if partialProbe.plain != 0 || partialProbe.tls != 0 {
		t.Fatalf("partial TLS configuration started a listener: %+v", partialProbe)
	}
	spaceProbe := &servingHTTPServerProbe{}
	if err := listenAndServe(spaceProbe, "  ", "\t"); err != nil {
		t.Fatal(err)
	}
	if spaceProbe.plain != 1 || spaceProbe.tls != 0 {
		t.Fatalf("whitespace-only TLS configuration did not select plaintext: %+v", spaceProbe)
	}
}

func TestLiveSidecarStartupRepairsDamageAndReportsStatus(t *testing.T) {
	if _, err := exec.LookPath("recollindex"); err != nil {
		t.Skip("recollindex not installed")
	}
	if _, err := exec.LookPath("recollq"); err != nil {
		t.Skip("recollq not installed")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg-config"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(dir, "xdg-runtime"))
	if err := os.MkdirAll(os.Getenv("XDG_RUNTIME_DIR"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Data.Directory = dir
	cfg.Data.DatabasePath = filepath.Join(dir, "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(dir, "assets")
	cfg.Data.ProjectionDir = filepath.Join(dir, "projections")
	cfg.SearchSidecar.Enabled = true
	cfg.SearchSidecar.Binary = "recollindex"
	cfg.SearchSidecar.IndexDir = filepath.Join(dir, "search-index")

	st, err := store.OpenSQLite(cfg.Data.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	doc, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{
		PreferredID: "doc_live_reconcile", Title: "Live reconciliation", Body: "native sidecar token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	notesDir := filepath.Join(cfg.Data.ProjectionDir, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notesDir, doc.ID+".md"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notesDir, "orphan.md"), []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer svc.Close()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	response := httptest.NewRecorder()
	svc.Handler.ServeHTTP(response, request)
	var status api.StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	reconciliation := status.SearchSidecar.Reconciliation
	if response.Code != http.StatusOK || !status.SearchSidecar.Active ||
		reconciliation == nil || !reconciliation.Complete ||
		reconciliation.Stale != 1 || reconciliation.Orphaned != 1 ||
		reconciliation.Repaired != 2 || status.SearchSidecar.LastIndexAt == "" {
		t.Fatalf("live sidecar status: code=%d status=%+v", response.Code, status.SearchSidecar)
	}
	if _, err := os.Stat(filepath.Join(notesDir, "orphan.md")); !os.IsNotExist(err) {
		t.Fatalf("orphan survived startup reconciliation: %v", err)
	}
}
