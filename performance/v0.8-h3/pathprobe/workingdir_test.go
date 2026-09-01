package pathprobe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
)

// An installed binary started with no --config reads config/config.example.yaml
// relative to whatever directory it was launched from. That is convenient in a
// checkout and wrong once the binary lives outside one: the configuration that
// decides the database location, the listen address, the public base URL and
// the remote-media policy is taken from the current directory.
//
// The file need not be planted maliciously for this to bite — a user with an
// unrelated config/ directory gets a surprise — but it is the security-relevant
// case that fixes the precedence: an installed binary must not read
// configuration from the current working directory at all.
func TestDefaultConfigurationIsReadFromTheCurrentWorkingDirectory(t *testing.T) {
	compiled := config.Default()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	planted := "server:\n  listen_addr: 127.0.0.1:59999\ndata:\n  database_path: /planted/notes.sqlite\n"
	if err := os.WriteFile(filepath.Join(dir, "config", "config.example.yaml"), []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	loaded, err := config.LoadDefaultOrExample()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Server.ListenAddr == compiled.Server.ListenAddr {
		t.Fatal("H4 may have landed: the working-directory config was no longer read")
	}
	if loaded.Server.ListenAddr != "127.0.0.1:59999" || loaded.Data.DatabasePath != "/planted/notes.sqlite" {
		t.Fatalf("unexpected loaded config: addr=%q db=%q", loaded.Server.ListenAddr, loaded.Data.DatabasePath)
	}
}

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
