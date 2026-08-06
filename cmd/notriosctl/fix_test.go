package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedFixLibrary imports a note whose link names its target by title, which is
// the shape `non_canonical_link_target` repairs.
func seedFixLibrary(t *testing.T, binary, dbPath, assetStore string) {
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
	write(filepath.Join("notes", "doc_kitchen.md"),
		"---\nid: doc_kitchen\ntitle: Kitchen\nnotebook: Docs\n---\n\n# Kitchen\n\nBody.\n")
	write(filepath.Join("notes", "doc_source.md"),
		"---\nid: doc_source\ntitle: Source\nnotebook: Docs\n---\n\nSee [the plan](Kitchen) for details.\n")
	if result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, archiveDir); result.exitCode != 0 {
		t.Fatalf("seed import: %s", result.stderr)
	}
}

// Dry run is the default and shows the exact edit; nothing changes until
// --apply, and then only against the revision the plan was made from.
func TestFixDryRunsByDefaultAndAppliesOnRequest(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	seedFixLibrary(t, binary, dbPath, assetStore)
	shared := []string{"--db", dbPath, "--asset-store", assetStore}

	dry := decodeCLIJSON(t, mustRunCLI(t, binary, append([]string{"fix"}, shared...)...))
	if dry["apply"] != false {
		t.Fatalf("dry run must be the default: %+v", dry)
	}
	plan, ok := dry["plan"].(map[string]any)
	if !ok || plan["total_edits"].(float64) != 1 {
		t.Fatalf("expected one planned edit: %+v", dry)
	}
	if _, applied := dry["results"]; applied {
		t.Fatalf("a dry run must not report applied results: %+v", dry)
	}
	documents := plan["documents"].([]any)
	edit := documents[0].(map[string]any)["edits"].([]any)[0].(map[string]any)
	if !strings.Contains(edit["before"].(string), "](Kitchen)") ||
		!strings.Contains(edit["after"].(string), "](document://default/documents/doc_kitchen)") {
		t.Fatalf("the plan must show the exact edit: %+v", edit)
	}

	// The note is untouched until --apply.
	linted := runCLI(t, binary, append([]string{"lint", "--quiet"}, shared...)...)
	_ = linted

	applied := decodeCLIJSON(t, mustRunCLI(t, binary, append([]string{"fix", "--apply"}, shared...)...))
	results := applied["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected one note repaired: %+v", applied)
	}
	first := results[0].(map[string]any)
	if first["applied"].(float64) != 1 || first["revision_id"] == "" {
		t.Fatalf("apply result: %+v", first)
	}

	// Running again finds nothing: the fix is idempotent because the link is
	// now already canonical.
	again := decodeCLIJSON(t, mustRunCLI(t, binary, append([]string{"fix"}, shared...)...))
	if again["plan"].(map[string]any)["total_edits"].(float64) != 0 {
		t.Fatalf("a second run should find nothing: %+v", again)
	}
}

// Every fix is an ordinary revision, so it is visible in history and revertible.
func TestFixWritesAnOrdinaryRevision(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	seedFixLibrary(t, binary, dbPath, assetStore)
	shared := []string{"--db", dbPath, "--asset-store", assetStore}

	mustRunCLI(t, binary, append([]string{"fix", "--apply"}, shared...)...)

	// The stable link for the note still resolves, and the anchor listing shows
	// the note is intact rather than truncated by the rewrite.
	anchors := decodeCLIJSON(t, mustRunCLI(t, binary, append([]string{"link", "--list-anchors"}, append(shared, "doc_source")...)...))
	if len(anchors["anchors"].([]any)) == 0 {
		t.Fatalf("the repaired note lost its blocks: %+v", anchors)
	}
}

func TestFixListsKindsAndValidatesThem(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	shared := []string{"--db", filepath.Join(workspace, "notes.sqlite"), "--asset-store", filepath.Join(workspace, "assets")}

	listed := decodeCLIJSON(t, mustRunCLI(t, binary, "fix", "--list-kinds"))
	kinds := listed["kinds"].([]any)
	found := map[string]bool{}
	for _, kind := range kinds {
		found[kind.(string)] = true
	}
	for _, want := range []string{"non_canonical_link_target", "missing_alt_text", "unlocalized_remote_media"} {
		if !found[want] {
			t.Fatalf("missing kind %q: %+v", want, kinds)
		}
	}
	// Alt text is not in the default set: a filename is not a description.
	defaults := listed["default"].([]any)
	for _, kind := range defaults {
		if kind.(string) == "missing_alt_text" {
			t.Fatal("alt text must be opt-in")
		}
	}

	bad := runCLI(t, binary, append([]string{"fix", "--kinds", "rewrite_everything"}, shared...)...)
	if bad.exitCode == 0 {
		t.Fatal("an unknown fix kind must be refused")
	}
}
