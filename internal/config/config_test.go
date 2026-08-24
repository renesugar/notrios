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
	if cfg.MCP.SyncScope != "" {
		t.Fatalf("MCP sync must default disabled/empty, got %q", cfg.MCP.SyncScope)
	}
	if cfg.Retention.UnreferencedResourceDays != 30 || cfg.Retention.PurgedResourceDays != 90 {
		t.Fatalf("unexpected retention defaults: %+v", cfg.Retention)
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

mcp:
  enabled: false
  default_profile: "disabled"
  sync_scope: "control"
  max_results: 3
  max_document_bytes: 2048

search_sidecar:
  enabled: true
  binary: "recollindex-dev"
  index_dir: "` + filepath.ToSlash(filepath.Join(dir, "state", "search-index")) + `"

retention:
  unreferenced_resource_days: 7
  purged_resource_days: 45
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
	if cfg.Search.MaxLimit != 77 || cfg.Search.DefaultLimit != 7 {
		t.Fatalf("search config not loaded: %+v", cfg.Search)
	}
	if cfg.MCP.Enabled || cfg.MCP.SyncScope != "control" || cfg.MCP.MaxResults != 3 || cfg.MCP.MaxDocumentBytes != 2048 {
		t.Fatalf("mcp config not loaded: %+v", cfg.MCP)
	}
	if !cfg.SearchSidecar.Enabled || cfg.SearchSidecar.Binary != "recollindex-dev" {
		t.Fatalf("search sidecar config not loaded: %+v", cfg.SearchSidecar)
	}
	if cfg.Retention.UnreferencedResourceDays != 7 || cfg.Retention.PurgedResourceDays != 45 {
		t.Fatalf("retention config not loaded: %+v", cfg.Retention)
	}
}

func TestLoadRetentionInvalidValuesKeepSafeDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.yaml")
	contents := `retention:
  unreferenced_resource_days: -1
  purged_resource_days: "later"
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Retention.UnreferencedResourceDays != 30 || cfg.Retention.PurgedResourceDays != 90 {
		t.Fatalf("invalid retention values must keep defaults: %+v", cfg.Retention)
	}
}

func TestDefaultRemoteMediaPolicy(t *testing.T) {
	cfg := Default()
	rm := cfg.RemoteMedia
	if rm.DefaultAction != "review" {
		t.Fatalf("default action must be review (never auto-download), got %q", rm.DefaultAction)
	}
	if rm.AllowPrivateNetworks {
		t.Fatalf("private networks must be blocked by default")
	}
	if rm.MaxRedirects != 5 || rm.FetchTimeoutSeconds != 30 {
		t.Fatalf("unexpected network limits: %+v", rm)
	}
	for _, scheme := range []string{"file", "data"} {
		found := false
		for _, blocked := range rm.BlockedSchemes {
			if blocked == scheme {
				found = true
			}
		}
		if !found {
			t.Fatalf("scheme %q must be blocked by default (SECURITY_AND_MEDIA_POLICY.md)", scheme)
		}
	}
	if rm.MaxBytes["image"] != 20*1024*1024 {
		t.Fatalf("unexpected default image cap: %d", rm.MaxBytes["image"])
	}
	if rm.QuarantineDir == "" {
		t.Fatalf("quarantine dir must have a default")
	}
}

func TestLoadRemoteMediaPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.yaml")
	contents := `remote_media:
  default_action: "block"
  allow_private_networks: true
  max_redirects: 2
  fetch_timeout_seconds: 10
  quarantine_dir: "` + filepath.ToSlash(filepath.Join(dir, "q")) + `"
  blocked_schemes: ["file", "gopher"]
  blocked_domains:
    - "*.example-blocked.invalid"
    - "tracker.example.com"
  allowed_domains:
    - "wikipedia.org"
    - "*.wikimedia.org"
  review_domains:
    - "cdn.example.net"
  max_bytes:
    image: "20MB"
    video: "200MB"
    pdf: "1GB"
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rm := cfg.RemoteMedia
	if rm.DefaultAction != "block" || !rm.AllowPrivateNetworks || rm.MaxRedirects != 2 || rm.FetchTimeoutSeconds != 10 {
		t.Fatalf("scalars not loaded: %+v", rm)
	}
	if len(rm.BlockedSchemes) != 2 || rm.BlockedSchemes[0] != "file" || rm.BlockedSchemes[1] != "gopher" {
		t.Fatalf("inline list replaced defaults incorrectly: %+v", rm.BlockedSchemes)
	}
	if len(rm.BlockedDomains) != 2 || rm.BlockedDomains[0] != "*.example-blocked.invalid" {
		t.Fatalf("blocked domains not loaded: %+v", rm.BlockedDomains)
	}
	if len(rm.AllowedDomains) != 2 || rm.AllowedDomains[1] != "*.wikimedia.org" {
		t.Fatalf("allowed domains not loaded: %+v", rm.AllowedDomains)
	}
	if len(rm.ReviewDomains) != 1 || rm.ReviewDomains[0] != "cdn.example.net" {
		t.Fatalf("review domains not loaded: %+v", rm.ReviewDomains)
	}
	if rm.MaxBytes["image"] != 20<<20 || rm.MaxBytes["video"] != 200<<20 || rm.MaxBytes["pdf"] != 1<<30 {
		t.Fatalf("byte sizes not parsed: %+v", rm.MaxBytes)
	}
	if rm.QuarantineDir != filepath.ToSlash(filepath.Join(dir, "q")) {
		t.Fatalf("quarantine dir not loaded: %q", rm.QuarantineDir)
	}
}

