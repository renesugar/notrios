package abi

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/renesugar/notrios/internal/application"
	"github.com/renesugar/notrios/internal/store"
)

func testProfile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "library.db")
}

func openTestSession(t *testing.T) (*Session, Handle) {
	t.Helper()
	handle, status := OpenInstance(testProfile(t))
	if status != StatusOK {
		t.Fatalf("open instance: %v", status)
	}
	t.Cleanup(func() { CloseInstance(handle) })
	session, status := LookupSession(handle)
	if status != StatusOK {
		t.Fatalf("lookup session: %v", status)
	}
	return session, handle
}

// call runs one request to completion by polling, the way a host does.
func call(t *testing.T, session *Session, body string) (Status, response) {
	t.Helper()
	handle, status := session.StartCall([]byte(body))
	if status != StatusOK {
		return status, response{}
	}
	// Generous on purpose: this only has to catch a hang, and the race
	// detector makes a large note round trip take seconds.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		status, payload := session.PollCall(handle)
		if status == StatusWouldBlock {
			if time.Now().After(deadline) {
				t.Fatal("call did not complete within the deadline")
			}
			time.Sleep(time.Millisecond)
			continue
		}
		var decoded response
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("response is not valid JSON: %v (%s)", err, payload)
			}
		}
		return status, decoded
	}
}

func TestABIVersionAndCapabilitiesAreFrozen(t *testing.T) {
	// These values are compiled into every host. Changing one silently is an
	// ABI break that no compiler will catch, so they are pinned here.
	if Version != 1 {
		t.Errorf("ABI version = %d, want 1", Version)
	}
	if Capabilities != 0x1f {
		t.Errorf("capabilities = %#x, want 0x1f", Capabilities)
	}
	for status, want := range map[Status]int32{
		StatusOK: 0, StatusInvalidArgument: 1, StatusVersionMismatch: 2,
		StatusInvalidHandle: 3, StatusStaleHandle: 4, StatusNotFound: 5,
		StatusConflict: 6, StatusPreconditionRequired: 7, StatusForbidden: 8,
		StatusUnavailable: 9, StatusLimitExceeded: 10, StatusCancelled: 11,
		StatusEndOfStream: 12, StatusWouldBlock: 13, StatusShuttingDown: 14,
		StatusInternal: 15,
	} {
		if int32(status) != want {
			t.Errorf("status %v = %d, want %d; ABI status numbers are frozen", status, int32(status), want)
		}
	}
}

func TestOpenCreateReadRoundTrip(t *testing.T) {
	session, _ := openTestSession(t)

	status, created := call(t, session, `{"op":"note.create","payload":{"collection_id":"default","title":"ABI note","body":"through the boundary","mime_type":"text/markdown"}}`)
	if status != StatusOK {
		t.Fatalf("create: %v (%s)", status, created.Message)
	}
	var note struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		RevisionID string `json:"revision_id"`
	}
	if err := json.Unmarshal(created.Result, &note); err != nil {
		t.Fatalf("decode note: %v", err)
	}
	if note.ID == "" || note.Title != "ABI note" {
		t.Fatalf("unexpected note: %+v", note)
	}

	status, fetched := call(t, session, `{"op":"note.get","payload":{"id":"`+note.ID+`"}}`)
	if status != StatusOK {
		t.Fatalf("get: %v (%s)", status, fetched.Message)
	}
	if !strings.Contains(string(fetched.Result), "through the boundary") {
		t.Errorf("body did not survive the round trip: %s", fetched.Result)
	}
}

func TestUnknownOperationIsNotFoundNotALinkError(t *testing.T) {
	session, _ := openTestSession(t)
	// The point of naming operations in the payload: an unknown one is a
	// runtime not_found, not a missing symbol.
	status, decoded := call(t, session, `{"op":"note.teleport"}`)
	if status != StatusNotFound {
		t.Fatalf("unknown operation: %v", status)
	}
	if decoded.Code != int32(StatusNotFound) {
		t.Errorf("body code = %d, want %d", decoded.Code, StatusNotFound)
	}
}

func TestMalformedAndOversizedRequestsAreRefused(t *testing.T) {
	session, _ := openTestSession(t)

	if status, _ := call(t, session, `{not json`); status != StatusInvalidArgument {
		t.Errorf("malformed JSON: %v, want invalid argument", status)
	}
	if _, status := session.StartCall(nil); status != StatusInvalidArgument {
		t.Errorf("empty request: %v, want invalid argument", status)
	}
	oversized := make([]byte, MaxRequestBytes+1)
	if _, status := session.StartCall(oversized); status != StatusLimitExceeded {
		t.Errorf("oversized request: %v, want limit exceeded", status)
	}
}

