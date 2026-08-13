package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// exportReadBatch bounds every identity-scoped export query. It matches the
// selection planner so an archive writer and a dry-run plan traverse the same
// canonical state in the same bounded steps.
const exportReadBatch = selectionReadBatch

// exportBundleBatch is smaller because each source-bundle key binds four
// parameters instead of one.
const exportBundleBatch = 100

var _ ExportReader = (*SQLiteStore)(nil)

// WithReadSnapshot wraps fn in one deferred SQLite read transaction so a long
// export sees a single consistent canonical state. Callers must own the store
// handle exclusively: the transaction is per-connection, so a concurrent
// writer using the same handle would join it.
func (s *SQLiteStore) WithReadSnapshot(ctx context.Context, fn func() error) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	err := s.execLocked("BEGIN DEFERRED")
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("begin read snapshot: %w", err)
	}
	fnErr := fn()
	s.mu.Lock()
	// A read-only transaction has nothing to write; COMMIT simply releases the
	// snapshot. ROLLBACK would be equivalent here but COMMIT keeps the intent
	// obvious if a future caller reads through a shared handle.
	endErr := s.execLocked("COMMIT")
	s.mu.Unlock()
	if fnErr != nil {
		return fnErr
	}
	if endErr != nil {
		return fmt.Errorf("end read snapshot: %w", endErr)
	}
	return nil
}

