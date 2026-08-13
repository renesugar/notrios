package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/renesugar/notrios/internal/syncassets"
	"github.com/renesugar/notrios/internal/syncmerge"
)

// stagingDirectory holds partially fetched objects. It is deliberately inside
// the asset root but outside the content-addressed tree: a half-downloaded
// object must never be reachable as a blob, and the boundary G8 was given says
// no placeholder bytes in the canonical asset store.
const stagingDirectory = "staging"

// resourceRecord is one resource as an operation describes it.
type resourceRecord struct {
	resourceID     string
	collectionID   string
	blobSHA256     string
	filename       string
	mimeType       string
	metadataJSON   string
	blobLength     int64
	blobMIMEType   string
	chunkCount     int
	manifestSHA256 string
	deleted        bool
	order          syncmerge.Order
	sourceReplica  string
}

type resourceReference struct {
	elementID    string
	documentID   string
	resourceID   string
	relationType string
	ordinal      int64
	anchorJSON   string
	present      bool
	order        syncmerge.Order
}

// reconcileSyncAssetsLocked converges resource metadata and its references. It
// runs inside the admission transaction after bodies, so a note and the
// attachment it names arrive together even when the bytes do not.
func (s *SQLiteStore) reconcileSyncAssetsLocked() error {
	records, references, err := s.foldSyncResourceOperationsLocked()
	if err != nil {
		return err
	}
	if len(records) == 0 && len(references) == 0 {
		return nil
	}
	return s.withSyncApplyGuardLocked(func() error {
		if err := s.applySyncResourceRecordsLocked(records); err != nil {
			return err
		}
		return s.applySyncResourceReferencesLocked(references)
	})
}

// foldSyncResourceOperationsLocked orders every post-boundary resource
// operation the same way G6 orders metadata — by hybrid logical clock, then
// replica, then sequence — and keeps the winner per record and per reference
// element. A resource is an immutable object, so the winner is the whole
// record rather than a per-field register.
func (s *SQLiteStore) foldSyncResourceOperationsLocked() (map[string]*resourceRecord, map[string]*resourceReference, error) {
	stmt, err := s.prepareLocked(`
		SELECT o.kind, o.record_type, o.record_id, o.payload_json,
		       o.hlc_wall_ms, o.hlc_logical, o.replica_id, o.sequence
		  FROM sync_operations o LEFT JOIN sync_metadata_baseline_floors f ON f.replica_id = o.replica_id
		 WHERE o.record_type IN ('resource', 'document_resource')
		   AND o.sequence > COALESCE(f.sequence, 0)
		 ORDER BY o.hlc_wall_ms, o.hlc_logical, o.replica_id, o.sequence`)
	if err != nil {
		return nil, nil, err
	}
	defer C.sqlite3_finalize(stmt)
	records := map[string]*resourceRecord{}
	references := map[string]*resourceReference{}
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return records, references, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, nil, s.stepErrLocked(rc)
		}
		kind, recordType, recordID := columnText(stmt, 0), columnText(stmt, 1), columnText(stmt, 2)
		order := syncmerge.Order{
			WallMS: columnInt64(stmt, 4), Logical: columnInt64(stmt, 5),
			ReplicaID: columnText(stmt, 6), Sequence: columnInt64(stmt, 7),
		}
		payload := []byte(columnText(stmt, 3))
		if recordType == "document_resource" {
			reference, err := decodeResourceReference(recordID, kind, payload, order)
			if err != nil {
				return nil, nil, err
			}
			if existing, found := references[recordID]; !found || syncmerge.Compare(order, existing.order) > 0 {
				references[recordID] = reference
			}
			continue
		}
		record, err := decodeResourceRecord(recordID, kind, payload, order)
		if err != nil {
			return nil, nil, err
		}
		if existing, found := records[recordID]; !found || syncmerge.Compare(order, existing.order) > 0 {
			records[recordID] = record
		}
	}
}

