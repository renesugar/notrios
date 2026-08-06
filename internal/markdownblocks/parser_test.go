package markdownblocks

import (
	"strings"
	"testing"
)

const sample = "# Title\n\nFirst paragraph.\n\n- one item\n- two item\n\n```go\nfmt.Println(\"x\")\n```\n\n| a | b |\n| - | - |\n\n## Section\n\nLast paragraph. ^anchor-one\n"

func kinds(blocks []Block) []string {
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		out = append(out, block.Kind)
	}
	return out
}

func TestExtractSplitsTheDocumentedKinds(t *testing.T) {
	blocks := Extract("doc_a", sample)
	want := []string{KindHeading, KindParagraph, KindListItem, KindListItem, KindCode, KindTable, KindHeading, KindParagraph}
	got := kinds(blocks)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds: %v", got)
	}
	if blocks[0].Level != 1 || blocks[6].Level != 2 {
		t.Fatalf("heading levels: %+v %+v", blocks[0], blocks[6])
	}
	for i, block := range blocks {
		if block.Ordinal != i {
			t.Fatalf("ordinal %d on block %d", block.Ordinal, i)
		}
		if block.StartByte < 0 || block.EndByte > len(sample) || block.StartByte >= block.EndByte {
			t.Fatalf("byte range outside the body: %+v", block)
		}
		if !strings.HasPrefix(block.ID, "blk_") || len(block.ContentSHA256) != 64 {
			t.Fatalf("unexpected identity: %+v", block)
		}
	}
}

// The decision this whole slice rests on: identity follows the text, not the
// position.
func TestIdentityFollowsContentNotPosition(t *testing.T) {
	original := "# Title\n\nAlpha paragraph.\n\nBeta paragraph.\n"
	reordered := "# Title\n\nBeta paragraph.\n\nAlpha paragraph.\n"

	before := byText(Extract("doc_a", original))
	after := byText(Extract("doc_a", reordered))

	if before["Alpha paragraph."].ID != after["Alpha paragraph."].ID {
		t.Fatal("moving a block must keep its identity")
	}
	if before["Alpha paragraph."].Ordinal == after["Alpha paragraph."].Ordinal {
		t.Fatal("the fixture did not actually move the block")
	}

	edited := byText(Extract("doc_a", "# Title\n\nAlpha paragraph, revised.\n\nBeta paragraph.\n"))
	if _, unchanged := edited["Alpha paragraph."]; unchanged {
		t.Fatal("the fixture did not actually edit the block")
	}
	for _, block := range edited {
		if block.ID == before["Alpha paragraph."].ID {
			t.Fatal("editing a block's text must mint a new identity")
		}
	}
}

// Identity is scoped to its note, so the same sentence in two notes is two
// blocks. A block key must not let one note's content be recognised in another.
func TestIdentityIsScopedToTheDocument(t *testing.T) {
	body := "Shared sentence.\n"
	if Extract("doc_a", body)[0].ID == Extract("doc_b", body)[0].ID {
		t.Fatal("block identity must be scoped to its document")
	}
}

func TestIdenticalBlocksAreDisambiguatedByOccurrence(t *testing.T) {
	blocks := Extract("doc_a", "Repeat.\n\nRepeat.\n\nRepeat.\n")
	if len(blocks) != 3 {
		t.Fatalf("expected three blocks, got %d", len(blocks))
	}
	seen := map[string]bool{}
	for i, block := range blocks {
		if block.Occurrence != i {
			t.Fatalf("occurrence %d on block %d", block.Occurrence, i)
		}
		if seen[block.ID] {
			t.Fatalf("identical blocks collided on %s", block.ID)
		}
		seen[block.ID] = true
	}
}

// Whitespace cleanup on save must not silently break every anchor in a note.
func TestNormalizationSurvivesWhitespaceAndLineEndingChanges(t *testing.T) {
	unix := Extract("doc_a", "# Title\n\nParagraph text.\n")
	windows := Extract("doc_a", "# Title\r\n\r\nParagraph text.\r\n")
	trailing := Extract("doc_a", "# Title   \n\nParagraph text.   \n")
	for i := range unix {
		if unix[i].ID != windows[i].ID {
			t.Fatalf("CRLF changed identity for block %d", i)
		}
		if unix[i].ID != trailing[i].ID {
			t.Fatalf("trailing whitespace changed identity for block %d", i)
		}
	}
}

// A heading and a paragraph that read the same are different blocks: one is a
// section, the other is prose.
func TestKindIsPartOfIdentity(t *testing.T) {
	heading := Extract("doc_a", "# Same words\n")[0]
	paragraph := Extract("doc_a", "Same words\n")[0]
	if heading.Text != paragraph.Text {
		t.Fatalf("fixture mismatch: %q vs %q", heading.Text, paragraph.Text)
	}
	if heading.ID == paragraph.ID {
		t.Fatal("a heading and a paragraph must not share identity")
	}
}

// An author-written marker is a name the author chose. It is kept separately
// and must not become part of the text that derives identity.
func TestAuthoredMarkersAreSeparatedFromContent(t *testing.T) {
	withMarker := Extract("doc_a", "Paragraph text. ^my-anchor\n")[0]
	withoutMarker := Extract("doc_a", "Paragraph text.\n")[0]
	if withMarker.Marker != "my-anchor" {
		t.Fatalf("marker: %q", withMarker.Marker)
	}
	if withMarker.Text != "Paragraph text." {
		t.Fatalf("marker leaked into the text: %q", withMarker.Text)
	}
	if withMarker.ID != withoutMarker.ID {
		t.Fatal("adding an author marker must not change the block's content identity")
	}
}

