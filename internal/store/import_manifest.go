package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImportManifestRecord is one opaque importer-owned payload in a temporary,
// indexed spool. SortKey defines deterministic batch order; LookupKey supports
// bounded relation lookups. The manifest is never canonical storage.
type ImportManifestRecord struct {
	Kind      string
	SortKey   string
	LookupKey string
	Payload   []byte
}

// ImportManifest is a private temporary SQLite spool used to keep large import
// inventories off the Go heap. Close removes its mktemp-owned directory.
type ImportManifest struct {
	store *SQLiteStore
	root  string
	path  string
}

func OpenImportManifest() (*ImportManifest, error) {
	root, err := os.MkdirTemp("", "notrios-import-manifest-*")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "manifest.sqlite")
	st, err := OpenSQLiteWithAssetStore(path, filepath.Join(root, "assets"))
	if err != nil {
		_ = os.RemoveAll(root)
		return nil, err
	}
	manifest := &ImportManifest{store: st, root: root, path: path}
	st.mu.Lock()
	err = st.execLocked(`CREATE TABLE manifest_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kind TEXT NOT NULL,
		sort_key TEXT NOT NULL,
		lookup_key TEXT NOT NULL,
		payload TEXT NOT NULL
	);
	CREATE INDEX manifest_records_order_idx ON manifest_records(kind, sort_key, lookup_key, id);
	CREATE INDEX manifest_records_lookup_idx ON manifest_records(kind, lookup_key, sort_key, id);`)
	st.mu.Unlock()
	if err != nil {
		_ = manifest.Close()
		return nil, err
	}
	return manifest, nil
}

func (m *ImportManifest) Close() error {
	if m == nil {
		return nil
	}
	var closeErr error
	if m.store != nil {
		closeErr = m.store.Close()
		m.store = nil
	}
	removeErr := os.RemoveAll(m.root)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

func (m *ImportManifest) Put(ctx context.Context, records []ImportManifestRecord) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	if len(records) > maxImportLookupItems {
		return fmt.Errorf("%w: manifest batch contains %d items; maximum is %d", ErrInvalidInput, len(records), maxImportLookupItems)
	}
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	if err := m.store.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = m.store.execLocked("ROLLBACK")
		}
	}()
	stmt, err := m.store.prepareLocked(`INSERT INTO manifest_records(kind, sort_key, lookup_key, payload) VALUES(?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	for index, record := range records {
		if strings.TrimSpace(record.Kind) == "" || strings.TrimSpace(record.SortKey) == "" {
			return fmt.Errorf("%w: manifest kind and sort key are required", ErrInvalidInput)
		}
		if index > 0 {
			if rc := C.sqlite3_reset(stmt); rc != C.SQLITE_OK {
				return m.store.stepErrLocked(rc)
			}
			if rc := C.sqlite3_clear_bindings(stmt); rc != C.SQLITE_OK {
				return m.store.stepErrLocked(rc)
			}
		}
		if err := bindAll(stmt, []string{record.Kind, record.SortKey, record.LookupKey, string(record.Payload)}); err != nil {
			return err
		}
		if rc := C.sqlite3_step(stmt); rc != C.SQLITE_DONE {
			return m.store.stepErrLocked(rc)
		}
	}
	if err := m.store.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// BatchAfter returns the next deterministic page after a sort key. Importers
// use this for sequential passes so cost does not grow with the page offset.
func (m *ImportManifest) BatchAfter(ctx context.Context, kind, afterSortKey string, limit int) ([]ImportManifestRecord, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxImportLookupItems {
		return nil, fmt.Errorf("%w: manifest limit is invalid", ErrInvalidInput)
	}
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	stmt, err := m.store.prepareLocked(`SELECT kind, sort_key, lookup_key, payload
		FROM manifest_records WHERE kind = ? AND sort_key > ? ORDER BY sort_key, lookup_key, id LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{kind, afterSortKey, fmt.Sprintf("%d", limit)}); err != nil {
		return nil, err
	}
	return manifestRecordsFromStmt(m.store, stmt)
}

func (m *ImportManifest) Batch(ctx context.Context, kind string, offset, limit int) ([]ImportManifestRecord, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if offset < 0 || limit <= 0 || limit > maxImportLookupItems {
		return nil, fmt.Errorf("%w: manifest offset/limit is invalid", ErrInvalidInput)
	}
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	stmt, err := m.store.prepareLocked(`SELECT kind, sort_key, lookup_key, payload
		FROM manifest_records WHERE kind = ? ORDER BY sort_key, lookup_key, id LIMIT ? OFFSET ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{kind, fmt.Sprintf("%d", limit), fmt.Sprintf("%d", offset)}); err != nil {
		return nil, err
	}
	return manifestRecordsFromStmt(m.store, stmt)
}

func (m *ImportManifest) Lookup(ctx context.Context, kind string, lookupKeys []string) (map[string][]ImportManifestRecord, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string][]ImportManifestRecord, len(lookupKeys))
	if len(lookupKeys) == 0 {
		return result, nil
	}
	if err := validateLookupItems(lookupKeys); err != nil {
		return nil, err
	}
	args := append([]string{kind}, lookupKeys...)
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	stmt, err := m.store.prepareLocked(`SELECT kind, sort_key, lookup_key, payload
		FROM manifest_records WHERE kind = ? AND lookup_key IN (` + lookupPlaceholders(len(lookupKeys)) + `)
		ORDER BY lookup_key, sort_key, id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, args); err != nil {
		return nil, err
	}
	records, err := manifestRecordsFromStmt(m.store, stmt)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		result[record.LookupKey] = append(result[record.LookupKey], record)
	}
	return result, nil
}

func manifestRecordsFromStmt(st *SQLiteStore, stmt *C.sqlite3_stmt) ([]ImportManifestRecord, error) {
	records := []ImportManifestRecord{}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			records = append(records, ImportManifestRecord{
				Kind: columnText(stmt, 0), SortKey: columnText(stmt, 1), LookupKey: columnText(stmt, 2), Payload: []byte(columnText(stmt, 3)),
			})
		case C.SQLITE_DONE:
			return records, nil
		default:
			return nil, st.stepErrLocked(rc)
		}
	}
}

func (m *ImportManifest) SizeBytes() int64 {
	if m == nil || m.path == "" {
		return 0
	}
	var total int64
	for _, path := range []string{m.path, m.path + "-wal", m.path + "-shm"} {
		if info, err := os.Stat(path); err == nil {
			total += info.Size()
		}
	}
	return total
}
