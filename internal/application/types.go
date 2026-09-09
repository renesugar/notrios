package application

import "time"

// Note is a managed note as the application layer sees it.
//
// It deliberately carries no struct tags. A serialization format is an adapter
// concern, and a tag here would make one adapter's wire shape into everybody's
// contract.
type Note struct {
	ID           string
	URI          string
	CollectionID string
	NotebookID   string
	Title        string
	Body         string
	MIMEType     string
	RevisionID   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// DeletedAt is the zero time for a live note. A trashed note is readable
	// so that it can be recovered; it is the caller's job to refuse edits.
	DeletedAt time.Time
}

// Trashed reports whether the note is in the trash.
func (n Note) Trashed() bool { return !n.DeletedAt.IsZero() }

// Revision is one historical version of a note.
type Revision struct {
	ID        string
	NoteID    string
	Title     string
	Body      string
	MIMEType  string
	Message   string
	CreatedAt time.Time
}

// Resource is an attachment's metadata. The bytes are read through
// [Application.OpenResource]; a resource can be far larger than a result.
type Resource struct {
	ID           string
	URI          string
	CollectionID string
	Filename     string
	MIMEType     string
	SizeBytes    int64
	SHA256       string
	CreatedAt    time.Time
}

// CreateNoteInput creates a note. An empty NotebookID uses the default
// notebook; an empty ID lets the store assign one.
type CreateNoteInput struct {
	ID           string
	CollectionID string
	NotebookID   string
	Title        string
	Body         string
	MIMEType     string
	Message      string
}

// UpdateNoteInput replaces a note's body.
//
// RevisionID is the revision the edit was based on. The store decides whether
// an empty value is permitted; when it is required and absent the facade
// reports KindPreconditionRequired rather than silently overwriting.
type UpdateNoteInput struct {
	ID         string
	RevisionID string
	Title      string
	Body       string
	MIMEType   string
	Message    string
}

// DeleteNoteInput moves a note to the trash, preserving its history.
type DeleteNoteInput struct {
	ID         string
	RevisionID string
	Message    string
}

// RestoreRevisionInput makes an older revision current by writing a new one.
// History is appended to, never rewritten.
type RestoreRevisionInput struct {
	NoteID string
	// RevisionID is the historical revision to restore.
	RevisionID string
	// BaseRevisionID is the revision the caller believed was current.
	BaseRevisionID string
	Message        string
}

// Sort orders for [SearchInput].
const (
	// SortDefault follows the query shape: a positive text-only query ranks by
	// relevance, and anything mixing fields or negation traverses newest-first.
	SortDefault   = ""
	SortRelevance = "relevance"
	SortUpdated   = "updated"
)

// SearchInput is a full-text search request. Cursor continues a previous page
// and must come from [SearchResult.NextCursor].
type SearchInput struct {
	CollectionID string
	Query        string
	Limit        int
	Cursor       string
	Sort         string
}

// SearchResult is one page of hits.
type SearchResult struct {
	Hits []SearchHit
	// NextCursor is empty on the last page.
	NextCursor string
	// Truncated reports that an optional derived index reached its own cap,
	// so the page is short for a reason the caller should surface.
	Truncated bool
}

// SearchHit is one match. Snippet is already excerpted by the index.
type SearchHit struct {
	ID           string
	URI          string
	CollectionID string
	NotebookID   string
	Title        string
	Snippet      string
	Score        float64
	UpdatedAt    time.Time
	// Sources names the indexes that produced the hit, so a caller can tell a
	// native match from one contributed by an optional sidecar.
	Sources []string
}
