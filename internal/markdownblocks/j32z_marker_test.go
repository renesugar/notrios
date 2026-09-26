package markdownblocks

import (
	"math/rand"
	"strings"
	"testing"
)

// referenceSplitMarker is splitMarker as it was before J32-Z: the regexp, run
// over the whole block.
func referenceSplitMarker(text string) (string, string) {
	match := markerRE.FindStringSubmatchIndex(text)
	if match == nil {
		return text, ""
	}
	return strings.TrimRight(text[:match[0]], " \t"), text[match[2]:match[3]]
}

// j32zTexts covers what the pattern can meet at a block's end: a marker of
// every length including one past the limit, no space before the caret, a caret
// with nothing after it, trailing whitespace of each kind, two marker-shaped
// tails, characters the class excludes, multi-byte text before the marker, and
// a block that is only a marker.
func j32zTexts() []string {
	texts := []string{
		"", " ", "^", "^x", " ^", " ^x", " ^x ", " ^x\n", " ^x\t\n ",
		"text ^marker", "text  ^marker", "text\t^marker", "text\n^marker",
		"text ^marker   ", "text ^marker\n\n", "text ^mark-er_1",
		"text ^x y", "text ^x y ^z", "text ^x ^y", "text ^x\ty",
		"text ^" + strings.Repeat("a", MaxMarkerBytes),
		"text ^" + strings.Repeat("a", MaxMarkerBytes+1),
		"text ^" + strings.Repeat("a", MaxMarkerBytes-1),
		"text ^with.dot", "text ^with space", "text ^with!bang",
		"ελληνικά ^marker", "ünïcode text ^m", "ελληνικά^marker",
		"^marker at the start", "text^marker", "text ^^marker",
		"line one\nline two ^marker", "trailing tabs ^m\t\t",
		strings.Repeat("word ", 200) + "^tail",
		strings.Repeat("word ", 200) + " ^tail",
	}
	pieces := []string{"^", " ", "a", "-", "_", "1", "\n", "\t", "!", ".", "ü", "x"}
	random := rand.New(rand.NewSource(32))
	for attempt := 0; attempt < 20000; attempt++ {
		var builder strings.Builder
		for length := random.Intn(14); length >= 0; length-- {
			builder.WriteString(pieces[random.Intn(len(pieces))])
		}
		texts = append(texts, builder.String())
	}
	return texts
}

// TestSplitMarkerMatchesTheRegexp holds the tail scan to the pattern it
// replaces, on every text: same remainder, same marker (v1.0 J32-Z).
func TestSplitMarkerMatchesTheRegexp(t *testing.T) {
	for _, text := range j32zTexts() {
		gotText, gotMarker := splitMarker(text)
		wantText, wantMarker := referenceSplitMarker(text)
		if gotText != wantText || gotMarker != wantMarker {
			t.Fatalf("splitMarker(%q):\n got  %q / %q\n want %q / %q",
				text, gotText, gotMarker, wantText, wantMarker)
		}
	}
}

// BenchmarkSplitMarkerLargeBlock is the claim: finding a marker at the end of a
// large block must not depend on how large the block is.
func BenchmarkSplitMarkerLargeBlock(b *testing.B) {
	body := strings.Repeat("near limit text and more of it ", 2_000_000) + " ^marker"
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.Run("tail", func(b *testing.B) {
		for attempt := 0; attempt < b.N; attempt++ {
			if _, marker := splitMarker(body); marker != "marker" {
				b.Fatal(marker)
			}
		}
	})
	b.Run("regexp", func(b *testing.B) {
		for attempt := 0; attempt < b.N; attempt++ {
			if _, marker := referenceSplitMarker(body); marker != "marker" {
				b.Fatal(marker)
			}
		}
	})
}
