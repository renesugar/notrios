package credentials

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// runProviderContract is the one suite every provider must pass. Writing it
// once is what makes "swap the provider" a real claim rather than a hope.
func runProviderContract(t *testing.T, p Provider) {
	t.Helper()
	ref := Reference{Service: "notrios-contract-test", Account: "contract"}
	secret := []byte("a 32-byte data key would go here")

	if _, err := p.Get(ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reading an absent secret: want ErrNotFound, got %v", err)
	}
	if err := p.Set(ref, secret); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Cleanup(func() { _ = p.Delete(ref) })

	got, err := p.Get(ref)
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("round trip changed the secret")
	}

	// A returned slice must not alias stored material.
	got[0] ^= 0xff
	again, err := p.Get(ref)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if !bytes.Equal(again, secret) {
		t.Fatalf("mutating a returned slice changed the stored secret")
	}

	if err := p.Delete(ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := p.Get(ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after Delete: want ErrNotFound, got %v", err)
	}
	if err := p.Delete(ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting twice: want ErrNotFound, got %v", err)
	}

	if err := p.Set(ref, make([]byte, MaxSecretBytes+1)); !errors.Is(err, ErrSecretTooLarge) {
		t.Fatalf("oversized secret: want ErrSecretTooLarge, got %v", err)
	}
	if err := p.Set(Reference{Service: "x"}, secret); err == nil {
		t.Fatalf("a reference with no account must be refused")
	}
}

func TestMemoryProviderContract(t *testing.T) {
	runProviderContract(t, NewMemoryProvider())
}

// TestNativeProviderContract runs the same suite against the operating
// system's own store. It is skipped rather than failed where no store is
// reachable, because "this machine has no keyring" is a fact about the machine
// and not a defect in the code -- but where a store does answer, the provider
// is held to exactly the contract the memory one is.
func TestNativeProviderContract(t *testing.T) {
	provider := newNativeProvider()
	if availability := provider.Availability(); !availability.Available {
		t.Skipf("no native credential store here: %s", availability.Reason)
	}
	runProviderContract(t, provider)
}

// TestSelectRefusesRatherThanSubstitutes is the item's central boundary as a
// test. A provider that cannot be used must produce an error naming the
// reason, never a different provider that happens to work.
func TestSelectRefusesRatherThanSubstitutes(t *testing.T) {
	if _, err := Select("no-such-store"); err == nil {
		t.Fatalf("an unknown store must be refused")
	}
	provider := newNativeProvider()
	availability := provider.Availability()
	selected, err := Select(KindNative)
	switch {
	case availability.Available:
		if err != nil {
			t.Fatalf("native store is available but Select failed: %v", err)
		}
		if selected.Name() != provider.Name() {
			t.Fatalf("Select returned %q, not the requested %q", selected.Name(), provider.Name())
		}
	default:
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("unavailable native store: want ErrUnavailable, got %v", err)
		}
		if selected != nil {
			t.Fatalf("a refused selection must return no provider, got %q", selected.Name())
		}
		if !strings.Contains(err.Error(), availability.Reason) {
			t.Fatalf("the refusal must carry the reason a user can act on; got %v", err)
		}
	}
}

// TestNoDesktopProviderInMobileBuilds is the cross-build gate. The desktop
// provider compiles for Android unless its constraint says !android, because
// Go sets the linux tag for GOOS=android; the same holds for ios and darwin.
// Asserting the linked package set is cheaper than an emulator and catches the
// mistake at the only time it is visible.
func TestNoDesktopProviderInMobileBuilds(t *testing.T) {
	forbidden := []string{"zalando/go-keyring", "godbus/dbus", "danieljoos/wincred"}
	for _, target := range []struct{ goos, goarch string }{
		{"android", "arm64"},
		{"ios", "arm64"},
	} {
		cmd := exec.Command("go", "list", "-deps", ".")
		cmd.Env = append(cmd.Environ(), "GOOS="+target.goos, "GOARCH="+target.goarch, "CGO_ENABLED=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			// ios/arm64 refuses to resolve without cgo, which is a toolchain
			// rule rather than a result. Report it instead of hiding it.
			t.Logf("%s/%s not resolvable here: %v", target.goos, target.goarch, err)
			continue
		}
		for _, pkg := range forbidden {
			if strings.Contains(string(out), pkg) {
				t.Errorf("%s/%s links %s; the desktop provider must be excluded from mobile builds",
					target.goos, target.goarch, pkg)
			}
		}
	}
}

// unavailableProvider stands in for a machine with no keyring, a locked
// keychain, or a service started without a session.
type unavailableProvider struct{ reason string }

func (unavailableProvider) Name() string { return "stub-unavailable" }
func (p unavailableProvider) Availability() Availability {
	return Availability{Reason: p.reason}
}
func (unavailableProvider) Get(Reference) ([]byte, error) { return nil, ErrUnavailable }
func (unavailableProvider) Set(Reference, []byte) error   { return ErrUnavailable }
func (unavailableProvider) Delete(Reference) error        { return ErrUnavailable }

// TestUnavailableStoreIsRefusedNotReplaced exercises the branch a developer
// machine never reaches, because its keyring always answers.
func TestUnavailableStoreIsRefusedNotReplaced(t *testing.T) {
	const reason = "the session bus has no org.freedesktop.secrets"
	provider, err := selectProvider(unavailableProvider{reason: reason})
	if provider != nil {
		t.Fatalf("a refused selection must return no provider, got %q", provider.Name())
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if !strings.Contains(err.Error(), reason) {
		t.Fatalf("the refusal must name the reason the user can act on; got %v", err)
	}
}
