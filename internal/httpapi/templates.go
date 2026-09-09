package httpapi

import (
	"net/http"

	"github.com/renesugar/notrios/internal/store"
)

// REST for note templates and task extraction (v0.6 F4).

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	templates, err := s.store.ListTemplates(r.Context(), r.URL.Query().Get("collection_id"))
	if writeStoreError(w, err, "template_list_failed") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
}

func (s *Server) handleTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	template, err := s.store.GetTemplate(r.Context(), r.PathValue("document_id"))
	if writeStoreError(w, err, "template_read_failed") {
		return
	}
	writeJSON(w, http.StatusOK, template)
}

// handleCreateFromTemplate fills a template in and stores the result.
//
// A malformed template is a `400`, not a `200` carrying an error: unlike a
// note-query block — which has to render inside a note that must still display
// — this call has one job, and half-doing it would leave a note with `{{...}}`
// in it that nobody asked for.
func (s *Server) handleCreateFromTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req struct {
		Title      string            `json:"title"`
		NotebookID string            `json:"notebook_id,omitempty"`
		Values     map[string]string `json:"values,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	doc, err := s.store.CreateFromTemplate(r.Context(), store.CreateFromTemplateRequest{
		TemplateID: r.PathValue("document_id"),
		Title:      req.Title,
		NotebookID: req.NotebookID,
		Values:     req.Values,
	})
	if writeStoreError(w, err, "template_create_failed") {
		return
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	writeJSON(w, http.StatusCreated, toAPIDocument(doc))
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	query := r.URL.Query()
	result, err := s.store.ListTasks(r.Context(), store.TaskListRequest{
		CollectionID: query.Get("collection_id"),
		DocumentID:   query.Get("document_id"),
		NotebookID:   query.Get("notebook_id"),
		State:        query.Get("state"),
		Limit:        queryLimit(r),
		// Off unless asked for: a checkbox in a quoted example is not work
		// somebody owes, and a task list that counted them would report on the
		// library's punctuation.
		Untagged: query.Get("untagged") == "true",
	})
	if writeStoreError(w, err, "task_list_failed") {
		return
	}
	writeJSON(w, http.StatusOK, result)
}
