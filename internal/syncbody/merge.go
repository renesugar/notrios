// Package syncbody implements G7's bounded three-way note-body merge and the
// revision-graph rules around it. Like the G5 state core and the G6 metadata
// core, it deliberately knows nothing about SQLite, HTTP, folders, or
// cryptography: it is given three bodies and returns either merged bytes or a
// typed conflict, and the caller decides what to persist.
//
// The behavior implemented here is the one G1 selected after measuring line,
// word, and byte granularities against real corpora: line-first merging, with
// Unicode-aware word-token refinement applied only to a bounded conflicting
// region. Byte merging was rejected — it was slower on the small cases, needed
// a low token limit on long lines, and offered no semantic boundary that word
// refinement did not already provide.
package syncbody

import (
	"unicode/utf8"
)

// Conflict kinds. Each names why the merge stopped, because "conflict" alone
// does not tell a reader whether to edit the note, split it, or raise a limit.
const (
	// ConflictSameToken is the ordinary overlap: both replicas changed the same
	// line, and word refinement could not separate the two edits either.
	ConflictSameToken = "same_token"
	// ConflictDeleteEdit is one replica removing text the other rewrote. It is
	// kept distinct because no merged text can honour both intents.
	ConflictDeleteEdit = "delete_edit"
	// ConflictBoundsExceeded means the inputs were too large or too far apart
	// for a bounded merge. It is a statement about this merge attempt, not
	// about the notes: raising a limit or editing either side can resolve it.
	ConflictBoundsExceeded = "bounds_exceeded"
	// ConflictInvalidUTF8 means a body was not valid UTF-8. Notrios note bodies
	// are text, and merging bytes that are not text would produce a body no
	// editor can safely display.
	ConflictInvalidUTF8 = "invalid_utf8"
)

// Limits bounds every merge before allocation-heavy work begins.
type Limits struct {
	MaxBodyBytes         int
	MaxLineTokens        int
	MaxDiffDistance      int
	MaxRegionBytes       int
	MaxWordTokens        int
	MaxConflictRegions   int
	MaxConflictTextBytes int
}

// DefaultLimits are the production bounds.
func DefaultLimits() Limits {
	return Limits{
		// One mebibyte is three times the largest body G1 observed in any real
		// corpus and matches the 1 MiB scale probe it ran deliberately. A body
		// above it still synchronizes; it just conflicts instead of merging,
		// which is visible and recoverable rather than slow and unbounded.
		MaxBodyBytes: 1 << 20,
		// A 1 MiB body of single-character lines is the worst realistic token
		// count; this bound sits above it without admitting a generated file
		// with millions of empty lines.
		MaxLineTokens: 262_144,
		// Myers' trace costs (d+1)² integers, so this bound is also a memory
		// bound: about eight megabytes, transient, per merge. G1's worst
		// measured 30-day divergence was 32 edits per side, so a merge that
		// needs more than a thousand is not the case this is optimized for.
		MaxDiffDistance: 1_024,
		// Word refinement is quadratic in a way line merging is not: G1
		// measured a 50 KiB repeated-token line at 3.8 ms per line and 181.9 ms
		// per word. Refinement is therefore regional and small.
		MaxRegionBytes: 64 << 10,
		MaxWordTokens:  32_768,
		// A note with hundreds of separate conflicts is not something a person
		// resolves region by region; it becomes one whole-body conflict.
		MaxConflictRegions:   256,
		MaxConflictTextBytes: 64 << 10,
	}
}

func normalizeLimits(requested Limits) Limits {
	defaults := DefaultLimits()
	if requested.MaxBodyBytes <= 0 {
		requested.MaxBodyBytes = defaults.MaxBodyBytes
	}
	if requested.MaxLineTokens <= 0 {
		requested.MaxLineTokens = defaults.MaxLineTokens
	}
	if requested.MaxDiffDistance <= 0 {
		requested.MaxDiffDistance = defaults.MaxDiffDistance
	}
	if requested.MaxRegionBytes <= 0 {
		requested.MaxRegionBytes = defaults.MaxRegionBytes
	}
	if requested.MaxWordTokens <= 0 {
		requested.MaxWordTokens = defaults.MaxWordTokens
	}
	if requested.MaxConflictRegions <= 0 {
		requested.MaxConflictRegions = defaults.MaxConflictRegions
	}
	if requested.MaxConflictTextBytes <= 0 {
		requested.MaxConflictTextBytes = defaults.MaxConflictTextBytes
	}
	return requested
}