func decodeResourceRecord(recordID, kind string, payload []byte, order syncmerge.Order) (*resourceRecord, error) {
	record := &resourceRecord{resourceID: recordID, order: order, sourceReplica: order.ReplicaID}
	if kind == "record.delete" {
		record.deleted = true
		return record, nil
	}
	var fields struct {
		CollectionID   string `json:"collection_id"`
		BlobSHA256     string `json:"blob_sha256"`
		Filename       string `json:"filename"`
		MIMEType       string `json:"mime_type"`
		MetadataJSON   string `json:"metadata_json"`
		BlobLength     int64  `json:"blob_length"`
		BlobMIMEType   string `json:"blob_mime_type"`
		ChunkCount     int    `json:"chunk_count"`
		ManifestSHA256 string `json:"manifest_sha256"`
	}
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, fmt.Errorf("resource operation payload: %w", err)
	}
	if len(fields.BlobSHA256) != 64 || fields.BlobLength < 0 || fields.BlobLength > syncassets.MaxObjectBytes {
		return nil, fmt.Errorf("%w: resource %q does not name a bounded object", ErrInvalidInput, recordID)
	}
	record.collectionID, record.blobSHA256 = fields.CollectionID, fields.BlobSHA256
	record.filename, record.mimeType = fields.Filename, defaultMIME(fields.MIMEType)
	record.metadataJSON = jsonOrEmptyObject(fields.MetadataJSON)
	record.blobLength, record.blobMIMEType = fields.BlobLength, fields.BlobMIMEType
	record.chunkCount, record.manifestSHA256 = fields.ChunkCount, fields.ManifestSHA256
	return record, nil
}

func decodeResourceReference(elementID, kind string, payload []byte, order syncmerge.Order) (*resourceReference, error) {
	var fields struct {
		DocumentID   string `json:"document_id"`
		ResourceID   string `json:"resource_id"`
		RelationType string `json:"relation_type"`
		Ordinal      int64  `json:"ordinal"`
		AnchorJSON   string `json:"anchor_json"`
	}
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, fmt.Errorf("document_resource operation payload: %w", err)
	}
	if fields.DocumentID == "" || fields.ResourceID == "" {
		return nil, fmt.Errorf("%w: document_resource %q is incomplete", ErrInvalidInput, elementID)
	}
	return &resourceReference{
		elementID: elementID, documentID: fields.DocumentID, resourceID: fields.ResourceID,
		relationType: fields.RelationType, ordinal: fields.Ordinal,
		anchorJSON: jsonOrEmptyObject(fields.AnchorJSON),
		present:    kind != "membership.remove", order: order,
	}, nil
}

// applySyncResourceRecordsLocked creates the blob and resource rows a peer's
// operations describe. A blob whose bytes this replica does not hold becomes a
// row with no storage path and `unavailable` availability, which is what lets
// the resource — and every note that references it — exist immediately.
func (s *SQLiteStore) applySyncResourceRecordsLocked(records map[string]*resourceRecord) error {
	ids := sortedKeys(records)
	for _, id := range ids {
		record := records[id]
		if record.deleted {
			if err := s.execPreparedLocked(`DELETE FROM document_resource_refs WHERE resource_id = ?`, id); err != nil {
				return err
			}
			if err := s.execPreparedLocked(`DELETE FROM resources WHERE id = ?`, id); err != nil {
				return err
			}
			continue
		}
		if err := s.admitRemoteBlobLocked(record); err != nil {
			return err
		}
		collectionID := record.collectionID
		if collectionID == "" {
			collectionID = syncmerge.DefaultCollectionID
		}
		if err := s.execPreparedLocked(
			`INSERT INTO resources(id, collection_id, blob_sha256, filename, mime_type, metadata_json)
			 VALUES(?, ?, ?, NULLIF(?, ''), ?, ?)
			 ON CONFLICT(id) DO UPDATE SET collection_id=excluded.collection_id, blob_sha256=excluded.blob_sha256,
				filename=excluded.filename, mime_type=excluded.mime_type, metadata_json=excluded.metadata_json`,
			id, collectionID, record.blobSHA256, record.filename, record.mimeType, record.metadataJSON); err != nil {
			return err
		}
	}
	return nil
}

