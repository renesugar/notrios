package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
)

// buildWebRoot writes a minimal built interface at dir.
func buildWebRoot(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>Notrios</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// chdir moves the process into dir for one test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

// The working directory wins, because a developer running from a checkout means
// the checkout they are standing in.
func TestResolveWebRootPrefersTheWorkingDirectory(t *testing.T) {
	workspace := t.TempDir()
	buildWebRoot(t, filepath.Join(workspace, "web", "dist"))
	chdir(t, workspace)

	root, err := ResolveWebRoot("")
	if err != nil {
		t.Fatalf("ResolveWebRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); err != nil {
		t.Fatalf("resolved root has no index.html: %v", err)
	}
}

// An explicit path is the *only* candidate. An answer that is wrong should fail
// loudly rather than silently fall through to a directory that happens to work.
func TestResolveWebRootExplicitPathDoesNotFallThrough(t *testing.T) {
	workspace := t.TempDir()
	buildWebRoot(t, filepath.Join(workspace, "web", "dist"))
	chdir(t, workspace)

	elsewhere := buildWebRoot(t, filepath.Join(t.TempDir(), "assets"))
	root, err := ResolveWebRoot(elsewhere)
	if err != nil || root != filepath.Clean(elsewhere) {
		t.Fatalf("explicit path should win: %q %v", root, err)
	}

	if candidates := WebRootCandidates("/nowhere"); len(candidates) != 1 {
		t.Fatalf("an explicit path must be the only candidate, got %v", candidates)
	}
	_, err = ResolveWebRoot("/nowhere/at/all")
	var notFound *WebRootNotFoundError
	if !errors.As(err, &notFound) || !notFound.Explicit {
		t.Fatalf("expected an explicit-path failure, got %v", err)
	}
	// The advice has to differ: the path is wrong, not the build.
	if !strings.Contains(err.Error(), "--web-dir") || strings.Contains(err.Error(), "make web") {
		t.Fatalf("explicit failures must not advise rebuilding: %v", err)
	}
}

// The failure names every directory tried, deduplicated. Before v0.5 E11 the
// message named none of them and told the reader to rebuild assets that may
// already exist somewhere else.
func TestResolveWebRootFailureNamesEveryDirectoryOnce(t *testing.T) {
	chdir(t, t.TempDir())

	_, err := ResolveWebRoot("")
	var notFound *WebRootNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected WebRootNotFoundError, got %v", err)
	}
	if len(notFound.Searched) == 0 {
		t.Fatal("the failure must name where it looked")
	}
	seen := map[string]bool{}
	for _, dir := range notFound.Searched {
		if seen[dir] {
			t.Fatalf("directory listed twice: %v", notFound.Searched)
		}
		seen[dir] = true
		if !filepath.IsAbs(dir) {
			t.Fatalf("searched directories must be absolute to be actionable: %q", dir)
		}
	}
	if !strings.Contains(err.Error(), "Looked in:") {
		t.Fatalf("the message must list the directories: %v", err)
	}
}

// Serving still works, and a miss reports where it looked rather than claiming
// the interface was never built.
func TestWebAppServesFromResolvedRootAndReportsAMiss(t *testing.T) {
	workspace := t.TempDir()
	root := buildWebRoot(t, filepath.Join(workspace, "built"))

	cfg := config.Default()
	cfg.Server.WebDir = root
	server := NewServerWithOptions(ServerOptions{Config: cfg})
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Notrios") {
		t.Fatalf("expected the built index: %d %s", rr.Code, rr.Body.String())
	}

	chdir(t, t.TempDir())
	missing := config.Default()
	missing.Server.WebDir = filepath.Join(t.TempDir(), "absent")
	server = NewServerWithOptions(ServerOptions{Config: missing})
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "web_ui_not_found") {
		t.Fatalf("expected the not-found code: %s", body)
	}
	if !strings.Contains(body, "Looked in") {
		t.Fatalf("the response must say where it looked: %s", body)
	}
}
