// Package application is the transport-neutral application facade.
//
// v0.8 H0 chose this package as the owner of the application contract, in
// preference to narrowing internal/service. The reason is direction:
// internal/service imports both internal/httpapi and net/http, so operations
// expressed there inherit an HTTP shape whether or not they want one. A second
// consumer that is not HTTP — the C ABI in cmd/notrioslib, and later a mobile
// client — cannot be served by a package that already answers to a router.
//
// The rules that keep this package useful:
//
//   - No transport concerns. Nothing here knows about HTTP status codes,
//     headers, JSON-RPC, Wails, or a URL. Callers map [Error] to their own
//     vocabulary; see [ErrorKind].
//   - No storage concerns. Nothing exported here names a SQL statement, a
//     driver, a database handle, or a filesystem path.
//   - No serialization concerns. Result types carry no struct tags. A note is
//     a note, not a JSON document that happens to be a note.
//
// [Repository] is the one deliberate exception. It is the transitional seam
// onto internal/store that H0 permits: the store-backed implementation may
// wrap store.Store, but the facade's exported operation contract must not
// expose it. TestExportedSurfaceIsTransportAndStorageNeutral enforces exactly
// that boundary mechanically, so the exception cannot quietly widen.
//
// Errors are values with a [Kind], not strings to be matched. Every operation
// returns either a typed [Error] or a context error, and the sentinels
// ([ErrNotFound] and friends) work with errors.Is.
package application
