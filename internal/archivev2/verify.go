package archivev2

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type VerificationReport struct {
	SnapshotID   string `json:"snapshot_id"`
	DatabaseID   string `json:"database_id"`
	Target       string `json:"target"`
	CommitSHA256 string `json:"commit_sha256"`
	IndexObjects int    `json:"index_objects"`
	Objects      int    `json:"objects"`
	RecordChunks int    `json:"record_chunks"`
	Blobs        int    `json:"blobs"`
	Records      int    `json:"records"`
	Bytes        int64  `json:"bytes"`
	Counts       Counts `json:"counts"`
}

// verificationState holds only what is genuinely bounded by the format: the
// collection set (MaxCollections) and the notebook tree (MaxNotebooks, needed
// for cycle and depth checks). Every set proportional to the archived library
// — object hashes and the twelve record identity spaces — goes to disk-backed
// spools instead, so verifying a million-note archive does not need a
// million-note heap.
type verificationState struct {
	collections   map[string]bool
	notebooks     map[string]string
	declarations  *keySpool
	references    *keySpool
	packs         *packSource
	counts        Counts
	limits        Limits
	notebookCount int
	// fanoutObjects counts objects stored as their own file. Packed objects
	// live inside a pack, so only these contribute to the archive's file count.
	fanoutObjects int
}

// Declaration and reference keys share one namespace so a single sorted merge
// checks every cross-reference at once. Composite keys fold a consistency
// check into the join: a revision reference carries its document ID, and a
// blob reference carries its exact byte length, so a mismatch simply fails to
// find a declaration.
const (
	keyPack             = "pack:"
	keyBlob             = "blob:"
	keyCollection       = "collection:"
	keyNotebook         = "notebook:"
	keySearchNotebook   = "search_notebook:"
	keyTag              = "tag:"
	keyDocument         = "document:"
	keyRevision         = "revision:"
	keyDocumentTag      = "document_tag:"
	keyResource         = "resource:"
	keyDocumentResource = "document_resource:"
	keyLink             = "link:"
	keyProvenance       = "provenance:"
	keySourceBundle     = "source_bundle:"
)

// VerifyDirectory performs a complete read-only admission pass. The manifest
// is the sole completion marker; it names the index chunks, the index names
// every object, every object is size/hash/type checked, every typed record is
// decoded under depth/size limits, and references are reconciled before a
// future restore path may begin canonical writes.
//
// Memory is bounded: objects and records stream, and cross-reference checking
// uses external-sorted spools rather than in-memory record maps.
func VerifyDirectory(root string, limits Limits) (VerificationReport, error) {
	if err := validateLimits(limits); err != nil {
		return VerificationReport{}, err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return VerificationReport{}, err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return VerificationReport{}, err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return VerificationReport{}, fmt.Errorf("archive root must be a real directory")
	}
	manifest, err := readManifestFile(root, limits)
	if err != nil {
		return VerificationReport{}, err
	}

	spoolRoot, err := os.MkdirTemp("", "notrios-archive-verify-*")
	if err != nil {
		return VerificationReport{}, err
	}
	defer os.RemoveAll(spoolRoot)
	state, err := newVerificationState(spoolRoot, limits)
	if err != nil {
		return VerificationReport{}, err
	}
	defer state.close()

	totals, err := state.scanIndex(root, manifest)
	if err != nil {
		return VerificationReport{}, err
	}
	if totals != manifest.Totals {
		return VerificationReport{}, fmt.Errorf("index totals do not match the manifest")
	}
	if state.counts != manifest.Counts {
		return VerificationReport{}, fmt.Errorf("decoded record counts do not match manifest")
	}
	if err := state.validateContainers(manifest); err != nil {
		return VerificationReport{}, err
	}
	if err := validateArchiveTree(root, manifest, state.fanoutObjects); err != nil {
		return VerificationReport{}, err
	}
	if err := state.reconcile(); err != nil {
		return VerificationReport{}, err
	}

	return VerificationReport{
		SnapshotID:   manifest.Snapshot.ID,
		DatabaseID:   manifest.Snapshot.DatabaseID,
		Target:       manifest.Snapshot.Target,
		CommitSHA256: manifest.CommitSHA256,
		IndexObjects: len(manifest.Index),
		Objects:      totals.Objects,
		RecordChunks: totals.RecordChunks,
		Blobs:        totals.Blobs,
		Records:      manifest.Counts.Total(),
		Bytes:        totals.Bytes,
		Counts:       manifest.Counts,
	}, nil
}

