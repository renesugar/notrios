package store

/*
#include "csqlite/sqlite3.h"
*/
import "C"

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrPairingInvitation reports an invitation that cannot be used: unknown,
// expired, already consumed, or revoked. Which one is deliberately not
// distinguished to a caller over the network — an attacker guessing codes
// learns nothing from the difference — while the audit record says exactly
// which it was.
var ErrPairingInvitation = errors.New("pairing invitation is not usable")

// ensureSchemaV25 adds G13's peer signing keys and pairing invitations. It is a
// CREATE-only migration: nothing existing changes meaning.
func (s *SQLiteStore) ensureSchemaV25(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	version, err := s.pragmaUserVersionLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if version >= 25 {
		return nil
	}
	migration, err := migrationFS.ReadFile("migrations/0025_sync_rest_security.sql")
	if err != nil {
		return fmt.Errorf("read schema v25 migration: %w", err)
	}
	return s.Exec(ctx, string(migration))
}

// PeerSigningKey is one enrolled peer identity.
type PeerSigningKey struct {
	SignerKeyID string
	ReplicaID   string
	PublicKey   ed25519.PublicKey
	Status      string
	EnrolledAt  string
	RevokedAt   string
	Reason      string
}

// EnrollPeerSigningKey records the public key a peer signs with.
//
// Enrolling is idempotent for the same key and refused for a different one: a
// replica that appears with a new key has either rotated it — which is a
// deliberate act with its own command — or is not that replica.
func (s *SQLiteStore) EnrollPeerSigningKey(ctx context.Context, replicaID string, public ed25519.PublicKey, reason string) (string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if replicaID == "" || len(public) != ed25519.PublicKeySize {
		return "", fmt.Errorf("%w: a peer key names its replica and is an Ed25519 public key", ErrInvalidInput)
	}
	keyID := signerKeyID(public)
	encoded := base64.StdEncoding.EncodeToString(public)
	auditID, err := NewID("audit")
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return "", err
	}
	existing, found, err := s.peerSigningKeyLocked(keyID)
	if err != nil {
		return "", err
	}
	if found {
		if existing.ReplicaID != replicaID {
			return "", fmt.Errorf("%w: that key is already enrolled for another replica", ErrConflict)
		}
		if existing.Status == "revoked" {
			return "", fmt.Errorf("%w: that key was revoked and cannot be re-enrolled", ErrConflict)
		}
		return keyID, nil
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`INSERT INTO sync_peer_keys(signer_key_id, replica_id, public_key, status, reason)
		VALUES(?, ?, ?, 'active', ?)`, keyID, replicaID, encoded, reason); err != nil {
		return "", err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'peer.key_enrolled', ?, ?, ?)`, auditID, local.ReplicaID, replicaID,
		auditDetails(map[string]string{"signer_key_id": keyID, "reason": boundedReason(reason)})); err != nil {
		return "", err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return "", err
	}
	committed = true
	return keyID, nil
}

// RevokePeerSigningKey refuses every future request signed by a key.
//
// It is separate from retiring a peer, which G17 owns: revoking a key says
// "this device's credential is no longer trusted", while retiring a peer says
// "this replica is gone and its history may be collected". Conflating them
// would make a lost laptop a reason to start deleting tombstones.
func (s *SQLiteStore) RevokePeerSigningKey(ctx context.Context, signerKeyID, reason string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	auditID, err := NewID("audit")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return err
	}
	existing, found, err := s.peerSigningKeyLocked(signerKeyID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: no such enrolled key", ErrNotFound)
	}
	if existing.Status == "revoked" {
		return nil
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`UPDATE sync_peer_keys
		SET status = 'revoked', revoked_at = CURRENT_TIMESTAMP, reason = ?
		WHERE signer_key_id = ?`, boundedReason(reason), signerKeyID); err != nil {
		return err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'peer.key_revoked', ?, ?, ?)`, auditID, local.ReplicaID, existing.ReplicaID,
		auditDetails(map[string]string{"signer_key_id": signerKeyID, "reason": boundedReason(reason)})); err != nil {
		return err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// ListPeerSigningKeys returns every enrolled key, revoked ones included, so a
// user can see what was trusted as well as what is.
func (s *SQLiteStore) ListPeerSigningKeys(ctx context.Context) ([]PeerSigningKey, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT signer_key_id, replica_id, public_key, status,
		enrolled_at, COALESCE(revoked_at, ''), reason FROM sync_peer_keys ORDER BY enrolled_at, signer_key_id`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	keys := []PeerSigningKey{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			key := PeerSigningKey{
				SignerKeyID: columnText(stmt, 0), ReplicaID: columnText(stmt, 1),
				Status: columnText(stmt, 3), EnrolledAt: columnText(stmt, 4),
				RevokedAt: columnText(stmt, 5), Reason: columnText(stmt, 6),
			}
			if raw, err := base64.StdEncoding.DecodeString(columnText(stmt, 2)); err == nil && len(raw) == ed25519.PublicKeySize {
				key.PublicKey = ed25519.PublicKey(raw)
			}
			keys = append(keys, key)
		case C.SQLITE_DONE:
			return keys, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func (s *SQLiteStore) peerSigningKeyLocked(signerKeyID string) (PeerSigningKey, bool, error) {
	stmt, err := s.prepareLocked(`SELECT signer_key_id, replica_id, public_key, status,
		enrolled_at, COALESCE(revoked_at, ''), reason FROM sync_peer_keys WHERE signer_key_id = ?`)
	if err != nil {
		return PeerSigningKey{}, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{signerKeyID}); err != nil {
		return PeerSigningKey{}, false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return PeerSigningKey{}, false, nil
	case C.SQLITE_ROW:
		key := PeerSigningKey{
			SignerKeyID: columnText(stmt, 0), ReplicaID: columnText(stmt, 1),
			Status: columnText(stmt, 3), EnrolledAt: columnText(stmt, 4),
			RevokedAt: columnText(stmt, 5), Reason: columnText(stmt, 6),
		}
		if raw, err := base64.StdEncoding.DecodeString(columnText(stmt, 2)); err == nil && len(raw) == ed25519.PublicKeySize {
			key.PublicKey = ed25519.PublicKey(raw)
		}
		return key, true, nil
	default:
		return PeerSigningKey{}, false, s.stepErrLocked(rc)
	}
}

// SyncPeerVerifier resolves a signer key id to an enrolled, active public key.
// It satisfies syncwire.Verifier, so the carrier, the REST surface, and the
// artifact codec all decide "do we trust this signature?" from one place.
type SyncPeerVerifier struct{ store *SQLiteStore }

// PeerVerifier returns the database-backed verifier.
func (s *SQLiteStore) PeerVerifier() *SyncPeerVerifier { return &SyncPeerVerifier{store: s} }

// PublicKey implements syncwire.Verifier. A revoked key is reported as unknown,
// which is the honest answer to "may I trust this signature?".
func (v *SyncPeerVerifier) PublicKey(signerKeyID string) (ed25519.PublicKey, bool) {
	v.store.mu.Lock()
	defer v.store.mu.Unlock()
	key, found, err := v.store.peerSigningKeyLocked(signerKeyID)
	if err != nil || !found || key.Status != "active" || len(key.PublicKey) != ed25519.PublicKeySize {
		return nil, false
	}
	return key.PublicKey, true
}

// PairingInvitation is one short-lived, single-use pairing offer.
type PairingInvitation struct {
	ID                string
	Status            string
	CreatedAt         string
	ExpiresAt         string
	ConsumedAt        string
	ConsumerReplicaID string
	Label             string
}

// InvitationID derives the lookup identifier from the secret, so a presenter
// can be found without transmitting the secret itself.
func InvitationID(secret []byte) string {
	digest := sha256.Sum256(append([]byte("notrios.pairing-id.v1"), secret...))
	return hex.EncodeToString(digest[:8])
}

func invitationSecretHash(secret []byte) string {
	digest := sha256.Sum256(append([]byte("notrios.pairing-secret.v1"), secret...))
	return hex.EncodeToString(digest[:])
}

// CreatePairingInvitation records a one-use invitation and returns it. The
// secret is supplied by the caller and never stored: only its hash is.
func (s *SQLiteStore) CreatePairingInvitation(ctx context.Context, secret []byte, ttl time.Duration, label string) (PairingInvitation, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return PairingInvitation{}, err
	}
	if len(secret) < 16 {
		return PairingInvitation{}, fmt.Errorf("%w: a pairing secret is at least 16 bytes", ErrInvalidInput)
	}
	if ttl <= 0 || ttl > 24*time.Hour {
		return PairingInvitation{}, fmt.Errorf("%w: a pairing invitation lives between a moment and a day", ErrInvalidInput)
	}
	invitation := PairingInvitation{
		ID: InvitationID(secret), Status: "open",
		ExpiresAt: time.Now().UTC().Add(ttl).Format(time.RFC3339), Label: boundedReason(label),
	}
	auditID, err := NewID("audit")
	if err != nil {
		return PairingInvitation{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return PairingInvitation{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_pairing_invitations(id, secret_sha256, status, expires_at, label)
		VALUES(?, ?, 'open', ?, ?)`, invitation.ID, invitationSecretHash(secret), invitation.ExpiresAt, invitation.Label); err != nil {
		return PairingInvitation{}, err
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'pairing.invited', ?, ?, ?)`, auditID, local.ReplicaID, invitation.ID,
		auditDetails(map[string]string{"expires_at": invitation.ExpiresAt})); err != nil {
		return PairingInvitation{}, err
	}
	return invitation, nil
}

// ConsumePairingInvitation spends an invitation exactly once.
//
// The single-use property is the transaction: the row moves to `consumed` in
// the same statement that checks it is open and unexpired, so two peers racing
// one code produce one winner and one refusal rather than two pairings.
func (s *SQLiteStore) ConsumePairingInvitation(ctx context.Context, secret []byte, consumerReplicaID string, now time.Time) (PairingInvitation, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return PairingInvitation{}, err
	}
	if len(secret) < 16 || consumerReplicaID == "" {
		return PairingInvitation{}, ErrPairingInvitation
	}
	id := InvitationID(secret)
	hash := invitationSecretHash(secret)
	stamp := now.UTC().Format(time.RFC3339)
	auditID, err := NewID("audit")
	if err != nil {
		return PairingInvitation{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return PairingInvitation{}, err
	}
	if err := s.execLocked("BEGIN IMMEDIATE"); err != nil {
		return PairingInvitation{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.execLocked("ROLLBACK")
		}
	}()
	if err := s.execPreparedLocked(`UPDATE sync_pairing_invitations
		SET status = 'consumed', consumed_at = ?, consumer_replica_id = ?
		WHERE id = ? AND secret_sha256 = ? AND status = 'open' AND expires_at > ?`,
		stamp, consumerReplicaID, id, hash, stamp); err != nil {
		return PairingInvitation{}, err
	}
	if int(C.sqlite3_changes(s.db)) != 1 {
		// Record the attempt: a refused pairing is exactly the event an
		// operator wants to see, and the reason is kept local rather than
		// returned to whoever presented the code.
		reason := "unknown_or_expired"
		if invitation, found, lookupErr := s.pairingInvitationLocked(id); lookupErr == nil && found {
			reason = invitation.Status
			if invitation.Status == "open" {
				reason = "expired"
			}
		}
		_ = s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
			VALUES(?, 'pairing.refused', ?, ?, ?)`, auditID, local.ReplicaID, id,
			auditDetails(map[string]string{"reason": reason}))
		if err := s.execLocked("COMMIT"); err != nil {
			return PairingInvitation{}, err
		}
		committed = true
		return PairingInvitation{}, ErrPairingInvitation
	}
	if err := s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, 'pairing.consumed', ?, ?, ?)`, auditID, local.ReplicaID, id,
		auditDetails(map[string]string{"consumer_replica_id": consumerReplicaID})); err != nil {
		return PairingInvitation{}, err
	}
	if err := s.execLocked("COMMIT"); err != nil {
		return PairingInvitation{}, err
	}
	committed = true
	invitation, _, err := s.pairingInvitationLocked(id)
	return invitation, err
}

// ListPairingInvitations reports invitations newest first, with their state.
func (s *SQLiteStore) ListPairingInvitations(ctx context.Context, limit int) ([]PairingInvitation, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT id, status, created_at, expires_at,
		COALESCE(consumed_at, ''), COALESCE(consumer_replica_id, ''), label
		FROM sync_pairing_invitations ORDER BY created_at DESC, id LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{fmt.Sprint(limit)}); err != nil {
		return nil, err
	}
	invitations := []PairingInvitation{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			invitations = append(invitations, PairingInvitation{
				ID: columnText(stmt, 0), Status: columnText(stmt, 1), CreatedAt: columnText(stmt, 2),
				ExpiresAt: columnText(stmt, 3), ConsumedAt: columnText(stmt, 4),
				ConsumerReplicaID: columnText(stmt, 5), Label: columnText(stmt, 6),
			})
		case C.SQLITE_DONE:
			return invitations, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

// RevokePairingInvitation closes an invitation that has not been used.
func (s *SQLiteStore) RevokePairingInvitation(ctx context.Context, id string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execPreparedLocked(`UPDATE sync_pairing_invitations SET status = 'revoked'
		WHERE id = ? AND status = 'open'`, id)
}

func (s *SQLiteStore) pairingInvitationLocked(id string) (PairingInvitation, bool, error) {
	stmt, err := s.prepareLocked(`SELECT id, status, created_at, expires_at,
		COALESCE(consumed_at, ''), COALESCE(consumer_replica_id, ''), label
		FROM sync_pairing_invitations WHERE id = ?`)
	if err != nil {
		return PairingInvitation{}, false, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{id}); err != nil {
		return PairingInvitation{}, false, err
	}
	switch rc := C.sqlite3_step(stmt); rc {
	case C.SQLITE_DONE:
		return PairingInvitation{}, false, nil
	case C.SQLITE_ROW:
		return PairingInvitation{
			ID: columnText(stmt, 0), Status: columnText(stmt, 1), CreatedAt: columnText(stmt, 2),
			ExpiresAt: columnText(stmt, 3), ConsumedAt: columnText(stmt, 4),
			ConsumerReplicaID: columnText(stmt, 5), Label: columnText(stmt, 6),
		}, true, nil
	default:
		return PairingInvitation{}, false, s.stepErrLocked(rc)
	}
}

// RecordSyncAuthEvent stores one authentication outcome.
//
// Reason codes are bounded and no credential, signature, body, or header value
// is ever recorded: an audit log that captures the thing it is auditing becomes
// the easiest place to steal it from.
func (s *SQLiteStore) RecordSyncAuthEvent(ctx context.Context, eventType, replicaID, reason string) error {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	auditID, err := NewID("audit")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	local, err := s.localSyncHandshakeLocked()
	if err != nil {
		return err
	}
	return s.execPreparedLocked(`INSERT INTO sync_audit_events(id, event_type, replica_id, subject_id, details_json)
		VALUES(?, ?, ?, ?, ?)`, auditID, boundedEventType(eventType), local.ReplicaID, replicaID,
		auditDetails(map[string]string{"reason": boundedReason(reason)}))
}

// ListSyncAuthEvents returns recent authentication and pairing events.
func (s *SQLiteStore) ListSyncAuthEvents(ctx context.Context, limit int) ([]map[string]string, error) {
	ctx = contextOrBackground(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stmt, err := s.prepareLocked(`SELECT id, event_type, COALESCE(subject_id, ''), details_json, created_at
		FROM sync_audit_events WHERE event_type LIKE 'peer.%' OR event_type LIKE 'pairing.%'
		ORDER BY created_at DESC, id DESC LIMIT ?`)
	if err != nil {
		return nil, err
	}
	defer C.sqlite3_finalize(stmt)
	if err := bindAll(stmt, []string{fmt.Sprint(limit)}); err != nil {
		return nil, err
	}
	events := []map[string]string{}
	for {
		switch rc := C.sqlite3_step(stmt); rc {
		case C.SQLITE_ROW:
			events = append(events, map[string]string{
				"id": columnText(stmt, 0), "event_type": columnText(stmt, 1),
				"subject_id": columnText(stmt, 2), "details": columnText(stmt, 3),
				"created_at": columnText(stmt, 4),
			})
		case C.SQLITE_DONE:
			return events, nil
		default:
			return nil, s.stepErrLocked(rc)
		}
	}
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func signerKeyID(public ed25519.PublicKey) string {
	digest := sha256.Sum256(append([]byte("notrios.signer.v1"), public...))
	return hex.EncodeToString(digest[:16])
}

// boundedReason keeps a free-text reason from becoming a place to store data.
func boundedReason(reason string) string {
	if len(reason) > 120 {
		return reason[:120]
	}
	return reason
}

func boundedEventType(eventType string) string {
	if eventType == "" {
		return "peer.auth_refused"
	}
	return boundedReason(eventType)
}

func auditDetails(values map[string]string) string {
	encoded := "{"
	first := true
	for _, key := range sortedStringKeys(values) {
		if !first {
			encoded += ","
		}
		first = false
		encoded += fmt.Sprintf("%q:%q", key, boundedReason(values[key]))
	}
	return encoded + "}"
}
