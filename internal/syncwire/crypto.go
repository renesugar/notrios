package syncwire

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
)

// Every primitive here comes from the Go standard library, which is what G0's
// selection rule asked for: use a maintained implementation, do not invent one.
// AES-256-GCM is the authenticated encryption, HKDF-SHA256 the key derivation,
// Ed25519 the signature, and HMAC-SHA256 the routing-name blind. G9 therefore
// adds no dependency at all, and the license inventory it was asked for has one
// line: the Go standard library, BSD-3-Clause.

// Domain separation strings. Every one of them is a distinct purpose for the
// same key material, and mixing two would let a value produced for one purpose
// be accepted as the other.
const (
	domainArtifactKey = "notrios.artifact-key.v1"
	domainArtifactSig = "notrios.artifact-signature.v1"
	domainRoutingName = "notrios.routing-name.v1"
	domainDeathCert   = "notrios.death-certificate.v1"
	domainRoutingKey  = "notrios.routing-key.v1"
	domainCarrierName = "notrios.carrier-name.v1"
)

// Sizes fixed by the chosen primitives.
const (
	GroupKeyBytes  = 32
	SaltBytes      = 32
	NonceBytes     = 12
	TagBytes       = 16
	SignatureBytes = ed25519.SignatureSize
)

// ArtifactKind names what an artifact carries. It is visible to the carrier
// because a transport has to know what it is moving; nothing else about the
// content is.
type ArtifactKind string

const (
	KindEnvelope ArtifactKind = "envelope"
	KindObject   ArtifactKind = "object"
	KindManifest ArtifactKind = "manifest"
	KindSnapshot ArtifactKind = "snapshot"
)

var artifactKinds = map[ArtifactKind]bool{
	KindEnvelope: true, KindObject: true, KindManifest: true, KindSnapshot: true,
}

var (
	// ErrNoKey reports that no enrolled key matches an artifact's header.
	ErrNoKey = errors.New("no enrolled key matches this artifact")
	// ErrRevokedEpoch reports an artifact encrypted under a retired epoch.
	ErrRevokedEpoch = errors.New("artifact epoch has been revoked")
	// ErrBadSignature reports an artifact whose signature does not verify.
	// It is checked before decryption, so tampered bytes are never processed.
	ErrBadSignature = errors.New("artifact signature does not verify")
	// ErrUnknownSigner reports a signature from a key this replica has not
	// enrolled. It is distinct from a bad signature: one is a stranger, the
	// other is a forgery or corruption.
	ErrUnknownSigner = errors.New("artifact signer is not enrolled")
	// ErrDecrypt reports authenticated decryption failure — the ciphertext,
	// the header bound to it, or the key is wrong. Which one is deliberately
	// not distinguished.
	ErrDecrypt = errors.New("artifact did not decrypt and authenticate")
	// ErrSenderMismatch reports an envelope whose declared sender is not the
	// replica whose key signed it.
	ErrSenderMismatch = errors.New("envelope sender does not match its signer")
)

// GroupKey is the symmetric secret shared by the replicas of one database, in
// one epoch. G9 defines the interface and an in-memory provider; binding it to
// an operating-system secret store is v0.8's.
type GroupKey struct {
	KeyID string
	Epoch uint32
	Key   [GroupKeyBytes]byte
}

// KeyRing supplies group keys. Encryption always uses the current epoch;
// decryption looks one up by the identifiers in the visible header.
type KeyRing interface {
	Current() (GroupKey, error)
	Lookup(keyID string, epoch uint32) (GroupKey, error)
}

// Signer produces per-replica signatures. Its key identifies the replica to
// every peer, which is why a signature is not a substitute for encryption:
// it says who sent an artifact, not who may read it.
type Signer interface {
	SignerKeyID() string
	Sign(message []byte) []byte
}

// Verifier resolves a signer key id to the public key enrolled for it.
type Verifier interface {
	PublicKey(signerKeyID string) (ed25519.PublicKey, bool)
}

