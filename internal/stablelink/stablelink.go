// Package stablelink parses and formats the external Notrios link:
//
//	notrios://databases/{database_id}/documents/{document_id}[#anchor]
//
// The link carries the logical database identity rather than a profile name,
// a filesystem path, or a hostname, so it keeps working after the database
// moves, the profile is renamed, or the note is opened on another machine that
// holds a copy of the same logical database.
//
// Parsing is deliberately strict and hand-written rather than delegated to
// net/url. This value arrives from outside the application — a clicked link, a
// pasted string, an OS protocol handler — and the only safe reading is one
// that either matches the documented shape exactly or fails. net/url accepts a
// great deal more than that shape.
package stablelink

import (
	"errors"
	"fmt"
	"strings"
)

// Scheme is the URI scheme registered for external Notrios links.
const Scheme = "notrios"

// authority is the fixed authority component. It names the route family so a
// later link type (a notebook, a search) can be added without ambiguity.
const authority = "databases"

// routeDocuments is the only route this version understands. Others are
// rejected by name rather than ignored, so an older build never silently
// mishandles a link a newer build wrote.
const routeDocuments = "documents"

// Limits bound what an external caller can hand us. IDs are opaque
// application identifiers (`db_…`, `doc_…`), which are far shorter than these
// bounds; the bounds exist to stop a hostile or corrupt input, not to describe
// a real identifier.
const (
	MaxURIBytes    = 512
	MaxIDBytes     = 128
	MaxAnchorBytes = 256
)

var (
	// ErrNotStableLink means the value is not a notrios:// URI at all. It is
	// distinct from ErrMalformed so a caller handling mixed link types can
	// fall through to its other schemes instead of reporting an error.
	ErrNotStableLink = errors.New("not a notrios link")
	// ErrMalformed means the value claims to be a notrios:// URI but does not
	// match the documented shape.
	ErrMalformed = errors.New("malformed notrios link")
	// ErrTooLong means the value exceeds a documented bound.
	ErrTooLong = errors.New("notrios link exceeds length limits")
	// ErrUnsupportedRoute means the shape is well formed but names a route
	// this build does not implement.
	ErrUnsupportedRoute = errors.New("unsupported notrios link route")
)

// Link is a parsed external link. Anchor is the optional fragment without its
// leading '#'.
type Link struct {
	DatabaseID string
	DocumentID string
	Anchor     string
}

// String renders the canonical form of the link.
func (l Link) String() string {
	if l.Anchor == "" {
		return Format(l.DatabaseID, l.DocumentID)
	}
	return Format(l.DatabaseID, l.DocumentID) + "#" + l.Anchor
}

// Format renders a stable document link. It does not validate: callers hold
// canonical identifiers, and a formatter that could fail would push error
// handling into every display path. Parse is where validation belongs.
func Format(databaseID, documentID string) string {
	return fmt.Sprintf("%s://%s/%s/%s/%s", Scheme, authority, databaseID, routeDocuments, documentID)
}

// HasScheme reports whether the value looks like a notrios URI. It is the
// cheap check a link router uses before committing to Parse and its errors.
func HasScheme(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), Scheme+"://")
}

// Parse reads a stable link. Every rejection is a distinct sentinel so a
// caller can tell "this is someone else's URI" from "this is a broken Notrios
// URI", and so the OS handler can report an actionable reason rather than a
// generic failure.
func Parse(raw string) (Link, error) {
	value := strings.TrimSpace(raw)
	if len(value) > MaxURIBytes {
		return Link{}, fmt.Errorf("%w: %d bytes exceeds %d", ErrTooLong, len(value), MaxURIBytes)
	}
	if !HasScheme(value) {
		return Link{}, ErrNotStableLink
	}
	rest := value[len(Scheme)+len("://"):]

	// A fragment is the only permitted suffix. A query string is rejected
	// rather than ignored: a link carrying one means something the reader
	// does not understand, and dropping it would open a target the writer did
	// not name.
	anchor := ""
	if idx := strings.IndexByte(rest, '#'); idx >= 0 {
		anchor = rest[idx+1:]
		rest = rest[:idx]
	}
	if strings.ContainsAny(rest, "?") {
		return Link{}, fmt.Errorf("%w: query strings are not part of a stable link", ErrMalformed)
	}
	if err := validateAnchor(anchor); err != nil {
		return Link{}, err
	}

	segments := strings.Split(rest, "/")
	if len(segments) != 4 {
		return Link{}, fmt.Errorf("%w: expected %s://%s/{database_id}/%s/{document_id}", ErrMalformed, Scheme, authority, routeDocuments)
	}
	if !strings.EqualFold(segments[0], authority) {
		return Link{}, fmt.Errorf("%w: authority must be %q", ErrMalformed, authority)
	}
	databaseID := segments[1]
	route := segments[2]
	documentID := segments[3]
	if !strings.EqualFold(route, routeDocuments) {
		return Link{}, fmt.Errorf("%w: %q", ErrUnsupportedRoute, route)
	}
	if err := validateID("database ID", databaseID); err != nil {
		return Link{}, err
	}
	if err := validateID("document ID", documentID); err != nil {
		return Link{}, err
	}
	return Link{DatabaseID: databaseID, DocumentID: documentID, Anchor: anchor}, nil
}

