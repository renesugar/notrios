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
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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
	MaxSlugBytes         = 128
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
	// Slug is the URI-safe name of a heading block, empty for every other kind.
	// It is what a `#section-title` anchor resolves against.
	Slug string
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
	var extractor Extractor
	return extractor.Extract(documentID, body)
}

// Extractor extracts blocks through buffers it keeps between notes (v1.0
// J32-T).
//
// Extract allocates a block slice, a line table and the maps behind them for
// every note it is given. An import parses every note in a library, so those
// are thousands of allocations that hold nothing once the note is written.
//
// The blocks an Extractor returns are valid until its next call: it is for a
// caller that writes each note's blocks before parsing the next one, which is
// what the import and save paths do. A caller that keeps blocks around uses
// Extract instead.
//
// Reset releases what the buffers hold without giving up their capacity, and
// matters more than it looks: a block's Text is a slice of the note's body, so
// a buffer still holding the last note's blocks keeps that whole body alive —
// 60 MiB for one large note.
type Extractor struct {
	blocks      []Block
	lines       []string
	offsets     []int
	scratch     []byte
	occurrences map[string]int
	slugs       map[string]int
}

// Reset drops what the buffers refer to, keeping their capacity.
func (e *Extractor) Reset() {
	for index := range e.blocks {
		e.blocks[index] = Block{}
	}
	e.blocks = e.blocks[:0]
	for index := range e.lines {
		e.lines[index] = ""
	}
	e.lines = e.lines[:0]
	e.offsets = e.offsets[:0]
	e.scratch = e.scratch[:0]
}

// Extract returns the blocks of one note, in document order. The result is
// valid until the next call on this Extractor.
func (e *Extractor) Extract(documentID, body string) []Block {
	e.Reset()
	lines, offsets := e.physicalLines(body)
	scratch := e.scratch
	blocks := e.blocks
	if e.occurrences == nil {
		e.occurrences, e.slugs = map[string]int{}, map[string]int{}
	}
	clear(e.occurrences)
	clear(e.slugs)
	occurrences, slugs := e.occurrences, e.slugs
	defer func() {
		e.scratch = scratch
		e.blocks = blocks
	}()

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
		if block.Kind == KindHeading {
			block.Slug = disambiguate(Slugify(block.Text), slugs)
		}
		scratch, block.ContentSHA256, block.ID = identify(documentID, block, scratch)
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
			// Written straight into a builder rather than collected into a
			// slice and joined: the text is the same, one allocation instead
			// of two (v1.0 J32-Q).
			stop := index + 1
			for stop < len(lines) {
				end = offsets[stop] + len(lines[stop])
				closed := strings.HasPrefix(strings.TrimSpace(lines[stop]), fence)
				stop++
				if closed {
					break
				}
			}
			text := joinLines(line, lines[index+1:stop], false, end-start)
			index = stop
			if !appendBlock(Block{Kind: KindCode, StartByte: start, EndByte: end, Text: text}) {
				return blocks
			}
			continue
		}

		if level, headingText, ok := matchHeading(trimmed); ok {
			if !appendBlock(Block{
				Kind:      KindHeading,
				Level:     level,
				StartByte: offsets[index],
				EndByte:   offsets[index] + len(line),
				Text:      headingText,
			}) {
				return blocks
			}
			index++
			continue
		}

		if markerEnd, ok := matchListMarker(line); ok {
			// One list item is one block: a link anchor points at an item, not
			// at the whole list.
			if !appendBlock(Block{
				Kind:      KindListItem,
				StartByte: offsets[index],
				EndByte:   offsets[index] + len(line),
				Text:      strings.TrimSpace(line[markerEnd:]),
			}) {
				return blocks
			}
			index++
			continue
		}

		if matchTableRow(line) {
			start := offsets[index]
			end := offsets[index] + len(line)
			stop := index + 1
			for stop < len(lines) && matchTableRow(lines[stop]) {
				end = offsets[stop] + len(lines[stop])
				stop++
			}
			rows := joinLines(trimmed, lines[index+1:stop], true, end-start)
			index = stop
			if !appendBlock(Block{Kind: KindTable, StartByte: start, EndByte: end, Text: rows}) {
				return blocks
			}
			continue
		}

		// Otherwise: a paragraph, running until a blank line or a line that
		// starts a different kind of block.
		start := offsets[index]
		end := offsets[index] + len(line)
		stop := index + 1
		for stop < len(lines) {
			next := lines[stop]
			nextTrimmed := strings.TrimSpace(next)
			if startsAnotherBlock(next, nextTrimmed) {
				break
			}
			end = offsets[stop] + len(next)
			stop++
		}
		paragraph := joinLines(trimmed, lines[index+1:stop], true, end-start)
		index = stop
		if !appendBlock(Block{Kind: KindParagraph, StartByte: start, EndByte: end, Text: paragraph}) {
			return blocks
		}
	}
	return blocks
}

