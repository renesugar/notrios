package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxImportLookupItems = 500

func lookupPlaceholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func validateLookupItems(values []string) error {
	if len(values) > maxImportLookupItems {
		return fmt.Errorf("%w: lookup contains %d items; maximum is %d", ErrInvalidInput, len(values), maxImportLookupItems)
	}
	return nil
}

func (s *SQLiteStore) GetDocuments(ctx context.Context, ids []string) (map[string]Document, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string]Document, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	if err := validateLookupItems(ids); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT d.id, d.collection_id, d.title, r.body,
			COALESCE(r.body_mime_type, d.body_mime_type), d.current_revision_id,
			d.created_at, d.updated_at, COALESCE(d.deleted_at, ''), COALESCE(d.notebook_id, '')
		FROM documents d
		JOIN document_revisions r ON r.id = d.current_revision_id
		WHERE d.deleted_at IS NULL AND d.id IN (` + lookupPlaceholders(len(ids)) + `)`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, ids); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
			updatedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 7)))
			doc := Document{
				ID:                columnText(stmt, 0),
				CollectionID:      columnText(stmt, 1),
				Title:             columnText(stmt, 2),
				Body:              columnText(stmt, 3),
				BodyMIMEType:      columnText(stmt, 4),
				CurrentRevisionID: columnText(stmt, 5),
				CreatedAt:         createdAt,
				UpdatedAt:         updatedAt,
				NotebookID:        columnText(stmt, 9),
			}
			doc.URI = DocumentURI(doc.CollectionID, doc.ID)
			result[doc.ID] = doc
		case C.SQLITE_DONE:
			return result, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) GetResources(ctx context.Context, ids []string) (map[string]Resource, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string]Resource, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	if err := validateLookupItems(ids); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT r.id, r.collection_id, COALESCE(r.filename, ''),
			r.mime_type, b.size_bytes, b.sha256, r.created_at
		FROM resources r
		JOIN blobs b ON b.sha256 = r.blob_sha256
		WHERE r.id IN (` + lookupPlaceholders(len(ids)) + `)`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, ids); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			createdAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 6)))
			resource := Resource{
				ID:           columnText(stmt, 0),
				CollectionID: columnText(stmt, 1),
				Filename:     columnText(stmt, 2),
				MIMEType:     columnText(stmt, 3),
				SizeBytes:    columnInt64(stmt, 4),
				SHA256:       columnText(stmt, 5),
				CreatedAt:    createdAt,
			}
			resource.URI = ResourceURI(resource.CollectionID, resource.ID)
			result[resource.ID] = resource
		case C.SQLITE_DONE:
			return result, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) GetDocumentTags(ctx context.Context, ids []string) (map[string][]Tag, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string][]Tag, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	if err := validateLookupItems(ids); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Batch consumers reconcile document membership and do not need the global
	// per-tag note count. Computing that correlated aggregate once per returned
	// relation becomes quadratic on large imports with popular tags.
	stmt, err := s.prepareLocked(`SELECT nt.document_id, t.id, t.name, 0
		FROM note_tags nt
		JOIN tags t ON t.id = nt.tag_id
		WHERE nt.document_id IN (` + lookupPlaceholders(len(ids)) + `)
		ORDER BY nt.document_id, t.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, ids); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			documentID := columnText(stmt, 0)
			result[documentID] = append(result[documentID], Tag{
				ID:        columnText(stmt, 1),
				Name:      columnText(stmt, 2),
				NoteCount: columnInt64(stmt, 3),
			})
		case C.SQLITE_DONE:
			return result, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) FindDocumentsBySourceIDs(ctx context.Context, sourceSystem string, externalIDs []string) (map[string]string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(externalIDs))
	if len(externalIDs) == 0 {
		return result, nil
	}
	if err := validateLookupItems(externalIDs); err != nil {
		return nil, err
	}
	sourceSystem = strings.TrimSpace(strings.ToLower(sourceSystem))
	s.mu.Lock()
	defer s.mu.Unlock()
	values := append([]string{sourceSystem}, externalIDs...)
	stmt, err := s.prepareLocked(`SELECT external_id, document_id
		FROM document_sources
		WHERE source_system = ? AND external_id IN (` + lookupPlaceholders(len(externalIDs)) + `)`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, values); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			result[columnText(stmt, 0)] = columnText(stmt, 1)
		case C.SQLITE_DONE:
			return result, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func normalizeImportScope(sourceSystem, sourceKey, collectionID string) (string, string, string, error) {
	sourceSystem = strings.TrimSpace(strings.ToLower(sourceSystem))
	sourceKey = strings.TrimSpace(sourceKey)
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		collectionID = "default"
	}
	if sourceSystem == "" || sourceKey == "" {
		return "", "", "", fmt.Errorf("%w: source system and source key are required", ErrInvalidInput)
	}
	return sourceSystem, sourceKey, collectionID, nil
}

func (s *SQLiteStore) GetImportCheckpoint(ctx context.Context, sourceSystem, sourceKey, collectionID string) (ImportCheckpoint, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return ImportCheckpoint{}, err
	}
	sourceSystem, sourceKey, collectionID, err := normalizeImportScope(sourceSystem, sourceKey, collectionID)
	if err != nil {
		return ImportCheckpoint{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT source_system, source_key, collection_id,
			inventory_fingerprint, phase, next_index, total_items, processed_items,
			status, report_json, updated_at, COALESCE(completed_at, '')
		FROM import_checkpoints
		WHERE source_system = ? AND source_key = ? AND collection_id = ?`)
	if err != nil {
		return ImportCheckpoint{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{sourceSystem, sourceKey, collectionID}); err != nil {
		return ImportCheckpoint{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return ImportCheckpoint{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return ImportCheckpoint{}, s.stepErrLocked(rc)
	}
	updatedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 10)))
	var completedAt time.Time
	if value := columnText(stmt, 11); value != "" {
		completedAt, _ = time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(value))
	}
	return ImportCheckpoint{
		SourceSystem:         columnText(stmt, 0),
		SourceKey:            columnText(stmt, 1),
		CollectionID:         columnText(stmt, 2),
		InventoryFingerprint: columnText(stmt, 3),
		Phase:                columnText(stmt, 4),
		NextIndex:            int(columnInt64(stmt, 5)),
		TotalItems:           int(columnInt64(stmt, 6)),
		ProcessedItems:       int(columnInt64(stmt, 7)),
		Status:               columnText(stmt, 8),
		ReportJSON:           columnText(stmt, 9),
		UpdatedAt:            updatedAt,
		CompletedAt:          completedAt,
	}, nil
}

