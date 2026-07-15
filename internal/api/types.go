package api

// StatusResponse describes service health, runtime configuration, storage roots,
// and currently implemented capability flags.
type StatusResponse struct {
	Service      string          `json:"service"`
	Version      string          `json:"version"`
	Status       string          `json:"status"`
	Database     string          `json:"database,omitempty"` // Deprecated summary retained for early UI compatibility.
	ConfigPath   string          `json:"config_path,omitempty"`
	DatabaseInfo DatabaseStatus  `json:"database_info,omitempty"`
	Storage      StorageStatus   `json:"storage,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	Limits       map[string]int  `json:"limits,omitempty"`
}

type DatabaseStatus struct {
	Driver        string `json:"driver"`
	Path          string `json:"path,omitempty"`
	State         string `json:"state"`
	SchemaVersion int    `json:"schema_version,omitempty"`
}

type StorageStatus struct {
	DataDirectory string `json:"data_directory"`
	DatabasePath  string `json:"database_path"`
	AssetStore    string `json:"asset_store"`
	ProjectionDir string `json:"projection_dir"`
	Sist2IndexDir string `json:"sist2_index_dir"`
}

// ErrorEnvelope is the stable REST error shape.
type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type Collection struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	Description  string         `json:"description,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Settings     map[string]any `json:"settings,omitempty"`
}

type CollectionPage struct {
	Collections []Collection `json:"collections"`
	NextCursor  string       `json:"next_cursor,omitempty"`
}

type SearchRequest struct {
	Collections  []string      `json:"collections,omitempty"`
	Collection   string        `json:"collection,omitempty"` // Deprecated single-collection convenience.
	Query        string        `json:"query,omitempty"`
	Mode         string        `json:"mode,omitempty"`
	Filters      SearchFilters `json:"filters,omitempty"`
	Sort         []SortField   `json:"sort,omitempty"`
	Limit        int           `json:"limit,omitempty"`
	Cursor       string        `json:"cursor,omitempty"`
	IncludeTotal bool          `json:"include_total,omitempty"`
}

type SearchFilters struct {
	Tags           []string `json:"tags,omitempty"`
	Authors        []string `json:"authors,omitempty"`
	DateFrom       string   `json:"date_from,omitempty"`
	DateTo         string   `json:"date_to,omitempty"`
	HasMedia       *bool    `json:"has_media,omitempty"`
	MIMETypes      []string `json:"mime_types,omitempty"`
	DocumentTypes  []string `json:"document_types,omitempty"`
	IncludeDeleted bool     `json:"include_deleted,omitempty"`
}

type SortField struct {
	Field     string `json:"field"`
	Direction string `json:"direction,omitempty"`
}

type SearchHit struct {
	ID           string         `json:"id"`
	URI          string         `json:"uri"`
	Source       string         `json:"source"`
	CollectionID string         `json:"collection_id,omitempty"`
	Title        string         `json:"title,omitempty"`
	Snippet      string         `json:"snippet,omitempty"`
	Score        float64        `json:"score,omitempty"`
	Editable     bool           `json:"editable,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type SearchResponse struct {
	Hits       []SearchHit `json:"hits"`
	NextCursor string      `json:"next_cursor,omitempty"`
	Total      *int64      `json:"total,omitempty"`
}

type Document struct {
	ID                string         `json:"id"`
	URI               string         `json:"uri"`
	CollectionID      string         `json:"collection_id"`
	Title             string         `json:"title"`
	BodyMIMEType      string         `json:"body_mime_type"`
	Body              string         `json:"body,omitempty"`
	CurrentRevisionID string         `json:"current_revision_id"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	Tags              []string       `json:"tags,omitempty"`
	CreatedAt         string         `json:"created_at,omitempty"`
	UpdatedAt         string         `json:"updated_at,omitempty"`
	DeletedAt         string         `json:"deleted_at,omitempty"`
}

type DocumentMutationRequest struct {
	CollectionID   string         `json:"collection_id"`
	Title          string         `json:"title"`
	Body           string         `json:"body"`
	BodyMIMEType   string         `json:"body_mime_type,omitempty"`
	BaseRevisionID string         `json:"base_revision_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Message        string         `json:"message,omitempty"`
}

type DocumentPatchRequest struct {
	BaseRevisionID string         `json:"base_revision_id,omitempty"`
	Title          string         `json:"title,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Edits          []SurgicalEdit `json:"edits,omitempty"`
	DryRun         bool           `json:"dry_run,omitempty"`
}

type SurgicalEdit struct {
	Search  string `json:"search"`
	Replace string `json:"replace"`
	Fuzzy   bool   `json:"fuzzy,omitempty"`
}

type DocumentRevision struct {
	ID           string         `json:"id"`
	DocumentID   string         `json:"document_id"`
	Title        string         `json:"title"`
	Body         string         `json:"body,omitempty"`
	BodyMIMEType string         `json:"body_mime_type,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Message      string         `json:"message,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
}

