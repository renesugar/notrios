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

	"github.com/renesugar/notrios/internal/archive"
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
	report, err := joplinraw.Import(ctx, st, fs.Arg(0), joplinraw.Options{CollectionID: *collectionID, DryRun: *dryRun})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *localizeMedia && !*dryRun {
		localizeImportedNotes(ctx, cfg, st, report.DocumentIDs, false)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runImportObsidian(args []string) {
	fs := flag.NewFlagSet("notriosctl import obsidian", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "default", "collection ID")
	dryRun := fs.Bool("dry-run", false, "scan and report without writing")
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
	report, err := obsidian.Import(ctx, st, fs.Arg(0), obsidian.Options{CollectionID: *collectionID, DryRun: *dryRun})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *localizeMedia && !*dryRun {
		localizeImportedNotes(ctx, cfg, st, report.DocumentIDs, false)
	}
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
  notriosctl import joplin-raw [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--dry-run] [--localize-media] <raw-export-dir>
  notriosctl import obsidian [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--dry-run] [--localize-media] <vault-dir>
  notriosctl import twitter [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook Twitter] [--dry-run] <extracted-archive-dir>
  notriosctl import chatgpt [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook ChatGPT] [--dry-run] <conversations.json|export-dir>
  notriosctl import claude  [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook Claude] [--dry-run] <conversations.json|export-dir>
  notriosctl import archive [--db ...] [--dry-run] [--write-config path] [--import-config path] <archive-dir>
  notriosctl export archive [--db ...] [--query "tag:todo"] <out-dir>
  notriosctl seed-help [--db ...] [docs-dir]     # mirror docs/ into the read-only Help notebook
  notriosctl localize [--config config.yaml] [--db ...] [--dry-run] [--allow-review] [--base-revision rev] <document-id>
                                                 # download policy-allowed remote media and rewrite the note to resource:// URIs
  notriosctl resources report [--config config.yaml] [--db ...] [--asset-store ...]
                                                 # exact duplicates, unreferenced blobs, notebook usage, and review-only perceptual signals

Future commands:
  notriosctl publish quartz --profile <name>
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
	if len(args) == 0 || args[0] != "archive" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl export archive [options] <out-dir>")
		os.Exit(2)
	}
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
