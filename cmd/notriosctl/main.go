package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/archive"
	"github.com/renesugar/notrios/internal/archivev2"
	"github.com/renesugar/notrios/internal/clispec"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/credentials"
	"github.com/renesugar/notrios/internal/helpdocs"
	"github.com/renesugar/notrios/internal/importers/chatgpt"
	claudeimport "github.com/renesugar/notrios/internal/importers/claude"
	"github.com/renesugar/notrios/internal/importers/joplinraw"
	"github.com/renesugar/notrios/internal/importers/obsidian"
	"github.com/renesugar/notrios/internal/importers/twitter"
	"github.com/renesugar/notrios/internal/localize"
	"github.com/renesugar/notrios/internal/migrate"
	"github.com/renesugar/notrios/internal/paths"
	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/version"
)

func main() {
	cmd := "help"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	if len(os.Args) > 1 && helpAsked(os.Args[1:]) {
		return
	}

	switch cmd {
	case "version":
		fmt.Println(version.Version)
	case "doctor":
		runDoctor(os.Args[2:])
	case "paths":
		runPaths(os.Args[2:])
	case "config":
		runConfig(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	case "import":
		runImport(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
	case "seed-help":
		runSeedHelp(os.Args[2:])
	case "localize":
		runLocalize(os.Args[2:])
	case "resources":
		runResources(os.Args[2:])
	case "gc":
		runGarbageCollection(os.Args[2:])
	case "verify":
		runVerify(os.Args[2:])
	case "compatibility":
		runCompatibility(os.Args[2:])
	case "restore":
		runRestore(os.Args[2:])
	case "profile":
		runProfile(os.Args[2:])
	case "link":
		runLink(os.Args[2:])
	case "open":
		runOpen(os.Args[2:])
	case "register-url-handler":
		runRegisterURLHandler(os.Args[2:])
	case "publish":
		runPublish(os.Args[2:])
	case "lint":
		runLint(os.Args[2:])
	case "fix":
		runFix(os.Args[2:])
	case "tags":
		runTags(os.Args[2:])
	case "tasks":
		runTasks(os.Args[2:])
	case "templates":
		runTemplates(os.Args[2:])
	case "notebooks":
		runNotebooks(os.Args[2:])
	case "collections":
		runCollections(os.Args[2:])
	case "search":
		runSearch(os.Args[2:])
	case "notes":
		runNotes(os.Args[2:])
	case "graph":
		runGraph(os.Args[2:])
	case "jobs":
		runJobs(os.Args[2:])
	case "sync":
		runSync(os.Args[2:])
	case "snapshot":
		runSnapshot(os.Args[2:])
	case "help", "-h", "--help":
		if len(os.Args) > 2 && helpFor(os.Args[2:]...) {
			return
		}
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		printHelp()
		os.Exit(2)
	}
}

func runImport(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "missing import source type")
		printHelp()
		os.Exit(2)
	}
	switch args[0] {
	case "joplin-raw":
		runImportJoplinRaw(args[1:])
	case "obsidian":
		runImportObsidian(args[1:])
	case "twitter":
		runImportTwitter(args[1:])
	case "chatgpt":
		runImportConversations(args[1:], "chatgpt")
	case "claude":
		runImportConversations(args[1:], "claude")
	case "archive":
		runImportArchive(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown import source type %q\n", args[0])
		printHelp()
		os.Exit(2)
	}
}

func runImportJoplinRaw(args []string) {
	fs := flag.NewFlagSet("notriosctl import joplin-raw", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "default", "collection ID")
	dryRun := fs.Bool("dry-run", false, "scan and report without writing")
	batchSize := fs.Int("batch-size", 100, "maximum source items per durable import batch (1-500)")
	preserveSource := fs.Bool("preserve-source", false, "store exact RAW source items in a content-addressed source bundle")
	writeConfig := fs.String("write-config", "", "dry run: optionally write the import configuration to this path")
	importConfig := fs.String("import-config", "", "import configuration file with notebook-path renames")
	localizeMedia := fs.Bool("localize-media", false, "after importing, localize policy-allowed remote media in the imported notes")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl import joplin-raw [options] <raw-export-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()
	ctx := context.Background()
	bootstrapOrExit(st, ctx)
	ensureCollectionOrExit(st, ctx, *collectionID)
	sourceDir := fs.Arg(0)
	options := joplinraw.Options{
		CollectionID:   *collectionID,
		BatchSize:      *batchSize,
		PreserveSource: *preserveSource,
	}
	if *dryRun {
		importCfg, report, err := joplinraw.DryRun(ctx, st, sourceDir, options)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		target, err := writeJoplinDryRunConfig(importCfg, *writeConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if target != "" {
			fmt.Fprintf(os.Stderr, "import configuration written to %s\n", target)
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *importConfig != "" {
		importCfg, err := joplinraw.LoadConfig(*importConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		options.Config = importCfg
	}
	// A dry run writes nothing and is fast, so only a real import gets a job
	// record. A control plane full of records for runs that changed nothing
	// would make the list harder to read for no gain.
	runner, jobCtx := startTrackedJob(st, store.JobKindImportJoplinRaw, []store.JobParameter{
		{Name: "collection", Value: *collectionID},
		{Name: "batch-size", Value: strconv.Itoa(*batchSize)},
		{Name: "preserve-source", Value: strconv.FormatBool(*preserveSource)},
		{Name: "import-config", Value: *importConfig, Path: *importConfig != ""},
		{Name: "localize-media", Value: strconv.FormatBool(*localizeMedia)},
		{Name: "_source-dir", Value: sourceDir, Path: true},
	})
	// AfterBatch runs *after* the batch is committed and the checkpoint saved,
	// and aborting it aborts the import. That is exactly where a cooperative
	// cancellation belongs, so progress and the stop signal share one hook.
	options.AfterBatch = func(phase string, processed, total int) error {
		fmt.Fprintf(os.Stderr, "joplin import: %s %d/%d\n", phase, processed, total)
		return runner.Progress(phase, processed, total)
	}
	report, err := joplinraw.Import(jobCtx, st, sourceDir, options)
	if err == nil && *localizeMedia {
		localizeImportedNotes(jobCtx, cfg, st, report.DocumentIDs, false)
	}
	finishTrackedJob(runner, map[string]any{
		"notes_imported":    report.NotesImported,
		"notes_updated":     report.NotesUpdated,
		"resources":         report.ResourcesImported,
		"checkpoint_status": report.CheckpointStatus,
	}, err)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeJoplinDryRunConfig(config joplinraw.ImportConfig, target string) (string, error) {
	if target == "" {
		return "", nil
	}
	if err := joplinraw.WriteConfig(config, target); err != nil {
		return "", err
	}
	return target, nil
}

func runImportObsidian(args []string) {
	fs := flag.NewFlagSet("notriosctl import obsidian", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "default", "collection ID")
	dryRun := fs.Bool("dry-run", false, "scan and report without writing")
	batchSize := fs.Int("batch-size", 100, "maximum source items per durable import batch (1-500)")
	preserveSource := fs.Bool("preserve-source", false, "store exact vault files in a content-addressed source bundle")
	writeConfig := fs.String("write-config", "", "dry run: where to write the import configuration (default <vault>/.notrios/import-config.json)")
	importConfig := fs.String("import-config", "", "import configuration file with vault-folder renames")
	localizeMedia := fs.Bool("localize-media", false, "after importing, localize policy-allowed remote media in the imported notes")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl import obsidian [options] <vault-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()
	ctx := context.Background()
	bootstrapOrExit(st, ctx)
	ensureCollectionOrExit(st, ctx, *collectionID)
	sourceDir := fs.Arg(0)
	options := obsidian.Options{
		CollectionID:   *collectionID,
		BatchSize:      *batchSize,
		PreserveSource: *preserveSource,
	}
	if *dryRun {
		importCfg, report, err := obsidian.DryRun(ctx, st, sourceDir, options)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		target := *writeConfig
		if target == "" {
			target = filepath.Join(sourceDir, ".notrios", "import-config.json")
		}
		if err := obsidian.WriteConfig(importCfg, target); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "import configuration written to %s\n", target)
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *importConfig != "" {
		importCfg, err := obsidian.LoadConfig(*importConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		options.Config = importCfg
	}
	runner, jobCtx := startTrackedJob(st, store.JobKindImportObsidian, []store.JobParameter{
		{Name: "collection", Value: *collectionID},
		{Name: "batch-size", Value: strconv.Itoa(*batchSize)},
		{Name: "preserve-source", Value: strconv.FormatBool(*preserveSource)},
		{Name: "import-config", Value: *importConfig, Path: *importConfig != ""},
		{Name: "localize-media", Value: strconv.FormatBool(*localizeMedia)},
		{Name: "_vault-dir", Value: sourceDir, Path: true},
	})
	options.AfterBatch = func(phase string, processed, total int) error {
		fmt.Fprintf(os.Stderr, "obsidian import: %s %d/%d\n", phase, processed, total)
		return runner.Progress(phase, processed, total)
	}
	report, err := obsidian.Import(jobCtx, st, sourceDir, options)
	if err == nil && *localizeMedia {
		localizeImportedNotes(jobCtx, cfg, st, report.DocumentIDs, false)
	}
	finishTrackedJob(runner, map[string]any{
		"notes_imported":    report.NotesImported,
		"notes_updated":     report.NotesUpdated,
		"resources":         report.ResourcesImported,
		"checkpoint_status": report.CheckpointStatus,
	}, err)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runLocalize converts a note's remote media into local resources through
// the shared localize engine (v0.3 task H4).
func runLocalize(args []string) {
	fs := flag.NewFlagSet("notriosctl localize", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	dryRun := fs.Bool("dry-run", false, "report policy decisions without fetching or writing")
	allowReview := fs.Bool("allow-review", false, "also localize URLs whose policy decision is review")
	baseRevision := fs.String("base-revision", "", "base revision precondition (defaults to the note's current revision)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl localize [options] <document-id>")
		fs.PrintDefaults()
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()
	ctx := context.Background()
	bootstrapOrExit(st, ctx)

	result, err := localize.New(cfg.RemoteMedia, st).LocalizeDocument(ctx, localize.Options{
		DocumentID:     fs.Arg(0),
		BaseRevisionID: *baseRevision,
		DryRun:         *dryRun,
		AllowReview:    *allowReview,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// printResourcesUsage names both resource subcommands.
func printResourcesUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl resources report [--config config.yaml] [--db path] [--asset-store path]
  notriosctl resources get --resource <id> [--output <file>] [--db ...]
`)
}

func runResources(args []string) {
	if len(args) == 0 {
		printResourcesUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "report":
		runResourceReport(args[1:])
	case "get":
		runResourceGet(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown resources subcommand %q\n", args[0])
		printResourcesUsage()
		os.Exit(2)
	}
}

func runResourceReport(args []string) {
	fs := flag.NewFlagSet("notriosctl resources report", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 {
		printResourcesUsage()
		os.Exit(2)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	report, err := st.ResourceReport(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
}

func runGarbageCollection(args []string) {
	fs := flag.NewFlagSet("notriosctl gc", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	dryRun := fs.Bool("dry-run", false, "explicitly plan and report without deleting (also the default)")
	apply := fs.Bool("apply", false, "delete only retention-expired, currently unreferenced resources")
	snapshotPath := fs.String("snapshot", "", "retained physical snapshot to verify for sync-aware collection")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 || (*dryRun && *apply) {
		fmt.Fprintln(os.Stderr, "usage: notriosctl gc [--config config.yaml] [--db path] [--asset-store path] [--snapshot retained-snapshot-dir] [--dry-run | --apply]")
		fs.PrintDefaults()
		os.Exit(2)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()
	ctx := context.Background()
	bootstrapOrExit(st, ctx)
	var gate store.RetentionGate
	if journal, journalErr := st.JournalStatus(ctx); journalErr == nil && journal.Enabled {
		if strings.TrimSpace(*snapshotPath) == "" {
			fmt.Fprintln(os.Stderr, "sync-aware resource collection requires --snapshot so the retained recovery image can be re-verified")
			os.Exit(2)
		}
		verified, verifyErr := snapshotimage.VerifyDirectory(ctx, *snapshotPath, snapshotimage.DefaultLimits())
		if verifyErr != nil {
			fmt.Fprintln(os.Stderr, verifyErr)
			os.Exit(1)
		}
		identity, identityErr := st.GetDatabaseIdentity(ctx)
		if identityErr != nil || verified.DatabaseID != identity.DatabaseID {
			if identityErr != nil {
				fmt.Fprintln(os.Stderr, identityErr)
			} else {
				fmt.Fprintf(os.Stderr, "retained snapshot belongs to database %s, not %s\n", verified.DatabaseID, identity.DatabaseID)
			}
			os.Exit(1)
		}
		if err := st.RecordVerifiedSyncSnapshot(ctx, store.VerifiedSyncSnapshot{
			SnapshotID: verified.SnapshotID, CommitSHA256: verified.CommitSHA256, Vector: verified.SnapshotVector,
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		gate, err = st.BuildSyncRetentionGate(ctx, store.SyncRetentionRequest{
			HistoryFor: cfg.Retention.SyncHistoryDuration(), WarningBefore: cfg.Retention.SyncPeerWarningDuration(),
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	report, err := st.GarbageCollect(ctx, store.GarbageCollectionRequest{
		Policy: store.GarbageCollectionPolicy{
			UnreferencedFor:   cfg.Retention.UnreferencedDuration(),
			PurgedResourceFor: cfg.Retention.PurgedResourceDuration(),
		},
		Apply: *apply,
		Gate:  gate,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
}

// localizeImportedNotes runs the shared localize engine over the notes an
// import touched (--localize-media). Failures are reported per note and do
// not fail the completed import.
func localizeImportedNotes(ctx context.Context, cfg config.Config, st store.Store, documentIDs []string, allowReview bool) {
	localizer := localize.New(cfg.RemoteMedia, st)
	total := struct{ localized, blocked, review, failed int }{}
	for _, documentID := range documentIDs {
		result, err := localizer.LocalizeDocument(ctx, localize.Options{DocumentID: documentID, AllowReview: allowReview})
		if err != nil {
			fmt.Fprintf(os.Stderr, "localize %s: %v\n", documentID, err)
			total.failed++
			continue
		}
		total.localized += len(result.Localized)
		total.blocked += len(result.Blocked)
		total.review += len(result.Review)
		total.failed += len(result.Failed)
	}
	fmt.Fprintf(os.Stderr, "localize-media: %d localized, %d blocked, %d needing review, %d failed across %d notes\n",
		total.localized, total.blocked, total.review, total.failed, len(documentIDs))
}

// printHelp is the finite command and flag usage registry shown by notriosctl.
//
// printHelp renders the whole command line from internal/clispec.
//
// It used to be a string literal that this program, docs/cli.md, the Help
// notebook and the documentation coverage gate all depended on, and that
// nothing checked against the dispatcher. Eight commands were missing from it.
//
//notrios:doc user cli-usage-forms
//notrios:help cli the-notriosctl-cli
//notrios:enumerates go:github.com/renesugar/notrios/cmd/notriosctl#printHelp
func printHelp() {
	commandSpec().Help(os.Stdout)
}

// helpFor answers a request for help at any level and reports whether it could.
func helpFor(path ...string) bool {
	return commandSpec().Help(os.Stdout, path...)
}

// commandSpec is the compiled-in description of this command line.
func commandSpec() clispec.Registry {
	return clispec.Must()
}

// helpAsked answers a request for help at whatever depth it was asked, and
// reports whether it did. It runs before any dispatcher, so one rule decides
// what asking for help means at every level of the command line.
//
// Two behaviours it replaces: a command group answered `unknown notes
// subcommand "--help"` and exited 2, and a leaf command fell through to the
// flag package, whose dump names flags `-body` where the documentation says
// `--body` and never mentions positional arguments at all.
func helpAsked(args []string) bool {
	if len(args) == 0 {
		return false
	}
	// `notriosctl help notes show` names its subject after the word, so the
	// walk starts past it and the request needs no second marker.
	asked := args[0] == "help"
	if asked {
		args = args[1:]
	}
	spec := commandSpec()
	path, rest := []string{}, args
	for len(rest) > 0 {
		candidate := append(append([]string{}, path...), rest[0])
		_, isCommand := spec.Lookup(candidate...)
		if !isCommand && len(spec.Children(candidate...)) == 0 {
			break
		}
		path, rest = candidate, rest[1:]
	}
	if !asked && !clispec.HelpRequested(rest) {
		return false
	}
	for _, argument := range rest {
		if argument == "--json" {
			if err := spec.JSON(os.Stdout, path...); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return true
		}
	}
	return spec.Help(os.Stdout, path...)
}

func runImportTwitter(args []string) {
	fs := flag.NewFlagSet("notriosctl import twitter", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "default", "collection ID")
	notebookName := fs.String("notebook", "Twitter", "notebook name for imported tweets")
	dryRun := fs.Bool("dry-run", false, "scan and report without writing")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl import twitter [options] <extracted-archive-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()
	ctx := context.Background()
	bootstrapOrExit(st, ctx)
	ensureCollectionOrExit(st, ctx, *collectionID)
	report, err := twitter.Import(ctx, st, fs.Arg(0), twitter.Options{CollectionID: *collectionID, NotebookName: *notebookName, DryRun: *dryRun})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runImportConversations(args []string, kind string) {
	fs := flag.NewFlagSet("notriosctl import "+kind, flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "default", "collection ID")
	notebookName := fs.String("notebook", "", "notebook name for imported conversations")
	dryRun := fs.Bool("dry-run", false, "scan and report without writing")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: notriosctl import %s [options] <conversations.json|export-dir>\n", kind)
		fs.PrintDefaults()
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()
	ctx := context.Background()
	bootstrapOrExit(st, ctx)
	ensureCollectionOrExit(st, ctx, *collectionID)

	var report any
	switch kind {
	case "chatgpt":
		report, err = chatgpt.Import(ctx, st, fs.Arg(0), chatgpt.Options{CollectionID: *collectionID, NotebookName: *notebookName, DryRun: *dryRun})
	case "claude":
		report, err = claudeimport.Import(ctx, st, fs.Arg(0), claudeimport.Options{CollectionID: *collectionID, NotebookName: *notebookName, DryRun: *dryRun})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func openStoreFromFlags(configPath, dbPath, assetStore string) *store.SQLiteStore {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if dbPath != "" {
		// An explicit --db names where the library is, so the roots derived
		// from it follow it. Otherwise `notriosctl lint --db /tmp/x/notes.sqlite`
		// would create ./data/quarantine and ./data/search-index in whatever
		// directory the command was run from -- unrelated to the database it
		// was told to open.
		if dbPath != ":memory:" {
			config.UseDataDirectory(&cfg, filepath.Dir(dbPath), nil)
		}
		cfg.Data.DatabasePath = dbPath
	}
	if assetStore != "" {
		cfg.Data.AssetStore = assetStore
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bootstrapOrExit(st, context.Background())
	return st
}

func printJSON(v any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runExport(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl export archive|archive-v2 [options] <out-dir>")
		os.Exit(2)
	}
	switch args[0] {
	case "archive":
		runExportArchiveV1(args)
	case "archive-v2":
		runExportArchiveV2(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown export format %q\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: notriosctl export archive|archive-v2 [options] <out-dir>")
		os.Exit(2)
	}
}

// runExportArchiveV2 writes the lossless native archive-v2 snapshot. It is a
// local filesystem operation on purpose: no REST or MCP surface accepts an
// output path.
func runExportArchiveV2(args []string) {
	fs := flag.NewFlagSet("notriosctl export archive-v2", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	target := fs.String("target", "full_archive", "full_archive (complete backup) or subset_transfer")
	notebooks := fs.String("notebooks", "", "comma-separated notebook IDs (recursive) for a subset transfer")
	tags := fs.String("tags", "", "comma-separated tag names for a subset transfer")
	query := fs.String("query", "", "query-language scope for a subset transfer")
	documents := fs.String("documents", "", "comma-separated document IDs for a subset transfer")
	match := fs.String("match", "any", "combine populated selector types with any or all")
	maxDocuments := fs.Int("max-documents", 0, "maximum selected documents (0 = planner default)")
	recordsPerObject := fs.Int("records-per-object", 0, "maximum records per JSONL object (0 = format maximum)")
	pack := fs.Bool("pack", false, "concatenate objects into large pack files instead of one file per object (far fewer file operations; an interrupted packed export restarts rather than resumes)")
	packBytes := fs.Int64("pack-bytes", 0, "target bytes per pack file (0 = default 256 MiB)")
	overwrite := fs.Bool("overwrite", false, "replace an existing complete archive in the destination")
	skipVerify := fs.Bool("no-verify", false, "skip the read-only verification pass after publishing the manifest")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl export archive-v2 [options] <out-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	options := archivev2.ExportOptions{
		Target: *target,
		// No collection selector. Every note goes into the archive with the
		// identifier it has, and a `full_archive` that could omit a provenance
		// would not be one -- which is what it did, silently, for as long as
		// the selector defaulted to `default`.
		Selection: store.SelectionSpec{
			NotebookIDs: splitCommaList(*notebooks),
			Tags:        splitCommaList(*tags),
			Query:       *query,
			DocumentIDs: splitCommaList(*documents),
			Match:       *match,
		},
		MaxDocuments:     *maxDocuments,
		RecordsPerObject: *recordsPerObject,
		Pack:             *pack,
		PackTargetBytes:  *packBytes,
		Overwrite:        *overwrite,
		SkipVerification: *skipVerify,
	}
	// Export has no durable batch boundary to stop at, but it does thread a
	// context through every store read. Cancelling that aborts at the next one,
	// and because the manifest is written last and is the completion marker, a
	// stopped export leaves nothing that could pass as a complete archive.
	runner, jobCtx := startTrackedJob(st, store.JobKindExportArchiveV2, []store.JobParameter{
		{Name: "target", Value: *target},
		{Name: "notebooks", Value: *notebooks},
		{Name: "tags", Value: *tags},
		{Name: "query", Value: *query},
		{Name: "documents", Value: *documents},
		{Name: "match", Value: *match},
		{Name: "pack", Value: strconv.FormatBool(*pack)},
		{Name: "overwrite", Value: strconv.FormatBool(*overwrite)},
		{Name: "no-verify", Value: strconv.FormatBool(*skipVerify)},
		{Name: "_out-dir", Value: fs.Arg(0), Path: true},
	})
	report, err := archivev2.Export(jobCtx, st, fs.Arg(0), options)
	finishTrackedJob(runner, map[string]any{
		"documents":   report.SelectedDocuments,
		"objects":     report.Objects,
		"bytes":       report.Bytes,
		"full_backup": report.FullBackup,
		"verified":    report.Verified,
	}, err)
	if !report.FullBackup {
		fmt.Fprintln(os.Stderr, "note: this archive is a scoped snapshot and is not a complete database backup")
	}
	printJSON(report)
}

func splitCommaList(value string) []string {
	values := []string{}
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func runExportArchiveV1(args []string) {
	fs := flag.NewFlagSet("notriosctl export archive", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "", "narrow to one collection (default: every collection)")
	query := fs.String("query", "", "query-language scope (empty = all notes)")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl export archive [options] <out-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	report, err := archive.Export(context.Background(), st, fs.Arg(0), archive.ExportOptions{Query: *query, CollectionID: *collectionID})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
}

func runImportArchive(args []string) {
	fs := flag.NewFlagSet("notriosctl import archive", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "default", "collection ID")
	dryRun := fs.Bool("dry-run", false, "analyze conflicts and write an import configuration without importing")
	writeConfig := fs.String("write-config", "", "dry run: where to write the import configuration (default <archive>/import-config.json)")
	importConfig := fs.String("import-config", "", "import configuration file with notebook renames")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl import archive [options] <archive-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}
	archiveDir := fs.Arg(0)
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	if *dryRun {
		cfg, report, err := archive.DryRun(ctx, st, archiveDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		target := *writeConfig
		if target == "" {
			target = archiveDir + "/import-config.json"
		}
		if err := archive.WriteConfig(cfg, target); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "import configuration written to %s\n", target)
		printJSON(report)
		return
	}

	options := archive.ImportOptions{CollectionID: *collectionID}
	if *importConfig != "" {
		cfg, err := archive.LoadConfig(*importConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		options.Config = cfg
	}
	report, err := archive.Import(ctx, st, archiveDir, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
}

// runVerify is read-only by construction: it opens no database and performs no
// canonical write, so an archive can be checked before anything is restored
// from it.
func runVerify(args []string) {
	if len(args) == 0 || args[0] != "archive-v2" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl verify archive-v2 <archive-dir>")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("notriosctl verify archive-v2", flag.ExitOnError)
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl verify archive-v2 <archive-dir>")
		os.Exit(2)
	}
	report, err := archivev2.VerifyDirectory(fs.Arg(0), archivev2.DefaultLimits())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
}

// runCompatibility identifies a manifest and evaluates only its declared
// version/schema/capabilities. It never opens archive objects or a physical
// SQLite image; accepted archive-v2 declarations still require runVerify.
func runCompatibility(args []string) {
	if len(args) == 0 || args[0] != "archive-v2" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl compatibility archive-v2 [--reader current-v2|previous-loose-v2] <archive-dir|manifest.json>")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("notriosctl compatibility archive-v2", flag.ExitOnError)
	reader := fs.String("reader", archivev2.ReaderProfileCurrent, "frozen reader capability profile")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl compatibility archive-v2 [--reader current-v2|previous-loose-v2] <archive-dir|manifest.json>")
		os.Exit(2)
	}
	profile, err := archivev2.ReaderProfileByName(*reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	report, err := archivev2.EvaluateCompatibility(fs.Arg(0), profile, archivev2.DefaultLimits())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
	if report.Decision != "accept" {
		os.Exit(1)
	}
}

func runSnapshot(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl snapshot create|verify|restore [options]")
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		runSnapshotCreate(args[1:])
	case "verify":
		runSnapshotVerify(args[1:])
	case "restore":
		runSnapshotRestore(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown snapshot operation %q\n", args[0])
		os.Exit(2)
	}
}

// runSnapshotCreate is local-filesystem only. It does not wrap, transmit, or
// install the result; those security and cutover boundaries remain G14d.
func runSnapshotCreate(args []string) {
	fs := flag.NewFlagSet("notriosctl snapshot create", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl snapshot create [--config path] [--db path] [--asset-store path] <out-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}
	if err := config.EnsureDirectories(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()
	bootstrapOrExit(st, context.Background())
	runner, jobCtx := startTrackedJob(st, store.JobKindSnapshotImage, []store.JobParameter{
		{Name: "_out-dir", Value: fs.Arg(0), Path: true},
	})
	report, err := snapshotimage.Create(jobCtx, st, cfg.Data.AssetStore, fs.Arg(0), snapshotimage.CreateOptions{})
	if err == nil && report.Verified {
		createdAt, parseErr := time.Parse(time.RFC3339Nano, report.CreatedAt)
		if parseErr != nil {
			err = parseErr
		} else {
			err = st.RecordVerifiedSyncSnapshot(jobCtx, store.VerifiedSyncSnapshot{
				SnapshotID: report.SnapshotID, CommitSHA256: report.CommitSHA256,
				CreatedAt: createdAt, Vector: report.SnapshotVector,
			})
		}
	}
	finishTrackedJob(runner, map[string]any{
		"snapshot_id": report.SnapshotID, "packs": report.Packs,
		"objects": report.Objects, "bytes": report.DatabaseBytes + report.ExternalBytes,
		"verified": report.Verified,
	}, err)
	printJSON(report)
}

func runSnapshotVerify(args []string) {
	fs := flag.NewFlagSet("notriosctl snapshot verify", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl snapshot verify <snapshot-dir>")
		os.Exit(2)
	}
	report, err := snapshotimage.VerifyDirectory(context.Background(), fs.Arg(0), snapshotimage.DefaultLimits())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
}

func runSnapshotRestore(args []string) {
	fs := flag.NewFlagSet("notriosctl snapshot restore", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	intent := fs.String("intent", "", "required: replace or adopt")
	emergency := fs.String("emergency", "", "emergency snapshot directory (default beside the database)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 || (*intent != "replace" && *intent != "adopt") {
		fmt.Fprintln(os.Stderr, "usage: notriosctl snapshot restore --intent replace|adopt [--db path] [--asset-store path] [--emergency dir] <snapshot-dir>")
		os.Exit(2)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}
	report, err := snapshotimage.Restore(context.Background(), fs.Arg(0), snapshotimage.RestoreOptions{
		Intent: *intent, TargetDatabase: cfg.Data.DatabasePath,
		TargetAssetRoot: cfg.Data.AssetStore, EmergencyDirectory: *emergency,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
}

// runRestore admits a verified archive into a database. Intent is mandatory:
// each choice has a different consequence for the logical database universe
// and Notrios never guesses one.
func runRestore(args []string) {
	if len(args) == 0 || args[0] != "archive-v2" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl restore archive-v2 --intent replace|adopt|merge|fork [options] <archive-dir>")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("notriosctl restore archive-v2", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	intent := fs.String("intent", "", "required: replace, adopt, merge, or fork")
	newDatabaseID := fs.String("new-database-id", "", "fork only: the new logical database ID")
	batchSize := fs.Int("batch-size", 0, "records per canonical transaction (0 = default)")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 || strings.TrimSpace(*intent) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl restore archive-v2 --intent replace|adopt|merge|fork [options] <archive-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	summary, err := archivev2.Restore(context.Background(), st, fs.Arg(0), archivev2.RestoreOptions{
		Intent:        archivev2.RestoreIntent(strings.ToLower(strings.TrimSpace(*intent))),
		NewDatabaseID: *newDatabaseID,
		BatchSize:     *batchSize,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(summary)
}

func runSeedHelp(args []string) {
	fs := flag.NewFlagSet("notriosctl seed-help", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	// The positional argument wins. Otherwise the documentation ships with the
	// program, so it comes from the installed assets root; the bare relative
	// "docs" is a checkout convenience and is used only in a checkout, where it
	// is what a developer means.
	docsDir := ""
	if fs.NArg() == 1 {
		docsDir = fs.Arg(0)
	}
	if docsDir == "" {
		docsDir = defaultHelpDocsDir()
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	report, err := helpdocs.Seed(context.Background(), st, docsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(report)
}

// runDoctor performs real environment/configuration diagnostics: config
// loading, database open + schema version, asset-store writability, web UI
// asset presence, and optional Recoll sidecar availability. It exits 1 when a
// required check fails; optional checks only inform.
// doctorCheck is one thing doctor looked at.
//
// `required` is kept beside `state` because the two answer different questions:
// what doctor found, and whether it is allowed to be like that. A monitoring
// script wants the second without re-deriving it from the wording.
type doctorCheck struct {
	Check    string `json:"check"`
	State    string `json:"state"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

// doctorReport collects the checks so the same run can be printed either way.
//
// doctor was the one command a person could read and a script could not parse,
// which is backwards in a product whose other eighty commands print JSON. The
// checks are collected rather than streamed so that both forms come from one
// pass; doctor is fast enough that nothing is lost by holding them.
type doctorReport struct {
	checks   []doctorCheck
	failed   bool
	asJSON   bool
	finished bool
}

func (r *doctorReport) add(ok, required bool, label, detail string) {
	state := "ok"
	if !ok {
		if required {
			state = "failed"
			r.failed = true
		} else {
			state = "info"
		}
	}
	r.checks = append(r.checks, doctorCheck{Check: label, State: state, Required: required, Detail: detail})
}

// finish prints the run and exits, and is the only place that does either, so
// the early exit on an unreadable config reports the same way as a full run
// rather than leaving a `--json` caller with nothing to parse.
func (r *doctorReport) finish() {
	r.finished = true
	if r.asJSON {
		summary := "required checks passed"
		if r.failed {
			summary = "one or more required checks failed"
		}
		printJSON(map[string]any{
			"checks": r.checks, "failed": r.failed, "summary": summary,
		})
	} else {
		for _, check := range r.checks {
			mark := map[string]string{"ok": "ok  ", "failed": "FAIL", "info": "info"}[check.State]
			fmt.Printf("%s  %-16s %s\n", mark, check.Check, check.Detail)
		}
		if r.failed {
			fmt.Println("doctor: one or more required checks FAILED")
		} else {
			fmt.Println("doctor: required checks passed")
		}
	}
	if r.failed {
		os.Exit(1)
	}
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("notriosctl doctor", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	asJSON := fs.Bool("json", false, "print the checks as JSON instead of a report")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	run := &doctorReport{asJSON: *asJSON}
	report := run.add

	report(true, true, "go runtime", runtime.Version())

	var cfg config.Config
	var err error
	if *configPath != "" {
		cfg, err = config.Load(*configPath)
	} else {
		cfg, err = config.LoadDefault()
	}
	if err != nil {
		report(false, true, "config", err.Error())
		run.finish()
		return
	}
	source := cfg.ConfigPath
	if source == "" {
		source = "built-in defaults"
	}
	report(true, true, "config", source)
	if *dbPath != "" {
		cfg.Data.DatabasePath = *dbPath
	}
	if *assetStore != "" {
		cfg.Data.AssetStore = *assetStore
	}

	if err := config.EnsureDirectories(cfg); err != nil {
		report(false, true, "directories", err.Error())
	} else {
		report(true, true, "directories", "storage directories exist or were created")
	}

	st, err := store.OpenSQLiteWithAssetStore(cfg.Data.DatabasePath, cfg.Data.AssetStore)
	if err != nil {
		report(false, true, "database", err.Error())
	} else {
		defer st.Close()
		if err := st.Bootstrap(context.Background()); err != nil {
			report(false, true, "database", "bootstrap: "+err.Error())
		} else if status, err := st.Status(context.Background()); err != nil {
			report(false, true, "database", err.Error())
		} else {
			report(true, true, "database", fmt.Sprintf("%s (schema version %d)", cfg.Data.DatabasePath, status.SchemaVersion))
			// doctor is where a user looks after an upgrade, so it is the most
			// likely place for the migration notice to actually be read.
			if migration, migrated := st.LastMigration(); migrated {
				report(true, false, "schema migration", fmt.Sprintf("migrated %d -> %d; a verified copy of the database as it was is in %s",
					migration.FromVersion, migration.ToVersion, migration.BackupDir))
			}
		}
	}

	// A pre-0.8 library in the working directory is worth saying out loud here
	// above all: doctor *creates* the database at the resolved path, so without
	// this it cheerfully reports a healthy, empty library at schema version 27
	// while the user's real notes sit in the directory they ran it from.
	if resolution, err := paths.ForProcess(nil); err == nil {
		if working, err := os.Getwd(); err == nil {
			if candidate, found := migrate.Detect(working, resolution, cfg.Data.DatabasePath); found {
				report(false, false, "pre-0.8 layout",
					candidate.DatabasePath+" is not in use; run `notriosctl migrate --dry-run`")
			}
		}
	}

	probe := filepath.Join(cfg.Data.AssetStore, ".doctor-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		report(false, true, "asset store", err.Error())
	} else {
		_ = os.Remove(probe)
		report(true, true, "asset store", cfg.Data.AssetStore+" is writable")
	}

	// Which store holds this library's sync credential, and whether it can be
	// reached. This is here because the alternative is discovering it when a
	// sync first runs: by then the user has already paired, and the failure
	// looks like a network problem rather than a machine that has no keyring.
	credentialStore := resolveDoctorCredentialStore(cfg)
	switch kind := credentialStore.Kind; kind {
	case config.CredentialStoreDevelopmentFile:
		detail := "development file: sync keys are protected by file permissions only, not by a keychain"
		if credentialStore.Advisory != "" {
			// Reported as a failed check rather than a note. An installed
			// profile that no longer defaults to this store is in a state the
			// user has to act on, and doctor is where they look.
			report(false, false, "credential store", credentialStore.Advisory)
		} else {
			report(true, false, "credential store", detail)
		}
	case config.CredentialStoreNative:
		if provider, err := credentials.Select(credentials.KindNative); err != nil {
			// Required: the profile asked for a store that is not there, and
			// nothing else will be substituted for it.
			report(false, true, "credential store", err.Error())
		} else {
			report(true, true, "credential store", provider.Name()+" is reachable")
		}
	default:
		report(false, true, "credential store", "unknown credential store "+strconv.Quote(kind))
	}

	if _, err := os.Stat(filepath.Join("web", "dist", "index.html")); err == nil {
		report(true, false, "web ui", "web/dist present (browser UI will be served)")
	} else {
		report(false, false, "web ui", "web/dist missing here; run `make web` or serve API-only")
	}

	if _, err := exec.LookPath(firstNonEmptyString(cfg.SearchSidecar.Binary, "recollindex")); err == nil {
		report(true, false, "recoll", "recollindex found (optional search sidecar available)")
	} else {
		report(false, false, "recoll", "recollindex not found (optional; FTS5 search works without it)")
	}
	if cfg.SearchSidecar.Enabled {
		report(true, false, "recoll", "search_sidecar.enabled is true in this config")
	}

	run.finish()
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// defaultHelpDocsDir locates the documentation shipped with this build.
func defaultHelpDocsDir() string {
	resolution, err := paths.ForProcess(nil)
	if err != nil {
		return "docs"
	}
	if resolution.Mode == paths.ModeSource {
		return "docs"
	}
	if assets := strings.TrimSpace(resolution.Root(paths.RootProgramAssets)); assets != "" {
		return filepath.Join(assets, "docs")
	}
	return "docs"
}

// runConfig dispatches the config subcommands. `show` is the only one: reading
// the resolved configuration is a diagnostic, and writing it is what the
// configuration file and the flags are for.
func runConfig(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl config show [--config config.yaml] [--json] [--no-redact]")
		os.Exit(2)
	}
	switch args[0] {
	case "show":
		runConfigShow(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown config command %q\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: notriosctl config show [--config config.yaml] [--json] [--no-redact]")
		os.Exit(2)
	}
}

// bootstrapOrExit bootstraps a store and reports a schema migration if one
// happened.
//
// Every subcommand did this inline, which was fine until a migration became
// something a user should be told about. A migration rewrites their library; it
// used to happen with no backup and no notice, and the only sign was the
// absence of a complaint. The notice goes to stderr so it cannot corrupt the
// output of a command being piped somewhere.
// ensureCollectionOrExit records the provenance an import is labelling.
//
// documents.collection_id is a foreign key, so naming a collection that does
// not exist fails on the constraint rather than on anything a reader could act
// on: `sqlite step rc=19: FOREIGN KEY constraint failed`, after the notebooks
// had already been written. Every importer offers --collection and nothing
// created the row, so the flag was unusable with any value but the default.
//
// Created rather than refused, because the flag exists to label where notes
// came from and requiring a separate command first would only move a typo one
// step earlier.
func ensureCollectionOrExit(st *store.SQLiteStore, ctx context.Context, collectionID string) {
	id := strings.TrimSpace(collectionID)
	if id == "" || id == "default" {
		return
	}
	if _, err := st.EnsureCollection(ctx, id, id); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func bootstrapOrExit(st *store.SQLiteStore, ctx context.Context) {
	if err := st.Bootstrap(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if report, migrated := st.LastMigration(); migrated {
		fmt.Fprintf(os.Stderr,
			"notriosctl migrated this database from schema %d to %d.\n"+
				"A verified copy of it as it was is in %s\n",
			report.FromVersion, report.ToVersion, report.BackupDir)
	}
}
