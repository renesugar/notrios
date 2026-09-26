// Package obsidian imports an Obsidian Markdown vault into the managed store.
// The canonical notes contain stable Notrios links and source metadata; an
// optional source bundle preserves every discovered file byte-for-byte.
package obsidian

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/hashmeter"
	"github.com/renesugar/notrios/internal/markdownlinks"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncdelta"
)

const (
	sourceSystem        = "obsidian"
	defaultBatchSize    = 100
	maxBatchSize        = 500
	maxMarkdownBytes    = 64 * 1024 * 1024
	importConfigVersion = 1
)

type Options struct {
	CollectionID   string
	DryRun         bool
	PreserveSource bool
	BatchSize      int
	SourceKey      string
	Config         *ImportConfig
	// AfterBatch is called only after a durable checkpoint. It is useful to
	// display progress and to test interruption/resume behavior.
	AfterBatch func(phase string, processed, total int) error
}

// ImportConfig is emitted by a dry run and accepted by the real import.
// Rename keys are slash-separated vault folder paths; values replace only the
// leaf name under the same parent.
type ImportConfig struct {
	Version   int               `json:"version"`
	Source    string            `json:"source,omitempty"`
	Renames   map[string]string `json:"renames"`
	Conflicts []string          `json:"conflicts,omitempty"`
	Creates   []string          `json:"creates,omitempty"`
	Merges    []string          `json:"merges,omitempty"`
}

type Report struct {
	SourceDir            string        `json:"source_dir"`
	SourceKey            string        `json:"source_key"`
	CollectionID         string        `json:"collection_id"`
	DryRun               bool          `json:"dry_run"`
	Resumed              bool          `json:"resumed"`
	CheckpointStatus     string        `json:"checkpoint_status"`
	BatchesCompleted     int           `json:"batches_completed"`
	MarkdownSeen         int           `json:"markdown_seen"`
	NotesImported        int           `json:"notes_imported"`
	NotesUpdated         int           `json:"notes_updated"`
	NotesUnchanged       int           `json:"notes_unchanged"`
	NotebooksSeen        int           `json:"notebooks_seen"`
	NotebooksCreated     int           `json:"notebooks_created"`
	NotebooksUpdated     int           `json:"notebooks_updated"`
	NotebooksSkipped     int           `json:"notebooks_skipped"`
	NotebooksExisting    int           `json:"notebooks_existing"`
	NotebooksMerged      int           `json:"notebooks_merged"`
	NotebookConflicts    []string      `json:"notebook_conflicts,omitempty"`
	ResourcesSeen        int           `json:"resources_seen"`
	ResourcesImported    int           `json:"resources_imported"`
	ResourcesUpdated     int           `json:"resources_updated"`
	ResourcesExisting    int           `json:"resources_existing"`
	ResourcesSkipped     int           `json:"resources_skipped"`
	SourceBundleItems    int           `json:"source_bundle_items"`
	SourceBundleBytes    int64         `json:"source_bundle_bytes"`
	AttachmentsCreated   int           `json:"attachments_created"`
	LinksRewritten       int           `json:"links_rewritten"`
	LinkIndexesRefreshed int           `json:"link_indexes_refreshed"`
	Warnings             []string      `json:"warnings,omitempty"`
	SuggestedConfig      *ImportConfig `json:"suggested_config,omitempty"`
	DocumentIDs          []string      `json:"-"`
}

type vaultFile struct {
	RelPath        string
	ItemType       string
	Fingerprint    [sha256.Size]byte
	SizeBytes      int64
	Title          string
	Aliases        []string
	FrontmatterSHA [sha256.Size]byte
	PropertyOrder  []string
	NotebookPath   string
	TargetID       string
	MIMEType       string
}

type vaultFolder struct {
	RelPath     string
	Name        string
	ParentPath  string
	ItemKey     string
	Fingerprint string
	TargetID    string
}

type inventory struct {
	Files       []vaultFile
	Notes       []vaultFile
	Assets      []vaultFile
	Folders     []vaultFolder
	Fingerprint string
}

type folderPlan struct {
	Folder     vaultFolder
	TargetName string
	TargetID   string
	ParentID   string
	Action     string
}

type linkNamespace struct {
	notesByPath  map[string]string
	notesByName  map[string][]string
	assetsByPath map[string]string
	assetsByBase map[string][]string
}

type importRun struct {
	ctx         context.Context
	st          store.Store
	sourceDir   string
	options     Options
	inventory   inventory
	report      Report
	config      ImportConfig
	folderPlans []folderPlan
	folderIDs   map[string]string
	namespace   linkNamespace
	// noteBytes is the buffer the notes phase reads each note into, reused
	// between notes and given back when a note is unusually large (v1.0
	// J32-W2).
	noteBytes []byte
	phase     string
	nextIndex int
	workTotal int
	processed int
}

// Import applies the deterministic plan, resuming only a checkpoint whose
// inventory fingerprint exactly matches the current vault.
func Import(ctx context.Context, st store.Store, sourceDir string, options Options) (Report, error) {
	run, err := newImportRun(ctx, st, sourceDir, options)
	if err != nil {
		return Report{SourceDir: sourceDir, CollectionID: normalizedCollection(options.CollectionID), DryRun: options.DryRun}, err
	}
	if options.DryRun {
		run.report.SuggestedConfig = &run.config
		if err := run.runAll(false); err != nil {
			return run.report, err
		}
		run.report.CheckpointStatus = "dry-run"
		return run.report, nil
	}
	if len(run.report.NotebookConflicts) > 0 {
		return run.report, fmt.Errorf("%w: Obsidian folder paths conflict with source-bound or builtin notebooks; run --dry-run and apply the generated import configuration", store.ErrNameConflict)
	}
	if err := run.loadCheckpoint(); err != nil {
		return run.report, err
	}
	if err := run.runAll(true); err != nil {
		return run.report, err
	}
	run.report.CheckpointStatus = "completed"
	run.report.DocumentIDs, err = run.currentDocumentIDs()
	if err != nil {
		return run.report, err
	}
	if err := run.saveCheckpoint("done", 0, "completed"); err != nil {
		return run.report, err
	}
	return run.report, nil
}

// DryRun uses the same inventory, planner, and action classifiers as Import.
func DryRun(ctx context.Context, st store.Store, sourceDir string, options Options) (ImportConfig, Report, error) {
	options.DryRun = true
	run, err := newImportRun(ctx, st, sourceDir, options)
	if err != nil {
		return ImportConfig{Version: importConfigVersion, Source: sourceDir, Renames: map[string]string{}}, Report{}, err
	}
	run.report.SuggestedConfig = &run.config
	if err := run.runAll(false); err != nil {
		return run.config, run.report, err
	}
	run.report.CheckpointStatus = "dry-run"
	return run.config, run.report, nil
}

func WriteConfig(config ImportConfig, path string) error {
	if config.Renames == nil {
		config.Renames = map[string]string{}
	}
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".obsidian-import-config-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func LoadConfig(path string) (*ImportConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config ImportConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("parse Obsidian import config %s: %w", path, err)
	}
	if config.Version != importConfigVersion {
		return nil, fmt.Errorf("%w: unsupported Obsidian import config version %d", store.ErrInvalidInput, config.Version)
	}
	if config.Renames == nil {
		config.Renames = map[string]string{}
	}
	return &config, nil
}

