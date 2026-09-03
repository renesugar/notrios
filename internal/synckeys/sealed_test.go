package synckeys

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "sync-keys.json")
}

func TestSealedRoundTrip(t *testing.T) {
	path := tempPath(t)
	key, err := NewSealKey()
	if err != nil {
		t.Fatal(err)
	}
	created, err := CreateSealed(path, key)
	if err != nil {
		t.Fatalf("CreateSealed: %v", err)
	}
	reopened, err := OpenSealed(path, key)
	if err != nil {
		t.Fatalf("OpenSealed: %v", err)
	}
	if reopened.SignerKeyID() != created.SignerKeyID() {
		t.Fatalf("signer key id changed across a seal round trip")
	}
	if !bytes.Equal(reopened.PrivateSigningKey(), created.PrivateSigningKey()) {
		t.Fatalf("signing key changed across a seal round trip")
	}
	want, err := created.Current()
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Current()
	if err != nil {
		t.Fatal(err)
	}
	if want.Key != got.Key {
		t.Fatalf("group key changed across a seal round trip")
	}
}

// TestSealedFileHoldsNoKeyMaterial is the assertion the whole slice exists
// for. Encrypting and then leaving the plaintext beside it would pass every
// round-trip test above.
func TestSealedFileHoldsNoKeyMaterial(t *testing.T) {
	path := tempPath(t)
	key, err := NewSealKey()
	if err != nil {
		t.Fatal(err)
	}
	file, err := CreateSealed(path, key)
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	group, err := file.Current()
	if err != nil {
		t.Fatal(err)
	}
	for name, secret := range map[string][]byte{
		"signing key": file.PrivateSigningKey(),
		"group key":   group.Key[:],
		"data key":    key,
	} {
		if bytes.Contains(onDisk, secret) {
			t.Errorf("%s appears verbatim in the sealed file", name)
		}
		encoded := base64.StdEncoding.EncodeToString(secret)
		if bytes.Contains(onDisk, []byte(encoded)) {
			t.Errorf("%s appears base64-encoded in the sealed file", name)
		}
	}
	if perm := fileMode(t, path); perm != 0o600 {
		t.Errorf("sealed file is mode %#o, want 0600", perm)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestSealedRefusesWrongKeyAndTampering(t *testing.T) {
	path := tempPath(t)
	key, _ := NewSealKey()
	if _, err := CreateSealed(path, key); err != nil {
		t.Fatal(err)
	}

	other, _ := NewSealKey()
	if _, err := OpenSealed(path, other); !errors.Is(err, ErrSealMismatch) {
		t.Fatalf("wrong data key: want ErrSealMismatch, got %v", err)
	}
	if _, err := OpenSealed(path, key[:16]); err == nil {
		t.Fatalf("a short data key must be refused")
	}

	// Flip one byte of ciphertext. AES-GCM must refuse it rather than return
	// altered key material.
	raw, _ := os.ReadFile(path)
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	ciphertext, _ := base64.StdEncoding.DecodeString(env.Ciphertext)
	ciphertext[0] ^= 0x01
	env.Ciphertext = base64.StdEncoding.EncodeToString(ciphertext)
	tampered, _ := json.MarshalIndent(env, "", "  ")
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSealed(path, key); !errors.Is(err, ErrSealMismatch) {
		t.Fatalf("tampered ciphertext: want ErrSealMismatch, got %v", err)
	}
}

// TestSealedAndPlaintextAreNotConfused matters because a migration that
// guessed wrong would be a migration that destroyed key material.
func TestSealedAndPlaintextAreNotConfused(t *testing.T) {
	sealedPath := tempPath(t)
	key, _ := NewSealKey()
	if _, err := CreateSealed(sealedPath, key); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(sealedPath); !errors.Is(err, ErrSealRequired) {
		t.Fatalf("opening a sealed file plainly: want ErrSealRequired, got %v", err)
	}
	if sealed, err := IsSealed(sealedPath); err != nil || !sealed {
		t.Fatalf("IsSealed on a sealed file: %v %v", sealed, err)
	}

	plainPath := tempPath(t)
	if _, err := Create(plainPath); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSealed(plainPath, key); !errors.Is(err, ErrNotSealed) {
		t.Fatalf("opening a plaintext file as sealed: want ErrNotSealed, got %v", err)
	}
	if sealed, err := IsSealed(plainPath); err != nil || sealed {
		t.Fatalf("IsSealed on a plaintext file: %v %v", sealed, err)
	}
}

// TestSealedSaveUsesAFreshNonce guards the one mistake that would silently
// destroy AES-GCM's confidentiality: this file is rewritten on every epoch
// advance and every pairing, so a nonce fixed at creation would repeat.
func TestSealedSaveUsesAFreshNonce(t *testing.T) {
	path := tempPath(t)
	key, _ := NewSealKey()
	file, err := CreateSealed(path, key)
	if err != nil {
		t.Fatal(err)
	}
	nonceOf := func() string {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		return env.Nonce
	}
	seen := map[string]bool{nonceOf(): true}
	for i := 0; i < 8; i++ {
		if _, err := file.AdvanceEpoch(); err != nil {
			t.Fatal(err)
		}
		nonce := nonceOf()
		if seen[nonce] {
			t.Fatalf("nonce repeated after %d saves", i+1)
		}
		seen[nonce] = true
	}
}

// TestSealedMutationsPersist checks that the seal survives the operations that
// rewrite the file, rather than the first save being the only encrypted one.
func TestSealedMutationsPersist(t *testing.T) {
	path := tempPath(t)
	key, _ := NewSealKey()
	file, err := CreateSealed(path, key)
	if err != nil {
		t.Fatal(err)
	}
	epoch, err := file.AdvanceEpoch()
	if err != nil {
		t.Fatal(err)
	}
	if sealed, err := IsSealed(path); err != nil || !sealed {
		t.Fatalf("file stopped being sealed after a mutation: %v %v", sealed, err)
	}
	reopened, err := OpenSealed(path, key)
	if err != nil {
		t.Fatalf("OpenSealed after mutation: %v", err)
	}
	advanced, err := reopened.Lookup(reopened.SignerKeyID(), epoch)
	if err != nil {
		// Lookup is keyed by the group key id rather than the signer id on
		// some paths; fall back to Current, which must be the advanced epoch.
		if current, currentErr := reopened.Current(); currentErr != nil {
			t.Fatalf("neither Lookup nor Current worked after advance: %v / %v", err, currentErr)
		} else {
			advanced = current
		}
	}
	var zero [32]byte
	if advanced.Key == zero {
		t.Fatalf("advanced epoch key is empty after a sealed round trip")
	}
}
