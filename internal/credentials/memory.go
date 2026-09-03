package credentials

import "sync"

// MemoryProvider keeps secrets for the life of the process and never writes
// them anywhere. It has two legitimate users: tests, and a host that supplies
// a secret across the ABI on a platform where the core is not the owner.
//
// It is never reached by falling back. Select returns it only when a caller
// asked for KindMemory by name, because a provider that quietly replaced a
// keychain with process memory would lose a user's credential on restart while
// appearing to work.
type MemoryProvider struct {
	mu      sync.Mutex
	secrets map[Reference][]byte
}

func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{secrets: map[Reference][]byte{}}
}

func (*MemoryProvider) Name() string { return "process-memory" }

func (*MemoryProvider) Availability() Availability { return Availability{Available: true} }

func (p *MemoryProvider) Get(ref Reference) ([]byte, error) {
	if err := ref.validate(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	secret, ok := p.secrets[ref]
	if !ok {
		return nil, ErrNotFound
	}
	// Copied on the way out so a caller cannot mutate stored material through
	// the slice it was handed.
	return append([]byte(nil), secret...), nil
}

func (p *MemoryProvider) Set(ref Reference, secret []byte) error {
	if err := ref.validate(); err != nil {
		return err
	}
	if len(secret) > MaxSecretBytes {
		return ErrSecretTooLarge
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.secrets[ref] = append([]byte(nil), secret...)
	return nil
}

func (p *MemoryProvider) Delete(ref Reference) error {
	if err := ref.validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.secrets[ref]; !ok {
		return ErrNotFound
	}
	delete(p.secrets, ref)
	return nil
}
