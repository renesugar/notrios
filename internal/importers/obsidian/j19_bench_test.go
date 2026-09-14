package obsidian

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// J19-A benchmarks. Each pairs the code as it is with the change the external
// performance review proposed, so the verdict is a measurement. The variants
// live here, not in the importer, until a measurement justifies moving one.

// fingerprintStreamed composes the same bytes as noteFingerprint without
// building the concatenated string or copying the canonical body to a []byte.
func fingerprintStreamed(item vaultFile, notebookID, canonical string) string {
	inner := sha256.New()
	_, _ = io.WriteString(inner, canonical)
	var innerHex [sha256.Size * 2]byte
	hex.Encode(innerHex[:], inner.Sum(nil))
	outer := sha256.New()
	_, _ = io.WriteString(outer, item.Fingerprint)
	_, _ = outer.Write([]byte{0})
	_, _ = io.WriteString(outer, notebookID)
	_, _ = outer.Write([]byte{0})
	_, _ = outer.Write(innerHex[:])
	return hex.EncodeToString(outer.Sum(nil))
}

// A fingerprint is persisted import state: a different value makes every
// unchanged item look changed. The streamed variant is only a candidate if it
// is byte-identical on every input, including empty and non-ASCII ones.
func TestJ19StreamedFingerprintIsByteIdentical(t *testing.T) {
	bodies := []string{"", "x", "---\ntitle: a\n---\n# Heading\n", strings.Repeat("Grüße 🍲\n", 5000)}
	notebooks := []string{"", "nb_default", "nb_ümlaut"}
	for i, body := range bodies {
		for _, notebook := range notebooks {
			item := vaultFile{Fingerprint: sha256Hex([]byte(fmt.Sprintf("file-%d", i)))}
			if got, want := fingerprintStreamed(item, notebook, body), noteFingerprint(item, notebook, body); got != want {
				t.Fatalf("body %d notebook %q: streamed %s, current %s", i, notebook, got, want)
			}
		}
	}
}

func benchmarkBody(size int) string {
	line := "Zutaten: 200 g Mehl, 2 Eier, [[Teig]] und etwas Salz.\n"
	return strings.Repeat(line, size/len(line)+1)[:size]
}

func BenchmarkJ19Fingerprint(b *testing.B) {
	item := vaultFile{Fingerprint: sha256Hex([]byte("file"))}
	for _, size := range []int{2 << 10, 32 << 10} {
		body := benchmarkBody(size)
		b.Run(fmt.Sprintf("current/%dKiB", size>>10), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = noteFingerprint(item, "nb_default", body)
			}
		})
		b.Run(fmt.Sprintf("streamed/%dKiB", size>>10), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = fingerprintStreamed(item, "nb_default", body)
			}
		})
	}
}

var j19BufferPool = sync.Pool{New: func() any {
	buffer := make([]byte, 32*1024)
	return &buffer
}}

// hashFilePooled is hashFile with its buffer drawn from a sync.Pool.
func hashFilePooled(ctx context.Context, path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	bufferPtr := j19BufferPool.Get().(*[]byte)
	defer j19BufferPool.Put(bufferPtr)
	buffer := *bufferPtr
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return "", size, err
		}
		count, readErr := file.Read(buffer)
		if count > 0 {
			_, _ = hash.Write(buffer[:count])
			size += int64(count)
		}
		if readErr == io.EOF {
			return hex.EncodeToString(hash.Sum(nil)), size, nil
		}
		if readErr != nil {
			return "", size, readErr
		}
	}
}

func BenchmarkJ19HashFile(b *testing.B) {
	ctx := context.Background()
	dir := b.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte(benchmarkBody(4<<10)), 0o600); err != nil {
		b.Fatal(err)
	}
	b.Run("current", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, _, err := hashFile(ctx, path); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("pooled", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, _, err := hashFilePooled(ctx, path); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// The review says the string conversions in splitFrontmatterBytes allocate.
// The compiler does not allocate for string(b) used only in a comparison; the
// allocation count reported here settles which is true on this toolchain.
func BenchmarkJ19SplitFrontmatterBytes(b *testing.B) {
	body := []byte("---\ntitle: Kartoffelsalat\naliases: [a, b]\ntags: [salat]\n---\n" + benchmarkBody(4<<10))
	b.ReportAllocs()
	for b.Loop() {
		if _, _, ok := splitFrontmatterBytes(body); !ok {
			b.Fatal("frontmatter not found")
		}
	}
}
