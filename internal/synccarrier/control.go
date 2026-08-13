package synccarrier

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/renesugar/notrios/internal/syncstate"
)

// Control-message bounds. A control message is decoded from bytes that have
// already been authenticated, so these guard against a peer that is enrolled
// and wrong rather than against a stranger — which is exactly the case a
// protocol usually forgets.
const (
	MaxRequestedObjects  = 1_024
	MaxRequestedOrdinals = 4_096
	MaxControlBytes      = 1 << 20
)

// ErrControl reports a control message that is malformed or out of bounds.
var ErrControl = errors.New("malformed carrier control message")

// Advertisement is what a replica says about itself: its compatibility and its
// contiguous state vector.
//
// The vector is also the acknowledgement. A G5 vector entry for a peer is by
// construction the highest sequence of that peer this replica has durably
// admitted, so publishing a separate acknowledgement artifact would create a
// second source for one fact — and two sources that can disagree are worse than
// one that is occasionally stale. G11's illustrative layout in
// SYNCHRONIZATION.md listed both; this is the deviation, and it is deliberate.
type Advertisement struct {
	Handshake syncstate.Handshake `json:"handshake"`
	// PublishedAt is observation metadata. Nothing in the protocol orders
	// anything by it: a carrier's clock, and a peer's, are both untrusted.
	PublishedAt string `json:"published_at"`
}

// ObjectWant names resource bytes a replica is missing. Empty Ordinals asks for
// the manifest and every chunk.
type ObjectWant struct {
	BlobSHA256 string `json:"blob_sha256"`
	Ordinals   []int  `json:"ordinals,omitempty"`
}

// Request asks for what a state vector cannot express: resource object bytes,
// and sequence ranges from a replica whose own carrier state was lost.
//
// Ranges are redundant while every peer can read every advertisement — a holder
// derives what a peer lacks by comparing vectors. They matter when a carrier is
// recreated empty and the requester's own advertisement has not yet been seen,
// which is precisely the case G11 has to survive.
type Request struct {
	DatabaseID         string                    `json:"database_id"`
	RequesterReplicaID string                    `json:"requester_replica_id"`
	Ranges             []syncstate.SequenceRange `json:"ranges,omitempty"`
	Objects            []ObjectWant              `json:"objects,omitempty"`
	CreatedAt          string                    `json:"created_at"`
}

// NewAdvertisement builds this replica's advertisement.
func NewAdvertisement(handshake syncstate.Handshake, now time.Time) Advertisement {
	return Advertisement{Handshake: handshake, PublishedAt: now.UTC().Format(time.RFC3339Nano)}
}

// EncodeAdvertisement produces the plaintext that gets sealed.
func EncodeAdvertisement(advertisement Advertisement) ([]byte, error) {
	if advertisement.Handshake.DatabaseID == "" || advertisement.Handshake.ReplicaID == "" {
		return nil, fmt.Errorf("%w: an advertisement names its database and replica", ErrControl)
	}
	if err := syncstate.ValidateVector(advertisement.Handshake.StateVector); err != nil {
		return nil, err
	}
	return json.Marshal(advertisement)
}

// DecodeAdvertisement parses and validates one advertisement.
//
// Unknown fields are ignored rather than refused: a later protocol minor
// version must be able to add one without every older replica treating a peer
// as broken. What is not tolerated is a malformed vector, because that reaches
// admission.
func DecodeAdvertisement(plaintext []byte) (Advertisement, error) {
	if len(plaintext) > MaxControlBytes {
		return Advertisement{}, fmt.Errorf("%w: %d bytes", ErrControl, len(plaintext))
	}
	var advertisement Advertisement
	if err := json.Unmarshal(plaintext, &advertisement); err != nil {
		return Advertisement{}, fmt.Errorf("%w: %v", ErrControl, err)
	}
	if advertisement.Handshake.DatabaseID == "" || advertisement.Handshake.ReplicaID == "" {
		return Advertisement{}, fmt.Errorf("%w: an advertisement names its database and replica", ErrControl)
	}
	if len(advertisement.Handshake.StateVector) > syncstate.MaxStateVectorEntries {
		return Advertisement{}, fmt.Errorf("%w: %d vector entries", ErrControl, len(advertisement.Handshake.StateVector))
	}
	if err := syncstate.ValidateVector(advertisement.Handshake.StateVector); err != nil {
		return Advertisement{}, err
	}
	return advertisement, nil
}

