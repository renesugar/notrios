package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// Tasks and templates from the command line.
//
// Both are read-computed views over ordinary Markdown rather than tables: a
// task is a checkbox line in a note somebody tagged, and a template is a note
// carrying a ```note-template block. Nothing here creates a task, because
// nothing can -- writing `- [ ] chase the permit` into a note is how a task
// comes to exist, which `notes create` already does.
//
// What was missing is the other half of that loop. A script can write work into
// a library and had no way to ask what remained, and `templates create` -- the
// repeatable version of exactly that workflow -- had no command at all despite
// being a write, which is the shape the command line is for.
func runTasks(args []string) {
	if len(args) == 0 || args[0] != "list" {
		printTasksUsage()
		os.Exit(2)
	}
	fs := flag.NewFlagSet("notriosctl tasks list", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	documentID := fs.String("document", "", "restrict to one note")
	notebookID := fs.String("notebook", "", "restrict to one notebook")
	state := fs.String("state", "", "open, done, or empty for both")
	limit := fs.Int("limit", 0, "maximum tasks to print (0 = default)")
	untagged := fs.Bool("untagged", false,
		"include notes carrying no task tag; by default only notes tagged "+strings.Join(store.TaskTags, " or "))
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()

	list, err := st.ListTasks(context.Background(), store.TaskListRequest{
		DocumentID: strings.TrimSpace(*documentID),
		NotebookID: strings.TrimSpace(*notebookID),
		State:      strings.TrimSpace(*state),
		Limit:      *limit,
		Untagged:   *untagged,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(list)
}

func printTasksUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl tasks list [--document <id>] [--notebook <id>] [--state open|done] [--untagged]
`)
}

// runTemplates lists templates, and makes notes from them.
func runTemplates(args []string) {
	if len(args) == 0 {
		printTemplatesUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "list":
		runTemplateList(args[1:])
	case "create":
		runTemplateCreate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown templates command %q\n", args[0])
		printTemplatesUsage()
		os.Exit(2)
	}
}

func printTemplatesUsage() {
	fmt.Fprint(os.Stderr, `usage:
  notriosctl templates list [--db ...]
  notriosctl templates create --template <id> --title <title> [--notebook <id|name>] [--set name=value ...]
`)
}

func runTemplateList(args []string) {
	fs := flag.NewFlagSet("notriosctl templates list", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()

	// Listing exists so `templates create` is usable: the identifier it needs
	// is not discoverable from a terminal any other way.
	templates, err := st.ListTemplates(context.Background(), "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{"templates": templates})
}

// placeholderValues collects repeated --set name=value pairs.
type placeholderValues map[string]string

func (v placeholderValues) String() string { return fmt.Sprintf("%v", map[string]string(v)) }

func (v placeholderValues) Set(raw string) error {
	name, value, found := strings.Cut(raw, "=")
	if !found || strings.TrimSpace(name) == "" {
		return fmt.Errorf("--set expects name=value, got %q", raw)
	}
	if _, repeated := v[name]; repeated {
		// Refused rather than last-wins: a script that sets one placeholder
		// twice has a bug, and picking a winner hides it.
		return fmt.Errorf("--set %s was given twice", name)
	}
	v[name] = value
	return nil
}

func runTemplateCreate(args []string) {
	fs := flag.NewFlagSet("notriosctl templates create", flag.ExitOnError)
	configPath := fs.String("config", "", "optional config file")
	dbPath := fs.String("db", "", "SQLite database path override")
	assetStore := fs.String("asset-store", "", "asset store directory override")
	templateID := fs.String("template", "", "required: the template note's document ID")
	title := fs.String("title", "", "required: the new note's title")
	notebook := fs.String("notebook", "", "notebook ID, or its name when unambiguous")
	values := placeholderValues{}
	fs.Var(values, "set", "placeholder value as name=value; repeat for each")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if strings.TrimSpace(*templateID) == "" || strings.TrimSpace(*title) == "" {
		printTemplatesUsage()
		fs.PrintDefaults()
		os.Exit(2)
	}

	st := openStoreFromFlags(*configPath, *dbPath, *assetStore)
	defer st.Close()
	ctx := context.Background()

	request := store.CreateFromTemplateRequest{
		TemplateID: strings.TrimSpace(*templateID),
		Title:      strings.TrimSpace(*title),
		Values:     values,
	}
	if ref := strings.TrimSpace(*notebook); ref != "" {
		notebookID, err := resolveNotebookRef(ctx, st, ref)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		request.NotebookID = notebookID
	}
	// A missing placeholder is refused by the store rather than filled with a
	// blank, which is what makes this safe to script: a template that gains a
	// field fails the runs that do not know about it instead of quietly
	// producing notes with a hole in them.
	doc, err := st.CreateFromTemplate(ctx, request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printJSON(map[string]any{
		"document_id": doc.ID, "title": doc.Title, "notebook_id": doc.NotebookID,
	})
}