// Region is one unresolved area, reported with the text all three sides hold
// so a person can see the disagreement without reconstructing it.
type Region struct {
	Kind       string `json:"kind"`
	BaseLine   int    `json:"base_line"`
	LocalLine  int    `json:"local_line"`
	RemoteLine int    `json:"remote_line"`
	Base       string `json:"base"`
	Local      string `json:"local"`
	Remote     string `json:"remote"`
	Truncated  bool   `json:"truncated,omitempty"`
	// Refined records that word-level refinement ran on this region and still
	// could not separate the edits, which is the difference between "these
	// edits touch the same line" and "these edits touch the same words".
	Refined bool `json:"refined,omitempty"`
}

// Result is either a merged body or a typed conflict. It is never both, and
// Merged is empty whenever Clean is false — a partially merged body would be
// content nobody wrote.
type Result struct {
	Merged  string
	Clean   bool
	Kind    string
	Regions []Region
}

// Merge performs the bounded three-way merge of one note body. It never
// returns an error: every refusal is a typed conflict, because the caller's
// job is the same either way — record something durable and visible that holds
// all three inputs.
func Merge(base, local, remote string, requested Limits) Result {
	limits := normalizeLimits(requested)
	if !utf8.ValidString(base) || !utf8.ValidString(local) || !utf8.ValidString(remote) {
		return wholeBodyConflict(ConflictInvalidUTF8, base, local, remote, limits)
	}
	if len(base) > limits.MaxBodyBytes || len(local) > limits.MaxBodyBytes || len(remote) > limits.MaxBodyBytes {
		return wholeBodyConflict(ConflictBoundsExceeded, base, local, remote, limits)
	}
	// The three trivial cases are settled without a diff. They are not an
	// optimization: they are what makes an ordinary one-sided edit cost nothing
	// and stay exact regardless of how far it moved the body.
	switch {
	case local == remote:
		return Result{Merged: local, Clean: true}
	case base == local:
		return Result{Merged: remote, Clean: true}
	case base == remote:
		return Result{Merged: local, Clean: true}
	}

	baseLines, localLines, remoteLines := SplitLines(base), SplitLines(local), SplitLines(remote)
	if len(baseLines) > limits.MaxLineTokens || len(localLines) > limits.MaxLineTokens || len(remoteLines) > limits.MaxLineTokens {
		return wholeBodyConflict(ConflictBoundsExceeded, base, local, remote, limits)
	}
	segments, ok := merge3(baseLines, localLines, remoteLines, limits.MaxDiffDistance)
	if !ok {
		return wholeBodyConflict(ConflictBoundsExceeded, base, local, remote, limits)
	}

	merged := make([]string, 0, len(baseLines))
	regions := make([]Region, 0, 4)
	for _, segment := range segments {
		if segment.conflict == nil {
			merged = append(merged, segment.resolved...)
			continue
		}
		span := *segment.conflict
		refinedTokens, refined, resolved := refineRegion(span, limits)
		if resolved {
			merged = append(merged, refinedTokens...)
			continue
		}
		if len(regions) >= limits.MaxConflictRegions {
			return wholeBodyConflict(ConflictBoundsExceeded, base, local, remote, limits)
		}
		regions = append(regions, region(span, refined, limits))
	}
	if len(regions) > 0 {
		return Result{Clean: false, Kind: regions[0].Kind, Regions: regions}
	}
	return Result{Merged: joinTokens(merged), Clean: true}
}

