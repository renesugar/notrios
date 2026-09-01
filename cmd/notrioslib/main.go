// Command notrioslib is the version-1 Notrios C ABI shared library.
//
// Build it as a shared library or a static archive:
//
//	go build -buildmode=c-shared -o libnotrios.so ./cmd/notrioslib
//	go build -buildmode=c-archive -o libnotrios.a ./cmd/notrioslib
//
// This file is deliberately thin. It moves bytes across the C boundary and
// nothing else; every decision — handle validity, bounds, cancellation,
// shutdown ordering, database ownership — is made in internal/abi, which is
// ordinary Go with tests. Go forbids cgo in _test.go files, so anything left
// here is untestable by construction.
//
// Memory rules, which a host must follow exactly:
//
//   - Input pointers are borrowed. They are copied before this function
//     returns and are never retained.
//   - Output buffers are allocated by C and owned by the caller. Release each
//     one exactly once with notrios_buffer_release. Releasing twice, or
//     releasing a pointer this library did not produce, is refused rather than
//     acted on, because a double free here would be a host crash with no
//     diagnosable cause.
//   - No Go pointer is ever written into C-visible memory.
package main

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/renesugar/notrios/internal/abi"
)

func main() {} // required for buildmode=c-shared and c-archive

// issuedBuffers records every pointer handed to C, so a release can be
// validated instead of trusted. A host that frees a foreign or already-freed
// pointer gets an error return; the alternative is a heap corruption whose
// stack trace points nowhere near the bug.
var issuedBuffers = struct {
	sync.Mutex
	sizes map[uintptr]C.size_t
}{sizes: make(map[uintptr]C.size_t)}

// emit copies payload into C memory and records it for release validation.
func emit(payload []byte, outData **C.uchar, outLen *C.size_t) {
	if outData == nil || outLen == nil {
		return
	}
	if len(payload) == 0 {
		*outData = nil
		*outLen = 0
		return
	}
	block := C.CBytes(payload)
	issuedBuffers.Lock()
	issuedBuffers.sizes[uintptr(block)] = C.size_t(len(payload))
	issuedBuffers.Unlock()
	*outData = (*C.uchar)(block)
	*outLen = C.size_t(len(payload))
}

// borrow copies a caller-owned buffer into Go memory.
func borrow(data *C.uchar, length C.size_t, limit int) ([]byte, abi.Status) {
	if data == nil {
		if length != 0 {
			return nil, abi.StatusInvalidArgument
		}
		return nil, abi.StatusOK
	}
	if int(length) > limit {
		return nil, abi.StatusLimitExceeded
	}
	return C.GoBytes(unsafe.Pointer(data), C.int(length)), abi.StatusOK
}

//export notrios_abi_version
func notrios_abi_version() C.uint32_t { return C.uint32_t(abi.Version) }

//export notrios_capabilities
func notrios_capabilities() C.uint64_t { return C.uint64_t(abi.Capabilities) }

//export notrios_instance_open
func notrios_instance_open(profile *C.char, length C.size_t, out *C.uint64_t) C.int32_t {
	if out == nil {
		return C.int32_t(abi.StatusInvalidArgument)
	}
	*out = 0
	if profile == nil || length == 0 || int(length) > abi.MaxProfileBytes {
		return C.int32_t(abi.StatusInvalidArgument)
	}
	path := C.GoStringN(profile, C.int(length))
	handle, status := abi.OpenInstance(path)
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	*out = C.uint64_t(handle)
	return C.int32_t(abi.StatusOK)
}

//export notrios_instance_close
func notrios_instance_close(instance C.uint64_t) C.int32_t {
	return C.int32_t(abi.CloseInstance(abi.Handle(instance)))
}

//export notrios_call_start
func notrios_call_start(instance C.uint64_t, request *C.uchar, length C.size_t, out *C.uint64_t) C.int32_t {
	if out == nil {
		return C.int32_t(abi.StatusInvalidArgument)
	}
	*out = 0
	session, status := abi.LookupSession(abi.Handle(instance))
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	payload, status := borrow(request, length, abi.MaxRequestBytes)
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	handle, status := session.StartCall(payload)
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	*out = C.uint64_t(handle)
	return C.int32_t(abi.StatusOK)
}