// Header is the only part of an artifact a carrier can read. G0 fixed its
// contents: protocol and key identifiers, the artifact kind, a bounded length,
// and a routing address where one is needed. Record ids, titles, MIME types,
// filenames, and state vectors are all inside the ciphertext.
type Header struct {
	ProtocolMajor int
	ProtocolMinor int
	Kind          ArtifactKind
	KeyID         string
	Epoch         uint32
	SignerKeyID   string
	// RoutingName addresses an artifact on a carrier that needs a name. For an
	// object it is a keyed blind of the content hash, never the hash itself:
	// a shared folder's file names would otherwise tell anyone who could list
	// the directory exactly which content the library holds.
	RoutingName string
	// Salt makes every artifact's encryption key unique. That is what allows a
	// fixed nonce below.
	Salt []byte
	// CiphertextLength lets a reader bound its allocation before reading, and
	// is bound into the authenticated data so it cannot be edited in transit.
	CiphertextLength int64
}

// EncodeHeader produces the canonical header bytes. They are what the AEAD
// authenticates and what the signature covers, so this encoding is as
// load-bearing as the ciphertext.
func EncodeHeader(header Header) ([]byte, error) {
	if !artifactKinds[header.Kind] {
		return nil, fmt.Errorf("%w: unknown artifact kind %q", ErrMalformed, header.Kind)
	}
	if len(header.Salt) != SaltBytes {
		return nil, fmt.Errorf("%w: salt must be %d bytes", ErrMalformed, SaltBytes)
	}
	if header.KeyID == "" || header.SignerKeyID == "" {
		return nil, fmt.Errorf("%w: a header names its encryption and signing keys", ErrMalformed)
	}
	if header.CiphertextLength < 0 {
		return nil, fmt.Errorf("%w: negative ciphertext length", ErrMalformed)
	}
	output := appendUvarint(nil, uint64(header.ProtocolMajor))
	output = appendUvarint(output, uint64(header.ProtocolMinor))
	output = appendString(output, string(header.Kind))
	output = appendString(output, header.KeyID)
	output = appendUvarint(output, uint64(header.Epoch))
	output = appendString(output, header.SignerKeyID)
	output = appendString(output, header.RoutingName)
	output = appendUvarint(output, uint64(len(header.Salt)))
	output = append(output, header.Salt...)
	output = appendUvarint(output, uint64(header.CiphertextLength))
	return output, nil
}

// DecodeHeader reverses EncodeHeader, refusing anything a header encoder would
// not have produced.
func DecodeHeader(encoded []byte, requested Limits) (Header, error) {
	limits := normalizeLimits(requested)
	r := &reader{value: encoded}
	major, err := r.uvarint()
	if err != nil {
		return Header{}, err
	}
	minor, err := r.uvarint()
	if err != nil {
		return Header{}, err
	}
	header := Header{ProtocolMajor: int(major), ProtocolMinor: int(minor)}
	kind, err := r.boundedString(limits.MaxIdentifierBytes, "artifact kind")
	if err != nil {
		return Header{}, err
	}
	header.Kind = ArtifactKind(kind)
	if !artifactKinds[header.Kind] {
		return Header{}, fmt.Errorf("%w: unknown artifact kind %q", ErrMalformed, kind)
	}
	if header.KeyID, err = r.boundedString(limits.MaxIdentifierBytes, "key id"); err != nil {
		return Header{}, err
	}
	epoch, err := r.uvarint()
	if err != nil {
		return Header{}, err
	}
	if epoch > 0xFFFFFFFF {
		return Header{}, fmt.Errorf("%w: epoch out of range", ErrMalformed)
	}
	header.Epoch = uint32(epoch)
	if header.SignerKeyID, err = r.boundedString(limits.MaxIdentifierBytes, "signer key id"); err != nil {
		return Header{}, err
	}
	if header.RoutingName, err = r.boundedString(limits.MaxIdentifierBytes, "routing name"); err != nil {
		return Header{}, err
	}
	saltLength, err := r.uvarint()
	if err != nil {
		return Header{}, err
	}
	if saltLength != SaltBytes {
		return Header{}, fmt.Errorf("%w: salt must be %d bytes", ErrMalformed, SaltBytes)
	}
	salt, err := r.take(int(saltLength))
	if err != nil {
		return Header{}, err
	}
	header.Salt = append([]byte(nil), salt...)
	length, err := r.uvarint()
	if err != nil {
		return Header{}, err
	}
	if int64(length) > limits.MaxEncodedBytes+int64(TagBytes) {
		return Header{}, fmt.Errorf("%w: declared ciphertext is %d bytes", ErrLimitExceeded, length)
	}
	header.CiphertextLength = int64(length)
	if r.remaining() != 0 {
		return Header{}, fmt.Errorf("%w: %d trailing header bytes", ErrNotCanonical, r.remaining())
	}
	return header, nil
}

