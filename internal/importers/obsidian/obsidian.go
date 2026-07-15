// Package obsidian imports a minimal Obsidian-style Markdown vault into the
// managed companion store. The importer is intentionally conservative: it keeps
// Markdown source mostly intact, preserves original frontmatter, records vault
// paths in frontmatter, imports local assets as resources, and lets the store's
// Markdown graph parser resolve wikilinks, embeds, and resource links.
package obsidian

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/markdownlinks"
	"github.com/renesugar/notrios/internal/store"
)

const sourceSystem = "obsidian"

type Options struct {
	CollectionID string
	DryRun       bool
}

type Report struct {
	SourceDir            string   `json:"source_dir"`
	CollectionID         string   `json:"collection_id"`
	DryRun               bool     `json:"dry_run"`
	MarkdownSeen         int      `json:"markdown_seen"`
	NotesImported        int      `json:"notes_imported"`
	NotesUpdated         int      `json:"notes_updated"`
	NotesUnchanged       int      `json:"notes_unchanged"`
	ResourcesSeen        int      `json:"resources_seen"`
	ResourcesImported    int      `json:"resources_imported"`
	ResourcesExisting    int      `json:"resources_existing"`
	ResourcesSkipped     int      `json:"resources_skipped"`
	AttachmentsCreated   int      `json:"attachments_created"`
	LinkIndexesRefreshed int      `json:"link_indexes_refreshed"`
	Warnings             []string `json:"warnings,omitempty"`
}

type markdownFile struct {
	AbsPath string
	RelPath string
	Title   string
	Body    string
}

type assetFile struct {
	AbsPath  string
	RelPath  string
	Filename string
	MIMEType string
}

// Import imports a directory of Markdown files and non-Markdown assets. It is
// idempotent for deterministic document/resource IDs derived from vault-relative
// paths.
func Import(ctx context.Context, st store.Store, sourceDir string, options Options) (Report, error) {
	collectionID := strings.TrimSpace(options.CollectionID)
	if collectionID == "" {
		collectionID = "default"
	}
	report := Report{SourceDir: sourceDir, CollectionID: collectionID, DryRun: options.DryRun}
	notes, assets, err := scanVault(sourceDir)
	if err != nil {
		return report, err
	}
	report.MarkdownSeen = len(notes)
	report.ResourcesSeen = len(assets)

	assetIDByRel := map[string]string{}
	assetIDByBase := map[string][]string{}
	for _, asset := range assets {
		id := resourceID(asset.RelPath)
		assetIDByRel[normalizeVaultPath(asset.RelPath)] = id
		baseKey := strings.ToLower(filepath.Base(asset.RelPath))
		assetIDByBase[baseKey] = append(assetIDByBase[baseKey], id)
	}
	for key := range assetIDByBase {
		sort.Strings(assetIDByBase[key])
	}

	if !options.DryRun {
		for _, asset := range assets {
			logicalID := resourceID(asset.RelPath)
			if _, err := st.GetResource(ctx, logicalID); err == nil {
				report.ResourcesExisting++
				continue
			} else if !errors.Is(err, store.ErrNotFound) {
				return report, err
			}
			file, err := os.Open(asset.AbsPath)
			if err != nil {
				return report, err
			}
			_, createErr := st.CreateResource(ctx, store.CreateResourceRequest{
				PreferredID:  logicalID,
				CollectionID: collectionID,
				Filename:     asset.Filename,
				MIMEType:     asset.MIMEType,
				Content:      file,
			})
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

	importedDocIDs := []string{}
	for _, note := range notes {
		logicalID := documentID(note.RelPath)
		body := augmentFrontmatter(note.Body, note.RelPath)
		if options.DryRun {
			report.NotesImported++
			continue
		}
		existing, err := st.GetDocument(ctx, logicalID)
		if err == nil {
			if existing.Title == note.Title && existing.Body == body {
				report.NotesUnchanged++
			} else {
				_, err = st.UpdateDocument(ctx, store.UpdateDocumentRequest{
					ID:             logicalID,
					Title:          note.Title,
					Body:           body,
					BodyMIMEType:   "text/markdown",
					BaseRevisionID: existing.CurrentRevisionID,
					Message:        "import update from Obsidian vault",
				})
				if err != nil {
					return report, err
				}
				report.NotesUpdated++
			}
		} else if errors.Is(err, store.ErrNotFound) {
			_, err = st.CreateDocument(ctx, store.CreateDocumentRequest{
				PreferredID:  logicalID,
				CollectionID: collectionID,
				Title:        note.Title,
				Body:         body,
				BodyMIMEType: "text/markdown",
				Message:      "import from Obsidian vault",
			})
			if err != nil {
				return report, err
			}
			report.NotesImported++
		} else {
			return report, err
		}
		importedDocIDs = append(importedDocIDs, logicalID)
		attachments, warnings := referencedAssetIDs(note, assetIDByRel, assetIDByBase)
		report.Warnings = append(report.Warnings, warnings...)
		for _, attachment := range attachments {
			_, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{
				DocumentID:   logicalID,
				ResourceID:   attachment.ResourceID,
				RelationType: attachment.RelationType,
				AnchorJSON:   fmt.Sprintf(`{"obsidian_path":%q,"raw_target":%q}`, note.RelPath, attachment.RawTarget),
			})
			if err == nil {
				report.AttachmentsCreated++
			} else if !errors.Is(err, store.ErrNotFound) {
				return report, err
			}
		}
	}

	if !options.DryRun {
		// Rebuild link rows after all documents and resources have been imported.
		// Without this pass, a note imported before its target may retain an
		// unresolved wikilink until the next edit.
		for _, docID := range importedDocIDs {
			if err := st.RebuildDocumentLinks(ctx, docID); err != nil {
				return report, err
			}
			report.LinkIndexesRefreshed++
		}
	}

	return report, nil
}

func scanVault(sourceDir string) ([]markdownFile, []assetFile, error) {
	root, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, nil, err
	}
	notes := []markdownFile{}
	assets := []assetFile{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if shouldSkipDir(name) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".md" || ext == ".markdown" {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			body := normalizeNewlines(string(b))
			notes = append(notes, markdownFile{AbsPath: path, RelPath: rel, Title: markdownTitle(rel, body), Body: body})
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() == 0 {
			return nil
		}
		assets = append(assets, assetFile{AbsPath: path, RelPath: rel, Filename: filepath.Base(rel), MIMEType: assetMIMEType(path)})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].RelPath < notes[j].RelPath })
	sort.Slice(assets, func(i, j int) bool { return assets[i].RelPath < assets[j].RelPath })
	return notes, assets, nil
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", ".obsidian", ".trash", "node_modules":
		return true
	default:
		return false
	}
}

