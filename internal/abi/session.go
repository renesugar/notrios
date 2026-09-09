package abi

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/renesugar/notrios/internal/application"
)

// Session is one open library: an application facade, the calls and streams a
// host has started against it, and the queued events it has not collected.
//
// Shutdown order is fixed by H0 and implemented in Close:
//
//	refuse new work -> cancel calls -> close streams -> join workers ->
//	close the store -> release the owner lock -> invalidate the generation
//
// The order matters. Releasing the owner lock before the store is closed would
// let a second owner open the database while this one still has it open, which
// is the exact condition the lock exists to prevent. Invalidating the
// generation first would turn a concurrent in-flight call into a confusing
// stale-handle error instead of an honest shutting_down.
type Session struct {
	app   *application.Application
	owner *ownerLock
	// closeStore releases whatever backs the facade. It is separate from the
	// facade because the facade deliberately does not expose its repository.
	closeStore func() error

	mu       sync.Mutex
	shutting bool

	calls   *table[*Call]
	streams *table[*Stream]

	eventsMu sync.Mutex
	events   [][]byte
	// eventsDropped counts events discarded because a host stopped polling.
	// Reporting the count is more useful than either blocking or silence.
	eventsDropped uint64

	workers sync.WaitGroup
}

// NewSession builds a session over an existing facade. closeStore may be nil.
func NewSession(app *application.Application, owner *ownerLock, closeStore func() error) *Session {
	return &Session{
		app:        app,
		owner:      owner,
		closeStore: closeStore,
		calls:      newTable[*Call](),
		streams:    newTable[*Stream](),
	}
}

// Call is one in-flight or completed request.
type Call struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu     sync.Mutex
	status Status
	result []byte
	// collected marks a result already handed to the host. The buffer is C's
	// after that, and handing out a second pointer to the same bytes would
	// invite a double free.
	collected bool
}

// Stream is a bounded reader over one resource.
type Stream struct {
	mu     sync.Mutex
	source *application.ResourceStream
	closed bool
}

// StartCall begins an operation and returns its handle immediately. The work
// runs on a goroutine so a host is never blocked inside the ABI; results are
// collected with PollCall.
func (s *Session) StartCall(request []byte) (Handle, Status) {
	if len(request) == 0 {
		return 0, StatusInvalidArgument
	}
	if len(request) > MaxRequestBytes {
		return 0, StatusLimitExceeded
	}

	s.mu.Lock()
	if s.shutting {
		s.mu.Unlock()
		return 0, StatusShuttingDown
	}
	s.workers.Add(1)
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	call := &Call{cancel: cancel, done: make(chan struct{}), status: StatusWouldBlock}
	handle := s.calls.insert(call)

	go func() {
		defer s.workers.Done()
		defer close(call.done)
		status, payload := s.dispatch(ctx, request)
		call.mu.Lock()
		call.status = status
		call.result = payload
		call.mu.Unlock()
	}()
	return handle, StatusOK
}

// PollCall reports a call's state. StatusWouldBlock means the host should poll
// again; any other status means the call is finished and, on success, that the
// returned bytes are now the host's to release.
func (s *Session) PollCall(handle Handle) (Status, []byte) {
	call, status := s.calls.lookup(handle)
	if status != StatusOK {
		return status, nil
	}
	select {
	case <-call.done:
	default:
		return StatusWouldBlock, nil
	}

	call.mu.Lock()
	defer call.mu.Unlock()
	if call.collected {
		// The result already crossed the boundary. Returning it twice would
		// hand out two owning pointers to one allocation.
		return StatusInvalidArgument, nil
	}
	call.collected = true
	result := call.result
	call.result = nil
	callStatus := call.status

	// A collected call is finished; release its slot so a long-running host
	// does not accumulate handles.
	s.calls.remove(handle)
	return callStatus, result
}

// CancelCall asks a call to stop. It returns once cancellation is signalled,
// not once the call has noticed: a host that must know it finished polls.
func (s *Session) CancelCall(handle Handle) Status {
	call, status := s.calls.lookup(handle)
	if status != StatusOK {
		return status
	}
	call.cancel()
	return StatusOK
}

// PollEvent removes and returns the oldest queued event.
func (s *Session) PollEvent() (Status, []byte) {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	if len(s.events) == 0 {
		return StatusWouldBlock, nil
	}
	event := s.events[0]
	s.events = s.events[1:]
	return StatusOK, event
}