func readManifestFile(root string, limits Limits) (Manifest, error) {
	raw, err := readRegularBounded(root, "manifest.json", limits.MaxManifestBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, fmt.Errorf("incomplete archive: manifest.json is absent")
		}
		return Manifest{}, fmt.Errorf("manifest.json: %w", err)
	}
	if err := validateJSONDepth(raw, limits.MaxJSONDepth); err != nil {
		return Manifest{}, fmt.Errorf("manifest.json: %w", err)
	}
	var manifest Manifest
	if err := decodeStrict(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("manifest.json: %w", err)
	}
	if err := validateManifest(manifest, limits); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func newVerificationState(spoolRoot string, limits Limits) (*verificationState, error) {
	declarations, err := newKeySpool(spoolRoot, "declarations")
	if err != nil {
		return nil, err
	}
	references, err := newKeySpool(spoolRoot, "references")
	if err != nil {
		_ = declarations.close()
		return nil, err
	}
	return &verificationState{
		collections:  map[string]bool{},
		notebooks:    map[string]string{},
		declarations: declarations,
		references:   references,
		packs:        newPackSource(limits),
		limits:       limits,
	}, nil
}

func (state *verificationState) close() {
	state.packs.close()
	_ = state.declarations.close()
	_ = state.references.close()
}

func (state *verificationState) declare(key string) error { return state.declarations.add(key) }
func (state *verificationState) require(key string) error { return state.references.add(key) }

// scanIndex reads every index chunk, verifies the bytes of every object it
// names, and decodes record chunks in the same pass so records are read once.
func (state *verificationState) scanIndex(root string, manifest Manifest) (ObjectTotals, error) {
	var totals ObjectTotals
	previousHash := ""
	for _, indexObject := range manifest.Index {
		if err := verifyObjectBytes(root, indexObject.Location.Path, indexObject.SHA256, indexObject.SizeBytes); err != nil {
			return ObjectTotals{}, err
		}
		entries, err := state.scanIndexObject(root, indexObject, &previousHash, &totals)
		if err != nil {
			return ObjectTotals{}, err
		}
		if entries != indexObject.Entries {
			return ObjectTotals{}, fmt.Errorf("index object %s declares %d entries but holds %d", indexObject.SHA256, indexObject.Entries, entries)
		}
	}
	return totals, nil
}

func (state *verificationState) scanIndexObject(root string, indexObject IndexObject, previousHash *string, totals *ObjectTotals) (int, error) {
	file, err := openRegular(root, indexObject.Location.Path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if err := requireTrailingLF(file, indexObject.SizeBytes, indexObject.SHA256); err != nil {
		return 0, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 16<<10), state.limits.MaxIndexEntryBytes)
	entries := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			return 0, fmt.Errorf("index object %s contains a blank entry", indexObject.SHA256)
		}
		if err := validateJSONDepth(line, state.limits.MaxJSONDepth); err != nil {
			return 0, fmt.Errorf("index object %s entry %d: %w", indexObject.SHA256, entries+1, err)
		}
		var entry IndexEntry
		if err := decodeStrict(line, &entry); err != nil {
			return 0, fmt.Errorf("index object %s entry %d: %w", indexObject.SHA256, entries+1, err)
		}
		if err := entry.validate(state.limits); err != nil {
			return 0, fmt.Errorf("index object %s entry %d: %w", indexObject.SHA256, entries+1, err)
		}
		if entry.SHA256 <= *previousHash {
			return 0, fmt.Errorf("index entries must be sorted by unique object hash")
		}
		*previousHash = entry.SHA256
		if totals.Bytes > state.limits.MaxTotalBytes-entry.SizeBytes {
			return 0, fmt.Errorf("archive byte limit exceeded")
		}
		totals.Objects++
		// Storage bytes are counted once: a packed object's bytes live inside
		// the pack entry that already contributed them.
		if entry.Location.Layout == LayoutFanout {
			totals.Bytes += entry.SizeBytes
		}
		if totals.Objects > state.limits.MaxObjects {
			return 0, fmt.Errorf("archive declares more than %d objects", state.limits.MaxObjects)
		}
		if err := state.admitObject(root, entry, totals); err != nil {
			return 0, err
		}
		entries++
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("index object %s: entry exceeds %d bytes or cannot be read: %w", indexObject.SHA256, state.limits.MaxIndexEntryBytes, err)
	}
	return entries, nil
}

