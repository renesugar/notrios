package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/credentials"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/synckeys"
)

// syncSecretProvider is what the service needs beyond the HTTP surface: the
// opened key file itself, which durable jobs and the peer-authenticated
// surface both hold directly. Keeping it unexported keeps the choice of store
// an implementation detail of this package rather than something the API
// layer can reach around.
type syncSecretProvider interface {
	httpapi.SyncSecretStore
	openKeyFile() (*synckeys.KeyFile, error)
}

// newSyncSecretStore selects the configured store. It never falls back: a
// profile configured for the operating system's keychain that cannot reach it
// gets an error naming the reason, not a quiet downgrade to a file on disk.
// That refusal is the whole point of the configuration existing.
func newSyncSecretStore(cfg config.Config, st *store.SQLiteStore) (syncSecretProvider, error) {
	path, err := resolveKeyFilePath(cfg, st)
	if err != nil {
		return nil, err
	}
	resolution := resolveCredentialStore(cfg, path)
	switch kind := resolution.Kind; kind {
	case config.CredentialStoreDevelopmentFile:
		return &fileSyncSecretStore{path: path, advisory: resolution.Advisory}, nil
	case config.CredentialStoreNative:
		provider, err := credentials.Select(credentials.KindNative)
		if err != nil {
			return nil, err
		}
		identity, err := st.GetDatabaseIdentity(context.Background())
		if err != nil {
			return nil, err
		}
		return &nativeSyncSecretStore{
			path:     path,
			provider: provider,
			ref:      credentials.SyncReference(identity.DatabaseID, path),
		}, nil
	default:
		return nil, fmt.Errorf("unknown sync credential store %q", kind)
	}
}

// nativeSyncSecretStore keeps the key material sealed on disk and only the
// data key that opens it in the operating system's store. The split is
// forced by size: the key material grows with every peer and epoch and
// overruns what a native store will hold, while a data key never does.
type nativeSyncSecretStore struct {
	path     string
	provider credentials.Provider
	ref      credentials.Reference
	mu       sync.Mutex
	keys     *synckeys.KeyFile
}

func (p *nativeSyncSecretStore) ProviderName() string { return p.provider.Name() }

// Warning is empty because there is nothing to warn about: this is the store
// the operating system provides. The development file's warning is not a
// house style to copy, it is a statement about that file specifically.
func (p *nativeSyncSecretStore) Warning() string { return "" }

func (p *nativeSyncSecretStore) openKeyFile() (*synckeys.KeyFile, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.keys != nil {
		return p.keys, nil
	}
	dataKey, err := p.provider.Get(p.ref)
	if errors.Is(err, credentials.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s holds no data key for this library", synckeys.ErrNoKeyFile, p.provider.Name())
	}
	if err != nil {
		return nil, err
	}
	keys, err := synckeys.OpenSealed(p.path, dataKey)
	if err != nil {
		return nil, err
	}
	p.keys = keys
	return keys, nil
}

func (p *nativeSyncSecretStore) Open() (httpapi.SyncLocalKeys, error) { return p.openKeyFile() }

func (p *nativeSyncSecretStore) Create() (httpapi.SyncLocalKeys, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.keys != nil {
		return p.keys, nil
	}
	dataKey, err := synckeys.NewSealKey()
	if err != nil {
		return nil, err
	}
	// The file is written first because it is the operation that refuses to
	// overwrite, so it is what detects a library that already has key
	// material. Only then is the data key stored.
	keys, err := synckeys.CreateSealed(p.path, dataKey)
	if err != nil {
		return nil, err
	}
	if err := p.provider.Set(p.ref, dataKey); err != nil {
		// The file was created moments ago and nothing has used it, so it
		// holds no material anyone can lose. Leaving it would be worse than
		// removing it: it would be an unopenable file that makes every later
		// Create refuse, with no data key anywhere to open it.
		_ = os.Remove(p.path)
		return nil, fmt.Errorf("storing the data key in %s failed, so no key material was kept: %w",
			p.provider.Name(), err)
	}
	p.keys = keys
	return keys, nil
}
