package joplinraw

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

// referenceRewriteJoplinLinkLine is the byte-at-a-time loop J38-E replaced, kept
// so the run-skipping version can be held to it. The inline-code state machine
// and the escape rule are what make this worth pinning.
func referenceRewriteJoplinLinkLine(output *strings.Builder, line string, inlineCodeLength *int,
	noteIDMap, resourceIDMap map[string]string, collectionID string,
	result *linkRewriteResult, resources map[string]resourceReference) {
	for index := 0; index < len(line); {
		if line[index] == '`' {
			end := index + 1
			for end < len(line) && line[end] == '`' {
				end++
			}
			runLength := end - index
			if *inlineCodeLength == 0 {
				*inlineCodeLength = runLength
			} else if *inlineCodeLength == runLength {
				*inlineCodeLength = 0
			}
			output.WriteString(line[index:end])
			index = end
			continue
		}
		if *inlineCodeLength != 0 || index+2 > len(line) || line[index:index+2] != ":/" || isEscaped(line, index) {
			output.WriteByte(line[index])
			index++
			continue
		}
		end := index + 2
		for end < len(line) && isJoplinIDCharacter(line[end]) {
			end++
		}
		if end == index+2 {
			output.WriteString(":/")
			index += 2
			continue
		}
		id := line[index+2 : end]
		if documentID := noteIDMap[id]; documentID != "" {
			output.WriteString(store.DocumentURI(collectionID, documentID))
			result.Rewritten++
		} else if resourceID := resourceIDMap[id]; resourceID != "" {
			output.WriteString(store.ResourceURI(collectionID, resourceID))
			result.Rewritten++
			resources[resourceID] = resourceReference{SourceID: id, TargetID: resourceID}
		} else {
			output.WriteString(line[index:end])
			result.Unresolved++
		}
		index = end
	}
}

func referenceSplitPhysicalLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := make([]string, 0, strings.Count(text, "\n")+1)
	start := 0
	for index := 0; index < len(text); index++ {
		if text[index] != '\r' && text[index] != '\n' {
			continue
		}
		lines = append(lines, text[start:index])
		if text[index] == '\r' && index+1 < len(text) && text[index+1] == '\n' {
			index++
		}
		start = index + 1
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

// j38eLines covers what the two scans have to agree about: links resolved to a
// note and to a resource, unresolved ones, escaped colons, inline code of every
// backtick run length, code that never closes, a colon with nothing after it,
// and multi-byte text.
func j38eLines() []string {
	lines := []string{
		"", ":", ":/", ":/abc", "text :/note1 text", "text :/res1 text",
		"text :/unknown text", "escaped \\:/note1", "double \\\\:/note1",
		"`code :/note1`", "``code :/note1``", "`unclosed :/note1",
		"a `b` :/note1 `c` :/res1", "```\n:/note1\n```", ":/note1:/res1",
		"ελληνικά :/note1 ünïcode", "colon: not a link", ":/-_09AZ",
		"`:`", "``:``", "`` ` ``", ":/note1 at the end", "at the start :/note1",
		strings.Repeat("plain text with no links at all ", 40),
		strings.Repeat("x :/note1 ", 30),
	}
	pieces := []string{":", "/", "`", "\\", "note1", "res1", "unknown", " ", "a", "ü", "\r", "\n", "-", "_"}
	random := rand.New(rand.NewSource(38))
	for attempt := 0; attempt < 20000; attempt++ {
		var builder strings.Builder
		for length := random.Intn(14); length >= 0; length-- {
			builder.WriteString(pieces[random.Intn(len(pieces))])
		}
		lines = append(lines, builder.String())
	}
	return lines
}

// TestRewriteJoplinLinkLineMatchesReference holds the run-skipping rewriter to
// the byte-at-a-time loop: same output, same counts, same resources, and the
// same inline-code state carried out of the line (v1.0 J38-E).
func TestRewriteJoplinLinkLineMatchesReference(t *testing.T) {
	notes := map[string]string{"note1": "doc_note1"}
	resources := map[string]string{"res1": "res_one"}
	for _, line := range j38eLines() {
		for _, startingCode := range []int{0, 1, 2} {
			var gotOut, wantOut strings.Builder
			gotCode, wantCode := startingCode, startingCode
			gotResult, wantResult := linkRewriteResult{}, linkRewriteResult{}
			gotResources := map[string]resourceReference{}
			wantResources := map[string]resourceReference{}

			rewriteJoplinLinkLine(&gotOut, line, &gotCode, notes, resources, "default", &gotResult, gotResources)
			referenceRewriteJoplinLinkLine(&wantOut, line, &wantCode, notes, resources, "default", &wantResult, wantResources)

			if gotOut.String() != wantOut.String() {
				t.Fatalf("line %q (code %d):\n got  %q\n want %q", line, startingCode, gotOut.String(), wantOut.String())
			}
			if gotCode != wantCode {
				t.Fatalf("line %q (code %d): inline code state %d, want %d", line, startingCode, gotCode, wantCode)
			}
			if gotResult.Rewritten != wantResult.Rewritten || gotResult.Unresolved != wantResult.Unresolved {
				t.Fatalf("line %q (code %d): %d rewritten and %d unresolved, want %d and %d",
					line, startingCode, gotResult.Rewritten, gotResult.Unresolved,
					wantResult.Rewritten, wantResult.Unresolved)
			}
			if len(gotResources) != len(wantResources) {
				t.Fatalf("line %q: resources %v, want %v", line, gotResources, wantResources)
			}
		}
	}
}

// TestSplitPhysicalLinesMatchesReference holds the jumping splitter to the
// per-byte one over every line ending shape.
func TestSplitPhysicalLinesMatchesReference(t *testing.T) {
	texts := append(j38eLines(),
		"", "\n", "\r", "\r\n", "a\nb", "a\r\nb", "a\rb", "a\n\nb", "a\r\n\r\nb",
		"trailing\n", "trailing\r\n", "no trailing", "\n\nleading")
	for _, text := range texts {
		got, want := splitPhysicalLines(text), referenceSplitPhysicalLines(text)
		if len(got) != len(want) {
			t.Fatalf("%q: %d lines, want %d (%q against %q)", text, len(got), len(want), got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("%q line %d: %q, want %q", text, index, got[index], want[index])
			}
		}
	}
}

func benchmarkRewrite(b *testing.B, rewrite func(*strings.Builder, string, *int, map[string]string, map[string]string, string, *linkRewriteResult, map[string]resourceReference)) {
	line := strings.Repeat("a paragraph of ordinary prose with no links in it at all ", 20) + " :/note1"
	notes := map[string]string{"note1": "doc_note1"}
	resources := map[string]string{}
	b.SetBytes(int64(len(line)))
	b.ReportAllocs()
	b.ResetTimer()
	for attempt := 0; attempt < b.N; attempt++ {
		var out strings.Builder
		code := 0
		result := linkRewriteResult{}
		rewrite(&out, line, &code, notes, resources, "default", &result, map[string]resourceReference{})
	}
}

func BenchmarkRewriteJoplinLinkLine(b *testing.B) {
	b.Run("runs", func(b *testing.B) { benchmarkRewrite(b, rewriteJoplinLinkLine) })
	b.Run("bytes", func(b *testing.B) { benchmarkRewrite(b, referenceRewriteJoplinLinkLine) })
}

func BenchmarkSplitPhysicalLines(b *testing.B) {
	text := strings.Repeat(fmt.Sprintf("%s\n", strings.Repeat("word ", 12)), 20000)
	b.SetBytes(int64(len(text)))
	b.Run("jump", func(b *testing.B) {
		b.ReportAllocs()
		for attempt := 0; attempt < b.N; attempt++ {
			splitPhysicalLines(text)
		}
	})
	b.Run("bytes", func(b *testing.B) {
		b.ReportAllocs()
		for attempt := 0; attempt < b.N; attempt++ {
			referenceSplitPhysicalLines(text)
		}
	})
}
