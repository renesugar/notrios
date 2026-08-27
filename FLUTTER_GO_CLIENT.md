# Flutter/Go client and shared-core contract

Status: v0.7 G18 handoff contract completed 2026-08-25. It does not claim that a
C ABI, Flutter client, or Mermaid support is implemented.

## Decision

Notrios will not make mobile delivery depend exclusively on Wails. Before 1.0,
v0.8 will extract a framework-neutral application facade and build a versioned,
no-GUI Go C ABI suitable for native clients. A separate post-1.0 Flutter client
may use that ABI on Android, iOS, Linux, macOS, and Windows. Wails remains the
current desktop shell and may still gain mobile support if its mobile releases
become production-ready.

Flutter Web is different: `dart:ffi` is for Dart Native applications, while web
interop uses JavaScript/Wasm interfaces. A browser client therefore continues
to use REST or needs a separately designed Go-Wasm/JS adapter. The native C ABI
must not be described as one bridge for all six Flutter targets.

Other supplied claims also need qualification. FFI removes HTTP/socket framing,
but calls, thread transitions, serialization, copies, and two managed runtimes
are not zero-cost. Go's documented hazard is the cgo pointer-retention and
pinning contract—not a promise that Go's collector will move a live object out
from under Dart. The temporary copy count is implementation-dependent rather
than always five. Flutter's FFI-package/native-assets tooling helps bundle a
native library; it does not cross-compile Go, SQLite, sign Apple artifacts, or
make one binary portable by itself.

## Does the shared library need APIs beyond REST?

It needs additional **ABI infrastructure**, but not a second set of business
semantics or new ordinary REST routes merely because calls are in-process.
Existing REST request/response schemas are the starting application contract.
The implementation should move transport-neutral orchestration out of HTTP
handlers into one application facade, then let HTTP and FFI adapters translate
to it.

G17 sharpens that split for G18: retention planning and peer-retirement preview
are transport-neutral bounded application operations, while physical snapshot
path selection/re-verification is native-host capability and must not be
smuggled through a generic web request. The future ABI needs typed
`full_resync_required`, retirement confirmation, retention digest, and snapshot
verification/stream or file-picker handoff equivalents; it must preserve the
same no-automatic-install and no-time-only-deletion boundaries.

The ABI needs contracts that HTTP supplies implicitly:

- open/close an isolated Notrios instance for a named profile;
- query ABI, schema, protocol, and capability versions;
- submit bounded serialized requests and return typed status/error data;
- cancel long calls and poll jobs/events without calling Dart from arbitrary Go
  threads;
- open/read/close bounded streams for resources, archives, and other bulk data;
- define input/output-buffer ownership and provide an explicit free operation
  when the library allocates a result;
- use opaque numeric handles for instances, calls, and streams;
- define shutdown, concurrent-call, callback/thread, logging, and crash behavior.

Those concerns should not become one exported C function per REST route. A
small versioned dispatch surface plus stream/lifecycle functions is easier to
keep compatible and test. HTTP-only concepts such as status codes, headers,
`Range`, ETags, and connection cancellation map to typed ABI status, capability,
stream-range, object-hash, and cancellation fields.

## Proposed boundary to investigate in v0.8

The narrow initial shape is:

```text
Flutter/Dart             C ABI adapter                 Go core
-------------            -------------                 -------
open profile       ->    instance handle        ->     application facade
bounded request    ->    serialized envelope    ->     typed operation
small response     <-    owned byte buffer       <-     typed result
bulk response      <-    stream handle/read      <-     bounded reader
cancel/poll/close  ->    opaque handles          ->     jobs/lifecycle
```

G18 selects caller-owned input buffers that remain valid only for the duration
of a call and immutable library-owned output buffers released exactly once with
`notrios_buffer_release`. Leak, double-release, wrong-instance, cancellation,
and concurrent-shutdown tests are mandatory. Never expose a pointer to
Go-managed memory or a live Store/service object. Generation-bearing opaque
64-bit handles may be backed by `runtime/cgo.Handle`, subject to Go's cgo
pointer rules; zero is invalid and stale generations are typed refusals.

