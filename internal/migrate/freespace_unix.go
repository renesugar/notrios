//go:build linux || darwin

package migrate

import (
	"os"
	"path/filepath"
	"syscall"
)

// freeBytes reports the space available on the filesystem holding path.
//
// The nearest existing ancestor is measured, because the destination root
// usually does not exist yet on a first migration -- that is the normal case,
// not an error.
func freeBytes(path string) (int64, error) {
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
