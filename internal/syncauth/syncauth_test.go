package syncauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func testKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return public, private
}

func testRequest(nonce string) Request {
	return Request{
		Method: "GET", Path: "/api/v1/sync/handshake", DatabaseID: "db_test", ReplicaID: "rep_a",
		Timestamp: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC), Nonce: nonce, BodySHA256: BodyDigest(nil),
	}
}

func randomNonce(t *testing.T) string {
	t.Helper()
	nonce := make([]byte, NonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(nonce)
}

// TestEveryFieldIsBound is the test that says what a signature means. Changing
// any field the receiver checks must invalidate it; a field that can be changed
// freely is a field an attacker chooses.
func TestEveryFieldIsBound(t *testing.T) {
	public, private := testKeyPair(t)
	base := testRequest(randomNonce(t))
	signature := ed25519.Sign(private, SigningString(base))

	for name, mutate := range map[string]func(Request) Request{
		"method":    func(r Request) Request { r.Method = "POST"; return r },
		"path":      func(r Request) Request { r.Path = "/api/v1/sync/status"; return r },
		"database":  func(r Request) Request { r.DatabaseID = "db_other"; return r },
		"replica":   func(r Request) Request { r.ReplicaID = "rep_b"; return r },
		"timestamp": func(r Request) Request { r.Timestamp = r.Timestamp.Add(time.Second); return r },
		"nonce":     func(r Request) Request { r.Nonce = strings.Repeat("00", NonceBytes); return r },
		"body":      func(r Request) Request { r.BodySHA256 = BodyDigest([]byte("something else")); return r },
	} {
		if ed25519.Verify(public, SigningString(mutate(base)), signature) {
			t.Fatalf("a signature survived changing the %s", name)
		}
	}
	if !ed25519.Verify(public, SigningString(base), signature) {
		t.Fatal("the unmodified request does not verify")
	}
}

func TestVerifierAcceptsThenRefusesAReplay(t *testing.T) {
	public, private := testKeyPair(t)
	now := time.Now().UTC()
	verifier := NewVerifier("db_test", func(string) (string, ed25519.PublicKey, bool) {
		return "rep_a", public, true
	}, NewLimiter(DefaultLimits()), func() time.Time { return now })

	request := testRequest(randomNonce(t))
	request.Timestamp = now
	header, err := Sign(private, "key_a", request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(header, request.Method, request.Path, request.BodySHA256); err != nil {
		t.Fatalf("a valid credential was refused: %v", err)
	}
	if _, err := verifier.Verify(header, request.Method, request.Path, request.BodySHA256); err == nil {
		t.Fatal("a replayed credential was accepted")
	}
}

func TestVerifierRefusesAKeyEnrolledForAnotherReplica(t *testing.T) {
	public, private := testKeyPair(t)
	now := time.Now().UTC()
	verifier := NewVerifier("db_test", func(string) (string, ed25519.PublicKey, bool) {
		// The key is enrolled, but for a different replica than the credential
		// claims to be. Without this check a peer could speak as another.
		return "rep_somebody_else", public, true
	}, NewLimiter(DefaultLimits()), func() time.Time { return now })
	request := testRequest(randomNonce(t))
	request.Timestamp = now
	header, err := Sign(private, "key_a", request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(header, request.Method, request.Path, request.BodySHA256); err == nil {
		t.Fatal("a key enrolled for another replica authenticated this one")
	}
}

func TestClockSkewWindowIsSymmetric(t *testing.T) {
	public, private := testKeyPair(t)
	now := time.Now().UTC()
	verifier := NewVerifier("db_test", func(string) (string, ed25519.PublicKey, bool) {
		return "rep_a", public, true
	}, NewLimiter(DefaultLimits()), func() time.Time { return now })
	for name, offset := range map[string]time.Duration{
		"far in the past":   -2 * MaxSkew,
		"far in the future": 2 * MaxSkew,
	} {
		request := testRequest(randomNonce(t))
		request.Timestamp = now.Add(offset)
		header, err := Sign(private, "key_a", request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := verifier.Verify(header, request.Method, request.Path, request.BodySHA256); err == nil {
			t.Fatalf("a timestamp %s was accepted", name)
		}
	}
}

func TestPairingCodeSurvivesBeingRetyped(t *testing.T) {
	secret, err := NewPairingSecret()
	if err != nil {
		t.Fatal(err)
	}
	code := FormatCode(secret)
	if !strings.Contains(code, "-") {
		t.Fatalf("the code is not grouped for reading: %q", code)
	}
	for _, variant := range []string{
		code,
		strings.ToLower(code),
		strings.ReplaceAll(code, "-", ""),
		"  " + code + "\n",
		strings.ReplaceAll(code, "-", " "),
	} {
		parsed, err := ParseCode(variant)
		if err != nil {
			t.Fatalf("ParseCode(%q): %v", variant, err)
		}
		if string(parsed) != string(secret) {
			t.Fatalf("ParseCode(%q) produced different bytes", variant)
		}
	}
	if _, err := ParseCode("not a code"); err == nil {
		t.Fatal("a nonsense code parsed")
	}
}

func TestTheWrappedGroupKeyNeedsTheCode(t *testing.T) {
	secret, err := NewPairingSecret()
	if err != nil {
		t.Fatal(err)
	}
	group := make([]byte, 32)
	if _, err := rand.Read(group); err != nil {
		t.Fatal(err)
	}
	wrapped, err := WrapGroupKey(secret, group)
	if err != nil {
		t.Fatal(err)
	}
	// The whole point: the file half is useless without the spoken half.
	if strings.Contains(wrapped, string(group)) {
		t.Fatal("the wrapped key contains the key")
	}
	opened, err := UnwrapGroupKey(secret, wrapped)
	if err != nil || string(opened) != string(group) {
		t.Fatalf("the code did not open its own wrapping: %v", err)
	}
	other, err := NewPairingSecret()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UnwrapGroupKey(other, wrapped); err == nil {
		t.Fatal("a different code opened the wrapping")
	}
	// And a single altered byte is refused rather than producing rubbish.
	altered := []byte(wrapped)
	altered[len(altered)-2] ^= 0x01
	if _, err := UnwrapGroupKey(secret, string(altered)); err == nil {
		t.Fatal("an altered wrapping opened")
	}
}

func TestPairingProofsBindWhatTheyClaim(t *testing.T) {
	secret, err := NewPairingSecret()
	if err != nil {
		t.Fatal(err)
	}
	request := PairingRequest{
		InvitationID: "inv_1", DatabaseID: "db_test", ReplicaID: "rep_joiner", PublicKey: "AAAA",
	}
	if request.Proof, err = ProveRequest(secret, request); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRequestProof(secret, request); err != nil {
		t.Fatalf("a correct proof was refused: %v", err)
	}
	for name, mutate := range map[string]func(PairingRequest) PairingRequest{
		"replica":    func(r PairingRequest) PairingRequest { r.ReplicaID = "rep_impostor"; return r },
		"public key": func(r PairingRequest) PairingRequest { r.PublicKey = "BBBB"; return r },
		"database":   func(r PairingRequest) PairingRequest { r.DatabaseID = "db_other"; return r },
		"invitation": func(r PairingRequest) PairingRequest { r.InvitationID = "inv_2"; return r },
	} {
		if err := VerifyRequestProof(secret, mutate(request)); err == nil {
			t.Fatalf("the proof survived changing the %s", name)
		}
	}
}

func TestFailureBudgetSlowsGuessing(t *testing.T) {
	limiter := NewLimiter(Limits{RequestsPerMinute: 60, Burst: 5, FailuresPerMinute: 5})
	now := time.Now()
	allowed := 0
	for attempt := 0; attempt < 20; attempt++ {
		if limiter.AllowFailure("198.51.100.1", now) {
			allowed++
		}
	}
	if allowed > 5 {
		t.Fatalf("%d guesses allowed against a budget of 5", allowed)
	}
	// A different address has its own budget: one noisy peer must not lock out
	// everyone else.
	if !limiter.AllowFailure("198.51.100.2", now) {
		t.Fatal("one address's failures exhausted another's budget")
	}
	// And the budget refills with time rather than being spent forever.
	if !limiter.AllowFailure("198.51.100.1", now.Add(time.Minute)) {
		t.Fatal("the failure budget never refilled")
	}
}

func TestReasonVocabularyIsClosed(t *testing.T) {
	allowed := map[string]bool{}
	for _, reason := range Reasons() {
		allowed[reason] = true
	}
	for _, err := range []error{
		nil, ErrMalformed, ErrUnknownKey, ErrBadSignature, ErrStale, ErrReplay,
		ErrWrongDatabase, ErrWrongPrincipal, ErrRateLimited, ErrPairingProof, ErrPairingBundle,
	} {
		if !allowed[Reason(err)] {
			t.Fatalf("Reason(%v) = %q, which is outside the vocabulary", err, Reason(err))
		}
	}
}

func TestClientRefusesPlaintextToARemoteHost(t *testing.T) {
	_, private := testKeyPair(t)
	client := &Client{
		BaseURL: "http://notes.example:8080", DatabaseID: "db_test",
		ReplicaID: "rep_a", SignerKeyID: "key_a", Private: private,
	}
	if _, _, err := client.Do(nil, "GET", "/api/v1/sync/handshake", nil); err == nil {
		t.Fatal("the client signed a request over plaintext to a remote host")
	}
	// Loopback plaintext stays usable, because that is how the surface is
	// tested and the traffic never leaves the machine.
	loopback := &Client{
		BaseURL: "http://127.0.0.1:9/", DatabaseID: "db_test",
		ReplicaID: "rep_a", SignerKeyID: "key_a", Private: private,
	}
	if _, _, err := loopback.Do(nil, "GET", "/api/v1/sync/handshake", nil); err == nil {
		t.Fatal("expected a connection error, not a policy refusal")
	} else if strings.Contains(err.Error(), "plaintext") {
		t.Fatalf("loopback was refused by policy: %v", err)
	}
}
