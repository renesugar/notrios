package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/store"
)

// fakeRepository returns exactly what a test asks it to. The narrow Repository
// seam is what makes this possible in a few lines; doubling the 124-method
// store interface would not be worth anyone's time.
type fakeRepository struct {
	err      error
	document store.Document
	revision store.DocumentRevision
	search   store.SearchResponse
	resource store.Resource
	content  io.ReadCloser
}

func (f *fakeRepository) CreateDocument(context.Context, store.CreateDocumentRequest) (store.Document, error) {
	return f.document, f.err
}

func (f *fakeRepository) GetDocument(context.Context, string) (store.Document, error) {
	return f.document, f.err
}

func (f *fakeRepository) GetDocumentIncludingTrashed(context.Context, string) (store.Document, error) {
	return f.document, f.err
}

func (f *fakeRepository) UpdateDocument(context.Context, store.UpdateDocumentRequest) (store.Document, error) {
	return f.document, f.err
}

func (f *fakeRepository) DeleteDocument(context.Context, store.DeleteDocumentRequest) error {
	return f.err
}

func (f *fakeRepository) ListDocumentRevisions(context.Context, string) ([]store.DocumentRevision, error) {
	return []store.DocumentRevision{f.revision}, f.err
}

func (f *fakeRepository) GetDocumentRevision(context.Context, string, string) (store.DocumentRevision, error) {
	return f.revision, f.err
}

func (f *fakeRepository) RestoreDocumentRevision(context.Context, store.RestoreRevisionRequest) (store.Document, error) {
	return f.document, f.err
}

func (f *fakeRepository) Search(context.Context, store.SearchRequest) (store.SearchResponse, error) {
	return f.search, f.err
}

func (f *fakeRepository) GetResource(context.Context, string) (store.Resource, error) {
	return f.resource, f.err
}

func (f *fakeRepository) OpenResourceContent(context.Context, string) (store.Resource, io.ReadCloser, error) {
	return f.resource, f.content, f.err
}

var _ Repository = (*fakeRepository)(nil)

// TestStoreErrorsMapToKinds covers the whole translation table, including the
// errors a real in-memory store will not produce on demand. The mapping is the
// facade's contract with every adapter, so an unmapped error silently becoming
// KindInternal is a regression worth catching here.
func TestStoreErrorsMapToKinds(t *testing.T) {
	cases := []struct {
		name     string
		from     error
		wantKind Kind
		sentinel error
	}{
		{"not found", store.ErrNotFound, KindNotFound, ErrNotFound},
		{"conflict", store.ErrConflict, KindConflict, ErrConflict},
		{"precondition", store.ErrPreconditionRequired, KindPreconditionRequired, ErrPreconditionRequired},
		{"invalid input", store.ErrInvalidInput, KindInvalidInput, ErrInvalidInput},
		{"invalid cursor", store.ErrInvalidCursor, KindInvalidCursor, ErrInvalidCursor},
		{"name conflict", store.ErrNameConflict, KindNameConflict, ErrNameConflict},
		{"protected", store.ErrProtected, KindForbidden, ErrForbidden},
		{"resource unavailable", store.ErrResourceUnavailable, KindUnavailable, ErrUnavailable},
		{"restore in progress", store.ErrPhysicalRestoreInProgress, KindUnavailable, ErrUnavailable},
		{"unclassified", errors.New("disk caught fire"), KindInternal, ErrInternal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			app := New(&fakeRepository{err: testCase.from})
			_, err := app.GetNote(context.Background(), "doc_x")
			if KindOf(err) != testCase.wantKind {
				t.Errorf("KindOf = %v, want %v", KindOf(err), testCase.wantKind)
			}
			if !errors.Is(err, testCase.sentinel) {
				t.Errorf("errors.Is(%v, %v) = false", err, testCase.sentinel)
			}
			// The originating error stays reachable for callers that need it.
			if !errors.Is(err, testCase.from) {
				t.Errorf("the wrapped store error is no longer reachable from %v", err)
			}
		})
	}
}

