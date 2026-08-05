// Package markdownblocks splits canonical Markdown into addressable blocks.
//
// A block is the unit a link can point at with an anchor: a heading, a
// paragraph, a list item, a fenced code block, or a table. Identity is
// content-based by decision (`PROJECT_DECISIONS.md` 17): a block's key derives
// from its text, so moving a block within a note keeps its identity and
// editing its text mints a new one. An anchor therefore always names exactly
// the text it was written against.
//
// The parser is deliberately conservative and deterministic, matching
// `internal/markdownlinks`: it favours predictable behaviour over full
// CommonMark coverage, and a later AST-backed implementation can replace it
// without changing the stored block model.
package markdownblocks

import (
	"crypto/sha256"
	"encoding/base32"
	"regexp"
	"strings"
)

// Block kinds. They are stored, so they are part of the schema contract.
const (
	KindHeading   = "heading"
	KindParagraph = "paragraph"
	KindListItem  = "list_item"
	KindCode      = "code"
	KindTable     = "table"
)

// Limits bound what one note contributes. A pathological note must not be able
// to write an unbounded number of rows in a single save.
const (
	MaxBlocksPerDocument = 10000
	MaxMarkerBytes       = 128
)

// Block is one addressable region of a note body.
type Block struct {
	Kind string
	// Level is the heading level (1-6) and zero for every other kind.
	Level int
	// Ordinal is the block's position in the document, from zero. It is
	// recorded for display and ordering only: identity never depends on it.
	Ordinal int
	// StartByte and EndByte bound the block in the body it was parsed from.
	StartByte int
	EndByte   int
	// Text is the block's normalized content — the exact bytes hashed.
	Text string
	// Marker is an author-written Obsidian-style `^marker`, without the caret,
	// when the block ends with one. Authored names outrank derived ones.
	Marker string
	// ContentSHA256 is the hash over document scope, kind, normalized text, and
	// occurrence. ID is its short opaque rendering.
	ContentSHA256 string
	ID            string
	// Occurrence disambiguates blocks with identical text in one note.
	Occurrence int
}

var (
	headingRE  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	listItemRE = regexp.MustCompile(`^\s*([-*+]|\d+[.)])\s+`)
	markerRE   = regexp.MustCompile(`\s\^([A-Za-z0-9_-]{1,` + itoa(MaxMarkerBytes) + `})\s*$`)
	tableRowRE = regexp.MustCompile(`^\s*\|.*\|\s*$`)
)

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// Extract splits a note body into blocks. documentID scopes every identity, so
// the same paragraph in two notes never shares a block ID — identity belongs to
// a note, and cross-note correlation is not something a block key should
// enable.
func Extract(documentID, body string) []Block {
	lines, offsets := physicalLines(body)
	blocks := []Block{}
	occurrences := map[string]int{}

	appendBlock := func(block Block) bool {
		if len(blocks) >= MaxBlocksPerDocument {
			return false
		}
		block.Text, block.Marker = splitMarker(block.Text)
		block.Text = normalize(block.Text)
		if strings.TrimSpace(block.Text) == "" {
			return true
		}
		key := block.Kind + "\x00" + block.Text
		block.Occurrence = occurrences[key]
		occurrences[key]++
		block.Ordinal = len(blocks)
		block.ContentSHA256, block.ID = identify(documentID, block)
		blocks = append(blocks, block)
		return true
	}

	index := 0
	for index < len(lines) {
		line := lines[index]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			index++
			continue
		}

		// A fenced code block runs to its closing fence, or to the end of the
		// note when the fence is never closed.
		if fence := codeFence(trimmed); fence != "" {
			start := offsets[index]
			end := offsets[index] + len(line)
			body := []string{line}
			index++
			for index < len(lines) {
				body = append(body, lines[index])
				end = offsets[index] + len(lines[index])
				closed := strings.HasPrefix(strings.TrimSpace(lines[index]), fence)
				index++
				if closed {
					break
				}
			}
			if !appendBlock(Block{Kind: KindCode, StartByte: start, EndByte: end, Text: strings.Join(body, "\n")}) {
				return blocks
			}
			continue
		}

		if match := headingRE.FindStringSubmatch(trimmed); match != nil {
			if !appendBlock(Block{
				Kind:      KindHeading,
				Level:     len(match[1]),
				StartByte: offsets[index],
				EndByte:   offsets[index] + len(line),
				Text:      match[2],
			}) {
				return blocks
			}
			index++
			continue
		}

		if listItemRE.MatchString(line) {
			// One list item is one block: a link anchor points at an item, not
			// at the whole list.
			if !appendBlock(Block{
				Kind:      KindListItem,
				StartByte: offsets[index],
				EndByte:   offsets[index] + len(line),
				Text:      strings.TrimSpace(listItemRE.ReplaceAllString(line, "")),
			}) {
				return blocks
			}
			index++
			continue
		}

		if tableRowRE.MatchString(line) {
			start := offsets[index]
			end := offsets[index] + len(line)
			rows := []string{trimmed}
			index++
			for index < len(lines) && tableRowRE.MatchString(lines[index]) {
				rows = append(rows, strings.TrimSpace(lines[index]))
				end = offsets[index] + len(lines[index])
				index++
			}
			if !appendBlock(Block{Kind: KindTable, StartByte: start, EndByte: end, Text: strings.Join(rows, "\n")}) {
				return blocks
			}
			continue
		}

		// Otherwise: a paragraph, running until a blank line or a line that
		// starts a different kind of block.
		start := offsets[index]
		end := offsets[index] + len(line)
		paragraph := []string{trimmed}
		index++
		for index < len(lines) {
			next := lines[index]
			nextTrimmed := strings.TrimSpace(next)
			if nextTrimmed == "" || headingRE.MatchString(nextTrimmed) ||
				listItemRE.MatchString(next) || codeFence(nextTrimmed) != "" || tableRowRE.MatchString(next) {
				break
			}
			paragraph = append(paragraph, nextTrimmed)
			end = offsets[index] + len(next)
			index++
		}
		if !appendBlock(Block{Kind: KindParagraph, StartByte: start, EndByte: end, Text: strings.Join(paragraph, "\n")}) {
			return blocks
		}
	}
	return blocks
}

