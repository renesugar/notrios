package purge

import (
	"os"
	"path/filepath"
	"strings"
)

// realpath resolves symlinks as far as the filesystem allows, then re-attaches
// the rest.
//
// filepath.EvalSymlinks refuses a path that does not exist, and "does not
// exist" is a verdict this oracle has to reach rather than an error it can fail
// on -- a repeated purge must be a no-op. Python's os.path.realpath, which the
// reference uses, resolves the longest existing prefix and keeps the remainder;
// this does the same.
func realpath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	parent, base := filepath.Split(path)
	parent = strings.TrimRight(parent, "/")
	if parent == "" || parent == path || base == "" {
		return path
	}
	return filepath.Join(realpath(parent), base)
}

func lexists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}
