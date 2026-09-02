//go:build linux || darwin

package paths

import (
	"os"
	"path/filepath"
	"syscall"
)

// FreeBytes reports the space available on the filesystem holding path.
//
// It lives here rather than beside either caller because two of them need it --
// migration sizing a copy between roots, and the pre-migration backup sizing a
// copy beside the database -- and a platform probe that exists twice is a
// platform probe that will eventually disagree with itself.
//
// The nearest existing ancestor is measured, because the destination root
// usually does not exist yet on a first migration -- that is the normal case,
// not an error.
func FreeBytes(path string) (int64, error) {
	probe, err := nearestExisting(path)
	if err != nil {
		return 0, err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(probe, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

func nearestExisting(path string) (string, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(current); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return current, nil
		}
		current = parent
	}
}
