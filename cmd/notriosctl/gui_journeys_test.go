package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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
	config := g18eConfig(t, replica, address, "", true, webDir)
	// The remote-media journey photographs a policy decision, so the domain its
	// fixture points at has to be one the policy allows: a blocked or reviewed
	// decision renders the list without the button the journey presses towards.
	// Nothing is fetched by the scan and nothing is fetched by the capture.
	allowRemoteMediaDomain(t, config, "images.example.org")
	startDaemon(t, daemon, config, address)
	// Jobs exist only once something has made one, and every job control is
	// gated on state: Retry renders on a job that failed or was cancelled and
	// nowhere else. This is the crawl's seeding, reused rather than reinvented,
	// and it leaves the carrier replaced by a file on purpose -- which is why
	// the directory is put back below, before any journey asks for a sync.
	seedJobRecords(t, cli, replica, "http://"+address)
	restoreSyncCarrier(t, replica)

	runner := exec.Command("node", filepath.Join(repoRoot, "performance", "v0.8-h14", "gui_journeys.mjs"))
	runner.Dir = repoRoot
	runner.Env = append(os.Environ(), "NOTRIOS_GUI_URL=http://"+address,
		// The folder a sync journey types into the transport field. It is this
		// run's disposable carrier: a catalogue that carried one machine's
		// temporary directory would be documenting the harness.
		"NOTRIOS_GUI_CARRIER="+filepath.Join(filepath.Dir(replica.db), "sync-carrier"))
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
		// A second between notes, which buys the only thing that makes a
		// screenshot diff readable.
		//
		// Notes are listed by `updated_at DESC, id DESC`, `CURRENT_TIMESTAMP`
		// has one-second resolution, and document identifiers are random. Seven
		// notes written inside one second therefore list in an order that
		// changes between runs, and capturing a single new journey rewrote
		// thirty-four existing screenshots that showed nothing new -- the same
		// notes in a different order, one of them scrolled out of frame. An
		// image diff that cannot mean anything is worse than no image diff,
		// because the manifest exists to make a changed picture a signal.
		//
		// Sleeping is the honest fix here rather than sorting by identifier:
		// the order these notes appear in is the product's, and the harness has
		// no business changing it to make its pictures stable. What it can do
		// is stop writing them all in the same second.
		time.Sleep(1100 * time.Millisecond)
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

	// A note pointing at an image on somebody else's server, so the policy scan
	// has something to decide about. The domain is one the config allows; the
	// scan is server-side and downloads nothing, and neither does the capture.
	create("Heron drawings", "![heron](https://images.example.org/heron.png)\n")

	// A note with a live query in it. The block's `query:` line is the same
	// language the search box takes, and `field/dusk` is a tag one of the notes
	// above carries, so the block renders a result rather than an empty state --
	// which is the difference between photographing the feature and
	// photographing its absence.
	create("What I still owe the survey",
		"Everything tagged for dusk work:\n\n```note-query\nquery: tag:field/dusk\nfields: notebook, updated\n```\n")

	// Two notes and a link between them, for the graph. A graph journey against
	// a note that links to nothing photographs "Nothing links to or from this
	// note", which is true and is not the feature.
	pool := create("Marsh survey, north pool", "Four teal, one snipe.\n")
	create("Marsh survey, index",
		"Where each count is written up.\n\n[North pool](document://default/documents/"+pool+")\n")

	// A note of its own to attach a file to, because attaching changes the note
	// and every other journey here has a note nothing else touches for exactly
	// that reason.
	create("Nest box plans", "The drawings live with this note.\n")

	// An import that names a collection, which is the only way a note comes to
	// carry one: nothing creates a collection as an act of its own. The
	// inspector shows the row only when the collection is not `default`, so a
	// note written here would photograph the feature's absence.
	source := t.TempDir()
	for name, body := range map[string]string{
		"folder.md": "Ringing records\n\nid: ringing-folder\ntype_: 2\n",
		"note.md": "Ringing record, 12 May\n\nTwo blackcaps, one chiffchaff.\n\n" +
			"id: ringing-note\nparent_id: ringing-folder\ntype_: 1\n",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if imported := runCLI(t, cli, "import", "joplin-raw", "--db", replica.db,
		"--asset-store", replica.assets, "--collection", "ringing-archive", source); imported.exitCode != 0 {
		t.Fatalf("import joplin-raw exited %d: %s", imported.exitCode, imported.stderr)
	}
}

// restoreSyncCarrier puts back the carrier directory the job seeding replaces.
//
// seedJobRecords parks a sync job by turning the carrier into a file, which is
// what a person meets when a removable drive is not mounted. That is the right
// way to seed a job in a known state and the wrong state to leave behind: the
// sync journeys that follow ask the interface to save a transport, discover on
// the carrier and start a sync, and every one of them would photograph an error
// about a directory that is a file.
func restoreSyncCarrier(t *testing.T, replica *syncReplica) {
	t.Helper()
	carrier := filepath.Join(filepath.Dir(replica.db), "sync-carrier")
	if err := os.RemoveAll(carrier); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(carrier, 0o755); err != nil {
		t.Fatal(err)
	}
}
