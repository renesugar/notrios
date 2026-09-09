package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

type syncUITestKeys struct{ *testKeys }

func (k *syncUITestKeys) AdoptGroupKey(keyID string, epoch uint32, key []byte) error {
	k.group = syncwire.GroupKey{KeyID: keyID, Epoch: epoch}
	copy(k.group.Key[:], key)
	return nil
}

type syncUITestSecrets struct{ keys *syncUITestKeys }

func (s syncUITestSecrets) ProviderName() string           { return "test-memory" }
func (s syncUITestSecrets) Warning() string                { return "test-only ephemeral keys" }
func (s syncUITestSecrets) Open() (SyncLocalKeys, error)   { return s.keys, nil }
func (s syncUITestSecrets) Create() (SyncLocalKeys, error) { return s.keys, nil }

func newSyncUITestSecrets(t *testing.T) syncUITestSecrets {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return syncUITestSecrets{keys: &syncUITestKeys{testKeys: &testKeys{private: private}}}
}

func syncUITestServer(t *testing.T) (*Server, *store.SQLiteStore, config.Config) {
	t.Helper()
	root := t.TempDir()
	st, err := store.OpenSQLiteWithAssetStore(filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.ConfigPath = filepath.Join(root, "profile.yaml")
	cfg.Profile.ID, cfg.Profile.Name = "profile_ui", "Work"
	cfg.Data.Directory, cfg.Data.DatabasePath, cfg.Data.AssetStore = root, filepath.Join(root, "notes.sqlite"), filepath.Join(root, "assets")
	cfg.Server.ListenAddr, cfg.Server.PublicBaseURL = "127.0.0.1:18191", "http://127.0.0.1:18191"
	if err := config.WriteProfileFile(cfg.ConfigPath, cfg); err != nil {
		t.Fatal(err)
	}
	return NewServerWithOptions(ServerOptions{Store: st, Config: cfg}), st, cfg
}

func localSyncUIRequest(t *testing.T, server *Server, method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.RemoteAddr = "127.0.0.1:45231"
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	return response
}

func TestSyncUIStatusIsLoopbackOnlyAndProfileExplicit(t *testing.T) {
	server, _, _ := syncUITestServer(t)
	local := localSyncUIRequest(t, server, http.MethodGet, "/api/v1/sync-ui", nil, "")
	if local.Code != http.StatusOK {
		t.Fatalf("local status: %d %s", local.Code, local.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(local.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	active := status["active_profile"].(map[string]any)
	if active["name"] != "Work" || active["profile_id"] != "profile_ui" || active["database_id"] == "" || active["replica_id"] == "" {
		t.Fatalf("active profile is not explicit: %+v", active)
	}
	if strings.Contains(local.Body.String(), "sync-keys") || strings.Contains(local.Body.String(), "private_key") {
		t.Fatalf("status disclosed secret material: %s", local.Body.String())
	}
	remoteRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sync-ui", nil)
	remoteRequest.RemoteAddr = "192.0.2.10:4123"
	remote := httptest.NewRecorder()
	server.ServeHTTP(remote, remoteRequest)
	if remote.Code != http.StatusForbidden {
		t.Fatalf("remote status = %d, want 403", remote.Code)
	}
}

func TestSyncUIConfigurationWritesOnlyActiveProfileAndRequiresRestart(t *testing.T) {
	server, _, cfg := syncUITestServer(t)
	directory := filepath.Join(filepath.Dir(cfg.ConfigPath), "carrier")
	body := []byte(`{"target":"directory","directory":` + strconvQuote(directory) + `}`)
	response := localSyncUIRequest(t, server, http.MethodPut, "/api/v1/sync-ui/configuration", body, "application/json")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"restart_required":true`) {
		t.Fatalf("configuration: %d %s", response.Code, response.Body.String())
	}
	written, err := config.Load(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if written.Sync.Target != "directory" || written.Sync.Directory != directory || written.Sync.RESTBaseURL != "" {
		t.Fatalf("written sync config = %+v", written.Sync)
	}
	info, err := os.Stat(cfg.ConfigPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("profile config mode = %v err=%v", info, err)
	}
	bad := localSyncUIRequest(t, server, http.MethodPut, "/api/v1/sync-ui/configuration",
		[]byte(`{"target":"rest","rest_base_url":"http://example.com"}`), "application/json")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("plaintext remote URL = %d %s", bad.Code, bad.Body.String())
	}
	rest := localSyncUIRequest(t, server, http.MethodPut, "/api/v1/sync-ui/configuration",
		[]byte(`{"target":"rest","rest_base_url":"http://127.0.0.1:18192","rest_inbound_enabled":true}`), "application/json")
	if rest.Code != http.StatusOK {
		t.Fatalf("loopback REST configuration = %d %s", rest.Code, rest.Body.String())
	}
	written, err = config.Load(cfg.ConfigPath)
	if err != nil || !written.Sync.REST.Enabled {
		t.Fatalf("explicit inbound setting was not retained: enabled=%v err=%v", written.Sync.REST.Enabled, err)
	}
}

func TestSyncUIPasswordBackupWrongPasswordRetryAndReview(t *testing.T) {
	server, st, _ := syncUITestServer(t)
	if _, err := st.CreateDocument(context.Background(), store.CreateDocumentRequest{Title: "Backup", Body: "private local fixture"}); err != nil {
		t.Fatal(err)
	}
	created := localSyncUIRequest(t, server, http.MethodPost, "/api/v1/sync-ui/backups",
		[]byte(`{"password":"correct horse battery staple"}`), "application/json")
	if created.Code != http.StatusOK || created.Header().Get("Content-Disposition") == "" || !bytes.HasPrefix(created.Body.Bytes(), []byte("NPB1")) {
		t.Fatalf("backup create: %d headers=%v body-prefix=%q", created.Code, created.Header(), created.Body.Bytes()[:min(12, created.Body.Len())])
	}
	inspect := func(password string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("password", password)
		part, err := writer.CreateFormFile("backup", "backup.npb")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(created.Body.Bytes())
		_ = writer.Close()
		return localSyncUIRequest(t, server, http.MethodPost, "/api/v1/sync-ui/backups/inspect", body.Bytes(), writer.FormDataContentType())
	}
	wrong := inspect("wrong password")
	if wrong.Code != http.StatusUnauthorized || !strings.Contains(wrong.Body.String(), "Nothing was changed") {
		t.Fatalf("wrong password: %d %s", wrong.Code, wrong.Body.String())
	}
	correct := inspect("correct horse battery staple")
	if correct.Code != http.StatusOK || !strings.Contains(correct.Body.String(), `"verified":true`) ||
		!strings.Contains(correct.Body.String(), `"applied":false`) {
		t.Fatalf("correct password review: %d %s", correct.Code, correct.Body.String())
	}
}

func TestSyncUISnapshotPermissionIsSeparateAndExplicit(t *testing.T) {
	server, st, _ := syncUITestServer(t)
	identity, err := st.GetDatabaseIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const replicaID = "replica_g16_peer"
	if _, err := st.EnrollLocalJournal(context.Background(), "G16 fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollPeerSigningKey(context.Background(), replicaID, public, "G16 fixture"); err != nil {
		t.Fatal(err)
	}
	if err := st.ConfigureSyncAdmissionPeer(context.Background(), syncstate.NewHandshake(identity.DatabaseID,
		replicaID, store.CurrentSchemaVersion, syncstate.Vector{replicaID: 0})); err != nil {
		t.Fatal(err)
	}
	if st.SnapshotSources().PermittedSource(replicaID) {
		t.Fatal("pairing silently granted complete-snapshot permission")
	}
	grant := localSyncUIRequest(t, server, http.MethodPost,
		"/api/v1/sync-ui/peers/"+replicaID+"/snapshot-permission", []byte(`{"permitted":true}`), "application/json")
	if grant.Code != http.StatusOK || !st.SnapshotSources().PermittedSource(replicaID) {
		t.Fatalf("grant = %d %s", grant.Code, grant.Body.String())
	}
	revoke := localSyncUIRequest(t, server, http.MethodPost,
		"/api/v1/sync-ui/peers/"+replicaID+"/snapshot-permission", []byte(`{"permitted":false}`), "application/json")
	if revoke.Code != http.StatusOK || st.SnapshotSources().PermittedSource(replicaID) {
		t.Fatalf("revoke = %d %s", revoke.Code, revoke.Body.String())
	}
}

func TestSyncUIRetentionIsDryRunOnlyAndRedacted(t *testing.T) {
	server, st, _ := syncUITestServer(t)
	if _, err := st.EnrollLocalJournal(context.Background(), "G17 retention fixture"); err != nil {
		t.Fatal(err)
	}
	response := localSyncUIRequest(t, server, http.MethodGet, "/api/v1/sync-ui/retention", nil, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"dry_run":true`) ||
		!strings.Contains(response.Body.String(), "No verified retained snapshot") {
		t.Fatalf("retention review = %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "path") || strings.Contains(response.Body.String(), "signature") {
		t.Fatalf("retention review disclosed local details: %s", response.Body.String())
	}
	apply := localSyncUIRequest(t, server, http.MethodPost, "/api/v1/sync-ui/retention/apply",
		[]byte(`{"digest":"anything"}`), "application/json")
	if apply.Code != http.StatusMethodNotAllowed {
		t.Fatalf("web retention apply exists: %d %s", apply.Code, apply.Body.String())
	}
}

func TestSyncUIPeerRetirementRequiresPreviewAndExactConfirmation(t *testing.T) {
	server, st, _ := syncUITestServer(t)
	ctx := context.Background()
	if _, err := st.EnrollLocalJournal(ctx, "G17 retirement fixture"); err != nil {
		t.Fatal(err)
	}
	identity, _ := st.GetDatabaseIdentity(ctx)
	const replicaID = "replica_g17_drawer_phone"
	peer := syncstate.NewHandshake(identity.DatabaseID, replicaID, store.CurrentSchemaVersion, syncstate.Vector{replicaID: 0})
	if err := st.ConfigureSyncAdmissionPeer(ctx, peer); err != nil {
		t.Fatal(err)
	}
	peerPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollPeerSigningKey(ctx, replicaID, peerPublic, "G17 fixture"); err != nil {
		t.Fatal(err)
	}
	server.AttachSyncSecretStore(newSyncUITestSecrets(t))
	preview := localSyncUIRequest(t, server, http.MethodPost,
		"/api/v1/sync-ui/peers/"+replicaID+"/retirement-preview", []byte(`{}`), "application/json")
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), `"confirmation":"retire-peer:`+replicaID+`"`) ||
		!strings.Contains(preview.Body.String(), "old credentials cannot re-enroll") {
		t.Fatalf("retirement preview = %d %s", preview.Code, preview.Body.String())
	}
	wrong := localSyncUIRequest(t, server, http.MethodPost, "/api/v1/sync-ui/peers/"+replicaID+"/retire",
		[]byte(`{"confirmation":"retire-peer:somebody-else"}`), "application/json")
	if wrong.Code != http.StatusBadRequest {
		t.Fatalf("wrong confirmation = %d %s", wrong.Code, wrong.Body.String())
	}
	if active, err := st.SyncPeerEnrolled(ctx, replicaID); err != nil || !active {
		t.Fatalf("wrong confirmation mutated peer: active=%v err=%v", active, err)
	}
	confirmed := localSyncUIRequest(t, server, http.MethodPost, "/api/v1/sync-ui/peers/"+replicaID+"/retire",
		[]byte(`{"confirmation":"retire-peer:`+replicaID+`","reason":"device recycled"}`), "application/json")
	if confirmed.Code != http.StatusOK || !strings.Contains(confirmed.Body.String(), `"status":"retired"`) {
		t.Fatalf("confirmed retirement = %d %s", confirmed.Code, confirmed.Body.String())
	}
	status := localSyncUIRequest(t, server, http.MethodGet, "/api/v1/sync-ui", nil, "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"status":"retired"`) {
		t.Fatalf("retired status = %d %s", status.Code, status.Body.String())
	}
	for _, forbidden := range []string{"device recycled", `"signature"`, `"signer_key_id"`} {
		if strings.Contains(status.Body.String(), forbidden) {
			t.Fatalf("status disclosed retirement secret/detail %q: %s", forbidden, status.Body.String())
		}
	}
}

func strconvQuote(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
