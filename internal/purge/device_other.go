//go:build !unix

package purge

import "errors"

// deviceOfPath has no portable answer off Unix. Refusing is the right default:
// the mount-boundary rule exists to prevent a purge crossing a filesystem, and
// a platform where that cannot be determined is a platform where it cannot be
// ruled out.
func deviceOfPath(string) (uint64, error) {
	return 0, errors.New("the filesystem of a path cannot be determined on this platform")
}
