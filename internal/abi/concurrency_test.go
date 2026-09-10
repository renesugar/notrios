package abi

import (
	"sync"
	"testing"
	"time"
)

// The edges that only exist when more than one thread is holding the boundary.
//
// v0.9 I8 tried to assert these through the C boundary under ThreadSanitizer
// and could not: TSan maps a large shadow region and must do it as the process
// starts, and loaded through a c-shared library it fails with "failed to
// allocate ... bytes" and then, with the host instrumented too, "unexpected
// memory mapping" -- with ASLR disabled as well. AddressSanitizer works through
// that boundary and is clean; the race detector does not.
//
// So the race detector runs where it does work: here, against the same
// dispatch, session and handle code the C entry points call straight into.
// `go test -race ./internal/abi/` is the assertion, and these are the schedules
// worth giving it.

// Concurrent callers on one session, which is what a GUI with a background
// sync and a foreground search actually is.
func TestConcurrentCallsOnOneSession(t *testing.T) {
	session, _ := openTestSession(t)

	var wg sync.WaitGroup
	failures := make(chan string, 64)
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				handle, status := session.StartCall([]byte(`{"op":"abi.info"}`))
				if status != StatusOK {
					failures <- "start: " + status.String()
					return
				}
				deadline := time.Now().Add(30 * time.Second)
				for {
					status, _ := session.PollCall(handle)
					if status != StatusWouldBlock {
						if status != StatusOK {
							failures <- "poll: " + status.String()
						}
						break
					}
					if time.Now().After(deadline) {
						failures <- "a call never settled"
						return
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()
	close(failures)
	for message := range failures {
		t.Error(message)
	}
}

// Closing an instance while calls are in flight. A host does this when the
// window closes: the user does not wait for the sync to finish.
//
// What is asserted is that nothing here corrupts anything and nothing hangs --
// the calls may succeed, or be refused as stale. Both are correct answers; the
// race detector is watching for the third thing.
func TestCloseWhileCallsAreInFlight(t *testing.T) {
	handle, status := OpenInstance(testProfile(t))
	if status != StatusOK {
		t.Fatalf("open instance: %v", status)
	}
	session, status := LookupSession(handle)
	if status != StatusOK {
		t.Fatalf("lookup session: %v", status)
	}

	var wg sync.WaitGroup
	started := make(chan struct{})
	var once sync.Once
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				call, status := session.StartCall([]byte(`{"op":"abi.info"}`))
				once.Do(func() { close(started) })
				if status != StatusOK {
					return // the instance is closing; refusing is correct
				}
				deadline := time.Now().Add(10 * time.Second)
				for {
					status, _ := session.PollCall(call)
					if status != StatusWouldBlock {
						break
					}
					if time.Now().After(deadline) {
						t.Error("a call never settled while the instance was closing")
						return
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	<-started
	time.Sleep(5 * time.Millisecond)
	CloseInstance(handle)
	wg.Wait()

	// After the close the handle is gone, and saying so is the contract.
	if _, status := LookupSession(handle); status == StatusOK {
		t.Fatal("the session survived its instance being closed")
	}
}

// Cancellation racing completion. The two outcomes are both correct and the
// interesting case is that the loser of the race does not leave the call handle
// in a state that never settles.
func TestCancelRacingCompletion(t *testing.T) {
	session, _ := openTestSession(t)

	for attempt := 0; attempt < 40; attempt++ {
		call, status := session.StartCall([]byte(`{"op":"abi.info"}`))
		if status != StatusOK {
			t.Fatalf("start: %v", status)
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			session.CancelCall(call)
		}()

		deadline := time.Now().Add(30 * time.Second)
		for {
			status, _ := session.PollCall(call)
			if status != StatusWouldBlock {
				if status != StatusOK && status != StatusCancelled {
					t.Fatalf("a cancelled call ended as %v", status)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("a call racing cancellation never settled")
			}
			time.Sleep(time.Millisecond)
		}
		wg.Wait()
	}
}
