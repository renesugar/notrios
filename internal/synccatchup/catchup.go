// Package synccatchup implements G10's snapshot catch-up: how a blank,
// far-behind, repaired, or deliberately reset replica asks for a verified
// snapshot, chooses among peers that answer, and knows where it is in a process
// that can be interrupted at any point.
//
// Like the rest of the sync core it is transport- and storage-neutral. It signs
// and verifies requests, decides who may answer and which answer to take, wraps
// a payload key two ways, and defines the states a session can be in. Producing
// the archive, moving the bytes, and writing canonical rows all belong to
// callers.
package synccatchup

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// Bounds. A catch-up is a large, slow, interruptible operation, so every one of
// these exists to stop it becoming an unbounded one.
const (
	// MaxResponses bounds how many answers a requester will consider. More than
	// a handful of peers answering one request is not a situation to optimize;
	// it is a situation to bound.
	MaxResponses = 32

	// DefaultTTL is how long a response stays usable. A snapshot describes a
	// state vector that keeps moving, so an old response is not wrong — it is
	// just further behind than the requester was promised.
	DefaultTTL = 24 * time.Hour

	// MaxNonceBytes and NonceBytes fix the request nonce. It exists so two
	// requests from the same replica in the same second are distinguishable.
	NonceBytes    = 16
	MaxNonceBytes = 64
)

// WrappingMode names how the payload key is protected. G10's resolved decision
// is one archive payload format with two wrapping modes, and this is that
// choice made explicit rather than implied by which code path produced a file.
type WrappingMode string

const (
	// WrapPeerKey wraps for the enrolled group of replicas. Ordinary catch-up
	// between a person's own devices uses it, and no password is involved.
	WrapPeerKey WrappingMode = "peer-key"
	// WrapPassword wraps under a memory-hard password derivation. A portable or
	// cloud backup uses it, and the password is supplied at decrypt time and
	// never stored in the archive or in a command line.
	WrapPassword WrappingMode = "password"
)

var wrappingModes = map[WrappingMode]bool{WrapPeerKey: true, WrapPassword: true}

var (
	// ErrNotPermitted reports a responder that is not an enrolled peer this
	// replica has explicitly allowed to answer with a snapshot.
	ErrNotPermitted = errors.New("responder is not a permitted snapshot source")
	// ErrIncompatible reports a response this replica could not use.
	ErrIncompatible = errors.New("snapshot response is incompatible")
	// ErrExpired reports a response whose validity has passed.
	ErrExpired = errors.New("snapshot response has expired")
	// ErrNoUsableResponse reports that nothing offered was usable.
	ErrNoUsableResponse = errors.New("no usable snapshot response")
	// ErrBadTransition reports a state machine move that is not allowed.
	ErrBadTransition = errors.New("invalid catch-up state transition")
	// ErrBadPassword reports a password that did not unwrap the payload key.
	ErrBadPassword = errors.New("password did not unwrap the snapshot key")
	// ErrInvalid reports a malformed request or response.
	ErrInvalid = errors.New("invalid catch-up message")
)

// Request is a replica asking for a snapshot. It is signed, so a responder can
// tell who is asking before it spends anything answering.
type Request struct {
	DatabaseID         string       `json:"database_id"`
	RequesterReplicaID string       `json:"requester_replica_id"`
	ProtocolMajor      int          `json:"protocol_major"`
	ProtocolMinor      int          `json:"protocol_minor"`
	SchemaVersion      int          `json:"schema_version"`
	WrappingMode       WrappingMode `json:"wrapping_mode"`
	Nonce              string       `json:"nonce"`
	CreatedAt          string       `json:"created_at"`
	// KnownVector is what the requester already has. A responder may use it to
	// decide a snapshot is not needed at all, which is the cheapest possible
	// catch-up.
	KnownVector syncstate.Vector `json:"known_vector"`
}

// NewRequest builds a request with a fresh nonce.
func NewRequest(databaseID, requesterReplicaID string, schemaVersion int, mode WrappingMode, known syncstate.Vector, now time.Time) (Request, error) {
	if databaseID == "" || requesterReplicaID == "" {
		return Request{}, fmt.Errorf("%w: a request names its database and requester", ErrInvalid)
	}
	if !wrappingModes[mode] {
		return Request{}, fmt.Errorf("%w: unknown wrapping mode %q", ErrInvalid, mode)
	}
	nonce := make([]byte, NonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return Request{}, err
	}
	return Request{
		DatabaseID: databaseID, RequesterReplicaID: requesterReplicaID,
		ProtocolMajor: syncwire.ProtocolMajor, ProtocolMinor: syncwire.ProtocolMinor,
		SchemaVersion: schemaVersion, WrappingMode: mode,
		Nonce: hex.EncodeToString(nonce), CreatedAt: now.UTC().Format(time.RFC3339Nano),
		KnownVector: syncstate.CloneVector(known),
	}, nil
}

