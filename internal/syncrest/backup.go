package syncrest

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"

	"github.com/renesugar/notrios/internal/snapshotimage"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/syncbackup"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// Backup is what a peer says about a snapshot it has produced for this replica.
type Backup struct {
	ID              string           `json:"backup_id"`
	Format          string           `json:"format"`
	SnapshotID      string           `json:"snapshot_id"`
	CommitSHA256    string           `json:"commit_sha256"`
	SourceReplicaID string           `json:"source_replica_id"`
	SealedBytes     int64            `json:"sealed_bytes"`
	SealedSHA256    string           `json:"sealed_sha256"`
	WrappedKey      string           `json:"wrapped_payload_key"`
	SnapshotVector  syncstate.Vector `json:"snapshot_vector"`
}

var (
	// ErrBackupRefused reports a peer that will not produce a snapshot.
	ErrBackupRefused = errors.New("the peer will not produce a snapshot for this replica")
	// ErrBackupCorrupt reports bytes that arrived but do not match what the
	// peer said they would be.
	ErrBackupCorrupt = errors.New("the downloaded snapshot does not match its declared hash")
)

const maxBackupDownloadChunk = int64(syncauth.MaxRangeResponseBytes)

// RequestBackup asks a peer for a snapshot.
func RequestBackup(ctx context.Context, client *syncauth.Client) (Backup, error) {
	requestCtx, cancel := context.WithTimeout(ctx, syncauth.BackupCreationTimeout)
	defer cancel()
	status, body, err := client.DoWithin(requestCtx, http.MethodPost, "/api/v1/sync/backups", []byte(`{}`), syncauth.BackupCreationTimeout)
	if err != nil {
		return Backup{}, err
	}
	if status == http.StatusForbidden {
		return Backup{}, ErrBackupRefused
	}
	if status != http.StatusOK {
		return Backup{}, fmt.Errorf("the peer answered %d to a snapshot request", status)
	}
	var backup Backup
	if err := json.Unmarshal(body, &backup); err != nil {
		return Backup{}, err
	}
	if backup.ID == "" || backup.Format != snapshotimage.CapabilitySQLiteImage || backup.SnapshotID == "" ||
		backup.CommitSHA256 == "" || backup.SourceReplicaID == "" || backup.SealedBytes <= 0 || backup.SealedSHA256 == "" {
		return Backup{}, fmt.Errorf("the peer described a snapshot it did not produce")
	}
	return backup, nil
}

// DownloadBackup fetches a snapshot into a file, resuming from whatever is
// already there.
//
// Resume is the whole point of the byte offset: a library's snapshot is large,
// a connection is not guaranteed, and a transfer that starts again from zero
// every time is a transfer that never finishes on a bad link. The local file's
// own length is the offset, so nothing has to be remembered between attempts.
//
// `limit` bounds one call, which is what makes an interrupted transfer testable
// and a scheduled one bounded.
func DownloadBackup(ctx context.Context, client *syncauth.Client, backup Backup, path string, limit int64) (int64, bool, error) {
	existing := int64(0)
	if info, err := os.Stat(path); err == nil {
		existing = info.Size()
	}
	if existing > backup.SealedBytes {
		// Longer than the peer says it is: this is not a partial copy of that
		// backup, so it is not something to resume.
		return 0, false, fmt.Errorf("%w: local file is longer than the snapshot", ErrBackupCorrupt)
	}
	if existing == backup.SealedBytes {
		return existing, true, nil
	}
	// syncauth's HTTP helper returns a byte slice, so no caller can turn an
	// omitted or huge limit into a multi-gigabyte allocation.
	if limit <= 0 || limit > maxBackupDownloadChunk {
		limit = maxBackupDownloadChunk
	}
	end := backup.SealedBytes - 1
	if limit > 0 && existing+limit-1 < end {
		end = existing + limit - 1
	}
	status, body, err := client.DoRange(ctx, http.MethodGet, "/api/v1/sync/backups/"+backup.ID,
		fmt.Sprintf("bytes=%d-%d", existing, end))
	if err != nil {
		return existing, false, err
	}
	if status != http.StatusPartialContent {
		return existing, false, fmt.Errorf("the peer answered %d to a snapshot download", status)
	}
	if int64(len(body)) != end-existing+1 {
		return existing, false, fmt.Errorf("%w: the peer returned the wrong range length", ErrBackupCorrupt)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return existing, false, err
	}
	if _, err := file.WriteAt(body, existing); err != nil {
		file.Close()
		return existing, false, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return existing, false, err
	}
	if err := file.Close(); err != nil {
		return existing, false, err
	}
	written := existing + int64(len(body))
	return written, written >= backup.SealedBytes, nil
}

