package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var _ RestoreTarget = (*SQLiteStore)(nil)

// LibraryIsEmpty reports whether any document exists. Restore intent depends
// on it: adopt requires an empty target, replace requires a populated one.
func (s *SQLiteStore) LibraryIsEmpty(ctx context.Context) (bool, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	count, err := s.countLocked(`SELECT COUNT(*) FROM documents`)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// ClearLibraryForReplace removes canonical note state in one transaction so a
// replacement restore begins from a fresh initialized database. Asset bytes
// are left in place: they are content-addressed, so a later restore reuses
// them and ordinary garbage collection reclaims anything orphaned.
func (s *SQLiteStore) ClearLibraryForReplace(ctx context.Context) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
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
	// Order matters: dependents before the rows they reference.
	for _, statement := range []string{
		`DELETE FROM documents_fts`,
		`DELETE FROM document_links`,
		`DELETE FROM document_resource_refs`,
		`DELETE FROM note_tags`,
		`DELETE FROM document_sources`,
		`DELETE FROM index_outbox`,
		`DELETE FROM document_revisions`,
		`DELETE FROM documents`,
		`DELETE FROM resources`,
		`DELETE FROM source_bundle_items`,
		`DELETE FROM import_item_states`,
		`DELETE FROM import_checkpoints`,
		`DELETE FROM notebooks`,
		`DELETE FROM search_notebooks`,
		`DELETE FROM tags`,
	} {
		if err := s.execLocked(statement); err != nil {
			return fmt.Errorf("clear for replace: %w", err)
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// AdoptDatabaseIdentity preserves the archive's logical database universe
// while always minting a fresh replica ID, so a restored writable copy never
// impersonates the copy that produced the archive.
func (s *SQLiteStore) AdoptDatabaseIdentity(ctx context.Context, databaseID string) (DatabaseIdentity, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return DatabaseIdentity{}, err
	}
	if strings.TrimSpace(databaseID) == "" {
		return DatabaseIdentity{}, fmt.Errorf("%w: a database ID is required", ErrInvalidInput)
	}
	replicaID, err := NewID("replica")
	if err != nil {
		return DatabaseIdentity{}, err
	}
	s.mu.Lock()
	err = s.execPreparedLocked(`UPDATE database_identity SET database_id = ?, replica_id = ?,
		replica_created_at = CURRENT_TIMESTAMP WHERE singleton = 1`, databaseID, replicaID)
	s.mu.Unlock()
	if err != nil {
		return DatabaseIdentity{}, err
	}
	return s.GetDatabaseIdentity(ctx)
}

// AdmitRestoredBlob streams archive bytes into the content-addressed asset
// store. The archive's declared MIME type is a hint only: admission sniffs the
// bytes exactly as an upload would, so a restore cannot be used to install
// content the ordinary resource path would refuse.
func (s *SQLiteStore) AdmitRestoredBlob(ctx context.Context, expectedSHA256, mimeType string, content io.Reader) (int64, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	stored, cleanup, err := s.writeBlob(ctx, content, mimeType)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return 0, err
	}
	if stored.SHA256 != expectedSHA256 {
		return 0, fmt.Errorf("%w: restored blob hashed to %s but the archive named %s", ErrInvalidInput, stored.SHA256, expectedSHA256)
	}
	s.mu.Lock()
	err = s.execPreparedLocked(`INSERT INTO blobs(sha256, storage_path, size_bytes, mime_type)
		VALUES(?, ?, ?, ?) ON CONFLICT(sha256) DO NOTHING`,
		stored.SHA256, stored.StoragePath, strconv.FormatInt(stored.SizeBytes, 10), stored.MIMEType)
	s.mu.Unlock()
	if err != nil {
		return 0, err
	}
	return stored.SizeBytes, nil
}

// AdmitRestoredSourceBundle streams exact preserved source bytes into the
// source-bundle namespace. That namespace sits deliberately outside `blobs`
// and ordinary resource garbage collection, so a restored bundle must never
// create a blobs row: doing so would expose preserved source bytes to a GC
// that is not supposed to be able to reach them.
func (s *SQLiteStore) AdmitRestoredSourceBundle(ctx context.Context, expectedSHA256 string, content io.Reader) (int64, string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return 0, "", err
	}
	bundleRoot := filepath.Join(s.assetRoot, "source-bundles")
	if err := os.MkdirAll(bundleRoot, 0o755); err != nil {
		return 0, "", err
	}
	temporary, err := os.CreateTemp(bundleRoot, "restoring-*")
	if err != nil {
		return 0, "", err
	}
	name := temporary.Name()
	digest := sha256.New()
	size, copyErr := copyWithContext(ctx, io.MultiWriter(temporary, digest), content)
	closeErr := temporary.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(name)
		if copyErr != nil {
			return 0, "", copyErr
		}
		return 0, "", closeErr
	}
	hash := hex.EncodeToString(digest.Sum(nil))
	if hash != expectedSHA256 {
		_ = os.Remove(name)
		return 0, "", fmt.Errorf("%w: restored source bundle hashed to %s but the archive named %s", ErrInvalidInput, hash, expectedSHA256)
	}
	relative := filepath.Join("source-bundles", "sha256", hash[0:2], hash[2:4], hash)
	final := filepath.Join(s.assetRoot, relative)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		_ = os.Remove(name)
		return 0, "", err
	}
	if _, statErr := os.Stat(final); statErr == nil {
		_ = os.Remove(name)
	} else if err := os.Rename(name, final); err != nil {
		_ = os.Remove(name)
		return 0, "", err
	}
	return size, filepath.ToSlash(relative), nil
}