func newImportRun(ctx context.Context, st store.Store, sourceDir string, options Options) (*importRun, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	absoluteDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absoluteDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: Obsidian source is not a directory", store.ErrInvalidInput)
	}
	options.CollectionID = normalizedCollection(options.CollectionID)
	if options.BatchSize == 0 {
		options.BatchSize = defaultBatchSize
	}
	if options.BatchSize < 1 || options.BatchSize > maxBatchSize {
		return nil, fmt.Errorf("%w: Obsidian batch size must be between 1 and %d", store.ErrInvalidInput, maxBatchSize)
	}
	if options.Config != nil && options.Config.Version != 0 && options.Config.Version != importConfigVersion {
		return nil, fmt.Errorf("%w: unsupported Obsidian import config version %d", store.ErrInvalidInput, options.Config.Version)
	}
	sourceKey := strings.TrimSpace(options.SourceKey)
	if sourceKey == "" {
		sourceKey = filepath.Clean(absoluteDir)
	}
	inv, err := readInventory(ctx, absoluteDir, options.PreserveSource)
	if err != nil {
		return nil, err
	}
	if err := applyLegacySourceIDs(ctx, st, &inv); err != nil {
		return nil, err
	}
	config, plans, folderIDs, err := analyzeFolderPlans(ctx, st, inv.Folders, sourceKey, options.CollectionID, options.Config)
	if err != nil {
		return nil, err
	}
	config.Source = absoluteDir
	inv.Fingerprint = importFingerprint(inv.Fingerprint, options.PreserveSource, config.Renames)
	run := &importRun{
		ctx: ctx, st: st, sourceDir: absoluteDir, options: options, inventory: inv,
		config: config, folderPlans: plans, folderIDs: folderIDs,
		namespace: buildLinkNamespace(inv),
		report: Report{
			SourceDir: absoluteDir, SourceKey: sourceKey, CollectionID: options.CollectionID,
			DryRun: options.DryRun, MarkdownSeen: len(inv.Notes), NotebooksSeen: len(inv.Folders),
			ResourcesSeen: len(inv.Assets), CheckpointStatus: "not-started",
		},
	}
	run.applyFolderReport()
	run.workTotal = len(inv.Folders) + len(inv.Assets) + len(inv.Notes) + len(inv.Notes)
	if options.PreserveSource {
		run.workTotal += len(inv.Files)
	}
	return run, nil
}

