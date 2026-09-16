package chatgpt

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/importers/archivesource"
)

// J26: ChatGPT is exported in two shapes, and both arrive as a ZIP.
//
//   - The export from ChatGPT itself: conversations.json beside chat.html and
//     the asset files, named file-<id>-<name>.<ext> or file_<hash>-<name>.<ext>.
//   - The export from the OpenAI Privacy Portal: an outer ZIP whose
//     "User Online Activity" folder holds nested ZIPs. The conversations are
//     sharded (conversations-000.json, -001.json) inside one of them, the assets
//     are file-<id>.dat with their extensions stripped, and the file library is
//     a nested ZIP of its own.
//
// What is here is what only a ChatGPT archive knows; the reading is shared.

// conversationsFileRE matches a conversations file in either export:
// conversations.json, or the sharded conversations-000.json form.
var conversationsFileRE = regexp.MustCompile(`^conversations(?:-([0-9]{1,6}))?\.json$`)

// portalConversationsRE and portalFilesRE match the Privacy Portal's nested
// ZIPs, which are named by kind and a digest.
var (
	portalConversationsRE = regexp.MustCompile(`(?i)^Conversations__.*\.zip$`)
	portalFilesRE         = regexp.MustCompile(`(?i)^Files__.*\.zip$`)
	// portalActivityDir is where the portal keeps them.
	portalActivityDir = "User Online Activity"
)

func archiveSpec() archivesource.Spec {
	return archivesource.Spec{
		IsDataFile: conversationsFileRE.MatchString,
		// A ChatGPT ZIP keeps conversations.json at the root; data/ is tried
		// too, for an archive a user has rearranged.
		DirCandidates: []string{"", "data"},
		NotAnArchive: func(name string, err error) error {
			return fmt.Errorf("%s is neither a ChatGPT export ZIP nor an extracted export folder: %w", name, err)
		},
		NoDataDirectory: func(where, kind string) error {
			if kind == archivesource.KindZip {
				return fmt.Errorf("no conversations.json in %s; expected a ChatGPT export ZIP or an OpenAI Privacy Portal export ZIP", where)
			}
			return fmt.Errorf("no conversations.json found under %q; expected a ChatGPT export ZIP or its extracted folder", where)
		},
	}
}

// portalSpec opens the outer Privacy Portal ZIP, whose own data directory is
// the folder holding the nested ZIPs rather than any conversations file.
func portalSpec() archivesource.Spec {
	return archivesource.Spec{
		IsDataFile:    func(base string) bool { return portalConversationsRE.MatchString(base) },
		DirCandidates: []string{portalActivityDir, ""},
		NotAnArchive: func(name string, err error) error {
			return fmt.Errorf("%s is neither a ChatGPT export ZIP nor an OpenAI Privacy Portal export ZIP: %w", name, err)
		},
		NoDataDirectory: func(where, kind string) error {
			return fmt.Errorf("no Conversations__*.zip under %q in %s", portalActivityDir, where)
		},
	}
}

// conversationFiles returns the conversations files among names, in shard
// order: conversations.json is shard zero, then conversations-000.json and on.
func conversationFiles(names []string) []string {
	type shard struct {
		name   string
		number int
	}
	shards := []shard{}
	for _, name := range names {
		match := conversationsFileRE.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		number := -1
		if match[1] != "" {
			number, _ = strconv.Atoi(match[1])
		}
		shards = append(shards, shard{name: name, number: number})
	}
	sort.Slice(shards, func(i, j int) bool {
		if shards[i].number != shards[j].number {
			return shards[i].number < shards[j].number
		}
		return shards[i].name < shards[j].name
	})
	out := make([]string, 0, len(shards))
	for _, s := range shards {
		out = append(out, s.name)
	}
	return out
}

// assetID extracts the file ID an archive name or asset pointer carries. Both
// prefixes are in use, and the underscore form is the common one:
// "file-ABC-photo.jpeg", "file_0000abc-photo.png", "file-ABC.dat",
// "file-service://file-ABC", "sediment://file_0000abc".
var assetIDRE = regexp.MustCompile(`file[-_][A-Za-z0-9]+`)

func assetID(value string) string {
	if slash := strings.LastIndex(value, "/"); slash >= 0 {
		value = value[slash+1:]
	}
	return assetIDRE.FindString(value)
}
