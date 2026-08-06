package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopEntryRegistersOnlyTheNotriosScheme(t *testing.T) {
	entry := desktopEntry("/usr/local/bin/notriosctl", "")
	for _, want := range []string{
		"MimeType=x-scheme-handler/notrios;",
		"Exec=/usr/local/bin/notriosctl open --launch %u",
		"Type=Application",
	} {
		if !strings.Contains(entry, want) {
			t.Fatalf("desktop entry missing %q:\n%s", want, entry)
		}
	}
	// A handler that claimed http/https or file would hijack ordinary browsing.
	for _, forbidden := range []string{"x-scheme-handler/http", "x-scheme-handler/file", "%f", "%U"} {
		if strings.Contains(entry, forbidden) {
			t.Fatalf("desktop entry claims more than it should (%q):\n%s", forbidden, entry)
		}
	}

	// A desktop launch has no working directory of the user's choosing, so a
	// configured handler must carry the config path rather than fall back to
	// built-in defaults and open somebody else's service.
	configured := desktopEntry("/usr/local/bin/notriosctl", "/etc/notrios/config.yaml")
	if !strings.Contains(configured, "Exec=/usr/local/bin/notriosctl open --config /etc/notrios/config.yaml --launch %u") {
		t.Fatalf("configured desktop entry:\n%s", configured)
	}
}

func TestLocalNoteURLCarriesTheDocumentAndAnchor(t *testing.T) {
	url := localNoteURL("", "doc_abc", "heading-two")
	if !strings.HasSuffix(url, "/#document=doc_abc&anchor=heading-two") {
		t.Fatalf("unexpected local URL: %q", url)
	}
	if !strings.HasPrefix(url, "http://127.0.0.1") {
		t.Fatalf("the deep link must stay on the local service: %q", url)
	}
}

// buildCLI compiles notriosctl once so the exit-code contract can be tested
// the way an OS protocol handler experiences it.
func buildCLI(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping CLI build in short mode")
	}
	binary := filepath.Join(t.TempDir(), "notriosctl")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build notriosctl: %v", err)
	}
	return binary
}

type cliResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func runCLI(t *testing.T, binary string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(binary, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := cliResult{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run %v: %v", args, err)
		}
		result.exitCode = exit.ExitCode()
	}
	return result
}

func decodeCLIJSON(t *testing.T, output string) map[string]any {
	t.Helper()
	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode CLI output %q: %v", output, err)
	}
	return decoded
}

// The whole point of a stable link is that it names a logical database rather
// than a path, so the registry is what turns it back into a local database —
// and it must refuse rather than guess whenever that mapping is not exactly
// one database.
func TestStableLinkRoutingThroughTheProfileRegistry(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	registry := filepath.Join(workspace, "profiles.json")

	primaryDB := filepath.Join(workspace, "primary", "notes.sqlite")
	primaryAssets := filepath.Join(workspace, "primary", "assets")
	secondaryDB := filepath.Join(workspace, "secondary", "notes.sqlite")
	secondaryAssets := filepath.Join(workspace, "secondary", "assets")

	// Two independent databases: two logical database IDs.
	for _, db := range []struct{ path, assets, name string }{
		{primaryDB, primaryAssets, "primary"},
		{secondaryDB, secondaryAssets, "secondary"},
	} {
		result := runCLI(t, binary, "profile", "register", "--name", db.name,
			"--db", db.path, "--asset-store", db.assets, "--registry", registry)
		if result.exitCode != 0 {
			t.Fatalf("register %s: exit %d %s", db.name, result.exitCode, result.stderr)
		}
	}

	// Create a note in the primary database and take its stable link.
	documentID := createNoteForCLI(t, binary, primaryDB, primaryAssets)
	linkResult := runCLI(t, binary, "link", "--db", primaryDB, "--asset-store", primaryAssets, documentID)
	if linkResult.exitCode != 0 {
		t.Fatalf("link: exit %d %s", linkResult.exitCode, linkResult.stderr)
	}
	linkOutput := decodeCLIJSON(t, linkResult.stdout)
	uri, _ := linkOutput["stable_uri"].(string)
	if !strings.HasPrefix(uri, "notrios://databases/") {
		t.Fatalf("unexpected stable URI %q", uri)
	}

	// Resolved through the registry, with no database flag at all.
	opened := runCLI(t, binary, "open", "--registry", registry, uri)
	if opened.exitCode != 0 {
		t.Fatalf("open: exit %d %s", opened.exitCode, opened.stderr)
	}
	openOutput := decodeCLIJSON(t, opened.stdout)
	if openOutput["status"] != "resolved" || openOutput["profile"] != "primary" {
		t.Fatalf("unexpected resolution: %+v", openOutput)
	}
	if openOutput["document_id"] != documentID {
		t.Fatalf("resolved the wrong note: %+v", openOutput)
	}

	// The same link checked against the other database must be refused, not
	// answered with whatever local note happens to share the ID.
	foreign := runCLI(t, binary, "open", "--db", secondaryDB, "--asset-store", secondaryAssets, uri)
	if foreign.exitCode != 1 {
		t.Fatalf("a link for another database must fail, got exit %d", foreign.exitCode)
	}
	foreignOutput := decodeCLIJSON(t, foreign.stdout)
	if foreignOutput["status"] != "foreign_database" {
		t.Fatalf("unexpected foreign status: %+v", foreignOutput)
	}
	if _, leaked := foreignOutput["document_uri"]; leaked {
		t.Fatalf("a foreign link must not report local note state: %+v", foreignOutput)
	}

	// An unregistered database is reported as such rather than falling back to
	// the only database this machine happens to have.
	unknown := runCLI(t, binary, "open", "--registry", registry,
		"notrios://databases/db_not_registered/documents/"+documentID)
	if unknown.exitCode != 1 {
		t.Fatalf("unregistered database: exit %d", unknown.exitCode)
	}
	if decodeCLIJSON(t, unknown.stdout)["status"] != "unregistered_database" {
		t.Fatalf("unexpected output: %s", unknown.stdout)
	}

	// A malformed link is a usage error, distinct from an unresolvable one.
	malformed := runCLI(t, binary, "open", "--registry", registry, "notrios://databases/db_a/resources/res_b")
	if malformed.exitCode != 2 {
		t.Fatalf("malformed link should exit 2, got %d (%s)", malformed.exitCode, malformed.stderr)
	}
}

