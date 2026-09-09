package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// testKeys is the smallest thing satisfying SyncKeys.
type testKeys struct {
	group   syncwire.GroupKey
	private ed25519.PrivateKey
}

func (k *testKeys) Current() (syncwire.GroupKey, error)              { return k.group, nil }
func (k *testKeys) Lookup(string, uint32) (syncwire.GroupKey, error) { return k.group, nil }
func (k *testKeys) Sign(message []byte) []byte                       { return ed25519.Sign(k.private, message) }
func (k *testKeys) SignerKeyID() string {
	return syncwire.SignerKeyID(k.private.Public().(ed25519.PublicKey))
}
func (k *testKeys) PublicSigningKey() ed25519.PublicKey {
	return k.private.Public().(ed25519.PublicKey)
}

type securityFixture struct {
	server   *Server
	store    *store.SQLiteStore
	peer     ed25519.PrivateKey
	peerID   string
	keyID    string
	database string
	now      time.Time
}

// newSecurityFixture builds a service with the sync surface on and one enrolled
// peer, which is the state every authorization question is asked from.
func newSecurityFixture(t *testing.T) *securityFixture {
	t.Helper()
	ctx := context.Background()
	st, err := store.OpenSQLiteWithAssetStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollLocalJournal(ctx, "G13 security fixture"); err != nil {
		t.Fatal(err)
	}
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	// Put every root in this test's own directory. Left at the compiled
	// default, the sync spools would be created under ./data relative to the
	// package, which is source-tree pollution and a test not exercising the
	// layout it claims.
	config.UseDataDirectory(&cfg, t.TempDir(), nil)
	cfg.Sync.REST.Enabled = true
	server := NewServerWithOptions(ServerOptions{Store: st, Config: cfg})

	_, localPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys := &testKeys{private: localPrivate, group: syncwire.GroupKey{KeyID: "key_test", Epoch: 1}}
	fixture := &securityFixture{
		server: server, store: st, database: identity.DatabaseID,
		peerID: "rep_peer_g13", now: time.Now().UTC(),
	}
	if err := server.AttachSyncSecurity(st, keys, syncauth.DefaultLimits(), func() time.Time { return fixture.now }); err != nil {
		t.Fatal(err)
	}

	peerPublic, peerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fixture.peer = peerPrivate
	if fixture.keyID, err = st.EnrollPeerSigningKey(ctx, fixture.peerID, peerPublic, "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := st.ConfigureSyncAdmissionPeer(ctx, syncstate.NewHandshake(identity.DatabaseID, fixture.peerID,
		store.CurrentSchemaVersion, syncstate.Vector{fixture.peerID: 0})); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// signed builds an authenticated request the way a peer's client does.
