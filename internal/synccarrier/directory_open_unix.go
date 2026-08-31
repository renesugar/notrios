//go:build android || darwin || ios || linux

package synccarrier

import (
	"os"
	"syscall"
)

func openRootDirectoryNoFollow(root *os.Root, path string) (*os.File, error) {
	return root.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
