package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/media"
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
	if status.SchemaVersion != 9 {
		t.Fatalf("schema version = %d, want 8", status.SchemaVersion)
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
	if status.SchemaVersion != 9 {
		t.Fatalf("schema version after re-bootstrap = %d, want 8", status.SchemaVersion)
	}
}

func TestRecordAndListMediaAttempts(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Media note", Body: "x"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	refused, err := st.RecordMediaAttempt(ctx, MediaAttempt{
		DocumentID:  doc.ID,
		OriginalURL: "https://tracker.example.com/pixel.gif",
		Decision:    "block",
		Reason:      "domain matches blocked pattern tracker.example.com",
		Status:      "refused",
	})
	if err != nil {
		t.Fatalf("RecordMediaAttempt(refused): %v", err)
	}
	if refused.ID == "" || !strings.HasPrefix(refused.ID, "mpd_") {
		t.Fatalf("attempt ID not generated: %+v", refused)
	}
	if _, err := st.RecordMediaAttempt(ctx, MediaAttempt{
		DocumentID:     doc.ID,
		OriginalURL:    "https://upload.wikimedia.org/a.png",
		FinalURL:       "https://upload.wikimedia.org/real/a.png",
		Decision:       "allow",
		Reason:         "fetched into quarantine",
		Status:         "quarantined",
		ContentType:    "image/png",
		SizeBytes:      72,
		SHA256:         "abc123",
		QuarantinePath: "/tmp/q/sha256-abc123.png",
	}); err != nil {
		t.Fatalf("RecordMediaAttempt(quarantined): %v", err)
	}
	// Document-less attempts (ad-hoc check-url style) must also record.
	if _, err := st.RecordMediaAttempt(ctx, MediaAttempt{
		OriginalURL: "https://cdn.example.net/x.png",
		Decision:    "review",
		Reason:      "review",
		Status:      "refused",
	}); err != nil {
		t.Fatalf("RecordMediaAttempt(no document): %v", err)
	}

	// Missing required fields are rejected.
	if _, err := st.RecordMediaAttempt(ctx, MediaAttempt{Status: "refused"}); err == nil {
		t.Fatalf("attempt without URL must be rejected")
	}
	if _, err := st.RecordMediaAttempt(ctx, MediaAttempt{OriginalURL: "https://x.org/a.png"}); err == nil {
		t.Fatalf("attempt without status must be rejected")
	}

	all, err := st.ListMediaAttempts(ctx, "", 100)
	if err != nil {
		t.Fatalf("ListMediaAttempts(all): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 attempts, got %d: %+v", len(all), all)
	}

	forDoc, err := st.ListMediaAttempts(ctx, doc.ID, 100)
	if err != nil {
		t.Fatalf("ListMediaAttempts(doc): %v", err)
	}
	if len(forDoc) != 2 {
		t.Fatalf("expected 2 attempts for the document, got %d", len(forDoc))
	}
	var quarantined *MediaAttempt
	for i := range forDoc {
		if forDoc[i].Status == "quarantined" {
			quarantined = &forDoc[i]
		}
	}
	if quarantined == nil {
		t.Fatalf("quarantined attempt missing: %+v", forDoc)
	}
	if quarantined.FinalURL != "https://upload.wikimedia.org/real/a.png" || quarantined.SizeBytes != 72 ||
		quarantined.SHA256 != "abc123" || quarantined.ContentType != "image/png" || quarantined.QuarantinePath == "" {
		t.Fatalf("quarantined attempt fields lost: %+v", quarantined)
	}
}

// mediaAttemptRecorder adapts the store to media.AttemptRecorder — the same
// five-line seam the H4 localization flow will use.
type mediaAttemptRecorder struct{ st *SQLiteStore }

func (r mediaAttemptRecorder) RecordMediaAttempt(ctx context.Context, attempt media.Attempt) error {
	_, err := r.st.RecordMediaAttempt(ctx, MediaAttempt{
		DocumentID:     attempt.DocumentID,
		OriginalURL:    attempt.OriginalURL,
		FinalURL:       attempt.FinalURL,
		Decision:       attempt.Decision,
		Reason:         attempt.Reason,
		Status:         attempt.Status,
		ContentType:    attempt.ContentType,
		SizeBytes:      attempt.SizeBytes,
		SHA256:         attempt.SHA256,
		QuarantinePath: attempt.QuarantinePath,
	})
	return err
}

func TestQuarantinePipelineRecordsIntoStore(t *testing.T) {
	st, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	doc, err := st.CreateDocument(ctx, CreateDocumentRequest{Title: "Quarantine target", Body: "x"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(png)
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)

	cfg := config.Default().RemoteMedia
	cfg.AllowPrivateNetworks = true // httptest listens on loopback
	cfg.AllowedDomains = []string{parsed.Hostname()}
	cfg.BlockedDomains = []string{"tracker.example.com"}
	cfg.QuarantineDir = filepath.Join(t.TempDir(), "quarantine")
	fetcher, err := media.NewFetcher(cfg, mediaAttemptRecorder{st})
	if err != nil {
		t.Fatalf("NewFetcher: %v", err)
	}

	results := fetcher.Quarantine(ctx, media.QuarantineRequest{
		DocumentID: doc.ID,
		URLs:       []string{server.URL + "/a.png", "https://tracker.example.com/pixel.gif"},
	})
	if results[0].Status != media.StatusQuarantined || results[1].Status != media.StatusRefused {
		t.Fatalf("unexpected pipeline results: %+v", results)
	}

	attempts, err := st.ListMediaAttempts(ctx, doc.ID, 10)
	if err != nil {
		t.Fatalf("ListMediaAttempts: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected 2 recorded attempts, got %d: %+v", len(attempts), attempts)
	}
	statuses := map[string]bool{}
	for _, attempt := range attempts {
		statuses[attempt.Status] = true
		if attempt.Status == "quarantined" && (attempt.SHA256 == "" || attempt.QuarantinePath == "" || attempt.SizeBytes != int64(len(png))) {
			t.Fatalf("quarantined attempt missing content facts: %+v", attempt)
		}
	}
	if !statuses["quarantined"] || !statuses["refused"] {
		t.Fatalf("both outcomes must be recorded: %+v", attempts)
	}
}

func TestSchemaUpgradeFromV6(t *testing.T) {
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
	if status.SchemaVersion != 9 {
		t.Fatalf("upgraded schema version = %d, want 8", status.SchemaVersion)
	}
	for _, table := range []string{"media_domain_rules", "media_hash_rules", "resource_hashes"} {
		if !mediaTableExists(t, st, table) {
			t.Fatalf("table %s missing after v6→current upgrade", table)
		}
	}
}
