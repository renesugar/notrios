package syncbody

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func mergeBoth(t *testing.T, base, local, remote string) (Result, Result) {
	t.Helper()
	forward := Merge(base, local, remote, Limits{})
	reverse := Merge(base, remote, local, Limits{})
	if forward.Clean != reverse.Clean {
		t.Fatalf("merge is order dependent: local-first clean=%v remote-first clean=%v", forward.Clean, reverse.Clean)
	}
	if forward.Clean && forward.Merged != reverse.Merged {
		t.Fatalf("merge is order dependent:\nlocal-first:  %q\nremote-first: %q", forward.Merged, reverse.Merged)
	}
	return forward, reverse
}

func TestDisjointEditsMergeCleanly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		base, local, remote string
		want                string
	}{
		{
			name:   "separate-paragraphs",
			base:   "alpha\nbeta\ngamma\ndelta\n",
			local:  "alpha CHANGED\nbeta\ngamma\ndelta\n",
			remote: "alpha\nbeta\ngamma\ndelta CHANGED\n",
			want:   "alpha CHANGED\nbeta\ngamma\ndelta CHANGED\n",
		},
		{
			name:   "insert-at-both-ends",
			base:   "middle\n",
			local:  "top\nmiddle\n",
			remote: "middle\nbottom\n",
			want:   "top\nmiddle\nbottom\n",
		},
		{
			name:   "append-only-from-one-side",
			base:   "one\ntwo\n",
			local:  "one\ntwo\n",
			remote: "one\ntwo\nthree\n",
			want:   "one\ntwo\nthree\n",
		},
		{
			name:   "delete-and-edit-different-lines",
			base:   "a\nb\nc\nd\n",
			local:  "a\nc\nd\n",
			remote: "a\nb\nc\nD\n",
			want:   "a\nc\nD\n",
		},
		{
			name:   "identical-change-on-both-sides",
			base:   "a\nb\nc\n",
			local:  "a\nB\nc\n",
			remote: "a\nB\nc\n",
			want:   "a\nB\nc\n",
		},
		{
			name:   "no-trailing-newline-is-preserved",
			base:   "one\ntwo",
			local:  "ONE\ntwo",
			remote: "one\ntwo",
			want:   "ONE\ntwo",
		},
		{
			name:   "crlf-line-endings-survive",
			base:   "a\r\nb\r\nc\r\n",
			local:  "a\r\nB\r\nc\r\n",
			remote: "a\r\nb\r\nC\r\n",
			want:   "a\r\nB\r\nC\r\n",
		},
		{
			name:   "unicode-paragraphs",
			base:   "# Café 東京\n\nJournée normale.\n\n絵文字 😀\n",
			local:  "# Café 東京 2026\n\nJournée normale.\n\n絵文字 😀\n",
			remote: "# Café 東京\n\nJournée normale.\n\n絵文字 😀🎉\n",
			want:   "# Café 東京 2026\n\nJournée normale.\n\n絵文字 😀🎉\n",
		},
		{
			name:   "markdown-list-and-table",
			base:   "- one\n- two\n\n| a | b |\n|---|---|\n| 1 | 2 |\n",
			local:  "- one\n- two\n- three\n\n| a | b |\n|---|---|\n| 1 | 2 |\n",
			remote: "- one\n- two\n\n| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n",
			want:   "- one\n- two\n- three\n\n| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, _ := mergeBoth(t, test.base, test.local, test.remote)
			if !result.Clean {
				t.Fatalf("expected a clean merge, got %s: %+v", result.Kind, result.Regions)
			}
			if result.Merged != test.want {
				t.Fatalf("merged mismatch\n got: %q\nwant: %q", result.Merged, test.want)
			}
		})
	}
}