func TestLoadRemoteMediaInvalidValuesKeepSafeDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.yaml")
	contents := `remote_media:
  default_action: "download-everything"
  max_redirects: "many"
  max_bytes:
    image: "huge"
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rm := cfg.RemoteMedia
	if rm.DefaultAction != "review" {
		t.Fatalf("invalid action must fall back to review, got %q", rm.DefaultAction)
	}
	if rm.MaxRedirects != 5 {
		t.Fatalf("invalid redirect count must keep the default, got %d", rm.MaxRedirects)
	}
	if rm.MaxBytes["image"] != 20*1024*1024 {
		t.Fatalf("invalid size must keep the default, got %d", rm.MaxBytes["image"])
	}
}

func TestLoadExampleConfigRemoteMedia(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config", "config.example.yaml"))
	if err != nil {
		t.Fatalf("Load example config: %v", err)
	}
	rm := cfg.RemoteMedia
	if rm.DefaultAction != "review" || rm.AllowPrivateNetworks {
		t.Fatalf("example policy scalars: %+v", rm)
	}
	if len(rm.BlockedSchemes) != 3 {
		t.Fatalf("example blocked_schemes: %+v", rm.BlockedSchemes)
	}
	if len(rm.BlockedDomains) != 1 || len(rm.AllowedDomains) != 4 || len(rm.ReviewDomains) != 0 {
		t.Fatalf("example domain lists: blocked=%v allowed=%v review=%v", rm.BlockedDomains, rm.AllowedDomains, rm.ReviewDomains)
	}
	if rm.MaxBytes["image"] != 20<<20 || rm.MaxBytes["video"] != 200<<20 || rm.MaxBytes["pdf"] != 100<<20 {
		t.Fatalf("example max_bytes: %+v", rm.MaxBytes)
	}
	if rm.QuarantineDir != "./data/quarantine" || rm.FetchTimeoutSeconds != 30 {
		t.Fatalf("example quarantine/timeout: %+v", rm)
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
	cfg.RemoteMedia.QuarantineDir = filepath.Join(dir, "data", "quarantine")
	if err := EnsureDirectories(cfg); err != nil {
		t.Fatalf("EnsureDirectories: %v", err)
	}
	for _, path := range []string{cfg.Data.Directory, cfg.Data.AssetStore, cfg.Data.ProjectionDir, cfg.SearchSidecar.IndexDir, cfg.RemoteMedia.QuarantineDir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected directory %s: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", path)
		}
	}
}

func TestWriteProfileFileRoundTripIsOwnerOnly(t *testing.T) {
	cfg := Default()
	root := t.TempDir()
	path := filepath.Join(root, "profiles", "profile_one.yaml")
	cfg.Profile = ProfileConfig{ID: "profile_one", Name: "Work #1", RegistryPath: filepath.Join(root, "profiles.json")}
	cfg.Server.ListenAddr = "127.0.0.1:8123"
	cfg.Server.PublicBaseURL = "http://127.0.0.1:8123"
	cfg.Data.Directory = filepath.Join(root, "data")
	cfg.Data.DatabasePath = filepath.Join(root, "data", "notes.sqlite")
	cfg.Data.AssetStore = filepath.Join(root, "data", "assets")
	cfg.Data.ProjectionDir = filepath.Join(root, "data", "projections")
	cfg.SearchSidecar.IndexDir = filepath.Join(root, "data", "search-index")
	cfg.RemoteMedia.QuarantineDir = filepath.Join(root, "data", "quarantine")
	cfg.Sync = SyncConfig{Target: "rest", RESTBaseURL: "https://sync.example.invalid/base", CredentialRef: "secret-service:notrios/work#1"}
	if err := WriteProfileFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("profile config permissions = %v, want 0600", info.Mode().Perm())
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profile != cfg.Profile || loaded.Sync != cfg.Sync || loaded.Server.ListenAddr != cfg.Server.ListenAddr {
		t.Fatalf("generated profile did not round trip: %+v", loaded)
	}
}
