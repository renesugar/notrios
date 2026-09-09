package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncjobs"
)

type httpSyncTarget struct{}

func (httpSyncTarget) Plan(context.Context, string, int64) (map[string]any, error) {
	return map[string]any{"state_vector_entries": 2, "wanted_resources": 1}, nil
}
func (httpSyncTarget) Run(context.Context, string, int64, syncjobs.Progress) (map[string]any, error) {
	return map[string]any{}, nil
}

func g15Server(t *testing.T, syncScope string) (*Server, *store.SQLiteStore, *syncjobs.Manager, string) {
	t.Helper()
	st, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.MCP.SyncScope = syncScope
	server := NewServerWithOptions(ServerOptions{Store: st, Config: cfg})
	manager := syncjobs.New(st)
	targetID := store.SyncTargetID("directory:/private/carrier?token=secret")
	if err := manager.Register(targetID, httpSyncTarget{}); err != nil {
		t.Fatal(err)
	}
	server.AttachSyncJobs(manager, targetID)
	return server, st, manager, targetID
}

func TestG15RESTPlansStartsInspectsRetriesAndNeverReturnsTargetLocation(t *testing.T) {
	s, _, _, targetID := g15Server(t, MCPSyncDisabled)
	plan := doJSON(t, s, http.MethodPost, "/api/v1/jobs/sync/plan", `{"kind":"incremental","byte_budget":4096}`)
	if plan.Code != http.StatusOK || !strings.Contains(plan.Body.String(), targetID) {
		t.Fatalf("plan: %d %s", plan.Code, plan.Body.String())
	}
	started := doJSON(t, s, http.MethodPost, "/api/v1/jobs/sync/start", `{"kind":"resource_fetch","byte_budget":8192,"max_attempts":3}`)
	if started.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", started.Code, started.Body.String())
	}
	if strings.Contains(started.Body.String(), "/private") || strings.Contains(started.Body.String(), "secret") {
		t.Fatalf("target location/secret leaked: %s", started.Body.String())
	}
	var body struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	if err := json.Unmarshal(started.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	jobID := body.Job.ID
	if jobID == "" {
		t.Fatalf("missing job ID: %s", started.Body.String())
	}
	status := doJSON(t, s, http.MethodGet, "/api/v1/jobs/"+jobID+"/sync", "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), "queued") {
		t.Fatalf("status: %d %s", status.Code, status.Body.String())
	}
	reset := doJSON(t, s, http.MethodPost, "/api/v1/jobs/"+jobID+"/reset", "")
	if reset.Code != http.StatusOK || strings.Contains(reset.Body.String(), "/private") || strings.Contains(reset.Body.String(), "secret") {
		t.Fatalf("reset: %d %s", reset.Code, reset.Body.String())
	}
}

func TestSyncControlAndUIJSONConsumeExactlyOneBoundedValue(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		body       string
		length     int64
		wantStatus int
	}{
		{name: "second value", body: `{} {}`, length: int64(len(`{} {}`)), wantStatus: http.StatusBadRequest},
		{name: "oversized declared body", body: `{}`, length: (16 << 10) + 1, wantStatus: http.StatusRequestEntityTooLarge},
	} {
		t.Run("sync control "+testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/sync/plan", strings.NewReader(testCase.body))
			request.ContentLength = testCase.length
			response := httptest.NewRecorder()
			if _, ok := decodeSyncControl(response, request); ok {
				t.Fatal("sync-control decoder accepted a body outside its whole-body contract")
			}
			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.wantStatus, response.Body.String())
			}
		})
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sync-ui/configuration", strings.NewReader(`{} {}`))
	response := httptest.NewRecorder()
	var target struct{}
	if decodeBoundedJSON(response, request, 16<<10, &target) {
		t.Fatal("sync UI decoder accepted a second JSON value")
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("sync UI status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestG15MCPSyncToolsRequireExplicitScopeAndControlOnlyOwnSafeJobs(t *testing.T) {
	disabled, _, _, _ := g15Server(t, MCPSyncDisabled)
	errorText := syncToolError(t, disabled, "get_sync_status", `{}`)
	if !strings.Contains(errorText, "mcp.sync_scope") {
		t.Fatalf("disabled error = %q", errorText)
	}

	statusOnly, _, _, _ := g15Server(t, MCPSyncStatus)
	if result := callToolJSON(t, statusOnly, "get_sync_status", `{}`); result["available"] != true {
		t.Fatalf("status result = %+v", result)
	}
	if got := syncToolError(t, statusOnly, "start_sync", `{}`); !strings.Contains(got, "control") {
		t.Fatalf("status-only start error = %q", got)
	}

	control, st, _, _ := g15Server(t, MCPSyncControl)
	started := callToolJSON(t, control, "start_sync", `{"byte_budget":4096}`)
	jobMap, _ := started["job"].(map[string]any)
	jobID, _ := jobMap["id"].(string)
	if jobID == "" {
		t.Fatalf("MCP start = %+v", started)
	}
	if got := syncToolError(t, control, "cancel_sync_job", `{"job_id":"`+jobID+`"}`); got != "" {
		t.Fatalf("cancel own MCP job: %q", got)
	}
	local, err := st.CreateSyncJob(context.Background(), store.CreateSyncJobRequest{
		Kind: store.JobKindSyncCatchup, Actor: store.SyncJobActorREST,
		TargetID: store.SyncTargetID("directory:other"), ByteBudget: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := syncToolError(t, control, "cancel_sync_job", `{"job_id":"`+local.Job.ID+`"}`); !strings.Contains(got, "only its own") {
		t.Fatalf("MCP controlled another actor/catch-up: %q", got)
	}
}

func TestG15MCPGenericJobToolsHideSyncRecordsWithoutSyncScope(t *testing.T) {
	s, st, _, _ := g15Server(t, MCPSyncDisabled)
	job, err := st.CreateSyncJob(context.Background(), store.CreateSyncJobRequest{
		Kind: store.JobKindSyncIncremental, Actor: store.SyncJobActorREST,
		TargetID: store.SyncTargetID("rest:https://private.invalid/token"), ByteBudget: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	listed := callToolJSON(t, s, "list_jobs", `{}`)
	rows, _ := listed["jobs"].([]any)
	if len(rows) != 0 {
		t.Fatalf("generic list exposed sync job without sync scope: %+v", listed)
	}
	if got := syncToolError(t, s, "get_job", `{"job_id":"`+job.Job.ID+`"}`); !strings.Contains(got, "sync_scope") {
		t.Fatalf("generic get error = %q", got)
	}
}

func syncToolError(t *testing.T, s *Server, name, arguments string) string {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"` + name + `","arguments":` + arguments + `}}`
	rr := doJSON(t, s, http.MethodPost, "/mcp", body)
	var response struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result *struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode MCP response: %v (%s)", err, rr.Body.String())
	}
	if response.Error != nil {
		return response.Error.Message
	}
	if response.Result != nil && response.Result.IsError && len(response.Result.Content) > 0 {
		return response.Result.Content[0].Text
	}
	return ""
}
