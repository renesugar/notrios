package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"strconv"

	"github.com/renesugar/notrios/internal/syncassets"
	"github.com/renesugar/notrios/internal/syncstate"
)

// SyncPeerEnrolled reports whether a replica has been explicitly configured for
// admission on this database.
//
// G11's carrier needs it to keep pairing an explicit act. A shared folder is a
// place anyone holding the folder can write to, so a round has to be able to
// say "this peer is present but not enrolled" and report it, rather than
// discovering the fact by having its admission refused and guessing why.
func (s *SQLiteStore) SyncPeerEnrolled(ctx context.Context, replicaID string) (bool, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if replicaID == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return false, err
	}
	compatibility, err := s.syncPeerCompatibilityLocked(replicaID)
	if err != nil || compatibility == nil {
		return false, err
	}
	role, databaseID, status, found, err := s.syncReplicaLocked(replicaID)
	if err != nil {
		return false, err
	}
	return found && role == "peer" && status == "active" && databaseID == local.DatabaseID, nil
}

// SyncPeerState is an enrolled peer as this replica durably remembers it: the
// compatibility that was configured, and the last acknowledgement it recorded.
//
// The acknowledgement is why this exists. A carrier is disposable, so after it
// is deleted there is nothing on it to say what a peer already has — but the
// journal remembers, and that memory is what lets a replica publish exactly the
// missing work into an empty folder instead of waiting to be asked.
type SyncPeerState struct {
	Handshake syncstate.Handshake
}

// ListSyncPeers returns every explicitly enrolled, active peer of this
// database, each carrying the state vector it last acknowledged.
func (s *SQLiteStore) ListSyncPeers(ctx context.Context) ([]SyncPeerState, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return nil, err
	}
	stmt, err := s.prepareLocked(`SELECT replica_id FROM sync_replicas
		WHERE role = 'peer' AND status = 'active' AND database_id = ? ORDER BY replica_id`)
	if err != nil {
		return nil, err
	}
	replicaIDs := []string{}
	if err := bindAll(stmt, []string{local.DatabaseID}); err != nil {
		C.sqlite3_finalize(stmt)
		return nil, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_ROW {
			replicaIDs = append(replicaIDs, columnText(stmt, 0))
			continue
		}
		C.sqlite3_finalize(stmt)
		if rc != C.SQLITE_DONE {
			return nil, s.stepErrLocked(rc)
		}
		break
	}

	peers := make([]SyncPeerState, 0, len(replicaIDs))
	for _, replicaID := range replicaIDs {
		compatibility, err := s.syncPeerCompatibilityLocked(replicaID)
		if err != nil {
			return nil, err
		}
		if compatibility == nil {
			continue
		}
		handshake := *compatibility
		handshake.DatabaseID = local.DatabaseID
		handshake.StateVector, err = s.syncPeerAcknowledgedVectorLocked(replicaID)
		if err != nil {
			return nil, err
		}
		peers = append(peers, SyncPeerState{Handshake: handshake})
	}
	return peers, nil
}

func (s *SQLiteStore) syncPeerAcknowledgedVectorLocked(peerReplicaID string) (syncstate.Vector, error) {
	stmt, err := s.prepareLocked(`SELECT subject_replica_id, contiguous_sequence
		FROM sync_peer_acknowledgements WHERE peer_replica_id = ? ORDER BY subject_replica_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{peerReplicaID}); err != nil {
		return nil, err
	}
	vector := syncstate.Vector{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			vector[columnText(stmt, 0)] = columnInt64(stmt, 1)
		case C.SQLITE_DONE:
			return vector, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// UnavailableBlob is one resource whose metadata this replica holds and whose
// bytes it does not.
type UnavailableBlob struct {
	SHA256     string
	ByteLength int64
}

// UnavailableBlobs lists resources a transport could usefully ask a peer for.
// It returns hashes and lengths only: a caller publishing this to a carrier
// must not be able to leak a filename or a MIME type by accident.
func (s *SQLiteStore) UnavailableBlobs(ctx context.Context, limit int) ([]UnavailableBlob, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > syncassets.MaxConcurrentFetches {
		limit = syncassets.MaxConcurrentFetches
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT sha256, size_bytes FROM blobs
		WHERE availability = 'unavailable' ORDER BY sha256 LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{strconv.Itoa(limit)}); err != nil {
		return nil, err
	}
	blobs := []UnavailableBlob{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			blobs = append(blobs, UnavailableBlob{SHA256: columnText(stmt, 0), ByteLength: columnInt64(stmt, 1)})
		case C.SQLITE_DONE:
			return blobs, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}
