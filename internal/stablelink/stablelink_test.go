package stablelink

import (
	"errors"
	"strings"
	"testing"
)

func TestParseAcceptsTheDocumentedShape(t *testing.T) {
	link, err := Parse("notrios://databases/db_abc123/documents/doc_xyz789")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if link.DatabaseID != "db_abc123" || link.DocumentID != "doc_xyz789" || link.Anchor != "" {
		t.Fatalf("unexpected link: %+v", link)
	}
	if got := link.String(); got != "notrios://databases/db_abc123/documents/doc_xyz789" {
		t.Fatalf("round trip: %q", got)
	}
}

func TestParsePreservesAnchors(t *testing.T) {
	link, err := Parse("notrios://databases/db_a/documents/doc_b#heading-two")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if link.Anchor != "heading-two" {
		t.Fatalf("anchor: %q", link.Anchor)
	}
	if got := link.String(); got != "notrios://databases/db_a/documents/doc_b#heading-two" {
		t.Fatalf("round trip: %q", got)
	}
}

// The scheme and authority are case-insensitive per RFC 3986; the identifiers
// are not, because folding them could make two distinct opaque IDs collide.
func TestParseFoldsSchemeAndAuthorityButNotIdentifiers(t *testing.T) {
	link, err := Parse("NOTRIOS://DataBases/db_Case/DOCUMENTS/doc_Case")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if link.DatabaseID != "db_Case" || link.DocumentID != "doc_Case" {
		t.Fatalf("identifiers were folded: %+v", link)
	}
}

func TestParseDistinguishesForeignSchemes(t *testing.T) {
	for _, raw := range []string{
		"document://default/documents/doc_a",
		"https://example.com/notes/1",
		"mailto:someone@example.com",
		"notriosx://databases/db_a/documents/doc_b",
		"",
	} {
		if _, err := Parse(raw); !errors.Is(err, ErrNotStableLink) {
			t.Fatalf("%q: expected ErrNotStableLink, got %v", raw, err)
		}
	}
}

func TestParseRejectsMalformedShapes(t *testing.T) {
	cases := map[string]string{
		"missing route":       "notrios://databases/db_a/doc_b",
		"extra segment":       "notrios://databases/db_a/documents/doc_b/extra",
		"trailing slash":      "notrios://databases/db_a/documents/doc_b/",
		"empty database":      "notrios://databases//documents/doc_b",
		"empty document":      "notrios://databases/db_a/documents/",
		"wrong authority":     "notrios://profiles/db_a/documents/doc_b",
		"query string":        "notrios://databases/db_a/documents/doc_b?intent=replace",
		"percent escape":      "notrios://databases/db_a/documents/doc%5Fb",
		"path traversal":      "notrios://databases/db_a/documents/../../etc/passwd",
		"backslash":           `notrios://databases/db_a/documents/doc_b\..\x`,
		"embedded newline":    "notrios://databases/db_a/documents/doc_b\nnotrios://databases/db_c/documents/doc_d",
		"space in identifier": "notrios://databases/db a/documents/doc_b",
	}
	for name, raw := range cases {
		_, err := Parse(raw)
		if err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
		if errors.Is(err, ErrNotStableLink) {
			t.Fatalf("%s: should be reported as malformed, not as a foreign scheme", name)
		}
	}
}

func TestParseRejectsUnsupportedRoutesByName(t *testing.T) {
	_, err := Parse("notrios://databases/db_a/resources/res_b")
	if !errors.Is(err, ErrUnsupportedRoute) {
		t.Fatalf("expected ErrUnsupportedRoute, got %v", err)
	}
}

func TestParseEnforcesLengthBounds(t *testing.T) {
	long := "notrios://databases/db_a/documents/" + strings.Repeat("d", MaxIDBytes+1)
	if _, err := Parse(long); !errors.Is(err, ErrTooLong) {
		t.Fatalf("oversized ID: %v", err)
	}
	huge := "notrios://databases/db_a/documents/" + strings.Repeat("d", MaxURIBytes)
	if _, err := Parse(huge); !errors.Is(err, ErrTooLong) {
		t.Fatalf("oversized URI: %v", err)
	}
	anchored := "notrios://databases/db_a/documents/doc_b#" + strings.Repeat("a", MaxAnchorBytes+1)
	if _, err := Parse(anchored); !errors.Is(err, ErrTooLong) {
		t.Fatalf("oversized anchor: %v", err)
	}
}

