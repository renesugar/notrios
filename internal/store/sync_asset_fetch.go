package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/renesugar/notrios/internal/syncassets"
)

// materializeOne fetches, verifies, and installs one object. It returns the
// bytes newly fetched and a reason naming the outcome. Only "materialized"
// means the object is now local; every other reason leaves the object exactly
// as it was — known, referenced, and unavailable — which is the state G8 was
// told to preserve indefinitely rather than paper over.
func (s *SQLiteStore) materializeOne(ctx context.Context, provider ObjectProvider, object wantedObject) (int64, string, error) {
	staged, err := s.stagedBytes()
	if err != nil {
		return 0, "", err
	}
	if staged+object.byteLength > syncassets.MaxStagingBytes {
		return 0, "deferred_staging_budget", s.recordMaterializationOutcome(object.sha256, "deferred_staging_budget", false)
	}

	manifest, reason, err := s.resolveManifest(ctx, provider, object)
	if err != nil {
		return 0, "", err
	}
	if reason != "" {
		return 0, reason, s.recordMaterializationOutcome(object.sha256, reason, true)
	}

	fetched, reason, err := s.fetchMissingChunks(ctx, provider, object, manifest)
	if err != nil {
		return 0, "", err
	}
	if reason != "" {
		return fetched, reason, s.recordMaterializationOutcome(object.sha256, reason, true)
	}

	reason, err = s.assembleAndInstall(ctx, object, manifest)
	if err != nil {
		return fetched, "", err
	}
	if reason != "materialized" {
		return fetched, reason, s.recordMaterializationOutcome(object.sha256, reason, true)
	}
	return fetched, reason, nil
}

// resolveManifest produces the manifest a fetch verifies against. A whole
// object's manifest is derived locally; a chunked object's is fetched and then
// checked against the digest the resource operation already carried, so a
// tampered manifest is refused before a single chunk is requested.
func (s *SQLiteStore) resolveManifest(ctx context.Context, provider ObjectProvider, object wantedObject) (syncassets.Manifest, string, error) {
	if !syncassets.IsChunked(object.byteLength) {
		manifest, err := syncassets.WholeManifest(object.sha256, object.byteLength)
		if err != nil {
			return syncassets.Manifest{}, "invalid_manifest", nil
		}
		return manifest, "", nil
	}
	stored, complete, err := s.storedManifest(object.sha256)
	if err != nil {
		return syncassets.Manifest{}, "", err
	}
	if complete {
		return stored, "", nil
	}
	manifest, err := provider.FetchManifest(ctx, object.sha256)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return syncassets.Manifest{}, "", err
		}
		return syncassets.Manifest{}, "no_source", nil
	}
	if err := manifest.Validate(object.sha256, object.byteLength); err != nil {
		return syncassets.Manifest{}, "invalid_manifest", nil
	}
	if object.manifestSHA != "" && manifest.Digest() != object.manifestSHA {
		// The manifest is internally consistent but is not the one the
		// protocol named. Accepting it would let a source choose the chunk
		// hashes it will later be checked against.
		return syncassets.Manifest{}, "manifest_digest_mismatch", nil
	}
	s.mu.Lock()
	err = s.storeBlobManifestLocked(manifest, object.mimeType, false)
	s.mu.Unlock()
	if err != nil {
		return syncassets.Manifest{}, "", err
	}
	if err := s.markManifestComplete(object.sha256); err != nil {
		return syncassets.Manifest{}, "", err
	}
	return manifest, "", nil
}

// fetchMissingChunks asks only for the segments this replica does not already
// hold, so an interrupted transfer resumes instead of restarting. Every chunk
// is verified against the manifest before it is written, and a chunk that fails
// is discarded rather than staged.
func (s *SQLiteStore) fetchMissingChunks(ctx context.Context, provider ObjectProvider, object wantedObject, manifest syncassets.Manifest) (int64, string, error) {
	directory := s.stagingPath(object.sha256)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return 0, "", err
	}
	var fetched int64
	for ordinal, chunk := range manifest.Chunks {
		if err := ctx.Err(); err != nil {
			return fetched, "", err
		}
		path := filepath.Join(directory, strconv.Itoa(ordinal))
		if held, err := s.stagedChunkIsValid(path, chunk); err != nil {
			return fetched, "", err
		} else if held {
			continue
		}
		content, err := provider.FetchChunk(ctx, object.sha256, ordinal)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return fetched, "", err
			}
			return fetched, "no_source", nil
		}
		if err := manifest.VerifyChunk(ordinal, content); err != nil {
			return fetched, "corrupt_chunk", nil
		}
		if err := writeFileAtomically(path, content); err != nil {
			return fetched, "", err
		}
		fetched += int64(len(content))
		if err := s.markChunkFetched(object.sha256, ordinal); err != nil {
			return fetched, "", err
		}
	}
	return fetched, "", nil
}

