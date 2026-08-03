package joplinraw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/renesugar/notrios/internal/store"
)

const sourceSystem = "joplin_raw"

type Options struct {
	CollectionID   string
	DryRun         bool
	PreserveSource bool
	BatchSize      int
	SourceKey      string
	Config         *ImportConfig
	// AfterBatch is a test/embedding hook invoked after a durable checkpoint.
	AfterBatch func(phase string, processed, total int) error
}

type Report struct {
	SourceDir          string         `json:"source_dir"`
	SourceKey          string         `json:"source_key"`
	CollectionID       string         `json:"collection_id"`
	DryRun             bool           `json:"dry_run"`
	Resumed            bool           `json:"resumed"`
	CheckpointStatus   string         `json:"checkpoint_status"`
	BatchesCompleted   int            `json:"batches_completed"`
	MetadataFilesSeen  int            `json:"metadata_files_seen"`
	ItemsSeen          int            `json:"items_seen"`
	ItemTypeCounts     map[string]int `json:"item_type_counts"`
	MalformedItems     int            `json:"malformed_items"`
	UnsupportedItems   int            `json:"unsupported_items"`
	IgnoredFiles       int            `json:"ignored_files"`
	NotesSeen          int            `json:"notes_seen"`
	NotesImported      int            `json:"notes_imported"`
	NotesUpdated       int            `json:"notes_updated"`
	NotesUnchanged     int            `json:"notes_unchanged"`
	NotebooksSeen      int            `json:"notebooks_seen"`
	NotebooksCreated   int            `json:"notebooks_created"`
	NotebooksUpdated   int            `json:"notebooks_updated"`
	NotebooksSkipped   int            `json:"notebooks_skipped"`
	NotebooksExisting  int            `json:"notebooks_existing"`
	NotebooksMerged    int            `json:"notebooks_merged"`
	NotebookConflicts  []string       `json:"notebook_conflicts,omitempty"`
	TagsSeen           int            `json:"tags_seen"`
	TagsCreated        int            `json:"tags_created"`
	TagsUpdated        int            `json:"tags_updated"`
	TagsSkipped        int            `json:"tags_skipped"`
	TagsExisting       int            `json:"tags_existing"`
	TagsApplied        int            `json:"tags_applied"`
	TagsRemoved        int            `json:"tags_removed"`
	ResourcesSeen      int            `json:"resources_seen"`
	ResourcesImported  int            `json:"resources_imported"`
	ResourcesUpdated   int            `json:"resources_updated"`
	ResourcesExisting  int            `json:"resources_existing"`
	ResourcesSkipped   int            `json:"resources_skipped"`
	ResourcesMissing   int            `json:"resources_missing_content"`
	SourceBundleItems  int            `json:"source_bundle_items"`
	SourceBundleBytes  int64          `json:"source_bundle_bytes"`
	LinksRewritten     int            `json:"links_rewritten"`
	UnresolvedLinks    int            `json:"unresolved_links"`
	AttachmentsCreated int            `json:"attachments_created"`
	Warnings           []string       `json:"warnings,omitempty"`
	SuggestedConfig    *ImportConfig  `json:"suggested_config,omitempty"`
	// DocumentIDs lists the notes this run touched (created/updated/kept),
	// for post-import passes like --localize-media. Not part of the JSON
	// report.
	DocumentIDs []string `json:"-"`
}

type parsedItem struct {
	Path          string
	ID            string
	Type          string
	Fields        map[string]string
	Body          string
	PropertyOrder []string
}

func parseItem(path string, text string) (parsedItem, bool) {
	text = strings.TrimPrefix(text, "\uFEFF")
	fields, order, body, trailing := parseTrailingMetadata(text)
	if fields["type_"] == "" {
		fields, order, body = parseLeadingMetadata(text)
		trailing = false
	}
	if fields["type_"] == "" {
		return parsedItem{}, false
	}
	itemType := strings.TrimSpace(fields["type_"])
	if trailing && body != "" {
		title, remaining := splitSourceTitle(body)
		if title != "" {
			// Canonical Joplin RAW stores the item title as the first physical
			// body line, not in a title: property. Keep accepting the old
			// metadata-first fixtures below, but prefer the canonical title.
			fields["title"] = title
		}
		if itemType == "1" {
			body = remaining
		}
	}
	id := strings.TrimSpace(fields["id"])
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return parsedItem{
		Path: path, ID: id, Type: itemType, Fields: fields,
		Body: strings.TrimRight(body, "\n"), PropertyOrder: order,
	}, true
}

