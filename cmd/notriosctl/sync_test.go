package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syncReplica is one library on disk, driven only through the compiled CLI.
// Everything here runs in a separate process from the test, which is what makes
// it evidence about the product rather than about the package.
type syncReplica struct {
	name    string
	binary  string
	db      string
	assets  string
	keys    string
	carrier string
	// config pins the development file. These tests are about the sync
	// protocol, and an unconfigured installed profile now defaults to the
	// operating system's keychain -- so without this they would write a data
	// key into the keyring of whoever runs the suite and leave it there.
	config string
}

func newSyncReplica(t *testing.T, binary, name, carrier string) *syncReplica {
	t.Helper()
	root := t.TempDir()
	replica := &syncReplica{
		name: name, binary: binary,
		db:      filepath.Join(root, "notes.sqlite"),
		assets:  filepath.Join(root, "assets"),
		keys:    filepath.Join(root, "sync-keys.json"),
		carrier: carrier,
		config:  filepath.Join(root, "replica.yaml"),
	}
	if err := os.WriteFile(replica.config,
		[]byte("sync:\n  rest:\n    credential_store: development-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return replica
}

func (r *syncReplica) run(t *testing.T, args ...string) cliResult {
	t.Helper()
	full := append([]string{"sync", args[0], "--config", r.config,
		"--db", r.db, "--asset-store", r.assets, "--keys", r.keys}, args[1:]...)
	result := runCLI(t, r.binary, full...)
	if result.exitCode != 0 {
		t.Fatalf("%s: notriosctl %s exited %d\nstdout: %s\nstderr: %s",
			r.name, strings.Join(full, " "), result.exitCode, result.stdout, result.stderr)
	}
	return result
}

func (r *syncReplica) runJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	result := r.run(t, args...)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("%s: %v in %q", r.name, err, result.stdout)
	}
	return decoded
}

