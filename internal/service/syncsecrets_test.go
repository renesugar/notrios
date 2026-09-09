package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/credentials"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synckeys"
)

func secretTestStore(t *testing.T) (config.Config, *store.SQLiteStore) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg-config"))
	cfg := config.Default()
	cfg.Data.Directory = dir
	config.UseDataDirectory(&cfg, dir, nil)
	cfg.Data.DatabasePath = filepath.Join(dir, "notes.sqlite")
	st, err := store.OpenSQLite(cfg.Data.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg.Sync.REST.KeyFile = filepath.Join(dir, "sync-keys.json")
	return cfg, st
}

// TestDefaultConfigStillUsesTheDevelopmentFile pins the working state. Slice B
// adds a capability; it must not change what an existing profile does, because
// moving an installed library's key material is a migration and migrations are
// explicit.
func TestDefaultConfigStillUsesTheDevelopmentFile(t *testing.T) {
	cfg, st := secretTestStore(t)
	provider, err := newSyncSecretStore(cfg, st)
	if err != nil {
		t.Fatalf("default config: %v", err)
	}
	if got := provider.ProviderName(); got != "locked-file-development" {
		t.Fatalf("default provider is %q, want the development file", got)
	}
	if provider.Warning() == "" {
		t.Fatalf("the development file must keep its warning")
	}
}

func TestUnknownCredentialStoreIsRefused(t *testing.T) {
	cfg, st := secretTestStore(t)
	cfg.Sync.REST.CredentialStore = "somewhere-else"
	if _, err := newSyncSecretStore(cfg, st); err == nil {
		t.Fatalf("an unknown credential store must be refused")
	}
}

func TestNativeStoreConfigSelectsTheNativeProvider(t *testing.T) {
	cfg, st := secretTestStore(t)
	cfg.Sync.REST.CredentialStore = config.CredentialStoreNative
	provider, err := newSyncSecretStore(cfg, st)
	if err != nil {
		t.Skipf("no native credential store on this machine: %v", err)
	}
	if provider.Warning() != "" {
		t.Errorf("the native store has nothing to warn about, got %q", provider.Warning())
	}
	created, err := provider.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	native := provider.(*nativeSyncSecretStore)
	t.Cleanup(func() { _ = native.provider.Delete(native.ref) })

	sealed, err := synckeys.IsSealed(native.path)
	if err != nil || !sealed {
		t.Fatalf("the key file must be sealed on disk: sealed=%v err=%v", sealed, err)
	}
	if _, err := synckeys.Open(native.path); !errors.Is(err, synckeys.ErrSealRequired) {
		t.Fatalf("the sealed file must refuse a plain open, got %v", err)
	}

	// A second provider over the same store and path must reach the same keys,
	// which is what proves the data key really came back out of the keychain.
	fresh := &nativeSyncSecretStore{path: native.path, provider: native.provider, ref: native.ref}
	reopened, err := fresh.Open()
	if err != nil {
		t.Fatalf("reopening through the credential store: %v", err)
	}
	if reopened.SignerKeyID() != created.SignerKeyID() {
		t.Fatalf("reopened key material is not the created material")
	}
}

// failingSetProvider stores nothing and reports why.
type failingSetProvider struct{ *credentials.MemoryProvider }

func (failingSetProvider) Set(credentials.Reference, []byte) error {
	return errors.New("the keychain refused the write")
}

// TestCreateLeavesNoUnopenableFile covers the half-written state: the sealed
// file is written before the data key is stored, so a store that refuses the
// write would otherwise leave a file nothing can ever open and which makes
// every later Create refuse.
func TestCreateLeavesNoUnopenableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync-keys.json")
	provider := &nativeSyncSecretStore{
		path:     path,
		provider: failingSetProvider{credentials.NewMemoryProvider()},
		ref:      credentials.Reference{Service: "notrios-test", Account: "db"},
	}
	if _, err := provider.Create(); err == nil {
		t.Fatalf("Create must fail when the data key cannot be stored")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a key file was left behind that no data key can open: %v", err)
	}
}

// TestOpenWithoutAStoredKeyIsATypedRefusal keeps "the keychain is empty"
// distinguishable from "the keychain is broken", because enrolment tells the
// user to do different things about them.
func TestOpenWithoutAStoredKeyIsATypedRefusal(t *testing.T) {
	dir := t.TempDir()
	provider := &nativeSyncSecretStore{
		path:     filepath.Join(dir, "sync-keys.json"),
		provider: credentials.NewMemoryProvider(),
		ref:      credentials.Reference{Service: "notrios-test", Account: "db"},
	}
	_, err := provider.Open()
	if !errors.Is(err, synckeys.ErrNoKeyFile) {
		t.Fatalf("want ErrNoKeyFile, got %v", err)
	}
}
