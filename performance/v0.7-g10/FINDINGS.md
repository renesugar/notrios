# G10 findings

## What catch-up avoids

Two replicas, one built from a snapshot and one replaying everything:

| Library | Total operations | In the snapshot | After it | Full replay | Tail replay | Work avoided | Time saved |
|---|---:|---:|---:|---:|---:|---:|---:|
| 200 notes | 440 | 400 | 40 | 2,231 ms | 219 ms | 90.91% | 90.18% |
| 1,000 notes | 2,200 | 2,000 | 200 | 20,790 ms | 1,274 ms | 90.91% | 93.87% |

The work avoided is fixed by the fixture — nine tenths of the history is inside
the snapshot in both cases — so the number to read is the *time*, and it tracks
the work rather than merely correlating with it. At a thousand notes replaying
the whole history took twenty seconds and the tail took one and a quarter.

Time saved slightly exceeds work avoided at the larger size, and that is
expected rather than flattering: admission cost is superlinear in the history
already present, because each operation is folded against everything before it.
A replica that starts from a snapshot never pays that.

## The measurement found a real defect

The first evidence run failed outright: `admitted operation has no contiguous
HLC predecessor`.

G5 admits an operation only when the row for the sequence before it is present.
A replica built from a snapshot does not have those rows and *correctly should
not* — the snapshot is that history, already folded into canonical state. So the
first operation after the snapshot looked like a gap, and catch-up could not
resume at all.

The fix is a **catch-up floor**: cutover records, per replica, the sequence the
snapshot already accounts for. Admission consults it in exactly two places — the
missing-predecessor check, and the below-the-vector check that now reports an
already-contained operation as an inert duplicate rather than a conflict. The
floor is the only thing permitted to stand in for a missing predecessor, and a
test asserts that a replica *without* a snapshot still leaves the same operations
pending rather than admitting a history with a hole in it.

This is worth stating plainly because it was invisible from the unit tests. Every
piece worked; the seam between the snapshot's vector and G5's operation set did
not, and only running the whole flow showed it.

## What the state machine refuses

- **Enrollment is not permission.** A peer may exchange operations and still not
  be allowed to answer a backup request, because answering means handing over a
  complete copy of the library. Permission is explicit, revocable, and requires
  the replica to still be an active enrolled peer.
- **Competing offers are chosen among, never merged.** The furthest-ahead usable
  offer wins, ties break by responder id, and the arrival order does not matter.
  Two snapshots are two different points in a library's history.
- **A restore in progress cannot re-fetch underneath itself**, and cannot be
  cancelled once canonical rows are being written. Expiry skips it for the same
  reason: a clock is not a reason to abandon a half-written library.
- **Cutover refuses without an explicit intent.** There is no default, because an
  unstated intent is exactly how a restore becomes destructive by accident.
- **Cutover writes no peer acknowledgement.** This replica holding a copy says
  nothing about what any *peer* has durably admitted, so a backup must not hold
  back collection. Asserted directly.
- **A wrong password is reported as a wrong password**, not as a corrupt archive,
  because the two lead a person to do entirely different things. Wrapping is
  Argon2id at 64 MiB, and two wraps of one key are not equal.

## What this does not cover

**The archive itself is not re-measured.** Producing, verifying, and restoring an
archive-v2 container is P2–P4's, measured there at 382,206 notes. G10 records
where a snapshot came from and what it contained; it does not re-implement the
container.

**No carrier.** Nothing here publishes a request or fetches an archive. G11's
shared directory and G14's REST data plane do that.

**One new direct dependency.** Password wrapping needs a memory-hard derivation,
which the Go standard library does not provide, so `golang.org/x/crypto/argon2`
is now a direct requirement. It was already in the module graph as an indirect
dependency of Wails, at the same version, and is BSD-3-Clause. G9's zero-
dependency position covered the artifact crypto and still does; this is the one
place a password, rather than a key, has to be turned into a key.
