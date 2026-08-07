package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// runNotes dispatches the note subcommands.
func runNotes(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: notriosctl notes move --document <id> --notebook <id|name>")
		os.Exit(2)
	}
	switch args[0] {
	case "move":
		runNoteMove(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown notes subcommand %q\n", args[0])
		os.Exit(2)
	}
}

// runNoteMove files one note into another notebook.
//
// It exists because the move was reachable from the store, REST, and MCP but
// from neither the CLI nor the GUI, so a note filed wrongly could not be
// corrected from either surface a person actually uses.
//
// There is no dry run and no `--apply`, unlike `fix`, `gc`, and `tags rename`.
// A move is not destructive and not lossy: it changes where one note lives, and
// moving it back is the same command with the other notebook. A confirmation
// step here would be ceremony rather than safety.
func runNoteMove(args []string) {
	fs := flag.NewFlagSet("notriosctl notes move", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "note to move")
	notebook := fs.String("notebook", "", "destination notebook ID, or its name when unambiguous")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 || strings.TrimSpace(*documentID) == "" || strings.TrimSpace(*notebook) == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl notes move --document <id> --notebook <id|name>")
		fs.PrintDefaults()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	notebookID, err := resolveNotebookRef(ctx, st, strings.TrimSpace(*notebook))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	doc, err := st.MoveDocumentToNotebook(ctx, strings.TrimSpace(*documentID), notebookID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"document_id": doc.ID,
		"title":       doc.Title,
		"notebook_id": doc.NotebookID,
	})
}

// resolveNotebookRef accepts a notebook ID or a name.
//
// A name is accepted because notebook IDs are not memorable, but it is refused
// when it matches more than one notebook: names are unique only among siblings,
// so `Contacts/Work` and `Personal/Work` can both exist. Filing a note into
// whichever one came back first is exactly the silent mistake this command is
// meant to correct.
func resolveNotebookRef(ctx context.Context, st *store.SQLiteStore, ref string) (string, error) {
	if _, err := st.GetNotebook(ctx, ref); err == nil {
		return ref, nil
	}
	notebooks, err := st.ListNotebooks(ctx)
	if err != nil {
		return "", err
	}
	matches := []store.Notebook{}
	for _, nb := range notebooks {
		if strings.EqualFold(nb.Name, ref) {
			matches = append(matches, nb)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no notebook with ID or name %q", ref)
	case 1:
		return matches[0].ID, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, nb := range matches {
			ids = append(ids, nb.ID)
		}
		return "", fmt.Errorf("notebook name %q is ambiguous (%s); use the ID", ref, strings.Join(ids, ", "))
	}
}