func parseItemBytes(path string, raw []byte) (parsedItem, bool, error) {
	if !utf8.Valid(raw) {
		return parsedItem{}, false, fmt.Errorf("%w: Joplin RAW item %s is not valid UTF-8", store.ErrInvalidInput, filepath.Base(path))
	}
	item, ok := parseItem(path, string(raw))
	return item, ok, nil
}

// splitPhysicalLines splits only at CR and LF. In particular it must not use
// a Unicode line splitter: Joplin PDF resource ocr_text values can contain
// vertical tab, form feed, file/record separators, and NEL as data.
func splitPhysicalLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := make([]string, 0, strings.Count(text, "\n")+1)
	start := 0
	for index := 0; index < len(text); index++ {
		if text[index] != '\r' && text[index] != '\n' {
			continue
		}
		lines = append(lines, text[start:index])
		if text[index] == '\r' && index+1 < len(text) && text[index+1] == '\n' {
			index++
		}
		start = index + 1
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func parseTrailingMetadata(text string) (map[string]string, []string, string, bool) {
	lines := splitPhysicalLines(text)
	end := len(lines)
	for end > 0 && lines[end-1] == "" {
		end--
	}
	separator := -1
	for index := end - 1; index >= 0; index-- {
		if lines[index] == "" {
			separator = index
			break
		}
		if !isMetadataLine(lines[index]) {
			break
		}
	}
	var bodyLines, metadataLines []string
	if separator >= 0 {
		bodyLines = lines[:separator]
		metadataLines = lines[separator+1 : end]
	} else {
		allMetadata := end > 0
		for _, line := range lines[:end] {
			if line != "" && !isMetadataLine(line) {
				allMetadata = false
				break
			}
		}
		if !allMetadata {
			return map[string]string{}, nil, normalizePhysicalLines(lines), false
		}
		metadataLines = lines[:end]
	}
	fields, order := parseFields(metadataLines)
	if strings.TrimSpace(fields["type_"]) == "" {
		return map[string]string{}, nil, normalizePhysicalLines(lines), false
	}
	return fields, order, normalizePhysicalLines(bodyLines), true
}

func parseLeadingMetadata(text string) (map[string]string, []string, string) {
	lines := splitPhysicalLines(text)
	metadataLines := []string{}
	fields := map[string]string{}
	bodyStart := 0
	for index, line := range lines {
		if line == "" {
			bodyStart = index + 1
			break
		}
		if !isMetadataLine(line) {
			break
		}
		metadataLines = append(metadataLines, line)
		bodyStart = index + 1
	}
	fields, order := parseFields(metadataLines)
	return fields, order, normalizePhysicalLines(lines[bodyStart:])
}

func isMetadataLine(line string) bool {
	index := strings.IndexByte(line, ':')
	return index > 0 && strings.TrimSpace(line[:index]) != ""
}

func parseFields(lines []string) (map[string]string, []string) {
	fields := map[string]string{}
	order := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		index := strings.IndexByte(line, ':')
		if index <= 0 {
			continue
		}
		rawKey := line[:index]
		key := strings.ToLower(strings.TrimSpace(rawKey))
		value := line[index+1:]
		if strings.HasPrefix(value, " ") {
			value = value[1:]
		}
		fields[key] = value
		order = append(order, strings.TrimSpace(rawKey))
	}
	return fields, order
}

func normalizePhysicalLines(lines []string) string {
	return strings.Join(lines, "\n")
}

func splitSourceTitle(body string) (string, string) {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 {
		return "", ""
	}
	title := strings.TrimSpace(lines[0])
	start := 1
	if start < len(lines) && lines[start] == "" {
		start++
	}
	return title, strings.Join(lines[start:], "\n")
}

