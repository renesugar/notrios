package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// seedTagLibrary imports two notes carrying a small tag hierarchy.
func seedTagLibrary(t *testing.T, binary, dbPath, assetStore string) {
	t.Helper()
	archiveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(archiveDir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, contents string) {
		if err := os.WriteFile(filepath.Join(archiveDir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", `{"format":"notrios-archive","version":1,"query":"","notes":2}`)
	write("notebooks.json", `[{"path":"Docs"}]`)
	write(filepath.Join("notes", "doc_one.md"),
		"---\nid: doc_one\ntitle: One\nnotebook: Docs\ntags:\n  - project\n  - project/alpha\n---\n\nfirst\n")
	write(filepath.Join("notes", "doc_two.md"),
		"---\nid: doc_two\ntitle: Two\nnotebook: Docs\ntags:\n  - project/alpha\n---\n\nsecond\n")
	if result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, archiveDir); result.exitCode != 0 {
		t.Fatalf("seed import: %s", result.stderr)
	}
}

// The dry run is the default and it changes nothing; --apply is the only way to
// alter the library. A rename that a script did not mean to run should be a
// report, not a repair job.
func TestTagRenameCLIDryRunIsDefault(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	seedTagLibrary(t, binary, dbPath, assetStore)

	dry := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project", "--to", "work", "--include-children")
	if dry.exitCode != 0 {
		t.Fatalf("dry run: exit %d %s", dry.exitCode, dry.stderr)
	}
	report := decodeCLIJSON(t, dry.stdout)
	if report["dry_run"] != true {
		t.Fatalf("dry_run must default to true: %s", dry.stdout)
	}
	changes, ok := report["changes"].([]any)
	if !ok || len(changes) != 2 {
		t.Fatalf("expected two planned changes: %s", dry.stdout)
	}
	if report["notes"].(float64) != 2 {
		t.Fatalf("expected two affected notes: %s", dry.stdout)
	}

	// Nothing moved: the same dry run reports the same plan.
	again := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project", "--to", "work", "--include-children")
	if again.exitCode != 0 || len(decodeCLIJSON(t, again.stdout)["changes"].([]any)) != 2 {
		t.Fatalf("dry run was not repeatable: exit %d %s", again.exitCode, again.stdout)
	}

	applied := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project", "--to", "work", "--include-children", "--apply")
	if applied.exitCode != 0 {
		t.Fatalf("apply: exit %d %s", applied.exitCode, applied.stderr)
	}
	if decodeCLIJSON(t, applied.stdout)["dry_run"] != false {
		t.Fatalf("apply reported a dry run: %s", applied.stdout)
	}

	// The old name is gone, so planning it again is a not-found failure.
	after := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project", "--to", "work")
	if after.exitCode != 1 {
		t.Fatalf("renaming a vanished tag should fail: exit %d %s", after.exitCode, after.stdout)
	}
}

// A dry run whose plan contains a merge exits 1, so a script that meant to
// rename and would instead have combined two hierarchies stops.
func TestTagRenameCLIExitsNonZeroOnPlannedMerge(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	seedTagLibrary(t, binary, dbPath, assetStore)

	merge := runCLI(t, binary, "tags", "rename", "--db", dbPath, "--asset-store", assetStore,
		"--from", "project/alpha", "--to", "project")
	if merge.exitCode != 1 {
		t.Fatalf("a planned merge should exit 1: exit %d %s", merge.exitCode, merge.stdout)
	}
	report := decodeCLIJSON(t, merge.stdout)
	changes := report["changes"].([]any)
	if len(changes) != 1 || changes[0].(map[string]any)["action"] != "merge" {
		t.Fatalf("expected a reported merge: %s", merge.stdout)
	}
}

// TestTagAddRemoveAndList covers the commands that close v0.8 H14's running
// example: tagging a note was reachable from the store, REST and MCP and from
// neither surface a person uses.
func TestTagAddRemoveAndList(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{"--db", filepath.Join(sandbox, "notes.sqlite"), "--asset-store", filepath.Join(sandbox, "assets")}

	created := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--title", "Reed beds", "--body", "dusk"}, roots...)...)
	if created.exitCode != 0 {
		t.Fatalf("notes create: %s", created.stderr)
	}
	var note map[string]any
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatal(err)
	}
	id, _ := note["document_id"].(string)

	tagsOf := func(result cliResult) []string {
		t.Helper()
		var decoded map[string]any
		if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
			t.Fatalf("%v in %q", err, result.stdout)
		}
		names := []string{}
		for _, item := range decoded["tags"].([]any) {
			names = append(names, item.(string))
		}
		sort.Strings(names)
		return names
	}

	// A hierarchical tag, because that is the shape `tags rename` already works
	// in and the one most likely to be mishandled.
	added := runCLIIn(t, sandbox, binary, append([]string{"tags", "add", "--document", id, "--tag", "field/dusk"}, roots...)...)
	if added.exitCode != 0 {
		t.Fatalf("tags add: %s", added.stderr)
	}
	if got := tagsOf(added); len(got) != 1 || got[0] != "field/dusk" {
		t.Fatalf("after add, tags are %v", got)
	}

	second := runCLIIn(t, sandbox, binary, append([]string{"tags", "add", "--document", id, "--tag", "birds"}, roots...)...)
	if got := tagsOf(second); len(got) != 2 {
		t.Fatalf("after a second add, tags are %v", got)
	}

	// The change is verifiable from the same surface that made it, which is the
	// point of `tags list`: a command that changes something and offers no way
	// to see the change asks its caller to take it on trust.
	listed := runCLIIn(t, sandbox, binary, append([]string{"tags", "list", "--document", id}, roots...)...)
	if got := tagsOf(listed); len(got) != 2 || got[0] != "birds" || got[1] != "field/dusk" {
		t.Fatalf("tags list reports %v", got)
	}

	removed := runCLIIn(t, sandbox, binary, append([]string{"tags", "remove", "--document", id, "--tag", "field/dusk"}, roots...)...)
	if removed.exitCode != 0 {
		t.Fatalf("tags remove: %s", removed.stderr)
	}
	if got := tagsOf(removed); len(got) != 1 || got[0] != "birds" {
		t.Fatalf("after remove, tags are %v", got)
	}

	all := runCLIIn(t, sandbox, binary, append([]string{"tags", "list"}, roots...)...)
	if !strings.Contains(all.stdout, "birds") {
		t.Fatalf("the library tag list does not mention the remaining tag: %s", all.stdout)
	}
}

