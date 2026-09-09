// Command evidence measures the shared-directory carrier: what a round costs,
// what the carrier holds, how long a bounded scan takes as the folder fills,
// and what deleting the whole carrier costs to recover from.
//
// It builds real replicas on real SQLite files and drives the shipped round
// through a real directory. Everything is generated; no private corpus content,
// path, or title is read or recorded.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

type tier struct {
	Notes                int     `json:"notes"`
	Operations           int     `json:"operations"`
	RoundsToConverge     int     `json:"rounds_to_converge"`
	FirstExchangeMS      int64   `json:"first_exchange_ms"`
	QuietRoundMS         int64   `json:"quiet_round_ms"`
	CarrierArtifacts     int     `json:"carrier_artifacts"`
	CarrierBytes         int64   `json:"carrier_bytes"`
	DatabaseBytes        int64   `json:"database_bytes"`
	CarrierOverheadRatio float64 `json:"carrier_bytes_over_database_bytes"`
	QuietRoundsAdded     int     `json:"artifacts_added_by_ten_quiet_rounds"`
	DirectExchangeMS     int64   `json:"direct_exchange_ms"`
	SealPublishMS        int64   `json:"seal_and_publish_ms"`
	ReadOpenMS           int64   `json:"read_and_open_ms"`
	CarrierLayerOps      int     `json:"carrier_layer_operations"`
	RepublishedNotes     int     `json:"notes_pending_when_carrier_deleted"`
	RecoveryRoundsAfter  int     `json:"rounds_to_reconverge_after_carrier_deleted"`
	RecoveryMS           int64   `json:"reconverge_after_delete_ms"`
	Converged            bool    `json:"converged"`
}

type scanTier struct {
	ArtifactsInOneClass int   `json:"artifacts_in_one_class"`
	ListMicroseconds    int64 `json:"list_microseconds"`
	Listed              int   `json:"listed"`
}

type report struct {
	Schema       string     `json:"schema"`
	GeneratedFor string     `json:"generated_for"`
	SchemaV      int        `json:"database_schema_version"`
	Tiers        []tier     `json:"tiers"`
	ScanBounds   []scanTier `json:"scan_bounds"`
	Notes        []string   `json:"notes"`
}

func main() {
	out := flag.String("out", "", "path to write the JSON report")
	flag.Parse()
	if *out == "" {
		fail(fmt.Errorf("-out is required"))
	}
	result := report{
		Schema:       "notrios.g11.carrier.v1",
		GeneratedFor: "v0.7 G11 ephemeral shared-directory protocol and peer discovery",
		SchemaV:      store.CurrentSchemaVersion,
		Notes: []string{
			"Every note is generated. No private corpus content is read or recorded.",
			"Two replicas of one database exchange through one real directory on the local filesystem.",
			"first_exchange_ms is the wall time of the rounds that carry a fresh library across.",
			"quiet_round_ms is one round with nothing new to say, which is what a poll costs.",
			"seal_and_publish_ms and read_and_open_ms isolate the carrier layer itself over carrier_layer_operations operations.",
			"direct_exchange_ms admits the same workload with no carrier at all; it is context, and one run of each is not a benchmark.",
			"carrier_bytes counts every artifact in the folder; the folder is disposable and never a backup.",
			"artifacts_added_by_ten_quiet_rounds must be zero: artifacts are named by what they are, not by their sealed bytes.",
			"Desktop measurements on one host and one filesystem; no cloud provider or removable device is involved (that is G12).",
		},
	}
	for _, notes := range []int{200, 1_000} {
		measured, err := measure(notes)
		if err != nil {
			fail(err)
		}
		result.Tiers = append(result.Tiers, measured)
	}
	bounds, err := measureScan()
	if err != nil {
		fail(err)
	}
	result.ScanBounds = bounds
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s\n", *out)
}

type replica struct {
	store   *store.SQLiteStore
	round   *synccarrier.Round
	carrier *synccarrier.Directory
	signer  *syncwire.MemorySigner
	id      string
	path    string
}