func markdownTitle(relPath, body string) string {
	if fm, _, ok := splitFrontmatter(body); ok {
		if title := frontmatterScalar(fm, "title"); title != "" {
			return title
		}
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	base := filepath.Base(relPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "%20", " ")
	return firstNonEmpty(base, "Untitled")
}

func frontmatterScalar(frontmatter, key string) string {
	key = strings.ToLower(key)
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		idx := strings.Index(trimmed, ":")
		if idx <= 0 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(trimmed[:idx])) != key {
			continue
		}
		return trimYAMLScalar(strings.TrimSpace(trimmed[idx+1:]))
	}
	return ""
}

func trimYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}

func augmentFrontmatter(body, relPath string) string {
	body = normalizeNewlines(body)
	fields := []string{
		"source_system: obsidian",
		"obsidian_path: " + yamlQuote(filepath.ToSlash(relPath)),
	}
	if folder := filepath.ToSlash(filepath.Dir(relPath)); folder != "." && folder != "" {
		fields = append(fields, "obsidian_folder: "+yamlQuote(folder))
	}
	if fm, rest, ok := splitFrontmatter(body); ok {
		lines := []string{"---"}
		lines = append(lines, fields...)
		if strings.TrimSpace(fm) != "" {
			lines = append(lines, strings.TrimRight(fm, "\n"))
		}
		lines = append(lines, "---")
		return strings.Join(lines, "\n") + "\n" + strings.TrimLeft(rest, "\n")
	}
	lines := []string{"---"}
	lines = append(lines, fields...)
	lines = append(lines, "---", "")
	return strings.Join(lines, "\n") + strings.TrimRight(body, "\n") + "\n"
}

