package store

/*
#cgo CFLAGS: -I${SRCDIR}/csqlite
#cgo CFLAGS: -DSQLITE_THREADSAFE=1
#cgo CFLAGS: -DSQLITE_ENABLE_FTS5
#cgo CFLAGS: -DSQLITE_ENABLE_JSON1
#cgo CFLAGS: -DSQLITE_DQS=0
#cgo CFLAGS: -DSQLITE_OMIT_LOAD_EXTENSION
#cgo CFLAGS: -DSQLITE_SECURE_DELETE
#cgo CFLAGS: -DSQLITE_USE_URI=1
#cgo CFLAGS: -fvisibility=hidden
#cgo LDFLAGS: -lm
*/
import "C"

// This file owns how SQLite is built, and nothing else.
//
// v0.8 H0 chose the checksum-pinned official amalgamation over the system
// library and over modernc.org/sqlite. The reasons that matter to this file:
//
//   - One engine owns the canonical database. Linking statically means the
//     version the tests ran against is the version that ships, rather than
//     whatever libsqlite3 the host happens to have.
//   - -fvisibility=hidden keeps every sqlite3_* symbol out of the dynamic
//     table. The C ABI in cmd/notrioslib must export its own twelve symbols
//     and nothing else; a host that could resolve sqlite3_open against this
//     library could open the canonical file behind the owner's back.
//   - SQLITE_DQS=0 refuses MySQL-style double-quoted string literals, so a
//     mistyped identifier is an error rather than a silently empty result.
//   - SQLITE_OMIT_LOAD_EXTENSION removes the loadable-extension entry points
//     entirely. Imported notes are untrusted input and nothing in the product
//     loads an extension.
//   - SQLITE_SECURE_DELETE overwrites deleted content rather than leaving it
//     in free pages, which is what a note the user deleted deserves.
//   - FTS5 and JSON1 are load-bearing: search and the metadata columns.
//
// Runtime policy (FULLMUTEX opens, WAL, the five-second busy timeout,
// checkpoint before handoff, and the Online Backup API for physical
// snapshots) is enforced in sqlite.go, not here.
//
// Updating the pin is its own reviewed slice; see csqlite/PROVENANCE.json for
// the required steps and the exact hashes this tree was built from.
