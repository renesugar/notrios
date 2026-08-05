package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedPublishLibrary creates a database holding one public note that links to a
// note held back as private, so the publication has something to withhold.
func seedPublishLibrary(t *testing.T, binary, dbPath, assetStore string) {
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
	write("notebooks.json", `[{"path":"Public"},{"path":"Private"}]`)
	write(filepath.Join("notes", "doc_public.md"),
		"---\nid: doc_public\ntitle: Public note\nnotebook: Public\ntags: [published]\n---\n\n"+
			"See [the internal one](document://default/documents/doc_private) for details.\n")
	write(filepath.Join("notes", "doc_private.md"),
		"---\nid: doc_private\ntitle: Private note\nnotebook: Private\ntags: [internal]\n---\n\nInternal only.\n")

	if result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, archiveDir); result.exitCode != 0 {
		t.Fatalf("seed import: exit %d %s", result.exitCode, result.stderr)
	}
}

// Publishing is explicit after a reviewed plan: the plan prints a digest over
// the complete selection, and the run refuses any digest that no longer
// describes this library.
func TestPublishRunRequiresTheReviewedPlanDigest(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, "notes.sqlite")
	assetStore := filepath.Join(workspace, "assets")
	profiles := filepath.Join(workspace, "publish-profiles.json")
	seedPublishLibrary(t, binary, dbPath, assetStore)

	shared := []string{"--db", dbPath, "--asset-store", assetStore, "--profiles", profiles}
	save := append([]string{"publish", "profile", "save", "--name", "public-site",
		"--query", `notebook:"Public"`, "--link-action", "plain_text"}, shared...)
	if result := runCLI(t, binary, save...); result.exitCode != 0 {
		t.Fatalf("profile save: exit %d %s", result.exitCode, result.stderr)
	}

	planResult := runCLI(t, binary, append([]string{"publish", "plan", "--profile", "public-site"}, shared...)...)
	if planResult.exitCode != 0 {
		t.Fatalf("publish plan: exit %d %s", planResult.exitCode, planResult.stderr)
	}
	plan := decodeCLIJSON(t, planResult.stdout)
	planBody, ok := plan["plan"].(map[string]any)
	if !ok {
		t.Fatalf("plan output: %s", planResult.stdout)
	}
	digest, _ := planBody["manifest_sha256"].(string)
	if digest == "" {
		t.Fatalf("the plan must print a digest to review: %s", planResult.stdout)
	}
	// Assert the selection is real. An empty selection would make every
	// withholding check below pass without publishing anything.
	counts, _ := planBody["counts"].(map[string]any)
	if selected, _ := counts["selected_documents"].(float64); selected != 1 {
		t.Fatalf("expected exactly the public note to be selected, got %v: %s", counts["selected_documents"], planResult.stdout)
	}
	if private, _ := counts["private_links"].(float64); private != 1 {
		t.Fatalf("the plan must report the link into the withheld note before anything is written: %v", counts["private_links"])
	}

	// Publishing without a review is refused outright.
	destination := filepath.Join(workspace, "site")
	noReview := runCLI(t, binary, append(append([]string{"publish", "run", "--profile", "public-site",
		"--reviewed-plan", ""}, shared...), destination)...)
	if noReview.exitCode == 0 {
		t.Fatal("publishing with no reviewed plan must fail")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("a refused publication must write nothing: %v", err)
	}

	// A stale review is refused too.
	stale := runCLI(t, binary, append(append([]string{"publish", "run", "--profile", "public-site",
		"--reviewed-plan", strings.Repeat("a", 64)}, shared...), destination)...)
	if stale.exitCode == 0 {
		t.Fatal("publishing against a stale review must fail")
	}
	if !strings.Contains(stale.stderr, "re-run the plan") {
		t.Fatalf("the refusal must say what to do next: %s", stale.stderr)
	}

	// The reviewed digest publishes.
	ran := runCLI(t, binary, append(append([]string{"publish", "run", "--profile", "public-site",
		"--reviewed-plan", digest}, shared...), destination)...)
	if ran.exitCode != 0 {
		t.Fatalf("publish run: exit %d %s", ran.exitCode, ran.stderr)
	}
	report, ok := decodeCLIJSON(t, ran.stdout)["report"].(map[string]any)
	if !ok {
		t.Fatalf("publish output: %s", ran.stdout)
	}
	if report["full_backup"] != false {
		t.Fatalf("a publication must never claim to be a backup: %v", report["full_backup"])
	}
	if report["selection_manifest_sha256"] != digest {
		t.Fatalf("the archive must bind the reviewed plan: %v", report["selection_manifest_sha256"])
	}
	if verified, _ := report["verified"].(bool); !verified {
		t.Fatalf("the publication should verify: %v", report["verified"])
	}

	if documents, _ := report["selected_documents"].(float64); documents != 1 {
		t.Fatalf("the publication should carry exactly the public note: %v", report["selected_documents"])
	}
	if rewritten, _ := report["rewritten_links"].(float64); rewritten != 1 {
		t.Fatalf("the link into the withheld note should have been rewritten: %v", report["rewritten_links"])
	}
	// The withheld note must not appear anywhere in the published bytes —
	// neither as a record, nor as a link target inside a published body.
	assertPublicationWithholds(t, destination, "doc_private", "Internal only", "Private note")

	// A library change invalidates the earlier review.
	if result := runCLI(t, binary, "link", "--db", dbPath, "--asset-store", assetStore, "doc_public"); result.exitCode != 0 {
		t.Fatalf("link: %s", result.stderr)
	}
	newNote := t.TempDir()
	if err := os.MkdirAll(filepath.Join(newNote, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newNote, "manifest.json"), []byte(`{"format":"notrios-archive","version":1,"query":"","notes":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newNote, "notebooks.json"), []byte(`[{"path":"Public"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newNote, "notes", "doc_late.md"),
		[]byte("---\nid: doc_late\ntitle: Late note\nnotebook: Public\ntags: [published]\n---\n\nAdded after the review.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result := runCLI(t, binary, "import", "archive", "--db", dbPath, "--asset-store", assetStore, newNote); result.exitCode != 0 {
		t.Fatalf("second import: %s", result.stderr)
	}
	afterChange := runCLI(t, binary, append(append([]string{"publish", "run", "--profile", "public-site",
		"--reviewed-plan", digest, "--overwrite"}, shared...), filepath.Join(workspace, "site2"))...)
	if afterChange.exitCode == 0 {
		t.Fatal("a note joining the selection after the review must stop the publication")
	}
}

// assertPublicationWithholds walks every published file and fails if the
// withheld identifiers appear anywhere in the archive's bytes.
func assertPublicationWithholds(t *testing.T, root string, forbidden ...string) {
	t.Helper()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, needle := range forbidden {
			if strings.Contains(string(contents), needle) {
				t.Fatalf("%s leaked %q", path, needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublishProfileRefusesAFullArchive(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	profiles := filepath.Join(workspace, "publish-profiles.json")
	result := runCLI(t, binary, "publish", "profile", "save", "--name", "everything",
		"--target", "full_archive", "--tags", "published", "--profiles", profiles,
		"--db", filepath.Join(workspace, "notes.sqlite"))
	if result.exitCode == 0 {
		t.Fatal("a full archive must not be reachable through a publication profile name")
	}
	if !strings.Contains(result.stderr, "backup, not a publication") {
		t.Fatalf("unexpected refusal: %s", result.stderr)
	}
}

func TestPublishProfileRefusesAnEmptySelection(t *testing.T) {
	binary := buildCLI(t)
	workspace := t.TempDir()
	result := runCLI(t, binary, "publish", "profile", "save", "--name", "everything",
		"--profiles", filepath.Join(workspace, "publish-profiles.json"),
		"--db", filepath.Join(workspace, "notes.sqlite"))
	if result.exitCode == 0 {
		t.Fatal("an empty selection would publish the whole library")
	}
	if !strings.Contains(result.stderr, "must select something") {
		t.Fatalf("unexpected refusal: %s", result.stderr)
	}
}
