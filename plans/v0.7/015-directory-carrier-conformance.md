# v0.7 G12 — directory-carrier conformance over Google Drive and removable media

Status: **complete**, 2026-08-13. Product remains 0.6.0 and the canonical schema
remains **v24**. **No production code changed**: G12 is evidence. Implemented by
Claude Opus 5 (`claude-opus-5`) under Claude Code, after explicit user approval
naming G12.

## Goal

Prove the shared-directory adapter against the available mapped cloud folder and
a USB-like handoff without making either provider part of the engine.

## What landed

`performance/v0.7-g12/cmd/conformance` drives `internal/synccarrier` exactly as
`notriosctl sync` does, against a Google Drive folder mounted with `rclone
mount` and against a directory that is alternately available to each peer. It
adds no product code and no dependency: a failing phase means the protocol is
wrong, not the harness.

It measures the provider first — visibility delay, partial-write exposure,
rename, case, mtime, name length, throughput — and then runs eight protocol
phases through it. Its refusal list is enforced in code: `rclone sync`,
`bisync`, `move`, `moveto`, `delete`, `deletefile`, `purge`, `rmdir`, and
`rmdirs` cannot be invoked against protocol state, and a phase asserts the
refusal rather than trusting the comment.

`docs/operations.md` gains the operator section the plan asked for: the safe
commands, the never-run list with the reason each one is dangerous, the measured
provider behavior, and a symptom-to-action table.

## What the provider does

| | Observed |
|---|---|
| Another device's file becomes visible in a listing | 55,549 / 56,664 / 56,392 ms |
| The same file resolved by name | 55,548 / 56,662 / 56,390 ms |
| A half-written file exposed to other readers | not observed |
| `rename` | available |
| Filename case | preserved and distinct |
| mtime preserved across a rename | true in one run, false in another |
| Throughput through the mount | 56.7 MiB/s write, 23.1 MiB/s read |

**A minute is the floor.** Nothing above the provider can be fresher than its
change-notification cadence, and G11 already showed the carrier layer is about
1% of an exchange. **Asking by name is no fresher than listing**, so the obvious
optimization — fetch the object whose address you already know — does not exist
here. Both figures are now recorded in `SYNCHRONIZATION.md`, because they
constrain G15's scheduling rather than merely describing a drive.

## The result worth the milestone

**Publication order is a latency optimization on a cloud folder, not a
correctness mechanism.** G11 publishes envelopes first and advertises last so
that everything an advertised vector implies is already readable — which is the
publisher's to decide on one filesystem and *not* the publisher's to decide on
an eventually-consistent provider. A dedicated phase makes the advertisement
visible while its envelopes are not, and asserts the reader admits nothing,
records no progress it did not make, and converges on the next ordinary round.
Correctness was always the state vector's; this is the run that proves the
ordering was never load-bearing.

## The boundary the conflicting-name phase found

An artifact substituted while a peer still needs it is republished by its owner
on the next round. An artifact substituted *after* every peer has admitted what
it carried is not — and should not be, since nobody is waiting for it and
cleanup removes it once acknowledged. The first version of this phase asserted
the wrong one and failed, which is how the boundary came to be written down
rather than assumed.

## Validation

Evidence is `performance/v0.7-g12/` (`conformance-results.json`, `README.md`,
`FINDINGS.md`, `validate_evidence.py`). All eight phases passed against the
mounted provider:

- 12 notes converged in 3 rounds, 21.7 s;
- 3 of replica A's artifacts unchanged byte for byte after replica B's rounds;
- 2 entries refused at a substituted name, then 4,473 bytes republished by the
  owner and both replicas converged;
- an advertisement without its envelopes admitted nothing and claimed nothing;
- a disconnected carrier refused and did not create the absent mount point,
  then converged after reconnecting;
- the carrier deleted in full rebuilt and reconverged in 3 rounds, 20.9 s;
- 5 artifacts copied out and back with `rclone copy --immutable` unchanged, and
  the destructive verbs refused;
- 3 notes out and 2 back on a drive only one peer can see at a time.

Repository validation ran audit-first per `AGENTS.md`.

## Boundaries kept

No cloud API or OAuth integration, no mobile rclone, and no rclone dependency in
the product — the harness is under `performance/`. Nothing was written to the
provider except generated sealed artifacts, and the folder was removed after the
run; libraries stayed on local disk throughout. No cloud contents and no private
notes are committed, and the results file records aggregates and behaviors
rather than paths or content. No provider limitation forced a protocol change,
which is what the plan required: had one, it would have reopened G11 rather than
becoming an rclone-specific workaround.

## What this does not prove

One provider, one mount configuration, one network, one day, and one host —
a second machine's cache behavior is approximated by publishing through the
provider API and reading through the mount. Twelve notes, because this measures
a carrier and not a library; G11 holds the scale evidence and G20 owns the
large-corpus convergence run.
