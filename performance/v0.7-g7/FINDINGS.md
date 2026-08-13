# G7 findings

## Transfer

Two replicas, 24 generated notes, edits on both sides, measured through the
production journal:

| Offline interval | Edits/side | Revisions | With a delta | Transferred | Complete bodies | Transferred share |
|---|---:|---:|---:|---:|---:|---:|
| 1 hour | 1 | 48 | 24 | 176,664 B | 319,080 B | 55.37% |
| 1 day | 4 | 120 | 96 | 220,840 B | 790,968 B | 27.92% |
| 1 week | 12 | 312 | 288 | 338,728 B | 2,054,688 B | 16.49% |
| 30 days | 32 | 792 | 768 | 633,384 B | 5,253,408 B | 12.06% |

The share falls as divergence grows because the count of *revisions* grows while
each one still changes about one line. That is the shape the feature is for: a
replica that has been away for a month sends more messages, not bigger ones.

Every interval's first revision of each note is a creation with no parent, so it
has no base and travels complete. That is the whole of the 1-hour row's 24
non-delta revisions and most of why its share is the worst of the four: with one
edit per side, half of everything sent is the original note.

## Correctness

- **Every document converged at every interval**: 24 of 24, on both replicas,
  agreeing on the current revision id *and* the exact body bytes.
- **Every conflict was `same_token`.** No `bounds_exceeded`, no
  `invalid_utf8`, and no `delete_edit` in this workload — which is what should
  happen when the only overlapping edits are two replicas rewriting the same
  sentence. A `bounds_exceeded` conflict here would have meant a merge ran out
  of budget on an ordinary note, and none did.
- **No revision was left without bytes.** `pending_reasons` was empty at every
  interval: no `oversize`, no `missing_base`, no `unverified`. Every delta this
  run generated named a base its receiver already held and reconstructed to the
  exact named result.

## Two things the numbers do not say

**The merge graph's shape depends on delivery, and the run shows it.** At one
hour and one day there are exactly 16 merge revisions for the 16 disjoint
documents — one each, computed once and adopted by the other replica because the
identity is derived from the parents and the merged bytes. At one week there are
18 and at thirty days 27. The extra ones are real and expected: the harness
ships in pages of 500 operations, so a replica sometimes merges a *partial* view
of the other's chain, and the merge it computes is one the sender never had.
Those intermediate merges are ordinary revisions, they are exchanged like any
other, and the documents still converge — but the number of merge revisions is
not a function of the edits alone. Content is order independent; the graph is
not, exactly as in any DAG-based merge system.

The ninth conflict in the thirty-day row has the same cause. Eight documents
were built to overlap; the ninth is a disjoint document whose partial-view merge
met a head it overlapped with. It is a `same_token` conflict like the others,
visible and resolvable, not a lost edit.

**These are desktop numbers.** One host, one SQLite file per replica, no
carrier, no encryption, and no compression — G9 owns the codec and the envelope,
and it will change the bytes on the wire. The comparison that survives is the
*relative* one: what a delta costs against the complete body it replaces.

## Bounds this slice fixed

| Bound | Value | Why |
|---|---|---|
| Body object ceiling | 8 MiB | Far above the 326,359-byte largest body G1 saw in any corpus, far below G2's 16 MiB envelope. |
| Merge input ceiling | 1 MiB | A larger body still synchronizes; it conflicts instead of merging, which is visible rather than slow. |
| Line-token ceiling | 262,144 | Above a 1 MiB body of single-character lines. |
| Edit-distance ceiling | 1,024 per diff | Myers' trace costs (d+1)² integers, so this is a memory bound of about eight transient megabytes. G1's worst measured 30-day divergence was 32 edits per side. |
| Word-refinement region | 64 KiB, 32,768 tokens | G1 measured a 50 KiB repeated-token line at 3.8 ms per line and 181.9 ms per word. |
| Conflict regions | 256, then one whole-body conflict | Hundreds of separate conflicts is not something a person resolves region by region. |
| Trigger inline fallback | 65,536 raw body bytes | Worst-case JSON escaping is six bytes per byte, and 6 × 65,536 stays under G2's 512 KiB payload ceiling for *any* input. |
| Delta benefit gate | ≤75% of the body and ≥64 bytes saved | G1 measured line deltas between 2.49% and 10.90% of a full body; anything near 75% is a degenerate case. |
| Delta chain depth | 1 | A reconstructed body is stored complete, so a delta's base is always a complete local object and chains never accumulate. Asserted by test. |

## What word refinement will not do

Refinement is confined to a conflict region that is exactly one line on all
three sides. Two randomized cases forced that boundary during implementation,
and both are now fixtures:

- allowing a multi-line region let each side delete a different line, which word
  merging "resolved" by deleting both and keeping a stray prefix;
- allowing equal-but-multiple lines let refinement rejoin words across a line
  boundary into `L R line 5`, a line neither replica ever held.

Words can separate two edits inside one line. They cannot decide which lines
exist, and a merge that invents a line is worse than a conflict that asks.
