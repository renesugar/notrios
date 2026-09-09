package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// This file makes `store.RunBatch` reachable from a terminal, added in v0.8 H17.
//
// The capability was on REST as `POST /api/v1/batch` and on MCP as `run_batch`,
// and the command line could not call either — because a batch over an explicit
// list of ids is unusable where nothing produces ids. H19 built
// `notriosctl search`, so a query can name the set instead, which is also the
// language the search box takes.
//
// The operations grow a `--query` rather than a `notriosctl batch` command
// growing an `--operation`. One way to say move, two ways to name what it moves.

// batchSelection are the flags every operation gains.
type batchSelection struct {
	query *string
	apply *bool
	mode  *string
	limit *int
}

func registerBatchFlags(fs *flag.FlagSet, what string) *batchSelection {
	return &batchSelection{
		query: fs.String("query", "", "act on every note this query matches, instead of one note"),
		apply: fs.Bool("apply", false, "perform the "+what+"; without it the selection is shown and nothing changes"),
		// The default matches REST and MCP rather than being chosen afresh
		// here. A sweep over forty notes should not be abandoned because one of
		// them is protected, and the same request sent from a terminal and from
		// a client should do the same thing.
		mode: fs.String("mode", store.BatchModeBestEffort,
			"best_effort applies what it can and reports each; atomic applies every item or none"),
		limit: fs.Int("limit", 0, "refuse a selection larger than this; the ceiling is "+itoaBatch(store.MaxBatchItems)),
	}
}

func itoaBatch(n int) string { return fmt.Sprintf("%d", n) }

// selected resolves the query to the notes it names.
//
// It refuses rather than truncates past the ceiling, the way the REST surface
// does: a batch that quietly acted on the first five hundred of a thousand
// matches would be the worst possible answer, because it looks like success.
func (b *batchSelection) selected(ctx context.Context, st *store.SQLiteStore, query string) []store.SearchHit {
	ceiling := store.MaxBatchItems
	if *b.limit > 0 && *b.limit < ceiling {
		ceiling = *b.limit
	}
	response, err := st.Search(ctx, store.SearchRequest{Query: query, Limit: ceiling + 1})
	if err != nil {
		exitSearchError(err, query)
	}
	if len(response.Hits) > ceiling {
		fmt.Fprintf(os.Stderr, "that query matches more than %d notes; narrow it, or raise --limit up to %d\n",
			ceiling, store.MaxBatchItems)
		os.Exit(1)
	}
	if len(response.Hits) == 0 {
		fmt.Fprintf(os.Stderr, "that query matches no notes: %s\n", query)
		os.Exit(1)
	}
	return response.Hits
}

// runOverQuery shows what a query selected and, with --apply, acts on it.
//
// Showing first is what every wide-reaching command here already does --
// `tags rename` dry-runs, the importers scan before writing, `publish run`
// refuses unless the reviewed digest still matches. A batch that moved forty
// notes because a query matched more than its author expected is exactly the
// failure that pattern exists to prevent.
func (b *batchSelection) runOverQuery(ctx context.Context, st *store.SQLiteStore, req store.BatchRequest) {
	hits := b.selected(ctx, st, strings.TrimSpace(*b.query))

	if !*b.apply {
		notes := make([]map[string]any, 0, len(hits))
		for _, hit := range hits {
			notes = append(notes, map[string]any{
				"document_id": hit.ID, "title": hit.Title, "notebook_id": hit.NotebookID,
			})
		}
		report := map[string]any{
			"dry_run": true, "operation": req.Operation, "query": strings.TrimSpace(*b.query),
			"selected": len(hits), "notes": notes,
			"next_step": "run the same command with --apply to perform it",
		}
		if req.NotebookID != "" {
			report["notebook_id"] = req.NotebookID
		}
		if len(req.Tags) > 0 {
			report["tags"] = req.Tags
		}
		printJSON(report)
		return
	}

	req.Mode = strings.TrimSpace(*b.mode)
	req.Items = make([]store.BatchItem, 0, len(hits))
	for _, hit := range hits {
		item := store.BatchItem{DocumentID: hit.ID}
		if req.Operation == store.BatchOpTrash {
			// Trash is revision-preconditioned and the store refuses an item
			// without one. A search hit does not carry a revision, so each is
			// read here — immediately before the batch, which is the same
			// guarantee a client gets when it reads a note and then trashes it.
			// The alternative would be refusing --query on delete, which would
			// leave the one destructive operation as the one a query cannot
			// name.
			current := documentOrExit(ctx, st, hit.ID)
			item.BaseRevisionID = current.CurrentRevisionID
		}
		req.Items = append(req.Items, item)
	}
	result, err := st.RunBatch(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(result)
	if result.Failed > 0 || result.RolledBack > 0 {
		// A run that happened is not a run that worked. REST answers 200 here
		// because the report is the content; a shell has only the exit code, so
		// it gets the distinction that way.
		os.Exit(1)
	}
}

// wantsQuery reports whether the caller named a set rather than one note.
func (b *batchSelection) wantsQuery() bool { return strings.TrimSpace(*b.query) != "" }

// refuseBothTargets stops a command being given a note and a query at once.
func refuseBothTargets(documentID string, b *batchSelection) {
	if strings.TrimSpace(documentID) != "" && b.wantsQuery() {
		fmt.Fprintln(os.Stderr, "--document and --query name different sets; send one")
		os.Exit(2)
	}
}

// requireTarget is refuseBothTargets plus the requirement that one be given.
//
// It returns the single note when there is one, and the empty string when the
// caller named a set instead; the usage function is passed in because the two
// families of command print different ones.
func requireTarget(documentID string, b *batchSelection, extras int, usage func()) string {
	refuseBothTargets(documentID, b)
	wanted := strings.TrimSpace(documentID)
	if extras != 0 || (wanted == "" && !b.wantsQuery()) {
		usage()
		os.Exit(2)
	}
	return wanted
}

// runNoteDuplicate copies notes.
//
// It exists because `duplicate` is one of the six batch operations and had no
// single-note command to hang `--query` on. Leaving it out for that reason would
// have been arbitrary: the set is what makes the batch reachable, not five
// sixths of it.
func runNoteDuplicate(args []string) {
	fs := flag.NewFlagSet("notriosctl notes duplicate", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "note to duplicate")
	selection := registerBatchFlags(fs, "duplication")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	wanted := requireTarget(*documentID, selection, fs.NArg(), printNotesUsage)

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	if selection.wantsQuery() {
		selection.runOverQuery(ctx, st, store.BatchRequest{Operation: store.BatchOpDuplicate})
		return
	}
	doc := documentOrExit(ctx, st, wanted)
	result, err := st.RunBatch(ctx, store.BatchRequest{
		Operation: store.BatchOpDuplicate, Mode: store.BatchModeAtomic,
		Items: []store.BatchItem{{DocumentID: doc.ID}},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(result)
	if result.Failed > 0 || result.RolledBack > 0 {
		os.Exit(1)
	}
}
