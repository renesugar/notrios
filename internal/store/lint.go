package store

// Workspace lint: a read-only report of what has rotted in a library.
//
// Three properties shape the whole design. It never writes — a report that
// cannot mutate is safe to run on a library nobody has backed up yet, which is
// exactly when someone wants to know what is broken. It is bounded: every check
// streams its rows, hashing and counting each one before discarding it, keeping
// only the first `DetailLimit` examples, so counts and the digest cover the
// whole library while memory stays flat. And it reads the library once: the six
// checks whose findings come from `document_links` share a single ordered scan
// (`sqlite_lint_links.go`) rather than scanning that table six times, which is
// where a first implementation spent most of its time.
//
// Findings are content-free at the API boundary, following the P1 planner: a
// finding carries stable IDs, a location, a reason code, and a SHA-256
// fingerprint of the offending target — never a note title, a body excerpt, a
// raw link target, or a local path. A document ID with a line and column is
// enough to find the problem; the raw text of a broken wikilink is often a
// private note's title.
//
// Heading anchors were unverifiable until v0.5 E1a: a heading anchor is a slug
// and block rows store a content hash rather than heading text, so there was
// nothing to compare against. Schema v15 stores the slug, and
// `unresolved_heading_anchor` now checks them the same way block anchors are
// checked.

// Lint check names. They are stable reason codes: a caller may key behaviour off
// them, so they are part of the contract rather than display strings.
const (
	LintBrokenDocumentLink      = "broken_document_link"
	LintBrokenResourceLink      = "broken_resource_link"
	LintAmbiguousLink           = "ambiguous_link"
	LintUnresolvedBlockAnchor   = "unresolved_block_anchor"
	LintUnresolvedHeadingAnchor = "unresolved_heading_anchor"
	LintDuplicateSourceID       = "duplicate_source_id"
	LintMissingTitle            = "missing_title"
	LintUnlocalizedRemoteMedia  = "unlocalized_remote_media"
	LintMissingAltText          = "missing_alt_text"
	LintUnreferencedResource    = "unreferenced_resource"
	LintProjectionBacklog       = "projection_backlog"
	// LintDanglingCollection finds a note whose collection identifier names no
	// collection row.
	//
	// `documents.collection_id` is a foreign key and `PRAGMA foreign_keys` is
	// ON, so no supported write can produce this. A physical snapshot restore
	// suspends the constraint while it installs an image, and sync applies
	// canonical metadata, so a library can still *arrive* in this state -- and
	// nothing said so. That is what a lint check is for: a library that came in
	// broken rather than one that was broken here.
	LintDanglingCollection = "dangling_collection"
)

// LintChecks lists every check in report order.
func LintChecks() []string {
	return []string{
		LintBrokenDocumentLink,
		LintBrokenResourceLink,
		LintAmbiguousLink,
		LintUnresolvedBlockAnchor,
		LintUnresolvedHeadingAnchor,
		LintDuplicateSourceID,
		LintMissingTitle,
		LintUnlocalizedRemoteMedia,
		LintMissingAltText,
		LintUnreferencedResource,
		LintProjectionBacklog,
		LintDanglingCollection,
	}
}

// Lint limits keep one report bounded regardless of library size.
const (
	MaxLintDetailItems     = 1000
	DefaultLintDetailItems = 100
)

// LintRequest selects what to check. The zero value checks everything in the
// default collection with the default detail cap.
type LintRequest struct {
	CollectionID string   `json:"collection_id,omitempty"`
	Checks       []string `json:"checks,omitempty"`
	DetailLimit  int      `json:"detail_limit,omitempty"`
}

// LintFinding is one problem, located but not quoted.
type LintFinding struct {
	Check string `json:"check"`
	// DocumentID and ResourceID identify the object; exactly one is set for
	// most checks, and neither for library-wide findings like a projection
	// backlog.
	DocumentID string `json:"document_id,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	// Line and Column locate the problem inside the note, which is what a user
	// needs to fix it. They are 1-indexed and zero when not applicable.
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
	// TargetSHA256 fingerprints the offending link target or anchor so two
	// findings can be compared without the raw value crossing the API. A raw
	// wikilink target is frequently a private note's title.
	TargetSHA256 string `json:"target_sha256,omitempty"`
	// Detail is a bounded, content-free qualifier: a resolution status, an
	// anchor kind, a count. Never note text.
	Detail string `json:"detail,omitempty"`
}

// LintCheckResult is one check's complete count plus its capped examples.
type LintCheckResult struct {
	Check    string        `json:"check"`
	Count    int           `json:"count"`
	Findings []LintFinding `json:"findings"`
	// ElapsedMS is how long this check took. A lint pass reads the whole
	// library, so which check is expensive is information the operator needs
	// — and the number that tells a future maintainer where to look.
	ElapsedMS float64 `json:"elapsed_ms"`
	Truncated bool    `json:"truncated"`
}

// LintReport is the whole read-only result.
type LintReport struct {
	Version      int               `json:"version"`
	CollectionID string            `json:"collection_id"`
	DetailLimit  int               `json:"detail_limit"`
	Checks       []LintCheckResult `json:"checks"`
	// TotalFindings is the sum of every check's complete count, not of the
	// capped examples.
	TotalFindings int `json:"total_findings"`
	// ReportSHA256 covers every finding the library contains, including those
	// the detail cap hid, so two runs over unchanged canonical state produce the
	// same digest.
	ReportSHA256 string   `json:"report_sha256"`
	Warnings     []string `json:"warnings"`
}
