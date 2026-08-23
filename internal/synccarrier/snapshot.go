package synccarrier

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MaxSnapshotTransferBytes is the sealed form of syncbackup's 16 GiB
// plaintext ceiling plus bounded frame overhead. Snapshot bytes use streaming
// methods and never pass through Carrier.Read/Publish's in-memory artifact
// interface.
const MaxSnapshotTransferBytes = int64(17) << 30

// PublishSnapshotFile copies a locally produced encrypted snapshot into this
// replica's snapshot namespace. A partial private staging file resumes only
// after its prefix is compared byte-for-byte with the immutable source.
// `limit` bounds one call; zero means copy to completion.
func (d *Directory) PublishSnapshotFile(ctx context.Context, name, sourcePath, expectedSHA256 string, limit int64) (int64, bool, error) {
	if !validArtifactName(name) || !validSHA256(expectedSHA256) {
		return 0, false, fmt.Errorf("synccarrier: invalid snapshot name or digest")
	}
	if err := d.Initialize(ctx); err != nil {
		return 0, false, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return 0, false, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > MaxSnapshotTransferBytes {
		return 0, false, fmt.Errorf("%w: snapshot source is not a bounded regular file", ErrTooLarge)
	}
	target := d.artifactPath(d.namespace, ClassSnapshot, name)
	if digest, size, err := hashFile(target); err == nil && size == info.Size() && digest == expectedSHA256 {
		return size, true, nil
	}
	partial := filepath.Join(d.namespacePath(d.namespace), stagingDir, name+".snapshot.partial")
	if err := os.MkdirAll(filepath.Dir(partial), 0o700); err != nil {
		return 0, false, err
	}
	existing, err := verifiedPrefix(source, partial, info.Size())
	if err != nil {
		return 0, false, err
	}
	if _, err := source.Seek(existing, io.SeekStart); err != nil {
		return existing, false, err
	}
	output, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return existing, false, err
	}
	if _, err := output.Seek(existing, io.SeekStart); err != nil {
		output.Close()
		return existing, false, err
	}
	remaining := info.Size() - existing
	if limit > 0 && limit < remaining {
		remaining = limit
	}
	written, copyErr := io.CopyN(output, source, remaining)
	if copyErr == nil {
		copyErr = output.Sync()
	}
	if closeErr := output.Close(); copyErr == nil {
		copyErr = closeErr
	}
	position := existing + written
	if copyErr != nil {
		return position, false, copyErr
	}
	if position < info.Size() {
		return position, false, nil
	}
	digest, size, err := hashFile(partial)
	if err != nil || size != info.Size() || digest != expectedSHA256 {
		return position, false, fmt.Errorf("%w: completed snapshot digest mismatch", ErrUnreadable)
	}
	if err := d.rename(partial, target); err != nil {
		return position, false, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	return position, true, nil
}

// DownloadSnapshotFile range-copies one encrypted snapshot from a peer
// namespace into a local file. The destination length is the durable resume
// checkpoint, matching REST's Range downloader. Finality is the declared size
// plus SHA-256, not a filesystem timestamp.
func (d *Directory) DownloadSnapshotFile(ctx context.Context, namespace, name, expectedSHA256, destination string, expectedBytes, limit int64) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	if !validBlindName(namespace) || !validArtifactName(name) || !validSHA256(expectedSHA256) || expectedBytes <= 0 || expectedBytes > MaxSnapshotTransferBytes {
		return 0, false, ErrUnreadable
	}
	source, err := os.Open(d.artifactPath(namespace, ClassSnapshot, name))
	if err != nil {
		return 0, false, ErrUnreadable
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != expectedBytes {
		return 0, false, ErrUnreadable
	}
	existing := int64(0)
	if local, err := os.Stat(destination); err == nil {
		existing = local.Size()
	}
	if existing > expectedBytes {
		return 0, false, ErrUnreadable
	}
	if existing == expectedBytes {
		digest, _, err := hashFile(destination)
		return existing, err == nil && digest == expectedSHA256, err
	}
	if _, err := source.Seek(existing, io.SeekStart); err != nil {
		return existing, false, err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return existing, false, err
	}
	if _, err := output.Seek(existing, io.SeekStart); err != nil {
		output.Close()
		return existing, false, err
	}
	remaining := expectedBytes - existing
	if limit > 0 && limit < remaining {
		remaining = limit
	}
	written, copyErr := io.CopyN(output, source, remaining)
	if copyErr == nil {
		copyErr = output.Sync()
	}
	if closeErr := output.Close(); copyErr == nil {
		copyErr = closeErr
	}
	position := existing + written
	if copyErr != nil || position < expectedBytes {
		return position, false, copyErr
	}
	digest, _, err := hashFile(destination)
	if err != nil || digest != expectedSHA256 {
		return position, false, fmt.Errorf("%w: downloaded snapshot digest mismatch", ErrUnreadable)
	}
	return position, true, nil
}

func verifiedPrefix(source *os.File, partial string, sourceBytes int64) (int64, error) {
	staged, err := os.OpenFile(partial, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return 0, err
	}
	defer staged.Close()
	info, err := staged.Stat()
	if err != nil {
		return 0, err
	}
	if info.Size() > sourceBytes {
		if err := staged.Truncate(0); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	left := make([]byte, 64<<10)
	right := make([]byte, len(left))
	var checked int64
	for checked < info.Size() {
		want := int64(len(left))
		if info.Size()-checked < want {
			want = info.Size() - checked
		}
		if _, err := io.ReadFull(source, left[:want]); err != nil {
			return 0, err
		}
		if _, err := io.ReadFull(staged, right[:want]); err != nil {
			return 0, err
		}
		if !bytes.Equal(left[:want], right[:want]) {
			if err := staged.Truncate(0); err != nil {
				return 0, err
			}
			return 0, nil
		}
		checked += want
	}
	return checked, nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}
	digest := sha256.New()
	if _, err := io.CopyBuffer(digest, file, make([]byte, 64<<10)); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(digest.Sum(nil)), info.Size(), nil
}

func validSHA256(value string) bool {
	if len(value) != 2*sha256.Size {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
