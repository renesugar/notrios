package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ApplyImportDocumentBatch commits canonical document state and the durable
// checkpoint in one bounded transaction. Importers plan actions with the
// existing batch read APIs; this method is intentionally not exposed over
// REST/MCP and accepts no SQL or filesystem input.
func (s *SQLiteStore) ApplyImportDocumentBatch(ctx context.Context, req ImportDocumentBatchRequest) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(req.Documents) == 0 || len(req.Documents) > maxImportLookupItems {
		return fmt.Errorf("%w: import document batch contains %d items; expected 1-%d", ErrInvalidInput, len(req.Documents), maxImportLookupItems)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	changedBodies := make([]CreateDocumentRequest, 0, len(req.Documents))
	for _, mutation := range req.Documents {
		action := strings.TrimSpace(mutation.Action)
		if (mutation.SkipSource || mutation.SkipState) && action != "unchanged" {
			return fmt.Errorf("%w: import metadata may be skipped only for unchanged documents", ErrInvalidInput)
		}
		if mutation.SkipDocument {
			continue
		}
		document := NormalizeCreateRequest(mutation.Document)
		if document.PreferredID == "" {
			return fmt.Errorf("%w: import document ID is required", ErrInvalidInput)
		}
		if IsReadOnlyNotebook(document.NotebookID) {
			return fmt.Errorf("%w: imported notes cannot be placed in %s", ErrProtected, ReadOnlyNotebookName(document.NotebookID))
		}
		if exists, err := s.notebookExistsLocked(document.NotebookID); err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("%w: notebook %q", ErrNotFound, document.NotebookID)
		}

		switch action {
		case "create":
			revisionID, err := NewID("rev")
			if err != nil {
				return err
			}
			if err := s.execPreparedLocked(`INSERT INTO documents(id, collection_id, notebook_id, title, body_mime_type, current_revision_id)
				VALUES(?, ?, ?, ?, ?, ?)`, document.PreferredID, document.CollectionID, document.NotebookID, document.Title, document.BodyMIMEType, revisionID); err != nil {
				return err
			}
			if err := s.execPreparedLocked(`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message)
				VALUES(?, ?, ?, ?, ?, ?)`, revisionID, document.PreferredID, document.Title, document.Body, document.BodyMIMEType, document.Message); err != nil {
				return err
			}
			if err := s.execPreparedLocked(`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`,
				document.PreferredID, document.CollectionID, document.Title, document.Body); err != nil {
				return err
			}
			if err := s.enqueueProjectionLocked(document.PreferredID, "upsert"); err != nil {
				return err
			}
			changedBodies = append(changedBodies, document)
		case "update":
			current, err := s.getDocumentLocked(document.PreferredID)
			if err != nil {
				return err
			}
			if strings.TrimSpace(mutation.BaseRevisionID) == "" {
				return ErrPreconditionRequired
			}
			if current.CurrentRevisionID != mutation.BaseRevisionID {
				return ErrConflict
			}
			if current.CollectionID != document.CollectionID {
				return fmt.Errorf("%w: import cannot change document collection", ErrInvalidInput)
			}
			contentChanged := current.Title != document.Title || current.Body != document.Body || current.BodyMIMEType != document.BodyMIMEType
			if contentChanged {
				revisionID, err := NewID("rev")
				if err != nil {
					return err
				}
				if err := s.execPreparedLocked(`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type, message)
					VALUES(?, ?, ?, ?, ?, ?)`, revisionID, document.PreferredID, document.Title, document.Body, document.BodyMIMEType, document.Message); err != nil {
					return err
				}
				if err := s.execPreparedLocked(`UPDATE documents SET notebook_id = ?, title = ?, body_mime_type = ?, current_revision_id = ?, updated_at = CURRENT_TIMESTAMP
					WHERE id = ? AND deleted_at IS NULL`, document.NotebookID, document.Title, document.BodyMIMEType, revisionID, document.PreferredID); err != nil {
					return err
				}
				if err := s.execPreparedLocked(`DELETE FROM documents_fts WHERE document_id = ?`, document.PreferredID); err != nil {
					return err
				}
				if err := s.execPreparedLocked(`INSERT INTO documents_fts(document_id, collection_id, title, body) VALUES(?, ?, ?, ?)`,
					document.PreferredID, document.CollectionID, document.Title, document.Body); err != nil {
					return err
				}
				if err := s.enqueueProjectionLocked(document.PreferredID, "upsert"); err != nil {
					return err
				}
				changedBodies = append(changedBodies, document)
			} else if current.NotebookID != document.NotebookID {
				if err := s.execPreparedLocked(`UPDATE documents SET notebook_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, document.NotebookID, document.PreferredID); err != nil {
					return err
				}
			}
		case "unchanged":
			if _, err := s.getDocumentLocked(document.PreferredID); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unsupported import document action %q", ErrInvalidInput, mutation.Action)
		}
	}

	// All documents in the batch now exist, so same-batch canonical targets can
	// resolve. A final bounded importer link phase resolves targets in later
	// batches without creating revisions or projection jobs.
	for _, document := range changedBodies {
		if err := s.rebuildDocumentLinksLocked(document.PreferredID, document.CollectionID, document.Body); err != nil {
			return err
		}
	}

	for _, mutation := range req.Documents {
		if !mutation.SkipDocument {
			documentID := strings.TrimSpace(mutation.Document.PreferredID)
			if !mutation.SkipSource {
				if strings.TrimSpace(mutation.Source.DocumentID) != documentID {
					return fmt.Errorf("%w: import provenance document ID does not match mutation", ErrInvalidInput)
				}
				if err := s.setDocumentSourceLocked(mutation.Source); err != nil {
					return err
				}
			}
			for _, name := range mutation.RemoveTags {
				tag, found, err := s.findTagByNameLocked(strings.TrimSpace(name))
				if err != nil {
					return err
				}
				if found {
					if err := s.execPreparedLocked(`DELETE FROM note_tags WHERE document_id = ? AND tag_id = ?`, documentID, tag.ID); err != nil {
						return err
					}
				}
			}
			for _, name := range mutation.AddTags {
				tag, found, err := s.findTagByNameLocked(strings.TrimSpace(name))
				if err != nil {
					return err
				}
				if !found {
					return fmt.Errorf("%w: import tag %q", ErrNotFound, name)
				}
				if err := s.execPreparedLocked(`INSERT OR IGNORE INTO note_tags(document_id, tag_id) VALUES(?, ?)`, documentID, tag.ID); err != nil {
					return err
				}
			}
			for _, reference := range mutation.Resources {
				reference = NormalizeAttachResourceRequest(reference)
				if reference.DocumentID != documentID {
					return fmt.Errorf("%w: import resource document ID does not match mutation", ErrInvalidInput)
				}
				if _, err := s.getResourceLocked(reference.ResourceID); err != nil {
					if err == ErrNotFound {
						continue
					}
					return err
				}
				if err := s.execPreparedLocked(`INSERT INTO document_resource_refs(document_id, resource_id, relation_type, ordinal, anchor_json)
					VALUES(?, ?, ?, ?, ?)
					ON CONFLICT(document_id, resource_id, relation_type, ordinal) DO UPDATE
					SET anchor_json = excluded.anchor_json
					WHERE anchor_json IS NOT excluded.anchor_json`, documentID, reference.ResourceID, reference.RelationType, strconv.Itoa(reference.Ordinal), reference.AnchorJSON); err != nil {
					return err
				}
				if err := s.execPreparedLocked(`UPDATE resources SET unreferenced_at = NULL, unreferenced_reason = '' WHERE id = ?`, reference.ResourceID); err != nil {
					return err
				}
			}
		}
		if !mutation.SkipState {
			if err := s.putImportItemStateLocked(mutation.State); err != nil {
				return err
			}
		}
	}
	if err := s.putImportCheckpointLocked(req.Checkpoint); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SQLiteStore) RebuildImportDocumentLinksBatch(ctx context.Context, req ImportLinkBatchRequest) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(req.DocumentIDs) == 0 || len(req.DocumentIDs) > maxImportLookupItems {
		return fmt.Errorf("%w: import link batch contains %d items; expected 1-%d", ErrInvalidInput, len(req.DocumentIDs), maxImportLookupItems)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	for _, documentID := range req.DocumentIDs {
		document, err := s.getDocumentLocked(strings.TrimSpace(documentID))
		if err != nil {
			if err == ErrNotFound {
				continue
			}
			return err
		}
		if err := s.rebuildDocumentLinksLocked(document.ID, document.CollectionID, document.Body); err != nil {
			return err
		}
	}
	if err := s.putImportCheckpointLocked(req.Checkpoint); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}
