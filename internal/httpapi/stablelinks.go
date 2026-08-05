package httpapi

import (
	"net/http"
	"strings"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/stablelink"
)

// maxStableLinkRequestBytes bounds the URI a caller may submit. The parser has
// its own limit; this stops an oversized body before it is decoded.
const maxStableLinkRequestBytes = 4 * stablelink.MaxURIBytes

// handleResolveStableLink answers "which note does this notrios:// link name
// in this database?".
//
// It resolves against the database this service already has open and nothing
// else. Choosing which local database answers a link is a desktop routing
// decision made by the profile registry, so this endpoint deliberately accepts
// no path, profile, or database selector — an HTTP caller cannot make the
// service consult another database.
func (s *Server) handleResolveStableLink(w http.ResponseWriter, r *http.Request) {
	var req api.StableLinkResolveRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_not_wired", "link resolution requires the canonical store")
		return
	}
	uri := strings.TrimSpace(req.URI)
	if uri == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "uri is required")
		return
	}
	if len(uri) > maxStableLinkRequestBytes {
		writeError(w, http.StatusBadRequest, "limit_too_large", "uri is too long")
		return
	}
	resolution, err := s.store.ResolveStableLink(r.Context(), uri)
	if writeStoreError(w, err, "link_resolve_failed") {
		return
	}
	writeJSON(w, http.StatusOK, api.StableLinkResolveResponse{
		URI:             resolution.URI,
		Status:          resolution.Status,
		DatabaseID:      resolution.DatabaseID,
		LocalDatabaseID: resolution.LocalDatabaseID,
		DocumentID:      resolution.DocumentID,
		Anchor:          resolution.Anchor,
		DocumentURI:     resolution.DocumentURI,
		Title:           resolution.Title,
		NotebookID:      resolution.NotebookID,
	})
}
