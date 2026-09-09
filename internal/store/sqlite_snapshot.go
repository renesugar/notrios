package store

/*
#include "csqlite/sqlite3.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
	"unsafe"

	"github.com/renesugar/notrios/internal/syncstate"
)

// SnapshotLocalTables are copied by SQLite's Online Backup API and then
// securely emptied in the private image before it can be published. They are
// resumptions, local observations, or in-flight protocol state; none is
// canonical library history.
var SnapshotLocalTables = []string{
	"jobs",
	"batch_operations",
	"restore_state",
	"import_checkpoints",
	"import_item_states",
	"sync_pairing_invitations",
	"sync_catchup_permissions",
	"sync_catchup_sessions",
	"sync_revision_transfer",
	"sync_apply_guard",
	"sync_journal_capture",
	"sync_pending_admissions",
	"sync_peer_acknowledgements",
	"sync_verified_snapshot_vectors",
	"sync_verified_snapshots",
	"sync_state_gaps",
	"sync_blob_chunks",
	"sync_blob_sources",
	"sync_blob_materialization",
}

// SQLiteSnapshotState is the state bound by a physical-snapshot manifest.
// Floors are kept separate from the observed vector: a floor is allowed to
// stand in for compacted operation history, while a vector is an observer's
// durable contiguous acknowledgement.
type SQLiteSnapshotState struct {
	SchemaVersion   int
	DatabaseID      string
	SourceReplicaID string
	Vector          syncstate.Vector
	Floors          syncstate.Vector
}

// SnapshotExternalObject names one immutable filesystem object claimed by the
// copied SQLite image. StoragePath is store-relative and must not cross a
// process or network boundary without validation by the snapshot package.
type SnapshotExternalObject struct {
	StoragePath string
	SHA256      string
	SizeBytes   int64
	Kind        string
}

// CreateSQLiteSnapshotImage uses SQLite's Online Backup API to make a
// transactionally consistent standalone image. The caller supplies a private
// temporary path and is responsible for fsync and publication. An interrupted
// image is restarted; this API has no page-level resume contract.
func (s *SQLiteStore) CreateSQLiteSnapshotImage(ctx context.Context, target string) (SQLiteSnapshotState, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SQLiteSnapshotState{}, err
	}
	if target == "" || target == ":memory:" {
		return SQLiteSnapshotState{}, fmt.Errorf("%w: snapshot image path is required", ErrInvalidInput)
	}
	if _, err := os.Lstat(target); err == nil {
		return SQLiteSnapshotState{}, fmt.Errorf("%w: snapshot image target already exists", ErrConflict)
	} else if !os.IsNotExist(err) {
		return SQLiteSnapshotState{}, err
	}

	cTarget := C.CString(target)
	defer C.free(unsafe.Pointer(cTarget))
	var destination *C.sqlite3
	flags := C.int(C.SQLITE_OPEN_READWRITE | C.SQLITE_OPEN_CREATE | C.SQLITE_OPEN_EXCLUSIVE | C.SQLITE_OPEN_FULLMUTEX)
	if rc := C.sqlite3_open_v2(cTarget, &destination, flags, nil); rc != C.SQLITE_OK {
		message := "unknown error"
		if destination != nil {
			message = C.GoString(C.sqlite3_errmsg(destination))
			C.sqlite3_close(destination)
		}
		return SQLiteSnapshotState{}, fmt.Errorf("open snapshot sqlite: %s", message)
	}
	closed := false
	defer func() {
		if !closed {
			C.sqlite3_close(destination)
		}
	}()

	s.mu.Lock()
	if s.db == nil {
		s.mu.Unlock()
		return SQLiteSnapshotState{}, fmt.Errorf("sqlite store is closed")
	}
	cMain := C.CString("main")
	backup := C.sqlite3_backup_init(destination, cMain, s.db, cMain)
	C.free(unsafe.Pointer(cMain))
	if backup == nil {
		message := C.GoString(C.sqlite3_errmsg(destination))
		s.mu.Unlock()
		return SQLiteSnapshotState{}, fmt.Errorf("start sqlite online backup: %s", message)
	}
	backupErr := stepSQLiteBackup(ctx, destination, backup)
	finishRC := C.sqlite3_backup_finish(backup)
	s.mu.Unlock()
	if backupErr != nil {
		return SQLiteSnapshotState{}, backupErr
	}
	if finishRC != C.SQLITE_OK {
		return SQLiteSnapshotState{}, fmt.Errorf("finish sqlite online backup: %s", C.GoString(C.sqlite3_errmsg(destination)))
	}

	copyStore := &SQLiteStore{db: destination, path: target}
	if err := copyStore.sanitizeSnapshotImageLocked(); err != nil {
		return SQLiteSnapshotState{}, err
	}
	state, err := copyStore.snapshotStateLocked()
	if err != nil {
		return SQLiteSnapshotState{}, err
	}
	if err := copyStore.integrityCheckLocked(); err != nil {
		return SQLiteSnapshotState{}, err
	}
	if rc := C.sqlite3_close(destination); rc != C.SQLITE_OK {
		return SQLiteSnapshotState{}, fmt.Errorf("close snapshot sqlite: %s", C.GoString(C.sqlite3_errmsg(destination)))
	}
	closed = true
	return state, nil
}

func stepSQLiteBackup(ctx context.Context, destination *C.sqlite3, backup *C.sqlite3_backup) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch rc := C.sqlite3_backup_step(backup, 128); rc {
		case C.SQLITE_DONE:
			return nil
		case C.SQLITE_OK:
			continue
		case C.SQLITE_BUSY, C.SQLITE_LOCKED:
			timer := time.NewTimer(10 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		default:
			return fmt.Errorf("sqlite online backup: %s", C.GoString(C.sqlite3_errmsg(destination)))
		}
	}
}

func (s *SQLiteStore) sanitizeSnapshotImageLocked() error {
	// secure_delete prevents local paths, invitation hashes, and pending bytes
	// from surviving in free pages after their rows are cleared.
	if err := s.execLocked("PRAGMA foreign_keys = OFF; PRAGMA secure_delete = ON; BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	for _, table := range SnapshotLocalTables {
		if err := s.execLocked("DELETE FROM " + table); err != nil {
			return fmt.Errorf("clear snapshot-local table %s: %w", table, err)
		}
	}
	// Complete manifests are immutable transfer metadata used when a local blob
	// is advertised again. Incomplete manifests are resumable staging state.
	if err := s.execLocked("DELETE FROM sync_blob_manifests WHERE complete = 0"); err != nil {
		return fmt.Errorf("clear incomplete snapshot blob manifests: %w", err)
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	// Online Backup necessarily copies the source freelist too. secure_delete
	// scrubs the rows removed above, while VACUUM rebuilds the image so older
	// discarded importer paths or transient bytes cannot survive in freelist
	// pages from before this snapshot operation.
	if err := s.execLocked("VACUUM"); err != nil {
		return fmt.Errorf("compact sanitized snapshot image: %w", err)
	}
	// The selected representation is one standalone database file. Force any
	// copied WAL mode back to DELETE before the handle closes.
	return s.execLocked("PRAGMA journal_mode = DELETE")
}

// OpenSQLiteSnapshotReadOnly opens an already-created image without creating
// directories, enabling WAL, running migrations, or mutating canonical state.
func OpenSQLiteSnapshotReadOnly(path string) (*SQLiteStore, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var db *C.sqlite3
	flags := C.int(C.SQLITE_OPEN_READONLY | C.SQLITE_OPEN_FULLMUTEX)
	if rc := C.sqlite3_open_v2(cPath, &db, flags, nil); rc != C.SQLITE_OK {
		message := "unknown error"
		if db != nil {
			message = C.GoString(C.sqlite3_errmsg(db))
			C.sqlite3_close(db)
		}
		return nil, fmt.Errorf("open snapshot sqlite read-only: %s", message)
	}
	return &SQLiteStore{db: db, path: path}, nil
}

// InspectSQLiteSnapshot validates the database-internal admission boundary.
// Filesystem pack and manifest checks are owned by internal/snapshotimage.
func InspectSQLiteSnapshot(ctx context.Context, path string) (SQLiteSnapshotState, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SQLiteSnapshotState{}, err
	}
	image, err := OpenSQLiteSnapshotReadOnly(path)
	if err != nil {
		return SQLiteSnapshotState{}, err
	}
	defer image.Close()
	image.mu.Lock()
	defer image.mu.Unlock()
	if err := image.integrityCheckLocked(); err != nil {
		return SQLiteSnapshotState{}, err
	}
	if err := image.verifySnapshotSanitizationLocked(); err != nil {
		return SQLiteSnapshotState{}, err
	}
	return image.snapshotStateLocked()
}

func (s *SQLiteStore) integrityCheckLocked() error {
	stmt, err := s.prepareLocked("PRAGMA integrity_check")
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	rows := 0
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			rows++
			if rows != 1 || columnText(stmt, 0) != "ok" {
				return fmt.Errorf("sqlite integrity_check failed: %s", columnText(stmt, 0))
			}
		case C.SQLITE_DONE:
			if rows != 1 {
				return fmt.Errorf("sqlite integrity_check returned %d rows", rows)
			}
			return nil
		default:
			return s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) verifySnapshotSanitizationLocked() error {
	for _, table := range SnapshotLocalTables {
		count, err := s.countLocked("SELECT COUNT(*) FROM "+table, nil...)
		if err != nil {
			return fmt.Errorf("inspect snapshot-local table %s: %w", table, err)
		}
		if count != 0 {
			return fmt.Errorf("snapshot-local table %s is not empty", table)
		}
	}
	count, err := s.countLocked("SELECT COUNT(*) FROM sync_blob_manifests WHERE complete = 0", nil...)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("snapshot carries incomplete blob manifests")
	}
	return nil
}

func (s *SQLiteStore) snapshotStateLocked() (SQLiteSnapshotState, error) {
	schema, err := s.pragmaUserVersionLocked()
	if err != nil {
		return SQLiteSnapshotState{}, err
	}
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return SQLiteSnapshotState{}, err
	}
	vector, err := s.syncStateVectorLocked(identity.ReplicaID)
	if err != nil {
		return SQLiteSnapshotState{}, err
	}
	floors, err := s.snapshotFloorsLocked()
	if err != nil {
		return SQLiteSnapshotState{}, err
	}
	return SQLiteSnapshotState{
		SchemaVersion: schema, DatabaseID: identity.DatabaseID,
		SourceReplicaID: identity.ReplicaID, Vector: vector, Floors: floors,
	}, nil
}

func (s *SQLiteStore) snapshotFloorsLocked() (syncstate.Vector, error) {
	stmt, err := s.prepareLocked(`SELECT replica_id, MAX(sequence) FROM (
		SELECT replica_id, sequence FROM sync_catchup_floors
		UNION ALL
		SELECT replica_id, sequence FROM sync_retention_floors
	) GROUP BY replica_id ORDER BY replica_id LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{strconv.Itoa(syncstate.MaxStateVectorEntries + 1)}); err != nil {
		return nil, err
	}
	floors := syncstate.Vector{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			if len(floors) == syncstate.MaxStateVectorEntries {
				return nil, fmt.Errorf("%w: snapshot floors exceed limit", syncstate.ErrInvalidState)
			}
			floors[columnText(stmt, 0)] = columnInt64(stmt, 1)
		case C.SQLITE_DONE:
			return floors, syncstate.ValidateVector(floors)
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// SnapshotExternalObjects returns a bounded, storage-path ordered page of
// immutable objects claimed local by this image. The query de-duplicates source
// bundle items that share one content-addressed file.
func (s *SQLiteStore) SnapshotExternalObjects(ctx context.Context, after string, limit int) ([]SnapshotExternalObject, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("%w: snapshot object page limit must be 1-1000", ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`
		SELECT storage_path, sha256, size_bytes, kind FROM (
			SELECT storage_path, sha256, size_bytes, 'blob' AS kind
			  FROM blobs WHERE availability = 'local' AND storage_path <> ''
			UNION
			SELECT storage_path, sha256, size_bytes, 'source_bundle' AS kind
			  FROM source_bundle_items WHERE storage_path <> ''
		) WHERE storage_path > ? ORDER BY storage_path LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{after, strconv.Itoa(limit)}); err != nil {
		return nil, err
	}
	objects := make([]SnapshotExternalObject, 0, limit)
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			objects = append(objects, SnapshotExternalObject{
				StoragePath: columnText(stmt, 0), SHA256: columnText(stmt, 1),
				SizeBytes: columnInt64(stmt, 2), Kind: columnText(stmt, 3),
			})
		case C.SQLITE_DONE:
			return objects, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}