func importFingerprint(inventoryFingerprint string, preserveSource bool, renames map[string]string) string {
	hash := sha256.New()
	_, _ = io.WriteString(hash, inventoryFingerprint+"\x00preserve_source="+strconv.FormatBool(preserveSource)+"\x00")
	keys := make([]string, 0, len(renames))
	for key := range renames {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, _ = io.WriteString(hash, key+"\x00"+renames[key]+"\x00")
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func normalizedCollection(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "default"
}

// readInventory walks the vault once. retainFiles keeps a full per-file list
// in Files, which only the source-bundle phase reads. Every note and asset is
// otherwise already held in Notes or Assets, so an import that does not
// preserve source skips that second copy: ~160 MB at 382,206 notes (v1.0 J20).
// It also decides whether each note's frontmatter property order is parsed and
// kept, for the same reason: only the source-bundle phase reads it (v1.0
// J32-C). The inventory fingerprint covers every file either way.
func readInventory(ctx context.Context, root string, retainFiles bool) (inventory, error) {
	result := inventory{}
	hash := sha256.New()
	seenPaths := map[string]string{}
	seenTargets := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			if path == root {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			parent := filepath.ToSlash(filepath.Dir(rel))
			if parent == "." {
				parent = ""
			}
			fingerprint := sha256Hex([]byte("folder\x00" + rel))
			folder := vaultFolder{
				RelPath: rel, Name: filepath.Base(rel), ParentPath: parent,
				ItemKey: "folder:" + rel, Fingerprint: fingerprint,
				TargetID: "nb_obsidian_" + safeFolderPathID(rel),
			}
			if previous := seenTargets[folder.TargetID]; previous != "" {
				folder.TargetID += "_" + shortPathHash(rel)
			}
			seenTargets[folder.TargetID] = rel
			result.Folders = append(result.Folders, folder)
			_, _ = io.WriteString(hash, folder.ItemKey+"\x00"+fingerprint+"\x00")
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || entry.Type()&os.ModeSymlink != 0 {
			if entry.Type()&os.ModeSymlink != 0 {
				rel, _ := filepath.Rel(root, path)
				return fmt.Errorf("%w: Obsidian vault symlink %s is not allowed", store.ErrInvalidInput, filepath.ToSlash(rel))
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// filepath.Rel returns a slice of path, so without the clone RelPath
		// would keep the whole absolute path alive for the entire import.
		rel = strings.Clone(filepath.ToSlash(rel))
		normalized := normalizeVaultPath(rel)
		if previous := seenPaths[normalized]; previous != "" {
			return fmt.Errorf("%w: case-insensitive vault path collision between %s and %s", store.ErrInvalidInput, previous, rel)
		}
		seenPaths[normalized] = rel
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fingerprint, size, err := hashFile(ctx, path)
		if err != nil {
			return err
		}
		file := vaultFile{
			RelPath:     rel,
			Fingerprint: sha256FromHex(fingerprint), SizeBytes: size,
			NotebookPath: folderPath(rel),
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".md" || ext == ".markdown" {
			if info.Size() > maxMarkdownBytes {
				return fmt.Errorf("%w: Obsidian note %s exceeds the %d-byte limit", store.ErrInvalidInput, rel, maxMarkdownBytes)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			frontmatter, _, _ := splitFrontmatterBytes(raw)
			body := normalizeNewlines(string(raw))
			file.ItemType = "markdown"
			// These are substrings of a copy of the note's frontmatter or body.
			// A substring keeps its whole backing string alive, and the
			// inventory lives for the entire import, so without the clones
			// each note's text stays on the heap to keep a title and a few
			// property names. J19 measured it at 382,206 notes.
			file.Title = strings.Clone(markdownTitle(rel, body))
			file.Aliases = cloneStrings(frontmatterAliases(string(frontmatter)))
			file.FrontmatterSHA = sha256.Sum256(frontmatter)
			if retainFiles {
				// Only the source-bundle phase reads this, and that phase runs
				// only when source is preserved. An import that does not
				// preserve source parsed and kept a slice per note for nothing
				// (v1.0 J32-C). The inventory fingerprint is built from item
				// keys and content hashes, so gating it cannot move a
				// checkpoint.
				file.PropertyOrder = cloneStrings(frontmatterPropertyOrder(string(frontmatter)))
			}
			file.TargetID = documentID(rel)
			if previous := seenTargets[file.TargetID]; previous != "" {
				file.TargetID += "_" + shortPathHash(rel)
			}
			seenTargets[file.TargetID] = rel
			result.Notes = append(result.Notes, file)
		} else {
			file.ItemType = "asset"
			file.MIMEType = assetMIMEType(path)
			file.TargetID = resourceID(rel)
			if previous := seenTargets[file.TargetID]; previous != "" {
				file.TargetID += "_" + shortPathHash(rel)
			}
			seenTargets[file.TargetID] = rel
			if size > 0 {
				result.Assets = append(result.Assets, file)
			}
		}
		if retainFiles {
			result.Files = append(result.Files, file)
		}
		_, _ = io.WriteString(hash, file.itemKey()+"\x00"+file.fingerprintHex()+"\x00")
		return nil
	})
	if err != nil {
		return inventory{}, err
	}
	sort.Slice(result.Folders, func(i, j int) bool {
		leftDepth, rightDepth := strings.Count(result.Folders[i].RelPath, "/"), strings.Count(result.Folders[j].RelPath, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return result.Folders[i].RelPath < result.Folders[j].RelPath
	})
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].RelPath < result.Files[j].RelPath })
	sort.Slice(result.Notes, func(i, j int) bool { return result.Notes[i].RelPath < result.Notes[j].RelPath })
	sort.Slice(result.Assets, func(i, j int) bool { return result.Assets[i].RelPath < result.Assets[j].RelPath })
	result.Fingerprint = hex.EncodeToString(hash.Sum(nil))
	return result, nil
}

func applyLegacySourceIDs(ctx context.Context, st store.Store, inv *inventory) error {
	for start := 0; start < len(inv.Notes); start += maxBatchSize {
		end := min(start+maxBatchSize, len(inv.Notes))
		externalIDs := make([]string, 0, end-start)
		for _, note := range inv.Notes[start:end] {
			externalIDs = append(externalIDs, note.RelPath)
		}
		found, err := st.FindDocumentsBySourceIDs(ctx, sourceSystem, externalIDs)
		if err != nil {
			return err
		}
		for index := start; index < end; index++ {
			if targetID := found[inv.Notes[index].RelPath]; targetID != "" {
				inv.Notes[index].TargetID = targetID
			}
		}
	}
	noteIDs := map[string]string{}
	for _, note := range inv.Notes {
		noteIDs[note.RelPath] = note.TargetID
	}
	for index := range inv.Files {
		if targetID := noteIDs[inv.Files[index].RelPath]; targetID != "" {
			inv.Files[index].TargetID = targetID
		}
	}
	return nil
}

func analyzeFolderPlans(ctx context.Context, st store.Store, folders []vaultFolder, sourceKey, collectionID string, supplied *ImportConfig) (ImportConfig, []folderPlan, map[string]string, error) {
	config := ImportConfig{Version: importConfigVersion, Renames: map[string]string{}}
	if supplied != nil {
		for key, value := range supplied.Renames {
			config.Renames[filepath.ToSlash(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	existing, err := st.ListNotebooks(ctx)
	if err != nil {
		return config, nil, nil, err
	}
	byID := map[string]store.Notebook{}
	bySibling := map[string]store.Notebook{}
	for _, notebook := range existing {
		byID[notebook.ID] = notebook
		bySibling[notebook.ParentID+"\x00"+strings.ToLower(notebook.Name)] = notebook
	}
	keys := make([]string, len(folders))
	for index, folder := range folders {
		keys[index] = folder.ItemKey
	}
	states := map[string]store.ImportItemState{}
	for start := 0; start < len(keys); start += maxBatchSize {
		end := min(start+maxBatchSize, len(keys))
		batch, err := st.GetImportItemStates(ctx, sourceSystem, sourceKey, collectionID, keys[start:end])
		if err != nil {
			return config, nil, nil, err
		}
		for key, state := range batch {
			states[key] = state
		}
	}
	plans := make([]folderPlan, 0, len(folders))
	targetIDs := map[string]string{}
	for _, folder := range folders {
		targetName := firstNonEmpty(config.Renames[folder.RelPath], folder.Name, "Untitled")
		parentID := targetIDs[folder.ParentPath]
		plan := folderPlan{Folder: folder, TargetName: targetName, TargetID: folder.TargetID, ParentID: parentID, Action: "create"}
		previousTarget := states[folder.ItemKey].TargetID
		if notebook, ok := byID[previousTarget]; ok && previousTarget != "" {
			plan.TargetID = notebook.ID
			plan.Action = "existing"
			if notebook.Name != targetName || notebook.ParentID != parentID {
				plan.Action = "update"
			}
		} else if notebook, ok := byID[folder.TargetID]; ok {
			plan.TargetID = notebook.ID
			plan.Action = "existing"
			if notebook.Name != targetName || notebook.ParentID != parentID {
				plan.Action = "update"
			}
		} else if notebook, ok := bySibling[parentID+"\x00"+strings.ToLower(targetName)]; ok {
			sourced, err := st.NotebookHasSourcedDocuments(ctx, notebook.ID)
			if err != nil {
				return config, nil, nil, err
			}
			if notebook.Builtin || sourced {
				plan.Action = "conflict"
				config.Conflicts = append(config.Conflicts, folder.RelPath)
				if strings.TrimSpace(config.Renames[folder.RelPath]) == "" {
					config.Renames[folder.RelPath] = suggestFolderRename(targetName, parentID, bySibling)
				}
			} else {
				plan.Action = "merge"
				plan.TargetID = notebook.ID
				config.Merges = append(config.Merges, folder.RelPath)
			}
		} else {
			config.Creates = append(config.Creates, folder.RelPath)
		}
		targetIDs[folder.RelPath] = plan.TargetID
		if plan.Action != "conflict" {
			bySibling[parentID+"\x00"+strings.ToLower(targetName)] = store.Notebook{ID: plan.TargetID, ParentID: parentID, Name: targetName}
		}
		plans = append(plans, plan)
	}
	return config, plans, targetIDs, nil
}

func suggestFolderRename(name, parentID string, existing map[string]store.Notebook) string {
	base := name + " (Obsidian)"
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate += " " + strconv.Itoa(suffix)
		}
		if _, found := existing[parentID+"\x00"+strings.ToLower(candidate)]; !found {
			return candidate
		}
	}
}

func (run *importRun) applyFolderReport() {
	for _, plan := range run.folderPlans {
		switch plan.Action {
		case "create":
			run.report.NotebooksCreated++
		case "update":
			run.report.NotebooksUpdated++
			run.report.NotebooksExisting++
		case "merge":
			run.report.NotebooksMerged++
			run.report.NotebooksSkipped++
			run.report.NotebooksExisting++
		case "existing":
			run.report.NotebooksSkipped++
			run.report.NotebooksExisting++
		case "conflict":
			run.report.NotebookConflicts = append(run.report.NotebookConflicts, plan.Folder.RelPath)
		}
	}
}

func buildLinkNamespace(inv inventory) linkNamespace {
	ns := linkNamespace{
		notesByPath: map[string]string{}, notesByName: map[string][]string{},
		assetsByPath: map[string]string{}, assetsByBase: map[string][]string{},
	}
	for _, note := range inv.Notes {
		pathKey := normalizeNotePath(note.RelPath)
		ns.notesByPath[pathKey] = note.TargetID
		names := append([]string{note.Title, strings.TrimSuffix(filepath.Base(note.RelPath), filepath.Ext(note.RelPath))}, note.Aliases...)
		for _, name := range names {
			key := strings.ToLower(strings.TrimSpace(name))
			if key != "" {
				ns.notesByName[key] = appendUnique(ns.notesByName[key], note.TargetID)
			}
		}
	}
	for _, asset := range inv.Assets {
		ns.assetsByPath[normalizeVaultPath(asset.RelPath)] = asset.TargetID
		key := strings.ToLower(filepath.Base(asset.RelPath))
		ns.assetsByBase[key] = appendUnique(ns.assetsByBase[key], asset.TargetID)
	}
	for key := range ns.notesByName {
		sort.Strings(ns.notesByName[key])
	}
	for key := range ns.assetsByBase {
		sort.Strings(ns.assetsByBase[key])
	}
	return ns
}

// cloneStrings copies each string so the slice holds no reference into the
// larger string its values were cut from.
func cloneStrings(values []string) []string {
	for index, value := range values {
		values[index] = strings.Clone(value)
	}
	return values
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func (run *importRun) firstPhase() string {
	if run.options.PreserveSource {
		return "source_bundle"
	}
	return "notebooks"
}

func (run *importRun) loadCheckpoint() error {
	checkpoint, err := run.st.GetImportCheckpoint(run.ctx, sourceSystem, run.report.SourceKey, run.options.CollectionID)
	if errors.Is(err, store.ErrNotFound) {
		run.phase = run.firstPhase()
		run.report.CheckpointStatus = "running"
		return nil
	}
	if err != nil {
		return err
	}
	if checkpoint.InventoryFingerprint == run.inventory.Fingerprint && checkpoint.Status == "running" {
		var saved Report
		if err := json.Unmarshal([]byte(checkpoint.ReportJSON), &saved); err != nil {
			return fmt.Errorf("decode Obsidian import checkpoint: %w", err)
		}
		saved.Resumed = true
		saved.CheckpointStatus = "running"
		saved.SourceDir = run.sourceDir
		saved.SourceKey = run.report.SourceKey
		saved.CollectionID = run.options.CollectionID
		run.report = saved
		run.phase = checkpoint.Phase
		run.nextIndex = checkpoint.NextIndex
		run.processed = checkpoint.ProcessedItems
		return nil
	}
	run.phase = run.firstPhase()
	run.report.CheckpointStatus = "running"
	return nil
}

func (run *importRun) runAll(write bool) error {
	if !write {
		run.phase = run.firstPhase()
	}
	if run.phase == "done" {
		return nil
	}
	phases := []string{}
	if run.options.PreserveSource {
		phases = append(phases, "source_bundle")
	}
	phases = append(phases, "notebooks", "resources", "notes", "link_rebuild")
	start := -1
	for index, phase := range phases {
		if run.phase == phase {
			start = index
			break
		}
	}
	if start < 0 {
		return fmt.Errorf("%w: unknown Obsidian checkpoint phase %q", store.ErrInvalidInput, run.phase)
	}
	for index := start; index < len(phases); index++ {
		phase := phases[index]
		nextPhase := "done"
		if index+1 < len(phases) {
			nextPhase = phases[index+1]
		}
		var err error
		switch phase {
		case "source_bundle":
			err = run.processSourceBundle(write, nextPhase)
		case "notebooks":
			err = run.processNotebooks(write, nextPhase)
		case "resources":
			err = run.processResources(write, nextPhase)
		case "notes":
			err = run.processNotes(write, nextPhase)
		case "link_rebuild":
			err = run.processLinkRebuild(write, nextPhase)
		}
		if err != nil {
			return err
		}
		run.nextIndex = 0
		run.phase = nextPhase
	}
	return nil
}

func (run *importRun) processSourceBundle(write bool, nextPhase string) error {
	// A source bundle streams each file, but it reads one per item and commits
	// per batch, so the same budget keeps a batch of large files bounded.
	return run.eachSizedBatch("source_bundle", len(run.inventory.Files), func(index int) int64 {
		return run.inventory.Files[index].SizeBytes
	}, write, nextPhase, func(start, end int) error {
		for _, item := range run.inventory.Files[start:end] {
			if write {
				file, err := os.Open(run.absPath(item))
				if err != nil {
					return err
				}
				bundled, putErr := run.st.PutSourceBundleItem(run.ctx, store.PutSourceBundleItemRequest{
					SourceSystem: sourceSystem, SourceKey: run.report.SourceKey,
					CollectionID: run.options.CollectionID, ItemKey: item.itemKey(),
					ItemType: item.ItemType, ExternalID: item.RelPath,
					RelativePath: item.RelPath, PropertyOrder: item.PropertyOrder, Content: file,
				})
				closeErr := file.Close()
				if putErr != nil {
					return putErr
				}
				if closeErr != nil {
					return closeErr
				}
				if bundled.SHA256 != item.fingerprintHex() {
					return fmt.Errorf("%w: Obsidian file %s changed during source-bundle capture", store.ErrConflict, item.RelPath)
				}
			}
			run.report.SourceBundleItems++
			run.report.SourceBundleBytes += item.SizeBytes
		}
		return nil
	})
}

func (run *importRun) processNotebooks(write bool, nextPhase string) error {
	return run.eachBatch("notebooks", len(run.folderPlans), write, nextPhase, func(start, end int) error {
		if write {
			states := make([]store.ImportItemState, 0, end-start)
			for _, plan := range run.folderPlans[start:end] {
				switch plan.Action {
				case "create":
					if _, err := run.st.CreateNotebook(run.ctx, store.CreateNotebookRequest{PreferredID: plan.TargetID, ParentID: plan.ParentID, Name: plan.TargetName}); err != nil {
						return err
					}
				case "update":
					name, parentID := plan.TargetName, plan.ParentID
					if _, err := run.st.UpdateNotebook(run.ctx, store.UpdateNotebookRequest{ID: plan.TargetID, Name: &name, ParentID: &parentID}); err != nil {
						return err
					}
				}
				states = append(states, store.ImportItemState{
					SourceSystem: sourceSystem, SourceKey: run.report.SourceKey, CollectionID: run.options.CollectionID,
					ItemKey: plan.Folder.ItemKey, ItemType: "folder", Fingerprint: plan.Folder.Fingerprint,
					TargetID: plan.TargetID, Action: plan.Action,
				})
			}
			return run.st.PutImportItemStates(run.ctx, states)
		}
		return nil
	})
}

func (run *importRun) processResources(write bool, nextPhase string) error {
	return run.eachBatch("resources", len(run.inventory.Assets), write, nextPhase, func(start, end int) error {
		items := run.inventory.Assets[start:end]
		ids, keys := make([]string, 0, len(items)), make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.TargetID)
			keys = append(keys, item.itemKey())
		}
		existing, err := run.st.GetResources(run.ctx, ids)
		if err != nil {
			return err
		}
		states, err := run.st.GetImportItemStates(run.ctx, sourceSystem, run.report.SourceKey, run.options.CollectionID, keys)
		if err != nil {
			return err
		}
		newStates := make([]store.ImportItemState, 0, len(items))
		for _, item := range items {
			current, found := existing[item.TargetID]
			action := "create"
			if found {
				action = "update"
				if current.SHA256 == item.fingerprintHex() && current.Filename == filepath.Base(item.RelPath) && current.MIMEType == item.MIMEType {
					action = "unchanged"
				}
			}
			if state, ok := states[item.itemKey()]; ok && state.Fingerprint == item.fingerprintHex() && found &&
				current.SHA256 == item.fingerprintHex() && current.Filename == filepath.Base(item.RelPath) && current.MIMEType == item.MIMEType {
				action = "unchanged"
			}
			switch action {
			case "create":
				run.report.ResourcesImported++
			case "update":
				run.report.ResourcesUpdated++
			case "unchanged":
				run.report.ResourcesExisting++
			}
			if write && action != "unchanged" {
				actual, _, err := hashFile(run.ctx, run.absPath(item))
				if err != nil {
					return err
				}
				if actual != item.fingerprintHex() {
					return fmt.Errorf("%w: Obsidian asset %s changed during import", store.ErrConflict, item.RelPath)
				}
				file, err := os.Open(run.absPath(item))
				if err != nil {
					return err
				}
				if action == "create" {
					_, err = run.st.CreateResource(run.ctx, store.CreateResourceRequest{
						PreferredID: item.TargetID, CollectionID: run.options.CollectionID,
						Filename: filepath.Base(item.RelPath), MIMEType: item.MIMEType, Content: file,
					})
				} else {
					_, err = run.st.UpdateResource(run.ctx, store.UpdateResourceRequest{
						ID: item.TargetID, Filename: filepath.Base(item.RelPath), MIMEType: item.MIMEType, Content: file,
					})
				}
				closeErr := file.Close()
				if err != nil {
					return err
				}
				if closeErr != nil {
					return closeErr
				}
			}
			if write {
				newStates = append(newStates, run.itemState(item, action))
			}
		}
		if write {
			return run.st.PutImportItemStates(run.ctx, newStates)
		}
		return nil
	})
}

func (run *importRun) processNotes(write bool, nextPhase string) error {
	// The notes phase holds every canonical body of a batch at once, so this is
	// the phase the byte budget is for (v1.0 J32-K).
	return run.eachSizedBatch("notes", len(run.inventory.Notes), func(index int) int64 {
		return run.inventory.Notes[index].SizeBytes
	}, write, nextPhase, func(start, end int) error {
		items := run.inventory.Notes[start:end]
		ids, externalIDs, keys := make([]string, 0, len(items)), make([]string, 0, len(items)), make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.TargetID)
			externalIDs = append(externalIDs, item.RelPath)
			keys = append(keys, item.itemKey())
		}
		documents, err := run.st.GetDocuments(run.ctx, ids)
		if err != nil {
			return err
		}
		sourceDocuments, err := run.st.FindDocumentsBySourceIDs(run.ctx, sourceSystem, externalIDs)
		if err != nil {
			return err
		}
		states, err := run.st.GetImportItemStates(run.ctx, sourceSystem, run.report.SourceKey, run.options.CollectionID, keys)
		if err != nil {
			return err
		}
		mutations := make([]store.ImportDocumentMutation, 0, len(items))
		for _, item := range items {
			raw, err := run.readNote(item)
			if err != nil {
				return err
			}
			canonical, attachments, rewritten, warnings := run.canonicalBody(item, raw)
			run.report.LinksRewritten += rewritten
			for _, warning := range warnings {
				run.addWarning(warning)
			}
			notebookID := run.folderIDs[item.NotebookPath]
			if notebookID == "" {
				notebookID = store.DefaultNotebookID
			}
			targetID := item.TargetID
			current, found := documents[targetID]
			if mapped := sourceDocuments[item.RelPath]; mapped != "" && !found {
				targetID = mapped
				current, found = documents[mapped]
			}
			trashed := !found && sourceDocuments[item.RelPath] != ""
			fingerprint := noteFingerprint(item, notebookID, canonical)
			action := "create"
			if trashed {
				action = "unchanged"
			} else if found {
				action = "update"
				if current.Title == item.Title && current.Body == canonical && current.NotebookID == notebookID {
					action = "unchanged"
				}
				if state, ok := states[item.itemKey()]; ok && state.Fingerprint == fingerprint &&
					current.Title == item.Title && current.Body == canonical && current.NotebookID == notebookID {
					action = "unchanged"
				}
			}
			switch action {
			case "create":
				run.report.NotesImported++
			case "update":
				run.report.NotesUpdated++
			case "unchanged":
				run.report.NotesUnchanged++
			}
			run.report.AttachmentsCreated += len(attachments)
			if !write {
				continue
			}
			// Every note in the batch becomes one mutation, applied in one
			// transaction with the checkpoint. J17 measured the per-note calls
			// this replaces at three or more durable commits per note, and on
			// a disk that syncs each commit that waiting was 42% of the import.
			state := run.itemState(item, action)
			state.TargetID = targetID
			state.Fingerprint = fingerprint
			mutation := store.ImportDocumentMutation{
				Action: action,
				Document: store.CreateDocumentRequest{
					PreferredID: targetID, CollectionID: run.options.CollectionID, NotebookID: notebookID,
					Title: item.Title, Body: canonical, BodyMIMEType: "text/markdown",
					Message: "import from Obsidian vault",
				},
				State:        state,
				SkipDocument: trashed,
			}
			if action == "update" {
				mutation.Document.Message = "import update from Obsidian vault"
				mutation.BaseRevisionID = current.CurrentRevisionID
			}
			if !trashed {
				metadata, _ := json.Marshal(map[string]any{
					"relative_path": item.RelPath, "aliases": item.Aliases,
					"frontmatter_sha256": item.frontmatterSHAHex(),
				})
				mutation.Source = store.SetDocumentSourceRequest{
					DocumentID: targetID, SourceSystem: sourceSystem, ExternalID: item.RelPath, MetadataJSON: string(metadata),
				}
				for _, attachment := range attachments {
					mutation.Resources = append(mutation.Resources, store.AttachResourceRequest{
						DocumentID: targetID, ResourceID: attachment.ResourceID, RelationType: attachment.RelationType,
						AnchorJSON: fmt.Sprintf(`{"obsidian_path":%q,"raw_target":%q}`, item.RelPath, attachment.RawTarget),
					})
				}
			}
			mutations = append(mutations, mutation)
		}
		if !write || len(mutations) == 0 {
			return nil
		}
		checkpoint, err := run.batchCheckpoint("notes", end, len(run.inventory.Notes), nextPhase)
		if err != nil {
			return err
		}
		return run.st.ApplyImportDocumentBatch(run.ctx, store.ImportDocumentBatchRequest{Documents: mutations, Checkpoint: checkpoint})
	})
}

func (run *importRun) processLinkRebuild(write bool, nextPhase string) error {
	return run.eachBatch("link_rebuild", len(run.inventory.Notes), write, nextPhase, func(start, end int) error {
		if !write {
			return nil
		}
		ids := make([]string, 0, end-start)
		for _, note := range run.inventory.Notes[start:end] {
			ids = append(ids, note.TargetID)
		}
		// Presence, not documents: this pass refreshes link indexes and never
		// reads a body (v1.0 J32-H).
		existing, err := run.st.ExistingDocumentIDs(run.ctx, ids)
		if err != nil {
			return err
		}
		present := make([]string, 0, len(ids))
		for _, id := range ids {
			if existing[id] {
				present = append(present, id)
			}
		}
		if len(present) == 0 {
			return nil
		}
		run.report.LinkIndexesRefreshed += len(present)
		checkpoint, err := run.batchCheckpoint("link_rebuild", end, len(run.inventory.Notes), nextPhase)
		if err != nil {
			return err
		}
		return run.st.RebuildImportDocumentLinksBatch(run.ctx, store.ImportLinkBatchRequest{DocumentIDs: present, Checkpoint: checkpoint})
	})
}

func (run *importRun) canonicalBody(note vaultFile, raw []byte) (string, []attachmentRef, int, []string) {
	body := normalizeNewlines(string(raw))
	replacements, attachments, count, warnings := planObsidianLinks(body, note.RelPath, run.options.CollectionID, run.namespace)
	if canonical, ok := canonicalBodyOnce(body, note.RelPath, replacements); ok {
		return canonical, attachments, count, warnings
	}
	// The staged path, for the bodies the one-pass assembly declines: a link
	// rewrite that straddles the frontmatter delimiters, or replacements that
	// overlap each other (v1.0 J32-D, J32-U).
	return augmentFrontmatter(applyReplacements(body, replacements), note.RelPath), attachments, count, warnings
}

// canonicalBodyOnce writes a note's canonical body in one pass (v1.0 J32-U).
//
// canonicalBody builds it in stages, each a new string: the newlines are
// normalized, the links are rewritten into a second body, and the frontmatter
// is then rebuilt around a third. The link rewrites are already an edit list --
// J32-D made them one -- so the frontmatter is written as part of the same
// assembly rather than by a second pass over the result.
//
// It returns false when the edits cannot be applied this way, which is when a
// link rewrite crosses the frontmatter boundary; the caller then uses the
// staged path, because this slice is about how the bytes are assembled and not
// about what they say.
func canonicalBodyOnce(body, relPath string, replacements []replacement) (string, bool) {
	sortReplacements(replacements)
	if overlapping(replacements) {
		return "", false
	}

	fields := []string{"source_system: obsidian", "obsidian_path: " + yamlQuote(filepath.ToSlash(relPath))}
	if folder := folderPath(relPath); folder != "" {
		fields = append(fields, "obsidian_folder: "+yamlQuote(folder))
	}

	// The frontmatter's own bytes, and where the body after it begins.
	frontmatterStart, frontmatterEnd, restStart, hasFrontmatter := frontmatterRegion(body)
	if hasFrontmatter {
		// Trailing newlines of the frontmatter and leading newlines of the
		// rest are trimmed, exactly as augmentFrontmatter trims them.
		for frontmatterEnd > frontmatterStart && body[frontmatterEnd-1] == '\n' {
			frontmatterEnd--
		}
		for restStart < len(body) && body[restStart] == '\n' {
			restStart++
		}
	}
	if hasFrontmatter && crossesRegion(replacements, frontmatterStart, frontmatterEnd, restStart) {
		return "", false
	}

	// Sized exactly: the body, plus what the edits add, plus the delimiters and
	// the fields. A builder given slack allocated more than the staged path it
	// replaces on a small note with links (measured, v1.0 J32-U).
	size := len(body) + len("---\n---\n") + 1
	for _, field := range fields {
		size += len(field) + 1
	}
	for _, change := range replacements {
		size += len(change.text) - (change.end - change.start)
	}
	var out strings.Builder
	out.Grow(size)
	out.WriteString("---\n")
	for _, field := range fields {
		out.WriteString(field)
		out.WriteByte('\n')
	}
	if hasFrontmatter {
		if strings.TrimSpace(body[frontmatterStart:frontmatterEnd]) != "" {
			writeRegion(&out, body, frontmatterStart, frontmatterEnd, replacements)
			out.WriteByte('\n')
		}
		out.WriteString("---\n")
		writeRegion(&out, body, restStart, len(body), replacements)
		return out.String(), true
	}
	out.WriteString("---\n")
	end := len(body)
	for end > 0 && body[end-1] == '\n' {
		end--
	}
	writeRegion(&out, body, 0, end, replacements)
	out.WriteByte('\n')
	return out.String(), true
}

// writeRegion writes body[from:to] with the replacements that fall inside it.
func writeRegion(out *strings.Builder, body string, from, to int, replacements []replacement) {
	cursor := from
	for _, change := range replacements {
		if change.start < from || change.end > to {
			continue
		}
		out.WriteString(body[cursor:change.start])
		out.WriteString(change.text)
		cursor = change.end
	}
	out.WriteString(body[cursor:to])
}

// crossesRegion reports whether a replacement straddles the frontmatter
// delimiters, which the one-pass assembly cannot place.
func crossesRegion(replacements []replacement, frontmatterStart, frontmatterEnd, restStart int) bool {
	for _, change := range replacements {
		insideFrontmatter := change.start >= frontmatterStart && change.end <= frontmatterEnd
		insideRest := change.start >= restStart
		if !insideFrontmatter && !insideRest {
			return true
		}
	}
	return false
}

// frontmatterRegion locates a note's frontmatter by offset rather than by
// copying it out: where its content starts and ends, and where the body after
// the closing delimiter begins. It answers exactly what splitFrontmatterBytes
// answers, which the tests check on the same bodies.
func frontmatterRegion(body string) (start, end, restStart int, ok bool) {
	firstEnd := strings.IndexByte(body, '\n')
	if firstEnd < 0 || strings.TrimSuffix(body[:firstEnd], "\r") != "---" {
		return 0, 0, 0, false
	}
	contentStart := firstEnd + 1
	for lineStart := contentStart; lineStart <= len(body); {
		lineEnd := strings.IndexByte(body[lineStart:], '\n')
		next := len(body)
		if lineEnd >= 0 {
			lineEnd += lineStart
			next = lineEnd + 1
		} else {
			lineEnd = len(body)
		}
		if strings.TrimSuffix(body[lineStart:lineEnd], "\r") == "---" {
			return contentStart, lineStart, next, true
		}
		if next >= len(body) {
			break
		}
		lineStart = next
	}
	return 0, 0, 0, false
}

func sortReplacements(replacements []replacement) {
	sort.Slice(replacements, func(i, j int) bool {
		if replacements[i].start != replacements[j].start {
			return replacements[i].start < replacements[j].start
		}
		return replacements[i].end < replacements[j].end
	})
}

func overlapping(replacements []replacement) bool {
	for index := 1; index < len(replacements); index++ {
		if replacements[index].start < replacements[index-1].end {
			return true
		}
	}
	return false
}

// replacement is one link's rewritten text and the bytes of the body it
// stands for.
type replacement struct {
	start int
	end   int
	text  string
}

// rewriteObsidianLinks returns the body with its links rewritten. canonicalBody
// uses planObsidianLinks instead and applies the edits while it assembles.
func rewriteObsidianLinks(body, notePath, collectionID string, ns linkNamespace) (string, []attachmentRef, int, []string) {
	replacements, attachments, count, warnings := planObsidianLinks(body, notePath, collectionID, ns)
	return applyReplacements(body, replacements), attachments, count, warnings
}

// planObsidianLinks resolves a note's links and returns the edits that rewrite
// them, rather than a rewritten body: the caller decides when to apply them,
// which lets the canonical body be assembled in one pass (v1.0 J32-U).
func planObsidianLinks(body, notePath, collectionID string, ns linkNamespace) ([]replacement, []attachmentRef, int, []string) {
	replacements := []replacement{}
	attachmentByID := map[string]attachmentRef{}
	warnings := []string{}
	for _, candidate := range markdownlinks.Extract(body) {
		raw := strings.TrimSpace(candidate.RawTarget)
		if raw == "" || isExternal(raw) || strings.HasPrefix(raw, "document://") || strings.HasPrefix(raw, "resource://") || strings.HasPrefix(raw, "mailto:") {
			continue
		}
		target := ""
		isResource := false
		ambiguous := false
		if looksLikeMarkdownNote(raw) {
			target, ambiguous = resolveNoteID(notePath, raw, ns)
		}
		if target == "" && !ambiguous {
			target, ambiguous = resolveAssetID(notePath, raw, ns)
			isResource = target != ""
		}
		if ambiguous {
			warnings = append(warnings, fmt.Sprintf("ambiguous Obsidian link %q in %s", raw, notePath))
			continue
		}
		if target == "" {
			continue
		}
		uri := store.DocumentURI(collectionID, target)
		if isResource {
			uri = store.ResourceURI(collectionID, target)
			relation := "link"
			if candidate.RelationType == "embed" {
				relation = "embed"
			}
			attachmentByID[target] = attachmentRef{ResourceID: target, RelationType: relation, RawTarget: raw}
		}
		uri += anchorSuffix(candidate.AnchorType, candidate.AnchorValue)
		snippet := body[candidate.StartByte:candidate.EndByte]
		var replacementText string
		if candidate.SourceFormat == "obsidian-wikilink" {
			prefix := ""
			if candidate.RelationType == "embed" {
				prefix = "!"
			}
			inner := uri
			if strings.Contains(snippet, "|") {
				inner += "|" + candidate.DisplayText
			}
			replacementText = prefix + "[[" + inner + "]]"
		} else {
			prefix := ""
			if candidate.RelationType == "embed" {
				prefix = "!"
			}
			replacementText = prefix + "[" + candidate.DisplayText + "](" + uri + ")"
		}
		replacements = append(replacements, replacement{start: candidate.StartByte, end: candidate.EndByte, text: replacementText})
	}
	attachments := make([]attachmentRef, 0, len(attachmentByID))
	for _, attachment := range attachmentByID {
		attachments = append(attachments, attachment)
	}
	sort.Slice(attachments, func(i, j int) bool { return attachments[i].ResourceID < attachments[j].ResourceID })
	return replacements, attachments, len(replacements), warnings
}

// applyReplacements writes a note's rewritten body (v1.0 J32-D).
//
// It used to splice each replacement into the body in descending order:
// body[:start] + text + body[end:], once per link. Every splice copies the
// whole note, so a note with thousands of links copied itself thousands of
// times -- 11 GB allocated to import four 1 MiB notes (J32-A). This walks the
// body once instead, and the pinned cases in j32d_rewrite_test.go say the
// result is the same.
//
// Two replacements overlap only if a vault holds a file whose name is itself
// a wiki link, such as `[[Target]].md`, because then the Markdown match around
// a nested wiki link resolves too. A one-pass assembly cannot reproduce what
// the descending splices did to those bytes, so that case keeps the splices:
// this change is about how a body is assembled, not about what it says.
func applyReplacements(body string, replacements []replacement) string {
	if len(replacements) == 0 {
		return body
	}
	sort.Slice(replacements, func(i, j int) bool {
		if replacements[i].start != replacements[j].start {
			return replacements[i].start < replacements[j].start
		}
		return replacements[i].end < replacements[j].end
	})
	size := len(body)
	overlapping := false
	for index, change := range replacements {
		if index > 0 && change.start < replacements[index-1].end {
			overlapping = true
			break
		}
		size += len(change.text) - (change.end - change.start)
	}
	if overlapping {
		// Descending, as the splice loop ran. Its sort compared starts alone,
		// so two replacements starting at the same byte were ordered
		// arbitrarily; the widest is applied first here, which is the only
		// part of this path that is decided rather than reproduced.
		sort.Slice(replacements, func(i, j int) bool {
			if replacements[i].start != replacements[j].start {
				return replacements[i].start > replacements[j].start
			}
			return replacements[i].end > replacements[j].end
		})
		for _, change := range replacements {
			body = body[:change.start] + change.text + body[change.end:]
		}
		return body
	}
	var out strings.Builder
	out.Grow(size)
	cursor := 0
	for _, change := range replacements {
		out.WriteString(body[cursor:change.start])
		out.WriteString(change.text)
		cursor = change.end
	}
	out.WriteString(body[cursor:])
	return out.String()
}

func resolveNoteID(notePath, raw string, ns linkNamespace) (string, bool) {
	raw = decodeVaultTarget(raw)
	noteDir := folderPath(notePath)
	candidates := []string{}
	if strings.Contains(raw, "/") || strings.HasPrefix(raw, ".") {
		candidates = append(candidates, filepath.ToSlash(filepath.Join(noteDir, raw)), raw)
	} else {
		candidates = append(candidates, filepath.ToSlash(filepath.Join(noteDir, raw)))
	}
	for _, candidate := range candidates {
		if id := ns.notesByPath[normalizeNotePath(candidate)]; id != "" {
			return id, false
		}
	}
	key := strings.ToLower(strings.TrimSuffix(filepath.Base(raw), filepath.Ext(raw)))
	ids := ns.notesByName[key]
	if len(ids) == 1 {
		return ids[0], false
	}
	return "", len(ids) > 1
}

func resolveAssetID(notePath, raw string, ns linkNamespace) (string, bool) {
	raw = decodeVaultTarget(raw)
	noteDir := folderPath(notePath)
	for _, candidate := range []string{filepath.ToSlash(filepath.Join(noteDir, raw)), raw} {
		if id := ns.assetsByPath[normalizeVaultPath(candidate)]; id != "" {
			return id, false
		}
	}
	ids := ns.assetsByBase[strings.ToLower(filepath.Base(raw))]
	if len(ids) == 1 {
		return ids[0], false
	}
	return "", len(ids) > 1
}

func anchorSuffix(anchorType, value string) string {
	if value == "" {
		return ""
	}
	if anchorType == "block" {
		return "#^" + value
	}
	return "#" + value
}

type attachmentRef struct {
	ResourceID   string
	RelationType string
	RawTarget    string
}

// importBatchByteBudget bounds a batch by the bytes its notes hold, as well as
// by how many there are (v1.0 J32-K).
//
// The notes phase keeps each note's canonical body until the batch is written,
// so a batch's memory is the sum of its notes' sizes, and only the note count
// was bounded: a hundred notes of 60 MiB would ask for 6 GB. The budget is the
// size one note is already allowed to reach, so a large note becomes a batch of
// its own and nothing legal is refused.
const importBatchByteBudget = int64(maxMarkdownBytes)

// eachBatch walks a phase in windows, saving a checkpoint after each.
//
// A window ends at the count limit, or -- when the caller says how large its
// items are -- once the window holds importBatchByteBudget of them, whichever
// comes first. A window always holds at least one item, so a note larger than
// the budget still imports. A checkpoint records the index the next window
// starts at and not how large a window was, so varying the length changes
// nothing about resume.
func (run *importRun) eachBatch(phase string, total int, write bool, nextPhase string, process func(start, end int) error) error {
	return run.eachSizedBatch(phase, total, nil, write, nextPhase, process)
}

func (run *importRun) eachSizedBatch(phase string, total int, sizeOf func(index int) int64, write bool, nextPhase string, process func(start, end int) error) error {
	start := 0
	if run.phase == phase {
		start = run.nextIndex
	}
	for start < total {
		end := min(start+run.options.BatchSize, total)
		if sizeOf != nil {
			bytes := int64(0)
			for index := start; index < end; index++ {
				bytes += sizeOf(index)
				if bytes >= importBatchByteBudget {
					end = index + 1
					break
				}
			}
		}
		if err := process(start, end); err != nil {
			return err
		}
		run.processed += end - start
		run.report.BatchesCompleted++
		if write {
			checkpointPhase, checkpointIndex := phase, end
			if end == total {
				checkpointPhase, checkpointIndex = nextPhase, 0
			}
			if err := run.saveCheckpoint(checkpointPhase, checkpointIndex, "running"); err != nil {
				return err
			}
			if run.options.AfterBatch != nil {
				if err := run.options.AfterBatch(phase, end, total); err != nil {
					return err
				}
			}
		}
		start = end
	}
	if total == 0 && write {
		return run.saveCheckpoint(nextPhase, 0, "running")
	}
	return nil
}

func (run *importRun) saveCheckpoint(phase string, nextIndex int, status string) error {
	checkpoint, err := run.checkpoint(phase, nextIndex, status)
	if err != nil {
		return err
	}
	return run.st.PutImportCheckpoint(run.ctx, checkpoint)
}

func (run *importRun) checkpoint(phase string, nextIndex int, status string) (store.ImportCheckpoint, error) {
	raw, err := json.Marshal(run.report)
	if err != nil {
		return store.ImportCheckpoint{}, err
	}
	return store.ImportCheckpoint{
		SourceSystem: sourceSystem, SourceKey: run.report.SourceKey, CollectionID: run.options.CollectionID,
		InventoryFingerprint: run.inventory.Fingerprint, Phase: phase, NextIndex: nextIndex,
		TotalItems: run.workTotal, ProcessedItems: run.processed, Status: status, ReportJSON: string(raw),
	}, nil
}

// batchCheckpoint is the checkpoint a batch commits with its own writes, so a
// crash cannot leave the writes durable and the position behind them. It names
// the same phase and index eachBatch saves after the batch; that later save
// still runs, and carries the report counters the batch has just advanced.
func (run *importRun) batchCheckpoint(phase string, end, total int, nextPhase string) (store.ImportCheckpoint, error) {
	if end == total {
		return run.checkpoint(nextPhase, 0, "running")
	}
	return run.checkpoint(phase, end, "running")
}

func (run *importRun) currentDocumentIDs() ([]string, error) {
	result := []string{}
	for start := 0; start < len(run.inventory.Notes); start += maxBatchSize {
		end := min(start+maxBatchSize, len(run.inventory.Notes))
		ids := make([]string, 0, end-start)
		for _, note := range run.inventory.Notes[start:end] {
			ids = append(ids, note.TargetID)
		}
		existing, err := run.st.ExistingDocumentIDs(run.ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if existing[id] {
				result = append(result, id)
			}
		}
	}
	return result, nil
}

func (run *importRun) itemState(item vaultFile, action string) store.ImportItemState {
	return store.ImportItemState{
		SourceSystem: sourceSystem, SourceKey: run.report.SourceKey, CollectionID: run.options.CollectionID,
		ItemKey: item.itemKey(), ItemType: item.ItemType, Fingerprint: item.fingerprintHex(),
		TargetID: item.TargetID, Action: action,
	}
}

func (run *importRun) addWarning(message string) {
	const limit = 100
	if len(run.report.Warnings) < limit {
		run.report.Warnings = append(run.report.Warnings, message)
	} else if len(run.report.Warnings) == limit {
		run.report.Warnings = append(run.report.Warnings, "additional warnings omitted")
	}
}

func noteFingerprint(item vaultFile, notebookID, canonical string) string {
	// The canonical body is hashed from the string itself; converting it to
	// bytes copied the whole note, 240 MB of a near-limit import (v1.0
	// J32-W1). The outer hash is over a few dozen bytes and stays as it is.
	return sha256Hex([]byte(item.fingerprintHex() + "\x00" + notebookID + "\x00" + syncdelta.SHA256HexString(canonical)))
}

// fingerprintHex is the file's SHA-256 as lowercase hex, the form item states,
// resource records, bundle items and the inventory fingerprint were written
// with. The inventory holds the 32 bytes rather than the 64-character string,
// for every file for the whole import (v1.0 J20), and every comparison and
// composition goes through this method so the text it sees is unchanged.
func (file vaultFile) fingerprintHex() string {
	return hex.EncodeToString(file.Fingerprint[:])
}

// frontmatterSHAHex is the frontmatter's SHA-256 as lowercase hex, as recorded
// in a note source's metadata.
func (file vaultFile) frontmatterSHAHex() string {
	return hex.EncodeToString(file.FrontmatterSHA[:])
}

// sha256FromHex decodes a digest hashFile produced with hex.EncodeToString.
// That input is always 64 valid hex characters. Were it ever not, the zero
// value would make readExact refuse the file as changed during import, which
// fails loudly, not silently.
func sha256FromHex(value string) [sha256.Size]byte {
	var sum [sha256.Size]byte
	_, _ = hex.Decode(sum[:], []byte(value))
	return sum
}

// itemKey is the import item-state key for a vault file. It is derived from
// RelPath, not stored beside it, so the inventory does not hold a second copy
// of every path for the whole import (v1.0 J20). The composition is the one
// item states were written with, and must not change.
func (file vaultFile) itemKey() string {
	return "file:" + file.RelPath
}

// absPath is where a vault file lives on disk. The inventory holds only the
// slash-separated path relative to the vault, and readInventory walked that
// same cleaned root, so joining them rebuilds the path it read. Storing the
// absolute path as well kept a second, longer copy of every path for the whole
// import (v1.0 J20).
func (run *importRun) absPath(item vaultFile) string {
	return filepath.Join(run.sourceDir, filepath.FromSlash(item.RelPath))
}

// maxRetainedNoteBytes is how large a note buffer the import keeps between
// notes: the largest note the importer accepts at all, so every legal note is
// read into the same buffer (v1.0 J32-W2).
//
// It was 8 MiB first, on the rule J32-T set for the block buffers, and that
// measured flat on the near-limit corpus for the obvious reason -- every note
// there is 60 MiB, so every note allocated anyway. The bound that makes reuse
// mean something is the one the importer already enforces: a buffer this large
// is one a single note could legitimately need, and holding it between notes
// costs no more than reading one more such note would.
const maxRetainedNoteBytes = maxMarkdownBytes

// retainNoteBuffer reports whether a buffer of this capacity is kept for the
// next note. A buffer larger than any note the importer accepts was grown by a
// file that is being refused anyway, so it is given back.
func retainNoteBuffer(capacity int) bool {
	return capacity <= maxRetainedNoteBytes
}

// readNote reads one note into the import's own buffer and checks that it is
// still the note the inventory hashed (v1.0 J32-W2).
//
// os.ReadFile allocated a buffer per note, and the notes phase reads every note
// in the library. The buffer is reused instead, which is safe because what the
// batch keeps -- the canonical body, the attachment references, the warnings --
// is built from the string canonicalBody makes, not from these bytes.
func (run *importRun) readNote(item vaultFile) ([]byte, error) {
	path := run.absPath(item)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	wanted := item.SizeBytes
	buffer := run.noteBytes[:0]
	if int64(cap(buffer)) < wanted+1 {
		// One byte past the size the inventory recorded, so a note that grew
		// under us is read whole and caught by the hash below rather than
		// silently truncated.
		buffer = make([]byte, 0, wanted+1)
	}
	for {
		if len(buffer) == cap(buffer) {
			buffer = append(buffer, 0)[:len(buffer)]
		}
		count, readErr := file.Read(buffer[len(buffer):cap(buffer)])
		buffer = buffer[:len(buffer)+count]
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	if retainNoteBuffer(cap(buffer)) {
		run.noteBytes = buffer
	} else {
		run.noteBytes = nil
	}
	if sha256Hex(buffer) != item.fingerprintHex() {
		return nil, fmt.Errorf("%w: Obsidian note %s changed during import", store.ErrConflict, item.RelPath)
	}
	return buffer, nil
}

func hashFile(ctx context.Context, path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	buffer := make([]byte, 32*1024)
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return "", size, err
		}
		count, readErr := file.Read(buffer)
		if count > 0 {
			_, _ = hash.Write(buffer[:count])
			hashmeter.Add(count)
			size += int64(count)
		}
		if readErr == io.EOF {
			return hex.EncodeToString(hash.Sum(nil)), size, nil
		}
		if readErr != nil {
			return "", size, readErr
		}
	}
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", ".obsidian", ".trash", ".notrios", "node_modules":
		return true
	default:
		return false
	}
}

