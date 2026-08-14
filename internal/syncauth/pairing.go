package syncauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Pairing is the first exchange between two replicas, and the only one that
// happens before they trust each other.
//
// The shape is fixed by G13's resolved decision: a short-lived, one-use bundle
// carried out of band, containing no reusable library decryption key in
// displayable text. So the code a user types is a *transfer* secret, not a
// library key. It authorizes exactly one pairing, expires, and is spent
// atomically; the library's group key travels back encrypted under a key
// derived from it, so the key exists in ciphertext on the wire and in no
// human-readable form anywhere.
//
// What this does not claim: the joining side authenticates the inviter only
// through the code. Whoever hands over the code is trusted to be who they say,
// which is why it is short-lived, single-use, and displayed rather than mailed.
// A stronger mutual proof needs a channel this protocol does not have; G18 owns
// how the code is presented.
const (
	// PairingSecretBytes is the entropy behind a typed code. Twenty bytes is
	// 32 base32 characters — long enough that guessing is hopeless against the
	// failure limiter, short enough to read aloud.
	PairingSecretBytes = 20
	// PairingDomain separates every derivation in this exchange.
	PairingDomain = "notrios.pairing.v1"
	// MaxPairingBodyBytes bounds an unauthenticated request. A pairing request
	// is small and fixed; anything larger is not one.
	MaxPairingBodyBytes = 8 << 10
)

var (
	// ErrPairingProof reports a pairing request that does not prove possession
	// of the invitation code.
	ErrPairingProof = errors.New("pairing proof does not verify")
	// ErrPairingBundle reports an unusable wrapped response.
	ErrPairingBundle = errors.New("pairing response could not be opened")
)

var codeEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewPairingSecret generates a transfer secret.
func NewPairingSecret() ([]byte, error) {
	secret := make([]byte, PairingSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// FormatCode renders a secret as the code a person reads or scans: uppercase
// base32 in groups of four, which survives being spoken, typed, and wrapped.
func FormatCode(secret []byte) string {
	encoded := codeEncoding.EncodeToString(secret)
	var grouped strings.Builder
	for index, character := range encoded {
		if index > 0 && index%4 == 0 {
			grouped.WriteByte('-')
		}
		grouped.WriteRune(character)
	}
	return grouped.String()
}

// ParseCode reverses FormatCode, tolerating the ways a person retypes it:
// lower case, missing or extra grouping, and surrounding space.
func ParseCode(code string) ([]byte, error) {
	cleaned := strings.ToUpper(strings.TrimSpace(code))
	cleaned = strings.NewReplacer("-", "", " ", "", "\t", "", "\n", "").Replace(cleaned)
	secret, err := codeEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("%w: not a pairing code", ErrMalformed)
	}
	if len(secret) != PairingSecretBytes {
		return nil, fmt.Errorf("%w: a pairing code is %d bytes", ErrMalformed, PairingSecretBytes)
	}
	return secret, nil
}

// PairingRequest is what a joining replica sends. It is unauthenticated by
// definition — the joiner is not enrolled yet — so every field is treated as a
// claim until the proof is checked.
type PairingRequest struct {
	InvitationID string `json:"invitation_id"`
	DatabaseID   string `json:"database_id"`
	ReplicaID    string `json:"replica_id"`
	PublicKey    string `json:"public_key"`
	SchemaVerion int    `json:"schema_version"`
	Proof        string `json:"proof"`
}

// PairingResponse is what the inviting replica returns once the proof holds.
// The group key is wrapped; nothing in this structure is a secret in the clear.
type PairingResponse struct {
	DatabaseID     string `json:"database_id"`
	ReplicaID      string `json:"replica_id"`
	PublicKey      string `json:"public_key"`
	KeyID          string `json:"key_id"`
	Epoch          uint32 `json:"epoch"`
	WrappedGroup   string `json:"wrapped_group_key"`
	ResponseProof  string `json:"response_proof"`
	SchemaVersion  int    `json:"schema_version"`
	ProtocolMajor  int    `json:"protocol_major"`
	ProtocolMinor  int    `json:"protocol_minor"`
	PublicBaseURL  string `json:"public_base_url,omitempty"`
	InviterKeyID   string `json:"inviter_signer_key_id"`
	InviterReplica string `json:"inviter_replica_id"`
}

// pairingKey derives a purpose-specific key from the transfer secret.
func pairingKey(secret []byte, purpose string) ([]byte, error) {
	return hkdf.Key(sha256.New, secret, nil, PairingDomain+"."+purpose, 32)
}

// ProveRequest binds the joiner's claims to the invitation code.
//
// It is an HMAC rather than a signature because the joiner's signing key is
// exactly what it is asking to have enrolled: signing with it would prove only
// that it holds the key it just sent. The proof that matters is possession of
// the code.
func ProveRequest(secret []byte, request PairingRequest) (string, error) {
	key, err := pairingKey(secret, "request")
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	writeField(mac, request.InvitationID)
	writeField(mac, request.DatabaseID)
	writeField(mac, request.ReplicaID)
	writeField(mac, request.PublicKey)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifyRequestProof checks the joiner's proof in constant time.
func VerifyRequestProof(secret []byte, request PairingRequest) error {
	expected, err := ProveRequest(secret, request)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(request.Proof)) != 1 {
		return ErrPairingProof
	}
	return nil
}

// ProveResponse lets the joiner check that the answer came from something
// holding the same code, rather than from whoever answered the address it
// dialled. It is the only assurance the joiner gets about the inviter, and it
// is exactly as strong as the code's confidentiality.
func ProveResponse(secret []byte, response PairingResponse) (string, error) {
	key, err := pairingKey(secret, "response")
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	writeField(mac, response.DatabaseID)
	writeField(mac, response.ReplicaID)
	writeField(mac, response.PublicKey)
	writeField(mac, response.KeyID)
	writeField(mac, response.WrappedGroup)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifyResponseProof checks the inviter's proof in constant time.
func VerifyResponseProof(secret []byte, response PairingResponse) error {
	expected, err := ProveResponse(secret, response)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(response.ResponseProof)) != 1 {
		return ErrPairingProof
	}
	return nil
}

// WrapGroupKey seals the library's group key under the transfer secret.
//
// A fresh random nonce is used rather than a fixed one: unlike a G9 artifact,
// whose key is unique per artifact by construction, this key is derived from a
// secret that seals exactly one payload but may be attempted more than once.
func WrapGroupKey(secret, groupKey []byte) (string, error) {
	key, err := pairingKey(secret, "wrap")
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, groupKey, []byte(PairingDomain))
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// UnwrapGroupKey opens what WrapGroupKey sealed.
func UnwrapGroupKey(secret []byte, wrapped string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(wrapped)
	if err != nil {
		return nil, fmt.Errorf("%w: not base64", ErrPairingBundle)
	}
	key, err := pairingKey(secret, "wrap")
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < aead.NonceSize() {
		return nil, fmt.Errorf("%w: truncated", ErrPairingBundle)
	}
	plaintext, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte(PairingDomain))
	if err != nil {
		return nil, fmt.Errorf("%w: wrong code or altered response", ErrPairingBundle)
	}
	return plaintext, nil
}

// DecodePublicKey parses an advertised Ed25519 public key.
func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: not an Ed25519 public key", ErrMalformed)
	}
	return ed25519.PublicKey(raw), nil
}

// EncodePublicKey renders a public key for transport.
func EncodePublicKey(public ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(public)
}

// writeField length-prefixes a field so no rearrangement of two fields
// produces the same MAC input.
func writeField(mac interface{ Write([]byte) (int, error) }, value string) {
	_, _ = mac.Write([]byte(fmt.Sprintf("%d:", len(value))))
	_, _ = mac.Write([]byte(value))
}
