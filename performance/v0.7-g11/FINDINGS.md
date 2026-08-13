# G11 findings — the folder is not the expensive part

## What the numbers say

| | 200 notes | 1,000 notes |
|---|---|---|
| operations exchanged | 400 | 2,000 |
| rounds to converge | 3 | 3 |
| first exchange | 2,980 ms | 21,838 ms |
| quiet round (a poll with nothing to say) | 132 ms | 452 ms |
| artifacts left on the carrier | 5 | 5 |
| bytes on the carrier | 189,360 | 942,163 |
| carrier bytes ÷ database bytes | 0.114 | 0.183 |
| artifacts added by ten quiet rounds | 0 | 0 |
| seal, sign, and publish | 21 ms | 100 ms |
| read, verify, decrypt, and decode | 17 ms | 125 ms |
| operations in that carrier-layer measurement | 240 | 1,200 |
| rounds to reconverge after the carrier was deleted | 2 | 2 |

**The carrier layer is about one percent of an exchange.** Sealing, signing, and
writing 1,200 operations took 100 ms and reading them back took 125 ms, against
21,838 ms for the exchange as a whole. The rest is G5 admission committing to
SQLite. That is the number worth keeping, because it settles what a shared
folder costs relative to what synchronizing costs: the folder is close to free,
and making it faster would be optimizing the wrong end. The direct-admission
baseline in the JSON (`direct_exchange_ms`) is context for that claim, not a
benchmark — it is one run of each and the two are within the same order.

**A quiet carrier stops growing.** Ten further rounds with nothing new to say
added zero artifacts at both sizes. This is a design property rather than a
happy accident, and it is the second thing the measurement was built to check:
every seal draws a fresh random salt, so artifacts named by their sealed bytes
would leave a new file per round on a shared drive forever. Naming an artifact
by *what it logically is* — a keyed blind of the sender, the author, and the
sequence range it covers — makes republication a no-op that the round detects by
reading its own copy back.

**Five artifacts hold a thousand notes.** Two advertisements and three
envelopes. Envelopes close at G2's bounds, so this scales as operations rather
than as files, which is what the pack-layout work under P3b said the sync
transports would need.

**A deleted carrier costs two rounds.** The measurement deletes the folder with
a batch published and not yet read — deleting it when both replicas are already
level costs nothing, and measuring *that* would have flattered the design. What
comes back is republication from the durable journal: the sender still knows
what its peer acknowledged, so it republishes exactly the missing range rather
than the library.

**A scan is bounded, and the bound is visible.** Listing one class directory
took 362 µs at 100 artifacts, 3.9 ms at 1,000, and 50 ms at 5,000 — where it
returned 4,096, the cap, rather than everything present. A carrier that grows
without limit cannot make a round grow without limit.

## Two defects this work found

**A torn artifact could never be repaired.** The first implementation treated a
name that already existed as proof that the artifact was there, and skipped the
publish. That is correct for an immutable name and wrong for a real carrier: a
half-copied file keeps its name, so the peer waiting for it would wait forever
while the sender believed it had already sent it. Publishing now asks a
different question — is a *readable* copy there? — by reading its own artifact
back and opening it. The truncation test fails without this and passes with it.

**A fresh carrier was a standoff.** Publishing was driven only by advertisements
found on the carrier, so on an empty folder neither replica had anything to
publish for a peer it had not yet heard from, and the first exchange took an
extra full round trip. On removable media that is not a round trip but a
physical journey: the drive would have to visit the second replica once merely
to say hello. A round now starts from what the journal durably remembers about
each enrolled peer — the acknowledgement G5 already records — and lets a
carrier advertisement replace that memory when one is present. The removable
media test asserts the two-trip behavior directly.

## What this does not cover

- **No cloud provider and no removable device.** Delayed listings, provider
  filename mangling, partial uploads, and throughput over a mapped drive are
  G12's, and claiming them from a local filesystem would be exactly the
  substitution G12 exists to prevent.
- **No snapshot transfer.** The carrier reserves a snapshot class and the round
  never fills it; a new replica is still made by restoring an archive-v2
  snapshot with an explicit intent, and carrying that archive over the carrier
  belongs with G14's resumable download.
- **No scheduling.** Every round here is explicit. There is no watcher, no
  daemon integration, and no retry policy — G15 owns durable sync jobs, and a
  background scheduler added here would be untested surface.
- **One host, one filesystem, one run each.** Timings are wall clock on a
  desktop; they are not a mobile claim, and G2's emulator and physical-device
  checklists still gate that.
