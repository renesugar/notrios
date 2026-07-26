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
	ID           string
	URI          string
	CollectionID string
	NotebookID   string
	Title        string
	Snippet      string
	Score        float64
}

// SearchResponse is the store-level search response.
type SearchResponse struct {
	Hits       []SearchHit
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
	GetResource(ctx context.Context, id string) (Resource, error)
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
	Status(ctx context.Context) (StoreStatus, error)

	CreateNotebook(ctx context.Context, req CreateNotebookRequest) (Notebook, error)
	GetNotebook(ctx context.Context, id string) (Notebook, error)
	ListNotebooks(ctx context.Context) ([]Notebook, error)
	UpdateNotebook(ctx context.Context, req UpdateNotebookRequest) (Notebook, error)
	DeleteNotebook(ctx context.Context, id string) error
	ListNotebookDocuments(ctx context.Context, notebookID string, limit int) ([]Document, error)
	MoveDocumentToNotebook(ctx context.Context, documentID, notebookID string) (Document, error)

	AddDocumentTag(ctx context.Context, documentID, tagName string) (Tag, error)
	RemoveDocumentTag(ctx context.Context, documentID, tagName string) error
	ListDocumentTags(ctx context.Context, documentID string) ([]Tag, error)
	ListTags(ctx context.Context) ([]Tag, error)

	CreateSearchNotebook(ctx context.Context, req CreateSearchNotebookRequest) (SearchNotebook, error)
	ListSearchNotebooks(ctx context.Context) ([]SearchNotebook, error)
	DeleteSearchNotebook(ctx context.Context, id string) error

	ListTrash(ctx context.Context, limit int) ([]Document, error)
	RestoreDocument(ctx context.Context, id string) (Document, error)
	PurgeDocument(ctx context.Context, id string) error

	SetDocumentSource(ctx context.Context, req SetDocumentSourceRequest) (DocumentSource, error)
	GetDocumentSource(ctx context.Context, documentID string) (DocumentSource, error)
	FindDocumentBySource(ctx context.Context, sourceSystem, externalID string) (string, error)
	ListThreadDocuments(ctx context.Context, threadID string) ([]DocumentSource, error)

	PendingProjectionJobs(ctx context.Context, limit int) ([]OutboxJob, error)
	CompleteProjectionJob(ctx context.Context, sequence int64, jobErr error) error

	NotebookHasSourcedDocuments(ctx context.Context, notebookID string) (bool, error)

	RecordMediaAttempt(ctx context.Context, attempt MediaAttempt) (MediaAttempt, error)
	ListMediaAttempts(ctx context.Context, documentID string, limit int) ([]MediaAttempt, error)
	AddMediaHashRule(ctx context.Context, rule MediaHashRule) error
	FindMediaHashRule(ctx context.Context, algo, hash string) (MediaHashRule, error)
}

// OutboxJob is one pending projection/indexing job. Document mutations enqueue
// jobs transactionally; the projection worker drains them after commit.
type OutboxJob struct {
	Sequence   int64
	ObjectType string // "document"
	ObjectID   string
	Operation  string // "upsert" or "delete"
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
