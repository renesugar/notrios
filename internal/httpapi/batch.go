package httpapi

import (
	"net/http"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// handleBatch runs one bounded organizer transaction.
//
// A request that ran is a `200` even when every item failed: per-item failure
// is the report's content, not the request's fate. Only a malformed request — a
// bad operation, a missing argument, too many items, a reused key — is a `4xx`,
// because that is the caller getting the call itself wrong.
func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req api.BatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	items := make([]store.BatchItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, store.BatchItem{DocumentID: item.DocumentID, BaseRevisionID: item.BaseRevisionID})
	}
	result, err := s.store.RunBatch(r.Context(), store.BatchRequest{
		RequestKey: req.RequestKey,
		Operation:  req.Operation,
		Mode:       req.Mode,
		Items:      items,
		NotebookID: req.NotebookID,
		Tags:       req.Tags,
	})
	if writeStoreError(w, err, "batch_failed") {
		return
	}
	out := make([]api.BatchItemResult, 0, len(result.Items))
	for _, item := range result.Items {
		out = append(out, api.BatchItemResult{
			DocumentID:    item.DocumentID,
			Status:        item.Status,
			Reason:        item.Reason,
			Error:         item.Error,
			NewDocumentID: item.NewDocumentID,
		})
	}
	writeJSON(w, http.StatusOK, api.BatchResult{
		RequestKey: result.RequestKey,
		Operation:  result.Operation,
		Mode:       result.Mode,
		Items:      out,
		Applied:    result.Applied,
		Skipped:    result.Skipped,
		Failed:     result.Failed,
		RolledBack: result.RolledBack,
		Replayed:   result.Replayed,
	})
}