func (s *SQLiteStore) PutImportCheckpoint(ctx context.Context, checkpoint ImportCheckpoint) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putImportCheckpointLocked(checkpoint)
}

func (s *SQLiteStore) putImportCheckpointLocked(checkpoint ImportCheckpoint) error {
	sourceSystem, sourceKey, collectionID, err := normalizeImportScope(checkpoint.SourceSystem, checkpoint.SourceKey, checkpoint.CollectionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(checkpoint.InventoryFingerprint) == "" || strings.TrimSpace(checkpoint.Phase) == "" || strings.TrimSpace(checkpoint.Status) == "" {
		return fmt.Errorf("%w: inventory fingerprint, phase, and status are required", ErrInvalidInput)
	}
	reportJSON := strings.TrimSpace(checkpoint.ReportJSON)
	if reportJSON == "" {
		reportJSON = "{}"
	}
	completed := ""
	if checkpoint.Status == "completed" {
		completed = "1"
	}
	return s.execPreparedLocked(`INSERT INTO import_checkpoints(
			source_system, source_key, collection_id, inventory_fingerprint, phase,
			next_index, total_items, processed_items, status, report_json, completed_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CASE WHEN ? = '1' THEN CURRENT_TIMESTAMP ELSE NULL END)
		ON CONFLICT(source_system, source_key, collection_id) DO UPDATE SET
			inventory_fingerprint = excluded.inventory_fingerprint,
			phase = excluded.phase,
			next_index = excluded.next_index,
			total_items = excluded.total_items,
			processed_items = excluded.processed_items,
			status = excluded.status,
			report_json = excluded.report_json,
			updated_at = CURRENT_TIMESTAMP,
			completed_at = excluded.completed_at`,
		sourceSystem, sourceKey, collectionID, checkpoint.InventoryFingerprint,
		checkpoint.Phase, strconv.Itoa(checkpoint.NextIndex), strconv.Itoa(checkpoint.TotalItems),
		strconv.Itoa(checkpoint.ProcessedItems), checkpoint.Status, reportJSON, completed)
}

func (s *SQLiteStore) GetImportItemStates(ctx context.Context, sourceSystem, sourceKey, collectionID string, itemKeys []string) (map[string]ImportItemState, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string]ImportItemState, len(itemKeys))
	if len(itemKeys) == 0 {
		return result, nil
	}
	if err := validateLookupItems(itemKeys); err != nil {
		return nil, err
	}
	sourceSystem, sourceKey, collectionID, err := normalizeImportScope(sourceSystem, sourceKey, collectionID)
	if err != nil {
		return nil, err
	}
	values := append([]string{sourceSystem, sourceKey, collectionID}, itemKeys...)
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT source_system, source_key, collection_id,
			item_key, item_type, fingerprint, COALESCE(target_id, ''), action, processed_at
		FROM import_item_states
		WHERE source_system = ? AND source_key = ? AND collection_id = ?
			AND item_key IN (` + lookupPlaceholders(len(itemKeys)) + `)`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, values); err != nil {
		return nil, err
	}
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			processedAt, _ := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 8)))
			state := ImportItemState{
				SourceSystem: columnText(stmt, 0),
				SourceKey:    columnText(stmt, 1),
				CollectionID: columnText(stmt, 2),
				ItemKey:      columnText(stmt, 3),
				ItemType:     columnText(stmt, 4),
				Fingerprint:  columnText(stmt, 5),
				TargetID:     columnText(stmt, 6),
				Action:       columnText(stmt, 7),
				ProcessedAt:  processedAt,
			}
			result[state.ItemKey] = state
		case C.SQLITE_DONE:
			return result, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) PutImportItemStates(ctx context.Context, states []ImportItemState) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(states) == 0 {
		return nil
	}
	if len(states) > maxImportLookupItems {
		return fmt.Errorf("%w: import state batch contains %d items; maximum is %d", ErrInvalidInput, len(states), maxImportLookupItems)
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
	for _, state := range states {
		if err := s.putImportItemStateLocked(state); err != nil {
			return err
		}
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SQLiteStore) putImportItemStateLocked(state ImportItemState) error {
	sourceSystem, sourceKey, collectionID, err := normalizeImportScope(state.SourceSystem, state.SourceKey, state.CollectionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(state.ItemKey) == "" || strings.TrimSpace(state.ItemType) == "" || strings.TrimSpace(state.Fingerprint) == "" || strings.TrimSpace(state.Action) == "" {
		return fmt.Errorf("%w: import item key, type, fingerprint, and action are required", ErrInvalidInput)
	}
	return s.execPreparedLocked(`INSERT INTO import_item_states(
			source_system, source_key, collection_id, item_key, item_type,
			fingerprint, target_id, action)
		VALUES(?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
		ON CONFLICT(source_system, source_key, collection_id, item_key) DO UPDATE SET
			item_type = excluded.item_type,
			fingerprint = excluded.fingerprint,
			target_id = excluded.target_id,
			action = excluded.action,
			processed_at = CURRENT_TIMESTAMP`,
		sourceSystem, sourceKey, collectionID, state.ItemKey, state.ItemType,
		state.Fingerprint, state.TargetID, state.Action)
}

func (s *SQLiteStore) PutSourceBundleItem(ctx context.Context, req PutSourceBundleItemRequest) (SourceBundleItem, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SourceBundleItem{}, err
	}
	sourceSystem, sourceKey, collectionID, err := normalizeImportScope(req.SourceSystem, req.SourceKey, req.CollectionID)
	if err != nil {
		return SourceBundleItem{}, err
	}
	if strings.TrimSpace(req.ItemKey) == "" || strings.TrimSpace(req.RelativePath) == "" || req.Content == nil {
		return SourceBundleItem{}, fmt.Errorf("%w: item key, relative path, and content are required", ErrInvalidInput)
	}
	bundleRoot := filepath.Join(s.assetRoot, "source-bundles")
	if err := os.MkdirAll(bundleRoot, 0o755); err != nil {
		return SourceBundleItem{}, err
	}
	tmp, err := os.CreateTemp(bundleRoot, "incoming-*")
	if err != nil {
		return SourceBundleItem{}, err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	hash := sha256.New()
	written, copyErr := copyWithContext(ctx, io.MultiWriter(tmp, hash), req.Content)
	closeErr := tmp.Close()
	if copyErr != nil {
		cleanup()
		return SourceBundleItem{}, copyErr
	}
	if closeErr != nil {
		cleanup()
		return SourceBundleItem{}, closeErr
	}
	shaHex := hex.EncodeToString(hash.Sum(nil))
	relativeStoragePath := filepath.Join("source-bundles", "sha256", shaHex[0:2], shaHex[2:4], shaHex)
	finalPath := filepath.Join(s.assetRoot, relativeStoragePath)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		cleanup()
		return SourceBundleItem{}, err
	}
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		if err := os.Rename(tmpName, finalPath); err != nil {
			cleanup()
			return SourceBundleItem{}, err
		}
	} else if err != nil {
		cleanup()
		return SourceBundleItem{}, err
	} else {
		cleanup()
	}
	propertyJSON, err := json.Marshal(req.PropertyOrder)
	if err != nil {
		return SourceBundleItem{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execPreparedLocked(`INSERT INTO source_bundle_items(
			source_system, source_key, collection_id, item_key, item_type, external_id,
			relative_path, sha256, size_bytes, storage_path, property_order_json)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_system, source_key, collection_id, item_key) DO UPDATE SET
			item_type = excluded.item_type,
			external_id = excluded.external_id,
			relative_path = excluded.relative_path,
			sha256 = excluded.sha256,
			size_bytes = excluded.size_bytes,
			storage_path = excluded.storage_path,
			property_order_json = excluded.property_order_json,
			updated_at = CURRENT_TIMESTAMP`,
		sourceSystem, sourceKey, collectionID, req.ItemKey, req.ItemType, req.ExternalID,
		filepath.ToSlash(req.RelativePath), shaHex, strconv.FormatInt(written, 10),
		filepath.ToSlash(relativeStoragePath), string(propertyJSON)); err != nil {
		return SourceBundleItem{}, err
	}
	return SourceBundleItem{
		SourceSystem:  sourceSystem,
		SourceKey:     sourceKey,
		CollectionID:  collectionID,
		ItemKey:       req.ItemKey,
		ItemType:      req.ItemType,
		ExternalID:    req.ExternalID,
		RelativePath:  filepath.ToSlash(req.RelativePath),
		SHA256:        shaHex,
		SizeBytes:     written,
		StoragePath:   filepath.ToSlash(relativeStoragePath),
		PropertyOrder: append([]string(nil), req.PropertyOrder...),
	}, nil
}

func (s *SQLiteStore) GetSourceBundleItem(ctx context.Context, sourceSystem, sourceKey, collectionID, itemKey string) (SourceBundleItem, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return SourceBundleItem{}, err
	}
	sourceSystem, sourceKey, collectionID, err := normalizeImportScope(sourceSystem, sourceKey, collectionID)
	if err != nil {
		return SourceBundleItem{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT source_system, source_key, collection_id,
			item_key, item_type, external_id, relative_path, sha256, size_bytes,
			storage_path, property_order_json
		FROM source_bundle_items
		WHERE source_system = ? AND source_key = ? AND collection_id = ? AND item_key = ?`)
	if err != nil {
		return SourceBundleItem{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{sourceSystem, sourceKey, collectionID, strings.TrimSpace(itemKey)}); err != nil {
		return SourceBundleItem{}, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return SourceBundleItem{}, ErrNotFound
	}
	if rc != C.SQLITE_ROW {
		return SourceBundleItem{}, s.stepErrLocked(rc)
	}
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
	}
	if err := json.Unmarshal([]byte(columnText(stmt, 10)), &item.PropertyOrder); err != nil {
		return SourceBundleItem{}, fmt.Errorf("decode source bundle property order: %w", err)
	}
	return item, nil
}

func (s *SQLiteStore) OpenSourceBundleItem(ctx context.Context, sourceSystem, sourceKey, collectionID, itemKey string) (SourceBundleItem, io.ReadCloser, error) {
	item, err := s.GetSourceBundleItem(ctx, sourceSystem, sourceKey, collectionID, itemKey)
	if err != nil {
		return SourceBundleItem{}, nil, err
	}
	path, ok := safeAssetPath(s.assetRoot, item.StoragePath)
	if !ok {
		return SourceBundleItem{}, nil, fmt.Errorf("%w: unsafe source bundle path", ErrInvalidInput)
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SourceBundleItem{}, nil, ErrNotFound
		}
		return SourceBundleItem{}, nil, err
	}
	return item, file, nil
}
