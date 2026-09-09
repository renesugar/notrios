package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// runSearch finds notes from the command line.
//
// Searching was the one everyday capability with no command at all, and the
// consequence was sharper than a missing convenience: nothing at a terminal
// produced note identifiers, so `notes show`, `notes move`, `tags add` and the
// rest could only be used on an id somebody already had. The documented
// workaround was `export archive --query`, which applies the query language to
// a *file export* -- an answer to a different question.
func runSearch(args []string) {
	fs := flag.NewFlagSet("notriosctl search", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	limit := fs.Int("limit", 0, "hits per page, up to the API's ceiling of 100")
	cursor := fs.String("cursor", "", "continue from a previous page's next_cursor")
	count := fs.Bool("count", false, "how many notes match, instead of the matches")
	links := fs.Bool("links", false, "report notrios:// links rather than document:// URIs")
	output := fs.String("output", "", "write to a file instead of standard output")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, `usage: notriosctl search [--limit N] [--cursor c] [--count] [--links] "<query>"`)
		os.Exit(2)
	}
	if *count && (*limit > 0 || strings.TrimSpace(*cursor) != "" || *links) {
		// Paging and link form are about the hits, and --count returns none.
		// Accepting them would be accepting input that changes nothing, which
		// this command line has been removing rather than adding.
		fmt.Fprintln(os.Stderr, "--count returns a number, so --limit, --cursor and --links do not apply to it")
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()
	request := store.SearchRequest{Query: fs.Arg(0), Limit: *limit, Cursor: strings.TrimSpace(*cursor)}

	if *count {
		total, err := store.SearchCount(ctx, st, request)
		if err != nil {
			exitSearchError(err, fs.Arg(0))
		}
		writeOut(*output, jsonBytes(map[string]any{"query": fs.Arg(0), "count": total}))
		return
	}

	response, err := st.Search(ctx, request)
	if err != nil {
		exitSearchError(err, fs.Arg(0))
	}
	hits := make([]map[string]any, 0, len(response.Hits))
	for _, hit := range response.Hits {
		row := map[string]any{
			"document_id":   hit.ID,
			"title":         hit.Title,
			"notebook_id":   hit.NotebookID,
			"collection_id": hit.CollectionID,
			"updated_at":    hit.UpdatedAt.UTC().Format(time.RFC3339),
			"uri":           hit.URI,
			"snippet":       hit.Snippet,
		}
		if *links {
			// Resolved per hit rather than assembled here, so the link a search
			// prints is the one `notriosctl link` prints for the same note.
			uri, err := st.StableDocumentURI(ctx, hit.ID)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			row["uri"] = uri
		}
		hits = append(hits, row)
	}
	report := map[string]any{"query": fs.Arg(0), "hits": hits}
	if response.NextCursor != "" {
		report["next_cursor"] = response.NextCursor
	}
	writeOut(*output, jsonBytes(report))
}

// exitSearchError reports a rejected query as a query problem.
//
// The store returns ErrInvalidInput for a malformed query, and printing it bare
// gives a person a parser message with no indication that the thing at fault is
// the text they typed.
func exitSearchError(err error, queryText string) {
	if strings.Contains(strings.ToLower(err.Error()), "invalid input") {
		fmt.Fprintf(os.Stderr, "that query was refused: %v\nsee `notriosctl help search` and docs/query-language.md\n", err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
