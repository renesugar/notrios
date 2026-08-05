package store

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	ErrNotFound             = errors.New("not found")
	ErrConflict             = errors.New("revision conflict")
	ErrPreconditionRequired = errors.New("base revision is required")
	ErrInvalidInput         = errors.New("invalid input")
	ErrNameConflict         = errors.New("name already in use")
	ErrProtected            = errors.New("builtin object cannot be modified")
	ErrInvalidCursor        = errors.New("invalid cursor")
)

// DefaultNotebookID is the bootstrap "Notes" notebook that holds managed
// documents whose notebook is not specified. It cannot be deleted.
const DefaultNotebookID = "nb_notes"

// HelpNotebookID holds the built-in read-only documentation notes.
const HelpNotebookID = "nb_help"

// Builtin search-notebook IDs. "All notes" sorts first in sidebars and Trash
// sorts last; neither can be deleted.
const (
	AllNotesSearchNotebookID = "snb_all_notes"
	TrashSearchNotebookID    = "snb_trash"
)

// CurrentSchemaVersion is the canonical SQLite schema understood by this
// build. Archive-v2 manifests record this source schema but never include
// derived FTS5 or Recoll state.
const CurrentSchemaVersion = 13

// DatabaseIdentity separates the stable logical synchronization/archive
// universe from one writable database copy. Copy/restore workflows preserve
// DatabaseID only when explicitly requested and always mint a ReplicaID.
type DatabaseIdentity struct {
	DatabaseID       string    `json:"database_id"`
	ReplicaID        string    `json:"replica_id"`
	CreatedAt        time.Time `json:"created_at"`
	ReplicaCreatedAt time.Time `json:"replica_created_at"`
}

// Selection planner limits keep read-only dry runs bounded even when the
// canonical library is large. P3 can stream the same manifest contract into
// an archive without widening REST or MCP inputs.
const (
	MaxSelectionSelectors     = 100
	MaxSelectionSelectorBytes = 512
	MaxSelectionDocumentIDs   = 1000
	MaxSelectionDocuments     = 1_000_000
	MaxSelectionDetailItems   = 1000
)

const (
	SelectionTargetFullArchive        = "full_archive"
	SelectionTargetSubsetTransfer     = "subset_transfer"
	SelectionTargetPublicationHandoff = "publication_handoff"
)

// SelectionSpec identifies notes without exposing SQL or filesystem paths.
// Values within one selector type are ORed; Match controls how the populated
// selector types combine ("any" or "all").
type SelectionSpec struct {
	CollectionID               string   `json:"collection_id,omitempty"`
	NotebookIDs                []string `json:"notebook_ids,omitempty"`
	IncludeNotebookDescendants *bool    `json:"include_notebook_descendants,omitempty"`
	Tags                       []string `json:"tags,omitempty"`
	Query                      string   `json:"query,omitempty"`
	DocumentIDs                []string `json:"document_ids,omitempty"`
	Match                      string   `json:"match,omitempty"`
}

// PrivacyPolicy is a reusable policy override. Pointer booleans distinguish
// secure target defaults from an explicit opt-in. Publication defaults strip
// provenance/private metadata/source bundles and exclude common private tags.
type PrivacyPolicy struct {
	ExcludeTags            []string `json:"exclude_tags,omitempty"`
	PrivateTags            []string `json:"private_tags,omitempty"`
	LinkAction             string   `json:"link_action,omitempty"`
	IncludeSourceBundles   *bool    `json:"include_source_bundles,omitempty"`
	IncludeProvenance      *bool    `json:"include_provenance,omitempty"`
	IncludePrivateMetadata *bool    `json:"include_private_metadata,omitempty"`
	IncludeTrashed         *bool    `json:"include_trashed,omitempty"`
	MaxResourceBytes       int64    `json:"max_resource_bytes,omitempty"`
}

// EffectivePrivacyPolicy records the concrete target policy used by a plan.
type EffectivePrivacyPolicy struct {
	ExcludeTags            []string `json:"exclude_tags"`
	PrivateTags            []string `json:"private_tags"`
	LinkAction             string   `json:"link_action"`
	IncludeSourceBundles   bool     `json:"include_source_bundles"`
	IncludeProvenance      bool     `json:"include_provenance"`
	IncludePrivateMetadata bool     `json:"include_private_metadata"`
	IncludeTrashed         bool     `json:"include_trashed"`
	MaxResourceBytes       int64    `json:"max_resource_bytes,omitempty"`
}

type SelectionPlanRequest struct {
	Target       string        `json:"target"`
	Selection    SelectionSpec `json:"selection"`
	Policy       PrivacyPolicy `json:"policy"`
	DetailLimit  int           `json:"detail_limit,omitempty"`
	MaxDocuments int           `json:"max_documents,omitempty"`
}