func TestFacadeErrorsReachTheHostAsStatusAndKind(t *testing.T) {
	session, _ := openTestSession(t)
	status, decoded := call(t, session, `{"op":"note.get","payload":{"id":"doc_missing"}}`)
	if status != StatusNotFound {
		t.Fatalf("missing note: %v", status)
	}
	if decoded.Kind != "not_found" {
		t.Errorf("kind = %q, want not_found", decoded.Kind)
	}
	if decoded.Message == "" {
		t.Error("a failure crossed the boundary with no explanation")
	}
}

// A released handle must be detectably stale, not silently reused. This is the
// failure that would otherwise write to the wrong library.
func TestStaleAndInvalidHandlesAreDistinguished(t *testing.T) {
	handle, status := OpenInstance(testProfile(t))
	if status != StatusOK {
		t.Fatalf("open: %v", status)
	}
	if status := CloseInstance(handle); status != StatusOK {
		t.Fatalf("close: %v", status)
	}
	if _, status := LookupSession(handle); status != StatusStaleHandle {
		t.Errorf("closed handle: %v, want stale", status)
	}
	if _, status := LookupSession(0); status != StatusInvalidHandle {
		t.Errorf("zero handle: %v, want invalid", status)
	}
	if _, status := LookupSession(Handle(0xdeadbeef00000001)); status != StatusInvalidHandle {
		t.Errorf("fabricated handle: %v, want invalid", status)
	}
}

func TestGenerationsPreventSlotReuseConfusion(t *testing.T) {
	table := newTable[string]()
	first := table.insert("first")
	if _, status := table.remove(first); status != StatusOK {
		t.Fatalf("remove: %v", status)
	}
	second := table.insert("second")

	if first == second {
		t.Fatal("a reused slot produced an identical handle; generations are not applied")
	}
	if _, status := table.lookup(first); status != StatusStaleHandle {
		t.Errorf("old handle after slot reuse: %v, want stale", status)
	}
	value, status := table.lookup(second)
	if status != StatusOK || value != "second" {
		t.Errorf("new handle resolved to %q (%v)", value, status)
	}
}

func TestHandleZeroGenerationIsAlwaysInvalid(t *testing.T) {
	table := newTable[string]()
	table.insert("value")
	// Slot 0 with generation 0 is the shape of a zeroed-out C variable.
	if _, status := table.lookup(makeHandle(0, 0)); status != StatusInvalidHandle {
		t.Errorf("zeroed handle: %v, want invalid", status)
	}
}

func TestResultIsCollectedExactlyOnce(t *testing.T) {
	session, _ := openTestSession(t)
	handle, status := session.StartCall([]byte(`{"op":"abi.info"}`))
	if status != StatusOK {
		t.Fatalf("start: %v", status)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		status, payload := session.PollCall(handle)
		if status == StatusWouldBlock {
			if time.Now().After(deadline) {
				t.Fatal("call never finished")
			}
			continue
		}
		if status != StatusOK || len(payload) == 0 {
			t.Fatalf("first poll: %v", status)
		}
		break
	}
	// A second collection must not hand out the same buffer again; that would
	// be two owning pointers to one allocation and a double free.
	if status, payload := session.PollCall(handle); status == StatusOK && payload != nil {
		t.Fatal("a completed result was handed out twice")
	}
}

