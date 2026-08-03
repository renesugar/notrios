package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/renesugar/notrios/internal/store"
)

const restSelectionMaxDocuments = 100_000

// handleSelectionPlan is the shared read-only dry-run surface used by future
// archive-v2, subset-transfer, and publication-handoff workflows. It returns
// content-free IDs/hashes/decisions and never accepts SQL or filesystem paths.
func (s *Server) handleSelectionPlan(w http.ResponseWriter, r *http.Request) {
	var req store.SelectionPlanRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "selection planning requires the canonical store")
		return
	}
	if req.DetailLimit > store.MaxSelectionDetailItems {
		writeError(w, http.StatusBadRequest, "limit_too_large", fmt.Sprintf("detail_limit must be %d or less", store.MaxSelectionDetailItems))
		return
	}
	if req.MaxDocuments <= 0 {
		req.MaxDocuments = restSelectionMaxDocuments
	}
	if req.MaxDocuments > restSelectionMaxDocuments {
		writeError(w, http.StatusBadRequest, "limit_too_large", fmt.Sprintf("REST max_documents must be %d or less", restSelectionMaxDocuments))
		return
	}
	plan, err := s.store.PlanSelection(r.Context(), req)
	if writeStoreError(w, err, "selection_plan_failed") {
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) mcpPlanSelection(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var req store.SelectionPlanRequest
	if err := unmarshalMCPArgs(raw, &req); err != nil {
		return mcpToolResult{}, err
	}
	req.DetailLimit = clampMCPLimit(req.DetailLimit, s.config.MCP.MaxResults)
	if req.MaxDocuments <= 0 {
		req.MaxDocuments = restSelectionMaxDocuments
	}
	if req.MaxDocuments > restSelectionMaxDocuments {
		return mcpToolResult{}, fmt.Errorf("max_documents must be %d or less", restSelectionMaxDocuments)
	}
	plan, err := s.store.PlanSelection(r.Context(), req)
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(plan)
}
