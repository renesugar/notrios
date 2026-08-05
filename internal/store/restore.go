package store

import (
	"context"
	"io"
	"time"
)

// RestoreRecords is one bounded batch of canonical rows recovered from a
// native archive. Restore preserves exact identities and timestamps, which the
// ordinary create/update paths deliberately do not: those mint new revisions.
//
// A batch is applied in one SQLite transaction. Callers must respect
// dependency order across batches — containers, then documents and resources,
// then relations — because the archive emits records in writer order rather
// than in restore order.
type RestoreRecords struct {
	Collections       []ExportCollection
	Notebooks         []Notebook
	SearchNotebooks   []SearchNotebook
	Tags              []ExportTag
	Documents         []RestoreDocument
	Revisions         []ExportRevision
	DocumentTags      []ExportDocumentTag
	Resources         []ExportResource
	DocumentResources []ExportDocumentResource
	Links             []DocumentLink
	Provenance        []DocumentSource
	SourceBundles     []SourceBundleItem
}

// RestoreDocument carries one document row. The documents table denormalizes
// the current revision's title and the search index needs its body, but a
// revision may live in a different records chunk than its document, so neither
// is supplied here: FinalizeRestoredDocuments derives both with a join once
// every document and revision has been written.
type RestoreDocument struct {
	Document     ExportDocument
	BodyMIMEType string
}

// RestoreConflict reports a record an additive import could not apply because
// the target already holds that identity. Merge restores report conflicts
// rather than overwriting canonical state.
type RestoreConflict struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// RestoreTarget is the canonical write surface a restore uses. It is kept
// separate from Store for the same reason ExportReader is: it admits exact
// identities and raw blob content, which no REST or MCP adapter may reach.
type RestoreTarget interface {
	Bootstrap(ctx context.Context) error
	Status(ctx context.Context) (StoreStatus, error)
	GetDatabaseIdentity(ctx context.Context) (DatabaseIdentity, error)

	// LibraryIsEmpty reports whether the target holds any document, which is
	// what distinguishes an adopt target from a replace target.
	LibraryIsEmpty(ctx context.Context) (bool, error)
	// ClearLibraryForReplace removes canonical note state so a replacement
	// restore starts from a fresh initialized database. Builtin bootstrap rows
	// are recreated afterwards.
	ClearLibraryForReplace(ctx context.Context) error
	// AdoptDatabaseIdentity sets the logical database ID and always mints a new
	// replica ID, so a restored writable copy never impersonates its source.
	AdoptDatabaseIdentity(ctx context.Context, databaseID string) (DatabaseIdentity, error)

	// AdmitRestoredBlob streams archive bytes into the asset store, sniffing
	// and recording the canonical MIME type rather than trusting the archive.
	AdmitRestoredBlob(ctx context.Context, expectedSHA256, mimeType string, content io.Reader) (int64, error)
	// AdmitRestoredSourceBundle streams exact source bytes into the
	// source-bundle namespace, which sits outside `blobs` and resource garbage
	// collection. It returns the storage path the bundle row must record.
	AdmitRestoredSourceBundle(ctx context.Context, expectedSHA256 string, content io.Reader) (int64, string, error)
	// ApplyRestoreRecords writes one batch atomically, reporting identities it
	// refused to overwrite when additive is true.
	ApplyRestoreRecords(ctx context.Context, batch RestoreRecords, additive bool) ([]RestoreConflict, error)
	// FinalizeRestoredDocuments fills each document's denormalized title from
	// its current revision and rebuilds the search index for current notes.
	// It runs once, after every document and revision is present.
	FinalizeRestoredDocuments(ctx context.Context) error

	// BeginRestore durably records that a restore is underway, and
	// CompleteRestore clears it. A restore writes many transactions, so a crash
	// between them leaves real rows behind; without a marker that library is
	// indistinguishable from a complete one, and a half-restored backup that
	// looks whole is worse than one that obviously failed.
	BeginRestore(ctx context.Context, marker RestoreMarker) error
	CompleteRestore(ctx context.Context) error
	// PendingRestore reports an unfinished restore, if any.
	PendingRestore(ctx context.Context) (RestoreMarker, bool, error)
}

// RestoreMarker identifies the restore that is underway, so an interrupted
// library reports which archive it was being rebuilt from rather than only
// that something failed.
type RestoreMarker struct {
	SnapshotID   string `json:"snapshot_id"`
	CommitSHA256 string `json:"commit_sha256"`
	Intent       string `json:"intent"`
	StartedAt    string `json:"started_at,omitempty"`
}

// RestoreSummary is the aggregate result of one restore.
type RestoreSummary struct {
	Intent       string            `json:"intent"`
	DatabaseID   string            `json:"database_id"`
	ReplicaID    string            `json:"replica_id"`
	Applied      RestoreCounts     `json:"applied"`
	Conflicts    []RestoreConflict `json:"conflicts"`
	BlobBytes    int64             `json:"blob_bytes"`
	Blobs        int               `json:"blobs"`
	Elapsed      float64           `json:"elapsed_seconds"`
	Warnings     []string          `json:"warnings"`
	SourceBundle SourceBundleNote  `json:"source_bundles"`
}

// SourceBundleNote records a deliberate fidelity limit: archives carry hashed
// source and item keys so local paths never travel, so a restored bundle keeps
// its exact bytes but cannot reproduce the original importer keys.
type SourceBundleNote struct {
	Restored    int    `json:"restored"`
	KeysAreHash bool   `json:"keys_are_hashes"`
	Note        string `json:"note,omitempty"`
}

type RestoreCounts struct {
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

func (c *RestoreCounts) addBatch(batch RestoreRecords) {
	c.Collections += len(batch.Collections)
	c.Notebooks += len(batch.Notebooks)
	c.SearchNotebooks += len(batch.SearchNotebooks)
	c.Tags += len(batch.Tags)
	c.Documents += len(batch.Documents)
	c.Revisions += len(batch.Revisions)
	c.DocumentTags += len(batch.DocumentTags)
	c.Resources += len(batch.Resources)
	c.DocumentResources += len(batch.DocumentResources)
	c.Links += len(batch.Links)
	c.Provenance += len(batch.Provenance)
	c.SourceBundles += len(batch.SourceBundles)
}

// AddBatch is exported so an archive reader can account for what it applied.
func (c *RestoreCounts) AddBatch(batch RestoreRecords) { c.addBatch(batch) }

func restoreTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
