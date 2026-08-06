package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// Editor link intelligence over REST.
//
// Both routes are read-only, bounded, and called while someone types, which is
// what makes their bounds load-bearing rather than decorative. Neither writes a
// revision, and neither returns a note body — a suggestion is an ID and a title,
// and a link check is a classification of bytes the caller already had.

func (s *Server) handleDocumentSuggest(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "link suggestions require the canonical store")
		return
	}
	req := store.DocumentSuggestionRequest{
		CollectionID:      r.URL.Query().Get("collection_id"),
		Query:             r.URL.Query().Get("q"),
		ExcludeDocumentID: r.URL.Query().Get("exclude_document_id"),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
			return
		}
		req.Limit = limit
	}
	suggestions, err := s.store.SuggestDocuments(r.Context(), req)
	if writeStoreError(w, err, "suggest_failed") {
		return
	}
	out := make([]api.DocumentSuggestion, 0, len(suggestions.Suggestions))
	for _, suggestion := range suggestions.Suggestions {
		out = append(out, api.DocumentSuggestion{
			DocumentID: suggestion.DocumentID,
			Title:      suggestion.Title,
			URI:        suggestion.URI,
			NotebookID: suggestion.NotebookID,
			Match:      suggestion.Match,
		})
	}
	writeJSON(w, http.StatusOK, api.DocumentSuggestionResponse{
		Suggestions: out,
		Truncated:   suggestions.Truncated,
		Limit:       suggestions.Limit,
	})
}

// handleCheckLinks classifies the links in an unsaved buffer.
//
// The request carries the body rather than a list of targets the client
// extracted. Extracting Markdown links belongs to the canonical parser: a
// client reimplementation would drift, and markers that disagree with what a
// save records are worse than no markers. The body is parsed and discarded —
// this endpoint stores nothing and creates no revision.
func (s *Server) handleCheckLinks(w http.ResponseWriter, r *http.Request) {
	var req api.CheckLinksRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "link checking requires the canonical store")
		return
	}
	checked, err := s.store.CheckLinks(r.Context(), store.CheckLinksRequest{
		CollectionID: req.CollectionID,
		DocumentID:   req.DocumentID,
		Body:         req.Body,
	})
	if writeStoreError(w, err, "check_links_failed") {
		return
	}
	links := make([]api.CheckedLink, 0, len(checked.Links))
	for _, link := range checked.Links {
		links = append(links, api.CheckedLink{
			RawTarget:        link.RawTarget,
			DisplayText:      link.DisplayText,
			RelationType:     link.RelationType,
			SourceFormat:     link.SourceFormat,
			AnchorType:       link.AnchorType,
			AnchorValue:      link.AnchorValue,
			Status:           link.Status,
			TargetDocumentID: link.TargetDocumentID,
			TargetResourceID: link.TargetResourceID,
			TargetURI:        link.TargetURI,
			CanonicalTarget:  link.CanonicalTarget,
			StartByte:        link.StartByte,
			EndByte:          link.EndByte,
			Line:             link.Line,
			Column:           link.Column,
		})
	}
	writeJSON(w, http.StatusOK, api.CheckLinksResponse{
		Links:      links,
		Total:      checked.Total,
		Unresolved: checked.Unresolved,
		Truncated:  checked.Truncated,
	})
}
