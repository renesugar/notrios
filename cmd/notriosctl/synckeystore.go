package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/credentials"
	"github.com/renesugar/notrios/internal/synckeys"
)

// syncKeyStore is where this profile's key material lives and what protects
// it. The CLI resolves it exactly as the service does, because the two
// disagreeing about that is not a cosmetic difference: the H8 matrix already
// found one case of a command and a service addressing different libraries,
// and a command that wrote a plaintext key file for a profile configured to
// use the keychain would be the same mistake with worse consequences.
type syncKeyStore struct {
	path string
	// provider is nil for the development file, which is protected only by
	// its mode and says so.
	provider credentials.Provider
	ref      credentials.Reference
	// unavailable records why a configured store could not be used. It is
	// carried rather than returned so the caller can refuse with the reason.
	unavailable error
}

// keyStore resolves the store without opening anything, so a caller can report
// a problem before it acts. The precedence matches the service: an explicit
// --keys flag, then the config's key_file, then the default path.
func (f *syncFlags) keyStore(databaseID string) *syncKeyStore {
	cfg, err := config.Load(*f.configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	path := strings.TrimSpace(*f.keysPath)
	if path == "" {
		path = strings.TrimSpace(cfg.Sync.REST.KeyFile)
	}
	if path == "" {
		resolved, err := synckeys.DefaultPath(databaseID)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		path = resolved
	}
	store := &syncKeyStore{path: path}
	switch kind := strings.TrimSpace(cfg.Sync.REST.CredentialStore); kind {
	case "", config.CredentialStoreDevelopmentFile:
		return store
	case config.CredentialStoreNative:
		provider, err := credentials.Select(credentials.KindNative)
		if err != nil {
			store.unavailable = err
			return store
		}
		store.provider = provider
		store.ref = credentials.Reference{Service: "notrios-sync", Account: databaseID}
		return store
	default:
		store.unavailable = fmt.Errorf("unknown sync credential store %q", kind)
		return store
	}
}

// warning is what a user should be told about this store, or empty when there
// is nothing to say.
func (s *syncKeyStore) warning() string {
	if s.provider == nil && s.unavailable == nil {
		return developmentKeyWarning
	}
	return ""
}

func (s *syncKeyStore) describe() string {
	switch {
	case s.unavailable != nil:
		return "unavailable"
	case s.provider != nil:
		return s.provider.Name()
	default:
		return "locked-file-development"
	}
}

func (s *syncKeyStore) open() (*synckeys.KeyFile, error) {
	if s.unavailable != nil {
		return nil, s.unavailable
	}
	if s.provider == nil {
		return synckeys.Open(s.path)
	}
	dataKey, err := s.provider.Get(s.ref)
	if errors.Is(err, credentials.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s holds no data key for this library",
			synckeys.ErrNoKeyFile, s.provider.Name())
	}
	if err != nil {
		return nil, err
	}
	return synckeys.OpenSealed(s.path, dataKey)
}

func (s *syncKeyStore) create() (*synckeys.KeyFile, error) {
	if s.unavailable != nil {
		return nil, s.unavailable
	}
	if s.provider == nil {
		return synckeys.Create(s.path)
	}
	dataKey, err := synckeys.NewSealKey()
	if err != nil {
		return nil, err
	}
	keys, err := synckeys.CreateSealed(s.path, dataKey)
	if err != nil {
		return nil, err
	}
	if err := s.provider.Set(s.ref, dataKey); err != nil {
		// Same reasoning as the service: a sealed file whose data key was
		// never stored can never be opened, and would block every later
		// attempt. It has held nothing for long enough to matter.
		_ = os.Remove(s.path)
		return nil, fmt.Errorf("storing the data key in %s failed, so no key material was kept: %w",
			s.provider.Name(), err)
	}
	return keys, nil
}

// mustOpen exits with the reason rather than a bare failure, because the two
// reasons a user sees here -- no key material yet, and no reachable credential
// store -- call for completely different next steps.
func (s *syncKeyStore) mustOpen() *synckeys.KeyFile {
	keys, err := s.open()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return keys
}
