package archivev2

import "encoding/json"

const (
	RecordCollection       = "collection"
	RecordNotebook         = "notebook"
	RecordSearchNotebook   = "search_notebook"
	RecordTag              = "tag"
	RecordDocument         = "document"
	RecordRevision         = "revision"
	RecordDocumentTag      = "document_tag"
	RecordResource         = "resource"
	RecordDocumentResource = "document_resource"
	RecordLink             = "link"
	RecordProvenance       = "provenance"
	RecordSourceBundle     = "source_bundle"
)

type RecordEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type BlobReference struct {
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
	MediaType string `json:"media_type"`
}

// CollectionRecord is a collection as an archive carries it.
//
// It had a `capabilities` array until v0.8 H16. Nothing chose the value: the
// store returned the same five words -- documents, search, resources, links,
// graph -- for every collection, so every archive ever written recorded the
// same sentence about every collection anyone had. A field that cannot differ
// between rows is a fact about the code, and an archive is for facts about the
// library.
//
// Removing it is a format change with a reader outside this repository:
// movenotes-v3's verifier had `capabilities` in the required half of this
// payload. That verifier stopped requiring it first, in commit cc4ec25; the
// other order would have made every archive written in between unreadable
// there. Archives written before this change still carry the field and are
// still accepted, by that verifier and by the reader below.
//
// Not to be confused with the archive's own required capabilities, which gate
// whether a reader may open an archive at all. Those are real and untouched.
type CollectionRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Capabilities is read and never written. The field stays because
	// decodeStrict refuses unknown fields, so deleting it would make this
	// reader reject every archive Notrios has already produced -- a worse break
	// than the one being fixed, and in the direction that loses data rather
	// than tidiness. `omitempty` is what stops it being written again.
	Capabilities []string        `json:"capabilities,omitempty"`
	SettingsJSON json.RawMessage `json:"settings_json"`
	CreatedAt    string          `json:"created_at"`
}

type NotebookRecord struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id,omitempty"`
	Name      string `json:"name"`
	IconEmoji string `json:"icon_emoji,omitempty"`
	Builtin   bool   `json:"builtin"`
	Position  int    `json:"position"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type SearchNotebookRecord struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IconEmoji  string `json:"icon_emoji,omitempty"`
	Query      string `json:"query"`
	Builtin    bool   `json:"builtin"`
	SortAnchor string `json:"sort_anchor"`
	CreatedAt  string `json:"created_at"`
}

type TagRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type DocumentRecord struct {
	ID                string `json:"id"`
	CollectionID      string `json:"collection_id"`
	NotebookID        string `json:"notebook_id"`
	CurrentRevisionID string `json:"current_revision_id"`
	DeletedAt         string `json:"deleted_at,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type RevisionRecord struct {
	ID           string          `json:"id"`
	DocumentID   string          `json:"document_id"`
	Title        string          `json:"title"`
	Body         BlobReference   `json:"body"`
	BodyMIMEType string          `json:"body_mime_type"`
	MetadataJSON json.RawMessage `json:"metadata_json"`
	Message      string          `json:"message,omitempty"`
	CreatedAt    string          `json:"created_at"`
}

type DocumentTagRecord struct {
	DocumentID string `json:"document_id"`
	TagID      string `json:"tag_id"`
}

type ResourceRecord struct {
	ID                 string          `json:"id"`
	CollectionID       string          `json:"collection_id"`
	Blob               BlobReference   `json:"blob"`
	Filename           string          `json:"filename,omitempty"`
	MIMEType           string          `json:"mime_type"`
	MetadataJSON       json.RawMessage `json:"metadata_json"`
	UnreferencedAt     string          `json:"unreferenced_at,omitempty"`
	UnreferencedReason string          `json:"unreferenced_reason,omitempty"`
	CreatedAt          string          `json:"created_at"`
}

type DocumentResourceRecord struct {
	DocumentID   string          `json:"document_id"`
	ResourceID   string          `json:"resource_id"`
	RelationType string          `json:"relation_type"`
	Ordinal      int             `json:"ordinal"`
	AnchorJSON   json.RawMessage `json:"anchor_json"`
}

type LinkRecord struct {
	ID               string `json:"id"`
	SourceDocumentID string `json:"source_document_id"`
	TargetDocumentID string `json:"target_document_id,omitempty"`
	TargetResourceID string `json:"target_resource_id,omitempty"`
	TargetURI        string `json:"target_uri,omitempty"`
	RelationType     string `json:"relation_type"`
	SourceFormat     string `json:"source_format"`
	RawTarget        string `json:"raw_target"`
	DisplayText      string `json:"display_text,omitempty"`
	AnchorType       string `json:"anchor_type,omitempty"`
	AnchorValue      string `json:"anchor_value,omitempty"`
	Context          string `json:"context,omitempty"`
	SourceStartByte  int    `json:"source_start_byte"`
	SourceEndByte    int    `json:"source_end_byte"`
	SourceLine       int    `json:"source_line"`
	SourceColumn     int    `json:"source_column"`
	ResolutionStatus string `json:"resolution_status"`
}

type ProvenanceRecord struct {
	DocumentID   string          `json:"document_id"`
	SourceSystem string          `json:"source_system"`
	ExternalID   string          `json:"external_id,omitempty"`
	Author       string          `json:"author,omitempty"`
	AuthorID     string          `json:"author_id,omitempty"`
	ThreadID     string          `json:"thread_id,omitempty"`
	ReplyTo      string          `json:"reply_to,omitempty"`
	SourceURL    string          `json:"source_url,omitempty"`
	PublishedAt  string          `json:"published_at,omitempty"`
	PublishedTS  *int64          `json:"published_ts,omitempty"`
	MetadataJSON json.RawMessage `json:"metadata_json"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

type SourceBundleRecord struct {
	SourceSystem      string          `json:"source_system"`
	SourceKeySHA256   string          `json:"source_key_sha256"`
	CollectionID      string          `json:"collection_id"`
	ItemKeySHA256     string          `json:"item_key_sha256"`
	ItemType          string          `json:"item_type"`
	ExternalID        string          `json:"external_id,omitempty"`
	RelativePath      string          `json:"relative_path"`
	Content           BlobReference   `json:"content"`
	PropertyOrderJSON json.RawMessage `json:"property_order_json"`
	UpdatedAt         string          `json:"updated_at"`
}

func countForRecordType(recordType string) (Counts, bool) {
	var counts Counts
	switch recordType {
	case RecordCollection:
		counts.Collections = 1
	case RecordNotebook:
		counts.Notebooks = 1
	case RecordSearchNotebook:
		counts.SearchNotebooks = 1
	case RecordTag:
		counts.Tags = 1
	case RecordDocument:
		counts.Documents = 1
	case RecordRevision:
		counts.Revisions = 1
	case RecordDocumentTag:
		counts.DocumentTags = 1
	case RecordResource:
		counts.Resources = 1
	case RecordDocumentResource:
		counts.DocumentResources = 1
	case RecordLink:
		counts.Links = 1
	case RecordProvenance:
		counts.Provenance = 1
	case RecordSourceBundle:
		counts.SourceBundles = 1
	default:
		return Counts{}, false
	}
	return counts, true
}
