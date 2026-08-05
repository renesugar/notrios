package archivev2

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// PackMediaType marks a pack container object.
const PackMediaType = "application/vnd.notrios.archive-v2-pack"

// packMagic ends every pack file. A pack is therefore self-describing: its
// trailer can be found and read without the archive index, which is what keeps
// a pack verifiable on its own.
var packMagic = [16]byte{'n', 'o', 't', 'r', 'i', 'o', 's', '-', 'p', 'a', 'c', 'k', '-', 'v', '1', '\n'}

// packFooterBytes is the fixed footer: trailer offset, trailer length, magic.
const packFooterBytes = 8 + 8 + 16

// packTrailerEntry records where one object lives inside its pack.
type packTrailerEntry struct {
	SHA256    string `json:"sha256"`
	Offset    int64  `json:"offset"`
	Length    int64  `json:"length"`
	Kind      string `json:"kind"`
	MediaType string `json:"media_type"`
	Records   int    `json:"records,omitempty"`
	// Pointer for the same reason as IndexEntry.RecordCounts: a struct value
	// is never omitted, so blob entries carried twelve zeroes each.
	RecordCounts *Counts `json:"record_counts,omitempty"`
}

// packWriter concatenates objects into large sequential files. It exists
// because one file per object made throughput track filesystem operations
// rather than bytes: the real 382,206-note corpus needed 382,407
// create+fsync+rename cycles to store 1.14 GB.
type packWriter struct {
	root        string
	staging     string
	limits      Limits
	targetBytes int64
	maxObjects  int

	file     *os.File
	buffered *bufio.Writer
	hasher   hashWriter
	offset   int64
	entries  []packTrailerEntry
	sequence int

	// emit receives every index entry once its pack hash is known.
	emit func(IndexEntry) error
	// bytesWritten counts bytes actually placed on disk.
	bytesWritten int64
	packs        int
}

type hashWriter interface {
	io.Writer
	Sum(b []byte) []byte
	Reset()
}

func newPackWriter(root, staging string, limits Limits, targetBytes int64, maxObjects int, emit func(IndexEntry) error) *packWriter {
	if targetBytes <= 0 {
		targetBytes = DefaultPackTargetBytes
	}
	if maxObjects <= 0 {
		maxObjects = DefaultPackMaxObjects
	}
	return &packWriter{root: root, staging: staging, limits: limits, targetBytes: targetBytes, maxObjects: maxObjects, emit: emit}
}

// DefaultPackTargetBytes and DefaultPackMaxObjects bound one pack so a trailer
// stays small enough to hold while writing and a pack stays small enough to
// re-fetch.
const (
	DefaultPackTargetBytes = 256 << 20
	DefaultPackMaxObjects  = 65536
)

// add copies one object's bytes into the open pack. The object's own SHA-256
// is computed here, so identity never depends on placement. Index entries are
// emitted only when the pack is finalized, because a packed entry names the
// pack that contains it and that hash is not known until the pack is closed.
func (w *packWriter) add(content io.Reader, kind, mediaType string, describe func(*packTrailerEntry)) (string, int64, error) {
	if w.file == nil {
		if err := w.open(); err != nil {
			return "", 0, err
		}
	}
	start := w.offset
	objectDigest := sha256.New()
	size, err := io.CopyBuffer(io.MultiWriter(w.buffered, w.hasher, objectDigest), content, make([]byte, copyBufferBytes))
	if err != nil {
		return "", 0, err
	}
	w.offset += size
	hash := hex.EncodeToString(objectDigest.Sum(nil))
	if size > w.limits.MaxBlobBytes {
		return "", 0, fmt.Errorf("object %s exceeds the blob byte limit", hash)
	}
	entry := packTrailerEntry{SHA256: hash, Offset: start, Length: size, Kind: kind, MediaType: mediaType}
	if describe != nil {
		describe(&entry)
	}
	w.entries = append(w.entries, entry)
	if w.offset >= w.targetBytes || len(w.entries) >= w.maxObjects {
		if err := w.finalize(); err != nil {
			return "", 0, err
		}
	}
	return hash, size, nil
}

func (w *packWriter) open() error {
	w.sequence++
	path := filepath.Join(w.staging, "pack-"+strconv.Itoa(w.sequence))
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	w.file = file
	w.buffered = bufio.NewWriterSize(file, copyBufferBytes)
	w.hasher = sha256.New()
	w.offset = 0
	w.entries = w.entries[:0]
	return nil
}

