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

func seedTaggedNote(t *testing.T, s *Server, title string, tags ...string) string {
	t.Helper()
	ctx := context.Background()
	doc, err := s.store.CreateDocument(ctx, store.CreateDocumentRequest{Title: title, Body: title})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	for _, tag := range tags {
		if _, err := s.store.AddDocumentTag(ctx, doc.ID, tag); err != nil {
			t.Fatalf("AddDocumentTag: %v", err)
		}
	}
	return doc.ID
}

func decodeRename(t *testing.T, body string) api.TagRenameResult {
	t.Helper()
	var result api.TagRenameResult
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&result); err != nil {
		t.Fatalf("decode rename result: %v (%s)", err, body)
	}
	return result
}

func tagNamesREST(t *testing.T, s *Server) string {
	t.Helper()
	rr := doJSON(t, s, http.MethodGet, "/api/v1/tags", "")
	var payload struct {
		Tags []api.Tag `json:"tags"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&payload)
	names := make([]string, 0, len(payload.Tags))
	for _, tag := range payload.Tags {
		names = append(names, tag.Name)
	}
	return strings.Join(names, ",")
}

// The safe direction is the default: a body with no dry_run changes nothing.
func TestTagRenameRESTDefaultsToDryRun(t *testing.T) {
	s := newNotebookServer(t)
	seedTaggedNote(t, s, "one", "project", "project/alpha")

	rr := doJSON(t, s, http.MethodPost, "/api/v1/tags/rename", `{"from":"project","to":"work","include_children":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", rr.Code, rr.Body.String())
	}
	result := decodeRename(t, rr.Body.String())
	if !result.DryRun {
		t.Fatalf("an omitted dry_run must default to true: %+v", result)
	}
	if len(result.Changes) != 2 {
		t.Fatalf("expected two planned changes, got %+v", result.Changes)
	}
	if got := tagNamesREST(t, s); got != "project,project/alpha" {
		t.Fatalf("dry run changed the library: %s", got)
	}

	rr = doJSON(t, s, http.MethodPost, "/api/v1/tags/rename", `{"from":"project","to":"work","include_children":true,"dry_run":false}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", rr.Code, rr.Body.String())
	}
	if got := tagNamesREST(t, s); got != "work,work/alpha" {
		t.Fatalf("apply did not rename: %s", got)
	}
}

func TestTagRenameRESTErrors(t *testing.T) {
	s := newNotebookServer(t)
	seedTaggedNote(t, s, "one", "project")

	rr := doJSON(t, s, http.MethodPost, "/api/v1/tags/rename", `{"from":"absent","to":"work"}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing tag: expected 404, got %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodPost, "/api/v1/tags/rename", `{"from":"project","to":"work/"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad destination: expected 400, got %d %s", rr.Code, rr.Body.String())
	}
}

// A rename never rewrites a saved search, and says so rather than leaving the
// caller to discover a search notebook that now matches nothing.
func TestTagRenameRESTWarnsAboutSavedSearches(t *testing.T) {
	s := newNotebookServer(t)
	seedTaggedNote(t, s, "one", "project")
	if _, err := s.store.CreateSearchNotebook(context.Background(), store.CreateSearchNotebookRequest{Name: "Project work", Query: `tag:"project"`}); err != nil {
		t.Fatalf("CreateSearchNotebook: %v", err)
	}

	rr := doJSON(t, s, http.MethodPost, "/api/v1/tags/rename", `{"from":"project","to":"work"}`)
	result := decodeRename(t, rr.Body.String())
	joined := strings.Join(result.Warnings, " ")
	if !strings.Contains(joined, "Project work") {
		t.Fatalf("expected a warning naming the saved search, got %v", result.Warnings)
	}
}

func TestNotebookDeletionPreviewREST(t *testing.T) {
	s := newNotebookServer(t)
	ctx := context.Background()

	nb, err := s.store.CreateNotebook(ctx, store.CreateNotebookRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateNotebook: %v", err)
	}
	docID := seedTaggedNote(t, s, "note")
	if _, err := s.store.MoveDocumentToNotebook(ctx, docID, nb.ID); err != nil {
		t.Fatalf("MoveDocumentToNotebook: %v", err)
	}

	rr := doJSON(t, s, http.MethodGet, "/api/v1/notebooks/"+nb.ID+"/deletion-preview", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rr.Code, rr.Body.String())
	}
	var preview api.NotebookDeletionPreview
	_ = json.NewDecoder(rr.Body).Decode(&preview)
	if preview.Notes != 1 || !preview.Deletable || preview.RehomeNotebookID != store.DefaultNotebookID {
		t.Fatalf("preview wrong: %+v", preview)
	}

	// The Help notebook previews as protected rather than 403-ing: a client
	// asking "what would happen" deserves an answer, including "nothing".
	rr = doJSON(t, s, http.MethodGet, "/api/v1/notebooks/"+store.HelpNotebookID+"/deletion-preview", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("help preview: %d %s", rr.Code, rr.Body.String())
	}
	_ = json.NewDecoder(rr.Body).Decode(&preview)
	if preview.Deletable || preview.Reason == "" {
		t.Fatalf("Help must preview as undeletable: %+v", preview)
	}

	rr = doJSON(t, s, http.MethodGet, "/api/v1/notebooks/nb_missing/deletion-preview", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing notebook: expected 404, got %d", rr.Code)
	}
}