func markdownTitle(relPath, body string) string {
	// Both steps used to copy the whole note to answer a question about a few
	// of its bytes (v1.0 J32-O): splitFrontmatter converted it to []byte and
	// converted both halves back, and strings.Split built a slice of every
	// line -- about four million of them for a 60 MiB note -- to find the
	// first heading. The frontmatter is located by offset and the lines are
	// walked one at a time; both are slices of the body the caller holds.
	if start, end, _, ok := frontmatterRegion(body); ok {
		if title := frontmatterScalar(body[start:end], "title"); title != "" {
			return title
		}
	}
	for rest := body; rest != ""; {
		line, remainder, _ := strings.Cut(rest, "\n")
		rest = remainder
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "# ") {
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
		index := strings.Index(trimmed, ":")
		if index > 0 && strings.ToLower(strings.TrimSpace(trimmed[:index])) == key {
			return trimYAMLScalar(strings.TrimSpace(trimmed[index+1:]))
		}
	}
	return ""
}

func frontmatterAliases(frontmatter string) []string {
	aliases := []string{}
	lines := strings.Split(normalizeNewlines(frontmatter), "\n")
	inAliases := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		index := strings.Index(trimmed, ":")
		if index > 0 && !strings.HasPrefix(trimmed, "-") {
			key := strings.ToLower(strings.TrimSpace(trimmed[:index]))
			inAliases = key == "alias" || key == "aliases"
			if inAliases {
				value := strings.TrimSpace(trimmed[index+1:])
				if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
					for _, alias := range strings.Split(strings.Trim(value, "[]"), ",") {
						aliases = appendUnique(aliases, trimYAMLScalar(alias))
					}
				} else if value != "" {
					aliases = appendUnique(aliases, trimYAMLScalar(value))
				}
			}
			continue
		}
		if inAliases && strings.HasPrefix(trimmed, "-") {
			aliases = appendUnique(aliases, trimYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))))
		} else if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inAliases = false
		}
	}
	filtered := aliases[:0]
	for _, alias := range aliases {
		if alias != "" {
			filtered = append(filtered, alias)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func frontmatterPropertyOrder(frontmatter string) []string {
	order := []string{}
	for _, line := range strings.Split(normalizeNewlines(frontmatter), "\n") {
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '-' || line[0] == '#' {
			continue
		}
		if index := strings.Index(line, ":"); index > 0 {
			order = append(order, strings.TrimSpace(line[:index]))
		}
	}
	return order
}

func trimYAMLScalar(value string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
}

func augmentFrontmatter(body, relPath string) string {
	body = normalizeNewlines(body)
	fields := []string{"source_system: obsidian", "obsidian_path: " + yamlQuote(filepath.ToSlash(relPath))}
	if folder := folderPath(relPath); folder != "" {
		fields = append(fields, "obsidian_folder: "+yamlQuote(folder))
	}
	if fm, rest, ok := splitFrontmatter(body); ok {
		lines := append([]string{"---"}, fields...)
		if strings.TrimSpace(fm) != "" {
			lines = append(lines, strings.TrimRight(fm, "\n"))
		}
		lines = append(lines, "---")
		return strings.Join(lines, "\n") + "\n" + strings.TrimLeft(rest, "\n")
	}
	lines := append([]string{"---"}, fields...)
	lines = append(lines, "---", "")
	return strings.Join(lines, "\n") + strings.TrimRight(body, "\n") + "\n"
}

func splitFrontmatter(body string) (frontmatter, rest string, ok bool) {
	raw, remainder, ok := splitFrontmatterBytes([]byte(body))
	return string(raw), string(remainder), ok
}

func splitFrontmatterBytes(body []byte) (frontmatter, rest []byte, ok bool) {
	firstEnd := bytes.IndexByte(body, '\n')
	if firstEnd < 0 || string(bytes.TrimSuffix(body[:firstEnd], []byte{'\r'})) != "---" {
		return nil, body, false
	}
	contentStart := firstEnd + 1
	for lineStart := contentStart; lineStart <= len(body); {
		lineEnd := bytes.IndexByte(body[lineStart:], '\n')
		next := len(body)
		if lineEnd >= 0 {
			lineEnd += lineStart
			next = lineEnd + 1
		} else {
			lineEnd = len(body)
		}
		line := bytes.TrimSuffix(body[lineStart:lineEnd], []byte{'\r'})
		if string(line) == "---" {
			return body[contentStart:lineStart], body[next:], true
		}
		if next >= len(body) {
			break
		}
		lineStart = next
	}
	return nil, body, false
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
	if value := mime.TypeByExtension(filepath.Ext(path)); value != "" {
		return value
	}
	return "application/octet-stream"
}

func documentID(relPath string) string { return "doc_obsidian_" + safePathID(relPath) }
func resourceID(relPath string) string { return "res_obsidian_" + safePathID(relPath) }
func safeFolderPathID(relPath string) string {
	return safePathID(filepath.ToSlash(relPath) + "/_")
}

func safePathID(relPath string) string {
	relPath = strings.ToLower(filepath.ToSlash(strings.TrimSpace(relPath)))
	base := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	var builder strings.Builder
	lastUnderscore := false
	for _, runeValue := range base {
		ok := runeValue >= 'a' && runeValue <= 'z' || runeValue >= '0' && runeValue <= '9' || runeValue == '-' || runeValue == '_'
		if ok {
			builder.WriteRune(runeValue)
			lastUnderscore = false
		} else if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	id := strings.Trim(builder.String(), "_")
	if id == "" {
		id = "item"
	}
	if len(id) > 80 {
		id = strings.TrimRight(id[:60], "_") + "_" + shortPathHash(relPath)
	}
	return id
}

func shortPathHash(path string) string {
	sum := sha1.Sum([]byte(filepath.ToSlash(path)))
	return hex.EncodeToString(sum[:])[:12]
}

func normalizeVaultPath(path string) string {
	path = decodeVaultTarget(path)
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	return strings.ToLower(path)
}

func normalizeNotePath(path string) string {
	path = normalizeVaultPath(path)
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".md" || ext == ".markdown" {
		path = strings.TrimSuffix(path, ext)
	}
	return path
}

func decodeVaultTarget(path string) string {
	path = strings.TrimSpace(path)
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	return strings.TrimPrefix(path, "/")
}

func folderPath(relPath string) string {
	folder := filepath.ToSlash(filepath.Dir(relPath))
	if folder == "." {
		return ""
	}
	return folder
}

func normalizeNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func yamlQuote(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func sha256Hex(value []byte) string {
	hashmeter.Add(len(value))
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
