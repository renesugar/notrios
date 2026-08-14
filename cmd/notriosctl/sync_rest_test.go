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

// writeSyncConfig writes the generated config a daemon runs from, with the peer
// surface enabled on loopback.
func writeSyncConfig(t *testing.T, replica *syncReplica, address string) string {
	t.Helper()
	root := filepath.Dir(replica.db)
	path := filepath.Join(root, "config.yaml")
	contents := fmt.Sprintf(`server:
  listen_addr: %q
  public_base_url: %q

data:
  directory: %q
  database_path: %q
  asset_store: %q

sync:
  target: "rest"
  rest:
    enabled: true
    require_tls: true
    key_file: %q
`, address, "http://"+address, root, replica.db, replica.assets, replica.keys)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func startDaemon(t *testing.T, daemon, configPath, address string) {
	t.Helper()
	command := exec.Command(daemon, "-config", configPath)
	var stderr strings.Builder
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.Process != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if response, err := http.Get("http://" + address + "/api/v1/status"); err == nil {
			response.Body.Close()
			return
		}
		if command.ProcessState != nil && command.ProcessState.Exited() {
			t.Fatalf("notriosd exited during startup: %s", stderr.String())
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("notriosd never became ready at %s: %s", address, stderr.String())
}

// TestAPairedPeerAuthenticatesToARunningService is G13's working state, end to
// end and across processes: a code is displayed, spent once over the network,
// and the replica that spent it can then authenticate to the peer's sync
// surface — while everyone else cannot.
func TestAPairedPeerAuthenticatesToARunningService(t *testing.T) {
	cli := buildCLI(t)
	daemon := buildDaemon(t)
	carrier := t.TempDir()
	host := newSyncReplica(t, cli, "host", carrier)
	joiner := newSyncReplica(t, cli, "joiner", carrier)

	host.run(t, "init")
	host.note(t, "hosted", "# Hosted\n\nbody\n")
	adoptSnapshot(t, host, joiner)
	joiner.run(t, "init")

	address := unusedLoopbackAddress(t)
	startDaemon(t, daemon, writeSyncConfig(t, host, address), address)
	baseURL := "http://" + address

	// Before pairing, the joiner is a stranger with a key nobody enrolled.
	before := runCLI(t, cli, "sync", "handshake", "--db", joiner.db,
		"--asset-store", joiner.assets, "--keys", joiner.keys, "--url", baseURL)
	if before.exitCode == 0 {
		t.Fatalf("an unenrolled replica authenticated: %s", before.stdout)
	}

	invite := host.runJSON(t, "invite", "--ttl", "5m", "--label", "rest test")
	code, _ := invite["code"].(string)
	if code == "" {
		t.Fatalf("invite produced no code: %v", invite)
	}
	joined := runCLI(t, cli, "sync", "join", "--db", joiner.db, "--asset-store", joiner.assets,
		"--keys", joiner.keys, "--url", baseURL, "--code", code)
	if joined.exitCode != 0 {
		t.Fatalf("pairing over REST failed: %s\n%s", joined.stderr, joined.stdout)
	}

	// The whole point: the same replica now authenticates and the peer answers
	// with what it is compatible with.
	after := runCLI(t, cli, "sync", "handshake", "--db", joiner.db,
		"--asset-store", joiner.assets, "--keys", joiner.keys, "--url", baseURL)
	if after.exitCode != 0 {
		t.Fatalf("a paired replica was refused: %s", after.stderr)
	}
	var handshake map[string]any
	if err := json.Unmarshal([]byte(after.stdout), &handshake); err != nil {
		t.Fatalf("%v in %q", err, after.stdout)
	}
	if handshake["database_id"] == nil || handshake["state_vector"] == nil {
		t.Fatalf("handshake is missing its contents: %v", handshake)
	}

	// The code is spent. Running the same join again must fail, from any
	// replica, because single use is a transaction rather than a convention.
	replay := runCLI(t, cli, "sync", "join", "--db", joiner.db, "--asset-store", joiner.assets,
		"--keys", joiner.keys, "--url", baseURL, "--code", code)
	if replay.exitCode == 0 {
		t.Fatal("a pairing code worked twice")
	}

	// An anonymous request to the same surface is refused, and refused without
	// explaining itself.
	anonymous, err := http.Get(baseURL + "/api/v1/sync/handshake")
	if err != nil {
		t.Fatal(err)
	}
	defer anonymous.Body.Close()
	if anonymous.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous request = %d, want 401", anonymous.StatusCode)
	}
	if anonymous.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("the sync surface emitted a CORS header")
	}

	// Revoking the key ends it, without touching anything else about the peer.
	peers := host.runJSON(t, "peers")
	rows, _ := peers["peers"].([]any)
	if len(rows) != 1 {
		t.Fatalf("host peers = %v", peers)
	}
	row, _ := rows[0].(map[string]any)
	keyID, _ := row["signer_key_id"].(string)
	if keyID == "" || row["status"] != "active" {
		t.Fatalf("peer row: %v", row)
	}
	host.run(t, "revoke", "--key", keyID, "--reason", "test revocation")
	revoked := runCLI(t, cli, "sync", "handshake", "--db", joiner.db,
		"--asset-store", joiner.assets, "--keys", joiner.keys, "--url", baseURL)
	if revoked.exitCode == 0 {
		t.Fatal("a revoked key still authenticated")
	}
}

// TestTheServiceRefusesToExposeTheSyncSurfaceWithoutTLS is the startup half of
// the transport policy: a misconfiguration is a service that does not start and
// says why, rather than one that starts and is exposed.
func TestTheServiceRefusesToExposeTheSyncSurfaceWithoutTLS(t *testing.T) {
	cli := buildCLI(t)
	daemon := buildDaemon(t)
	replica := newSyncReplica(t, cli, "exposed", t.TempDir())
	replica.run(t, "init")

	root := filepath.Dir(replica.db)
	configPath := filepath.Join(root, "exposed.yaml")
	contents := fmt.Sprintf(`server:
  listen_addr: "0.0.0.0:0"

data:
  directory: %q
  database_path: %q
  asset_store: %q

sync:
  target: "rest"
  rest:
    enabled: true
    require_tls: true
    key_file: %q
`, root, replica.db, replica.assets, replica.keys)
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(daemon, "-config", configPath).CombinedOutput()
	if err == nil {
		t.Fatal("the service started with the sync surface exposed without TLS")
	}
	if !strings.Contains(string(output), "sync.rest") || !strings.Contains(string(output), "TLS") {
		t.Fatalf("the refusal does not say what to change: %s", output)
	}
}
