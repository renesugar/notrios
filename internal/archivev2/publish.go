package archivev2

import (
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// redactedPlaceholder replaces a link whose target the publication excludes
// when the profile asks for redaction rather than plain text. It says a link
// was removed instead of pretending the sentence never had one.
const redactedPlaceholder = "[redacted]"

// linkRewrite is one span of a note body the publication projection replaces.
type linkRewrite struct {
	start       int
	end         int
	displayText string
	rawTarget   string
}

// offsetDelta records how one applied rewrite moved every byte after it, so
// the link records the projection keeps still point at the published bytes
// rather than at the canonical ones.
type offsetDelta struct {
	after int
	delta int
}

// shiftOffset maps a canonical byte offset onto the published body.
func shiftOffset(offset int, deltas []offsetDelta) int {
	shifted := offset
	for _, delta := range deltas {
		if offset >= delta.after {
			shifted += delta.delta
		}
	}
	if shifted < 0 {
		return 0
	}
	return shifted
}

// rewriteBodyLinks replaces the given link spans in a note body.
//
// Publication is the one export that changes note content, so it does so under
// two rules. Spans are applied from the end backwards, because replacing an
// earlier span would invalidate every later offset. And a span is applied only
// when the bytes still look like the link the record describes: link offsets
// are recorded when a note is saved, and a body the offsets no longer match
// must be left alone rather than cut at an arbitrary position. A skipped span
// is reported, never silently dropped.
func rewriteBodyLinks(body string, action string, rewrites []linkRewrite) (string, int, int, []offsetDelta) {
	if len(rewrites) == 0 {
		return body, 0, 0, nil
	}
	ordered := make([]linkRewrite, len(rewrites))
	copy(ordered, rewrites)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].start > ordered[j].start })

	applied, skipped := 0, 0
	deltas := []offsetDelta{}
	previousStart := len(body) + 1
	for _, rewrite := range ordered {
		if rewrite.start < 0 || rewrite.end > len(body) || rewrite.start >= rewrite.end {
			skipped++
			continue
		}
		if rewrite.end > previousStart {
			// Overlapping spans cannot both be applied coherently. This should
			// not happen for links parsed from one body, so treat it as a
			// reason to leave the note alone rather than to guess.
			skipped++
			continue
		}
		span := body[rewrite.start:rewrite.end]
		if !spanLooksLikeLink(span, rewrite) {
			skipped++
			continue
		}
		replacement := replacementFor(action, rewrite)
		deltas = append(deltas, offsetDelta{after: rewrite.end, delta: len(replacement) - (rewrite.end - rewrite.start)})
		body = body[:rewrite.start] + replacement + body[rewrite.end:]
		previousStart = rewrite.start
		applied++
	}
	return body, applied, skipped, deltas
}

// spanLooksLikeLink is the safety check that keeps a stale offset from cutting
// a hole in an unrelated sentence. It is deliberately loose about syntax and
// strict about identity: the span must still contain the target the link
// record names.
func spanLooksLikeLink(span string, rewrite linkRewrite) bool {
	if !strings.HasPrefix(span, "[") && !strings.HasPrefix(span, "![") {
		return false
	}
	target := strings.TrimSpace(rewrite.rawTarget)
	if target == "" {
		// A link with no raw target (a bare anchor) is identified by its
		// display text instead.
		return rewrite.displayText == "" || strings.Contains(span, rewrite.displayText)
	}
	return strings.Contains(span, target)
}

func replacementFor(action string, rewrite linkRewrite) string {
	if action == store.SelectionLinkActionRedact {
		return redactedPlaceholder
	}
	if text := strings.TrimSpace(rewrite.displayText); text != "" {
		return rewrite.displayText
	}
	// An image or bare link with no display text leaves nothing readable
	// behind; the placeholder keeps the sentence honest.
	return redactedPlaceholder
}

// publicationLinkNeedsRewrite decides whether one link's syntax survives into
// the published projection.
//
// Links to notes and resources the publication includes are kept: they are the
// internal structure of the published set. Everything else — a link to a note
// held back as private, a link to a resource the policy excluded, and any link
// that never resolved — is rewritten, because a published note must not carry
// a reference a reader cannot follow and must not name a note that was
// deliberately withheld. External http(s) links are untouched.
func publicationLinkNeedsRewrite(link store.DocumentLink, includedDocuments, includedResources []string) bool {
	switch strings.ToLower(strings.TrimSpace(link.ResolutionStatus)) {
	case "external":
		return false
	case "resolved":
		if link.TargetDocumentID != "" {
			return !containsSorted(includedDocuments, link.TargetDocumentID)
		}
		if link.TargetResourceID != "" {
			return !containsSorted(includedResources, link.TargetResourceID)
		}
		// A resolved link with no target ID is a self-anchor; it stays.
		return false
	default:
		// unresolved, ambiguous, invalid, target_deleted: broken for a reader.
		return true
	}
}
