package joplinraw

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

const (
	defaultBatchSize = 100
	maxBatchSize     = 500
	maxRAWItemSize   = 64 * 1024 * 1024
)

// ImportConfig is emitted by a dry run and accepted by a real import.
// Rename keys are original slash-separated Joplin folder paths; values are
// replacement leaf names at the same parent.
type ImportConfig struct {
	Version   int               `json:"version"`
	Source    string            `json:"source,omitempty"`
	Renames   map[string]string `json:"renames"`
	Conflicts []string          `json:"conflicts,omitempty"`
	Creates   []string          `json:"creates,omitempty"`
	Merges    []string          `json:"merges,omitempty"`
}

type inventoryItem struct {
	parsedItem
	RelativePath  string
	ItemKey       string
	Fingerprint   string
	SizeBytes     int64
	PropertyOrder []string
	ContentPath   string
	ContentSHA    string
	ContentSize   int64
}

type inventory struct {
	Folders           map[string]inventoryItem
	Tags              map[string]string
	NoteIDs           map[string]string
	Resources         []inventoryItem
	Manifest          *store.ImportManifest
	Fingerprint       string
	NoteCount         int
	BundleCount       int
	MetadataFilesSeen int
	ItemsSeen         int
	ItemTypeCounts    map[string]int
	MalformedItems    int
	UnsupportedItems  int
	IgnoredFiles      int
}

type folderPlan struct {
	Item       inventoryItem
	SourcePath string
	TargetName string
	TargetID   string
	ParentID   string
	Action     string
}

type importRun struct {
	ctx             context.Context
	st              store.Store
	sourceDir       string
	options         Options
	inventory       inventory
	report          Report
	config          ImportConfig
	folderPlans     []folderPlan
	folderIDs       map[string]string
	noteIDMap       map[string]string
	resourceMap     map[string]string
	manifestCursors map[string]manifestCursor
	phase           string
	nextIndex       int
	workTotal       int
	processed       int
}

type manifestCursor struct {
	nextOffset int
	sortKey    string
}

func Import(ctx context.Context, st store.Store, sourceDir string, options Options) (Report, error) {
	run, err := newImportRun(ctx, st, sourceDir, options)
	if err != nil {
		return Report{SourceDir: sourceDir, CollectionID: normalizedCollection(options.CollectionID), DryRun: options.DryRun}, err
	}
	defer run.inventory.close()
	if options.DryRun {
		run.report.SuggestedConfig = &run.config
		if err := run.runAll(false); err != nil {
			return run.report, err
		}
		run.report.CheckpointStatus = "dry-run"
		return run.report, nil
	}
	if len(run.report.NotebookConflicts) > 0 {
		return run.report, fmt.Errorf("%w: Joplin notebook paths conflict with source-bound or builtin notebooks; run --dry-run and apply the generated import configuration", store.ErrNameConflict)
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

func (run *importRun) currentDocumentIDs() ([]string, error) {
	currentIDs := make([]string, 0, run.inventory.NoteCount)
	for start := 0; start < run.inventory.NoteCount; start += maxBatchSize {
		end := start + maxBatchSize
		if end > run.inventory.NoteCount {
			end = run.inventory.NoteCount
		}
		items, err := run.inventoryItems("notes", start, end)
		if err != nil {
			return nil, err
		}
		externalIDs := make([]string, 0, len(items))
		for _, item := range items {
			externalIDs = append(externalIDs, item.ID)
		}
		mapped, err := run.st.FindDocumentsBySourceIDs(run.ctx, "joplin", externalIDs)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(items))
		for _, item := range items {
			if documentID := mapped[item.ID]; documentID != "" {
				run.noteIDMap[item.ID] = documentID
			}
			ids = append(ids, run.noteIDMap[item.ID])
		}
		documents, err := run.st.GetDocuments(run.ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if _, found := documents[id]; found {
				currentIDs = append(currentIDs, id)
			}
		}
	}
	return currentIDs, nil
}

// DryRun returns both the rename configuration and the exact action report
// produced by the same planner used by Import.
func DryRun(ctx context.Context, st store.Store, sourceDir string, options Options) (ImportConfig, Report, error) {
	options.DryRun = true
	run, err := newImportRun(ctx, st, sourceDir, options)
	if err != nil {
		return ImportConfig{Version: 1, Source: sourceDir, Renames: map[string]string{}}, Report{}, err
	}
	defer run.inventory.close()
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
	tmp, err := os.CreateTemp(filepath.Dir(path), ".joplin-import-config-*")
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
		return nil, fmt.Errorf("parse Joplin import config %s: %w", path, err)
	}
	if config.Version != 1 {
		return nil, fmt.Errorf("%w: unsupported Joplin import config version %d", store.ErrInvalidInput, config.Version)
	}
	if config.Renames == nil {
		config.Renames = map[string]string{}
	}
	return &config, nil
}

