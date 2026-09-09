// The build constraint is load-bearing and the negations are not redundant.
// Go sets the `linux` tag for GOOS=android and the `darwin` tag for GOOS=ios,
// so `linux || darwin` alone compiles this file into a mobile build, where
// go-keyring reaches for a Secret Service or an /usr/bin/security that is not
// there. That failure would appear at runtime rather than at build time, on
// the platforms where it is least debuggable, so it is excluded here and
// asserted by a cross-build gate.
//go:build (linux || windows || darwin) && !android && !ios

package credentials

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// probeAccount is a reference that is never written. Its only use is asking
// the store a question it can answer without side effects.
const probeAccount = "notrios-availability-probe"

type nativeProvider struct{}

func newNativeProvider() Provider { return nativeProvider{} }

func (nativeProvider) Name() string { return nativeStoreName }

// Availability asks the store for a secret that was never stored. A store that
// is present answers "not found"; a store that is absent fails to answer at
// all. That distinction is the whole probe, and it is read-only, which matters
// because a probe that wrote would leave litter in a user's keychain.
func (nativeProvider) Availability() Availability {
	_, err := keyring.Get(availabilityService, probeAccount)
	switch {
	case err == nil, errors.Is(err, keyring.ErrNotFound):
		return Availability{Available: true}
	default:
		return Availability{Reason: fmt.Sprintf("%s did not respond: %v", nativeStoreName, err)}
	}
}

func (nativeProvider) Get(ref Reference) ([]byte, error) {
	if err := ref.validate(); err != nil {
		return nil, err
	}
	encoded, err := keyring.Get(ref.Service, ref.Account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	// Secrets are stored base64-encoded because these stores carry strings and
	// key material is bytes; a raw key would be mangled by any store that
	// assumes UTF-8.
	secret, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("stored credential is not valid base64")
	}
	return secret, nil
}

func (nativeProvider) Set(ref Reference, secret []byte) error {
	if err := ref.validate(); err != nil {
		return err
	}
	if len(secret) > MaxSecretBytes {
		return fmt.Errorf("%w: %d bytes exceeds %d", ErrSecretTooLarge, len(secret), MaxSecretBytes)
	}
	if err := keyring.Set(ref.Service, ref.Account, base64.StdEncoding.EncodeToString(secret)); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

func (nativeProvider) Delete(ref Reference) error {
	if err := ref.validate(); err != nil {
		return err
	}
	err := keyring.Delete(ref.Service, ref.Account)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}
