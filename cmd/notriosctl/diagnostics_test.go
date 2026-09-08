package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func plantConfig(t *testing.T, sandbox, body string) {
	t.Helper()
	path := filepath.Join(sandbox, "config", "notrios", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// `notriosctl paths` answers the first question in any support thread that
// starts "it cannot find my notes": which roots did this instance resolve, and
// in which mode. Before H4 there was no way to ask.
func TestPathsReportsEveryRootAndTheMode(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()

	result := runCLIIn(t, sandbox, binary, "paths")
	if result.exitCode != 0 {
		t.Fatalf("paths exited %d: %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "mode: installed") {
		t.Errorf("the resolved mode was not reported:\n%s", result.stdout)
	}
	for _, root := range []string{"config", "data", "state", "cache", "runtime", "program_assets"} {
		if !strings.Contains(result.stdout, root) {
			t.Errorf("root %q missing:\n%s", root, result.stdout)
		}
	}
}

// A checkout reports itself as one. This is the diagnostic that makes instance
// isolation visible: a developer who wonders which library they are about to
// write to can see "mode: source" and checkout-local roots.
func TestPathsReportsSourceModeInACheckout(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	for name, body := range map[string]string{
		"go.mod": "module github.com/renesugar/notrios\n", "PLAN.md": "#\n", "AGENTS.md": "#\n",
	} {
		if err := os.WriteFile(filepath.Join(sandbox, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result := runCLIIn(t, sandbox, binary, "paths")
	if result.exitCode != 0 {
		t.Fatalf("paths exited %d: %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "mode: source") {
		t.Fatalf("a checkout did not report source mode:\n%s", result.stdout)
	}
	if !strings.Contains(result.stdout, "data/config") {
		t.Errorf("the checkout-local config root was not reported:\n%s", result.stdout)
	}
}

// Redacted by default: these get pasted into issue reports, and a home
// directory carries a username.
func TestPathsRedactsTheHomeDirectoryUnlessAsked(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()

	redacted := runCLIIn(t, sandbox, binary, "paths")
	if strings.Contains(redacted.stdout, sandbox) {
		t.Errorf("the home directory survived redaction:\n%s", redacted.stdout)
	}
	if !strings.Contains(redacted.stdout, "~/") {
		t.Errorf("nothing was redacted, so the test proves nothing:\n%s", redacted.stdout)
	}

	plain := runCLIIn(t, sandbox, binary, "paths", "--no-redact")
	if !strings.Contains(plain.stdout, sandbox) {
		t.Errorf("--no-redact still redacted:\n%s", plain.stdout)
	}
}

func TestPathsJSONIsMachineReadable(t *testing.T) {
	binary := buildCLI(t)
	result := runCLI(t, binary, "paths", "--json")
	if result.exitCode != 0 {
		t.Fatalf("paths --json exited %d: %s", result.exitCode, result.stderr)
	}
	var decoded struct {
		Mode     string            `json:"mode"`
		Roots    map[string]string `json:"roots"`
		Redacted bool              `json:"redacted"`
		Notices  []struct {
			Code string `json:"code"`
		} `json:"notices"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, result.stdout)
	}
	if decoded.Mode == "" || len(decoded.Roots) != 6 {
		t.Fatalf("mode=%q roots=%d", decoded.Mode, len(decoded.Roots))
	}
	if !decoded.Redacted {
		t.Error("redaction should be on by default")
	}
	// Notices are conditional, so their absence here is legitimate: this
	// machine's XDG_RUNTIME_DIR is inherited and valid. What must hold is that
	// any notice present carries a code.
	for _, notice := range decoded.Notices {
		if notice.Code == "" {
			t.Error("a notice was reported without a code")
		}
	}
}

// The notices are the part doctor could never give: which variable was ignored
// and why. This forces one rather than relying on the developer's environment
// happening to produce it.
func TestPathsReportsWhyAVariableWasIgnored(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()

	result := runCLIInEnv(t, sandbox, binary,
		[]string{"XDG_CONFIG_HOME=relative/config", "XDG_RUNTIME_DIR="},
		"paths", "--json")
	if result.exitCode != 0 {
		t.Fatalf("paths exited %d: %s", result.exitCode, result.stderr)
	}
	var decoded struct {
		Roots   map[string]string `json:"roots"`
		Notices []struct {
			Code, Message string
		} `json:"notices"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, result.stdout)
	}
	codes := map[string]string{}
	for _, notice := range decoded.Notices {
		codes[notice.Code] = notice.Message
	}
	for _, want := range []string{"xdg_relative_ignored", "runtime_dir_unset"} {
		if _, ok := codes[want]; !ok {
			t.Errorf("expected notice %q, got %v", want, decoded.Notices)
		}
	}
	if !strings.Contains(codes["xdg_relative_ignored"], "XDG_CONFIG_HOME") {
		t.Errorf("the notice does not name the variable: %q", codes["xdg_relative_ignored"])
	}
	// And the ignored value was not used.
	if strings.Contains(decoded.Roots["config"], "relative/config") {
		t.Errorf("the relative value was used anyway: %q", decoded.Roots["config"])
	}
}

// `config show` reports where each value came from. "The database is /srv/x" is
// much less useful than "/srv/x, and your config file says so" -- or "nothing
// said otherwise, so it was resolved".
func TestConfigShowReportsValuesWithTheirOrigin(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	library := filepath.Join(sandbox, "library")
	plantConfig(t, sandbox, "data:\n  directory: "+library+"\nremote_media:\n  default_action: block\n")

	result := runCLIIn(t, sandbox, binary, "config", "show")
	if result.exitCode != 0 {
		t.Fatalf("config show exited %d: %s", result.exitCode, result.stderr)
	}

	lines := map[string]string{}
	for _, line := range strings.Split(result.stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.Contains(fields[0], ".") {
			lines[fields[0]] = fields[len(fields)-1]
		}
	}
	if got := lines["data.directory"]; got != "file" {
		t.Errorf("data.directory origin = %q, want file", got)
	}
	if got := lines["remote_media.default_action"]; got != "file" {
		t.Errorf("remote_media.default_action origin = %q, want file", got)
	}
	// Placed under the stated directory, not stated themselves.
	if got := lines["data.state_dir"]; got != "resolved" {
		t.Errorf("data.state_dir origin = %q, want resolved", got)
	}
	if got := lines["server.listen_addr"]; got != "compiled" {
		t.Errorf("server.listen_addr origin = %q, want compiled", got)
	}
}

func TestConfigShowJSONCarriesOrigins(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	plantConfig(t, sandbox, "server:\n  listen_addr: 127.0.0.1:18999\n")

	result := runCLIIn(t, sandbox, binary, "config", "show", "--json")
	if result.exitCode != 0 {
		t.Fatalf("config show --json exited %d: %s", result.exitCode, result.stderr)
	}
	var decoded struct {
		Source   string `json:"source"`
		Settings []struct {
			Key, Value, Origin string
		} `json:"settings"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, result.stdout)
	}
	if len(decoded.Settings) == 0 {
		t.Fatal("no settings reported")
	}
	found := false
	for _, setting := range decoded.Settings {
		if setting.Key == "server.listen_addr" {
			found = true
			if setting.Value != "127.0.0.1:18999" || setting.Origin != "file" {
				t.Errorf("listen_addr = %q from %q", setting.Value, setting.Origin)
			}
		}
	}
	if !found {
		t.Error("server.listen_addr was not reported")
	}
}

// The command is a diagnostic, and a diagnostic that leaks is worse than none.
// sync.credential_ref names a native credential-store entry; it is never a
// credential, which is why it can be shown at all.
func TestConfigShowPrintsNoCredentialMaterial(t *testing.T) {
	binary := buildCLI(t)
	sandbox := t.TempDir()
	plantConfig(t, sandbox, "sync:\n  credential_ref: keyring://notrios/peer\n")

	result := runCLIIn(t, sandbox, binary, "config", "show")
	if result.exitCode != 0 {
		t.Fatalf("config show exited %d: %s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "keyring://notrios/peer") {
		t.Error("the credential reference should be shown: it is a name, not a secret")
	}
	for _, forbidden := range []string{"BEGIN ", "PRIVATE KEY", "password"} {
		if strings.Contains(result.stdout, forbidden) {
			t.Errorf("output contains %q", forbidden)
		}
	}
}

// TestDoctorReportsTheSameRunInBothForms is what a monitoring script needs and
// what doctor could not give it: doctor was the one command a person could read
// and a program could not parse.
func TestDoctorReportsTheSameRunInBothForms(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	roots := []string{
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}

	human := runCLIIn(t, sandbox, binary, append([]string{"doctor"}, roots...)...)
	structured := runCLIIn(t, sandbox, binary, append([]string{"doctor", "--json"}, roots...)...)
	if human.exitCode != structured.exitCode {
		t.Errorf("doctor exited %d and doctor --json exited %d", human.exitCode, structured.exitCode)
	}

	var report struct {
		Checks []struct {
			Check    string `json:"check"`
			State    string `json:"state"`
			Required bool   `json:"required"`
			Detail   string `json:"detail"`
		} `json:"checks"`
		Failed  bool   `json:"failed"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(structured.stdout), &report); err != nil {
		t.Fatalf("doctor --json is not parseable: %v\n%s", err, structured.stdout)
	}
	if len(report.Checks) == 0 {
		t.Fatal("doctor --json reported no checks")
	}

	// Every check in the report is a line in the prose, and every line is a
	// check. A second rendering that quietly says less than the first is worse
	// than not having it.
	for _, check := range report.Checks {
		if !strings.Contains(human.stdout, check.Check) {
			t.Errorf("doctor --json reports %q and the prose form does not mention it", check.Check)
		}
		if !strings.Contains(human.stdout, check.Detail) {
			t.Errorf("detail for %q differs between the two forms", check.Check)
		}
		switch check.State {
		case "ok", "failed", "info":
		default:
			t.Errorf("check %q has state %q, which is none of ok, failed or info", check.Check, check.State)
		}
	}
	proseLines := 0
	for _, line := range strings.Split(strings.TrimSpace(human.stdout), "\n") {
		if strings.HasPrefix(line, "ok  ") || strings.HasPrefix(line, "FAIL") || strings.HasPrefix(line, "info") {
			proseLines++
		}
	}
	if proseLines != len(report.Checks) {
		t.Errorf("the prose form has %d checks and --json has %d", proseLines, len(report.Checks))
	}
}

// TestDoctorFailingStillReportsAsJSON guards the path that matters most to a
// script: doctor exits before its later checks when the config will not load,
// and a caller that asked for JSON must still get JSON rather than nothing.
func TestDoctorFailingStillReportsAsJSON(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	missing := filepath.Join(sandbox, "absent", "config.yaml")

	result := runCLIIn(t, sandbox, binary, "doctor", "--config", missing, "--json")
	if result.exitCode != 1 {
		t.Errorf("doctor --json exited %d on an unreadable config, want 1", result.exitCode)
	}
	var report struct {
		Checks []struct {
			Check string `json:"check"`
			State string `json:"state"`
		} `json:"checks"`
		Failed bool `json:"failed"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &report); err != nil {
		t.Fatalf("a failing doctor --json is not parseable: %v\n%s", err, result.stdout)
	}
	if !report.Failed {
		t.Error("doctor --json reported failed=false while exiting 1")
	}
	found := false
	for _, check := range report.Checks {
		if check.Check == "config" && check.State == "failed" {
			found = true
		}
	}
	if !found {
		t.Errorf("the failing check is not named in the report: %s", result.stdout)
	}
}
