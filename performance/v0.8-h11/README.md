# v0.8 H11 — Android-emulator shared-core acceptance

What this proves: the H1 shared core is a viable backend on one Android
emulator. What it does not prove is longer than what it does, and both are in
`REPORT.json` rather than in prose that can drift from it.

## Running it

```
ANDROID_HOME=~/Android/Sdk bash performance/v0.8-h11/run_acceptance.sh
```

It needs a booted emulator on `adb` and does not start one: which device an
acceptance runs against is the operator's decision. It takes about eight
minutes, most of it cross-compiling the core twice.

`validate_evidence.py` checks the committed record without a device, and runs
in `make validate`. It is not a second measurement — it cannot be, since it
never runs anything — it holds the record to what was executed: the pinned
device image, the twelve exported symbols, every phase, the check count, and
every claim the run is required to keep disclaiming.

## What runs on the device

`acceptance_host.c` is a C program that links the cross-compiled
`libnotrios.so` through `notrios_abi.h` and nothing else. It is the same kind
of program as `cmd/notrioslib/hosttest/host_test.c` and a different claim: that
one proves the boundary holds where the library was built, and this one proves
it under a different libc, a different filesystem, an enforcing SELinux policy,
a process the system may kill, and a device that reboots.

The phases exist because each can fail on its own:

| Phase | What only it can fail |
| --- | --- |
| `open` | the ABI version, the frozen capability word, and the one-owner rule on Android's filesystem |
| `work` | FTS5 over rows the desktop wrote, create/read/update/revisions, cancellation, the event queue |
| `stream` | a bounded read, and the difference between a read size and a stream budget |
| `abandon` + SIGKILL | what a process killed by the system leaves behind |
| `verify-after-kill` | that committed work survives, over a write-ahead log and an unreleased ownership marker |
| reboot + `verify-after-reboot` | that the file survives the device restarting, not just the process |

The database crosses machines in both directions: the desktop build seeds it and
writes the resource the stream reads, and the file comes back for
`PRAGMA integrity_check`, a WAL-mode check, and searches for the rows Android
wrote. Interchange in one direction would prove half of it.

## What is not claimed

No physical device, no iOS, no user interface, no app-store artifact, no
background or battery behaviour, no production secure store, no Flutter client,
and no arm64 runtime. The credential provider is supplied by the host that
pushes the files; an Android keystore provider is a documented gap rather than a
capability, and H9-F carries it post-v1.0.

arm64 is built and hashed and never executed. The artifact is pushed to the
device once and refused — `CANNOT LINK EXECUTABLE` — and that refusal is
recorded, because a build-only claim nobody tried is a claim nobody checked.
