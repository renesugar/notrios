package application

import (
	"context"
	"io"
	"strings"

	"github.com/renesugar/notrios/internal/store"
)

// Repository is the persistence seam this facade sits on.
//
// It is deliberately narrow: the canonical store interface carries 124 methods,
// and depending on all of them would make the facade as hard to implement
// against as the store itself. Listing only what the facade calls keeps the
// dependency honest and makes a test double a few lines rather than a mock of
// the whole database.
//
// This is the one exported declaration permitted to name store types. H0 chose
// to allow the store-backed implementation to wrap store.Store during the
// transition while keeping the facade's *operation* contract free of it;
// TestExportedSurfaceIsTransportAndStorageNeutral enforces that split.
// *store.SQLiteStore and store.Store both satisfy this interface already, so
// nothing has to be adapted to construct an Application.
type Repository interface {
	CreateDocument(ctx context.Context, req store.CreateDocumentRequest) (store.Document, error)
	GetDocument(ctx context.Context, id string) (store.Document, error)
	GetDocumentIncludingTrashed(ctx context.Context, id string) (store.Document, error)
	UpdateDocument(ctx context.Context, req store.UpdateDocumentRequest) (store.Document, error)
	DeleteDocument(ctx context.Context, req store.DeleteDocumentRequest) error
	ListDocumentRevisions(ctx context.Context, documentID string) ([]store.DocumentRevision, error)
	GetDocumentRevision(ctx context.Context, documentID, revisionID string) (store.DocumentRevision, error)
	RestoreDocumentRevision(ctx context.Context, req store.RestoreRevisionRequest) (store.Document, error)
	Search(ctx context.Context, req store.SearchRequest) (store.SearchResponse, error)
	GetResource(ctx context.Context, id string) (store.Resource, error)
	OpenResourceContent(ctx context.Context, id string) (store.Resource, io.ReadCloser, error)
}

// Application is the transport-neutral entry point. It is safe for concurrent
// use to exactly the degree its repository is.
type Application struct {
	repo Repository
}

// New returns an Application backed by repo. It panics on a nil repository,
// because a facade with nothing behind it can only fail later and further away.
func New(repo Repository) *Application {
	if repo == nil {
		panic("application: nil repository")
	}
	return &Application{repo: repo}
}

// CreateNote creates a note and returns it as stored.
func (a *Application) CreateNote(ctx context.Context, in CreateNoteInput) (Note, error) {
	const op = "create_note"
	if err := checkContext(ctx); err != nil {
		return Note{}, err
	}
	document, err := a.repo.CreateDocument(ctx, store.CreateDocumentRequest{
		PreferredID:  in.ID,
		CollectionID: in.CollectionID,
		NotebookID:   in.NotebookID,
		Title:        in.Title,
		Body:         in.Body,
		BodyMIMEType: in.MIMEType,
		Message:      in.Message,
	})
	if err != nil {
		return Note{}, wrap(op, err)
	}
	return note(document), nil
}

// GetNote reads a live note. A trashed note reports KindNotFound; use
// [Application.GetNoteIncludingTrashed] to read one for recovery.
func (a *Application) GetNote(ctx context.Context, id string) (Note, error) {
	const op = "get_note"
	if err := checkContext(ctx); err != nil {
		return Note{}, err
	}
	if err := requireID(op, "note id", id); err != nil {
		return Note{}, err
	}
	document, err := a.repo.GetDocument(ctx, id)
	if err != nil {
		return Note{}, wrap(op, err)
	}
	return note(document), nil
}

// GetNoteIncludingTrashed reads a note whether or not it is trashed. Check
// [Note.Trashed] before offering an edit: the facade returns the note, it does
// not decide policy.
func (a *Application) GetNoteIncludingTrashed(ctx context.Context, id string) (Note, error) {
	const op = "get_note_including_trashed"
	if err := checkContext(ctx); err != nil {
		return Note{}, err
	}
	if err := requireID(op, "note id", id); err != nil {
		return Note{}, err
	}
	document, err := a.repo.GetDocumentIncludingTrashed(ctx, id)
	if err != nil {
		return Note{}, wrap(op, err)
	}
	return note(document), nil
}

// UpdateNote replaces a note's body, returning the note at its new revision.
// A stale RevisionID reports KindConflict.
func (a *Application) UpdateNote(ctx context.Context, in UpdateNoteInput) (Note, error) {
	const op = "update_note"
	if err := checkContext(ctx); err != nil {
		return Note{}, err
	}
	if err := requireID(op, "note id", in.ID); err != nil {
		return Note{}, err
	}
	document, err := a.repo.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID:             in.ID,
		Title:          in.Title,
		Body:           in.Body,
		BodyMIMEType:   in.MIMEType,
		BaseRevisionID: in.RevisionID,
		Message:        in.Message,
	})
	if err != nil {
		return Note{}, wrap(op, err)
	}
	return note(document), nil
}

// DeleteNote moves a note to the trash. History is preserved.
func (a *Application) DeleteNote(ctx context.Context, in DeleteNoteInput) error {
	const op = "delete_note"
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := requireID(op, "note id", in.ID); err != nil {
		return err
	}
	return wrap(op, a.repo.DeleteDocument(ctx, store.DeleteDocumentRequest{
		ID:             in.ID,
		BaseRevisionID: in.RevisionID,
		Message:        in.Message,
	}))
}

