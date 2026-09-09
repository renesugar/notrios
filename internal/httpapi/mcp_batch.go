package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/renesugar/notrios/internal/store"
)

// mcpRunBatch exposes bounded batch organizer transactions under the
// `organizer` scope.
//
// v0.6 F1 deliberately shipped the batch surface without an MCP tool, so bulk
// mutation would not land on the default read-only surface before scopes
// existed to gate it. This is where it lands.
//
// The tool is a thin pass-through: the store validates, bounds, and reports.
// Per-item failure is a value in the result rather than an error, for the same
// reason it is a 200 over REST — a caller needs to know which half happened.
func (s *Server) mcpRunBatch(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		RequestKey string   `json:"request_key,omitempty"`
		Operation  string   `json:"operation,omitempty"`
		Mode       string   `json:"mode,omitempty"`
		NotebookID string   `json:"notebook_id,omitempty"`
		Tags       []string `json:"tags,omitempty"`
		Items      []struct {
			DocumentID     string `json:"document_id,omitempty"`
			BaseRevisionID string `json:"base_revision_id,omitempty"`
		} `json:"items,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	items := make([]store.BatchItem, 0, len(args.Items))
	for _, item := range args.Items {
		items = append(items, store.BatchItem{DocumentID: item.DocumentID, BaseRevisionID: item.BaseRevisionID})
	}
	result, err := s.store.RunBatch(r.Context(), store.BatchRequest{
		RequestKey: args.RequestKey,
		Operation:  args.Operation,
		Mode:       args.Mode,
		Items:      items,
		NotebookID: args.NotebookID,
		Tags:       args.Tags,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(result)
}