Serialized JSON is a sensible first compatibility format because Notrios
already owns JSON REST schemas and most calls are bounded. That is not “zero
overhead”: calls, serialization, copies, and dual runtimes all cost memory.
Large note lists, blobs, archives, and sync objects must use existing cursor,
range, job, and streaming concepts rather than one large JSON result. A compact
format is justified only by measured emulator/desktop evidence.

The Go wrapper must be a `main` package because `-buildmode=c-shared` and
`c-archive` export only cgo `//export` symbols from one main package. Keeping the
wrapper inside this module lets it import Notrios `internal/...` packages while
the public C header remains the compatibility surface. Android/iOS compilation
still needs explicit evidence for the C toolchain, SQLite linkage, native
library packaging/signing, Go runtime lifecycle, and app sandbox.

## G18 frozen source audit

G18 reconciled 109 normalized non-HEAD OpenAPI operations against 113 registered
handler patterns: 109 API operations, three `HEAD` aliases, and the presentation-
only web root. The reusable seams are `store.Store`, the injectable
`SyncSecretStore`, sidecar interfaces, context-aware operations, durable jobs,
and bounded resource/export readers. Wails imports occur only in
`cmd/notrios/gui_wails.go` behind the `gui` build tag.

That is not yet a framework-neutral application facade. `internal/service`
owns both `*httpapi.Server` and `*http.Server`; 24 production HTTP-adapter files
use `net/http`, and decoding/orchestration remain interleaved in handlers. v0.8
must extract an application package used by HTTP and the ABI. Calling those
handlers over an in-process or loopback HTTP request would retain the coupling
and is expressly not the selected design.

The G18 candidate is ABI major 1 with 12 symbols: version/capability query,
instance open/close, asynchronous bounded call start/poll/cancel, result-buffer
release, event poll, and stream open/read/close. Inputs are caller-owned and
borrowed only during a call. Outputs are immutable library allocations released
exactly once. Handles are generation-bearing opaque 64-bit values; zero is
invalid. No Go pointer crosses C-visible memory, and Go never calls Dart from an
arbitrary runtime thread. JSON requests/responses are capped at 1 MiB; bulk
streams read at most 1 MiB per call and events at 256 per poll.

HTTP method/routes become an operation name and payload; path/query values
become typed fields; status codes become a closed ABI/domain-error taxonomy;
ETag/If-Match becomes a revision precondition; Range becomes offset/length/
total/EOF; connection cancellation becomes call cancel/deadline/poll. CSP,
cookies, Origin, remote address, TLS, web assets, and peer rate limiting remain
adapter responsibilities. Peer-carrier REST remains an authenticated courier,
not a privileged local-FFI shortcut.

The complete machine-checked source audit, platform matrix, ABI proposal, and
test contract are under `performance/v0.7-g18/`.

## G18 platform and build findings

Nineteen finite capabilities cover profiles/config, database/assets,
projection/Recoll, quarantine/catch-up, shared directories and pickers, secrets,
TLS/listeners/network, workers, notifications, deep links, web assets, native
import/export/snapshot paths, logging/crash behavior, and SQLite/cgo. Current
Linux behavior is recorded separately from Windows/macOS v0.8 gates, one
pre-1.0 Android-emulator gate, and post-1.0 iOS/physical-device gates.

Linux `c-shared` and `c-archive` probes built from the existing main package and
dynamically linked host `libsqlite3`, but emitted no C header because there are
no exported ABI symbols. They are feasibility artifacts, not libraries. An
Android/arm64 API-35 cgo probe reached the installed NDK compiler and then
failed at `sqlite3.h`; the NDK contains no matching SQLite development boundary.
Debian's `/usr/include/sqlite3.h` and x86-64 library are installed, but they are
host files, not Android-arm64 artifacts. A follow-up that forced `/usr/include`
failed immediately in incompatible glibc/Android sysroot headers. v0.8 therefore
needs an explicit Android SQLite build/link/package choice before an emulator
can load the shared library. Flutter Doctor passes the Android SDK, Android
Studio, bundled-Java, license, Chrome, network, and Linux desktop checks after
the host PATH was corrected to `/usr/bin/clang` and `/usr/bin/clang++`; it now
reports no issues. Swiftly is reachable and Swift 6.3.3 is installed, but no
Swift toolchain is selected, so `swift --version` still refuses. No AVD or
Android device is configured, and G18 claims no Android or Flutter build.

