package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Embedded query blocks: a fenced ```note-query in a note, evaluated at render
// time into a bounded list of notes.
//
// Three constraints from `PLAN.md` E7 shape every line of this file.
//
// **No SQL, no JavaScript, no filesystem reach.** A block declares a Q1 query
// and a typed selection of fields, a sort, and a limit. Nothing else is
// accepted, and an unknown key is an error rather than something ignored — a
// block that silently does less than it says is worse than one that refuses.
//
// **Note content is untrusted.** The block text comes from a note, which may
// have been imported from anywhere. It is parsed by the same Q1 parser every
// other search surface uses, so a block cannot express anything the user's own
// search box cannot, and it reaches exactly the same bounded Store operation.
//
// **A block never blocks the note.** Every failure here is a value, not an
// error: a malformed block returns a message to render inside the block, and
// the note around it is unaffected.

// Note-query bounds. A block is note content, so these are hard.
const (
	// MaxNoteQueryBlockBytes bounds one block's source.
	MaxNoteQueryBlockBytes = 4096
	// MaxNoteQueryLines bounds how many directives one block may carry.
	MaxNoteQueryLines = 32
	// MaxNoteQueryRows is the hard result cap. DefaultNoteQueryRows applies
	// when a block names no limit.
	MaxNoteQueryRows     = 100
	DefaultNoteQueryRows = 10
)

// Note-query sort orders. These are the two a keyset can reproduce, which is
// why they are the two on offer: `updated` traverses `(updated_at, id)` and
// `relevance` ranks FTS5 results. There is no ascending variant because there
// is no index that would serve one.
const (
	NoteQuerySortUpdated   = "updated"
	NoteQuerySortRelevance = "relevance"
)

// Note-query field names. `title` is always returned; the rest are opt-in.
const (
	NoteQueryFieldTitle    = "title"
	NoteQueryFieldNotebook = "notebook"
	NoteQueryFieldTags     = "tags"
	NoteQueryFieldUpdated  = "updated"
	NoteQueryFieldSnippet  = "snippet"
)

// NoteQueryFields lists every selectable field, in render order.
func NoteQueryFields() []string {
	return []string{
		NoteQueryFieldTitle,
		NoteQueryFieldNotebook,
		NoteQueryFieldTags,
		NoteQueryFieldUpdated,
		NoteQueryFieldSnippet,
	}
}

// NoteQueryKeys lists every directive a block may use, for error messages.
func NoteQueryKeys() []string {
	return []string{"query", "fields", "sort", "limit"}
}

// NoteQueryRequest evaluates one block.
type NoteQueryRequest struct {
	CollectionID string `json:"collection_id,omitempty"`
	// Block is the text between the fences, exactly as it appears in the note.
	Block string `json:"block"`
}

// NoteQuerySpec is a parsed, validated block. It exists separately from the
// request so a caller can see what a block actually asked for.
type NoteQuerySpec struct {
	Query  string   `json:"query"`
	Fields []string `json:"fields"`
	Sort   string   `json:"sort"`
	Limit  int      `json:"limit"`
}

// NoteQueryRow is one matching note. It carries only what the selected fields
// asked for, so a block that requests a title does not receive a body excerpt.
type NoteQueryRow struct {
	DocumentID string     `json:"document_id"`
	URI        string     `json:"uri"`
	Title      string     `json:"title"`
	Notebook   string     `json:"notebook,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	Snippet    string     `json:"snippet,omitempty"`
}

// NoteQueryResult is what a block renders.
//
// `Error` is a value rather than a returned error on purpose: a note with a
// broken query block still has to render, so the block shows the message and
// the note is untouched. A transport-level failure is still an error.
type NoteQueryResult struct {
	Spec NoteQuerySpec  `json:"spec"`
	Rows []NoteQueryRow `json:"rows"`
	// Truncated says the result hit the block's limit and more notes match.
	Truncated bool `json:"truncated"`
	// Error is a human-readable reason the block could not run. When it is set,
	// Rows is empty.
	Error string `json:"error,omitempty"`
}

// parseNoteQueryBlock reads a block's directives.
//
// The format is deliberately line-oriented `key: value` rather than YAML: the
// service carries no YAML parser, a nested document would need one, and every
// filtering idea a block might express is already expressible in the Q1 query
// language that `query:` accepts.
func parseNoteQueryBlock(block string) (NoteQuerySpec, error) {
	spec := NoteQuerySpec{Limit: DefaultNoteQueryRows, Fields: []string{NoteQueryFieldTitle}}
	if len(block) > MaxNoteQueryBlockBytes {
		return spec, fmt.Errorf("block is longer than %d bytes", MaxNoteQueryBlockBytes)
	}
	lines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	if len(lines) > MaxNoteQueryLines {
		return spec, fmt.Errorf("block has more than %d lines", MaxNoteQueryLines)
	}

	seen := map[string]bool{}
	sawQuery := false
	for number, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			return spec, fmt.Errorf("line %d is not `key: value`", number+1)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if seen[key] {
			return spec, fmt.Errorf("line %d repeats %q", number+1, key)
		}
		seen[key] = true

		switch key {
		case "query":
			spec.Query = value
			sawQuery = true
		case "limit":
			limit, err := strconv.Atoi(value)
			if err != nil || limit <= 0 {
				return spec, fmt.Errorf("line %d: limit must be a positive whole number", number+1)
			}
			if limit > MaxNoteQueryRows {
				return spec, fmt.Errorf("line %d: limit %d exceeds the ceiling of %d", number+1, limit, MaxNoteQueryRows)
			}
			spec.Limit = limit
		case "sort":
			switch strings.ToLower(value) {
			case NoteQuerySortUpdated, NoteQuerySortRelevance:
				spec.Sort = strings.ToLower(value)
			default:
				return spec, fmt.Errorf("line %d: sort must be %s or %s", number+1, NoteQuerySortUpdated, NoteQuerySortRelevance)
			}
		case "fields":
			fields, err := parseNoteQueryFields(value)
			if err != nil {
				return spec, fmt.Errorf("line %d: %v", number+1, err)
			}
			spec.Fields = fields
		default:
			return spec, fmt.Errorf("line %d: unknown key %q; use %s", number+1, key, strings.Join(NoteQueryKeys(), ", "))
		}
	}
	if !sawQuery {
		return spec, fmt.Errorf("a note-query block needs a `query:` line")
	}
	if spec.Sort == "" {
		// Relevance needs something to rank. A query with no text terms has
		// nothing, so the sensible default depends on the query — and saying
		// which one was used is the result's job, not a silent choice.
		spec.Sort = NoteQuerySortUpdated
	}
	return spec, nil
}

// parseNoteQueryFields validates a comma-separated field list, keeping the
// canonical render order rather than the order they were written in.
func parseNoteQueryFields(value string) ([]string, error) {
	requested := map[string]bool{NoteQueryFieldTitle: true}
	for _, part := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		known := false
		for _, field := range NoteQueryFields() {
			if field == name {
				known = true
				break
			}
		}
		if !known {
			return nil, fmt.Errorf("unknown field %q; use %s", name, strings.Join(NoteQueryFields(), ", "))
		}
		requested[name] = true
	}
	ordered := []string{}
	for _, field := range NoteQueryFields() {
		if requested[field] {
			ordered = append(ordered, field)
		}
	}
	return ordered, nil
}

func (s NoteQuerySpec) wants(field string) bool {
	for _, name := range s.Fields {
		if name == field {
			return true
		}
	}
	return false
}