type RevisionPage struct {
	Revisions  []DocumentRevision `json:"revisions"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

type RestoreRevisionRequest struct {
	BaseRevisionID string `json:"base_revision_id,omitempty"`
	Message        string `json:"message,omitempty"`
}

type Resource struct {
	ID           string         `json:"id"`
	URI          string         `json:"uri"`
	CollectionID string         `json:"collection_id"`
	Filename     string         `json:"filename,omitempty"`
	MIMEType     string         `json:"mime_type"`
	SizeBytes    int64          `json:"size_bytes,omitempty"`
	SHA256       string         `json:"sha256,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
}

// AttachResourceRequest creates a document-resource reference.
type AttachResourceRequest struct {
	RelationType string         `json:"relation_type,omitempty"`
	Ordinal      int            `json:"ordinal,omitempty"`
	Anchor       map[string]any `json:"anchor,omitempty"`
}

type ResourceReference struct {
	DocumentID   string         `json:"document_id"`
	ResourceID   string         `json:"resource_id"`
	Resource     *Resource      `json:"resource,omitempty"`
	RelationType string         `json:"relation_type"`
	Ordinal      int            `json:"ordinal,omitempty"`
	Anchor       map[string]any `json:"anchor,omitempty"`
}

type ResourceReferencePage struct {
	Resources  []ResourceReference `json:"resources"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

type DocumentLink struct {
	ID               string         `json:"id"`
	SourceDocumentID string         `json:"source_document_id"`
	TargetDocumentID string         `json:"target_document_id,omitempty"`
	TargetResourceID string         `json:"target_resource_id,omitempty"`
	TargetURI        string         `json:"target_uri,omitempty"`
	RelationType     string         `json:"relation_type"`
	SourceFormat     string         `json:"source_format,omitempty"`
	RawTarget        string         `json:"raw_target,omitempty"`
	DisplayText      string         `json:"display_text,omitempty"`
	AnchorType       string         `json:"anchor_type,omitempty"`
	AnchorValue      string         `json:"anchor_value,omitempty"`
	Context          string         `json:"context,omitempty"`
	ResolutionStatus string         `json:"resolution_status"`
	SourcePosition   SourcePosition `json:"source_position,omitempty"`
}

type SourcePosition struct {
	StartByte int `json:"start_byte,omitempty"`
	EndByte   int `json:"end_byte,omitempty"`
	Line      int `json:"line,omitempty"`
	Column    int `json:"column,omitempty"`
}

type DocumentLinkPage struct {
	Outgoing   []DocumentLink `json:"outgoing,omitempty"`
	Incoming   []DocumentLink `json:"incoming,omitempty"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type DocumentOutline struct {
	DocumentID string            `json:"document_id"`
	Headings   []DocumentHeading `json:"headings"`
}

type DocumentHeading struct {
	Level  int    `json:"level"`
	Title  string `json:"title"`
	Anchor string `json:"anchor,omitempty"`
	Line   int    `json:"line,omitempty"`
}

type RemoteMediaRequest struct {
	Policy string   `json:"policy,omitempty"`
	Types  []string `json:"types,omitempty"`
	URLs   []string `json:"urls,omitempty"`
	DryRun bool     `json:"dry_run,omitempty"`
}

type RemoteMediaResult struct {
	RevisionID string           `json:"revision_id,omitempty"`
	Localized  []map[string]any `json:"localized"`
	Blocked    []map[string]any `json:"blocked"`
	Review     []map[string]any `json:"review"`
	Failed     []map[string]any `json:"failed"`
}

type GraphRequest struct {
	Roots            []string `json:"roots"`
	Direction        string   `json:"direction,omitempty"`
	Depth            int      `json:"depth,omitempty"`
	IncludeResources bool     `json:"include_resources,omitempty"`
	MaxNodes         int      `json:"max_nodes,omitempty"`
	MaxEdges         int      `json:"max_edges,omitempty"`
}

type GraphResponse struct {
	Nodes     []map[string]any `json:"nodes"`
	Edges     []map[string]any `json:"edges"`
	Truncated bool             `json:"truncated,omitempty"`
}

type PublishPlanRequest struct {
	Profile string         `json:"profile,omitempty"`
	Target  string         `json:"target,omitempty"`
	Include map[string]any `json:"include,omitempty"`
	Exclude map[string]any `json:"exclude,omitempty"`
}

type PublishPlanResponse struct {
	NotesIncluded     int      `json:"notes_included"`
	ResourcesIncluded int      `json:"resources_included"`
	NotesExcluded     int      `json:"notes_excluded"`
	Warnings          []string `json:"warnings"`
}

type JobStatus struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress,omitempty"`
	Message  string  `json:"message,omitempty"`
}
