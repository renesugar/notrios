package main

// #include <stdint.h>
// #include <stdlib.h>
import "C"

import (
	"sync"
	"time"
	"unsafe"
)

const (
	ok int32 = iota
	invalidArgument
	versionMismatch
	invalidHandle
	staleHandle
	notFound
	conflict
	preconditionRequired
	forbidden
	unavailable
	limitExceeded
	cancelled
	endOfStream
	wouldBlock
	shuttingDown
	internal
)

const (
	maxJSON   = 1 << 20
	maxStream = 1 << 20
	maxEvents = 256
)

type call struct {
	mu        sync.Mutex
	done      chan struct{}
	cancel    chan struct{}
	result    []byte
	status    int32
	delivered bool
	once      sync.Once
}
type stream struct {
	data   []byte
	pos    int
	closed bool
}
type instance struct {
	mu       sync.Mutex
	shutting bool
	calls    map[uint64]*call
	streams  map[uint64]*stream
	events   [][]byte
	wg       sync.WaitGroup
}

var registry = struct {
	sync.Mutex
	next      uint64
	instances map[uint64]*instance
	closed    map[uint64]bool
	buffers   map[uintptr]uint64
}{instances: make(map[uint64]*instance), closed: make(map[uint64]bool), buffers: make(map[uintptr]uint64)}

func handle(m uint64, slot uint32) uint64 { return (m << 32) | uint64(slot) }
func split(h uint64) (uint64, uint32)     { return h >> 32, uint32(h) }
func alloc(b []byte) (*C.uchar, C.size_t) {
	if len(b) == 0 {
		b = []byte{}
	}
	p := C.CBytes(b)
	return (*C.uchar)(p), C.size_t(len(b))
}
func allocOwned(owner uint64, b []byte) (*C.uchar, C.size_t) {
	p, n := alloc(b)
	registry.Lock()
	registry.buffers[uintptr(unsafe.Pointer(p))] = owner
	registry.Unlock()
	return p, n
}
func copyInput(p *C.uchar, n C.size_t) ([]byte, int32) {
	if p == nil && n != 0 {
		return nil, invalidArgument
	}
	if n > maxJSON {
		return nil, limitExceeded
	}
	return C.GoBytes(unsafe.Pointer(p), C.int(n)), ok
}
func get(h uint64) (*instance, int32) {
	if h == 0 {
		return nil, invalidHandle
	}
	registry.Lock()
	x := registry.instances[h]
	registry.Unlock()
	if x == nil {
		return nil, staleHandle
	}
	return x, ok
}

//export notrios_abi_version
func notrios_abi_version() C.uint32_t { return 1 }

//export notrios_capabilities
func notrios_capabilities() C.uint64_t { return 0x1f } // polling, streams, events, cancellation, generation handles

//export notrios_instance_open
func notrios_instance_open(profile *C.char, n C.size_t, out *C.uint64_t) C.int32_t {
	if out == nil || (profile == nil && n != 0) || n == 0 || n > 128 {
		return C.int32_t(invalidArgument)
	}
	// Copy the borrowed profile only to validate its lifetime; it is not retained.
	_ = C.GoBytes(unsafe.Pointer(profile), C.int(n))
	registry.Lock()
	registry.next++
	if registry.next == 0 {
		registry.next++
	}
	h := handle(registry.next, 1)
	registry.instances[h] = &instance{calls: map[uint64]*call{}, streams: map[uint64]*stream{}}
	registry.Unlock()
	*out = C.uint64_t(h)
	return 0
}

//export notrios_instance_close
func notrios_instance_close(h C.uint64_t) C.int32_t {
	if h == 0 {
		return C.int32_t(invalidHandle)
	}
	registry.Lock()
	x := registry.instances[uint64(h)]
	if x == nil {
		if registry.closed[uint64(h)] {
			registry.Unlock()
			return 0
		}
		registry.Unlock()
		return C.int32_t(staleHandle)
	}
	registry.Unlock()
	x.mu.Lock()
	x.shutting = true
	for _, c := range x.calls {
		c.mu.Lock()
		c.status = cancelled
		c.mu.Unlock()
		c.once.Do(func() { close(c.cancel) })
	}
	x.mu.Unlock()
	// Keep the instance registered while workers observe cancellation. This makes
	// concurrent callers receive shutting_down rather than an ambiguous stale
	// handle, then invalidate the generation only after the join.
	time.Sleep(20 * time.Millisecond)
	x.wg.Wait()
	registry.Lock()
	delete(registry.instances, uint64(h))
	registry.closed[uint64(h)] = true
	registry.Unlock()
	return 0
}