func measure(notes int) (tier, error) {
	ctx := context.Background()
	workspace, err := os.MkdirTemp("", "notrios-g11-")
	if err != nil {
		return tier{}, err
	}
	defer os.RemoveAll(workspace)
	carrierRoot := filepath.Join(workspace, "carrier")
	if err := os.MkdirAll(carrierRoot, 0o755); err != nil {
		return tier{}, err
	}

	keys, err := syncwire.NewMemoryKeyRing("key_g11_evidence")
	if err != nil {
		return tier{}, err
	}
	group, err := keys.Current()
	if err != nil {
		return tier{}, err
	}
	verifier := syncwire.NewMemoryVerifier()

	replicas := make([]*replica, 0, 2)
	handshakes := make([]syncstate.Handshake, 0, 2)
	for index := 0; index < 2; index++ {
		path := filepath.Join(workspace, fmt.Sprintf("replica-%d.sqlite", index))
		canonical, err := store.OpenSQLiteWithAssetStore(path, filepath.Join(workspace, fmt.Sprintf("assets-%d", index)))
		if err != nil {
			return tier{}, err
		}
		defer canonical.Close()
		if err := canonical.Bootstrap(ctx); err != nil {
			return tier{}, err
		}
		if _, err := canonical.AdoptDatabaseIdentity(ctx, "db_g11_evidence"); err != nil {
			return tier{}, err
		}
		if _, err := canonical.EnrollLocalJournal(ctx, "G11 evidence"); err != nil {
			return tier{}, err
		}
		handshake, err := canonical.LocalSyncHandshake(ctx)
		if err != nil {
			return tier{}, err
		}
		signer, err := syncwire.NewMemorySigner()
		if err != nil {
			return tier{}, err
		}
		verifier.Enroll(signer.Public())
		carrier, err := synccarrier.NewDirectory(carrierRoot, group, "db_g11_evidence", handshake.ReplicaID)
		if err != nil {
			return tier{}, err
		}
		replicas = append(replicas, &replica{
			store: canonical, carrier: carrier, signer: signer, id: handshake.ReplicaID, path: path,
			round: synccarrier.NewRound(synccarrier.NewStoreReplica(canonical), carrier, keys, signer,
				verifier, store.NewLocalObjectProvider(canonical), synccarrier.Options{}),
		})
		handshakes = append(handshakes, handshake)
	}
	for receiver := range replicas {
		for source := range replicas {
			if receiver == source {
				continue
			}
			if err := replicas[receiver].store.ConfigureSyncAdmissionPeer(ctx, handshakes[source]); err != nil {
				return tier{}, err
			}
		}
	}

	// Half the library on each side, so the measurement is an exchange rather
	// than a one-way push.
	for index := 0; index < notes; index++ {
		author := replicas[index%2]
		if _, err := author.store.CreateDocument(ctx, store.CreateDocumentRequest{
			Title: fmt.Sprintf("Generated note %d", index),
			Body:  strings.Repeat(fmt.Sprintf("generated line %d\n", index), 12),
		}); err != nil {
			return tier{}, err
		}
	}

	measurement := tier{Notes: notes}
	started := time.Now()
	rounds, err := converge(ctx, replicas)
	if err != nil {
		return tier{}, err
	}
	measurement.FirstExchangeMS = time.Since(started).Milliseconds()
	measurement.RoundsToConverge = rounds
	measurement.Converged, err = converged(ctx, replicas)
	if err != nil {
		return tier{}, err
	}

	quiet := time.Now()
	if _, err := replicas[0].round.Run(ctx); err != nil {
		return tier{}, err
	}
	measurement.QuietRoundMS = time.Since(quiet).Milliseconds()

	artifacts, bytes, err := carrierSize(carrierRoot)
	if err != nil {
		return tier{}, err
	}
	measurement.CarrierArtifacts = artifacts
	measurement.CarrierBytes = bytes
	if info, err := os.Stat(replicas[0].path); err == nil {
		measurement.DatabaseBytes = info.Size()
		if info.Size() > 0 {
			measurement.CarrierOverheadRatio = float64(bytes) / float64(info.Size())
		}
	}
	vector, err := replicas[0].store.SyncStateVector(ctx)
	if err != nil {
		return tier{}, err
	}
	for _, sequence := range vector {
		measurement.Operations += int(sequence)
	}

	// Ten more rounds with nothing new. A carrier that grows while a library is
	// idle is a carrier that fills a shared drive.
	for pass := 0; pass < 10; pass++ {
		for _, current := range replicas {
			if _, err := current.round.Run(ctx); err != nil {
				return tier{}, err
			}
		}
	}
	after, _, err := carrierSize(carrierRoot)
	if err != nil {
		return tier{}, err
	}
	measurement.QuietRoundsAdded = after - artifacts

	// Delete the carrier while work is in flight. A carrier deleted when both
	// replicas are already level costs nothing to recover from — there is
	// nothing anyone is missing — so measuring that would flatter the design.
	// This deletes it with a batch published and not yet read.
	measurement.RepublishedNotes = notes / 10
	for index := 0; index < measurement.RepublishedNotes; index++ {
		if _, err := replicas[0].store.CreateDocument(ctx, store.CreateDocumentRequest{
			Title: fmt.Sprintf("Republished note %d", index),
			Body:  strings.Repeat(fmt.Sprintf("republished line %d\n", index), 12),
		}); err != nil {
			return tier{}, err
		}
	}
	if _, err := replicas[0].round.Run(ctx); err != nil {
		return tier{}, err
	}
	if err := os.RemoveAll(filepath.Join(carrierRoot, "notrios-sync")); err != nil {
		return tier{}, err
	}
	recovery := time.Now()
	recovered, err := converge(ctx, replicas)
	if err != nil {
		return tier{}, err
	}
	measurement.RecoveryMS = time.Since(recovery).Milliseconds()
	measurement.RecoveryRoundsAfter = recovered
	stillConverged, err := converged(ctx, replicas)
	if err != nil {
		return tier{}, err
	}
	measurement.Converged = measurement.Converged && stillConverged

	direct, err := measureDirect(ctx, notes)
	if err != nil {
		return tier{}, err
	}
	measurement.DirectExchangeMS = direct

	// The carrier layer measured on its own: encode, seal, sign, write, then
	// read, verify, decrypt, decode. Comparing two whole-system runs would
	// attribute SQLite's admission cost to the folder, which is where the
	// interesting number is not.
	operations, err := replicas[0].store.ListSyncOperations(ctx, replicas[0].id, 0, int(syncstate.MaxPlannedSequences))
	if err != nil {
		return tier{}, err
	}
	if len(operations) > 0 {
		sealed := time.Now()
		envelope := syncwire.Envelope{
			ProtocolMajor: syncstate.ProtocolMajor, ProtocolMinor: syncstate.ProtocolMinor,
			DatabaseID: "db_g11_evidence", SenderReplicaID: replicas[0].id,
			StateVector: syncstate.Vector{replicas[0].id: operations[len(operations)-1].Sequence},
			Operations:  operations,
		}
		plaintext, err := syncwire.EncodeEnvelope(envelope, syncwire.Limits{})
		if err != nil {
			return tier{}, err
		}
		artifact, err := syncwire.Seal(keys, replicas[0].signer, syncwire.KindEnvelope, "", plaintext, syncwire.Limits{})
		if err != nil {
			return tier{}, err
		}
		name, err := replicas[0].carrier.Publish(ctx, synccarrier.ClassSnapshot, "", artifact)
		if err != nil {
			return tier{}, err
		}
		measurement.SealPublishMS = time.Since(sealed).Milliseconds()

		opened := time.Now()
		stored, err := replicas[0].carrier.Read(ctx, replicas[0].carrier.Namespace(), synccarrier.ClassSnapshot, name)
		if err != nil {
			return tier{}, err
		}
		_, decrypted, err := syncwire.Open(keys, verifier, stored, syncwire.Limits{})
		if err != nil {
			return tier{}, err
		}
		if _, err := syncwire.DecodeEnvelope(decrypted, syncwire.Limits{}); err != nil {
			return tier{}, err
		}
		measurement.ReadOpenMS = time.Since(opened).Milliseconds()
		measurement.CarrierLayerOps = len(operations)
		if err := replicas[0].carrier.Remove(ctx, synccarrier.ClassSnapshot, name); err != nil {
			return tier{}, err
		}
	}
	return measurement, nil
}

