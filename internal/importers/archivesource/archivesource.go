// Package archivesource reads a downloaded export archive in place.
//
// An export arrives as a ZIP: Twitter/X, ChatGPT, the OpenAI Privacy Portal,
// Claude. A user should not have to unzip it, and an importer should not have
// to extract it to read it, so a Source reads entries out of the ZIP where they
// lie, or out of an already extracted folder, behind one interface. The Privacy
// Portal nests a ZIP inside the ZIP; Nested reads that in place too.
//
// An archive is untrusted input. Names that escape it are refused, sizes are
// bounded, and nothing is ever written to a path taken from the archive.
//
// It began as the Twitter/X importer's own reader (J25) and was made shared by
// J26, when three more archive formats needed the same thing.
package archivesource

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/tempspace"
)

// Limits bound what an import will read. They are a variable so tests can lower
// them; nothing else changes them.
var Limits = struct {
	// Entries bounds a ZIP's directory. The owner's 3.3 GB Twitter/X archive
	// has 15,088 entries.
	Entries int
	// DataFileBytes bounds one decompressed data file. A Twitter/X post file
	// is about 105 MB, and a Privacy Portal conversations shard about 60 MB.
	DataFileBytes int64
	// MediaFileBytes bounds one decompressed media file.
	MediaFileBytes int64
	// NestedArchiveBytes bounds a ZIP stored inside a ZIP. The Privacy
	// Portal's conversations ZIP is about 139 MB.
	NestedArchiveBytes int64
	// NestedInMemoryBytes is the largest nested ZIP read into memory when no
	// instance temp space is available to spool it to.
	NestedInMemoryBytes int64
}{
	Entries:             1_000_000,
	DataFileBytes:       1 << 30,
	MediaFileBytes:      4 << 30,
	NestedArchiveBytes:  8 << 30,
	NestedInMemoryBytes: 256 << 20,
}

// ErrTooLarge is returned when a file or archive exceeds a bound above.
var ErrTooLarge = errors.New("exceeds the import size limit")

// Kind says what a Source is reading.
const (
	KindZip       = "zip"
	KindDirectory = "directory"
)

// Source is an archive's data directory: an extracted folder, or a ZIP read in
// place. Names are slash-separated and relative to that directory.
type Source interface {
	// Open opens one file, refusing it if it is larger than limit.
	Open(name string, limit int64) (io.ReadCloser, error)
	// List names the regular files directly inside dir ("" is the data
	// directory itself), sorted.
	List(dir string) ([]string, error)
	// All names every regular file at or below the data directory, as
	// slash-separated paths relative to it, sorted. An archive that keeps some
	// of its assets in subdirectories is indexed with this.
	All() ([]string, error)
	// Kind is KindZip or KindDirectory.
	Kind() string
	Close() error
}

// Spec is what one archive format knows about itself: which files mark its data
// directory, where to look for that directory in an extracted folder, and how
// to say that this is not one of its archives. The messages belong to the
// importer because a user reads them.
type Spec struct {
	// IsDataFile reports whether a base name marks the data directory, such as
	// "tweets.js" or "conversations.json".
	IsDataFile func(base string) bool
	// DirCandidates are the subdirectories tried in an extracted folder, in
	// order. Empty means {"data", ""}: data/ under the folder, then the folder.
	DirCandidates []string
	// NotAnArchive explains that a file is not one of this format's ZIPs.
	NotAnArchive func(name string, err error) error
	// NoDataDirectory explains that no data file was found in a ZIP or folder.
	NoDataDirectory func(where, kind string) error
}

func (s Spec) candidates() []string {
	if len(s.DirCandidates) > 0 {
		return s.DirCandidates
	}
	return []string{"data", ""}
}

func (s Spec) notAnArchive(name string, err error) error {
	if s.NotAnArchive != nil {
		return s.NotAnArchive(name, err)
	}
	return fmt.Errorf("%s is neither an archive ZIP nor an extracted archive folder: %w", name, err)
}

func (s Spec) noDataDirectory(where, kind string) error {
	if s.NoDataDirectory != nil {
		return s.NoDataDirectory(where, kind)
	}
	return fmt.Errorf("no data file found in %s", where)
}

// Open reads sourcePath as an extracted archive folder or as a ZIP, and returns
// how many ZIP entries were refused as unsafe.
func Open(sourcePath string, spec Spec) (Source, int, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, 0, err
	}
	if info.IsDir() {
		dataDir, err := findDataDir(sourcePath, spec)
		if err != nil {
			return nil, 0, err
		}
		return &dirSource{dataDir: dataDir}, 0, nil
	}
	reader, err := zip.OpenReader(sourcePath)
	// Since Go 1.20 a ZIP with a ".." or absolute entry name opens with
	// ErrInsecurePath and a usable reader. Such names are refused below, one by
	// one, rather than refusing the whole archive over them.
	if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && reader != nil) {
		return nil, 0, spec.notAnArchive(filepath.Base(sourcePath), err)
	}
	source, rejected, err := fromZip(&reader.Reader, reader, filepath.Base(sourcePath), spec)
	if err != nil {
		reader.Close()
		return nil, rejected, err
	}
	return source, rejected, nil
}

