package config

import "testing"

func TestResolveCredentialStore(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		configured                   string
		installed, plaintext, sealed bool
		wantKind                     string
		wantFromConfig               bool
		wantAdvisory                 bool
	}{
		{"explicit native wins everywhere", CredentialStoreNative, false, true, false,
			CredentialStoreNative, true, false},
		{"explicit development file is honoured", CredentialStoreDevelopmentFile, true, false, true,
			CredentialStoreDevelopmentFile, true, false},
		// An unknown value must survive to the caller, which refuses it. Turning
		// it into a working default here would undo that refusal.
		{"an unknown value is passed back for the caller to refuse", "nonsense", true, false, false,
			"nonsense", true, false},
		{"a source checkout keeps the development file", "", false, false, false,
			CredentialStoreDevelopmentFile, false, false},
		{"a source checkout with keys keeps them and says nothing", "", false, true, false,
			CredentialStoreDevelopmentFile, false, false},
		{"a fresh installed profile defaults to the keychain", "", true, false, false,
			CredentialStoreNative, false, false},
		{"an installed profile with existing keys keeps them and advises", "", true, true, false,
			CredentialStoreDevelopmentFile, false, true},
		// The library's own state outranks the process's mode. A service run
		// from a checkout and a command run from a sandbox must not disagree
		// about where an existing library's keys live.
		{"sealed material means the keychain even in a checkout", "", false, false, true,
			CredentialStoreNative, false, false},
		{"sealed material means the keychain when installed", "", true, false, true,
			CredentialStoreNative, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveCredentialStore(tc.configured, tc.installed, tc.plaintext, tc.sealed)
			if got.Kind != tc.wantKind {
				t.Errorf("kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if got.FromConfig != tc.wantFromConfig {
				t.Errorf("from config = %v, want %v", got.FromConfig, tc.wantFromConfig)
			}
			if (got.Advisory != "") != tc.wantAdvisory {
				t.Errorf("advisory = %q, want present=%v", got.Advisory, tc.wantAdvisory)
			}
		})
	}
}

// TestUpgradeNeverStrandsExistingKeys is the case the recommendation turned on:
// an installed library that predates this milestone must keep reading the keys
// it has, and must be told rather than left to find out.
func TestUpgradeNeverStrandsExistingKeys(t *testing.T) {
	got := ResolveCredentialStore("", true, true, false)
	if got.Kind != CredentialStoreDevelopmentFile {
		t.Fatalf("an upgraded library was pointed at a store that does not hold its keys")
	}
	if got.Advisory == "" {
		t.Fatalf("continuing on the old store without saying so is the silent downgrade this item forbids")
	}
	if !contains(got.Advisory, "migrate-credentials") {
		t.Fatalf("the advisory must name the command that fixes it: %q", got.Advisory)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
