# Flutter/Go client and shared-core contract

Status: planning contract, reviewed 2026-08-11. It does not claim that a C ABI,
Flutter client, or Mermaid support is implemented.

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

Prefer caller-owned input buffers that remain valid only for the duration of a
call. For outputs, compare a size-query/caller-owned-buffer convention with an
allocation plus `notrios_free`; whichever is selected must have leak, double-
free, wrong-handle, cancellation, and concurrent-shutdown tests. Never expose a
pointer to Go-managed memory or a live Store/service object. `runtime/cgo.Handle`
may back opaque handles internally, subject to Go's cgo pointer rules.

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

## Milestone split

- **v0.7 G18:** audit and write the facade/ABI/platform/Mermaid contract; no ABI
  implementation.
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
