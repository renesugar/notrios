package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/api"
	"github.com/renesugar/notrios/internal/store"
)

func newNotebookServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return NewServerWithStore(st)
}

func doJSON(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body == "" {
		reader = bytes.NewBufferString("")
	} else {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	return rr
}

func TestNotebookRESTLifecycle(t *testing.T) {
	s := newNotebookServer(t)

	// Create a nested notebook pair with an emoji icon.
	rr := doJSON(t, s, http.MethodPost, "/api/v1/notebooks", `{"name":"Contacts","icon_emoji":"📇"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create notebook: %d %s", rr.Code, rr.Body.String())
	}
	var contacts api.Notebook
	_ = json.NewDecoder(rr.Body).Decode(&contacts)
	if contacts.IconEmoji != "📇" {
		t.Fatalf("emoji lost: %+v", contacts)
	}
	rr = doJSON(t, s, http.MethodPost, "/api/v1/notebooks", `{"name":"Plumbers","parent_id":"`+contacts.ID+`"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create child: %d %s", rr.Code, rr.Body.String())
	}

	// Case-insensitive duplicate is a 409 name_conflict.
	rr = doJSON(t, s, http.MethodPost, "/api/v1/notebooks", `{"name":"contacts"}`)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "name_conflict") {
		t.Fatalf("expected 409 name_conflict, got %d %s", rr.Code, rr.Body.String())
	}

	// Tree includes Contacts with its child, plus builtin Notes/Help.
	rr = doJSON(t, s, http.MethodGet, "/api/v1/notebooks/tree", "")
	var tree struct {
		Notebooks []api.NotebookTreeNode `json:"notebooks"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&tree)
	var found bool
	for _, node := range tree.Notebooks {
		if node.ID == contacts.ID && len(node.Children) == 1 && node.Children[0].Name == "Plumbers" {
			found = true
		}
	}
	if !found {
		t.Fatalf("nested tree missing: %+v", tree)
	}

	// Rename via PATCH.
	rr = doJSON(t, s, http.MethodPatch, "/api/v1/notebooks/"+contacts.ID, `{"name":"People","icon_emoji":"👥"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body.String())
	}

	// Builtin protection surfaces as 403.
	rr = doJSON(t, s, http.MethodDelete, "/api/v1/notebooks/"+store.HelpNotebookID, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting Help, got %d %s", rr.Code, rr.Body.String())
	}

	// Deleting a notebook trashes its notes.
	rr = doJSON(t, s, http.MethodPost, "/api/v1/documents", `{"title":"in contacts","body":"gone soon","notebook_id":"`+contacts.ID+`"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create doc: %d %s", rr.Code, rr.Body.String())
	}
	var doc api.Document
	_ = json.NewDecoder(rr.Body).Decode(&doc)
	if doc.NotebookID != contacts.ID {
		t.Fatalf("notebook_id not honored on create: %+v", doc)
	}
	rr = doJSON(t, s, http.MethodDelete, "/api/v1/notebooks/"+contacts.ID, "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete notebook: %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodGet, "/api/v1/trash", "")
	if !strings.Contains(rr.Body.String(), doc.ID) {
		t.Fatalf("trashed note missing from /trash: %s", rr.Body.String())
	}
}

func TestTagAndMoveREST(t *testing.T) {
	s := newNotebookServer(t)

	rr := doJSON(t, s, http.MethodPost, "/api/v1/documents", `{"title":"taggable","body":"alpha"}`)
	var doc api.Document
	_ = json.NewDecoder(rr.Body).Decode(&doc)

	rr = doJSON(t, s, http.MethodPost, "/api/v1/documents/"+doc.ID+"/tags/shopping%20mall", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("add tag: %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodGet, "/api/v1/tags", "")
	if !strings.Contains(rr.Body.String(), `"shopping mall"`) || !strings.Contains(rr.Body.String(), `"note_count":1`) {
		t.Fatalf("tag list wrong: %s", rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodDelete, "/api/v1/documents/"+doc.ID+"/tags/SHOPPING%20MALL", "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("remove tag: %d %s", rr.Code, rr.Body.String())
	}

	// Move the note to a new notebook.
	rr = doJSON(t, s, http.MethodPost, "/api/v1/notebooks", `{"name":"Bookmarks","icon_emoji":"🔖"}`)
	var nb api.Notebook
	_ = json.NewDecoder(rr.Body).Decode(&nb)
	rr = doJSON(t, s, http.MethodPost, "/api/v1/documents/"+doc.ID+"/notebook", `{"notebook_id":"`+nb.ID+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("move: %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodGet, "/api/v1/notebooks/"+nb.ID+"/notes", "")
	if !strings.Contains(rr.Body.String(), doc.ID) {
		t.Fatalf("note not listed in notebook: %s", rr.Body.String())
	}
}

func TestSearchNotebookAndTrashREST(t *testing.T) {
	s := newNotebookServer(t)

	// Builtin search notebooks are listed in sidebar order.
	rr := doJSON(t, s, http.MethodGet, "/api/v1/search-notebooks", "")
	var list struct {
		SearchNotebooks []api.SearchNotebook `json:"search_notebooks"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&list)
	if len(list.SearchNotebooks) != 2 || list.SearchNotebooks[0].Name != "All notes" || list.SearchNotebooks[1].Name != "Trash" {
		t.Fatalf("builtin search notebooks wrong: %+v", list.SearchNotebooks)
	}

	rr = doJSON(t, s, http.MethodPost, "/api/v1/search-notebooks", `{"name":"TODO","query":"tag:todo","icon_emoji":"✅"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create search notebook: %d %s", rr.Code, rr.Body.String())
	}
	var todo api.SearchNotebook
	_ = json.NewDecoder(rr.Body).Decode(&todo)

	rr = doJSON(t, s, http.MethodDelete, "/api/v1/search-notebooks/"+store.AllNotesSearchNotebookID, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 deleting All notes, got %d", rr.Code)
	}
	rr = doJSON(t, s, http.MethodDelete, "/api/v1/search-notebooks/"+todo.ID, "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete user search notebook: %d", rr.Code)
	}

	// Trash restore round trip over REST.
	rr = doJSON(t, s, http.MethodPost, "/api/v1/documents", `{"title":"trash me","body":"beta"}`)
	var doc api.Document
	_ = json.NewDecoder(rr.Body).Decode(&doc)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+doc.ID, nil)
	req.Header.Set("If-Match", `"`+doc.CurrentRevisionID+`"`)
	del := httptest.NewRecorder()
	s.ServeHTTP(del, req)
	if del.Code != http.StatusNoContent {
		t.Fatalf("soft delete: %d %s", del.Code, del.Body.String())
	}
	rr = doJSON(t, s, http.MethodPost, "/api/v1/trash/"+doc.ID+"/restore", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodGet, "/api/v1/documents/"+doc.ID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("restored doc unreadable: %d", rr.Code)
	}

	// Purge a local trashed note via REST.
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+doc.ID, nil)
	rr2 := doJSON(t, s, http.MethodGet, "/api/v1/documents/"+doc.ID, "")
	var doc2 api.Document
	_ = json.NewDecoder(rr2.Body).Decode(&doc2)
	req.Header.Set("If-Match", `"`+doc2.CurrentRevisionID+`"`)
	del = httptest.NewRecorder()
	s.ServeHTTP(del, req)
	if del.Code != http.StatusNoContent {
		t.Fatalf("second delete: %d %s", del.Code, del.Body.String())
	}
	rr = doJSON(t, s, http.MethodDelete, "/api/v1/trash/"+doc.ID, "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("purge: %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodGet, "/api/v1/trash", "")
	if strings.Contains(rr.Body.String(), doc.ID) {
		t.Fatalf("purged doc still in trash: %s", rr.Body.String())
	}
}

func TestMCPNotebookTools(t *testing.T) {
	s := newNotebookServer(t)

	rr := doJSON(t, s, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	body := rr.Body.String()
	for _, tool := range []string{"list_notebooks", "get_notebook_tree", "list_tags", "list_search_notebooks"} {
		if !strings.Contains(body, tool) {
			t.Fatalf("tools/list missing %s: %s", tool, body)
		}
	}

	rr = doJSON(t, s, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_notebooks"}}`)
	if !strings.Contains(rr.Body.String(), store.DefaultNotebookID) || !strings.Contains(rr.Body.String(), "Help") {
		t.Fatalf("list_notebooks result wrong: %s", rr.Body.String())
	}
	rr = doJSON(t, s, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_search_notebooks"}}`)
	if !strings.Contains(rr.Body.String(), "All notes") || !strings.Contains(rr.Body.String(), "Trash") {
		t.Fatalf("list_search_notebooks result wrong: %s", rr.Body.String())
	}
}
