package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/renesugar/notrios/internal/syncmerge"
	"github.com/renesugar/notrios/internal/syncstate"
)

const RecoveredNotebookID = syncmerge.RecoveredNotebookID

// ensureSyncMetadataBaseline captures an upgrade baseline only once. Explicit
// first enrollment refreshes it so writes made before enrollment belong to the
// sequence-zero snapshot rather than a synthetic operation history.
func (s *SQLiteStore) ensureSyncMetadataBaseline(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	count, err := s.countLocked(`SELECT COUNT(*) FROM sync_metadata_baseline_records`)
	if err != nil || count != 0 {
		return err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.captureSyncMetadataBaselineLocked(true); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *SQLiteStore) captureSyncMetadataBaselineLocked(includeExistingOperationFloors bool) error {
	for _, table := range []string{"sync_metadata_baseline_memberships", "sync_metadata_baseline_records", "sync_metadata_baseline_floors", "sync_metadata_baseline_deaths"} {
		if err := s.execLocked("DELETE FROM " + table); err != nil {
			return err
		}
	}
	statements := []string{
		`INSERT INTO sync_metadata_baseline_records(record_type, record_id, payload_json, active)
		 SELECT 'collection', id, json_object('name', name, 'description', COALESCE(description, '')), 1 FROM collections`,
		`INSERT INTO sync_metadata_baseline_records(record_type, record_id, payload_json, active)
		 SELECT 'document', id, json_object('collection_id', collection_id, 'notebook_id', COALESCE(notebook_id, ''), 'title', title, 'body_mime_type', body_mime_type), CASE WHEN deleted_at IS NULL THEN 1 ELSE 0 END FROM documents`,
		`INSERT INTO sync_metadata_baseline_records(record_type, record_id, payload_json, active)
		 SELECT 'notebook', id, json_object('parent_id', COALESCE(parent_id, ''), 'name', name, 'icon_emoji', icon_emoji, 'position', position), 1 FROM notebooks`,
		`INSERT INTO sync_metadata_baseline_records(record_type, record_id, payload_json, active)
		 SELECT 'tag', id, json_object('name', name), 1 FROM tags`,
		`INSERT INTO sync_metadata_baseline_records(record_type, record_id, payload_json, active)
		 SELECT 'search_notebook', id, json_object('name', name, 'icon_emoji', icon_emoji, 'query', query), 1 FROM search_notebooks`,
		`INSERT INTO sync_metadata_baseline_records(record_type, record_id, payload_json, active)
		 SELECT 'document_source', document_id, json_object('source_system', source_system, 'external_id', external_id, 'author', author, 'author_id', author_id, 'thread_id', thread_id, 'reply_to', reply_to, 'source_url', source_url, 'published_at', published_at, 'metadata_json', metadata_json), 1 FROM document_sources`,
		`INSERT INTO sync_metadata_baseline_memberships(element_id, document_id, tag_id, present)
		 SELECT document_id || ':' || tag_id, document_id, tag_id, 1 FROM note_tags`,
	}
	for _, statement := range statements {
		if err := s.execLocked(statement); err != nil {
			return err
		}
	}
	if includeExistingOperationFloors {
		if err := s.execLocked(`INSERT INTO sync_metadata_baseline_floors(replica_id, sequence)
			SELECT replica_id, MAX(sequence) FROM sync_operations GROUP BY replica_id`); err != nil {
			return err
		}
	}
	if err := s.execLocked(`INSERT INTO sync_metadata_baseline_deaths(
		document_id, signer_replica_id, signer_sequence, signature, hlc_wall_ms, hlc_logical)
		SELECT document_id, signer_replica_id, signer_sequence, signature, hlc_wall_ms, hlc_logical
		FROM sync_death_certificates`); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) reconcileSyncMetadataLocked() error {
	baseline, err := s.syncMetadataBaselineLocked()
	if err != nil {
		return err
	}
	baselineMemberships, err := s.syncMetadataBaselineMembershipsLocked()
	if err != nil {
		return err
	}
	baselineDeaths, err := s.syncMetadataBaselineDeathsLocked()
	if err != nil {
		return err
	}
	operations, err := s.syncMetadataOperationsLocked()
	if err != nil {
		return err
	}
	projection, err := syncmerge.ConvergeWithDeaths(baseline, baselineMemberships, baselineDeaths, operations)
	if err != nil {
		return fmt.Errorf("sync metadata convergence: %w", err)
	}
	if err := s.persistSyncProjectionLocked(projection); err != nil {
		return err
	}
	return s.applySyncProjectionLocked(projection)
}

func (s *SQLiteStore) syncMetadataBaselineDeathsLocked() ([]syncmerge.DeathCertificate, error) {
	stmt, err := s.prepareLocked(`SELECT document_id, signer_replica_id, signer_sequence,
		signature, hlc_wall_ms, hlc_logical FROM sync_metadata_baseline_deaths ORDER BY document_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var deaths []syncmerge.DeathCertificate
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			deaths = append(deaths, syncmerge.DeathCertificate{
				DocumentID: columnText(stmt, 0), Signer: columnText(stmt, 1), Signature: columnText(stmt, 3),
				Order: syncmerge.Order{ReplicaID: columnText(stmt, 1), Sequence: columnInt64(stmt, 2), WallMS: columnInt64(stmt, 4), Logical: columnInt64(stmt, 5)},
			})
		case C.SQLITE_DONE:
			return deaths, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) syncMetadataBaselineMembershipsLocked() ([]syncmerge.Membership, error) {
	stmt, err := s.prepareLocked(`SELECT element_id, document_id, tag_id, present FROM sync_metadata_baseline_memberships ORDER BY element_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var memberships []syncmerge.Membership
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			memberships = append(memberships, syncmerge.Membership{ID: columnText(stmt, 0), Document: columnText(stmt, 1), Tag: columnText(stmt, 2), Present: columnInt64(stmt, 3) == 1})
		case C.SQLITE_DONE:
			return memberships, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) syncMetadataBaselineLocked() ([]*syncmerge.Record, error) {
	stmt, err := s.prepareLocked(`SELECT record_type, record_id, payload_json, active FROM sync_metadata_baseline_records ORDER BY record_type, record_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var records []*syncmerge.Record
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(columnText(stmt, 2)), &fields); err != nil {
				return nil, err
			}
			record := &syncmerge.Record{Type: columnText(stmt, 0), ID: columnText(stmt, 1), Active: columnInt64(stmt, 3) == 1, Baseline: true, Fields: map[string]syncmerge.Register{}}
			for name, value := range fields {
				record.Fields[name] = syncmerge.Register{Value: value}
			}
			records = append(records, record)
		case C.SQLITE_DONE:
			return records, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) syncMetadataOperationsLocked() ([]syncstate.Operation, error) {
	stmt, err := s.prepareLocked(`SELECT o.replica_id, o.sequence, o.operation_id, o.kind, o.record_type, o.record_id,
		o.payload_json, o.hlc_wall_ms, o.hlc_logical, o.created_at
		FROM sync_operations o LEFT JOIN sync_metadata_baseline_floors f ON f.replica_id = o.replica_id
		WHERE o.sequence > COALESCE(f.sequence, 0)
		ORDER BY o.hlc_wall_ms, o.hlc_logical, o.replica_id, o.sequence`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var operations []syncstate.Operation
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			createdAt, err := time.Parse(time.RFC3339Nano, sqliteTimeToRFC3339(columnText(stmt, 9)))
			if err != nil {
				return nil, err
			}
			operations = append(operations, syncstate.Operation{
				ReplicaID: columnText(stmt, 0), Sequence: columnInt64(stmt, 1), OperationID: columnText(stmt, 2),
				Kind: columnText(stmt, 3), RecordType: columnText(stmt, 4), RecordID: columnText(stmt, 5), Payload: json.RawMessage(columnText(stmt, 6)),
				HLC: syncstate.HLC{WallMS: columnInt64(stmt, 7), Logical: columnInt64(stmt, 8)}, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
			})
		case C.SQLITE_DONE:
			return operations, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) persistSyncProjectionLocked(projection syncmerge.Projection) error {
	for _, table := range []string{"sync_field_registers", "sync_lifecycle_registers", "sync_membership_registers", "sync_death_certificates", "sync_repair_events"} {
		if err := s.execLocked("DELETE FROM " + table); err != nil {
			return err
		}
	}
	for _, record := range projection.Records {
		order := record.LifecycleOrder
		if err := s.execPreparedLocked(`INSERT INTO sync_lifecycle_registers(record_type, record_id, active, hlc_wall_ms, hlc_logical, replica_id, sequence) VALUES(?, ?, ?, ?, ?, ?, ?)`,
			record.Type, record.ID, boolNumber(record.Active), orderNumber(order.WallMS), orderNumber(order.Logical), order.ReplicaID, orderNumber(order.Sequence)); err != nil {
			return err
		}
		for field, register := range record.Fields {
			if err := s.execPreparedLocked(`INSERT INTO sync_field_registers(record_type, record_id, field_name, value_json, hlc_wall_ms, hlc_logical, replica_id, sequence) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
				record.Type, record.ID, field, string(register.Value), orderNumber(register.Order.WallMS), orderNumber(register.Order.Logical), register.Order.ReplicaID, orderNumber(register.Order.Sequence)); err != nil {
				return err
			}
		}
	}
	for _, membership := range projection.Memberships {
		if err := s.execPreparedLocked(`INSERT INTO sync_membership_registers(element_id, document_id, tag_id, present, hlc_wall_ms, hlc_logical, replica_id, sequence) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			membership.ID, membership.Document, membership.Tag, boolNumber(membership.Present), orderNumber(membership.Order.WallMS), orderNumber(membership.Order.Logical), membership.Order.ReplicaID, orderNumber(membership.Order.Sequence)); err != nil {
			return err
		}
	}
	for _, death := range projection.Deaths {
		if err := s.execPreparedLocked(`INSERT INTO sync_death_certificates(document_id, signer_replica_id, signer_sequence, signature, hlc_wall_ms, hlc_logical) VALUES(?, ?, ?, ?, ?, ?)`,
			death.DocumentID, death.Signer, orderNumber(death.Order.Sequence), death.Signature, orderNumber(death.Order.WallMS), orderNumber(death.Order.Logical)); err != nil {
			return err
		}
	}
	for _, repair := range projection.Repairs {
		if err := s.execPreparedLocked(`INSERT INTO sync_repair_events(id, event_type, subject_id, details_json, hlc_wall_ms, hlc_logical, replica_id, sequence) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			repair.ID, repair.Kind, repair.SubjectID, string(repair.Details), orderNumber(repair.Order.WallMS), orderNumber(repair.Order.Logical), repair.Order.ReplicaID, orderNumber(repair.Order.Sequence)); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) applySyncProjectionLocked(projection syncmerge.Projection) error {
	if err := s.execLocked(`INSERT INTO sync_apply_guard(singleton) VALUES(1)`); err != nil {
		return err
	}
	defer func() { _ = s.execLocked(`DELETE FROM sync_apply_guard`) }()

	// Free case-insensitive unique names before applying the deterministic names.
	for _, statement := range []string{
		`UPDATE notebooks SET name = '__sync__' || id WHERE id NOT IN ('nb_notes','nb_help','nb_reports','nb_recovered')`,
		`UPDATE tags SET name = '__sync__' || id`,
		`UPDATE search_notebooks SET name = '__sync__' || id WHERE builtin = 0`,
	} {
		if err := s.execLocked(statement); err != nil {
			return err
		}
	}

	for _, record := range projection.Records {
		if !record.Active || record.Type != "collection" {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO collections(id, name, description) VALUES(?, ?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name, description=excluded.description`, record.ID, syncmerge.String(record, "name", record.ID), syncmerge.String(record, "description", "")); err != nil {
			return err
		}
	}
	for _, record := range projection.Records {
		if !record.Active || record.Type != "notebook" {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO notebooks(id, parent_id, name, icon_emoji, builtin, position) VALUES(?, NULL, '__sync__' || ?, ?, 0, ?) ON CONFLICT(id) DO UPDATE SET parent_id=NULL, name='__sync__' || excluded.id`, record.ID, record.ID, syncmerge.String(record, "icon_emoji", ""), strconv.FormatInt(syncmerge.Int(record, "position", 0), 10)); err != nil {
			return err
		}
	}
	for _, record := range projection.Records {
		if !record.Active || record.Type != "notebook" {
			continue
		}
		parent := projection.NotebookParent[record.ID]
		if parent == "" {
			if err := s.execPreparedLocked(`UPDATE notebooks SET parent_id=NULL WHERE id=?`, record.ID); err != nil {
				return err
			}
		} else if err := s.execPreparedLocked(`UPDATE notebooks SET parent_id=? WHERE id=?`, parent, record.ID); err != nil {
			return err
		}
	}
	for _, record := range projection.Records {
		if !record.Active || record.Type != "notebook" {
			continue
		}
		if err := s.execPreparedLocked(`UPDATE notebooks SET name=?, icon_emoji=?, position=? WHERE id=?`, projection.NotebookName[record.ID], syncmerge.String(record, "icon_emoji", ""), strconv.FormatInt(syncmerge.Int(record, "position", 0), 10), record.ID); err != nil {
			return fmt.Errorf("name notebook %s as %q: %w", record.ID, projection.NotebookName[record.ID], err)
		}
	}
	for _, record := range projection.Records {
		if !record.Active || record.Type != "tag" {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO tags(id, name) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name`, record.ID, projection.RecordName["tag\x00"+record.ID]); err != nil {
			return err
		}
	}
	for _, record := range projection.Records {
		if !record.Active || record.Type != "search_notebook" {
			continue
		}
		if err := s.execPreparedLocked(`INSERT INTO search_notebooks(id, name, icon_emoji, query, builtin, sort_anchor) VALUES(?, ?, ?, ?, 0, 'normal') ON CONFLICT(id) DO UPDATE SET name=excluded.name, icon_emoji=excluded.icon_emoji, query=excluded.query`, record.ID, projection.RecordName["search_notebook\x00"+record.ID], syncmerge.String(record, "icon_emoji", ""), syncmerge.String(record, "query", "")); err != nil {
			return err
		}
	}
	for _, record := range projection.Records {
		if record.Type != "document" {
			continue
		}
		if _, dead := projection.Deaths[record.ID]; dead {
			record.Active = false
		}
		deletedAt := ""
		if !record.Active {
			deletedAt = time.UnixMilli(max64(1, record.LifecycleOrder.WallMS)).UTC().Format(time.RFC3339Nano)
		}
		if deletedAt == "" {
			if err := s.execPreparedLocked(`INSERT INTO documents(id, collection_id, notebook_id, title, body_mime_type, deleted_at) VALUES(?, ?, NULLIF(?,''), ?, ?, NULL) ON CONFLICT(id) DO UPDATE SET collection_id=excluded.collection_id, notebook_id=excluded.notebook_id, title=excluded.title, body_mime_type=excluded.body_mime_type, deleted_at=NULL, updated_at=CURRENT_TIMESTAMP`, record.ID, projection.DocumentCollection[record.ID], projection.DocumentNotebook[record.ID], syncmerge.String(record, "title", "Untitled"), syncmerge.String(record, "body_mime_type", "text/markdown")); err != nil {
				return err
			}
		} else if err := s.execPreparedLocked(`INSERT INTO documents(id, collection_id, notebook_id, title, body_mime_type, deleted_at) VALUES(?, ?, NULLIF(?,''), ?, ?, ?) ON CONFLICT(id) DO UPDATE SET collection_id=excluded.collection_id, notebook_id=excluded.notebook_id, title=excluded.title, body_mime_type=excluded.body_mime_type, deleted_at=excluded.deleted_at, updated_at=CURRENT_TIMESTAMP`, record.ID, projection.DocumentCollection[record.ID], projection.DocumentNotebook[record.ID], syncmerge.String(record, "title", "Untitled"), syncmerge.String(record, "body_mime_type", "text/markdown"), deletedAt); err != nil {
			return err
		}
	}
	for _, record := range projection.Records {
		if !record.Active || record.Type != "document_source" {
			continue
		}
		publishedAt := syncmerge.String(record, "published_at", "")
		if err := s.execPreparedLocked(`INSERT INTO document_sources(document_id, source_system, external_id, author, author_id, thread_id, reply_to, source_url, published_at, published_ts, metadata_json)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(document_id) DO UPDATE SET source_system=excluded.source_system, external_id=excluded.external_id,
				author=excluded.author, author_id=excluded.author_id, thread_id=excluded.thread_id, reply_to=excluded.reply_to,
				source_url=excluded.source_url, published_at=excluded.published_at, published_ts=excluded.published_ts,
				metadata_json=excluded.metadata_json, updated_at=CURRENT_TIMESTAMP`,
			record.ID, syncmerge.String(record, "source_system", "local"), syncmerge.String(record, "external_id", ""),
			syncmerge.String(record, "author", ""), syncmerge.String(record, "author_id", ""), syncmerge.String(record, "thread_id", ""),
			syncmerge.String(record, "reply_to", ""), syncmerge.String(record, "source_url", ""), publishedAt,
			strconv.FormatInt(parsePublishedTS(publishedAt), 10), syncmerge.String(record, "metadata_json", "{}")); err != nil {
			return err
		}
	}
	if err := s.execLocked(`DELETE FROM note_tags`); err != nil {
		return err
	}
	for _, membership := range projection.Memberships {
		if membership.Present {
			if err := s.execPreparedLocked(`INSERT INTO note_tags(document_id, tag_id) VALUES(?, ?)`, membership.Document, membership.Tag); err != nil {
				return err
			}
		}
	}
	// Deletions occur after references have been repaired. Protected baseline
	// rows are never removed even if a malformed future operation names them.
	for _, record := range projection.Records {
		if record.Active {
			continue
		}
		switch record.Type {
		case "tag":
			if err := s.execPreparedLocked(`DELETE FROM tags WHERE id=?`, record.ID); err != nil {
				return err
			}
		case "search_notebook":
			if err := s.execPreparedLocked(`DELETE FROM search_notebooks WHERE id=? AND builtin=0`, record.ID); err != nil {
				return err
			}
		case "notebook":
			if err := s.execPreparedLocked(`DELETE FROM notebooks WHERE id=? AND builtin=0 AND id<>?`, record.ID, DefaultNotebookID); err != nil {
				return err
			}
		case "collection":
			if record.ID != syncmerge.DefaultCollectionID {
				if err := s.execPreparedLocked(`DELETE FROM collections WHERE id=?`, record.ID); err != nil {
					return err
				}
			}
		case "document_source":
			if err := s.execPreparedLocked(`DELETE FROM document_sources WHERE document_id=?`, record.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func boolNumber(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
func orderNumber(value int64) string { return strconv.FormatInt(value, 10) }
func max64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