// Slugify turns heading text into the URI-safe name a `#section-title` anchor
// uses.
//
// The rules are the ordinary Markdown ones — lowercase, spaces to hyphens, drop
// anything that is not a letter, digit, hyphen, or underscore — because that is
// what a hand-written `[text](note.md#some-heading)` link already assumes, and
// what Joplin and GitHub produce. Letters keep their Unicode case folding, so a
// heading in any script still slugs to something addressable rather than being
// emptied.
//
// A heading that slugs to nothing (an emoji, punctuation alone) gets no slug
// rather than a made-up one: it is reachable by its block ID, and inventing a
// name would make two unrelated headings collide.
func Slugify(text string) string {
	// Lowercased a rune at a time into a buffer sized from the text, rather
	// than lowercasing the whole string into a copy and collecting runes into
	// a slice to convert once more (v1.0 J32-S).
	text = strings.TrimSpace(text)
	out := make([]byte, 0, len(text))
	previousHyphen := false
	for _, r := range text {
		r = unicode.ToLower(r)
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			out = utf8.AppendRune(out, r)
			previousHyphen = false
		case r == '_' || r == '-' || r == ' ' || r == '\t':
			if len(out) > 0 && !previousHyphen {
				out = append(out, '-')
				previousHyphen = true
			}
		}
	}
	slug := strings.Trim(string(out), "-")
	if len(slug) > MaxSlugBytes {
		slug = strings.Trim(slug[:MaxSlugBytes], "-")
	}
	return slug
}

// disambiguate keeps repeated headings addressable: the first "Notes" is
// `notes`, the second `notes-1`. Obsidian resolves a duplicate heading to the
// first match and offers no way to name the second; a numbered suffix is the
// convention Markdown renderers already use and it costs nothing.
func disambiguate(slug string, seen map[string]int) string {
	if slug == "" {
		return ""
	}
	count := seen[slug]
	seen[slug]++
	if count == 0 {
		return slug
	}
	return slug + "-" + itoa(count)
}

// identify derives the content hash and the opaque block ID.
//
// The hash covers the document ID, the kind, the normalized text, and the
// occurrence index. Document scope keeps identity local to a note; kind keeps a
// heading distinct from a paragraph that reads the same; occurrence keeps two
// identical paragraphs in one note distinguishable without reintroducing
// position as identity.
// identify derives a block's content hash and its opaque ID, through a scratch
// buffer the caller reuses (v1.0 J32-Q, J32-S).
//
// The bytes hashed are exactly what a strings.Join of the five parts produced.
// What changed is the copying: the join allocated a copy of the block's text
// and the []byte conversion allocated a second, 0.96 GB on the near-limit
// corpus. Appending into a buffer that the next block reuses copies the text
// once and allocates only when a block is longer than any before it, and the
// occurrence is appended as digits rather than built as a string first.
//
// A streaming hash would copy nothing at all, but sha256.New returns an
// interface, and handing it the digest array makes that array escape for every
// block: measured at 40,000 more allocations on a 150,000-block note, which is
// worse than what it saves.
//
// Two strings are returned because both are stored, so two allocations are the
// floor. Each is written into an array on the stack and converted once.
func identify(documentID string, block Block, scratch []byte) ([]byte, string, string) {
	scratch = scratch[:0]
	for index, part := range [4]string{
		"notrios-block-v1", documentID, block.Kind, block.Text,
	} {
		if index > 0 {
			scratch = append(scratch, 0)
		}
		scratch = append(scratch, part...)
	}
	scratch = append(scratch, 0)
	scratch = strconv.AppendInt(scratch, int64(block.Occurrence), 10)
	digest := sha256.Sum256(scratch)

	var hashBytes [sha256.Size * 2]byte
	encodeHex(hashBytes[:0], digest[:])

	// "blk_" and the base32 of the first ten bytes, lowercased in place: the
	// encoder's own EncodeToString allocated, and the concatenation allocated
	// again.
	var idBytes [4 + 16]byte
	copy(idBytes[:4], "blk_")
	base32.StdEncoding.WithPadding(base32.NoPadding).Encode(idBytes[4:], digest[:10])
	for index := 4; index < len(idBytes); index++ {
		if idBytes[index] >= 'A' && idBytes[index] <= 'Z' {
			idBytes[index] += 'a' - 'A'
		}
	}
	return scratch, string(hashBytes[:]), string(idBytes[:])
}

func hex(value []byte) string {
	out := make([]byte, 0, len(value)*2)
	return string(encodeHex(out, value))
}