### Android SQLite follow-up (2026-08-26)

The missing NDK header is not solved by Jetpack's `BundledSQLiteDriver` alone.
Notrios currently keeps canonical persistence in the Go core: 43 store files
call SQLite's C API through cgo, the adapter links with `pkg-config: sqlite3`,
the one connection opens with `SQLITE_OPEN_FULLMUTEX`, and bootstrap requires
FTS5 plus JSON SQL functions. Android's supported framework surface is
`android.database.sqlite`; SQLite is not listed among the public NDK native
libraries, and the installed NDK contains neither a public `sqlite3.h` nor a
linkable `libsqlite3` development boundary.

Jetpack `androidx.sqlite:sqlite-bundled` does include a native SQLite build and
offers consistent Kotlin/JVM `SQLiteDriver` connections. It does not provide
the header/link contract consumed by Go cgo. Making it canonical would move
database ownership to Kotlin/Room and require a new Go/Kotlin application
bridge, or risk two engines sharing one database file. Neither is an implicit
packaging change. The driver also compiles SQLite in multi-thread mode, so its
connections require a pool or explicit `FULLMUTEX` handling for concurrent use.

The v0.8 H0 investigation therefore retains Go database ownership and compares
two unselected packages: a checksum-pinned upstream SQLite amalgamation as the
C control, and exact `modernc.org/sqlite`/`modernc.org/libc` pins as the
CGo-free candidate. H0 must resolve version/update cadence, licenses and compile
flags; minSdk and ABI set; symbol/one-engine policy; sandbox/backup/WAL/crash
lifecycle; driver/error/connection semantics; and desktop/Android database
compatibility. It must record `sqlite_version()` and `PRAGMA compile_options`,
execute FTS5 and JSON, run store/snapshot/sync tests, round-trip a checkpointed
database across engines and platforms, measure build/APK/RSS/performance deltas,
and prove no second engine opens the canonical file. The official SQLite
Android bindings remain a useful AAR/JNI source example, not a drop-in Go link
boundary.

### modernc/cznic evaluation (2026-08-26)

`modernc.org/sqlite` v1.57.0 is a BSD-3-Clause, `database/sql`, CGo-free port
generated from SQLite C sources; it pins `modernc.org/libc` v1.74.4 and reports
SQLite 3.53.3. On this host its Linux runtime reported `ENABLE_FTS5`, executed
FTS5 and JSON, used file-backed WAL, and passed `integrity_check`. A disposable
C 3.45.1 → modernc 3.53.3 → C 3.45.1 WAL/FTS5/JSON round trip also passed. That
is file-format evidence, not a promise that independent executions produce
byte-identical database files.

The same disposable module cross-built Linux/amd64, macOS/arm64, and
Windows/amd64 executables. More importantly, it built an Android/arm64
executable with CGo disabled and a 9,469,448-byte `c-shared` library with the
API-35 NDK compiler. Android selects the generated Linux/arm64 port. Upstream's
published support table nevertheless omits Android and iOS, and zero configured
AVDs/devices initially meant the Android artifact could not be loaded. The
later emulator follow-up below closes that gap for API-35 x86_64 only. iOS
selected the Darwin/arm64 sources but the Go target requires Apple external
linking, which this Linux host cannot validate. These are candidate facts, not
general mobile support claims.

### Android emulator runtime follow-up (2026-08-26)

KVM is installed and usable. Two AVDs now exist and Flutter detects both: the
modern `android emulator create medium_phone` profile automatically selected an
Android 16/API-36 Google Play x86_64 image, while `avdmanager` created the exact
Android 15/API-35 Google APIs x86_64 evidence target. The latter cold-booted
headlessly, connected through ADB, ran with SELinux enforcing, and was stopped
after the tests. No further user configuration is required for emulator tests.

The exact modernc/libc probe cross-built for Android/amd64. Go requires external
linking for that Android target, so the API-35 NDK clang and `CGO_ENABLED=1`
were required even though `modernc.org/sqlite` and its selected Linux/amd64
generated port contained zero CGo files. Inside the emulator, SQLite 3.53.3
reported `ENABLE_FTS5`, `THREADSAFE=1`, and `MUTEX_PTHREADS`; FTS5 MATCH, JSON,
file-backed WAL, and `integrity_check` passed. Two close/reopen runs and an
emulator reboot preserved the file and increased the expected FTS5 count from
one to three. A 9,758,432-byte Android x86_64 Go `c-shared` feasibility library
also passed `dlopen`/`dlclose`.