// Response is a peer offering a snapshot it has produced. The state vector is
// the load-bearing field: it says exactly what the snapshot contains, so after
// restoring it the requester asks only for what came later.
type Response struct {
	DatabaseID         string           `json:"database_id"`
	ResponderReplicaID string           `json:"responder_replica_id"`
	RequestNonce       string           `json:"request_nonce"`
	ProtocolMajor      int              `json:"protocol_major"`
	ProtocolMinor      int              `json:"protocol_minor"`
	SchemaVersion      int              `json:"schema_version"`
	SnapshotID         string           `json:"snapshot_id"`
	SnapshotVector     syncstate.Vector `json:"snapshot_vector"`
	ArchiveSHA256      string           `json:"archive_sha256"`
	ArchiveLength      int64            `json:"archive_length"`
	WrappingMode       WrappingMode     `json:"wrapping_mode"`
	ExpiresAt          string           `json:"expires_at"`
}

// canonicalRequest and canonicalResponse produce the exact bytes a signature
// covers. They reuse syncwire's length-prefixed encoding so a field cannot be
// moved between neighbours by choosing its content.
func canonicalRequest(request Request) []byte {
	fields := []string{
		request.DatabaseID, request.RequesterReplicaID, string(request.WrappingMode),
		request.Nonce, request.CreatedAt,
	}
	message := []byte("notrios.catchup-request.v1")
	message = appendInt(message, int64(request.ProtocolMajor))
	message = appendInt(message, int64(request.ProtocolMinor))
	message = appendInt(message, int64(request.SchemaVersion))
	for _, field := range fields {
		message = appendField(message, field)
	}
	return appendVector(message, request.KnownVector)
}

func canonicalResponse(response Response) []byte {
	message := []byte("notrios.catchup-response.v1")
	message = appendInt(message, int64(response.ProtocolMajor))
	message = appendInt(message, int64(response.ProtocolMinor))
	message = appendInt(message, int64(response.SchemaVersion))
	message = appendInt(message, response.ArchiveLength)
	for _, field := range []string{
		response.DatabaseID, response.ResponderReplicaID, response.RequestNonce,
		response.SnapshotID, response.ArchiveSHA256, string(response.WrappingMode), response.ExpiresAt,
	} {
		message = appendField(message, field)
	}
	return appendVector(message, response.SnapshotVector)
}

func appendField(message []byte, value string) []byte {
	message = appendInt(message, int64(len(value)))
	return append(message, value...)
}

func appendInt(message []byte, value int64) []byte {
	var scratch [10]byte
	index := len(scratch)
	unsigned := uint64(value)
	for {
		index--
		scratch[index] = byte(unsigned & 0x7f)
		unsigned >>= 7
		if unsigned == 0 {
			break
		}
		scratch[index] |= 0x80
	}
	return append(message, scratch[index:]...)
}

// appendVector emits entries in sorted replica order, because a map has no
// order and two replicas signing the same vector must sign the same bytes.
func appendVector(message []byte, vector syncstate.Vector) []byte {
	replicaIDs := make([]string, 0, len(vector))
	for replicaID := range vector {
		replicaIDs = append(replicaIDs, replicaID)
	}
	sort.Strings(replicaIDs)
	message = appendInt(message, int64(len(replicaIDs)))
	for _, replicaID := range replicaIDs {
		message = appendField(message, replicaID)
		message = appendInt(message, vector[replicaID])
	}
	return message
}

// SignRequest and SignResponse produce hex signatures over the canonical bytes.
func SignRequest(signer syncwire.Signer, request Request) string {
	return hex.EncodeToString(signer.Sign(canonicalRequest(request)))
}

// VerifyRequest checks a request's signature against an enrolled key.
func VerifyRequest(verifier syncwire.Verifier, signerKeyID, signature string, request Request) error {
	return verifySignature(verifier, signerKeyID, signature, canonicalRequest(request))
}

// SignResponse signs an offer.
func SignResponse(signer syncwire.Signer, response Response) string {
	return hex.EncodeToString(signer.Sign(canonicalResponse(response)))
}

// VerifyResponse checks an offer's signature.
func VerifyResponse(verifier syncwire.Verifier, signerKeyID, signature string, response Response) error {
	return verifySignature(verifier, signerKeyID, signature, canonicalResponse(response))
}

func verifySignature(verifier syncwire.Verifier, signerKeyID, signature string, message []byte) error {
	public, enrolled := verifier.PublicKey(signerKeyID)
	if !enrolled {
		return fmt.Errorf("%w: %s", syncwire.ErrUnknownSigner, signerKeyID)
	}
	raw, err := hex.DecodeString(signature)
	if err != nil || len(raw) != syncwire.SignatureBytes {
		return syncwire.ErrBadSignature
	}
	if !ed25519.Verify(public, message, raw) {
		return syncwire.ErrBadSignature
	}
	return nil
}

