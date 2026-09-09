package config

import "fmt"

// CredentialStoreResolution is which store a profile actually uses, and what
// to tell the user about it.
//
// It exists because three callers have to agree -- the service, `notriosctl
// sync`, and `doctor` -- and the last time this kind of resolution was written
// twice, the CLI and the service silently addressed different key files.
type CredentialStoreResolution struct {
	// Kind is the store to use: CredentialStoreNative or
	// CredentialStoreDevelopmentFile.
	Kind string
	// FromConfig is true when the profile named the store itself. An explicit
	// choice is never overridden by anything below.
	FromConfig bool
	// Advisory is non-empty when the user should act. It is not a warning
	// about danger so much as an instruction with a reason, and every surface
	// that can show it should.
	Advisory string
}

// ResolveCredentialStore decides where a profile's sync secret lives.
//
// The rule, and why each part of it is there:
//
// An explicit setting always wins, including an explicit choice of the
// development file. A user who wrote it down meant it.
//
// An installed profile with no key material yet defaults to the operating
// system's store, because that is the whole point of this milestone and a new
// library has nothing to lose by starting there.
//
// An installed profile that already has a plaintext key file keeps using it,
// and is told to migrate. This is the case that needs the most care: defaulting
// it to the keychain would strand a library whose keys are in a file the
// keychain knows nothing about, and silently continuing would be the downgrade
// this milestone forbids. It is neither -- nothing changes underneath the user,
// and every surface says what is happening and what to do about it.
//
// A source checkout keeps the development file. A working tree is not an
// installation, and putting a developer's throwaway libraries into their real
// keychain would leave litter they never asked for.
func ResolveCredentialStore(configured string, installed, plaintextKeyMaterial, sealedKeyMaterial bool) CredentialStoreResolution {
	// Any non-empty value is passed straight back, including one this package
	// does not recognise. Normalising an unknown store to a working default
	// here would silently undo the caller's refusal of it, and a profile naming
	// a store that does not exist must fail rather than quietly get another.
	if configured != "" {
		return CredentialStoreResolution{Kind: configured, FromConfig: true}
	}
	// A library that already has key material answers this itself, and must,
	// because the process's own mode is the wrong thing to ask. A service
	// started from a checkout and a command run from a sandbox resolve
	// different modes while addressing the same library, and would then
	// disagree about where its keys live -- a divergence found by exactly that
	// pairing in the sync tests. Only a library with no material yet falls
	// through to the mode.
	if sealedKeyMaterial {
		return CredentialStoreResolution{Kind: CredentialStoreNative}
	}
	if plaintextKeyMaterial {
		return CredentialStoreResolution{
			Kind:     CredentialStoreDevelopmentFile,
			Advisory: advisoryWhenInstalled(installed),
		}
	}
	if !installed {
		return CredentialStoreResolution{Kind: CredentialStoreDevelopmentFile}
	}
	return CredentialStoreResolution{Kind: CredentialStoreNative}
}

// advisoryWhenInstalled is empty for a source checkout: a working tree keeping
// its keys in a file is the expected arrangement, not something to nag about.
func advisoryWhenInstalled(installed bool) string {
	if !installed {
		return ""
	}
	return fmt.Sprintf(
		"this library's sync keys are still in the %s, which an installed profile no longer "+
			"defaults to; move them with `notriosctl sync migrate-credentials --to native`",
		"development file")
}
