# v0.8 Android-emulator bounds checklist

G2 measured an amd64 Linux desktop process. It makes no Android performance or
memory claim. The v0.8 shared-core/FFI slice must run this checklist before
retaining the provisional limits.

- Build the real bounded decoder for Android arm64 and x86_64; record OS image,
  API level, ABI, emulator memory, Go version, and build flags.
- Decode, verify, and apply synthetic 100- and 10,000-operation envelopes.
  Confirm exact round trips, cancellation, no UI-thread work, and incremental
  PSS/RSS. Repeat after a warm run and after process restart.
- Refuse a 10,001st operation, a record over 1 MiB, a payload over 512 KiB,
  more than 64 dependencies, compressed bytes over 4 MiB, expanded bytes over
  16 MiB, and expansion over 64:1 before canonical writes.
- Require one maximum envelope to finish within 2 seconds on the configured
  emulator and use at most 64 MiB incremental PSS. A miss lowers the envelope
  bound; it does not get explained away as an emulator artifact.
- Stream a 1 MiB fixed chunk and range-resume a 108 MB generated attachment.
  Verify each chunk hash before atomic whole-resource admission; unavailable
  bytes remain visible rather than becoming an empty resource.
- Fill the 10,000-operation/64 MiB per-peer pending limit using unavailable
  dependencies. Confirm disk-backed accounting, foreground editing, explicit
  backpressure, restart recovery, and snapshot-repair escalation.
- Read a 64 MiB/4,096-object sync pack with a 4 MiB trailer ceiling without
  loading the pack body into heap. Record PSS, open descriptors, and time.
- Run malformed gzip, truncated NCB1, non-minimal varint, duplicate/replayed
  operation, count overflow, chunk-hash mismatch, and cancellation cases.
- Retain, lower, or split each provisional G2 number in a committed v0.8
  evidence report. Do not generalize emulator results to physical devices.
