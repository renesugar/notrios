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
	"sort"
	"strings"
	"time"
)

type VerificationReport struct {
	SnapshotID      string `json:"snapshot_id"`
	DatabaseID      string `json:"database_id"`
	Target          string `json:"target"`
	CommitSHA256    string `json:"commit_sha256"`
	Objects         int    `json:"objects"`
	Records         int    `json:"records"`
	Bytes           int64  `json:"bytes"`
	RequiredObjects int    `json:"required_objects"`
	Counts          Counts `json:"counts"`
}

type verificationState struct {
	collections       map[string]CollectionRecord
	notebooks         map[string]NotebookRecord
	searchNotebooks   map[string]SearchNotebookRecord
	tags              map[string]TagRecord
	documents         map[string]DocumentRecord
	revisions         map[string]RevisionRecord
	documentTags      map[string]DocumentTagRecord
	resources         map[string]ResourceRecord
	documentResources map[string]DocumentResourceRecord
	links             map[string]LinkRecord
	provenance        map[string]ProvenanceRecord
	sourceBundles     map[string]SourceBundleRecord
	referencedBlobs   map[string]bool
	counts            Counts
}

func newVerificationState() *verificationState {
	return &verificationState{
		collections:       map[string]CollectionRecord{},
		notebooks:         map[string]NotebookRecord{},
		searchNotebooks:   map[string]SearchNotebookRecord{},
		tags:              map[string]TagRecord{},
		documents:         map[string]DocumentRecord{},
		revisions:         map[string]RevisionRecord{},
		documentTags:      map[string]DocumentTagRecord{},
		resources:         map[string]ResourceRecord{},
		documentResources: map[string]DocumentResourceRecord{},
		links:             map[string]LinkRecord{},
		provenance:        map[string]ProvenanceRecord{},
		sourceBundles:     map[string]SourceBundleRecord{},
		referencedBlobs:   map[string]bool{},
	}
}

// VerifyDirectory performs a complete read-only admission pass. The manifest
// is the sole completion marker; every listed object is size/hash/type checked,
// every typed record is decoded under depth/size limits, and references are
// reconciled before a future restore path may begin canonical writes.
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
	manifestRaw, err := readRegularBounded(root, "manifest.json", limits.MaxManifestBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return VerificationReport{}, fmt.Errorf("incomplete archive: manifest.json is absent")
		}
		return VerificationReport{}, fmt.Errorf("manifest.json: %w", err)
	}
	if err := validateJSONDepth(manifestRaw, limits.MaxJSONDepth); err != nil {
		return VerificationReport{}, fmt.Errorf("manifest.json: %w", err)
	}
	var manifest Manifest
	if err := decodeStrict(manifestRaw, &manifest); err != nil {
		return VerificationReport{}, fmt.Errorf("manifest.json: %w", err)
	}
	if err := validateManifest(manifest, limits); err != nil {
		return VerificationReport{}, err
	}
	if err := validateArchiveTree(root, manifest.Objects); err != nil {
		return VerificationReport{}, err
	}

	objectsByHash := make(map[string]Object, len(manifest.Objects))
	state := newVerificationState()
	var totalBytes int64
	for _, object := range manifest.Objects {
		objectsByHash[object.SHA256] = object
		if err := verifyObjectBytes(root, object); err != nil {
			return VerificationReport{}, err
		}
		totalBytes += object.SizeBytes
		if object.Kind == "records" {
			if err := verifyRecordObject(root, object, limits, state); err != nil {
				return VerificationReport{}, err
			}
		}
	}
	if state.counts != manifest.Counts {
		return VerificationReport{}, fmt.Errorf("decoded record counts do not match manifest")
	}
	if err := state.validateReferences(manifest, objectsByHash, limits); err != nil {
		return VerificationReport{}, err
	}
	for hash, object := range objectsByHash {
		if object.Kind == "blob" && !state.referencedBlobs[hash] {
			return VerificationReport{}, fmt.Errorf("unreferenced blob object %s", hash)
		}
	}
	return VerificationReport{
		SnapshotID:      manifest.Snapshot.ID,
		DatabaseID:      manifest.Snapshot.DatabaseID,
		Target:          manifest.Snapshot.Target,
		CommitSHA256:    manifest.CommitSHA256,
		Objects:         len(manifest.Objects),
		Records:         manifest.Counts.Total(),
		Bytes:           totalBytes,
		RequiredObjects: len(state.referencedBlobs),
		Counts:          manifest.Counts,
	}, nil
}