This retires only the x86_64 emulator runtime pre-gate. The repository still has
no Flutter Android application or real versioned Notrios ABI, and the current
direct-C store still lacks its selected Android SQLite package. H0 must still
run both complete store candidates, schema-v27, snapshot/sync/failure and
performance/resource gates; Android/arm64 and physical devices remain open.

The supplied Android CLI reference was partly inaccurate. Command-line tools
22.0.0 do deprecate `sdkmanager` in favor of `android sdk`, but the new package
ID is slash-separated, `android sdk --licenses` and `--accept-licenses` are not
supported, and no documented `.androidrc` license override was used. The
profile-based `android emulator create` cannot pin an exact image, and official
`android run` deploys supplied APKs without building them. No `JAVA_HOME`
override was needed: Android provisioning used the PATH-selected Temurin 21;
Flutter independently used its configured Android Studio JBR.

The driver is not a Flutter Web solution: `GOOS=js GOARCH=wasm` failed because
the required `modernc.org/libc` platform packages select no files. Web remains a
REST client unless a separate browser-local SQLite/Wasm/OPFS store and ownership
contract is approved.

The maintainer's May 2025 benchmark does not justify “slightly slower” or a
universal ranking. Against `mattn/go-sqlite3`, modernc was about 3.2–4.8× slower
in representative Linux/macOS bulk insert cases while often faster in repeated
or concurrent reads. It used older driver versions, `journal_mode=DELETE`,
`synchronous=FULL`, and two averaged runs—not Notrios WAL/FTS5/snapshot/sync.
H0 therefore runs both candidates on one emulator and a production-shaped
Notrios corpus before choosing mobile or changing the current desktop store.

## Milestone split

- **v0.7 G18 (complete):** audited and froze the facade/ABI/platform/Mermaid
  contract; no ABI implementation.
- **v0.8:** investigate the facade seam, implement and test the headless C ABI,
  package desktop libraries, run an Android-emulator host smoke test, and prove
  current Wails/headless behavior remains unchanged. iOS and physical devices
  remain documented gates.
- **v0.9/v1.0:** freeze the ABI candidate, include headers/artifacts for the
  supported pre-1.0 build matrix, and publish compatibility/ownership examples.
- **post-1.0:** build the Flutter app; test physical Android first and iOS when
  Apple tooling is available, followed by supported desktop targets.

## Markdown and Mermaid

`flutter_smooth_markdown` currently advertises a Markdown editor and renderer,
LaTeX, and a native Flutter Mermaid implementation on all Flutter platforms. It
is a useful candidate, not an architectural decision. A post-1.0 spike must
check Markdown round-trip fidelity, Notrios link/resource interception,
sanitization, the supported Mermaid grammar versus Mermaid.js, accessibility,
large-note/diagram performance, maintenance, and its BSD-3-Clause dependency
tree before pinning a version.

The current React GUI does **not** support Mermaid today. `md-editor-rt` has an
optional Mermaid extension, but `web/src/editor-assets.ts` deliberately sets
`noMermaid: true`: v0.5 E6a disabled optional components that otherwise fetched
runtime code/assets from a CDN. Enabling it requires a local pinned dependency,
no-network evidence, preview sanitization/CSP tests, malformed and oversized
diagram bounds, browser tests, and a Wails smoke test. Until that slice passes,
the honest behavior is a fenced code block rather than a diagram.

G18 freezes exact candidate refusal bounds for v0.8: 64 KiB source, 500 nodes,
1,000 edges, 256 KiB of labels, a two-second render deadline, and a 64 MiB
browser-heap delta. Boundary-plus-one, malformed syntax, script/handler/unsafe-
URL payloads, timeout/cleanup, theme, offline cache-disabled, CSP, accessibility,
and Wails fixtures must pass. Rendering uses strict security mode and sanitizes
the generated SVG before insertion; every refusal keeps the fenced source
visible. These are enablement gates, not a claim that rendering works today.