func (state *verificationState) admitObject(root string, entry IndexEntry, totals *ObjectTotals) error {
	if entry.Location.Layout == LayoutFanout {
		state.fanoutObjects++
	}
	if entry.Location.Layout == LayoutPack {
		// A packed object's bytes live inside a pack this scan also verifies.
		// The reference ties the two together, so a packed entry naming a pack
		// the archive does not carry fails the same join as any other dangling
		// reference.
		if err := state.require(keyPack + entry.Location.PackSHA256); err != nil {
			return err
		}
	} else if err := verifyObjectBytes(root, entry.Location.Path, entry.SHA256, entry.SizeBytes); err != nil {
		return err
	}
	switch entry.Kind {
	case "pack":
		totals.Packs++
		if err := state.packs.verifyContents(root, entry); err != nil {
			return err
		}
		return state.declare(keyPack + entry.SHA256)
	case "blob":
		totals.Blobs++
		if entry.Location.Layout == LayoutPack {
			if err := state.packs.verifySlice(root, entry); err != nil {
				return err
			}
		}
		// The declaration carries the exact length, so a record referring to
		// this blob with a different length simply finds no declaration.
		return state.declare(blobKey(entry.SHA256, entry.SizeBytes))
	case "records":
		totals.RecordChunks++
		if entry.Location.Layout == LayoutPack {
			if err := state.packs.verifySlice(root, entry); err != nil {
				return err
			}
		}
		return state.verifyRecordObject(root, entry)
	default:
		return fmt.Errorf("unsupported object kind %q", entry.Kind)
	}
}

func verifyObjectBytes(root, relative, hash string, size int64) error {
	file, err := openRegular(root, relative)
	if err != nil {
		return fmt.Errorf("object %s: %w", hash, err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() != size {
		return fmt.Errorf("object %s size mismatch", hash)
	}
	digest := sha256.New()
	written, err := io.Copy(digest, file)
	if err != nil || written != size || hex.EncodeToString(digest.Sum(nil)) != hash {
		return fmt.Errorf("object %s checksum mismatch", hash)
	}
	return nil
}

func requireTrailingLF(file *os.File, size int64, hash string) error {
	if size == 0 {
		return fmt.Errorf("object %s is empty", hash)
	}
	last := []byte{0}
	if _, err := file.ReadAt(last, size-1); err != nil || last[0] != '\n' {
		return fmt.Errorf("object %s must end with LF", hash)
	}
	return nil
}

func (state *verificationState) verifyRecordObject(root string, entry IndexEntry) error {
	reader, closer, err := state.packs.open(root, entry)
	if err != nil {
		return err
	}
	defer closer()
	if err := requireTrailingLFAt(reader, entry.SizeBytes, entry.SHA256); err != nil {
		return err
	}
	scanner := bufio.NewScanner(io.NewSectionReader(reader, 0, entry.SizeBytes))
	scanner.Buffer(make([]byte, 64<<10), state.limits.MaxRecordBytes)
	var actual Counts
	records := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			return fmt.Errorf("records object %s contains a blank record", entry.SHA256)
		}
		if err := validateJSONDepth(line, state.limits.MaxJSONDepth); err != nil {
			return fmt.Errorf("records object %s line %d: %w", entry.SHA256, records+1, err)
		}
		var envelope RecordEnvelope
		if err := decodeStrict(line, &envelope); err != nil {
			return fmt.Errorf("records object %s line %d: %w", entry.SHA256, records+1, err)
		}
		count, ok := countForRecordType(envelope.Type)
		if !ok {
			return fmt.Errorf("records object %s line %d: unsupported record type %q", entry.SHA256, records+1, envelope.Type)
		}
		if err := state.addRecord(envelope); err != nil {
			return fmt.Errorf("records object %s line %d: %w", entry.SHA256, records+1, err)
		}
		actual.Add(count)
		records++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("records object %s: record exceeds %d bytes or cannot be read: %w", entry.SHA256, state.limits.MaxRecordBytes, err)
	}
	if records != entry.Records || entry.RecordCounts == nil || actual != *entry.RecordCounts {
		return fmt.Errorf("records object %s count mismatch", entry.SHA256)
	}
	state.counts.Add(actual)
	if state.counts.Total() > state.limits.MaxRecords {
		return fmt.Errorf("archive holds more than %d records", state.limits.MaxRecords)
	}
	return nil
}

