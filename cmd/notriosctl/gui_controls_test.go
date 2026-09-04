package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGUIControlInventory measures what the Wails GUI presents.
//
// It is opt-in for the same reason the journey capture is: a browser is
// required. What it produces is the thing the capability table has been
// missing -- every other adapter is countable from source, and the GUI was
// filled in from whichever journeys had been written, which understated it with
// nothing able to notice.
func TestGUIControlInventory(t *testing.T) {
	if os.Getenv("NOTRIOS_GUI_CONTROLS_RUN") != "1" {
		t.Skip("set NOTRIOS_GUI_CONTROLS_RUN=1 to crawl the GUI for its controls")
	}
	cli := buildCLI(t)
	daemon := buildDaemon(t)
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(repoRoot, "web", "dist")
	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		t.Fatalf("crawling the GUI requires built web assets at %s: %v", webDir, err)
	}

	replica := newSyncReplica(t, cli, "gui-controls", t.TempDir())
	replica.run(t, "init")
	// Seeded, because a control that only appears when there is something to
	// act on would otherwise be invisible to the crawl.
	if seed := runCLI(t, cli, "seed-help", "--db", replica.db, "--asset-store", replica.assets,
		filepath.Join(repoRoot, "docs")); seed.exitCode != 0 {
		t.Fatalf("seed-help exited %d: %s", seed.exitCode, seed.stderr)
	}
	address := unusedLoopbackAddress(t)
	startDaemon(t, daemon, g18eConfig(t, replica, address, "", true, webDir), address)

	runner := exec.Command("node", filepath.Join(repoRoot, "performance", "v0.8-h15", "gui_controls.mjs"))
	runner.Dir = repoRoot
	runner.Env = append(os.Environ(), "NOTRIOS_GUI_URL=http://"+address)
	output, err := runner.CombinedOutput()
	if err != nil {
		t.Fatalf("GUI control crawl failed: %v\n%s", err, output)
	}
	t.Logf("GUI control crawl:\n%s", output)
}
