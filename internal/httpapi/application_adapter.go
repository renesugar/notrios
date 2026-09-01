package httpapi

import (
	"errors"
	"net/http"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/application"
	"github.com/renesugar/notrios/internal/store"
)

// This file is the REST adapter over internal/application. HTTP status codes,
// JSON shapes, and header conventions live here; the facade knows none of them.
//
// The migration constraint is that REST behavior does not change. Every
// mapping below is therefore the existing writeStoreError table re-expressed
// over application.Kind, and the response body still carries the message the
// store produced rather than the facade's decorated one.

// writeApplicationError renders a facade error using exactly the status codes
// and error codes writeStoreError produced for the equivalent store error.
//
// KindCanceled deliberately falls through to the fallback 500. A cancelled
// request arguably deserves a different answer, but the store path returned
// 500 for it and this slice is not the place to change that.
func writeApplicationError(w http.ResponseWriter, err error, fallbackCode string) bool {
	if err == nil {
		return false
	}
	message := applicationMessage(err)
	switch application.KindOf(err) {
	case application.KindNotFound:
		writeError(w, http.StatusNotFound, "not_found", "requested object was not found")
	case application.KindPreconditionRequired:
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "base_revision_id or If-Match is required")
	case application.KindConflict:
		writeError(w, http.StatusConflict, "conflict", "operation conflicts with the current resource state")
	case application.KindInvalidInput:
		writeError(w, http.StatusBadRequest, "validation_failed", message)
	case application.KindInvalidCursor:
		writeError(w, http.StatusBadRequest, "cursor_invalid", message)
	case application.KindNameConflict:
		writeError(w, http.StatusConflict, "name_conflict", message)
	case application.KindForbidden:
		writeError(w, http.StatusForbidden, "forbidden", message)
	default:
		writeError(w, http.StatusInternalServerError, fallbackCode, message)
	}
	return true
}

// applicationMessage recovers the message the underlying layer produced.
//
// The facade decorates errors with their operation and kind, which is right for
// a log and wrong for a response body that clients and tests already depend on.
// Error.Message holds the original text verbatim.
func applicationMessage(err error) string {
	var typed *application.Error
	if errors.As(err, &typed) && typed.Message != "" {
		return typed.Message
	}
	return err.Error()
}

// apiTimeFormat is the timestamp layout the REST surface has always emitted.
const apiTimeFormat = "2006-01-02T15:04:05Z07:00"

func apiDocumentFromNote(note application.Note) api.Document {
	deletedAt := ""
	if !note.DeletedAt.IsZero() {
		deletedAt = note.DeletedAt.Format(apiTimeFormat)
	}
	return api.Document{
		ID:                note.ID,
		URI:               note.URI,
		CollectionID:      note.CollectionID,
		NotebookID:        note.NotebookID,
		Editable:          !store.IsReadOnlyNotebook(note.NotebookID) && note.DeletedAt.IsZero(),
		Title:             note.Title,
		BodyMIMEType:      note.MIMEType,
		Body:              note.Body,
		CurrentRevisionID: note.RevisionID,
		CreatedAt:         note.CreatedAt.Format(apiTimeFormat),
		UpdatedAt:         note.UpdatedAt.Format(apiTimeFormat),
		DeletedAt:         deletedAt,
	}
}

func apiRevisionFromRevision(revision application.Revision) api.DocumentRevision {
	return api.DocumentRevision{
		ID:           revision.ID,
		DocumentID:   revision.NoteID,
		Title:        revision.Title,
		Body:         revision.Body,
		BodyMIMEType: revision.MIMEType,
		Message:      revision.Message,
		CreatedAt:    revision.CreatedAt.Format(apiTimeFormat),
	}
}

func apiResourceFromResource(resource application.Resource) api.Resource {
	return api.Resource{
		ID:           resource.ID,
		URI:          resource.URI,
		CollectionID: resource.CollectionID,
		Filename:     resource.Filename,
		MIMEType:     resource.MIMEType,
		SizeBytes:    resource.SizeBytes,
		SHA256:       resource.SHA256,
		CreatedAt:    resource.CreatedAt.Format(apiTimeFormat),
	}
}