type resourceReference struct {
	SourceID string
	TargetID string
}

type linkRewriteResult struct {
	Body       string
	Rewritten  int
	Unresolved int
	Resources  []resourceReference
}

func buildDocumentBody(note parsedItem, folders map[string]parsedItem, tags []string, noteIDMap, resourceIDMap map[string]string, collectionID string) (string, linkRewriteResult) {
	frontmatter := []string{"---"}
	frontmatter = append(frontmatter, "source_system: joplin_raw")
	frontmatter = append(frontmatter, "joplin_id: "+yamlQuote(note.ID))
	if parentID := strings.TrimSpace(note.Fields["parent_id"]); parentID != "" {
		frontmatter = append(frontmatter, "joplin_parent_id: "+yamlQuote(parentID))
		if notebook := notebookPath(parentID, folders); notebook != "" {
			frontmatter = append(frontmatter, "joplin_notebook: "+yamlQuote(notebook))
		}
	}
	if len(tags) > 0 {
		sort.Strings(tags)
		frontmatter = append(frontmatter, "joplin_tags:")
		for _, tag := range tags {
			frontmatter = append(frontmatter, "  - "+yamlQuote(tag))
		}
	}
	for _, key := range []string{"created_time", "updated_time", "user_created_time", "user_updated_time", "source_url"} {
		if value := strings.TrimSpace(note.Fields[key]); value != "" {
			frontmatter = append(frontmatter, key+": "+yamlQuote(value))
		}
	}
	frontmatter = append(frontmatter, "---", "")
	result := rewriteJoplinLinks(note.Body, noteIDMap, resourceIDMap, collectionID)
	return strings.Join(frontmatter, "\n") + result.Body + "\n", result
}

// rewriteJoplinLinks rewrites each resolvable :/id target while it scans the
// note once. It leaves fenced code, inline code, and escaped targets untouched,
// and returns direct resource references for bounded attachment planning.
func rewriteJoplinLinks(body string, noteIDMap, resourceIDMap map[string]string, collectionID string) linkRewriteResult {
	var output strings.Builder
	output.Grow(len(body))
	result := linkRewriteResult{}
	resources := map[string]resourceReference{}
	fenceCharacter := byte(0)
	fenceLength := 0
	inlineCodeLength := 0

	for start := 0; start < len(body); {
		end := strings.IndexByte(body[start:], '\n')
		hasNewline := end >= 0
		if hasNewline {
			end += start
		} else {
			end = len(body)
		}
		line := body[start:end]
		character, length, closing := markdownFence(line, fenceCharacter, fenceLength)
		if fenceCharacter != 0 {
			output.WriteString(line)
			if closing {
				fenceCharacter, fenceLength = 0, 0
			}
		} else if character != 0 {
			output.WriteString(line)
			fenceCharacter, fenceLength = character, length
			inlineCodeLength = 0
		} else {
			rewriteJoplinLinkLine(&output, line, &inlineCodeLength, noteIDMap, resourceIDMap, collectionID, &result, resources)
		}
		if hasNewline {
			output.WriteByte('\n')
			start = end + 1
		} else {
			start = end
		}
	}
	result.Body = output.String()
	result.Resources = make([]resourceReference, 0, len(resources))
	for _, reference := range resources {
		result.Resources = append(result.Resources, reference)
	}
	sort.Slice(result.Resources, func(i, j int) bool {
		if result.Resources[i].TargetID != result.Resources[j].TargetID {
			return result.Resources[i].TargetID < result.Resources[j].TargetID
		}
		return result.Resources[i].SourceID < result.Resources[j].SourceID
	})
	return result
}

func markdownFence(line string, activeCharacter byte, activeLength int) (byte, int, bool) {
	indent := 0
	for indent < len(line) && indent < 3 && line[indent] == ' ' {
		indent++
	}
	if indent >= len(line) || (line[indent] != '`' && line[indent] != '~') {
		return 0, 0, false
	}
	character := line[indent]
	end := indent
	for end < len(line) && line[end] == character {
		end++
	}
	length := end - indent
	if length < 3 {
		return 0, 0, false
	}
	if activeCharacter == 0 {
		return character, length, false
	}
	if character == activeCharacter && length >= activeLength && strings.TrimSpace(line[end:]) == "" {
		return character, length, true
	}
	return 0, 0, false
}

