package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteProfileFile writes the small generated config used to start one named
// runtime profile. Values are quoted as YAML strings so paths, URLs, comments,
// and credential-store references are data rather than syntax. The file is
// owner-only because even a reference can disclose local account topology.
func WriteProfileFile(path string, cfg Config) error {
	q := strconv.Quote
	var b strings.Builder
	fmt.Fprintf(&b, "profile:\n  id: %s\n  name: %s\n  registry_path: %s\n\n", q(cfg.Profile.ID), q(cfg.Profile.Name), q(cfg.Profile.RegistryPath))
	fmt.Fprintf(&b, "server:\n  listen_addr: %s\n  public_base_url: %s\n\n", q(cfg.Server.ListenAddr), q(cfg.Server.PublicBaseURL))
	fmt.Fprintf(&b, "data:\n  directory: %s\n  database_path: %s\n  asset_store: %s\n  projection_dir: %s\n\n",
		q(cfg.Data.Directory), q(cfg.Data.DatabasePath), q(cfg.Data.AssetStore), q(cfg.Data.ProjectionDir))
	fmt.Fprintf(&b, "sync:\n  target: %s\n", q(cfg.Sync.Target))
	if cfg.Sync.Directory != "" {
		fmt.Fprintf(&b, "  directory: %s\n", q(cfg.Sync.Directory))
	}
	if cfg.Sync.RESTBaseURL != "" {
		fmt.Fprintf(&b, "  rest_base_url: %s\n", q(cfg.Sync.RESTBaseURL))
	}
	if cfg.Sync.CredentialRef != "" {
		fmt.Fprintf(&b, "  credential_ref: %s\n", q(cfg.Sync.CredentialRef))
	}
	fmt.Fprintf(&b, "\nsearch_sidecar:\n  enabled: %t\n  binary: %s\n  index_dir: %s\n\n",
		cfg.SearchSidecar.Enabled, q(cfg.SearchSidecar.Binary), q(cfg.SearchSidecar.IndexDir))
	fmt.Fprintf(&b, "remote_media:\n  quarantine_dir: %s\n", q(cfg.RemoteMedia.QuarantineDir))

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create profile config directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".profile-*.yaml")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.WriteString(b.String()); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempName, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
