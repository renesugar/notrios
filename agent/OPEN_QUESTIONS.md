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

17. Archive v2 stores one immutable object per revision, resource, and source
    bundle, and `manifest.json` lists every object inline. With the documented
    10,000-object and 4 MiB-manifest limits, one archive holds roughly 9,900
    revisions — far below the million-note libraries the J3 importer handles,
    so v2 cannot yet back up a large library. Measured evidence is under
    `performance/v0.4-p3/` (5,000 notes consumed 50.5 % of the object budget).
    Raising `MaxObjects` alone does not work because the inline inventory would
    then exceed the manifest bound. Should a format revision move the object
    inventory into its own checksummed `records`-style object (and if so,
    behind which declared capability), or should full backups of large
    libraries use a different container? P3 deliberately did not widen the
    limits: the exporter fails with an explicit object-budget error before
    publishing any manifest.

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