// Word refinement is the whole reason G1 chose line-first-plus-words over
// either alone: these cases conflict at line granularity and merge at word
// granularity.
func TestWordRefinementResolvesOneLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		base, local, remote string
		want                string
	}{
		{
			name:   "disjoint-words-on-one-line",
			base:   "the quick brown fox jumps over the lazy dog\n",
			local:  "the SWIFT brown fox jumps over the lazy dog\n",
			remote: "the quick brown fox jumps over the SLEEPY dog\n",
			want:   "the SWIFT brown fox jumps over the SLEEPY dog\n",
		},
		{
			name:   "neighbouring-unicode-words",
			base:   "café naïve résumé\n",
			local:  "CAFÉ naïve résumé\n",
			remote: "café naïve RÉSUMÉ\n",
			want:   "CAFÉ naïve RÉSUMÉ\n",
		},
		{
			name:   "punctuation-is-its-own-token",
			base:   "one, two, three\n",
			local:  "one; two, three\n",
			remote: "one, two, THREE\n",
			want:   "one; two, THREE\n",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, _ := mergeBoth(t, test.base, test.local, test.remote)
			if !result.Clean {
				t.Fatalf("word refinement should have resolved this: %s %+v", result.Kind, result.Regions)
			}
			if result.Merged != test.want {
				t.Fatalf("merged mismatch\n got: %q\nwant: %q", result.Merged, test.want)
			}
		})
	}
}

func TestOverlappingEditsStayVisibleConflicts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		base, local, remote string
		wantKind            string
		wantRefined         bool
	}{
		{
			name:     "same-token-rewritten",
			base:     "the quick brown fox\n",
			local:    "the SWIFT brown fox\n",
			remote:   "the RAPID brown fox\n",
			wantKind: ConflictSameToken, wantRefined: true,
		},
		{
			name:     "overlapping-markdown-url",
			base:     "See [docs](https://example.com/a).\n",
			local:    "See [docs](https://example.com/b).\n",
			remote:   "See [docs](https://example.com/c).\n",
			wantKind: ConflictSameToken, wantRefined: true,
		},
		{
			name:     "delete-versus-edit",
			base:     "keep\nremove me\nkeep\n",
			local:    "keep\nkeep\n",
			remote:   "keep\nremove me BUT EDITED\nkeep\n",
			wantKind: ConflictDeleteEdit, wantRefined: false,
		},
		{
			// Each side deleted a different line of the same two-line region.
			// Words cannot settle which lines exist, so refinement must not run.
			name:     "each-side-deletes-a-different-line",
			base:     "first\nsecond\n",
			local:    "first\n",
			remote:   "SECOND\n",
			wantKind: ConflictSameToken, wantRefined: false,
		},
		{
			name:     "whole-body-replaced-differently",
			base:     "a\nb\nc\n",
			local:    "x\ny\nz\n",
			remote:   "1\n2\n3\n",
			wantKind: ConflictSameToken, wantRefined: false,
		},
		{
			// Both replicas prefixed the same line. The two insertions land at
			// the same word position, so refinement runs and still conflicts
			// rather than concatenating both prefixes.
			name:     "both-sides-prefix-the-same-line",
			base:     "line five\n",
			local:    "L line five\n",
			remote:   "R line five\n",
			wantKind: ConflictSameToken, wantRefined: true,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, reverse := mergeBoth(t, test.base, test.local, test.remote)
			if result.Clean {
				t.Fatalf("overlapping edits merged silently into %q", result.Merged)
			}
			if result.Merged != "" {
				t.Fatalf("a conflicted merge must return no body, got %q", result.Merged)
			}
			if result.Kind != test.wantKind {
				t.Fatalf("kind = %q, want %q", result.Kind, test.wantKind)
			}
			if len(result.Regions) == 0 {
				t.Fatal("a conflict must report at least one region")
			}
			for _, region := range result.Regions {
				if region.Local == region.Remote {
					t.Fatalf("conflict region does not actually differ: %+v", region)
				}
				if region.Refined != test.wantRefined {
					t.Fatalf("region refined = %v, want %v", region.Refined, test.wantRefined)
				}
			}
			// The same conflict must be reported whichever side arrives first,
			// with local and remote swapped rather than reclassified.
			if len(reverse.Regions) != len(result.Regions) || reverse.Kind != result.Kind {
				t.Fatalf("conflict is order dependent: %+v vs %+v", result.Regions, reverse.Regions)
			}
			for index := range result.Regions {
				if reverse.Regions[index].Local != result.Regions[index].Remote ||
					reverse.Regions[index].Remote != result.Regions[index].Local ||
					reverse.Regions[index].Base != result.Regions[index].Base {
					t.Fatalf("swapped inputs did not swap the region: %+v vs %+v", result.Regions[index], reverse.Regions[index])
				}
			}
		})
	}
}

