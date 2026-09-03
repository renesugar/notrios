package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGUIJourneyCapture drives the interface and photographs each step.
//
// It is opt-in for the same reason the G18e journeys are: it needs Playwright
// and a real browser, neither of which an ordinary `go test ./...` should
// require. The difference is what it produces -- these runs write the images
// the documentation ships, so this is the command that regenerates
// docs/images/journeys/ rather than merely checking something.
//
// The seeded library is deliberately disposable and deliberately not anybody's
// notes: a screenshot of this interface is a picture of whatever is in it.
func TestGUIJourneyCapture(t *testing.T) {
	if os.Getenv("NOTRIOS_GUI_JOURNEYS") != "1" {
		t.Skip("set NOTRIOS_GUI_JOURNEYS=1 to capture the GUI journey screenshots")
	}
	cli := buildCLI(t)
	daemon := buildDaemon(t)
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(repoRoot, "web", "dist")
	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		t.Fatalf("capturing GUI journeys requires built web assets at %s: %v", webDir, err)
	}

	replica := newSyncReplica(t, cli, "gui-journeys", t.TempDir())
	replica.run(t, "init")
	// The Help notebook is seeded because one journey points at it, and because
	// a library with nothing in it photographs as an empty rectangle.
	if seed := runCLI(t, cli, "seed-help", "--db", replica.db, "--asset-store", replica.assets,
		filepath.Join(repoRoot, "docs")); seed.exitCode != 0 {
		t.Fatalf("seed-help exited %d: %s", seed.exitCode, seed.stderr)
	}
	address := unusedLoopbackAddress(t)
	startDaemon(t, daemon, g18eConfig(t, replica, address, "", true, webDir), address)

	runner := exec.Command("node", filepath.Join(repoRoot, "performance", "v0.8-h14", "gui_journeys.mjs"))
	runner.Dir = repoRoot
	runner.Env = append(os.Environ(), "NOTRIOS_GUI_URL=http://"+address)
	output, err := runner.CombinedOutput()
	if err != nil {
		t.Fatalf("GUI journey capture failed: %v\n%s", err, output)
	}
	t.Logf("GUI journey capture:\n%s", output)
}
