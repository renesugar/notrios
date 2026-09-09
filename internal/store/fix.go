package store

// Workspace fix: the apply half of lint, for the findings that can be repaired
// mechanically.
//
// Three rules shape it, and they come from `PROJECT_DECISIONS.md` 18 plus what
// E2 established.
//
// It is single-note and revision-preconditioned. Each note is repaired on its
// own, against the revision the plan was computed from, and a note edited in the
// meantime fails rather than being overwritten. Bulk, all-or-nothing transactions
// belong to the v0.6 organizer.
//
// It writes ordinary revisions. There is no silent rewrite path: every fix is
// visible in a note's history and revertible by restoring the previous revision.
//
// It refuses rather than guesses. A finding whose repair would require deciding
// what the author meant stays reported and unfixed — which is most of what lint
// finds. Fixing is the small, boring subset.

// Fix kinds. They are stable identifiers used by the CLI and reported back, and
// each maps to a lint check of the same name.
const (
	// FixMissingAltText fills an image's empty alt text from the resource
	// filename. It is off by default: a filename is a starting point for a
	// description, not a description, so asking for it is an explicit choice.
	FixMissingAltText = "missing_alt_text"
	// FixNonCanonicalLinkTarget rewrites a link that resolved by title or
	// filename into the canonical `document://`/`resource://` URI it already
	// resolves to, so a later rename cannot break it.
	FixNonCanonicalLinkTarget = "non_canonical_link_target"
)

// FixKinds lists the kinds this store applies directly, in report order.
// Remote-media localization is a fix too, but it runs through the media policy
// engine rather than through a body rewrite, so it is driven above the store.
func FixKinds() []string {
	return []string{FixNonCanonicalLinkTarget, FixMissingAltText}
}

// DefaultFixKinds are applied when the caller names none. Alt text is excluded:
// see FixMissingAltText.
func DefaultFixKinds() []string {
	return []string{FixNonCanonicalLinkTarget}
}

// MaxFixDocuments bounds one planning pass. Fix is deliberately not a bulk
// operation; the bound keeps a run reviewable and its report readable.
const MaxFixDocuments = 1000

// FixRequest selects what to plan.
type FixRequest struct {
	CollectionID string   `json:"collection_id,omitempty"`
	Kinds        []string `json:"kinds,omitempty"`
	// DocumentID restricts the plan to one note.
	DocumentID string `json:"document_id,omitempty"`
	// MaxDocuments bounds the plan; zero uses MaxFixDocuments.
	MaxDocuments int `json:"max_documents,omitempty"`
}

// FixEdit is one replacement inside one note, reported exactly as it would be
// applied. Both sides are shown because a fix is meant to be read before it is
// run — this is the whole point of the dry run being the default.
type FixEdit struct {
	Kind      string `json:"kind"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Before    string `json:"before"`
	After     string `json:"after"`
}

// FixDocumentPlan is every edit for one note, bound to the revision they were
// computed against.
type FixDocumentPlan struct {
	DocumentID string `json:"document_id"`
	// BaseRevisionID is the precondition: applying against anything else fails.
	BaseRevisionID string    `json:"base_revision_id"`
	Edits          []FixEdit `json:"edits"`
}

// FixPlan is the whole dry run.
type FixPlan struct {
	Version      int               `json:"version"`
	CollectionID string            `json:"collection_id"`
	Kinds        []string          `json:"kinds"`
	Documents    []FixDocumentPlan `json:"documents"`
	// TotalEdits and TotalDocuments count what the plan covers, which the
	// document bound may have limited; Truncated says when it did.
	TotalEdits     int      `json:"total_edits"`
	TotalDocuments int      `json:"total_documents"`
	Truncated      bool     `json:"truncated"`
	Warnings       []string `json:"warnings"`
}

// FixApplyResult reports one note's outcome. A run reports per-note outcomes
// rather than failing as a whole, because each note is its own transaction.
type FixApplyResult struct {
	DocumentID string `json:"document_id"`
	Applied    int    `json:"applied"`
	Skipped    int    `json:"skipped"`
	RevisionID string `json:"revision_id,omitempty"`
	Error      string `json:"error,omitempty"`
}
