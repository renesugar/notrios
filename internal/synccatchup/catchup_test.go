package synccatchup

import (
	"errors"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

type permitted map[string]bool

func (p permitted) PermittedSource(replicaID string) bool { return p[replicaID] }

func fixture(t *testing.T) (*syncwire.MemorySigner, *syncwire.MemoryVerifier, string, time.Time) {
	t.Helper()
	signer, err := syncwire.NewMemorySigner()
	if err != nil {
		t.Fatal(err)
	}
	verifier := syncwire.NewMemoryVerifier()
	keyID := verifier.Enroll(signer.Public())
	return signer, verifier, keyID, time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
}

func offerFrom(t *testing.T, signer *syncwire.MemorySigner, keyID string, request Request, responder string, vector syncstate.Vector, now time.Time) Offer {
	t.Helper()
	response := Response{
		DatabaseID: request.DatabaseID, ResponderReplicaID: responder, RequestNonce: request.Nonce,
		ProtocolMajor: syncwire.ProtocolMajor, ProtocolMinor: syncwire.ProtocolMinor,
		SchemaVersion: 24, SnapshotID: "snap_" + responder, SnapshotVector: vector,
		ArchiveSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ArchiveLength: 4096, WrappingMode: request.WrappingMode,
		ExpiresAt: now.Add(DefaultTTL).Format(time.RFC3339Nano),
	}
	return Offer{Response: response, SignerKeyID: keyID, Signature: SignResponse(signer, response)}
}

func TestRequestSigningBindsEveryField(t *testing.T) {
	t.Parallel()
	signer, verifier, keyID, now := fixture(t)
	request, err := NewRequest("db_one", "replica-new", 24, WrapPeerKey, syncstate.Vector{"replica-a": 3}, now)
	if err != nil {
		t.Fatal(err)
	}
	signature := SignRequest(signer, request)
	if err := VerifyRequest(verifier, keyID, signature, request); err != nil {
		t.Fatalf("a genuine request did not verify: %v", err)
	}
	for name, mutate := range map[string]func(Request) Request{
		"database":      func(r Request) Request { r.DatabaseID = "db_two"; return r },
		"requester":     func(r Request) Request { r.RequesterReplicaID = "replica-other"; return r },
		"wrapping-mode": func(r Request) Request { r.WrappingMode = WrapPassword; return r },
		"nonce":         func(r Request) Request { r.Nonce = "00"; return r },
		"schema":        func(r Request) Request { r.SchemaVersion = 23; return r },
		"known-vector":  func(r Request) Request { r.KnownVector = syncstate.Vector{"replica-a": 4}; return r },
	} {
		if err := VerifyRequest(verifier, keyID, signature, mutate(request)); !errors.Is(err, syncwire.ErrBadSignature) {
			t.Fatalf("the %s field is not bound by the signature", name)
		}
	}
	if err := VerifyRequest(verifier, "unknown", signature, request); !errors.Is(err, syncwire.ErrUnknownSigner) {
		t.Fatal("an unenrolled requester was accepted")
	}
	// Two requests must be distinguishable even in the same second.
	other, _ := NewRequest("db_one", "replica-new", 24, WrapPeerKey, nil, now)
	if other.Nonce == request.Nonce {
		t.Fatal("two requests shared a nonce")
	}
}

func TestOnlyPermittedSourcesMayAnswer(t *testing.T) {
	t.Parallel()
	signer, verifier, keyID, now := fixture(t)
	request, _ := NewRequest("db_one", "replica-new", 24, WrapPeerKey, nil, now)
	offer := offerFrom(t, signer, keyID, request, "replica-a", syncstate.Vector{"replica-a": 10}, now)

	// Enrollment is not permission. An enrolled peer that has not been allowed
	// as a snapshot source is refused before its signature is even considered.
	if err := Usable(request, offer, permitted{}, verifier, 24, now); !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("an unpermitted responder was accepted: %v", err)
	}
	if err := Usable(request, offer, permitted{"replica-a": true}, verifier, 24, now); err != nil {
		t.Fatalf("a permitted responder was refused: %v", err)
	}
}