func blobKey(hash string, size int64) string {
	return fmt.Sprintf("%s%s:%d", keyBlob, hash, size)
}

// addRecord validates one record's shape and emits its identity declaration
// plus every reference it makes. Nothing about the record is retained except
// the bounded collection and notebook sets.
func (state *verificationState) addRecord(envelope RecordEnvelope) error {
	maxDepth := state.limits.MaxJSONDepth
	switch envelope.Type {
	case RecordCollection:
		var record CollectionRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || strings.TrimSpace(record.Name) == "" || len(record.Name) > 4096 || !validTimestamp(record.CreatedAt) || !validJSONObject(record.SettingsJSON, maxDepth) || !sortedUnique(record.Capabilities) {
			return fmt.Errorf("invalid collection record")
		}
		if len(state.collections) >= state.limits.MaxCollections {
			return fmt.Errorf("archive declares more than %d collections", state.limits.MaxCollections)
		}
		state.collections[record.ID] = true
		return state.declare(keyCollection + record.ID)
	case RecordNotebook:
		var record NotebookRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || (record.ParentID != "" && !validID(record.ParentID)) || strings.TrimSpace(record.Name) == "" || len(record.Name) > 4096 || !validTimestamp(record.CreatedAt) || !validTimestamp(record.UpdatedAt) {
			return fmt.Errorf("invalid notebook record")
		}
		state.notebookCount++
		if state.notebookCount > state.limits.MaxNotebooks {
			return fmt.Errorf("archive declares more than %d notebooks", state.limits.MaxNotebooks)
		}
		if _, exists := state.notebooks[record.ID]; exists {
			return fmt.Errorf("duplicate notebook record %q", record.ID)
		}
		state.notebooks[record.ID] = record.ParentID
		if err := state.declare(keyNotebook + record.ID); err != nil {
			return err
		}
		if record.ParentID != "" {
			return state.require(keyNotebook + record.ParentID)
		}
		return nil
	case RecordSearchNotebook:
		var record SearchNotebookRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || strings.TrimSpace(record.Name) == "" || len(record.Name) > 4096 || len(record.Query) > 4096 || !validTimestamp(record.CreatedAt) {
			return fmt.Errorf("invalid search_notebook record")
		}
		switch record.SortAnchor {
		case "first", "normal", "last":
		default:
			return fmt.Errorf("invalid search_notebook sort anchor")
		}
		return state.declare(keySearchNotebook + record.ID)
	case RecordTag:
		var record TagRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || strings.TrimSpace(record.Name) == "" || len(record.Name) > 4096 || !validTimestamp(record.CreatedAt) {
			return fmt.Errorf("invalid tag record")
		}
		return state.declare(keyTag + record.ID)
	case RecordDocument:
		var record DocumentRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || !validID(record.CollectionID) || !validID(record.NotebookID) || !validID(record.CurrentRevisionID) || !validTimestamp(record.CreatedAt) || !validTimestamp(record.UpdatedAt) || (record.DeletedAt != "" && !validTimestamp(record.DeletedAt)) {
			return fmt.Errorf("invalid document record")
		}
		if err := state.declare(keyDocument + record.ID); err != nil {
			return err
		}
		if err := state.require(keyCollection + record.CollectionID); err != nil {
			return err
		}
		if err := state.require(keyNotebook + record.NotebookID); err != nil {
			return err
		}
		// The composite key also proves the current revision belongs to this
		// document without keeping either record.
		return state.require(revisionKey(record.CurrentRevisionID, record.ID))
	case RecordRevision:
		var record RevisionRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || !validID(record.DocumentID) || len(record.Title) > 1<<20 || !validTimestamp(record.CreatedAt) || !validJSONObject(record.MetadataJSON, maxDepth) {
			return fmt.Errorf("invalid revision record")
		}
		if err := validateBlobReferenceShape(record.Body); err != nil {
			return err
		}
		if normalized, err := normalizeMediaType(record.BodyMIMEType); err != nil || normalized != record.Body.MediaType {
			return fmt.Errorf("revision body MIME mismatch")
		}
		if err := state.declare(revisionKey(record.ID, record.DocumentID)); err != nil {
			return err
		}
		if err := state.require(keyDocument + record.DocumentID); err != nil {
			return err
		}
		return state.require(blobKey(record.Body.SHA256, record.Body.SizeBytes))
	case RecordDocumentTag:
		var record DocumentTagRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.DocumentID) || !validID(record.TagID) {
			return fmt.Errorf("invalid document_tag record")
		}
		if err := state.declare(keyDocumentTag + record.DocumentID + "\x1f" + record.TagID); err != nil {
			return err
		}
		if err := state.require(keyDocument + record.DocumentID); err != nil {
			return err
		}
		return state.require(keyTag + record.TagID)
	case RecordResource:
		var record ResourceRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || !validID(record.CollectionID) || len(record.Filename) > 4096 || !validTimestamp(record.CreatedAt) || (record.UnreferencedAt != "" && !validTimestamp(record.UnreferencedAt)) || !validJSONObject(record.MetadataJSON, maxDepth) {
			return fmt.Errorf("invalid resource record")
		}
		if err := validateBlobReferenceShape(record.Blob); err != nil {
			return err
		}
		if normalized, err := normalizeMediaType(record.MIMEType); err != nil || normalized != record.Blob.MediaType {
			return fmt.Errorf("resource MIME mismatch")
		}
		if err := state.declare(keyResource + record.ID); err != nil {
			return err
		}
		if err := state.require(keyCollection + record.CollectionID); err != nil {
			return err
		}
		return state.require(blobKey(record.Blob.SHA256, record.Blob.SizeBytes))
	case RecordDocumentResource:
		var record DocumentResourceRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.DocumentID) || !validID(record.ResourceID) || strings.TrimSpace(record.RelationType) == "" || record.Ordinal < 0 || !validJSONObject(record.AnchorJSON, maxDepth) {
			return fmt.Errorf("invalid document_resource record")
		}
		key := fmt.Sprintf("%s%s\x1f%s\x1f%s\x1f%d", keyDocumentResource, record.DocumentID, record.ResourceID, record.RelationType, record.Ordinal)
		if err := state.declare(key); err != nil {
			return err
		}
		if err := state.require(keyDocument + record.DocumentID); err != nil {
			return err
		}
		return state.require(keyResource + record.ResourceID)
	case RecordLink:
		var record LinkRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || !validID(record.SourceDocumentID) || (record.TargetDocumentID != "" && !validID(record.TargetDocumentID)) || (record.TargetResourceID != "" && !validID(record.TargetResourceID)) || strings.TrimSpace(record.RelationType) == "" || strings.TrimSpace(record.SourceFormat) == "" || strings.TrimSpace(record.ResolutionStatus) == "" || record.SourceStartByte < 0 || record.SourceEndByte < record.SourceStartByte || record.SourceLine < 0 || record.SourceColumn < 0 {
			return fmt.Errorf("invalid link record")
		}
		if err := state.declare(keyLink + record.ID); err != nil {
			return err
		}
		if err := state.require(keyDocument + record.SourceDocumentID); err != nil {
			return err
		}
		if record.TargetDocumentID != "" {
			if err := state.require(keyDocument + record.TargetDocumentID); err != nil {
				return err
			}
		}
		if record.TargetResourceID != "" {
			return state.require(keyResource + record.TargetResourceID)
		}
		return nil
	case RecordProvenance:
		var record ProvenanceRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.DocumentID) || strings.TrimSpace(record.SourceSystem) == "" || !validTimestamp(record.CreatedAt) || !validTimestamp(record.UpdatedAt) || !validJSONObject(record.MetadataJSON, maxDepth) {
			return fmt.Errorf("invalid provenance record")
		}
		if err := state.declare(keyProvenance + record.DocumentID); err != nil {
			return err
		}
		return state.require(keyDocument + record.DocumentID)
	case RecordSourceBundle:
		var record SourceBundleRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if strings.TrimSpace(record.SourceSystem) == "" || !validSHA256(record.SourceKeySHA256) || !validID(record.CollectionID) || !validSHA256(record.ItemKeySHA256) || strings.TrimSpace(record.ItemType) == "" || !validRelativePath(record.RelativePath, state.limits) || !validTimestamp(record.UpdatedAt) || !validStringArray(record.PropertyOrderJSON, maxDepth) {
			return fmt.Errorf("invalid source_bundle record")
		}
		if err := validateBlobReferenceShape(record.Content); err != nil {
			return err
		}
		key := keySourceBundle + strings.Join([]string{record.SourceSystem, record.SourceKeySHA256, record.CollectionID, record.ItemKeySHA256}, "\x1f")
		if err := state.declare(key); err != nil {
			return err
		}
		if err := state.require(keyCollection + record.CollectionID); err != nil {
			return err
		}
		return state.require(blobKey(record.Content.SHA256, record.Content.SizeBytes))
	default:
		return fmt.Errorf("unsupported record type %q", envelope.Type)
	}
}

