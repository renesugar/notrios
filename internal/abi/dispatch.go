package abi

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/renesugar/notrios/internal/application"
)

// Requests name an operation rather than getting their own C symbol. That is
// what keeps ABI major 1 at twelve functions and lets a new operation ship
// without an ABI break: a host that does not know an operation sends it anyway
// and gets not_found, instead of failing to link.
type request struct {
	Operation string          `json:"op"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// response is what a host parses. Status is echoed in the body as well as
// returned as an integer so a log of the JSON alone is diagnosable.
type response struct {
	Status  string          `json:"status"`
	Code    int32           `json:"code"`
	Message string          `json:"message,omitempty"`
	Kind    string          `json:"kind,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// dispatch runs one request and returns the status plus the encoded response.
//
// It always produces a body, including for failures: a host that gets a status
// with no explanation has to guess, and guessing across an FFI boundary is how
// bug reports become unactionable.
func (s *Session) dispatch(ctx context.Context, raw []byte) (Status, []byte) {
	var parsed request
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return StatusInvalidArgument, encode(StatusInvalidArgument, "request is not valid JSON", "", nil)
	}

	status, result, err := s.invoke(ctx, parsed)
	if err != nil {
		status = statusForError(err)
		kind := application.KindOf(err).String()
		return status, encode(status, err.Error(), kind, nil)
	}
	if status != StatusOK {
		return status, encode(status, "", "", nil)
	}

	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return StatusInternal, encode(StatusInternal, marshalErr.Error(), "", nil)
	}
	// A result that does not fit the bound is a programming error in the
	// operation, not something to truncate: silently short JSON would be
	// parsed as complete.
	if len(encoded) > MaxRequestBytes {
		return StatusLimitExceeded, encode(StatusLimitExceeded,
			"result exceeds the ABI response limit; use a stream", "", nil)
	}
	return StatusOK, encode(StatusOK, "", "", encoded)
}

func encode(status Status, message, kind string, result json.RawMessage) []byte {
	body := response{Status: status.String(), Code: int32(status), Message: message, Kind: kind, Result: result}
	encoded, err := json.Marshal(body)
	if err != nil {
		// Marshalling a struct of strings cannot realistically fail, but a
		// host must never receive a zero-length body it would parse as EOF.
		return []byte(`{"status":"internal","code":15}`)
	}
	return encoded
}

