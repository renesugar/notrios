package httpapi

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/renesugar/notrios/internal/store"
	"github.com/renesugar/notrios/internal/syncauth"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// SyncKeys is the local key material the sync surface needs: the group key it
// hands to a replica it is pairing with, its own signing identity, and — since
// G14 — the ability to seal a payload key for the enrolled group.
//
// It is an interface so the HTTP layer never learns where keys live. G13 ships
// the warned `0600` file provider from G11; v0.8 replaces it with a platform
// store and this signature does not change.
type SyncKeys interface {
	Current() (syncwire.GroupKey, error)
	Lookup(keyID string, epoch uint32) (syncwire.GroupKey, error)
	SignerKeyID() string
	Sign(message []byte) []byte
	PublicSigningKey() ed25519.PublicKey
}

// SyncSecurity is the peer-authenticated surface's runtime state.
type SyncSecurity struct {
	store    *store.SQLiteStore
	keys     SyncKeys
	verifier *syncauth.Verifier
	limiter  *syncauth.Limiter
	// maxBodyBytes bounds an authenticated request body.
	maxBodyBytes int64
	databaseID   string
	publicURL    string
	now          func() time.Time
	// signer is the same key material, named for what the data plane uses it
	// for: sealing a backup's payload key for the enrolled group.
	signer syncwire.Signer
	// carrierRoot is the folder this service hosts for its peers, and backups
	// is where produced snapshots live. Both are empty when the service carries
	// artifacts for nobody, which is the default.
	carrierRoot string
	backups     *backupStore
}

// DefaultSyncBodyBytes bounds a G13 request. The data plane is G14's, and it
// will raise this deliberately for object uploads; until then a sync request is
// a handshake or a pairing, both small.
const DefaultSyncBodyBytes = int64(1 << 20)

// AttachSyncSecurity enables the peer-authenticated sync surface.
//
// Nothing else about the service changes. Ordinary note routes keep exactly the
// posture they had — local, unauthenticated, loopback by default — because a
// peer credential is not a login and G13's resolved decision is that it reaches
// `/api/v1/sync/...` and nothing else.
func (s *Server) AttachSyncSecurity(canonical *store.SQLiteStore, keys SyncKeys, limits syncauth.Limits, now func() time.Time) error {
	if canonical == nil || keys == nil {
		return errors.New("httpapi: the sync surface needs a store and key material")
	}
	if now == nil {
		now = time.Now
	}
	identity, err := canonical.GetDatabaseIdentity(nil)
	if err != nil {
		return err
	}
	limiter := syncauth.NewLimiter(limits)
	lookup := func(signerKeyID string) (string, ed25519.PublicKey, bool) {
		keys, err := canonical.ListPeerSigningKeys(nil)
		if err != nil {
			return "", nil, false
		}
		for _, key := range keys {
			if key.SignerKeyID == signerKeyID && key.Status == "active" {
				return key.ReplicaID, key.PublicKey, true
			}
		}
		return "", nil, false
	}
	maxBody := s.config.Sync.REST.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = DefaultSyncBodyBytes
	}
	s.sync = &SyncSecurity{
		store: canonical, keys: keys, limiter: limiter, signer: keys,
		verifier:     syncauth.NewVerifier(identity.DatabaseID, lookup, limiter, now),
		maxBodyBytes: maxBody, databaseID: identity.DatabaseID,
		publicURL: s.config.Server.PublicBaseURL, now: now,
	}
	// The data plane needs somewhere to keep what it carries. Both directories
	// are derived from the configured data directory rather than from anything
	// a request says, which is what keeps "no arbitrary path parameters" true
	// at the only place it could stop being true.
	// The state root, not the data root: carrier spools and sync backups are
	// things Notrios must remember across runs but the user did not write.
	// They are backed up before a purge and are not the library itself.
	root := strings.TrimSpace(s.config.Data.StateDir)
	if root == "" {
		root = strings.TrimSpace(s.config.Data.Directory)
	}
	// Only when the peer surface is actually on. The spools belong to the REST
	// data plane, and creating them for a service with sync switched off makes
	// directories nothing will ever use -- which, with the compiled default
	// data root, meant every test that built a Config by hand created
	// ./data/sync-carrier wherever it happened to be running.
	if root != "" && s.config.Sync.REST.Enabled {
		// Both are created here rather than on first use. They are this
		// service's own directories under its own data root — not a mount point
		// somebody might not have plugged in — so a missing one is a directory
		// to make, and creating it at startup means a peer's first request does
		// not fail on a race two of them could lose.
		carrierRoot := filepath.Join(root, "sync-carrier")
		if err := os.MkdirAll(carrierRoot, 0o700); err == nil {
			s.sync.carrierRoot = carrierRoot
		}
		backupRoot := filepath.Join(root, "sync-backups")
		if err := os.MkdirAll(backupRoot, 0o700); err == nil {
			s.sync.backups = newBackupStore(backupRoot)
		}
	}
	return nil
}

