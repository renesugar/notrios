package obsidian

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// referenceMarkdownTitle is what markdownTitle did before J32-O: split the
// frontmatter out by copying it, then split the whole body into lines. The
// title is stored and a note is addressed by it, so the new reading has to
// agree with it exactly.
func referenceMarkdownTitle(relPath, body string) string {
	if fm, _, ok := splitFrontmatter(body); ok {
		if title := frontmatterScalar(fm, "title"); title != "" {
			return title
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	base := relPath
	if index := strings.LastIndexByte(base, '/'); index >= 0 {
		base = base[index+1:]
	}
	if index := strings.LastIndexByte(base, '.'); index > 0 {
		base = base[:index]
	}
	base = strings.ReplaceAll(base, "%20", " ")
	return firstNonEmpty(base, "Untitled")
}

func TestMarkdownTitleMatchesReference(t *testing.T) {
	bodies := []string{
		"", "# Heading\n", "#NoSpace\n", "no heading here\n",
		"---\ntitle: From frontmatter\n---\n# Heading\n",
		"---\ntitle:\n---\n# Heading\n",
		"---\nno title key: x\n---\n# Heading\n",
		"---\n---\n# After empty frontmatter\n",
		"---\nunclosed frontmatter\n# Heading\n",
		"---\r\ntitle: CRLF title\r\n---\r\n# Heading\r\n",
		"\n\n   # Indented heading\n",
		"text\n## Not level one\n# Level one\n",
		"# Heading with trailing spaces   \n",
		"# ελληνικά ünïcode\n",
		"---\ntitle: \"quoted\"\n---\n",
		"---\ntitle: 'single'\n---\n",
		"#  double space after hash\n",
		"# ", "#", "\n# \n",
	}
	pieces := []string{"---\n", "title: T\n", "other: x\n", "# H\n", "text\n", "\n", "#NoSpace\n", "\r\n", "  # Indented\n"}
	random := rand.New(rand.NewSource(32))
	for attempt := 0; attempt < 3000; attempt++ {
		var builder strings.Builder
		for length := random.Intn(8); length >= 0; length-- {
			builder.WriteString(pieces[random.Intn(len(pieces))])
		}
		bodies = append(bodies, builder.String())
	}

	for _, relPath := range []string{"Note.md", "Folder/Note%20Name.md", "A/B/.hidden.md", "NoExtension"} {
		for index, body := range bodies {
			got := markdownTitle(relPath, body)
			want := referenceMarkdownTitle(relPath, body)
			if got != want {
				t.Fatalf("body %d at %s: got %q, reference %q, body %q", index, relPath, got, want, body)
			}
		}
	}
}

func BenchmarkMarkdownTitleLargeNote(b *testing.B) {
	var builder strings.Builder
	builder.WriteString("---\naliases: [A]\n---\n")
	for index := 0; index < 200_000; index++ {
		fmt.Fprintf(&builder, "paragraph line %d of a large note\n", index)
	}
	body := builder.String()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for attempt := 0; attempt < b.N; attempt++ {
		markdownTitle("Folder/Note.md", body)
	}
}

func BenchmarkMarkdownTitleLargeNoteReference(b *testing.B) {
	var builder strings.Builder
	builder.WriteString("---\naliases: [A]\n---\n")
	for index := 0; index < 200_000; index++ {
		fmt.Fprintf(&builder, "paragraph line %d of a large note\n", index)
	}
	body := builder.String()
	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for attempt := 0; attempt < b.N; attempt++ {
		referenceMarkdownTitle("Folder/Note.md", body)
	}
}
