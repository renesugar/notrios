package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const kickoffTemplate = "```note-template\n" +
	"description: Kickoff note for a new project\n" +
	"prompt: project — the project this note is about\n" +
	"prompt: owner\n" +
	"```\n" +
	"# {{project}}\n\nOwner: {{owner}}\nStarted: {{date}} in {{notebook}}\n"

func seedTemplate(t *testing.T, st *SQLiteStore, title, body string) Document {
	t.Helper()
	doc, err := st.CreateDocument(context.Background(), CreateDocumentRequest{Title: title, Body: body})
	if err != nil {
		t.Fatalf("CreateDocument(%q): %v", title, err)
	}
	return doc
}

func TestTemplateDeclaresPromptsAndVariables(t *testing.T) {
	st := newOrganizerTestStore(t)
	doc := seedTemplate(t, st, "Project kickoff", kickoffTemplate)

	template, err := st.GetTemplate(context.Background(), doc.ID)
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if template.Error != "" {
		t.Fatalf("unexpected error: %s", template.Error)
	}
	if len(template.Prompts) != 2 || template.Prompts[0].Name != "project" || template.Prompts[1].Name != "owner" {
		t.Fatalf("prompts wrong: %+v", template.Prompts)
	}
	// The label is the prose after the name, so a dialog can explain itself.
	if !strings.Contains(template.Prompts[0].Label, "the project this note is about") {
		t.Fatalf("label lost: %+v", template.Prompts[0])
	}
	// Only the automatic names actually used are reported.
	if strings.Join(template.Variables, ",") != "date,notebook" {
		t.Fatalf("variables wrong: %v", template.Variables)
	}
	if template.Description == "" {
		t.Fatal("description lost")
	}
}

func TestCreateFromTemplateSubstitutesAndStripsTheBlock(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	doc := seedTemplate(t, st, "Project kickoff", kickoffTemplate)

	created, err := st.CreateFromTemplate(ctx, CreateFromTemplateRequest{
		TemplateID: doc.ID,
		Title:      "Apollo kickoff",
		Values:     map[string]string{"project": "Apollo", "owner": "Rene"},
	})
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}
	if strings.Contains(created.Body, "note-template") {
		t.Fatalf("the declaration must not survive into the note:\n%s", created.Body)
	}
	if !strings.Contains(created.Body, "# Apollo") || !strings.Contains(created.Body, "Owner: Rene") {
		t.Fatalf("substitution failed:\n%s", created.Body)
	}
	if strings.Contains(created.Body, "{{") {
		t.Fatalf("an unsubstituted placeholder survived:\n%s", created.Body)
	}
	// The automatic notebook name is the note's real notebook.
	if !strings.Contains(created.Body, "in Notes") {
		t.Fatalf("notebook substitution wrong:\n%s", created.Body)
	}
}

// The security property: a supplied value is data. It is inserted once and
// never re-scanned, so it cannot introduce a placeholder or recurse.
func TestTemplateValuesAreNeverReExpanded(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	doc := seedTemplate(t, st, "Echo", "```note-template\nprompt: text\n```\nBody: {{text}}\n")

	created, err := st.CreateFromTemplate(ctx, CreateFromTemplateRequest{
		TemplateID: doc.ID,
		Title:      "Echo test",
		Values:     map[string]string{"text": "{{date}} and {{text}}"},
	})
	if err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}
	if !strings.Contains(created.Body, "Body: {{date}} and {{text}}") {
		t.Fatalf("a supplied value must be inserted literally, not expanded:\n%s", created.Body)
	}
}

func TestTemplateRefusals(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	good := seedTemplate(t, st, "Good", kickoffTemplate)

	// A missing value is refused rather than silently blank.
	if _, err := st.CreateFromTemplate(ctx, CreateFromTemplateRequest{
		TemplateID: good.ID, Title: "x", Values: map[string]string{"project": "Apollo"},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a missing value must be refused, got %v", err)
	}
	// A value for a prompt that does not exist is a caller bug, not ignored.
	if _, err := st.CreateFromTemplate(ctx, CreateFromTemplateRequest{
		TemplateID: good.ID, Title: "x",
		Values: map[string]string{"project": "A", "owner": "B", "nonsense": "C"},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an undeclared value must be refused, got %v", err)
	}

	// A template whose body uses an undeclared placeholder is broken, and says
	// so on listing rather than at creation time.
	typo := seedTemplate(t, st, "Typo", "```note-template\nprompt: owner\n```\nOwner: {{onwer}}\n")
	broken, err := st.GetTemplate(ctx, typo.ID)
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if !strings.Contains(broken.Error, "unknown placeholder {{onwer}}") {
		t.Fatalf("expected the typo to be reported: %q", broken.Error)
	}
	if !strings.Contains(broken.Error, "date, notebook") && !strings.Contains(broken.Error, "date") {
		t.Fatalf("the error should name the automatic vocabulary: %q", broken.Error)
	}
	if _, err := st.CreateFromTemplate(ctx, CreateFromTemplateRequest{
		TemplateID: typo.ID, Title: "x", Values: map[string]string{"owner": "R"},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("a malformed template must refuse to create, got %v", err)
	}

	// An automatic name cannot be prompted for: two sources for one name would
	// make which one wins a coin toss.
	clash := seedTemplate(t, st, "Clash", "```note-template\nprompt: date\n```\n{{date}}\n")
	clashed, _ := st.GetTemplate(ctx, clash.ID)
	if !strings.Contains(clashed.Error, "automatic name") {
		t.Fatalf("expected a reserved-name error: %q", clashed.Error)
	}

	// An unknown key names the ones that work.
	badKey := seedTemplate(t, st, "Bad key", "```note-template\nexec: rm -rf /\n```\nhi\n")
	bad, _ := st.GetTemplate(ctx, badKey.ID)
	if !strings.Contains(bad.Error, "unknown key") || !strings.Contains(bad.Error, "prompt") {
		t.Fatalf("expected an actionable unknown-key error: %q", bad.Error)
	}
}

func TestListTemplatesFindsOnlyTemplates(t *testing.T) {
	ctx := context.Background()
	st := newOrganizerTestStore(t)
	seedTemplate(t, st, "Project kickoff", kickoffTemplate)
	seedTemplate(t, st, "Ordinary note", "# Just a note\n\nNo template here.\n")

	templates, err := st.ListTemplates(ctx, "default")
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(templates) != 1 || templates[0].Title != "Project kickoff" {
		t.Fatalf("expected exactly the template: %+v", templates)
	}
}