// ResolveSelection runs the shared P1 planner and additionally returns the
// complete selected identity sets. The returned plan is byte-identical to the
// one PlanSelection produces for the same request, so an archive manifest can
// bind the same manifest digest a dry run reported.
func (s *SQLiteStore) ResolveSelection(ctx context.Context, req SelectionPlanRequest) (SelectionResolution, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SelectionResolution{}, err
	}
	norm, err := normalizeSelectionPlanRequest(req)
	if err != nil {
		return SelectionResolution{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, resolution, err := s.planSelectionLocked(ctx, norm)
	if err != nil {
		return SelectionResolution{}, err
	}
	resolution.Plan = plan
	return resolution, nil
}

func (s *SQLiteStore) ExportCollections(ctx context.Context, ids []string) ([]ExportCollection, error) {
	ctx = contextOrBackground(ctx)
	collections := []ExportCollection{}
	err := s.forEachIDBatch(ctx, ids, exportReadBatch, func(batch []string) error {
		sql := `SELECT id, name, COALESCE(description, ''), created_at FROM collections`
		if batch != nil {
			sql += ` WHERE id IN (` + lookupPlaceholders(len(batch)) + `)`
		}
		sql += ` ORDER BY id`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			collections = append(collections, ExportCollection{
				ID:          columnText(stmt, 0),
				Name:        columnText(stmt, 1),
				Description: columnText(stmt, 2),
				// Collection capabilities are a fixed property of this build;
				// the schema has no per-collection capability column yet.
				Capabilities: []string{"documents", "graph", "links", "resources", "search"},
				CreatedAt:    parseExportTime(columnText(stmt, 3)),
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(collections, func(i, j int) bool { return collections[i].ID < collections[j].ID })
	return collections, nil
}

func (s *SQLiteStore) ExportTags(ctx context.Context, ids []string) ([]ExportTag, error) {
	ctx = contextOrBackground(ctx)
	tags := []ExportTag{}
	err := s.forEachIDBatch(ctx, ids, exportReadBatch, func(batch []string) error {
		sql := `SELECT id, name, created_at FROM tags`
		if batch != nil {
			sql += ` WHERE id IN (` + lookupPlaceholders(len(batch)) + `)`
		}
		sql += ` ORDER BY id`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			tags = append(tags, ExportTag{ID: columnText(stmt, 0), Name: columnText(stmt, 1), CreatedAt: parseExportTime(columnText(stmt, 2))})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].ID < tags[j].ID })
	return tags, nil
}

func (s *SQLiteStore) ExportDocuments(ctx context.Context, ids []string) ([]ExportDocument, error) {
	ctx = contextOrBackground(ctx)
	documents := make([]ExportDocument, 0, len(ids))
	err := s.forEachIDBatch(ctx, ids, exportReadBatch, func(batch []string) error {
		sql := `SELECT id, collection_id, COALESCE(notebook_id, ''), COALESCE(current_revision_id, ''),
				COALESCE(deleted_at, ''), created_at, updated_at
			FROM documents`
		if batch != nil {
			sql += ` WHERE id IN (` + lookupPlaceholders(len(batch)) + `)`
		}
		sql += ` ORDER BY id`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			documents = append(documents, ExportDocument{
				ID:                columnText(stmt, 0),
				CollectionID:      columnText(stmt, 1),
				NotebookID:        columnText(stmt, 2),
				CurrentRevisionID: columnText(stmt, 3),
				DeletedAt:         parseExportTime(columnText(stmt, 4)),
				CreatedAt:         parseExportTime(columnText(stmt, 5)),
				UpdatedAt:         parseExportTime(columnText(stmt, 6)),
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(documents, func(i, j int) bool { return documents[i].ID < documents[j].ID })
	return documents, nil
}

// ExportRevisions streams revisions for the given documents. currentOnly keeps
// only the revision each document currently points at, which a scoped
// publication projection needs; a full or subset archive keeps complete
// history.
func (s *SQLiteStore) ExportRevisions(ctx context.Context, documentIDs []string, currentOnly bool, visit func(ExportRevision) error) error {
	ctx = contextOrBackground(ctx)
	return s.forEachIDBatch(ctx, documentIDs, exportReadBatch, func(batch []string) error {
		if batch == nil {
			return fmt.Errorf("%w: revision export requires document IDs", ErrInvalidInput)
		}
		sql := `SELECT r.id, r.document_id, r.title, r.body, r.body_mime_type,
				r.metadata_json, COALESCE(r.message, ''), r.created_at
			FROM document_revisions r`
		if currentOnly {
			sql += ` JOIN documents d ON d.current_revision_id = r.id`
		}
		sql += ` WHERE r.document_id IN (` + lookupPlaceholders(len(batch)) + `) ORDER BY r.document_id, r.created_at, r.id`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			return visit(ExportRevision{
				ID:           columnText(stmt, 0),
				DocumentID:   columnText(stmt, 1),
				Title:        columnText(stmt, 2),
				Body:         columnText(stmt, 3),
				BodyMIMEType: columnText(stmt, 4),
				MetadataJSON: columnText(stmt, 5),
				Message:      columnText(stmt, 6),
				CreatedAt:    parseExportTime(columnText(stmt, 7)),
			})
		})
	})
}

func (s *SQLiteStore) ExportDocumentTags(ctx context.Context, documentIDs []string) ([]ExportDocumentTag, error) {
	ctx = contextOrBackground(ctx)
	memberships := []ExportDocumentTag{}
	err := s.forEachIDBatch(ctx, documentIDs, exportReadBatch, func(batch []string) error {
		if batch == nil {
			return fmt.Errorf("%w: tag membership export requires document IDs", ErrInvalidInput)
		}
		sql := `SELECT document_id, tag_id FROM note_tags
			WHERE document_id IN (` + lookupPlaceholders(len(batch)) + `)
			ORDER BY document_id, tag_id`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			memberships = append(memberships, ExportDocumentTag{DocumentID: columnText(stmt, 0), TagID: columnText(stmt, 1)})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return memberships, nil
}

func (s *SQLiteStore) ExportResources(ctx context.Context, ids []string) ([]ExportResource, error) {
	ctx = contextOrBackground(ctx)
	resources := make([]ExportResource, 0, len(ids))
	err := s.forEachIDBatch(ctx, ids, exportReadBatch, func(batch []string) error {
		sql := `SELECT r.id, r.collection_id, r.blob_sha256, b.size_bytes, COALESCE(r.filename, ''),
				r.mime_type, r.metadata_json, COALESCE(r.unreferenced_at, ''),
				COALESCE(r.unreferenced_reason, ''), r.created_at
			FROM resources r JOIN blobs b ON b.sha256 = r.blob_sha256`
		if batch != nil {
			sql += ` WHERE r.id IN (` + lookupPlaceholders(len(batch)) + `)`
		}
		sql += ` ORDER BY r.id`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			resources = append(resources, ExportResource{
				ID:                 columnText(stmt, 0),
				CollectionID:       columnText(stmt, 1),
				BlobSHA256:         columnText(stmt, 2),
				BlobSizeBytes:      columnInt64(stmt, 3),
				Filename:           columnText(stmt, 4),
				MIMEType:           columnText(stmt, 5),
				MetadataJSON:       columnText(stmt, 6),
				UnreferencedAt:     parseExportTime(columnText(stmt, 7)),
				UnreferencedReason: columnText(stmt, 8),
				CreatedAt:          parseExportTime(columnText(stmt, 9)),
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	return resources, nil
}

func (s *SQLiteStore) ExportDocumentResources(ctx context.Context, documentIDs []string) ([]ExportDocumentResource, error) {
	ctx = contextOrBackground(ctx)
	references := []ExportDocumentResource{}
	err := s.forEachIDBatch(ctx, documentIDs, exportReadBatch, func(batch []string) error {
		if batch == nil {
			return fmt.Errorf("%w: resource relation export requires document IDs", ErrInvalidInput)
		}
		sql := `SELECT document_id, resource_id, relation_type, ordinal, anchor_json
			FROM document_resource_refs
			WHERE document_id IN (` + lookupPlaceholders(len(batch)) + `)
			ORDER BY document_id, resource_id, relation_type, ordinal`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			references = append(references, ExportDocumentResource{
				DocumentID:   columnText(stmt, 0),
				ResourceID:   columnText(stmt, 1),
				RelationType: columnText(stmt, 2),
				Ordinal:      int(columnInt64(stmt, 3)),
				AnchorJSON:   columnText(stmt, 4),
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return references, nil
}

func (s *SQLiteStore) ExportLinks(ctx context.Context, documentIDs []string, visit func(DocumentLink) error) error {
	ctx = contextOrBackground(ctx)
	return s.forEachIDBatch(ctx, documentIDs, exportReadBatch, func(batch []string) error {
		if batch == nil {
			return fmt.Errorf("%w: link export requires document IDs", ErrInvalidInput)
		}
		sql := `SELECT id, source_document_id, COALESCE(target_document_id, ''), COALESCE(target_resource_id, ''),
				COALESCE(target_uri, ''), relation_type, source_format, raw_target,
				COALESCE(display_text, ''), COALESCE(anchor_type, ''), COALESCE(anchor_value, ''),
				COALESCE(context, ''), COALESCE(source_start_byte, 0), COALESCE(source_end_byte, 0),
				COALESCE(source_line, 0), COALESCE(source_column, 0), resolution_status
			FROM document_links
			WHERE source_document_id IN (` + lookupPlaceholders(len(batch)) + `)
			ORDER BY source_document_id, id`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			return visit(DocumentLink{
				ID:               columnInt64(stmt, 0),
				SourceDocumentID: columnText(stmt, 1),
				TargetDocumentID: columnText(stmt, 2),
				TargetResourceID: columnText(stmt, 3),
				TargetURI:        columnText(stmt, 4),
				RelationType:     columnText(stmt, 5),
				SourceFormat:     columnText(stmt, 6),
				RawTarget:        columnText(stmt, 7),
				DisplayText:      columnText(stmt, 8),
				AnchorType:       columnText(stmt, 9),
				AnchorValue:      columnText(stmt, 10),
				Context:          columnText(stmt, 11),
				SourceStartByte:  int(columnInt64(stmt, 12)),
				SourceEndByte:    int(columnInt64(stmt, 13)),
				SourceLine:       int(columnInt64(stmt, 14)),
				SourceColumn:     int(columnInt64(stmt, 15)),
				ResolutionStatus: columnText(stmt, 16),
			})
		})
	})
}

func (s *SQLiteStore) ExportProvenance(ctx context.Context, documentIDs []string) ([]DocumentSource, error) {
	ctx = contextOrBackground(ctx)
	sources := []DocumentSource{}
	err := s.forEachIDBatch(ctx, documentIDs, exportReadBatch, func(batch []string) error {
		if batch == nil {
			return fmt.Errorf("%w: provenance export requires document IDs", ErrInvalidInput)
		}
		sql := `SELECT document_id, source_system, external_id, author, author_id, thread_id,
				reply_to, source_url, published_at, COALESCE(published_ts, 0),
				metadata_json, created_at, updated_at
			FROM document_sources
			WHERE document_id IN (` + lookupPlaceholders(len(batch)) + `)
			ORDER BY document_id`
		return s.scanRowsLocked(ctx, sql, batch, func(stmt *C.sqlite3_stmt) error {
			sources = append(sources, DocumentSource{
				DocumentID:   columnText(stmt, 0),
				SourceSystem: columnText(stmt, 1),
				ExternalID:   columnText(stmt, 2),
				Author:       columnText(stmt, 3),
				AuthorID:     columnText(stmt, 4),
				ThreadID:     columnText(stmt, 5),
				ReplyTo:      columnText(stmt, 6),
				SourceURL:    columnText(stmt, 7),
				PublishedAt:  columnText(stmt, 8),
				PublishedTS:  columnInt64(stmt, 9),
				MetadataJSON: columnText(stmt, 10),
				CreatedAt:    parseExportTime(columnText(stmt, 11)),
				UpdatedAt:    parseExportTime(columnText(stmt, 12)),
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return sources, nil
}

func (s *SQLiteStore) ExportSourceBundles(ctx context.Context, keys []SourceBundleKey) ([]SourceBundleItem, error) {
	ctx = contextOrBackground(ctx)
	items := make([]SourceBundleItem, 0, len(keys))
	for start := 0; start < len(keys); start += exportBundleBatch {
		end := min(start+exportBundleBatch, len(keys))
		batch := keys[start:end]
		predicates := make([]string, 0, len(batch))
		values := make([]string, 0, len(batch)*4)
		for _, key := range batch {
			predicates = append(predicates, "(source_system = ? AND source_key = ? AND collection_id = ? AND item_key = ?)")
			values = append(values, key.SourceSystem, key.SourceKey, key.CollectionID, key.ItemKey)
		}
		sql := `SELECT source_system, source_key, collection_id, item_key, item_type, external_id,
				relative_path, sha256, size_bytes, storage_path, property_order_json, updated_at
			FROM source_bundle_items WHERE ` + strings.Join(predicates, " OR ") + `
			ORDER BY source_system, source_key, collection_id, item_key`
		s.mu.Lock()
		err := s.scanRowsLocked(ctx, sql, values, func(stmt *C.sqlite3_stmt) error {
			item := SourceBundleItem{
				SourceSystem: columnText(stmt, 0),
				SourceKey:    columnText(stmt, 1),
				CollectionID: columnText(stmt, 2),
				ItemKey:      columnText(stmt, 3),
				ItemType:     columnText(stmt, 4),
				ExternalID:   columnText(stmt, 5),
				RelativePath: columnText(stmt, 6),
				SHA256:       columnText(stmt, 7),
				SizeBytes:    columnInt64(stmt, 8),
				StoragePath:  columnText(stmt, 9),
				UpdatedAt:    parseExportTime(columnText(stmt, 11)),
			}
			if err := json.Unmarshal([]byte(columnText(stmt, 10)), &item.PropertyOrder); err != nil {
				return fmt.Errorf("decode source bundle property order: %w", err)
			}
			items = append(items, item)
			return nil
		})
		s.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(items, func(i, j int) bool { return sourceBundleSortKey(items[i]) < sourceBundleSortKey(items[j]) })
	return items, nil
}

func sourceBundleSortKey(item SourceBundleItem) string {
	return strings.Join([]string{item.SourceSystem, item.SourceKey, item.CollectionID, item.ItemKey}, "\x00")
}

// OpenBlobContent streams one physical blob by its content hash. Logical
// resources that share a blob are exported once.
func (s *SQLiteStore) OpenBlobContent(ctx context.Context, sha256Hex string) (io.ReadCloser, int64, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.mu.Lock()
	var storagePath string
	var size int64
	err := s.scanRowsLocked(ctx, `SELECT storage_path, size_bytes FROM blobs WHERE sha256 = ?`, []string{sha256Hex}, func(stmt *C.sqlite3_stmt) error {
		storagePath = columnText(stmt, 0)
		size = columnInt64(stmt, 1)
		return nil
	})
	s.mu.Unlock()
	if err != nil {
		return nil, 0, err
	}
	if storagePath == "" {
		// The object is known but its bytes were never materialized. An archive
		// that silently omitted it would not be the full snapshot it claims to
		// be, so the export refuses and says which object is missing; pinning
		// and materializing it first is the fix.
		return nil, 0, fmt.Errorf("%w: object %s must be materialized before it can be exported", ErrResourceUnavailable, sha256Hex)
	}
	path, ok := safeAssetPath(s.assetRoot, storagePath)
	if !ok {
		return nil, 0, fmt.Errorf("%w: unsafe blob storage path", ErrInvalidInput)
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, err
	}
	return file, size, nil
}

// OpenSourceBundleContent streams exact preserved source bytes for one bundle
// item that ExportSourceBundles already resolved.
func (s *SQLiteStore) OpenSourceBundleContent(ctx context.Context, item SourceBundleItem) (io.ReadCloser, error) {
	if err := contextOrBackground(ctx).Err(); err != nil {
		return nil, err
	}
	path, ok := safeAssetPath(s.assetRoot, item.StoragePath)
	if !ok {
		return nil, fmt.Errorf("%w: unsafe source bundle path", ErrInvalidInput)
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return file, nil
}

// forEachIDBatch applies fn to bounded slices of ids. A nil ids slice means
// "every row"; fn then receives a single nil batch and must build an
// unfiltered query.
func (s *SQLiteStore) forEachIDBatch(ctx context.Context, ids []string, batchSize int, fn func(batch []string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ids == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return fn(nil)
	}
	for start := 0; start < len(ids); start += batchSize {
		end := min(start+batchSize, len(ids))
		s.mu.Lock()
		err := fn(ids[start:end])
		s.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) scanRowsLocked(ctx context.Context, sql string, values []string, row func(stmt *C.sqlite3_stmt) error) error {
	stmt, err := s.prepareLocked(sql)
	if err != nil {
		return err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, values); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			if err := row(stmt); err != nil {
				return err
			}
		case C.SQLITE_DONE:
			return nil
		default:
			return s.stepErrLocked(rc)
		}
	}
}

func parseExportTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
