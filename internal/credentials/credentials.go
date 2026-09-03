// Package credentials owns where a secret lives. Its one rule is that it
// refuses rather than guesses: when the store a profile was configured to use
// is unavailable, no other store is tried, because a credential that silently
// moved to a weaker place is worse than one that could not be read at all.
package credentials

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound reports that the store answered and holds no such secret.
	// It is distinct from ErrUnavailable on purpose: the first means "the
	// keychain is there and empty", the second means "there is no keychain",
	// and enrolment must treat them differently.
	ErrNotFound = errors.New("credential not found")

	// ErrUnavailable reports that the store could not be reached at all.
	ErrUnavailable = errors.New("credential store unavailable")

	// ErrSecretTooLarge reports a secret this store cannot hold. Windows
	// Credential Manager caps a blob at 2560 bytes, so a caller that stores a
	// growing document rather than a fixed-size key would work until a library
	// had enough peers and then stop. Refusing early turns that into a bug
	// found once rather than a failure found in the field.
	ErrSecretTooLarge = errors.New("credential exceeds the smallest supported store limit")
)

// MaxSecretBytes is the largest secret any supported store is guaranteed to
// hold. It is deliberately below the 2560-byte Windows limit: callers should be
// storing a key, not a document, and the margin makes that a design constraint
// rather than a coincidence.
const MaxSecretBytes = 2048

// Reference names one secret without describing it. Callers hold references;
// only a Provider knows what a reference means to its own store.
type Reference struct {
	// Service groups a profile's secrets. Account distinguishes them within it.
	Service string
	Account string
}

func (r Reference) validate() error {
	if r.Service == "" || r.Account == "" {
		return fmt.Errorf("%w: reference needs both a service and an account", ErrNotFound)
	}
	return nil
}

// Availability answers whether a provider can be used right now, and says why
// when it cannot. The reason is written to be shown to a person: it appears in
// `notriosctl doctor` and in the refusal at enrolment, so the user learns that
// this installation cannot hold sync credentials before they try to pair,
// rather than when a sync first fails.
type Availability struct {
	Available bool
	// Reason is empty when Available. It never contains secret material.
	Reason string
}

// Provider is one place a secret can live.
type Provider interface {
	// Name identifies the store for logs, doctor output and the sync UI.
	Name() string
	// Availability probes the store. It must not create or modify anything.
	Availability() Availability
	Get(Reference) ([]byte, error)
	Set(Reference, []byte) error
	Delete(Reference) error
}

// Kind is the set of providers a profile may be configured to use. There is no
// "automatic" kind, and that is the point: selection is a recorded decision
// rather than whatever happened to answer first.
type Kind string

const (
	// KindNative is the operating system's own credential store.
	KindNative Kind = "native"
	// KindMemory is process-local and never persisted. It exists for tests and
	// for a host that supplies a secret across an ABI boundary; it is never
	// selected as a substitute for a store that failed.
	KindMemory Kind = "memory"
)

// Select returns the named provider, or an error explaining why it cannot be
// used. It never returns a different provider than the one asked for.
func Select(kind Kind) (Provider, error) {
	var provider Provider
	switch kind {
	case KindNative:
		provider = newNativeProvider()
	case KindMemory:
		provider = NewMemoryProvider()
	default:
		return nil, fmt.Errorf("unknown credential store %q", kind)
	}
	return selectProvider(provider)
}

// selectProvider is the refusal itself, split out from Select so it can be
// tested against a store that is unavailable. On a developer's desktop the
// native store always answers, which would leave the one branch this item
// exists to guarantee -- refuse, do not substitute -- exercised nowhere.
func selectProvider(provider Provider) (Provider, error) {
	if availability := provider.Availability(); !availability.Available {
		return nil, fmt.Errorf("%w: %s is not usable here: %s",
			ErrUnavailable, provider.Name(), availability.Reason)
	}
	return provider, nil
}
