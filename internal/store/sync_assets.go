package store

/*
#include <sqlite3.h>
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/renesugar/notrios/internal/syncassets"
)

// ErrResourceUnavailable reports a resource whose metadata this replica holds
// but whose bytes it has not materialized. It is deliberately distinct from
// ErrNotFound: the resource exists, is referenced, and can be requested, and a
// caller that conflated the two would tell a user their attachment was gone.
var ErrResourceUnavailable = errors.New("resource bytes are not available on this replica")

// ensureSchemaV23 adds G8's blob availability, chunk manifests, and fetch
// state. Every blob already present is local by definition — it has a file —
// so the upgrade is a column default plus a manifest backfill, not a migration
// of content.
func (s *SQLiteStore) ensureSchemaV23(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	version, err := s.pragmaUserVersionLocked()
	hasAvailability := false
	if err == nil {
		stmt, prepareErr := s.prepareLocked(`PRAGMA table_info(blobs)`)
		if prepareErr != nil {
			err = prepareErr
		} else {
			for C.sqlite3_step(stmt) == C.SQLITE_ROW {
				if columnText(stmt, 1) == "availability" {
					hasAvailability = true
				}
			}
			C.sqlite3_finalize(stmt)
		}
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if version >= 23 {
		return nil
	}
	if !hasAvailability {
		if err := s.Exec(ctx, `ALTER TABLE blobs ADD COLUMN availability TEXT NOT NULL DEFAULT 'local';`); err != nil {
			return err
		}
	}
	migration, err := migrationFS.ReadFile("migrations/0023_sync_assets.sql")
	if err != nil {
		return fmt.Errorf("read schema v23 migration: %w", err)
	}
	if err := s.Exec(ctx, string(migration)); err != nil {
		return err
	}
	return s.backfillBlobManifests(ctx)
}

// backfillBlobManifests gives every already-local blob the manifest its
// resource operation will name. It is done in batches and skips anything
// already described, so an interrupted upgrade resumes.
func (s *SQLiteStore) backfillBlobManifests(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		pending, err := s.blobsMissingManifests()
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		for _, blob := range pending {
			if err := s.EnsureBlobManifest(ctx, blob.sha256); err != nil {
				return err
			}
		}
	}
}

type blobRow struct {
	sha256      string
	storagePath string
	sizeBytes   int64
	mimeType    string
}

func (s *SQLiteStore) blobsMissingManifests() ([]blobRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT b.sha256, b.storage_path, b.size_bytes, b.mime_type
		FROM blobs b LEFT JOIN sync_blob_manifests m ON m.blob_sha256 = b.sha256
		WHERE m.blob_sha256 IS NULL AND b.availability = 'local' LIMIT 256`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	var rows []blobRow
	for {
		rc := C.sqlite3_step(stmt)
		if rc == C.SQLITE_DONE {
			return rows, nil
		}
		if rc != C.SQLITE_ROW {
			return nil, s.stepErrLocked(rc)
		}
		rows = append(rows, blobRow{columnText(stmt, 0), columnText(stmt, 1), columnInt64(stmt, 2), columnText(stmt, 3)})
	}
}

// EnsureBlobManifest computes and stores the chunk manifest of a locally held
// blob. A whole object's manifest is derived without touching the file; a
// chunked object is read once, in bounded pieces, so a sixteen-gigabyte
// attachment never has to be in memory at all.
func (s *SQLiteStore) EnsureBlobManifest(ctx context.Context, sha256Hex string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	blob, found, err := s.blobRowLocked(sha256Hex)
	s.mu.Unlock()
	if err != nil || !found || blob.storagePath == "" {
		return err
	}
	manifest, err := s.buildBlobManifest(ctx, blob)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storeBlobManifestLocked(manifest, blob.mimeType, true)
}

func (s *SQLiteStore) buildBlobManifest(ctx context.Context, blob blobRow) (syncassets.Manifest, error) {
	if !syncassets.IsChunked(blob.sizeBytes) {
		return syncassets.WholeManifest(blob.sha256, blob.sizeBytes)
	}
	file, err := os.Open(filepath.Join(s.assetRoot, blob.storagePath))
	if err != nil {
		return syncassets.Manifest{}, err
	}
	defer file.Close()
	buffer := make([]byte, syncassets.ChunkBytes)
	manifest, err := syncassets.BuildManifest(blob.sizeBytes, func(offset, length int64) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		part := buffer[:length]
		if _, err := file.ReadAt(part, offset); err != nil {
			return nil, err
		}
		return part, nil
	})
	if err != nil {
		return syncassets.Manifest{}, err
	}
	if manifest.ObjectSHA256 != blob.sha256 {
		// The file on disk is not the object the database says it is. Refusing
		// here keeps a corrupted local blob from being advertised to peers as
		// something they can verify.
		return syncassets.Manifest{}, fmt.Errorf("%w: local blob %s hashes to %s", ErrInvalidInput, blob.sha256, manifest.ObjectSHA256)
	}
	return manifest, nil
}

// storeBlobManifestLocked persists a manifest and its chunk rows. `complete`
// marks every chunk as already held, which is true for a manifest built from a
// local file and false for one fetched from a peer.
func (s *SQLiteStore) storeBlobManifestLocked(manifest syncassets.Manifest, mimeType string, complete bool) error {
	if err := s.execPreparedLocked(
		`INSERT INTO sync_blob_manifests(blob_sha256, byte_length, mime_type, chunk_count, chunk_bytes, manifest_sha256, complete)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(blob_sha256) DO UPDATE SET byte_length=excluded.byte_length, mime_type=excluded.mime_type,
			chunk_count=excluded.chunk_count, chunk_bytes=excluded.chunk_bytes,
			manifest_sha256=excluded.manifest_sha256, complete=excluded.complete, updated_at=CURRENT_TIMESTAMP`,
		manifest.ObjectSHA256, strconv.FormatInt(manifest.ByteLength, 10), mimeType,
		strconv.Itoa(len(manifest.Chunks)), strconv.FormatInt(manifest.ChunkBytes, 10),
		manifest.Digest(), boolNumber(complete)); err != nil {
		return err
	}
	for _, chunk := range manifest.Chunks {
		if err := s.execPreparedLocked(
			`INSERT INTO sync_blob_chunks(blob_sha256, ordinal, chunk_sha256, byte_length, fetched)
			 VALUES(?, ?, ?, ?, ?)
			 ON CONFLICT(blob_sha256, ordinal) DO UPDATE SET chunk_sha256=excluded.chunk_sha256,
				byte_length=excluded.byte_length, fetched=MAX(sync_blob_chunks.fetched, excluded.fetched)`,
			manifest.ObjectSHA256, strconv.Itoa(chunk.Ordinal), chunk.SHA256,
			strconv.FormatInt(chunk.Length, 10), boolNumber(complete)); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) blobRowLocked(sha256Hex string) (blobRow, bool, error) {
	stmt, err := s.prepareLocked(`SELECT sha256, storage_path, size_bytes, mime_type FROM blobs WHERE sha256 = ?`)
	if err != nil {
		return blobRow{}, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{sha256Hex}); err != nil {
		return blobRow{}, false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return blobRow{columnText(stmt, 0), columnText(stmt, 1), columnInt64(stmt, 2), columnText(stmt, 3)}, true, nil
	case C.SQLITE_DONE:
		return blobRow{}, false, nil
	default:
		return blobRow{}, false, s.stepErrLocked(rc)
	}
}

// upsertLocalBlobLocked records a blob whose bytes are present. It is the one
// place a blob becomes local, so it is also where an object that was previously
// admitted as unavailable is promoted — which is what makes a materialized
// download and a fresh upload of the same bytes indistinguishable afterwards.
func (s *SQLiteStore) upsertLocalBlobLocked(blob storedBlob, mimeType string) error {
	return s.upsertLocalBlobWithManifestLocked(blob, mimeType, nil)
}

// upsertLocalBlobWithManifestLocked accepts a manifest the caller already
// computed. Materialization has one: it verified every chunk hash on the way
// in, so rebuilding the manifest would mean reading a sixteen-gigabyte file a
// second time to learn what it just checked.
func (s *SQLiteStore) upsertLocalBlobWithManifestLocked(blob storedBlob, mimeType string, known *syncassets.Manifest) error {
	if mimeType == "" {
		mimeType = blob.MIMEType
	}
	if err := s.execPreparedLocked(
		`INSERT INTO blobs(sha256, storage_path, size_bytes, mime_type, availability)
		 VALUES(?, ?, ?, ?, 'local')
		 ON CONFLICT(sha256) DO UPDATE SET storage_path=excluded.storage_path,
			size_bytes=excluded.size_bytes, availability='local'`,
		blob.SHA256, blob.StoragePath, strconv.FormatInt(blob.SizeBytes, 10), mimeType); err != nil {
		return err
	}
	manifest := syncassets.Manifest{}
	if known != nil && known.ObjectSHA256 == blob.SHA256 && known.ByteLength == blob.SizeBytes {
		manifest = *known
	} else {
		built, err := s.buildManifestLocked(blob)
		if err != nil {
			return err
		}
		manifest = built
	}
	if err := s.storeBlobManifestLocked(manifest, mimeType, true); err != nil {
		return err
	}
	// Whatever this replica wanted about the object is satisfied now.
	return s.execPreparedLocked(
		`UPDATE sync_blob_materialization SET requested = 0, staged_bytes = 0, last_reason = '', updated_at = CURRENT_TIMESTAMP
		  WHERE blob_sha256 = ?`, blob.SHA256)
}

// buildManifestLocked reads the stored file only when the object is chunked.
// Below G2's threshold the manifest is one chunk that is the object, so the
// hash the caller already computed is the whole answer.
func (s *SQLiteStore) buildManifestLocked(blob storedBlob) (syncassets.Manifest, error) {
	if !syncassets.IsChunked(blob.SizeBytes) {
		return syncassets.WholeManifest(blob.SHA256, blob.SizeBytes)
	}
	return s.buildBlobManifest(context.Background(), blobRow{blob.SHA256, blob.StoragePath, blob.SizeBytes, blob.MIMEType})
}
