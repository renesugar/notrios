package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/paths"
)

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func makeCheckout(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), "module github.com/renesugar/notrios\n\ngo 1.27\n")
	writeFile(t, filepath.Join(dir, "PLAN.md"), "# plan\n")
	writeFile(t, filepath.Join(dir, "AGENTS.md"), "# agents\n")
}

// Portable mode is selected by the marker file and by nothing else. This is the
// decision H3 recorded and H4 is bound by: inferring it from a writable
// directory makes a USB stick and a home directory the same decision.
func TestPortableModeRequiresTheMarkerAndNothingElse(t *testing.T) {
	dir := t.TempDir()
	if paths.DetectPortable(dir) {
		t.Fatal("a writable directory with no marker must not select portable mode")
	}

	// Writability alone is not the signal, and neither is the directory looking
	// like an install tree.
	if err := os.MkdirAll(filepath.Join(dir, "share", "notrios"), 0o700); err != nil {
		t.Fatal(err)
	}
	if paths.DetectPortable(dir) {
		t.Fatal("an install-shaped directory must not select portable mode")
	}

	writeFile(t, filepath.Join(dir, paths.PortableMarkerName), "")
	if !paths.DetectPortable(dir) {
		t.Fatal("the marker file must select portable mode")
	}

	// A directory of that name is not a marker file.
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, paths.PortableMarkerName), 0o700); err != nil {
		t.Fatal(err)
	}
	if paths.DetectPortable(other) {
		t.Fatal("a directory named like the marker must not select portable mode")
	}
}

// Source detection must not fire on somebody else's Go project, which is why
// it checks the module path rather than the presence of go.mod.
func TestSourceDetectionRequiresThisModule(t *testing.T) {
	mine := t.TempDir()
	makeCheckout(t, mine)
	if !paths.DetectSourceCheckout(mine) {
		t.Fatal("a real checkout was not detected")
	}

	theirs := t.TempDir()
	writeFile(t, filepath.Join(theirs, "go.mod"), "module example.com/other\n")
	writeFile(t, filepath.Join(theirs, "PLAN.md"), "# someone else's plan\n")
	writeFile(t, filepath.Join(theirs, "AGENTS.md"), "# theirs\n")
	if paths.DetectSourceCheckout(theirs) {
		t.Fatal("another project's checkout was mistaken for this one")
	}

	bare := t.TempDir()
	writeFile(t, filepath.Join(bare, "go.mod"), "module github.com/renesugar/notrios\n")
	if paths.DetectSourceCheckout(bare) {
		t.Fatal("go.mod alone must not be enough")
	}
}

func TestSourceCheckoutIsFoundFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	makeCheckout(t, root)
	deep := filepath.Join(root, "internal", "store", "csqlite")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	found, ok := paths.FindSourceCheckout(deep)
	if !ok {
		t.Fatal("running from a subdirectory of a checkout is still source mode")
	}
	// t.TempDir can sit behind a symlink (/tmp -> /private/tmp on macOS), so
	// compare resolved paths rather than the strings.
	wantResolved, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(found)
	if gotResolved != wantResolved {
		t.Fatalf("found %q, want %q", gotResolved, wantResolved)
	}

	outside := t.TempDir()
	if _, ok := paths.FindSourceCheckout(outside); ok {
		t.Fatal("a directory outside any checkout must not be source mode")
	}
}

// Precedence: portable wins over source, and an installed binary standing in a
// checkout is still source mode -- but only because the checkout is detected,
// never because the directory happened to be writable.
func TestModePrecedence(t *testing.T) {
	checkout := t.TempDir()
	makeCheckout(t, checkout)

	if mode, _ := paths.DetectMode(checkout, checkout); mode != paths.ModeSource {
		t.Fatalf("expected source mode, got %s", mode)
	}

	writeFile(t, filepath.Join(checkout, paths.PortableMarkerName), "")
	if mode, _ := paths.DetectMode(checkout, checkout); mode != paths.ModePortable {
		t.Fatalf("the marker must outrank a checkout, got %s", mode)
	}

	elsewhere := t.TempDir()
	if mode, _ := paths.DetectMode(elsewhere, elsewhere); mode != paths.ModeInstalled {
		t.Fatalf("expected installed mode, got %s", mode)
	}
}

func TestRuntimeDirMustBeOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	private := filepath.Join(dir, "private")
	shared := filepath.Join(dir, "shared")
	if err := os.Mkdir(private, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	// os.Mkdir applies the umask, so set the modes explicitly: the question is
	// what the directory *is*, not what this machine's umask allowed.
	if err := os.Chmod(private, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}

	if !paths.RuntimeDirIsPrivate(private) {
		t.Error("an owner-only directory must be accepted")
	}
	if paths.RuntimeDirIsPrivate(shared) {
		t.Error("a world-readable runtime directory must be refused: backup staging holds decrypted library contents")
	}
	if paths.RuntimeDirIsPrivate(filepath.Join(dir, "missing")) {
		t.Error("a missing directory must be refused")
	}
	file := filepath.Join(dir, "file")
	writeFile(t, file, "")
	if paths.RuntimeDirIsPrivate(file) {
		t.Error("a file is not a runtime directory")
	}
}
