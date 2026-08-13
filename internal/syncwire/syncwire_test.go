package syncwire

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/syncstate"
)

func fixtureOperation(replicaID string, sequence int64, payload string, dependencies ...syncstate.OperationRef) syncstate.Operation {
	return syncstate.Operation{
		ReplicaID: replicaID, Sequence: sequence,
		OperationID: fmt.Sprintf("%s:%020d", replicaID, sequence),
		Kind:        "record.update", RecordType: "document", RecordID: "doc_café_東京",
		Payload: []byte(payload), HLC: syncstate.HLC{WallMS: 1_700_000_000_000 + sequence, Logical: 3},
		CreatedAt: "2026-08-13T05:00:00Z", Dependencies: dependencies,
	}
}

func fixtureEnvelope() Envelope {
	return Envelope{
		ProtocolMajor: ProtocolMajor, ProtocolMinor: ProtocolMinor,
		DatabaseID: "db_fixture", SenderReplicaID: "replica-a",
		StateVector: syncstate.Vector{"replica-a": 2, "replica-b": 7, "replica-c": 0},
		Operations: []syncstate.Operation{
			fixtureOperation("replica-a", 1, `{"title":"Café 東京 😀"}`),
			fixtureOperation("replica-a", 2, `{"title":"second"}`, syncstate.OperationRef{ReplicaID: "replica-b", Sequence: 4}),
		},
	}
}

