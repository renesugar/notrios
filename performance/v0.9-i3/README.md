# v0.9 I3 — the Ubuntu installer, actually installed

H6a built the package and read it. Its claim ladder called that level 2,
"structurally inspected", and it listed *"whether the arm64 .deb installs or
runs"* among the things it had not verified — the amd64 one had not been
installed either. Nothing installed it in the eight items since.

This item installs it, in clean Ubuntu containers with no source tree, no Go, no
Node and no compiler, and runs the application as an unprivileged user. That
combination is what makes the answer mean anything: the machine that builds a
package can hide every missing dependency, and running as root can hide every
permission mistake.

## Running it

```sh
bash performance/v0.9-i3/build_packages.sh    # release and prerelease packages
bash performance/v0.9-i3/run_matrix.sh        # seven scenarios, one container each
python3 performance/v0.9-i3/validate_evidence.py
```

The first two need Docker and a build toolchain. The validator needs neither:
it checks the committed record, which is why it can run in `make validate`
everywhere.

## What each scenario is for

| Scenario | The question it answers |
| --- | --- |
| `01-fresh-install` | Are the declared dependencies sufficient on a pristine image, and does `doctor` pass with no keyring? |
| `02-no-toolchain-operation` | Can someone create, find and **export** notes where nothing can be compiled? |
| `03-upgrade-prerelease-to-release` | Does the binary change and the library survive? |
| `04-downgrade-refusal` | Is going backwards refused unasked, and possible when asked, with data intact either way? |
| `05-remove-and-reinstall` | Does removing the package leave the user's notes alone? |
| `06-profile-discovery` | Is a named profile found again, and do writable roots stay out of `/usr`? |
| `07-source-layout-migration` | Does a pre-0.8 `./data` library move into the resolved roots? |

## Two mistakes worth keeping

The first draft of `06` forbade `/usr` outright. That is wrong in one direction
and right in the other: the packaged frontend *belongs* in `/usr/share/notrios`,
read-only and replaced on upgrade, while every root the user writes to must stay
out of it. The check now asserts both halves rather than one.

The first draft of `02` ran the export with invented flags and swallowed the
failure with `|| true`, then recorded `export_written=no` as though that were a
finding. A scenario that reports a failure as an observation is worse than one
that has no scenario, because it looks like coverage. The export is asserted
now, manifest and all.

## What this does not establish

`REPORT.json` carries the list and the validator fails if it shrinks. In short:
arm64 is untouched, only 24.04 was tested, the GUI was never launched, and a
container is not a machine — it shares the host kernel and has no init, no
D-Bus and no keyring, which is exactly why the headless credential path is the
one these runs exercise.
