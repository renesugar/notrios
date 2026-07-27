package joplinraw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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
	SourceDir          string        `json:"source_dir"`
	SourceKey          string        `json:"source_key"`
	CollectionID       string        `json:"collection_id"`
	DryRun             bool          `json:"dry_run"`
	Resumed            bool          `json:"resumed"`
	CheckpointStatus   string        `json:"checkpoint_status"`
	BatchesCompleted   int           `json:"batches_completed"`
	NotesSeen          int           `json:"notes_seen"`
	NotesImported      int           `json:"notes_imported"`
	NotesUpdated       int           `json:"notes_updated"`
	NotesUnchanged     int           `json:"notes_unchanged"`
	NotebooksSeen      int           `json:"notebooks_seen"`
	NotebooksCreated   int           `json:"notebooks_created"`
	NotebooksUpdated   int           `json:"notebooks_updated"`
	NotebooksSkipped   int           `json:"notebooks_skipped"`
	NotebooksExisting  int           `json:"notebooks_existing"`
	NotebooksMerged    int           `json:"notebooks_merged"`
	NotebookConflicts  []string      `json:"notebook_conflicts,omitempty"`
	TagsSeen           int           `json:"tags_seen"`
	TagsCreated        int           `json:"tags_created"`
	TagsUpdated        int           `json:"tags_updated"`
	TagsSkipped        int           `json:"tags_skipped"`
	TagsExisting       int           `json:"tags_existing"`
	TagsApplied        int           `json:"tags_applied"`
	TagsRemoved        int           `json:"tags_removed"`
	ResourcesSeen      int           `json:"resources_seen"`
	ResourcesImported  int           `json:"resources_imported"`
	ResourcesUpdated   int           `json:"resources_updated"`
	ResourcesExisting  int           `json:"resources_existing"`
	ResourcesSkipped   int           `json:"resources_skipped"`
	SourceBundleItems  int           `json:"source_bundle_items"`
	SourceBundleBytes  int64         `json:"source_bundle_bytes"`
	LinksRewritten     int           `json:"links_rewritten"`
	AttachmentsCreated int           `json:"attachments_created"`
	Warnings           []string      `json:"warnings,omitempty"`
	SuggestedConfig    *ImportConfig `json:"suggested_config,omitempty"`
	// DocumentIDs lists the notes this run touched (created/updated/kept),
	// for post-import passes like --localize-media. Not part of the JSON
	// report.
	DocumentIDs []string `json:"-"`
}

type parsedItem struct {
	Path   string
	ID     string
	Type   string
	Fields map[string]string
	Body   string
}

var metadataLineRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*:\s*.*$`)

func parseItem(path string, text string) (parsedItem, bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	fields, body := parseTrailingMetadata(text)
	if fields["type_"] == "" {
		fields, body = parseLeadingMetadata(text)
	}
	if fields["type_"] == "" {
		return parsedItem{}, false
	}
	id := strings.TrimSpace(fields["id"])
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return parsedItem{Path: path, ID: id, Type: strings.TrimSpace(fields["type_"]), Fields: fields, Body: strings.TrimRight(body, "\n")}, true
}

func parseTrailingMetadata(text string) (map[string]string, string) {
	lines := strings.Split(text, "\n")
	start := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" && start == len(lines) {
			start = i
			continue
		}
		if metadataLineRE.MatchString(line) {
			start = i
			continue
		}
		break
	}
	fields := map[string]string{}
	if start < len(lines) {
		for _, line := range lines[start:] {
			parseField(fields, line)
		}
	}
	if fields["type_"] == "" {
		return map[string]string{}, text
	}
	return fields, strings.TrimRight(strings.Join(lines[:start], "\n"), "\n")
}

func parseLeadingMetadata(text string) (map[string]string, string) {
	lines := strings.Split(text, "\n")
	fields := map[string]string{}
	bodyStart := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			bodyStart = i + 1
			break
		}
		if !metadataLineRE.MatchString(trimmed) {
			break
		}
		parseField(fields, trimmed)
		bodyStart = i + 1
	}
	return fields, strings.Join(lines[bodyStart:], "\n")
}

func parseField(fields map[string]string, line string) {
	trimmed := strings.TrimSpace(line)
	idx := strings.Index(trimmed, ":")
	if idx <= 0 {
		return
	}
	key := strings.ToLower(strings.TrimSpace(trimmed[:idx]))
	value := strings.TrimSpace(trimmed[idx+1:])
	fields[key] = value
}

func buildDocumentBody(note parsedItem, folders map[string]parsedItem, tags []string, noteIDMap, resourceIDMap map[string]string, collectionID string) (string, int) {
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
	rewritten, count := rewriteJoplinLinks(note.Body, noteIDMap, resourceIDMap, collectionID)
	return strings.Join(frontmatter, "\n") + rewritten + "\n", count
}

var joplinLinkRE = regexp.MustCompile(`:/([A-Za-z0-9_-]+)`)

func rewriteJoplinLinks(body string, noteIDMap, resourceIDMap map[string]string, collectionID string) (string, int) {
	count := 0
	out := joplinLinkRE.ReplaceAllStringFunc(body, func(match string) string {
		id := strings.TrimPrefix(match, ":/")
		if docID := noteIDMap[id]; docID != "" {
			count++
			return store.DocumentURI(collectionID, docID)
		}
		if resID := resourceIDMap[id]; resID != "" {
			count++
			return store.ResourceURI(collectionID, resID)
		}
		return match
	})
	return out, count
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