// stagedChunkIsValid reports whether a previously staged chunk is still exactly
// what the manifest says it should be. Re-verifying costs one read and is what
// makes resume safe: a staged file that was truncated by a crash, or written by
// an earlier manifest, must not be mistaken for progress.
func (s *SQLiteStore) stagedChunkIsValid(path string, chunk syncassets.Chunk) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Size() != chunk.Length {
		return false, os.Remove(path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if syncassets.SHA256Hex(content) != chunk.SHA256 {
		return false, os.Remove(path)
	}
	return true, nil
}

// assembleAndInstall concatenates the staged chunks through the ordinary blob
// write path, so the object is hashed, sniffed, and placed by exactly the code
// an upload uses. Nothing here writes into the content-addressed tree directly.
func (s *SQLiteStore) assembleAndInstall(ctx context.Context, object wantedObject, manifest syncassets.Manifest) (string, error) {
	directory := s.stagingPath(object.sha256)
	files := make([]*os.File, 0, len(manifest.Chunks))
	readers := make([]io.Reader, 0, len(manifest.Chunks))
	defer func() {
		for _, file := range files {
			_ = file.Close()
		}
	}()
	for ordinal := range manifest.Chunks {
		file, err := os.Open(filepath.Join(directory, strconv.Itoa(ordinal)))
		if err != nil {
			if os.IsNotExist(err) {
				return "incomplete", nil
			}
			return "", err
		}
		files = append(files, file)
		readers = append(readers, file)
	}

	// The MIME type is deliberately left empty so the blob writer sniffs the
	// assembled content rather than trusting what the sender advertised.
	stored, cleanup, err := s.writeBlob(ctx, io.MultiReader(readers...), "")
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return "", err
	}
	if stored.SHA256 != object.sha256 || stored.SizeBytes != object.byteLength {
		// Every chunk verified individually and the whole still is not the
		// named object. The assembled bytes are discarded, not stored.
		return "object_mismatch", nil
	}
	if err := syncassets.AcceptableMIME(object.mimeType, stored.MIMEType); err != nil {
		return "mime_mismatch", nil
	}
	stored.MIMEType = defaultMIME(object.mimeType)

	s.mu.Lock()
	err = s.upsertLocalBlobWithManifestLocked(stored, stored.MIMEType, &manifest)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(directory); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return "materialized", nil
}

