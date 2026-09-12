package purge_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/purge"
)

func TestExternalPathsKeepsOnlyWhatIsOutsideTheOwnedRoots(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "share", "notrios")
	outside := filepath.Join(root, "elsewhere", "library.sqlite")
	for _, path := range []string{data, filepath.Dir(outside)} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	inside := filepath.Join(data, "notes.sqlite")
	for _, path := range []string{inside, outside} {
		if err := os.WriteFile(path, []byte("a library"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	refs := []purge.ExternalRef{
		{Path: inside, Profile: "default", Field: "database"},
		{Path: outside, Profile: "recipes", Field: "database"},
		// Relative paths are dropped rather than resolved: which directory they
		// would resolve against depends on where the process was started.
		{Path: "relative/library.sqlite", Profile: "broken", Field: "database"},
		{Path: "", Profile: "empty", Field: "database"},
	}
	external := purge.ExternalPaths(refs, []string{data})

	if len(external) != 1 {
		t.Fatalf("want only the outside path, got %+v", external)
	}
	if external[0].Profile != "recipes" {
		t.Errorf("attribution lost: %+v", external[0])
	}
}

// A path named twice -- a profile whose database and asset store sit in one
// directory, or two profiles sharing a library -- is listed once. A user reading
// "this is not deleted" twice about one directory learns nothing the second time.
func TestExternalPathsDeduplicates(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "shared")
	if err := os.MkdirAll(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	external := purge.ExternalPaths([]purge.ExternalRef{
		{Path: shared, Profile: "one", Field: "database"},
		{Path: shared, Profile: "two", Field: "asset store"},
	}, []string{filepath.Join(root, "owned")})
	if len(external) != 1 {
		t.Fatalf("want one entry for one directory, got %+v", external)
	}
}

// The step exists so the user sees the path. It must not be something any other
// part of this package acts on -- that is the whole safety property.
func TestExternalStepsAreInertAndMeasured(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library")
	if err := os.MkdirAll(library, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "notes.sqlite"), []byte("twelve chars"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "sync-keys.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	steps := purge.ExternalSteps([]purge.ExternalRef{
		{Path: library, Profile: "recipes", Field: "database"},
	}, nil)
	if len(steps) != 1 {
		t.Fatalf("want one step, got %d", len(steps))
	}
	step := steps[0]
	if step.Action != "enumerate" {
		t.Errorf("action %q would be acted on; want enumerate", step.Action)
	}
	if step.Category != "external" || step.Policy != purge.BackupPolicy("external") {
		t.Errorf("category/policy not the external pair: %+v", step)
	}
	if step.Files != 2 || step.Bytes != 14 {
		t.Errorf("want the tree measured (2 files, 14 bytes), got %d files %d bytes",
			step.Files, step.Bytes)
	}
	if !strings.Contains(step.Reason, "recipes") || !strings.Contains(step.Reason, "database") {
		t.Errorf("the reason does not say which profile named it: %q", step.Reason)
	}

	// Inert, asserted rather than assumed: the two functions that touch the
	// filesystem are handed this step and must do nothing with it.
	destination := filepath.Join(root, "backup")
	manifest, err := purge.CreateBackup(steps, destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Entries) != 0 {
		t.Errorf("an enumerate step reached the backup: %+v", manifest.Entries)
	}
	removed, err := purge.Remove(steps)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("an enumerate step was deleted: %v", removed)
	}
	if _, err := os.Stat(filepath.Join(library, "notes.sqlite")); err != nil {
		t.Fatalf("the external library was removed: %v", err)
	}
}

// The oracle's rule works, and `notriosctl purge` deliberately does not use it.
//
// Handing the enumerated paths to the oracle looked right and was wrong. Every
// enumerated path is outside every owned root -- that is what made it external
// -- so the rule can never fire to protect a root. The only case it fires on is
// the reverse: a profile naming an *ancestor* of the roots, which
// `profile register --asset-store ~/.local/share` produces by accident. The
// whole data root was then refused and the user's notes survived a confirmed
// purge, described as "enumerated and backed up" when nothing was copied.
//
// Both halves are asserted here so that re-wiring it fails a test rather than
// shipping again.
func TestTheExternalRuleFiresOnlyOnTheAncestorCaseThatWouldBlockAPurge(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "share", "notrios")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}

	// The ancestor case: what a misconfigured asset store produces, and the
	// refusal that made purge keep the library.
	ancestor := filepath.Join(root, "share")
	steps := purge.Plan(map[string]string{"data": data}, purge.Environment{
		Home:                 root,
		ExternalProfilePaths: []string{ancestor},
	})
	if steps[0].Action != "refuse" {
		t.Fatalf("the ancestor case should still refuse when the oracle is told: %+v", steps[0])
	}

	// And the command's own call, without that list: the root is deleted, which
	// is what the user asked for.
	steps = purge.Plan(map[string]string{"data": data}, purge.Environment{Home: root})
	if steps[0].Action != "backup_then_delete" {
		t.Fatalf("purge must not be blocked by a profile naming a parent directory: %+v", steps[0])
	}
}

// The ancestor case is reported rather than acted on, and the report says what
// happens to the roots inside it -- "not deleted" is true of the directory and
// badly misleading about its contents.
func TestAnExternalAncestorSaysTheRootsInsideItAreStillDeleted(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "share", "notrios")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	ancestor := filepath.Join(root, "share")

	steps := purge.ExternalSteps([]purge.ExternalRef{
		{Path: ancestor, Profile: "wide", Field: "asset store"},
	}, []string{data})
	if len(steps) != 1 {
		t.Fatalf("want one step, got %+v", steps)
	}
	if !strings.Contains(steps[0].Reason, "still") || !strings.Contains(steps[0].Reason, "root(s)") {
		t.Fatalf("the report does not say the roots inside it are still deleted: %q", steps[0].Reason)
	}

	// A path with no root inside it does not carry that sentence.
	elsewhere := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	plain := purge.ExternalSteps([]purge.ExternalRef{
		{Path: elsewhere, Profile: "recipes", Field: "database"},
	}, []string{data})
	if strings.Contains(plain[0].Reason, "still") {
		t.Errorf("an ordinary external path should not claim to contain roots: %q", plain[0].Reason)
	}
}
