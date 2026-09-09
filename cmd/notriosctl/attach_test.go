package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func attachLibrary(t *testing.T) (string, string, []string, string) {
	t.Helper()
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}
	created := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "create", "--title", "Field notes", "--body", "Seen at dusk."}, roots...)...)
	var note struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatalf("reading the note id: %v", err)
	}
	return binary, sandbox, roots, note.DocumentID
}

// TestAddingAFileNeverTouchesTheBody is the boundary the command exists to
// respect: placing the link is the author's, and a command that wrote one would
// be guessing at the one thing only the writer knows.
func TestAddingAFileNeverTouchesTheBody(t *testing.T) {
	binary, sandbox, roots, note := attachLibrary(t)
	before := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", note}, roots...)...)

	file := filepath.Join(sandbox, "photo.png")
	if err := os.WriteFile(file, []byte("\x89PNG\r\n\x1a\nnot really a png"), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}
	added := runCLIIn(t, sandbox, binary, append([]string{
		"resources", "add", "--file", file, "--document", note}, roots...)...)
	if added.exitCode != 0 {
		t.Fatalf("resources add: %s", added.stderr)
	}
	var report struct {
		ResourceID string `json:"resource_id"`
		URI        string `json:"uri"`
		MIMEType   string `json:"mime_type"`
		SizeBytes  int64  `json:"size_bytes"`
		SHA256     string `json:"sha256"`
	}
	if err := json.Unmarshal([]byte(added.stdout), &report); err != nil {
		t.Fatalf("resources add is not parseable: %v\n%s", err, added.stdout)
	}
	for name, value := range map[string]string{
		"resource_id": report.ResourceID, "uri": report.URI,
		"mime_type": report.MIMEType, "sha256": report.SHA256,
	} {
		if value == "" {
			t.Errorf("the report omits %s: %s", name, added.stdout)
		}
	}
	if !strings.HasPrefix(report.URI, "resource://") {
		t.Errorf("the URI to paste is not a resource:// link: %q", report.URI)
	}

	after := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", note}, roots...)...)
	if after.stdout != before.stdout {
		t.Errorf("adding a file changed the note body:\nbefore %q\nafter  %q", before.stdout, after.stdout)
	}

	// The reference is recorded even though the body is not, which is what
	// `notes resources` lists and what stops the file becoming litter.
	listed := runCLIIn(t, sandbox, binary, append([]string{"notes", "resources", "--document", note}, roots...)...)
	if !strings.Contains(listed.stdout, report.ResourceID) {
		t.Errorf("the attachment is not recorded against the note: %s", listed.stdout)
	}

	// And the bytes come back exactly.
	target := filepath.Join(sandbox, "roundtrip.png")
	got := runCLIIn(t, sandbox, binary, append([]string{
		"resources", "get", "--resource", report.ResourceID, "--output", target}, roots...)...)
	if got.exitCode != 0 {
		t.Fatalf("resources get: %s", got.stderr)
	}
	original, _ := os.ReadFile(file)
	returned, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading the round-tripped file: %v", err)
	}
	if string(returned) != string(original) {
		t.Errorf("the bytes came back different: %q vs %q", string(returned), string(original))
	}
}

// TestTheStoredTypeComesFromTheBytes is the defect this slice found. The store
// sniffs a file whose caller supplied no type, and then threw the answer away:
// NormalizeCreateResourceRequest fills the absent type with
// "application/octet-stream" before the sniff runs, and the request's value
// then won over the sniffed one. A PNG was recorded as octet-stream by a store
// that had already identified it.
func TestTheStoredTypeComesFromTheBytes(t *testing.T) {
	binary, sandbox, roots, _ := attachLibrary(t)

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"real.png", "\x89PNG\r\n\x1a\nAAAA", "image/png"},
		{"misnamed.png", "just text here\n", "text/plain"},
	}
	for _, test := range cases {
		file := filepath.Join(sandbox, test.name)
		if err := os.WriteFile(file, []byte(test.content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", test.name, err)
		}
		added := runCLIIn(t, sandbox, binary, append([]string{"resources", "add", "--file", file}, roots...)...)
		var report struct {
			MIMEType string `json:"mime_type"`
		}
		if err := json.Unmarshal([]byte(added.stdout), &report); err != nil {
			t.Fatalf("resources add: %v\n%s", err, added.stdout)
		}
		if !strings.HasPrefix(report.MIMEType, test.want) {
			t.Errorf("%s was recorded as %q, want %s — the extension won over the bytes",
				test.name, report.MIMEType, test.want)
		}
	}
}

// TestAppendAndPrependPlaceTheLinkWithoutRewritingTheNote is why those commands
// are here: no surface can patch a range, so without them placing a link means
// reading the whole note and writing it back.
func TestAppendAndPrependPlaceTheLinkWithoutRewritingTheNote(t *testing.T) {
	binary, sandbox, roots, note := attachLibrary(t)

	appended := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "append", "--document", note, "--text", "![](resource://default/resources/res_x)"}, roots...)...)
	if appended.exitCode != 0 {
		t.Fatalf("notes append: %s", appended.stderr)
	}
	prepended := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "prepend", "--document", note, "--text", "# Field notes"}, roots...)...)
	if prepended.exitCode != 0 {
		t.Fatalf("notes prepend: %s", prepended.stderr)
	}

	shown := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", note}, roots...)...)
	body := shown.stdout
	if !strings.Contains(body, "# Field notes") || !strings.Contains(body, "resource://default/resources/res_x") {
		t.Fatalf("the additions are not in the note: %s", body)
	}
	// The original text survives both, which whole-body replacement is exactly
	// what would put at risk.
	if !strings.Contains(body, "Seen at dusk.") {
		t.Errorf("appending lost the note's original text: %s", body)
	}
	if strings.Index(body, "# Field notes") > strings.Index(body, "Seen at dusk.") {
		t.Errorf("prepend did not put its text first: %s", body)
	}

	// A stale precondition is refused rather than applied to whatever is there.
	stale := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "append", "--document", note, "--text", "x", "--base-revision", "rev_gone"}, roots...)...)
	if stale.exitCode == 0 {
		t.Error("an append against a revision that is not current was accepted")
	}
}
