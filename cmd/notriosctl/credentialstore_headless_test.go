package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What a machine without a keyring is allowed to be told.
//
// These exist because CI found the answer was wrong. `doctor` treated an
// unreachable native store as a required failure regardless of whether the
// profile stored anything in it, so a temporary library with no sync
// credentials at all reported FAILED on every headless machine -- server,
// container, CI runner. It passed on every developer workstation, because a
// desktop session has a Secret Service, which is exactly why four milestones
// went by without anyone seeing it.
//
// The rule now is that an empty store cannot be broken. What must not change is
// the other half: sealed key material plus an unreachable store is a profile
// whose keys cannot be read, that is a required failure, and nothing is ever
// substituted for the store.
const noKeyringEnv = "DBUS_SESSION_BUS_ADDRESS=unix:path=/nonexistent"

func TestHeadlessCredentialStoreReporting(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")

	t.Run("a fresh profile with no keys is healthy without a keyring", func(t *testing.T) {
		sandbox := t.TempDir()
		result := runCLIInEnv(t, sandbox, binary, []string{noKeyringEnv}, "doctor",
			"--db", filepath.Join(sandbox, "notes.sqlite"),
			"--asset-store", filepath.Join(sandbox, "assets"))
		if result.exitCode != 0 {
			t.Fatalf("doctor must not fail a profile that stores no credentials:\n%s\n%s",
				result.stdout, result.stderr)
		}
		if !strings.Contains(result.stdout, "no sync credentials are stored yet") {
			t.Fatalf("doctor must say why the unreachable store is not a failure:\n%s", result.stdout)
		}
		// Reported, not hidden. Someone who intends to sync here has to learn
		// that this machine cannot hold the keys.
		if !strings.Contains(result.stdout, "will need one") {
			t.Fatalf("doctor must still say a keyring is needed to store keys:\n%s", result.stdout)
		}
	})

	t.Run("sealed keys with no keyring is still a required failure", func(t *testing.T) {
		sandbox := t.TempDir()
		keyFile := filepath.Join(sandbox, "sync-keys.json")
		// Sealed material is key material that needs a data key from the store.
		// Fabricating the envelope is what lets this run where no store exists:
		// the point under test is the severity, not the cryptography.
		if err := os.WriteFile(keyFile, []byte(`{"sealed":"not-openable-without-the-store"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		configPath := filepath.Join(sandbox, "notrios.yaml")
		body := "sync:\n  rest:\n    credential_store: native\n    key_file: " + keyFile + "\n"
		if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		result := runCLIInEnv(t, sandbox, binary, []string{noKeyringEnv}, "doctor", "--config", configPath,
			"--db", filepath.Join(sandbox, "notes.sqlite"),
			"--asset-store", filepath.Join(sandbox, "assets"))
		if result.exitCode == 0 {
			t.Fatalf("doctor must fail when sealed keys cannot be read:\n%s", result.stdout)
		}
		if strings.Contains(result.stdout, "no sync credentials are stored yet") {
			t.Fatalf("doctor must not call sealed material an empty store:\n%s", result.stdout)
		}
	})

	t.Run("a dry run reports the plan rather than only the obstacle", func(t *testing.T) {
		// Enrol into the development file first, so there is something to
		// move. That is the situation the journey describes and the only one in
		// which asking to migrate to the keychain means anything.
		sandbox, configPath := writeCredentialConfig(t, "development-file")
		args := []string{"--config", configPath,
			"--db", filepath.Join(sandbox, "notes.sqlite"),
			"--asset-store", filepath.Join(sandbox, "assets"),
			"--keys", filepath.Join(sandbox, "sync-keys.json")}
		if result := runCLIInEnv(t, sandbox, binary, []string{noKeyringEnv},
			append([]string{"sync", "init"}, args...)...); result.exitCode != 0 {
			t.Fatalf("sync init failed: %s", result.stderr)
		}
		result := runCLIInEnv(t, sandbox, binary, []string{noKeyringEnv},
			append([]string{"sync", "migrate-credentials", "--to", "native", "--dry-run"}, args...)...)
		// Non-zero, because this migration would not succeed here. The exit
		// code is the verdict; the output is still the plan.
		if result.exitCode == 0 {
			t.Fatalf("a migration that cannot happen must not report success:\n%s", result.stdout)
		}
		for _, want := range []string{"from:", "to:   native", "dry run: nothing was moved"} {
			if !strings.Contains(result.stdout, want) {
				t.Fatalf("the dry run must still report the plan, missing %q:\n%s", want, result.stdout)
			}
		}
	})
}

// What a diagnostic is allowed to disclose.
//
// `paths` and `config show` replace the home directory with "~" by default and
// both offer --no-redact. `doctor` did not: it printed absolute paths, username
// and all -- and doctor is the command whose output gets pasted into an issue.
// paths.Redact's own comment says resolved paths are printed "in `notriosctl
// doctor`" and redacted there, so this was documented intent nobody had wired
// up. Found in v0.9 I7 while establishing what a support bundle would have to
// redact.
func TestDoctorRedactsTheHomeDirectoryByDefault(t *testing.T) {
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()

	for _, form := range []struct {
		name string
		args []string
	}{
		{"report", []string{"doctor"}},
		{"json", []string{"doctor", "--json"}},
	} {
		t.Run(form.name, func(t *testing.T) {
			result := runCLIIn(t, sandbox, binary, form.args...)
			if strings.Contains(result.stdout, sandbox) {
				t.Fatalf("doctor disclosed the home directory:\n%s", result.stdout)
			}
			if !strings.Contains(result.stdout, "~/") {
				t.Fatalf("doctor printed no redacted path, so this proves nothing:\n%s", result.stdout)
			}

			// The escape hatch has to work, or somebody debugging a path
			// problem cannot see the path.
			plain := runCLIIn(t, sandbox, binary, append(append([]string{}, form.args...), "--no-redact")...)
			if !strings.Contains(plain.stdout, sandbox) {
				t.Fatalf("--no-redact did not print the real paths:\n%s", plain.stdout)
			}
		})
	}
}