// SyncEnabled reports whether the peer surface is attached and configured on.
func (s *Server) SyncEnabled() bool {
	return s.sync != nil && s.config.Sync.REST.Enabled
}

// syncRoutes registers the peer surface. The routes exist unconditionally and
// refuse when the surface is off, so a probe cannot distinguish "not
// configured" from "not built" and an operator gets an explanation rather than
// a 404.
func (s *Server) syncRoutes() {
	s.mux.HandleFunc("POST /api/v1/sync/pair", s.handleSyncPair)
	s.mux.HandleFunc("GET /api/v1/sync/handshake", s.requirePeer(s.handleSyncHandshake))
	s.mux.HandleFunc("GET /api/v1/sync/status", s.handleSyncStatus)
	s.dataRoutes()
}

// refuseBrowsers is the CSRF and CORS posture in one place.
//
// A peer is a program with a private key, never a browser. So the sync surface
// emits no CORS header at all — a browser therefore cannot read a response even
// if it makes the request — and refuses anything carrying browser context. A
// cross-site form cannot set the authorization header, but it can carry
// cookies, and a surface that ignored them would eventually be reached by a
// deputy that has some.
func refuseBrowsers(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Referer") != "" {
		writeError(w, http.StatusForbidden, "browser_context_refused",
			"the sync surface is for enrolled peers and refuses browser requests")
		return true
	}
	return false
}

// requirePeer authenticates a peer principal and authorizes it for this
// database's sync surface only.
func (s *Server) requirePeer(handler func(http.ResponseWriter, *http.Request, syncauth.Principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if refuseBrowsers(w, r) {
			return
		}
		if !s.SyncEnabled() {
			writeError(w, http.StatusNotFound, "sync_disabled",
				"the sync surface is not enabled on this service")
			return
		}
		address := remoteAddress(r)
		if s.sync.limiter.FailureBudgetExhausted(address, s.sync.now()) {
			// An address that has been guessing is answered before any work is
			// done. The budget is *checked* here and *spent* only on a refusal:
			// charging a peer for its own successful requests would throttle
			// the legitimate exchange rather than the guesser.
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many refused requests from this address")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.sync.maxBodyBytes))
		if err != nil {
			s.sync.limiter.AllowFailure(address, s.sync.now())
			s.recordAuthRefusal("", "oversized_body")
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "request body exceeds the configured limit")
			return
		}
		principal, err := s.sync.verifier.Verify(r.Header.Get("Authorization"), r.Method, r.URL.Path, syncauth.BodyDigest(body))
		if err != nil {
			reason := syncauth.Reason(err)
			s.sync.limiter.AllowFailure(address, s.sync.now())
			s.recordAuthRefusal(principalReplica(r), reason)
			status := http.StatusUnauthorized
			if errors.Is(err, syncauth.ErrRateLimited) {
				status = http.StatusTooManyRequests
			}
			// One message for every refusal: which check failed is in the audit
			// log, not in the answer to whoever failed it.
			writeError(w, status, "unauthorized", "this request is not authorized for this database's sync surface")
			return
		}
		enrolled, err := s.sync.store.SyncPeerEnrolled(r.Context(), principal.ReplicaID)
		if err != nil || !enrolled {
			s.sync.limiter.AllowFailure(address, s.sync.now())
			s.recordAuthRefusal(principal.ReplicaID, "not_enrolled_for_admission")
			writeError(w, http.StatusForbidden, "not_enrolled",
				"that replica is not configured for admission on this database")
			return
		}
		s.replaceBody(r, body)
		_ = s.sync.store.RecordSyncAuthEvent(r.Context(), "peer.auth_ok", principal.ReplicaID, "ok")
		handler(w, r, principal)
	}
}

func (s *Server) replaceBody(r *http.Request, body []byte) {
	r.Body = io.NopCloser(strings.NewReader(string(body)))
}

