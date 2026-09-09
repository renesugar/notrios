package syncassets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand"
	"testing"
)

func generated(seed int64, size int) []byte {
	random := rand.New(rand.NewSource(seed))
	value := make([]byte, size)
	_, _ = random.Read(value)
	return value
}

func readerAt(content []byte) func(int64, int64) ([]byte, error) {
	return func(offset, length int64) ([]byte, error) {
		return content[offset : offset+length], nil
	}
}

func TestChunkPlanMatchesTheSelectedShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		length     int64
		wantChunks int
		wantSplit  bool
	}{
		{name: "empty", length: 0, wantChunks: 1},
		{name: "one-byte", length: 1, wantChunks: 1},
		{name: "just-below-the-threshold", length: WholeObjectThreshold - 1, wantChunks: 1},
		{name: "exactly-the-threshold", length: WholeObjectThreshold, wantChunks: 1},
		{name: "one-byte-above", length: WholeObjectThreshold + 1, wantChunks: 2, wantSplit: true},
		{name: "four-chunks", length: 4 * ChunkBytes, wantChunks: 4, wantSplit: true},
		{name: "ragged-tail", length: 4*ChunkBytes + 7, wantChunks: 5, wantSplit: true},
		{name: "the-ceiling", length: MaxObjectBytes, wantChunks: MaxChunks, wantSplit: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			count, err := ChunkCount(test.length)
			if err != nil || count != test.wantChunks {
				t.Fatalf("ChunkCount(%d) = %d, %v; want %d", test.length, count, err, test.wantChunks)
			}
			if IsChunked(test.length) != test.wantSplit {
				t.Fatalf("IsChunked(%d) = %v, want %v", test.length, IsChunked(test.length), test.wantSplit)
			}
			// Every chunk range must tile the object exactly: no gap, no
			// overlap, nothing past the end.
			var covered int64
			for ordinal := 0; ordinal < count; ordinal++ {
				offset, length, err := ChunkRange(test.length, ordinal)
				if err != nil {
					t.Fatal(err)
				}
				if offset != covered {
					t.Fatalf("chunk %d starts at %d, expected %d", ordinal, offset, covered)
				}
				covered += length
			}
			if covered != test.length {
				t.Fatalf("chunks covered %d bytes of %d", covered, test.length)
			}
			if _, _, err := ChunkRange(test.length, count); err == nil {
				t.Fatal("a chunk past the end was accepted")
			}
		})
	}

	if _, err := ChunkCount(MaxObjectBytes + 1); !errors.Is(err, ErrObjectTooLarge) {
		t.Fatalf("above the ceiling: %v", err)
	}
	if _, err := ChunkCount(-1); !errors.Is(err, ErrObjectTooLarge) {
		t.Fatalf("negative length: %v", err)
	}
}

func TestManifestDescribesAndVerifiesItsObject(t *testing.T) {
	t.Parallel()
	content := generated(3, 3*ChunkBytes+1_234)
	digest := sha256.Sum256(content)
	objectSHA := hex.EncodeToString(digest[:])

	manifest, err := BuildManifest(int64(len(content)), readerAt(content))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ObjectSHA256 != objectSHA {
		t.Fatal("BuildManifest did not hash the object it read")
	}
	if err := manifest.Validate(objectSHA, int64(len(content))); err != nil {
		t.Fatalf("a manifest built from the object failed to validate it: %v", err)
	}
	if len(manifest.Chunks) != 4 {
		t.Fatalf("chunks = %d", len(manifest.Chunks))
	}
	for ordinal := range manifest.Chunks {
		offset, length, _ := ChunkRange(int64(len(content)), ordinal)
		if err := manifest.VerifyChunk(ordinal, content[offset:offset+length]); err != nil {
			t.Fatalf("chunk %d: %v", ordinal, err)
		}
		corrupt := append([]byte(nil), content[offset:offset+length]...)
		corrupt[0] ^= 0xff
		if err := manifest.VerifyChunk(ordinal, corrupt); !errors.Is(err, ErrChunkMismatch) {
			t.Fatalf("chunk %d accepted corrupted bytes: %v", ordinal, err)
		}
		if err := manifest.VerifyChunk(ordinal, content[offset:offset+length-1]); !errors.Is(err, ErrChunkMismatch) {
			t.Fatalf("chunk %d accepted a short read", ordinal)
		}
	}
	if err := manifest.VerifyChunk(len(manifest.Chunks), nil); !errors.Is(err, ErrChunkMismatch) {
		t.Fatal("a chunk outside the manifest was accepted")
	}
}

