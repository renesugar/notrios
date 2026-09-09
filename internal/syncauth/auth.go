// Package syncauth authenticates one Notrios replica to another over HTTP.
//
// It is v0.7 G13's answer to a question the project has deferred since v0.1:
// what does it mean for a request to Notrios to be authorized? The answer here
// is deliberately narrow. A peer principal is one enrolled replica of one
// database, proved by an Ed25519 signature over the request it is making, and
// it authorizes exactly one thing: the sync surface of that database. It is not
// a login, it grants nothing on any ordinary note route, and it creates no
// concept of a user.
//
// Why signatures rather than a bearer token: a token is a reusable secret that
// travels on every request, so anything that logs, proxies, caches, or
// mis-terminates TLS holds a credential. A signature over the method, path,
// body, timestamp, and nonce is useless once the request it covers is spent,
// and the private key never leaves the replica that owns it.
package syncauth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Scheme is the authorization scheme name.
const Scheme = "Notrios-Sync-v1"

// Domain separates these signatures from every other thing this project signs.
const Domain = "notrios.rest-auth.v1"

// Bounds. Each is checked before the work it guards, and each is a promise
// about what an unauthenticated caller can make this process do.
const (
	// MaxHeaderBytes bounds the credential itself.
	MaxHeaderBytes = 1024
	// MaxSkew is how far a peer's clock may differ from this one. Two minutes
	// is generous for NTP-synchronized machines and short enough that a
	// captured request is stale before it is useful.
	MaxSkew = 2 * time.Minute
	// NonceBytes is the size of the per-request uniqueness value.
	NonceBytes = 16
	// MaxNoncesPerPeer bounds the replay cache. It is deliberately larger than
	// the rate limit allows within one skew window, so an entry can only be
	// evicted after the timestamp window has already made it unusable.
	MaxNoncesPerPeer = 4096
	// MaxTrackedPeers bounds the whole cache, so an attacker inventing replica
	// identities cannot grow it without bound.
	MaxTrackedPeers = 256
)

var (
	// ErrMalformed reports a credential that is not well-formed.
	ErrMalformed = errors.New("malformed sync authorization")
	// ErrUnknownKey reports a signing key this replica has not enrolled, or has
	// revoked. The two are the same answer on purpose.
	ErrUnknownKey = errors.New("sync authorization names an unenrolled key")
	// ErrBadSignature reports a signature that does not verify.
	ErrBadSignature = errors.New("sync authorization signature does not verify")
	// ErrStale reports a timestamp outside the accepted window.
	ErrStale = errors.New("sync authorization is outside the accepted time window")
	// ErrReplay reports a nonce already seen inside the window.
	ErrReplay = errors.New("sync authorization has already been used")
	// ErrWrongDatabase reports a credential for a different library.
	ErrWrongDatabase = errors.New("sync authorization is for a different database")
	// ErrWrongPrincipal reports a key enrolled for a different replica than the
	// one the request claims to be.
	ErrWrongPrincipal = errors.New("sync authorization key belongs to another replica")
	// ErrRateLimited reports a peer asking for more than its share.
	ErrRateLimited = errors.New("sync authorization is rate limited")
)

// Credential is a parsed authorization header.
type Credential struct {
	ReplicaID   string
	SignerKeyID string
	Timestamp   time.Time
	Nonce       string
	Signature   []byte
}

// Request is what a signature commits to. Everything in it is either something
// the receiver can check independently or something whose absence would let a
// signed request be replayed somewhere it was not meant for.
type Request struct {
	Method     string
	Path       string
	DatabaseID string
	ReplicaID  string
	Timestamp  time.Time
	Nonce      string
	BodySHA256 string
}

// SigningString is the exact byte string a signature covers.
//
// The method and path are in it so a signed read cannot be replayed as a write
// or against another route. The body hash is in it so the body cannot be
// swapped. The database id is in it so a peer of one library cannot present the
// same request to another. The timestamp and nonce are in it so the whole thing
// expires and cannot be used twice.
func SigningString(request Request) []byte {
	return []byte(strings.Join([]string{
		Domain,
		strings.ToUpper(request.Method),
		request.Path,
		request.DatabaseID,
		request.ReplicaID,
		request.Timestamp.UTC().Format(time.RFC3339Nano),
		request.Nonce,
		request.BodySHA256,
	}, "\n"))
}