func TestEnvelopeRoundTripIsExact(t *testing.T) {
	t.Parallel()
	envelope := fixtureEnvelope()
	encoded, err := EncodeEnvelope(envelope, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEnvelope(encoded, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.DatabaseID != envelope.DatabaseID || decoded.SenderReplicaID != envelope.SenderReplicaID {
		t.Fatalf("identity did not round trip: %+v", decoded)
	}
	if len(decoded.StateVector) != len(envelope.StateVector) {
		t.Fatalf("state vector = %+v", decoded.StateVector)
	}
	for replicaID, sequence := range envelope.StateVector {
		if decoded.StateVector[replicaID] != sequence {
			t.Fatalf("vector entry %s = %d, want %d", replicaID, decoded.StateVector[replicaID], sequence)
		}
	}
	if len(decoded.Operations) != len(envelope.Operations) {
		t.Fatalf("operations = %d", len(decoded.Operations))
	}
	for index, operation := range decoded.Operations {
		want, _, err := syncstate.NormalizeOperation(envelope.Operations[index])
		if err != nil {
			t.Fatal(err)
		}
		if operation.OperationID != want.OperationID || string(operation.Payload) != string(want.Payload) ||
			operation.RecordID != want.RecordID || operation.HLC != want.HLC ||
			len(operation.Dependencies) != len(want.Dependencies) {
			t.Fatalf("operation %d did not round trip:\n got %+v\nwant %+v", index, operation, want)
		}
	}
}

// One logical envelope must have exactly one byte representation, or two
// replicas that agree on the content will disagree on its hash and signature.
func TestEncodingIsCanonical(t *testing.T) {
	t.Parallel()
	envelope := fixtureEnvelope()
	first, err := EncodeEnvelope(envelope, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 20; run++ {
		// Rebuilding the map each time gives Go's randomized iteration order a
		// chance to change the output if the encoder depended on it.
		shuffled := Envelope{
			ProtocolMajor: envelope.ProtocolMajor, ProtocolMinor: envelope.ProtocolMinor,
			DatabaseID: envelope.DatabaseID, SenderReplicaID: envelope.SenderReplicaID,
			StateVector: syncstate.Vector{}, Operations: envelope.Operations,
		}
		for replicaID, sequence := range envelope.StateVector {
			shuffled.StateVector[replicaID] = sequence
		}
		again, err := EncodeEnvelope(shuffled, Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("run %d produced different bytes for the same envelope", run)
		}
	}

	// A dependency list supplied in the other order must encode identically,
	// because normalization sorts it and the encoding follows.
	unsorted := fixtureEnvelope()
	unsorted.Operations[1].Dependencies = []syncstate.OperationRef{
		{ReplicaID: "replica-c", Sequence: 1}, {ReplicaID: "replica-b", Sequence: 4},
	}
	sorted := fixtureEnvelope()
	sorted.Operations[1].Dependencies = []syncstate.OperationRef{
		{ReplicaID: "replica-b", Sequence: 4}, {ReplicaID: "replica-c", Sequence: 1},
	}
	left, err := EncodeEnvelope(unsorted, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	right, err := EncodeEnvelope(sorted, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(left, right) {
		t.Fatal("dependency order changed the canonical bytes")
	}
}

func TestNonCanonicalBytesAreRefused(t *testing.T) {
	t.Parallel()
	encoded, err := EncodeOperations(fixtureEnvelope().Operations, Limits{})
	if err != nil {
		t.Fatal(err)
	}

	// A non-minimal varint decodes to the same number but is not what an
	// encoder produces. Accepting it would mean two byte strings, two hashes,
	// and two signatures for one logical block.
	nonMinimal := append([]byte(nil), encoded[:len(OperationsMagic)]...)
	nonMinimal = append(nonMinimal, 0x82, 0x00) // 2, spelled in two bytes
	nonMinimal = append(nonMinimal, encoded[len(OperationsMagic)+1:]...)
	if _, err := DecodeOperations(nonMinimal, Limits{}); !errors.Is(err, ErrNotCanonical) {
		t.Fatalf("non-minimal varint: %v", err)
	}

	trailing := append(append([]byte(nil), encoded...), 0x00)
	if _, err := DecodeOperations(trailing, Limits{}); !errors.Is(err, ErrNotCanonical) {
		t.Fatalf("trailing byte: %v", err)
	}

	wrongMagic := append([]byte(nil), encoded...)
	copy(wrongMagic, "XXXX")
	if _, err := DecodeOperations(wrongMagic, Limits{}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("wrong magic: %v", err)
	}

	for length := 0; length < len(encoded); length++ {
		if _, err := DecodeOperations(encoded[:length], Limits{}); err == nil {
			t.Fatalf("truncation to %d bytes was accepted", length)
		}
	}

	// Vector entries must be sorted and unique on the wire, so an envelope
	// cannot be re-spelled by listing replicas in another order.
	envelopeBytes, err := EncodeEnvelope(fixtureEnvelope(), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	swapped := bytes.Replace(envelopeBytes,
		[]byte("\treplica-a\x02\treplica-b"), []byte("\treplica-b\x02\treplica-a"), 1)
	if !bytes.Equal(swapped, envelopeBytes) {
		if _, err := DecodeEnvelope(swapped, Limits{}); !errors.Is(err, ErrNotCanonical) {
			t.Fatalf("unsorted vector entries: %v", err)
		}
	}
}

func TestDecodingIsBoundedBeforeItAllocates(t *testing.T) {
	t.Parallel()
	// A count field claiming four billion operations must be refused by the
	// bound, not by running out of memory.
	hostile := append([]byte(nil), OperationsMagic...)
	hostile = appendUvarint(hostile, 1<<32)
	if _, err := DecodeOperations(hostile, Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("operation count: %v", err)
	}

	oversizedIdentifier := append([]byte(nil), OperationsMagic...)
	oversizedIdentifier = appendUvarint(oversizedIdentifier, 1)
	oversizedIdentifier = appendUvarint(oversizedIdentifier, 1<<30)
	if _, err := DecodeOperations(oversizedIdentifier, Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("identifier length: %v", err)
	}

	if _, err := EncodeOperations(make([]syncstate.Operation, DefaultLimits().MaxOperations+1), Limits{}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatal("an oversized operation list was accepted")
	}
}

func TestDeterministicCompressionAndBoundedExpansion(t *testing.T) {
	t.Parallel()
	// Realistic envelope-shaped bytes: repetitive structure with distinct
	// payloads, which is what an operation block actually looks like. A wholly
	// repetitive fixture would compress past the ratio bound and fail the
	// round trip for a reason that has nothing to do with determinism.
	envelope := fixtureEnvelope()
	for index := 0; index < 200; index++ {
		envelope.Operations = append(envelope.Operations,
			fixtureOperation("replica-a", int64(index+3), fmt.Sprintf(`{"n":%d,"note":%q}`, index, strings.Repeat("x", index%37))))
	}
	value, err := EncodeEnvelope(envelope, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := Compress(value)
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 5; run++ {
		again, err := Compress(value)
		if err != nil || !bytes.Equal(first, again) {
			t.Fatalf("run %d is not deterministic: %v", run, err)
		}
	}
	decompressed, err := Decompress(first, Limits{})
	if err != nil || !bytes.Equal(decompressed, value) {
		t.Fatalf("round trip: %v", err)
	}

	// A decompression bomb: a few kilobytes that expand to far more than the
	// ratio allows. The ratio catches it long before the absolute ceiling.
	bomb, err := Compress(make([]byte, 8<<20))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decompress(bomb, Limits{MaxExpansionRatio: 4}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expansion ratio: %v", err)
	}
	if _, err := Decompress(bomb, Limits{MaxEncodedBytes: 1024, MaxExpansionRatio: 1 << 20}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("absolute ceiling: %v", err)
	}
	if _, err := Decompress(append(append([]byte(nil), first...), first...), Limits{}); !errors.Is(err, ErrNotCanonical) {
		t.Fatalf("two gzip members: %v", err)
	}
	if _, err := Decompress([]byte("not gzip at all"), Limits{}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("not gzip: %v", err)
	}
}

func sealedFixture(t *testing.T) (*MemoryKeyRing, *MemorySigner, *MemoryVerifier, []byte, []byte) {
	t.Helper()
	keys, err := NewMemoryKeyRing("key_fixture")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewMemorySigner()
	if err != nil {
		t.Fatal(err)
	}
	verifier := NewMemoryVerifier()
	verifier.Enroll(signer.Public())
	plaintext, err := EncodeEnvelope(fixtureEnvelope(), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := Seal(keys, signer, KindEnvelope, "", plaintext, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return keys, signer, verifier, plaintext, artifact
}

func TestSealAndOpenRoundTrip(t *testing.T) {
	t.Parallel()
	keys, signer, verifier, plaintext, artifact := sealedFixture(t)

	header, opened, err := Open(keys, verifier, artifact, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatal("plaintext did not survive the round trip")
	}
	if header.Kind != KindEnvelope || header.SignerKeyID != signer.SignerKeyID() || header.Epoch != 1 {
		t.Fatalf("header = %+v", header)
	}

	// Nothing recognizable may appear in the artifact. The record id, the
	// title, the database id, and the replica names are all inside the
	// ciphertext, and a carrier can see none of them.
	for _, secret := range []string{"doc_café_東京", "Café 東京 😀", "db_fixture", "replica-b", "record.update"} {
		if bytes.Contains(artifact, []byte(secret)) {
			t.Fatalf("%q is visible in the sealed artifact", secret)
		}
	}
	// What *is* visible is the routing tuple, and only that.
	if !bytes.Contains(artifact, []byte("envelope")) || !bytes.Contains(artifact, []byte("key_fixture")) {
		t.Fatal("the routing tuple should be readable by a carrier")
	}
}

// Two seals of identical plaintext must differ, because each derives its own
// key from a fresh salt. That is what makes the fixed nonce safe.
func TestEveryArtifactGetsItsOwnKey(t *testing.T) {
	t.Parallel()
	keys, signer, verifier, plaintext, first := sealedFixture(t)
	second, err := Seal(keys, signer, KindEnvelope, "", plaintext, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two seals of the same plaintext produced identical artifacts")
	}
	firstHeader, _, _ := Open(keys, verifier, first, Limits{})
	secondHeader, _, _ := Open(keys, verifier, second, Limits{})
	if bytes.Equal(firstHeader.Salt, secondHeader.Salt) {
		t.Fatal("two artifacts shared a salt, so they shared a key and a nonce")
	}
	if _, opened, err := Open(keys, verifier, second, Limits{}); err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("the second artifact did not open: %v", err)
	}

	// Distinct salts must produce distinct keys.
	group, _ := keys.Current()
	firstBytes, _ := EncodeHeader(firstHeader)
	secondBytes, _ := EncodeHeader(secondHeader)
	firstKey, err := deriveArtifactKey(group, firstBytes)
	if err != nil {
		t.Fatal(err)
	}
	secondKey, err := deriveArtifactKey(group, secondBytes)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(firstKey, secondKey) {
		t.Fatal("two headers derived the same artifact key")
	}
}

func TestEveryTamperingIsRefused(t *testing.T) {
	t.Parallel()
	keys, _, verifier, _, artifact := sealedFixture(t)

	// Flipping any single bit anywhere must be caught, by the signature if it
	// lands in the covered bytes and by the framing otherwise. Nothing may
	// decode to a different plaintext without an error.
	for position := 0; position < len(artifact); position++ {
		tampered := append([]byte(nil), artifact...)
		tampered[position] ^= 0x01
		if _, _, err := Open(keys, verifier, tampered, Limits{}); err == nil {
			t.Fatalf("a flipped bit at offset %d was accepted", position)
		}
	}

	truncations := []int{0, 4, 20, len(artifact) - 1, len(artifact) - SignatureBytes}
	for _, length := range truncations {
		if length < 0 {
			continue
		}
		if _, _, err := Open(keys, verifier, artifact[:length], Limits{}); err == nil {
			t.Fatalf("truncation to %d bytes was accepted", length)
		}
	}
	if _, _, err := Open(keys, verifier, append(append([]byte(nil), artifact...), 0x00), Limits{}); !errors.Is(err, ErrNotCanonical) {
		t.Fatal("a trailing byte was accepted")
	}
}

func TestWrongKeyWrongSignerAndRevokedEpoch(t *testing.T) {
	t.Parallel()
	keys, signer, verifier, plaintext, artifact := sealedFixture(t)

	// A different group entirely: right format, wrong secret.
	otherKeys, err := NewMemoryKeyRing("key_fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(otherKeys, verifier, artifact, Limits{}); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("wrong group key: %v", err)
	}

	// A signer nobody enrolled. This is distinct from a bad signature: one is
	// a stranger, the other is a forgery.
	strangerSigner, err := NewMemorySigner()
	if err != nil {
		t.Fatal(err)
	}
	stranger, err := Seal(keys, strangerSigner, KindEnvelope, "", plaintext, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(keys, verifier, stranger, Limits{}); !errors.Is(err, ErrUnknownSigner) {
		t.Fatalf("unenrolled signer: %v", err)
	}
	// Enrolling then revoking the replica has the same effect, which is what
	// makes revocation meaningful at this layer.
	keyID := verifier.Enroll(strangerSigner.Public())
	if _, _, err := Open(keys, verifier, stranger, Limits{}); err != nil {
		t.Fatalf("an enrolled signer should verify: %v", err)
	}
	verifier.Revoke(keyID)
	if _, _, err := Open(keys, verifier, stranger, Limits{}); !errors.Is(err, ErrUnknownSigner) {
		t.Fatalf("revoked signer: %v", err)
	}

	// Advancing the epoch is what a compromise does: the new artifact is
	// unreadable to anyone holding only the old key, and the old artifact
	// stays readable until the epoch is explicitly retired.
	if _, err := keys.AdvanceEpoch(); err != nil {
		t.Fatal(err)
	}
	next, err := Seal(keys, signer, KindEnvelope, "", plaintext, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	nextHeader, _, err := Open(keys, verifier, next, Limits{})
	if err != nil || nextHeader.Epoch != 2 {
		t.Fatalf("post-revocation artifact: %+v %v", nextHeader, err)
	}
	if _, _, err := Open(keys, verifier, artifact, Limits{}); err != nil {
		t.Fatalf("history must stay readable after an epoch advance: %v", err)
	}
	keys.RetireEpoch(1)
	if _, _, err := Open(keys, verifier, artifact, Limits{}); !errors.Is(err, ErrRevokedEpoch) {
		t.Fatalf("retired epoch: %v", err)
	}
	if _, _, err := Open(keys, verifier, next, Limits{}); err != nil {
		t.Fatalf("retiring an old epoch must not affect the current one: %v", err)
	}
}

// A header that decodes but was not the one authenticated must fail, and it
// must fail because of the cryptography rather than a field comparison.
func TestHeaderIsBoundToTheCiphertext(t *testing.T) {
	t.Parallel()
	keys, signer, verifier, plaintext, _ := sealedFixture(t)
	group, err := keys.Current()
	if err != nil {
		t.Fatal(err)
	}
	salt := bytes.Repeat([]byte{0x11}, SaltBytes)
	header := Header{
		ProtocolMajor: ProtocolMajor, ProtocolMinor: ProtocolMinor, Kind: KindEnvelope,
		KeyID: group.KeyID, Epoch: group.Epoch, SignerKeyID: signer.SignerKeyID(),
		Salt: salt, CiphertextLength: int64(len(plaintext) + TagBytes),
	}
	headerBytes, err := EncodeHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := newAEAD(group, headerBytes)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := aead.Seal(nil, artifactNonce, plaintext, headerBytes)

	// Re-sign with a *different* header claiming another artifact kind. The
	// signature is valid, so only the AEAD's associated data catches it.
	forged := header
	forged.Kind = KindSnapshot
	forgedBytes, err := EncodeHeader(forged)
	if err != nil {
		t.Fatal(err)
	}
	artifact := append([]byte(nil), ArtifactMagic...)
	artifact = appendUvarint(artifact, uint64(len(forgedBytes)))
	artifact = append(artifact, forgedBytes...)
	artifact = appendUvarint(artifact, uint64(len(ciphertext)))
	artifact = append(artifact, ciphertext...)
	artifact = append(artifact, signer.Sign(signingMessage(forgedBytes, ciphertext))...)

	if _, _, err := Open(keys, verifier, artifact, Limits{}); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("a substituted header with a valid signature was accepted: %v", err)
	}
}

func TestSenderBinding(t *testing.T) {
	t.Parallel()
	envelope := fixtureEnvelope()
	header := Header{Kind: KindEnvelope}
	if err := BindEnvelopeSender(header, envelope, "replica-a"); err != nil {
		t.Fatalf("a matching sender should bind: %v", err)
	}
	if err := BindEnvelopeSender(header, envelope, "replica-b"); !errors.Is(err, ErrSenderMismatch) {
		t.Fatalf("a relayed envelope claiming another author: %v", err)
	}
	mixed := fixtureEnvelope()
	mixed.Operations = append(mixed.Operations, fixtureOperation("replica-b", 1, `{"x":1}`))
	if err := BindEnvelopeSender(header, mixed, "replica-a"); !errors.Is(err, ErrSenderMismatch) {
		t.Fatal("an envelope carrying another replica's operations was accepted")
	}
	if err := BindEnvelopeSender(Header{Kind: KindObject}, envelope, "replica-a"); !errors.Is(err, ErrMalformed) {
		t.Fatal("only an envelope artifact carries an envelope")
	}
}

// A carrier that needs a file name must not learn which content a library
// holds from that name.
func TestRoutingNamesAreBlinded(t *testing.T) {
	t.Parallel()
	keys, err := NewMemoryKeyRing("key_routing")
	if err != nil {
		t.Fatal(err)
	}
	group, err := keys.Current()
	if err != nil {
		t.Fatal(err)
	}
	contentAddress := hex.EncodeToString(sha256.New().Sum(nil))
	name := RoutingName(group, KindObject, contentAddress)
	if name == "" || strings.Contains(name, contentAddress) || strings.Contains(contentAddress, name) {
		t.Fatalf("routing name %q leaks the content address", name)
	}
	if RoutingName(group, KindObject, contentAddress) != name {
		t.Fatal("routing names must be stable, or a peer could not find the object")
	}
	if RoutingName(group, KindManifest, contentAddress) == name {
		t.Fatal("two artifact kinds for one content address share a routing name")
	}

	// A different group produces a different name for the same content, so two
	// libraries sharing a folder cannot tell they hold the same file.
	otherKeys, err := NewMemoryKeyRing("key_routing")
	if err != nil {
		t.Fatal(err)
	}
	otherGroup, err := otherKeys.Current()
	if err != nil {
		t.Fatal(err)
	}
	if RoutingName(otherGroup, KindObject, contentAddress) == name {
		t.Fatal("the routing blind is not keyed to the group")
	}
}

func TestDeathCertificateSigning(t *testing.T) {
	t.Parallel()
	signer, err := NewMemorySigner()
	if err != nil {
		t.Fatal(err)
	}
	verifier := NewMemoryVerifier()
	keyID := verifier.Enroll(signer.Public())

	signature := SignDeathCertificate(signer, "doc_one", "replica-a", 42)
	if err := VerifyDeathCertificate(verifier, keyID, signature, "doc_one", "replica-a", 42); err != nil {
		t.Fatalf("a genuine certificate did not verify: %v", err)
	}
	// Every field is bound: a certificate for one document must not authorize
	// the permanent deletion of another.
	for _, wrong := range []struct {
		document, replica string
		sequence          int64
	}{
		{"doc_two", "replica-a", 42}, {"doc_one", "replica-b", 42}, {"doc_one", "replica-a", 43},
	} {
		if err := VerifyDeathCertificate(verifier, keyID, signature, wrong.document, wrong.replica, wrong.sequence); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("a certificate verified for %+v", wrong)
		}
	}
	if err := VerifyDeathCertificate(verifier, "unknown", signature, "doc_one", "replica-a", 42); !errors.Is(err, ErrUnknownSigner) {
		t.Fatal("an unenrolled signer was accepted")
	}
	if err := VerifyDeathCertificate(verifier, keyID, "not-hex", "doc_one", "replica-a", 42); !errors.Is(err, ErrBadSignature) {
		t.Fatal("a malformed signature was accepted")
	}
}

// Known-answer tests. These prove the primitives are wired up correctly rather
// than merely self-consistent: a round trip against my own code would pass even
// if the key were the plaintext.
func TestKnownAnswerVectors(t *testing.T) {
	t.Parallel()

	// RFC 8032 Ed25519 test vector 1.
	t.Run("ed25519-rfc8032", func(t *testing.T) {
		seed, _ := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
		wantPublic := "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"
		wantSignature := "e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b"
		private := ed25519.NewKeyFromSeed(seed)
		public := private.Public().(ed25519.PublicKey)
		if hex.EncodeToString(public) != wantPublic {
			t.Fatalf("public key = %s", hex.EncodeToString(public))
		}
		signature := ed25519.Sign(private, nil)
		if hex.EncodeToString(signature) != wantSignature {
			t.Fatalf("signature = %s", hex.EncodeToString(signature))
		}
		if !ed25519.Verify(public, nil, signature) {
			t.Fatal("the vector's own signature did not verify")
		}
	})

	// NIST GCM test case 14 (AES-256, 96-bit IV, empty plaintext, no AAD).
	t.Run("aes-256-gcm-nist", func(t *testing.T) {
		key := make([]byte, 32)
		nonce := make([]byte, 12)
		block, err := aes.NewCipher(key)
		if err != nil {
			t.Fatal(err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			t.Fatal(err)
		}
		got := hex.EncodeToString(aead.Seal(nil, nonce, nil, nil))
		if got != "530f8afbc74536b9a963b4f1c4cb738b" {
			t.Fatalf("GCM tag = %s", got)
		}
	})

	// RFC 5869 HKDF-SHA256 test case 1.
	t.Run("hkdf-sha256-rfc5869", func(t *testing.T) {
		secret, _ := hex.DecodeString("0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b")
		salt, _ := hex.DecodeString("000102030405060708090a0b0c")
		info, _ := hex.DecodeString("f0f1f2f3f4f5f6f7f8f9")
		key, err := hkdfKeyForTest(secret, salt, string(info), 42)
		if err != nil {
			t.Fatal(err)
		}
		want := "3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865"
		if hex.EncodeToString(key) != want {
			t.Fatalf("HKDF output = %s", hex.EncodeToString(key))
		}
	})
}

func TestSealedArtifactsHaveStableStructure(t *testing.T) {
	t.Parallel()
	// The header layout is a wire contract. This pins the field order by
	// building a header from known values and comparing the exact bytes, so a
	// reordering shows up here rather than as an interoperability failure.
	header := Header{
		ProtocolMajor: 1, ProtocolMinor: 0, Kind: KindObject,
		KeyID: "key_a", Epoch: 3, SignerKeyID: "signer_b", RoutingName: "route_c",
		Salt: bytes.Repeat([]byte{0xAB}, SaltBytes), CiphertextLength: 1234,
	}
	encoded, err := EncodeHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	// The salt is spelled out by repetition rather than typed, so a miscount in
	// the literal cannot masquerade as a layout change.
	want := "0100066f626a656374056b65795f6103087369676e65725f6207726f7574655f6320" +
		strings.Repeat("ab", SaltBytes) + "d209"
	if hex.EncodeToString(encoded) != want {
		t.Fatalf("header layout drift\n got: %s\nwant: %s", hex.EncodeToString(encoded), want)
	}
	decoded, err := DecodeHeader(encoded, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Kind != header.Kind || decoded.KeyID != header.KeyID || decoded.Epoch != header.Epoch ||
		decoded.SignerKeyID != header.SignerKeyID || decoded.RoutingName != header.RoutingName ||
		decoded.CiphertextLength != header.CiphertextLength || !bytes.Equal(decoded.Salt, header.Salt) {
		t.Fatalf("header did not round trip: %+v", decoded)
	}
}

func TestRandomizedRoundTrips(t *testing.T) {
	t.Parallel()
	keys, signer, verifier, _, _ := sealedFixture(t)
	random := rand.New(rand.NewSource(9))
	for seed := 0; seed < 200; seed++ {
		envelope := Envelope{
			ProtocolMajor: ProtocolMajor, ProtocolMinor: ProtocolMinor,
			DatabaseID: fmt.Sprintf("db_%d", seed), SenderReplicaID: "replica-a",
			StateVector: syncstate.Vector{"replica-a": int64(seed)},
		}
		for index := 0; index < random.Intn(6); index++ {
			envelope.Operations = append(envelope.Operations,
				fixtureOperation("replica-a", int64(index+1), fmt.Sprintf(`{"n":%d,"s":%q}`, seed, strings.Repeat("é", random.Intn(40)))))
		}
		plaintext, err := EncodeEnvelope(envelope, Limits{})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		artifact, err := Seal(keys, signer, KindEnvelope, "", plaintext, Limits{})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		_, opened, err := Open(keys, verifier, artifact, Limits{})
		if err != nil || !bytes.Equal(opened, plaintext) {
			t.Fatalf("seed %d did not round trip: %v", seed, err)
		}
		decoded, err := DecodeEnvelope(opened, Limits{})
		if err != nil || decoded.DatabaseID != envelope.DatabaseID || len(decoded.Operations) != len(envelope.Operations) {
			t.Fatalf("seed %d: %v", seed, err)
		}
	}
}

func FuzzOpenNeverPanics(f *testing.F) {
	keys, signer, verifier, plaintext, artifact := sealedFixture(&testing.T{})
	_ = signer
	_ = plaintext
	f.Add(artifact)
	f.Add([]byte(ArtifactMagic))
	f.Fuzz(func(t *testing.T, value []byte) {
		_, _, _ = Open(keys, verifier, value, Limits{MaxEncodedBytes: 1 << 20})
	})
}

func FuzzDecodeEnvelopeNeverPanics(f *testing.F) {
	encoded, _ := EncodeEnvelope(fixtureEnvelope(), Limits{})
	f.Add(encoded)
	f.Add([]byte(EnvelopeMagic))
	f.Fuzz(func(t *testing.T, value []byte) {
		_, _ = DecodeEnvelope(value, Limits{MaxEncodedBytes: 1 << 20})
	})
}
