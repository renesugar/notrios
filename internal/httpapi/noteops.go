package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// Note-content operations for full-featured clients (Notrios redesign task
// R8): append/prepend, line-range reads, and in-note search — parity with the
// joplin-mcp tool surface.

// guardHelpNote refuses API mutations of the read-only Help notebook's notes
// (seeded from docs/ by `notriosctl seed-help`; task R15).
func (s *Server) guardHelpNote(w http.ResponseWriter, r *http.Request, docID string) bool {
	doc, err := s.store.GetDocument(r.Context(), docID)
	if err != nil {
		return false // let the handler produce its own not-found/error
	}
	if doc.NotebookID == store.HelpNotebookID {
		writeError(w, http.StatusForbidden, "forbidden", "Help notebook notes are read-only")
		return true
	}
	return false
}

func (s *Server) handleAppendDocument(w http.ResponseWriter, r *http.Request) {
	s.handleAppendOrPrepend(w, r, false)
}

func (s *Server) handlePrependDocument(w http.ResponseWriter, r *http.Request) {
	s.handleAppendOrPrepend(w, r, true)
}

func (s *Server) handleAppendOrPrepend(w http.ResponseWriter, r *http.Request, prepend bool) {
	if !s.requireStore(w) {
		return
	}
	var req api.AppendTextRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "text is required")
		return
	}
	baseRevisionID := firstNonEmpty(req.BaseRevisionID, revisionFromIfMatch(r.Header.Get("If-Match")))
	docID := r.PathValue("document_id")
	if s.guardHelpNote(w, r, docID) {
		return
	}

	// Without an explicit precondition, apply to the current revision and
	// retry once if another client writes in between.
	attempts := 1
	if baseRevisionID == "" {
		attempts = 2
	}
	var doc store.Document
	for attempt := 0; attempt < attempts; attempt++ {
		current, err := s.store.GetDocument(r.Context(), docID)
		if writeStoreError(w, err, "document_read_failed") {
			return
		}
		base := baseRevisionID
		if base == "" {
			base = current.CurrentRevisionID
		}
		body := joinNoteText(current.Body, req.Text, prepend)
		message := "append"
		if prepend {
			message = "prepend"
		}
		doc, err = s.store.UpdateDocument(r.Context(), store.UpdateDocumentRequest{
			ID:             docID,
			Title:          current.Title,
			Body:           body,
			BodyMIMEType:   current.BodyMIMEType,
			BaseRevisionID: base,
			Message:        message,
		})
		if err == nil {
			break
		}
		if errors.Is(err, store.ErrConflict) && baseRevisionID == "" && attempt+1 < attempts {
			continue
		}
		if writeStoreError(w, err, "document_update_failed") {
			return
		}
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	writeJSON(w, http.StatusOK, toAPIDocument(doc))
}

func joinNoteText(body, text string, prepend bool) string {
	if prepend {
		if body == "" {
			return text
		}
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text + body
	}
	if body == "" {
		return text
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body + text
}

// handleDocumentLines returns a 1-indexed inclusive slice of the note body.
func (s *Server) handleDocumentLines(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	doc, err := s.store.GetDocument(r.Context(), r.PathValue("document_id"))
	if writeStoreError(w, err, "document_read_failed") {
		return
	}
	lines := strings.Split(doc.Body, "\n")
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	end, _ := strconv.Atoi(r.URL.Query().Get("end"))
	if start < 1 {
		start = 1
	}
	if end < start || end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		start = len(lines)
	}
	writeJSON(w, http.StatusOK, api.DocumentLines{
		DocumentID: doc.ID,
		StartLine:  start,
		EndLine:    end,
		TotalLines: len(lines),
		Lines:      lines[start-1 : end],
	})
}

// handleDocumentSearchIn performs a case-insensitive in-note search and
// returns matches with line numbers and surrounding context.
func (s *Server) handleDocumentSearchIn(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	pattern := r.URL.Query().Get("pattern")
	if strings.TrimSpace(pattern) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "pattern is required")
		return
	}
	doc, err := s.store.GetDocument(r.Context(), r.PathValue("document_id"))
	if writeStoreError(w, err, "document_read_failed") {
		return
	}
	writeJSON(w, http.StatusOK, searchInNote(doc, pattern))
}

func searchInNote(doc store.Document, pattern string) api.NoteSearchResponse {
	lines := strings.Split(doc.Body, "\n")
	needle := strings.ToLower(pattern)
	matches := []api.NoteSearchMatch{}
	for i, line := range lines {
		if !strings.Contains(strings.ToLower(line), needle) {
			continue
		}
		contextStart := max(0, i-1)
		contextEnd := min(len(lines), i+2)
		matches = append(matches, api.NoteSearchMatch{
			Line:    i + 1,
			Text:    line,
			Context: lines[contextStart:contextEnd],
		})
		if len(matches) >= 100 {
			break
		}
	}
	return api.NoteSearchResponse{DocumentID: doc.ID, Pattern: pattern, Matches: matches}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