// admitRemoteBlobLocked records an object's identity, size, and transfer shape
// without any of its bytes, remembers which peer advertised it, and decides
// whether this replica fetches it on its own.
func (s *SQLiteStore) admitRemoteBlobLocked(record *resourceRecord) error {
	existing, found, err := s.blobRowLocked(record.blobSHA256)
	if err != nil {
		return err
	}
	if found && existing.storagePath != "" {
		// The bytes are already here — the same attachment shared between two
		// replicas is the same object, so this is deduplication, not a fetch.
		return s.recordBlobSourceLocked(record)
	}
	if !found {
		if err := s.execPreparedLocked(
			`INSERT INTO blobs(sha256, storage_path, size_bytes, mime_type, availability)
			 VALUES(?, '', ?, ?, 'unavailable')`,
			record.blobSHA256, strconv.FormatInt(record.blobLength, 10), defaultMIME(record.blobMIMEType)); err != nil {
			return err
		}
	}
	if err := s.recordRemoteManifestShellLocked(record); err != nil {
		return err
	}
	if err := s.recordBlobSourceLocked(record); err != nil {
		return err
	}
	return s.decideMaterializationLocked(record)
}

// recordRemoteManifestShellLocked stores what the operation said about an
// object's transfer shape. For an object below G2's threshold the manifest is
// derivable here and complete; above it, only the digest is known until the
// manifest itself is fetched, and the shell records that it is incomplete.
func (s *SQLiteStore) recordRemoteManifestShellLocked(record *resourceRecord) error {
	if !syncassets.IsChunked(record.blobLength) {
		manifest, err := syncassets.WholeManifest(record.blobSHA256, record.blobLength)
		if err != nil {
			return err
		}
		return s.storeBlobManifestLocked(manifest, defaultMIME(record.blobMIMEType), false)
	}
	count, err := syncassets.ChunkCount(record.blobLength)
	if err != nil {
		return err
	}
	if record.chunkCount != 0 && record.chunkCount != count {
		return fmt.Errorf("%w: resource %q claims %d chunks for %d bytes", ErrInvalidInput, record.resourceID, record.chunkCount, record.blobLength)
	}
	return s.execPreparedLocked(
		`INSERT INTO sync_blob_manifests(blob_sha256, byte_length, mime_type, chunk_count, chunk_bytes, manifest_sha256, complete)
		 VALUES(?, ?, ?, ?, ?, ?, 0)
		 ON CONFLICT(blob_sha256) DO UPDATE SET byte_length=excluded.byte_length, mime_type=excluded.mime_type,
			chunk_count=excluded.chunk_count, chunk_bytes=excluded.chunk_bytes,
			manifest_sha256=CASE WHEN sync_blob_manifests.complete = 1 THEN sync_blob_manifests.manifest_sha256 ELSE excluded.manifest_sha256 END,
			updated_at=CURRENT_TIMESTAMP`,
		record.blobSHA256, strconv.FormatInt(record.blobLength, 10), defaultMIME(record.blobMIMEType),
		strconv.Itoa(count), strconv.Itoa(syncassets.ChunkBytes), record.manifestSHA256)
}

func (s *SQLiteStore) recordBlobSourceLocked(record *resourceRecord) error {
	if record.sourceReplica == "" {
		return nil
	}
	return s.execPreparedLocked(
		`INSERT INTO sync_blob_sources(blob_sha256, replica_id, byte_length) VALUES(?, ?, ?)
		 ON CONFLICT(blob_sha256, replica_id) DO UPDATE SET byte_length=excluded.byte_length, advertised_at=CURRENT_TIMESTAMP`,
		record.blobSHA256, record.sourceReplica, strconv.FormatInt(record.blobLength, 10))
}

