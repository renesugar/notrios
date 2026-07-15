package joplinraw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

const sourceSystem = "joplin_raw"

type Options struct {
	CollectionID string
	DryRun       bool
}

type Report struct {
	SourceDir          string   `json:"source_dir"`
	CollectionID       string   `json:"collection_id"`
	DryRun             bool     `json:"dry_run"`
	NotesSeen          int      `json:"notes_seen"`
	NotesImported      int      `json:"notes_imported"`
	NotesUpdated       int      `json:"notes_updated"`
	NotesUnchanged     int      `json:"notes_unchanged"`
	ResourcesSeen      int      `json:"resources_seen"`
	ResourcesImported  int      `json:"resources_imported"`
	ResourcesExisting  int      `json:"resources_existing"`
	ResourcesSkipped   int      `json:"resources_skipped"`
	LinksRewritten     int      `json:"links_rewritten"`
	AttachmentsCreated int      `json:"attachments_created"`
	Warnings           []string `json:"warnings,omitempty"`
}

type parsedItem struct {
	Path   string
	ID     string
	Type   string
	Fields map[string]string
	Body   string
}

func Import(ctx context.Context, st store.Store, sourceDir string, options Options) (Report, error) {
	collectionID := strings.TrimSpace(options.CollectionID)
	if collectionID == "" {
		collectionID = "default"
	}
	report := Report{SourceDir: sourceDir, CollectionID: collectionID, DryRun: options.DryRun}
	items, err := readItems(sourceDir)
	if err != nil {
		return report, err
	}

	folders := map[string]parsedItem{}
	tags := map[string]string{}
	noteTagIDs := map[string][]string{}
	notes := []parsedItem{}
	resources := []parsedItem{}
	for _, item := range items {
		switch item.Type {
		case "1":
			report.NotesSeen++
			notes = append(notes, item)
		case "2":
			folders[item.ID] = item
		case "4":
			report.ResourcesSeen++
			resources = append(resources, item)
		case "5":
			tags[item.ID] = item.Fields["title"]
		case "6":
			noteID := item.Fields["note_id"]
			tagID := item.Fields["tag_id"]
			if noteID != "" && tagID != "" {
				noteTagIDs[noteID] = append(noteTagIDs[noteID], tagID)
			}
		}
	}

	noteTags := map[string][]string{}
	for noteID, tagIDs := range noteTagIDs {
		for _, tagID := range tagIDs {
			if tag := tags[tagID]; tag != "" {
				noteTags[noteID] = append(noteTags[noteID], tag)
			}
		}
	}

	noteIDMap := map[string]string{}
	resourceIDMap := map[string]string{}
	for _, note := range notes {
		noteIDMap[note.ID] = noteDocumentID(note.ID)
	}
	for _, res := range resources {
		resourceIDMap[res.ID] = resourceID(res.ID)
	}

	if !options.DryRun {
		for _, res := range resources {
			logicalID := resourceIDMap[res.ID]
			if _, err := st.GetResource(ctx, logicalID); err == nil {
				report.ResourcesExisting++
				continue
			} else if !errors.Is(err, store.ErrNotFound) {
				return report, err
			}
			path, ok := findResourceContent(sourceDir, res)
			if !ok {
				report.ResourcesSkipped++
				report.Warnings = append(report.Warnings, fmt.Sprintf("resource %s has no content file", res.ID))
				continue
			}
			file, err := os.Open(path)
			if err != nil {
				return report, err
			}
			filename := resourceFilename(res, path)
			mimeType := firstNonEmpty(res.Fields["mime"], res.Fields["mime_type"], mime.TypeByExtension(filepath.Ext(filename)), "application/octet-stream")
			_, createErr := st.CreateResource(ctx, store.CreateResourceRequest{PreferredID: logicalID, CollectionID: collectionID, Filename: filename, MIMEType: mimeType, Content: file})
			closeErr := file.Close()
			if createErr != nil {
				return report, createErr
			}
			if closeErr != nil {
				return report, closeErr
			}
			report.ResourcesImported++
		}
	}

	for _, note := range notes {
		logicalID := noteIDMap[note.ID]
		body, rewrites := buildDocumentBody(note, folders, noteTags[note.ID], noteIDMap, resourceIDMap, collectionID)
		report.LinksRewritten += rewrites
		if options.DryRun {
			report.NotesImported++
			continue
		}
		existing, err := st.GetDocument(ctx, logicalID)
		if err == nil {
			if existing.Title == noteTitle(note) && existing.Body == body {
				report.NotesUnchanged++
				continue
			}
			_, err = st.UpdateDocument(ctx, store.UpdateDocumentRequest{ID: logicalID, Title: noteTitle(note), Body: body, BodyMIMEType: "text/markdown", BaseRevisionID: existing.CurrentRevisionID, Message: "import update from Joplin RAW"})
			if err != nil {
				return report, err
			}
			report.NotesUpdated++
		} else if errors.Is(err, store.ErrNotFound) {
			_, err = st.CreateDocument(ctx, store.CreateDocumentRequest{PreferredID: logicalID, CollectionID: collectionID, Title: noteTitle(note), Body: body, BodyMIMEType: "text/markdown", Message: "import from Joplin RAW"})
			if err != nil {
				return report, err
			}
			report.NotesImported++
		} else {
			return report, err
		}
		for originalResourceID, newResourceID := range resourceIDMap {
			uri := store.ResourceURI(collectionID, newResourceID)
			if strings.Contains(body, uri) {
				_, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{DocumentID: logicalID, ResourceID: newResourceID, RelationType: "referenced", AnchorJSON: fmt.Sprintf(`{"joplin_resource_id":%q}`, originalResourceID)})
				if err == nil {
					report.AttachmentsCreated++
				} else if !errors.Is(err, store.ErrNotFound) {
					return report, err
				}
			}
		}
	}
	return report, nil
}

func readItems(sourceDir string) ([]parsedItem, error) {
	var items []parsedItem
	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > 8*1024*1024 {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		item, ok := parseItem(path, string(b))
		if !ok {
			return nil
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items, nil
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

var joplinLinkRE = regexp.MustCompile(`:/([A-Za-z0-9_-]+)`) // Joplin internal note/resource link.

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
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
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
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func yamlQuote(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}
