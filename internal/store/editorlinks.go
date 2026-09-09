package store

import (
	"fmt"
	"strings"
)

// Editor link intelligence: the two read-only questions an editor asks while
// someone is typing.
//
// "Which notes could this link name?" — SuggestDocuments.
// "Do the links in what I have typed so far resolve?" — CheckLinks.
//
// Both are bounded and neither writes anything. A keystroke must not be able to
// start a whole-library query, and nothing an editor does before saving may
// create a revision.
//
// The second one takes the unsaved body rather than a list of targets the
// client extracted. Extracting Markdown links is the canonical parser's job:
// asking a client to reimplement it in another language would give an editor
// markers that disagree with what a save actually records, which is worse than
// no markers at all. The body is parsed and thrown away — nothing is stored.

// Suggestion limits. They bound one keystroke's work.
const (
	MaxDocumentSuggestions     = 50
	DefaultDocumentSuggestions = 10
	// MinSuggestionQueryBytes is the shortest prefix that will be answered. A
	// single letter matches so much of a large library that ranking it means
	// reading the library, and no useful suggestion comes out of it anyway.
	MinSuggestionQueryBytes = 2
	MaxSuggestionQueryBytes = 256
	// maxSuggestionWordCandidates bounds the interior-word pass. See
	// SuggestDocuments for what the bound costs.
	maxSuggestionWordCandidates = 200
)

// Buffer-check limits.
const (
	// MaxCheckBodyBytes bounds one unsaved buffer. It is far above any note a
	// person types and far below anything that would make a keystroke expensive.
	MaxCheckBodyBytes = 1 << 20
	// MaxCheckedLinks bounds the reply. A note with more links than this is
	// checked up to the bound and says so.
	MaxCheckedLinks = 2000
)

// How a suggestion matched, which is also its rank order. A client shows them
// in the order returned; it does not re-sort.
const (
	// SuggestionMatchTitlePrefix means the note's title starts with the query.
	// These come first and arrive in title order from an index range scan.
	SuggestionMatchTitlePrefix = "title_prefix"
	// SuggestionMatchWordPrefix means a later word in the title starts with the
	// query — "plan" finding "Kitchen Plan".
	SuggestionMatchWordPrefix = "word_prefix"
)

// DocumentSuggestion is one candidate link target. It carries what an editor
// needs to insert a canonical link and nothing else: never a body, never a
// snippet.
type DocumentSuggestion struct {
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	URI        string `json:"uri"`
	NotebookID string `json:"notebook_id,omitempty"`
	Match      string `json:"match"`
}

// DocumentSuggestionRequest asks what a partial link target could name.
type DocumentSuggestionRequest struct {
	CollectionID string `json:"collection_id,omitempty"`
	Query        string `json:"query"`
	Limit        int    `json:"limit,omitempty"`
	// ExcludeDocumentID drops the note being edited: a note rarely wants to
	// link to itself, and offering it pushes a real candidate off a short list.
	ExcludeDocumentID string `json:"exclude_document_id,omitempty"`
}

// DocumentSuggestionResponse is one bounded page. There is no cursor: an
// autocomplete that pages is an autocomplete nobody uses, so a caller that ran
// out of room types more instead.
type DocumentSuggestionResponse struct {
	Suggestions []DocumentSuggestion `json:"suggestions"`
	// Truncated says more notes matched than the limit allowed.
	Truncated bool `json:"truncated,omitempty"`
	Limit     int  `json:"limit"`
}

// CheckLinksRequest carries an unsaved buffer.
type CheckLinksRequest struct {
	CollectionID string `json:"collection_id,omitempty"`
	// DocumentID is the note being edited, when there is one. It is what makes
	// an anchor-only link (`[x](#heading)`) resolve to this note, and it is
	// what tells the checker to read anchors out of the submitted body rather
	// than out of the saved blocks.
	DocumentID string `json:"document_id,omitempty"`
	Body       string `json:"body"`
}

// CheckedLink is one link found in the buffer, located and classified.
type CheckedLink struct {
	RawTarget    string `json:"raw_target"`
	DisplayText  string `json:"display_text,omitempty"`
	RelationType string `json:"relation_type,omitempty"`
	SourceFormat string `json:"source_format,omitempty"`
	AnchorType   string `json:"anchor_type,omitempty"`
	AnchorValue  string `json:"anchor_value,omitempty"`
	// Status is the ordinary link resolution status — resolved, unresolved,
	// ambiguous, external, invalid — plus stale_anchor when the note resolves
	// but the section or block inside it does not.
	Status           string `json:"status"`
	TargetDocumentID string `json:"target_document_id,omitempty"`
	TargetResourceID string `json:"target_resource_id,omitempty"`
	TargetURI        string `json:"target_uri,omitempty"`
	// CanonicalTarget is the URI this link already points at, filled in when the
	// link resolved by title or filename. It is what an editor offers to
	// substitute, and it is the same repair `notriosctl fix` performs.
	CanonicalTarget string `json:"canonical_target,omitempty"`
	StartByte       int    `json:"start_byte"`
	EndByte         int    `json:"end_byte"`
	Line            int    `json:"line"`
	Column          int    `json:"column"`
}

// Resolved reports whether this link would open something.
func (l CheckedLink) Resolved() bool {
	return l.Status == "resolved" || l.Status == "external"
}

// CheckLinksResponse classifies every link in the buffer.
type CheckLinksResponse struct {
	Links []CheckedLink `json:"links"`
	// Counts are over the whole buffer even when Links was capped.
	Total      int  `json:"total"`
	Unresolved int  `json:"unresolved"`
	Truncated  bool `json:"truncated,omitempty"`
}

func (r *DocumentSuggestionRequest) validate() error {
	// Empty means every collection; see CollectionScopeSQL. Somebody linking to
	// a note they migrated should be offered it.
	r.CollectionID = strings.TrimSpace(r.CollectionID)
	r.Query = strings.TrimSpace(r.Query)
	if len(r.Query) < MinSuggestionQueryBytes {
		return fmt.Errorf("%w: query must be at least %d characters", ErrInvalidInput, MinSuggestionQueryBytes)
	}
	if len(r.Query) > MaxSuggestionQueryBytes {
		return fmt.Errorf("%w: query exceeds %d bytes", ErrInvalidInput, MaxSuggestionQueryBytes)
	}
	if r.Limit < 0 {
		return fmt.Errorf("%w: limit must not be negative", ErrInvalidInput)
	}
	if r.Limit == 0 {
		r.Limit = DefaultDocumentSuggestions
	}
	if r.Limit > MaxDocumentSuggestions {
		return fmt.Errorf("%w: limit %d exceeds the ceiling of %d", ErrInvalidInput, r.Limit, MaxDocumentSuggestions)
	}
	return nil
}

func (r *CheckLinksRequest) validate() error {
	// Empty means every collection; see CollectionScopeSQL. Somebody linking to
	// a note they migrated should be offered it.
	r.CollectionID = strings.TrimSpace(r.CollectionID)
	r.DocumentID = strings.TrimSpace(r.DocumentID)
	if len(r.Body) > MaxCheckBodyBytes {
		return fmt.Errorf("%w: body exceeds %d bytes", ErrInvalidInput, MaxCheckBodyBytes)
	}
	return nil
}
