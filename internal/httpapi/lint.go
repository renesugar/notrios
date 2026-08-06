package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// handleLintReport is the read-only workspace lint.
//
// It is a GET with no apply endpoint, exactly like the garbage-collection
// report: a maintenance surface that cannot mutate is safe to run on a library
// nobody has backed up, which is when people most want to know what is broken.
// Fixing is E3 and is deliberately CLI-only with a per-note revision
// precondition.
func (s *Server) handleLintReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	request := store.LintRequest{
		CollectionID: strings.TrimSpace(r.URL.Query().Get("collection")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("detail_limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "validation_failed", "detail_limit must be a positive integer")
			return
		}
		if parsed > store.MaxLintDetailItems {
			writeError(w, http.StatusBadRequest, "limit_too_large", "detail_limit is too large")
			return
		}
		request.DetailLimit = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("checks")); raw != "" {
		for _, check := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(check); trimmed != "" {
				request.Checks = append(request.Checks, trimmed)
			}
		}
	}
	report, err := s.store.LintWorkspace(r.Context(), request)
	if writeStoreError(w, err, "lint_failed") {
		return
	}
	writeJSON(w, http.StatusOK, report)
}
