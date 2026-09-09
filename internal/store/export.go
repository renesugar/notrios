package store

import (
	"context"
	"io"
	"time"
)

// SourceBundleKey is the raw composite identity of one exact source-bundle
// item. Raw keys may contain local filesystem paths, so they never cross the
// REST/MCP planner boundary; only in-process writers such as the archive-v2
// exporter resolve them, and they hash the keys before encoding records.
type SourceBundleKey struct {
	SourceSystem string
	SourceKey    string
	CollectionID string
	ItemKey      string
}

// SelectionResolution is the in-process view of one selection plan. The plan
// itself keeps REST/MCP-safe capped detail arrays; the resolution additionally
// carries the complete selected identity sets a local writer needs in order to
// stream every selected record. It is never returned by a network adapter.
type SelectionResolution struct {
	Plan             SelectionPlan
	DocumentIDs      []string
	ResourceIDs      []string
	SourceBundleKeys []SourceBundleKey
}

// ExportCollection is one canonical collection row. Capabilities and settings
// are reported in the canonical form the archive records expect.
type ExportCollection struct {
	ID           string
	Name         string
	Description  string
	Capabilities []string
	CreatedAt    time.Time
}

// ExportTag is one canonical tag row including its creation time. The sidebar
// tag listing reports live note counts instead and is not archivable state.
type ExportTag struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// ExportDocument is the canonical document identity row without body text.
type ExportDocument struct {
	ID                string
	CollectionID      string
	NotebookID        string
	CurrentRevisionID string
	DeletedAt         time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ExportRevision is one durable revision including its body. Bodies are
// visited one row at a time so an exporter never buffers a whole batch of
// note bodies in memory.
type ExportRevision struct {
	ID           string
	DocumentID   string
	Title        string
	Body         string
	BodyMIMEType string
	MetadataJSON string
	Message      string
	CreatedAt    time.Time
}

// ExportDocumentTag is one note/tag membership row.
type ExportDocumentTag struct {
	DocumentID string
	TagID      string
}

// ExportResource is one logical resource plus the physical blob identity that
// backs it. Storage paths stay inside the store package.
type ExportResource struct {
	ID                 string
	CollectionID       string
	BlobSHA256         string
	BlobSizeBytes      int64
	Filename           string
	MIMEType           string
	MetadataJSON       string
	UnreferencedAt     time.Time
	UnreferencedReason string
	CreatedAt          time.Time
}

// ExportDocumentResource is one document/resource relation row.
type ExportDocumentResource struct {
	DocumentID   string
	ResourceID   string
	RelationType string
	Ordinal      int
	AnchorJSON   string
}

// ExportReader is the read-only canonical surface a local archive writer uses.
// Every method is bounded: identity sets are supplied by the caller in batches,
// and the two large collections (revisions and links) are streamed through a
// visitor instead of being returned as slices.
//
// This interface is deliberately not part of Store: it exposes complete
// identity sets and blob content streams that must never be reachable from a
// REST or MCP adapter.
type ExportReader interface {
	// WithReadSnapshot runs fn inside one SQLite read transaction so an export
	// observes a single consistent canonical state. The caller must own the
	// store handle exclusively for the duration.
	WithReadSnapshot(ctx context.Context, fn func() error) error
	ResolveSelection(ctx context.Context, req SelectionPlanRequest) (SelectionResolution, error)
	GetDatabaseIdentity(ctx context.Context) (DatabaseIdentity, error)
	Status(ctx context.Context) (StoreStatus, error)

	ExportCollections(ctx context.Context, ids []string) ([]ExportCollection, error)
	ListNotebooks(ctx context.Context) ([]Notebook, error)
	ListSearchNotebooks(ctx context.Context) ([]SearchNotebook, error)
	ExportTags(ctx context.Context, ids []string) ([]ExportTag, error)
	ExportDocuments(ctx context.Context, ids []string) ([]ExportDocument, error)
	ExportRevisions(ctx context.Context, documentIDs []string, currentOnly bool, visit func(ExportRevision) error) error
	ExportDocumentTags(ctx context.Context, documentIDs []string) ([]ExportDocumentTag, error)
	ExportResources(ctx context.Context, ids []string) ([]ExportResource, error)
	ExportDocumentResources(ctx context.Context, documentIDs []string) ([]ExportDocumentResource, error)
	ExportLinks(ctx context.Context, documentIDs []string, visit func(DocumentLink) error) error
	ExportProvenance(ctx context.Context, documentIDs []string) ([]DocumentSource, error)
	ExportSourceBundles(ctx context.Context, keys []SourceBundleKey) ([]SourceBundleItem, error)

	OpenBlobContent(ctx context.Context, sha256Hex string) (io.ReadCloser, int64, error)
	OpenSourceBundleContent(ctx context.Context, item SourceBundleItem) (io.ReadCloser, error)
}