func region(span conflictSpan, refined bool, limits Limits) Region {
	baseText, baseTruncated := truncateRunes(joinTokens(span.base), limits.MaxConflictTextBytes)
	localText, localTruncated := truncateRunes(joinTokens(span.local), limits.MaxConflictTextBytes)
	remoteText, remoteTruncated := truncateRunes(joinTokens(span.remote), limits.MaxConflictTextBytes)
	return Region{
		Kind:       classify(span),
		BaseLine:   span.baseStart + 1,
		LocalLine:  span.localStart + 1,
		RemoteLine: span.remoteStart + 1,
		Base:       baseText,
		Local:      localText,
		Remote:     remoteText,
		Truncated:  baseTruncated || localTruncated || remoteTruncated,
		Refined:    refined,
	}
}

// classify separates a deletion that met an edit from two edits that met each
// other. A side that removed everything the other side rewrote cannot be
// honoured by any merged text, so it is worth naming on its own.
func classify(span conflictSpan) string {
	if len(span.base) > 0 && (len(span.local) == 0 || len(span.remote) == 0) {
		return ConflictDeleteEdit
	}
	return ConflictSameToken
}

func wholeBodyConflict(kind, base, local, remote string, limits Limits) Result {
	baseText, baseTruncated := truncateRunes(base, limits.MaxConflictTextBytes)
	localText, localTruncated := truncateRunes(local, limits.MaxConflictTextBytes)
	remoteText, remoteTruncated := truncateRunes(remote, limits.MaxConflictTextBytes)
	return Result{
		Clean: false,
		Kind:  kind,
		Regions: []Region{{
			Kind: kind, BaseLine: 1, LocalLine: 1, RemoteLine: 1,
			Base: baseText, Local: localText, Remote: remoteText,
			Truncated: baseTruncated || localTruncated || remoteTruncated,
		}},
	}
}

// truncateRunes cuts on a rune boundary so a truncated conflict never hands a
// reader a replacement character that looks like content.
func truncateRunes(value string, maximum int) (string, bool) {
	if len(value) <= maximum {
		return value, false
	}
	cut := maximum
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut], true
}

// refineRegion retries one conflicting region at word granularity. It runs
// only inside explicit byte and token bounds, and a refinement that itself
// conflicts leaves the original line-level region in place rather than
// replacing it with a narrower one that hides how much text is involved.
//
// Refinement is confined to a region that is exactly one line on all three
// sides, which is the case G1 selected it for: "a line conflict may be retried
// as Unicode-aware word tokens". Two randomized cases showed why the wider
// reading is wrong. Allowing a multi-line region let each side delete a
// different line and let word merging "resolve" that by deleting both and
// keeping a stray prefix; allowing equal-but-multiple lines let refinement
// rejoin words across a line boundary into "L R line 5", a line neither
// replica ever held. Words can separate two edits inside one line. They cannot
// decide which lines exist, and a merge that invents a line is worse than a
// conflict that asks.
func refineRegion(span conflictSpan, limits Limits) ([]string, bool, bool) {
	if len(span.base) != 1 || len(span.local) != 1 || len(span.remote) != 1 {
		return nil, false, false
	}
	baseText, localText, remoteText := joinTokens(span.base), joinTokens(span.local), joinTokens(span.remote)
	if len(baseText) > limits.MaxRegionBytes || len(localText) > limits.MaxRegionBytes || len(remoteText) > limits.MaxRegionBytes {
		return nil, false, false
	}
	baseWords, localWords, remoteWords := SplitWords(baseText), SplitWords(localText), SplitWords(remoteText)
	if len(baseWords) > limits.MaxWordTokens || len(localWords) > limits.MaxWordTokens || len(remoteWords) > limits.MaxWordTokens {
		return nil, false, false
	}
	segments, ok := merge3(baseWords, localWords, remoteWords, limits.MaxDiffDistance)
	if !ok {
		return nil, true, false
	}
	merged := make([]string, 0, len(baseWords))
	for _, segment := range segments {
		if segment.conflict != nil {
			return nil, true, false
		}
		merged = append(merged, segment.resolved...)
	}
	return []string{joinTokens(merged)}, true, true
}
