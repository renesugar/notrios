package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// plantLegacyLayout writes a pre-0.8 library into the sandbox, where a user who
// ran an old binary from this directory would have left one.
func plantLegacyLayout(t *testing.T, sandbox string) string {
	t.Helper()
	root := filepath.Join(sandbox, "data")
	for rel, body := range map[string]string{
		"notes.sqlite":       "the library",
		"assets/ab/blob.bin": "an attachment",
		"quarantine/q.bin":   "untrusted bytes",
		"search-index/x.dat": "derived",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// The defect this whole slice exists for: `paths` printed an impeccable list of
// empty native roots while the user's library sat in the directory the command
// was run from, and said nothing about it.
func TestPathsNamesAPre08LibraryInTheWorkingDirectory(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	plantLegacyLayout(t, sandbox)

	result := runCLIIn(t, sandbox, binary, "paths")
	if result.exitCode != 0 {
		t.Fatalf("exit %d: %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "pre-0.8 layout:") {
		t.Fatalf("the old library was not mentioned:\n%s", result.stdout)
	}
	if !strings.Contains(result.stdout, "notriosctl migrate --dry-run") {
		t.Fatalf("no way forward was offered:\n%s", result.stdout)
	}
}

func TestPathsSaysNothingWhenThereIsNoOldLibrary(t *testing.T) {
	binary := buildCLI(t)
	result := runCLI(t, binary, "paths")
	if strings.Contains(result.stdout, "pre-0.8 layout") {
		t.Fatalf("an empty sandbox reported a migration:\n%s", result.stdout)
	}
}

func TestPathsJSONCarriesTheLegacyLayout(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	plantLegacyLayout(t, sandbox)

	result := runCLIIn(t, sandbox, binary, "paths", "--json")
	if result.exitCode != 0 {
		t.Fatalf("exit %d: %s", result.exitCode, result.stderr)
	}
	var payload struct {
		LegacyLayout *struct {
			Root     string `json:"root"`
			Database string `json:"database"`
			Occupied bool   `json:"occupied"`
		} `json:"legacy_layout"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("%v: %s", err, result.stdout)
	}
	if payload.LegacyLayout == nil {
		t.Fatalf("no legacy_layout in %s", result.stdout)
	}
	if !strings.HasSuffix(payload.LegacyLayout.Database, "notes.sqlite") {
		t.Fatalf("database %q", payload.LegacyLayout.Database)
	}
}

func TestMigrateDryRunThenMigrateMovesTheLibrary(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	legacy := plantLegacyLayout(t, sandbox)

	dry := runCLIIn(t, sandbox, binary, "migrate", "--dry-run")
	if dry.exitCode != 0 {
		t.Fatalf("exit %d: %s", dry.exitCode, dry.stderr)
	}
	if !strings.Contains(dry.stdout, "nothing was copied") {
		t.Fatalf("a dry run did not say so:\n%s", dry.stdout)
	}
	if _, err := os.Stat(filepath.Join(sandbox, "share", "notrios", "notes.sqlite")); err == nil {
		t.Fatal("the dry run copied the database")
	}

	done := runCLIIn(t, sandbox, binary, "migrate")
	if done.exitCode != 0 {
		t.Fatalf("exit %d: %s", done.exitCode, done.stderr)
	}
	moved := filepath.Join(sandbox, "share", "notrios", "notes.sqlite")
	raw, err := os.ReadFile(moved)
	if err != nil {
		t.Fatalf("the library did not arrive: %v", err)
	}
	if string(raw) != "the library" {
		t.Fatalf("%s holds %q", moved, raw)
	}
	// The quarantine is state, not data.
	if _, err := os.Stat(filepath.Join(sandbox, "state", "notrios", "quarantine", "q.bin")); err != nil {
		t.Fatalf("the quarantine did not reach the state root: %v", err)
	}
	// Derived data is rebuilt, not carried.
	if _, err := os.Stat(filepath.Join(sandbox, "cache", "notrios", "search-index")); err == nil {
		t.Fatal("the search index was copied; it must be rebuilt")
	}
	// The source was renamed, never deleted.
	if _, err := os.Stat(legacy); err == nil {
		t.Fatal("the old root is still in place")
	}
	if !strings.Contains(done.stdout, "renamed to") {
		t.Fatalf("the user was not told where the old copy went:\n%s", done.stdout)
	}
}

func TestMigrateRefusesToMergeAndSaysWhy(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	plantLegacyLayout(t, sandbox)

	existing := filepath.Join(sandbox, "share", "notrios", "notes.sqlite")
	if err := os.MkdirAll(filepath.Dir(existing), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("a different library"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := runCLIIn(t, sandbox, binary, "migrate")
	if result.exitCode == 0 {
		t.Fatalf("a merge was allowed:\n%s", result.stdout)
	}
	if !strings.Contains(result.stderr, "already holds a database") {
		t.Fatalf("the refusal did not say why:\n%s", result.stderr)
	}
	raw, err := os.ReadFile(existing)
	if err != nil || string(raw) != "a different library" {
		t.Fatalf("the existing library was disturbed: %q %v", raw, err)
	}
}

func TestMigrateSaysThereIsNothingToDo(t *testing.T) {
	binary := buildCLI(t)
	result := runCLI(t, binary, "migrate")
	if result.exitCode != 0 {
		t.Fatalf("exit %d: %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "Nothing to migrate") {
		t.Fatalf("unhelpful on an empty sandbox:\n%s", result.stdout)
	}
	if !strings.Contains(result.stdout, "--from") {
		t.Fatalf("a user whose notes are elsewhere was given no next step:\n%s", result.stdout)
	}
}

// The notice must not fire on a library that is in use. A configuration saying
// `directory: ./data` resolves against the working directory, so without this
// check `paths` told a user their current library was stranded and `migrate`
// offered to move it out from under the configuration naming it.
func TestNoMigrationIsOfferedForTheLibraryInUse(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	plantLegacyLayout(t, sandbox)
	plantConfig(t, sandbox, "data:\n  directory: ./data\n")

	paths := runCLIIn(t, sandbox, binary, "paths")
	if strings.Contains(paths.stdout, "pre-0.8 layout") {
		t.Fatalf("the library in use was reported as stranded:\n%s", paths.stdout)
	}

	result := runCLIIn(t, sandbox, binary, "migrate")
	if result.exitCode != 0 {
		t.Fatalf("exit %d: %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "already uses") {
		t.Fatalf("migrate did not explain why there is nothing to do:\n%s", result.stdout)
	}
	// And nothing moved.
	if _, err := os.Stat(filepath.Join(sandbox, "data", "notes.sqlite")); err != nil {
		t.Fatalf("the library in use was disturbed: %v", err)
	}
}

// The destination holding an *empty* library is the ordinary case, not a
// merge: a user discovers their notes are missing by running the new binary,
// and doctor or the service creates an empty library at the resolved path in
// the act of looking. Following doctor's own advice must therefore work.
func TestMigrateSetsAsideAnUnusedDestinationLibrary(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	plantLegacyLayout(t, sandbox)

	// doctor creates the empty library at the resolved path, and says so.
	doctor := runCLIIn(t, sandbox, binary, "doctor")
	if !strings.Contains(doctor.stdout, "pre-0.8 layout") {
		t.Fatalf("doctor did not mention the stranded library:\n%s", doctor.stdout)
	}
	destination := filepath.Join(sandbox, "share", "notrios", "notes.sqlite")
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("doctor did not create a database to be trapped by: %v", err)
	}

	result := runCLIIn(t, sandbox, binary, "migrate")
	if result.exitCode != 0 {
		t.Fatalf("following doctor's advice failed: exit %d: %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "set aside") {
		t.Fatalf("the unused library was not reported as set aside:\n%s", result.stdout)
	}
	raw, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("the real library did not arrive: %v", err)
	}
	if string(raw) != "the library" {
		t.Fatalf("%s holds %q", destination, raw)
	}
	// Set aside, never deleted.
	matches, err := filepath.Glob(destination + ".unused-*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("the displaced empty library was not kept: %v %v", matches, err)
	}
}

// A destination library with real content is a different matter: that is a
// merge, and it is refused with both paths named.
func TestMigrateStillRefusesADestinationHoldingRealNotes(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	plantLegacyLayout(t, sandbox)

	destination := filepath.Join(sandbox, "share", "notrios", "notes.sqlite")
	populateLibrary(t, destination)

	result := runCLIIn(t, sandbox, binary, "migrate")
	if result.exitCode == 0 {
		t.Fatalf("a library with notes in it was overwritten:\n%s", result.stdout)
	}
	if !strings.Contains(result.stderr, "already holds a database") {
		t.Fatalf("the refusal did not say why:\n%s", result.stderr)
	}
	if _, err := os.Stat(filepath.Join(sandbox, "data", "notes.sqlite")); err != nil {
		t.Fatalf("the source was disturbed by a refused migration: %v", err)
	}
	if matches, _ := filepath.Glob(destination + ".unused-*"); len(matches) != 0 {
		t.Fatalf("a populated library was set aside as unused: %v", matches)
	}
}

// populateLibrary creates a real note so the database is not empty.
func populateLibrary(t *testing.T, path string) {
	t.Helper()
	st, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
		Title: "a real note", Body: "kept", Message: "seed",
	}); err != nil {
		t.Fatal(err)
	}
}
