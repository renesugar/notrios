package twitter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"

	"github.com/renesugar/notrios/internal/importers/archivesource"
)

// The archive reader is shared with the other importers (J26). What stays here
// is what only a Twitter/X archive knows: which files hold posts, and what to
// say when this is not one of its archives.
type archiveSource = archivesource.Source

// maxYTDPrefixBytes bounds the `window.YTD.<name>.partN =` assignment before a
// data file's JSON array.
var maxYTDPrefixBytes = 4096

// tweetFileRE matches a post file: tweets.js is part zero, tweets-part1.js,
// tweets-part2.js and so on follow, and older archives use tweet.js.
var tweetFileRE = regexp.MustCompile(`^(tweets|tweet)(?:-part([0-9]{1,6}))?\.js$`)

func archiveSpec() archivesource.Spec {
	return archivesource.Spec{
		IsDataFile: tweetFileRE.MatchString,
		NotAnArchive: func(name string, err error) error {
			return fmt.Errorf("%s is neither a Twitter/X archive ZIP nor an extracted archive folder: %w", name, err)
		},
		NoDataDirectory: func(where, kind string) error {
			if kind == archivesource.KindZip {
				return fmt.Errorf("no tweets.js, tweets-partN.js or tweet.js in %s; expected a Twitter/X archive ZIP", where)
			}
			return fmt.Errorf("no tweets.js, tweets-partN.js or tweet.js found under %q; expected a Twitter/X archive ZIP or its extracted folder", where)
		},
	}
}

// openArchive opens sourcePath as an extracted archive folder or as the ZIP as
// downloaded. It also returns how many ZIP entries were refused as unsafe.
func openArchive(sourcePath string) (archiveSource, int, error) {
	return archivesource.Open(sourcePath, archiveSpec())
}

func hasName(names []string, want string) bool { return archivesource.HasName(names, want) }

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
	return archivesource.DecodeArray(buffered, name, each)
}