func (s *Server) recordAuthRefusal(replicaID, reason string) {
	if s.sync == nil {
		return
	}
	_ = s.sync.store.RecordSyncAuthEvent(nil, "peer.auth_refused", replicaID, reason)
}

// principalReplica reads the claimed replica id from an unverified credential,
// for the audit record only. It is a claim, and the audit row says so by
// pairing it with the reason the request was refused.
func principalReplica(r *http.Request) string {
	credential, err := syncauth.ParseCredential(r.Header.Get("Authorization"))
	if err != nil {
		return ""
	}
	return credential.ReplicaID
}

func remoteAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// handleSyncHandshake is the authenticated test endpoint G13's working state
// names: an enrolled peer proves who it is and learns what this replica is
// compatible with. It carries no note content and no operations — G14 owns the
// data plane.
func (s *Server) handleSyncHandshake(w http.ResponseWriter, r *http.Request, principal syncauth.Principal) {
	handshake, err := s.sync.store.LocalSyncHandshake(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "handshake_failed", "could not read the local sync state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"database_id":           handshake.DatabaseID,
		"replica_id":            handshake.ReplicaID,
		"protocol_major":        handshake.ProtocolMajor,
		"protocol_min_minor":    handshake.ProtocolMinMinor,
		"protocol_max_minor":    handshake.ProtocolMaxMinor,
		"schema_version":        handshake.SchemaVersion,
		"min_compatible_schema": handshake.MinCompatibleSchema,
		"max_compatible_schema": handshake.MaxCompatibleSchema,
		"required_capabilities": handshake.RequiredCapabilities,
		"state_vector":          handshake.StateVector,
		"peer":                  principal.ReplicaID,
	})
}

