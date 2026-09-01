package pathprobe

// Retired by H4 slice B:
//
//   - TestDefaultConfigurationIsReadFromTheCurrentWorkingDirectory
//
// config.LoadDefaultOrExample became config.LoadDefault, which reads the
// checkout's example only in source mode. The inverted assertions live in
// internal/config as TestLoadDefaultIgnoresAWorkingDirectoryConfigWhenInstalled
// and TestLoadDefaultReadsTheCheckoutExampleInSourceMode.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/httpapi"
)

// The same precedence applies to the interface itself. WebRootCandidates puts
// the working directory ahead of the executable's own directory, so a
// directory named web/dist beside wherever the process was started supplies
// the HTML, CSS and JavaScript loaded into the application window ahead of the
// assets shipped with the binary.
//
// H4 must invert this for an installed binary: explicit flag, then the
// installed data directory, then executable-relative, with the working
// directory considered only in source mode.
func TestWebRootPrefersTheWorkingDirectoryOverTheExecutable(t *testing.T) {
	candidates := httpapi.WebRootCandidates("")
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if candidates[0] != filepath.Clean("web/dist") {
		t.Fatalf("H4 may have landed: first candidate is %q, not the working-directory relative web/dist", candidates[0])
	}
	for _, candidate := range candidates[1:] {
		if !filepath.IsAbs(candidate) {
			t.Fatalf("expected later candidates to be executable-relative and absolute, got %q", candidate)
		}
	}

	// And it resolves: a planted web/dist in the working directory wins.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "web", "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "web", "dist", "index.html"), []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	root, err := httpapi.ResolveWebRoot("")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if root != filepath.Clean("web/dist") {
		t.Fatalf("expected the planted working-directory root, got %q", root)
	}
}

// An explicit --web-dir is the only candidate, which is the behavior H4 keeps.
func TestExplicitWebRootIsTheOnlyCandidate(t *testing.T) {
	candidates := httpapi.WebRootCandidates("/opt/notrios/web")
	if len(candidates) != 1 || candidates[0] != filepath.Clean("/opt/notrios/web") {
		t.Fatalf("explicit web dir must not fall through, got %v", candidates)
	}
}
