// Package abi implements the version-1 Notrios C ABI over the transport-neutral
// application facade.
//
// The C-visible surface lives in cmd/notrioslib, which does nothing but move
// bytes across the boundary. Everything that can be wrong — handle validity,
// generation staleness, bounds, cancellation, shutdown ordering, database
// ownership — is decided here, in ordinary Go that can be tested. Go forbids
// cgo in _test.go files, so logic left in the cgo file would be logic nobody
// could write a test for.
//
// Three rules shape the design and are worth stating before the code:
//
//   - No Go pointer and no live store or service object ever becomes visible to
//     C. Callers hold integers. A handle is looked up under a lock and the
//     object behind it never leaves this package.
//   - Nothing calls back into C from a Go thread. Hosts poll; the ABI never
//     hands control to caller code from inside a Go runtime thread.
//   - The surface is twelve functions, not one per REST route. Operations are
//     named in a bounded JSON request, so adding an operation is not an ABI
//     change and ABI major 1 can stay frozen.
package abi

import (
	"context"
	"errors"

	"github.com/renesugar/notrios/internal/application"
)

// Status is the integer every ABI function returns. The values are frozen at
// ABI major 1: a host compiled against this contract must keep working, so
// numbers are never reordered, reused, or removed. New conditions get new
// numbers at the end and a capability bit if a host needs to detect them.
type Status int32

const (
	StatusOK Status = iota
	StatusInvalidArgument
	StatusVersionMismatch
	StatusInvalidHandle
	StatusStaleHandle
	StatusNotFound
	StatusConflict
	StatusPreconditionRequired
	StatusForbidden
	StatusUnavailable
	StatusLimitExceeded
	StatusCancelled
	StatusEndOfStream
	StatusWouldBlock
	StatusShuttingDown
	StatusInternal
)

var statusNames = map[Status]string{
	StatusOK:                   "ok",
	StatusInvalidArgument:      "invalid_argument",
	StatusVersionMismatch:      "version_mismatch",
	StatusInvalidHandle:        "invalid_handle",
	StatusStaleHandle:          "stale_handle",
	StatusNotFound:             "not_found",
	StatusConflict:             "conflict",
	StatusPreconditionRequired: "precondition_required",
	StatusForbidden:            "forbidden",
	StatusUnavailable:          "unavailable",
	StatusLimitExceeded:        "limit_exceeded",
	StatusCancelled:            "cancelled",
	StatusEndOfStream:          "end_of_stream",
	StatusWouldBlock:           "would_block",
	StatusShuttingDown:         "shutting_down",
	StatusInternal:             "internal",
}

func (s Status) String() string {
	if name, ok := statusNames[s]; ok {
		return name
	}
	return "internal"
}

// ABI contract constants. Version and capability bits are part of what a host
// compiles against, so they live beside the status codes they travel with.
const (
	// Version is the ABI major number reported by notrios_abi_version.
	Version uint32 = 1

	// Capability bits reported by notrios_capabilities. A host tests bits
	// rather than assuming a feature exists, which is how ABI major 1 can gain
	// abilities without becoming ABI major 2.
	CapabilityPolling          uint64 = 1 << 0
	CapabilityStreams          uint64 = 1 << 1
	CapabilityEvents           uint64 = 1 << 2
	CapabilityCancellation     uint64 = 1 << 3
	CapabilityGenerationHandle uint64 = 1 << 4

	Capabilities = CapabilityPolling | CapabilityStreams | CapabilityEvents |
		CapabilityCancellation | CapabilityGenerationHandle

	// MaxRequestBytes bounds one JSON request or response. A resource that
	// does not fit belongs in a stream; putting a blob in a JSON result would
	// make every host allocate the whole thing.
	MaxRequestBytes = 1 << 20

	// MaxStreamChunkBytes bounds one stream read, so a host chooses its own
	// memory ceiling and a hostile length cannot be honoured.
	MaxStreamChunkBytes = 1 << 20

	// MaxProfileBytes bounds the profile path accepted by instance_open.
	MaxProfileBytes = 4096

	// MaxQueuedEvents bounds the per-instance event queue. A host that stops
	// polling must not be able to grow this without limit; the oldest events
	// are dropped and the drop is reported.
	MaxQueuedEvents = 256
)

// statusForError maps a facade error onto the frozen status set. It is the
// only place that translation happens.
func statusForError(err error) Status {
	if err == nil {
		return StatusOK
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return StatusCancelled
	}
	switch application.KindOf(err) {
	case application.KindNotFound:
		return StatusNotFound
	case application.KindConflict:
		return StatusConflict
	case application.KindPreconditionRequired:
		return StatusPreconditionRequired
	case application.KindInvalidInput, application.KindInvalidCursor:
		return StatusInvalidArgument
	case application.KindNameConflict:
		return StatusConflict
	case application.KindForbidden:
		return StatusForbidden
	case application.KindUnavailable:
		return StatusUnavailable
	case application.KindCanceled:
		return StatusCancelled
	default:
		return StatusInternal
	}
}
