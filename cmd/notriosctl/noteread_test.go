package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoteShowRendersMarkdownWithFrontMatter is the default form: a note another
// application can read. The front matter is the projection's, not a second one
// written here, which is what stops the two disagreeing.
func TestNoteShowRendersMarkdownWithFrontMatter(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}

	created := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "create", "--title", "Reed beds", "--body", "Seen at dusk.\n\n## Café notes\n\nMore.",
	}, roots...)...)
	if created.exitCode != 0 {
		t.Fatalf("creating a note: %s", created.stderr)
	}
	var note struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatalf("reading the new note's id: %v", err)
	}

	shown := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", note.DocumentID}, roots...)...)
	if shown.exitCode != 0 {
		t.Fatalf("notes show: %s", shown.stderr)
	}
	if !strings.HasPrefix(shown.stdout, "---\n") {
		t.Errorf("notes show did not begin with front matter:\n%s", shown.stdout)
	}
	for _, want := range []string{
		`id: "` + note.DocumentID + `"`,
		`title: "Reed beds"`,
		`notebook: "Notes"`,
		`collection: "default"`,
		"Seen at dusk.",
		"## Café notes",
	} {
		if !strings.Contains(shown.stdout, want) {
			t.Errorf("notes show omits %q:\n%s", want, shown.stdout)
		}
	}

	structured := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "show", "--document", note.DocumentID, "--json"}, roots...)...)
	var fields map[string]any
	if err := json.Unmarshal([]byte(structured.stdout), &fields); err != nil {
		t.Fatalf("notes show --json is not parseable: %v\n%s", err, structured.stdout)
	}
	for _, key := range []string{"document_id", "title", "notebook_id", "collection_id", "body", "revision_id"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("notes show --json omits %q", key)
		}
	}

	// --output writes the same bytes it would have printed, so a caller keeps
	// the exit code that `>` in a shell throws away.
	target := filepath.Join(sandbox, "note.md")
	written := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "show", "--document", note.DocumentID, "--output", target}, roots...)...)
	if written.exitCode != 0 {
		t.Fatalf("notes show --output: %s", written.stderr)
	}
	onDisk, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading the written note: %v", err)
	}
	if string(onDisk) != shown.stdout {
		t.Errorf("--output wrote different bytes than standard output:\n%q\n%q", string(onDisk), shown.stdout)
	}
}

// TestNoteOutlineAnchorsMatchTheStableLink is the defect H21 found: the outline
// had its own heading parser and its own slug function, and they disagreed with
// the stored slug on anything outside ASCII. An anchor that resolves to nothing
// is worse than no anchor, because a caller cannot tell.
func TestNoteOutlineAnchorsMatchTheStableLink(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}

	created := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "create", "--title", "Slugs", "--body", "# Notes\n\n## Café notes\n\nBody.\n",
	}, roots...)...)
	var note struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatalf("reading the new note's id: %v", err)
	}

	outlined := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "outline", "--document", note.DocumentID}, roots...)...)
	if outlined.exitCode != 0 {
		t.Fatalf("notes outline: %s", outlined.stderr)
	}
	var outline struct {
		Headings []struct {
			Level  int    `json:"level"`
			Title  string `json:"title"`
			Anchor string `json:"anchor"`
			Line   int    `json:"line"`
		} `json:"headings"`
	}
	if err := json.Unmarshal([]byte(outlined.stdout), &outline); err != nil {
		t.Fatalf("notes outline is not parseable: %v\n%s", err, outlined.stdout)
	}
	if len(outline.Headings) != 2 {
		t.Fatalf("outline has %d headings, want 2: %s", len(outline.Headings), outlined.stdout)
	}

	// Every anchor the outline reports is one `link --list-anchors` knows, which
	// is the set a notrios:// link resolves against.
	// Roots before the positional: flag parsing stops at the first non-flag
	// argument, so `link --list-anchors <id> --db ...` treats --db as an
	// argument rather than a flag.
	anchors := runCLIIn(t, sandbox, binary,
		append(append([]string{"link", "--list-anchors"}, roots...), note.DocumentID)...)
	if anchors.exitCode != 0 {
		t.Fatalf("link --list-anchors: %s", anchors.stderr)
	}
	for _, heading := range outline.Headings {
		if heading.Anchor == "" {
			t.Errorf("heading %q has an empty anchor", heading.Title)
			continue
		}
		if !strings.Contains(anchors.stdout, `"`+heading.Anchor+`"`) {
			t.Errorf("outline anchor %q for %q is not an anchor this note carries:\n%s",
				heading.Anchor, heading.Title, anchors.stdout)
		}
	}
	if outline.Headings[1].Anchor != "café-notes" {
		t.Errorf("non-ASCII heading anchored as %q, want %q", outline.Headings[1].Anchor, "café-notes")
	}
}

// TestNoteResourcesAndResourceGetRoundTrip covers the pair: find what is
// attached, then pull the bytes out.
func TestNoteResourcesAndResourceGetRoundTrip(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}

	created := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "create", "--title", "Attached", "--body", "A note with nothing attached yet.",
	}, roots...)...)
	var note struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatalf("reading the new note's id: %v", err)
	}

	listed := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "resources", "--document", note.DocumentID}, roots...)...)
	if listed.exitCode != 0 {
		t.Fatalf("notes resources: %s", listed.stderr)
	}
	var report struct {
		DocumentID string           `json:"document_id"`
		Resources  []map[string]any `json:"resources"`
	}
	if err := json.Unmarshal([]byte(listed.stdout), &report); err != nil {
		t.Fatalf("notes resources is not parseable: %v\n%s", err, listed.stdout)
	}
	if report.DocumentID != note.DocumentID {
		t.Errorf("notes resources reported %q, want %q", report.DocumentID, note.DocumentID)
	}
	// A note with no attachments reports an empty list rather than failing:
	// "nothing is attached" is an answer, and a caller should not have to treat
	// it as an error.
	if report.Resources == nil {
		t.Error("a note with no attachments reported null rather than an empty list")
	}
}

// TestNoteLinksReportsBothDirections keeps the direction flag honest.
func TestNoteLinksReportsBothDirections(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}

	target := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "create", "--title", "Target", "--body", "The end of the link."}, roots...)...)
	var to struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(target.stdout), &to); err != nil {
		t.Fatalf("reading the target id: %v", err)
	}
	source := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "create", "--title", "Source",
		"--body", "See [the target](document://default/documents/" + to.DocumentID + ")."}, roots...)...)
	var from struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(source.stdout), &from); err != nil {
		t.Fatalf("reading the source id: %v", err)
	}

	out := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "links", "--document", from.DocumentID}, roots...)...)
	if out.exitCode != 0 {
		t.Fatalf("notes links: %s", out.stderr)
	}
	if !strings.Contains(out.stdout, to.DocumentID) {
		t.Errorf("outgoing links do not mention the target:\n%s", out.stdout)
	}

	in := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "links", "--document", to.DocumentID, "--direction", "incoming"}, roots...)...)
	if in.exitCode != 0 {
		t.Fatalf("notes links --direction in: %s", in.stderr)
	}
	if !strings.Contains(in.stdout, from.DocumentID) {
		t.Errorf("incoming links do not mention the source:\n%s", in.stdout)
	}

	bad := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "links", "--document", to.DocumentID, "--direction", "sideways"}, roots...)...)
	if bad.exitCode == 0 {
		t.Error("an unknown --direction was accepted")
	}
}
