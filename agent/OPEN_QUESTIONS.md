# Open Questions

Resolved historical questions are recorded in archived plans and the attempt
log. Current implementation-affecting questions:

## Existing local product

1. Long-term SQLite driver: keep the cgo/libsqlite3 adapter or approve a
   compatible pure-Go driver migration as a dedicated cleanup slice?
2. Which official MCP Go SDK/version should replace the dependency-free
   adapter, and what compatibility fixtures freeze current tool semantics?
3. On regular-notebook deletion, should notes continue to move to Trash
   (current implemented/spec behavior), or be rehomed live to parent/Notes?
4. FTS5 is currently the always-on baseline and Recoll is optional. Is there
   any product requirement that would justify making external Recoll mandatory?
5. Which perceptual-hash algorithm should eventually occupy the implemented
   H5 hook? Core ships none; any future choice remains optional and
   suggest-only.

## Pagination, publishing, and mobile

6. For relevance queries that cannot reproduce a stable `(score,id)` keyset,
   should H7 use bounded server result snapshots or explicitly cap deep
   relevance traversal?
7. Which v0.4 large-site search adapter wins the measured
   Bluge/Recoll/SQLite-FTS spike, and who owns maintenance if upstream is stale?
8. What Wails v3 stability/release threshold is required before an Android
   spike can propose migrating the Wails v2 desktop shell?

## Native archive v2 format bounds

17. **Resolved 2026-08-04; scheduled as plan task P3a.** Archive v2 stores one
    immutable object per revision, resource, and source bundle, and
    `manifest.json` lists every object inline at ~645 bytes each. The 4 MiB
    manifest bound binds before the 10,000-object limit and caps one archive
    near 6,500 objects (~6,400 notes), so v2 cannot archive the supplied
    382,206-note corpora at all.

    Decision: move the object inventory into checksummed `index` objects that
    use the same bounded JSONL chunking as records, keeping the commit digest
    binding every object hash transitively. Index entries carry a discriminated
    `location` so a packed-object layout can follow behind an optional
    capability without a second breaking revision. Limits are re-derived from a
    1,000,000-note target, and both the writer's object dedup state and the
    verifier's cross-reference state move to an indexed temporary spool. The
    index form becomes the only v2 form; fixtures are regenerated rather than
    carrying an inline-inventory compatibility path, since no archive exists
    outside this repository and P6 has not pinned the format.

    Still open and gated on P3a measurement: whether loose objects remain
    viable at ~1.6M files, or whether a packed-object layout is needed before
    P6 and the v0.7 rclone/folder transport.

## Synchronization (resolve in separate v0.7 plans)

9. Which records use per-field versus whole-record LWW?
10. Are concurrent set membership add/remove operations add-wins, remove-wins,
    or LWW per membership row (current proposal)?
11. What are the default offline retention horizon and peer-retirement UX?
12. Is end-to-end encryption mandatory in protocol v1 above REST/rclone, and
    how are writer keys enrolled/recovered?
13. Which deterministic envelope encoding/compression is protocol v1?
14. What blob size triggers fixed chunking, and what measurements justify
    FastCDC later?
15. What deterministic notebook-cycle repair rule best preserves user intent?
16. What envelope/blob/pending limits are safe on the first real Android
    target?

See `SYNCHRONIZATION.md` for the proposed defaults and validation needed before
these choices become implementation contracts.
