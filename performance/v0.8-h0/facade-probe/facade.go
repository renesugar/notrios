// Package facadeprobe is a disposable H0 experiment for a transport-neutral
// application facade. It is intentionally outside internal/ and must not be
// used as a production package.
package facadeprobe

import (
	"context"
	"errors"
	"io"

	"github.com/renesugar/notrios/internal/store"
)

var (
	ErrNotFound = errors.New("facade: not found")
	ErrConflict = errors.New("facade: conflict")
)

// Note is the transport-neutral representation exposed by this probe. It has
// no JSON tags, HTTP concerns, SQL fields, or local filesystem paths.
type Note struct {
	ID, CollectionID, NotebookID string
	Title, Body, MIMEType        string
	RevisionID                   string
}

type CreateNoteInput struct {
	ID, CollectionID, NotebookID string
	Title, Body, MIMEType        string
}

type UpdateNoteInput struct {
	ID, RevisionID        string
	Title, Body, MIMEType string
}

type SearchInput struct {
	CollectionID, Query string
	Limit               int
}

type SearchResult struct {
	Hits      []SearchHit
	Next      string
	Truncated bool
}

type SearchHit struct {
	ID, Title, Snippet string
	Score              float64
}

// Backend is deliberately unexported: the experiment keeps store ownership
// behind the facade while still allowing the real store to be injected.
type Facade struct{ backend store.Store }

type facadeContract interface {
	Create(context.Context, CreateNoteInput) (Note, error)
	Get(context.Context, string) (Note, error)
	Update(context.Context, UpdateNoteInput) (Note, error)
	Search(context.Context, SearchInput) (SearchResult, error)
	OpenResource(context.Context, string, int64) (*ResourceStream, error)
}

var _ facadeContract = (*Facade)(nil)

func New(s store.Store) *Facade { return &Facade{backend: s} }

func (f *Facade) Create(ctx context.Context, in CreateNoteInput) (Note, error) {
	if err := contextErr(ctx); err != nil {
		return Note{}, err
	}
	d, err := f.backend.CreateDocument(ctx, store.CreateDocumentRequest{
		PreferredID: in.ID, CollectionID: in.CollectionID, NotebookID: in.NotebookID,
		Title: in.Title, Body: in.Body, BodyMIMEType: in.MIMEType,
	})
	return note(d), mapError(err)
}

func (f *Facade) Get(ctx context.Context, id string) (Note, error) {
	if err := contextErr(ctx); err != nil {
		return Note{}, err
	}
	d, err := f.backend.GetDocument(ctx, id)
	return note(d), mapError(err)
}

func (f *Facade) Update(ctx context.Context, in UpdateNoteInput) (Note, error) {
	if err := contextErr(ctx); err != nil {
		return Note{}, err
	}
	d, err := f.backend.UpdateDocument(ctx, store.UpdateDocumentRequest{
		ID: in.ID, BaseRevisionID: in.RevisionID, Title: in.Title,
		Body: in.Body, BodyMIMEType: in.MIMEType,
	})
	return note(d), mapError(err)
}

func (f *Facade) Search(ctx context.Context, in SearchInput) (SearchResult, error) {
	if err := contextErr(ctx); err != nil {
		return SearchResult{}, err
	}
	r, err := f.backend.Search(ctx, store.SearchRequest{CollectionID: in.CollectionID, Query: in.Query, Limit: in.Limit})
	result := SearchResult{Next: r.NextCursor, Truncated: r.Truncated, Hits: make([]SearchHit, 0, len(r.Hits))}
	for _, h := range r.Hits {
		result.Hits = append(result.Hits, SearchHit{ID: h.ID, Title: h.Title, Snippet: h.Snippet, Score: h.Score})
	}
	return result, mapError(err)
}

// ResourceStream is a bounded, cancellation-aware reader. The facade does
// not expose the store's path or resource implementation.
type ResourceStream struct {
	ctx  context.Context
	src  io.ReadCloser
	left int64
}

func (f *Facade) OpenResource(ctx context.Context, id string, maxBytes int64) (*ResourceStream, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		return nil, errors.New("facade: maxBytes must be positive")
	}
	_, r, err := f.backend.OpenResourceContent(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}
	return &ResourceStream{ctx: ctx, src: r, left: maxBytes}, nil
}

func (r *ResourceStream) Read(p []byte) (int, error) {
	if err := contextErr(r.ctx); err != nil {
		return 0, err
	}
	if r.left == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.left {
		p = p[:int(r.left)]
	}
	n, err := r.src.Read(p)
	r.left -= int64(n)
	if r.left == 0 && err == nil {
		return n, io.EOF
	}
	return n, err
}

func (r *ResourceStream) Close() error { return r.src.Close() }

func note(d store.Document) Note {
	return Note{ID: d.ID, CollectionID: d.CollectionID, NotebookID: d.NotebookID, Title: d.Title, Body: d.Body, MIMEType: d.BodyMIMEType, RevisionID: d.CurrentRevisionID}
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, store.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, store.ErrConflict) {
		return ErrConflict
	}
	return err
}
