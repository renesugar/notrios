# v0.7 G18 amendment — modernc/cznic SQLite evaluation

Date: 2026-08-26
Model: GPT-5 (exact serving variant unavailable)
Working state: complete investigation; no product dependency selected

## Goal and boundary

Check whether `modernc.org/sqlite` changes G18's Android SQLite premise, compare
the available C-driver performance evidence, and decide its place for Flutter
mobile, desktop, and Web. This amendment changes evidence and the v0.8 H0
investigation design only. It does not add a root-module dependency, replace the
store, build a Flutter app, claim device support, or authorize G18b.

## Result

Keep canonical SQLite ownership in the Go shared core. Jetpack's driver is not
a per-query bridge for the current Go store; moving ownership would be a
separately approved whole-store redesign and two engines must never open the
same canonical file concurrently.

H0 now compares two candidates on the same Android emulator and corpus:

- a checksum-pinned upstream C amalgamation is the control;
- exact `modernc.org/sqlite` v1.57.0 and `modernc.org/libc` v1.74.4 pins are the
  promising CGo-free candidate, not a selection.

The modernc Linux runtime reported SQLite 3.53.3 and `ENABLE_FTS5`; FTS5 MATCH,
JSON, WAL, and integrity checks passed. A C 3.45.1 → modernc 3.53.3 → C 3.45.1
WAL/FTS5/JSON round trip passed. Linux/amd64, macOS/arm64, and Windows/amd64
executables cross-built. Android/arm64 produced both a CGo-disabled executable
and a 9,469,448-byte NDK `c-shared` artifact by selecting the generated
Linux/arm64 sources. This is build feasibility only: upstream does not list
Android or iOS among supported targets, and no AVD/device existed. iOS still
requires an Apple external-linking host. `js/wasm` failed at absent
`modernc.org/libc` platform files, so it is not the Flutter Web adapter.

The supplied “slightly slower” performance claim is not defensible. In the
maintainer's pinned May 2025 data, modernc bulk inserts were roughly 3.2–4.8×
slower in representative Linux/macOS cases, while it was often faster on
repeated and concurrent reads. The benchmark itself warns that two-run,
application-specific results are not scientific; it uses old driver versions
and DELETE/FULL rather than Notrios WAL/FTS5/snapshot/sync. H0 must measure the
actual workload, package/build size, latency, CPU, allocation, and peak RSS.

Automated C translation and a matching SQLite release improve provenance but
do not prove perfect semantic parity. v1.57.0 even carries a pre-upstream
journal-recovery patch, and exact `modernc.org/libc` pinning is a documented
requirement. SQLite-format interoperability is the correct contract, not
byte-identical files from different executions.

## Evidence and validation

- Machine record: `performance/v0.7-g18/MODERNC_SQLITE_EVALUATION.json`
- Updated decision record: `performance/v0.7-g18/ANDROID_SQLITE_FOLLOWUP.json`
- G18 evidence suite: seven tests and the source validator pass.
- Disposable module and all binaries stayed under `/tmp`; root `go.mod` and
  product code are unchanged.
- Primary sources are recorded in the machine evidence and
  `FLUTTER_GO_CLIENT.md`.

The remaining H0 gates cover same-emulator build/load and runtime features,
schema-v27/store/snapshot/sync/failure behavior, bidirectional checkpointed
database interoperability, exact pins/licenses/update policy, Notrios workload
performance and resource cost, driver semantics, and the single-owner rule.
