package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/renesugar/notrios/internal/application"
	"github.com/renesugar/notrios/internal/store"
)

// The migration to internal/application is only safe if the new error path
// answers exactly as the old one did. This drives both writers with the same
// underlying store error and requires byte-identical responses.
//
// The store errors are passed through application.New so the facade does the
// classifying, which is the arrangement the handlers actually use.
func TestApplicationErrorMatchesStoreErrorResponse(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"not found", store.ErrNotFound},
		{"precondition required", store.ErrPreconditionRequired},
		{"conflict", store.ErrConflict},
		{"invalid input", fmt.Errorf("%w: title is required", store.ErrInvalidInput)},
		{"invalid cursor", fmt.Errorf("%w: cursor is not decodable", store.ErrInvalidCursor)},
		{"name conflict", fmt.Errorf("%w: notebook exists", store.ErrNameConflict)},
		{"protected", fmt.Errorf("%w: builtin", store.ErrProtected)},
		{"unclassified", errors.New("something broke")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			const fallback = "document_read_failed"

			storeRecorder := httptest.NewRecorder()
			if !writeStoreError(storeRecorder, testCase.err, fallback) {
				t.Fatal("writeStoreError did not handle the error")
			}

			// Route the same error through the facade, as a handler would.
			app := application.New(&errorRepository{err: testCase.err})
			_, facadeErr := app.GetNote(context.Background(), "doc_x")
			applicationRecorder := httptest.NewRecorder()
			if !writeApplicationError(applicationRecorder, facadeErr, fallback) {
				t.Fatal("writeApplicationError did not handle the error")
			}

			if storeRecorder.Code != applicationRecorder.Code {
				t.Errorf("status: store=%d application=%d", storeRecorder.Code, applicationRecorder.Code)
			}
			if storeRecorder.Body.String() != applicationRecorder.Body.String() {
				t.Errorf("body differs:\n store       = %s\n application = %s",
					storeRecorder.Body.String(), applicationRecorder.Body.String())
			}
		})
	}
}

func TestWriteApplicationErrorIgnoresNil(t *testing.T) {
	recorder := httptest.NewRecorder()
	if writeApplicationError(recorder, nil, "unused") {
		t.Error("a nil error was reported as handled")
	}
	if recorder.Body.Len() != 0 {
		t.Error("a nil error produced a response body")
	}
}

// A cancelled request answered 500 on the store path. The facade passes
// context errors through untouched, so this pins the status that behavior
// produces rather than letting it drift silently.
func TestCancelledRequestStillAnswersInternalError(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	app := application.New(&errorRepository{})
	_, err := app.GetNote(cancelled, "doc_x")

	recorder := httptest.NewRecorder()
	if !writeApplicationError(recorder, err, "document_read_failed") {
		t.Fatal("cancellation was not handled")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}

// The response body must carry the message the store produced, not the
// facade's decorated "op: kind: message" rendering.
func TestApplicationMessageUsesTheUnderlyingText(t *testing.T) {
	underlying := fmt.Errorf("%w: title is required", store.ErrInvalidInput)
	app := application.New(&errorRepository{err: underlying})
	_, err := app.GetNote(context.Background(), "doc_x")

	if got := applicationMessage(err); got != underlying.Error() {
		t.Errorf("applicationMessage = %q, want %q", got, underlying.Error())
	}

	recorder := httptest.NewRecorder()
	writeApplicationError(recorder, err, "document_read_failed")
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Error.Message != underlying.Error() {
		t.Errorf("message = %q, want %q", envelope.Error.Message, underlying.Error())
	}
	if envelope.Error.Code != "validation_failed" {
		t.Errorf("code = %q, want validation_failed", envelope.Error.Code)
	}
}

// applicationMessage must not panic or return an empty string for an error
// that never passed through the facade.
func TestApplicationMessageFallsBackToTheErrorText(t *testing.T) {
	plain := errors.New("raw failure")
	if got := applicationMessage(plain); got != "raw failure" {
		t.Errorf("applicationMessage = %q", got)
	}
}

// The converters must agree with the store-based ones they replaced; a note
// and the document it came from have to render identically.
func TestNoteAndDocumentRenderIdentically(t *testing.T) {
	ctx := context.Background()
	backing, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = backing.Close() })
	if err := backing.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	app := application.New(backing)

	created, err := backing.CreateDocument(ctx, store.CreateDocumentRequest{
		CollectionID: "default",
		Title:        "Parity",
		Body:         "body text",
		BodyMIMEType: "text/markdown",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	note, err := app.GetNote(ctx, created.ID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}

	fromStore, err := json.Marshal(toAPIDocument(created))
	if err != nil {
		t.Fatalf("marshal store document: %v", err)
	}
	fromNote, err := json.Marshal(apiDocumentFromNote(note))
	if err != nil {
		t.Fatalf("marshal note: %v", err)
	}
	if string(fromStore) != string(fromNote) {
		t.Errorf("document rendering differs:\n store = %s\n note  = %s", fromStore, fromNote)
	}

	revisions, err := backing.ListDocumentRevisions(ctx, created.ID)
	if err != nil || len(revisions) == 0 {
		t.Fatalf("list revisions: %v (%d)", err, len(revisions))
	}
	appRevisions, err := app.ListRevisions(ctx, created.ID)
	if err != nil {
		t.Fatalf("facade revisions: %v", err)
	}
	fromStoreRevision, err := json.Marshal(toAPIRevision(revisions[0]))
	if err != nil {
		t.Fatalf("marshal store revision: %v", err)
	}
	fromAppRevision, err := json.Marshal(apiRevisionFromRevision(appRevisions[0]))
	if err != nil {
		t.Fatalf("marshal facade revision: %v", err)
	}
	if string(fromStoreRevision) != string(fromAppRevision) {
		t.Errorf("revision rendering differs:\n store = %s\n app   = %s", fromStoreRevision, fromAppRevision)
	}
}

// errorRepository returns one configured error from every operation.
type errorRepository struct {
	err error
}

func (e *errorRepository) CreateDocument(context.Context, store.CreateDocumentRequest) (store.Document, error) {
	return store.Document{}, e.err
}
func (e *errorRepository) GetDocument(context.Context, string) (store.Document, error) {
	return store.Document{}, e.err
}
func (e *errorRepository) GetDocumentIncludingTrashed(context.Context, string) (store.Document, error) {
	return store.Document{}, e.err
}
func (e *errorRepository) UpdateDocument(context.Context, store.UpdateDocumentRequest) (store.Document, error) {
	return store.Document{}, e.err
}
func (e *errorRepository) DeleteDocument(context.Context, store.DeleteDocumentRequest) error {
	return e.err
}
func (e *errorRepository) ListDocumentRevisions(context.Context, string) ([]store.DocumentRevision, error) {
	return nil, e.err
}
func (e *errorRepository) GetDocumentRevision(context.Context, string, string) (store.DocumentRevision, error) {
	return store.DocumentRevision{}, e.err
}
func (e *errorRepository) RestoreDocumentRevision(context.Context, store.RestoreRevisionRequest) (store.Document, error) {
	return store.Document{}, e.err
}
func (e *errorRepository) Search(context.Context, store.SearchRequest) (store.SearchResponse, error) {
	return store.SearchResponse{}, e.err
}
func (e *errorRepository) GetResource(context.Context, string) (store.Resource, error) {
	return store.Resource{}, e.err
}
func (e *errorRepository) OpenResourceContent(context.Context, string) (store.Resource, io.ReadCloser, error) {
	return store.Resource{}, nil, e.err
}

var _ application.Repository = (*errorRepository)(nil)
