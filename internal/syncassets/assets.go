// Package syncassets implements G8's transport- and storage-neutral rules for
// synchronizing attachment bytes: how an object is divided for transfer, what a
// receiver must verify before those bytes become canonical, and when a replica
// should fetch an object at all rather than merely knowing it exists.
//
// Like the G5 state core, the G6 metadata core, and the G7 body core, it knows
// nothing about SQLite, HTTP, folders, or cryptography. It is given lengths and
// hashes and returns plans and verdicts; the caller owns every byte written.
package syncassets

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// The transfer shape G2 selected after measuring real corpora.
const (
	// WholeObjectThreshold is the size below which an object travels whole.
	// G2 chose it because a small attachment gains nothing from segmentation
	// and pays a manifest for it.
	WholeObjectThreshold = 1 << 20

	// ChunkBytes is the fixed chunk size above that threshold. G2 deferred
	// content-defined chunking (FastCDC) because static corpora contain no
	// repeated binary-edit trace to justify it.
	ChunkBytes = 1 << 20

	// MaxChunks and MaxObjectBytes are the inherited ceilings: 16,384 chunks of
	// one mebibyte each is the 16 GiB resource limit Notrios already enforced
	// before synchronization existed.
	MaxChunks      = 16_384
	MaxObjectBytes = int64(MaxChunks) * ChunkBytes
)

// The bounds a receiver applies to its own fetching. They are about this
// replica's disk and patience, not about the protocol.
const (
	// MaxConcurrentFetches bounds how many objects are in flight at once. A
	// mobile client on a metered link is the constraint this exists for, and
	// v0.8's emulator work is where it will be revisited.
	MaxConcurrentFetches = 4

	// MaxStagingBytes bounds the bytes a replica will hold in partially
	// fetched objects at one time. Exceeding it defers a fetch rather than
	// failing it: the object stays known and unavailable, which is a state the
	// user can see and act on.
	MaxStagingBytes = int64(512) << 20

	// MaxEagerBytes is the budget for automatic fetching of small objects in
	// one pass. Beyond it, everything waits for a pin or an explicit request.
	MaxEagerBytes = int64(64) << 20
)

var (
	// ErrObjectTooLarge reports an object outside the inherited ceiling.
	ErrObjectTooLarge = errors.New("object exceeds the resource size ceiling")
	// ErrManifestInvalid reports a manifest that does not describe its object.
	ErrManifestInvalid = errors.New("chunk manifest is invalid")
	// ErrChunkMismatch reports a chunk whose bytes are not what the manifest
	// named. It is a refusal, never a reason to keep the bytes.
	ErrChunkMismatch = errors.New("chunk does not match its manifest entry")
	// ErrObjectMismatch reports assembled bytes that are not the named object.
	ErrObjectMismatch = errors.New("assembled object does not match its hash")
	// ErrMIMEMismatch reports content whose sniffed type contradicts what the
	// sender advertised.
	ErrMIMEMismatch = errors.New("content type contradicts the advertised type")
)

// SHA256Hex is the single content-identity function for objects and chunks.
// Exact SHA-256 remains the only deduplication identity Notrios recognizes.
func SHA256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

// Chunk is one transfer segment of an object.
type Chunk struct {
	Ordinal int    `json:"ordinal"`
	SHA256  string `json:"sha256"`
	Length  int64  `json:"length"`
}

// Manifest describes how an object is divided and what each part must hash to.
// It is itself content-addressed, so a receiver can be told a manifest's digest
// in a bounded operation payload and fetch the manifest separately without
// having to trust whoever handed it over.
type Manifest struct {
	ObjectSHA256 string  `json:"object_sha256"`
	ByteLength   int64   `json:"byte_length"`
	ChunkBytes   int64   `json:"chunk_bytes"`
	Chunks       []Chunk `json:"chunks"`
}

