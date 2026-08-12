package store

/*
#cgo pkg-config: sqlite3
#include <sqlite3.h>
#include <stdlib.h>

static int notes_sqlite_bind_text(sqlite3_stmt *stmt, int idx, char *value) {
	return sqlite3_bind_text(stmt, idx, value, -1, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/renesugar/notrios/internal/markdownlinks"
	"github.com/renesugar/notrios/internal/stablelink"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// SQLiteStore is a small cgo-backed SQLite adapter. It is intentionally narrow
// until the project can decide whether to use mattn/go-sqlite3, modernc.org/sqlite,
// or this local wrapper long term.
type SQLiteStore struct {
	mu        sync.Mutex
	db        *C.sqlite3
	path      string
	assetRoot string
	// cachedDatabaseID memoizes the logical database ID. Link resolution
	// consults it once per link in a batch import, and the value changes only
	// through the explicit identity operations, which clear it.
	cachedDatabaseID   string
	perceptualHashHook PerceptualHashHook
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	return OpenSQLiteWithAssetStore(path, defaultAssetRoot(path))
}

func OpenSQLiteWithAssetStore(path, assetRoot string) (*SQLiteStore, error) {
	if strings.TrimSpace(assetRoot) == "" {
		assetRoot = defaultAssetRoot(path)
	}
	if path != ":memory:" {
		if err := os.MkdirAll(parentDir(path), 0o755); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(assetRoot, 0o755); err != nil {
		return nil, err
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var db *C.sqlite3
	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_FULLMUTEX)
	if rc := C.sqlite3_open_v2(cpath, &db, flags, nil); rc != C.SQLITE_OK {
		msg := C.GoString(C.sqlite3_errmsg(db))
		if db != nil {
			C.sqlite3_close(db)
		}
		return nil, fmt.Errorf("open sqlite: %s", msg)
	}
	s := &SQLiteStore{db: db, path: path, assetRoot: assetRoot}
	// busy_timeout lets a CLI import and the running service share the file
	// without immediate "database is locked" failures on write overlap.
	if err := s.exec("PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;"); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func defaultAssetRoot(path string) string {
	if path == ":memory:" || strings.TrimSpace(path) == "" {
		return filepath.Join(os.TempDir(), "notrios-assets")
	}
	return filepath.Join(parentDir(path), "assets")
}

func parentDir(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "."
	}
	if idx == 0 {
		return "/"
	}
	return path[:idx]
}

func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	if rc := C.sqlite3_close(s.db); rc != C.SQLITE_OK {
		return fmt.Errorf("close sqlite: %s", C.GoString(C.sqlite3_errmsg(s.db)))
	}
	s.db = nil
	return nil
}

func (s *SQLiteStore) Bootstrap(ctx context.Context) error {
	ctx = contextOrBackground(ctx)
	migration, err := migrationFS.ReadFile("migrations/0001_initial.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if err := s.Exec(ctx, string(migration)); err != nil {
		return err
	}
	if err := s.ensureSchemaV4(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV5(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV6(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV7(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV8(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV9(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV10(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV11(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV12(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV13(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV14(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV15(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV16(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV17(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV18(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV19(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV20(ctx); err != nil {
		return err
	}
	if err := s.ensureSchemaV21(ctx); err != nil {
		return err
	}
	if err := s.ensureDatabaseIdentity(ctx); err != nil {
		return err
	}
	if err := s.Exec(ctx, `INSERT OR IGNORE INTO collections(id, name, description) VALUES('default', 'Default', 'Managed notes created by the companion service.');`); err != nil {
		return err
	}
	if err := s.seedNotebooks(ctx); err != nil {
		return err
	}
	return s.ensureSyncMetadataBaseline(ctx)
}

func (s *SQLiteStore) seedNotebooks(ctx context.Context) error {
	statements := []string{
		`INSERT OR IGNORE INTO notebooks(id, parent_id, name, icon_emoji, builtin) VALUES('` + DefaultNotebookID + `', NULL, 'Notes', '', 0);`,
		`INSERT OR IGNORE INTO notebooks(id, parent_id, name, icon_emoji, builtin) VALUES('` + ReportsNotebookID + `', NULL, 'Reports', '', 1);`,
		`INSERT OR IGNORE INTO notebooks(id, parent_id, name, icon_emoji, builtin) VALUES('` + HelpNotebookID + `', NULL, 'Help', '', 1);`,
		`INSERT OR IGNORE INTO notebooks(id, parent_id, name, icon_emoji, builtin) VALUES('` + RecoveredNotebookID + `', NULL, 'Recovered', '', 1);`,
		`INSERT OR IGNORE INTO search_notebooks(id, name, icon_emoji, query, builtin, sort_anchor) VALUES('` + AllNotesSearchNotebookID + `', 'All notes', '', '', 1, 'first');`,
		`INSERT OR IGNORE INTO search_notebooks(id, name, icon_emoji, query, builtin, sort_anchor) VALUES('` + TrashSearchNotebookID + `', 'Trash', '', 'is:trashed', 1, 'last');`,
		`UPDATE documents SET notebook_id = '` + DefaultNotebookID + `' WHERE notebook_id IS NULL;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) Exec(ctx context.Context, sql string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.exec(sql)
}

func (s *SQLiteStore) ensureSchemaV4(ctx context.Context) error {
	statements := []string{
		`ALTER TABLE document_revisions ADD COLUMN body_mime_type TEXT NOT NULL DEFAULT 'text/markdown';`,
		`ALTER TABLE document_revisions ADD COLUMN message TEXT;`,
		`CREATE INDEX IF NOT EXISTS resources_collection_idx ON resources(collection_id);`,
		`CREATE INDEX IF NOT EXISTS resources_blob_idx ON resources(blob_sha256);`,
		`CREATE INDEX IF NOT EXISTS document_resource_refs_document_idx ON document_resource_refs(document_id);`,
		`CREATE INDEX IF NOT EXISTS document_resource_refs_resource_idx ON document_resource_refs(resource_id);`,
		`ALTER TABLE document_links ADD COLUMN target_uri TEXT;`,
		`ALTER TABLE document_links ADD COLUMN context TEXT;`,
		`PRAGMA user_version = 4;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

// ensureSchemaV5 upgrades pre-notebook databases: the migration file creates
// the v5 tables with IF NOT EXISTS, but the documents.notebook_id column and
// its index only exist on fresh databases.
func (s *SQLiteStore) ensureSchemaV5(ctx context.Context) error {
	statements := []string{
		`ALTER TABLE documents ADD COLUMN notebook_id TEXT REFERENCES notebooks(id);`,
		`CREATE INDEX IF NOT EXISTS documents_notebook_idx ON documents(notebook_id);`,
		`PRAGMA user_version = 5;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

// ensureSchemaV6 records the provenance-table schema version. The
// document_sources table itself is created by the migration file (IF NOT
// EXISTS covers pre-v6 databases); the shim only has to bump the version
// because the earlier shims reset it during bootstrap.
func (s *SQLiteStore) ensureSchemaV6(ctx context.Context) error {
	return s.Exec(ctx, `PRAGMA user_version = 6;`)
}

// ensureSchemaV7 upgrades pre-v7 databases for media-policy hardening
// (v0.3 task H1). The rule/hash tables come from the migration file (IF NOT
// EXISTS); this shim widens media_policy_decisions with quarantine-state
// columns, which fresh databases receive here too since ALTERs cannot be
// idempotent inside the migration file.
func (s *SQLiteStore) ensureSchemaV7(ctx context.Context) error {
	statements := []string{
		`ALTER TABLE media_policy_decisions ADD COLUMN status TEXT NOT NULL DEFAULT 'recorded';`,
		`ALTER TABLE media_policy_decisions ADD COLUMN content_type TEXT;`,
		`ALTER TABLE media_policy_decisions ADD COLUMN size_bytes INTEGER;`,
		`ALTER TABLE media_policy_decisions ADD COLUMN quarantine_path TEXT;`,
		`ALTER TABLE media_policy_decisions ADD COLUMN updated_at TEXT;`,
		`CREATE INDEX IF NOT EXISTS media_policy_decisions_document_idx ON media_policy_decisions(document_id);`,
		`CREATE INDEX IF NOT EXISTS media_policy_decisions_url_idx ON media_policy_decisions(original_url);`,
		`PRAGMA user_version = 7;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

// ensureSchemaV8 adds retention state for logical resources. Existing
// unreferenced resources start their retention clock at their original
// creation time; referenced resources remain explicitly ineligible.
func (s *SQLiteStore) ensureSchemaV8(ctx context.Context) error {
	statements := []string{
		`ALTER TABLE resources ADD COLUMN unreferenced_at TEXT;`,
		`ALTER TABLE resources ADD COLUMN unreferenced_reason TEXT NOT NULL DEFAULT '';`,
		`CREATE INDEX IF NOT EXISTS resources_unreferenced_idx ON resources(unreferenced_at);`,
		`UPDATE resources
		 SET unreferenced_at = COALESCE(unreferenced_at, created_at),
		     unreferenced_reason = CASE WHEN unreferenced_reason = '' THEN 'legacy_unreferenced' ELSE unreferenced_reason END
		 WHERE NOT EXISTS (SELECT 1 FROM document_resource_refs rr WHERE rr.resource_id = resources.id);`,
		`UPDATE resources
		 SET unreferenced_at = NULL, unreferenced_reason = ''
		 WHERE EXISTS (SELECT 1 FROM document_resource_refs rr WHERE rr.resource_id = resources.id);`,
		`PRAGMA user_version = 8;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

// ensureSchemaV9 adds the composite indexes used by H7 keyset traversal.
func (s *SQLiteStore) ensureSchemaV9(ctx context.Context) error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS documents_collection_state_updated_idx
			ON documents(collection_id, deleted_at, updated_at DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS documents_notebook_state_updated_idx
			ON documents(notebook_id, deleted_at, updated_at DESC, id DESC);`,
		`CREATE INDEX IF NOT EXISTS documents_trash_deleted_idx
			ON documents(deleted_at DESC, id DESC)
			WHERE deleted_at IS NOT NULL;`,
		`CREATE INDEX IF NOT EXISTS note_tags_tag_document_idx
			ON note_tags(tag_id, document_id);`,
		`PRAGMA user_version = 9;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchemaV10 adds durable, input-scoped importer checkpoints and exact
// source-bundle manifests. Source bytes are stored beneath the asset root.
func (s *SQLiteStore) ensureSchemaV10(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS import_checkpoints (
			source_system TEXT NOT NULL,
			source_key TEXT NOT NULL,
			collection_id TEXT NOT NULL,
			inventory_fingerprint TEXT NOT NULL,
			phase TEXT NOT NULL,
			next_index INTEGER NOT NULL DEFAULT 0,
			total_items INTEGER NOT NULL DEFAULT 0,
			processed_items INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			report_json TEXT NOT NULL DEFAULT '{}',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at TEXT,
			PRIMARY KEY(source_system, source_key, collection_id)
		);`,
		`CREATE TABLE IF NOT EXISTS import_item_states (
			source_system TEXT NOT NULL,
			source_key TEXT NOT NULL,
			collection_id TEXT NOT NULL,
			item_key TEXT NOT NULL,
			item_type TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			target_id TEXT,
			action TEXT NOT NULL,
			processed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(source_system, source_key, collection_id, item_key)
		);`,
		`CREATE INDEX IF NOT EXISTS import_item_states_type_idx
			ON import_item_states(source_system, source_key, collection_id, item_type, item_key);`,
		`CREATE TABLE IF NOT EXISTS source_bundle_items (
			source_system TEXT NOT NULL,
			source_key TEXT NOT NULL,
			collection_id TEXT NOT NULL,
			item_key TEXT NOT NULL,
			item_type TEXT NOT NULL,
			external_id TEXT NOT NULL,
			relative_path TEXT NOT NULL,
			sha256 TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			storage_path TEXT NOT NULL,
			property_order_json TEXT NOT NULL DEFAULT '[]',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(source_system, source_key, collection_id, item_key)
		);`,
		`CREATE INDEX IF NOT EXISTS source_bundle_items_hash_idx
			ON source_bundle_items(sha256);`,
		`PRAGMA user_version = 10;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchemaV11 adds durable exponential retry scheduling for projection
// jobs. The projection and Recoll index remain reconstructible derived state.
func (s *SQLiteStore) ensureSchemaV11(ctx context.Context) error {
	statements := []string{
		`ALTER TABLE index_outbox ADD COLUMN next_attempt_at TEXT;`,
		`CREATE INDEX IF NOT EXISTS index_outbox_pending_idx
			ON index_outbox(completed_at, next_attempt_at, sequence);`,
		`PRAGMA user_version = 11;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

// ensureSchemaV12 adds the persisted identity row used by native archive v2
// and later stable links/synchronization. The table is canonical state, not a
// path-, host-, or projection-derived identifier.
func (s *SQLiteStore) ensureSchemaV12(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS database_identity (
			singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
			database_id TEXT NOT NULL UNIQUE,
			replica_id TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			replica_created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`PRAGMA user_version = 12;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchemaV13 adds the restore-in-progress marker. A restore commits many
// transactions, so an interruption leaves committed rows behind; this row is
// what stops that partial library from being mistaken for a complete one.
func (s *SQLiteStore) ensureSchemaV13(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS restore_state (
			singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
			snapshot_id TEXT NOT NULL,
			commit_sha256 TEXT NOT NULL,
			intent TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`PRAGMA user_version = 13;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchemaV14 adds content-addressed note blocks. Block rows are derived
// state: they are rebuilt from the note body in the same transaction as the
// save that produced them, so an upgrade needs no backfill — the first save of
// each note fills them in, and `RebuildDocumentBlocks` fills them in for notes
// nobody edits.
func (s *SQLiteStore) ensureSchemaV14(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS document_blocks (
			id TEXT NOT NULL,
			document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			ordinal INTEGER NOT NULL,
			kind TEXT NOT NULL,
			heading_level INTEGER NOT NULL DEFAULT 0,
			marker TEXT,
			content_sha256 TEXT NOT NULL,
			start_byte INTEGER NOT NULL,
			end_byte INTEGER NOT NULL,
			PRIMARY KEY (document_id, id)
		);`,
		`CREATE INDEX IF NOT EXISTS document_blocks_document_idx ON document_blocks(document_id, ordinal);`,
		`CREATE INDEX IF NOT EXISTS document_blocks_marker_idx ON document_blocks(document_id, marker) WHERE marker IS NOT NULL;`,
		`PRAGMA user_version = 14;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchemaV15 adds the heading slug a `#section-title` anchor resolves
// against. Block rows deliberately store a content hash rather than heading
// text, so before this column a heading anchor had nothing to compare against.
// Like every block column it is derived state: a save fills it in, and
// RebuildDocumentBlocks fills it in for notes nobody edits.
func (s *SQLiteStore) ensureSchemaV15(ctx context.Context) error {
	statements := []string{
		`ALTER TABLE document_blocks ADD COLUMN heading_slug TEXT;`,
		`CREATE INDEX IF NOT EXISTS document_blocks_slug_idx ON document_blocks(document_id, heading_slug) WHERE heading_slug IS NOT NULL;`,
		`PRAGMA user_version = 15;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}

// ensureSchemaV16 adds the title and filename indexes editor link intelligence
// needs.
//
// Resolving a link by title ran `lower(title) = lower(?)`, which no index can
// serve: every link that did not name a URI cost a full scan of the document
// table, once per link, on every save and every lint pass. The comparison moves
// to the NOCASE collation — identical semantics, since SQLite's `lower()` folds
// ASCII only, exactly as NOCASE does — so the same lookup becomes an index
// probe. The collation additionally makes `title LIKE 'prefix%'` a range scan,
// which is what lets the suggestion endpoint stop after a page instead of
// reading the library.
func (s *SQLiteStore) ensureSchemaV16(ctx context.Context) error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS documents_title_idx ON documents(collection_id, deleted_at, title COLLATE NOCASE, id);`,
		`CREATE INDEX IF NOT EXISTS resources_filename_idx ON resources(collection_id, filename COLLATE NOCASE, id);`,
		`PRAGMA user_version = 16;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchemaV17 adds the batch idempotency ledger. A batch is retried exactly
// when something went wrong, so the record of "this key already ran" has to
// outlive the process; an in-memory map would forget precisely when it matters.
func (s *SQLiteStore) ensureSchemaV17(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS batch_operations (
			request_key TEXT PRIMARY KEY,
			operation TEXT NOT NULL,
			mode TEXT NOT NULL,
			response TEXT NOT NULL,
			request_sha256 TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS batch_operations_created_idx ON batch_operations(created_at);`,
		`PRAGMA user_version = 17;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchemaV18 adds the job control plane's one table (v0.6 F6).
//
// Records persist across a restart; the work does not. There is no queue
// column, no priority, and no dependency column, because none of those is a
// job *record* — they are a scheduler, which this deliberately is not.
func (s *SQLiteStore) ensureSchemaV18(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			collection_id TEXT NOT NULL DEFAULT 'default',
			parameters TEXT NOT NULL DEFAULT '[]',
			phase TEXT NOT NULL DEFAULT '',
			processed INTEGER NOT NULL DEFAULT 0,
			total INTEGER NOT NULL DEFAULT 0,
			summary TEXT NOT NULL DEFAULT '{}',
			error TEXT NOT NULL DEFAULT '',
			cancel_requested INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at TEXT,
			finished_at TEXT,
			heartbeat_at TEXT
		);`,
		// Listing is newest-first by rowid, which the table already provides in
		// reverse order for free — CURRENT_TIMESTAMP's one-second resolution
		// makes `created_at` an unreliable sort key for jobs started together.
		// The one index that earns its place is the state filter.
		`CREATE INDEX IF NOT EXISTS jobs_status_idx ON jobs(status);`,
		`PRAGMA user_version = 18;`,
	}
	for _, statement := range statements {
		if err := s.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// ensureSchemaV19 adds the local replication journal. The migration is kept in
// its own SQL file because the same triggers must be byte-for-byte identical on
// fresh databases and upgrades. Capture triggers are inert until explicit
// enrollment creates sync_local_journal's singleton row.
func (s *SQLiteStore) ensureSchemaV19(ctx context.Context) error {
	s.mu.Lock()
	version, versionErr := s.pragmaUserVersionLocked()
	s.mu.Unlock()
	if versionErr != nil {
		return versionErr
	}
	if version >= 19 {
		return nil
	}
	migration, err := migrationFS.ReadFile("migrations/0019_sync_journal.sql")
	if err != nil {
		return fmt.Errorf("read schema v19 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}

// ensureSchemaV20 adds G5's persisted peer compatibility tuple and a hard
// sequence-exhaustion guard. Admission uses the G4 journal tables; it does not
// need or create a transport outbox.
func (s *SQLiteStore) ensureSchemaV20(ctx context.Context) error {
	s.mu.Lock()
	version, versionErr := s.pragmaUserVersionLocked()
	s.mu.Unlock()
	if versionErr != nil {
		return versionErr
	}
	if version >= 20 {
		return nil
	}
	migration, err := migrationFS.ReadFile("migrations/0020_sync_admission.sql")
	if err != nil {
		return fmt.Errorf("read schema v20 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}

// ensureSchemaV21 adds G6's durable HLC and convergence projection. Unlike
// earlier CREATE-only migrations it adds two columns independently before the
// idempotent SQL portion, so interrupted development migrations can resume.
func (s *SQLiteStore) ensureSchemaV21(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	version, err := s.pragmaUserVersionLocked()
	hasWall, hasLogical := false, false
	if err == nil {
		stmt, prepareErr := s.prepareLocked(`PRAGMA table_info(sync_operations)`)
		if prepareErr != nil {
			err = prepareErr
		} else {
			for C.sqlite3_step(stmt) == C.SQLITE_ROW {
				switch columnText(stmt, 1) {
				case "hlc_wall_ms":
					hasWall = true
				case "hlc_logical":
					hasLogical = true
				}
			}
			C.sqlite3_finalize(stmt)
		}
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if version >= 21 {
		return nil
	}
	// Add each operation column independently so an interrupted/partially
	// constructed development database resumes through the idempotent table and
	// trigger portion below instead of being mistaken for a complete migration.
	if !hasWall {
		if err := s.Exec(ctx, `ALTER TABLE sync_operations ADD COLUMN hlc_wall_ms INTEGER NOT NULL DEFAULT 0;`); err != nil {
			return err
		}
	}
	if !hasLogical {
		if err := s.Exec(ctx, `ALTER TABLE sync_operations ADD COLUMN hlc_logical INTEGER NOT NULL DEFAULT 0;`); err != nil {
			return err
		}
	}
	migration, err := migrationFS.ReadFile("migrations/0021_sync_metadata.sql")
	if err != nil {
		return fmt.Errorf("read schema v21 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}

func (s *SQLiteStore) exec(sql string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execLocked(sql)
}

func (s *SQLiteStore) execLocked(sql string) error {
	if s.db == nil {
		return fmt.Errorf("sqlite store is closed")
	}
	csql := C.CString(sql)
	defer C.free(unsafe.Pointer(csql))
	var errmsg *C.char
	if rc := C.sqlite3_exec(s.db, csql, nil, nil, &errmsg); rc != C.SQLITE_OK {
		msg := C.GoString(errmsg)
		C.sqlite3_free(unsafe.Pointer(errmsg))
		return fmt.Errorf("sqlite exec: %s", msg)
	}
	return nil
}

func (s *SQLiteStore) Status(ctx context.Context) (StoreStatus, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return StoreStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return StoreStatus{Driver: "sqlite", Path: s.path, State: "closed"}, nil
	}
	version, err := s.pragmaUserVersionLocked()
	if err != nil {
		return StoreStatus{}, err
	}
	return StoreStatus{
		Driver:        "sqlite",
		Path:          s.path,
		State:         "open",
		SchemaVersion: version,
	}, nil
}

func (s *SQLiteStore) pragmaUserVersionLocked() (int, error) {
	stmt, err := s.prepareLocked(`PRAGMA user_version`)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return 0, nil
	}
	if rc != C.SQLITE_ROW {
		return 0, s.stepErrLocked(rc)
	}
	return int(C.sqlite3_column_int(stmt, 0)), nil
}

func (s *SQLiteStore) ListCollections(ctx context.Context) ([]Collection, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.prepareLocked(`SELECT id, name, COALESCE(description, '') FROM collections ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)

	collections := []Collection{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			collections = append(collections, Collection{
				ID:           columnText(stmt, 0),
				Name:         columnText(stmt, 1),
				Description:  columnText(stmt, 2),
				Capabilities: []string{"documents", "search", "resources", "links", "graph"},
			})
		case C.SQLITE_DONE:
			return collections, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) CreateDocument(ctx context.Context, req CreateDocumentRequest) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	req = NormalizeCreateRequest(req)
	docID := req.PreferredID
	if docID == "" {
		var err error
		docID, err = NewID("doc")
		if err != nil {
			return Document{}, err
		}
	}
	revID, err := NewID("rev")
	if err != nil {
		return Document{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return Document{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	if exists, err := s.notebookExistsLocked(req.NotebookID); err != nil {
		return Document{}, err
	} else if !exists {
		return Document{}, fmt.Errorf("%w: notebook %q", ErrNotFound, req.NotebookID)
	}
	if err := s.execPreparedLocked(`INSERT INTO documents(id, collection_id, notebook_id, title, body_mime_type, current_revision_id)
		VALUES(?, ?, ?, ?, ?, ?)`, docID, req.CollectionID, req.NotebookID, req.Title, req.BodyMIMEType, revID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message) VALUES(?, ?, ?, ?, ?, ?)`, revID, docID, req.Title, req.Body, req.BodyMIMEType, req.Message); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`, docID, req.CollectionID, req.Title, req.Body); err != nil {
		return Document{}, err
	}
	if err := s.rebuildDocumentLinksLocked(docID, req.CollectionID, req.Body); err != nil {
		return Document{}, err
	}
	if err := s.enqueueProjectionLocked(docID, "upsert"); err != nil {
		return Document{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return Document{}, err
	}
	committed = true

	return s.getDocumentLocked(docID)
}

func (s *SQLiteStore) GetDocument(ctx context.Context, id string) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDocumentLocked(id)
}

// GetDocumentIncludingTrashed reads a note whether or not it is in the Trash.
//
// It exists because a trashed note has to be *readable* to be recoverable: the
// Trash is a list of notes someone may want to look at before restoring one,
// and a stable link that resolves to `trashed` has to open something. The
// returned Document carries DeletedAt, which is what makes it read-only above
// the store.
//
// Everything else keeps using GetDocument, which stops at the Trash. That is
// the right default for every write path and for the agent-facing surfaces:
// trashed notes are outside the ordinary query scope by design, and `is:trashed`
// is a deliberate opt-in rather than something a caller falls into.
func (s *SQLiteStore) GetDocumentIncludingTrashed(ctx context.Context, id string) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readDocumentLocked(id, true)
}

func (s *SQLiteStore) getDocumentLocked(id string) (Document, error) {
	return s.readDocumentLocked(id, false)
}

func (s *SQLiteStore) readDocumentLocked(id string, includeTrashed bool) (Document, error) {
	trashFilter := ` AND d.deleted_at IS NULL`
	if includeTrashed {
		trashFilter = ""
	}
	stmt, err := s.prepareLocked(`SELECT d.id, d.collection_id, d.title, r.body, COALESCE(r.body_mime_type, d.body_mime_type), d.current_revision_id, d.created_at, d.updated_at, COALESCE(d.deleted_at, ''), COALESCE(d.notebook_id, '')
		FROM documents d
		JOIN document_revisions r ON r.id = d.current_revision_id
		WHERE d.id = ?` + trashFilter)
	if err != nil {
		return Document{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return Document{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return Document{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return Document{}, s.stepErrLocked(rc)
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
	updatedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 7)))
	doc := Document{
		ID:                columnText(stmt, 0),
		CollectionID:      columnText(stmt, 1),
		Title:             columnText(stmt, 2),
		Body:              columnText(stmt, 3),
		BodyMIMEType:      columnText(stmt, 4),
		CurrentRevisionID: columnText(stmt, 5),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
		NotebookID:        columnText(stmt, 9),
	}
	// DeletedAt is what makes a trashed note read-only above the store, so it
	// has to survive the read. It is always empty on the non-trashed path.
	if deleted := columnText(stmt, 8); deleted != "" {
		doc.DeletedAt, _ = time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(deleted))
	}
	doc.URI = DocumentURI(doc.CollectionID, doc.ID)
	return doc, nil
}

func sqliteTimeToRFC3339(value string) string {
	if strings.Contains(value, "T") {
		return value
	}
	return strings.Replace(value, " ", "T", 1) + "Z"
}

func (s *SQLiteStore) UpdateDocument(ctx context.Context, req UpdateDocumentRequest) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	req = NormalizeUpdateRequest(req)
	if req.ID == "" {
		return Document{}, ErrNotFound
	}
	if strings.TrimSpace(req.BaseRevisionID) == "" {
		return Document{}, ErrPreconditionRequired
	}
	revID, err := NewID("rev")
	if err != nil {
		return Document{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return Document{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	current, err := s.getDocumentLocked(req.ID)
	if err != nil {
		return Document{}, err
	}
	if current.CurrentRevisionID != req.BaseRevisionID {
		return Document{}, ErrConflict
	}
	if err := s.execPreparedLocked(`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message) VALUES(?, ?, ?, ?, ?, ?)`, revID, req.ID, req.Title, req.Body, req.BodyMIMEType, req.Message); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`UPDATE documents SET title = ?, body_mime_type = ?, current_revision_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, req.Title, req.BodyMIMEType, revID, req.ID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`DELETE FROM documents_fts WHERE document_id = ?`, req.ID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`, req.ID, current.CollectionID, req.Title, req.Body); err != nil {
		return Document{}, err
	}
	if err := s.rebuildDocumentLinksLocked(req.ID, current.CollectionID, req.Body); err != nil {
		return Document{}, err
	}
	if err := s.enqueueProjectionLocked(req.ID, "upsert"); err != nil {
		return Document{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return Document{}, err
	}
	committed = true
	return s.getDocumentLocked(req.ID)
}

func (s *SQLiteStore) DeleteDocument(ctx context.Context, req DeleteDocumentRequest) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	req = NormalizeDeleteRequest(req)
	if req.ID == "" {
		return ErrNotFound
	}
	if strings.TrimSpace(req.BaseRevisionID) == "" {
		return ErrPreconditionRequired
	}
	revID, err := NewID("rev")
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	if err := s.deleteDocumentLocked(req, revID); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// deleteDocumentLocked is the trash-first delete without its transaction, so a
// batch can run many inside one. The caller holds the mutex and owns the
// transaction boundary.
func (s *SQLiteStore) deleteDocumentLocked(req DeleteDocumentRequest, revID string) error {
	current, err := s.getDocumentLocked(req.ID)
	if err != nil {
		return err
	}
	if current.CurrentRevisionID != req.BaseRevisionID {
		return ErrConflict
	}
	if err := s.execPreparedLocked(`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message) VALUES(?, ?, ?, ?, ?, ?)`, revID, req.ID, current.Title, current.Body, current.BodyMIMEType, req.Message); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`UPDATE documents SET current_revision_id = ?, deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, revID, req.ID); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`DELETE FROM documents_fts WHERE document_id = ?`, req.ID); err != nil {
		return err
	}
	return s.enqueueProjectionLocked(req.ID, "delete")
}

func (s *SQLiteStore) ListDocumentRevisions(ctx context.Context, documentID string) ([]DocumentRevision, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if exists, err := s.documentExistsLocked(documentID); err != nil {
		return nil, err
	} else if !exists {
		return nil, ErrNotFound
	}
	stmt, err := s.prepareLocked(`SELECT id, document_id, title, body, body_mime_type, COALESCE(message, ''), created_at FROM document_revisions WHERE document_id = ? ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return nil, err
	}
	revisions := []DocumentRevision{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			revisions = append(revisions, revisionFromStmt(stmt))
		case C.SQLITE_DONE:
			return revisions, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) GetDocumentRevision(ctx context.Context, documentID, revisionID string) (DocumentRevision, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DocumentRevision{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDocumentRevisionLocked(documentID, revisionID)
}

func (s *SQLiteStore) RestoreDocumentRevision(ctx context.Context, req RestoreRevisionRequest) (Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	req = NormalizeRestoreRevisionRequest(req)
	if req.DocumentID == "" || req.RevisionID == "" {
		return Document{}, ErrNotFound
	}
	if strings.TrimSpace(req.BaseRevisionID) == "" {
		return Document{}, ErrPreconditionRequired
	}
	newRevID, err := NewID("rev")
	if err != nil {
		return Document{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return Document{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	collectionID, currentRevisionID, err := s.documentCurrentStateLocked(req.DocumentID)
	if err != nil {
		return Document{}, err
	}
	if currentRevisionID != req.BaseRevisionID {
		return Document{}, ErrConflict
	}
	target, err := s.getDocumentRevisionLocked(req.DocumentID, req.RevisionID)
	if err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message) VALUES(?, ?, ?, ?, ?, ?)`, newRevID, req.DocumentID, target.Title, target.Body, target.BodyMIMEType, req.Message); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`UPDATE documents SET title = ?, body_mime_type = ?, current_revision_id = ?, deleted_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, target.Title, target.BodyMIMEType, newRevID, req.DocumentID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`DELETE FROM documents_fts WHERE document_id = ?`, req.DocumentID); err != nil {
		return Document{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`, req.DocumentID, collectionID, target.Title, target.Body); err != nil {
		return Document{}, err
	}
	if err := s.rebuildDocumentLinksLocked(req.DocumentID, collectionID, target.Body); err != nil {
		return Document{}, err
	}
	if err := s.enqueueProjectionLocked(req.DocumentID, "upsert"); err != nil {
		return Document{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return Document{}, err
	}
	committed = true
	return s.getDocumentLocked(req.DocumentID)
}

func (s *SQLiteStore) documentExistsLocked(documentID string) (bool, error) {
	stmt, err := s.prepareLocked(`SELECT 1 FROM documents WHERE id = ?`)
	if err != nil {
		return false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return false, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_ROW {
		return true, nil
	}
	if rc == C.SQLITE_DONE {
		return false, nil
	}
	return false, s.stepErrLocked(rc)
}

func (s *SQLiteStore) documentCurrentStateLocked(documentID string) (collectionID string, currentRevisionID string, err error) {
	stmt, err := s.prepareLocked(`SELECT collection_id, current_revision_id FROM documents WHERE id = ?`)
	if err != nil {
		return "", "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return "", "", err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return "", "", ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return "", "", s.stepErrLocked(rc)
	}
	return columnText(stmt, 0), columnText(stmt, 1), nil
}

func (s *SQLiteStore) getDocumentRevisionLocked(documentID, revisionID string) (DocumentRevision, error) {
	stmt, err := s.prepareLocked(`SELECT id, document_id, title, body, body_mime_type, COALESCE(message, ''), created_at FROM document_revisions WHERE document_id = ? AND id = ?`)
	if err != nil {
		return DocumentRevision{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID, revisionID}); err != nil {
		return DocumentRevision{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return DocumentRevision{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return DocumentRevision{}, s.stepErrLocked(rc)
	}
	return revisionFromStmt(stmt), nil
}

func revisionFromStmt(stmt *C.sqlite3_stmt) DocumentRevision {
	createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
	return DocumentRevision{
		ID:           columnText(stmt, 0),
		DocumentID:   columnText(stmt, 1),
		Title:        columnText(stmt, 2),
		Body:         columnText(stmt, 3),
		BodyMIMEType: columnText(stmt, 4),
		Message:      columnText(stmt, 5),
		CreatedAt:    createdAt,
	}
}

func (s *SQLiteStore) CreateResource(ctx context.Context, req CreateResourceRequest) (Resource, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Resource{}, err
	}
	req = NormalizeCreateResourceRequest(req)
	if req.Content == nil {
		return Resource{}, fmt.Errorf("%w: resource content is required", ErrInvalidInput)
	}
	resourceID := req.PreferredID
	if resourceID == "" {
		var err error
		resourceID, err = NewID("res")
		if err != nil {
			return Resource{}, err
		}
	}

	blob, cleanup, err := s.writeBlob(ctx, req.Content, req.MIMEType)
	if err != nil {
		return Resource{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	mimeType := firstNonEmptyString(req.MIMEType, blob.MIMEType, "application/octet-stream")
	perceptualHash, err := s.computePerceptualHash(ctx, blob, mimeType)
	if err != nil {
		return Resource{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return Resource{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	if err := s.execPreparedLocked(`INSERT OR IGNORE INTO blobs(sha256, storage_path, size_bytes, mime_type) VALUES(?, ?, ?, ?)`, blob.SHA256, blob.StoragePath, strconv.FormatInt(blob.SizeBytes, 10), mimeType); err != nil {
		return Resource{}, err
	}
	if perceptualHash != nil {
		if err := s.execPreparedLocked(`INSERT OR IGNORE INTO resource_hashes(blob_sha256, algo, hash) VALUES(?, ?, ?)`,
			blob.SHA256, perceptualHash.Algorithm, perceptualHash.Hash); err != nil {
			return Resource{}, err
		}
		// This is the admission-time policy-check slot. A matching perceptual
		// rule is deliberately non-blocking; it is surfaced by ResourceReport
		// as a review suggestion.
		if err := s.checkPerceptualReviewRuleLocked(perceptualHash.Algorithm, perceptualHash.Hash); err != nil {
			return Resource{}, err
		}
	}
	if err := s.execPreparedLocked(`INSERT INTO resources(id, collection_id, blob_sha256, filename, mime_type, unreferenced_at, unreferenced_reason)
		VALUES(?, ?, ?, ?, ?, CURRENT_TIMESTAMP, 'created_unattached')`, resourceID, req.CollectionID, blob.SHA256, req.Filename, mimeType); err != nil {
		return Resource{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return Resource{}, err
	}
	committed = true
	if cleanup != nil {
		cleanup()
		cleanup = nil
	}
	return s.getResourceLocked(resourceID)
}

func (s *SQLiteStore) UpdateResource(ctx context.Context, req UpdateResourceRequest) (Resource, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Resource{}, err
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	if req.ID == "" || req.Content == nil {
		return Resource{}, fmt.Errorf("%w: resource ID and content are required", ErrInvalidInput)
	}
	if req.MIMEType == "" {
		req.MIMEType = "application/octet-stream"
	}
	if _, err := s.GetResource(ctx, req.ID); err != nil {
		return Resource{}, err
	}
	blob, cleanup, err := s.writeBlob(ctx, req.Content, req.MIMEType)
	if err != nil {
		return Resource{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	mimeType := firstNonEmptyString(req.MIMEType, blob.MIMEType, "application/octet-stream")
	perceptualHash, err := s.computePerceptualHash(ctx, blob, mimeType)
	if err != nil {
		return Resource{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return Resource{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	oldSHA, oldStoragePath, err := s.resourceBlobLocked(req.ID)
	if err != nil {
		return Resource{}, err
	}
	if err := s.execPreparedLocked(`INSERT OR IGNORE INTO blobs(sha256, storage_path, size_bytes, mime_type)
		VALUES(?, ?, ?, ?)`, blob.SHA256, blob.StoragePath, strconv.FormatInt(blob.SizeBytes, 10), mimeType); err != nil {
		return Resource{}, err
	}
	if perceptualHash != nil {
		if err := s.execPreparedLocked(`INSERT OR IGNORE INTO resource_hashes(blob_sha256, algo, hash)
			VALUES(?, ?, ?)`, blob.SHA256, perceptualHash.Algorithm, perceptualHash.Hash); err != nil {
			return Resource{}, err
		}
		if err := s.checkPerceptualReviewRuleLocked(perceptualHash.Algorithm, perceptualHash.Hash); err != nil {
			return Resource{}, err
		}
	}
	if err := s.execPreparedLocked(`UPDATE resources
		SET blob_sha256 = ?, filename = ?, mime_type = ?
		WHERE id = ?`, blob.SHA256, req.Filename, mimeType, req.ID); err != nil {
		return Resource{}, err
	}
	removeOldBlob := false
	if oldSHA != blob.SHA256 {
		resourceCount, err := s.countLocked(`SELECT COUNT(*) FROM resources WHERE blob_sha256 = ?`, oldSHA)
		if err != nil {
			return Resource{}, err
		}
		removeOldBlob = resourceCount == 0
		if removeOldBlob {
			if err := s.execPreparedLocked(`DELETE FROM resource_hashes WHERE blob_sha256 = ?`, oldSHA); err != nil {
				return Resource{}, err
			}
			if err := s.execPreparedLocked(`DELETE FROM blobs WHERE sha256 = ?`, oldSHA); err != nil {
				return Resource{}, err
			}
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return Resource{}, err
	}
	committed = true
	if cleanup != nil {
		cleanup()
		cleanup = nil
	}
	if removeOldBlob {
		if oldPath, ok := safeAssetPath(s.assetRoot, oldStoragePath); ok {
			_ = os.Remove(oldPath)
		}
	}
	return s.getResourceLocked(req.ID)
}

type storedBlob struct {
	SHA256      string
	StoragePath string
	SizeBytes   int64
	MIMEType    string
}

func (s *SQLiteStore) writeBlob(ctx context.Context, content io.Reader, mimeType string) (storedBlob, func(), error) {
	if err := os.MkdirAll(s.assetRoot, 0o755); err != nil {
		return storedBlob{}, nil, err
	}
	tmp, err := os.CreateTemp(s.assetRoot, "incoming-*")
	if err != nil {
		return storedBlob{}, nil, err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	h := sha256.New()
	written, copyErr := copyWithContext(ctx, io.MultiWriter(tmp, h), content)
	closeErr := tmp.Close()
	if copyErr != nil {
		cleanup()
		return storedBlob{}, nil, copyErr
	}
	if closeErr != nil {
		cleanup()
		return storedBlob{}, nil, closeErr
	}
	shaHex := hex.EncodeToString(h.Sum(nil))
	if strings.TrimSpace(mimeType) == "" || strings.EqualFold(mimeType, "application/octet-stream") {
		detected, err := sniffFileMIME(tmpName)
		if err != nil {
			cleanup()
			return storedBlob{}, nil, err
		}
		mimeType = detected
	}
	rel := filepath.Join("sha256", shaHex[0:2], shaHex[2:4], shaHex)
	finalPath := filepath.Join(s.assetRoot, rel)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		cleanup()
		return storedBlob{}, nil, err
	}
	if _, err := os.Stat(finalPath); err == nil {
		cleanup()
		return storedBlob{SHA256: shaHex, StoragePath: rel, SizeBytes: written, MIMEType: mimeType}, nil, nil
	} else if err != nil && !os.IsNotExist(err) {
		cleanup()
		return storedBlob{}, nil, err
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		cleanup()
		return storedBlob{}, nil, err
	}
	return storedBlob{SHA256: shaHex, StoragePath: rel, SizeBytes: written, MIMEType: mimeType}, nil, nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			m, writeErr := dst.Write(buf[:n])
			written += int64(m)
			if writeErr != nil {
				return written, writeErr
			}
			if m != n {
				return written, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

func sniffFileMIME(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var buf [512]byte
	n, err := file.Read(buf[:])
	if err != nil && err != io.EOF {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}

func (s *SQLiteStore) GetResource(ctx context.Context, id string) (Resource, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Resource{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getResourceLocked(id)
}

func (s *SQLiteStore) getResourceLocked(id string) (Resource, error) {
	stmt, err := s.prepareLocked(`SELECT r.id, r.collection_id, COALESCE(r.filename, ''), r.mime_type, b.size_bytes, b.sha256, r.created_at
		FROM resources r
		JOIN blobs b ON b.sha256 = r.blob_sha256
		WHERE r.id = ?`)
	if err != nil {
		return Resource{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return Resource{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return Resource{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return Resource{}, s.stepErrLocked(rc)
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
	res := Resource{
		ID:           columnText(stmt, 0),
		CollectionID: columnText(stmt, 1),
		Filename:     columnText(stmt, 2),
		MIMEType:     columnText(stmt, 3),
		SizeBytes:    columnInt64(stmt, 4),
		SHA256:       columnText(stmt, 5),
		CreatedAt:    createdAt,
	}
	res.URI = ResourceURI(res.CollectionID, res.ID)
	return res, nil
}

func (s *SQLiteStore) OpenResourceContent(ctx context.Context, id string) (Resource, io.ReadCloser, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return Resource{}, nil, err
	}
	s.mu.Lock()
	res, storagePath, err := s.getResourceWithStoragePathLocked(id)
	s.mu.Unlock()
	if err != nil {
		return Resource{}, nil, err
	}
	file, err := os.Open(filepath.Join(s.assetRoot, storagePath))
	if err != nil {
		if os.IsNotExist(err) {
			return Resource{}, nil, ErrNotFound
		}
		return Resource{}, nil, err
	}
	return res, file, nil
}

func (s *SQLiteStore) getResourceWithStoragePathLocked(id string) (Resource, string, error) {
	stmt, err := s.prepareLocked(`SELECT r.id, r.collection_id, COALESCE(r.filename, ''), r.mime_type, b.size_bytes, b.sha256, r.created_at, b.storage_path
		FROM resources r
		JOIN blobs b ON b.sha256 = r.blob_sha256
		WHERE r.id = ?`)
	if err != nil {
		return Resource{}, "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return Resource{}, "", err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return Resource{}, "", ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return Resource{}, "", s.stepErrLocked(rc)
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
	res := Resource{
		ID:           columnText(stmt, 0),
		CollectionID: columnText(stmt, 1),
		Filename:     columnText(stmt, 2),
		MIMEType:     columnText(stmt, 3),
		SizeBytes:    columnInt64(stmt, 4),
		SHA256:       columnText(stmt, 5),
		CreatedAt:    createdAt,
	}
	res.URI = ResourceURI(res.CollectionID, res.ID)
	return res, columnText(stmt, 7), nil
}

func (s *SQLiteStore) DeleteResource(ctx context.Context, id string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	blobSHA, storagePath, err := s.resourceBlobLocked(id)
	if err != nil {
		return err
	}
	refCount, err := s.countLocked(`SELECT COUNT(*) FROM document_resource_refs WHERE resource_id = ?`, id)
	if err != nil {
		return err
	}
	if refCount > 0 {
		return ErrConflict
	}
	if err := s.execPreparedLocked(`DELETE FROM resources WHERE id = ?`, id); err != nil {
		return err
	}
	resourceCount, err := s.countLocked(`SELECT COUNT(*) FROM resources WHERE blob_sha256 = ?`, blobSHA)
	if err != nil {
		return err
	}
	deleteBlob := resourceCount == 0
	if deleteBlob {
		if err := s.execPreparedLocked(`DELETE FROM resource_hashes WHERE blob_sha256 = ?`, blobSHA); err != nil {
			return err
		}
		if err := s.execPreparedLocked(`DELETE FROM blobs WHERE sha256 = ?`, blobSHA); err != nil {
			return err
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	if deleteBlob {
		_ = os.Remove(filepath.Join(s.assetRoot, storagePath))
	}
	return nil
}

func (s *SQLiteStore) resourceBlobLocked(id string) (sha256Hex, storagePath string, err error) {
	stmt, err := s.prepareLocked(`SELECT b.sha256, b.storage_path FROM resources r JOIN blobs b ON b.sha256 = r.blob_sha256 WHERE r.id = ?`)
	if err != nil {
		return "", "", err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return "", "", err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return "", "", ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return "", "", s.stepErrLocked(rc)
	}
	return columnText(stmt, 0), columnText(stmt, 1), nil
}

func (s *SQLiteStore) ListDocumentResources(ctx context.Context, documentID string) ([]ResourceReference, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if exists, err := s.documentExistsLocked(documentID); err != nil {
		return nil, err
	} else if !exists {
		return nil, ErrNotFound
	}
	stmt, err := s.prepareLocked(`SELECT rr.document_id, rr.resource_id, rr.relation_type, rr.ordinal, rr.anchor_json,
		r.id, r.collection_id, COALESCE(r.filename, ''), r.mime_type, b.size_bytes, b.sha256, r.created_at
		FROM document_resource_refs rr
		JOIN resources r ON r.id = rr.resource_id
		JOIN blobs b ON b.sha256 = r.blob_sha256
		WHERE rr.document_id = ?
		ORDER BY rr.ordinal, rr.relation_type, rr.resource_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{documentID}); err != nil {
		return nil, err
	}
	refs := []ResourceReference{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 11)))
			res := Resource{
				ID:           columnText(stmt, 5),
				CollectionID: columnText(stmt, 6),
				Filename:     columnText(stmt, 7),
				MIMEType:     columnText(stmt, 8),
				SizeBytes:    columnInt64(stmt, 9),
				SHA256:       columnText(stmt, 10),
				CreatedAt:    createdAt,
			}
			res.URI = ResourceURI(res.CollectionID, res.ID)
			refs = append(refs, ResourceReference{
				DocumentID:   columnText(stmt, 0),
				ResourceID:   columnText(stmt, 1),
				RelationType: columnText(stmt, 2),
				Ordinal:      int(columnInt64(stmt, 3)),
				AnchorJSON:   columnText(stmt, 4),
				Resource:     res,
			})
		case C.SQLITE_DONE:
			return refs, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) AttachDocumentResource(ctx context.Context, req AttachResourceRequest) (ResourceReference, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return ResourceReference{}, err
	}
	req = NormalizeAttachResourceRequest(req)
	if req.DocumentID == "" || req.ResourceID == "" {
		return ResourceReference{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.getDocumentLocked(req.DocumentID); err != nil {
		return ResourceReference{}, err
	}
	res, err := s.getResourceLocked(req.ResourceID)
	if err != nil {
		return ResourceReference{}, err
	}
	if err := s.execPreparedLocked(`INSERT OR REPLACE INTO document_resource_refs(document_id, resource_id, relation_type, ordinal, anchor_json) VALUES(?, ?, ?, ?, ?)`, req.DocumentID, req.ResourceID, req.RelationType, strconv.Itoa(req.Ordinal), req.AnchorJSON); err != nil {
		return ResourceReference{}, err
	}
	if err := s.execPreparedLocked(`UPDATE resources SET unreferenced_at = NULL, unreferenced_reason = '' WHERE id = ?`, req.ResourceID); err != nil {
		return ResourceReference{}, err
	}
	return ResourceReference{DocumentID: req.DocumentID, ResourceID: req.ResourceID, Resource: res, RelationType: req.RelationType, Ordinal: req.Ordinal, AnchorJSON: req.AnchorJSON}, nil
}

func (s *SQLiteStore) DetachDocumentResource(ctx context.Context, documentID, resourceID string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if exists, err := s.documentExistsLocked(documentID); err != nil {
		return err
	} else if !exists {
		return ErrNotFound
	}
	if _, err := s.getResourceLocked(resourceID); err != nil {
		return err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`DELETE FROM document_resource_refs WHERE document_id = ? AND resource_id = ?`, documentID, resourceID); err != nil {
		return err
	}
	refCount, err := s.countLocked(`SELECT COUNT(*) FROM document_resource_refs WHERE resource_id = ?`, resourceID)
	if err != nil {
		return err
	}
	if refCount == 0 {
		if err := s.execPreparedLocked(`UPDATE resources
			SET unreferenced_at = CURRENT_TIMESTAMP, unreferenced_reason = 'detached'
			WHERE id = ?`, resourceID); err != nil {
			return err
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// RebuildDocumentLinks reparses the current document body and refreshes link rows
// without creating a new document revision. Importers use this after a batch
// creates multiple notes so links to later-created notes can resolve.
func (s *SQLiteStore) RebuildDocumentLinks(ctx context.Context, documentID string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	doc, err := s.getDocumentLocked(documentID)
	if err != nil {
		return err
	}
	if err := s.rebuildDocumentLinksLocked(doc.ID, doc.CollectionID, doc.Body); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// rebuildDocumentLinksLocked re-derives a note's links and blocks. Both are
// derived from the same body and must describe the same one, so they are
// rebuilt together inside the caller's transaction rather than by separate
// calls that could disagree.
func (s *SQLiteStore) rebuildDocumentLinksLocked(documentID, collectionID, body string) error {
	if err := s.rebuildDocumentBlocksLocked(documentID, body); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`DELETE FROM document_links WHERE source_document_id = ?`, documentID); err != nil {
		return err
	}
	for _, candidate := range markdownlinks.Extract(body) {
		link := s.resolveLinkCandidateLocked(documentID, collectionID, candidate)
		if err := s.execPreparedLocked(`INSERT INTO document_links(
			source_document_id, target_document_id, target_resource_id, target_uri,
			relation_type, source_format, raw_target, display_text, anchor_type, anchor_value,
			context, source_start_byte, source_end_byte, source_line, source_column, resolution_status
		) VALUES(?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			documentID, link.TargetDocumentID, link.TargetResourceID, link.TargetURI,
			link.RelationType, link.SourceFormat, link.RawTarget, link.DisplayText, link.AnchorType, link.AnchorValue,
			link.Context, strconv.Itoa(link.SourceStartByte), strconv.Itoa(link.SourceEndByte), strconv.Itoa(link.SourceLine), strconv.Itoa(link.SourceColumn), link.ResolutionStatus); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) resolveLinkCandidateLocked(sourceDocumentID, collectionID string, candidate markdownlinks.Candidate) DocumentLink {
	link := DocumentLink{
		SourceDocumentID: sourceDocumentID,
		RelationType:     candidate.RelationType,
		SourceFormat:     candidate.SourceFormat,
		RawTarget:        candidate.RawTarget,
		DisplayText:      candidate.DisplayText,
		AnchorType:       candidate.AnchorType,
		AnchorValue:      candidate.AnchorValue,
		Context:          candidate.Context,
		SourceStartByte:  candidate.StartByte,
		SourceEndByte:    candidate.EndByte,
		SourceLine:       candidate.Line,
		SourceColumn:     candidate.Column,
		ResolutionStatus: "unresolved",
	}
	raw := strings.TrimSpace(candidate.RawTarget)
	if raw == "" && candidate.AnchorValue != "" {
		// An anchor with no target names a section of the note it is written
		// in. Without a source note there is nothing for it to name, which only
		// happens when a caller checks a buffer that has never been saved.
		if sourceDocumentID == "" {
			return link
		}
		link.TargetDocumentID = sourceDocumentID
		link.TargetURI = DocumentURI(collectionID, sourceDocumentID)
		link.ResolutionStatus = "resolved"
		return link
	}
	if isExternalTarget(raw) {
		link.TargetURI = raw
		link.ResolutionStatus = "external"
		return link
	}
	if stablelink.HasScheme(raw) {
		return s.resolveStableLinkCandidateLocked(link, raw)
	}
	if docID := documentIDFromURI(raw); docID != "" {
		link.TargetURI = raw
		if ok, err := s.documentExistsLocked(docID); err == nil && ok {
			link.TargetDocumentID = docID
			link.ResolutionStatus = "resolved"
		}
		return link
	}
	if resourceID := resourceIDFromURI(raw); resourceID != "" {
		link.TargetURI = raw
		if _, err := s.getResourceLocked(resourceID); err == nil {
			link.TargetResourceID = resourceID
			link.ResolutionStatus = "resolved"
		}
		return link
	}
	if raw == "" {
		return link
	}
	if docID, ambiguous, err := s.findDocumentByTitleLocked(collectionID, raw); err == nil {
		if ambiguous {
			link.ResolutionStatus = "ambiguous"
			return link
		}
		if docID != "" {
			link.TargetDocumentID = docID
			link.TargetURI = DocumentURI(collectionID, docID)
			link.ResolutionStatus = "resolved"
			return link
		}
	}
	if resourceID, ambiguous, err := s.findResourceByFilenameLocked(collectionID, raw); err == nil {
		if ambiguous {
			link.ResolutionStatus = "ambiguous"
			return link
		}
		if resourceID != "" {
			link.TargetResourceID = resourceID
			link.TargetURI = ResourceURI(collectionID, resourceID)
			link.ResolutionStatus = "resolved"
			return link
		}
	}
	link.TargetURI = raw
	return link
}

// resolveStableLinkCandidateLocked classifies a notrios:// link found in a
// note body. A stable link is portable by design, so the same syntax can name
// this database or another one, and the two must not be confused:
//
//   - this database, note present  -> resolved, exactly like document://
//   - this database, note missing  -> unresolved (a stale target, not an error)
//   - another database             -> external; nothing local may be opened
//   - malformed                    -> invalid
//
// A link naming a foreign database is never resolved against local IDs even if
// a document with that ID happens to exist here. Document IDs are unique per
// database, not globally, so matching one across universes would silently open
// the wrong note.
func (s *SQLiteStore) resolveStableLinkCandidateLocked(link DocumentLink, raw string) DocumentLink {
	link.TargetURI = raw
	parsed, err := stablelink.Parse(raw)
	if err != nil {
		link.ResolutionStatus = "invalid"
		return link
	}
	localID, err := s.databaseIDLocked()
	if err != nil || localID == "" {
		link.ResolutionStatus = "unresolved"
		return link
	}
	if parsed.DatabaseID != localID {
		link.ResolutionStatus = "external"
		return link
	}
	if ok, err := s.documentExistsLocked(parsed.DocumentID); err == nil && ok {
		link.TargetDocumentID = parsed.DocumentID
		link.ResolutionStatus = "resolved"
	}
	return link
}

func isExternalTarget(target string) bool {
	lower := strings.ToLower(strings.TrimSpace(target))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:")
}

func documentIDFromURI(uri string) string {
	const marker = "/documents/"
	if !strings.HasPrefix(uri, "document://") {
		return ""
	}
	idx := strings.Index(uri, marker)
	if idx < 0 {
		return ""
	}
	id := uri[idx+len(marker):]
	if cut := strings.IndexAny(id, "#?"); cut >= 0 {
		id = id[:cut]
	}
	return strings.TrimSpace(id)
}

func resourceIDFromURI(uri string) string {
	const marker = "/resources/"
	if !strings.HasPrefix(uri, "resource://") {
		return ""
	}
	idx := strings.Index(uri, marker)
	if idx < 0 {
		return ""
	}
	id := uri[idx+len(marker):]
	if cut := strings.IndexAny(id, "#?"); cut >= 0 {
		id = id[:cut]
	}
	return strings.TrimSpace(id)
}

func (s *SQLiteStore) findDocumentByTitleLocked(collectionID, target string) (string, bool, error) {
	name := normalizeLinkName(target)
	if name == "" {
		return "", false, nil
	}
	stmt, err := s.prepareLocked(`SELECT id FROM documents WHERE collection_id = ? AND deleted_at IS NULL AND title = ? COLLATE NOCASE ORDER BY id LIMIT 2`)
	if err != nil {
		return "", false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{collectionID, name}); err != nil {
		return "", false, err
	}
	ids := []string{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			ids = append(ids, columnText(stmt, 0))
		case C.SQLITE_DONE:
			if len(ids) == 0 {
				return "", false, nil
			}
			return ids[0], len(ids) > 1, nil
		default:
			return "", false, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) findResourceByFilenameLocked(collectionID, target string) (string, bool, error) {
	name := strings.TrimSpace(target)
	if name == "" {
		return "", false, nil
	}
	name = strings.TrimPrefix(filepath.Base(name), "/")
	stmt, err := s.prepareLocked(`SELECT id FROM resources WHERE collection_id = ? AND filename = ? COLLATE NOCASE ORDER BY id LIMIT 2`)
	if err != nil {
		return "", false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{collectionID, name}); err != nil {
		return "", false, err
	}
	ids := []string{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			ids = append(ids, columnText(stmt, 0))
		case C.SQLITE_DONE:
			if len(ids) == 0 {
				return "", false, nil
			}
			return ids[0], len(ids) > 1, nil
		default:
			return "", false, s.stepErrLocked(rc)
		}
	}
}

func normalizeLinkName(target string) string {
	name := strings.TrimSpace(target)
	name = strings.TrimSuffix(name, ".md")
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "%20", " ")
	return strings.TrimSpace(name)
}

func (s *SQLiteStore) ListDocumentLinks(ctx context.Context, documentID, direction string) (DocumentLinkPage, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DocumentLinkPage{}, err
	}
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return DocumentLinkPage{}, ErrNotFound
	}
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction == "" {
		direction = "both"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ok, err := s.documentExistsLocked(documentID); err != nil {
		return DocumentLinkPage{}, err
	} else if !ok {
		return DocumentLinkPage{}, ErrNotFound
	}
	page := DocumentLinkPage{Outgoing: []DocumentLink{}, Incoming: []DocumentLink{}}
	if direction == "outgoing" || direction == "both" {
		links, err := s.listLinksLocked(`WHERE source_document_id = ?`, documentID)
		if err != nil {
			return DocumentLinkPage{}, err
		}
		page.Outgoing = links
	}
	if direction == "incoming" || direction == "both" {
		links, err := s.listLinksLocked(`WHERE target_document_id = ?`, documentID)
		if err != nil {
			return DocumentLinkPage{}, err
		}
		page.Incoming = links
	}
	return page, nil
}

func (s *SQLiteStore) listLinksLocked(whereClause string, values ...string) ([]DocumentLink, error) {
	stmt, err := s.prepareLocked(`SELECT id, source_document_id, COALESCE(target_document_id, ''), COALESCE(target_resource_id, ''), COALESCE(target_uri, ''), relation_type, source_format, raw_target, COALESCE(display_text, ''), COALESCE(anchor_type, ''), COALESCE(anchor_value, ''), COALESCE(context, ''), COALESCE(source_start_byte, 0), COALESCE(source_end_byte, 0), COALESCE(source_line, 0), COALESCE(source_column, 0), resolution_status FROM document_links ` + whereClause + ` ORDER BY source_line, source_column, id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, values); err != nil {
		return nil, err
	}
	links := []DocumentLink{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			links = append(links, linkFromStmt(stmt))
		case C.SQLITE_DONE:
			return links, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func linkFromStmt(stmt *C.sqlite3_stmt) DocumentLink {
	return DocumentLink{
		ID:               columnInt64(stmt, 0),
		SourceDocumentID: columnText(stmt, 1),
		TargetDocumentID: columnText(stmt, 2),
		TargetResourceID: columnText(stmt, 3),
		TargetURI:        columnText(stmt, 4),
		RelationType:     columnText(stmt, 5),
		SourceFormat:     columnText(stmt, 6),
		RawTarget:        columnText(stmt, 7),
		DisplayText:      columnText(stmt, 8),
		AnchorType:       columnText(stmt, 9),
		AnchorValue:      columnText(stmt, 10),
		Context:          columnText(stmt, 11),
		SourceStartByte:  int(columnInt64(stmt, 12)),
		SourceEndByte:    int(columnInt64(stmt, 13)),
		SourceLine:       int(columnInt64(stmt, 14)),
		SourceColumn:     int(columnInt64(stmt, 15)),
		ResolutionStatus: columnText(stmt, 16),
	}
}

func (s *SQLiteStore) countLocked(sql string, values ...string) (int64, error) {
	stmt, err := s.prepareLocked(sql)
	if err != nil {
		return 0, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, values); err != nil {
		return 0, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return 0, nil
	}
	if rc != C.SQLITE_ROW {
		return 0, s.stepErrLocked(rc)
	}
	return columnInt64(stmt, 0), nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func MIMETypeFromFilename(filename string) string {
	if strings.TrimSpace(filename) == "" {
		return ""
	}
	return mime.TypeByExtension(filepath.Ext(filename))
}

func (s *SQLiteStore) readHitsLocked(stmt *C.sqlite3_stmt) (SearchResponse, error) {
	resp := SearchResponse{Hits: []SearchHit{}}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			collectionID := columnText(stmt, 1)
			id := columnText(stmt, 0)
			resp.Hits = append(resp.Hits, SearchHit{
				ID:           id,
				URI:          DocumentURI(collectionID, id),
				CollectionID: collectionID,
				Title:        columnText(stmt, 2),
				Snippet:      columnText(stmt, 3),
				Score:        columnFloat(stmt, 4),
				NotebookID:   columnText(stmt, 5),
				UpdatedAt:    parseSQLiteTime(columnText(stmt, 6)),
				sortTime:     columnText(stmt, 6),
			})
		case C.SQLITE_DONE:
			return resp, nil
		default:
			return SearchResponse{}, s.stepErrLocked(rc)
		}
	}
}

func parseSQLiteTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(value))
	return parsed
}

func (s *SQLiteStore) execPreparedLocked(sql string, values ...string) error {
	stmt, err := s.prepareLocked(sql)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, values); err != nil {
		return err
	}
	rc := C.sqlite3_step(stmt)
	if rc != C.SQLITE_DONE {
		return s.stepErrLocked(rc)
	}
	return nil
}

func (s *SQLiteStore) prepareLocked(sql string) (*C.sqlite3_stmt, error) {
	if s.db == nil {
		return nil, fmt.Errorf("sqlite store is closed")
	}
	csql := C.CString(sql)
	defer C.free(unsafe.Pointer(csql))
	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(s.db, csql, -1, &stmt, nil); rc != C.SQLITE_OK {
		return nil, fmt.Errorf("sqlite prepare: %s", C.GoString(C.sqlite3_errmsg(s.db)))
	}
	return stmt, nil
}

func bindAll(stmt *C.sqlite3_stmt, values []string) error {
	for i, value := range values {
		idx := C.int(i + 1)
		cvalue := C.CString(value)
		rc := C.notes_sqlite_bind_text(stmt, idx, cvalue)
		C.free(unsafe.Pointer(cvalue))
		if rc != C.SQLITE_OK {
			return fmt.Errorf("sqlite bind parameter %d failed", i+1)
		}
	}
	return nil
}

func (s *SQLiteStore) stepErrLocked(rc C.int) error {
	return fmt.Errorf("sqlite step rc=%d: %s", int(rc), C.GoString(C.sqlite3_errmsg(s.db)))
}

func columnText(stmt *C.sqlite3_stmt, index int) string {
	text := C.sqlite3_column_text(stmt, C.int(index))
	if text == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(text)))
}

func columnFloat(stmt *C.sqlite3_stmt, index int) float64 {
	return float64(C.sqlite3_column_double(stmt, C.int(index)))
}

func columnInt64(stmt *C.sqlite3_stmt, index int) int64 {
	return int64(C.sqlite3_column_int64(stmt, C.int(index)))
}