// identify derives the content hash and the opaque block ID.
//
// The hash covers the document ID, the kind, the normalized text, and the
// occurrence index. Document scope keeps identity local to a note; kind keeps a
// heading distinct from a paragraph that reads the same; occurrence keeps two
// identical paragraphs in one note distinguishable without reintroducing
// position as identity.
func identify(documentID string, block Block) (string, string) {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"notrios-block-v1", documentID, block.Kind, block.Text, itoa(block.Occurrence),
	}, "\x00")))
	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:10]))
	return hex(digest[:]), "blk_" + encoded
}

func hex(value []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(value)*2)
	for _, b := range value {
		out = append(out, digits[b>>4], digits[b&0x0f])
	}
	return string(out)
}

// normalize is the documented part of identity: line endings are normalized and
// trailing whitespace is trimmed, so an editor that cleans whitespace on save
// does not silently break every anchor in the note. Nothing else is touched —
// case, punctuation, and emphasis are content.
func normalize(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// splitMarker removes an author-written `^marker` from the end of a block and
// returns it separately. The marker is a name the author chose; it is not part
// of the text that derives identity.
func splitMarker(text string) (string, string) {
	match := markerRE.FindStringSubmatchIndex(text)
	if match == nil {
		return text, ""
	}
	marker := text[match[2]:match[3]]
	return strings.TrimRight(text[:match[0]], " \t"), marker
}

func codeFence(trimmed string) string {
	switch {
	case strings.HasPrefix(trimmed, "```"):
		return "```"
	case strings.HasPrefix(trimmed, "~~~"):
		return "~~~"
	default:
		return ""
	}
}

// physicalLines splits on LF while recording each line's byte offset, so a
// block's range refers to the body it was parsed from.
func physicalLines(body string) ([]string, []int) {
	lines := []string{}
	offsets := []int{}
	start := 0
	for i := 0; i < len(body); i++ {
		if body[i] == '\n' {
			lines = append(lines, strings.TrimSuffix(body[start:i], "\r"))
			offsets = append(offsets, start)
			start = i + 1
		}
	}
	lines = append(lines, body[start:])
	offsets = append(offsets, start)
	return lines, offsets
}
