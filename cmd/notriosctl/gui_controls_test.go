package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	seedCrawlFixtures(t, cli, replica)
	address := unusedLoopbackAddress(t)
	config := g18eConfig(t, replica, address, "", true, webDir)
	allowRemoteMediaDomain(t, config, "images.example.org")
	startDaemon(t, daemon, config, address)

	runner := exec.Command("node", filepath.Join(repoRoot, "performance", "v0.8-h15", "gui_controls.mjs"))
	runner.Dir = repoRoot
	runner.Env = append(os.Environ(), "NOTRIOS_GUI_URL=http://"+address)
	output, err := runner.CombinedOutput()
	if err != nil {
		t.Fatalf("GUI control crawl failed: %v\n%s", err, output)
	}
	t.Logf("GUI control crawl:\n%s", output)
}

// seedCrawlFixtures creates the notes the crawl needs in order to reach the
// states where most controls live.
//
// The six-view crawl this replaces reported that the GUI had no delete, no
// restore, no purge and no remote-media controls. It has had all of them
// throughout. They are not on other pages: they only exist once a note of the
// right kind is open, and the crawl never opened one, so the interface was
// measured in the one state that shows the fewest controls and the result was
// recorded as a capability gap.
//
// Seeding is therefore part of the measurement rather than setup for it. Each
// note here corresponds to exactly one state in gui_controls.mjs and is matched
// by title, because clicking whichever card happens to be first would have kept
// landing on a read-only Help note.
func seedCrawlFixtures(t *testing.T, cli string, replica *syncReplica) {
	t.Helper()
	create := func(title, body string) string {
		result := runCLI(t, cli, "notes", "create", "--db", replica.db, "--asset-store", replica.assets,
			"--title", title, "--body", body)
		if result.exitCode != 0 {
			t.Fatalf("notes create %q exited %d: %s", title, result.exitCode, result.stderr)
		}
		var created struct {
			DocumentID string `json:"document_id"`
		}
		if err := json.Unmarshal([]byte(result.stdout), &created); err != nil {
			t.Fatalf("notes create %q printed unreadable JSON: %v\n%s", title, err, result.stdout)
		}
		if created.DocumentID == "" {
			t.Fatalf("notes create %q returned no document id: %s", title, result.stdout)
		}
		return created.DocumentID
	}

	create("Crawl fixture note", "A writable note, so that the controls acting on an open note exist.")

	// The remote-media list and its localize button are drawn from a
	// server-side policy scan. Nothing is fetched: not by the scan, not by the
	// crawl, which enumerates and never clicks. The domain is one the config
	// allows, because a reviewed or blocked decision renders the list without
	// the button and the button is the control being looked for.
	create("Remote media fixture", "![diagram](https://images.example.org/diagram.png)\n")

	trashed := create("Trashed fixture note", "Deleted so that restore and purge have somewhere to appear.")
	if result := runCLI(t, cli, "notes", "delete", "--db", replica.db, "--asset-store", replica.assets,
		"--document", trashed); result.exitCode != 0 {
		t.Fatalf("notes delete exited %d: %s", result.exitCode, result.stderr)
	}
}

// allowRemoteMediaDomain appends a remote-media policy to a generated config.
//
// It appends rather than rewrites because the config is produced by a shared
// helper that several tests depend on, and because a policy that silently
// replaced an existing one would be the kind of test-only difference that makes
// a measurement describe the harness instead of the product.
func allowRemoteMediaDomain(t *testing.T, configPath, domain string) {
	t.Helper()
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "remote_media:") {
		t.Fatalf("%s already carries a remote_media policy; appending one would create a duplicate key", configPath)
	}
	policy := fmt.Sprintf("\nremote_media:\n  allowed_domains:\n    - %q\n", domain)
	if err := os.WriteFile(configPath, append(contents, policy...), 0o600); err != nil {
		t.Fatal(err)
	}
}
