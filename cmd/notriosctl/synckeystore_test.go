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

// TestSyncInitStoreDefaults covers what an unconfigured profile does. These
// tests run a binary from a temp directory, which resolves as an installed
// profile rather than a source checkout -- the distinction the default turns
// on.
func TestSyncInitStoreDefaults(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")

	t.Run("a fresh installed profile defaults to the keychain", func(t *testing.T) {
		if _, err := credentials.Select(credentials.KindNative); err != nil {
			t.Skipf("no native credential store here: %v", err)
		}
		sandbox := t.TempDir()
		keyFile := filepath.Join(sandbox, "sync-keys.json")
		result := runCLIIn(t, sandbox, binary, "sync", "init", "--keys", keyFile,
			"--db", filepath.Join(sandbox, "notes.sqlite"),
			"--asset-store", filepath.Join(sandbox, "assets"))
		if result.exitCode != 0 {
			t.Fatalf("sync init failed: %s", result.stderr)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
			t.Fatalf("%v in %q", err, result.stdout)
		}
		databaseID, _ := decoded["database_id"].(string)
		provider, err := credentials.Select(credentials.KindNative)
		if err != nil {
			t.Fatal(err)
		}
		ref := credentials.SyncReference(databaseID, keyFile)
		t.Cleanup(func() { _ = provider.Delete(ref) })

		if got := decoded["credential_store"]; got == "locked-file-development" {
			t.Fatalf("a fresh installed profile still defaulted to the development file")
		}
		if strings.Contains(result.stderr, "not an OS keychain") {
			t.Fatalf("the development warning must not appear for a keychain profile: %q", result.stderr)
		}
		if sealed, err := synckeys.IsSealed(keyFile); err != nil || !sealed {
			t.Fatalf("the default did not seal the key file: %v %v", sealed, err)
		}
	})

	t.Run("an explicit development file is honoured and warned about", func(t *testing.T) {
		sandbox, configPath := writeCredentialConfig(t, "development-file")
		result := runCLIIn(t, sandbox, binary, "sync", "init", "--config", configPath,
			"--keys", filepath.Join(sandbox, "sync-keys.json"),
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
		// An explicit choice is a decision, not a state to nag about.
		if strings.Contains(result.stderr, "migrate-credentials") {
			t.Fatalf("an explicitly chosen store must not be advised against: %q", result.stderr)
		}
	})
}

// TestUpgradedProfileKeepsItsKeysAndIsTold is the case the default turns on. A
// library whose keys predate this milestone must keep reading them -- pointing
// it at a keychain that does not hold them would strand it -- and must be told,
// because continuing silently is the downgrade this milestone forbids.
func TestUpgradedProfileKeepsItsKeysAndIsTold(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox, configPath := writeCredentialConfig(t, "development-file")
	keyFile := filepath.Join(sandbox, "sync-keys.json")
	args := []string{"--config", configPath, "--keys", keyFile,
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets")}
	if result := runCLIIn(t, sandbox, binary, append([]string{"sync", "init"}, args...)...); result.exitCode != 0 {
		t.Fatalf("sync init failed: %s", result.stderr)
	}

	// Drop the setting, as an upgrade to a build with the new default does.
	if err := os.WriteFile(configPath, []byte("sync:\n  rest:\n    key_file: "+keyFile+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	status := runCLIIn(t, sandbox, binary, append([]string{"sync", "status"}, args...)...)
	if status.exitCode != 0 {
		t.Fatalf("an upgraded profile must still read its keys: %s", status.stderr)
	}
	if !strings.Contains(status.stderr, "migrate-credentials") {
		t.Fatalf("the upgraded profile was not told to migrate; stderr: %q", status.stderr)
	}
	if sealed, err := synckeys.IsSealed(keyFile); err != nil || sealed {
		t.Fatalf("an upgrade must not seal existing key material: %v %v", sealed, err)
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
	// The store is pinned because this test is about the path, not the store:
	// left unset, an installed profile defaults to the keychain and this would
	// write a data key into the keyring of whoever runs the suite.
	body := "sync:\n  rest:\n    key_file: " + configured + "\n    credential_store: development-file\n"
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

	t.Run("an explicit development file is named as such", func(t *testing.T) {
		sandbox, configPath := writeCredentialConfig(t, "development-file")
		result := runCLIIn(t, sandbox, binary, "doctor", "--config", configPath,
			"--db", filepath.Join(sandbox, "notes.sqlite"),
			"--asset-store", filepath.Join(sandbox, "assets"))
		if !strings.Contains(result.stdout, "credential store") {
			t.Fatalf("doctor did not report a credential store:\n%s", result.stdout)
		}
		if !strings.Contains(result.stdout, "not by a keychain") {
			t.Fatalf("doctor must say the development file is not a keychain:\n%s", result.stdout)
		}
	})

	t.Run("a fresh installed profile reports the keychain", func(t *testing.T) {
		if _, err := credentials.Select(credentials.KindNative); err != nil {
			t.Skipf("no native credential store here: %v", err)
		}
		sandbox := t.TempDir()
		result := runCLIIn(t, sandbox, binary, "doctor",
			"--db", filepath.Join(sandbox, "notes.sqlite"),
			"--asset-store", filepath.Join(sandbox, "assets"))
		if !strings.Contains(result.stdout, "is reachable") {
			t.Fatalf("doctor must report the default keychain as reachable:\n%s", result.stdout)
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
	ref := credentials.SyncReference(databaseID, keyFile)
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
