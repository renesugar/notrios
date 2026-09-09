package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// batchLibrary builds a library with three notes carrying one tag and one that
// does not, so a query can be wrong in both directions.
func batchLibrary(t *testing.T) (string, string, []string, []string) {
	t.Helper()
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}
	ids := []string{}
	for _, title := range []string{"Kettle", "Kettle descaling", "Kettle warranty", "Bicycle"} {
		created := runCLIIn(t, sandbox, binary, append([]string{
			"notes", "create", "--title", title, "--body", title + " notes."}, roots...)...)
		if created.exitCode != 0 {
			t.Fatalf("notes create %q: %s", title, created.stderr)
		}
		var note struct {
			DocumentID string `json:"document_id"`
		}
		if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
			t.Fatalf("reading the note id: %v", err)
		}
		ids = append(ids, note.DocumentID)
	}
	for _, id := range ids[:3] {
		tagged := runCLIIn(t, sandbox, binary, append([]string{
			"tags", "add", "--document", id, "--tag", "kitchen"}, roots...)...)
		if tagged.exitCode != 0 {
			t.Fatalf("tags add: %s", tagged.stderr)
		}
	}
	return binary, sandbox, roots, ids
}

type batchReport struct {
	DryRun   bool   `json:"dry_run"`
	Selected int    `json:"selected"`
	Query    string `json:"query"`
	Notes    []struct {
		DocumentID string `json:"document_id"`
		Title      string `json:"title"`
	} `json:"notes"`
	Operation  string `json:"operation"`
	Applied    int    `json:"applied"`
	Skipped    int    `json:"skipped"`
	Failed     int    `json:"failed"`
	RolledBack int    `json:"rolled_back"`
	Items      []struct {
		DocumentID    string `json:"document_id"`
		Status        string `json:"status"`
		NewDocumentID string `json:"new_document_id"`
	} `json:"items"`
}

func parseBatch(t *testing.T, out string) batchReport {
	t.Helper()
	var report batchReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("the batch report is not parseable: %v\n%s", err, out)
	}
	return report
}

// TestQueryShowsTheSelectionBeforeItActs is the property the dry run exists
// for. A query is written by a person guessing at what it matches, and the
// difference between three notes and thirty is not visible until something
// shows it.
func TestQueryShowsTheSelectionBeforeItActs(t *testing.T) {
	binary, sandbox, roots, ids := batchLibrary(t)

	shown := runCLIIn(t, sandbox, binary, append([]string{
		"tags", "add", "--query", "tag:kitchen", "--tag", "audit"}, roots...)...)
	if shown.exitCode != 0 {
		t.Fatalf("the dry run failed: %s", shown.stderr)
	}
	report := parseBatch(t, shown.stdout)
	if !report.DryRun || report.Selected != 3 {
		t.Fatalf("expected a dry run over three notes, got %+v", report)
	}
	if report.Operation != "add_tags" {
		t.Errorf("the report does not name what it would do: %q", report.Operation)
	}

	// Nothing changed, which is the whole claim.
	for _, id := range ids[:3] {
		listed := runCLIIn(t, sandbox, binary, append([]string{"tags", "list", "--document", id}, roots...)...)
		if strings.Contains(listed.stdout, "audit") {
			t.Fatalf("the dry run tagged %s anyway: %s", id, listed.stdout)
		}
	}

	applied := runCLIIn(t, sandbox, binary, append([]string{
		"tags", "add", "--query", "tag:kitchen", "--tag", "audit", "--apply"}, roots...)...)
	if applied.exitCode != 0 {
		t.Fatalf("the applied run failed: %s", applied.stderr)
	}
	done := parseBatch(t, applied.stdout)
	if done.Applied != 3 || done.Failed != 0 {
		t.Fatalf("expected three applied and none failed, got %+v", done)
	}
	for _, id := range ids[:3] {
		listed := runCLIIn(t, sandbox, binary, append([]string{"tags", "list", "--document", id}, roots...)...)
		if !strings.Contains(listed.stdout, "audit") {
			t.Errorf("%s was not tagged: %s", id, listed.stdout)
		}
	}
	// And the note the query did not name is untouched, which is the failure a
	// batch over a query is most likely to have.
	untouched := runCLIIn(t, sandbox, binary, append([]string{"tags", "list", "--document", ids[3]}, roots...)...)
	if strings.Contains(untouched.stdout, "audit") {
		t.Errorf("a note the query did not match was tagged: %s", untouched.stdout)
	}
}

// TestQueryTrashCarriesTheRevisionPrecondition is the one operation the store
// refuses without a base revision, and a search hit does not carry one. Without
// this the destructive operation would be the one a query could not name.
func TestQueryTrashCarriesTheRevisionPrecondition(t *testing.T) {
	binary, sandbox, roots, ids := batchLibrary(t)

	deleted := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "delete", "--query", "tag:kitchen", "--apply"}, roots...)...)
	if deleted.exitCode != 0 {
		t.Fatalf("deleting by query failed: %s\n%s", deleted.stderr, deleted.stdout)
	}
	report := parseBatch(t, deleted.stdout)
	if report.Applied != 3 || report.Failed != 0 {
		t.Fatalf("expected three trashed, got %+v", report)
	}
	for _, id := range ids[:3] {
		shown := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", id}, roots...)...)
		if !strings.Contains(strings.ToLower(shown.stdout+shown.stderr), "trash") {
			t.Errorf("%s does not read as trashed: %s%s", id, shown.stdout, shown.stderr)
		}
	}

	// And back out again, which is the pair `notes delete` was added with.
	restored := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "restore", "--query", "is:trashed tag:kitchen", "--apply"}, roots...)...)
	if restored.exitCode != 0 {
		t.Fatalf("restoring by query failed: %s\n%s", restored.stderr, restored.stdout)
	}
	if back := parseBatch(t, restored.stdout); back.Applied != 3 {
		t.Fatalf("expected three restored, got %+v", back)
	}
}