func rewriteJoplinLinkLine(output *strings.Builder, line string, inlineCodeLength *int, noteIDMap, resourceIDMap map[string]string, collectionID string, result *linkRewriteResult, resources map[string]resourceReference) {
	for index := 0; index < len(line); {
		if line[index] == '`' {
			end := index + 1
			for end < len(line) && line[end] == '`' {
				end++
			}
			runLength := end - index
			if *inlineCodeLength == 0 {
				*inlineCodeLength = runLength
			} else if *inlineCodeLength == runLength {
				*inlineCodeLength = 0
			}
			output.WriteString(line[index:end])
			index = end
			continue
		}
		if *inlineCodeLength != 0 || index+2 > len(line) || line[index:index+2] != ":/" || isEscaped(line, index) {
			output.WriteByte(line[index])
			index++
			continue
		}
		end := index + 2
		for end < len(line) && isJoplinIDCharacter(line[end]) {
			end++
		}
		if end == index+2 {
			output.WriteString(":/")
			index += 2
			continue
		}
		id := line[index+2 : end]
		if documentID := noteIDMap[id]; documentID != "" {
			output.WriteString(store.DocumentURI(collectionID, documentID))
			result.Rewritten++
		} else if resourceID := resourceIDMap[id]; resourceID != "" {
			output.WriteString(store.ResourceURI(collectionID, resourceID))
			result.Rewritten++
			resources[resourceID] = resourceReference{SourceID: id, TargetID: resourceID}
		} else {
			output.WriteString(line[index:end])
			result.Unresolved++
		}
		index = end
	}
}

func isEscaped(text string, index int) bool {
	backslashes := 0
	for index > 0 && text[index-1] == '\\' {
		backslashes++
		index--
	}
	return backslashes%2 == 1
}

func isJoplinIDCharacter(character byte) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || character == '_' || character == '-'
}

func notebookPath(id string, folders map[string]parsedItem) string {
	seen := map[string]bool{}
	parts := []string{}
	for id != "" && !seen[id] {
		seen[id] = true
		folder, ok := folders[id]
		if !ok {
			break
		}
		parts = append([]string{folder.Fields["title"]}, parts...)
		id = folder.Fields["parent_id"]
	}
	return strings.Join(parts, "/")
}

func findResourceContent(sourceDir string, item parsedItem) (string, bool) {
	ext := strings.TrimPrefix(strings.TrimSpace(firstNonEmpty(item.Fields["file_extension"], filepath.Ext(item.Fields["filename"]), filepath.Ext(item.Fields["title"]))), ".")
	candidates := []string{}
	if ext != "" {
		candidates = append(candidates,
			filepath.Join(sourceDir, "resources", item.ID+"."+ext),
			filepath.Join(sourceDir, ".resource", item.ID+"."+ext),
			filepath.Join(sourceDir, item.ID+"."+ext),
		)
	}
	candidates = append(candidates,
		filepath.Join(sourceDir, "resources", item.ID),
		filepath.Join(sourceDir, ".resource", item.ID),
		filepath.Join(sourceDir, item.ID),
	)
	metadataPath := filepath.Clean(item.Path)
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if candidate == metadataPath {
			continue
		}
		if info, err := os.Lstat(candidate); err == nil && !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			return candidate, true
		}
	}
	return "", false
}

func resourceFilename(item parsedItem, contentPath string) string {
	return firstNonEmpty(item.Fields["filename"], item.Fields["title"], filepath.Base(contentPath), item.ID)
}

func noteTitle(item parsedItem) string {
	return firstNonEmpty(item.Fields["title"], item.ID, "Untitled")
}

func noteDocumentID(joplinID string) string { return "doc_joplin_" + safeID(joplinID) }
func resourceID(joplinID string) string     { return "res_joplin_" + safeID(joplinID) }

func safeID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	var builder strings.Builder
	for _, character := range id {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func yamlQuote(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
