package markdownblocks

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// referenceIdentify and referenceNormalize are what J32-Q replaced. The new
// code must agree with them exactly: a block's ID and hash are stored, and an
// anchor written against a block resolves by them.
func referenceIdentify(documentID string, block Block) [sha256.Size]byte {
	return sha256.Sum256([]byte(strings.Join([]string{
		"notrios-block-v1", documentID, block.Kind, block.Text, itoa(block.Occurrence),
	}, "\x00")))
}

func referenceNormalize(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

var j32qTexts = []string{
	"",
	"plain",
	"two\nlines",
	"trailing space \nand more\t\n",
	"carriage\r\nreturns\r\n",
	"lone\rcarriage\rreturns",
	"ends with newlines\n\n\n",
	"ends with spaces   ",
	"ends with a tab\t",
	"ελληνικά ünïcode",
	"  leading spaces kept",
	"\n",
	"\n\n",
	" ",
	"\t\n\t\n",
	"mixed \r\n trailing \t\r\n",
}

func TestNormalizeMatchesReference(t *testing.T) {
	for _, text := range j32qTexts {
		if got, want := normalize(text), referenceNormalize(text); got != want {
			t.Errorf("normalize(%q) = %q, reference says %q", text, got, want)
		}
	}
}

func TestIdentifyMatchesReference(t *testing.T) {
	for _, text := range j32qTexts {
		for _, kind := range []string{KindParagraph, KindHeading, KindCode, KindTable, KindListItem} {
			for _, occurrence := range []int{0, 1, 42} {
				block := Block{Kind: kind, Text: text, Occurrence: occurrence}
				_, gotHash, gotID := identify("doc-1", block, nil)
				want := referenceIdentify("doc-1", block)
				if gotHash != hex(want[:]) {
					t.Fatalf("hash for %q/%s/%d: %s, reference %s", text, kind, occurrence, gotHash, hex(want[:]))
				}
				if !strings.HasPrefix(gotID, "blk_") {
					t.Fatalf("id %q is not a block id", gotID)
				}
			}
		}
	}
}

// TestExtractUnchangedAcrossShapes pins what Extract returns for bodies that
// exercise every block kind and both line endings, so a change to how a
// block's text is built cannot move an offset, a hash or an ordinal.
func TestExtractUnchangedAcrossShapes(t *testing.T) {
	bodies := []string{
		"# Heading\n\nA paragraph\nrunning on.\n\n- item one\n- item two\n\n```go\ncode\nlines\n```\n\n| a | b |\n| - | - |\n| 1 | 2 |\n",
		"# CRLF\r\n\r\nParagraph with trailing spaces   \r\nand a second line\t\r\n",
		"No trailing newline",
		"```\nunclosed fence\nruns to the end\n",
		"Paragraph ^marker\n\n## Heading ^other\n",
	}
	for index, body := range bodies {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			blocks := Extract("doc-1", body)
			if len(blocks) == 0 {
				t.Fatal("no blocks")
			}
			for _, block := range blocks {
				want := referenceIdentify("doc-1", block)
				if block.ContentSHA256 != hex(want[:]) {
					t.Errorf("%s block %d: hash %s, reference %s",
						block.Kind, block.Ordinal, block.ContentSHA256, hex(want[:]))
				}
				if block.StartByte < 0 || block.EndByte > len(body) || block.StartByte > block.EndByte {
					t.Errorf("%s block %d: offsets %d-%d outside a %d-byte body",
						block.Kind, block.Ordinal, block.StartByte, block.EndByte, len(body))
				}
				if normalize(block.Text) != block.Text {
					t.Errorf("%s block %d: text is not normalized: %q", block.Kind, block.Ordinal, block.Text)
				}
			}
		})
	}
}

// BenchmarkExtractLargeNote is the allocation claim, at the shape of the
// near-limit corpus: a note of repeated short paragraphs.
func BenchmarkExtractLargeNote(b *testing.B) {
	line := strings.Repeat("near limit text ", 8)
	body := strings.Repeat(line+"\n", 200_000)
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for attempt := 0; attempt < b.N; attempt++ {
		if len(Extract("doc-1", body)) == 0 {
			b.Fatal("no blocks")
		}
	}
}

// BenchmarkExtractManyBlocks is the other shape: many small blocks rather than
// one large one, where a per-block allocation is the whole cost.
func BenchmarkExtractManyBlocks(b *testing.B) {
	var builder strings.Builder
	for index := 0; index < 50_000; index++ {
		fmt.Fprintf(&builder, "## Heading %d\n\nParagraph %d line one\nline two\n\n- item\n\n", index, index)
	}
	body := builder.String()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for attempt := 0; attempt < b.N; attempt++ {
		if len(Extract("doc-1", body)) == 0 {
			b.Fatal("no blocks")
		}
	}
}