// EncodeRequest produces the plaintext that gets sealed.
func EncodeRequest(request Request) ([]byte, error) {
	if request.DatabaseID == "" || request.RequesterReplicaID == "" {
		return nil, fmt.Errorf("%w: a request names its database and requester", ErrControl)
	}
	if len(request.Objects) > MaxRequestedObjects {
		return nil, fmt.Errorf("%w: %d requested objects", ErrControl, len(request.Objects))
	}
	if len(request.Ranges) > syncstate.MaxMissingRanges {
		return nil, fmt.Errorf("%w: %d requested ranges", ErrControl, len(request.Ranges))
	}
	return json.Marshal(request)
}

// DecodeRequest parses and validates one request, including the bounds that
// stop a peer from asking for more work than a round is allowed to do.
func DecodeRequest(plaintext []byte) (Request, error) {
	if len(plaintext) > MaxControlBytes {
		return Request{}, fmt.Errorf("%w: %d bytes", ErrControl, len(plaintext))
	}
	var request Request
	if err := json.Unmarshal(plaintext, &request); err != nil {
		return Request{}, fmt.Errorf("%w: %v", ErrControl, err)
	}
	if request.DatabaseID == "" || request.RequesterReplicaID == "" {
		return Request{}, fmt.Errorf("%w: a request names its database and requester", ErrControl)
	}
	if len(request.Objects) > MaxRequestedObjects || len(request.Ranges) > syncstate.MaxMissingRanges {
		return Request{}, fmt.Errorf("%w: request exceeds its bounds", ErrControl)
	}
	ordinals := 0
	for _, want := range request.Objects {
		if len(want.BlobSHA256) != 2*sha256.Size || !lowercaseHex(want.BlobSHA256) {
			return Request{}, fmt.Errorf("%w: a requested object is not a content hash", ErrControl)
		}
		ordinals += len(want.Ordinals)
		if ordinals > MaxRequestedOrdinals {
			return Request{}, fmt.Errorf("%w: too many requested chunks", ErrControl)
		}
		for _, ordinal := range want.Ordinals {
			if ordinal < 0 {
				return Request{}, fmt.Errorf("%w: negative chunk ordinal", ErrControl)
			}
		}
	}
	for _, span := range request.Ranges {
		if span.ReplicaID == "" || span.Start < 1 || span.End < span.Start {
			return Request{}, fmt.Errorf("%w: invalid requested range", ErrControl)
		}
	}
	return request, nil
}

// vectorDigest names a state vector so an advertisement whose contents have not
// changed is published under the name it already has.
//
// This is what keeps the carrier bounded without any local bookkeeping: a
// replica that syncs every minute and writes nothing new republishes the same
// name, the carrier already holds that file, and the publish is a no-op. Naming
// artifacts by their sealed bytes instead would produce a new file every round
// forever, because every seal draws a fresh salt.
func vectorDigest(vector syncstate.Vector) string {
	replicas := make([]string, 0, len(vector))
	for replicaID := range vector {
		replicas = append(replicas, replicaID)
	}
	sort.Strings(replicas)
	digest := sha256.New()
	for _, replicaID := range replicas {
		digest.Write([]byte(replicaID))
		digest.Write([]byte{0})
		digest.Write([]byte(strconv.FormatInt(vector[replicaID], 10)))
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// requestDigest names a request by its content for the same reason.
func requestDigest(request Request) string {
	digest := sha256.New()
	digest.Write([]byte(request.RequesterReplicaID))
	digest.Write([]byte{0})
	for _, span := range request.Ranges {
		fmt.Fprintf(digest, "%s:%d-%d\x00", span.ReplicaID, span.Start, span.End)
	}
	for _, want := range request.Objects {
		digest.Write([]byte(want.BlobSHA256))
		for _, ordinal := range want.Ordinals {
			fmt.Fprintf(digest, ":%d", ordinal)
		}
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}
