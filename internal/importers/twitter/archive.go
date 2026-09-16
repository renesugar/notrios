package twitter

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// An archive is untrusted input, and a large one is several gigabytes. These
// bound what an import will read. They are variables so tests can lower them;
// nothing else changes them.
var (
	// maxArchiveEntries bounds the ZIP's directory. The owner's 3.3 GB archive
	// has 15,088 entries.
	maxArchiveEntries = 1_000_000
	// maxDataFileBytes bounds one decompressed JSON data file. A post file
	// in a large archive is about 105 MB.
	maxDataFileBytes int64 = 1 << 30
	// maxMediaFileBytes bounds one decompressed media file.
	maxMediaFileBytes int64 = 4 << 30
	// maxYTDPrefixBytes bounds the `window.YTD.<name>.partN =` assignment
	// before a data file's JSON array.
	maxYTDPrefixBytes = 4096
)

// errTooLarge is returned when a file or archive exceeds a bound above.
var errTooLarge = errors.New("exceeds the Twitter/X import size limit")

// archiveSource is where an archive's data directory is read from: an extracted
// folder, or the ZIP as it was downloaded, read in place. Names are
// slash-separated and relative to the data directory ("tweets.js",
// "tweets_media/123-abc.jpg").
type archiveSource interface {
	// Open opens one file, refusing it if it is larger than limit.
	Open(name string, limit int64) (io.ReadCloser, error)
	// List names the regular files directly inside dir ("" is the data
	// directory itself), sorted.
	List(dir string) ([]string, error)
	// Kind is "zip" or "directory".
	Kind() string
	Close() error
}

// tweetFileRE matches a post file: tweets.js is part zero, tweets-part1.js,
// tweets-part2.js and so on follow, and older archives use tweet.js.
var tweetFileRE = regexp.MustCompile(`^(tweets|tweet)(?:-part([0-9]{1,6}))?\.js$`)

// openArchive opens sourcePath as an extracted archive folder or as a ZIP. It
// also returns how many ZIP entries were rejected as unsafe.
func openArchive(sourcePath string) (archiveSource, int, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, 0, err
	}
	if info.IsDir() {
		dataDir, err := findDataDir(sourcePath)
		if err != nil {
			return nil, 0, err
		}
		return &dirSource{dataDir: dataDir}, 0, nil
	}
	return openZipSource(sourcePath)
}

// tweetFiles returns the post files among names, in part order. When an archive
// has both the current "tweets" names and the older "tweet" ones, the current
// names are used.
func tweetFiles(names []string) []string {
	type part struct {
		name   string
		number int
	}
	byBase := map[string][]part{}
	for _, name := range names {
		match := tweetFileRE.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		number := 0
		if match[2] != "" {
			number, _ = strconv.Atoi(match[2])
		}
		byBase[match[1]] = append(byBase[match[1]], part{name: name, number: number})
	}
	parts := byBase["tweets"]
	if len(parts) == 0 {
		parts = byBase["tweet"]
	}
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].number != parts[j].number {
			return parts[i].number < parts[j].number
		}
		return parts[i].name < parts[j].name
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, p.name)
	}
	return out
}

func hasName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

// cleanRelative reports whether an archive name is safe to use as a lookup
// key: relative, with no parent segments, backslashes, drive letters or NULs.
// Names are never joined onto the filesystem for writing, but a name that
// escapes the archive is refused rather than trusted anywhere.
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

