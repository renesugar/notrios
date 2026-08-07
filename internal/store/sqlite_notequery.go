package store

import (
	"context"
	"errors"
	"strings"
)

// RunNoteQuery evaluates one embedded query block.
//
// It reaches the ordinary bounded search — the same Store operation the search
// box, REST, and MCP use — so a block cannot see anything the user's own search
// cannot, and it inherits the Q1 parser's limits without restating them.
//
// A block that cannot run returns a result carrying `Error`, not a Go error. A
// note with one broken block must still render; only a transport or storage
// failure is an error.
func (s *SQLiteStore) RunNoteQuery(ctx context.Context, req NoteQueryRequest) (NoteQueryResult, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return NoteQueryResult{}, err
	}
	collectionID := strings.TrimSpace(req.CollectionID)
	if collectionID == "" {
		collectionID = "default"
	}

	spec, err := parseNoteQueryBlock(req.Block)
	if err != nil {
		return NoteQueryResult{Spec: spec, Rows: []NoteQueryRow{}, Error: err.Error()}, nil
	}

	search := SearchRequest{
		CollectionID: collectionID,
		Query:        spec.Query,
		Limit:        spec.Limit,
	}
	if spec.Sort == NoteQuerySortUpdated {
		search.Sort = SortUpdated
	}

	response, err := s.Search(ctx, search)
	if err != nil {
		// A rejected query is the block author's mistake, not a service fault,
		// so it is reported inside the block like any other malformed input.
		if errors.Is(err, ErrInvalidInput) {
			return NoteQueryResult{Spec: spec, Rows: []NoteQueryRow{}, Error: err.Error()}, nil
		}
		return NoteQueryResult{}, err
	}

	// Truncation comes from the search's own cursor rather than from
	// over-fetching by one: NormalizeSearchRequest clamps a limit at 100, so
	// asking for one extra row would be silently ignored at exactly the block
	// ceiling — the case where knowing there is more matters most.
	result := NoteQueryResult{Spec: spec, Rows: []NoteQueryRow{}, Truncated: response.NextCursor != ""}
	hits := response.Hits

	notebooks := map[string]string{}
	for _, hit := range hits {
		if err := ctx.Err(); err != nil {
			return NoteQueryResult{}, err
		}
		row := NoteQueryRow{DocumentID: hit.ID, URI: hit.URI, Title: hit.Title}
		if spec.wants(NoteQueryFieldUpdated) && !hit.UpdatedAt.IsZero() {
			updated := hit.UpdatedAt
			row.UpdatedAt = &updated
		}
		if spec.wants(NoteQueryFieldSnippet) {
			row.Snippet = hit.Snippet
		}
		if spec.wants(NoteQueryFieldNotebook) && hit.NotebookID != "" {
			name, cached := notebooks[hit.NotebookID]
			if !cached {
				// Bounded by the row cap, and each is a primary-key probe.
				if notebook, err := s.GetNotebook(ctx, hit.NotebookID); err == nil {
					name = notebook.Name
				}
				notebooks[hit.NotebookID] = name
			}
			row.Notebook = name
		}
		if spec.wants(NoteQueryFieldTags) {
			tags, err := s.ListDocumentTags(ctx, hit.ID)
			if err != nil {
				return NoteQueryResult{}, err
			}
			names := make([]string, 0, len(tags))
			for _, tag := range tags {
				names = append(names, tag.Name)
			}
			row.Tags = names
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}