func (f *securityFixture) signed(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	nonce := make([]byte, syncauth.NonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	request := syncauth.Request{
		Method: method, Path: path, DatabaseID: f.database, ReplicaID: f.peerID,
		Timestamp: f.now, Nonce: hex.EncodeToString(nonce), BodySHA256: syncauth.BodyDigest(body),
	}
	header, err := syncauth.Sign(f.peer, f.keyID, request)
	if err != nil {
		t.Fatal(err)
	}
	httpRequest := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	httpRequest.Header.Set("Authorization", header)
	httpRequest.RemoteAddr = "127.0.0.1:54321"
	return httpRequest
}

func (f *securityFixture) do(request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	f.server.ServeHTTP(recorder, request)
	return recorder
}

func TestAnEnrolledPeerReachesTheSyncHandshake(t *testing.T) {
	fixture := newSecurityFixture(t)
	response := fixture.do(fixture.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("an enrolled peer was refused: %d %s", response.Code, response.Body.String())
	}
	var handshake map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &handshake); err != nil {
		t.Fatal(err)
	}
	if handshake["database_id"] != fixture.database || handshake["peer"] != fixture.peerID {
		t.Fatalf("handshake does not name the database and the peer: %v", handshake)
	}
	// A peer surface never emits CORS headers: a browser must not be able to
	// read this even if something persuades it to make the request.
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("the sync surface emitted a CORS header")
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

// TestTheAuthenticationMatrix is the table G13's validation asks for: one row
// per way a request can fail to be authorized, each asserting a refusal rather
// than a specific message.
func TestTheAuthenticationMatrix(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		prepare func(*testing.T, *securityFixture) *http.Request
		status  int
	}{
		{
			name: "anonymous",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				request := httptest.NewRequest(http.MethodGet, "/api/v1/sync/handshake", nil)
				request.RemoteAddr = "127.0.0.1:1"
				return request
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "signed by a key nobody enrolled",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				_, stranger, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				saved := f.peer
				f.peer = stranger
				request := f.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil)
				f.peer = saved
				return request
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "a good signature over a different body",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				request := f.signed(t, http.MethodGet, "/api/v1/sync/handshake", []byte(`{"a":1}`))
				request.Body = http.NoBody
				return request
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "a good signature for a different path",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				request := f.signed(t, http.MethodGet, "/api/v1/sync/status", nil)
				replay := httptest.NewRequest(http.MethodGet, "/api/v1/sync/handshake", nil)
				replay.Header.Set("Authorization", request.Header.Get("Authorization"))
				replay.RemoteAddr = "127.0.0.1:2"
				return replay
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "a stale timestamp",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				request := f.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil)
				f.now = f.now.Add(2 * syncauth.MaxSkew)
				return request
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "a revoked key",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				if err := f.store.RevokePeerSigningKey(context.Background(), f.keyID, "lost device"); err != nil {
					t.Fatal(err)
				}
				return f.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil)
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "a browser, by its Origin",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				request := f.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil)
				request.Header.Set("Origin", "https://evil.example")
				return request
			},
			status: http.StatusForbidden,
		},
		{
			name: "a browser, by its cookies",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				request := f.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil)
				request.Header.Set("Cookie", "session=whatever")
				return request
			},
			status: http.StatusForbidden,
		},
		{
			name: "an enrolled key whose replica is not configured for admission",
			prepare: func(t *testing.T, f *securityFixture) *http.Request {
				public, private, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				keyID, err := f.store.EnrollPeerSigningKey(context.Background(), "rep_unadmitted", public, "test")
				if err != nil {
					t.Fatal(err)
				}
				saved, savedID, savedPeer := f.peer, f.keyID, f.peerID
				f.peer, f.keyID, f.peerID = private, keyID, "rep_unadmitted"
				request := f.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil)
				f.peer, f.keyID, f.peerID = saved, savedID, savedPeer
				return request
			},
			status: http.StatusForbidden,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newSecurityFixture(t)
			response := fixture.do(testCase.prepare(t, fixture))
			if response.Code != testCase.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, testCase.status, response.Body.String())
			}
			// Whatever was wrong, the answer says only that it was not
			// authorized. Telling a caller which check it failed is telling it
			// how to pass.
			body := response.Body.String()
			for _, leak := range []string{"signature", "nonce", "timestamp", "revoked", "expired"} {
				if strings.Contains(strings.ToLower(body), leak) {
					t.Fatalf("the refusal explains itself (%q): %s", leak, body)
				}
			}
		})
	}
}

func TestAReplayedRequestIsRefusedOnce(t *testing.T) {
	fixture := newSecurityFixture(t)
	request := fixture.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil)
	header := request.Header.Get("Authorization")
	if response := fixture.do(request); response.Code != http.StatusOK {
		t.Fatalf("the first use was refused: %d", response.Code)
	}
	replay := httptest.NewRequest(http.MethodGet, "/api/v1/sync/handshake", nil)
	replay.Header.Set("Authorization", header)
	replay.RemoteAddr = "127.0.0.1:54321"
	if response := fixture.do(replay); response.Code != http.StatusUnauthorized {
		t.Fatalf("a replayed credential was accepted: %d", response.Code)
	}
	// And the audit trail says what happened, in the closed vocabulary.
	events, err := fixture.store.ListSyncAuthEvents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event["event_type"] == "peer.auth_refused" && strings.Contains(event["details"], "replayed_nonce") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the replay was not audited: %v", events)
	}
}

func TestFailedRequestsAreRateLimitedPerAddress(t *testing.T) {
	fixture := newSecurityFixture(t)
	refused := 0
	limited := 0
	for attempt := 0; attempt < 40; attempt++ {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/sync/handshake", nil)
		request.Header.Set("Authorization", syncauth.Scheme+` replica="x", key="y", ts="`+
			fixture.now.Format(time.RFC3339Nano)+`", nonce="`+strings.Repeat("ab", syncauth.NonceBytes)+`", sig="AAAA"`)
		request.RemoteAddr = "203.0.113.9:5000"
		switch fixture.do(request).Code {
		case http.StatusUnauthorized:
			refused++
		case http.StatusTooManyRequests:
			limited++
		}
	}
	if limited == 0 {
		t.Fatalf("guessing was never rate limited (%d refusals)", refused)
	}
}

func TestSyncCredentialsAuthorizeNothingOutsideTheSyncSurface(t *testing.T) {
	fixture := newSecurityFixture(t)
	// The resolved decision: a peer credential reaches /api/v1/sync/... and
	// nothing else. Presenting one on an ordinary route must not change what
	// that route does — it neither grants nor denies anything, because ordinary
	// routes keep their existing local posture.
	request := fixture.signed(t, http.MethodGet, "/api/v1/documents/doc_missing", nil)
	response := fixture.do(request)
	if response.Code == http.StatusOK {
		t.Fatalf("a sync credential produced a successful ordinary-route response: %s", response.Body.String())
	}
	anonymous := httptest.NewRequest(http.MethodGet, "/api/v1/documents/doc_missing", nil)
	anonymous.RemoteAddr = "127.0.0.1:1"
	if plain := fixture.do(anonymous); plain.Code != response.Code {
		t.Fatalf("a sync credential changed an ordinary route's answer: %d with, %d without",
			response.Code, plain.Code)
	}
}

