package claude

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/renesugar/notrios/internal/importers/archivesource"
)

// J26: a Claude export arrives as a ZIP, and a large one arrives as several:
// claude-data-<id>-<time>-<n>-batch-0000.zip, batch-0001, and so on. What is
// here is what only a Claude archive knows; the reading is shared.

// conversationsFileRE matches the conversation files: conversations.json, and
// the sharded conversations-000.json form other exports use.
var conversationsFileRE = regexp.MustCompile(`^conversations(?:-([0-9]{1,6}))?\.json$`)

func archiveSpec() archivesource.Spec {
	return archivesource.Spec{
		IsDataFile: conversationsFileRE.MatchString,
		// A Claude ZIP keeps its files at the root, and an extracted one is
		// the folder itself; data/ is tried too, for an archive a user has
		// rearranged.
		DirCandidates: []string{"", "data"},
		NotAnArchive: func(name string, err error) error {
			return fmt.Errorf("%s is neither a Claude export ZIP nor an extracted export folder: %w", name, err)
		},
		NoDataDirectory: func(where, kind string) error {
			if kind == archivesource.KindZip {
				return fmt.Errorf("no conversations.json in %s; expected a Claude export ZIP", where)
			}
			return fmt.Errorf("no conversations.json found under %q; expected a Claude export ZIP or its extracted folder", where)
		},
	}
}

// conversationFiles returns the conversation files among names, in shard order.
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
		number := 0
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

// batchSiblingRE matches the batch ZIPs one export is split into.
var batchSiblingRE = regexp.MustCompile(`^claude-data-.*-batch-([0-9]{1,6})\.zip$`)

// batchSiblings returns the Claude batch ZIPs among names, in batch order, so a
// folder holding a split export imports as one archive.
func batchSiblings(names []string) []string {
	type batch struct {
		name   string
		number int
	}
	batches := []batch{}
	for _, name := range names {
		match := batchSiblingRE.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		number, _ := strconv.Atoi(match[1])
		batches = append(batches, batch{name: name, number: number})
	}
	sort.Slice(batches, func(i, j int) bool {
		if batches[i].number != batches[j].number {
			return batches[i].number < batches[j].number
		}
		return batches[i].name < batches[j].name
	})
	out := make([]string, 0, len(batches))
	for _, b := range batches {
		out = append(out, b.name)
	}
	return out
}