func TestClonedDatabasesAreReportedAsAmbiguousRatherThanPicked(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	registry := filepath.Join(workspace, "profiles.json")

	originalDB := filepath.Join(workspace, "original", "notes.sqlite")
	originalAssets := filepath.Join(workspace, "original", "assets")
	documentID := createNoteForCLI(t, binary, originalDB, originalAssets)

	// A raw filesystem copy carries the same logical database ID: exactly the
	// case the identity contract says cannot announce itself as a clone.
	cloneDB := filepath.Join(workspace, "clone", "notes.sqlite")
	if err := os.MkdirAll(filepath.Dir(cloneDB), 0o755); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(originalDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cloneDB, contents, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, db := range []struct{ path, name string }{{originalDB, "original"}, {cloneDB, "clone"}} {
		if result := runCLI(t, binary, "profile", "register", "--name", db.name,
			"--db", db.path, "--asset-store", filepath.Join(filepath.Dir(db.path), "assets"),
			"--registry", registry); result.exitCode != 0 {
			t.Fatalf("register %s: %s", db.name, result.stderr)
		}
	}

	linkOutput := decodeCLIJSON(t, mustRunCLI(t, binary, "link", "--db", originalDB,
		"--asset-store", originalAssets, documentID))
	uri, _ := linkOutput["stable_uri"].(string)

	ambiguous := runCLI(t, binary, "open", "--registry", registry, uri)
	if ambiguous.exitCode != 1 {
		t.Fatalf("ambiguity must not resolve, got exit %d", ambiguous.exitCode)
	}
	output := decodeCLIJSON(t, ambiguous.stdout)
	if output["status"] != "ambiguous_database" {
		t.Fatalf("unexpected status: %+v", output)
	}
	candidates, ok := output["candidates"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("every candidate must be offered: %+v", output)
	}

	// Naming a profile settles it.
	chosen := runCLI(t, binary, "open", "--registry", registry, "--profile", "clone", uri)
	if chosen.exitCode != 0 {
		t.Fatalf("explicit profile: exit %d %s", chosen.exitCode, chosen.stderr)
	}
	if decodeCLIJSON(t, chosen.stdout)["profile"] != "clone" {
		t.Fatalf("wrong profile answered: %s", chosen.stdout)
	}
}

func mustRunCLI(t *testing.T, binary string, args ...string) string {
	t.Helper()
	result := runCLI(t, binary, args...)
	if result.exitCode != 0 {
		t.Fatalf("%v: exit %d %s", args, result.exitCode, result.stderr)
	}
	return result.stdout
}

// createNoteForCLI seeds a database with one note by importing a tiny archive,
// which is the only note-creating path the CLI exposes.
func createNoteForCLI(t *testing.T, binary, dbPath, assetStore string) string {
	t.Helper()
	archiveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(archiveDir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"format":"notrios-archive","version":1,"query":"","notes":1}`
	if err := os.WriteFile(filepath.Join(archiveDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archiveDir, "notebooks.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	note := "---\nid: doc_stable_link_fixture\ntitle: Linked note\nnotebook: Notes\n---\n\nBody text.\n"
	if err := os.WriteFile(filepath.Join(archiveDir, "notes", "doc_stable_link_fixture.md"), []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}
	result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, archiveDir)
	if result.exitCode != 0 {
		t.Fatalf("seed import: exit %d %s", result.exitCode, result.stderr)
	}
	listed := decodeCLIJSON(t, mustRunCLI(t, binary, "link", "--db", dbPath, "--asset-store", assetStore, "doc_stable_link_fixture"))
	documentID, _ := listed["document_id"].(string)
	if documentID == "" {
		t.Fatalf("seeded note has no ID: %+v", listed)
	}
	return documentID
}

// A stable link can name a section. The anchor is checked before it is printed:
// a link meant to be pasted somewhere permanent should not be one that never
// resolved.
func TestLinkEmitsAnchoredStableLinks(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")

	archiveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(archiveDir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, contents string) {
		if err := os.WriteFile(filepath.Join(archiveDir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", `{"format":"notrios-archive","version":1,"query":"","notes":1}`)
	write("notebooks.json", `[{"path":"Docs"}]`)
	write(filepath.Join("notes", "doc_guide.md"),
		"---\nid: doc_guide\ntitle: Guide\nnotebook: Docs\n---\n\n# Getting Started\n\nIntro.\n\n## Install & Setup\n\nSteps. ^install-note\n")
	if result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, archiveDir); result.exitCode != 0 {
		t.Fatalf("seed import: %s", result.stderr)
	}
	shared := []string{"--db", dbPath, "--asset-store", assetStore}

	listed := decodeCLIJSON(t, mustRunCLI(t, binary, append([]string{"link", "--list-anchors"}, append(shared, "doc_guide")...)...))
	anchors, ok := listed["anchors"].([]any)
	if !ok || len(anchors) == 0 {
		t.Fatalf("expected anchors: %+v", listed)
	}

	// By heading slug.
	bySlug := decodeCLIJSON(t, mustRunCLI(t, binary, append([]string{"link", "--anchor", "install-setup"}, append(shared, "doc_guide")...)...))
	if bySlug["anchor_kind"] != "heading" || !strings.HasSuffix(bySlug["stable_uri"].(string), "#install-setup") {
		t.Fatalf("heading anchor: %+v", bySlug)
	}

	// By heading text — the spelling Obsidian uses — normalizes to the slug.
	byText := decodeCLIJSON(t, mustRunCLI(t, binary, append([]string{"link", "--anchor", "Install & Setup"}, append(shared, "doc_guide")...)...))
	if byText["stable_uri"] != bySlug["stable_uri"] {
		t.Fatalf("heading text should normalize to the slug: %v vs %v", byText["stable_uri"], bySlug["stable_uri"])
	}

	// By author-written marker, which keeps its caret.
	byMarker := decodeCLIJSON(t, mustRunCLI(t, binary, append([]string{"link", "--anchor", "^install-note"}, append(shared, "doc_guide")...)...))
	if !strings.HasSuffix(byMarker["stable_uri"].(string), "#^install-note") {
		t.Fatalf("marker anchor: %+v", byMarker)
	}

	// An anchor that does not resolve is refused rather than printed.
	missing := runCLI(t, binary, append([]string{"link", "--anchor", "no-such-heading"}, append(shared, "doc_guide")...)...)
	if missing.exitCode == 0 {
		t.Fatal("an unresolvable anchor must not be printed as a link")
	}
	if !strings.Contains(missing.stderr, "list-anchors") {
		t.Fatalf("the refusal should say how to find the anchors: %s", missing.stderr)
	}

	// The anchored link resolves end to end.
	opened := runCLI(t, binary, append([]string{"open"}, append(shared, bySlug["stable_uri"].(string))...)...)
	if opened.exitCode != 0 {
		t.Fatalf("anchored link did not open: exit %d %s", opened.exitCode, opened.stderr)
	}
	if decodeCLIJSON(t, opened.stdout)["status"] != "resolved" {
		t.Fatalf("unexpected resolution: %s", opened.stdout)
	}
}
