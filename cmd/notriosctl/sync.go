package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
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
	case "invite":
		runSyncInvite(args[1:])
	case "join":
		runSyncJoin(args[1:])
	case "accept":
		runSyncAccept(args[1:])
	case "enroll":
		runSyncEnroll(args[1:])
	case "peers":
		runSyncPeers(args[1:])
	case "revoke":
		runSyncRevoke(args[1:])
	case "status":
		runSyncStatus(args[1:])
	case "discover":
		runSyncDiscover(args[1:])
	case "handshake":
		runSyncHandshake(args[1:])
	case "exchange":
		runSyncPush(args[1:])
	case "fetch-backup":
		runSyncFetchBackup(args[1:])
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
  notriosctl sync init     [--db ...] [--keys path]
  notriosctl sync invite   [--ttl 15m] [--label ...] [--offline --out <file>]
  notriosctl sync join     --url <base-url> --code <code>
  notriosctl sync accept   --invite <file> --code <code> --out <file>
  notriosctl sync enroll   --acceptance <file> --code <code>
  notriosctl sync peers    [--db ...]
  notriosctl sync handshake --url <base-url> [--db ...] [--keys path]
  notriosctl sync exchange --url <base-url> [--materialize N]
  notriosctl sync fetch-backup --url <base-url> --out <dir> [--chunk-bytes N]
  notriosctl sync revoke   --key <signer-key-id> [--reason ...] [--advance-epoch]
  notriosctl sync status   [--db ...] [--keys path]
  notriosctl sync discover [--db ...] [--keys path] [--carrier dir]
  notriosctl sync once     [--db ...] [--keys path] [--carrier dir] [--cleanup] [--materialize N]
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

// runSyncHandshake proves the whole security foundation in one command: this
// replica signs a request, a peer authenticates it as an enrolled principal of
// the same database, and answers with what it is compatible with. It carries no
// note content — the data plane is G14's.
func runSyncHandshake(args []string) {
	flags := newSyncFlags("handshake")
	peerURL := flags.set.String("url", "", "the peer's base URL")
	flags.parse(args)
	if strings.TrimSpace(*peerURL) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl sync handshake --url <base-url>")
		os.Exit(2)
	}
	st, status, databaseID := flags.openSyncStore()
	defer st.Close()
	requireEnrolled(status)
	keys := mustOpenKeys(flags.keyPath(databaseID))
	client := &syncauth.Client{
		BaseURL: *peerURL, DatabaseID: databaseID, ReplicaID: status.ReplicaID,
		SignerKeyID: keys.SignerKeyID(), Private: keys.PrivateSigningKey(),
	}
	code, body, err := client.Do(context.Background(), "GET", "/api/v1/sync/handshake", nil)
	if err != nil {
		exit(err)
	}
	if code != 200 {
		fmt.Fprintf(os.Stderr, "the peer refused this replica (HTTP %d): %s\n", code, strings.TrimSpace(string(body)))
		os.Exit(1)
	}
	fmt.Println(strings.TrimSpace(string(body)))
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
		provider := synccarrier.NewProvider(round.Carrier(), keys,
			syncwire.MultiVerifier{keys, st.PeerVerifier()}, syncwire.Limits{})
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
	// Whose signatures this replica trusts now has two sources: its own key,
	// so it can read back what it published, and the peer keys the database
	// holds, where enrolment and revocation are transactional and auditable.
	verifier := syncwire.MultiVerifier{keys, st.PeerVerifier()}
	round := synccarrier.NewRound(synccarrier.NewStoreReplica(st), carrier, keys, keys, verifier,
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
