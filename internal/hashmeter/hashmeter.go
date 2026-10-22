// Package hashmeter counts the content bytes this process has put through
// SHA-256 (v1.0 J32-AA).
//
// An import hashes a note more than once: the inventory hashes the file, the
// read verifies it against that hash, the fingerprint hashes the canonical body,
// the revision insert hashes the same body again, and block identity hashes each
// block's text. A CPU profile said SHA-256 was a third of a near-limit import
// but could not say how many passes that was, and the arithmetic did not fit
// four — so the passes are counted rather than inferred.
//
// It is a process-wide count and it is content-free: a length and a call, never
// a byte of what was hashed. The cost is one atomic add beside work that is
// already hashing kilobytes.
//
// It lives in its own package because the paths that hash content are in
// internal/store, internal/markdownblocks and the importers, and a counter they
// can all reach cannot sit in any of them.
package hashmeter

import "sync/atomic"

var (
	hashedBytes atomic.Int64
	hashCalls   atomic.Int64
)

// Add records one hash over length bytes.
func Add(length int) {
	hashedBytes.Add(int64(length))
	hashCalls.Add(1)
}

// Bytes is how many bytes this process has hashed.
func Bytes() int64 { return hashedBytes.Load() }

// Calls is how many times it has hashed something.
func Calls() int64 { return hashCalls.Load() }
