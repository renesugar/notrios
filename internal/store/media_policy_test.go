package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func mediaTableExists(t *testing.T, st *SQLiteStore, name string) bool {
	t.Helper()
	st.mu.Lock()
	defer st.mu.Unlock()
	count, err := st.countLocked(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name)
	if err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return count == 1
}

func TestSchemaV7MediaPolicyTables(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	status, err := st.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.SchemaVersion != 7 {
		t.Fatalf("schema version = %d, want 7", status.SchemaVersion)
	}
	for _, table := range []string{"media_domain_rules", "media_hash_rules", "resource_hashes", "media_policy_decisions"} {
		if !mediaTableExists(t, st, table) {
			t.Fatalf("table %s missing after bootstrap", table)
		}
	}

	// The v7 shim widens media_policy_decisions with quarantine-state columns.
	if err := st.Exec(ctx, `INSERT INTO media_policy_decisions(id, original_url, decision, status, content_type, size_bytes, quarantine_path)
		VALUES('mpd_1', 'https://example.com/a.png', 'review', 'quarantined', 'image/png', 12345, '/tmp/q/a');`); err != nil {
		t.Fatalf("v7 columns missing on media_policy_decisions: %v", err)
	}

	// Action/kind CHECK constraints are part of the schema contract.
	if err := st.Exec(ctx, `INSERT INTO media_domain_rules(id, pattern, action) VALUES('mdr_bad', 'x.invalid', 'download');`); err == nil {
		t.Fatalf("invalid media_domain_rules.action must be rejected")
	} else if !strings.Contains(err.Error(), "CHECK") && !strings.Contains(err.Error(), "constraint") {
		t.Fatalf("expected constraint error, got: %v", err)
	}
	if err := st.Exec(ctx, `INSERT INTO media_hash_rules(algo, hash, kind, action) VALUES('sha256', 'abc', 'fuzzy', 'block');`); err == nil {
		t.Fatalf("invalid media_hash_rules.kind must be rejected")
	}

	// Bootstrap must stay idempotent.
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("second Bootstrap: %v", err)
	}
	status, err = st.Status(ctx)
	if err != nil {
		t.Fatalf("Status after re-bootstrap: %v", err)
	}
	if status.SchemaVersion != 7 {
		t.Fatalf("schema version after re-bootstrap = %d, want 7", status.SchemaVersion)
	}
}

func TestSchemaV7UpgradeFromV6(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.sqlite")
	ctx := context.Background()

	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	// Rewind the database to its v6 shape: drop the v7 tables and the schema
	// version marker, as a database created by a v0.2 binary would look.
	rewind := []string{
		`DROP TABLE media_domain_rules;`,
		`DROP TABLE media_hash_rules;`,
		`DROP TABLE resource_hashes;`,
		`PRAGMA user_version = 6;`,
	}
	for _, statement := range rewind {
		if err := st.Exec(ctx, statement); err != nil {
			t.Fatalf("rewind %q: %v", statement, err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	st, err = OpenSQLite(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st.Close()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap after rewind: %v", err)
	}
	status, err := st.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.SchemaVersion != 7 {
		t.Fatalf("upgraded schema version = %d, want 7", status.SchemaVersion)
	}
	for _, table := range []string{"media_domain_rules", "media_hash_rules", "resource_hashes"} {
		if !mediaTableExists(t, st, table) {
			t.Fatalf("table %s missing after v6→v7 upgrade", table)
		}
	}
}