func TestEveryReasonAnOfferIsUnusable(t *testing.T) {
	t.Parallel()
	signer, verifier, keyID, now := fixture(t)
	request, _ := NewRequest("db_one", "replica-new", 24, WrapPeerKey, nil, now)
	policy := permitted{"replica-a": true, "replica-new": true}
	good := offerFrom(t, signer, keyID, request, "replica-a", syncstate.Vector{"replica-a": 10}, now)

	cases := map[string]struct {
		mutate func(Offer) Offer
		want   error
	}{
		"different-database": {func(o Offer) Offer {
			o.Response.DatabaseID = "db_two"
			o.Signature = SignResponse(signer, o.Response)
			return o
		}, ErrIncompatible},
		"answers-another-request": {func(o Offer) Offer {
			o.Response.RequestNonce = "deadbeef"
			o.Signature = SignResponse(signer, o.Response)
			return o
		}, ErrIncompatible},
		"catching-up-from-itself": {func(o Offer) Offer {
			o.Response.ResponderReplicaID = request.RequesterReplicaID
			o.Signature = SignResponse(signer, o.Response)
			return o
		}, ErrIncompatible},
		"schema-mismatch": {func(o Offer) Offer {
			o.Response.SchemaVersion = 23
			o.Signature = SignResponse(signer, o.Response)
			return o
		}, ErrIncompatible},
		"wrapping-mode-mismatch": {func(o Offer) Offer {
			o.Response.WrappingMode = WrapPassword
			o.Signature = SignResponse(signer, o.Response)
			return o
		}, ErrIncompatible},
		"no-verifiable-archive": {func(o Offer) Offer {
			o.Response.ArchiveSHA256 = "short"
			o.Signature = SignResponse(signer, o.Response)
			return o
		}, ErrIncompatible},
		"expired": {func(o Offer) Offer {
			o.Response.ExpiresAt = now.Add(-time.Minute).Format(time.RFC3339Nano)
			o.Signature = SignResponse(signer, o.Response)
			return o
		}, ErrExpired},
		"tampered-after-signing": {func(o Offer) Offer {
			o.Response.ArchiveLength = 999999
			return o
		}, syncwire.ErrBadSignature},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Usable(request, test.mutate(good), policy, verifier, 24, now); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
	if err := Usable(request, good, policy, verifier, 24, now); err != nil {
		t.Fatalf("the unmutated fixture should be usable: %v", err)
	}
}

// Several peers answering is normal. Taking one of them, rather than combining
// them, is the decision G10 recorded: two snapshots are two different points in
// the library's history.
func TestCompetingOffersSelectOneDeterministically(t *testing.T) {
	t.Parallel()
	signer, verifier, keyID, now := fixture(t)
	request, _ := NewRequest("db_one", "replica-new", 24, WrapPeerKey, nil, now)
	policy := permitted{"replica-a": true, "replica-b": true, "replica-c": true}

	behind := offerFrom(t, signer, keyID, request, "replica-a", syncstate.Vector{"replica-a": 5, "replica-b": 1}, now)
	ahead := offerFrom(t, signer, keyID, request, "replica-b", syncstate.Vector{"replica-a": 9, "replica-b": 4}, now)
	unpermitted := offerFrom(t, signer, keyID, request, "replica-z", syncstate.Vector{"replica-a": 100}, now)
	expired := offerFrom(t, signer, keyID, request, "replica-c", syncstate.Vector{"replica-a": 500}, now.Add(-2*DefaultTTL))

	chosen, err := Select(request, []Offer{behind, ahead, unpermitted, expired}, policy, verifier, 24, now)
	if err != nil {
		t.Fatal(err)
	}
	if chosen.Response.ResponderReplicaID != "replica-b" {
		t.Fatalf("chose %s; the furthest-ahead usable offer is replica-b", chosen.Response.ResponderReplicaID)
	}
	// The order offers arrive in must not change the choice.
	reordered, err := Select(request, []Offer{expired, unpermitted, ahead, behind}, policy, verifier, 24, now)
	if err != nil || reordered.Response.ResponderReplicaID != chosen.Response.ResponderReplicaID {
		t.Fatalf("selection depends on arrival order: %v", err)
	}
	// A tie is broken by responder id, so two requesters choose alike.
	tieA := offerFrom(t, signer, keyID, request, "replica-b", syncstate.Vector{"replica-a": 7}, now)
	tieB := offerFrom(t, signer, keyID, request, "replica-a", syncstate.Vector{"replica-a": 7}, now)
	tied, err := Select(request, []Offer{tieA, tieB}, policy, verifier, 24, now)
	if err != nil || tied.Response.ResponderReplicaID != "replica-a" {
		t.Fatalf("tie-break = %s", tied.Response.ResponderReplicaID)
	}
	if _, err := Select(request, []Offer{unpermitted, expired}, policy, verifier, 24, now); !errors.Is(err, ErrNoUsableResponse) {
		t.Fatal("nothing usable should be reported as such")
	}
}

func TestStateMachineAllowsOnlyRealMoves(t *testing.T) {
	t.Parallel()
	happy := []State{StateRequested, StateOffered, StateTransferring, StateVerifying, StateRestoring, StateCutover, StateComplete}
	for index := 0; index+1 < len(happy); index++ {
		if err := Transition(happy[index], happy[index+1]); err != nil {
			t.Fatalf("%s -> %s: %v", happy[index], happy[index+1], err)
		}
	}
	// A resumed transfer is progress within the same state.
	if err := Transition(StateTransferring, StateTransferring); err != nil {
		t.Fatalf("resume: %v", err)
	}
	// Verification failing sends the transfer round again; restoring does not.
	if err := Transition(StateVerifying, StateTransferring); err != nil {
		t.Fatalf("re-fetch after a failed verify: %v", err)
	}
	if err := Transition(StateRestoring, StateTransferring); !errors.Is(err, ErrBadTransition) {
		t.Fatal("a restore in progress must not be able to re-fetch underneath itself")
	}
	if err := Transition(StateRestoring, StateCancelled); !errors.Is(err, ErrBadTransition) {
		t.Fatal("a restore that has begun writing rows cannot simply be cancelled")
	}
	for _, terminal := range []State{StateComplete, StateCancelled, StateExpired, StateFailed} {
		if !Terminal(terminal) {
			t.Fatalf("%s should be terminal", terminal)
		}
		if err := Transition(terminal, StateRequested); !errors.Is(err, ErrBadTransition) {
			t.Fatalf("%s was restartable", terminal)
		}
	}
	if err := Transition("invented", StateComplete); !errors.Is(err, ErrBadTransition) {
		t.Fatal("an unknown state was accepted")
	}
}

func TestPasswordWrappingIsMemoryHardAndDistinguishesAWrongPassword(t *testing.T) {
	t.Parallel()
	payloadKey := make([]byte, syncwire.GroupKeyBytes)
	for index := range payloadKey {
		payloadKey[index] = byte(index)
	}
	salt, wrapped, err := WrapWithPassword("correct horse battery staple", payloadKey)
	if err != nil {
		t.Fatal(err)
	}
	// The key must not be recoverable from the wrapped form.
	if string(wrapped[:len(payloadKey)]) == string(payloadKey) {
		t.Fatal("the wrapped key is the key")
	}
	unwrapped, err := UnwrapWithPassword("correct horse battery staple", salt, wrapped)
	if err != nil || string(unwrapped) != string(payloadKey) {
		t.Fatalf("round trip: %v", err)
	}
	// A wrong password is reported as a wrong password, not as a corrupt
	// archive: the two lead a person to do completely different things.
	if _, err := UnwrapWithPassword("wrong password", salt, wrapped); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := UnwrapWithPassword("correct horse battery staple", salt[:4], wrapped); !errors.Is(err, ErrInvalid) {
		t.Fatal("a malformed salt was accepted")
	}
	if _, _, err := WrapWithPassword("", payloadKey); !errors.Is(err, ErrInvalid) {
		t.Fatal("an empty password was accepted")
	}
	// Two wraps of the same key differ, so an observer cannot tell that two
	// archives protect the same payload.
	otherSalt, otherWrapped, err := WrapWithPassword("correct horse battery staple", payloadKey)
	if err != nil {
		t.Fatal(err)
	}
	if string(salt) == string(otherSalt) || string(wrapped) == string(otherWrapped) {
		t.Fatal("two wraps of one key were identical")
	}
}

// The manifest's state vector is what turns a restore into a catch-up: without
// it a restored replica would have to ask for everything again.
func TestRemainingWorkAfterASnapshot(t *testing.T) {
	t.Parallel()
	snapshot := syncstate.Vector{"replica-a": 100, "replica-b": 40}
	source := syncstate.Vector{"replica-a": 137, "replica-b": 40, "replica-c": 9}
	plan, err := RemainingWork(snapshot, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ranges) != 2 {
		t.Fatalf("ranges = %+v", plan.Ranges)
	}
	for _, want := range []syncstate.SequenceRange{
		{ReplicaID: "replica-a", Start: 101, End: 137},
		{ReplicaID: "replica-c", Start: 1, End: 9},
	} {
		found := false
		for _, got := range plan.Ranges {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing range %+v in %+v", want, plan.Ranges)
		}
	}
	// A snapshot already level with the source leaves nothing to fetch.
	empty, err := RemainingWork(source, source)
	if err != nil || len(empty.Ranges) != 0 {
		t.Fatalf("a current snapshot should leave no work: %+v %v", empty.Ranges, err)
	}
}
