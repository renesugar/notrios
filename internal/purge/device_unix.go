//go:build unix

package purge

import (
	"os"
	"syscall"
)

// deviceOfPath reports which filesystem a path is on, so the mount-boundary
// rule can refuse a target that is not on the same one as its root.
func deviceOfPath(path string) (uint64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, os.ErrInvalid
	}
	return uint64(stat.Dev), nil
}
