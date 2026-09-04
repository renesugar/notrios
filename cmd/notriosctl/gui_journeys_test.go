package main

import (
	"encoding/json"
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
	seedJourneyFixtures(t, cli, replica)
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

// seedJourneyFixtures creates the notes the update, delete, restore and search
// journeys act on.
//
// The Help notebook alone is not enough for them: its notes are read only, so
// an update journey pointed at one would photograph a disabled editor, and a
// delete journey would have nothing it is allowed to delete. These are ordinary
// writable notes with titles a reader can follow through a screenshot, which is
// the whole reason they are named rather than generated.
//
// Nothing here is anybody's real note. A screenshot of this interface is a
// picture of whatever is in it, so what is in it is disposable by construction.
func seedJourneyFixtures(t *testing.T, cli string, replica *syncReplica) {
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
		return created.DocumentID
	}

	// One note per journey that changes something. The journeys run in registry
	// order against one library, so a journey that renames a note can break a
	// later journey's postcondition -- and did: the update journey renamed the
	// note the search journeys assert on, and they only kept passing because
	// :has-text matches substrings. That is luck, not a design, so the note
	// that gets renamed is not the note anything else looks for.
	create("Notes from the eastern hide", "A first draft, waiting to be given a better title.\n")

	reed := create("Reed beds at dusk", "Seen from the eastern hide. Two marsh harriers.\n")
	if tagged := runCLI(t, cli, "tags", "add", "--db", replica.db, "--asset-store", replica.assets,
		"--document", reed, "--tag", "field/dusk"); tagged.exitCode != 0 {
		t.Fatalf("tags add exited %d: %s", tagged.exitCode, tagged.stderr)
	}
	create("Old shopping list", "Oats, tinned tomatoes, a new trowel.\n")

	// A tag of its own for the rename journey, for the same reason the update
	// journey has a note of its own: renaming field/dusk would pull the tag the
	// search journeys query out from under them, and that only passes today
	// because rename happens to run last.
	hide := create("Eastern hide, first visit", "Two teal and a water rail heard.\n")
	if tagged := runCLI(t, cli, "tags", "add", "--db", replica.db, "--asset-store", replica.assets,
		"--document", hide, "--tag", "place/hide"); tagged.exitCode != 0 {
		t.Fatalf("tags add exited %d: %s", tagged.exitCode, tagged.stderr)
	}

	// Already in the Trash, because a restore journey has to start from a note
	// that is in there. Creating and deleting it inside the journey would spend
	// four screenshots re-photographing the delete journey.
	recipe := create("Recipe I still want", "The one with brown butter and sage.\n")
	if deleted := runCLI(t, cli, "notes", "delete", "--db", replica.db, "--asset-store", replica.assets,
		"--document", recipe); deleted.exitCode != 0 {
		t.Fatalf("notes delete exited %d: %s", deleted.exitCode, deleted.stderr)
	}
}