// decideMaterializationLocked applies the policy to a newly known object. It
// only ever *raises* intent: an object already pinned or requested is not
// downgraded by a later operation describing the same bytes.
func (s *SQLiteStore) decideMaterializationLocked(record *resourceRecord) error {
	pinned, requested, err := s.materializationIntentLocked(record.blobSHA256)
	if err != nil {
		return err
	}
	spent, err := s.countLocked(`SELECT COALESCE(SUM(byte_length), 0) FROM sync_blob_manifests m
		JOIN blobs b ON b.sha256 = m.blob_sha256
		JOIN sync_blob_materialization x ON x.blob_sha256 = m.blob_sha256
		WHERE b.availability = 'unavailable' AND x.requested = 1`)
	if err != nil {
		return err
	}
	decision := syncassets.Decide(s.materializationPolicy(), record.blobLength, pinned, requested, spent)
	return s.execPreparedLocked(
		`INSERT INTO sync_blob_materialization(blob_sha256, pinned, requested, last_reason)
		 VALUES(?, 0, ?, ?)
		 ON CONFLICT(blob_sha256) DO UPDATE SET
			requested = MAX(sync_blob_materialization.requested, excluded.requested),
			last_reason = excluded.last_reason, updated_at = CURRENT_TIMESTAMP`,
		record.blobSHA256, boolNumber(decision.Fetch), decision.Reason)
}

func (s *SQLiteStore) materializationIntentLocked(sha256Hex string) (bool, bool, error) {
	stmt, err := s.prepareLocked(`SELECT pinned, requested FROM sync_blob_materialization WHERE blob_sha256 = ?`)
	if err != nil {
		return false, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{sha256Hex}); err != nil {
		return false, false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return columnInt64(stmt, 0) == 1, columnInt64(stmt, 1) == 1, nil
	case C.SQLITE_DONE:
		return false, false, nil
	default:
		return false, false, s.stepErrLocked(rc)
	}
}

// materializationPolicy returns the configured policy. It is a field rather
// than a config read so a test can set it without a config file, and it
// normalizes rather than failing: a typo should not stop a library from
// fetching its own attachments.
func (s *SQLiteStore) materializationPolicy() syncassets.MaterializationPolicy {
	return syncassets.NormalizePolicy(s.assetPolicy)
}

// SetMaterializationPolicy selects eager, lazy, or all. It affects future
// decisions only; anything already pinned or requested stays wanted.
func (s *SQLiteStore) SetMaterializationPolicy(policy string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assetPolicy = string(syncassets.NormalizePolicy(policy))
}