// A wrapped store error must be classified the same as a bare one; the store
// wraps its sentinels with context in several places.
func TestWrappedStoreErrorsKeepTheirKind(t *testing.T) {
	wrapped := fmt.Errorf("reading note %s: %w", "doc_x", store.ErrNotFound)
	app := New(&fakeRepository{err: wrapped})
	_, err := app.GetNote(context.Background(), "doc_x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want not found", err)
	}
}

func TestSentinelsDoNotMatchEachOther(t *testing.T) {
	all := []*Error{
		ErrNotFound, ErrConflict, ErrPreconditionRequired, ErrInvalidInput,
		ErrInvalidCursor, ErrNameConflict, ErrForbidden, ErrUnavailable, ErrInternal,
	}
	for _, left := range all {
		for _, right := range all {
			if left == right {
				continue
			}
			if errors.Is(left, right) {
				t.Errorf("%v matched %v; sentinels must be distinct", left, right)
			}
		}
	}
}

// A sentinel carries no Op or Message, so it matches any error of its kind. A
// concrete error carries both, so it must not be usable as a match target that
// swallows unrelated failures.
func TestConcreteErrorIsNotAMatchTarget(t *testing.T) {
	concrete := &Error{Kind: KindNotFound, Op: "get_note", Message: "gone"}
	if !errors.Is(concrete, ErrNotFound) {
		t.Error("a concrete not-found error does not match the sentinel")
	}
	other := &Error{Kind: KindNotFound, Op: "search", Message: "different"}
	if errors.Is(other, concrete) {
		t.Error("one concrete error matched another; only sentinels should match by kind")
	}
}

func TestErrorMessageIncludesOperationAndKind(t *testing.T) {
	cases := map[string]*Error{
		"get_note: not_found: gone": {Kind: KindNotFound, Op: "get_note", Message: "gone"},
		"get_note: not_found":       {Kind: KindNotFound, Op: "get_note"},
		"not_found: gone":           {Kind: KindNotFound, Message: "gone"},
		"not_found":                 {Kind: KindNotFound},
	}
	for want, err := range cases {
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	}
}

func TestKindStringIsStableAndTotal(t *testing.T) {
	want := map[Kind]string{
		KindInternal:             "internal",
		KindNotFound:             "not_found",
		KindConflict:             "conflict",
		KindPreconditionRequired: "precondition_required",
		KindInvalidInput:         "invalid_input",
		KindInvalidCursor:        "invalid_cursor",
		KindNameConflict:         "name_conflict",
		KindForbidden:            "forbidden",
		KindUnavailable:          "unavailable",
		KindCanceled:             "canceled",
	}
	for kind, name := range want {
		if kind.String() != name {
			t.Errorf("Kind(%d).String() = %q, want %q", kind, kind.String(), name)
		}
	}
	// An out-of-range kind must still produce a usable name rather than a
	// format-verb artifact crossing the ABI.
	if Kind(9999).String() != "internal" {
		t.Errorf("unknown kind rendered as %q", Kind(9999).String())
	}
	// KindInternal is the zero value on purpose: a zero Error is never a
	// client mistake.
	if KindInternal != 0 {
		t.Error("KindInternal is no longer the zero value")
	}
}

func TestKindOfNilAndPlainErrors(t *testing.T) {
	if KindOf(nil) != KindInternal {
		t.Errorf("KindOf(nil) = %v", KindOf(nil))
	}
	if KindOf(errors.New("plain")) != KindInternal {
		t.Error("a plain error is not internal")
	}
	if KindOf(context.DeadlineExceeded) != KindCanceled {
		t.Error("a deadline is not classified as cancellation")
	}
}

