package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// runLint reports what is broken in a library. It writes nothing: fixing is a
// separate, explicitly confirmed operation.
//
// The exit code carries the answer so the command is usable from a script or a
// pre-commit hook: 0 when the library is clean, 1 when findings exist, 2 for a
// usage error. `--quiet` suppresses the report and leaves only that code.
func runLint(args []string) {
	fs := flag.NewFlagSet("notriosctl lint", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "default", "collection ID")
	checks := fs.String("checks", "", "comma-separated checks (default: all)")
	detailLimit := fs.Int("detail-limit", 0, "maximum examples per check (0 = default 100)")
	quiet := fs.Bool("quiet", false, "print nothing; report findings through the exit code only")
	list := fs.Bool("list-checks", false, "print the available checks and exit")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *list {
		printJSON(map[string]any{"checks": store.LintChecks()})
		return
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl lint [--checks a,b] [--detail-limit N] [--quiet]")
		fs.PrintDefaults()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	report, err := st.LintWorkspace(context.Background(), store.LintRequest{
		CollectionID: strings.TrimSpace(*collectionID),
		Checks:       splitCommaList(*checks),
		DetailLimit:  *detailLimit,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !*quiet {
		printJSON(report)
	}
	if report.TotalFindings > 0 {
		os.Exit(1)
	}
}
