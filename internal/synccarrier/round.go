package synccarrier

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/renesugar/notrios/internal/syncassets"
	"github.com/renesugar/notrios/internal/syncstate"
	"github.com/renesugar/notrios/internal/syncwire"
)

// AdmissionResult is what a round learns from handing operations to the local
// replica. It is a copy of the store's result rather than the store's own type
// so that the protocol in this file, and every test of it, needs no database.
type AdmissionResult struct {
	Received   int
	Admitted   int
	Pending    int
	Duplicates int
}

// PeerState is one enrolled peer as the local replica remembers it, carrying
// the vector that peer last acknowledged.
type PeerState struct {
	Handshake syncstate.Handshake
}

// WantedObject names resource bytes this replica holds metadata for but not
// content.
type WantedObject struct {
	BlobSHA256 string
	ByteLength int64
}

// Replica is the local canonical state a round reads and writes. Everything it
// exposes already existed before G11: the carrier adds a transport, not a
// second opinion about what is true.
type Replica interface {
	LocalSyncHandshake(ctx context.Context) (syncstate.Handshake, error)
	SyncStateVector(ctx context.Context) (syncstate.Vector, error)
	// SyncPeerEnrolled reports whether a replica has been explicitly configured
	// for admission. Discovery uses it to keep pairing an explicit act: an
	// unenrolled peer is reported and never applied.
	SyncPeerEnrolled(ctx context.Context, replicaID string) (bool, error)
	// EnrolledPeers is what this replica durably remembers about its peers,
	// including the vector each last acknowledged. A round starts from this
	// rather than from the carrier, so a folder that is empty — new, deleted,
	// or handed to a peer that has not written to it yet — still receives
	// exactly the work the peer is missing.
	EnrolledPeers(ctx context.Context) ([]PeerState, error)
	ListSyncOperations(ctx context.Context, replicaID string, afterSequence int64, limit int) ([]syncstate.Operation, error)
	AdmitSyncOperations(ctx context.Context, peer syncstate.Handshake, operations []syncstate.Operation) (AdmissionResult, error)
	RecordSyncPeerAcknowledgement(ctx context.Context, peer syncstate.Handshake) error
	// WantedSyncObjects lists resources this replica would materialize if a
	// peer published the bytes.
	WantedSyncObjects(ctx context.Context, limit int) ([]WantedObject, error)
}

// ObjectSource reads locally held resource bytes so they can be published for
// a peer that asked. It is deliberately the same shape as the store's
// ObjectProvider, so the local provider G8 already ships satisfies it.
type ObjectSource interface {
	FetchManifest(ctx context.Context, blobSHA256 string) (syncassets.Manifest, error)
	FetchChunk(ctx context.Context, blobSHA256 string, ordinal int) ([]byte, error)
}

// Candidate is a peer seen on the carrier that this replica has not enrolled.
// Discovery reports; it never enrolls. A carrier is a place anyone with the
// folder can write to, and treating presence as permission would make the
// folder the authority the whole design says it is not.
type Candidate struct {
	// Namespace is the blinded directory the candidate writes to.
	Namespace string
	// SignerKeyID identifies the signing key. It is the only thing knowable
	// about a candidate whose key is not enrolled, because an artifact from an
	// unknown signer is refused before it is decrypted.
	SignerKeyID string
	// ReplicaID, DatabaseID, and SchemaVersion are present only when the
	// signing key is enrolled but the replica is not configured for admission.
	ReplicaID     string
	DatabaseID    string
	SchemaVersion int
	Reason        string
}

// Result summarizes one round. It counts and names peers, never notes: a
// caller may log it whole.
type Result struct {
	Namespaces           int
	Peers                []string
	Candidates           []Candidate
	PublishedEnvelopes   int
	PublishedOperations  int
	PublishedObjects     int
	PublishedRequests    int
	AdmittedEnvelopes    int
	AdmittedOperations   int
	PendingOperations    int
	DuplicateOperations  int
	PublishedObjectBytes int64
	Removed              int
	ScannedArtifacts     int
	// Skipped counts carrier entries that were not usable, by reason. A hostile
	// or half-copied carrier is an ordinary outcome, so these are counts rather
	// than errors.
	Skipped map[string]int
}

func (r *Result) skip(reason string) {
	if r.Skipped == nil {
		r.Skipped = map[string]int{}
	}
	r.Skipped[reason]++
}