// PublishEvent queues an event for the host. The queue is bounded; the oldest
// event is dropped when it is full, and the drop is counted rather than hidden.
func (s *Session) PublishEvent(event []byte) {
	if len(event) == 0 || len(event) > MaxRequestBytes {
		return
	}
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	if len(s.events) >= MaxQueuedEvents {
		s.events = s.events[1:]
		s.eventsDropped++
	}
	s.events = append(s.events, event)
}

// EventsDropped reports how many events were discarded because the host stopped
// polling.
func (s *Session) EventsDropped() uint64 {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	return s.eventsDropped
}

// OpenStream begins a bounded read over a resource.
func (s *Session) OpenStream(request []byte) (Handle, Status) {
	if len(request) == 0 || len(request) > MaxRequestBytes {
		if len(request) > MaxRequestBytes {
			return 0, StatusLimitExceeded
		}
		return 0, StatusInvalidArgument
	}
	s.mu.Lock()
	if s.shutting {
		s.mu.Unlock()
		return 0, StatusShuttingDown
	}
	s.mu.Unlock()

	var parsed struct {
		ResourceID string `json:"resource_id"`
		MaxBytes   int64  `json:"max_bytes"`
	}
	if err := json.Unmarshal(request, &parsed); err != nil {
		return 0, StatusInvalidArgument
	}
	if parsed.MaxBytes <= 0 {
		return 0, StatusInvalidArgument
	}

	_, source, err := s.app.OpenResource(context.Background(), parsed.ResourceID, parsed.MaxBytes)
	if err != nil {
		return 0, statusForError(err)
	}
	return s.streams.insert(&Stream{source: source}), StatusOK
}

// ReadStream returns up to max bytes. StatusEndOfStream means the stream is
// exhausted; the handle stays valid so the host still calls close exactly once.
func (s *Session) ReadStream(handle Handle, max int) (Status, []byte) {
	if max <= 0 {
		return StatusInvalidArgument, nil
	}
	if max > MaxStreamChunkBytes {
		return StatusLimitExceeded, nil
	}
	stream, status := s.streams.lookup(handle)
	if status != StatusOK {
		return status, nil
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed {
		return StatusStaleHandle, nil
	}
	buffer := make([]byte, max)
	read, err := stream.source.Read(buffer)
	if read > 0 {
		return StatusOK, buffer[:read]
	}
	if err != nil {
		if isEOF(err) {
			return StatusEndOfStream, nil
		}
		return statusForError(err), nil
	}
	return StatusOK, nil
}

// CloseStream releases a stream. A second close reports StatusStaleHandle
// rather than closing the underlying reader again.
func (s *Session) CloseStream(handle Handle) Status {
	stream, status := s.streams.remove(handle)
	if status != StatusOK {
		return status
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed {
		return StatusStaleHandle
	}
	stream.closed = true
	_ = stream.source.Close()
	return StatusOK
}

// Close shuts the session down in the order H0 fixed. It is safe to call more
// than once; the second call reports StatusStaleHandle to its caller through
// the registry rather than repeating the teardown.
func (s *Session) Close() Status {
	s.mu.Lock()
	if s.shutting {
		s.mu.Unlock()
		return StatusShuttingDown
	}
	// 1. Refuse new work. In-flight callers now see shutting_down instead of
	//    an ambiguous stale handle.
	s.shutting = true
	s.mu.Unlock()

	// 2. Cancel every call.
	for _, call := range s.calls.values() {
		call.cancel()
	}
	// 3. Close every stream, so no reader is left holding the store open.
	for _, stream := range s.streams.values() {
		stream.mu.Lock()
		if !stream.closed {
			stream.closed = true
			_ = stream.source.Close()
		}
		stream.mu.Unlock()
	}
	// 4. Join workers. Nothing may touch the store after this point.
	s.workers.Wait()
	// 5. Close the store, checkpointing SQLite.
	if s.closeStore != nil {
		_ = s.closeStore()
	}
	// 6. Release the owner lock only now. Releasing earlier would let a second
	//    owner open the database while this one still had it open.
	s.owner.release()
	return StatusOK
}

// IsShuttingDown reports whether Close has begun.
func (s *Session) IsShuttingDown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutting
}