// finalize appends the trailer and footer, publishes the pack at its
// content-addressed path, and records its index entry.
func (w *packWriter) finalize() error {
	if w.file == nil {
		return nil
	}
	trailerOffset := w.offset
	var trailer []byte
	for _, entry := range w.entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		trailer = append(trailer, line...)
		trailer = append(trailer, '\n')
	}
	if _, err := w.buffered.Write(trailer); err != nil {
		return err
	}
	if _, err := w.hasher.Write(trailer); err != nil {
		return err
	}
	footer := make([]byte, packFooterBytes)
	binary.BigEndian.PutUint64(footer[0:8], uint64(trailerOffset))
	binary.BigEndian.PutUint64(footer[8:16], uint64(len(trailer)))
	copy(footer[16:], packMagic[:])
	if _, err := w.buffered.Write(footer); err != nil {
		return err
	}
	if _, err := w.hasher.Write(footer); err != nil {
		return err
	}
	if err := w.buffered.Flush(); err != nil {
		return err
	}
	if err := w.file.Sync(); err != nil {
		return err
	}
	temporary := w.file.Name()
	if err := w.file.Close(); err != nil {
		return err
	}
	w.file = nil

	size := trailerOffset + int64(len(trailer)) + packFooterBytes
	hash := hex.EncodeToString(w.hasher.Sum(nil))
	location := newFanoutLocation(hash)
	final := filepath.Join(w.root, filepath.FromSlash(location.Path))
	if info, statErr := os.Lstat(final); statErr == nil && info.Mode().IsRegular() && info.Size() == size {
		_ = os.Remove(temporary)
	} else {
		if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
			return err
		}
		if err := os.Rename(temporary, final); err != nil {
			return err
		}
		w.bytesWritten += size
	}
	for _, contained := range w.entries {
		if err := w.emit(IndexEntry{
			SHA256: contained.SHA256, Kind: contained.Kind, MediaType: contained.MediaType,
			SizeBytes: contained.Length, Records: contained.Records, RecordCounts: contained.RecordCounts,
			Location: ObjectLocation{Layout: LayoutPack, PackSHA256: hash, Offset: contained.Offset, Length: contained.Length},
		}); err != nil {
			return err
		}
	}
	if err := w.emit(IndexEntry{
		SHA256: hash, Kind: "pack", MediaType: PackMediaType, SizeBytes: size, Location: location,
	}); err != nil {
		return err
	}
	w.packs++
	w.entries = nil
	return nil
}

func (w *packWriter) close() error {
	if w.file == nil {
		return nil
	}
	name := w.file.Name()
	err := w.file.Close()
	w.file = nil
	_ = os.Remove(name)
	return err
}

// readPackFooter locates a pack's trailer without consulting the archive
// index, which is what makes a pack independently verifiable.
func readPackFooter(file *os.File, size int64) (trailerOffset, trailerLength int64, err error) {
	if size < packFooterBytes {
		return 0, 0, fmt.Errorf("pack is smaller than its footer")
	}
	footer := make([]byte, packFooterBytes)
	if _, err := file.ReadAt(footer, size-packFooterBytes); err != nil {
		return 0, 0, err
	}
	if string(footer[16:]) != string(packMagic[:]) {
		return 0, 0, fmt.Errorf("pack footer magic is absent")
	}
	trailerOffset = int64(binary.BigEndian.Uint64(footer[0:8]))
	trailerLength = int64(binary.BigEndian.Uint64(footer[8:16]))
	if trailerOffset < 0 || trailerLength < 0 || trailerOffset+trailerLength+packFooterBytes != size {
		return 0, 0, fmt.Errorf("pack trailer bounds are inconsistent")
	}
	return trailerOffset, trailerLength, nil
}