// Options bound one round. Every limit here is a promise about how much work a
// single sync can cause, which is what makes running one from a phone or a cron
// job safe.
type Options struct {
	Now                      func() time.Time
	MaxOperationsPerEnvelope int
	MaxOperationsPerRound    int
	MaxArtifactsPerScan      int
	MaxObjectsPerRound       int
	// Cleanup removes this replica's own superseded and fully acknowledged
	// artifacts. It defaults off: G11's resolved decision is that correctness
	// must survive no cleanup at all, and a test that never cleans is the only
	// way to keep that true.
	Cleanup bool
	Limits  syncwire.Limits
	// Checkpoint runs after each durable phase. Returning an error stops before
	// the next phase. G15 uses this for heartbeats, cooperative cancellation,
	// and content-free job checkpoints; nil preserves the pre-G15 behaviour.
	Checkpoint func(ctx context.Context, phase string, result Result) error
}

func (o Options) normalized() Options {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.MaxOperationsPerEnvelope <= 0 {
		o.MaxOperationsPerEnvelope = 1_000
	}
	if o.MaxOperationsPerRound <= 0 {
		o.MaxOperationsPerRound = int(syncstate.MaxPlannedSequences)
	}
	if o.MaxArtifactsPerScan <= 0 {
		o.MaxArtifactsPerScan = MaxEntriesPerClass
	}
	if o.MaxObjectsPerRound <= 0 {
		o.MaxObjectsPerRound = 64
	}
	return o
}

// Round performs one complete exchange through a carrier.
//
// The order of its phases is the protocol's only real invariant: read before
// write, and advertise last. A peer that reads this replica's advertisement
// must already be able to find the envelopes that advertisement implies exist,
// which is the same rule archive-v2 follows by writing its manifest last.
type Round struct {
	replica  Replica
	carrier  Carrier
	keys     syncwire.KeyRing
	signer   syncwire.Signer
	verifier syncwire.Verifier
	objects  ObjectSource
	options  Options
}

// NewRound binds a local replica to a carrier. The object source may be nil,
// which means this replica serves no resource bytes — it still sends and
// receives operations, and a note whose attachment nobody publishes stays
// exactly as G8 leaves it: known, referenced, and unavailable.
func NewRound(replica Replica, carrier Carrier, keys syncwire.KeyRing, signer syncwire.Signer, verifier syncwire.Verifier, objects ObjectSource, options Options) *Round {
	return &Round{
		replica: replica, carrier: carrier, keys: keys, signer: signer,
		verifier: verifier, objects: objects, options: options.normalized(),
	}
}

// peerView is what one round knows about one peer: what the journal remembers,
// then whatever the carrier had to say about it.
type peerView struct {
	namespace string
	handshake syncstate.Handshake
	requests  []Request
	// advertised records that this peer's own current statement was read from
	// the carrier, rather than recalled from the last one it made.
	advertised bool
}

// Carrier returns the carrier this round publishes to, so a caller can build an
// object provider over the same one.
func (r *Round) Carrier() Carrier { return r.carrier }

// Discover reads the carrier and reports who is on it, without publishing,
// admitting, or enrolling anything.
//
// It exists so that "who else is using this folder?" is answerable before a
// user decides to trust anyone, and so that asking cannot itself change
// anything: a discovery pass that advertised would announce this replica to a
// folder its owner has not yet decided to join.
func (r *Round) Discover(ctx context.Context) (Result, error) {
	result := Result{}
	local, err := r.replica.LocalSyncHandshake(ctx)
	if err != nil {
		return result, err
	}
	peers, err := r.rememberedPeers(ctx)
	if err != nil {
		return result, err
	}
	for replicaID := range peers {
		peers[replicaID].handshake.StateVector = syncstate.Vector{}
	}
	if err := r.scanPeers(ctx, local, peers, &result); err != nil {
		return result, err
	}
	for replicaID, peer := range peers {
		if peer.advertised {
			result.Peers = append(result.Peers, replicaID)
		}
	}
	sort.Strings(result.Peers)
	return result, nil
}

