package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// notriosctl graph — the two graph jobs that do not belong in a GUI.
//
// `report` measures the whole collection, and `export` writes files to a path
// the user names. Neither is a choice a REST caller or an MCP client should
// make on someone's behalf, which is the same line archive export draws.
func runGraph(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl graph report|export ...")
		os.Exit(2)
	}
	switch args[0] {
	case "report":
		runGraphReport(args[1:])
	case "export":
		runGraphExport(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown graph subcommand %q (want report or export)\n", args[0])
		os.Exit(2)
	}
}

// runGraphReport prints the shape of the link graph, and optionally writes it
// into the library as a note.
//
// Writing is opt-in and explicit. The scan reads every note, so doing it on a
// schedule or on every save would be the one unbounded thing in an otherwise
// bounded design.
func runGraphReport(args []string) {
	fs := flag.NewFlagSet("notriosctl graph report", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "", "narrow to one collection (default: every collection)")
	limit := fs.Int("limit", 0, "entries per list (0 = 20 with --write-note, 100 otherwise)")
	writeNote := fs.Bool("write-note", false, "overwrite the read-only report note in the Reports notebook")
	quiet := fs.Bool("quiet", false, "print nothing; useful with --write-note")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl graph report [--write-note] [--limit N]")
		fs.PrintDefaults()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	req := store.GraphReportRequest{CollectionID: strings.TrimSpace(*collectionID), Limit: *limit}

	if *writeNote {
		doc, report, err := st.WriteGraphReportNote(context.Background(), req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !*quiet {
			printJSON(map[string]any{"document_id": doc.ID, "uri": doc.URI, "title": doc.Title, "report": report})
		}
		return
	}
	report, err := st.GraphReport(context.Background(), req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !*quiet {
		printJSON(report)
	}
}

// runGraphExport writes the link graph as CSV node and edge lists.
func runGraphExport(args []string) {
	fs := flag.NewFlagSet("notriosctl graph export", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "", "narrow to one collection (default: every collection)")
	overwrite := fs.Bool("overwrite", false, "replace nodes.csv and edges.csv if they already exist")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl graph export [--overwrite] <out-dir>")
		fs.PrintDefaults()
		os.Exit(2)
	}
	outDir := fs.Arg(0)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Refusing by default rather than overwriting: an export names a directory
	// the user chose, and silently replacing a file there is not this command's
	// decision to make.
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if *overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	nodesPath := filepath.Join(outDir, "nodes.csv")
	edgesPath := filepath.Join(outDir, "edges.csv")
	nodes, err := os.OpenFile(nodesPath, flags, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer nodes.Close()
	edges, err := os.OpenFile(edgesPath, flags, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer edges.Close()

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	summary, err := st.ExportGraphCSV(context.Background(),
		store.ExportGraphRequest{CollectionID: strings.TrimSpace(*collectionID)}, nodes, edges)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"collection_id": summary.CollectionID,
		"nodes":         summary.Nodes,
		"edges":         summary.Edges,
		"nodes_path":    nodesPath,
		"edges_path":    edgesPath,
	})
}