// This is the non-vacuous version of the aliasing check: the real store never
// populates SearchSources, so only a double can prove the facade copies it.
func TestSearchHitSourcesAreDefensivelyCopied(t *testing.T) {
	repo := &fakeRepository{search: store.SearchResponse{
		Hits: []store.SearchHit{{
			ID:            "doc_x",
			Title:         "Hit",
			UpdatedAt:     time.Unix(1, 0),
			SearchSources: []string{"native", "recoll"},
		}},
	}}
	app := New(repo)

	result, err := app.Search(context.Background(), SearchInput{Query: "x", Limit: 1})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Hits) != 1 || len(result.Hits[0].Sources) != 2 {
		t.Fatalf("unexpected hits: %+v", result.Hits)
	}
	result.Hits[0].Sources[0] = "mutated-by-caller"

	if repo.search.Hits[0].SearchSources[0] != "native" {
		t.Fatal("mutating a facade result reached back into the repository's slice")
	}
	again, err := app.Search(context.Background(), SearchInput{Query: "x", Limit: 1})
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if again.Hits[0].Sources[0] != "native" {
		t.Error("a caller mutating a result changed what the next call returns")
	}
}

// OpenResourceContent can legally report success with no reader on a replica
// that has metadata but not bytes. Returning a nil stream to a caller that is
// about to Read would be a panic in the adapter.
func TestOpenResourceRefusesANilReader(t *testing.T) {
	app := New(&fakeRepository{resource: store.Resource{ID: "res_x"}, content: nil})
	_, stream, err := app.OpenResource(context.Background(), "res_x", 16)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v, want unavailable", err)
	}
	if stream != nil {
		t.Error("a stream was returned alongside an error")
	}
}

type errorReader struct{ err error }

func (e errorReader) Read([]byte) (int, error) { return 0, e.err }
func (e errorReader) Close() error             { return nil }

// A mid-stream repository failure must surface unchanged. Translating it into
// io.EOF would silently truncate an attachment.
func TestResourceStreamPropagatesReadErrors(t *testing.T) {
	boom := errors.New("blob store went away")
	app := New(&fakeRepository{resource: store.Resource{ID: "res_x"}, content: errorReader{err: boom}})
	_, stream, err := app.OpenResource(context.Background(), "res_x", 32)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer stream.Close()
	if _, err := stream.Read(make([]byte, 8)); !errors.Is(err, boom) {
		t.Fatalf("read error = %v, want the underlying failure", err)
	}
}

type countingCloser struct {
	io.Reader
	closes int
}

func (c *countingCloser) Close() error {
	c.closes++
	return nil
}