// Run executes one round: scan, admit, acknowledge, publish, advertise, clean.
func (r *Round) Run(ctx context.Context) (Result, error) {
	result := Result{}
	local, err := r.replica.LocalSyncHandshake(ctx)
	if err != nil {
		return result, err
	}
	if err := r.carrier.Initialize(ctx); err != nil {
		return result, err
	}
	if err := r.checkpoint(ctx, "plan", result); err != nil {
		return result, err
	}

	peers, err := r.rememberedPeers(ctx)
	if err != nil {
		return result, err
	}
	if err := r.scanPeers(ctx, local, peers, &result); err != nil {
		return result, err
	}
	if err := r.admit(ctx, local, peers, &result); err != nil {
		return result, err
	}
	if err := r.checkpoint(ctx, "pull", result); err != nil {
		return result, err
	}
	// The vector is re-read after admission so this replica advertises what it
	// now holds rather than what it held when the round started. A peer reading
	// a stale vector would resend work that is already durable here.
	local, err = r.replica.LocalSyncHandshake(ctx)
	if err != nil {
		return result, err
	}
	if err := r.serveObjects(ctx, peers, &result); err != nil {
		return result, err
	}
	if err := r.checkpoint(ctx, "resource_serve", result); err != nil {
		return result, err
	}
	if err := r.publishOperations(ctx, local, peers, &result); err != nil {
		return result, err
	}
	if err := r.checkpoint(ctx, "push", result); err != nil {
		return result, err
	}
	requestName, err := r.publishRequest(ctx, local, peers, &result)
	if err != nil {
		return result, err
	}
	if err := r.advertise(ctx, local, &result); err != nil {
		return result, err
	}
	if err := r.checkpoint(ctx, "advertise", result); err != nil {
		return result, err
	}
	if r.options.Cleanup {
		if err := r.cleanup(ctx, local, peers, requestName, &result); err != nil {
			return result, err
		}
	}
	for replicaID := range peers {
		result.Peers = append(result.Peers, replicaID)
	}
	sort.Strings(result.Peers)
	sort.Slice(result.Candidates, func(i, j int) bool {
		if result.Candidates[i].Namespace != result.Candidates[j].Namespace {
			return result.Candidates[i].Namespace < result.Candidates[j].Namespace
		}
		return result.Candidates[i].SignerKeyID < result.Candidates[j].SignerKeyID
	})
	return result, nil
}

func (r *Round) checkpoint(ctx context.Context, phase string, result Result) error {
	if r.options.Checkpoint == nil {
		return ctx.Err()
	}
	return r.options.Checkpoint(ctx, phase, result)
}

// rememberedPeers seeds the round from durable local state, so a carrier that
// holds nothing at all still receives exactly what each enrolled peer lacks.
//
// Without this a fresh or deleted folder is a standoff: every replica waits to
// learn what its peer has before publishing anything, and nothing moves until
// one of them happens to speak first. It also makes a removable drive work in
// two trips instead of three, because the peer does not have to visit the drive
// once merely to say hello.
func (r *Round) rememberedPeers(ctx context.Context) (map[string]*peerView, error) {
	remembered, err := r.replica.EnrolledPeers(ctx)
	if err != nil {
		return nil, err
	}
	peers := map[string]*peerView{}
	for _, peer := range remembered {
		if peer.Handshake.ReplicaID == "" {
			continue
		}
		if peer.Handshake.StateVector == nil {
			peer.Handshake.StateVector = syncstate.Vector{}
		}
		peers[peer.Handshake.ReplicaID] = &peerView{
			namespace: r.namespaceOf(peer.Handshake.ReplicaID), handshake: peer.Handshake,
		}
	}
	return peers, nil
}

// scanPeers reads every advertisement on the carrier and sorts what it finds
// into enrolled peers and reportable candidates.
func (r *Round) scanPeers(ctx context.Context, local syncstate.Handshake, peers map[string]*peerView, result *Result) error {
	namespaces, err := r.carrier.Namespaces(ctx)
	if err != nil {
		return err
	}
	result.Namespaces = len(namespaces)
	for _, namespace := range namespaces {
		if namespace == r.carrier.Namespace() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		view, candidate, err := r.readNamespace(ctx, local, namespace, result)
		if err != nil {
			return err
		}
		if candidate != nil {
			result.Candidates = append(result.Candidates, *candidate)
			continue
		}
		if view == nil {
			continue
		}
		// A namespace holding two advertisements is normal — a peer that has
		// not cleaned up leaves its previous one behind. The higher vector is
		// the newer statement, and neither file's timestamp is consulted.
		//
		// A peer's own advertisement replaces what this replica remembered
		// about it, rather than being merged with it: the peer is the authority
		// on itself, and taking the higher of the two would let a stale memory
		// suppress work a reset peer now needs.
		if existing, seen := peers[view.handshake.ReplicaID]; seen && existing.advertised {
			if relation, err := syncstate.CompareVectors(view.handshake.StateVector, existing.handshake.StateVector); err == nil &&
				relation != syncstate.VectorAhead {
				continue
			}
		}
		view.advertised = true
		peers[view.handshake.ReplicaID] = view
	}
	return nil
}

