# v0.8 H10 — Wails v3 migration spike

**Recommendation: migrate, after v3 reaches a release version.** `REPORT.json`
is the decision; `MEASUREMENTS.json` is what it was decided from; this file says
how to reproduce both.

```
bash performance/v0.8-h10/run_spike.sh
```

It builds the production v2 shell and the prototype from the same tree, runs
both under Xvfb, and writes the measurements. `validate_evidence.py` checks the
committed record offline and, more usefully, reads `go.mod` to confirm the
production module never learned that v3 exists — a spike that quietly added a
dependency would otherwise pass on its own say-so.

## The prototype is isolated by construction, not by promise

`prototype/` is a **nested module**. `go build ./...` and `go test ./...` at the
repository root do not see it, and the root `go.mod` has no v3 requirement.
Rolling back is deleting the directory.

Its module path stays under `github.com/renesugar/notrios/` for a reason worth
knowing: Go's internal-package rule is about import paths rather than modules,
so the prototype can import `internal/service` and serve **the real handler**.
It renders the real frontend against a real library over the real `/api/v1`. A
spike that served a stub would have measured a stub.

## What it found

Everything the production shell does ports, and the binary gets lighter: eight
linked Go modules against v2's sixteen. Two shape changes need care, and one is
the actual work:

- **`ShouldQuit` replaces `OnBeforeClose`**, without a context.
- **A question dialog no longer returns the button.** v2's
  `runtime.MessageDialog` hands back the answer; v3's `Show()` returns nothing
  and each button carries an `OnClick`. The unsaved-work guard has to decide
  *now*, so it waits on a channel and fails closed — which is what production
  already does when a dialog cannot be shown.
- **`window.go.main.X.Y` does not exist in v3.** Zero occurrences in its
  compiled runtime. The frontend has eleven references across seven files, and
  a migration rewrites them onto `@wailsio/runtime` (MIT) or generated bindings.

And the native stack changes: GTK3 + webkit2gtk-4.1 becomes GTK4 +
webkitgtk-6.0. Six documented install lines say `libwebkit2gtk-4.1-dev` today.

## What it did not measure

Calling a bound service from JavaScript — the frontend still speaks the v2
convention, so services are proven to *register*, not to answer. Nor packaging a
`.deb` against v3, the desktop journey harness under v3, deep links end to end,
or any platform but Linux. Those are the migration's remaining unknowns and they
belong to whoever schedules it.