// OpenBackup unwraps the payload key, decrypts the sealed file, extracts the
// container, and verifies it as the exact-schema physical snapshot.
//
// The order is the resolved decision made literal. The transport's own checks
// come first — the sealed bytes must hash to what the peer declared — then the
// frames authenticate, and only then is USTAR opened. **The physical verifier
// decides whether the result is a snapshot**; tar parsing is never the trust
// boundary, and nothing canonical is written before that verifier has run.
func OpenBackup(backup Backup, sealedPath, workspace string, keys syncwire.KeyRing,
	verifier syncwire.Verifier) (string, snapshotimage.VerificationReport, error) {
	if err := verifySealedDigest(sealedPath, backup.SealedSHA256); err != nil {
		return "", snapshotimage.VerificationReport{}, err
	}
	wrapped, err := base64.StdEncoding.DecodeString(backup.WrappedKey)
	if err != nil {
		return "", snapshotimage.VerificationReport{}, fmt.Errorf("the wrapped payload key is not base64: %w", err)
	}
	_, payloadKey, err := syncwire.Open(keys, verifier, wrapped, syncwire.Limits{})
	if err != nil {
		return "", snapshotimage.VerificationReport{}, fmt.Errorf("the payload key did not open: %w", err)
	}

	sealed, err := os.Open(sealedPath)
	if err != nil {
		return "", snapshotimage.VerificationReport{}, err
	}
	defer sealed.Close()
	container := filepath.Join(workspace, "snapshot.tar")
	plaintext, err := os.OpenFile(container, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", snapshotimage.VerificationReport{}, err
	}
	openErr := syncbackup.Open(sealed, payloadKey, plaintext)
	closeErr := plaintext.Close()
	if openErr != nil {
		return "", snapshotimage.VerificationReport{}, openErr
	}
	if closeErr != nil {
		return "", snapshotimage.VerificationReport{}, closeErr
	}

	snapshotDir := filepath.Join(workspace, "snapshot")
	// An interrupted open is restartable from the already durable sealed file.
	// Extraction itself has no resumable boundary, so discard only this fixed
	// derived staging subtree before trying it again.
	if err := os.RemoveAll(snapshotDir); err != nil {
		return "", snapshotimage.VerificationReport{}, err
	}
	if err := syncbackup.Unpack(container, snapshotDir); err != nil {
		return "", snapshotimage.VerificationReport{}, err
	}
	// USTAR has no entry for an empty directory in our bounded wrapper. The
	// physical format always has packs/, including for a library with no local
	// external objects; creating that fixed format directory does not make the
	// wrapper a trust boundary because the snapshot verifier still decides.
	if err := os.MkdirAll(filepath.Join(snapshotDir, "packs"), 0o700); err != nil {
		return "", snapshotimage.VerificationReport{}, err
	}
	report, err := snapshotimage.VerifyDirectory(context.Background(), snapshotDir, snapshotimage.DefaultLimits())
	if err != nil {
		return snapshotDir, report, err
	}
	if report.SnapshotID != backup.SnapshotID || report.CommitSHA256 != backup.CommitSHA256 ||
		report.SourceReplicaID != backup.SourceReplicaID || !reflect.DeepEqual(report.SnapshotVector, backup.SnapshotVector) {
		return snapshotDir, report, fmt.Errorf("%w: opened snapshot metadata does not match the authenticated offer", ErrBackupCorrupt)
	}
	return snapshotDir, report, nil
}

func verifySealedDigest(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	digest, err := fileSHA256(file)
	if err != nil {
		return err
	}
	if digest != expected {
		return fmt.Errorf("%w: %s", ErrBackupCorrupt, digest)
	}
	return nil
}

func fileSHA256(file *os.File) (string, error) {
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