// SourcePolicy answers the one question G10's resolved decision asks: may this
// peer answer a backup request at all? Enrollment is not enough — a replica is
// permitted as a snapshot source explicitly, because answering means producing
// and handing over a complete copy of the library.
type SourcePolicy interface {
	PermittedSource(replicaID string) bool
}

// Offer pairs a response with the signature that authenticated it.
type Offer struct {
	Response    Response
	SignerKeyID string
	Signature   string
}

// Select chooses among competing responses.
//
// It never merges: G10's decision is that a requester takes one compatible
// verified response rather than combining several, because two snapshots are
// two different points in the library's history and stitching them produces a
// state neither peer ever had.
//
// Among usable offers it prefers the one whose snapshot vector is furthest
// ahead — that is the least remaining work — and breaks ties by responder id so
// two requesters facing the same offers make the same choice.
func Select(request Request, offers []Offer, policy SourcePolicy, verifier syncwire.Verifier, localSchema int, now time.Time) (Offer, error) {
	if len(offers) > MaxResponses {
		offers = offers[:MaxResponses]
	}
	type scored struct {
		offer Offer
		total int64
	}
	usable := make([]scored, 0, len(offers))
	for _, offer := range offers {
		if err := usableOffer(request, offer, policy, verifier, localSchema, now); err != nil {
			continue
		}
		var total int64
		for _, sequence := range offer.Response.SnapshotVector {
			total += sequence
		}
		usable = append(usable, scored{offer, total})
	}
	if len(usable) == 0 {
		return Offer{}, ErrNoUsableResponse
	}
	sort.Slice(usable, func(i, j int) bool {
		if usable[i].total != usable[j].total {
			return usable[i].total > usable[j].total
		}
		return usable[i].offer.Response.ResponderReplicaID < usable[j].offer.Response.ResponderReplicaID
	})
	return usable[0].offer, nil
}

// Usable reports why one offer is or is not acceptable. Select uses it; a
// caller that wants to explain a refusal to a person calls it directly.
func Usable(request Request, offer Offer, policy SourcePolicy, verifier syncwire.Verifier, localSchema int, now time.Time) error {
	return usableOffer(request, offer, policy, verifier, localSchema, now)
}

func usableOffer(request Request, offer Offer, policy SourcePolicy, verifier syncwire.Verifier, localSchema int, now time.Time) error {
	response := offer.Response
	if policy == nil || !policy.PermittedSource(response.ResponderReplicaID) {
		return fmt.Errorf("%w: %s", ErrNotPermitted, response.ResponderReplicaID)
	}
	if err := VerifyResponse(verifier, offer.SignerKeyID, offer.Signature, response); err != nil {
		return err
	}
	if response.DatabaseID != request.DatabaseID {
		return fmt.Errorf("%w: response is for database %q", ErrIncompatible, response.DatabaseID)
	}
	if subtle.ConstantTimeCompare([]byte(response.RequestNonce), []byte(request.Nonce)) != 1 {
		return fmt.Errorf("%w: response answers a different request", ErrIncompatible)
	}
	if response.ResponderReplicaID == request.RequesterReplicaID {
		return fmt.Errorf("%w: a replica cannot catch up from itself", ErrIncompatible)
	}
	if response.ProtocolMajor != syncwire.ProtocolMajor || response.ProtocolMinor != syncwire.ProtocolMinor {
		return fmt.Errorf("%w: protocol %d.%d", ErrIncompatible, response.ProtocolMajor, response.ProtocolMinor)
	}
	if response.SchemaVersion != localSchema {
		return fmt.Errorf("%w: schema %d, local %d", ErrIncompatible, response.SchemaVersion, localSchema)
	}
	if response.WrappingMode != request.WrappingMode {
		return fmt.Errorf("%w: wrapping mode %q", ErrIncompatible, response.WrappingMode)
	}
	if len(response.ArchiveSHA256) != 64 || response.ArchiveLength <= 0 || response.SnapshotID == "" {
		return fmt.Errorf("%w: response does not name a verifiable archive", ErrIncompatible)
	}
	if err := syncstate.ValidateVector(response.SnapshotVector); err != nil {
		return fmt.Errorf("%w: %w", ErrIncompatible, err)
	}
	expiry, err := time.Parse(time.RFC3339Nano, response.ExpiresAt)
	if err != nil {
		return fmt.Errorf("%w: unparseable expiry", ErrIncompatible)
	}
	if !now.Before(expiry) {
		return fmt.Errorf("%w: expired at %s", ErrExpired, response.ExpiresAt)
	}
	return nil
}

// State is where a catch-up has got to. It is durable because every step can be
// interrupted — a snapshot is large, and the whole point of the feature is that
// it survives a closed laptop.
type State string

