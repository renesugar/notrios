package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// runCollections dispatches the collection subcommands.
//
// A query can say `collection:"joplin"`, and until this existed nothing at the
// command line could tell you that "joplin" was a value to type. The command
// line could bring a collection into existence -- `import --collection <id>`
// creates one silently through ensureCollectionOrExit -- and could not then
// name it back to you, which is the worst of both: a surface that writes and
// cannot read.
//
// Listing and showing only. What a collection's identity means, and whether one
// can be renamed or deleted, is v0.8 H16's to decide; adding management here
// would build the thing that item is deciding about.
func runCollections(args []string) {
	if len(args) == 0 {
		printCollectionsUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "list":
		runCollectionList(args[1:])
	case "show":
		runCollectionShow(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown collections subcommand %q\n", args[0])
		printCollectionsUsage()
		os.Exit(2)
	}
}

func printCollectionsUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl collections list [--json] [--db ...]
  notriosctl collections show --collection <id> [--json] [--db ...]
`)
}

// collectionRow is one collection as both output forms report it.
//
// `kind` and `capabilities` are deliberately absent. The store hard-codes the
// capability list for every collection and records no kind at all, so printing
// either would be printing a constant back to the reader as though it were a
// fact about their library. H16 is removing both from the API for that reason.
type collectionRow struct {
	ID          string `json:"collection_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Notes       int    `json:"notes"`
}

func collectionRows(ctx context.Context, st *store.SQLiteStore) []collectionRow {
	collections, err := st.ListCollections(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	counts, err := st.CollectionNoteCounts(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rows := make([]collectionRow, 0, len(collections))
	for _, collection := range collections {
		rows = append(rows, collectionRow{
			ID:          collection.ID,
			Name:        collection.Name,
			Description: collection.Description,
			Notes:       counts[collection.ID],
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func runCollectionList(args []string) {
	fs := flag.NewFlagSet("notriosctl collections list", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	asJSON := fs.Bool("json", false, "print the structured form instead of a table")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 {
		printCollectionsUsage()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	rows := collectionRows(context.Background(), st)

	if *asJSON {
		printJSON(map[string]any{"collections": rows})
		return
	}
	if len(rows) == 0 {
		// Not an empty table. A library always has `default`, so no rows at all
		// means something is wrong with the database rather than with the user.
		fmt.Println("no collections")
		return
	}
	width := len("COLLECTION")
	for _, row := range rows {
		if len(row.ID) > width {
			width = len(row.ID)
		}
	}
	fmt.Printf("%-*s  %5s  %s\n", width, "COLLECTION", "NOTES", "NAME")
	for _, row := range rows {
		fmt.Printf("%-*s  %5d  %s\n", width, row.ID, row.Notes, row.Name)
	}
	fmt.Println("\nName one in a query: collection:\"<id>\"")
}

func runCollectionShow(args []string) {
	fs := flag.NewFlagSet("notriosctl collections show", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	collectionID := fs.String("collection", "", "collection to show")
	asJSON := fs.Bool("json", false, "print the structured form instead of a summary")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	wanted := strings.TrimSpace(*collectionID)
	if fs.NArg() != 0 || wanted == "" {
		printCollectionsUsage()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	for _, row := range collectionRows(context.Background(), st) {
		if row.ID != wanted {
			continue
		}
		if *asJSON {
			printJSON(row)
			return
		}
		fmt.Printf("%s\n", row.ID)
		fmt.Printf("  name   %s\n", row.Name)
		if row.Description != "" {
			fmt.Printf("  about  %s\n", row.Description)
		}
		fmt.Printf("  notes  %d\n", row.Notes)
		return
	}
	fmt.Fprintf(os.Stderr, "no collection %q\n", wanted)
	os.Exit(1)
}
