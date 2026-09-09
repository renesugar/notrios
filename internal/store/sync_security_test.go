package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"
)

func newSecurityStore(t *testing.T) *SQLiteStore {
	t.Helper()
	st := newSyncJournalTestStore(t)
	if _, err := st.EnrollLocalJournal(context.Background(), "G13 security"); err != nil {
		t.Fatal(err)
	}
	return st
}

func newPublicKey(t *testing.T) ed25519.PublicKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return public
}

func TestPeerKeyEnrolmentIsIdempotentAndBoundToOneReplica(t *testing.T) {
	ctx := context.Background()
	st := newSecurityStore(t)
	public := newPublicKey(t)

	first, err := st.EnrollPeerSigningKey(ctx, "rep_a", public, "test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.EnrollPeerSigningKey(ctx, "rep_a", public, "test")
	if err != nil || first != second {
		t.Fatalf("re-enrolling the same key was not idempotent: %v %q %q", err, first, second)
	}
	// The same key claiming to be a second replica is the interesting case: one
	// device, two identities, would let it speak as both.
	if _, err := st.EnrollPeerSigningKey(ctx, "rep_b", public, "test"); !errors.Is(err, ErrConflict) {
		t.Fatalf("one key was enrolled for two replicas: %v", err)
	}
}

func TestRevokedKeysAreUnknownToTheVerifierAndCannotReturn(t *testing.T) {
	ctx := context.Background()
	st := newSecurityStore(t)
	public := newPublicKey(t)
	keyID, err := st.EnrollPeerSigningKey(ctx, "rep_a", public, "test")
	if err != nil {
		t.Fatal(err)
	}
	verifier := st.PeerVerifier()
	if _, found := verifier.PublicKey(keyID); !found {
		t.Fatal("an enrolled key is not visible to the verifier")
	}
	if err := st.RevokePeerSigningKey(ctx, keyID, "lost laptop"); err != nil {
		t.Fatal(err)
	}
	// "We no longer trust this" and "we never knew this" are the same answer.
	if _, found := verifier.PublicKey(keyID); found {
		t.Fatal("a revoked key still verifies")
	}
	// And a revoked key cannot be quietly re-enrolled: that decision needs a
	// new key, so the old one stays evidence.
	if _, err := st.EnrollPeerSigningKey(ctx, "rep_a", public, "test"); !errors.Is(err, ErrConflict) {
		t.Fatalf("a revoked key was re-enrolled: %v", err)
	}
	keys, err := st.ListPeerSigningKeys(ctx)
	if err != nil || len(keys) != 1 || keys[0].Status != "revoked" || keys[0].RevokedAt == "" {
		t.Fatalf("the revocation is not visible in the listing: %v %+v", err, keys)
	}
}

func TestAnInvitationIsSpentExactlyOnceUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	st := newSecurityStore(t)
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePairingInvitation(ctx, secret, time.Hour, "race"); err != nil {
		t.Fatal(err)
	}

	// Eight callers race one code. Single use is a transaction, not a
	// convention, so exactly one may win.
	var group sync.WaitGroup
	results := make([]error, 8)
	for index := range results {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			_, results[index] = st.ConsumePairingInvitation(ctx, secret, "rep_joiner", time.Now())
		}(index)
	}
	group.Wait()
	winners := 0
	for _, err := range results {
		if err == nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("%d callers consumed one invitation", winners)
	}
	invitations, err := st.ListPairingInvitations(ctx, 10)
	if err != nil || len(invitations) != 1 || invitations[0].Status != "consumed" {
		t.Fatalf("the invitation is not recorded as consumed: %v %+v", err, invitations)
	}
}

func TestAnExpiredOrRevokedInvitationIsRefused(t *testing.T) {
	ctx := context.Background()
	st := newSecurityStore(t)
	expiring := make([]byte, 20)
	if _, err := rand.Read(expiring); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePairingInvitation(ctx, expiring, time.Minute, "short"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ConsumePairingInvitation(ctx, expiring, "rep_late", time.Now().Add(time.Hour)); !errors.Is(err, ErrPairingInvitation) {
		t.Fatalf("an expired invitation was accepted: %v", err)
	}

	revoked := make([]byte, 20)
	if _, err := rand.Read(revoked); err != nil {
		t.Fatal(err)
	}
	invitation, err := st.CreatePairingInvitation(ctx, revoked, time.Hour, "cancelled")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RevokePairingInvitation(ctx, invitation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ConsumePairingInvitation(ctx, revoked, "rep_late", time.Now()); !errors.Is(err, ErrPairingInvitation) {
		t.Fatalf("a revoked invitation was accepted: %v", err)
	}
	// Every refusal is recorded, because a refused pairing is exactly what an
	// operator wants to see.
	events, err := st.ListSyncAuthEvents(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	refusals := 0
	for _, event := range events {
		if event["event_type"] == "pairing.refused" {
			refusals++
		}
	}
	if refusals < 2 {
		t.Fatalf("refused pairings were not audited: %v", events)
	}
}

func TestTheInvitationSecretIsNeverStored(t *testing.T) {
	ctx := context.Background()
	st := newSecurityStore(t)
	secret := []byte("0123456789abcdefghij")
	if _, err := st.CreatePairingInvitation(ctx, secret, time.Hour, "storage"); err != nil {
		t.Fatal(err)
	}
	// A stolen database must not yield a usable code.
	stored, err := st.syncJournalTextForTest(`SELECT COALESCE(GROUP_CONCAT(secret_sha256 || id || label), '')
		FROM sync_pairing_invitations`)
	if err != nil {
		t.Fatal(err)
	}
	if stored == "" {
		t.Fatal("nothing was stored at all")
	}
	if contains(stored, string(secret)) {
		t.Fatal("the invitation secret is in the database")
	}
}

func TestSchemaV25SecuritySurfaceSurvivesSchemaV27(t *testing.T) {
	ctx := context.Background()
	st := newSecurityStore(t)
	version, err := st.syncJournalTextForTest(`SELECT CAST((SELECT * FROM pragma_user_version) AS TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	if version != "27" {
		t.Fatalf("schema version = %s, want 27", version)
	}
	for _, table := range []string{"sync_peer_keys", "sync_pairing_invitations"} {
		found, err := st.syncJournalTextForTest(
			`SELECT COALESCE((SELECT name FROM sqlite_master WHERE type='table' AND name = ?), '')`, table)
		if err != nil || found != table {
			t.Fatalf("table %s missing after upgrade: %v %q", table, err, found)
		}
	}
	// The tables the earlier slices added are still there: a later migration
	// must not shadow an earlier one by claiming its version.
	for _, table := range []string{"sync_catchup_sessions", "sync_catchup_floors", "sync_replicas"} {
		found, err := st.syncJournalTextForTest(
			`SELECT COALESCE((SELECT name FROM sqlite_master WHERE type='table' AND name = ?), '')`, table)
		if err != nil || found != table {
			t.Fatalf("table %s disappeared: %v %q", table, err, found)
		}
	}
	_ = ctx
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for index := 0; index+len(needle) <= len(haystack); index++ {
		if haystack[index:index+len(needle)] == needle {
			return index
		}
	}
	return -1
}
