package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// REST handlers for notebooks, tags, search notebooks, and the trash
// (Notrios redesign task R5). All handlers require a configured store; the
// scaffold in-memory mode has no notebook model.

func (s *Server) requireStore(w http.ResponseWriter) bool {
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "notebook APIs require a configured SQLite store")
		return false
	}
	return true
}

func toAPINotebook(nb store.Notebook) api.Notebook {
	return api.Notebook{
		ID:        nb.ID,
		ParentID:  nb.ParentID,
		Name:      nb.Name,
		IconEmoji: nb.IconEmoji,
		Builtin:   nb.Builtin,
		Position:  nb.Position,
		CreatedAt: nb.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: nb.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *Server) handleListNotebooks(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	notebooks, err := s.store.ListNotebooks(r.Context())
	if writeStoreError(w, err, "notebook_list_failed") {
		return
	}
	out := make([]api.Notebook, 0, len(notebooks))
	for _, nb := range notebooks {
		out = append(out, toAPINotebook(nb))
	}
	writeJSON(w, http.StatusOK, map[string]any{"notebooks": out})
}

// handleNotebookTree returns notebooks nested under their parents, in sidebar
// order (position, then name case-insensitively — the store list order).
func (s *Server) handleNotebookTree(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	notebooks, err := s.store.ListNotebooks(r.Context())
	if writeStoreError(w, err, "notebook_tree_failed") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notebooks": buildNotebookTree(notebooks)})
}

func buildNotebookTree(notebooks []store.Notebook) []api.NotebookTreeNode {
	children := map[string][]store.Notebook{}
	for _, nb := range notebooks {
		children[nb.ParentID] = append(children[nb.ParentID], nb)
	}
	var build func(parentID string) []api.NotebookTreeNode
	build = func(parentID string) []api.NotebookTreeNode {
		nodes := []api.NotebookTreeNode{}
		for _, nb := range children[parentID] {
			nodes = append(nodes, api.NotebookTreeNode{
				Notebook: toAPINotebook(nb),
				Children: build(nb.ID),
			})
		}
		return nodes
	}
	return build("")
}

func (s *Server) handleCreateNotebook(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req api.NotebookMutationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	create := store.CreateNotebookRequest{Name: *req.Name}
	if req.ParentID != nil {
		create.ParentID = *req.ParentID
	}
	if req.IconEmoji != nil {
		create.IconEmoji = *req.IconEmoji
	}
	if req.Position != nil {
		create.Position = *req.Position
	}
	nb, err := s.store.CreateNotebook(r.Context(), create)
	if writeStoreError(w, err, "notebook_create_failed") {
		return
	}
	writeJSON(w, http.StatusCreated, toAPINotebook(nb))
}

func (s *Server) handleNotebook(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	id := r.PathValue("notebook_id")
	switch r.Method {
	case http.MethodGet:
		nb, err := s.store.GetNotebook(r.Context(), id)
		if writeStoreError(w, err, "notebook_read_failed") {
			return
		}
		writeJSON(w, http.StatusOK, toAPINotebook(nb))
	case http.MethodPatch:
		var req api.NotebookMutationRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		nb, err := s.store.UpdateNotebook(r.Context(), store.UpdateNotebookRequest{
			ID:        id,
			Name:      req.Name,
			ParentID:  req.ParentID,
			IconEmoji: req.IconEmoji,
			Position:  req.Position,
		})
		if writeStoreError(w, err, "notebook_update_failed") {
			return
		}
		writeJSON(w, http.StatusOK, toAPINotebook(nb))
	case http.MethodDelete:
		if writeStoreError(w, s.store.DeleteNotebook(r.Context(), id), "notebook_delete_failed") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleNotebookNotes(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	page, err := s.store.ListNotebookDocuments(r.Context(), r.PathValue("notebook_id"), store.DocumentPageRequest{
		Limit:  queryLimit(r),
		Cursor: r.URL.Query().Get("cursor"),
	})
	if writeStoreError(w, err, "notebook_notes_failed") {
		return
	}
	writeJSON(w, http.StatusOK, api.DocumentPage{
		Documents:  toAPIDocuments(page.Documents),
		NextCursor: page.NextCursor,
	})
}

func toAPIDocuments(docs []store.Document) []api.Document {
	out := make([]api.Document, 0, len(docs))
	for _, doc := range docs {
		out = append(out, toAPIDocument(doc))
	}
	return out
}

func queryLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return limit
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	query, err := tagQueryFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "tag_query_invalid", err.Error())
		return
	}
	page, err := s.store.ListTags(r.Context(), query)
	if writeStoreError(w, err, "tag_list_failed") {
		return
	}
	if strings.TrimSpace(query.Name) != "" && len(page.Tags) == 0 {
		// Asking about one tag is a lookup, and a lookup that finds nothing is
		// a 404. Returning an empty list would make "no such tag" and "a tag
		// with nothing on it" the same answer, which is the distinction the
		// caller asked for.
		writeError(w, http.StatusNotFound, "tag_not_found", "no tag named "+strconv.Quote(query.Name))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": toAPITags(page.Tags), "truncated": page.Truncated})
}

// tagQueryFromRequest reads the narrowing a caller asked for.
func tagQueryFromRequest(r *http.Request) (store.TagQuery, error) {
	values := r.URL.Query()
	query := store.TagQuery{
		Name:   strings.TrimSpace(values.Get("name")),
		Prefix: strings.TrimSpace(values.Get("prefix")),
	}
	if query.Name != "" && query.Prefix != "" {
		return store.TagQuery{}, fmt.Errorf("name and prefix ask different questions; send one")
	}
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return store.TagQuery{}, fmt.Errorf("limit must be a positive whole number")
		}
		query.Limit = limit
	}
	return query, nil
}

