package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/credentials"
	"github.com/renesugar/notrios/internal/synckeys"
)

// writeCredentialConfig plants a config naming a credential store, and returns
// the sandbox to run in.
func writeCredentialConfig(t *testing.T, store string) (sandbox, configPath string) {
	t.Helper()
	sandbox = t.TempDir()
	configPath = filepath.Join(sandbox, "notrios.yaml")
	body := "sync:\n  rest:\n    credential_store: " + store + "\n"
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return sandbox, configPath
}

// TestSyncInitRefusesAnUnreachableStore is the enrolment half of this item's
// boundary. A profile naming a store this machine cannot provide must be told
// so before anything is created, not when a sync first runs.
func TestSyncInitRefusesAnUnreachableStore(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox, configPath := writeCredentialConfig(t, "no-such-store")
	result := runCLIIn(t, sandbox, binary,
		"sync", "init", "--config", configPath,
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"))
	if result.exitCode == 0 {
		t.Fatalf("enrolling against an unknown store must fail\nstdout: %s", result.stdout)
	}
	if !strings.Contains(result.stderr, "no-such-store") {
		t.Fatalf("the refusal must name the store the user configured; got %q", result.stderr)
	}
	if !strings.Contains(result.stderr, "cannot enrol") {
		t.Fatalf("the refusal must say enrolment is what failed; got %q", result.stderr)
	}
	// Nothing may be created by a refused enrolment.
	if entries, err := filepath.Glob(filepath.Join(sandbox, "config", "notrios", "sync-keys*.json")); err == nil && len(entries) > 0 {
		t.Fatalf("a refused enrolment left key material behind: %v", entries)
	}
}

// TestSyncInitReportsTheDevelopmentStore pins the default: unchanged
// behaviour, and the warning still printed.
func TestSyncInitReportsTheDevelopmentStore(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	result := runCLIIn(t, sandbox, binary,
		"sync", "init",
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"))
	if result.exitCode != 0 {
		t.Fatalf("sync init failed: %s", result.stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("%v in %q", err, result.stdout)
	}
	if got := decoded["credential_store"]; got != "locked-file-development" {
		t.Fatalf("credential_store = %v, want locked-file-development", got)
	}
	if !strings.Contains(result.stderr, "not an OS keychain") {
		t.Fatalf("the development warning must still be printed; got %q", result.stderr)
	}
}

// TestCLIHonoursTheConfiguredKeyFile is the divergence this slice found. The
// CLI resolved key material from its own flag or the default path and ignored
// sync.rest.key_file, which the service honours -- so a configured profile had
// the command and the service reading different files while both reported
// success.
func TestCLIHonoursTheConfiguredKeyFile(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	configured := filepath.Join(sandbox, "elsewhere", "sync-keys.json")
	if err := os.MkdirAll(filepath.Dir(configured), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(sandbox, "notrios.yaml")
	body := "sync:\n  rest:\n    key_file: " + configured + "\n"
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	result := runCLIIn(t, sandbox, binary,
		"sync", "init", "--config", configPath,
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"))
	if result.exitCode != 0 {
		t.Fatalf("sync init failed: %s", result.stderr)
	}
	if _, err := os.Stat(configured); err != nil {
		t.Fatalf("the CLI ignored sync.rest.key_file and wrote elsewhere: %v", err)
	}
}

// TestDoctorReportsTheCredentialStore covers the other half: a user who has
// not enrolled yet can still find out what this installation would use.
func TestDoctorReportsTheCredentialStore(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")

	t.Run("development file by default", func(t *testing.T) {
		sandbox := t.TempDir()
		result := runCLIIn(t, sandbox, binary, "doctor",
			"--db", filepath.Join(sandbox, "notes.sqlite"),
			"--asset-store", filepath.Join(sandbox, "assets"))
		if !strings.Contains(result.stdout, "credential store") {
			t.Fatalf("doctor did not report a credential store:\n%s", result.stdout)
		}
		if !strings.Contains(result.stdout, "not by a keychain") {
			t.Fatalf("doctor must say the development file is not a keychain:\n%s", result.stdout)
		}
	})

	t.Run("unknown store fails", func(t *testing.T) {
		sandbox, configPath := writeCredentialConfig(t, "no-such-store")
		result := runCLIIn(t, sandbox, binary, "doctor", "--config", configPath,
			"--db", filepath.Join(sandbox, "notes.sqlite"),
			"--asset-store", filepath.Join(sandbox, "assets"))
		if result.exitCode == 0 {
			t.Fatalf("doctor must fail on an unknown credential store:\n%s", result.stdout)
		}
		if !strings.Contains(result.stdout, "no-such-store") {
			t.Fatalf("doctor must name the unknown store:\n%s", result.stdout)
		}
	})
}

// TestSyncInitUsesTheNativeStoreEndToEnd drives the compiled CLI against this
// machine's real credential store. It is the only test here that proves the
// data key genuinely leaves the process and comes back: everything else could
// pass with an in-memory stand-in.
func TestSyncInitUsesTheNativeStoreEndToEnd(t *testing.T) {
	if _, err := credentials.Select(credentials.KindNative); err != nil {
		t.Skipf("no native credential store here: %v", err)
	}
	binary := sharedBinary(t, "notriosctl")
	sandbox, configPath := writeCredentialConfig(t, "native")
	keyFile := filepath.Join(sandbox, "sync-keys.json")
	result := runCLIIn(t, sandbox, binary,
		"sync", "init", "--config", configPath, "--keys", keyFile,
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"))
	if result.exitCode != 0 {
		t.Fatalf("sync init against the native store failed: %s", result.stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("%v in %q", err, result.stdout)
	}
	databaseID, _ := decoded["database_id"].(string)
	if databaseID == "" {
		t.Fatalf("no database_id in %q", result.stdout)
	}
	ref := credentials.Reference{Service: "notrios-sync", Account: databaseID}
	provider, err := credentials.Select(credentials.KindNative)
	if err != nil {
		t.Fatal(err)
	}
	// Registered before any assertion so a failure below still cleans up the
	// developer's keyring.
	t.Cleanup(func() { _ = provider.Delete(ref) })

	if got := decoded["credential_store"]; got == "locked-file-development" {
		t.Fatalf("credential_store = %v; the native store was configured", got)
	}
	if warning := strings.TrimSpace(result.stderr); strings.Contains(warning, "not an OS keychain") {
		t.Fatalf("the development warning must not appear for a keychain-backed profile: %q", warning)
	}
	sealed, err := synckeys.IsSealed(keyFile)
	if err != nil || !sealed {
		t.Fatalf("the CLI wrote an unsealed key file: sealed=%v err=%v", sealed, err)
	}
	dataKey, err := provider.Get(ref)
	if err != nil {
		t.Fatalf("the CLI stored no data key in %s: %v", provider.Name(), err)
	}
	if len(dataKey) != synckeys.SealKeyBytes {
		t.Fatalf("stored data key is %d bytes, want %d", len(dataKey), synckeys.SealKeyBytes)
	}
	if _, err := synckeys.OpenSealed(keyFile, dataKey); err != nil {
		t.Fatalf("the stored data key does not open the file the CLI wrote: %v", err)
	}
}
