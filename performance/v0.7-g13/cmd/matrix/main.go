// Command matrix produces v0.7 G13's authentication and authorization matrix.
//
// Every row is one way a request can arrive at the sync surface, run against a
// real service with a real enrolled peer. The point is not that the code passes
// its own tests — the test suite does that — but that the matrix is written
// down as a table someone can read against the threat model, and regenerated
// when the surface changes.
//
// Everything here is generated. No private corpus, note, path, or key material
// is read or recorded.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/config"
	"github.com/renesugar/notrios/internal/httpapi"
	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

type row struct {
	Case          string `json:"case"`
	Route         string `json:"route"`
	Status        int    `json:"status"`
	Authorized    bool   `json:"authorized"`
	AuditedReason string `json:"audited_reason,omitempty"`
	Explains      bool   `json:"answer_explains_the_failure"`
}

type report struct {
	Schema       string   `json:"schema"`
	GeneratedFor string   `json:"generated_for"`
	SchemaV      int      `json:"database_schema_version"`
	Matrix       []row    `json:"matrix"`
	Vocabulary   []string `json:"audit_reason_vocabulary"`
	Transport    []struct {
		Listen  string `json:"listen_addr"`
		TLS     bool   `json:"tls_configured"`
		Refused bool   `json:"startup_refused"`
	} `json:"transport_policy"`
	Notes []string `json:"notes"`
}

type keys struct {
	group   syncwire.GroupKey
	private ed25519.PrivateKey
}

func (k *keys) Current() (syncwire.GroupKey, error) { return k.group, nil }
func (k *keys) SignerKeyID() string {
	return syncwire.SignerKeyID(k.private.Public().(ed25519.PublicKey))
}
func (k *keys) PublicSigningKey() ed25519.PublicKey { return k.private.Public().(ed25519.PublicKey) }

