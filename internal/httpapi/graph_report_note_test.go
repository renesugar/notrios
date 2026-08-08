package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

// Regeneration is a POST and never anything automatic: the scan reads the whole
// collection, so it happens when a person asks for it.
func TestGraphReportNoteIsWrittenOnRequestAndIsReadOnly(t *testing.T) {
	s := newNotebookServer(t)
	createNote(t, s, "A note", "hello\n")

	rr := doJSON(t, s, http.MethodPost, "/api/v1/graph/report/note", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("write report note: %d %s", rr.Code, rr.Body.String())
	}
	var written api.GraphReportNote
	if err := json.NewDecoder(rr.Body).Decode(&written); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if written.Document.ID != store.GraphReportNoteID || written.Document.NotebookID != store.ReportsNotebookID {
		t.Fatalf("the report has a stable home: %+v", written.Document)
	}
	if written.Report.DocumentCount != 1 {
		t.Fatalf("the report should carry its numbers so a caller need not re-read the note: %+v", written.Report)
	}
	// The API's own read of the note agrees it cannot be edited — this is the
	// flag the GUI uses to decide whether to offer an editor at all.
	if written.Document.Editable {
		t.Fatalf("a generated report a reader can edit is a report that silently stops being true: %+v", written.Document)
	}

	// And the refusal is enforced, not merely advertised. Before F5 this check
	// named one notebook by ID; it now asks the predicate, so Reports was
	// protected the day it was added.
	for _, attempt := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/documents/" + store.GraphReportNoteID, `{"title":"Mine now","body":"x"}`},
		{http.MethodPost, "/api/v1/documents/" + store.GraphReportNoteID + "/append", `{"text":"tampered"}`},
		{http.MethodDelete, "/api/v1/documents/" + store.GraphReportNoteID, ""},
	} {
		rr := doJSON(t, s, attempt.method, attempt.path, attempt.body)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("%s %s = %d, want 403: %s", attempt.method, attempt.path, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "Reports") {
			t.Fatalf("the refusal should name the notebook: %s", rr.Body.String())
		}
	}
}

// The report note is regenerated in place, so a second request must not mint a
// second note.
func TestGraphReportNoteRegeneratesInPlaceOverREST(t *testing.T) {
	s := newNotebookServer(t)
	first := doJSON(t, s, http.MethodPost, "/api/v1/graph/report/note", "")
	second := doJSON(t, s, http.MethodPost, "/api/v1/graph/report/note", "")
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("regeneration should succeed twice: %d, %d", first.Code, second.Code)
	}
	rr := doJSON(t, s, http.MethodGet, "/api/v1/notebooks/"+store.ReportsNotebookID+"/notes", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list Reports: %d %s", rr.Code, rr.Body.String())
	}
	var page api.DocumentPage
	if err := json.NewDecoder(rr.Body).Decode(&page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Documents) != 1 {
		t.Fatalf("Reports should hold exactly one report note, got %d", len(page.Documents))
	}
}

// The other half of the same F7 finding: tagging had no read-only guard, so a
// Help or Reports note could be tagged over REST and the tag outlived a reseed,
// because `note_tags` is keyed by a stable document ID.
func TestTaggingAReadOnlyNoteIsRefused(t *testing.T) {
	s := newNotebookServer(t)
	if _, _, err := s.store.WriteGraphReportNote(context.Background(), store.GraphReportRequest{}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		rr := doJSON(t, s, method, "/api/v1/documents/"+store.GraphReportNoteID+"/tags/mine", "")
		if rr.Code != http.StatusForbidden {
			t.Fatalf("%s tag = %d, want 403: %s", method, rr.Code, rr.Body.String())
		}
	}
	// An ordinary note still tags, so the guard is not simply refusing
	// everything.
	doc := createNote(t, s, "Ordinary", "x\n")
	if rr := doJSON(t, s, http.MethodPost, "/api/v1/documents/"+doc.ID+"/tags/mine", ""); rr.Code != http.StatusOK {
		t.Fatalf("tagging an ordinary note = %d: %s", rr.Code, rr.Body.String())
	}
}