func TestTheSyncSurfaceRefusesWhenDisabled(t *testing.T) {
	fixture := newSecurityFixture(t)
	cfg := fixture.server.config
	cfg.Sync.REST.Enabled = false
	fixture.server.config = cfg
	if response := fixture.do(fixture.signed(t, http.MethodGet, "/api/v1/sync/handshake", nil)); response.Code != http.StatusNotFound {
		t.Fatalf("a disabled surface answered %d", response.Code)
	}
}

func TestSyncStatusIsLoopbackOnlyAndRedacted(t *testing.T) {
	fixture := newSecurityFixture(t)
	local := httptest.NewRequest(http.MethodGet, "/api/v1/sync/status", nil)
	local.RemoteAddr = "127.0.0.1:9999"
	response := fixture.do(local)
	if response.Code != http.StatusOK {
		t.Fatalf("loopback status = %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, fixture.keyID) {
		t.Fatalf("status does not report the enrolled key: %s", body)
	}
	// Key ids and status, never key material.
	if strings.Contains(body, "public_key") || strings.Contains(body, "signing_key") {
		t.Fatalf("status printed key material: %s", body)
	}
	remote := httptest.NewRequest(http.MethodGet, "/api/v1/sync/status", nil)
	remote.RemoteAddr = "198.51.100.7:5000"
	if response := fixture.do(remote); response.Code != http.StatusForbidden {
		t.Fatalf("remote status = %d, want 403", response.Code)
	}
	// A forwarding header is written by whoever is talking to us and must not
	// turn a remote request into a local one.
	forged := httptest.NewRequest(http.MethodGet, "/api/v1/sync/status", nil)
	forged.RemoteAddr = "198.51.100.7:5000"
	forged.Header.Set("X-Forwarded-For", "127.0.0.1")
	if response := fixture.do(forged); response.Code != http.StatusForbidden {
		t.Fatalf("a forwarded-for header granted loopback access: %d", response.Code)
	}
}

func TestPairingSpendsACodeExactlyOnce(t *testing.T) {
	ctx := context.Background()
	fixture := newSecurityFixture(t)
	secret, err := syncauth.NewPairingSecret()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.CreatePairingInvitation(ctx, secret, 15*time.Minute, "test"); err != nil {
		t.Fatal(err)
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	request := syncauth.PairingRequest{
		InvitationID: store.InvitationID(secret), DatabaseID: fixture.database,
		ReplicaID: "rep_joiner", PublicKey: syncauth.EncodePublicKey(public),
	}
	if request.Proof, err = syncauth.ProveRequest(secret, request); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	pair := func(code string, payload []byte) *httptest.ResponseRecorder {
		httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sync/pair", strings.NewReader(string(payload)))
		httpRequest.Header.Set("X-Notrios-Pairing-Code", code)
		httpRequest.RemoteAddr = "127.0.0.1:5555"
		return fixture.do(httpRequest)
	}

	response := pair(syncauth.FormatCode(secret), body)
	if response.Code != http.StatusOK {
		t.Fatalf("pairing was refused: %d %s", response.Code, response.Body.String())
	}
	var answer syncauth.PairingResponse
	if err := json.Unmarshal(response.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if err := syncauth.VerifyResponseProof(secret, answer); err != nil {
		t.Fatalf("the response does not prove the inviter holds the code: %v", err)
	}
	// The group key came back sealed, and nothing in the response is a secret
	// in the clear.
	group, err := syncauth.UnwrapGroupKey(secret, answer.WrappedGroup)
	if err != nil || len(group) != syncwire.GroupKeyBytes {
		t.Fatalf("the wrapped group key did not open: %v", err)
	}
	if strings.Contains(response.Body.String(), string(group)) {
		t.Fatal("the response carried the group key in the clear")
	}

	// The same code again, from anyone, is refused: single use is a
	// transaction, not a convention.
	if second := pair(syncauth.FormatCode(secret), body); second.Code == http.StatusOK {
		t.Fatal("a pairing code worked twice")
	}
	// And a wrong code never worked in the first place.
	other, err := syncauth.NewPairingSecret()
	if err != nil {
		t.Fatal(err)
	}
	if wrong := pair(syncauth.FormatCode(other), body); wrong.Code == http.StatusOK {
		t.Fatal("a pairing code nobody issued was accepted")
	}
}

func TestAnExpiredCodeIsRefused(t *testing.T) {
	ctx := context.Background()
	fixture := newSecurityFixture(t)
	secret, err := syncauth.NewPairingSecret()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.CreatePairingInvitation(ctx, secret, time.Minute, "short"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.ConsumePairingInvitation(ctx, secret, "rep_late", time.Now().Add(2*time.Hour)); err == nil {
		t.Fatal("an expired invitation was consumed")
	}
}
