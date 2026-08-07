package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/renesugar/notrios/internal/store"
)

// runTags dispatches the tag subcommands.
func runTags(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl tags rename --from <tag> --to <tag> [--include-children] [--apply]")
		os.Exit(2)
	}
	switch args[0] {
	case "rename":
		runTagRename(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown tags subcommand %q\n", args[0])
		os.Exit(2)
	}
}

// runTagRename renames a tag hierarchy.
//
// Dry run is the default, as it is for `gc` and `fix`. The report it prints is
// not a prediction: the service runs the rename inside a transaction and rolls
// it back, so what a dry run prints is what an apply does.
//
// Exit 1 when a rename would merge tags without `--apply`, so a script that
// meant to rename and would instead have combined two hierarchies stops rather
// than continuing.
func runTagRename(args []string) {
	fs := flag.NewFlagSet("notriosctl tags rename", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	from := fs.String("from", "", "tag to rename")
	to := fs.String("to", "", "new tag name")
	includeChildren := fs.Bool("include-children", false, "also rename every tag under <from>/")
	apply := fs.Bool("apply", false, "perform the rename (default: dry run)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 || *from == "" || *to == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl tags rename --from <tag> --to <tag> [--include-children] [--apply]")
		fs.PrintDefaults()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()

	result, err := st.RenameTag(context.Background(), store.TagRenameRequest{
		From:            *from,
		To:              *to,
		IncludeChildren: *includeChildren,
		DryRun:          !*apply,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(result)

	if !*apply {
		for _, change := range result.Changes {
			if change.Action == store.TagRenameActionMerge {
				fmt.Fprintf(os.Stderr, "refusing to continue silently: %q would merge into an existing %q; re-run with --apply if that is intended\n", change.From, change.To)
				os.Exit(1)
			}
		}
	}
}
