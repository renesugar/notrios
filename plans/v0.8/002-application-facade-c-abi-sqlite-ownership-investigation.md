# v0.8 H0 — Application-facade, C-ABI, and SQLite ownership investigation

Date: 2026-08-31
Status: complete investigation; H1 inputs selected, no production adoption
Model: GPT-5 (Codex; exact serving variant unavailable), with bounded Luna
workers for inventories and disposable probe scaffolding; architecture,
experiment design, selection, corrections, and final review remained with the
parent.

## Outcome

H0 selects a new `internal/application` owner for transport-neutral use cases,
the official SQLite 3.53.4 amalgamation as the one SQLite engine statically
linked into the Go core, and the frozen ABI-major-1 12-symbol polling/stream
contract. The only Android runtime-qualified target is the existing API-35
x86_64 emulator. Android arm64-v8a builds pass but remain build-only; H8 must
repeat the runtime matrix on arm64 before support is claimed.

This is an investigation result, not an implementation. No production store,
facade, ABI, dependency graph, schema, API, or platform-support declaration
changed. H1 is authorized only by a separate user instruction.

## Facade decision

The current source audit reproduced G18's dependency baseline with zero numeric
drift: 113 registered routes, 109 non-HEAD OpenAPI operations and handler
methods, 24 production HTTP files importing `net/http`, 128 `s.store` call
sites, 20 concrete sync-store call sites, four concrete `*store.SQLiteStore`
type sites, a 124-method store interface, and 43 store files that directly use
the SQLite C API. `internal/service` imports both `internal/httpapi` and
`net/http`, so narrowing it would preserve the wrong ownership direction.

H1 must create `internal/application`, expose application request/result/error
types with no HTTP, JSON-RPC, SQL, Wails, or filesystem-path concerns, and keep
HTTP/MCP/ABI/Flutter as adapters. A disposable real-store facade proved typed
create/get/update/search, stale-revision conflict mapping, not-found and
context cancellation, and a bounded cancelable resource stream. During the
transition, the store-backed implementation may wrap `store.Store`, but the
facade's exported operation contract must not expose it.

## SQLite decision and provenance

The selected pin is official SQLite 3.53.4,
`sqlite-amalgamation-3530400.zip`:

- official SHA3-256:
  `628a44cfe82c66aed1ccbbe85a562d2e33ebe64b3288981ed76285612227934e`;
- recorded SHA-256:
  `1e71ddf93849c6a6ecf58b827c0692073d2dd7ee40196158068f7b29f422e87d`;
- `sqlite3.c` SHA-256:
  `b1dd5d74ec7f29055a6684fa06fb3c2f6821c87dd38f9a458dfd2e8a1db28189`;
- `sqlite3.h` SHA-256:
  `919e7f2e8ed1d8f56ac17b412b8971c76aa5d1a879752cc6058f75e7d5910e1d`;
- license: SQLite public domain, compatible with the Apache-2.0 project.

The tested and selected compile policy is `SQLITE_THREADSAFE=1`, FTS5, the
JSON1 compatibility define, `SQLITE_DQS=0`, no loadable extensions, secure
delete, and URI filenames. Runtime policy retains FULLMUTEX opens, WAL, the
five-second busy timeout, checkpoint-before-handoff, and the Online Backup API
for physical snapshots. H1 must compile with hidden visibility, export no
`sqlite3_*` symbol, and dynamically depend on no second SQLite library.

The rejected v0.8 candidate remains exactly `modernc.org/sqlite` v1.57.0 with
`modernc.org/libc` v1.74.4 and embedded SQLite 3.53.3. Its BSD-3-Clause,
public-domain SQLite, MIT sqlite-vec, libc, and third-party notices are all
hashed in `performance/v0.8-h0/REPORT.json` and are license-compatible. It was
not selected because:

- the unchanged real store cannot use it without replacing 43 direct-C files,
  including backup and driver semantics;
- upstream does not list Android in its supported-platform table;
- it is one SQLite patch behind the selected official pin;
- its measured workload binary was roughly 7.3–7.8 times larger and used
  roughly 2.3–3.1 times the RSS of the C control;
- the C control passed the actual schema-v27 store/snapshot/sync code on both
  Linux and Android.

Its advantage is a faster measured cold build in this run and no direct cgo
SQLite API in a future rewritten store. That did not outweigh the rewrite and
runtime/support costs for v0.8.

## Real-store, compatibility, and cost evidence

The full desktop package set passed against the hidden static 3.53.4 control:
`internal/store`, `snapshotimage`, `syncstate`, `syncmerge`, `syncwire`,
`synccarrier`, `syncrest`, `synccatchup`, and `syncjobs`. The store package took
175.010 seconds. Its 17,243,320-byte test executable had no dynamic
`libsqlite3` dependency.

