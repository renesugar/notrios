// Package main is a disposable H0 link/runtime probe. It proves that a Go
// shared library can own the real schema-v27 store with one statically linked
// SQLite engine; it is not the production ABI.
package main

/*
#cgo pkg-config: sqlite3
#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include <sqlite3.h>
*/
import "C"

import (
	"bytes"
	"context"
	"unsafe"

	"github.com/renesugar/notrios/internal/store"
)

const (
	probeOK              = 0
	probeInvalidArgument = 1
	probeStoreFailure    = 2
	probeSchemaMismatch  = 3
)

//export notrios_store_probe_sqlite_version
func notrios_store_probe_sqlite_version(out *C.char, capacity C.size_t) C.size_t {
	version := []byte(C.GoString(C.sqlite3_libversion()))
	required := C.size_t(len(version))
	if out == nil || capacity == 0 {
		return required
	}
	n := len(version)
	if n >= int(capacity) {
		n = int(capacity) - 1
	}
	if n > 0 {
		C.memcpy(unsafe.Pointer(out), unsafe.Pointer(&version[0]), C.size_t(n))
	}
	*(*byte)(unsafe.Add(unsafe.Pointer(out), n)) = 0
	return required
}

//export notrios_store_probe_open
func notrios_store_probe_open(path *C.char, pathLen C.size_t) C.int32_t {
	if path == nil || pathLen == 0 || pathLen > 4096 {
		return probeInvalidArgument
	}
	pathBytes := C.GoBytes(unsafe.Pointer(path), C.int(pathLen))
	if bytes.IndexByte(pathBytes, 0) >= 0 {
		return probeInvalidArgument
	}
	st, err := store.OpenSQLite(string(pathBytes))
	if err != nil {
		return probeStoreFailure
	}
	defer st.Close()
	if err := st.Bootstrap(context.Background()); err != nil {
		return probeStoreFailure
	}
	status, err := st.Status(context.Background())
	if err != nil {
		return probeStoreFailure
	}
	if status.SchemaVersion != store.CurrentSchemaVersion {
		return probeSchemaMismatch
	}
	return probeOK
}

func main() {}
