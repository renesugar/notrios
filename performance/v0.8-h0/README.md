# v0.8 H0 application/ABI/SQLite investigation

H0 selects a new `internal/application` facade owner, a checksum-pinned SQLite
3.53.4 amalgamation owned by the Go core, ABI major 1 with the frozen 12-symbol
polling/stream surface, and API-35 x86_64 as the only runtime-qualified Android
target. Android arm64-v8a is build-only until H8 obtains equivalent runtime
evidence. These are implementation inputs for H1, not production changes.

The strongest result is the real-store run: the unchanged schema-v27 store,
snapshot, and sync code passed against the hidden static amalgamation on Linux
and on the API-35 emulator. A real-store `c-shared` probe bootstrapped schema
v27 with no dynamic SQLite dependency and no exported `sqlite3_*` symbols.
The separate ABI harness passed c-shared/c-archive headers, exact symbols,
caller/library ownership, generation handles, cancel/poll/events, streams, and
the live-to-shutting-to-stale transition on Linux and Android.

The exact modernc v1.57.0/libc v1.74.4 candidate remains file-compatible: a
checkpointed desktop-C database was updated on Android by modernc and verified
again by desktop C with integrity and the exact new row intact. It is not the
v0.8 selection because it cannot exercise the current real store without
rewriting 43 direct-C files, upstream does not list Android support, and the
measured executable used more RSS and disk.

Contents:

- `REPORT.json` — decisions, pins, costs, Android matrix, and limitations.
- `source-audit/` — deterministic current-source dependency/cost audit.
- `facade-probe/` — typed transport-neutral facade over the real store.
- `abi-probe/` — disposable frozen 12-symbol lifecycle harness.
- `storelink-probe/` — real schema-v27 c-shared/static-engine proof.
- `sqlite-probe/` — reproducible checksum-pinned C/modernc A/B and owner lock.
- `validate_evidence.py` — fail-closed report/source/probe validator.

No downloaded amalgamation, module cache, database, binary, private content,
or emulator image is tracked. The official SQLite archive hash and every
candidate dependency/license hash are recorded in `REPORT.json`.
