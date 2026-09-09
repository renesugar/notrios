# v0.7 G7 — note revision objects, transfer deltas, three-way merge, and conflicts

Status: **complete**, 2026-08-12. Product remains 0.6.0; the canonical schema is
now **v22**. Implemented by Claude Opus 5 (`claude-opus-5`) under Claude Code,
after explicit user approval naming G7.

## Goal

Synchronize note bodies without silently overwriting concurrent edits and
without making every small edit transfer the entire body.

## What landed

**A revision became an object.** `document_revisions` gained `content_sha256`,
`content_length`, and `parent_revision_ids`, so every revision names its exact
content and its place in the note's history. `store.insertRevisionLocked` is now
the single place a revision becomes a row, and all seven existing write paths —
create, update, trash, revision restore, duplicate, and both importer paths —
go through it. Archive restore computes the same identity from the restored
bytes rather than trusting a field.

**The capture trigger refuses an unhashed revision.** An enrolled database
aborts an insert whose `content_sha256` is missing or malformed. A revision
without one could not be verified after transfer, could not be a delta base, and
could not be found as a merge ancestor, so it is refused where it would be
created rather than discovered later as a gap.

**`internal/syncdelta`** is the reviewed promotion of the G1a prototype:
constrained RFC 3284 default-table VCDIFF over a Subversion-style match finder,
with production bounds, a benefit gate, and a `Delta` type bound to both
endpoints by exact hash. The promotion dropped the unselected private `NXD1`
container and the buffering stream wrappers. `EncodeBodyDelta` never publishes a
delta it cannot itself reconstruct; `ReconstructBody` verifies the named base,
decodes, and verifies the exact result hash and length before returning any
bytes.

**`internal/syncbody`** implements G1's selected merge — bounded line-first with
Unicode-aware word-token refinement — plus the revision DAG: heads, a merge-base
search whose cost tracks divergence rather than history, and the derived
identities that let two replicas compute one merge and one conflict.

**Admission converges bodies** in the same transaction that advances the state
vector, after G6 applies metadata, so a merge sees the settled title and
lifecycle. Remote revisions are materialized under the apply guard; a merge this
replica computes is journaled like any other local revision, because it is one.

**Schema v22** adds `sync_revision_deltas`, `sync_revision_pending_bodies`,
`sync_document_conflicts`, and the transient `sync_revision_transfer` hand-off.
The upgrade backfills content identity and a synthesized parent chain for every
existing revision.

## Decisions taken, and why

**`current_revision_id` is not a last-writer-wins field.** G6 already excluded it
from the metadata registers; G7 makes that deliberate. It is derived from the
revision graph — the single head, the merge that reduced two heads to one, or
the newer of two heads while a conflict stands. Two replicas evaluate the same
rule and display the same side, so a conflicted document does not additionally
disagree about what it is showing.

**A conflict's two revisions are stored sorted, not as "mine" and "theirs".**
Each replica calls a different one local. A conflict with two identities would
be reported twice and resolved once. The UI names the sides by comparing each
revision's authoring replica with its own.

**Conflicts and merges are derived, not transferred.** Both are a function of
the revision DAG, which both replicas hold. A merge is additionally *journaled*,
because a third replica that only ever sees the merge should not have to
recompute it — and because the derived identity makes the second copy to arrive
an exact replay that admission already discards.

**Delete/edit is answered exactly, not heuristically.** Every `document.trash`
operation already names the revision current when it was made. If that is the
revision the document ended up on, the deletion accounted for everything. If it
is anything else, edits existed the deleter never saw, and a `delete_edit`
conflict records what the losing intent was. A deletion made *after* seeing the
edit produces no conflict; asserted both ways.

**A body that fits no operation is named, not dropped.**
`sync_revision_pending_bodies` records `oversize`, `missing_base`, or
`unverified` with the revision's identity. `unverified` is terminal and no later
pass may downgrade it. The G8/G9 object path is what will clear the first two;
this slice makes the gap explicit rather than silent.

## Two defects found and fixed during implementation

Both were found by a randomized property test asserting that a clean merge never
invents a line and never drops one neither side changed.

