package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"example.com/notes-companion/internal/config"
	"example.com/notes-companion/internal/importers/joplinraw"
	"example.com/notes-companion/internal/importers/obsidian"
	"example.com/notes-companion/internal/store"
	"example.com/notes-companion/internal/version"
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
		fmt.Println("notesctl doctor: scaffold checks passed")
	case "import":
		runImport(os.Args[2:])
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
	default:
		fmt.Fprintf(os.Stderr, "unknown import source type %q\n", args[0])
		printHelp()
		os.Exit(2)
	}
}

func runImportJoplinRaw(args []string) {
	fs := flag.NewFlagSet("notesctl import joplin-raw", flag.ExitOnError)
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
		fmt.Fprintln(os.Stderr, "usage: notesctl import joplin-raw [options] <raw-export-dir>")
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
	fs := flag.NewFlagSet("notesctl import obsidian", flag.ExitOnError)
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
		fmt.Fprintln(os.Stderr, "usage: notesctl import obsidian [options] <vault-dir>")
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
	fmt.Print(`notesctl - admin/import CLI scaffold

Usage:
  notesctl doctor
  notesctl version
  notesctl import joplin-raw [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--dry-run] <raw-export-dir>
  notesctl import obsidian [--config config.yaml] [--db data/notes.sqlite] [--asset-store data/assets] [--collection default] [--dry-run] <vault-dir>

Future commands:
  notesctl publish quartz --profile <name>
`)
}