func revisionKey(revisionID, documentID string) string {
	return keyRevision + revisionID + "\x1f" + documentID
}

// validateContainers checks the two bounded sets the join cannot express:
// snapshot collection scope and the notebook tree's acyclicity and depth.
func (state *verificationState) validateContainers(manifest Manifest) error {
	for _, collectionID := range manifest.Snapshot.CollectionIDs {
		if !state.collections[collectionID] {
			return fmt.Errorf("snapshot collection %q is missing its record", collectionID)
		}
	}
	for id := range state.collections {
		if !contains(manifest.Snapshot.CollectionIDs, id) {
			return fmt.Errorf("collection %q is outside snapshot collection_ids", id)
		}
	}
	for id := range state.notebooks {
		if err := state.validateNotebookDepth(id); err != nil {
			return err
		}
	}
	return nil
}

func (state *verificationState) validateNotebookDepth(start string) error {
	seen := map[string]bool{}
	current := start
	for depth := 0; current != ""; depth++ {
		if depth >= state.limits.MaxNotebookDepth {
			return fmt.Errorf("notebook %q exceeds depth %d", start, state.limits.MaxNotebookDepth)
		}
		if seen[current] {
			return fmt.Errorf("notebook %q participates in a cycle", start)
		}
		seen[current] = true
		current = state.notebooks[current]
	}
	return nil
}

