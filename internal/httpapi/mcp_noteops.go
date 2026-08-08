package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/localize"
	"github.com/renesugar/notrios/internal/store"
)

// MCP note-content read tools and profile-gated write tools (Notrios redesign
// task R8). Write tools require the "editor" MCP profile; the default profile
// stays read-only. Mutations use optimistic revision preconditions where the
// operation is destructive (update_note, delete_note); append/prepend/edit
// read-modify-write against the current revision like their REST equivalents.

func (s *Server) mcpGetNoteLineRange(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		DocumentID string `json:"document_id,omitempty"`
		URI        string `json:"uri,omitempty"`
		StartLine  int    `json:"start_line,omitempty"`
		EndLine    int    `json:"end_line,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	doc, err := s.mcpResolveDocument(r, args.DocumentID, args.URI)
	if err != nil {
		return mcpToolResult{}, err
	}
	lines := strings.Split(doc.Body, "\n")
	start := args.StartLine
	end := args.EndLine
	if start < 1 {
		start = 1
	}
	if end < start || end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		start = len(lines)
	}
	return mcpStructured(api.DocumentLines{
		DocumentID: doc.ID,
		StartLine:  start,
		EndLine:    end,
		TotalLines: len(lines),
		Lines:      lines[start-1 : end],
	})
}

func (s *Server) mcpSearchInNote(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		DocumentID string `json:"document_id,omitempty"`
		URI        string `json:"uri,omitempty"`
		Pattern    string `json:"pattern,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return mcpToolResult{}, fmt.Errorf("pattern is required")
	}
	doc, err := s.mcpResolveDocument(r, args.DocumentID, args.URI)
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(searchInNote(doc, args.Pattern))
}

