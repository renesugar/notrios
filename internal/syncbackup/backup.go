// Package syncbackup wraps and encrypts a physical snapshot for transfer, and
// opens one again.
//
// G14d replaces the loose-object stored-ZIP path with G14c's database image and
// bounded asset packs. USTAR is only a sequential transport wrapper around
// those few physical files; snapshotimage's exact-schema verifier remains the
// trust boundary. Nothing requires a seekable multi-gigabyte buffer: the
// payload is sealed in fixed frames and both directions stream, so memory is
// one frame regardless of library size.
package syncbackup

import (
	"archive/tar"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// FrameBytes is the plaintext size of one sealed frame. It is the memory
	// bound on both ends and the granularity a resumed download restarts at.
	FrameBytes = 1 << 20
	// MaxFrames bounds a backup at 16 GiB of plaintext, matching the resource
	// ceiling G2 fixed.
	MaxFrames = 16 * 1024
	// KeyBytes is the payload key size.
	KeyBytes = 32
	// Magic identifies a sealed backup so a decoder refuses the wrong bytes
	// rather than misreading them.
	Magic = "NBK1"
	// domain separates frame authentication from every other AEAD use.
	domain = "notrios.backup-frame.v1"
	// MaxEntryBytes bounds one packed file, so a hostile container cannot ask
	// for an unbounded allocation while being extracted.
	MaxEntryBytes = int64(16) << 30
)

var (
	// ErrMalformed reports bytes that are not a sealed backup.
	ErrMalformed = errors.New("malformed sealed backup")
	// ErrFrame reports a frame that did not authenticate.
	ErrFrame = errors.New("sealed backup frame did not authenticate")
	// ErrUnsafeEntry reports a container entry whose name would escape the
	// destination. It is checked on extraction, not on packing, because the
	// container may not be one we produced.
	ErrUnsafeEntry = errors.New("backup entry name is not safe to extract")
)

// Pack writes a directory into a deterministic USTAR stream.
//
// Ordering matters only so two packs of one archive produce the same bytes,
// which makes a transfer's content hash reproducible; correctness still comes
// from the physical snapshot verifier after extraction.
func Pack(root string, out io.Writer) (int64, error) {
	writer := tar.NewWriter(out)
	var total int64
	var names []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("%w: %s is not a regular file", ErrUnsafeEntry, path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(relative))
		return nil
	}); err != nil {
		return 0, err
	}
	sortStrings(names)
	for _, name := range names {
		contents, err := os.Open(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return 0, err
		}
		info, err := contents.Stat()
		if err != nil {
			contents.Close()
			return 0, err
		}
		header := &tar.Header{
			Name: name, Mode: 0o600, Size: info.Size(), Typeflag: tar.TypeReg,
			ModTime: time.Unix(0, 0).UTC(), Format: tar.FormatUSTAR,
		}
		if err := writer.WriteHeader(header); err != nil {
			contents.Close()
			return 0, err
		}
		written, err := io.Copy(writer, contents)
		contents.Close()
		if err != nil {
			return 0, err
		}
		total += written
	}
	return total, writer.Close()
}

// Unpack extracts a deterministic USTAR stream into a directory, refusing
// links, duplicate paths, and any entry whose name would escape it.
func Unpack(path, destination string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	defer file.Close()
	reader := tar.NewReader(file)
	seen := map[string]bool{}
	for {
		entry, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		if entry.Typeflag != tar.TypeReg || entry.Linkname != "" || entry.Size < 0 || entry.Size > MaxEntryBytes {
			return fmt.Errorf("%w: entry %q is not a bounded regular file", ErrMalformed, entry.Name)
		}
		target, err := safeJoin(destination, entry.Name)
		if err != nil {
			return err
		}
		if seen[target] {
			return fmt.Errorf("%w: duplicate entry %q", ErrMalformed, entry.Name)
		}
		seen[target] = true
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(output, io.LimitReader(reader, entry.Size+1))
		if copyErr == nil && written != entry.Size {
			copyErr = fmt.Errorf("%w: entry %q length changed", ErrMalformed, entry.Name)
		}
		if closeErr := output.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr != nil {
			return copyErr
		}
	}
}

