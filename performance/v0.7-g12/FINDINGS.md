# G12 findings — the provider is slow, not hostile, and the slowness is the design constraint

## What the provider does

Google Drive, mounted with `rclone mount`, measured through the mount:

| | Observed |
|---|---|
| A file published through the provider API becomes visible in a **listing** | 55,549 / 56,664 / 56,392 ms |
| The same file resolved **by name** | 55,548 / 56,662 / 56,390 ms |
| Slowest listing across all samples in two runs | 56,664 ms |
| A partially written file exposed to another reader | not observed |
| `rename` | available |
| Filename case | preserved and distinct |
| Modification time preserved across a rename | **true in one run, false in another** |
| Protocol-length names (64 hex + suffix) | accepted |
| Throughput through the mount | 56.7 MiB/s write, 23.1 MiB/s read |

**The headline is a minute.** A change published by another device took
45–57 seconds to become visible, across two runs and six samples. That is not
a protocol cost — the carrier layer is about 1% of an exchange (G11) — it is the
provider's change-notification cadence, and nothing above it can go faster.

**Asking by name is no fresher than listing.** The two figures differ by one or
two milliseconds, every time. This kills an optimization that looks obvious from
the outside: you cannot dodge a stale directory listing by fetching an object
whose name you already know. Both go through the same cache, and both wait.

**The provider never showed a half-written file.** Four separate megabyte writes
through the mount, checked from the provider side mid-write, produced nothing
visible until the file was closed. That is one hostile case this provider does
not produce — and exactly why the protocol does not depend on the observation:
another provider, an interrupted copy, or a drive pulled mid-write still can.

**Modification time is not stable enough to be wrong consistently.** One run
preserved mtime across a rename, another did not, with no change in
configuration. Nothing in the protocol reads mtime; this is the measurement that
says it must stay that way.

## What passed, and what each phase actually proves

All eight phases passed against the mounted provider.

| Phase | Result |
|---|---|
| converge through the provider | 12 notes, 3 rounds, 21.7 s |
| peer files are never written by another peer | 3 of replica A's artifacts unchanged, byte for byte, after B's rounds |
| a conflicting file at a protocol name is refused and repaired | 2 entries refused, owner republished 4,473 bytes, both converged |
| an advertisement seen before its envelopes costs latency only | admitted nothing, claimed nothing, converged on the next round |
| an unavailable carrier refuses rather than diverging | refused while disconnected, converged after reconnecting |
| full carrier loss and reconstruction | rebuilt and reconverged in 3 rounds, 20.9 s |
| `rclone copy --immutable` moves the carrier without changing it | 5 artifacts out and back unchanged; sync/bisync/move/delete/purge refused |
| a drive passed between peers converges | 3 notes out, 2 back, one trip each |

Three of these are worth more than a tick.

**The advertisement-before-envelopes phase is the one this milestone exists
for.** G11 publishes envelopes first and advertises last, so that everything an
advertised vector implies is already readable. On one filesystem that ordering
is the publisher's to decide. On an eventually-consistent provider it is not:
the reader may see the advertisement and not the envelopes. The phase makes
exactly that happen and asserts the reader admits nothing, records no progress
it did not make, and converges on the next ordinary round. **Publication order
is a latency optimization on a cloud folder, not a correctness mechanism** — and
it was only ever claimed as one because the state vector is what actually
carries correctness.

**The conflicting-name phase found the boundary of repair.** An artifact
substituted while a peer still needs it is republished on the owner's next
round. An artifact substituted after every peer has admitted what it carried is
*not* republished — and should not be, since nobody is waiting for it and
cleanup removes it once acknowledged. The first version of this phase asserted
the wrong one and failed, which is how the boundary got written down.

**Nothing writes outside its own namespace, on a provider that renames things.**
Replica A's artifacts were fingerprinted by content hash before and after
replica B's rounds. A carrier where two peers share a mutable manifest would not
survive a provider whose listing lags by a minute; this one has no shared
mutable state to lag.

## What this does not prove

- **One provider, one mount, one network, one day.** Another provider, or
  `rclone mount` with different cache flags, will produce different numbers. The
  protocol properties above are what generalize; the milliseconds are not.
- **Not two machines.** A second device's cache behavior is approximated by
  publishing through the provider API and reading through the mount. That is the
  closest a single host gets, and it is not the same thing.
- **Not a large corpus.** Twelve notes, because this measures a carrier and not
  a library; G11 holds the scale evidence, and G20 owns the large-corpus
  convergence run.
- **No provider limitation forced a protocol change.** The plan required that a
  surprising limitation reopen G11 rather than hide in an rclone-specific
  workaround. None did: the harness added no product code, and every phase runs
  the shipped round.