// readNamespace resolves one namespace to a peer or a candidate.
func (r *Round) readNamespace(ctx context.Context, local syncstate.Handshake, namespace string, result *Result) (*peerView, *Candidate, error) {
	names, err := r.carrier.List(ctx, namespace, ClassAdvertisement)
	if err != nil {
		return nil, nil, err
	}
	var best *peerView
	var candidate *Candidate
	for index, name := range names {
		if index >= r.options.MaxArtifactsPerScan {
			result.skip("scan_bound")
			break
		}
		result.ScannedArtifacts++
		artifact, err := r.carrier.Read(ctx, namespace, ClassAdvertisement, name)
		if err != nil {
			result.skip("unreadable_artifact")
			continue
		}
		header, plaintext, err := syncwire.Open(r.keys, r.verifier, artifact, r.options.Limits)
		if err != nil {
			if errors.Is(err, syncwire.ErrUnknownSigner) && candidate == nil {
				candidate = &Candidate{Namespace: namespace, Reason: "unenrolled_signing_key"}
				if decoded, decodeErr := syncwire.PeekSignerKeyID(artifact, r.options.Limits); decodeErr == nil {
					candidate.SignerKeyID = decoded
				}
			}
			result.skip(openReason(err))
			continue
		}
		advertisement, err := DecodeAdvertisement(plaintext)
		if err != nil {
			result.skip("malformed_advertisement")
			continue
		}
		peer := advertisement.Handshake
		if peer.DatabaseID != local.DatabaseID || peer.ReplicaID == local.ReplicaID {
			result.skip("foreign_database")
			continue
		}
		// The namespace is bound to the identity claimed inside the artifact.
		// Without this an enrolled peer could publish under another peer's
		// folder and have its advertisement read as that peer's.
		if namespace != r.namespaceOf(peer.ReplicaID) {
			result.skip("namespace_identity_mismatch")
			continue
		}
		enrolled, err := r.replica.SyncPeerEnrolled(ctx, peer.ReplicaID)
		if err != nil {
			return nil, nil, err
		}
		if !enrolled {
			candidate = &Candidate{
				Namespace: namespace, SignerKeyID: header.SignerKeyID, ReplicaID: peer.ReplicaID,
				DatabaseID: peer.DatabaseID, SchemaVersion: peer.SchemaVersion, Reason: "not_enrolled_for_admission",
			}
			continue
		}
		if err := syncstate.ValidateHandshake(local.DatabaseID, local.ReplicaID, local.SchemaVersion, peer); err != nil {
			result.skip("incompatible_peer")
			continue
		}
		if best == nil {
			best = &peerView{namespace: namespace, handshake: peer}
			continue
		}
		if relation, err := syncstate.CompareVectors(peer.StateVector, best.handshake.StateVector); err == nil &&
			relation == syncstate.VectorAhead {
			best.handshake = peer
		}
	}
	if best == nil {
		return nil, candidate, nil
	}
	best.requests = r.readRequests(ctx, local, best, result)
	return best, nil, nil
}

// readRequests collects what one enrolled peer has asked for.
func (r *Round) readRequests(ctx context.Context, local syncstate.Handshake, peer *peerView, result *Result) []Request {
	names, err := r.carrier.List(ctx, peer.namespace, ClassRequest)
	if err != nil {
		return nil
	}
	var requests []Request
	for index, name := range names {
		if index >= r.options.MaxArtifactsPerScan {
			result.skip("scan_bound")
			break
		}
		result.ScannedArtifacts++
		artifact, err := r.carrier.Read(ctx, peer.namespace, ClassRequest, name)
		if err != nil {
			result.skip("unreadable_artifact")
			continue
		}
		_, plaintext, err := syncwire.Open(r.keys, r.verifier, artifact, r.options.Limits)
		if err != nil {
			result.skip(openReason(err))
			continue
		}
		request, err := DecodeRequest(plaintext)
		if err != nil {
			result.skip("malformed_request")
			continue
		}
		if request.DatabaseID != local.DatabaseID || request.RequesterReplicaID != peer.handshake.ReplicaID {
			result.skip("request_identity_mismatch")
			continue
		}
		requests = append(requests, request)
	}
	return requests
}