// encodeHex appends the lowercase hex of value to out.
func encodeHex(out []byte, value []byte) []byte {
	const digits = "0123456789abcdef"
	for _, b := range value {
		out = append(out, digits[b>>4], digits[b&0x0f])
	}
	return out
}

// normalize is the documented part of identity: line endings are normalized and
// trailing whitespace is trimmed, so an editor that cleans whitespace on save
// does not silently break every anchor in the note. Nothing else is touched —
// case, punctuation, and emphasis are content.
func normalize(text string) string {
	if !needsNormalizing(text) {
		// The common case: no carriage return and no line ending in spaces, so
		// the answer is the text itself, minus trailing newlines. Splitting it
		// into lines and joining them back produced the same bytes at the cost
		// of copying every block (v1.0 J32-Q).
		return strings.TrimRight(text, "\n")
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// The three matchers below replace the regexps they are named after in the
// parsing loop, which runs for every line of every note (v1.0 J32-S). The
// regexps allocated: a replacement string per list item, a submatch slice per
// heading, and a `sync.Pool` entry per call. The regexps stay in the package
// as the reference the tests hold these to, line for line, so what is
// recognised cannot drift.
//
// `\s` in Go's regexp is [\t\n\f\r ], which is what isSpaceByte matches. A
// line never contains a newline, because the body is split on newlines first.

// startsAnotherBlock reports whether a line ends the paragraph before it: a
// blank line, or the start of a block of another kind.
func startsAnotherBlock(line, trimmed string) bool {
	if trimmed == "" {
		return true
	}
	if _, _, ok := matchHeading(trimmed); ok {
		return true
	}
	if _, ok := matchListMarker(line); ok {
		return true
	}
	return codeFence(trimmed) != "" || matchTableRow(line)
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\f' || b == '\r'
}

// matchListMarker implements `^\s*([-*+]|\d+[.)])\s+`, returning the end of
// the match. It reports false when the line is not a list item.
func matchListMarker(line string) (int, bool) {
	index := 0
	for index < len(line) && isSpaceByte(line[index]) {
		index++
	}
	if index == len(line) {
		return 0, false
	}
	switch line[index] {
	case '-', '*', '+':
		index++
	default:
		digits := index
		for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
			digits++
		}
		if digits == index || digits == len(line) || (line[digits] != '.' && line[digits] != ')') {
			return 0, false
		}
		index = digits + 1
	}
	if index == len(line) || !isSpaceByte(line[index]) {
		return 0, false
	}
	for index < len(line) && isSpaceByte(line[index]) {
		index++
	}
	return index, true
}

// matchHeading implements `^(#{1,6})\s+(.*)$`, returning the level and the
// text after the marker.
func matchHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level == len(line) || !isSpaceByte(line[level]) {
		return 0, "", false
	}
	index := level
	for index < len(line) && isSpaceByte(line[index]) {
		index++
	}
	return level, line[index:], true
}

// matchTableRow implements `^\s*\|.*\|\s*$`.
func matchTableRow(line string) bool {
	start := 0
	for start < len(line) && isSpaceByte(line[start]) {
		start++
	}
	end := len(line)
	for end > start && isSpaceByte(line[end-1]) {
		end--
	}
	return end-start >= 2 && line[start] == '|' && line[end-1] == '|'
}

// joinLines is a block's text: its first line, then the rest joined with
// newlines, trimmed when the kind trims (v1.0 J32-Q).
//
// A block of one line is that line, which is already a slice of the body and
// costs nothing. A longer one is written into a builder grown once, from the
// block's extent in the body, which is an upper bound because the lines may be
// trimmed. Collecting the lines into a slice and joining them allocated twice
// per block; a builder left to grow by doubling allocated about twice the
// block's size; and a builder for every block, including the one-line ones,
// cost more allocations than the join it replaced. All three were measured.
func joinLines(first string, rest []string, trim bool, capacity int) string {
	if len(rest) == 0 {
		return first
	}
	var builder strings.Builder
	if capacity > len(first) {
		builder.Grow(capacity)
	}
	builder.WriteString(first)
	for _, line := range rest {
		if trim {
			line = strings.TrimSpace(line)
		}
		builder.WriteByte('\n')
		builder.WriteString(line)
	}
	return builder.String()
}

// needsNormalizing reports whether normalize would change anything: a carriage
// return anywhere, or a space or tab before a newline or at the end.
func needsNormalizing(text string) bool {
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '\r':
			return true
		case ' ', '\t':
			if index+1 == len(text) || text[index+1] == '\n' {
				return true
			}
		}
	}
	return false
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
func (e *Extractor) physicalLines(body string) ([]string, []int) {
	lines, offsets := e.lines[:0], e.offsets[:0]
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
	e.lines, e.offsets = lines, offsets
	return lines, offsets
}
