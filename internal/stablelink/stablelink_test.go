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
