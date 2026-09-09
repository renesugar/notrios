package abi

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// H0 requires one owner enforced twice: canonical-path identity within this
// process, and a lifetime-held exclusive lock across cooperating processes.
//
// One check is not enough, and the two catch different mistakes. The in-process
// registry catches a host that opens the same library twice through this
// library — common, and cheap to detect. The file lock catches a second
// process, which the registry cannot see at all. Two SQLite engines writing one
// database is not a race that produces a wrong answer; it is a corrupted
// library, so the refusal is deliberately blunt.
//
// The lock is non-blocking on purpose. Waiting would turn "another process owns
// this" into a hang that looks like a deadlock, and the honest answer is
// available immediately.

var processOwners = struct {
	sync.Mutex
	held map[string]bool
}{held: make(map[string]bool)}

// ownerLock is a held claim on one canonical database path.
type ownerLock struct {
	canonicalPath string
	lockPath      string
	file          *os.File
	released      bool
}

// canonicalProfilePath resolves a profile path to the identity used for
// ownership. Symlinks are resolved so that two different spellings of the same
// database cannot both be claimed; a path that does not exist yet is resolved
// through its parent, because opening a new library is legitimate.
func canonicalProfilePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("profile path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved, nil
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		// The parent does not exist either; the absolute path is the best
		// identity available and open will fail for its own reasons.
		return absolute, nil
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}

// acquireOwnerLock claims exclusive ownership of a profile path.
func acquireOwnerLock(path string) (*ownerLock, Status, error) {
	canonical, err := canonicalProfilePath(path)
	if err != nil {
		return nil, StatusInvalidArgument, err
	}

	processOwners.Lock()
	if processOwners.held[canonical] {
		processOwners.Unlock()
		return nil, StatusConflict, fmt.Errorf("this process already owns %s", canonical)
	}
	processOwners.held[canonical] = true
	processOwners.Unlock()

	release := func() {
		processOwners.Lock()
		delete(processOwners.held, canonical)
		processOwners.Unlock()
	}

	lockPath := canonical + ".owner"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		release()
		return nil, StatusUnavailable, fmt.Errorf("open owner lock: %w", err)
	}
	// LOCK_NB: report the conflict now rather than hanging until the other
	// owner exits.
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		release()
		return nil, StatusConflict, fmt.Errorf("another process owns %s: %w", canonical, err)
	}
	return &ownerLock{canonicalPath: canonical, lockPath: lockPath, file: file}, StatusOK, nil
}

// release drops the claim. It is safe to call more than once.
//
// The lock file is left in place. Removing it would race with another process
// that has already opened it and is about to flock it: that process would end
// up holding a lock on an unlinked inode while a third took a fresh one, and
// both would believe they owned the database.
func (o *ownerLock) release() {
	if o == nil || o.released {
		return
	}
	o.released = true
	if o.file != nil {
		_ = syscall.Flock(int(o.file.Fd()), syscall.LOCK_UN)
		_ = o.file.Close()
	}
	processOwners.Lock()
	delete(processOwners.held, o.canonicalPath)
	processOwners.Unlock()
}
