package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/credentials"
	"github.com/renesugar/notrios/internal/synckeys"
)

// TestNoCommandPrintsSecretMaterial walks the commands that touch key material
// and checks that none of them puts any of it on stdout or stderr.
//
// This is a scan rather than a review of each output because the risk is not
// that someone deliberately prints a key: it is that a future change adds a
// field, or widens a report, and nobody notices. A test that looks for the
// bytes will notice.
func TestNoCommandPrintsSecretMaterial(t *testing.T) {
	if _, err := credentials.Select(credentials.KindNative); err != nil {
		t.Skipf("no native credential store here: %v", err)
	}
	binary := sharedBinary(t, "notriosctl")
	sandbox := t.TempDir()
	keyFile := filepath.Join(sandbox, "sync-keys.json")
	args := []string{
		"--keys", keyFile,
		"--db", filepath.Join(sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(sandbox, "assets"),
	}

	initResult := runCLIIn(t, sandbox, binary, append([]string{"sync", "init"}, args...)...)
	if initResult.exitCode != 0 {
		t.Fatalf("sync init failed: %s", initResult.stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(initResult.stdout), &decoded); err != nil {
		t.Fatalf("%v in %q", err, initResult.stdout)
	}
	databaseID, _ := decoded["database_id"].(string)
	provider, err := credentials.Select(credentials.KindNative)
	if err != nil {
		t.Fatal(err)
	}
	ref := credentials.SyncReference(databaseID, keyFile)
	t.Cleanup(func() { _ = provider.Delete(ref) })

	dataKey, err := provider.Get(ref)
	if err != nil {
		t.Fatalf("no data key was stored: %v", err)
	}
	keys, err := synckeys.OpenSealed(keyFile, dataKey)
	if err != nil {
		t.Fatal(err)
	}
	group, err := keys.Current()
	if err != nil {
		t.Fatal(err)
	}

	// Every form each secret could plausibly be printed in. Raw bytes are
	// included because a byte slice formatted with %s or %q would carry them.
	secrets := map[string][]byte{
		"data key":            dataKey,
		"signing private key": keys.PrivateSigningKey(),
		"group key":           group.Key[:],
	}

	scan := func(label, text string) {
		t.Helper()
		for name, secret := range secrets {
			for form, encoded := range map[string]string{
				"raw":       string(secret),
				"base64":    base64.StdEncoding.EncodeToString(secret),
				"base64url": base64.RawURLEncoding.EncodeToString(secret),
			} {
				if encoded == "" {
					continue
				}
				if strings.Contains(text, encoded) {
					t.Errorf("%s printed the %s in %s form", label, name, form)
				}
			}
		}
	}

	scan("sync init stdout", initResult.stdout)
	scan("sync init stderr", initResult.stderr)

	for _, command := range [][]string{
		{"sync", "status"},
		{"sync", "peers"},
		{"doctor"},
		{"paths", "--no-redact"},
		{"config", "show"},
	} {
		result := runCLIIn(t, sandbox, binary, append(command, args...)...)
		label := strings.Join(command, " ")
		scan(label+" stdout", result.stdout)
		scan(label+" stderr", result.stderr)
	}

	// The sealed file on disk is the other surface a reader reaches for.
	onDisk, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	scan("the sealed key file", string(onDisk))
}