func newImportRun(ctx context.Context, st store.Store, sourceDir string, options Options) (*importRun, error) {
	ctx = contextOrBackground(ctx)
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
		return nil, fmt.Errorf("%w: Joplin RAW source is not a directory", store.ErrInvalidInput)
	}
	options.CollectionID = normalizedCollection(options.CollectionID)
	if options.BatchSize <= 0 {
		options.BatchSize = defaultBatchSize
	}
	if options.BatchSize > maxBatchSize {
		return nil, fmt.Errorf("%w: batch size cannot exceed %d", store.ErrInvalidInput, maxBatchSize)
	}
	sourceKey := strings.TrimSpace(options.SourceKey)
	if sourceKey == "" {
		sourceKey = filepath.Clean(absoluteDir)
	}
	inv, err := readInventory(ctx, absoluteDir, options.PreserveSource)
	if err != nil {
		return nil, err
	}
	keepInventory := false
	defer func() {
		if !keepInventory {
			inv.close()
		}
	}()
	run := &importRun{
		ctx:             ctx,
		st:              st,
		sourceDir:       absoluteDir,
		options:         options,
		inventory:       inv,
		folderIDs:       map[string]string{},
		noteIDMap:       map[string]string{},
		resourceMap:     map[string]string{},
		manifestCursors: map[string]manifestCursor{},
		report: Report{
			SourceDir:         absoluteDir,
			SourceKey:         sourceKey,
			CollectionID:      options.CollectionID,
			DryRun:            options.DryRun,
			MetadataFilesSeen: inv.MetadataFilesSeen,
			ItemsSeen:         inv.ItemsSeen,
			ItemTypeCounts:    inv.ItemTypeCounts,
			MalformedItems:    inv.MalformedItems,
			UnsupportedItems:  inv.UnsupportedItems,
			IgnoredFiles:      inv.IgnoredFiles,
			ManifestBytes:     inv.Manifest.SizeBytes(),
			NotesSeen:         inv.NoteCount,
			NotebooksSeen:     len(inv.Folders),
			TagsSeen:          len(inv.Tags),
			ResourcesSeen:     len(inv.Resources),
			CheckpointStatus:  "not-started",
		},
	}
	if inv.MalformedItems > 0 {
		run.addWarning(fmt.Sprintf("%d Markdown metadata file(s) were malformed and skipped", inv.MalformedItems))
	}
	if inv.UnsupportedItems > 0 {
		run.addWarning(fmt.Sprintf("%d parsed Joplin item(s) used unsupported type values and were skipped", inv.UnsupportedItems))
	}
	for noteID, documentID := range inv.NoteIDs {
		run.noteIDMap[noteID] = documentID
	}
	for _, resource := range inv.Resources {
		run.resourceMap[resource.ID] = resourceID(resource.ID)
	}
	if options.Config != nil && options.Config.Version != 0 && options.Config.Version != 1 {
		return nil, fmt.Errorf("%w: unsupported Joplin import config version %d", store.ErrInvalidInput, options.Config.Version)
	}
	config, plans, ids, err := analyzeFolderPlans(ctx, st, inv.Folders, sourceKey, options.CollectionID, options.Config)
	if err != nil {
		return nil, err
	}
	run.config = config
	run.config.Source = absoluteDir
	run.folderPlans = plans
	run.folderIDs = ids
	run.applyFolderReport()
	run.workTotal = len(inv.Folders) + len(inv.Tags) + len(inv.Resources) + 2*inv.NoteCount
	if options.PreserveSource {
		run.workTotal += inv.BundleCount
	}
	keepInventory = true
	return run, nil
}

func (inv *inventory) close() {
	if inv != nil && inv.Manifest != nil {
		_ = inv.Manifest.Close()
		inv.Manifest = nil
	}
}

