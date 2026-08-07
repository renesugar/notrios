package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/store"
)

const restTemplate = "```note-template\nprompt: project\n```\n# {{project}}\n\nOn {{date}}.\n"

func seedTemplateNote(t *testing.T, s *Server, title, body string) string {
	t.Helper()
	doc, err := s.store.CreateDocument(context.Background(), store.CreateDocumentRequest{Title: title, Body: body})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	return doc.ID
}

func TestTemplateRESTLifecycle(t *testing.T) {
	s := newNotebookServer(t)
	id := seedTemplateNote(t, s, "Kickoff", restTemplate)
	seedTemplateNote(t, s, "Plain note", "no template here")

	rr := doJSON(t, s, http.MethodGet, "/api/v1/templates", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}
	var listed struct {
		Templates []store.Template `json:"templates"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&listed)
	if len(listed.Templates) != 1 || listed.Templates[0].Prompts[0].Name != "project" {
		t.Fatalf("expected exactly the template with its prompt: %+v", listed.Templates)
	}

	rr = doJSON(t, s, http.MethodPost, "/api/v1/templates/"+id+"/create",
		`{"title":"Apollo","values":{"project":"Apollo"}}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "# Apollo") || strings.Contains(body, "note-template") {
		t.Fatalf("created note wrong: %s", body)
	}

	// A missing value is a 400, not a note with a hole in it.
	rr = doJSON(t, s, http.MethodPost, "/api/v1/templates/"+id+"/create", `{"title":"Apollo"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("a missing value must be refused: %d %s", rr.Code, rr.Body.String())
	}
}

func TestTaskRESTReportsCountsAndFilters(t *testing.T) {
	s := newNotebookServer(t)
	seedTemplateNote(t, s, "Chores", "- [ ] buy milk\n- [x] pay rent\n")

	rr := doJSON(t, s, http.MethodGet, "/api/v1/tasks", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("tasks: %d %s", rr.Code, rr.Body.String())
	}
	var all store.TaskList
	_ = json.NewDecoder(rr.Body).Decode(&all)
	if all.OpenCount != 1 || all.DoneCount != 1 || len(all.Tasks) != 2 {
		t.Fatalf("expected one of each: %+v", all)
	}
	// Every task carries a resolvable anchor URI, which is what makes it
	// addressable rather than merely listed.
	for _, task := range all.Tasks {
		if !strings.Contains(task.URI, "#^") {
			t.Fatalf("a task must be addressable: %+v", task)
		}
	}

	rr = doJSON(t, s, http.MethodGet, "/api/v1/tasks?state=open", "")
	var open store.TaskList
	_ = json.NewDecoder(rr.Body).Decode(&open)
	if len(open.Tasks) != 1 || open.Tasks[0].Text != "buy milk" {
		t.Fatalf("open filter: %+v", open.Tasks)
	}

	rr = doJSON(t, s, http.MethodGet, "/api/v1/tasks?state=maybe", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("an unknown state must be refused: %d", rr.Code)
	}
}

// The task anchor a listing hands out must actually resolve — otherwise it is
// a string that looks like a link.
func TestTaskAnchorResolves(t *testing.T) {
	s := newNotebookServer(t)
	ctx := context.Background()
	docID := seedTemplateNote(t, s, "Chores", "- [ ] buy milk ^milk\n")

	tasks, err := s.store.ListTasks(ctx, store.TaskListRequest{DocumentID: docID})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks.Tasks) != 1 {
		t.Fatalf("expected one task: %+v", tasks.Tasks)
	}
	blocks, err := s.store.ListDocumentBlocks(ctx, docID)
	if err != nil {
		t.Fatalf("ListDocumentBlocks: %v", err)
	}
	found := false
	for _, block := range blocks {
		if block.Marker == tasks.Tasks[0].Marker && block.ID == tasks.Tasks[0].BlockID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the task's identity must match a stored block row: task=%+v blocks=%+v", tasks.Tasks[0], blocks)
	}
}