// Nested opens a ZIP held inside another archive, read in place: nothing is
// extracted from the outer archive to a path of its choosing.
//
// A ZIP is read with random access, which a compressed entry does not support,
// so the nested archive is spooled into the instance's own temp space (J22) and
// removed when the Source closes. With no instance temp space, as in library
// use and tests, a small nested archive is held in memory instead.
func Nested(space *tempspace.Space, outer Source, name string, spec Spec) (Source, int, error) {
	entry, err := outer.Open(name, Limits.NestedArchiveBytes)
	if err != nil {
		return nil, 0, err
	}
	defer entry.Close()

	if space == nil {
		body, err := io.ReadAll(io.LimitReader(entry, Limits.NestedInMemoryBytes+1))
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", name, err)
		}
		if int64(len(body)) > Limits.NestedInMemoryBytes {
			return nil, 0, fmt.Errorf("%s is larger than the %d bytes a nested archive may take in memory, and no instance temp directory is configured: %w",
				name, Limits.NestedInMemoryBytes, ErrTooLarge)
		}
		reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
			return nil, 0, spec.notAnArchive(path.Base(name), err)
		}
		return fromZip(reader, nil, path.Base(name), spec)
	}

	dir, err := space.MkdirTemp("notrios-import-nested-*")
	if err != nil {
		return nil, 0, err
	}
	spooled := filepath.Join(dir, "nested.zip")
	file, err := os.OpenFile(spooled, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		os.RemoveAll(dir)
		return nil, 0, err
	}
	_, copyErr := io.Copy(file, entry)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		os.RemoveAll(dir)
		return nil, 0, errors.Join(fmt.Errorf("%s: spool the nested archive", name), copyErr, closeErr)
	}
	reader, err := zip.OpenReader(spooled)
	if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && reader != nil) {
		os.RemoveAll(dir)
		return nil, 0, spec.notAnArchive(path.Base(name), err)
	}
	source, rejected, err := fromZip(&reader.Reader, reader, path.Base(name), spec)
	if err != nil {
		reader.Close()
		os.RemoveAll(dir)
		return nil, rejected, err
	}
	source.(*zipSource).spoolDir = dir
	return source, rejected, nil
}

