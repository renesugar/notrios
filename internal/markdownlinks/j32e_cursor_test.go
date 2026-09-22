package markdownlinks

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// The shapes a column counted in runes since the last newline can meet
// (v1.0 J32-E).
var j32eBodies = map[string]string{
	"plain":            "one [[A]] two\nthree [[B]] four\n",
	"multi-byte":       "ünïcode ελληνικά [[A]] ελληνικά\nsecond ünïcode [[B]] line\n",
	"crlf":             "one [[A]] two\r\nthree [[B]] four\r\n",
	"no trailing":      "one [[A]] two\nthree [[B]] four",
	"one long line":    strings.Repeat("word ελληνικά [[A]] more ", 200),
	"empty":            "",
	"newlines only":    "\n\n\n\n",
	"leading newline":  "\n[[A]] after a newline\n",
	"link at byte one": "[[A]] first\n",
}

// TestLineCursorAgreesWithLineColumn holds the cursor to the reading it
// replaced, at every byte of every shape: same line, same column, whatever
// order the offsets are asked in.
func TestLineCursorAgreesWithLineColumn(t *testing.T) {
	for name, body := range j32eBodies {
		t.Run(name, func(t *testing.T) {
			cursor := newLineCursor(body)
			for offset := 0; offset <= len(body); offset++ {
				wantLine, wantColumn := lineColumn(body, offset)
				gotLine, gotColumn := cursor.at(offset)
				if gotLine != wantLine || gotColumn != wantColumn {
					t.Fatalf("offset %d: cursor says %d:%d, lineColumn says %d:%d",
						offset, gotLine, gotColumn, wantLine, wantColumn)
				}
			}
		})
	}
}

// TestLineCursorOutOfOrder proves the cursor does not depend on the order it is
// asked, which is what makes it safe for a caller that is not strictly
// ascending.
func TestLineCursorOutOfOrder(t *testing.T) {
	body := j32eBodies["multi-byte"] + j32eBodies["one long line"]
	random := rand.New(rand.NewSource(32))
	cursor := newLineCursor(body)
	for attempt := 0; attempt < 2000; attempt++ {
		offset := random.Intn(len(body) + 1)
		wantLine, wantColumn := lineColumn(body, offset)
		gotLine, gotColumn := cursor.at(offset)
		if gotLine != wantLine || gotColumn != wantColumn {
			t.Fatalf("offset %d: cursor says %d:%d, lineColumn says %d:%d",
				offset, gotLine, gotColumn, wantLine, wantColumn)
		}
	}
}

// TestExtractCoordinatesUnchanged states what Extract reports for bodies whose
// links sit on several lines and among multi-byte text: the coordinates a
// document_links row stores.
func TestExtractCoordinatesUnchanged(t *testing.T) {
	body := "first [[A]] line\nünïcode ελληνικά [[B]] here\n\n[text](C.md) at line four\n"
	want := []struct {
		raw    string
		line   int
		column int
		start  int
	}{
		{"C.md", 4, 1, 56},
		{"A", 1, 7, 6},
		{"B", 2, 18, 44},
	}
	got := Extract(body)
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d", len(got), len(want))
	}
	for index, expected := range want {
		candidate := got[index]
		if candidate.RawTarget != expected.raw || candidate.Line != expected.line ||
			candidate.Column != expected.column || candidate.StartByte != expected.start {
			t.Errorf("candidate %d: %q at %d:%d byte %d; want %q at %d:%d byte %d",
				index, candidate.RawTarget, candidate.Line, candidate.Column, candidate.StartByte,
				expected.raw, expected.line, expected.column, expected.start)
		}
	}
}

// TestExtractManyLinksCoordinates checks a body with many links on many lines,
// where a cursor that advanced wrongly would drift rather than fail outright.
func TestExtractManyLinksCoordinates(t *testing.T) {
	lines := make([]string, 0, 500)
	for index := 0; index < 500; index++ {
		lines = append(lines, fmt.Sprintf("ελληνικά line %d [[Target]] tail", index))
	}
	body := strings.Join(lines, "\n") + "\n"
	for _, candidate := range Extract(body) {
		wantLine, wantColumn := lineColumn(body, candidate.StartByte)
		if candidate.Line != wantLine || candidate.Column != wantColumn {
			t.Fatalf("byte %d reported %d:%d, want %d:%d",
				candidate.StartByte, candidate.Line, candidate.Column, wantLine, wantColumn)
		}
	}
}