// deriveArtifactKey produces a key used for exactly one artifact.
//
// The header is mixed into the derivation as well as being the AEAD's
// associated data. That is belt and braces on purpose: the associated data
// makes a modified header fail authentication, and the derivation makes a
// modified header produce a different key, so there is no single check whose
// absence would let a header be edited.
func deriveArtifactKey(group GroupKey, headerBytes []byte) ([]byte, error) {
	salt := headerBytes
	return hkdf.Key(sha256.New, group.Key[:], salt, domainArtifactKey, GroupKeyBytes)
}

// artifactNonce is fixed, and that is safe precisely because it is not the
// thing providing uniqueness. Every artifact derives its own key from a fresh
// 32-byte random salt, so no two artifacts share a (key, nonce) pair even
// though they share a nonce. Deriving a random nonce as well would add nothing
// and would make the encoding one field longer.
var artifactNonce = make([]byte, NonceBytes)

// SealedArtifact is the complete on-the-wire object.
type SealedArtifact struct {
	Header     Header
	Ciphertext []byte
	Signature  []byte
}

// Seal encrypts a plaintext and signs the result.
//
// The order is encrypt-then-sign, so a receiver can reject a forged or
// corrupted artifact by checking one signature over bytes it has not decrypted
// and does not have to allocate a plaintext for.
func Seal(keys KeyRing, signer Signer, kind ArtifactKind, routingName string, plaintext []byte, requested Limits) ([]byte, error) {
	limits := normalizeLimits(requested)
	if int64(len(plaintext)) > limits.MaxEncodedBytes {
		return nil, fmt.Errorf("%w: plaintext is %d bytes", ErrLimitExceeded, len(plaintext))
	}
	group, err := keys.Current()
	if err != nil {
		return nil, err
	}
	salt := make([]byte, SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	header := Header{
		ProtocolMajor: ProtocolMajor, ProtocolMinor: ProtocolMinor, Kind: kind,
		KeyID: group.KeyID, Epoch: group.Epoch, SignerKeyID: signer.SignerKeyID(),
		RoutingName: routingName, Salt: salt,
		CiphertextLength: int64(len(plaintext) + TagBytes),
	}
	headerBytes, err := EncodeHeader(header)
	if err != nil {
		return nil, err
	}
	aead, err := newAEAD(group, headerBytes)
	if err != nil {
		return nil, err
	}
	ciphertext := aead.Seal(nil, artifactNonce, plaintext, headerBytes)
	signature := signer.Sign(signingMessage(headerBytes, ciphertext))
	if len(signature) != SignatureBytes {
		return nil, fmt.Errorf("%w: signer produced %d bytes", ErrBadSignature, len(signature))
	}

	output := append([]byte(nil), ArtifactMagic...)
	output = appendUvarint(output, uint64(len(headerBytes)))
	output = append(output, headerBytes...)
	output = appendUvarint(output, uint64(len(ciphertext)))
	output = append(output, ciphertext...)
	output = append(output, signature...)
	return output, nil
}

// Open verifies an artifact's signature, then decrypts it. Nothing is returned
// unless both succeed, and the signature is checked first so unauthenticated
// bytes never reach the cipher.
func Open(keys KeyRing, verifier Verifier, artifact []byte, requested Limits) (Header, []byte, error) {
	limits := normalizeLimits(requested)
	header, headerBytes, ciphertext, signature, err := parseArtifact(artifact, limits)
	if err != nil {
		return Header{}, nil, err
	}
	publicKey, enrolled := verifier.PublicKey(header.SignerKeyID)
	if !enrolled {
		return Header{}, nil, fmt.Errorf("%w: %s", ErrUnknownSigner, header.SignerKeyID)
	}
	if !ed25519.Verify(publicKey, signingMessage(headerBytes, ciphertext), signature) {
		return Header{}, nil, ErrBadSignature
	}
	group, err := keys.Lookup(header.KeyID, header.Epoch)
	if err != nil {
		return Header{}, nil, err
	}
	aead, err := newAEAD(group, headerBytes)
	if err != nil {
		return Header{}, nil, err
	}
	plaintext, err := aead.Open(nil, artifactNonce, ciphertext, headerBytes)
	if err != nil {
		return Header{}, nil, ErrDecrypt
	}
	return header, plaintext, nil
}

// PeekSignerKeyID reads the signing key identifier from an artifact this
// replica cannot verify. It exists for one purpose: G11's discovery has to be
// able to say "an unenrolled key is publishing here" without pretending to know
// anything else. Nothing is authenticated, nothing is decrypted, and the caller
// must treat the result as a claim rather than an identity.
func PeekSignerKeyID(artifact []byte, requested Limits) (string, error) {
	header, _, _, _, err := parseArtifact(artifact, normalizeLimits(requested))
	if err != nil {
		return "", err
	}
	return header.SignerKeyID, nil
}

func parseArtifact(artifact []byte, limits Limits) (Header, []byte, []byte, []byte, error) {
	if int64(len(artifact)) > limits.MaxEncodedBytes+int64(limits.MaxIdentifierBytes)+1024 {
		return Header{}, nil, nil, nil, fmt.Errorf("%w: artifact is %d bytes", ErrLimitExceeded, len(artifact))
	}
	r := &reader{value: artifact}
	magic, err := r.take(len(ArtifactMagic))
	if err != nil {
		return Header{}, nil, nil, nil, err
	}
	if string(magic) != ArtifactMagic {
		return Header{}, nil, nil, nil, fmt.Errorf("%w: not an %s artifact", ErrMalformed, ArtifactMagic)
	}
	headerLength, err := r.uvarint()
	if err != nil {
		return Header{}, nil, nil, nil, err
	}
	if headerLength > uint64(4*limits.MaxIdentifierBytes+SaltBytes+64) {
		return Header{}, nil, nil, nil, fmt.Errorf("%w: header is %d bytes", ErrLimitExceeded, headerLength)
	}
	headerBytes, err := r.take(int(headerLength))
	if err != nil {
		return Header{}, nil, nil, nil, err
	}
	header, err := DecodeHeader(headerBytes, limits)
	if err != nil {
		return Header{}, nil, nil, nil, err
	}
	ciphertextLength, err := r.uvarint()
	if err != nil {
		return Header{}, nil, nil, nil, err
	}
	if int64(ciphertextLength) != header.CiphertextLength {
		return Header{}, nil, nil, nil, fmt.Errorf("%w: framed length disagrees with the header", ErrNotCanonical)
	}
	ciphertext, err := r.take(int(ciphertextLength))
	if err != nil {
		return Header{}, nil, nil, nil, err
	}
	signature, err := r.take(SignatureBytes)
	if err != nil {
		return Header{}, nil, nil, nil, err
	}
	if r.remaining() != 0 {
		return Header{}, nil, nil, nil, fmt.Errorf("%w: %d trailing artifact bytes", ErrNotCanonical, r.remaining())
	}
	return header, headerBytes, ciphertext, signature, nil
}

func newAEAD(group GroupKey, headerBytes []byte) (cipher.AEAD, error) {
	key, err := deriveArtifactKey(group, headerBytes)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// signingMessage is the exact byte string an Ed25519 signature covers: a domain
// tag, the canonical header, and the ciphertext, each separated so that no
// rearrangement of the parts produces the same message.
func signingMessage(headerBytes, ciphertext []byte) []byte {
	message := make([]byte, 0, len(domainArtifactSig)+len(headerBytes)+len(ciphertext)+16)
	message = append(message, domainArtifactSig...)
	message = appendUvarint(message, uint64(len(headerBytes)))
	message = append(message, headerBytes...)
	message = appendUvarint(message, uint64(len(ciphertext)))
	message = append(message, ciphertext...)
	return message
}

// RoutingName blinds a content address for a carrier that needs a file name.
// G0 froze this: a shared folder's directory listing must not tell anyone who
// can read it which content the library holds, and a plaintext content hash
// would do exactly that to anyone holding a copy of the same file.
func RoutingName(group GroupKey, kind ArtifactKind, contentAddress string) string {
	mac := hmac.New(sha256.New, routingKey(group))
	mac.Write([]byte(domainRoutingName))
	mac.Write([]byte{0})
	mac.Write([]byte(kind))
	mac.Write([]byte{0})
	mac.Write([]byte(contentAddress))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

// CarrierName blinds a path segment for a carrier that addresses artifacts by
// directory and file name. G11 needs it for database and replica identifiers,
// which G0's leakage budget keeps out of a directory listing exactly as it
// keeps content hashes out: a peer identifier in a folder name tells anyone who
// can list the folder how many replicas a library has and which one is writing.
//
// It is a distinct domain from RoutingName because it is a distinct purpose for
// the same key material, and it lowercases nothing: the output is hex, so a
// case-insensitive filesystem cannot fold two different names together.
func CarrierName(group GroupKey, scope, value string) string {
	mac := hmac.New(sha256.New, routingKey(group))
	mac.Write([]byte(domainCarrierName))
	mac.Write([]byte{0})
	mac.Write([]byte(scope))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

func routingKey(group GroupKey) []byte {
	key, err := hkdf.Key(sha256.New, group.Key[:], nil, domainRoutingKey, GroupKeyBytes)
	if err != nil {
		// HKDF with these fixed, valid parameters cannot fail; returning the
		// group key would silently weaken the blind, so this refuses loudly.
		panic("syncwire: routing key derivation failed: " + err.Error())
	}
	return key
}

// BindEnvelopeSender refuses an envelope whose declared sender is not the
// replica whose key signed the artifact. Without it a peer could relay a valid
// envelope while claiming to be its author.
//
// Operation-level replay stays where it already works: G5 treats an exact
// resend as inert and refuses a conflicting one, so this layer does not need a
// second, weaker copy of that rule.
func BindEnvelopeSender(header Header, envelope Envelope, signerReplicaID string) error {
	if header.Kind != KindEnvelope {
		return fmt.Errorf("%w: artifact is a %s", ErrMalformed, header.Kind)
	}
	if envelope.SenderReplicaID != signerReplicaID {
		return fmt.Errorf("%w: envelope says %q, signature says %q", ErrSenderMismatch, envelope.SenderReplicaID, signerReplicaID)
	}
	for _, operation := range envelope.Operations {
		if operation.ReplicaID != signerReplicaID {
			return fmt.Errorf("%w: operation from %q in an envelope signed by %q",
				ErrSenderMismatch, operation.ReplicaID, signerReplicaID)
		}
	}
	return nil
}

// SignDeathCertificate produces the signature G6 recorded as owed by G9. A
// permanent deletion is the one operation that cannot be undone by another
// operation, so it carries proof of who ordered it.
func SignDeathCertificate(signer Signer, documentID, replicaID string, sequence int64) string {
	return hex.EncodeToString(signer.Sign(deathCertificateMessage(documentID, replicaID, sequence)))
}

// VerifyDeathCertificate checks such a signature against an enrolled key.
func VerifyDeathCertificate(verifier Verifier, signerKeyID, signature, documentID, replicaID string, sequence int64) error {
	publicKey, enrolled := verifier.PublicKey(signerKeyID)
	if !enrolled {
		return fmt.Errorf("%w: %s", ErrUnknownSigner, signerKeyID)
	}
	raw, err := hex.DecodeString(signature)
	if err != nil || len(raw) != SignatureBytes {
		return ErrBadSignature
	}
	if !ed25519.Verify(publicKey, deathCertificateMessage(documentID, replicaID, sequence), raw) {
		return ErrBadSignature
	}
	return nil
}

func deathCertificateMessage(documentID, replicaID string, sequence int64) []byte {
	message := append([]byte(nil), domainDeathCert...)
	message = appendString(message, documentID)
	message = appendString(message, replicaID)
	message = appendUvarint(message, uint64(sequence))
	return message
}

// MemoryKeyRing is the G9 test provider. It holds group keys per epoch in
// memory and can retire an epoch, which is what revocation does: a compromised
// replica loses the ability to read anything published afterwards, because the
// remaining peers move to a new epoch and stop accepting the old one.
//
// v0.8 owns the operating-system secret store this stands in for.
type MemoryKeyRing struct {
	keyID   string
	current uint32
	keys    map[uint32][GroupKeyBytes]byte
	retired map[uint32]bool
}

// NewMemoryKeyRing creates a ring holding one epoch.
func NewMemoryKeyRing(keyID string) (*MemoryKeyRing, error) {
	ring := &MemoryKeyRing{
		keyID: keyID, current: 1,
		keys: map[uint32][GroupKeyBytes]byte{}, retired: map[uint32]bool{},
	}
	var key [GroupKeyBytes]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}
	ring.keys[1] = key
	return ring, nil
}

// AdvanceEpoch mints a new key and retires the old one, which is what G0
// required when a replica is revoked: remaining peers must be able to publish
// something the compromised replica cannot read.
//
// The retired key is kept but marked, so artifacts published before the
// revocation remain readable. Losing the ability to read a library's own
// history would be a worse outcome than the compromise.
func (r *MemoryKeyRing) AdvanceEpoch() (GroupKey, error) {
	var key [GroupKeyBytes]byte
	if _, err := rand.Read(key[:]); err != nil {
		return GroupKey{}, err
	}
	r.current++
	r.keys[r.current] = key
	return GroupKey{KeyID: r.keyID, Epoch: r.current, Key: key}, nil
}

// RetireEpoch refuses artifacts published under an epoch. It is separate from
// advancing, because the two answer different questions: advancing decides what
// this replica publishes next, retiring decides what it will still read.
func (r *MemoryKeyRing) RetireEpoch(epoch uint32) { r.retired[epoch] = true }

func (r *MemoryKeyRing) Current() (GroupKey, error) {
	key, found := r.keys[r.current]
	if !found {
		return GroupKey{}, ErrNoKey
	}
	return GroupKey{KeyID: r.keyID, Epoch: r.current, Key: key}, nil
}

func (r *MemoryKeyRing) Lookup(keyID string, epoch uint32) (GroupKey, error) {
	if subtle.ConstantTimeCompare([]byte(keyID), []byte(r.keyID)) != 1 {
		return GroupKey{}, fmt.Errorf("%w: key %q", ErrNoKey, keyID)
	}
	if r.retired[epoch] {
		return GroupKey{}, fmt.Errorf("%w: epoch %d", ErrRevokedEpoch, epoch)
	}
	key, found := r.keys[epoch]
	if !found {
		return GroupKey{}, fmt.Errorf("%w: epoch %d", ErrNoKey, epoch)
	}
	return GroupKey{KeyID: keyID, Epoch: epoch, Key: key}, nil
}

// MemorySigner is the G9 test signer. Its key id is derived from the public key
// so two replicas cannot claim the same identity by choosing the same label.
type MemorySigner struct {
	keyID   string
	private ed25519.PrivateKey
}

// NewMemorySigner generates a replica signing key.
func NewMemorySigner() (*MemorySigner, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &MemorySigner{keyID: SignerKeyID(public), private: private}, nil
}

// SignerKeyID names a public key by its hash rather than by a chosen label.
func SignerKeyID(public ed25519.PublicKey) string {
	digest := sha256.Sum256(append([]byte("notrios.signer.v1"), public...))
	return hex.EncodeToString(digest[:16])
}

func (s *MemorySigner) SignerKeyID() string        { return s.keyID }
func (s *MemorySigner) Sign(message []byte) []byte { return ed25519.Sign(s.private, message) }
func (s *MemorySigner) Public() ed25519.PublicKey  { return s.private.Public().(ed25519.PublicKey) }

// MemoryVerifier holds the enrolled signing keys of known replicas.
type MemoryVerifier struct{ keys map[string]ed25519.PublicKey }

// NewMemoryVerifier creates an empty enrollment set. It is empty on purpose:
// enrolling a peer is a deliberate act, and G13 owns proving possession of the
// key before it happens.
func NewMemoryVerifier() *MemoryVerifier {
	return &MemoryVerifier{keys: map[string]ed25519.PublicKey{}}
}

// Enroll adds a replica's public key.
func (v *MemoryVerifier) Enroll(public ed25519.PublicKey) string {
	keyID := SignerKeyID(public)
	v.keys[keyID] = public
	return keyID
}

// Revoke removes one, after which its artifacts are refused as coming from an
// unknown signer.
func (v *MemoryVerifier) Revoke(signerKeyID string) { delete(v.keys, signerKeyID) }

func (v *MemoryVerifier) PublicKey(signerKeyID string) (ed25519.PublicKey, bool) {
	public, found := v.keys[signerKeyID]
	return public, found
}

var (
	_ KeyRing  = (*MemoryKeyRing)(nil)
	_ Signer   = (*MemorySigner)(nil)
	_ Verifier = (*MemoryVerifier)(nil)
)

// ProtocolMajor and ProtocolMinor mirror the negotiated protocol version so an
// artifact header states which rules produced it.
const (
	ProtocolMajor = 1
	ProtocolMinor = 0
)

// hkdfKeyForTest exposes the exact derivation call the artifact key uses, so a
// published RFC vector can prove it is wired correctly rather than merely
// self-consistent.
func hkdfKeyForTest(secret, salt []byte, info string, length int) ([]byte, error) {
	return hkdf.Key(sha256.New, secret, salt, info, length)
}