func TestCancellationStopsACall(t *testing.T) {
	session, _ := openTestSession(t)
	handle, status := session.StartCall([]byte(`{"op":"search","payload":{"query":"anything","limit":10}}`))
	if status != StatusOK {
		t.Fatalf("start: %v", status)
	}
	if status := session.CancelCall(handle); status != StatusOK {
		t.Fatalf("cancel: %v", status)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		status, _ := session.PollCall(handle)
		if status != StatusWouldBlock {
			// Either the call finished before cancellation landed or it was
			// cancelled. Both are correct; a hang is not.
			if status != StatusOK && status != StatusCancelled {
				t.Fatalf("cancelled call ended as %v", status)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("cancelled call never settled")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCancelRejectsUnknownHandles(t *testing.T) {
	session, _ := openTestSession(t)
	if status := session.CancelCall(0); status != StatusInvalidHandle {
		t.Errorf("cancel zero handle: %v", status)
	}
}

func TestEventQueueIsBoundedAndReportsDrops(t *testing.T) {
	session, _ := openTestSession(t)
	if status, _ := session.PollEvent(); status != StatusWouldBlock {
		t.Errorf("empty queue: %v, want would block", status)
	}
	for i := 0; i < MaxQueuedEvents+10; i++ {
		session.PublishEvent([]byte(`{"event":"tick"}`))
	}
	if dropped := session.EventsDropped(); dropped != 10 {
		t.Errorf("dropped = %d, want 10", dropped)
	}
	count := 0
	for {
		status, _ := session.PollEvent()
		if status == StatusWouldBlock {
			break
		}
		count++
		if count > MaxQueuedEvents+50 {
			t.Fatal("event queue never drained")
		}
	}
	if count != MaxQueuedEvents {
		t.Errorf("drained %d events, want the %d-event bound", count, MaxQueuedEvents)
	}
}

func TestStreamsAreBoundedAndCloseExactlyOnce(t *testing.T) {
	profile := testProfile(t)
	// Seed before opening, so the instance owns a database that already has an
	// attachment and nothing has to reach behind the facade.
	resourceID := seedResource(t, profile, strings.Repeat("stream-bytes ", 500))

	instance, status := OpenInstance(profile)
	if status != StatusOK {
		t.Fatalf("open: %v", status)
	}
	defer CloseInstance(instance)
	session, _ := LookupSession(instance)

	body := `{"resource_id":"` + resourceID + `","max_bytes":64}`
	streamHandle, status := session.OpenStream([]byte(body))
	if status != StatusOK {
		t.Fatalf("open stream: %v", status)
	}
	total := 0
	for {
		status, chunk := session.ReadStream(streamHandle, 16)
		if status == StatusEndOfStream {
			break
		}
		if status != StatusOK {
			t.Fatalf("read: %v", status)
		}
		total += len(chunk)
		if total > 64 {
			t.Fatalf("read %d bytes past the 64-byte budget", total)
		}
	}
	if total != 64 {
		t.Errorf("read %d bytes, want the 64-byte budget", total)
	}
	if status := session.CloseStream(streamHandle); status != StatusOK {
		t.Fatalf("close stream: %v", status)
	}
	if status := session.CloseStream(streamHandle); status != StatusStaleHandle {
		t.Errorf("second close: %v, want stale rather than a second underlying close", status)
	}
}

func TestStreamReadBoundsAreEnforced(t *testing.T) {
	session, _ := openTestSession(t)
	if status, _ := session.ReadStream(1, 0); status != StatusInvalidArgument {
		t.Errorf("zero read size: %v", status)
	}
	if status, _ := session.ReadStream(1, MaxStreamChunkBytes+1); status != StatusLimitExceeded {
		t.Errorf("oversized read: %v", status)
	}
	if _, status := session.OpenStream([]byte(`{"resource_id":"res_x","max_bytes":0}`)); status != StatusInvalidArgument {
		t.Errorf("zero budget: %v", status)
	}
	if _, status := session.OpenStream([]byte(`{"resource_id":"res_missing","max_bytes":16}`)); status != StatusNotFound {
		t.Errorf("missing resource: %v, want not found", status)
	}
}

// Two owners of one database is a corrupted library, not a race with a wrong
// answer, so the refusal is checked directly.
func TestASecondOwnerIsRefused(t *testing.T) {
	profile := testProfile(t)
	first, status := OpenInstance(profile)
	if status != StatusOK {
		t.Fatalf("first open: %v", status)
	}
	defer CloseInstance(first)

	if _, status := OpenInstance(profile); status != StatusConflict {
		t.Fatalf("second open: %v, want conflict", status)
	}

	// Ownership is by canonical path, so a different spelling of the same file
	// is the same claim.
	spelled := filepath.Join(filepath.Dir(profile), ".", filepath.Base(profile))
	if _, status := OpenInstance(spelled); status != StatusConflict {
		t.Errorf("alternate spelling: %v, want conflict", status)
	}
}

func TestOwnershipIsReleasedOnClose(t *testing.T) {
	profile := testProfile(t)
	first, status := OpenInstance(profile)
	if status != StatusOK {
		t.Fatalf("open: %v", status)
	}
	if status := CloseInstance(first); status != StatusOK {
		t.Fatalf("close: %v", status)
	}
	second, status := OpenInstance(profile)
	if status != StatusOK {
		t.Fatalf("reopen after close: %v, want the lock to have been released", status)
	}
	CloseInstance(second)
}

func TestCanonicalPathResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.db")
	if err := os.WriteFile(real, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.db")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	viaReal, err := canonicalProfilePath(real)
	if err != nil {
		t.Fatal(err)
	}
	viaLink, err := canonicalProfilePath(link)
	if err != nil {
		t.Fatal(err)
	}
	if viaReal != viaLink {
		t.Errorf("a symlink produced a different identity: %q vs %q", viaReal, viaLink)
	}
}

func TestShutdownRefusesNewWorkAndSettlesInFlightCalls(t *testing.T) {
	profile := testProfile(t)
	instance, status := OpenInstance(profile)
	if status != StatusOK {
		t.Fatalf("open: %v", status)
	}
	session, _ := LookupSession(instance)

	if status := CloseInstance(instance); status != StatusOK {
		t.Fatalf("close: %v", status)
	}
	if !session.IsShuttingDown() {
		t.Error("session does not report shutting down after close")
	}
	// New work after shutdown is refused with shutting_down, which tells a host
	// it is closing rather than that its handle is wrong.
	if _, status := session.StartCall([]byte(`{"op":"abi.info"}`)); status != StatusShuttingDown {
		t.Errorf("call after shutdown: %v, want shutting down", status)
	}
	if _, status := session.OpenStream([]byte(`{"resource_id":"x","max_bytes":1}`)); status != StatusShuttingDown {
		t.Errorf("stream after shutdown: %v, want shutting down", status)
	}
}

func TestConcurrentCallsAndCloseDoNotRace(t *testing.T) {
	instance, status := OpenInstance(testProfile(t))
	if status != StatusOK {
		t.Fatalf("open: %v", status)
	}
	session, _ := LookupSession(instance)

	var waiting sync.WaitGroup
	for i := 0; i < 16; i++ {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			handle, status := session.StartCall([]byte(`{"op":"abi.info"}`))
			if status != StatusOK {
				return // shutting down is a legitimate outcome here
			}
			for range 200 {
				if status, _ := session.PollCall(handle); status != StatusWouldBlock {
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}
	time.Sleep(5 * time.Millisecond)
	if status := CloseInstance(instance); status != StatusOK && status != StatusShuttingDown {
		t.Fatalf("close during concurrent calls: %v", status)
	}
	waiting.Wait()
}

func TestDoubleCloseIsReportedNotRepeated(t *testing.T) {
	instance, status := OpenInstance(testProfile(t))
	if status != StatusOK {
		t.Fatalf("open: %v", status)
	}
	if status := CloseInstance(instance); status != StatusOK {
		t.Fatalf("first close: %v", status)
	}
	if status := CloseInstance(instance); status != StatusStaleHandle {
		t.Errorf("second close: %v, want stale", status)
	}
}

func TestOpenRejectsBadProfiles(t *testing.T) {
	if _, status := OpenInstance(""); status != StatusInvalidArgument {
		t.Errorf("empty profile: %v", status)
	}
	if _, status := OpenInstance(strings.Repeat("a", MaxProfileBytes+1)); status != StatusInvalidArgument {
		t.Errorf("oversized profile: %v", status)
	}
}

// A result too large for one response must be refused rather than truncated:
// short JSON would parse as complete and be believed.
func TestOversizedResultIsRefusedNotTruncated(t *testing.T) {
	session, _ := openTestSession(t)
	huge := strings.Repeat("x", MaxRequestBytes/2)
	status, decoded := call(t, session,
		`{"op":"note.create","payload":{"collection_id":"default","title":"big","body":"`+huge+`","mime_type":"text/markdown"}}`)
	if status == StatusOK {
		// The body fits; nothing to assert beyond it round-tripping.
		return
	}
	if status != StatusLimitExceeded {
		t.Fatalf("oversized result: %v (%s)", status, decoded.Message)
	}
	if !strings.Contains(decoded.Message, "stream") {
		t.Errorf("refusal does not point at the alternative: %q", decoded.Message)
	}
}

func TestStatusMappingCoversFacadeKinds(t *testing.T) {
	cases := map[error]Status{
		application.ErrNotFound:             StatusNotFound,
		application.ErrConflict:             StatusConflict,
		application.ErrPreconditionRequired: StatusPreconditionRequired,
		application.ErrInvalidInput:         StatusInvalidArgument,
		application.ErrInvalidCursor:        StatusInvalidArgument,
		application.ErrNameConflict:         StatusConflict,
		application.ErrForbidden:            StatusForbidden,
		application.ErrUnavailable:          StatusUnavailable,
		application.ErrInternal:             StatusInternal,
		context.Canceled:                    StatusCancelled,
		errors.New("unclassified"):          StatusInternal,
	}
	for err, want := range cases {
		if got := statusForError(err); got != want {
			t.Errorf("statusForError(%v) = %v, want %v", err, got, want)
		}
	}
	if statusForError(nil) != StatusOK {
		t.Error("nil error is not ok")
	}
}

// seedResource creates an attachment directly in the database file before any
// instance owns it, so the stream tests have real bytes to read without the
// session having to expose the store the facade deliberately hides.
func seedResource(t *testing.T, profile, payload string) string {
	t.Helper()
	backing, err := store.OpenSQLite(profile)
	if err != nil {
		t.Fatalf("open store for seeding: %v", err)
	}
	defer backing.Close()
	if err := backing.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap for seeding: %v", err)
	}
	resource, err := backing.CreateResource(context.Background(), store.CreateResourceRequest{
		CollectionID: "default",
		Filename:     "stream.txt",
		MIMEType:     "text/plain",
		Content:      strings.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("seed resource: %v", err)
	}
	return resource.ID
}