func validateLimits(limits Limits) error {
	defaults := DefaultLimits()
	if limits.MaxManifestBytes <= 0 || limits.MaxManifestBytes > defaults.MaxManifestBytes ||
		limits.MaxObjects <= 0 || limits.MaxObjects > defaults.MaxObjects ||
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
		limits.MaxNotebookDepth <= 0 || limits.MaxNotebookDepth > defaults.MaxNotebookDepth ||
		limits.MaxCapabilities <= 0 || limits.MaxCapabilities > defaults.MaxCapabilities ||
		limits.MaxCapabilityNameBytes <= 0 || limits.MaxCapabilityNameBytes > defaults.MaxCapabilityNameBytes {
		return fmt.Errorf("archive limits must be positive and no wider than defaults")
	}
	return nil
}

func validateArchiveTree(root string, objects []Object) error {
	expectedFiles := map[string]bool{"manifest.json": true}
	expectedDirs := map[string]bool{"objects": true, "objects/sha256": true}
	for _, object := range objects {
		expectedFiles[object.Path] = true
		dir := path.Dir(object.Path)
		for dir != "." && dir != "/" {
			expectedDirs[dir] = true
			dir = path.Dir(dir)
		}
	}
	return filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive contains symlink %q", rel)
		}
		if entry.IsDir() {
			if !expectedDirs[rel] {
				return fmt.Errorf("archive contains unexpected directory %q", rel)
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !expectedFiles[rel] {
			return fmt.Errorf("archive contains unexpected non-regular or unlisted file %q", rel)
		}
		return nil
	})
}

