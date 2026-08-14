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

	"github.com/renesugar/notrios/internal/archivev2"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/syncbackup"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// Backup is what a peer says about a snapshot it has produced for this replica.
type Backup struct {
	ID             string           `json:"backup_id"`
	SealedBytes    int64            `json:"sealed_bytes"`
	SealedSHA256   string           `json:"sealed_sha256"`
	WrappedKey     string           `json:"wrapped_payload_key"`
	SnapshotVector syncstate.Vector `json:"snapshot_vector"`
}

var (
	// ErrBackupRefused reports a peer that will not produce a snapshot.
	ErrBackupRefused = errors.New("the peer will not produce a snapshot for this replica")
	// ErrBackupCorrupt reports bytes that arrived but do not match what the
	// peer said they would be.
	ErrBackupCorrupt = errors.New("the downloaded snapshot does not match its declared hash")
)

// RequestBackup asks a peer for a snapshot.
func RequestBackup(ctx context.Context, client *syncauth.Client) (Backup, error) {
	status, body, err := client.Do(ctx, http.MethodPost, "/api/v1/sync/backups", []byte(`{}`))
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
	if backup.ID == "" || backup.SealedBytes <= 0 || backup.SealedSHA256 == "" {
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
	end := backup.SealedBytes - 1
	if limit > 0 && existing+limit-1 < end {
		end = existing + limit - 1
	}
	status, body, err := client.DoRange(ctx, http.MethodGet, "/api/v1/sync/backups/"+backup.ID,
		fmt.Sprintf("bytes=%d-%d", existing, end))
	if err != nil {
		return existing, false, err
	}
	if status != http.StatusPartialContent && status != http.StatusOK {
		return existing, false, fmt.Errorf("the peer answered %d to a snapshot download", status)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return existing, false, err
	}
	defer file.Close()
	if _, err := file.WriteAt(body, existing); err != nil {
		return existing, false, err
	}
	written := existing + int64(len(body))
	return written, written >= backup.SealedBytes, nil
}

// OpenBackup unwraps the payload key, decrypts the sealed file, extracts the
// container, and verifies it as an archive-v2 snapshot.
//
// The order is the resolved decision made literal. The transport's own checks
// come first — the sealed bytes must hash to what the peer declared — then the
// frames authenticate, and only then is the ZIP opened. **Archive-v2's verifier
// decides whether the result is a snapshot**; ZIP parsing is never the trust
// boundary, and nothing canonical is written before that verifier has run.
func OpenBackup(backup Backup, sealedPath, workspace string, keys syncwire.KeyRing,
	verifier syncwire.Verifier) (string, archivev2.VerificationReport, error) {
	if err := verifySealedDigest(sealedPath, backup.SealedSHA256); err != nil {
		return "", archivev2.VerificationReport{}, err
	}
	wrapped, err := base64.StdEncoding.DecodeString(backup.WrappedKey)
	if err != nil {
		return "", archivev2.VerificationReport{}, fmt.Errorf("the wrapped payload key is not base64: %w", err)
	}
	_, payloadKey, err := syncwire.Open(keys, verifier, wrapped, syncwire.Limits{})
	if err != nil {
		return "", archivev2.VerificationReport{}, fmt.Errorf("the payload key did not open: %w", err)
	}

	sealed, err := os.Open(sealedPath)
	if err != nil {
		return "", archivev2.VerificationReport{}, err
	}
	defer sealed.Close()
	container := filepath.Join(workspace, "snapshot.zip")
	plaintext, err := os.OpenFile(container, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", archivev2.VerificationReport{}, err
	}
	openErr := syncbackup.Open(sealed, payloadKey, plaintext)
	closeErr := plaintext.Close()
	if openErr != nil {
		return "", archivev2.VerificationReport{}, openErr
	}
	if closeErr != nil {
		return "", archivev2.VerificationReport{}, closeErr
	}

	archiveDir := filepath.Join(workspace, "archive")
	if err := syncbackup.Unpack(container, archiveDir); err != nil {
		return "", archivev2.VerificationReport{}, err
	}
	report, err := archivev2.VerifyDirectory(archiveDir, archivev2.DefaultLimits())
	if err != nil {
		return archiveDir, report, err
	}
	return archiveDir, report, nil
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
