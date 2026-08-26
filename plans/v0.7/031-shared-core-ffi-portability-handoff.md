# v0.7 G18 — shared-core, FFI, Mermaid, and portability handoff

**Status:** complete

**Date:** 2026-08-25

**Model:** GPT-5 (exact serving variant unavailable)

## Goal and boundary

End v0.7 synchronization with a finite, testable input to v0.8 shared-core and
installation work and the independent post-1.0 Flutter client. G18 is a source
audit, feasibility probe, and contract slice. It did not implement a production
application facade, C ABI, installer, Flutter app, APK, Wails v3 migration,
Mermaid rendering, or physical-device support.

## Current seams and extraction finding

The audit reconciles 109 normalized non-HEAD OpenAPI operations with 113
registered handler patterns: the 109 API operations, three HEAD aliases, and
the presentation-only web root. `store.Store`, `SyncSecretStore`, sidecar
interfaces, context-aware calls, durable jobs, and bounded readers are reusable
seams. Wails occurs only in the build-tagged GUI adapter.

The core is not yet a facade. `internal/service` owns `*httpapi.Server` and
`*http.Server`; 24 production HTTP-adapter files use `net/http`, and handlers
still combine transport decoding with application orchestration. v0.8 must
extract one application package consumed by HTTP and the C adapter. An in-
process loopback request would preserve the wrong dependency direction and is
not an implementation option.

## Frozen ABI-major-1 candidate

The proposed boundary has 12 C symbols covering ABI/capability query, instance
open/close, bounded asynchronous call start/poll/cancel, result release, event
poll, and stream open/read/close. It deliberately avoids one symbol per route.

- Inputs are caller-owned and borrowed only for the duration of a call.
- Outputs are immutable library-owned buffers released exactly once.
- Handles are generation-bearing opaque unsigned 64-bit values; zero, stale,
  foreign, and wrong-kind handles are typed refusals.
- Go pointers never enter C-visible memory. Dart is never called from an
  arbitrary Go runtime thread; clients poll calls/events.
- JSON requests and responses and each stream read are capped at 1 MiB; event
  polls return at most 256 records and error text at most 4 KiB.
- Close enters `shutting_down`, refuses new work, cancels calls, closes streams,
  joins workers, invalidates the generation, and cannot unwind a Go panic into C.

HTTP routes/path/query translate to operation/payload. Status codes translate
to the closed ABI/domain-error set; ETag/If-Match to revision preconditions;
Range to offset/length/total/EOF; connection cancellation to call handles and
deadlines. HTTP TLS, CSP, cookies, Origin, remote address, web assets, and peer
rate limiting stay adapters. The authenticated peer carrier remains a courier,
not a privileged local ABI shortcut.

## Platform and packaging matrix

Nineteen capabilities cover profile/config paths, SQLite/database/assets,
projection/Recoll, quarantine/catch-up, shared directories/pickers, secrets,
TLS/listener/outbound network, workers, notifications, deep links, web assets,
native import/export/snapshot paths, logging/crash behavior, and cgo linkage.
Each has an owner and an explicit state for current Linux, v0.8 Windows/macOS,
the pre-1.0 Android emulator, and post-1.0 iOS/physical mobile.

The current Linux `c-shared` and `c-archive` probes built and dynamically linked
host `libsqlite3`. They emitted no header because the current main package has
no exported ABI symbol; the `/tmp` files are not release libraries. The
Android/arm64 API-35 probe reached NDK 30's compiler and failed at missing
`sqlite3.h`; no NDK SQLite header/library was found. Debian does provide
`/usr/include/sqlite3.h` plus an x86-64 library, but these are host artifacts;
forcing the host include root into the Android/arm64 probe fails in incompatible
glibc/Android sysroot headers and cannot supply the target library. This
converts the SQLite linkage premise into a v0.8 decision/probe rather than
hiding it in bridge work. A follow-up Flutter Doctor run passes the Android SDK,
Android Studio, bundled-Java, and license checks. There is no configured AVD or
connected Android device, and no Android or Flutter build is claimed.

The emulator gate requires library load, two isolated instances, SQLite
bootstrap plus CRUD/search, one bounded resource stream, cancellation/polling,
capability negotiation, no exposed Go pointer, and recorded RSS/lifecycle.
Physical storage, secure-store migration, background/battery/notifications,
pairing/catch-up/backup, responsive UI, accessibility, and iOS remain post-1.0.

## 2026-08-26 Android SQLite and editor-search follow-up