// ApplyRestoreRecords writes one bounded batch atomically. With additive true
// an identity the target already holds is reported as a conflict and left
// untouched; a replacement restore runs against a cleared library so nothing
// collides.
func (s *SQLiteStore) ApplyRestoreRecords(ctx context.Context, batch RestoreRecords, additive bool) ([]RestoreConflict, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()

	conflicts := []RestoreConflict{}
	skip := map[string]bool{}
	note := func(kind, id, reason string) {
		conflicts = append(conflicts, RestoreConflict{Kind: kind, ID: id, Reason: reason})
		skip[kind+"\x00"+id] = true
	}
	taken := func(kind, table, column, id string) (bool, error) {
		if !additive {
			return false, nil
		}
		count, err := s.countLocked(`SELECT COUNT(*) FROM `+table+` WHERE `+column+` = ?`, id)
		if err != nil {
			return false, err
		}
		if count > 0 {
			note(kind, id, "already present in the target")
			return true, nil
		}
		return false, nil
	}

	for _, collection := range batch.Collections {
		if exists, err := taken("collection", "collections", "id", collection.ID); err != nil {
			return nil, err
		} else if exists {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO collections(id, name, description, created_at)
			VALUES(?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
			collection.ID, collection.Name, collection.Description, restoreTimestamp(collection.CreatedAt)); err != nil {
			return nil, err
		}
	}
	for _, notebook := range batch.Notebooks {
		if exists, err := taken("notebook", "notebooks", "id", notebook.ID); err != nil {
			return nil, err
		} else if exists {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO notebooks(id, parent_id, name, icon_emoji, builtin, position, created_at, updated_at)
			VALUES(?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
			notebook.ID, nullableText(notebook.ParentID), notebook.Name, notebook.IconEmoji,
			boolText(notebook.Builtin), strconv.Itoa(notebook.Position),
			restoreTimestamp(notebook.CreatedAt), restoreTimestamp(notebook.UpdatedAt)); err != nil {
			return nil, err
		}
	}
	for _, searchNotebook := range batch.SearchNotebooks {
		if exists, err := taken("search_notebook", "search_notebooks", "id", searchNotebook.ID); err != nil {
			return nil, err
		} else if exists {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO search_notebooks(id, name, icon_emoji, query, builtin, sort_anchor, created_at)
			VALUES(?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
			searchNotebook.ID, searchNotebook.Name, searchNotebook.IconEmoji, searchNotebook.Query,
			boolText(searchNotebook.Builtin), searchNotebook.SortAnchor, restoreTimestamp(searchNotebook.CreatedAt)); err != nil {
			return nil, err
		}
	}
	for _, tag := range batch.Tags {
		if exists, err := taken("tag", "tags", "id", tag.ID); err != nil {
			return nil, err
		} else if exists {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO tags(id, name, created_at) VALUES(?, ?, ?)
			ON CONFLICT(id) DO NOTHING`, tag.ID, tag.Name, restoreTimestamp(tag.CreatedAt)); err != nil {
			return nil, err
		}
	}
	for _, resource := range batch.Resources {
		if exists, err := taken("resource", "resources", "id", resource.ID); err != nil {
			return nil, err
		} else if exists {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO resources(id, collection_id, blob_sha256, filename, mime_type,
				metadata_json, unreferenced_at, unreferenced_reason, created_at)
			VALUES(?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?) ON CONFLICT(id) DO NOTHING`,
			resource.ID, resource.CollectionID, resource.BlobSHA256, nullableText(resource.Filename),
			resource.MIMEType, jsonOrEmptyObject(resource.MetadataJSON),
			nullableText(restoreTimestamp(resource.UnreferencedAt)), resource.UnreferencedReason,
			restoreTimestamp(resource.CreatedAt)); err != nil {
			return nil, err
		}
	}
	for _, document := range batch.Documents {
		row := document.Document
		if exists, err := taken("document", "documents", "id", row.ID); err != nil {
			return nil, err
		} else if exists {
			continue
		}
		// The title is filled in by FinalizeRestoredDocuments, because the
		// current revision may arrive in a later records chunk.
		if err := s.execPreparedLocked(`INSERT INTO documents(id, collection_id, notebook_id, title, body_mime_type,
				current_revision_id, deleted_at, created_at, updated_at)
			VALUES(?, ?, NULLIF(?, ''), '', ?, ?, NULLIF(?, ''), ?, ?) ON CONFLICT(id) DO NOTHING`,
			row.ID, row.CollectionID, nullableText(row.NotebookID),
			defaultMIME(document.BodyMIMEType), row.CurrentRevisionID,
			nullableText(restoreTimestamp(row.DeletedAt)),
			restoreTimestamp(row.CreatedAt), restoreTimestamp(row.UpdatedAt)); err != nil {
			return nil, err
		}
	}
	for _, revision := range batch.Revisions {
		if skip["document\x00"+revision.DocumentID] {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO document_revisions(id, document_id, title, body, body_mime_type,
				metadata_json, message, created_at)
			VALUES(?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?) ON CONFLICT(id) DO NOTHING`,
			revision.ID, revision.DocumentID, revision.Title, revision.Body, defaultMIME(revision.BodyMIMEType),
			jsonOrEmptyObject(revision.MetadataJSON), nullableText(revision.Message),
			restoreTimestamp(revision.CreatedAt)); err != nil {
			return nil, err
		}
	}
	for _, membership := range batch.DocumentTags {
		if skip["document\x00"+membership.DocumentID] {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO note_tags(document_id, tag_id) VALUES(?, ?)
			ON CONFLICT(document_id, tag_id) DO NOTHING`, membership.DocumentID, membership.TagID); err != nil {
			return nil, err
		}
	}
	for _, reference := range batch.DocumentResources {
		if skip["document\x00"+reference.DocumentID] || skip["resource\x00"+reference.ResourceID] {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO document_resource_refs(document_id, resource_id, relation_type, ordinal, anchor_json)
			VALUES(?, ?, ?, ?, ?) ON CONFLICT(document_id, resource_id, relation_type, ordinal) DO NOTHING`,
			reference.DocumentID, reference.ResourceID, reference.RelationType,
			strconv.Itoa(reference.Ordinal), jsonOrEmptyObject(reference.AnchorJSON)); err != nil {
			return nil, err
		}
	}
	for _, link := range batch.Links {
		if skip["document\x00"+link.SourceDocumentID] {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO document_links(source_document_id, target_document_id, target_resource_id,
				target_uri, relation_type, source_format, raw_target, display_text, anchor_type, anchor_value,
				context, source_start_byte, source_end_byte, source_line, source_column, resolution_status)
			VALUES(?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''),
				NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?)`,
			link.SourceDocumentID, nullableText(link.TargetDocumentID), nullableText(link.TargetResourceID),
			nullableText(link.TargetURI), link.RelationType, link.SourceFormat, link.RawTarget,
			nullableText(link.DisplayText), nullableText(link.AnchorType), nullableText(link.AnchorValue),
			nullableText(link.Context), strconv.Itoa(link.SourceStartByte), strconv.Itoa(link.SourceEndByte),
			strconv.Itoa(link.SourceLine), strconv.Itoa(link.SourceColumn), link.ResolutionStatus); err != nil {
			return nil, err
		}
	}
	for _, source := range batch.Provenance {
		if skip["document\x00"+source.DocumentID] {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO document_sources(document_id, source_system, external_id, author,
				author_id, thread_id, reply_to, source_url, published_at, published_ts, metadata_json, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(document_id) DO NOTHING`,
			source.DocumentID, source.SourceSystem, source.ExternalID, source.Author, source.AuthorID,
			source.ThreadID, source.ReplyTo, source.SourceURL, source.PublishedAt,
			strconv.FormatInt(source.PublishedTS, 10), jsonOrEmptyObject(source.MetadataJSON),
			restoreTimestamp(source.CreatedAt), restoreTimestamp(source.UpdatedAt)); err != nil {
			return nil, err
		}
	}
	for _, bundle := range batch.SourceBundles {
		if err := s.execPreparedLocked(`INSERT INTO source_bundle_items(source_system, source_key, collection_id, item_key,
				item_type, external_id, relative_path, sha256, size_bytes, storage_path, property_order_json, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(source_system, source_key, collection_id, item_key) DO NOTHING`,
			bundle.SourceSystem, bundle.SourceKey, bundle.CollectionID, bundle.ItemKey, bundle.ItemType,
			bundle.ExternalID, bundle.RelativePath, bundle.SHA256, strconv.FormatInt(bundle.SizeBytes, 10),
			bundle.StoragePath, propertyOrderJSON(bundle.PropertyOrder), restoreTimestamp(bundle.UpdatedAt)); err != nil {
			return nil, err
		}
	}

	if err := s.execLocked("COMMIT"); err != nil {
		return nil, err
	}
	committed = true
	return conflicts, nil
}

// FinalizeRestoredDocuments derives each document's denormalized title from
// its current revision and rebuilds the search index for current notes. Doing
// it as a join means restore never has to hold a revision map in memory.
func (s *SQLiteStore) FinalizeRestoredDocuments(ctx context.Context) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
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
	for _, statement := range []string{
		`UPDATE documents SET title = COALESCE((SELECT r.title FROM document_revisions r
			WHERE r.id = documents.current_revision_id), 'Untitled') WHERE title = ''`,
		`DELETE FROM documents_fts`,
		`INSERT INTO documents_fts(document_id, collection_id, title, body)
			SELECT d.id, d.collection_id, r.title, r.body
			FROM documents d JOIN document_revisions r ON r.id = d.current_revision_id
			WHERE d.deleted_at IS NULL`,
	} {
		if err := s.execLocked(statement); err != nil {
			return fmt.Errorf("finalize restored documents: %w", err)
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// nullableText marks a value the SQL wraps in NULLIF(?, ”). The adapter binds
// text only, so emptiness is turned into NULL in SQL rather than in Go; a bare
// ” in a foreign key column would look for a row whose id is the empty string.
func nullableText(value string) string { return value }

func boolText(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func defaultMIME(value string) string {
	if strings.TrimSpace(value) == "" {
		return "text/markdown"
	}
	return value
}

func jsonOrEmptyObject(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}

func propertyOrderJSON(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	encoded := make([]string, 0, len(values))
	for _, value := range values {
		encoded = append(encoded, strconv.Quote(value))
	}
	return "[" + strings.Join(encoded, ",") + "]"
}