// TestQueryMoveAndDuplicateReachTheirOperations covers the remaining two of the
// six, so that no batch operation is reachable from REST and not from here.
func TestQueryMoveAndDuplicateReachTheirOperations(t *testing.T) {
	binary, sandbox, roots, _ := batchLibrary(t)

	made := runCLIIn(t, sandbox, binary, append([]string{"notebooks", "create", "--name", "Kitchen"}, roots...)...)
	if made.exitCode != 0 {
		t.Fatalf("notebooks create: %s", made.stderr)
	}
	moved := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "move", "--query", "tag:kitchen", "--notebook", "Kitchen", "--apply"}, roots...)...)
	if moved.exitCode != 0 {
		t.Fatalf("moving by query failed: %s\n%s", moved.stderr, moved.stdout)
	}
	if report := parseBatch(t, moved.stdout); report.Applied != 3 {
		t.Fatalf("expected three moved, got %+v", report)
	}

	copied := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "duplicate", "--query", "tag:kitchen", "--apply"}, roots...)...)
	if copied.exitCode != 0 {
		t.Fatalf("duplicating by query failed: %s\n%s", copied.stderr, copied.stdout)
	}
	report := parseBatch(t, copied.stdout)
	if report.Applied != 3 {
		t.Fatalf("expected three copies, got %+v", report)
	}
	for _, item := range report.Items {
		if item.NewDocumentID == "" {
			t.Errorf("a copy was made and the report does not name it: %+v", item)
		}
	}
}

// TestQueryAndDocumentAreRefusedTogether stops the ambiguity rather than
// picking one. A command given both has been asked two different questions, and
// answering the wrong one silently is the failure a batch cannot undo.
func TestQueryAndDocumentAreRefusedTogether(t *testing.T) {
	binary, sandbox, roots, ids := batchLibrary(t)

	both := runCLIIn(t, sandbox, binary, append([]string{
		"notes", "delete", "--document", ids[0], "--query", "tag:kitchen", "--apply"}, roots...)...)
	if both.exitCode == 0 {
		t.Fatalf("naming a note and a query at once was accepted: %s", both.stdout)
	}
	if !strings.Contains(both.stderr, "--query") {
		t.Errorf("the refusal does not say what was wrong: %s", both.stderr)
	}
	// And the note it named is still there: a refusal that acted first would be
	// worse than one that acted wrongly.
	shown := runCLIIn(t, sandbox, binary, append([]string{"notes", "show", "--document", ids[0]}, roots...)...)
	if shown.exitCode != 0 {
		t.Errorf("the refused command deleted the note anyway: %s", shown.stderr)
	}
}

// TestAQueryThatMatchesNothingIsAnError is the case a script most needs told.
// A batch over an empty selection is a typo almost every time, and reporting
// "0 applied" with a zero exit code hides it.
func TestAQueryThatMatchesNothingIsAnError(t *testing.T) {
	binary, sandbox, roots, _ := batchLibrary(t)
	empty := runCLIIn(t, sandbox, binary, append([]string{
		"tags", "add", "--query", "tag:nosuchtag", "--tag", "audit", "--apply"}, roots...)...)
	if empty.exitCode == 0 {
		t.Fatalf("an empty selection was reported as success: %s", empty.stdout)
	}
	if !strings.Contains(empty.stderr, "no notes") {
		t.Errorf("the refusal does not say the query matched nothing: %s", empty.stderr)
	}
}

// TestASelectionOverTheLimitIsRefusedNotTruncated is the ceiling the REST
// surface enforces, said the same way here.
func TestASelectionOverTheLimitIsRefusedNotTruncated(t *testing.T) {
	binary, sandbox, roots, _ := batchLibrary(t)
	over := runCLIIn(t, sandbox, binary, append([]string{
		"tags", "add", "--query", "tag:kitchen", "--tag", "audit", "--limit", "2", "--apply"}, roots...)...)
	if over.exitCode == 0 {
		t.Fatalf("a selection past --limit was acted on: %s", over.stdout)
	}
	if !strings.Contains(over.stderr, "narrow it") {
		t.Errorf("the refusal does not say what to do: %s", over.stderr)
	}
	// Nothing was applied, rather than the first two.
	listed := runCLIIn(t, sandbox, binary, append([]string{"tags", "list", "--prefix", "audit"}, roots...)...)
	if strings.Contains(listed.stdout, "\"audit\"") {
		t.Errorf("a refused batch tagged something anyway: %s", listed.stdout)
	}
}
