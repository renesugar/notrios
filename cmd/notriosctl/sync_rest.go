package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/synckeys"
	"github.com/renesugar/notrios/internal/syncrest"
	"github.com/renesugar/notrios/internal/syncwire"
)

// peerClient builds a signing client for a peer's sync surface.
func peerClient(st *store.SQLiteStore, keys *synckeys.KeyFile, status store.SyncJournalStatus,
	databaseID, peerURL string) *syncauth.Client {
	return &syncauth.Client{
		BaseURL: peerURL, DatabaseID: databaseID, ReplicaID: status.ReplicaID,
		SignerKeyID: keys.SignerKeyID(), Private: keys.PrivateSigningKey(),
	}
}

// runSyncPush exchanges with a peer over its authenticated sync surface.
//
// It is `sync once` with a different courier, and deliberately the same round:
// the peer moves artifacts and this replica merges them, exactly as it does
// through a folder.
func runSyncPush(args []string) {
	flags := newSyncFlags("exchange")
	peerURL := flags.set.String("url", "", "the peer's base URL")
	materialize := flags.set.Int("materialize", 16, "at most this many attachment objects to fetch afterwards")
	flags.parse(args)
	if strings.TrimSpace(*peerURL) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync exchange --url <base-url> [--materialize N]")
		os.Exit(2)
	}
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	requireEnrolled(status)
	keys := mustOpenKeys(flags.keyPath(databaseID))
	group, err := keys.Current()
	if err != nil {
		exit(err)
	}
	client := peerClient(st, keys, status, databaseID, *peerURL)
	carrier := syncrest.New(client, group, status.ReplicaID)
	verifier := syncwire.MultiVerifier{keys, st.PeerVerifier()}
	round := synccarrier.NewRound(synccarrier.NewStoreReplica(st), carrier, keys, keys, verifier,
		store.NewLocalObjectProvider(st), synccarrier.Options{})

	result, err := round.Run(context.Background())
	if err != nil {
		exit(err)
	}
	report := map[string]any{"exchange": result}
	if *materialize > 0 {
		provider := synccarrier.NewProvider(carrier, keys, verifier, syncwire.Limits{})
		materialized, err := st.MaterializeResources(context.Background(), provider, *materialize)
		if err != nil {
			exit(err)
		}
		report["resources"] = materialized
	}
	printJSON(report)
}

// runSyncFetchBackup downloads a snapshot from a peer that is permitted to give
// one, resuming until it is complete, and verifies it as an archive.
//
// It stops there on purpose. Restoring is the existing explicit act —
// `notriosctl restore archive-v2 --intent ...` — because deciding what a
// restore does to a library is not something a download should decide.
func runSyncFetchBackup(args []string) {
	flags := newSyncFlags("fetch-backup")
	peerURL := flags.set.String("url", "", "the peer's base URL")
	out := flags.set.String("out", "", "directory to place the verified archive in")
	chunk := flags.set.Int64("chunk-bytes", 4<<20, "how much to fetch per request")
	flags.parse(args)
	if strings.TrimSpace(*peerURL) == "" || strings.TrimSpace(*out) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync fetch-backup --url <base-url> --out <dir>")
		os.Exit(2)
	}
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	requireEnrolled(status)
	keys := mustOpenKeys(flags.keyPath(databaseID))
	client := peerClient(st, keys, status, databaseID, *peerURL)
	ctx := context.Background()

	backup, err := syncrest.RequestBackup(ctx, client)
	if err != nil {
		exit(err)
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		exit(err)
	}
	sealed := filepath.Join(*out, "snapshot.nbk")
	started := time.Now()
	requests := 0
	for {
		written, complete, err := syncrest.DownloadBackup(ctx, client, backup, sealed, *chunk)
		if err != nil {
			exit(err)
		}
		requests++
		if complete {
			break
		}
		if requests%16 == 0 {
			fmt.Fprintf(os.Stderr, "  %d of %d bytes\n", written, backup.SealedBytes)
		}
	}
	archiveDir, report, err := syncrest.OpenBackup(backup, sealed, *out, keys,
		syncwire.MultiVerifier{keys, st.PeerVerifier()})
	if err != nil {
		exit(err)
	}
	// The sealed file has served its purpose and is a complete copy of somebody's
	// library; leaving it beside the extracted archive doubles that exposure.
	_ = os.Remove(sealed)
	printJSON(map[string]any{
		"archive":         archiveDir,
		"sealed_bytes":    backup.SealedBytes,
		"requests":        requests,
		"elapsed_ms":      time.Since(started).Milliseconds(),
		"snapshot_vector": backup.SnapshotVector,
		"verified": map[string]any{
			"database_id":   report.DatabaseID,
			"snapshot_id":   report.SnapshotID,
			"commit_sha256": report.CommitSHA256,
			"records":       report.Records,
			"objects":       report.Objects,
		},
		"next": "notriosctl restore archive-v2 --intent adopt --db <this library> " + archiveDir,
	})
}