func verifyObjectBytes(root string, object Object) error {
	file, err := openRegular(root, object.Path)
	if err != nil {
		return fmt.Errorf("object %s: %w", object.SHA256, err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() != object.SizeBytes {
		return fmt.Errorf("object %s size mismatch", object.SHA256)
	}
	hash := sha256.New()
	written, err := io.Copy(hash, file)
	if err != nil || written != object.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != object.SHA256 {
		return fmt.Errorf("object %s checksum mismatch", object.SHA256)
	}
	return nil
}

func verifyRecordObject(root string, object Object, limits Limits, state *verificationState) error {
	file, err := openRegular(root, object.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	if object.SizeBytes == 0 {
		return fmt.Errorf("records object %s is empty", object.SHA256)
	}
	last := []byte{0}
	if _, err := file.ReadAt(last, object.SizeBytes-1); err != nil || last[0] != '\n' {
		return fmt.Errorf("records object %s must end with LF", object.SHA256)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), limits.MaxRecordBytes)
	var actual Counts
	records := 0
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(bytes.TrimSpace(line)) == 0 {
			return fmt.Errorf("records object %s contains a blank record", object.SHA256)
		}
		if err := validateJSONDepth(line, limits.MaxJSONDepth); err != nil {
			return fmt.Errorf("records object %s line %d: %w", object.SHA256, records+1, err)
		}
		var envelope RecordEnvelope
		if err := decodeStrict(line, &envelope); err != nil {
			return fmt.Errorf("records object %s line %d: %w", object.SHA256, records+1, err)
		}
		count, ok := countForRecordType(envelope.Type)
		if !ok {
			return fmt.Errorf("records object %s line %d: unsupported record type %q", object.SHA256, records+1, envelope.Type)
		}
		if err := state.addRecord(envelope, limits); err != nil {
			return fmt.Errorf("records object %s line %d: %w", object.SHA256, records+1, err)
		}
		actual.Add(count)
		records++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("records object %s: record exceeds %d bytes or cannot be read: %w", object.SHA256, limits.MaxRecordBytes, err)
	}
	if records != object.Records || actual != object.RecordCounts {
		return fmt.Errorf("records object %s count mismatch", object.SHA256)
	}
	state.counts.Add(actual)
	return nil
}

func (state *verificationState) addRecord(envelope RecordEnvelope, limits Limits) error {
	switch envelope.Type {
	case RecordCollection:
		var record CollectionRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || strings.TrimSpace(record.Name) == "" || len(record.Name) > 4096 || !validTimestamp(record.CreatedAt) || !validJSONObject(record.SettingsJSON, limits.MaxJSONDepth) || !sortedUnique(record.Capabilities) {
			return fmt.Errorf("invalid collection record")
		}
		return addUnique(state.collections, record.ID, record)
	case RecordNotebook:
		var record NotebookRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || (record.ParentID != "" && !validID(record.ParentID)) || strings.TrimSpace(record.Name) == "" || len(record.Name) > 4096 || !validTimestamp(record.CreatedAt) || !validTimestamp(record.UpdatedAt) {
			return fmt.Errorf("invalid notebook record")
		}
		return addUnique(state.notebooks, record.ID, record)
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
		return addUnique(state.searchNotebooks, record.ID, record)
	case RecordTag:
		var record TagRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || strings.TrimSpace(record.Name) == "" || len(record.Name) > 4096 || !validTimestamp(record.CreatedAt) {
			return fmt.Errorf("invalid tag record")
		}
		return addUnique(state.tags, record.ID, record)
	case RecordDocument:
		var record DocumentRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || !validID(record.CollectionID) || !validID(record.NotebookID) || !validID(record.CurrentRevisionID) || !validTimestamp(record.CreatedAt) || !validTimestamp(record.UpdatedAt) || (record.DeletedAt != "" && !validTimestamp(record.DeletedAt)) {
			return fmt.Errorf("invalid document record")
		}
		return addUnique(state.documents, record.ID, record)
	case RecordRevision:
		var record RevisionRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || !validID(record.DocumentID) || len(record.Title) > 1<<20 || !validTimestamp(record.CreatedAt) || !validJSONObject(record.MetadataJSON, limits.MaxJSONDepth) {
			return fmt.Errorf("invalid revision record")
		}
		if err := validateBlobReferenceShape(record.Body); err != nil {
			return err
		}
		if normalized, err := normalizeMediaType(record.BodyMIMEType); err != nil || normalized != record.Body.MediaType {
			return fmt.Errorf("revision body MIME mismatch")
		}
		return addUnique(state.revisions, record.ID, record)
	case RecordDocumentTag:
		var record DocumentTagRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.DocumentID) || !validID(record.TagID) {
			return fmt.Errorf("invalid document_tag record")
		}
		return addUnique(state.documentTags, record.DocumentID+"\x00"+record.TagID, record)
	case RecordResource:
		var record ResourceRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || !validID(record.CollectionID) || len(record.Filename) > 4096 || !validTimestamp(record.CreatedAt) || (record.UnreferencedAt != "" && !validTimestamp(record.UnreferencedAt)) || !validJSONObject(record.MetadataJSON, limits.MaxJSONDepth) {
			return fmt.Errorf("invalid resource record")
		}
		if err := validateBlobReferenceShape(record.Blob); err != nil {
			return err
		}
		if normalized, err := normalizeMediaType(record.MIMEType); err != nil || normalized != record.Blob.MediaType {
			return fmt.Errorf("resource MIME mismatch")
		}
		return addUnique(state.resources, record.ID, record)
	case RecordDocumentResource:
		var record DocumentResourceRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.DocumentID) || !validID(record.ResourceID) || strings.TrimSpace(record.RelationType) == "" || record.Ordinal < 0 || !validJSONObject(record.AnchorJSON, limits.MaxJSONDepth) {
			return fmt.Errorf("invalid document_resource record")
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%d", record.DocumentID, record.ResourceID, record.RelationType, record.Ordinal)
		return addUnique(state.documentResources, key, record)
	case RecordLink:
		var record LinkRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.ID) || !validID(record.SourceDocumentID) || (record.TargetDocumentID != "" && !validID(record.TargetDocumentID)) || (record.TargetResourceID != "" && !validID(record.TargetResourceID)) || strings.TrimSpace(record.RelationType) == "" || strings.TrimSpace(record.SourceFormat) == "" || strings.TrimSpace(record.ResolutionStatus) == "" || record.SourceStartByte < 0 || record.SourceEndByte < record.SourceStartByte || record.SourceLine < 0 || record.SourceColumn < 0 {
			return fmt.Errorf("invalid link record")
		}
		return addUnique(state.links, record.ID, record)
	case RecordProvenance:
		var record ProvenanceRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if !validID(record.DocumentID) || strings.TrimSpace(record.SourceSystem) == "" || !validTimestamp(record.CreatedAt) || !validTimestamp(record.UpdatedAt) || !validJSONObject(record.MetadataJSON, limits.MaxJSONDepth) {
			return fmt.Errorf("invalid provenance record")
		}
		return addUnique(state.provenance, record.DocumentID, record)
	case RecordSourceBundle:
		var record SourceBundleRecord
		if err := decodeStrict(envelope.Payload, &record); err != nil {
			return err
		}
		if strings.TrimSpace(record.SourceSystem) == "" || !validSHA256(record.SourceKeySHA256) || !validID(record.CollectionID) || !validSHA256(record.ItemKeySHA256) || strings.TrimSpace(record.ItemType) == "" || !validRelativePath(record.RelativePath, limits) || !validTimestamp(record.UpdatedAt) || !validStringArray(record.PropertyOrderJSON, limits.MaxJSONDepth) {
			return fmt.Errorf("invalid source_bundle record")
		}
		if err := validateBlobReferenceShape(record.Content); err != nil {
			return err
		}
		key := record.SourceSystem + "\x00" + record.SourceKeySHA256 + "\x00" + record.CollectionID + "\x00" + record.ItemKeySHA256
		return addUnique(state.sourceBundles, key, record)
	default:
		return fmt.Errorf("unsupported record type %q", envelope.Type)
	}
}