// measureDirect exchanges the same workload with no carrier at all, by handing
// operations straight to admission. It is the counterfactual the carrier
// numbers are only meaningful against: everything above it is encoding,
// sealing, signing, and file IO.
func measureDirect(ctx context.Context, notes int) (int64, error) {
	workspace, err := os.MkdirTemp("", "notrios-g11-direct-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(workspace)
	stores := make([]*store.SQLiteStore, 0, 2)
	handshakes := make([]syncstate.Handshake, 0, 2)
	for index := 0; index < 2; index++ {
		canonical, err := store.OpenSQLiteWithAssetStore(
			filepath.Join(workspace, fmt.Sprintf("direct-%d.sqlite", index)),
			filepath.Join(workspace, fmt.Sprintf("assets-%d", index)))
		if err != nil {
			return 0, err
		}
		defer canonical.Close()
		if err := canonical.Bootstrap(ctx); err != nil {
			return 0, err
		}
		if _, err := canonical.AdoptDatabaseIdentity(ctx, "db_g11_direct"); err != nil {
			return 0, err
		}
		if _, err := canonical.EnrollLocalJournal(ctx, "G11 evidence baseline"); err != nil {
			return 0, err
		}
		handshake, err := canonical.LocalSyncHandshake(ctx)
		if err != nil {
			return 0, err
		}
		stores = append(stores, canonical)
		handshakes = append(handshakes, handshake)
	}
	for receiver := range stores {
		for source := range stores {
			if receiver != source {
				if err := stores[receiver].ConfigureSyncAdmissionPeer(ctx, handshakes[source]); err != nil {
					return 0, err
				}
			}
		}
	}
	for index := 0; index < notes; index++ {
		if _, err := stores[index%2].CreateDocument(ctx, store.CreateDocumentRequest{
			Title: fmt.Sprintf("Generated note %d", index),
			Body:  strings.Repeat(fmt.Sprintf("generated line %d\n", index), 12),
		}); err != nil {
			return 0, err
		}
	}
	started := time.Now()
	for receiver := range stores {
		for source := range stores {
			if receiver == source {
				continue
			}
			refreshed, err := stores[source].LocalSyncHandshake(ctx)
			if err != nil {
				return 0, err
			}
			operations, err := stores[source].ListSyncOperations(ctx, refreshed.ReplicaID, 0, int(syncstate.MaxPlannedSequences))
			if err != nil {
				return 0, err
			}
			if len(operations) == 0 {
				continue
			}
			if _, err := stores[receiver].AdmitSyncOperations(ctx, refreshed, operations); err != nil {
				return 0, err
			}
		}
	}
	return time.Since(started).Milliseconds(), nil
}

// converge runs rounds until no operation moves, and reports how many it took.
func converge(ctx context.Context, replicas []*replica) (int, error) {
	for round := 1; round <= 8; round++ {
		moved := 0
		for _, current := range replicas {
			result, err := current.round.Run(ctx)
			if err != nil {
				return round, err
			}
			moved += result.AdmittedOperations + result.PublishedOperations
		}
		if moved == 0 {
			return round, nil
		}
	}
	return 8, nil
}

func converged(ctx context.Context, replicas []*replica) (bool, error) {
	var reference string
	for _, current := range replicas {
		vector, err := current.store.SyncStateVector(ctx)
		if err != nil {
			return false, err
		}
		encoded, err := json.Marshal(vector)
		if err != nil {
			return false, err
		}
		if reference == "" {
			reference = string(encoded)
			continue
		}
		if reference != string(encoded) {
			return false, nil
		}
	}
	return true, nil
}

func carrierSize(root string) (int, int64, error) {
	artifacts := 0
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".nar") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		artifacts++
		total += info.Size()
		return nil
	})
	return artifacts, total, err
}

