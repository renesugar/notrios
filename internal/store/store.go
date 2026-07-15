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
	ID           string
	URI          string
	CollectionID string
	Filename     string
	MIMEType     string
	SizeBytes    int64
	SHA256       string
	CreatedAt    time.Time
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