## Current React editor search

Notrios currently pins `md-editor-rt` 6.5.3 with `@codemirror/search` 6.7.1.
Source inspection shows that md-editor-rt installs CodeMirror's search keymap,
and the Notrios `MdEditor` integration does not exclude it. A rendered browser
pass on 2026-08-26 opened the editable-note panel with Ctrl+F and verified
Find/Replace, next/previous/all, match case, regexp, by word, single replace,
replace all, persistence after Save, desktop containment, narrow-panel reflow,
and zero console warnings/errors. CodeMirror's default keymap defines Mod-f,
F3/Mod-g navigation, and related selection commands; it does not define
Ctrl/Cmd+H. Replacement is exposed inside the Ctrl/Cmd+F panel.

## Secret storage

`flutter_secure_storage` is a plausible Flutter-side provider and advertises
Android, iOS, Linux, macOS, Windows, and experimental Web support. It is not a
drop-in decision for the Go core: Linux requires libsecret plus a keyring
service, Windows has build/runtime details, Android backup/migration behavior is
security-relevant, and Web storage is explicitly experimental. v0.8 keeps the
Go secret-store interface injectable; the Flutter spike validates and pins its
own provider per platform.

G16 exercises that injectable interface with an explicitly warned owner-only
file provider and designs the React Sync Center down to a 390×844 touch layout,
but it does not claim a Flutter, Android, or other mobile build. The later
client must supply its own validated credential provider and native directory/
document pickers while preserving the same no-password-persistence and
destructive-review boundaries.

## Primary references checked 2026-08-11

- Dart C interop: <https://dart.dev/interop/c-interop>
- Dart web/JavaScript interop: <https://dart.dev/web/libraries>
- Flutter package/FFI guidance: <https://docs.flutter.dev/packages-and-plugins/developing-packages>
- Go build modes: <https://pkg.go.dev/cmd/go#hdr-Build_modes>
- Go cgo pointer rules: <https://pkg.go.dev/cmd/cgo#hdr-Passing_pointers>
- Flutter Smooth Markdown package: <https://pub.dev/packages/flutter_smooth_markdown>
- Flutter Secure Storage package: <https://pub.dev/packages/flutter_secure_storage>

## Follow-up primary references checked 2026-08-26

- CodeMirror search reference: <https://codemirror.net/docs/ref/#search>
- md-editor-rt source repository: <https://github.com/imzbf/md-editor-rt>
- Android NDK public native APIs: <https://developer.android.com/ndk/guides/stable_apis>
- Android framework SQLite guidance: <https://developer.android.com/training/data-storage/sqlite>
- Jetpack `BundledSQLiteDriver`: <https://developer.android.com/reference/androidx/sqlite/driver/bundled/BundledSQLiteDriver>
- Jetpack SQLite releases: <https://developer.android.com/jetpack/androidx/releases/sqlite>
- Kotlin Multiplatform SQLite drivers: <https://developer.android.com/kotlin/multiplatform/sqlite>
- Official SQLite Android bindings: <https://www.sqlite.org/android/doc/trunk/www/install.wiki>
- SQLite amalgamation: <https://www.sqlite.org/amalgamation.html>
- SQLite FTS5 build contract: <https://www.sqlite.org/fts5.html>
- SQLite JSON build contract: <https://www.sqlite.org/json1.html>
- modernc.org/sqlite package and support table: <https://pkg.go.dev/modernc.org/sqlite>
- cznic SQLite driver benchmark: <https://gitlab.com/cznic/sqlite-bench/-/raw/v1.1.3/README.md>
- ccgo v4 package: <https://pkg.go.dev/modernc.org/ccgo/v4>
- SQLite file format: <https://www.sqlite.org/fileformat2.html>
- SQLite multiple-copy corruption warning: <https://www.sqlite.org/howtocorrupt.html>
- SQLite Wasm persistence: <https://sqlite.org/wasm/doc/trunk/persistence.md>
- Android CLI reference: <https://developer.android.com/tools/agents/android-cli>
- Android AVD management: <https://developer.android.com/studio/run/managing-avds>
- `avdmanager` reference: <https://developer.android.com/studio/command-line/avdmanager>
- Emulator command line: <https://developer.android.com/studio/run/emulator-commandline>