// BodyDigest is the hash a request commits to. An empty body has a hash too:
// omitting it would let a body be added to a signed empty request.
func BodyDigest(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

// Sign produces the authorization header value for a request.
func Sign(signer ed25519.PrivateKey, signerKeyID string, request Request) (string, error) {
	if len(signer) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("%w: not an Ed25519 private key", ErrMalformed)
	}
	if request.ReplicaID == "" || request.DatabaseID == "" || request.Nonce == "" || signerKeyID == "" {
		return "", fmt.Errorf("%w: a signed request names its database, replica, key, and nonce", ErrMalformed)
	}
	signature := ed25519.Sign(signer, SigningString(request))
	header := fmt.Sprintf(`%s replica="%s", key="%s", ts="%s", nonce="%s", sig="%s"`,
		Scheme, request.ReplicaID, signerKeyID,
		request.Timestamp.UTC().Format(time.RFC3339Nano), request.Nonce,
		base64.StdEncoding.EncodeToString(signature))
	if len(header) > MaxHeaderBytes {
		return "", fmt.Errorf("%w: credential is %d bytes", ErrMalformed, len(header))
	}
	return header, nil
}

// ParseCredential reads an authorization header without trusting any of it.
func ParseCredential(header string) (Credential, error) {
	if len(header) > MaxHeaderBytes {
		return Credential{}, fmt.Errorf("%w: %d bytes", ErrMalformed, len(header))
	}
	rest, found := strings.CutPrefix(header, Scheme+" ")
	if !found {
		return Credential{}, fmt.Errorf("%w: not a %s credential", ErrMalformed, Scheme)
	}
	fields := map[string]string{}
	for _, part := range strings.Split(rest, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return Credential{}, fmt.Errorf("%w: unparsable field", ErrMalformed)
		}
		fields[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	credential := Credential{
		ReplicaID: fields["replica"], SignerKeyID: fields["key"], Nonce: fields["nonce"],
	}
	if credential.ReplicaID == "" || credential.SignerKeyID == "" || credential.Nonce == "" {
		return Credential{}, fmt.Errorf("%w: a credential names its replica, key, and nonce", ErrMalformed)
	}
	if len(credential.Nonce) != 2*NonceBytes || !lowercaseHex(credential.Nonce) {
		return Credential{}, fmt.Errorf("%w: nonce must be %d lowercase hex characters", ErrMalformed, 2*NonceBytes)
	}
	timestamp, err := time.Parse(time.RFC3339Nano, fields["ts"])
	if err != nil {
		return Credential{}, fmt.Errorf("%w: unparsable timestamp", ErrMalformed)
	}
	credential.Timestamp = timestamp
	signature, err := base64.StdEncoding.DecodeString(fields["sig"])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return Credential{}, fmt.Errorf("%w: signature is not %d bytes", ErrMalformed, ed25519.SignatureSize)
	}
	credential.Signature = signature
	return credential, nil
}

// KeyLookup resolves a signer key id to the replica it is enrolled for and its
// public key. A revoked key must report found=false: "we no longer trust this"
// and "we never knew this" are the same answer to a caller.
type KeyLookup func(signerKeyID string) (replicaID string, public ed25519.PublicKey, found bool)

// Verifier checks credentials against enrolled keys, a clock, and a replay
// cache. One instance serves one database.
type Verifier struct {
	databaseID string
	lookup     KeyLookup
	now        func() time.Time

	mu     sync.Mutex
	nonces map[string]map[string]time.Time
	limits *Limiter
}

// NewVerifier builds a verifier for one database.
func NewVerifier(databaseID string, lookup KeyLookup, limits *Limiter, now func() time.Time) *Verifier {
	if now == nil {
		now = time.Now
	}
	if limits == nil {
		limits = NewLimiter(DefaultLimits())
	}
	return &Verifier{
		databaseID: databaseID, lookup: lookup, now: now,
		nonces: map[string]map[string]time.Time{}, limits: limits,
	}
}

// Principal is an authenticated peer. It carries no capabilities beyond being
// this replica of this database: authorization is decided by which routes the
// middleware is attached to, not by a field here that could be widened later.
type Principal struct {
	ReplicaID   string
	SignerKeyID string
}