func TestParseRejectsHostileAnchors(t *testing.T) {
	for _, raw := range []string{
		"notrios://databases/db_a/documents/doc_b#with space",
		"notrios://databases/db_a/documents/doc_b#with\x00null",
		"notrios://databases/db_a/documents/doc_b#a/b",
	} {
		if _, err := Parse(raw); !errors.Is(err, ErrMalformed) {
			t.Fatalf("%q: expected ErrMalformed, got %v", raw, err)
		}
	}
}

func TestFormatMatchesParse(t *testing.T) {
	raw := Format("db_round", "doc_trip")
	link, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse own output: %v", err)
	}
	if link.DatabaseID != "db_round" || link.DocumentID != "doc_trip" {
		t.Fatalf("unexpected link: %+v", link)
	}
}

func TestIsURITargetRecognizesOnlyNotriosSchemes(t *testing.T) {
	for _, target := range []string{
		"notrios://databases/db_a/documents/doc_b",
		"document://default/documents/doc_b",
		"resource://default/resources/res_b",
		"DOCUMENT://default/documents/doc_b",
	} {
		if !IsURITarget(target) {
			t.Fatalf("%q should be a URI target", target)
		}
	}
	// A bare Markdown target is not a URI, so nothing in it may be reread as an
	// escape.
	for _, target := range []string{
		"", "Kitchen", "Kitchen%20Plan", "notes/kitchen.md",
		"https://example.com", "mailto:someone@example.com",
	} {
		if IsURITarget(target) {
			t.Fatalf("%q must not be treated as a Notrios URI target", target)
		}
	}
}

func TestDecodeAnchor(t *testing.T) {
	cases := map[string]string{
		"Kitchen%20Plan":   "Kitchen Plan",
		"kitchen-plan":     "kitchen-plan",
		"100%25%20off":     "100% off",
		"caf%C3%A9":        "café",
		"%2Fslash":         "/slash",
		"lower%2fcase-hex": "lower/case-hex",
		"":                 "",
	}
	for encoded, want := range cases {
		if got := DecodeAnchor(encoded); got != want {
			t.Fatalf("DecodeAnchor(%q) = %q, want %q", encoded, got, want)
		}
	}

	// An invalid or truncated escape is left exactly as written. A heading with
	// a literal percent sign is far likelier than a typo in an escape, and
	// deleting bytes would break the link invisibly.
	for _, literal := range []string{"100%", "%zz", "%2", "50%off", "a%g0b"} {
		if got := DecodeAnchor(literal); got != literal {
			t.Fatalf("DecodeAnchor(%q) = %q, want it unchanged", literal, got)
		}
	}

	// A literal percent followed by a valid escape is decided per sequence: the
	// first % is not a valid escape and stays, the second one decodes. This is
	// what every lenient decoder does, including Python's urllib.
	if got := DecodeAnchor("%%20"); got != "% " {
		t.Fatalf("DecodeAnchor(%q) = %q, want %q", "%%20", got, "% ")
	}

	// `+` is form encoding, not fragment encoding: a heading with a plus keeps
	// it.
	if got := DecodeAnchor("c+%2B+rules"); got != "c+++rules" {
		t.Fatalf("plus handling: %q", got)
	}
}

// Decoding must not become a second way to write a document ID: only the
// anchor is decoded, and the ID validator still refuses escapes outright.
func TestPercentEscapesStillRejectedInIdentifiers(t *testing.T) {
	if _, err := Parse("notrios://databases/db_a/documents/doc%5Fb"); !errors.Is(err, ErrMalformed) {
		t.Fatalf("an escaped identifier must stay refused, got %v", err)
	}
	link, err := Parse("notrios://databases/db_a/documents/doc_b#Kitchen%20Plan")
	if err != nil {
		t.Fatalf("an escaped anchor is accepted: %v", err)
	}
	// Parse keeps the anchor as written so the link round-trips byte for byte;
	// decoding is a resolution-time reading of it.
	if link.Anchor != "Kitchen%20Plan" {
		t.Fatalf("anchor: %q", link.Anchor)
	}
	if link.String() != "notrios://databases/db_a/documents/doc_b#Kitchen%20Plan" {
		t.Fatalf("round trip: %q", link.String())
	}
	if DecodeAnchor(link.Anchor) != "Kitchen Plan" {
		t.Fatalf("decoded: %q", DecodeAnchor(link.Anchor))
	}
}
