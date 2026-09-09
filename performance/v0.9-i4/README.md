# v0.9 I4 — drills against the command whose job is deletion

Nine faults, each in its own disposable installation: install into an isolated
HOME outside the checkout, put a real note in the library, apply one fault,
check what survived.

```sh
bash performance/v0.9-i4/purge_drills.sh
python3 performance/v0.9-i4/validate_evidence.py
```

The drills need a build (`make build web`). The validator needs nothing — it
reads the record, which is why it can run in `make validate` on a machine where
running the drills would be inappropriate.

## Why these faults

They are unprivileged and deterministic. A read-only parent directory is *the
backup cannot be written*; `ulimit -f` is *the backup does not fit*, which is
the capacity fault without a filesystem to fill; a symlink out of the profile is
*deletion reaching data it was never pointed at*; a running daemon is the
process race. None need root, so they run where the developer runs.

## Three mistakes that shaped the harness

**The first version drilled the developer's own library.** It ran with the
working directory inside the checkout, so `notriosctl` resolved *source mode*,
where every root is `data` relative to the working directory. The notes went
into this repository's `data/`, and purge backups landed in the repository root.
Nothing was destroyed, and only because purge refuses a relative path as a
deletion target — *"a purge target must be absolute"*. `install_home` now
refuses to drill unless the resolved mode is `installed`.

**The first library check could not tell data loss from its own tooling.** It
asked the installed `notriosctl` whether the note survived — and purge removes
that binary, so every successful purge reported "the library is gone" whether or
not it was. The library is checked on the filesystem now.

**`tar -tf | grep -q` lies under `pipefail`.** `grep -q` exits at the first
match, `tar` dies of SIGPIPE, and the pipeline reports failure — so a backup
that *contained* the library was reported as one that did not.

Each of those produced a confident wrong answer rather than an error, which is
the failure mode a harness for destructive operations can least afford.

## The profile race

Fixed, not worked around: startup opens only the database it is starting.
`REPORT.json` records what the narrowing gave up, and
`internal/profiles/runtime_race_test.go` pins it. Racing two daemons is *not*
how it is asserted, and the report says why — the window is milliseconds and the
unfixed code passed five consecutive runs.

## What this does not establish

`REPORT.json` carries the list and the validator fails if it shrinks: a
genuinely full filesystem, mount races, interruption mid-purge, multi-user or
root-owned installations, and the packaged (`apt`) lifecycle, which the I3
matrix covers separately.