const (
	StateRequested    State = "requested"
	StateOffered      State = "offered"
	StateTransferring State = "transferring"
	StateVerifying    State = "verifying"
	StateRestoring    State = "restoring"
	StateCutover      State = "cutover"
	StateComplete     State = "complete"
	StateCancelled    State = "cancelled"
	StateExpired      State = "expired"
	StateFailed       State = "failed"
)

// transitions is the whole state machine, written out rather than inferred. In
// particular there is no edge from `restoring` back to `transferring`: once
// canonical rows are being written, the answer to a problem is to finish or to
// fail, not to fetch the archive again underneath a half-restored library.
var transitions = map[State][]State{
	StateRequested:    {StateOffered, StateCancelled, StateExpired, StateFailed},
	StateOffered:      {StateTransferring, StateCancelled, StateExpired, StateFailed},
	StateTransferring: {StateTransferring, StateVerifying, StateCancelled, StateExpired, StateFailed},
	StateVerifying:    {StateRestoring, StateTransferring, StateCancelled, StateFailed},
	StateRestoring:    {StateCutover, StateFailed},
	StateCutover:      {StateComplete, StateFailed},
	StateComplete:     {},
	StateCancelled:    {},
	StateExpired:      {},
	StateFailed:       {},
}

// Transition reports whether a move is allowed, and says so rather than
// silently permitting it. `transferring` to itself is allowed because a resumed
// transfer is progress within the same state.
func Transition(from, to State) error {
	allowed, known := transitions[from]
	if !known {
		return fmt.Errorf("%w: unknown state %q", ErrBadTransition, from)
	}
	for _, candidate := range allowed {
		if candidate == to {
			return nil
		}
	}
	return fmt.Errorf("%w: %s cannot become %s", ErrBadTransition, from, to)
}

// Terminal reports whether a state can still change.
func Terminal(state State) bool {
	allowed, known := transitions[state]
	return known && len(allowed) == 0
}

// Argon2id parameters for password wrapping. They are recorded here rather than
// chosen at call sites so every archive this build produces is reproducible,
// and so raising them later is a visible decision.
const (
	passwordTime      = 3
	passwordMemory    = 64 * 1024 // KiB, so 64 MiB
	passwordThreads   = 4
	PasswordSaltBytes = 16
)

// WrapWithPassword protects a payload key with a memory-hard derivation. The
// password is an argument, never a field: G10's resolved decision is that it is
// entered at decrypt time and never stored in the archive or a command line,
// and a struct field is the easiest way to end up logging one.
func WrapWithPassword(password string, payloadKey []byte) (salt, wrapped []byte, err error) {
	if password == "" {
		return nil, nil, fmt.Errorf("%w: an empty password wraps nothing", ErrInvalid)
	}
	if len(payloadKey) != syncwire.GroupKeyBytes {
		return nil, nil, fmt.Errorf("%w: payload key must be %d bytes", ErrInvalid, syncwire.GroupKeyBytes)
	}
	salt = make([]byte, PasswordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	derived := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, uint32(len(payloadKey)))
	wrapped = make([]byte, len(payloadKey))
	for index := range payloadKey {
		wrapped[index] = payloadKey[index] ^ derived[index]
	}
	// The check value lets a wrong password be reported as a wrong password
	// rather than as a corrupt archive, without revealing the key.
	check := argon2.IDKey(derived, salt, 1, passwordMemory, passwordThreads, 16)
	return salt, append(wrapped, check...), nil
}

// UnwrapWithPassword reverses it, distinguishing a wrong password from damage.
func UnwrapWithPassword(password string, salt, wrapped []byte) ([]byte, error) {
	if len(salt) != PasswordSaltBytes || len(wrapped) != syncwire.GroupKeyBytes+16 {
		return nil, fmt.Errorf("%w: wrapped key is malformed", ErrInvalid)
	}
	derived := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, syncwire.GroupKeyBytes)
	check := argon2.IDKey(derived, salt, 1, passwordMemory, passwordThreads, 16)
	if subtle.ConstantTimeCompare(check, wrapped[syncwire.GroupKeyBytes:]) != 1 {
		return nil, ErrBadPassword
	}
	payloadKey := make([]byte, syncwire.GroupKeyBytes)
	for index := range payloadKey {
		payloadKey[index] = wrapped[index] ^ derived[index]
	}
	return payloadKey, nil
}

// RemainingWork reports what a replica must still fetch after restoring a
// snapshot taken at `snapshot`, given what the source now has. It is the whole
// justification for recording the vector in the manifest: without it a restored
// replica would have to ask for everything again to find out what it had.
func RemainingWork(snapshot, source syncstate.Vector) (syncstate.MissingPlan, error) {
	return syncstate.PlanMissing(snapshot, source, nil)
}
