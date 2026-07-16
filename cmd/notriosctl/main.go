package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/renesugar/notrios/internal/archive"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/importers/chatgpt"
	claudeimport "github.com/renesugar/notrios/internal/importers/claude"
	"github.com/renesugar/notrios/internal/importers/joplinraw"
	"github.com/renesugar/notrios/internal/importers/obsidian"
	"github.com/renesugar/notrios/internal/importers/twitter"
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
		fmt.Println("notriosctl doctor: scaffold checks passed")
	case "import":
		runImport(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
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
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`notriosctl - admin/import CLI scaffold

Usage:
  notriosctl doctor
  notriosctl version
  notriosctl import joplin-raw [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--dry-run] <raw-export-dir>
  notriosctl import obsidian [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--dry-run] <vault-dir>
  notriosctl import twitter [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook Twitter] [--dry-run] <extracted-archive-dir>
  notriosctl import chatgpt [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook ChatGPT] [--dry-run] <conversations.json|export-dir>
  notriosctl import claude  [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--notebook Claude] [--dry-run] <conversations.json|export-dir>
  notriosctl import archive [--db ...] [--dry-run] [--write-config path] [--import-config path] <archive-dir>
  notriosctl export archive [--db ...] [--query "tag:todo"] <out-dir>

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
