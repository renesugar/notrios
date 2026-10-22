package syncdelta

import (
	"fmt"
	"strings"
	"testing"
)

// TestSHA256HexStringMatchesSHA256Hex holds the string form to the byte form
// it exists to avoid: the same bytes, so the same hash, at every size around
// the chunk boundary (v1.0 J32-W1).
func TestSHA256HexStringMatchesSHA256Hex(t *testing.T) {
	sizes := []int{0, 1, 2, 1023, hashChunkBytes - 1, hashChunkBytes, hashChunkBytes + 1,
		2*hashChunkBytes - 1, 2 * hashChunkBytes, 2*hashChunkBytes + 1, 5*hashChunkBytes + 17}
	for _, size := range sizes {
		value := strings.Repeat("näve ελληνικά body ", size/20+1)[:size]
		if got, want := SHA256HexString(value), SHA256Hex([]byte(value)); got != want {
			t.Errorf("size %d: string form %s, byte form %s", size, got, want)
		}
	}
	for _, value := range []string{"", "\x00", "\x00\x00 embedded nulls \x00", "ünïcode"} {
		if got, want := SHA256HexString(value), SHA256Hex([]byte(value)); got != want {
			t.Errorf("%q: string form %s, byte form %s", value, got, want)
		}
	}
}

// BenchmarkSHA256HexString is the allocation claim: hashing a large body must
// not allocate a second copy of it.
func BenchmarkSHA256HexString(b *testing.B) {
	body := strings.Repeat("near limit text and some more of it\n", 2_000_000)
	b.Run(fmt.Sprintf("string-%dMiB", len(body)>>20), func(b *testing.B) {
		b.ReportAllocs()
		for attempt := 0; attempt < b.N; attempt++ {
			SHA256HexString(body)
		}
	})
	b.Run(fmt.Sprintf("bytes-%dMiB", len(body)>>20), func(b *testing.B) {
		b.ReportAllocs()
		for attempt := 0; attempt < b.N; attempt++ {
			SHA256Hex([]byte(body))
		}
	})
}
