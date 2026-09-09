package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSyncUITwoProcessRecoveryFlow is G16's user-facing working state. Both
// replicas are real notriosd processes; setup and operation use the same local
// HTTP routes as React rather than package seams or direct SQL.
func TestSyncUITwoProcessRecoveryFlow(t *testing.T) {
	cli := buildCLI(t)
	daemon := buildDaemon(t)
	host := newSyncReplica(t, cli, "host-ui", t.TempDir())
	joiner := newSyncReplica(t, cli, "joiner-ui", t.TempDir())
	host.run(t, "init")
	hostAddress, joinerAddress := unusedLoopbackAddress(t), unusedLoopbackAddress(t)
	hostBase, joinerBase := "http://"+hostAddress, "http://"+joinerAddress
	startDaemon(t, daemon, writeSyncUIProcessConfig(t, host, hostAddress, "", true), hostAddress)

	shared := uiJSON(t, http.MethodPost, hostBase+"/api/v1/documents",
		`{"title":"Shared fixture","body":"common base"}`, http.StatusCreated)
	sharedID, baseRevision := stringField(t, shared, "id"), stringField(t, shared, "current_revision_id")

	// Adopt is the explicit product path to a second replica of one logical
	// database; independently created libraries are correctly refused.
	adoptSnapshot(t, host, joiner)
	joiner.run(t, "init")
	startDaemon(t, daemon, writeSyncUIProcessConfig(t, joiner, joinerAddress, hostBase, false), joinerAddress)

	invitation := uiJSON(t, http.MethodPost, hostBase+"/api/v1/sync-ui/invitations",
		`{"label":"G16 local fixture","ttl_minutes":15}`, http.StatusCreated)
	code := stringField(t, invitation, "code")
	uiJSON(t, http.MethodPost, joinerBase+"/api/v1/sync-ui/pair",
		fmt.Sprintf(`{"base_url":%q,"code":%q}`, hostBase, code), http.StatusOK)
	joinerIdentity := uiRequest(t, http.MethodGet, joinerBase+"/api/v1/sync-ui", nil, "", http.StatusOK)
	joinerReplicaID := stringField(t, mapField(t, joinerIdentity["active_profile"]), "replica_id")
	uiJSON(t, http.MethodPost, hostBase+"/api/v1/sync-ui/peers/"+joinerReplicaID+"/snapshot-permission",
		`{"permitted":true}`, http.StatusOK)

	// Two offline edits of the same named parent must become a visible conflict,
	// while a new attachment's metadata arrives before its lazily fetched bytes.
	uiJSON(t, http.MethodPut, hostBase+"/api/v1/documents/"+sharedID,
		fmt.Sprintf(`{"title":"Shared fixture","body":"host edit","base_revision_id":%q}`, baseRevision), http.StatusOK)
	uiJSON(t, http.MethodPut, joinerBase+"/api/v1/documents/"+sharedID,
		fmt.Sprintf(`{"title":"Shared fixture","body":"joiner edit","base_revision_id":%q}`, baseRevision), http.StatusOK)
	resourceDoc := uiJSON(t, http.MethodPost, hostBase+"/api/v1/documents",
		`{"title":"Attachment fixture","body":"lazy bytes"}`, http.StatusCreated)
	resource := uiRequest(t, http.MethodPost, hostBase+"/api/v1/resources?filename=g16.txt",
		[]byte("G16 attachment bytes"), "text/plain", http.StatusCreated)
	resourceID := stringField(t, resource, "id")
	uiJSON(t, http.MethodPost, hostBase+"/api/v1/documents/"+stringField(t, resourceDoc, "id")+"/resources/"+resourceID,
		`{"relation_type":"attachment","ordinal":0}`, http.StatusOK)

	queueAndWaitSync(t, joinerBase, "incremental")
	queueAndWaitSync(t, hostBase, "incremental")
	queueAndWaitSync(t, joinerBase, "incremental")

	status := uiRequest(t, http.MethodGet, joinerBase+"/api/v1/sync-ui", nil, "", http.StatusOK)
	conflicts := sliceField(t, status, "conflicts")
	if len(conflicts) != 1 {
		hostStatus := uiRequest(t, http.MethodGet, hostBase+"/api/v1/sync-ui", nil, "", http.StatusOK)
		joinerDoc := uiRequest(t, http.MethodGet, joinerBase+"/api/v1/documents/"+sharedID, nil, "", http.StatusOK)
		hostDoc := uiRequest(t, http.MethodGet, hostBase+"/api/v1/documents/"+sharedID, nil, "", http.StatusOK)
		t.Fatalf("joiner conflicts = %v doc=%v jobs=%v; host conflicts = %v doc=%v jobs=%v; want one visible conflict",
			conflicts, joinerDoc, sliceField(t, status, "jobs"), sliceField(t, hostStatus, "conflicts"), hostDoc, sliceField(t, hostStatus, "jobs"))
	}
	resources := sliceField(t, status, "resources")
	if len(resources) == 0 || mapField(t, resources[0])["availability"] != "unavailable" {
		t.Fatalf("joiner lazy resources = %v, want unavailable metadata", resources)
	}
	intent := uiJSON(t, http.MethodPost, joinerBase+"/api/v1/sync-ui/resources/"+resourceID+"/intent",
		`{"pinned":true,"requested":true}`, http.StatusOK)
	job := mapField(t, intent["job"])
	waitUIJob(t, joinerBase, stringField(t, job, "id"))
	// The first fetch publishes the request. The REST host then serves the
	// requested object into its carrier, and an explicit retry materializes it.
	queueAndWaitSync(t, hostBase, "incremental")
	intent = uiJSON(t, http.MethodPost, joinerBase+"/api/v1/sync-ui/resources/"+resourceID+"/intent",
		`{"pinned":true,"requested":true}`, http.StatusOK)
	waitUIJob(t, joinerBase, stringField(t, mapField(t, intent["job"]), "id"))
	content := uiRaw(t, http.MethodGet, joinerBase+"/api/v1/resources/"+resourceID+"/content", nil, "", http.StatusOK)
	if string(content) != "G16 attachment bytes" {
		t.Fatalf("materialized attachment = %q", content)
	}

	conflict := mapField(t, conflicts[0])
	conflictID := stringField(t, conflict, "id")
	detail := uiRequest(t, http.MethodGet, joinerBase+"/api/v1/sync-ui/conflicts/"+conflictID, nil, "", http.StatusOK)
	if mapField(t, detail["base"])["body"] != "common base" {
		t.Fatalf("conflict comparison omitted the common base: %v", detail)
	}
	uiJSON(t, http.MethodPost, joinerBase+"/api/v1/sync-ui/conflicts/"+conflictID+"/resolve",
		`{"title":"Shared fixture","body":"explicit resolution"}`, http.StatusOK)
	queueAndWaitSync(t, joinerBase, "incremental")
	queueAndWaitSync(t, hostBase, "incremental")

	backup := uiRaw(t, http.MethodPost, joinerBase+"/api/v1/sync-ui/backups",
		[]byte(`{"password":"fixture backup password"}`), "application/json", http.StatusOK)
	if wrong := inspectUIBackup(t, joinerBase, backup, "wrong fixture password"); wrong.StatusCode != http.StatusUnauthorized {
		defer wrong.Body.Close()
		body, _ := io.ReadAll(wrong.Body)
		t.Fatalf("wrong backup password = HTTP %d %s", wrong.StatusCode, body)
	} else {
		wrong.Body.Close()
	}
	correct := inspectUIBackup(t, joinerBase, backup, "fixture backup password")
	defer correct.Body.Close()
	if correct.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(correct.Body)
		t.Fatalf("correct backup password = HTTP %d %s", correct.StatusCode, body)
	}
	var review map[string]any
	if err := json.NewDecoder(correct.Body).Decode(&review); err != nil || review["verified"] != true || review["applied"] != false {
		t.Fatalf("backup review mutated or failed verification: %v err=%v", review, err)
	}

	recovery := uiJSON(t, http.MethodPost, joinerBase+"/api/v1/sync-ui/recovery",
		`{"action":"catchup"}`, http.StatusAccepted)
	waitUIJob(t, joinerBase, stringField(t, mapField(t, recovery["job"]), "id"))
	if _, err := os.Stat(filepath.Join(filepath.Dir(joiner.db), "catchup-inbox")); err != nil {
		t.Fatalf("verified catch-up inbox missing: %v", err)
	}
}