// handleSyncPair is the one unauthenticated route on the sync surface, because
// pairing is what happens before authentication is possible.
//
// Its gate is the invitation: short-lived, single-use, spent in one
// transaction. Every refusal costs the caller a token from the failure budget,
// so guessing codes is bounded by the limiter rather than by the network.
func (s *Server) handleSyncPair(w http.ResponseWriter, r *http.Request) {
	if refuseBrowsers(w, r) {
		return
	}
	if !s.SyncEnabled() {
		writeError(w, http.StatusNotFound, "sync_disabled", "the sync surface is not enabled on this service")
		return
	}
	address := remoteAddress(r)
	if s.sync.limiter.FailureBudgetExhausted(address, s.sync.now()) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many pairing attempts from this address")
		return
	}
	// Every pairing attempt spends a token whether or not it succeeds: unlike an
	// authenticated request, a pairing attempt *is* a guess at a secret, and
	// there is no legitimate reason to make hundreds of them.
	s.sync.limiter.AllowFailure(address, s.sync.now())
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, syncauth.MaxPairingBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "pairing request is too large")
		return
	}
	var request syncauth.PairingRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writeError(w, http.StatusBadRequest, "malformed_request", "pairing request is not valid JSON")
		return
	}
	code := r.Header.Get("X-Notrios-Pairing-Code")
	secret, err := syncauth.ParseCode(code)
	if err != nil {
		_ = s.sync.store.RecordSyncAuthEvent(r.Context(), "pairing.refused", request.ReplicaID, "malformed_code")
		writeError(w, http.StatusUnauthorized, "pairing_refused", "that pairing code is not usable")
		return
	}
	if err := syncauth.VerifyRequestProof(secret, request); err != nil {
		_ = s.sync.store.RecordSyncAuthEvent(r.Context(), "pairing.refused", request.ReplicaID, "bad_proof")
		writeError(w, http.StatusUnauthorized, "pairing_refused", "that pairing code is not usable")
		return
	}
	if request.DatabaseID != "" && request.DatabaseID != s.sync.databaseID {
		_ = s.sync.store.RecordSyncAuthEvent(r.Context(), "pairing.refused", request.ReplicaID, "wrong_database")
		writeError(w, http.StatusUnauthorized, "pairing_refused", "that pairing code is not usable")
		return
	}
	public, err := syncauth.DecodePublicKey(request.PublicKey)
	if err != nil {
		_ = s.sync.store.RecordSyncAuthEvent(r.Context(), "pairing.refused", request.ReplicaID, "malformed_key")
		writeError(w, http.StatusBadRequest, "malformed_request", "pairing request does not carry a usable public key")
		return
	}

	// Spending the invitation comes before enrolling anything, and it is one
	// transaction: two peers racing one code produce one pairing and one
	// refusal rather than two pairings.
	if _, err := s.sync.store.ConsumePairingInvitation(r.Context(), secret, request.ReplicaID, s.sync.now()); err != nil {
		writeError(w, http.StatusUnauthorized, "pairing_refused", "that pairing code is not usable")
		return
	}
	if _, err := s.sync.store.EnrollPeerSigningKey(r.Context(), request.ReplicaID, public, "paired over REST"); err != nil {
		writeError(w, http.StatusConflict, "pairing_conflict", "that replica or key is already enrolled differently")
		return
	}
	local, err := s.sync.store.LocalSyncHandshake(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pairing_failed", "could not read the local sync state")
		return
	}
	peer := syncstate.NewHandshake(local.DatabaseID, request.ReplicaID, store.CurrentSchemaVersion,
		syncstate.Vector{request.ReplicaID: 0})
	if err := s.sync.store.ConfigureSyncAdmissionPeer(r.Context(), peer); err != nil {
		writeError(w, http.StatusConflict, "pairing_conflict", "that replica could not be configured for admission")
		return
	}
	group, err := s.sync.keys.Current()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pairing_failed", "no local group key")
		return
	}
	wrapped, err := syncauth.WrapGroupKey(secret, group.Key[:])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pairing_failed", "could not wrap the group key")
		return
	}
	response := syncauth.PairingResponse{
		DatabaseID: local.DatabaseID, ReplicaID: local.ReplicaID,
		PublicKey: syncauth.EncodePublicKey(s.sync.keys.PublicSigningKey()),
		KeyID:     group.KeyID, Epoch: group.Epoch, WrappedGroup: wrapped,
		SchemaVersion: store.CurrentSchemaVersion,
		ProtocolMajor: syncstate.ProtocolMajor, ProtocolMinor: syncstate.ProtocolMinor,
		PublicBaseURL: s.sync.publicURL,
		InviterKeyID:  s.sync.keys.SignerKeyID(), InviterReplica: local.ReplicaID,
	}
	if response.ResponseProof, err = syncauth.ProveResponse(secret, response); err != nil {
		writeError(w, http.StatusInternalServerError, "pairing_failed", "could not prove the response")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// handleSyncStatus is the redacted local view. It is loopback-only: a peer has
// no business reading this replica's operational posture, and the operator
// reading it is already on the machine.
func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "loopback_only", "sync status is available on the loopback interface only")
		return
	}
	status := map[string]any{
		"enabled":     s.SyncEnabled(),
		"require_tls": s.config.Sync.REST.RequireTLS,
		"tls":         s.config.Sync.REST.TLSCertFile != "" && s.config.Sync.REST.TLSKeyFile != "",
		"limits": map[string]any{
			"max_body_bytes":      s.config.Sync.REST.MaxBodyBytes,
			"requests_per_minute": s.config.Sync.REST.RequestsPerMinute,
			"burst":               s.config.Sync.REST.Burst,
			"failures_per_minute": s.config.Sync.REST.FailuresPerMinute,
		},
	}
	if s.sync == nil {
		writeJSON(w, http.StatusOK, status)
		return
	}
	keys, err := s.sync.store.ListPeerSigningKeys(r.Context())
	if err == nil {
		// Key ids and status only. The public keys are not secret, but a status
		// endpoint that prints credentials teaches people to paste them.
		peers := make([]map[string]any, 0, len(keys))
		for _, key := range keys {
			peers = append(peers, map[string]any{
				"replica_id": key.ReplicaID, "signer_key_id": key.SignerKeyID,
				"status": key.Status, "enrolled_at": key.EnrolledAt, "revoked_at": key.RevokedAt,
			})
		}
		status["peer_keys"] = peers
	}
	if invitations, err := s.sync.store.ListPairingInvitations(r.Context(), 20); err == nil {
		open := 0
		for _, invitation := range invitations {
			if invitation.Status == "open" {
				open++
			}
		}
		status["open_invitations"] = open
	}
	if events, err := s.sync.store.ListSyncAuthEvents(r.Context(), 20); err == nil {
		status["recent_events"] = events
	}
	writeJSON(w, http.StatusOK, status)
}

// isLoopbackRequest reports whether a request arrived over the loopback
// interface. It reads the connection's own address and never a forwarding
// header: a header is written by whoever is talking to us.
func isLoopbackRequest(r *http.Request) bool {
	host := remoteAddress(r)
	if host == "" {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