// invoke routes one operation. Operations are read-oriented for ABI major 1's
// first release; write operations exist where the facade already proves them.
func (s *Session) invoke(ctx context.Context, parsed request) (Status, any, error) {
	switch parsed.Operation {
	case "":
		return StatusInvalidArgument, nil, nil

	case "abi.info":
		return StatusOK, map[string]any{
			"abi_major":       Version,
			"capabilities":    Capabilities,
			"max_request":     MaxRequestBytes,
			"max_stream_read": MaxStreamChunkBytes,
			"events_dropped":  s.EventsDropped(),
		}, nil

	case "note.get":
		var payload struct {
			ID             string `json:"id"`
			IncludeTrashed bool   `json:"include_trashed"`
		}
		if err := decode(parsed.Payload, &payload); err != nil {
			return StatusInvalidArgument, nil, err
		}
		var (
			note application.Note
			err  error
		)
		if payload.IncludeTrashed {
			note, err = s.app.GetNoteIncludingTrashed(ctx, payload.ID)
		} else {
			note, err = s.app.GetNote(ctx, payload.ID)
		}
		if err != nil {
			return StatusInternal, nil, err
		}
		return StatusOK, noteJSON(note), nil

	case "note.create":
		var payload struct {
			CollectionID string `json:"collection_id"`
			NotebookID   string `json:"notebook_id"`
			Title        string `json:"title"`
			Body         string `json:"body"`
			MIMEType     string `json:"mime_type"`
			Message      string `json:"message"`
		}
		if err := decode(parsed.Payload, &payload); err != nil {
			return StatusInvalidArgument, nil, err
		}
		note, err := s.app.CreateNote(ctx, application.CreateNoteInput{
			CollectionID: payload.CollectionID,
			NotebookID:   payload.NotebookID,
			Title:        payload.Title,
			Body:         payload.Body,
			MIMEType:     payload.MIMEType,
			Message:      payload.Message,
		})
		if err != nil {
			return StatusInternal, nil, err
		}
		return StatusOK, noteJSON(note), nil

	case "note.update":
		var payload struct {
			ID         string `json:"id"`
			RevisionID string `json:"revision_id"`
			Title      string `json:"title"`
			Body       string `json:"body"`
			MIMEType   string `json:"mime_type"`
			Message    string `json:"message"`
		}
		if err := decode(parsed.Payload, &payload); err != nil {
			return StatusInvalidArgument, nil, err
		}
		note, err := s.app.UpdateNote(ctx, application.UpdateNoteInput{
			ID:         payload.ID,
			RevisionID: payload.RevisionID,
			Title:      payload.Title,
			Body:       payload.Body,
			MIMEType:   payload.MIMEType,
			Message:    payload.Message,
		})
		if err != nil {
			return StatusInternal, nil, err
		}
		return StatusOK, noteJSON(note), nil

	case "note.delete":
		var payload struct {
			ID         string `json:"id"`
			RevisionID string `json:"revision_id"`
			Message    string `json:"message"`
		}
		if err := decode(parsed.Payload, &payload); err != nil {
			return StatusInvalidArgument, nil, err
		}
		if err := s.app.DeleteNote(ctx, application.DeleteNoteInput{
			ID: payload.ID, RevisionID: payload.RevisionID, Message: payload.Message,
		}); err != nil {
			return StatusInternal, nil, err
		}
		return StatusOK, map[string]any{"deleted": true}, nil

	case "note.revisions":
		var payload struct {
			ID string `json:"id"`
		}
		if err := decode(parsed.Payload, &payload); err != nil {
			return StatusInvalidArgument, nil, err
		}
		revisions, err := s.app.ListRevisions(ctx, payload.ID)
		if err != nil {
			return StatusInternal, nil, err
		}
		out := make([]map[string]any, 0, len(revisions))
		for _, revision := range revisions {
			out = append(out, map[string]any{
				"id":         revision.ID,
				"note_id":    revision.NoteID,
				"title":      revision.Title,
				"message":    revision.Message,
				"mime_type":  revision.MIMEType,
				"created_at": revision.CreatedAt,
			})
		}
		return StatusOK, map[string]any{"revisions": out}, nil

	case "search":
		var payload struct {
			CollectionID string `json:"collection_id"`
			Query        string `json:"query"`
			Limit        int    `json:"limit"`
			Cursor       string `json:"cursor"`
			Sort         string `json:"sort"`
		}
		if err := decode(parsed.Payload, &payload); err != nil {
			return StatusInvalidArgument, nil, err
		}
		result, err := s.app.Search(ctx, application.SearchInput{
			CollectionID: payload.CollectionID,
			Query:        payload.Query,
			Limit:        payload.Limit,
			Cursor:       payload.Cursor,
			Sort:         payload.Sort,
		})
		if err != nil {
			return StatusInternal, nil, err
		}
		hits := make([]map[string]any, 0, len(result.Hits))
		for _, hit := range result.Hits {
			hits = append(hits, map[string]any{
				"id":      hit.ID,
				"title":   hit.Title,
				"snippet": hit.Snippet,
				"score":   hit.Score,
			})
		}
		return StatusOK, map[string]any{
			"hits":        hits,
			"next_cursor": result.NextCursor,
			"truncated":   result.Truncated,
		}, nil

	case "resource.get":
		var payload struct {
			ID string `json:"id"`
		}
		if err := decode(parsed.Payload, &payload); err != nil {
			return StatusInvalidArgument, nil, err
		}
		resource, err := s.app.GetResource(ctx, payload.ID)
		if err != nil {
			return StatusInternal, nil, err
		}
		// Metadata only. Bytes go through a stream, never a JSON result.
		return StatusOK, map[string]any{
			"id":         resource.ID,
			"filename":   resource.Filename,
			"mime_type":  resource.MIMEType,
			"size_bytes": resource.SizeBytes,
			"sha256":     resource.SHA256,
		}, nil

	default:
		return StatusNotFound, nil, nil
	}
}

// decode parses an operation payload. A missing payload is an empty object
// rather than an error, so an operation with all-optional fields can be called
// with no payload at all.
//
// A parse failure is returned as a facade invalid-input error, not as a bare
// json error. dispatch classifies every failure through statusForError, and an
// unclassified error becomes StatusInternal — which would tell a host that sent
// a malformed payload that the library broke, when the caller made the mistake.
func decode(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return &application.Error{
			Kind:    application.KindInvalidInput,
			Op:      "decode_payload",
			Message: err.Error(),
		}
	}
	return nil
}

func noteJSON(note application.Note) map[string]any {
	return map[string]any{
		"id":            note.ID,
		"uri":           note.URI,
		"collection_id": note.CollectionID,
		"notebook_id":   note.NotebookID,
		"title":         note.Title,
		"body":          note.Body,
		"mime_type":     note.MIMEType,
		"revision_id":   note.RevisionID,
		"trashed":       note.Trashed(),
	}
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
