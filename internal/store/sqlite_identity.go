package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"fmt"
	"time"
)

func (s *SQLiteStore) ensureDatabaseIdentity(ctx context.Context) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	databaseID, err := NewID("db")
	if err != nil {
		return err
	}
	replicaID, err := NewID("replica")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedDatabaseID = ""
	return s.execPreparedLocked(`INSERT OR IGNORE INTO database_identity(singleton, database_id, replica_id)
		VALUES(1, ?, ?)`, databaseID, replicaID)
}

// databaseIDLocked returns the logical database ID, reading it once. Stable
// links are resolved while rebuilding a note's links, which happens for every
// note of an import; a query per link would be pure overhead for a value that
// changes only under an explicit identity operation.
func (s *SQLiteStore) databaseIDLocked() (string, error) {
	if s.cachedDatabaseID != "" {
		return s.cachedDatabaseID, nil
	}
	identity, err := s.getDatabaseIdentityLocked()
	if err != nil {
		return "", err
	}
	s.cachedDatabaseID = identity.DatabaseID
	return s.cachedDatabaseID, nil
}

// GetDatabaseIdentity returns stable canonical identifiers and never derives
// them from the SQLite path, hostname, profile, or asset directory.
func (s *SQLiteStore) GetDatabaseIdentity(ctx context.Context) (DatabaseIdentity, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DatabaseIdentity{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDatabaseIdentityLocked()
}

// RotateReplicaIdentity marks this SQLite copy as a newly writable replica.
// DatabaseID remains unchanged. Restore/clone code must call this explicitly;
// raw filesystem copies cannot be detected safely from inside SQLite.
func (s *SQLiteStore) RotateReplicaIdentity(ctx context.Context) (DatabaseIdentity, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DatabaseIdentity{}, err
	}
	replicaID, err := NewID("replica")
	if err != nil {
		return DatabaseIdentity{}, err
	}
	auditID, err := NewID("audit")
	if err != nil {
		return DatabaseIdentity{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return DatabaseIdentity{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.retireLocalJournalLocked(auditID, replicaID, "replica identity rotated"); err != nil {
		return DatabaseIdentity{}, err
	}
	if err := s.execPreparedLocked(`UPDATE database_identity
		SET replica_id = ?, replica_created_at = CURRENT_TIMESTAMP
		WHERE singleton = 1`, replicaID); err != nil {
		return DatabaseIdentity{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return DatabaseIdentity{}, err
	}
	committed = true
	return s.getDatabaseIdentityLocked()
}

func (s *SQLiteStore) getDatabaseIdentityLocked() (DatabaseIdentity, error) {
	stmt, err := s.prepareLocked(`SELECT database_id, replica_id, created_at, replica_created_at
		FROM database_identity WHERE singleton = 1`)
	if err != nil {
		return DatabaseIdentity{}, err
	}
	defer C.sqlite3_finalize(stmt)
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return DatabaseIdentity{}, fmt.Errorf("%w: database identity", ErrNotFound)
	}
	if rc != C.SQLITE_ROW {
		return DatabaseIdentity{}, s.stepErrLocked(rc)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 2)))
	if err != nil {
		return DatabaseIdentity{}, fmt.Errorf("parse database identity created_at: %w", err)
	}
	replicaCreatedAt, err := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 3)))
	if err != nil {
		return DatabaseIdentity{}, fmt.Errorf("parse replica identity created_at: %w", err)
	}
	return DatabaseIdentity{
		DatabaseID:       columnText(stmt, 0),
		ReplicaID:        columnText(stmt, 1),
		CreatedAt:        createdAt,
		ReplicaCreatedAt: replicaCreatedAt,
	}, nil
}
