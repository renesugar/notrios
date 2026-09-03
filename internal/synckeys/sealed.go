package synckeys

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// SealKeyBytes is the size of the data key that protects a sealed key file.
const SealKeyBytes = 32

// sealedAlgorithm names the construction in the file itself, so a future
// change is a value a reader can refuse rather than a silent reinterpretation
// of the same bytes.
const sealedAlgorithm = "aes-256-gcm"

// sealDomain is mixed into the authenticated data so a sealed key file cannot
// be presented to some other part of this system that also uses AES-GCM.
const sealDomain = "notrios/synckeys/sealed/v1"

var (
	// ErrSealRequired reports a sealed file opened without a data key.
	ErrSealRequired = errors.New("sync key file is sealed and needs its data key from the credential store")
	// ErrNotSealed reports a plaintext file opened as though it were sealed.
	// It is separate from ErrSealRequired because the two say opposite things
	// about what the caller should do next, and a migration that guessed would
	// be a migration that could destroy key material.
	ErrNotSealed = errors.New("sync key file is not sealed")
	// ErrSealMismatch reports a data key that does not open this file. It is
	// the same error whether the key is wrong or the file was altered, because
	// AES-GCM cannot tell the caller which and neither should this.
	ErrSealMismatch = errors.New("sync key file did not open with the supplied data key")
)

// envelope is what a sealed key file contains instead of the key material.
// The plaintext form has no "sealed" member, which is how Open tells them
// apart without a second file or a naming convention.
type envelope struct {
	Version    int    `json:"version"`
	Sealed     string `json:"sealed"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func sealAEAD(dataKey []byte) (cipher.AEAD, error) {
	if len(dataKey) != SealKeyBytes {
		return nil, fmt.Errorf("data key must be %d bytes, got %d", SealKeyBytes, len(dataKey))
	}
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealedAAD binds a ciphertext to its own header. Without it an attacker who
// could edit the file could claim a different algorithm or version and have
// the reader accept the same bytes under different rules.
func sealedAAD(version int, algorithm string) []byte {
	return []byte(fmt.Sprintf("%s\x00%d\x00%s", sealDomain, version, algorithm))
}

// IsSealed reports whether the file at path is sealed, without opening it.
// Callers use it to decide which constructor to use rather than trying one and
// interpreting the failure.
func IsSealed(path string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var probe envelope
	if err := json.Unmarshal(contents, &probe); err != nil {
		return false, fmt.Errorf("%w: %v", ErrUnknownFormat, err)
	}
	return probe.Sealed != "", nil
}

// NewSealKey returns a fresh data key. It is the only key this package asks a
// credential store to hold: the key material itself stays on disk, because it
// grows with every peer and epoch and would outgrow what a native store will
// accept.
func NewSealKey() ([]byte, error) {
	key := make([]byte, SealKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// CreateSealed writes new key material encrypted under dataKey. Like Create it
// refuses to overwrite, for the same reason: replacing a group key silently
// makes everything a peer already published unreadable.
func CreateSealed(path string, dataKey []byte) (*KeyFile, error) {
	if _, err := sealAEAD(dataKey); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("sync key file already exists: %s", path)
	}
	file, err := newKeyMaterial(path)
	if err != nil {
		return nil, err
	}
	file.seal = append([]byte(nil), dataKey...)
	if err := file.save(); err != nil {
		return nil, err
	}
	return file, nil
}

// OpenSealed reads key material encrypted under dataKey.
func OpenSealed(path string, dataKey []byte) (*KeyFile, error) {
	aead, err := sealAEAD(dataKey)
	if err != nil {
		return nil, err
	}
	contents, err := readKeyFileBytes(path)
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(contents, &env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnknownFormat, err)
	}
	if env.Sealed == "" {
		return nil, fmt.Errorf("%w: %s", ErrNotSealed, path)
	}
	if env.Sealed != sealedAlgorithm || env.Version != FileVersion {
		return nil, fmt.Errorf("%w: sealed with %q version %d", ErrUnknownFormat, env.Sealed, env.Version)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil || len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("%w: nonce is not %d bytes", ErrUnknownFormat, aead.NonceSize())
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: ciphertext is not base64", ErrUnknownFormat)
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, sealedAAD(env.Version, env.Sealed))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSealMismatch, path)
	}
	file := &KeyFile{path: path, seal: append([]byte(nil), dataKey...)}
	if err := file.decode(plaintext); err != nil {
		return nil, err
	}
	return file, nil
}

// sealBytes encrypts the marshalled key material for save().
func (f *KeyFile) sealBytes(plaintext []byte) ([]byte, error) {
	aead, err := sealAEAD(f.seal)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// A fresh nonce every save is required: AES-GCM loses all confidentiality
	// if one key ever reuses a nonce, and this file is rewritten on every
	// epoch advance and every pairing.
	ciphertext := aead.Seal(nil, nonce, plaintext, sealedAAD(FileVersion, sealedAlgorithm))
	return json.MarshalIndent(envelope{
		Version:    FileVersion,
		Sealed:     sealedAlgorithm,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, "", "  ")
}

// HasPlaintextMaterial reports whether path holds key material this version
// can read without a data key. It is how callers tell an upgraded library --
// one whose keys predate the credential store -- from a fresh one, and it
// answers false for a missing file, an unreadable one, or a sealed one.
func HasPlaintextMaterial(path string) bool {
	sealed, err := IsSealed(path)
	return err == nil && !sealed
}

// HasSealedMaterial reports whether path holds key material that needs a data
// key. Together with HasPlaintextMaterial it lets a caller ask the library
// which store it is already using, rather than inferring it from the process.
func HasSealedMaterial(path string) bool {
	sealed, err := IsSealed(path)
	return err == nil && sealed
}