// admit reads every enrolled peer's envelopes and hands them to the local
// replica. Refusals are counted, never fatal: one bad envelope on a shared
// folder must not stop the round that would have delivered the good ones.
func (r *Round) admit(ctx context.Context, local syncstate.Handshake, peers map[string]*peerView, result *Result) error {
	for _, replicaID := range sortedPeerIDs(peers) {
		peer := peers[replicaID]
		names, err := r.carrier.List(ctx, peer.namespace, ClassEnvelope)
		if err != nil {
			return err
		}
		for index, name := range names {
			if err := ctx.Err(); err != nil {
				return err
			}
			if index >= r.options.MaxArtifactsPerScan {
				result.skip("scan_bound")
				break
			}
			result.ScannedArtifacts++
			artifact, err := r.carrier.Read(ctx, peer.namespace, ClassEnvelope, name)
			if err != nil {
				result.skip("unreadable_artifact")
				continue
			}
			header, plaintext, err := syncwire.Open(r.keys, r.verifier, artifact, r.options.Limits)
			if err != nil {
				result.skip(openReason(err))
				continue
			}
			envelope, err := syncwire.DecodeEnvelope(plaintext, r.options.Limits)
			if err != nil {
				result.skip("malformed_envelope")
				continue
			}
			if envelope.DatabaseID != local.DatabaseID {
				result.skip("foreign_database")
				continue
			}
			// The sender named inside the envelope must be the replica whose
			// key signed the artifact, and that replica must own the namespace
			// the artifact was found in.
			if err := syncwire.BindEnvelopeSender(header, envelope, peer.handshake.ReplicaID); err != nil {
				result.skip("sender_mismatch")
				continue
			}
			admission, err := r.replica.AdmitSyncOperations(ctx, peer.handshake, envelope.Operations)
			if err != nil {
				result.skip("refused_admission")
				continue
			}
			result.AdmittedEnvelopes++
			result.AdmittedOperations += admission.Admitted
			result.PendingOperations += admission.Pending
			result.DuplicateOperations += admission.Duplicates
		}
		// The peer's advertised vector is its acknowledgement of this replica's
		// work, which is what lets retention and cleanup know what is safe.
		// Only a statement actually read from the carrier is recorded: writing
		// back what was already remembered would refresh an acknowledgement's
		// timestamp without any peer having said anything.
		if peer.advertised {
			if err := r.replica.RecordSyncPeerAcknowledgement(ctx, peer.handshake); err != nil {
				result.skip("unrecorded_acknowledgement")
			}
		}
	}
	return nil
}

// publishOperations writes the envelopes each peer is missing.
//
// A peer's own advertisement says what it has, so the sender computes the gap
// rather than waiting to be asked. When no advertisement is present at all —
// a carrier that was just created, or just deleted and recreated — every peer
// is treated as knowing nothing, which is what makes republication automatic.
func (r *Round) publishOperations(ctx context.Context, local syncstate.Handshake, peers map[string]*peerView, result *Result) error {
	wanted, err := r.wantedRanges(ctx, local, peers)
	if err != nil {
		return err
	}
	published := 0
	for _, span := range wanted {
		if published >= r.options.MaxOperationsPerRound {
			result.skip("round_operation_bound")
			break
		}
		for start := span.Start; start <= span.End; {
			if err := ctx.Err(); err != nil {
				return err
			}
			batch := r.options.MaxOperationsPerEnvelope
			if remaining := int(span.End - start + 1); remaining < batch {
				batch = remaining
			}
			if remaining := r.options.MaxOperationsPerRound - published; remaining < batch {
				batch = remaining
			}
			if batch <= 0 {
				break
			}
			operations, err := r.replica.ListSyncOperations(ctx, span.ReplicaID, start-1, batch)
			if err != nil {
				return err
			}
			if len(operations) == 0 {
				break
			}
			last := operations[len(operations)-1].Sequence
			name, plaintext, err := r.encodeEnvelope(local, span.ReplicaID, operations)
			if err != nil {
				return err
			}
			written, err := r.publishOnce(ctx, ClassEnvelope, name, syncwire.KindEnvelope, plaintext)
			if err != nil {
				return err
			}
			if written {
				result.PublishedEnvelopes++
				result.PublishedOperations += len(operations)
			}
			published += len(operations)
			start = last + 1
		}
	}
	return nil
}

// wantedRanges is the union of what every peer lacks, plus anything a peer
// asked for explicitly.
func (r *Round) wantedRanges(ctx context.Context, local syncstate.Handshake, peers map[string]*peerView) ([]syncstate.SequenceRange, error) {
	needed := map[string]int64{}
	add := func(replicaID string, upTo int64) {
		if current, seen := needed[replicaID]; !seen || upTo > current {
			needed[replicaID] = upTo
		}
	}
	floors := map[string]int64{}
	setFloor := func(replicaID string, from int64) {
		if current, seen := floors[replicaID]; !seen || from < current {
			floors[replicaID] = from
		}
	}
	for _, replicaID := range sortedPeerIDs(peers) {
		peer := peers[replicaID]
		plan, err := syncstate.PlanMissing(peer.handshake.StateVector, local.StateVector, nil)
		if err != nil {
			return nil, err
		}
		for _, span := range plan.Ranges {
			add(span.ReplicaID, span.End)
			setFloor(span.ReplicaID, span.Start)
		}
		for _, request := range peer.requests {
			for _, span := range request.Ranges {
				if held, known := local.StateVector[span.ReplicaID]; known && held > 0 {
					end := span.End
					if held < end {
						end = held
					}
					add(span.ReplicaID, end)
					setFloor(span.ReplicaID, span.Start)
				}
			}
		}
	}
	ranges := make([]syncstate.SequenceRange, 0, len(needed))
	for _, replicaID := range sortedKeys(needed) {
		start := floors[replicaID]
		if start < 1 {
			start = 1
		}
		ranges = append(ranges, syncstate.SequenceRange{ReplicaID: replicaID, Start: start, End: needed[replicaID]})
	}
	return ranges, nil
}