func toAPITags(tags []store.Tag) []api.Tag {
	out := make([]api.Tag, 0, len(tags))
	for _, tag := range tags {
		out = append(out, api.Tag{ID: tag.ID, Name: tag.Name, NoteCount: tag.NoteCount})
	}
	return out
}

func (s *Server) handleDocumentTags(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	tags, err := s.store.ListDocumentTags(r.Context(), r.PathValue("document_id"))
	if writeStoreError(w, err, "document_tags_failed") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": toAPITags(tags)})
}

func (s *Server) handleDocumentTag(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	docID := r.PathValue("document_id")
	tagName := r.PathValue("tag")
	// A tag is a modification of a note the contract calls read-only, and it
	// outlives a reseed because `note_tags` is keyed by a stable document ID.
	// This route was the one mutation path without the guard until v0.6 F7.
	if s.guardReadOnlyNote(w, r, docID) {
		return
	}
	switch r.Method {
	case http.MethodPost:
		tag, err := s.store.AddDocumentTag(r.Context(), docID, tagName)
		if writeStoreError(w, err, "tag_add_failed") {
			return
		}
		writeJSON(w, http.StatusOK, api.Tag{ID: tag.ID, Name: tag.Name, NoteCount: tag.NoteCount})
	case http.MethodDelete:
		if writeStoreError(w, s.store.RemoveDocumentTag(r.Context(), docID, tagName), "tag_remove_failed") {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleRenameTag renames a tag hierarchy.
//
// `dry_run` defaults to **true**. A caller who forgets the field gets the
// report, and the only way to change the library is to say so. That asymmetry
// is deliberate: a rename that swept up a hierarchy nobody meant to touch is
// tedious to undo by hand, and the report costs one extra round trip.
func (s *Server) handleRenameTag(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req api.TagRenameRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	result, err := s.store.RenameTag(r.Context(), store.TagRenameRequest{
		From:            req.From,
		To:              req.To,
		IncludeChildren: req.IncludeChildren,
		DryRun:          dryRun,
	})
	if writeStoreError(w, err, "tag_rename_failed") {
		return
	}
	changes := make([]api.TagRenameChange, 0, len(result.Changes))
	for _, change := range result.Changes {
		changes = append(changes, api.TagRenameChange{
			TagID:           change.TagID,
			From:            change.From,
			To:              change.To,
			Action:          change.Action,
			MergedIntoTagID: change.MergedIntoTagID,
			Notes:           change.Notes,
			NotesGained:     change.NotesGained,
		})
	}
	writeJSON(w, http.StatusOK, api.TagRenameResult{
		From:            result.From,
		To:              result.To,
		IncludeChildren: result.IncludeChildren,
		DryRun:          result.DryRun,
		Changes:         changes,
		Notes:           result.Notes,
		Warnings:        result.Warnings,
	})
}

// handleNotebookDeletionPreview reports what a notebook deletion would do
// without doing it — the counts a confirmation needs and the re-homing rule it
// cannot infer.
func (s *Server) handleNotebookDeletionPreview(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	preview, err := s.store.PreviewNotebookDeletion(r.Context(), r.PathValue("notebook_id"))
	if writeStoreError(w, err, "notebook_deletion_preview_failed") {
		return
	}
	writeJSON(w, http.StatusOK, api.NotebookDeletionPreview{
		NotebookID:       preview.NotebookID,
		Name:             preview.Name,
		Notebooks:        preview.Notebooks,
		DescendantNames:  preview.DescendantNames,
		Truncated:        preview.Truncated,
		Notes:            preview.Notes,
		TrashedNotes:     preview.TrashedNotes,
		RehomeNotebookID: preview.RehomeNotebookID,
		Deletable:        preview.Deletable,
		Reason:           preview.Reason,
	})
}

func (s *Server) handleMoveDocumentNotebook(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req api.MoveDocumentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	doc, err := s.store.MoveDocumentToNotebook(r.Context(), r.PathValue("document_id"), req.NotebookID)
	if writeStoreError(w, err, "document_move_failed") {
		return
	}
	writeJSON(w, http.StatusOK, toAPIDocument(doc))
}

func toAPISearchNotebook(nb store.SearchNotebook) api.SearchNotebook {
	return api.SearchNotebook{
		ID:         nb.ID,
		Name:       nb.Name,
		IconEmoji:  nb.IconEmoji,
		Query:      nb.Query,
		Builtin:    nb.Builtin,
		SortAnchor: nb.SortAnchor,
		CreatedAt:  nb.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *Server) handleListSearchNotebooks(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	notebooks, err := s.store.ListSearchNotebooks(r.Context())
	if writeStoreError(w, err, "search_notebook_list_failed") {
		return
	}
	out := make([]api.SearchNotebook, 0, len(notebooks))
	for _, nb := range notebooks {
		out = append(out, toAPISearchNotebook(nb))
	}
	writeJSON(w, http.StatusOK, map[string]any{"search_notebooks": out})
}

func (s *Server) handleCreateSearchNotebook(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	var req api.SearchNotebookMutationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	nb, err := s.store.CreateSearchNotebook(r.Context(), store.CreateSearchNotebookRequest{
		Name:      req.Name,
		IconEmoji: req.IconEmoji,
		Query:     req.Query,
	})
	if writeStoreError(w, err, "search_notebook_create_failed") {
		return
	}
	writeJSON(w, http.StatusCreated, toAPISearchNotebook(nb))
}

func (s *Server) handleDeleteSearchNotebook(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	if writeStoreError(w, s.store.DeleteSearchNotebook(r.Context(), r.PathValue("search_notebook_id")), "search_notebook_delete_failed") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListTrash(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	page, err := s.store.ListTrash(r.Context(), store.DocumentPageRequest{
		Limit:  queryLimit(r),
		Cursor: r.URL.Query().Get("cursor"),
	})
	if writeStoreError(w, err, "trash_list_failed") {
		return
	}
	writeJSON(w, http.StatusOK, api.DocumentPage{
		Documents:  toAPIDocuments(page.Documents),
		NextCursor: page.NextCursor,
	})
}

func (s *Server) handleRestoreTrashedDocument(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	doc, err := s.store.RestoreDocument(r.Context(), r.PathValue("document_id"))
	if writeStoreError(w, err, "trash_restore_failed") {
		return
	}
	setRevisionETag(w, doc.CurrentRevisionID)
	writeJSON(w, http.StatusOK, toAPIDocument(doc))
}

func (s *Server) handlePurgeDocument(w http.ResponseWriter, r *http.Request) {
	if !s.requireStore(w) {
		return
	}
	documentID := r.PathValue("document_id")
	if !requireConfirmation(w, r, "purge-document:"+documentID) {
		return
	}
	purgeErr := error(nil)
	if canonical, ok := s.sqliteStore(); ok {
		if journal, journalErr := canonical.JournalStatus(r.Context()); journalErr == nil && journal.Enabled {
			if s.syncSecrets == nil {
				writeError(w, http.StatusServiceUnavailable, "sync_keys_unavailable", "signed permanent deletion requires the configured local sync secret provider")
				return
			}
			keys, keyErr := s.syncSecrets.Open()
			if keyErr != nil {
				writeError(w, http.StatusServiceUnavailable, "sync_keys_unavailable", "signed permanent deletion requires available local sync keys")
				return
			}
			purgeErr = canonical.PurgeDocumentWithCertificate(r.Context(), documentID, keys)
		} else {
			purgeErr = s.store.PurgeDocument(r.Context(), documentID)
		}
	} else {
		purgeErr = s.store.PurgeDocument(r.Context(), documentID)
	}
	if writeStoreError(w, purgeErr, "trash_purge_failed") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