//export notrios_call_poll
func notrios_call_poll(instance C.uint64_t, callHandle C.uint64_t, outData **C.uchar, outLen *C.size_t) C.int32_t {
	if outData == nil || outLen == nil {
		return C.int32_t(abi.StatusInvalidArgument)
	}
	*outData = nil
	*outLen = 0
	session, status := abi.LookupSession(abi.Handle(instance))
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	callStatus, payload := session.PollCall(abi.Handle(callHandle))
	if len(payload) > 0 {
		emit(payload, outData, outLen)
	}
	return C.int32_t(callStatus)
}

//export notrios_call_cancel
func notrios_call_cancel(instance C.uint64_t, callHandle C.uint64_t) C.int32_t {
	session, status := abi.LookupSession(abi.Handle(instance))
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	return C.int32_t(session.CancelCall(abi.Handle(callHandle)))
}

//export notrios_buffer_release
func notrios_buffer_release(instance C.uint64_t, data *C.uchar, length C.size_t) C.int32_t {
	// The instance is accepted for symmetry with the rest of the surface but
	// is not required to be live: a host must be able to release a buffer it
	// collected just before closing, and refusing that would force a leak.
	_ = instance
	if data == nil {
		return C.int32_t(abi.StatusInvalidArgument)
	}
	key := uintptr(unsafe.Pointer(data))
	issuedBuffers.Lock()
	recorded, present := issuedBuffers.sizes[key]
	if present {
		delete(issuedBuffers.sizes, key)
	}
	issuedBuffers.Unlock()
	if !present {
		// Either a double release or a pointer from somewhere else. Refusing
		// is the only safe answer; freeing it would corrupt the host heap.
		return C.int32_t(abi.StatusInvalidArgument)
	}
	if length != 0 && length != recorded {
		// A wrong length means the host is confused about which buffer this
		// is. Put the record back and refuse.
		issuedBuffers.Lock()
		issuedBuffers.sizes[key] = recorded
		issuedBuffers.Unlock()
		return C.int32_t(abi.StatusInvalidArgument)
	}
	C.free(unsafe.Pointer(data))
	return C.int32_t(abi.StatusOK)
}

//export notrios_event_poll
func notrios_event_poll(instance C.uint64_t, outData **C.uchar, outLen *C.size_t) C.int32_t {
	if outData == nil || outLen == nil {
		return C.int32_t(abi.StatusInvalidArgument)
	}
	*outData = nil
	*outLen = 0
	session, status := abi.LookupSession(abi.Handle(instance))
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	eventStatus, payload := session.PollEvent()
	if len(payload) > 0 {
		emit(payload, outData, outLen)
	}
	return C.int32_t(eventStatus)
}

//export notrios_stream_open
func notrios_stream_open(instance C.uint64_t, request *C.uchar, length C.size_t, out *C.uint64_t) C.int32_t {
	if out == nil {
		return C.int32_t(abi.StatusInvalidArgument)
	}
	*out = 0
	session, status := abi.LookupSession(abi.Handle(instance))
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	payload, status := borrow(request, length, abi.MaxRequestBytes)
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	handle, status := session.OpenStream(payload)
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	*out = C.uint64_t(handle)
	return C.int32_t(abi.StatusOK)
}

//export notrios_stream_read
func notrios_stream_read(instance C.uint64_t, streamHandle C.uint64_t, max C.size_t, outData **C.uchar, outLen *C.size_t) C.int32_t {
	if outData == nil || outLen == nil {
		return C.int32_t(abi.StatusInvalidArgument)
	}
	*outData = nil
	*outLen = 0
	session, status := abi.LookupSession(abi.Handle(instance))
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	if max > C.size_t(abi.MaxStreamChunkBytes) {
		return C.int32_t(abi.StatusLimitExceeded)
	}
	readStatus, chunk := session.ReadStream(abi.Handle(streamHandle), int(max))
	if len(chunk) > 0 {
		emit(chunk, outData, outLen)
	}
	return C.int32_t(readStatus)
}

//export notrios_stream_close
func notrios_stream_close(instance C.uint64_t, streamHandle C.uint64_t) C.int32_t {
	session, status := abi.LookupSession(abi.Handle(instance))
	if status != abi.StatusOK {
		return C.int32_t(status)
	}
	return C.int32_t(session.CloseStream(abi.Handle(streamHandle)))
}