// reconcile merges the declaration and reference spools. It rejects any
// reference with no matching declaration (a record naming something the
// archive omits, or naming it with the wrong size or owner), any duplicate
// declaration, and any blob object nothing references.
func (state *verificationState) reconcile() error {
	if err := state.declarations.flush(); err != nil {
		return err
	}
	if err := state.references.flush(); err != nil {
		return err
	}
	return joinSpools(state.declarations, state.references, keyBlob)
}

// validateArchiveTree rejects symlinks, unexpected directories, and files that
// are not well-formed object paths, then proves there are no extra files by
// counting: every object stored as its own file was opened during the index
// scan, so an equal file count leaves no room for an unlisted one. Packed
// objects are not files, so only their pack counts here.
func validateArchiveTree(root string, manifest Manifest, fanoutObjects int) error {
	indexPaths := make(map[string]bool, len(manifest.Index))
	for _, indexObject := range manifest.Index {
		indexPaths[indexObject.Location.Path] = true
	}
	files := 0
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive contains symlink %q", relative)
		}
		if entry.IsDir() {
			if !validObjectDirectory(relative) {
				return fmt.Errorf("archive contains unexpected directory %q", relative)
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive contains a non-regular file %q", relative)
		}
		if relative == "manifest.json" {
			return nil
		}
		if !indexPaths[relative] && !validObjectFilePath(relative) {
			return fmt.Errorf("archive contains unexpected file %q", relative)
		}
		files++
		return nil
	})
	if err != nil {
		return err
	}
	if files != fanoutObjects+len(manifest.Index) {
		return fmt.Errorf("archive holds %d object files but the index and manifest declare %d", files, fanoutObjects+len(manifest.Index))
	}
	return nil
}