// applySyncResourceReferencesLocked converges which notes name which
// attachments. A reference whose document or resource has not arrived is left
// for a later pass rather than dropped: the operations are immutable and the
// next admission that supplies the missing end applies it.
func (s *SQLiteStore) applySyncResourceReferencesLocked(references map[string]*resourceReference) error {
	for _, id := range sortedKeys(references) {
		reference := references[id]
		if !reference.present {
			if err := s.execPreparedLocked(
				`DELETE FROM document_resource_refs WHERE document_id = ? AND resource_id = ? AND relation_type = ? AND ordinal = ?`,
				reference.documentID, reference.resourceID, reference.relationType,
				strconv.FormatInt(reference.ordinal, 10)); err != nil {
				return err
			}
			continue
		}
		ready, err := s.countLocked(
			`SELECT COUNT(*) FROM documents d JOIN resources r ON r.id = ? WHERE d.id = ?`,
			reference.resourceID, reference.documentID)
		if err != nil {
			return err
		}
		if ready == 0 {
			continue
		}
		if err := s.execPreparedLocked(
			`INSERT INTO document_resource_refs(document_id, resource_id, relation_type, ordinal, anchor_json)
			 VALUES(?, ?, ?, ?, ?)
			 ON CONFLICT(document_id, resource_id, relation_type, ordinal) DO UPDATE SET anchor_json=excluded.anchor_json`,
			reference.documentID, reference.resourceID, reference.relationType,
			strconv.FormatInt(reference.ordinal, 10), reference.anchorJSON); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// ObjectProvider fetches attachment bytes from somewhere else. G8 defines it
// and verifies everything it returns; G11's shared directory and G14's REST
// data plane implement it. Nothing in this file knows which is in use, and a
// provider is never trusted — every byte it returns is checked against a hash
// the protocol already carried.
type ObjectProvider interface {
	// FetchManifest returns the chunk manifest of an object. It is called only
	// for objects above the whole-object threshold; below it the manifest is
	// derivable and asking would be pointless.
	FetchManifest(ctx context.Context, blobSHA256 string) (syncassets.Manifest, error)
	// FetchChunk returns one transfer segment.
	FetchChunk(ctx context.Context, blobSHA256 string, ordinal int) ([]byte, error)
}

// MaterializationReport summarizes one pass. It counts rather than names, so a
// caller can log it without disclosing which attachments a library holds.
type MaterializationReport struct {
	Considered   int
	Materialized int
	Deferred     int
	Failed       int
	FetchedBytes int64
	Reasons      map[string]int
}

// wantedObject is one object this replica intends to fetch.
type wantedObject struct {
	sha256      string
	byteLength  int64
	mimeType    string
	chunkCount  int
	manifestSHA string
	pinned      bool
}

// MaterializeResources fetches the bytes of objects this replica wants,
// verifies every one of them, and atomically marks them local. It is safe to
// call repeatedly: an interrupted pass leaves verified chunks staged, and the
// next call asks only for what is still missing.
func (s *SQLiteStore) MaterializeResources(ctx context.Context, provider ObjectProvider, limit int) (MaterializationReport, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return MaterializationReport{}, err
	}
	if provider == nil {
		return MaterializationReport{}, fmt.Errorf("%w: an object provider is required", ErrInvalidInput)
	}
	if limit <= 0 || limit > syncassets.MaxChunks {
		limit = syncassets.MaxConcurrentFetches
	}
	wanted, err := s.wantedObjects(limit)
	if err != nil {
		return MaterializationReport{}, err
	}
	report := MaterializationReport{Considered: len(wanted), Reasons: map[string]int{}}
	for _, object := range wanted {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		fetched, reason, err := s.materializeOne(ctx, provider, object)
		if err != nil {
			return report, err
		}
		report.Reasons[reason]++
		switch reason {
		case "materialized":
			report.Materialized++
			report.FetchedBytes += fetched
		case "deferred_staging_budget":
			report.Deferred++
		default:
			report.Failed++
		}
	}
	return report, nil
}

func (s *SQLiteStore) wantedObjects(limit int) ([]wantedObject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`
		SELECT b.sha256, m.byte_length, m.mime_type, m.chunk_count, m.manifest_sha256, x.pinned
		  FROM blobs b
		  JOIN sync_blob_manifests m ON m.blob_sha256 = b.sha256
		  JOIN sync_blob_materialization x ON x.blob_sha256 = b.sha256
		 WHERE b.availability = 'unavailable' AND (x.pinned = 1 OR x.requested = 1)
		 ORDER BY x.pinned DESC, m.byte_length ASC, b.sha256 ASC
		 LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{strconv.Itoa(limit)}); err != nil {
		return nil, err
	}
	var wanted []wantedObject
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return wanted, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		wanted = append(wanted, wantedObject{
			sha256: columnText(stmt, 0), byteLength: columnInt64(stmt, 1), mimeType: columnText(stmt, 2),
			chunkCount: int(columnInt64(stmt, 3)), manifestSHA: columnText(stmt, 4), pinned: columnInt64(stmt, 5) == 1,
		})
	}
}
