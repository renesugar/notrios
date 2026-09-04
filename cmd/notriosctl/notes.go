package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// runNotes dispatches the note subcommands.
func runNotes(args []string) {
	if len(args) == 0 {
		printNotesUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		runNoteCreate(args[1:])
	case "show":
		runNoteShow(args[1:])
	case "edit":
		runNoteEdit(args[1:])
	case "move":
		runNoteMove(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown notes subcommand %q\n", args[0])
		printNotesUsage()
		os.Exit(2)
	}
}

func printNotesUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl notes create --title <title> [--notebook <id|name>] [--body-file path | --body text]
  notriosctl notes show --document <id> [--body]
  notriosctl notes edit --document <id> [--title <title>] [--body-file path | --body text]
  notriosctl notes move --document <id> --notebook <id|name>
`)
}

// runNoteShow reports one note's properties.
//
// It is the general read the narrow verbs were missing. Every other note
// command changes something and then describes what it did; none of them could
// answer "what does this note look like now?", so a script had to trust the
// report of the command that made the change.
//
// The body is omitted unless asked for. A note body can be long, and a command
// people will run to check a title should not print a page of Markdown to do
// it.
func runNoteShow(args []string) {
	fs := flag.NewFlagSet("notriosctl notes show", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "note to show")
	withBody := fs.Bool("body", false, "include the note body, which may be long")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 || strings.TrimSpace(*documentID) == "" {
		printNotesUsage()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	doc, err := st.GetDocument(ctx, strings.TrimSpace(*documentID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "no note %q\n", strings.TrimSpace(*documentID))
		os.Exit(1)
	}
	tags, err := st.ListDocumentTags(ctx, doc.ID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	report := map[string]any{
		"document_id": doc.ID,
		"title":       doc.Title,
		"notebook_id": doc.NotebookID,
		"tags":        names,
		"created_at":  doc.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":  doc.UpdatedAt.UTC().Format(time.RFC3339),
		"revision_id": doc.CurrentRevisionID,
	}
	if !doc.DeletedAt.IsZero() {
		// Reported rather than hidden: a note in Trash still exists, and a
		// script that cannot tell is a script that will overwrite one.
		report["trashed_at"] = doc.DeletedAt.UTC().Format(time.RFC3339)
	}
	if *withBody {
		report["body"] = doc.Body
	}
	printJSON(report)
}

// runNoteEdit changes a note's title or body.
//
// Only those two, and deliberately. The other properties are not the same kind
// of thing: identity and revision are history, the timestamps are consequences
// of edits rather than inputs, the notebook is a reference that has to be
// resolved and can be ambiguous, and tags are a set. Each already has its own
// command, and a single generic setter would flatten all of that and lose the
// refusals that make them safe.
func runNoteEdit(args []string) {
	fs := flag.NewFlagSet("notriosctl notes edit", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "note to edit")
	title := fs.String("title", "", "new title")
	bodyFile := fs.String("body-file", "", "read the new body from a file, or - for standard input")
	body := fs.String("body", "", "the new body as an argument")
	message := fs.String("message", "", "why this edit was made; recorded with the revision")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	id := strings.TrimSpace(*documentID)
	if fs.NArg() != 0 || id == "" {
		printNotesUsage()
		os.Exit(2)
	}
	if strings.TrimSpace(*bodyFile) != "" && *body != "" {
		fmt.Fprintln(os.Stderr, "pass --body or --body-file, not both")
		os.Exit(2)
	}
	if strings.TrimSpace(*title) == "" && strings.TrimSpace(*bodyFile) == "" && *body == "" {
		fmt.Fprintln(os.Stderr, "nothing to change: pass --title, --body or --body-file")
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	current, err := st.GetDocument(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no note %q\n", id)
		os.Exit(1)
	}
	// Unset fields keep their current value rather than being blanked. An edit
	// that changed a title and silently emptied the body would be the most
	// expensive kind of surprise this command could produce.
	request := store.UpdateDocumentRequest{
		ID: current.ID, Title: current.Title, Body: current.Body,
		BaseRevisionID: current.CurrentRevisionID,
		Message:        firstNonEmptyString(strings.TrimSpace(*message), "notriosctl notes edit"),
	}
	if trimmed := strings.TrimSpace(*title); trimmed != "" {
		request.Title = trimmed
	}
	if path := strings.TrimSpace(*bodyFile); path != "" {
		var (
			raw []byte
			err error
		)
		if path == "-" {
			raw, err = io.ReadAll(os.Stdin)
		} else {
			raw, err = os.ReadFile(path)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		request.Body = string(raw)
	} else if *body != "" {
		request.Body = *body
	}

	updated, err := st.UpdateDocument(ctx, request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"document_id": updated.ID,
		"title":       updated.Title,
		"revision_id": updated.CurrentRevisionID,
		"updated_at":  updated.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

// runNoteCreate writes one note.
//
// It exists for the same reason `notes move` does, and the case is if anything
// plainer: creating a note was reachable from the store, REST and MCP and from
// neither the command line nor a script. The absence had already been worked
// around rather than noticed -- this repository's own sync tests make a note by
// writing a Markdown file and importing it as a one-file Obsidian vault, with a
// comment explaining that the CLI cannot do it -- and the v0.8 H14 journey
// catalogue hit the same wall when it tried to document the task.
//
// The body comes from a file, an argument, or standard input. Standard input is
// the one that matters: it makes a note the end of a pipeline rather than
// something that has to be staged on disk first.
func runNoteCreate(args []string) {
	fs := flag.NewFlagSet("notriosctl notes create", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	title := fs.String("title", "", "note title")
	notebook := fs.String("notebook", "", "notebook ID, or its name when unambiguous")
	bodyFile := fs.String("body-file", "", "read the note body from a file, or - for standard input")
	body := fs.String("body", "", "the note body as an argument")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 || strings.TrimSpace(*title) == "" {
		printNotesUsage()
		fs.PrintDefaults()
		os.Exit(2)
	}
	if strings.TrimSpace(*bodyFile) != "" && *body != "" {
		fmt.Fprintln(os.Stderr, "pass --body or --body-file, not both")
		os.Exit(2)
	}

	contents := *body
	if path := strings.TrimSpace(*bodyFile); path != "" {
		var (
			raw []byte
			err error
		)
		if path == "-" {
			raw, err = io.ReadAll(os.Stdin)
		} else {
			raw, err = os.ReadFile(path)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		contents = string(raw)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	request := store.CreateDocumentRequest{
		Title:   strings.TrimSpace(*title),
		Body:    contents,
		Message: "notriosctl notes create",
	}
	if ref := strings.TrimSpace(*notebook); ref != "" {
		// Resolved before the write, and refused rather than guessed: filing a
		// note into whichever notebook matched first is the silent mistake
		// `notes move` exists to correct, and creating one wrongly is the same
		// mistake a step earlier.
		notebookID, err := resolveNotebookRef(ctx, st, ref)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		request.NotebookID = notebookID
	}
	doc, err := st.CreateDocument(ctx, request)
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
