// Package archivev2 defines and verifies the lossless Notrios native archive
// v2 format. It deliberately contains no canonical-store write path: P2
// validates an entire archive before P3 export and P4 restore are implemented.
package archivev2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

const (
	FormatName           = "notrios-archive"
	FormatVersion        = 2
	MinimumSchemaVersion = 12

	RecordsMediaType = "application/vnd.notrios.archive-v2-records+jsonl"

	CapabilitySHA256Objects = "objects.sha256.v1"
	CapabilityRecordsJSONL  = "records.jsonl.v1"
	CapabilityIdentity      = "identity.database-replica.v1"
	CapabilityRevisions     = "revisions.complete.v1"

	TargetFullArchive        = store.SelectionTargetFullArchive
	TargetSubsetTransfer     = store.SelectionTargetSubsetTransfer
	TargetPublicationHandoff = store.SelectionTargetPublicationHandoff
)

var (
	requiredBaseCapabilities = []string{
		CapabilityIdentity,
		CapabilitySHA256Objects,
		CapabilityRecordsJSONL,
		CapabilityRevisions,
	}
	idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]{1,127}$`)
)

// Limits are hard admission bounds. Verification may use stricter caller
// limits, but never wider values than these defaults without a new format
// review and evidence.
type Limits struct {
	MaxManifestBytes       int64
	MaxObjects             int
	MaxRecords             int
	MaxRecordsPerObject    int
	MaxRecordBytes         int
	MaxRecordObjectBytes   int64
	MaxBlobBytes           int64
	MaxTotalBytes          int64
	MaxPathBytes           int
	MaxPathDepth           int
	MaxJSONDepth           int
	MaxCollections         int
	MaxNotebookDepth       int
	MaxCapabilities        int
	MaxCapabilityNameBytes int
}

func DefaultLimits() Limits {
	return Limits{
		MaxManifestBytes:       4 << 20,
		MaxObjects:             10_000,
		MaxRecords:             store.MaxSelectionDocuments * 12,
		MaxRecordsPerObject:    10_000,
		MaxRecordBytes:         1 << 20,
		MaxRecordObjectBytes:   16 << 20,
		MaxBlobBytes:           16 << 30,
		MaxTotalBytes:          4 << 40,
		MaxPathBytes:           255,
		MaxPathDepth:           8,
		MaxJSONDepth:           32,
		MaxCollections:         1_000,
		MaxNotebookDepth:       32,
		MaxCapabilities:        64,
		MaxCapabilityNameBytes: 128,
	}
}

type Manifest struct {
	Format        string           `json:"format"`
	Version       int              `json:"version"`
	CommitSHA256  string           `json:"commit_sha256,omitempty"`
	Snapshot      SnapshotMetadata `json:"snapshot"`
	Compatibility Compatibility    `json:"compatibility"`
	Objects       []Object         `json:"objects"`
	Counts        Counts           `json:"counts"`
}

type SnapshotMetadata struct {
	ID                      string   `json:"id"`
	CreatedAt               string   `json:"created_at"`
	Target                  string   `json:"target"`
	DatabaseID              string   `json:"database_id"`
	SourceReplicaID         string   `json:"source_replica_id"`
	Consistency             string   `json:"consistency"`
	CollectionIDs           []string `json:"collection_ids"`
	SelectionManifestSHA256 string   `json:"selection_manifest_sha256"`
}

type Compatibility struct {
	MinimumReaderVersion int      `json:"minimum_reader_version"`
	SourceSchemaVersion  int      `json:"source_schema_version"`
	MinimumSchemaVersion int      `json:"minimum_schema_version"`
	MaximumSchemaVersion int      `json:"maximum_schema_version"`
	RequiredCapabilities []string `json:"required_capabilities"`
	OptionalCapabilities []string `json:"optional_capabilities"`
}

type Object struct {
	SHA256       string `json:"sha256"`
	Path         string `json:"path"`
	Kind         string `json:"kind"`
	MediaType    string `json:"media_type"`
	SizeBytes    int64  `json:"size_bytes"`
	Records      int    `json:"records"`
	RecordCounts Counts `json:"record_counts"`
}

type Counts struct {
	Collections       int `json:"collections"`
	Notebooks         int `json:"notebooks"`
	SearchNotebooks   int `json:"search_notebooks"`
	Tags              int `json:"tags"`
	Documents         int `json:"documents"`
	Revisions         int `json:"revisions"`
	DocumentTags      int `json:"document_tags"`
	Resources         int `json:"resources"`
	DocumentResources int `json:"document_resources"`
	Links             int `json:"links"`
	Provenance        int `json:"provenance"`
	SourceBundles     int `json:"source_bundles"`
}

func (c Counts) Total() int {
	return c.Collections + c.Notebooks + c.SearchNotebooks + c.Tags + c.Documents + c.Revisions +
		c.DocumentTags + c.Resources + c.DocumentResources + c.Links +
		c.Provenance + c.SourceBundles
}

func (c *Counts) Add(other Counts) {
	c.Collections += other.Collections
	c.Notebooks += other.Notebooks
	c.SearchNotebooks += other.SearchNotebooks
	c.Tags += other.Tags
	c.Documents += other.Documents
	c.Revisions += other.Revisions
	c.DocumentTags += other.DocumentTags
	c.Resources += other.Resources
	c.DocumentResources += other.DocumentResources
	c.Links += other.Links
	c.Provenance += other.Provenance
	c.SourceBundles += other.SourceBundles
}

func RequiredCapabilities() []string {
	return append([]string(nil), requiredBaseCapabilities...)
}

// ComputeCommitSHA256 hashes the canonical semantic manifest with the commit
// field omitted. It binds snapshot identity, compatibility, object inventory,
// and counts without relying on JSON whitespace or object write times.
func ComputeCommitSHA256(manifest Manifest) (string, error) {
	manifest.CommitSHA256 = ""
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func FinalizeManifest(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is required")
	}
	digest, err := ComputeCommitSHA256(*manifest)
	if err != nil {
		return err
	}
	manifest.CommitSHA256 = digest
	return nil
}

func validateManifest(manifest Manifest, limits Limits) error {
	if manifest.Format != FormatName || manifest.Version != FormatVersion {
		return fmt.Errorf("unsupported archive format/version %q/%d", manifest.Format, manifest.Version)
	}
	if !validSHA256(manifest.CommitSHA256) || !validSHA256(manifest.Snapshot.SelectionManifestSHA256) {
		return fmt.Errorf("manifest and selection digests must be lowercase SHA-256")
	}
	wantCommit, err := ComputeCommitSHA256(manifest)
	if err != nil || wantCommit != manifest.CommitSHA256 {
		return fmt.Errorf("manifest commit checksum mismatch")
	}
	if !validID(manifest.Snapshot.ID) || !validID(manifest.Snapshot.DatabaseID) || !validID(manifest.Snapshot.SourceReplicaID) {
		return fmt.Errorf("snapshot, database, and replica IDs must be bounded opaque IDs")
	}
	created, err := time.Parse(time.RFC3339Nano, manifest.Snapshot.CreatedAt)
	if err != nil || created.Location() != time.UTC {
		return fmt.Errorf("snapshot created_at must be RFC3339 UTC")
	}
	switch manifest.Snapshot.Target {
	case TargetFullArchive, TargetSubsetTransfer, TargetPublicationHandoff:
	default:
		return fmt.Errorf("unsupported snapshot target %q", manifest.Snapshot.Target)
	}
	if manifest.Snapshot.Consistency != "sqlite_read_transaction" {
		return fmt.Errorf("unsupported snapshot consistency %q", manifest.Snapshot.Consistency)
	}
	if len(manifest.Snapshot.CollectionIDs) == 0 || len(manifest.Snapshot.CollectionIDs) > limits.MaxCollections || !sortedUniqueIDs(manifest.Snapshot.CollectionIDs) {
		return fmt.Errorf("collection_ids must be non-empty, sorted, unique, and bounded")
	}
	compat := manifest.Compatibility
	if compat.MinimumReaderVersion != FormatVersion {
		return fmt.Errorf("unsupported minimum archive reader version %d", compat.MinimumReaderVersion)
	}
	if compat.MinimumSchemaVersion < MinimumSchemaVersion || compat.SourceSchemaVersion < compat.MinimumSchemaVersion || compat.SourceSchemaVersion > compat.MaximumSchemaVersion || store.CurrentSchemaVersion < compat.MinimumSchemaVersion || store.CurrentSchemaVersion > compat.MaximumSchemaVersion {
		return fmt.Errorf("unsupported schema bounds source=%d reader=%d range=%d..%d", compat.SourceSchemaVersion, store.CurrentSchemaVersion, compat.MinimumSchemaVersion, compat.MaximumSchemaVersion)
	}
	if err := validateCapabilities(compat, limits); err != nil {
		return err
	}
	if len(manifest.Objects) == 0 || len(manifest.Objects) > limits.MaxObjects {
		return fmt.Errorf("object count is outside 1..%d", limits.MaxObjects)
	}
	if err := validateCounts(manifest.Counts, limits.MaxRecords); err != nil {
		return fmt.Errorf("manifest counts: %w", err)
	}
	var descriptorCounts Counts
	var totalBytes int64
	seenHashes := map[string]bool{}
	previousPath := ""
	for _, object := range manifest.Objects {
		if object.Path <= previousPath {
			return fmt.Errorf("objects must be sorted by unique path")
		}
		previousPath = object.Path
		if !validSHA256(object.SHA256) || seenHashes[object.SHA256] {
			return fmt.Errorf("object SHA-256 values must be valid and unique: %q", object.SHA256)
		}
		seenHashes[object.SHA256] = true
		if object.Path != objectPath(object.SHA256) || len(object.Path) > limits.MaxPathBytes {
			return fmt.Errorf("object path does not match SHA-256: %q", object.Path)
		}
		if object.SizeBytes < 0 || object.SizeBytes > limits.MaxBlobBytes || totalBytes > limits.MaxTotalBytes-object.SizeBytes {
			return fmt.Errorf("object or total byte limit exceeded")
		}
		totalBytes += object.SizeBytes
		switch object.Kind {
		case "records":
			if object.MediaType != RecordsMediaType || object.SizeBytes > limits.MaxRecordObjectBytes || object.Records <= 0 || object.Records > limits.MaxRecordsPerObject || object.RecordCounts.Total() != object.Records {
				return fmt.Errorf("invalid records object %q", object.Path)
			}
			if err := validateCounts(object.RecordCounts, limits.MaxRecordsPerObject); err != nil {
				return fmt.Errorf("record object %q: %w", object.Path, err)
			}
			descriptorCounts.Add(object.RecordCounts)
		case "blob":
			if object.Records != 0 || object.RecordCounts.Total() != 0 {
				return fmt.Errorf("blob object %q declares records", object.Path)
			}
			if _, err := normalizeMediaType(object.MediaType); err != nil {
				return fmt.Errorf("blob object %q: %w", object.Path, err)
			}
		default:
			return fmt.Errorf("unsupported object kind %q", object.Kind)
		}
	}
	if descriptorCounts != manifest.Counts {
		return fmt.Errorf("manifest counts do not match object descriptors")
	}
	return nil
}

func validateCounts(counts Counts, max int) error {
	values := []int{counts.Collections, counts.Notebooks, counts.SearchNotebooks, counts.Tags, counts.Documents, counts.Revisions, counts.DocumentTags, counts.Resources, counts.DocumentResources, counts.Links, counts.Provenance, counts.SourceBundles}
	for _, value := range values {
		if value < 0 {
			return fmt.Errorf("negative record count")
		}
	}
	if counts.Total() > max {
		return fmt.Errorf("record count %d exceeds %d", counts.Total(), max)
	}
	return nil
}

func validateCapabilities(compat Compatibility, limits Limits) error {
	if len(compat.RequiredCapabilities)+len(compat.OptionalCapabilities) > limits.MaxCapabilities || !sortedUnique(compat.RequiredCapabilities) || !sortedUnique(compat.OptionalCapabilities) {
		return fmt.Errorf("capabilities must be sorted, unique, and bounded")
	}
	supported := map[string]bool{}
	for _, capability := range requiredBaseCapabilities {
		supported[capability] = true
	}
	for _, capability := range compat.RequiredCapabilities {
		if len(capability) == 0 || len(capability) > limits.MaxCapabilityNameBytes || !supported[capability] {
			return fmt.Errorf("unsupported required capability %q", capability)
		}
	}
	for _, required := range requiredBaseCapabilities {
		if !contains(compat.RequiredCapabilities, required) {
			return fmt.Errorf("missing required base capability %q", required)
		}
	}
	for _, capability := range compat.OptionalCapabilities {
		if len(capability) == 0 || len(capability) > limits.MaxCapabilityNameBytes {
			return fmt.Errorf("invalid optional capability")
		}
		if contains(compat.RequiredCapabilities, capability) {
			return fmt.Errorf("capability %q is both required and optional", capability)
		}
	}
	return nil
}

func validID(value string) bool { return idPattern.MatchString(value) }

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func objectPath(hash string) string { return "objects/sha256/" + hash[:2] + "/" + hash }

func sortedUnique(values []string) bool {
	return sort.StringsAreSorted(values) && unique(values)
}

func sortedUniqueIDs(values []string) bool {
	return sortedUnique(values) && all(values, validID)
}

func unique(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] {
			return false
		}
	}
	return true
}

func all(values []string, predicate func(string) bool) bool {
	for _, value := range values {
		if !predicate(value) {
			return false
		}
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