func writeFileAtomically(path string, content []byte) error {
	temporary := path + ".part"
	if err := os.WriteFile(temporary, content, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func (s *SQLiteStore) stagingPath(sha256Hex string) string {
	return filepath.Join(s.assetRoot, stagingDirectory, sha256Hex)
}

func (s *SQLiteStore) storedManifest(sha256Hex string) (syncassets.Manifest, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT byte_length, chunk_bytes, complete FROM sync_blob_manifests WHERE blob_sha256 = ?`)
	if err != nil {
		return syncassets.Manifest{}, false, err
	}
	if err := bindAll(stmt, []string{sha256Hex}); err != nil {
		C.sqlite3_finalize(stmt)
		return syncassets.Manifest{}, false, err
	}
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		C.sqlite3_finalize(stmt)
		return syncassets.Manifest{}, false, nil
	}
	if rc != C.SQLITE_ROW {
		err := s.stepErrLocked(rc)
		C.sqlite3_finalize(stmt)
		return syncassets.Manifest{}, false, err
	}
	manifest := syncassets.Manifest{
		ObjectSHA256: sha256Hex, ByteLength: columnInt64(stmt, 0), ChunkBytes: columnInt64(stmt, 1),
	}
	complete := columnInt64(stmt, 2) == 1
	C.sqlite3_finalize(stmt)
	if !complete {
		return manifest, false, nil
	}

	chunks, err := s.prepareLocked(`SELECT ordinal, chunk_sha256, byte_length FROM sync_blob_chunks WHERE blob_sha256 = ? ORDER BY ordinal`)
	if err != nil {
		return syncassets.Manifest{}, false, err
	}
	defer C.sqlite3_finalize(chunks)
	if err := bindAll(chunks, []string{sha256Hex}); err != nil {
		return syncassets.Manifest{}, false, err
	}
	for {
		rc := C.sqlite3_step(chunks)
		if rc == C.SQLITE_DONE {
			return manifest, true, nil
		}
		if rc != C.SQLITE_ROW {
			return syncassets.Manifest{}, false, s.stepErrLocked(rc)
		}
		manifest.Chunks = append(manifest.Chunks, syncassets.Chunk{
			Ordinal: int(columnInt64(chunks, 0)), SHA256: columnText(chunks, 1), Length: columnInt64(chunks, 2),
		})
	}
}

func (s *SQLiteStore) markManifestComplete(sha256Hex string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execPreparedLocked(`UPDATE sync_blob_manifests SET complete = 1, updated_at = CURRENT_TIMESTAMP WHERE blob_sha256 = ?`, sha256Hex)
}

// markChunkFetched records one verified segment. Staged bytes are recomputed
// from the chunks actually held rather than accumulated per pass, so a resumed
// transfer reports what is on disk instead of what this call fetched.
func (s *SQLiteStore) markChunkFetched(sha256Hex string, ordinal int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.execPreparedLocked(
		`UPDATE sync_blob_chunks SET fetched = 1 WHERE blob_sha256 = ? AND ordinal = ?`,
		sha256Hex, strconv.Itoa(ordinal)); err != nil {
		return err
	}
	return s.execPreparedLocked(
		`INSERT INTO sync_blob_materialization(blob_sha256, staged_bytes)
		 VALUES(?, (SELECT COALESCE(SUM(byte_length), 0) FROM sync_blob_chunks WHERE blob_sha256 = ? AND fetched = 1))
		 ON CONFLICT(blob_sha256) DO UPDATE SET
			staged_bytes = (SELECT COALESCE(SUM(byte_length), 0) FROM sync_blob_chunks WHERE blob_sha256 = ? AND fetched = 1),
			updated_at = CURRENT_TIMESTAMP`,
		sha256Hex, sha256Hex, sha256Hex)
}

func (s *SQLiteStore) stagedBytes() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.countLocked(`SELECT COALESCE(SUM(staged_bytes), 0) FROM sync_blob_materialization`)
}

// recordMaterializationOutcome keeps the reason a pass produced no bytes.
// `countAttempt` separates a refusal from a deferral: being over a local disk
// budget is not the object's fault and must not look like a failing source.
func (s *SQLiteStore) recordMaterializationOutcome(sha256Hex, reason string, countAttempt bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execPreparedLocked(
		`INSERT INTO sync_blob_materialization(blob_sha256, attempts, last_reason) VALUES(?, ?, ?)
		 ON CONFLICT(blob_sha256) DO UPDATE SET
			attempts = sync_blob_materialization.attempts + ?,
			last_reason = excluded.last_reason, updated_at = CURRENT_TIMESTAMP`,
		sha256Hex, boolNumber(countAttempt), reason, boolNumber(countAttempt))
}

// PinResource marks an attachment as one this replica keeps locally regardless
// of size, and requests it if it is not here yet. Unpinning does not delete
// anything: retention is G17's, and a pin is about wanting bytes rather than
// about keeping them.
func (s *SQLiteStore) PinResource(ctx context.Context, resourceID string, pinned bool) error {
	return s.setResourceIntent(ctx, resourceID, "pinned", pinned)
}

// RequestResource asks for one attachment's bytes without pinning it.
func (s *SQLiteStore) RequestResource(ctx context.Context, resourceID string) error {
	return s.setResourceIntent(ctx, resourceID, "requested", true)
}

func (s *SQLiteStore) setResourceIntent(ctx context.Context, resourceID, column string, value bool) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sha256Hex, _, err := s.resourceBlobLocked(resourceID)
	if err != nil {
		return err
	}
	if sha256Hex == "" {
		return ErrNotFound
	}
	// Column names are not user input: this method has exactly two callers and
	// both pass a literal.
	return s.execPreparedLocked(
		`INSERT INTO sync_blob_materialization(blob_sha256, `+column+`) VALUES(?, ?)
		 ON CONFLICT(blob_sha256) DO UPDATE SET `+column+` = excluded.`+column+`, updated_at = CURRENT_TIMESTAMP`,
		sha256Hex, boolNumber(value))
}

// ResourceAvailability reports what this replica holds of one attachment.
type ResourceAvailability struct {
	ResourceID    string
	BlobSHA256    string
	ByteLength    int64
	Available     bool
	Pinned        bool
	Requested     bool
	FetchedChunks int
	TotalChunks   int
	LastReason    string
	SourceCount   int
}

// ResourceAvailabilityFor reports one attachment's local state. It is the read
// a UI needs to say "not downloaded yet" instead of showing a broken image.
func (s *SQLiteStore) ResourceAvailabilityFor(ctx context.Context, resourceID string) (ResourceAvailability, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return ResourceAvailability{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`
		SELECT r.id, b.sha256, b.size_bytes, b.availability,
		       COALESCE(x.pinned, 0), COALESCE(x.requested, 0), COALESCE(x.last_reason, ''),
		       COALESCE(m.chunk_count, 1),
		       (SELECT COUNT(*) FROM sync_blob_chunks c WHERE c.blob_sha256 = b.sha256 AND c.fetched = 1),
		       (SELECT COUNT(*) FROM sync_blob_sources s2 WHERE s2.blob_sha256 = b.sha256)
		  FROM resources r
		  JOIN blobs b ON b.sha256 = r.blob_sha256
		  LEFT JOIN sync_blob_materialization x ON x.blob_sha256 = b.sha256
		  LEFT JOIN sync_blob_manifests m ON m.blob_sha256 = b.sha256
		 WHERE r.id = ?`)
	if err != nil {
		return ResourceAvailability{}, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{resourceID}); err != nil {
		return ResourceAvailability{}, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_ROW:
		return ResourceAvailability{
			ResourceID: columnText(stmt, 0), BlobSHA256: columnText(stmt, 1), ByteLength: columnInt64(stmt, 2),
			Available: columnText(stmt, 3) == "local", Pinned: columnInt64(stmt, 4) == 1,
			Requested: columnInt64(stmt, 5) == 1, LastReason: columnText(stmt, 6),
			TotalChunks: int(columnInt64(stmt, 7)), FetchedChunks: int(columnInt64(stmt, 8)),
			SourceCount: int(columnInt64(stmt, 9)),
		}, nil
	case C.SQLITE_DONE:
		return ResourceAvailability{}, ErrNotFound
	default:
		return ResourceAvailability{}, s.stepErrLocked(rc)
	}
}

// LocalObjectProvider serves objects out of one store's asset tree. It is the
// G8 fixture carrier: G11 and G14 own the real ones, and nothing in the
// materialization path knows the difference.
type LocalObjectProvider struct {
	source *SQLiteStore
}

// NewLocalObjectProvider returns a provider backed by another replica's blobs.
func NewLocalObjectProvider(source *SQLiteStore) *LocalObjectProvider {
	return &LocalObjectProvider{source: source}
}

func (p *LocalObjectProvider) FetchManifest(ctx context.Context, blobSHA256 string) (syncassets.Manifest, error) {
	p.source.mu.Lock()
	blob, found, err := p.source.blobRowLocked(blobSHA256)
	p.source.mu.Unlock()
	if err != nil {
		return syncassets.Manifest{}, err
	}
	if !found || blob.storagePath == "" {
		return syncassets.Manifest{}, ErrResourceUnavailable
	}
	return p.source.buildBlobManifest(ctx, blob)
}

func (p *LocalObjectProvider) FetchChunk(ctx context.Context, blobSHA256 string, ordinal int) ([]byte, error) {
	p.source.mu.Lock()
	blob, found, err := p.source.blobRowLocked(blobSHA256)
	p.source.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if !found || blob.storagePath == "" {
		return nil, ErrResourceUnavailable
	}
	offset, length, err := syncassets.ChunkRange(blob.sizeBytes, ordinal)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(p.source.assetRoot, blob.storagePath))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content := make([]byte, length)
	if _, err := file.ReadAt(content, offset); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return content, nil
}

var _ ObjectProvider = (*LocalObjectProvider)(nil)
