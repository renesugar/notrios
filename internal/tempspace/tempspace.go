// Package tempspace gives each Notrios instance a temp directory of its own.
//
// Several instances can run on one machine, and one instance can have several
// processes: the service, a command-line export, the GUI. The system temp root
// is shared by all of them. Nothing an instance puts there can be told apart
// from another instance's, so nothing there can be cleaned up without risking
// work in flight elsewhere. J7 found 17,078 Notrios entries left in it.
//
// An instance's temp directory is data.temp_dir, <cache dir>/tmp by default.
// Each process of the instance opens a Space there: a directory p-<id> of its
// own, beside a lock file p-<id>.lock that the process holds for as long as the
// Space is open. The lock is released when the process exits, however it
// exits.
//
// Opening a Space removes the pairs in the same temp directory whose lock no
// live process holds, which are what a crashed process left behind, and
// nothing else. It never looks at another instance's temp directory, and never
// at the system temp root.
package tempspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const (
	processPrefix = "p-"
	lockSuffix    = ".lock"
	idBytes       = 8
)

// ErrClosed is returned for work asked of a Space that has been closed.
var ErrClosed = errors.New("tempspace: the temp space is closed")

// Space is one process's share of an instance temp directory.
type Space struct {
	mu     sync.Mutex
	root   string
	dir    string
	lock   *os.File
	swept  []string
	closed bool
}

// Open claims a process directory in the instance temp directory root,
// creating root owner-only if needed. It first removes that directory's
// leftovers: process directories whose lock no live process holds.
func Open(root string) (*Space, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("tempspace: an instance temp directory is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Stat(root); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("tempspace: %s is not a directory", root)
	}
	space := &Space{root: root, swept: sweep(root)}
	for attempt := 0; attempt < 8; attempt++ {
		id, err := newID()
		if err != nil {
			return nil, err
		}
		lockPath := filepath.Join(root, processPrefix+id+lockSuffix)
		file, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		// Between creating the file and locking it, another process's sweep may
		// have locked it as a leftover and unlinked it. Then this process holds
		// a lock nobody can see, so it starts again under a new name.
		if lockErr := lock(file); lockErr != nil || !sameFile(file, lockPath) {
			if lockErr == nil {
				_ = unlock(file)
			}
			_ = file.Close()
			if lockErr != nil && !errors.Is(lockErr, syscall.EWOULDBLOCK) {
				return nil, lockErr
			}
			continue
		}
		dir := filepath.Join(root, processPrefix+id)
		if err := os.Mkdir(dir, 0o700); err != nil {
			_ = os.Remove(lockPath)
			_ = unlock(file)
			_ = file.Close()
			return nil, err
		}
		space.dir, space.lock = dir, file
		return space, nil
	}
	return nil, fmt.Errorf("tempspace: could not claim a process directory in %s", root)
}

// MkdirTemp creates a private, owner-only directory for one piece of temporary
// work, named from pattern as os.MkdirTemp names it. The caller removes it.
//
// In an open Space it is created inside this process's directory. A nil Space
// means no instance is configured, as in library use and tests. The directory
// is then created in the system temp root, which is the only place left.
func (s *Space) MkdirTemp(pattern string) (string, error) {
	if s == nil {
		return os.MkdirTemp("", pattern)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", ErrClosed
	}
	return os.MkdirTemp(s.dir, pattern)
}

// Dir is this process's directory, or "" for a nil Space.
func (s *Space) Dir() string {
	if s == nil {
		return ""
	}
	return s.dir
}

// Root is the instance temp directory, or "" for a nil Space.
func (s *Space) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// Swept lists the leftover process directories Open removed.
func (s *Space) Swept() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.swept...)
}

// Close removes this process's directory and everything in it, then its lock
// file, then releases the lock. The order is what lets a crash at any point
// leave nothing, or an unlocked pair the next Open removes. If the directory
// cannot be removed, the lock file is kept so a later Open retries it.
func (s *Space) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	removeErr := os.RemoveAll(s.dir)
	if removeErr == nil {
		if err := os.Remove(s.lock.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			removeErr = err
		}
	}
	_ = unlock(s.lock)
	return errors.Join(removeErr, s.lock.Close())
}

// sweep removes root's process directories whose lock it can take itself.
//
// Only a regular file named p-<id>.lock is a candidate, and it is removed
// together with p-<id>, which must be a real directory: a symlink in its place
// is left alone rather than followed. Anything else in root is not the
// sweep's to touch.
func sweep(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var removed []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasPrefix(name, processPrefix) || !strings.HasSuffix(name, lockSuffix) {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, processPrefix), lockSuffix)
		if !validID(id) {
			continue
		}
		lockPath := filepath.Join(root, name)
		file, err := os.OpenFile(lockPath, os.O_RDWR, 0)
		if err != nil {
			continue
		}
		if lock(file) != nil {
			// A live process holds it.
			_ = file.Close()
			continue
		}
		if !sameFile(file, lockPath) {
			_ = unlock(file)
			_ = file.Close()
			continue
		}
		dir := filepath.Join(root, processPrefix+id)
		info, statErr := os.Lstat(dir)
		switch {
		case statErr == nil && info.IsDir():
			if os.RemoveAll(dir) != nil {
				_ = unlock(file)
				_ = file.Close()
				continue
			}
			removed = append(removed, dir)
		case statErr == nil:
			_ = unlock(file)
			_ = file.Close()
			continue
		case !errors.Is(statErr, os.ErrNotExist):
			_ = unlock(file)
			_ = file.Close()
			continue
		}
		_ = os.Remove(lockPath)
		_ = unlock(file)
		_ = file.Close()
	}
	return removed
}

func lock(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func unlock(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

// sameFile reports whether the open file is still the one at path.
func sameFile(file *os.File, path string) bool {
	opened, err := file.Stat()
	if err != nil {
		return false
	}
	named, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return os.SameFile(opened, named)
}

func newID() (string, error) {
	buffer := make([]byte, idBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func validID(id string) bool {
	if len(id) != 2*idBytes {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil && strings.ToLower(id) == id
}
