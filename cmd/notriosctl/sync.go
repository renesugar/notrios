package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synccarrier"
	"github.com/renesugar/notrios/internal/synckeys"
	"github.com/renesugar/notrios/internal/syncwire"
)

// developmentKeyWarning is printed by every command that touches key material.
//
// It is not decoration. v0.7 ships a locked-file development secret provider on
// purpose, and the difference between that and a platform keychain is exactly
// the kind of thing a user should be told once per command rather than once in
// a document they read a year ago.
const developmentKeyWarning = "warning: sync keys are stored in a local 0600 file, not an OS keychain. " +
	"v0.8 selects the platform secret store; treat this file as the library's password."

func runSync(args []string) {
	if len(args) == 0 {
		printSyncUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "init":
		runSyncInit(args[1:])
	case "bundle":
		runSyncBundle(args[1:])
	case "pair":
		runSyncPair(args[1:])
	case "status":
		runSyncStatus(args[1:])
	case "discover":
		runSyncDiscover(args[1:])
	case "once":
		runSyncOnce(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown sync subcommand %q\n", args[0])
		printSyncUsage()
		os.Exit(2)
	}
}

func printSyncUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl sync init    [--db ...] [--keys path]
  notriosctl sync bundle  [--db ...] [--keys path] --out <file>
  notriosctl sync pair    [--db ...] [--keys path] <bundle-file>
  notriosctl sync status  [--db ...] [--keys path]
  notriosctl sync discover[--db ...] [--keys path] [--carrier dir]
  notriosctl sync once    [--db ...] [--keys path] [--carrier dir] [--cleanup] [--materialize N]
`)
}

// syncFlags are the options every sync subcommand shares.
type syncFlags struct {
	set        *flag.FlagSet
	configPath *string
	dbPath     *string
	assetStore *string
	keysPath   *string
	carrier    *string
}

func newSyncFlags(name string) *syncFlags {
	set := flag.NewFlagSet("notriosctl sync "+name, flag.ExitOnError)
	return &syncFlags{
		set:        set,
		configPath: set.String("config", "", "optional config file"),
		dbPath:     set.String("db", "", "SQLite database path override"),
		assetStore: set.String("asset-store", "", "asset store directory override"),
		keysPath:   set.String("keys", "", "sync key file (default: the user config directory)"),
		carrier:    set.String("carrier", "", "shared directory to sync through (default: sync.directory)"),
	}
}

func (f *syncFlags) parse(args []string) {
	if err := f.set.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// openSyncStore opens the library and refuses early when it is not enrolled for
// synchronization, which is a configuration answer rather than a failure.
func (f *syncFlags) openSyncStore() (*store.SQLiteStore, store.SyncJournalStatus, string) {
	st := openStoreFromFlags(*f.configPath, *f.dbPath, *f.assetStore)
	status, err := st.JournalStatus(context.Background())
	if err != nil {
		st.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	identity, err := st.GetDatabaseIdentity(context.Background())
	if err != nil {
		st.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return st, status, identity.DatabaseID
}

func (f *syncFlags) keyPath(databaseID string) string {
	if strings.TrimSpace(*f.keysPath) != "" {
		return *f.keysPath
	}
	path, err := synckeys.DefaultPath(databaseID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return path
}

func (f *syncFlags) carrierDirectory() string {
	if strings.TrimSpace(*f.carrier) != "" {
		return *f.carrier
	}
	cfg, err := config.Load(*f.configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if strings.TrimSpace(cfg.Sync.Directory) == "" {
		fmt.Fprintln(os.Stderr, "no carrier directory: pass --carrier or set sync.directory in the config")
		os.Exit(2)
	}
	return cfg.Sync.Directory
}

func runSyncInit(args []string) {
	flags := newSyncFlags("init")
	flags.parse(args)
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()

	if !status.Enabled {
		// Enrolling starts journaling every canonical write and records the
		// sequence-zero boundary G4 defined. It is a deliberate act, which is
		// why it happens in an explicit command rather than the first time
		// something touches a carrier.
		enrolled, err := st.EnrollLocalJournal(context.Background(), "notriosctl sync init")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		status = enrolled
	}
	path := flags.keyPath(databaseID)
	keys, err := synckeys.Open(path)
	if err != nil {
		keys, err = synckeys.Create(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Fprintln(os.Stderr, developmentKeyWarning)
	printJSON(map[string]any{
		"database_id": databaseID,
		"replica_id":  status.ReplicaID,
		"enrolled":    true,
		"keys":        keys.Redacted(),
	})
}

func runSyncBundle(args []string) {
	flags := newSyncFlags("bundle")
	out := flags.set.String("out", "", "file to write the pairing bundle to")
	flags.parse(args)
	if strings.TrimSpace(*out) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync bundle --out <file>")
		os.Exit(2)
	}
	st, _, databaseID := flags.openSyncStore()
	defer st.Close()
	keys := mustOpenKeys(flags.keyPath(databaseID))
	handshake, err := st.LocalSyncHandshake(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bundle, err := keys.ExportBundle(handshake)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	contents, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, append(contents, '\n'), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, developmentKeyWarning)
	fmt.Fprintln(os.Stderr, "warning: this bundle contains the library's group key in clear text. "+
		"Transfer it the way you would a password, and delete it afterwards.")
	printJSON(map[string]any{"bundle": *out, "replica_id": handshake.ReplicaID, "database_id": handshake.DatabaseID})
}

func runSyncPair(args []string) {
	flags := newSyncFlags("pair")
	flags.parse(args)
	if flags.set.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync pair <bundle-file>")
		os.Exit(2)
	}
	contents, err := os.ReadFile(flags.set.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var bundle synckeys.Bundle
	if err := json.Unmarshal(contents, &bundle); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	if !status.Enabled {
		fmt.Fprintln(os.Stderr, "this library is not enrolled for sync: run `notriosctl sync init` first")
		os.Exit(2)
	}
	if bundle.Handshake.DatabaseID != databaseID {
		fmt.Fprintf(os.Stderr, "this bundle is for database %s, not %s\n", bundle.Handshake.DatabaseID, databaseID)
		os.Exit(1)
	}
	keys := mustOpenKeys(flags.keyPath(databaseID))
	signerKeyID, err := keys.ImportBundle(bundle)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Two separate enrollments, because they answer two questions: the key file
	// decides whose signatures are believed, and the store decides whose
	// operations may be admitted. Pairing is the one act that does both.
	if err := st.ConfigureSyncAdmissionPeer(context.Background(), bundle.Handshake); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"paired_replica_id": bundle.Handshake.ReplicaID,
		"signer_key_id":     signerKeyID,
		"database_id":       bundle.Handshake.DatabaseID,
	})
}

func runSyncStatus(args []string) {
	flags := newSyncFlags("status")
	flags.parse(args)
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	report := map[string]any{
		"enrolled":    status.Enabled,
		"database_id": databaseID,
		"replica_id":  status.ReplicaID,
	}
	if status.Enabled {
		vector, err := st.SyncStateVector(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		peers, err := st.ListSyncPeers(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		acknowledged := make([]map[string]any, 0, len(peers))
		for _, peer := range peers {
			acknowledged = append(acknowledged, map[string]any{
				"replica_id": peer.Handshake.ReplicaID, "acknowledged": peer.Handshake.StateVector,
			})
		}
		report["state_vector"] = vector
		report["peers"] = acknowledged
		if keys, err := synckeys.Open(flags.keyPath(databaseID)); err == nil {
			report["keys"] = keys.Redacted()
		} else {
			report["keys"] = map[string]any{"error": err.Error()}
		}
	}
	printJSON(report)
}

func runSyncDiscover(args []string) {
	flags := newSyncFlags("discover")
	flags.parse(args)
	st, keys, round := mustBuildRound(flags, synccarrier.Options{})
	defer st.Close()
	_ = keys
	result, err := round.Discover(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(result)
}

func runSyncOnce(args []string) {
	flags := newSyncFlags("once")
	cleanup := flags.set.Bool("cleanup", false, "remove this replica's own artifacts once every peer has acknowledged them")
	materialize := flags.set.Int("materialize", 16, "at most this many attachment objects to fetch after the exchange")
	flags.parse(args)
	st, keys, round := mustBuildRound(flags, synccarrier.Options{Cleanup: *cleanup})
	defer st.Close()

	result, err := round.Run(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report := map[string]any{"exchange": result}
	if *materialize > 0 {
		provider := synccarrier.NewProvider(round.Carrier(), keys, keys, syncwire.Limits{})
		materialized, err := st.MaterializeResources(context.Background(), provider, *materialize)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		report["resources"] = materialized
	}
	printJSON(report)
}

// mustBuildRound assembles a store, its key material, and a carrier round.
func mustBuildRound(flags *syncFlags, options synccarrier.Options) (*store.SQLiteStore, *synckeys.KeyFile, *synccarrier.Round) {
	st, status, databaseID := flags.openSyncStore()
	if !status.Enabled {
		st.Close()
		fmt.Fprintln(os.Stderr, "this library is not enrolled for sync: run `notriosctl sync init` first")
		os.Exit(2)
	}
	keys := mustOpenKeys(flags.keyPath(databaseID))
	group, err := keys.Current()
	if err != nil {
		st.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	carrier, err := synccarrier.NewDirectory(flags.carrierDirectory(), group, databaseID, status.ReplicaID)
	if err != nil {
		st.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !carrier.Available() {
		st.Close()
		fmt.Fprintf(os.Stderr, "carrier directory is not available: %s\n", flags.carrierDirectory())
		os.Exit(1)
	}
	round := synccarrier.NewRound(synccarrier.NewStoreReplica(st), carrier, keys, keys, keys,
		store.NewLocalObjectProvider(st), options)
	return st, keys, round
}

func mustOpenKeys(path string) *synckeys.KeyFile {
	keys, err := synckeys.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return keys
}