func TestResourceStreamClosesUnderlyingReaderExactlyOnce(t *testing.T) {
	closer := &countingCloser{Reader: strings.NewReader("payload")}
	app := New(&fakeRepository{resource: store.Resource{ID: "res_x"}, content: closer})
	_, stream, err := app.OpenResource(context.Background(), "res_x", 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for range 3 {
		if err := stream.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
	if closer.closes != 1 {
		t.Errorf("underlying reader closed %d times, want exactly 1", closer.closes)
	}
}

// TestEveryOperationMapsRepositoryErrors proves the translation is applied by
// each operation, not just by the one that happens to be tested elsewhere. An
// operation that forgot to call wrap would leak a bare store error to adapters,
// which would then fall through to a 500 instead of the right status.
func TestEveryOperationMapsRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	operations := map[string]func(*Application) error{
		"CreateNote": func(a *Application) error {
			_, err := a.CreateNote(ctx, CreateNoteInput{CollectionID: "default"})
			return err
		},
		"GetNote": func(a *Application) error {
			_, err := a.GetNote(ctx, "doc_x")
			return err
		},
		"GetNoteIncludingTrashed": func(a *Application) error {
			_, err := a.GetNoteIncludingTrashed(ctx, "doc_x")
			return err
		},
		"UpdateNote": func(a *Application) error {
			_, err := a.UpdateNote(ctx, UpdateNoteInput{ID: "doc_x"})
			return err
		},
		"DeleteNote": func(a *Application) error {
			return a.DeleteNote(ctx, DeleteNoteInput{ID: "doc_x"})
		},
		"ListRevisions": func(a *Application) error {
			_, err := a.ListRevisions(ctx, "doc_x")
			return err
		},
		"GetRevision": func(a *Application) error {
			_, err := a.GetRevision(ctx, "doc_x", "rev_x")
			return err
		},
		"RestoreRevision": func(a *Application) error {
			_, err := a.RestoreRevision(ctx, RestoreRevisionInput{NoteID: "doc_x", RevisionID: "rev_x"})
			return err
		},
		"Search": func(a *Application) error {
			_, err := a.Search(ctx, SearchInput{Query: "x"})
			return err
		},
		"GetResource": func(a *Application) error {
			_, err := a.GetResource(ctx, "res_x")
			return err
		},
		"OpenResource": func(a *Application) error {
			_, _, err := a.OpenResource(ctx, "res_x", 16)
			return err
		},
	}
	for name, call := range operations {
		t.Run(name, func(t *testing.T) {
			app := New(&fakeRepository{err: store.ErrNotFound})
			err := call(app)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("got %v, want a mapped not-found error", err)
			}
			var typed *Error
			if !errors.As(err, &typed) {
				t.Fatalf("error is not an *Error: %v", err)
			}
			if typed.Op == "" {
				t.Error("mapped error does not name the operation that failed")
			}
		})
	}
}

// TestEveryOperationRefusesACancelledContext keeps cancellation comparable with
// the standard library sentinel across the whole surface.
func TestEveryOperationRefusesACancelledContext(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	app := New(&fakeRepository{})

	checks := map[string]error{}
	_, err := app.CreateNote(cancelled, CreateNoteInput{})
	checks["CreateNote"] = err
	_, err = app.GetNote(cancelled, "doc_x")
	checks["GetNote"] = err
	_, err = app.GetNoteIncludingTrashed(cancelled, "doc_x")
	checks["GetNoteIncludingTrashed"] = err
	_, err = app.UpdateNote(cancelled, UpdateNoteInput{ID: "doc_x"})
	checks["UpdateNote"] = err
	checks["DeleteNote"] = app.DeleteNote(cancelled, DeleteNoteInput{ID: "doc_x"})
	_, err = app.ListRevisions(cancelled, "doc_x")
	checks["ListRevisions"] = err
	_, err = app.GetRevision(cancelled, "doc_x", "rev_x")
	checks["GetRevision"] = err
	_, err = app.RestoreRevision(cancelled, RestoreRevisionInput{NoteID: "doc_x", RevisionID: "rev_x"})
	checks["RestoreRevision"] = err
	_, err = app.Search(cancelled, SearchInput{})
	checks["Search"] = err
	_, err = app.GetResource(cancelled, "res_x")
	checks["GetResource"] = err
	_, _, err = app.OpenResource(cancelled, "res_x", 16)
	checks["OpenResource"] = err

	for name, err := range checks {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s: got %v, want context.Canceled", name, err)
		}
	}
}

// wrap must be idempotent: an error that is already a facade error keeps its
// original operation rather than being re-labelled by an outer caller.
func TestWrapDoesNotRelabelAFacadeError(t *testing.T) {
	inner := &Error{Kind: KindConflict, Op: "inner_op", Message: "already typed"}
	app := New(&fakeRepository{err: inner})
	_, err := app.GetNote(context.Background(), "doc_x")

	var typed *Error
	if !errors.As(err, &typed) {
		t.Fatalf("error is not an *Error: %v", err)
	}
	if typed.Op != "inner_op" {
		t.Errorf("Op = %q, want the original inner_op", typed.Op)
	}
	if typed.Kind != KindConflict {
		t.Errorf("Kind = %v, want the original conflict", typed.Kind)
	}
}
