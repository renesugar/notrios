//go:build !linux && !darwin

package paths

// FreeBytes has no portable implementation.
//
// Reporting zero would make every migration refuse on an untested platform,
// which is the wrong failure: the check exists to catch a full disk, and a
// platform where it cannot run should not be told its disk is full. The
// caller's comparison is skipped when there is nothing to copy, and a real
// out-of-space condition still surfaces as a copy error.
func FreeBytes(string) (int64, error) {
	const unknown = int64(1) << 62
	return unknown, nil
}