func TestConflictRegionsAreLocatedAndBounded(t *testing.T) {
	t.Parallel()
	base := "one\ntwo\nthree\nfour\nfive\n"
	local := "one\ntwo\nLOCAL\nfour\nfive\n"
	remote := "one\ntwo\nREMOTE\nfour\nfive\n"
	result := Merge(base, local, remote, Limits{})
	if result.Clean || len(result.Regions) != 1 {
		t.Fatalf("expected exactly one conflict region: %+v", result)
	}
	region := result.Regions[0]
	if region.BaseLine != 3 || region.LocalLine != 3 || region.RemoteLine != 3 {
		t.Fatalf("region located at base=%d local=%d remote=%d, want line 3 on all three", region.BaseLine, region.LocalLine, region.RemoteLine)
	}
	if region.Base != "three\n" || region.Local != "LOCAL\n" || region.Remote != "REMOTE\n" {
		t.Fatalf("region text mismatch: %+v", region)
	}

	long := strings.Repeat("x", 1024)
	wide := Merge("head\n"+long+"\n", "head\nLOCAL"+long+"\n", "head\nREMOTE"+long+"\n", Limits{MaxConflictTextBytes: 64})
	if wide.Clean {
		t.Fatal("expected a conflict")
	}
	if !wide.Regions[0].Truncated || len(wide.Regions[0].Local) > 64 {
		t.Fatalf("conflict text was not bounded: truncated=%v len=%d", wide.Regions[0].Truncated, len(wide.Regions[0].Local))
	}
}

func TestBoundsProduceTypedConflictsNotSilentTruncation(t *testing.T) {
	t.Parallel()
	base := strings.Repeat("line\n", 100)
	local := strings.Repeat("LOCAL\n", 100)
	remote := strings.Repeat("REMOTE\n", 100)

	tooBig := Merge(base, local, remote, Limits{MaxBodyBytes: 16})
	if tooBig.Clean || tooBig.Kind != ConflictBoundsExceeded {
		t.Fatalf("oversize body: %+v", tooBig)
	}
	tooManyLines := Merge(base, local, remote, Limits{MaxLineTokens: 4})
	if tooManyLines.Clean || tooManyLines.Kind != ConflictBoundsExceeded {
		t.Fatalf("line-token bound: %+v", tooManyLines)
	}
	tooFarApart := Merge(base, local, remote, Limits{MaxDiffDistance: 2})
	if tooFarApart.Clean || tooFarApart.Kind != ConflictBoundsExceeded {
		t.Fatalf("edit-distance bound: %+v", tooFarApart)
	}

	// A body that is far apart at line level but within the diff bound must
	// still refuse *refinement* rather than spend unbounded time on it.
	wide := strings.Repeat("word ", 4_000)
	unrefinable := Merge(wide+"\n", wide+"LOCAL\n", wide+"REMOTE\n", Limits{MaxRegionBytes: 128})
	if unrefinable.Clean {
		t.Fatal("expected a conflict when refinement is out of bounds")
	}
	if unrefinable.Regions[0].Refined {
		t.Fatal("refinement ran despite exceeding the region byte bound")
	}

	manyConflicts := strings.Builder{}
	manyLocal := strings.Builder{}
	manyRemote := strings.Builder{}
	for index := 0; index < 40; index++ {
		manyConflicts.WriteString(fmt.Sprintf("base %d\nspacer\n", index))
		manyLocal.WriteString(fmt.Sprintf("local %d\nspacer\n", index))
		manyRemote.WriteString(fmt.Sprintf("remote %d\nspacer\n", index))
	}
	capped := Merge(manyConflicts.String(), manyLocal.String(), manyRemote.String(), Limits{MaxConflictRegions: 4})
	if capped.Clean || capped.Kind != ConflictBoundsExceeded {
		t.Fatalf("conflict-region cap: %+v", capped)
	}
	if len(capped.Regions) != 1 {
		t.Fatalf("an exceeded region cap becomes one whole-body conflict, got %d regions", len(capped.Regions))
	}
}

