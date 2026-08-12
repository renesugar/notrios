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

	"github.com/renesugar/notrios/internal/archive"
	"github.com/renesugar/notrios/internal/archivev2"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/helpdocs"
	"github.com/renesugar/notrios/internal/importers/chatgpt"
	claudeimport "github.com/renesugar/notrios/internal/importers/claude"
	"github.com/renesugar/notrios/internal/importers/joplinraw"
	"github.com/renesugar/notrios/internal/importers/obsidian"
	"github.com/renesugar/notrios/internal/importers/twitter"
	"github.com/renesugar/notrios/internal/localize"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/version"
)

func main() {
	cmd := "help"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "version":
		fmt.Println(version.Version)
	case "doctor":
		runDoctor(os.Args[2:])
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
	case "notes":
		runNotes(os.Args[2:])
	case "graph":
		runGraph(os.Args[2:])
	case "jobs":
		runJobs(os.Args[2:])
	case "help", "-h", "--help":
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
	if err := st.Bootstrap(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
	if err := st.Bootstrap(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
	if err := st.Bootstrap(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

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

func runResources(args []string) {
	if len(args) == 0 || args[0] != "report" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl resources report [--config config.yaml] [--db path] [--asset-store path]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("notriosctl resources report", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl resources report [--config config.yaml] [--db path] [--asset-store path]")
		fs.PrintDefaults()
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
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 || (*dryRun && *apply) {
		fmt.Fprintln(os.Stderr, "usage: notriosctl gc [--config config.yaml] [--db path] [--asset-store path] [--dry-run | --apply]")
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
	if err := st.Bootstrap(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report, err := st.GarbageCollect(ctx, store.GarbageCollectionRequest{
		Policy: store.GarbageCollectionPolicy{
			UnreferencedFor:   cfg.Retention.UnreferencedDuration(),
			PurgedResourceFor: cfg.Retention.PurgedResourceDuration(),
		},
		Apply: *apply,
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

func printHelp() {
	fmt.Print(`notriosctl - Notrios import/export/maintenance CLI

Usage:
  notriosctl doctor [--config config.yaml] [--db path] [--asset-store path]
  notriosctl version
  notriosctl import joplin-raw [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--batch-size 100] [--preserve-source] [--dry-run] [--write-config path] [--import-config path] [--localize-media] <raw-export-dir>
  notriosctl import obsidian [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--dry-run] [--localize-media] <vault-dir>
  notriosctl import twitter [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook Twitter] [--dry-run] <extracted-archive-dir>
  notriosctl import chatgpt [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook ChatGPT] [--dry-run] <conversations.json|export-dir>
  notriosctl import claude  [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook Claude] [--dry-run] <conversations.json|export-dir>
  notriosctl import archive [--db ...] [--dry-run] [--write-config path] [--import-config path] <archive-dir>
  notriosctl export archive [--db ...] [--query "tag:todo"] <out-dir>
  notriosctl verify archive-v2 <archive-dir>
  notriosctl restore archive-v2 --intent replace|adopt|merge|fork [--db ...] [--new-database-id id] <archive-dir>
  notriosctl export archive-v2 [--db ...] [--target full_archive|subset_transfer] [--notebooks id,id] [--tags a,b] [--query "tag:todo"] [--documents id,id] [--match any|all] [--pack] [--overwrite] [--no-verify] <out-dir>
  notriosctl seed-help [--db ...] [docs-dir]     # mirror docs/ into the read-only Help notebook
  notriosctl localize [--config config.yaml] [--db ...] [--dry-run] [--allow-review] [--base-revision rev] <document-id>
                                                 # download policy-allowed remote media and rewrite the note to resource:// URIs
  notriosctl resources report [--config config.yaml] [--db ...] [--asset-store ...]
                                                 # exact duplicates, unreferenced blobs, notebook usage, and review-only perceptual signals
  notriosctl gc [--config config.yaml] [--db ...] [--asset-store ...] [--dry-run | --apply]
                                                 # retention-aware resource GC; dry-run is the default
  notriosctl lint [--db ...] [--checks a,b] [--detail-limit 100] [--quiet] [--list-checks]
                                                 # read-only workspace report; exit 1 when findings exist
  notriosctl fix [--db ...] [--kinds a,b] [--document id] [--apply] [--list-kinds]
                                                 # repair the mechanically safe findings; dry run is the default
  notriosctl tags rename --from <tag> --to <tag> [--db ...] [--include-children] [--apply]
                                                 # hierarchical tag rename; dry run is the default and reports every tag and count
  notriosctl notes move --document <id> --notebook <id|name> [--db ...]
                                                 # file one note into another notebook; a name is refused when it matches more than one
  notriosctl graph report [--db ...] [--collection default] [--limit N] [--write-note] [--quiet]
                                                 # link-graph shape; --write-note overwrites the read-only note in Reports
  notriosctl graph export [--db ...] [--collection default] [--overwrite] <out-dir>
                                                 # nodes.csv and edges.csv for Gephi, Cytoscape, NetworkX or igraph
  notriosctl jobs list [--db ...] [--kind k] [--state s] [--limit 50]
  notriosctl jobs status [--db ...] [--wait] [--timeout 30m] [--quiet] <job-id>
  notriosctl jobs show [--db ...] [--command] <job-id>
  notriosctl jobs cancel [--db ...] <job-id>
                                                 # long imports and exports record a job; status exits 0 succeeded,
                                                 # 1 failed, 3 running, 4 cancelled, 5 no such job, 6 interrupted
  notriosctl link [--db ...] [--anchor slug|^block] [--list-anchors] <document-id>
                                                 # print the stable notrios:// link for a note or one of its sections
  notriosctl open [--profile name] [--registry path] [--db path] [--launch] <notrios-uri>
                                                 # resolve a stable link on this machine (exit 1 unresolved, 2 malformed)
  notriosctl profile create --name <profile> [--listen 127.0.0.1:8080] [--db ...] [--sync-target none|directory|rest]
  notriosctl profile show --name <profile> [--registry path]
  notriosctl profile list [--registry path]
  notriosctl profile validate [--name <profile>] [--registry path]
  notriosctl profile start --name <profile> [--binary notriosd] [--dry-run]
                                                 # owner-only config, identity/path/port checks, and secret-free startup
  notriosctl profile register --name <profile> [--db ...] [--registry path]
  notriosctl profile forget --name <profile> [--registry path]
                                                 # legacy stable-link routing entry; ambiguity remains explicit
  notriosctl register-url-handler [--apply] [--binary path] [--dir path]
                                                 # Ubuntu/XDG notrios:// protocol handler; prints unless --apply
  notriosctl publish profile save --name <profile> [--notebooks id,id] [--tags a,b] [--link-action plain_text]
  notriosctl publish profile list|delete [--name <profile>]
  notriosctl publish plan --profile <profile>    # read-only privacy review; prints the plan digest
  notriosctl publish run --profile <profile> --reviewed-plan <sha256> <out-dir>
                                                 # scoped sanitized archive-v2 handoff; refuses a stale review

Future commands:
  notriosctl sync status
`)
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
	if err := st.Bootstrap(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
	if err := st.Bootstrap(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

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
	if err := st.Bootstrap(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
	collectionID := fs.String("collection", "default", "collection ID")
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
		Selection: store.SelectionSpec{
			CollectionID: *collectionID,
			NotebookIDs:  splitCommaList(*notebooks),
			Tags:         splitCommaList(*tags),
			Query:        *query,
			DocumentIDs:  splitCommaList(*documents),
			Match:        *match,
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
		{Name: "collection", Value: *collectionID},
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
	collectionID := fs.String("collection", "default", "collection ID")
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
	docsDir := "docs"
	if fs.NArg() == 1 {
		docsDir = fs.Arg(0)
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
func runDoctor(args []string) {
	fs := flag.NewFlagSet("notriosctl doctor", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	failed := false
	report := func(ok bool, required bool, label, detail string) {
		mark := "ok  "
		if !ok {
			if required {
				mark = "FAIL"
				failed = true
			} else {
				mark = "info"
			}
		}
		fmt.Printf("%s  %-16s %s\n", mark, label, detail)
	}

	report(true, true, "go runtime", runtime.Version())

	var cfg config.Config
	var err error
	if *configPath != "" {
		cfg, err = config.Load(*configPath)
	} else {
		cfg, err = config.LoadDefaultOrExample()
	}
	if err != nil {
		report(false, true, "config", err.Error())
		os.Exit(1)
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
		}
	}

	probe := filepath.Join(cfg.Data.AssetStore, ".doctor-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		report(false, true, "asset store", err.Error())
	} else {
		_ = os.Remove(probe)
		report(true, true, "asset store", cfg.Data.AssetStore+" is writable")
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

	if failed {
		fmt.Println("doctor: one or more required checks FAILED")
		os.Exit(1)
	}
	fmt.Println("doctor: required checks passed")
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