On the API-35 x86_64 Android 15 emulator, the focused real store matrix passed
in 26.857 seconds. It covered migrations through schema v27, CRUD/revisions,
FTS5 search, generated data, resources, sanitized physical snapshots,
three-replica sync admission and dependency drain, restart/pre-commit failure,
journals, metadata convergence, retention, resource repair, and conflict
resolution. A real-store `c-shared` probe independently bootstrapped schema v27
using SQLite 3.53.4 with no dynamic SQLite and no visible `sqlite3_*` symbols.

The identical 1,000-note FTS5/JSON/WAL/integrity workload measured:

| Target/candidate | Wall | Max RSS | Artifact |
|---|---:|---:|---:|
| Linux C 3.53.4 | 73 ms | 3,584 KiB | 1,397,896 B |
| Linux modernc 3.53.3 | 637 ms | 11,264 KiB | 10,893,119 B |
| Android x86_64 C 3.53.4 | 94 ms | 5,120 KiB | 1,471,848 B |
| Android x86_64 modernc 3.53.3 | 155 ms | 11,904 KiB | 10,776,816 B |

Cold Linux builds took 268,197 ms for the amalgamation/C harness and 155,712
ms for modernc with isolated caches. These small process-level measurements
include startup and adapter overhead and are selection evidence on this host,
not universal product latency claims.

A checkpointed desktop-C database crossed to Android, where modernc verified
FTS5/JSON/WAL/integrity and inserted exact row 1001. After checkpoint and close,
the pulled 245,760-byte file matched the emulator SHA-256
`36ddb65cfdba5956aa8350bcf3f89ca00d93fd17c564853f9869cab1d4e9b7b7`.
Desktop C then verified integrity, WAL, all 1,001 rows, and the exact modernc
row. No engines opened the file concurrently.

Android arm64-v8a build-only evidence includes the ABI shared library, C and
modernc workloads, real-store test executable, and real-store shared library.
Exact byte sizes and SHA-256 hashes are in the report. None ran, so there is no
arm64 runtime claim.

## ABI, owner, and crash lifecycle

The disposable ABI harness passed `c-shared` and `c-archive`, generated
headers, the exact frozen 12-symbol set, numeric ABI major, caller-borrowed
input copying, C-owned output allocation and exact-once release, 64-bit
generation handles, wrong/stale/foreign/zero refusals, one-byte-over-limit
refusal, cancellation, polling, events, partial streams/EOF, and concurrent
live-to-shutting-to-stale close on Linux. The same C host passed an NDK API-35
x86_64 shared library on Android. There are no callbacks from Go threads and
no Go pointer enters C-visible memory. `dlclose` is not a lifecycle primitive;
hosts close instances before process teardown.

H1 must enforce one owner twice: canonical-path identity in the process
registry and a lifetime-held nonblocking exclusive sidecar lock across
cooperating processes. The negative prototype acquired the first owner and
refused the second with exit 3/`Resource temporarily unavailable`. Packaging
must not include Room, Jetpack SQLite, a Flutter SQLite plugin, or another
SQLite library on the canonical-file path. This policy is selected but not yet
implemented in production.

Shutdown order is: refuse new work, cancel calls/jobs, close streams, join
workers, checkpoint and close SQLite, release the owner lock, and invalidate
the generation. Live raw-file transfer is forbidden; use the Online Backup API
or a clean TRUNCATE checkpoint/close. Android passed the selected snapshot,
restart/injected-precommit, and close-rollback tests.

## Update and rollback policy

H1 may vendor only `sqlite3.c`, `sqlite3.h`, the public-domain notice, and an
exact provenance manifest. Each later update is its own reviewed slice and
must verify the official SHA3, record source/file SHA-256 and source ID, diff
compile options, rerun both target matrices, ABI/symbol/owner checks, the
checkpointed round-trip, measured costs, and license inventory. The previous
pin remains available until release acceptance succeeds.

Before release acceptance, rollback removes the vendored build wiring and
retains the existing desktop `pkg-config` adapter. Schema-v27 databases remain
ordinary SQLite files; checkpoint and close before a rollback build opens one,
and never switch an open file between engines.

## Evidence and validation

- Decision report: `performance/v0.8-h0/REPORT.json`
- Source audit: `performance/v0.8-h0/source-audit/`
- Facade probe: `performance/v0.8-h0/facade-probe/`
- ABI lifecycle probe: `performance/v0.8-h0/abi-probe/`
- Real-store shared-link probe: `performance/v0.8-h0/storelink-probe/`
- SQLite A/B and owner probe: `performance/v0.8-h0/sqlite-probe/`
- Machine validator: `python3 performance/v0.8-h0/validate_evidence.py`

The official archive and all binaries, databases, caches, and emulator files
remained temporary and are not tracked. The disposable emulator directory was
removed, the emulator was stopped cleanly, and no H0 process remains. No
GitHub push, PR, merge, tag, release/upload, evidence-reserve write, ISO, or
physical burn was performed.