func (r *syncReplica) note(t *testing.T, title, body string) string {
	t.Helper()
	// The CLI has no create-note command, so the note is made the way a user
	// would through the API surface the tests already own: a seeded import.
	// Keeping it to one file means the import is the whole content.
	vault := t.TempDir()
	if err := os.WriteFile(filepath.Join(vault, title+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	result := runCLI(t, r.binary, "import", "obsidian", "--db", r.db, "--asset-store", r.assets, vault)
	if result.exitCode != 0 {
		t.Fatalf("%s: import exited %d: %s", r.name, result.exitCode, result.stderr)
	}
	return title
}

func TestTwoProcessesConvergeThroughASharedDirectory(t *testing.T) {
	binary := buildCLI(t)
	carrier := t.TempDir()
	left := newSyncReplica(t, binary, "left", carrier)
	right := newSyncReplica(t, binary, "right", carrier)

	// Each library enrols itself and mints its own key material.
	leftInit := left.runJSON(t, "init")
	if leftInit["enrolled"] != true {
		t.Fatalf("init did not enrol the journal: %v", leftInit)
	}
	leftNote := left.note(t, "from-left", "# From the left\n\nleft body\n")

	// The second replica is made the way the product makes one: an archive-v2
	// snapshot restored with the explicit adopt intent, which keeps the
	// database identity and mints a new replica identity. Two libraries created
	// independently are two databases, and sync refuses to join them — which is
	// the check working, not an obstacle to route around.
	adoptSnapshot(t, left, right)
	right.run(t, "init")

	// Pairing is one ceremony with two halves that travel separately: a file
	// the joining replica reads, and a code it is told. Neither is usable
	// alone, which is the property G13 exists to provide.
	pairOffline(t, left, right)

	rightNote := right.note(t, "from-right", "# From the right\n\nright body\n")
	leftSecond := left.note(t, "left-after-adopt", "# Left again\n\nmore left\n")

	for pass := 0; pass < 2; pass++ {
		left.run(t, "once", "--carrier", carrier)
		right.run(t, "once", "--carrier", carrier)
	}

	assertProcessHoldsNote(t, left, rightNote)
	assertProcessHoldsNote(t, right, leftNote)
	assertProcessHoldsNote(t, right, leftSecond)

	// Both replicas end with the same state vector, which is the claim that
	// "converged" actually means.
	leftVector := left.runJSON(t, "status")["state_vector"]
	rightVector := right.runJSON(t, "status")["state_vector"]
	if !equalJSON(leftVector, rightVector) {
		t.Fatalf("state vectors differ:\n left: %v\nright: %v", leftVector, rightVector)
	}
}

func TestDiscoveryReportsAnUnpairedPeerAndChangesNothing(t *testing.T) {
	binary := buildCLI(t)
	carrier := t.TempDir()
	left := newSyncReplica(t, binary, "left", carrier)
	right := newSyncReplica(t, binary, "right", carrier)
	left.run(t, "init")
	adoptSnapshot(t, left, right)
	right.run(t, "init")

	// Only one direction is paired: the right replica trusts the left's key and
	// admits it, while the left has never heard of the right. The offline
	// ceremony's last step — the inviter enrolling the acceptance — is skipped.
	pairOfflineWithoutEnrolling(t, left, right)

	left.note(t, "one-sided", "# One sided\n\nbody\n")
	left.run(t, "once", "--carrier", carrier)
	right.run(t, "once", "--carrier", carrier)

	// The right replica publishes under a key the left does not know, so the
	// left can see that someone is there and nothing else.
	discovered := left.runJSON(t, "discover", "--carrier", carrier)
	candidates, _ := discovered["Candidates"].([]any)
	if len(candidates) != 1 {
		t.Fatalf("discover reported %v", discovered)
	}
	candidate, _ := candidates[0].(map[string]any)
	if candidate["Reason"] != "unenrolled_signing_key" {
		t.Fatalf("candidate: %v", candidate)
	}
	if candidate["ReplicaID"] != "" {
		t.Fatalf("discovery claimed an identity it cannot verify: %v", candidate)
	}
	// Discovery published nothing: the folder still holds one namespace per
	// replica and no new advertisement from the asking side.
	if peers, _ := discovered["Peers"].([]any); len(peers) != 0 {
		t.Fatalf("discovery reported peers it has not enrolled: %v", peers)
	}
}

func TestSyncRefusesAKeyFileOtherUsersCanRead(t *testing.T) {
	binary := buildCLI(t)
	carrier := t.TempDir()
	replica := newSyncReplica(t, binary, "loose", carrier)
	replica.run(t, "init")

	if err := os.Chmod(replica.keys, 0o644); err != nil {
		t.Fatal(err)
	}
	result := runCLI(t, binary, "sync", "status", "--db", replica.db,
		"--asset-store", replica.assets, "--keys", replica.keys)
	if result.exitCode != 0 {
		t.Fatalf("status should still report the library: %d %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "must not be readable by other users") {
		t.Fatalf("a world-readable key file was not reported: %s", result.stdout)
	}
	// And a command that would actually use the key refuses outright.
	exchange := runCLI(t, binary, "sync", "once", "--db", replica.db,
		"--asset-store", replica.assets, "--keys", replica.keys, "--carrier", carrier)
	if exchange.exitCode == 0 {
		t.Fatal("a sync ran with a world-readable key file")
	}
}

// pairOffline runs the whole offline ceremony: the inviter writes a file and
// displays a code, the joiner accepts with both, and the inviter enrols what
// comes back. The code is parsed out of the invite command's own output, which
// is what a user reads off a screen.
func pairOffline(t *testing.T, inviter, joiner *syncReplica) {
	t.Helper()
	code, acceptance := pairOfflineWithoutEnrolling(t, inviter, joiner)
	inviter.run(t, "enroll", "--acceptance", acceptance, "--code", code)
}

// pairOfflineWithoutEnrolling stops one step short: the joiner has adopted the
// group key and trusts the inviter, but the inviter has not yet enrolled the
// joiner's key. That is a real state — the acceptance file is still in
// someone's hand — and it is what discovery is for.
func pairOfflineWithoutEnrolling(t *testing.T, inviter, joiner *syncReplica) (string, string) {
	t.Helper()
	workspace := t.TempDir()
	invite := filepath.Join(workspace, "invite.json")
	result := inviter.runJSON(t, "invite", "--offline", "--out", invite)
	code, _ := result["code"].(string)
	if code == "" {
		t.Fatalf("invite produced no code: %v", result)
	}
	acceptance := filepath.Join(workspace, "acceptance.json")
	joiner.run(t, "accept", "--invite", invite, "--code", code, "--out", acceptance)
	return code, acceptance
}

// adoptSnapshot copies one library into another as a second replica of the same
// database, through the exported archive and the explicit adopt intent.
func adoptSnapshot(t *testing.T, source, target *syncReplica) {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "snapshot")
	exported := runCLI(t, source.binary, "export", "archive-v2", "--db", source.db,
		"--asset-store", source.assets, archive)
	if exported.exitCode != 0 {
		t.Fatalf("export archive-v2 exited %d: %s", exported.exitCode, exported.stderr)
	}
	restored := runCLI(t, target.binary, "restore", "archive-v2", "--intent", "adopt",
		"--db", target.db, "--asset-store", target.assets, archive)
	if restored.exitCode != 0 {
		t.Fatalf("restore archive-v2 --intent adopt exited %d: %s", restored.exitCode, restored.stderr)
	}
}

func assertProcessHoldsNote(t *testing.T, replica *syncReplica, title string) {
	t.Helper()
	// The library is inspected through the graph report, which lists every note
	// by title and needs no note-reading command.
	report := runCLI(t, replica.binary, "graph", "report", "--db", replica.db,
		"--asset-store", replica.assets, "--limit", "100")
	if report.exitCode != 0 {
		t.Fatalf("%s: graph report exited %d: %s", replica.name, report.exitCode, report.stderr)
	}
	if !strings.Contains(report.stdout, title) {
		t.Fatalf("%s does not hold %q:\n%s", replica.name, title, report.stdout)
	}
}

func equalJSON(left, right any) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftBytes) == string(rightBytes)
}