func validObjectDirectory(relative string) bool {
	segments := strings.Split(relative, "/")
	switch len(segments) {
	case 1:
		return segments[0] == "objects"
	case 2:
		return segments[0] == "objects" && segments[1] == "sha256"
	case 3, 4:
		return segments[0] == "objects" && segments[1] == "sha256" && isLowerHex(segments[2], 2) &&
			(len(segments) == 3 || isLowerHex(segments[3], 2))
	default:
		return false
	}
}

func validObjectFilePath(relative string) bool {
	segments := strings.Split(relative, "/")
	if len(segments) != 5 || segments[0] != "objects" || segments[1] != "sha256" {
		return false
	}
	hash := segments[4]
	return validSHA256(hash) && segments[2] == hash[:2] && segments[3] == hash[2:4]
}

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for index := 0; index < len(value); index++ {
		if _, ok := hexValue(value[index]); !ok {
			return false
		}
	}
	return true
}

func validateLimits(limits Limits) error {
	defaults := DefaultLimits()
	if limits.MaxManifestBytes <= 0 || limits.MaxManifestBytes > defaults.MaxManifestBytes ||
		limits.MaxObjects <= 0 || limits.MaxObjects > defaults.MaxObjects ||
		limits.MaxIndexObjects <= 0 || limits.MaxIndexObjects > defaults.MaxIndexObjects ||
		limits.MaxIndexEntriesPerObject <= 0 || limits.MaxIndexEntriesPerObject > defaults.MaxIndexEntriesPerObject ||
		limits.MaxIndexEntryBytes <= 0 || limits.MaxIndexEntryBytes > defaults.MaxIndexEntryBytes ||
		limits.MaxIndexObjectBytes <= 0 || limits.MaxIndexObjectBytes > defaults.MaxIndexObjectBytes ||
		limits.MaxRecords <= 0 || limits.MaxRecords > defaults.MaxRecords ||
		limits.MaxRecordsPerObject <= 0 || limits.MaxRecordsPerObject > defaults.MaxRecordsPerObject ||
		limits.MaxRecordBytes <= 0 || limits.MaxRecordBytes > defaults.MaxRecordBytes ||
		limits.MaxRecordObjectBytes <= 0 || limits.MaxRecordObjectBytes > defaults.MaxRecordObjectBytes ||
		limits.MaxBlobBytes <= 0 || limits.MaxBlobBytes > defaults.MaxBlobBytes ||
		limits.MaxTotalBytes <= 0 || limits.MaxTotalBytes > defaults.MaxTotalBytes ||
		limits.MaxPathBytes <= 0 || limits.MaxPathBytes > defaults.MaxPathBytes ||
		limits.MaxPathDepth <= 0 || limits.MaxPathDepth > defaults.MaxPathDepth ||
		limits.MaxJSONDepth <= 0 || limits.MaxJSONDepth > defaults.MaxJSONDepth ||
		limits.MaxCollections <= 0 || limits.MaxCollections > defaults.MaxCollections ||
		limits.MaxNotebooks <= 0 || limits.MaxNotebooks > defaults.MaxNotebooks ||
		limits.MaxNotebookDepth <= 0 || limits.MaxNotebookDepth > defaults.MaxNotebookDepth ||
		limits.MaxCapabilities <= 0 || limits.MaxCapabilities > defaults.MaxCapabilities ||
		limits.MaxCapabilityNameBytes <= 0 || limits.MaxCapabilityNameBytes > defaults.MaxCapabilityNameBytes {
		return fmt.Errorf("archive limits must be positive and no wider than defaults")
	}
	return nil
}