func splitFrontmatter(body string) (frontmatter string, rest string, ok bool) {
	if !strings.HasPrefix(body, "---\n") {
		return "", body, false
	}
	remaining := body[len("---\n"):]
	idx := strings.Index(remaining, "\n---")
	if idx < 0 {
		return "", body, false
	}
	frontmatter = remaining[:idx]
	after := remaining[idx+len("\n---"):]
	if strings.HasPrefix(after, "\n") {
		after = after[1:]
	}
	return frontmatter, after, true
}

type attachmentRef struct {
	ResourceID   string
	RelationType string
	RawTarget    string
}

func referencedAssetIDs(note markdownFile, assetIDByRel map[string]string, assetIDByBase map[string][]string) ([]attachmentRef, []string) {
	seen := map[string]attachmentRef{}
	warnings := []string{}
	for _, candidate := range markdownlinks.Extract(note.Body) {
		raw := strings.TrimSpace(candidate.RawTarget)
		if raw == "" || isExternal(raw) || strings.HasPrefix(raw, "document://") || strings.HasPrefix(raw, "resource://") || strings.HasPrefix(raw, "mailto:") {
			continue
		}
		if looksLikeMarkdownNote(raw) {
			continue
		}
		resourceID, ambiguous := resolveAssetID(note.RelPath, raw, assetIDByRel, assetIDByBase)
		if ambiguous {
			warnings = append(warnings, fmt.Sprintf("ambiguous asset reference %q in %s", raw, note.RelPath))
			continue
		}
		if resourceID == "" {
			continue
		}
		relation := "link"
		if candidate.RelationType == "embed" {
			relation = "embed"
		}
		seen[resourceID] = attachmentRef{ResourceID: resourceID, RelationType: relation, RawTarget: raw}
	}
	refs := []attachmentRef{}
	for _, ref := range seen {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].ResourceID < refs[j].ResourceID })
	return refs, warnings
}

func resolveAssetID(noteRelPath, raw string, assetIDByRel map[string]string, assetIDByBase map[string][]string) (string, bool) {
	candidates := candidateAssetPaths(noteRelPath, raw)
	for _, candidate := range candidates {
		if id := assetIDByRel[normalizeVaultPath(candidate)]; id != "" {
			return id, false
		}
	}
	base := strings.ToLower(filepath.Base(normalizeVaultPath(raw)))
	ids := assetIDByBase[base]
	if len(ids) == 1 {
		return ids[0], false
	}
	if len(ids) > 1 {
		return "", true
	}
	return "", false
}

func candidateAssetPaths(noteRelPath, raw string) []string {
	raw = strings.TrimSpace(raw)
	if unescaped, err := url.PathUnescape(raw); err == nil {
		raw = unescaped
	}
	raw = strings.TrimPrefix(raw, "./")
	raw = strings.TrimPrefix(raw, "/")
	noteDir := filepath.ToSlash(filepath.Dir(noteRelPath))
	if noteDir == "." {
		noteDir = ""
	}
	out := []string{raw}
	if noteDir != "" {
		out = append(out, filepath.ToSlash(filepath.Join(noteDir, raw)))
	}
	return out
}

func looksLikeMarkdownNote(raw string) bool {
	ext := strings.ToLower(filepath.Ext(raw))
	return ext == "" || ext == ".md" || ext == ".markdown"
}

func isExternal(target string) bool {
	lower := strings.ToLower(strings.TrimSpace(target))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func assetMIMEType(path string) string {
	if mt := mime.TypeByExtension(filepath.Ext(path)); mt != "" {
		return mt
	}
	return "application/octet-stream"
}

func documentID(relPath string) string { return "doc_obsidian_" + safePathID(relPath) }
func resourceID(relPath string) string { return "res_obsidian_" + safePathID(relPath) }

func safePathID(relPath string) string {
	relPath = strings.ToLower(filepath.ToSlash(strings.TrimSpace(relPath)))
	base := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range base {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	id := strings.Trim(b.String(), "_")
	if id == "" {
		id = "item"
	}
	if len(id) > 80 {
		sum := sha1.Sum([]byte(relPath))
		id = strings.TrimRight(id[:60], "_") + "_" + hex.EncodeToString(sum[:])[:12]
	}
	return id
}

func normalizeVaultPath(path string) string {
	path = strings.TrimSpace(path)
	if unescaped, err := url.PathUnescape(path); err == nil {
		path = unescaped
	}
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	return strings.ToLower(path)
}

func normalizeNewlines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
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
	b, _ := json.Marshal(value)
	return string(b)
}
