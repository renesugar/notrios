package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
)

func createNote(t *testing.T, s *Server, title, body string) api.Document {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
	rr := doJSON(t, s, http.MethodPost, "/api/v1/documents", string(payload))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create note: %d %s", rr.Code, rr.Body.String())
	}
	var doc api.Document
	if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return doc
}

func TestAppendPrependAndLineOps(t *testing.T) {
	s := newNotebookServer(t)
	doc := createNote(t, s, "ops note", "line one\nline two\nline three")

	rr := doJSON(t, s, http.MethodPost, "/api/v1/documents/"+doc.ID+"/append", `{"text":"appended tail"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("append: %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodPost, "/api/v1/documents/"+doc.ID+"/prepend", `{"text":"prepended head"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("prepend: %d %s", rr.Code, rr.Body.String())
	}
	var updated api.Document
	_ = json.NewDecoder(rr.Body).Decode(&updated)
	if !strings.HasPrefix(updated.Body, "prepended head\n") || !strings.HasSuffix(updated.Body, "appended tail") {
		t.Fatalf("append/prepend body wrong: %q", updated.Body)
	}

	// Line-range read (1-indexed, inclusive).
	rr = doJSON(t, s, http.MethodGet, "/api/v1/documents/"+doc.ID+"/lines?start=2&end=3", "")
	var lines api.DocumentLines
	_ = json.NewDecoder(rr.Body).Decode(&lines)
	if lines.StartLine != 2 || lines.EndLine != 3 || len(lines.Lines) != 2 || lines.Lines[0] != "line one" {
		t.Fatalf("line range wrong: %+v", lines)
	}

	// In-note search with context and line numbers.
	rr = doJSON(t, s, http.MethodGet, "/api/v1/documents/"+doc.ID+"/search-in?pattern=LINE%20TWO", "")
	var found api.NoteSearchResponse
	_ = json.NewDecoder(rr.Body).Decode(&found)
	if len(found.Matches) != 1 || found.Matches[0].Line != 3 || found.Matches[0].Text != "line two" {
		t.Fatalf("in-note search wrong: %+v", found)
	}
	if len(found.Matches[0].Context) != 3 {
		t.Fatalf("context missing: %+v", found.Matches[0])
	}

	// Outline endpoint returns real headings now.
	head := createNote(t, s, "outlined", "# Top\n\ntext\n\n## Sub Heading\n\nmore")
	rr = doJSON(t, s, http.MethodGet, "/api/v1/documents/"+head.ID+"/outline", "")
	if !strings.Contains(rr.Body.String(), "Sub Heading") {
		t.Fatalf("outline missing headings: %s", rr.Body.String())
	}
}

func TestPatchEditAmbiguityAndReplaceAll(t *testing.T) {
	s := newNotebookServer(t)
	doc := createNote(t, s, "edit note", "alpha beta alpha")

	// Ambiguous match without replace_all fails.
	rr := doJSON(t, s, http.MethodPatch, "/api/v1/documents/"+doc.ID,
		`{"base_revision_id":"`+doc.CurrentRevisionID+`","edits":[{"search":"alpha","replace":"gamma"}]}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "2 locations") {
		t.Fatalf("ambiguity must fail: %d %s", rr.Code, rr.Body.String())
	}

	// replace_all succeeds.
	rr = doJSON(t, s, http.MethodPatch, "/api/v1/documents/"+doc.ID,
		`{"base_revision_id":"`+doc.CurrentRevisionID+`","edits":[{"search":"alpha","replace":"gamma","replace_all":true}]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("replace_all: %d %s", rr.Code, rr.Body.String())
	}
	var updated api.Document
	_ = json.NewDecoder(rr.Body).Decode(&updated)
	if updated.Body != "gamma beta gamma" {
		t.Fatalf("replace_all body wrong: %q", updated.Body)
	}
}

func TestMCPWriteToolsProfileGating(t *testing.T) {
	// Default read-only profile: write tools hidden and rejected.
	s := newNotebookServer(t)
	rr := doJSON(t, s, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if strings.Contains(rr.Body.String(), "create_note") {
		t.Fatalf("write tools must be hidden in read-only profile: %s", rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_note","arguments":{"title":"nope"}}}`)
	if !strings.Contains(rr.Body.String(), "read-only") {
		t.Fatalf("write call must be rejected in read-only profile: %s", rr.Body.String())
	}

	// Editor profile enables the write tools.
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(t.Context()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	cfg := config.Default()
	cfg.MCP.DefaultProfile = "editor"
	se := NewServerWithOptions(ServerOptions{Store: st, Config: cfg})

	rr = doJSON(t, se, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	if !strings.Contains(rr.Body.String(), "edit_note") || !strings.Contains(rr.Body.String(), "delete_note") {
		t.Fatalf("editor profile must list write tools: %s", rr.Body.String())
	}

	rr = doJSON(t, se, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"create_note","arguments":{"title":"mcp made","body":"alpha alpha"}}}`)
	body := rr.Body.String()
	if !strings.Contains(body, "mcp made") {
		t.Fatalf("create_note failed: %s", body)
	}
	idStart := strings.Index(body, "doc_")
	docID := body[idStart : idStart+strings.IndexAny(body[idStart:], "\\\" ")]

	// edit_note: ambiguous fails, replace_all works.
	rr = doJSON(t, se, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"edit_note","arguments":{"document_id":"`+docID+`","search":"alpha","replace":"beta"}}}`)
	if !strings.Contains(rr.Body.String(), "2 locations") {
		t.Fatalf("edit_note ambiguity not enforced: %s", rr.Body.String())
	}
	rr = doJSON(t, se, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"edit_note","arguments":{"document_id":"`+docID+`","search":"alpha","replace":"beta","replace_all":true}}}`)
	if !strings.Contains(rr.Body.String(), "beta beta") {
		t.Fatalf("edit_note replace_all failed: %s", rr.Body.String())
	}

	// append works and delete_note requires a revision precondition.
	rr = doJSON(t, se, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"append_to_note","arguments":{"document_id":"`+docID+`","text":"tail"}}}`)
	if !strings.Contains(rr.Body.String(), "tail") {
		t.Fatalf("append_to_note failed: %s", rr.Body.String())
	}
	rr = doJSON(t, se, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"delete_note","arguments":{"document_id":"`+docID+`"}}}`)
	if !strings.Contains(rr.Body.String(), "base revision") {
		t.Fatalf("delete_note must require base_revision_id: %s", rr.Body.String())
	}
}

func TestHelpNotesAreReadOnly(t *testing.T) {
	s := newNotebookServer(t)

	// Seed a help note directly at the store level (as notriosctl seed-help does).
	doc, err := s.store.CreateDocument(t.Context(), store.CreateDocumentRequest{
		PreferredID: "doc_help_test", NotebookID: store.HelpNotebookID,
		Title: "Help page", Body: "read-only body",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Reading works; every mutation path returns 403.
	rr := doJSON(t, s, http.MethodGet, "/api/v1/documents/"+doc.ID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("read: %d", rr.Code)
	}
	put := httptest.NewRequest(http.MethodPut, "/api/v1/documents/"+doc.ID, strings.NewReader(`{"title":"x","body":"y"}`))
	put.Header.Set("If-Match", `"`+doc.CurrentRevisionID+`"`)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, put)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("PUT must be forbidden: %d %s", rec.Code, rec.Body.String())
	}
	del := httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+doc.ID, nil)
	del.Header.Set("If-Match", `"`+doc.CurrentRevisionID+`"`)
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, del)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("DELETE must be forbidden: %d", rec.Code)
	}
	rr = doJSON(t, s, http.MethodPost, "/api/v1/documents/"+doc.ID+"/append", `{"text":"nope"}`)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("append must be forbidden: %d", rr.Code)
	}
	rr = doJSON(t, s, http.MethodPost, "/api/v1/documents/"+doc.ID+"/notebook", `{"notebook_id":"nb_notes"}`)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("move out of Help must be forbidden: %d", rr.Code)
	}
}