func normalizedCollection(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	return value
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func readInventory(ctx context.Context, sourceDir string, retainBundleInventory bool) (inventory, error) {
	manifest, err := store.OpenImportManifest()
	if err != nil {
		return inventory{}, err
	}
	result := inventory{
		Folders:        map[string]inventoryItem{},
		Tags:           map[string]string{},
		NoteIDs:        map[string]string{},
		Manifest:       manifest,
		ItemTypeCounts: map[string]int{},
	}
	keepManifest := false
	defer func() {
		if !keepManifest {
			_ = manifest.Close()
		}
	}()
	manifestRecords := make([]store.ImportManifestRecord, 0, maxBatchSize)
	flushManifest := func() error {
		if err := manifest.Put(ctx, manifestRecords); err != nil {
			return err
		}
		manifestRecords = manifestRecords[:0]
		return nil
	}
	appendManifest := func(record store.ImportManifestRecord) error {
		manifestRecords = append(manifestRecords, record)
		if len(manifestRecords) == cap(manifestRecords) {
			return flushManifest()
		}
		return nil
	}
	var inventoryHash [sha256.Size]byte
	seenIDs := map[string]string{}
	err = walkDirStreaming(sourceDir, func(path string, entry os.DirEntry) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		relativeParts := strings.Split(filepath.ToSlash(relative), "/")
		if len(relativeParts) > 1 && (relativeParts[0] == "resources" || relativeParts[0] == ".resource") {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: Joplin RAW metadata symlink %s is not allowed", store.ErrInvalidInput, filepath.ToSlash(relative))
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			result.IgnoredFiles++
			return nil
		}
		result.MetadataFilesSeen++
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxRAWItemSize {
			if strings.EqualFold(filepath.Ext(path), ".md") {
				return fmt.Errorf("%w: Joplin RAW item %s exceeds the %d-byte metadata limit", store.ErrInvalidInput, filepath.ToSlash(relative), maxRAWItemSize)
			}
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var parsed parsedItem
		var ok bool
		if retainBundleInventory {
			parsed, ok, err = parseItemBytes(path, raw)
		} else {
			parsed, ok, err = parseInventoryItemBytes(path, raw)
		}
		if err != nil {
			return err
		}
		if !ok {
			result.MalformedItems++
			return nil
		}
		result.ItemsSeen++
		result.ItemTypeCounts[parsed.Type]++
		if !supportedJoplinItemType(parsed.Type) {
			result.UnsupportedItems++
		}
		// Inventory retains only bounded routing/render metadata. Note bodies
		// and unknown properties are reread per batch; optional source bundles
		// preserve the exact original bytes.
		propertyOrder := parsed.PropertyOrder
		parsed.Body = ""
		parsed.Fields = compactInventoryFields(parsed.Fields)
		parsed.PropertyOrder = nil
		sum := sha256.Sum256(raw)
		item := inventoryItem{
			parsedItem:   parsed,
			RelativePath: filepath.ToSlash(relative),
			Fingerprint:  hex.EncodeToString(sum[:]),
			SizeBytes:    info.Size(),
		}
		if retainBundleInventory {
			item.PropertyOrder = append([]string(nil), propertyOrder...)
		}
		item.ItemKey = item.Type + ":" + item.ID + ":" + item.RelativePath
		if item.Type == "1" || item.Type == "2" || item.Type == "4" || item.Type == "5" {
			identity := item.Type + ":" + item.ID
			if previousPath := seenIDs[identity]; previousPath != "" {
				return fmt.Errorf("%w: duplicate Joplin item %s appears in %s and %s", store.ErrInvalidInput, identity, previousPath, item.RelativePath)
			}
			seenIDs[identity] = item.RelativePath
		}
		addInventoryFingerprint(&inventoryHash, item.ItemKey+"\x00"+item.Fingerprint+"\x00")
		switch item.Type {
		case "1":
			noteItem := item
			noteItem.Fields = nil
			payload, err := json.Marshal(noteItem)
			if err != nil {
				return err
			}
			if err := appendManifest(store.ImportManifestRecord{Kind: "notes", SortKey: item.ItemKey, LookupKey: item.ID, Payload: payload}); err != nil {
				return err
			}
			result.NoteIDs[item.ID] = noteDocumentID(item.ID)
			result.NoteCount++
		case "2":
			result.Folders[item.ID] = item
		case "4":
			if contentPath, ok := findResourceContent(sourceDir, item.parsedItem); ok {
				contentSHA, size, err := hashFile(ctx, contentPath)
				if err != nil {
					return err
				}
				item.ContentPath = contentPath
				item.ContentSHA = contentSHA
				item.ContentSize = size
				addInventoryFingerprint(&inventoryHash, item.ID+"\x00"+item.ContentSHA+"\x00")
			}
			result.Resources = append(result.Resources, item)
		case "5":
			result.Tags[item.ID] = strings.TrimSpace(item.Fields["title"])
		case "6":
			noteID := strings.TrimSpace(item.Fields["note_id"])
			tagID := strings.TrimSpace(item.Fields["tag_id"])
			if noteID != "" && tagID != "" {
				if err := appendManifest(store.ImportManifestRecord{
					Kind: "note_tags", SortKey: tagID + "\x1f" + item.ItemKey, LookupKey: noteID, Payload: []byte(tagID),
				}); err != nil {
					return err
				}
			}
		}
		if retainBundleInventory {
			bundleItem := item
			bundleItem.Fields = nil
			payload, err := json.Marshal(bundleItem)
			if err != nil {
				return err
			}
			if err := appendManifest(store.ImportManifestRecord{Kind: "bundle", SortKey: item.RelativePath, LookupKey: item.ID, Payload: payload}); err != nil {
				return err
			}
			result.BundleCount++
		}
		return nil
	})
	if err != nil {
		return inventory{}, err
	}
	if err := flushManifest(); err != nil {
		return inventory{}, err
	}
	sort.Slice(result.Resources, func(i, j int) bool { return result.Resources[i].ItemKey < result.Resources[j].ItemKey })
	result.Fingerprint = hex.EncodeToString(inventoryHash[:])
	keepManifest = true
	return result, nil
}

// walkDirStreaming avoids filepath.WalkDir's whole-directory name sort. RAW
// exports can contain more than a million entries in one directory, so even
// the directory enumeration itself must be bounded.
func walkDirStreaming(root string, visit func(path string, entry os.DirEntry) error) error {
	directories := []string{root}
	for len(directories) > 0 {
		directory := directories[len(directories)-1]
		directories = directories[:len(directories)-1]
		handle, err := os.Open(directory)
		if err != nil {
			return err
		}
		for {
			entries, readErr := handle.ReadDir(1024)
			for _, entry := range entries {
				path := filepath.Join(directory, entry.Name())
				if entry.IsDir() {
					relative, err := filepath.Rel(root, path)
					if err != nil {
						_ = handle.Close()
						return err
					}
					first := strings.SplitN(filepath.ToSlash(relative), "/", 2)[0]
					if first != "resources" && first != ".resource" {
						directories = append(directories, path)
					}
					continue
				}
				if err := visit(path, entry); err != nil {
					_ = handle.Close()
					return err
				}
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				_ = handle.Close()
				return readErr
			}
		}
		if err := handle.Close(); err != nil {
			return err
		}
	}
	return nil
}

// addInventoryFingerprint computes a stable multiset digest without retaining
// or sorting every source name. SHA-256 digests are added modulo 2^256, so the
// result is independent of filesystem enumeration order and preserves count.
func addInventoryFingerprint(accumulator *[sha256.Size]byte, value string) {
	digest := sha256.Sum256([]byte(value))
	carry := uint16(0)
	for index := sha256.Size - 1; index >= 0; index-- {
		total := uint16(accumulator[index]) + uint16(digest[index]) + carry
		accumulator[index] = byte(total)
		carry = total >> 8
	}
}

func compactInventoryFields(fields map[string]string) map[string]string {
	compact := make(map[string]string, 16)
	for _, key := range []string{
		"id", "type_", "title", "parent_id", "note_id", "tag_id",
		"filename", "file_extension", "mime", "mime_type",
		"author", "source_url", "created_time", "updated_time",
		"user_created_time", "user_updated_time",
	} {
		if value, ok := fields[key]; ok {
			compact[key] = value
		}
	}
	return compact
}

func supportedJoplinItemType(itemType string) bool {
	switch itemType {
	case "1", "2", "4", "5", "6":
		return true
	default:
		return false
	}
}

func hashFile(ctx context.Context, path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	var size int64
	buffer := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", size, err
		}
		count, readErr := file.Read(buffer)
		if count > 0 {
			_, _ = hash.Write(buffer[:count])
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

func analyzeFolderPlans(ctx context.Context, st store.Store, folders map[string]inventoryItem, sourceKey, collectionID string, supplied *ImportConfig) (ImportConfig, []folderPlan, map[string]string, error) {
	config := ImportConfig{Version: 1, Renames: map[string]string{}}
	if supplied != nil {
		for key, value := range supplied.Renames {
			config.Renames[key] = strings.TrimSpace(value)
		}
	}
	paths := map[string]string{}
	var resolvePath func(string, map[string]bool) string
	resolvePath = func(id string, visiting map[string]bool) string {
		if path := paths[id]; path != "" {
			return path
		}
		item, ok := folders[id]
		if !ok || visiting[id] {
			return ""
		}
		visiting[id] = true
		parentPath := resolvePath(strings.TrimSpace(item.Fields["parent_id"]), visiting)
		delete(visiting, id)
		name := firstNonEmpty(item.Fields["title"], item.ID, "Untitled")
		if parentPath == "" {
			paths[id] = name
		} else {
			paths[id] = parentPath + "/" + name
		}
		return paths[id]
	}
	ordered := make([]inventoryItem, 0, len(folders))
	for id, item := range folders {
		resolvePath(id, map[string]bool{})
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool {
		left, right := paths[ordered[i].ID], paths[ordered[j].ID]
		leftDepth, rightDepth := strings.Count(left, "/"), strings.Count(right, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		if left != right {
			return left < right
		}
		return ordered[i].ID < ordered[j].ID
	})
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
	folderKeys := make([]string, 0, len(ordered))
	for _, item := range ordered {
		folderKeys = append(folderKeys, item.ItemKey)
	}
	folderStates := map[string]store.ImportItemState{}
	for start := 0; start < len(folderKeys); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(folderKeys) {
			end = len(folderKeys)
		}
		batch, err := st.GetImportItemStates(ctx, sourceSystem, sourceKey, collectionID, folderKeys[start:end])
		if err != nil {
			return config, nil, nil, err
		}
		for key, state := range batch {
			folderStates[key] = state
		}
	}
	plans := make([]folderPlan, 0, len(ordered))
	targetIDs := map[string]string{}
	for _, item := range ordered {
		sourcePath := paths[item.ID]
		targetName := firstNonEmpty(config.Renames[sourcePath], item.Fields["title"], item.ID, "Untitled")
		parentID := targetIDs[strings.TrimSpace(item.Fields["parent_id"])]
		preferredID := "nb_joplin_" + safeID(item.ID)
		plan := folderPlan{Item: item, SourcePath: sourcePath, TargetName: targetName, TargetID: preferredID, ParentID: parentID, Action: "create"}
		previousTarget := folderStates[item.ItemKey].TargetID
		if notebook, ok := byID[previousTarget]; ok && previousTarget != "" {
			plan.TargetID = notebook.ID
			if notebook.Name != targetName || notebook.ParentID != parentID {
				plan.Action = "update"
			} else {
				plan.Action = "existing"
			}
		} else if notebook, ok := byID[preferredID]; ok {
			plan.TargetID = notebook.ID
			if notebook.Name != targetName || notebook.ParentID != parentID {
				plan.Action = "update"
			} else {
				plan.Action = "existing"
			}
		} else if notebook, ok := bySibling[parentID+"\x00"+strings.ToLower(targetName)]; ok {
			sourced, err := st.NotebookHasSourcedDocuments(ctx, notebook.ID)
			if err != nil {
				return config, nil, nil, err
			}
			if notebook.Builtin || sourced {
				plan.Action = "conflict"
				config.Conflicts = append(config.Conflicts, sourcePath)
				if strings.TrimSpace(config.Renames[sourcePath]) == "" {
					config.Renames[sourcePath] = suggestFolderRename(targetName, parentID, bySibling)
				}
			} else {
				plan.Action = "merge"
				plan.TargetID = notebook.ID
				config.Merges = append(config.Merges, sourcePath)
			}
		} else {
			config.Creates = append(config.Creates, sourcePath)
		}
		if plan.Action != "conflict" {
			targetIDs[item.ID] = plan.TargetID
			bySibling[parentID+"\x00"+strings.ToLower(targetName)] = store.Notebook{ID: plan.TargetID, ParentID: parentID, Name: targetName}
		} else {
			// A dry run can still plan note placement under the suggested,
			// deterministic notebook ID without mutating the store.
			targetIDs[item.ID] = preferredID
		}
		plans = append(plans, plan)
	}
	config.Source = ""
	return config, plans, targetIDs, nil
}

func suggestFolderRename(name, parentID string, existing map[string]store.Notebook) string {
	base := name + " (Joplin)"
	candidate := base
	for suffix := 2; ; suffix++ {
		if _, found := existing[parentID+"\x00"+strings.ToLower(candidate)]; !found {
			return candidate
		}
		candidate = base + " " + strconv.Itoa(suffix)
	}
}

func (run *importRun) applyFolderReport() {
	for _, plan := range run.folderPlans {
		switch plan.Action {
		case "create":
			run.report.NotebooksCreated++
		case "merge":
			run.report.NotebooksMerged++
			run.report.NotebooksSkipped++
			run.report.NotebooksExisting++
		case "existing":
			run.report.NotebooksSkipped++
			run.report.NotebooksExisting++
		case "update":
			run.report.NotebooksUpdated++
			run.report.NotebooksExisting++
		case "conflict":
			run.report.NotebookConflicts = append(run.report.NotebookConflicts, plan.SourcePath)
		}
	}
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
			return fmt.Errorf("decode Joplin import checkpoint: %w", err)
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

func (run *importRun) firstPhase() string {
	if run.options.PreserveSource {
		return "source_bundle"
	}
	return "notebooks"
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
	phases = append(phases, "notebooks", "tags", "resources", "notes", "links")
	start := -1
	for i, phase := range phases {
		if phase == run.phase {
			start = i
			break
		}
	}
	if start < 0 {
		return fmt.Errorf("%w: unknown Joplin checkpoint phase %q", store.ErrInvalidInput, run.phase)
	}
	for index := start; index < len(phases); index++ {
		phase := phases[index]
		nextPhase := "done"
		if index+1 < len(phases) {
			nextPhase = phases[index+1]
		}
		switch phase {
		case "source_bundle":
			if err := run.processSourceBundle(write, nextPhase); err != nil {
				return err
			}
		case "notebooks":
			if err := run.processNotebooks(write, nextPhase); err != nil {
				return err
			}
		case "tags":
			if err := run.processTags(write, nextPhase); err != nil {
				return err
			}
		case "resources":
			if err := run.processResources(write, nextPhase); err != nil {
				return err
			}
		case "notes":
			if err := run.processNotes(write, nextPhase); err != nil {
				return err
			}
		case "links":
			if err := run.processLinks(write, nextPhase); err != nil {
				return err
			}
		}
		run.nextIndex = 0
		run.phase = nextPhase
	}
	return nil
}

func (run *importRun) processSourceBundle(write bool, nextPhase string) error {
	return run.eachBatch("source_bundle", run.inventory.BundleCount, write, nextPhase, func(start, end int) error {
		items, err := run.inventoryItems("bundle", start, end)
		if err != nil {
			return err
		}
		for _, item := range items {
			if write {
				file, err := os.Open(item.Path)
				if err != nil {
					return err
				}
				bundled, putErr := run.st.PutSourceBundleItem(run.ctx, store.PutSourceBundleItemRequest{
					SourceSystem:  sourceSystem,
					SourceKey:     run.report.SourceKey,
					CollectionID:  run.options.CollectionID,
					ItemKey:       item.ItemKey,
					ItemType:      item.Type,
					ExternalID:    item.ID,
					RelativePath:  item.RelativePath,
					PropertyOrder: item.PropertyOrder,
					Content:       file,
				})
				closeErr := file.Close()
				if putErr != nil {
					return putErr
				}
				if closeErr != nil {
					return closeErr
				}
				if bundled.SHA256 != item.Fingerprint {
					return fmt.Errorf("%w: Joplin RAW item %s changed during source-bundle capture", store.ErrConflict, item.RelativePath)
				}
			}
			run.report.SourceBundleItems++
			run.report.SourceBundleBytes += item.SizeBytes
		}
		return nil
	})
}

func (run *importRun) processNotebooks(write bool, nextPhase string) error {
	if run.nextIndex > 0 {
		return nil
	}
	if write {
		states := make([]store.ImportItemState, 0, run.options.BatchSize)
		flushStates := func() error {
			if len(states) == 0 {
				return nil
			}
			if err := run.st.PutImportItemStates(run.ctx, states); err != nil {
				return err
			}
			states = states[:0]
			return nil
		}
		for _, plan := range run.folderPlans {
			switch plan.Action {
			case "create":
				if _, err := run.st.CreateNotebook(run.ctx, store.CreateNotebookRequest{
					PreferredID: plan.TargetID,
					ParentID:    plan.ParentID,
					Name:        plan.TargetName,
				}); err != nil {
					return err
				}
			case "update":
				name, parentID := plan.TargetName, plan.ParentID
				if _, err := run.st.UpdateNotebook(run.ctx, store.UpdateNotebookRequest{
					ID: plan.TargetID, Name: &name, ParentID: &parentID,
				}); err != nil {
					return err
				}
			}
			states = append(states, run.itemState(plan.Item, plan.TargetID, plan.Action, plan.Item.Fingerprint))
			if len(states) == cap(states) {
				if err := flushStates(); err != nil {
					return err
				}
			}
		}
		if err := flushStates(); err != nil {
			return err
		}
	}
	run.processed += len(run.folderPlans)
	return run.finishSinglePhase("notebooks", len(run.folderPlans), write, nextPhase)
}

func (run *importRun) processTags(write bool, nextPhase string) error {
	if run.nextIndex > 0 {
		return nil
	}
	existing, err := run.st.ListTags(run.ctx, store.TagQuery{})
	if err != nil {
		return err
	}
	byID := map[string]store.Tag{}
	byName := map[string]store.Tag{}
	for _, tag := range existing.Tags {
		byID[tag.ID] = tag
		byName[strings.ToLower(tag.Name)] = tag
	}
	tagIDs := make([]string, 0, len(run.inventory.Tags))
	for id := range run.inventory.Tags {
		tagIDs = append(tagIDs, id)
	}
	sort.Strings(tagIDs)
	for _, sourceID := range tagIDs {
		name := strings.TrimSpace(run.inventory.Tags[sourceID])
		if name == "" {
			continue
		}
		preferredID := "tag_joplin_" + safeID(sourceID)
		action := "create"
		targetID := preferredID
		oldName := ""
		if current, found := byID[preferredID]; found {
			action = "skip"
			oldName = current.Name
			if current.Name != name {
				action = "update"
			}
		} else if current, found := byName[strings.ToLower(name)]; found {
			action = "merge"
			targetID = current.ID
		}
		switch action {
		case "create":
			run.report.TagsCreated++
		case "update":
			run.report.TagsUpdated++
			run.report.TagsExisting++
		case "skip", "merge":
			run.report.TagsExisting++
			run.report.TagsSkipped++
		}
		if write {
			tag, actualAction, err := run.st.UpsertTag(run.ctx, preferredID, name)
			if err != nil {
				return err
			}
			if actualAction != action {
				return fmt.Errorf("%w: tag plan changed from %s to %s", store.ErrConflict, action, actualAction)
			}
			targetID = tag.ID
		}
		tag := store.Tag{ID: targetID, Name: name}
		if action == "update" {
			delete(byName, strings.ToLower(oldName))
		}
		byID[targetID] = tag
		byName[strings.ToLower(name)] = tag
	}
	run.processed += len(run.inventory.Tags)
	return run.finishSinglePhase("tags", len(run.inventory.Tags), write, nextPhase)
}

func (run *importRun) processResources(write bool, nextPhase string) error {
	return run.eachBatch("resources", len(run.inventory.Resources), write, nextPhase, func(start, end int) error {
		items := run.inventory.Resources[start:end]
		ids := make([]string, 0, len(items))
		keys := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, run.resourceMap[item.ID])
			keys = append(keys, item.ItemKey)
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
			targetID := run.resourceMap[item.ID]
			fingerprint := resourceFingerprint(item)
			if item.ContentPath == "" {
				run.report.ResourcesSkipped++
				run.report.ResourcesMissing++
				run.addWarning(fmt.Sprintf("resource %s has no content file", item.ID))
				if write {
					newStates = append(newStates, run.itemState(item, targetID, "skipped", fingerprint))
				}
				continue
			}
			filename := resourceFilename(item.parsedItem, item.ContentPath)
			declaredMIME := firstNonEmpty(item.Fields["mime"], item.Fields["mime_type"], mime.TypeByExtension(filepath.Ext(filename)))
			mimeType := firstNonEmpty(declaredMIME, "application/octet-stream")
			current, found := existing[targetID]
			action := "create"
			if found {
				action = "update"
				mimeMatches := declaredMIME == "" || current.MIMEType == mimeType
				if current.SHA256 == item.ContentSHA && current.Filename == filename && mimeMatches {
					action = "unchanged"
				}
			}
			if state, ok := states[item.ItemKey]; ok && state.Fingerprint == fingerprint && found &&
				current.SHA256 == item.ContentSHA && current.Filename == filename &&
				(declaredMIME == "" || current.MIMEType == mimeType) {
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
				actualSHA, _, err := hashFile(run.ctx, item.ContentPath)
				if err != nil {
					return err
				}
				if actualSHA != item.ContentSHA {
					return fmt.Errorf("%w: Joplin resource %s changed during import", store.ErrConflict, item.RelativePath)
				}
				file, err := os.Open(item.ContentPath)
				if err != nil {
					return err
				}
				if action == "create" {
					_, err = run.st.CreateResource(run.ctx, store.CreateResourceRequest{
						PreferredID: targetID, CollectionID: run.options.CollectionID,
						Filename: filename, MIMEType: mimeType, Content: file,
					})
				} else {
					_, err = run.st.UpdateResource(run.ctx, store.UpdateResourceRequest{
						ID: targetID, Filename: filename, MIMEType: mimeType, Content: file,
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
				newStates = append(newStates, run.itemState(item, targetID, action, fingerprint))
			}
		}
		if write {
			return run.st.PutImportItemStates(run.ctx, newStates)
		}
		return nil
	})
}

func (run *importRun) processNotes(write bool, nextPhase string) error {
	folders := make(map[string]parsedItem, len(run.inventory.Folders))
	for id, folder := range run.inventory.Folders {
		folders[id] = folder.parsedItem
	}
	total := run.inventory.NoteCount
	start := 0
	if run.phase == "notes" {
		start = run.nextIndex
	}
	for start < total {
		end := start + run.options.BatchSize
		if end > total {
			end = total
		}
		items, err := run.inventoryItems("notes", start, end)
		if err != nil {
			return err
		}
		externalIDs := make([]string, 0, len(items))
		keys := make([]string, 0, len(items))
		for _, item := range items {
			externalIDs = append(externalIDs, item.ID)
			keys = append(keys, item.ItemKey)
		}
		sourceDocuments, err := run.st.FindDocumentsBySourceIDs(run.ctx, "joplin", externalIDs)
		if err != nil {
			return err
		}
		for externalID, documentID := range sourceDocuments {
			run.noteIDMap[externalID] = documentID
		}
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, run.noteIDMap[item.ID])
		}
		documents, err := run.st.GetDocuments(run.ctx, ids)
		if err != nil {
			return err
		}
		tagRows, err := run.st.GetDocumentTags(run.ctx, ids)
		if err != nil {
			return err
		}
		states, err := run.st.GetImportItemStates(run.ctx, sourceSystem, run.report.SourceKey, run.options.CollectionID, keys)
		if err != nil {
			return err
		}
		noteTagRecords, err := run.inventory.Manifest.Lookup(run.ctx, "note_tags", externalIDs)
		if err != nil {
			return err
		}
		mutations := make([]store.ImportDocumentMutation, 0, len(items))
		for _, item := range items {
			parsed, err := readInventoryItem(item)
			if err != nil {
				return err
			}
			tagNames := run.noteTagNames(item.ID, noteTagRecords)
			body, links := buildDocumentBody(parsed, folders, append([]string(nil), tagNames...), run.noteIDMap, run.resourceMap, run.options.CollectionID)
			run.report.LinksRewritten += links.Rewritten
			run.report.UnresolvedLinks += links.Unresolved
			targetID := run.noteIDMap[item.ID]
			notebookID := run.folderIDs[strings.TrimSpace(parsed.Fields["parent_id"])]
			if notebookID == "" {
				notebookID = store.DefaultNotebookID
			}
			fingerprint := noteFingerprint(item, notebookID, tagNames)
			current, currentFound := documents[targetID]
			if !currentFound {
				// Deterministic IDs are the normal case; provenance preserves
				// idempotence for legacy imports and trashed notes.
				if mapped := sourceDocuments[item.ID]; mapped != "" {
					targetID = mapped
					current, currentFound = documents[mapped]
				}
			}
			trashed := !currentFound && sourceDocuments[item.ID] != ""
			missingTags := desiredMissingTags(tagNames, tagRows[targetID])
			staleTags := staleJoplinTags(tagNames, tagRows[targetID])
			action := "create"
			if trashed {
				action = "unchanged"
				missingTags = nil
				staleTags = nil
			} else if currentFound {
				action = "update"
				if current.Title == noteTitle(parsed) && current.Body == body && current.NotebookID == notebookID && len(missingTags) == 0 && len(staleTags) == 0 {
					action = "unchanged"
				}
				if state, ok := states[item.ItemKey]; ok && state.Fingerprint == fingerprint &&
					current.Title == noteTitle(parsed) && current.Body == body && current.NotebookID == notebookID &&
					len(missingTags) == 0 && len(staleTags) == 0 {
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
			run.report.TagsApplied += len(missingTags)
			run.report.TagsRemoved += len(staleTags)
			run.report.AttachmentsCreated += len(links.Resources)
			if write {
				state, stateFound := states[item.ItemKey]
				skipStableMetadata := action == "unchanged" && !trashed && stateFound &&
					state.Fingerprint == fingerprint && state.TargetID == targetID && sourceDocuments[item.ID] == targetID
				message := "import from Joplin RAW"
				if action == "update" {
					message = "import update from Joplin RAW"
				}
				mutation := store.ImportDocumentMutation{
					Action: action,
					Document: store.CreateDocumentRequest{
						PreferredID: targetID, CollectionID: run.options.CollectionID,
						NotebookID: notebookID, Title: noteTitle(parsed), Body: body,
						BodyMIMEType: "text/markdown", Message: message,
					},
					BaseRevisionID: current.CurrentRevisionID,
					Source: store.SetDocumentSourceRequest{
						DocumentID: targetID, SourceSystem: "joplin", ExternalID: item.ID,
						Author:      strings.TrimSpace(parsed.Fields["author"]),
						SourceURL:   strings.TrimSpace(parsed.Fields["source_url"]),
						PublishedAt: firstNonEmpty(parsed.Fields["user_created_time"], parsed.Fields["created_time"]),
					},
					AddTags:      append([]string(nil), missingTags...),
					State:        run.itemState(item, targetID, action, fingerprint),
					SkipDocument: trashed,
					SkipSource:   skipStableMetadata,
					SkipState:    skipStableMetadata,
				}
				for _, tag := range staleTags {
					mutation.RemoveTags = append(mutation.RemoveTags, tag.Name)
				}
				for _, reference := range links.Resources {
					mutation.Resources = append(mutation.Resources, store.AttachResourceRequest{
						DocumentID: targetID, ResourceID: reference.TargetID, RelationType: "referenced",
						AnchorJSON: fmt.Sprintf(`{"joplin_resource_id":%q}`, reference.SourceID),
					})
				}
				mutations = append(mutations, mutation)
			}
		}
		run.processed += end - start
		run.report.BatchesCompleted++
		if write {
			run.report.CanonicalBatches++
			checkpointPhase, checkpointIndex := "notes", end
			if end == total {
				checkpointPhase, checkpointIndex = nextPhase, 0
			}
			checkpoint, err := run.checkpoint(checkpointPhase, checkpointIndex, "running")
			if err != nil {
				return err
			}
			if err := run.st.ApplyImportDocumentBatch(run.ctx, store.ImportDocumentBatchRequest{Documents: mutations, Checkpoint: checkpoint}); err != nil {
				return err
			}
			if run.options.AfterBatch != nil {
				if err := run.options.AfterBatch("notes", end, total); err != nil {
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

func (run *importRun) processLinks(write bool, nextPhase string) error {
	if !write {
		return nil
	}
	total := run.inventory.NoteCount
	start := 0
	if run.phase == "links" {
		start = run.nextIndex
	}
	for start < total {
		end := start + run.options.BatchSize
		if end > total {
			end = total
		}
		items, err := run.inventoryItems("notes", start, end)
		if err != nil {
			return err
		}
		externalIDs := make([]string, 0, len(items))
		for _, item := range items {
			externalIDs = append(externalIDs, item.ID)
		}
		mapped, err := run.st.FindDocumentsBySourceIDs(run.ctx, "joplin", externalIDs)
		if err != nil {
			return err
		}
		documentIDs := make([]string, 0, len(items))
		for _, item := range items {
			if documentID := mapped[item.ID]; documentID != "" {
				run.noteIDMap[item.ID] = documentID
			}
			documentIDs = append(documentIDs, run.noteIDMap[item.ID])
		}
		run.processed += end - start
		run.report.BatchesCompleted++
		run.report.LinkBatches++
		checkpointPhase, checkpointIndex := "links", end
		if end == total {
			checkpointPhase, checkpointIndex = nextPhase, 0
		}
		checkpoint, err := run.checkpoint(checkpointPhase, checkpointIndex, "running")
		if err != nil {
			return err
		}
		if err := run.st.RebuildImportDocumentLinksBatch(run.ctx, store.ImportLinkBatchRequest{DocumentIDs: documentIDs, Checkpoint: checkpoint}); err != nil {
			return err
		}
		if run.options.AfterBatch != nil {
			if err := run.options.AfterBatch("links", end, total); err != nil {
				return err
			}
		}
		start = end
	}
	if total == 0 {
		return run.saveCheckpoint(nextPhase, 0, "running")
	}
	return nil
}

func (run *importRun) eachBatch(phase string, total int, write bool, nextPhase string, process func(start, end int) error) error {
	start := 0
	if run.phase == phase {
		start = run.nextIndex
	}
	for start < total {
		end := start + run.options.BatchSize
		if end > total {
			end = total
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

func (run *importRun) finishSinglePhase(phase string, total int, write bool, nextPhase string) error {
	run.report.BatchesCompleted++
	if !write {
		return nil
	}
	if err := run.saveCheckpoint(nextPhase, 0, "running"); err != nil {
		return err
	}
	if run.options.AfterBatch != nil {
		return run.options.AfterBatch(phase, total, total)
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
		SourceSystem:         sourceSystem,
		SourceKey:            run.report.SourceKey,
		CollectionID:         run.options.CollectionID,
		InventoryFingerprint: run.inventory.Fingerprint,
		Phase:                phase,
		NextIndex:            nextIndex,
		TotalItems:           run.workTotal,
		ProcessedItems:       run.processed,
		Status:               status,
		ReportJSON:           string(raw),
	}, nil
}

func (run *importRun) addWarning(message string) {
	const warningLimit = 100
	if len(run.report.Warnings) < warningLimit {
		run.report.Warnings = append(run.report.Warnings, message)
		return
	}
	if len(run.report.Warnings) == warningLimit {
		run.report.Warnings = append(run.report.Warnings, "additional warnings omitted")
	}
}

func (run *importRun) itemState(item inventoryItem, targetID, action, fingerprint string) store.ImportItemState {
	return store.ImportItemState{
		SourceSystem: sourceSystem,
		SourceKey:    run.report.SourceKey,
		CollectionID: run.options.CollectionID,
		ItemKey:      item.ItemKey,
		ItemType:     item.Type,
		Fingerprint:  fingerprint,
		TargetID:     targetID,
		Action:       action,
	}
}

func resourceFingerprint(item inventoryItem) string {
	hash := sha256.Sum256([]byte(item.Fingerprint + "\x00" + item.ContentSHA))
	return hex.EncodeToString(hash[:])
}

func noteFingerprint(item inventoryItem, notebookID string, tags []string) string {
	hash := sha256.New()
	_, _ = io.WriteString(hash, item.Fingerprint+"\x00"+notebookID+"\x00")
	for _, tag := range tags {
		_, _ = io.WriteString(hash, strings.ToLower(tag)+"\x00")
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func readInventoryItem(item inventoryItem) (parsedItem, error) {
	raw, err := os.ReadFile(item.Path)
	if err != nil {
		return parsedItem{}, err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != item.Fingerprint {
		return parsedItem{}, fmt.Errorf("%w: Joplin RAW item %s changed during import", store.ErrConflict, item.RelativePath)
	}
	parsed, ok, err := parseItemBytes(item.Path, raw)
	if err != nil {
		return parsedItem{}, err
	}
	if !ok || parsed.ID != item.ID || parsed.Type != item.Type {
		return parsedItem{}, fmt.Errorf("%w: Joplin RAW item %s no longer matches inventory", store.ErrConflict, item.RelativePath)
	}
	return parsed, nil
}

func (run *importRun) inventoryItems(kind string, start, end int) ([]inventoryItem, error) {
	var records []store.ImportManifestRecord
	var err error
	cursor, sequential := run.manifestCursors[kind]
	if start == 0 {
		records, err = run.inventory.Manifest.BatchAfter(run.ctx, kind, "", end-start)
	} else if sequential && cursor.nextOffset == start {
		records, err = run.inventory.Manifest.BatchAfter(run.ctx, kind, cursor.sortKey, end-start)
	} else {
		// A resumed phase pays one indexed OFFSET lookup, then subsequent pages
		// return to keyset pagination through the cursor recorded below.
		records, err = run.inventory.Manifest.Batch(run.ctx, kind, start, end-start)
	}
	if err != nil {
		return nil, err
	}
	items := make([]inventoryItem, 0, len(records))
	for _, record := range records {
		var item inventoryItem
		if err := json.Unmarshal(record.Payload, &item); err != nil {
			return nil, fmt.Errorf("decode Joplin %s manifest item: %w", kind, err)
		}
		items = append(items, item)
	}
	if len(items) != end-start {
		return nil, fmt.Errorf("%w: Joplin %s manifest batch %d:%d returned %d items", store.ErrConflict, kind, start, end, len(items))
	}
	if len(records) > 0 {
		run.manifestCursors[kind] = manifestCursor{nextOffset: end, sortKey: records[len(records)-1].SortKey}
	}
	return items, nil
}

func (run *importRun) noteTagNames(noteID string, records map[string][]store.ImportManifestRecord) []string {
	names := []string{}
	seen := map[string]bool{}
	for _, record := range records[noteID] {
		tagID := string(record.Payload)
		name := strings.TrimSpace(run.inventory.Tags[tagID])
		key := strings.ToLower(name)
		if name != "" && !seen[key] {
			seen[key] = true
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}

func desiredMissingTags(desired []string, actual []store.Tag) []string {
	present := map[string]bool{}
	for _, tag := range actual {
		present[strings.ToLower(tag.Name)] = true
	}
	missing := []string{}
	for _, name := range desired {
		if !present[strings.ToLower(name)] {
			missing = append(missing, name)
		}
	}
	return missing
}

func staleJoplinTags(desired []string, actual []store.Tag) []store.Tag {
	wanted := map[string]bool{}
	for _, name := range desired {
		wanted[strings.ToLower(name)] = true
	}
	stale := []store.Tag{}
	for _, tag := range actual {
		if strings.HasPrefix(tag.ID, "tag_joplin_") && !wanted[strings.ToLower(tag.Name)] {
			stale = append(stale, tag)
		}
	}
	return stale
}

func (run *importRun) sourceTagForTarget(targetID string) (string, string, bool) {
	for sourceID, name := range run.inventory.Tags {
		if "tag_joplin_"+safeID(sourceID) == targetID {
			return sourceID, name, true
		}
	}
	return "", "", false
}