// Verify authenticates one request and returns its principal.
//
// The order matters and is not an accident: cheap structural checks first,
// then the clock, then the rate limit, then the signature, and only then the
// replay cache. Signature verification is the expensive step, so it happens
// after the checks that can refuse without it; the replay cache is recorded
// last so a failed signature cannot consume a nonce and lock out the real peer.
func (v *Verifier) Verify(header string, method, path, bodyDigest string) (Principal, error) {
	credential, err := ParseCredential(header)
	if err != nil {
		return Principal{}, err
	}
	now := v.now()
	if credential.Timestamp.Before(now.Add(-MaxSkew)) || credential.Timestamp.After(now.Add(MaxSkew)) {
		return Principal{}, ErrStale
	}
	replicaID, public, found := v.lookup(credential.SignerKeyID)
	if !found {
		return Principal{}, ErrUnknownKey
	}
	if replicaID != credential.ReplicaID {
		return Principal{}, ErrWrongPrincipal
	}
	if !v.limits.Allow(credential.ReplicaID, now) {
		return Principal{}, ErrRateLimited
	}
	message := SigningString(Request{
		Method: method, Path: path, DatabaseID: v.databaseID, ReplicaID: credential.ReplicaID,
		Timestamp: credential.Timestamp, Nonce: credential.Nonce, BodySHA256: bodyDigest,
	})
	if !ed25519.Verify(public, message, credential.Signature) {
		return Principal{}, ErrBadSignature
	}
	if err := v.recordNonce(credential.ReplicaID, credential.Nonce, now); err != nil {
		return Principal{}, err
	}
	return Principal{ReplicaID: credential.ReplicaID, SignerKeyID: credential.SignerKeyID}, nil
}

// recordNonce refuses a nonce already seen inside the acceptance window.
//
// Eviction cannot open a replay hole: the cache holds more nonces per peer than
// the rate limit permits within two skew windows, so anything evicted is
// already outside the window the timestamp check enforces.
func (v *Verifier) recordNonce(replicaID, nonce string, now time.Time) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	seen, tracked := v.nonces[replicaID]
	if !tracked {
		if len(v.nonces) >= MaxTrackedPeers {
			v.evictEmptyLocked(now)
		}
		if len(v.nonces) >= MaxTrackedPeers {
			return ErrRateLimited
		}
		seen = map[string]time.Time{}
		v.nonces[replicaID] = seen
	}
	if _, replayed := seen[nonce]; replayed {
		return ErrReplay
	}
	if len(seen) >= MaxNoncesPerPeer {
		for candidate, stamp := range seen {
			if now.Sub(stamp) > MaxSkew {
				delete(seen, candidate)
			}
		}
	}
	if len(seen) >= MaxNoncesPerPeer {
		// Every entry is still inside the window, which the rate limiter should
		// have prevented. Refusing is the safe direction: it costs the peer a
		// retry, while accepting would cost the replay guarantee.
		return ErrRateLimited
	}
	seen[nonce] = now
	return nil
}

func (v *Verifier) evictEmptyLocked(now time.Time) {
	for replicaID, seen := range v.nonces {
		for nonce, stamp := range seen {
			if now.Sub(stamp) > MaxSkew {
				delete(seen, nonce)
			}
		}
		if len(seen) == 0 {
			delete(v.nonces, replicaID)
		}
	}
}

// Limits bound what one peer, and everyone together, may ask for.
type Limits struct {
	RequestsPerMinute int
	Burst             int
	FailuresPerMinute int
}

// DefaultLimits bound a peer generously and a guesser tightly.
//
// The request figures went up when G14 added the data plane: one exchange lists
// namespaces, lists several classes, reads each artifact, and publishes its
// own, so a round is tens of requests rather than one. The failure figure did
// not move, because it bounds something else entirely — how fast an
// unauthenticated caller can guess — and that has nothing to do with how
// chatty a legitimate peer is.
func DefaultLimits() Limits {
	return Limits{RequestsPerMinute: 600, Burst: 120, FailuresPerMinute: 10}
}