// readPackTrailer decodes a pack's self-describing contents.
func readPackTrailer(file *os.File, limits Limits) ([]packTrailerEntry, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	trailerOffset, trailerLength, err := readPackFooter(file, stat.Size())
	if err != nil {
		return nil, err
	}
	if trailerLength > limits.MaxIndexObjectBytes {
		return nil, fmt.Errorf("pack trailer exceeds %d bytes", limits.MaxIndexObjectBytes)
	}
	raw := make([]byte, trailerLength)
	if _, err := file.ReadAt(raw, trailerOffset); err != nil && err != io.EOF {
		return nil, err
	}
	entries := []packTrailerEntry{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 16<<10), limits.MaxIndexEntryBytes)
	for scanner.Scan() {
		var entry packTrailerEntry
		if err := decodeStrict(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("pack trailer entry: %w", err)
		}
		if !validSHA256(entry.SHA256) || entry.Offset < 0 || entry.Length < 0 || entry.Offset+entry.Length > trailerOffset {
			return nil, fmt.Errorf("pack trailer entry %q is out of bounds", entry.SHA256)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// packSource reads objects from either layout during verification and restore.
// Pack files are few and large, so their handles are cached; a loose object is
// opened and closed per read exactly as before.
type packSource struct {
	limits Limits
	open_  map[string]*packHandle
	clock  uint64
}

// packHandle is one cached pack file. borrowed counts the readers currently
// holding a section over it: a borrowed pack must never be evicted, because
// closing it would invalidate a live reader rather than merely cost a reopen.
type packHandle struct {
	file     *os.File
	borrowed int
	used     uint64
}

func newPackSource(limits Limits) *packSource {
	return &packSource{limits: limits, open_: map[string]*packHandle{}}
}

func (s *packSource) close() {
	for hash, handle := range s.open_ {
		_ = handle.file.Close()
		delete(s.open_, hash)
	}
}

// acquire returns a cached handle for one pack and marks it in use. The caller
// must release it.
//
// Restore reads a record object and, for each record inside it, opens the blob
// that record names. Those blobs live in other packs, so a bounded cache that
// evicted by arbitrary choice could close the record object's own pack while
// its reader was still mid-scan. That is not a performance problem but a
// correctness one, and it only appears once an archive holds more packs than
// the cache — which no small fixture does.
func (s *packSource) acquire(root, hash string) (*packHandle, error) {
	s.clock++
	if handle, ok := s.open_[hash]; ok {
		handle.used = s.clock
		handle.borrowed++
		return handle, nil
	}
	s.evictIdle()
	file, err := openRegular(root, fanoutObjectPath(hash))
	if err != nil {
		return nil, fmt.Errorf("pack %s: %w", hash, err)
	}
	handle := &packHandle{file: file, borrowed: 1, used: s.clock}
	s.open_[hash] = handle
	return handle, nil
}

func (s *packSource) release(handle *packHandle) {
	if handle != nil && handle.borrowed > 0 {
		handle.borrowed--
	}
}

// evictIdle closes the least recently used idle handle to make room. If every
// cached handle is borrowed the cache is allowed to exceed its bound: the
// excess is bounded by how many readers are open at once, which is small, and
// keeping a live reader valid matters more than the handle ceiling.
func (s *packSource) evictIdle() {
	if len(s.open_) < maxOpenPacks {
		return
	}
	oldest, found := "", uint64(0)
	for hash, handle := range s.open_ {
		if handle.borrowed > 0 {
			continue
		}
		if found == 0 || handle.used < found {
			oldest, found = hash, handle.used
		}
	}
	if found == 0 {
		return
	}
	_ = s.open_[oldest].file.Close()
	delete(s.open_, oldest)
}

// maxOpenPacks bounds cached pack handles. Packs are hundreds of megabytes, so
// even a very large archive has few.
const maxOpenPacks = 16

// open returns a reader over one object's bytes regardless of layout. The
// returned closer must be called: for a packed object it releases the pack
// handle back to the cache.
func (s *packSource) open(root string, entry IndexEntry) (io.ReaderAt, func(), error) {
	if entry.Location.Layout != LayoutPack {
		file, err := openRegular(root, entry.Location.Path)
		if err != nil {
			return nil, func() {}, err
		}
		return file, func() { _ = file.Close() }, nil
	}
	handle, err := s.acquire(root, entry.Location.PackSHA256)
	if err != nil {
		return nil, func() {}, err
	}
	// The handle stays cached; the section is a bounded view into it.
	return io.NewSectionReader(handle.file, entry.Location.Offset, entry.Location.Length),
		func() { s.release(handle) }, nil
}

// verifySlice confirms a packed object's bytes hash to the identity its index
// entry claims, so placement can never launder content.
func (s *packSource) verifySlice(root string, entry IndexEntry) error {
	handle, err := s.acquire(root, entry.Location.PackSHA256)
	if err != nil {
		return err
	}
	defer s.release(handle)
	section := io.NewSectionReader(handle.file, entry.Location.Offset, entry.Location.Length)
	digest := sha256.New()
	written, err := io.CopyBuffer(digest, section, make([]byte, copyBufferBytes))
	if err != nil || written != entry.SizeBytes || hex.EncodeToString(digest.Sum(nil)) != entry.SHA256 {
		return fmt.Errorf("packed object %s checksum mismatch", entry.SHA256)
	}
	return nil
}

// verifyContents checks a pack's own trailer against the pack it describes.
// The trailer is what makes a pack independently verifiable, so it must agree
// with the bytes it sits behind.
func (s *packSource) verifyContents(root string, entry IndexEntry) error {
	handle, err := s.acquire(root, entry.SHA256)
	if err != nil {
		return err
	}
	defer s.release(handle)
	entries, err := readPackTrailer(handle.file, s.limits)
	if err != nil {
		return fmt.Errorf("pack %s: %w", entry.SHA256, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("pack %s contains no objects", entry.SHA256)
	}
	if len(entries) > s.limits.MaxObjects {
		return fmt.Errorf("pack %s declares more objects than the archive admits", entry.SHA256)
	}
	return nil
}

// requireTrailingLFAt enforces the JSONL terminator through a ReaderAt so a
// record chunk is checked identically whether it is loose or packed.
func requireTrailingLFAt(reader io.ReaderAt, size int64, hash string) error {
	if size == 0 {
		return fmt.Errorf("object %s is empty", hash)
	}
	last := []byte{0}
	if _, err := reader.ReadAt(last, size-1); err != nil || last[0] != '\n' {
		return fmt.Errorf("object %s must end with LF", hash)
	}
	return nil
}
