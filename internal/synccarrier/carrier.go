// Package synccarrier implements v0.7 G11: the ephemeral shared-directory
// carrier and the sync round that drives it.
//
// The directory is a disposable postbox, not a database and not an authority.
// Every byte in it is a G9 artifact — encrypted, signed, and self-describing —
// and every fact it carries also exists in the publisher's durable journal.
// Deleting the whole directory therefore loses nothing: the peers republish.
//
// Three rules shape the whole design, and each of them is a refusal to trust
// the filesystem:
//
//  1. A file's name is derived from its own bytes or from a keyed blind, so a
//     reader checks what it read rather than believing a name, an mtime, a
//     size, or a lock. Torn writes, stale caches, and non-atomic renames are
//     then ordinary cases rather than corruption.
//  2. Every writable path lives inside the writing replica's own namespace.
//     No peer overwrites another's file, and no shared mutable manifest exists
//     to be raced for.
//  3. Correctness never depends on a filesystem watcher or on listing order.
//     A round is an explicit scan; watching can only make it happen sooner.
package synccarrier

import (
	"context"
	"errors"
)

// Class names one kind of artifact directory inside a replica's namespace.
// The class is visible to the carrier because a scanner has to know which
// directory to read; G0's leakage budget permits the artifact class and nothing
// else about the content.
type Class string

const (
	// ClassAdvertisement holds a replica's signed handshake: who it is, what it
	// is compatible with, and its contiguous state vector. The vector doubles as
	// an acknowledgement, because a G5 vector entry for a peer is by
	// construction the highest sequence of that peer this replica has durably
	// admitted. Publishing it twice would create two sources for one fact.
	ClassAdvertisement Class = "advertisements"
	// ClassEnvelope holds sealed NEV1 envelopes of journal operations.
	ClassEnvelope Class = "envelopes"
	// ClassRequest holds signed requests for what a vector cannot express:
	// resource object bytes, and sequence ranges a peer lost with its carrier.
	ClassRequest Class = "requests"
	// ClassObject holds sealed resource chunks and manifests, addressed by a
	// keyed blind of the content hash so the carrier never sees a plaintext
	// hash it could compare against a file it already has.
	ClassObject Class = "objects"
	// ClassSnapshot holds G10's signed catch-up requests and offers.
	ClassSnapshot Class = "snapshots"
)

var classes = map[Class]bool{
	ClassAdvertisement: true, ClassEnvelope: true,
	ClassRequest: true, ClassObject: true, ClassSnapshot: true,
}

var (
	// ErrUnreadable reports a carrier entry that could not be read as an
	// artifact — truncated, torn, renamed non-atomically, or simply not ours.
	// It is a skip, never a failure of the round: a carrier is allowed to be
	// in a bad state, and a peer that stopped working because of one would hand
	// the carrier control over its availability.
	ErrUnreadable = errors.New("carrier entry is not a readable artifact")
	// ErrNotOwned reports an attempt to write outside this replica's namespace.
	ErrNotOwned = errors.New("carrier path is not inside this replica's namespace")
	// ErrCarrierUnavailable reports a carrier root that cannot be reached —
	// an unmounted drive, a disconnected share, a deleted directory.
	ErrCarrierUnavailable = errors.New("carrier is not available")
	// ErrTooLarge reports an entry larger than the protocol permits. It is
	// checked before the bytes are read, which is the point of checking it.
	ErrTooLarge = errors.New("carrier entry exceeds the artifact size limit")
)

// Carrier is the transport-neutral surface a sync round uses. G11 implements it
// over a shared directory; G14's REST data plane implements the same operations
// over HTTP, which is why the round below never touches a file path.
type Carrier interface {
	// Initialize creates whatever skeleton the carrier needs. Any enrolled
	// replica may do this: no peer owns the carrier, and a carrier that had an
	// owner would stop working when that peer was retired.
	Initialize(ctx context.Context) error
	// Publish writes one artifact into this replica's own namespace and returns
	// the name it was stored under.
	Publish(ctx context.Context, class Class, name string, artifact []byte) (string, error)
	// Namespaces lists the replica namespaces present, including this replica's
	// own. Names are opaque; a scanner learns who a namespace belongs to by
	// opening its advertisement, not by reading its name.
	Namespaces(ctx context.Context) ([]string, error)
	// List names the artifacts in one namespace and class, in a deterministic
	// order this method imposes rather than one the filesystem supplied.
	List(ctx context.Context, namespace string, class Class) ([]string, error)
	// Read returns one artifact's bytes, or ErrUnreadable.
	Read(ctx context.Context, namespace string, class Class, name string) ([]byte, error)
	// Remove deletes one of this replica's own artifacts. A carrier
	// implementation must refuse any other namespace: cleanup is not a licence
	// to delete a peer's only copy of something.
	Remove(ctx context.Context, class Class, name string) error
	// Namespace returns this replica's own namespace name.
	Namespace() string
}