// Limiter is a token bucket per peer plus a failure bucket per source address.
//
// The two are separate because they defend different things: the peer bucket
// bounds work an authenticated peer can cause, and the failure bucket bounds
// how fast an unauthenticated caller can guess. Keying failures by address is
// the only option — an unauthenticated caller has no identity yet.
type Limiter struct {
	limits Limits

	mu       sync.Mutex
	buckets  map[string]*bucket
	failures map[string]*bucket
}

type bucket struct {
	tokens  float64
	updated time.Time
}

// NewLimiter builds a limiter.
func NewLimiter(limits Limits) *Limiter {
	if limits.RequestsPerMinute <= 0 {
		limits.RequestsPerMinute = DefaultLimits().RequestsPerMinute
	}
	if limits.Burst <= 0 {
		limits.Burst = DefaultLimits().Burst
	}
	if limits.FailuresPerMinute <= 0 {
		limits.FailuresPerMinute = DefaultLimits().FailuresPerMinute
	}
	return &Limiter{limits: limits, buckets: map[string]*bucket{}, failures: map[string]*bucket{}}
}

// Allow spends one token for a peer.
func (l *Limiter) Allow(replicaID string, now time.Time) bool {
	return l.spend(l.buckets, replicaID, float64(l.limits.RequestsPerMinute), float64(l.limits.Burst), now)
}

// AllowFailure spends one token from a source address's failure budget. It is
// called **after** a refusal, so a caller guessing keys or codes slows to the
// budget rather than to the speed of the network.
//
// Spending it on every request instead would charge a legitimate peer for its
// own successes — which is exactly the bug G14's first data-plane run hit, when
// a round's dozen ordinary requests exhausted a budget meant for guessers.
func (l *Limiter) AllowFailure(address string, now time.Time) bool {
	return l.spend(l.failures, address, float64(l.limits.FailuresPerMinute), float64(l.limits.FailuresPerMinute), now)
}

// FailureBudgetExhausted reports whether an address has already spent its
// failure budget, without spending anything itself. A caller that is being
// refused repeatedly is answered from here before any work is done.
func (l *Limiter) FailureBudgetExhausted(address string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	current, found := l.failures[address]
	if !found {
		return false
	}
	elapsed := now.Sub(current.updated).Seconds()
	tokens := current.tokens + elapsed*float64(l.limits.FailuresPerMinute)/60
	return tokens < 1
}

func (l *Limiter) spend(buckets map[string]*bucket, key string, perMinute, burst float64, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	current, found := buckets[key]
	if !found {
		if len(buckets) >= MaxTrackedPeers {
			for candidate, entry := range buckets {
				if now.Sub(entry.updated) > time.Minute {
					delete(buckets, candidate)
				}
			}
		}
		if len(buckets) >= MaxTrackedPeers {
			return false
		}
		current = &bucket{tokens: burst, updated: now}
		buckets[key] = current
	}
	elapsed := now.Sub(current.updated).Seconds()
	if elapsed > 0 {
		current.tokens += elapsed * perMinute / 60
		if current.tokens > burst {
			current.tokens = burst
		}
		current.updated = now
	}
	if current.tokens < 1 {
		return false
	}
	current.tokens--
	return true
}

// Reason maps an error to the bounded code an audit record stores. Free text
// from an error would eventually carry something that should not be written
// down; a closed vocabulary cannot.
func Reason(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, ErrMalformed):
		return "malformed"
	case errors.Is(err, ErrUnknownKey):
		return "unenrolled_key"
	case errors.Is(err, ErrBadSignature):
		return "bad_signature"
	case errors.Is(err, ErrStale):
		return "stale_timestamp"
	case errors.Is(err, ErrReplay):
		return "replayed_nonce"
	case errors.Is(err, ErrWrongDatabase):
		return "wrong_database"
	case errors.Is(err, ErrWrongPrincipal):
		return "wrong_principal"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	default:
		return "refused"
	}
}

// Reasons lists the closed vocabulary, so a test can assert nothing else is
// ever written.
func Reasons() []string {
	reasons := []string{
		"ok", "malformed", "unenrolled_key", "bad_signature", "stale_timestamp",
		"replayed_nonce", "wrong_database", "wrong_principal", "rate_limited", "refused",
	}
	sort.Strings(reasons)
	return reasons
}

func lowercaseHex(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch {
		case character >= '0' && character <= '9':
		case character >= 'a' && character <= 'f':
		default:
			return false
		}
	}
	return len(value) > 0
}