// ChunkCount returns the number of transfer segments an object of this length
// uses. An object at or below the threshold is one chunk, which is the object.
func ChunkCount(byteLength int64) (int, error) {
	if byteLength < 0 || byteLength > MaxObjectBytes {
		return 0, fmt.Errorf("%w: %d bytes", ErrObjectTooLarge, byteLength)
	}
	if byteLength <= WholeObjectThreshold {
		return 1, nil
	}
	count := (byteLength + ChunkBytes - 1) / ChunkBytes
	if count > MaxChunks {
		return 0, fmt.Errorf("%w: %d chunks", ErrObjectTooLarge, count)
	}
	return int(count), nil
}

// IsChunked reports whether an object of this length is transferred in parts.
// A whole object needs no manifest: its single chunk hash is the object hash,
// so fetching one would be asking a peer to confirm what we already know.
func IsChunked(byteLength int64) bool { return byteLength > WholeObjectThreshold }

// ChunkRange returns the byte offset and length of one chunk.
func ChunkRange(byteLength int64, ordinal int) (int64, int64, error) {
	count, err := ChunkCount(byteLength)
	if err != nil {
		return 0, 0, err
	}
	if ordinal < 0 || ordinal >= count {
		return 0, 0, fmt.Errorf("%w: chunk %d of %d", ErrManifestInvalid, ordinal, count)
	}
	if !IsChunked(byteLength) {
		return 0, byteLength, nil
	}
	offset := int64(ordinal) * ChunkBytes
	length := int64(ChunkBytes)
	if remaining := byteLength - offset; remaining < length {
		length = remaining
	}
	return offset, length, nil
}

// BuildManifest computes the manifest of an object held locally. `at` reads a
// byte range, which is what lets a caller build a manifest for a 16 GiB file
// without holding it in memory.
func BuildManifest(byteLength int64, at func(offset, length int64) ([]byte, error)) (Manifest, error) {
	count, err := ChunkCount(byteLength)
	if err != nil {
		return Manifest{}, err
	}
	digest := sha256.New()
	manifest := Manifest{ByteLength: byteLength, ChunkBytes: ChunkBytes, Chunks: make([]Chunk, 0, count)}
	if !IsChunked(byteLength) {
		manifest.ChunkBytes = byteLength
	}
	for ordinal := 0; ordinal < count; ordinal++ {
		offset, length, err := ChunkRange(byteLength, ordinal)
		if err != nil {
			return Manifest{}, err
		}
		bytes, err := at(offset, length)
		if err != nil {
			return Manifest{}, err
		}
		if int64(len(bytes)) != length {
			return Manifest{}, fmt.Errorf("%w: chunk %d read %d of %d bytes", ErrManifestInvalid, ordinal, len(bytes), length)
		}
		digest.Write(bytes)
		manifest.Chunks = append(manifest.Chunks, Chunk{Ordinal: ordinal, SHA256: SHA256Hex(bytes), Length: length})
	}
	manifest.ObjectSHA256 = hex.EncodeToString(digest.Sum(nil))
	return manifest, nil
}

// Validate checks a manifest against the object it claims to describe, before
// any chunk is fetched. A manifest that does not add up is refused here rather
// than discovered after a multi-gigabyte download.
func (m Manifest) Validate(objectSHA256 string, byteLength int64) error {
	if m.ObjectSHA256 != objectSHA256 {
		return fmt.Errorf("%w: manifest names object %q, expected %q", ErrManifestInvalid, m.ObjectSHA256, objectSHA256)
	}
	if m.ByteLength != byteLength {
		return fmt.Errorf("%w: manifest length %d, expected %d", ErrManifestInvalid, m.ByteLength, byteLength)
	}
	count, err := ChunkCount(byteLength)
	if err != nil {
		return err
	}
	if len(m.Chunks) != count {
		return fmt.Errorf("%w: %d chunks, expected %d", ErrManifestInvalid, len(m.Chunks), count)
	}
	var total int64
	for ordinal, chunk := range m.Chunks {
		if chunk.Ordinal != ordinal {
			return fmt.Errorf("%w: chunk %d is out of order", ErrManifestInvalid, ordinal)
		}
		_, expected, err := ChunkRange(byteLength, ordinal)
		if err != nil {
			return err
		}
		if chunk.Length != expected {
			return fmt.Errorf("%w: chunk %d is %d bytes, expected %d", ErrManifestInvalid, ordinal, chunk.Length, expected)
		}
		if !validHexDigest(chunk.SHA256) {
			return fmt.Errorf("%w: chunk %d has no valid hash", ErrManifestInvalid, ordinal)
		}
		total += chunk.Length
	}
	if total != byteLength {
		return fmt.Errorf("%w: chunks total %d bytes, expected %d", ErrManifestInvalid, total, byteLength)
	}
	return nil
}