func validateBlobReferenceShape(reference BlobReference) error {
	if !validSHA256(reference.SHA256) || reference.SizeBytes < 0 {
		return fmt.Errorf("invalid blob reference")
	}
	normalized, err := normalizeMediaType(reference.MediaType)
	if err != nil || normalized != reference.MediaType {
		return fmt.Errorf("blob reference MIME must be canonical")
	}
	return nil
}

func normalizeMediaType(value string) (string, error) {
	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil || mediaType == "" || len(value) > 255 {
		return "", fmt.Errorf("invalid MIME type %q", value)
	}
	mediaType = strings.ToLower(mediaType)
	return mime.FormatMediaType(mediaType, parameters), nil
}

func validTimestamp(value string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && parsed.Location() == time.UTC
}

func validEmbeddedJSON(raw json.RawMessage, maxDepth int) bool {
	return len(raw) > 0 && json.Valid(raw) && validateJSONDepth(raw, maxDepth) == nil && validateNoDuplicateJSONKeys(raw) == nil
}

func validJSONObject(raw json.RawMessage, maxDepth int) bool {
	return validEmbeddedJSON(raw, maxDepth) && len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '{'
}

func validStringArray(raw json.RawMessage, maxDepth int) bool {
	if !validEmbeddedJSON(raw, maxDepth) {
		return false
	}
	var values []string
	return json.Unmarshal(raw, &values) == nil && values != nil
}

func validRelativePath(value string, limits Limits) bool {
	if value == "" || len(value) > limits.MaxPathBytes || strings.Contains(value, "\\") || strings.ContainsRune(value, 0) || strings.HasPrefix(value, "/") {
		return false
	}
	clean := path.Clean(value)
	if clean != value || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	return len(strings.Split(clean, "/")) <= limits.MaxPathDepth
}

func decodeStrict(raw []byte, target any) error {
	if err := validateNoDuplicateJSONKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func validateNoDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := consumeUniqueJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON data")
		}
		return err
	}
	return nil
}

func consumeUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key is not a string")
			}
			if keys[key] {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			keys[key] = true
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("invalid JSON object")
		}
	case '[':
		for decoder.More() {
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("invalid JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
	return nil
}

func validateJSONDepth(raw []byte, maximum int) error {
	depth := 0
	inString := false
	escaped := false
	for _, char := range raw {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maximum {
				return fmt.Errorf("JSON nesting exceeds %d", maximum)
			}
		case '}', ']':
			depth--
			if depth < 0 {
				return fmt.Errorf("invalid JSON nesting")
			}
		}
	}
	if depth != 0 || inString {
		return fmt.Errorf("invalid JSON structure")
	}
	return nil
}

func readRegularBounded(root, relative string, maximum int64) ([]byte, error) {
	file, err := openRegular(root, relative)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return raw, nil
}

func openRegular(root, relative string) (*os.File, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") || filepath.ToSlash(filepath.Clean(relative)) != relative {
		return nil, fmt.Errorf("unsafe archive path %q", relative)
	}
	current := root
	parts := strings.Split(relative, "/")
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, fmt.Errorf("unsafe archive path %q", relative)
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink is forbidden at %q", relative)
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("archive path component is not a directory")
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("archive object is not a regular file")
		}
	}
	return os.Open(current)
}