// encodeEnvelope names and encodes one envelope.
//
// The name is a keyed blind of what the envelope logically covers, not a hash
// of its sealed bytes. Sealing draws a fresh salt every time, so byte-named
// artifacts would pile up a new file per round forever; a logically named one
// is recognized as already published and the round leaves it alone.
func (r *Round) encodeEnvelope(local syncstate.Handshake, authorReplicaID string, operations []syncstate.Operation) (string, []byte, error) {
	group, err := r.keys.Current()
	if err != nil {
		return "", nil, err
	}
	envelope := syncwire.Envelope{
		ProtocolMajor: syncstate.ProtocolMajor, ProtocolMinor: syncstate.ProtocolMinor,
		DatabaseID: local.DatabaseID, SenderReplicaID: local.ReplicaID,
		StateVector: local.StateVector, Operations: operations,
	}
	plaintext, err := syncwire.EncodeEnvelope(envelope, r.options.Limits)
	if err != nil {
		return "", nil, err
	}
	address := fmt.Sprintf("%s|%s|%d-%d", local.ReplicaID, authorReplicaID,
		operations[0].Sequence, operations[len(operations)-1].Sequence)
	return syncwire.CarrierName(group, "envelope", address), plaintext, nil
}

// publishRequest asks for what a vector cannot express: resource bytes this
// replica lacks, and ranges no peer's advertisement covers.
func (r *Round) publishRequest(ctx context.Context, local syncstate.Handshake, peers map[string]*peerView, result *Result) (string, error) {
	request := Request{
		DatabaseID: local.DatabaseID, RequesterReplicaID: local.ReplicaID,
		CreatedAt: r.options.Now().UTC().Format(time.RFC3339Nano),
	}
	wanted, err := r.replica.WantedSyncObjects(ctx, r.options.MaxObjectsPerRound)
	if err != nil {
		return "", err
	}
	for _, object := range wanted {
		request.Objects = append(request.Objects, ObjectWant{BlobSHA256: object.BlobSHA256})
	}
	for _, replicaID := range sortedPeerIDs(peers) {
		plan, err := syncstate.PlanMissing(local.StateVector, peers[replicaID].handshake.StateVector, nil)
		if err != nil {
			return "", err
		}
		request.Ranges = append(request.Ranges, plan.Ranges...)
	}
	if len(request.Objects) == 0 && len(request.Ranges) == 0 {
		// Nothing to ask for. Publishing an empty request would leave a file
		// saying "I want nothing" that a holder would keep reading.
		return "", nil
	}
	sortRanges(request.Ranges)
	group, err := r.keys.Current()
	if err != nil {
		return "", err
	}
	plaintext, err := EncodeRequest(request)
	if err != nil {
		return "", err
	}
	name := syncwire.CarrierName(group, "request", local.ReplicaID+"|"+requestDigest(request))
	written, err := r.publishOnce(ctx, ClassRequest, name, syncwire.KindManifest, plaintext)
	if err != nil {
		return "", err
	}
	if written {
		result.PublishedRequests++
	}
	return name, nil
}