func TestInvalidUTF8IsRefusedRatherThanMerged(t *testing.T) {
	t.Parallel()
	invalid := "valid\n\xff\xfe\n"
	for name, inputs := range map[string][3]string{
		"base":   {invalid, "valid\nlocal\n", "valid\nremote\n"},
		"local":  {"valid\nbase\n", invalid, "valid\nremote\n"},
		"remote": {"valid\nbase\n", "valid\nlocal\n", invalid},
	} {
		result := Merge(inputs[0], inputs[1], inputs[2], Limits{})
		if result.Clean || result.Kind != ConflictInvalidUTF8 {
			t.Fatalf("invalid UTF-8 in %s: %+v", name, result)
		}
	}
}

func TestTokenizersRoundTrip(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"", "\n", "\n\n\n", "no newline", "a\nb\n", "a\r\nb\r\n",
		"café 東京 😀\ttab  spaces\n", "one, two; three! [link](url)\n",
		"don't — do not\n", strings.Repeat("x", 1000),
	}
	for _, input := range inputs {
		if joined := joinTokens(SplitLines(input)); joined != input {
			t.Fatalf("SplitLines lost bytes for %q: %q", input, joined)
		}
		if joined := joinTokens(SplitWords(input)); joined != input {
			t.Fatalf("SplitWords lost bytes for %q: %q", input, joined)
		}
	}
}

// A merge that is clean must be a merge a person could have typed: it contains
// every line both sides kept and nothing either side deleted.
func TestRandomizedMergesNeverInventOrLoseContent(t *testing.T) {
	t.Parallel()
	for seed := int64(0); seed < 4_000; seed++ {
		random := rand.New(rand.NewSource(seed))
		lineCount := 5 + random.Intn(20)
		baseLines := make([]string, lineCount)
		for index := range baseLines {
			baseLines[index] = fmt.Sprintf("line %d word%d\n", index, random.Intn(5))
		}
		local := editLines(random, baseLines, "L")
		remote := editLines(random, baseLines, "R")
		base := strings.Join(baseLines, "")
		result := Merge(base, local, remote, Limits{})
		reverse := Merge(base, remote, local, Limits{})
		if result.Clean != reverse.Clean {
			t.Fatalf("seed %d: order changed the outcome", seed)
		}
		if !result.Clean {
			if result.Merged != "" {
				t.Fatalf("seed %d: conflicted merge returned a body", seed)
			}
			continue
		}
		if result.Merged != reverse.Merged {
			t.Fatalf("seed %d: order changed the merged body", seed)
		}
		for _, line := range SplitLines(result.Merged) {
			if !strings.Contains(base, line) && !strings.Contains(local, line) && !strings.Contains(remote, line) {
				t.Fatalf("seed %d: merged body invented %q", seed, line)
			}
		}
		// Anything both sides left untouched must survive.
		for _, line := range baseLines {
			if strings.Contains(local, line) && strings.Contains(remote, line) && !strings.Contains(result.Merged, line) {
				t.Fatalf("seed %d: merge dropped %q, which neither side changed", seed, line)
			}
		}
	}
}

func editLines(random *rand.Rand, baseLines []string, marker string) string {
	edited := append([]string(nil), baseLines...)
	for count := 0; count < 3; count++ {
		if len(edited) == 0 {
			break
		}
		position := random.Intn(len(edited))
		switch random.Intn(3) {
		case 0:
			edited[position] = marker + " " + edited[position]
		case 1:
			edited = append(edited[:position], edited[position+1:]...)
		default:
			inserted := fmt.Sprintf("%s inserted %d\n", marker, count)
			edited = append(edited[:position], append([]string{inserted}, edited[position:]...)...)
		}
	}
	return strings.Join(edited, "")
}
