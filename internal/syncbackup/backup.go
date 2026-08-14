// Package syncbackup packages and encrypts an archive-v2 snapshot for transfer,
// and opens one again.
//
// Two rules from v0.7's resolved decisions shape it. **ZIP may be the wrapper,
// but archive-v2 defines correctness**: the container here is a convenience for
// moving one file, and the restoring replica verifies the extracted archive
// with archive-v2's own verifier before anything canonical is written. ZIP
// central-directory parsing is never the trust boundary. And **nothing requires
// a seekable multi-gigabyte buffer**: the payload is sealed in fixed frames and
// both directions stream, so memory is one frame regardless of library size.
package syncbackup

import (
	"archive/zip"
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
	MaxEntryBytes = int64(8) << 30
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

// Pack writes a directory into a ZIP stream, deterministically ordered.
//
// Ordering matters only so two packs of one archive produce the same bytes,
// which makes a transfer's content hash reproducible; correctness still comes
// from archive-v2 after extraction.
func Pack(root string, out io.Writer) (int64, error) {
	writer := zip.NewWriter(out)
	var total int64
	var names []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
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
		// Stored rather than deflated: archive-v2 objects are already
		// content-addressed blobs, most of them incompressible, and spending
		// CPU to re-compress a gigabyte of hashes is not a trade worth making.
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			contents.Close()
			return 0, err
		}
		written, err := io.Copy(entry, contents)
		contents.Close()
		if err != nil {
			return 0, err
		}
		total += written
	}
	return total, writer.Close()
}

// Unpack extracts a ZIP into a directory, refusing any entry whose name would
// escape it.
func Unpack(path, destination string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	defer reader.Close()
	for _, entry := range reader.File {
		target, err := safeJoin(destination, entry.Name)
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if int64(entry.UncompressedSize64) > MaxEntryBytes {
			return fmt.Errorf("%w: entry %q declares %d bytes", ErrMalformed, entry.Name, entry.UncompressedSize64)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(file, io.LimitReader(source, MaxEntryBytes+1))
		source.Close()
		if closeErr := file.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
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
