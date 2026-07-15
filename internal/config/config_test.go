package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.Server.ListenAddr != "127.0.0.1:8080" {
		t.Fatalf("unexpected listen addr %q", cfg.Server.ListenAddr)
	}
	if cfg.Data.DatabasePath == "" || cfg.Data.AssetStore == "" || cfg.SearchSidecar.IndexDir == "" {
		t.Fatalf("default storage paths were not populated: %+v", cfg)
	}
	if !cfg.MCP.Enabled {
		t.Fatalf("MCP should be enabled by default for documented API parity")
	}
}

func TestLoadConfigOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.yaml")
	contents := `server:
  listen_addr: "127.0.0.1:9099"
  public_base_url: "http://localhost:9099"

data:
  directory: "` + filepath.ToSlash(filepath.Join(dir, "state")) + `"
  database_path: "` + filepath.ToSlash(filepath.Join(dir, "state", "custom.sqlite")) + `"
  asset_store: "` + filepath.ToSlash(filepath.Join(dir, "state", "assets")) + `"
  projection_dir: "` + filepath.ToSlash(filepath.Join(dir, "state", "projections")) + `"

search:
  default_limit: 7
  max_limit: 77
  max_offset: 777

mcp:
  enabled: false
  default_profile: "disabled"
  max_results: 3
  max_document_bytes: 2048

search_sidecar:
  enabled: true
  binary: "recollindex-dev"
  index_dir: "` + filepath.ToSlash(filepath.Join(dir, "state", "search-index")) + `"
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ConfigPath != path || cfg.Server.ListenAddr != "127.0.0.1:9099" {
		t.Fatalf("server config not loaded: %+v", cfg)
	}
	if cfg.Data.DatabasePath != filepath.ToSlash(filepath.Join(dir, "state", "custom.sqlite")) {
		t.Fatalf("data config not loaded: %+v", cfg.Data)
	}
	if cfg.Search.MaxLimit != 77 || cfg.Search.DefaultLimit != 7 || cfg.Search.MaxOffset != 777 {
		t.Fatalf("search config not loaded: %+v", cfg.Search)
	}
	if cfg.MCP.Enabled || cfg.MCP.MaxResults != 3 || cfg.MCP.MaxDocumentBytes != 2048 {
		t.Fatalf("mcp config not loaded: %+v", cfg.MCP)
	}
	if !cfg.SearchSidecar.Enabled || cfg.SearchSidecar.Binary != "recollindex-dev" {
		t.Fatalf("search sidecar config not loaded: %+v", cfg.SearchSidecar)
	}
}

func TestEnsureDirectories(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.Data.Directory = filepath.Join(dir, "data")
	cfg.Data.DatabasePath = filepath.Join(dir, "data", "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(dir, "data", "assets")
	cfg.Data.ProjectionDir = filepath.Join(dir, "data", "projections")
	cfg.SearchSidecar.IndexDir = filepath.Join(dir, "data", "search-index")
	if err := EnsureDirectories(cfg); err != nil {
		t.Fatalf("EnsureDirectories: %v", err)
	}
	for _, path := range []string{cfg.Data.Directory, cfg.Data.AssetStore, cfg.Data.ProjectionDir, cfg.SearchSidecar.IndexDir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected directory %s: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", path)
		}
	}
}