func (state *verificationState) validateReferences(manifest Manifest, objects map[string]Object, limits Limits) error {
	for _, collectionID := range manifest.Snapshot.CollectionIDs {
		if _, ok := state.collections[collectionID]; !ok {
			return fmt.Errorf("snapshot collection %q is missing its record", collectionID)
		}
	}
	for id := range state.collections {
		if !contains(manifest.Snapshot.CollectionIDs, id) {
			return fmt.Errorf("collection %q is outside snapshot collection_ids", id)
		}
	}
	for id, notebook := range state.notebooks {
		if notebook.ParentID != "" {
			if _, ok := state.notebooks[notebook.ParentID]; !ok {
				return fmt.Errorf("notebook %q has missing parent", id)
			}
		}
		if err := state.validateNotebookDepth(id, limits.MaxNotebookDepth); err != nil {
			return err
		}
	}
	for id, document := range state.documents {
		if _, ok := state.collections[document.CollectionID]; !ok {
			return fmt.Errorf("document %q has missing collection", id)
		}
		if _, ok := state.notebooks[document.NotebookID]; !ok {
			return fmt.Errorf("document %q has missing notebook", id)
		}
		revision, ok := state.revisions[document.CurrentRevisionID]
		if !ok || revision.DocumentID != id {
			return fmt.Errorf("document %q has inconsistent current revision", id)
		}
	}
	for id, revision := range state.revisions {
		if _, ok := state.documents[revision.DocumentID]; !ok {
			return fmt.Errorf("revision %q has missing document", id)
		}
		if err := state.validateBlobReference(revision.Body, objects); err != nil {
			return fmt.Errorf("revision %q body: %w", id, err)
		}
	}
	for _, membership := range state.documentTags {
		if _, ok := state.documents[membership.DocumentID]; !ok {
			return fmt.Errorf("document_tag has missing document")
		}
		if _, ok := state.tags[membership.TagID]; !ok {
			return fmt.Errorf("document_tag has missing tag")
		}
	}
	for id, resource := range state.resources {
		if _, ok := state.collections[resource.CollectionID]; !ok {
			return fmt.Errorf("resource %q has missing collection", id)
		}
		if err := state.validateBlobReference(resource.Blob, objects); err != nil {
			return fmt.Errorf("resource %q blob: %w", id, err)
		}
	}
	for _, reference := range state.documentResources {
		if _, ok := state.documents[reference.DocumentID]; !ok {
			return fmt.Errorf("document_resource has missing document")
		}
		if _, ok := state.resources[reference.ResourceID]; !ok {
			return fmt.Errorf("document_resource has missing resource")
		}
	}
	for id, link := range state.links {
		if _, ok := state.documents[link.SourceDocumentID]; !ok {
			return fmt.Errorf("link %q has missing source document", id)
		}
		if link.TargetDocumentID != "" {
			if _, ok := state.documents[link.TargetDocumentID]; !ok {
				return fmt.Errorf("link %q has missing target document", id)
			}
		}
		if link.TargetResourceID != "" {
			if _, ok := state.resources[link.TargetResourceID]; !ok {
				return fmt.Errorf("link %q has missing target resource", id)
			}
		}
	}
	for documentID := range state.provenance {
		if _, ok := state.documents[documentID]; !ok {
			return fmt.Errorf("provenance has missing document %q", documentID)
		}
	}
	for key, bundle := range state.sourceBundles {
		if _, ok := state.collections[bundle.CollectionID]; !ok {
			return fmt.Errorf("source bundle %q has missing collection", key)
		}
		if err := state.validateBlobReference(bundle.Content, objects); err != nil {
			return fmt.Errorf("source bundle %q content: %w", key, err)
		}
	}
	return nil
}

func (state *verificationState) validateNotebookDepth(start string, maxDepth int) error {
	seen := map[string]bool{}
	current := start
	for depth := 0; current != ""; depth++ {
		if depth >= maxDepth {
			return fmt.Errorf("notebook %q exceeds depth %d", start, maxDepth)
		}
		if seen[current] {
			return fmt.Errorf("notebook %q participates in a cycle", start)
		}
		seen[current] = true
		current = state.notebooks[current].ParentID
	}
	return nil
}

func (state *verificationState) validateBlobReference(reference BlobReference, objects map[string]Object) error {
	object, ok := objects[reference.SHA256]
	if !ok || object.Kind != "blob" {
		return fmt.Errorf("missing blob object %s", reference.SHA256)
	}
	if object.SizeBytes != reference.SizeBytes {
		return fmt.Errorf("blob size mismatch")
	}
	state.referencedBlobs[reference.SHA256] = true
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

func addUnique[T any](values map[string]T, key string, value T) error {
	if _, exists := values[key]; exists {
		return fmt.Errorf("duplicate record key %q", key)
	}
	values[key] = value
	return nil
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

// SortedObjects returns a deterministic descriptor order for P3 writers and
// fixture builders.
func SortedObjects(objects []Object) []Object {
	result := append([]Object(nil), objects...)
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}