// validateID keeps identifiers to the opaque-ASCII alphabet Notrios mints.
// Percent-encoding is rejected rather than decoded: an ID that needs escaping
// is not an ID this application produced, and decoding would let two spellings
// name one document.
func validateID(label, value string) error {
	if value == "" {
		return fmt.Errorf("%w: empty %s", ErrMalformed, label)
	}
	if len(value) > MaxIDBytes {
		return fmt.Errorf("%w: %s is %d bytes, limit %d", ErrTooLong, label, len(value), MaxIDBytes)
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_' || c == '-' || c == '.':
		default:
			return fmt.Errorf("%w: %s contains an unsupported character", ErrMalformed, label)
		}
	}
	return nil
}

// validateAnchor allows a UTF-8 heading or block anchor but no control
// characters, whitespace, or separators that would let one link look like two.
func validateAnchor(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > MaxAnchorBytes {
		return fmt.Errorf("%w: anchor is %d bytes, limit %d", ErrTooLong, len(value), MaxAnchorBytes)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: anchor contains a control character", ErrMalformed)
		}
		if r == ' ' || r == '\t' || r == '/' || r == '#' || r == '?' {
			return fmt.Errorf("%w: anchor contains an unsupported character", ErrMalformed)
		}
	}
	return nil
}

// URISchemes are the link schemes Notrios mints. Percent-encoding is defined
// for URIs, so a target carrying one of these is the only place an escape
// sequence may be read as an escape rather than as literal text.
var URISchemes = []string{Scheme + "://", "document://", "resource://"}

// IsURITarget reports whether a link target is one of Notrios' URI schemes.
//
// It is the signal that decides whether an anchor's `%XX` sequences mean
// anything. A bare Markdown target is not a URI: `[x](#100%20off)` is text the
// author typed, and reinterpreting it would be guessing. A URI-schemed target
// declared itself, and RFC 3986 already says what `%20` means there.
func IsURITarget(target string) bool {
	lower := strings.ToLower(strings.TrimSpace(target))
	for _, scheme := range URISchemes {
		if strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	return false
}

// DecodeAnchor resolves percent-escapes in an anchor that arrived inside a
// URI-schemed link, so `Kitchen%20Plan` and `Kitchen Plan` name one heading.
//
// An invalid or truncated escape is left exactly as written rather than
// dropped: `%zz` and a trailing `%` are literal text in someone's heading far
// more often than they are a typo in an escape, and silently deleting bytes
// from an anchor would make a link fail in a way nobody could see.
//
// Only `%XX` is decoded. `+` stays a plus: that is form encoding, not fragment
// encoding, and treating it as a space would break every heading with one.
func DecodeAnchor(anchor string) string {
	if !strings.ContainsRune(anchor, '%') {
		return anchor
	}
	var out strings.Builder
	out.Grow(len(anchor))
	for i := 0; i < len(anchor); i++ {
		if anchor[i] != '%' || i+2 >= len(anchor) {
			out.WriteByte(anchor[i])
			continue
		}
		high, highOK := hexValue(anchor[i+1])
		low, lowOK := hexValue(anchor[i+2])
		if !highOK || !lowOK {
			out.WriteByte(anchor[i])
			continue
		}
		out.WriteByte(high<<4 | low)
		i += 2
	}
	return out.String()
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}
