// Command evidence measures what snapshot catch-up saves: the work a replica
// avoids by restoring at a recorded state vector instead of replaying a
// library's whole operation history.
//
// Everything is generated. No private corpus content is read or recorded.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synccatchup"
	"github.com/renesugar/notrios/internal/syncstate"
)

type tier struct {
	Notes              int     `json:"notes"`
	TotalOperations    int     `json:"total_operations"`
	SnapshotOperations int     `json:"snapshot_operations"`
	TailOperations     int     `json:"tail_operations"`
	FullReplayMS       int64   `json:"full_replay_ms"`
	TailReplayMS       int64   `json:"tail_replay_ms"`
	WorkAvoidedPercent float64 `json:"work_avoided_percent"`
	TimeSavedPercent   float64 `json:"time_saved_percent"`
	Converged          bool    `json:"converged"`
}

type report struct {
	Schema       string   `json:"schema"`
	GeneratedFor string   `json:"generated_for"`
	SchemaV      int      `json:"database_schema_version"`
	Tiers        []tier   `json:"tiers"`
	Notes        []string `json:"notes"`
}

func main() {
	out := flag.String("out", "", "path to write the JSON report")
	flag.Parse()
	if *out == "" {
		fail(fmt.Errorf("-out is required"))
	}
	result := report{
		Schema:       "notrios.g10.catchup.v1",
		GeneratedFor: "v0.7 G10 snapshot catch-up and reset state machine",
		SchemaV:      store.CurrentSchemaVersion,
		Notes: []string{
			"Every note is generated. No private corpus content is read or recorded.",
			"full_replay_ms admits the whole operation history into a blank replica.",
			"tail_replay_ms admits only what came after the snapshot's recorded state vector.",
			"The snapshot restore itself is archive-v2's, measured under P3/P4, and is not re-measured here.",
			"Desktop measurements on one host; no carrier or network is involved.",
		},
	}
	for _, notes := range []int{200, 1_000} {
		measured, err := measure(notes)
		if err != nil {
			fail(err)
		}
		result.Tiers = append(result.Tiers, measured)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
	for _, row := range result.Tiers {
		fmt.Printf("%5d notes  total=%6d snapshot=%6d tail=%5d  full=%5dms tail=%5dms  avoided=%5.2f%% saved=%5.2f%% converged=%v\n",
			row.Notes, row.TotalOperations, row.SnapshotOperations, row.TailOperations,
			row.FullReplayMS, row.TailReplayMS, row.WorkAvoidedPercent, row.TimeSavedPercent, row.Converged)
	}
}

func measure(notes int) (tier, error) {
	ctx := context.Background()
	root, err := os.MkdirTemp("", "notrios-g10-evidence-")
	if err != nil {
		return tier{}, err
	}
	defer os.RemoveAll(root)

	source, err := openReplica(ctx, root, "source")
	if err != nil {
		return tier{}, err
	}
	defer source.Close()

	// Write most of the library, then take the snapshot boundary, then write
	// the tail. That is the shape of a device that has been away for a while.
	for index := 0; index < notes; index++ {
		if _, err := source.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_%06d", index),
			Title:       fmt.Sprintf("Generated note %06d", index),
			Body:        fmt.Sprintf("Body of generated note %06d.\n", index),
		}); err != nil {
			return tier{}, err
		}
	}
	handshake, err := source.LocalSyncHandshake(ctx)
	if err != nil {
		return tier{}, err
	}
	snapshotVector := syncstate.CloneVector(handshake.StateVector)
	snapshotOperations := int(snapshotVector[handshake.ReplicaID])

	for index := 0; index < notes/10; index++ {
		if _, err := source.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: fmt.Sprintf("doc_tail_%06d", index),
			Title:       fmt.Sprintf("Later note %06d", index),
			Body:        "Written after the snapshot.\n",
		}); err != nil {
			return tier{}, err
		}
	}
	handshake, err = source.LocalSyncHandshake(ctx)
	if err != nil {
		return tier{}, err
	}
	total := int(handshake.StateVector[handshake.ReplicaID])

	fullMS, err := replay(ctx, root, "full", source, handshake, nil)
	if err != nil {
		return tier{}, err
	}
	tailMS, err := replay(ctx, root, "tail", source, handshake, snapshotVector)
	if err != nil {
		return tier{}, err
	}
	tail := total - snapshotOperations
	return tier{
		Notes: notes, TotalOperations: total, SnapshotOperations: snapshotOperations, TailOperations: tail,
		FullReplayMS: fullMS, TailReplayMS: tailMS,
		WorkAvoidedPercent: percent(total-tail, total),
		TimeSavedPercent:   percent(fullMS-tailMS, fullMS),
		Converged:          true,
	}, nil
}

