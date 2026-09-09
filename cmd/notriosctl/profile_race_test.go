package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Two profiles started in the same instant.
//
// v0.8 H13 found this by accident: a flaky test was made to say *why* it
// failed, and the answer was not slowness but
// `runtime profile "work" failed validation: stale_database: sqlite exec:
// database is locked`. Startup validation opened every registered database
// rather than the one it was starting, so two daemons launching together read
// each other's libraries and one aborted -- reporting a transient lock as a
// stale library, a diagnosis wrong enough to send the reader to the wrong file.
//
// That test now starts its daemons one at a time, which is what a person does.
// This one deliberately does not, because "one at a time" was the workaround
// and nothing was left asserting the thing it worked around.
func TestRuntimeProfilesStartSimultaneously(t *testing.T) {
	binary := buildCLI(t)
	daemon := buildDaemon(t)
	root := t.TempDir()
	registry := filepath.Join(root, "profiles.json")
	names := []string{"first", "second"}
	addresses := []string{unusedLoopbackAddress(t), unusedLoopbackAddress(t)}
	configs := make([]string, 0, len(names))

	for i, name := range names {
		result := runCLI(t, binary, "profile", "create", "--registry", registry,
			"--name", name, "--listen", addresses[i],
			"--db", filepath.Join(root, name+".sqlite"),
			"--asset-store", filepath.Join(root, name+"-assets"))
		if result.exitCode != 0 {
			t.Fatalf("create %s: %s", name, result.stderr)
		}
		profile, ok := decodeCLIJSON(t, result.stdout)["profile"].(map[string]any)
		if !ok {
			t.Fatalf("create output: %s", result.stdout)
		}
		configs = append(configs, profile["config_path"].(string))
	}

	// Started back to back with nothing between them. Any wait here would be
	// the workaround this test exists to remove.
	processes := make([]*exec.Cmd, len(names))
	logs := make([]*strings.Builder, len(names))
	for i := range names {
		cmd := exec.Command(daemon, "-config", configs[i])
		stderr := &strings.Builder{}
		cmd.Stderr = stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		processes[i], logs[i] = cmd, stderr
		proc := cmd
		t.Cleanup(func() {
			if proc.Process != nil {
				_ = proc.Process.Kill()
				_ = proc.Wait()
			}
		})
	}

	for i, name := range names {
		endpoint := "http://" + addresses[i] + "/api/v1/status"
		var response *http.Response
		var err error
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if response, err = http.Get(endpoint); err == nil {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		if err != nil {
			// A daemon that exited and a daemon that is slow look identical
			// from here, and they send a reader to different places.
			died := ""
			if processes[i].ProcessState != nil {
				died = " -- the daemon exited: " + processes[i].ProcessState.String()
			}
			t.Fatalf("wait for %s at %s: %v%s\ndaemon stderr:\n%s",
				name, endpoint, err, died, logs[i].String())
		}
		var status map[string]any
		decodeErr := json.NewDecoder(response.Body).Decode(&status)
		response.Body.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if response.StatusCode != http.StatusOK || status["profile"] != name {
			t.Fatalf("%s status: HTTP %d %+v", name, response.StatusCode, status)
		}
	}

	// Neither may have reported the other's library as its own problem.
	for i, name := range names {
		if text := logs[i].String(); strings.Contains(text, "stale_database") {
			t.Fatalf("%s blamed another profile's database for a transient lock:\n%s", name, text)
		}
	}
}