func main() {
	out := flag.String("out", "", "path to write the JSON report")
	flag.Parse()
	if *out == "" {
		fail(fmt.Errorf("-out is required"))
	}

	result := report{
		Schema:       "notrios.g13.authmatrix.v1",
		GeneratedFor: "v0.7 G13 REST security foundation, pairing, and transport policy",
		SchemaV:      store.CurrentSchemaVersion,
		Vocabulary:   syncauth.Reasons(),
		Notes: []string{
			"Every library, key, and note here is generated; nothing private is read or recorded.",
			"Each row is a real request against a real service with one enrolled peer.",
			"answer_explains_the_failure must be false everywhere: which check failed belongs in the audit log, not in the reply.",
			"A peer credential authorizes /api/v1/sync/... only; ordinary note routes keep their existing local posture.",
		},
	}

	for _, testCase := range cases() {
		result.Matrix = append(result.Matrix, run(testCase))
	}
	for _, policy := range transportCases() {
		result.Transport = append(result.Transport, policy)
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
	authorized := 0
	for _, entry := range result.Matrix {
		if entry.Authorized {
			authorized++
		}
	}
	fmt.Printf("%d cases, %d authorized, %d refused\n", len(result.Matrix), authorized, len(result.Matrix)-authorized)
	fmt.Printf("wrote %s\n", *out)
}

type harness struct {
	server   *httpapi.Server
	store    *store.SQLiteStore
	peer     ed25519.PrivateKey
	peerID   string
	keyID    string
	database string
	now      time.Time
}

func newHarness() (*harness, error) {
	ctx := context.Background()
	workspace, err := os.MkdirTemp("", "notrios-g13-")
	if err != nil {
		return nil, err
	}
	st, err := store.OpenSQLiteWithAssetStore(":memory:", workspace)
	if err != nil {
		return nil, err
	}
	if err := st.Bootstrap(ctx); err != nil {
		return nil, err
	}
	if _, err := st.EnrollLocalJournal(ctx, "G13 matrix"); err != nil {
		return nil, err
	}
	identity, err := st.GetDatabaseIdentity(ctx)
	if err != nil {
		return nil, err
	}
	cfg := config.Default()
	cfg.Sync.REST.Enabled = true
	server := httpapi.NewServerWithOptions(httpapi.ServerOptions{Store: st, Config: cfg})
	_, localPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	h := &harness{server: server, store: st, database: identity.DatabaseID, peerID: "rep_matrix_peer", now: time.Now().UTC()}
	if err := server.AttachSyncSecurity(st, &keys{private: localPrivate,
		group: syncwire.GroupKey{KeyID: "key_matrix", Epoch: 1}},
		syncauth.DefaultLimits(), func() time.Time { return h.now }); err != nil {
		return nil, err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	h.peer = private
	if h.keyID, err = st.EnrollPeerSigningKey(ctx, h.peerID, public, "matrix"); err != nil {
		return nil, err
	}
	if err := st.ConfigureSyncAdmissionPeer(ctx, syncstate.NewHandshake(identity.DatabaseID, h.peerID,
		store.CurrentSchemaVersion, syncstate.Vector{h.peerID: 0})); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *harness) sign(method, path string, body []byte) *http.Request {
	nonce := make([]byte, syncauth.NonceBytes)
	_, _ = rand.Read(nonce)
	header, err := syncauth.Sign(h.peer, h.keyID, syncauth.Request{
		Method: method, Path: path, DatabaseID: h.database, ReplicaID: h.peerID,
		Timestamp: h.now, Nonce: hex.EncodeToString(nonce), BodySHA256: syncauth.BodyDigest(body),
	})
	if err != nil {
		fail(err)
	}
	request := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	request.Header.Set("Authorization", header)
	request.RemoteAddr = "127.0.0.1:40000"
	return request
}

type matrixCase struct {
	name    string
	route   string
	prepare func(*harness) *http.Request
}

func cases() []matrixCase {
	return []matrixCase{
		{"an enrolled peer", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			return h.sign(http.MethodGet, "/api/v1/sync/handshake", nil)
		}},
		{"no credential at all", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/sync/handshake", nil)
			request.RemoteAddr = "127.0.0.1:40001"
			return request
		}},
		{"a signature that is not the enrolled key's", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			_, stranger, _ := ed25519.GenerateKey(rand.Reader)
			saved := h.peer
			h.peer = stranger
			request := h.sign(http.MethodGet, "/api/v1/sync/handshake", nil)
			h.peer = saved
			return request
		}},
		{"a revoked credential", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			_ = h.store.RevokePeerSigningKey(context.Background(), h.keyID, "matrix")
			return h.sign(http.MethodGet, "/api/v1/sync/handshake", nil)
		}},
		{"a signature for another path", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			signed := h.sign(http.MethodGet, "/api/v1/sync/status", nil)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/sync/handshake", nil)
			request.Header.Set("Authorization", signed.Header.Get("Authorization"))
			request.RemoteAddr = "127.0.0.1:40002"
			return request
		}},
		{"a signature for another body", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			request := h.sign(http.MethodGet, "/api/v1/sync/handshake", []byte(`{"changed":true}`))
			request.Body = http.NoBody
			return request
		}},
		{"a stale timestamp", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			request := h.sign(http.MethodGet, "/api/v1/sync/handshake", nil)
			h.now = h.now.Add(2 * syncauth.MaxSkew)
			return request
		}},
		{"a browser, by Origin", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			request := h.sign(http.MethodGet, "/api/v1/sync/handshake", nil)
			request.Header.Set("Origin", "https://example.invalid")
			return request
		}},
		{"a browser, by Cookie", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			request := h.sign(http.MethodGet, "/api/v1/sync/handshake", nil)
			request.Header.Set("Cookie", "session=x")
			return request
		}},
		{"an enrolled key whose replica is not admitted", "/api/v1/sync/handshake", func(h *harness) *http.Request {
			public, private, _ := ed25519.GenerateKey(rand.Reader)
			keyID, err := h.store.EnrollPeerSigningKey(context.Background(), "rep_unadmitted", public, "matrix")
			if err != nil {
				fail(err)
			}
			savedKey, savedID, savedPeer := h.peer, h.keyID, h.peerID
			h.peer, h.keyID, h.peerID = private, keyID, "rep_unadmitted"
			request := h.sign(http.MethodGet, "/api/v1/sync/handshake", nil)
			h.peer, h.keyID, h.peerID = savedKey, savedID, savedPeer
			return request
		}},
		{"a peer credential on an ordinary note route", "/api/v1/documents/doc_absent", func(h *harness) *http.Request {
			return h.sign(http.MethodGet, "/api/v1/documents/doc_absent", nil)
		}},
		{"pairing with no code", "/api/v1/sync/pair", func(h *harness) *http.Request {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/sync/pair", strings.NewReader("{}"))
			request.RemoteAddr = "127.0.0.1:40003"
			return request
		}},
		{"pairing with a code nobody issued", "/api/v1/sync/pair", func(h *harness) *http.Request {
			secret, _ := syncauth.NewPairingSecret()
			public, _, _ := ed25519.GenerateKey(rand.Reader)
			pairing := syncauth.PairingRequest{
				InvitationID: store.InvitationID(secret), DatabaseID: h.database,
				ReplicaID: "rep_stranger", PublicKey: syncauth.EncodePublicKey(public),
			}
			pairing.Proof, _ = syncauth.ProveRequest(secret, pairing)
			body, _ := json.Marshal(pairing)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/sync/pair", strings.NewReader(string(body)))
			request.Header.Set("X-Notrios-Pairing-Code", syncauth.FormatCode(secret))
			request.RemoteAddr = "127.0.0.1:40004"
			return request
		}},
		{"pairing with a valid code", "/api/v1/sync/pair", func(h *harness) *http.Request {
			secret, _ := syncauth.NewPairingSecret()
			if _, err := h.store.CreatePairingInvitation(context.Background(), secret, 15*time.Minute, "matrix"); err != nil {
				fail(err)
			}
			public, _, _ := ed25519.GenerateKey(rand.Reader)
			pairing := syncauth.PairingRequest{
				InvitationID: store.InvitationID(secret), DatabaseID: h.database,
				ReplicaID: "rep_joiner", PublicKey: syncauth.EncodePublicKey(public),
			}
			pairing.Proof, _ = syncauth.ProveRequest(secret, pairing)
			body, _ := json.Marshal(pairing)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/sync/pair", strings.NewReader(string(body)))
			request.Header.Set("X-Notrios-Pairing-Code", syncauth.FormatCode(secret))
			request.RemoteAddr = "127.0.0.1:40005"
			return request
		}},
		{"status from loopback", "/api/v1/sync/status", func(h *harness) *http.Request {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/sync/status", nil)
			request.RemoteAddr = "127.0.0.1:40006"
			return request
		}},
		{"status from elsewhere", "/api/v1/sync/status", func(h *harness) *http.Request {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/sync/status", nil)
			request.RemoteAddr = "198.51.100.4:40007"
			return request
		}},
		{"status from elsewhere claiming to be loopback", "/api/v1/sync/status", func(h *harness) *http.Request {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/sync/status", nil)
			request.RemoteAddr = "198.51.100.4:40008"
			request.Header.Set("X-Forwarded-For", "127.0.0.1")
			return request
		}},
	}
}