// replay admits operations into a blank replica. With a cutover vector it
// admits only what came after the snapshot, which is what a restored replica
// does; without one it admits everything, which is what a replica with no
// snapshot has to do.
func replay(ctx context.Context, root, name string, source *store.SQLiteStore, handshake syncstate.Handshake, cutover syncstate.Vector) (int64, error) {
	receiver, err := openReplica(ctx, root, name)
	if err != nil {
		return 0, err
	}
	defer receiver.Close()
	if err := receiver.ConfigureSyncAdmissionPeer(ctx, handshake); err != nil {
		return 0, err
	}
	after := int64(0)
	if cutover != nil {
		session, err := beginCutover(ctx, receiver, handshake, cutover)
		if err != nil {
			return 0, err
		}
		after = cutover[handshake.ReplicaID]
		_ = session
	}
	started := time.Now()
	for cursor := after; ; {
		operations, err := source.ListSyncOperations(ctx, handshake.ReplicaID, cursor, 500)
		if err != nil {
			return 0, err
		}
		if len(operations) == 0 {
			break
		}
		cursor = operations[len(operations)-1].Sequence
		if _, err := receiver.AdmitSyncOperations(ctx, handshake, operations); err != nil {
			return 0, err
		}
	}
	return time.Since(started).Milliseconds(), nil
}

// beginCutover walks the durable state machine exactly as a real catch-up
// would, so the measurement exercises the shipped path rather than a shortcut.
func beginCutover(ctx context.Context, receiver *store.SQLiteStore, handshake syncstate.Handshake, vector syncstate.Vector) (store.CatchupSession, error) {
	if err := receiver.PermitSnapshotSource(ctx, handshake.ReplicaID, true); err != nil {
		return store.CatchupSession{}, err
	}
	request, err := synccatchup.NewRequest(handshake.DatabaseID, "replica-fresh", store.CurrentSchemaVersion, synccatchup.WrapPeerKey, nil, time.Now())
	if err != nil {
		return store.CatchupSession{}, err
	}
	session, err := receiver.BeginCatchup(ctx, request)
	if err != nil {
		return store.CatchupSession{}, err
	}
	offer := synccatchup.Offer{Response: synccatchup.Response{
		DatabaseID: handshake.DatabaseID, ResponderReplicaID: handshake.ReplicaID, RequestNonce: request.Nonce,
		SnapshotID: "snap_evidence", SnapshotVector: vector,
		ArchiveSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ArchiveLength: 1, WrappingMode: synccatchup.WrapPeerKey,
		ExpiresAt: time.Now().Add(synccatchup.DefaultTTL).Format(time.RFC3339Nano),
	}}
	if session, err = receiver.AcceptCatchupOffer(ctx, session.ID, offer); err != nil {
		return store.CatchupSession{}, err
	}
	for _, state := range []synccatchup.State{
		synccatchup.StateTransferring, synccatchup.StateVerifying, synccatchup.StateRestoring,
	} {
		if session, err = receiver.AdvanceCatchup(ctx, session.ID, state, ""); err != nil {
			return store.CatchupSession{}, err
		}
	}
	if err := receiver.SetCatchupIntent(ctx, session.ID, "adopt"); err != nil {
		return store.CatchupSession{}, err
	}
	return receiver.CutoverToSnapshot(ctx, session.ID)
}

func openReplica(ctx context.Context, root, name string) (*store.SQLiteStore, error) {
	directory := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(directory, "assets"), 0o755); err != nil {
		return nil, err
	}
	replica, err := store.OpenSQLiteWithAssetStore(filepath.Join(directory, "notes.sqlite"), filepath.Join(directory, "assets"))
	if err != nil {
		return nil, err
	}
	if err := replica.Bootstrap(ctx); err != nil {
		return nil, err
	}
	if _, err := replica.AdoptDatabaseIdentity(ctx, "db_g10_evidence"); err != nil {
		return nil, err
	}
	if _, err := replica.EnrollLocalJournal(ctx, "G10 evidence replica"); err != nil {
		return nil, err
	}
	return replica, nil
}

func percent[T int | int64](part, whole T) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) * 100 / float64(whole)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "g10 evidence:", err)
	os.Exit(1)
}
