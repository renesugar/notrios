package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestG18eBrowserJourneys is the opt-in, real-browser companion to the
// documentation journey registry. It is deliberately absent from ordinary Go
// test runs because it needs Playwright and a locally installed browser.
func TestG18eBrowserJourneys(t *testing.T) {
	if os.Getenv("NOTRIOS_G18E_BROWSER") != "1" {
		t.Skip("set NOTRIOS_G18E_BROWSER=1 to run the G18e Playwright journeys")
	}

	cli := buildCLI(t)
	daemon := buildDaemon(t)
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Join(repoRoot, "web", "dist")
	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		t.Fatalf("G18e requires built web assets at %s: %v", webDir, err)
	}

	type pair struct {
		label, hostURL, joinerURL string
	}
	pairs := make([]pair, 0, 2)
	for _, label := range []string{"desktop", "narrow"} {
		host := newSyncReplica(t, cli, label+"-host", t.TempDir())
		joiner := newSyncReplica(t, cli, label+"-joiner", t.TempDir())
		host.run(t, "init")
		seed := runCLI(t, cli, "seed-help", "--db", host.db, "--asset-store", host.assets, filepath.Join(repoRoot, "docs"))
		if seed.exitCode != 0 {
			t.Fatalf("%s: seed-help exited %d\nstdout: %s\nstderr: %s", label, seed.exitCode, seed.stdout, seed.stderr)
		}
		hostAddress, joinerAddress := unusedLoopbackAddress(t), unusedLoopbackAddress(t)
		hostURL, joinerURL := "http://"+hostAddress, "http://"+joinerAddress
		startDaemon(t, daemon, g18eConfig(t, host, hostAddress, "", true, webDir), hostAddress)
		uiJSON(t, http.MethodPost, hostURL+"/api/v1/documents",
			fmt.Sprintf(`{"title":%q,"body":%q}`, "Shared fixture "+label, "G18e "+label+" fixture"), http.StatusCreated)
		adoptSnapshot(t, host, joiner)
		joiner.run(t, "init")
		startDaemon(t, daemon, g18eConfig(t, joiner, joinerAddress, hostURL, false, webDir), joinerAddress)
		pairs = append(pairs, pair{label: label, hostURL: hostURL, joinerURL: joinerURL})
	}

	reportPath := os.Getenv("NOTRIOS_G18E_REPORT")
	if reportPath == "" {
		reportPath = filepath.Join(t.TempDir(), "g18e-browser-report.json")
	}
	runner := exec.Command("node", filepath.Join(repoRoot, "performance", "v0.7-g18e", "browser_journeys.mjs"))
	runner.Dir = repoRoot
	runner.Env = append(os.Environ(),
		"G18E_DESKTOP_HOST_URL="+pairs[0].hostURL,
		"G18E_DESKTOP_JOINER_URL="+pairs[0].joinerURL,
		"G18E_NARROW_HOST_URL="+pairs[1].hostURL,
		"G18E_NARROW_JOINER_URL="+pairs[1].joinerURL,
		"G18E_REPORT_PATH="+reportPath,
	)
	output, err := runner.CombinedOutput()
	if err != nil {
		t.Fatalf("G18e browser runner failed: %v\n%s", err, output)
	}
	t.Logf("G18e browser report: %s\n%s", reportPath, output)
}

// g18eConfig reuses the shared process configuration and adds the absolute
// web directory required when the daemon is launched by a test subprocess.
func g18eConfig(t *testing.T, replica *syncReplica, address, peerURL string, hostCarrier bool, webDir string) string {
	t.Helper()
	path := writeSyncUIProcessConfig(t, replica, address, peerURL, hostCarrier)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(contents), "server:\n", fmt.Sprintf("server:\n  web_dir: %q\n", webDir), 1)
	if updated == string(contents) {
		t.Fatal("sync UI config has no server section")
	}
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
