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
Studio, bundled-Java, and license checks. No AVD or Android device is configured,
and G18 claims no Android or Flutter build.

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
