package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// ImportConfig is the rename-on-import configuration a dry run emits and a
// real import validates. Renames apply to top-level notebook names from the
// archive; the new names must not collide (case-insensitively) with notebooks
// bound to other data sources.
type ImportConfig struct {
	Version   int               `json:"version"`
	Archive   string            `json:"archive,omitempty"`
	Renames   map[string]string `json:"renames"`
	Conflicts []string          `json:"conflicts,omitempty"` // informational: names that conflicted at dry-run time
	Creates   []string          `json:"creates,omitempty"`   // informational: notebook paths that will be created
	Merges    []string          `json:"merges,omitempty"`    // informational: existing plain notebooks that will be merged into
}

// ImportOptions controls one archive import.
type ImportOptions struct {
	CollectionID string
	Config       *ImportConfig
	DryRun       bool
}

// ImportReport summarizes a dry run or import.
type ImportReport struct {
	NotesSeen        int      `json:"notes_seen"`
	NotesImported    int      `json:"notes_imported"`
	NotesUpdated     int      `json:"notes_updated"`
	NotesUnchanged   int      `json:"notes_unchanged"`
	NotebooksCreated int      `json:"notebooks_created"`
	ResourcesCreated int      `json:"resources_created"`
	TagsApplied      int      `json:"tags_applied"`
	DryRun           bool     `json:"dry_run,omitempty"`
	Conflicts        []string `json:"conflicts,omitempty"`
	Creates          []string `json:"creates,omitempty"`
	Merges           []string `json:"merges,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

type archiveNote struct {
	ID        string
	Title     string
	Notebook  string // path from front matter
	Tags      []string
	Resources []string // "res_id|filename"
	Body      string
}

// DryRun analyzes an archive against the target store: which notebooks would
// be created, which existing notebooks would be merged into, and which names
// conflict with notebooks bound to other data sources (builtin notebooks or
// notebooks holding externally-sourced notes). It returns an ImportConfig
// prefilled with rename suggestions for every conflict.
func DryRun(ctx context.Context, st store.Store, archiveDir string) (ImportConfig, ImportReport, error) {
	report := ImportReport{DryRun: true}
	cfg := ImportConfig{Version: 1, Archive: archiveDir, Renames: map[string]string{}}

	notes, notebooks, _, err := loadArchive(archiveDir)
	if err != nil {
		return cfg, report, err
	}
	report.NotesSeen = len(notes)

	topLevel := topLevelNames(notes, notebooks)
	existingByName, err := existingTopLevel(ctx, st)
	if err != nil {
		return cfg, report, err
	}
	for _, name := range topLevel {
		existing, ok := existingByName[strings.ToLower(name)]
		if !ok {
			report.Creates = append(report.Creates, name)
			continue
		}
		sourced, err := notebookSourceBound(ctx, st, existing)
		if err != nil {
			return cfg, report, err
		}
		if sourced {
			report.Conflicts = append(report.Conflicts, name)
			cfg.Renames[name] = suggestRename(name, existingByName)
		} else {
			report.Merges = append(report.Merges, name)
		}
	}
	cfg.Conflicts = report.Conflicts
	cfg.Creates = report.Creates
	cfg.Merges = report.Merges
	return cfg, report, nil
}

// WriteConfig saves an import configuration file (default archive/import-config.json).
func WriteConfig(cfg ImportConfig, path string) error {
	return writeJSONFile(path, cfg)
}

// LoadConfig reads an import configuration file.
func LoadConfig(path string) (*ImportConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ImportConfig
	if err := jsonUnmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse import config %s: %w", path, err)
	}
	if cfg.Renames == nil {
		cfg.Renames = map[string]string{}
	}
	return &cfg, nil
}

// Import applies an archive. Renames from the config map top-level notebook
// names; the chosen names are validated against source-bound notebooks before
// anything is written. Imported notes are plain local notes (no provenance).
func Import(ctx context.Context, st store.Store, archiveDir string, options ImportOptions) (ImportReport, error) {
	report := ImportReport{}
	if strings.TrimSpace(options.CollectionID) == "" {
		options.CollectionID = "default"
	}
	if options.DryRun {
		_, dryReport, err := DryRun(ctx, st, archiveDir)
		return dryReport, err
	}
	renames := map[string]string{}
	if options.Config != nil {
		renames = options.Config.Renames
	}

	archiveRoot, err := os.OpenRoot(archiveDir)
	if err != nil {
		return report, err
	}
	defer archiveRoot.Close()
	notes, notebooks, resourceNames, err := loadArchiveRoot(archiveRoot)
	if err != nil {
		return report, err
	}
	report.NotesSeen = len(notes)

	// Validate every post-rename top-level name before writing anything.
	existingByName, err := existingTopLevel(ctx, st)
	if err != nil {
		return report, err
	}
	for _, name := range topLevelNames(notes, notebooks) {
		target := renameName(name, renames)
		if existing, ok := existingByName[strings.ToLower(target)]; ok {
			sourced, err := notebookSourceBound(ctx, st, existing)
			if err != nil {
				return report, err
			}
			if sourced {
				return report, fmt.Errorf("%w: notebook name %q conflicts with a notebook bound to another data source; rename it in the import configuration (dry run generates one)", store.ErrNameConflict, target)
			}
		}
	}

	// Notebooks: create missing paths (renamed at the top level).
	pathIDs := map[string]string{}
	emojiByPath := map[string]string{}
	for _, entry := range notebooks {
		emojiByPath[entry.Path] = entry.IconEmoji
	}
	allPaths := map[string]bool{}
	for _, entry := range notebooks {
		allPaths[entry.Path] = true
	}
	for _, note := range notes {
		if note.Notebook != "" {
			allPaths[note.Notebook] = true
		}
	}
	sortedPaths := make([]string, 0, len(allPaths))
	for path := range allPaths {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths) // parents sort before children
	for _, path := range sortedPaths {
		if _, err := ensurePath(ctx, st, path, renames, emojiByPath[path], pathIDs, &report); err != nil {
			return report, err
		}
	}

	// Resources first so note bodies' resource:// links resolve.
	if err := importResources(ctx, st, archiveRoot, resourceNames, options.CollectionID, &report); err != nil {
		return report, err
	}

	for _, note := range notes {
		if err := importNote(ctx, st, note, pathIDs, options, &report); err != nil {
			return report, err
		}
	}

	// Resolve links after every note exists. A note that links to another note
	// in the same archive is imported before its target as often as not, and
	// the link is recorded unresolved at that moment. Rebuilding here adds no
	// revision and is what makes an imported archive's internal links work —
	// including for a later publication, which rewrites unresolved links out of
	// the published bodies.
	for _, note := range notes {
		if err := st.RebuildDocumentLinks(ctx, note.ID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			return report, err
		}
	}
	return report, nil
}

func topLevelNames(notes []archiveNote, notebooks []NotebookEntry) []string {
	seen := map[string]string{}
	add := func(path string) {
		if path == "" {
			return
		}
		top := strings.SplitN(path, "/", 2)[0]
		seen[strings.ToLower(top)] = top
	}
	for _, entry := range notebooks {
		add(entry.Path)
	}
	for _, note := range notes {
		add(note.Notebook)
	}
	names := make([]string, 0, len(seen))
	for _, name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func existingTopLevel(ctx context.Context, st store.Store) (map[string]store.Notebook, error) {
	notebooks, err := st.ListNotebooks(ctx)
	if err != nil {
		return nil, err
	}
	byName := map[string]store.Notebook{}
	for _, nb := range notebooks {
		if nb.ParentID == "" {
			byName[strings.ToLower(nb.Name)] = nb
		}
	}
	return byName, nil
}

// notebookSourceBound: builtin notebooks and notebooks holding externally-
// sourced notes belong to their data source and must not absorb archive notes.
func notebookSourceBound(ctx context.Context, st store.Store, nb store.Notebook) (bool, error) {
	if nb.Builtin {
		return true, nil
	}
	return st.NotebookHasSourcedDocuments(ctx, nb.ID)
}

func suggestRename(name string, existing map[string]store.Notebook) string {
	for i := 1; i < 100; i++ {
		candidate := fmt.Sprintf("%s (imported)", name)
		if i > 1 {
			candidate = fmt.Sprintf("%s (imported %d)", name, i)
		}
		if _, ok := existing[strings.ToLower(candidate)]; !ok {
			return candidate
		}
	}
	return name + " (imported)"
}

func renameName(top string, renames map[string]string) string {
	for from, to := range renames {
		if strings.EqualFold(from, top) && strings.TrimSpace(to) != "" {
			return to
		}
	}
	return top
}

func renamePath(path string, renames map[string]string) string {
	if path == "" {
		return ""
	}
	parts := strings.SplitN(path, "/", 2)
	parts[0] = renameName(parts[0], renames)
	return strings.Join(parts, "/")
}

// ensurePath creates the notebook chain for a path, reusing existing
// notebooks by case-insensitive name at each level.
func ensurePath(ctx context.Context, st store.Store, rawPath string, renames map[string]string, emoji string, pathIDs map[string]string, report *ImportReport) (string, error) {
	path := renamePath(rawPath, renames)
	if id, ok := pathIDs[rawPath]; ok {
		return id, nil
	}
	notebooks, err := st.ListNotebooks(ctx)
	if err != nil {
		return "", err
	}
	childByParent := map[string]map[string]store.Notebook{}
	for _, nb := range notebooks {
		key := strings.ToLower(nb.Name)
		if childByParent[nb.ParentID] == nil {
			childByParent[nb.ParentID] = map[string]store.Notebook{}
		}
		childByParent[nb.ParentID][key] = nb
	}
	parentID := ""
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if existing, ok := childByParent[parentID][strings.ToLower(segment)]; ok {
			parentID = existing.ID
			continue
		}
		nbEmoji := ""
		if i == len(segments)-1 {
			nbEmoji = emoji
		}
		created, err := st.CreateNotebook(ctx, store.CreateNotebookRequest{ParentID: parentID, Name: segment, IconEmoji: nbEmoji})
		if err != nil {
			return "", err
		}
		report.NotebooksCreated++
		if childByParent[parentID] == nil {
			childByParent[parentID] = map[string]store.Notebook{}
		}
		childByParent[parentID][strings.ToLower(segment)] = created
		parentID = created.ID
	}
	pathIDs[rawPath] = parentID
	return parentID, nil
}

func importResources(ctx context.Context, st store.Store, root *os.Root, names []string, collectionID string, report *ImportReport) error {
	for _, name := range names {
		idx := strings.Index(name, "__")
		if idx <= 0 {
			report.Warnings = append(report.Warnings, "resource file without id prefix skipped: "+name)
			continue
		}
		resourceID, filename := name[:idx], name[idx+2:]
		if _, err := st.GetResource(ctx, resourceID); err == nil {
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		file, _, err := openArchiveRegular(root, "resources/"+name, store.MaxResourceContentBytes)
		if err != nil {
			return err
		}
		_, createErr := st.CreateResource(ctx, store.CreateResourceRequest{
			PreferredID: resourceID, CollectionID: collectionID,
			Filename: filename, MIMEType: firstNonEmpty(store.MIMETypeFromFilename(filename), "application/octet-stream"),
			Content: io.LimitReader(file, store.MaxResourceContentBytes+1),
		})
		closeErr := file.Close()
		if createErr != nil {
			return createErr
		}
		if closeErr != nil {
			return closeErr
		}
		report.ResourcesCreated++
	}
	return nil
}

func importNote(ctx context.Context, st store.Store, note archiveNote, pathIDs map[string]string, options ImportOptions, report *ImportReport) error {
	notebookID := pathIDs[note.Notebook]
	existing, err := st.GetDocument(ctx, note.ID)
	switch {
	case err == nil:
		if existing.Title == note.Title && existing.Body == note.Body {
			report.NotesUnchanged++
		} else {
			if _, err := st.UpdateDocument(ctx, store.UpdateDocumentRequest{
				ID: note.ID, Title: note.Title, Body: note.Body, BodyMIMEType: "text/markdown",
				BaseRevisionID: existing.CurrentRevisionID, Message: "import update from Notrios archive",
			}); err != nil {
				return err
			}
			report.NotesUpdated++
		}
	case errors.Is(err, store.ErrNotFound):
		if _, err := st.CreateDocument(ctx, store.CreateDocumentRequest{
			PreferredID: note.ID, CollectionID: options.CollectionID, NotebookID: notebookID,
			Title: note.Title, Body: note.Body, BodyMIMEType: "text/markdown",
			Message: "import from Notrios archive",
		}); err != nil {
			return err
		}
		report.NotesImported++
	default:
		return err
	}
	for _, tag := range note.Tags {
		if _, err := st.AddDocumentTag(ctx, note.ID, tag); err != nil {
			return err
		}
		report.TagsApplied++
	}
	for _, ref := range note.Resources {
		resourceID := strings.SplitN(ref, "|", 2)[0]
		if _, err := st.AttachDocumentResource(ctx, store.AttachResourceRequest{
			DocumentID: note.ID, ResourceID: resourceID, RelationType: "attachment",
		}); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
