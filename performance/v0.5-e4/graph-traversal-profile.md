# v0.5 E4 — graph traversal scale profile

Date: 2026-08-06. Generated data only; no private corpus, no note content, no
local paths. Produced by `scripts/run_large_library_profile.sh`, which now
measures neighbourhood expansion, shortest-path search, and the whole-library
graph report at each tier.

Reference machine: Intel Core i5-9300H, Linux/amd64, Go 1.26.5, SQLite 3.45.1.

## Results

| Tier | Notes | Links | Neighbours d=1 (p95) | Neighbours d=5 (p95) | Path found (p95) | Graph report | Peak RSS |
|---|---:|---:|---:|---:|---:|---:|---:|
| 10k | 10,000 | 20,200 | 1.04 ms | 18.57 ms | 2.19 ms | 0.21 s | 27 MB |
| 100k | 100,000 | 200,200 | 0.95 ms | 20.43 ms | 1.85 ms | 3.92 s | 88 MB |
| 500k | 500,000 | 1,000,200 | 0.72 ms | 19.16 ms | 2.03 ms | 21.6 s | 376 MB |

The bounded operations are **flat across a fifty-fold library**, which is the
property the ceilings exist to produce. A depth-5 expansion of 221 nodes and 237
edges costs the same 19 ms whether the library holds ten thousand notes or half a
million, because the work is proportional to the neighbourhood rather than to the
graph. Both frontier queries resolve to covering index searches:

```
graph_outgoing_frontier  SEARCH document_links USING COVERING INDEX document_links_source_idx (source_document_id=?)
graph_incoming_frontier  SEARCH document_links USING COVERING INDEX document_links_target_document_idx (target_document_id=?)
```

Peak RSS at 500k is 376 MB against 376 MB for the same tier before this slice —
traversal adds nothing measurable, because it holds one frontier and one node
map bounded by `MaxGraphNodes`.

## What the generated graph is, and what that means for these numbers

The seeded library is a **circulant graph**: note *n* links to *n+1* and *n+2*, so
every note has in-degree 2 and out-degree 2 and the 500k tier carries 1,000,200
edges. That shape has to be read with the timings.

A BFS frontier in a circulant graph grows **linearly**, not exponentially: five
hops from one note reaches 221 nodes, and the far-path search visits 22 before
running out of depth. A densely cross-linked real library would hit the node and
edge ceilings much sooner, and the honest way to state that is: **the
neighbourhood and path timings above are a floor, not a worst case.** They are
still the right measurement for the question this tier can answer — whether the
index behind every hop holds at a million edges — and the ceilings are what
guarantee the upper bound the shape cannot demonstrate.

The upper bound is bounded by construction rather than by measurement: a request
can visit at most `MaxGraphNodes` (5,000) nodes and `MaxGraphEdges` (20,000)
edges, and a path search at most `MaxGraphPathVisits` (200,000) nodes. Reaching
either ends the traversal and says so.

## The report is linear, like lint

`GET /api/v1/graph/report` reads every note once and counts both its degrees:
0.21 s / 3.92 s / 21.6 s at 10k/100k/500k. That is the same shape and the same
order as `notriosctl lint` on the same run (0.14 s / 3.01 s / 15.3 s), and for
the same reason — it is a whole-library read.

Memory stays flat: two example lists capped at `limit` and a `limit`-sized heap
for the hubs. A sorted insert would have been O(documents × limit); the heap is
O(documents × log limit) and holds `limit` entries, which is what makes the
report bounded rather than merely capped at the end.

The remaining time is two indexed subqueries per note — one range over
`document_links_target_document_idx`, one over `document_links_source_idx` —
which is 1,000,000 index ranges at the 500k tier. If a future library makes 21 s
unacceptable, the next step is one grouped join per direction merged against the
document scan, not a restructure of the report.

Twenty-one seconds at half a million notes is a maintenance command's cost, not
an interactive one, and the API says so: `elapsed_ms` is in every response
precisely because this report reads the whole library and an operator should be
able to see what that cost on *their* library.

## Assertions the profile makes

Beyond timing, each tier asserts behaviour at scale:

- a depth-5 expansion completes at least one level and reports its truncation
  state;
- a note ten steps along the `+1`/`+2` circulant is exactly **five hops** away
  and comes back `found`;
- a note half the library away comes back **`depth_exhausted`**, not `no_path` —
  the distinction this slice exists to preserve, tested where it actually
  matters;
- the report's `link_count` equals the seeded edge count plus the block sample's
  links, and its `orphan_count` equals exactly the sample notes nothing links to.

Committed JSON: `graph-10k.json`, `graph-100k.json`, `graph-500k.json`.
