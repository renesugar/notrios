package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/markdownblocks"
	"github.com/renesugar/notrios/internal/projection"
	"github.com/renesugar/notrios/internal/store"
)

// This file is the reading half of `notes`, added in v0.8 H21.
//
// Reading a note's structure was reachable over REST and MCP and nowhere else:
// its outline, its attachments and its links each had an endpoint and a tool
// and no command. The features registry recorded that as a decision -- "No
// command line: reading a note is what the GUI and the API are for" -- which
// was false in both directions, since `notes show` already read a note and
// nobody had decided a terminal should not.

// writeOut sends rendered bytes to a file or to standard output.
//
// A file is offered because the natural next thing to do with a rendered note
// is keep it, and `>` in a shell loses the exit code of the command that
// produced it.
func writeOut(path string, content []byte) {
	if strings.TrimSpace(path) == "" {
		os.Stdout.Write(content)
		return
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// documentOrExit reads a note, reporting a trashed one as trashed.
//
// GetDocument excludes Trash, so a deleted note would otherwise report as "no
// such note" -- the answer someone gets immediately after deleting, when they
// are trying to confirm what happened and find the id to restore.
func documentOrExit(ctx context.Context, st *store.SQLiteStore, id string) store.Document {
	doc, err := st.GetDocument(ctx, id)
	if err == nil {
		return doc
	}
	found, lookupErr := findTrashedDocument(ctx, st, id)
	if lookupErr != nil || found.ID == "" {
		fmt.Fprintf(os.Stderr, "no note %q\n", id)
		os.Exit(1)
	}
	return found
}

// runNoteOutline reports a note's headings and the anchors a link can name.
func runNoteOutline(args []string) {
	fs := flag.NewFlagSet("notriosctl notes outline", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "note to read")
	output := fs.String("output", "", "write to a file instead of standard output")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	wanted := strings.TrimSpace(*documentID)
	if fs.NArg() != 0 || wanted == "" {
		printNotesUsage()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()
	doc := documentOrExit(ctx, st, wanted)

	// The same parse that stores each heading's slug, so an anchor printed here
	// is one a stable link resolves against. Two parsers disagreed on anything
	// outside ASCII until H21 unified them.
	headings := []map[string]any{}
	for _, block := range markdownblocks.Extract(doc.ID, doc.Body) {
		if block.Kind != markdownblocks.KindHeading {
			continue
		}
		headings = append(headings, map[string]any{
			"level":  block.Level,
			"title":  strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(block.Text), "#")),
			"anchor": block.Slug,
			"line":   1 + strings.Count(doc.Body[:block.StartByte], "\n"),
		})
	}
	writeOut(*output, jsonBytes(map[string]any{"document_id": doc.ID, "headings": headings}))
}

// runNoteResources reports what is attached to a note.
func runNoteResources(args []string) {
	fs := flag.NewFlagSet("notriosctl notes resources", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "note to read")
	output := fs.String("output", "", "write to a file instead of standard output")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	wanted := strings.TrimSpace(*documentID)
	if fs.NArg() != 0 || wanted == "" {
		printNotesUsage()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()
	doc := documentOrExit(ctx, st, wanted)

	references, err := st.ListDocumentResources(ctx, doc.ID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rows := make([]map[string]any, 0, len(references))
	for _, reference := range references {
		rows = append(rows, map[string]any{
			"resource_id":   reference.ResourceID,
			"filename":      reference.Resource.Filename,
			"mime_type":     reference.Resource.MIMEType,
			"size_bytes":    reference.Resource.SizeBytes,
			"sha256":        reference.Resource.SHA256,
			"relation_type": reference.RelationType,
		})
	}
	writeOut(*output, jsonBytes(map[string]any{"document_id": doc.ID, "resources": rows}))
}

// runNoteLinks reports the links a note carries.
//
// Broken links are reported rather than filtered, because a broken link is the
// interesting one: a listing that silently omitted them would answer "what does
// this note point at?" with the subset that still works.
func runNoteLinks(args []string) {
	fs := flag.NewFlagSet("notriosctl notes links", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "note to read")
	direction := fs.String("direction", "outgoing", "outgoing, incoming, or both")
	output := fs.String("output", "", "write to a file instead of standard output")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	wanted := strings.TrimSpace(*documentID)
	if fs.NArg() != 0 || wanted == "" {
		printNotesUsage()
		os.Exit(2)
	}
	// Checked here as well as in the store, and with the store's own words: a
	// direction it does not recognise returns an empty page rather than an
	// error, so a typo would report "this note has no links" and be believed.
	switch strings.TrimSpace(*direction) {
	case "outgoing", "incoming", "both":
	default:
		fmt.Fprintln(os.Stderr, `--direction must be "outgoing", "incoming" or "both"`)
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()
	doc := documentOrExit(ctx, st, wanted)

	page, err := st.ListDocumentLinks(ctx, doc.ID, strings.TrimSpace(*direction))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report := map[string]any{"document_id": doc.ID, "direction": strings.TrimSpace(*direction)}
	switch strings.TrimSpace(*direction) {
	case "outgoing":
		report["links"] = page.Outgoing
	case "incoming":
		report["links"] = page.Incoming
	default:
		report["outgoing"] = page.Outgoing
		report["incoming"] = page.Incoming
	}
	writeOut(*output, jsonBytes(report))
}

// runResourceGet writes one attachment's bytes to a file.
//
// It lives under `resources` rather than under `notes` because a resource is
// addressable on its own and several notes may reference the same one; hanging
// retrieval off a single note would misdescribe the model.
func runResourceGet(args []string) {
	fs := flag.NewFlagSet("notriosctl resources get", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	resourceID := fs.String("resource", "", "attachment to read")
	output := fs.String("output", "", "file to write; standard output when omitted")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	wanted := strings.TrimSpace(*resourceID)
	if fs.NArg() != 0 || wanted == "" {
		printResourcesUsage()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	resource, reader, err := st.OpenResourceContent(ctx, wanted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no attachment %q: %v\n", wanted, err)
		os.Exit(1)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	writeOut(*output, content)
	if strings.TrimSpace(*output) != "" {
		// Reported on stderr so a caller redirecting stdout still sees it, and
		// so a file written here is exactly the stored bytes and nothing else.
		fmt.Fprintf(os.Stderr, "%s  %s  %d bytes\n", resource.ID, resource.MIMEType, len(content))
	}
}

// renderNoteMarkdown is `notes show`'s default: the note as a Markdown file
// another application can read.
//
// It takes the document rather than its id because the caller has already
// resolved a trashed note, and re-reading by id would turn "this note is in
// Trash" back into "no such note".
func renderNoteMarkdown(ctx context.Context, st *store.SQLiteStore, doc store.Document) []byte {
	content, err := projection.RenderDocument(ctx, st, doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return content
}

// jsonBytes renders a value the way printJSON does, for the paths that may write
// to a file instead of standard output.
func jsonBytes(v any) []byte {
	content, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return append(content, '\n')
}