// TestTagRefusalsNameWhatWasAsked holds these commands to the standard the
// journey catalogue found `import --collection` failing: a refusal must say
// what the user asked for, not what the database said.
func TestTagRefusalsNameWhatWasAsked(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{"--db", filepath.Join(sandbox, "notes.sqlite"), "--asset-store", filepath.Join(sandbox, "assets")}

	created := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--title", "t", "--body", "b"}, roots...)...)
	var note map[string]any
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatal(err)
	}
	id, _ := note["document_id"].(string)

	absent := runCLIIn(t, sandbox, binary, append([]string{"tags", "remove", "--document", id, "--tag", "never-applied"}, roots...)...)
	if absent.exitCode == 0 {
		t.Errorf("removing a tag the note does not carry must fail")
	}
	if !strings.Contains(absent.stderr, "never-applied") {
		t.Errorf("the refusal must name the tag: %q", absent.stderr)
	}

	// "This note has no tags" and "there is no such note" are different answers,
	// and a command whose job is verifying an edit must not conflate them.
	missing := runCLIIn(t, sandbox, binary, append([]string{"tags", "list", "--document", "doc_nope"}, roots...)...)
	if missing.exitCode == 0 {
		t.Errorf("listing tags for a note that does not exist must fail, not report zero tags")
	}
	if !strings.Contains(missing.stderr, "doc_nope") {
		t.Errorf("the refusal must name the note: %q", missing.stderr)
	}

	if result := runCLIIn(t, sandbox, binary, append([]string{"tags", "add", "--document", id}, roots...)...); result.exitCode == 0 {
		t.Errorf("adding with no --tag must be refused")
	}
}