// cleanRelative reports whether an archive name is safe to use as a lookup key:
// relative, with no parent segments, backslashes, drive letters or NULs. Names
// are never joined onto the filesystem for writing, but a name that escapes the
// archive is refused rather than trusted anywhere.
func cleanRelative(name string) bool {
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" || strings.HasPrefix(trimmed, "/") || strings.ContainsAny(trimmed, "\\:\x00") {
		return false
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

type dirSource struct {
	dataDir string
}

// findDataDir finds an extracted archive's data directory among the spec's
// candidate subdirectories.
func findDataDir(sourceDir string, spec Spec) (string, error) {
	for _, candidate := range spec.candidates() {
		dir := sourceDir
		if candidate != "" {
			dir = filepath.Join(sourceDir, candidate)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() && spec.IsDataFile(entry.Name()) {
				return dir, nil
			}
		}
	}
	return "", spec.noDataDirectory(sourceDir, KindDirectory)
}

func (d *dirSource) Open(name string, limit int64) (io.ReadCloser, error) {
	if !cleanRelative(name) {
		return nil, fmt.Errorf("%s: unsafe archive name", name)
	}
	file, err := os.Open(filepath.Join(d.dataDir, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("%s: not a regular file", name)
	}
	if info.Size() > limit {
		file.Close()
		return nil, fmt.Errorf("%s is %d bytes: %w of %d bytes", name, info.Size(), ErrTooLarge, limit)
	}
	return NewLimitedReader(file, file, limit, name), nil
}

func (d *dirSource) List(dir string) ([]string, error) {
	target := d.dataDir
	if dir != "" {
		if !cleanRelative(dir) {
			return nil, fmt.Errorf("%s: unsafe archive name", dir)
		}
		target = filepath.Join(d.dataDir, filepath.FromSlash(dir))
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (d *dirSource) All() ([]string, error) {
	names := []string{}
	err := filepath.WalkDir(d.dataDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if len(names) >= Limits.Entries {
			return fmt.Errorf("the archive has more than %d files: %w", Limits.Entries, ErrTooLarge)
		}
		relative, err := filepath.Rel(d.dataDir, path)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func (d *dirSource) Kind() string { return KindDirectory }
func (d *dirSource) Close() error { return nil }

type zipSource struct {
	reader   *zip.Reader
	closer   io.Closer
	prefix   string
	files    map[string]*zip.File
	spoolDir string
}

// fromZip indexes a ZIP's directory and finds the directory holding its data
// files. Nothing is extracted.
func fromZip(reader *zip.Reader, closer io.Closer, label string, spec Spec) (Source, int, error) {
	if len(reader.File) > Limits.Entries {
		return nil, 0, fmt.Errorf("the archive has %d entries: %w of %d entries", len(reader.File), ErrTooLarge, Limits.Entries)
	}
	files := make(map[string]*zip.File, len(reader.File))
	rejected := 0
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "/") {
			// A directory entry has no content, and nothing here creates
			// directories, so its name is never used. X's own archives carry
			// some with odd names ("assets//"); counting those as refused
			// would warn about a perfectly ordinary download.
			continue
		}
		if !cleanRelative(file.Name) || !file.Mode().IsRegular() {
			rejected++
			continue
		}
		files[file.Name] = file
	}
	prefix, ok := dataPrefix(files, spec)
	if !ok {
		return nil, rejected, spec.noDataDirectory(label, KindZip)
	}
	return &zipSource{reader: reader, closer: closer, prefix: prefix, files: files}, rejected, nil
}

// dataPrefix finds the directory holding the data files. A downloaded archive
// keeps them at a known depth, but a ZIP re-made around an extracted folder
// adds a level; the shallowest directory wins.
func dataPrefix(files map[string]*zip.File, spec Spec) (string, bool) {
	best, found := "", false
	for name := range files {
		if !spec.IsDataFile(path.Base(name)) {
			continue
		}
		dir := path.Dir(name)
		if dir == "." {
			dir = ""
		}
		switch {
		case !found,
			strings.Count(dir, "/") < strings.Count(best, "/"),
			strings.Count(dir, "/") == strings.Count(best, "/") && len(dir) < len(best),
			strings.Count(dir, "/") == strings.Count(best, "/") && len(dir) == len(best) && dir < best:
			best, found = dir, true
		}
	}
	return best, found
}

func (z *zipSource) full(name string) string {
	if z.prefix == "" {
		return name
	}
	return z.prefix + "/" + name
}

func (z *zipSource) Open(name string, limit int64) (io.ReadCloser, error) {
	if !cleanRelative(name) {
		return nil, fmt.Errorf("%s: unsafe archive name", name)
	}
	file, ok := z.files[z.full(name)]
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
	}
	if file.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("%s is %d bytes: %w of %d bytes", name, file.UncompressedSize64, ErrTooLarge, limit)
	}
	stream, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return NewLimitedReader(stream, stream, limit, name), nil
}

func (z *zipSource) List(dir string) ([]string, error) {
	target := z.prefix
	if dir != "" {
		target = z.full(dir)
	}
	names := []string{}
	for name := range z.files {
		parent := path.Dir(name)
		if parent == "." {
			parent = ""
		}
		if parent == target {
			names = append(names, path.Base(name))
		}
	}
	sort.Strings(names)
	return names, nil
}

func (z *zipSource) All() ([]string, error) {
	names := make([]string, 0, len(z.files))
	for name := range z.files {
		switch {
		case z.prefix == "":
			names = append(names, name)
		case strings.HasPrefix(name, z.prefix+"/"):
			names = append(names, strings.TrimPrefix(name, z.prefix+"/"))
		}
	}
	sort.Strings(names)
	return names, nil
}

func (z *zipSource) Kind() string { return KindZip }

func (z *zipSource) Close() error {
	var err error
	if z.closer != nil {
		err = z.closer.Close()
	}
	if z.spoolDir != "" {
		err = errors.Join(err, os.RemoveAll(z.spoolDir))
		z.spoolDir = ""
	}
	return err
}

// NewLimitedReader fails a read that would take a file past limit, so a file
// whose recorded size understates it cannot grow without bound. closer may be
// nil.
func NewLimitedReader(reader io.Reader, closer io.Closer, limit int64, name string) io.ReadCloser {
	return &limitedReadCloser{reader: reader, closer: closer, limit: limit, name: name}
}

type limitedReadCloser struct {
	reader io.Reader
	closer io.Closer
	limit  int64
	read   int64
	name   string
}

func (l *limitedReadCloser) Read(p []byte) (int, error) {
	if remaining := l.limit - l.read + 1; int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := l.reader.Read(p)
	l.read += int64(n)
	if l.read > l.limit {
		return n - int(l.read-l.limit), fmt.Errorf("%s: %w of %d bytes", l.name, ErrTooLarge, l.limit)
	}
	return n, err
}

func (l *limitedReadCloser) Close() error {
	if l.closer == nil {
		return nil
	}
	return l.closer.Close()
}

// DecodeArray decodes a JSON array one entry at a time, calling each for every
// entry, and returns how many there were. The reader must be positioned at the
// opening bracket. A data file is decoded as a stream, never read whole.
func DecodeArray[T any](r io.Reader, name string, each func(T) error) (int, error) {
	decoder := json.NewDecoder(r)
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return 0, fmt.Errorf("%s: expected a JSON array", name)
	}
	count := 0
	for decoder.More() {
		var entry T
		if err := decoder.Decode(&entry); err != nil {
			return count, fmt.Errorf("%s: entry %d: %w", name, count+1, err)
		}
		count++
		if err := each(entry); err != nil {
			return count, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return count, fmt.Errorf("%s: %w", name, err)
	}
	return count, nil
}

// DecodeObject decodes one JSON object from a data file into value. It is for
// the small records beside a conversation file, such as a project or an asset
// name map; a list of records belongs in DecodeArray, which streams.
func DecodeObject(r io.Reader, name string, value any) error {
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// HasName reports whether names holds want.
func HasName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