type SelectionPlanCounts struct {
	SelectedDocuments      int `json:"selected_documents"`
	ExcludedDocuments      int `json:"excluded_documents"`
	ReachableResources     int `json:"reachable_resources"`
	AvailableSourceBundles int `json:"available_source_bundles"`
	IncludedSourceBundles  int `json:"included_source_bundles"`
	InternalLinks          int `json:"internal_links"`
	PrivateLinks           int `json:"private_links"`
	BrokenLinks            int `json:"broken_links"`
	ExternalLinks          int `json:"external_links"`
	OversizedResources     int `json:"oversized_resources"`
}

type SelectionDocumentManifest struct {
	ID                string   `json:"id"`
	URI               string   `json:"uri"`
	CollectionID      string   `json:"collection_id"`
	NotebookID        string   `json:"notebook_id"`
	CurrentRevisionID string   `json:"current_revision_id"`
	Deleted           bool     `json:"deleted"`
	InclusionReasons  []string `json:"inclusion_reasons"`
}

type SelectionResourceManifest struct {
	ID           string `json:"id"`
	URI          string `json:"uri"`
	CollectionID string `json:"collection_id"`
	MIMEType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	Oversized    bool   `json:"oversized,omitempty"`
}

type SelectionLinkDecision struct {
	SourceDocumentID string `json:"source_document_id"`
	TargetDocumentID string `json:"target_document_id,omitempty"`
	LinkSHA256       string `json:"link_sha256"`
	Classification   string `json:"classification"`
	ResolutionStatus string `json:"resolution_status"`
	Action           string `json:"action"`
}

type SelectionSourceBundleManifest struct {
	SourceSystem    string `json:"source_system"`
	SourceKeySHA256 string `json:"source_key_sha256"`
	CollectionID    string `json:"collection_id"`
	ItemKeySHA256   string `json:"item_key_sha256"`
	ItemType        string `json:"item_type"`
	SHA256          string `json:"sha256"`
	SizeBytes       int64  `json:"size_bytes"`
}

type SelectionExclusion struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type SelectionMetadataDecision struct {
	Field         string `json:"field"`
	Action        string `json:"action"`
	Reason        string `json:"reason"`
	AffectedItems int    `json:"affected_items"`
}

// SelectionPlan is a content-free dry-run manifest: it contains stable IDs,
// hashes, counts, and policy decisions, but never note bodies, source metadata
// JSON, local paths, or resource bytes. Detail arrays are capped while the
// digest and counts cover the complete bounded selection.
type SelectionPlan struct {
	Version           int                             `json:"version"`
	Target            string                          `json:"target"`
	Policy            EffectivePrivacyPolicy          `json:"policy"`
	ManifestSHA256    string                          `json:"manifest_sha256"`
	Counts            SelectionPlanCounts             `json:"counts"`
	Documents         []SelectionDocumentManifest     `json:"documents"`
	Resources         []SelectionResourceManifest     `json:"resources"`
	Links             []SelectionLinkDecision         `json:"links"`
	SourceBundles     []SelectionSourceBundleManifest `json:"source_bundles"`
	Exclusions        []SelectionExclusion            `json:"exclusions"`
	MetadataDecisions []SelectionMetadataDecision     `json:"metadata_decisions"`
	Warnings          []string                        `json:"warnings"`
	Truncated         bool                            `json:"truncated"`
}

// Collection is a logical group of documents.
type Collection struct {
	ID           string
	Name         string
	Description  string
	Capabilities []string
}

// Document is the current editable representation of a managed note.
type Document struct {
	ID                string
	URI               string
	CollectionID      string
	NotebookID        string
	Title             string
	Body              string
	BodyMIMEType      string
	CurrentRevisionID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         time.Time
}

