package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// J11: `notriosctl paths --report` is the record of what an installation
// occupies, and the oracle for whether a purge removed it.

// j11Library fills a sandbox with the shape of a small library and returns the
// path of a registry naming one profile kept outside the roots.
func j11Library(t *testing.T, sandbox string) string {
	t.Helper()
	plantConfig(t, sandbox, "data:\n  directory: "+filepath.Join(sandbox, "share", "notrios")+"\n")
	for path, body := range map[string]string{
		filepath.Join(sandbox, "share/notrios/notes.sqlite"):      "sqlite",
		filepath.Join(sandbox, "share/notrios/assets/ab/cd/blob"): "bytes",
		filepath.Join(sandbox, "state/notrios/quarantine/.keep"):  "",
		filepath.Join(sandbox, "nas/library/notes.sqlite"):        "elsewhere",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	registry := filepath.Join(sandbox, "profiles.json")
	body := `{"version":2,"profiles":[{"name":"nas","database_id":"db_nas","database_path":"` +
		filepath.Join(sandbox, "nas/library/notes.sqlite") + `","registered_at":"2026-09-18T00:00:00Z"}]}`
	if err := os.WriteFile(registry, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestJ11ReportNamesEveryDirectoryFileAndExternalLibrary(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	registry := j11Library(t, sandbox)

	result := runCLIIn(t, sandbox, binary, "paths", "--report", "--registry", registry)
	if result.exitCode != 0 {
		t.Fatalf("exit %d: %s", result.exitCode, result.stderr)
	}
	for _, want := range []string{"structure:", "profiles: 1 registered", "nas", "which a purge keeps"} {
		if !strings.Contains(result.stdout, want) {
			t.Errorf("the report does not mention %q:\n%s", want, result.stdout)
		}
	}
	// Redacted by default, like `paths`, `config show` and `doctor`.
	if strings.Contains(result.stdout, sandbox) {
		t.Errorf("the default report must redact the home directory:\n%s", result.stdout)
	}

	decoded := struct {
		Schema    string `json:"schema"`
		Structure []struct{ Path, Kind, Category string }
		Manifest  []struct {
			Path, Kind, Category, Profile string
			Owned                         bool
		}
		Counts struct {
			Directories, Files, External, Profiles int
			Bytes                                  int64
		}
		Redacted bool `json:"redacted"`
	}{}
	result = runCLIIn(t, sandbox, binary, "paths", "--report", "--json", "--registry", registry)
	if result.exitCode != 0 {
		t.Fatalf("exit %d: %s", result.exitCode, result.stderr)
	}
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, result.stdout)
	}
	if decoded.Schema != "notrios.install-report/1" || !decoded.Redacted {
		t.Errorf("schema=%s redacted=%t", decoded.Schema, decoded.Redacted)
	}
	if decoded.Counts.Files < 3 || decoded.Counts.Directories < 3 || decoded.Counts.External != 1 || decoded.Counts.Profiles != 1 {
		t.Errorf("counts = %+v", decoded.Counts)
	}
	external := 0
	for _, entry := range decoded.Manifest {
		if !entry.Owned {
			external++
			if entry.Profile != "nas" || !strings.Contains(entry.Path, "nas/library") {
				t.Errorf("external entry = %+v", entry)
			}
		}
	}
	if external != 1 {
		t.Errorf("%d external entries in the manifest", external)
	}
	if !strings.Contains(result.stdout, "~") {
		t.Error("JSON output redacts the home directory too")
	}
}

func TestJ11PathsOutputIsLiteralAndMarksOwnership(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	registry := j11Library(t, sandbox)

	result := runCLIIn(t, sandbox, binary, "paths", "--report", "--paths", "--registry", registry)
	if result.exitCode != 0 {
		t.Fatalf("exit %d: %s", result.exitCode, result.stderr)
	}
	owned, external := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(result.stdout), "\n") {
		state, path, found := strings.Cut(line, "\t")
		if !found || !filepath.IsAbs(path) {
			t.Fatalf("each line is a state and an absolute path, got %q", line)
		}
		if strings.Contains(path, "~") {
			t.Fatalf("a checker compares real paths, so they are never redacted: %q", line)
		}
		switch state {
		case "owned":
			owned++
		case "external":
			external++
		default:
			t.Fatalf("unknown state in %q", line)
		}
	}
	if owned < 4 || external != 1 {
		t.Errorf("owned=%d external=%d", owned, external)
	}

	// The flags that would contradict each other are refused rather than
	// silently preferred.
	if result := runCLIIn(t, sandbox, binary, "paths", "--report", "--paths", "--no-redact"); result.exitCode != 2 ||
		!strings.Contains(result.stderr, "already literal") {
		t.Errorf("--paths --no-redact: exit %d %s", result.exitCode, result.stderr)
	}
	if result := runCLIIn(t, sandbox, binary, "paths", "--report", "--paths", "--json"); result.exitCode != 2 ||
		!strings.Contains(result.stderr, "choose one") {
		t.Errorf("--paths --json: exit %d %s", result.exitCode, result.stderr)
	}
	if result := runCLIIn(t, sandbox, binary, "paths", "--paths"); result.exitCode != 2 ||
		!strings.Contains(result.stderr, "--report only") {
		t.Errorf("--paths without --report: exit %d %s", result.exitCode, result.stderr)
	}
	// And `paths` itself still answers its own question.
	if result := runCLIIn(t, sandbox, binary, "paths"); result.exitCode != 0 || !strings.Contains(result.stdout, "mode") {
		t.Errorf("plain paths: exit %d %s", result.exitCode, result.stdout)
	}
}

func TestJ11ReportAndCheckAgreeAcrossAPurge(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	registry := j11Library(t, sandbox)

	manifest := filepath.Join(t.TempDir(), "before-purge.txt") // kept outside the library
	result := runCLIIn(t, sandbox, binary, "paths", "--report", "--paths", "--registry", registry)
	if result.exitCode != 0 {
		t.Fatalf("exit %d: %s", result.exitCode, result.stderr)
	}
	if err := os.WriteFile(manifest, []byte(result.stdout), 0o600); err != nil {
		t.Fatal(err)
	}

	check := func() (int, string) {
		t.Helper()
		outcome := runCLIIn(t, sandbox, "/usr/bin/env", "bash",
			filepath.Join(repositoryRoot(t), "scripts/check_purged.sh"), manifest)
		return outcome.exitCode, outcome.stdout + outcome.stderr
	}

	// Before anything is deleted, the check fails: nothing has been purged.
	if code, output := check(); code != 1 || !strings.Contains(output, "left") {
		t.Fatalf("before the purge: exit %d\n%s", code, output)
	}

	// Delete every owned path the manifest names, deepest first, which is what
	// a purge does. The external library is left alone, as purge leaves it.
	owned := []string{}
	for _, line := range strings.Split(strings.TrimSpace(result.stdout), "\n") {
		if state, path, _ := strings.Cut(line, "\t"); state == "owned" {
			owned = append(owned, path)
		}
	}
	for i := len(owned) - 1; i >= 0; i-- {
		_ = os.RemoveAll(owned[i])
	}
	if code, output := check(); code != 0 || !strings.Contains(output, "purge verified") {
		t.Fatalf("after the purge: exit %d\n%s", code, output)
	}

	// One file left behind is the failure the check exists for.
	left := owned[len(owned)-1]
	if err := os.MkdirAll(filepath.Dir(left), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(left, []byte("left behind"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, output := check(); code != 1 || !strings.Contains(output, left) {
		t.Fatalf("with one file left: exit %d\n%s", code, output)
	}
}