func writeSyncUIProcessConfig(t *testing.T, replica *syncReplica, address, peerURL string, hostCarrier bool) string {
	t.Helper()
	root := filepath.Dir(replica.db)
	path := filepath.Join(root, "ui-profile.yaml")
	target := "rest"
	targetConfig := fmt.Sprintf("  rest_base_url: %q\n", peerURL)
	if hostCarrier {
		target = "directory"
		targetConfig = fmt.Sprintf("  directory: %q\n", filepath.Join(root, "sync-carrier"))
	}
	contents := fmt.Sprintf(`server:
  listen_addr: %q
  public_base_url: %q

data:
  directory: %q
  database_path: %q
  asset_store: %q

sync:
  target: %q
%s  rest:
    enabled: true
    require_tls: true
    key_file: %q
`, address, "http://"+address, root, replica.db, replica.assets, target, targetConfig, replica.keys)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func queueAndWaitSync(t *testing.T, baseURL, kind string) {
	t.Helper()
	started := uiJSON(t, http.MethodPost, baseURL+"/api/v1/jobs/sync/start",
		fmt.Sprintf(`{"kind":%q}`, kind), http.StatusAccepted)
	waitUIJob(t, baseURL, stringField(t, mapField(t, started["job"]), "id"))
}

func waitUIJob(t *testing.T, baseURL, jobID string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		job := uiRequest(t, http.MethodGet, baseURL+"/api/v1/jobs/"+jobID, nil, "", http.StatusOK)
		switch job["state"] {
		case "succeeded":
			return
		case "failed", "cancelled":
			t.Fatalf("job %s settled as %v: %v", jobID, job["state"], job)
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("job %s did not settle", jobID)
}

func inspectUIBackup(t *testing.T, baseURL string, backup []byte, password string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("password", password); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("backup", "fixture.npb")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(backup); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/sync-ui/backups/inspect", &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func uiJSON(t *testing.T, method, url, body string, want int) map[string]any {
	t.Helper()
	return uiRequest(t, method, url, []byte(body), "application/json", want)
}

func uiRequest(t *testing.T, method, url string, body []byte, contentType string, want int) map[string]any {
	t.Helper()
	raw := uiRaw(t, method, url, body, contentType, want)
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode %s %s response %q: %v", method, url, raw, err)
	}
	return value
}

func uiRaw(t *testing.T, method, url string, body []byte, contentType string, want int) []byte {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != want {
		t.Fatalf("%s %s = HTTP %d, want %d: %s", method, url, response.StatusCode, want, raw)
	}
	return raw
}

func stringField(t *testing.T, value map[string]any, field string) string {
	t.Helper()
	text, _ := value[field].(string)
	if strings.TrimSpace(text) == "" {
		t.Fatalf("missing %s in %v", field, value)
	}
	return text
}

func mapField(t *testing.T, value any) map[string]any {
	t.Helper()
	item, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value is not an object: %T %v", value, value)
	}
	return item
}

func sliceField(t *testing.T, value map[string]any, field string) []any {
	t.Helper()
	items, ok := value[field].([]any)
	if !ok {
		t.Fatalf("%s is not an array in %v", field, value)
	}
	return items
}
