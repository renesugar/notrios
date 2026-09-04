package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// runNotebooks dispatches the notebook subcommands.
//
// These exist because the absence of a command-line adapter is not the same
// thing as the absence of a capability. Creating a notebook happens once, in
// the store; REST, MCP, the interface and this are four ways to reach it, and
// none of them is where the capability lives. v0.8 H15 initially recorded the
// command line's inability to make a notebook as an explained asymmetry, which
// was wrong: an asymmetry is explained when a surface *cannot* do a thing --
// importing reads directories on this machine and a browser cannot -- and this
// one had no such reason behind it. It was simply unwritten.
func runNotebooks(args []string) {
	if len(args) == 0 {
		printNotebooksUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		runNotebookCreate(args[1:])
	case "list":
		runNotebookList(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown notebooks subcommand %q\n", args[0])
		printNotebooksUsage()
		os.Exit(2)
	}
}

func printNotebooksUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl notebooks create --name <name> [--parent <id|name>] [--icon <emoji>] [--query <query>]
  notriosctl notebooks list [--db ...]
`)
}

// runNotebookCreate makes a notebook, or a notebook defined by a query.
//
// The two are one command because they are one idea to a person: a notebook is
// somewhere notes live, and a query notebook is somewhere notes appear because
// they match. Splitting them into `notebooks create` and `search-notebooks
// create` would make the reader learn a distinction the product draws in its
// storage rather than in its behaviour -- the sidebar lists both together.
func runNotebookCreate(args []string) {
	fs := flag.NewFlagSet("notriosctl notebooks create", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	name := fs.String("name", "", "notebook name")
	parent := fs.String("parent", "", "parent notebook ID or name, for a nested notebook")
	icon := fs.String("icon", "", "optional emoji shown before the name")
	query := fs.String("query", "", "make a notebook whose contents are whatever this query matches")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 || strings.TrimSpace(*name) == "" {
		printNotebooksUsage()
		os.Exit(2)
	}
	if strings.TrimSpace(*query) != "" && strings.TrimSpace(*parent) != "" {
		// A query notebook has no place in the tree: its contents are decided
		// by the query, so nesting it would suggest a containment that is not
		// there.
		fmt.Fprintln(os.Stderr, "a query notebook cannot have a parent: its contents come from the query")
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	if q := strings.TrimSpace(*query); q != "" {
		notebook, err := st.CreateSearchNotebook(ctx, store.CreateSearchNotebookRequest{
			Name: strings.TrimSpace(*name), IconEmoji: strings.TrimSpace(*icon), Query: q,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printJSON(map[string]any{
			"search_notebook_id": notebook.ID, "name": notebook.Name, "query": notebook.Query,
		})
		return
	}

	request := store.CreateNotebookRequest{
		Name: strings.TrimSpace(*name), IconEmoji: strings.TrimSpace(*icon),
	}
	if ref := strings.TrimSpace(*parent); ref != "" {
		// Resolved and refused rather than guessed, for the reason `notes move`
		// gives: names are unique only among siblings.
		parentID, err := resolveNotebookRef(ctx, st, ref)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		request.ParentID = parentID
	}
	notebook, err := st.CreateNotebook(ctx, request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"notebook_id": notebook.ID, "name": notebook.Name, "parent_id": notebook.ParentID,
	})
}

// runNotebookList reports the notebooks and the query notebooks together,
// which is how the sidebar shows them.
func runNotebookList(args []string) {
	fs := flag.NewFlagSet("notriosctl notebooks list", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	notebooks, err := st.ListNotebooks(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rows := make([]map[string]any, 0, len(notebooks))
	for _, notebook := range notebooks {
		rows = append(rows, map[string]any{
			"notebook_id": notebook.ID, "name": notebook.Name, "parent_id": notebook.ParentID,
		})
	}
	searches, err := st.ListSearchNotebooks(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	queries := make([]map[string]any, 0, len(searches))
	for _, search := range searches {
		queries = append(queries, map[string]any{
			"search_notebook_id": search.ID, "name": search.Name, "query": search.Query,
		})
	}
	printJSON(map[string]any{"notebooks": rows, "query_notebooks": queries})
}
