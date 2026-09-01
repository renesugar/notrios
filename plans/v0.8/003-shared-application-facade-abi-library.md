# v0.8 H1 — Shared application facade and ABI-major-1 library

Date: 2026-09-01
Status: complete
Model: Claude Opus 5 (`claude-opus-5`) via Claude Code 2.1.252, parent-owned
throughout; no subagents were used.

## Outcome

H1 extracts one transport-neutral application facade, migrates the REST note
surface onto it, replaces the system SQLite linkage with the H0-pinned vendored
amalgamation, and lands the frozen 12-symbol ABI-major-1 C library over the same
facade. REST behavior is unchanged and that is enforced by test rather than
asserted. Business semantics did not change; no schema, route, or support claim
moved.

The work was delivered as four separately committed slices at the user's
direction, each leaving the tree green so a usage-limit interruption could not
land mid-rewrite.

| Slice | Commit | Content |
|---|---|---|
| A | `7810dc9` | `internal/application` facade, typed error model, guards |
| B | `e1ba989` | REST orchestration migrated onto the facade |
| C | `ba18209` | Vendored SQLite 3.53.4, static hidden linkage |
| D | `6d57d8e`, `5b2b215` | `cmd/notrioslib` ABI-major-1 C library |

## Slice A — the facade

`internal/application` owns the application contract, as H0 selected.
`internal/service` was not narrowed because it imports both `internal/httpapi`
and `net/http`, so operations expressed there inherit an HTTP shape whether or
not they want one.

The package exposes notes, revisions, search, and resource operations with no
transport, serialization, or storage concerns. `Kind`/`Error` give adapters one
classification to switch on instead of matching error strings, and
`kindFromStore` mirrors `writeStoreError`'s ordering so slice B's parity review
was a line-by-line comparison. Context cancellation passes through unchanged, so
`errors.Is(err, context.Canceled)` keeps working for every select-driven caller.
`ResourceStream` bounds reads and is safe to close twice, which the ABI needs
because a host cannot be trusted to release a handle exactly once.

`Repository` is a narrow eleven-method seam onto the 124-method store interface
and is the one exported declaration permitted to name store types.
`TestExportedSurfaceIsTransportAndStorageNeutral` walks the package AST and
fails on any other store-typed export, so the transitional exception cannot
widen without an explicit edit to a named allowlist. Both architectural guards
were verified to fail against deliberate violations before being trusted.

32 tests, 97.2% statement coverage. Behavioral tests run against the real
in-memory SQLite store and compare facade results with direct store reads,
because a mock can only prove the facade agrees with itself.

## Slice B — REST migration

Nineteen call sites in `server.go` and `noteops.go` now call the facade: create,
get, put, patch, delete, document body, revision list/read/restore, resource GET
and HEAD metadata, outline, remote-media scan, append/prepend including its
conflict retry, the read-only-notebook guard, line ranges, and in-note search.
Zero facade-covered store calls remain in those files.

`internal/httpapi/application_adapter.go` re-expresses the existing
`writeStoreError` table over `application.Kind`. `applicationMessage` recovers
the store's original message, because the facade decorates errors with operation
and kind, which is right for a log and wrong for a response body that clients
already depend on.

`TestApplicationErrorMatchesStoreErrorResponse` drives both writers with the
same eight store errors and requires byte-identical status and body; it was
verified to fail on a single reworded message. `TestNoteAndDocumentRenderIdentically`
requires `api.Document` and `api.DocumentRevision` to marshal identically from
either path. The complete unchanged `internal/httpapi` suite passing after each
migration step is the regression proof.

**Two handlers deliberately stay on the store, documented where they live:**

- `handleResourceContent` needs the `io.ReadSeeker` that `http.ServeContent`
  uses for HTTP Range. The facade's bounded forward-only `ResourceStream` hides
  it, so routing this through the facade as it stands would silently drop range
  support.
- `searchMerged` would make the facade own the optional Recoll sidecar and the
  query parser. That is a design decision about where sidecar search belongs and
  deserves its own slice rather than a fork inside this one.

`searchInNote` now takes an id and a body rather than a document type, so the
REST and MCP adapters can migrate on separate schedules. **The MCP adapter is
unchanged and still calls the store directly**; H1's scope was REST.

## Slice C — vendored SQLite

The archive was downloaded from the H0-recorded URL and verified before anything
was staged: official SHA3-256 `628a44cf…27934e`, SHA-256 `1e71ddf9…22e87d`,
`sqlite3.c` `b1dd5d74…b28189`, `sqlite3.h` `919e7f2e…5910e1d`, all matching the
pin exactly. Version 3.53.4, source id
`2026-07-24 19:02:57 bf7c7f30031888f4e796e429ab3978879485813aaca6f641c7b33e4e09459bcc`.

Only `sqlite3.c` and `sqlite3.h` are vendored under `internal/store/csqlite/`,
per H0 policy, with `NOTICE` and `PROVENANCE.json`. `shell.c` and `sqlite3ext.h`
were deliberately omitted. `internal/store/sqlite3_amalgamation.c` is a shim so
cgo compiles the amalgamation while third-party source stays in its own
directory; `internal/store/sqlite_cgo.go` is the single build owner and carries
the H0 compile options.