// TestTagShowAnswersExistenceInTheExitCode is the point of the command: a
// script can test for a tag without parsing anything. Before it, asking meant
// fetching every tag in the library and searching the result -- and
// `tags list --tag <name>` looked like the answer while returning all of them.
func TestTagShowAnswersExistenceInTheExitCode(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}
	created := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--title", "T", "--body", "b"}, roots...)...)
	var note struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatalf("reading the note id: %v", err)
	}
	for _, name := range []string{"todo", "shopping", "shopping/mall", "shoppingcart"} {
		if added := runCLIIn(t, sandbox, binary, append([]string{
			"tags", "add", "--document", note.DocumentID, "--tag", name}, roots...)...); added.exitCode != 0 {
			t.Fatalf("tagging %q: %s", name, added.stderr)
		}
	}

	present := runCLIIn(t, sandbox, binary, append([]string{"tags", "show", "--tag", "todo"}, roots...)...)
	if present.exitCode != 0 {
		t.Errorf("an existing tag exited %d: %s", present.exitCode, present.stderr)
	}
	if !strings.Contains(present.stdout, `"notes": 1`) {
		t.Errorf("the count is missing: %s", present.stdout)
	}

	absent := runCLIIn(t, sandbox, binary, append([]string{"tags", "show", "--tag", "nope"}, roots...)...)
	if absent.exitCode != 1 {
		t.Errorf("a tag that does not exist exited %d, want 1", absent.exitCode)
	}
	if strings.TrimSpace(absent.stdout) != "" {
		t.Errorf("a refusal wrote to standard output: %q", absent.stdout)
	}

	// A branch reports its children, which is part of what "how much is on this
	// tag" means when tags nest.
	branch := runCLIIn(t, sandbox, binary, append([]string{"tags", "show", "--tag", "shopping"}, roots...)...)
	if !strings.Contains(branch.stdout, "shopping/mall") {
		t.Errorf("the branch does not report its child: %s", branch.stdout)
	}
	if strings.Contains(branch.stdout, "shoppingcart") {
		t.Errorf("a tag that merely starts with the same letters was reported as a child: %s", branch.stdout)
	}
}

// TestTagListNarrowsToABranchAndReportsTruncation covers the listing half.
func TestTagListNarrowsToABranchAndReportsTruncation(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}
	created := runCLIIn(t, sandbox, binary, append([]string{"notes", "create", "--title", "T", "--body", "b"}, roots...)...)
	var note struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal([]byte(created.stdout), &note); err != nil {
		t.Fatalf("reading the note id: %v", err)
	}
	for _, name := range []string{"todo", "shopping", "shopping/mall", "shoppingcart"} {
		runCLIIn(t, sandbox, binary, append([]string{
			"tags", "add", "--document", note.DocumentID, "--tag", name}, roots...)...)
	}

	branch := runCLIIn(t, sandbox, binary, append([]string{"tags", "list", "--prefix", "shopping"}, roots...)...)
	if strings.Contains(branch.stdout, "shoppingcart") || strings.Contains(branch.stdout, "todo") {
		t.Errorf("the branch listing is not a branch: %s", branch.stdout)
	}
	if !strings.Contains(branch.stdout, "shopping/mall") {
		t.Errorf("the branch listing omits a child: %s", branch.stdout)
	}

	bounded := runCLIIn(t, sandbox, binary, append([]string{"tags", "list", "--limit", "2"}, roots...)...)
	if !strings.Contains(bounded.stdout, `"truncated": true`) {
		t.Errorf("a limit that cut the answer short did not say so: %s", bounded.stdout)
	}
	whole := runCLIIn(t, sandbox, binary, append([]string{"tags", "list"}, roots...)...)
	if !strings.Contains(whole.stdout, `"truncated": false`) {
		t.Errorf("an unbounded listing did not report truncated=false: %s", whole.stdout)
	}
}