// serveObjects publishes the resource bytes peers asked for and that this
// replica actually holds. Nothing is published unasked: a carrier that
// received every attachment automatically would be a copy of the library,
// which is exactly what an ephemeral postbox must not become.
func (r *Round) serveObjects(ctx context.Context, peers map[string]*peerView, result *Result) error {
	if r.objects == nil {
		return nil
	}
	group, err := r.keys.Current()
	if err != nil {
		return err
	}
	served := map[string]bool{}
	for _, replicaID := range sortedPeerIDs(peers) {
		for _, request := range peers[replicaID].requests {
			for _, want := range request.Objects {
				if err := ctx.Err(); err != nil {
					return err
				}
				if served[want.BlobSHA256] {
					continue
				}
				if len(served) >= r.options.MaxObjectsPerRound {
					result.skip("round_object_bound")
					return nil
				}
				served[want.BlobSHA256] = true
				if err := r.serveOneObject(ctx, group, want, result); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *Round) serveOneObject(ctx context.Context, group syncwire.GroupKey, want ObjectWant, result *Result) error {
	manifest, err := r.objects.FetchManifest(ctx, want.BlobSHA256)
	if err != nil {
		// Not holding an object a peer asked for is the ordinary case in a
		// library where attachments are materialized lazily.
		result.skip("object_not_held")
		return nil
	}
	encoded, err := encodeManifest(manifest)
	if err != nil {
		return err
	}
	if err := r.publishObject(ctx, group, want.BlobSHA256, manifestOrdinal, encoded, result); err != nil {
		return err
	}
	ordinals := want.Ordinals
	if len(ordinals) == 0 {
		ordinals = make([]int, 0, len(manifest.Chunks))
		for ordinal := range manifest.Chunks {
			ordinals = append(ordinals, ordinal)
		}
	}
	for _, ordinal := range ordinals {
		if ordinal < 0 || ordinal >= len(manifest.Chunks) {
			result.skip("object_ordinal_out_of_range")
			continue
		}
		content, err := r.objects.FetchChunk(ctx, want.BlobSHA256, ordinal)
		if err != nil {
			result.skip("object_chunk_unavailable")
			continue
		}
		if err := r.publishObject(ctx, group, want.BlobSHA256, ordinal, content, result); err != nil {
			return err
		}
	}
	return nil
}

// manifestOrdinal addresses an object's manifest in the same namespace as its
// chunks without colliding with chunk zero.
const manifestOrdinal = -1

func (r *Round) publishObject(ctx context.Context, group syncwire.GroupKey, blobSHA256 string, ordinal int, content []byte, result *Result) error {
	name := ObjectName(group, blobSHA256, ordinal)
	written, err := r.publishOnce(ctx, ClassObject, name, syncwire.KindObject, content)
	if err != nil {
		return err
	}
	if written {
		result.PublishedObjects++
		result.PublishedObjectBytes += int64(len(content))
	}
	return nil
}

// ObjectName is the stable carrier address of one resource manifest or chunk.
// It is a keyed blind rather than the content hash, because a folder listing
// full of plaintext hashes would let anyone holding the same file confirm that
// this library holds it — the exact leak G0 froze the routing rule to prevent.
func ObjectName(group syncwire.GroupKey, blobSHA256 string, ordinal int) string {
	address := blobSHA256 + "#manifest"
	if ordinal >= 0 {
		address = fmt.Sprintf("%s#%d", blobSHA256, ordinal)
	}
	return syncwire.CarrierName(group, "object", address)
}

// advertise publishes this replica's handshake last, so that everything the
// advertised vector implies is already readable on the carrier.
func (r *Round) advertise(ctx context.Context, local syncstate.Handshake, result *Result) error {
	group, err := r.keys.Current()
	if err != nil {
		return err
	}
	plaintext, err := EncodeAdvertisement(NewAdvertisement(local, r.options.Now()))
	if err != nil {
		return err
	}
	name := syncwire.CarrierName(group, "advertisement", local.ReplicaID+"|"+vectorDigest(local.StateVector))
	_, err = r.publishOnce(ctx, ClassAdvertisement, name, syncwire.KindManifest, plaintext)
	return err
}

// cleanup removes this replica's own superseded advertisements and requests.
//
// Envelopes are removed only once every enrolled peer's advertised vector
// covers them, which is G11's resolved decision: the writer removes its own
// expired artifacts after durable acknowledgement by all active peers, and
// never touches anyone else's. Carrier cleanup is not canonical garbage
// collection — the journal keeps its own copy either way, which is why a
// carrier that is never cleaned stays correct and merely grows.
func (r *Round) cleanup(ctx context.Context, local syncstate.Handshake, peers map[string]*peerView, currentRequest string, result *Result) error {
	group, err := r.keys.Current()
	if err != nil {
		return err
	}
	current := syncwire.CarrierName(group, "advertisement", local.ReplicaID+"|"+vectorDigest(local.StateVector))
	names, err := r.carrier.List(ctx, r.carrier.Namespace(), ClassAdvertisement)
	if err != nil {
		return err
	}
	for _, name := range names {
		if name == current {
			continue
		}
		if err := r.carrier.Remove(ctx, ClassAdvertisement, name); err != nil {
			return err
		}
		result.Removed++
	}
	// A request this replica no longer makes is withdrawn, including every one
	// of them once it is asking for nothing: a stale request makes a holder
	// republish work it has already served, and leaves the folder describing a
	// need that no longer exists.
	requests, err := r.carrier.List(ctx, r.carrier.Namespace(), ClassRequest)
	if err != nil {
		return err
	}
	for _, name := range requests {
		if name == currentRequest {
			continue
		}
		if err := r.carrier.Remove(ctx, ClassRequest, name); err != nil {
			return err
		}
		result.Removed++
	}
	if len(peers) == 0 {
		// With no peer to acknowledge anything, nothing is expired. A lone
		// replica that deleted its own envelopes here would have to republish
		// them the moment a peer reappeared.
		return nil
	}
	acknowledged := lowestCommonVector(peers)
	envelopes, err := r.carrier.List(ctx, r.carrier.Namespace(), ClassEnvelope)
	if err != nil {
		return err
	}
	for _, name := range envelopes {
		// An envelope's coverage is read from the envelope, not inferred from
		// its name. A name says only that two publications are the same thing;
		// reconstructing what it contains from a naming convention would break
		// the day a batch size changed, and would break silently, by deleting
		// something a peer still needed.
		artifact, err := r.carrier.Read(ctx, r.carrier.Namespace(), ClassEnvelope, name)
		if err != nil {
			continue
		}
		_, plaintext, err := syncwire.Open(r.keys, r.verifier, artifact, r.options.Limits)
		if err != nil {
			continue
		}
		envelope, err := syncwire.DecodeEnvelope(plaintext, r.options.Limits)
		if err != nil {
			continue
		}
		if !fullyAcknowledged(envelope.Operations, acknowledged) {
			continue
		}
		if err := r.carrier.Remove(ctx, ClassEnvelope, name); err != nil {
			return err
		}
		result.Removed++
	}
	return nil
}

// lowestCommonVector is what every enrolled peer has durably admitted. A
// subject missing from any peer's vector is zero there, so one peer that has
// never heard of a replica holds its envelopes on the carrier for everyone.
func lowestCommonVector(peers map[string]*peerView) syncstate.Vector {
	common := syncstate.Vector{}
	for index, replicaID := range sortedPeerIDs(peers) {
		vector := peers[replicaID].handshake.StateVector
		if index == 0 {
			for subject, sequence := range vector {
				common[subject] = sequence
			}
			continue
		}
		for subject := range common {
			sequence, present := vector[subject]
			if !present {
				common[subject] = 0
				continue
			}
			if sequence < common[subject] {
				common[subject] = sequence
			}
		}
	}
	return common
}

func fullyAcknowledged(operations []syncstate.Operation, acknowledged syncstate.Vector) bool {
	for _, operation := range operations {
		if acknowledged[operation.ReplicaID] < operation.Sequence {
			return false
		}
	}
	return len(operations) > 0
}

// publishOnce writes an artifact unless this replica has already published a
// readable one under the same name.
//
// The check is a real read and a real decryption of our own copy, not a
// stat. That is what makes two opposite requirements hold at once: a quiet
// carrier stops growing, because an unchanged artifact is recognized and left
// alone; and a damaged artifact is repaired, because "present" and "readable"
// are not the same question. A publisher that trusted the name alone would
// leave a half-copied envelope in place forever, and the peer waiting for it
// would never be told why.
func (r *Round) publishOnce(ctx context.Context, class Class, name string, kind syncwire.ArtifactKind, plaintext []byte) (bool, error) {
	if existing, err := r.carrier.Read(ctx, r.carrier.Namespace(), class, name); err == nil {
		if _, _, err := syncwire.Open(r.keys, r.verifier, existing, r.options.Limits); err == nil {
			return false, nil
		}
	}
	artifact, err := syncwire.Seal(r.keys, r.signer, kind, name, plaintext, r.options.Limits)
	if err != nil {
		return false, err
	}
	if _, err := r.carrier.Publish(ctx, class, name, artifact); err != nil {
		return false, err
	}
	return true, nil
}

// namespaceOf computes the namespace a replica must be writing under. It is how
// a scan binds a folder name to the identity claimed inside the artifacts it
// holds.
func (r *Round) namespaceOf(replicaID string) string {
	group, err := r.keys.Current()
	if err != nil {
		return ""
	}
	return syncwire.CarrierName(group, "replica", replicaID)
}

func openReason(err error) string {
	switch {
	case errors.Is(err, syncwire.ErrUnknownSigner):
		return "unenrolled_signer"
	case errors.Is(err, syncwire.ErrBadSignature):
		return "bad_signature"
	case errors.Is(err, syncwire.ErrRevokedEpoch):
		return "revoked_epoch"
	case errors.Is(err, syncwire.ErrNoKey):
		return "unknown_key"
	case errors.Is(err, syncwire.ErrDecrypt):
		return "decryption_failed"
	case errors.Is(err, syncwire.ErrLimitExceeded):
		return "over_limit"
	default:
		return "malformed_artifact"
	}
}

func sortedPeerIDs(peers map[string]*peerView) []string {
	ids := make([]string, 0, len(peers))
	for replicaID := range peers {
		ids = append(ids, replicaID)
	}
	sort.Strings(ids)
	return ids
}

func sortedKeys(values map[string]int64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortRanges(ranges []syncstate.SequenceRange) {
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].ReplicaID != ranges[j].ReplicaID {
			return ranges[i].ReplicaID < ranges[j].ReplicaID
		}
		return ranges[i].Start < ranges[j].Start
	})
}