Hidden visibility is the security-relevant part. `readelf` reports 275
`GLOBAL HIDDEN` sqlite3 symbols and **zero GLOBAL or WEAK symbols with DEFAULT
visibility**. The 12 `DEFAULT` symbols present are all `LOCAL` binding — GCC
`.cold`/`.part.0` clones, one static function, and the filename symbol — and
LOCAL symbols never export from a shared library regardless of visibility.
Slice D confirmed this on the real `.so`: zero exported `sqlite3_*`.

`scripts/check_sqlite_provenance.py` re-hashes the vendored files, refuses
undocumented or missing compile options, and fails if anything reintroduces
`pkg-config` or a system `<sqlite3.h>`. It is in the scaffold gate and was
verified to catch an edited source, a changed option, and weakened visibility.
It immediately found **33 store files** including `<sqlite3.h>` that resolved to
the vendored header only through the `-I` flag and would have silently fallen
back to a system header if that flag ever moved. All 43 SQLite-API files now
include it explicitly, matching the count H0 recorded.

`internal/store/sqlite_pin_test.go` interrogates the running library through SQL
rather than compile-time constants: version, source id, `PRAGMA compile_options`,
a rejected double-quoted literal, a refused `load_extension`, JSON, and WAL on a
file database.

## Slice D — the C ABI

`cmd/notrioslib` exposes the frozen twelve-symbol contract over the facade.
Logic lives in `internal/abi` because Go forbids cgo in `_test.go` files, so
anything left in the cgo shim would be untestable by construction.

Handles carry a 32-bit generation beside the slot. Slots are reused, generations
are not, so a host holding a closed handle gets `stale_handle` rather than
resolving to whatever now occupies the slot — a failure that would not be a
crash but a silent write to the wrong library.

Ownership is enforced twice as H0 required: canonical-path identity inside the
process, and a lifetime-held non-blocking exclusive `flock` across processes.
Each catches what the other cannot see. The lock is non-blocking because waiting
would make "another process owns this" indistinguishable from a deadlock.

Shutdown follows H0's fixed order and releases the owner lock only after the
store is closed; releasing earlier would let a second owner open the database
while this one still had it open.

Operations are named in a bounded JSON request rather than each getting a C
symbol, so a new operation is not an ABI break and an unknown one is a runtime
`not_found` instead of a link failure. Requests and stream reads are bounded at
1 MiB; the event queue is bounded at 256 and counts drops. A result exceeding
the response bound is refused with a pointer to the stream, never truncated,
because short JSON parses as complete and would be believed.

28 tests, 85.3% coverage, clean under `-race`.

## Validation

- `go test ./...`, `go vet ./...`, `gofmt` clean.
- `internal/application`: 32 tests, 97.2% coverage.
- `internal/abi`: 28 tests, 85.3% coverage, clean under `-race`.
- `internal/httpapi`: complete suite unchanged and passing; 24 new adapter
  parity assertions.
- `internal/store`: complete suite passing against the vendored engine in 57
  seconds; 6 new pin tests.
- Built `c-shared` and `c-archive`. The shared library exports **exactly the 12
  frozen symbols**, diffed byte-identical against H0's contract header, with
  **zero exported `sqlite3_*`** and no dynamic SQLite dependency.
- C host acceptance test: 19 checks through `notrios_abi.h` alone, including
  double release refused, foreign-pointer release refused, second owner refused,
  stale handle after close, and reopen. Run with `make abi`.
- `python3 scripts/check_sqlite_provenance.py`, `python3 scripts/check_required_files.py`
  (234 files), and `bash scripts/validate-scaffold.sh` pass.

## Defects found and fixed during the work

- **`dispatch` misclassified payload errors.** It re-derived status from the
  error and overwrote the `StatusInvalidArgument` `invoke` had chosen, so a
  malformed operation payload was reported as `internal` and a host could not
  tell its own bad JSON from a library failure. Found by extending tests to
  every operation; `decode` now returns a facade invalid-input error.
- **A vacuous test.** `TestSearchHitSourcesAreCopiedNotAliased` checked
  defensive copying of `SearchHit.Sources`, but the store never populates
  `SearchSources` — only the Recoll merge path does — so it returned early every
  run. Replaced with a version driven by a test double.
- **Ambiguous SQLite includes.** See slice C; 33 files were relying on `-I`
  ordering.

## Limitations and follow-ups

- The MCP adapter still calls the store directly. Migrating it is a scoped
  follow-up, unblocked by `searchInNote`'s decoupling.
- `handleResourceContent` and `searchMerged` remain on the store for the reasons
  above; each is a candidate slice.
- The ABI's operation set is read-oriented plus note create/update/delete. It is
  deliberately smaller than REST; extending it is not an ABI change.
- Cold `internal/store` builds take roughly 3m40s while the 9.5 MB amalgamation
  compiles, cached thereafter. The repository gained 9.8 MB of tracked
  third-party source.
- Android was not exercised in H1. H0's API-35 x86_64 evidence stands; H11 owns
  the emulator acceptance run and arm64 remains build-only.
- Frozen v0.7 evidence describing the pre-vendoring state was left intact. The
  G18 validator now excludes the directives-only cgo build owner so its recorded
  43-file finding still holds; any other new cgo file still fails it.
- No push, PR, merge, tag, release upload, evidence-reserve write, ISO, or
  physical burn was performed.
