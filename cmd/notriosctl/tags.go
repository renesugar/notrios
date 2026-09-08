package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// runTags dispatches the tag subcommands.
func runTags(args []string) {
	if len(args) == 0 {
		printTagsUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "add":
		runTagAdd(args[1:])
	case "remove":
		runTagRemove(args[1:])
	case "list":
		runTagList(args[1:])
	case "rename":
		runTagRename(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown tags subcommand %q\n", args[0])
		printTagsUsage()
		os.Exit(2)
	}
}

func printTagsUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl tags add --document <id> --tag <tag>
  notriosctl tags remove --document <id> --tag <tag>
  notriosctl tags list [--document <id>]
  notriosctl tags rename --from <tag> --to <tag> [--include-children] [--apply]
`)
}

// tagFlags is the shared shape of the three note-level tag commands.
type tagFlags struct {
	set                            *flag.FlagSet
	configPath, dbPath, assetStore *string
	document, tag                  *string
}

func newTagFlags(name string) *tagFlags {
	set := flag.NewFlagSet("notriosctl tags "+name, flag.ExitOnError)
	return &tagFlags{
		set:        set,
		configPath: set.String("config", "", "optional config file"),
		dbPath:     set.String("db", "", "SQLite database path override"),
		assetStore: set.String("asset-store", "", "asset store directory override"),
		document:   set.String("document", "", "note to act on"),
		tag:        set.String("tag", "", "tag name"),
	}
}

// newTagListFlags is `tags list`, which takes no `--tag`.
//
// It is a separate function rather than a flag on the one above because
// sharing gave `tags list` a `--tag` it never read: `notriosctl tags list --tag
// todo` was accepted and returned every tag in the library, which reads as a
// filter that found everything. A flag accepted and discarded is worse than one
// that does not exist, because the caller cannot tell.
//
// Written without a conditional so that what this command accepts can be read
// off the source. A branch here would leave the flags gate guessing, and a gate
// that guesses is one that reports a problem nobody can act on.
func newTagListFlags() *tagFlags {
	set := flag.NewFlagSet("notriosctl tags list", flag.ExitOnError)
	unused := ""
	return &tagFlags{
		set:        set,
		configPath: set.String("config", "", "optional config file"),
		dbPath:     set.String("db", "", "SQLite database path override"),
		assetStore: set.String("asset-store", "", "asset store directory override"),
		document:   set.String("document", "", "note to act on"),
		tag:        &unused,
	}
}

func (f *tagFlags) parse(args []string) {
	if err := f.set.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// runTagAdd attaches one tag to one note.
//
// Adding and removing a tag were reachable from the store, REST and MCP and
// from neither the command line nor the GUI. v0.8 H14 found that by reading the
// documentation, then again by comparing the two surfaces mechanically; this
// closes the command-line half.
//
// There is no dry run, for the reason `notes move` gives: adding a tag is not
// destructive and not lossy, and the way to undo it is `tags remove` with the
// same arguments. A confirmation step would be ceremony rather than safety.
func runTagAdd(args []string) {
	flags := newTagFlags("add")
	flags.parse(args)
	documentID, tag := requireTagTarget(flags)

	st := openStoreFromFlags(*flags.configPath, *flags.dbPath, *flags.assetStore)
	defer st.Close()
	ctx := context.Background()

	if _, err := st.AddDocumentTag(ctx, documentID, tag); err != nil {
		exitTagError(err, documentID, tag, "add")
	}
	// The note's tags are printed back rather than a bare confirmation, because
	// the question a caller actually has is what the note carries now, and a
	// script that has to run a second command to find out is a script that will
	// not bother.
	printDocumentTags(ctx, st, documentID, "added", tag)
}

// runTagRemove takes one tag off one note.
//
// Removing a tag a note does not have is reported as an error rather than
// passed over: a script that misspells a tag and is told nothing happened has
// been told the truth, and a script told nothing at all has not.
func runTagRemove(args []string) {
	flags := newTagFlags("remove")
	flags.parse(args)
	documentID, tag := requireTagTarget(flags)

	st := openStoreFromFlags(*flags.configPath, *flags.dbPath, *flags.assetStore)
	defer st.Close()
	ctx := context.Background()

	if err := st.RemoveDocumentTag(ctx, documentID, tag); err != nil {
		exitTagError(err, documentID, tag, "remove")
	}
	printDocumentTags(ctx, st, documentID, "removed", tag)
}

// runTagList reports the tags on one note, or every tag in the library.
//
// It exists so that adding and removing are verifiable from the same surface
// that performs them. A command that changes something and offers no way to see
// the change asks its caller to take it on trust.
func runTagList(args []string) {
	flags := newTagListFlags()
	flags.parse(args)
	if flags.set.NArg() != 0 {
		printTagsUsage()
		os.Exit(2)
	}

	st := openStoreFromFlags(*flags.configPath, *flags.dbPath, *flags.assetStore)
	defer st.Close()
	ctx := context.Background()

	if documentID := strings.TrimSpace(*flags.document); documentID != "" {
		printDocumentTags(ctx, st, documentID, "", "")
		return
	}
	tags, err := st.ListTags(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rows := make([]map[string]any, 0, len(tags))
	for _, tag := range tags {
		rows = append(rows, map[string]any{"tag": tag.Name, "notes": tag.NoteCount})
	}
	printJSON(map[string]any{"tags": rows})
}

// exitTagError names what the user asked for.
//
// The store answers a missing note and a missing tag with the same bare "not
// found", which tells a caller nothing about which of the two they got wrong.
// That is the failure this milestone criticised elsewhere -- `import
// --collection` reporting a raw `FOREIGN KEY constraint failed` for a
// collection that does not exist -- and it would be poor to reproduce it in the
// command written to close that class of gap.
func exitTagError(err error, documentID, tag, action string) {
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		fmt.Fprintf(os.Stderr, "cannot %s tag %q: no note %q, or it does not carry that tag\n",
			action, tag, documentID)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func requireTagTarget(flags *tagFlags) (string, string) {
	documentID := strings.TrimSpace(*flags.document)
	tag := strings.TrimSpace(*flags.tag)
	if flags.set.NArg() != 0 || documentID == "" || tag == "" {
		printTagsUsage()
		flags.set.PrintDefaults()
		os.Exit(2)
	}
	return documentID, tag
}

// printDocumentTags reports what the note carries now.
func printDocumentTags(ctx context.Context, st *store.SQLiteStore, documentID, action, tag string) {
	// Checked first, because ListDocumentTags answers a note that does not
	// exist with an empty list. "This note has no tags" and "there is no such
	// note" are different answers, and a command whose job is verifying an
	// edit must not report the first when the second is true.
	if _, err := st.GetDocument(ctx, documentID); err != nil {
		fmt.Fprintf(os.Stderr, "no note %q\n", documentID)
		os.Exit(1)
	}
	tags, err := st.ListDocumentTags(ctx, documentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	names := make([]string, 0, len(tags))
	for _, item := range tags {
		names = append(names, item.Name)
	}
	report := map[string]any{"document_id": documentID, "tags": names}
	if action != "" {
		report["action"] = action
		report["tag"] = tag
	}
	printJSON(report)
}

// runTagRename renames a tag hierarchy.
//
// Dry run is the default, as it is for `gc` and `fix`. The report it prints is
// not a prediction: the service runs the rename inside a transaction and rolls
// it back, so what a dry run prints is what an apply does.
//
// Exit 1 when a rename would merge tags without `--apply`, so a script that
// meant to rename and would instead have combined two hierarchies stops rather
// than continuing.
func runTagRename(args []string) {
	fs := flag.NewFlagSet("notriosctl tags rename", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	from := fs.String("from", "", "tag to rename")
	to := fs.String("to", "", "new tag name")
	includeChildren := fs.Bool("include-children", false, "also rename every tag under <from>/")
	apply := fs.Bool("apply", false, "perform the rename (default: dry run)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 0 || *from == "" || *to == "" {
		fmt.Fprintln(os.Stderr, "usage: notriosctl tags rename --from <tag> --to <tag> [--include-children] [--apply]")
		fs.PrintDefaults()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()

	result, err := st.RenameTag(context.Background(), store.TagRenameRequest{
		From:            *from,
		To:              *to,
		IncludeChildren: *includeChildren,
		DryRun:          !*apply,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(result)

	if !*apply {
		for _, change := range result.Changes {
			if change.Action == store.TagRenameActionMerge {
				fmt.Fprintf(os.Stderr, "refusing to continue silently: %q would merge into an existing %q; re-run with --apply if that is intended\n", change.From, change.To)
				os.Exit(1)
			}
		}
	}
}
