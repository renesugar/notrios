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

// credentialFixture is an enrolled library on the development file, ready to
// be migrated.
type credentialFixture struct {
	binary     string
	sandbox    string
	configPath string
	keyFile    string
	databaseID string
	ref        credentials.Reference
}

func newCredentialFixture(t *testing.T) *credentialFixture {
	t.Helper()
	if _, err := credentials.Select(credentials.KindNative); err != nil {
		t.Skipf("no native credential store here: %v", err)
	}
	f := &credentialFixture{binary: sharedBinary(t, "notriosctl"), sandbox: t.TempDir()}
	f.keyFile = filepath.Join(f.sandbox, "sync-keys.json")
	f.configPath = filepath.Join(f.sandbox, "notrios.yaml")
	if err := os.WriteFile(f.configPath, []byte("sync:\n  rest:\n    key_file: "+f.keyFile+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := f.run(t, "init")
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("%v in %q", err, result.stdout)
	}
	f.databaseID, _ = decoded["database_id"].(string)
	if f.databaseID == "" {
		t.Fatalf("no database_id in %q", result.stdout)
	}
	f.ref = credentials.Reference{Service: "notrios-sync", Account: f.databaseID}
	provider, err := credentials.Select(credentials.KindNative)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Delete(f.ref) })
	return f
}

func (f *credentialFixture) run(t *testing.T, args ...string) cliResult {
	t.Helper()
	full := append([]string{"sync", args[0], "--config", f.configPath,
		"--db", filepath.Join(f.sandbox, "notes.sqlite"),
		"--asset-store", filepath.Join(f.sandbox, "assets")}, args[1:]...)
	return runCLIIn(t, f.sandbox, f.binary, full...)
}

func (f *credentialFixture) mustRun(t *testing.T, args ...string) cliResult {
	t.Helper()
	result := f.run(t, args...)
	if result.exitCode != 0 {
		t.Fatalf("notriosctl sync %s exited %d\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), result.exitCode, result.stdout, result.stderr)
	}
	return result
}

// TestMigrateCredentialsRoundTrip is the whole slice: the same keys, moved to
// the keychain and back, still being the same keys. Minting new material would
// pass a naive "did it migrate" check and make every artifact a peer has
// already published unreadable.
func TestMigrateCredentialsRoundTrip(t *testing.T) {
	f := newCredentialFixture(t)
	before, err := synckeys.Open(f.keyFile)
	if err != nil {
		t.Fatal(err)
	}
	wantSigner := before.SignerKeyID()
	wantGroup, err := before.Current()
	if err != nil {
		t.Fatal(err)
	}

	f.mustRun(t, "migrate-credentials", "--to", "native", "--confirm")

	if sealed, err := synckeys.IsSealed(f.keyFile); err != nil || !sealed {
		t.Fatalf("after migrating to the keychain the file is not sealed: %v %v", sealed, err)
	}
	provider, err := credentials.Select(credentials.KindNative)
	if err != nil {
		t.Fatal(err)
	}
	dataKey, err := provider.Get(f.ref)
	if err != nil {
		t.Fatalf("no data key was stored: %v", err)
	}
	sealedKeys, err := synckeys.OpenSealed(f.keyFile, dataKey)
	if err != nil {
		t.Fatalf("the stored data key does not open the migrated file: %v", err)
	}
	if sealedKeys.SignerKeyID() != wantSigner {
		t.Fatalf("migration minted a new signing key")
	}
	if got, _ := sealedKeys.Current(); got.Key != wantGroup.Key || got.Epoch != wantGroup.Epoch {
		t.Fatalf("migration minted a new group key")
	}

	// And back again, which is the rollback this item's validation asks for.
	if err := os.WriteFile(f.configPath,
		[]byte("sync:\n  rest:\n    key_file: "+f.keyFile+"\n    credential_store: native\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.mustRun(t, "migrate-credentials", "--to", "development-file", "--confirm")

	if sealed, err := synckeys.IsSealed(f.keyFile); err != nil || sealed {
		t.Fatalf("after rolling back the file is still sealed: %v %v", sealed, err)
	}
	after, err := synckeys.Open(f.keyFile)
	if err != nil {
		t.Fatalf("the rolled-back file does not open: %v", err)
	}
	if after.SignerKeyID() != wantSigner {
		t.Fatalf("rollback changed the signing key")
	}
	if _, err := provider.Get(f.ref); err == nil {
		t.Fatalf("rollback left the data key in the credential store")
	}
}

// TestMigrateCredentialsRequiresConfirmation keeps the move deliberate, and
// keeps --dry-run genuinely read-only.
func TestMigrateCredentialsRequiresConfirmation(t *testing.T) {
	f := newCredentialFixture(t)

	dry := f.mustRun(t, "migrate-credentials", "--to", "native", "--dry-run")
	if !strings.Contains(dry.stdout, "dry run: nothing was moved") {
		t.Fatalf("dry run did not say so:\n%s", dry.stdout)
	}
	if sealed, _ := synckeys.IsSealed(f.keyFile); sealed {
		t.Fatalf("a dry run sealed the key file")
	}
	provider, _ := credentials.Select(credentials.KindNative)
	if _, err := provider.Get(f.ref); err == nil {
		t.Fatalf("a dry run stored a data key")
	}

	unconfirmed := f.run(t, "migrate-credentials", "--to", "native")
	if unconfirmed.exitCode == 0 {
		t.Fatalf("migrating without --confirm must fail")
	}
	if !strings.Contains(unconfirmed.stderr, "--confirm") {
		t.Fatalf("the refusal must name what is missing; got %q", unconfirmed.stderr)
	}
	if sealed, _ := synckeys.IsSealed(f.keyFile); sealed {
		t.Fatalf("an unconfirmed run sealed the key file")
	}
}

func TestMigrateCredentialsRefusals(t *testing.T) {
	f := newCredentialFixture(t)

	if result := f.run(t, "migrate-credentials", "--to", "development-file", "--confirm"); result.exitCode == 0 {
		t.Fatalf("migrating to the store it already uses must fail")
	} else if !strings.Contains(result.stderr, "already held by") {
		t.Fatalf("the refusal must say why; got %q", result.stderr)
	}

	if result := f.run(t, "migrate-credentials", "--to", "somewhere", "--confirm"); result.exitCode == 0 {
		t.Fatalf("an unknown destination must fail")
	}
	if result := f.run(t, "migrate-credentials", "--confirm"); result.exitCode == 0 {
		t.Fatalf("a missing --to must fail")
	}

	// An occupied slot is refused rather than overwritten: the data key that is
	// already there opens material somewhere, and replacing it would strand it.
	provider, _ := credentials.Select(credentials.KindNative)
	if err := provider.Set(f.ref, make([]byte, synckeys.SealKeyBytes)); err != nil {
		t.Fatal(err)
	}
	result := f.run(t, "migrate-credentials", "--to", "native", "--confirm")
	if result.exitCode == 0 {
		t.Fatalf("migrating onto an occupied data key must fail")
	}
	if !strings.Contains(result.stderr, "already holds a data key") {
		t.Fatalf("the refusal must name the collision; got %q", result.stderr)
	}
	if sealed, _ := synckeys.IsSealed(f.keyFile); sealed {
		t.Fatalf("a refused migration sealed the key file anyway")
	}
}