// measureScan fills one class directory and times a listing, so the bound on a
// scan is a measured number rather than a claim.
func measureScan() ([]scanTier, error) {
	ctx := context.Background()
	workspace, err := os.MkdirTemp("", "notrios-g11-scan-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspace)
	keys, err := syncwire.NewMemoryKeyRing("key_g11_scan")
	if err != nil {
		return nil, err
	}
	group, err := keys.Current()
	if err != nil {
		return nil, err
	}
	carrier, err := synccarrier.NewDirectory(workspace, group, "db_g11_scan", "rep_g11_scan")
	if err != nil {
		return nil, err
	}
	if err := carrier.Initialize(ctx); err != nil {
		return nil, err
	}
	var tiers []scanTier
	published := 0
	for _, target := range []int{100, 1_000, 5_000} {
		for ; published < target; published++ {
			if _, err := carrier.Publish(ctx, synccarrier.ClassSnapshot, "",
				[]byte(fmt.Sprintf("scan probe artifact %d", published))); err != nil {
				return nil, err
			}
		}
		started := time.Now()
		names, err := carrier.List(ctx, carrier.Namespace(), synccarrier.ClassSnapshot)
		if err != nil {
			return nil, err
		}
		tiers = append(tiers, scanTier{
			ArtifactsInOneClass: target,
			ListMicroseconds:    time.Since(started).Microseconds(),
			Listed:              len(names),
		})
	}
	return tiers, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
