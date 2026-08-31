//go:build !android && !darwin && !ios && !linux

package synccarrier

import "os"

// Platforms without Unix directory/open flags still receive Root containment
// plus Lstat/open/fstat identity checks. They do not claim the Unix FIFO race
// guarantee and are recorded as a G20 platform limitation.
func openRootDirectoryNoFollow(root *os.Root, path string) (*os.File, error) {
	return root.Open(path)
}
