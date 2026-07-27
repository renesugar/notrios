package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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
