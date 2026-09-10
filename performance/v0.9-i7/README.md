# v0.9 I7 — soak, recover, and freeze what can be claimed

```sh
python3 performance/v0.9-i7/soak.py --minutes 15
bash    performance/v0.9-i7/recovery_drills.sh
python3 performance/v0.9-i7/validate_evidence.py
```

## The soak measures the slope, not the peak

900 seconds, 347,092 requests, 0 errors. Resident memory climbs ~6 MB in the
first thirty seconds and then oscillates inside a 1.16 MB band for the remaining
fourteen and a half minutes; descriptors do not grow at all.

That **rules out the fast failures** this is aimed at — a handle opened per
request, memory that climbs and never returns. It **does not rule out a slow
leak**: the fitted slope over the settled window is +794 kB/hour against a
1.16 MB band, which fifteen minutes cannot distinguish from the sawtooth of a
garbage-collected runtime. Calling that "no leak" would be a claim the run does
not support, so the record says what it can and the validator refuses any
version that says more.

## Two things this found

**`doctor` did not redact.** `paths` and `config show` replace the home
directory with `~` by default and both offer `--no-redact`; `doctor` printed
absolute paths, username and all — and `doctor` is the command whose output gets
pasted into an issue. `paths.Redact`'s own comment says resolved paths are
printed *"in `notriosctl doctor`"* and redacted there, so this was documented
intent nobody had wired up. Fixed, with `--no-redact` for parity, and tested.

**An assertion that tested nothing, in two places.** `notriosctl search` echoes
the query back in its JSON — `{"hits": [], "query": "x"}` — so grepping the
output for the search term matches an *empty* result. I4's restore check passed
for exactly that reason, and the first version of the recovery drill here
concluded that a library it had just deleted still held its notes. Both count
hits now; I4's drill was re-run and records `restored_hits: 1`.

## The matrix claims only what ran

Six rows, two supported. The row most at risk of overclaiming is the desktop
shell: CI **compiles** it and I3's containers are headless, so it sits at level 1
while the command line and service earned level 3 *in those same containers*.
Compiling a GUI proves it links, not that it runs. Windows and macOS are absent
from release claims rather than listed as forthcoming.

The validator derives the shipped platform's level from I3's report, so a row
cannot be promoted by editing the matrix.

## What this does not establish

`REPORT.json` carries the list and the validator fails if it shrinks: fifteen
minutes is not a long-lived soak, the load is one endpoint on loopback with no
writes or concurrency, no emulator soak ran, no crash was induced (the
crash-reporting position is checked by absence — no telemetry dependency, no
sync target on a fresh profile), there is no support bundle to redact so what
was tested is the diagnostics that exist, and `config show` prints a credential
*reference*, which names where a credential lives rather than being one.