// findDataDir finds the extracted archive's data directory: data/ under the
// folder given, or the folder itself.
func findDataDir(sourceDir string) (string, error) {
	for _, candidate := range []string{filepath.Join(sourceDir, "data"), sourceDir} {
		entries, err := os.ReadDir(candidate)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if len(tweetFiles(names)) > 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no tweets.js, tweets-partN.js or tweet.js found under %q; expected a Twitter/X archive ZIP or its extracted folder", sourceDir)
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
		return nil, fmt.Errorf("%s is %d bytes: %w of %d bytes", name, info.Size(), errTooLarge, limit)
	}
	return &limitedReadCloser{reader: file, closer: file, limit: limit, name: name}, nil
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

func (d *dirSource) Kind() string { return "directory" }
func (d *dirSource) Close() error { return nil }

type zipSource struct {
	reader *zip.ReadCloser
	prefix string
	files  map[string]*zip.File
}

// openZipSource reads the ZIP's directory in place. Nothing is extracted.
func openZipSource(zipPath string) (*zipSource, int, error) {
	reader, err := zip.OpenReader(zipPath)
	// Since Go 1.20 a ZIP with a ".." or absolute entry name opens with
	// ErrInsecurePath and a usable reader. Such names are rejected below, one
	// by one, rather than refusing the whole archive over them.
	if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && reader != nil) {
		return nil, 0, fmt.Errorf("%s is neither a Twitter/X archive ZIP nor an extracted archive folder: %w", filepath.Base(zipPath), err)
	}
	if len(reader.File) > maxArchiveEntries {
		count := len(reader.File)
		reader.Close()
		return nil, 0, fmt.Errorf("the archive has %d entries: %w of %d entries", count, errTooLarge, maxArchiveEntries)
	}
	files := make(map[string]*zip.File, len(reader.File))
	rejected := 0
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "/") {
			// A directory entry has no content, and the importer never
			// creates directories, so its name is never used. X's own archives
			// carry some with odd names ("assets//", "./"); counting those as
			// refused would warn about a perfectly ordinary download.
			continue
		}
		if !cleanRelative(file.Name) || !file.Mode().IsRegular() {
			rejected++
			continue
		}
		files[file.Name] = file
	}
	prefix, ok := zipDataPrefix(files)
	if !ok {
		reader.Close()
		return nil, rejected, fmt.Errorf("no tweets.js, tweets-partN.js or tweet.js in %s; expected a Twitter/X archive ZIP", filepath.Base(zipPath))
	}
	return &zipSource{reader: reader, prefix: prefix, files: files}, rejected, nil
}

// zipDataPrefix finds the directory holding the post files. A downloaded
// archive keeps them in data/, but a ZIP re-made around the extracted folder
// adds a level; the shallowest directory wins.
func zipDataPrefix(files map[string]*zip.File) (string, bool) {
	best, found := "", false
	for name := range files {
		if !tweetFileRE.MatchString(path.Base(name)) {
			continue
		}
		dir := path.Dir(name)
		if dir == "." {
			dir = ""
		}
		if !found || strings.Count(dir, "/") < strings.Count(best, "/") ||
			(strings.Count(dir, "/") == strings.Count(best, "/") && len(dir) < len(best)) ||
			(strings.Count(dir, "/") == strings.Count(best, "/") && len(dir) == len(best) && dir < best) {
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
		return nil, fmt.Errorf("%s is %d bytes: %w of %d bytes", name, file.UncompressedSize64, errTooLarge, limit)
	}
	stream, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &limitedReadCloser{reader: stream, closer: stream, limit: limit, name: name}, nil
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

func (z *zipSource) Kind() string { return "zip" }
func (z *zipSource) Close() error { return z.reader.Close() }

// limitedReadCloser fails a read that would take a file past limit, so a file
// whose recorded size understates it cannot grow without bound.
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
		return n - int(l.read-l.limit), fmt.Errorf("%s: %w of %d bytes", l.name, errTooLarge, l.limit)
	}
	return n, err
}

func (l *limitedReadCloser) Close() error { return l.closer.Close() }

// decodeYTDArray decodes a `window.YTD.<name>.partN = [ ... ]` data file one
// entry at a time, calling each for every entry, and returns how many there
// were. A post file is decoded as a stream, never read whole.
func decodeYTDArray[T any](r io.Reader, name string, each func(T) error) (int, error) {
	// The buffer is sized for reading, not for the prefix: a 4 KiB buffer
	// would feed a 105 MB post file to the decoder 4 KiB at a time.
	buffered := bufio.NewReaderSize(r, 1<<16)
	prefix, err := buffered.ReadSlice('=')
	if errors.Is(err, bufio.ErrBufferFull) || len(prefix) > maxYTDPrefixBytes {
		return 0, fmt.Errorf("%s: no window.YTD assignment in its first %d bytes", name, maxYTDPrefixBytes)
	}
	if err != nil {
		return 0, fmt.Errorf("%s: missing window.YTD assignment: %w", name, err)
	}
	decoder := json.NewDecoder(buffered)
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return 0, fmt.Errorf("%s: expected a JSON array after the window.YTD assignment", name)
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
