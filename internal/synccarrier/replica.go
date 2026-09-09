package synccarrier

import (
	"context"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncstate"
)

// StoreReplica adapts the canonical SQLite store to the Replica surface a round
// uses.
//
// It is the only file in this package that knows a database exists. The
// protocol above is written against interfaces so it can be exercised without
// one, and so that G14's REST data plane can reuse it unchanged.
type StoreReplica struct{ store *store.SQLiteStore }

// NewStoreReplica wraps a store.
func NewStoreReplica(canonical *store.SQLiteStore) *StoreReplica {
	return &StoreReplica{store: canonical}
}

func (r *StoreReplica) LocalSyncHandshake(ctx context.Context) (syncstate.Handshake, error) {
	return r.store.LocalSyncHandshake(ctx)
}

func (r *StoreReplica) SyncStateVector(ctx context.Context) (syncstate.Vector, error) {
	return r.store.SyncStateVector(ctx)
}

func (r *StoreReplica) SyncPeerEnrolled(ctx context.Context, replicaID string) (bool, error) {
	return r.store.SyncPeerEnrolled(ctx, replicaID)
}

func (r *StoreReplica) EnrolledPeers(ctx context.Context) ([]PeerState, error) {
	remembered, err := r.store.ListSyncPeers(ctx)
	if err != nil {
		return nil, err
	}
	peers := make([]PeerState, 0, len(remembered))
	for _, peer := range remembered {
		peers = append(peers, PeerState{Handshake: peer.Handshake})
	}
	return peers, nil
}

func (r *StoreReplica) ListSyncOperations(ctx context.Context, replicaID string, afterSequence int64, limit int) ([]syncstate.Operation, error) {
	return r.store.ListSyncOperations(ctx, replicaID, afterSequence, limit)
}

func (r *StoreReplica) AdmitSyncOperations(ctx context.Context, peer syncstate.Handshake, operations []syncstate.Operation) (AdmissionResult, error) {
	admission, err := r.store.AdmitSyncOperations(ctx, peer, operations)
	if err != nil {
		return AdmissionResult{}, err
	}
	return AdmissionResult{
		Received: admission.Received, Admitted: admission.Admitted,
		Pending: admission.Pending, Duplicates: admission.Duplicates,
	}, nil
}

func (r *StoreReplica) RecordSyncPeerAcknowledgement(ctx context.Context, peer syncstate.Handshake) error {
	return r.store.RecordSyncPeerAcknowledgement(ctx, peer)
}

func (r *StoreReplica) WantedSyncObjects(ctx context.Context, limit int) ([]WantedObject, error) {
	unavailable, err := r.store.UnavailableBlobs(ctx, limit)
	if err != nil {
		return nil, err
	}
	wanted := make([]WantedObject, 0, len(unavailable))
	for _, blob := range unavailable {
		wanted = append(wanted, WantedObject{BlobSHA256: blob.SHA256, ByteLength: blob.ByteLength})
	}
	return wanted, nil
}