func TestManifestValidationRefusesEveryWayItCanDisagree(t *testing.T) {
	t.Parallel()
	content := generated(5, 2*ChunkBytes+10)
	length := int64(len(content))
	base, err := BuildManifest(length, readerAt(content))
	if err != nil {
		t.Fatal(err)
	}
	objectSHA := base.ObjectSHA256

	mutations := map[string]func(Manifest) Manifest{
		"wrong-object-hash": func(m Manifest) Manifest { m.ObjectSHA256 = SHA256Hex([]byte("other")); return m },
		"wrong-length":      func(m Manifest) Manifest { m.ByteLength = length + 1; return m },
		"missing-chunk": func(m Manifest) Manifest {
			m.Chunks = append([]Chunk(nil), m.Chunks[:len(m.Chunks)-1]...)
			return m
		},
		"extra-chunk": func(m Manifest) Manifest {
			m.Chunks = append(append([]Chunk(nil), m.Chunks...), Chunk{Ordinal: len(m.Chunks), SHA256: SHA256Hex(nil), Length: 1})
			return m
		},
		"reordered-chunks": func(m Manifest) Manifest {
			m.Chunks = append([]Chunk(nil), m.Chunks...)
			m.Chunks[0], m.Chunks[1] = m.Chunks[1], m.Chunks[0]
			return m
		},
		"wrong-chunk-length": func(m Manifest) Manifest {
			m.Chunks = append([]Chunk(nil), m.Chunks...)
			m.Chunks[0].Length = 7
			return m
		},
		"malformed-chunk-hash": func(m Manifest) Manifest {
			m.Chunks = append([]Chunk(nil), m.Chunks...)
			m.Chunks[0].SHA256 = "not-a-hash"
			return m
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			if err := mutate(base).Validate(objectSHA, length); err == nil {
				t.Fatal("an invalid manifest was accepted")
			}
		})
	}
}

// The digest is what a bounded operation payload carries in place of up to
// 16,384 chunk hashes, so it must change whenever anything it stands for does.
func TestManifestDigestBindsEveryField(t *testing.T) {
	t.Parallel()
	content := generated(11, 2*ChunkBytes)
	base, err := BuildManifest(int64(len(content)), readerAt(content))
	if err != nil {
		t.Fatal(err)
	}
	original := base.Digest()
	if original != base.Digest() {
		t.Fatal("the digest is not deterministic")
	}
	// Chunk order must not change it: the manifest is a set of ordinals, and
	// two peers must agree on the digest however they enumerated them.
	shuffled := base
	shuffled.Chunks = []Chunk{base.Chunks[1], base.Chunks[0]}
	if shuffled.Digest() != original {
		t.Fatal("chunk enumeration order changed the digest")
	}

	for name, mutate := range map[string]func(Manifest) Manifest{
		"object-hash": func(m Manifest) Manifest { m.ObjectSHA256 = SHA256Hex([]byte("x")); return m },
		"length":      func(m Manifest) Manifest { m.ByteLength++; return m },
		"chunk-size":  func(m Manifest) Manifest { m.ChunkBytes++; return m },
		"chunk-hash": func(m Manifest) Manifest {
			m.Chunks = []Chunk{{0, SHA256Hex([]byte("y")), m.Chunks[0].Length}, m.Chunks[1]}
			return m
		},
		"chunk-length": func(m Manifest) Manifest {
			m.Chunks = []Chunk{{0, m.Chunks[0].SHA256, m.Chunks[0].Length - 1}, m.Chunks[1]}
			return m
		},
	} {
		if mutate(base).Digest() == original {
			t.Fatalf("changing the %s did not change the digest", name)
		}
	}
}