func TestCodeFencesAndTablesStayWhole(t *testing.T) {
	blocks := Extract("doc_a", "```\nline one\n\nline two\n```\n")
	if len(blocks) != 1 || blocks[0].Kind != KindCode {
		t.Fatalf("a blank line inside a fence must not split it: %v", kinds(blocks))
	}
	if !strings.Contains(blocks[0].Text, "line two") {
		t.Fatalf("fence content truncated: %q", blocks[0].Text)
	}

	unclosed := Extract("doc_a", "```\nno closing fence\n")
	if len(unclosed) != 1 || unclosed[0].Kind != KindCode {
		t.Fatalf("an unclosed fence runs to the end: %v", kinds(unclosed))
	}

	table := Extract("doc_a", "| a | b |\n| - | - |\n| 1 | 2 |\n")
	if len(table) != 1 || table[0].Kind != KindTable {
		t.Fatalf("a table is one block: %v", kinds(table))
	}
}

func TestExtractIsBoundedAndSkipsEmptyContent(t *testing.T) {
	if blocks := Extract("doc_a", "\n\n   \n\n"); len(blocks) != 0 {
		t.Fatalf("whitespace-only body produced %d blocks", len(blocks))
	}
	body := strings.Repeat("paragraph\n\n", MaxBlocksPerDocument+50)
	if blocks := Extract("doc_a", body); len(blocks) != MaxBlocksPerDocument {
		t.Fatalf("expected the per-document bound, got %d", len(blocks))
	}
}

func TestExtractIsDeterministic(t *testing.T) {
	first := Extract("doc_a", sample)
	second := Extract("doc_a", sample)
	for i := range first {
		if first[i].ID != second[i].ID || first[i].StartByte != second[i].StartByte {
			t.Fatalf("block %d differs between runs: %+v %+v", i, first[i], second[i])
		}
	}
}

func byText(blocks []Block) map[string]Block {
	out := map[string]Block{}
	for _, block := range blocks {
		out[block.Text] = block
	}
	return out
}

func TestSlugifyFollowsTheOrdinaryMarkdownRules(t *testing.T) {
	cases := map[string]string{
		"Section Title":            "section-title",
		"  Padded  Heading  ":      "padded-heading",
		"Punctuation: it's here!":  "punctuation-its-here",
		"Already-Hyphenated":       "already-hyphenated",
		"snake_case_heading":       "snake-case-heading",
		"Multiple   Spaces":        "multiple-spaces",
		"Ünïcode Ström":            "ünïcode-ström",
		"日本語 見出し":                  "日本語-見出し",
		"2026 Plans":               "2026-plans",
		"-- leading and trailing -": "leading-and-trailing",
	}
	for text, want := range cases {
		if got := Slugify(text); got != want {
			t.Fatalf("Slugify(%q) = %q, want %q", text, got, want)
		}
	}
	// A heading that slugs to nothing gets no slug rather than an invented one.
	for _, text := range []string{"", "   ", "!!!", "🎉"} {
		if got := Slugify(text); got != "" {
			t.Fatalf("Slugify(%q) = %q, want empty", text, got)
		}
	}
	long := strings.Repeat("a", MaxSlugBytes+50)
	if got := Slugify(long); len(got) > MaxSlugBytes {
		t.Fatalf("slug is %d bytes, limit %d", len(got), MaxSlugBytes)
	}
}

func TestHeadingBlocksCarrySlugsAndRepeatsStayAddressable(t *testing.T) {
	blocks := Extract("doc_a", "# Notes\n\nbody\n\n## Notes\n\nbody\n\n### Other\n\nbody\n")
	headings := []Block{}
	for _, block := range blocks {
		if block.Kind == KindHeading {
			headings = append(headings, block)
		}
	}
	if len(headings) != 3 {
		t.Fatalf("expected three headings, got %d", len(headings))
	}
	// Obsidian resolves a duplicate heading to the first match and offers no way
	// to name the second; a numbered suffix keeps both addressable.
	if headings[0].Slug != "notes" || headings[1].Slug != "notes-1" || headings[2].Slug != "other" {
		t.Fatalf("slugs: %q %q %q", headings[0].Slug, headings[1].Slug, headings[2].Slug)
	}
	for _, block := range blocks {
		if block.Kind != KindHeading && block.Slug != "" {
			t.Fatalf("only headings carry slugs: %+v", block)
		}
	}
}

// A slug names a position in the document's outline; block identity stays
// content-based. Renaming a heading changes both, which is the point.
func TestHeadingSlugTracksTheHeadingText(t *testing.T) {
	before := Extract("doc_a", "# Original Title\n\nbody\n")[0]
	after := Extract("doc_a", "# Renamed Title\n\nbody\n")[0]
	if before.Slug != "original-title" || after.Slug != "renamed-title" {
		t.Fatalf("slugs: %q -> %q", before.Slug, after.Slug)
	}
	if before.ID == after.ID {
		t.Fatal("renaming a heading must also mint a new block ID")
	}
}
