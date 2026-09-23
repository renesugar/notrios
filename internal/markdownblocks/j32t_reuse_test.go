package markdownblocks

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func j32tNote(index, blocks int) string {
	var builder strings.Builder
	for block := 0; block < blocks; block++ {
		fmt.Fprintf(&builder, "## Note %d heading %d\n\nParagraph %d of note %d\nsecond line\n\n- item %d\n\n",
			index, block, block, index, block)
	}
	return builder.String()
}

// TestExtractorReuseMatchesFreshExtract is the leak test: notes of decreasing
// block count go through one Extractor, and every block must equal what a
// fresh Extract returns. A buffer that kept a longer note's blocks would show
// here (v1.0 J32-T).
func TestExtractorReuseMatchesFreshExtract(t *testing.T) {
	var extractor Extractor
	for index, blocks := range []int{40, 25, 7, 1, 12, 0, 3} {
		body := j32tNote(index, blocks)
		documentID := fmt.Sprintf("doc-%d", index)
		reused := extractor.Extract(documentID, body)
		fresh := Extract(documentID, body)
		if len(reused) != len(fresh) {
			t.Fatalf("note %d: reused returned %d blocks, fresh %d", index, len(reused), len(fresh))
		}
		if !reflect.DeepEqual(reused, fresh) {
			for position := range fresh {
				if !reflect.DeepEqual(reused[position], fresh[position]) {
					t.Fatalf("note %d block %d: reused %+v, fresh %+v", index, position, reused[position], fresh[position])
				}
			}
		}
	}
}

// TestExtractorResetReleasesTheBody proves Reset drops what the buffers refer
// to. A block's Text is a slice of the body, so a buffer still holding the
// last note's blocks would keep that body alive.
func TestExtractorResetReleasesTheBody(t *testing.T) {
	var extractor Extractor
	extractor.Extract("doc-1", j32tNote(1, 20))
	if len(extractor.blocks) == 0 {
		t.Fatal("nothing was extracted")
	}
	held := cap(extractor.blocks)
	extractor.Reset()
	if len(extractor.blocks) != 0 || len(extractor.lines) != 0 {
		t.Fatal("Reset must empty the buffers")
	}
	if cap(extractor.blocks) != held {
		t.Fatalf("Reset must keep capacity: %d became %d", held, cap(extractor.blocks))
	}
	for _, block := range extractor.blocks[:cap(extractor.blocks)] {
		if block.Text != "" || block.ID != "" {
			t.Fatalf("Reset left a block behind: %+v", block)
		}
	}
	for _, line := range extractor.lines[:cap(extractor.lines)] {
		if line != "" {
			t.Fatalf("Reset left a line behind: %q", line)
		}
	}
}

// BenchmarkExtractPerNote is what J32-T is for: many notes, each parsed and
// then written, which is what an import does.
func BenchmarkExtractPerNote(b *testing.B) {
	notes := make([]string, 200)
	for index := range notes {
		notes[index] = j32tNote(index, 30)
	}
	b.Run("fresh", func(b *testing.B) {
		b.ReportAllocs()
		for attempt := 0; attempt < b.N; attempt++ {
			for index, body := range notes {
				if len(Extract(fmt.Sprintf("doc-%d", index), body)) == 0 {
					b.Fatal("no blocks")
				}
			}
		}
	})
	b.Run("reused", func(b *testing.B) {
		b.ReportAllocs()
		var extractor Extractor
		for attempt := 0; attempt < b.N; attempt++ {
			for index, body := range notes {
				if len(extractor.Extract(fmt.Sprintf("doc-%d", index), body)) == 0 {
					b.Fatal("no blocks")
				}
			}
		}
	})
}