func TestWholeObjectNeedsNoManifestFetch(t *testing.T) {
	t.Parallel()
	content := generated(7, 4_096)
	objectSHA := SHA256Hex(content)
	derived, err := WholeManifest(objectSHA, int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	built, err := BuildManifest(int64(len(content)), readerAt(content))
	if err != nil {
		t.Fatal(err)
	}
	// A receiver derives the same manifest a sender would have built, so asking
	// for it would be asking a peer to confirm what is already known.
	if derived.Digest() != built.Digest() {
		t.Fatalf("derived manifest differs from the built one:\n%+v\n%+v", derived, built)
	}
	if err := derived.Validate(objectSHA, int64(len(content))); err != nil {
		t.Fatal(err)
	}
	if _, err := WholeManifest(objectSHA, WholeObjectThreshold+1); !errors.Is(err, ErrManifestInvalid) {
		t.Fatal("a chunked object must not derive a whole manifest")
	}
	if _, err := WholeManifest("short", 10); !errors.Is(err, ErrManifestInvalid) {
		t.Fatal("a malformed hash was accepted")
	}
}

func TestEmptyObjectRoundTrips(t *testing.T) {
	t.Parallel()
	manifest, err := BuildManifest(0, readerAt(nil))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ObjectSHA256 != SHA256Hex(nil) || len(manifest.Chunks) != 1 || manifest.Chunks[0].Length != 0 {
		t.Fatalf("empty object manifest: %+v", manifest)
	}
	if err := manifest.Validate(SHA256Hex(nil), 0); err != nil {
		t.Fatal(err)
	}
	if err := manifest.VerifyChunk(0, nil); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializationPolicy(t *testing.T) {
	t.Parallel()
	small := int64(64 << 10)
	large := int64(8 << 20)

	cases := []struct {
		name       string
		policy     MaterializationPolicy
		length     int64
		pinned     bool
		requested  bool
		spent      int64
		wantFetch  bool
		wantReason string
	}{
		{name: "small-object-by-default", policy: PolicyEager, length: small, wantFetch: true, wantReason: "eager_small_object"},
		{name: "large-object-waits", policy: PolicyEager, length: large, wantFetch: false, wantReason: "above_eager_threshold"},
		{name: "pinning-overrides-size", policy: PolicyEager, length: large, pinned: true, wantFetch: true, wantReason: "pinned"},
		{name: "pinning-overrides-lazy", policy: PolicyLazy, length: large, pinned: true, wantFetch: true, wantReason: "pinned"},
		{name: "an-explicit-request-is-honoured", policy: PolicyLazy, length: large, requested: true, wantFetch: true, wantReason: "requested"},
		{name: "lazy-fetches-nothing-on-its-own", policy: PolicyLazy, length: small, wantFetch: false, wantReason: "lazy_policy"},
		{name: "all-takes-the-large-one", policy: PolicyAll, length: large, wantFetch: true, wantReason: "all_policy"},
		{name: "eager-budget-stops-it", policy: PolicyEager, length: small, spent: MaxEagerBytes, wantFetch: false, wantReason: "budget_exhausted"},
		{name: "all-budget-stops-it", policy: PolicyAll, length: large, spent: MaxStagingBytes, wantFetch: false, wantReason: "budget_exhausted"},
	}
	for _, test := range cases {
		decision := Decide(test.policy, test.length, test.pinned, test.requested, test.spent)
		if decision.Fetch != test.wantFetch || decision.Reason != test.wantReason {
			t.Fatalf("%s: Decide = %+v, want fetch=%v reason=%q", test.name, decision, test.wantFetch, test.wantReason)
		}
	}

	// A pin must win over an exhausted budget, or pinning would be advice.
	if decision := Decide(PolicyEager, large, true, false, MaxStagingBytes*2); !decision.Fetch {
		t.Fatal("a pinned object was deferred by the budget")
	}
	for _, value := range []string{"", "nonsense", "EAGER"} {
		if NormalizePolicy(value) != PolicyEager {
			t.Fatalf("NormalizePolicy(%q) = %q, want the default", value, NormalizePolicy(value))
		}
	}
	if NormalizePolicy("lazy") != PolicyLazy || NormalizePolicy("all") != PolicyAll {
		t.Fatal("a recognized policy was not preserved")
	}
}

func TestAcceptableMIME(t *testing.T) {
	t.Parallel()
	accepted := [][2]string{
		{"image/png", "image/png"},
		{"image/PNG", "image/png; charset=binary"},
		// DetectContentType is inconclusive for most formats; refusing those
		// would reject ordinary attachments rather than hostile ones.
		{"application/vnd.oasis.opendocument.text", "application/octet-stream"},
		{"text/markdown", "text/plain; charset=utf-8"},
		{"application/xml", "text/xml"},
		// Same family, different specific type: a re-encode, not a lie.
		{"image/jpeg", "image/webp"},
		{"", "image/png"},
		{"image/png", ""},
	}
	for _, pair := range accepted {
		if err := AcceptableMIME(pair[0], pair[1]); err != nil {
			t.Fatalf("AcceptableMIME(%q, %q) = %v, want acceptance", pair[0], pair[1], err)
		}
	}
	refused := [][2]string{
		{"image/png", "application/pdf"},
		{"application/pdf", "text/html; charset=utf-8"},
		{"image/jpeg", "application/zip"},
	}
	for _, pair := range refused {
		if err := AcceptableMIME(pair[0], pair[1]); !errors.Is(err, ErrMIMEMismatch) {
			t.Fatalf("AcceptableMIME(%q, %q) = %v, want a refusal", pair[0], pair[1], err)
		}
	}
}

func TestManifestSurvivesAssemblyOfAChunkedObject(t *testing.T) {
	t.Parallel()
	content := generated(21, 5*ChunkBytes+99)
	manifest, err := BuildManifest(int64(len(content)), readerAt(content))
	if err != nil {
		t.Fatal(err)
	}
	// Fetching out of order and assembling by ordinal must reproduce the object
	// exactly, because a carrier delivers whatever arrives first.
	assembled := make([][]byte, len(manifest.Chunks))
	for _, ordinal := range []int{3, 0, 5, 1, 4, 2} {
		offset, length, _ := ChunkRange(int64(len(content)), ordinal)
		part := content[offset : offset+length]
		if err := manifest.VerifyChunk(ordinal, part); err != nil {
			t.Fatal(err)
		}
		assembled[ordinal] = part
	}
	var whole bytes.Buffer
	for _, part := range assembled {
		whole.Write(part)
	}
	if SHA256Hex(whole.Bytes()) != manifest.ObjectSHA256 {
		t.Fatal("out-of-order assembly did not reproduce the object")
	}
}
