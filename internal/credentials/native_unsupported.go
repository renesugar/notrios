// The inverse of native_desktop.go's constraint. Every target that is not a
// supported desktop gets a provider that refuses with a reason, rather than no
// provider at all: a missing symbol would be a link error, while this is a
// typed refusal the caller can report. Mobile reaches this file, which is
// correct -- on mobile the host supplies the credential across the ABI and the
// core is never the owner.
//go:build !((linux || windows || darwin) && !android && !ios)

package credentials

import "runtime"

type nativeProvider struct{}

func newNativeProvider() Provider { return nativeProvider{} }

func (nativeProvider) Name() string { return "unsupported-platform" }

func (nativeProvider) Availability() Availability {
	return Availability{Reason: "no native credential store is supported on " + runtime.GOOS +
		"; a host must supply the credential instead"}
}

func (nativeProvider) Get(Reference) ([]byte, error) { return nil, ErrUnavailable }
func (nativeProvider) Set(Reference, []byte) error   { return ErrUnavailable }
func (nativeProvider) Delete(Reference) error        { return ErrUnavailable }
