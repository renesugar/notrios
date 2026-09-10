# v0.9 I8 — freeze the surfaces 1.0 will promise, and push on the ABI's edges

```sh
python3 performance/v0.9-i8/validate_evidence.py   # re-derives the surfaces; runs in make validate
bash    performance/v0.9-i8/run_edges.sh           # symbol + typed ABI freeze, asan host, race tests
python3 performance/v0.9-i8/build_freeze.py        # after a deliberate surface change
```

## The freeze is live, not a record

`validate_evidence.py` re-derives all seven surfaces from the source that
defines them on every run — 91 CLI commands, 113 REST routes, 45 MCP tools, 34
Make targets, 12 ABI symbols, 27 archive-contract files, 60 configuration keys —
and fails if any differs from what was frozen, naming what was added and
removed. Adding a route is not forbidden; adding one silently is.

Both halves of the recorded summary are checked against each other too. Trusting
the stored hash let a **truncated member list pass**: the summary still agreed
with live source while the list it summarised did not. The hash is now recomputed
from the members it claims to summarise.

## Three derivations were wrong before they were right

`make_lifecycle` found 5 targets of 34 — the pattern excluded any target whose
prerequisites contain `=`. `configuration` found **zero** keys, because it looked
for `yaml:` tags in a package that uses `json:`; a freeze of an empty surface
passes for ever. Both were caught by reading the counts rather than the exit
code.

## valgrind is the wrong instrument here, and wrong loudly

Go grows a goroutine stack by allocating a larger one and copying the frames
into it, rewriting pointers as it goes. To memcheck, which tracks addressability
per byte, every one of those writes lands outside a known block. The first run
produced **ten million** invalid-access reports, every frame in `runtime.*`, and
a host that does nothing but call `notrios_abi_version()` produces them too —
the control that settles it. Targeted suppressions removed 9.5 million and it
still hit memcheck's cap.

So each instrument runs where it works:

- **AddressSanitizer** crosses the c-shared boundary. 15/15 edge checks, 0 reports.
- **ThreadSanitizer** cannot: it maps a large shadow region at process start, so
  through a c-shared library it fails with *"failed to allocate … bytes"*, and
  with the host instrumented too, *"unexpected memory mapping"* — with ASLR
  disabled as well. The race detector therefore runs on the Go side against the
  same dispatch, session and handle code the twelve entry points call into. It
  was **proved live before it was trusted**: a deliberately racy probe made it
  fire, then the probe was removed.
- **valgrind** stays behind `I8_VALGRIND=1` for leak accounting, the one number
  it still reports usefully.

## What the edges host asks

Handles that were never issued, a buffer released twice, a pointer the library
never issued, a cancelled call polled to a verdict, two threads on one instance,
and an instance closed with a call outstanding. Every one is refused cleanly —
`INVALID_HANDLE` for a bad handle, `STALE_HANDLE` after a close — and none
corrupts anything.

One check was my own bug first: it polled ten thousand times in a tight loop and
reported that a cancelled call never answered. It answers; the loop never let
the runtime schedule the goroutine that would produce the verdict. Counting
iterations measures the host's scheduling luck. It uses a wall-clock deadline
that yields now.

## What this does not establish

`FROZEN.json` and `EDGES.json` carry the lists and the validator fails if they
shrink: behaviour is not frozen (two releases can agree on every name here and
disagree about what a call does), the sync wire protocol and the installer's
on-disk layout are owned elsewhere, there is no fuzzing, `-msan` did not run,
and nothing tests `dlclose`, a second `dlopen`, or two processes opening one
profile.
