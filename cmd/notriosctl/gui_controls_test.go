package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	seedJobRecords(t, cli, replica, "http://"+address)

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

// seedJobRecords makes sure there is work for the interface to show.
//
// The jobs row in the capability table was recorded as "no GUI journey" and
// then, when the control crawl could not find a job control either, as
// unmeasured rather than absent -- because job rows render only once a job
// exists, and nothing had ever created one. This creates both kinds so the
// crawl can answer the question properly.
//
// The two are not the same question. The sync centre lists the four sync job
// kinds and only those (internal/httpapi.syncUIJobs), so a sync job should
// appear. An import job is a job by the same definition -- it has a record, a
// state and a kind, and `notriosctl jobs list` shows it -- and if it does not
// appear anywhere in the interface, that is a real gap rather than a seeding
// accident. Seeding both is what makes the difference between those two
// answers visible.
func seedJobRecords(t *testing.T, cli string, replica *syncReplica, baseURL string) {
	t.Helper()

	// An import, which writes a job record of kind import_joplin_raw.
	source := t.TempDir()
	for name, body := range map[string]string{
		"folder.md": "Imported Notes\n\nid: jobs-folder\ntype_: 2\n",
		"note.md":   "Job fixture note\n\nA note that exists so a job record does.\n\nid: jobs-note\nparent_id: jobs-folder\ntype_: 1\n",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if imported := runCLI(t, cli, "import", "joplin-raw", "--db", replica.db,
		"--asset-store", replica.assets, source); imported.exitCode != 0 {
		t.Fatalf("import joplin-raw exited %d: %s", imported.exitCode, imported.stderr)
	}

	// And sync jobs, queued through the surface the interface itself uses
	// rather than written into the store, so what the crawl sees is what
	// pressing "Sync now" would produce.
	queueAndWaitSync(t, baseURL, "incremental")

	// Two more, because every job control in this interface is gated on state.
	// Cancel renders only while a job is running or queued, Retry only on one
	// that failed or was cancelled, and a job that succeeded is a row of text
	// with no control on it at all. Seeding only the happy path measured an
	// interface with no job controls, which would have been read as their
	// absence.
	//
	// Replacing the carrier with a file is what a person meets when a removable
	// drive is not mounted, and the worker treats that as offline rather than
	// as a failure: the job stays queued with a retry pending and never
	// settles. That is the product behaving sensibly, and it is also why the
	// first attempt at seeding a failed job waited a minute for one that was
	// never coming. It does give a job parked in a known state, which makes
	// cancelling it exact rather than a race against a job that might finish
	// first.
	carrier := filepath.Join(filepath.Dir(replica.db), "sync-carrier")
	if err := os.RemoveAll(carrier); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(carrier, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Something for them to carry: a sync with nothing to send settles without
	// ever reaching the carrier.
	if note := runCLI(t, cli, "notes", "create", "--db", replica.db, "--asset-store", replica.assets,
		"--title", "Written after the drive went away",
		"--body", "So the next synchronization has something to carry.\n"); note.exitCode != 0 {
		t.Fatalf("notes create exited %d: %s", note.exitCode, note.stderr)
	}

	// One left waiting, so the interface has a job to offer Cancel on.
	waiting := queueSyncJob(t, baseURL)
	waitForSyncJobState(t, baseURL, waiting, "queued", "running")

	// And one cancelled, so it has a job to offer Retry on.
	cancelled := queueSyncJob(t, baseURL)
	uiRequest(t, http.MethodPost, baseURL+"/api/v1/jobs/"+cancelled+"/cancel", nil, "", http.StatusOK)
	waitForSyncJobState(t, baseURL, cancelled, "cancelled")
}

// queueSyncJob asks for a synchronization the way the interface does.
func queueSyncJob(t *testing.T, baseURL string) string {
	t.Helper()
	started := uiJSON(t, http.MethodPost, baseURL+"/api/v1/jobs/sync/start",
		`{"kind":"incremental"}`, http.StatusAccepted)
	return stringField(t, mapField(t, started["job"]), "id")
}

// waitForSyncJobState waits for a job to reach any of the given states.
//
// queueAndWaitSync fails on anything but success, which is right for its other
// callers and wrong here: what is being arranged is a job that does not
// succeed. The failure message names the state the job was actually in,
// because "never settled" on its own sent the previous round of this into
// guesswork -- the answer, once the job said it, was "sync retry pending:
// offline", which no amount of reasoning about carriers had produced.
func waitForSyncJobState(t *testing.T, baseURL, id string, want ...string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		job := uiRequest(t, http.MethodGet, baseURL+"/api/v1/jobs/"+id, nil, "", http.StatusOK)
		for _, state := range want {
			if job["state"] == state {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	final := uiRequest(t, http.MethodGet, baseURL+"/api/v1/jobs/"+id, nil, "", http.StatusOK)
	t.Fatalf("the sync job never reached %v; it was last seen as %v", want, final)
}