func (s *Server) mcpGetNotebookNotes(r *http.Request, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		NotebookID string `json:"notebook_id,omitempty"`
		Limit      int    `json:"limit,omitempty"`
		Cursor     string `json:"cursor,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	if strings.TrimSpace(args.NotebookID) == "" {
		return mcpToolResult{}, fmt.Errorf("notebook_id is required")
	}
	page, err := s.store.ListNotebookDocuments(r.Context(), args.NotebookID, store.DocumentPageRequest{
		Limit:  args.Limit,
		Cursor: args.Cursor,
	})
	if err != nil {
		return mcpToolResult{}, err
	}
	return mcpStructured(api.DocumentPage{
		Documents:  toAPIDocuments(page.Documents),
		NextCursor: page.NextCursor,
	})
}

func (s *Server) mcpResolveDocument(r *http.Request, documentID, uri string) (store.Document, error) {
	id := firstNonEmpty(documentID, documentIDFromURI(uri))
	if id == "" {
		return store.Document{}, fmt.Errorf("document_id or document:// URI is required")
	}
	return s.store.GetDocument(r.Context(), id)
}

func (s *Server) mcpWriteTool(r *http.Request, name string, raw json.RawMessage) (mcpToolResult, error) {
	var args struct {
		DocumentID     string   `json:"document_id,omitempty"`
		URI            string   `json:"uri,omitempty"`
		Title          string   `json:"title,omitempty"`
		Body           string   `json:"body,omitempty"`
		NotebookID     string   `json:"notebook_id,omitempty"`
		BaseRevisionID string   `json:"base_revision_id,omitempty"`
		Text           string   `json:"text,omitempty"`
		Search         string   `json:"search,omitempty"`
		Replace        string   `json:"replace,omitempty"`
		ReplaceAll     bool     `json:"replace_all,omitempty"`
		DryRun         bool     `json:"dry_run,omitempty"`
		AllowReview    bool     `json:"allow_review,omitempty"`
		Tags           []string `json:"tags,omitempty"`
	}
	if err := unmarshalMCPArgs(raw, &args); err != nil {
		return mcpToolResult{}, err
	}
	ctx := r.Context()

	switch name {
	case "create_note":
		if strings.TrimSpace(args.Title) == "" {
			return mcpToolResult{}, fmt.Errorf("title is required")
		}
		doc, err := s.store.CreateDocument(ctx, store.CreateDocumentRequest{
			Title:      args.Title,
			Body:       args.Body,
			NotebookID: args.NotebookID,
			Message:    "mcp create",
		})
		if err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(toAPIDocument(doc))

	case "update_note":
		doc, err := s.mcpResolveDocument(r, args.DocumentID, args.URI)
		if err != nil {
			return mcpToolResult{}, err
		}
		if store.IsReadOnlyNotebook(doc.NotebookID) {
			return mcpToolResult{}, fmt.Errorf("notes in the %s notebook are read-only", store.ReadOnlyNotebookName(doc.NotebookID))
		}
		title := firstNonEmpty(args.Title, doc.Title)
		body := doc.Body
		if args.Body != "" {
			body = args.Body
		}
		updated, err := s.store.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID:             doc.ID,
			Title:          title,
			Body:           body,
			BodyMIMEType:   doc.BodyMIMEType,
			BaseRevisionID: args.BaseRevisionID,
			Message:        "mcp update",
		})
		if err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(toAPIDocument(updated))

	case "append_to_note", "prepend_to_note":
		if args.Text == "" {
			return mcpToolResult{}, fmt.Errorf("text is required")
		}
		doc, err := s.mcpResolveDocument(r, args.DocumentID, args.URI)
		if err != nil {
			return mcpToolResult{}, err
		}
		if store.IsReadOnlyNotebook(doc.NotebookID) {
			return mcpToolResult{}, fmt.Errorf("notes in the %s notebook are read-only", store.ReadOnlyNotebookName(doc.NotebookID))
		}
		body := joinNoteText(doc.Body, args.Text, name == "prepend_to_note")
		updated, err := s.store.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID:             doc.ID,
			Title:          doc.Title,
			Body:           body,
			BodyMIMEType:   doc.BodyMIMEType,
			BaseRevisionID: doc.CurrentRevisionID,
			Message:        strings.ReplaceAll(name, "_", " "),
		})
		if err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(toAPIDocument(updated))

	case "edit_note":
		if args.Search == "" {
			return mcpToolResult{}, fmt.Errorf("search is required")
		}
		doc, err := s.mcpResolveDocument(r, args.DocumentID, args.URI)
		if err != nil {
			return mcpToolResult{}, err
		}
		if store.IsReadOnlyNotebook(doc.NotebookID) {
			return mcpToolResult{}, fmt.Errorf("notes in the %s notebook are read-only", store.ReadOnlyNotebookName(doc.NotebookID))
		}
		body, err := applySurgicalEdits(doc.Body, []api.SurgicalEdit{{Search: args.Search, Replace: args.Replace, ReplaceAll: args.ReplaceAll}})
		if err != nil {
			return mcpToolResult{}, err
		}
		if args.DryRun {
			doc.Body = body
			return mcpStructured(map[string]any{"dry_run": true, "document": toAPIDocument(doc)})
		}
		updated, err := s.store.UpdateDocument(ctx, store.UpdateDocumentRequest{
			ID:             doc.ID,
			Title:          doc.Title,
			Body:           body,
			BodyMIMEType:   doc.BodyMIMEType,
			BaseRevisionID: doc.CurrentRevisionID,
			Message:        "mcp edit",
		})
		if err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(toAPIDocument(updated))

	case "tag_note", "untag_note":
		// Tagging one note is a single-note write, which is what `editor` is
		// for. Before v0.6 F7 the only way to do it over MCP was `run_batch`
		// under `organizer` — so labelling one note you just created required
		// granting the ability to trash five hundred. That is a scope-design
		// inconsistency rather than a missing convenience.
		if len(args.Tags) == 0 {
			return mcpToolResult{}, fmt.Errorf("at least one tag is required")
		}
		if len(args.Tags) > store.MaxBatchTags {
			return mcpToolResult{}, fmt.Errorf("at most %d tags at once", store.MaxBatchTags)
		}
		doc, err := s.mcpResolveDocument(r, args.DocumentID, args.URI)
		if err != nil {
			return mcpToolResult{}, err
		}
		if store.IsReadOnlyNotebook(doc.NotebookID) {
			return mcpToolResult{}, fmt.Errorf("notes in the %s notebook are read-only", store.ReadOnlyNotebookName(doc.NotebookID))
		}
		for _, tag := range args.Tags {
			if name == "tag_note" {
				if _, err := s.store.AddDocumentTag(ctx, doc.ID, tag); err != nil {
					return mcpToolResult{}, err
				}
				continue
			}
			if err := s.store.RemoveDocumentTag(ctx, doc.ID, tag); err != nil && !errors.Is(err, store.ErrNotFound) {
				return mcpToolResult{}, err
			}
		}
		tags, err := s.store.ListDocumentTags(ctx, doc.ID)
		if err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(map[string]any{"document_id": doc.ID, "tags": tags})

	case "delete_note":
		doc, err := s.mcpResolveDocument(r, args.DocumentID, args.URI)
		if err != nil {
			return mcpToolResult{}, err
		}
		if store.IsReadOnlyNotebook(doc.NotebookID) {
			return mcpToolResult{}, fmt.Errorf("notes in the %s notebook are read-only", store.ReadOnlyNotebookName(doc.NotebookID))
		}
		if err := s.store.DeleteDocument(ctx, store.DeleteDocumentRequest{
			ID:             doc.ID,
			BaseRevisionID: args.BaseRevisionID,
			Message:        "mcp delete",
		}); err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(map[string]any{"deleted": true, "document_id": doc.ID, "note": "moved to Trash; restore via the trash API"})

	case "move_note_to_notebook":
		doc, err := s.store.MoveDocumentToNotebook(ctx, firstNonEmpty(args.DocumentID, documentIDFromURI(args.URI)), args.NotebookID)
		if err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(toAPIDocument(doc))
	case "localize_remote_media":
		if s.localizer == nil {
			return mcpToolResult{}, fmt.Errorf("store is not wired")
		}
		if strings.TrimSpace(args.BaseRevisionID) == "" && !args.DryRun {
			return mcpToolResult{}, fmt.Errorf("base_revision_id is required (localization rewrites the note)")
		}
		result, err := s.localizer.LocalizeDocument(ctx, localize.Options{
			DocumentID:     firstNonEmpty(args.DocumentID, documentIDFromURI(args.URI)),
			BaseRevisionID: args.BaseRevisionID,
			DryRun:         args.DryRun,
			AllowReview:    args.AllowReview,
		})
		if err != nil {
			return mcpToolResult{}, err
		}
		return mcpStructured(result)
	}
	return mcpToolResult{}, fmt.Errorf("unknown MCP write tool %q", name)
}