//export notrios_call_start
func notrios_call_start(ih C.uint64_t, req *C.uchar, n C.size_t, out *C.uint64_t) C.int32_t {
	if out == nil {
		return C.int32_t(invalidArgument)
	}
	x, s := get(uint64(ih))
	if s != ok {
		return C.int32_t(s)
	}
	b, s := copyInput(req, n)
	if s != ok {
		return C.int32_t(s)
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.shutting {
		return C.int32_t(shuttingDown)
	}
	id := uint64(len(x.calls) + 1)
	h := handle(uint64(ih)>>32, uint32(id))
	c := &call{done: make(chan struct{}), cancel: make(chan struct{}), status: ok}
	x.calls[h] = c
	if len(x.events) < maxEvents {
		x.events = append(x.events, []byte("{\"kind\":\"call_started\"}"))
	}
	x.wg.Add(1)
	*out = C.uint64_t(h)
	go func() {
		defer x.wg.Done()
		select {
		case <-time.After(5 * time.Millisecond):
			c.mu.Lock()
			if c.status == ok {
				c.result = append([]byte(nil), b...)
			}
			c.mu.Unlock()
		case <-c.cancel:
			c.mu.Lock()
			c.status = cancelled
			c.mu.Unlock()
		}
		close(c.done)
	}()
	return 0
}

//export notrios_call_poll
func notrios_call_poll(ih, ch C.uint64_t, out **C.uchar, n *C.size_t) C.int32_t {
	x, s := get(uint64(ih))
	if s != ok {
		return C.int32_t(s)
	}
	if out == nil || n == nil {
		return C.int32_t(invalidArgument)
	}
	x.mu.Lock()
	c := x.calls[uint64(ch)]
	x.mu.Unlock()
	if c == nil {
		return C.int32_t(staleHandle)
	}
	c.mu.Lock()
	if c.delivered {
		c.mu.Unlock()
		return C.int32_t(staleHandle)
	}
	c.mu.Unlock()
	select {
	case <-c.done:
		c.mu.Lock()
		status := c.status
		if status == ok {
			c.delivered = true
		}
		result := append([]byte(nil), c.result...)
		c.mu.Unlock()
		if status != ok {
			return C.int32_t(status)
		}
		p, z := allocOwned(uint64(ih), result)
		*out = p
		*n = z
		return 0
	default:
		return C.int32_t(wouldBlock)
	}
}

//export notrios_call_cancel
func notrios_call_cancel(ih, ch C.uint64_t) C.int32_t {
	x, s := get(uint64(ih))
	if s != ok {
		return C.int32_t(s)
	}
	x.mu.Lock()
	c := x.calls[uint64(ch)]
	x.mu.Unlock()
	if c == nil {
		return C.int32_t(staleHandle)
	}
	c.mu.Lock()
	c.status = cancelled
	c.mu.Unlock()
	c.once.Do(func() { close(c.cancel) })
	return 0
}

//export notrios_buffer_release
func notrios_buffer_release(ih C.uint64_t, p *C.uchar, n C.size_t) C.int32_t {
	if p == nil || n > maxJSON {
		return C.int32_t(invalidArgument)
	}
	registry.Lock()
	owner, okbuf := registry.buffers[uintptr(unsafe.Pointer(p))]
	if !okbuf {
		registry.Unlock()
		return C.int32_t(staleHandle)
	}
	if owner != uint64(ih) {
		registry.Unlock()
		return C.int32_t(invalidHandle)
	}
	delete(registry.buffers, uintptr(unsafe.Pointer(p)))
	registry.Unlock()
	C.free(unsafe.Pointer(p))
	return 0
}

//export notrios_event_poll
func notrios_event_poll(ih C.uint64_t, out **C.uchar, n *C.size_t) C.int32_t {
	x, s := get(uint64(ih))
	if s != ok {
		return C.int32_t(s)
	}
	if out == nil || n == nil {
		return C.int32_t(invalidArgument)
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if len(x.events) == 0 {
		return C.int32_t(wouldBlock)
	}
	b := x.events[0]
	x.events = x.events[1:]
	p, z := allocOwned(uint64(ih), b)
	*out = p
	*n = z
	return 0
}

//export notrios_stream_open
func notrios_stream_open(ih C.uint64_t, data *C.uchar, n C.size_t, out *C.uint64_t) C.int32_t {
	x, s := get(uint64(ih))
	if s != ok {
		return C.int32_t(s)
	}
	if out == nil {
		return C.int32_t(invalidArgument)
	}
	if n > maxJSON {
		return C.int32_t(limitExceeded)
	}
	b, s := copyInput(data, n)
	if s != ok {
		return C.int32_t(s)
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.shutting {
		return C.int32_t(shuttingDown)
	}
	id := uint64(len(x.streams) + 1)
	h := handle(uint64(ih)>>32, uint32(id))
	x.streams[h] = &stream{data: b}
	*out = C.uint64_t(h)
	return 0
}

//export notrios_stream_read
func notrios_stream_read(ih, sh C.uint64_t, max C.size_t, out **C.uchar, n *C.size_t) C.int32_t {
	x, s := get(uint64(ih))
	if s != ok {
		return C.int32_t(s)
	}
	if out == nil || n == nil || max == 0 {
		return C.int32_t(invalidArgument)
	}
	if max > maxStream {
		max = maxStream
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	q := x.streams[uint64(sh)]
	if q == nil || q.closed {
		return C.int32_t(staleHandle)
	}
	if q.pos == len(q.data) {
		*out = nil
		*n = 0
		return C.int32_t(endOfStream)
	}
	e := q.pos + int(max)
	if e > len(q.data) {
		e = len(q.data)
	}
	p, z := allocOwned(uint64(ih), q.data[q.pos:e])
	q.pos = e
	*out = p
	*n = z
	return 0
}

//export notrios_stream_close
func notrios_stream_close(ih, sh C.uint64_t) C.int32_t {
	x, s := get(uint64(ih))
	if s != ok {
		return C.int32_t(s)
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	q := x.streams[uint64(sh)]
	if q == nil {
		return C.int32_t(staleHandle)
	}
	q.closed = true
	delete(x.streams, uint64(sh))
	return 0
}

func main() {}