// ListRevisions returns a note's revision history, newest first as the
// repository orders it.
func (a *Application) ListRevisions(ctx context.Context, noteID string) ([]Revision, error) {
	const op = "list_revisions"
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := requireID(op, "note id", noteID); err != nil {
		return nil, err
	}
	found, err := a.repo.ListDocumentRevisions(ctx, noteID)
	if err != nil {
		return nil, wrap(op, err)
	}
	revisions := make([]Revision, 0, len(found))
	for _, item := range found {
		revisions = append(revisions, revision(item))
	}
	return revisions, nil
}

// GetRevision reads one historical revision of a note.
func (a *Application) GetRevision(ctx context.Context, noteID, revisionID string) (Revision, error) {
	const op = "get_revision"
	if err := checkContext(ctx); err != nil {
		return Revision{}, err
	}
	if err := requireID(op, "note id", noteID); err != nil {
		return Revision{}, err
	}
	if err := requireID(op, "revision id", revisionID); err != nil {
		return Revision{}, err
	}
	found, err := a.repo.GetDocumentRevision(ctx, noteID, revisionID)
	if err != nil {
		return Revision{}, wrap(op, err)
	}
	return revision(found), nil
}

// RestoreRevision makes an older revision current by appending a new one.
func (a *Application) RestoreRevision(ctx context.Context, in RestoreRevisionInput) (Note, error) {
	const op = "restore_revision"
	if err := checkContext(ctx); err != nil {
		return Note{}, err
	}
	if err := requireID(op, "note id", in.NoteID); err != nil {
		return Note{}, err
	}
	if err := requireID(op, "revision id", in.RevisionID); err != nil {
		return Note{}, err
	}
	document, err := a.repo.RestoreDocumentRevision(ctx, store.RestoreRevisionRequest{
		DocumentID:     in.NoteID,
		RevisionID:     in.RevisionID,
		BaseRevisionID: in.BaseRevisionID,
		Message:        in.Message,
	})
	if err != nil {
		return Note{}, wrap(op, err)
	}
	return note(document), nil
}

// Search runs a full-text query and returns one page of hits.
func (a *Application) Search(ctx context.Context, in SearchInput) (SearchResult, error) {
	const op = "search"
	if err := checkContext(ctx); err != nil {
		return SearchResult{}, err
	}
	if in.Limit < 0 {
		return SearchResult{}, invalid(op, "limit must not be negative")
	}
	switch in.Sort {
	case SortDefault, SortRelevance, SortUpdated:
	default:
		return SearchResult{}, invalid(op, "unknown sort "+in.Sort)
	}
	response, err := a.repo.Search(ctx, store.SearchRequest{
		CollectionID: in.CollectionID,
		Query:        in.Query,
		Limit:        in.Limit,
		Cursor:       in.Cursor,
		Sort:         in.Sort,
	})
	if err != nil {
		return SearchResult{}, wrap(op, err)
	}
	result := SearchResult{
		NextCursor: response.NextCursor,
		Truncated:  response.Truncated,
		Hits:       make([]SearchHit, 0, len(response.Hits)),
	}
	for _, hit := range response.Hits {
		result.Hits = append(result.Hits, SearchHit{
			ID:           hit.ID,
			URI:          hit.URI,
			CollectionID: hit.CollectionID,
			NotebookID:   hit.NotebookID,
			Title:        hit.Title,
			Snippet:      hit.Snippet,
			Score:        hit.Score,
			UpdatedAt:    hit.UpdatedAt,
			Sources:      append([]string(nil), hit.SearchSources...),
		})
	}
	return result, nil
}

// GetResource reads an attachment's metadata without opening its bytes.
func (a *Application) GetResource(ctx context.Context, id string) (Resource, error) {
	const op = "get_resource"
	if err := checkContext(ctx); err != nil {
		return Resource{}, err
	}
	if err := requireID(op, "resource id", id); err != nil {
		return Resource{}, err
	}
	found, err := a.repo.GetResource(ctx, id)
	if err != nil {
		return Resource{}, wrap(op, err)
	}
	return resource(found), nil
}

func note(document store.Document) Note {
	return Note{
		ID:           document.ID,
		URI:          document.URI,
		CollectionID: document.CollectionID,
		NotebookID:   document.NotebookID,
		Title:        document.Title,
		Body:         document.Body,
		MIMEType:     document.BodyMIMEType,
		RevisionID:   document.CurrentRevisionID,
		CreatedAt:    document.CreatedAt,
		UpdatedAt:    document.UpdatedAt,
		DeletedAt:    document.DeletedAt,
	}
}

func revision(from store.DocumentRevision) Revision {
	return Revision{
		ID:        from.ID,
		NoteID:    from.DocumentID,
		Title:     from.Title,
		Body:      from.Body,
		MIMEType:  from.BodyMIMEType,
		Message:   from.Message,
		CreatedAt: from.CreatedAt,
	}
}

func resource(from store.Resource) Resource {
	return Resource{
		ID:           from.ID,
		URI:          from.URI,
		CollectionID: from.CollectionID,
		Filename:     from.Filename,
		MIMEType:     from.MIMEType,
		SizeBytes:    from.SizeBytes,
		SHA256:       from.SHA256,
		CreatedAt:    from.CreatedAt,
	}
}

// checkContext fails fast on an already-cancelled context so an operation does
// not reach the repository only to be abandoned.
func checkContext(ctx context.Context) error {
	if ctx == nil {
		return &Error{Kind: KindInvalidInput, Message: "nil context"}
	}
	return ctx.Err()
}

func requireID(op, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalid(op, field+" is required")
	}
	return nil
}
