//go:build plan9 || js

package synccarrier

import "os"

func singleLink(_ *os.File, _ os.FileInfo) bool { return false }