// safeJoin refuses absolute paths, parent traversal, and anything that resolves
// outside the destination. A container arrives from a peer, and a peer is
// authenticated rather than trusted.
func safeJoin(destination, name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrUnsafeEntry, name)
	}
	target := filepath.Join(destination, cleaned)
	relative, err := filepath.Rel(destination, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrUnsafeEntry, name)
	}
	return target, nil
}

// Seal encrypts a stream into frames and returns the sealed length and the
// SHA-256 of the sealed bytes.
//
// Each frame's nonce is its index, which is safe because the key is fresh for
// one backup and an index never repeats. The index is also authenticated, so
// frames cannot be reordered, dropped, or duplicated by whoever moves them.
func Seal(source io.Reader, key []byte, out io.Writer) (int64, string, error) {
	aead, err := frameAEAD(key)
	if err != nil {
		return 0, "", err
	}
	digest := sha256.New()
	writer := io.MultiWriter(out, digest)
	if _, err := writer.Write([]byte(Magic)); err != nil {
		return 0, "", err
	}
	total := int64(len(Magic))
	plaintext := make([]byte, FrameBytes)
	for index := 0; ; index++ {
		if index > MaxFrames {
			return 0, "", fmt.Errorf("%w: more than %d frames", ErrMalformed, MaxFrames)
		}
		read, err := io.ReadFull(source, plaintext)
		if read == 0 && (err == io.EOF || err == io.ErrUnexpectedEOF) {
			break
		}
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return 0, "", err
		}
		sealed := aead.Seal(nil, frameNonce(aead, index), plaintext[:read], frameAAD(index, read < FrameBytes))
		var header [8]byte
		binary.BigEndian.PutUint32(header[0:4], uint32(len(sealed)))
		binary.BigEndian.PutUint32(header[4:8], uint32(index))
		if _, err := writer.Write(header[:]); err != nil {
			return 0, "", err
		}
		if _, err := writer.Write(sealed); err != nil {
			return 0, "", err
		}
		total += int64(len(header) + len(sealed))
		if read < FrameBytes {
			break
		}
	}
	return total, hex.EncodeToString(digest.Sum(nil)), nil
}

// Open reverses Seal, streaming, one frame at a time.
func Open(source io.Reader, key []byte, out io.Writer) error {
	aead, err := frameAEAD(key)
	if err != nil {
		return err
	}
	magic := make([]byte, len(Magic))
	if _, err := io.ReadFull(source, magic); err != nil || string(magic) != Magic {
		return fmt.Errorf("%w: not a %s backup", ErrMalformed, Magic)
	}
	for index := 0; ; index++ {
		var header [8]byte
		if _, err := io.ReadFull(source, header[:]); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		length := binary.BigEndian.Uint32(header[0:4])
		declared := int(binary.BigEndian.Uint32(header[4:8]))
		if declared != index {
			return fmt.Errorf("%w: frame %d declares index %d", ErrFrame, index, declared)
		}
		if length > uint32(FrameBytes+aead.Overhead()) {
			return fmt.Errorf("%w: frame %d is %d bytes", ErrMalformed, index, length)
		}
		sealed := make([]byte, length)
		if _, err := io.ReadFull(source, sealed); err != nil {
			return fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		final := int(length)-aead.Overhead() < FrameBytes
		plaintext, err := aead.Open(nil, frameNonce(aead, index), sealed, frameAAD(index, final))
		if err != nil {
			return fmt.Errorf("%w: frame %d", ErrFrame, index)
		}
		if _, err := out.Write(plaintext); err != nil {
			return err
		}
		if final {
			return nil
		}
	}
}

func frameAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != KeyBytes {
		return nil, fmt.Errorf("%w: a payload key is %d bytes", ErrMalformed, KeyBytes)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func frameNonce(aead cipher.AEAD, index int) []byte {
	nonce := make([]byte, aead.NonceSize())
	binary.BigEndian.PutUint64(nonce[len(nonce)-8:], uint64(index))
	return nonce
}

// frameAAD authenticates the frame's index and whether it is the last one, so a
// truncated transfer cannot pass as a complete backup.
func frameAAD(index int, final bool) []byte {
	marker := byte(0)
	if final {
		marker = 1
	}
	return append([]byte(fmt.Sprintf("%s|%d|", domain, index)), marker)
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
