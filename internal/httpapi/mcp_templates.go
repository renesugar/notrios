package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/renesugar/notrios/internal/store"
)

// mcpListTemplates lists the templates a caller could use.
//
// Read-shaped: it returns what each template *asks for*, so an agent can decide
// whether it has the values before trying.
func (s *Server) mcpListTemplates(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		CollectionID string `json:"collection_id,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	templates, err := s.store.ListTemplates(r.Context(), args.CollectionID)
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(map[string]any{"templates": templates, "variables": store.TemplateVariables()})
}

// mcpCreateFromTemplate is an `editor`-scope tool: it creates a note.
func (s *Server) mcpCreateFromTemplate(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		TemplateID string            `json:"template_id,omitempty"`
		Title      string            `json:"title,omitempty"`
		NotebookID string            `json:"notebook_id,omitempty"`
		Values     map[string]string `json:"values,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	doc, err := s.store.CreateFromTemplate(r.Context(), store.CreateFromTemplateRequest{
		TemplateID: args.TemplateID,
		Title:      args.Title,
		NotebookID: args.NotebookID,
		Values:     args.Values,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(toAPIDocument(doc))
}

// mcpListTasks extracts checkbox items.
func (s *Server) mcpListTasks(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		CollectionID string `json:"collection_id,omitempty"`
		DocumentID   string `json:"document_id,omitempty"`
		NotebookID   string `json:"notebook_id,omitempty"`
		State        string `json:"state,omitempty"`
		Limit        int    `json:"limit,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	result, err := s.store.ListTasks(r.Context(), store.TaskListRequest{
		CollectionID: args.CollectionID,
		DocumentID:   args.DocumentID,
		NotebookID:   args.NotebookID,
		State:        args.State,
		Limit:        args.Limit,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(result)
}
