package httpapi

import (
	"net/http"
	"time"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// handleRunNoteQuery evaluates one embedded ```note-query block.
//
// The request carries the block's text and a collection, and nothing else. It
// accepts no SQL, no filesystem path, and no output target — a block declares a
// Q1 query plus a typed selection of fields, a sort, and a limit, and reaches
// exactly the same bounded Store search the user's own search box does. A block
// therefore cannot read anything its author could not already search for.
//
// A malformed block is a `200` carrying `error`, not a `400`. The note has to
// render; only the block should show a problem.
func (s *Server) handleRunNoteQuery(w http.ResponseWriter, r *http.Request) {
	var req api.NoteQueryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "note queries require the canonical store")
		return
	}
	result, err := s.store.RunNoteQuery(r.Context(), store.NoteQueryRequest{
		CollectionID: req.CollectionID,
		Block:        req.Block,
	})
	if writeStoreError(w, err, "note_query_failed") {
		return
	}
	rows := make([]api.NoteQueryRow, 0, len(result.Rows))
	for _, row := range result.Rows {
		out := api.NoteQueryRow{
			DocumentID: row.DocumentID,
			URI:        row.URI,
			Title:      row.Title,
			Notebook:   row.Notebook,
			Tags:       row.Tags,
			Snippet:    row.Snippet,
		}
		if row.UpdatedAt != nil {
			out.UpdatedAt = row.UpdatedAt.UTC().Format(time.RFC3339)
		}
		rows = append(rows, out)
	}
	writeJSON(w, http.StatusOK, api.NoteQueryResult{
		Spec: api.NoteQuerySpec{
			Query:  result.Spec.Query,
			Fields: result.Spec.Fields,
			Sort:   result.Spec.Sort,
			Limit:  result.Spec.Limit,
		},
		Rows:      rows,
		Truncated: result.Truncated,
		Error:     result.Error,
	})
}
