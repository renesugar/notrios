package abi

import (
	"context"
	"fmt"

	"github.com/renesugar/notrios/internal/application"
	"github.com/renesugar/notrios/internal/store"
)

// sessions is the process-wide instance table. It is package state because the
// C ABI is process-wide by nature: a host holds integers, and the only place
// those integers can be resolved is here.
var sessions = newTable[*Session]()

// OpenInstance opens a library at profilePath and returns its handle.
//
// The store is opened only after ownership is claimed, so a refused claim never
// leaves a second SQLite connection open against the same file even briefly.
func OpenInstance(profilePath string) (Handle, Status) {
	if profilePath == "" || len(profilePath) > MaxProfileBytes {
		return 0, StatusInvalidArgument
	}

	owner, status, _ := acquireOwnerLock(profilePath)
	if status != StatusOK {
		return 0, status
	}

	backing, err := store.OpenSQLite(profilePath)
	if err != nil {
		owner.release()
		return 0, StatusUnavailable
	}
	if err := backing.Bootstrap(context.Background()); err != nil {
		_ = backing.Close()
		owner.release()
		return 0, StatusInternal
	}

	session := NewSession(application.New(backing), owner, backing.Close)
	return sessions.insert(session), StatusOK
}

// CloseInstance shuts a session down and invalidates its handle.
//
// The handle is removed only after Close returns, which is what makes a
// concurrent caller see shutting_down rather than a stale handle while teardown
// is still running. That distinction is the difference between "wait, this is
// closing" and "your handle is wrong", and a host debugging a race needs it.
func CloseInstance(handle Handle) Status {
	session, status := sessions.lookup(handle)
	if status != StatusOK {
		return status
	}
	if closeStatus := session.Close(); closeStatus == StatusShuttingDown {
		// Another caller is already tearing this down. Report that rather than
		// racing it.
		return StatusShuttingDown
	}
	_, removeStatus := sessions.remove(handle)
	if removeStatus != StatusOK {
		return removeStatus
	}
	return StatusOK
}

// LookupSession resolves an instance handle.
func LookupSession(handle Handle) (*Session, Status) {
	return sessions.lookup(handle)
}

// LiveInstances reports how many sessions are open. Used by tests and by the
// abi.info operation; a host that leaks instances can see it.
func LiveInstances() int { return sessions.len() }

// describeOwnerConflict is used in tests and diagnostics to explain a refusal
// without exposing the lock internals.
func describeOwnerConflict(path string) string {
	return fmt.Sprintf("another owner holds %s", path)
}