The host toolchain limitation has changed without changing G18's product
boundary. Ubuntu Clang/Clang++ 18.1.3 now resolve from `/usr/bin`, and Flutter
Doctor 3.44.9 reports no issues across Android, Chrome, Linux, devices, and
network. Swiftly 1.1.2 is on PATH and lists Swift 6.3.3, but no Swift toolchain
is selected. There are still zero configured AVDs and connected Android
devices, so no build or emulator claim follows from the all-passing Doctor.

The supplied Android SQLite material was only partly accurate. Android exposes
framework SQLite to Java/Kotlin, but SQLite is not a public NDK C API and this
installed NDK has no `sqlite3.h`/`libsqlite3` development pair. Jetpack's
`BundledSQLiteDriver` genuinely packages a compiled native SQLite and gives
Kotlin/Room a consistent driver; it does not satisfy Notrios's current
`pkg-config: sqlite3` cgo contract. Canonical storage is owned by the Go shared
core across 43 C-importing store files, with FULLMUTEX, WAL, FTS5, and JSON
requirements. Selecting the Jetpack driver for that database would be a
database-ownership redesign and bridge, not a linkage fix.

The v0.8 H0 investigation uses a checksum-pinned upstream SQLite amalgamation
compiled into the Go shared library as its C baseline. The later
`modernc.org/sqlite` amendment adds a second unselected Go-owned candidate.
Implementation remains blocked on the recorded package/version/update/compile-option,
minSdk/ABI, symbol/duplicate-engine, Android sandbox/lifecycle, and cross-
platform database-compatibility decisions. The exact build/load, compile-
option, FTS5/JSON, store/snapshot/sync, database round-trip, size/RSS, and
single-engine gates are machine-recorded in
`performance/v0.7-g18/ANDROID_SQLITE_FOLLOWUP.json`.

## 2026-08-26 modernc/cznic SQLite evaluation amendment

Pinned v1.57.0/native evidence proves SQLite 3.53.3 with FTS5/JSON/WAL and a
C-modernc-C Linux database round trip. Disposable Linux, macOS, and Windows
executables compiled. Android/arm64 compiled both an ordinary executable and an
NDK `c-shared` artifact, but Android/iOS are absent from upstream's support
table and no device/AVD existed to execute the library. iOS remains an Apple-
host external-linking gate. `js/wasm` failed in the required modernc libc layer.

The maintainer benchmark is mixed rather than “slightly slower”: representative
bulk inserts were about 3.2–4.8× slower than the CGo driver, while many repeated
and concurrent reads were faster. Its older versions and DELETE/FULL workload
are not a Notrios decision oracle. H0 now A/B tests the pinned C baseline and
exact modernc/libc candidate on the same emulator and production-shaped
WAL/FTS5/snapshot/sync corpus. Go retains sole canonical ownership; Jetpack is
not a query bridge, and Web remains REST or a separately planned browser store.
Machine evidence: `performance/v0.7-g18/MODERNC_SQLITE_EVALUATION.json`.

The installed editor stack was also tested rather than inferred. md-editor-rt
6.5.3 installs CodeMirror search 6.7.1. Ctrl+F opened the editable-note search
panel; case-sensitive, regexp, and whole-word counts, single replace,
whole-word replace-all, and persistence passed at desktop and narrow viewports
with zero console warnings/errors. The panel owns replacement controls;
Ctrl/Cmd+H is not a default CodeMirror binding. Aggregate evidence is in
`EDITOR_SEARCH_QA.json`; disposable screenshots and the test database remained
outside the repository.

## Mermaid baseline and enablement gate

The current React GUI still sets `noMermaid: true`; fenced source is the honest
fallback. G18 selects candidate v0.8 limits of 64 KiB source, 500 nodes, 1,000
edges, 256 KiB total labels, two seconds, and 64 MiB browser-heap delta. Tests
cover valid flow/sequence diagrams, themes, malformed source, scripts/handlers/
unsafe URLs, each limit plus one, timeout cleanup, offline cache-disabled use,
CSP, sanitization, accessibility, and Wails. Rendering must use strict security
mode, sanitize generated SVG before insertion, make zero cross-origin requests,
and retain source on every refusal. No setting or dependency changed in G18.

## Evidence and validation

`performance/v0.7-g18/` contains the source audit, platform matrix, ABI
contract, Mermaid contract, Android feasibility record, validator, and unit
tests. The validator recomputes route, Wails, cgo, service/HTTP, bounds, matrix,
and disabled-baseline facts without network or private data.

Standard validation covered npm audit before/after clean install, frontend
typecheck/tests/build, Go tests/vet, headless smoke, Wails v2 desktop build/
smoke, G18 evidence tests, scaffold/required-file checks, docs, plan/log
consistency, release packaging, and the mandatory evidence pre-push gate. No
production application code, schema, dependency, or existing API changed.