// Notebook is a nested container for managed notes. Names are case-insensitively
// unique among siblings; an optional emoji icon is shown before the name in
// sidebars. Builtin notebooks (Help) cannot be deleted and their notes are
// read-only at the API layer.
type Notebook struct {
	ID        string
	ParentID  string
	Name      string
	IconEmoji string
	Builtin   bool
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateNotebookRequest creates a notebook, optionally nested under a parent.
type CreateNotebookRequest struct {
	PreferredID string
	ParentID    string
	Name        string
	IconEmoji   string
	Position    int
}

// UpdateNotebookRequest changes notebook fields; nil pointers leave a field
// unchanged. Setting ParentID to the empty string moves the notebook to the
// top level.
type UpdateNotebookRequest struct {
	ID        string
	Name      *string
	ParentID  *string
	IconEmoji *string
	Position  *int
}

// Tag is a note label. NoteCount counts current non-deleted notes.
type Tag struct {
	ID        string
	Name      string
	NoteCount int64
}

// SearchNotebook is a query-backed virtual notebook. Deleting one never
// deletes notes. SortAnchor is "first" (All notes), "normal", or "last" (Trash).
type SearchNotebook struct {
	ID         string
	Name       string
	IconEmoji  string
	Query      string
	Builtin    bool
	SortAnchor string
	CreatedAt  time.Time
}

// CreateSearchNotebookRequest saves a user query as a search notebook.
type CreateSearchNotebookRequest struct {
	PreferredID string
	Name        string
	IconEmoji   string
	Query       string
}

// DocumentSource records where an imported note came from. One row per
// externally-sourced document (Joplin, Obsidian, Twitter/X, ChatGPT, Claude);
// purely local notes have no row. Author (display name) and AuthorID
// (canonical account identity) are kept separate so two people with the same
// display name are never conflated. ThreadID/ReplyTo use source-native
// external IDs so conversation threads can be recovered in order and links
// can point back at the original posts.
type DocumentSource struct {
	DocumentID   string
	SourceSystem string
	ExternalID   string
	Author       string
	AuthorID     string
	ThreadID     string
	ReplyTo      string
	SourceURL    string
	PublishedAt  string
	PublishedTS  int64
	MetadataJSON string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SetDocumentSourceRequest upserts provenance for a document. PublishedAt
// accepts ISO 8601 (or epoch seconds/milliseconds as digits); the UTC Unix
// seconds used for range queries are derived from it.
type SetDocumentSourceRequest struct {
	DocumentID   string
	SourceSystem string
	ExternalID   string
	Author       string
	AuthorID     string
	ThreadID     string
	ReplyTo      string
	SourceURL    string
	PublishedAt  string
	MetadataJSON string
}

// DocumentRevision is one durable saved state for a managed document.
type DocumentRevision struct {
	ID           string
	DocumentID   string
	Title        string
	Body         string
	BodyMIMEType string
	Message      string
	CreatedAt    time.Time
}

// Resource is a logical attachment or embedded resource. Multiple resources may
// point at the same content-addressed blob.
type Resource struct {
	ID           string    `json:"id"`
	URI          string    `json:"uri"`
	CollectionID string    `json:"collection_id"`
	Filename     string    `json:"filename,omitempty"`
	MIMEType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	SHA256       string    `json:"sha256"`
	CreatedAt    time.Time `json:"created_at"`
}

// ResourceReference connects a document to a resource without duplicating bytes.
type ResourceReference struct {
	DocumentID   string
	ResourceID   string
	Resource     Resource
	RelationType string
	Ordinal      int
	AnchorJSON   string
}

// CreateResourceRequest streams bytes into the content-addressed blob store.
type CreateResourceRequest struct {
	PreferredID  string
	CollectionID string
	Filename     string
	MIMEType     string
	Content      io.Reader
}

// UpdateResourceRequest replaces one logical resource's content/metadata while
// preserving its stable resource ID and document references.
type UpdateResourceRequest struct {
	ID       string
	Filename string
	MIMEType string
	Content  io.Reader
}

// ImportCheckpoint is durable progress for one source directory and target
// collection. ReportJSON is importer-owned, versioned JSON.
type ImportCheckpoint struct {
	SourceSystem         string
	SourceKey            string
	CollectionID         string
	InventoryFingerprint string
	Phase                string
	NextIndex            int
	TotalItems           int
	ProcessedItems       int
	Status               string
	ReportJSON           string
	UpdatedAt            time.Time
	CompletedAt          time.Time
}

// ImportItemState stores the last successfully applied source fingerprint.
type ImportItemState struct {
	SourceSystem string
	SourceKey    string
	CollectionID string
	ItemKey      string
	ItemType     string
	Fingerprint  string
	TargetID     string
	Action       string
	ProcessedAt  time.Time
}

// ImportDocumentMutation is one planned canonical document change inside a
// bounded importer transaction. SkipDocument records only State (used when an
// externally sourced document is already trashed). Action is create, update,
// or unchanged. Tags and resources are applied after every document in the
// batch is visible, preserving source relations without per-field transactions.
type ImportDocumentMutation struct {
	Action         string
	Document       CreateDocumentRequest
	BaseRevisionID string
	Source         SetDocumentSourceRequest
	AddTags        []string
	RemoveTags     []string
	Resources      []AttachResourceRequest
	State          ImportItemState
	SkipDocument   bool
	SkipSource     bool
	SkipState      bool
}

// ImportDocumentBatchRequest atomically applies a bounded set of document
// mutations, item fingerprints, and the checkpoint that makes them durable.
type ImportDocumentBatchRequest struct {
	Documents  []ImportDocumentMutation
	Checkpoint ImportCheckpoint
}

// ImportLinkBatchRequest atomically refreshes links for a bounded set of
// already-created documents and advances the import checkpoint.
type ImportLinkBatchRequest struct {
	DocumentIDs []string
	Checkpoint  ImportCheckpoint
}

// SourceBundleItem identifies one exact source file in an optional bundle.
type SourceBundleItem struct {
	SourceSystem  string
	SourceKey     string
	CollectionID  string
	ItemKey       string
	ItemType      string
	ExternalID    string
	RelativePath  string
	SHA256        string
	SizeBytes     int64
	StoragePath   string
	PropertyOrder []string
	UpdatedAt     time.Time
}

type PutSourceBundleItemRequest struct {
	SourceSystem  string
	SourceKey     string
	CollectionID  string
	ItemKey       string
	ItemType      string
	ExternalID    string
	RelativePath  string
	PropertyOrder []string
	Content       io.Reader
}

// AttachResourceRequest creates or replaces a document-resource reference.
type AttachResourceRequest struct {
	DocumentID   string
	ResourceID   string
	RelationType string
	Ordinal      int
	AnchorJSON   string
}

// ResourceReport summarizes logical resources, physical blobs, and their
// document/notebook references. Exact SHA-256 groups are authoritative;
// perceptual results are suggestions only.
type ResourceReport struct {
	ExactDuplicates   []ExactDuplicateGroup   `json:"exact_duplicates"`
	UnreferencedBlobs []UnreferencedBlob      `json:"unreferenced_blobs"`
	NotebookUsage     []NotebookResourceUsage `json:"notebook_usage"`
	Perceptual        PerceptualHashReport    `json:"perceptual"`
}

// ExactDuplicateGroup is one physical blob referenced by multiple logical
// resources. CrossCollection is true when those resources span collections.
type ExactDuplicateGroup struct {
	SHA256          string               `json:"sha256"`
	MIMEType        string               `json:"mime_type"`
	SizeBytes       int64                `json:"size_bytes"`
	ResourceCount   int                  `json:"resource_count"`
	ReferenceCount  int                  `json:"reference_count"`
	CollectionIDs   []string             `json:"collection_ids"`
	CrossCollection bool                 `json:"cross_collection"`
	Resources       []ResourceReportItem `json:"resources"`
}

// ResourceReportItem adds total document-reference usage to resource metadata.
type ResourceReportItem struct {
	Resource       Resource `json:"resource"`
	ReferenceCount int      `json:"reference_count"`
}

// UnreferencedBlob is a physical blob with no document-resource reference
// through any of its logical resources. H5 reports it; H6 decides retention
// and deletion eligibility.
type UnreferencedBlob struct {
	SHA256    string     `json:"sha256"`
	MIMEType  string     `json:"mime_type"`
	SizeBytes int64      `json:"size_bytes"`
	Resources []Resource `json:"resources"`
}

// NotebookResourceUsage counts only current (non-trashed) documents directly
// assigned to the notebook. ReferencedBytes counts every reference; UniqueBytes
// counts each physical blob once within the notebook.
type NotebookResourceUsage struct {
	NotebookID      string `json:"notebook_id"`
	NotebookName    string `json:"notebook_name"`
	DocumentCount   int    `json:"document_count"`
	ReferenceCount  int    `json:"reference_count"`
	ResourceCount   int    `json:"resource_count"`
	UniqueBlobCount int    `json:"unique_blob_count"`
	ReferencedBytes int64  `json:"referenced_bytes"`
	UniqueBytes     int64  `json:"unique_bytes"`
}

// PerceptualHashHook is the pluggable H5 extension point. Notrios ships no
// implementation. A configured hook computes one algorithm-specific hash per
// supported blob and returns review-only candidate pairs. It must never mutate
// the store or decide exact deduplication.
type PerceptualHashHook interface {
	Algorithm() string
	SupportsMIME(mimeType string) bool
	Compute(ctx context.Context, content io.Reader, mimeType string) (string, error)
	SuggestNearDuplicates(ctx context.Context, candidates []PerceptualHashCandidate) ([]PerceptualHashSuggestion, error)
}

// PerceptualHashCandidate is the store metadata supplied to a hook.
type PerceptualHashCandidate struct {
	BlobSHA256 string `json:"blob_sha256"`
	Hash       string `json:"hash"`
	MIMEType   string `json:"mime_type"`
}

// PerceptualHashSuggestion is returned by a hook. Distance is
// algorithm-specific; lower is conventionally closer. Notrios validates that
// both blob IDs were supplied and treats the result only as a review hint.
type PerceptualHashSuggestion struct {
	LeftBlobSHA256  string  `json:"left_blob_sha256"`
	RightBlobSHA256 string  `json:"right_blob_sha256"`
	Distance        float64 `json:"distance"`
	Reason          string  `json:"reason,omitempty"`
}

// PerceptualPolicyReview is a current review rule matching a stored
// perceptual hash. Perceptual policy never blocks admission.
type PerceptualPolicyReview struct {
	Algorithm   string   `json:"algorithm"`
	Hash        string   `json:"hash"`
	BlobSHA256  string   `json:"blob_sha256"`
	ResourceIDs []string `json:"resource_ids"`
	Reason      string   `json:"reason,omitempty"`
}

// NearDuplicateReview enriches a hook suggestion with affected resources.
type NearDuplicateReview struct {
	Algorithm        string   `json:"algorithm"`
	LeftBlobSHA256   string   `json:"left_blob_sha256"`
	RightBlobSHA256  string   `json:"right_blob_sha256"`
	LeftResourceIDs  []string `json:"left_resource_ids"`
	RightResourceIDs []string `json:"right_resource_ids"`
	Distance         float64  `json:"distance"`
	Reason           string   `json:"reason,omitempty"`
}

// PerceptualHashReport exposes stored hook state and review suggestions. An
// empty/default report is the normal production state until an algorithm is
// explicitly configured.
type PerceptualHashReport struct {
	HookEnabled    bool                     `json:"hook_enabled"`
	Algorithm      string                   `json:"algorithm,omitempty"`
	StoredHashes   int                      `json:"stored_hashes"`
	PolicyReviews  []PerceptualPolicyReview `json:"policy_reviews"`
	NearDuplicates []NearDuplicateReview    `json:"near_duplicates"`
}

// GarbageCollectionPolicy is the time-based portion of H6 eligibility. A
// future sync-aware gate adds peer acknowledgement requirements without
// weakening these minimum local retention windows.
type GarbageCollectionPolicy struct {
	UnreferencedFor   time.Duration
	PurgedResourceFor time.Duration
}

// GarbageCollectionRequest plans or applies resource collection. Apply is
// false by default. Now exists for deterministic tests and reports; zero means
// the current UTC time. Gate defaults to LocalRetentionGate.
type GarbageCollectionRequest struct {
	Policy GarbageCollectionPolicy
	Apply  bool
	Now    time.Time
	Gate   RetentionGate
}

// RetentionGate is the sync-aware extension point. v0.3's local gate allows a
// time-expired unreferenced resource; v0.7 can additionally require peer
// acknowledgement watermarks before returning true.
type RetentionGate interface {
	CanCollect(ctx context.Context, candidate GarbageCollectionCandidate) (allowed bool, reason string, err error)
}

type LocalRetentionGate struct{}

func (LocalRetentionGate) CanCollect(_ context.Context, _ GarbageCollectionCandidate) (bool, string, error) {
	return true, "local_retention_satisfied", nil
}

type GarbageCollectionPolicySummary struct {
	UnreferencedSeconds   int64  `json:"unreferenced_seconds"`
	PurgedResourceSeconds int64  `json:"purged_resource_seconds"`
	Gate                  string `json:"gate"`
}

// GarbageCollectionCandidate is one logical resource with no references.
// Removing it may or may not free its shared physical blob.
type GarbageCollectionCandidate struct {
	Resource           Resource  `json:"resource"`
	UnreferencedAt     time.Time `json:"unreferenced_at"`
	UnreferencedReason string    `json:"unreferenced_reason"`
	RetentionSeconds   int64     `json:"retention_seconds"`
	EligibleAt         time.Time `json:"eligible_at"`
	Decision           string    `json:"decision"`
}

type GarbageCollectionReport struct {
	DryRun                  bool                           `json:"dry_run"`
	AsOf                    time.Time                      `json:"as_of"`
	Policy                  GarbageCollectionPolicySummary `json:"policy"`
	Eligible                []GarbageCollectionCandidate   `json:"eligible"`
	Retained                []GarbageCollectionCandidate   `json:"retained"`
	Removed                 []GarbageCollectionCandidate   `json:"removed"`
	ReferencedResourceCount int                            `json:"referenced_resource_count"`
	BlobsRemoved            int                            `json:"blobs_removed"`
	BytesRemoved            int64                          `json:"bytes_removed"`
	Warnings                []string                       `json:"warnings"`
}

// DocumentLink stores one parsed link edge from a Markdown document. It preserves
// source syntax and resolution details so the UI, exporters, MCP tools, and
// future graph backends can make policy-aware choices.
type DocumentLink struct {
	ID               int64
	SourceDocumentID string
	TargetDocumentID string
	TargetResourceID string
	TargetURI        string
	RelationType     string
	SourceFormat     string
	RawTarget        string
	DisplayText      string
	AnchorType       string
	AnchorValue      string
	Context          string
	ResolutionStatus string
	SourceStartByte  int
	SourceEndByte    int
	SourceLine       int
	SourceColumn     int
}

// DocumentLinkPage contains outgoing and/or incoming document links.
type DocumentLinkPage struct {
	Outgoing []DocumentLink
	Incoming []DocumentLink
}

// GraphRequest is a small graph-expansion request for the MVP graph endpoint.
type GraphRequest struct {
	Roots            []string
	Direction        string
	Depth            int
	IncludeResources bool
	MaxNodes         int
	MaxEdges         int
}

// GraphNode is a document or resource node returned by graph expansion.
type GraphNode struct {
	ID    string
	URI   string
	Kind  string
	Label string
}

// GraphEdge is one link edge returned by graph expansion.
type GraphEdge struct {
	ID        string
	SourceID  string
	TargetID  string
	Kind      string
	Status    string
	RawTarget string
}

// GraphResponse contains the current graph slice.
type GraphResponse struct {
	Nodes     []GraphNode
	Edges     []GraphEdge
	Truncated bool
}

// CreateDocumentRequest creates an initial managed Markdown document. An empty
// NotebookID places the note in the default "Notes" notebook.
type CreateDocumentRequest struct {
	PreferredID  string
	CollectionID string
	NotebookID   string
	Title        string
	Body         string
	BodyMIMEType string
	Message      string
}

// UpdateDocumentRequest replaces the current managed Markdown document body.
type UpdateDocumentRequest struct {
	ID             string
	Title          string
	Body           string
	BodyMIMEType   string
	BaseRevisionID string
	Message        string
}

// DeleteDocumentRequest soft-deletes a document while preserving history.
type DeleteDocumentRequest struct {
	ID             string
	BaseRevisionID string
	Message        string
}

// RestoreRevisionRequest creates a new current revision from an older revision.
type RestoreRevisionRequest struct {
	DocumentID     string
	RevisionID     string
	BaseRevisionID string
	Message        string
}

// SearchRequest is the store-level search request.
type SearchRequest struct {
	CollectionID string
	Query        string
	Limit        int
	Cursor       string
}

// SearchHit is one full-text search hit.
type SearchHit struct {
	ID            string
	URI           string
	CollectionID  string
	NotebookID    string
	Title         string
	Snippet       string
	Score         float64
	UpdatedAt     time.Time
	SearchSources []string
	sortTime      string
}

// SearchResponse is the store-level search response.
type SearchResponse struct {
	Hits       []SearchHit
	NextCursor string
	Truncated  bool
}

type DocumentPageRequest struct {
	Limit  int
	Cursor string
}

type DocumentPage struct {
	Documents  []Document
	NextCursor string
}

// StoreStatus reports implementation and schema state for health/status endpoints.
type StoreStatus struct {
	Driver        string
	Path          string
	State         string
	SchemaVersion int
}

// Store is the persistence contract used by HTTP, MCP, CLI, importers, and tests.
type Store interface {
	Close() error
	Bootstrap(ctx context.Context) error
	ListCollections(ctx context.Context) ([]Collection, error)
	CreateDocument(ctx context.Context, req CreateDocumentRequest) (Document, error)
	GetDocument(ctx context.Context, id string) (Document, error)
	UpdateDocument(ctx context.Context, req UpdateDocumentRequest) (Document, error)
	DeleteDocument(ctx context.Context, req DeleteDocumentRequest) error
	ListDocumentRevisions(ctx context.Context, documentID string) ([]DocumentRevision, error)
	GetDocumentRevision(ctx context.Context, documentID, revisionID string) (DocumentRevision, error)
	RestoreDocumentRevision(ctx context.Context, req RestoreRevisionRequest) (Document, error)
	CreateResource(ctx context.Context, req CreateResourceRequest) (Resource, error)
	UpdateResource(ctx context.Context, req UpdateResourceRequest) (Resource, error)
	GetResource(ctx context.Context, id string) (Resource, error)
	GetDocuments(ctx context.Context, ids []string) (map[string]Document, error)
	GetResources(ctx context.Context, ids []string) (map[string]Resource, error)
	GetDocumentTags(ctx context.Context, ids []string) (map[string][]Tag, error)
	OpenResourceContent(ctx context.Context, id string) (Resource, io.ReadCloser, error)
	DeleteResource(ctx context.Context, id string) error
	ListDocumentResources(ctx context.Context, documentID string) ([]ResourceReference, error)
	AttachDocumentResource(ctx context.Context, req AttachResourceRequest) (ResourceReference, error)
	DetachDocumentResource(ctx context.Context, documentID, resourceID string) error
	ResourceReport(ctx context.Context) (ResourceReport, error)
	GarbageCollect(ctx context.Context, req GarbageCollectionRequest) (GarbageCollectionReport, error)
	ListDocumentLinks(ctx context.Context, documentID, direction string) (DocumentLinkPage, error)
	RebuildDocumentLinks(ctx context.Context, documentID string) error
	Graph(ctx context.Context, req GraphRequest) (GraphResponse, error)
	Search(ctx context.Context, req SearchRequest) (SearchResponse, error)
	PlanSelection(ctx context.Context, req SelectionPlanRequest) (SelectionPlan, error)
	GetDatabaseIdentity(ctx context.Context) (DatabaseIdentity, error)
	RotateReplicaIdentity(ctx context.Context) (DatabaseIdentity, error)
	StableDocumentURI(ctx context.Context, documentID string) (string, error)
	ResolveStableLink(ctx context.Context, uri string) (StableLinkResolution, error)
	Status(ctx context.Context) (StoreStatus, error)

	CreateNotebook(ctx context.Context, req CreateNotebookRequest) (Notebook, error)
	GetNotebook(ctx context.Context, id string) (Notebook, error)
	ListNotebooks(ctx context.Context) ([]Notebook, error)
	UpdateNotebook(ctx context.Context, req UpdateNotebookRequest) (Notebook, error)
	DeleteNotebook(ctx context.Context, id string) error
	ListNotebookDocuments(ctx context.Context, notebookID string, req DocumentPageRequest) (DocumentPage, error)
	MoveDocumentToNotebook(ctx context.Context, documentID, notebookID string) (Document, error)

	AddDocumentTag(ctx context.Context, documentID, tagName string) (Tag, error)
	UpsertTag(ctx context.Context, preferredID, tagName string) (Tag, string, error)
	RemoveDocumentTag(ctx context.Context, documentID, tagName string) error
	ListDocumentTags(ctx context.Context, documentID string) ([]Tag, error)
	ListTags(ctx context.Context) ([]Tag, error)

	CreateSearchNotebook(ctx context.Context, req CreateSearchNotebookRequest) (SearchNotebook, error)
	ListSearchNotebooks(ctx context.Context) ([]SearchNotebook, error)
	DeleteSearchNotebook(ctx context.Context, id string) error

	ListTrash(ctx context.Context, req DocumentPageRequest) (DocumentPage, error)
	RestoreDocument(ctx context.Context, id string) (Document, error)
	PurgeDocument(ctx context.Context, id string) error

	SetDocumentSource(ctx context.Context, req SetDocumentSourceRequest) (DocumentSource, error)
	GetDocumentSource(ctx context.Context, documentID string) (DocumentSource, error)
	GetDocumentSources(ctx context.Context, documentIDs []string) (map[string]DocumentSource, error)
	FindDocumentBySource(ctx context.Context, sourceSystem, externalID string) (string, error)
	ListThreadDocuments(ctx context.Context, threadID string) ([]DocumentSource, error)

	PendingProjectionJobs(ctx context.Context, limit int) ([]OutboxJob, error)
	CompleteProjectionJob(ctx context.Context, sequence int64, jobErr error) error
	ProjectionQueueStatus(ctx context.Context) (ProjectionQueueStatus, error)

	NotebookHasSourcedDocuments(ctx context.Context, notebookID string) (bool, error)
	FindDocumentsBySourceIDs(ctx context.Context, sourceSystem string, externalIDs []string) (map[string]string, error)

	GetImportCheckpoint(ctx context.Context, sourceSystem, sourceKey, collectionID string) (ImportCheckpoint, error)
	PutImportCheckpoint(ctx context.Context, checkpoint ImportCheckpoint) error
	GetImportItemStates(ctx context.Context, sourceSystem, sourceKey, collectionID string, itemKeys []string) (map[string]ImportItemState, error)
	PutImportItemStates(ctx context.Context, states []ImportItemState) error
	ApplyImportDocumentBatch(ctx context.Context, req ImportDocumentBatchRequest) error
	RebuildImportDocumentLinksBatch(ctx context.Context, req ImportLinkBatchRequest) error
	PutSourceBundleItem(ctx context.Context, req PutSourceBundleItemRequest) (SourceBundleItem, error)
	GetSourceBundleItem(ctx context.Context, sourceSystem, sourceKey, collectionID, itemKey string) (SourceBundleItem, error)
	OpenSourceBundleItem(ctx context.Context, sourceSystem, sourceKey, collectionID, itemKey string) (SourceBundleItem, io.ReadCloser, error)

	RecordMediaAttempt(ctx context.Context, attempt MediaAttempt) (MediaAttempt, error)
	ListMediaAttempts(ctx context.Context, documentID string, limit int) ([]MediaAttempt, error)
	AddMediaHashRule(ctx context.Context, rule MediaHashRule) error
	FindMediaHashRule(ctx context.Context, algo, hash string) (MediaHashRule, error)
}

// OutboxJob is one pending projection/indexing job. Document mutations enqueue
// jobs transactionally; the projection worker drains them after commit.
type OutboxJob struct {
	Sequence      int64
	ObjectType    string // "document"
	ObjectID      string
	Operation     string // "upsert" or "delete"
	AttemptCount  int
	NextAttemptAt time.Time
}

// ProjectionQueueStatus is bounded queue telemetry for the optional derived
// projection/search worker. Pending includes delayed retries; Due is runnable
// now; Failed counts rows that have recorded at least one failed attempt.
type ProjectionQueueStatus struct {
	Pending         int
	Due             int
	Failed          int
	OldestCreatedAt time.Time
	NextAttemptAt   time.Time
}

func NormalizeCreateRequest(req CreateDocumentRequest) CreateDocumentRequest {
	req.PreferredID = strings.TrimSpace(req.PreferredID)
	req.CollectionID = strings.TrimSpace(req.CollectionID)
	if req.CollectionID == "" {
		req.CollectionID = "default"
	}
	req.NotebookID = strings.TrimSpace(req.NotebookID)
	if req.NotebookID == "" {
		req.NotebookID = DefaultNotebookID
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = "Untitled"
	}
	if req.BodyMIMEType == "" {
		req.BodyMIMEType = "text/markdown"
	}
	if strings.TrimSpace(req.Message) == "" {
		req.Message = "create"
	}
	return req
}

func NormalizeUpdateRequest(req UpdateDocumentRequest) UpdateDocumentRequest {
	req.ID = strings.TrimSpace(req.ID)
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = "Untitled"
	}
	if req.BodyMIMEType == "" {
		req.BodyMIMEType = "text/markdown"
	}
	if strings.TrimSpace(req.Message) == "" {
		req.Message = "update"
	}
	return req
}

func NormalizeDeleteRequest(req DeleteDocumentRequest) DeleteDocumentRequest {
	req.ID = strings.TrimSpace(req.ID)
	if strings.TrimSpace(req.Message) == "" {
		req.Message = "soft delete"
	}
	return req
}

func NormalizeRestoreRevisionRequest(req RestoreRevisionRequest) RestoreRevisionRequest {
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.RevisionID = strings.TrimSpace(req.RevisionID)
	if strings.TrimSpace(req.Message) == "" {
		req.Message = "restore revision"
	}
	return req
}

func NormalizeCreateResourceRequest(req CreateResourceRequest) CreateResourceRequest {
	req.PreferredID = strings.TrimSpace(req.PreferredID)
	req.CollectionID = strings.TrimSpace(req.CollectionID)
	if req.CollectionID == "" {
		req.CollectionID = "default"
	}
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	if req.MIMEType == "" {
		req.MIMEType = "application/octet-stream"
	}
	return req
}

func NormalizeAttachResourceRequest(req AttachResourceRequest) AttachResourceRequest {
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.ResourceID = strings.TrimSpace(req.ResourceID)
	req.RelationType = strings.TrimSpace(req.RelationType)
	if req.RelationType == "" {
		req.RelationType = "attachment"
	}
	req.AnchorJSON = strings.TrimSpace(req.AnchorJSON)
	if req.AnchorJSON == "" {
		req.AnchorJSON = "{}"
	}
	return req
}

func NormalizeSearchRequest(req SearchRequest) SearchRequest {
	req.CollectionID = strings.TrimSpace(req.CollectionID)
	if req.CollectionID == "" {
		req.CollectionID = "default"
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	return req
}

func NewID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return strings.ToLower(prefix + "_" + encoded), nil
}

func ResourceURI(collectionID, resourceID string) string {
	if collectionID == "" {
		collectionID = "default"
	}
	return fmt.Sprintf("resource://%s/resources/%s", collectionID, resourceID)
}

func DocumentURI(collectionID, documentID string) string {
	if collectionID == "" {
		collectionID = "default"
	}
	return fmt.Sprintf("document://%s/documents/%s", collectionID, documentID)
}