// Digest is the manifest's own content address. It is what travels inside a
// bounded operation payload: 16,384 chunk hashes would not fit, and a digest
// makes the manifest verifiable when it arrives by another route.
func (m Manifest) Digest() string {
	digest := sha256.New()
	digest.Write([]byte("notrios.chunk-manifest.v1"))
	for _, part := range []string{m.ObjectSHA256, strconv.FormatInt(m.ByteLength, 10), strconv.FormatInt(m.ChunkBytes, 10)} {
		digest.Write([]byte{0})
		digest.Write([]byte(part))
	}
	chunks := append([]Chunk(nil), m.Chunks...)
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Ordinal < chunks[j].Ordinal })
	for _, chunk := range chunks {
		digest.Write([]byte{0})
		digest.Write([]byte(strconv.Itoa(chunk.Ordinal)))
		digest.Write([]byte{0})
		digest.Write([]byte(chunk.SHA256))
		digest.Write([]byte{0})
		digest.Write([]byte(strconv.FormatInt(chunk.Length, 10)))
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// WholeManifest returns the manifest of an unchunked object, which is derivable
// without asking anyone: one chunk, and it is the object.
func WholeManifest(objectSHA256 string, byteLength int64) (Manifest, error) {
	if IsChunked(byteLength) {
		return Manifest{}, fmt.Errorf("%w: %d bytes is above the whole-object threshold", ErrManifestInvalid, byteLength)
	}
	if !validHexDigest(objectSHA256) {
		return Manifest{}, fmt.Errorf("%w: invalid object hash", ErrManifestInvalid)
	}
	return Manifest{
		ObjectSHA256: objectSHA256, ByteLength: byteLength, ChunkBytes: byteLength,
		Chunks: []Chunk{{Ordinal: 0, SHA256: objectSHA256, Length: byteLength}},
	}, nil
}

// VerifyChunk checks fetched bytes against the manifest entry they claim to be.
func (m Manifest) VerifyChunk(ordinal int, content []byte) error {
	if ordinal < 0 || ordinal >= len(m.Chunks) {
		return fmt.Errorf("%w: chunk %d is outside the manifest", ErrChunkMismatch, ordinal)
	}
	chunk := m.Chunks[ordinal]
	if int64(len(content)) != chunk.Length {
		return fmt.Errorf("%w: chunk %d is %d bytes, manifest says %d", ErrChunkMismatch, ordinal, len(content), chunk.Length)
	}
	if SHA256Hex(content) != chunk.SHA256 {
		return fmt.Errorf("%w: chunk %d hash", ErrChunkMismatch, ordinal)
	}
	return nil
}

func validHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// MaterializationPolicy names when a replica fetches bytes on its own.
type MaterializationPolicy string

const (
	// PolicyLazy fetches nothing automatically. Metadata still converges.
	PolicyLazy MaterializationPolicy = "lazy"
	// PolicyEager is the default: small objects are fetched automatically
	// within a byte budget, larger ones wait for a pin or an explicit request.
	PolicyEager MaterializationPolicy = "eager"
	// PolicyAll fetches everything, budget permitting. It is the choice of a
	// replica that intends to be a complete copy.
	PolicyAll MaterializationPolicy = "all"
)

// NormalizePolicy resolves an unrecognized or empty policy to the default
// rather than failing. A typo in configuration should not silently stop a
// library from fetching its own attachments.
func NormalizePolicy(value string) MaterializationPolicy {
	switch MaterializationPolicy(value) {
	case PolicyLazy:
		return PolicyLazy
	case PolicyAll:
		return PolicyAll
	default:
		return PolicyEager
	}
}

// Decision is why an object is or is not fetched now. The reason is carried
// because "not downloaded" and "too big to download automatically" and "no peer
// has it" are three different things to a person looking at a note.
type Decision struct {
	Fetch  bool
	Reason string
}

// Decide applies the materialization policy to one object.
//
// A pinned object is always fetched: pinning is the user saying they want it
// regardless of size. Otherwise the default admits every object's metadata
// immediately and fetches automatically only what is small enough to be viewed
// inline, within a budget, which is the G2 threshold doing double duty — the
// same one megabyte that decides whole-versus-chunked transfer.
func Decide(policy MaterializationPolicy, byteLength int64, pinned, requested bool, spentEagerBytes int64) Decision {
	switch {
	case pinned:
		return Decision{Fetch: true, Reason: "pinned"}
	case requested:
		return Decision{Fetch: true, Reason: "requested"}
	case policy == PolicyLazy:
		return Decision{Fetch: false, Reason: "lazy_policy"}
	case policy == PolicyAll:
		if spentEagerBytes+byteLength > MaxStagingBytes {
			return Decision{Fetch: false, Reason: "budget_exhausted"}
		}
		return Decision{Fetch: true, Reason: "all_policy"}
	case byteLength > WholeObjectThreshold:
		return Decision{Fetch: false, Reason: "above_eager_threshold"}
	case spentEagerBytes+byteLength > MaxEagerBytes:
		return Decision{Fetch: false, Reason: "budget_exhausted"}
	default:
		return Decision{Fetch: true, Reason: "eager_small_object"}
	}
}

// AcceptableMIME reports whether content sniffed as `sniffed` may be stored
// under the advertised type.
//
// The rule is deliberately narrow rather than an equality check.
// `http.DetectContentType` is inconclusive for most formats — it answers
// `application/octet-stream` or `text/plain` for anything it does not
// specifically recognize — so demanding equality would reject ordinary
// attachments. What it *is* good at is recognizing a handful of dangerous
// types confidently, and those are exactly the cases where an advertised type
// that disagrees means the sender is lying about what it sent.
func AcceptableMIME(advertised, sniffed string) error {
	if sniffed == "" || advertised == "" {
		return nil
	}
	if normalizeMIME(advertised) == normalizeMIME(sniffed) {
		return nil
	}
	if inconclusive(sniffed) {
		return nil
	}
	if familyOf(sniffed) == familyOf(advertised) {
		return nil
	}
	return fmt.Errorf("%w: advertised %q, content is %q", ErrMIMEMismatch, advertised, sniffed)
}

func inconclusive(sniffed string) bool {
	switch normalizeMIME(sniffed) {
	case "application/octet-stream", "text/plain", "text/xml":
		return true
	}
	return false
}

func normalizeMIME(value string) string {
	for index := 0; index < len(value); index++ {
		if value[index] == ';' {
			value = value[:index]
			break
		}
	}
	lowered := make([]byte, 0, len(value))
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char == ' ' || char == '\t' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			char += 'a' - 'A'
		}
		lowered = append(lowered, char)
	}
	return string(lowered)
}

func familyOf(value string) string {
	normalized := normalizeMIME(value)
	for index := 0; index < len(normalized); index++ {
		if normalized[index] == '/' {
			return normalized[:index]
		}
	}
	return normalized
}