1. **Word refinement resolved a structural disagreement.** With a two-line
   conflict region where each side deleted a different line, word merging
   "resolved" it by deleting both and keeping a stray prefix — text neither
   replica ever held (seed 29).
2. **Narrowing the guard to equal line counts was not enough.** Refinement then
   rejoined words across a line boundary into `L R line 5`, again a line neither
   replica held (seed 3306).

Refinement is now confined to a region that is exactly one line on all three
sides, which is the case G1 selected it for. Both cases are fixtures. Mutation
check: restoring either wider guard makes the randomized test fail at those
seeds.

A third defect was found by the schema test: the parent-chain backfill ordered
ties by revision `id`, which is random, so revisions sharing a `created_at`
second could be linked backwards. It orders by `rowid` — insertion order — now.

## Validation

Feature evidence is `performance/v0.7-g7/`
(`transfer-results.json`, `README.md`, `FINDINGS.md`, `validate_evidence.py`),
produced by real replicas running the production store.

- Transfer against the complete-body counterfactual at G1's four offline
  intervals: **55.37% / 27.92% / 16.49% / 12.06%** at 1 hour, 1 day, 1 week, and
  30 days. Every document converged on both replicas — current revision *and*
  exact bytes — at every interval, with no pending bodies and only `same_token`
  conflicts.
- `internal/syncdelta`: round trips over empty, identical, Unicode, NUL-bearing,
  repetitive, random-sparse and unrelated inputs; a pinned golden that both G1a
  oracles decoded; RFC target-copy overlap; 100 randomized round trips; every
  hostile case and every decoder limit; integer-overflow and expansion
  arithmetic; a fuzz target; the benefit gate; and reconstruction refusals for
  wrong base bytes, wrong named base, wrong base length, wrong named result,
  wrong result length, unknown format, every truncation, and every single-byte
  corruption.
- `internal/syncbody`: disjoint merges including CRLF, no-trailing-newline,
  Unicode and Markdown structure; word refinement; overlapping, delete/edit and
  bounds conflicts asserted in both delivery orders with the sides swapped;
  region location and bounding; every limit producing a typed conflict rather
  than silent truncation; invalid UTF-8 in each of the three inputs; tokenizer
  round trips; 4,000 randomized merges committed (50,000 run once); merge-base
  cases including uneven depth, an earlier merge, containment, unrelated roots,
  the divergence bound, and a long shared history that must *not* hit it; and
  replica-independent derived identities.
- `internal/store`: two-replica clean merge to one revision, durable conflicts
  with one identity on both replicas, conflict re-derivation, delta transfer and
  exact reconstruction, four broken-delta cases each with the accurate reason,
  delta bases always being complete local objects, tampered operations never
  reaching canonical storage, six delivery orders over three replicas,
  concurrent and sequential delete/edit, restore/edit, Unicode/Markdown/long-line
  end-to-end fixtures, the v22 upgrade backfill, and the unhashed-revision guard.

Repository validation, audit first as `AGENTS.md` requires:

```text
cd web && npm audit                                  0 vulnerabilities
cd web && npm ci && npm audit                        0 vulnerabilities
cd web && npm run typecheck                          pass
cd web && npm test -- --run                          15 files, 155 tests
cd web && npm run build                              pass
bash scripts/run_offline_assets_check.sh             no third-party request
go vet ./...                                         pass
go test ./...                                        pass
python3 scripts/check_required_files.py              70 files
python3 scripts/check_plan_loops.py                  pass
bash scripts/validate-scaffold.sh                    pass
bash scripts/build_docs_site.sh                      15 pages
bash scripts/mvp_smoke.sh                            pass
bash scripts/run_performance_smoke.sh                pass
python3 performance/v0.7-g7/validate_evidence.py     4 intervals
git diff --check                                     clean
scripts/package_release.sh + check_release_zip.py    verified
```

## Out of scope, and still owned elsewhere

No resource bytes (G8), no carrier or wire codec, no cryptography or signatures
(G9), no snapshot catch-up (G10), no transport (G11–G14), and no REST, MCP, or
UI surface for revisions, deltas, or conflicts (G16 owns the conflict UI). The
`oversize` and `missing_base` pending states are the explicit handoff to G8 and
G9's object transfer. G8 remains unapproved.
