package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// This file is the writing half of attachments, added in v0.8 H27.
//
// The command line could read attachments and not add one, and that was
// recorded as a boundary: attaching means putting a `resource://` link at a
// point in the body only the author knows. The reasoning was right about the
// placement and wrong to stop there. The product already separates three acts —
// the resource, the reference to it, and the link in the body — so a command
// that does the first two and prints the URI for the third guesses at nothing.

// runResourceAdd puts local bytes into the library and prints what to paste.
func runResourceAdd(args []string) {
	fs := flag.NewFlagSet("notriosctl resources add", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	path := fs.String("file", "", "file to put into the library")
	filename := fs.String("filename", "", "name to record for a reader; defaults to the file's own")
	documentID := fs.String("document", "", "also record the attachment against this note")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	source := strings.TrimSpace(*path)
	if fs.NArg() != 0 || source == "" {
		printResourcesUsage()
		os.Exit(2)
	}

	handle, err := os.Open(source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer handle.Close()
	if info, err := handle.Stat(); err == nil && info.Size() > store.MaxResourceContentBytes {
		// Refused rather than truncated, and refused before any bytes move: a
		// half-written resource is worse than none, and the ceiling is the same
		// one the HTTP surface enforces.
		fmt.Fprintf(os.Stderr, "%s is %d bytes; the limit is %d\n",
			source, info.Size(), store.MaxResourceContentBytes)
		os.Exit(1)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	name := strings.TrimSpace(*filename)
	if name == "" {
		name = filepath.Base(source)
	}
	// No MIME type is supplied, deliberately. The store sniffs the bytes when
	// the caller offers none, and passing the extension's guess would suppress
	// that -- an extension is what somebody typed, not what the file is. The
	// HTTP surface takes a Content-Type because a client sends one; there is no
	// such claim here, so the bytes decide.
	resource, err := st.CreateResource(ctx, store.CreateResourceRequest{
		Filename: name, Content: handle,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	report := map[string]any{
		"resource_id": resource.ID,
		"uri":         resource.URI,
		"filename":    resource.Filename,
		"mime_type":   resource.MIMEType,
		"size_bytes":  resource.SizeBytes,
		"sha256":      resource.SHA256,
	}
	if note := strings.TrimSpace(*documentID); note != "" {
		doc := documentOrExit(ctx, st, note)
		if _, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{
			DocumentID: doc.ID, ResourceID: resource.ID, RelationType: "attachment",
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		report["document_id"] = doc.ID
	}
	// The body is untouched, and the URI is the thing to paste. Said in the
	// output because a caller who does not know that will go looking for the
	// link in the note and not find it.
	report["next_step"] = "place the link yourself: notriosctl notes append --document <id> --text '![](" +
		resource.URI + ")'"
	printJSON(report)
}

// runNoteAppend adds text to the end of a note.
//
// It exists because the alternative was worse than it sounds. No surface can
// patch a range of a note body — REST and MCP can read one and neither can
// write one — so without this, placing a link means reading the whole note,
// editing it elsewhere and writing the whole note back, losing any concurrent
// edit in between. Append and prepend are the two writes that need no range,
// and REST and MCP have both; this is the missing adapter rather than a new
// capability.
func runNoteAppend(args []string) { runNoteJoin(args, false) }

// runNotePrepend adds text to the start of a note.
func runNotePrepend(args []string) { runNoteJoin(args, true) }

func runNoteJoin(args []string, prepend bool) {
	verb := "append"
	if prepend {
		verb = "prepend"
	}
	fs := flag.NewFlagSet("notriosctl notes "+verb, flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "note to add to")
	text := fs.String("text", "", "text to add")
	textFile := fs.String("text-file", "", "read the text from a file, or - for standard input")
	baseRevision := fs.String("base-revision", "", "refuse unless the note is still at this revision")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	wanted := strings.TrimSpace(*documentID)
	if fs.NArg() != 0 || wanted == "" {
		printNotesUsage()
		os.Exit(2)
	}
	addition, err := readTextArgument(*text, *textFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if addition == "" {
		fmt.Fprintln(os.Stderr, "--text or --text-file is required, and must not be empty")
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	// Without an explicit precondition, apply to the current revision and retry
	// once if another writer lands in between — the same rule the REST append
	// follows, because a person appending from a terminal wants the same
	// forgiveness a client gets.
	attempts := 1
	if strings.TrimSpace(*baseRevision) == "" {
		attempts = 2
	}
	for attempt := 0; attempt < attempts; attempt++ {
		current := documentOrExit(ctx, st, wanted)
		base := strings.TrimSpace(*baseRevision)
		if base == "" {
			base = current.CurrentRevisionID
		}
		updated, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID:             current.ID,
			Title:          current.Title,
			Body:           store.JoinNoteText(current.Body, addition, prepend),
			BaseRevisionID: base,
			Message:        "notriosctl notes " + verb,
		})
		if err == nil {
			printJSON(map[string]any{
				"document_id": updated.ID,
				"revision_id": updated.CurrentRevisionID,
				verb + "ed":   true,
			})
			return
		}
		if attempt+1 < attempts && strings.Contains(strings.ToLower(err.Error()), "revision conflict") {
			continue
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// readTextArgument takes the addition from a flag, a file, or standard input.
func readTextArgument(text, file string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return text, nil
	}
	if text != "" {
		return "", fmt.Errorf("--text and --text-file name two different sources; send one")
	}
	if file == "-" {
		content, err := io.ReadAll(os.Stdin)
		return string(content), err
	}
	content, err := os.ReadFile(file)
	return string(content), err
}
