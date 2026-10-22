package obsidian

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// The v1.0 J32-U comparison: three ways to produce a note's canonical body,
// measured on the same notes.
//
//	today      normalize, rewrite, then rebuild the frontmatter, each into a
//	           new string
//	onePass    the same stages, assembled once and without converting the note
//	           between []byte and string (what J32-O and J32-P propose)
//	editScript every change collected as an edit against the original bytes and
//	           applied once (what the owner proposed)
//
// Whichever wins has to produce the same bytes, so the equivalence test runs
// first and the benchmark only measures shapes it agrees on.

func j32uRun() *importRun {
	return &importRun{
		options: Options{CollectionID: "default"},
		namespace: linkNamespace{
			notesByPath:  map[string]string{"target": "doc-target", "other": "doc-other"},
			notesByName:  map[string][]string{"target": {"doc-target"}, "other": {"doc-other"}},
			assetsByPath: map[string]string{},
			assetsByBase: map[string][]string{},
		},
	}
}

// j32uNotes are the shapes a canonical body is built from: with and without
// frontmatter, with and without links, CRLF, no trailing newline, and a large
// one.
func j32uNotes() map[string]string {
	large := strings.Builder{}
	for index := 0; index < 20_000; index++ {
		fmt.Fprintf(&large, "Paragraph %d with a [[Target]] link and some ελληνικά text.\n\n", index)
	}
	return map[string]string{
		"plain":               "# Note\n\nA paragraph with no links.\n",
		"frontmatter":         "---\naliases: [A]\nunknown: kept\n---\n# Note\n\nBody [[Target]].\n",
		"links":               "# Note\n\n[[Target]] and [[Other]] and [text](Target.md).\n",
		"crlf":                "---\r\naliases: [A]\r\n---\r\n# Note\r\n\r\nBody [[Target]].\r\n",
		"no trailing":         "# Note\n\nEnds without a newline [[Target]]",
		"empty frontmatter":   "---\n---\n# Note\n\nBody.\n",
		"no frontmatter link": "Body [[Target]] only.",
		"large":               large.String(),
	}
}

func TestCanonicalBodyShapesAreStable(t *testing.T) {
	run := j32uRun()
	for name, body := range j32uNotes() {
		note := vaultFile{RelPath: "Folder/Note.md"}
		canonical, _, _, warnings := run.canonicalBody(note, []byte(body))
		if len(warnings) != 0 {
			t.Errorf("%s: unexpected warnings %v", name, warnings)
		}
		if !strings.HasPrefix(canonical, "---\nsource_system: obsidian\n") {
			t.Errorf("%s: canonical body does not start with the added frontmatter: %q",
				name, canonical[:min(80, len(canonical))])
		}
		if strings.Contains(canonical, "\r") {
			t.Errorf("%s: canonical body kept a carriage return", name)
		}
	}
}

func BenchmarkCanonicalBodyToday(b *testing.B) {
	run := j32uRun()
	note := vaultFile{RelPath: "Folder/Note.md"}
	for name, body := range j32uNotes() {
		raw := []byte(body)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for attempt := 0; attempt < b.N; attempt++ {
				if _, _, _, _ = run.canonicalBody(note, raw); false {
					b.Fatal("unreachable")
				}
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// stagedCanonicalBody is what canonicalBody did before J32-U: rewrite the
// links into a new body, then rebuild the frontmatter around it. It is the
// reference the one-pass assembly is held to, and the path canonicalBody still
// takes for the bodies the one pass declines.
func stagedCanonicalBody(run *importRun, note vaultFile, raw []byte) string {
	body := normalizeNewlines(string(raw))
	rewritten, _, _, _ := rewriteObsidianLinks(body, note.RelPath, run.options.CollectionID, run.namespace)
	return augmentFrontmatter(rewritten, note.RelPath)
}

// TestCanonicalBodyOnceMatchesStaged is the J32-U gate: the same bytes, for
// every shape and for randomly assembled notes.
func TestCanonicalBodyOnceMatchesStaged(t *testing.T) {
	run := j32uRun()
	notes := j32uNotes()

	pieces := []string{
		"---\n", "aliases: [A]\n", "unknown: kept\n", "---\n", "# Heading\n", "\n",
		"Body [[Target]] text.\n", "[[Other]]\n", "[text](Target.md)\n", "plain line\n",
		"\r\n", "ελληνικά [[Target|shown]]\n", "![[Target#^block]]\n", "- item [[Other]]\n",
		"```\ncode [[Target]]\n```\n", "trailing spaces   \n", "[[Missing]]\n", "text",
	}
	random := rand.New(rand.NewSource(32))
	for attempt := 0; attempt < 3000; attempt++ {
		var builder strings.Builder
		for length := random.Intn(10); length >= 0; length-- {
			builder.WriteString(pieces[random.Intn(len(pieces))])
		}
		notes[fmt.Sprintf("random-%d", attempt)] = builder.String()
	}

	for _, relPath := range []string{"Note.md", "Folder/Note.md", "A/B/C/Note.md"} {
		for name, body := range notes {
			note := vaultFile{RelPath: relPath}
			want := stagedCanonicalBody(run, note, []byte(body))
			got, _, _, _ := run.canonicalBody(note, []byte(body))
			if got != want {
				t.Fatalf("%s at %s:\n one pass %q\n   staged %q\n     body %q", name, relPath, got, want, body)
			}
		}
	}
}

// TestFrontmatterRegionMatchesTheSplitter holds the offsets the one-pass
// assembly works from to the splitter that returns the bytes.
func TestFrontmatterRegionMatchesTheSplitter(t *testing.T) {
	bodies := []string{
		"---\naliases: [A]\n---\nBody\n", "---\n---\nBody\n", "no frontmatter\n",
		"---\r\naliases: [A]\r\n---\r\nBody\r\n", "---\nunclosed\nBody\n", "",
		"---\n", "---\n---\n", "--- \n---\nBody\n", "text\n---\nnot frontmatter\n---\n",
	}
	for _, body := range bodies {
		frontmatter, rest, ok := splitFrontmatterBytes([]byte(body))
		start, end, restStart, regionOK := frontmatterRegion(body)
		if regionOK != ok {
			t.Fatalf("%q: region says %v, splitter says %v", body, regionOK, ok)
		}
		if !ok {
			continue
		}
		if body[start:end] != string(frontmatter) {
			t.Errorf("%q: region frontmatter %q, splitter %q", body, body[start:end], frontmatter)
		}
		if body[restStart:] != string(rest) {
			t.Errorf("%q: region rest %q, splitter %q", body, body[restStart:], rest)
		}
	}
}

func BenchmarkCanonicalBodyOnce(b *testing.B) {
	run := j32uRun()
	note := vaultFile{RelPath: "Folder/Note.md"}
	for name, body := range j32uNotes() {
		raw := []byte(body)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for attempt := 0; attempt < b.N; attempt++ {
				run.canonicalBody(note, raw)
			}
		})
	}
}

func BenchmarkCanonicalBodyStaged(b *testing.B) {
	run := j32uRun()
	note := vaultFile{RelPath: "Folder/Note.md"}
	for name, body := range j32uNotes() {
		raw := []byte(body)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for attempt := 0; attempt < b.N; attempt++ {
				stagedCanonicalBody(run, note, raw)
			}
		})
	}
}
