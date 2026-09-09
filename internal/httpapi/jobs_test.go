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

// seedJob records a run with a filesystem path among its parameters, which is
// the case the control-plane boundary exists for.
func seedJob(t *testing.T, s *Server) store.Job {
	t.Helper()
	job, err := s.store.CreateJob(context.Background(), store.CreateJobRequest{
		Kind: store.JobKindImportObsidian,
		Parameters: []store.JobParameter{
			{Name: "_vault-dir", Value: "/home/someone/Private Vault", Path: true},
		},
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	return job
}

func TestJobRoutesReadListAndCancel(t *testing.T) {
	s := newNotebookServer(t)
	job := seedJob(t, s)

	rr := doJSON(t, s, http.MethodGet, "/api/v1/jobs/"+job.ID, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get job: %d %s", rr.Code, rr.Body.String())
	}
	var got api.JobStatus
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != job.ID || got.Kind != store.JobKindImportObsidian || got.State != store.JobQueued {
		t.Fatalf("unexpected job: %+v", got)
	}
	if got.Settled {
		t.Fatal("a queued job has not settled")
	}

	list := doJSON(t, s, http.MethodGet, "/api/v1/jobs", "")
	var page api.JobPage
	if err := json.NewDecoder(list.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Jobs) != 1 || page.Jobs[0].ID != job.ID {
		t.Fatalf("listing: %+v", page)
	}

	cancel := doJSON(t, s, http.MethodPost, "/api/v1/jobs/"+job.ID+"/cancel", "")
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", cancel.Code, cancel.Body.String())
	}
	var cancelled api.JobStatus
	if err := json.NewDecoder(cancel.Body).Decode(&cancelled); err != nil {
		t.Fatal(err)
	}
	// Asking is not stopping. The work stops at its next durable boundary, so
	// the state is unchanged here and that is the contract, not a bug.
	if !cancelled.CancelRequested || cancelled.State != store.JobQueued {
		t.Fatalf("a cancel request sets a flag: %+v", cancelled)
	}

	missing := doJSON(t, s, http.MethodGet, "/api/v1/jobs/job_nope", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown job: %d %s", missing.Code, missing.Body.String())
	}
}

// The boundary that makes a control plane a control plane: it says what
// happened, never where. A job's parameters name places on this machine.
func TestJobRoutesNeverDiscloseParameters(t *testing.T) {
	s := newNotebookServer(t)
	job := seedJob(t, s)

	for _, path := range []string{"/api/v1/jobs", "/api/v1/jobs/" + job.ID} {
		body := doJSON(t, s, http.MethodGet, path, "").Body.String()
		// Asserted against the real value, so the test fails if the field is
		// ever added back under any name.
		if strings.Contains(body, "Private Vault") || strings.Contains(body, "parameters") {
			t.Fatalf("%s disclosed a job parameter: %s", path, body)
		}
		if !strings.Contains(body, job.ID) {
			t.Fatalf("%s should still identify the job: %s", path, body)
		}
	}
}

// MCP watches and nothing more, and its view drops the failure message too:
// a failure from a filesystem operation reads like a path.
func TestMCPJobToolsWatchWithoutDisclosingPaths(t *testing.T) {
	s := scopedServer(t, MCPScopeReadOnly)
	ctx := context.Background()
	job, err := s.store.CreateJob(ctx, store.CreateJobRequest{
		Kind:       store.JobKindExportArchiveV2,
		Parameters: []store.JobParameter{{Name: "_out-dir", Value: "/home/someone/Backups", Path: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.StartJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.FinishJob(ctx, job.ID, store.JobFailed, map[string]any{"documents": 12},
		errRefused("open /home/someone/Backups/secret: permission denied")); err != nil {
		t.Fatal(err)
	}

	out := callToolJSON(t, s, "get_job", `{"job_id":"`+job.ID+`"}`)
	if out["state"] != store.JobFailed || out["failed"] != true {
		t.Fatalf("the state must still be legible: %+v", out)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	// The path appears in the stored error, which REST returns — assert here
	// that the *MCP* view does not, and that the model is told where to ask.
	if strings.Contains(string(encoded), "/home/someone") || strings.Contains(string(encoded), "permission denied") {
		t.Fatalf("the MCP view leaked a local path: %s", encoded)
	}
	if !strings.Contains(string(encoded), "notriosctl jobs show") {
		t.Fatalf("a withheld reason should say where to find it: %s", encoded)
	}
	// A content-free summary does cross, because it is counts by construction.
	summary, _ := out["summary"].(map[string]any)
	if summary["documents"] != float64(12) {
		t.Fatalf("the summary should survive: %+v", out["summary"])
	}

	listed := callToolJSON(t, s, "list_jobs", `{}`)
	rows, _ := listed["jobs"].([]any)
	if len(rows) != 1 {
		t.Fatalf("expected one row: %+v", listed)
	}

	// REST, by contrast, keeps the message: it is the surface a person drives.
	body := doJSON(t, s, http.MethodGet, "/api/v1/jobs/"+job.ID, "").Body.String()
	if !strings.Contains(body, "permission denied") {
		t.Fatalf("REST should report why a job failed: %s", body)
	}
}

type errRefused string

func (e errRefused) Error() string { return string(e) }

// Tagging one note is a single-note write, which is what `editor` is for.
// Before v0.6 F7 the only MCP route to it was `run_batch` under `organizer`, so
// labelling a note you had just created required granting the ability to trash
// five hundred. That is a scope-design inconsistency, not a missing convenience.
func TestTagNoteIsAnEditorScopeWrite(t *testing.T) {
	ctx := context.Background()
	s := scopedServer(t, MCPScopeEditor)
	doc, err := s.store.CreateDocument(ctx, store.CreateDocumentRequest{Title: "Kitchen Plan", Body: "x\n"})
	if err != nil {
		t.Fatal(err)
	}

	out := callToolJSON(t, s, "tag_note", `{"document_id":"`+doc.ID+`","tags":["kitchen","todo"]}`)
	tags, _ := out["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("expected two tags: %+v", out)
	}

	// Removing a tag the note does not carry is not an error: a caller
	// enforcing a desired state should not have to check first.
	after := callToolJSON(t, s, "untag_note", `{"document_id":"`+doc.ID+`","tags":["todo","never-had-it"]}`)
	remaining, _ := after["tags"].([]any)
	if len(remaining) != 1 {
		t.Fatalf("expected one tag left: %+v", after)
	}

	// And it stops at the same boundary every other write does.
	report, _, err := s.store.WriteGraphReportNote(ctx, store.GraphReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"tag_note","arguments":{"document_id":"` +
		report.ID + `","tags":["mine"]}}}`
	if rr := doJSON(t, s, http.MethodPost, "/mcp", body); !strings.Contains(rr.Body.String(), "read-only") {
		t.Fatalf("tagging a read-only note should be refused: %s", rr.Body.String())
	}
}
