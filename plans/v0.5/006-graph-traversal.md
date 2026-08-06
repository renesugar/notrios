# v0.5 E4 — Graph traversal, paths, and visualization data

Status: complete on 2026-08-06.

Model: Claude Opus 5 (Claude Code).

## Why

The MVP graph endpoint returned the roots' immediate links. That was enough to
draw a note's neighbours in a sidebar and nothing else: no second hop, no route
between two notes, and no way to ask which notes the library has forgotten
about.

The document reconciliation that preceded this slice found the sharper version
of the problem. `POST /api/v1/graph` **declared** `depth` — in
`api/openapi.yaml` with a minimum, a maximum, and a default, and through
`internal/api.GraphRequest` into `store.GraphRequest` — and `store.Graph` never
read the field. A caller asking for three hops received one, with nothing in the
response saying so. The published node and edge defaults (250/500) also
disagreed with the implemented ones (100/200).

Accepting a parameter and ignoring it is worse than rejecting it. The caller
cannot tell a shallow graph from a small one, so the failure is invisible on
exactly the libraries where it matters.

## What it does

| Surface | Answers |
|---|---|
| `POST /api/v1/graph` | the neighbourhood of up to 100 roots, `depth` hops out |
| `POST /api/v1/graph/path` | a shortest link path between two notes, or why there is none |
| `GET /api/v1/graph/report` | orphans, isolates, and in-degree hubs across a collection |

`store.Graph`, `store.GraphPath`, and `store.GraphReport` are the Store
operations underneath. No MCP tool and no CLI command: MCP profile expansion is
v0.6's, and this is a client-rendering feature rather than a maintenance one.

## Decisions worth recording

**A bound wider than a ceiling is refused, not clamped.** Depth 5, 100 roots,
5,000 nodes, 20,000 edges. Asking for more returns `validation_failed` naming
the ceiling. Clamping is what produced the original defect in a different form:
a caller who asked for 50,000 nodes and got 5,000 has no way to know.

**A traversal stopped by a ceiling names it.** `truncated_by` is `nodes` or
`edges`, and `completed_depth` is the deepest level expanded in full, beside
`requested_depth`. A partial neighbourhood is never presented as a complete one.

**`no_path` is a proof; `depth_exhausted` and `budget_exhausted` are not.**
This is the decision that most shapes the path API. A single status would let a
large library answer "these two notes are unconnected" when the search simply
ran out of hops or visits. `no_path` means everything reachable was searched.
The other two mean the search stopped, and raising a bound may change the answer.

**Expansion is level by level in Go, not a recursive CTE.** A recursive SQL walk
decides how far it has gone only after SQLite has already walked, so a ceiling
would bound the *result* rather than the *work*. One bounded `IN (...)` query per
400-ID batch of the frontier — the selection planner's batch size — makes
MaxNodes and MaxEdges limits on effort.

**Path search runs from both ends.** A note graph fans out fast: a single-ended
BFS visits on the order of b^d nodes where a bidirectional one visits 2·b^(d/2).
Whole levels are expanded alternately, always the cheaper frontier, and the
minimum over every meeting node found at that level is the shortest path.

**Node metadata is batch-loaded, never per node.** `getDocumentLocked` joins the
current revision and returns the body; using it would have pulled thousands of
note bodies to label a graph. Traversal reads id, collection, and title only.

**A hub is ranked by in-degree, not total degree.** Out-degree describes how one
author wrote one note. In-degree describes how the rest of the library refers to
it, and that is the property that makes a note a hub.

**Trashed notes are not neighbours, endpoints, or waypoints.** Soft delete
already clears a note's outgoing links; the traversal additionally refuses to
follow a link *into* a trashed note, and a trashed endpoint is `not_found`.

**The service returns nodes, edges, and depths — no layout.** No coordinates, no
clustering. Putting layout here would make every client inherit this one's
opinion, and `E4`'s plan text asked for the opposite.

## What this slice does not do

- No MCP tool. `get_document_graph` stays on the v0.6 list with the rest of the
  profile work.
- No centrality or community detection. Degree is what the link table supports
  cheaply; anything else needs a measured need and probably the LadybugDB
  question, which stays research.
- No GUI graph pane. `UI_DESIGN.md`'s four-pane layout has no graph region and
  E4 asked for renderable data, not a renderer.

## Evidence

`performance/v0.5-e4/`, generated at 10k/100k/500k by
`scripts/run_large_library_profile.sh`. The 500k tier carries 1,000,000 links.

| Tier | Notes | Links | Neighbours d=1 | Neighbours d=5 | Path found | Report | Peak RSS |
|---|---:|---:|---:|---:|---:|---:|---:|
| 10k | 10,000 | 20,200 | 1.04 ms | 18.57 ms | 2.19 ms | 0.21 s | 27 MB |
| 100k | 100,000 | 200,200 | 0.95 ms | 20.43 ms | 1.85 ms | 3.92 s | 88 MB |
| 500k | 500,000 | 1,000,200 | 0.72 ms | 19.16 ms | 2.03 ms | 21.6 s | 376 MB |

The bounded operations are flat across a fifty-fold library, which is the
property the ceilings exist to produce: a depth-5 expansion of 221 nodes costs
the same 19 ms at half a million notes as at ten thousand, because the work is
proportional to the neighbourhood rather than to the graph. Both frontier
queries resolve to covering index searches, and peak RSS at 500k is unchanged
against the same tier before this slice.

The whole-library report is linear, like lint (0.21/3.92/21.6 s against lint's
0.14/3.01/15.3 s on the same run), and is reported as measured rather than as a
target. Its memory is flat: two capped example lists and a `limit`-sized heap.

One honest caveat is recorded with the numbers: the generated library is a
circulant graph (every note links to n+1 and n+2), so a BFS frontier grows
linearly rather than exponentially. The neighbourhood and path timings are
therefore a floor for a densely cross-linked library. What the tier does exercise
at full size is the index behind every hop — both frontier queries are covering
index searches — and the report's whole-library scan.

## Validation

`go vet ./...`, `go test ./...`, required-files, scaffold validation, OpenAPI
parse, migration-copy equality, web typecheck/tests/build, `make gui`, docs-site
build, REST/MCP smoke, performance smoke, and the three generated graph profiles.
