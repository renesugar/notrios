package synccarrier

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

type snapshotPrefix struct {
	sourcePath     string
	expectedSHA256 string
	sourceBytes    int64
	position       int64
}

// PublishSnapshotFile copies a locally produced encrypted snapshot into this
// replica's snapshot namespace. A partial private staging file resumes only
// after its prefix is compared byte-for-byte with the immutable source.
// `limit` bounds one call; zero means copy to completion.
func (d *Directory) PublishSnapshotFile(ctx context.Context, name, sourcePath, expectedSHA256 string, limit int64) (int64, bool, error) {
	d.snapshotMu.Lock()
	defer d.snapshotMu.Unlock()
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
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return 0, false, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	defer root.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > MaxSnapshotTransferBytes {
		return 0, false, fmt.Errorf("%w: snapshot source is not a bounded regular file", ErrTooLarge)
	}
	target := d.artifactPath(d.namespace, ClassSnapshot, name)
	if digest, size, err := hashRootFile(root, d.rel(target), MaxSnapshotTransferBytes); err == nil && size == info.Size() && digest == expectedSHA256 {
		return size, true, nil
	}
	partial := filepath.Join(d.namespacePath(d.namespace), stagingDir, name+".snapshot.partial")
	if err := rejectSymlinkAncestors(root, d.rel(filepath.Dir(partial))); err != nil {
		return 0, false, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if err := root.MkdirAll(d.rel(filepath.Dir(partial)), 0o700); err != nil {
		return 0, false, err
	}
	existing := int64(-1)
	if staged, statErr := root.Lstat(d.rel(partial)); statErr == nil && staged.Mode().IsRegular() && staged.Size() <= MaxSnapshotTransferBytes {
		cached, found := d.snapshotPrefixes[partial]
		if found && cached.sourcePath == sourcePath && cached.expectedSHA256 == expectedSHA256 &&
			cached.sourceBytes == info.Size() && cached.position == staged.Size() {
			existing = cached.position
		}
	}
	if existing < 0 {
		existing, err = d.verifySnapshotPrefix(root, source, d.rel(partial), info.Size())
		if err != nil {
			delete(d.snapshotPrefixes, partial)
			return 0, false, err
		}
		d.snapshotPrefixes[partial] = snapshotPrefix{sourcePath, expectedSHA256, info.Size(), existing}
	}
	if _, err := source.Seek(existing, io.SeekStart); err != nil {
		return existing, false, err
	}
	output, err := openOrCreateBoundedRegular(root, d.rel(partial), MaxSnapshotTransferBytes, 0o600)
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
		delete(d.snapshotPrefixes, partial)
		return position, false, copyErr
	}
	d.snapshotPrefixes[partial] = snapshotPrefix{sourcePath, expectedSHA256, info.Size(), position}
	if position < info.Size() {
		return position, false, nil
	}
	digest, size, err := hashRootFile(root, d.rel(partial), MaxSnapshotTransferBytes)
	if err != nil || size != info.Size() || digest != expectedSHA256 {
		delete(d.snapshotPrefixes, partial)
		return position, false, fmt.Errorf("%w: completed snapshot digest mismatch", ErrUnreadable)
	}
	var renameErr error
	if err := rejectSymlinkAncestors(root, d.rel(filepath.Dir(target))); err != nil {
		return position, false, fmt.Errorf("%w: %v", ErrCarrierUnavailable, err)
	}
	if d.rename != nil {
		renameErr = d.rename(partial, target)
	} else {
		renameErr = root.Rename(d.rel(partial), d.rel(target))
	}
	if renameErr != nil {
		delete(d.snapshotPrefixes, partial)
		return position, false, fmt.Errorf("%w: %v", ErrCarrierUnavailable, renameErr)
	}
	delete(d.snapshotPrefixes, partial)
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
	root, err := os.OpenRoot(d.root)
	if err != nil {
		return 0, false, ErrUnreadable
	}
	defer root.Close()
	source, err := openBoundedRegular(root, d.rel(d.artifactPath(namespace, ClassSnapshot, name)), MaxSnapshotTransferBytes)
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

func verifiedPrefixRoot(root *os.Root, source *os.File, partial string, sourceBytes int64) (int64, error) {
	if existing, err := root.Lstat(partial); err == nil && !existing.Mode().IsRegular() {
		return 0, ErrUnreadable
	}
	staged, err := openOrCreateBoundedRegular(root, partial, MaxSnapshotTransferBytes, 0o600)
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

func hashRootFile(root *os.Root, path string, maxBytes int64) (string, int64, error) {
	file, err := openBoundedRegular(root, path, maxBytes)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	digest := sha256.New()
	written, err := io.CopyBuffer(digest, io.LimitReader(file, maxBytes+1), make([]byte, 64<<10))
	if err != nil {
		return "", 0, err
	}
	if written > maxBytes {
		return "", 0, ErrTooLarge
	}
	return hex.EncodeToString(digest.Sum(nil)), written, nil
}

// openOrCreateBoundedRegular obtains a root-anchored writable descriptor
// without ever following a final symlink. An existing object must pass the
// same Lstat/open/fstat identity check as an untrusted read.
func openOrCreateBoundedRegular(root *os.Root, path string, maxBytes int64, perm os.FileMode) (*os.File, error) {
	if err := rejectSymlinkAncestors(root, path); err != nil {
		return nil, ErrUnreadable
	}
	lstat, err := root.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, perm)
		if createErr != nil {
			return nil, ErrUnreadable
		}
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() || !singleLink(file, info) {
			file.Close()
			return nil, ErrUnreadable
		}
		return file, nil
	}
	if err != nil || !lstat.Mode().IsRegular() || lstat.Mode()&os.ModeSymlink != 0 || lstat.Size() > maxBytes {
		return nil, ErrUnreadable
	}
	file, err := root.OpenFile(path, os.O_RDWR, perm)
	if err != nil {
		return nil, ErrUnreadable
	}
	fstat, err := file.Stat()
	if err != nil || !fstat.Mode().IsRegular() || !os.SameFile(lstat, fstat) || fstat.Size() > maxBytes || !singleLink(file, fstat) {
		file.Close()
		return nil, ErrUnreadable
	}
	return file, nil
}

func validSHA256(value string) bool {
	if len(value) != 2*sha256.Size {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