func run(testCase matrixCase) row {
	h, err := newHarness()
	if err != nil {
		fail(err)
	}
	defer h.store.Close()
	recorder := httptest.NewRecorder()
	h.server.ServeHTTP(recorder, testCase.prepare(h))
	entry := row{
		Case: testCase.name, Route: testCase.route, Status: recorder.Code,
		Authorized: recorder.Code >= 200 && recorder.Code < 300,
	}
	// Only a refusal is checked for explaining itself. The loopback status
	// endpoint reports the local audit log on purpose — that is the operator's
	// own view of why things were refused, which is the opposite of leaking it
	// to whoever was refused.
	if !entry.Authorized {
		body := strings.ToLower(recorder.Body.String())
		for _, leak := range []string{"signature", "nonce", "timestamp", "revoked", "expired"} {
			if strings.Contains(body, leak) {
				entry.Explains = true
			}
		}
	}
	if events, err := h.store.ListSyncAuthEvents(context.Background(), 20); err == nil {
		for _, event := range events {
			switch event["event_type"] {
			case "peer.auth_refused", "peer.auth_ok", "pairing.refused", "pairing.consumed":
				if entry.AuditedReason == "" {
					entry.AuditedReason = event["event_type"] + ":" + extractReason(event["details"])
				}
			}
		}
	}
	return entry
}

func extractReason(details string) string {
	var decoded map[string]string
	if err := json.Unmarshal([]byte(details), &decoded); err != nil {
		return ""
	}
	return decoded["reason"]
}

func transportCases() []struct {
	Listen  string `json:"listen_addr"`
	TLS     bool   `json:"tls_configured"`
	Refused bool   `json:"startup_refused"`
} {
	workspace, err := os.MkdirTemp("", "notrios-g13-tls-")
	if err != nil {
		fail(err)
	}
	certificate := workspace + "/cert.pem"
	key := workspace + "/key.pem"
	for _, path := range []string{certificate, key} {
		if err := os.WriteFile(path, []byte("generated placeholder"), 0o600); err != nil {
			fail(err)
		}
	}
	var results []struct {
		Listen  string `json:"listen_addr"`
		TLS     bool   `json:"tls_configured"`
		Refused bool   `json:"startup_refused"`
	}
	for _, probe := range []struct {
		listen string
		tls    bool
	}{
		{"127.0.0.1:8080", false},
		{"0.0.0.0:8080", false},
		{":8080", false},
		{"192.168.1.10:8080", false},
		{"0.0.0.0:8443", true},
	} {
		cfg := config.Default()
		cfg.Server.ListenAddr = probe.listen
		cfg.Sync.REST.Enabled = true
		if probe.tls {
			cfg.Sync.REST.TLSCertFile = certificate
			cfg.Sync.REST.TLSKeyFile = key
		}
		results = append(results, struct {
			Listen  string `json:"listen_addr"`
			TLS     bool   `json:"tls_configured"`
			Refused bool   `json:"startup_refused"`
		}{probe.listen, probe.tls, config.ValidateSyncTransport(cfg) != nil})
	}
	return results
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
